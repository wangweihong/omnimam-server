package postgresql

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type applicationPlatformStore struct{ ds *datastore }

func newApplicationPlatform(ds *datastore) *applicationPlatformStore {
	return &applicationPlatformStore{ds: ds}
}

func (s *applicationPlatformStore) ListEngineInstances(ctx context.Context, req *iapiserver.EngineInstanceListRequest) ([]*iapiserver.EngineInstance, int64, error) {
	var items []*iapiserver.EngineInstance
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.EngineInstance{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if req.ApplicationEngineTypeID != "" {
			q = q.Where("application_engine_type_id = ?", req.ApplicationEngineTypeID)
		}
		if req.HealthStatus != "" {
			q = q.Where("health_status = ?", req.HealthStatus)
		}
		if req.Enabled != nil {
			q = q.Where("enabled = ?", *req.Enabled)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	if err == nil {
		err = s.attachObjectInfoSummaries(ctx, items)
	}
	return items, total, err
}

func (s *applicationPlatformStore) attachObjectInfoSummaries(ctx context.Context, items []*iapiserver.EngineInstance) error {
	ids := make([]string, 0, len(items))
	byID := make(map[string]*iapiserver.EngineInstance, len(items))
	for _, item := range items {
		if item.ApplicationEngineTypeID == "comfyui" {
			ids = append(ids, item.ID)
			byID[item.ID] = item
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var catalogs []*iapiserver.ComfyUIEngineObjectInfo
	if err := s.ds.db.WithContext(ctx).Where("engine_instance_id IN ?", ids).Find(&catalogs).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, catalog := range catalogs {
		item := byID[catalog.EngineInstanceID]
		item.ObjectInfoAvailable = true
		refreshedAt := catalog.RefreshedAt
		item.ObjectInfoRefreshedAt = &refreshedAt
	}
	return nil
}

// ListEnabledEngineInstancesAfter 使用稳定 ID 游标读取巡检分块，避免资源增删导致 offset 漏检。
func (s *applicationPlatformStore) ListEnabledEngineInstancesAfter(ctx context.Context, cursor string, limit int) ([]*iapiserver.EngineInstance, error) {
	if limit <= 0 {
		limit = 1
	}
	var items []*iapiserver.EngineInstance
	err := s.ds.db.WithContext(ctx).Where("enabled = ? AND id > ?", true, cursor).Order("id ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

// ListRefreshableComfyUIEngineInstancesAfter 只返回定时刷新有资格处理的实例。
func (s *applicationPlatformStore) ListRefreshableComfyUIEngineInstancesAfter(ctx context.Context, cursor string, limit int) ([]*iapiserver.EngineInstance, error) {
	if limit <= 0 {
		limit = 1
	}
	var items []*iapiserver.EngineInstance
	err := s.ds.db.WithContext(ctx).
		Where("application_engine_type_id = ? AND enabled = ? AND health_status = ? AND id > ?", "comfyui", true, iapiserver.EngineHealthOnline, cursor).
		Order("id ASC").Limit(limit).Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *applicationPlatformStore) GetEngineInstance(ctx context.Context, id string) (*iapiserver.EngineInstance, error) {
	var item iapiserver.EngineInstance
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddEngineInstance(ctx context.Context, data *iapiserver.EngineInstance) (*iapiserver.EngineInstance, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) AddEngineInstanceWithBindings(ctx context.Context, data *iapiserver.EngineInstance, bindings []*iapiserver.EngineCapabilityBinding) (*iapiserver.EngineInstance, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		for _, binding := range bindings {
			binding.EngineInstanceID = data.ID
			if err := tx.Create(binding).Error; err != nil {
				return fmt.Errorf("%w: %w", store.ErrRequiredEngineBindingFailed, err)
			}
		}
		return nil
	})
	return data, errors.WithStack(err)
}

// EnsureRequiredEngineBindings 复用现有唯一边保证多个进程并发启动时收敛到同一系统绑定。
func (s *applicationPlatformStore) EnsureRequiredEngineBindings(ctx context.Context, engineTypeID, capabilityID, revision, name, description string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var engineIDs []string
		if err := tx.Model(&iapiserver.EngineInstance{}).
			Where("application_engine_type_id = ?", engineTypeID).
			Order("id ASC").
			Pluck("id", &engineIDs).Error; err != nil {
			return err
		}
		for _, engineID := range engineIDs {
			binding := &iapiserver.EngineCapabilityBinding{
				EngineInstanceID:           engineID,
				ProviderCapabilityID:       capabilityID,
				ProviderCapabilityRevision: revision,
				Enabled:                    true,
				Restrictions:               map[string]any{},
			}
			binding.Name = name
			binding.Description = description
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "engine_instance_id"}, {Name: "provider_capability_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"provider_capability_revision": clause.Column{Table: "excluded", Name: "provider_capability_revision"},
					"enabled":                      clause.Column{Table: "excluded", Name: "enabled"},
					"restrictions_json":            clause.Column{Table: "excluded", Name: "restrictions_json"},
					"updated_at":                   gorm.Expr("CURRENT_TIMESTAMP"),
					"resource_version":             gorm.Expr("aiapp_engine_capability_bindings.resource_version + 1"),
				}),
				Where: clause.Where{Exprs: []clause.Expression{gorm.Expr(
					"aiapp_engine_capability_bindings.provider_capability_revision IS DISTINCT FROM EXCLUDED.provider_capability_revision OR aiapp_engine_capability_bindings.enabled IS DISTINCT FROM EXCLUDED.enabled OR aiapp_engine_capability_bindings.restrictions_json IS DISTINCT FROM EXCLUDED.restrictions_json",
				)}},
			}).Create(binding).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func (s *applicationPlatformStore) UpdateEngineInstance(ctx context.Context, data *iapiserver.EngineInstance, expected int64) (*iapiserver.EngineInstance, error) {
	return data, optimisticUpdate(ctx, s.ds.db, data, data.ID, expected)
}

// UpdateEngineInstanceHealth 将健康事实与状态变化 outbox 放在同一事务；状态未变化时 event 为 nil。
func (s *applicationPlatformStore) UpdateEngineInstanceHealth(ctx context.Context, data *iapiserver.EngineInstance, expected int64, event *iapiserver.ApplicationPlatformEvent) (*iapiserver.EngineInstance, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := optimisticUpdate(ctx, tx, data, data.ID, expected); err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		payload := event.Payload
		payload["occurred_at"] = event.OccurredAt
		return publishOutbox(tx, OutboxTopicEngineHealthChanged, event.IdempotencyKey, payload)
	})
	return data, err
}

func (s *applicationPlatformStore) GetComfyUIEngineObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfo, error) {
	var catalog iapiserver.ComfyUIEngineObjectInfo
	if err := s.ds.db.WithContext(ctx).First(&catalog, "engine_instance_id = ?", engineID).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &catalog, nil
}

