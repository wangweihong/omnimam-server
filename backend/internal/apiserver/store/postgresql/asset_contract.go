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

// ListDeletedUserAssetIDs 返回当前用户回收站中的全部素材 ID，供清空操作按稳定顺序逐项处理。
func (s *assetV1Store) ListDeletedUserAssetIDs(ctx context.Context, owner string) ([]string, error) {
	ids := make([]string, 0)
	err := s.ds.db.WithContext(ctx).
		Model(&iapiserver.UserAsset{}).
		Where("owner_user_id = ? AND status = ?", owner, iapiserver.AssetStatusDeleted).
		Order("created_at ASC").
		Order("id ASC").
		Pluck("id", &ids).Error
	return ids, errors.WithStack(err)
}

// HardDeleteUserAsset 从任意素材状态直接执行永久删除，仍保留强引用和 Blob 共享检查。
func (s *assetV1Store) HardDeleteUserAsset(ctx context.Context, owner, id string) (*iapiserver.PermanentDeleteResult, []store.StoredAssetContent, error) {
	return s.permanentlyDeleteUserAsset(ctx, owner, id, false)
}

// PermanentlyDeleteUserAsset 只永久删除已进入回收站的素材，保持既有 permanent endpoint 语义。
func (s *assetV1Store) PermanentlyDeleteUserAsset(ctx context.Context, owner, id string) (*iapiserver.PermanentDeleteResult, []store.StoredAssetContent, error) {
	return s.permanentlyDeleteUserAsset(ctx, owner, id, true)
}

