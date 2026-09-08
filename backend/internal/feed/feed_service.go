package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"log"
	"strconv"
	"sync"

	"time"

	"feed/backend/internal/like"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/video"
	"golang.org/x/sync/singleflight"
)

// followingCacheTTL 关注流响应缓存有效期。
//
// 背景：关注流（ListByFollowing）从 DB 实时 JOIN 关注关系作为事实源，本缓存只是
// 旁路缓存，用于卸载重复的 DB 查询。被关注作者发布新视频时不做逐粉丝失效扇出
// （扇出需遍历作者全部粉丝并对每个粉丝 SCAN 删键，大 V 场景成本高且受粉丝表
// 200 行单次上限影响），因此采用"短 TTL"策略兜底新视频可见性：
// 缓存最多滞后 followingCacheTTL 时长，即被关注作者新发布最迟该时长后出现在关注流。
// DB 始终是权威源，TTL 到点或缓存未命中都会自动回源 DB，语义完全正确、零扇出成本。
const followingCacheTTL = 60 * time.Second

type FeedService struct {
	repo         FeedRepositoryer
	likeRepo     like.LikeRepositoryer
	rediscache   *rediscache.Client
	localcache   *cache.Cache
	requestGroup singleflight.Group
}

type CachedFeedData struct {
	PublicVideos []video.Video `json:"public_videos"`
}

// go 语言接口一般不要用指针，因为接口本身已经是一个指针
func NewFeedService(repo FeedRepositoryer, likeRepo like.LikeRepositoryer, rediscache *rediscache.Client) *FeedService {
	return &FeedService{
		repo:         repo,
		likeRepo:     likeRepo,
		rediscache:   rediscache,
		localcache:   cache.New(3*time.Second, 5*time.Second),
		requestGroup: singleflight.Group{},
	}
}

// 通过视频ID查询视频信息 批量获取
func (f *FeedService) GetVideoByIDs(ctx context.Context, videoIDs []uint) ([]*video.Video, error) {
	// 采用 L1(本地缓存) -> L2(Redis) -> L3(MySQL) 三级架构
	if len(videoIDs) == 0 {
		return []*video.Video{}, nil
	}
	videoMap := make(map[uint]*video.Video)
	// 查l1
	var missedL1 []uint
	for _, id := range videoIDs {
		cacheKey := f.rediscache.VideoEntityKey(id)
		if f.localcache != nil {
			if v, found := f.localcache.Get(cacheKey); found {
				// 类型断言，校验缓存内数据是否为Video结构体
				if data, ok := v.(video.Video); ok {
					videoMap[id] = &data
					continue
				}
			}
		}
		missedL1 = append(missedL1, id)
	}
	// 查redis
	if len(missedL1) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}
	var missedL2 []uint
	if len(missedL1) > 0 {
		cacheKeys := make([]string, len(missedL1))
		for i, id := range missedL1 {
			cacheKeys[i] = f.rediscache.VideoEntityKey(id)
		}
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		results, err := f.rediscache.MGet(cacheCtx, cacheKeys...)
		cancel()
		// 成功， 校验数据
		if err == nil {
			for i, res := range results {
				id := missedL1[i]
				if res != nil {
					if str, ok := res.(string); ok {
						var v video.Video
						if err := json.Unmarshal([]byte(str), &v); err == nil {
							videoMap[id] = &v
							// 回写更新 L1 本地缓存
							if f.localcache != nil {
								f.localcache.Set(cacheKeys[i], v, 5*time.Second)
							}
							continue
						}
					}
				}
				missedL2 = append(missedL2, id)
			}
		} else {
			// 如果 Redis 挂了或者超时了，全部降级到 L3
			missedL2 = missedL1
			log.Printf("L2 Redis MGet 失败，全部降级到 MySQL: %v", err)
		}
	}
	// mysql
	if len(missedL2) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, id := range missedL2 {
		wg.Add(1)
		go func(videoID uint) {
			defer wg.Done()
			sfKey := f.rediscache.Key(rediscache.SFEntityFmt, videoID)

			v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
				videoList, err := f.repo.GetByIDs(ctx, []uint{videoID})

				if err != nil || len(videoList) == 0 {
					return nil, err
				}

				safeCopy := *videoList[0]
				cachekey := f.rediscache.VideoEntityKey(safeCopy.ID)
				if b, err := json.Marshal(safeCopy); err == nil {
					//异步回写redis
					go func(k string, b []byte) {
						setCtx, setCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
						defer setCancel()

						f.rediscache.SetBytes(setCtx, k, b, time.Hour)
					}(cachekey, b)
				}
				return videoList[0], err
			})

			if err == nil && v != nil {
				safeCopy := *(v.(*video.Video))
				mu.Lock()
				videoMap[id] = &safeCopy
				mu.Unlock()
				f.localcache.Set(f.rediscache.VideoEntityKey(safeCopy.ID), safeCopy, 5*time.Second)
			}
		}(id)
	}
	wg.Wait()
	return buildOrderedResult(videoIDs, videoMap), nil
}

