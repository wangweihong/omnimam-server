package taskcenter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestCreateTaskGroupRejectsInvalidType(t *testing.T) {
	srv := NewService(&fakeFactory{})
	_, err := srv.CreateTaskGroup(context.Background(), &iapiserver.TaskGroupCreateRequest{
		Name:      "invalid-group",
		GroupType: "DAG",
		Children: []iapiserver.TaskDefinitionChild{
			{DefinitionType: iapiserver.TaskDefinitionTypeAtomic, DefinitionID: "atomic-1"},
		},
	})
	if err == nil {
		t.Fatal("expected invalid task group type to fail")
	}
}

func TestValidateDAGRejectsCycle(t *testing.T) {
	err := validateDAG(
		[]iapiserver.DAGNode{
			{NodeID: "a", Name: "A"},
			{NodeID: "b", Name: "B"},
		},
		[]iapiserver.DAGEdge{
			{FromNodeID: "a", ToNodeID: "b"},
			{FromNodeID: "b", ToNodeID: "a"},
		},
	)
	if err == nil {
		t.Fatal("expected cyclic dag to fail")
	}
	status := toolboxerrors.ToStatus(err)
	if status.Code != code.ErrTaskDAGCycleDetected {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrTaskDAGCycleDetected)
	}
	if status.HTTPStatus != 200 {
		t.Fatalf("http status = %d, want 200", status.HTTPStatus)
	}
}

func TestDeleteRunRequiresTerminalStatus(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		runs: map[string]*iapiserver.TaskRun{
			"run-1": {TaskCenterMeta: iapiserver.TaskCenterMeta{ID: "run-1"}, Status: iapiserver.TaskRunStatusRunning},
		},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	_, err := srv.DeleteRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("expected running task run delete to fail")
	}
	if taskStore.deleted {
		t.Fatal("running task run should not be soft deleted")
	}
}

func TestRetryRunMovesFailedRunToReady(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		runs: map[string]*iapiserver.TaskRun{
			"run-1": {
				TaskCenterMeta: iapiserver.TaskCenterMeta{ID: "run-1"},
				Status:         iapiserver.TaskRunStatusFailed,
				Progress:       1,
			},
		},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	run, err := srv.RetryRun(context.Background(), &iapiserver.RetryTaskRunRequest{RunID: "run-1"})
	if err != nil {
		t.Fatalf("retry failed task run: %v", err)
	}
	if run.Status != iapiserver.TaskRunStatusReady {
		t.Fatalf("status = %s, want %s", run.Status, iapiserver.TaskRunStatusReady)
	}
	if run.Progress != 0 {
		t.Fatalf("progress = %v, want 0", run.Progress)
	}
}

