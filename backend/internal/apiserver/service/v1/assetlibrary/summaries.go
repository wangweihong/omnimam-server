package assetlibrary

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// ArtifactSummaryReader 是 asset-library 对跨域消费者提供的 owner 裁剪批量投影适配器。
type ArtifactSummaryReader struct{ store store.AssetV1Store }

func NewArtifactSummaryReader(assetStore store.AssetV1Store) *ArtifactSummaryReader {
	return &ArtifactSummaryReader{store: assetStore}
}

// ResolveArtifactSummaries 最多解析 200 个 Artifact ID，不返回缺失、删除或其他 owner 目标的差异。
func (r *ArtifactSummaryReader) ResolveArtifactSummaries(ctx context.Context, owner string, ids []string) (map[string]*iapiserver.ArtifactReadableSummary, error) {
	if r == nil || r.store == nil || len(ids) == 0 {
		return map[string]*iapiserver.ArtifactReadableSummary{}, nil
	}
	if len(ids) > 200 {
		ids = ids[:200]
	}
	return r.store.ResolveArtifactSummaries(ctx, owner, ids)
}