// 查询最新视频 (冷热分离 + 游标分页)
func (f *FeedService) ListLatest(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
	// 如果redis为空，就从mysql游标查询最新的1000条视频数据
	if f.rediscache == nil {
		return f.listLatestFromDB(ctx, limit, latestBefore, viewerAccountID)
	}
	// 获取 ZSET 中最老的一条数据
	zsetTail, err := f.rediscache.ZRangeWithScores(ctx, f.rediscache.Key(rediscache.FeedGlobalTimelineKey), 0, 0)
	if err != nil {
		return f.listLatestFromDB(ctx, limit, latestBefore, viewerAccountID)
	}
	// 判断ZSet是否为空，空代表缓存还未初始化，需要重建全局时间线ZSet 全量重建机制
	isZsetEmpty := len(zsetTail) == 0

	if isZsetEmpty {
		sfKey := f.rediscache.Key(rediscache.SFTimelineRebuildKey)

		v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
			// 无视游标，直接去 MySQL 捞最新的 1000 条
			dbVideos, err := f.repo.ListLatest(ctx, 1000, time.Time{})
			if err != nil {
				return nil, err
			}
			if len(dbVideos) == 0 {
				return "EMPTY_DB", nil // 防无限递归
			}

			// 重建 ZSET
			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var zElements []redis.Z
			for _, vid := range dbVideos {
				zElements = append(zElements, redis.Z{
					Score:  float64(vid.CreateTime.UnixMilli()),
					Member: fmt.Sprintf("%d", vid.ID),
				})
			}
			f.rediscache.ZAdd(bgCtx, f.rediscache.Key(rediscache.FeedGlobalTimelineKey), zElements...)
			return "SUCCESS", nil
		})

		if err != nil {
			return ListLatestResponse{}, err
		}
		if v == "EMPTY_DB" {
			return ListLatestResponse{HasMore: false}, nil
		}

		// 让所有被阻塞的请求重新查一遍
		return f.ListLatest(ctx, limit, latestBefore, viewerAccountID)
	}

	// watermark：冷热分界线，ZSet中保存的最老视频的毫秒时间戳
	watermark := int64(zsetTail[0].Score)
	reqTime := time.Now().UnixMilli()
	if !latestBefore.IsZero() {
		reqTime = latestBefore.UnixMilli()
	}
	var baseVideos []*video.Video
	if reqTime <= watermark {
		//冷数据降级查库
		sfKey := f.rediscache.Key(rediscache.SFColdListLatestFmt, limit, reqTime)
		v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
			return f.repo.ListLatest(ctx, limit, latestBefore)
		})
		if err != nil {
			return ListLatestResponse{}, err
		}
		baseVideos = v.([]*video.Video)
		// 不回写 ZSET，防止冷数据污染热点时间线
	} else {
		// 热数据直接查redis
		maxScore := "+inf"
		if !latestBefore.IsZero() {
			maxScore = fmt.Sprintf("%d", reqTime-1) // 防重复
		}
		// 按照时间查询
		videoIDsStr, err := f.rediscache.ZRevRangeByScore(ctx, f.rediscache.Key(rediscache.FeedGlobalTimelineKey), maxScore, "-inf", 0, int64(limit))
		if err != nil {
			return ListLatestResponse{}, err
		}

		var videoIDs []uint
		for _, idStr := range videoIDsStr {
			if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
				videoIDs = append(videoIDs, uint(id))
			}
		}

		if len(videoIDs) > 0 {
			//在通过视频查询视频信息
			baseVideos, err = f.GetVideoByIDs(ctx, videoIDs)
			if err != nil {
				return ListLatestResponse{}, err
			}
		}

		// 刚好击穿了冷热边界
		if len(baseVideos) < limit {
			remainLimit := limit - len(baseVideos) // 计算还差几个

			var coldCursor time.Time
			if len(baseVideos) > 0 {
				coldCursor = baseVideos[len(baseVideos)-1].CreateTime
			} else {
				coldCursor = latestBefore
			}

			sfKey := f.rediscache.Key(rediscache.SFStitchListLatestFmt, remainLimit, coldCursor.UnixMilli())
			v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
				// 查询之前的数据
				return f.repo.ListLatest(ctx, remainLimit, coldCursor)
			})

			if err == nil {
				coldVideos := v.([]*video.Video)
				baseVideos = append(baseVideos, coldVideos...)
			}
		}
	}

	var nextTime int64
	if len(baseVideos) > 0 {
		// 将本页最后一条视频的时间作为下一次请求的游标
		nextTime = baseVideos[len(baseVideos)-1].CreateTime.UnixMilli()
	}
	hasMore := len(baseVideos) == limit

	feedVideos, err := f.buildFeedVideos(ctx, baseVideos, viewerAccountID)
	if err != nil {
		return ListLatestResponse{}, err
	}

	return ListLatestResponse{
		VideoList: feedVideos,
		NextTime:  nextTime,
		HasMore:   hasMore,
	}, nil
}

