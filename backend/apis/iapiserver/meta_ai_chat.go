package iapiserver

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

	AIChatQuickPhraseTypeText   = "text"
	AIChatQuickPhraseTypePrompt = "prompt"

	AIChatAttachmentKindImage   = "image"
	AIChatAttachmentDisplayIcon = "icon"

	AIChatImageMaxSizeBytes = 5 * 1024 * 1024
)

type AIChatModel struct {
	ID                   string    `json:"id"                     gorm:"primary_key;column:id;type:varchar(64)"`
	OwnerUserID          string    `json:"owner_user_id"          gorm:"column:owner_user_id;type:varchar(64);not null;uniqueIndex:uq_ai_chat_models_owner_provider_model;index:idx_ai_chat_models_owner_enabled,priority:1"`
	Provider             string    `json:"provider"               gorm:"column:provider;type:varchar(64);not null;uniqueIndex:uq_ai_chat_models_owner_provider_model"`
	ProviderModelID      string    `json:"provider_model_id"      gorm:"column:provider_model_id;type:varchar(128);not null;uniqueIndex:uq_ai_chat_models_owner_provider_model"`
	Name                 string    `json:"name"                   gorm:"column:name;type:varchar(128);not null"`
	Capabilities         []string  `json:"capabilities"           gorm:"-"`
	CapabilitiesShadow   string    `json:"-"                      gorm:"column:capabilities;type:jsonb;not null;default:'[]'"`
	Enabled              bool      `json:"enabled"                gorm:"column:enabled;type:boolean;not null;default:true;index:idx_ai_chat_models_owner_enabled,priority:2"`
	HealthStatus         string    `json:"health_status,omitempty" gorm:"-"`
	HealthReason         string    `json:"health_reason,omitempty" gorm:"-"`
	IsDefaultTranslation bool      `json:"is_default_translation" gorm:"column:is_default_translation;type:boolean;not null;default:false"`
	CreatedAt            time.Time `json:"created_at"             gorm:"column:created_at"`
	UpdatedAt            time.Time `json:"updated_at"             gorm:"column:updated_at"`
}

func (AIChatModel) TableName() string { return "ai_chat_models" }

func (m *AIChatModel) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	return marshalAIChatJSON(m.Capabilities, &m.CapabilitiesShadow)
}

func (m *AIChatModel) AfterCreate(tx *gorm.DB) error { return nil }

func (m *AIChatModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return marshalAIChatJSON(m.Capabilities, &m.CapabilitiesShadow)
}

func (m *AIChatModel) AfterUpdate(tx *gorm.DB) error { return nil }

func (m *AIChatModel) AfterFind(tx *gorm.DB) error {
	return unmarshalAIChatJSON(m.CapabilitiesShadow, &m.Capabilities)
}

