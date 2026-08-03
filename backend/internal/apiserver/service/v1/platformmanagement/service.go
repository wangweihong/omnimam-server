package platformmanagement

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	identitymiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/platformaudit"
)

var startedAt = time.Now()

var (
	platformSourceDomainPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	platformSourceModulePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	platformAuditActionPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)
)

// Service implements v1.11 platform-management without owning Identity tables or business-domain statistics.
type Service struct{ store store.PlatformManagementStore }

func NewService(platformStore store.PlatformManagementStore) *Service {
	return &Service{store: platformStore}
}

func (s *Service) Overview(ctx context.Context) (*iapiserver.PlatformOverview, error) {
	if s.store == nil {
		return nil, errors.NewStatus(code.ErrPlatformOverviewUnavailable, "platform store is unavailable")
	}
	req := &iapiserver.PlatformAuditLogListRequest{}
	req.PageSize = 10
	req.Action = "platform.auth_config.update"
	req.SetDefaults()
	logs, _, err := s.store.ListAuditLogs(ctx, req)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformOverviewUnavailable, "audit summary is unavailable")
	}
	result := &iapiserver.PlatformOverview{Platform: iapiserver.PlatformRuntimeMetadata{PlatformName: "OmniMAM", Version: os.Getenv("OMNIMAM_VERSION"), Environment: os.Getenv("OMNIMAM_ENVIRONMENT"), DeploymentMode: "standalone", Timezone: time.Local.String(), StartedAt: imachinery.NewTime(startedAt), UptimeSeconds: int64(time.Since(startedAt).Seconds())}, RecentAuditLogs: make([]iapiserver.PlatformAuditLogSummary, 0, len(logs))}
	if result.Platform.Version == "" {
		result.Platform.Version = "unknown"
	}
	if result.Platform.Environment == "" {
		result.Platform.Environment = "unknown"
	}
	for _, log := range logs {
		result.RecentAuditLogs = append(result.RecentAuditLogs, iapiserver.PlatformAuditLogSummary{ID: log.ID, SourceDomain: log.SourceDomain, SourceModule: log.SourceModule, Action: log.Action, TargetType: log.TargetType, TargetID: log.TargetID, Result: log.Result, OccurredAt: log.OccurredAt, CreatedAt: log.CreatedAt})
	}
	return result, nil
}

func (s *Service) GetAuthConfig(ctx context.Context) (*iapiserver.PlatformSystemAuthConfigResponse, error) {
	config, err := s.store.GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	return authConfigResponse(config), nil
}

func (s *Service) ReplaceAuthConfig(ctx context.Context, req *iapiserver.PlatformAuthConfigReplaceRequest) (*iapiserver.PlatformSystemAuthConfigResponse, error) {
	if err := validateAuthConfig(req); err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, err.Error())
	}
	p, _ := identitymiddleware.PrincipalFromContext(ctx)
	config := &iapiserver.PlatformSystemAuthConfig{RegistrationMode: req.RegistrationMode, PasswordPolicy: req.PasswordPolicy, LoginFailurePolicy: req.LoginFailurePolicy, OnlinePresenceWindowSeconds: req.OnlinePresenceWindowSeconds, AccessTokenLifetimeSeconds: req.AccessTokenLifetime, RefreshTokenLifetimeSeconds: req.RefreshTokenLifetime, UpdatedByPrincipalType: p.PrincipalType, UpdatedByPrincipalID: p.PrincipalID, ActorUserID: p.ActorUserID}
	updated, err := s.store.ReplaceSystemAuthConfig(ctx, config, req.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return authConfigResponse(updated), nil
}

func (s *Service) ListAudit(ctx context.Context, req *iapiserver.PlatformAuditLogListRequest) (*iapiserver.PlatformAuditLogListResponse, error) {
	req.SetDefaults()
	if err := req.Validate(); err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuditQueryInvalid, err.Error())
	}
	items, total, err := s.store.ListAuditLogs(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.PlatformAuditLogListResponse{Total: total, Items: items}, nil
}

func (s *Service) GetAudit(ctx context.Context, id string) (*iapiserver.PlatformAuditLog, error) {
	item, err := s.store.GetAuditLog(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuditLogNotVisible, "audit record is not visible")
	}
	return item, nil
}

