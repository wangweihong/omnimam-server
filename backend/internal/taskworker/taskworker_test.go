package apiserver

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
)

type recordingInfrastructureExecutor struct {
	requests []*infrastructure.CommandRequest
	response *infrastructure.CommandResponse
	err      error
}

func (f *recordingInfrastructureExecutor) Execute(_ context.Context, request *infrastructure.CommandRequest) (*infrastructure.CommandResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

func appStudioReadyResponse(runtimeID, endpointID string) *infrastructure.CommandResponse {
	runtime := &iapiserver.InfraRuntime{}
	runtime.ID = runtimeID
	runtime.Status = "RUNNING"
	runtime.EndpointRef = "infra-endpoint://" + endpointID
	endpoint := &iapiserver.InfraRuntimeEndpoint{}
	endpoint.ID = endpointID
	endpoint.RuntimeID = runtimeID
	endpoint.Status = "READY"
	return &infrastructure.CommandResponse{Result: &iapiserver.InfraOperationResult{Runtime: runtime, Endpoint: endpoint}}
}

func appStudioStopResponse(runtimeID, status string) *infrastructure.CommandResponse {
	runtime := &iapiserver.InfraRuntime{}
	runtime.ID = runtimeID
	runtime.Status = status
	return &infrastructure.CommandResponse{Result: &iapiserver.InfraOperationResult{Runtime: runtime}}
}

func appStudioTask(t *testing.T, functionRef string, arguments map[string]any) (*taskfunctionregistry.Registry, workflowruntime.WorkerTask, *iapiserver.AtomicTask) {
	t.Helper()
	registry, err := taskfunctionregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := registry.Active(functionRef, "appstudio", arguments)
	if err != nil {
		t.Fatal(err)
	}
	atomic := &iapiserver.AtomicTask{FunctionRef: functionRef, FunctionContractVersion: contract.ContractVersion, FunctionContractDigest: contract.ContractDigest, Arguments: arguments, CreatedBy: "user-1"}
	atomic.ID = "atomic-1"
	worker := workflowruntime.WorkerTask{AtomicTaskID: atomic.ID, FunctionRef: functionRef, RetryCount: 1, Arguments: arguments}
	return registry, worker, atomic
}

func appStudioPreviewEnsureTestArguments(existing any) map[string]any {
	return map[string]any{
		"studio_application_id": "application-1", "preview_runtime_id": "preview-1", "existing_infra_runtime_id": existing,
		"workspace_id": "workspace-1", "workspace_revision": 7, "workspace_revision_source_ref": "studio-workspace-revision://workspace-1/7",
		"runtime_profile_id": "appstudio.preview.static-web", "runtime_profile_revision": "profile-rev-1", "endpoint_visibility": "USER_ACCESSIBLE",
		"authorization_ref": "appstudio-preview-grant://grant-1", "expected_resource_version": 3,
		"resource_requirement": map[string]any{"cpu_cores": 1.5, "memory_mb": 512, "disk_mb": 1024},
	}
}

func appStudioProductionReconcileTestArguments(existing any) map[string]any {
	return map[string]any{
		"studio_application_id": "application-1", "studio_release_id": "release-1", "studio_runtime_instance_id": "runtime-1", "existing_infra_runtime_id": existing,
		"studio_application_version_id": "version-1", "runtime_config_id": "config-1", "artifact_id": "artifact-1", "artifact_digest": "sha256:artifact",
		"artifact_source_ref": "artifact://artifact-1@sha256:artifact", "environment": "production", "deployment_reason": "DEPLOY",
		"runtime_profile_id": "studioapp.runtime.web-backend", "runtime_profile_revision": "profile-rev-2", "health_check_ref": "appstudio-health-check://check-1",
		"endpoint_visibility": "INTERNAL", "authorization_ref": "appstudio-release-grant://grant-1", "expected_resource_version": 4,
		"resource_requirement": map[string]any{"cpu_cores": 2, "memory_mb": 1024, "disk_mb": 2048, "gpu_count": 0, "gpu_memory_mb": 0},
	}
}

type recordingRepresentationTaskCreator struct {
	request *iapiserver.DAGTaskGroupCreateRequest
	err     error
}

func (f *recordingRepresentationTaskCreator) CreateDAGTaskGroup(_ context.Context, request *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error) {
	f.request = request
	return &iapiserver.DAGTaskGroup{}, f.err
}

type terminalProjectionTaskStore struct {
	store.TaskCenterStore
	task  *iapiserver.AtomicTask
	err   error
	calls int
}

func (s *terminalProjectionTaskStore) GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error) {
	s.calls++
	return s.task, s.err
}

