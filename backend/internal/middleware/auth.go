package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT 鉴权配置，由应用入口在启动时通过 InitAuth 注入。
// 签发与验签必须使用同一份密钥，进程生命周期内不可变更
var (
	authMu       sync.RWMutex
	secretKey    []byte
	expireHours  int // access-token 有效期（小时）
	refreshHours int // refresh-token 有效期（小时）
)

// InitAuth 初始化 JWT 密钥与有效期。
// 必须在进程启动、任何签发/验签发生之前调用；
// secret 来自 configs/config.yaml 的 jwt.secret，多实例部署必须保持一致，
// 否则 A 实例签发的 token 在 B 实例上无法通过验签
func InitAuth(secret string, expire, refresh int) {
	if secret == "" {
		panic("middleware.InitAuth: jwt.secret 不能为空，请检查 configs/config.yaml")
	}
	if expire <= 0 {
		expire = 24
	}
	if refresh <= 0 {
		refresh = 168
	}
	authMu.Lock()
	defer authMu.Unlock()
	secretKey = []byte(secret)
	expireHours = expire
	refreshHours = refresh
}

// jwtSecret 返回签发/验签密钥。
// 优先使用 InitAuth 注入的配置密钥；未初始化时兜底读取 JWT_SECRET 环境变量
// （兼容单元测试与临时脚本）；两者都缺失则直接报错——
// 绝不允许每次调用随机生成密钥，否则签发与验签密钥不同，token 永远校验失败
func jwtSecret() ([]byte, error) {
	authMu.RLock()
	s := secretKey
	authMu.RUnlock()
	if len(s) > 0 {
		return s, nil
	}
	if s := envSecret(); len(s) > 0 {
		return s, nil
	}
	return nil, errors.New("JWT 密钥未初始化：请调用 middleware.InitAuth 或设置 JWT_SECRET 环境变量")
}

var envOnce sync.Once
var envSecretValue []byte

// envSecret 兜底读取 JWT_SECRET 环境变量，结果缓存避免重复解析
func envSecret() []byte {
	envOnce.Do(func() {
		if s := os.Getenv("JWT_SECRET"); s != "" {
			envSecretValue = []byte(s)
		}
	})
	return envSecretValue
}

type Claims struct {
	AccountID uint   `json:"account_id"`
	Username  string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 access-token，有效期由配置 jwt.expire_hours 决定
func GenerateToken(accountID uint, username string) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}
	now := time.Now()

	claims := Claims{
		AccountID: accountID,
		Username:  username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL())),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(secret)
}

// GenerateRefreshToken 生成随机 opaque refresh-token，
// 状态保存在 Redis，生命周期由服务层管理
func GenerateRefreshToken(accountID uint) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ParseToken(tokenString string) (*Claims, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return secret, nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

// AccessTokenTTL access-token 的有效期，与 JWT exp 声明共用同一配置，
// 服务层写入 Redis 的 token TTL 必须使用同一值，避免出现
// "JWT 已过期但 Redis 还留有旧 token" 或 "Redis 已清理但 JWT 仍有效" 的不一致窗口
func AccessTokenTTL() time.Duration {
	authMu.RLock()
	defer authMu.RUnlock()
	hours := expireHours
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

// RefreshTokenTTL refresh-token 的有效期，服务层写 Redis 与刷新校验共用
func RefreshTokenTTL() time.Duration {
	authMu.RLock()
	defer authMu.RUnlock()
	hours := refreshHours
	if hours <= 0 {
		hours = 168
	}
	return time.Duration(hours) * time.Hour
}
