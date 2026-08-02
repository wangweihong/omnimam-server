package iapiserver

import (
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// WorkflowPortDefinition 描述节点定义的稳定、带类型输入输出端口。
type WorkflowPortDefinition struct {
	Key                         string `json:"key"`
	Label                       string `json:"label"`
	Direction                   string `json:"direction"`
	DataType                    string `json:"data_type"`
	Required                    bool   `json:"required"`
	Cardinality                 string `json:"cardinality"`
	ConnectionType              string `json:"connection_type"`
	AllowsLiteral               bool   `json:"allows_literal"`
	DefaultValue                any    `json:"default_value,omitempty"`
	RequiredForCompletion       bool   `json:"required_for_completion"`
	ArtifactReadyTimeoutSeconds *int   `json:"artifact_ready_timeout_seconds,omitempty"`
}

// WorkflowExecutionBinding 只允许被注册的编译器、functionRef 或已发布 ApplicationVersion。
type WorkflowExecutionBinding struct {
	Mode           string `json:"mode"`
	BindingVersion string `json:"binding_version"`
	// CompilerKey 标识服务端注册的确定性编译期能力；该模式不创建自身 AtomicTask。
	CompilerKey          *string `json:"compiler_key,omitempty"`
	FunctionRef          *string `json:"function_ref,omitempty"`
	ApplicationVersionID *string `json:"application_version_id,omitempty"`
	MaxDynamicTasks      *int    `json:"max_dynamic_tasks,omitempty"`
}

// WorkflowRendererCapability 引用前端已注册的受控渲染器，不携带可执行代码。
type WorkflowRendererCapability struct {
	RendererKey                   string `json:"renderer_key"`
	RendererVersion               string `json:"renderer_version"`
	SupportsClientGeneratedOutput bool   `json:"supports_client_generated_output"`
}

// WorkflowNodeDefinition 是不可变节点能力版本；deprecated 只阻止新引用。
type WorkflowNodeDefinition struct {
	imachinery.ObjectMeta     `json:"-"`
	NodeType                  string                   `json:"node_type"                           gorm:"column:node_type;type:varchar(200);not null;uniqueIndex:idx_workflow_node_definition_version,priority:1"`
	DefinitionVersion         string                   `json:"definition_version"                  gorm:"column:definition_version;type:varchar(64);not null;uniqueIndex:idx_workflow_node_definition_version,priority:2"`
	Title                     string                   `json:"title"                               gorm:"column:title;type:varchar(200);not null"`
	Category                  string                   `json:"category"                            gorm:"column:category;type:varchar(100);not null;index"`
	NodeKind                  string                   `json:"node_kind"                           gorm:"column:node_kind;type:varchar(32);not null;index"`
	Ports                     []WorkflowPortDefinition `json:"ports"                               gorm:"-"`
	PortsJSON                 string                   `json:"-"                                   gorm:"column:ports_json;type:text;not null"`
	ConfigSchema              map[string]any           `json:"config_schema"                       gorm:"-"`
	ConfigSchemaJSON          string                   `json:"-"                                   gorm:"column:config_schema_json;type:text;not null"`
	ControllerStateSchema     map[string]any           `json:"controller_state_schema,omitempty"   gorm:"-"`
	ControllerStateSchemaJSON *string                  `json:"-"                                   gorm:"column:controller_state_schema_json;type:text"`
	ControllerSchemaVersion   *string                  `json:"controller_schema_version,omitempty" gorm:"column:controller_schema_version;type:varchar(64)"`
	ExecutionMode             string                   `json:"-"                                   gorm:"column:execution_mode;type:varchar(16);not null"`
	// CompilerKey 固定 compile_time 定义使用的内置编译器，其他执行模式必须为空。
	CompilerKey                   *string                     `json:"-"                                   gorm:"column:compiler_key;type:varchar(64)"`
	FunctionRef                   *string                     `json:"-"                                   gorm:"column:function_ref;type:varchar(256)"`
	ApplicationVersionID          *string                     `json:"-"                                   gorm:"column:application_version_id;type:varchar(64)"`
	BindingVersion                string                      `json:"-"                                   gorm:"column:binding_version;type:varchar(64);not null"`
	MaxDynamicTasks               *int                        `json:"-"                                   gorm:"column:max_dynamic_tasks"`
	RendererKey                   *string                     `json:"-"                                   gorm:"column:renderer_key;type:varchar(100)"`
	RendererVersion               *string                     `json:"-"                                   gorm:"column:renderer_version;type:varchar(64)"`
	SupportsClientGeneratedOutput bool                        `json:"-"                                   gorm:"column:supports_client_generated_output;not null;default:false"`
	CacheAllowed                  bool                        `json:"cache_allowed"                       gorm:"column:cache_allowed;not null;default:false"`
	ReuseTTLSeconds               *int                        `json:"reuse_ttl_seconds,omitempty"         gorm:"column:reuse_ttl_seconds"`
	AvailabilityScope             string                      `json:"availability_scope"                  gorm:"column:availability_scope;type:varchar(16);not null"`
	ProjectID                     *string                     `json:"project_id,omitempty"                gorm:"column:project_id;type:varchar(128)"`
	Namespace                     *string                     `json:"namespace,omitempty"                 gorm:"column:namespace;type:varchar(128)"`
	RegisteredBy                  string                      `json:"-"                                   gorm:"column:registered_by;type:varchar(128);not null"`
	Deprecated                    bool                        `json:"deprecated"                          gorm:"column:deprecated;not null;default:false;index"`
	DeprecatedAt                  imachinery.Time             `json:"-"                                   gorm:"column:deprecated_at"`
	ExecutionBinding              WorkflowExecutionBinding    `json:"execution_binding"                   gorm:"-"`
	Renderer                      *WorkflowRendererCapability `json:"renderer,omitempty"                  gorm:"-"`
}

func (WorkflowNodeDefinition) TableName() string { return "workflow_node_definitions" }
func (d *WorkflowNodeDefinition) BeforeCreate(tx *gorm.DB) error {
	if err := d.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return d.marshal()
}
func (d *WorkflowNodeDefinition) AfterCreate(*gorm.DB) error { d.hydrate(); return nil }
func (d *WorkflowNodeDefinition) BeforeUpdate(tx *gorm.DB) error {
	if err := d.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return d.marshal()
}
func (d *WorkflowNodeDefinition) AfterUpdate(*gorm.DB) error { d.hydrate(); return nil }
func (d *WorkflowNodeDefinition) AfterFind(tx *gorm.DB) error {
	if err := d.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(d.PortsJSON, &d.Ports); err != nil {
		return err
	}
	if err := unmarshalCanvasJSON(d.ConfigSchemaJSON, &d.ConfigSchema); err != nil {
		return err
	}
	if d.ControllerStateSchemaJSON != nil {
		if err := unmarshalCanvasJSON(*d.ControllerStateSchemaJSON, &d.ControllerStateSchema); err != nil {
			return err
		}
	}
	d.hydrate()
	return nil
}
func (d *WorkflowNodeDefinition) marshal() error {
	d.ExecutionMode = d.ExecutionBinding.Mode
	d.BindingVersion = d.ExecutionBinding.BindingVersion
	d.CompilerKey = d.ExecutionBinding.CompilerKey
	d.FunctionRef = d.ExecutionBinding.FunctionRef
	d.ApplicationVersionID = d.ExecutionBinding.ApplicationVersionID
	d.MaxDynamicTasks = d.ExecutionBinding.MaxDynamicTasks
	if d.Renderer != nil {
		d.RendererKey, d.RendererVersion = &d.Renderer.RendererKey, &d.Renderer.RendererVersion
		d.SupportsClientGeneratedOutput = d.Renderer.SupportsClientGeneratedOutput
	}
	if err := marshalCanvasJSON(d.Ports, &d.PortsJSON, `[]`); err != nil {
		return err
	}
	if err := marshalCanvasJSON(d.ConfigSchema, &d.ConfigSchemaJSON, `{}`); err != nil {
		return err
	}
	if d.ControllerStateSchema != nil {
		var raw string
		if err := marshalCanvasJSON(d.ControllerStateSchema, &raw, `{}`); err != nil {
			return err
		}
		d.ControllerStateSchemaJSON = &raw
	}
	return nil
}
func (d *WorkflowNodeDefinition) hydrate() {
	d.ExecutionBinding = WorkflowExecutionBinding{
		Mode:                 d.ExecutionMode,
		BindingVersion:       d.BindingVersion,
		CompilerKey:          d.CompilerKey,
		FunctionRef:          d.FunctionRef,
		ApplicationVersionID: d.ApplicationVersionID,
		MaxDynamicTasks:      d.MaxDynamicTasks,
	}
	if d.RendererKey != nil && d.RendererVersion != nil {
		d.Renderer = &WorkflowRendererCapability{
			RendererKey:                   *d.RendererKey,
			RendererVersion:               *d.RendererVersion,
			SupportsClientGeneratedOutput: d.SupportsClientGeneratedOutput,
		}
	}
}

// WorkflowRunScope 固定本次运行选择的流或节点闭包。
type WorkflowRunScope struct {
	Mode    string   `json:"mode"`
	FlowIDs []string `json:"flow_ids,omitempty"`
	NodeIDs []string `json:"node_ids,omitempty"`
}

// WorkflowRunPolicy 固定复用和独立流失败传播策略。
type WorkflowRunPolicy struct {
	ReusePolicy   string `json:"reuse_policy"`
	FailurePolicy string `json:"failure_policy"`
}

// WorkflowProgressSummary 是运行和流的有界聚合摘要。
type WorkflowProgressSummary struct {
	Progress  float64 `json:"progress"`
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Success   int     `json:"success"`
	Failed    int     `json:"failed"`
	Canceled  int     `json:"canceled"`
	Skipped   int     `json:"skipped"`
	Running   int     `json:"running,omitempty"`
	Warnings  int     `json:"warnings,omitempty"`
}

// WorkflowRunWarning 是运行投影中的可定位非致命问题。
type WorkflowRunWarning struct {
	Code       string  `json:"code"`
	Message    string  `json:"message"`
	NodeID     *string `json:"node_id,omitempty"`
	PortKey    *string `json:"port_key,omitempty"`
	ArtifactID *string `json:"artifact_id,omitempty"`
}

// CanvasFlowRun 是 CanvasRun 内显式流的业务状态投影，不是 Task Center Group。
type CanvasFlowRun struct {
	imachinery.ObjectMeta `json:"-"`
	CanvasFlowRunID       string                  `json:"canvas_flow_run_id"    gorm:"-"`
	CanvasRunID           string                  `json:"canvas_run_id"         gorm:"column:canvas_run_id;type:varchar(64);not null;uniqueIndex:idx_canvas_flow_run,priority:1;index"`
	FlowID                string                  `json:"flow_id"               gorm:"column:flow_id;type:varchar(200);not null;uniqueIndex:idx_canvas_flow_run,priority:2"`
	ExecutionKeys         []string                `json:"execution_keys"        gorm:"-"`
	ExecutionKeysJSON     string                  `json:"-"                     gorm:"column:execution_keys_json;type:text;not null"`
	Status                string                  `json:"status"                gorm:"column:status;type:varchar(32);not null;index"`
	Progress              float64                 `json:"progress"              gorm:"column:progress;not null;default:0"`
	Summary               WorkflowProgressSummary `json:"summary"               gorm:"-"`
	SummaryJSON           string                  `json:"-"                     gorm:"column:summary_json;type:text;not null"`
	ResultSummary         map[string]any          `json:"result_summary"        gorm:"-"`
	ResultSummaryJSON     string                  `json:"-"                     gorm:"column:result_summary_json;type:text;not null"`
	Warnings              []WorkflowRunWarning    `json:"warnings"              gorm:"-"`
	WarningsJSON          string                  `json:"-"                     gorm:"column:warnings_json;type:text;not null"`
	AggregateVersion      int64                   `json:"aggregate_version"     gorm:"column:aggregate_version;not null;default:0"`
	StartedAt             imachinery.Time         `json:"started_at,omitempty"  gorm:"column:started_at"`
	FinishedAt            imachinery.Time         `json:"finished_at,omitempty" gorm:"column:finished_at"`
}

// CanvasNodeRunFlowRef 允许一个去重后的执行实例被多个 FlowRun 引用。
type CanvasNodeRunFlowRef struct {
	imachinery.ObjectMeta `       json:"-"`
	CanvasNodeRunID       string `json:"-" gorm:"column:canvas_node_run_id;type:varchar(64);not null;uniqueIndex:idx_canvas_node_flow_ref,priority:1"`
	CanvasFlowRunID       string `json:"-" gorm:"column:canvas_flow_run_id;type:varchar(64);not null;uniqueIndex:idx_canvas_node_flow_ref,priority:2;index"`
}

func (CanvasNodeRunFlowRef) TableName() string                 { return "canvas_node_run_flow_refs" }
func (r *CanvasNodeRunFlowRef) BeforeCreate(tx *gorm.DB) error { return r.ObjectMeta.BeforeCreate(tx) }
func (r *CanvasNodeRunFlowRef) AfterCreate(*gorm.DB) error     { return nil }
func (r *CanvasNodeRunFlowRef) BeforeUpdate(tx *gorm.DB) error { return r.ObjectMeta.BeforeUpdate(tx) }
func (r *CanvasNodeRunFlowRef) AfterUpdate(*gorm.DB) error     { return nil }

func (CanvasFlowRun) TableName() string { return "canvas_flow_runs" }
func (r *CanvasFlowRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	r.CanvasFlowRunID = r.ID
	return r.marshal()
}
func (r *CanvasFlowRun) AfterCreate(*gorm.DB) error { r.CanvasFlowRunID = r.ID; return nil }
func (r *CanvasFlowRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (r *CanvasFlowRun) AfterUpdate(*gorm.DB) error { r.CanvasFlowRunID = r.ID; return nil }
func (r *CanvasFlowRun) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	r.CanvasFlowRunID = r.ID
	return r.unmarshal()
}
func (r *CanvasFlowRun) marshal() error {
	for _, x := range []struct {
		v any
		p *string
		f string
	}{{r.ExecutionKeys, &r.ExecutionKeysJSON, "[]"}, {r.Summary, &r.SummaryJSON, "{}"}, {r.ResultSummary, &r.ResultSummaryJSON, "{}"}, {r.Warnings, &r.WarningsJSON, "[]"}} {
		if err := marshalCanvasJSON(x.v, x.p, x.f); err != nil {
			return err
		}
	}
	return nil
}
func (r *CanvasFlowRun) unmarshal() error {
	for _, x := range []struct {
		s string
		p any
	}{{r.ExecutionKeysJSON, &r.ExecutionKeys}, {r.SummaryJSON, &r.Summary}, {r.ResultSummaryJSON, &r.ResultSummary}, {r.WarningsJSON, &r.Warnings}} {
		if err := unmarshalCanvasJSON(x.s, x.p); err != nil {
			return err
		}
	}
	return nil
}

// CanvasNodeRunTaskBinding 保存 NodeRun 到 Task Center AtomicTask 的 1:N 受控引用。
type CanvasNodeRunTaskBinding struct {
	imachinery.ObjectMeta `                      json:"-"`
	BindingID             string                `json:"binding_id"            gorm:"-"`
	CanvasNodeRunID       string                `json:"-"                     gorm:"column:canvas_node_run_id;type:varchar(64);not null;index"`
	DAGTaskGroupID        string                `json:"dag_task_group_id"     gorm:"column:dag_task_group_id;type:varchar(64);not null;index"`
	AtomicTaskID          string                `json:"atomic_task_id"        gorm:"column:atomic_task_id;type:varchar(64);not null;uniqueIndex"`
	TaskChildKey          string                `json:"task_child_key"        gorm:"column:task_child_key;type:varchar(200);not null"`
	BindingRole           string                `json:"binding_role"          gorm:"column:binding_role;type:varchar(32);not null"`
	ShardKey              string                `json:"shard_key"             gorm:"column:shard_key;type:varchar(100);not null;default:root"`
	ShardIndex            *int                  `json:"shard_index,omitempty" gorm:"column:shard_index"`
	TaskResourceVersion   int64                 `json:"task_resource_version" gorm:"column:task_resource_version;not null;default:0"`
	AtomicTask            *AtomicTaskRefSummary `json:"atomic_task,omitempty" gorm:"-"`
}

func (CanvasNodeRunTaskBinding) TableName() string { return "canvas_node_run_task_bindings" }
func (b *CanvasNodeRunTaskBinding) BeforeCreate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	b.BindingID = b.ID
	return nil
}
func (b *CanvasNodeRunTaskBinding) AfterCreate(*gorm.DB) error { b.BindingID = b.ID; return nil }
func (b *CanvasNodeRunTaskBinding) BeforeUpdate(tx *gorm.DB) error {
	return b.ObjectMeta.BeforeUpdate(tx)
}
func (b *CanvasNodeRunTaskBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *CanvasNodeRunTaskBinding) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	b.BindingID = b.ID
	return nil
}

