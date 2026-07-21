package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func (s *assetV1Store) ListUserAssets(ctx context.Context, owner string, req *iapiserver.UserAssetListRequest) ([]*iapiserver.UserAsset, int64, error) {
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.UserAsset{}).Where("owner_user_id = ?", owner)
	if req.Status == "" {
		query = query.Where("status IN ?", []string{iapiserver.AssetStatusActive, iapiserver.AssetStatusArchived})
	} else {
		query = query.Where("status = ?", req.Status)
	}
	if req.MediaType != "" {
		query = query.Where("media_type = ?", req.MediaType)
	}
	if req.Format != "" {
		query = query.Where("format = ?", req.Format)
	}
	if req.SourceType != "" {
		query = query.Where("source_type = ?", req.SourceType)
	}
	if req.Width > 0 {
		query = query.Where("width = ?", req.Width)
	}
	if req.Height > 0 {
		query = query.Where("height = ?", req.Height)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		query = query.Where("display_name ILIKE ? OR original_name ILIKE ? OR description ILIKE ?", like, like, like)
	}
	if req.SelectorExpression != nil {
		condition, args, err := compileAssetSelector(req.SelectorExpression, owner)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where(condition, args...)
	}
	if req.CreatedAfter > 0 {
		query = query.Where("created_at >= ?", time.Unix(req.CreatedAfter, 0))
	}
	if req.CreatedBefore > 0 {
		query = query.Where("created_at <= ?", time.Unix(req.CreatedBefore, 0))
	}
	sortField := req.SortField
	allowedSort := map[string]bool{"display_name": true, "size_bytes": true, "media_type": true, "format": true, "created_at": true, "updated_at": true}
	if !allowedSort[sortField] {
		sortField = "created_at"
	}
	sortOrder := "ASC"
	if strings.EqualFold(req.SortOrder, "desc") {
		sortOrder = "DESC"
	}
	query = query.Order(sortField + " " + sortOrder)
	var items []*iapiserver.UserAsset
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	if err != nil {
		return nil, 0, err
	}
	if err := s.decorateUserAssets(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *assetV1Store) GetUserAsset(ctx context.Context, owner, id string, includeDeleted bool) (*iapiserver.UserAsset, error) {
	query := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, owner)
	if !includeDeleted {
		query = query.Where("status <> ?", iapiserver.AssetStatusDeleted)
	}
	var asset iapiserver.UserAsset
	if err := query.First(&asset).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{&asset}); err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *assetV1Store) GetAssetDetail(ctx context.Context, owner, id string) (*iapiserver.AssetDetail, error) {
	asset, err := s.GetUserAsset(ctx, owner, id, false)
	if err != nil {
		return nil, err
	}
	versions, err := s.ListAssetVersions(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	representations := make([]*iapiserver.AssetRepresentation, 0)
	if len(versions) > 0 {
		ids := make([]string, 0, len(versions))
		for _, version := range versions {
			ids = append(ids, version.ID)
		}
		if err := s.ds.db.WithContext(ctx).Where("owner_user_id = ? AND asset_version_id IN ? AND deleted_at IS NULL", owner, ids).Find(&representations).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		if err := s.decorateRepresentations(ctx, representations); err != nil {
			return nil, err
		}
	}
	var collections []*iapiserver.AssetCollection
	if err := s.ds.db.WithContext(ctx).Table("user_asset_groups AS g").
		Joins("JOIN user_asset_group_memberships AS m ON m.group_id = g.id AND m.deleted_at IS NULL").
		Where("g.owner_user_id = ? AND g.deleted_at IS NULL AND m.asset_id = ?", owner, id).
		Find(&collections).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	for _, collection := range collections {
		if err := s.decorateCollection(ctx, collection); err != nil {
			return nil, err
		}
	}
	var current *iapiserver.AssetVersion
	for _, version := range versions {
		if version.ID == asset.CurrentVersionID {
			current = version
			break
		}
	}
	return &iapiserver.AssetDetail{
		Asset: asset, CurrentVersion: current, Versions: versions, Representations: representations, Collections: collections,
		ReferenceSummary: &iapiserver.ReferenceSummary{ReferenceCount: asset.ReferenceCount, Sources: asset.ReferenceSources},
	}, nil
}

func (s *assetV1Store) CreateCanonicalAsset(ctx context.Context, owner string, req *iapiserver.CreateCanonicalAssetRequest) (*iapiserver.AssetDetail, error) {
	assetID, versionID, representationID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	profile := req.ProfileVersion
	if profile == "" {
		profile = "canonical-v1"
	}
	asset := &iapiserver.UserAsset{OwnerUserID: owner, DisplayName: strings.TrimSpace(req.DisplayName), MediaType: req.MediaType,
		SourceType: "upload", Status: iapiserver.AssetStatusActive, CurrentVersionID: versionID,
		ThumbnailStatus: "none", PreviewStatus: "none", Labels: map[string]string{}, Tags: []string{}}
	asset.ID, asset.Name, asset.Description = assetID, asset.DisplayName, req.Description
	version := &iapiserver.AssetVersion{AssetID: assetID, OwnerUserID: owner, VersionNo: 1, Status: iapiserver.AssetVersionStatusReady,
		SourceType: "upload", Content: req.CanonicalContent, Metadata: map[string]any{}, ProfileVersion: profile, ExpectedCount: 1, CompletedCount: 1}
	version.ID, version.Name = versionID, "Asset version 1"
	representation := &iapiserver.AssetRepresentation{AssetVersionID: versionID, OwnerUserID: owner,
		RepresentationType: iapiserver.AssetRepresentationCanonical, Profile: "default", ProfileVersion: profile,
		Content: req.CanonicalContent, Metadata: map[string]any{}, Status: "ready", Required: true}
	representation.ID, representation.Name = representationID, "canonical"
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(asset).Error; err != nil {
			return err
		}
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		if err := tx.Create(representation).Error; err != nil {
			return err
		}
		if err := replaceLabelsTx(tx, owner, assetID, req.Labels); err != nil {
			return err
		}
		if err := addTagsTx(tx, owner, assetID, req.Tags); err != nil {
			return err
		}
		return publishRepresentationRequested(tx, version)
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetAssetDetail(ctx, owner, assetID)
}

func (s *assetV1Store) UpdateUserAsset(ctx context.Context, owner, id string, req *iapiserver.UpdateUserAssetRequest) (*iapiserver.UserAsset, error) {
	var result iapiserver.UserAsset
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status <> ?", id, owner, iapiserver.AssetStatusDeleted).First(&result).Error; err != nil {
			return err
		}
		if req.ResourceVersion != nil && result.ResourceVersion != *req.ResourceVersion {
			return errors.Errorf("asset resource version conflict")
		}
		if req.DisplayName != nil {
			result.DisplayName, result.Name = strings.TrimSpace(*req.DisplayName), strings.TrimSpace(*req.DisplayName)
		}
		if req.Description != nil {
			result.Description = *req.Description
		}
		if req.Status != nil {
			result.Status = *req.Status
		}
		return tx.Save(&result).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{&result}); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *assetV1Store) SetUserAssetDeleted(ctx context.Context, owner, id string, deleted bool) (*iapiserver.UserAsset, error) {
	var asset iapiserver.UserAsset
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner).First(&asset).Error; err != nil {
			return err
		}
		if deleted {
			if asset.Status == iapiserver.AssetStatusDeleted {
				return nil
			}
			asset.Status = iapiserver.AssetStatusDeleted
			now := imachinery.NewTime(time.Now())
			asset.DeletedAt = &now
		} else {
			if asset.Status != iapiserver.AssetStatusDeleted {
				return errors.Errorf("asset is not deleted")
			}
			asset.Status, asset.DeletedAt = iapiserver.AssetStatusActive, nil
		}
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

