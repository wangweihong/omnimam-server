package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	IdentityUserActive   = "ACTIVE"
	IdentityUserPending  = "PENDING"
	IdentityUserRejected = "REJECTED"
	IdentityUserDisabled = "DISABLED"
	IdentityUserLocked   = "LOCKED"
	IdentityUserDeleted  = "DELETED"
	IdentityGrantUser    = "USER"
	IdentityGrantGroup   = "GROUP"
	IdentityAccessView   = "VIEW"
	IdentityAccessUse    = "USE"
	IdentityAccessEdit   = "EDIT"
	IdentityAccessManage = "MANAGE"
)

const (
	IdentityRegistrationPending  = "PENDING"
	IdentityRegistrationApproved = "APPROVED"
	IdentityRegistrationRejected = "REJECTED"

	IdentityDeletionCheckComplete   = "COMPLETE"
	IdentityDeletionCheckIncomplete = "INCOMPLETE"
	IdentityDeletionCheckStale      = "STALE"
	IdentityDeletionCheckConsumed   = "CONSUMED"
)

// IdentityUser 是 v1.11 Identity 的本地用户事实；password_hash 只保存 Argon2id PHC 字符串。
type IdentityUser struct {
	imachinery.ObjectMeta
	Username             string           `json:"username" gorm:"column:username;type:text;not null"`
	NormalizedUsername   string           `json:"-" gorm:"column:normalized_username;type:text;not null;uniqueIndex"`
	DisplayName          string           `json:"display_name" gorm:"column:display_name;type:text;not null;default:''"`
	Alias                string           `json:"alias,omitempty" gorm:"column:alias;type:text;not null;default:''"`
	Email                *string          `json:"email,omitempty" gorm:"column:email;type:text"`
	NormalizedEmail      *string          `json:"-" gorm:"column:normalized_email;type:text;uniqueIndex"`
	Phone                string           `json:"phone,omitempty" gorm:"column:phone;type:text;not null;default:''"`
	PasswordHash         string           `json:"-" gorm:"column:password_hash;type:text;not null"`
	Status               string           `json:"status" gorm:"column:status;type:text;not null;index"`
	FirstLoginRequired   bool             `json:"first_login_required" gorm:"column:first_login_required;not null;default:false"`
	FailedLoginCount     int              `json:"-" gorm:"column:failed_login_count;not null;default:0"`
	LockedUntil          *imachinery.Time `json:"-" gorm:"column:locked_until"`
	SecurityVersion      int64            `json:"security_version" gorm:"column:security_version;not null;default:0"`
	AuthorizationVersion int64            `json:"authorization_version" gorm:"column:authorization_version;not null;default:0"`
	PasswordChangedAt    *imachinery.Time `json:"-" gorm:"column:password_changed_at"`
	LastLoginAt          *imachinery.Time `json:"last_login_at,omitempty" gorm:"column:last_login_at"`
	CreatedBy            string           `json:"-" gorm:"column:created_by;type:text"`
	DeletedAt            *imachinery.Time `json:"-" gorm:"column:deleted_at"`
}

func (IdentityUser) TableName() string                 { return "identity_users" }
func (m *IdentityUser) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (m *IdentityUser) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *IdentityUser) AfterCreate(*gorm.DB) error     { return nil }
func (m *IdentityUser) AfterUpdate(*gorm.DB) error     { return nil }
func (m *IdentityUser) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }

// IdentityRegistrationApplication records one immutable registration decision attempt.
type IdentityRegistrationApplication struct {
	imachinery.ObjectMeta
	UserID         string           `json:"user_id" gorm:"column:user_id;type:text;not null;uniqueIndex:idx_identity_registration_attempt,priority:1;index"`
	AttemptNo      int              `json:"attempt_no" gorm:"column:attempt_no;not null;uniqueIndex:idx_identity_registration_attempt,priority:2"`
	Status         string           `json:"status" gorm:"column:status;type:text;not null;index"`
	SubmittedAt    imachinery.Time  `json:"submitted_at" gorm:"column:submitted_at;not null"`
	DecidedAt      *imachinery.Time `json:"decided_at,omitempty" gorm:"column:decided_at"`
	DecidedBy      *string          `json:"decided_by,omitempty" gorm:"column:decided_by;type:text"`
	DecisionReason *string          `json:"decision_reason,omitempty" gorm:"column:decision_reason;type:text"`
}