func (s *Service) AppendAudit(ctx context.Context, req *iapiserver.PlatformAuditRecordRequest) (*iapiserver.PlatformAuditLog, error) {
	if len(req.Detail) == 0 || string(req.Detail) == "null" {
		req.Detail = json.RawMessage(`{}`)
	}
	if err := validateAuditRecord(req, time.Now()); err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, err.Error())
	}
	principal, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || principal.PrincipalType != "SERVICE_ACCOUNT" {
		return nil, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, "service principal is required")
	}
	fingerprint, err := platformaudit.CanonicalJSONDigest(req)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, "audit request cannot be canonicalized")
	}
	record := &iapiserver.PlatformAuditLog{SourceDomain: req.SourceDomain, SourceModule: req.SourceModule, PrincipalType: req.PrincipalType, PrincipalID: req.PrincipalID, ActorUserID: req.ActorUserID, Action: req.Action, TargetType: req.TargetType, TargetID: req.TargetID, OwnerUserID: req.OwnerUserID, Result: req.Result, ReasonCode: req.ReasonCode, RequestID: req.RequestID, TraceID: req.TraceID, IPAddress: req.IPAddress, UserAgent: req.UserAgent, OccurredAt: req.OccurredAt, Detail: req.Detail, IdempotencyKey: req.IdempotencyKey, ContentFingerprint: fingerprint}
	return s.store.AppendAuditLog(ctx, record)
}

func validateAuthConfig(req *iapiserver.PlatformAuthConfigReplaceRequest) error {
	if req == nil {
		return fmt.Errorf("system auth config is required")
	}
	if req.RegistrationMode != "OPEN" && req.RegistrationMode != "ADMIN_APPROVAL" {
		return fmt.Errorf("registration_mode is invalid")
	}
	password := req.PasswordPolicy
	if password.MinLength < 8 || password.MinLength > 64 || password.MaxLength < 64 || password.MaxLength > 256 || password.MaxLength < password.MinLength {
		return fmt.Errorf("password_policy length range is invalid")
	}
	login := req.LoginFailurePolicy
	if login.MaxFailedAttempts < 3 || login.MaxFailedAttempts > 20 || login.FailureWindowSeconds < 60 || login.FailureWindowSeconds > 86400 || login.LockoutDurationSeconds < 60 || login.LockoutDurationSeconds > 86400 {
		return fmt.Errorf("login_failure_policy is invalid")
	}
	if req.OnlinePresenceWindowSeconds < 30 || req.OnlinePresenceWindowSeconds > 3600 {
		return fmt.Errorf("online_presence_window_seconds is invalid")
	}
	if req.AccessTokenLifetime < 300 || req.AccessTokenLifetime > 86400 {
		return fmt.Errorf("access_token_lifetime is invalid")
	}
	if req.RefreshTokenLifetime < 3600 || req.RefreshTokenLifetime > 31536000 || req.RefreshTokenLifetime <= req.AccessTokenLifetime {
		return fmt.Errorf("refresh_token_lifetime is invalid")
	}
	if req.ResourceVersion < 0 {
		return fmt.Errorf("resource_version is invalid")
	}
	return nil
}

func validateAuditRecord(req *iapiserver.PlatformAuditRecordRequest, now time.Time) error {
	if req == nil {
		return fmt.Errorf("audit record is required")
	}
	if len(req.SourceDomain) < 1 || len(req.SourceDomain) > 64 || !platformSourceDomainPattern.MatchString(req.SourceDomain) {
		return fmt.Errorf("source_domain is invalid")
	}
	if len(req.SourceModule) < 1 || len(req.SourceModule) > 64 || !platformSourceModulePattern.MatchString(req.SourceModule) {
		return fmt.Errorf("source_module is invalid")
	}
	if !platformValueAllowed(req.PrincipalType, "USER", "SERVICE_ACCOUNT", "ANONYMOUS") || !platformValueAllowed(req.Result, "SUCCESS", "FAILED", "DENIED") {
		return fmt.Errorf("audit principal_type or result is invalid")
	}
	if len(req.Action) < 3 || len(req.Action) > 128 || !platformAuditActionPattern.MatchString(req.Action) {
		return fmt.Errorf("action is invalid")
	}
	for name, value := range map[string]string{
		"principal_id": req.PrincipalID, "actor_user_id": req.ActorUserID, "target_type": req.TargetType,
		"owner_user_id": req.OwnerUserID, "reason_code": req.ReasonCode, "request_id": req.RequestID, "trace_id": req.TraceID,
	} {
		if len(value) > 128 {
			return fmt.Errorf("%s exceeds 128 bytes", name)
		}
	}
	if len(req.TargetID) > 256 || len(req.IPAddress) > 64 || len(req.UserAgent) > 512 || len(req.IdempotencyKey) < 1 || len(req.IdempotencyKey) > 256 {
		return fmt.Errorf("audit target or request metadata is invalid")
	}
	if req.OccurredAt.IsZero() || req.OccurredAt.Time.After(now.Add(5*time.Minute)) {
		return fmt.Errorf("occurred_at is invalid")
	}
	return validateAuditDetail(req.Detail)
}

