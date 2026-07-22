package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

const representationOrchestratorConsumerGroup = "task-center-representation-orchestrator"

type representationTaskCreator interface {
	CreateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error)
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
	if cfg.WorkflowRuntimeOptions == nil || !cfg.WorkflowRuntimeOptions.Enabled {
		return fmt.Errorf("workflow runtime must be enabled for taskworker")
	}
	if err := InitializeStore(cfg); err != nil {
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
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		return errors.Wrap(err, "load application runtime registry")
	}
	capabilities, err := appregistry.LoadProviderCapabilityRegistry(cfg.ApplicationPlatformOptions.ProviderCapabilityDirectory, runtimeRegistry)
	if err != nil {
		return errors.Wrap(err, "load provider capabilities")
	}
	reconcileRegistry := taskcentersvc.NewReconcileRegistry()
	tasks := taskcentersvc.NewServiceWithRegistries(storeIns, runtime, reconcileRegistry,
		platformsvc.FunctionAssetThumbnailGenerate, "application-platform.run", "task.schedule.acquire",
		"comfyui.submit", "comfyui.poll", "comfyui.collect_preview",
		assetlibrarysvc.FunctionArtifactProcess, assetlibrarysvc.FunctionRepresentationInspect,
		assetlibrarysvc.FunctionRepresentationGenerate, assetlibrarysvc.FunctionRepresentationFinalize)
	adapters := appsvc.NewEngineAdapters()
	executors := appsvc.NewOperationExecutors()
	events := appsvc.NoopEventPublisher{}
	assetRegistrar := &workerAssetRegistrar{service: platformsvc.NewService(storeIns)}
	applicationExecutor, err := appsvc.NewApplicationRunExecutor(storeIns, runtimeRegistry, capabilities, adapters, executors, assetRegistrar, events)
	if err != nil {
		return err
	}
	thumbnailExecutor := platformsvc.NewThumbnailExecutor(storeIns)
	artifactProcessExecutor := assetlibrarysvc.NewArtifactProcessExecutor(storeIns)
	assetStorage := assetlibrarysvc.NewLocalContentStorage(storeIns)
	representationInspectExecutor := assetlibrarysvc.NewRepresentationInspectExecutor(storeIns)
	ffmpegRuntime, err := assetlibrarysvc.NewLocalFFmpegRuntime()
	if err != nil {
		return errors.Wrap(err, "construct ffmpeg runtime")
	}
	thumbnailGenerators := assetlibrarysvc.NewThumbnailGenerators(assetlibrarysvc.ImageThumbnailGenerator{}, assetlibrarysvc.NewVideoThumbnailGenerator(ffmpegRuntime))
	representationGenerateExecutor := assetlibrarysvc.NewRepresentationGenerateExecutor(storeIns, assetStorage, thumbnailGenerators)
	representationFinalizeExecutor := assetlibrarysvc.NewRepresentationFinalizeExecutor(storeIns)
	representationPolicy := assetlibrarysvc.DefaultRepresentationPolicy{}
	applicationService, err := appsvc.NewService(appsvc.Dependencies{Store: storeIns, Runtime: runtimeRegistry, Capabilities: capabilities, Adapters: adapters, Executors: executors, Tasks: tasks, Assets: assetRegistrar, Events: events})
	if err != nil {
		return err
	}
	if err := reconcileRegistry.Register(appsvc.NewEngineHealthReconcileHandler(storeIns, applicationService)); err != nil {
		return err
	}
	if err := reconcileRegistry.Register(appsvc.NewComfyUIObjectInfoReconcileHandler(storeIns, applicationService)); err != nil {
		return err
	}
	if err := reconcileRegistry.Register(assetlibrarysvc.NewRepresentationBackfillHandler(storeIns, representationPolicy)); err != nil {
		return err
	}
	comfyTestExecutor := appsvc.NewComfyUITestExecutor(storeIns)
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
		output, err := applicationExecutor.Execute(ctx, atomicTask)
		if err == nil {
			task.Log(ctx, workflowruntime.WorkerLog("application.execution.completed", workflowruntime.TaskLogLevelInfo, "Application provider execution returned a result."))
		}
		return output, err
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(platformsvc.FunctionAssetThumbnailGenerate, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
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
		execution := &iapiserver.TaskScheduleExecution{ScheduleID: schedule.ID, ExecutionMode: iapiserver.TaskScheduleModeMaterialized, ScheduledAt: imachinery.NewTime(scheduledAt), TriggeredAt: imachinery.Now(), TargetType: schedule.Target.Type, RuntimeExecutionID: task.WorkflowID, Status: iapiserver.ScheduleExecutionStatusTriggered}
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
	if err := startAssetLibraryTaskConsumers(ctx, tasks); err != nil {
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
				_, err = tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{Key: "thumbnail", Name: "Generate asset thumbnail", FunctionRef: platformsvc.FunctionAssetThumbnailGenerate, Arguments: map[string]any{"asset_id": event.AssetID, "thumbnail_id": thumbnail.ID}, RequiredCapabilities: platformsvc.CapabilityAssetThumbnail, ProjectID: event.ProjectID, Namespace: event.Namespace, IdempotencyScope: "asset-thumbnail", IdempotencyKey: "thumbnail:" + event.AssetID + ":" + event.ProfileVersion})
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
	reconciler := taskcentersvc.NewReconciler(storeIns, runtime, cfg.WorkflowRuntimeOptions.ReconcileInterval)
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

func startAssetLibraryTaskConsumers(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
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
				Key: "artifact-process", Name: "Process uploaded Artifact", FunctionRef: assetlibrarysvc.FunctionArtifactProcess,
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
		{Key: "inspect", Task: iapiserver.AtomicTaskTemplate{Key: "inspect", Name: "Inspect AssetVersion representations", FunctionRef: assetlibrarysvc.FunctionRepresentationInspect, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationInspect, Arguments: map[string]any{"asset_id": event.AssetID, "asset_version_id": event.AssetVersionID, "owner_user_id": event.OwnerUserID, "media_type": event.MediaType, "profile_version": event.ProfileVersion}}},
	}
	for _, requested := range event.RequestedRepresentations {
		if requested.RepresentationType == "" || requested.Profile == "" {
			return nil, errors.Errorf("representation requested event contains an invalid representation")
		}
		childKey := requested.RepresentationType + ":" + requested.Profile
		nodes = append(nodes, iapiserver.DAGNode{Key: childKey, Task: iapiserver.AtomicTaskTemplate{Key: childKey, Name: "Generate " + requested.RepresentationType, FunctionRef: assetlibrarysvc.FunctionRepresentationGenerate, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationGenerate, Arguments: map[string]any{"asset_id": event.AssetID, "asset_version_id": event.AssetVersionID, "owner_user_id": event.OwnerUserID, "media_type": event.MediaType, "representation_type": requested.RepresentationType, "profile": requested.Profile, "profile_version": event.ProfileVersion, "required": requested.Required, "max_attempts": 3}, RetryPolicy: iapiserver.RetryPolicy{MaxAttempts: 3, RetryDelaySeconds: 5, BackoffType: "EXPONENTIAL_BACKOFF", MaxRetryDelaySeconds: 30}}})
	}
	nodes = append(nodes, iapiserver.DAGNode{Key: "finalize", Task: iapiserver.AtomicTaskTemplate{Key: "finalize", Name: "Finalize AssetVersion representations", FunctionRef: assetlibrarysvc.FunctionRepresentationFinalize, RequiredCapabilities: assetlibrarysvc.FunctionRepresentationFinalize, Arguments: map[string]any{"asset_version_id": event.AssetVersionID, "owner_user_id": event.OwnerUserID}}})
	edges := make([]iapiserver.DAGEdge, 0, max(1, 2*len(event.RequestedRepresentations)))
	for _, node := range nodes[1 : len(nodes)-1] {
		edges = append(edges, iapiserver.DAGEdge{FromNode: "inspect", ToNode: node.Key}, iapiserver.DAGEdge{FromNode: node.Key, ToNode: "finalize"})
	}
	if len(nodes) == 2 {
		edges = append(edges, iapiserver.DAGEdge{FromNode: "inspect", ToNode: "finalize"})
	}
	return &iapiserver.DAGTaskGroupCreateRequest{
		Name: "Build AssetVersion representations", Nodes: nodes, Edges: edges,
		Input: map[string]any{"asset_version_id": event.AssetVersionID}, ProjectID: event.ProjectID,
		Namespace: event.Namespace, CreatedBy: event.OwnerUserID, IdempotencyScope: "asset-representations", IdempotencyKey: event.IdempotencyKey,
		TriggerType: iapiserver.DAGTriggerDomainEvent, TriggerSourceID: event.AssetVersionID, TriggerSourceName: "asset_version_representation_requested",
	}, nil
}

