package apiserver

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/timeutil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

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
	"github.com/wangweihong/omnimam/backend/internal/taskworker/agentexecutor"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/appstudioexecutor"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/comfyuiexecutor"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/contracts"
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
		appplatformsvc.FunctionAssetThumbnailGenerate, iapiserver.TaskWorkerFunctionApplicationRun, iapiserver.TaskWorkerFunctionScheduleAcquire,
		iapiserver.TaskWorkerFunctionComfyUISubmit, iapiserver.TaskWorkerFunctionComfyUIPoll, iapiserver.TaskWorkerFunctionComfyUICollectPreview,
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
		return errors.Wrap(err, "register engine health reconcile handler")
	}
	if err := reconcileRegistry.Register(comfyuiadapter.NewComfyUIObjectInfoReconcileHandler(storeIns, applicationService)); err != nil {
		return errors.Wrap(err, "register ComfyUI object info reconcile handler")
	}
	if err := reconcileRegistry.Register(assetlibrarysvc.NewRepresentationBackfillHandler(storeIns, representationPolicy)); err != nil {
		return errors.Wrap(err, "register representation backfill handler")
	}
	comfyTestExecutor := comfyuiadapter.NewTestExecutor(storeIns)
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAgentRuntimeEnsure, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return agentexecutor.ExecuteRuntimeEnsure(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAgentRuntimeStop, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return agentexecutor.ExecuteRuntimeStop(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAppStudioPreviewEnsure, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return appstudioexecutor.ExecutePreviewEnsure(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAppStudioPreviewStop, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return appstudioexecutor.ExecutePreviewStop(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAppStudioBuildExecute, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return appstudioexecutor.ExecuteBuild(ctx, infrastructureClient, artifactLifecycle, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAppStudioProductionReconcile, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return appstudioexecutor.ExecuteProductionReconcile(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAppStudioProductionStop, 8, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		return appstudioexecutor.ExecuteProductionStop(ctx, infrastructureClient, functionRegistry, task, atomicTask)
	}); err != nil {
		return err
	}
	if err := comfyuiexecutor.RegisterHandlers(runtime, comfyTestExecutor); err != nil {
		return errors.Wrap(err, "register comfyui handlers")
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionApplicationRun, 16, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		if atomicTask.CanvasRunID != "" {
			run, ensureErr := applicationService.EnsureCanvasApplicationRun(ctx, &appsvc.CanvasApplicationRunRequest{
				AtomicTaskID:         atomicTask.ID,
				CanvasRunID:          atomicTask.CanvasRunID,
				CanvasNodeRunID:      atomicTask.CanvasNodeRunID,
				ExecutionKey:         atomicTask.ChildKey,
				ApplicationVersionID: fmt.Sprint(task.Arguments[iapiserver.TaskWorkerKeyApplicationVersionID]),
				OwnerUserID:          atomicTask.CreatedBy,
				Inputs:               typeutil.AnyToMapAny(task.Arguments[iapiserver.TaskWorkerKeyResolvedInputs]),
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
			if inProgress, _ := output[iapiserver.TaskWorkerKeyInProgress].(bool); inProgress {
				task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogApplicationExecutionWaiting, workflowruntime.TaskLogLevelInfo, "External application job is waiting for the next callback."))
			} else {
				task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogApplicationExecutionCompleted, workflowruntime.TaskLogLevelInfo, "Application provider execution returned a result."))
			}
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), appplatformsvc.FunctionAssetThumbnailGenerate, 16, func(ctx context.Context, task workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask) (map[string]any, error) {
		output, err := thumbnailExecutor.Execute(ctx, atomicTask)
		if err == nil {
			message := "Thumbnail processing completed."
			if output[iapiserver.TaskWorkerKeyThumbnailStatus] == iapiserver.ThumbnailStatusUnsupported {
				message = "Thumbnail generation is unsupported for this asset."
			}
			task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogAssetThumbnailCompleted, workflowruntime.TaskLogLevelInfo, message))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, assetlibrarysvc.FunctionArtifactProcess, 16, artifactProcessExecutor.Execute); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, assetlibrarysvc.FunctionRepresentationInspect, 16, representationInspectExecutor.Execute); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, assetlibrarysvc.FunctionRepresentationGenerate, 8, representationGenerateExecutor.Execute); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, assetlibrarysvc.FunctionRepresentationFinalize, 16, representationFinalizeExecutor.Execute); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, taskcentersvc.ReconcileControllerTask, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		scheduleID, _ := task.Arguments[iapiserver.TaskWorkerKeyTaskScheduleID].(string)
		output, err := tasks.RunScheduleReconcile(ctx, scheduleID, task.WorkflowID, timeutil.ScheduleTime(task.Arguments[iapiserver.TaskWorkerKeyScheduledAt]))
		if err == nil {
			message := "Reconcile cycle completed."
			if summary, ok := output[iapiserver.TaskWorkerKeyReconcileSummary].(iapiserver.ReconcileSummary); ok {
				message = fmt.Sprintf("Reconcile cycle completed with %d scanned, %d findings, %d actions, and %d deferred.", summary.Scanned, summary.Findings, summary.ActionsCreated, summary.Deferred)
			}
			task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogScheduleReconcileCompleted, workflowruntime.TaskLogLevelInfo, message))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, taskcentersvc.ManualScheduleControllerTask, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		executionID, _ := task.Arguments[iapiserver.TaskWorkerKeyScheduleExecutionID].(string)
		return tasks.RunManualScheduleExecution(ctx, executionID, task.WorkflowID)
	}); err != nil {
		return err
	}
	if err := registerWorkerHandler(runtime, iapiserver.TaskWorkerFunctionScheduleAcquire, 1, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogScheduleAcquireStarted, workflowruntime.TaskLogLevelInfo, "Evaluating scheduled execution ownership."))
		scheduleID, _ := task.Arguments[iapiserver.TaskWorkerKeyTaskScheduleID].(string)
		schedule, err := storeIns.TaskCenters().GetTaskSchedule(ctx, scheduleID)
		if err != nil {
			return nil, err
		}
		scheduledAt := timeutil.ScheduleTime(task.Arguments[iapiserver.TaskWorkerKeyScheduledAt])
		if taskcentersvc.ScheduleTriggerMisfired(scheduledAt, time.Now()) {
			existing, getErr := storeIns.TaskCenters().GetScheduleExecutionAt(ctx, schedule.ID, scheduledAt)
			if getErr != nil {
				return nil, getErr
			}
			if existing == nil {
				task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogScheduleAcquireMisfire, workflowruntime.TaskLogLevelWarn, "Delayed schedule trigger was skipped by misfire policy."))
				return map[string]any{
					iapiserver.TaskWorkerKeyStatus:      iapiserver.TaskSchedulePolicySkip,
					iapiserver.TaskWorkerKeyScheduledAt: scheduledAt.UTC().Format(time.RFC3339Nano),
					iapiserver.TaskWorkerKeyReason:      iapiserver.TaskWorkerScheduleReasonMisfire,
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
			task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogScheduleAcquireOverlap, workflowruntime.TaskLogLevelWarn, "Schedule trigger reused an existing execution record."))
			return map[string]any{iapiserver.TaskWorkerKeyScheduleExecutionID: record.ID, iapiserver.TaskWorkerKeyStatus: record.Status}, nil
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
			task.Log(ctx, workflowruntime.WorkerLog(iapiserver.TaskWorkerLogScheduleTargetCreated, workflowruntime.TaskLogLevelInfo, "Scheduled target was created and started."))
		}
		return map[string]any{iapiserver.TaskWorkerKeyScheduleExecutionID: record.ID, iapiserver.TaskWorkerKeyTargetID: targetID, iapiserver.TaskWorkerKeyStatus: record.Status}, err
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
	messages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicAssetUploaded, iapiserver.TaskWorkerConsumerGroupThumbnail)
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
				_, err = tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{Key: iapiserver.TaskWorkerTaskKeyThumbnail, Name: "Generate asset thumbnail", FunctionRef: appplatformsvc.FunctionAssetThumbnailGenerate, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetID: event.AssetID, iapiserver.TaskWorkerKeyThumbnailID: thumbnail.ID}, RequiredCapabilities: appplatformsvc.CapabilityAssetThumbnail, ProjectID: event.ProjectID, Namespace: event.Namespace, IdempotencyScope: iapiserver.TaskWorkerIdempotencyScopeThumbnail, IdempotencyKey: iapiserver.TaskWorkerIdempotencyPrefixThumbnail + event.AssetID + ":" + event.ProfileVersion, SystemName: iapiserver.SystemNameSpec{Key: taskname.AssetThumbnail}})
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