func (s *assetV1Store) permanentlyDeleteUserAsset(ctx context.Context, owner, id string, requireDeleted bool) (*iapiserver.PermanentDeleteResult, []store.StoredAssetContent, error) {
	result := &iapiserver.PermanentDeleteResult{AssetID: id, DeletedBlobIDs: []string{}, RetainedBlobIDs: []string{}}
	contents := make([]store.StoredAssetContent, 0)
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset iapiserver.UserAsset
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, owner)
		if requireDeleted {
			query = query.Where("status = ?", iapiserver.AssetStatusDeleted)
		}
		if err := query.First(&asset).Error; err != nil {
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
				return store.ErrAssetDeleteBlocked
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
				return store.ErrAssetDeleteBlocked
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
			// Artifact 软删除只隐藏投影并保留 Blob；共享检查不能把它当成已释放引用。
			if err := tx.Model(&iapiserver.Artifact{}).Where("blob_id = ?", representation.BlobID).Count(&blocking).Error; err != nil {
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
		asset.CurrentVersion = nil
		asset.Labels, asset.Tags = map[string]string{}, []string{}
		asset.LabelSources, asset.TagSources = map[string]string{}, map[string]string{}
	}
	versionIDs := make([]string, 0, len(assets))
	versionOwners := make(map[string][]*iapiserver.UserAsset)
	for _, asset := range assets {
		if asset.CurrentVersionID != "" {
			versionIDs = append(versionIDs, asset.CurrentVersionID)
			versionOwners[asset.CurrentVersionID] = append(versionOwners[asset.CurrentVersionID], asset)
		}
	}
	if len(versionIDs) > 0 {
		var versions []*iapiserver.AssetVersion
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ?", versionIDs, assets[0].OwnerUserID).Find(&versions).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, version := range versions {
			for _, asset := range versionOwners[version.ID] {
				if asset.OwnerUserID == version.OwnerUserID && asset.ID == version.AssetID {
					asset.CurrentVersion = assetVersionSummary(version)
				}
			}
		}
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

func assetVersionSummary(version *iapiserver.AssetVersion) *iapiserver.AssetVersionSummary {
	if version == nil {
		return nil
	}
	return &iapiserver.AssetVersionSummary{ID: version.ID, AssetID: version.AssetID, VersionNo: version.VersionNo, Status: version.Status, SourceType: version.SourceType}
}

func userAssetSummary(asset *iapiserver.UserAsset) *iapiserver.UserAssetSummary {
	if asset == nil {
		return nil
	}
	return &iapiserver.UserAssetSummary{ID: asset.ID, DisplayName: asset.DisplayName, MediaType: asset.MediaType, Status: asset.Status, ThumbnailStatus: asset.ThumbnailStatus, CurrentVersionID: asset.CurrentVersionID}
}

func publishRepresentationRequested(tx *gorm.DB, version *iapiserver.AssetVersion) error {
	var asset iapiserver.UserAsset
	if err := tx.Select("id", "media_type").Where("id = ? AND owner_user_id = ?", version.AssetID, version.OwnerUserID).First(&asset).Error; err != nil {
		return err
	}
	return publishRepresentationRequestedWithPlan(tx, version, defaultRepresentationPlan(asset.MediaType, version.ProfileVersion))
}

func publishRepresentationRequestedWithPlan(tx *gorm.DB, version *iapiserver.AssetVersion, plan store.RepresentationPlan) error {
	resolved, err := validateRepresentationPlan(plan, plan.MediaType, version.ProfileVersion)
	if err != nil {
		return err
	}
	requested := make([]map[string]any, 0, len(resolved.Requested))
	for _, item := range resolved.Requested {
		requested = append(requested, map[string]any{"representation_type": item.Type, "profile": item.Profile, "required": item.Required})
	}
	sourceID := fmt.Sprintf("%s:%s:representation_requested", version.ID, version.ProfileVersion)
	return publishOutbox(tx, OutboxTopicAssetVersionRepresentationRequested, sourceID, map[string]any{
		"source_event_id": sourceID, "source_domain": iapiserver.SSESourceDomainAssetLibrary,
		"asset_id": version.AssetID, "asset_version_id": version.ID, "owner_user_id": version.OwnerUserID,
		"project_id": iapiserver.DefaultTaskCenterProjectID, "namespace": iapiserver.DefaultTaskCenterNamespace,
		"media_type": resolved.MediaType, "profile_version": version.ProfileVersion,
		"requested_representations": requested, "idempotency_key": "asset-representations:" + version.ID + ":" + version.ProfileVersion,
		"occurred_at": version.UpdatedAt,
	})
}

func expectedRepresentationCount(mediaType string) int {
	return defaultRepresentationPlan(mediaType, "default-v1").ExpectedCount
}

func initialRepresentationStatuses(mediaType string) (string, string) {
	return initialRepresentationStatusesForPlan(defaultRepresentationPlan(mediaType, "default-v1"))
}

func initialRepresentationStatusesForPlan(plan store.RepresentationPlan) (string, string) {
	if len(plan.Requested) > 0 {
		return "pending", "none"
	}
	return "none", "none"
}

func defaultRepresentationPlan(mediaType, profileVersion string) store.RepresentationPlan {
	plan := store.RepresentationPlan{MediaType: mediaType, ProfileVersion: profileVersion, ExpectedCount: 1}
	if mediaType == iapiserver.AssetMediaTypeImage || mediaType == iapiserver.AssetMediaTypeVideo {
		plan.ExpectedCount = 2
		plan.Requested = []store.ExpectedRepresentation{{Type: "thumbnail", Profile: "list-320", Required: false}}
	}
	return plan
}

func validateRepresentationPlan(plan store.RepresentationPlan, mediaType, profileVersion string) (store.RepresentationPlan, error) {
	if plan.MediaType == "" {
		plan = defaultRepresentationPlan(mediaType, profileVersion)
	}
	if plan.MediaType != mediaType || plan.ProfileVersion != profileVersion || plan.ExpectedCount != 1+len(plan.Requested) {
		return store.RepresentationPlan{}, errors.Errorf("representation plan is invalid")
	}
	for _, item := range plan.Requested {
		if item.Type != "thumbnail" || item.Profile != "list-320" {
			return store.RepresentationPlan{}, errors.Errorf("representation plan contains an unsupported item")
		}
	}
	return plan, nil
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
