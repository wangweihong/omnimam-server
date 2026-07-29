package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	NotificationCategorySystem      = "system"
	NotificationCategoryTask        = "task"
	NotificationCategoryAsset       = "asset"
	NotificationCategoryApplication = "application"
	NotificationCategoryProvider    = "provider"
	NotificationCategoryStorage     = "storage"
	NotificationCategorySecurity    = "security"
	NotificationCategoryAgent       = "agent"
	NotificationCategoryCanvas      = "canvas"

	NotificationSeverityInfo     = "info"
	NotificationSeveritySuccess  = "success"
	NotificationSeverityWarning  = "warning"
	NotificationSeverityError    = "error"
	NotificationSeverityCritical = "critical"

	NotificationInboxUnread   = "unread"
	NotificationInboxRead     = "read"
	NotificationInboxArchived = "archived"

	NotificationAttentionInformational  = "informational"
	NotificationAttentionActionRequired = "action_required"
	NotificationAttentionResolved       = "resolved"

	NotificationTopicActive      = "ACTIVE"
	NotificationTopicContractGap = "CONTRACT_GAP"
	NotificationTopicFuture      = "FUTURE"

	NotificationEventPending    = "PENDING"
	NotificationEventProcessed  = "PROCESSED"
	NotificationEventIgnored    = "IGNORED"
	NotificationEventFailed     = "FAILED"
	NotificationEventDeadLetter = "DEAD_LETTER"

	NotificationOutboxPending   = "PENDING"
	NotificationOutboxPublished = "PUBLISHED"
	NotificationOutboxFailed    = "FAILED"

	NotificationEventCreated            = "notification_created"
	NotificationEventUpdated            = "notification_updated"
	NotificationEventDeleted            = "notification_deleted"
	NotificationEventUnreadCountChanged = "notification_unread_count_changed"

	NotificationInboxReadPermission        = "notification.inbox.read"
	NotificationInboxManagePermission      = "notification.inbox.manage"
	NotificationPreferenceReadPermission   = "notification.preference.read"
	NotificationPreferenceManagePermission = "notification.preference.manage"
	NotificationAdminReceivePermission     = "notification.admin.receive"
	NotificationIngestionPermission        = "notification.ingestion.internal"
)

// NotificationSourceMapping 描述 topic 可接受的源域事件；它只由服务端 seed 写入。
// +k8s:deepcopy-gen=true
type NotificationSourceMapping struct {
	SourceDomain    string `json:"source_domain"`
	SourceEventType string `json:"source_event_type"`
}

// NotificationRuleConfig 保存预设规则的受控参数，不接受动态表达式或代码。
// +k8s:deepcopy-gen=true
type NotificationRuleConfig struct {
	TerminalStatuses []string `json:"terminal_statuses,omitempty"`
	Recipient        string   `json:"recipient,omitempty"`
}

// NotificationNavigationConfig 保存预设导航解析器标识和视图。
// +k8s:deepcopy-gen=true
type NotificationNavigationConfig struct {
	Resolver string `json:"resolver,omitempty"`
	View     string `json:"view,omitempty"`
}