type AIChatAssistant struct {
	ID                     string         `json:"id"                    gorm:"primary_key;column:id;type:varchar(64)"`
	OwnerUserID            *string        `json:"owner_user_id"         gorm:"column:owner_user_id;type:varchar(64);index;uniqueIndex:uq_ai_chat_assistants_owner_name,where:deleted_at IS NULL AND system = FALSE"`
	Name                   string         `json:"name"                  gorm:"column:name;type:varchar(128);not null;uniqueIndex:uq_ai_chat_assistants_owner_name,where:deleted_at IS NULL AND system = FALSE;uniqueIndex:uq_ai_chat_assistants_system_name,where:deleted_at IS NULL AND system = TRUE"`
	System                 bool           `json:"system"                gorm:"column:system;type:boolean;not null;default:false;uniqueIndex:uq_ai_chat_assistants_owner_name,where:deleted_at IS NULL AND system = FALSE;uniqueIndex:uq_ai_chat_assistants_system_name,where:deleted_at IS NULL AND system = TRUE"`
	SuggestedModelID       string         `json:"suggested_model_id"    gorm:"column:suggested_model_id;type:varchar(64)"`
	UseSuggestedModel      bool           `json:"use_suggested_model"   gorm:"column:use_suggested_model;type:boolean;not null;default:false"`
	SystemPrompt           string         `json:"system_prompt"         gorm:"column:system_prompt;type:text;not null;default:''"`
	ContextMessageCount    int            `json:"context_message_count" gorm:"column:context_message_count;type:integer;not null;default:20"`
	Stream                 bool           `json:"stream"                gorm:"column:stream;type:boolean;not null;default:true"`
	ToolMode               string         `json:"tool_mode"             gorm:"column:tool_mode;type:varchar(32);not null;default:'none'"`
	MaxToolCalls           int            `json:"max_tool_calls"        gorm:"column:max_tool_calls;type:integer;not null;default:0"`
	Temperature            *float64       `json:"temperature"           gorm:"column:temperature;type:numeric(4,3)"`
	TopP                   *float64       `json:"top_p"                 gorm:"column:top_p;type:numeric(4,3)"`
	MaxTokens              *int           `json:"max_tokens"            gorm:"column:max_tokens;type:integer"`
	CustomParameters       map[string]any `json:"runtime_config" gorm:"-"`
	CustomParametersShadow string         `json:"-"                     gorm:"column:custom_parameters;type:jsonb;not null;default:'{}'"`
	CreatedAt              time.Time      `json:"created_at"            gorm:"column:created_at"`
	UpdatedAt              time.Time      `json:"updated_at"            gorm:"column:updated_at"`
	DeletedAt              *time.Time     `json:"deleted_at,omitempty"  gorm:"column:deleted_at;index"`
}

func (AIChatAssistant) TableName() string { return "ai_chat_assistants" }

func (a *AIChatAssistant) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	applyAIChatAssistantDefaults(a)
	return marshalAIChatJSON(a.CustomParameters, &a.CustomParametersShadow)
}

func (a *AIChatAssistant) AfterCreate(tx *gorm.DB) error { return nil }

func (a *AIChatAssistant) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now()
	applyAIChatAssistantDefaults(a)
	return marshalAIChatJSON(a.CustomParameters, &a.CustomParametersShadow)
}

func (a *AIChatAssistant) AfterUpdate(tx *gorm.DB) error { return nil }

func (a *AIChatAssistant) AfterFind(tx *gorm.DB) error {
	return unmarshalAIChatJSON(a.CustomParametersShadow, &a.CustomParameters)
}

type AIChatTopic struct {
	ID                    string     `json:"id"                       gorm:"primary_key;column:id;type:varchar(64)"`
	OwnerUserID           string     `json:"owner_user_id"            gorm:"column:owner_user_id;type:varchar(64);not null;index:idx_ai_chat_topics_owner_activity,priority:1"`
	Title                 string     `json:"title"                    gorm:"column:title;type:varchar(160);not null"`
	Pinned                bool       `json:"pinned"                   gorm:"column:pinned;type:boolean;not null;default:false;index:idx_ai_chat_topics_owner_activity,priority:2,sort:desc"`
	AssistantID           string     `json:"assistant_id"             gorm:"column:assistant_id;type:varchar(64)"`
	ModelID               string     `json:"model_id"                 gorm:"column:model_id;type:varchar(64)"`
	BranchSourceTopicID   string     `json:"branch_source_topic_id"   gorm:"column:branch_source_topic_id;type:varchar(64);index:idx_ai_chat_topics_branch_source,priority:1"`
	BranchSourceMessageID string     `json:"branch_source_message_id" gorm:"column:branch_source_message_id;type:varchar(64);index:idx_ai_chat_topics_branch_source,priority:2"`
	LastActiveAt          time.Time  `json:"last_active_at"           gorm:"column:last_active_at;index:idx_ai_chat_topics_owner_activity,priority:3,sort:desc"`
	CreatedAt             time.Time  `json:"created_at"               gorm:"column:created_at"`
	UpdatedAt             time.Time  `json:"updated_at"               gorm:"column:updated_at"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"     gorm:"column:deleted_at;index"`
}

