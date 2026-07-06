package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type promptItem struct {
	ds *datastore
}

func newPromptItem(ds *datastore) *promptItem {
	return &promptItem{ds}
}

func (s *promptItem) ListByLibrary(ctx context.Context, libraryID string) ([]*iapiserver.PromptItem, error) {
	var list []*iapiserver.PromptItem
	err := s.ds.db.WithContext(ctx).
		Where("library_id = ?", libraryID).
		Order("created_at DESC").
		Find(&list).Error
	return list, errors.WithStack(err)
}

func (s *promptItem) Get(ctx context.Context, id string) (*iapiserver.PromptItem, error) {
	var meta iapiserver.PromptItem
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *promptItem) Add(ctx context.Context, data *iapiserver.PromptItem) (*iapiserver.PromptItem, error) {
	err := s.ds.db.WithContext(ctx).Create(data).Error
	return data, errors.WithStack(err)
}

func (s *promptItem) Update(ctx context.Context, data *iapiserver.PromptItem) (*iapiserver.PromptItem, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.PromptItem{}).
		Where("id = ?", data.ID).
		Updates(map[string]any{
			"name":        data.Name,
			"category_id": data.CategoryID,
			"positive":    data.Positive,
			"negative":    data.Negative,
			"scene":       data.Scene,
		})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.Errorf("prompt item not found with id %v", data.ID)
	}
	return data, nil
}

func (s *promptItem) Delete(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&iapiserver.PromptItem{})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("prompt item not found with id %v", id)
	}
	return nil
}

func (s *promptItem) BatchDelete(ctx context.Context, ids []string) (int, error) {
	result := s.ds.db.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&iapiserver.PromptItem{})
	if result.Error != nil {
		return 0, errors.WithStack(result.Error)
	}
	return int(result.RowsAffected), nil
}

func (s *promptItem) ReassignCategory(ctx context.Context, oldCategoryID, newCategoryID string) error {
	return s.ds.db.WithContext(ctx).
		Model(&iapiserver.PromptItem{}).
		Where("category_id = ?", oldCategoryID).
		Update("category_id", newCategoryID).Error
}
