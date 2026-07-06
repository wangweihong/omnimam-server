package asset

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type AssetSrv interface {
	AssetLibraryList(
		ctx context.Context,
		req *iapiserver.AssetLibraryListRequest,
	) (*iapiserver.AssetLibraryListResponse, error)
	AssetLibraryCreate(
		ctx context.Context,
		req *iapiserver.AssetLibraryCreateRequest,
	) (*iapiserver.AssetLibraryCreateResponse, error)
	AssetLibraryUpdate(ctx context.Context, req *iapiserver.AssetLibraryUpdateRequest) (*iapiserver.AssetLibrary, error)
	AssetLibraryDelete(ctx context.Context, req *iapiserver.AssetLibraryDeleteRequest) error

	AssetCategoryList(
		ctx context.Context,
		req *iapiserver.AssetCategoryListRequest,
	) (*iapiserver.AssetCategoryListResponse, error)
	AssetCategoryCreate(
		ctx context.Context,
		req *iapiserver.AssetCategoryCreateRequest,
	) (*iapiserver.AssetCategoryCreateResponse, error)
	AssetCategoryUpdate(
		ctx context.Context,
		req *iapiserver.AssetCategoryUpdateRequest,
	) (*iapiserver.AssetCategory, error)
	AssetCategoryDelete(ctx context.Context, req *iapiserver.AssetCategoryDeleteRequest) error

	AssetItemList(ctx context.Context, req *iapiserver.AssetItemListRequest) (*iapiserver.AssetItemListResponse, error)
	AssetItemCreate(
		ctx context.Context,
		req *iapiserver.AssetItemCreateRequest,
	) (*iapiserver.AssetItemCreateResponse, error)
	AssetItemBatchCreate(
		ctx context.Context,
		req *iapiserver.AssetItemBatchCreateRequest,
	) (*iapiserver.AssetItemBatchCreateResponse, error)
	AssetItemUpdate(ctx context.Context, req *iapiserver.AssetItemUpdateRequest) (*iapiserver.AssetItem, error)
	AssetItemDelete(ctx context.Context, req *iapiserver.AssetItemDeleteRequest) error
	AssetItemBatchDelete(ctx context.Context, req *iapiserver.AssetItemBatchDeleteRequest) (int, error)
	AssetItemBatchMove(ctx context.Context, req *iapiserver.AssetItemBatchMoveRequest) (int, error)
	AssetItemClassify(
		ctx context.Context,
		req *iapiserver.AssetItemClassifyRequest,
	) (*iapiserver.AssetItemClassifyResponse, error)
}

type assetService struct {
	store store.Factory
}

func NewService(str store.Factory) *assetService {
	return &assetService{store: str}
}

func (s *assetService) AssetLibraryList(
	ctx context.Context,
	req *iapiserver.AssetLibraryListRequest,
) (*iapiserver.AssetLibraryListResponse, error) {
	metas, total, err := s.store.AssetLibraries().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetLibraryListResponse{
		ListRet: imachineryListRet(total),
		List:    metas,
	}, nil
}

func (s *assetService) AssetLibraryCreate(
	ctx context.Context,
	req *iapiserver.AssetLibraryCreateRequest,
) (*iapiserver.AssetLibraryCreateResponse, error) {
	lib := &iapiserver.AssetLibrary{}
	lib.Name = req.Name

	created, err := s.store.AssetLibraries().Add(ctx, lib)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defaultImageCat := &iapiserver.AssetCategory{}
	defaultImageCat.Name = "默认分组"
	defaultImageCat.LibraryID = created.ID
	defaultImageCat.Type = iapiserver.CategoryTypeImage
	if _, err := s.store.AssetCategories().Add(ctx, defaultImageCat); err != nil {
		return nil, errors.WithStack(err)
	}

	workflowCat := &iapiserver.AssetCategory{}
	workflowCat.Name = "工作流"
	workflowCat.LibraryID = created.ID
	workflowCat.Type = iapiserver.CategoryTypeWorkflow
	if _, err := s.store.AssetCategories().Add(ctx, workflowCat); err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.AssetLibraryCreateResponse{AssetLibrary: *created}, nil
}

