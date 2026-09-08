package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"feed/backend/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"strconv"
	"time"
)

// RabbitMQ 只管理 Connection，Channel 由各组件按需创建
type RabbitMQ struct {
	Conn *amqp.Connection
}

func NewRabbitMQ(cfg *config.RabbitMQ) (*RabbitMQ, error) {
	if cfg == nil {
		return nil, errors.New("rabbitmq config is nil")
	}
	url := "amqp://" + cfg.Username + ":" + cfg.Password + "@" + cfg.Host + ":" + strconv.Itoa(cfg.Port) + "/"
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &RabbitMQ{Conn: conn}, nil
}

func (r *RabbitMQ) Close() error {
	if r == nil {
		return nil
	}
	if r.Conn != nil {
		return r.Conn.Close()
	}
	return nil
}

func (r *RabbitMQ) NewChannel() (*amqp.Channel, error) {
	if r == nil || r.Conn == nil {
		return nil, errors.New("rabbitmq connection is not initialized")
	}
	return r.Conn.Channel()
}

// 声明交换机 ，队列 和 绑定队列
func DeclareTopic(ch *amqp.Channel, exchange string, queue string, bindingKey string) error {
	if ch == nil {
		return errors.New("channel is not initialized")
	}
	if exchange == "" || queue == "" || bindingKey == "" {
		return errors.New("exchange/queue/bindingKey is required")
	}

	// 声明topic类型交换机
	if err := ch.ExchangeDeclare(
		exchange,
		"topic",
		true,  // durable
		false, // auto‑delete
		false, // internal
		false, // no‑wait
		nil,   // arguments
	); err != nil {
		return err
	}

	// 声明普通队列，移除x‑dead‑letter‑exchange参数
	q, err := ch.QueueDeclare(
		queue,
		true,  // durable
		false, // auto‑delete
		false, // exclusive
		false, // no‑wait
		nil,   // arguments 去掉死信配置
	)
	if err != nil {
		return err
	}

	// 队列绑定到交换机
	if err := ch.QueueBind(
		q.Name,
		bindingKey,
		exchange,
		false, // no‑wait
		nil,   // arguments
	); err != nil {
		return err
	}

	return nil
}

// PublishJSON 将 payload 序列化为 JSON 后发布到交换机
// messageID 为消息唯一ID（雪花算法生成），写入 AMQP MessageId 属性，用于消息追踪与消费幂等
func PublishJSON(ctx context.Context, ch *amqp.Channel, exchange string, routingKey string, messageID string, payload any) error {
	if ch == nil {
		return errors.New("channel is not initialized")
	}
	if exchange == "" || routingKey == "" {
		return errors.New("exchange/routingKey is required")
	}
	// 序列化
	data, err := json.Marshal(payload)
	if err != nil {
		return err // 直接报错
	}
	// 发布消息
	// 保证消息不丢失
	return ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent, // 设置为持久化(2)，确保消息写入磁盘，RabbitMQ 重启后不丢失
		MessageId:    messageID,       // 消息唯一ID（雪花算法生成）
		Timestamp:    time.Now(),      // 设置消息的时间戳
		Body:         data,
	})
}