func (AIChatTopic) TableName() string { return "ai_chat_topics" }

func (t *AIChatTopic) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if t.LastActiveAt.IsZero() {
		t.LastActiveAt = t.CreatedAt
	}
	return nil
}

func (t *AIChatTopic) AfterCreate(tx *gorm.DB) error { return nil }

func (t *AIChatTopic) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = time.Now()
	return nil
}

func (t *AIChatTopic) AfterUpdate(tx *gorm.DB) error { return nil }

type AIChatMessage struct {
	ID                      string                     `json:"id"                gorm:"primary_key;column:id;type:varchar(64)"`
	TopicID                 string                     `json:"topic_id"          gorm:"column:topic_id;type:varchar(64);not null;index:idx_ai_chat_messages_topic_order,priority:1;uniqueIndex:uq_ai_chat_messages_client_id,where:client_message_id IS NOT NULL"`
	OwnerUserID             string                     `json:"owner_user_id"     gorm:"column:owner_user_id;type:varchar(64);not null;index"`
	Role                    string                     `json:"role"              gorm:"column:role;type:varchar(24);not null"`
	Content                 string                     `json:"content"           gorm:"column:content;type:text;not null;default:''"`
	Status                  string                     `json:"status"            gorm:"column:status;type:varchar(24);not null"`
	Version                 int                        `json:"version"           gorm:"column:version;type:integer;not null;default:1;index:idx_ai_chat_messages_topic_order,priority:3"`
	ParentMessageID         string                     `json:"parent_message_id" gorm:"column:parent_message_id;type:varchar(64);index:idx_ai_chat_messages_parent"`
	ModelSnapshot           map[string]any             `json:"model_snapshot"     gorm:"-"`
	ModelSnapshotShadow     string                     `json:"-"                 gorm:"column:model_snapshot;type:jsonb;not null;default:'{}'"`
	AssistantSnapshot       map[string]any             `json:"assistant_snapshot" gorm:"-"`
	AssistantSnapshotShadow string                     `json:"-"                 gorm:"column:assistant_snapshot;type:jsonb;not null;default:'{}'"`
	ClientMessageID         string                     `json:"client_message_id" gorm:"column:client_message_id;type:varchar(128);uniqueIndex:uq_ai_chat_messages_client_id,where:client_message_id IS NOT NULL"`
	TruncatedAfter          bool                       `json:"truncated_after"   gorm:"column:truncated_after;type:boolean;not null;default:false"`
	CreatedAt               time.Time                  `json:"created_at"        gorm:"column:created_at;index:idx_ai_chat_messages_topic_order,priority:2,sort:asc"`
	UpdatedAt               time.Time                  `json:"updated_at"        gorm:"column:updated_at"`
	Attachments             []*AIChatMessageAttachment `json:"attachments,omitempty" gorm:"-"`
}

func (AIChatMessage) TableName() string { return "ai_chat_messages" }

func (m *AIChatMessage) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if m.Version == 0 {
		m.Version = 1
	}
	if err := marshalAIChatJSON(m.ModelSnapshot, &m.ModelSnapshotShadow); err != nil {
		return err
	}
	return marshalAIChatJSON(m.AssistantSnapshot, &m.AssistantSnapshotShadow)
}

func (m *AIChatMessage) AfterCreate(tx *gorm.DB) error { return nil }

func (m *AIChatMessage) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	if err := marshalAIChatJSON(m.ModelSnapshot, &m.ModelSnapshotShadow); err != nil {
		return err
	}
	return marshalAIChatJSON(m.AssistantSnapshot, &m.AssistantSnapshotShadow)
}

func (m *AIChatMessage) AfterUpdate(tx *gorm.DB) error { return nil }

func (m *AIChatMessage) AfterFind(tx *gorm.DB) error {
	if err := unmarshalAIChatJSON(m.ModelSnapshotShadow, &m.ModelSnapshot); err != nil {
		return err
	}
	return unmarshalAIChatJSON(m.AssistantSnapshotShadow, &m.AssistantSnapshot)
}