type infrastructureCommandExecutor = contracts.InfrastructureCommandExecutor

type infrastructureBuildExecutor = contracts.InfrastructureBuildExecutor

type appStudioBuildArtifactLifecycle = appstudioexecutor.BuildArtifactLifecycle

func executeAgentRuntimeEnsure(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return agentexecutor.ExecuteRuntimeEnsure(ctx, client, registry, workerTask, atomicTask)
}

func executeAgentRuntimeStop(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return agentexecutor.ExecuteRuntimeStop(ctx, client, registry, workerTask, atomicTask)
}

func executeAppStudioPreviewEnsure(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return appstudioexecutor.ExecutePreviewEnsure(ctx, client, registry, workerTask, atomicTask)
}

func executeAppStudioPreviewStop(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return appstudioexecutor.ExecutePreviewStop(ctx, client, registry, workerTask, atomicTask)
}

func executeAppStudioBuild(
	ctx context.Context,
	client infrastructureBuildExecutor,
	lifecycle appStudioBuildArtifactLifecycle,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return appstudioexecutor.ExecuteBuild(ctx, client, lifecycle, registry, workerTask, atomicTask)
}

func executeAppStudioProductionReconcile(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return appstudioexecutor.ExecuteProductionReconcile(ctx, client, registry, workerTask, atomicTask)
}