func validateAuditDetail(raw json.RawMessage) error {
	if len(raw) > 16*1024 || !utf8.Valid(raw) || !json.Valid(raw) {
		return fmt.Errorf("audit detail must be valid UTF-8 JSON no larger than 16 KiB")
	}
	var detail map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&detail); err != nil || detail == nil {
		return fmt.Errorf("audit detail must be a JSON object")
	}
	if len(detail) > 32 {
		return fmt.Errorf("audit detail exceeds 32 top-level properties")
	}
	if auditJSONDepth(detail, 1) > 4 {
		return fmt.Errorf("audit detail exceeds maximum depth 4")
	}
	if auditDetailContainsProhibitedContent(detail) {
		return fmt.Errorf("audit detail contains prohibited sensitive content")
	}
	return nil
}

func auditJSONDepth(value any, depth int) int {
	maxDepth := depth
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			childDepth := depth
			switch child.(type) {
			case map[string]any, []any:
				childDepth = auditJSONDepth(child, depth+1)
			}
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
		}
	case []any:
		for _, child := range typed {
			childDepth := depth
			switch child.(type) {
			case map[string]any, []any:
				childDepth = auditJSONDepth(child, depth+1)
			}
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
		}
	}
	return maxDepth
}

func auditDetailContainsProhibitedContent(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
			if platformAuditSensitiveKey(normalized) || auditDetailContainsProhibitedContent(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if auditDetailContainsProhibitedContent(child) {
				return true
			}
		}
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		if strings.HasPrefix(lower, "bearer ") || strings.HasPrefix(lower, "basic ") || strings.HasPrefix(lower, "file://") ||
			strings.HasPrefix(lower, "/home/") || strings.HasPrefix(lower, "/etc/") || strings.HasPrefix(lower, "/var/") ||
			strings.HasPrefix(lower, "/tmp/") || strings.HasPrefix(lower, "/usr/") {
			return true
		}
	}
	return false
}

func platformAuditSensitiveKey(key string) bool {
	if strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "credential") || strings.Contains(key, "authorization") {
		return true
	}
	switch key {
	case "token", "access_token", "refresh_token", "id_token", "api_key", "cookie", "set_cookie", "raw_request", "raw_request_body", "request_body", "provider_response", "response_body", "filesystem_path", "file_path":
		return true
	default:
		return false
	}
}

func platformValueAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func authConfigResponse(config *iapiserver.PlatformSystemAuthConfig) *iapiserver.PlatformSystemAuthConfigResponse {
	return &iapiserver.PlatformSystemAuthConfigResponse{ID: config.ID, RegistrationMode: config.RegistrationMode, AllowRegistration: config.RegistrationMode == "OPEN", PasswordPolicy: config.PasswordPolicy, PasswordHashPolicy: iapiserver.PlatformPasswordHashPolicy{Algorithm: "ARGON2ID", Version: 19, MemoryKib: 65536, Iterations: 3, Parallelism: 1, OutputBytes: 32, SaltBytes: 16}, LoginFailurePolicy: config.LoginFailurePolicy, OnlinePresenceWindowSeconds: config.OnlinePresenceWindowSeconds, AccessTokenLifetime: config.AccessTokenLifetimeSeconds, RefreshTokenLifetime: config.RefreshTokenLifetimeSeconds, ResourceVersion: config.ResourceVersion, UpdatedAt: config.UpdatedAt}
}
