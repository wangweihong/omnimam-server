package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"gorm.io/gorm"
)

const (
	AgentKindPlatform = "platform"
	AgentKindCoding   = "coding"

	AgentWorkspaceTypeAgent  = "agent"
	AgentWorkspaceTypeStudio = "studio"
)

// AgentProfile 是平台只读加载的 Agent 运行模板，不由用户写入。
// +k8s:deepcopy-gen=true
type AgentProfile struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Revision            string   `json:"revision"`
	Status              string   `json:"status"`
	SupportedAgentKinds []string `json:"supported_agent_kinds"`
	Description         string   `json:"description,omitempty"`
}

// AgentReference 是 Agent 消息附件使用的稳定跨域引用，不包含物理路径。
// +k8s:deepcopy-gen=true
type AgentReference struct {
	ReferenceType string `json:"reference_type"`
	ReferenceID   string `json:"reference_id"`
}

// Agent 管理持久化代理身份、固定 Workspace 和业务生命周期。
// +k8s:deepcopy-gen=true
type Agent struct {
	imachinery.ObjectMeta
	// OwnerUserID 来自可信身份上下文，用于所有 Agent 资源隔离。
	OwnerUserID string `json:"-" gorm:"column:owner_user_id;type:text;not null;index:idx_agents_owner_status,priority:1"`
	// Kind 决定 platform/coding 业务类型，并约束固定 Workspace 类型。
	Kind string `json:"kind" gorm:"column:kind;type:text;not null"`
	// AgentProfileID 固定创建时选择的平台 Profile。
	AgentProfileID string `json:"agent_profile_id" gorm:"column:agent_profile_id;type:text;not null"`
	// AgentProfileRevision 防止 Profile 升级改写历史 Agent 运行合同。
	AgentProfileRevision string `json:"agent_profile_revision" gorm:"column:agent_profile_revision;type:text;not null"`
	// WorkspaceType 创建后不可变，platform/coding 分别固定 agent/studio。
	WorkspaceType string `json:"-" gorm:"column:workspace_type;type:text;not null;index:idx_agents_workspace,priority:1"`
	// WorkspaceID 是固定 Workspace 稳定标识，不是存储路径。
	WorkspaceID string `json:"-" gorm:"column:workspace_id;type:text;not null;index:idx_agents_workspace,priority:2"`
	// Status 是 Agent 自有状态，不复制 InfraRuntime 或 AtomicTask 状态机。
	Status string `json:"status" gorm:"column:status;type:text;not null;index:idx_agents_owner_status,priority:2"`
	// Disabled 显式阻止启动、恢复和新 Invocation。
	Disabled bool `json:"disabled" gorm:"column:disabled;not null;default:false"`
	// RuntimePolicy 保存已校验的恢复、空闲和资源策略，不含 Provider 私有参数。
	RuntimePolicy       json.RawMessage `json:"runtime_policy,omitempty" gorm:"-"`
	RuntimePolicyShadow string          `json:"-" gorm:"column:runtime_policy_json;type:text;not null;default:'{}'"`
	// LastActiveAt 记录最近持久化业务活动，不由容器心跳直接覆盖。
	LastActiveAt imachinery.Time `json:"last_active_at,omitempty" gorm:"column:last_active_at"`
}

func (Agent) TableName() string { return "agents" }
func (a *Agent) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&a.ObjectMeta, tx, a.marshalJSON)
}
func (*Agent) AfterCreate(*gorm.DB) error { return nil }
func (a *Agent) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&a.ObjectMeta, tx, a.marshalJSON)
}
func (*Agent) AfterUpdate(*gorm.DB) error { return nil }
func (a *Agent) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(a.RuntimePolicyShadow, &a.RuntimePolicy, "{}")
	return nil
}
func (a *Agent) marshalJSON() error {
	return marshalJSONFields(jsonField{a.RuntimePolicy, &a.RuntimePolicyShadow, "{}"})
}