func (IdentityRegistrationApplication) TableName() string {
	return "identity_registration_applications"
}
func (m *IdentityRegistrationApplication) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityRegistrationApplication) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityRegistrationApplication) AfterCreate(*gorm.DB) error { return nil }
func (m *IdentityRegistrationApplication) AfterUpdate(*gorm.DB) error { return nil }
func (m *IdentityRegistrationApplication) AfterFind(tx *gorm.DB) error {
	return m.ObjectMeta.AfterFind(tx)
}

// IdentityRole 是平台 RBAC 角色；builtin 角色只能由系统初始化或受控变更维护。
type IdentityRole struct {
	imachinery.ObjectMeta
	Code                  string   `json:"code" gorm:"column:code;type:text;not null;uniqueIndex"`
	Builtin               bool     `json:"builtin" gorm:"column:builtin;not null;default:false"`
	Status                string   `json:"status" gorm:"column:status;type:text;not null;default:ACTIVE"`
	PermissionCodes       []string `json:"permission_codes" gorm:"-"`
	PermissionCodesShadow string   `json:"-" gorm:"column:permission_codes_json;type:text;not null;default:'[]'"`
}

func (IdentityRole) TableName() string                 { return "identity_roles" }
func (m *IdentityRole) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (m *IdentityRole) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *IdentityRole) AfterCreate(*gorm.DB) error     { return nil }
func (m *IdentityRole) AfterUpdate(*gorm.DB) error     { return nil }
func (m *IdentityRole) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if m.PermissionCodesShadow == "" {
		return nil
	}
	return json.Unmarshal([]byte(m.PermissionCodesShadow), &m.PermissionCodes)
}

// IdentityPermissionDefinition 是 middleware 和 RBAC 使用的稳定权限目录项。
type IdentityPermissionDefinition struct {
	imachinery.ObjectMeta
	Code      string `json:"code" gorm:"column:code;type:text;not null;uniqueIndex"`
	Domain    string `json:"domain" gorm:"column:domain;type:text;not null"`
	Resource  string `json:"resource" gorm:"column:resource;type:text;not null"`
	Action    string `json:"action" gorm:"column:action;type:text;not null"`
	RiskLevel string `json:"risk_level" gorm:"column:risk_level;type:text;not null"`
	Status    string `json:"status" gorm:"column:status;type:text;not null;default:ACTIVE"`
}

func (IdentityPermissionDefinition) TableName() string { return "identity_permission_definitions" }
func (m *IdentityPermissionDefinition) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityPermissionDefinition) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityPermissionDefinition) AfterCreate(*gorm.DB) error { return nil }
func (m *IdentityPermissionDefinition) AfterUpdate(*gorm.DB) error { return nil }
func (m *IdentityPermissionDefinition) AfterFind(tx *gorm.DB) error {
	return m.ObjectMeta.AfterFind(tx)
}

// IdentityGroup 是静态用户组；组成员和组角色通过关联表维护。
type IdentityGroup struct {
	imachinery.ObjectMeta
	Code                string   `json:"code" gorm:"column:code;type:text;not null;uniqueIndex"`
	Status              string   `json:"status" gorm:"column:status;type:text;not null;default:ACTIVE"`
	MemberUserIDs       []string `json:"member_user_ids" gorm:"-"`
	RoleIDs             []string `json:"role_ids" gorm:"-"`
	MemberUserIDsShadow string   `json:"-" gorm:"column:member_user_ids_json;type:text;not null;default:'[]'"`
	RoleIDsShadow       string   `json:"-" gorm:"column:role_ids_json;type:text;not null;default:'[]'"`
}

func (IdentityGroup) TableName() string                 { return "identity_groups" }
func (m *IdentityGroup) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (m *IdentityGroup) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *IdentityGroup) AfterCreate(*gorm.DB) error     { return nil }
func (m *IdentityGroup) AfterUpdate(*gorm.DB) error     { return nil }
func (m *IdentityGroup) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if m.MemberUserIDsShadow != "" {
		if err := json.Unmarshal([]byte(m.MemberUserIDsShadow), &m.MemberUserIDs); err != nil {
			return err
		}
	}
	if m.RoleIDsShadow != "" {
		return json.Unmarshal([]byte(m.RoleIDsShadow), &m.RoleIDs)
	}
	return nil
}

