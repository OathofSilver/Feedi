package like

import (
	"time"
)

// Like 点赞数据表模型，记录用户点赞视频关系
//
//	videoid 和 accountid 联合唯一索引
type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`                                          // 自增主键ID
	VideoID   uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"video_id"`   // 被点赞视频ID，联合唯一索引字段，非空
	AccountID uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"account_id"` // 点赞用户账号ID，联合唯一索引字段，非空
	CreatedAt time.Time `json:"created_at"`                                                    // 点赞创建时间
}

type LikeRequest struct {
	VideoID uint `json:"video_id"`
}