// AgentSession 保存可跨 Runtime 恢复的会话事实。
// +k8s:deepcopy-gen=true
type AgentSession struct {
	imachinery.ObjectMeta
	AgentID     string `json:"agent_id" gorm:"column:agent_id;type:text;not null;index:idx_agent_sessions_agent_status,priority:1"`
	OwnerUserID string `json:"-" gorm:"column:owner_user_id;type:text;not null"`
	Title       string `json:"title" gorm:"column:title;type:text;not null;default:''"`
	// OPEN ──[Close]──> CLOSED ──[Archive]──> ARCHIVED
	Status            string          `json:"status" gorm:"column:status;type:text;not null;index:idx_agent_sessions_agent_status,priority:2"`
	RuntimeSessionRef string          `json:"runtime_session_ref,omitempty" gorm:"column:runtime_session_ref;type:text"`
	LastMessageAt     imachinery.Time `json:"last_message_at,omitempty" gorm:"column:last_message_at"`
}

func (AgentSession) TableName() string { return "agent_sessions" }

// AgentMessage 保存 Session 原始消息正文和稳定附件引用。
// +k8s:deepcopy-gen=true
type AgentMessage struct {
	imachinery.ObjectMeta
	SessionID         string           `json:"session_id" gorm:"column:session_id;type:text;not null;index:idx_agent_messages_session_created,priority:1"`
	AgentID           string           `json:"agent_id" gorm:"column:agent_id;type:text;not null"`
	InvocationID      string           `json:"invocation_id,omitempty" gorm:"column:invocation_id;type:text"`
	Role              string           `json:"role" gorm:"column:role;type:text;not null"`
	Content           string           `json:"content" gorm:"column:content;type:text;not null"`
	Attachments       []AgentReference `json:"attachments,omitempty" gorm:"-"`
	AttachmentsShadow string           `json:"-" gorm:"column:attachments_json;type:text;not null;default:'[]'"`
}

func (AgentMessage) TableName() string { return "agent_messages" }
func (m *AgentMessage) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&m.ObjectMeta, tx, m.marshalJSON)
}
func (*AgentMessage) AfterCreate(*gorm.DB) error { return nil }
func (m *AgentMessage) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&m.ObjectMeta, tx, m.marshalJSON)
}
func (*AgentMessage) AfterUpdate(*gorm.DB) error { return nil }
func (m *AgentMessage) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(m.AttachmentsShadow, &m.Attachments, "[]")
	return nil
}
func (m *AgentMessage) marshalJSON() error {
	return marshalJSONFields(jsonField{m.Attachments, &m.AttachmentsShadow, "[]"})
}

// AgentInvocation 是一次 Agent 交互或后台操作的业务投影。
// +k8s:deepcopy-gen=true
type AgentInvocation struct {
	imachinery.ObjectMeta
	AgentID                     string          `json:"agent_id" gorm:"column:agent_id;type:text;not null;uniqueIndex:idx_agent_invocations_idempotency,priority:1"`
	SessionID                   string          `json:"session_id" gorm:"column:session_id;type:text;not null;index:idx_agent_invocations_session_status,priority:1"`
	Type                        string          `json:"type" gorm:"column:type;type:text;not null"`
	Status                      string          `json:"status" gorm:"column:status;type:text;not null;index:idx_agent_invocations_session_status,priority:2"`
	UserMessageID               string          `json:"user_message_id,omitempty" gorm:"column:user_message_id;type:text"`
	AssistantMessageID          string          `json:"assistant_message_id,omitempty" gorm:"column:assistant_message_id;type:text"`
	AtomicTaskID                *string         `json:"atomic_task_id,omitempty" gorm:"column:atomic_task_id;type:text;index"`
	RuntimeBindingID            string          `json:"runtime_binding_id,omitempty" gorm:"column:runtime_binding_id;type:text;index"`
	RuntimeSessionRef           string          `json:"runtime_session_ref,omitempty" gorm:"column:runtime_session_ref;type:text"`
	RuntimeInvocationRef        string          `json:"runtime_invocation_ref,omitempty" gorm:"column:runtime_invocation_ref;type:text"`
	LastEventSequence           int             `json:"last_event_sequence" gorm:"column:last_event_sequence;not null;default:0"`
	SubmissionGeneration        int             `json:"submission_generation" gorm:"column:submission_generation;not null;default:0"`
	TaskExpectedResourceVersion *int64          `json:"task_expected_resource_version,omitempty" gorm:"column:task_expected_resource_version"`
	TerminalProjectedTaskID     string          `json:"terminal_projected_task_id,omitempty" gorm:"column:terminal_projected_task_id;type:text"`
	TerminalProjectedAt         imachinery.Time `json:"terminal_projected_at,omitempty" gorm:"column:terminal_projected_at"`
	FailureCode                 string          `json:"failure_code,omitempty" gorm:"column:failure_code;type:text"`
	FailureMessage              string          `json:"failure_message,omitempty" gorm:"column:failure_message;type:text;not null;default:''"`
	StartedAt                   imachinery.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt                 imachinery.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	IdempotencyKey              string          `json:"-" gorm:"column:idempotency_key;type:text;not null;uniqueIndex:idx_agent_invocations_idempotency,priority:2"`
}

