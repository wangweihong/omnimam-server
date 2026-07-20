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

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

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
		return errors.Wrap(err, "start SSE task-center projector")
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
		"comfyui.submit", "comfyui.poll", "comfyui.collect_preview")
	adapters := appsvc.NewEngineAdapters()
	executors := appsvc.NewOperationExecutors()
	events := appsvc.NoopEventPublisher{}
	assetRegistrar := &workerAssetRegistrar{service: platformsvc.NewService(storeIns)}
	applicationExecutor, err := appsvc.NewApplicationRunExecutor(storeIns, runtimeRegistry, capabilities, adapters, executors, assetRegistrar, events)
	if err != nil {
		return err
	}
	thumbnailExecutor := platformsvc.NewThumbnailExecutor(storeIns)
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
	comfyTestExecutor := appsvc.NewComfyUITestExecutor(storeIns)
	if err := runtime.RegisterHandler("comfyui.submit", 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		return comfyTestExecutor.Submit(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("comfyui.poll", 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		return comfyTestExecutor.Poll(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("comfyui.collect_preview", 8, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		return comfyTestExecutor.Collect(ctx, fmt.Sprint(task.Arguments["test_run_id"]))
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("application-platform.run", 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, err := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
		if err != nil {
			return nil, err
		}
		return applicationExecutor.Execute(ctx, atomicTask)
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(platformsvc.FunctionAssetThumbnailGenerate, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		atomicTask, err := storeIns.TaskCenters().GetAtomicTask(ctx, task.AtomicTaskID)
		if err != nil {
			return nil, err
		}
		return thumbnailExecutor.Execute(ctx, atomicTask)
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler(taskcentersvc.ReconcileControllerTask, 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		scheduleID, _ := task.Arguments["task_schedule_id"].(string)
		return tasks.RunScheduleReconcile(ctx, scheduleID, task.WorkflowID, scheduleTime(task.Arguments["scheduled_at"]))
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("task.schedule.acquire", 1, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
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
			return map[string]any{"schedule_execution_id": record.ID, "status": record.Status}, nil
		}
		targetID, err := createScheduleTarget(ctx, tasks, schedule)
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
		return map[string]any{"schedule_execution_id": record.ID, "target_id": targetID, "status": record.Status}, err
	}); err != nil {
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

type workerAssetRegistrar struct{ service platformsvc.PlatformSrv }

func (r *workerAssetRegistrar) Register(ctx context.Context, artifact *iapiserver.ApplicationArtifact) (string, error) {
	response, err := r.service.RegisterArtifact(ctx, &iapiserver.ArtifactRegistrationRequest{ArtifactID: artifact.ID, ApplicationRunID: artifact.ApplicationRunID, OwnerUserID: artifact.OwnerUserID, OutputName: artifact.OutputKey, MediaType: artifact.MediaType, ContentRef: artifact.ContentRef, SizeBytes: 0})
	if err != nil {
		return "", err
	}
	return response.Asset.ID, nil
}

func createScheduleTarget(ctx context.Context, tasks taskcentersvc.TaskCenterSrv, schedule *iapiserver.TaskSchedule) (string, error) {
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
		applyDAGScheduleOwnership(&req, schedule)
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

func applyDAGScheduleOwnership(req *iapiserver.DAGTaskGroupCreateRequest, schedule *iapiserver.TaskSchedule) {
	req.ProjectID = schedule.ProjectID
	req.Namespace = schedule.Namespace
	req.CreatedBy = schedule.CreatedBy
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
