package assetlibrary

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type relationAssetStore struct {
	store.AssetV1Store
	calls int
}

func (s *relationAssetStore) DecorateArtifacts(_ context.Context, owner string, items []*iapiserver.Artifact) error {
	s.calls++
	for _, item := range items {
		if item.OwnerUserID == owner && item.AssetID != "" {
			item.Asset = &iapiserver.UserAssetSummary{ID: item.AssetID, DisplayName: "Registered asset"}
		}
	}
	return nil
}

type taskSummaryReader struct{ calls int }

func (r *taskSummaryReader) GetAtomicTaskSummaries(_ context.Context, ids []string) (map[string]*iapiserver.AtomicTaskSummary, error) {
	r.calls++
	result := make(map[string]*iapiserver.AtomicTaskSummary)
	for _, id := range ids {
		if id != "missing-task" {
			result[id] = &iapiserver.AtomicTaskSummary{ID: id, Name: "Generate image", Status: "SUCCESS"}
		}
	}
	return result, nil
}

type applicationRunSummaryReader struct{ calls int }

func (r *applicationRunSummaryReader) GetApplicationRunSummaries(_ context.Context, owner string, ids []string) (map[string]*iapiserver.RelatedResourceSummary, error) {
	r.calls++
	result := make(map[string]*iapiserver.RelatedResourceSummary)
	for _, id := range ids {
		if owner == "user-a" && id != "missing-app" {
			result[id] = &iapiserver.RelatedResourceSummary{Type: "application_run", ID: id, Name: "Upscale", Status: "SUCCESS"}
		}
	}
	return result, nil
}

type canvasRunSummaryReader struct{ calls int }

func (r *canvasRunSummaryReader) GetCanvasRunSummaries(_ context.Context, owner string, ids []string) (map[string]*iapiserver.RelatedResourceSummary, error) {
	r.calls++
	result := make(map[string]*iapiserver.RelatedResourceSummary)
	for _, id := range ids {
		if owner == "user-a" && id != "missing-canvas" {
			result[id] = &iapiserver.RelatedResourceSummary{Type: "canvas_run", ID: id, Name: "Storyboard", Status: "RUNNING"}
		}
	}
	return result, nil
}

func TestAttachArtifactRelationsUsesFixedBatchesAndPreservesMissingIDs(t *testing.T) {
	assetStore := &relationAssetStore{}
	tasks := &taskSummaryReader{}
	applications := &applicationRunSummaryReader{}
	canvases := &canvasRunSummaryReader{}
	service := &service{store: assetStore, readers: RelationReaders{AtomicTasks: tasks, ApplicationRuns: applications, CanvasRuns: canvases}}
	items := make([]*iapiserver.Artifact, 0, 50)
	for i := 0; i < 50; i++ {
		items = append(items, &iapiserver.Artifact{OwnerUserID: "user-a", ProducerType: "application_run", ProducerID: "app-run", AtomicTaskID: "task", ApplicationRunID: "app-run", CanvasRunID: "canvas-run", AssetID: "asset"})
	}
	items[49].ProducerID = "missing-app"
	items[49].AtomicTaskID = "missing-task"
	items[49].CanvasRunID = "missing-canvas"

	if err := service.attachArtifactRelations(context.Background(), "user-a", items); err != nil {
		t.Fatalf("attach relations: %v", err)
	}
	if assetStore.calls != 1 || tasks.calls != 1 || applications.calls != 1 || canvases.calls != 1 {
		t.Fatalf("batch calls asset=%d task=%d app=%d canvas=%d", assetStore.calls, tasks.calls, applications.calls, canvases.calls)
	}
	if items[0].Producer == nil || items[0].Producer.Name != "Upscale" || items[0].AtomicTask == nil || items[0].CanvasRun == nil || items[0].Asset == nil {
		t.Fatalf("visible summaries = %#v", items[0])
	}
	if items[49].Producer != nil || items[49].AtomicTask != nil || items[49].CanvasRun != nil {
		t.Fatalf("missing summaries should be omitted = %#v", items[49])
	}
	if items[49].ProducerID != "missing-app" || items[49].AtomicTaskID != "missing-task" || items[49].CanvasRunID != "missing-canvas" {
		t.Fatalf("stable IDs changed = %#v", items[49])
	}
}