type workerAssetRegistrar struct{ service platformsvc.PlatformSrv }

func (r *workerAssetRegistrar) Register(ctx context.Context, artifact *iapiserver.ApplicationArtifact) (string, error) {
	response, err := r.service.RegisterArtifact(ctx, &iapiserver.ArtifactRegistrationRequest{ArtifactID: artifact.ID, ApplicationRunID: artifact.ApplicationRunID, OwnerUserID: artifact.OwnerUserID, OutputName: artifact.OutputKey, MediaType: artifact.MediaType, ContentRef: artifact.ContentRef, SizeBytes: 0})
	if err != nil {
		return "", err
	}
	return response.Asset.ID, nil
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
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: "application-platform.engine-health", Description: "Periodic EngineInstance health reconcile"}, SystemKey: appsvc.EngineHealthReconcileRef, CronExpression: cron, TimeZone: "UTC", ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: appsvc.EngineHealthReconcileRef, Config: map[string]any{}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 4, OverallTimeoutSeconds: 5}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func ensureComfyUIObjectInfoSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: "application-platform.comfyui-object-info-refresh", Description: "Daily ComfyUI object_info refresh"}, SystemKey: appsvc.ComfyUIObjectInfoReconcileRef, CronExpression: "0 0 3 * * *", TimeZone: "UTC", ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: appsvc.ComfyUIObjectInfoReconcileRef, Config: map[string]any{}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 300}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
	return err
}

func ensureRepresentationBackfillSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv) error {
	_, err := tasks.EnsureSystemReconcileSchedule(ctx, &iapiserver.TaskSchedule{ObjectMeta: imachinery.ObjectMeta{Name: assetlibrarysvc.RepresentationBackfillRef, Description: "Daily AssetVersion representation backfill"}, SystemKey: assetlibrarysvc.RepresentationBackfillRef, CronExpression: "0 30 3 * * *", TimeZone: "UTC", ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: assetlibrarysvc.RepresentationBackfillRef, Config: map[string]any{"max_actions_per_run": 100}, MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 300}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy})
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