// listLatestFromDB 从数据库获取最新视频列表（基于创建时间降序），并填充当前用户的点赞状态。
func (f *FeedService) listLatestFromDB(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
	videos, err := f.repo.ListLatest(ctx, limit, latestBefore)
	if err != nil {
		return ListLatestResponse{}, err
	}
	feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListLatestResponse{}, err
	}
	// nextTime 获取的是最旧的时间值， timeline feed ,  从最新不断获取之前旧的视频
	var nextTime int64
	if len(videos) > 0 {
		nextTime = videos[len(videos)-1].CreateTime.UnixMilli()
	}
	return ListLatestResponse{
		VideoList: feedVideos,
		NextTime:  nextTime,
		HasMore:   len(videos) == limit,
	}, nil
}

// 按照点赞数查询视频
func (f *FeedService) ListLikesCount(ctx context.Context, limit int, cursor *LikesCountCursor, viewerAccountID uint) (ListLikesCountResponse, error) {
	videos, err := f.repo.ListLikesCountWithCursor(ctx, limit, cursor)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	hasMore := len(videos) == limit
	feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	resp := ListLikesCountResponse{
		VideoList: feedVideos,
		HasMore:   hasMore,
	}
	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextLikesCountBefore := last.LikesCount
		nextIDBefore := last.ID
		resp.NextLikesCountBefore = &nextLikesCountBefore
		resp.NextIDBefore = &nextIDBefore
	}
	return resp, nil
}

// 按照关注列表查询视频
// 构建一个高并发场景下的“关注流”读服务。它采用了经典的 Cache-Aside（旁路缓存） 模式，并深度融入了防缓存击穿和快速失败的弹性设计
func (f *FeedService) ListByFollowing(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListByFollowingResponse, error) {
	doListByFollowingFromDB := func() (ListByFollowingResponse, error) {
		// 查询当前用户关注作者发布的视频
		videos, err := f.repo.ListByFollowing(ctx, limit, viewerAccountID, latestBefore)
		if err != nil {
			return ListByFollowingResponse{}, err
		}
		var nextTime int64
		if len(videos) > 0 {
			nextTime = videos[len(videos)-1].CreateTime.Unix()
		} else {
			nextTime = 0
		}
		hasMore := len(videos) == limit
		feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
		if err != nil {
			return ListByFollowingResponse{}, err
		}
		resp := ListByFollowingResponse{
			VideoList: feedVideos,
			NextTime:  nextTime,
			HasMore:   hasMore,
		}
		return resp, nil
	}
	var cacheKey string
	if viewerAccountID != 0 && f.rediscache != nil {
		// before 游标时间戳，0表示首页，不做时间过滤
		before := int64(0)
		if !latestBefore.IsZero() {
			before = latestBefore.Unix()
		}
		cacheKey = f.rediscache.Key(rediscache.FeedFollowingFmt, limit, viewerAccountID, before)
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		b, err := f.rediscache.GetBytes(cacheCtx, cacheKey)
		if err == nil {
			var cached ListByFollowingResponse
			if err := json.Unmarshal(b, &cached); err == nil {
				return cached, nil
			}
		} else if rediscache.IsMiss(err) { // 缓存未命中
			// 锁竞争 + 自旋等待（Spin-Wait）+ 锁超时兜底
			lockKey := rediscache.LockKeyFor(cacheKey)
			token, locked, _ := f.rediscache.Lock(cacheCtx, lockKey, 500*time.Millisecond)
			if locked {
				defer func() { _ = f.rediscache.Unlock(context.Background(), lockKey, token) }()
				if b, err := f.rediscache.GetBytes(cacheCtx, cacheKey); err == nil {
					var cached ListByFollowingResponse
					if err := json.Unmarshal(b, &cached); err == nil {
						return cached, nil
					}
				} else { // 缓存未命中，从数据库中查询
					resp, err := doListByFollowingFromDB()
					if err != nil {
						return ListByFollowingResponse{}, err
					}
					if b, err := json.Marshal(resp); err == nil {
						_ = f.rediscache.SetBytes(cacheCtx, cacheKey, b, followingCacheTTL)
					}
					return resp, nil
				}
			}
		} else {
			for i := 0; i < 5; i++ {
				time.Sleep(20 * time.Millisecond)
				if b, err := f.rediscache.GetBytes(cacheCtx, cacheKey); err == nil {
					var cached ListByFollowingResponse
					if err := json.Unmarshal(b, &cached); err == nil {
						return cached, nil
					}
				}
			}
		}
	}
	resp, err := doListByFollowingFromDB()
	if err != nil {
		return ListByFollowingResponse{}, err
	}
	if cacheKey != "" {
		if b, err := json.Marshal(resp); err == nil {
			cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			defer cancel()
			_ = f.rediscache.SetBytes(cacheCtx, cacheKey, b, followingCacheTTL)
		}
	}
	return resp, nil
}