func (s *assetService) AssetLibraryUpdate(
	ctx context.Context,
	req *iapiserver.AssetLibraryUpdateRequest,
) (*iapiserver.AssetLibrary, error) {
	lib := &iapiserver.AssetLibrary{}
	lib.ID = req.ID
	lib.Name = req.Name

	updated, err := s.store.AssetLibraries().Update(ctx, lib)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *assetService) AssetLibraryDelete(ctx context.Context, req *iapiserver.AssetLibraryDeleteRequest) error {
	libs, _, err := s.store.AssetLibraries().List(ctx, &iapiserver.AssetLibraryListRequest{})
	if err != nil {
		return errors.WithStack(err)
	}
	if len(libs) <= 1 {
		return errors.Errorf("at least one asset library must be kept")
	}

	if err := s.store.AssetLibraries().Delete(ctx, req.ID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *assetService) AssetCategoryList(
	ctx context.Context,
	req *iapiserver.AssetCategoryListRequest,
) (*iapiserver.AssetCategoryListResponse, error) {
	metas, total, err := s.store.AssetCategories().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetCategoryListResponse{
		ListRet: imachineryListRet(total),
		List:    metas,
	}, nil
}

func (s *assetService) AssetCategoryCreate(
	ctx context.Context,
	req *iapiserver.AssetCategoryCreateRequest,
) (*iapiserver.AssetCategoryCreateResponse, error) {
	cat := &iapiserver.AssetCategory{}
	cat.Name = req.Name
	cat.LibraryID = req.LibraryID
	cat.Type = req.Type
	if cat.Type == "" {
		cat.Type = iapiserver.CategoryTypeImage
	}

	created, err := s.store.AssetCategories().Add(ctx, cat)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetCategoryCreateResponse{Category: *created}, nil
}

func (s *assetService) AssetCategoryUpdate(
	ctx context.Context,
	req *iapiserver.AssetCategoryUpdateRequest,
) (*iapiserver.AssetCategory, error) {
	cat := &iapiserver.AssetCategory{}
	cat.ID = req.ID
	cat.Name = req.Name

	updated, err := s.store.AssetCategories().Update(ctx, cat)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *assetService) AssetCategoryDelete(ctx context.Context, req *iapiserver.AssetCategoryDeleteRequest) error {
	if err := s.store.AssetCategories().Delete(ctx, req.ID, req.LibraryID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *assetService) AssetItemList(
	ctx context.Context,
	req *iapiserver.AssetItemListRequest,
) (*iapiserver.AssetItemListResponse, error) {
	metas, total, err := s.store.AssetItems().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetItemListResponse{
		ListRet: imachineryListRet(total),
		List:    metas,
	}, nil
}

func (s *assetService) AssetItemCreate(
	ctx context.Context,
	req *iapiserver.AssetItemCreateRequest,
) (*iapiserver.AssetItemCreateResponse, error) {
	cat, err := s.store.AssetCategories().Get(ctx, req.CategoryID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if cat.Type != iapiserver.CategoryTypeImage {
		return nil, errors.Errorf("category type '%v' does not support adding media", cat.Type)
	}

	item := &iapiserver.AssetItem{}
	item.Name = req.Name
	item.LibraryID = req.LibraryID
	item.CategoryID = req.CategoryID
	item.URL = req.URL
	item.Kind = iapiserver.AssetKindImage

	created, err := s.store.AssetItems().Add(ctx, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetItemCreateResponse{Item: *created}, nil
}

func (s *assetService) AssetItemBatchCreate(
	ctx context.Context,
	req *iapiserver.AssetItemBatchCreateRequest,
) (*iapiserver.AssetItemBatchCreateResponse, error) {
	cat, err := s.store.AssetCategories().Get(ctx, req.CategoryID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if cat.Type != iapiserver.CategoryTypeImage {
		return nil, errors.Errorf("category type '%v' does not support adding media", cat.Type)
	}

	items := make([]*iapiserver.AssetItem, 0, len(req.Items))
	for _, entry := range req.Items {
		item := &iapiserver.AssetItem{}
		item.Name = entry.Name
		item.LibraryID = req.LibraryID
		item.CategoryID = req.CategoryID
		item.URL = entry.URL
		item.Kind = iapiserver.AssetKindImage
		items = append(items, item)
	}

	created, err := s.store.AssetItems().BatchAdd(ctx, items)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetItemBatchCreateResponse{Items: created}, nil
}

func (s *assetService) AssetItemUpdate(
	ctx context.Context,
	req *iapiserver.AssetItemUpdateRequest,
) (*iapiserver.AssetItem, error) {
	item := &iapiserver.AssetItem{}
	item.ID = req.ID
	item.Name = req.Name

	updated, err := s.store.AssetItems().Update(ctx, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *assetService) AssetItemDelete(ctx context.Context, req *iapiserver.AssetItemDeleteRequest) error {
	if err := s.store.AssetItems().Delete(ctx, req.ID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *assetService) AssetItemBatchDelete(
	ctx context.Context,
	req *iapiserver.AssetItemBatchDeleteRequest,
) (int, error) {
	removed, err := s.store.AssetItems().BatchDelete(ctx, req.IDs, req.LibraryID)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	return removed, nil
}

func (s *assetService) AssetItemBatchMove(ctx context.Context, req *iapiserver.AssetItemBatchMoveRequest) (int, error) {
	_, err := s.store.AssetCategories().Get(ctx, req.TargetCategoryID)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	moved, err := s.store.AssetItems().BatchMove(ctx, req.IDs, req.TargetLibraryID, req.TargetCategoryID)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	return moved, nil
}

func (s *assetService) AssetItemClassify(
	ctx context.Context,
	req *iapiserver.AssetItemClassifyRequest,
) (*iapiserver.AssetItemClassifyResponse, error) {
	items, err := s.store.AssetItems().FindByIDs(ctx, req.IDs, req.LibraryID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	results := make([]iapiserver.AssetItemClassifyResultItem, 0, len(req.IDs))
	successCount := 0

	for _, id := range req.IDs {
		result := iapiserver.AssetItemClassifyResultItem{ID: id}

		var found *iapiserver.AssetItem
		for _, item := range items {
			if item.ID == id {
				found = item
				break
			}
		}
		if found == nil {
			result.Error = "资产不存在"
			results = append(results, result)
			continue
		}

		if found.Kind != iapiserver.AssetKindImage {
			result.Error = "仅支持图片素材智能分类"
			results = append(results, result)
			continue
		}

		result.OK = true
		successCount++
		results = append(results, result)
	}

	return &iapiserver.AssetItemClassifyResponse{
		Count: successCount,
		Items: results,
	}, nil
}

func imachineryListRet(total int64) imachinery.ListRet {
	return imachinery.ListRet{Total: total}
}
