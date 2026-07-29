package sse

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type recordingNotificationOutboxStore struct {
	published       []string
	failed          []string
	publishFailures int
}

func (s *recordingNotificationOutboxStore) MarkNotificationOutboxPublished(_ context.Context, id string, _ time.Time) error {
	if s.publishFailures > 0 {
		s.publishFailures--
		return errors.New("notification outbox confirmation unavailable")
	}
	s.published = append(s.published, id)
	return nil
}
func (s *recordingNotificationOutboxStore) MarkNotificationOutboxFailed(_ context.Context, id string, _ time.Time, _, _ string) error {
	s.failed = append(s.failed, id)
	return nil
}

type failingNotificationUserEventStore struct{ fakeUserEventStore }

func (s *failingNotificationUserEventStore) AddIdempotent(context.Context, *iapiserver.UserEvent) (*iapiserver.UserEvent, bool, error) {
	return nil, false, errors.New("user event unavailable")
}

type transientNotificationUserEventStore struct {
	recordingUserEventStore
	failures int
}

func (s *transientNotificationUserEventStore) AddIdempotent(
	ctx context.Context,
	event *iapiserver.UserEvent,
) (*iapiserver.UserEvent, bool, error) {
	if s.failures > 0 {
		s.failures--
		return nil, false, errors.New("user event temporarily unavailable")
	}
	return s.recordingUserEventStore.AddIdempotent(ctx, event)
}

