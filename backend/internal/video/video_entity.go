package video

import (
	"time"
)

// Video 短视频数据库模型，对应数据表 videos
type Video struct {
	ID          uint      `gorm:"primaryKey" json:"id"`                            // 视频唯一主键ID
	AuthorID    uint      `gorm:"index;not null" json:"author_id"`                 // 作者用户ID
	Username    string    `gorm:"type:varchar(255);not null" json:"username"`      // 作者用户名
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`         // 视频标题
	Description string    `gorm:"type:varchar(255);" json:"description,omitempty"` // 视频简介描述，允许为空
	PlayURL     string    `gorm:"type:varchar(255);not null" json:"play_url"`      // 视频播放地址
	CoverURL    string    `gorm:"type:varchar(255);not null" json:"cover_url"`     // 视频封面图地址
	CreateTime  time.Time `gorm:"autoCreateTime;index:idx_videos_create_time,sort:desc;index:idx_videos_popularity_time_id,priority:2,sort:desc" json:"create_time"`
	// 创建时间；自动填充；参与两个复合索引
	LikesCount int64 `gorm:"column:likes_count;not null;default:0;index:idx_videos_likes_count_id,priority:1,sort:desc" json:"likes_count"`
	// 点赞总数，默认0；用于按点赞排序索引
	Popularity int64 `gorm:"column:popularity;not null;default:0;index:idx_videos_popularity_time_id,priority:1,sort:desc" json:"popularity"`
	// 热度分值，推荐算法排序核心字段
}

// PublishVideoRequest 发布短视频 HTTP 请求结构体
type PublishVideoRequest struct {
	Title       string `json:"title"`       // 视频标题
	Description string `json:"description"` // 视频简介
	PlayURL     string `json:"play_url"`    // 上传完成后的视频资源地址
	CoverURL    string `json:"cover_url"`   // 视频封面图片地址
}

type DeleteVideoRequest struct {
	ID uint `json:"id"`
}

type ListByAuthorIDRequest struct {
	AuthorID uint `json:"author_id"`
}

type GetDetailRequest struct {
	ID uint `json:"id"`
}

type UpdateLikesCountRequest struct {
	ID         uint  `json:"id"`
	LikesCount int64 `json:"likes_count"`
}

// UpdatePopularityRequest 更新视频热度值 HTTP 请求结构体
type UpdatePopularityRequest struct {
	ID         uint  `json:"id"`
	Popularity int64 `json:"popularity"`
}
