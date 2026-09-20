package video

import (
	"context"
	"time"

	"gorm.io/gorm"
)


// OutboxMsg 事务消息表（Outbox 模式）
// 用于解决本地数据库事务与发送MQ无法原子执行的问题
// 业务操作和消息记录在同一个事务入库，由独立轮询程序读取记录投递消息
type OutboxMsg struct {
	ID         uint      `gorm:"primaryKey"`             // 自增主键
	VideoID    uint      `gorm:"index"`                  // 关联视频ID，建立索引便于根据视频查询事件
	EventType  string    `gorm:"type:varchar(50)"`       // 事件类型，例如 video_published、video_like
	CreateTime time.Time `gorm:"autoCreateTime"`         // 消息创建时间，数据库自动填充
	Status     string    `gorm:"type:varchar(50);index"` // 消息状态：pending待投递 / success投递成功 / failed投递失败
}



// OutboxRepository 事务消息（Outbox 表）数据访问接口，只做单表操作，多表事务由 Service 层编排
type OutboxRepository interface {
	Create(ctx context.Context, msg *OutboxMsg) error
	// WithTx 返回绑定指定事务的仓库实例，供 Service 层多表事务操作时传入事务
	WithTx(tx *gorm.DB) OutboxRepository
}

type outboxRepo struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepo{db: db}
}

func (r *outboxRepo) Create(ctx context.Context, msg *OutboxMsg) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// WithTx 返回绑定指定事务的仓库实例
func (r *outboxRepo) WithTx(tx *gorm.DB) OutboxRepository {
	return &outboxRepo{db: tx}
}
