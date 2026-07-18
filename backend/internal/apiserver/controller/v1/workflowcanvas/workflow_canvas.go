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
func (c *Controller) List(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasListRequest{}, func(req *iapiserver.WorkflowCanvasListRequest) (any, error) { return c.service.List(ctx, req) })
}
func (c *Controller) Create(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasCreateRequest{}, func(req *iapiserver.WorkflowCanvasCreateRequest) (any, error) { return c.service.Create(ctx, req) })
}
func (c *Controller) Get(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Get(ctx, ctx.Param("canvas_id")) })
}
func (c *Controller) Update(ctx *gin.Context) {
	req := &iapiserver.WorkflowCanvasUpdateRequest{ID: ctx.Param("canvas_id")}
	core.Run(ctx, req, func(value *iapiserver.WorkflowCanvasUpdateRequest) (any, error) { return c.service.Update(ctx, value) })
}
func (c *Controller) Delete(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return nil, c.service.Delete(ctx, ctx.Param("canvas_id")) })
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
func (c *Controller) ListRuns(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunListRequest{}, func(req *iapiserver.WorkflowCanvasRunListRequest) (any, error) { return c.service.ListRuns(ctx, req) })
}
func (c *Controller) CreateRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunCreateRequest{}, func(req *iapiserver.WorkflowCanvasRunCreateRequest) (any, error) {
		return c.service.CreateRun(ctx, req)
	})
}
func (c *Controller) GetRun(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetRun(ctx, ctx.Param("canvas_run_id")) })
}
func (c *Controller) ListNodeRuns(ctx *gin.Context) {
	req := &iapiserver.CanvasNodeRunListRequest{CanvasRunID: ctx.Param("canvas_run_id")}
	core.Run(ctx, req, func(value *iapiserver.CanvasNodeRunListRequest) (any, error) {
		return c.service.ListNodeRuns(ctx, value)
	})
}
func (c *Controller) CancelRun(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.CancelRun(ctx, ctx.Param("canvas_run_id")) })
}
func (c *Controller) RetryRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.WorkflowCanvasRunRetryRequest{}, func(req *iapiserver.WorkflowCanvasRunRetryRequest) (any, error) {
		return c.service.RetryRun(ctx, ctx.Param("canvas_run_id"), req)
	})
}
