//go:build integration

package postgresql

import (
	"context"
	"errors"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestPostgresAssetContractLifecycle(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{&iapiserver.UserAsset{}, &iapiserver.AssetBlob{}, &iapiserver.Artifact{}, &iapiserver.AssetVersion{},
		&iapiserver.AssetRepresentation{}, &iapiserver.ArtifactAssetRegistration{}, &iapiserver.UserAssetLabel{},
		&iapiserver.UserAssetTag{}, &iapiserver.AssetUploadSession{}, &iapiserver.AssetCollection{}, &iapiserver.AssetCollectionItem{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	ds := &datastore{db: db}
	if err := ds.ensureAssetLibraryScheme(); err != nil {
		t.Fatal(err)
	}
	if err := ds.ensureOutboxScheme(); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	assetStore := newAssetV1Store(&datastore{db: tx})
	ctx := context.Background()

	detail, err := assetStore.CreateCanonicalAsset(ctx, "user-1", &iapiserver.CreateCanonicalAssetRequest{
		DisplayName: "Prompt", MediaType: "prompt", CanonicalContent: map[string]any{"positive_prompt": "city"},
		Labels: map[string]string{"project": "demo"}, Tags: []string{"approved"}, ProfileVersion: "canonical-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Asset.CurrentVersionID == "" || len(detail.Versions) != 1 || len(detail.Representations) != 1 {
		t.Fatalf("canonical asset detail = %#v", detail)
	}
	if _, err := assetStore.GetUserAsset(ctx, "user-2", detail.Asset.ID, false); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user read error = %v", err)
	}

	directDelete, err := assetStore.CreateCanonicalAsset(ctx, "user-1", &iapiserver.CreateCanonicalAssetRequest{
		DisplayName: "Direct delete", MediaType: "text", CanonicalContent: map[string]any{"text": "remove"}, ProfileVersion: "canonical-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := assetStore.HardDeleteUserAsset(ctx, "user-2", directDelete.Asset.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user hard delete error = %v", err)
	}
	if result, _, err := assetStore.HardDeleteUserAsset(ctx, "user-1", directDelete.Asset.ID); err != nil || !result.Deleted {
		t.Fatalf("direct hard delete result=%#v error=%v", result, err)
	}

	trashAsset, err := assetStore.CreateCanonicalAsset(ctx, "user-1", &iapiserver.CreateCanonicalAssetRequest{
		DisplayName: "Trash", MediaType: "text", CanonicalContent: map[string]any{"text": "trash"}, ProfileVersion: "canonical-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assetStore.SetUserAssetDeleted(ctx, "user-1", trashAsset.Asset.ID, true); err != nil {
		t.Fatal(err)
	}
	otherTrashAsset, err := assetStore.CreateCanonicalAsset(ctx, "user-2", &iapiserver.CreateCanonicalAssetRequest{
		DisplayName: "Other trash", MediaType: "text", CanonicalContent: map[string]any{"text": "other"}, ProfileVersion: "canonical-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assetStore.SetUserAssetDeleted(ctx, "user-2", otherTrashAsset.Asset.ID, true); err != nil {
		t.Fatal(err)
	}
	deletedIDs, err := assetStore.ListDeletedUserAssetIDs(ctx, "user-1")
	if err != nil || len(deletedIDs) != 1 || deletedIDs[0] != trashAsset.Asset.ID {
		t.Fatalf("deleted ids=%#v error=%v", deletedIDs, err)
	}

	collection, err := assetStore.CreateCollection(ctx, "user-1", &iapiserver.CreateCollectionRequest{Name: "Project"})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := assetStore.AddCollectionItems(ctx, "user-1", collection.ID, []iapiserver.AddCollectionItem{{AssetID: detail.Asset.ID, PinnedVersionID: detail.Versions[0].ID}})
	if err != nil || batch.Success != 1 {
		t.Fatalf("collection batch = %#v error=%v", batch, err)
	}

	artifact := &iapiserver.Artifact{OwnerUserID: "user-1", ProducerType: "atomic_task", ProducerID: "task-1",
		ProducerIdempotencyKey: "task-1:images:0", OutputKey: "images", ArtifactType: "image", MediaType: "image",
		SavePolicy: iapiserver.ArtifactSaveManual, ProcessingProfileVersion: "default-v1", Metadata: map[string]any{}}
	created, inserted, err := assetStore.CreateArtifact(ctx, artifact)
	if err != nil || !inserted {
		t.Fatalf("CreateArtifact inserted=%t error=%v", inserted, err)
	}
	created, err = assetStore.StoreArtifactContent(ctx, "user-1", created.ID, store.StoredAssetContent{StorageBackendID: "local-main", ObjectKey: "blobs/aa/aabb", SHA256: "aabb", SizeBytes: 10, MIMEType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	created, err = assetStore.UpdateArtifactProcessing(ctx, created.ID, "user-1", created.ResourceVersion, store.ArtifactProcessingMutation{ChangeType: "ready", ProcessingStatus: iapiserver.ArtifactProcessingReady})
	if err != nil {
		t.Fatal(err)
	}
	registration, err := assetStore.RegisterArtifactLifecycle(ctx, "user-1", created.ID, &iapiserver.RegisterArtifactRequest{Mode: "create_asset", Name: "Generated image"})
	if err != nil {
		t.Fatal(err)
	}
	if registration.AssetVersion == nil || registration.AssetVersion.SourceRefID != created.ID {
		t.Fatalf("registration = %#v", registration)
	}

	for table, minimum := range map[string]int64{"watermill_artifact_created": 1, "watermill_artifact_processing_changed": 2,
		"watermill_artifact_registration_changed": 1, "watermill_asset_version_representation_requested": 2} {
		var count int64
		if err := tx.Table(table).Count(&count).Error; err != nil || count < minimum {
			t.Fatalf("outbox %s count=%d minimum=%d error=%v", table, count, minimum, err)
		}
	}
}
