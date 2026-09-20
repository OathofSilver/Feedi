package account

import (
	"context"
	"errors"
	"feed/backend/internal/middleware"
	rediscache "feed/backend/internal/utils/redis"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"feed/backend/internal/apierror"
)

// Redis token key 模板统一引自 rediscache/keys.go（唯一事实来源），
// 签发、校验（utils/jwt）、登出三处共用同一常量，杜绝手写不一致
const (
	accessTokenKeyFmt     = rediscache.AccessTokenFmt
	refreshTokenKeyFmt    = rediscache.RefreshTokenFmt
	refreshTokenMapKeyFmt = rediscache.RefreshTokenMapFmt
)

// 业务错误，Handler 层据此映射 HTTP 状态码与业务响应码
var (
	ErrInvalidParam      = errors.New("请求参数错误")
	ErrAccountNotFound   = errors.New("账号不存在")
	ErrUsernameExists    = errors.New("用户名已存在")
	ErrInvalidCredential = errors.New("用户名或密码错误")
	ErrInvalidToken      = errors.New("令牌无效或已过期")
	ErrUnauthorized      = apierror.ErrUnauthorized
)

type AccountService struct {
	accountRepository AccountRepositoryer // 接口不要用指针
	cache             *rediscache.Client
}

func NewAccountService(accountRepository AccountRepositoryer, cache *rediscache.Client) *AccountService {
	return &AccountService{accountRepository: accountRepository, cache: cache}
}

func (as *AccountService) CreateAccount(ctx context.Context, account *Account) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(account.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	account.Password = string(passwordHash)
	if err := as.accountRepository.Create(ctx, account); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) Rename(ctx context.Context, accountID uint, newUsername string) (string, error) {
	if newUsername == "" {
		return "", ErrInvalidParam
	}
	// access-token
	token, err := middleware.GenerateToken(accountID, newUsername)
	if err != nil {
		return "", err
	}

	if err := as.accountRepository.UpdateUsername(ctx, accountID, newUsername); err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return "", ErrUsernameExists
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		return "", err
	}

	// 更改redis 的access-token（使旧客户端持有的 token 立即失效）
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := as.cache.SetBytes(cacheCtx, as.cache.Key(accessTokenKeyFmt, accountID), []byte(token), middleware.AccessTokenTTL()); err != nil {
			slog.Warn("更新 access-token 缓存失败", "account_id", accountID, "err", err)
		}
	}
	return token, nil
}

