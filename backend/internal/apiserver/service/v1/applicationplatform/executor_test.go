package applicationplatform

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	enginegateway "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type executorFactory struct {
	store.Factory
	applications store.ApplicationPlatformStore
	tasks        store.TaskCenterStore
}

func (f *executorFactory) ApplicationPlatforms() store.ApplicationPlatformStore {
	return f.applications
}
func (f *executorFactory) TaskCenters() store.TaskCenterStore { return f.tasks }

type executorApplicationStore struct {
	store.ApplicationPlatformStore
	run       *iapiserver.ApplicationRun
	engine    *iapiserver.EngineInstance
	projected *iapiserver.ApplicationRun
	artifact  *iapiserver.Artifact
	ref       *iapiserver.ApplicationArtifactRef
}

type executorTaskStore struct {
	store.TaskCenterStore
	task *iapiserver.AtomicTask
}

func (s *executorTaskStore) GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error) {
	return s.task, nil
}

func (s *executorApplicationStore) GetApplicationRun(context.Context, string) (*iapiserver.ApplicationRun, error) {
	return s.run, nil
}
func (s *executorApplicationStore) GetEngineInstance(context.Context, string) (*iapiserver.EngineInstance, error) {
	return s.engine, nil
}
func (s *executorApplicationStore) ProjectApplicationRun(_ context.Context, _ string, version int64, status, _ string, values []map[string]any) (*iapiserver.ApplicationRun, error) {
	projected := *s.run
	projected.TaskResourceVersion = version
	projected.TaskStatusProjection = &status
	projected.OutputValues = values
	s.projected = &projected
	return &projected, nil
}
func (s *executorApplicationStore) ListArtifactsByRun(context.Context, string) ([]*iapiserver.ApplicationArtifact, error) {
	return []*iapiserver.ApplicationArtifact{}, nil
}
func (s *executorApplicationStore) ProjectApplicationArtifactRef(_ context.Context, ref *iapiserver.ApplicationArtifactRef) (*iapiserver.ApplicationArtifactRef, bool, error) {
	if s.ref != nil && s.ref.ArtifactResourceVersion >= ref.ArtifactResourceVersion {
		return s.ref, false, nil
	}
	copyRef := *ref
	copyRef.ID = "ref-1"
	copyRef.ResourceVersion = ref.ArtifactResourceVersion
	s.ref = &copyRef
	return &copyRef, true, nil
}

type fakeEngineAdapter struct{ id string }

func (a fakeEngineAdapter) ID() string { return a.id }
func (fakeEngineAdapter) Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	return &iapiserver.EngineHealthCheckResult{HealthStatus: iapiserver.EngineHealthOnline}, nil
}

type fakeOperationExecutor struct {
	id      string
	active  atomic.Int32
	maximum atomic.Int32
}

type fakeCheckpointOperationExecutor struct {
	fakeOperationExecutor
	checkpoint       map[string]any
	cancelCheckpoint map[string]any
}

func (e *fakeCheckpointOperationExecutor) ExecuteCheckpoint(_ context.Context, _ *iapiserver.EngineInstance, _ *iapiserver.ApplicationRun, checkpoint map[string]any) (map[string]any, error) {
	e.checkpoint = checkpoint
	return map[string]any{"external_job_id": checkpoint["external_job_id"], "in_progress": true}, nil
}

func (e *fakeCheckpointOperationExecutor) CancelExternalJob(_ context.Context, _ *iapiserver.EngineInstance, _ *iapiserver.ApplicationRun, checkpoint map[string]any) error {
	e.cancelCheckpoint = checkpoint
	return nil
}

func (e *fakeOperationExecutor) ID() string { return e.id }
func (e *fakeOperationExecutor) Execute(context.Context, *iapiserver.EngineInstance, *iapiserver.ApplicationRun) (map[string]any, error) {
	active := e.active.Add(1)
	defer e.active.Add(-1)
	for {
		maximum := e.maximum.Load()
		if active <= maximum || e.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond)
	return map[string]any{"values": map[string]any{"text": "ok"}}, nil
}

type fakeArtifactLifecycle struct{ calls atomic.Int32 }