type recordingTerminalProjector struct {
	task  *iapiserver.AtomicTask
	err   error
	calls int
}

type recordingPayloadProjector struct {
	payload []byte
	err     error
	calls   int
}

func (p *recordingPayloadProjector) Project(_ context.Context, payload []byte) error {
	p.calls++
	p.payload = payload
	return p.err
}

type taskworkerApplicationCatalogStore struct {
	store.WorkflowCanvasStore
	calls int
}

func (s *taskworkerApplicationCatalogStore) AddWorkflowNodeDefinitionIdempotent(
	context.Context,
	*iapiserver.WorkflowNodeDefinition,
) (*iapiserver.WorkflowNodeDefinition, bool, error) {
	s.calls++
	return nil, true, nil
}

type publishedCanvasApplicationListerStub struct {
	items []*appsvc.CanvasApplicationVersion
	err   error
}

func (s *publishedCanvasApplicationListerStub) ListPublishedCanvasApplicationVersions(
	context.Context,
) ([]*appsvc.CanvasApplicationVersion, error) {
	return s.items, s.err
}

func (p *recordingTerminalProjector) Completed(_ context.Context, task *iapiserver.AtomicTask) error {
	p.calls++
	p.task = task
	return p.err
}

func TestHealthCron(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     string
	}{
		{name: "seconds", interval: 30 * time.Second, want: "*/30 * * * * *"},
		{name: "minutes", interval: 5 * time.Minute, want: "0 */5 * * * *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthCron(tt.interval); got != tt.want {
				t.Fatalf("healthCron(%s) = %q, want %q", tt.interval, got, tt.want)
			}
		})
	}
}

func TestScheduleTargetOwnership(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{ProjectID: "project", Namespace: "namespace", CreatedBy: "user-1"}
	schedule.ID = "schedule-1"

	atomic := &iapiserver.AtomicTaskCreateRequest{}
	applyAtomicScheduleOwnership(atomic, schedule)
	if atomic.ProjectID != "project" || atomic.Namespace != "namespace" || atomic.CreatedBy != "user-1" || atomic.OwnerType != iapiserver.TaskOwnerTypeSchedule || atomic.OwnerID != "schedule-1" {
		t.Fatalf("atomic ownership = %#v", atomic)
	}
	group := &iapiserver.TaskGroupCreateRequest{}
	applyGroupScheduleOwnership(group, schedule)
	if group.ProjectID != "project" || group.Namespace != "namespace" || group.CreatedBy != "user-1" {
		t.Fatalf("group created_by = %q", group.CreatedBy)
	}
	dag := &iapiserver.DAGTaskGroupCreateRequest{}
	applyDAGScheduleOwnership(dag, schedule)
	if dag.ProjectID != "project" || dag.Namespace != "namespace" || dag.CreatedBy != "user-1" {
		t.Fatalf("dag created_by = %q", dag.CreatedBy)
	}
}

func TestScheduleTimeReadsConductorMilliseconds(t *testing.T) {
	want := time.Date(2026, time.July, 18, 1, 20, 30, 0, time.UTC)
	if got := scheduleTime(float64(want.UnixMilli())); !got.Equal(want) {
		t.Fatalf("scheduleTime = %s, want %s", got, want)
	}
}

