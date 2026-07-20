package sse

import (
	"context"
	"testing"
	"time"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type fakeUserEventStore struct {
	visible        bool
	expired        bool
	earliest       int64
	latest         int64
	listRecipient  string
	afterRecipient string
	items          []*iapiserver.UserEvent
}

func (s *fakeUserEventStore) AddIdempotent(context.Context, *iapiserver.UserEvent) (*iapiserver.UserEvent, bool, error) {
	return nil, false, nil
}
func (s *fakeUserEventStore) List(_ context.Context, req *iapiserver.UserEventListRequest, _ time.Time) ([]*iapiserver.UserEvent, int64, error) {
	s.listRecipient = req.RecipientUserID
	return s.items, int64(len(s.items)), nil
}
func (s *fakeUserEventStore) ListAfter(_ context.Context, recipient string, _ int64, _ int, _ time.Time) ([]*iapiserver.UserEvent, error) {
	s.afterRecipient = recipient
	return s.items, nil
}
func (s *fakeUserEventStore) CursorVisible(context.Context, string, int64) (bool, bool, error) {
	return s.visible, s.expired, nil
}
func (s *fakeUserEventStore) SyncState(context.Context, string, time.Time) (int64, int64, error) {
	return s.earliest, s.latest, nil
}
func (s *fakeUserEventStore) PruneExpired(context.Context, time.Time) (int64, error) { return 0, nil }

func userContext(id string) context.Context {
	user := &iapiserver.User{}
	user.ID = id
	return context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
}

func TestServiceScopesHistoryAndStreamReadsToCurrentUser(t *testing.T) {
	store := &fakeUserEventStore{visible: true, items: []*iapiserver.UserEvent{{EventSequence: 8}}}
	service := NewStore(store, 24*time.Hour)
	ctx := userContext("user-1")

	result, err := service.List(ctx, &iapiserver.UserEventListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if result.Total != 1 || store.listRecipient != "user-1" {
		t.Fatalf("List() result=%+v recipient=%q", result, store.listRecipient)
	}
	if _, err := service.ListAfter(ctx, 7, 20); err != nil {
		t.Fatalf("ListAfter() error = %v", err)
	}
	if store.afterRecipient != "user-1" {
		t.Fatalf("ListAfter() recipient = %q", store.afterRecipient)
	}
}

func TestServiceResolveCursor(t *testing.T) {
	tests := []struct {
		name    string
		store   fakeUserEventStore
		eventID int64
		want    CursorState
	}{
		{name: "visible", store: fakeUserEventStore{visible: true, earliest: 5, latest: 10}, eventID: 7, want: CursorValid},
		{name: "expired row", store: fakeUserEventStore{visible: true, expired: true, earliest: 8, latest: 10}, eventID: 7, want: CursorExpired},
		{name: "pruned cursor", store: fakeUserEventStore{earliest: 8, latest: 10}, eventID: 7, want: CursorExpired},
		{name: "not visible", store: fakeUserEventStore{earliest: 5, latest: 10}, eventID: 9, want: CursorNotVisible},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewStore(&test.store, time.Hour)
			got, _, err := service.ResolveCursor(userContext("user-1"), test.eventID)
			if err != nil {
				t.Fatalf("ResolveCursor() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ResolveCursor() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestServiceHistoryReturnsContractCursorError(t *testing.T) {
	store := &fakeUserEventStore{earliest: 8, latest: 10}
	service := NewStore(store, time.Hour)
	_, err := service.List(userContext("user-1"), &iapiserver.UserEventListRequest{AfterEventID: 7})
	if err == nil {
		t.Fatal("List() error = nil")
	}
	if got := toolerrors.ToStatus(err).Code; got != code.ErrSSECursorExpired {
		t.Fatalf("List() code = %d, want %d", got, code.ErrSSECursorExpired)
	}
}