// RefreshComfyUIEngineObjectInfo 持有 EngineInstance 行锁完成远端读取和原子 upsert，使手动与定时刷新跨副本串行。
func (s *applicationPlatformStore) RefreshComfyUIEngineObjectInfo(ctx context.Context, engineID string, fetch func(*iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error)) (*iapiserver.ComfyUIEngineObjectInfo, error) {
	var catalog *iapiserver.ComfyUIEngineObjectInfo
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var engine iapiserver.EngineInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&engine, "id = ?", engineID).Error; err != nil {
			return err
		}
		fetched, err := fetch(&engine)
		if err != nil {
			return err
		}
		fetched.EngineInstanceID = engineID
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "engine_instance_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"object_info_json", "comfyui_version", "refreshed_at"}),
		}).Create(fetched).Error; err != nil {
			return err
		}
		catalog = fetched
		return nil
	})
	return catalog, errors.WithStack(err)
}

// WithEngineInstanceLock 串行化依赖当前实例事实的复合操作，并与 object-info 刷新使用同一行锁边界。
func (s *applicationPlatformStore) WithEngineInstanceLock(ctx context.Context, engineID string, action func() error) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var engine iapiserver.EngineInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&engine, "id = ?", engineID).Error; err != nil {
			return err
		}
		return action()
	}))
}