func TestRepresentationDAGRequestCreatesImageThumbnailPlan(t *testing.T) {
	payload, err := json.Marshal(representationRequestedEvent{
		AssetID: "asset-1", AssetVersionID: "version-1", OwnerUserID: "user-1",
		ProjectID: "default", Namespace: "default", MediaType: "image", ProfileVersion: "default-v1",
		RequestedRepresentations: []requestedRepresentation{{RepresentationType: "thumbnail", Profile: "list-320"}},
		IdempotencyKey:           "asset-representations:version-1:default-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	creator := &recordingRepresentationTaskCreator{}
	if err := handleRepresentationRequested(context.Background(), creator, payload); err != nil {
		t.Fatalf("handleRepresentationRequested() error = %v", err)
	}
	request := creator.request
	if request == nil || request.ProjectID != "default" || request.Namespace != "default" || request.CreatedBy != "user-1" {
		t.Fatalf("request ownership = %#v", request)
	}
	if request.IdempotencyScope != "asset-representations" || request.IdempotencyKey != "asset-representations:version-1:default-v1" {
		t.Fatalf("request idempotency = %q/%q", request.IdempotencyScope, request.IdempotencyKey)
	}
	wantKeys := []string{"inspect", "thumbnail:list-320", "finalize"}
	if len(request.Nodes) != len(wantKeys) {
		t.Fatalf("node count = %d, want %d", len(request.Nodes), len(wantKeys))
	}
	for index, want := range wantKeys {
		if request.Nodes[index].Key != want {
			t.Fatalf("node[%d].key = %q, want %q", index, request.Nodes[index].Key, want)
		}
	}
	generate := request.Nodes[1].Task
	if generate.Arguments["media_type"] != "image" || generate.Arguments["max_attempts"] != 3 || generate.RetryPolicy.MaxAttempts != 3 || generate.RetryPolicy.BackoffType != "EXPONENTIAL_BACKOFF" {
		t.Fatalf("generate task = %#v", generate)
	}
	if len(request.Edges) != 2 || request.Edges[0].FromNode != "inspect" || request.Edges[0].ToNode != "thumbnail:list-320" || request.Edges[1].FromNode != "thumbnail:list-320" || request.Edges[1].ToNode != "finalize" {
		t.Fatalf("edges = %#v", request.Edges)
	}
}

func TestRepresentationDAGRequestRejectsLegacyEvent(t *testing.T) {
	_, err := representationDAGRequest([]byte(`{"asset_id":"asset-1","asset_version_id":"version-1","owner_user_id":"user-1","profile_version":"default-v1"}`))
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("representationDAGRequest() error = %v", err)
	}
}

func TestRepresentationOrchestratorConsumerGroupIsStable(t *testing.T) {
	if representationOrchestratorConsumerGroup != "task-center-representation-orchestrator" {
		t.Fatalf("consumer group = %q", representationOrchestratorConsumerGroup)
	}
}

func TestApplicationRunTerminalProjectionConsumerGroupIsStable(t *testing.T) {
	if applicationRunTerminalProjectionConsumerGroup != "application-platform-terminal-projection" {
		t.Fatalf("consumer group = %q", applicationRunTerminalProjectionConsumerGroup)
	}
}

func TestCanvasApplicationConsumerGroupsAreStable(t *testing.T) {
	if applicationCatalogConsumerGroup != "workflow-canvas-application-catalog" {
		t.Fatalf("catalog consumer group = %q", applicationCatalogConsumerGroup)
	}
	if applicationArtifactProjectionConsumerGroup != "workflow-canvas-application-artifact-projection" {
		t.Fatalf("artifact consumer group = %q", applicationArtifactProjectionConsumerGroup)
	}
}

func TestHandleApplicationRunTerminalProjection(t *testing.T) {
	terminalTask := &iapiserver.AtomicTask{
		ApplicationRunID: "run-1",
		Status:           iapiserver.AtomicTaskStatusSuccess,
	}
	terminalTask.ID = "task-1"

	tests := []struct {
		name               string
		payload            string
		task               *iapiserver.AtomicTask
		wantStoreCalls     int
		wantProjectorCalls int
		wantError          bool
	}{
		{
			name:               "terminal application run projects current task",
			payload:            `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"SUCCESS"}`,
			task:               terminalTask,
			wantStoreCalls:     1,
			wantProjectorCalls: 1,
		},
		{
			name:    "non-terminal event is acknowledged without lookup",
			payload: `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"RUNNING"}`,
		},
		{
			name:    "unrelated terminal task is acknowledged without lookup",
			payload: `{"atomic_task_id":"task-1","application_run_id":null,"to_status":"SUCCESS"}`,
		},
		{
			name:      "incomplete event is retried",
			payload:   `{"application_run_id":"run-1","to_status":"SUCCESS"}`,
			wantError: true,
		},
		{
			name:           "missing current task is retried",
			payload:        `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"SUCCESS"}`,
			wantStoreCalls: 1,
			wantError:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &terminalProjectionTaskStore{task: tt.task}
			projector := &recordingTerminalProjector{}
			err := handleApplicationRunTerminalProjection(
				context.Background(),
				tasks,
				projector,
				[]byte(tt.payload),
			)
			if (err != nil) != tt.wantError {
				t.Fatalf("handleApplicationRunTerminalProjection() error = %v, wantError %t", err, tt.wantError)
			}
			if tasks.calls != tt.wantStoreCalls {
				t.Fatalf("task store calls = %d, want %d", tasks.calls, tt.wantStoreCalls)
			}
			if projector.calls != tt.wantProjectorCalls {
				t.Fatalf("projector calls = %d, want %d", projector.calls, tt.wantProjectorCalls)
			}
		})
	}
}

