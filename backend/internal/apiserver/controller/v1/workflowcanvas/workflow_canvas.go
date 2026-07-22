package workflowcanvas

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service workflowcanvassvc.Service }

func NewController(service workflowcanvassvc.Service) *Controller {
	return &Controller{service: service}
}

// ListNodeDefinitions 返回当前主体可见的受控节点能力目录。
func (c *Controller) ListNodeDefinitions(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowNodeDefinitionListRequest{}, func(req *iapiserver.WorkflowNodeDefinitionListRequest) (any, error) {
		return c.service.ListNodeDefinitions(ctx, req)
	})
}

// RegisterNodeDefinition 注册新的不可变节点定义版本。
func (c *Controller) RegisterNodeDefinition(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowNodeDefinitionRegisterRequest{}, func(req *iapiserver.WorkflowNodeDefinitionRegisterRequest) (any, error) {
		return c.service.RegisterNodeDefinition(ctx, req)
	})
}

// GetNodeDefinition 查询固定节点定义版本。
func (c *Controller) GetNodeDefinition(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetNodeDefinition(ctx, ctx.Param("node_type"), ctx.Param("definition_version"))
	})
}

// DeprecateNodeDefinition 阻止节点定义被新画布引用。
func (c *Controller) DeprecateNodeDefinition(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowNodeDefinitionDeprecateRequest{}, func(req *iapiserver.WorkflowNodeDefinitionDeprecateRequest) (any, error) {
		return c.service.DeprecateNodeDefinition(ctx, ctx.Param("node_type"), ctx.Param("definition_version"), req)
	})
}
func (c *Controller) List(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasListRequest{}, func(req *iapiserver.WorkflowCanvasListRequest) (any, error) { return c.service.List(ctx, req) })
}
func (c *Controller) Create(ctx *gin.Context) {
	core.Run(
		ctx,
		&iapiserver.WorkflowCanvasCreateRequest{},
		func(req *iapiserver.WorkflowCanvasCreateRequest) (any, error) { return c.service.Create(ctx, req) },
	)
}
func (c *Controller) Get(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Get(ctx, ctx.Param("canvas_id")) })
}
func (c *Controller) Update(ctx *gin.Context) {
	req := &iapiserver.WorkflowCanvasUpdateRequest{ID: ctx.Param("canvas_id")}
	core.Run(ctx, req, func(value *iapiserver.WorkflowCanvasUpdateRequest) (any, error) { return c.service.Update(ctx, value) })
}
func (c *Controller) Delete(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		if err := c.service.Delete(ctx, ctx.Param("canvas_id")); err != nil {
			return nil, err
		}
		return &iapiserver.WorkflowActionResult{Success: true}, nil
	})
}

// ValidateDraft 校验草稿但不创建版本或异步任务。
func (c *Controller) ValidateDraft(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasValidateRequest{}, func(req *iapiserver.WorkflowCanvasValidateRequest) (any, error) {
		return c.service.ValidateDraft(ctx, ctx.Param("canvas_id"), req)
	})
}
func (c *Controller) Publish(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasPublishRequest{}, func(req *iapiserver.WorkflowCanvasPublishRequest) (any, error) {
		return c.service.Publish(ctx, ctx.Param("canvas_id"), req)
	})
}
func (c *Controller) ListVersions(ctx *gin.Context) {
	req := &iapiserver.CanvasVersionListRequest{CanvasID: ctx.Param("canvas_id")}
	core.Run(ctx, req, func(value *iapiserver.CanvasVersionListRequest) (any, error) {
		return c.service.ListVersions(ctx, value)
	})
}
func (c *Controller) GetVersion(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetVersion(ctx, ctx.Param("canvas_version_id")) })
}

// ValidateRun 预检固定版本的 scope、输入和运行策略。
func (c *Controller) ValidateRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunValidateRequest{}, func(req *iapiserver.WorkflowCanvasRunValidateRequest) (any, error) {
		return c.service.ValidateRun(ctx, ctx.Param("canvas_version_id"), req)
	})
}
func (c *Controller) ListRuns(ctx *gin.Context) {
	core.Run(
		ctx,
		&iapiserver.WorkflowCanvasRunListRequest{},
		func(req *iapiserver.WorkflowCanvasRunListRequest) (any, error) { return c.service.ListRuns(ctx, req) },
	)
}
func (c *Controller) CreateRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunCreateRequest{}, func(req *iapiserver.WorkflowCanvasRunCreateRequest) (any, error) {
		return c.service.CreateRun(ctx, req)
	})
}
func (c *Controller) GetRun(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetRun(ctx, ctx.Param("canvas_run_id")) })
}

// ListFlowRuns 返回父 CanvasRun 内的显式流投影。
func (c *Controller) ListFlowRuns(ctx *gin.Context) {
	req := &iapiserver.CanvasFlowRunListRequest{CanvasRunID: ctx.Param("canvas_run_id")}
	core.Run(ctx, req, func(value *iapiserver.CanvasFlowRunListRequest) (any, error) {
		return c.service.ListFlowRuns(ctx, value)
	})
}
func (c *Controller) ListNodeRuns(ctx *gin.Context) {
	req := &iapiserver.CanvasNodeRunListRequest{CanvasRunID: ctx.Param("canvas_run_id")}
	core.Run(ctx, req, func(value *iapiserver.CanvasNodeRunListRequest) (any, error) {
		return c.service.ListNodeRuns(ctx, value)
	})
}

// GetNodeRun 返回节点执行实例及其任务、输出绑定。
func (c *Controller) GetNodeRun(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetNodeRun(ctx, ctx.Param("canvas_node_run_id")) })
}
func (c *Controller) CancelRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunCancelRequest{}, func(req *iapiserver.WorkflowCanvasRunCancelRequest) (any, error) {
		return c.service.CancelRun(ctx, ctx.Param("canvas_run_id"), req)
	})
}
func (c *Controller) RetryRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunRetryRequest{}, func(req *iapiserver.WorkflowCanvasRunRetryRequest) (any, error) {
		return c.service.RetryRun(ctx, ctx.Param("canvas_run_id"), req)
	})
}