func (s *applicationPlatformStore) DeleteEngineInstance(ctx context.Context, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Delete(&iapiserver.EngineInstance{}, "id = ?", id).Error)
}

func (s *applicationPlatformStore) CountRunsByEngineInstance(ctx context.Context, id string) (int64, error) {
	var count int64
	err := s.ds.db.WithContext(ctx).Model(&iapiserver.ApplicationRun{}).Where("engine_instance_id = ?", id).Count(&count).Error
	if err != nil {
		return 0, errors.WithStack(err)
	}
	var validationCount int64
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.ComfyUIWorkflowValidation{}).Where("engine_instance_id = ?", id).Count(&validationCount).Error; err != nil {
		return 0, errors.WithStack(err)
	}
	return count + validationCount, nil
}

func (s *applicationPlatformStore) ListEngineBindings(ctx context.Context, req *iapiserver.EngineCapabilityBindingListRequest) ([]*iapiserver.EngineCapabilityBinding, int64, error) {
	var items []*iapiserver.EngineCapabilityBinding
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.EngineCapabilityBinding{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if req.EngineInstanceID != "" {
			q = q.Where("engine_instance_id = ?", req.EngineInstanceID)
		}
		if req.ProviderCapabilityID != "" {
			q = q.Where("provider_capability_id = ?", req.ProviderCapabilityID)
		}
		if req.Enabled != nil {
			q = q.Where("enabled = ?", *req.Enabled)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *applicationPlatformStore) GetEngineBinding(ctx context.Context, id string) (*iapiserver.EngineCapabilityBinding, error) {
	var item iapiserver.EngineCapabilityBinding
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding) (*iapiserver.EngineCapabilityBinding, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding, expected int64) (*iapiserver.EngineCapabilityBinding, error) {
	return data, optimisticUpdate(ctx, s.ds.db, data, data.ID, expected)
}

func (s *applicationPlatformStore) DeleteEngineBinding(ctx context.Context, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Delete(&iapiserver.EngineCapabilityBinding{}, "id = ?", id).Error)
}

func (s *applicationPlatformStore) ListTemplates(ctx context.Context, req *iapiserver.ApplicationTemplateListRequest) ([]*iapiserver.ApplicationTemplate, int64, error) {
	var items []*iapiserver.ApplicationTemplate
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.ApplicationTemplate{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if req.OwnerUserID != "" {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.CapabilitySourceType != "" {
			q = q.Where("capability_source_type = ?", req.CapabilitySourceType)
		}
		if req.CapabilityDefinitionID != "" {
			q = q.Where("capability_definition_id = ?", req.CapabilityDefinitionID)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *applicationPlatformStore) GetTemplate(ctx context.Context, id string) (*iapiserver.ApplicationTemplate, error) {
	var item iapiserver.ApplicationTemplate
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddTemplateWithVersion(ctx context.Context, data *iapiserver.ApplicationTemplate, version *iapiserver.ApplicationTemplateVersion) (*iapiserver.ApplicationTemplate, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		version.ApplicationTemplateID = data.ID
		return tx.Create(version).Error
	})
	return data, errors.WithStack(err)
}

func (s *applicationPlatformStore) ListTemplateVersions(ctx context.Context, req *iapiserver.ApplicationTemplateVersionListRequest) ([]*iapiserver.ApplicationTemplateVersion, int64, error) {
	var items []*iapiserver.ApplicationTemplateVersion
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.ApplicationTemplateVersion{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		q = q.Where("application_template_id = ?", req.ApplicationTemplateID)
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *applicationPlatformStore) GetTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error) {
	var item iapiserver.ApplicationTemplateVersion
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddTemplateVersion(ctx context.Context, data *iapiserver.ApplicationTemplateVersion) (*iapiserver.ApplicationTemplateVersion, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var template iapiserver.ApplicationTemplate
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&template, "id = ?", data.ApplicationTemplateID).Error; err != nil {
			return err
		}
		var maximum int
		if err := tx.Model(&iapiserver.ApplicationTemplateVersion{}).
			Where("application_template_id = ?", data.ApplicationTemplateID).
			Select("COALESCE(MAX(version), 0)").Scan(&maximum).Error; err != nil {
			return err
		}
		data.Version = maximum + 1
		if data.Name == "" {
			data.Name = fmt.Sprintf("%s v%d", template.Name, data.Version)
		}
		return tx.Create(data).Error
	})
	return data, errors.WithStack(err)
}

func (s *applicationPlatformStore) PublishTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error) {
	var result iapiserver.ApplicationTemplateVersion
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, "id = ?", id).Error; err != nil {
			return err
		}
		if result.Status != iapiserver.VersionStatusDraft {
			return errors.NewStatus(code.ErrAIAppTemplateVersionNotPublishable, "template version is not draft")
		}
		now := imachinery.Now()
		if err := tx.Model(&result).Updates(map[string]any{"status": iapiserver.VersionStatusPublished, "published_at": now, "resource_version": result.ResourceVersion + 1}).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.ApplicationTemplate{}).Where("id = ?", result.ApplicationTemplateID).Updates(map[string]any{"current_version_id": result.ID, "resource_version": gorm.Expr("resource_version + 1")}).Error; err != nil {
			return err
		}
		result.Status, result.PublishedAt, result.ResourceVersion = iapiserver.VersionStatusPublished, &now, result.ResourceVersion+1
		return nil
	})
	return &result, errors.WithStack(err)
}

