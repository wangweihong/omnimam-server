package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// PlatformSystemAuthConfig 是平台认证策略事实；Identity 只能通过受控内部读取接口消费。
type PlatformSystemAuthConfig struct {
	imachinery.ObjectMeta
	RegistrationMode            string                     `json:"registration_mode" gorm:"column:registration_mode;type:text;not null"`
	PasswordPolicy              PlatformPasswordPolicy     `json:"password_policy" gorm:"-"`
	PasswordPolicyShadow        string                     `json:"-" gorm:"column:password_policy_json;type:text;not null;default:'{}'"`
	LoginFailurePolicy          PlatformLoginFailurePolicy `json:"login_failure_policy" gorm:"-"`
	LoginFailurePolicyShadow    string                     `json:"-" gorm:"column:login_failure_policy_json;type:text;not null;default:'{}'"`
	OnlinePresenceWindowSeconds int                        `json:"online_presence_window_seconds" gorm:"column:online_presence_window_seconds;not null;default:300"`
	AccessTokenLifetimeSeconds  int                        `json:"access_token_lifetime" gorm:"column:access_token_lifetime_seconds;not null"`
	RefreshTokenLifetimeSeconds int                        `json:"refresh_token_lifetime" gorm:"column:refresh_token_lifetime_seconds;not null"`
	UpdatedByPrincipalType      string                     `json:"-" gorm:"column:updated_by_principal_type;type:text;not null"`
	UpdatedByPrincipalID        string                     `json:"-" gorm:"column:updated_by_principal_id;type:text;not null"`
	ActorUserID                 string                     `json:"-" gorm:"-"`
}

func (PlatformSystemAuthConfig) TableName() string { return "platform_system_auth_configs" }
func (m *PlatformSystemAuthConfig) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	m.ID = "default"
	m.Name = "default"
	m.ResourceVersion = 0
	return m.syncPolicyShadows()
}
func (m *PlatformSystemAuthConfig) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return m.syncPolicyShadows()
}
func (m *PlatformSystemAuthConfig) AfterCreate(*gorm.DB) error { return nil }
func (m *PlatformSystemAuthConfig) AfterUpdate(*gorm.DB) error { return nil }
func (m *PlatformSystemAuthConfig) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(m.PasswordPolicyShadow), &m.PasswordPolicy); err != nil {
		return err
	}
	return json.Unmarshal([]byte(m.LoginFailurePolicyShadow), &m.LoginFailurePolicy)
}

func (m *PlatformSystemAuthConfig) syncPolicyShadows() error {
	passwordPolicy, err := json.Marshal(m.PasswordPolicy)
	if err != nil {
		return err
	}
	loginFailurePolicy, err := json.Marshal(m.LoginFailurePolicy)
	if err != nil {
		return err
	}
	m.PasswordPolicyShadow = string(passwordPolicy)
	m.LoginFailurePolicyShadow = string(loginFailurePolicy)
	return nil
}

// PlatformAuditLog 是跨 domain 的追加式脱敏审计记录，detail 仅保存有界 JSON。
type PlatformAuditLog struct {
	imachinery.ObjectMeta
	SourceDomain       string          `json:"source_domain" gorm:"column:source_domain;type:text;not null;index;uniqueIndex:idx_platform_audit_idempotency,priority:1"`
	SourceModule       string          `json:"source_module" gorm:"column:source_module;type:text;not null;uniqueIndex:idx_platform_audit_idempotency,priority:2"`
	PrincipalType      string          `json:"principal_type" gorm:"column:principal_type;type:text;not null"`
	PrincipalID        string          `json:"principal_id,omitempty" gorm:"column:principal_id;type:text"`
	ActorUserID        string          `json:"actor_user_id,omitempty" gorm:"column:actor_user_id;type:text"`
	Action             string          `json:"action" gorm:"column:action;type:text;not null"`
	TargetType         string          `json:"target_type,omitempty" gorm:"column:target_type;type:text"`
	TargetID           string          `json:"target_id,omitempty" gorm:"column:target_id;type:text"`
	OwnerUserID        string          `json:"owner_user_id,omitempty" gorm:"column:owner_user_id;type:text"`
	Result             string          `json:"result" gorm:"column:result;type:text;not null;index"`
	ReasonCode         string          `json:"reason_code,omitempty" gorm:"column:reason_code;type:text"`
	RequestID          string          `json:"request_id,omitempty" gorm:"column:request_id;type:text"`
	TraceID            string          `json:"trace_id,omitempty" gorm:"column:trace_id;type:text"`
	IPAddress          string          `json:"ip_address,omitempty" gorm:"column:ip_address;type:text"`
	UserAgent          string          `json:"user_agent,omitempty" gorm:"column:user_agent;type:text"`
	OccurredAt         imachinery.Time `json:"occurred_at" gorm:"column:occurred_at;not null;index"`
	Detail             json.RawMessage `json:"detail,omitempty" gorm:"-"`
	DetailShadow       string          `json:"-" gorm:"column:detail_json;type:text;not null;default:'{}'"`
	IdempotencyKey     string          `json:"idempotency_key" gorm:"column:idempotency_key;type:text;not null;uniqueIndex:idx_platform_audit_idempotency,priority:3"`
	ContentFingerprint string          `json:"-" gorm:"column:content_fingerprint;type:text;not null"`
}

func (PlatformAuditLog) TableName() string { return "platform_audit_logs" }
func (m *PlatformAuditLog) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	m.ResourceVersion = 0
	m.UpdatedAt = m.CreatedAt
	m.ExtendShadow = ""
	if len(m.Detail) == 0 {
		m.Detail = json.RawMessage(`{}`)
	}
	m.DetailShadow = string(m.Detail)
	return nil
}
func (m *PlatformAuditLog) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *PlatformAuditLog) AfterCreate(*gorm.DB) error     { return nil }
func (m *PlatformAuditLog) AfterUpdate(*gorm.DB) error     { return nil }
func (m *PlatformAuditLog) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if m.DetailShadow == "" {
		m.Detail = json.RawMessage(`{}`)
		return nil
	}
	m.Detail = json.RawMessage(m.DetailShadow)
	return nil
}

// PlatformOutboxEvent 是平台配置和审计事实的可靠事件载体。
type PlatformOutboxEvent struct {
	imachinery.ObjectMeta
	EventName        string           `gorm:"column:event_name;type:text;not null"`
	AggregateType    string           `gorm:"column:aggregate_type;type:text;not null"`
	AggregateID      string           `gorm:"column:aggregate_id;type:text;not null"`
	AggregateVersion int64            `gorm:"column:aggregate_version;not null"`
	PayloadJSON      string           `gorm:"column:payload_json;type:text;not null"`
	IdempotencyKey   string           `gorm:"column:idempotency_key;type:text;not null;uniqueIndex"`
	PublishedAt      *imachinery.Time `gorm:"column:published_at"`
}

func (PlatformOutboxEvent) TableName() string { return "platform_outbox_events" }
func (m *PlatformOutboxEvent) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	m.ResourceVersion = 0
	return nil
}
func (m *PlatformOutboxEvent) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *PlatformOutboxEvent) AfterCreate(*gorm.DB) error     { return nil }
func (m *PlatformOutboxEvent) AfterUpdate(*gorm.DB) error     { return nil }
func (m *PlatformOutboxEvent) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }
