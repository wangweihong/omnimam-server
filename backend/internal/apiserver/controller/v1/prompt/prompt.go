package prompt

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type PromptController struct {
	srv srvv1.Service
}

func NewController(storeIns store.Factory) *PromptController {
	return &PromptController{
		srv: srvv1.NewService(storeIns),
	}
}

func (pc *PromptController) ListLibraries(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Prompts().PromptLibraryList(c)
	})
}

func (pc *PromptController) CreateLibrary(c *gin.Context) {
	core.Run(c, &iapiserver.PromptLibraryCreateRequest{}, func(r *iapiserver.PromptLibraryCreateRequest) (any, error) {
		return pc.srv.Prompts().PromptLibraryCreate(c, r)
	})
}

func (pc *PromptController) UpdateLibrary(c *gin.Context) {
	req := &iapiserver.PromptLibraryUpdateRequest{}
	req.ID = c.Param("library_id")
	core.Run(c, req, func(r *iapiserver.PromptLibraryUpdateRequest) (any, error) {
		return pc.srv.Prompts().PromptLibraryUpdate(c, r)
	})
}

func (pc *PromptController) DeleteLibrary(c *gin.Context) {
	req := &iapiserver.PromptLibraryDeleteRequest{}
	req.ID = c.Param("library_id")
	core.Run(c, req, func(r *iapiserver.PromptLibraryDeleteRequest) (any, error) {
		if err := pc.srv.Prompts().PromptLibraryDelete(c, r); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (pc *PromptController) CreateItem(c *gin.Context) {
	core.Run(c, &iapiserver.PromptItemCreateRequest{}, func(r *iapiserver.PromptItemCreateRequest) (any, error) {
		return pc.srv.Prompts().PromptItemCreate(c, r)
	})
}

func (pc *PromptController) UpdateItem(c *gin.Context) {
	req := &iapiserver.PromptItemUpdateRequest{}
	req.ID = c.Param("item_id")
	core.Run(c, req, func(r *iapiserver.PromptItemUpdateRequest) (any, error) {
		return pc.srv.Prompts().PromptItemUpdate(c, r)
	})
}

func (pc *PromptController) DeleteItem(c *gin.Context) {
	req := &iapiserver.PromptItemDeleteRequest{}
	req.ID = c.Param("item_id")
	core.Run(c, req, func(r *iapiserver.PromptItemDeleteRequest) (any, error) {
		if err := pc.srv.Prompts().PromptItemDelete(c, r); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (pc *PromptController) BatchDeleteItems(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.PromptItemBatchDeleteRequest{},
		func(r *iapiserver.PromptItemBatchDeleteRequest) (any, error) {
			removed, err := pc.srv.Prompts().PromptItemBatchDelete(c, r)
			if err != nil {
				return nil, err
			}
			return gin.H{"removed": removed}, nil
		},
	)
}

func (pc *PromptController) CreateCategory(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.PromptCategoryCreateRequest{},
		func(r *iapiserver.PromptCategoryCreateRequest) (any, error) {
			return pc.srv.Prompts().PromptCategoryCreate(c, r)
		},
	)
}

func (pc *PromptController) UpdateCategory(c *gin.Context) {
	req := &iapiserver.PromptCategoryUpdateRequest{}
	req.ID = c.Param("category_id")
	core.Run(c, req, func(r *iapiserver.PromptCategoryUpdateRequest) (any, error) {
		return pc.srv.Prompts().PromptCategoryUpdate(c, r)
	})
}

func (pc *PromptController) DeleteCategory(c *gin.Context) {
	req := &iapiserver.PromptCategoryDeleteRequest{}
	req.ID = c.Param("category_id")
	core.Run(c, req, func(r *iapiserver.PromptCategoryDeleteRequest) (any, error) {
		if err := pc.srv.Prompts().PromptCategoryDelete(c, r); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}