// NotificationRecipientBasis 是源事件给出的最小接收者依据，不进入公共 API。
// +k8s:deepcopy-gen=true
type NotificationRecipientBasis struct {
	OwnerUserID string `json:"owner_user_id,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

// NotificationPayloadSnapshot 是规则重放所需的白名单快照，不保存完整源 payload。
// +k8s:deepcopy-gen=true
type NotificationPayloadSnapshot struct {
	AtomicTaskID     string   `json:"atomic_task_id,omitempty"`
	CanvasRunID      string   `json:"canvas_run_id,omitempty"`
	CanvasID         string   `json:"canvas_id,omitempty"`
	OwnerType        string   `json:"owner_type,omitempty"`
	OwnerID          string   `json:"owner_id,omitempty"`
	ApplicationRunID string   `json:"application_run_id,omitempty"`
	FromStatus       string   `json:"from_status,omitempty"`
	Status           string   `json:"status,omitempty"`
	ErrorCode        string   `json:"error_code,omitempty"`
	ErrorSummary     string   `json:"error_summary,omitempty"`
	Retryable        bool     `json:"retryable,omitempty"`
	WarningCodes     []string `json:"warning_codes,omitempty"`
	Summary          string   `json:"summary,omitempty"`
}

// NotificationTopic 是预设 topic/rule catalog；公共 API 不开放增删改。
// +k8s:deepcopy-gen=true
type NotificationTopic struct {
	imachinery.ObjectMeta
	Topic                  string                       `json:"topic" gorm:"column:topic;type:text;not null;uniqueIndex"`
	Category               string                       `json:"category" gorm:"column:category;type:text;not null"`
	ActivationStatus       string                       `json:"activation_status" gorm:"column:activation_status;type:text;not null"`
	Enabled                bool                         `json:"enabled" gorm:"column:enabled;not null;default:false"`
	MandatoryInApp         bool                         `json:"mandatory_in_app" gorm:"column:mandatory_in_app;not null;default:false"`
	DefaultSeverity        string                       `json:"default_severity" gorm:"column:default_severity;type:text;not null"`
	DefaultAttentionStatus string                       `json:"default_attention_status" gorm:"column:default_attention_status;type:text;not null"`
	AggregationMode        string                       `json:"aggregation_mode" gorm:"column:aggregation_mode;type:text;not null"`
	RuleVersion            int                          `json:"rule_version" gorm:"column:rule_version;not null;default:1"`
	SourceMappings         []NotificationSourceMapping  `json:"source_mappings" gorm:"-"`
	SourceMappingsShadow   string                       `json:"-" gorm:"column:source_mapping_json;type:jsonb;not null;default:'[]'"`
	RuleConfig             NotificationRuleConfig       `json:"rule_config" gorm:"-"`
	RuleConfigShadow       string                       `json:"-" gorm:"column:rule_config_json;type:jsonb;not null;default:'{}'"`
	NavigationConfig       NotificationNavigationConfig `json:"navigation_config" gorm:"-"`
	NavigationConfigShadow string                       `json:"-" gorm:"column:navigation_config_json;type:jsonb;not null;default:'{}'"`
}

func (NotificationTopic) TableName() string { return "notification_topics" }
func (t *NotificationTopic) BeforeCreate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}
func (*NotificationTopic) AfterCreate(*gorm.DB) error { return nil }
func (t *NotificationTopic) BeforeUpdate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}
func (*NotificationTopic) AfterUpdate(*gorm.DB) error { return nil }
func (t *NotificationTopic) AfterFind(tx *gorm.DB) error {
	if err := t.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalNotificationJSON(t.SourceMappingsShadow, &t.SourceMappings, "[]", t.RuleConfigShadow, &t.RuleConfig, "{}", t.NavigationConfigShadow, &t.NavigationConfig, "{}")
}
func (t *NotificationTopic) marshalShadows() error {
	return marshalNotificationJSON(t.SourceMappings, &t.SourceMappingsShadow, "[]", t.RuleConfig, &t.RuleConfigShadow, "{}", t.NavigationConfig, &t.NavigationConfigShadow, "{}")
}

// NotificationEvent 是可靠源事件归一化后的候选，不是用户可见通知。
// +k8s:deepcopy-gen=true
type NotificationEvent struct {
	imachinery.ObjectMeta
	SourceDomain           string                      `json:"source_domain" gorm:"column:source_domain;type:text;not null"`
	SourceEventType        string                      `json:"source_event_type" gorm:"column:source_event_type;type:text;not null"`
	SourceEventID          string                      `json:"source_event_id" gorm:"column:source_event_id;type:text;not null"`
	SourceAggregateType    string                      `json:"source_aggregate_type" gorm:"column:source_aggregate_type;type:text;not null"`
	SourceAggregateID      string                      `json:"source_aggregate_id" gorm:"column:source_aggregate_id;type:text;not null"`
	SourceAggregateVersion int64                       `json:"source_aggregate_version" gorm:"column:source_aggregate_version;not null"`
	NotificationTopic      string                      `json:"notification_topic" gorm:"column:notification_topic;type:text;not null"`
	SourceType             string                      `json:"source_type" gorm:"column:source_type;type:text;not null"`
	SourceID               string                      `json:"source_id" gorm:"column:source_id;type:text;not null"`
	RecipientBasis         NotificationRecipientBasis  `json:"recipient_basis" gorm:"-"`
	RecipientBasisShadow   string                      `json:"-" gorm:"column:recipient_basis_json;type:jsonb;not null;default:'{}'"`
	ActorType              string                      `json:"actor_type" gorm:"column:actor_type;type:text;not null;default:''"`
	ActorID                string                      `json:"actor_id" gorm:"column:actor_id;type:text;not null;default:''"`
	OccurredAt             imachinery.Time             `json:"occurred_at" gorm:"column:occurred_at;type:timestamptz;not null"`
	PayloadSnapshot        NotificationPayloadSnapshot `json:"payload_snapshot" gorm:"-"`
	PayloadSnapshotShadow  string                      `json:"-" gorm:"column:payload_snapshot_json;type:jsonb;not null;default:'{}'"`
	ProcessingStatus       string                      `json:"processing_status" gorm:"column:processing_status;type:text;not null"`
	ProcessingAttemptCount int                         `json:"processing_attempt_count" gorm:"column:processing_attempt_count;not null;default:0"`
	NextAttemptAt          imachinery.Time             `json:"next_attempt_at,omitempty" gorm:"column:next_attempt_at"`
	ProcessedAt            imachinery.Time             `json:"processed_at,omitempty" gorm:"column:processed_at"`
	LastErrorCode          string                      `json:"last_error_code" gorm:"column:last_error_code;type:text;not null;default:''"`
	LastErrorSummary       string                      `json:"last_error_summary" gorm:"column:last_error_summary;type:text;not null;default:''"`
	DeduplicationKey       string                      `json:"deduplication_key" gorm:"column:deduplication_key;type:text;not null;uniqueIndex"`
	RuleVersion            int                         `json:"rule_version" gorm:"column:rule_version;not null"`
	ExpiresAt              imachinery.Time             `json:"expires_at" gorm:"column:expires_at;type:timestamptz;not null"`
}

func (NotificationEvent) TableName() string { return "notification_events" }
func (e *NotificationEvent) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return e.marshalShadows()
}
func (*NotificationEvent) AfterCreate(*gorm.DB) error { return nil }
func (e *NotificationEvent) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return e.marshalShadows()
}
func (*NotificationEvent) AfterUpdate(*gorm.DB) error { return nil }
func (e *NotificationEvent) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalNotificationJSON(e.RecipientBasisShadow, &e.RecipientBasis, "{}", e.PayloadSnapshotShadow, &e.PayloadSnapshot, "{}")
}
func (e *NotificationEvent) marshalShadows() error {
	return marshalNotificationJSON(e.RecipientBasis, &e.RecipientBasisShadow, "{}", e.PayloadSnapshot, &e.PayloadSnapshotShadow, "{}")
}

// NotificationNavigationTarget 是受控同源导航语义，不授予目标资源权限。
// +k8s:deepcopy-gen=true
type NotificationNavigationTarget struct {
	Type       string            `json:"type"`
	TargetType string            `json:"target_type"`
	TargetID   string            `json:"target_id"`
	View       string            `json:"view"`
	Params     map[string]string `json:"params"`
}

// Notification 是用户可见收件箱事实；Name/Description 分别持久化 title/content。
// +k8s:deepcopy-gen=true
type Notification struct {
	imachinery.ObjectMeta
	RecipientUserID        string                        `json:"-" gorm:"column:recipient_user_id;type:text;not null"`
	Category               string                        `json:"category" gorm:"column:category;type:text;not null"`
	NotificationTopic      string                        `json:"notification_topic" gorm:"column:notification_topic;type:text;not null"`
	Severity               string                        `json:"severity" gorm:"column:severity;type:text;not null"`
	InboxStatus            string                        `json:"inbox_status" gorm:"column:inbox_status;type:text;not null"`
	AttentionStatus        string                        `json:"attention_status" gorm:"column:attention_status;type:text;not null"`
	SourceType             string                        `json:"source_type" gorm:"column:source_type;type:text;not null"`
	SourceID               string                        `json:"source_id" gorm:"column:source_id;type:text;not null"`
	SourceAggregateVersion int64                         `json:"-" gorm:"column:source_aggregate_version;not null"`
	NavigationTarget       *NotificationNavigationTarget `json:"navigation_target,omitempty" gorm:"-"`
	NavigationTargetShadow string                        `json:"-" gorm:"column:navigation_target_json;type:jsonb"`
	ActionPath             *string                       `json:"action_path" gorm:"column:action_path;type:text"`
	DeduplicationKey       string                        `json:"-" gorm:"column:deduplication_key;type:text;not null"`
	AggregateKey           string                        `json:"-" gorm:"column:aggregate_key;type:text;not null;default:''"`
	AggregationWindow      string                        `json:"-" gorm:"column:aggregation_window;type:text;not null"`
	AggregationBucket      imachinery.Time               `json:"-" gorm:"column:aggregation_bucket"`
	OccurrenceCount        int                           `json:"occurrence_count" gorm:"column:occurrence_count;not null;default:1"`
	FirstOccurredAt        imachinery.Time               `json:"first_occurred_at" gorm:"column:first_occurred_at;type:timestamptz;not null"`
	LastOccurredAt         imachinery.Time               `json:"last_occurred_at" gorm:"column:last_occurred_at;type:timestamptz;not null"`
	ReadAt                 imachinery.Time               `json:"read_at,omitempty" gorm:"column:read_at"`
	ArchivedAt             imachinery.Time               `json:"archived_at,omitempty" gorm:"column:archived_at"`
	ResolvedAt             imachinery.Time               `json:"resolved_at,omitempty" gorm:"column:resolved_at"`
	ExpiresAt              imachinery.Time               `json:"expires_at,omitempty" gorm:"column:expires_at"`
	DeletedAt              imachinery.Time               `json:"-" gorm:"column:deleted_at"`
}

func (Notification) TableName() string { return "notifications" }
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if err := n.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return n.marshalShadows()
}
func (*Notification) AfterCreate(*gorm.DB) error { return nil }
func (n *Notification) BeforeUpdate(tx *gorm.DB) error {
	if err := n.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return n.marshalShadows()
}
func (*Notification) AfterUpdate(*gorm.DB) error { return nil }
func (n *Notification) AfterFind(tx *gorm.DB) error {
	if err := n.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if n.NavigationTargetShadow == "" {
		n.NavigationTarget = nil
		return nil
	}
	return json.Unmarshal([]byte(n.NavigationTargetShadow), &n.NavigationTarget)
}
func (n *Notification) marshalShadows() error {
	if n.NavigationTarget == nil {
		n.NavigationTargetShadow = ""
		return nil
	}
	raw, err := json.Marshal(n.NavigationTarget)
	if err != nil {
		return err
	}
	n.NavigationTargetShadow = string(raw)
	return nil
}

// NotificationEventLink 记录聚合通知的来源关系。
// +k8s:deepcopy-gen=true
type NotificationEventLink struct {
	imachinery.ObjectMeta
	NotificationID         string `json:"notification_id" gorm:"column:notification_id;type:text;not null"`
	NotificationEventID    string `json:"notification_event_id" gorm:"column:notification_event_id;type:text;not null"`
	SourceAggregateVersion int64  `json:"source_aggregate_version" gorm:"column:source_aggregate_version;not null"`
	OccurrenceDelta        int    `json:"occurrence_delta" gorm:"column:occurrence_delta;not null;default:1"`
}

func (NotificationEventLink) TableName() string                 { return "notification_event_links" }
func (m *NotificationEventLink) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (*NotificationEventLink) AfterCreate(*gorm.DB) error       { return nil }
func (m *NotificationEventLink) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (*NotificationEventLink) AfterUpdate(*gorm.DB) error       { return nil }

// NotificationRecipientCounter 是每用户的原子计数投影。
// +k8s:deepcopy-gen=true
type NotificationRecipientCounter struct {
	imachinery.ObjectMeta
	RecipientUserID     string `json:"-" gorm:"column:recipient_user_id;type:text;not null;uniqueIndex"`
	UnreadCount         int    `json:"unread_count" gorm:"column:unread_count;not null;default:0"`
	CriticalCount       int    `json:"critical_count" gorm:"column:critical_count;not null;default:0"`
	ActionRequiredCount int    `json:"action_required_count" gorm:"column:action_required_count;not null;default:0"`
}

func (NotificationRecipientCounter) TableName() string { return "notification_recipient_counters" }
func (m *NotificationRecipientCounter) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (*NotificationRecipientCounter) AfterCreate(*gorm.DB) error { return nil }
func (m *NotificationRecipientCounter) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (*NotificationRecipientCounter) AfterUpdate(*gorm.DB) error { return nil }

// NotificationPreference 保存用户显式覆盖；空 topic 表示分类级设置。
// +k8s:deepcopy-gen=true
type NotificationPreference struct {
	imachinery.ObjectMeta
	UserID            string          `json:"-" gorm:"column:user_id;type:text;not null"`
	Category          string          `json:"category" gorm:"column:category;type:text;not null"`
	NotificationTopic string          `json:"notification_topic,omitempty" gorm:"column:notification_topic;type:text;not null;default:''"`
	InAppEnabled      bool            `json:"in_app_enabled" gorm:"column:in_app_enabled;not null;default:true"`
	EmailEnabled      bool            `json:"-" gorm:"column:email_enabled;not null;default:false"`
	WebhookEnabled    bool            `json:"-" gorm:"column:webhook_enabled;not null;default:false"`
	MobilePushEnabled bool            `json:"-" gorm:"column:mobile_push_enabled;not null;default:false"`
	MinimumSeverity   string          `json:"minimum_severity" gorm:"column:minimum_severity;type:text;not null;default:'info'"`
	MergeRepeated     bool            `json:"merge_repeated" gorm:"column:merge_repeated;not null;default:true"`
	DigestMode        string          `json:"-" gorm:"column:digest_mode;type:text;not null;default:'none'"`
	MuteUntil         imachinery.Time `json:"-" gorm:"column:mute_until"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }
