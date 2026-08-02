package platformmanagement

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	identitymiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

var startedAt = time.Now()

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
		result.RecentAuditLogs = append(result.RecentAuditLogs, iapiserver.PlatformAuditLogSummary{ID: log.ID, SourceDomain: log.SourceDomain, SourceModule: log.SourceModule, Action: log.Action, TargetType: log.TargetType, TargetID: log.TargetID, Result: log.Result, CreatedAt: log.CreatedAt})
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
	if req.RefreshTokenLifetime <= req.AccessTokenLifetime {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "refresh token lifetime must exceed access token lifetime")
	}
	if len(req.PasswordPolicy) == 0 || !json.Valid(req.PasswordPolicy) || len(req.LoginFailurePolicy) == 0 || !json.Valid(req.LoginFailurePolicy) {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "authentication policy must be valid JSON")
	}
	p, _ := identitymiddleware.PrincipalFromContext(ctx)
	config := &iapiserver.PlatformSystemAuthConfig{RegistrationMode: req.RegistrationMode, PasswordPolicy: req.PasswordPolicy, LoginFailurePolicy: req.LoginFailurePolicy, OnlinePresenceWindowSeconds: req.OnlinePresenceWindowSeconds, AccessTokenLifetimeSeconds: req.AccessTokenLifetime, RefreshTokenLifetimeSeconds: req.RefreshTokenLifetime, UpdatedByPrincipalType: p.PrincipalType, UpdatedByPrincipalID: p.PrincipalID}
	updated, err := s.store.ReplaceSystemAuthConfig(ctx, config, req.ResourceVersion)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigVersionConflict, "system auth config version conflict")
	}
	return authConfigResponse(updated), nil
}

func (s *Service) ListAudit(ctx context.Context, req *iapiserver.PlatformAuditLogListRequest) (*iapiserver.PlatformAuditLogListResponse, error) {
	items, total, err := s.store.ListAuditLogs(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.PlatformAuditLogListResponse{Total: total, Items: items}, nil
}

func (s *Service) GetAudit(ctx context.Context, id string) (*iapiserver.PlatformAuditLog, error) {
	item, err := s.store.GetAuditLog(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, "audit record is not visible")
	}
	return item, nil
}

func (s *Service) AppendAudit(ctx context.Context, req *iapiserver.PlatformAuditRecordRequest) (*iapiserver.PlatformAuditLog, error) {
	if req.Detail == nil {
		req.Detail = json.RawMessage(`{}`)
	}
	if !json.Valid(req.Detail) {
		return nil, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, "audit detail is invalid")
	}
	principal, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || principal.PrincipalType != "SERVICE_ACCOUNT" {
		return nil, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, "service principal is required")
	}
	record := &iapiserver.PlatformAuditLog{SourceDomain: req.SourceDomain, SourceModule: req.SourceModule, PrincipalType: req.PrincipalType, PrincipalID: req.PrincipalID, ActorUserID: req.ActorUserID, Action: req.Action, TargetType: req.TargetType, TargetID: req.TargetID, OwnerUserID: req.OwnerUserID, Result: req.Result, ReasonCode: req.ReasonCode, RequestID: req.RequestID, TraceID: req.TraceID, IPAddress: req.IPAddress, UserAgent: req.UserAgent, Detail: req.Detail, IdempotencyKey: req.IdempotencyKey}
	return s.store.AppendAuditLog(ctx, record)
}

func authConfigResponse(config *iapiserver.PlatformSystemAuthConfig) *iapiserver.PlatformSystemAuthConfigResponse {
	return &iapiserver.PlatformSystemAuthConfigResponse{ID: config.ID, RegistrationMode: config.RegistrationMode, AllowRegistration: config.RegistrationMode == "OPEN", PasswordPolicy: config.PasswordPolicy, PasswordHashPolicy: iapiserver.PlatformPasswordHashPolicy{Algorithm: "ARGON2ID", Version: 19, MemoryKib: 65536, Iterations: 3, Parallelism: 1, OutputBytes: 32, SaltBytes: 16}, LoginFailurePolicy: config.LoginFailurePolicy, OnlinePresenceWindowSeconds: config.OnlinePresenceWindowSeconds, AccessTokenLifetime: config.AccessTokenLifetimeSeconds, RefreshTokenLifetime: config.RefreshTokenLifetimeSeconds, ResourceVersion: config.ResourceVersion, UpdatedAt: config.UpdatedAt}
}