// WorkflowArtifactSummary 是 Asset Library 权限裁剪后的一跳 Artifact 摘要。
type WorkflowArtifactSummary struct {
	ArtifactID       string  `json:"artifact_id"`
	Name             *string `json:"name,omitempty"`
	ArtifactType     string  `json:"artifact_type"`
	MediaType        *string `json:"media_type,omitempty"`
	ProcessingStatus string  `json:"processing_status"`
	PreviewAvailable bool    `json:"preview_available"`
	PreviewRef       *string `json:"preview_ref,omitempty"`
	ResourceVersion  int64   `json:"resource_version"`
}

// CanvasNodeRunOutputBinding 保存端口输出槽位与结构化值或 Artifact 引用。
type CanvasNodeRunOutputBinding struct {
	imachinery.ObjectMeta   `                         json:"-"`
	OutputBindingID         string                   `json:"output_binding_id"          gorm:"-"`
	CanvasNodeRunID         string                   `json:"-"                          gorm:"column:canvas_node_run_id;type:varchar(64);not null;index"`
	PortKey                 string                   `json:"port_key"                   gorm:"column:port_key;type:varchar(100);not null"`
	Required                bool                     `json:"required"                   gorm:"column:required;not null"`
	ShardKey                string                   `json:"shard_key"                  gorm:"column:shard_key;type:varchar(100);not null;default:root"`
	ShardIndex              *int                     `json:"shard_index,omitempty"      gorm:"column:shard_index"`
	ProducerKey             string                   `json:"producer_key"               gorm:"column:producer_key;type:varchar(300);not null;uniqueIndex"`
	AtomicTaskID            *string                  `json:"atomic_task_id,omitempty"   gorm:"column:atomic_task_id;type:varchar(64)"`
	ArtifactID              *string                  `json:"artifact_id,omitempty"      gorm:"column:artifact_id;type:varchar(64);index"`
	Artifact                *WorkflowArtifactSummary `json:"artifact,omitempty"         gorm:"-"`
	StructuredValue         any                      `json:"structured_value,omitempty" gorm:"-"`
	StructuredValueJSON     *string                  `json:"-"                          gorm:"column:structured_value_json;type:text"`
	AvailabilityStatus      string                   `json:"availability_status"        gorm:"column:availability_status;type:varchar(16);not null"`
	ArtifactResourceVersion int64                    `json:"-"                          gorm:"column:artifact_resource_version;not null;default:0"`
	Warning                 map[string]any           `json:"warning,omitempty"          gorm:"-"`
	WarningJSON             *string                  `json:"-"                          gorm:"column:warning_json;type:text"`
	SourceOutputBindingID   *string                  `json:"-"                          gorm:"column:source_output_binding_id;type:varchar(64)"`
	AggregateVersion        int64                    `json:"aggregate_version"          gorm:"column:aggregate_version;not null;default:0"`
}

