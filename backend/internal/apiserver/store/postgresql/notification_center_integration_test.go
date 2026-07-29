//go:build integration

package postgresql

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestNotificationCenterPostgresMaterializationAndInboxLifecycle(t *testing.T) {
	db := openNotificationIntegrationDB(t)
	notificationStore := newNotificationStore(&datastore{db: db})
	ctx := context.Background()
	suffix := uuid.NewString()
	recipient := "notification-user-" + suffix

	t.Cleanup(func() {
		cleanupNotificationIntegrationRows(t, db, recipient, suffix)
	})

	first := notificationIntegrationCandidate(
		suffix+"-event-1",
		suffix+"-canvas-run",
		1,
		recipient,
	)
	if err := notificationStore.AddNotificationCandidates(ctx, []*iapiserver.NotificationEvent{first}); err != nil {
		t.Fatal(err)
	}
	created, inserted, err := notificationStore.MaterializeNotification(
		ctx,
		first,
		notificationIntegrationDraft(recipient, first.SourceID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted || created == nil || created.OccurrenceCount != 1 {
		t.Fatalf("created=%+v inserted=%t", created, inserted)
	}

	replayed, inserted, err := notificationStore.MaterializeNotification(
		ctx,
		first,
		notificationIntegrationDraft(recipient, first.SourceID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted || replayed == nil || replayed.ID != created.ID || replayed.OccurrenceCount != 1 {
		t.Fatalf("replayed=%+v inserted=%t", replayed, inserted)
	}

	second := notificationIntegrationCandidate(
		suffix+"-event-2",
		first.SourceID,
		2,
		recipient,
	)
	if err := notificationStore.AddNotificationCandidates(ctx, []*iapiserver.NotificationEvent{second}); err != nil {
		t.Fatal(err)
	}
	aggregated, inserted, err := notificationStore.MaterializeNotification(
		ctx,
		second,
		notificationIntegrationDraft(recipient, second.SourceID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted || aggregated.ID != created.ID || aggregated.OccurrenceCount != 2 ||
		aggregated.SourceAggregateVersion != 2 {
		t.Fatalf("aggregated=%+v inserted=%t", aggregated, inserted)
	}
	assertNotificationUpdatedFields(
		t,
		db,
		aggregated.ID,
		aggregated.ResourceVersion,
		[]string{"occurrence_count", "last_occurred_at", "expires_at"},
	)

	stale := notificationIntegrationCandidate(
		suffix+"-event-stale",
		first.SourceID,
		1,
		recipient,
	)
	if err := notificationStore.AddNotificationCandidates(ctx, []*iapiserver.NotificationEvent{stale}); err != nil {
		t.Fatal(err)
	}
	staleResult, inserted, err := notificationStore.MaterializeNotification(
		ctx,
		stale,
		notificationIntegrationDraft(recipient, stale.SourceID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted || staleResult.OccurrenceCount != 2 || staleResult.SourceAggregateVersion != 2 {
		t.Fatalf("stale result=%+v inserted=%t", staleResult, inserted)
	}

	var linkCount int64
	if err := db.Model(&iapiserver.NotificationEventLink{}).
		Where("notification_id = ?", created.ID).
		Count(&linkCount).Error; err != nil {
		t.Fatal(err)
	}
	if linkCount != 2 {
		t.Fatalf("event links=%d, want 2", linkCount)
	}

	disabledPreference := &iapiserver.NotificationPreference{
		Category:        iapiserver.NotificationCategoryTask,
		InAppEnabled:    false,
		MinimumSeverity: iapiserver.NotificationSeverityInfo,
		MergeRepeated:   true,
		DigestMode:      "none",
	}
	if _, err := notificationStore.ReplaceNotificationPreferences(
		ctx,
		recipient,
		[]*iapiserver.NotificationPreference{disabledPreference},
	); err != nil {
		t.Fatal(err)
	}
	preferenceSuppressed := notificationIntegrationCandidate(
		suffix+"-preference-event",
		suffix+"-preference-task",
		1,
		recipient,
	)
	preferenceSuppressed.NotificationTopic = "task.atomic_task.failed"
	preferenceSuppressed.SourceDomain = iapiserver.SSESourceDomainTaskCenter
	preferenceSuppressed.SourceEventType = "atomic_task_status_changed"
	preferenceSuppressed.SourceAggregateType = "atomic_task"
	preferenceSuppressed.SourceType = "atomic_task"
	if err := notificationStore.AddNotificationCandidates(
		ctx,
		[]*iapiserver.NotificationEvent{preferenceSuppressed},
	); err != nil {
		t.Fatal(err)
	}
	suppressed, inserted, err := notificationStore.MaterializeNotification(
		ctx,
		preferenceSuppressed,
		notificationIntegrationDraft(recipient, preferenceSuppressed.SourceID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if suppressed != nil || inserted {
		t.Fatalf("disabled preference materialized notification=%+v inserted=%t", suppressed, inserted)
	}

	counter, err := notificationStore.GetNotificationCounter(ctx, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if counter.UnreadCount != 1 || counter.CriticalCount != 0 || counter.ActionRequiredCount != 1 {
		t.Fatalf("counter after materialization=%+v", counter)
	}

	read, counter, err := notificationStore.MutateNotificationInbox(ctx, recipient, created.ID, "read")
	if err != nil {
		t.Fatal(err)
	}
	if read.InboxStatus != iapiserver.NotificationInboxRead || read.ReadAt.IsZero() ||
		counter.UnreadCount != 0 || counter.ActionRequiredCount != 1 {
		t.Fatalf("read=%+v counter=%+v", read, counter)
	}
	assertNotificationUpdatedFields(
		t,
		db,
		read.ID,
		read.ResourceVersion,
		[]string{"inbox_status", "read_at", "expires_at"},
	)
	archived, counter, err := notificationStore.MutateNotificationInbox(ctx, recipient, created.ID, "archive")
	if err != nil {
		t.Fatal(err)
	}
	if archived.InboxStatus != iapiserver.NotificationInboxArchived ||
		archived.ArchivedAt.IsZero() || counter.ActionRequiredCount != 0 {
		t.Fatalf("archived=%+v counter=%+v", archived, counter)
	}
	unarchived, counter, err := notificationStore.MutateNotificationInbox(ctx, recipient, created.ID, "unarchive")
	if err != nil {
		t.Fatal(err)
	}
	if unarchived.InboxStatus != iapiserver.NotificationInboxRead ||
		!unarchived.ArchivedAt.IsZero() || counter.ActionRequiredCount != 1 {
		t.Fatalf("unarchived=%+v counter=%+v", unarchived, counter)
	}
	assertNotificationUpdatedFields(
		t,
		db,
		unarchived.ID,
		unarchived.ResourceVersion,
		[]string{"inbox_status", "archived_at", "expires_at"},
	)

	var notificationOutboxCount int64
	if err := db.Model(&iapiserver.NotificationOutbox{}).
		Where("recipient_user_id = ?", recipient).
		Count(&notificationOutboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if notificationOutboxCount < 6 {
		t.Fatalf("notification outbox count=%d, want at least 6", notificationOutboxCount)
	}

	expiredAt := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&iapiserver.Notification{}).
		Where("id = ?", created.ID).
		Updates(map[string]any{
			"first_occurred_at": expiredAt.Add(-time.Minute),
			"last_occurred_at":  expiredAt.Add(-time.Minute),
			"expires_at":        expiredAt,
		}).Error; err != nil {
		t.Fatal(err)
	}
	cleanup, err := notificationStore.CleanupNotifications(ctx, time.Now().UTC(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if cleanup.Notifications != 1 {
		t.Fatalf("retention cleanup=%+v", cleanup)
	}
	items, total, err := notificationStore.ListNotifications(
		ctx,
		&iapiserver.NotificationListRequest{
			BasicQueryParam: imachinery.BasicQueryParam{
				PagingParams: imachinery.PagingParams{PageSize: 20},
				SortField:    "created_at",
				SortOrder:    "desc",
			},
			RecipientUserID: recipient,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("retained notifications total=%d items=%+v", total, items)
	}
	counter, err = notificationStore.GetNotificationCounter(ctx, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if counter.UnreadCount != 0 || counter.CriticalCount != 0 || counter.ActionRequiredCount != 0 {
		t.Fatalf("counter after retention=%+v", counter)
	}
}

func assertNotificationUpdatedFields(
	t *testing.T,
	db *gorm.DB,
	notificationID string,
	resourceVersion int64,
	expected []string,
) {
	t.Helper()
	var outbox iapiserver.NotificationOutbox
	if err := db.Where(
		"aggregate_type = ? AND aggregate_id = ? AND aggregate_version = ? AND event_name = ?",
		"notification",
		notificationID,
		resourceVersion,
		iapiserver.NotificationEventUpdated,
	).First(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(outbox.Payload.ChangedFields, expected) {
		t.Fatalf(
			"notification_updated changed_fields=%v, want %v",
			outbox.Payload.ChangedFields,
			expected,
		)
	}
}

func TestNotificationCenterPostgresConcurrentCounterProjection(t *testing.T) {
	db := openNotificationIntegrationDB(t)
	notificationStore := newNotificationStore(&datastore{db: db})
	ctx := context.Background()
	suffix := uuid.NewString()
	recipient := "notification-concurrent-user-" + suffix

	t.Cleanup(func() {
		cleanupNotificationIntegrationRows(t, db, recipient, suffix)
	})

	const total = 16
	events := make([]*iapiserver.NotificationEvent, 0, total)
	for index := 0; index < total; index++ {
		event := notificationIntegrationCandidate(
			fmt.Sprintf("%s-concurrent-event-%d", suffix, index),
			fmt.Sprintf("%s-concurrent-task-%d", suffix, index),
			1,
			recipient,
		)
		event.NotificationTopic = "task.atomic_task.failed"
		event.SourceDomain = iapiserver.SSESourceDomainTaskCenter
		event.SourceEventType = "atomic_task_status_changed"
		event.SourceAggregateType = "atomic_task"
		event.SourceType = "atomic_task"
		event.PayloadSnapshot = iapiserver.NotificationPayloadSnapshot{
			AtomicTaskID: event.SourceID,
			Status:       iapiserver.AtomicTaskStatusFailed,
		}
		events = append(events, event)
	}
	if err := notificationStore.AddNotificationCandidates(ctx, events); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, total)
	var wg sync.WaitGroup
	for _, event := range events {
		event := event
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := notificationStore.MaterializeNotification(
				ctx,
				event,
				notificationIntegrationDraft(recipient, event.SourceID),
			)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	counter, err := notificationStore.GetNotificationCounter(ctx, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if counter.UnreadCount != total || counter.ActionRequiredCount != total {
		t.Fatalf("counter=%+v, want unread/action_required=%d", counter, total)
	}
}

func TestNotificationCenterPostgresRejectsStaleCandidateCompletion(t *testing.T) {
	db := openNotificationIntegrationDB(t)
	notificationStore := newNotificationStore(&datastore{db: db})
	ctx := context.Background()
	suffix := uuid.NewString()
	recipient := "notification-lease-user-" + suffix

	t.Cleanup(func() {
		cleanupNotificationIntegrationRows(t, db, recipient, suffix)
	})

	candidate := notificationIntegrationCandidate(
		suffix+"-lease-event",
		suffix+"-lease-canvas-run",
		1,
		recipient,
	)
	if err := notificationStore.AddNotificationCandidates(
		ctx,
		[]*iapiserver.NotificationEvent{candidate},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first, err := notificationStore.ClaimNotificationCandidates(ctx, now, 1, time.Second)
	if err != nil || len(first) != 1 || first[0].ProcessingAttemptCount != 1 {
		t.Fatalf("first claim=%+v error=%v", first, err)
	}
	second, err := notificationStore.ClaimNotificationCandidates(
		ctx,
		now.Add(2*time.Second),
		1,
		time.Second,
	)
	if err != nil || len(second) != 1 || second[0].ProcessingAttemptCount != 2 {
		t.Fatalf("second claim=%+v error=%v", second, err)
	}
	if err := notificationStore.FinishNotificationCandidate(
		ctx,
		candidate.ID,
		first[0].ProcessingAttemptCount,
		iapiserver.NotificationEventProcessed,
		time.Time{},
		"",
		"",
	); err != nil {
		t.Fatal(err)
	}
	var current iapiserver.NotificationEvent
	if err := db.First(&current, "id = ?", candidate.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.ProcessingStatus != iapiserver.NotificationEventFailed ||
		current.ProcessingAttemptCount != 2 {
		t.Fatalf("stale completion changed candidate=%+v", current)
	}
	if err := notificationStore.FinishNotificationCandidate(
		ctx,
		candidate.ID,
		second[0].ProcessingAttemptCount,
		iapiserver.NotificationEventProcessed,
		time.Time{},
		"",
		"",
	); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&current, "id = ?", candidate.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.ProcessingStatus != iapiserver.NotificationEventProcessed ||
		current.ProcessingAttemptCount != 2 {
		t.Fatalf("current completion was not persisted=%+v", current)
	}
}

func TestNotificationCenterPostgresDeadLetterAndOutboxRecovery(t *testing.T) {
	db := openNotificationIntegrationDB(t)
	notificationStore := newNotificationStore(&datastore{db: db})
	ctx := context.Background()
	suffix := uuid.NewString()
	recipient := "notification-recovery-user-" + suffix

	t.Cleanup(func() {
		cleanupNotificationIntegrationRows(t, db, recipient, suffix)
	})

	unresolved := notificationIntegrationCandidate(
		suffix+"-unresolved-event",
		suffix+"-unresolved-task",
		1,
		"",
	)
	unresolved.NotificationTopic = "task.atomic_task.failed"
	unresolved.SourceDomain = iapiserver.SSESourceDomainTaskCenter
	unresolved.SourceEventType = "atomic_task_status_changed"
	unresolved.SourceAggregateType = "atomic_task"
	unresolved.SourceType = "atomic_task"
	if err := notificationStore.AddNotificationCandidates(
		ctx,
		[]*iapiserver.NotificationEvent{unresolved},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&iapiserver.NotificationEvent{}).
		Where("id = ?", unresolved.ID).
		Updates(map[string]any{
			"processing_status":        iapiserver.NotificationEventFailed,
			"processing_attempt_count": 10,
			"next_attempt_at":          time.Now().UTC().Add(time.Minute),
		}).Error; err != nil {
		t.Fatal(err)
	}
	if err := notificationStore.FinishNotificationCandidate(
		ctx,
		unresolved.ID,
		10,
		iapiserver.NotificationEventDeadLetter,
		time.Time{},
		"ERR_NOTIFICATION_RECIPIENT_UNRESOLVED",
		"notification recipient is unresolved",
	); err != nil {
		t.Fatal(err)
	}
	var deadLetter iapiserver.NotificationEvent
	if err := db.First(&deadLetter, "id = ?", unresolved.ID).Error; err != nil {
		t.Fatal(err)
	}
	if deadLetter.ProcessingStatus != iapiserver.NotificationEventDeadLetter ||
		deadLetter.LastErrorCode != "ERR_NOTIFICATION_RECIPIENT_UNRESOLVED" ||
		deadLetter.ProcessedAt.IsZero() || !deadLetter.NextAttemptAt.IsZero() {
		t.Fatalf("dead-letter candidate=%+v", deadLetter)
	}

	candidate := notificationIntegrationCandidate(
		suffix+"-outbox-event",
		suffix+"-outbox-canvas-run",
		1,
		recipient,
	)
	if err := notificationStore.AddNotificationCandidates(
		ctx,
		[]*iapiserver.NotificationEvent{candidate},
	); err != nil {
		t.Fatal(err)
	}
	notification, inserted, err := notificationStore.MaterializeNotification(
		ctx,
		candidate,
		notificationIntegrationDraft(recipient, candidate.SourceID),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted || notification == nil {
		t.Fatalf("notification=%+v inserted=%t", notification, inserted)
	}
	var outbox iapiserver.NotificationOutbox
	if err := db.Where(
		"aggregate_type = ? AND aggregate_id = ? AND aggregate_version = ? AND event_name = ?",
		"notification",
		notification.ID,
		notification.ResourceVersion,
		iapiserver.NotificationEventCreated,
	).First(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	nextAttempt := time.Now().UTC().Add(time.Second)
	if err := notificationStore.MarkNotificationOutboxFailed(
		ctx,
		outbox.ID,
		nextAttempt,
		"ERR_NOTIFICATION_RULE_PROCESSING_FAILED",
		"UserEvent store unavailable",
	); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&outbox, "id = ?", outbox.ID).Error; err != nil {
		t.Fatal(err)
	}
	if outbox.DeliveryStatus != iapiserver.NotificationOutboxFailed ||
		outbox.AttemptCount != 1 || outbox.NextAttemptAt.IsZero() ||
		outbox.Payload.NotificationID != notification.ID {
		t.Fatalf("failed outbox=%+v", outbox)
	}
	publishedAt := time.Now().UTC()
	if err := notificationStore.MarkNotificationOutboxPublished(
		ctx,
		outbox.ID,
		publishedAt,
	); err != nil {
		t.Fatal(err)
	}
	if err := notificationStore.MarkNotificationOutboxPublished(
		ctx,
		outbox.ID,
		publishedAt,
	); err != nil {
		t.Fatal(err)
	}
	var publishedOutbox iapiserver.NotificationOutbox
	if err := db.First(&publishedOutbox, "id = ?", outbox.ID).Error; err != nil {
		t.Fatal(err)
	}
	if publishedOutbox.DeliveryStatus != iapiserver.NotificationOutboxPublished ||
		publishedOutbox.AttemptCount != 1 || !publishedOutbox.NextAttemptAt.IsZero() ||
		publishedOutbox.PublishedAt.IsZero() || publishedOutbox.Description != "" ||
		publishedOutbox.Payload.NotificationID != notification.ID {
		t.Fatalf("published outbox=%+v", publishedOutbox)
	}
}

func openNotificationIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&iapiserver.NotificationTopic{},
		&iapiserver.NotificationEvent{},
		&iapiserver.Notification{},
		&iapiserver.NotificationEventLink{},
		&iapiserver.NotificationRecipientCounter{},
		&iapiserver.NotificationPreference{},
		&iapiserver.NotificationDelivery{},
		&iapiserver.NotificationOutbox{},
	); err != nil {
		t.Fatal(err)
	}
	ds := &datastore{db: db}
	if err := ds.ensureOutboxScheme(); err != nil {
		t.Fatal(err)
	}
	if err := ds.ensureNotificationCenterScheme(); err != nil {
		t.Fatal(err)
	}
	return db
}

func notificationIntegrationCandidate(
	eventID string,
	sourceID string,
	version int64,
	recipient string,
) *iapiserver.NotificationEvent {
	occurredAt := time.Now().UTC()
	event := &iapiserver.NotificationEvent{
		SourceDomain:           iapiserver.SSESourceDomainWorkflowCanvas,
		SourceEventType:        "canvas_run_status_changed",
		SourceEventID:          eventID,
		SourceAggregateType:    "canvas_run",
		SourceAggregateID:      sourceID,
		SourceAggregateVersion: version,
		NotificationTopic:      "canvas.run.failed",
		SourceType:             "canvas_run",
		SourceID:               sourceID,
		RecipientBasis:         iapiserver.NotificationRecipientBasis{CreatedBy: recipient},
		OccurredAt:             imachinery.NewTime(occurredAt),
		PayloadSnapshot: iapiserver.NotificationPayloadSnapshot{
			CanvasRunID: sourceID,
			CanvasID:    "canvas-" + sourceID,
			Status:      iapiserver.CanvasRunStatusFailed,
		},
		ProcessingStatus: iapiserver.NotificationEventPending,
		DeduplicationKey: eventID + ":canvas.run.failed",
		RuleVersion:      1,
		ExpiresAt:        imachinery.NewTime(occurredAt.Add(90 * 24 * time.Hour)),
	}
	event.Name = eventID
	return event
}

func notificationIntegrationDraft(
	recipient string,
	sourceID string,
) store.NotificationMaterialization {
	path := "/notifications/integration/" + sourceID
	return store.NotificationMaterialization{
		RecipientUserID: recipient,
		Title:           "画布运行失败",
		Content:         "画布运行失败，请查看运行详情。",
		Severity:        iapiserver.NotificationSeverityError,
		AttentionStatus: iapiserver.NotificationAttentionActionRequired,
		NavigationTarget: &iapiserver.NotificationNavigationTarget{
			Type:       "navigate",
			TargetType: "canvas_run",
			TargetID:   sourceID,
			View:       "detail",
			Params:     map[string]string{},
		},
		ActionPath:        &path,
		AggregateKey:      sourceID,
		AggregationWindow: "per_source",
		ExpiresAt:         imachinery.NewTime(time.Now().UTC().Add(365 * 24 * time.Hour)),
	}
}

func cleanupNotificationIntegrationRows(
	t *testing.T,
	db *gorm.DB,
	recipient string,
	suffix string,
) {
	t.Helper()
	if err := db.Transaction(func(tx *gorm.DB) error {
		var notificationIDs []string
		if err := tx.Model(&iapiserver.Notification{}).
			Where("recipient_user_id = ?", recipient).
			Pluck("id", &notificationIDs).Error; err != nil {
			return err
		}
		if len(notificationIDs) > 0 {
			if err := tx.Where("notification_id IN ?", notificationIDs).
				Delete(&iapiserver.NotificationEventLink{}).Error; err != nil {
				return err
			}
			if err := tx.Where("notification_id IN ?", notificationIDs).
				Delete(&iapiserver.NotificationDelivery{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("recipient_user_id = ?", recipient).
			Delete(&iapiserver.NotificationOutbox{}).Error; err != nil {
			return err
		}
		if err := tx.Where("recipient_user_id = ?", recipient).
			Delete(&iapiserver.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("recipient_user_id = ?", recipient).
			Delete(&iapiserver.NotificationRecipientCounter{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", recipient).
			Delete(&iapiserver.NotificationPreference{}).Error; err != nil {
			return err
		}
		if err := tx.Where("source_event_id LIKE ?", suffix+"%").
			Delete(&iapiserver.NotificationEvent{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Errorf("cleanup notification integration rows: %v", err)
	}
}
