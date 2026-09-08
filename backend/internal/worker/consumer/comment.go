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
	// commentHandleTimeout 单条消息处理的超时时间，防止数据库故障时消息被长期占用
	commentHandleTimeout = 10 * time.Second
	// commentMsgProcessedKey 评论事件已消费去重 key 模板：msg:comment:processed:{messageID}
	// 模板统一引自 rediscache/keys.go
	commentMsgProcessedKey = rediscache.MsgCommentProcessedFmt
	// commentMsgProcessedTTL 消费标记保留时间，过期后允许重复消费（正常情况下不会被再次投递）
	commentMsgProcessedTTL = 24 * time.Hour
)

// CommentWorker 评论事件消费者。
// 职责边界：评论记录由 API 进程同步落库/删除（评论数据不依赖 MQ），本消费者只做
// 副作用（视频热度 DB 增量更新），幂等性依赖事件 MessageId 去重——
// 绝不在此重复插入/删除评论记录，否则评论会双写
type CommentWorker struct {
	ch     *amqp.Channel
	videos video.VideoRepositoryer // 视频仓库：热度增量更新
	cache  *rediscache.Client      // Redis：事件消费幂等去重
	queue  string
}

func NewCommentWorker(ch *amqp.Channel, videos video.VideoRepositoryer, cache *rediscache.Client, queue string) *CommentWorker {
	return &CommentWorker{ch: ch, videos: videos, cache: cache, queue: queue}
}

func (w *CommentWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.videos == nil || w.cache == nil {
		return errors.New("comment worker is not initialized")
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
func (w *CommentWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	// 单条消息设置超时，避免长时间阻塞导致消息堆积
	processCtx, cancel := context.WithTimeout(ctx, commentHandleTimeout)
	defer cancel()

	var evt producer.CommentEvent
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		// 解析失败视为毒消息，丢弃不重投，防止死循环
		slog.Error("解析评论事件失败", "message_id", d.MessageId, "error", err)
		_ = d.Nack(false, false)
		return
	}

	// 消息ID优先取 AMQP MessageId，其次取事件体 event_id（生产者保证两者一致，均为雪花算法生成）
	messageID := d.MessageId
	if messageID == "" {
		messageID = evt.EventID
	}

	// 幂等去重：SETNX 原子抢占消费权，仅抢占成功（首次消费）才继续处理
	// 并发重复投递时只有一个消费者能写入成功，其余直接确认跳过，杜绝热度被重复加减
	if messageID != "" {
		key := w.cache.Key(commentMsgProcessedKey, messageID)
		first, err := w.cache.SetNX(processCtx, key, "1", commentMsgProcessedTTL)
		if err != nil {
			// 无法确认消费权，重新入队等待下次重试
			slog.Warn("抢占评论消息消费权失败", "message_id", messageID, "error", err)
			_ = d.Nack(false, true)
			return
		}
		if !first {
			slog.Debug("评论消息已消费过，跳过处理", "message_id", messageID)
			_ = d.Ack(false)
			return
		}
	}

	if err := w.process(processCtx, &evt); err != nil {
		// 处理失败：先释放消费权（删除幂等键）再重新入队，避免消息被幂等标记永久跳过
		if messageID != "" {
			if delErr := w.cache.Del(processCtx, w.cache.Key(commentMsgProcessedKey, messageID)); delErr != nil {
				slog.Warn("释放评论消息消费权失败", "message_id", messageID, "error", delErr)
			}
		}
		slog.Error("处理评论事件失败", "message_id", messageID, "action", evt.Action,
			"comment_id", evt.CommentID, "video_id", evt.VideoID, "error", err)
		_ = d.Nack(false, true)
		return
	}

	_ = d.Ack(false)
}

func (w *CommentWorker) process(ctx context.Context, evt *producer.CommentEvent) error {
	if evt == nil {
		return nil
	}

	switch evt.Action {
	case "publish":
		return w.applyPublish(ctx, evt)
	case "delete":
		return w.applyDelete(ctx, evt)
	default:
		return nil
	}
}

// applyPublish 评论发布副作用：视频热度 +1（评论记录已由 API 进程同步落库）
func (w *CommentWorker) applyPublish(ctx context.Context, evt *producer.CommentEvent) error {
	if evt.VideoID == 0 || evt.AuthorID == 0 {
		return nil
	}

	// 校验视频是否存在，不存在直接返回不抛错
	ok, err := w.videos.IsExist(ctx, evt.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	// 视频热度值 +1（repo 层为增量写）
	return w.videos.UpdatePopularity(ctx, evt.VideoID, 1)
}

// applyDelete 评论删除副作用：视频热度 -1（评论记录已由 API 进程同步删除）
// 删除事件在投递时已冗余携带 video_id，直接据此做热度 -1，无需再反查——
// 反查已删除的评论行必然落空，会导致热度漏减
func (w *CommentWorker) applyDelete(ctx context.Context, evt *producer.CommentEvent) error {
	if evt.VideoID == 0 {
		return nil
	}

	// 校验视频是否存在，不存在直接返回不抛错
	ok, err := w.videos.IsExist(ctx, evt.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	// 视频热度值 -1（repo 层为增量写，GREATEST 兜底不为负），随后失效详情/实体缓存
	if err := w.videos.UpdatePopularity(ctx, evt.VideoID, -1); err != nil {
		return err
	}
	w.cache.InvalidateVideoCache(ctx, evt.VideoID)
	return nil
}
