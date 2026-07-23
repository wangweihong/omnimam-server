package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func (s *assetV1Store) ListArtifacts(ctx context.Context, owner string, req *iapiserver.ArtifactListRequest) ([]*iapiserver.Artifact, int64, error) {
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.Artifact{}).Where("owner_user_id = ?", owner)
	if req.ProcessingStatus != "" {
		query = query.Where("processing_status = ?", req.ProcessingStatus)
	} else {
		query = query.Where("deleted_at IS NULL")
	}
	if req.RegistrationStatus != "" {
		query = query.Where("registration_status = ?", req.RegistrationStatus)
	}
	if req.ProducerType != "" {
		query = query.Where("producer_type = ?", req.ProducerType)
	}
	query = query.Order("created_at DESC")
	var items []*iapiserver.Artifact
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

// ResolveArtifactSummaries 以固定批次读取 owner 可见 Artifact；缺失、删除或其他 owner 目标均不进入结果 map。
func (s *assetV1Store) ResolveArtifactSummaries(ctx context.Context, owner string, ids []string) (map[string]*iapiserver.ArtifactReadableSummary, error) {
	result := make(map[string]*iapiserver.ArtifactReadableSummary)
	if len(ids) == 0 {
		return result, nil
	}
	var artifacts []*iapiserver.Artifact
	if err := s.ds.db.WithContext(ctx).
		Where("id IN ? AND owner_user_id = ? AND deleted_at IS NULL", ids, owner).
		Find(&artifacts).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.DecorateArtifacts(ctx, owner, artifacts); err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		var assetID *string
		if artifact.AssetID != "" {
			value := artifact.AssetID
			assetID = &value
		}
		result[artifact.ID] = &iapiserver.ArtifactReadableSummary{
			ID: artifact.ID, OutputKey: artifact.OutputKey, ArtifactType: artifact.ArtifactType,
			MediaType: artifact.MediaType, ProcessingStatus: artifact.ProcessingStatus,
			RegistrationStatus: artifact.RegistrationStatus, PreviewAvailable: artifact.PreviewAvailable,
			AssetID: assetID, Asset: artifact.Asset,
		}
	}
	return result, nil
}

func (s *assetV1Store) DecorateArtifacts(ctx context.Context, owner string, items []*iapiserver.Artifact) error {
	assetIDs := make([]string, 0, len(items))
	versionIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		item.Asset, item.AssetVersion = nil, nil
		if item.AssetID != "" {
			assetIDs = append(assetIDs, item.AssetID)
		}
		if item.AssetVersionID != "" {
			versionIDs = append(versionIDs, item.AssetVersionID)
		}
	}
	assetsByID := make(map[string]*iapiserver.UserAssetSummary)
	if len(assetIDs) > 0 {
		var assets []*iapiserver.UserAsset
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ?", assetIDs, owner).Find(&assets).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, asset := range assets {
			assetsByID[asset.ID] = userAssetSummary(asset)
		}
	}
	versionsByID := make(map[string]*iapiserver.AssetVersionSummary)
	if len(versionIDs) > 0 {
		var versions []*iapiserver.AssetVersion
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ?", versionIDs, owner).Find(&versions).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, version := range versions {
			versionsByID[version.ID] = assetVersionSummary(version)
		}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.Asset = assetsByID[item.AssetID]
		item.AssetVersion = versionsByID[item.AssetVersionID]
	}
	return nil
}

func (s *assetV1Store) GetArtifact(ctx context.Context, owner, id string) (*iapiserver.Artifact, error) {
	var artifact iapiserver.Artifact
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&artifact).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &artifact, nil
}

func (s *assetV1Store) StoreArtifactContent(ctx context.Context, owner, id string, content store.StoredAssetContent) (*iapiserver.Artifact, error) {
	var artifact iapiserver.Artifact
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&artifact).Error; err != nil {
			return err
		}
		if artifact.BlobID != "" {
			var blob iapiserver.AssetBlob
			if err := tx.Where("id = ?", artifact.BlobID).First(&blob).Error; err != nil {
				return err
			}
			if !strings.EqualFold(blob.SHA256, content.SHA256) || blob.SizeBytes != content.SizeBytes {
				return errors.Errorf("artifact content conflicts with existing blob")
			}
			return nil
		}
		if artifact.ProcessingStatus != iapiserver.ArtifactProcessingCreated && artifact.ProcessingStatus != iapiserver.ArtifactProcessingTransferring {
			return errors.Errorf("artifact state does not allow content upload")
		}
		blob := &iapiserver.AssetBlob{StorageBackendID: content.StorageBackendID, ObjectKey: content.ObjectKey, SHA256: strings.ToLower(content.SHA256), SizeBytes: content.SizeBytes, MIMEType: content.MIMEType, Status: "available"}
		blob.ID, blob.Name = uuid.NewString(), artifact.OutputKey
		if err := tx.Create(blob).Error; err != nil {
			return err
		}
		artifact.BlobID, artifact.ProcessingStatus = blob.ID, iapiserver.ArtifactProcessingTransferring
		if err := tx.Save(&artifact).Error; err != nil {
			return err
		}
		return publishArtifactProcessingChanged(tx, &artifact, store.ArtifactProcessingMutation{ChangeType: "transferring", ProcessingStatus: "transferring"})
	})
	return &artifact, errors.WithStack(err)
}