type AIChatGeneration struct {
	ID                 string     `json:"id"                   gorm:"primary_key;column:id;type:varchar(64)"`
	TopicID            string     `json:"topic_id"             gorm:"column:topic_id;type:varchar(64);not null;uniqueIndex:uq_ai_chat_generations_active_topic,where:status = 'queued' OR status = 'generating'"`
	OwnerUserID        string     `json:"owner_user_id"        gorm:"column:owner_user_id;type:varchar(64);not null;index:idx_ai_chat_generations_owner_status,priority:1"`
	AssistantMessageID string     `json:"assistant_message_id" gorm:"column:assistant_message_id;type:varchar(64);not null"`
	Operation          string     `json:"operation"            gorm:"column:operation;type:varchar(24);not null"`
	Status             string     `json:"status"               gorm:"column:status;type:varchar(24);not null;index:idx_ai_chat_generations_owner_status,priority:2"`
	ModelID            string     `json:"model_id"             gorm:"column:model_id;type:varchar(64);not null"`
	AssistantID        string     `json:"assistant_id"         gorm:"column:assistant_id;type:varchar(64)"`
	StartedAt          *time.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	StoppedAt          *time.Time `json:"stopped_at,omitempty" gorm:"column:stopped_at"`
	ErrorCode          string     `json:"error_code,omitempty" gorm:"column:error_code;type:varchar(64)"`
	ErrorMessage       string     `json:"error_message,omitempty" gorm:"column:error_message;type:text"`
	CreatedAt          time.Time  `json:"created_at"           gorm:"column:created_at;index:idx_ai_chat_generations_owner_status,priority:3,sort:desc"`
	UpdatedAt          time.Time  `json:"updated_at"           gorm:"column:updated_at"`
}

func (AIChatGeneration) TableName() string { return "ai_chat_generations" }

func (g *AIChatGeneration) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&g.ID, &g.CreatedAt, &g.UpdatedAt)
	return nil
}

func (g *AIChatGeneration) AfterCreate(tx *gorm.DB) error { return nil }

func (g *AIChatGeneration) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = time.Now()
	return nil
}

func (g *AIChatGeneration) AfterUpdate(tx *gorm.DB) error { return nil }

type AIChatQuickPhrase struct {
	ID          string     `json:"id"           gorm:"primary_key;column:id;type:varchar(64)"`
	OwnerUserID string     `json:"owner_user_id" gorm:"column:owner_user_id;type:varchar(64);not null;index:idx_ai_chat_quick_phrases_owner_scope,priority:1"`
	Title       string     `json:"title"         gorm:"column:title;type:varchar(128);not null"`
	Content     string     `json:"content"       gorm:"column:content;type:text;not null"`
	Scope       string     `json:"scope"         gorm:"column:scope;type:varchar(24);not null;index:idx_ai_chat_quick_phrases_owner_scope,priority:2"`
	AssistantID string     `json:"assistant_id"  gorm:"column:assistant_id;type:varchar(64);index:idx_ai_chat_quick_phrases_owner_scope,priority:3"`
	PhraseType  string     `json:"phrase_type"   gorm:"column:phrase_type;type:varchar(24);not null;default:'text'"`
	CreatedAt   time.Time  `json:"created_at"    gorm:"column:created_at"`
	UpdatedAt   time.Time  `json:"updated_at"    gorm:"column:updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" gorm:"column:deleted_at;index"`
}

func (AIChatQuickPhrase) TableName() string { return "ai_chat_quick_phrases" }

func (p *AIChatQuickPhrase) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if p.PhraseType == "" {
		p.PhraseType = AIChatQuickPhraseTypeText
	}
	return nil
}

func (p *AIChatQuickPhrase) AfterCreate(tx *gorm.DB) error { return nil }

func (p *AIChatQuickPhrase) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	if p.PhraseType == "" {
		p.PhraseType = AIChatQuickPhraseTypeText
	}
	return nil
}

func (p *AIChatQuickPhrase) AfterUpdate(tx *gorm.DB) error { return nil }

