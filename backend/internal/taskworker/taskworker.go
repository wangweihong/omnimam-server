package apiserver

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	agentsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/agent"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	engine "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	modeladapters "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters"
	comfyuiadapter "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/comfyui"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
)

const (
	representationOrchestratorConsumerGroup       = "task-center-representation-orchestrator"
	applicationRunTerminalProjectionConsumerGroup = "application-platform-terminal-projection"
	applicationCatalogConsumerGroup               = "workflow-canvas-application-catalog"
	applicationArtifactProjectionConsumerGroup    = "workflow-canvas-application-artifact-projection"
)

type representationTaskCreator interface {
	CreateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error)
}

type applicationRunTerminalProjector interface {
	Completed(context.Context, *iapiserver.AtomicTask) error
}

type reliablePayloadProjector interface {
	Project(context.Context, []byte) error
}

type publishedCanvasApplicationLister interface {
	ListPublishedCanvasApplicationVersions(context.Context) ([]*appsvc.CanvasApplicationVersion, error)
}

type representationRequestedEvent struct {
	AssetID                  string                    `json:"asset_id"`
	AssetVersionID           string                    `json:"asset_version_id"`
	OwnerUserID              string                    `json:"owner_user_id"`
	ProjectID                string                    `json:"project_id"`
	Namespace                string                    `json:"namespace"`
	MediaType                string                    `json:"media_type"`
	ProfileVersion           string                    `json:"profile_version"`
	RequestedRepresentations []requestedRepresentation `json:"requested_representations"`
	IdempotencyKey           string                    `json:"idempotency_key"`
}

type requestedRepresentation struct {
	RepresentationType string `json:"representation_type"`
	Profile            string `json:"profile"`
	Required           bool   `json:"required"`
}