func (s *assetV1Store) CompleteArtifact(ctx context.Context, owner, id string, req *iapiserver.CompleteArtifactRequest) (*iapiserver.Artifact, error) {
	var artifact iapiserver.Artifact
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&artifact).Error; err != nil {
			return err
		}
		if artifact.ProcessingStatus == iapiserver.ArtifactProcessingProcessing || artifact.ProcessingStatus == iapiserver.ArtifactProcessingReady {
			return nil
		}
		if artifact.ProcessingStatus != iapiserver.ArtifactProcessingTransferring || artifact.BlobID == "" {
			return errors.Errorf("artifact state does not allow completion")
		}
		var blob iapiserver.AssetBlob
		if err := tx.Where("id = ?", artifact.BlobID).First(&blob).Error; err != nil {
			return err
		}
		if !strings.EqualFold(blob.SHA256, req.SHA256) || blob.SizeBytes != req.SizeBytes || blob.MIMEType != req.MIMEType || artifact.ProcessingProfileVersion != req.ProcessingProfileVersion {
			return errors.Errorf("artifact completion metadata mismatch")
		}
		artifact.ProcessingStatus = iapiserver.ArtifactProcessingProcessing
		if artifact.Metadata == nil {
			artifact.Metadata = map[string]any{}
		}
		for key, value := range req.Metadata {
			artifact.Metadata[key] = value
		}
		artifact.Metadata["sha256"], artifact.Metadata["size_bytes"], artifact.Metadata["mime_type"] = blob.SHA256, blob.SizeBytes, blob.MIMEType
		if err := tx.Save(&artifact).Error; err != nil {
			return err
		}
		if err := publishArtifactProcessingChanged(tx, &artifact, store.ArtifactProcessingMutation{ChangeType: "processing", ProcessingStatus: "processing"}); err != nil {
			return err
		}
		sourceID := fmt.Sprintf("%s:%s:content_completed", artifact.ID, artifact.ProcessingProfileVersion)
		return publishOutbox(tx, OutboxTopicArtifactContentCompleted, sourceID, map[string]any{"source_event_id": sourceID, "source_domain": iapiserver.SSESourceDomainAssetLibrary, "artifact_id": artifact.ID, "owner_user_id": owner, "processing_profile_version": artifact.ProcessingProfileVersion, "occurred_at": artifact.UpdatedAt})
	})
	return &artifact, errors.WithStack(err)
}

func (s *assetV1Store) DeleteArtifact(ctx context.Context, owner, id string) (*iapiserver.Artifact, error) {
	var artifact iapiserver.Artifact
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&artifact).Error; err != nil {
			return err
		}
		if artifact.ProcessingStatus == iapiserver.ArtifactProcessingDeleted {
			return nil
		}
		artifact.ProcessingStatus = iapiserver.ArtifactProcessingDeleted
		now := imachinery.NewTime(time.Now())
		artifact.DeletedAt = &now
		if err := tx.Save(&artifact).Error; err != nil {
			return err
		}
		return publishArtifactProcessingChanged(tx, &artifact, store.ArtifactProcessingMutation{ChangeType: "deleted", ProcessingStatus: "deleted"})
	})
	return &artifact, errors.WithStack(err)
}

func (s *assetV1Store) RegisterArtifactLifecycle(ctx context.Context, owner, id string, req *iapiserver.RegisterArtifactRequest) (*iapiserver.ArtifactRegistrationResponse, error) {
	return s.registerArtifactLifecycle(ctx, owner, id, req, store.RepresentationPlan{})
}

func (s *assetV1Store) RegisterArtifactLifecycleWithPlan(ctx context.Context, owner, id string, req *iapiserver.RegisterArtifactRequest, plan store.RepresentationPlan) (*iapiserver.ArtifactRegistrationResponse, error) {
	return s.registerArtifactLifecycle(ctx, owner, id, req, plan)
}

