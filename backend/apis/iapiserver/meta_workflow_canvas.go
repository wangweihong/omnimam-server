package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	CanvasVisibilityPrivate   = "PRIVATE"
	CanvasVisibilityProject   = "PROJECT"
	CanvasAvailabilitySystem  = "SYSTEM"
	CanvasAvailabilityProject = "PROJECT"

	CanvasExecutionPassive     = "passive"
	CanvasExecutionCompileTime = "compile_time"
	CanvasExecutionAtomic      = "atomic"
	CanvasExecutionExpanded    = "expanded"

	CanvasCompilerMediaInput  = "builtin.media_input"
	CanvasCompilerPrompt      = "builtin.prompt"
	CanvasCompilerLoop        = "builtin.loop"
	CanvasCompilerPromptGroup = "builtin.prompt_group"
	CanvasCompilerOutput      = "builtin.output"

	CanvasRunScopeAll        = "all"
	CanvasRunScopeFlows      = "flows"
	CanvasRunScopeOnlyNodes  = "only_nodes"
	CanvasRunScopeUntilNodes = "until_nodes"
	CanvasRunScopeFromNodes  = "from_nodes"

	CanvasReuseRerunAll         = "rerun_all"
	CanvasReuseValidOutputs     = "reuse_valid_outputs"
	CanvasReuseRequired         = "reuse_required"
	CanvasFailureContinueFlows  = "continue_independent_flows"
	CanvasFailureFailFast       = "fail_fast"
	CanvasResultExecuted        = "executed"
	CanvasResultReused          = "reused"
	CanvasResultPassive         = "passive"
	CanvasResultClientGenerated = "client_generated"

	CanvasNodeTypeApplication = "APPLICATION"
	CanvasNodeTypeFunction    = "FUNCTION"
	CanvasNodeTypeDynamicFork = "DYNAMIC_FORK"

	CanvasRunStatusPending        = "PENDING"
	CanvasRunStatusRunning        = "RUNNING"
	CanvasRunStatusSuccess        = "SUCCESS"
	CanvasRunStatusPartialSuccess = "PARTIAL_SUCCESS"
	CanvasRunStatusFailed         = "FAILED"
	CanvasRunStatusCanceled       = "CANCELED"
	CanvasRunStatusTimeout        = "TIMEOUT"

	CanvasTaskCreationPending = "PENDING"
	CanvasTaskCreationCreated = "CREATED"
	CanvasTaskCreationFailed  = "RETRYABLE_FAILED"

	CanvasRetryFailed   = "retry_failed"
	CanvasRetryNode     = "retry_node"
	CanvasRetryFromNode = "retry_from_node"
	CanvasRetryFlow     = "retry_flow"
	CanvasRetryAll      = "rerun_all"
)

// WorkflowCanvasGraph 是草稿和发布版本共用的有向图快照。
type WorkflowCanvasGraph struct {
	Nodes    []WorkflowCanvasNode `json:"nodes"`
	Edges    []WorkflowCanvasEdge `json:"edges"`
	Flows    []WorkflowCanvasFlow `json:"flows"`
	Groups   []map[string]any     `json:"groups,omitempty"`
	Viewport map[string]any       `json:"viewport,omitempty"`
}

// WorkflowCanvasNode 只允许引用已发布 ApplicationVersion 或已注册 functionRef。
type WorkflowCanvasNode struct {
	NodeID            string         `json:"node_id,omitempty"`
	DefinitionVersion string         `json:"definition_version,omitempty"`
	Size              *CanvasSize    `json:"size,omitempty"`
	ControllerState   map[string]any `json:"controller_state,omitempty"`
	LiteralInputs     map[string]any `json:"literal_inputs,omitempty"`
	UIState           map[string]any `json:"ui_state,omitempty"`
	// Deprecated compatibility fields are accepted when reading pre-v1.7 drafts.
	NodeKey         string         `json:"node_key,omitempty"`
	NodeType        string         `json:"node_type"`
	Config          map[string]any `json:"config"`
	InputBindings   map[string]any `json:"input_bindings,omitempty"`
	Position        map[string]any `json:"position,omitempty"`
	MaxDynamicTasks *int           `json:"max_dynamic_tasks,omitempty"`
}