func TestConsumeApplicationRunTerminalProjectionAcknowledgement(t *testing.T) {
	task := &iapiserver.AtomicTask{
		ApplicationRunID: "run-1",
		Status:           iapiserver.AtomicTaskStatusSuccess,
	}
	task.ID = "task-1"
	payload := []byte(`{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"SUCCESS"}`)

	t.Run("ack after projection", func(t *testing.T) {
		msg := message.NewMessage("message-1", payload)
		messages := make(chan *message.Message, 1)
		messages <- msg
		close(messages)
		consumeApplicationRunTerminalProjections(
			context.Background(),
			messages,
			&terminalProjectionTaskStore{task: task},
			&recordingTerminalProjector{},
		)
		select {
		case <-msg.Acked():
		default:
			t.Fatal("terminal projection message was not acknowledged")
		}
	})

	t.Run("nack after projection failure", func(t *testing.T) {
		msg := message.NewMessage("message-2", payload)
		messages := make(chan *message.Message, 1)
		messages <- msg
		close(messages)
		consumeApplicationRunTerminalProjections(
			context.Background(),
			messages,
			&terminalProjectionTaskStore{task: task},
			&recordingTerminalProjector{err: stderrors.New("temporary failure")},
		)
		select {
		case <-msg.Nacked():
		default:
			t.Fatal("failed terminal projection message was not negatively acknowledged")
		}
	})
}

func TestConsumeReliablePayloadAcknowledgement(t *testing.T) {
	payload := []byte(`{"artifact_id":"artifact-1"}`)
	tests := []struct {
		name    string
		err     error
		wantAck bool
	}{
		{name: "ack after projection", wantAck: true},
		{name: "nack after projection failure", err: stderrors.New("temporary failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := message.NewMessage(tt.name, payload)
			messages := make(chan *message.Message, 1)
			messages <- msg
			close(messages)
			projector := &recordingPayloadProjector{err: tt.err}
			consumeReliablePayloads(t.Context(), messages, projector, "test-consumer")
			if projector.calls != 1 || string(projector.payload) != string(payload) {
				t.Fatalf("projector = %#v", projector)
			}
			if tt.wantAck {
				select {
				case <-msg.Acked():
				default:
					t.Fatal("message was not acknowledged")
				}
			} else {
				select {
				case <-msg.Nacked():
				default:
					t.Fatal("message was not negatively acknowledged")
				}
			}
		})
	}
}

func TestConsumeApplicationCatalogAcknowledgesLossySchemaDiagnostic(t *testing.T) {
	msg := message.NewMessage("catalog-diagnostic", []byte(`{
		"application_id":"application-1",
		"application_version_id":"version-1",
		"application_template_version_id":"template-version-1",
		"semantic_version":"1.0.0",
		"application_name":"Application",
		"owner_user_id":"user-1",
		"visibility":"private",
		"canvas_enabled":true,
		"run_enabled":true,
		"input_schema":{"type":"object","properties":{"choice":{"oneOf":[{"type":"string"},{"type":"number"}]}}},
		"output_schema":{"type":"object","properties":{}}
	}`))
	messages := make(chan *message.Message, 1)
	messages <- msg
	close(messages)
	target := &taskworkerApplicationCatalogStore{}
	consumeApplicationCatalog(
		t.Context(),
		messages,
		workflowcanvassvc.NewApplicationCatalogProjector(target),
	)
	if target.calls != 0 {
		t.Fatalf("lossy schema reached catalog store: calls=%d", target.calls)
	}
	select {
	case <-msg.Acked():
	default:
		t.Fatal("diagnostic event was not acknowledged")
	}
}