// IdentityResourceAccessGrant 是资源域主动接入后的共享授权，不代表资源存在性。
type IdentityResourceAccessGrant struct {
	imachinery.ObjectMeta
	ResourceType           string           `json:"resource_type" gorm:"column:resource_type;type:text;not null;uniqueIndex:idx_identity_grant_subject,priority:1"`
	ResourceID             string           `json:"resource_id" gorm:"column:resource_id;type:text;not null;uniqueIndex:idx_identity_grant_subject,priority:2"`
	SubjectType            string           `json:"subject_type" gorm:"column:subject_type;type:text;not null;uniqueIndex:idx_identity_grant_subject,priority:3"`
	SubjectID              string           `json:"subject_id" gorm:"column:subject_id;type:text;not null;uniqueIndex:idx_identity_grant_subject,priority:4"`
	AccessLevel            string           `json:"access_level" gorm:"column:access_level;type:text;not null"`
	GrantedByPrincipalType string           `json:"granted_by_principal_type" gorm:"column:granted_by_principal_type;type:text;not null"`
	GrantedByPrincipalID   string           `json:"granted_by_principal_id" gorm:"column:granted_by_principal_id;type:text;not null"`
	ExpiresAt              *imachinery.Time `json:"expires_at,omitempty" gorm:"column:expires_at"`
	RevokedAt              *imachinery.Time `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
}

func (IdentityResourceAccessGrant) TableName() string { return "identity_resource_access_grants" }
func (m *IdentityResourceAccessGrant) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityResourceAccessGrant) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityResourceAccessGrant) AfterCreate(*gorm.DB) error  { return nil }
func (m *IdentityResourceAccessGrant) AfterUpdate(*gorm.DB) error  { return nil }
func (m *IdentityResourceAccessGrant) AfterFind(tx *gorm.DB) error { return m.ObjectMeta.AfterFind(tx) }

// IdentityAuthSession 记录登录设备、会话期限和派生在线状态的活动时间。
type IdentityAuthSession struct {
	imachinery.ObjectMeta
	UserID       string           `json:"user_id" gorm:"column:user_id;type:text;not null;index"`
	ClientID     string           `json:"client_id" gorm:"column:client_id;type:text;not null"`
	DeviceInfo   string           `json:"device_info,omitempty" gorm:"column:device_info;type:text;not null;default:''"`
	IPAddress    string           `json:"ip_address,omitempty" gorm:"column:ip_address;type:text"`
	UserAgent    string           `json:"user_agent,omitempty" gorm:"column:user_agent;type:text"`
	Status       string           `json:"status" gorm:"column:status;type:text;not null;index"`
	LastActiveAt *imachinery.Time `json:"last_active_at,omitempty" gorm:"column:last_active_at"`
	ExpiresAt    imachinery.Time  `json:"expires_at" gorm:"column:expires_at;not null"`
	RevokedAt    *imachinery.Time `json:"-" gorm:"column:revoked_at"`
	RevokeReason string           `json:"-" gorm:"column:revoke_reason;type:text"`
}

func (IdentityAuthSession) TableName() string                 { return "identity_auth_sessions" }
func (m *IdentityAuthSession) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (m *IdentityAuthSession) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *IdentityAuthSession) AfterCreate(*gorm.DB) error     { return nil }
func (m *IdentityAuthSession) AfterUpdate(*gorm.DB) error     { return nil }
func (m *IdentityAuthSession) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }

// IdentityTokenCredential 保存 Access Token 的 JTI 和当前凭据状态，不保存 Token 原文。
type IdentityTokenCredential struct {
	imachinery.ObjectMeta
	PrincipalType     string           `json:"principal_type" gorm:"column:principal_type;type:text;not null"`
	PrincipalID       string           `json:"principal_id" gorm:"column:principal_id;type:text;not null;index"`
	AuthSessionID     string           `json:"auth_session_id" gorm:"column:auth_session_id;type:text;index"`
	AccessTokenJTI    string           `json:"-" gorm:"column:access_token_jti;type:text;not null;uniqueIndex"`
	SecurityVersion   int64            `json:"-" gorm:"column:security_version;not null;default:0"`
	CredentialVersion int64            `json:"credential_version" gorm:"column:credential_version;not null;default:0"`
	Status            string           `json:"status" gorm:"column:status;type:text;not null;index"`
	IssuedAt          imachinery.Time  `json:"-" gorm:"column:issued_at;not null"`
	ExpiresAt         imachinery.Time  `json:"-" gorm:"column:expires_at;not null"`
	RevokedAt         *imachinery.Time `json:"-" gorm:"column:revoked_at"`
}

func (IdentityTokenCredential) TableName() string { return "identity_token_credentials" }
func (m *IdentityTokenCredential) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityTokenCredential) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityTokenCredential) AfterCreate(*gorm.DB) error  { return nil }
func (m *IdentityTokenCredential) AfterUpdate(*gorm.DB) error  { return nil }
func (m *IdentityTokenCredential) AfterFind(tx *gorm.DB) error { return m.ObjectMeta.AfterFind(tx) }

// IdentityRefreshToken 只保存 Refresh Token hash，USED/REVOKED 状态用于重用检测。
type IdentityRefreshToken struct {
	imachinery.ObjectMeta
	SessionID string           `json:"session_id" gorm:"column:session_id;type:text;not null;index"`
	TokenHash string           `json:"-" gorm:"column:token_hash;type:text;not null;uniqueIndex"`
	ParentID  string           `json:"-" gorm:"column:parent_id;type:text"`
	Status    string           `json:"-" gorm:"column:status;type:text;not null;index"`
	IssuedAt  imachinery.Time  `json:"-" gorm:"column:issued_at;not null"`
	ExpiresAt imachinery.Time  `json:"-" gorm:"column:expires_at;not null"`
	UsedAt    *imachinery.Time `json:"-" gorm:"column:used_at"`
	RevokedAt *imachinery.Time `json:"-" gorm:"column:revoked_at"`
}

func (IdentityRefreshToken) TableName() string                 { return "identity_refresh_tokens" }
func (m *IdentityRefreshToken) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (m *IdentityRefreshToken) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *IdentityRefreshToken) AfterCreate(*gorm.DB) error     { return nil }
func (m *IdentityRefreshToken) AfterUpdate(*gorm.DB) error     { return nil }
func (m *IdentityRefreshToken) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }

// IdentityServiceAccount 是 Worker/Runtime 等受控服务主体。
type IdentityServiceAccount struct {
	imachinery.ObjectMeta
	Code                 string           `json:"code" gorm:"column:code;type:text;not null;uniqueIndex"`
	OwnerType            string           `json:"owner_type" gorm:"column:owner_type;type:text;not null"`
	OwnerID              string           `json:"owner_id,omitempty" gorm:"column:owner_id;type:text"`
	Status               string           `json:"status" gorm:"column:status;type:text;not null;index"`
	SecurityVersion      int64            `json:"security_version" gorm:"column:security_version;not null;default:0"`
	AuthorizationVersion int64            `json:"authorization_version" gorm:"column:authorization_version;not null;default:0"`
	CreatedBy            string           `json:"-" gorm:"column:created_by;type:text"`
	DisabledAt           *imachinery.Time `json:"-" gorm:"column:disabled_at"`
}

func (IdentityServiceAccount) TableName() string { return "identity_service_accounts" }
func (m *IdentityServiceAccount) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityServiceAccount) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityServiceAccount) AfterCreate(*gorm.DB) error  { return nil }
func (m *IdentityServiceAccount) AfterUpdate(*gorm.DB) error  { return nil }
func (m *IdentityServiceAccount) AfterFind(tx *gorm.DB) error { return m.ObjectMeta.AfterFind(tx) }

// IdentityServiceAccountCredential 只在创建或轮换响应中返回一次明文凭据。
type IdentityServiceAccountCredential struct {
	imachinery.ObjectMeta
	ServiceAccountID string           `json:"service_account_id" gorm:"column:service_account_id;type:text;not null;index"`
	CredentialHash   string           `json:"-" gorm:"column:credential_hash;type:text;not null;uniqueIndex"`
	Prefix           string           `json:"prefix" gorm:"column:prefix;type:text;not null"`
	Status           string           `json:"status" gorm:"column:status;type:text;not null;index"`
	IssuedAt         imachinery.Time  `json:"issued_at" gorm:"column:issued_at;not null"`
	ExpiresAt        *imachinery.Time `json:"expires_at,omitempty" gorm:"column:expires_at"`
	RevokedAt        *imachinery.Time `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
	LastUsedAt       *imachinery.Time `json:"last_used_at,omitempty" gorm:"column:last_used_at"`
	RotatedFromID    *string          `json:"rotated_from_id,omitempty" gorm:"column:rotated_from_id;type:text"`
	RevokedReason    *string          `json:"revoked_reason,omitempty" gorm:"column:revoked_reason;type:text"`
	CreatedBy        string           `json:"-" gorm:"column:created_by;type:text"`
}