func (s *assetV1Store) registerArtifactLifecycle(ctx context.Context, owner, id string, req *iapiserver.RegisterArtifactRequest, plan store.RepresentationPlan) (*iapiserver.ArtifactRegistrationResponse, error) {
	var response *iapiserver.ArtifactRegistrationResponse
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var artifact iapiserver.Artifact
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&artifact).Error; err != nil {
			return err
		}
		var existing iapiserver.ArtifactAssetRegistration
		err := tx.Where("artifact_id = ?", id).First(&existing).Error
		if err == nil {
			return loadArtifactRegistrationResponseTx(tx, &artifact, &existing, "already_registered", &response)
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if artifact.ProcessingStatus != iapiserver.ArtifactProcessingReady || artifact.BlobID == "" {
			return errors.Errorf("only ready artifact with content can be registered")
		}
		profile := req.ProfileVersion
		if profile == "" {
			profile = artifact.ProcessingProfileVersion
		}
		resolvedPlan, err := validateRepresentationPlan(plan, artifact.MediaType, profile)
		if err != nil {
			return err
		}
		var asset *iapiserver.UserAsset
		if req.Mode == "append_version" {
			if req.AssetID == "" {
				return errors.Errorf("append version requires asset id")
			}
			var current iapiserver.UserAsset
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status <> ?", req.AssetID, owner, iapiserver.AssetStatusDeleted).First(&current).Error; err != nil {
				return err
			}
			asset = &current
		} else {
			name := strings.TrimSpace(req.Name)
			if name == "" {
				name = artifact.OutputKey
			}
			sourceType := "application_output"
			if artifact.ProducerType == "canvas_run" {
				sourceType = "canvas_output"
			}
			asset = &iapiserver.UserAsset{OwnerUserID: owner, DisplayName: name, MediaType: artifact.MediaType, SourceType: sourceType,
				Status: iapiserver.AssetStatusActive, Labels: map[string]string{}, Tags: []string{}}
			asset.ThumbnailStatus, asset.PreviewStatus = initialRepresentationStatusesForPlan(resolvedPlan)
			asset.ID, asset.Name = uuid.NewString(), name
			if value, ok := artifact.Metadata["size_bytes"].(float64); ok {
				asset.SizeBytes = int64(value)
			}
			if err := tx.Create(asset).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&iapiserver.AssetVersion{}).Where("asset_id = ?", asset.ID).Count(&count).Error; err != nil {
			return err
		}
		version := &iapiserver.AssetVersion{AssetID: asset.ID, OwnerUserID: owner, VersionNo: int(count) + 1, Status: iapiserver.AssetVersionStatusProcessing,
			SourceType: "artifact", SourceRefID: artifact.ID, Content: map[string]any{}, Metadata: artifact.Metadata, VersionNote: req.VersionNote, ProfileVersion: profile, ExpectedCount: resolvedPlan.ExpectedCount}
		version.ID, version.Name = uuid.NewString(), fmtVersionName(int(count)+1)
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		representation := &iapiserver.AssetRepresentation{AssetVersionID: version.ID, OwnerUserID: owner, RepresentationType: iapiserver.AssetRepresentationOriginal,
			Profile: "default", ProfileVersion: profile, BlobID: artifact.BlobID, Content: map[string]any{}, Metadata: artifact.Metadata, Status: "ready", Required: true}
		representation.ID, representation.Name = uuid.NewString(), "original"
		if err := tx.Create(representation).Error; err != nil {
			return err
		}
		asset.CurrentVersionID = version.ID
		if value, ok := artifact.Metadata["sha256"].(string); ok {
			asset.SHA256 = value
		}
		if err := tx.Save(asset).Error; err != nil {
			return err
		}
		registration := &iapiserver.ArtifactAssetRegistration{ArtifactID: artifact.ID, ApplicationRunID: artifact.ApplicationRunID,
			OwnerUserID: owner, AssetID: asset.ID, AssetVersionID: version.ID, RegistrationMode: req.Mode, RegistrationResult: "created", MediaType: artifact.MediaType}
		registration.ID, registration.Name = uuid.NewString(), "Artifact registration"
		if err := tx.Create(registration).Error; err != nil {
			return err
		}
		artifact.RegistrationStatus, artifact.AssetID, artifact.AssetVersionID = iapiserver.ArtifactRegistrationRegistered, asset.ID, version.ID
		if err := tx.Save(&artifact).Error; err != nil {
			return err
		}
		if err := publishArtifactRegistrationChanged(tx, &artifact, store.ArtifactRegistrationMutation{RegistrationStatus: iapiserver.ArtifactRegistrationRegistered, RegistrationResult: "created", AssetID: asset.ID, AssetVersionID: version.ID}); err != nil {
			return err
		}
		if err := publishRepresentationRequestedWithPlan(tx, version, resolvedPlan); err != nil {
			return err
		}
		response = &iapiserver.ArtifactRegistrationResponse{ArtifactID: artifact.ID, RegistrationResult: "created", Asset: asset, AssetVersion: version}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if response != nil && response.Asset != nil {
		if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{response.Asset}); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *assetV1Store) ListAssetVersions(ctx context.Context, owner, assetID string) ([]*iapiserver.AssetVersion, error) {
	if _, err := s.GetUserAsset(ctx, owner, assetID, false); err != nil {
		return nil, err
	}
	var items []*iapiserver.AssetVersion
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ? AND owner_user_id = ? AND deleted_at IS NULL", assetID, owner).Order("version_no DESC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *assetV1Store) CreateCanonicalVersion(ctx context.Context, owner, assetID string, req *iapiserver.CreateCanonicalVersionRequest) (*iapiserver.AssetVersion, error) {
	var version *iapiserver.AssetVersion
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset iapiserver.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status <> ?", assetID, owner, iapiserver.AssetStatusDeleted).First(&asset).Error; err != nil {
			return err
		}
		if !oneOf(asset.MediaType, "text", "prompt", "prompt_template") {
			return errors.Errorf("asset does not accept canonical versions")
		}
		var count int64
		if err := tx.Model(&iapiserver.AssetVersion{}).Where("asset_id = ?", assetID).Count(&count).Error; err != nil {
			return err
		}
		version = &iapiserver.AssetVersion{AssetID: assetID, OwnerUserID: owner, VersionNo: int(count) + 1, Status: iapiserver.AssetVersionStatusReady,
			SourceType: "asset_edit", Content: req.CanonicalContent, Metadata: map[string]any{}, VersionNote: req.VersionNote,
			ProfileVersion: req.ProfileVersion, ExpectedCount: 1, CompletedCount: 1}
		version.ID, version.Name = uuid.NewString(), fmtVersionName(int(count)+1)
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		rep := &iapiserver.AssetRepresentation{AssetVersionID: version.ID, OwnerUserID: owner, RepresentationType: iapiserver.AssetRepresentationCanonical,
			Profile: "default", ProfileVersion: req.ProfileVersion, Content: req.CanonicalContent, Metadata: map[string]any{}, Status: "ready", Required: true}
		rep.ID, rep.Name = uuid.NewString(), "canonical"
		if err := tx.Create(rep).Error; err != nil {
			return err
		}
		asset.CurrentVersionID = version.ID
		if err := tx.Save(&asset).Error; err != nil {
			return err
		}
		return publishRepresentationRequested(tx, version)
	})
	return version, errors.WithStack(err)
}

