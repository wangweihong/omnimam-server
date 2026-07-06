package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type assetItem struct {
	ds *datastore
}

func newAssetItem(ds *datastore) *assetItem {
	return &assetItem{ds}
}

func (s *assetItem) List(
	ctx context.Context,
	param *iapiserver.AssetItemListRequest,
) ([]*iapiserver.AssetItem, int64, error) {
	var meta []*iapiserver.AssetItem
	var total int64

	resourceSpecificFilter := func(q *gorm.DB) *gorm.DB {
		if param.LibraryID != "" {
			q = q.Where("library_id = ?", param.LibraryID)
		}
		if param.CategoryID != "" {
			q = q.Where("category_id = ?", param.CategoryID)
		}
		if param.Kind != "" {
			q = q.Where("kind = ?", param.Kind)
		}
		return q
	}

	err := param.ToQuery(ctx, s.ds.db, resourceSpecificFilter).
		Find(&meta).Count(&total).Error

	return meta, total, err
}

func (s *assetItem) Get(ctx context.Context, id string) (*iapiserver.AssetItem, error) {
	var meta iapiserver.AssetItem
	err := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		First(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &meta, nil
}

func (s *assetItem) Add(ctx context.Context, data *iapiserver.AssetItem) (*iapiserver.AssetItem, error) {
	err := s.ds.db.WithContext(ctx).Create(data).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *assetItem) BatchAdd(ctx context.Context, items []*iapiserver.AssetItem) ([]*iapiserver.AssetItem, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(items, 100).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *assetItem) Update(ctx context.Context, data *iapiserver.AssetItem) (*iapiserver.AssetItem, error) {
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.AssetItem{}).
		Where("id = ?", data.ID).
		Updates(map[string]any{
			"name": data.Name,
		})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.Errorf("asset item not found with id %v", data.ID)
	}
	return data, nil
}

func (s *assetItem) Delete(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&iapiserver.AssetItem{})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("asset item not found with id %v", id)
	}
	return nil
}

func (s *assetItem) BatchDelete(ctx context.Context, ids []string, libraryID string) (int, error) {
	query := s.ds.db.WithContext(ctx)
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	result := query.Where("id IN ?", ids).Delete(&iapiserver.AssetItem{})
	if result.Error != nil {
		return 0, errors.WithStack(result.Error)
	}
	return int(result.RowsAffected), nil
}

func (s *assetItem) BatchMove(
	ctx context.Context,
	ids []string,
	targetLibraryID, targetCategoryID string,
) (int, error) {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.AssetItem{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"library_id":  targetLibraryID,
			"category_id": targetCategoryID,
		})
	if result.Error != nil {
		return 0, errors.WithStack(result.Error)
	}
	return int(result.RowsAffected), nil
}

func (s *assetItem) FindByIDs(ctx context.Context, ids []string, libraryID string) ([]*iapiserver.AssetItem, error) {
	var meta []*iapiserver.AssetItem
	query := s.ds.db.WithContext(ctx)
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Where("id IN ?", ids).Find(&meta).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return meta, nil
}
