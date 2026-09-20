package social

import (
	"context"
	"errors"
	"feed/backend/internal/account"
	"feed/backend/internal/notification"
	rediscache "feed/backend/internal/utils/redis"
	"log"
	"time"
)

var (
	ErrInvalidParam    = errors.New("请求参数错误")
	ErrAccountNotFound = errors.New("账号不存在")
	ErrSelfFollow      = errors.New("不能关注自己")
	ErrAlreadyFollowed = errors.New("已关注该用户")
	ErrNotFollowed     = errors.New("尚未关注该用户")
)

// listCacheTTL 关注/粉丝列表缓存有效期
const listCacheTTL = 5 * time.Minute

type SocialService struct {
	repo        SocialRepositoryer
	accountrepo account.AccountRepositoryer
	cache       *rediscache.Client
	notifier    *notification.Service // 通知服务：被关注后通知对方（可空）
}

func NewSocialService(repo SocialRepositoryer, accountrepo account.AccountRepositoryer, cache *rediscache.Client, notifier *notification.Service) *SocialService {
	return &SocialService{repo: repo, accountrepo: accountrepo, cache: cache, notifier: notifier}
}

func (s *SocialService) Follow(ctx context.Context, social *Social) error {
	_, err := s.accountrepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return err
	}
	_, err = s.accountrepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	if social.FollowerID == social.VloggerID {
		return errors.New("can not follow self")
	}
	isFollowed, err := s.repo.IsFollowed(ctx, social.FollowerID, social.VloggerID)
	if err != nil {
		return err
	}
	if isFollowed {
		return ErrAlreadyFollowed
	}
	// 先写 DB，确保数据持久化
	if err := s.repo.Follow(ctx, social.FollowerID, social.VloggerID); err != nil {
		return err
	}
	// DB 成功后，失效该用户的关注列表缓存
	s.invalidateFollowingFeedCache(context.Background(), social.FollowerID)
	// 关注流（ListByFollowing）直接查 DB 并按粉丝实时可见，无需通过 MQ 做时间线写扩散

	// 通知被关注者"被关注"（自关注已在前面拦截，此处仍防御性过滤）
	if s.notifier != nil && social.VloggerID != social.FollowerID {
		if err := s.notifier.Notify(ctx, social.VloggerID, social.FollowerID, notification.TypeFollow, func(n *notification.Notification) {
			n.TargetID = social.VloggerID
		}); err != nil {
			log.Printf("通知被关注失败: recipient=%d, actor=%d, err=%v", social.VloggerID, social.FollowerID, err)
		}
	}

	return nil
}

func (s *SocialService) Unfollow(ctx context.Context, social *Social) error {
	_, err := s.accountrepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return err
	}
	_, err = s.accountrepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	isFollowed, err := s.repo.IsFollowed(ctx, social.FollowerID, social.VloggerID)
	if err != nil {
		return err
	}
	if !isFollowed {
		return ErrNotFollowed
	}

	// 先写 DB
	if err := s.repo.Unfollow(ctx, social.FollowerID, social.VloggerID); err != nil {
		return err
	}
	// 失效缓存
	s.invalidateFollowingFeedCache(context.Background(), social.FollowerID)

	return nil
}

func (s *SocialService) invalidateFollowingFeedCache(ctx context.Context, accountID uint) {
	if s.cache == nil {
		return
	}
	// pattern 模板统一引自 rediscache/keys.go，与 feed 侧写 key 格式编译期同源
	pattern := s.cache.Key(rediscache.FeedFollowingPatternFmt, accountID)
	if err := s.cache.DelByPattern(ctx, pattern); err != nil {
		log.Printf("失效 Following 缓存失败: accountID=%d, err=%v", accountID, err)
	}
}

func (s *SocialService) GetAllFollowers(ctx context.Context, VloggerID uint) ([]*account.Account, error) {
	_, err := s.accountrepo.FindByID(ctx, VloggerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetAllFollowers(ctx, VloggerID)
}

func (s *SocialService) GetAllVloggers(ctx context.Context, FollowerID uint) ([]*account.Account, error) {
	_, err := s.accountrepo.FindByID(ctx, FollowerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetAllVloggers(ctx, FollowerID)
}

func (s *SocialService) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	return s.repo.CountFollowers(ctx, vloggerID)
}

func (s *SocialService) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	return s.repo.CountVloggers(ctx, followerID)
}

func (s *SocialService) IsFollowed(ctx context.Context, social *Social) (bool, error) {
	_, err := s.accountrepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return false, err
	}
	_, err = s.accountrepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return false, err
	}
	return s.repo.IsFollowed(ctx, social.FollowerID, social.VloggerID)
}