func (s *assetV1Store) GetAssetVersionDetail(ctx context.Context, owner, id string) (*iapiserver.AssetVersionDetail, error) {
	var version iapiserver.AssetVersion
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&version).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	representations, err := s.ListRepresentations(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	return &iapiserver.AssetVersionDetail{Version: &version, Representations: representations}, nil
}

func (s *assetV1Store) SetCurrentAssetVersion(ctx context.Context, owner, assetID, versionID string) (*iapiserver.UserAsset, error) {
	var asset iapiserver.UserAsset
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status <> ?", assetID, owner, iapiserver.AssetStatusDeleted).First(&asset).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&iapiserver.AssetVersion{}).Where("id = ? AND asset_id = ? AND owner_user_id = ?", versionID, assetID, owner).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		asset.CurrentVersionID = versionID
		return tx.Save(&asset).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{&asset}); err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *assetV1Store) ListRepresentations(ctx context.Context, owner, versionID string) ([]*iapiserver.AssetRepresentation, error) {
	var versionCount int64
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.AssetVersion{}).Where("id = ? AND owner_user_id = ?", versionID, owner).Count(&versionCount).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if versionCount == 0 {
		return nil, errors.WithStack(gorm.ErrRecordNotFound)
	}
	var items []*iapiserver.AssetRepresentation
	if err := s.ds.db.WithContext(ctx).Where("asset_version_id = ? AND owner_user_id = ? AND deleted_at IS NULL", versionID, owner).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateRepresentations(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *assetV1Store) RegisterRepresentation(ctx context.Context, owner, versionID string, req *iapiserver.RegisterRepresentationRequest) (*iapiserver.AssetRepresentation, error) {
	var result *iapiserver.AssetRepresentation
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var version iapiserver.AssetVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", versionID, owner).First(&version).Error; err != nil {
			return err
		}
		var existing iapiserver.AssetRepresentation
		err := tx.Where("asset_version_id = ? AND representation_type = ? AND profile = ? AND profile_version = ?", versionID, req.RepresentationType, req.Profile, req.ProfileVersion).First(&existing).Error
		if err == nil {
			if existing.BlobID != req.BlobID || existing.Status != req.Status {
				return errors.Errorf("representation write conflict")
			}
			result = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if req.BlobID != "" {
			var count int64
			if err := tx.Model(&iapiserver.AssetBlob{}).Where("id = ?", req.BlobID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		item := &iapiserver.AssetRepresentation{AssetVersionID: versionID, OwnerUserID: owner, RepresentationType: req.RepresentationType,
			Profile: req.Profile, ProfileVersion: req.ProfileVersion, BlobID: req.BlobID, Content: req.Content, Metadata: req.Metadata,
			Status: req.Status, Required: req.Required, ErrorCode: req.ErrorCode, ErrorDetail: req.ErrorDetail}
		item.ID, item.Name = uuid.NewString(), req.RepresentationType+":"+req.Profile
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		var completed, failed int64
		if err := tx.Model(&iapiserver.AssetRepresentation{}).Where("asset_version_id = ? AND status = 'ready'", versionID).Count(&completed).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.AssetRepresentation{}).Where("asset_version_id = ? AND status IN ?", versionID, []string{"failed", "irreparable"}).Count(&failed).Error; err != nil {
			return err
		}
		version.CompletedCount, version.FailedCount = int(completed), int(failed)
		if req.Required && req.Status != "ready" {
			version.Status = iapiserver.AssetVersionStatusFailed
		} else if failed > 0 {
			version.Status = iapiserver.AssetVersionStatusReadyWithWarnings
		} else if completed >= int64(version.ExpectedCount) {
			version.Status = iapiserver.AssetVersionStatusReady
		}
		if err := tx.Save(&version).Error; err != nil {
			return err
		}
		if req.RepresentationType == "thumbnail" {
			thumbnailStatus := "failed"
			if req.Status == "ready" {
				thumbnailStatus = "ready"
			}
			if err := tx.Model(&iapiserver.UserAsset{}).Where("id = ? AND owner_user_id = ? AND current_version_id = ?", version.AssetID, owner, version.ID).Update("thumbnail_status", thumbnailStatus).Error; err != nil {
				return err
			}
		}
		if err := publishAssetVersionProcessingChanged(tx, &version, "progressed", "", "", req.ErrorCode); err != nil {
			return err
		}
		result = item
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateRepresentations(ctx, []*iapiserver.AssetRepresentation{result}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *assetV1Store) CompleteRepresentationGeneration(ctx context.Context, owner, versionID string, mutation store.RepresentationGenerationMutation) (*iapiserver.AssetRepresentation, error) {
	var result *iapiserver.AssetRepresentation
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if mutation.Type == "" || mutation.Profile == "" || mutation.ProfileVersion == "" || !oneOf(mutation.Status, "ready", "failed", "irreparable") {
			return errors.Errorf("representation generation mutation is invalid")
		}
		var version iapiserver.AssetVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", versionID, owner).First(&version).Error; err != nil {
			return err
		}
		if mutation.BlobID != "" {
			var count int64
			if err := tx.Model(&iapiserver.AssetBlob{}).Where("id = ? AND status = 'available'", mutation.BlobID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		var item iapiserver.AssetRepresentation
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("asset_version_id = ? AND representation_type = ? AND profile = ? AND profile_version = ? AND deleted_at IS NULL", versionID, mutation.Type, mutation.Profile, mutation.ProfileVersion).First(&item).Error
		if findErr == nil && item.Status == "ready" {
			var existingBlobAvailable int64
			if item.BlobID != "" {
				if err := tx.Model(&iapiserver.AssetBlob{}).Where("id = ? AND status = 'available'", item.BlobID).Count(&existingBlobAvailable).Error; err != nil {
					return err
				}
			}
			if existingBlobAvailable > 0 && mutation.Status == "ready" && item.BlobID == mutation.BlobID {
				result = &item
				return nil
			}
			if existingBlobAvailable > 0 {
				return errors.Errorf("representation write conflict")
			}
		}
		if findErr != nil && !stderrors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if stderrors.Is(findErr, gorm.ErrRecordNotFound) {
			item = iapiserver.AssetRepresentation{AssetVersionID: versionID, OwnerUserID: owner, RepresentationType: mutation.Type, Profile: mutation.Profile, ProfileVersion: mutation.ProfileVersion}
			item.ID, item.Name = uuid.NewString(), mutation.Type+":"+mutation.Profile
		}
		item.BlobID, item.Metadata, item.Status, item.Required = mutation.BlobID, mutation.Metadata, mutation.Status, mutation.Required
		item.RetryCount, item.RetryAfter, item.ErrorCode, item.ErrorDetail = mutation.RetryCount, mutation.RetryAfter, mutation.ErrorCode, mutation.ErrorDetail
		if mutation.Status == "ready" {
			item.RetryAfter, item.ErrorCode, item.ErrorDetail = nil, "", ""
		}
		if stderrors.Is(findErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&item).Error; err != nil {
			return err
		}
		var completed, failed int64
		if err := tx.Model(&iapiserver.AssetRepresentation{}).Where("asset_version_id = ? AND deleted_at IS NULL AND status = 'ready'", versionID).Count(&completed).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.AssetRepresentation{}).Where("asset_version_id = ? AND deleted_at IS NULL AND status IN ?", versionID, []string{"failed", "irreparable"}).Count(&failed).Error; err != nil {
			return err
		}
		version.CompletedCount, version.FailedCount = int(completed), int(failed)
		switch {
		case mutation.Required && mutation.Status != "ready":
			version.Status = iapiserver.AssetVersionStatusFailed
		case failed > 0:
			version.Status = iapiserver.AssetVersionStatusReadyWithWarnings
		case completed >= int64(version.ExpectedCount):
			version.Status = iapiserver.AssetVersionStatusReady
		default:
			version.Status = iapiserver.AssetVersionStatusProcessing
		}
		if err := tx.Save(&version).Error; err != nil {
			return err
		}
		if mutation.Type == "thumbnail" {
			thumbnailStatus := "failed"
			if mutation.Status == "ready" {
				thumbnailStatus = "ready"
			}
			if err := tx.Model(&iapiserver.UserAsset{}).Where("id = ? AND owner_user_id = ? AND current_version_id = ?", version.AssetID, owner, version.ID).Update("thumbnail_status", thumbnailStatus).Error; err != nil {
				return err
			}
		}
		if err := publishAssetVersionProcessingChanged(tx, &version, "progressed", "", "", mutation.ErrorCode); err != nil {
			return err
		}
		result = &item
		return nil
	})
	return result, errors.WithStack(err)
}