func (s *applicationPlatformStore) ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) ([]*iapiserver.Application, int64, error) {
	var items []*iapiserver.Application
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.Application{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if req.OwnerUserID != "" {
			if req.IncludeGlobal {
				q = q.Where("owner_user_id = ? OR visibility = ?", req.OwnerUserID, iapiserver.ApplicationVisibilityGlobal)
			} else {
				q = q.Where("owner_user_id = ?", req.OwnerUserID)
			}
		}
		if req.CapabilityDefinitionID != "" {
			q = q.Where("capability_definition_id = ?", req.CapabilityDefinitionID)
		}
		if req.Visibility != "" {
			q = q.Where("visibility = ?", req.Visibility)
		}
		if req.RunEnabled != nil {
			q = q.Where("run_enabled = ?", *req.RunEnabled)
		}
		if req.PublishedOnly {
			q = q.Where("current_version_id IS NOT NULL AND current_version_id <> ''")
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *applicationPlatformStore) GetApplication(ctx context.Context, id string) (*iapiserver.Application, error) {
	var item iapiserver.Application
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) GetApplicationsByIDs(ctx context.Context, ids []string) ([]*iapiserver.Application, error) {
	if len(ids) == 0 {
		return []*iapiserver.Application{}, nil
	}
	var items []*iapiserver.Application
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) AddApplication(ctx context.Context, data *iapiserver.Application) (*iapiserver.Application, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateApplication(ctx context.Context, data *iapiserver.Application, expected int64) (*iapiserver.Application, error) {
	return data, optimisticUpdate(ctx, s.ds.db, data, data.ID, expected)
}

func (s *applicationPlatformStore) ListApplicationVersions(ctx context.Context, req *iapiserver.ApplicationVersionListRequest) ([]*iapiserver.ApplicationVersion, int64, error) {
	var items []*iapiserver.ApplicationVersion
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.ApplicationVersion{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		q = q.Where("application_id = ?", req.ApplicationID)
		if req.Status != "" {
			q = q.Where("status = ?", req.Status)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *applicationPlatformStore) ListPublishedApplicationVersions(ctx context.Context) ([]*iapiserver.ApplicationVersion, error) {
	var items []*iapiserver.ApplicationVersion
	if err := s.ds.db.WithContext(ctx).Where("status = ?", iapiserver.VersionStatusPublished).
		Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) GetApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error) {
	var item iapiserver.ApplicationVersion
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) GetApplicationVersionsByIDs(
	ctx context.Context,
	ids []string,
) ([]*iapiserver.ApplicationVersion, error) {
	if len(ids) == 0 {
		return []*iapiserver.ApplicationVersion{}, nil
	}
	var items []*iapiserver.ApplicationVersion
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) AddApplicationVersion(ctx context.Context, data *iapiserver.ApplicationVersion) (*iapiserver.ApplicationVersion, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) PublishApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error) {
	var result iapiserver.ApplicationVersion
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, "id = ?", id).Error; err != nil {
			return err
		}
		if result.Status != iapiserver.VersionStatusDraft {
			return errors.NewStatus(code.ErrAIAppApplicationVersionNotPublishable, "application version is not draft")
		}
		now := imachinery.Now()
		if err := tx.Model(&result).Updates(map[string]any{"status": iapiserver.VersionStatusPublished, "published_at": now, "resource_version": result.ResourceVersion + 1}).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.Application{}).Where("id = ?", result.ApplicationID).Updates(map[string]any{"current_version_id": result.ID, "resource_version": gorm.Expr("resource_version + 1")}).Error; err != nil {
			return err
		}
		result.Status, result.PublishedAt, result.ResourceVersion = iapiserver.VersionStatusPublished, &now, result.ResourceVersion+1
		var application iapiserver.Application
		if err := tx.First(&application, "id = ?", result.ApplicationID).Error; err != nil {
			return err
		}
		payload := map[string]any{
			"application_id":                  application.ID,
			"application_version_id":          result.ID,
			"application_template_version_id": result.ApplicationTemplateVersionID,
			"semantic_version":                result.SemanticVersion,
			"application_name":                application.Name,
			"owner_user_id":                   application.OwnerUserID,
			"visibility":                      application.Visibility,
			"canvas_enabled":                  application.CanvasEnabled,
			"run_enabled":                     application.RunEnabled,
			"input_schema":                    result.InputSchema,
			"output_schema":                   result.OutputSchema,
			"published_at":                    now,
		}
		return publishOutbox(tx, OutboxTopicApplicationVersionPublished, result.ID+":"+result.SemanticVersion, payload)
	})
	return &result, errors.WithStack(err)
}