func (m *NotificationPreference) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (*NotificationPreference) AfterCreate(*gorm.DB) error { return nil }
func (m *NotificationPreference) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (*NotificationPreference) AfterUpdate(*gorm.DB) error { return nil }

// NotificationDelivery 是后续渠道预留记录；首期不得写外部渠道。
// +k8s:deepcopy-gen=true
type NotificationDelivery struct {
	imachinery.ObjectMeta
	NotificationID   string          `json:"notification_id" gorm:"column:notification_id;type:text;not null"`
	Channel          string          `json:"channel" gorm:"column:channel;type:text;not null"`
	DeliveryStatus   string          `json:"delivery_status" gorm:"column:delivery_status;type:text;not null"`
	Destination      string          `json:"destination" gorm:"column:destination;type:text;not null;default:''"`
	AttemptCount     int             `json:"attempt_count" gorm:"column:attempt_count;not null;default:0"`
	LastErrorCode    string          `json:"last_error_code" gorm:"column:last_error_code;type:text;not null;default:''"`
	LastErrorSummary string          `json:"last_error_summary" gorm:"column:last_error_summary;type:text;not null;default:''"`
	ScheduledAt      imachinery.Time `json:"scheduled_at,omitempty" gorm:"column:scheduled_at"`
	NextAttemptAt    imachinery.Time `json:"next_attempt_at,omitempty" gorm:"column:next_attempt_at"`
	SentAt           imachinery.Time `json:"sent_at,omitempty" gorm:"column:sent_at"`
}