// ApplyAssetMediaMetadata 原子更新版本与 original Representation metadata，并保护 current Asset 投影不被旧版本迟到任务覆盖。
func (s *assetV1Store) ApplyAssetMediaMetadata(ctx context.Context, owner, versionID string, mutation store.AssetMediaMetadataMutation) error {
	if owner == "" || versionID == "" || mutation.AssetID == "" || mutation.OriginalRepresentationID == "" ||
		mutation.SizeBytes < 0 || mutation.Width < 0 || mutation.Height < 0 || mutation.DurationSeconds < 0 ||
		math.IsNaN(mutation.DurationSeconds) || math.IsInf(mutation.DurationSeconds, 0) {
		return errors.Errorf("asset media metadata mutation is invalid")
	}
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 统一按 Asset -> Version -> Representation 加锁，避免与切换 current version 的事务形成反向锁序。
		var asset iapiserver.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ?", mutation.AssetID, owner).
			First(&asset).Error; err != nil {
			return err
		}
		var version iapiserver.AssetVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ? AND asset_id = ? AND deleted_at IS NULL", versionID, owner, mutation.AssetID).
			First(&version).Error; err != nil {
			return err
		}
		var original iapiserver.AssetRepresentation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ? AND asset_version_id = ? AND representation_type = ? AND deleted_at IS NULL",
				mutation.OriginalRepresentationID, owner, versionID, iapiserver.AssetRepresentationOriginal).
			First(&original).Error; err != nil {
			return err
		}

		version.Metadata = applyMediaMetadata(version.Metadata, mutation)
		if err := tx.Save(&version).Error; err != nil {
			return err
		}
		original.Metadata = applyMediaMetadata(original.Metadata, mutation)
		if err := tx.Save(&original).Error; err != nil {
			return err
		}

		if asset.CurrentVersionID != versionID {
			return nil
		}
		asset.Width, asset.Height, asset.DurationSeconds = mutation.Width, mutation.Height, mutation.DurationSeconds
		return tx.Save(&asset).Error
	})
	return errors.WithStack(err)
}

