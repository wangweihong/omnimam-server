package applicationplatform

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type runSummaryStore struct {
	store.ApplicationPlatformStore
	calls int
}

func (s *runSummaryStore) GetApplicationRunsByIDs(_ context.Context, owner string, _ []string) ([]*iapiserver.ApplicationRun, error) {
	s.calls++
	status := "SUCCESS"
	return []*iapiserver.ApplicationRun{
		{ObjectMeta: imachinery.ObjectMeta{ID: "run-a"}, OwnerUserID: owner, TaskStatusProjection: &status, CapabilitySourceSnapshot: map[string]any{"application": map[string]any{"id": "app-a", "name": "Upscale", "visibility": "private"}}},
	}, nil
}

func TestRunSummaryReaderUsesOneOwnerScopedBatch(t *testing.T) {
	store := &runSummaryStore{}
	reader := NewRunSummaryReader(store)
	summaries, err := reader.GetApplicationRunSummaries(context.Background(), "user-a", []string{"run-a", "run-a", "missing"})
	if err != nil {
		t.Fatalf("get summaries: %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("batch calls = %d", store.calls)
	}
	if summaries["run-a"] == nil || summaries["run-a"].Name != "Upscale" || summaries["run-a"].Status != "SUCCESS" {
		t.Fatalf("summary = %#v", summaries["run-a"])
	}
}