func (IdentityServiceAccountCredential) TableName() string {
	return "identity_service_account_credentials"
}
func (m *IdentityServiceAccountCredential) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityServiceAccountCredential) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityServiceAccountCredential) AfterCreate(*gorm.DB) error { return nil }
func (m *IdentityServiceAccountCredential) AfterUpdate(*gorm.DB) error { return nil }
func (m *IdentityServiceAccountCredential) AfterFind(tx *gorm.DB) error {
	return m.ObjectMeta.AfterFind(tx)
}

type IdentityRolePermissionGrant struct {
	RoleID         string          `gorm:"column:role_id;type:text;primaryKey"`
	PermissionCode string          `gorm:"column:permission_code;type:text;primaryKey"`
	CreatedBy      string          `gorm:"column:created_by;type:text"`
	CreatedAt      imachinery.Time `gorm:"column:created_at;not null"`
}

func (IdentityRolePermissionGrant) TableName() string { return "identity_role_permission_grants" }

type IdentityUserRoleGrant struct {
	ID            string           `gorm:"column:id;type:text;primaryKey"`
	UserID        string           `gorm:"column:user_id;type:text;index;not null"`
	RoleID        string           `gorm:"column:role_id;type:text;index;not null"`
	EffectiveFrom *imachinery.Time `gorm:"column:effective_from"`
	EffectiveTo   *imachinery.Time `gorm:"column:effective_to"`
	CreatedBy     string           `gorm:"column:created_by;type:text"`
	CreatedAt     imachinery.Time  `gorm:"column:created_at;not null"`
}