func executeAppStudioProductionStop(
	ctx context.Context,
	client infrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	return appstudioexecutor.ExecuteProductionStop(ctx, client, registry, workerTask, atomicTask)
}

func registerWorkerHandler(
	runtime workflowruntime.WorkerRegistrar,
	functionRef string,
	concurrency int,
	handler workflowruntime.Handler,
) error {
	if runtime == nil {
		return fmt.Errorf("workflow runtime is required")
	}
	if err := runtime.RegisterHandler(functionRef, concurrency, handler); err != nil {
		return errors.Wrap(err, "register "+functionRef+" handler")
	}
	return nil
}

func registerAtomicTaskHandler(
	runtime workflowruntime.WorkerRegistrar,
	tasks store.TaskCenterStore,
	functionRef string,
	concurrency int,
	executor func(context.Context, workflowruntime.WorkerTask, *iapiserver.AtomicTask) (map[string]any, error),
) error {
	if tasks == nil {
		return fmt.Errorf("task center store is required")
	}
	if executor == nil {
		return fmt.Errorf("atomic task executor is required")
	}
	return registerWorkerHandler(runtime, functionRef, concurrency, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, err := tasks.GetAtomicTask(ctx, task.AtomicTaskID)
		if err != nil {
			return nil, errors.Wrap(err, "load "+functionRef+" atomic task")
		}
		if atomicTask == nil {
			return nil, fmt.Errorf("atomic task %q is missing", task.AtomicTaskID)
		}
		return executor(ctx, task, atomicTask)
	})
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
		iapiserver.TaskWorkerConsumerGroupApplicationCatalog,
	)
	if err != nil {
		return err
	}
	go consumeApplicationCatalog(ctx, catalogMessages, catalog)
	artifactMessages, err := postgresql.SubscribeOutbox(
		ctx,
		postgresql.OutboxTopicApplicationRunArtifactRefChanged,
		iapiserver.TaskWorkerConsumerGroupApplicationArtifactProjection,
	)
	if err != nil {
		return err
	}
	go consumeReliablePayloads(
		ctx,
		artifactMessages,
		artifacts,
		iapiserver.TaskWorkerConsumerGroupApplicationArtifactProjection,
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
				iapiserver.TaskWorkerConsumerGroupApplicationCatalog,
				msg.UUID,
				err,
			)
			msg.Ack()
			continue
		}
		if err != nil {
			log.Errorf(
				"application catalog projection failed: consumer_group=%s message_id=%s error=%v",
				iapiserver.TaskWorkerConsumerGroupApplicationCatalog,
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
		iapiserver.TaskWorkerConsumerGroupApplicationRunTerminal,
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
				iapiserver.TaskWorkerConsumerGroupApplicationRunTerminal,
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
		messages, err := postgresql.SubscribeOutbox(ctx, topic, iapiserver.TaskWorkerConsumerGroupArtifactProjection)
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
	artifactMessages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicArtifactContentCompleted, iapiserver.TaskWorkerConsumerGroupArtifactProcess)
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
				Key: iapiserver.TaskWorkerTaskKeyArtifactProcess, Name: "Process uploaded Artifact", FunctionRef: assetlibrarysvc.FunctionArtifactProcess, SystemName: iapiserver.SystemNameSpec{Key: taskname.ArtifactProcess},
				Arguments:            map[string]any{iapiserver.TaskWorkerKeyArtifactID: event.ArtifactID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID},
				RequiredCapabilities: assetlibrarysvc.FunctionArtifactProcess,
				ProjectID:            iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: event.OwnerUserID,
				IdempotencyScope: iapiserver.TaskWorkerIdempotencyScopeArtifactProcess, IdempotencyKey: iapiserver.TaskWorkerIdempotencyScopeArtifactProcess + ":" + event.ArtifactID + ":" + event.ProcessingProfileVersion,
			})
			if err != nil {
				msg.Nack()
			} else {
				msg.Ack()
			}
		}
	}()

	representationMessages, err := postgresql.SubscribeOutbox(ctx, postgresql.OutboxTopicAssetVersionRepresentationRequested, iapiserver.TaskWorkerConsumerGroupRepresentationOrchestrator)
	if err != nil {
		return err
	}
	go func() {
		for msg := range representationMessages {
			if err := handleRepresentationRequested(ctx, tasks, msg.Payload); err != nil {
				log.Errorf("asset representation orchestration failed: consumer_group=%s message_id=%s error=%v", iapiserver.TaskWorkerConsumerGroupRepresentationOrchestrator, msg.UUID, err)
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
		{Key: iapiserver.TaskWorkerTaskKeyRepresentationInspect, Task: iapiserver.AtomicTaskTemplate{Key: iapiserver.TaskWorkerTaskKeyRepresentationInspect, Name: "Inspect AssetVersion representations", SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationInspect}, FunctionRef: assetlibrarysvc.FunctionRepresentationInspect, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationInspect, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetID: event.AssetID, iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID, iapiserver.TaskWorkerKeyMediaType: event.MediaType, iapiserver.TaskWorkerKeyProfileVersion: event.ProfileVersion}}},
	}
	for _, requested := range event.RequestedRepresentations {
		if requested.RepresentationType == "" || requested.Profile == "" {
			return nil, errors.Errorf("representation requested event contains an invalid representation")
		}
		childKey := requested.RepresentationType + iapiserver.TaskWorkerCompositeKeySeparator + requested.Profile
		nodes = append(nodes, iapiserver.DAGNode{Key: childKey, Task: iapiserver.AtomicTaskTemplate{Key: childKey, Name: "Generate " + requested.RepresentationType, SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationGenerate, Params: map[string]string{iapiserver.TaskWorkerKeyRepresentationType: requested.RepresentationType}}, FunctionRef: assetlibrarysvc.FunctionRepresentationGenerate, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationGenerate, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetID: event.AssetID, iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID, iapiserver.TaskWorkerKeyMediaType: event.MediaType, iapiserver.TaskWorkerKeyRepresentationType: requested.RepresentationType, iapiserver.TaskWorkerKeyProfile: requested.Profile, iapiserver.TaskWorkerKeyProfileVersion: event.ProfileVersion, iapiserver.TaskWorkerKeyRequired: requested.Required, iapiserver.TaskWorkerKeyMaxAttempts: 3}, RetryPolicy: iapiserver.RetryPolicy{MaxAttempts: 3, RetryDelaySeconds: 5, BackoffType: iapiserver.TaskWorkerRetryBackoffExponential, MaxRetryDelaySeconds: 30}}})
	}
	nodes = append(nodes, iapiserver.DAGNode{Key: iapiserver.TaskWorkerTaskKeyRepresentationFinalize, Task: iapiserver.AtomicTaskTemplate{Key: iapiserver.TaskWorkerTaskKeyRepresentationFinalize, Name: "Finalize AssetVersion representations", SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationFinalize}, FunctionRef: assetlibrarysvc.FunctionRepresentationFinalize, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationFinalize, Arguments: map[string]any{iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID, iapiserver.TaskWorkerKeyOwnerUserID: event.OwnerUserID}}})
	edges := make([]iapiserver.DAGEdge, 0, max(1, 2*len(event.RequestedRepresentations)))
	for _, node := range nodes[1 : len(nodes)-1] {
		edges = append(edges, iapiserver.DAGEdge{FromNode: iapiserver.TaskWorkerTaskKeyRepresentationInspect, ToNode: node.Key}, iapiserver.DAGEdge{FromNode: node.Key, ToNode: iapiserver.TaskWorkerTaskKeyRepresentationFinalize})
	}
	if len(nodes) == 2 {
		edges = append(edges, iapiserver.DAGEdge{FromNode: iapiserver.TaskWorkerTaskKeyRepresentationInspect, ToNode: iapiserver.TaskWorkerTaskKeyRepresentationFinalize})
	}
	return &iapiserver.DAGTaskGroupCreateRequest{
		Name: "Build AssetVersion representations", Nodes: nodes, Edges: edges, SystemName: iapiserver.SystemNameSpec{Key: taskname.RepresentationBuild},
		Input: map[string]any{iapiserver.TaskWorkerKeyAssetVersionID: event.AssetVersionID}, ProjectID: event.ProjectID,
		Namespace: event.Namespace, CreatedBy: event.OwnerUserID, IdempotencyScope: iapiserver.TaskWorkerIdempotencyScopeRepresentations, IdempotencyKey: event.IdempotencyKey,
		TriggerType: iapiserver.DAGTriggerDomainEvent, TriggerSourceID: event.AssetVersionID, TriggerSourceName: iapiserver.TaskWorkerRepresentationTriggerSourceName,
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
		ChangeType: iapiserver.TaskWorkerArtifactProcessingChangeTypeReady, ProcessingStatus: iapiserver.ArtifactProcessingReady, ReadyAt: &readyAt,
	})
}

