package taskcenter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
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
			"run-1": {ObjectMeta: imachinery.ObjectMeta{ID: "run-1"}, Status: iapiserver.TaskRunStatusRunning},
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
				ObjectMeta: imachinery.ObjectMeta{ID: "run-1"},
				Status:     iapiserver.TaskRunStatusFailed,
				Progress:   1,
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
				ObjectMeta:     imachinery.ObjectMeta{ID: "def-1", Name: "atomic"},
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
				ObjectMeta:     imachinery.ObjectMeta{ID: "def-1", Name: "atomic"},
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
				ObjectMeta:     imachinery.ObjectMeta{ID: "def-1", Name: "atomic"},
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

func TestCreateRunRequiresApplicationIdempotencyPair(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		definitions: map[string]*iapiserver.TaskDefinition{
			"def-1": {
				ObjectMeta:     imachinery.ObjectMeta{ID: "def-1", Name: "atomic"},
				DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
			},
		},
		runs: map[string]*iapiserver.TaskRun{},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	_, err := srv.CreateRun(context.Background(), &iapiserver.TaskRunCreateRequest{
		DefinitionType:   iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:     "def-1",
		ApplicationRunID: "app-run-1",
	})
	if err == nil {
		t.Fatal("expected incomplete application idempotency pair to fail")
	}
}

func TestCreateRunReturnsExistingForSameApplicationIdempotencyKey(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		definitions: map[string]*iapiserver.TaskDefinition{
			"def-1": {
				ObjectMeta:     imachinery.ObjectMeta{ID: "def-1", Name: "atomic"},
				DefinitionType: iapiserver.TaskDefinitionTypeAtomic,
			},
		},
		runs: map[string]*iapiserver.TaskRun{},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	req := &iapiserver.TaskRunCreateRequest{
		DefinitionType:   iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:     "def-1",
		ApplicationRunID: "app-run-1",
		IdempotencyKey:   "submit-1",
	}
	first, err := srv.CreateRun(context.Background(), req)
	if err != nil {
		t.Fatalf("first create run: %v", err)
	}
	second, err := srv.CreateRun(context.Background(), req)
	if err != nil {
		t.Fatalf("idempotent create run: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("second run id = %s, want %s", second.ID, first.ID)
	}
	if taskStore.eventCount != 1 {
		t.Fatalf("created event count = %d, want 1", taskStore.eventCount)
	}
}

func TestCreateRunRejectsDifferentRequestForSameApplicationIdempotencyKey(t *testing.T) {
	taskStore := &fakeTaskCenterStore{
		definitions: map[string]*iapiserver.TaskDefinition{
			"def-1": {ObjectMeta: imachinery.ObjectMeta{ID: "def-1", Name: "one"}, DefinitionType: iapiserver.TaskDefinitionTypeAtomic},
			"def-2": {ObjectMeta: imachinery.ObjectMeta{ID: "def-2", Name: "two"}, DefinitionType: iapiserver.TaskDefinitionTypeAtomic},
		},
		runs: map[string]*iapiserver.TaskRun{},
	}
	srv := NewService(&fakeFactory{taskCenter: taskStore})
	first := &iapiserver.TaskRunCreateRequest{
		DefinitionType:   iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:     "def-1",
		ApplicationRunID: "app-run-1",
		IdempotencyKey:   "submit-1",
	}
	if _, err := srv.CreateRun(context.Background(), first); err != nil {
		t.Fatalf("first create run: %v", err)
	}
	second := *first
	second.DefinitionID = "def-2"
	_, err := srv.CreateRun(context.Background(), &second)
	if err == nil {
		t.Fatal("expected idempotency conflict")
	}
	status := toolboxerrors.ToStatus(err)
	if status.Code != code.ErrTaskRunIdempotencyConflict {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrTaskRunIdempotencyConflict)
	}
}

func TestTaskCenterObjectMetaUsesContractTimestampFields(t *testing.T) {
	data, err := json.Marshal(iapiserver.TaskRun{
		ObjectMeta: imachinery.ObjectMeta{ID: "run-1"},
	})
	if err != nil {
		t.Fatalf("marshal task run: %v", err)
	}
	payload := string(data)
	if !strings.Contains(payload, `"created_at"`) {
		t.Fatalf("json = %s, want created_at field", payload)
	}
	if strings.Contains(payload, `"createdAt"`) {
		t.Fatalf("json = %s, should not contain createdAt field", payload)
	}
}

func TestTaskRunEventCarriesApplicationProjectionKey(t *testing.T) {
	run := &iapiserver.TaskRun{
		ObjectMeta:       imachinery.ObjectMeta{ID: "run-1", ResourceVersion: 7},
		ApplicationRunID: "app-run-1",
		DefinitionType:   iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:     "application.execute",
		Status:           iapiserver.TaskRunStatusRunning,
	}
	event := newTaskRunEvent(
		run,
		iapiserver.TaskCenterEventProgressUpdated,
		iapiserver.TaskRunStatusRunning,
		iapiserver.TaskRunStatusRunning,
	)
	if event.Payload["application_run_id"] != "app-run-1" {
		t.Fatalf("application_run_id = %v, want app-run-1", event.Payload["application_run_id"])
	}
	if event.Payload["resource_version"] != int64(7) {
		t.Fatalf("resource_version = %v, want 7", event.Payload["resource_version"])
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
	eventCount  int
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

func (s *fakeTaskCenterStore) AddRunIdempotent(
	_ context.Context,
	data *iapiserver.TaskRun,
) (*iapiserver.TaskRun, bool, error) {
	for _, existing := range s.runs {
		if data.ApplicationRunID != "" &&
			existing.ApplicationRunID == data.ApplicationRunID &&
			existing.IdempotencyKey == data.IdempotencyKey {
			if existing.DefinitionType != data.DefinitionType || existing.DefinitionID != data.DefinitionID {
				return nil, false, toolboxerrors.NewStatusF(
					code.ErrTaskRunIdempotencyConflict,
					"idempotency conflict",
				)
			}
			cloned := *existing
			return &cloned, false, nil
		}
	}
	created, err := s.AddRun(context.Background(), data)
	return created, err == nil, err
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
	s.eventCount++
	return data, nil
}

func errNotFound() error {
	return context.Canceled
}
