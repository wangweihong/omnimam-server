package postgresql

import (
	"context"
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// CreateArtifact 幂等持久化 asset-library Artifact，并把创建事实与 outbox 放在同一事务。
func (s *assetV1Store) CreateArtifact(ctx context.Context, data *iapiserver.Artifact) (*iapiserver.Artifact, bool, error) {
	if data == nil {
		return nil, false, errors.Errorf("artifact is required")
	}
	if data.ProcessingStatus == "" {
		data.ProcessingStatus = iapiserver.ArtifactProcessingCreated
	}
	if data.RegistrationStatus == "" {
		data.RegistrationStatus = iapiserver.ArtifactRegistrationPending
	}
	if data.OwnerUserID == "" || data.ProducerType == "" || data.ProducerID == "" || data.ProducerIdempotencyKey == "" || data.OutputKey == "" || data.ArtifactType == "" || data.MediaType == "" || data.ProcessingProfileVersion == "" {
		return nil, false, errors.Errorf("artifact identity and routing fields are required")
	}
	if !oneOf(data.ProducerType, "application_run", "canvas_run", "atomic_task", "studio_build") ||
		!oneOf(data.MediaType, "image", "video", "audio", "text", "document", "model_3d", "prompt", "prompt_template", "pdf", "other") ||
		!oneOf(data.SavePolicy, iapiserver.ArtifactSaveTransient, iapiserver.ArtifactSaveManual, iapiserver.ArtifactSaveAutomatic) || data.Sequence < 0 {
		return nil, false, errors.Errorf("artifact producer, media type, save policy, or sequence is invalid")
	}
	if data.ProcessingStatus != iapiserver.ArtifactProcessingCreated || data.RegistrationStatus != iapiserver.ArtifactRegistrationPending {
		return nil, false, errors.Errorf("new artifact must start in created and pending status")
	}
	if data.Name == "" {
		data.Name = data.OutputKey
	}
	var result *iapiserver.Artifact
	inserted := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		create := tx.Clauses(clause.OnConflict{Columns: []clause.Column{
			{Name: "owner_user_id"}, {Name: "producer_type"}, {Name: "producer_idempotency_key"},
		}, DoNothing: true}).Create(data)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			var existing iapiserver.Artifact
			if err := tx.Where("owner_user_id = ? AND producer_type = ? AND producer_idempotency_key = ?", data.OwnerUserID, data.ProducerType, data.ProducerIdempotencyKey).First(&existing).Error; err != nil {
				return err
			}
			if !sameArtifactIdentity(&existing, data) {
				return errors.Errorf("artifact producer idempotency key conflicts with existing fact")
			}
			result = &existing
			return nil
		}
		if err := publishArtifactCreated(tx, data); err != nil {
			return err
		}
		result, inserted = data, true
		return nil
	})
	return result, inserted, errors.WithStack(err)
}

// UpdateArtifactProcessing 只推进 Artifact 的处理维度；事实与可靠事件共同提交或回滚。
func (s *assetV1Store) UpdateArtifactProcessing(ctx context.Context, id, owner string, expectedVersion int64, mutation store.ArtifactProcessingMutation) (*iapiserver.Artifact, error) {
	if !validArtifactProcessingChange(mutation.ChangeType) {
		return nil, errors.Errorf("unsupported artifact processing change type %s", mutation.ChangeType)
	}
	if mutation.ChangeType != "preview_ready" && mutation.ProcessingStatus != mutation.ChangeType {
		return nil, errors.Errorf("artifact processing status does not match change type")
	}
	var result *iapiserver.Artifact
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockArtifact(tx, id, owner, expectedVersion)
		if err != nil {
			return err
		}
		nextStatus := current.ProcessingStatus
		if mutation.ProcessingStatus != "" {
			nextStatus = mutation.ProcessingStatus
		}
		if artifactProcessingUnchanged(current, nextStatus, mutation) {
			result = current
			return nil
		}
		current.ProcessingStatus = nextStatus
		current.PreviewAvailable = mutation.PreviewAvailable
		current.PreviewRef = mutation.PreviewRef
		current.ThumbnailRef = mutation.ThumbnailRef
		current.ProcessingErrorCode = mutation.ProcessingErrorCode
		current.ProcessingErrorDetail = mutation.ProcessingErrorDetail
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		if mutation.Progress != nil {
			current.Metadata["processing_progress"] = *mutation.Progress
		}
		if mutation.ProcessingPhase != "" {
			current.Metadata["processing_phase"] = mutation.ProcessingPhase
		}
		if err := tx.Save(current).Error; err != nil {
			return err
		}
		if err := publishArtifactProcessingChanged(tx, current, mutation); err != nil {
			return err
		}
		result = current
		return nil
	})
	return result, errors.WithStack(err)
}