func (r *fakeArtifactLifecycle) Prepare(_ context.Context, artifact *iapiserver.Artifact) (*iapiserver.Artifact, bool, error) {
	copyArtifact := *artifact
	copyArtifact.ID, copyArtifact.ProcessingStatus, copyArtifact.RegistrationStatus, copyArtifact.ResourceVersion =
		"artifact-1", iapiserver.ArtifactProcessingCreated, iapiserver.ArtifactRegistrationPending, 1
	return &copyArtifact, true, nil
}
func (r *fakeArtifactLifecycle) StoreContent(_ context.Context, artifact *iapiserver.Artifact, _ string, _ io.Reader) (*iapiserver.Artifact, error) {
	copyArtifact := *artifact
	copyArtifact.ProcessingStatus, copyArtifact.ResourceVersion = iapiserver.ArtifactProcessingReady, artifact.ResourceVersion+1
	return &copyArtifact, nil
}
func (r *fakeArtifactLifecycle) Register(_ context.Context, artifact *iapiserver.Artifact, _ string) (*iapiserver.Artifact, error) {
	r.calls.Add(1)
	copyArtifact := *artifact
	copyArtifact.RegistrationStatus, copyArtifact.AssetID, copyArtifact.AssetVersionID, copyArtifact.ResourceVersion =
		iapiserver.ArtifactRegistrationRegistered, "asset-1", "version-1", artifact.ResourceVersion+1
	return &copyArtifact, nil
}
func (r *fakeArtifactLifecycle) FailProcessing(_ context.Context, artifact *iapiserver.Artifact, code, detail string) (*iapiserver.Artifact, error) {
	copyArtifact := *artifact
	copyArtifact.ProcessingStatus, copyArtifact.ProcessingErrorCode, copyArtifact.ProcessingErrorDetail =
		iapiserver.ArtifactProcessingFailed, code, detail
	copyArtifact.ResourceVersion++
	return &copyArtifact, nil
}
func (r *fakeArtifactLifecycle) FailRegistration(_ context.Context, artifact *iapiserver.Artifact, code, detail string) (*iapiserver.Artifact, error) {
	copyArtifact := *artifact
	copyArtifact.RegistrationStatus, copyArtifact.RegistrationErrorCode, copyArtifact.RegistrationErrorDetail =
		iapiserver.ArtifactRegistrationFailed, code, detail
	copyArtifact.ResourceVersion++
	return &copyArtifact, nil
}

type recordingEventPublisher struct {
	mu     sync.Mutex
	events []*iapiserver.ApplicationPlatformEvent
}

func (p *recordingEventPublisher) Publish(_ context.Context, event *iapiserver.ApplicationPlatformEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

func TestApplicationRunExecutorUsesRegistriesAndEngineConcurrency(t *testing.T) {
	runtimeRegistry, _ := testStaticRegistries(t)
	applicationStore := &executorApplicationStore{
		run:    &iapiserver.ApplicationRun{EngineInstanceID: "engine-1", ExecutionSnapshot: map[string]any{"capability_definition_id": "text.chat_completion"}},
		engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline, MaxConcurrency: 1, TaskTimeoutSeconds: 2},
	}
	operation := &fakeOperationExecutor{id: "deepseek_chat_completions"}
	executor, err := NewApplicationRunExecutor(
		&executorFactory{applications: applicationStore}, runtimeRegistry, nil,
		map[string]enginegateway.Adapter{"deepseek_official": fakeEngineAdapter{id: "deepseek_official"}},
		map[string]enginegateway.OperationExecutor{"deepseek_chat_completions": operation}, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, executeErr := executor.Execute(context.Background(), &iapiserver.AtomicTask{ApplicationRunID: "run-1"}); executeErr != nil {
				t.Errorf("execute failed: %v", executeErr)
			}
		}()
	}
	wait.Wait()
	if operation.maximum.Load() != 1 {
		t.Fatalf("engine concurrency was not enforced: maximum=%d", operation.maximum.Load())
	}
}

