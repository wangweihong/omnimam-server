package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// PlatformAuthConfigReplaceRequest 替换认证策略；hash 算法和基线参数不允许客户端修改。
type PlatformAuthConfigReplaceRequest struct {
	RegistrationMode            string          `json:"registration_mode" binding:"required,oneof=OPEN ADMIN_APPROVAL"`
	PasswordPolicy              json.RawMessage `json:"password_policy" binding:"required"`
	LoginFailurePolicy          json.RawMessage `json:"login_failure_policy" binding:"required"`
	OnlinePresenceWindowSeconds int             `json:"online_presence_window_seconds" binding:"required,min=1,max=86400"`
	AccessTokenLifetime         int             `json:"access_token_lifetime" binding:"required,min=1,max=86400"`
	RefreshTokenLifetime        int             `json:"refresh_token_lifetime" binding:"required,min=1,max=2592000"`
	ResourceVersion             int64           `json:"resource_version" binding:"min=0"`
}

// PlatformAuditRecordRequest 是跨 domain 提交的最小审计上下文，detail 会在服务端再次脱敏和限长。
type PlatformAuditRecordRequest struct {
	SourceDomain   string          `json:"source_domain" binding:"required,max=128"`
	SourceModule   string          `json:"source_module" binding:"required,max=128"`
	PrincipalType  string          `json:"principal_type" binding:"required,oneof=USER SERVICE_ACCOUNT ANONYMOUS"`
	PrincipalID    string          `json:"principal_id" binding:"max=128"`
	ActorUserID    string          `json:"actor_user_id" binding:"max=128"`
	Action         string          `json:"action" binding:"required,max=256"`
	TargetType     string          `json:"target_type" binding:"max=128"`
	TargetID       string          `json:"target_id" binding:"max=128"`
	OwnerUserID    string          `json:"owner_user_id" binding:"max=128"`
	Result         string          `json:"result" binding:"required,oneof=SUCCESS FAILED DENIED"`
	ReasonCode     string          `json:"reason_code" binding:"max=128"`
	RequestID      string          `json:"request_id" binding:"max=128"`
	TraceID        string          `json:"trace_id" binding:"max=128"`
	IPAddress      string          `json:"ip_address" binding:"max=128"`
	UserAgent      string          `json:"user_agent" binding:"max=1024"`
	Detail         json.RawMessage `json:"detail"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required,max=512"`
}

type PlatformAuditLogListRequest struct {
	imachinery.BasicQueryParam
	SourceDomain string `form:"source_domain"`
	SourceModule string `form:"source_module"`
	Principal    string `form:"principal"`
	Action       string `form:"action"`
	Result       string `form:"result"`
}

type PlatformRuntimeMetadata struct {
	PlatformName   string          `json:"platform_name"`
	Version        string          `json:"version"`
	Environment    string          `json:"environment"`
	DeploymentMode string          `json:"deployment_mode"`
	Timezone       string          `json:"timezone"`
	StartedAt      imachinery.Time `json:"started_at"`
	UptimeSeconds  int64           `json:"uptime_seconds"`
}
type PlatformAuditLogSummary struct {
	ID           string          `json:"id"`
	SourceDomain string          `json:"source_domain"`
	SourceModule string          `json:"source_module"`
	Action       string          `json:"action"`
	TargetType   string          `json:"target_type,omitempty"`
	TargetID     string          `json:"target_id,omitempty"`
	Result       string          `json:"result"`
	CreatedAt    imachinery.Time `json:"created_at"`
}
type PlatformOverview struct {
	Platform        PlatformRuntimeMetadata   `json:"platform"`
	RecentAuditLogs []PlatformAuditLogSummary `json:"recent_audit_logs"`
}
type PlatformPasswordHashPolicy struct {
	Algorithm   string `json:"algorithm"`
	Version     int    `json:"version"`
	MemoryKib   int    `json:"memory_kib"`
	Iterations  int    `json:"iterations"`
	Parallelism int    `json:"parallelism"`
	OutputBytes int    `json:"output_bytes"`
	SaltBytes   int    `json:"salt_bytes"`
}
type PlatformSystemAuthConfigResponse struct {
	ID                          string                     `json:"id"`
	RegistrationMode            string                     `json:"registration_mode"`
	AllowRegistration           bool                       `json:"allow_registration"`
	PasswordPolicy              json.RawMessage            `json:"password_policy"`
	PasswordHashPolicy          PlatformPasswordHashPolicy `json:"password_hash_policy"`
	LoginFailurePolicy          json.RawMessage            `json:"login_failure_policy"`
	OnlinePresenceWindowSeconds int                        `json:"online_presence_window_seconds"`
	AccessTokenLifetime         int                        `json:"access_token_lifetime"`
	RefreshTokenLifetime        int                        `json:"refresh_token_lifetime"`
	ResourceVersion             int64                      `json:"resource_version"`
	UpdatedAt                   imachinery.Time            `json:"updated_at"`
}
type PlatformAuditLogListResponse struct {
	Total int64               `json:"total"`
	Items []*PlatformAuditLog `json:"items"`
}
