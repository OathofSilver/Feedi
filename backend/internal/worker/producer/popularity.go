package producer

import (
	"context"
	"errors"
	"feed/backend/internal/utils/rabbitmq"
	"feed/backend/internal/utils/snowflake"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

type PopularityMQ struct {
	ch *amqp.Channel
	sf *snowflake.Node // 雪花算法发号器节点，生成消息唯一ID
}

const (
	popularityExchange   = "video.popularity.events"
	popularityQueue      = "video.popularity.events"
	popularityBindingKey = "video.popularity.*"

	popularityUpdateRK = "video.popularity.update"
)

// PopularityEvent 视频热度变更事件，MQ异步消息载体
// 用户点赞、取消点赞、新增评论、删除评论、播放、转发等会产生该事件
// 由API服务投递到RabbitMQ，热度消费Worker接收后更新Redis分钟热度窗口、清理视频缓存
type PopularityEvent struct {
	EventID string `json:"event_id"`
	VideoID uint   `json:"video_id"`
	Change  int64  `json:"change"`
	// OccurredAt 事件实际发生时间戳，用于区分时序、日志排查、窗口时间校正
	OccurredAt time.Time `json:"occurred_at"`
}

// NewPopularityMQ 创建热度事件生产者；workerID 为雪花算法节点ID，多实例部署时各节点须唯一
func NewPopularityMQ(base *rabbitmq.RabbitMQ, workerID int64) (*PopularityMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := rabbitmq.DeclareTopic(ch, popularityExchange, popularityQueue, popularityBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	// 初始化雪花算法发号器节点，生成消息唯一ID
	sfNode, err := snowflake.NewNode(workerID)
	if err != nil {
		ch.Close()
		return nil, err
	}
	return &PopularityMQ{ch: ch, sf: sfNode}, nil
}

func (p *PopularityMQ) Update(ctx context.Context, videoID uint, change int64) error {
	if p == nil || p.ch == nil {
		return errors.New("popularity mq is not initialized")
	}
	if videoID == 0 || change == 0 {
		return errors.New("videoID and change are required")
	}
	messageID, err := p.sf.NextIDStr()
	if err != nil {
		return err
	}
	event := PopularityEvent{
		EventID:    messageID,
		VideoID:    videoID,
		Change:     change,
		OccurredAt: time.Now().UTC(),
	}
	return rabbitmq.PublishJSON(ctx, p.ch, popularityExchange, popularityUpdateRK, messageID, event)
}
