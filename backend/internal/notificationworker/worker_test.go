package notificationworker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type candidateFinish struct {
	status, code, summary string
	attempt               int
	next                  time.Time
}

type fakeCandidateStore struct {
	mu             sync.Mutex
	addErr         error
	added          int
	claims         []*iapiserver.NotificationEvent
	claimLease     time.Duration
	finishes       []candidateFinish
	materialized   *iapiserver.Notification
	materializeErr error
	finishErr      error
}

func (s *fakeCandidateStore) AddNotificationCandidates(_ context.Context, items []*iapiserver.NotificationEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.added += len(items)
	return s.addErr
}
func (s *fakeCandidateStore) ClaimNotificationCandidates(_ context.Context, _ time.Time, _ int, lease time.Duration) ([]*iapiserver.NotificationEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimLease = lease
	items := s.claims
	s.claims = nil
	return items, nil
}
func (s *fakeCandidateStore) FinishNotificationCandidate(_ context.Context, _ string, attempt int, status string, next time.Time, code, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishes = append(s.finishes, candidateFinish{status: status, code: code, summary: summary, attempt: attempt, next: next})
	return s.finishErr
}
func (s *fakeCandidateStore) MaterializeNotification(context.Context, *iapiserver.NotificationEvent, store.NotificationMaterialization) (*iapiserver.Notification, bool, error) {
	return s.materialized, false, s.materializeErr
}

func TestSourceWorkerAckAndNackBoundaries(t *testing.T) {
	registry, err := BuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeCandidateStore{}
	channels := map[string]chan *message.Message{}
	subscribe := func(_ context.Context, topic, _ string) (<-chan *message.Message, error) {
		channels[topic] = make(chan *message.Message, 4)
		return channels[topic], nil
	}
	worker := NewSourceWorker(fake, registry, subscribe)
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer worker.Close()

	valid := message.NewMessage("valid", []byte(`{"source_domain":"task-center","source_event_id":"event-1","atomic_task_id":"task-1","to_status":"FAILED","resource_version":2,"created_by":"user-1","occurred_at":"2026-07-29T01:00:00Z","last_error":{"message":"failed"}}`))
	channels[atomicTaskSourceTopic] <- valid
	select {
	case <-valid.Acked():
	case <-time.After(time.Second):
		t.Fatal("persisted candidate was not ACKed")
	}

	ignored := message.NewMessage("ignored", []byte(`{"source_domain":"task-center","source_event_id":"event-2","atomic_task_id":"task-1","to_status":"RUNNING","resource_version":3,"created_by":"user-1","occurred_at":"2026-07-29T01:00:00Z"}`))
	channels[atomicTaskSourceTopic] <- ignored
	select {
	case <-ignored.Acked():
	case <-time.After(time.Second):
		t.Fatal("non-target status was not ACKed")
	}

	invalid := message.NewMessage("invalid", []byte(`{"source_domain":"task-center","atomic_task_id":"task-1"}`))
	channels[atomicTaskSourceTopic] <- invalid
	select {
	case <-invalid.Nacked():
	case <-time.After(time.Second):
		t.Fatal("invalid source event was not NACKed")
	}

	fake.mu.Lock()
	fake.addErr = errors.New("store unavailable")
	fake.mu.Unlock()
	storeFailure := message.NewMessage("store-failure", valid.Payload)
	channels[atomicTaskSourceTopic] <- storeFailure
	select {
	case <-storeFailure.Nacked():
	case <-time.After(time.Second):
		t.Fatal("candidate store failure was not NACKed")
	}
}

type failingRule struct{}

func (failingRule) Evaluate(context.Context, *iapiserver.NotificationEvent) (*RuleDecision, error) {
	return nil, errors.New("recipient unavailable")
}