// ListByPopularity 按热度查询热门短视频榜单（冷热分离方案）
func (f *FeedService) ListByPopularity(ctx context.Context, limit int, reqAsOf int64, offset int, viewerAccountID uint, latestPopularity int64, latestBefore time.Time, latestIDBefore uint) (ListByPopularityResponse, error) {
	// 优先走Redis热度榜单逻辑，缓存实例正常才执行
	// 因为rediscache需要判断指针是否为空
	if f.rediscache != nil {
		//`asOf` 得到的是**当前时刻所属的整分钟时间** as‑of time 快照时间
		// 所有分页共用同一个时间基准保证榜单不变
		asOf := time.Now().UTC().Truncate(time.Minute)
		if reqAsOf > 0 {
			asOf = time.Unix(reqAsOf, 0).UTC().Truncate(time.Minute)
		}
		// win=60 代表聚合最近60个分钟窗口，统计近1小时累计热度
		const win = 60
		keys := make([]string, 0, win)
		for i := 0; i < win; i++ {
			keys = append(keys, f.rediscache.Key(rediscache.HotVideoWindowFmt, asOf.Add(-time.Duration(i)*time.Minute).Format("200601021504")))
		}

		// dest ZUnionStore输出目标key：多份分钟快照合并后的综合热度ZSet
		dest := f.rediscache.Key(rediscache.HotVideoMergeFmt, asOf.Format("200601021504"))

		opCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
		defer cancel()
		// 这里不是原子性的， 有并发问题  zset 聚合对资源消耗很大
		exists, _ := f.rediscache.Exists(opCtx, dest)
		if !exists {
			_ = f.rediscache.ZUnionStore(opCtx, dest, keys, "SUM")
			_ = f.rediscache.Expire(opCtx, dest, 2*time.Minute)
		}
		// 当前分钟热榜在redis 中存在
		// 计算分页起止下标，ZRevRange从高热度到低热度倒序查询
		start := int64(offset)
		stop := start + int64(limit) - 1
		members, err := f.rediscache.ZRevRange(opCtx, dest, start, stop)
		if err == nil && len(members) == 0 {
			if offset > 0 {
				return ListByPopularityResponse{
					VideoList:  []FeedVideoItem{},
					AsOf:       asOf.Unix(),
					NextOffset: offset,
					HasMore:    false,
				}, nil
			}
		}

		if err == nil && len(members) > 0 {
			ids := make([]uint, 0, len(members))
			// 将ZSet中字符串格式的video id转为uint
			for _, m := range members {
				u, err := strconv.ParseUint(m, 10, 64)
				if err == nil && u > 0 {
					ids = append(ids, uint(u))
				}
			}
			// 批量从数据库查询视频完整实体信息
			videos, err := f.repo.GetByIDs(ctx, ids)
			if err == nil {
				byID := make(map[uint]*video.Video, len(videos))
				for _, v := range videos {
					byID[v.ID] = v
				}
				// 严格按照热度ZSet返回顺序重组列表，保证热度从高到低
				ordered := make([]*video.Video, 0, len(ids))
				for _, id := range ids {
					if v := byID[id]; v != nil {
						ordered = append(ordered, v)
					}
				}
				items, err := f.buildFeedVideos(ctx, ordered, viewerAccountID)
				if err != nil {
					return ListByPopularityResponse{}, err
				}
				// 组装分页返回体
				resp := ListByPopularityResponse{
					VideoList:  items,
					AsOf:       asOf.Unix(),         // 返回当前快照时间戳，前端翻页携带复用快照
					NextOffset: offset + len(items), // 下一页偏移量
					HasMore:    len(items) == limit, // 返回条数等于limit则判定还有下一页
				}
				// 记录最后一条视频的游标，用于Redis快照失效降级MySQL时使用复合游标分页
				if len(ordered) > 0 {
					last := ordered[len(ordered)-1]
					nextPopularity := last.Popularity
					nextBefore := last.CreateTime
					nextID := last.ID
					resp.NextLatestPopularity = &nextPopularity
					resp.NextLatestBefore = &nextBefore
					resp.NextLatestIDBefore = &nextID
				}
				return resp, nil
			}
		}
	}
	// 降级分支：Redis不可用 / 快照无数据，走MySQL复合游标分页
	videos, err := f.repo.ListByPopularity(ctx, limit, latestPopularity, latestBefore, latestIDBefore)
	if err != nil {
		return ListByPopularityResponse{}, err
	}
	items, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, err
	}
	resp := ListByPopularityResponse{
		VideoList:  items,
		AsOf:       0,                   // 显示无Redis快照，asOf置0标识走DB兜底
		NextOffset: 0,                   // DB分页不使用offset，改用复合游标
		HasMore:    len(items) == limit, // 返回满页则存在下一页
	}
	// 填充下一页复合游标（热度、创建时间、视频ID，解决相同热度分页重复/漏数据）
	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextPopularity := last.Popularity
		nextBefore := last.CreateTime
		nextID := last.ID
		resp.NextLatestPopularity = &nextPopularity
		resp.NextLatestBefore = &nextBefore
		resp.NextLatestIDBefore = &nextID
	}
	return resp, nil
}

