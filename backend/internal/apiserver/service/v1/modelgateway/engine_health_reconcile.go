package modelgateway

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const (
	EngineHealthReconcileRef                = "application-platform.engine-health"
	maxEngineHealthReconcileInstanceSummary = 20
)

type engineHealthReconcileCheck struct {
	engine  *iapiserver.EngineInstance
	result  *iapiserver.EngineHealthCheckResult
	changed bool
	err     error
}

// engineHealthReconcileInstanceSummary 是计划详情使用的有界实例摘要，不包含连接地址或鉴权配置。
type engineHealthReconcileInstanceSummary struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	ApplicationEngineTypeID string `json:"application_engine_type_id"`
	Enabled                 bool   `json:"enabled"`
	PreviousHealthStatus    string `json:"previous_health_status"`
	HealthStatus            string `json:"health_status"`
	CheckedAt               string `json:"checked_at,omitempty"`
	FailureSummary          string `json:"failure_summary,omitempty"`
	Changed                 bool   `json:"changed"`
	Deferred                bool   `json:"deferred"`
}

// EngineHealthReconcileHandler 以稳定 EngineInstance ID 分块巡检；完整分块成功后才推进 checkpoint。
type EngineHealthReconcileHandler struct {
	store   store.ApplicationPlatformStore
	service EngineHealthChecker
}

// EngineHealthChecker 是健康巡检消费的最小引擎服务边界。
type EngineHealthChecker interface {
	CheckEngineInstanceHealthInternal(context.Context, string) (*iapiserver.EngineHealthCheckResult, error)
}

// NewEngineHealthReconcileHandler 构造由 Task Center 调度的引擎健康巡检处理器。
func NewEngineHealthReconcileHandler(factory store.Factory, service EngineHealthChecker) *EngineHealthReconcileHandler {
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
	checks := make([]engineHealthReconcileCheck, 0, len(items))
	for start := 0; start < len(items); start += req.MaxParallelism {
		end := min(start+req.MaxParallelism, len(items))
		chunk := items[start:end]
		results := make(chan engineHealthReconcileCheck, len(chunk))
		var wg sync.WaitGroup
		for _, engine := range chunk {
			engine := engine
			wg.Add(1)
			go func() {
				defer wg.Done()
				itemCtx, cancel := context.WithTimeout(ctx, req.PerItemTimeout)
				defer cancel()
				checkedResult, checkErr := h.service.CheckEngineInstanceHealthInternal(itemCtx, engine.ID)
				changed := checkErr == nil && checkedResult != nil && checkedResult.HealthStatus != engine.HealthStatus
				results <- engineHealthReconcileCheck{engine: engine, result: checkedResult, changed: changed, err: checkErr}
			}()
		}
		wg.Wait()
		close(results)
		chunkComplete := true
		for item := range results {
			checks = append(checks, item)
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
			updateEngineHealthReconcileSummary(&result, checks)
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
	updateEngineHealthReconcileSummary(&result, checks)
	return result, nil
}

func updateEngineHealthReconcileSummary(result *taskcenter.ReconcileResult, checks []engineHealthReconcileCheck) {
	result.Summary["checked_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	ordered := append([]engineHealthReconcileCheck(nil), checks...)
	sort.Slice(ordered, func(i, j int) bool {
		leftPriority, rightPriority := engineHealthReconcilePriority(ordered[i]), engineHealthReconcilePriority(ordered[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return ordered[i].engine.ID < ordered[j].engine.ID
	})
	total := len(ordered)
	if len(ordered) > maxEngineHealthReconcileInstanceSummary {
		ordered = ordered[:maxEngineHealthReconcileInstanceSummary]
	}
	instances := make([]engineHealthReconcileInstanceSummary, 0, len(ordered))
	for _, check := range ordered {
		healthStatus := check.engine.HealthStatus
		failureSummary, checkedAt := "", ""
		if check.result != nil {
			healthStatus = check.result.HealthStatus
			failureSummary = check.result.FailureSummary
			if !check.result.CheckedAt.IsZero() {
				checkedAt = check.result.CheckedAt.UTC().Format(time.RFC3339Nano)
			}
		} else if check.err != nil {
			failureSummary = "health check did not complete"
		}
		instances = append(instances, engineHealthReconcileInstanceSummary{
			ID:                      check.engine.ID,
			Name:                    check.engine.Name,
			ApplicationEngineTypeID: check.engine.ApplicationEngineTypeID,
			Enabled:                 check.engine.Enabled,
			PreviousHealthStatus:    check.engine.HealthStatus,
			HealthStatus:            healthStatus,
			CheckedAt:               checkedAt,
			FailureSummary:          failureSummary,
			Changed:                 check.changed,
			Deferred:                check.err != nil,
		})
	}
	result.Summary["engine_instances"] = instances
	result.Summary["engine_instances_total"] = total
	result.Summary["engine_instances_truncated"] = total > len(instances)
}

func engineHealthReconcilePriority(check engineHealthReconcileCheck) int {
	if check.err != nil {
		return 0
	}
	if check.changed {
		return 1
	}
	return 2
}

var _ taskcenter.ReconcileHandler = (*EngineHealthReconcileHandler)(nil)