func (CanvasNodeRunOutputBinding) TableName() string { return "canvas_node_run_output_bindings" }
func (b *CanvasNodeRunOutputBinding) BeforeCreate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	b.OutputBindingID = b.ID
	return b.marshal()
}
func (b *CanvasNodeRunOutputBinding) AfterCreate(*gorm.DB) error {
	b.OutputBindingID = b.ID
	return nil
}
func (b *CanvasNodeRunOutputBinding) BeforeUpdate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return b.marshal()
}
func (b *CanvasNodeRunOutputBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *CanvasNodeRunOutputBinding) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	b.OutputBindingID = b.ID
	if b.StructuredValueJSON != nil {
		if err := unmarshalCanvasJSON(*b.StructuredValueJSON, &b.StructuredValue); err != nil {
			return err
		}
	}
	if b.WarningJSON != nil {
		return unmarshalCanvasJSON(*b.WarningJSON, &b.Warning)
	}
	return nil
}
func (b *CanvasNodeRunOutputBinding) marshal() error {
	if b.StructuredValue != nil {
		var raw string
		if err := marshalCanvasJSON(b.StructuredValue, &raw, "null"); err != nil {
			return err
		}
		b.StructuredValueJSON = &raw
	}
	if b.Warning != nil {
		var raw string
		if err := marshalCanvasJSON(b.Warning, &raw, "{}"); err != nil {
			return err
		}
		b.WarningJSON = &raw
	}
	return nil
}

