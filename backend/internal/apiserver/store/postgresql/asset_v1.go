package postgresql

import (
	"context"
	stderrors "errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type assetV1Store struct{ ds *datastore }

func newAssetV1Store(ds *datastore) *assetV1Store { return &assetV1Store{ds: ds} }

func (s *assetV1Store) RegisterArtifact(ctx context.Context, req *iapiserver.ArtifactRegistrationRequest) (*iapiserver.UserAsset, bool, error) {
	var asset *iapiserver.UserAsset
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var registration iapiserver.ArtifactAssetRegistration
		err := tx.Where("artifact_id = ?", req.ArtifactID).First(&registration).Error
		if err == nil {
			if registration.ApplicationRunID != req.ApplicationRunID || registration.OwnerUserID != req.OwnerUserID || registration.ContentRef != req.ContentRef {
				return errors.Errorf("artifact registration conflicts with existing mapping")
			}
			var existing iapiserver.UserAsset
			if err := tx.Where("id = ? AND owner_user_id = ?", registration.AssetID, registration.OwnerUserID).First(&existing).Error; err != nil {
				return err
			}
			asset = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		item := &iapiserver.UserAsset{OwnerUserID: req.OwnerUserID, DisplayName: req.OutputName, MediaType: req.MediaType, Format: req.Format, SizeBytes: req.SizeBytes, Width: req.Width, Height: req.Height, DurationSeconds: req.DurationSeconds, SourceType: "application_output", ObjectPath: req.ContentRef, ThumbnailStatus: "none", PreviewStatus: "none", SHA256: req.SHA256, Labels: map[string]string{}, Tags: []string{}, LabelSources: map[string]string{}, TagSources: map[string]string{}}
		item.ID = uuid.NewString()
		item.Name = req.OutputName
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		mapping := &iapiserver.ArtifactAssetRegistration{ArtifactID: req.ArtifactID, ApplicationRunID: req.ApplicationRunID, OwnerUserID: req.OwnerUserID, AssetID: item.ID, ContentRef: req.ContentRef, MediaType: req.MediaType}
		mapping.ID = uuid.NewString()
		mapping.Name = "Artifact registration"
		if err := tx.Create(mapping).Error; err != nil {
			return err
		}
		if err := publishOutbox(tx, OutboxTopicArtifactRegistered, req.ArtifactID+":"+item.ID, map[string]any{"artifact_id": req.ArtifactID, "application_run_id": req.ApplicationRunID, "owner_user_id": req.OwnerUserID, "asset_id": item.ID, "registration_result": "created"}); err != nil {
			return err
		}
		asset = item
		created = true
		return nil
	})
	return asset, created, errors.WithStack(err)
}

func (s *assetV1Store) ApplyLabels(ctx context.Context, owner, id string, upsert map[string]string, add, remove []string) (*iapiserver.BatchLabelData, error) {
	var result *iapiserver.BatchLabelData
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset iapiserver.UserAsset
		if err := tx.Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&asset).Error; err != nil {
			return err
		}
		for key, value := range upsert {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			var label iapiserver.UserAssetLabel
			err := tx.Where("asset_id = ? AND key = ?", id, key).First(&label).Error
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				label.ID = uuid.NewString()
				label.Name = "Asset label"
				label.AssetID = id
				label.Key = key
			} else if err != nil {
				return err
			}
			label.Value = value
			label.Source = "manual"
			if err := tx.Save(&label).Error; err != nil {
				return err
			}
		}
		for _, tag := range add {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			item := &iapiserver.UserAssetTag{AssetID: id, Tag: tag, Source: "manual"}
			item.ID = uuid.NewString()
			item.Name = "Asset tag"
			if err := tx.Where("asset_id = ? AND tag = ?", id, tag).FirstOrCreate(item).Error; err != nil {
				return err
			}
		}
		if len(remove) > 0 {
			if err := tx.Where("asset_id = ? AND tag IN ?", id, remove).Delete(&iapiserver.UserAssetTag{}).Error; err != nil {
				return err
			}
		}
		var labels []iapiserver.UserAssetLabel
		var tags []iapiserver.UserAssetTag
		if err := tx.Where("asset_id = ?", id).Find(&labels).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", id).Find(&tags).Error; err != nil {
			return err
		}
		data := &iapiserver.BatchLabelData{ID: id, Labels: map[string]string{}, Tags: []string{}, LabelSources: map[string]string{}, TagSources: map[string]string{}, ResourceVersion: asset.ResourceVersion + 1}
		for _, l := range labels {
			data.Labels[l.Key] = l.Value
			data.LabelSources[l.Key] = l.Source
		}
		for _, t := range tags {
			data.Tags = append(data.Tags, t.Tag)
			data.TagSources[t.Tag] = t.Source
		}
		sort.Strings(data.Tags)
		if err := tx.Model(&asset).Update("resource_version", data.ResourceVersion).Error; err != nil {
			return err
		}
		result = data
		return nil
	})
	return result, errors.WithStack(err)
}
