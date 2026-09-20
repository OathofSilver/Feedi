package like

import (
	"context"
	"errors"
	"feed/backend/internal/video"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type LikeRepositoryer interface {
	// Like 新增点赞记录，重复点赞会返回mysql 1062冲突错误
	Like(ctx context.Context, like *Like) error

	// Unlike 根据结构体条件物理删除点赞记录
	Unlike(ctx context.Context, like *Like) error

	// LikeIgnoreDuplicate 新增点赞，自动忽略重复点赞冲突；
	// 返回值 created:true=本次真正插入；false=已存在/未插入
	LikeIgnoreDuplicate(ctx context.Context, like *Like) (created bool, err error)

	// DeleteByVideoAndAccount 根据videoID+accountID删除点赞
	// deleted:true代表确实有记录被删除
	DeleteByVideoAndAccount(ctx context.Context, videoID, accountID uint) (deleted bool, err error)

	// IsLiked 判断某个用户是否点赞该视频
	IsLiked(ctx context.Context, videoID, accountID uint) (bool, error)

	// BatchGetLiked 批量查询一个用户对一批视频的点赞状态
	// 返回map key:videoID，value:true代表已点赞
	BatchGetLiked(ctx context.Context, videoIDs []uint, accountID uint) (map[uint]bool, error)

	// ListLikedVideos 查询用户点赞过的视频列表，按点赞时间倒序，最多返回200条
	ListLikedVideos(ctx context.Context, accountID uint) ([]video.Video, error)
}

type LikeRepository struct {
	db *gorm.DB
}

func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

func (r *LikeRepository) Like(ctx context.Context, like *Like) error {
	return r.db.WithContext(ctx).Create(like).Error
}

func (r *LikeRepository) Unlike(ctx context.Context, like *Like) error {
	return r.db.WithContext(ctx).
		Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).
		Delete(&Like{}).Error
}

// LikeIgnoreDuplicate 新增点赞记录，忽略重复点赞（唯一索引冲突1062）
func (r *LikeRepository) LikeIgnoreDuplicate(ctx context.Context, like *Like) (created bool, err error) {
	// 参数校验：对象为空，视频ID或用户ID为0，直接返回不处理
	if like == nil || like.VideoID == 0 || like.AccountID == 0 {
		return false, nil
	}

	// GORM执行插入点赞记录
	err = r.db.WithContext(ctx).Create(like).Error
	// 插入成功，返回true代表新增点赞
	if err == nil {
		return true, nil
	}

	// 捕获MySQL唯一索引冲突错误码 1062，代表用户已经对该视频点过赞
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return false, nil
	}

	// 其他数据库错误，向上抛出
	return false, err
}

func (r *LikeRepository) DeleteByVideoAndAccount(ctx context.Context, videoID, accountID uint) (deleted bool, err error) {
	if videoID == 0 || accountID == 0 {
		return false, nil
	}
	res := r.db.WithContext(ctx).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Delete(&Like{})
	return res.RowsAffected > 0, res.Error
}

func (r *LikeRepository) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Like{}).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// BatchGetLiked 批量查询指定账户对多个视频的点赞状态
func (r *LikeRepository) BatchGetLiked(ctx context.Context, videoIDs []uint, accountID uint) (map[uint]bool, error) {
	likeMap := make(map[uint]bool)
	if len(videoIDs) == 0 {
		return likeMap, nil
	}
	if accountID == 0 {
		return likeMap, nil
	}
	var likes []Like
	err := r.db.WithContext(ctx).Model(&Like{}).
		Where("video_id IN ? AND account_id = ?", videoIDs, accountID).
		Find(&likes).Error
	if err != nil {
		return nil, err
	}
	for _, like := range likes {
		likeMap[like.VideoID] = true
	}
	return likeMap, nil
}

func (r *LikeRepository) ListLikedVideos(ctx context.Context, accountID uint) ([]video.Video, error) {
	var videos []video.Video
	if accountID == 0 {
		return videos, nil
	}
	err := r.db.WithContext(ctx).
		Model(&video.Video{}).
		Joins("JOIN likes ON likes.video_id = videos.id").
		Where("likes.account_id = ?", accountID).
		Order("likes.created_at desc").
		Limit(200).
		Find(&videos).Error
	if err != nil {
		return nil, err
	}
	return videos, nil
}