// RunTaskWorker starts only Conductor AtomicTask handlers and runtime projection reconciliation.
func RunTaskWorker(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("taskworker config is required")
	}
	if cfg.WorkflowRuntimeOptions == nil || !cfg.WorkflowRuntimeOptions.Enabled {
		return fmt.Errorf("workflow runtime must be enabled for taskworker")
	}
	if cfg.InfrastructureClientOptions == nil {
		return fmt.Errorf("infrastructure client options are required for taskworker")
	}
	infrastructureClient, err := infrastructure.NewClient(cfg.InfrastructureClientOptions.BaseURL, cfg.InfrastructureClientOptions.Token)
	if err != nil {
		return errors.Wrap(err, "construct infrastructure client")
	}
	if err := apiserver.InitializeStore(cfg); err != nil {
		return err
	}
	runtime, err := workflowruntime.NewConductor(workflowruntime.ConductorConfig{
		BaseURL: cfg.WorkflowRuntimeOptions.BaseURL, AuthKey: cfg.WorkflowRuntimeOptions.AuthKey,
		AuthSecret: cfg.WorkflowRuntimeOptions.AuthSecret, HTTPTimeout: cfg.WorkflowRuntimeOptions.HTTPTimeout,
		PollInterval: cfg.WorkflowRuntimeOptions.PollInterval,
	})
	if err != nil {
		return errors.Wrap(err, "construct workflow runtime")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	storeIns := store.Client()
	projector := ssesvc.NewProjector(storeIns.UserEvents(), cfg.SSEOptions.Retention, postgresql.SubscribeOutbox)
	if err := projector.Start(ctx); err != nil {
		return errors.Wrap(err, "start SSE source projector")
	}
	defer projector.Close()
	registrations, err := modeladapters.NewRegistrations()
	if err != nil {
		return errors.Wrap(err, "load application platform adapter registrations")
	}
	runtimeRegistry, err := engine.NewRuntimeRegistry(registrations)
	if err != nil {
		return errors.Wrap(err, "build application runtime registry")
	}
	capabilities, err := engine.NewProviderCapabilityRegistry(registrations, runtimeRegistry, modeladapters.NewCapabilityValidators()...)
	if err != nil {
		return errors.Wrap(err, "build provider capabilities")
	}
	reconcileRegistry := taskcentersvc.NewReconcileRegistry()
	functionRegistry, err := taskfunctionregistry.New()
	if err != nil {
		return errors.Wrap(err, "load task center function registry")
	}
	tasks := taskcentersvc.NewServiceWithFunctionRegistry(storeIns, runtime, reconcileRegistry, functionRegistry, nil,
		appplatformsvc.FunctionAssetThumbnailGenerate, "application-platform.run", "task.schedule.acquire",
		"comfyui.submit", "comfyui.poll", "comfyui.collect_preview",
		assetlibrarysvc.FunctionArtifactProcess, assetlibrarysvc.FunctionRepresentationInspect,
		assetlibrarysvc.FunctionRepresentationGenerate, assetlibrarysvc.FunctionRepresentationFinalize)
	agentProjector, err := agentsvc.New(agentsvc.Dependencies{Store: storeIns.Agents()})
	if err != nil {
		return errors.Wrap(err, "construct agent runtime projector")
	}
	adapters := modeladapters.NewEngineAdapters()
	executors := modeladapters.NewOperationExecutors()
	if err := modeladapters.ValidateImplementations(runtimeRegistry, adapters, executors); err != nil {
		return errors.Wrap(err, "validate application platform adapter implementations")
	}
	events := appsvc.NoopEventPublisher{}
	artifactLifecycle := &workerArtifactLifecycle{
		store: storeIns.AssetsV1(), storage: assetlibrarysvc.NewLocalContentStorage(storeIns),
		policy: assetlibrarysvc.DefaultRepresentationPolicy{},
	}
	applicationExecutor, err := appsvc.NewApplicationRunExecutor(storeIns, runtimeRegistry, capabilities, adapters, executors, artifactLifecycle, events)
	if err != nil {
		return err
	}
	thumbnailExecutor := appplatformsvc.NewThumbnailExecutor(storeIns)
	artifactProcessExecutor := assetlibrarysvc.NewArtifactProcessExecutor(storeIns)
	assetStorage := assetlibrarysvc.NewLocalContentStorage(storeIns)
	ffprobeInspector, err := assetlibrarysvc.NewLocalFFprobeMediaMetadataInspector()
	if err != nil {
		return errors.Wrap(err, "construct ffprobe metadata inspector")
	}
	mediaMetadataInspectors := assetlibrarysvc.NewMediaMetadataInspectors(assetlibrarysvc.ImageMediaMetadataInspector{}, ffprobeInspector)
	representationInspectExecutor := assetlibrarysvc.NewRepresentationInspectExecutor(storeIns, assetStorage, mediaMetadataInspectors)
	ffmpegRuntime, err := assetlibrarysvc.NewLocalFFmpegRuntime()
	if err != nil {
		return errors.Wrap(err, "construct ffmpeg runtime")
	}
	thumbnailGenerators := assetlibrarysvc.NewThumbnailGenerators(assetlibrarysvc.ImageThumbnailGenerator{}, assetlibrarysvc.NewVideoThumbnailGenerator(ffmpegRuntime))
	representationGenerateExecutor := assetlibrarysvc.NewRepresentationGenerateExecutor(storeIns, assetStorage, thumbnailGenerators)
	representationFinalizeExecutor := assetlibrarysvc.NewRepresentationFinalizeExecutor(storeIns)
	representationPolicy := assetlibrarysvc.DefaultRepresentationPolicy{}
	applicationService, err := appsvc.NewService(appsvc.Dependencies{Store: storeIns, Runtime: runtimeRegistry, Capabilities: capabilities, Adapters: adapters, Tasks: tasks, Assets: artifactLifecycle, Events: events})
	if err != nil {
		return err
	}
	applicationCatalogProjector := workflowcanvassvc.NewApplicationCatalogProjector(storeIns.WorkflowCanvases())
	if err := reconcilePublishedApplicationCatalog(ctx, applicationService, applicationCatalogProjector); err != nil {
		return errors.Wrap(err, "reconcile published application canvas catalog")
	}
	if err := applicationService.ReconcileRequiredEngineBindings(ctx); err != nil {
		return errors.Wrap(err, "reconcile required application platform bindings")
	}
	if err := reconcileRegistry.Register(engine.NewEngineHealthReconcileHandler(storeIns, applicationService)); err != nil {
		return err
	}
	if err := reconcileRegistry.Register(comfyuiadapter.NewComfyUIObjectInfoReconcileHandler(storeIns, applicationService)); err != nil {
		return err
	}
	if err := reconcileRegistry.Register(assetlibrarysvc.NewRepresentationBackfillHandler(storeIns, representationPolicy)); err != nil {
		return err
	}
	comfyTestExecutor := comfyuiadapter.NewTestExecutor(storeIns)
	if err := runtime.RegisterHandler("agent.runtime.ensure", 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, loadErr := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
		if loadErr != nil {
			return nil, errors.Wrap(loadErr, "load agent runtime ensure atomic task")
		}
		return executeAgentRuntimeEnsure(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return errors.Wrap(err, "register agent runtime ensure handler")
	}
	if err := runtime.RegisterHandler("agent.runtime.stop", 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, loadErr := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
		if loadErr != nil {
			return nil, errors.Wrap(loadErr, "load agent runtime stop atomic task")
		}
		return executeAgentRuntimeStop(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return errors.Wrap(err, "register agent runtime stop handler")
	}
	for _, registration := range []struct {
		functionRef string
		handler     func(context.Context, workflowruntime.WorkerTask, *iapiserver.AtomicTask) (map[string]any, error)
	}{
		{"appstudio.preview.ensure", func(ctx context.Context, task workflowruntime.WorkerTask, atomic *iapiserver.AtomicTask) (map[string]any, error) {
			return executeAppStudioPreviewEnsure(ctx, infrastructureClient, functionRegistry, task, atomic)
		}},
		{"appstudio.preview.stop", func(ctx context.Context, task workflowruntime.WorkerTask, atomic *iapiserver.AtomicTask) (map[string]any, error) {
			return executeAppStudioPreviewStop(ctx, infrastructureClient, functionRegistry, task, atomic)
		}},
		{"appstudio.build.execute", func(ctx context.Context, task workflowruntime.WorkerTask, atomic *iapiserver.AtomicTask) (map[string]any, error) {
			return executeAppStudioBuild(ctx, infrastructureClient, functionRegistry, task, atomic)
		}},
		{"appstudio.production.reconcile", func(ctx context.Context, task workflowruntime.WorkerTask, atomic *iapiserver.AtomicTask) (map[string]any, error) {
			return executeAppStudioProductionReconcile(ctx, infrastructureClient, functionRegistry, task, atomic)
		}},
		{"appstudio.production.stop", func(ctx context.Context, task workflowruntime.WorkerTask, atomic *iapiserver.AtomicTask) (map[string]any, error) {
			return executeAppStudioProductionStop(ctx, infrastructureClient, functionRegistry, task, atomic)
		}},
	} {
		registration := registration
		if err := runtime.RegisterHandler(registration.functionRef, 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
			atomicTask, loadErr := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
			if loadErr != nil {
				return nil, errors.Wrap(loadErr, "load appstudio atomic task")
			}
			return registration.handler(ctx, task, atomicTask)
		}); err != nil {
			return errors.Wrap(err, "register appstudio handler")
		}
	}
	if err := runtime.RegisterHandler("comfyui.submit", 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		output, err := comfyTestExecutor.Submit(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
		if err == nil {
			task.Log(ctx, workflowruntime.WorkerLog("comfyui.submit.ready", workflowruntime.TaskLogLevelInfo, "External job is ready for polling."))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("comfyui.poll", 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		output, err := comfyTestExecutor.Poll(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
		if err == nil {
			if waiting, _ := output["in_progress"].(bool); waiting {
				key, message := "comfyui.poll.running", "External job is still running."
				if output["queue_position"] != nil {
					key, message = "comfyui.poll.queued", "External job is queued."
				}
				task.Log(ctx, workflowruntime.WorkerLog(key, workflowruntime.TaskLogLevelInfo, message))
			} else {
				task.Log(ctx, workflowruntime.WorkerLog("comfyui.poll.completed", workflowruntime.TaskLogLevelInfo, "External job completed."))
			}
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("comfyui.collect_preview", 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		output, err := comfyTestExecutor.Collect(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
		if err == nil {
			task.Log(ctx, workflowruntime.WorkerLog("comfyui.preview.collected", workflowruntime.TaskLogLevelInfo, fmt.Sprintf("Collected %v preview outputs.", output["output_count"])))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("application-platform.run", 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, err := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
		if err != nil {
			return nil, err
		}
		if atomicTask.CanvasRunID != "" {
			run, ensureErr := applicationService.EnsureCanvasApplicationRun(ctx, &appsvc.CanvasApplicationRunRequest{
				AtomicTaskID:         atomicTask.ID,
				CanvasRunID:          atomicTask.CanvasRunID,
				CanvasNodeRunID:      atomicTask.CanvasNodeRunID,
				ExecutionKey:         atomicTask.ChildKey,
				ApplicationVersionID: fmt.Sprint(task.Arguments["application_version_id"]),
				OwnerUserID:          atomicTask.CreatedBy,
				Inputs:               workerMap(task.Arguments["resolved_inputs"]),
				Arguments:            task.Arguments,
			})
			if ensureErr != nil {
				return nil, ensureErr
			}
			atomicTask.ApplicationRunID = run.ID
			atomicTask.Arguments = task.Arguments
		}
		checkpoint, err := task.LoadCheckpoint(ctx)
		if err != nil {
			return nil, err
		}
		output, err := applicationExecutor.ExecuteWithCheckpoint(ctx, atomicTask, checkpoint)
		if err == nil {
			if inProgress, _ := output["in_progress"].(bool); inProgress {
				task.Log(ctx, workflowruntime.WorkerLog("application.execution.waiting", workflowruntime.TaskLogLevelInfo, "External application job is waiting for the next callback."))
			} else {
				task.Log(ctx, workflowruntime.WorkerLog("application.execution.completed", workflowruntime.TaskLogLevelInfo, "Application provider execution returned a result."))
			}
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(appplatformsvc.FunctionAssetThumbnailGenerate, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, err := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
		if err != nil {
			return nil, err
		}
		output, err := thumbnailExecutor.Execute(ctx, atomicTask)
		if err == nil {
			message := "Thumbnail processing completed."
			if output["thumbnail_status"] == iapiserver.ThumbnailStatusUnsupported {
				message = "Thumbnail generation is unsupported for this asset."
			}
			task.Log(ctx, workflowruntime.WorkerLog("asset.thumbnail.completed", workflowruntime.TaskLogLevelInfo, message))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(assetlibrarysvc.FunctionArtifactProcess, 16, artifactProcessExecutor.Execute); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(assetlibrarysvc.FunctionRepresentationInspect, 16, representationInspectExecutor.Execute); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(assetlibrarysvc.FunctionRepresentationGenerate, 8, representationGenerateExecutor.Execute); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(assetlibrarysvc.FunctionRepresentationFinalize, 16, representationFinalizeExecutor.Execute); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(taskcentersvc.ReconcileControllerTask, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		scheduleID, _ := task.Arguments["task_schedule_id"].(string)
		output, err := tasks.RunScheduleReconcile(ctx, scheduleID, task.WorkflowID, scheduleTime(task.Arguments["scheduled_at"]))
		if err == nil {
			message := "Reconcile cycle completed."
			if summary, ok := output["reconcile_summary"].(iapiserver.ReconcileSummary); ok {
				message = fmt.Sprintf("Reconcile cycle completed with %d scanned, %d findings, %d actions, and %d deferred.", summary.Scanned, summary.Findings, summary.ActionsCreated, summary.Deferred)
			}
			task.Log(ctx, workflowruntime.WorkerLog("schedule.reconcile.completed", workflowruntime.TaskLogLevelInfo, message))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(taskcentersvc.ManualScheduleControllerTask, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		executionID, _ := task.Arguments["schedule_execution_id"].(string)
		return tasks.RunManualScheduleExecution(ctx, executionID, task.WorkflowID)
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("task.schedule.acquire", 1, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		task.Log(ctx, workflowruntime.WorkerLog("schedule.acquire.started", workflowruntime.TaskLogLevelInfo, "Evaluating scheduled execution ownership."))
		scheduleID, _ := task.Arguments["task_schedule_id"].(string)
		schedule, err := storeIns.TaskCenters().GetTaskSchedule(ctx, scheduleID)
		if err != nil {
			return nil, err
		}
		scheduledAt := scheduleTime(task.Arguments["scheduled_at"])
		if taskcentersvc.ScheduleTriggerMisfired(scheduledAt, time.Now()) {
			existing, getErr := storeIns.TaskCenters().GetScheduleExecutionAt(ctx, schedule.ID, scheduledAt)
			if getErr != nil {
				return nil, getErr
			}
			if existing == nil {
				task.Log(ctx, workflowruntime.WorkerLog("schedule.acquire.misfire", workflowruntime.TaskLogLevelWarn, "Delayed schedule trigger was skipped by misfire policy."))
				return map[string]any{
					"status":       iapiserver.TaskSchedulePolicySkip,
					"scheduled_at": scheduledAt.UTC().Format(time.RFC3339Nano),
					"reason":       "misfire policy skipped delayed schedule trigger",
				}, nil
			}
		}
		execution := &iapiserver.TaskScheduleExecution{ScheduleID: schedule.ID, ExecutionMode: iapiserver.TaskScheduleModeMaterialized, TriggerSource: iapiserver.ScheduleExecutionTriggerSchedule, TriggeredBy: schedule.CreatedBy, ScheduledAt: imachinery.NewTime(scheduledAt), TriggeredAt: imachinery.Now(), TargetType: schedule.Target.Type, RuntimeExecutionID: task.WorkflowID, Status: iapiserver.ScheduleExecutionStatusTriggered}
		execution.ID = uuid.NewString()
		execution.Name = "Schedule execution"
		record, acquired, err := storeIns.TaskCenters().AcquireScheduleExecution(ctx, execution)
		if err != nil {
			return nil, err
		}
		if !acquired {
			task.Log(ctx, workflowruntime.WorkerLog("schedule.acquire.overlap", workflowruntime.TaskLogLevelWarn, "Schedule trigger reused an existing execution record."))
			return map[string]any{"schedule_execution_id": record.ID, "status": record.Status}, nil
		}
		targetID, err := createScheduleTarget(ctx, tasks, schedule, record.TriggeredAt)
		if err != nil {
			record.Status = iapiserver.ScheduleExecutionStatusTriggerFailed
			record.Reason = err.Error()
			record.CompletedAt = imachinery.Now()
			_, _ = storeIns.TaskCenters().UpdateScheduleExecution(ctx, record)
			return nil, err
		}
		record.TargetID = targetID
		record.Status = iapiserver.ScheduleExecutionStatusRunning
		_, err = storeIns.TaskCenters().UpdateScheduleExecution(ctx, record)
		if err == nil {
			task.Log(ctx, workflowruntime.WorkerLog("schedule.target.created", workflowruntime.TaskLogLevelInfo, "Scheduled target was created and started."))
		}
		return map[string]any{"schedule_execution_id": record.ID, "target_id": targetID, "status": record.Status}, err
	}); err != nil {
		return err
	}
	if err := startAssetLibraryTaskConsumers(ctx, tasks, appsvc.NewApplicationArtifactProjector(storeIns.ApplicationPlatforms())); err != nil {
		return err
	}
	if err := startApplicationRunTerminalProjectionConsumer(ctx, storeIns.TaskCenters(), applicationExecutor); err != nil {
		return err
	}
	if err := startCanvasApplicationConsumers(
		ctx,
		applicationCatalogProjector,
		workflowcanvassvc.NewApplicationArtifactProjector(storeIns.TaskCenters(), storeIns.WorkflowCanvases()),
	); err != nil {
		return err
	}
	messages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicAssetUploaded, "task-center-thumbnail")
	if err != nil {
		return err
	}
	go func() {
		for msg := range messages {
			var event struct {
				AssetID        string `json:"asset_id"`
				AssetVersionID string `json:"asset_version_id"`
				OwnerUserID    string `json:"owner_user_id"`
				ProjectID      string `json:"project_id"`
				Namespace      string `json:"namespace"`
				MediaType      string `json:"media_type"`
				ProfileVersion string `json:"profile_version"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err != nil {
				msg.Nack()
				continue
			}
			if event.AssetVersionID != "" {
				msg.Ack()
				continue
			}
			thumbnail, err := storeIns.AssetThumbnails().GetByAsset(ctx, event.AssetID)
			if err == nil {
				_, err = tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{Key: "thumbnail", Name: "Generate asset thumbnail", FunctionRef: appplatformsvc.FunctionAssetThumbnailGenerate, Arguments: map[string]any{"asset_id": event.AssetID, "thumbnail_id": thumbnail.ID}, RequiredCapabilities: appplatformsvc.CapabilityAssetThumbnail, ProjectID: event.ProjectID, Namespace: event.Namespace, IdempotencyScope: "asset-thumbnail", IdempotencyKey: "thumbnail:" + event.AssetID + ":" + event.ProfileVersion, SystemName: iapiserver.SystemNameSpec{Key: taskname.AssetThumbnail}})
			}
			if err != nil {
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()
	if err := ensureEngineHealthSchedule(ctx, tasks, cfg.ApplicationPlatformOptions.EngineHealthInterval); err != nil {
		return err
	}
	if err := ensureComfyUIObjectInfoSchedule(ctx, tasks); err != nil {
		return err
	}
	if err := ensureRepresentationBackfillSchedule(ctx, tasks); err != nil {
		return err
	}
	reconciler := taskcentersvc.NewReconciler(
		storeIns,
		runtime,
		cfg.WorkflowRuntimeOptions.ReconcileInterval,
		applicationExecutor.Completed,
		agentProjector.ProjectTaskTerminal,
		storeIns.AppStudio().ProjectStudioTaskTerminal,
	)
	errCh := make(chan error, 1)
	go func() { errCh <- reconciler.Run(ctx) }()
	select {
	case <-ctx.Done():
		_ = runtime.Close()
		return nil
	case err := <-errCh:
		_ = runtime.Close()
		return err
	}
}

type infrastructureCommandExecutor interface {
	Execute(context.Context, *infrastructure.CommandRequest) (*infrastructure.CommandResponse, error)
}

type agentRuntimeEnsureArguments struct {
	AgentID                 string                              `json:"agent_id"`
	AgentRuntimeID          string                              `json:"agent_runtime_id"`
	ExistingInfraRuntimeID  *string                             `json:"existing_infra_runtime_id"`
	Operation               string                              `json:"operation"`
	AgentKind               string                              `json:"agent_kind"`
	WorkspaceType           string                              `json:"workspace_type"`
	WorkspaceID             string                              `json:"workspace_id"`
	WorkspaceSourceRef      *string                             `json:"workspace_source_ref"`
	RuntimeProfileID        string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision  string                              `json:"runtime_profile_revision"`
	ModelAccessSpecRef      string                              `json:"model_access_spec_ref"`
	RuntimeConfigurationRef string                              `json:"runtime_configuration_ref"`
	AuthorizationRef        string                              `json:"authorization_ref"`
	ExpectedResourceVersion int64                               `json:"expected_resource_version"`
	ResourceRequirement     iapiserver.InfraResourceRequirement `json:"resource_requirement"`
	LifecyclePolicy         agentRuntimeLifecyclePolicy         `json:"lifecycle_policy"`
}

type agentRuntimeLifecyclePolicy struct {
	RestartPolicy          string `json:"restart_policy"`
	IdleTimeoutSeconds     *int   `json:"idle_timeout_seconds"`
	MaximumLifetimeSeconds *int   `json:"maximum_lifetime_seconds"`
}

type agentRuntimeStopArguments struct {
	AgentID                 string `json:"agent_id"`
	AgentRuntimeID          string `json:"agent_runtime_id"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	Action                  string `json:"action"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

type appStudioPreviewEnsureArguments struct {
	StudioApplicationID        string                              `json:"studio_application_id"`
	PreviewRuntimeID           string                              `json:"preview_runtime_id"`
	ExistingInfraRuntimeID     *string                             `json:"existing_infra_runtime_id"`
	WorkspaceID                string                              `json:"workspace_id"`
	WorkspaceRevision          int64                               `json:"workspace_revision"`
	WorkspaceRevisionSourceRef string                              `json:"workspace_revision_source_ref"`
	RuntimeProfileID           string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision     string                              `json:"runtime_profile_revision"`
	EndpointVisibility         string                              `json:"endpoint_visibility"`
	AuthorizationRef           string                              `json:"authorization_ref"`
	ExpectedResourceVersion    int64                               `json:"expected_resource_version"`
	ResourceRequirement        iapiserver.InfraResourceRequirement `json:"resource_requirement"`
}

type appStudioPreviewStopArguments struct {
	StudioApplicationID     string `json:"studio_application_id"`
	PreviewRuntimeID        string `json:"preview_runtime_id"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	Action                  string `json:"action"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

type appStudioBuildArguments struct {
	StudioApplicationID     string                              `json:"studio_application_id"`
	StudioBuildID           string                              `json:"studio_build_id"`
	SourceSnapshotID        string                              `json:"source_snapshot_id"`
	SourceSnapshotDigest    string                              `json:"source_snapshot_digest"`
	SourceSnapshotSourceRef string                              `json:"source_snapshot_source_ref"`
	RuntimeProfileID        string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision  string                              `json:"runtime_profile_revision"`
	BuildConfigRef          string                              `json:"build_config_ref"`
	DependencyLockDigest    string                              `json:"dependency_lock_digest"`
	AuthorizationRef        string                              `json:"authorization_ref"`
	ExpectedResourceVersion int64                               `json:"expected_resource_version"`
	ResourceRequirement     iapiserver.InfraResourceRequirement `json:"resource_requirement"`
}

type appStudioProductionReconcileArguments struct {
	StudioApplicationID        string                              `json:"studio_application_id"`
	StudioReleaseID            string                              `json:"studio_release_id"`
	StudioRuntimeInstanceID    string                              `json:"studio_runtime_instance_id"`
	ExistingInfraRuntimeID     *string                             `json:"existing_infra_runtime_id"`
	StudioApplicationVersionID string                              `json:"studio_application_version_id"`
	RuntimeConfigID            string                              `json:"runtime_config_id"`
	ArtifactID                 string                              `json:"artifact_id"`
	ArtifactDigest             string                              `json:"artifact_digest"`
	ArtifactSourceRef          string                              `json:"artifact_source_ref"`
	Environment                string                              `json:"environment"`
	DeploymentReason           string                              `json:"deployment_reason"`
	RuntimeProfileID           string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision     string                              `json:"runtime_profile_revision"`
	HealthCheckRef             string                              `json:"health_check_ref"`
	EndpointVisibility         string                              `json:"endpoint_visibility"`
	AuthorizationRef           string                              `json:"authorization_ref"`
	ExpectedResourceVersion    int64                               `json:"expected_resource_version"`
	ResourceRequirement        iapiserver.InfraResourceRequirement `json:"resource_requirement"`
}

type appStudioProductionStopArguments struct {
	StudioApplicationID     string `json:"studio_application_id"`
	StudioReleaseID         string `json:"studio_release_id"`
	StudioRuntimeInstanceID string `json:"studio_runtime_instance_id"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

func executeAgentRuntimeEnsure(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, err := resolveAgentRuntimeContract(registry, workerTask, atomicTask, "agent.runtime.ensure")
	if err != nil {
		return nil, err
	}
	rawArguments, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "encode agent runtime ensure arguments")
	}
	var arguments agentRuntimeEnsureArguments
	if err := json.Unmarshal(rawArguments, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode agent runtime ensure arguments")
	}
	if err := validateAgentRuntimeEnsureArguments(arguments); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}

	command := &infrastructure.CommandRequest{}
	if arguments.ExistingInfraRuntimeID != nil {
		command.Operation = "start"
		command.RuntimeID = *arguments.ExistingInfraRuntimeID
	} else {
		sourceRef := ""
		if arguments.WorkspaceSourceRef != nil {
			sourceRef = *arguments.WorkspaceSourceRef
		}
		timeoutPolicy := iapiserver.InfraTimeoutPolicy{}
		if arguments.LifecyclePolicy.IdleTimeoutSeconds != nil {
			timeoutPolicy.IdleTimeoutSeconds = *arguments.LifecyclePolicy.IdleTimeoutSeconds
		}
		if arguments.LifecyclePolicy.MaximumLifetimeSeconds != nil {
			timeoutPolicy.MaximumLifetimeSeconds = *arguments.LifecyclePolicy.MaximumLifetimeSeconds
		}
		command.Operation = "create"
		command.Create = &iapiserver.InfraCreateRuntimeRequest{
			RequestID:              fmt.Sprintf("%s:%d", atomicTask.ID, workerTask.RetryCount+1),
			RequestingService:      "task-center",
			OwnerDomain:            "agent",
			OwnerReference:         arguments.AgentRuntimeID,
			RequestUserID:          atomicTask.CreatedBy,
			RuntimeMode:            "SERVICE",
			RuntimeProfileID:       arguments.RuntimeProfileID,
			RuntimeProfileRevision: arguments.RuntimeProfileRevision,
			SourceRef:              sourceRef,
			ResourceRequirement:    arguments.ResourceRequirement,
			TimeoutPolicy:          timeoutPolicy,
			AuthorizationRef:       arguments.AuthorizationRef,
			EndpointVisibility:     "INTERNAL",
			FunctionRef:            contract.FunctionRef,
			FunctionArguments:      rawArguments,
		}
	}
	response, err := client.Execute(ctx, command)
	if err != nil {
		return nil, errors.Wrap(err, "execute agent runtime ensure infrastructure command")
	}
	runtime, err := requireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate agent runtime ensure response")
	}
	if arguments.ExistingInfraRuntimeID != nil && runtime.ID != *arguments.ExistingInfraRuntimeID {
		return nil, fmt.Errorf("agent runtime ensure returned unexpected infrastructure runtime %q", runtime.ID)
	}
	if runtime.Status != "RUNNING" {
		return nil, fmt.Errorf("agent runtime ensure returned runtime status %q", runtime.Status)
	}
	if response.Result.Endpoint == nil || response.Result.Endpoint.ID == "" || response.Result.Endpoint.Status != "READY" || runtime.EndpointRef != response.Result.Endpoint.ID {
		return nil, fmt.Errorf("agent runtime ensure returned an invalid ready endpoint")
	}
	result := map[string]any{
		"infra_runtime_id":    runtime.ID,
		"runtime_status":      "RUNNING",
		"health_status":       "HEALTHY",
		"endpoint_ref":        runtime.EndpointRef,
		"diagnostics_summary": map[string]any{},
	}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate agent runtime ensure output")
	}
	return result, nil
}

func executeAgentRuntimeStop(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, err := resolveAgentRuntimeContract(registry, workerTask, atomicTask, "agent.runtime.stop")
	if err != nil {
		return nil, err
	}
	rawArguments, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "encode agent runtime stop arguments")
	}
	var arguments agentRuntimeStopArguments
	if err := json.Unmarshal(rawArguments, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode agent runtime stop arguments")
	}
	if err := validateAgentRuntimeStopArguments(arguments); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}
	response, err := client.Execute(ctx, &infrastructure.CommandRequest{
		Operation: "stop",
		RuntimeID: arguments.InfraRuntimeID,
		Delete:    arguments.Action == "DELETE",
	})
	if err != nil {
		return nil, errors.Wrap(err, "execute agent runtime stop infrastructure command")
	}
	runtime, err := requireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate agent runtime stop response")
	}
	if runtime.ID != arguments.InfraRuntimeID {
		return nil, fmt.Errorf("agent runtime stop returned unexpected infrastructure runtime %q", runtime.ID)
	}
	expectedStatus := "STOPPED"
	if arguments.Action == "DELETE" {
		expectedStatus = "DELETED"
	}
	if runtime.Status != expectedStatus {
		return nil, fmt.Errorf("agent runtime stop returned runtime status %q, want %q", runtime.Status, expectedStatus)
	}
	result := map[string]any{
		"infra_runtime_id": runtime.ID,
		"runtime_status":   runtime.Status,
		"completed_action": arguments.Action,
	}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate agent runtime stop output")
	}
	return result, nil
}

func executeAppStudioPreviewEnsure(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
	contract, raw, err := resolveAppStudioArguments(registry, workerTask, atomicTask, "appstudio.preview.ensure")
	if err != nil {
		return nil, err
	}
	var arguments appStudioPreviewEnsureArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode appstudio preview ensure arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.PreviewRuntimeID == "" || arguments.WorkspaceID == "" || arguments.WorkspaceRevision < 0 || arguments.RuntimeProfileRevision == "" || arguments.ExpectedResourceVersion < 0 {
		return nil, fmt.Errorf("appstudio preview ensure arguments are incomplete")
	}
	if !strings.HasPrefix(arguments.WorkspaceRevisionSourceRef, "studio-workspace-revision://") || !strings.HasPrefix(arguments.AuthorizationRef, "appstudio-preview-grant://") {
		return nil, fmt.Errorf("appstudio preview ensure references are invalid")
	}
	if arguments.RuntimeProfileID != "appstudio.preview.static-web" && arguments.RuntimeProfileID != "appstudio.preview.web-backend" {
		return nil, fmt.Errorf("appstudio preview ensure profile is invalid")
	}
	if arguments.EndpointVisibility != "USER_ACCESSIBLE" || invalidResourceRequirement(arguments.ResourceRequirement) {
		return nil, fmt.Errorf("appstudio preview ensure runtime configuration is invalid")
	}
	command := appStudioEnsureCommand(atomicTask, workerTask, contract, raw, arguments.ExistingInfraRuntimeID, arguments.PreviewRuntimeID, arguments.RuntimeProfileID, arguments.RuntimeProfileRevision, arguments.WorkspaceRevisionSourceRef, arguments.AuthorizationRef, arguments.EndpointVisibility, arguments.ResourceRequirement)
	return executeAppStudioReady(ctx, client, registry, contract, command, arguments.ExistingInfraRuntimeID, "preview ensure")
}

func executeAppStudioPreviewStop(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
	contract, raw, err := resolveAppStudioArguments(registry, workerTask, atomicTask, "appstudio.preview.stop")
	if err != nil {
		return nil, err
	}
	var arguments appStudioPreviewStopArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode appstudio preview stop arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.PreviewRuntimeID == "" || arguments.InfraRuntimeID == "" || arguments.ExpectedResourceVersion < 0 || !strings.HasPrefix(arguments.AuthorizationRef, "appstudio-preview-grant://") {
		return nil, fmt.Errorf("appstudio preview stop arguments are invalid")
	}
	if arguments.Action != "STOP" && arguments.Action != "DELETE" {
		return nil, fmt.Errorf("appstudio preview stop action is invalid")
	}
	return executeAppStudioStop(ctx, client, registry, contract, arguments.InfraRuntimeID, arguments.Action, arguments.Action == "DELETE", "preview stop")
}

func executeAppStudioBuild(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
	_, raw, err := resolveAppStudioArguments(registry, workerTask, atomicTask, "appstudio.build.execute")
	if err != nil {
		return nil, err
	}
	var arguments appStudioBuildArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode appstudio build arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.StudioBuildID == "" || arguments.SourceSnapshotID == "" || arguments.SourceSnapshotDigest == "" || arguments.RuntimeProfileRevision == "" || arguments.DependencyLockDigest == "" || arguments.ExpectedResourceVersion < 0 {
		return nil, fmt.Errorf("appstudio build arguments are incomplete")
	}
	if !strings.HasPrefix(arguments.SourceSnapshotSourceRef, "studio-snapshot://") || !strings.HasPrefix(arguments.BuildConfigRef, "appstudio-build-config://") || !strings.HasPrefix(arguments.AuthorizationRef, "appstudio-build-grant://") {
		return nil, fmt.Errorf("appstudio build references are invalid")
	}
	if arguments.RuntimeProfileID != "appstudio.build.static-web" && arguments.RuntimeProfileID != "appstudio.build.web-backend" {
		return nil, fmt.Errorf("appstudio build profile is invalid")
	}
	if invalidResourceRequirement(arguments.ResourceRequirement) {
		return nil, fmt.Errorf("appstudio build resource requirement is invalid")
	}
	return nil, fmt.Errorf("appstudio build artifact registration is unavailable: infrastructure output artifact identity is not verified by Asset Library")
}

func executeAppStudioProductionReconcile(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
	contract, raw, err := resolveAppStudioArguments(registry, workerTask, atomicTask, "appstudio.production.reconcile")
	if err != nil {
		return nil, err
	}
	var arguments appStudioProductionReconcileArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode appstudio production reconcile arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.StudioReleaseID == "" || arguments.StudioRuntimeInstanceID == "" || arguments.StudioApplicationVersionID == "" || arguments.RuntimeConfigID == "" || arguments.ArtifactID == "" || arguments.ArtifactDigest == "" || arguments.RuntimeProfileRevision == "" || arguments.ExpectedResourceVersion < 0 {
		return nil, fmt.Errorf("appstudio production reconcile arguments are incomplete")
	}
	if !strings.HasPrefix(arguments.ArtifactSourceRef, "artifact://") || !strings.HasPrefix(arguments.HealthCheckRef, "appstudio-health-check://") || !strings.HasPrefix(arguments.AuthorizationRef, "appstudio-release-grant://") {
		return nil, fmt.Errorf("appstudio production reconcile references are invalid")
	}
	if arguments.Environment != "preview" && arguments.Environment != "production" {
		return nil, fmt.Errorf("appstudio production environment is invalid")
	}
	if arguments.DeploymentReason != "DEPLOY" && arguments.DeploymentReason != "UPGRADE" && arguments.DeploymentReason != "ROLLBACK" {
		return nil, fmt.Errorf("appstudio production deployment reason is invalid")
	}
	if arguments.RuntimeProfileID != "studioapp.runtime.static-web" && arguments.RuntimeProfileID != "studioapp.runtime.web-backend" {
		return nil, fmt.Errorf("appstudio production profile is invalid")
	}
	if arguments.EndpointVisibility != "INTERNAL" && arguments.EndpointVisibility != "USER_ACCESSIBLE" {
		return nil, fmt.Errorf("appstudio production endpoint visibility is invalid")
	}
	if invalidResourceRequirement(arguments.ResourceRequirement) {
		return nil, fmt.Errorf("appstudio production resource requirement is invalid")
	}
	command := appStudioEnsureCommand(atomicTask, workerTask, contract, raw, arguments.ExistingInfraRuntimeID, arguments.StudioRuntimeInstanceID, arguments.RuntimeProfileID, arguments.RuntimeProfileRevision, arguments.ArtifactSourceRef, arguments.AuthorizationRef, arguments.EndpointVisibility, arguments.ResourceRequirement)
	return executeAppStudioReady(ctx, client, registry, contract, command, arguments.ExistingInfraRuntimeID, "production reconcile")
}

func executeAppStudioProductionStop(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
	contract, raw, err := resolveAppStudioArguments(registry, workerTask, atomicTask, "appstudio.production.stop")
	if err != nil {
		return nil, err
	}
	var arguments appStudioProductionStopArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode appstudio production stop arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.StudioReleaseID == "" || arguments.StudioRuntimeInstanceID == "" || arguments.InfraRuntimeID == "" || arguments.ExpectedResourceVersion < 0 || !strings.HasPrefix(arguments.AuthorizationRef, "appstudio-release-grant://") {
		return nil, fmt.Errorf("appstudio production stop arguments are invalid")
	}
	return executeAppStudioStop(ctx, client, registry, contract, arguments.InfraRuntimeID, "STOP", false, "production stop")
}

func resolveAppStudioArguments(registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask, expectedFunctionRef string) (*taskfunctionregistry.Contract, []byte, error) {
	if registry == nil {
		return nil, nil, fmt.Errorf("task function registry is required")
	}
	if atomicTask == nil || workerTask.AtomicTaskID == "" || atomicTask.ID != workerTask.AtomicTaskID {
		return nil, nil, fmt.Errorf("appstudio atomic task identity is invalid")
	}
	if workerTask.RetryCount < 0 || atomicTask.FunctionRef != expectedFunctionRef || atomicTask.FunctionContractVersion == "" || atomicTask.FunctionContractDigest == "" || atomicTask.Arguments == nil {
		return nil, nil, fmt.Errorf("appstudio atomic task contract pin is invalid")
	}
	contract, err := registry.Resolve(atomicTask.FunctionRef, atomicTask.FunctionContractVersion, atomicTask.FunctionContractDigest)
	if err != nil {
		return nil, nil, errors.Wrap(err, "resolve appstudio atomic task contract")
	}
	raw, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, nil, errors.Wrap(err, "encode appstudio arguments")
	}
	return contract, raw, nil
}

func appStudioEnsureCommand(atomicTask *iapiserver.AtomicTask, workerTask workflowruntime.WorkerTask, contract *taskfunctionregistry.Contract, raw []byte, existing *string, ownerReference, profileID, profileRevision, sourceRef, authorizationRef, endpointVisibility string, resource iapiserver.InfraResourceRequirement) *infrastructure.CommandRequest {
	if existing != nil {
		return &infrastructure.CommandRequest{Operation: "start", RuntimeID: *existing}
	}
	return &infrastructure.CommandRequest{Operation: "create", Create: &iapiserver.InfraCreateRuntimeRequest{
		RequestID: fmt.Sprintf("%s:%d", atomicTask.ID, workerTask.RetryCount+1), RequestingService: "task-center", OwnerDomain: "appstudio", OwnerReference: ownerReference, RequestUserID: atomicTask.CreatedBy,
		RuntimeMode: "SERVICE", RuntimeProfileID: profileID, RuntimeProfileRevision: profileRevision, SourceRef: sourceRef, ResourceRequirement: resource, AuthorizationRef: authorizationRef, EndpointVisibility: endpointVisibility, FunctionRef: contract.FunctionRef, FunctionArguments: raw,
	}}
}

func executeAppStudioReady(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, contract *taskfunctionregistry.Contract, command *infrastructure.CommandRequest, existing *string, operation string) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}
	response, err := client.Execute(ctx, command)
	if err != nil {
		return nil, errors.Wrap(err, "execute appstudio "+operation+" infrastructure command")
	}
	runtime, err := requireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" response")
	}
	if existing != nil && runtime.ID != *existing {
		return nil, fmt.Errorf("appstudio %s returned unexpected infrastructure runtime %q", operation, runtime.ID)
	}
	if runtime.Status != "RUNNING" || response.Result.Endpoint == nil || response.Result.Endpoint.ID == "" || response.Result.Endpoint.Status != "READY" || runtime.EndpointRef != response.Result.Endpoint.ID {
		return nil, fmt.Errorf("appstudio %s returned an invalid ready runtime", operation)
	}
	result := map[string]any{"infra_runtime_id": runtime.ID, "runtime_status": "RUNNING", "health_status": "HEALTHY", "endpoint_ref": runtime.EndpointRef, "diagnostics_summary": map[string]any{}}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" output")
	}
	return result, nil
}

func executeAppStudioStop(ctx context.Context, client infrastructureCommandExecutor, registry *taskfunctionregistry.Registry, contract *taskfunctionregistry.Contract, runtimeID, action string, deleteRuntime bool, operation string) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}
	response, err := client.Execute(ctx, &infrastructure.CommandRequest{Operation: "stop", RuntimeID: runtimeID, Delete: deleteRuntime})
	if err != nil {
		return nil, errors.Wrap(err, "execute appstudio "+operation+" infrastructure command")
	}
	runtime, err := requireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" response")
	}
	wantStatus := "STOPPED"
	if deleteRuntime {
		wantStatus = "DELETED"
	}
	if runtime.ID != runtimeID || runtime.Status != wantStatus {
		return nil, fmt.Errorf("appstudio %s returned unexpected runtime result", operation)
	}
	result := map[string]any{"infra_runtime_id": runtime.ID, "runtime_status": runtime.Status, "completed_action": action}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" output")
	}
	return result, nil
}

