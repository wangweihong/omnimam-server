package workflowcanvas

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type applicationArtifactProjectionStore struct {
	store.WorkflowCanvasStore
	projection *store.CanvasApplicationArtifactProjection
	calls      int
	err        error
}

func (s *applicationArtifactProjectionStore) ProjectCanvasApplicationArtifact(
	_ context.Context,
	projection *store.CanvasApplicationArtifactProjection,
) (bool, error) {
	s.calls++
	s.projection = projection
	return true, s.err
}

func TestApplicationArtifactProjectorPreservesOutputIdentity(t *testing.T) {
	target := &applicationArtifactProjectionStore{}
	err := NewApplicationArtifactProjector(target).Project(t.Context(), []byte(`{
		"atomic_task_id":"task-1",
		"output_key":"images",
		"sequence":2,
		"artifact_id":"artifact-1",
		"media_type":"image",
		"artifact_processing_status":"ready",
		"artifact_resource_version":7
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if target.calls != 1 || target.projection == nil ||
		target.projection.AtomicTaskID != "task-1" ||
		target.projection.OutputKey != "images" ||
		target.projection.Sequence != 2 ||
		target.projection.ArtifactID != "artifact-1" ||
		target.projection.ArtifactResourceVersion != 7 {
		t.Fatalf("projection = %#v calls=%d", target.projection, target.calls)
	}
}

func TestApplicationArtifactProjectorRejectsIncompleteEvent(t *testing.T) {
	target := &applicationArtifactProjectionStore{}
	err := NewApplicationArtifactProjector(target).Project(t.Context(), []byte(`{"atomic_task_id":"task-1"}`))
	if err == nil {
		t.Fatal("incomplete event was accepted")
	}
	if target.calls != 0 {
		t.Fatalf("store calls = %d", target.calls)
	}
}
