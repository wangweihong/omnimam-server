package apiserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"strings"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
)

type recordingInfrastructureExecutor struct {
	requests       []*infrastructure.CommandRequest
	response       *infrastructure.CommandResponse
	err            error
	outputContent  *infrastructure.OutputContent
	readCalls      int
	attachRequests []*iapiserver.InfraAttachArtifactRequest
	attachedOutput *iapiserver.InfraRuntimeOutput
}

func (f *recordingInfrastructureExecutor) Execute(_ context.Context, request *infrastructure.CommandRequest) (*infrastructure.CommandResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

func (f *recordingInfrastructureExecutor) ReadOutputContent(_ context.Context, _ string) (*infrastructure.OutputContent, error) {
	f.readCalls++
	return f.outputContent, f.err
}

func (f *recordingInfrastructureExecutor) AttachOutputArtifact(_ context.Context, outputID string, request *iapiserver.InfraAttachArtifactRequest) (*iapiserver.InfraRuntimeOutput, error) {
	copy := *request
	f.attachRequests = append(f.attachRequests, &copy)
	if f.attachedOutput != nil || f.err != nil {
		return f.attachedOutput, f.err
	}
	return &iapiserver.InfraRuntimeOutput{ObjectMeta: imachinery.ObjectMeta{ID: outputID}, ArtifactID: request.ArtifactID}, nil
}

func appStudioReadyResponse(runtimeID, endpointID string) *infrastructure.CommandResponse {
	runtime := &iapiserver.InfraRuntime{}
	runtime.ID = runtimeID
	runtime.Status = iapiserver.TaskWorkerRuntimeStatusRunning
	runtime.EndpointRef = iapiserver.TaskWorkerRefPrefixInfraEndpoint + endpointID
	endpoint := &iapiserver.InfraRuntimeEndpoint{}
	endpoint.ID = endpointID
	endpoint.RuntimeID = runtimeID
	endpoint.Status = iapiserver.TaskWorkerRuntimeEndpointStatusReady
	return &infrastructure.CommandResponse{Result: &iapiserver.InfraOperationResult{Runtime: runtime, Endpoint: endpoint}}
}

func appStudioStopResponse(runtimeID, status string) *infrastructure.CommandResponse {
	runtime := &iapiserver.InfraRuntime{}
	runtime.ID = runtimeID
	runtime.Status = status
	return &infrastructure.CommandResponse{Result: &iapiserver.InfraOperationResult{Runtime: runtime}}
}

func appStudioBuildResponse(content []byte) (*infrastructure.CommandResponse, *iapiserver.InfraRuntimeOutput) {
	sum := sha256.Sum256(content)
	digest := iapiserver.TaskWorkerContentDigestSHA256Prefix + hex.EncodeToString(sum[:])
	collectedAt := imachinery.Now()
	runtime := &iapiserver.InfraRuntime{}
	runtime.ID = "infra-build-1"
	runtime.Status = iapiserver.TaskWorkerRuntimeStatusSucceeded
	output := &iapiserver.InfraRuntimeOutput{
		ObjectMeta: imachinery.ObjectMeta{ID: "output-1"}, RuntimeID: runtime.ID, OutputKey: iapiserver.TaskWorkerArtifactOutputKeyBuildBundle, Status: iapiserver.TaskWorkerRuntimeOutputStatusCollected,
		MediaType: iapiserver.TaskWorkerArtifactMediaTypeBuildBundle, SizeBytes: int64(len(content)), ContentDigest: digest, ContentRef: iapiserver.TaskWorkerRefPrefixInfraOutput + "output-1", CollectedAt: &collectedAt,
	}
	return &infrastructure.CommandResponse{Result: &iapiserver.InfraOperationResult{Runtime: runtime, Outputs: []*iapiserver.InfraRuntimeOutput{output}}}, output
}

type recordingBuildArtifactLifecycle struct {
	existing    *iapiserver.Artifact
	prepared    *iapiserver.Artifact
	storedBytes []byte
	storeCalls  int
}

func (l *recordingBuildArtifactLifecycle) Prepare(_ context.Context, desired *iapiserver.Artifact) (*iapiserver.Artifact, bool, error) {
	l.prepared = desired.DeepCopy()
	if l.existing != nil {
		return l.existing.DeepCopy(), false, nil
	}
	created := desired.DeepCopy()
	created.ID = "artifact-build-1"
	created.ProcessingStatus = iapiserver.ArtifactProcessingCreated
	created.RegistrationStatus = iapiserver.ArtifactRegistrationPending
	return created, true, nil
}

func (l *recordingBuildArtifactLifecycle) StoreContent(_ context.Context, artifact *iapiserver.Artifact, mimeType string, reader io.Reader) (*iapiserver.Artifact, error) {
	l.storeCalls++
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	l.storedBytes = raw
	sum := sha256.Sum256(raw)
	completed := artifact.DeepCopy()
	completed.ProcessingStatus = iapiserver.ArtifactProcessingReady
	completed.Metadata = map[string]any{iapiserver.TaskWorkerKeySHA256: hex.EncodeToString(sum[:]), iapiserver.TaskWorkerKeySizeBytes: int64(len(raw)), iapiserver.TaskWorkerKeyMIMEType: mimeType}
	return completed, nil
}

func appStudioTask(t *testing.T, functionRef string, arguments map[string]any) (*taskfunctionregistry.Registry, workflowruntime.WorkerTask, *iapiserver.AtomicTask) {
	t.Helper()
	registry, err := taskfunctionregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := registry.Active(functionRef, iapiserver.TaskWorkerOwnerDomainAppStudio, arguments)
	if err != nil {
		t.Fatal(err)
	}
	atomic := &iapiserver.AtomicTask{FunctionRef: functionRef, FunctionContractVersion: contract.ContractVersion, FunctionContractDigest: contract.ContractDigest, Arguments: arguments, CreatedBy: "user-1"}
	atomic.ID = "atomic-1"
	worker := workflowruntime.WorkerTask{AtomicTaskID: atomic.ID, RuntimeTaskID: "runtime-task-1", FunctionRef: functionRef, RetryCount: 1, Arguments: arguments}
	return registry, worker, atomic
}

func appStudioPreviewEnsureTestArguments(existing any) map[string]any {
	return map[string]any{
		iapiserver.TaskWorkerKeyStudioApplicationID: "application-1", iapiserver.TaskWorkerKeyPreviewRuntimeID: "preview-1", iapiserver.TaskWorkerKeyExistingInfraRuntimeID: existing,
		iapiserver.TaskWorkerKeyWorkspaceID: "workspace-1", iapiserver.TaskWorkerKeyWorkspaceRevision: 7, iapiserver.TaskWorkerKeyWorkspaceRevisionSourceRef: iapiserver.TaskWorkerRefPrefixStudioWorkspaceRevision + "workspace-1/7",
		iapiserver.TaskWorkerKeyRuntimeProfileID: iapiserver.TaskWorkerAppStudioPreviewProfileStaticWeb, iapiserver.TaskWorkerKeyRuntimeProfileRevision: "profile-rev-1", iapiserver.TaskWorkerKeyEndpointVisibility: iapiserver.TaskWorkerEndpointVisibilityUserAccessible,
		iapiserver.TaskWorkerKeyAuthorizationRef: iapiserver.TaskWorkerRefPrefixAppStudioPreviewGrant + "grant-1", iapiserver.TaskWorkerKeyExpectedResourceVersion: 3,
		iapiserver.TaskWorkerKeyResourceRequirement: map[string]any{"cpu_cores": 1.5, "memory_mb": 512, "disk_mb": 1024},
	}
}

func appStudioProductionReconcileTestArguments(existing any) map[string]any {
	return map[string]any{
		iapiserver.TaskWorkerKeyStudioApplicationID: "application-1", iapiserver.TaskWorkerKeyStudioReleaseID: "release-1", iapiserver.TaskWorkerKeyStudioRuntimeInstanceID: "runtime-1", iapiserver.TaskWorkerKeyExistingInfraRuntimeID: existing,
		iapiserver.TaskWorkerKeyStudioApplicationVersionID: "version-1", iapiserver.TaskWorkerKeyRuntimeConfigID: "config-1", iapiserver.TaskWorkerKeyArtifactID: "artifact-1", iapiserver.TaskWorkerKeyArtifactDigest: iapiserver.TaskWorkerContentDigestSHA256Prefix + "artifact",
		iapiserver.TaskWorkerKeyArtifactSourceRef: iapiserver.TaskWorkerRefPrefixArtifact + "artifact-1@" + iapiserver.TaskWorkerContentDigestSHA256Prefix + "artifact", iapiserver.TaskWorkerKeyEnvironment: iapiserver.TaskWorkerEnvironmentProduction, iapiserver.TaskWorkerKeyDeploymentReason: iapiserver.TaskWorkerDeploymentReasonDeploy,
		iapiserver.TaskWorkerKeyRuntimeProfileID: iapiserver.TaskWorkerAppStudioProductionProfileWebBackend, iapiserver.TaskWorkerKeyRuntimeProfileRevision: "profile-rev-2", iapiserver.TaskWorkerKeyHealthCheckRef: iapiserver.TaskWorkerRefPrefixAppStudioHealthCheck + "check-1",
		iapiserver.TaskWorkerKeyEndpointVisibility: iapiserver.TaskWorkerEndpointVisibilityInternal, iapiserver.TaskWorkerKeyAuthorizationRef: iapiserver.TaskWorkerRefPrefixAppStudioReleaseGrant + "grant-1", iapiserver.TaskWorkerKeyExpectedResourceVersion: 4,
		iapiserver.TaskWorkerKeyResourceRequirement: map[string]any{"cpu_cores": 2, "memory_mb": 1024, "disk_mb": 2048, "gpu_count": 0, "gpu_memory_mb": 0},
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

func TestRepresentationDAGRequestCreatesImageThumbnailPlan(t *testing.T) {
	payload, err := json.Marshal(representationRequestedEvent{
		AssetID: "asset-1", AssetVersionID: "version-1", OwnerUserID: "user-1",
		ProjectID: "default", Namespace: "default", MediaType: iapiserver.AssetMediaTypeImage, ProfileVersion: "default-v1",
		RequestedRepresentations: []requestedRepresentation{{RepresentationType: iapiserver.TaskWorkerTaskKeyThumbnail, Profile: "list-320"}},
		IdempotencyKey:           iapiserver.TaskWorkerIdempotencyPrefixRepresentations + "version-1:default-v1",
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
	if request.IdempotencyScope != iapiserver.TaskWorkerIdempotencyScopeRepresentations || request.IdempotencyKey != iapiserver.TaskWorkerIdempotencyPrefixRepresentations+"version-1:default-v1" {
		t.Fatalf("request idempotency = %q/%q", request.IdempotencyScope, request.IdempotencyKey)
	}
	thumbnailKey := iapiserver.TaskWorkerTaskKeyThumbnail + iapiserver.TaskWorkerCompositeKeySeparator + "list-320"
	wantKeys := []string{iapiserver.TaskWorkerTaskKeyRepresentationInspect, thumbnailKey, iapiserver.TaskWorkerTaskKeyRepresentationFinalize}
	if len(request.Nodes) != len(wantKeys) {
		t.Fatalf("node count = %d, want %d", len(request.Nodes), len(wantKeys))
	}
	for index, want := range wantKeys {
		if request.Nodes[index].Key != want {
			t.Fatalf("node[%d].key = %q, want %q", index, request.Nodes[index].Key, want)
		}
	}
	generate := request.Nodes[1].Task
	if generate.Arguments[iapiserver.TaskWorkerKeyMediaType] != iapiserver.AssetMediaTypeImage || generate.Arguments[iapiserver.TaskWorkerKeyMaxAttempts] != 3 || generate.RetryPolicy.MaxAttempts != 3 || generate.RetryPolicy.BackoffType != iapiserver.TaskWorkerRetryBackoffExponential {
		t.Fatalf("generate task = %#v", generate)
	}
	if len(request.Edges) != 2 || request.Edges[0].FromNode != iapiserver.TaskWorkerTaskKeyRepresentationInspect || request.Edges[0].ToNode != thumbnailKey || request.Edges[1].FromNode != thumbnailKey || request.Edges[1].ToNode != iapiserver.TaskWorkerTaskKeyRepresentationFinalize {
		t.Fatalf("edges = %#v", request.Edges)
	}
}

func TestRepresentationDAGRequestRejectsLegacyEvent(t *testing.T) {
	_, err := representationDAGRequest([]byte(`{"asset_id":"asset-1","asset_version_id":"version-1","owner_user_id":"user-1","profile_version":"default-v1"}`))
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("representationDAGRequest() error = %v", err)
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
			payload:            `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"` + iapiserver.AtomicTaskStatusSuccess + `"}`,
			task:               terminalTask,
			wantStoreCalls:     1,
			wantProjectorCalls: 1,
		},
		{
			name:    "non-terminal event is acknowledged without lookup",
			payload: `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"` + iapiserver.AtomicTaskStatusRunning + `"}`,
		},
		{
			name:    "unrelated terminal task is acknowledged without lookup",
			payload: `{"atomic_task_id":"task-1","application_run_id":null,"to_status":"` + iapiserver.AtomicTaskStatusSuccess + `"}`,
		},
		{
			name:      "incomplete event is retried",
			payload:   `{"application_run_id":"run-1","to_status":"` + iapiserver.AtomicTaskStatusSuccess + `"}`,
			wantError: true,
		},
		{
			name:           "missing current task is retried",
			payload:        `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"` + iapiserver.AtomicTaskStatusSuccess + `"}`,
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
	payload := []byte(`{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"` + iapiserver.AtomicTaskStatusSuccess + `"}`)

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
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioPreviewEnsure, arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("infra-preview-1", "endpoint-1")}
	result, err := executeAppStudioPreviewEnsure(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("infrastructure calls = %d, want 1", len(executor.requests))
	}
	request := executor.requests[0]
	if request.Operation != iapiserver.TaskWorkerInfrastructureOperationCreate || request.Create == nil {
		t.Fatalf("request = %#v, want create request", request)
	}
	if request.Create.RequestID != "atomic-1:2" || request.Create.OwnerReference != "preview-1" || request.Create.RuntimeMode != iapiserver.TaskWorkerRuntimeModeService || request.Create.SourceRef != arguments["workspace_revision_source_ref"] {
		t.Fatalf("create request identity/source = %#v", request.Create)
	}
	if request.Create.RuntimeProfileID != iapiserver.TaskWorkerAppStudioPreviewProfileStaticWeb || request.Create.EndpointVisibility != iapiserver.TaskWorkerEndpointVisibilityUserAccessible || request.Create.AuthorizationRef != iapiserver.TaskWorkerRefPrefixAppStudioPreviewGrant+"grant-1" {
		t.Fatalf("create request profile/security = %#v", request.Create)
	}
	if result[iapiserver.TaskWorkerKeyInfraRuntimeID] != "infra-preview-1" || result[iapiserver.TaskWorkerKeyRuntimeStatus] != iapiserver.TaskWorkerRuntimeStatusRunning || result[iapiserver.TaskWorkerKeyHealthStatus] != iapiserver.TaskWorkerRuntimeHealthStatusHealthy || result[iapiserver.TaskWorkerKeyEndpointRef] != iapiserver.TaskWorkerRefPrefixInfraEndpoint+"endpoint-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioPreviewEnsureStartsExistingRuntime(t *testing.T) {
	arguments := appStudioPreviewEnsureTestArguments("infra-preview-existing")
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioPreviewEnsure, arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("infra-preview-existing", "endpoint-2")}
	if _, err := executeAppStudioPreviewEnsure(t.Context(), executor, registry, worker, atomic); err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 || executor.requests[0].Operation != iapiserver.TaskWorkerInfrastructureOperationStart || executor.requests[0].RuntimeID != "infra-preview-existing" || executor.requests[0].Create != nil {
		t.Fatalf("request = %#v, want start existing runtime", executor.requests[0])
	}
}

func TestExecuteAppStudioPreviewStopDelete(t *testing.T) {
	arguments := map[string]any{
		iapiserver.TaskWorkerKeyStudioApplicationID: "application-1", iapiserver.TaskWorkerKeyPreviewRuntimeID: "preview-1", iapiserver.TaskWorkerKeyInfraRuntimeID: "infra-preview-1",
		iapiserver.TaskWorkerKeyAction: iapiserver.TaskWorkerActionDelete, iapiserver.TaskWorkerKeyAuthorizationRef: iapiserver.TaskWorkerRefPrefixAppStudioPreviewGrant + "grant-1", iapiserver.TaskWorkerKeyExpectedResourceVersion: 5,
	}
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioPreviewStop, arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioStopResponse("infra-preview-1", iapiserver.TaskWorkerRuntimeStatusDeleted)}
	result, err := executeAppStudioPreviewStop(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	request := executor.requests[0]
	if request.Operation != iapiserver.TaskWorkerInfrastructureOperationStop || request.RuntimeID != "infra-preview-1" || !request.Delete {
		t.Fatalf("request = %#v, want delete stop", request)
	}
	if result[iapiserver.TaskWorkerKeyRuntimeStatus] != iapiserver.TaskWorkerRuntimeStatusDeleted || result[iapiserver.TaskWorkerKeyCompletedAction] != iapiserver.TaskWorkerActionDelete {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioBuildDeliversArtifact(t *testing.T) {
	arguments := appStudioBuildTestArguments()
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioBuildExecute, arguments)
	content := []byte("build bundle bytes")
	response, output := appStudioBuildResponse(content)
	executor := &recordingInfrastructureExecutor{response: response, outputContent: &infrastructure.OutputContent{
		Body: io.NopCloser(bytes.NewReader(content)), MediaType: output.MediaType, SizeBytes: output.SizeBytes, ContentDigest: output.ContentDigest,
	}}
	lifecycle := &recordingBuildArtifactLifecycle{}
	result, err := executeAppStudioBuild(t.Context(), executor, lifecycle, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 || executor.requests[0].Create == nil {
		t.Fatalf("infrastructure requests = %#v, want one create", executor.requests)
	}
	create := executor.requests[0].Create
	if create.RuntimeMode != iapiserver.TaskWorkerRuntimeModeJob || create.OwnerReference != "build-1" || len(create.OutputDeclarations) != 1 || create.OutputDeclarations[0].OutputKey != iapiserver.TaskWorkerArtifactOutputKeyBuildBundle || create.OutputDeclarations[0].RelativePath != iapiserver.TaskWorkerArtifactRelativePathBuildBundle {
		t.Fatalf("build create request = %#v", create)
	}
	if lifecycle.prepared == nil || lifecycle.prepared.ProducerType != iapiserver.TaskWorkerArtifactProducerTypeStudioBuild || lifecycle.prepared.ProducerIdempotencyKey != iapiserver.TaskWorkerBuildIdempotencyPrefix+"build-1"+iapiserver.TaskWorkerCompositeKeySeparator+iapiserver.TaskWorkerArtifactOutputKeyBuildBundle || lifecycle.prepared.OutputKey != iapiserver.TaskWorkerArtifactOutputKeyBuildBundle {
		t.Fatalf("prepared artifact = %#v", lifecycle.prepared)
	}
	if !bytes.Equal(lifecycle.storedBytes, content) || len(executor.attachRequests) != 1 || executor.attachRequests[0].ArtifactID != "artifact-build-1" || executor.attachRequests[0].ContentDigest != output.ContentDigest {
		t.Fatalf("delivery state bytes=%q attach=%#v", lifecycle.storedBytes, executor.attachRequests)
	}
	if result[iapiserver.TaskWorkerKeyArtifactID] != "artifact-build-1" || result[iapiserver.TaskWorkerKeyArtifactDigest] != output.ContentDigest || result[iapiserver.TaskWorkerKeyProcessingStatus] != iapiserver.ArtifactProcessingReady || result[iapiserver.TaskWorkerKeyValidationStatus] != iapiserver.TaskWorkerBuildValidationStatusPassed || result[iapiserver.TaskWorkerKeyLogsRef] != iapiserver.TaskWorkerTaskAttemptLogRefPrefix+"runtime-task-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioBuildRejectsContentMismatch(t *testing.T) {
	for _, test := range []struct {
		name        string
		mutate      func(*iapiserver.InfraRuntimeOutput)
		contentBody []byte
	}{
		{name: "size", mutate: func(output *iapiserver.InfraRuntimeOutput) { output.SizeBytes++ }, contentBody: []byte("build bundle bytes")},
		{name: "digest", mutate: func(output *iapiserver.InfraRuntimeOutput) {
			output.ContentDigest = iapiserver.TaskWorkerContentDigestSHA256Prefix + strings.Repeat("0", 64)
		}, contentBody: []byte("build bundle bytes")},
	} {
		t.Run(test.name, func(t *testing.T) {
			arguments := appStudioBuildTestArguments()
			registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioBuildExecute, arguments)
			response, output := appStudioBuildResponse(test.contentBody)
			test.mutate(output)
			executor := &recordingInfrastructureExecutor{response: response, outputContent: &infrastructure.OutputContent{
				Body: io.NopCloser(bytes.NewReader(test.contentBody)), MediaType: output.MediaType, SizeBytes: output.SizeBytes, ContentDigest: output.ContentDigest,
			}}
			lifecycle := &recordingBuildArtifactLifecycle{}
			if _, err := executeAppStudioBuild(t.Context(), executor, lifecycle, registry, worker, atomic); err == nil {
				t.Fatal("expected content integrity failure")
			}
			if lifecycle.storeCalls != 0 || len(executor.attachRequests) != 0 {
				t.Fatalf("store calls = %d attach calls = %d, want zero", lifecycle.storeCalls, len(executor.attachRequests))
			}
		})
	}
}

func TestExecuteAppStudioBuildReusesReadyArtifact(t *testing.T) {
	arguments := appStudioBuildTestArguments()
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioBuildExecute, arguments)
	content := []byte("build bundle bytes")
	response, output := appStudioBuildResponse(content)
	existing := &iapiserver.Artifact{ProcessingStatus: iapiserver.ArtifactProcessingReady, Metadata: map[string]any{
		iapiserver.TaskWorkerKeySHA256: strings.TrimPrefix(output.ContentDigest, iapiserver.TaskWorkerContentDigestSHA256Prefix), iapiserver.TaskWorkerKeySizeBytes: output.SizeBytes, iapiserver.TaskWorkerKeyMIMEType: output.MediaType,
	}}
	existing.ID = "artifact-existing"
	executor := &recordingInfrastructureExecutor{response: response}
	lifecycle := &recordingBuildArtifactLifecycle{existing: existing}
	result, err := executeAppStudioBuild(t.Context(), executor, lifecycle, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	if executor.readCalls != 0 || lifecycle.storeCalls != 0 || len(executor.attachRequests) != 1 {
		t.Fatalf("read=%d store=%d attach=%d", executor.readCalls, lifecycle.storeCalls, len(executor.attachRequests))
	}
	if result[iapiserver.TaskWorkerKeyArtifactID] != "artifact-existing" {
		t.Fatalf("result = %#v", result)
	}
}

func appStudioBuildTestArguments() map[string]any {
	return map[string]any{
		iapiserver.TaskWorkerKeyStudioApplicationID: "application-1", iapiserver.TaskWorkerKeyStudioBuildID: "build-1", iapiserver.TaskWorkerKeySourceSnapshotID: "snapshot-1", iapiserver.TaskWorkerKeySourceSnapshotDigest: iapiserver.TaskWorkerContentDigestSHA256Prefix + "snapshot",
		iapiserver.TaskWorkerKeySourceSnapshotSourceRef: iapiserver.TaskWorkerRefPrefixStudioSnapshot + "snapshot-1", iapiserver.TaskWorkerKeyRuntimeProfileID: iapiserver.TaskWorkerAppStudioBuildProfileStaticWeb, iapiserver.TaskWorkerKeyRuntimeProfileRevision: "profile-rev-1",
		iapiserver.TaskWorkerKeyBuildConfigRef: iapiserver.TaskWorkerRefPrefixAppStudioBuildConfig + "config-1", iapiserver.TaskWorkerKeyDependencyLockDigest: iapiserver.TaskWorkerContentDigestSHA256Prefix + "lock", iapiserver.TaskWorkerKeyAuthorizationRef: iapiserver.TaskWorkerRefPrefixAppStudioBuildGrant + "grant-1", iapiserver.TaskWorkerKeyExpectedResourceVersion: 1,
	}
}

func TestExecuteAppStudioProductionReconcileCreatesArtifactRuntime(t *testing.T) {
	arguments := appStudioProductionReconcileTestArguments(nil)
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioProductionReconcile, arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioReadyResponse("infra-production-1", "endpoint-production-1")}
	result, err := executeAppStudioProductionReconcile(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	request := executor.requests[0]
	if request.Operation != iapiserver.TaskWorkerInfrastructureOperationCreate || request.Create == nil || request.Create.SourceRef != iapiserver.TaskWorkerRefPrefixArtifact+"artifact-1@"+iapiserver.TaskWorkerContentDigestSHA256Prefix+"artifact" || request.Create.OwnerReference != "runtime-1" {
		t.Fatalf("request = %#v, want artifact create", request)
	}
	if request.Create.RuntimeProfileID != iapiserver.TaskWorkerAppStudioProductionProfileWebBackend || request.Create.EndpointVisibility != iapiserver.TaskWorkerEndpointVisibilityInternal {
		t.Fatalf("request profile/visibility = %#v", request.Create)
	}
	if result[iapiserver.TaskWorkerKeyInfraRuntimeID] != "infra-production-1" || result[iapiserver.TaskWorkerKeyEndpointRef] != iapiserver.TaskWorkerRefPrefixInfraEndpoint+"endpoint-production-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioProductionStopUsesStopWithoutDelete(t *testing.T) {
	arguments := map[string]any{
		"studio_application_id": "application-1", "studio_release_id": "release-1", "studio_runtime_instance_id": "runtime-1", "infra_runtime_id": "infra-production-1",
		iapiserver.TaskWorkerKeyAuthorizationRef: iapiserver.TaskWorkerRefPrefixAppStudioReleaseGrant + "grant-1", iapiserver.TaskWorkerKeyExpectedResourceVersion: 6,
	}
	registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioProductionStop, arguments)
	executor := &recordingInfrastructureExecutor{response: appStudioStopResponse("infra-production-1", iapiserver.TaskWorkerRuntimeStatusStopped)}
	result, err := executeAppStudioProductionStop(t.Context(), executor, registry, worker, atomic)
	if err != nil {
		t.Fatal(err)
	}
	request := executor.requests[0]
	if request.Operation != iapiserver.TaskWorkerInfrastructureOperationStop || request.RuntimeID != "infra-production-1" || request.Delete {
		t.Fatalf("request = %#v, want non-delete stop", request)
	}
	if result[iapiserver.TaskWorkerKeyRuntimeStatus] != iapiserver.TaskWorkerRuntimeStatusStopped || result[iapiserver.TaskWorkerKeyCompletedAction] != iapiserver.TaskWorkerActionStop {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteAppStudioRejectsIncorrectContractPinBeforeInfrastructure(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*iapiserver.AtomicTask)
	}{
		{name: "digest", mutate: func(task *iapiserver.AtomicTask) {
			task.FunctionContractDigest = iapiserver.TaskWorkerContentDigestSHA256Prefix + "wrong"
		}},
		{name: "version", mutate: func(task *iapiserver.AtomicTask) { task.FunctionContractVersion = "9.9" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			arguments := appStudioPreviewEnsureTestArguments(nil)
			registry, worker, atomic := appStudioTask(t, iapiserver.TaskWorkerFunctionAppStudioPreviewEnsure, arguments)
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
