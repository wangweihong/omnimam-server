package iapiserver

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	AIChatOperationChat      = "chat"
	AIChatOperationTranslate = "translate"

	AIChatMessageRoleUser      = "user"
	AIChatMessageRoleAssistant = "assistant"
	AIChatMessageRoleSystem    = "system"

	AIChatStatusQueued      = "queued"
	AIChatStatusGenerating  = "generating"
	AIChatStatusDone        = "done"
	AIChatStatusInterrupted = "interrupted"
	AIChatStatusFailed      = "failed"

	AIChatQuickPhraseScopeGlobal    = "global"
	AIChatQuickPhraseScopeAssistant = "assistant"

	AIChatQuickPhraseTypeText   = "plain"
	AIChatQuickPhraseTypePrompt = "prompt"

	AIChatAttachmentKindImage   = "image"
	AIChatAttachmentDisplayIcon = "icon"

	AIChatImageMaxSizeBytes = 5 * 1024 * 1024
)

type AIChatModel struct {
	ID                   string   `json:"id"`
	OwnerUserID          string   `json:"owner_user_id"`
	Provider             string   `json:"provider"`
	ProviderModelID      string   `json:"provider_model_id"`
	Name                 string   `json:"name"`
	Capabilities         []string `json:"capabilities"`
	Enabled              bool     `json:"enabled"`
	HealthStatus         string   `json:"health_status,omitempty"`
	HealthReason         string   `json:"health_reason,omitempty"`
	IsDefaultTranslation bool     `json:"is_default_translation"`
}

// AIChatAssistant 保存 ai-chat S2 助手配置，用户助手按 OwnerUserID 隔离，系统助手受保护。
type AIChatAssistant struct {
	imachinery.ObjectMeta
	// OwnerUserID 为空时表示系统助手；非空时仅该用户可见和管理。
	OwnerUserID *string `json:"owner_user_id"          gorm:"column:owner_user_id;type:varchar(64);index;uniqueIndex:idx_ai_chat_assistants_owner_name,where:deleted_at = '' AND is_system = FALSE"`
	// System 标识内置助手，内置助手受保护且不能被普通用户删除。
	System bool `json:"is_system"              gorm:"column:is_system;type:boolean;not null;default:false"`
	// SuggestedModelID 指向 model-management 的 provider model，用作该助手推荐模型。
	SuggestedModelID string `json:"suggested_model_id"     gorm:"column:suggested_model_id;type:varchar(64)"`
	// UseSuggestedModel 控制生成时是否强制使用助手推荐模型。
	UseSuggestedModel bool `json:"use_suggested_model"    gorm:"column:use_suggested_model;type:boolean;not null;default:false"`
	// SystemPrompt 保存助手提示词，参与生成上下文但不作为用户消息展示。
	SystemPrompt string `json:"system_prompt"          gorm:"column:system_prompt;type:text;not null;default:''"`
	// ContextMessageCount 控制生成时回溯的历史消息数量，避免上下文无限增长。
	ContextMessageCount int `json:"context_message_count"  gorm:"column:context_message_count;type:integer;not null;default:20"`
	// Stream 表示该助手默认是否启用流式生成。
	Stream bool `json:"stream_enabled"         gorm:"column:stream;type:boolean;not null;default:true"`
	// ToolMode 表示工具调用模式，当前草稿默认 none。
	ToolMode string `json:"tool_call_mode"         gorm:"column:tool_mode;type:varchar(32);not null;default:'none'"`
	// MaxToolCalls 限制单次生成可触发的工具调用次数。
	MaxToolCalls int `json:"max_tool_call_count"    gorm:"column:max_tool_calls;type:integer;not null;default:0"`
	// Temperature 覆盖模型采样温度；为空时使用模型或 provider 默认值。
	Temperature *float64 `json:"temperature"           gorm:"column:temperature;type:numeric(4,3)"`
	// TopP 覆盖 nucleus sampling 参数；为空时使用模型或 provider 默认值。
	TopP *float64 `json:"top_p"                 gorm:"column:top_p;type:numeric(4,3)"`
	// MaxTokens 限制生成 token 数；为空时不在助手层覆盖。
	MaxTokens *int `json:"max_tokens"            gorm:"column:max_tokens;type:integer"`
	// CustomParameters 保存运行时额外参数，当前通过 shadow JSONB 字段存储。
	CustomParameters map[string]any `json:"custom_params,omitempty" gorm:"-"`
	// CustomParametersShadow 是 CustomParameters 的 JSONB 存储字段。
	CustomParametersShadow string `json:"-"                     gorm:"column:runtime_config_json;type:jsonb;not null;default:'{}'"`
	// DeletedAt 用于软删除用户助手，系统助手不走该删除路径。
	DeletedAt string `json:"-"                        gorm:"column:deleted_at;type:text;default:'';index"`
}

