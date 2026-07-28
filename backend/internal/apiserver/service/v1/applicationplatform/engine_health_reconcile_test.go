package applicationplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type healthReconcileStore struct {
	store.ApplicationPlatformStore
	items []*iapiserver.EngineInstance
}

func (s *healthReconcileStore) ListEnabledEngineInstancesAfter(_ context.Context, cursor string, limit int) ([]*iapiserver.EngineInstance, error) {
	items := append([]*iapiserver.EngineInstance(nil), s.items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	result := make([]*iapiserver.EngineInstance, 0, limit)
	for _, item := range items {
		if item.Enabled && item.ID > cursor {
			result = append(result, item)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

type healthReconcileService struct {
	ApplicationPlatformSrv
	active  atomic.Int32
	maximum atomic.Int32
	failID  string
}

func (s *healthReconcileService) CheckEngineInstanceHealthInternal(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		maximum := s.maximum.Load()
		if active <= maximum || s.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	if id == s.failID {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	time.Sleep(5 * time.Millisecond)
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: id, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

func TestEngineHealthReconcileHonorsConcurrencyAndStableCursor(t *testing.T) {
	items := make([]*iapiserver.EngineInstance, 5)
	for i := range items {
		items[i] = &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthUnknown}
		items[i].ID = "engine-0" + string(rune('1'+i))
		items[i].Name = "Engine " + items[i].ID
	}
	applicationStore := &healthReconcileStore{items: items}
	service := &healthReconcileService{}
	handler := NewEngineHealthReconcileHandler(&executorFactory{applications: applicationStore}, service)
	result, err := handler.Reconcile(context.Background(), taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, MaxParallelism: 2, MaxItemsPerRun: 4, PerItemTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if service.maximum.Load() != 2 {
		t.Fatalf("maximum concurrency = %d, want 2", service.maximum.Load())
	}
	if result.Scanned != 4 || result.CycleCompleted {
		t.Fatalf("result = %#v", result)
	}
	if cursor := result.NextCheckpoint["engine_instance_id"]; cursor != "engine-04" {
		t.Fatalf("cursor = %v", cursor)
	}
	summary := decodeEngineHealthScheduleSummary(t, result)
	if len(summary.EngineInstances) != 4 || summary.EngineInstances[0].ID != "engine-01" || summary.EngineInstances[0].Name != "Engine engine-01" || summary.EngineInstances[0].ApplicationEngineTypeID != "comfyui" || summary.EngineInstances[0].HealthStatus != iapiserver.EngineHealthOnline {
		t.Fatalf("engine_instances = %#v", summary.EngineInstances)
	}
	if summary.EngineInstancesTotal != 4 || summary.EngineInstancesTruncated {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestEngineHealthReconcileDoesNotAdvanceIncompleteChunk(t *testing.T) {
	items := []*iapiserver.EngineInstance{}
	for i := 1; i <= 4; i++ {
		item := &iapiserver.EngineInstance{Enabled: true}
		item.ID = "engine-0" + string(rune('0'+i))
		items = append(items, item)
	}
	applicationStore := &healthReconcileStore{items: items}
	service := &healthReconcileService{failID: "engine-03"}
	handler := NewEngineHealthReconcileHandler(&executorFactory{applications: applicationStore}, service)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result, err := handler.Reconcile(ctx, taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, MaxParallelism: 2, MaxItemsPerRun: 4, PerItemTimeout: 20 * time.Millisecond})
	if err == nil {
		t.Fatal("expected incomplete chunk error")
	}
	if cursor := result.NextCheckpoint["engine_instance_id"]; cursor != "engine-02" {
		t.Fatalf("cursor = %v, want engine-02", cursor)
	}
	summary := decodeEngineHealthScheduleSummary(t, result)
	if len(summary.EngineInstances) == 0 || summary.EngineInstances[0].ID != "engine-03" || !summary.EngineInstances[0].Deferred {
		t.Fatalf("engine_instances = %#v", summary.EngineInstances)
	}
}

func TestEngineHealthReconcileBoundsAndPrioritizesDeferredInstances(t *testing.T) {
	items := make([]*iapiserver.EngineInstance, 22)
	for i := range items {
		items[i] = &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
		items[i].ID = fmt.Sprintf("engine-%02d", i+1)
		items[i].Name = "Engine " + items[i].ID
	}
	applicationStore := &healthReconcileStore{items: items}
	service := &healthReconcileService{failID: "engine-22"}
	handler := NewEngineHealthReconcileHandler(&executorFactory{applications: applicationStore}, service)
	result, err := handler.Reconcile(context.Background(), taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, MaxParallelism: 22, MaxItemsPerRun: 22, PerItemTimeout: 20 * time.Millisecond})
	if err == nil {
		t.Fatal("expected incomplete chunk error")
	}
	summary := decodeEngineHealthScheduleSummary(t, result)
	if len(summary.EngineInstances) != maxEngineHealthReconcileInstanceSummary || summary.EngineInstancesTotal != len(items) || !summary.EngineInstancesTruncated {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.EngineInstances[0].ID != "engine-22" || !summary.EngineInstances[0].Deferred {
		t.Fatalf("first engine instance = %#v", summary.EngineInstances[0])
	}
}

type engineHealthScheduleSummaryJSON struct {
	EngineInstances []struct {
		ID                      string `json:"id"`
		Name                    string `json:"name"`
		ApplicationEngineTypeID string `json:"application_engine_type_id"`
		HealthStatus            string `json:"health_status"`
		Deferred                bool   `json:"deferred"`
	} `json:"engine_instances"`
	EngineInstancesTotal     int  `json:"engine_instances_total"`
	EngineInstancesTruncated bool `json:"engine_instances_truncated"`
}

func decodeEngineHealthScheduleSummary(t *testing.T, result taskcenter.ReconcileResult) engineHealthScheduleSummaryJSON {
	t.Helper()
	schedule := &iapiserver.TaskSchedule{LastExecution: &iapiserver.TaskScheduleExecution{ReconcileSummary: iapiserver.ReconcileSummary{Scanned: result.Scanned, Findings: result.Findings, Deferred: result.Deferred, CycleCompleted: result.CycleCompleted, Summary: result.Summary}}}
	payload, err := json.Marshal(schedule)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		LastExecution struct {
			ReconcileSummary struct {
				Summary engineHealthScheduleSummaryJSON `json:"summary"`
			} `json:"reconcile_summary"`
		} `json:"last_execution"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	return response.LastExecution.ReconcileSummary.Summary
}
