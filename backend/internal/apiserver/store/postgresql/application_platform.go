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
		if req.SourceKind != "" {
			q = q.Where("source_kind = ?", req.SourceKind)
		}
		if req.AdapterKey != "" {
			q = q.Where("adapter_key = ?", req.AdapterKey)
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
		if req.SourceType != "" {
			q = q.Where("source_type = ?", req.SourceType)
		}
		if req.CapabilityType != "" {
			q = q.Where("capability_type = ?", req.CapabilityType)
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
	inputs []*iapiserver.InputMapping,
	outputs []*iapiserver.OutputMapping,
) (*iapiserver.Application, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		if len(inputs) > 0 {
			for _, mapping := range inputs {
				mapping.ApplicationID = data.ID
			}
			if err := tx.Create(&inputs).Error; err != nil {
				return err
			}
		}
		if len(outputs) > 0 {
			for _, mapping := range outputs {
				mapping.ApplicationID = data.ID
			}
			if err := tx.Create(&outputs).Error; err != nil {
				return err
			}
		}
		if data.TemplateID == "" {
			return nil
		}
		return tx.Model(&iapiserver.AppTemplate{}).
			Where("id = ?", data.TemplateID).
			UpdateColumn("reference_application_count", gorm.Expr("reference_application_count + ?", 1)).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	data.InputMappings = inputs
	data.OutputMappings = outputs
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
		if err := tx.Where("application_id = ?", id).Delete(&iapiserver.InputMapping{}).Error; err != nil {
			return err
		}
		if err := tx.Where("application_id = ?", id).Delete(&iapiserver.OutputMapping{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&iapiserver.Application{}, "id = ?", id).Error; err != nil {
			return err
		}
		if app.TemplateID == "" {
			return nil
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
		if req.AdapterKey != "" {
			q = q.Where("adapter_key = ?", req.AdapterKey)
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
			Where("resolved_engine_id = ?", id).
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
		if req.ApplicationID != "" {
			q = q.Where("application_id = ?", req.ApplicationID)
		}
		if req.RunMode != "" {
			q = q.Where("run_mode = ?", req.RunMode)
		}
		if req.TaskStatus != "" {
			q = q.Where("task_status_projection = ?", req.TaskStatus)
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
		if data.ResolvedEngineID == "" {
			return nil
		}
		return tx.Model(&iapiserver.AppEngine{}).
			Where("id = ?", data.ResolvedEngineID).
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

func (s *applicationPlatformStore) ListInputMappings(
	ctx context.Context,
	applicationID string,
) ([]*iapiserver.InputMapping, error) {
	var items []*iapiserver.InputMapping
	if err := s.ds.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("sort_order ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) ReplaceInputMappings(
	ctx context.Context,
	applicationID string,
	mappings []*iapiserver.InputMapping,
) ([]*iapiserver.InputMapping, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("application_id = ?", applicationID).Delete(&iapiserver.InputMapping{}).Error; err != nil {
			return err
		}
		if len(mappings) == 0 {
			return nil
		}
		return tx.Create(&mappings).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return s.ListInputMappings(ctx, applicationID)
}

func (s *applicationPlatformStore) ListOutputMappings(
	ctx context.Context,
	applicationID string,
) ([]*iapiserver.OutputMapping, error) {
	var items []*iapiserver.OutputMapping
	if err := s.ds.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("sort_order ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) ReplaceOutputMappings(
	ctx context.Context,
	applicationID string,
	mappings []*iapiserver.OutputMapping,
) ([]*iapiserver.OutputMapping, error) {
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("application_id = ?", applicationID).Delete(&iapiserver.OutputMapping{}).Error; err != nil {
			return err
		}
		if len(mappings) == 0 {
			return nil
		}
		return tx.Create(&mappings).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return s.ListOutputMappings(ctx, applicationID)
}

func (s *applicationPlatformStore) ListAvailableAppEngines(
	ctx context.Context,
	app *iapiserver.Application,
	ownerUserID string,
) ([]*iapiserver.AppEngine, error) {
	var items []*iapiserver.AppEngine
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ?", ownerUserID).
		Where("adapter_key = ?", app.AdapterKey).
		Where("status = ? AND health_status = ?", iapiserver.AppEngineStatusActive, iapiserver.AppEngineHealthHealthy).
		Where("current_inflight < max_concurrency").
		Order("priority ASC, current_inflight ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	filtered := items[:0]
	for _, item := range items {
		if appEngineSupportsOperation(item, app.OperationKey, app.OperationVersion) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *applicationPlatformStore) ReserveAppEngine(ctx context.Context, id string) (*iapiserver.AppEngine, error) {
	var engine iapiserver.AppEngine
	if err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&engine).Error; err != nil {
			return err
		}
		if engine.Status != iapiserver.AppEngineStatusActive ||
			engine.HealthStatus != iapiserver.AppEngineHealthHealthy ||
			engine.CurrentInflight >= engine.MaxConcurrency {
			return errors.NewStatusF(code.ErrSelectedEngineUnavailable, "app engine is unavailable")
		}
		engine.CurrentInflight++
		return tx.Save(&engine).Error
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return &engine, nil
}

func (s *applicationPlatformStore) ReleaseAppEngine(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return errors.WithStack(s.ds.db.WithContext(ctx).Model(&iapiserver.AppEngine{}).
		Where("id = ? AND current_inflight > 0", id).
		UpdateColumn("current_inflight", gorm.Expr("current_inflight - ?", 1)).Error)
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
		"source_kind":          "source_kind",
		"source_type":          "source_type",
		"adapter_key":          "adapter_key",
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

func appEngineSupportsOperation(engine *iapiserver.AppEngine, operationKey, operationVersion string) bool {
	for _, supported := range engine.SupportedOperations {
		if supported.OperationKey != operationKey {
			continue
		}
		if supported.MinVersion != "" && operationVersion < supported.MinVersion {
			continue
		}
		if supported.MaxVersion != "" && operationVersion > supported.MaxVersion {
			continue
		}
		return true
	}
	return false
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