func (AIChatAssistant) TableName() string { return "ai_chat_assistants" }

func (a *AIChatAssistant) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	applyAIChatAssistantDefaults(a)
	return marshalAIChatJSON(a.CustomParameters, &a.CustomParametersShadow)
}

func (a *AIChatAssistant) AfterCreate(tx *gorm.DB) error { return nil }

func (a *AIChatAssistant) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	applyAIChatAssistantDefaults(a)
	return marshalAIChatJSON(a.CustomParameters, &a.CustomParametersShadow)
}

func (a *AIChatAssistant) AfterUpdate(tx *gorm.DB) error { return nil }

func (a *AIChatAssistant) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalAIChatJSON(a.CustomParametersShadow, &a.CustomParameters)
}

// AIChatTopic 保存用户会话入口，负责串联助手、默认模型和消息历史。
type AIChatTopic struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识话题所属用户，列表和详情均按该字段隔离。
	OwnerUserID string `json:"owner_user_id"            gorm:"column:owner_user_id;type:varchar(64);not null;index:idx_ai_chat_topics_owner_activity,priority:1"`
	// Title 是会话标题，创建时可由首条用户消息或请求标题产生。
	Title string `json:"title"                    gorm:"column:title;type:varchar(160);not null"`
	// Pinned 控制话题在用户列表中的置顶排序。
	Pinned bool `json:"pinned"                   gorm:"column:pinned;type:boolean;not null;default:false;index:idx_ai_chat_topics_owner_activity,priority:2,sort:desc"`
	// AssistantID 记录话题默认助手，生成时可被请求级参数覆盖。
	AssistantID string `json:"assistant_id"             gorm:"column:assistant_id;type:varchar(64)"`
	// ModelID 指向 model-management 的 provider model，用作话题默认模型。
	ModelID string `json:"model_id"                 gorm:"column:model_id;type:varchar(64)"`
	// BranchSourceTopicID 记录分支来源话题，普通话题为空。
	BranchSourceTopicID string `json:"branch_source_topic_id"   gorm:"column:branch_source_topic_id;type:varchar(64);index:idx_ai_chat_topics_branch_source,priority:1"`
	// BranchSourceMessageID 记录分支来源消息，用于回溯上下文来源。
	BranchSourceMessageID string `json:"branch_source_message_id" gorm:"column:branch_source_message_id;type:varchar(64);index:idx_ai_chat_topics_branch_source,priority:2"`
	// LastActiveAt 用于话题列表按最近交互排序，消息创建和生成状态变化会更新。
	LastActiveAt time.Time `json:"last_active_at"           gorm:"column:last_active_at;index:idx_ai_chat_topics_owner_activity,priority:3,sort:desc"`
	// DeletedAt 用于用户话题软删除，保留历史消息数据。
	DeletedAt string `json:"-"                        gorm:"column:deleted_at;type:text;default:'';index"`
}

func (AIChatTopic) TableName() string { return "ai_chat_topics" }

func (t *AIChatTopic) BeforeCreate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if t.Name == "" {
		t.Name = t.Title
	}
	if t.LastActiveAt.IsZero() {
		t.LastActiveAt = t.CreatedAt.Time
	}
	return nil
}

