package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type assetCategory struct {
	ds *datastore
}

func newAssetCategory(ds *datastore) *assetCategory {
	return &assetCategory{ds}
}

func (s *assetCategory) List(
	ctx context.Context,
	param *iapiserver.AssetCategoryListRequest,
) ([]*iapiserver.AssetCategory, int64, error) {
	var meta []*iapiserver.AssetCategory
	var total int64

	resourceSpecificFilter := func(q *gorm.DB) *gorm.DB {
		if param.LibraryID != "" {
			q = q.Where("library_id = ?", param.LibraryID)
		}
		return q
	}

	err := param.ToQuery(ctx, s.ds.db, resourceSpecificFilter).
		Find(&meta).Count(&total).Error

	return meta, total, err
}

func (s *assetCategory) Get(ctx context.Context, id string) (*iapiserver.AssetCategory, error) {
	var meta iapiserver.AssetCategory
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *assetCategory) Add(ctx context.Context, data *iapiserver.AssetCategory) (*iapiserver.AssetCategory, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.AssetCategory{}, map[string]any{
			"name":       data.Name,
			"library_id": data.LibraryID,
		}) {
			return errors.Errorf("exists category name '%v' in library '%v'", data.Name, data.LibraryID)
		}

		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *assetCategory) Update(ctx context.Context, data *iapiserver.AssetCategory) (*iapiserver.AssetCategory, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&iapiserver.AssetCategory{}).Where("id = ?", data.ID).
			Updates(map[string]any{
				"name": data.Name,
			})
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("asset category not found with id %v", data.ID)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *assetCategory) Delete(ctx context.Context, id string, libraryID string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("category_id = ?", id).Delete(&iapiserver.AssetItem{}).Error; err != nil {
			return errors.WithStack(err)
		}

		result := tx.Where("id = ? AND library_id = ?", id, libraryID).Delete(&iapiserver.AssetCategory{})
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("asset category not found with id %v", id)
		}
		return nil
	})
}

func (s *assetCategory) DeleteByLibraryID(ctx context.Context, libraryID string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("library_id = ?", libraryID).Delete(&iapiserver.AssetItem{}).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Where("library_id = ?", libraryID).Delete(&iapiserver.AssetCategory{}).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})
}