func (IdentityUserRoleGrant) TableName() string { return "identity_user_role_grants" }

type IdentityServiceAccountRoleGrant struct {
	ID               string           `gorm:"column:id;type:text;primaryKey"`
	ServiceAccountID string           `gorm:"column:service_account_id;type:text;not null;uniqueIndex:idx_identity_service_account_role,priority:1"`
	RoleID           string           `gorm:"column:role_id;type:text;not null;uniqueIndex:idx_identity_service_account_role,priority:2"`
	EffectiveFrom    *imachinery.Time `gorm:"column:effective_from"`
	EffectiveTo      *imachinery.Time `gorm:"column:effective_to"`
	CreatedBy        string           `gorm:"column:created_by;type:text"`
	CreatedAt        imachinery.Time  `gorm:"column:created_at;not null"`
}

func (IdentityServiceAccountRoleGrant) TableName() string {
	return "identity_service_account_role_grants"
}

// IdentityUserDeletionCheck is a short-lived snapshot of cross-domain dependencies.
type IdentityUserDeletionCheck struct {
	imachinery.ObjectMeta
	UserID           string          `json:"user_id" gorm:"column:user_id;type:text;not null;index"`
	Status           string          `json:"status" gorm:"column:status;type:text;not null"`
	RequestedBy      string          `json:"-" gorm:"column:requested_by;type:text;not null"`
	CheckedAt        imachinery.Time `json:"checked_at" gorm:"column:checked_at;not null"`
	ExpiresAt        imachinery.Time `json:"expires_at" gorm:"column:expires_at;not null"`
	SourceSetVersion string          `json:"-" gorm:"column:source_set_version;type:text;not null"`
}

func (IdentityUserDeletionCheck) TableName() string { return "identity_user_deletion_checks" }
func (m *IdentityUserDeletionCheck) BeforeCreate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeCreate(tx)
}
func (m *IdentityUserDeletionCheck) BeforeUpdate(tx *gorm.DB) error {
	return m.ObjectMeta.BeforeUpdate(tx)
}
func (m *IdentityUserDeletionCheck) AfterCreate(*gorm.DB) error { return nil }
func (m *IdentityUserDeletionCheck) AfterUpdate(*gorm.DB) error { return nil }
func (m *IdentityUserDeletionCheck) AfterFind(tx *gorm.DB) error {
	return m.ObjectMeta.AfterFind(tx)
}