func (NotificationDelivery) TableName() string                 { return "notification_deliveries" }
func (m *NotificationDelivery) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (*NotificationDelivery) AfterCreate(*gorm.DB) error       { return nil }
func (m *NotificationDelivery) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (*NotificationDelivery) AfterUpdate(*gorm.DB) error       { return nil }

// NotificationOutboxPayload 是 SSE projector 需要的最小通知变化摘要。
// +k8s:deepcopy-gen=true
type NotificationOutboxPayload struct {
	NotificationOutboxID string          `json:"notification_outbox_id"`
	NotificationID       string          `json:"notification_id,omitempty"`
	NotificationTopic    string          `json:"notification_topic,omitempty"`
	Category             string          `json:"category,omitempty"`
	Severity             string          `json:"severity,omitempty"`
	InboxStatus          string          `json:"inbox_status,omitempty"`
	AttentionStatus      string          `json:"attention_status,omitempty"`
	SourceType           string          `json:"source_type,omitempty"`
	SourceID             string          `json:"source_id,omitempty"`
	OccurrenceCount      int             `json:"occurrence_count,omitempty"`
	ChangedFields        []string        `json:"changed_fields,omitempty"`
	NavigationAvailable  bool            `json:"navigation_available,omitempty"`
	UnreadCount          int             `json:"unread_count,omitempty"`
	CriticalCount        int             `json:"critical_count,omitempty"`
	ActionRequiredCount  int             `json:"action_required_count,omitempty"`
	ResourceVersion      int64           `json:"resource_version,omitempty"`
	CounterVersion       int64           `json:"counter_version,omitempty"`
	DeletedAt            imachinery.Time `json:"deleted_at,omitempty"`
	OccurredAt           imachinery.Time `json:"occurred_at"`
}