func invalidResourceRequirement(resource iapiserver.InfraResourceRequirement) bool {
	return resource.CPUCores < 0 || resource.MemoryMB < 0 || resource.DiskMB < 0 || resource.GPUCount < 0 || resource.GPUMemoryMB < 0
}

func resolveAgentRuntimeContract(
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
	expectedFunctionRef string,
) (*taskfunctionregistry.Contract, error) {
	if registry == nil {
		return nil, fmt.Errorf("task function registry is required")
	}
	if atomicTask == nil {
		return nil, fmt.Errorf("agent runtime atomic task is missing")
	}
	if workerTask.AtomicTaskID == "" || atomicTask.ID != workerTask.AtomicTaskID {
		return nil, fmt.Errorf("agent runtime atomic task identity does not match worker task")
	}
	if workerTask.RetryCount < 0 {
		return nil, fmt.Errorf("agent runtime worker retry count is invalid")
	}
	if atomicTask.FunctionRef != expectedFunctionRef {
		return nil, fmt.Errorf("agent runtime atomic task function is %q, want %q", atomicTask.FunctionRef, expectedFunctionRef)
	}
	if atomicTask.FunctionContractVersion == "" || atomicTask.FunctionContractDigest == "" {
		return nil, fmt.Errorf("agent runtime atomic task contract pin is missing")
	}
	if atomicTask.Arguments == nil {
		return nil, fmt.Errorf("agent runtime atomic task arguments are missing")
	}
	contract, err := registry.Resolve(atomicTask.FunctionRef, atomicTask.FunctionContractVersion, atomicTask.FunctionContractDigest)
	if err != nil {
		return nil, errors.Wrap(err, "resolve agent runtime atomic task contract")
	}
	return contract, nil
}