// buildFeedVideos 将视频列表转换为 Feed 流专用的视频条目列表，并填充当前用户的点赞状态。
func (f *FeedService) buildFeedVideos(ctx context.Context, videos []*video.Video, viewerAccountID uint) ([]FeedVideoItem, error) {
	feedVideos := make([]FeedVideoItem, 0, len(videos))
	// 收集所有视频 ID，用于批量查询点赞状态
	videoIDs := make([]uint, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}

	// 批量获取当前用户对每个视频的点赞情况 查询的mysql
	likedMap, err := f.likeRepo.BatchGetLiked(ctx, videoIDs, viewerAccountID)
	if err != nil {
		return nil, err // 查询失败时直接返回错误，不继续处理
	}

	// 遍历每个视频，组装 FeedVideoItem，并填充点赞状态
	for _, video := range videos {
		feedVideos = append(feedVideos, FeedVideoItem{
			ID:          video.ID,
			Author:      FeedAuthor{ID: video.AuthorID, Username: video.Username},
			Title:       video.Title,
			Description: video.Description,
			PlayURL:     video.PlayURL,
			CoverURL:    video.CoverURL,
			CreateTime:  video.CreateTime.Unix(), // 将时间转换为 Unix 时间戳（秒）
			LikesCount:  video.LikesCount,
			IsLiked:     likedMap[video.ID], // 若不存在该 ID，则 Go 会返回 bool 的零值 false
		})
	}
	return feedVideos, nil
}

// buildOrderedResult 根据id顺序从map构建有序视频切片，过滤不存在/nil数据
func buildOrderedResult(orderedIDs []uint, dataMap map[uint]*video.Video) []*video.Video {
	res := make([]*video.Video, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if v, exists := dataMap[id]; exists && v != nil {
			res = append(res, v)
		}
	}
	return res
}

// ListByTag 根据标签名称查询标签下的视频列表，并组装前端需要的Feed视图数据
func (f *FeedService) ListByTag(ctx context.Context, tagName string, limit int, viewerAccountID uint) ([]FeedVideoItem, error) {
	videos, err := f.repo.ListByTag(ctx, tagName, limit)
	if err != nil {
		return nil, err
	}
	// 填充个性化字段（点赞、关注、是否自己作品等），转换为对外展示的Feed结构体
	return f.buildFeedVideos(ctx, videos, viewerAccountID)
}
