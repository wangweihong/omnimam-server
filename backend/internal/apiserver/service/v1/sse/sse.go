package sse

import (
	"context"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type CursorState string

const (
	CursorValid      CursorState = "valid"
	CursorNotVisible CursorState = "not_visible"
	CursorExpired    CursorState = "expired"
)

// Service 提供当前登录用户的短期事件历史、同步边界与 SSE 增量读取。
type Service interface {
	List(context.Context, *iapiserver.UserEventListRequest) (*iapiserver.UserEventListResponse, error)
	SyncState(context.Context) (*iapiserver.EventSyncState, error)
	ResolveCursor(context.Context, int64) (CursorState, *iapiserver.EventSyncState, error)
	ListAfter(context.Context, int64, int) ([]*iapiserver.UserEvent, error)
	CurrentUserID(context.Context) (string, error)
}

type service struct {
	store     store.UserEventStore
	retention time.Duration
}

func New(factory store.Factory, retention time.Duration) Service {
	return NewStore(factory.UserEvents(), retention)
}

// NewStore 使用消费方事件存储构造服务，便于网关测试和替换持久化实现。
func NewStore(eventStore store.UserEventStore, retention time.Duration) Service {
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	return &service{store: eventStore, retention: retention}
}

func (s *service) CurrentUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "", errors.NewStatus(code.ErrSSEPermissionDenied, "authenticated user is required")
	}
	return user.ID, nil
}

func (s *service) List(ctx context.Context, req *iapiserver.UserEventListRequest) (*iapiserver.UserEventListResponse, error) {
	userID, err := s.CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.AfterEventID > 0 {
		state, _, err := s.ResolveCursor(ctx, req.AfterEventID)
		if err != nil {
			return nil, err
		}
		switch state {
		case CursorExpired:
			return nil, errors.NewStatus(code.ErrSSECursorExpired, "event cursor expired")
		case CursorNotVisible:
			return nil, errors.NewStatus(code.ErrSSECursorNotVisible, "event cursor is not visible")
		}
	}
	req.RecipientUserID = userID
	items, total, err := s.store.List(ctx, req, time.Now())
	if err != nil {
		return nil, err
	}
	return &iapiserver.UserEventListResponse{Total: total, Items: items}, nil
}

func (s *service) SyncState(ctx context.Context) (*iapiserver.EventSyncState, error) {
	userID, err := s.CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	earliest, latest, err := s.store.SyncState(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	return &iapiserver.EventSyncState{LatestEventID: latest, EarliestAvailableEventID: earliest, RetentionSeconds: int64(s.retention / time.Second), ServerTime: imachinery.NewTime(now)}, nil
}

func (s *service) ResolveCursor(ctx context.Context, eventID int64) (CursorState, *iapiserver.EventSyncState, error) {
	state, err := s.SyncState(ctx)
	if err != nil {
		return "", nil, err
	}
	if eventID == 0 {
		return CursorValid, state, nil
	}
	userID, err := s.CurrentUserID(ctx)
	if err != nil {
		return "", nil, err
	}
	visible, expired, err := s.store.CursorVisible(ctx, userID, eventID)
	if err != nil {
		return "", nil, err
	}
	if expired {
		return CursorExpired, state, nil
	}
	if !visible {
		if state.EarliestAvailableEventID > 0 && eventID < state.EarliestAvailableEventID {
			return CursorExpired, state, nil
		}
		return CursorNotVisible, state, nil
	}
	return CursorValid, state, nil
}

func (s *service) ListAfter(ctx context.Context, after int64, limit int) ([]*iapiserver.UserEvent, error) {
	userID, err := s.CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.ListAfter(ctx, userID, after, limit, time.Now())
}
