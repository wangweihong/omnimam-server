package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type serviceProvider struct {
	ds *datastore
}

func newServiceProvider(ds *datastore) *serviceProvider {
	return &serviceProvider{ds}
}

func (s *serviceProvider) List(
	ctx context.Context,
	param *iapiserver.ServiceProviderListRequest,
) ([]*iapiserver.ServiceProvider, int64, error) {
	var meta []*iapiserver.ServiceProvider
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

func (s *serviceProvider) Get(ctx context.Context, id string) (*iapiserver.ServiceProvider, error) {
	var meta iapiserver.ServiceProvider
	meta.ID = id
	err := s.ds.db.
		Model(&iapiserver.ServiceProvider{}).
		Find(&meta).Error
	return &meta, err
}

func (s *serviceProvider) GetByKey(
	ctx context.Context,
	protocol string,
	key string,
) (*iapiserver.ServiceProvider, error) {
	metas, _, err := s.List(ctx, &iapiserver.ServiceProviderListRequest{Protocol: protocol})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, meta := range metas {
		if meta.SAML != nil && meta.SAML.Key == key {
			return meta, nil
		}
		if meta.Oauth2 != nil && meta.Oauth2.ClientID == key {
			return meta, nil
		}
	}
	return nil, errors.Errorf("no service provider with key:%v", key)
}

func (s *serviceProvider) GetByName(ctx context.Context, name string) (*iapiserver.ServiceProvider, error) {
	var meta iapiserver.ServiceProvider

	err := s.ds.db.WithContext(ctx).Model(&iapiserver.ServiceProvider{}).
		Where("name = ?", name).
		First(&meta).Error
	return &meta, err
}

func (s *serviceProvider) Add(
	ctx context.Context,
	data *iapiserver.ServiceProvider,
) (*iapiserver.ServiceProvider, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.ServiceProvider{}, map[string]any{
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

func (s *serviceProvider) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Where("id = ?", id).Delete(&iapiserver.ServiceProvider{}).Error
	})
}

func (s *serviceProvider) Update(
	ctx context.Context,
	data *iapiserver.ServiceProvider,
) (*iapiserver.ServiceProvider, error) {
	result := iapiserver.ServiceProvider{}
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

func (s *serviceProvider) Sync(ctx context.Context, pages []*iapiserver.ServiceProvider) error {
	if len(pages) == 0 {
		return nil
	}

	return nil
}
