package notification

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Repository 通知数据访问接口
type Repository interface {
	// Create 写入一条通知
	Create(ctx context.Context, n *Notification) error
	// ListByRecipient 按收件人倒序查询通知；before 为游标时间（0 表示首页，不设上限过滤）
	ListByRecipient(ctx context.Context, recipientID uint, before time.Time, limit int) ([]Notification, error)
}

type repo struct {
	db *gorm.DB
}

// NewRepository 创建通知仓库实例
func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) Create(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *repo) ListByRecipient(ctx context.Context, recipientID uint, before time.Time, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var list []Notification
	q := r.db.WithContext(ctx).
		Where("recipient_id = ?", recipientID)
	if !before.IsZero() {
		q = q.Where("created_at < ?", before)
	}
	err := q.Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