type IdentityUserDeletionCheckItem struct {
	ID              string          `json:"id,omitempty" gorm:"column:id;type:text;primaryKey"`
	CheckID         string          `json:"-" gorm:"column:check_id;type:text;not null;index"`
	SourceDomain    string          `json:"source_domain" gorm:"column:source_domain;type:text;not null"`
	Category        string          `json:"category" gorm:"column:category;type:text;not null"`
	ObjectType      string          `json:"object_type" gorm:"column:object_type;type:text;not null"`
	ItemCount       int             `json:"count" gorm:"column:item_count;not null;default:0"`
	Blocking        bool            `json:"blocking" gorm:"column:blocking;not null"`
	SourceStatus    string          `json:"source_status" gorm:"column:source_status;type:text;not null"`
	HandlingMode    string          `json:"handling_mode" gorm:"column:handling_mode;type:text;not null"`
	ManagementEntry *string         `json:"management_entry,omitempty" gorm:"column:management_entry;type:text"`
	SourceVersion   *string         `json:"source_version,omitempty" gorm:"column:source_version;type:text"`
	CreatedAt       imachinery.Time `json:"-" gorm:"column:created_at;not null"`
}

func (IdentityUserDeletionCheckItem) TableName() string {
	return "identity_user_deletion_check_items"
}

type IdentityGroupMember struct {
	GroupID   string          `gorm:"column:group_id;type:text;primaryKey"`
	UserID    string          `gorm:"column:user_id;type:text;primaryKey;index"`
	CreatedBy string          `gorm:"column:created_by;type:text"`
	CreatedAt imachinery.Time `gorm:"column:created_at;not null"`
}

func (IdentityGroupMember) TableName() string { return "identity_group_members" }

type IdentityGroupRoleGrant struct {
	GroupID   string          `gorm:"column:group_id;type:text;primaryKey"`
	RoleID    string          `gorm:"column:role_id;type:text;primaryKey"`
	CreatedBy string          `gorm:"column:created_by;type:text"`
	CreatedAt imachinery.Time `gorm:"column:created_at;not null"`
}

func (IdentityGroupRoleGrant) TableName() string { return "identity_group_role_grants" }

// IdentityOutboxEvent stores reliable Identity changes with an idempotency key for asynchronous consumers.
type IdentityOutboxEvent struct {
	imachinery.ObjectMeta
	EventName        string           `gorm:"column:event_name;type:text;not null"`
	AggregateType    string           `gorm:"column:aggregate_type;type:text;not null"`
	AggregateID      string           `gorm:"column:aggregate_id;type:text;not null"`
	AggregateVersion int64            `gorm:"column:aggregate_version;not null"`
	PayloadJSON      string           `gorm:"column:payload_json;type:text;not null"`
	IdempotencyKey   string           `gorm:"column:idempotency_key;type:text;not null;uniqueIndex"`
	Status           string           `gorm:"column:status;type:text;not null;index"`
	AttemptCount     int              `gorm:"column:attempt_count;not null;default:0"`
	NextAttemptAt    *imachinery.Time `gorm:"column:next_attempt_at"`
	PublishedAt      *imachinery.Time `gorm:"column:published_at"`
	LastErrorCode    string           `gorm:"column:last_error_code;type:text"`
}

func (IdentityOutboxEvent) TableName() string                 { return "identity_outbox_events" }
func (m *IdentityOutboxEvent) BeforeCreate(tx *gorm.DB) error { return m.ObjectMeta.BeforeCreate(tx) }
func (m *IdentityOutboxEvent) BeforeUpdate(tx *gorm.DB) error { return m.ObjectMeta.BeforeUpdate(tx) }
func (m *IdentityOutboxEvent) AfterCreate(*gorm.DB) error     { return nil }
func (m *IdentityOutboxEvent) AfterUpdate(*gorm.DB) error     { return nil }
func (m *IdentityOutboxEvent) AfterFind(tx *gorm.DB) error    { return m.ObjectMeta.AfterFind(tx) }
