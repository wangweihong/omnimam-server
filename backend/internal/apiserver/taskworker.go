package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"strings"
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
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		return errors.Wrap(err, "load application runtime registry")
	}
	capabilities, err := appregistry.LoadProviderCapabilityRegistry(cfg.ApplicationPlatformOptions.ProviderCapabilityDirectory, runtimeRegistry)
	if err != nil {
		return errors.Wrap(err, "load provider capabilities")
	}
	tasks := taskcentersvc.NewServiceWithFunctions(storeIns, runtime,
		platformsvc.FunctionAssetThumbnailGenerate, "application-platform.run", "task.schedule.acquire",
		"application-platform.engine-health-plan", "application-platform.engine-health-check")
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
	if err := runtime.RegisterHandler("application-platform.engine-health-check", 16, func(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
		engineID, _ := task.Arguments["engine_instance_id"].(string)
		result, err := applicationService.CheckEngineInstanceHealthInternal(ctx, engineID)
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(result)
		var output map[string]any
		_ = json.Unmarshal(raw, &output)
		return output, nil
	}); err != nil {
		return err
	}
	if err := runtime.RegisterHandler("application-platform.engine-health-plan", 1, func(ctx context.Context, workerTask workflowruntime.WorkerTask) (map[string]any, error) {
		planner, err := storeIns.TaskCenters().GetAtomicTask(ctx, workerTask.AtomicTaskID)
		if err != nil {
			return nil, err
		}
		enabled := true
		engines, _, err := storeIns.ApplicationPlatforms().ListEngineInstances(ctx, &iapiserver.EngineInstanceListRequest{Enabled: &enabled})
		if err != nil {
			return nil, err
		}
		cutoff := time.Now().Add(-cfg.ApplicationPlatformOptions.EngineHealthInterval)
		children := make([]*iapiserver.AtomicTask, 0, len(engines))
		dynamicTasks := make([]map[string]any, 0, len(engines))
		dynamicInputs := make(map[string]any, len(engines))
		for _, engine := range engines {
			if engine.LastHealthCheckAt != nil && engine.LastHealthCheckAt.Time.After(cutoff) {
				continue
			}
			key := "engine_" + strings.ReplaceAll(engine.ID, "-", "_")
			child := &iapiserver.AtomicTask{
				FunctionRef: "application-platform.engine-health-check", Arguments: map[string]any{"engine_instance_id": engine.ID},
				TimeoutPolicy: iapiserver.TimeoutPolicy{PerAttemptTimeoutSeconds: 4, OverallTimeoutSeconds: 4},
				Status:        iapiserver.AtomicTaskStatusBlocked, ChildKey: key, ProjectID: planner.ProjectID,
				Namespace: planner.Namespace, CreatedBy: planner.CreatedBy,
			}
			child.ID = uuid.NewString()
			child.RootTaskID = child.ID
			child.Name = "Engine health check"
			children = append(children, child)
			dynamicTasks = append(dynamicTasks, map[string]any{"name": child.FunctionRef, "taskReferenceName": key, "type": "SIMPLE"})
			dynamicInputs[key] = map[string]any{"atomic_task_id": child.ID, "arguments": child.Arguments}
		}
		if len(children) > 0 {
			if err := storeIns.TaskCenters().AddOwnedAtomicTasks(ctx, iapiserver.TaskOwnerTypeDAGGroup, planner.OwnerID, children); err != nil {
				return nil, err
			}
		}
		return map[string]any{"dynamic_tasks": dynamicTasks, "dynamic_inputs": dynamicInputs, "total": len(children)}, nil
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
		execution := &iapiserver.TaskScheduleExecution{ScheduleID: schedule.ID, ScheduledAt: imachinery.NewTime(scheduledAt), TriggeredAt: imachinery.Now(), TargetType: schedule.Target.Type, RuntimeExecutionID: task.WorkflowID, Status: iapiserver.ScheduleExecutionStatusTriggered}
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
		req.ProjectID = schedule.ProjectID
		req.Namespace = schedule.Namespace
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
		req.ProjectID = schedule.ProjectID
		req.Namespace = schedule.Namespace
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
		req.ProjectID = schedule.ProjectID
		req.Namespace = schedule.Namespace
		created, err := tasks.CreateDAGTaskGroup(ctx, &req)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	default:
		return "", fmt.Errorf("unsupported schedule target %s", schedule.Target.Type)
	}
}

func ensureEngineHealthSchedule(ctx context.Context, tasks taskcentersvc.TaskCenterSrv, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	dag := iapiserver.DAGTaskGroupCreateRequest{Name: "Engine health planner", Nodes: []iapiserver.DAGNode{{Key: "plan", Task: iapiserver.AtomicTaskTemplate{Key: "plan", Name: "Plan engine health checks", FunctionRef: "application-platform.engine-health-plan", TimeoutPolicy: iapiserver.TimeoutPolicy{OverallTimeoutSeconds: 5}}, DynamicFork: true, MaxDynamicTasks: iapiserver.MaxDynamicForkTasks}}, Edges: []iapiserver.DAGEdge{}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace}
	if _, err := tasks.CreateDAGTaskGroup(ctx, &dag); err != nil {
		return err
	}
	raw, _ := json.Marshal(dag)
	template := map[string]any{}
	_ = json.Unmarshal(raw, &template)
	target := iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetDAG, Template: template}
	cron := healthCron(interval)
	list, err := tasks.ListTaskSchedules(ctx, &iapiserver.TaskScheduleListRequest{})
	if err != nil {
		return err
	}
	for _, schedule := range list.Items {
		if schedule.Name == "application-platform.engine-health" && schedule.Status != iapiserver.TaskScheduleStatusDeleted {
			_, err := tasks.UpdateTaskSchedule(ctx, &iapiserver.TaskScheduleUpdateRequest{ID: schedule.ID, CronExpression: &cron, Target: &target})
			return err
		}
	}
	_, err = tasks.CreateTaskSchedule(ctx, &iapiserver.TaskScheduleCreateRequest{Name: "application-platform.engine-health", Description: "Periodic EngineInstance health planner", TriggerType: iapiserver.TaskScheduleTriggerCron, CronExpression: cron, TimeZone: "UTC", Target: target, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace})
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