func (s *applicationPlatformStore) GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error) {
	var item iapiserver.ApplicationRun
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	artifacts, err := s.ListApplicationArtifactRefsByRun(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Artifacts = artifacts
	return &item, nil
}

func (s *applicationPlatformStore) ListApplicationRuns(ctx context.Context, req *iapiserver.ApplicationRunListRequest) ([]*iapiserver.ApplicationRun, int64, error) {
	var items []*iapiserver.ApplicationRun
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.ApplicationRun{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		return q.Where("application_id = ?", req.ApplicationID)
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	if err != nil || len(items) == 0 {
		return items, total, err
	}
	runIDs := make([]string, 0, len(items))
	byID := make(map[string]*iapiserver.ApplicationRun, len(items))
	for _, item := range items {
		item.Artifacts = make([]*iapiserver.ApplicationArtifactRef, 0)
		runIDs = append(runIDs, item.ID)
		byID[item.ID] = item
	}
	var artifacts []*iapiserver.ApplicationArtifactRef
	if err := s.ds.db.WithContext(ctx).Where("application_run_id IN ?", runIDs).Order("output_key ASC, sequence ASC, id ASC").Find(&artifacts).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	for _, artifact := range artifacts {
		byID[artifact.ApplicationRunID].Artifacts = append(byID[artifact.ApplicationRunID].Artifacts, artifact)
	}
	return items, total, nil
}

func (s *applicationPlatformStore) GetApplicationRunsByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.ApplicationRun, error) {
	if len(ids) == 0 {
		return []*iapiserver.ApplicationRun{}, nil
	}
	var items []*iapiserver.ApplicationRun
	if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ?", ids, ownerUserID).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *applicationPlatformStore) GetApplicationRunByIdempotency(ctx context.Context, owner, key string) (*iapiserver.ApplicationRun, error) {
	var item iapiserver.ApplicationRun
	if err := s.ds.db.WithContext(ctx).First(&item, "owner_user_id = ? AND idempotency_key = ?", owner, key).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddApplicationRun(ctx context.Context, data *iapiserver.ApplicationRun) (*iapiserver.ApplicationRun, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		return publishOutbox(tx, OutboxTopicApplicationRunCreated, data.ID+":created", applicationRunCreatedPayload(data))
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) BindApplicationRunTask(ctx context.Context, id, atomicTaskID, status, taskStatus string, taskVersion int64, failure string) (*iapiserver.ApplicationRun, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current iapiserver.ApplicationRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", id).Error; err != nil {
			return err
		}
		if current.AtomicTaskID != nil && *current.AtomicTaskID != "" && atomicTaskID != "" && *current.AtomicTaskID != atomicTaskID {
			return errors.NewStatus(code.ErrAIAppApplicationRunCreateFailed, "application run is already bound to another atomic task")
		}
		values := map[string]any{"task_creation_status": status, "task_creation_failure": failure, "resource_version": gorm.Expr("resource_version + 1")}
		newBinding := atomicTaskID != "" && (current.AtomicTaskID == nil || *current.AtomicTaskID == "")
		if atomicTaskID != "" {
			values["atomic_task_id"] = atomicTaskID
			values["task_status_projection"] = taskStatus
			values["task_resource_version"] = taskVersion
		}
		if err := tx.Model(&current).Updates(values).Error; err != nil {
			return err
		}
		if !newBinding {
			return nil
		}
		payload := map[string]any{
			"application_run_id": current.ID, "atomic_task_id": atomicTaskID,
			"task_creation_status": status, "task_resource_version": taskVersion,
			"occurred_at": imachinery.Now(),
		}
		return publishOutbox(tx, OutboxTopicApplicationRunAtomicTaskBound, current.ID+":"+atomicTaskID, payload)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetApplicationRun(ctx, id)
}

func applicationRunCreatedPayload(run *iapiserver.ApplicationRun) map[string]any {
	originType := snapshotString(run.ExecutionSnapshot, "origin_type")
	if originType == "" {
		originType = "api"
	}
	return map[string]any{
		"application_run_id": run.ID, "application_id": run.ApplicationID,
		"application_version_id":          run.ApplicationVersionID,
		"application_template_version_id": run.ApplicationTemplateVersionID,
		"engine_instance_id":              run.EngineInstanceID, "capability_source_type": run.CapabilitySourceType,
		"source_revision": run.SourceRevision, "provider_capability_id": run.ProviderCapabilityID,
		"provider_capability_revision": run.ProviderCapabilityRevision, "provider_operation_id": run.ProviderOperationID,
		"workflow_contract_revision": run.WorkflowContractRevision, "execution_snapshot": run.ExecutionSnapshot,
		"origin_type": originType, "canvas_run_id": snapshotString(run.ExecutionSnapshot, "canvas_run_id"),
		"canvas_node_run_id":   snapshotString(run.ExecutionSnapshot, "canvas_node_run_id"),
		"execution_key":        snapshotString(run.ExecutionSnapshot, "execution_key"),
		"task_creation_status": run.TaskCreationStatus, "occurred_at": imachinery.Now(),
	}
}

func snapshotString(snapshot map[string]any, key string) string {
	value, _ := snapshot[key].(string)
	return value
}

func (s *applicationPlatformStore) ProjectApplicationRun(ctx context.Context, id string, taskVersion int64, status, failure string, outputs []map[string]any) (*iapiserver.ApplicationRun, error) {
	raw, err := jsonString(outputs)
	if err != nil {
		return nil, err
	}
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.ApplicationRun{}).Where("id = ? AND task_resource_version < ?", id, taskVersion).Updates(map[string]any{"task_resource_version": taskVersion, "task_status_projection": status, "failure_summary": failure, "output_values_json": raw, "resource_version": gorm.Expr("resource_version + 1")})
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.NewStatus(code.ErrAIAppTaskProjectionStale, "task projection is stale")
	}
	return s.GetApplicationRun(ctx, id)
}

