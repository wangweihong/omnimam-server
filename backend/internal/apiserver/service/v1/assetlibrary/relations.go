package assetlibrary

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// AtomicTaskSummaryReader 是 asset-library 消费的 Task Center 批量只读边界。
type AtomicTaskSummaryReader interface {
	GetAtomicTaskSummaries(context.Context, []string) (map[string]*iapiserver.AtomicTaskSummary, error)
}

// ApplicationRunSummaryReader 是 asset-library 消费的 application-platform 批量只读边界。
type ApplicationRunSummaryReader interface {
	GetApplicationRunSummaries(context.Context, string, []string) (map[string]*iapiserver.RelatedResourceSummary, error)
}

// CanvasRunSummaryReader 是 asset-library 消费的 workflow-canvas 批量只读边界。
type CanvasRunSummaryReader interface {
	GetCanvasRunSummaries(context.Context, string, []string) (map[string]*iapiserver.RelatedResourceSummary, error)
}

// RelationReaders 汇总 Artifact 跨领域关系的受控读取能力。
type RelationReaders struct {
	AtomicTasks     AtomicTaskSummaryReader
	ApplicationRuns ApplicationRunSummaryReader
	CanvasRuns      CanvasRunSummaryReader
}

func (s *service) attachArtifactRelations(ctx context.Context, ownerUserID string, items []*iapiserver.Artifact) error {
	if err := s.store.DecorateArtifacts(ctx, ownerUserID, items); err != nil {
		return err
	}
	taskIDs, applicationRunIDs, canvasRunIDs := []string{}, []string{}, []string{}
	for _, item := range items {
		if item == nil {
			continue
		}
		taskIDs = append(taskIDs, item.AtomicTaskID)
		applicationRunIDs = append(applicationRunIDs, item.ApplicationRunID)
		canvasRunIDs = append(canvasRunIDs, item.CanvasRunID)
		switch item.ProducerType {
		case "atomic_task":
			taskIDs = append(taskIDs, item.ProducerID)
		case "application_run":
			applicationRunIDs = append(applicationRunIDs, item.ProducerID)
		case "canvas_run":
			canvasRunIDs = append(canvasRunIDs, item.ProducerID)
		}
	}
	tasks := map[string]*iapiserver.RelatedResourceSummary{}
	if s.readers.AtomicTasks != nil {
		summaries, err := s.readers.AtomicTasks.GetAtomicTaskSummaries(ctx, uniqueRelationIDs(taskIDs))
		if err != nil {
			return err
		}
		for id, item := range summaries {
			if item != nil {
				tasks[id] = &iapiserver.RelatedResourceSummary{Type: "atomic_task", ID: item.ID, Name: item.Name, Status: item.Status}
			}
		}
	}
	applicationRuns := map[string]*iapiserver.RelatedResourceSummary{}
	if s.readers.ApplicationRuns != nil {
		var err error
		applicationRuns, err = s.readers.ApplicationRuns.GetApplicationRunSummaries(ctx, ownerUserID, uniqueRelationIDs(applicationRunIDs))
		if err != nil {
			return err
		}
	}
	canvasRuns := map[string]*iapiserver.RelatedResourceSummary{}
	if s.readers.CanvasRuns != nil {
		var err error
		canvasRuns, err = s.readers.CanvasRuns.GetCanvasRunSummaries(ctx, ownerUserID, uniqueRelationIDs(canvasRunIDs))
		if err != nil {
			return err
		}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.AtomicTask = tasks[item.AtomicTaskID]
		item.ApplicationRun = applicationRuns[item.ApplicationRunID]
		item.CanvasRun = canvasRuns[item.CanvasRunID]
		switch item.ProducerType {
		case "atomic_task":
			item.Producer = tasks[item.ProducerID]
		case "application_run":
			item.Producer = applicationRuns[item.ProducerID]
		case "canvas_run":
			item.Producer = canvasRuns[item.ProducerID]
		}
	}
	return nil
}

func uniqueRelationIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
