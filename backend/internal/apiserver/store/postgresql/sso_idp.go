package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type ssoIdp struct {
	ds *datastore
}

func newIdentityProvider(ds *datastore) *ssoIdp {
	return &ssoIdp{ds}
}

func (s *ssoIdp) List(
	ctx context.Context,
	param *iapiserver.IdentityProviderListRequest,
) ([]*iapiserver.IdentityProvider, int64, error) {
	var meta []*iapiserver.IdentityProvider
	var total int64

	resourceSpecificFilter := func(q *gorm.DB) *gorm.DB {
		if param.Protocol != "" {
			q = q.Where("protocol = ?", param.Protocol)
		}
		return q
	}

	err := param.ToQuery(ctx, s.ds.db, resourceSpecificFilter).
		Find(&meta).Count(&total).Error

	return meta, total, err
}

func (s *ssoIdp) Get(ctx context.Context, id string) (*iapiserver.IdentityProvider, error) {
	var meta iapiserver.IdentityProvider
	meta.ID = id
	err := s.ds.db.
		Model(&iapiserver.IdentityProvider{}).
		Find(&meta).Error
	return &meta, err
}

func (s *ssoIdp) GetByName(ctx context.Context, name string) (*iapiserver.IdentityProvider, error) {
	var meta iapiserver.IdentityProvider

	err := s.ds.db.WithContext(ctx).Model(&iapiserver.ServiceProvider{}).
		Where("name = ?", name).
		//sWhere("protocol = ?", protocol).
		First(&meta).Error
	return &meta, err
}

func (s *ssoIdp) Add(ctx context.Context, data *iapiserver.IdentityProvider) (*iapiserver.IdentityProvider, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.IdentityProvider{}, map[string]any{
			"name":     data.Name,
			"endpoint": data.Endpoint,
			"protocol": data.Protocol,
		}) {
			return errors.Errorf(
				"exists with name '%v' protocol '%v' endpoint '%v'",
				data.Name,
				data.Protocol,
				data.Endpoint,
			)
		}

		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *ssoIdp) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Where("id = ?", id).Delete(&iapiserver.IdentityProvider{}).Error
	})
}

func (s *ssoIdp) Update(ctx context.Context, data *iapiserver.IdentityProvider) (*iapiserver.IdentityProvider, error) {
	result := iapiserver.IdentityProvider{}
	// 1. 尝试锁定并查询现有记录（使用悲观锁）
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", data.ID).
			First(&result).Error
		if err != nil {
			return errors.WithStack(err)
		}

		result.Extend = data.Extend
		result.ExtendShadow = data.ExtendShadow
		result.Description = data.Description
		result.Endpoint = data.Endpoint
		result.SAML = data.SAML
		return tx.Model(&result).
			Where("id = ?", result.ID).
			Updates(&result).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &result, nil
}

func (s *ssoIdp) Sync(ctx context.Context, pages []*iapiserver.IdentityProvider) error {
	if len(pages) == 0 {
		return nil
	}

	return nil
}
