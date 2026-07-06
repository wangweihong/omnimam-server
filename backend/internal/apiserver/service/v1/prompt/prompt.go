package prompt

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type PromptSrv interface {
	PromptLibraryList(ctx context.Context) (*iapiserver.PromptLibraryListResponse, error)
	PromptLibraryCreate(
		ctx context.Context,
		req *iapiserver.PromptLibraryCreateRequest,
	) (*iapiserver.PromptLibraryFull, error)
	PromptLibraryUpdate(
		ctx context.Context,
		req *iapiserver.PromptLibraryUpdateRequest,
	) (*iapiserver.PromptLibraryFull, error)
	PromptLibraryDelete(ctx context.Context, req *iapiserver.PromptLibraryDeleteRequest) error

	PromptItemCreate(
		ctx context.Context,
		req *iapiserver.PromptItemCreateRequest,
	) (*iapiserver.PromptItemCreateResponse, error)
	PromptItemUpdate(ctx context.Context, req *iapiserver.PromptItemUpdateRequest) (*iapiserver.PromptItem, error)
	PromptItemDelete(ctx context.Context, req *iapiserver.PromptItemDeleteRequest) error
	PromptItemBatchDelete(ctx context.Context, req *iapiserver.PromptItemBatchDeleteRequest) (int, error)

	PromptCategoryCreate(
		ctx context.Context,
		req *iapiserver.PromptCategoryCreateRequest,
	) (*iapiserver.PromptCategory, error)
	PromptCategoryUpdate(
		ctx context.Context,
		req *iapiserver.PromptCategoryUpdateRequest,
	) (*iapiserver.PromptCategory, error)
	PromptCategoryDelete(ctx context.Context, req *iapiserver.PromptCategoryDeleteRequest) error
}

type promptService struct {
	store store.Factory
}

func NewService(str store.Factory) *promptService {
	return &promptService{store: str}
}

func (s *promptService) buildLibraryFull(
	ctx context.Context,
	lib *iapiserver.PromptLibrary,
) (*iapiserver.PromptLibraryFull, error) {
	categories, err := s.store.PromptCategories().ListByLibrary(ctx, lib.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	items, err := s.store.PromptItems().ListByLibrary(ctx, lib.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.PromptLibraryFull{
		PromptLibrary: *lib,
		Categories:    categories,
		Items:         items,
	}, nil
}

func (s *promptService) PromptLibraryList(ctx context.Context) (*iapiserver.PromptLibraryListResponse, error) {
	libs, err := s.store.PromptLibraries().List(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	activeID := ""
	var fullLibs []*iapiserver.PromptLibraryFull
	for _, lib := range libs {
		if lib.Active {
			activeID = lib.ID
		}
		full, err := s.buildLibraryFull(ctx, lib)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		fullLibs = append(fullLibs, full)
	}

	if activeID == "" && len(fullLibs) > 0 {
		activeID = fullLibs[0].ID
	}

	return &iapiserver.PromptLibraryListResponse{
		ActiveLibraryID: activeID,
		Libraries:       fullLibs,
	}, nil
}

func (s *promptService) PromptLibraryCreate(
	ctx context.Context,
	req *iapiserver.PromptLibraryCreateRequest,
) (*iapiserver.PromptLibraryFull, error) {
	lib := &iapiserver.PromptLibrary{}
	lib.Name = req.Name
	lib.System = false
	lib.Readonly = false

	created, err := s.store.PromptLibraries().Add(ctx, lib)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err := s.store.PromptLibraries().SetActive(ctx, created.ID); err != nil {
		return nil, errors.WithStack(err)
	}
	created.Active = true

	return s.buildLibraryFull(ctx, created)
}

func (s *promptService) PromptLibraryUpdate(
	ctx context.Context,
	req *iapiserver.PromptLibraryUpdateRequest,
) (*iapiserver.PromptLibraryFull, error) {
	lib := &iapiserver.PromptLibrary{}
	lib.ID = req.ID
	lib.Name = req.Name

	updated, err := s.store.PromptLibraries().Update(ctx, lib)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.buildLibraryFull(ctx, updated)
}

func (s *promptService) PromptLibraryDelete(ctx context.Context, req *iapiserver.PromptLibraryDeleteRequest) error {
	if req.ID == iapiserver.SystemPromptLibraryID {
		return errors.Errorf("system prompt library cannot be deleted")
	}
	if err := s.store.PromptLibraries().Delete(ctx, req.ID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *promptService) PromptItemCreate(
	ctx context.Context,
	req *iapiserver.PromptItemCreateRequest,
) (*iapiserver.PromptItemCreateResponse, error) {
	_, err := s.store.PromptLibraries().Get(ctx, req.LibraryID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	item := &iapiserver.PromptItem{}
	item.Name = req.Name
	item.LibraryID = req.LibraryID
	item.CategoryID = req.CategoryID
	item.Positive = req.Positive
	item.Negative = req.Negative
	item.Scene = req.Scene

	created, err := s.store.PromptItems().Add(ctx, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err := s.store.PromptLibraries().SetActive(ctx, req.LibraryID); err != nil {
		return nil, errors.WithStack(err)
	}

	return &iapiserver.PromptItemCreateResponse{Item: created}, nil
}

func (s *promptService) PromptItemUpdate(
	ctx context.Context,
	req *iapiserver.PromptItemUpdateRequest,
) (*iapiserver.PromptItem, error) {
	item := &iapiserver.PromptItem{}
	item.ID = req.ID
	item.Name = req.Name
	item.CategoryID = req.CategoryID
	item.Positive = req.Positive
	item.Negative = req.Negative
	item.Scene = req.Scene

	updated, err := s.store.PromptItems().Update(ctx, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *promptService) PromptItemDelete(ctx context.Context, req *iapiserver.PromptItemDeleteRequest) error {
	if err := s.store.PromptItems().Delete(ctx, req.ID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *promptService) PromptItemBatchDelete(
	ctx context.Context,
	req *iapiserver.PromptItemBatchDeleteRequest,
) (int, error) {
	removed, err := s.store.PromptItems().BatchDelete(ctx, req.IDs)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	return removed, nil
}

func (s *promptService) PromptCategoryCreate(
	ctx context.Context,
	req *iapiserver.PromptCategoryCreateRequest,
) (*iapiserver.PromptCategory, error) {
	cat := &iapiserver.PromptCategory{}
	cat.Name = req.Name
	cat.LibraryID = req.LibraryID

	created, err := s.store.PromptCategories().Add(ctx, cat)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *promptService) PromptCategoryUpdate(
	ctx context.Context,
	req *iapiserver.PromptCategoryUpdateRequest,
) (*iapiserver.PromptCategory, error) {
	cat := &iapiserver.PromptCategory{}
	cat.ID = req.ID
	cat.Name = req.Name

	updated, err := s.store.PromptCategories().Update(ctx, cat)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *promptService) PromptCategoryDelete(ctx context.Context, req *iapiserver.PromptCategoryDeleteRequest) error {
	cats, err := s.store.PromptCategories().ListByLibrary(ctx, req.LibraryID)
	if err != nil {
		return errors.WithStack(err)
	}

	var fallbackID string
	for _, c := range cats {
		if c.ID != req.ID {
			fallbackID = c.ID
			break
		}
	}

	if err := s.store.PromptItems().ReassignCategory(ctx, req.ID, fallbackID); err != nil {
		return errors.WithStack(err)
	}

	if err := s.store.PromptCategories().Delete(ctx, req.ID, req.LibraryID); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
