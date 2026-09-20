package notification

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidParam = errors.New("请求参数错误")
	ErrNotFound     = errors.New("通知不存在")
)

// Pusher 投递"通知就绪"事件驱动实时推送（尽力而为）。
// 由 api 进程持有：写库成功后调用，失败不影响落库。
type Pusher interface {
	Push(ctx context.Context, notificationID, recipientID uint) error
}

// AccountResolver 按账号ID解析用户名（冗余 actorName 用，避免列表 join 账号表）。
type AccountResolver interface {
	UsernameByID(ctx context.Context, accountID uint) (string, error)
}

// Service 通知业务服务：负责组装并写入通知 + 触发实时推送 + 列表查询。
type Service struct {
	repo   Repository
	pusher Pusher
	acct   AccountResolver
}

// NewService 创建通知服务；pusher/acct 可为 nil（如关闭实时推送或无需冗余用户名），写库不受影响。
func NewService(repo Repository, pusher Pusher, acct AccountResolver) *Service {
	return &Service{repo: repo, pusher: pusher, acct: acct}
}

// Notify 组装并落库一条通知，成功后触发实时推送。
// recipientID 收件人、actorID 发起者、t 类型；extra 由调用方按类型填充(VideoID/TargetID/Content)。
func (s *Service) Notify(ctx context.Context, recipientID, actorID uint, t Type, extra func(*Notification)) error {
	if recipientID == 0 || actorID == 0 {
		return ErrInvalidParam
	}

	n := &Notification{
		RecipientID: recipientID,
		ActorID:     actorID,
		Type:        t,
	}
	if s.acct != nil {
		if name, err := s.acct.UsernameByID(ctx, actorID); err == nil {
			n.ActorName = name
		}
	}
	if extra != nil {
		extra(n)
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return err
	}

	// 实时推送尽力而为，失败只记日志不阻断业务写库
	if s.pusher != nil {
		_ = s.pusher.Push(context.WithoutCancel(ctx), n.ID, n.RecipientID)
	}
	return nil
}

// ListByRecipient 游标分页查询某用户的倒序通知列表。
// 返回列表与"下一页游标"（末条 CreatedAt 毫秒时间戳，无更多则为 0）。
// 游标单位统一为毫秒，与 feed/latest 及 Redis 时间线保持一致。
func (s *Service) ListByRecipient(ctx context.Context, recipientID uint, beforeTime int64, limit int) (ListResponse, error) {
	var before time.Time
	if beforeTime > 0 {
		before = time.UnixMilli(beforeTime)
	}

	items, err := s.repo.ListByRecipient(ctx, recipientID, before, limit)
	if err != nil {
		return ListResponse{}, err
	}

	out := ListResponse{Notifications: []ListItem{}}
	for _, it := range items {
		out.Notifications = append(out.Notifications, ListItem{
			ID:        it.ID,
			Type:      it.Type,
			ActorID:   it.ActorID,
			ActorName: it.ActorName,
			VideoID:   it.VideoID,
			TargetID:  it.TargetID,
			Content:   it.Content,
			CreatedAt: it.CreatedAt.UnixMilli(),
		})
	}
	if len(items) > 0 {
		out.NextBeforeTime = items[len(items)-1].CreatedAt.UnixMilli()
	}
	return out, nil
}
