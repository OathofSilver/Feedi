package producer

import (
	"context"
	"errors"
	"feed/backend/internal/utils/rabbitmq"
	"feed/backend/internal/utils/snowflake"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NotificationMQ 通知事件生产者：通知落库成功后投递"就绪"信号，驱动 SSE 实时推送。
// 消费者由 api 进程持有（ssehub.Broadcaster），worker 不参与通知推送。
type NotificationMQ struct {
	ch *amqp.Channel
	sf *snowflake.Node
}

const (
	notificationExchange   = "notification.events"
	notificationQueue      = "notification.events"
	notificationBindingKey = "notification.*"

	notificationPushRK = "notification.push"
)

// NotificationEvent 通知事件消息体
type NotificationEvent struct {
	EventID        string    `json:"event_id"`        // 消息唯一ID，消费者据此幂等去重
	NotificationID uint      `json:"notification_id"` // 已落库的通知主键ID
	RecipientID    uint      `json:"recipient_id"`    // 收件人
	OccurredAt     time.Time `json:"occurred_at"`
}

// NotificationExchangeName 通知交换机名（供消费者跨包声明拓扑）
func NotificationExchangeName() string { return notificationExchange }

// NotificationQueueName 通知队列名（供消费者跨包消费）
func NotificationQueueName() string { return notificationQueue }

// NotificationBindingKey 通知队列绑定键（供消费者跨包声明拓扑）
func NotificationBindingKey() string { return notificationBindingKey }

// NewNotificationMQ 创建通知事件生产者；workerID 为雪花节点ID，须与 api main 其它节点错开。
func NewNotificationMQ(base *rabbitmq.RabbitMQ, workerID int64) (*NotificationMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := rabbitmq.DeclareTopic(ch, notificationExchange, notificationQueue, notificationBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	sfNode, err := snowflake.NewNode(workerID)
	if err != nil {
		ch.Close()
		return nil, err
	}
	return &NotificationMQ{ch: ch, sf: sfNode}, nil
}

// Push 投递通知就绪事件
func (n *NotificationMQ) Push(ctx context.Context, notificationID, recipientID uint) error {
	if n == nil || n.ch == nil {
		return errors.New("notification mq is not initialized")
	}
	if notificationID == 0 || recipientID == 0 {
		return errors.New("notificationID and recipientID are required")
	}
	messageID, err := n.sf.NextIDStr()
	if err != nil {
		return err
	}
	evt := NotificationEvent{
		EventID:        messageID,
		NotificationID: notificationID,
		RecipientID:    recipientID,
		OccurredAt:     time.Now(),
	}
	return rabbitmq.PublishJSON(ctx, n.ch, notificationExchange, notificationPushRK, messageID, evt)
}

// Close 关闭 channel
func (n *NotificationMQ) Close() error {
	if n == nil || n.ch == nil {
		return nil
	}
	return n.ch.Close()
}
