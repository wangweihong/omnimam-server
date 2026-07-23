package assetlibrary

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

type generateAssetStore struct {
	store.AssetV1Store
	completed *store.RepresentationGenerationMutation
}

func (s *generateAssetStore) GetAssetVersionDetail(context.Context, string, string) (*iapiserver.AssetVersionDetail, error) {
	return &iapiserver.AssetVersionDetail{Version: &iapiserver.AssetVersion{ObjectMeta: imachinery.ObjectMeta{ID: "version-1"}}, Representations: []*iapiserver.AssetRepresentation{{ObjectMeta: imachinery.ObjectMeta{ID: "original-1"}, RepresentationType: iapiserver.AssetRepresentationOriginal, Status: "ready"}}}, nil
}
func (s *generateAssetStore) GetRepresentation(context.Context, string, string) (*iapiserver.AssetRepresentation, *store.StoredAssetContent, error) {
	return &iapiserver.AssetRepresentation{}, &store.StoredAssetContent{BlobID: "original-blob", MIMEType: "image/png"}, nil
}
func (s *generateAssetStore) CreateRepresentationBlob(context.Context, store.StoredAssetContent) (string, error) {
	return "thumbnail-blob", nil
}
func (s *generateAssetStore) CompleteRepresentationGeneration(_ context.Context, _, _ string, mutation store.RepresentationGenerationMutation) (*iapiserver.AssetRepresentation, error) {
	s.completed = &mutation
	return &iapiserver.AssetRepresentation{ObjectMeta: imachinery.ObjectMeta{ID: "thumbnail-1"}, BlobID: mutation.BlobID}, nil
}

type generateStorage struct {
	ContentStorage
	source  []byte
	derived []byte
}

func (s *generateStorage) Open(context.Context, store.StoredAssetContent) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.source)), nil
}
func (s *generateStorage) WriteDerived(_ context.Context, _ string, reader io.Reader) (store.StoredAssetContent, error) {
	s.derived, _ = io.ReadAll(reader)
	return store.StoredAssetContent{StorageBackendID: "local", ObjectKey: "blobs/thumb", SHA256: "thumb", SizeBytes: int64(len(s.derived)), MIMEType: "image/png"}, nil
}

type inspectAssetStore struct {
	store.AssetV1Store
	mutation   *store.AssetMediaMetadataMutation
	candidates []store.AssetMediaMetadataBackfillCandidate
}

func (s *inspectAssetStore) GetAssetVersionDetail(context.Context, string, string) (*iapiserver.AssetVersionDetail, error) {
	return &iapiserver.AssetVersionDetail{
		Version: &iapiserver.AssetVersion{
			ObjectMeta: imachinery.ObjectMeta{ID: "version-1"},
			AssetID:    "asset-1",
		},
		Representations: []*iapiserver.AssetRepresentation{{
			ObjectMeta:         imachinery.ObjectMeta{ID: "original-1"},
			RepresentationType: iapiserver.AssetRepresentationOriginal,
			Status:             "ready",
		}},
	}, nil
}

func (s *inspectAssetStore) GetRepresentation(context.Context, string, string) (*iapiserver.AssetRepresentation, *store.StoredAssetContent, error) {
	return &iapiserver.AssetRepresentation{}, &store.StoredAssetContent{
		BlobID: "original-blob", MIMEType: "video/mp4", SizeBytes: 7,
	}, nil
}

func (s *inspectAssetStore) ApplyAssetMediaMetadata(_ context.Context, _, _ string, mutation store.AssetMediaMetadataMutation) error {
	s.mutation = &mutation
	return nil
}

func (s *inspectAssetStore) ListAssetMediaMetadataBackfillCandidatesAfter(
	_ context.Context,
	afterAssetID string,
	_ int,
) ([]store.AssetMediaMetadataBackfillCandidate, error) {
	if afterAssetID != "" {
		return nil, nil
	}
	return s.candidates, nil
}

type fakeMediaMetadataInspector struct {
	result MediaMetadata
}

func (i fakeMediaMetadataInspector) Inspect(context.Context, MediaMetadataRequest) (MediaMetadata, error) {
	return i.result, nil
}

type executorAssetStore struct {
	store.AssetV1Store
	artifact *iapiserver.Artifact
	version  *iapiserver.AssetVersion
	mutation store.AssetVersionProcessingMutation
}

func (s *executorAssetStore) GetArtifact(context.Context, string, string) (*iapiserver.Artifact, error) {
	return s.artifact, nil
}
func (s *executorAssetStore) UpdateArtifactProcessing(_ context.Context, _ string, _ string, _ int64, mutation store.ArtifactProcessingMutation) (*iapiserver.Artifact, error) {
	s.artifact.ProcessingStatus = mutation.ProcessingStatus
	return s.artifact, nil
}
func (s *executorAssetStore) GetAssetVersionDetail(context.Context, string, string) (*iapiserver.AssetVersionDetail, error) {
	return &iapiserver.AssetVersionDetail{Version: s.version, Representations: []*iapiserver.AssetRepresentation{{Status: "ready", Required: true}, {Status: "failed", Required: false}}}, nil
}
func (s *executorAssetStore) UpdateAssetVersionProcessing(_ context.Context, _ string, _ string, _ int64, mutation store.AssetVersionProcessingMutation) (*iapiserver.AssetVersion, error) {
	s.mutation = mutation
	s.version.Status = mutation.Status
	return s.version, nil
}