func (as *AccountService) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	account, err := as.accountRepository.FindByUsername(ctx, username)
	if err != nil {
		return err
	}
	// 还原password
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(oldPassword)); err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := as.accountRepository.UpdatePassword(ctx, account.ID, string(passwordHash)); err != nil {
		return err
	}
	// 清空redis token
	if err := as.Logout(ctx, account.ID); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) FindByID(ctx context.Context, id uint) (*Account, error) {
	if account, err := as.accountRepository.FindByID(ctx, id); err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (as *AccountService) FindByUsername(ctx context.Context, username string) (*Account, error) {
	if account, err := as.accountRepository.FindByUsername(ctx, username); err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (as *AccountService) Login(ctx context.Context, username, password string) (string, string, error) {
	account, err := as.accountRepository.FindByUsername(ctx, username)
	if err != nil {
		return "", "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return "", "", err
	}
	// 生成新的access-token
	accessToken, err := middleware.GenerateToken(account.ID, account.Username)
	if err != nil {
		return "", "", err
	}
	// 生成新的RefreshToke
	refreshToken, err := middleware.GenerateRefreshToken(account.ID)
	if err != nil {
		return "", "", err
	}
	// 更新缓存中的 access-token 与双向 refresh-token 映射：
	// 正向 accountID -> refreshToken（单会话校验/登出用），反向 refreshToken -> accountID（刷新时反查用户）
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		refreshTTL := middleware.RefreshTokenTTL()
		if err := as.cache.SetBytes(cacheCtx, as.cache.Key(accessTokenKeyFmt, account.ID), []byte(accessToken), middleware.AccessTokenTTL()); err != nil {
			slog.Warn("写入 access-token 缓存失败", "account_id", account.ID, "err", err)
		}
		if err := as.cache.SetBytes(cacheCtx, as.cache.Key(refreshTokenKeyFmt, account.ID), []byte(refreshToken), refreshTTL); err != nil {
			slog.Warn("写入 refresh-token 缓存失败", "account_id", account.ID, "err", err)
		}
		if err := as.cache.SetBytes(cacheCtx, as.cache.Key(refreshTokenMapKeyFmt, refreshToken), []byte(strconv.FormatUint(uint64(account.ID), 10)), refreshTTL); err != nil {
			slog.Warn("写入 refresh-token 反向映射失败", "account_id", account.ID, "err", err)
		}
	}
	return accessToken, refreshToken, nil
}

func (as *AccountService) UpdateAvatar(ctx context.Context, accountID uint, avatarURL string) error {
	return as.accountRepository.UpdateAvatar(ctx, accountID, avatarURL)
}

func (as *AccountService) FindAll(ctx context.Context) ([]*Account, error) {
	return as.accountRepository.FindAll(ctx)
}

// UpdateProfile 更新个人资料（头像/简介均非必填，只更新传入的字段）
func (as *AccountService) UpdateProfile(ctx context.Context, accountID uint, req *UpdateProfileRequest) error {
	updates := map[string]interface{}{}
	if avatarURL := strings.TrimSpace(req.AvatarURL); avatarURL != "" {
		updates["avatar_url"] = avatarURL
	}
	if bio := strings.TrimSpace(req.Bio); bio != "" {
		updates["bio"] = bio
	}
	if len(updates) == 0 {
		return ErrInvalidParam
	}
	// 将 map 透传给仓库层，未传入的字段不会被更新或清空
	return as.accountRepository.UpdateProfile(ctx, accountID, updates)
}

// RefreshAccessToken 用 refresh-token 换发新的 access-token。
// 流程：反向映射查 accountID -> 正向映射校验单会话 -> 查账号确认存在 -> 签发新 access-token 并更新缓存
func (as *AccountService) RefreshAccessToken(ctx context.Context, refreshToken string) (string, uint, string, error) {
	if refreshToken == "" {
		return "", 0, "", ErrInvalidParam
	}
	if as.cache == nil {
		// refresh-token 状态完全依赖 Redis，无缓存则刷新链路不可用
		return "", 0, "", ErrInvalidToken
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	// 1. 反向映射：refreshToken -> accountID；不存在说明 token 已过期或被登出清理
	idBytes, err := as.cache.GetBytes(cacheCtx, as.cache.Key(refreshTokenMapKeyFmt, refreshToken))
	if err != nil || len(idBytes) == 0 {
		return "", 0, "", ErrInvalidToken
	}
	accountID64, err := strconv.ParseUint(string(idBytes), 10, 64)
	if err != nil {
		return "", 0, "", ErrInvalidToken
	}
	accountID := uint(accountID64)

	// 2. 正向映射校验：id -> token 必须与提交的一致，保证同一账号只有一个有效 refresh-token（单会话）
	storedToken, err := as.cache.GetBytes(cacheCtx, as.cache.Key(refreshTokenKeyFmt, accountID))
	if err != nil || string(storedToken) != refreshToken {
		// 正向映射已失效或不匹配：视为旧 token，清掉残留反向映射
		_ = as.cache.Del(cacheCtx, as.cache.Key(refreshTokenMapKeyFmt, refreshToken))
		return "", 0, "", ErrInvalidToken
	}

	// 3. 确认账号仍存在（防止账号删除后 token 仍可刷新）
	account, err := as.accountRepository.FindByID(ctx, accountID)
	if err != nil {
		return "", 0, "", ErrAccountNotFound
	}

	// 4. 签发新 access-token 并更新单会话校验缓存
	accessToken, err := middleware.GenerateToken(account.ID, account.Username)
	if err != nil {
		return "", 0, "", err
	}
	if err := as.cache.SetBytes(cacheCtx, as.cache.Key(accessTokenKeyFmt, account.ID), []byte(accessToken), middleware.AccessTokenTTL()); err != nil {
		slog.Warn("刷新后更新 access-token 缓存失败", "account_id", account.ID, "err", err)
	}
	return accessToken, account.ID, account.Username, nil
}

// Logout 清空 access-token 与双向 refresh-token 映射
func (as *AccountService) Logout(ctx context.Context, accountID uint) error {
	account, err := as.accountRepository.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := as.cache.Del(cacheCtx, as.cache.Key(accessTokenKeyFmt, account.ID)); err != nil {
			slog.Warn("删除 access-token 缓存失败", "account_id", account.ID, "err", err)
		}
		// 先读出 refresh-token 值，再删除双向映射（正向 id->token 与反向 token->id）
		if refreshBytes, err := as.cache.GetBytes(cacheCtx, as.cache.Key(refreshTokenKeyFmt, account.ID)); err == nil && len(refreshBytes) > 0 {
			if err := as.cache.Del(cacheCtx, as.cache.Key(refreshTokenMapKeyFmt, string(refreshBytes))); err != nil {
				slog.Warn("删除 refresh-token 反向映射失败", "account_id", account.ID, "err", err)
			}
		}
		if err := as.cache.Del(cacheCtx, as.cache.Key(refreshTokenKeyFmt, account.ID)); err != nil {
			slog.Warn("删除 refresh-token 缓存失败", "account_id", account.ID, "err", err)
		}
	}
	return nil
}
