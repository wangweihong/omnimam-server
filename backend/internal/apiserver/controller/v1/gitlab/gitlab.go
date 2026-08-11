package gitlab

import (
	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	gitlabsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/gitlab"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// Controller 暴露独立 GitLab domain 的管理员 Server/Project API。
type Controller struct{ service *gitlabsvc.Service }

func NewController(service *gitlabsvc.Service) *Controller { return &Controller{service: service} }

// ListServers 返回管理员可见的 GitLabServer 列表，不包含 credential。
func (c *Controller) ListServers(ctx *gin.Context) {
	req := &iapiserver.GitLabServerListRequest{}
	if err := core.DecodeParameter(ctx, req); err != nil {
		core.WriteResponse(ctx, errors.WrapStatus(err, code.ErrValidation), nil)
		return
	}
	result, err := c.service.ListServers(ctx, req)
	core.WriteResponse(ctx, err, result)
}

// CreateServer 创建 UNKNOWN GitLabServer；credential 仅写入服务端持久化。
func (c *Controller) CreateServer(ctx *gin.Context) {
	req := &iapiserver.GitLabServerCreateRequest{}
	core.Run(ctx, req, func(input *iapiserver.GitLabServerCreateRequest) (any, error) {
		return c.service.CreateServer(ctx, input)
	})
}

// GetServer 返回单个 GitLabServer 的脱敏元数据。
func (c *Controller) GetServer(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetServer(ctx, ctx.Param("server_id")) })
}

// UpdateServer 局部更新 GitLabServer；省略 credential 时保留原值。
func (c *Controller) UpdateServer(ctx *gin.Context) {
	req := &iapiserver.GitLabServerUpdateRequest{}
	core.Run(ctx, req, func(input *iapiserver.GitLabServerUpdateRequest) (any, error) {
		return c.service.UpdateServer(ctx, ctx.Param("server_id"), input)
	})
}

// DeleteServer 删除无关联 Project 的 GitLabServer。
func (c *Controller) DeleteServer(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		if err := c.service.DeleteServer(ctx, ctx.Param("server_id")); err != nil {
			return nil, err
		}
		return &imachinery.Empty{}, nil
	})
}

// TestServer 在五秒整体 deadline 内检测 GitLab API、用户和固定 Namespace。
func (c *Controller) TestServer(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.TestServer(ctx, ctx.Param("server_id")) })
}

// ListProjects 返回管理员可见的本地 GitLabProject 投影。
func (c *Controller) ListProjects(ctx *gin.Context) {
	req := &iapiserver.GitLabProjectListRequest{}
	if err := core.DecodeParameter(ctx, req); err != nil {
		core.WriteResponse(ctx, errors.WrapStatus(err, code.ErrValidation), nil)
		return
	}
	result, err := c.service.ListProjects(ctx, req)
	core.WriteResponse(ctx, err, result)
}

// CreateProject 在 READY Server 的固定 Namespace 创建 private Project 并保存投影。
func (c *Controller) CreateProject(ctx *gin.Context) {
	req := &iapiserver.GitLabProjectCreateRequest{}
	core.Run(ctx, req, func(input *iapiserver.GitLabProjectCreateRequest) (any, error) {
		return c.service.CreateProject(ctx, input)
	})
}

// GetProject 返回单个本地 GitLabProject 投影。
func (c *Controller) GetProject(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetProject(ctx, ctx.Param("project_id")) })
}

// DeleteProject 先删除远端 Project；远端 404 仍清理本地投影。
func (c *Controller) DeleteProject(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		if err := c.service.DeleteProject(ctx, ctx.Param("project_id")); err != nil {
			return nil, err
		}
		return &imachinery.Empty{}, nil
	})
}
