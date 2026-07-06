package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type promptCategory struct {
	ds *datastore
}

func newPromptCategory(ds *datastore) *promptCategory {
	return &promptCategory{ds}
}

func (s *promptCategory) ListByLibrary(ctx context.Context, libraryID string) ([]*iapiserver.PromptCategory, error) {
	var list []*iapiserver.PromptCategory
	err := s.ds.db.WithContext(ctx).
		Where("library_id = ?", libraryID).
		Order("created_at ASC").
		Find(&list).Error
	return list, errors.WithStack(err)
}

func (s *promptCategory) Get(ctx context.Context, id string) (*iapiserver.PromptCategory, error) {
	var meta iapiserver.PromptCategory
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *promptCategory) Add(ctx context.Context, data *iapiserver.PromptCategory) (*iapiserver.PromptCategory, error) {
	err := s.ds.db.WithContext(ctx).Create(data).Error
	return data, errors.WithStack(err)
}

func (s *promptCategory) Update(
	ctx context.Context,
	data *iapiserver.PromptCategory,
) (*iapiserver.PromptCategory, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.PromptCategory{}).
		Where("id = ?", data.ID).
		Updates(map[string]any{
			"name": data.Name,
		})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.Errorf("prompt category not found with id %v", data.ID)
	}
	return data, nil
}

func (s *promptCategory) Delete(ctx context.Context, id string, libraryID string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND library_id = ?", id, libraryID).Delete(&iapiserver.PromptCategory{})
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("prompt category not found with id %v", id)
		}
		return nil
	})
}

func (s *promptCategory) DeleteByLibraryID(ctx context.Context, libraryID string) error {
	return s.ds.db.WithContext(ctx).
		Where("library_id = ?", libraryID).
		Delete(&iapiserver.PromptCategory{}).Error
}
