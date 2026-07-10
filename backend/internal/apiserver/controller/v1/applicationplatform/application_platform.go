package applicationplatform

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

// ListTemplates 返回当前用户可见的应用模板列表，不返回运行结果或外部调用内容。
func (c *Controller) ListTemplates(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AppTemplateListRequest{}, func(r *iapiserver.AppTemplateListRequest) (any, error) {
		return c.srv.ApplicationPlatforms().ListTemplates(ctx, r)
	})
}

// CreateTemplate 创建应用模板并在保存前解析可映射变量。
func (c *Controller) CreateTemplate(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AppTemplateCreateRequest{}, func(r *iapiserver.AppTemplateCreateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().CreateTemplate(ctx, r)
	})
}

// GetTemplate 返回一个应用模板详情，包含创建时解析出的变量。
func (c *Controller) GetTemplate(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().GetTemplate(ctx, ctx.Param("template_id"))
	})
}

// UpdateTemplate 仅允许更新模板名称和描述，模板类型、内容和解析变量不可变。
func (c *Controller) UpdateTemplate(ctx *gin.Context) {
	req := &iapiserver.AppTemplateUpdateRequest{ID: ctx.Param("template_id")}
	core.Run(ctx, req, func(r *iapiserver.AppTemplateUpdateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().UpdateTemplate(ctx, r)
	})
}

// DeleteTemplate 删除无引用模板；存在应用引用时返回业务错误。
func (c *Controller) DeleteTemplate(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().DeleteTemplate(ctx, ctx.Param("template_id"))
	})
}

// ListTemplateReferences 返回引用指定模板的应用列表。
func (c *Controller) ListTemplateReferences(ctx *gin.Context) {
	req := &iapiserver.ApplicationListRequest{}
	core.Run(ctx, req, func(r *iapiserver.ApplicationListRequest) (any, error) {
		return c.srv.ApplicationPlatforms().ListTemplateReferences(ctx, ctx.Param("template_id"), r)
	})
}

// ListApplications 返回当前用户可见的正式应用列表。
func (c *Controller) ListApplications(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ApplicationListRequest{}, func(r *iapiserver.ApplicationListRequest) (any, error) {
		return c.srv.ApplicationPlatforms().ListApplications(ctx, r)
	})
}

// CreateApplication 基于模板创建正式应用，并一次性保存完整字段映射。
func (c *Controller) CreateApplication(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ApplicationCreateRequest{}, func(r *iapiserver.ApplicationCreateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().CreateApplication(ctx, r)
	})
}

// GetApplication 返回正式应用详情，包含当前字段映射。
func (c *Controller) GetApplication(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().GetApplication(ctx, ctx.Param("application_id"))
	})
}

// UpdateApplication 仅更新应用名称和描述，不改变 owner_user_id 或模板引用。
func (c *Controller) UpdateApplication(ctx *gin.Context) {
	req := &iapiserver.ApplicationUpdateRequest{ID: ctx.Param("application_id")}
	core.Run(ctx, req, func(r *iapiserver.ApplicationUpdateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().UpdateApplication(ctx, r)
	})
}

// DeleteApplication 删除应用并同步删除字段映射、更新模板引用计数。
func (c *Controller) DeleteApplication(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().DeleteApplication(ctx, ctx.Param("application_id"))
	})
}

// CreateApplicationRun 提交一次 Application 运行，创建 AppRun 并委托 task-center 创建 TaskRun。
func (c *Controller) CreateApplicationRun(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ApplicationRunCreateRequest{}, func(r *iapiserver.ApplicationRunCreateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().CreateApplicationRun(ctx, ctx.Param("application_id"), r)
	})
}

// ListApplicationRuns 返回当前用户可见的 AppRun 列表。
func (c *Controller) ListApplicationRuns(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ApplicationRunListRequest{}, func(r *iapiserver.ApplicationRunListRequest) (any, error) {
		return c.srv.ApplicationPlatforms().ListApplicationRuns(ctx, r)
	})
}

// GetApplicationRun 返回一个 AppRun 详情，包含 TaskRun 关联和运行快照。
func (c *Controller) GetApplicationRun(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().GetApplicationRun(ctx, ctx.Param("run_id"))
	})
}

// ListAppEngines 返回当前用户可见的应用引擎列表，包含明文认证配置。
func (c *Controller) ListAppEngines(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AppEngineListRequest{}, func(r *iapiserver.AppEngineListRequest) (any, error) {
		return c.srv.ApplicationPlatforms().ListAppEngines(ctx, r)
	})
}

// CheckAppEngineHealthByConfig 使用临时连接参数检测健康状态，不创建或更新 AppEngine。
func (c *Controller) CheckAppEngineHealthByConfig(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AppEngineHealthCheckRequest{}, func(r *iapiserver.AppEngineHealthCheckRequest) (any, error) {
		return c.srv.ApplicationPlatforms().CheckAppEngineHealthByConfig(ctx, r)
	})
}

// CreateAppEngine 创建用户级应用引擎；资源归属始终为当前用户。
func (c *Controller) CreateAppEngine(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AppEngineCreateRequest{}, func(r *iapiserver.AppEngineCreateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().CreateAppEngine(ctx, r)
	})
}

// GetAppEngine 返回一个应用引擎详情，包含明文认证配置和健康状态。
func (c *Controller) GetAppEngine(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().GetAppEngine(ctx, ctx.Param("app_engine_id"))
	})
}

// UpdateAppEngine 更新应用引擎元数据、认证配置、状态和能力标签，不改变 owner_user_id。
func (c *Controller) UpdateAppEngine(ctx *gin.Context) {
	req := &iapiserver.AppEngineUpdateRequest{ID: ctx.Param("app_engine_id")}
	core.Run(ctx, req, func(r *iapiserver.AppEngineUpdateRequest) (any, error) {
		return c.srv.ApplicationPlatforms().UpdateAppEngine(ctx, r)
	})
}

// DeleteAppEngine 删除无 AppRun 引用的应用引擎；存在引用时返回业务错误。
func (c *Controller) DeleteAppEngine(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().DeleteAppEngine(ctx, ctx.Param("app_engine_id"))
	})
}

// CheckAppEngineHealth 触发一次应用引擎健康检测并返回写回后的引擎详情。
func (c *Controller) CheckAppEngineHealth(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().CheckAppEngineHealth(ctx, ctx.Param("app_engine_id"))
	})
}

// ListFieldMappings 返回应用当前字段映射列表。
func (c *Controller) ListFieldMappings(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.srv.ApplicationPlatforms().ListFieldMappings(ctx, ctx.Param("application_id"))
	})
}

// SaveFieldMappings 整体替换应用字段映射，并按模板解析变量重新校验。
func (c *Controller) SaveFieldMappings(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.FieldMappingSaveRequest{}, func(r *iapiserver.FieldMappingSaveRequest) (any, error) {
		return c.srv.ApplicationPlatforms().SaveFieldMappings(ctx, ctx.Param("application_id"), r)
	})
}
