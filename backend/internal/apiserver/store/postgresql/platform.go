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

type providerStore struct{ ds *datastore }

func newProvider(ds *datastore) *providerStore { return &providerStore{ds: ds} }

func (s *providerStore) List(
	ctx context.Context,
	req *iapiserver.ProviderListRequest,
) ([]*iapiserver.Provider, int64, error) {
	var items []*iapiserver.Provider
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("deleted_at = ''")
		if req.OwnerUserID != "" {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.Type != "" {
			q = q.Where("provider_type = ?", req.Type)
		}
		if req.Enabled != nil {
			q = q.Where("enabled = ?", *req.Enabled)
		}
		return q
	}
	query := modelManagementQuery(ctx, s.ds.db.Model(&iapiserver.Provider{}), req.BasicQueryParam, filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	if err := modelManagementPaginate(query, req.PageNum, req.PageSize).Find(&items).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *providerStore) Get(ctx context.Context, id string) (*iapiserver.Provider, error) {
	var item iapiserver.Provider
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at = ''", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *providerStore) Add(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if CheckExists(tx, &iapiserver.Provider{}, map[string]any{
			"owner_user_id": data.OwnerUserID,
			"name":          data.Name,
			"deleted_at":    "",
		}) {
			return errors.NewStatusF(code.ErrModelProviderNameDuplicated, "provider name %s already exists", data.Name)
		}

		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	})

	return data, errors.WithStack(err)
}

func (s *providerStore) Update(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error) {
	updated := &iapiserver.Provider{}
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		if err := tx.Where("owner_user_id = ? AND name = ? AND deleted_at = ''", data.OwnerUserID, data.Name).
			First(updated).Error; err == nil && updated.ID != data.ID {
			return errors.NewStatusF(code.ErrModelProviderNameDuplicated, "provider name %s already exists", data.Name)
		}
		// 3. 执行更新，并返回新数据（或使用 Returning）
		if err := tx.Model(&data).Select("*").Updates(data).Error; err != nil {
			return err
		}
		// 4. 重新查询更新后的数据（或使用 clause.Returning）
		updated = data // 若 Updates 未填充模型，需重新查询
		return nil
	})

	return updated, errors.WithStack(err)
}

func (s *providerStore) Delete(ctx context.Context, id string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Model(&iapiserver.Provider{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now().UTC().Format(time.RFC3339)).Error)
}

type providerModelStore struct{ ds *datastore }

func newProviderModel(ds *datastore) *providerModelStore { return &providerModelStore{ds: ds} }

