package social

import (
	"context"

	"feed/backend/internal/account"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// socialListLimit 关注/粉丝列表单次返回上限
const socialListLimit = 200

type SocialRepositoryer interface {
	// AccountExists 检查账号是否存在，用于关注/取关前的目标校验
	AccountExists(ctx context.Context, id uint) (bool, error)
	// Follow 添加关注关系（重复关注幂等）
	Follow(ctx context.Context, followerID, vloggerID uint) error
	// Unfollow 删除关注关系
	Unfollow(ctx context.Context, followerID, vloggerID uint) error
	// GetAllFollowers 获取某用户的粉丝列表（最多 200 个，按关注时间倒序）
	GetAllFollowers(ctx context.Context, vloggerID uint) ([]*account.Account, error)
	// GetAllVloggers 获取某用户关注的用户列表（最多 200 个，按关注时间倒序）
	GetAllVloggers(ctx context.Context, followerID uint) ([]*account.Account, error)
	// IsFollowed 检查 followerID 是否已关注 vloggerID
	IsFollowed(ctx context.Context, followerID, vloggerID uint) (bool, error)
	// CountFollowers 统计某用户的粉丝数量
	CountFollowers(ctx context.Context, vloggerID uint) (int64, error)
	// CountVloggers 统计某用户关注的人数
	CountVloggers(ctx context.Context, followerID uint) (int64, error)
}

type socialRepo struct {
	db *gorm.DB
}

// NewSocialRepository 创建关注关系 Repository 实例
func NewSocialRepository(db *gorm.DB) SocialRepositoryer {
	return &socialRepo{db: db}
}

// AccountExists 检查账号是否存在
func (r *socialRepo) AccountExists(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&account.Account{}).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// Follow 添加关注关系，唯一索引冲突时忽略，保证重复关注幂等
func (r *socialRepo) Follow(ctx context.Context, followerID, vloggerID uint) error {
	rel := &Social{FollowerID: followerID, VloggerID: vloggerID}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(rel).Error
}

// Unfollow 删除关注关系
func (r *socialRepo) Unfollow(ctx context.Context, followerID, vloggerID uint) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Delete(&Social{}).Error
}

// GetAllFollowers 获取某用户的粉丝列表，按关注时间倒序，最多返回 200 个
func (r *socialRepo) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*account.Account, error) {
	result := make([]*account.Account, 0)
	err := r.db.WithContext(ctx).
		Select("accounts.id, accounts.username, accounts.avatar_url, accounts.bio").
		Joins("JOIN socials ON socials.follower_id = accounts.id AND socials.vlogger_id = ?", vloggerID).
		Order("socials.id DESC").
		Limit(socialListLimit).
		Find(&result).Error
	return result, err
}

// GetAllVloggers 获取某用户关注的用户列表，按关注时间倒序，最多返回 200 个
func (r *socialRepo) GetAllVloggers(ctx context.Context, followerID uint) ([]*account.Account, error) {
	result := make([]*account.Account, 0)
	err := r.db.WithContext(ctx).
		Select("accounts.id, accounts.username, accounts.avatar_url, accounts.bio").
		Joins("JOIN socials ON socials.vlogger_id = accounts.id AND socials.follower_id = ?", followerID).
		Order("socials.id DESC").
		Limit(socialListLimit).
		Find(&result).Error
	return result, err
}

// IsFollowed 检查 followerID 是否已关注 vloggerID
func (r *socialRepo) IsFollowed(ctx context.Context, followerID, vloggerID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Social{}).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Count(&count).Error
	return count > 0, err
}

// CountFollowers 统计某用户的粉丝数量
func (r *socialRepo) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Social{}).
		Where("vlogger_id = ?", vloggerID).
		Count(&count).Error
	return count, err
}

// CountVloggers 统计某用户关注的人数
func (r *socialRepo) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Social{}).
		Where("follower_id = ?", followerID).
		Count(&count).Error
	return count, err
}
