package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// ListAssetMediaMetadataBackfillCandidatesAfter 按稳定 Asset ID 扫描 current version 仍缺尺寸或时长的媒体素材。
func (s *assetV1Store) ListAssetMediaMetadataBackfillCandidatesAfter(
	ctx context.Context,
	afterAssetID string,
	limit int,
) ([]store.AssetMediaMetadataBackfillCandidate, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var candidates []store.AssetMediaMetadataBackfillCandidate
	err := s.ds.db.WithContext(ctx).
		Table("user_assets AS a").
		Select("a.id AS asset_id, a.current_version_id AS asset_version_id, a.owner_user_id, a.media_type").
		Joins("JOIN asset_versions AS v ON v.id = a.current_version_id AND v.owner_user_id = a.owner_user_id AND v.deleted_at IS NULL").
		Where("a.id > ? AND a.deleted_at IS NULL", afterAssetID).
		Where(
			"(a.media_type = 'image' AND (COALESCE(a.width, 0) = 0 OR COALESCE(a.height, 0) = 0)) OR " +
				"(a.media_type = 'video' AND (COALESCE(a.width, 0) = 0 OR COALESCE(a.height, 0) = 0 OR COALESCE(a.duration_seconds, 0) = 0)) OR " +
				"(a.media_type = 'audio' AND COALESCE(a.duration_seconds, 0) = 0)",
		).
		Order("a.id ASC").
		Limit(limit).
		Scan(&candidates).Error
	return candidates, errors.WithStack(err)
}