func (AgentInvocation) TableName() string { return "agent_invocations" }

func (i *AgentInvocation) BeforeCreate(tx *gorm.DB) error {
	return i.ObjectMeta.BeforeCreate(tx)
}

func (i *AgentInvocation) BeforeUpdate(tx *gorm.DB) error {
	return i.ObjectMeta.BeforeUpdate(tx)
}

// AgentMemory 保存 AGENT 或 SESSION 范围的长期记忆正文。
// +k8s:deepcopy-gen=true
type AgentMemory struct {
	imachinery.ObjectMeta
	AgentID         string          `json:"agent_id" gorm:"column:agent_id;type:text;not null;index:idx_agent_memories_scope,priority:1"`
	SessionID       *string         `json:"session_id,omitempty" gorm:"column:session_id;type:text"`
	Scope           string          `json:"scope" gorm:"column:scope;type:text;not null;index:idx_agent_memories_scope,priority:2"`
	Type            string          `json:"type" gorm:"column:type;type:text;not null"`
	Content         string          `json:"content" gorm:"column:content;type:text;not null"`
	SourceMessageID string          `json:"source_message_id,omitempty" gorm:"column:source_message_id;type:text"`
	DeletedAt       imachinery.Time `json:"-" gorm:"column:deleted_at;index"`
}

func (AgentMemory) TableName() string { return "agent_memories" }

func (m *AgentMemory) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}

func (m *AgentMemory) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}

// AgentModelBinding 固定 Agent 的模型来源和用途，不保存明文凭证。
// +k8s:deepcopy-gen=true
type AgentModelBinding struct {
	imachinery.ObjectMeta
	AgentID    string `json:"agent_id" gorm:"column:agent_id;type:text;not null"`
	SourceType string `json:"source_type" gorm:"column:source_type;type:text;not null"`
	SourceRef  string `json:"source_ref" gorm:"column:source_ref;type:text;not null"`
	Purpose    string `json:"purpose" gorm:"column:purpose;type:text;not null"`
	Status     string `json:"status" gorm:"column:status;type:text;not null"`
	IsPrimary  bool   `json:"is_primary" gorm:"column:is_primary;not null;default:false"`
}

func (AgentModelBinding) TableName() string { return "agent_model_bindings" }

// AgentAuthorizationSummary 是固定 Workspace 绑定的脱敏授权摘要。
// +k8s:deepcopy-gen=true
type AgentAuthorizationSummary struct {
	Source      string          `json:"source"`
	ValidatedAt imachinery.Time `json:"validated_at,omitempty"`
}

// AgentWorkspaceBinding 在 Agent 创建事务中固定唯一 Workspace 引用。
// +k8s:deepcopy-gen=true
type AgentWorkspaceBinding struct {
	imachinery.ObjectMeta
	AgentID                    string                    `json:"-" gorm:"column:agent_id;type:text;not null;uniqueIndex"`
	WorkspaceType              string                    `json:"-" gorm:"column:workspace_type;type:text;not null"`
	WorkspaceID                string                    `json:"-" gorm:"column:workspace_id;type:text;not null"`
	AccessMode                 string                    `json:"-" gorm:"column:access_mode;type:text;not null"`
	AuthorizationSummary       AgentAuthorizationSummary `json:"-" gorm:"-"`
	AuthorizationSummaryShadow string                    `json:"-" gorm:"column:authorization_summary_json;type:text;not null;default:'{}'"`
}

func (AgentWorkspaceBinding) TableName() string { return "agent_workspace_bindings" }
func (b *AgentWorkspaceBinding) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (*AgentWorkspaceBinding) AfterCreate(*gorm.DB) error { return nil }
func (b *AgentWorkspaceBinding) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (*AgentWorkspaceBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *AgentWorkspaceBinding) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(b.AuthorizationSummaryShadow, &b.AuthorizationSummary, "{}")
	return nil
}
func (b *AgentWorkspaceBinding) marshalJSON() error {
	return marshalJSONFields(jsonField{b.AuthorizationSummary, &b.AuthorizationSummaryShadow, "{}"})
}