func (s *applicationPlatformStore) ListArtifactsByRun(ctx context.Context, runID string) ([]*iapiserver.ApplicationArtifact, error) {
	var items []*iapiserver.ApplicationArtifact
	err := s.ds.db.WithContext(ctx).Where("application_run_id = ?", runID).Order("output_key ASC").Find(&items).Error
	return items, errors.WithStack(err)
}

func (s *applicationPlatformStore) ListApplicationArtifactRefsByRun(ctx context.Context, runID string) ([]*iapiserver.ApplicationArtifactRef, error) {
	var items []*iapiserver.ApplicationArtifactRef
	err := s.ds.db.WithContext(ctx).
		Where("application_run_id = ?", runID).
		Order("output_key ASC, sequence ASC, id ASC").
		Find(&items).Error
	return items, errors.WithStack(err)
}

// ProjectApplicationArtifactRef 将 Asset Library 事实按 artifact_resource_version 单调投影。
func (s *applicationPlatformStore) ProjectApplicationArtifactRef(ctx context.Context, data *iapiserver.ApplicationArtifactRef) (*iapiserver.ApplicationArtifactRef, bool, error) {
	if data == nil || data.ApplicationRunID == "" || data.ArtifactID == "" || data.OutputKey == "" || data.Sequence < 0 {
		return nil, false, errors.Errorf("application artifact reference identity is invalid")
	}
	var projected *iapiserver.ApplicationArtifactRef
	applied := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		atomicTaskID, err := applicationRunAtomicTaskID(tx, data.ApplicationRunID)
		if err != nil {
			return err
		}
		var current iapiserver.ApplicationArtifactRef
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("artifact_id = ? OR (application_run_id = ? AND output_key = ? AND sequence = ?)",
				data.ArtifactID, data.ApplicationRunID, data.OutputKey, data.Sequence).
			First(&current).Error
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			data.Name = data.OutputKey
			if err := tx.Create(data).Error; err != nil {
				return err
			}
			if err := publishApplicationArtifactRefChanged(tx, data, atomicTaskID); err != nil {
				return err
			}
			projected, applied = data, true
			return nil
		}
		if err != nil {
			return err
		}
		if current.ArtifactID != data.ArtifactID || current.ApplicationRunID != data.ApplicationRunID ||
			current.OutputKey != data.OutputKey || current.Sequence != data.Sequence {
			return errors.Errorf("application artifact reference conflicts with existing output mapping")
		}
		if current.ArtifactResourceVersion >= data.ArtifactResourceVersion {
			projected = &current
			return nil
		}
		current.MediaType = data.MediaType
		current.ArtifactProcessingStatus = data.ArtifactProcessingStatus
		current.ArtifactRegistrationStatus = data.ArtifactRegistrationStatus
		current.AssetID = data.AssetID
		current.AssetVersionID = data.AssetVersionID
		current.ArtifactResourceVersion = data.ArtifactResourceVersion
		current.LastErrorCode = data.LastErrorCode
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		if err := publishApplicationArtifactRefChanged(tx, &current, atomicTaskID); err != nil {
			return err
		}
		projected, applied = &current, true
		return nil
	})
	return projected, applied, errors.WithStack(err)
}

