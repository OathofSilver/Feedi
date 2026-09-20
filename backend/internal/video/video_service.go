package video

import (
	"context"
	"encoding/json"
	"errors"
	"feed/backend/internal/apierror"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/worker/producer"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

// 视频详情 "video:detail:id=%d   id

type VideoService struct {
	db           *gorm.DB
	repo         VideoRepositoryer
	cache        *rediscache.Client
	cacheTTL     time.Duration
	popularityMQ *producer.PopularityMQ
}

func NewVideoService(db *gorm.DB, repo VideoRepositoryer, cache *rediscache.Client, cacheTTL time.Duration, popularityMQ *producer.PopularityMQ) *VideoService {
	return &VideoService{
		db:           db,
		repo:         repo,
		cache:        cache,
		cacheTTL:     cacheTTL,
		popularityMQ: popularityMQ,
	}
}

func (vs *VideoService) Publish(ctx context.Context, video *Video) error {
	if video == nil {
		return errors.New("video is nil")
	}
	video.Title = strings.TrimSpace(video.Title)
	video.PlayURL = strings.TrimSpace(video.PlayURL)
	video.CoverURL = strings.TrimSpace(video.CoverURL)

	if video.Title == "" {
		return errors.New("title is required")
	}
	if video.PlayURL == "" {
		return errors.New("play url is required")
	}
	if video.CoverURL == "" {
		return errors.New("cover url is required")
	}
	err := vs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := vs.repo.WithTx(tx).Create(ctx, video); err != nil {
			return err
		}
		msg := OutboxMsg{
			VideoID:    video.ID,
			EventType:  "video_published", // 事件：视频发布
			Status:     "pending",         // 待消费（待由轮询器投递到RabbitMQ）
			CreateTime: video.CreateTime,
		}
		if err := tx.Create(&msg).Error; err != nil {
			return err
		}
		// 从标题+描述中提取文本标签
		tags := ExtractTags(video.Title + " " + video.Description)
		for _, tagName := range tags {
			var tag Tag
			// FirstOrCreate：标签不存在则新建，存在直接查询，复用已有标签记录
			if err := tx.Where("name = ?", tagName).FirstOrCreate(&tag, Tag{Name: tagName}).Error; err != nil {
				return err
			}
			// 建立视频和标签的多对多关联关系
			if err := tx.Create(&VideoTag{VideoID: video.ID, TagID: tag.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (vs *VideoService) Delete(ctx context.Context, id uint, authorID uint) error {
	video, err := vs.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if video == nil {
		return errors.New("video not found")
	}
	if video.AuthorID != authorID {
		return apierror.ErrUnauthorized
	}
	if err := vs.repo.Delete(ctx, id); err != nil {
		return err
	}
	// 删除 key：详情与 feed 实体是同一实体的两份缓存拷贝，必须一并失效
	if vs.cache != nil {
		vs.cache.InvalidateVideoCache(context.Background(), id)
	}
	return nil
}

// GetDetail 根据视频ID获取视频详情，实现L2 Redis缓存 + 分布式锁防缓存击穿逻辑
func (vs *VideoService) GetDetail(ctx context.Context, id uint) (*Video, error) {
	// 缓存 key 提前到函数顶部统一定义：下方 getCached/setCached 闭包都依赖它，
	// 定义在使用之后会导致编译错误；cache.Key 对 nil 客户端安全（返回无前缀 key）
	cacheKey := vs.cache.VideoDetailKey(id)

	// 读缓存
	getCached := func() (*Video, bool) {
		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel() // 函数退出释放上下文，避免goroutine泄漏

		b, err := vs.cache.GetBytes(opCtx, cacheKey)
		if err != nil {
			return nil, false
		}
		var cached Video
		if err := json.Unmarshal(b, &cached); err != nil {
			return nil, false
		}
		return &cached, true
	}

	// 写缓存
	setCached := func(video *Video) {

		b, err := json.Marshal(video)
		if err != nil {

			return
		}
		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		_ = vs.cache.SetBytes(opCtx, cacheKey, b, vs.cacheTTL)
	}

	if vs.cache != nil {
		if v, ok := getCached(); ok {
			return v, nil
		}
		// 二次兜底读取缓存（冗余逻辑，正常第一次getCached已覆盖）
		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		b, err := vs.cache.GetBytes(opCtx, cacheKey)
		cancel()
		if err == nil {
			var cached Video
			if err := json.Unmarshal(b, &cached); err == nil {
				return &cached, nil
			}
		} else if rediscache.IsMiss(err) { // 进入分布式锁防击穿流程
			lockKey := rediscache.LockKeyFor(cacheKey)
			lockCtx, lockCancel := context.WithTimeout(ctx, 50*time.Millisecond)
			token, locked, lockErr := vs.cache.Lock(lockCtx, lockKey, 2*time.Second)
			lockCancel()
			// 加锁无异常且成功抢到锁，当前goroutine负责回源DB并回填缓存
			if lockErr == nil && locked {
				defer func() { _ = vs.cache.Unlock(context.Background(), lockKey, token) }()
				if v, ok := getCached(); ok {
					return v, nil
				}
				video, err := vs.repo.FindByID(ctx, id)
				if err != nil {
					return nil, err
				}
				setCached(video)
				return video, nil
			}

			// 未抢到分布式锁，说明其他协程正在回源DB写缓存，循环轮询等待缓存生成
			// 最多轮询5次，每次间隔20ms，总等待时长100ms，避免无限阻塞
			for i := 0; i < 5; i++ {
				select {
				case <-ctx.Done(): // 上游请求取消/超时，直接返回上下文错误
					return nil, ctx.Err()
				case <-time.After(20 * time.Millisecond): // 休眠20ms后重试读取缓存
				}
				// 轮询期间缓存生成成功，直接返回缓存数据
				if v, ok := getCached(); ok {
					return v, nil
				}
			}
		}
	}

	// 兜底设计
	video, err := vs.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 数据库查询成功，同步写入缓存供下次请求使用
	if vs.cache != nil {
		setCached(video)
	}
	return video, nil
}

func (vs *VideoService) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	if err := vs.repo.UpdateLikesCount(ctx, id, likesCount); err != nil {
		return err
	}
	return nil
}

func (vs *VideoService) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	if err := vs.repo.UpdatePopularity(ctx, id, change); err != nil {
		return err
	}

	if vs.popularityMQ != nil {
		if err := vs.popularityMQ.Update(ctx, id, change); err == nil {
			return nil
		}
	}

	if vs.cache != nil {
		// 1) 详情/实体缓存：直接失效（最简单靠谱）
		vs.cache.InvalidateVideoCache(context.Background(), id)

		// 2) 热榜：写到“时间窗ZSET”，不要用 detail key
		now := time.Now().UTC().Truncate(time.Minute)
		windowKey := vs.cache.Key(rediscache.HotVideoWindowFmt, now.Format("200601021504"))
		member := strconv.FormatUint(uint64(id), 10)

		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		_ = vs.cache.ZincrBy(opCtx, windowKey, member, float64(change))
		_ = vs.cache.Expire(opCtx, windowKey, 2*time.Hour)
	}
	return nil
}
