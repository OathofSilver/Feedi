package producer

import (
	"context"
	"errors"
	"feed/backend/internal/utils/rabbitmq"
	"feed/backend/internal/utils/snowflake"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

type CommentMQ struct {
	ch *amqp.Channel
	sf *snowflake.Node
}

const (
	commentExchange   = "comment.events"
	commentQueue      = "comment.events"
	commentBindingKey = "comment.*"

	commentPublishRK = "comment.publish"
	commentDeleteRK  = "comment.delete"
)

type CommentEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	CommentID  uint      `json:"comment_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	VideoID    uint      `json:"video_id,omitempty"`
	AuthorID   uint      `json:"author_id,omitempty"`
	Content    string    `json:"content,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewCommentMQ(base *rabbitmq.RabbitMQ, workerID int64) (*CommentMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := rabbitmq.DeclareTopic(ch, commentExchange, commentQueue, commentBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	// 初始化雪花算法发号器节点，生成消息唯一ID
	sfNode, err := snowflake.NewNode(workerID)
	if err != nil {
		ch.Close()
		return nil, err
	}
	return &CommentMQ{ch: ch, sf: sfNode}, nil
}

func (c *CommentMQ) Publish(ctx context.Context, username string, videoID, authorID uint, content string) error {
	return c.publish(ctx, "publish", commentPublishRK, CommentEvent{
		Username: username,
		VideoID:  videoID,
		AuthorID: authorID,
		Content:  content,
	})
}

// Delete 评论删除事件。commentID 与 videoID 均冗余携带：
// 消费者收到事件后直接据此做热度 -1，无需再反查（评论行已由 API 同步删除，反查必然落空）
func (c *CommentMQ) Delete(ctx context.Context, commentID, videoID uint) error {
	return c.publish(ctx, "delete", commentDeleteRK, CommentEvent{
		CommentID: commentID,
		VideoID:   videoID,
	})
}

func (c *CommentMQ) publish(ctx context.Context, action, routingKey string, evt CommentEvent) error {
	if c == nil || c.ch == nil {
		return errors.New("comment mq is not initialized")
	}
	messageID, err := c.sf.NextIDStr()
	if err != nil {
		return err
	}
	evt.EventID = messageID
	evt.Action = action
	evt.OccurredAt = time.Now().UTC()
	return rabbitmq.PublishJSON(ctx, c.ch, commentExchange, routingKey, messageID, evt)
}
