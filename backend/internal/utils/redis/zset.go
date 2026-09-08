package rediscache

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

func (c *Client) ZincrBy(ctx context.Context, key string, member string, score float64) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZIncrBy(ctx, key, score, member).Err()
}

func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZAdd(ctx, key, members...).Err()
}

func (c *Client) ZRemRangeByRank(ctx context.Context, key string, start int64, stop int64) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZRemRangeByRank(ctx, key, start, stop).Err()
}

func (c *Client) ZRangeWithScores(ctx context.Context, key string, start int64, stop int64) ([]redis.Z, error) {
	if c == nil || c.rdb == nil {
		return nil, errors.New("redis client not initialized")
	}
	return c.rdb.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// ZUnionStore 计算给定的一个或多个有序集合的并集，并将结果存储到目标键 dst 中。
// 参数说明：
// - ctx: 上下文，用于控制请求的生命周期（如超时、取消）。
// - dst: 目标有序集合的键名，如果该键已存在，则会被直接覆盖。
// - keys: 参与并集计算的源有序集合键名列表。
// - aggregate: 聚合策略，支持 "SUM"（求和，默认）、"MIN"（取最小值）、"MAX"（取最大值）。
// 返回值：
// - error: 如果 Redis 客户端未初始化则直接返回 nil（静默失败），否则返回 Redis 执行结果中的错误信息。
func (c *Client) ZUnionStore(ctx context.Context, dst string, keys []string, aggregate string) error {
	// 防御性检查：如果客户端实例或底层 Redis 连接为空，直接返回 nil，避免空指针 panic
	if c == nil || c.rdb == nil {
		return nil
	}
	// 调用 go-redis 的 ZUnionStore 方法执行并集聚合操作
	return c.rdb.ZUnionStore(ctx, dst, &redis.ZStore{
		Keys:      keys,      // 指定参与计算的源有序集合
		Aggregate: aggregate, // 指定分数聚合方式
	}).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.ZRevRange(ctx, key, start, stop).Result()
}

// ZRevRangeByScore 按分数从高到低返回有序集合内的元素
// ctx: 上下文对象，用于控制超时、取消
// key: redis有序集合key
// max: 最大分数，redis语法，例如 "100"、"+inf"，分数上限
// min: 最小分数，redis语法，例如 "0"、"-inf"，分数下限
// offset: 分页偏移量
// count: 返回条数
// return: 返回对应范围内的成员字符串切片，出错返回error
func (c *Client) ZRevRangeByScore(ctx context.Context, key string, max, min string, offset, count int64) ([]string, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Max:    max,    // 分数上限，ZRevRangeByScore是倒序，优先取靠近Max的数据
		Min:    min,    // 分数下限
		Offset: offset, // 分页偏移
		Count:  count,  // 返回元素数量
	}).Result()
}