func TestReconcilePublishedApplicationCatalogRepairsConvertibleVersions(t *testing.T) {
	application := &iapiserver.Application{
		OwnerUserID:   "user-1",
		Visibility:    iapiserver.ApplicationVisibilityPrivate,
		CanvasEnabled: true,
		RunEnabled:    true,
	}
	application.ID = "application-1"
	application.Name = "Application"
	valid := &iapiserver.ApplicationVersion{
		ApplicationID:                application.ID,
		SemanticVersion:              "1.0.0",
		ApplicationTemplateVersionID: "template-version-1",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"prompt": map[string]any{"type": "string"}},
		},
		OutputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
	valid.ID = "version-1"
	invalid := *valid
	invalid.ID = "version-2"
	invalid.SemanticVersion = "2.0.0"
	invalid.InputSchema = map[string]any{
		"type":       "object",
		"properties": map[string]any{"choice": map[string]any{"oneOf": []any{map[string]any{"type": "string"}}}},
	}
	target := &taskworkerApplicationCatalogStore{}
	err := reconcilePublishedApplicationCatalog(
		t.Context(),
		&publishedCanvasApplicationListerStub{items: []*appsvc.CanvasApplicationVersion{
			{Application: application, Version: valid},
			{Application: application, Version: &invalid},
		}},
		workflowcanvassvc.NewApplicationCatalogProjector(target),
	)
	if err != nil {
		t.Fatal(err)
	}
	if target.calls != 1 {
		t.Fatalf("catalog repair calls = %d, want 1", target.calls)
	}
}

func TestExecuteAppStudioPreviewEnsureCreatesRuntime(t *testing.T) {
	arguments := appStudioPreviewEnsureTestArguments(nil)
	registry, worker, atomic := appStudioTask(t, "appstudio.preview.ensure", arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("infra-preview-1", "endpoint-1")}
	result, err := executeAppStudioPreviewEnsure(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("infrastructure calls = %d, want 1", len(executor.requests))
	}
	request := executor.requests[0]
	if request.Operation != "create" || request.Create == nil {
		t.Fatalf("request = %#v, want create request", request)
	}
	if request.Create.RequestID != "atomic-1:2" || request.Create.OwnerReference != "preview-1" || request.Create.RuntimeMode != "SERVICE" || request.Create.SourceRef != arguments["workspace_revision_source_ref"] {
		t.Fatalf("create request identity/source = %#v", request.Create)
	}
	if request.Create.RuntimeProfileID != "appstudio.preview.static-web" || request.Create.EndpointVisibility != "USER_ACCESSIBLE" || request.Create.AuthorizationRef != "appstudio-preview-grant://grant-1" {
		t.Fatalf("create request profile/security = %#v", request.Create)
	}
	if result["infra_runtime_id"] != "infra-preview-1" || result["runtime_status"] != "RUNNING" || result["health_status"] != "HEALTHY" || result["endpoint_ref"] != "infra-endpoint://endpoint-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioPreviewEnsureStartsExistingRuntime(t *testing.T) {
	arguments := appStudioPreviewEnsureTestArguments("infra-preview-existing")
	registry, worker, atomic := appStudioTask(t, "appstudio.preview.ensure", arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("infra-preview-existing", "endpoint-2")}
	if _, err := executeAppStudioPreviewEnsure(t.Context(), executor, registry, worker, atomic); err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 || executor.requests[0].Operation != "start" || executor.requests[0].RuntimeID != "infra-preview-existing" || executor.requests[0].Create != nil {
		t.Fatalf("request = %#v, want start existing runtime", executor.requests[0])
	}
}

