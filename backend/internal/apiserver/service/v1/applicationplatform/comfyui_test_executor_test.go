package applicationplatform

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"gorm.io/gorm"
)

type workflowTestRunStore struct {
	store.ApplicationPlatformStore
	workflow *iapiserver.ComfyUIWorkflow
	engine   *iapiserver.EngineInstance
	catalog  *iapiserver.ComfyUIEngineObjectInfo
	run      *iapiserver.ComfyUIWorkflowTestRun
	boundDAG string
}

func (s *workflowTestRunStore) GetComfyUIWorkflow(context.Context, string) (*iapiserver.ComfyUIWorkflow, error) {
	return s.workflow, nil
}

func (s *workflowTestRunStore) GetComfyUIWorkflowTestRunByIdempotency(context.Context, string, string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *workflowTestRunStore) GetEngineInstance(context.Context, string) (*iapiserver.EngineInstance, error) {
	return s.engine, nil
}

func (s *workflowTestRunStore) GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfo, error) {
	return s.catalog, nil
}

func (s *workflowTestRunStore) AddComfyUIWorkflowValidation(_ context.Context, validation *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error) {
	validation.ID = "validation-1"
	return validation, nil
}

func (s *workflowTestRunStore) AddComfyUIWorkflowTestRun(_ context.Context, run *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	s.run = run
	return run, nil
}

func (s *workflowTestRunStore) BindComfyUIWorkflowTestRunDAG(_ context.Context, testRunID, dagTaskGroupID string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	s.boundDAG = dagTaskGroupID
	s.run.DAGTaskGroupID = &dagTaskGroupID
	s.run.TaskCreationStatus = iapiserver.TaskCreationCreated
	return s.run, nil
}

type workflowTestRunTasks struct {
	taskcenter.TaskCenterSrv
	onCreate func()
}

func (s *workflowTestRunTasks) CreateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error) {
	if s.onCreate != nil {
		s.onCreate()
	}
	group := &iapiserver.DAGTaskGroup{Status: iapiserver.TaskGroupStatusRunning}
	group.ID = "dag-1"
	return group, nil
}

func (s *workflowTestRunTasks) GetDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error) {
	group := &iapiserver.DAGTaskGroup{Status: iapiserver.TaskGroupStatusRunning, Nodes: testRunDAGNodes()}
	group.ID = "dag-1"
	return group, nil
}

func (s *workflowTestRunTasks) GetDAGTaskGroupSummaries(_ context.Context, ids []string) (map[string]*iapiserver.DAGTaskGroupSummary, error) {
	result := make(map[string]*iapiserver.DAGTaskGroupSummary, len(ids))
	for _, id := range ids {
		result[id] = &iapiserver.DAGTaskGroupSummary{ID: id, Status: iapiserver.TaskGroupStatusRunning, Progress: 0.5}
	}
	return result, nil
}

func (s *workflowTestRunTasks) ListDAGTaskGroupTasks(context.Context, string, *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error) {
	return &iapiserver.AtomicTaskListResponse{}, nil
}

func TestCollectTestOutputsFiltersSelectedNodes(t *testing.T) {
	outputs := map[string]any{
		"save-a": map[string]any{"images": []any{map[string]any{"filename": "a.png"}}},
		"save-b": map[string]any{"images": []any{map[string]any{"filename": "b.png"}}},
	}
	selections := []iapiserver.ComfyUIWorkflowTestOutputSelection{
		{NodeID: "save-b", OutputIndex: 0},
		{NodeID: "save-b", OutputIndex: 1},
	}

	result := collectTestOutputs(outputs, selections)
	if len(result) != 1 || result[0].NodeID != "save-b" {
		t.Fatalf("collected outputs = %#v, want only save-b", result)
	}
}

func TestCollectTestOutputsKeepsLegacyUnfilteredBehavior(t *testing.T) {
	outputs := map[string]any{
		"text-a": map[string]any{"text": []any{"first"}},
		"text-b": map[string]any{"text": []any{"second"}},
	}

	result := collectTestOutputs(outputs, nil)
	if len(result) != 2 {
		t.Fatalf("legacy collected output count = %d, want 2", len(result))
	}
}

