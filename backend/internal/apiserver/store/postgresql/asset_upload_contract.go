package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func (s *assetV1Store) CreateAssetUploads(ctx context.Context, owner string, items []iapiserver.AssetUploadItemRequest, chunkThreshold int64) ([]iapiserver.AssetUploadInitResult, error) {
	results := make([]iapiserver.AssetUploadInitResult, 0, len(items))
	for _, item := range items {
		result := iapiserver.AssetUploadInitResult{ClientUploadKey: item.ClientUploadKey}
		var existing iapiserver.UserAsset
		err := s.ds.db.WithContext(ctx).Where("owner_user_id = ? AND sha256 = ? AND status <> ?", owner, strings.ToLower(item.SHA256), iapiserver.AssetStatusDeleted).First(&existing).Error
		if err == nil {
			if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{&existing}); err != nil {
				return nil, err
			}
			result.Deduplicated, result.ExistingAsset = true, &existing
			results = append(results, result)
			continue
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithStack(err)
		}
		mode, chunkSize := "single", int64(0)
		if chunkThreshold > 0 && item.SizeBytes > chunkThreshold {
			mode, chunkSize = "chunked", chunkThreshold
		}
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSuffix(filepath.Base(item.FileName), filepath.Ext(item.FileName))
		}
		profile := item.ProfileVersion
		if profile == "" {
			profile = "default-v1"
		}
		upload := &iapiserver.AssetUploadSession{OwnerUserID: owner, ClientUploadKey: item.ClientUploadKey,
			SHA256: strings.ToLower(item.SHA256), FileName: item.FileName, DisplayName: displayName, MIMEType: item.MIMEType,
			SizeBytes: item.SizeBytes, TargetAssetID: item.TargetAssetID, VersionNote: item.VersionNote, ProfileVersion: profile,
			UploadMode: mode, ChunkSizeBytes: chunkSize, UploadedParts: []iapiserver.UploadedPart{}, Status: "initialized",
			PendingLabels: item.Labels, PendingTags: item.Tags}
		upload.Name = item.ClientUploadKey
		create := s.ds.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "client_upload_key"}}, DoNothing: true}).Create(upload)
		if create.Error != nil {
			return nil, errors.WithStack(create.Error)
		}
		if create.RowsAffected == 0 {
			if err := s.ds.db.WithContext(ctx).Where("owner_user_id = ? AND client_upload_key = ?", owner, item.ClientUploadKey).First(upload).Error; err != nil {
				return nil, errors.WithStack(err)
			}
			if upload.SHA256 != strings.ToLower(item.SHA256) || upload.SizeBytes != item.SizeBytes || upload.FileName != item.FileName {
				return nil, errors.Errorf("upload idempotency conflict")
			}
		}
		result.Upload = upload
		results = append(results, result)
	}
	return results, nil
}

func (s *assetV1Store) GetAssetUpload(ctx context.Context, owner, id string) (*iapiserver.AssetUploadSession, error) {
	var upload iapiserver.AssetUploadSession
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, owner).First(&upload).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &upload, nil
}

func (s *assetV1Store) RecordAssetUploadPart(ctx context.Context, owner, id string, part iapiserver.UploadedPart) (*iapiserver.AssetUploadSession, error) {
	var upload iapiserver.AssetUploadSession
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&upload).Error; err != nil {
			return err
		}
		if upload.Status != "initialized" && upload.Status != "uploading" {
			return errors.Errorf("upload state does not allow content")
		}
		for index, existing := range upload.UploadedParts {
			if existing.PartNumber != part.PartNumber {
				continue
			}
			if existing.SHA256 != part.SHA256 || existing.SizeBytes != part.SizeBytes {
				return errors.Errorf("upload part conflicts with existing content")
			}
			upload.UploadedParts[index] = part
			upload.Status = "uploading"
			return tx.Save(&upload).Error
		}
		upload.UploadedParts = append(upload.UploadedParts, part)
		sort.Slice(upload.UploadedParts, func(i, j int) bool { return upload.UploadedParts[i].PartNumber < upload.UploadedParts[j].PartNumber })
		upload.Status = "uploading"
		return tx.Save(&upload).Error
	})
	return &upload, errors.WithStack(err)
}

