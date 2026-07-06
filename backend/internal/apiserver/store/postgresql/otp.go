package postgresql

import (
	"context"
	gerrors "errors"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type otp struct {
	ds *datastore
}

func newUserOTP(ds *datastore) *otp {
	return &otp{ds}
}

func (s *otp) List(ctx context.Context) ([]*iapiserver.UserOTP, error) {
	var meta []*iapiserver.UserOTP
	err := s.ds.db.WithContext(ctx).Model(&iapiserver.UserOTP{}).
		Find(&meta).Error
	return meta, err
}

func (s *otp) GetByUser(ctx context.Context, uid string) (*iapiserver.UserOTP, error) {
	var meta iapiserver.UserOTP
	meta.ID = uid
	err := s.ds.db.
		Model(&iapiserver.UserOTP{}).
		Where("user_id", uid).
		First(&meta).Error
	return &meta, errors.WithStack(err)
}

func (s *otp) Add(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.UserOTP{}, map[string]any{
			"user_id": data.UserID,
		}) {
			return errors.Errorf("exists user id  with %v", data.UserID)
		}

		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *otp) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Where("id = ?", id).Delete(&iapiserver.UserOTP{}).Error
	})
}

// 获取或创建
func (s *otp) FirstOrCreate(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := s.ds.db.WithContext(ctx).Model(&iapiserver.UserOTP{}).
			Where("name = ?", data.Name).
			FirstOrCreate(&data).Error
		return errors.WithStack(err)
	})
	return data, errors.WithStack(err)
}

// 创建或更新
func (s *otp) Upsert(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error) {
	result := iapiserver.UserOTP{}
	// 1. 尝试锁定并查询现有记录（使用悲观锁）
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("name = ?", data.Name).
			First(&result).Error

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
