package feed

import (
	"context"
	"feed/backend/internal/social"
	"feed/backend/internal/video"
	"gorm.io/gorm"
	"time"
)

type FeedRepositoryer interface {
	// ListLatest 按创建时间倒序查询最新视频，时间游标分页
	ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*video.Video, error)
	// ListLikesCountWithCursor 按点赞数降序分页查询，复合游标(点赞数+ID)，解决点赞数相同时分页重复漏数据
	ListLikesCountWithCursor(ctx context.Context, limit int, cursor *LikesCountCursor) ([]*video.Video, error)
	// ListByFollowing 查询当前用户关注作者发布的视频，时间游标分页
	ListByFollowing(ctx context.Context, limit int, viewerAccountID uint, latestBefore time.Time) ([]*video.Video, error)
	// ListByPopularity 按综合热度排序，三元复合游标(热度+创建时间+ID)，用于推荐流DB降级查询
	ListByPopularity(ctx context.Context, limit int, popularityBefore int64, timeBefore time.Time, idBefore uint) ([]*video.Video, error)
	// GetByIDs 根据视频ID集合批量查询视频
	GetByIDs(ctx context.Context, ids []uint) ([]*video.Video, error)
	// ListByTag 根据标签名称查询对应视频，多表关联查询标签关联的视频
	ListByTag(ctx context.Context, tagName string, limit int) ([]*video.Video, error)
}

type FeedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) *FeedRepository {
	return &FeedRepository{db: db}
}

func (repo *FeedRepository) ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*video.Video, error) {
	var videos []*video.Video
	// 构建基础查询，按创建时间倒序，新创建的视频排在前面
	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Order("create_time DESC")

	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}

	// 执行查询，限制返回记录条数
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListLikesCountWithCursor 按点赞数降序分页查询视频列表，支持游标分页。
func (repo *FeedRepository) ListLikesCountWithCursor(ctx context.Context, limit int, cursor *LikesCountCursor) ([]*video.Video, error) {
	var videos []*video.Video
	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Order("likes_count DESC, id DESC")
	if cursor != nil {
		query = query.Where(
			"(likes_count < ?) OR (likes_count = ? AND id < ?)",
			cursor.LikesCount,
			cursor.LikesCount, cursor.ID,
		)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListByFollowing 查询当前用户关注的创作者发布的视频列表，按创建时间降序排列。
func (repo *FeedRepository) ListByFollowing(ctx context.Context, limit int, viewerAccountID uint, latestBefore time.Time) ([]*video.Video, error) {
	var videos []*video.Video
	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Order("create_time DESC")
	if viewerAccountID > 0 {
		followingSubQuery := repo.db.WithContext(ctx).
			Model(&social.Social{}).
			Select("vlogger_id").
			Where("follower_id = ?", viewerAccountID)
		query = query.Where("author_id IN (?)", followingSubQuery)
	}
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListByPopularity 按热度综合排序分页查询视频列表，支持多字段游标分页。
func (repo *FeedRepository) ListByPopularity(ctx context.Context, limit int, popularityBefore int64, timeBefore time.Time, idBefore uint) ([]*video.Video, error) {
	var videos []*video.Video
	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Order("popularity DESC, create_time DESC, id DESC")

	// 只有当游标完整提供时才加过滤（popularity 允许为 0）
	if !timeBefore.IsZero() && idBefore > 0 {
		query = query.Where(
			"(popularity < ?) OR (popularity = ? AND create_time < ?) OR (popularity = ? AND create_time = ? AND id < ?)",
			popularityBefore,
			popularityBefore, timeBefore,
			popularityBefore, timeBefore, idBefore,
		)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// GetByIDs 根据视频ID批量查询视频信息
func (repo *FeedRepository) GetByIDs(ctx context.Context, ids []uint) ([]*video.Video, error) {
	var videos []*video.Video
	// 入参校验：id数组为空，直接返回空结果，避免生成无效SQL
	if len(ids) == 0 {
		return videos, nil
	}
	// 根据id in条件批量查询视频基础数据
	if err := repo.db.WithContext(ctx).Model(&video.Video{}).
		Where("id IN ?", ids).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByTag(ctx context.Context, tagName string, limit int) ([]*video.Video, error) {
	var videos []*video.Video
	err := repo.db.WithContext(ctx).Model(&video.Video{}).Table("videos").
		// 视频 和 视频标签中间表 关联（多对多关系）
		Joins("JOIN video_tags ON video_tags.video_id = videos.id").
		// 中间表 和 标签表关联
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		// 匹配传入的标签名称
		Where("tags.name = ?", tagName).
		// 按视频创建时间倒序，最新优先
		Order("videos.create_time desc").
		Limit(limit).
		Find(&videos).Error
	return videos, err
}
