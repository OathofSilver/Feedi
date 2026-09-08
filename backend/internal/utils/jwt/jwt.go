package jwt

import (
	"context"
	"errors"
	"feed/backend/internal/middleware"
	rediscache "feed/backend/internal/utils/redis"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

// JWTAuth check jwt token and ensure it matches the currently stored token.
func JWTAuth(cache *rediscache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		tokenString := parts[1]

		claims, err := middleware.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		check(c, claims, tokenString, cache)
	}
}

func SoftJWTAuth(cache *rediscache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		tokenString := parts[1]

		claims, err := middleware.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		check(c, claims, tokenString, cache)
	}
}

func check(c *gin.Context, claims *middleware.Claims, tokenString string, cache *rediscache.Client) {
	// 单会话校验：Redis 中 account-access-token:{id} 保存当前有效 token，
	// 与请求携带的不一致即说明已被登出/重置密码/改名换发，拒绝访问
	if cache != nil {
		cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
		defer cancel()

		b, err := cache.GetBytes(cacheCtx, cache.Key(rediscache.AccessTokenFmt, claims.AccountID))
		if err == nil {
			if string(b) != tokenString {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				return
			}
			c.Set("accountID", claims.AccountID)
			c.Set("username", claims.Username)
			c.Next()
			return
		}
		// Redis 查询失败或 token 未在缓存中（key 与 JWT 的 TTL 已对齐，正常不会出现）：
		// 降级为仅依赖 JWT 验签，保证 Redis 故障时不至于全站不可用；
		// 代价是故障窗口内已被登出的 token 仍可通过验签，属于可用性与安全性的权衡
	}

	c.Set("accountID", claims.AccountID)
	c.Set("username", claims.Username)
	c.Next()
}

func GetAccountID(c *gin.Context) (uint, error) {
	uidValue, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}

	accountID, ok := uidValue.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}

	return accountID, nil
}

func GetUsername(c *gin.Context) (string, error) {
	val, exists := c.Get("username")
	if !exists {
		return "", errors.New("username not found")
	}

	username, ok := val.(string)
	if !ok {
		return "", errors.New("username has invalid type")
	}

	return username, nil
}
