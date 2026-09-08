package outboxpoller

import (
	"context"
	"feed/backend/internal/video"
	"feed/backend/internal/worker/producer"
	"gorm.io/gorm"
	"log"
	"time"
)

// StartOutboxPoller 启动Outbox轮询协程，实现Outbox模式消息投递
// 功能：定时扫描outbox表中pending状态待投递消息，投递到RabbitMQ，投递成功删除消息，保障本地事务与MQ最终一致性
func StartOutboxPoller(db *gorm.DB, tmq *producer.TimelineMQ) {
	if db == nil || tmq == nil {
		log.Printf("Outbox poller disabled: timeline mq is not initialized")
		return
	}

	// 启动后台goroutine执行轮询任务
	go func() {
		for {
			var messages []video.OutboxMsg
			// 查询状态为pending（待投递）的消息，按创建时间升序，每次最多取100条，避免单次处理数据量过大
			err := db.Where("status = ?", "pending").Order("create_time ASC").Limit(100).Find(&messages).Error
			// 查询出错 或者 没有待投递消息，sleep1秒后进入下一轮循环
			if err != nil || len(messages) == 0 {
				time.Sleep(1 * time.Second)
				continue
			}

			for _, msg := range messages {
				err := tmq.PublishVideo(context.Background(), msg.VideoID, msg.CreateTime)
				if err == nil {
					// MQ投递成功，删除该outbox记录
					if err := db.Delete(&msg).Error; err != nil {
						log.Printf("删除 outbox 消息失败: id=%d, err=%v", msg.ID, err)
					}
				} else {
					// MQ投递失败，打印错误日志，消息保留在表中，下一轮轮询会重试投递
					log.Printf("投递MQ失败: VideoID: %d, err: %v", msg.VideoID, err)
				}
			}
		}
	}()
}
