package iapiserver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type PlatformPasswordPolicy struct {
	MinLength               int  `json:"min_length"`
	MaxLength               int  `json:"max_length"`
	RequireUppercase        bool `json:"require_uppercase"`
	RequireLowercase        bool `json:"require_lowercase"`
	RequireDigit            bool `json:"require_digit"`
	RequireSpecialCharacter bool `json:"require_special_character"`
	DisallowUsername        bool `json:"disallow_username"`
}

type PlatformLoginFailurePolicy struct {
	MaxFailedAttempts      int `json:"max_failed_attempts"`
	FailureWindowSeconds   int `json:"failure_window_seconds"`
	LockoutDurationSeconds int `json:"lockout_duration_seconds"`
}

// PlatformAuthConfigReplaceRequest 替换认证策略；hash 算法和基线参数不允许客户端修改。
type PlatformAuthConfigReplaceRequest struct {
	RegistrationMode            string                     `json:"registration_mode"`
	PasswordPolicy              PlatformPasswordPolicy     `json:"password_policy"`
	LoginFailurePolicy          PlatformLoginFailurePolicy `json:"login_failure_policy"`
	OnlinePresenceWindowSeconds int                        `json:"online_presence_window_seconds"`
	AccessTokenLifetime         int                        `json:"access_token_lifetime"`
	RefreshTokenLifetime        int                        `json:"refresh_token_lifetime"`
	ResourceVersion             int64                      `json:"resource_version" binding:"min=0"`
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
	OccurredAt     imachinery.Time `json:"occurred_at"`
	Detail         json.RawMessage `json:"detail"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required,max=512"`
}

type PlatformAuditLogListRequest struct {
	imachinery.PagingParams
	Keyword        string   `form:"keyword"`
	SearchFields   []string `form:"search_fields"`
	SortField      string   `form:"sort_field"`
	SortOrder      string   `form:"sort_order"`
	SourceDomain   string   `form:"source_domain"`
	SourceModule   string   `form:"source_module"`
	PrincipalType  string   `form:"principal_type"`
	PrincipalID    string   `form:"principal_id"`
	Action         string   `form:"action"`
	TargetType     string   `form:"target_type"`
	TargetID       string   `form:"target_id"`
	RequestID      string   `form:"request_id"`
	Result         string   `form:"result"`
	OccurredAfter  string   `form:"occurred_after"`
	OccurredBefore string   `form:"occurred_before"`
	CreatedAfter   string   `form:"created_after"`
	CreatedBefore  string   `form:"created_before"`
}

func (r *PlatformAuditLogListRequest) Decode(c *gin.Context) error {
	query := c.Request.URL.Query()
	if value := query.Get("page_num"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("page_num must be an integer")
		}
		r.PageNum = parsed
	}
	if value := query.Get("page_size"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("page_size must be an integer")
		}
		r.PageSize = parsed
	}
	r.Keyword = query.Get("keyword")
	if fields := query.Get("search_fields"); fields != "" {
		for _, field := range strings.Split(fields, ",") {
			if field = strings.TrimSpace(field); field != "" {
				r.SearchFields = append(r.SearchFields, field)
			}
		}
	}
	r.SortField = query.Get("sort_field")
	r.SortOrder = strings.ToLower(query.Get("sort_order"))
	r.SourceDomain = query.Get("source_domain")
	r.SourceModule = query.Get("source_module")
	r.PrincipalType = query.Get("principal_type")
	r.PrincipalID = query.Get("principal_id")
	r.Action = query.Get("action")
	r.TargetType = query.Get("target_type")
	r.TargetID = query.Get("target_id")
	r.RequestID = query.Get("request_id")
	r.Result = query.Get("result")
	r.OccurredAfter = query.Get("occurred_after")
	r.OccurredBefore = query.Get("occurred_before")
	r.CreatedAfter = query.Get("created_after")
	r.CreatedBefore = query.Get("created_before")
	return nil
}

func (r *PlatformAuditLogListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 20
	}
	if len(r.SearchFields) == 0 {
		r.SearchFields = []string{"action", "reason_code", "request_id", "target_type", "target_id"}
	}
	if r.SortField == "" {
		r.SortField = "occurred_at"
	}
	if r.SortOrder == "" {
		r.SortOrder = "desc"
	}
}

func (r *PlatformAuditLogListRequest) Validate() error {
	if r.PageNum < 0 || r.PageSize < 1 || r.PageSize > 100 || len(r.Keyword) > 128 {
		return fmt.Errorf("platform audit pagination is invalid")
	}
	if !platformAuditValueAllowed(r.SortField, "occurred_at", "created_at", "action", "result", "source_domain") ||
		!platformAuditValueAllowed(r.SortOrder, "asc", "desc") {
		return fmt.Errorf("platform audit sort is invalid")
	}
	for _, field := range r.SearchFields {
		if !platformAuditValueAllowed(field, "action", "reason_code", "request_id", "target_type", "target_id") {
			return fmt.Errorf("platform audit search_fields is invalid")
		}
	}
	for _, value := range []string{r.OccurredAfter, r.OccurredBefore, r.CreatedAfter, r.CreatedBefore} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return fmt.Errorf("platform audit time filters must use RFC3339")
			}
		}
	}
	if r.PrincipalType != "" && !platformAuditValueAllowed(r.PrincipalType, "USER", "SERVICE_ACCOUNT", "ANONYMOUS") {
		return fmt.Errorf("platform audit principal_type is invalid")
	}
	if r.Result != "" && !platformAuditValueAllowed(r.Result, "SUCCESS", "FAILED", "DENIED") {
		return fmt.Errorf("platform audit result is invalid")
	}
	return nil
}

func platformAuditValueAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
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
	OccurredAt   imachinery.Time `json:"occurred_at"`
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
	PasswordPolicy              PlatformPasswordPolicy     `json:"password_policy"`
	PasswordHashPolicy          PlatformPasswordHashPolicy `json:"password_hash_policy"`
	LoginFailurePolicy          PlatformLoginFailurePolicy `json:"login_failure_policy"`
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
