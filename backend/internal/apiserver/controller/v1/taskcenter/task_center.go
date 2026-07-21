package taskcenter

import (
	"maps"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service taskcenter.TaskCenterSrv }

func NewController(service taskcenter.TaskCenterSrv) *Controller {
	return &Controller{service: service}
}

// ListAtomicTasks 返回当前主体可见的 AtomicTask 契约投影，不暴露持久化与运行时修订字段。
func (c *Controller) ListAtomicTasks(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskListRequest{}, func(req *iapiserver.AtomicTaskListRequest) (any, error) {
		response, err := c.service.ListAtomicTasks(ctx, req)
		if err != nil {
			return nil, err
		}
		return atomicTaskListResponse(response), nil
	})
}

// CreateAtomicTask 创建并启动一个受控 functionRef 的 AtomicTask，响应使用公开契约投影。
func (c *Controller) CreateAtomicTask(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskCreateRequest{}, func(req *iapiserver.AtomicTaskCreateRequest) (any, error) {
		response, err := c.service.CreateAtomicTask(ctx, req)
		if err != nil {
			return nil, err
		}
		return atomicTaskResponse(response), nil
	})
}

// GetAtomicTask 返回当前主体可见的 AtomicTask 详情契约投影。
func (c *Controller) GetAtomicTask(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		response, err := c.service.GetAtomicTask(ctx, ctx.Param("atomic_task_id"))
		if err != nil {
			return nil, err
		}
		return atomicTaskResponse(response), nil
	})
}
func (c *Controller) ListAtomicTaskAttempts(ctx *gin.Context) {
	req := &iapiserver.TaskAttemptListRequest{AtomicTaskID: ctx.Param("atomic_task_id")}
	core.Run(ctx, req, func(value *iapiserver.TaskAttemptListRequest) (any, error) { return c.service.ListAttempts(ctx, value) })
}

// CancelAtomicTask 请求取消非终态 AtomicTask，并返回取消后的契约投影。
func (c *Controller) CancelAtomicTask(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ActionReasonRequest{}, func(req *iapiserver.ActionReasonRequest) (any, error) {
		response, err := c.service.CancelAtomicTask(ctx, ctx.Param("atomic_task_id"), req)
		if err != nil {
			return nil, err
		}
		return atomicTaskResponse(response), nil
	})
}

// RetryAtomicTask 为可手动重试的终态任务创建新 AtomicTask，并返回新任务契约投影。
func (c *Controller) RetryAtomicTask(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ActionReasonRequest{}, func(req *iapiserver.ActionReasonRequest) (any, error) {
		response, err := c.service.RetryAtomicTask(ctx, ctx.Param("atomic_task_id"), req)
		if err != nil {
			return nil, err
		}
		return atomicTaskResponse(response), nil
	})
}

func (c *Controller) ListTaskGroups(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.TaskGroupListRequest{}, func(req *iapiserver.TaskGroupListRequest) (any, error) { return c.service.ListTaskGroups(ctx, req) })
}
func (c *Controller) CreateTaskGroup(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.TaskGroupCreateRequest{}, func(req *iapiserver.TaskGroupCreateRequest) (any, error) { return c.service.CreateTaskGroup(ctx, req) })
}
func (c *Controller) GetTaskGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetTaskGroup(ctx, ctx.Param("task_group_id")) })
}

// ListTaskGroupTasks 返回 TaskGroup 内当前主体可见的 AtomicTask 契约投影。
func (c *Controller) ListTaskGroupTasks(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskListRequest{}, func(req *iapiserver.AtomicTaskListRequest) (any, error) {
		response, err := c.service.ListTaskGroupTasks(ctx, ctx.Param("task_group_id"), req)
		if err != nil {
			return nil, err
		}
		return atomicTaskListResponse(response), nil
	})
}
func (c *Controller) CancelTaskGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.CancelTaskGroup(ctx, ctx.Param("task_group_id")) })
}
func (c *Controller) RetryTaskGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.RetryTaskGroup(ctx, ctx.Param("task_group_id")) })
}

func (c *Controller) ListDAGTaskGroups(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.DAGTaskGroupListRequest{}, func(req *iapiserver.DAGTaskGroupListRequest) (any, error) {
		return c.service.ListDAGTaskGroups(ctx, req)
	})
}
func (c *Controller) CreateDAGTaskGroup(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.DAGTaskGroupCreateRequest{}, func(req *iapiserver.DAGTaskGroupCreateRequest) (any, error) {
		return c.service.CreateDAGTaskGroup(ctx, req)
	})
}
func (c *Controller) GetDAGTaskGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetDAGTaskGroup(ctx, ctx.Param("dag_task_group_id")) })
}

// ListDAGTaskGroupTasks 返回 DAGTaskGroup 内当前主体可见的 AtomicTask 契约投影。
func (c *Controller) ListDAGTaskGroupTasks(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskListRequest{}, func(req *iapiserver.AtomicTaskListRequest) (any, error) {
		response, err := c.service.ListDAGTaskGroupTasks(ctx, ctx.Param("dag_task_group_id"), req)
		if err != nil {
			return nil, err
		}
		return atomicTaskListResponse(response), nil
	})
}
func (c *Controller) CancelDAGTaskGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.CancelDAGTaskGroup(ctx, ctx.Param("dag_task_group_id")) })
}
func (c *Controller) RetryDAGTaskGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.RetryDAGTaskGroup(ctx, ctx.Param("dag_task_group_id")) })
}