func (t *AIChatTopic) AfterCreate(tx *gorm.DB) error { return nil }

func (t *AIChatTopic) BeforeUpdate(tx *gorm.DB) error {
	return t.ObjectMeta.BeforeUpdate(tx)
}

func (t *AIChatTopic) AfterUpdate(tx *gorm.DB) error { return nil }

// AIChatMessage 保存话题内消息事实，生成相关快照随消息固化以便审计。
type AIChatMessage struct {
	imachinery.ObjectMeta
	// TopicID 指向所属话题，消息列表按该字段和版本顺序读取。
	TopicID string `json:"topic_id"          gorm:"column:topic_id;type:varchar(64);not null;index:idx_ai_chat_messages_topic_order,priority:1;uniqueIndex:uq_ai_chat_messages_client_id,where:client_message_id IS NOT NULL"`
	// OwnerUserID 标识消息所属用户，避免跨用户访问话题消息。
	OwnerUserID string `json:"owner_user_id"     gorm:"column:owner_user_id;type:varchar(64);not null;index"`
	// Role 表示消息角色，取值来自 ai-chat S2 的 user/assistant/system。
	Role string `json:"role"              gorm:"column:role;type:varchar(24);not null"`
	// Content 保存消息正文，图片消息也可携带空文本并通过附件摘要展示。
	Content string `json:"content"           gorm:"column:content;type:text;not null;default:''"`
	// Status 表示消息生成状态，用户消息通常直接为 done。
	Status string `json:"status"            gorm:"column:status;type:varchar(24);not null"`
	// Version 支持同一消息的编辑/重生成版本排序。
	Version int `json:"version"           gorm:"column:version;type:integer;not null;default:1;index:idx_ai_chat_messages_topic_order,priority:3"`
	// ParentMessageID 指向生成或分支所依赖的上游消息。
	ParentMessageID string `json:"parent_message_id" gorm:"column:parent_message_id;type:varchar(64);index:idx_ai_chat_messages_parent"`
	// ModelSnapshot 保存生成时选用模型的快照，避免后续模型配置变更影响历史记录。
	ModelSnapshot map[string]any `json:"model_snapshot"    gorm:"-"`
	// ModelSnapshotShadow 是 ModelSnapshot 的 JSONB 存储字段。
	ModelSnapshotShadow string `json:"-"                 gorm:"column:model_snapshot_json;type:jsonb;not null;default:'{}'"`
	// AssistantSnapshot 保存生成时助手配置快照，用于审计历史输出来源。
	AssistantSnapshot map[string]any `json:"assistant_snapshot" gorm:"-"`
	// AssistantSnapshotShadow 是 AssistantSnapshot 的 JSONB 存储字段。
	AssistantSnapshotShadow string `json:"-"                 gorm:"column:assistant_snapshot_json;type:jsonb;not null;default:'{}'"`
	// AttachmentIcons 保存附件的轻量展示图标，避免列表接口返回原始内容。
	AttachmentIcons []string `json:"attachment_icons"  gorm:"-"`
	// AttachmentIconsShadow 是 AttachmentIcons 的 JSONB 存储字段。
	AttachmentIconsShadow string `json:"-"                 gorm:"column:attachment_icons_json;type:jsonb;not null;default:'[]'"`
	// ClientMessageID 是客户端幂等键，用于避免重复提交用户消息。
	ClientMessageID string `json:"client_message_id" gorm:"column:client_message_id;type:varchar(128);uniqueIndex:uq_ai_chat_messages_client_id,where:client_message_id IS NOT NULL"`
	// TruncatedAfter 标识该消息之后的上下文被截断，分支和再生成会参考该标记。
	TruncatedAfter bool `json:"truncated_after"   gorm:"column:truncated_after;type:boolean;not null;default:false"`
}

func (AIChatMessage) TableName() string { return "ai_chat_messages" }

