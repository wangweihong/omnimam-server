package applicationplatform

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appservice "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
)

type Controller struct {
	service appservice.ApplicationPlatformSrv
}

func NewController(service appservice.ApplicationPlatformSrv) *Controller {
	return &Controller{service: service}
}

func (c *Controller) ListProviderCapabilities(ctx *gin.Context) {
	run(ctx, &iapiserver.ProviderCapabilityListRequest{}, func(r *iapiserver.ProviderCapabilityListRequest) (any, error) {
		return c.service.ListProviderCapabilities(ctx, r)
	})
}
func (c *Controller) GetProviderCapability(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetProviderCapability(ctx, ctx.Param("provider_capability_id"))
	})
}
func (c *Controller) ListProviderCapabilityLoadResults(ctx *gin.Context) {
	run(ctx, &iapiserver.ProviderCapabilityLoadResultListRequest{}, func(r *iapiserver.ProviderCapabilityLoadResultListRequest) (any, error) {
		return c.service.ListProviderCapabilityLoadResults(ctx, r)
	})
}
func (c *Controller) ListApplicationEngineTypes(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationEngineTypeListRequest{}, func(r *iapiserver.ApplicationEngineTypeListRequest) (any, error) {
		return c.service.ListApplicationEngineTypes(ctx, r)
	})
}

func (c *Controller) ListEngineInstances(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineInstanceListRequest{}, func(r *iapiserver.EngineInstanceListRequest) (any, error) {
		return c.service.ListEngineInstances(ctx, r)
	})
}
func (c *Controller) CreateEngineInstance(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineInstanceCreateRequest{}, func(r *iapiserver.EngineInstanceCreateRequest) (any, error) {
		return c.service.CreateEngineInstance(ctx, r)
	})
}
func (c *Controller) GetEngineInstance(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetEngineInstance(ctx, ctx.Param("engine_instance_id")) })
}
func (c *Controller) UpdateEngineInstance(ctx *gin.Context) {
	req := &iapiserver.EngineInstanceUpdateRequest{ID: ctx.Param("engine_instance_id")}
	run(ctx, req, func(r *iapiserver.EngineInstanceUpdateRequest) (any, error) {
		return c.service.UpdateEngineInstance(ctx, r)
	})
}
func (c *Controller) DeleteEngineInstance(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.DeleteEngineInstance(ctx, ctx.Param("engine_instance_id")) })
}
func (c *Controller) CheckEngineInstanceHealth(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.CheckEngineInstanceHealth(ctx, ctx.Param("engine_instance_id"))
	})
}

func (c *Controller) ListEngineBindings(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineCapabilityBindingListRequest{}, func(r *iapiserver.EngineCapabilityBindingListRequest) (any, error) {
		return c.service.ListEngineBindings(ctx, r)
	})
}
func (c *Controller) CreateEngineBinding(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineCapabilityBindingCreateRequest{}, func(r *iapiserver.EngineCapabilityBindingCreateRequest) (any, error) {
		return c.service.CreateEngineBinding(ctx, r)
	})
}
func (c *Controller) UpdateEngineBinding(ctx *gin.Context) {
	req := &iapiserver.EngineCapabilityBindingUpdateRequest{ID: ctx.Param("binding_id")}
	run(ctx, req, func(r *iapiserver.EngineCapabilityBindingUpdateRequest) (any, error) {
		return c.service.UpdateEngineBinding(ctx, r)
	})
}
func (c *Controller) DeleteEngineBinding(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.DeleteEngineBinding(ctx, ctx.Param("binding_id")) })
}

func (c *Controller) ListTemplates(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationTemplateListRequest{}, func(r *iapiserver.ApplicationTemplateListRequest) (any, error) {
		return c.service.ListTemplates(ctx, r)
	})
}
func (c *Controller) CreateTemplate(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationTemplateCreateRequest{}, func(r *iapiserver.ApplicationTemplateCreateRequest) (any, error) {
		return c.service.CreateTemplate(ctx, r)
	})
}
func (c *Controller) GetTemplate(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetTemplate(ctx, ctx.Param("application_template_id")) })
}
func (c *Controller) ListTemplateVersions(ctx *gin.Context) {
	req := &iapiserver.ApplicationTemplateVersionListRequest{ApplicationTemplateID: ctx.Param("application_template_id")}
	run(ctx, req, func(r *iapiserver.ApplicationTemplateVersionListRequest) (any, error) {
		return c.service.ListTemplateVersions(ctx, r)
	})
}
func (c *Controller) CreateTemplateVersion(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationTemplateVersionCreateRequest{}, func(r *iapiserver.ApplicationTemplateVersionCreateRequest) (any, error) {
		return c.service.CreateTemplateVersion(ctx, ctx.Param("application_template_id"), r)
	})
}
func (c *Controller) GetTemplateVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetTemplateVersion(ctx, ctx.Param("application_template_version_id"))
	})
}
func (c *Controller) PublishTemplateVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.PublishTemplateVersion(ctx, ctx.Param("application_template_version_id"))
	})
}

func (c *Controller) ListApplications(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationListRequest{}, func(r *iapiserver.ApplicationListRequest) (any, error) { return c.service.ListApplications(ctx, r) })
}
func (c *Controller) CreateApplication(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationCreateRequest{}, func(r *iapiserver.ApplicationCreateRequest) (any, error) { return c.service.CreateApplication(ctx, r) })
}
func (c *Controller) GetApplication(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetApplication(ctx, ctx.Param("application_id")) })
}
func (c *Controller) UpdateApplication(ctx *gin.Context) {
	req := &iapiserver.ApplicationUpdateRequest{ID: ctx.Param("application_id")}
	run(ctx, req, func(r *iapiserver.ApplicationUpdateRequest) (any, error) { return c.service.UpdateApplication(ctx, r) })
}
func (c *Controller) ListApplicationVersions(ctx *gin.Context) {
	req := &iapiserver.ApplicationVersionListRequest{ApplicationID: ctx.Param("application_id")}
	run(ctx, req, func(r *iapiserver.ApplicationVersionListRequest) (any, error) {
		return c.service.ListApplicationVersions(ctx, r)
	})
}
func (c *Controller) CreateApplicationVersion(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationVersionCreateRequest{}, func(r *iapiserver.ApplicationVersionCreateRequest) (any, error) {
		return c.service.CreateApplicationVersion(ctx, ctx.Param("application_id"), r)
	})
}
func (c *Controller) GetApplicationVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetApplicationVersion(ctx, ctx.Param("application_version_id"))
	})
}
func (c *Controller) PublishApplicationVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.PublishApplicationVersion(ctx, ctx.Param("application_version_id"))
	})
}
func (c *Controller) ResolveRuntimeForm(ctx *gin.Context) {
	run(ctx, &iapiserver.RuntimeFormResolveRequest{}, func(r *iapiserver.RuntimeFormResolveRequest) (any, error) {
		return c.service.ResolveRuntimeForm(ctx, ctx.Param("application_id"), r)
	})
}
func (c *Controller) CreateApplicationRun(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationRunCreateRequest{}, func(r *iapiserver.ApplicationRunCreateRequest) (any, error) {
		return c.service.CreateApplicationRun(ctx, ctx.Param("application_id"), r)
	})
}
func (c *Controller) GetApplicationRun(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetApplicationRun(ctx, ctx.Param("application_run_id")) })
}