// WorkflowCanvasEdge 描述节点输出端口到下游输入端口的确定性绑定。
type WorkflowCanvasEdge struct {
	EdgeID         string         `json:"edge_id,omitempty"`
	SourceNodeID   string         `json:"source_node_id,omitempty"`
	SourcePortKey  string         `json:"source_port_key,omitempty"`
	TargetNodeID   string         `json:"target_node_id,omitempty"`
	TargetPortKey  string         `json:"target_port_key,omitempty"`
	ConnectionType string         `json:"connection_type,omitempty"`
	Order          *int           `json:"order,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
	Display        map[string]any `json:"display,omitempty"`
	// Deprecated compatibility fields are accepted when reading pre-v1.7 drafts.
	FromNodeKey string `json:"from_node_key,omitempty"`
	FromOutput  string `json:"from_output,omitempty"`
	ToNodeKey   string `json:"to_node_key,omitempty"`
	ToInput     string `json:"to_input,omitempty"`
}

// CanvasPosition 保存画布节点位置，不参与运行时调度。
type CanvasPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CanvasSize 保存可选节点展示尺寸。
type CanvasSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// WorkflowCanvasFlow 定义可独立选择运行的显式业务流。
type WorkflowCanvasFlow struct {
	FlowID        string         `json:"flow_id"`
	Name          string         `json:"name"`
	EntryNodeIDs  []string       `json:"entry_node_ids"`
	OutputNodeIDs []string       `json:"output_node_ids"`
	RunPolicy     map[string]any `json:"run_policy,omitempty"`
}

// WorkflowCanvas 保存可编辑草稿；发布后通过新版本固化，不修改历史版本。
type WorkflowCanvas struct {
	imachinery.ObjectMeta    `json:"-"`
	CanvasID                 string              `json:"canvas_id"                             gorm:"-"`
	Visibility               string              `json:"visibility"                            gorm:"column:visibility;type:varchar(16);not null"`
	DraftGraph               WorkflowCanvasGraph `json:"draft_graph"                           gorm:"-"`
	DraftGraphJSON           string              `json:"-"                                     gorm:"column:draft_graph_json;type:text;not null"`
	DraftRevision            int64               `json:"draft_revision"                        gorm:"column:draft_revision;not null;default:1"`
	LatestVersion            int                 `json:"latest_version"                        gorm:"column:latest_version;not null;default:0"`
	LatestPublishedVersionID *string             `json:"latest_published_version_id,omitempty" gorm:"column:latest_published_version_id;type:varchar(64)"`
	ProjectID                string              `json:"project_id"                            gorm:"column:project_id;type:varchar(128);not null;index"`
	Namespace                string              `json:"namespace"                             gorm:"column:namespace;type:varchar(128);not null;index"`
	CreatedBy                string              `json:"created_by"                            gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt                imachinery.Time     `json:"-"                                     gorm:"column:deleted_at;index"`
}

func (WorkflowCanvas) TableName() string { return "canvases" }
func (c *WorkflowCanvas) BeforeCreate(tx *gorm.DB) error {
	if err := c.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	c.CanvasID = c.ID
	return marshalCanvasJSON(c.DraftGraph, &c.DraftGraphJSON, `{"nodes":[],"edges":[]}`)
}
func (c *WorkflowCanvas) AfterCreate(*gorm.DB) error { c.CanvasID = c.ID; return nil }
func (c *WorkflowCanvas) BeforeUpdate(tx *gorm.DB) error {
	if err := c.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalCanvasJSON(c.DraftGraph, &c.DraftGraphJSON, `{"nodes":[],"edges":[]}`)
}
func (c *WorkflowCanvas) AfterUpdate(*gorm.DB) error { c.CanvasID = c.ID; return nil }
func (c *WorkflowCanvas) AfterFind(tx *gorm.DB) error {
	if err := c.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	c.CanvasID = c.ID
	return unmarshalCanvasJSON(c.DraftGraphJSON, &c.DraftGraph)
}