func applyMediaMetadata(current map[string]any, mutation store.AssetMediaMetadataMutation) map[string]any {
	result := maputil.Clone(current)
	if result == nil {
		result = make(map[string]any, 5)
	}
	if mutation.MIMEType != "" {
		result["mime_type"] = mutation.MIMEType
	}
	result["size_bytes"] = mutation.SizeBytes
	if mutation.Width > 0 {
		result["width"] = mutation.Width
	}
	if mutation.Height > 0 {
		result["height"] = mutation.Height
	}
	if mutation.DurationSeconds > 0 {
		result["duration_seconds"] = mutation.DurationSeconds
	}
	return result
}

// CreateRepresentationBlob 幂等登记 Representation Worker 已写入受控存储的派生内容。
func (s *assetV1Store) CreateRepresentationBlob(ctx context.Context, content store.StoredAssetContent) (string, error) {
	if content.StorageBackendID == "" || content.ObjectKey == "" || content.SHA256 == "" || content.MIMEType == "" || content.SizeBytes < 0 {
		return "", errors.Errorf("generated representation content reference is invalid")
	}
	var blob iapiserver.AssetBlob
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("storage_backend_id = ? AND object_key = ?", content.StorageBackendID, content.ObjectKey).First(&blob).Error
		if err == nil {
			if !strings.EqualFold(blob.SHA256, content.SHA256) || blob.SizeBytes != content.SizeBytes || blob.MIMEType != content.MIMEType {
				return errors.Errorf("generated representation blob conflicts with existing content")
			}
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		blob = iapiserver.AssetBlob{StorageBackendID: content.StorageBackendID, ObjectKey: content.ObjectKey,
			SHA256: strings.ToLower(content.SHA256), SizeBytes: content.SizeBytes, MIMEType: content.MIMEType, Status: "available"}
		blob.ID, blob.Name = uuid.NewString(), "representation:"+content.SHA256
		return tx.Create(&blob).Error
	})
	if err != nil {
		return "", errors.WithStack(err)
	}
	return blob.ID, nil
}

