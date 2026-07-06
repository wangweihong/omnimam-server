package postgresql

import (
	"context"

	gerrors "errors"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type setting struct {
	ds *datastore
}

func newSetting(ds *datastore) *setting {
	return &setting{ds}
}

func (s *setting) List(ctx context.Context) ([]*iapiserver.Setting, error) {
	var meta []*iapiserver.Setting
	err := s.ds.db.WithContext(ctx).Model(&iapiserver.Setting{}).
		Find(&meta).Error
	return meta, err
}

func (s *setting) Get(ctx context.Context, id string) (*iapiserver.Setting, error) {
	var meta iapiserver.Setting
	meta.ID = id
	err := s.ds.db.
		Model(&iapiserver.Setting{}).
		Find(&meta).Error
	if err != nil {
		return nil, err
	}
	return &meta, err
}

func (s *setting) GetMultiByNames(ctx context.Context, names ...string) ([]*iapiserver.Setting, error) {
	var meta []*iapiserver.Setting

	err := s.ds.db.WithContext(ctx).Model(&iapiserver.Setting{}).
		Where("name IN ?", names).
		First(&meta).Error
	return meta, err
}

func (s *setting) GetByName(ctx context.Context, name string) (*iapiserver.Setting, error) {
	var meta iapiserver.Setting

	err := s.ds.db.WithContext(ctx).Model(&iapiserver.Setting{}).
		Where("name = ?", name).
		First(&meta).Error
	if err != nil {
		return nil, err
	}
	return &meta, err
}

func (s *setting) Add(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.Setting{}, map[string]any{
			"name": data.Name,
		}) {
			return errors.Errorf("exists name with %v", data.Name)
		}

		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	return data, err
}

func (s *setting) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Where("id = ?", id).Delete(&iapiserver.Setting{}).Error
	})
}

// 获取或创建
func (s *setting) FirstOrCreate(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := s.ds.db.WithContext(ctx).Model(&iapiserver.Setting{}).
			Where("name = ?", data.Name).
			FirstOrCreate(&data).Error
		return errors.WithStack(err)
	})
	return data, errors.WithStack(err)
}

// 创建或更新
func (s *setting) Upsert(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error) {
	result := iapiserver.Setting{}
	// 1. 尝试锁定并查询现有记录（使用悲观锁）
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("name = ?", data.Name).
			First(&result).Error
		// err := tx.
		// 	Where("name = ?", data.Name).
		// 	First(&result).Error
		if gerrors.Is(err, gorm.ErrRecordNotFound) {
			result = *data
			return errors.WithStack(tx.Create(&result).Error) // 直接插入新记录
		}

		if err != nil {
			return errors.WithStack(err) // 其他查询错误
		}

		result.Extend = data.Extend
		result.ExtendShadow = data.ExtendShadow
		result.Description = data.Description

		return tx.Model(&result).
			Where("id = ?", result.ID).
			Updates(&result).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &result, nil
}