// NotificationOutbox 将收件箱和计数变化可靠交给统一 SSE projector。
// +k8s:deepcopy-gen=true
type NotificationOutbox struct {
	imachinery.ObjectMeta
	EventName        string                    `json:"event_name" gorm:"column:event_name;type:text;not null"`
	AggregateType    string                    `json:"aggregate_type" gorm:"column:aggregate_type;type:text;not null"`
	AggregateID      string                    `json:"aggregate_id" gorm:"column:aggregate_id;type:text;not null"`
	AggregateVersion int64                     `json:"aggregate_version" gorm:"column:aggregate_version;not null"`
	RecipientUserID  string                    `json:"recipient_user_id" gorm:"column:recipient_user_id;type:text;not null"`
	Payload          NotificationOutboxPayload `json:"payload" gorm:"-"`
	PayloadShadow    string                    `json:"-" gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
	DeliveryStatus   string                    `json:"delivery_status" gorm:"column:delivery_status;type:text;not null"`
	AttemptCount     int                       `json:"attempt_count" gorm:"column:attempt_count;not null;default:0"`
	NextAttemptAt    imachinery.Time           `json:"next_attempt_at,omitempty" gorm:"column:next_attempt_at"`
	PublishedAt      imachinery.Time           `json:"published_at,omitempty" gorm:"column:published_at"`
}

func (NotificationOutbox) TableName() string { return "notification_outbox" }
func (m *NotificationOutbox) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalNotificationJSON(m.Payload, &m.PayloadShadow, "{}")
}
func (*NotificationOutbox) AfterCreate(*gorm.DB) error { return nil }
func (m *NotificationOutbox) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalNotificationJSON(m.Payload, &m.PayloadShadow, "{}")
}
func (*NotificationOutbox) AfterUpdate(*gorm.DB) error { return nil }
func (m *NotificationOutbox) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalNotificationJSON(m.PayloadShadow, &m.Payload, "{}")
}

func marshalNotificationJSON(values ...any) error {
	for index := 0; index < len(values); index += 3 {
		raw, err := json.Marshal(values[index])
		if err != nil {
			return err
		}
		if string(raw) == "null" {
			raw = []byte(values[index+2].(string))
		}
		*values[index+1].(*string) = string(raw)
	}
	return nil
}

func unmarshalNotificationJSON(values ...any) error {
	for index := 0; index < len(values); index += 3 {
		raw := values[index].(string)
		if raw == "" {
			raw = values[index+2].(string)
		}
		if err := json.Unmarshal([]byte(raw), values[index+1]); err != nil {
			return err
		}
	}
	return nil
}