func (m *AIChatMessage) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if m.Version == 0 {
		m.Version = 1
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if err := marshalAIChatJSON(m.ModelSnapshot, &m.ModelSnapshotShadow); err != nil {
		return err
	}
	if err := marshalAIChatJSON(m.AssistantSnapshot, &m.AssistantSnapshotShadow); err != nil {
		return err
	}
	return marshalAIChatJSON(m.AttachmentIcons, &m.AttachmentIconsShadow)
}

func (m *AIChatMessage) AfterCreate(tx *gorm.DB) error { return nil }

func (m *AIChatMessage) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	if err := marshalAIChatJSON(m.ModelSnapshot, &m.ModelSnapshotShadow); err != nil {
		return err
	}
	if err := marshalAIChatJSON(m.AssistantSnapshot, &m.AssistantSnapshotShadow); err != nil {
		return err
	}
	return marshalAIChatJSON(m.AttachmentIcons, &m.AttachmentIconsShadow)
}

func (m *AIChatMessage) AfterUpdate(tx *gorm.DB) error { return nil }

func (m *AIChatMessage) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if err := unmarshalAIChatJSON(m.ModelSnapshotShadow, &m.ModelSnapshot); err != nil {
		return err
	}
	if err := unmarshalAIChatJSON(m.AssistantSnapshotShadow, &m.AssistantSnapshot); err != nil {
		return err
	}
	return unmarshalAIChatJSON(m.AttachmentIconsShadow, &m.AttachmentIcons)
}

// AIChatGeneration 记录一次模型生成过程，承载 stop/regenerate 等操作的运行状态。
type AIChatGeneration struct {
	imachinery.ObjectMeta
	// TopicID 指向生成所属话题，同一话题同时只允许一个 queued/generating 运行。
	TopicID string `json:"topic_id"             gorm:"column:topic_id;type:varchar(64);not null;uniqueIndex:uq_ai_chat_generations_active_topic,where:status = 'queued' OR status = 'generating'"`
	// OwnerUserID 标识生成所属用户，用于查询和停止操作的权限边界。
	OwnerUserID string `json:"owner_user_id"        gorm:"column:owner_user_id;type:varchar(64);not null;index:idx_ai_chat_generations_owner_status,priority:1"`
	// AssistantMessageID 指向承载模型输出的 assistant 消息。
	AssistantMessageID string `json:"assistant_message_id" gorm:"column:assistant_message_id;type:varchar(64);not null"`
	// Operation 表示本次生成用途，当前包括 chat 和 translate。
	Operation string `json:"operation"            gorm:"column:operation;type:varchar(24);not null"`
	// Status 表示生成生命周期状态，停止和失败会写入终态。
	Status string `json:"status"               gorm:"column:status;type:varchar(24);not null;index:idx_ai_chat_generations_owner_status,priority:2"`
	// ModelID 指向实际使用的 provider model，来自请求、话题、助手或默认模型解析。
	ModelID string `json:"model_id"             gorm:"column:model_id;type:varchar(64);not null"`
	// AssistantID 记录本次生成使用的助手，纯翻译或默认生成可为空。
	AssistantID string `json:"assistant_id"         gorm:"column:assistant_id;type:varchar(64)"`
	// StartedAt 记录模型调用开始时间，用于前端展示和运行诊断。
	StartedAt *time.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	// CompletedAt 记录生成完成或失败时间，对外沿用当前 draft 的 finished_at 字段。
	CompletedAt *time.Time `json:"finished_at,omitempty" gorm:"column:finished_at"`
	// StoppedAt 是停止请求的运行态字段，当前不作为 S2 响应字段落库。
	StoppedAt *time.Time `json:"-" gorm:"-"`
	// ErrorCode 保存生成失败时的业务错误码，成功生成时为空。
	ErrorCode string `json:"error_code,omitempty" gorm:"column:error_code;type:varchar(64)"`
	// ErrorMessage 保存生成失败时的可观测错误信息，避免静默吞错。
	ErrorMessage string `json:"error_message,omitempty" gorm:"column:error_message;type:text"`
}

func (AIChatGeneration) TableName() string { return "ai_chat_generation_runs" }

