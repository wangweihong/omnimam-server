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

func TestProjectorMapsAssetLibraryEvents(t *testing.T) {
	tests := []struct {
		name            string
		topic           string
		fields          map[string]any
		wantEventType   string
		wantAggregate   string
		wantAggregateID string
	}{
		{name: "artifact created", topic: topicArtifactCreated, fields: map[string]any{"artifact_id": "artifact-1"}, wantEventType: iapiserver.UserEventArtifactCreated, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "artifact transferring", topic: topicArtifactProcessingChanged, fields: map[string]any{"artifact_id": "artifact-1", "change_type": "transferring", "progress": 0.25, "retryable": true, "error_code": nil, "processing_error_code": nil}, wantEventType: iapiserver.UserEventArtifactTransferring, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "artifact processing", topic: topicArtifactProcessingChanged, fields: map[string]any{"artifact_id": "artifact-1", "change_type": "processing"}, wantEventType: iapiserver.UserEventArtifactProcessing, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "artifact preview ready", topic: topicArtifactProcessingChanged, fields: map[string]any{"artifact_id": "artifact-1", "change_type": "preview_ready"}, wantEventType: iapiserver.UserEventArtifactPreviewReady, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "artifact ready", topic: topicArtifactProcessingChanged, fields: map[string]any{"artifact_id": "artifact-1", "change_type": "ready"}, wantEventType: iapiserver.UserEventArtifactReady, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "artifact failed", topic: topicArtifactProcessingChanged, fields: map[string]any{"artifact_id": "artifact-1", "change_type": "failed"}, wantEventType: iapiserver.UserEventArtifactProcessingFailed, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "artifact deleted", topic: topicArtifactProcessingChanged, fields: map[string]any{"artifact_id": "artifact-1", "change_type": "deleted"}, wantEventType: iapiserver.UserEventArtifactDeleted, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "registration succeeded", topic: topicArtifactRegistrationChanged, fields: map[string]any{"artifact_id": "artifact-1", "registration_status": "registered", "asset_id": "asset-1", "asset_version_id": "version-1"}, wantEventType: iapiserver.UserEventArtifactRegistrationSucceeded, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "registration failed", topic: topicArtifactRegistrationChanged, fields: map[string]any{"artifact_id": "artifact-1", "registration_status": "failed"}, wantEventType: iapiserver.UserEventArtifactRegistrationFailed, wantAggregate: "artifact", wantAggregateID: "artifact-1"},
		{name: "version started", topic: topicAssetVersionProcessingChanged, fields: map[string]any{"asset_id": "asset-1", "asset_version_id": "version-1", "status": "processing", "change_type": "started"}, wantEventType: iapiserver.UserEventAssetVersionProcessingStarted, wantAggregate: "asset_version", wantAggregateID: "version-1"},
		{name: "version progressed", topic: topicAssetVersionProcessingChanged, fields: map[string]any{"asset_id": "asset-1", "asset_version_id": "version-1", "status": "processing", "change_type": "progressed"}, wantEventType: iapiserver.UserEventAssetVersionProcessingProgressed, wantAggregate: "asset_version", wantAggregateID: "version-1"},
		{name: "version ready", topic: topicAssetVersionProcessingChanged, fields: map[string]any{"asset_id": "asset-1", "asset_version_id": "version-1", "status": "ready"}, wantEventType: iapiserver.UserEventAssetVersionReady, wantAggregate: "asset_version", wantAggregateID: "version-1"},
		{name: "version ready with warnings", topic: topicAssetVersionProcessingChanged, fields: map[string]any{"asset_id": "asset-1", "asset_version_id": "version-1", "status": "ready_with_warnings"}, wantEventType: iapiserver.UserEventAssetVersionReadyWithWarnings, wantAggregate: "asset_version", wantAggregateID: "version-1"},
		{name: "version failed", topic: topicAssetVersionProcessingChanged, fields: map[string]any{"asset_id": "asset-1", "asset_version_id": "version-1", "status": "failed"}, wantEventType: iapiserver.UserEventAssetVersionProcessingFailed, wantAggregate: "asset_version", wantAggregateID: "version-1"},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &recordingUserEventStore{}
			projector := NewProjector(storage, time.Hour, nil)
			payload := map[string]any{
				"source_domain":    iapiserver.SSESourceDomainAssetLibrary,
				"source_event_id":  tt.name,
				"owner_user_id":    "user-1",
				"resource_version": index + 1,
				"occurred_at":      "2026-07-20T00:00:00Z",
				"project_id":       "internal-project",
				"producer_type":    "atomic_task",
				"producer_id":      "internal-task",
				"artifact_type":    "image",
				"sequence":         0,
			}
			for key, value := range tt.fields {
				payload[key] = value
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := projector.project(context.Background(), tt.topic, raw); err != nil {
				t.Fatalf("project() error = %v", err)
			}
			if len(storage.events) != 1 {
				t.Fatalf("events = %d, want 1", len(storage.events))
			}
			event := storage.events[0]
			if event.RecipientUserID != "user-1" || event.EventType != tt.wantEventType || event.AggregateType != tt.wantAggregate || event.AggregateID != tt.wantAggregateID {
				t.Fatalf("projected event = %#v", event)
			}
			for _, key := range []string{"source_domain", "source_event_id", "owner_user_id", "project_id", "resource_version", "change_type", "producer_type", "producer_id", "artifact_type", "sequence"} {
				if _, exists := event.Payload[key]; exists {
					t.Fatalf("payload retained internal field %q: %#v", key, event.Payload)
				}
			}
			if tt.name == "artifact transferring" {
				if event.Payload["processing_progress"] != 0.25 || event.Payload["processing_retryable"] != true {
					t.Fatalf("processing payload was not normalized: %#v", event.Payload)
				}
				if _, exists := event.Payload["progress"]; exists {
					t.Fatalf("source progress leaked into public payload: %#v", event.Payload)
				}
			}
		})
	}
}

func TestProjectorRejectsAssetLibraryEventWithWrongDomain(t *testing.T) {
	projector := NewProjector(&recordingUserEventStore{}, time.Hour, nil)
	raw := []byte(`{"source_domain":"task-center","source_event_id":"artifact-1:1:created","owner_user_id":"user-1","resource_version":1,"occurred_at":"2026-07-20T00:00:00Z","artifact_id":"artifact-1"}`)
	if err := projector.project(context.Background(), topicArtifactCreated, raw); err == nil {
		t.Fatal("project() error = nil")
	}
}
