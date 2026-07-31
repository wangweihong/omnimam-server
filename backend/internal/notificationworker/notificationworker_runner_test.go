package notificationworker

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type notificationWorkerCandidateStore struct {
	store.NotificationCandidateStore
}

func (notificationWorkerCandidateStore) ClaimNotificationCandidates(
	context.Context,
	time.Time,
	int,
	time.Duration,
) ([]*iapiserver.NotificationEvent, error) {
	return nil, nil
}

type notificationWorkerOutboxStore struct {
	store.NotificationOutboxStore
}

type notificationWorkerRetentionStore struct {
	store.NotificationRetentionStore
}

type notificationWorkerUserEventStore struct {
	store.UserEventStore
}

type notificationWorkerStore struct {
	store.Factory
	candidates store.NotificationCandidateStore
	outbox     store.NotificationOutboxStore
	retention  store.NotificationRetentionStore
	events     store.UserEventStore
}

func (s notificationWorkerStore) NotificationCandidates() store.NotificationCandidateStore {
	return s.candidates
}

func (s notificationWorkerStore) NotificationOutbox() store.NotificationOutboxStore {
	return s.outbox
}

func (s notificationWorkerStore) NotificationRetention() store.NotificationRetentionStore {
	return s.retention
}

func (s notificationWorkerStore) UserEvents() store.UserEventStore {
	return s.events
}

func completeNotificationWorkerStore() notificationWorkerStore {
	return notificationWorkerStore{
		candidates: notificationWorkerCandidateStore{},
		outbox:     notificationWorkerOutboxStore{},
		retention:  notificationWorkerRetentionStore{},
		events:     notificationWorkerUserEventStore{},
	}
}

func TestRunNotificationWorkerRejectsMissingDependencies(t *testing.T) {
	tests := []struct {
		name  string
		store store.Factory
		sub   func(context.Context, string, string) (<-chan *message.Message, error)
		want  string
	}{
		{name: "store", want: "notification worker store is not configured"},
		{
			name:  "candidate store",
			store: notificationWorkerStore{},
			want:  "notification candidate store is not configured",
		},
		{
			name: "outbox store",
			store: notificationWorkerStore{
				candidates: notificationWorkerCandidateStore{},
			},
			want: "notification outbox store is not configured",
		},
		{
			name: "retention store",
			store: notificationWorkerStore{
				candidates: notificationWorkerCandidateStore{},
				outbox:     notificationWorkerOutboxStore{},
			},
			want: "notification retention store is not configured",
		},
		{
			name: "user event store",
			store: notificationWorkerStore{
				candidates: notificationWorkerCandidateStore{},
				outbox:     notificationWorkerOutboxStore{},
				retention:  notificationWorkerRetentionStore{},
			},
			want: "notification user event store is not configured",
		},
		{
			name:  "subscriber",
			store: completeNotificationWorkerStore(),
			want:  "notification worker subscriber is not configured",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runNotificationWorker(context.Background(), tt.store, time.Hour, tt.sub)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("runNotificationWorker() error=%v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRunNotificationWorkerCleansUpAfterSubscriptionFailure(t *testing.T) {
	const failAt = 5
	contexts := make([]context.Context, 0, failAt)
	calls := 0
	subscribe := func(ctx context.Context, _, _ string) (<-chan *message.Message, error) {
		calls++
		contexts = append(contexts, ctx)
		if calls == failAt {
			return nil, stderrors.New("subscription unavailable")
		}
		return make(chan *message.Message), nil
	}

	err := runNotificationWorker(
		context.Background(),
		completeNotificationWorkerStore(),
		time.Hour,
		subscribe,
	)
	if err == nil || !strings.Contains(err.Error(), "start notification source worker") {
		t.Fatalf("runNotificationWorker() error=%v", err)
	}
	if calls != failAt {
		t.Fatalf("subscription calls=%d, want %d", calls, failAt)
	}
	for index, ctx := range contexts {
		select {
		case <-ctx.Done():
		default:
			t.Fatalf("subscription context %d was not canceled", index)
		}
	}
}

func TestRunNotificationWorkerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 6)
	subscribe := func(context.Context, string, string) (<-chan *message.Message, error) {
		started <- struct{}{}
		return make(chan *message.Message), nil
	}
	result := make(chan error, 1)
	go func() {
		result <- runNotificationWorker(
			ctx,
			completeNotificationWorkerStore(),
			time.Hour,
			subscribe,
		)
	}()

	for range 6 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("notification worker subscriptions did not start")
		}
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("runNotificationWorker() error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("notification worker did not stop after cancellation")
	}
}