// AgentSkillBinding 保存只读 Skill Definition 的版本化启用配置。
// +k8s:deepcopy-gen=true
type AgentSkillBinding struct {
	imachinery.ObjectMeta
	AgentID             string          `json:"agent_id" gorm:"column:agent_id;type:text;not null;uniqueIndex:idx_agent_skill_binding,priority:1"`
	SkillID             string          `json:"skill_id" gorm:"column:skill_id;type:text;not null;uniqueIndex:idx_agent_skill_binding,priority:2"`
	SkillVersion        string          `json:"skill_version" gorm:"column:skill_version;type:text;not null"`
	Enabled             bool            `json:"enabled" gorm:"column:enabled;not null;default:true"`
	Configuration       json.RawMessage `json:"configuration,omitempty" gorm:"-"`
	ConfigurationShadow string          `json:"-" gorm:"column:configuration_json;type:text;not null;default:'{}'"`
}

func (AgentSkillBinding) TableName() string { return "agent_skill_bindings" }
func (b *AgentSkillBinding) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (*AgentSkillBinding) AfterCreate(*gorm.DB) error { return nil }
func (b *AgentSkillBinding) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (*AgentSkillBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *AgentSkillBinding) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(b.ConfigurationShadow, &b.Configuration, "{}")
	return nil
}
func (b *AgentSkillBinding) marshalJSON() error {
	return marshalJSONFields(jsonField{b.Configuration, &b.ConfigurationShadow, "{}"})
}

// AgentMCPBinding 保存 MCP Endpoint、工具白名单和 CredentialRef，不返回凭证值。
// +k8s:deepcopy-gen=true
type AgentMCPBinding struct {
	imachinery.ObjectMeta
	AgentID             string          `json:"agent_id" gorm:"column:agent_id;type:text;not null;index"`
	ServerType          string          `json:"server_type" gorm:"column:server_type;type:text;not null"`
	EndpointRef         string          `json:"endpoint_ref" gorm:"column:endpoint_ref;type:text;not null"`
	CredentialRef       string          `json:"-" gorm:"column:credential_ref;type:text"`
	AllowedTools        []string        `json:"allowed_tools,omitempty" gorm:"-"`
	AllowedToolsShadow  string          `json:"-" gorm:"column:allowed_tools_json;type:text;not null;default:'[]'"`
	Configuration       json.RawMessage `json:"configuration,omitempty" gorm:"-"`
	ConfigurationShadow string          `json:"-" gorm:"column:configuration_json;type:text;not null;default:'{}'"`
	Enabled             bool            `json:"enabled" gorm:"column:enabled;not null;default:true"`
}

func (AgentMCPBinding) TableName() string { return "agent_mcp_bindings" }
func (b *AgentMCPBinding) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (*AgentMCPBinding) AfterCreate(*gorm.DB) error { return nil }
func (b *AgentMCPBinding) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (*AgentMCPBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *AgentMCPBinding) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(b.AllowedToolsShadow, &b.AllowedTools, "[]")
	unmarshalJSON(b.ConfigurationShadow, &b.Configuration, "{}")
	return nil
}
func (b *AgentMCPBinding) marshalJSON() error {
	return marshalJSONFields(jsonField{b.AllowedTools, &b.AllowedToolsShadow, "[]"}, jsonField{b.Configuration, &b.ConfigurationShadow, "{}"})
}

// AgentRuntimeBinding 保存 Agent 对 Infra Runtime 的业务绑定和健康投影。
// +k8s:deepcopy-gen=true
type AgentRuntimeBinding struct {
	imachinery.ObjectMeta
	AgentID                string          `json:"agent_id" gorm:"column:agent_id;type:text;not null;index"`
	InfraRuntimeID         string          `json:"infra_runtime_id,omitempty" gorm:"column:infra_runtime_id;type:text"`
	EndpointRef            string          `json:"endpoint_ref,omitempty" gorm:"column:endpoint_ref;type:text"`
	RuntimeProfileID       string          `json:"runtime_profile_id" gorm:"column:runtime_profile_id;type:text;not null"`
	RuntimeProfileRevision string          `json:"runtime_profile_revision" gorm:"column:runtime_profile_revision;type:text;not null"`
	State                  string          `json:"state" gorm:"column:state;type:text;not null"`
	ActivityState          string          `json:"activity_state" gorm:"column:activity_state;type:text;not null"`
	HealthStatus           string          `json:"health_status" gorm:"column:health_status;type:text;not null"`
	CurrentTaskID          string          `json:"current_task_id,omitempty" gorm:"column:current_task_id;type:text"`
	CurrentOperation       string          `json:"current_operation,omitempty" gorm:"column:current_operation;type:text"`
	LastHealthAt           imachinery.Time `json:"last_health_at,omitempty" gorm:"column:last_health_at"`
	StartedAt              imachinery.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	StoppedAt              imachinery.Time `json:"stopped_at,omitempty" gorm:"column:stopped_at"`
}