// UpdateArtifactRegistration 只推进登记维度，不反向修改 AtomicTask 或处理终态。
func (s *assetV1Store) UpdateArtifactRegistration(ctx context.Context, id, owner string, expectedVersion int64, mutation store.ArtifactRegistrationMutation) (*iapiserver.Artifact, error) {
	if !validArtifactRegistrationStatus(mutation.RegistrationStatus) {
		return nil, errors.Errorf("unsupported artifact registration status %s", mutation.RegistrationStatus)
	}
	if mutation.RegistrationStatus == iapiserver.ArtifactRegistrationRegistered && (mutation.AssetID == "" || mutation.AssetVersionID == "") {
		return nil, errors.Errorf("registered artifact requires asset and asset version")
	}
	var result *iapiserver.Artifact
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockArtifact(tx, id, owner, expectedVersion)
		if err != nil {
			return err
		}
		if mutation.RegistrationStatus == iapiserver.ArtifactRegistrationRegistered && current.ProcessingStatus != iapiserver.ArtifactProcessingReady {
			return errors.Errorf("only ready artifact can be registered")
		}
		if artifactRegistrationUnchanged(current, mutation) {
			result = current
			return nil
		}
		current.RegistrationStatus = mutation.RegistrationStatus
		current.AssetID = mutation.AssetID
		current.AssetVersionID = mutation.AssetVersionID
		current.RegistrationErrorCode = mutation.RegistrationErrorCode
		current.RegistrationErrorDetail = mutation.RegistrationErrorDetail
		if err := tx.Save(current).Error; err != nil {
			return err
		}
		if err := publishArtifactRegistrationChanged(tx, current, mutation); err != nil {
			return err
		}
		result = current
		return nil
	})
	return result, errors.WithStack(err)
}

// CreateAssetVersion 幂等持久化 processing 版本并发布 processing_started 源事件。
func (s *assetV1Store) CreateAssetVersion(ctx context.Context, data *iapiserver.AssetVersion, taskGroupID, atomicTaskID string) (*iapiserver.AssetVersion, bool, error) {
	if data == nil {
		return nil, false, errors.Errorf("asset version is required")
	}
	if data.AssetID == "" || data.OwnerUserID == "" || data.VersionNo < 1 || data.ProfileVersion == "" || data.Status != iapiserver.AssetVersionStatusProcessing {
		return nil, false, errors.Errorf("new asset version must have identity, profile, and processing status")
	}
	if !oneOf(data.SourceType, "upload", "artifact", "asset_edit", "asset_conversion", "external_import") {
		return nil, false, errors.Errorf("asset version source type is invalid")
	}
	if data.Name == "" {
		data.Name = fmt.Sprintf("Asset version %d", data.VersionNo)
	}
	if data.ExpectedCount < 0 || data.CompletedCount < 0 || data.FailedCount < 0 || data.CompletedCount+data.FailedCount > data.ExpectedCount {
		return nil, false, errors.Errorf("asset version processing counts are invalid")
	}
	var result *iapiserver.AssetVersion
	inserted := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		create := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "asset_id"}, {Name: "version_no"}}, DoNothing: true}).Create(data)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			var existing iapiserver.AssetVersion
			if err := tx.Where("asset_id = ? AND version_no = ?", data.AssetID, data.VersionNo).First(&existing).Error; err != nil {
				return err
			}
			if existing.OwnerUserID != data.OwnerUserID || existing.SourceType != data.SourceType || existing.SourceRefID != data.SourceRefID {
				return errors.Errorf("asset version number conflicts with existing fact")
			}
			result = &existing
			return nil
		}
		if data.Status == iapiserver.AssetVersionStatusProcessing {
			if err := publishAssetVersionProcessingChanged(tx, data, "started", taskGroupID, atomicTaskID, ""); err != nil {
				return err
			}
		}
		result, inserted = data, true
		return nil
	})
	return result, inserted, errors.WithStack(err)
}

