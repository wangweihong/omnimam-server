package postgresql

import (
	"context"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type applicationPlatformStore struct{ ds *datastore }

func newApplicationPlatform(ds *datastore) *applicationPlatformStore {
	return &applicationPlatformStore{ds: ds}
}

func (s *applicationPlatformStore) ListTemplates(
	ctx context.Context,
	req *iapiserver.AppTemplateListRequest,
) ([]*iapiserver.AppTemplate, int64, error) {
	var items []*iapiserver.AppTemplate
	var total int64
	query := applicationPlatformQuery(ctx, s.ds.db.Model(&iapiserver.AppTemplate{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if !req.IncludeAll {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.Kind != "" {
			q = q.Where("kind = ?", req.Kind)
		}
		return q
	})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	if err := applicationPlatformPaginate(query, req.PageNum, req.PageSize).Find(&items).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *applicationPlatformStore) GetTemplate(ctx context.Context, id string) (*iapiserver.AppTemplate, error) {
	var item iapiserver.AppTemplate
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) GetTemplateByOwnerName(
	ctx context.Context,
	ownerUserID, name string,
) (*iapiserver.AppTemplate, error) {
	var item iapiserver.AppTemplate
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND name = ?", ownerUserID, name).
		First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddTemplate(
	ctx context.Context,
	data *iapiserver.AppTemplate,
) (*iapiserver.AppTemplate, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateTemplate(
	ctx context.Context,
	data *iapiserver.AppTemplate,
) (*iapiserver.AppTemplate, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) DeleteTemplate(ctx context.Context, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var template iapiserver.AppTemplate
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&template).Error; err != nil {
			return err
		}
		var refs int64
		if err := tx.Model(&iapiserver.Application{}).
			Where("template_id = ?", id).
			Count(&refs).Error; err != nil {
			return err
		}
		if refs > 0 || template.ReferenceApplicationCount > 0 {
			return errors.NewStatusF(code.ErrTemplateReferenceBlocked, "template has references")
		}
		return tx.Delete(&iapiserver.AppTemplate{}, "id = ?", id).Error
	}))
}

func (s *applicationPlatformStore) ListApplications(
	ctx context.Context,
	req *iapiserver.ApplicationListRequest,
) ([]*iapiserver.Application, int64, error) {
	var items []*iapiserver.Application
	var total int64
	query := applicationPlatformQuery(ctx, s.ds.db.Model(&iapiserver.Application{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if !req.IncludeAll {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.TemplateID != "" {
			q = q.Where("template_id = ?", req.TemplateID)
		}
		if req.Kind != "" {
			q = q.Where("kind = ?", req.Kind)
		}
		return q
	})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	if err := applicationPlatformPaginate(query, req.PageNum, req.PageSize).Find(&items).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *applicationPlatformStore) GetApplication(ctx context.Context, id string) (*iapiserver.Application, error) {
	var item iapiserver.Application
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddApplication(
	ctx context.Context,
	data *iapiserver.Application,
	mappings []*iapiserver.FieldMapping,
) (*iapiserver.Application, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		if len(mappings) > 0 {
			for _, mapping := range mappings {
				mapping.ApplicationID = data.ID
			}
			if err := tx.Create(&mappings).Error; err != nil {
				return err
			}
		}
		return tx.Model(&iapiserver.AppTemplate{}).
			Where("id = ?", data.TemplateID).
			UpdateColumn("reference_application_count", gorm.Expr("reference_application_count + ?", 1)).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	data.FieldMappings = mappings
	return data, nil
}

func (s *applicationPlatformStore) UpdateApplication(
	ctx context.Context,
	data *iapiserver.Application,
) (*iapiserver.Application, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) DeleteApplication(ctx context.Context, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app iapiserver.Application
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&app).Error; err != nil {
			return err
		}
		if err := tx.Where("application_id = ?", id).Delete(&iapiserver.FieldMapping{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&iapiserver.Application{}, "id = ?", id).Error; err != nil {
			return err
		}
		return tx.Model(&iapiserver.AppTemplate{}).
			Where("id = ? AND reference_application_count > 0", app.TemplateID).
			UpdateColumn("reference_application_count", gorm.Expr("reference_application_count - ?", 1)).Error
	}))
}

func (s *applicationPlatformStore) ListFieldMappings(
	ctx context.Context,
	applicationID string,
) ([]*iapiserver.FieldMapping, error) {
	var items []*iapiserver.FieldMapping
	if err := s.ds.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("sort_order ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) ReplaceFieldMappings(
	ctx context.Context,
	applicationID string,
	mappings []*iapiserver.FieldMapping,
) ([]*iapiserver.FieldMapping, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("application_id = ?", applicationID).Delete(&iapiserver.FieldMapping{}).Error; err != nil {
			return err
		}
		if len(mappings) == 0 {
			return nil
		}
		return tx.Create(&mappings).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return s.ListFieldMappings(ctx, applicationID)
}

func applicationPlatformQuery(
	ctx context.Context,
	db *gorm.DB,
	params imachinery.BasicQueryParam,
	resourceSpecificFilter func(*gorm.DB) *gorm.DB,
) *gorm.DB {
	query := db.WithContext(ctx)
	if params.Keyword != "" {
		keyword := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", keyword, keyword)
	}
	if resourceSpecificFilter != nil {
		query = resourceSpecificFilter(query)
	}
	if params.CreatedAfter != 0 {
		query = query.Where("created_at >= ?", time.Unix(params.CreatedAfter, 0))
	}
	if params.CreatedBefore != 0 {
		query = query.Where("created_at <= ?", time.Unix(params.CreatedBefore, 0))
	}
	return query.Order(applicationPlatformOrder(params.SortField, params.SortOrder))
}

func applicationPlatformPaginate(query *gorm.DB, pageNum, pageSize int) *gorm.DB {
	if pageNum <= 0 || pageSize <= 0 {
		return query
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return query.Offset((pageNum - 1) * pageSize).Limit(pageSize)
}

func applicationPlatformOrder(sortField, sortOrder string) clause.OrderByColumn {
	columns := map[string]string{
		"id":         "id",
		"name":       "name",
		"createdAt":  "created_at",
		"created_at": "created_at",
		"updatedAt":  "updated_at",
		"updated_at": "updated_at",
	}
	column := columns[sortField]
	if column == "" {
		column = "created_at"
	}
	return clause.OrderByColumn{
		Column: clause.Column{Name: column},
		Desc:   strings.ToLower(sortOrder) != "asc",
	}
}