func (g *AIChatGeneration) BeforeCreate(tx *gorm.DB) error {
	return g.ObjectMeta.BeforeCreate(tx)
}

func (g *AIChatGeneration) AfterCreate(tx *gorm.DB) error { return nil }

func (g *AIChatGeneration) BeforeUpdate(tx *gorm.DB) error {
	return g.ObjectMeta.BeforeUpdate(tx)
}

func (g *AIChatGeneration) AfterUpdate(tx *gorm.DB) error { return nil }

// AIChatQuickPhrase 保存用户快捷短语，支持全局和助手级作用域。
type AIChatQuickPhrase struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识短语所属用户，系统不会跨用户共享用户短语。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:varchar(64);not null;index:idx_ai_chat_quick_phrases_owner_scope,priority:1"`
	// Title 是短语在选择器中的展示标题。
	Title string `json:"title"         gorm:"column:title;type:varchar(128);not null"`
	// Content 是插入到输入框或提示词中的实际文本。
	Content string `json:"content"       gorm:"column:content;type:text;not null"`
	// Scope 表示短语作用域，global 可跨助手使用，assistant 需绑定 AssistantID。
	Scope string `json:"scope"         gorm:"column:scope;type:varchar(24);not null;index:idx_ai_chat_quick_phrases_owner_scope,priority:2"`
	// AssistantID 在 assistant 作用域下限定短语所属助手。
	AssistantID string `json:"assistant_id"  gorm:"column:assistant_id;type:varchar(64);index:idx_ai_chat_quick_phrases_owner_scope,priority:3"`
	// PhraseType 区分普通文本和提示词短语，默认 plain。
	PhraseType string `json:"phrase_type"   gorm:"column:phrase_type;type:varchar(24);not null;default:'plain'"`
	// DeletedAt 用于软删除快捷短语，保留历史引用可能性。
	DeletedAt string `json:"-"                        gorm:"column:deleted_at;type:text;default:'';index"`
}

func (AIChatQuickPhrase) TableName() string { return "ai_chat_quick_phrases" }

func (p *AIChatQuickPhrase) BeforeCreate(tx *gorm.DB) error {
	if err := p.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if p.Name == "" {
		p.Name = p.Title
	}
	if p.PhraseType == "" {
		p.PhraseType = AIChatQuickPhraseTypeText
	}
	return nil
}

func (p *AIChatQuickPhrase) AfterCreate(tx *gorm.DB) error { return nil }

func (p *AIChatQuickPhrase) BeforeUpdate(tx *gorm.DB) error {
	if err := p.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	if p.PhraseType == "" {
		p.PhraseType = AIChatQuickPhraseTypeText
	}
	return nil
}

func (p *AIChatQuickPhrase) AfterUpdate(tx *gorm.DB) error { return nil }

// AIChatMessageTranslation 保存消息或纯文本翻译结果，模型快照随结果固化。
type AIChatMessageTranslation struct {
	imachinery.ObjectMeta
	// MessageID 指向被翻译的消息；纯文本翻译请求允许为空。
	MessageID string `json:"message_id"         gorm:"column:message_id;type:varchar(64);not null;index:idx_ai_chat_message_translations_message,priority:1"`
	// OwnerUserID 标识翻译结果所属用户，用于消息可见性校验。
	OwnerUserID string `json:"owner_user_id"      gorm:"column:owner_user_id;type:varchar(64);not null;index"`
	// SourceLanguage 保存检测或请求指定的源语言，可为空表示自动检测。
	SourceLanguage string `json:"source_language"    gorm:"column:source_language;type:varchar(32)"`
	// TargetLanguage 保存目标语言，是翻译请求的必填约束。
	TargetLanguage string `json:"target_language"    gorm:"column:target_language;type:varchar(32);not null;index:idx_ai_chat_message_translations_message,priority:2"`
	// TranslatedContent 保存翻译后的文本内容。
	TranslatedContent string `json:"translated_content" gorm:"column:translated_content;type:text;not null"`
	// ModelSnapshot 保存翻译时使用的 provider model 快照。
	ModelSnapshot map[string]any `json:"model_snapshot" gorm:"-"`
	// ModelSnapshotShadow 是 ModelSnapshot 的 JSONB 存储字段。
	ModelSnapshotShadow string `json:"-"                  gorm:"column:model_snapshot_json;type:jsonb;not null;default:'{}'"`
}

