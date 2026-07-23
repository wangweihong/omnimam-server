package assetlibrary

import (
	"context"
	"fmt"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const defaultAssetMediaMetadataBackfillBatchSize = 100

// AssetMediaMetadataBackfillSummary 汇总一次历史零值素材回填，不包含内容或物理存储引用。
type AssetMediaMetadataBackfillSummary struct {
	Scanned int
	Updated int
	Failed  int
}

// AssetMediaMetadataBackfiller 复用 representation.inspect 能力修复 current version 的历史媒体元数据。
type AssetMediaMetadataBackfiller struct {
	store     store.AssetV1Store
	storage   ContentStorage
	inspector MediaMetadataInspector
}

func NewAssetMediaMetadataBackfiller(factory store.Factory, storage ContentStorage, inspector MediaMetadataInspector) *AssetMediaMetadataBackfiller {
	return &AssetMediaMetadataBackfiller{store: factory.AssetsV1(), storage: storage, inspector: inspector}
}

func (b *AssetMediaMetadataBackfiller) Run(ctx context.Context, batchSize int) (AssetMediaMetadataBackfillSummary, error) {
	if b == nil || b.store == nil || b.storage == nil || b.inspector == nil {
		return AssetMediaMetadataBackfillSummary{}, fmt.Errorf("asset media metadata backfill dependencies are unavailable")
	}
	if batchSize <= 0 || batchSize > 500 {
		batchSize = defaultAssetMediaMetadataBackfillBatchSize
	}
	executor := &RepresentationInspectExecutor{store: b.store, storage: b.storage, inspector: b.inspector}
	summary := AssetMediaMetadataBackfillSummary{}
	cursor := ""
	var firstFailure error
	for {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		candidates, err := b.store.ListAssetMediaMetadataBackfillCandidatesAfter(ctx, cursor, batchSize)
		if err != nil {
			return summary, fmt.Errorf("list asset media metadata backfill candidates: %w", err)
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			summary.Scanned++
			detail, detailErr := b.store.GetAssetVersionDetail(ctx, candidate.OwnerUserID, candidate.AssetVersionID)
			if detailErr == nil {
				detailErr = executor.inspectOriginalMedia(ctx, candidate.OwnerUserID, candidate.MediaType, detail)
			}
			if detailErr != nil {
				summary.Failed++
				if firstFailure == nil {
					firstFailure = fmt.Errorf("backfill asset %s: %w", candidate.AssetID, detailErr)
				}
				continue
			}
			summary.Updated++
		}
		cursor = candidates[len(candidates)-1].AssetID
		if len(candidates) < batchSize {
			break
		}
	}
	if summary.Failed > 0 {
		return summary, fmt.Errorf("asset media metadata backfill completed with %d failures; first failure: %w", summary.Failed, firstFailure)
	}
	return summary, nil
}
