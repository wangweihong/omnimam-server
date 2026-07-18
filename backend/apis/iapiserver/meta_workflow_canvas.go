package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	CanvasVisibilityPrivate = "PRIVATE"
	CanvasVisibilityProject = "PROJECT"

	CanvasNodeTypeApplication = "APPLICATION"
	CanvasNodeTypeFunction    = "FUNCTION"
	CanvasNodeTypeDynamicFork = "DYNAMIC_FORK"

	CanvasRunStatusPending  = "PENDING"
	CanvasRunStatusRunning  = "RUNNING"
	CanvasRunStatusSuccess  = "SUCCESS"
	CanvasRunStatusFailed   = "FAILED"
	CanvasRunStatusCanceled = "CANCELED"
	CanvasRunStatusTimeout  = "TIMEOUT"

	CanvasTaskCreationPending = "PENDING"
	CanvasTaskCreationCreated = "CREATED"
	CanvasTaskCreationFailed  = "FAILED"
)

// WorkflowCanvasGraph 是草稿和发布版本共用的有向图快照。
type WorkflowCanvasGraph struct {
	Nodes []WorkflowCanvasNode `json:"nodes"`
	Edges []WorkflowCanvasEdge `json:"edges"`
}

// WorkflowCanvasNode 只允许引用已发布 ApplicationVersion 或已注册 functionRef。
type WorkflowCanvasNode struct {
	NodeKey         string         `json:"node_key"`
	NodeType        string         `json:"node_type"`
	Config          map[string]any `json:"config"`
	InputBindings   map[string]any `json:"input_bindings"`
	Position        map[string]any `json:"position,omitempty"`
	MaxDynamicTasks *int           `json:"max_dynamic_tasks,omitempty"`
}

// WorkflowCanvasEdge 描述节点输出端口到下游输入端口的确定性绑定。
type WorkflowCanvasEdge struct {
	FromNodeKey string `json:"from_node_key"`
	FromOutput  string `json:"from_output"`
	ToNodeKey   string `json:"to_node_key"`
	ToInput     string `json:"to_input"`
}