func validateAgentRuntimeEnsureArguments(arguments agentRuntimeEnsureArguments) error {
	if arguments.AgentID == "" || arguments.AgentRuntimeID == "" || arguments.WorkspaceID == "" ||
		arguments.RuntimeProfileRevision == "" || arguments.ExpectedResourceVersion < 0 {
		return fmt.Errorf("agent runtime ensure arguments are incomplete")
	}
	if arguments.Operation != "START" && arguments.Operation != "RECOVER" {
		return fmt.Errorf("agent runtime ensure operation %q is invalid", arguments.Operation)
	}
	if arguments.AgentKind != "platform" && arguments.AgentKind != "coding" {
		return fmt.Errorf("agent runtime ensure agent kind %q is invalid", arguments.AgentKind)
	}
	if arguments.WorkspaceType != "agent" && arguments.WorkspaceType != "studio" {
		return fmt.Errorf("agent runtime ensure workspace type %q is invalid", arguments.WorkspaceType)
	}
	if arguments.RuntimeProfileID != "agent.hermes" && arguments.RuntimeProfileID != "agent.coding" {
		return fmt.Errorf("agent runtime ensure profile %q is invalid", arguments.RuntimeProfileID)
	}
	if !strings.HasPrefix(arguments.ModelAccessSpecRef, "model-access://") ||
		!strings.HasPrefix(arguments.RuntimeConfigurationRef, "agent-runtime-config://") ||
		!strings.HasPrefix(arguments.AuthorizationRef, "agent-runtime-grant://") {
		return fmt.Errorf("agent runtime ensure references are invalid")
	}
	if arguments.ExistingInfraRuntimeID != nil && strings.TrimSpace(*arguments.ExistingInfraRuntimeID) == "" {
		return fmt.Errorf("agent runtime ensure existing infrastructure runtime id is invalid")
	}
	if arguments.WorkspaceSourceRef != nil && !strings.HasPrefix(*arguments.WorkspaceSourceRef, "agent-workspace://") {
		return fmt.Errorf("agent runtime ensure workspace source reference is invalid")
	}
	resource := arguments.ResourceRequirement
	if resource.CPUCores < 0 || resource.MemoryMB < 0 || resource.DiskMB < 0 || resource.GPUCount < 0 || resource.GPUMemoryMB < 0 {
		return fmt.Errorf("agent runtime ensure resource requirement is invalid")
	}
	lifecycle := arguments.LifecyclePolicy
	if lifecycle.RestartPolicy != "" && lifecycle.RestartPolicy != "NEVER" && lifecycle.RestartPolicy != "ON_FAILURE" && lifecycle.RestartPolicy != "ALWAYS" {
		return fmt.Errorf("agent runtime ensure restart policy %q is invalid", lifecycle.RestartPolicy)
	}
	if lifecycle.IdleTimeoutSeconds != nil && *lifecycle.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("agent runtime ensure idle timeout is invalid")
	}
	if lifecycle.MaximumLifetimeSeconds != nil && *lifecycle.MaximumLifetimeSeconds < 1 {
		return fmt.Errorf("agent runtime ensure maximum lifetime is invalid")
	}
	return nil
}

