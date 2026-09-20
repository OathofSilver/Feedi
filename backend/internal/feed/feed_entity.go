package feed

import "time"

// FeedAuthor 视频作者信息
type FeedAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// FeedVideoItem Feed流视频条目
type FeedVideoItem struct {
	ID          uint       `json:"id"`
	Author      FeedAuthor `json:"author"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	PlayURL     string     `json:"play_url"`
	CoverURL    string     `json:"cover_url"`
	CreateTime  int64      `json:"create_time"`
	LikesCount  int64      `json:"likes_count"`
	IsLiked     bool       `json:"is_liked"`
}

// ListLatestRequest 最新视频列表请求
type ListLatestRequest struct {
	Limit      int   `json:"limit"`
	LatestTime int64 `json:"latest_time"`
}

// ListLatestResponse 最新视频列表响应
type ListLatestResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	NextTime  int64           `json:"next_time"`
	HasMore   bool            `json:"has_more"`
}

// ListLikesCountRequest 按点赞数热门视频请求
type ListLikesCountRequest struct {
	Limit            int    `json:"limit"`
	LikesCountBefore *int64 `json:"likes_count_before,omitempty"`
	IDBefore         *uint  `json:"id_before,omitempty"`
}

// LikesCountCursor 点赞分页游标
type LikesCountCursor struct {
	LikesCount int64
	ID         uint
}

// ListLikesCountResponse 按点赞数热门视频响应
type ListLikesCountResponse struct {
	VideoList            []FeedVideoItem `json:"video_list"`
	NextLikesCountBefore *int64          `json:"next_likes_count_before,omitempty"`
	NextIDBefore         *uint           `json:"next_id_before,omitempty"`
	HasMore              bool            `json:"has_more"`
}

// ListByFollowingRequest 关注流视频请求
type ListByFollowingRequest struct {
	Limit      int   `json:"limit"`
	LatestTime int64 `json:"latest_time"`
}

// ListByFollowingResponse 关注流视频响应
type ListByFollowingResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	NextTime  int64           `json:"next_time"`
	HasMore   bool            `json:"has_more"`
}

// ListByPopularityRequest 热度推荐视频请求
type ListByPopularityRequest struct {
	Limit            int       `json:"limit"`
	AsOf             int64     `json:"as_of"`
	Offset           int       `json:"offset"`
	LatestIDBefore   *uint     `json:"latest_id_before,omitempty"`
	LatestPopularity int64     `json:"latest_popularity"`
	LatestBefore     time.Time `json:"latest_before"`
}

// ListByPopularityResponse 热度推荐视频响应
type ListByPopularityResponse struct {
	VideoList            []FeedVideoItem `json:"video_list"`
	AsOf                 int64           `json:"as_of"`
	NextOffset           int             `json:"next_offset"`
	HasMore              bool            `json:"has_more"`
	NextLatestPopularity *int64          `json:"next_latest_popularity,omitempty"`
	NextLatestBefore     *time.Time      `json:"next_latest_before,omitempty"`
	NextLatestIDBefore   *uint           `json:"next_latest_id_before,omitempty"`
}
