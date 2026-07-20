package sse

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type recordingUserEventStore struct {
	fakeUserEventStore
	events []*iapiserver.UserEvent
}

func TestProjectorConsumerAcknowledgesPersistedEventAndStops(t *testing.T) {
	store := &recordingUserEventStore{}
	channels := make(map[string]chan *message.Message)
	subscribe := func(_ context.Context, topic, _ string) (<-chan *message.Message, error) {
		channel := make(chan *message.Message, 1)
		channels[topic] = channel
		return channel, nil
	}
	projector := NewProjector(store, time.Hour, subscribe)
	if err := projector.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	payload := []byte(`{"source_domain":"task-center","source_event_id":"task-1:created","created_by":"user-1","resource_version":1,"occurred_at":"2026-07-20T00:00:00Z","correlation_id":"task-1","atomic_task_id":"task-1","status":"PENDING"}`)
	msg := message.NewMessage("message-1", payload)
	channels[topicAtomicTaskCreated] <- msg
	select {
	case <-msg.Acked():
	case <-time.After(time.Second):
		t.Fatal("projector did not acknowledge persisted event")
	}
	done := make(chan struct{})
	go func() {
		projector.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Projector.Close() did not stop consumers")
	}
}

func (s *recordingUserEventStore) AddIdempotent(_ context.Context, event *iapiserver.UserEvent) (*iapiserver.UserEvent, bool, error) {
	for _, existing := range s.events {
		if existing.RecipientUserID == event.RecipientUserID && existing.SourceDomain == event.SourceDomain && existing.SourceEventID == event.SourceEventID && existing.EventType == event.EventType {
			return existing, false, nil
		}
	}
	s.events = append(s.events, event)
	return event, true, nil
}

func TestProjectorMapsTaskCenterEventAndRedactsRoutingFields(t *testing.T) {
	store := &recordingUserEventStore{}
	projector := NewProjector(store, time.Hour, nil)
	occurred := imachinery.Now()
	payload := map[string]any{
		"source_domain": "task-center", "source_event_id": "task-1:2", "sse_event_type": "atomic_task.progressed",
		"created_by": "user-1", "resource_version": 2, "occurred_at": occurred, "correlation_id": "task-1",
		"atomic_task_id": "task-1", "project_id": "project-1", "namespace": "default", "status": "RUNNING", "progress": 0.5,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.project(context.Background(), topicAtomicTaskStatusChanged, raw); err != nil {
		t.Fatalf("project() error = %v", err)
	}
	if err := projector.project(context.Background(), topicAtomicTaskStatusChanged, raw); err != nil {
		t.Fatalf("duplicate project() error = %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("projected events = %d, want 1", len(store.events))
	}
	event := store.events[0]
	if event.RecipientUserID != "user-1" || event.EventType != iapiserver.UserEventAtomicTaskProgressed || event.AtomicTaskID != "task-1" || event.AggregateVersion != 2 {
		t.Fatalf("projected event = %#v", event)
	}
	for _, key := range []string{"source_domain", "source_event_id", "created_by", "project_id", "namespace", "resource_version"} {
		if _, exists := event.Payload[key]; exists {
			t.Fatalf("projected payload retained internal field %q: %#v", key, event.Payload)
		}
	}
	if event.ExpiresAt.Sub(event.OccurredAt.Time) != time.Hour {
		t.Fatalf("retention = %s, want 1h", event.ExpiresAt.Sub(event.OccurredAt.Time))
	}
}

func TestProjectorRejectsSourceWithoutOwner(t *testing.T) {
	projector := NewProjector(&recordingUserEventStore{}, time.Hour, nil)
	raw := []byte(`{"source_domain":"task-center","source_event_id":"task-1:1","resource_version":1,"occurred_at":"2026-07-20T00:00:00Z","atomic_task_id":"task-1"}`)
	if err := projector.project(context.Background(), topicAtomicTaskCreated, raw); err == nil {
		t.Fatal("project() error = nil")
	}
}
