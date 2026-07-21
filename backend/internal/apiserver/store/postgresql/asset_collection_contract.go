package postgresql

import (
	"context"
	stderrors "errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func (s *assetV1Store) ListCollections(ctx context.Context, owner string, req *iapiserver.CollectionListRequest) ([]*iapiserver.AssetCollection, int64, error) {
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.AssetCollection{}).Where("owner_user_id = ? AND deleted_at IS NULL", owner)
	if req.ParentCollectionID != "" {
		query = query.Where("parent_group_id = ?", req.ParentCollectionID)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", like, like)
	}
	sortField := req.SortField
	if !map[string]bool{"name": true, "sort_order": true, "created_at": true, "updated_at": true}[sortField] {
		sortField = "sort_order"
	}
	order := "ASC"
	if strings.EqualFold(req.SortOrder, "desc") {
		order = "DESC"
	}
	query = query.Order(sortField + " " + order)
	var items []*iapiserver.AssetCollection
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	if err != nil {
		return nil, 0, err
	}
	if err := s.decorateCollections(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *assetV1Store) GetCollection(ctx context.Context, owner, id string, paging imachinery.PagingParams) (*iapiserver.CollectionDetail, error) {
	var collection iapiserver.AssetCollection
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&collection).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateCollections(ctx, []*iapiserver.AssetCollection{&collection}); err != nil {
		return nil, err
	}
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.AssetCollectionItem{}).Where("group_id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).Order("sort_order ASC, joined_at DESC")
	var items []*iapiserver.AssetCollectionItem
	total, err := CountAndFindPage(query, paging, &items)
	if err != nil {
		return nil, err
	}
	assetIDs := make([]string, 0, len(items))
	pinnedVersionIDs := make([]string, 0, len(items))
	byID := map[string]*iapiserver.AssetCollectionItem{}
	for _, item := range items {
		assetIDs = append(assetIDs, item.AssetID)
		if item.PinnedVersionID != "" {
			pinnedVersionIDs = append(pinnedVersionIDs, item.PinnedVersionID)
		}
		byID[item.AssetID] = item
	}
	if len(pinnedVersionIDs) > 0 {
		var versions []*iapiserver.AssetVersion
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ?", pinnedVersionIDs, owner).Find(&versions).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		byVersionID := make(map[string]*iapiserver.AssetVersionSummary, len(versions))
		for _, version := range versions {
			byVersionID[version.ID] = assetVersionSummary(version)
		}
		for _, item := range items {
			if summary := byVersionID[item.PinnedVersionID]; summary != nil && summary.AssetID == item.AssetID {
				item.PinnedVersion = summary
			}
		}
	}
	if len(assetIDs) > 0 {
		var assets []*iapiserver.UserAsset
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ?", assetIDs, owner).Find(&assets).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		if err := s.decorateUserAssets(ctx, assets); err != nil {
			return nil, err
		}
		for _, asset := range assets {
			byID[asset.ID].Asset = asset
		}
	}
	return &iapiserver.CollectionDetail{Collection: &collection, Total: total, Items: items}, nil
}

func (s *assetV1Store) CreateCollection(ctx context.Context, owner string, req *iapiserver.CreateCollectionRequest) (*iapiserver.AssetCollection, error) {
	collection := &iapiserver.AssetCollection{OwnerUserID: owner, ParentCollectionID: req.ParentCollectionID, Color: req.Color, SortOrder: req.SortOrder}
	collection.Name, collection.Description = strings.TrimSpace(req.Name), req.Description
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateCollectionParentTx(tx, owner, "", collection.ParentCollectionID); err != nil {
			return err
		}
		if err := ensureCollectionNameAvailableTx(tx, owner, collection.Name, ""); err != nil {
			return err
		}
		return tx.Create(collection).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateCollection(ctx, collection); err != nil {
		return nil, err
	}
	return collection, nil
}