func (l *workerArtifactLifecycle) Register(ctx context.Context, artifact *iapiserver.Artifact, existingAssetID string) (*iapiserver.Artifact, error) {
	request := &iapiserver.RegisterArtifactRequest{Mode: iapiserver.TaskWorkerArtifactRegisterModeCreateAsset, Name: artifact.OutputKey, ProfileVersion: artifact.ProcessingProfileVersion}
	if existingAssetID != "" {
		request.Mode, request.AssetID = iapiserver.TaskWorkerArtifactRegisterModeAppendVersion, existingAssetID
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
		ChangeType: iapiserver.TaskWorkerArtifactProcessingChangeTypeFailed, ProcessingStatus: iapiserver.ArtifactProcessingFailed,
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
	cron := timeutil.DurtionToCron(interval)
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: iapiserver.TaskWorkerScheduleNameEngineHealth, Description: "Periodic EngineInstance health reconcile"}, TaskNameMeta: iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.EngineHealthReconcile}, SystemKey: engine.EngineHealthReconcileRef, CronExpression: cron, TimeZone: iapiserver.TaskWorkerTimeZoneUTC, ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: engine.EngineHealthReconcileRef, Config: map[string]any{}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 4, OverallTimeoutSeconds: 5}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func ensureComfyUIObjectInfoSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: iapiserver.TaskWorkerScheduleNameComfyUIObjectInfoRefresh, Description: "Daily ComfyUI object_info refresh"}, TaskNameMeta: iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.ObjectInfoRefresh}, SystemKey: comfyuiadapter.ComfyUIObjectInfoReconcileRef, CronExpression: iapiserver.TaskWorkerScheduleCronComfyUIObjectInfoRefresh, TimeZone: iapiserver.TaskWorkerTimeZoneUTC, ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: comfyuiadapter.ComfyUIObjectInfoReconcileRef, Config: map[string]any{}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 300}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func ensureRepresentationBackfillSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: assetlibrarysvc.RepresentationBackfillRef, Description: "Daily AssetVersion representation backfill"}, TaskNameMeta: iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.RepresentationBackfill}, SystemKey: assetlibrarysvc.RepresentationBackfillRef, CronExpression: iapiserver.TaskWorkerScheduleCronRepresentationBackfill, TimeZone: iapiserver.TaskWorkerTimeZoneUTC, ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: assetlibrarysvc.RepresentationBackfillRef, Config: map[string]any{iapiserver.TaskWorkerKeyMaxActionsPerRun: 100}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 300}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}
