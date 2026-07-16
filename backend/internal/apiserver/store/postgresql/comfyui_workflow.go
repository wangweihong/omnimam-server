package postgresql

import (
	"context"
	stderrors "errors"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func (s *applicationPlatformStore) ListComfyUIWorkflows(ctx context.Context, req *iapiserver.ComfyUIWorkflowListRequest) ([]*iapiserver.ComfyUIWorkflow, int64, error) {
	var items []*iapiserver.ComfyUIWorkflow
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.ComfyUIWorkflow{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB {
		if req.OwnerUserID != "" {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.LifecycleStatus != "" {
			q = q.Where("lifecycle_status = ?", req.LifecycleStatus)
		}
		if req.ParseStatus != "" {
			q = q.Where("parse_status = ?", req.ParseStatus)
		}
		if req.LatestValidationStatus != "" {
			q = q.Where("latest_validation_status = ?", req.LatestValidationStatus)
		}
		if req.Converted != nil {
			if *req.Converted {
				q = q.Where("converted_application_template_id IS NOT NULL")
			} else {
				q = q.Where("converted_application_template_id IS NULL")
			}
		}
		if req.SourceEngineInstanceID != "" {
			q = q.Where("source_engine_instance_id = ?", req.SourceEngineInstanceID)
		}
		return q
	})
	total, err := countAndFind(query, req.PageNum, req.PageSize, &items)
	return items, total, err
}

func (s *applicationPlatformStore) GetComfyUIWorkflow(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflow, error) {
	var item iapiserver.ComfyUIWorkflow
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow) (*iapiserver.ComfyUIWorkflow, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *applicationPlatformStore) UpdateComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow, expected int64) (*iapiserver.ComfyUIWorkflow, error) {
	if err := optimisticUpdate(ctx, s.ds.db, data, data.ID, expected); err != nil {
		if errors.ToStatus(err).Code == code.ErrAIAppResourceVersionConflict {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIResourceVersionConflict, "ComfyUI workflow resource version changed")
		}
		return nil, err
	}
	return data, nil
}

func (s *applicationPlatformStore) ListComfyUIWorkflowDuplicateIDs(ctx context.Context, owner, checksum string) ([]string, error) {
	var ids []string
	err := s.ds.db.WithContext(ctx).Model(&iapiserver.ComfyUIWorkflow{}).Where("owner_user_id = ? AND workflow_checksum = ?", owner, checksum).Order("created_at ASC").Pluck("id", &ids).Error
	return ids, errors.WithStack(err)
}

func (s *applicationPlatformStore) ListComfyUIWorkflowValidations(ctx context.Context, req *iapiserver.ComfyUIWorkflowValidationListRequest) ([]*iapiserver.ComfyUIWorkflowValidation, int64, error) {
	var items []*iapiserver.ComfyUIWorkflowValidation
	query := appQuery(ctx, s.ds.db.Model(&iapiserver.ComfyUIWorkflowValidation{}), req.BasicQueryParam, func(q *gorm.DB) *gorm.DB { return q.Where("workflow_id = ?", req.WorkflowID) })
	total, err := countAndFind(query, req.PageNum, req.PageSize, &items)
	return items, total, err
}

func (s *applicationPlatformStore) GetComfyUIWorkflowValidation(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowValidation, error) {
	var item iapiserver.ComfyUIWorkflowValidation
	if err := s.ds.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *applicationPlatformStore) AddComfyUIWorkflowValidation(ctx context.Context, data *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var workflow iapiserver.ComfyUIWorkflow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "lifecycle_status").First(&workflow, "id = ?", data.WorkflowID).Error; err != nil {
			return err
		}
		if workflow.LifecycleStatus != iapiserver.ComfyUIWorkflowActive {
			return errors.NewStatus(code.ErrAIAppComfyUIWorkflowArchived, "workflow is archived")
		}
		if err := tx.Create(data).Error; err != nil {
			return err
		}
		result := tx.Model(&iapiserver.ComfyUIWorkflow{}).Where("id = ?", data.WorkflowID).Updates(map[string]any{"latest_validation_status": data.Status, "resource_version": gorm.Expr("resource_version + 1"), "updated_at": imachinery.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	return data, errors.WithStack(err)
}

func (s *applicationPlatformStore) ConvertComfyUIWorkflow(ctx context.Context, workflowID, owner, actor, key string, template *iapiserver.ApplicationTemplate, version *iapiserver.ApplicationTemplateVersion) (*iapiserver.ComfyUIWorkflowConvertResult, error) {
	result := &iapiserver.ComfyUIWorkflowConvertResult{}
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var workflow iapiserver.ComfyUIWorkflow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&workflow, "id = ? AND owner_user_id = ?", workflowID, owner).Error; err != nil {
			return err
		}
		if workflow.ConvertedApplicationTemplateID != nil {
			if workflow.ConversionIdempotencyKey == nil || *workflow.ConversionIdempotencyKey != key {
				return errors.NewStatus(code.ErrAIAppComfyUIWorkflowAlreadyConverted, "workflow was already converted")
			}
			if err := tx.First(template, "id = ?", *workflow.ConvertedApplicationTemplateID).Error; err != nil {
				return err
			}
			if err := tx.First(version, "id = ?", *workflow.ConvertedTemplateVersionID).Error; err != nil {
				return err
			}
			result = &iapiserver.ComfyUIWorkflowConvertResult{WorkflowID: workflowID, ApplicationTemplate: template, ApplicationTemplateVersion: version, WorkflowContractRevision: *version.WorkflowContractRevision}
			return nil
		}
		if workflow.LifecycleStatus != iapiserver.ComfyUIWorkflowActive {
			return errors.NewStatus(code.ErrAIAppComfyUIWorkflowArchived, "workflow is archived")
		}
		var conflict int64
		if err := tx.Model(&iapiserver.ComfyUIWorkflow{}).Where("owner_user_id = ? AND conversion_idempotency_key = ? AND id <> ?", owner, key, workflowID).Count(&conflict).Error; err != nil {
			return err
		}
		if conflict > 0 {
			return errors.NewStatus(code.ErrAIAppComfyUIConversionIdempotencyConflict, "conversion idempotency key is already used")
		}
		if err := tx.Create(template).Error; err != nil {
			return err
		}
		version.ApplicationTemplateID = template.ID
		version.Version = 1
		if version.Name == "" {
			version.Name = template.Name + " v1"
		}
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		now := imachinery.Now()
		updates := map[string]any{"converted_application_template_id": template.ID, "converted_template_version_id": version.ID, "conversion_idempotency_key": key, "converted_at": now, "converted_by_user_id": actor, "updated_by_user_id": actor, "updated_at": now, "resource_version": gorm.Expr("resource_version + 1")}
		if update := tx.Model(&workflow).Where("id = ? AND converted_application_template_id IS NULL", workflowID).Updates(updates); update.Error != nil {
			return update.Error
		} else if update.RowsAffected != 1 {
			return errors.NewStatus(code.ErrAIAppComfyUIWorkflowAlreadyConverted, "workflow was already converted")
		}
		result = &iapiserver.ComfyUIWorkflowConvertResult{WorkflowID: workflowID, ApplicationTemplate: template, ApplicationTemplateVersion: version, WorkflowContractRevision: *version.WorkflowContractRevision}
		return nil
	})
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	return result, errors.WithStack(err)
}
