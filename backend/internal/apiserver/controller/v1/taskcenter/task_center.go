package taskcenter

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service taskcenter.TaskCenterSrv }

func NewController(service taskcenter.TaskCenterSrv) *Controller {
	return &Controller{service: service}
}

func (c *Controller) ListAtomicTasks(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskListRequest{}, func(req *iapiserver.AtomicTaskListRequest) (any, error) { return c.service.ListAtomicTasks(ctx, req) })
}
func (c *Controller) CreateAtomicTask(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskCreateRequest{}, func(req *iapiserver.AtomicTaskCreateRequest) (any, error) {
		return c.service.CreateAtomicTask(ctx, req)
	})
}
func (c *Controller) GetAtomicTask(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetAtomicTask(ctx, ctx.Param("atomic_task_id")) })
}
func (c *Controller) ListAtomicTaskAttempts(ctx *gin.Context) {
	req := &iapiserver.TaskAttemptListRequest{AtomicTaskID: ctx.Param("atomic_task_id")}
	core.Run(ctx, req, func(value *iapiserver.TaskAttemptListRequest) (any, error) { return c.service.ListAttempts(ctx, value) })
}
func (c *Controller) CancelAtomicTask(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ActionReasonRequest{}, func(req *iapiserver.ActionReasonRequest) (any, error) {
		return c.service.CancelAtomicTask(ctx, ctx.Param("atomic_task_id"), req)
	})
}
func (c *Controller) RetryAtomicTask(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ActionReasonRequest{}, func(req *iapiserver.ActionReasonRequest) (any, error) {
		return c.service.RetryAtomicTask(ctx, ctx.Param("atomic_task_id"), req)
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
func (c *Controller) ListTaskGroupTasks(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskListRequest{}, func(req *iapiserver.AtomicTaskListRequest) (any, error) {
		return c.service.ListTaskGroupTasks(ctx, ctx.Param("task_group_id"), req)
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
func (c *Controller) ListDAGTaskGroupTasks(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AtomicTaskListRequest{}, func(req *iapiserver.AtomicTaskListRequest) (any, error) {
		return c.service.ListDAGTaskGroupTasks(ctx, ctx.Param("dag_task_group_id"), req)
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

// GetScheduleReconcileState 返回 RECONCILE 单例状态投影，不暴露 checkpoint 原始 JSON。
func (c *Controller) GetScheduleReconcileState(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetScheduleReconcileState(ctx, ctx.Param("task_schedule_id")) })
}