func TestArtifactProcessExecutorAdvancesReady(t *testing.T) {
	assetStore := &executorAssetStore{artifact: &iapiserver.Artifact{ProcessingStatus: iapiserver.ArtifactProcessingProcessing}}
	assetStore.artifact.ID, assetStore.artifact.ResourceVersion = "artifact-1", 2
	executor := &ArtifactProcessExecutor{store: assetStore}
	result, err := executor.Execute(context.Background(), workflowruntime.WorkerTask{Arguments: map[string]any{"artifact_id": "artifact-1", "owner_user_id": "user-1"}})
	if err != nil || result["artifact_id"] != "artifact-1" || assetStore.artifact.ProcessingStatus != iapiserver.ArtifactProcessingReady {
		t.Fatalf("result=%#v artifact=%#v error=%v", result, assetStore.artifact, err)
	}
}

func TestRepresentationFinalizePreservesOptionalFailure(t *testing.T) {
	assetStore := &executorAssetStore{version: &iapiserver.AssetVersion{ExpectedCount: 2}}
	assetStore.version.ID, assetStore.version.ResourceVersion = "version-1", 3
	executor := &RepresentationFinalizeExecutor{store: assetStore}
	_, err := executor.Execute(context.Background(), workflowruntime.WorkerTask{AtomicTaskID: "task-1", Arguments: map[string]any{"asset_version_id": "version-1", "owner_user_id": "user-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if assetStore.mutation.Status != iapiserver.AssetVersionStatusReadyWithWarnings || assetStore.mutation.CompletedCount != 1 || assetStore.mutation.FailedCount != 1 {
		t.Fatalf("mutation = %#v", assetStore.mutation)
	}
}

func TestRepresentationInspectPersistsOriginalMediaMetadata(t *testing.T) {
	assetStore := &inspectAssetStore{}
	storage := &generateStorage{source: []byte("content")}
	executor := &RepresentationInspectExecutor{
		store:     assetStore,
		storage:   storage,
		inspector: fakeMediaMetadataInspector{result: MediaMetadata{Width: 1920, Height: 1080, DurationSeconds: 12.5}},
	}

	result, err := executor.Execute(context.Background(), workflowruntime.WorkerTask{Arguments: map[string]any{
		"asset_version_id": "version-1",
		"owner_user_id":    "user-1",
		"media_type":       "video",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result["asset_version_id"] != "version-1" || result["media_type"] != "video" {
		t.Fatalf("result = %#v", result)
	}
	if assetStore.mutation == nil || assetStore.mutation.AssetID != "asset-1" ||
		assetStore.mutation.OriginalRepresentationID != "original-1" ||
		assetStore.mutation.Width != 1920 || assetStore.mutation.Height != 1080 ||
		assetStore.mutation.DurationSeconds != 12.5 {
		t.Fatalf("mutation = %#v", assetStore.mutation)
	}
}

func TestAssetMediaMetadataBackfillerRepairsMissingCurrentVersion(t *testing.T) {
	assetStore := &inspectAssetStore{candidates: []store.AssetMediaMetadataBackfillCandidate{{
		AssetID: "asset-1", AssetVersionID: "version-1", OwnerUserID: "user-1", MediaType: "video",
	}}}
	backfiller := &AssetMediaMetadataBackfiller{
		store:     assetStore,
		storage:   &generateStorage{source: []byte("content")},
		inspector: fakeMediaMetadataInspector{result: MediaMetadata{Width: 1280, Height: 720, DurationSeconds: 9}},
	}

	summary, err := backfiller.Run(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Scanned != 1 || summary.Updated != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %#v", summary)
	}
	if assetStore.mutation == nil || assetStore.mutation.Width != 1280 ||
		assetStore.mutation.Height != 720 || assetStore.mutation.DurationSeconds != 9 {
		t.Fatalf("mutation = %#v", assetStore.mutation)
	}
}

func TestRepresentationGenerateCreatesThumbnailRepresentation(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 640, 320))
	source.Set(10, 10, color.RGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	assetStore := &generateAssetStore{}
	storage := &generateStorage{source: encoded.Bytes()}
	executor := &RepresentationGenerateExecutor{store: assetStore, storage: storage, generators: NewThumbnailGenerators(ImageThumbnailGenerator{}, nil)}
	result, err := executor.Execute(context.Background(), workflowruntime.WorkerTask{Arguments: map[string]any{
		"asset_version_id": "version-1", "owner_user_id": "user-1", "representation_type": "thumbnail",
		"media_type": "image", "profile": "list-320", "profile_version": "v1", "required": false,
	}})
	if err != nil {
		t.Fatal(err)
	}
	thumbnail, err := png.Decode(bytes.NewReader(storage.derived))
	if err != nil {
		t.Fatal(err)
	}
	if bounds := thumbnail.Bounds(); bounds.Dx() != 320 || bounds.Dy() != 160 {
		t.Fatalf("thumbnail bounds = %v", bounds)
	}
	if result["representation_id"] != "thumbnail-1" || assetStore.completed == nil || assetStore.completed.BlobID != "thumbnail-blob" || assetStore.completed.Status != "ready" {
		t.Fatalf("result=%#v mutation=%#v", result, assetStore.completed)
	}
}