func TestExecuteAppStudioPreviewStopDelete(t *testing.T) {
	arguments := map[string]any{
		"studio_application_id": "application-1", "preview_runtime_id": "preview-1", "infra_runtime_id": "infra-preview-1",
		"action": "DELETE", "authorization_ref": "appstudio-preview-grant://grant-1", "expected_resource_version": 5,
	}
	registry, worker, atomic := appStudioTask(t, "appstudio.preview.stop", arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioStopResponse("infra-preview-1", "DELETED")}
	result, err := executeAppStudioPreviewStop(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	request := executor.requests[0]
	if request.Operation != "stop" || request.RuntimeID != "infra-preview-1" || !request.Delete {
		t.Fatalf("request = %#v, want delete stop", request)
	}
	if result["runtime_status"] != "DELETED" || result["completed_action"] != "DELETE" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioBuildFailsClosedBeforeInfrastructure(t *testing.T) {
	arguments := map[string]any{
		"studio_application_id": "application-1", "studio_build_id": "build-1", "source_snapshot_id": "snapshot-1", "source_snapshot_digest": "sha256:snapshot",
		"source_snapshot_source_ref": "studio-snapshot://snapshot-1", "runtime_profile_id": "appstudio.build.static-web", "runtime_profile_revision": "profile-rev-1",
		"build_config_ref": "appstudio-build-config://config-1", "dependency_lock_digest": "sha256:lock", "authorization_ref": "appstudio-build-grant://grant-1", "expected_resource_version": 1,
	}
	registry, worker, atomic := appStudioTask(t, "appstudio.build.execute", arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("unused", "unused")}
	if _, err := executeAppStudioBuild(t.Context(), executor, registry, worker, atomic); err == nil || !strings.Contains(err.Error(), "artifact registration is unavailable") {
		t.Fatalf("error = %v, want artifact registration failure", err)
	}
	if len(executor.requests) != 0 {
		t.Fatalf("infrastructure calls = %d, want 0", len(executor.requests))
	}
}

func TestExecuteAppStudioProductionReconcileCreatesArtifactRuntime(t *testing.T) {
	arguments := appStudioProductionReconcileTestArguments(nil)
	registry, worker, atomic := appStudioTask(t, "appstudio.production.reconcile", arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("infra-production-1", "endpoint-production-1")}
	result, err := executeAppStudioProductionReconcile(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	request := executor.requests[0]
	if request.Operation != "create" || request.Create == nil || request.Create.SourceRef != "artifact://artifact-1@sha256:artifact" || request.Create.OwnerReference != "runtime-1" {
		t.Fatalf("request = %#v, want artifact create", request)
	}
	if request.Create.RuntimeProfileID != "studioapp.runtime.web-backend" || request.Create.EndpointVisibility != "INTERNAL" {
		t.Fatalf("request profile/visibility = %#v", request.Create)
	}
	if result["infra_runtime_id"] != "infra-production-1" || result["endpoint_ref"] != "infra-endpoint://endpoint-production-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioProductionStopUsesStopWithoutDelete(t *testing.T) {
	arguments := map[string]any{
		"studio_application_id": "application-1", "studio_release_id": "release-1", "studio_runtime_instance_id": "runtime-1", "infra_runtime_id": "infra-production-1",
		"authorization_ref": "appstudio-release-grant://grant-1", "expected_resource_version": 6,
	}
	registry, worker, atomic := appStudioTask(t, "appstudio.production.stop", arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioStopResponse("infra-production-1", "STOPPED")}
	result, err := executeAppStudioProductionStop(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	request := executor.requests[0]
	if request.Operation != "stop" || request.RuntimeID != "infra-production-1" || request.Delete {
		t.Fatalf("request = %#v, want non-delete stop", request)
	}
	if result["runtime_status"] != "STOPPED" || result["completed_action"] != "STOP" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioRejectsIncorrectContractPinBeforeInfrastructure(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*iapiserver.AtomicTask)
	}{
		{name: "digest", mutate: func(task *iapiserver.AtomicTask) { task.FunctionContractDigest = "sha256:wrong" }},
		{name: "version", mutate: func(task *iapiserver.AtomicTask) { task.FunctionContractVersion = "9.9" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			arguments := appStudioPreviewEnsureTestArguments(nil)
			registry, worker, atomic := appStudioTask(t, "appstudio.preview.ensure", arguments)
			test.mutate(atomic)
			executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("unused", "unused")}
			if _, err := executeAppStudioPreviewEnsure(t.Context(), executor, registry, worker, atomic); err == nil || !strings.Contains(err.Error(), "contract") {
				t.Fatalf("error = %v, want contract pin error", err)
			}
			if len(executor.requests) != 0 {
				t.Fatalf("infrastructure calls = %d, want 0", len(executor.requests))
			}
		})
	}
}
