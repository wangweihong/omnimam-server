package applicationplatform

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
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
	artifact  *iapiserver.ApplicationArtifact
}

func (s *executorApplicationStore) ListApplicationRunProjectionCandidates(context.Context, int) ([]*iapiserver.ApplicationRun, error) {
	return []*iapiserver.ApplicationRun{s.run}, nil
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
func (s *executorApplicationStore) UpsertArtifact(_ context.Context, artifact *iapiserver.ApplicationArtifact) (*iapiserver.ApplicationArtifact, error) {
	copyArtifact := *artifact
	copyArtifact.ID = "artifact-1"
	copyArtifact.ResourceVersion = 1
	s.artifact = &copyArtifact
	return &copyArtifact, nil
}
func (s *executorApplicationStore) UpdateArtifactRegistration(_ context.Context, id, status, assetID, errorCode, detail string, version int64) (*iapiserver.ApplicationArtifact, error) {
	updated := *s.artifact
	updated.ID, updated.RegistrationStatus, updated.RegistrationErrorCode, updated.RegistrationFailureDetail = id, status, errorCode, detail
	updated.ResourceVersion = version + 1
	if assetID != "" {
		updated.AssetID = &assetID
	}
	s.artifact = &updated
	return &updated, nil
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

type fakeAssetRegistrar struct{ calls atomic.Int32 }

func (r *fakeAssetRegistrar) Register(context.Context, *iapiserver.ApplicationArtifact) (string, error) {
	r.calls.Add(1)
	return "asset-1", nil
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
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	applicationStore := &executorApplicationStore{
		run:    &iapiserver.ApplicationRun{EngineInstanceID: "engine-1", ExecutionSnapshot: map[string]any{"capability_definition_id": "text.chat_completion"}},
		engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline, MaxConcurrency: 1, TaskTimeoutSeconds: 2},
	}
	operation := &fakeOperationExecutor{id: "deepseek_chat_completions"}
	executor, err := NewApplicationRunExecutor(
		&executorFactory{applications: applicationStore}, runtimeRegistry, nil,
		map[string]EngineAdapter{"deepseek_official": fakeEngineAdapter{id: "deepseek_official"}},
		map[string]OperationExecutor{"deepseek_chat_completions": operation}, nil, nil,
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

func TestApplicationRunExecutorProjectsAndRegistersArtifact(t *testing.T) {
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	applicationStore := &executorApplicationStore{run: &iapiserver.ApplicationRun{OwnerUserID: "user-1"}}
	assets := &fakeAssetRegistrar{}
	events := &recordingEventPublisher{}
	executor, err := NewApplicationRunExecutor(&executorFactory{applications: applicationStore}, runtimeRegistry, nil, nil, nil, assets, events)
	if err != nil {
		t.Fatal(err)
	}
	task := &iapiserver.AtomicTask{
		ApplicationRunID: "run-1", Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1,
		Output: map[string]any{"values": map[string]any{"video_url": "https://example.com/video.mp4"}, "artifacts": []any{map[string]any{"output_key": "video", "media_type": "video", "content_ref": "https://example.com/video.mp4"}}},
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
	if applicationStore.artifact == nil || applicationStore.artifact.RegistrationStatus != iapiserver.ArtifactRegistrationRegistered || assets.calls.Load() != 1 {
		t.Fatalf("artifact was not registered: %#v calls=%d", applicationStore.artifact, assets.calls.Load())
	}
	if len(events.events) != 2 || events.events[0].Type != "application_run_projection_changed" || events.events[1].Type != "application_artifact_registration_changed" {
		t.Fatalf("unexpected events: %#v", events.events)
	}
}

func TestApplicationRunExecutorRepairsExistingTerminalProjection(t *testing.T) {
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	taskID := "task-1"
	applicationStore := &executorApplicationStore{run: &iapiserver.ApplicationRun{OwnerUserID: "user-1", AtomicTaskID: &taskID}}
	task := &iapiserver.AtomicTask{ApplicationRunID: "run-1", Status: iapiserver.AtomicTaskStatusSuccess, Output: map[string]any{"values": map[string]any{"text": "done"}}}
	task.ID, task.ResourceVersion = taskID, 4
	executor, err := NewApplicationRunExecutor(&executorFactory{applications: applicationStore, tasks: &executorTaskStore{task: task}}, runtimeRegistry, nil, nil, nil, &fakeAssetRegistrar{}, NoopEventPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.ReconcileTerminalProjections(context.Background(), 200); err != nil {
		t.Fatal(err)
	}
	if applicationStore.projected == nil || applicationStore.projected.TaskResourceVersion != 4 {
		t.Fatalf("existing terminal run was not repaired: %#v", applicationStore.projected)
	}
}
