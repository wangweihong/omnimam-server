package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/timeutil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	agentsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/agent"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	gitlabsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/gitlab"
	engine "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	modeladapters "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters"
	comfyuiadapter "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/comfyui"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	usermodelsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/usermodel"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentgrant"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/agentexecutor"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/appstudioexecutor"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/comfyuiexecutor"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/consumer"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/contracts"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/gitlabexecutor"
)

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
	tasks := taskcentersvc.NewServiceWithFunctionRegistry(storeIns, runtime, reconcileRegistry, functionRegistry, nil, nil,
		appplatformsvc.FunctionAssetThumbnailGenerate, iapiserver.TaskWorkerFunctionApplicationRun, iapiserver.TaskWorkerFunctionScheduleAcquire,
		iapiserver.TaskWorkerFunctionComfyUISubmit, iapiserver.TaskWorkerFunctionComfyUIPoll, iapiserver.TaskWorkerFunctionComfyUICollectPreview,
		assetlibrarysvc.FunctionArtifactProcess, assetlibrarysvc.FunctionRepresentationInspect,
		assetlibrarysvc.FunctionRepresentationGenerate, assetlibrarysvc.FunctionRepresentationFinalize, iapiserver.GitLabFunctionPipelineRun)
	gitLabClientFactory := gitlabsvc.NewHTTPClientFactory()
	gitLabExecutor, err := gitlabexecutor.New(storeIns.GitLab(), gitLabClientFactory)
	if err != nil {
		return errors.Wrap(err, "construct gitlab pipeline executor")
	}
	adapters := modeladapters.NewEngineAdapters()
	executors := modeladapters.NewOperationExecutors()
	if err := modeladapters.ValidateImplementations(runtimeRegistry, adapters, executors); err != nil {
		return errors.Wrap(err, "validate application platform adapter implementations")
	}
	credentialBroker := usermodelsvc.NewCredentialBroker(0)
	grantCodec, err := agentgrant.NewCodec(cfg.InfrastructureClientOptions.Token, time.Hour)
	if err != nil {
		return errors.Wrap(err, "construct agent grant codec")
	}
	userModelGateway, err := engine.NewUserModelGatewayService(engine.UserModelGatewayDependencies{
		Runtime: runtimeRegistry, Adapters: adapters, Executors: executors, Credentials: credentialBroker,
	})
	if err != nil {
		return errors.Wrap(err, "construct user model gateway")
	}
	userModelService, err := usermodelsvc.New(usermodelsvc.Dependencies{
		Store: storeIns, Gateway: userModelGateway, Credentials: credentialBroker, Grants: grantCodec,
	})
	if err != nil {
		return errors.Wrap(err, "construct user model service")
	}
	sourceProvider, err := gitlabsvc.NewSourceProvider(storeIns.GitLab(), gitLabClientFactory)
	if err != nil {
		return errors.Wrap(err, "construct appstudio gitlab source provider")
	}
	appStudioService, err := appstudiosvc.New(appstudiosvc.Dependencies{
		Store: storeIns.AppStudio(), Tasks: tasks, SourceProvider: sourceProvider, ProjectInitializer: sourceProvider, Artifacts: storeIns.AssetsV1(), Grants: grantCodec,
	})
	if err != nil {
		return errors.Wrap(err, "construct appstudio service for agent projector")
	}
	agentProjector, err := agentsvc.New(agentsvc.Dependencies{
		Store: storeIns.Agents(), Tasks: tasks, Workspaces: appStudioService, Models: userModelService, Grants: grantCodec,
	})
	if err != nil {
		return errors.Wrap(err, "construct agent runtime projector")
	}
	invocationExecutor, err := agentexecutor.NewInvocationExecutor(agentexecutor.InvocationExecutorDependencies{
		Store: storeIns.Agents(), Endpoints: infrastructureClient, Models: userModelService,
		Credentials: credentialBroker, Grants: grantCodec, Workspaces: appStudioService, Registry: functionRegistry,
	})
	if err != nil {
		return errors.Wrap(err, "construct agent invocation executor")
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
	if err := consumer.ReconcilePublishedApplicationCatalog(ctx, applicationService, applicationCatalogProjector); err != nil {
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
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.TaskWorkerFunctionAgentInvocationExecute, 8, invocationExecutor.Execute); err != nil {
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
	if err := registerAtomicTaskHandler(runtime, storeIns.TaskCenters(), iapiserver.GitLabFunctionPipelineRun, 8, gitLabExecutor.Execute); err != nil {
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
	if err := consumer.StartAssetLibrary(ctx, tasks, appsvc.NewApplicationArtifactProjector(storeIns.ApplicationPlatforms())); err != nil {
		return err
	}
	if err := consumer.StartApplicationRunTerminalProjection(ctx, storeIns.TaskCenters(), applicationExecutor); err != nil {
		return err
	}
	if err := consumer.StartCanvasApplications(
		ctx,
		applicationCatalogProjector,
		workflowcanvassvc.NewApplicationArtifactProjector(storeIns.TaskCenters(), storeIns.WorkflowCanvases()),
	); err != nil {
		return err
	}
	if err := consumer.StartThumbnail(ctx, tasks, storeIns.AssetThumbnails()); err != nil {
		return err
	}
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
	reconciler.RegisterTerminalRecoverySource(storeIns.Agents().ListPendingAgentTerminalTaskIDs)
	reconciler.RegisterTerminalRecoverySource(storeIns.AppStudio().ListPendingStudioTerminalTaskIDs)
	reconciler.RegisterRecoveryHandler(agentProjector.ReconcileQueuedInvocations)
	reconciler.RegisterRecoveryHandler(agentProjector.ReconcileInvocationActivity)
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