// CanvasVersion 固化发布图、内容摘要和 Task Center 运行时定义绑定。
type CanvasVersion struct {
	imachinery.ObjectMeta     `json:"-"`
	CanvasVersionID           string                    `json:"canvas_version_id"           gorm:"-"`
	CanvasID                  string                    `json:"canvas_id"                   gorm:"column:canvas_id;type:varchar(64);not null;uniqueIndex:idx_canvas_version_number,priority:1;uniqueIndex:idx_canvas_version_digest,priority:1"`
	Version                   int                       `json:"version"                     gorm:"column:version;not null;uniqueIndex:idx_canvas_version_number,priority:2"`
	GraphSnapshot             WorkflowCanvasGraph       `json:"graph_snapshot"              gorm:"-"`
	GraphSnapshotJSON         string                    `json:"-"                           gorm:"column:graph_snapshot_json;type:text;not null"`
	DefinitionSnapshots       []*WorkflowNodeDefinition `json:"definition_snapshots"        gorm:"-"`
	DefinitionSnapshotsJSON   string                    `json:"-"                           gorm:"column:definition_snapshots_json;type:text;not null;default:'[]'"`
	InputSchema               map[string]any            `json:"input_schema"                gorm:"-"`
	InputSchemaJSON           string                    `json:"-"                           gorm:"column:input_schema_json;type:text;not null"`
	OutputSchema              map[string]any            `json:"output_schema"               gorm:"-"`
	OutputSchemaJSON          string                    `json:"-"                           gorm:"column:output_schema_json;type:text;not null"`
	ContentDigest             string                    `json:"content_digest"              gorm:"column:content_digest;type:varchar(80);not null;uniqueIndex:idx_canvas_version_digest,priority:2"`
	ExecutionTemplateDigest   string                    `json:"execution_template_digest"   gorm:"column:execution_template_digest;type:varchar(80);not null;default:''"`
	WorkflowDefinitionName    string                    `json:"workflow_definition_name"    gorm:"column:workflow_definition_name;type:varchar(256);not null;default:''"`
	WorkflowDefinitionVersion string                    `json:"workflow_definition_version" gorm:"column:workflow_definition_version;type:varchar(64);not null;default:''"`
	CompileSummary            map[string]any            `json:"-"                           gorm:"-"`
	CompileSummaryJSON        string                    `json:"-"                           gorm:"column:compile_summary_json;type:text;not null;default:'{}'"`
	// CompiledDefinition fields keep source compatibility with pre-v1.7 callers.
	CompiledDefinitionName    string          `json:"-"                           gorm:"-"`
	CompiledDefinitionVersion int             `json:"-"                           gorm:"-"`
	NodeCount                 int             `json:"node_count"                  gorm:"column:node_count;not null"`
	EdgeCount                 int             `json:"edge_count"                  gorm:"column:edge_count;not null"`
	PublishedBy               string          `json:"published_by"                gorm:"column:published_by;type:varchar(128);not null"`
	PublishedAt               imachinery.Time `json:"published_at"                gorm:"column:published_at;not null"`
	// Canvas 是当前主体可见的一跳画布摘要，不包含草稿图。
	Canvas *CanvasSummary `json:"canvas,omitempty"            gorm:"-"`
}

