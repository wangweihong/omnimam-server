package engine

import (
	"context"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type objectInfoReconcileStore struct {
	store.ApplicationPlatformStore
	items []*iapiserver.EngineInstance
}

func (s *objectInfoReconcileStore) ListRefreshableComfyUIEngineInstancesAfter(_ context.Context, cursor string, limit int) ([]*iapiserver.EngineInstance, error) {
	items := append([]*iapiserver.EngineInstance(nil), s.items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	result := make([]*iapiserver.EngineInstance, 0, limit)
	for _, item := range items {
		if item.ID > cursor && item.ApplicationEngineTypeID == "comfyui" && item.Enabled && item.HealthStatus == iapiserver.EngineHealthOnline {
			result = append(result, item)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

type objectInfoReconcileService struct {
	active  atomic.Int32
	maximum atomic.Int32
	failID  string
}

func (s *objectInfoReconcileService) RefreshComfyUIEngineObjectInfoInternal(_ context.Context, id string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		maximum := s.maximum.Load()
		if active <= maximum || s.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	if id == s.failID {
		return nil, context.DeadlineExceeded
	}
	return &iapiserver.ComfyUIEngineObjectInfoStatus{EngineInstanceID: id, Available: true}, nil
}

func TestComfyUIObjectInfoReconcileHonorsConcurrencyAndAdvancesPastFailures(t *testing.T) {
	items := make([]*iapiserver.EngineInstance, 4)
	for i := range items {
		items[i] = onlineComfyEngine()
		items[i].ID = "engine-0" + string(rune('1'+i))
	}
	storage := &objectInfoReconcileStore{items: items}
	service := &objectInfoReconcileService{failID: "engine-03"}
	handler := NewComfyUIObjectInfoReconcileHandler(&engineTestFactory{applications: storage}, service)
	result, err := handler.Reconcile(context.Background(), taskcenter.ReconcileRequest{Checkpoint: map[string]any{}, MaxParallelism: 2, MaxItemsPerRun: 4, PerItemTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if service.maximum.Load() != 2 || result.Scanned != 4 || result.Findings != 0 || result.Deferred != 1 || result.Summary["refreshed"] != 3 || !result.CycleCompleted {
		t.Fatalf("unexpected reconcile result=%#v max=%d", result, service.maximum.Load())
	}
	if cursor := result.NextCheckpoint["engine_instance_id"]; cursor != "" {
		t.Fatalf("cycle checkpoint was not reset: %v", cursor)
	}
}