// CanvasNodeRunDetail 一次返回单个节点运行的有界任务与输出绑定。
type CanvasNodeRunDetail struct {
	*CanvasNodeRun
	TaskBindings   []*CanvasNodeRunTaskBinding   `json:"task_bindings"`
	OutputBindings []*CanvasNodeRunOutputBinding `json:"output_bindings"`
}

// WorkflowCanvasOutbox 记录已原子写入可靠消息表的 Canvas 领域事件。
type WorkflowCanvasOutbox struct {
	imachinery.ObjectMeta `json:"-"`
	EventName             string          `json:"-" gorm:"column:event_name;type:varchar(100);not null;uniqueIndex:idx_workflow_canvas_outbox_event,priority:4"`
	AggregateType         string          `json:"-" gorm:"column:aggregate_type;type:varchar(64);not null;uniqueIndex:idx_workflow_canvas_outbox_event,priority:1"`
	AggregateID           string          `json:"-" gorm:"column:aggregate_id;type:varchar(64);not null;uniqueIndex:idx_workflow_canvas_outbox_event,priority:2"`
	AggregateVersion      int64           `json:"-" gorm:"column:aggregate_version;not null;uniqueIndex:idx_workflow_canvas_outbox_event,priority:3"`
	Payload               map[string]any  `json:"-" gorm:"-"`
	PayloadJSON           string          `json:"-" gorm:"column:payload_json;type:text;not null"`
	DeliveryStatus        string          `json:"-" gorm:"column:delivery_status;type:varchar(16);not null;index"`
	AttemptCount          int             `json:"-" gorm:"column:attempt_count;not null;default:0"`
	NextAttemptAt         imachinery.Time `json:"-" gorm:"column:next_attempt_at"`
	PublishedAt           imachinery.Time `json:"-" gorm:"column:published_at"`
}

