package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type assetLibrary struct {
	ds *datastore
}

func newAssetLibrary(ds *datastore) *assetLibrary {
	return &assetLibrary{ds}
}

func (s *assetLibrary) List(
	ctx context.Context,
	param *iapiserver.AssetLibraryListRequest,
) ([]*iapiserver.AssetLibrary, int64, error) {
	var meta []*iapiserver.AssetLibrary
	var total int64

	resourceSpecificFilter := func(q *gorm.DB) *gorm.DB {
		return q
	}

	err := param.ToQuery(ctx, s.ds.db, resourceSpecificFilter).
		Find(&meta).Count(&total).Error

	return meta, total, err
}

func (s *assetLibrary) Get(ctx context.Context, id string) (*iapiserver.AssetLibrary, error) {
	var meta iapiserver.AssetLibrary
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *assetLibrary) Add(ctx context.Context, data *iapiserver.AssetLibrary) (*iapiserver.AssetLibrary, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.AssetLibrary{}, map[string]any{
			"name": data.Name,
		}) {
			return errors.Errorf("exists name with %v", data.Name)
		}

		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *assetLibrary) Update(ctx context.Context, data *iapiserver.AssetLibrary) (*iapiserver.AssetLibrary, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.AssetLibrary{}, map[string]any{
			"name": data.Name,
		}) {
			existing := &iapiserver.AssetLibrary{}
			if err := tx.Where("name = ? AND id != ?", data.Name, data.ID).First(existing).Error; err == nil {
				return errors.Errorf("exists name with %v", data.Name)
			}
		}

		result := tx.Model(&iapiserver.AssetLibrary{}).Where("id = ?", data.ID).
			Updates(map[string]any{
				"name": data.Name,
			})
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("asset library not found with id %v", data.ID)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *assetLibrary) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("library_id = ?", id).Delete(&iapiserver.AssetCategory{}).Error; err != nil {
			return errors.WithStack(err)
		}
		if err := tx.Where("library_id = ?", id).Delete(&iapiserver.AssetItem{}).Error; err != nil {
			return errors.WithStack(err)
		}

		result := tx.Where("id = ?", id).Delete(&iapiserver.AssetLibrary{})
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.Errorf("asset library not found with id %v", id)
		}
		return nil
	})
}
