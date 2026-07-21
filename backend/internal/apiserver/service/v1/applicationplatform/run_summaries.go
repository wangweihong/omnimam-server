package applicationplatform

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// RunSummaryReader 提供 application-platform 所有的批量运行只读投影。
type RunSummaryReader struct {
	store store.ApplicationPlatformStore
}

// NewRunSummaryReader 创建只依赖 application-platform store 的受控摘要读取器。
func NewRunSummaryReader(source store.ApplicationPlatformStore) *RunSummaryReader {
	return &RunSummaryReader{store: source}
}

// GetApplicationRunSummaries 按 owner 批量返回非敏感运行身份与状态。
func (r *RunSummaryReader) GetApplicationRunSummaries(
	ctx context.Context,
	ownerUserID string,
	ids []string,
) (map[string]*iapiserver.RelatedResourceSummary, error) {
	result := make(map[string]*iapiserver.RelatedResourceSummary)
	if r == nil || r.store == nil || len(ids) == 0 {
		return result, nil
	}
	items, err := r.store.GetApplicationRunsByIDs(ctx, ownerUserID, ids)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		name := item.Name
		if name == "" {
			if application := snapshotValue[iapiserver.ApplicationSummary](item.CapabilitySourceSnapshot, "application"); application != nil {
				name = application.Name
			}
		}
		if name == "" {
			name = "Application run"
		}
		status := item.TaskCreationStatus
		if item.TaskStatusProjection != nil && *item.TaskStatusProjection != "" {
			status = *item.TaskStatusProjection
		}
		result[item.ID] = &iapiserver.RelatedResourceSummary{Type: "application_run", ID: item.ID, Name: name, Status: status}
	}
	return result, nil
}
