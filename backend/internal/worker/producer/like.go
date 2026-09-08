package producer

import (
	"context"
	"errors"
	"feed/backend/internal/utils/rabbitmq"
	"feed/backend/internal/utils/snowflake"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

type LikeMQ struct {
	ch *amqp.Channel
	sf *snowflake.Node
}

const (
	likeExchange   = "like.events"
	likeQueue      = "like.events"
	likeBindingKey = "like.*"

	likeLikeRK   = "like.like"
	likeUnlikeRK = "like.unlike"
)

type LikeEvent struct {
	EventID    string    `json:"event_id"` // 事件ID
	Action     string    `json:"action"`
	UserID     uint      `json:"user_id"`
	VideoID    uint      `json:"video_id"`
	OccurredAt time.Time `json:"occurred_at"` // 发布时间
}

// NewLikeMQ 创建点赞事件生产者；workerID 为雪花算法节点ID，多实例部署时各节点须唯一
func NewLikeMQ(base *rabbitmq.RabbitMQ, workerID int64) (*LikeMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := rabbitmq.DeclareTopic(ch, likeExchange, likeQueue, likeBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	// 初始化雪花算法发号器节点，生成消息唯一ID
	sfNode, err := snowflake.NewNode(workerID)
	if err != nil {
		ch.Close()
		return nil, err
	}
	return &LikeMQ{ch: ch, sf: sfNode}, nil
}

func (l *LikeMQ) Like(ctx context.Context, userID, videoID uint) error {
	return l.publish(ctx, "like", likeLikeRK, userID, videoID)
}

func (l *LikeMQ) Unlike(ctx context.Context, userID, videoID uint) error {
	return l.publish(ctx, "unlike", likeUnlikeRK, userID, videoID)
}

func (l *LikeMQ) publish(ctx context.Context, action, routingKey string, userID, videoID uint) error {
	if l == nil || l.ch == nil {
		return errors.New("like mq is not initialized")
	}
	if userID == 0 || videoID == 0 {
		return errors.New("userID and videoID are required")
	}
	messageID, err := l.sf.NextIDStr()
	if err != nil {
		return err
	}
	event := LikeEvent{
		EventID:    messageID,
		Action:     action,
		UserID:     userID,
		VideoID:    videoID,
		OccurredAt: time.Now(),
	}
	return rabbitmq.PublishJSON(ctx, l.ch, likeExchange, routingKey, messageID, event)
}
