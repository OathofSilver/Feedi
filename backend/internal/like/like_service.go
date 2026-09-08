package like

import (
	"context"
	"errors"
	"log/slog"

	"feed/backend/internal/notification"
	"feed/backend/internal/popularitycache"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/video"
	"feed/backend/internal/worker/producer"
)

// 业务哨兵错误，Handler 层据此做统一错误映射
var (
	ErrInvalidParam  = errors.New("请求参数错误")
	ErrVideoNotFound = errors.New("视频不存在")
	ErrAlreadyLiked  = errors.New("已点赞过该视频")
	ErrNotLiked      = errors.New("尚未点赞该视频")
)

// LikeService 点赞业务服务，负责点赞/取消点赞的防御校验、落库与 MQ 事件投递
type LikeService struct {
	repo         LikeRepositoryer        // 点赞记录单表数据访问
	videoRepo    video.VideoRepositoryer // 视频数据访问，点赞前校验视频存在性
	likeMQ       *producer.LikeMQ        // 点赞事件生产者，驱动粉丝时间线等异步写扩散
	popularityMQ *producer.PopularityMQ  // 视频热度事件生产者，异步更新热度值
	cache        *rediscache.Client      // Redis 客户端，MQ 不可用时降级直接更新热度值
	notifier     *notification.Service   // 通知服务：点赞后通知视频作者（可空，不影响点赞主流程）
}

// NewLikeService 创建点赞业务服务
func NewLikeService(repo LikeRepositoryer, videoRepo video.VideoRepositoryer, likeMQ *producer.LikeMQ, popularityMQ *producer.PopularityMQ, cache *rediscache.Client, notifier *notification.Service) *LikeService {
	return &LikeService{
		repo:         repo,
		videoRepo:    videoRepo,
		likeMQ:       likeMQ,
		popularityMQ: popularityMQ,
		cache:        cache,
		notifier:     notifier,
	}
}

// Like 点赞视频
// 流程：防御校验 → 校验视频存在 → 校验未重复点赞 → 落库点赞记录 →
// 投递点赞事件与热度事件到 MQ；MQ 不可用时降级直写 MySQL 点赞数并更新 Redis 热度值
func (s *LikeService) Like(ctx context.Context, like *Like) error {
	// 防御性校验：对象为空、视频ID或账号ID为0，直接返回参数错误
	if like == nil || like.VideoID == 0 || like.AccountID == 0 {
		return ErrInvalidParam
	}

	// 校验视频是否存在，不存在则拒绝点赞
	exists, err := s.videoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrVideoNotFound
	}

	// 判断是否已点赞过，重复点赞直接返回业务错误
	liked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if liked {
		return ErrAlreadyLiked
	}

	// 插入点赞记录，唯一索引兜底并发下的重复点赞（未插入视为已点赞）
	created, err := s.repo.LikeIgnoreDuplicate(ctx, like)
	if err != nil {
		return err
	}
	if !created {
		return ErrAlreadyLiked
	}
	// notify 点赞状态变更后的异步通知
	s.notify(ctx, like.VideoID, like.AccountID, true)
	// 通知视频作者"被点赞"（自我点赞被过滤）
	s.notifyActorLike(ctx, like.VideoID, like.AccountID)
	return nil
}

// Unlike 取消点赞
// 流程：防御校验 → 校验视频存在 → 校验已点赞 → 删除点赞记录 → 投递取消事件并降级兜底
func (s *LikeService) Unlike(ctx context.Context, like *Like) error {
	// 防御性校验：对象为空、视频ID或账号ID为0，直接返回参数错误
	if like == nil || like.VideoID == 0 || like.AccountID == 0 {
		return ErrInvalidParam
	}

	// 校验视频是否存在，不存在则拒绝取消点赞
	exists, err := s.videoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrVideoNotFound
	}

	// 判断是否已点赞过，未点赞则取消操作无意义
	liked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !liked {
		return ErrNotLiked
	}

	// 物理删除点赞记录，删除行数为0视为未点赞（并发下已被其他请求取消）
	deleted, err := s.repo.DeleteByVideoAndAccount(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotLiked
	}

	s.notify(ctx, like.VideoID, like.AccountID, false)
	return nil
}

// IsLiked 判断用户是否已点赞该视频
func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	if videoID == 0 || accountID == 0 {
		return false, ErrInvalidParam
	}
	return s.repo.IsLiked(ctx, videoID, accountID)
}

// ListLikedVideos 查询用户点赞过的视频列表，按点赞时间倒序，最多返回200条
func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]video.Video, error) {
	if accountID == 0 {
		return nil, ErrInvalidParam
	}
	return s.repo.ListLikedVideos(ctx, accountID)
}

// notify 点赞状态变更后的异步通知
// 正常路径：投递 LikeMQ 点赞事件（粉丝时间线等写扩散）与 PopularityMQ 热度事件（异步更新热度值）
// 降级路径：PopularityMQ 不可用或投递失败时，直写 MySQL 视频点赞数并直接更新 Redis 热度值，保证计数与热度最终一致
func (s *LikeService) notify(ctx context.Context, videoID, accountID uint, liked bool) {
	change := int64(1)
	if !liked {
		change = -1
	}

	// 点赞事件投递失败仅记录日志，不触发计数降级，避免与热度事件降级重复累加
	if s.likeMQ != nil {
		var err error
		if liked {
			err = s.likeMQ.Like(ctx, accountID, videoID)
		} else {
			err = s.likeMQ.Unlike(ctx, accountID, videoID)
		}
		if err != nil {
			slog.Warn("投递点赞事件失败", "video_id", videoID, "account_id", accountID, "err", err)
		}
	}

	// 热度事件投递成功则直接返回，由消费者异步更新 Redis 热度值
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, videoID, change); err == nil {
			return
		} else {
			slog.Warn("投递热度事件失败，降级直写 MySQL 并更新 Redis", "video_id", videoID, "change", change, "err", err)
		}
	}

	// 降级容灾：直写 更新MySQL 视频点赞数字段，并直接更新 Redis 热度值
	if err := s.videoRepo.UpdateLikesCount(ctx, videoID, change); err != nil {
		slog.Error("降级更新视频点赞数失败", "video_id", videoID, "change", change, "err", err)
	}

	if s.cache != nil {
		popularitycache.UpdatePopularityCache(ctx, s.cache, videoID, change)
	}
}

// notifyActorLike 在点赞成功后通知视频作者（尽力而为）：作者不存在或作者即点赞者时不通知。
// 通知失败不影响点赞主流程（点赞记录/计数均已落库）。
func (s *LikeService) notifyActorLike(ctx context.Context, videoID, actorID uint) {
	if s.notifier == nil {
		return
	}
	v, err := s.videoRepo.FindByID(ctx, videoID)
	if err != nil || v == nil {
		return
	}
	if v.AuthorID == 0 || v.AuthorID == actorID {
		return // 过滤自我触发
	}
	if err := s.notifier.Notify(ctx, v.AuthorID, actorID, notification.TypeLike, func(n *notification.Notification) {
		n.VideoID = videoID
	}); err != nil {
		slog.Warn("通知作者被点赞失败", "author_id", v.AuthorID, "actor_id", actorID, "err", err)
	}
}