// CanvasSummary 是 CanvasVersion 和 CanvasRun 返回的一跳画布摘要。
type CanvasSummary struct {
	CanvasID   string `json:"canvas_id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
}

// CanvasVersionSummary 是 CanvasRun 固定的不可变版本摘要。
type CanvasVersionSummary struct {
	CanvasVersionID string          `json:"canvas_version_id"`
	CanvasID        string          `json:"canvas_id"`
	Version         int             `json:"version"`
	ContentDigest   string          `json:"content_digest"`
	PublishedAt     imachinery.Time `json:"published_at"`
}

// CanvasRunSummary 是手动重跑直接来源的一跳摘要。
type CanvasRunSummary struct {
	CanvasRunID string          `json:"canvas_run_id"`
	Status      string          `json:"status"`
	Progress    float64         `json:"progress"`
	CreatedAt   imachinery.Time `json:"created_at"`
}

// DAGTaskGroupRefSummary 复用 Task Center 的 DAGTaskGroup 一跳摘要。
type DAGTaskGroupRefSummary = DAGTaskGroupSummary

func (CanvasVersion) TableName() string { return "canvas_versions" }
func (v *CanvasVersion) BeforeCreate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	v.CanvasVersionID = v.ID
	return v.marshal()
}
func (v *CanvasVersion) AfterCreate(*gorm.DB) error { v.CanvasVersionID = v.ID; return nil }
func (v *CanvasVersion) BeforeUpdate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (v *CanvasVersion) AfterUpdate(*gorm.DB) error { v.CanvasVersionID = v.ID; return nil }
func (v *CanvasVersion) AfterFind(tx *gorm.DB) error {
	if err := v.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	v.CanvasVersionID = v.ID
	return v.unmarshal()
}
func (v *CanvasVersion) marshal() error {
	if err := marshalCanvasJSON(v.GraphSnapshot, &v.GraphSnapshotJSON, `{"nodes":[],"edges":[]}`); err != nil {
		return err
	}
	if err := marshalCanvasJSON(v.InputSchema, &v.InputSchemaJSON, `{}`); err != nil {
		return err
	}
	if err := marshalCanvasJSON(v.OutputSchema, &v.OutputSchemaJSON, `{}`); err != nil {
		return err
	}
	if err := marshalCanvasJSON(v.DefinitionSnapshots, &v.DefinitionSnapshotsJSON, `[]`); err != nil {
		return err
	}
	return marshalCanvasJSON(v.CompileSummary, &v.CompileSummaryJSON, `{}`)
}
func (v *CanvasVersion) unmarshal() error {
	if err := unmarshalCanvasJSON(v.GraphSnapshotJSON, &v.GraphSnapshot); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(v.InputSchemaJSON, &v.InputSchema); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(v.OutputSchemaJSON, &v.OutputSchema); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(v.DefinitionSnapshotsJSON, &v.DefinitionSnapshots); err != nil {
		return err
	}
	return unmarshalCanvasJSON(v.CompileSummaryJSON, &v.CompileSummary)
}

// WorkflowCanvasRun 是固定 CanvasVersion 的业务运行投影。
type WorkflowCanvasRun struct {
	imachinery.ObjectMeta       `json:"-"`
	CanvasRunID                 string               `json:"canvas_run_id"                 gorm:"-"`
	CanvasID                    string               `json:"canvas_id"                     gorm:"column:canvas_id;type:varchar(64);not null;index"`
	CanvasVersionID             string               `json:"canvas_version_id"             gorm:"column:canvas_version_id;type:varchar(64);not null;index"`
	IdempotencyKey              string               `json:"idempotency_key"               gorm:"column:idempotency_key;type:varchar(200);not null;uniqueIndex:idx_canvas_run_idempotency,priority:4"`
	RequestDigest               string               `json:"request_digest"                gorm:"column:request_digest;type:varchar(80);not null"`
	InputSnapshot               map[string]any       `json:"input_snapshot"                gorm:"-"`
	InputSnapshotJSON           string               `json:"-"                             gorm:"column:input_snapshot_json;type:text;not null"`
	Scope                       WorkflowRunScope     `json:"scope"                         gorm:"-"`
	ScopeJSON                   string               `json:"-"                             gorm:"column:scope_json;type:text;not null;default:'{}'"`
	RunPolicy                   WorkflowRunPolicy    `json:"run_policy"                    gorm:"-"`
	RunPolicyJSON               string               `json:"-"                             gorm:"column:run_policy_json;type:text;not null;default:'{}'"`
	ReuseDecisions              []map[string]any     `json:"-"                             gorm:"-"`
	ReuseDecisionsJSON          string               `json:"-"                             gorm:"column:reuse_decisions_json;type:text;not null;default:'[]'"`
	ExecutionPlan               map[string]any       `json:"-"                             gorm:"-"`
	ExecutionPlanJSON           string               `json:"-"                             gorm:"column:execution_plan_json;type:text;not null;default:'{}'"`
	ExecutionPlanDigest         string               `json:"execution_plan_digest"         gorm:"column:execution_plan_digest;type:varchar(80);not null;default:''"`
	DAGTaskGroupID              *string              `json:"dag_task_group_id"             gorm:"column:dag_task_group_id;type:varchar(64);uniqueIndex"`
	TaskCreationStatus          string               `json:"task_creation_status"          gorm:"column:task_creation_status;type:varchar(16);not null"`
	TaskCreationAttempts        int                  `json:"task_creation_attempts"        gorm:"column:task_creation_attempts;not null;default:0"`
	NextTaskCreationRetryAt     imachinery.Time      `json:"-"                             gorm:"column:next_task_creation_retry_at"`
	Status                      string               `json:"status"                        gorm:"column:status;type:varchar(32);not null;index"`
	Progress                    float64              `json:"progress"                      gorm:"column:progress;not null;default:0"`
	Summary                     map[string]any       `json:"summary"                       gorm:"-"`
	SummaryJSON                 string               `json:"-"                             gorm:"column:summary_json;type:text;not null"`
	ResultSummary               map[string]any       `json:"result_summary"                gorm:"-"`
	ResultSummaryJSON           string               `json:"-"                             gorm:"column:result_summary_json;type:text;not null;default:'{}'"`
	Warnings                    []WorkflowRunWarning `json:"warnings"                      gorm:"-"`
	WarningsJSON                string               `json:"-"                             gorm:"column:warnings_json;type:text;not null;default:'[]'"`
	LastError                   map[string]any       `json:"last_error"                    gorm:"-"`
	LastErrorJSON               string               `json:"-"                             gorm:"column:last_error_json;type:text"`
	DAGTaskGroupResourceVersion int64                `json:"-"                             gorm:"column:dag_task_group_resource_version;not null;default:0"`
	AggregateVersion            int64                `json:"aggregate_version"             gorm:"column:aggregate_version;not null;default:0"`
	RetryOfCanvasRunID          *string              `json:"retry_of_canvas_run_id"        gorm:"column:retry_of_canvas_run_id;type:varchar(64)"`
	RetryIntent                 *string              `json:"retry_intent,omitempty"        gorm:"column:retry_intent;type:varchar(32)"`
	ProjectID                   string               `json:"project_id"                    gorm:"column:project_id;type:varchar(128);not null;uniqueIndex:idx_canvas_run_idempotency,priority:1"`
	Namespace                   string               `json:"namespace"                     gorm:"column:namespace;type:varchar(128);not null;uniqueIndex:idx_canvas_run_idempotency,priority:2"`
	CreatedBy                   string               `json:"created_by"                    gorm:"column:created_by;type:varchar(128);not null;uniqueIndex:idx_canvas_run_idempotency,priority:3"`
	FinishedAt                  imachinery.Time      `json:"finished_at,omitempty"         gorm:"column:finished_at"`
	StartedAt                   imachinery.Time      `json:"started_at,omitempty"          gorm:"column:started_at"`
	// Canvas 与 CanvasVersion 优先来自运行创建快照，旧数据才回查当前同域资源。
	Canvas        *CanvasSummary        `json:"canvas,omitempty"              gorm:"-"`
	CanvasVersion *CanvasVersionSummary `json:"canvas_version,omitempty"      gorm:"-"`
	// DAGTaskGroup 由 Task Center 受控批量读取，不包含节点和运行时配置。
	DAGTaskGroup *DAGTaskGroupRefSummary `json:"dag_task_group,omitempty"      gorm:"-"`
	// RetryOfCanvasRun 是手动重跑直接来源摘要，不递归展开重跑链。
	RetryOfCanvasRun *CanvasRunSummary `json:"retry_of_canvas_run,omitempty" gorm:"-"`
	FlowRuns         []*CanvasFlowRun  `json:"flow_runs"                     gorm:"-"`
}

func (WorkflowCanvasRun) TableName() string { return "canvas_runs" }
func (r *WorkflowCanvasRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	r.CanvasRunID = r.ID
	return r.marshal()
}
func (r *WorkflowCanvasRun) AfterCreate(*gorm.DB) error { r.CanvasRunID = r.ID; return nil }
func (r *WorkflowCanvasRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (r *WorkflowCanvasRun) AfterUpdate(*gorm.DB) error { r.CanvasRunID = r.ID; return nil }
func (r *WorkflowCanvasRun) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	r.CanvasRunID = r.ID
	return r.unmarshal()
}
func (r *WorkflowCanvasRun) marshal() error {
	for _, f := range []struct {
		v any
		p *string
	}{{r.InputSnapshot, &r.InputSnapshotJSON}, {r.Scope, &r.ScopeJSON}, {r.RunPolicy, &r.RunPolicyJSON}, {r.ReuseDecisions, &r.ReuseDecisionsJSON}, {r.ExecutionPlan, &r.ExecutionPlanJSON}, {r.Summary, &r.SummaryJSON}, {r.ResultSummary, &r.ResultSummaryJSON}, {r.Warnings, &r.WarningsJSON}, {r.LastError, &r.LastErrorJSON}} {
		if err := marshalCanvasJSON(f.v, f.p, `{}`); err != nil {
			return err
		}
	}
	return nil
}
func (r *WorkflowCanvasRun) unmarshal() error {
	for _, f := range []struct {
		s string
		p any
	}{{r.InputSnapshotJSON, &r.InputSnapshot}, {r.ScopeJSON, &r.Scope}, {r.RunPolicyJSON, &r.RunPolicy}, {r.ReuseDecisionsJSON, &r.ReuseDecisions}, {r.ExecutionPlanJSON, &r.ExecutionPlan}, {r.SummaryJSON, &r.Summary}, {r.ResultSummaryJSON, &r.ResultSummary}, {r.WarningsJSON, &r.Warnings}, {r.LastErrorJSON, &r.LastError}} {
		if err := unmarshalCanvasJSON(f.s, f.p); err != nil {
			return err
		}
	}
	return nil
}

// CanvasNodeRun 将画布节点稳定映射到 Task Center AtomicTask 投影。
type CanvasNodeRun struct {
	imachinery.ObjectMeta     `json:"-"`
	CanvasNodeRunID           string               `json:"canvas_node_run_id"                  gorm:"-"`
	CanvasRunID               string               `json:"canvas_run_id"                       gorm:"column:canvas_run_id;type:varchar(64);not null;uniqueIndex:idx_canvas_node_run_key,priority:1;index"`
	NodeID                    string               `json:"node_id"                             gorm:"column:node_id;type:varchar(100);not null;default:'';index"`
	ExecutionKey              string               `json:"execution_key"                       gorm:"column:execution_key;type:varchar(200);not null;default:''"`
	NodeKey                   string               `json:"-"                                   gorm:"-"`
	NodeType                  string               `json:"node_type"                           gorm:"column:node_type;type:varchar(32);not null"`
	DefinitionVersion         string               `json:"definition_version"                  gorm:"column:definition_version;type:varchar(64);not null;default:'legacy'"`
	ExecutionFingerprint      string               `json:"execution_fingerprint"               gorm:"column:execution_fingerprint;type:varchar(80);not null;default:'';index"`
	ResolvedInputSnapshot     map[string]any       `json:"-"                                   gorm:"-"`
	ResolvedInputSnapshotJSON string               `json:"-"                                   gorm:"column:resolved_input_snapshot_json;type:text;not null;default:'{}'"`
	ResultMode                string               `json:"result_mode"                         gorm:"column:result_mode;type:varchar(32);not null;default:'executed'"`
	Status                    string               `json:"status"                              gorm:"column:status;type:varchar(32);not null;index"`
	StatusReason              *string              `json:"status_reason,omitempty"             gorm:"column:status_reason;type:text"`
	Progress                  float64              `json:"progress"                            gorm:"column:progress;not null;default:0"`
	TaskCount                 int                  `json:"task_count"                          gorm:"column:task_count;not null;default:0"`
	OutputCount               int                  `json:"output_count"                        gorm:"column:output_count;not null;default:0"`
	RequiredOutputCount       int                  `json:"-"                                   gorm:"column:required_output_count;not null;default:0"`
	ReadyRequiredOutputCount  int                  `json:"-"                                   gorm:"column:ready_required_output_count;not null;default:0"`
	Warnings                  []WorkflowRunWarning `json:"warnings"                            gorm:"-"`
	WarningsJSON              string               `json:"-"                                   gorm:"column:warnings_json;type:text;not null;default:'[]'"`
	LastError                 map[string]any       `json:"last_error"                          gorm:"-"`
	LastErrorJSON             string               `json:"-"                                   gorm:"column:last_error_json;type:text"`
	SourceCanvasRunID         *string              `json:"source_canvas_run_id,omitempty"      gorm:"column:source_canvas_run_id;type:varchar(64)"`
	SourceCanvasNodeRunID     *string              `json:"source_canvas_node_run_id,omitempty" gorm:"column:source_canvas_node_run_id;type:varchar(64)"`
	AggregateVersion          int64                `json:"aggregate_version"                   gorm:"column:aggregate_version;not null;default:0"`
	StartedAt                 imachinery.Time      `json:"started_at,omitempty"                gorm:"column:started_at"`
	FinishedAt                imachinery.Time      `json:"finished_at,omitempty"               gorm:"column:finished_at"`
	FlowIDs                   []string             `json:"flow_ids"                            gorm:"-"`
	ReuseSource               *CanvasRunSummary    `json:"reuse_source,omitempty"              gorm:"-"`
	// Deprecated compatibility fields for older relation tests.
	AtomicTaskID *string               `json:"-"                                   gorm:"-"`
	AtomicTask   *AtomicTaskRefSummary `json:"-"                                   gorm:"-"`
}

func (CanvasNodeRun) TableName() string { return "canvas_node_runs" }
func (n *CanvasNodeRun) BeforeCreate(tx *gorm.DB) error {
	if err := n.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	n.CanvasNodeRunID = n.ID
	return n.marshal()
}
func (n *CanvasNodeRun) AfterCreate(*gorm.DB) error { n.CanvasNodeRunID = n.ID; return nil }
func (n *CanvasNodeRun) BeforeUpdate(tx *gorm.DB) error {
	if err := n.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return n.marshal()
}
func (n *CanvasNodeRun) AfterUpdate(*gorm.DB) error { n.CanvasNodeRunID = n.ID; return nil }
func (n *CanvasNodeRun) AfterFind(tx *gorm.DB) error {
	if err := n.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	n.CanvasNodeRunID = n.ID
	if err := unmarshalCanvasJSON(n.ResolvedInputSnapshotJSON, &n.ResolvedInputSnapshot); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(n.WarningsJSON, &n.Warnings); err != nil {
		return err
	}
	return unmarshalCanvasJSON(n.LastErrorJSON, &n.LastError)
}
func (n *CanvasNodeRun) marshal() error {
	if n.NodeID == "" {
		n.NodeID = n.NodeKey
	}
	if n.ExecutionKey == "" {
		n.ExecutionKey = n.NodeID
	}
	if err := marshalCanvasJSON(n.ResolvedInputSnapshot, &n.ResolvedInputSnapshotJSON, `{}`); err != nil {
		return err
	}
	if err := marshalCanvasJSON(n.Warnings, &n.WarningsJSON, `[]`); err != nil {
		return err
	}
	return marshalCanvasJSON(n.LastError, &n.LastErrorJSON, `{}`)
}

func marshalCanvasJSON(value any, target *string, fallback string) error {
	if value == nil {
		*target = fallback
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	*target = string(b)
	return nil
}
func unmarshalCanvasJSON(value string, target any) error {
	if value == "" {
		value = `{}`
	}
	return json.Unmarshal([]byte(value), target)
}