func applicationRunAtomicTaskID(tx *gorm.DB, runID string) (string, error) {
	var run iapiserver.ApplicationRun
	if err := tx.Select("atomic_task_id").First(&run, "id = ?", runID).Error; err != nil {
		return "", err
	}
	if run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
		return "", errors.Errorf("application run artifact reference requires a bound atomic task")
	}
	return *run.AtomicTaskID, nil
}

func publishApplicationArtifactRefChanged(tx *gorm.DB, ref *iapiserver.ApplicationArtifactRef, atomicTaskID string) error {
	sourceID := fmt.Sprintf("%s:%s:%d", ref.ApplicationRunID, ref.ArtifactID, ref.ArtifactResourceVersion)
	return publishOutbox(tx, OutboxTopicApplicationRunArtifactRefChanged, sourceID, applicationArtifactRefChangedPayload(ref, atomicTaskID, sourceID))
}

func applicationArtifactRefChangedPayload(ref *iapiserver.ApplicationArtifactRef, atomicTaskID, sourceID string) map[string]any {
	return map[string]any{
		"source_event_id": sourceID, "source_domain": iapiserver.SSESourceDomainApplicationPlatform,
		"artifact_id": ref.ArtifactID, "application_run_id": ref.ApplicationRunID, "atomic_task_id": atomicTaskID,
		"output_key": ref.OutputKey, "sequence": ref.Sequence, "media_type": ref.MediaType,
		"artifact_processing_status":   ref.ArtifactProcessingStatus,
		"artifact_registration_status": ref.ArtifactRegistrationStatus,
		"asset_id":                     ref.AssetID, "asset_version_id": ref.AssetVersionID,
		"artifact_resource_version": ref.ArtifactResourceVersion, "occurred_at": ref.UpdatedAt,
	}
}

