package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type user struct {
	ds *datastore
}

func newUser(ds *datastore) *user {
	return &user{ds}
}

func (s *user) List(ctx context.Context, param *iapiserver.UserListRequest) ([]*iapiserver.User, int64, error) {
	var meta []*iapiserver.User
	var total int64

	resourceSpecificFilter := func(q *gorm.DB) *gorm.DB {
		return q
	}

	err := param.ToQuery(ctx, s.ds.db, resourceSpecificFilter).
		Find(&meta).Count(&total).Error

	return meta, total, err
}

func (s *user) Get(ctx context.Context, id string) (*iapiserver.User, error) {
	var meta iapiserver.User
	meta.ID = id
	err := s.ds.db.WithContext(ctx).
		Model(&iapiserver.User{}).
		Find(&meta).Error
	return &meta, err
}

func (s *user) GetByName(ctx context.Context, name string) (*iapiserver.User, error) {
	var meta iapiserver.User

	err := s.ds.db.Model(&iapiserver.User{}).
		Where("name = ?", name).
		First(&meta).Error
	return &meta, err
}

func (s *user) Add(ctx context.Context, data *iapiserver.User) (*iapiserver.User, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.User{}, map[string]any{
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

func (s *user) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&iapiserver.User{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *user) Update(ctx context.Context, data *iapiserver.User) (*iapiserver.User, error) {
	result := iapiserver.User{}
	// 1. 尝试锁定并查询现有记录（使用悲观锁）
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Model(&result).Find(&result).Error; err != nil {
			return errors.WithStack(err)
		}

		if result.Name != data.Name {
			if CheckExists(tx, &result, map[string]any{
				"name": data.Name,
			}) {
				return errors.Errorf("usser exists name with %v", data.Name)
			}
		}

		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&result).
			Where("id = ?", data.ID).
			Updates(&result).Error
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &result, nil
}

func (s *user) Sync(ctx context.Context, pages []*iapiserver.User) error {
	if len(pages) == 0 {
		return nil
	}

	return nil
}