// UpdateAssetVersionProcessing 单调更新 Representation 汇总并发布对应 AssetVersion 用户事件源。
func (s *assetV1Store) UpdateAssetVersionProcessing(ctx context.Context, id, owner string, expectedVersion int64, mutation store.AssetVersionProcessingMutation) (*iapiserver.AssetVersion, error) {
	if !validAssetVersionProcessingStatus(mutation.Status) {
		return nil, errors.Errorf("unsupported asset version status %s", mutation.Status)
	}
	if mutation.ExpectedCount < 0 || mutation.CompletedCount < 0 || mutation.FailedCount < 0 || mutation.CompletedCount+mutation.FailedCount > mutation.ExpectedCount {
		return nil, errors.Errorf("asset version processing counts are invalid")
	}
	var result *iapiserver.AssetVersion
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current iapiserver.AssetVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&current).Error; err != nil {
			return err
		}
		if current.ResourceVersion != expectedVersion {
			return errors.Errorf("asset version resource version conflict")
		}
		if assetVersionProcessingUnchanged(&current, mutation) {
			result = &current
			return nil
		}
		current.Status = mutation.Status
		current.ExpectedCount = mutation.ExpectedCount
		current.CompletedCount = mutation.CompletedCount
		current.FailedCount = mutation.FailedCount
		if mutation.ErrorCode == "" {
			current.ProcessingError = map[string]any{}
		} else {
			current.ProcessingError = map[string]any{"error_code": mutation.ErrorCode}
		}
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		if err := publishAssetVersionProcessingChanged(tx, &current, "progressed", mutation.TaskGroupID, mutation.AtomicTaskID, mutation.ErrorCode); err != nil {
			return err
		}
		result = &current
		return nil
	})
	return result, errors.WithStack(err)
}

func lockArtifact(tx *gorm.DB, id, owner string, expectedVersion int64) (*iapiserver.Artifact, error) {
	var current iapiserver.Artifact
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&current).Error; err != nil {
		return nil, err
	}
	if current.ResourceVersion != expectedVersion {
		return nil, errors.Errorf("artifact resource version conflict")
	}
	return &current, nil
}

func sameArtifactIdentity(left, right *iapiserver.Artifact) bool {
	return left.ProducerID == right.ProducerID && left.OutputKey == right.OutputKey && left.Sequence == right.Sequence &&
		left.ArtifactType == right.ArtifactType && left.MediaType == right.MediaType && left.AtomicTaskID == right.AtomicTaskID &&
		left.ApplicationRunID == right.ApplicationRunID
}

func publishArtifactCreated(tx *gorm.DB, artifact *iapiserver.Artifact) error {
	payload := artifactEventPayload(artifact)
	sourceID := fmt.Sprintf("%s:%d:created", artifact.ID, artifact.ResourceVersion)
	payload["source_event_id"], payload["source_domain"] = sourceID, iapiserver.SSESourceDomainAssetLibrary
	return publishOutbox(tx, OutboxTopicArtifactCreated, sourceID, payload)
}

func publishArtifactProcessingChanged(tx *gorm.DB, artifact *iapiserver.Artifact, mutation store.ArtifactProcessingMutation) error {
	payload := artifactEventPayload(artifact)
	payload["change_type"], payload["progress"], payload["processing_phase"] = mutation.ChangeType, mutation.Progress, nullableString(mutation.ProcessingPhase)
	payload["error_code"], payload["retryable"] = nullableString(mutation.ProcessingErrorCode), mutation.Retryable
	payload["ready_at"] = nullableTimePointer(mutation.ReadyAt)
	sourceID := fmt.Sprintf("%s:%d:%s", artifact.ID, artifact.ResourceVersion, mutation.ChangeType)
	payload["source_event_id"], payload["source_domain"] = sourceID, iapiserver.SSESourceDomainAssetLibrary
	return publishOutbox(tx, OutboxTopicArtifactProcessingChanged, sourceID, payload)
}

func publishArtifactRegistrationChanged(tx *gorm.DB, artifact *iapiserver.Artifact, mutation store.ArtifactRegistrationMutation) error {
	payload := artifactEventPayload(artifact)
	payload["registration_result"] = nullableString(mutation.RegistrationResult)
	payload["error_code"], payload["retryable"] = nullableString(mutation.RegistrationErrorCode), mutation.Retryable
	sourceID := fmt.Sprintf("%s:%d:%s", artifact.ID, artifact.ResourceVersion, mutation.RegistrationStatus)
	payload["source_event_id"], payload["source_domain"] = sourceID, iapiserver.SSESourceDomainAssetLibrary
	return publishOutbox(tx, OutboxTopicArtifactRegistrationChanged, sourceID, payload)
}

