package rediscache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"feed/backend/internal/config"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

type Client struct {
	rdb       *redis.Client
	keyPrefix string
}

const defaultKeyPrefix = "v1:"

func NewClient(rdb *redis.Client, keyPrefix string) *Client {
	return &Client{rdb: rdb, keyPrefix: keyPrefix}
}

func NewFromEnv(cfg *config.Redis) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb: rdb, keyPrefix: defaultKeyPrefix}, nil
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return errors.New("redis client not initialized")
	}
	return c.rdb.Ping(ctx).Err()
}

func IsMiss(err error) bool {
	return err == redis.Nil
}

func (c *Client) Key(format string, args ...any) string {
	prefix := ""
	if c != nil {
		prefix = c.keyPrefix
	}
	return prefix + fmt.Sprintf(format, args...)
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	if c == nil || c.rdb == nil {
		return "", false, nil
	}
	token, err = randToken(16)
	if err != nil {
		return "", false, err
	}
	ok, err = c.rdb.SetNX(ctx, key, token, ttl).Result()
	return token, ok, err
}

// unlockScript Redis Lua脚本：安全释放分布式锁
// KEYS[1]: 锁key
// ARGV[1]: 锁持有者唯一标识（随机token/请求ID）
var unlockScript = redis.NewScript(`
-- 获取当前锁存储的值
if redis.call("GET", KEYS[1]) == ARGV[1] then
    -- 确认是自己持有的锁，执行删除释放
    return redis.call("DEL", KEYS[1])
else
    -- 不是自己的锁，禁止删除，返回0
    return 0
end
`)

// incrementWithExpireScript Redis Lua脚本：计数自增，首次创建时设置过期时间（限流专用）
// KEYS[1]: 限流计数器key
// ARGV[1]: 过期时长，单位 毫秒(pexpire)
var incrementWithExpireScript = redis.NewScript(`
-- 计数器自增 +1
local count = redis.call("INCR", KEYS[1])
-- count == 1 代表key刚刚新建（之前不存在）
if count == 1 then
    -- 仅首次创建key时设置过期时间
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
-- 返回当前计数值
return count
`)

func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	_, err := unlockScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}

func (c *Client) IncrementWithExpire(ctx context.Context, key string, expire time.Duration) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	return incrementWithExpireScript.Run(
		ctx,
		c.rdb,
		[]string{key},
		expire.Milliseconds(),
	).Int64()
}