func TestNotificationProjectorMapsAllOutboxEvents(t *testing.T) {
	tests := []struct {
		topic, eventType, aggregateType string
		counter                         bool
	}{
		{iapiserver.NotificationEventCreated, iapiserver.UserEventNotificationCreated, "notification", false},
		{iapiserver.NotificationEventUpdated, iapiserver.UserEventNotificationUpdated, "notification", false},
		{iapiserver.NotificationEventDeleted, iapiserver.UserEventNotificationDeleted, "notification", false},
		{iapiserver.NotificationEventUnreadCountChanged, iapiserver.UserEventNotificationUnreadCountChanged, "notification_recipient_counter", true},
	}
	for _, test := range tests {
		t.Run(test.topic, func(t *testing.T) {
			events := &recordingUserEventStore{}
			outbox := &recordingNotificationOutboxStore{}
			projector := NewNotificationProjector(events, outbox, time.Hour, nil)
			payload := map[string]any{
				"notification_outbox_id": "outbox-1", "notification_id": "notification-1",
				"notification_topic": "canvas.run.failed", "category": "canvas", "severity": "error",
				"inbox_status": "unread", "attention_status": "action_required",
				"recipient_user_id": "user-1", "aggregate_type": test.aggregateType,
				"aggregate_id": "notification-1", "aggregate_version": 3,
				"resource_version": 3, "counter_version": 3,
				"changed_fields": []string{"inbox_status"},
				"deleted_at":     "2026-07-29T01:00:00Z", "occurred_at": "2026-07-29T01:00:00Z",
			}
			if test.counter {
				payload["aggregate_id"] = "user-1"
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := projector.project(context.Background(), test.topic, raw); err != nil {
				t.Fatal(err)
			}
			if len(events.events) != 1 || events.events[0].EventType != test.eventType ||
				events.events[0].SourceEventID != "outbox-1" || events.events[0].AggregateVersion != 3 {
				t.Fatalf("events=%+v", events.events)
			}
			if test.counter {
				payload := events.events[0].Payload
				if len(payload) != 3 || payload["unread_count"] != 0 ||
					payload["critical_count"] != 0 ||
					payload["action_required_count"] != 0 {
					t.Fatalf("counter payload=%+v", payload)
				}
			}
			if len(outbox.published) != 1 || len(outbox.failed) != 0 {
				t.Fatalf("published=%v failed=%v", outbox.published, outbox.failed)
			}
		})
	}
}

func TestNotificationProjectorIsIdempotentAndDoesNotConfirmFailure(t *testing.T) {
	payload := []byte(`{"notification_outbox_id":"outbox-1","notification_id":"notification-1","recipient_user_id":"user-1","aggregate_type":"notification","aggregate_id":"notification-1","aggregate_version":1,"resource_version":1,"occurred_at":"2026-07-29T01:00:00Z"}`)
	events := &recordingUserEventStore{}
	outbox := &recordingNotificationOutboxStore{}
	projector := NewNotificationProjector(events, outbox, time.Hour, nil)
	if err := projector.project(context.Background(), iapiserver.NotificationEventCreated, payload); err != nil {
		t.Fatal(err)
	}
	if err := projector.project(context.Background(), iapiserver.NotificationEventCreated, payload); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 1 {
		t.Fatalf("duplicate events=%d", len(events.events))
	}

	failingOutbox := &recordingNotificationOutboxStore{}
	failing := NewNotificationProjector(&failingNotificationUserEventStore{}, failingOutbox, time.Hour, nil)
	if err := failing.project(context.Background(), iapiserver.NotificationEventCreated, payload); err == nil {
		t.Fatal("projection failure returned nil")
	}
	if len(failingOutbox.published) != 0 || len(failingOutbox.failed) != 1 {
		t.Fatalf("published=%v failed=%v", failingOutbox.published, failingOutbox.failed)
	}
}

func TestNotificationProjectorRecoversAfterTransientUserEventFailure(t *testing.T) {
	payload := []byte(`{"notification_outbox_id":"outbox-retry","notification_id":"notification-1","recipient_user_id":"user-1","aggregate_type":"notification","aggregate_id":"notification-1","aggregate_version":1,"resource_version":1,"occurred_at":"2026-07-29T01:00:00Z"}`)
	events := &transientNotificationUserEventStore{failures: 1}
	outbox := &recordingNotificationOutboxStore{}
	projector := NewNotificationProjector(events, outbox, time.Hour, nil)

	if err := projector.project(
		context.Background(),
		iapiserver.NotificationEventCreated,
		payload,
	); err == nil {
		t.Fatal("transient projection failure returned nil")
	}
	if err := projector.project(
		context.Background(),
		iapiserver.NotificationEventCreated,
		payload,
	); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 1 || len(outbox.failed) != 1 ||
		len(outbox.published) != 1 {
		t.Fatalf(
			"events=%d failed=%v published=%v",
			len(events.events),
			outbox.failed,
			outbox.published,
		)
	}
}

func TestNotificationProjectorRecoversAfterTransientOutboxConfirmationFailure(t *testing.T) {
	payload := []byte(`{"notification_outbox_id":"outbox-confirm-retry","notification_id":"notification-1","recipient_user_id":"user-1","aggregate_type":"notification","aggregate_id":"notification-1","aggregate_version":1,"resource_version":1,"occurred_at":"2026-07-29T01:00:00Z"}`)
	events := &recordingUserEventStore{}
	outbox := &recordingNotificationOutboxStore{publishFailures: 1}
	projector := NewNotificationProjector(events, outbox, time.Hour, nil)

	if err := projector.project(
		context.Background(),
		iapiserver.NotificationEventCreated,
		payload,
	); err == nil {
		t.Fatal("transient outbox confirmation failure returned nil")
	}
	if err := projector.project(
		context.Background(),
		iapiserver.NotificationEventCreated,
		payload,
	); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 1 || len(outbox.failed) != 1 ||
		len(outbox.published) != 1 {
		t.Fatalf(
			"events=%d failed=%v published=%v",
			len(events.events),
			outbox.failed,
			outbox.published,
		)
	}
}

func TestNotificationProjectorRejectsUpdatedEventWithoutChangedFields(t *testing.T) {
	payload := []byte(`{"notification_outbox_id":"outbox-invalid","notification_id":"notification-1","recipient_user_id":"user-1","aggregate_type":"notification","aggregate_id":"notification-1","aggregate_version":1,"resource_version":1,"occurred_at":"2026-07-29T01:00:00Z"}`)
	events := &recordingUserEventStore{}
	outbox := &recordingNotificationOutboxStore{}
	projector := NewNotificationProjector(events, outbox, time.Hour, nil)

	if err := projector.project(
		context.Background(),
		iapiserver.NotificationEventUpdated,
		payload,
	); err == nil {
		t.Fatal("notification_updated without changed_fields was accepted")
	}
	if len(events.events) != 0 || len(outbox.failed) != 1 ||
		len(outbox.published) != 0 {
		t.Fatalf(
			"events=%d failed=%v published=%v",
			len(events.events),
			outbox.failed,
			outbox.published,
		)
	}
}
