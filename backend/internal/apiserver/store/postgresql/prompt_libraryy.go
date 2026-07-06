package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type promptLibrary struct {
	ds *datastore
}

func newPromptLibrary(ds *datastore) *promptLibrary {
	return &promptLibrary{ds}
}

func (s *promptLibrary) List(ctx context.Context) ([]*iapiserver.PromptLibrary, error) {
	var list []*iapiserver.PromptLibrary
	err := s.ds.db.WithContext(ctx).
		Order("created_at ASC").
		Find(&list).Error
	return list, errors.WithStack(err)
}

func (s *promptLibrary) Get(ctx context.Context, id string) (*iapiserver.PromptLibrary, error) {
	var meta iapiserver.PromptLibrary
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *promptLibrary) Add(ctx context.Context, data *iapiserver.PromptLibrary) (*iapiserver.PromptLibrary, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})
	return data, errors.WithStack(err)
}

func (s *promptLibrary) Update(ctx context.Context, data *iapiserver.PromptLibrary) (*iapiserver.PromptLibrary, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.PromptLibrary{}).
		Where("id = ?", data.ID).
		Updates(map[string]any{
			"name": data.Name,
		})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.Errorf("prompt library not found with id %v", data.ID)
	}
	return data, nil
}

func (s *promptLibrary) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("library_id = ?", id).Delete(&iapiserver.PromptCategory{}).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Where("library_id = ?", id).Delete(&iapiserver.PromptItem{}).Error; err != nil {
			return errors.WithStack(err)
		}
		result := tx.Where("id = ?", id).Delete(&iapiserver.PromptLibrary{})
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("prompt library not found with id %v", id)
		}
		return nil
	})
}

func (s *promptLibrary) SetActive(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&iapiserver.PromptLibrary{}).Where("active = ?", true).Update("active", false).Error; err != nil {
			return errors.WithStack(err)
		}
		result := tx.Model(&iapiserver.PromptLibrary{}).Where("id = ?", id).Update("active", true)
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("prompt library not found with id %v", id)
		}
		return nil
	})
}

func (s *promptLibrary) GetActive(ctx context.Context) (*iapiserver.PromptLibrary, error) {
	var meta iapiserver.PromptLibrary
	err := s.ds.db.WithContext(ctx).
		Where("active = ?", true).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}