// WorkflowCanvas 保存可编辑草稿；发布后通过新版本固化，不修改历史版本。
type WorkflowCanvas struct {
	imachinery.ObjectMeta `json:"-"`
	CanvasID              string              `json:"canvas_id" gorm:"-"`
	Visibility            string              `json:"visibility" gorm:"column:visibility;type:varchar(16);not null"`
	DraftGraph            WorkflowCanvasGraph `json:"draft_graph" gorm:"-"`
	DraftGraphJSON        string              `json:"-" gorm:"column:draft_graph_json;type:text;not null"`
	DraftRevision         int64               `json:"draft_revision" gorm:"column:draft_revision;not null;default:1"`
	LatestVersion         int                 `json:"latest_version" gorm:"column:latest_version;not null;default:0"`
	ProjectID             string              `json:"project_id" gorm:"column:project_id;type:varchar(128);not null;index"`
	Namespace             string              `json:"namespace" gorm:"column:namespace;type:varchar(128);not null;index"`
	CreatedBy             string              `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt             imachinery.Time     `json:"-" gorm:"column:deleted_at;index"`
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
	CanvasVersionID           string              `json:"canvas_version_id" gorm:"-"`
	CanvasID                  string              `json:"canvas_id" gorm:"column:canvas_id;type:varchar(64);not null;uniqueIndex:idx_canvas_version_number,priority:1;uniqueIndex:idx_canvas_version_digest,priority:1"`
	Version                   int                 `json:"version" gorm:"column:version;not null;uniqueIndex:idx_canvas_version_number,priority:2"`
	GraphSnapshot             WorkflowCanvasGraph `json:"graph_snapshot" gorm:"-"`
	GraphSnapshotJSON         string              `json:"-" gorm:"column:graph_snapshot_json;type:text;not null"`
	InputSchema               map[string]any      `json:"input_schema" gorm:"-"`
	InputSchemaJSON           string              `json:"-" gorm:"column:input_schema_json;type:text;not null"`
	OutputSchema              map[string]any      `json:"output_schema" gorm:"-"`
	OutputSchemaJSON          string              `json:"-" gorm:"column:output_schema_json;type:text;not null"`
	ContentDigest             string              `json:"content_digest" gorm:"column:content_digest;type:varchar(80);not null;uniqueIndex:idx_canvas_version_digest,priority:2"`
	CompiledDefinitionName    string              `json:"compiled_definition_name" gorm:"column:compiled_definition_name;type:varchar(256);not null"`
	CompiledDefinitionVersion int                 `json:"compiled_definition_version" gorm:"column:compiled_definition_version;not null"`
	NodeCount                 int                 `json:"-" gorm:"column:node_count;not null"`
	EdgeCount                 int                 `json:"-" gorm:"column:edge_count;not null"`
	PublishedBy               string              `json:"published_by" gorm:"column:published_by;type:varchar(128);not null"`
	PublishedAt               imachinery.Time     `json:"published_at" gorm:"column:published_at;not null"`
}

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
	return marshalCanvasJSON(v.OutputSchema, &v.OutputSchemaJSON, `{}`)
}
func (v *CanvasVersion) unmarshal() error {
	if err := unmarshalCanvasJSON(v.GraphSnapshotJSON, &v.GraphSnapshot); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(v.InputSchemaJSON, &v.InputSchema); err != nil {
		return err
	}
	return unmarshalCanvasJSON(v.OutputSchemaJSON, &v.OutputSchema)
}

// WorkflowCanvasRun 是固定 CanvasVersion 的业务运行投影。
type WorkflowCanvasRun struct {
	imachinery.ObjectMeta `json:"-"`
	CanvasRunID           string          `json:"canvas_run_id" gorm:"-"`
	CanvasID              string          `json:"canvas_id" gorm:"column:canvas_id;type:varchar(64);not null;index"`
	CanvasVersionID       string          `json:"canvas_version_id" gorm:"column:canvas_version_id;type:varchar(64);not null;index"`
	IdempotencyKey        string          `json:"idempotency_key" gorm:"column:idempotency_key;type:varchar(200);not null;uniqueIndex:idx_canvas_run_idempotency,priority:4"`
	RequestDigest         string          `json:"-" gorm:"column:request_digest;type:varchar(80);not null"`
	InputSnapshot         map[string]any  `json:"input_snapshot" gorm:"-"`
	InputSnapshotJSON     string          `json:"-" gorm:"column:input_snapshot_json;type:text;not null"`
	DAGTaskGroupID        *string         `json:"dag_task_group_id" gorm:"column:dag_task_group_id;type:varchar(64);uniqueIndex"`
	TaskCreationStatus    string          `json:"task_creation_status" gorm:"column:task_creation_status;type:varchar(16);not null"`
	Status                string          `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	Progress              float64         `json:"progress" gorm:"column:progress;not null;default:0"`
	Summary               map[string]any  `json:"summary" gorm:"-"`
	SummaryJSON           string          `json:"-" gorm:"column:summary_json;type:text;not null"`
	Output                map[string]any  `json:"output" gorm:"-"`
	OutputJSON            string          `json:"-" gorm:"column:output_json;type:text;not null"`
	LastError             map[string]any  `json:"last_error" gorm:"-"`
	LastErrorJSON         string          `json:"-" gorm:"column:last_error_json;type:text;not null"`
	TaskResourceVersion   int64           `json:"task_resource_version" gorm:"column:task_resource_version;not null;default:0"`
	RetryOfCanvasRunID    *string         `json:"retry_of_canvas_run_id" gorm:"column:retry_of_canvas_run_id;type:varchar(64)"`
	ProjectID             string          `json:"project_id" gorm:"column:project_id;type:varchar(128);not null;uniqueIndex:idx_canvas_run_idempotency,priority:1"`
	Namespace             string          `json:"namespace" gorm:"column:namespace;type:varchar(128);not null;uniqueIndex:idx_canvas_run_idempotency,priority:2"`
	CreatedBy             string          `json:"created_by" gorm:"column:created_by;type:varchar(128);not null;uniqueIndex:idx_canvas_run_idempotency,priority:3"`
	FinishedAt            imachinery.Time `json:"finished_at,omitempty" gorm:"column:finished_at"`
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
	}{{r.InputSnapshot, &r.InputSnapshotJSON}, {r.Summary, &r.SummaryJSON}, {r.Output, &r.OutputJSON}, {r.LastError, &r.LastErrorJSON}} {
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
	}{{r.InputSnapshotJSON, &r.InputSnapshot}, {r.SummaryJSON, &r.Summary}, {r.OutputJSON, &r.Output}, {r.LastErrorJSON, &r.LastError}} {
		if err := unmarshalCanvasJSON(f.s, f.p); err != nil {
			return err
		}
	}
	return nil
}

// CanvasNodeRun 将画布节点稳定映射到 Task Center AtomicTask 投影。
type CanvasNodeRun struct {
	imachinery.ObjectMeta `json:"-"`
	CanvasNodeRunID       string          `json:"canvas_node_run_id" gorm:"-"`
	CanvasRunID           string          `json:"canvas_run_id" gorm:"column:canvas_run_id;type:varchar(64);not null;uniqueIndex:idx_canvas_node_run_key,priority:1;index"`
	NodeKey               string          `json:"node_key" gorm:"column:node_key;type:varchar(100);not null;uniqueIndex:idx_canvas_node_run_key,priority:2"`
	NodeType              string          `json:"node_type" gorm:"column:node_type;type:varchar(32);not null"`
	AtomicTaskID          *string         `json:"atomic_task_id" gorm:"column:atomic_task_id;type:varchar(64);uniqueIndex"`
	Status                string          `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	Progress              float64         `json:"progress" gorm:"column:progress;not null;default:0"`
	Output                map[string]any  `json:"output" gorm:"-"`
	OutputJSON            string          `json:"-" gorm:"column:output_json;type:text;not null"`
	LastError             map[string]any  `json:"last_error" gorm:"-"`
	LastErrorJSON         string          `json:"-" gorm:"column:last_error_json;type:text;not null"`
	TaskResourceVersion   int64           `json:"task_resource_version" gorm:"column:task_resource_version;not null;default:0"`
	FinishedAt            imachinery.Time `json:"finished_at,omitempty" gorm:"column:finished_at"`
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
	if err := unmarshalCanvasJSON(n.OutputJSON, &n.Output); err != nil {
		return err
	}
	return unmarshalCanvasJSON(n.LastErrorJSON, &n.LastError)
}
func (n *CanvasNodeRun) marshal() error {
	if err := marshalCanvasJSON(n.Output, &n.OutputJSON, `{}`); err != nil {
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
