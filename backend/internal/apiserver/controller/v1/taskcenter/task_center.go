package taskcenter

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct {
	srv srvv1.Service
}

func NewController(storeIns store.Factory) *Controller {
	return &Controller{srv: srvv1.NewService(storeIns)}
}

// ListTaskDefinitions 查询任务定义列表；只返回 metadata/definition，不创建 async run。
func (tc *Controller) ListTaskDefinitions(c *gin.Context) {
	core.Run(c, &iapiserver.TaskDefinitionListRequest{}, func(r *iapiserver.TaskDefinitionListRequest) (any, error) {
		return tc.srv.TaskCenters().ListDefinitions(c, r)
	})
}

// CreateAtomicTask 创建 AtomicTask 定义；不会直接执行 functionRef 或调用 AppEngine。
func (tc *Controller) CreateAtomicTask(c *gin.Context) {
	core.Run(c, &iapiserver.AtomicTaskCreateRequest{}, func(r *iapiserver.AtomicTaskCreateRequest) (any, error) {
		return tc.srv.TaskCenters().CreateAtomicTask(c, r)
	})
}

// CreateTaskGroup 创建 SERIAL/PARALLEL 任务组定义；只保存组合定义。
func (tc *Controller) CreateTaskGroup(c *gin.Context) {
	core.Run(c, &iapiserver.TaskGroupCreateRequest{}, func(r *iapiserver.TaskGroupCreateRequest) (any, error) {
		return tc.srv.TaskCenters().CreateTaskGroup(c, r)
	})
}

// CreateDAGFlowTask 创建 DAGFlowTask 定义；保存前会校验 DAG 无环。
func (tc *Controller) CreateDAGFlowTask(c *gin.Context) {
	core.Run(c, &iapiserver.DAGFlowTaskCreateRequest{}, func(r *iapiserver.DAGFlowTaskCreateRequest) (any, error) {
		return tc.srv.TaskCenters().CreateDAGFlowTask(c, r)
	})
}

// ListTaskRuns 查询 TaskRun 列表；不返回原始 asset 内容。
func (tc *Controller) ListTaskRuns(c *gin.Context) {
	core.Run(c, &iapiserver.TaskRunListRequest{}, func(r *iapiserver.TaskRunListRequest) (any, error) {
		return tc.srv.TaskCenters().ListRuns(c, r)
	})
}

// CreateTaskRun 创建 TaskRun 运行实例；实际执行由 Worker 协议异步推进。
func (tc *Controller) CreateTaskRun(c *gin.Context) {
	core.Run(c, &iapiserver.TaskRunCreateRequest{}, func(r *iapiserver.TaskRunCreateRequest) (any, error) {
		return tc.srv.TaskCenters().CreateRun(c, r)
	})
}

// GetTaskRun 获取 TaskRun 详情；返回任务状态、进度和结果引用。
func (tc *Controller) GetTaskRun(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return tc.srv.TaskCenters().GetRun(c, c.Param("run_id"))
	})
}

// DeleteTaskRun 软删除终态 TaskRun；不会物理删除 Attempt 或事件历史。
func (tc *Controller) DeleteTaskRun(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return tc.srv.TaskCenters().DeleteRun(c, c.Param("run_id"))
	})
}

// ListTaskAttempts 查询指定 TaskRun 的执行尝试历史。
func (tc *Controller) ListTaskAttempts(c *gin.Context) {
	req := &iapiserver.TaskAttemptListRequest{RunID: c.Param("run_id")}
	core.Run(c, req, func(r *iapiserver.TaskAttemptListRequest) (any, error) {
		return tc.srv.TaskCenters().ListAttempts(c, r)
	})
}

// CancelTaskRun 请求协作式取消 TaskRun；最终状态由 Worker/外部执行器回写。
func (tc *Controller) CancelTaskRun(c *gin.Context) {
	req := &iapiserver.CancelTaskRunRequest{RunID: c.Param("run_id")}
	core.Run(c, req, func(r *iapiserver.CancelTaskRunRequest) (any, error) {
		return tc.srv.TaskCenters().CancelRun(c, r)
	})
}

