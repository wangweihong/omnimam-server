package applicationplatform

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const EngineHealthReconcileRef = "application-platform.engine-health"

// EngineHealthReconcileHandler 以稳定 EngineInstance ID 分块巡检；完整分块成功后才推进 checkpoint。
type EngineHealthReconcileHandler struct {
	store   store.ApplicationPlatformStore
	service ApplicationPlatformSrv
}

func NewEngineHealthReconcileHandler(factory store.Factory, service ApplicationPlatformSrv) *EngineHealthReconcileHandler {
	return &EngineHealthReconcileHandler{store: factory.ApplicationPlatforms(), service: service}
}

func (*EngineHealthReconcileHandler) Ref() string         { return EngineHealthReconcileRef }
func (*EngineHealthReconcileHandler) DisplayName() string { return "引擎健康巡检" }
func (*EngineHealthReconcileHandler) ValidateConfig(config map[string]any) error {
	if len(config) != 0 {
		return fmt.Errorf("engine health reconcile config does not accept custom fields")
	}
	return nil
}

func (h *EngineHealthReconcileHandler) Reconcile(ctx context.Context, req taskcenter.ReconcileRequest) (taskcenter.ReconcileResult, error) {
	cursor, _ := req.Checkpoint["engine_instance_id"].(string)
	items, err := h.store.ListEnabledEngineInstancesAfter(ctx, cursor, req.MaxItemsPerRun+1)
	if err != nil {
		return taskcenter.ReconcileResult{}, err
	}
	cycleCompleted := len(items) <= req.MaxItemsPerRun
	if len(items) > req.MaxItemsPerRun {
		items = items[:req.MaxItemsPerRun]
	}
	result := taskcenter.ReconcileResult{NextCheckpoint: map[string]any{"engine_instance_id": cursor}, CycleCompleted: cycleCompleted, Summary: map[string]any{}}
	for start := 0; start < len(items); start += req.MaxParallelism {
		end := min(start+req.MaxParallelism, len(items))
		chunk := items[start:end]
		type checked struct {
			changed bool
			err     error
		}
		results := make(chan checked, len(chunk))
		var wg sync.WaitGroup
		for _, engine := range chunk {
			engine := engine
			wg.Add(1)
			go func() {
				defer wg.Done()
				itemCtx, cancel := context.WithTimeout(ctx, req.PerItemTimeout)
				defer cancel()
				checkedResult, checkErr := h.service.CheckEngineInstanceHealthInternal(itemCtx, engine.ID)
				results <- checked{changed: checkErr == nil && checkedResult.HealthStatus != engine.HealthStatus, err: checkErr}
			}()
		}
		wg.Wait()
		close(results)
		chunkComplete := true
		for item := range results {
			if item.err != nil {
				chunkComplete = false
				result.Deferred++
				continue
			}
			result.Scanned++
			if item.changed {
				result.Findings++
			}
		}
		if !chunkComplete || ctx.Err() != nil {
			result.CycleCompleted = false
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			return result, fmt.Errorf("engine health reconcile chunk was incomplete")
		}
		result.NextCheckpoint = map[string]any{"engine_instance_id": chunk[len(chunk)-1].ID}
	}
	if cycleCompleted {
		result.NextCheckpoint = map[string]any{"engine_instance_id": ""}
	}
	result.Summary["checked_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return result, nil
}

var _ taskcenter.ReconcileHandler = (*EngineHealthReconcileHandler)(nil)