func (s *providerModelStore) List(
	ctx context.Context,
	req *iapiserver.ProviderModelListRequest,
) ([]*iapiserver.ProviderModel, int64, error) {
	var items []*iapiserver.ProviderModel
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("deleted_at = ''")
		if req.OwnerUserID != "" {
			q = q.Where("owner_user_id = ?", req.OwnerUserID)
		}
		if req.ProviderID != "" {
			q = q.Where("provider_id = ?", req.ProviderID)
		}
		if req.Enabled != nil {
			q = q.Where("enabled = ?", *req.Enabled)
		}
		if req.Capability != "" {
			q = q.Where("capabilities_json LIKE ?", "%"+req.Capability+"%")
		}
		return q
	}
	query := modelManagementQuery(ctx, s.ds.db.Model(&iapiserver.ProviderModel{}), req.BasicQueryParam, filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	if err := modelManagementPaginate(query, req.PageNum, req.PageSize).Find(&items).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func modelManagementQuery(
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
	query = query.Order(modelManagementOrder(params.SortField, params.SortOrder))
	return query
}

func modelManagementPaginate(query *gorm.DB, pageNum, pageSize int) *gorm.DB {
	if pageNum <= 0 || pageSize <= 0 {
		return query
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return query.Offset((pageNum - 1) * pageSize).Limit(pageSize)
}

func modelManagementOrder(sortField, sortOrder string) clause.OrderByColumn {
	columns := map[string]string{
		"id":         "id",
		"name":       "name",
		"createdAt":  "created_at",
		"created_at": "created_at",
		"updatedAt":  "updated_at",
		"updated_at": "updated_at",
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

func (s *providerModelStore) Get(ctx context.Context, id string) (*iapiserver.ProviderModel, error) {
	var item iapiserver.ProviderModel
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND deleted_at = ''", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *providerModelStore) Add(
	ctx context.Context,
	data *iapiserver.ProviderModel,
) (*iapiserver.ProviderModel, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *providerModelStore) Update(
	ctx context.Context,
	data *iapiserver.ProviderModel,
) (*iapiserver.ProviderModel, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *providerModelStore) Delete(ctx context.Context, providerID, id string) error {
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.ProviderModel{}).Where("id = ?", id)
	if providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}
	return errors.WithStack(query.Update("deleted_at", time.Now().UTC().Format(time.RFC3339)).Error)
}

func (s *providerModelStore) DeleteByProviderID(ctx context.Context, providerID string) error {
	return errors.WithStack(
		s.ds.db.WithContext(ctx).Model(&iapiserver.ProviderModel{}).
			Where("provider_id = ?", providerID).
			Update("deleted_at", time.Now().UTC().Format(time.RFC3339)).Error,
	)
}

type providerCapabilityStore struct{ ds *datastore }

func newProviderCapability(ds *datastore) *providerCapabilityStore {
	return &providerCapabilityStore{ds: ds}
}

func (s *providerCapabilityStore) List(ctx context.Context) ([]*iapiserver.ProviderCapability, error) {
	var items []*iapiserver.ProviderCapability
	if err := s.ds.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *providerCapabilityStore) Add(
	ctx context.Context,
	data *iapiserver.ProviderCapability,
) (*iapiserver.ProviderCapability, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

type systemLLMConfigStore struct{ ds *datastore }

func newSystemLLMConfig(ds *datastore) *systemLLMConfigStore {
	return &systemLLMConfigStore{ds: ds}
}

func (s *systemLLMConfigStore) List(ctx context.Context) ([]*iapiserver.SystemLLMConfig, error) {
	var items []*iapiserver.SystemLLMConfig
	if err := s.ds.db.WithContext(ctx).Order("usage ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *systemLLMConfigStore) Upsert(
	ctx context.Context,
	data *iapiserver.SystemLLMConfig,
) (*iapiserver.SystemLLMConfig, error) {
	var existing iapiserver.SystemLLMConfig
	err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND usage = ?", data.OwnerUserID, data.Purpose).
		First(&existing).Error
	if err == nil {
		existing.Name = data.Name
		existing.ProviderID = data.ProviderID
		existing.ModelID = data.ModelID
		if err := s.ds.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		return &existing, nil
	}
	if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.WithStack(err)
	}
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *systemLLMConfigStore) DeleteByProviderID(ctx context.Context, providerID string) error {
	return errors.WithStack(
		s.ds.db.WithContext(ctx).Where("provider_id = ?", providerID).Delete(&iapiserver.SystemLLMConfig{}).Error,
	)
}

func (s *systemLLMConfigStore) DeleteByProviderModelID(ctx context.Context, providerID, modelID string) error {
	return errors.WithStack(
		s.ds.db.WithContext(ctx).
			Where("provider_id = ? AND model_id = ?", providerID, modelID).
			Delete(&iapiserver.SystemLLMConfig{}).Error,
	)
}

type storageBackendStore struct{ ds *datastore }

func newStorageBackend(ds *datastore) *storageBackendStore { return &storageBackendStore{ds: ds} }

func (s *storageBackendStore) List(
	ctx context.Context,
	req *iapiserver.StorageBackendListRequest,
) ([]*iapiserver.StorageBackend, int64, error) {
	var items []*iapiserver.StorageBackend
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		if req.Type != "" {
			q = q.Where("type = ?", req.Type)
		}
		if req.Enabled != nil {
			q = q.Where("enabled = ?", *req.Enabled)
		}
		return q
	}
	query := req.ToQuery(ctx, s.ds.db.Model(&iapiserver.StorageBackend{}), filter)
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *storageBackendStore) Get(ctx context.Context, id string) (*iapiserver.StorageBackend, error) {
	var item iapiserver.StorageBackend
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *storageBackendStore) Add(
	ctx context.Context,
	data *iapiserver.StorageBackend,
) (*iapiserver.StorageBackend, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *storageBackendStore) Update(
	ctx context.Context,
	data *iapiserver.StorageBackend,
) (*iapiserver.StorageBackend, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *storageBackendStore) GetDefaultLocal(ctx context.Context) (*iapiserver.StorageBackend, error) {
	var item iapiserver.StorageBackend
	err := s.ds.db.WithContext(ctx).
		Where("type = ? AND enabled = ? AND readonly = ?", iapiserver.StorageBackendTypeLocal, true, false).
		Order("created_at ASC").
		First(&item).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

type platformAssetStore struct{ ds *datastore }

func newPlatformAsset(ds *datastore) *platformAssetStore { return &platformAssetStore{ds: ds} }

func (s *platformAssetStore) List(
	ctx context.Context,
	req *iapiserver.AssetListRequest,
) ([]*iapiserver.Asset, int64, error) {
	var items []*iapiserver.Asset
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		if req.Status == "deleted" || (req.Deleted != nil && *req.Deleted) {
			q = q.Where("deleted_at > 0")
		} else {
			q = q.Where("(deleted_at = 0 OR deleted_at IS NULL)")
		}
		if req.MediaType != "" {
			q = q.Where("media_type = ?", req.MediaType)
		}
		if req.MimeType != "" {
			q = q.Where("mime_type = ?", req.MimeType)
		}
		if req.StorageBackendID != "" {
			q = q.Where("storage_backend_id = ?", req.StorageBackendID)
		}
		if req.SourceType != "" {
			q = q.Where("source_type = ?", req.SourceType)
		}
		if req.Format != "" {
			q = q.Where("format = ?", req.Format)
		}
		if req.MinSize > 0 {
			q = q.Where("size >= ?", req.MinSize)
		}
		if req.MaxSize > 0 {
			q = q.Where("size <= ?", req.MaxSize)
		}
		if req.Width > 0 {
			q = q.Where("width = ?", req.Width)
		}
		if req.Height > 0 {
			q = q.Where("height = ?", req.Height)
		}
		if req.MinWidth > 0 {
			q = q.Where("width >= ?", req.MinWidth)
		}
		if req.MaxWidth > 0 {
			q = q.Where("width <= ?", req.MaxWidth)
		}
		if req.MinHeight > 0 {
			q = q.Where("height >= ?", req.MinHeight)
		}
		if req.MaxHeight > 0 {
			q = q.Where("height <= ?", req.MaxHeight)
		}
		if req.MinDuration > 0 {
			q = q.Where("duration >= ?", req.MinDuration)
		}
		if req.MaxDuration > 0 {
			q = q.Where("duration <= ?", req.MaxDuration)
		}
		if len(req.Tags) > 0 {
			sub := s.ds.db.Table("asset_tags").
				Select("asset_tags.asset_id").
				Joins("JOIN tags ON tags.id = asset_tags.tag_id").
				Where("tags.name IN ?", req.Tags)
			q = q.Where("assets.id IN (?)", sub)
		}
		return q
	}
	query := req.ToQuery(ctx, s.ds.db.Model(&iapiserver.Asset{}), filter)
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *platformAssetStore) Get(ctx context.Context, id string) (*iapiserver.Asset, error) {
	var item iapiserver.Asset
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *platformAssetStore) Add(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *platformAssetStore) Update(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *platformAssetStore) Delete(ctx context.Context, id string) error {
	result := s.ds.db.WithContext(ctx).
		Model(&iapiserver.Asset{}).
		Where("id = ? AND (deleted_at = 0 OR deleted_at IS NULL)", id).
		Update("deleted_at", time.Now().UnixMilli())
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.Errorf("asset not found with id %v", id)
	}
	return nil
}

type assetThumbnailStore struct{ ds *datastore }

func newAssetThumbnail(ds *datastore) *assetThumbnailStore { return &assetThumbnailStore{ds: ds} }

func (s *assetThumbnailStore) GetByAsset(ctx context.Context, assetID string) (*iapiserver.AssetThumbnail, error) {
	var item iapiserver.AssetThumbnail
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ?", assetID).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *assetThumbnailStore) ListByAssetIDs(
	ctx context.Context,
	assetIDs []string,
) ([]*iapiserver.AssetThumbnail, error) {
	var items []*iapiserver.AssetThumbnail
	if len(assetIDs) == 0 {
		return items, nil
	}
	if err := s.ds.db.WithContext(ctx).Where("asset_id IN ?", assetIDs).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *assetThumbnailStore) Add(
	ctx context.Context,
	data *iapiserver.AssetThumbnail,
) (*iapiserver.AssetThumbnail, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *assetThumbnailStore) Update(
	ctx context.Context,
	data *iapiserver.AssetThumbnail,
) (*iapiserver.AssetThumbnail, error) {
	if err := s.ds.db.WithContext(ctx).Save(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *assetThumbnailStore) DeleteByAsset(ctx context.Context, assetID string) error {
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ?", assetID).Delete(&iapiserver.AssetThumbnail{}).Error; err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type tagStore struct{ ds *datastore }

func newTag(ds *datastore) *tagStore { return &tagStore{ds: ds} }

func (s *tagStore) ListByAssetIDs(ctx context.Context, assetIDs []string) (map[string][]*iapiserver.Tag, error) {
	result := map[string][]*iapiserver.Tag{}
	if len(assetIDs) == 0 {
		return result, nil
	}
	type row struct {
		AssetID string
		iapiserver.Tag
	}
	var rows []row
	if err := s.ds.db.WithContext(ctx).Table("asset_tags").
		Select("asset_tags.asset_id, tags.*").
		Joins("JOIN tags ON tags.id = asset_tags.tag_id").
		Where("asset_tags.asset_id IN ?", assetIDs).
		Scan(&rows).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	for _, r := range rows {
		tag := r.Tag
		result[r.AssetID] = append(result[r.AssetID], &tag)
	}
	return result, nil
}

func (s *tagStore) GetByName(ctx context.Context, name string, source string) (*iapiserver.Tag, error) {
	var item iapiserver.Tag
	if err := s.ds.db.WithContext(ctx).Where("name = ? AND source = ?", name, source).First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *tagStore) FirstOrCreate(ctx context.Context, data *iapiserver.Tag) (*iapiserver.Tag, error) {
	var item iapiserver.Tag
	err := s.ds.db.WithContext(ctx).Where("name = ? AND source = ?", data.Name, data.Source).First(&item).Error
	if err == nil {
		return &item, nil
	}
	if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.WithStack(err)
	}
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

type assetTagStore struct{ ds *datastore }

func newAssetTag(ds *datastore) *assetTagStore { return &assetTagStore{ds: ds} }

func (s *assetTagStore) Replace(ctx context.Context, assetID string, tags []*iapiserver.Tag, source string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("asset_id = ? AND source = ?", assetID, source).Delete(&iapiserver.AssetTag{}).Error; err != nil {
			return errors.WithStack(err)
		}
		for _, tag := range tags {
			link := &iapiserver.AssetTag{}
			link.Name = "asset-tag"
			link.AssetID = assetID
			link.TagID = tag.ID
			link.Source = source
			if err := tx.Create(link).Error; err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	})
}

func (s *assetTagStore) ListTagNames(ctx context.Context, assetID string) ([]string, error) {
	var names []string
	err := s.ds.db.WithContext(ctx).Table("asset_tags").
		Select("tags.name").
		Joins("JOIN tags ON tags.id = asset_tags.tag_id").
		Where("asset_tags.asset_id = ?", assetID).
		Scan(&names).Error
	return names, errors.WithStack(err)
}

func (s *assetTagStore) DeleteByAsset(ctx context.Context, assetID string) error {
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ?", assetID).Delete(&iapiserver.AssetTag{}).Error; err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type assetGroupStore struct{ ds *datastore }

func newAssetGroup(ds *datastore) *assetGroupStore { return &assetGroupStore{ds: ds} }

func (s *assetGroupStore) Add(ctx context.Context, data *iapiserver.AssetGroup) (*iapiserver.AssetGroup, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

type assetGroupMemberStore struct{ ds *datastore }

func newAssetGroupMember(ds *datastore) *assetGroupMemberStore {
	return &assetGroupMemberStore{ds: ds}
}

func (s *assetGroupMemberStore) BatchAdd(
	ctx context.Context,
	members []*iapiserver.AssetGroupMember,
) ([]*iapiserver.AssetGroupMember, error) {
	if len(members) == 0 {
		return members, nil
	}
	if err := s.ds.db.WithContext(ctx).CreateInBatches(members, 100).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return members, nil
}

func (s *assetGroupMemberStore) DeleteByAsset(ctx context.Context, assetID string) error {
	if err := s.ds.db.WithContext(ctx).Where("asset_id = ?", assetID).Delete(&iapiserver.AssetGroupMember{}).Error; err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type assetRelationStore struct{ ds *datastore }

func newAssetRelation(ds *datastore) *assetRelationStore { return &assetRelationStore{ds: ds} }

func (s *assetRelationStore) Add(
	ctx context.Context,
	data *iapiserver.AssetRelation,
) (*iapiserver.AssetRelation, error) {
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *assetRelationStore) DeleteByAsset(ctx context.Context, assetID string) error {
	if err := s.ds.db.WithContext(ctx).
		Where("source_asset_id = ? OR target_asset_id = ?", assetID, assetID).
		Delete(&iapiserver.AssetRelation{}).Error; err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type featureFlagStore struct{ ds *datastore }

func newFeatureFlag(ds *datastore) *featureFlagStore { return &featureFlagStore{ds: ds} }

func (s *featureFlagStore) List(ctx context.Context) ([]*iapiserver.FeatureFlag, error) {
	var items []*iapiserver.FeatureFlag
	if err := s.ds.db.WithContext(ctx).Order("key ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *featureFlagStore) Upsert(ctx context.Context, data *iapiserver.FeatureFlag) (*iapiserver.FeatureFlag, error) {
	var existing iapiserver.FeatureFlag
	err := s.ds.db.WithContext(ctx).Where("key = ?", data.Key).First(&existing).Error
	if err == nil {
		existing.Name = data.Name
		existing.Enabled = data.Enabled
		if err := s.ds.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		return &existing, nil
	}
	if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.WithStack(err)
	}
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

type roleStore struct{ ds *datastore }

func newRole(ds *datastore) *roleStore { return &roleStore{ds: ds} }

func (s *roleStore) List(ctx context.Context) ([]*iapiserver.Role, error) {
	var items []*iapiserver.Role
	if err := s.ds.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

type permissionStore struct{ ds *datastore }

func newPermission(ds *datastore) *permissionStore { return &permissionStore{ds: ds} }

func (s *permissionStore) List(ctx context.Context) ([]*iapiserver.Permission, error) {
	var items []*iapiserver.Permission
	if err := s.ds.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

type userRoleStore struct{ ds *datastore }

func newUserRole(ds *datastore) *userRoleStore { return &userRoleStore{ds: ds} }

func (s *userRoleStore) ListByUser(ctx context.Context, userID string) ([]*iapiserver.UserRole, error) {
	var items []*iapiserver.UserRole
	if err := s.ds.db.WithContext(ctx).Where("user_id = ?", userID).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}
