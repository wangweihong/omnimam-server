//go:build integration

package postgresql

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestPostgresTaskCenterProjectsReplayableUserEvents(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&iapiserver.AtomicTask{}, &iapiserver.TaskAttempt{}, &iapiserver.TaskGroup{}, &iapiserver.DAGTaskGroup{}, &iapiserver.RuntimeProjectionEvent{}, &iapiserver.UserEvent{}, &iapiserver.Artifact{}, &iapiserver.AssetVersion{}); err != nil {
		t.Fatal(err)
	}
	ds := &datastore{db: db}
	if err := ds.ensureOutboxScheme(); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	storage := newTaskCenterStore(&datastore{db: tx})
	ctx := context.Background()
	task := &iapiserver.AtomicTask{
		FunctionRef: "test.execute", Status: iapiserver.AtomicTaskStatusPending, ProjectID: "project-1",
		Namespace: "default", CreatedBy: "user-1", Output: map[string]any{}, Arguments: map[string]any{},
	}
	task.Name = "SSE integration task"
	created, inserted, err := storage.AddAtomicTaskIdempotent(ctx, task)
	if err != nil || !inserted {
		t.Fatalf("AddAtomicTaskIdempotent() inserted=%t error=%v", inserted, err)
	}
	created.Status = iapiserver.AtomicTaskStatusRunning
	created.Progress = 0.25
	if _, err := storage.UpdateAtomicTask(ctx, created); err != nil {
		t.Fatal(err)
	}

	for table, want := range map[string]int64{"watermill_atomic_task_created": 1, "watermill_atomic_task_status_changed": 1} {
		var count int64
		if err := tx.Table(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("outbox %s count=%d, want=%d, error=%v", table, count, want, err)
		}
	}
	userEvents := newUserEventStore(&datastore{db: tx})
	for sequence, eventType := range []string{iapiserver.UserEventAtomicTaskCreated, iapiserver.UserEventAtomicTaskStarted} {
		occurred := time.Now().Add(time.Duration(sequence) * time.Second)
		event := &iapiserver.UserEvent{
			RecipientUserID: "user-1", EventType: eventType, EventVersion: 1, AggregateType: "atomic_task",
			AggregateID: created.ID, AggregateVersion: int64(sequence + 1), AtomicTaskID: created.ID,
			Payload: map[string]any{"status": created.Status}, SourceDomain: iapiserver.SSESourceDomainTaskCenter,
			SourceEventID: eventType, OccurredAt: imachinery.NewTime(occurred), ExpiresAt: imachinery.NewTime(occurred.Add(time.Hour)),
		}
		if _, inserted, err := userEvents.AddIdempotent(ctx, event); err != nil || !inserted {
			t.Fatalf("AddIdempotent() inserted=%t error=%v", inserted, err)
		}
	}
	items, err := userEvents.ListAfter(ctx, "user-1", 0, 20, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].EventType != iapiserver.UserEventAtomicTaskCreated || items[1].EventType != iapiserver.UserEventAtomicTaskStarted {
		t.Fatalf("projected events = %#v", items)
	}
	if items[0].EventSequence >= items[1].EventSequence || items[1].AggregateVersion <= items[0].AggregateVersion {
		t.Fatalf("event order/version did not advance: %#v", items)
	}
	other, err := userEvents.ListAfter(ctx, "user-2", 0, 20, time.Now())
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-user events = %#v, error=%v", other, err)
	}
	visible, expired, err := userEvents.CursorVisible(ctx, "user-2", items[0].EventSequence)
	if err != nil || visible || expired {
		t.Fatalf("cross-user cursor visible=%t expired=%t error=%v", visible, expired, err)
	}
	if items[0].OccurredAt.IsZero() || items[0].ExpiresAt.IsZero() || !items[0].ExpiresAt.After(items[0].OccurredAt.Time) {
		t.Fatalf("event retention fields are invalid: %#v", items[0])
	}
}

