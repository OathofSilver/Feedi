package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"feed/backend/internal/utils/rabbitmq"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/worker/producer"

	amqp "github.com/rabbitmq/amqp091-go"
	oredis "github.com/redis/go-redis/v9"
)

const (
	// globalTimelineKeyFmt 全局推荐时间线 ZSet key（经 cache.Key 统一加前缀）
	// member 为视频ID字符串，score 为视频创建时间的毫秒时间戳，
	// 与 feed 模块 ListLatest 冷热分离机制共用同一份热区数据，写入规则必须保持一致
	// 模板统一引自 rediscache/keys.go
	globalTimelineKeyFmt = rediscache.FeedGlobalTimelineKey
	// globalTimelineMaxLen 全局推荐时间线保留的最大视频条数，超出部分删除最旧视频降级为冷数据
	globalTimelineMaxLen = 1000
	// timelinePrefetchCount 预取消息数，限制未确认消息堆积，防止内存溢出
	timelinePrefetchCount = 10
)

type GlobalTimelineConsumer struct {
	ch    *amqp.Channel
	cache *rediscache.Client
}

// NewGlobalTimelineConsumer 创建全局时间线消费者，声明交换机/队列/绑定（幂等操作）
func NewGlobalTimelineConsumer(base *rabbitmq.RabbitMQ, cache *rediscache.Client) (*GlobalTimelineConsumer, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	if cache == nil {
		return nil, errors.New("redis cache is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	// 声明交换机、队列与绑定，保证消费者先于生产者启动时拓扑已就绪
	if err := rabbitmq.DeclareTopic(ch, producer.TimelineExchange, producer.TimelineQueue, producer.TimelineBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	// 设置预取数量，实现按消费者负载分配
	if err := ch.Qos(timelinePrefetchCount, 0, false); err != nil {
		ch.Close()
		return nil, err
	}
	return &GlobalTimelineConsumer{ch: ch, cache: cache}, nil
}

// Run 启动消费循环，阻塞运行直到 ctx 取消
func (c *GlobalTimelineConsumer) Run(ctx context.Context) error {
	if c == nil || c.ch == nil || c.cache == nil {
		return errors.New("timeline consumer is not initialized")
	}
	// autoAck 为 false，由消费者显式确认，保证至少一次投递
	deliveries, err := c.ch.Consume(producer.TimelineQueue, "timeline-global-consumer", false, false, false, false, nil)
	if err != nil {
		return err
	}
	log.Println("timeline consumer started, queue:", producer.TimelineQueue)
	for {
		select {
		case <-ctx.Done():
			log.Println("timeline consumer stopped")
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			c.handleDelivery(ctx, d)
		}
	}
}

// Close 关闭消费者持有的 channel
func (c *GlobalTimelineConsumer) Close() error {
	if c == nil || c.ch == nil {
		return nil
	}
	return c.ch.Close()
}

// handleDelivery 处理单条投递消息，失败时指数退避重试，超过上限后丢弃避免消息堆积
func (c *GlobalTimelineConsumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			_ = d.Nack(false, true)
			return
		default:
		}
		if err := c.process(ctx, d.Body); err != nil {
			if i >= maxRetries {
				log.Printf("timeline consumer: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)
				_ = d.Ack(false)
				return
			}
			wait := time.Duration(1<<uint(i)) * time.Second
			log.Printf("timeline consumer: 处理失败, %v 后重试 (%d/%d): %v", wait, i+1, maxRetries, err)
			time.Sleep(wait)
			continue
		}
		_ = d.Ack(false)
		return
	}
}

// process 解析视频发布事件，写入全局推荐时间线 ZSet 并裁剪至 1000 条以内。
// 只返回处理结果错误，消息确认（Ack/Nack）统一由 handleDelivery 按结果执行
func (c *GlobalTimelineConsumer) process(ctx context.Context, body []byte) error {
	var evt producer.TimelineEvent
	// JSON 解析失败视为毒消息，直接丢弃不重试
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil
	}
	if evt.VideoID == 0 {
		return nil
	}
	// 事件时间缺失时使用事件发生时间兜底，避免分数异常打乱时间线排序
	if evt.CreateTime <= 0 {
		if evt.OccurredAt.IsZero() {
			return nil
		}
		evt.CreateTime = evt.OccurredAt.UnixMilli()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	timelineKey := c.cache.Key(rediscache.FeedGlobalTimelineKey)
	err := c.cache.ZAdd(ctx, timelineKey, oredis.Z{
		Score:  float64(evt.CreateTime),
		Member: fmt.Sprintf("%d", evt.VideoID),
	})
	if err != nil {
		log.Printf("Timeline consumer: 写入全局时间线 ZSet 失败: %v", err)
		return err
	}
	// 裁剪有序集合：只保留最新1000条，删除排名0 ~ -1001（淘汰最旧数据）
	if err := c.cache.ZRemRangeByRank(ctx, timelineKey, 0, -1001); err != nil {
		log.Printf("Timeline consumer: ZRem失败: %v", err)
	}
	return nil
}