func TestValidateTestOutputs(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{OutputCandidates: []iapiserver.ComfyUIWorkflowOutputCandidate{
		{NodeID: "save", OutputIndex: 0, Extractable: true, MediaType: "image"},
		{NodeID: "model", OutputIndex: 0, Extractable: true, MediaType: "other"},
	}}
	tests := []struct {
		name    string
		outputs []iapiserver.ComfyUIWorkflowTestOutputSelection
		valid   bool
	}{
		{name: "valid output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "save", OutputIndex: 0}}, valid: true},
		{name: "empty outputs"},
		{name: "unknown output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "missing", OutputIndex: 0}}},
		{name: "non preview output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "model", OutputIndex: 0}}},
		{name: "duplicate output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "save", OutputIndex: 0}, {NodeID: "save", OutputIndex: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTestOutputs(workflow, tt.outputs)
			if tt.valid && err != nil {
				t.Fatalf("valid outputs rejected: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("invalid outputs accepted")
			}
		})
	}
}

func TestCreateComfyUIWorkflowTestRunBindingPreservesWorkerPromptID(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{
		OwnerUserID:         "user-1",
		APIConversionStatus: iapiserver.ComfyUIAPIConversionReady,
		APIWorkflow: map[string]any{
			"save": map[string]any{"class_type": "SaveImage", "inputs": map[string]any{}},
		},
	}
	workflow.ID = "workflow-1"
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	storage := &workflowTestRunStore{workflow: workflow, engine: engine, catalog: saveImageTestCatalog()}
	tasks := &workflowTestRunTasks{onCreate: func() {
		promptID := "prompt-from-worker"
		storage.run.ExternalJobID = &promptID
	}}
	service := &applicationPlatformService{Dependencies: Dependencies{
		Store:      &executorFactory{applications: storage},
		Tasks:      tasks,
		Principals: staticPrincipal{principal: Principal{UserID: "user-1"}},
	}}

	run, err := service.CreateComfyUIWorkflowTestRun(context.Background(), workflow.ID, &iapiserver.ComfyUIWorkflowTestRunCreateRequest{
		EngineInstanceID: engine.ID,
		Outputs:          []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "save", OutputIndex: 0}},
		IdempotencyKey:   "run-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if storage.boundDAG == "" || run.DAGTaskGroupID == nil {
		t.Fatalf("DAG binding was not persisted: run=%#v", run)
	}
	if run.ExternalJobID == nil || *run.ExternalJobID != "prompt-from-worker" {
		t.Fatalf("worker prompt ID was lost: run=%#v", run)
	}
}

func TestApplyComfyTestRunProjectionExposesFailureAndClearsCurrentStep(t *testing.T) {
	staleCurrentStep := "submit"
	run := &iapiserver.ComfyUIWorkflowTestRun{CurrentStep: &staleCurrentStep}
	dag := &iapiserver.DAGTaskGroup{Status: iapiserver.TaskGroupStatusFailed, Progress: 2.0 / 3.0, Nodes: testRunDAGNodes()}
	tasks := []*iapiserver.AtomicTask{
		{ChildKey: "submit", Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1, TaskNameMeta: iapiserver.TaskNameMeta{}, ObjectMeta: objectMeta("submit-task", "提交")},
		{ChildKey: "poll", Status: iapiserver.AtomicTaskStatusFailed, Progress: 1, Output: map[string]any{"provider_state": "queued", "queue_position": float64(3)}, LastError: iapiserver.TaskError{Message: "prompt id is missing"}, ObjectMeta: objectMeta("poll-task", "轮询")},
		{ChildKey: "collect_preview", Status: iapiserver.AtomicTaskStatusBlocked},
	}

	applyComfyTestRunProjection(run, dag, tasks)

	if run.Status != iapiserver.TaskGroupStatusFailed || run.Progress != 2.0/3.0 {
		t.Fatalf("projected run status=%s progress=%f", run.Status, run.Progress)
	}
	if run.CurrentStep != nil {
		t.Fatalf("terminal run retained current step %q", *run.CurrentStep)
	}
	if run.FailureSummary == nil || *run.FailureSummary != "prompt id is missing" {
		t.Fatalf("failure summary=%v", run.FailureSummary)
	}
	if run.Steps[1].Error == nil || *run.Steps[1].Error != "prompt id is missing" {
		t.Fatalf("failed step error=%v", run.Steps[1].Error)
	}
	if run.Steps[1].ProviderState == nil || *run.Steps[1].ProviderState != "queued" || run.Steps[1].QueuePosition == nil || *run.Steps[1].QueuePosition != 3 {
		t.Fatalf("poll provider projection=%#v", run.Steps[1])
	}
}

func TestProjectComfyTestRunSummariesUsesBatchDAGProjection(t *testing.T) {
	dagA, dagB := "dag-a", "dag-b"
	tasks := &workflowTestRunTasks{}
	runs := []*iapiserver.ComfyUIWorkflowTestRun{{DAGTaskGroupID: &dagA}, {DAGTaskGroupID: &dagB}}

	projectComfyTestRunSummaries(context.Background(), tasks, runs)

	for _, run := range runs {
		if run.Status != iapiserver.TaskGroupStatusRunning || run.Progress != 0.5 {
			t.Fatalf("summary projection=%#v", run)
		}
	}
}

func testRunDAGNodes() []iapiserver.DAGNode {
	return []iapiserver.DAGNode{
		{Key: "submit", Task: iapiserver.AtomicTaskTemplate{Name: "提交"}},
		{Key: "poll", Task: iapiserver.AtomicTaskTemplate{Name: "轮询"}},
		{Key: "collect_preview", Task: iapiserver.AtomicTaskTemplate{Name: "收集预览"}},
	}
}