func (s *assetV1Store) GetRepresentation(ctx context.Context, owner, id string) (*iapiserver.AssetRepresentation, *store.StoredAssetContent, error) {
	var representation iapiserver.AssetRepresentation
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&representation).Error; err != nil {
		return nil, nil, errors.WithStack(err)
	}
	var content *store.StoredAssetContent
	if representation.BlobID != "" {
		var blob iapiserver.AssetBlob
		if err := s.ds.db.WithContext(ctx).Where("id = ? AND status = 'available'", representation.BlobID).First(&blob).Error; err != nil {
			return nil, nil, errors.WithStack(err)
		}
		content = &store.StoredAssetContent{StorageBackendID: blob.StorageBackendID, ObjectKey: blob.ObjectKey, SHA256: blob.SHA256, SizeBytes: blob.SizeBytes, MIMEType: blob.MIMEType, BlobID: blob.ID}
		representation.MIMEType = blob.MIMEType
	}
	if value, ok := representation.Metadata["format"].(string); ok {
		representation.Format = value
	}
	return &representation, content, nil
}

func (s *assetV1Store) ListAssetRelations(ctx context.Context, owner, assetID string, paging imachinery.PagingParams) (*iapiserver.AssetRelationListResponse, error) {
	asset, err := s.GetUserAsset(ctx, owner, assetID, false)
	if err != nil {
		return nil, err
	}
	if !s.ds.db.Migrator().HasTable(&iapiserver.AssetRelation{}) {
		return &iapiserver.AssetRelationListResponse{Total: 0, Items: []iapiserver.AssetRelationView{}}, nil
	}
	var relations []iapiserver.AssetRelation
	if err := s.ds.db.WithContext(ctx).Where("source_asset_id = ? OR target_asset_id = ?", assetID, assetID).Order("created_at DESC").Find(&relations).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	otherIDs := make([]string, 0, len(relations))
	for _, relation := range relations {
		if relation.SourceAssetID != assetID {
			otherIDs = append(otherIDs, relation.SourceAssetID)
		}
		if relation.TargetAssetID != assetID {
			otherIDs = append(otherIDs, relation.TargetAssetID)
		}
	}
	visible := map[string]*iapiserver.UserAssetSummary{assetID: userAssetSummary(asset)}
	if len(otherIDs) > 0 {
		var assets []*iapiserver.UserAsset
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ? AND status <> ?", otherIDs, owner, iapiserver.AssetStatusDeleted).Find(&assets).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		for _, asset := range assets {
			visible[asset.ID] = userAssetSummary(asset)
		}
	}
	items := make([]iapiserver.AssetRelationView, 0, len(relations))
	for _, relation := range relations {
		if visible[relation.SourceAssetID] == nil || visible[relation.TargetAssetID] == nil {
			continue
		}
		view := relationView(relation)
		view.SourceAsset = visible[relation.SourceAssetID]
		view.TargetAsset = visible[relation.TargetAssetID]
		items = append(items, view)
	}
	total := int64(len(items))
	window, err := paging.Normalize()
	if err != nil {
		return nil, err
	}
	items = imachinery.PaginateSlice(items, window)
	return &iapiserver.AssetRelationListResponse{Total: total, Items: items}, nil
}
func (s *assetV1Store) GetAssetLineage(ctx context.Context, owner, assetID string) (*iapiserver.AssetLineage, error) {
	asset, err := s.GetUserAsset(ctx, owner, assetID, false)
	if err != nil {
		return nil, err
	}
	relations, err := s.ListAssetRelations(ctx, owner, assetID, imachinery.PagingParams{PageSize: imachinery.MaxPageSize})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(relations.Items))
	for _, relation := range relations.Items {
		if relation.SourceAssetID != assetID {
			ids = append(ids, relation.SourceAssetID)
		}
		if relation.TargetAssetID != assetID {
			ids = append(ids, relation.TargetAssetID)
		}
	}
	nodes := []*iapiserver.UserAsset{asset}
	if len(ids) > 0 {
		var related []*iapiserver.UserAsset
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ? AND status <> ?", ids, owner, iapiserver.AssetStatusDeleted).Find(&related).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		if err := s.decorateUserAssets(ctx, related); err != nil {
			return nil, err
		}
		nodes = append(nodes, related...)
	}
	return &iapiserver.AssetLineage{AssetID: assetID, Nodes: nodes, Edges: relations.Items}, nil
}
func (s *assetV1Store) ListAssetReferences(ctx context.Context, owner, assetID string, paging imachinery.PagingParams) (*iapiserver.AssetReferenceListResponse, error) {
	asset, err := s.GetUserAsset(ctx, owner, assetID, false)
	if err != nil {
		return nil, err
	}
	items := append([]iapiserver.AssetReference(nil), asset.ReferenceSources...)
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.SourceType+":"+item.SourceID] = true
	}
	var memberships []iapiserver.AssetCollectionItem
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ? AND owner_user_id = ? AND deleted_at IS NULL", assetID, owner).Find(&memberships).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	for _, membership := range memberships {
		key := "collection:" + membership.CollectionID
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, iapiserver.AssetReference{ID: membership.ID, AssetID: assetID, AssetVersionID: membership.PinnedVersionID, SourceType: "collection", SourceID: membership.CollectionID, SourceLabel: membership.Name, VersionPolicy: versionPolicy(membership.PinnedVersionID), CreatedAt: membership.CreatedAt})
	}
	var artifacts []iapiserver.Artifact
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ? AND owner_user_id = ? AND deleted_at IS NULL", assetID, owner).Find(&artifacts).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	for _, artifact := range artifacts {
		sourceType, sourceID := artifact.ProducerType, artifact.ProducerID
		key := sourceType + ":" + sourceID
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, iapiserver.AssetReference{ID: artifact.ID, AssetID: assetID, AssetVersionID: artifact.AssetVersionID, SourceType: sourceType, SourceID: sourceID, SourceLabel: artifact.OutputKey, VersionPolicy: "pinned", CreatedAt: artifact.CreatedAt})
	}
	if s.ds.db.Migrator().HasTable(&iapiserver.ApplicationArtifact{}) {
		var projections []iapiserver.ApplicationArtifact
		if err := s.ds.db.WithContext(ctx).Where("asset_id = ? AND owner_user_id = ?", assetID, owner).Find(&projections).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		for _, projection := range projections {
			key := "application_run:" + projection.ApplicationRunID
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, iapiserver.AssetReference{ID: projection.ID, AssetID: assetID, SourceType: "application_run", SourceID: projection.ApplicationRunID, SourceLabel: projection.OutputKey, VersionPolicy: "pinned", CreatedAt: projection.CreatedAt})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Time.After(items[j].CreatedAt.Time) })
	total := int64(len(items))
	items = pageReferences(items, paging)
	return &iapiserver.AssetReferenceListResponse{Total: total, Items: items}, nil
}

