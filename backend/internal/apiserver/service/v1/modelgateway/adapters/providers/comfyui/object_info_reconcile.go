package comfyui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const ComfyUIObjectInfoReconcileRef = "application-platform.comfyui-object-info-refresh"

// ComfyUIObjectInfoReconcileHandler 按稳定实例 ID 刷新所有符合资格的当前目录，不创建逐实例任务。
type ComfyUIObjectInfoReconcileHandler struct {
	store   store.ApplicationPlatformStore
	service ComfyUIObjectInfoRefresher
}

// ComfyUIObjectInfoRefresher 是目录刷新 reconciler 消费的最小引擎服务边界。
type ComfyUIObjectInfoRefresher interface {
	RefreshComfyUIEngineObjectInfoInternal(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error)
}

// NewComfyUIObjectInfoReconcileHandler 构造由 Task Center 调度的 ComfyUI 目录刷新处理器。
func NewComfyUIObjectInfoReconcileHandler(factory store.Factory, service ComfyUIObjectInfoRefresher) *ComfyUIObjectInfoReconcileHandler {
	return &ComfyUIObjectInfoReconcileHandler{store: factory.ApplicationPlatforms(), service: service}
}

func (*ComfyUIObjectInfoReconcileHandler) Ref() string         { return ComfyUIObjectInfoReconcileRef }
func (*ComfyUIObjectInfoReconcileHandler) DisplayName() string { return "ComfyUI object_info 刷新" }
func (*ComfyUIObjectInfoReconcileHandler) ValidateConfig(config map[string]any) error {
	if len(config) != 0 {
		return fmt.Errorf("comfyui object_info reconcile config does not accept custom fields")
	}
	return nil
}

func (h *ComfyUIObjectInfoReconcileHandler) Reconcile(ctx context.Context, req taskcenter.ReconcileRequest) (taskcenter.ReconcileResult, error) {
	cursor, _ := req.Checkpoint["engine_instance_id"].(string)
	items, err := h.store.ListRefreshableComfyUIEngineInstancesAfter(ctx, cursor, req.MaxItemsPerRun+1)
	if err != nil {
		return taskcenter.ReconcileResult{}, err
	}
	cycleCompleted := len(items) <= req.MaxItemsPerRun
	if len(items) > req.MaxItemsPerRun {
		items = items[:req.MaxItemsPerRun]
	}
	result := taskcenter.ReconcileResult{NextCheckpoint: map[string]any{"engine_instance_id": cursor}, CycleCompleted: cycleCompleted, Summary: map[string]any{}}
	refreshed := 0
	for start := 0; start < len(items); start += req.MaxParallelism {
		end := min(start+req.MaxParallelism, len(items))
		chunk := items[start:end]
		results := make(chan error, len(chunk))
		var wg sync.WaitGroup
		for _, engine := range chunk {
			engine := engine
			wg.Add(1)
			go func() {
				defer wg.Done()
				itemCtx, cancel := context.WithTimeout(ctx, req.PerItemTimeout)
				defer cancel()
				_, refreshErr := h.service.RefreshComfyUIEngineObjectInfoInternal(itemCtx, engine.ID)
				results <- refreshErr
			}()
		}
		wg.Wait()
		close(results)
		for refreshErr := range results {
			result.Scanned++
			if refreshErr != nil {
				result.Deferred++
				continue
			}
			refreshed++
		}
		if err := ctx.Err(); err != nil {
			result.CycleCompleted = false
			return result, err
		}
		result.NextCheckpoint = map[string]any{"engine_instance_id": chunk[len(chunk)-1].ID}
	}
	if cycleCompleted {
		result.NextCheckpoint = map[string]any{"engine_instance_id": ""}
	}
	result.Summary["refreshed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	result.Summary["refreshed"] = refreshed
	return result, nil
}

var _ taskcenter.ReconcileHandler = (*ComfyUIObjectInfoReconcileHandler)(nil)
