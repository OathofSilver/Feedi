package ssehub

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"feed/backend/internal/utils/rabbitmq"
	rediscache "feed/backend/internal/utils/redis"
	"feed/backend/internal/worker/producer"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// notifyMsgProcessedTTL 推送事件消费标记保留时长；过期后允许重复消费
	notifyMsgProcessedTTL = 24 * time.Hour
	// notifyPrefetch 预取数
	notifyPrefetch = 10
)

// Broadcaster 订阅通知事件队列并实时推送：它是 notification.events 的消费者，
// 但运行在 api 进程（SSE 长连接只存在于 api），worker 不消费该队列。
type Broadcaster struct {
	hub   *Hub
	ch    *amqp.Channel
	cache *rediscache.Client
}

// NewBroadcaster 创建并返回广播器；调用方负责在进程退出时 Close。
func NewBroadcaster(hub *Hub, ch *amqp.Channel, cache *rediscache.Client) (*Broadcaster, error) {
	if hub == nil || ch == nil || cache == nil {
		return nil, errors.New("hub/channel/cache is nil")
	}
	// 幂等声明拓扑：即便生产者晚于消费者初始化，队列也已就绪
	if err := rabbitmq.DeclareTopic(ch, producer.NotificationExchangeName(), producer.NotificationQueueName(), producer.NotificationBindingKey()); err != nil {
		return nil, err
	}
	if err := ch.Qos(notifyPrefetch, 0, false); err != nil {
		return nil, err
	}
	return &Broadcaster{hub: hub, ch: ch, cache: cache}, nil
}

// Run 阻塞消费直到 ctx 取消
func (b *Broadcaster) Run(ctx context.Context) error {
	deliveries, err := b.ch.Consume(producer.NotificationQueueName(), "notification-broadcaster", false, false, false, false, nil)
	if err != nil {
		return err
	}
	log.Println("notification broadcaster started, queue:", producer.NotificationQueueName())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			b.handle(ctx, d)
		}
	}
}

// Close 释放 channel
func (b *Broadcaster) Close() error {
	if b == nil || b.ch == nil {
		return nil
	}
	return b.ch.Close()
}

func (b *Broadcaster) handle(ctx context.Context, d amqp.Delivery) {
	var evt producer.NotificationEvent
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		_ = d.Nack(false, false) // 毒消息丢弃
		return
	}

	// 幂等：同一推送事件只处理一次
	msgID := d.MessageId
	if msgID == "" {
		msgID = evt.EventID
	}
	if msgID != "" {
		first, err := b.cache.SetNX(ctx, b.cache.Key(rediscache.MsgNotificationProcessedFmt, msgID), "1", notifyMsgProcessedTTL)
		if err != nil {
			_ = d.Nack(false, true)
			return
		}
		if !first {
			_ = d.Ack(false)
			return
		}
	}

	payload := []byte(evtJSON(evt))
	b.hub.Push(evt.RecipientID, payload)
	_ = d.Ack(false)
}

// evtJSON 序列化为 SSE data 负载；序列化失败时退回最小字段
func evtJSON(evt producer.NotificationEvent) string {
	type slim struct {
		EventID        string `json:"event_id"`
		NotificationID uint   `json:"notification_id"`
		RecipientID    uint   `json:"recipient_id"`
	}
	b, err := json.Marshal(slim{EventID: evt.EventID, NotificationID: evt.NotificationID, RecipientID: evt.RecipientID})
	if err != nil {
		return "{}"
	}
	return string(b)
}
