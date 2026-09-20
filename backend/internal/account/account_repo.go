package account

import (
	"context"

	"gorm.io/gorm"
)

// AccountRepository 账号数据访问接口，屏蔽底层存储实现
type AccountRepositoryer interface {
	// Create 创建账号
	Create(ctx context.Context, account *Account) error
	// FindByID 按 ID 查询账号
	FindByID(ctx context.Context, id uint) (*Account, error)
	// FindByUsername 按用户名查询账号，用于登录与重名校验
	FindByUsername(ctx context.Context, username string) (*Account, error)
	// UpdateProfile 按需更新头像与简介（传入 map 仅更新给定字段）
	UpdateProfile(ctx context.Context, id uint, updates map[string]interface{}) error
	// UpdatePassword 更新密码
	UpdatePassword(ctx context.Context, id uint, password string) error
	// UpdateUsername 更新用户名
	UpdateUsername(ctx context.Context, id uint, username string) error
	// UpdateAvatar 更新头像
	UpdateAvatar(ctx context.Context, id uint, avatarURL string) error
	// FindAll 寻找全部
	FindAll(ctx context.Context) ([]*Account, error)
}

// accountRepo AccountRepository 的 GORM 实现
type accountRepo struct {
	db *gorm.DB
}

// NewAccountRepository 创建账号 Repository 实例
func NewAccountRepository(db *gorm.DB) AccountRepositoryer {
	return &accountRepo{db: db}
}

// Create 创建账号
func (r *accountRepo) Create(ctx context.Context, account *Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

// FindByID 按 ID 查询账号，记录不存在时返回 gorm.ErrRecordNotFound
func (r *accountRepo) FindByID(ctx context.Context, id uint) (*Account, error) {
	var acc Account
	if err := r.db.WithContext(ctx).First(&acc, id).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// FindByUsername 按用户名查询账号，记录不存在时返回 gorm.ErrRecordNotFound
func (r *accountRepo) FindByUsername(ctx context.Context, username string) (*Account, error) {
	var acc Account
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// UpdateProfile 按需更新头像与简介，map 只包含需更新的字段，可单独更新其中一项
func (r *accountRepo) UpdateProfile(ctx context.Context, id uint, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(updates).Error
}

// UpdatePassword 更新密码
func (r *accountRepo) UpdatePassword(ctx context.Context, id uint, password string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).
		Update("password", password).Error
}

// UpdateUsername 更新用户名
func (r *accountRepo) UpdateUsername(ctx context.Context, id uint, username string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).
		Update("username", username).Error
}

// UpdateAvatar 更新头像
func (r *accountRepo) UpdateAvatar(ctx context.Context, id uint, avatarURL string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).
		Update("avatar_url", avatarURL).Error
}

// FindAll 寻找全部
func (r *accountRepo) FindAll(ctx context.Context) ([]*Account, error) {
	var accs []*Account
	if err := r.db.WithContext(ctx).Find(&accs).Error; err != nil {
		return nil, err
	}
	return accs, nil
}
