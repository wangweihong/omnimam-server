//go:build integration

package postgresql

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestPostgresApplyAssetMediaMetadataProtectsCurrentVersionProjection(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	assetID, versionID, representationID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	asset := &iapiserver.UserAsset{
		ObjectMeta:       imachinery.ObjectMeta{ID: assetID, Name: "metadata-test"},
		OwnerUserID:      "metadata-test-user",
		DisplayName:      "metadata-test",
		MediaType:        iapiserver.AssetMediaTypeVideo,
		SizeBytes:        100,
		SourceType:       "upload",
		Status:           iapiserver.AssetStatusActive,
		CurrentVersionID: "newer-version",
		ThumbnailStatus:  "none",
		PreviewStatus:    "none",
	}
	if err := tx.Create(asset).Error; err != nil {
		t.Fatal(err)
	}
	version := &iapiserver.AssetVersion{
		ObjectMeta:      imachinery.ObjectMeta{ID: versionID, Name: "version-1"},
		AssetID:         assetID,
		OwnerUserID:     asset.OwnerUserID,
		VersionNo:       1,
		Status:          iapiserver.AssetVersionStatusProcessing,
		SourceType:      "upload",
		Content:         map[string]any{},
		Metadata:        map[string]any{"existing": "preserved"},
		ProcessingError: map[string]any{},
		ProfileVersion:  "v1",
	}
	if err := tx.Create(version).Error; err != nil {
		t.Fatal(err)
	}
	representation := &iapiserver.AssetRepresentation{
		ObjectMeta:         imachinery.ObjectMeta{ID: representationID, Name: "original"},
		AssetVersionID:     versionID,
		OwnerUserID:        asset.OwnerUserID,
		RepresentationType: iapiserver.AssetRepresentationOriginal,
		Profile:            "default",
		ProfileVersion:     "v1",
		Content:            map[string]any{},
		Metadata:           map[string]any{},
		Status:             "ready",
		Required:           true,
	}
	if err := tx.Create(representation).Error; err != nil {
		t.Fatal(err)
	}

	assetStore := newAssetV1Store(&datastore{db: tx})
	mutation := store.AssetMediaMetadataMutation{
		AssetID: assetID, OriginalRepresentationID: representationID,
		MIMEType: "video/mp4", SizeBytes: 100, Width: 1920, Height: 1080, DurationSeconds: 12.5,
	}
	if err := assetStore.ApplyAssetMediaMetadata(context.Background(), asset.OwnerUserID, versionID, mutation); err != nil {
		t.Fatal(err)
	}
	var staleProjection iapiserver.UserAsset
	if err := tx.First(&staleProjection, "id = ?", assetID).Error; err != nil {
		t.Fatal(err)
	}
	if staleProjection.Width != 0 || staleProjection.Height != 0 || staleProjection.DurationSeconds != 0 {
		t.Fatalf("stale version overwrote current projection: %#v", staleProjection)
	}

	if err := tx.Model(&iapiserver.UserAsset{}).Where("id = ?", assetID).Update("current_version_id", versionID).Error; err != nil {
		t.Fatal(err)
	}
	if err := assetStore.ApplyAssetMediaMetadata(context.Background(), asset.OwnerUserID, versionID, mutation); err != nil {
		t.Fatal(err)
	}
	var updatedAsset iapiserver.UserAsset
	if err := tx.First(&updatedAsset, "id = ?", assetID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedAsset.Width != 1920 || updatedAsset.Height != 1080 || updatedAsset.DurationSeconds != 12.5 {
		t.Fatalf("updated asset = %#v", updatedAsset)
	}
	var updatedVersion iapiserver.AssetVersion
	if err := tx.First(&updatedVersion, "id = ?", versionID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedVersion.Metadata["existing"] != "preserved" || updatedVersion.Metadata["width"] != float64(1920) {
		t.Fatalf("updated version metadata = %#v", updatedVersion.Metadata)
	}
	var updatedRepresentation iapiserver.AssetRepresentation
	if err := tx.First(&updatedRepresentation, "id = ?", representationID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedRepresentation.Metadata["duration_seconds"] != 12.5 {
		t.Fatalf("updated representation metadata = %#v", updatedRepresentation.Metadata)
	}
}
