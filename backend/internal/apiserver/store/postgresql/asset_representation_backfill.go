package postgresql

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func (s *assetV1Store) ListRepresentationBackfillCandidatesAfter(ctx context.Context, cursor string, limit int) ([]store.RepresentationBackfillCandidate, error) {
	if limit <= 0 {
		limit = 1000
	}
	var items []store.RepresentationBackfillCandidate
	query := `
SELECT
  a.id AS asset_id,
  v.id AS asset_version_id,
  v.owner_user_id,
  a.media_type,
  v.profile_version,
  COALESCE(t.status, '') AS representation_status,
  CASE WHEN tb.id IS NOT NULL AND tb.status = 'available' THEN TRUE ELSE FALSE END AS representation_blob_ok,
  COALESCE(t.retry_count, 0) AS retry_count,
  t.retry_after,
  EXISTS (
    SELECT 1
    FROM asset_representations src
    LEFT JOIN blobs sb ON sb.id = src.blob_id
    WHERE src.asset_version_id = v.id
      AND src.deleted_at IS NULL
      AND src.representation_type IN ('original', 'canonical')
      AND src.status = 'ready'
      AND (src.content_json <> '{}' OR (sb.id IS NOT NULL AND sb.status = 'available'))
  ) AS source_available
FROM asset_versions v
JOIN user_assets a ON a.id = v.asset_id
LEFT JOIN asset_representations t
  ON t.asset_version_id = v.id
 AND t.representation_type = 'thumbnail'
 AND t.profile = 'list-320'
 AND t.profile_version = v.profile_version
 AND t.deleted_at IS NULL
LEFT JOIN blobs tb ON tb.id = t.blob_id
WHERE v.deleted_at IS NULL
  AND a.deleted_at IS NULL
  AND a.status IN ('active', 'archived')
  AND a.media_type IN ('image', 'video')
  AND v.id > ?
ORDER BY v.id ASC
LIMIT ?`
	if err := s.ds.db.WithContext(ctx).Raw(query, cursor, limit).Scan(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *assetV1Store) PrepareRepresentationBackfill(ctx context.Context, owner, versionID string, expectedCount int) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var version iapiserver.AssetVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", versionID, owner).First(&version).Error; err != nil {
			return err
		}
		changed := false
		if version.ExpectedCount < expectedCount {
			version.ExpectedCount = expectedCount
			changed = true
		}
		if version.Status == iapiserver.AssetVersionStatusReady {
			version.Status = iapiserver.AssetVersionStatusProcessing
			changed = true
		}
		if changed {
			if err := tx.Save(&version).Error; err != nil {
				return err
			}
			if err := publishAssetVersionProcessingChanged(tx, &version, "processing", "", "", ""); err != nil {
				return err
			}
		}
		return tx.Model(&iapiserver.UserAsset{}).Where("id = ? AND owner_user_id = ? AND current_version_id = ? AND thumbnail_status <> 'ready'", version.AssetID, owner, version.ID).Update("thumbnail_status", "pending").Error
	}))
}
