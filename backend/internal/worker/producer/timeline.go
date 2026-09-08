package producer

import (
	"context"
	"errors"
	"time"

	"feed/backend/internal/utils/rabbitmq"
	"feed/backend/internal/utils/snowflake"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TimelineMQ struct {
	ch *amqp.Channel
	sf *snowflake.Node // 雪花算法发号器节点，生成消息唯一ID
}

const (
	// TimelineExchange 视频时间线 Topic 交换机名称，分发动态推送相关事件
	TimelineExchange = "video.timeline.events"
	// TimelineQueue 时间线更新消费队列，接收用户关注视频动态事件
	TimelineQueue = "video.timeline.update.queue"
	// TimelineBindingKey 队列绑定路由键，通配符匹配 video.timeline.xxx 事件
	TimelineBindingKey = "video.timeline.*"
	// TimelinePublishRK 视频发布事件投递路由 key，消息发送时使用
	TimelinePublishRK = "video.timeline.publish"
)

// TimelineEvent 视频时间线事件消息体
type TimelineEvent struct {
	EventID    string    `json:"event_id"`    // 消息唯一ID，雪花算法生成，消费者据此幂等去重
	VideoID    uint      `json:"video_id"`    // 视频ID
	CreateTime int64     `json:"create_time"` // 视频创建时间戳（毫秒）
	OccurredAt time.Time `json:"occurred_at"` // 事件发生时间
}

// base 为 RabbitMQ 连接管理实例；workerID 为雪花算法节点ID，多实例部署时各节点须唯一
func NewTimelineMQ(base *rabbitmq.RabbitMQ, workerID int64) (*TimelineMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := rabbitmq.DeclareTopic(ch, TimelineExchange, TimelineQueue, TimelineBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	// 初始化雪花算法发号器节点，生成消息唯一ID
	sfNode, err := snowflake.NewNode(workerID)
	if err != nil {
		ch.Close()
		return nil, err
	}
	return &TimelineMQ{ch: ch, sf: sfNode}, nil
}

func (t *TimelineMQ) PublishVideo(ctx context.Context, videoID uint, createTime time.Time) error {
	if t == nil || t.ch == nil {
		return errors.New("timeline mq is not initialized")
	}
	if videoID == 0 {
		return errors.New("videoID are required")
	}
	messageID, err := t.sf.NextIDStr()
	if err != nil {
		return err
	}
	timeline := TimelineEvent{
		EventID:    messageID,
		VideoID:    videoID,
		CreateTime: createTime.UnixMilli(),
		OccurredAt: time.Now(),
	}
	return rabbitmq.PublishJSON(ctx, t.ch, TimelineExchange, TimelinePublishRK, messageID, timeline)
}
