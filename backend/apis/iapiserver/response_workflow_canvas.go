package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type WorkflowActionResult struct {
	Success bool `json:"success"`
}

type WorkflowNodeDefinitionListItem struct {
	NodeType          string          `json:"node_type"`
	DefinitionVersion string          `json:"definition_version"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Category          string          `json:"category"`
	NodeKind          string          `json:"node_kind"`
	ExecutionMode     string          `json:"execution_mode"`
	RendererKey       *string         `json:"renderer_key"`
	AvailabilityScope string          `json:"availability_scope"`
	Deprecated        bool            `json:"deprecated"`
	CreatedAt         imachinery.Time `json:"created_at"`
}
type WorkflowCanvasListItem struct {
	CanvasID                 string          `json:"canvas_id"`
	Name                     string          `json:"name"`
	Description              string          `json:"description"`
	Visibility               string          `json:"visibility"`
	DraftRevision            int64           `json:"draft_revision"`
	LatestVersion            int             `json:"latest_version"`
	LatestPublishedVersionID *string         `json:"latest_published_version_id"`
	ProjectID                string          `json:"project_id"`
	Namespace                string          `json:"namespace"`
	CreatedBy                string          `json:"created_by"`
	CreatedAt                imachinery.Time `json:"created_at"`
	UpdatedAt                imachinery.Time `json:"updated_at"`
}
type CanvasVersionListItem struct {
	CanvasVersionID         string          `json:"canvas_version_id"`
	CanvasID                string          `json:"canvas_id"`
	Canvas                  *CanvasSummary  `json:"canvas"`
	Version                 int             `json:"version"`
	ContentDigest           string          `json:"content_digest"`
	ExecutionTemplateDigest string          `json:"execution_template_digest"`
	NodeCount               int             `json:"node_count"`
	EdgeCount               int             `json:"edge_count"`
	PublishedBy             string          `json:"published_by"`
	PublishedAt             imachinery.Time `json:"published_at"`
}
type WorkflowCanvasRunListItem struct {
	CanvasRunID        string                  `json:"canvas_run_id"`
	CanvasID           string                  `json:"canvas_id"`
	Canvas             *CanvasSummary          `json:"canvas"`
	CanvasVersionID    string                  `json:"canvas_version_id"`
	CanvasVersion      *CanvasVersionSummary   `json:"canvas_version"`
	DAGTaskGroupID     *string                 `json:"dag_task_group_id"`
	DAGTaskGroup       *DAGTaskGroupRefSummary `json:"dag_task_group"`
	TaskCreationStatus string                  `json:"task_creation_status"`
	Status             string                  `json:"status"`
	Progress           float64                 `json:"progress"`
	Summary            map[string]any          `json:"summary"`
	Warnings           []WorkflowRunWarning    `json:"warnings"`
	RetryOfCanvasRunID *string                 `json:"retry_of_canvas_run_id"`
	RetryOfCanvasRun   *CanvasRunSummary       `json:"retry_of_canvas_run"`
	RetryIntent        *string                 `json:"retry_intent"`
	AggregateVersion   int64                   `json:"aggregate_version"`
	ProjectID          string                  `json:"project_id"`
	Namespace          string                  `json:"namespace"`
	CreatedBy          string                  `json:"created_by"`
	CreatedAt          imachinery.Time         `json:"created_at"`
	FinishedAt         any                     `json:"finished_at"`
}

// MarshalJSON emits only the workflow-canvas S2 response fields instead of generic persistence metadata.
func (c WorkflowCanvas) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"canvas_id":                   c.ID,
			"name":                        c.Name,
			"description":                 c.Description,
			"visibility":                  c.Visibility,
			"draft_graph":                 c.DraftGraph,
			"draft_revision":              c.DraftRevision,
			"latest_version":              c.LatestVersion,
			"latest_published_version_id": c.LatestPublishedVersionID,
			"project_id":                  c.ProjectID,
			"namespace":                   c.Namespace,
			"created_by":                  c.CreatedBy,
			"resource_version":            c.ResourceVersion,
			"created_at":                  c.CreatedAt,
			"updated_at":                  c.UpdatedAt,
		},
	)
}

// MarshalJSON emits the immutable NodeDefinition contract without internal handlers or registration metadata.
func (d WorkflowNodeDefinition) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"node_type":                 d.NodeType,
			"definition_version":        d.DefinitionVersion,
			"title":                     d.Title,
			"description":               d.Description,
			"category":                  d.Category,
			"node_kind":                 d.NodeKind,
			"ports":                     d.Ports,
			"config_schema":             d.ConfigSchema,
			"controller_state_schema":   nullableMap(d.ControllerStateSchema),
			"controller_schema_version": d.ControllerSchemaVersion,
			"execution_binding":         d.ExecutionBinding,
			"renderer":                  d.Renderer,
			"cache_allowed":             d.CacheAllowed,
			"reuse_ttl_seconds":         d.ReuseTTLSeconds,
			"availability_scope":        d.AvailabilityScope,
			"project_id":                d.ProjectID,
			"namespace":                 d.Namespace,
			"deprecated":                d.Deprecated,
			"created_at":                d.CreatedAt,
		},
	)
}

// MarshalJSON emits the immutable CanvasVersion response and omits compiler-private data.
func (v CanvasVersion) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"canvas_version_id":           v.ID,
			"canvas_id":                   v.CanvasID,
			"canvas":                      v.Canvas,
			"version":                     v.Version,
			"graph_snapshot":              v.GraphSnapshot,
			"definition_snapshots":        v.DefinitionSnapshots,
			"input_schema":                v.InputSchema,
			"output_schema":               v.OutputSchema,
			"content_digest":              v.ContentDigest,
			"execution_template_digest":   v.ExecutionTemplateDigest,
			"workflow_definition_name":    v.WorkflowDefinitionName,
			"workflow_definition_version": v.WorkflowDefinitionVersion,
			"node_count":                  v.NodeCount,
			"edge_count":                  v.EdgeCount,
			"published_by":                v.PublishedBy,
			"published_at":                v.PublishedAt,
		},
	)
}

// MarshalJSON emits the fixed CanvasRun request and current aggregate projection.
func (r WorkflowCanvasRun) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"canvas_run_id":          r.ID,
			"canvas_id":              r.CanvasID,
			"canvas":                 r.Canvas,
			"canvas_version_id":      r.CanvasVersionID,
			"canvas_version":         r.CanvasVersion,
			"idempotency_key":        r.IdempotencyKey,
			"request_digest":         r.RequestDigest,
			"input_snapshot":         r.InputSnapshot,
			"scope":                  r.Scope,
			"run_policy":             r.RunPolicy,
			"execution_plan_digest":  r.ExecutionPlanDigest,
			"dag_task_group_id":      r.DAGTaskGroupID,
			"dag_task_group":         r.DAGTaskGroup,
			"task_creation_status":   r.TaskCreationStatus,
			"task_creation_attempts": r.TaskCreationAttempts,
			"status":                 r.Status,
			"progress":               r.Progress,
			"summary":                r.Summary,
			"result_summary":         r.ResultSummary,
			"warnings":               r.Warnings,
			"last_error":             nullableMap(r.LastError),
			"flow_runs":              r.FlowRuns,
			"retry_of_canvas_run_id": r.RetryOfCanvasRunID,
			"retry_of_canvas_run":    r.RetryOfCanvasRun,
			"retry_intent":           r.RetryIntent,
			"aggregate_version":      r.AggregateVersion,
			"project_id":             r.ProjectID,
			"namespace":              r.Namespace,
			"created_by":             r.CreatedBy,
			"created_at":             r.CreatedAt,
			"updated_at":             r.UpdatedAt,
			"started_at":             nullableTimeValue(&r.StartedAt),
			"finished_at":            nullableTimeValue(&r.FinishedAt),
		},
	)
}

func (r CanvasFlowRun) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"canvas_flow_run_id": r.ID,
			"canvas_run_id":      r.CanvasRunID,
			"flow_id":            r.FlowID,
			"name":               r.Name,
			"execution_keys":     r.ExecutionKeys,
			"status":             r.Status,
			"progress":           r.Progress,
			"summary":            r.Summary,
			"result_summary":     r.ResultSummary,
			"warnings":           r.Warnings,
			"aggregate_version":  r.AggregateVersion,
			"created_at":         r.CreatedAt,
			"updated_at":         r.UpdatedAt,
			"finished_at":        nullableTimeValue(&r.FinishedAt),
		},
	)
}

func (n CanvasNodeRun) MarshalJSON() ([]byte, error) {
	var reuse any
	if n.SourceCanvasRunID != nil && n.SourceCanvasNodeRunID != nil {
		reuse = map[string]any{
			"source_canvas_run_id":      n.SourceCanvasRunID,
			"source_canvas_node_run_id": n.SourceCanvasNodeRunID,
			"execution_fingerprint":     n.ExecutionFingerprint,
		}
	}
	return json.Marshal(
		map[string]any{
			"canvas_node_run_id":    n.ID,
			"canvas_run_id":         n.CanvasRunID,
			"node_id":               n.NodeID,
			"execution_key":         n.ExecutionKey,
			"node_type":             n.NodeType,
			"definition_version":    n.DefinitionVersion,
			"flow_ids":              n.FlowIDs,
			"execution_fingerprint": n.ExecutionFingerprint,
			"result_mode":           n.ResultMode,
			"status":                n.Status,
			"status_reason":         n.StatusReason,
			"progress":              n.Progress,
			"output_count":          n.OutputCount,
			"task_count":            n.TaskCount,
			"warnings":              n.Warnings,
			"last_error":            nullableMap(n.LastError),
			"reuse_source":          reuse,
			"aggregate_version":     n.AggregateVersion,
			"created_at":            n.CreatedAt,
			"updated_at":            n.UpdatedAt,
			"started_at":            nullableTimeValue(&n.StartedAt),
			"finished_at":           nullableTimeValue(&n.FinishedAt),
		},
	)
}

func (b CanvasNodeRunTaskBinding) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"binding_id":            b.ID,
			"dag_task_group_id":     b.DAGTaskGroupID,
			"atomic_task_id":        b.AtomicTaskID,
			"atomic_task":           b.AtomicTask,
			"task_child_key":        b.TaskChildKey,
			"binding_role":          b.BindingRole,
			"shard_key":             b.ShardKey,
			"shard_index":           b.ShardIndex,
			"task_resource_version": b.TaskResourceVersion,
		},
	)
}

func (b CanvasNodeRunOutputBinding) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"output_binding_id":   b.ID,
			"port_key":            b.PortKey,
			"required":            b.Required,
			"shard_key":           b.ShardKey,
			"shard_index":         b.ShardIndex,
			"producer_key":        b.ProducerKey,
			"atomic_task_id":      b.AtomicTaskID,
			"artifact_id":         b.ArtifactID,
			"artifact":            b.Artifact,
			"structured_value":    b.StructuredValue,
			"availability_status": b.AvailabilityStatus,
			"warning":             nullableMap(b.Warning),
			"aggregate_version":   b.AggregateVersion,
			"created_at":          b.CreatedAt,
			"updated_at":          b.UpdatedAt,
		},
	)
}

func (d CanvasNodeRunDetail) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(d.CanvasNodeRun)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	payload["task_bindings"] = d.TaskBindings
	payload["output_bindings"] = d.OutputBindings
	return json.Marshal(payload)
}

func nullableMap(value map[string]any) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
func nullableTimeValue(value *imachinery.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value
}