func relationView(relation iapiserver.AssetRelation) iapiserver.AssetRelationView {
	return iapiserver.AssetRelationView{ID: relation.ID, SourceAssetID: relation.SourceAssetID, RelationType: relation.RelationType, TargetAssetID: relation.TargetAssetID, AtomicTaskID: relation.TaskID, Metadata: relation.Params, CreatedAt: relation.CreatedAt}
}
func versionPolicy(versionID string) string {
	if versionID != "" {
		return "pinned"
	}
	return "latest"
}
func (s *assetV1Store) ListAssetUsages(ctx context.Context, owner, assetID string, paging imachinery.PagingParams) (*iapiserver.AssetUsageListResponse, error) {
	refs, err := s.ListAssetReferences(ctx, owner, assetID, paging)
	if err != nil {
		return nil, err
	}
	items := make([]iapiserver.AssetUsage, 0, len(refs.Items))
	for _, ref := range refs.Items {
		items = append(items, iapiserver.AssetUsage{SourceType: ref.SourceType, SourceID: ref.SourceID, SourceLabel: ref.SourceLabel, Location: map[string]any{}})
	}
	return &iapiserver.AssetUsageListResponse{Total: refs.Total, Items: items}, nil
}

func (s *assetV1Store) decorateRepresentations(ctx context.Context, items []*iapiserver.AssetRepresentation) error {
	blobIDs := make([]string, 0)
	byBlob := map[string][]*iapiserver.AssetRepresentation{}
	for _, item := range items {
		if item.BlobID != "" {
			if _, ok := byBlob[item.BlobID]; !ok {
				blobIDs = append(blobIDs, item.BlobID)
			}
			byBlob[item.BlobID] = append(byBlob[item.BlobID], item)
		}
		if value, ok := item.Metadata["format"].(string); ok {
			item.Format = value
		}
		if value, ok := item.Metadata["mime_type"].(string); ok {
			item.MIMEType = value
		}
	}
	if len(blobIDs) == 0 {
		return nil
	}
	var blobs []iapiserver.AssetBlob
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", blobIDs).Find(&blobs).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, blob := range blobs {
		for _, item := range byBlob[blob.ID] {
			item.MIMEType = blob.MIMEType
		}
	}
	return nil
}

func loadArtifactRegistrationResponseTx(tx *gorm.DB, artifact *iapiserver.Artifact, registration *iapiserver.ArtifactAssetRegistration, result string, response **iapiserver.ArtifactRegistrationResponse) error {
	var asset iapiserver.UserAsset
	if err := tx.Where("id = ? AND owner_user_id = ?", registration.AssetID, registration.OwnerUserID).First(&asset).Error; err != nil {
		return err
	}
	var version iapiserver.AssetVersion
	if err := tx.Where("id = ? AND owner_user_id = ?", registration.AssetVersionID, registration.OwnerUserID).First(&version).Error; err != nil {
		return err
	}
	*response = &iapiserver.ArtifactRegistrationResponse{ArtifactID: artifact.ID, RegistrationResult: result, Asset: &asset, AssetVersion: &version}
	return nil
}
func pageReferences(items []iapiserver.AssetReference, paging imachinery.PagingParams) []iapiserver.AssetReference {
	window, err := paging.Normalize()
	if err != nil || window.Offset >= len(items) {
		return []iapiserver.AssetReference{}
	}
	end := window.Offset + window.Limit
	if end > len(items) {
		end = len(items)
	}
	result := append([]iapiserver.AssetReference(nil), items[window.Offset:end]...)
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Time.After(result[j].CreatedAt.Time) })
	return result
}
