package assetlibrary

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type representationBackfillStore struct {
	store.AssetV1Store
	items     []store.RepresentationBackfillCandidate
	prepared  []string
	completed []store.RepresentationGenerationMutation
}

func (s *representationBackfillStore) ListRepresentationBackfillCandidatesAfter(context.Context, string, int) ([]store.RepresentationBackfillCandidate, error) {
	return s.items, nil
}
func (s *representationBackfillStore) PrepareRepresentationBackfill(_ context.Context, _ string, versionID string, _ int) error {
	s.prepared = append(s.prepared, versionID)
	return nil
}
func (s *representationBackfillStore) CompleteRepresentationGeneration(_ context.Context, _, _ string, mutation store.RepresentationGenerationMutation) (*iapiserver.AssetRepresentation, error) {
	s.completed = append(s.completed, mutation)
	return &iapiserver.AssetRepresentation{}, nil
}

func TestRepresentationBackfillCreatesMissingVideoAction(t *testing.T) {
	assetStore := &representationBackfillStore{items: []store.RepresentationBackfillCandidate{{AssetID: "asset-1", AssetVersionID: "version-1", OwnerUserID: "user-1", MediaType: "video", ProfileVersion: "default-v1", SourceAvailable: true}}}
	handler := &RepresentationBackfillHandler{store: assetStore, policy: DefaultRepresentationPolicy{}}
	result, err := handler.Reconcile(context.Background(), taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, Config: map[string]any{"max_actions_per_run": 100}, MaxItemsPerRun: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Findings != 1 || len(result.Actions) != 1 || len(assetStore.prepared) != 1 {
		t.Fatalf("result=%#v prepared=%#v", result, assetStore.prepared)
	}
	action := result.Actions[0]
	if action.IdempotencyKey != "asset-representation:version-1:thumbnail:default-v1" || action.Arguments["media_type"] != "video" || action.RetryPolicy.MaxAttempts != 3 {
		t.Fatalf("action = %#v", action)
	}
}

func TestRepresentationBackfillSkipsHealthyAndMarksIrrecoverable(t *testing.T) {
	assetStore := &representationBackfillStore{items: []store.RepresentationBackfillCandidate{
		{AssetVersionID: "healthy", MediaType: "video", ProfileVersion: "default-v1", RepresentationStatus: "ready", RepresentationBlobOK: true, SourceAvailable: true},
		{AssetVersionID: "missing-source", OwnerUserID: "user-1", MediaType: "video", ProfileVersion: "default-v1"},
	}}
	handler := &RepresentationBackfillHandler{store: assetStore, policy: DefaultRepresentationPolicy{}}
	result, err := handler.Reconcile(context.Background(), taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, Config: map[string]any{}, MaxItemsPerRun: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 2 || result.Findings != 1 || len(result.Actions) != 0 || len(assetStore.completed) != 1 || assetStore.completed[0].Status != "irreparable" {
		t.Fatalf("result=%#v completed=%#v", result, assetStore.completed)
	}
}

func TestRepresentationBackfillHonorsActionLimit(t *testing.T) {
	assetStore := &representationBackfillStore{items: []store.RepresentationBackfillCandidate{
		{AssetID: "asset-1", AssetVersionID: "version-1", OwnerUserID: "user-1", MediaType: "video", ProfileVersion: "v1", SourceAvailable: true},
		{AssetID: "asset-2", AssetVersionID: "version-2", OwnerUserID: "user-1", MediaType: "video", ProfileVersion: "v1", SourceAvailable: true},
	}}
	handler := &RepresentationBackfillHandler{store: assetStore, policy: DefaultRepresentationPolicy{}}
	result, err := handler.Reconcile(context.Background(), taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, Config: map[string]any{"max_actions_per_run": 1}, MaxItemsPerRun: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 1 || result.Deferred != 1 {
		t.Fatalf("result = %#v", result)
	}
}
