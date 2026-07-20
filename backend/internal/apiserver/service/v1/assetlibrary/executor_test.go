package assetlibrary

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

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