type AIChatMessageTranslation struct {
	ID                  string         `json:"id"                 gorm:"primary_key;column:id;type:varchar(64)"`
	MessageID           string         `json:"message_id"         gorm:"column:message_id;type:varchar(64);not null;index:idx_ai_chat_message_translations_message,priority:1"`
	OwnerUserID         string         `json:"owner_user_id"      gorm:"column:owner_user_id;type:varchar(64);not null;index"`
	SourceLanguage      string         `json:"source_language"    gorm:"column:source_language;type:varchar(32)"`
	TargetLanguage      string         `json:"target_language"    gorm:"column:target_language;type:varchar(32);not null;index:idx_ai_chat_message_translations_message,priority:2"`
	TranslatedContent   string         `json:"translated_content" gorm:"column:translated_content;type:text;not null"`
	ModelSnapshot       map[string]any `json:"model_snapshot" gorm:"-"`
	ModelSnapshotShadow string         `json:"-"                  gorm:"column:model_snapshot;type:jsonb;not null;default:'{}'"`
	CreatedAt           time.Time      `json:"created_at"         gorm:"column:created_at"`
}

func (AIChatMessageTranslation) TableName() string { return "ai_chat_message_translations" }

func (t *AIChatMessageTranslation) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&t.ID, &t.CreatedAt, nil)
	return marshalAIChatJSON(t.ModelSnapshot, &t.ModelSnapshotShadow)
}

func (t *AIChatMessageTranslation) AfterCreate(tx *gorm.DB) error { return nil }

func (t *AIChatMessageTranslation) BeforeUpdate(tx *gorm.DB) error {
	return marshalAIChatJSON(t.ModelSnapshot, &t.ModelSnapshotShadow)
}

func (t *AIChatMessageTranslation) AfterUpdate(tx *gorm.DB) error { return nil }

func (t *AIChatMessageTranslation) AfterFind(tx *gorm.DB) error {
	return unmarshalAIChatJSON(t.ModelSnapshotShadow, &t.ModelSnapshot)
}

type AIChatMessageAttachment struct {
	ID          string    `json:"id"           gorm:"primary_key;column:id;type:varchar(64)"`
	MessageID   string    `json:"message_id"   gorm:"column:message_id;type:varchar(64);not null;index:idx_ai_chat_message_attachments_message"`
	OwnerUserID string    `json:"owner_user_id" gorm:"column:owner_user_id;type:varchar(64);not null;index"`
	Kind        string    `json:"kind"         gorm:"column:kind;type:varchar(24);not null"`
	MimeType    string    `json:"mime_type"    gorm:"column:mime_type;type:varchar(64);not null"`
	FileName    string    `json:"file_name"    gorm:"column:file_name;type:varchar(255)"`
	SizeBytes   int       `json:"size_bytes"   gorm:"column:size_bytes;type:integer;not null"`
	DisplayMode string    `json:"display_mode" gorm:"column:display_mode;type:varchar(24);not null;default:'icon'"`
	Stored      bool      `json:"stored"       gorm:"column:stored;type:boolean;not null;default:false"`
	CreatedAt   time.Time `json:"created_at"   gorm:"column:created_at"`
}

func (AIChatMessageAttachment) TableName() string { return "ai_chat_message_attachments" }

func (a *AIChatMessageAttachment) BeforeCreate(tx *gorm.DB) error {
	setAIChatIDAndTimestamps(&a.ID, &a.CreatedAt, nil)
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
	a.Stored = false
	return nil
}
func (a *AIChatMessageAttachment) AfterUpdate(tx *gorm.DB) error { return nil }

func setAIChatIDAndTimestamps(id *string, createdAt *time.Time, updatedAt *time.Time) {
	now := time.Now()
	if id != nil && *id == "" {
		*id = uuid.New().String()
	}
	if createdAt != nil && createdAt.IsZero() {
		*createdAt = now
	}
	if updatedAt != nil && updatedAt.IsZero() {
		*updatedAt = now
	}
}

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
