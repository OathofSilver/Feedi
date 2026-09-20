package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/video"
	"feed/backend/internal/worker/producer"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// likeHandleTimeout 单条消息处理的超时时间，防止数据库故障时消息被长期占用
	likeHandleTimeout = 10 * time.Second
	// likeMsgProcessedKey 点赞事件已消费去重 key 模板：msg:like:processed:{messageID}
	// 模板统一引自 rediscache/keys.go
	likeMsgProcessedKey = rediscache.MsgLikeProcessedFmt
	// likeMsgProcessedTTL 消费标记保留时间，过期后允许重复消费（正常情况下不会被再次投递）
	likeMsgProcessedTTL = 24 * time.Hour
)

// LikeWorker 点赞事件消费者。
// 职责边界：点赞记录由 API 进程同步落库（唯一索引保证一条），本消费者只负责
// 异步更新视频点赞计数与热度值，幂等性依赖事件 MessageId 去重而非记录插入结果，
// 否则 API 已插入记录会导致本消费者永远跳过计数更新（计数死链）
type LikeWorker struct {
	ch     *amqp.Channel
	videos video.VideoRepositoryer // 视频仓库：校验存在性 + 更新点赞数/热度值
	cache  *rediscache.Client      // Redis：事件消费幂等去重
	queue  string
}

func NewLikeWorker(ch *amqp.Channel, videos video.VideoRepositoryer, cache *rediscache.Client, queue string) *LikeWorker {
	return &LikeWorker{ch: ch, videos: videos, cache: cache, queue: queue}
}

func (w *LikeWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.videos == nil || w.cache == nil {
		return errors.New("like worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}

	deliveries, err := w.ch.Consume(
		w.queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			w.handleDelivery(ctx, d)
		}
	}
}

// handleDelivery 处理单条投递消息：幂等去重、解析、处理并按结果确认或重投
func (w *LikeWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	// 单条消息设置超时，避免长时间阻塞导致消息堆积
	processCtx, cancel := context.WithTimeout(ctx, likeHandleTimeout)
	defer cancel()

	var evt producer.LikeEvent
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		// 解析失败视为毒消息，丢弃不重投，防止死循环
		slog.Error("解析点赞事件失败", "message_id", d.MessageId, "error", err)
		_ = d.Nack(false, false)
		return
	}

	// 消息ID优先取 AMQP MessageId，其次取事件体 event_id（生产者保证两者一致，均为雪花算法生成）
	messageID := d.MessageId
	if messageID == "" {
		messageID = evt.EventID
	}

	// 幂等去重：SETNX 原子抢占消费权，仅抢占成功（首次消费）才继续处理
	// 并发重复投递时只有一个消费者能写入成功，其余直接确认跳过，杜绝计数被重复加减
	if messageID != "" {
		key := w.cache.Key(likeMsgProcessedKey, messageID)
		first, err := w.cache.SetNX(processCtx, key, "1", likeMsgProcessedTTL)
		if err != nil {
			// 无法确认消费权，重新入队等待下次重试
			slog.Warn("抢占点赞消息消费权失败", "message_id", messageID, "error", err)
			_ = d.Nack(false, true)
			return
		}
		if !first {
			slog.Debug("点赞消息已消费过，跳过处理", "message_id", messageID)
			_ = d.Ack(false)
			return
		}
	}

	if err := w.process(processCtx, &evt); err != nil {
		// 处理失败：先释放消费权（删除幂等键）再重新入队，避免消息被幂等标记永久跳过
		if messageID != "" {
			if delErr := w.cache.Del(processCtx, w.cache.Key(likeMsgProcessedKey, messageID)); delErr != nil {
				slog.Warn("释放点赞消息消费权失败", "message_id", messageID, "error", delErr)
			}
		}
		slog.Error("处理点赞事件失败", "message_id", messageID, "action", evt.Action,
			"user_id", evt.UserID, "video_id", evt.VideoID, "error", err)
		_ = d.Nack(false, true)
		return
	}

	_ = d.Ack(false)
}

func (w *LikeWorker) process(ctx context.Context, evt *producer.LikeEvent) error {
	// 参数校验：用户ID或视频ID为0，无实际意义，直接丢弃
	if evt == nil || evt.UserID == 0 || evt.VideoID == 0 {
		return nil
	}

	switch evt.Action {
	case "like":
		return w.applyLike(ctx, evt.UserID, evt.VideoID)
	case "unlike":
		return w.applyUnlike(ctx, evt.UserID, evt.VideoID)
	default:
		return nil
	}
}

// applyLike 点赞计数 +1：点赞记录已由 API 进程同步落库，这里只更新计数
func (w *LikeWorker) applyLike(ctx context.Context, userID, videoID uint) error {
	// 校验视频是否存在，不存在直接返回不抛错（事件可能晚于视频删除到达）
	ok, err := w.videos.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	// 视频点赞数 +1（repo 层为增量写，GREATEST 兜底不为负）
	if err := w.videos.UpdateLikesCount(ctx, videoID, 1); err != nil {
		return err
	}
	// 视频热度值 +1
	return w.videos.UpdatePopularity(ctx, videoID, 1)
}

// applyUnlike 取消点赞计数 -1：点赞记录已由 API 进程同步删除，这里只回退计数
func (w *LikeWorker) applyUnlike(ctx context.Context, userID, videoID uint) error {
	// 校验视频是否存在，不存在直接返回不抛错
	ok, err := w.videos.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	// 视频点赞数 -1
	if err := w.videos.UpdateLikesCount(ctx, videoID, -1); err != nil {
		return err
	}
	// 视频热度值 -1
	if err := w.videos.UpdatePopularity(ctx, videoID, -1); err != nil {
		return err
	}
	// 同上：失效详情/实体缓存
	w.cache.InvalidateVideoCache(ctx, videoID)
	return nil
}