func TestApplicationRunExecutorPassesRuntimeCheckpoint(t *testing.T) {
	runtimeRegistry, _ := testStaticRegistries(t)
	applicationStore := &executorApplicationStore{
		run:    &iapiserver.ApplicationRun{EngineInstanceID: "engine-1", ExecutionSnapshot: map[string]any{"capability_definition_id": "text.chat_completion"}},
		engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline, MaxConcurrency: 1, TaskTimeoutSeconds: 2},
	}
	operation := &fakeCheckpointOperationExecutor{fakeOperationExecutor: fakeOperationExecutor{id: "deepseek_chat_completions"}}
	executor, err := NewApplicationRunExecutor(
		&executorFactory{applications: applicationStore}, runtimeRegistry, nil,
		map[string]enginegateway.Adapter{"deepseek_official": fakeEngineAdapter{id: "deepseek_official"}},
		map[string]enginegateway.OperationExecutor{"deepseek_chat_completions": operation}, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := map[string]any{"external_job_id": "job-1"}
	output, err := executor.ExecuteWithCheckpoint(context.Background(), &iapiserver.AtomicTask{ApplicationRunID: "run-1"}, checkpoint)
	if err != nil || operation.checkpoint["external_job_id"] != "job-1" || output["in_progress"] != true {
		t.Fatalf("checkpoint execution = output %#v, checkpoint %#v, err %v", output, operation.checkpoint, err)
	}
}

func TestApplicationRunExecutorCancelsCheckpointedExternalJob(t *testing.T) {
	runtimeRegistry, _ := testStaticRegistries(t)
	applicationStore := &executorApplicationStore{
		run:    &iapiserver.ApplicationRun{EngineInstanceID: "engine-1", ExecutionSnapshot: map[string]any{"capability_definition_id": "text.chat_completion"}},
		engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline},
	}
	operation := &fakeCheckpointOperationExecutor{fakeOperationExecutor: fakeOperationExecutor{id: "deepseek_chat_completions"}}
	executor, err := NewApplicationRunExecutor(
		&executorFactory{applications: applicationStore}, runtimeRegistry, nil,
		map[string]enginegateway.Adapter{"deepseek_official": fakeEngineAdapter{id: "deepseek_official"}},
		map[string]enginegateway.OperationExecutor{"deepseek_chat_completions": operation}, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	task := &iapiserver.AtomicTask{ApplicationRunID: "run-1", Status: iapiserver.AtomicTaskStatusCanceled, Output: map[string]any{"external_job_id": "job-1"}}
	task.ID, task.ResourceVersion = "task-1", 2
	if err := executor.Completed(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if operation.cancelCheckpoint["external_job_id"] != "job-1" {
		t.Fatalf("cancel checkpoint = %#v", operation.cancelCheckpoint)
	}
}

func TestApplicationRunExecutorProjectsAndRegistersArtifact(t *testing.T) {
	runtimeRegistry, _ := testStaticRegistries(t)
	contentServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "video/mp4")
		_, _ = response.Write([]byte("video"))
	}))
	defer contentServer.Close()
	applicationStore := &executorApplicationStore{
		run:    &iapiserver.ApplicationRun{OwnerUserID: "user-1", EngineInstanceID: "engine-1"},
		engine: &iapiserver.EngineInstance{BaseURL: contentServer.URL, ApplicationEngineTypeID: "comfyui"},
	}
	applicationStore.run.ID = "run-1"
	assets := &fakeArtifactLifecycle{}
	events := &recordingEventPublisher{}
	executor, err := NewApplicationRunExecutor(&executorFactory{applications: applicationStore}, runtimeRegistry, nil, nil, nil, assets, events)
	if err != nil {
		t.Fatal(err)
	}
	task := &iapiserver.AtomicTask{
		ApplicationRunID: "run-1", Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1,
		Output: map[string]any{"values": map[string]any{"video_url": contentServer.URL + "/video.mp4"}, "artifacts": []any{map[string]any{"output_key": "video", "media_type": "video", "content_ref": contentServer.URL + "/video.mp4"}}},
	}
	task.ID = "task-1"
	task.ResourceVersion = 3
	task.CompletedAt = imachinery.Now()
	if err := executor.Completed(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if applicationStore.projected == nil || applicationStore.projected.TaskResourceVersion != 3 {
		t.Fatalf("task projection was not applied: %#v", applicationStore.projected)
	}
	if applicationStore.ref == nil || applicationStore.ref.ArtifactRegistrationStatus != iapiserver.ArtifactRegistrationRegistered || assets.calls.Load() != 1 {
		t.Fatalf("artifact was not registered: %#v calls=%d", applicationStore.ref, assets.calls.Load())
	}
	if len(events.events) != 1 || events.events[0].Type != "application_run_projection_changed" {
		t.Fatalf("unexpected events: %#v", events.events)
	}
}