func (s *assetV1Store) PermanentlyDeleteUserAsset(ctx context.Context, owner, id string) (*iapiserver.PermanentDeleteResult, []store.StoredAssetContent, error) {
	result := &iapiserver.PermanentDeleteResult{AssetID: id, DeletedBlobIDs: []string{}, RetainedBlobIDs: []string{}}
	contents := make([]store.StoredAssetContent, 0)
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset iapiserver.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status = ?", id, owner, iapiserver.AssetStatusDeleted).First(&asset).Error; err != nil {
			return err
		}
		var blocking int64
		for _, check := range []struct {
			model any
			query string
		}{
			{&iapiserver.AssetCollectionItem{}, "asset_id = ? AND deleted_at IS NULL"},
			{&iapiserver.Artifact{}, "asset_id = ? AND deleted_at IS NULL"},
		} {
			if err := tx.Model(check.model).Where(check.query, id).Count(&blocking).Error; err != nil {
				return err
			}
			if blocking > 0 {
				return errors.Errorf("asset has blocking references")
			}
		}
		for _, check := range []struct {
			model any
			query string
		}{
			{&iapiserver.ApplicationArtifact{}, "asset_id = ?"},
			{&iapiserver.AssetRelation{}, "source_asset_id = ? OR target_asset_id = ?"},
			{&iapiserver.AssetGroupMember{}, "asset_id = ?"},
		} {
			if !tx.Migrator().HasTable(check.model) {
				continue
			}
			query := tx.Model(check.model).Where(check.query, id)
			if strings.Count(check.query, "?") == 2 {
				query = tx.Model(check.model).Where(check.query, id, id)
			}
			if err := query.Count(&blocking).Error; err != nil {
				return err
			}
			if blocking > 0 {
				return errors.Errorf("asset has blocking references")
			}
		}
		var versions []iapiserver.AssetVersion
		if err := tx.Where("asset_id = ? AND owner_user_id = ?", id, owner).Find(&versions).Error; err != nil {
			return err
		}
		versionIDs := make([]string, 0, len(versions))
		for _, version := range versions {
			versionIDs = append(versionIDs, version.ID)
		}
		var representations []iapiserver.AssetRepresentation
		if len(versionIDs) > 0 {
			if err := tx.Where("asset_version_id IN ?", versionIDs).Find(&representations).Error; err != nil {
				return err
			}
			if err := tx.Where("asset_version_id IN ?", versionIDs).Delete(&iapiserver.AssetRepresentation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", versionIDs).Delete(&iapiserver.AssetVersion{}).Error; err != nil {
				return err
			}
		}
		for _, representation := range representations {
			if representation.BlobID == "" {
				continue
			}
			var refs int64
			if err := tx.Model(&iapiserver.AssetRepresentation{}).Where("blob_id = ?", representation.BlobID).Count(&refs).Error; err != nil {
				return err
			}
			if err := tx.Model(&iapiserver.Artifact{}).Where("blob_id = ? AND deleted_at IS NULL", representation.BlobID).Count(&blocking).Error; err != nil {
				return err
			}
			if refs+blocking > 0 {
				result.RetainedBlobIDs = append(result.RetainedBlobIDs, representation.BlobID)
				continue
			}
			var blob iapiserver.AssetBlob
			if err := tx.Where("id = ?", representation.BlobID).First(&blob).Error; err != nil && !stderrors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if blob.ID != "" {
				contents = append(contents, store.StoredAssetContent{StorageBackendID: blob.StorageBackendID, ObjectKey: blob.ObjectKey, SHA256: blob.SHA256, SizeBytes: blob.SizeBytes, MIMEType: blob.MIMEType, BlobID: blob.ID})
				if err := tx.Delete(&blob).Error; err != nil {
					return err
				}
				result.DeletedBlobIDs = append(result.DeletedBlobIDs, blob.ID)
			}
		}
		if err := tx.Where("asset_id = ?", id).Delete(&iapiserver.UserAssetLabel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("asset_id = ?", id).Delete(&iapiserver.UserAssetTag{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&asset).Error; err != nil {
			return err
		}
		result.Deleted = true
		return nil
	})
	sort.Strings(result.DeletedBlobIDs)
	sort.Strings(result.RetainedBlobIDs)
	return result, contents, errors.WithStack(err)
}

