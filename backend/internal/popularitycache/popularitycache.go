package popularitycache

import (
	"context"
	rediscache "feed/backend/internal/utils/redis"
	"strconv"
	"time"
)

// UpdatePopularityCache 更新视频热度缓存
func UpdatePopularityCache(ctx context.Context, cache *rediscache.Client, id uint, change int64) {
	if cache == nil || id == 0 || change == 0 {
		return
	}
	// 详情与 feed 实体是同一实体的两份缓存拷贝，统一走失效方法一并删除
	cache.InvalidateVideoCache(context.Background(), id)

	now := time.Now().UTC().Truncate(time.Minute)
	windowKey := cache.Key(rediscache.HotVideoWindowFmt, now.Format("200601021504"))
	member := strconv.FormatUint(uint64(id), 10)

	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_ = cache.ZincrBy(opCtx, windowKey, member, float64(change))
	_ = cache.Expire(opCtx, windowKey, 2*time.Hour)
}