func (s *assetV1Store) UpdateCollection(ctx context.Context, owner, id string, req *iapiserver.UpdateCollectionRequest) (*iapiserver.AssetCollection, error) {
	var collection iapiserver.AssetCollection
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&collection).Error; err != nil {
			return err
		}
		if req.ResourceVersion != nil && collection.ResourceVersion != *req.ResourceVersion {
			return errors.Errorf("collection resource version conflict")
		}
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if err := ensureCollectionNameAvailableTx(tx, owner, name, id); err != nil {
				return err
			}
			collection.Name = name
		}
		if req.Description != nil {
			collection.Description = *req.Description
		}
		if req.ParentCollectionID != nil {
			if err := validateCollectionParentTx(tx, owner, id, *req.ParentCollectionID); err != nil {
				return err
			}
			collection.ParentCollectionID = *req.ParentCollectionID
		}
		if req.Color != nil {
			collection.Color = *req.Color
		}
		if req.SortOrder != nil {
			collection.SortOrder = *req.SortOrder
		}
		return tx.Save(&collection).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateCollection(ctx, &collection); err != nil {
		return nil, err
	}
	return &collection, nil
}

func (s *assetV1Store) DeleteCollection(ctx context.Context, owner, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var collection iapiserver.AssetCollection
		if err := tx.Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).First(&collection).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ? AND owner_user_id = ?", id, owner).Delete(&iapiserver.AssetCollectionItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&collection).Error
	}))
}

func (s *assetV1Store) AddCollectionItems(ctx context.Context, owner, collectionID string, requests []iapiserver.AddCollectionItem) (*iapiserver.CollectionItemBatchResponse, error) {
	response := &iapiserver.CollectionItemBatchResponse{Total: len(requests), Results: make([]iapiserver.CollectionItemResult, 0, len(requests))}
	for _, request := range requests {
		var result *iapiserver.AssetCollectionItem
		err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := requireCollectionTx(tx, owner, collectionID); err != nil {
				return err
			}
			if err := requireAssetAndVersionTx(tx, owner, request.AssetID, request.PinnedVersionID); err != nil {
				return err
			}
			var existing iapiserver.AssetCollectionItem
			err := tx.Where("owner_user_id = ? AND group_id = ? AND asset_id = ? AND deleted_at IS NULL", owner, collectionID, request.AssetID).First(&existing).Error
			if err == nil {
				result = &existing
				return nil
			}
			if !stderrors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			item := &iapiserver.AssetCollectionItem{OwnerUserID: owner, CollectionID: collectionID, AssetID: request.AssetID,
				PinnedVersionID: request.PinnedVersionID, Role: request.Role, SortOrder: request.SortOrder, Metadata: request.Metadata, CreatedBy: owner}
			item.Name = request.AssetID
			if err := tx.Create(item).Error; err != nil {
				return err
			}
			result = item
			return nil
		})
		itemResult := iapiserver.CollectionItemResult{AssetID: request.AssetID, Success: err == nil, Data: result}
		if err != nil {
			response.Fail++
			itemResult.Error = contractErrorBody(err)
		} else {
			response.Success++
		}
		response.Results = append(response.Results, itemResult)
	}
	return response, nil
}

func (s *assetV1Store) UpdateCollectionItem(ctx context.Context, owner, collectionID, itemID string, req *iapiserver.UpdateCollectionItemRequest) (*iapiserver.AssetCollectionItem, error) {
	var item iapiserver.AssetCollectionItem
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND group_id = ? AND owner_user_id = ? AND deleted_at IS NULL", itemID, collectionID, owner).First(&item).Error; err != nil {
			return err
		}
		if req.PinnedVersionID != nil {
			if err := requireAssetAndVersionTx(tx, owner, item.AssetID, *req.PinnedVersionID); err != nil {
				return err
			}
			item.PinnedVersionID = *req.PinnedVersionID
		}
		if req.Role != nil {
			item.Role = *req.Role
		}
		if req.SortOrder != nil {
			item.SortOrder = *req.SortOrder
		}
		if req.Metadata != nil {
			item.Metadata = *req.Metadata
		}
		return tx.Save(&item).Error
	})
	return &item, errors.WithStack(err)
}

func (s *assetV1Store) DeleteCollectionItem(ctx context.Context, owner, collectionID, itemID string) error {
	result := s.ds.db.WithContext(ctx).Where("id = ? AND group_id = ? AND owner_user_id = ? AND deleted_at IS NULL", itemID, collectionID, owner).Delete(&iapiserver.AssetCollectionItem{})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.WithStack(gorm.ErrRecordNotFound)
	}
	return nil
}

