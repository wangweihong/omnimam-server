package usermodel

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	usermodelsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/usermodel"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service *usermodelsvc.Service }

func New(service *usermodelsvc.Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListProviderTypes(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ListProviderTypes(ctx) })
}

func (c *Controller) ListProviders(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderListRequest{}, func(req *iapiserver.ProviderListRequest) (any, error) {
		return c.service.ListProviders(ctx, req)
	})
}

func (c *Controller) CreateProvider(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderCreateRequest{}, func(req *iapiserver.ProviderCreateRequest) (any, error) {
		return c.service.CreateProvider(ctx, req)
	})
}

func (c *Controller) TestUnsavedProvider(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderTestRequest{}, func(req *iapiserver.ProviderTestRequest) (any, error) {
		return c.service.TestUnsavedProvider(ctx, req)
	})
}

func (c *Controller) GetProvider(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetProvider(ctx, ctx.Param("provider_id")) })
}

func (c *Controller) UpdateProvider(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderUpdateRequest{}, func(req *iapiserver.ProviderUpdateRequest) (any, error) {
		return c.service.UpdateProvider(ctx, ctx.Param("provider_id"), req)
	})
}

func (c *Controller) DeleteProvider(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		if err := c.service.DeleteProvider(ctx, ctx.Param("provider_id")); err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil
	})
}

func (c *Controller) TestProvider(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.TestProvider(ctx, ctx.Param("provider_id")) })
}

func (c *Controller) ListProviderModels(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderModelListRequest{}, func(req *iapiserver.ProviderModelListRequest) (any, error) {
		return c.service.ListProviderModels(ctx, ctx.Param("provider_id"), req)
	})
}

func (c *Controller) CreateProviderModel(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderModelCreateRequest{}, func(req *iapiserver.ProviderModelCreateRequest) (any, error) {
		return c.service.CreateProviderModel(ctx, ctx.Param("provider_id"), req)
	})
}

func (c *Controller) SyncProviderModels(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.SyncProviderModels(ctx, ctx.Param("provider_id")) })
}

func (c *Controller) UpdateProviderModel(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderModelUpdateRequest{}, func(req *iapiserver.ProviderModelUpdateRequest) (any, error) {
		return c.service.UpdateProviderModel(ctx, ctx.Param("model_id"), req)
	})
}

func (c *Controller) DeleteProviderModel(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		if err := c.service.DeleteProviderModel(ctx, ctx.Param("model_id")); err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil
	})
}

func (c *Controller) TestProviderModel(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.TestProviderModel(ctx, ctx.Param("model_id")) })
}

func (c *Controller) GetDefaultModel(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetDefaultModel(ctx, ctx.Param("usage")) })
}

func (c *Controller) SaveDefaultModel(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.DefaultModelSaveRequest{}, func(req *iapiserver.DefaultModelSaveRequest) (any, error) {
		return c.service.SaveDefaultModel(ctx, ctx.Param("usage"), req)
	})
}

func (c *Controller) ListModelOptions(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ProviderModelListRequest{}, func(req *iapiserver.ProviderModelListRequest) (any, error) {
		return c.service.ListModelOptions(ctx, req)
	})
}