func (s *assetV1Store) CompleteAssetUpload(ctx context.Context, owner, id string, req *iapiserver.CompleteAssetUploadRequest, content store.StoredAssetContent) (*iapiserver.CompleteAssetUploadResponse, error) {
	var response *iapiserver.CompleteAssetUploadResponse
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var upload iapiserver.AssetUploadSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&upload).Error; err != nil {
			return err
		}
		if upload.Status == "completed" {
			return s.loadCompletedUploadTx(tx, owner, &upload, &response)
		}
		if upload.Status != "initialized" && upload.Status != "uploading" {
			return errors.Errorf("upload state does not allow completion")
		}
		if !strings.EqualFold(upload.SHA256, req.SHA256) || !strings.EqualFold(content.SHA256, upload.SHA256) || content.SizeBytes != upload.SizeBytes {
			return errors.Errorf("upload checksum or size mismatch")
		}
		if upload.UploadMode == "chunked" {
			if len(req.Parts) == 0 || !sameUploadedParts(upload.UploadedParts, req.Parts) {
				return errors.Errorf("upload parts do not match session")
			}
		}
		blob := &iapiserver.AssetBlob{StorageBackendID: content.StorageBackendID, ObjectKey: content.ObjectKey, SHA256: strings.ToLower(content.SHA256), SizeBytes: content.SizeBytes, MIMEType: content.MIMEType, Status: "available"}
		blob.ID, blob.Name = uuid.NewString(), upload.FileName
		if err := tx.Create(blob).Error; err != nil {
			return err
		}
		asset, err := s.lockOrCreateUploadAsset(tx, owner, &upload)
		if err != nil {
			return err
		}
		var versionCount int64
		if err := tx.Model(&iapiserver.AssetVersion{}).Where("asset_id = ?", asset.ID).Count(&versionCount).Error; err != nil {
			return err
		}
		version := &iapiserver.AssetVersion{AssetID: asset.ID, OwnerUserID: owner, VersionNo: int(versionCount) + 1,
			Status: iapiserver.AssetVersionStatusProcessing, SourceType: "upload", SourceRefID: upload.ID,
			Content: map[string]any{}, Metadata: map[string]any{"mime_type": upload.MIMEType, "size_bytes": upload.SizeBytes},
			VersionNote: upload.VersionNote, ProfileVersion: upload.ProfileVersion, ExpectedCount: 1}
		version.ID, version.Name = uuid.NewString(), fmtVersionName(int(versionCount)+1)
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		representation := &iapiserver.AssetRepresentation{AssetVersionID: version.ID, OwnerUserID: owner,
			RepresentationType: iapiserver.AssetRepresentationOriginal, Profile: "default", ProfileVersion: upload.ProfileVersion,
			BlobID: blob.ID, Content: map[string]any{}, Metadata: map[string]any{"format": asset.Format, "mime_type": upload.MIMEType}, Status: "ready", Required: true}
		representation.ID, representation.Name = uuid.NewString(), "original"
		if err := tx.Create(representation).Error; err != nil {
			return err
		}
		asset.CurrentVersionID, asset.SHA256, asset.SizeBytes = version.ID, blob.SHA256, blob.SizeBytes
		asset.Status, asset.ThumbnailStatus, asset.PreviewStatus = iapiserver.AssetStatusActive, "pending", "pending"
		if err := tx.Save(asset).Error; err != nil {
			return err
		}
		if err := replaceLabelsTx(tx, owner, asset.ID, upload.PendingLabels); err != nil {
			return err
		}
		if err := addTagsTx(tx, owner, asset.ID, upload.PendingTags); err != nil {
			return err
		}
		upload.Status = "completed"
		if err := tx.Save(&upload).Error; err != nil {
			return err
		}
		if err := publishRepresentationRequested(tx, version); err != nil {
			return err
		}
		if err := publishOutbox(tx, OutboxTopicAssetUploaded, version.ID+":uploaded", map[string]any{"asset_id": asset.ID, "asset_version_id": version.ID, "owner_user_id": owner, "source_event_id": version.ID + ":uploaded"}); err != nil {
			return err
		}
		response = &iapiserver.CompleteAssetUploadResponse{Upload: &upload, Asset: asset, AssetVersion: version, OriginalRepresentation: representation}
		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if response != nil && response.Asset != nil {
		if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{response.Asset}); err != nil {
			return nil, err
		}
		if err := s.decorateRepresentations(ctx, []*iapiserver.AssetRepresentation{response.OriginalRepresentation}); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *assetV1Store) CancelAssetUpload(ctx context.Context, owner, id string) (*iapiserver.AssetUploadSession, error) {
	var upload iapiserver.AssetUploadSession
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&upload).Error; err != nil {
			return err
		}
		if upload.Status == "completed" {
			return errors.Errorf("completed upload cannot be cancelled")
		}
		if upload.Status == "cancelled" {
			return nil
		}
		upload.Status = "cancelled"
		return tx.Save(&upload).Error
	})
	return &upload, errors.WithStack(err)
}

