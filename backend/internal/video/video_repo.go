package video

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const videoListLimit = 200

type VideoRepositoryer interface {
	IsExist(ctx context.Context, id uint) (bool, error)
	// Create 创建视频记录（单表操作）
	Create(ctx context.Context, video *Video) error
	// Delete 按 ID 删除视频
	Delete(ctx context.Context, id uint) error
	// FindByID 按 ID 查询视频，记录不存在时返回 gorm.ErrRecordNotFound
	FindByID(ctx context.Context, id uint) (*Video, error)
	// ListByAuthorID 获取作者的视频列表（最多 200 个，按创建时间倒序）
	ListByAuthorID(ctx context.Context, authorID uint) ([]*Video, error)
	// UpdateLikesCount 以增量方式原子更新点赞数，delta 可为负，
	// 避免并发下的读-改-写丢失更新；结果不会小于 0
	UpdateLikesCount(ctx context.Context, id uint, delta int64) error
	// UpdatePopularity 以增量方式原子更新热度分值，change 可为负，
	// 与 UpdateLikesCount 同样的并发安全策略；结果不会小于 0
	UpdatePopularity(ctx context.Context, id uint, change int64) error
	// CountByAuthorID 统计作者的视频总数
	CountByAuthorID(ctx context.Context, authorID uint) (int64, error)
	// SumLikesByAuthorID 统计作者全部视频的点赞总数
	SumLikesByAuthorID(ctx context.Context, authorID uint) (int64, error)
	// CountVideos 统计满足条件的视频总数，authorID 为 0 时统计全量
	CountVideos(ctx context.Context, authorID uint) (int64, error)
	// WithTx 返回绑定指定事务的仓库实例，供 Service 层多表事务操作时传入事务
	WithTx(tx *gorm.DB) VideoRepositoryer
}

type videoRepo struct {
	db *gorm.DB
}

// NewVideoRepository 创建视频 Repository 实例
func NewVideoRepository(db *gorm.DB) VideoRepositoryer {
	return &videoRepo{db: db}
}

// Create 创建视频记录
func (r *videoRepo) Create(ctx context.Context, video *Video) error {
	return r.db.WithContext(ctx).Create(video).Error
}

// WithTx 返回绑定指定事务的仓库实例，Service 层多表操作时传入事务保证原子性
func (r *videoRepo) WithTx(tx *gorm.DB) VideoRepositoryer {
	return &videoRepo{db: tx}
}

// Delete 按 ID 删除视频
func (r *videoRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&Video{}).Error
}

// FindByID 按 ID 查询视频，记录不存在时返回 gorm.ErrRecordNotFound
func (r *videoRepo) FindByID(ctx context.Context, id uint) (*Video, error) {
	var v Video
	if err := r.db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// ListByAuthorID 获取作者的视频列表，按创建时间倒序，最多返回 200 个
func (r *videoRepo) ListByAuthorID(ctx context.Context, authorID uint) ([]*Video, error) {
	result := make([]*Video, 0)
	err := r.db.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("create_time DESC, id DESC").
		Limit(videoListLimit).
		Find(&result).Error
	return result, err
}

// UpdateLikesCount 以增量方式原子更新点赞数（likes_count = likes_count + delta），
// GREATEST 兜底防止点赞数被扣为负数
func (r *videoRepo) UpdateLikesCount(ctx context.Context, id uint, delta int64) error {
	return r.db.WithContext(ctx).
		Model(&Video{}).
		Where("id = ?", id).
		Update("likes_count", gorm.Expr("GREATEST(likes_count + ?, 0)", delta)).Error
}

// UpdatePopularity 以增量方式原子更新热度分值，change 可为负（点赞 +1 / 取消点赞 -1）。
// 必须使用 gorm.Expr 增量写而非覆盖写：调用方（点赞/评论消费者、Service 层）传的都是增量，
// 覆盖写会把热度值直接改写为 ±1，导致热度数据错乱
func (r *videoRepo) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	return r.db.WithContext(ctx).
		Model(&Video{}).
		Where("id = ?", id).
		Update("popularity", gorm.Expr("GREATEST(popularity + ?, 0)", change)).Error
}

// CountByAuthorID 统计作者的视频总数
func (r *videoRepo) CountByAuthorID(ctx context.Context, authorID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Video{}).
		Where("author_id = ?", authorID).
		Count(&count).Error
	return count, err
}

// SumLikesByAuthorID 统计作者全部视频的点赞总数，无视频时返回 0
func (r *videoRepo) SumLikesByAuthorID(ctx context.Context, authorID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&Video{}).
		Where("author_id = ?", authorID).
		Select("COALESCE(SUM(likes_count), 0)").
		Scan(&total).Error
	return total, err
}

// CountVideos 统计视频总数，authorID 大于 0 时仅统计该作者的视频
func (r *videoRepo) CountVideos(ctx context.Context, authorID uint) (int64, error) {
	query := r.db.WithContext(ctx).Model(&Video{})
	if authorID > 0 {
		query = query.Where("author_id = ?", authorID)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (r *videoRepo) IsExist(ctx context.Context, id uint) (bool, error) {
	var video Video
	if err := r.db.WithContext(ctx).First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
