package postgresql

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func publishCanvasVersion(tx *gorm.DB, version *iapiserver.CanvasVersion, canvas *iapiserver.WorkflowCanvas) error {
	sourceID := fmt.Sprintf("%s:%d:published", version.ID, version.ResourceVersion)
	payload := map[string]any{
		"source_domain":               iapiserver.SSESourceDomainWorkflowCanvas,
		"source_event_id":             sourceID,
		"canvas_id":                   version.CanvasID,
		"canvas_version_id":           version.ID,
		"version":                     version.Version,
		"content_digest":              version.ContentDigest,
		"execution_template_digest":   version.ExecutionTemplateDigest,
		"workflow_definition_name":    version.WorkflowDefinitionName,
		"workflow_definition_version": version.WorkflowDefinitionVersion,
		"project_id":                  canvas.ProjectID,
		"namespace":                   canvas.Namespace,
		"published_by":                version.PublishedBy,
		"created_by":                  version.PublishedBy,
		"aggregate_version":           version.ResourceVersion,
		"occurred_at":                 version.PublishedAt,
	}
	return publishCanvasOutbox(tx, OutboxTopicCanvasVersionPublished, sourceID, "canvas_version", version.ID, version.ResourceVersion, payload)
}

func publishCanvasRunCreated(tx *gorm.DB, run *iapiserver.WorkflowCanvasRun) error {
	sourceID := fmt.Sprintf("%s:%d:created", run.ID, run.AggregateVersion)
	payload := canvasRunEventPayload(run)
	payload["source_event_id"] = sourceID
	payload["retry_of_canvas_run_id"] = run.RetryOfCanvasRunID
	payload["retry_intent"] = run.RetryIntent
	return publishCanvasOutbox(tx, OutboxTopicCanvasRunCreated, sourceID, "canvas_run", run.ID, run.AggregateVersion, payload)
}

func publishCanvasRunRetryCreated(tx *gorm.DB, run *iapiserver.WorkflowCanvasRun) error {
	if run.RetryOfCanvasRunID == nil || run.RetryIntent == nil {
		return nil
	}
	sourceID := run.ID + ":retry_created"
	payload := map[string]any{
		"source_domain":        iapiserver.SSESourceDomainWorkflowCanvas,
		"source_event_id":      sourceID,
		"source_canvas_run_id": *run.RetryOfCanvasRunID,
		"new_canvas_run_id":    run.ID,
		"retry_intent":         *run.RetryIntent,
		"created_by":           run.CreatedBy,
		"aggregate_version":    run.AggregateVersion,
		"occurred_at":          imachinery.Now(),
	}
	return publishCanvasOutbox(tx, OutboxTopicCanvasRunRetryCreated, sourceID, "canvas_run", run.ID, run.AggregateVersion, payload)
}

func publishCanvasRunCancelRequested(tx *gorm.DB, run *iapiserver.WorkflowCanvasRun) error {
	if run.DAGTaskGroupID == nil {
		return nil
	}
	sourceID := run.ID + ":cancel_requested"
	payload := map[string]any{
		"source_domain":     iapiserver.SSESourceDomainWorkflowCanvas,
		"source_event_id":   sourceID,
		"canvas_run_id":     run.ID,
		"dag_task_group_id": *run.DAGTaskGroupID,
		"requested_by":      run.CreatedBy,
		"reason":            nil,
		"created_by":        run.CreatedBy,
		"aggregate_version": run.AggregateVersion,
		"occurred_at":       imachinery.Now(),
	}
	return publishCanvasOutbox(tx, OutboxTopicCanvasRunCancelRequested, sourceID, "canvas_run", run.ID, run.AggregateVersion, payload)
}

func publishCanvasRunBound(tx *gorm.DB, run *iapiserver.WorkflowCanvasRun, bindingCount int) error {
	sourceID := fmt.Sprintf("%s:%s:bound", run.ID, derefString(run.DAGTaskGroupID))
	payload := canvasRunEventPayload(run)
	payload["source_event_id"] = sourceID
	payload["dag_task_group_id"] = run.DAGTaskGroupID
	payload["task_binding_count"] = bindingCount
	return publishCanvasOutbox(tx, OutboxTopicCanvasRunTaskGroupBound, sourceID, "canvas_run", run.ID, run.AggregateVersion, payload)
}

func publishCanvasRunChanged(tx *gorm.DB, previousStatus string, run *iapiserver.WorkflowCanvasRun) error {
	sourceID := fmt.Sprintf("%s:%d:status", run.ID, run.AggregateVersion)
	payload := canvasRunEventPayload(run)
	payload["source_event_id"] = sourceID
	payload["from_status"] = previousStatus
	return publishCanvasOutbox(tx, OutboxTopicCanvasRunStatusChanged, sourceID, "canvas_run", run.ID, run.AggregateVersion, payload)
}

func publishCanvasNodeChanged(tx *gorm.DB, previousStatus string, node *iapiserver.CanvasNodeRun) error {
	sourceID := fmt.Sprintf("%s:%d:status", node.ID, node.AggregateVersion)
	payload := map[string]any{
		"source_domain":      iapiserver.SSESourceDomainWorkflowCanvas,
		"source_event_id":    sourceID,
		"created_by":         "",
		"canvas_run_id":      node.CanvasRunID,
		"canvas_node_run_id": node.ID,
		"node_id":            node.NodeID,
		"execution_key":      node.ExecutionKey,
		"from_status":        previousStatus,
		"status":             node.Status,
		"status_reason":      node.StatusReason,
		"result_mode":        node.ResultMode,
		"progress":           node.Progress,
		"warnings":           node.Warnings,
		"last_error":         node.LastError,
		"aggregate_version":  node.AggregateVersion,
		"occurred_at":        imachinery.Now(),
	}
	var run iapiserver.WorkflowCanvasRun
	if err := tx.Select("created_by").Where("id = ?", node.CanvasRunID).First(&run).Error; err != nil {
		return err
	}
	payload["created_by"] = run.CreatedBy
	return publishCanvasOutbox(tx, OutboxTopicCanvasNodeRunStatusChanged, sourceID, "canvas_node_run", node.ID, node.AggregateVersion, payload)
}

func publishCanvasOutbox(tx *gorm.DB, topic, sourceID, aggregateType, aggregateID string, aggregateVersion int64, payload map[string]any) error {
	if err := publishOutbox(tx, topic, sourceID, payload); err != nil {
		return err
	}
	now := imachinery.Now()
	record := &iapiserver.WorkflowCanvasOutbox{
		EventName:        topic,
		AggregateType:    aggregateType,
		AggregateID:      aggregateID,
		AggregateVersion: aggregateVersion,
		Payload:          payload,
		DeliveryStatus:   "PUBLISHED",
		AttemptCount:     1,
		PublishedAt:      now,
	}
	record.ID = uuid.NewString()
	record.Name = sourceID
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
}

func canvasRunEventPayload(run *iapiserver.WorkflowCanvasRun) map[string]any {
	return map[string]any{
		"source_domain":          iapiserver.SSESourceDomainWorkflowCanvas,
		"created_by":             run.CreatedBy,
		"canvas_run_id":          run.ID,
		"canvas_id":              run.CanvasID,
		"canvas_version_id":      run.CanvasVersionID,
		"status":                 run.Status,
		"progress":               run.Progress,
		"task_creation_status":   run.TaskCreationStatus,
		"summary":                run.Summary,
		"changed_flow_summaries": []any{},
		"warnings":               run.Warnings,
		"aggregate_version":      run.AggregateVersion,
		"occurred_at":            imachinery.Now(),
	}
}
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
