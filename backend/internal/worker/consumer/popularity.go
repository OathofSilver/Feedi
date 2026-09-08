package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"feed/backend/internal/popularitycache"
	"feed/backend/internal/worker/producer"
	"log"
	"time"

	rediscache "feed/backend/internal/utils/redis"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PopularityWorker struct {
	ch    *amqp.Channel
	cache *rediscache.Client
	queue string
}

func NewPopularityWorker(ch *amqp.Channel, cache *rediscache.Client, queue string) *PopularityWorker {
	return &PopularityWorker{ch: ch, cache: cache, queue: queue}
}

func (w *PopularityWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.cache == nil {
		return errors.New("popularity worker is not initialized")
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

// handleDelivery 处理RabbitMQ投递过来的热度事件消息，自带本地指数退避重试逻辑
func (w *PopularityWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			_ = d.Nack(false, true)
			return
		default:
			// 上下文正常，继续执行业务
		}
		// 执行消息业务处理逻辑
		if err := w.process(ctx, d.Body); err != nil {
			if i >= maxRetries {
				log.Printf("popularity worker: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)
				_ = d.Ack(false)
				return
			}
			wait := time.Duration(1<<uint(i)) * time.Second
			log.Printf("popularity worker: 处理失败, %v 后重试 (%d/%d): %v", wait, i+1, maxRetries, err)
			time.Sleep(wait)
			continue
		}
		// 处理成功，手动Ack确认消息，MQ会删除该消息
		_ = d.Ack(false)
		return
	}
}

func (w *PopularityWorker) process(ctx context.Context, body []byte) error {
	var evt producer.PopularityEvent
	// 反序列化消息体为热度事件结构体
	if err := json.Unmarshal(body, &evt); err != nil {
		// JSON解析失败，消息格式非法，直接丢弃，返回nil不重试
		return nil
	}
	// 参数校验：视频ID为0或者热度变化量为0，无实际更新意义，直接丢弃消息
	if evt.VideoID == 0 || evt.Change == 0 {
		return nil
	}

	// 更新Redis中视频的热度缓存，Change为增量，可以是正数(点赞)或负数(取消点赞)
	popularitycache.UpdatePopularityCache(ctx, w.cache, evt.VideoID, evt.Change)
	return nil
}
