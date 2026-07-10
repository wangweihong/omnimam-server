package postgresql

import (
	"context"
	stderrors "errors"
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
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.AppTemplate{}, map[string]any{
			"owner_user_id": data.OwnerUserID,
			"name":          data.Name,
		}) {
			return errors.NewStatusF(code.ErrTemplateNameDuplicated, "template name %s already exists", data.Name)
		}
		if err := tx.Create(data).Error; err != nil {
			return mapApplicationPlatformUniqueError(err, map[string]int{
				"idx_aiapp_app_templates_owner_name": code.ErrTemplateNameDuplicated,
			}, "template name duplicated")
		}
		return nil
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateTemplate(
	ctx context.Context,
	data *iapiserver.AppTemplate,
) (*iapiserver.AppTemplate, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(mapApplicationPlatformUniqueError(err, map[string]int{
			"idx_aiapp_app_templates_owner_name": code.ErrTemplateNameDuplicated,
		}, "template name duplicated"))
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
		var refs int64
		if err := tx.Model(&iapiserver.ApplicationRun{}).
			Where("application_id = ?", id).
			Count(&refs).Error; err != nil {
			return err
		}
		if refs > 0 || app.ReferenceRunCount > 0 {
			return errors.NewStatusF(code.ErrApplicationReferenceBlocked, "application has run references")
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

func (s *applicationPlatformStore) ListAppEngines(
	ctx context.Context,
	req *iapiserver.AppEngineListRequest,
) ([]*iapiserver.AppEngine, int64, error) {
	var items []*iapiserver.AppEngine
	var total int64
	query := applicationPlatformQuery(ctx, s.ds.db.Model(&iapiserver.AppEngine{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if !req.IncludeAll {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.EngineType != "" {
			q = q.Where("engine_type = ?", req.EngineType)
		}
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		if req.HealthStatus != "" {
			q = q.Where("health_status = ?", req.HealthStatus)
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

func (s *applicationPlatformStore) GetAppEngine(ctx context.Context, id string) (*iapiserver.AppEngine, error) {
	var item iapiserver.AppEngine
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) GetAppEngineByOwnerName(
	ctx context.Context,
	ownerUserID, name string,
) (*iapiserver.AppEngine, error) {
	var item iapiserver.AppEngine
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND name = ?", ownerUserID, name).
		First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddAppEngine(
	ctx context.Context,
	data *iapiserver.AppEngine,
) (*iapiserver.AppEngine, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.AppEngine{}, map[string]any{
			"owner_user_id": data.OwnerUserID,
			"name":          data.Name,
		}) {
			return errors.NewStatusF(code.ErrAppEngineNameDuplicated, "app engine name %s already exists", data.Name)
		}
		if err := tx.Create(data).Error; err != nil {
			return mapApplicationPlatformUniqueError(err, map[string]int{
				"idx_aiapp_app_engines_owner_name": code.ErrAppEngineNameDuplicated,
			}, "app engine name duplicated")
		}
		return nil
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateAppEngine(
	ctx context.Context,
	data *iapiserver.AppEngine,
) (*iapiserver.AppEngine, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(mapApplicationPlatformUniqueError(err, map[string]int{
			"idx_aiapp_app_engines_owner_name": code.ErrAppEngineNameDuplicated,
		}, "app engine name duplicated"))
	}
	return data, nil
}

func (s *applicationPlatformStore) DeleteAppEngine(ctx context.Context, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var engine iapiserver.AppEngine
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&engine).Error; err != nil {
			return err
		}
		var refs int64
		if err := tx.Model(&iapiserver.ApplicationRun{}).
			Where("app_engine_id = ?", id).
			Count(&refs).Error; err != nil {
			return err
		}
		if refs > 0 || engine.ReferenceRunCount > 0 {
			return errors.NewStatusF(code.ErrAppEngineReferenceBlocked, "app engine has run references")
		}
		return tx.Delete(&iapiserver.AppEngine{}, "id = ?", id).Error
	}))
}

func (s *applicationPlatformStore) ListApplicationRuns(
	ctx context.Context,
	req *iapiserver.ApplicationRunListRequest,
) ([]*iapiserver.ApplicationRun, int64, error) {
	var items []*iapiserver.ApplicationRun
	var total int64
	query := applicationPlatformQuery(ctx, s.ds.db.Model(&iapiserver.ApplicationRun{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if !req.IncludeAll {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		if req.ApplicationID != "" {
			q = q.Where("application_id = ?", req.ApplicationID)
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

func (s *applicationPlatformStore) GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error) {
	var item iapiserver.ApplicationRun
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) CreateApplicationRun(
	ctx context.Context,
	data *iapiserver.ApplicationRun,
	definition *iapiserver.TaskDefinition,
	taskRun *iapiserver.TaskRun,
) (*iapiserver.ApplicationRun, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.TaskDefinition
		err := tx.Where("definition_type = ? AND id = ?", definition.DefinitionType, definition.ID).
			First(&existing).Error
		if err != nil {
			if !stderrors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Create(definition).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		taskRun.Input["application_run_id"] = data.ID
		taskRun.Name = data.Name
		if err := tx.Create(taskRun).Error; err != nil {
			return err
		}
		data.TaskRunID = taskRun.ID
		if err := tx.Save(data).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.Application{}).
			Where("id = ?", data.ApplicationID).
			UpdateColumn("reference_run_count", gorm.Expr("reference_run_count + ?", 1)).Error; err != nil {
			return err
		}
		return tx.Model(&iapiserver.AppEngine{}).
			Where("id = ?", data.AppEngineID).
			UpdateColumn("reference_run_count", gorm.Expr("reference_run_count + ?", 1)).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateApplicationRun(
	ctx context.Context,
	data *iapiserver.ApplicationRun,
) (*iapiserver.ApplicationRun, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
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
		"id":                   "id",
		"name":                 "name",
		"createdAt":            "created_at",
		"created_at":           "created_at",
		"updatedAt":            "updated_at",
		"updated_at":           "updated_at",
		"kind":                 "kind",
		"engine_type":          "engine_type",
		"status":               "status",
		"health_status":        "health_status",
		"last_health_check_at": "last_health_check_at",
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

func mapApplicationPlatformUniqueError(err error, constraintCodes map[string]int, message string) error {
	if err == nil {
		return nil
	}
	errText := err.Error()
	if !strings.Contains(errText, "SQLSTATE 23505") && !strings.Contains(errText, "duplicate key value") {
		return err
	}
	for constraint, errCode := range constraintCodes {
		if strings.Contains(errText, constraint) {
			return errors.NewStatus(errCode, message)
		}
	}
	return err
}