func (AgentRuntimeBinding) TableName() string { return "agent_runtime_bindings" }

// AgentOperationEvent 是单 Invocation 的短期有序 SSE 恢复事实。
// +k8s:deepcopy-gen=true
type AgentOperationEvent struct {
	imachinery.ObjectMeta
	InvocationID  string          `json:"invocation_id" gorm:"column:invocation_id;type:text;not null;uniqueIndex:idx_agent_operation_event_sequence,priority:1"`
	EventType     string          `json:"event_type" gorm:"column:event_type;type:text;not null"`
	SequenceNo    int             `json:"sequence_no" gorm:"column:sequence_no;not null;uniqueIndex:idx_agent_operation_event_sequence,priority:2"`
	Payload       json.RawMessage `json:"payload,omitempty" gorm:"-"`
	PayloadShadow string          `json:"-" gorm:"column:payload_json;type:text;not null;default:'{}'"`
}

func (AgentOperationEvent) TableName() string { return "agent_operation_events" }
func (e *AgentOperationEvent) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (*AgentOperationEvent) AfterCreate(*gorm.DB) error { return nil }
func (e *AgentOperationEvent) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (*AgentOperationEvent) AfterUpdate(*gorm.DB) error { return nil }
func (e *AgentOperationEvent) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(e.PayloadShadow, &e.Payload, "{}")
	return nil
}
func (e *AgentOperationEvent) marshalJSON() error {
	return marshalJSONFields(jsonField{e.Payload, &e.PayloadShadow, "{}"})
}

// AgentOutbox 保存可重放领域事件，正文不得包含消息内容或凭证。
// +k8s:deepcopy-gen=true
type AgentOutbox struct {
	imachinery.ObjectMeta
	AggregateType  string          `json:"aggregate_type" gorm:"column:aggregate_type;type:text;not null"`
	AggregateID    string          `json:"aggregate_id" gorm:"column:aggregate_id;type:text;not null"`
	EventType      string          `json:"event_type" gorm:"column:event_type;type:text;not null"`
	Payload        json.RawMessage `json:"payload" gorm:"-"`
	PayloadShadow  string          `json:"-" gorm:"column:payload_json;type:text;not null"`
	IdempotencyKey string          `json:"idempotency_key" gorm:"column:idempotency_key;type:text;not null;uniqueIndex"`
	DeliveryStatus string          `json:"delivery_status" gorm:"column:delivery_status;type:text;not null;index:idx_agent_outbox_delivery,priority:1"`
	NextAttemptAt  imachinery.Time `json:"next_attempt_at,omitempty" gorm:"column:next_attempt_at;index:idx_agent_outbox_delivery,priority:2"`
	DeliveredAt    imachinery.Time `json:"delivered_at,omitempty" gorm:"column:delivered_at"`
}

func (AgentOutbox) TableName() string { return "agent_outbox" }
func (e *AgentOutbox) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (*AgentOutbox) AfterCreate(*gorm.DB) error { return nil }
func (e *AgentOutbox) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (*AgentOutbox) AfterUpdate(*gorm.DB) error { return nil }
func (e *AgentOutbox) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(e.PayloadShadow, &e.Payload, "{}")
	return nil
}
func (e *AgentOutbox) marshalJSON() error {
	return marshalJSONFields(jsonField{e.Payload, &e.PayloadShadow, "{}"})
}

func beforeAgentJSONCreate(meta *imachinery.ObjectMeta, tx *gorm.DB, marshal func() error) error {
	if err := meta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshal()
}

func beforeAgentJSONUpdate(meta *imachinery.ObjectMeta, tx *gorm.DB, marshal func() error) error {
	if err := meta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshal()
}
