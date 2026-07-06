package asset

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type AssetController struct {
	srv srvv1.Service
}

func NewController(store store.Factory) *AssetController {
	return &AssetController{
		srv: srvv1.NewService(store),
	}
}

func (ac *AssetController) ListLibraries(c *gin.Context) {
	core.Run(c, &iapiserver.AssetLibraryListRequest{}, func(r *iapiserver.AssetLibraryListRequest) (any, error) {
		return ac.srv.Assets().AssetLibraryList(c, r)
	})
}

func (ac *AssetController) CreateLibrary(c *gin.Context) {
	core.Run(c, &iapiserver.AssetLibraryCreateRequest{}, func(r *iapiserver.AssetLibraryCreateRequest) (any, error) {
		return ac.srv.Assets().AssetLibraryCreate(c, r)
	})
}

func (ac *AssetController) UpdateLibrary(c *gin.Context) {
	req := &iapiserver.AssetLibraryUpdateRequest{}
	req.ID = c.Param("library_id")
	core.Run(c, req, func(r *iapiserver.AssetLibraryUpdateRequest) (any, error) {
		return ac.srv.Assets().AssetLibraryUpdate(c, r)
	})
}

func (ac *AssetController) DeleteLibrary(c *gin.Context) {
	req := &iapiserver.AssetLibraryDeleteRequest{}
	req.ID = c.Param("library_id")
	core.Run(c, req, func(r *iapiserver.AssetLibraryDeleteRequest) (any, error) {
		if err := ac.srv.Assets().AssetLibraryDelete(c, r); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (ac *AssetController) ListCategories(c *gin.Context) {
	core.Run(c, &iapiserver.AssetCategoryListRequest{}, func(r *iapiserver.AssetCategoryListRequest) (any, error) {
		return ac.srv.Assets().AssetCategoryList(c, r)
	})
}

func (ac *AssetController) CreateCategory(c *gin.Context) {
	core.Run(c, &iapiserver.AssetCategoryCreateRequest{}, func(r *iapiserver.AssetCategoryCreateRequest) (any, error) {
		return ac.srv.Assets().AssetCategoryCreate(c, r)
	})
}

func (ac *AssetController) UpdateCategory(c *gin.Context) {
	req := &iapiserver.AssetCategoryUpdateRequest{}
	req.ID = c.Param("category_id")
	core.Run(c, req, func(r *iapiserver.AssetCategoryUpdateRequest) (any, error) {
		return ac.srv.Assets().AssetCategoryUpdate(c, r)
	})
}

func (ac *AssetController) DeleteCategory(c *gin.Context) {
	req := &iapiserver.AssetCategoryDeleteRequest{}
	req.ID = c.Param("category_id")
	core.Run(c, req, func(r *iapiserver.AssetCategoryDeleteRequest) (any, error) {
		if err := ac.srv.Assets().AssetCategoryDelete(c, r); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (ac *AssetController) ListItems(c *gin.Context) {
	core.Run(c, &iapiserver.AssetItemListRequest{}, func(r *iapiserver.AssetItemListRequest) (any, error) {
		return ac.srv.Assets().AssetItemList(c, r)
	})
}

func (ac *AssetController) CreateItem(c *gin.Context) {
	core.Run(c, &iapiserver.AssetItemCreateRequest{}, func(r *iapiserver.AssetItemCreateRequest) (any, error) {
		return ac.srv.Assets().AssetItemCreate(c, r)
	})
}

func (ac *AssetController) BatchCreateItems(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.AssetItemBatchCreateRequest{},
		func(r *iapiserver.AssetItemBatchCreateRequest) (any, error) {
			return ac.srv.Assets().AssetItemBatchCreate(c, r)
		},
	)
}

func (ac *AssetController) UpdateItem(c *gin.Context) {
	req := &iapiserver.AssetItemUpdateRequest{}
	req.ID = c.Param("item_id")
	core.Run(c, req, func(r *iapiserver.AssetItemUpdateRequest) (any, error) {
		return ac.srv.Assets().AssetItemUpdate(c, r)
	})
}

func (ac *AssetController) DeleteItem(c *gin.Context) {
	req := &iapiserver.AssetItemDeleteRequest{}
	req.ID = c.Param("item_id")
	core.Run(c, req, func(r *iapiserver.AssetItemDeleteRequest) (any, error) {
		if err := ac.srv.Assets().AssetItemDelete(c, r); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (ac *AssetController) BatchDeleteItems(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.AssetItemBatchDeleteRequest{},
		func(r *iapiserver.AssetItemBatchDeleteRequest) (any, error) {
			removed, err := ac.srv.Assets().AssetItemBatchDelete(c, r)
			if err != nil {
				return nil, err
			}
			return gin.H{"removed": removed}, nil
		},
	)
}

func (ac *AssetController) BatchMoveItems(c *gin.Context) {
	core.Run(c, &iapiserver.AssetItemBatchMoveRequest{}, func(r *iapiserver.AssetItemBatchMoveRequest) (any, error) {
		moved, err := ac.srv.Assets().AssetItemBatchMove(c, r)
		if err != nil {
			return nil, err
		}
		return gin.H{"moved": moved}, nil
	})
}

func (ac *AssetController) ClassifyItems(c *gin.Context) {
	core.Run(c, &iapiserver.AssetItemClassifyRequest{}, func(r *iapiserver.AssetItemClassifyRequest) (any, error) {
		return ac.srv.Assets().AssetItemClassify(c, r)
	})
}