func (s *assetV1Store) decorateUserAssets(ctx context.Context, assets []*iapiserver.UserAsset) error {
	if len(assets) == 0 {
		return nil
	}
	ids := make([]string, 0, len(assets))
	byID := make(map[string]*iapiserver.UserAsset, len(assets))
	for _, asset := range assets {
		ids, byID[asset.ID] = append(ids, asset.ID), asset
		asset.Labels, asset.Tags = map[string]string{}, []string{}
		asset.LabelSources, asset.TagSources = map[string]string{}, map[string]string{}
	}
	var labels []iapiserver.UserAssetLabel
	if err := s.ds.db.WithContext(ctx).Where("asset_id IN ? AND deleted_at IS NULL", ids).Find(&labels).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, label := range labels {
		if asset := byID[label.AssetID]; asset != nil {
			asset.Labels[label.Key], asset.LabelSources[label.Key] = label.Value, label.Source
		}
	}
	var tags []iapiserver.UserAssetTag
	if err := s.ds.db.WithContext(ctx).Where("asset_id IN ? AND deleted_at IS NULL", ids).Find(&tags).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, tag := range tags {
		if asset := byID[tag.AssetID]; asset != nil {
			asset.Tags = append(asset.Tags, tag.Tag)
			asset.TagSources[tag.Tag] = tag.Source
		}
	}
	for _, asset := range assets {
		sort.Strings(asset.Tags)
	}
	return nil
}

