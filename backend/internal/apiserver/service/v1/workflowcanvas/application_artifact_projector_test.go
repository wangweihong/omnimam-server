package workflowcanvas

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type applicationArtifactProjectionStore struct {
	store.WorkflowCanvasStore
	projection *store.CanvasApplicationArtifactProjection
	calls      int
	err        error
}

type applicationArtifactTaskStore struct {
	store.TaskCenterStore
	task  *iapiserver.AtomicTask
	err   error
	calls int
}

func (s *applicationArtifactTaskStore) GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error) {
	s.calls++
	return s.task, s.err
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
	tasks := &applicationArtifactTaskStore{task: &iapiserver.AtomicTask{CanvasRunID: "canvas-run-1"}}
	err := NewApplicationArtifactProjector(tasks, target).Project(t.Context(), []byte(`{
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

func TestApplicationArtifactProjectorSkipsIndependentApplicationRun(t *testing.T) {
	target := &applicationArtifactProjectionStore{}
	tasks := &applicationArtifactTaskStore{task: &iapiserver.AtomicTask{}}
	err := NewApplicationArtifactProjector(tasks, target).Project(t.Context(), []byte(`{
		"atomic_task_id":"task-1",
		"output_key":"image",
		"sequence":0,
		"artifact_id":"artifact-1",
		"artifact_resource_version":1
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if tasks.calls != 1 || target.calls != 0 {
		t.Fatalf("task calls=%d projection calls=%d", tasks.calls, target.calls)
	}
}

func TestApplicationArtifactProjectorRetriesTaskLookupFailure(t *testing.T) {
	target := &applicationArtifactProjectionStore{}
	tasks := &applicationArtifactTaskStore{err: stderrors.New("temporary task lookup failure")}
	err := NewApplicationArtifactProjector(tasks, target).Project(t.Context(), []byte(`{
		"atomic_task_id":"task-1",
		"output_key":"image",
		"sequence":0,
		"artifact_id":"artifact-1",
		"artifact_resource_version":1
	}`))
	if err == nil {
		t.Fatal("task lookup failure was ignored")
	}
	if tasks.calls != 1 || target.calls != 0 {
		t.Fatalf("task calls=%d projection calls=%d", tasks.calls, target.calls)
	}
}

func TestApplicationArtifactProjectorRejectsIncompleteEvent(t *testing.T) {
	target := &applicationArtifactProjectionStore{}
	tasks := &applicationArtifactTaskStore{}
	err := NewApplicationArtifactProjector(tasks, target).Project(t.Context(), []byte(`{"atomic_task_id":"task-1"}`))
	if err == nil {
		t.Fatal("incomplete event was accepted")
	}
	if target.calls != 0 {
		t.Fatalf("store calls = %d", target.calls)
	}
	if tasks.calls != 0 {
		t.Fatalf("task store calls = %d", tasks.calls)
	}
}