func (s *assetV1Store) ReplaceLabels(ctx context.Context, owner, assetID string, labels map[string]string) (*iapiserver.BatchLabelData, error) {
	return s.changeLabels(ctx, owner, assetID, func(tx *gorm.DB) error { return replaceLabelsTx(tx, owner, assetID, labels) })
}

func (s *assetV1Store) DeleteLabel(ctx context.Context, owner, assetID, labelID string) (*iapiserver.BatchLabelData, error) {
	return s.changeLabels(ctx, owner, assetID, func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND asset_id = ? AND owner_user_id = ? AND deleted_at IS NULL", labelID, assetID, owner).Delete(&iapiserver.UserAssetLabel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *assetV1Store) AddTags(ctx context.Context, owner, assetID string, tags []string) (*iapiserver.BatchLabelData, error) {
	return s.changeLabels(ctx, owner, assetID, func(tx *gorm.DB) error { return addTagsTx(tx, owner, assetID, tags) })
}

func (s *assetV1Store) DeleteTag(ctx context.Context, owner, assetID, tagID string) (*iapiserver.BatchLabelData, error) {
	return s.changeLabels(ctx, owner, assetID, func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND asset_id = ? AND owner_user_id = ? AND deleted_at IS NULL", tagID, assetID, owner).Delete(&iapiserver.UserAssetTag{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *assetV1Store) changeLabels(ctx context.Context, owner, assetID string, mutation func(*gorm.DB) error) (*iapiserver.BatchLabelData, error) {
	var asset iapiserver.UserAsset
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND status <> ?", assetID, owner, iapiserver.AssetStatusDeleted).First(&asset).Error; err != nil {
			return err
		}
		if err := mutation(tx); err != nil {
			return err
		}
		return tx.Save(&asset).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.decorateUserAssets(ctx, []*iapiserver.UserAsset{&asset}); err != nil {
		return nil, err
	}
	return &iapiserver.BatchLabelData{ID: asset.ID, Labels: asset.Labels, Tags: asset.Tags, LabelSources: asset.LabelSources, TagSources: asset.TagSources, ResourceVersion: asset.ResourceVersion}, nil
}

func replaceLabelsTx(tx *gorm.DB, owner, assetID string, labels map[string]string) error {
	if err := tx.Where("asset_id = ? AND owner_user_id = ?", assetID, owner).Delete(&iapiserver.UserAssetLabel{}).Error; err != nil {
		return err
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		label := &iapiserver.UserAssetLabel{OwnerUserID: owner, AssetID: assetID, Key: strings.TrimSpace(key), Value: strings.TrimSpace(labels[key]), Source: "manual"}
		label.ID, label.Name = uuid.NewString(), strings.TrimSpace(key)
		if err := tx.Create(label).Error; err != nil {
			return err
		}
	}
	return nil
}

func addTagsTx(tx *gorm.DB, owner, assetID string, tags []string) error {
	for _, value := range tags {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		var existing iapiserver.UserAssetTag
		err := tx.Where("asset_id = ? AND owner_user_id = ? AND tag = ? AND deleted_at IS NULL", assetID, owner, value).First(&existing).Error
		if err == nil {
			continue
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		tag := &iapiserver.UserAssetTag{OwnerUserID: owner, AssetID: assetID, Tag: value, Source: "manual"}
		tag.ID, tag.Name = uuid.NewString(), value
		if err := tx.Create(tag).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *assetV1Store) decorateCollection(ctx context.Context, collection *iapiserver.AssetCollection) error {
	return s.decorateCollections(ctx, []*iapiserver.AssetCollection{collection})
}

func (s *assetV1Store) decorateCollections(ctx context.Context, collections []*iapiserver.AssetCollection) error {
	if len(collections) == 0 {
		return nil
	}
	owner := collections[0].OwnerUserID
	all := make(map[string]*iapiserver.AssetCollection, len(collections))
	frontier := make([]string, 0, len(collections))
	for _, item := range collections {
		all[item.ID] = item
		item.ParentCollection = nil
		if item.ParentCollectionID != "" {
			frontier = append(frontier, item.ParentCollectionID)
		}
	}
	for level := 0; level < 8 && len(frontier) > 0; level++ {
		frontier = uniqueStrings(frontier)
		missing := make([]string, 0, len(frontier))
		for _, id := range frontier {
			if all[id] == nil {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			break
		}
		var parents []*iapiserver.AssetCollection
		if err := s.ds.db.WithContext(ctx).Where("id IN ? AND owner_user_id = ? AND deleted_at IS NULL", missing, owner).Find(&parents).Error; err != nil {
			return errors.WithStack(err)
		}
		frontier = frontier[:0]
		for _, parent := range parents {
			all[parent.ID] = parent
			if parent.ParentCollectionID != "" {
				frontier = append(frontier, parent.ParentCollectionID)
			}
		}
	}
	type collectionCount struct {
		CollectionID string `gorm:"column:collection_id"`
		Count        int64  `gorm:"column:item_count"`
	}
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	var counts []collectionCount
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.AssetCollectionItem{}).
		Select("group_id AS collection_id, COUNT(*) AS item_count").
		Where("group_id IN ? AND owner_user_id = ? AND deleted_at IS NULL", ids, owner).
		Group("group_id").Scan(&counts).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, count := range counts {
		if item := all[count.CollectionID]; item != nil {
			item.ItemCount = count.Count
		}
	}
	depthOf := func(item *iapiserver.AssetCollection) int {
		depth, parentID := 0, item.ParentCollectionID
		seen := map[string]struct{}{item.ID: {}}
		for parentID != "" && depth < 8 {
			if _, ok := seen[parentID]; ok {
				break
			}
			seen[parentID] = struct{}{}
			parent := all[parentID]
			if parent == nil {
				break
			}
			depth++
			parentID = parent.ParentCollectionID
		}
		return depth
	}
	for _, item := range all {
		item.Depth = depthOf(item)
	}
	for _, item := range collections {
		if parent := all[item.ParentCollectionID]; parent != nil {
			item.ParentCollection = &iapiserver.CollectionSummary{ID: parent.ID, Name: parent.Name, Color: parent.Color, Depth: parent.Depth, ItemCount: parent.ItemCount}
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func ensureCollectionNameAvailableTx(tx *gorm.DB, owner, name, exceptID string) error {
	query := tx.Model(&iapiserver.AssetCollection{}).Where("owner_user_id = ? AND lower(trim(name)) = lower(trim(?)) AND deleted_at IS NULL", owner, name)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.Errorf("collection name conflict")
	}
	return nil
}

func validateCollectionParentTx(tx *gorm.DB, owner, collectionID, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == collectionID {
		return errors.Errorf("collection hierarchy cycle")
	}
	current := parentID
	for depth := 1; depth <= 8; depth++ {
		var parent iapiserver.AssetCollection
		if err := tx.Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", current, owner).First(&parent).Error; err != nil {
			return err
		}
		if parent.ParentCollectionID == "" {
			return nil
		}
		if parent.ParentCollectionID == collectionID {
			return errors.Errorf("collection hierarchy cycle")
		}
		current = parent.ParentCollectionID
	}
	return errors.Errorf("collection hierarchy exceeds depth limit")
}

func requireCollectionTx(tx *gorm.DB, owner, id string) error {
	var count int64
	if err := tx.Model(&iapiserver.AssetCollection{}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, owner).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func requireAssetAndVersionTx(tx *gorm.DB, owner, assetID, versionID string) error {
	var count int64
	if err := tx.Model(&iapiserver.UserAsset{}).Where("id = ? AND owner_user_id = ? AND status <> ?", assetID, owner, iapiserver.AssetStatusDeleted).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	if versionID == "" {
		return nil
	}
	if err := tx.Model(&iapiserver.AssetVersion{}).Where("id = ? AND asset_id = ? AND owner_user_id = ?", versionID, assetID, owner).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.Errorf("pinned version is invalid")
	}
	return nil
}

func contractErrorBody(err error) map[string]any {
	return map[string]any{"code": "collection_item_invalid", "value": 151403, "message": err.Error(), "retryable": false}
}
