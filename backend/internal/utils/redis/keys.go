package rediscache

import "context"

// Redis 缓存 key 模板（项目唯一事实来源，Single Source of Truth）。
//
// 约定：
//   - 层级用冒号分隔：业务域:对象:维度，参数占位与 Go fmt 动词一致
//   - 业务 key 必须通过 Client.Key()（或本文件提供的便捷方法）拼接，
//     自动带上全局前缀（如 "v1:"），禁止绕过前缀直接写 Redis
//   - 任何读写/失效同一份数据的代码只允许引用本文件的常量或方法，
//     禁止手写字符串模板——改格式时只改这里，编译器保证所有使用点同步
const (
	// ---------- 账号会话 ----------
	// AccessTokenFmt access token 正向映射：accountID -> token。
	// 登录/刷新时写入，鉴权时读出比对实现服务端单点失效
	AccessTokenFmt = "account-access-token:%d"
	// RefreshTokenFmt refresh token 正向映射：accountID -> refreshToken（单会话，重新登录覆盖）
	RefreshTokenFmt = "account-refresh-token:%d"
	// RefreshTokenMapFmt refresh token 反向映射：refreshToken -> accountID
	RefreshTokenMapFmt = "account-refresh-token-map:%s"

	// ---------- 视频 ----------
	// VideoDetailFmt 视频详情缓存（GetDetail 回源后写入）
	VideoDetailFmt = "video:detail:id=%d"
	// VideoEntityFmt feed 批量实体缓存（GetVideoByIDs 回源后写入）
	// 与 VideoDetailFmt 是同一实体的两份拷贝，失效时必须同时删除
	VideoEntityFmt = "video:entity:%d"

	// ---------- 热度 ----------
	// HotVideoWindowFmt 分钟热度窗口 ZSET，参数：time.Format("200601021504")
	HotVideoWindowFmt = "hot:video:1m:%s"
	// HotVideoMergeFmt 多窗口合并后的热榜快照 ZSET，参数：time.Format("200601021504")
	HotVideoMergeFmt = "hot:video:merge:1m:%s"

	// ---------- Feed 时间线 ----------
	// FeedGlobalTimelineKey 全局时间线 ZSET（member=videoID, score=发布时间毫秒）
	FeedGlobalTimelineKey = "feed:global_timeline"
	// FeedFollowingFmt 关注流响应缓存，参数：limit, accountID, before(秒时间戳, 0 表示首页)
	FeedFollowingFmt = "feed:listByFollowing:limit=%d:accountID=%d:before=%d"
	// FeedFollowingPatternFmt 关注流批量失效的 SCAN pattern，参数：accountID。
	// 与 FeedFollowingFmt 强耦合，格式变更时必须同步修改
	FeedFollowingPatternFmt = "feed:listByFollowing:*:accountID=%d:*"

	// ---------- singleflight 分片键（仅进程内请求合并，不落 Redis）----------
	SFEntityFmt           = "sf:entity:%d"
	SFTimelineRebuildKey  = "sf:fallback:global_timeline_rebuild"
	SFColdListLatestFmt   = "sf:cold:listLatest:%d:%d"
	SFStitchListLatestFmt = "sf:stitch:listLatest:%d:%d"

	// ---------- 消息幂等（worker 消费去重，value 固定 "1"）----------
	MsgProcessedFmt        = "msg:processed:%s"
	MsgLikeProcessedFmt    = "msg:like:processed:%s"
	MsgCommentProcessedFmt = "msg:comment:processed:%s"
	// MsgNotificationProcessedFmt SSE 通知推送事件幂等去重 key
	MsgNotificationProcessedFmt = "msg:notification:processed:%s"

	// ---------- 分布式锁 ----------
	// LockPrefix 锁 key 前缀。锁 key = LockPrefix + 被保护的业务 key
	// （业务 key 已含全局前缀，锁本身不再叠加前缀，与既有线上 key 保持兼容）
	LockPrefix = "lock:"
)

// LockKeyFor 为业务缓存 key 生成配套的分布式锁 key。
// 例：LockKeyFor("v1:video:detail:id=5") => "lock:v1:video:detail:id=5"
func LockKeyFor(bizKey string) string {
	return LockPrefix + bizKey
}

// VideoDetailKey 视频详情缓存 key
func (c *Client) VideoDetailKey(id uint) string {
	return c.Key(VideoDetailFmt, id)
}

// VideoEntityKey feed 批量实体缓存 key
func (c *Client) VideoEntityKey(id uint) string {
	return c.Key(VideoEntityFmt, id)
}

// InvalidateVideoCache 失效某个视频的全部实体类缓存（详情 + feed 实体）。
// 视频内容、点赞计数、热度值发生任何变更后都应调用，
// 避免两个读路径返回相互不一致或滞后的旧数据。
func (c *Client) InvalidateVideoCache(ctx context.Context, id uint) {
	if c == nil || id == 0 {
		return
	}
	_ = c.Del(ctx, c.VideoDetailKey(id))
	_ = c.Del(ctx, c.VideoEntityKey(id))
}