func (s *assetV1Store) lockOrCreateUploadAsset(tx *gorm.DB, owner string, upload *iapiserver.AssetUploadSession) (*iapiserver.UserAsset, error) {
	if upload.TargetAssetID != "" {
		var asset iapiserver.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status <> ?", upload.TargetAssetID, owner, iapiserver.AssetStatusDeleted).First(&asset).Error; err != nil {
			return nil, err
		}
		return &asset, nil
	}
	mediaType, format := mediaTypeAndFormat(upload.MIMEType, upload.FileName)
	asset := &iapiserver.UserAsset{OwnerUserID: owner, DisplayName: upload.DisplayName, OriginalName: upload.FileName,
		MediaType: mediaType, Format: format, SizeBytes: upload.SizeBytes, SourceType: "upload", Status: iapiserver.AssetStatusActive,
		ThumbnailStatus: "pending", PreviewStatus: "pending", Labels: map[string]string{}, Tags: []string{}}
	asset.ID, asset.Name = uuid.NewString(), upload.DisplayName
	if err := tx.Create(asset).Error; err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *assetV1Store) loadCompletedUploadTx(tx *gorm.DB, owner string, upload *iapiserver.AssetUploadSession, response **iapiserver.CompleteAssetUploadResponse) error {
	var version iapiserver.AssetVersion
	if err := tx.Where("owner_user_id = ? AND source_type = 'upload' AND source_ref_id = ?", owner, upload.ID).First(&version).Error; err != nil {
		return err
	}
	var asset iapiserver.UserAsset
	if err := tx.Where("id = ? AND owner_user_id = ?", version.AssetID, owner).First(&asset).Error; err != nil {
		return err
	}
	var representation iapiserver.AssetRepresentation
	if err := tx.Where("asset_version_id = ? AND representation_type = ?", version.ID, iapiserver.AssetRepresentationOriginal).First(&representation).Error; err != nil {
		return err
	}
	*response = &iapiserver.CompleteAssetUploadResponse{Upload: upload, Asset: &asset, AssetVersion: &version, OriginalRepresentation: &representation, Deduplicated: true}
	return nil
}

func sameUploadedParts(left, right []iapiserver.UploadedPart) bool {
	if len(left) != len(right) {
		return false
	}
	copyLeft, copyRight := append([]iapiserver.UploadedPart(nil), left...), append([]iapiserver.UploadedPart(nil), right...)
	sort.Slice(copyLeft, func(i, j int) bool { return copyLeft[i].PartNumber < copyLeft[j].PartNumber })
	sort.Slice(copyRight, func(i, j int) bool { return copyRight[i].PartNumber < copyRight[j].PartNumber })
	for index := range copyLeft {
		if copyLeft[index] != copyRight[index] {
			return false
		}
	}
	return true
}

func mediaTypeAndFormat(mimeType, filename string) (string, string) {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	base := strings.ToLower(strings.SplitN(mimeType, "/", 2)[0])
	switch base {
	case "image", "video", "audio", "text":
		return base, format
	case "model":
		return "model_3d", format
	case "application":
		if format == "pdf" {
			return "pdf", format
		}
		return "document", format
	default:
		return "other", format
	}
}

func fmtVersionName(version int) string { return fmt.Sprintf("Asset version %d", version) }