func TestCreateRunUsesDefinitionRetryPolicy(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		definitions: map[string]*iapiserver.TaskDefinition{
			"def-1": {
				TaskCenterMeta: iapiserver.TaskCenterMeta{ID: "def-1", Name: "atomic"},
				DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
				RetryPolicy:    iapiserver.RetryPolicy{MaxRetries: 2},
				ProjectID:      "project-a",
				Namespace:      "ns-a",
			},
		},
		runs: map[string]*iapiserver.TaskRun{},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	run, err := srv.CreateRun(context.Background(), &iapiserver.TaskRunCreateRequest{
		DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:   "def-1",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.MaxAttempts != 3 {
		t.Fatalf("max attempts = %d, want 3", run.MaxAttempts)
	}
	if run.ProjectID != "project-a" || run.Namespace != "ns-a" {
		t.Fatalf("scope = %s/%s, want project-a/ns-a", run.ProjectID, run.Namespace)
	}
}

func TestCreateRunRejectsInfiniteRetryWithoutExitProtection(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		definitions: map[string]*iapiserver.TaskDefinition{
			"def-1": {
				TaskCenterMeta: iapiserver.TaskCenterMeta{ID: "def-1", Name: "atomic"},
				DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
				RetryPolicy:    iapiserver.RetryPolicy{MaxRetries: -1},
			},
		},
		runs: map[string]*iapiserver.TaskRun{},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	_, err := srv.CreateRun(context.Background(), &iapiserver.TaskRunCreateRequest{
		DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:   "def-1",
	})
	if err == nil {
		t.Fatal("expected infinite retry without exit protection to fail")
	}
	status := toolboxerrors.ToStatus(err)
	if status.Code != code.ErrTaskRetryPolicyInvalid {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrTaskRetryPolicyInvalid)
	}
	if status.HTTPStatus != 200 {
		t.Fatalf("http status = %d, want 200", status.HTTPStatus)
	}
}

func TestCreateRunAllowsInfiniteRetryWithOverallTimeout(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		definitions: map[string]*iapiserver.TaskDefinition{
			"def-1": {
				TaskCenterMeta: iapiserver.TaskCenterMeta{ID: "def-1", Name: "atomic"},
				DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
				RetryPolicy:    iapiserver.RetryPolicy{MaxRetries: -1},
				TimeoutPolicy:  iapiserver.TimeoutPolicy{OverallTimeout: "1h"},
			},
		},
		runs: map[string]*iapiserver.TaskRun{},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	run, err := srv.CreateRun(context.Background(), &iapiserver.TaskRunCreateRequest{
		DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:   "def-1",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.MaxAttempts != -1 {
		t.Fatalf("max attempts = %d, want -1", run.MaxAttempts)
	}
	if run.TimeoutAt.IsZero() {
		t.Fatal("timeout_at should be set for infinite retry guarded by overall timeout")
	}
}

func TestTaskCenterMetaUsesContractTimestampFields(t *testing.T) {
	data, err := json.Marshal(iapiserver.TaskRun{
		TaskCenterMeta: iapiserver.TaskCenterMeta{ID: "run-1"},
	})
	if err != nil {
		t.Fatalf("marshal task run: %v", err)
	}
	payload := string(data)
	if !strings.Contains(payload, `"createdAt"`) {
		t.Fatalf("json = %s, want createdAt field", payload)
	}
	if strings.Contains(payload, `"created_at"`) {
		t.Fatalf("json = %s, should not contain created_at field", payload)
	}
}

type fakeFactory struct {
	store.Factory
	taskCenter store.TaskCenterStore
}

func (f *fakeFactory) TaskCenters() store.TaskCenterStore { return f.taskCenter }

type fakeTaskCenterStore struct {
	store.TaskCenterStore
	definitions map[string]*iapiserver.TaskDefinition
	runs        map[string]*iapiserver.TaskRun
	deleted     bool
}

func (s *fakeTaskCenterStore) GetDefinition(
	_ context.Context,
	definitionType, id string,
) (*iapiserver.TaskDefinition, error) {
	definition := s.definitions[id]
	if definition == nil || definition.DefinitionType != definitionType {
		return nil, errNotFound()
	}
	cloned := *definition
	return &cloned, nil
}

func (s *fakeTaskCenterStore) AddRun(_ context.Context, data *iapiserver.TaskRun) (*iapiserver.TaskRun, error) {
	if data.ID == "" {
		data.ID = "run-created"
	}
	cloned := *data
	s.runs[data.ID] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *fakeTaskCenterStore) GetRun(_ context.Context, id string) (*iapiserver.TaskRun, error) {
	run := s.runs[id]
	if run == nil {
		return nil, errNotFound()
	}
	cloned := *run
	return &cloned, nil
}

func (s *fakeTaskCenterStore) UpdateRun(_ context.Context, data *iapiserver.TaskRun) (*iapiserver.TaskRun, error) {
	cloned := *data
	s.runs[data.ID] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *fakeTaskCenterStore) SoftDeleteRun(_ context.Context, _ string) error {
	s.deleted = true
	return nil
}

func (s *fakeTaskCenterStore) AddEvent(
	_ context.Context,
	data *iapiserver.TaskRunEvent,
) (*iapiserver.TaskRunEvent, error) {
	return data, nil
}

func errNotFound() error {
	return context.Canceled
}