func (WorkflowCanvasOutbox) TableName() string { return "workflow_canvas_outbox" }
func (o *WorkflowCanvasOutbox) BeforeCreate(tx *gorm.DB) error {
	if err := o.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalCanvasJSON(o.Payload, &o.PayloadJSON, "{}")
}
func (o *WorkflowCanvasOutbox) AfterCreate(*gorm.DB) error { return nil }
func (o *WorkflowCanvasOutbox) BeforeUpdate(tx *gorm.DB) error {
	if err := o.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalCanvasJSON(o.Payload, &o.PayloadJSON, "{}")
}
func (o *WorkflowCanvasOutbox) AfterUpdate(*gorm.DB) error { return nil }
func (o *WorkflowCanvasOutbox) AfterFind(tx *gorm.DB) error {
	if err := o.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalCanvasJSON(o.PayloadJSON, &o.Payload)
}

// WorkflowCanvasReconcileCursor 保存 Task Center/Asset Library 对账进度。
type WorkflowCanvasReconcileCursor struct {
	imachinery.ObjectMeta `json:"-"`
	SourceDomain          string          `json:"-" gorm:"column:source_domain;type:varchar(32);not null;uniqueIndex:idx_workflow_canvas_reconcile_cursor,priority:1"`
	PartitionKey          string          `json:"-" gorm:"column:partition_key;type:varchar(128);not null;uniqueIndex:idx_workflow_canvas_reconcile_cursor,priority:2"`
	CursorValue           string          `json:"-" gorm:"column:cursor_value;type:text;not null"`
	LastReconciledAt      imachinery.Time `json:"-" gorm:"column:last_reconciled_at"`
	LastError             map[string]any  `json:"-" gorm:"-"`
	LastErrorJSON         string          `json:"-" gorm:"column:last_error_json;type:text"`
}

func (WorkflowCanvasReconcileCursor) TableName() string { return "workflow_canvas_reconcile_cursors" }
func (c *WorkflowCanvasReconcileCursor) BeforeCreate(tx *gorm.DB) error {
	if err := c.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalCanvasJSON(c.LastError, &c.LastErrorJSON, "{}")
}
func (c *WorkflowCanvasReconcileCursor) AfterCreate(*gorm.DB) error { return nil }
func (c *WorkflowCanvasReconcileCursor) BeforeUpdate(tx *gorm.DB) error {
	if err := c.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalCanvasJSON(c.LastError, &c.LastErrorJSON, "{}")
}
func (c *WorkflowCanvasReconcileCursor) AfterUpdate(*gorm.DB) error { return nil }
func (c *WorkflowCanvasReconcileCursor) AfterFind(tx *gorm.DB) error {
	if err := c.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalCanvasJSON(c.LastErrorJSON, &c.LastError)
}