func validateAgentRuntimeStopArguments(arguments agentRuntimeStopArguments) error {
	if arguments.AgentID == "" || arguments.AgentRuntimeID == "" || arguments.InfraRuntimeID == "" || arguments.ExpectedResourceVersion < 0 {
		return fmt.Errorf("agent runtime stop arguments are incomplete")
	}
	if arguments.Action != "SUSPEND" && arguments.Action != "STOP" && arguments.Action != "DELETE" {
		return fmt.Errorf("agent runtime stop action %q is invalid", arguments.Action)
	}
	if !strings.HasPrefix(arguments.AuthorizationRef, "agent-runtime-grant://") {
		return fmt.Errorf("agent runtime stop authorization reference is invalid")
	}
	return nil
}

func requireInfrastructureRuntime(response *infrastructure.CommandResponse) (*iapiserver.InfraRuntime, error) {
	if response == nil {
		return nil, fmt.Errorf("infrastructure command response is missing")
	}
	if response.Result == nil {
		return nil, fmt.Errorf("infrastructure command result is missing")
	}
	if response.Result.Runtime == nil || response.Result.Runtime.ID == "" {
		return nil, fmt.Errorf("infrastructure runtime result is missing")
	}
	return response.Result.Runtime, nil
}

func reconcilePublishedApplicationCatalog(
	ctx context.Context,
	applications publishedCanvasApplicationLister,
	projector *workflowcanvassvc.ApplicationCatalogProjector,
) error {
	versions, err := applications.ListPublishedCanvasApplicationVersions(ctx)
	if err != nil {
		return err
	}
	for _, item := range versions {
		if item == nil || item.Application == nil || item.Version == nil {
			continue
		}
		err := projector.ProjectPublication(ctx, workflowcanvassvc.ApplicationVersionPublication{
			ApplicationID:                item.Application.ID,
			ApplicationVersionID:         item.Version.ID,
			ApplicationTemplateVersionID: item.Version.ApplicationTemplateVersionID,
			SemanticVersion:              item.Version.SemanticVersion,
			ApplicationName:              item.Application.Name,
			OwnerUserID:                  item.Application.OwnerUserID,
			Visibility:                   item.Application.Visibility,
			CanvasEnabled:                item.Application.CanvasEnabled,
			RunEnabled:                   item.Application.RunEnabled,
			InputSchema:                  item.Version.InputSchema,
			OutputSchema:                 item.Version.OutputSchema,
		})
		var diagnostic *workflowcanvassvc.ApplicationCatalogDiagnosticError
		if stderrors.As(err, &diagnostic) {
			log.Warnf(
				"application version omitted from canvas catalog: application_version_id=%s error=%v",
				item.Version.ID,
				err,
			)
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func startCanvasApplicationConsumers(
	ctx context.Context,
	catalog *workflowcanvassvc.ApplicationCatalogProjector,
	artifacts reliablePayloadProjector,
) error {
	catalogMessages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicApplicationVersionPublished,
		applicationCatalogConsumerGroup,
	)
	if err != nil {
		return err
	}
	go consumeApplicationCatalog(ctx, catalogMessages, catalog)
	artifactMessages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicApplicationRunArtifactRefChanged,
		applicationArtifactProjectionConsumerGroup,
	)
	if err != nil {
		return err
	}
	go consumeReliablePayloads(
		ctx,
		artifactMessages,
		artifacts,
		applicationArtifactProjectionConsumerGroup,
	)
	return nil
}

func consumeApplicationCatalog(
	ctx context.Context,
	messages <-chan *message.Message,
	projector *workflowcanvassvc.ApplicationCatalogProjector,
) {
	for msg := range messages {
		err := projector.Project(ctx, msg.Payload)
		var diagnostic *workflowcanvassvc.ApplicationCatalogDiagnosticError
		if stderrors.As(err, &diagnostic) {
			log.Warnf(
				"application version omitted from canvas catalog: consumer_group=%s message_id=%s error=%v",
				applicationCatalogConsumerGroup,
				msg.UUID,
				err,
			)
			msg.Ack()
			continue
		}
		if err != nil {
			log.Errorf(
				"application catalog projection failed: consumer_group=%s message_id=%s error=%v",
				applicationCatalogConsumerGroup,
				msg.UUID,
				err,
			)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

func consumeReliablePayloads(
	ctx context.Context,
	messages <-chan *message.Message,
	projector reliablePayloadProjector,
	consumerGroup string,
) {
	for msg := range messages {
		if err := projector.Project(ctx, msg.Payload); err != nil {
			log.Errorf(
				"reliable projection failed: consumer_group=%s message_id=%s error=%v",
				consumerGroup,
				msg.UUID,
				err,
			)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

// startApplicationRunTerminalProjectionConsumer 使用 Task Center outbox 的持久 offset 重试 ApplicationRun 终态投影。
func startApplicationRunTerminalProjectionConsumer(
	ctx context.Context,
	tasks store.TaskCenterStore,
	projector applicationRunTerminalProjector,
) error {
	messages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicAtomicTaskStatusChanged,
		applicationRunTerminalProjectionConsumerGroup,
	)
	if err != nil {
		return err
	}
	go consumeApplicationRunTerminalProjections(ctx, messages, tasks, projector)
	return nil
}

func consumeApplicationRunTerminalProjections(
	ctx context.Context,
	messages <-chan *message.Message,
	tasks store.TaskCenterStore,
	projector applicationRunTerminalProjector,
) {
	for msg := range messages {
		if err := handleApplicationRunTerminalProjection(ctx, tasks, projector, msg.Payload); err != nil {
			log.Errorf(
				"application run terminal projection failed: consumer_group=%s message_id=%s error=%v",
				applicationRunTerminalProjectionConsumerGroup,
				msg.UUID,
				err,
			)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

func handleApplicationRunTerminalProjection(
	ctx context.Context,
	tasks store.TaskCenterStore,
	projector applicationRunTerminalProjector,
	payload []byte,
) error {
	var event struct {
		AtomicTaskID     string `json:"atomic_task_id"`
		ApplicationRunID string `json:"application_run_id"`
		Status           string `json:"status"`
		ToStatus         string `json:"to_status"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "decode atomic task status event")
	}
	if event.AtomicTaskID == "" {
		return errors.Errorf("atomic task status event is incomplete")
	}
	status := event.ToStatus
	if status == "" {
		status = event.Status
	}
	if event.ApplicationRunID == "" || !iapiserver.IsAtomicTaskTerminal(status) {
		return nil
	}
	task, err := tasks.GetAtomicTask(ctx, event.AtomicTaskID)
	if err != nil {
		return errors.Wrap(err, "load terminal application run atomic task")
	}
	if task == nil {
		return errors.Errorf("terminal application run atomic task is missing")
	}
	if task.ApplicationRunID == "" || !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil
	}
	return errors.Wrap(projector.Completed(ctx, task), "project terminal application run")
}

func startAssetLibraryTaskConsumers(ctx context.Context, tasks taskcentersvc.TaskCenterSrv, projector *appsvc.ApplicationArtifactProjector) error {
	for _, topic := range []string{
		postgresql.OutboxTopicArtifactCreated,
		postgresql.OutboxTopicArtifactProcessingChanged,
		postgresql.OutboxTopicArtifactRegistrationChanged,
	} {
		messages, err := postgresql.SubscribeOutbox(ctx, topic, "application-platform-artifact-projection")
		if err != nil {
			return err
		}
		go func(topic string, messages <-chan *message.Message) {
			for msg := range messages {
				if err := projector.Project(ctx, msg.Payload); err != nil {
					log.Errorf("application artifact projection failed: topic=%s message_id=%s error=%v", topic, msg.UUID, err)
					msg.Nack()
				} else {
					msg.Ack()
				}
			}
		}(topic, messages)
	}
	artifactMessages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicArtifactContentCompleted, "task-center-artifact-process")
	if err != nil {
		return err
	}
	go func() {
		for msg := range artifactMessages {
			var event struct {
				ArtifactID               string `json:"artifact_id"`
				OwnerUserID              string `json:"owner_user_id"`
				ProcessingProfileVersion string `json:"processing_profile_version"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err != nil || event.ArtifactID == "" || event.OwnerUserID == "" {
				msg.Nack()
				continue
			}
			_, err := tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{
				Key: "artifact-process", Name: "Process uploaded Artifact", FunctionRef: assetlibrarysvc.FunctionArtifactProcess, SystemName: iapiserver.SystemNameSpec{Key: taskname.ArtifactProcess},
				Arguments:            map[string]any{"artifact_id": event.ArtifactID, "owner_user_id": event.OwnerUserID},
				RequiredCapabilities: assetlibrarysvc.FunctionArtifactProcess,
				ProjectID:            iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: event.OwnerUserID,
				IdempotencyScope: "artifact-process", IdempotencyKey: "artifact-process:" + event.ArtifactID + ":" + event.ProcessingProfileVersion,
			})
			if err != nil {
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()

	representationMessages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicAssetVersionRepresentationRequested, representationOrchestratorConsumerGroup)
	if err != nil {
		return err
	}
	go func() {
		for msg := range representationMessages {
			if err := handleRepresentationRequested(ctx, tasks, msg.Payload); err != nil {
				log.Errorf("asset representation orchestration failed: consumer_group=%s message_id=%s error=%v", representationOrchestratorConsumerGroup, msg.UUID, err)
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()
	return nil
}

func handleRepresentationRequested(ctx context.Context, tasks representationTaskCreator, payload []byte) error {
	request, err := representationDAGRequest(payload)
	if err != nil {
		return err
	}
	_, err = tasks.CreateDAGTaskGroup(ctx, request)
	return errors.Wrap(err, "create representation DAG task group")
}

func representationDAGRequest(payload []byte) (*iapiserver.DAGTaskGroupCreateRequest, error) {
	var event representationRequestedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, errors.Wrap(err, "decode representation requested event")
	}
	if event.AssetID == "" || event.AssetVersionID == "" || event.OwnerUserID == "" || event.ProjectID == "" || event.Namespace == "" || event.MediaType == "" || event.ProfileVersion == "" || event.IdempotencyKey == "" || event.RequestedRepresentations == nil {
		return nil, errors.Errorf("representation requested event is incomplete")
	}
	nodes := []iapiserver.DAGNode{
		{Key: "inspect", Task: iapiserver.AtomicTaskTemplate{Key: "inspect", Name: "Inspect AssetVersion representations", SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationInspect}, FunctionRef: assetlibrarysvc.FunctionRepresentationInspect, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationInspect, Arguments: map[string]any{"asset_id": event.AssetID, "asset_version_id": event.AssetVersionID, "owner_user_id": event.OwnerUserID, "media_type": event.MediaType, "profile_version": event.ProfileVersion}}},
	}
	for _, requested := range event.RequestedRepresentations {
		if requested.RepresentationType == "" || requested.Profile == "" {
			return nil, errors.Errorf("representation requested event contains an invalid representation")
		}
		childKey := requested.RepresentationType + ":" + requested.Profile
		nodes = append(nodes, iapiserver.DAGNode{Key: childKey, Task: iapiserver.AtomicTaskTemplate{Key: childKey, Name: "Generate " + requested.RepresentationType, SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationGenerate, Params: map[string]string{"representation_type": requested.RepresentationType}}, FunctionRef: assetlibrarysvc.FunctionRepresentationGenerate, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationGenerate, Arguments: map[string]any{"asset_id": event.AssetID, "asset_version_id": event.AssetVersionID, "owner_user_id": event.OwnerUserID, "media_type": event.MediaType, "representation_type": requested.RepresentationType, "profile": requested.Profile, "profile_version": event.ProfileVersion, "required": requested.Required, "max_attempts": 3}, RetryPolicy: iapiserver.RetryPolicy{MaxAttempts: 3, RetryDelaySeconds: 5, BackoffType: "EXPONENTIAL_BACKOFF", MaxRetryDelaySeconds: 30}}})
	}
	nodes = append(nodes, iapiserver.DAGNode{Key: "finalize", Task: iapiserver.AtomicTaskTemplate{Key: "finalize", Name: "Finalize AssetVersion representations", SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationFinalize}, FunctionRef: assetlibrarysvc.FunctionRepresentationFinalize, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationFinalize, Arguments: map[string]any{"asset_version_id": event.AssetVersionID, "owner_user_id": event.OwnerUserID}}})
	edges := make([]iapiserver.DAGEdge, 0, max(1, 2*len(event.RequestedRepresentations)))
	for _, node := range nodes[1 : len(nodes)-1] {
		edges = append(edges, iapiserver.DAGEdge{FromNode: "inspect", ToNode: node.Key}, iapiserver.DAGEdge{FromNode: node.Key, ToNode: "finalize"})
	}
	if len(nodes) == 2 {
		edges = append(edges, iapiserver.DAGEdge{FromNode: "inspect", ToNode: "finalize"})
	}
	return &iapiserver.DAGTaskGroupCreateRequest{
		Name: "Build AssetVersion representations", Nodes: nodes, Edges: edges, SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationBuild},
		Input: map[string]any{"asset_version_id": event.AssetVersionID}, ProjectID: event.ProjectID,
		Namespace: event.Namespace, CreatedBy: event.OwnerUserID, IdempotencyScope: "asset-representations", IdempotencyKey: event.IdempotencyKey,
		TriggerType: iapiserver.DAGTriggerDomainEvent, TriggerSourceID: event.AssetVersionID, TriggerSourceName: "asset_version_representation_requested",
	}, nil
}

type workerArtifactLifecycle struct {
	store   store.AssetV1Store
	storage assetlibrarysvc.ContentStorage
	policy  assetlibrarysvc.RepresentationPolicy
}

func (l *workerArtifactLifecycle) Prepare(ctx context.Context, artifact *iapiserver.Artifact) (*iapiserver.Artifact, bool, error) {
	return l.store.CreateArtifact(ctx, artifact)
}

func (l *workerArtifactLifecycle) StoreContent(ctx context.Context, artifact *iapiserver.Artifact, mimeType string, reader io.Reader) (*iapiserver.Artifact, error) {
	content, err := l.storage.WriteArtifact(ctx, artifact.ID, mimeType, reader)
	if err != nil {
		return nil, err
	}
	current, err := l.store.StoreArtifactContent(ctx, artifact.OwnerUserID, artifact.ID, content)
	if err != nil {
		return nil, err
	}
	current, err = l.store.CompleteArtifact(ctx, artifact.OwnerUserID, artifact.ID, &iapiserver.CompleteArtifactRequest{
		SHA256: content.SHA256, SizeBytes: content.SizeBytes, MIMEType: content.MIMEType,
		ProcessingProfileVersion: artifact.ProcessingProfileVersion, Metadata: map[string]any{},
	})
	if err != nil {
		return nil, err
	}
	readyAt := imachinery.Now()
	return l.store.UpdateArtifactProcessing(ctx, current.ID, current.OwnerUserID, current.ResourceVersion, store.ArtifactProcessingMutation{
		ChangeType: "ready", ProcessingStatus: iapiserver.ArtifactProcessingReady, ReadyAt: &readyAt,
	})
}

func (l *workerArtifactLifecycle) Register(ctx context.Context, artifact *iapiserver.Artifact, existingAssetID string) (*iapiserver.Artifact, error) {
	request := &iapiserver.RegisterArtifactRequest{Mode: "create_asset", Name: artifact.OutputKey, ProfileVersion: artifact.ProcessingProfileVersion}
	if existingAssetID != "" {
		request.Mode, request.AssetID = "append_version", existingAssetID
	}
	plan := l.policy.Plan(artifact.MediaType, artifact.ProcessingProfileVersion)
	if _, err := l.store.RegisterArtifactLifecycleWithPlan(ctx, artifact.OwnerUserID, artifact.ID, request, plan); err != nil {
		return nil, err
	}
	return l.store.GetArtifact(ctx, artifact.OwnerUserID, artifact.ID)
}

func (l *workerArtifactLifecycle) FailProcessing(ctx context.Context, artifact *iapiserver.Artifact, errorCode, detail string) (*iapiserver.Artifact, error) {
	current, err := l.store.GetArtifact(ctx, artifact.OwnerUserID, artifact.ID)
	if err != nil {
		return nil, err
	}
	if current.ProcessingStatus == iapiserver.ArtifactProcessingReady {
		return current, nil
	}
	return l.store.UpdateArtifactProcessing(ctx, current.ID, current.OwnerUserID, current.ResourceVersion, store.ArtifactProcessingMutation{
		ChangeType: "failed", ProcessingStatus: iapiserver.ArtifactProcessingFailed,
		ProcessingErrorCode: errorCode, ProcessingErrorDetail: detail, Retryable: true,
	})
}

func (l *workerArtifactLifecycle) FailRegistration(ctx context.Context, artifact *iapiserver.Artifact, errorCode, detail string) (*iapiserver.Artifact, error) {
	current, err := l.store.GetArtifact(ctx, artifact.OwnerUserID, artifact.ID)
	if err != nil {
		return nil, err
	}
	if current.RegistrationStatus == iapiserver.ArtifactRegistrationRegistered {
		return current, nil
	}
	return l.store.UpdateArtifactRegistration(ctx, current.ID, current.OwnerUserID, current.ResourceVersion, store.ArtifactRegistrationMutation{
		RegistrationStatus:    iapiserver.ArtifactRegistrationFailed,
		RegistrationErrorCode: errorCode, RegistrationErrorDetail: detail, Retryable: true,
	})
}

func createScheduleTarget(ctx context.Context, tasks taskcentersvc.TaskCenterSrv, schedule *iapiserver.TaskSchedule, triggeredAt imachinery.Time) (string, error) {
	raw, err := json.Marshal(schedule.Target.Template)
	if err != nil {
		return "", err
	}
	switch schedule.Target.Type {
	case iapiserver.TaskScheduleTargetAtomic:
		var req iapiserver.AtomicTaskCreateRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return "", err
		}
		applyAtomicScheduleOwnership(&req, schedule)
		created, err := tasks.CreateAtomicTask(ctx, &req)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	case iapiserver.TaskScheduleTargetGroup:
		var req iapiserver.TaskGroupCreateRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return "", err
		}
		applyGroupScheduleOwnership(&req, schedule)
		created, err := tasks.CreateTaskGroup(ctx, &req)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	case iapiserver.TaskScheduleTargetDAG:
		var req iapiserver.DAGTaskGroupCreateRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return "", err
		}
		applyDAGScheduleOwnership(&req, schedule, triggeredAt)
		created, err := tasks.CreateDAGTaskGroup(ctx, &req)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	default:
		return "", fmt.Errorf("unsupported schedule target %s", schedule.Target.Type)
	}
}

func applyAtomicScheduleOwnership(req *iapiserver.AtomicTaskCreateRequest, schedule *iapiserver.TaskSchedule) {
	req.ProjectID = schedule.ProjectID
	req.Namespace = schedule.Namespace
	req.CreatedBy = schedule.CreatedBy
	req.OwnerType = iapiserver.TaskOwnerTypeSchedule
	req.OwnerID = schedule.ID
}

func applyGroupScheduleOwnership(req *iapiserver.TaskGroupCreateRequest, schedule *iapiserver.TaskSchedule) {
	req.ProjectID = schedule.ProjectID
	req.Namespace = schedule.Namespace
	req.CreatedBy = schedule.CreatedBy
}

func applyDAGScheduleOwnership(req *iapiserver.DAGTaskGroupCreateRequest, schedule *iapiserver.TaskSchedule, triggerTimes ...imachinery.Time) {
	req.ProjectID = schedule.ProjectID
	req.Namespace = schedule.Namespace
	req.CreatedBy = schedule.CreatedBy
	req.TriggerType = iapiserver.DAGTriggerSchedule
	req.TriggerSourceID = schedule.ID
	req.TriggerSourceName = schedule.Name
	if len(triggerTimes) > 0 {
		req.TriggeredAt = triggerTimes[0]
	}
}

func ensureEngineHealthSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv, interval time.Duration) error {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	cron := healthCron(interval)
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: "application-platform.engine-health", Description: "Periodic EngineInstance health reconcile"}, TaskNameMeta: iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.EngineHealthReconcile}, SystemKey: engine.EngineHealthReconcileRef, CronExpression: cron, TimeZone: "UTC", ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: engine.EngineHealthReconcileRef, Config: map[string]any{}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 4, OverallTimeoutSeconds: 5}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func ensureComfyUIObjectInfoSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: "application-platform.comfyui-object-info-refresh", Description: "Daily ComfyUI object_info refresh"}, TaskNameMeta: iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.ObjectInfoRefresh}, SystemKey: comfyuiadapter.ComfyUIObjectInfoReconcileRef, CronExpression: "0 0 3 * * *", TimeZone: "UTC", ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: comfyuiadapter.ComfyUIObjectInfoReconcileRef, Config: map[string]any{}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 300}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func ensureRepresentationBackfillSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: assetlibrarysvc.RepresentationBackfillRef, Description: "Daily AssetVersion representation backfill"}, TaskNameMeta: iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.RepresentationBackfill}, SystemKey: assetlibrarysvc.RepresentationBackfillRef, CronExpression: "0 30 3 * * *", TimeZone: "UTC", ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: assetlibrarysvc.RepresentationBackfillRef, Config: map[string]any{"max_actions_per_run": 100}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 300}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func scheduleTime(value any) time.Time {
	var milliseconds int64
	switch typed := value.(type) {
	case float64:
		milliseconds = int64(typed)
	case int64:
		milliseconds = typed
	case json.Number:
		milliseconds, _ = typed.Int64()
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, typed); err == nil {
			return parsed.UTC()
		}
		_, _ = fmt.Sscan(typed, &milliseconds)
	}
	if milliseconds > 0 {
		return time.UnixMilli(milliseconds).UTC()
	}
	return time.Now().UTC()
}

func workerMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func healthCron(interval time.Duration) string {
	seconds := int(interval / time.Second)
	if seconds > 0 && seconds < 60 {
		return fmt.Sprintf("*/%d * * * * *", seconds)
	}
	minutes := int(interval / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 59 {
		minutes = 59
	}
	return fmt.Sprintf("0 */%d * * * *", minutes)
}