func TestRuleWorkerRetryDeadLetterAndGracefulClose(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterRule("task.atomic_task.failed", failingRule{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	retry := &iapiserver.NotificationEvent{NotificationTopic: "task.atomic_task.failed", ProcessingAttemptCount: 2}
	retry.ID = "retry"
	dead := retry.DeepCopy()
	dead.ID, dead.ProcessingAttemptCount = "dead", 10
	fake := &fakeCandidateStore{claims: []*iapiserver.NotificationEvent{retry, dead}}
	worker := NewRuleWorker(fake, registry, RuleWorkerConfig{
		PollInterval: time.Millisecond, BatchSize: 100, ClaimLease: 30 * time.Second,
		MaxAttempts: 10, InitialBackoff: time.Second, MaxBackoff: 5 * time.Minute,
	})
	if err := worker.poll(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if fake.claimLease != 30*time.Second || len(fake.finishes) != 2 {
		t.Fatalf("lease=%s finishes=%+v", fake.claimLease, fake.finishes)
	}
	if fake.finishes[0].status != iapiserver.NotificationEventFailed || fake.finishes[0].next.Sub(now) != 2*time.Second {
		t.Fatalf("retry finish=%+v", fake.finishes[0])
	}
	if fake.finishes[0].attempt != 2 {
		t.Fatalf("retry attempt=%d, want 2", fake.finishes[0].attempt)
	}
	if fake.finishes[1].status != iapiserver.NotificationEventDeadLetter || !fake.finishes[1].next.IsZero() {
		t.Fatalf("dead-letter finish=%+v", fake.finishes[1])
	}

	running := NewRuleWorker(&fakeCandidateStore{}, registry, RuleWorkerConfig{PollInterval: time.Millisecond})
	if err := running.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		running.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RuleWorker.Close did not stop polling")
	}
}

func TestRuleWorkerDeadLettersUnresolvedRecipient(t *testing.T) {
	registry, err := BuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	item := &iapiserver.NotificationEvent{
		NotificationTopic:      "task.atomic_task.failed",
		SourceType:             "atomic_task",
		SourceID:               "task-1",
		ProcessingAttemptCount: 10,
		PayloadSnapshot: iapiserver.NotificationPayloadSnapshot{
			Status: iapiserver.AtomicTaskStatusFailed,
		},
	}
	item.ID = "unresolved-recipient"
	fake := &fakeCandidateStore{}
	worker := NewRuleWorker(fake, registry, RuleWorkerConfig{MaxAttempts: 10})
	if err := worker.process(context.Background(), item, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if len(fake.finishes) != 1 ||
		fake.finishes[0].status != iapiserver.NotificationEventDeadLetter ||
		fake.finishes[0].code != "ERR_NOTIFICATION_RECIPIENT_UNRESOLVED" {
		t.Fatalf("finish=%+v", fake.finishes)
	}
}

func TestRuleWorkerAuditsNotifyFalseAndPreferenceIgnore(t *testing.T) {
	registry, err := BuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeCandidateStore{}
	worker := NewRuleWorker(fake, registry, DefaultRuleWorkerConfig())
	occurred := imachinery.NewTime(time.Now().UTC())
	success := &iapiserver.NotificationEvent{
		NotificationTopic: "task.atomic_task.succeeded", SourceType: "atomic_task",
		PayloadSnapshot: iapiserver.NotificationPayloadSnapshot{Status: iapiserver.AtomicTaskStatusSuccess},
		OccurredAt:      occurred,
	}
	success.ID = "success"
	if err := worker.process(context.Background(), success, occurred.Time); err != nil {
		t.Fatal(err)
	}
	if len(fake.finishes) != 1 || fake.finishes[0].status != iapiserver.NotificationEventIgnored || fake.finishes[0].summary == "" {
		t.Fatalf("notify=false finish=%+v", fake.finishes)
	}

	failed := &iapiserver.NotificationEvent{
		NotificationTopic: "task.atomic_task.failed", SourceType: "atomic_task", SourceID: "task-1",
		RecipientBasis:  iapiserver.NotificationRecipientBasis{CreatedBy: "user-1"},
		PayloadSnapshot: iapiserver.NotificationPayloadSnapshot{Status: iapiserver.AtomicTaskStatusFailed},
		OccurredAt:      occurred,
	}
	failed.ID = "preference"
	if err := worker.process(context.Background(), failed, occurred.Time); err != nil {
		t.Fatal(err)
	}
	if len(fake.finishes) != 2 || fake.finishes[1].status != iapiserver.NotificationEventIgnored || fake.finishes[1].summary == "" {
		t.Fatalf("preference finish=%+v", fake.finishes)
	}
}

func TestRuleWorkerReportsCandidateFinishFailures(t *testing.T) {
	registry, err := BuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	expected := errors.New("candidate status unavailable")
	occurred := imachinery.NewTime(time.Now().UTC())
	item := &iapiserver.NotificationEvent{
		NotificationTopic: "task.atomic_task.succeeded",
		SourceType:        "atomic_task",
		PayloadSnapshot: iapiserver.NotificationPayloadSnapshot{
			Status: iapiserver.AtomicTaskStatusSuccess,
		},
		OccurredAt: occurred,
	}
	item.ID = "finish-failure"
	fake := &fakeCandidateStore{
		claims:    []*iapiserver.NotificationEvent{item},
		finishErr: expected,
	}
	worker := NewRuleWorker(fake, registry, DefaultRuleWorkerConfig())
	if err := worker.poll(context.Background(), occurred.Time); !errors.Is(err, expected) {
		t.Fatalf("poll error=%v, want %v", err, expected)
	}
}

var _ store.NotificationCandidateStore = (*fakeCandidateStore)(nil)
var _ RuleEvaluator = failingRule{}