func (AIChatMessageTranslation) TableName() string { return "ai_chat_message_translations" }

func (t *AIChatMessageTranslation) BeforeCreate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalAIChatJSON(t.ModelSnapshot, &t.ModelSnapshotShadow)
}

func (t *AIChatMessageTranslation) AfterCreate(tx *gorm.DB) error { return nil }

func (t *AIChatMessageTranslation) BeforeUpdate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalAIChatJSON(t.ModelSnapshot, &t.ModelSnapshotShadow)
}

func (t *AIChatMessageTranslation) AfterUpdate(tx *gorm.DB) error { return nil }

func (t *AIChatMessageTranslation) AfterFind(tx *gorm.DB) error {
	if err := t.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalAIChatJSON(t.ModelSnapshotShadow, &t.ModelSnapshot)
}

// AIChatMessageAttachment 是旧图片附件投影，当前仅保留轻量元数据，不返回原始内容。
type AIChatMessageAttachment struct {
	imachinery.ObjectMeta
	// MessageID 指向附件所属消息。
	MessageID string `json:"message_id"    gorm:"column:message_id;type:varchar(64);not null;index:idx_ai_chat_message_attachments_message"`
	// OwnerUserID 标识附件所属用户，用于跟随消息访问边界。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:varchar(64);not null;index"`
	// Kind 表示附件类型，当前 ai-chat 仅接收图片附件。
	Kind string `json:"kind"          gorm:"column:kind;type:varchar(24);not null"`
	// MimeType 保存上传图片 MIME 类型，受请求 DTO 白名单约束。
	MimeType string `json:"mime_type"     gorm:"column:mime_type;type:varchar(64);not null"`
	// FileName 保存客户端文件名，仅用于展示和诊断。
	FileName string `json:"file_name"     gorm:"column:file_name;type:varchar(255)"`
	// SizeBytes 保存附件大小，请求层限制最大 5MiB。
	SizeBytes int `json:"size_bytes"    gorm:"column:size_bytes;type:integer;not null"`
	// DisplayMode 控制附件在消息列表中的轻量展示方式，默认 icon。
	DisplayMode string `json:"display_mode"  gorm:"column:display_mode;type:varchar(24);not null;default:'icon'"`
	// Stored 表示附件是否已持久化；当前路径不保存原图内容。
	Stored bool `json:"stored"        gorm:"column:stored;type:boolean;not null;default:false"`
}

func (AIChatMessageAttachment) TableName() string { return "ai_chat_message_attachments" }

func (a *AIChatMessageAttachment) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if a.Kind == "" {
		a.Kind = AIChatAttachmentKindImage
	}
	if a.DisplayMode == "" {
		a.DisplayMode = AIChatAttachmentDisplayIcon
	}
	a.Stored = false
	return nil
}

func (a *AIChatMessageAttachment) AfterCreate(tx *gorm.DB) error { return nil }
func (a *AIChatMessageAttachment) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	a.Stored = false
	return nil
}
func (a *AIChatMessageAttachment) AfterUpdate(tx *gorm.DB) error { return nil }

func marshalAIChatJSON(value any, target *string) error {
	if value == nil {
		if target != nil && *target == "" {
			*target = "{}"
		}
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	*target = string(data)
	return nil
}

func unmarshalAIChatJSON[T any](source string, target *T) error {
	if source == "" {
		return nil
	}
	return json.Unmarshal([]byte(source), target)
}

func applyAIChatAssistantDefaults(a *AIChatAssistant) {
	if a.ContextMessageCount == 0 {
		a.ContextMessageCount = 20
	}
	if a.ToolMode == "" {
		a.ToolMode = "none"
	}
	if a.CustomParameters == nil {
		a.CustomParameters = map[string]any{}
	}
}
