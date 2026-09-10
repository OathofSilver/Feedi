package social

import "feed/backend/internal/account"

// uniqueIndex:idx_social_follower_vlogger: 复合唯一索引的第二列，防止重复关注
type Social struct {
	ID         uint `gorm:"primaryKey"`
	FollowerID uint `gorm:"not null;index:idx_social_follower;uniqueIndex:idx_social_follower_vlogger"`
	VloggerID  uint `gorm:"not null;index:idx_social_vlogger;uniqueIndex:idx_social_follower_vlogger"`
}

type FollowRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

type UnfollowRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

type GetAllFollowersRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

type GetAllFollowersResponse struct {
	Followers     []*account.Account `json:"followers"`
	FollowerCount int64              `json:"follower_count"`
}

type GetAllVloggersResponse struct {
	Vloggers     []*account.Account `json:"vloggers"`
	VloggerCount int64              `json:"vlogger_count"`
}

type SocialCounts struct {
	FollowerCount int64 `json:"follower_count"`
	VloggerCount  int64 `json:"vlogger_count"`
}

// SocialCountsRequest counts 查询条件：user_id 缺省时按当前登录者统计
type SocialCountsRequest struct {
	UserID uint `json:"user_id"`
}

// IsFollowedRequest 查询当前登录者是否已关注 vlogger_id
type IsFollowedRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

// IsFollowedResponse 关注关系查询结果
type IsFollowedResponse struct {
	IsFollowing bool `json:"is_following"`
}

type GetAllVloggersRequest struct {
	FollowerID uint `json:"follower_id"`
}