func (s *applicationPlatformStore) UpsertArtifact(ctx context.Context, data *iapiserver.ApplicationArtifact) (*iapiserver.ApplicationArtifact, error) {
	err := s.ds.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "application_run_id"}, {Name: "output_key"}}, DoNothing: true}).Create(data).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var result iapiserver.ApplicationArtifact
	if err := s.ds.db.WithContext(ctx).First(&result, "application_run_id = ? AND output_key = ?", data.ApplicationRunID, data.OutputKey).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &result, nil
}

func (s *applicationPlatformStore) UpdateArtifactRegistration(ctx context.Context, id, status, assetID, errorCode, failureDetail string, expectedVersion int64) (*iapiserver.ApplicationArtifact, error) {
	values := map[string]any{
		"registration_status":         status,
		"registration_error_code":     errorCode,
		"registration_failure_detail": failureDetail,
		"resource_version":            gorm.Expr("resource_version + 1"),
	}
	if assetID != "" {
		values["asset_id"] = assetID
	} else {
		values["asset_id"] = nil
	}
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.ApplicationArtifact{}).
		Where("id = ? AND resource_version = ?", id, expectedVersion).Updates(values)
	if result.Error != nil {
		return nil, errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.NewStatus(code.ErrAIAppResourceVersionConflict, "artifact resource version conflict")
	}
	var updated iapiserver.ApplicationArtifact
	if err := s.ds.db.WithContext(ctx).First(&updated, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &updated, nil
}

func appQuery(ctx context.Context, db *gorm.DB, params imachinery.BasicQueryParam, filter func(*gorm.DB) *gorm.DB) *gorm.DB {
	return params.ToUnpaginatedQuery(ctx, db, filter)
}

func optimisticUpdate(ctx context.Context, db *gorm.DB, data any, id string, expected int64) error {
	result := db.WithContext(ctx).Model(data).Where("id = ? AND resource_version = ?", id, expected).Select("*").Omit("id", "created_at").Updates(data)
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewStatus(code.ErrAIAppResourceVersionConflict, "resource version changed")
	}
	return nil
}

func jsonString(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return string(data), nil
}

func mapAIAppUniqueError(err error, constraint, message string, businessCode int) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), constraint) || strings.Contains(err.Error(), "duplicate key") {
		return errors.NewStatus(businessCode, message)
	}
	return fmt.Errorf("%s: %w", message, err)
}
