package notification

import "time"

// Type 通知类型
type Type string

const (
	TypeLike    Type = "like"    // 有人点赞了你的视频
	TypeFollow  Type = "follow"  // 有人关注了你
	TypeComment Type = "comment" // 有人评论了你的视频
)

// Notification 通知记录模型，对应数据表 notifications。
// 收件人 RecipientID 是核心查询维度；Actor 信息冗余存储避免读取时 join 账号表。
// 本轮不做已读/未读数，仅按时间倒序提供列表。
type Notification struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RecipientID uint      `gorm:"not null;index:idx_notif_recipient_time,sort:desc" json:"recipient_id"` // 收件人（被互动的作者/用户）
	ActorID     uint      `gorm:"not null" json:"actor_id"`                                              // 发起互动者账号ID
	ActorName   string    `gorm:"type:varchar(255)" json:"actor_name"`                                   // 发起互动者用户名（冗余）
	Type        Type      `gorm:"type:varchar(20);not null" json:"type"`                                 // like / follow / comment
	VideoID     uint      `json:"video_id,omitempty"`                                                    // like/comment 引用的视频ID；follow 为 0
	TargetID    uint      `json:"target_id,omitempty"`                                                   // follow 的被关注者ID(=recipient)；like/comment 为 0
	Content     string    `gorm:"type:text" json:"content,omitempty"`                                    // comment 的内容快照；like/follow 为空
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// ListRequest 通知列表查询请求；before_time 为上一页最后一条的 created_at 毫秒时间戳，0 表示首页
type ListRequest struct {
	BeforeTime int64 `json:"before_time"`
	Limit      int   `json:"limit"`
}

// ListItem 单条通知响应（CreatedAt 转毫秒）
type ListItem struct {
	ID        uint   `json:"id"`
	Type      Type   `json:"type"`
	ActorID   uint   `json:"actor_id"`
	ActorName string `json:"actor_name"`
	VideoID   uint   `json:"video_id,omitempty"`
	TargetID  uint   `json:"target_id,omitempty"`
	Content   string `json:"content,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// ListResponse 通知列表响应；next_before_time 供下一页游标，0 表示无更多
type ListResponse struct {
	Notifications  []ListItem `json:"notifications"`
	NextBeforeTime int64      `json:"next_before_time"`
}