func artifactEventPayload(artifact *iapiserver.Artifact) map[string]any {
	return map[string]any{
		"artifact_id": artifact.ID, "owner_user_id": artifact.OwnerUserID, "producer_type": artifact.ProducerType,
		"producer_id": artifact.ProducerID, "atomic_task_id": nullableString(artifact.AtomicTaskID),
		"application_run_id": nullableString(artifact.ApplicationRunID), "output_key": artifact.OutputKey,
		"sequence": artifact.Sequence, "artifact_type": artifact.ArtifactType, "media_type": artifact.MediaType,
		"processing_status": artifact.ProcessingStatus, "registration_status": artifact.RegistrationStatus,
		"preview_available": artifact.PreviewAvailable, "preview_ref": nullableString(artifact.PreviewRef),
		"thumbnail_ref": nullableString(artifact.ThumbnailRef), "size_bytes": metadataValue(artifact.Metadata, "size_bytes"),
		"processing_progress": metadataValue(artifact.Metadata, "processing_progress"), "processing_phase": metadataValue(artifact.Metadata, "processing_phase"),
		"asset_id": nullableString(artifact.AssetID), "asset_version_id": nullableString(artifact.AssetVersionID),
		"processing_error_code":   nullableString(artifact.ProcessingErrorCode),
		"processing_retryable":    nil,
		"registration_error_code": nullableString(artifact.RegistrationErrorCode),
		"registration_retryable":  nil,
		"ready_at":                nullableTimePointer(artifact.ReadyAt),
		"resource_version":        artifact.ResourceVersion, "occurred_at": artifact.UpdatedAt,
	}
}

func publishAssetVersionProcessingChanged(tx *gorm.DB, version *iapiserver.AssetVersion, changeType, taskGroupID, atomicTaskID, errorCode string) error {
	payload := map[string]any{
		"asset_id": version.AssetID, "asset_version_id": version.ID, "owner_user_id": version.OwnerUserID,
		"status": version.Status, "change_type": changeType, "expected_count": version.ExpectedCount,
		"completed_count": version.CompletedCount, "failed_count": version.FailedCount,
		"task_group_id": nullableString(taskGroupID), "atomic_task_id": nullableString(atomicTaskID),
		"error_code": nullableString(errorCode), "resource_version": version.ResourceVersion, "occurred_at": version.UpdatedAt,
	}
	sourceID := fmt.Sprintf("%s:%d:%s", version.ID, version.ResourceVersion, version.Status)
	payload["source_event_id"], payload["source_domain"] = sourceID, iapiserver.SSESourceDomainAssetLibrary
	return publishOutbox(tx, OutboxTopicAssetVersionProcessingChanged, sourceID, payload)
}

func metadataValue(metadata map[string]any, key string) any {
	if value, ok := metadata[key]; ok {
		return value
	}
	return nil
}

func artifactProcessingUnchanged(current *iapiserver.Artifact, nextStatus string, mutation store.ArtifactProcessingMutation) bool {
	progressUnchanged := mutation.Progress == nil || fmt.Sprint(metadataValue(current.Metadata, "processing_progress")) == fmt.Sprint(*mutation.Progress)
	phaseUnchanged := mutation.ProcessingPhase == "" || fmt.Sprint(metadataValue(current.Metadata, "processing_phase")) == mutation.ProcessingPhase
	return current.ProcessingStatus == nextStatus && current.PreviewAvailable == mutation.PreviewAvailable &&
		current.PreviewRef == mutation.PreviewRef && current.ThumbnailRef == mutation.ThumbnailRef &&
		current.ProcessingErrorCode == mutation.ProcessingErrorCode && current.ProcessingErrorDetail == mutation.ProcessingErrorDetail &&
		progressUnchanged && phaseUnchanged
}

func artifactRegistrationUnchanged(current *iapiserver.Artifact, mutation store.ArtifactRegistrationMutation) bool {
	return current.RegistrationStatus == mutation.RegistrationStatus && current.AssetID == mutation.AssetID &&
		current.AssetVersionID == mutation.AssetVersionID && current.RegistrationErrorCode == mutation.RegistrationErrorCode &&
		current.RegistrationErrorDetail == mutation.RegistrationErrorDetail
}

func assetVersionProcessingUnchanged(current *iapiserver.AssetVersion, mutation store.AssetVersionProcessingMutation) bool {
	currentErrorCode, _ := current.ProcessingError["error_code"].(string)
	return current.Status == mutation.Status && current.ExpectedCount == mutation.ExpectedCount &&
		current.CompletedCount == mutation.CompletedCount && current.FailedCount == mutation.FailedCount &&
		currentErrorCode == mutation.ErrorCode
}

func nullableTimePointer(value *imachinery.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value
}

func validArtifactProcessingChange(changeType string) bool {
	return oneOf(changeType, "transferring", "processing", "preview_ready", "ready", "failed", "deleted")
}

func validArtifactRegistrationStatus(status string) bool {
	return oneOf(status, "registered", "failed")
}

func validAssetVersionProcessingStatus(status string) bool {
	return oneOf(status, iapiserver.AssetVersionStatusProcessing, iapiserver.AssetVersionStatusReady, iapiserver.AssetVersionStatusReadyWithWarnings, iapiserver.AssetVersionStatusFailed)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