func publishRepresentationRequested(tx *gorm.DB, version *iapiserver.AssetVersion) error {
	var asset iapiserver.UserAsset
	if err := tx.Select("id", "media_type").Where("id = ? AND owner_user_id = ?", version.AssetID, version.OwnerUserID).First(&asset).Error; err != nil {
		return err
	}
	requested := []map[string]any{}
	if asset.MediaType == iapiserver.AssetMediaTypeImage {
		requested = append(requested, map[string]any{"representation_type": "thumbnail", "profile": "list-320", "required": false})
	}
	sourceID := fmt.Sprintf("%s:%s:representation_requested", version.ID, version.ProfileVersion)
	return publishOutbox(tx, OutboxTopicAssetVersionRepresentationRequested, sourceID, map[string]any{
		"source_event_id": sourceID, "source_domain": iapiserver.SSESourceDomainAssetLibrary,
		"asset_id": version.AssetID, "asset_version_id": version.ID, "owner_user_id": version.OwnerUserID,
		"project_id": iapiserver.DefaultTaskCenterProjectID, "namespace": iapiserver.DefaultTaskCenterNamespace,
		"media_type": asset.MediaType, "profile_version": version.ProfileVersion,
		"requested_representations": requested, "idempotency_key": "asset-representations:" + version.ID + ":" + version.ProfileVersion,
		"occurred_at": version.UpdatedAt,
	})
}

func expectedRepresentationCount(mediaType string) int {
	if mediaType == iapiserver.AssetMediaTypeImage {
		return 2
	}
	return 1
}

func initialRepresentationStatuses(mediaType string) (string, string) {
	if mediaType == iapiserver.AssetMediaTypeImage {
		return "pending", "none"
	}
	return "none", "none"
}

func compileAssetSelector(expression *iapiserver.AssetSelectorExpression, owner string) (string, []any, error) {
	if expression == nil {
		return "TRUE", nil, nil
	}
	if expression.Operator == "predicate" {
		if expression.Predicate == nil {
			return "", nil, errors.Errorf("selector predicate is missing")
		}
		predicate := expression.Predicate
		switch predicate.Kind {
		case "label":
			base := "SELECT 1 FROM user_asset_labels l WHERE l.asset_id = user_assets.id AND l.owner_user_id = ? AND l.deleted_at IS NULL AND l.label_key = ?"
			args := []any{owner, predicate.Key}
			switch predicate.Action {
			case "exists":
				return "EXISTS (" + base + ")", args, nil
			case "not_exists":
				return "NOT EXISTS (" + base + ")", args, nil
			case "eq":
				return "EXISTS (" + base + " AND l.label_value = ?)", append(args, predicate.Values[0]), nil
			case "neq":
				return "NOT EXISTS (" + base + " AND l.label_value = ?)", append(args, predicate.Values[0]), nil
			case "in":
				return "EXISTS (" + base + " AND l.label_value IN ?)", append(args, predicate.Values), nil
			case "notin":
				return "NOT EXISTS (" + base + " AND l.label_value IN ?)", append(args, predicate.Values), nil
			}
		case "tag":
			condition := "EXISTS (SELECT 1 FROM user_asset_tags t WHERE t.asset_id = user_assets.id AND t.owner_user_id = ? AND t.deleted_at IS NULL AND t.tag = ?)"
			if predicate.Action == "neq" {
				condition = "NOT " + condition
			}
			return condition, []any{owner, predicate.Values[0]}, nil
		case "group":
			return "EXISTS (SELECT 1 FROM user_asset_group_memberships m JOIN user_asset_groups g ON g.id = m.group_id WHERE m.asset_id = user_assets.id AND m.owner_user_id = ? AND m.deleted_at IS NULL AND g.owner_user_id = ? AND g.deleted_at IS NULL AND g.name = ?)", []any{owner, owner, predicate.Values[0]}, nil
		}
		return "", nil, errors.Errorf("unsupported selector predicate")
	}
	if expression.Operator != "and" && expression.Operator != "or" {
		return "", nil, errors.Errorf("unsupported selector operator")
	}
	parts := make([]string, 0, len(expression.Children))
	args := make([]any, 0)
	for _, child := range expression.Children {
		part, childArgs, err := compileAssetSelector(child, owner)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, "("+part+")")
		args = append(args, childArgs...)
	}
	joiner := " AND "
	if expression.Operator == "or" {
		joiner = " OR "
	}
	return strings.Join(parts, joiner), args, nil
}