// RetryTaskRun 将允许重试的失败 TaskRun 放回 READY 队列。
func (tc *Controller) RetryTaskRun(c *gin.Context) {
	req := &iapiserver.RetryTaskRunRequest{RunID: c.Param("run_id")}
	core.Run(c, req, func(r *iapiserver.RetryTaskRunRequest) (any, error) {
		return tc.srv.TaskCenters().RetryRun(c, r)
	})
}

// RegisterWorker 注册 Worker 协议主体及其能力声明。
func (tc *Controller) RegisterWorker(c *gin.Context) {
	core.Run(c, &iapiserver.WorkerRegisterRequest{}, func(r *iapiserver.WorkerRegisterRequest) (any, error) {
		return tc.srv.TaskCenters().RegisterWorker(c, r)
	})
}

// HeartbeatWorker 更新 Worker 心跳、状态和当前运行数量。
func (tc *Controller) HeartbeatWorker(c *gin.Context) {
	req := &iapiserver.WorkerHeartbeatRequest{WorkerID: c.Param("worker_id")}
	core.Run(c, req, func(r *iapiserver.WorkerHeartbeatRequest) (any, error) {
		return tc.srv.TaskCenters().HeartbeatWorker(c, r)
	})
}

// ClaimTaskRun 由 Worker 领取一个可执行 TaskRun，并创建 Attempt 与 ExecutionLease。
func (tc *Controller) ClaimTaskRun(c *gin.Context) {
	req := &iapiserver.ClaimTaskRunRequest{WorkerID: c.Param("worker_id")}
	core.Run(c, req, func(r *iapiserver.ClaimTaskRunRequest) (any, error) {
		return tc.srv.TaskCenters().ClaimRun(c, r)
	})
}

// UpdateTaskRunProgress 校验 lease 后更新 TaskRun 进度和 Attempt 快照。
func (tc *Controller) UpdateTaskRunProgress(c *gin.Context) {
	req := &iapiserver.ProgressUpdateRequest{RunID: c.Param("run_id")}
	core.Run(c, req, func(r *iapiserver.ProgressUpdateRequest) (any, error) {
		return tc.srv.TaskCenters().UpdateProgress(c, r)
	})
}

// CompleteTaskRun 校验 lease 后提交成功结果并释放 ExecutionLease。
func (tc *Controller) CompleteTaskRun(c *gin.Context) {
	req := &iapiserver.TaskRunCompleteRequest{RunID: c.Param("run_id")}
	core.Run(c, req, func(r *iapiserver.TaskRunCompleteRequest) (any, error) {
		return tc.srv.TaskCenters().CompleteRun(c, r)
	})
}

// FailTaskRun 校验 lease 后提交失败结果，并按 retry policy 决定是否等待重试。
func (tc *Controller) FailTaskRun(c *gin.Context) {
	req := &iapiserver.TaskRunFailRequest{RunID: c.Param("run_id")}
	core.Run(c, req, func(r *iapiserver.TaskRunFailRequest) (any, error) {
		return tc.srv.TaskCenters().FailRun(c, r)
	})
}

// RenewExecutionLease 续约 Worker 当前持有的 ExecutionLease。
func (tc *Controller) RenewExecutionLease(c *gin.Context) {
	req := &iapiserver.LeaseRenewRequest{LeaseID: c.Param("lease_id")}
	core.Run(c, req, func(r *iapiserver.LeaseRenewRequest) (any, error) {
		return tc.srv.TaskCenters().RenewLease(c, r)
	})
}

// GetTaskCenterHealth 返回任务中心健康摘要和队列/Worker/Lease 统计。
func (tc *Controller) GetTaskCenterHealth(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return tc.srv.TaskCenters().Health(c)
	})
}