func TestPostgresAssetLifecycleWritesReliableSSESourceEvents(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&iapiserver.Artifact{}, &iapiserver.AssetVersion{}); err != nil {
		t.Fatal(err)
	}
	ds := &datastore{db: db}
	if err := ds.ensureOutboxScheme(); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	storage := newAssetV1Store(&datastore{db: tx})
	ctx := context.Background()
	artifact := &iapiserver.Artifact{
		OwnerUserID: "user-asset-sse", ArtifactType: "image", MediaType: "image",
		ProducerType: "atomic_task", ProducerID: "task-asset-sse", ProducerIdempotencyKey: "task-asset-sse:images:0",
		AtomicTaskID: "task-asset-sse", OutputKey: "images", Sequence: 0,
		ProcessingStatus: iapiserver.ArtifactProcessingCreated, RegistrationStatus: iapiserver.ArtifactRegistrationPending,
		SavePolicy: iapiserver.ArtifactSaveTransient, ProcessingProfileVersion: "v1", Metadata: map[string]any{"size_bytes": 128},
	}
	artifact.Name = "Task output image"
	created, inserted, err := storage.CreateArtifact(ctx, artifact)
	if err != nil || !inserted {
		t.Fatalf("CreateArtifact() inserted=%t error=%v", inserted, err)
	}
	duplicate, inserted, err := storage.CreateArtifact(ctx, artifact)
	if err != nil || inserted || duplicate.ID != created.ID {
		t.Fatalf("duplicate CreateArtifact() inserted=%t artifact=%#v error=%v", inserted, duplicate, err)
	}
	progress := 1.0
	ready, err := storage.UpdateArtifactProcessing(ctx, created.ID, created.OwnerUserID, created.ResourceVersion, store.ArtifactProcessingMutation{
		ChangeType: "ready", ProcessingStatus: iapiserver.ArtifactProcessingReady, Progress: &progress, PreviewAvailable: true, PreviewRef: "preview-token", ReadyAt: func() *imachinery.Time { value := imachinery.Now(); return &value }(),
	})
	if err != nil || ready.ResourceVersion != created.ResourceVersion+1 {
		t.Fatalf("UpdateArtifactProcessing() artifact=%#v error=%v", ready, err)
	}
	unchangedArtifact, err := storage.UpdateArtifactProcessing(ctx, ready.ID, ready.OwnerUserID, ready.ResourceVersion, store.ArtifactProcessingMutation{
		ChangeType: "ready", ProcessingStatus: iapiserver.ArtifactProcessingReady, Progress: &progress, PreviewAvailable: true, PreviewRef: "preview-token",
	})
	if err != nil || unchangedArtifact.ResourceVersion != ready.ResourceVersion {
		t.Fatalf("duplicate UpdateArtifactProcessing() artifact=%#v error=%v", unchangedArtifact, err)
	}
	version := &iapiserver.AssetVersion{
		AssetID: "asset-sse", OwnerUserID: created.OwnerUserID, VersionNo: 1, Status: iapiserver.AssetVersionStatusProcessing,
		SourceType: "artifact", SourceRefID: created.ID, ProfileVersion: "v1", ExpectedCount: 2,
	}
	version.Name = "Asset version 1"
	createdVersion, inserted, err := storage.CreateAssetVersion(ctx, version, "group-sse", "task-sse")
	if err != nil || !inserted {
		t.Fatalf("CreateAssetVersion() inserted=%t error=%v", inserted, err)
	}
	registered, err := storage.UpdateArtifactRegistration(ctx, ready.ID, ready.OwnerUserID, ready.ResourceVersion, store.ArtifactRegistrationMutation{
		RegistrationStatus: iapiserver.ArtifactRegistrationRegistered, RegistrationResult: "created", AssetID: version.AssetID, AssetVersionID: createdVersion.ID,
	})
	if err != nil || registered.ResourceVersion != ready.ResourceVersion+1 {
		t.Fatalf("UpdateArtifactRegistration() artifact=%#v error=%v", registered, err)
	}
	unchangedRegistration, err := storage.UpdateArtifactRegistration(ctx, registered.ID, registered.OwnerUserID, registered.ResourceVersion, store.ArtifactRegistrationMutation{
		RegistrationStatus: iapiserver.ArtifactRegistrationRegistered, RegistrationResult: "already_registered", AssetID: version.AssetID, AssetVersionID: createdVersion.ID,
	})
	if err != nil || unchangedRegistration.ResourceVersion != registered.ResourceVersion {
		t.Fatalf("duplicate UpdateArtifactRegistration() artifact=%#v error=%v", unchangedRegistration, err)
	}
	finishedVersion, err := storage.UpdateAssetVersionProcessing(ctx, createdVersion.ID, createdVersion.OwnerUserID, createdVersion.ResourceVersion, store.AssetVersionProcessingMutation{
		Status: iapiserver.AssetVersionStatusReady, ExpectedCount: 2, CompletedCount: 2, TaskGroupID: "group-sse", AtomicTaskID: "task-sse",
	})
	if err != nil || finishedVersion.ResourceVersion != createdVersion.ResourceVersion+1 {
		t.Fatalf("UpdateAssetVersionProcessing() version=%#v error=%v", finishedVersion, err)
	}
	unchangedVersion, err := storage.UpdateAssetVersionProcessing(ctx, finishedVersion.ID, finishedVersion.OwnerUserID, finishedVersion.ResourceVersion, store.AssetVersionProcessingMutation{
		Status: iapiserver.AssetVersionStatusReady, ExpectedCount: 2, CompletedCount: 2, TaskGroupID: "group-sse", AtomicTaskID: "task-sse",
	})
	if err != nil || unchangedVersion.ResourceVersion != finishedVersion.ResourceVersion {
		t.Fatalf("duplicate UpdateAssetVersionProcessing() version=%#v error=%v", unchangedVersion, err)
	}

	for table, want := range map[string]int64{
		"watermill_artifact_created": 1, "watermill_artifact_processing_changed": 1,
		"watermill_artifact_registration_changed": 1, "watermill_asset_version_processing_changed": 2,
	} {
		var count int64
		if err := tx.Table(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("outbox %s count=%d, want=%d, error=%v", table, count, want, err)
		}
	}
}
