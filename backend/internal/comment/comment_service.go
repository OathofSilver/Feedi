package comment

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"feed/backend/internal/notification"
	"feed/backend/internal/popularitycache"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/video"
	"feed/backend/internal/worker/producer"
)

var (
	ErrInvalidParam    = errors.New("请求参数错误")
	ErrVideoNotFound   = errors.New("视频不存在")
	ErrCommentNotFound = errors.New("评论不存在")
	ErrNotAuthor       = errors.New("无权操作他人评论")
)

// CommentService 评论业务服务，负责评论发布/删除的防御校验、落库与 MQ 事件投递
type CommentService struct {
	repo         *CommentRepository      // 评论单表数据访问
	videoRepo    video.VideoRepositoryer // 视频数据访问，发布评论前校验视频存在性
	cache        *rediscache.Client      // Redis 客户端，MQ 不可用时降级直接更新热度值
	commentMQ    *producer.CommentMQ     // 评论事件生产者，异步通知/实时推送
	popularityMQ *producer.PopularityMQ  // 视频热度事件生产者，异步更新热度值
	notifier     *notification.Service   // 通知服务：评论后通知视频作者（可空）
}

func NewCommentService(repo *CommentRepository, videoRepo video.VideoRepositoryer, cache *rediscache.Client, commentMQ *producer.CommentMQ, popularityMQ *producer.PopularityMQ, notifier *notification.Service) *CommentService {
	return &CommentService{repo: repo, videoRepo: videoRepo, cache: cache, commentMQ: commentMQ, popularityMQ: popularityMQ, notifier: notifier}
}

// Publish 发布评论
// 流程：防御校验 → 校验视频存在 → 落库评论（允许重复评论，无需查重）→
// 投递评论事件与热度事件到 MQ；热度事件不可用时降级直接更新 Redis 热度值
func (s *CommentService) Publish(ctx context.Context, comment *Comment) error {
	// 防御性校验：对象为空、视频ID或作者ID为0、内容为空，直接返回参数错误
	if comment == nil || comment.VideoID == 0 || comment.AuthorID == 0 || comment.Content == "" {
		return ErrInvalidParam
	}

	// 校验视频是否存在，不存在则拒绝评论
	exists, err := s.videoRepo.IsExist(ctx, comment.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrVideoNotFound
	}

	// 允许重复评论，直接落库（直写 MySQL，评论数据本身不依赖 MQ，天然容灾）
	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return err
	}

	// 投递评论事件与热度事件到 MQ；热度事件失败时降级更新 Redis 热度值
	s.notify(ctx, "publish", comment, 1)
	// 通知视频作者"被评论"（作者评论自己的视频被过滤）
	s.notifyAuthorCommented(ctx, comment)
	return nil
}

// Delete 删除评论：校验评论存在与归属（只能删除自己的评论）后删除，并异步通知热度 -1
func (s *CommentService) Delete(ctx context.Context, commentID, accountID uint) error {
	// 防御性校验：评论ID或账号ID为0，直接返回参数错误
	if commentID == 0 || accountID == 0 {
		return ErrInvalidParam
	}

	// 查询评论，校验存在性与归属
	c, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrCommentNotFound
	}
	if c.AuthorID != accountID {
		return ErrNotAuthor
	}

	// 直写 MySQL 删除评论记录
	if err := s.repo.DeleteComment(ctx, &Comment{ID: commentID}); err != nil {
		return err
	}

	// 投递删除事件与热度事件（-1），热度事件失败时降级更新 Redis 热度值
	s.notify(ctx, "delete", c, -1)
	return nil
}

// GetAll 获取某视频的评论列表（显示接口），按评论时间正序
func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]Comment, error) {
	// 防御性校验：视频ID为0，直接返回参数错误
	if videoID == 0 {
		return nil, ErrInvalidParam
	}
	return s.repo.GetAllComments(ctx, videoID)
}

// notify 评论变更后的异步通知
// 正常路径：投递 CommentMQ 评论事件与 PopularityMQ 热度事件（消费者异步更新热度值）
// 降级路径：PopularityMQ 不可用或投递失败时，直接更新 Redis 热度值；
// 评论记录已直写 MySQL 落库，MQ 故障不影响评论数据本身
func (s *CommentService) notify(ctx context.Context, action string, comment *Comment, change int64) {
	// 评论事件投递失败仅记录日志，不触发热度降级，避免与热度事件降级重复累加
	if s.commentMQ != nil {
		var err error
		if action == "delete" {
			// 删除事件冗余携带 video_id，避免消费者反查已删除的评论行导致热度漏减
			err = s.commentMQ.Delete(ctx, comment.ID, comment.VideoID)
		} else {
			err = s.commentMQ.Publish(ctx, comment.Username, comment.VideoID, comment.AuthorID, comment.Content)
		}
		if err != nil {
			slog.Warn("投递评论事件失败", "action", action, "comment_id", comment.ID, "video_id", comment.VideoID, "err", err)
		}
	}

	// 热度事件投递成功则直接返回，由消费者异步更新 Redis 热度值
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, change); err == nil {
			return
		} else {
			slog.Warn("投递热度事件失败，降级直更 Redis 热度值", "video_id", comment.VideoID, "change", change, "err", err)
		}
	}

	// 降级容灾：直接更新 Redis 热度值（分钟热度窗口增量 + 失效视频详情缓存）
	if s.cache != nil {
		popularitycache.UpdatePopularityCache(ctx, s.cache, comment.VideoID, change)
	}
}

// notifyAuthorCommented 在评论落库成功后通知视频作者（尽力而为）：
// 视频不存在或评论者即作者时不通知。通知失败不影响评论主流程。
func (s *CommentService) notifyAuthorCommented(ctx context.Context, c *Comment) {
	if s.notifier == nil {
		return
	}
	v, err := s.videoRepo.FindByID(ctx, c.VideoID)
	if err != nil || v == nil {
		return
	}
	if v.AuthorID == 0 || v.AuthorID == c.AuthorID {
		return // 过滤自我触发
	}
	if err := s.notifier.Notify(ctx, v.AuthorID, c.AuthorID, notification.TypeComment, func(n *notification.Notification) {
		n.VideoID = c.VideoID
		n.Content = c.Content
	}); err != nil {
		slog.Warn("通知作者被评论失败", "author_id", v.AuthorID, "actor_id", c.AuthorID, "err", err)
	}
}

var mentionRegex = regexp.MustCompile(`@(\w+)`)

func (s *CommentService) notifyMentions(ctx context.Context, comment *Comment) {

}