func (c *Controller) ListTaskSchedules(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.TaskScheduleListRequest{}, func(req *iapiserver.TaskScheduleListRequest) (any, error) {
		return c.service.ListTaskSchedules(ctx, req)
	})
}
func (c *Controller) CreateTaskSchedule(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.TaskScheduleCreateRequest{}, func(req *iapiserver.TaskScheduleCreateRequest) (any, error) {
		return c.service.CreateTaskSchedule(ctx, req)
	})
}
func (c *Controller) GetTaskSchedule(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetTaskSchedule(ctx, ctx.Param("task_schedule_id")) })
}
func (c *Controller) UpdateTaskSchedule(ctx *gin.Context) {
	req := &iapiserver.TaskScheduleUpdateRequest{ID: ctx.Param("task_schedule_id")}
	core.Run(ctx, req, func(value *iapiserver.TaskScheduleUpdateRequest) (any, error) {
		return c.service.UpdateTaskSchedule(ctx, value)
	})
}
func (c *Controller) DeleteTaskSchedule(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return nil, c.service.DeleteTaskSchedule(ctx, ctx.Param("task_schedule_id")) })
}
func (c *Controller) PauseTaskSchedule(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.PauseTaskSchedule(ctx, ctx.Param("task_schedule_id")) })
}
func (c *Controller) ResumeTaskSchedule(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ResumeTaskSchedule(ctx, ctx.Param("task_schedule_id")) })
}
func (c *Controller) ListScheduleExecutions(ctx *gin.Context) {
	req := &iapiserver.ScheduleExecutionListRequest{ScheduleID: ctx.Param("task_schedule_id")}
	core.Run(ctx, req, func(value *iapiserver.ScheduleExecutionListRequest) (any, error) {
		return c.service.ListScheduleExecutions(ctx, value)
	})
}

func atomicTaskListResponse(response *iapiserver.AtomicTaskListResponse) *iapiserver.AtomicTaskListAPIResponse {
	if response == nil {
		return nil
	}
	items := make([]*iapiserver.AtomicTaskResponse, 0, len(response.Items))
	for _, item := range response.Items {
		if projected := atomicTaskResponse(item); projected != nil {
			items = append(items, projected)
		}
	}
	return &iapiserver.AtomicTaskListAPIResponse{Total: response.Total, Items: items}
}

func atomicTaskResponse(task *iapiserver.AtomicTask) *iapiserver.AtomicTaskResponse {
	if task == nil {
		return nil
	}
	return &iapiserver.AtomicTaskResponse{
		AtomicTaskTemplate: iapiserver.AtomicTaskTemplate{
			Key:                  task.ChildKey,
			Name:                 task.Name,
			FunctionRef:          task.FunctionRef,
			Arguments:            maps.Clone(task.Arguments),
			RequiredCapabilities: task.RequiredCapabilities,
			RetryPolicy:          task.RetryPolicy,
			TimeoutPolicy:        task.TimeoutPolicy,
		},
		ID:                 task.ID,
		Status:             task.Status,
		Progress:           task.Progress,
		ResourceVersion:    task.ResourceVersion,
		CurrentAttempt:     task.CurrentAttempt,
		Output:             maps.Clone(task.Output),
		LastError:          taskErrorResponse(task.LastError),
		RetryOfTaskID:      task.RetryOfTaskID,
		RootTaskID:         task.RootTaskID,
		OwnerType:          task.OwnerType,
		OwnerID:            task.OwnerID,
		RuntimeExecutionID: task.RuntimeExecutionID,
		RuntimeTaskID:      task.RuntimeTaskID,
		ProjectID:          task.ProjectID,
		Namespace:          task.Namespace,
		CreatedBy:          task.CreatedBy,
		CreatedAt:          task.CreatedAt,
		UpdatedAt:          task.UpdatedAt,
		StartedAt:          optionalTime(task.StartedAt),
		CompletedAt:        optionalTime(task.CompletedAt),
		ScheduleSource:     task.ScheduleSource,
	}
}

func taskErrorResponse(taskError iapiserver.TaskError) *iapiserver.TaskErrorResponse {
	if taskError.Code == "" && taskError.Message == "" && taskError.Detail == "" && !taskError.Retryable && taskError.OccurredAt.IsZero() {
		return nil
	}
	return &iapiserver.TaskErrorResponse{
		Code:       taskError.Code,
		Message:    taskError.Message,
		Detail:     taskError.Detail,
		Retryable:  taskError.Retryable,
		OccurredAt: optionalTime(taskError.OccurredAt),
	}
}

func optionalTime(value imachinery.Time) *imachinery.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// GetScheduleReconcileState 返回 RECONCILE 单例状态投影，不暴露 checkpoint 原始 JSON。
func (c *Controller) GetScheduleReconcileState(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetScheduleReconcileState(ctx, ctx.Param("task_schedule_id")) })
}
