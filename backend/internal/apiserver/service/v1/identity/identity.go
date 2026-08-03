package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"golang.org/x/crypto/argon2"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	identitymiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	argonMemory     = uint32(64 * 1024)
	argonTime       = uint32(3)
	argonThreads    = uint8(1)
	argonKeyLength  = uint32(32)
	argonSaltLength = 16
)

// DefaultPermissions 返回当前发布版本需要由系统登记的 Identity 与 Platform 权限定义。
func DefaultPermissions() []*iapiserver.IdentityPermissionDefinition {
	items := []struct{ code, domain, resource, action string }{
		{"identity.user.read", "identity", "user", "read"}, {"identity.user.manage", "identity", "user", "manage"}, {"identity.registration.review", "identity", "registration_application", "review"},
		{"identity.auth.session", "identity", "auth", "session"}, {"identity.permission.read", "identity", "permission", "read"},
		{"identity.role.manage", "identity", "role", "manage"}, {"identity.group.manage", "identity", "group", "manage"},
		{"identity.resource_grant.read", "identity", "resource_grant", "read"}, {"identity.resource_grant.manage", "identity", "resource_grant", "manage"},
		{"identity.service_account.read", "identity", "service_account", "read"}, {"identity.service_account.manage", "identity", "service_account", "manage"},
		{"identity.authz.check", "identity", "authz", "check"}, {"platform.overview.read", "platform-management", "overview", "read"},
		{"platform.auth_config.read", "platform-management", "auth_config", "read"}, {"platform.auth_config.manage", "platform-management", "auth_config", "manage"},
		{"platform.auth_config.read_internal", "platform-management", "auth_config", "read_internal"}, {"platform.audit.read", "platform-management", "audit", "read"},
		{"platform.audit.record", "platform-management", "audit", "record"},
		{"agent.profile.read", "agent", "profile", "read"}, {"agent.read", "agent", "agent", "read"},
		{"agent.manage", "agent", "agent", "manage"}, {"agent.invoke", "agent", "invocation", "invoke"},
		{"agent.session.read", "agent", "session", "read"}, {"agent.session.manage", "agent", "session", "manage"},
		{"agent.workspace.read", "agent", "workspace", "read"}, {"agent.memory.read", "agent", "memory", "read"},
		{"agent.memory.manage", "agent", "memory", "manage"}, {"agent.runtime.operate", "agent", "runtime", "operate"},
		{"agent.runtime.logs.read", "agent", "runtime_logs", "read"},
		{"appstudio.application.read", "appstudio", "application", "read"}, {"appstudio.application.manage", "appstudio", "application", "manage"},
		{"appstudio.workspace.read", "appstudio", "workspace", "read"}, {"appstudio.workspace.write", "appstudio", "workspace", "write"},
		{"appstudio.snapshot.manage", "appstudio", "snapshot", "manage"}, {"appstudio.build.manage", "appstudio", "build", "manage"},
		{"appstudio.preview.operate", "appstudio", "preview", "operate"}, {"appstudio.runtime_config.manage", "appstudio", "runtime_config", "manage"},
		{"appstudio.release.manage", "appstudio", "release", "manage"},
		{"asset.read", "asset-library", "asset", "read"}, {"asset.artifact.read", "asset-library", "artifact", "read"},
		{"asset.create", "asset-library", "asset", "create"}, {"asset.update", "asset-library", "asset", "update"},
		{"asset.delete", "asset-library", "asset", "delete"}, {"asset.upload", "asset-library", "asset_upload", "manage"},
		{"asset.content.read", "asset-library", "asset_content", "read"}, {"asset.collection.read", "asset-library", "collection", "read"},
		{"asset.collection.manage", "asset-library", "collection", "manage"}, {"asset.label.manage", "asset-library", "asset_label", "manage"},
		{"asset.reference.read", "asset-library", "asset_reference", "read"}, {"asset.representation.read", "asset-library", "asset_representation", "read"},
		{"asset.artifact.register", "asset-library", "artifact_registration", "create"}, {"asset.artifact.delete", "asset-library", "artifact", "delete"},
		{"asset.storage.read", "asset-library", "asset_storage", "read"}, {"asset.storage.manage", "asset-library", "asset_storage", "manage"},
		{"asset.artifact.create", "asset-library", "artifact", "create"}, {"asset.representation.write", "asset-library", "asset_representation", "create"},
		{"aiapp.application.read", "application-platform", "application", "read"}, {"aiapp.application.manage", "application-platform", "application", "manage"},
		{"aiapp.application.run", "application-platform", "application_run", "run"},
		{"aiapp.comfyui_workflow.read", "application-platform", "comfyui_workflow", "read"},
		{"aiapp.comfyui_workflow.manage", "application-platform", "comfyui_workflow", "manage"},
		{"aiapp.comfyui_workflow.validate", "application-platform", "comfyui_workflow_validation", "validate"},
		{"aiapp.comfyui_workflow.convert", "application-platform", "comfyui_workflow", "convert"},
		{"aiapp.comfyui_workflow.test", "application-platform", "comfyui_workflow_test_run", "test"},
		{"aiapp.engine_instance.read", "modelgateway", "application_engine_instance", "read"},
		{"aiapp.engine_instance.manage", "modelgateway", "application_engine_instance", "manage"},
		{"aiapp.engine_binding.manage", "modelgateway", "engine_capability_binding", "manage"},
		{"aiapp.provider_capability.read", "modelgateway", "provider_capability", "read"},
		{"task.atomic.operate", "task-center", "atomic_task, task_attempt", "create, read, cancel, retry"},
		{"task.group.operate", "task-center", "task_group, dag_task_group", "create, read, cancel, retry"},
		{"task.schedule.manage", "task-center", "task_schedule, task_schedule_execution", "create, read, update, pause, resume, run, delete"},
		{"task.operation.admin", "task-center", "atomic_task, task_group, dag_task_group, task_schedule, runtime_projection", "inspect_all, reconcile, read_health"},
		{"task.runtime.internal", "task-center", "workflow_runtime, runtime_projection_event, task_log", "register_definition, start, query, append_task_log, read_task_log, cancel, reconcile, project_event"},
		{"workflow.canvas.read", "workflow-canvas", "canvas, canvas_version", "read"}, {"workflow.canvas.edit", "workflow-canvas", "canvas", "create, update, validate"},
		{"workflow.canvas.publish", "workflow-canvas", "canvas_version", "publish"}, {"workflow.canvas.delete", "workflow-canvas", "canvas", "delete"},
		{"workflow.run.create", "workflow-canvas", "canvas_run, canvas_flow_run, canvas_node_run", "validate, create"},
		{"workflow.run.read", "workflow-canvas", "canvas_run, canvas_flow_run, canvas_node_run, task_binding, output_binding", "read"},
		{"workflow.run.cancel", "workflow-canvas", "canvas_run", "cancel"}, {"workflow.run.retry", "workflow-canvas", "canvas_run", "retry"},
		{"workflow.node_definition.read", "workflow-canvas", "node_definition", "read"}, {"workflow.node_definition.manage", "workflow-canvas", "node_definition", "register, deprecate"},
		{"workflow.projection.internal", "workflow-canvas", "canvas_run, canvas_flow_run, canvas_node_run, task_binding, output_binding, outbox, reconcile_cursor", "bind_task, project_task, project_artifact, publish_event, reconcile"},
	}
	result := make([]*iapiserver.IdentityPermissionDefinition, 0, len(items))
	for _, item := range items {
		result = append(result, &iapiserver.IdentityPermissionDefinition{ObjectMeta: imachinery.ObjectMeta{Name: item.code}, Code: item.code, Domain: item.domain, Resource: item.resource, Action: item.action, RiskLevel: "NORMAL", Status: "ACTIVE"})
	}
	return result
}

// DefaultRolePermissions 返回管理员和超级管理员可用于前端入口及业务操作的默认权限。
// artifact/representation/task/workflow 的内部权限仅登记到权限目录，不授予用户角色。
func DefaultRolePermissions() map[string][]string {
	permissions := []string{
		"identity.auth.session", "identity.user.read", "identity.permission.read",
		"asset.read", "asset.artifact.read", "asset.create", "asset.update", "asset.delete", "asset.upload",
		"asset.content.read", "asset.collection.read", "asset.collection.manage", "asset.label.manage", "asset.reference.read",
		"asset.representation.read", "asset.artifact.register", "asset.artifact.delete", "asset.storage.read", "asset.storage.manage",
		"aiapp.application.read", "aiapp.application.manage", "aiapp.application.run",
		"aiapp.comfyui_workflow.read", "aiapp.comfyui_workflow.manage", "aiapp.comfyui_workflow.validate",
		"aiapp.comfyui_workflow.convert", "aiapp.comfyui_workflow.test",
		"aiapp.engine_instance.read", "aiapp.engine_instance.manage", "aiapp.engine_binding.manage", "aiapp.provider_capability.read",
		"task.atomic.operate", "task.group.operate", "task.schedule.manage", "task.operation.admin",
		"workflow.canvas.read", "workflow.canvas.edit", "workflow.canvas.publish", "workflow.canvas.delete",
		"workflow.run.create", "workflow.run.read", "workflow.run.cancel", "workflow.run.retry",
		"workflow.node_definition.read", "workflow.node_definition.manage",
	}
	return map[string][]string{
		"ADMIN":       append([]string(nil), permissions...),
		"SUPER_ADMIN": append([]string(nil), permissions...),
	}
}

func (s *Service) Register(ctx context.Context, req *iapiserver.IdentityRegisterRequest) (any, error) {
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	if err := validatePassword(req.Password, req.Username, config.PasswordPolicy); err != nil {
		return nil, err
	}
	normalizedUsername := normalize(req.Username)
	normalizedEmail := normalize(req.Email)
	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password hashing failed")
	}
	email := strings.TrimSpace(req.Email)
	user := &iapiserver.IdentityUser{ObjectMeta: imachinery.ObjectMeta{Name: req.Username}, Username: strings.TrimSpace(req.Username), NormalizedUsername: normalizedUsername, DisplayName: strings.TrimSpace(req.DisplayName), Email: &email, NormalizedEmail: &normalizedEmail, PasswordHash: hash, Status: iapiserver.IdentityUserActive, SecurityVersion: 1, AuthorizationVersion: 1, PasswordChangedAt: pointerTime(imachinery.Now())}
	if config.RegistrationMode == "ADMIN_APPROVAL" {
		created, createErr := s.store.Identities().CreatePendingRegistration(ctx, user)
		if createErr != nil {
			if strings.Contains(strings.ToLower(createErr.Error()), "unique") {
				return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
			}
			return nil, createErr
		}
		return &iapiserver.IdentityPendingRegistrationResponse{RegistrationApplicationID: created.Application.ID, UserID: created.User.ID, Status: created.Application.Status, SubmittedAt: created.Application.SubmittedAt}, nil
	}
	if config.RegistrationMode != "OPEN" {
		return nil, errors.NewStatus(code.ErrIdentityRegistrationStateInvalid, "registration mode is invalid")
	}
	if existing, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalizedUsername); lookupErr == nil && existing != nil && existing.ID != "" {
		return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username already exists")
	}
	if existing, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalizedEmail); lookupErr == nil && existing != nil && existing.ID != "" {
		return nil, errors.NewStatus(code.ErrIdentityEmailAlreadyExists, "email already exists")
	}
	created, err := s.store.Identities().CreateOpenRegistration(ctx, user)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
		}
		return nil, err
	}
	return s.issueSession(ctx, created, "register", "", "")
}

func (s *Service) Login(ctx context.Context, req *iapiserver.IdentityLoginRequest, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	user, err := s.store.Identities().GetUserByLogin(ctx, normalize(req.Login))
	if err != nil || user == nil {
		return nil, errors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	valid, verifyErr := verifyPassword(user.PasswordHash, req.Password)
	if verifyErr != nil || !valid {
		return nil, errors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	switch user.Status {
	case iapiserver.IdentityUserPending:
		return nil, errors.NewStatus(code.ErrIdentityAccountPending, "account is pending approval")
	case iapiserver.IdentityUserRejected:
		return nil, errors.NewStatus(code.ErrIdentityAccountRejected, "account registration was rejected")
	case iapiserver.IdentityUserDisabled, iapiserver.IdentityUserDeleted:
		return nil, errors.NewStatus(code.ErrIdentityAccountDisabled, "account is disabled")
	case iapiserver.IdentityUserLocked:
		if user.LockedUntil != nil && user.LockedUntil.Time.After(time.Now()) {
			return nil, errors.NewStatus(code.ErrIdentityAccountLocked, "account is locked")
		}
	case iapiserver.IdentityUserActive:
	default:
		return nil, errors.NewStatus(code.ErrIdentityUserStateInvalid, "account state is invalid")
	}
	user.FailedLoginCount = 0
	user.LastLoginAt = pointerTime(imachinery.Now())
	user.Status = iapiserver.IdentityUserActive
	updated, err := s.store.Identities().UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, updated, "login", ip, userAgent)
}

func (s *Service) Refresh(ctx context.Context, req *iapiserver.IdentityRefreshRequest) (*iapiserver.IdentityAuthUserResponse, error) {
	refresh, err := s.store.Identities().GetRefreshTokenByHash(ctx, identitymiddleware.HashRefreshToken(req.RefreshToken))
	if err != nil || refresh == nil {
		return nil, errors.NewStatus(code.ErrIdentityRefreshTokenInvalid, "refresh token is invalid")
	}
	if refresh.Status != "ACTIVE" || refresh.ExpiresAt.Time.Before(time.Now()) {
		if refresh.Status == "USED" {
			_ = s.store.Identities().RevokeSession(ctx, refresh.SessionID, "TOKEN_REUSE")
			_ = s.store.Identities().RevokeSessionRefreshTokens(ctx, refresh.SessionID, "TOKEN_REUSE")
			return nil, errors.NewStatus(code.ErrIdentityRefreshTokenReused, "refresh token reuse detected")
		}
		return nil, errors.NewStatus(code.ErrIdentityRefreshTokenInvalid, "refresh token is invalid")
	}
	if err := s.store.Identities().MarkRefreshTokenUsed(ctx, refresh.ID); err != nil {
		_ = s.store.Identities().RevokeSession(ctx, refresh.SessionID, "TOKEN_REUSE")
		_ = s.store.Identities().RevokeSessionRefreshTokens(ctx, refresh.SessionID, "TOKEN_REUSE")
		return nil, errors.NewStatus(code.ErrIdentityRefreshTokenReused, "refresh token reuse detected")
	}
	session, err := s.store.Identities().GetSession(ctx, refresh.SessionID)
	if err != nil || session.Status != "ACTIVE" {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "session is revoked")
	}
	user, err := s.store.Identities().GetUser(ctx, session.UserID)
	if err != nil || user.Status != iapiserver.IdentityUserActive {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "user is not active")
	}
	return s.issueSessionOnExisting(ctx, user, session, "refresh", "", "")
}

func (s *Service) Me(ctx context.Context) (*iapiserver.IdentityUser, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity principal context is missing")
	}
	user, err := s.store.Identities().GetUser(ctx, p.PrincipalID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	user.Email = redactEmail(user.Email)
	return user, nil
}

func (s *Service) Heartbeat(ctx context.Context) (*iapiserver.IdentityPresenceHeartbeatResponse, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.SessionID == "" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "session context is missing")
	}
	session, err := s.store.Identities().TouchSession(ctx, p.SessionID, imachinery.Now())
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "session is revoked")
	}
	return &iapiserver.IdentityPresenceHeartbeatResponse{Online: true, LastActiveAt: *session.LastActiveAt}, nil
}

func (s *Service) Sessions(ctx context.Context, req *iapiserver.IdentitySessionListRequest) (*iapiserver.IdentitySessionListResponse, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	items, total, err := s.store.Identities().ListSessions(ctx, p.PrincipalID, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentitySessionListResponse{Total: total, Items: items}, nil
}

func (s *Service) RevokeSession(ctx context.Context, id string) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	session, err := s.store.Identities().GetSession(ctx, id)
	if err != nil || session.UserID != p.PrincipalID {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "session is not visible")
	}
	if err := s.store.Identities().RevokeSession(ctx, id, "LOGOUT"); err != nil {
		return nil, err
	}
	_ = s.store.Identities().RevokeSessionRefreshTokens(ctx, id, "LOGOUT")
	return &iapiserver.IdentityActionResult{Success: true, Message: "session revoked"}, nil
}

func (s *Service) Permissions(ctx context.Context) (*iapiserver.IdentityPermissionProjection, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "principal context is missing")
	}
	codes, version, err := s.store.Identities().PermissionCodes(ctx, p.PrincipalType, p.PrincipalID)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentityPermissionProjection{PrincipalType: p.PrincipalType, PrincipalID: p.PrincipalID, ActorUserID: p.ActorUserID, AuthorizationVersion: version, PermissionCodes: codes}, nil
}

func (s *Service) ListUsers(ctx context.Context, req *iapiserver.IdentityUserListRequest) (any, error) {
	items, total, err := s.store.Identities().ListUsers(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.PasswordHash = ""
		item.NormalizedEmail = nil
	}
	return &iapiserver.IdentityUserListResponse{Total: total, Items: items}, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (*iapiserver.IdentityUser, error) {
	user, err := s.store.Identities().GetUser(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	return user, nil
}

// ListRegistrationApplications 查询管理员可见的自主注册申请及最小用户摘要。
func (s *Service) ListRegistrationApplications(ctx context.Context, req *iapiserver.IdentityRegistrationApplicationListRequest) (any, error) {
	items, total, err := s.store.Identities().ListRegistrationApplications(ctx, req)
	if err != nil {
		return nil, err
	}
	result := &iapiserver.IdentityRegistrationApplicationListResponse{Total: total, Items: make([]*iapiserver.IdentityRegistrationApplicationResponse, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, registrationApplicationResponse(item))
	}
	return result, nil
}

// GetRegistrationApplication 返回单个注册申请详情；不可见申请统一返回稳定业务错误。
func (s *Service) GetRegistrationApplication(ctx context.Context, id string) (*iapiserver.IdentityRegistrationApplicationResponse, error) {
	item, err := s.store.Identities().GetRegistrationApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	return registrationApplicationResponse(item), nil
}

// ReviewRegistrationApplication 原子批准或拒绝注册申请，并保留审批审计与可靠事件。
func (s *Service) ReviewRegistrationApplication(ctx context.Context, id, decision, reason string) (*iapiserver.IdentityRegistrationDecisionResponse, error) {
	principal, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || principal.PrincipalID == "" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "principal context is missing")
	}
	item, err := s.store.Identities().ReviewRegistrationApplication(ctx, id, decision, reason, principal.PrincipalType, principal.PrincipalID, principal.ActorUserID)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentityRegistrationDecisionResponse{Application: registrationApplicationResponse(item), User: item.User}, nil
}

func registrationApplicationResponse(item *store.IdentityRegistrationApplicationView) *iapiserver.IdentityRegistrationApplicationResponse {
	return &iapiserver.IdentityRegistrationApplicationResponse{
		ID: item.Application.ID, User: item.User, AttemptNo: item.Application.AttemptNo, Status: item.Application.Status,
		SubmittedAt: item.Application.SubmittedAt, DecidedAt: item.Application.DecidedAt, DecidedBy: item.Application.DecidedBy,
		DecisionReason: item.Application.DecisionReason, CreatedAt: item.Application.CreatedAt, UpdatedAt: item.Application.UpdatedAt,
	}
}

func (s *Service) AdminCreateUser(ctx context.Context, req *iapiserver.IdentityAdminUserCreateRequest) (*iapiserver.IdentityUser, error) {
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	password := req.InitialPassword
	if password == "" {
		password = "ChangeMe!123"
	}
	if err := validatePassword(password, req.Username, config.PasswordPolicy); err != nil {
		return nil, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(req.Email)
	normalizedEmail := normalize(email)
	status := req.Status
	if status == "" {
		status = iapiserver.IdentityUserActive
	}
	user := &iapiserver.IdentityUser{ObjectMeta: imachinery.ObjectMeta{Name: req.Username}, Username: strings.TrimSpace(req.Username), NormalizedUsername: normalize(req.Username), DisplayName: strings.TrimSpace(req.DisplayName), Email: &email, NormalizedEmail: &normalizedEmail, PasswordHash: hash, Status: status, FirstLoginRequired: req.InitialPassword == "", SecurityVersion: 1, AuthorizationVersion: 1, PasswordChangedAt: pointerTime(imachinery.Now())}
	created, err := s.store.Identities().CreateUser(ctx, user)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
	}
	created.PasswordHash = ""
	created.NormalizedEmail = nil
	return created, nil
}

// UpdateUser 更新管理员可修改的用户资料和状态；停用或删除用户时同步提升安全版本并撤销全部会话。
func (s *Service) UpdateUser(ctx context.Context, id string, req *iapiserver.IdentityAdminUserUpdateRequest) (*iapiserver.IdentityUser, error) {
	user, err := s.store.Identities().GetUser(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	statusChanged := req.Status != nil && user.Status != *req.Status
	if req.Status != nil {
		if err := rejectSelfUserStatusChange(ctx, id, *req.Status); err != nil {
			return nil, err
		}
	}
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Alias != nil {
		user.Alias = *req.Alias
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Status != nil {
		user.Status = *req.Status
		if statusChanged && (*req.Status == iapiserver.IdentityUserDisabled || *req.Status == iapiserver.IdentityUserDeleted) {
			user.SecurityVersion++
		}
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		normalized := normalize(email)
		user.Email = &email
		user.NormalizedEmail = &normalized
	}
	updated, err := s.store.Identities().UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	if statusChanged && (user.Status == iapiserver.IdentityUserDisabled || user.Status == iapiserver.IdentityUserDeleted) {
		if err := s.store.Identities().RevokeUserSessions(ctx, id, "USER_STATUS_CHANGED"); err != nil {
			return nil, err
		}
	}
	updated.PasswordHash = ""
	updated.NormalizedEmail = nil
	return updated, nil
}

// SetUserStatus 执行管理员用户状态动作；普通用户不得停用或删除自身，敏感状态变化会撤销全部会话。
func (s *Service) SetUserStatus(ctx context.Context, id, status string) (*iapiserver.IdentityUser, error) {
	user, err := s.store.Identities().GetUser(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	if err := rejectSelfUserStatusChange(ctx, id, status); err != nil {
		return nil, err
	}
	statusChanged := user.Status != status
	user.Status = status
	if status == iapiserver.IdentityUserActive {
		user.LockedUntil = nil
		user.FailedLoginCount = 0
	}
	if statusChanged && (status == iapiserver.IdentityUserDisabled || status == iapiserver.IdentityUserDeleted) {
		user.SecurityVersion++
	}
	updated, err := s.store.Identities().UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	if statusChanged && (status == iapiserver.IdentityUserDisabled || status == iapiserver.IdentityUserDeleted) {
		if err := s.store.Identities().RevokeUserSessions(ctx, id, "USER_STATUS_CHANGED"); err != nil {
			return nil, err
		}
	}
	updated.PasswordHash = ""
	updated.NormalizedEmail = nil
	return updated, nil
}

func rejectSelfUserStatusChange(ctx context.Context, id, status string) error {
	principal, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || principal.PrincipalType != "USER" || principal.PrincipalID != id {
		return nil
	}
	if status == iapiserver.IdentityUserDeleted {
		return errors.NewStatus(code.ErrIdentitySelfDeleteForbidden, "a user cannot delete itself")
	}
	if status == iapiserver.IdentityUserDisabled {
		return errors.NewStatus(code.ErrIdentityUserStateInvalid, "a user cannot disable itself")
	}
	return nil
}

func (s *Service) ListPermissionDefinitions(ctx context.Context, req *iapiserver.IdentityPermissionListRequest) (any, error) {
	items, total, err := s.store.Identities().ListPermissionDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentityPermissionListResponse{Total: total, Items: items}, nil
}

func (s *Service) adminStore() (store.IdentityAdminStore, error) {
	admin, ok := s.store.Identities().(store.IdentityAdminStore)
	if !ok || admin == nil {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity admin store is unavailable")
	}
	return admin, nil
}

func (s *Service) ListRoles(ctx context.Context, req *iapiserver.IdentityRoleListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListRoles(ctx, req)
	return &iapiserver.IdentityRoleListResponse{Total: total, Items: items}, err
}
func (s *Service) GetRole(ctx context.Context, id string) (*iapiserver.IdentityRole, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.GetRole(ctx, id)
}
func (s *Service) CreateRole(ctx context.Context, req *iapiserver.IdentityRoleWriteRequest) (*iapiserver.IdentityRole, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	role := &iapiserver.IdentityRole{ObjectMeta: imachinery.ObjectMeta{Name: req.Name, Description: req.Description}, Code: req.Code, Status: req.Status}
	if role.Status == "" {
		role.Status = "ACTIVE"
	}
	return admin.CreateRole(ctx, role)
}
func (s *Service) UpdateRole(ctx context.Context, id string, req *iapiserver.IdentityRoleWriteRequest) (*iapiserver.IdentityRole, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	role := &iapiserver.IdentityRole{ObjectMeta: imachinery.ObjectMeta{ID: id, Name: req.Name, Description: req.Description}, Code: req.Code, Status: req.Status}
	return admin.UpdateRole(ctx, role)
}
func (s *Service) ReplaceRolePermissions(ctx context.Context, id string, req *iapiserver.IdentityPermissionReplaceRequest) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.ReplaceRolePermissions(ctx, id, req.PermissionCodes); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}

func (s *Service) ListGroups(ctx context.Context, req *iapiserver.IdentityGroupListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListGroups(ctx, req)
	return &iapiserver.IdentityGroupListResponse{Total: total, Items: items}, err
}
func (s *Service) GetGroup(ctx context.Context, id string) (*iapiserver.IdentityGroup, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.GetGroup(ctx, id)
}
func (s *Service) CreateGroup(ctx context.Context, req *iapiserver.IdentityGroupWriteRequest) (*iapiserver.IdentityGroup, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	group := &iapiserver.IdentityGroup{ObjectMeta: imachinery.ObjectMeta{Name: req.Name, Description: req.Description}, Code: req.Code, Status: req.Status}
	if group.Status == "" {
		group.Status = "ACTIVE"
	}
	return admin.CreateGroup(ctx, group)
}
func (s *Service) UpdateGroup(ctx context.Context, id string, req *iapiserver.IdentityGroupWriteRequest) (*iapiserver.IdentityGroup, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	group := &iapiserver.IdentityGroup{ObjectMeta: imachinery.ObjectMeta{ID: id, Name: req.Name, Description: req.Description}, Code: req.Code, Status: req.Status}
	return admin.UpdateGroup(ctx, group)
}
func (s *Service) ReplaceGroupMembers(ctx context.Context, id string, req *iapiserver.IdentityGroupMembersReplaceRequest) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.ReplaceGroupMembers(ctx, id, req.Items); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}
func (s *Service) ReplaceGroupRoles(ctx context.Context, id string, req *iapiserver.IdentityRoleIDsReplaceRequest) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.ReplaceGroupRoles(ctx, id, req.Items); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}

func (s *Service) ListResourceGrants(ctx context.Context, resourceType, resourceID string, req *iapiserver.IdentityResourceGrantListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListResourceGrants(ctx, resourceType, resourceID, req)
	return &iapiserver.IdentityResourceGrantListResponse{Total: total, Items: items}, err
}
func (s *Service) CreateResourceGrant(ctx context.Context, resourceType, resourceID string, req *iapiserver.IdentityResourceGrantCreateRequest) (*iapiserver.IdentityResourceAccessGrant, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "principal context is missing")
	}
	grant := &iapiserver.IdentityResourceAccessGrant{ObjectMeta: imachinery.ObjectMeta{Name: resourceType + ":" + resourceID}, ResourceType: resourceType, ResourceID: resourceID, SubjectType: req.SubjectType, SubjectID: req.SubjectID, AccessLevel: req.AccessLevel, GrantedByPrincipalType: p.PrincipalType, GrantedByPrincipalID: p.PrincipalID, ExpiresAt: req.ExpiresAt}
	return admin.CreateResourceGrant(ctx, grant)
}
func (s *Service) UpdateResourceGrant(ctx context.Context, id string, req *iapiserver.IdentityResourceGrantUpdateRequest) (*iapiserver.IdentityResourceAccessGrant, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	grant := &iapiserver.IdentityResourceAccessGrant{ObjectMeta: imachinery.ObjectMeta{ID: id}, ExpiresAt: req.ExpiresAt}
	if req.AccessLevel != nil {
		grant.AccessLevel = *req.AccessLevel
	}
	return admin.UpdateResourceGrant(ctx, grant)
}
func (s *Service) RevokeResourceGrant(ctx context.Context, id string) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.RevokeResourceGrant(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}

func (s *Service) ListServiceAccounts(ctx context.Context, req *iapiserver.IdentityServiceAccountListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListServiceAccounts(ctx, req)
	return &iapiserver.IdentityServiceAccountListResponse{Total: total, Items: items}, err
}
func (s *Service) GetServiceAccount(ctx context.Context, id string) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.GetServiceAccount(ctx, id)
}
func (s *Service) CreateServiceAccount(ctx context.Context, req *iapiserver.IdentityServiceAccountCreateRequest) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	p, _ := identitymiddleware.PrincipalFromContext(ctx)
	account := &iapiserver.IdentityServiceAccount{ObjectMeta: imachinery.ObjectMeta{Name: req.Name, Description: req.Description}, Code: req.Code, OwnerType: req.OwnerType, OwnerID: req.OwnerID, Status: "ACTIVE", CreatedBy: p.PrincipalID, SecurityVersion: 1, AuthorizationVersion: 1}
	return admin.CreateServiceAccount(ctx, account, req.PermissionCodes)
}
func (s *Service) UpdateServiceAccount(ctx context.Context, id string, req *iapiserver.IdentityServiceAccountUpdateRequest) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	description := ""
	if req.Description != nil {
		description = *req.Description
	}
	account := &iapiserver.IdentityServiceAccount{ObjectMeta: imachinery.ObjectMeta{ID: id, Name: req.Name, Description: description}}
	return admin.UpdateServiceAccount(ctx, account, req.PermissionCodes)
}
func (s *Service) SetServiceAccountStatus(ctx context.Context, id, status string) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.SetServiceAccountStatus(ctx, id, status)
}
func (s *Service) RotateServiceAccountCredential(ctx context.Context, id string) (*iapiserver.IdentityServiceAccountCredentialResponse, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.RotateServiceAccountCredential(ctx, id)
}

func (s *Service) Logout(ctx context.Context) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "session context is missing")
	}
	if err := s.store.Identities().RevokeSession(ctx, p.SessionID, "LOGOUT"); err != nil {
		return nil, err
	}
	_ = s.store.Identities().RevokeSessionRefreshTokens(ctx, p.SessionID, "LOGOUT")
	return &iapiserver.IdentityActionResult{Success: true, Message: "logged out"}, nil
}

func (s *Service) LogoutAll(ctx context.Context) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "principal context is missing")
	}
	if err := s.store.Identities().RevokeUserSessions(ctx, p.PrincipalID, "LOGOUT_ALL"); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true, Message: "all sessions logged out"}, nil
}

// ChangePassword verifies the current password, writes a new Argon2id hash, and revokes all old sessions.
func (s *Service) ChangePassword(ctx context.Context, req *iapiserver.IdentityChangePasswordRequest) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	if req.NewPassword != req.ConfirmPassword {
		return nil, errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password confirmation does not match")
	}
	user, err := s.store.Identities().GetUser(ctx, p.PrincipalID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	valid, verifyErr := verifyPassword(user.PasswordHash, req.OldPassword)
	if verifyErr != nil || !valid {
		return nil, errors.NewStatus(code.ErrIdentityInvalidCredentials, "current password is invalid")
	}
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	if err := validatePassword(req.NewPassword, user.Username, config.PasswordPolicy); err != nil {
		return nil, err
	}
	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password hashing failed")
	}
	user.PasswordHash = hash
	user.PasswordChangedAt = pointerTime(imachinery.Now())
	user.SecurityVersion++
	if _, err := s.store.Identities().UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	if err := s.store.Identities().RevokeUserSessions(ctx, p.PrincipalID, "PASSWORD_CHANGED"); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true, Message: "password changed"}, nil
}

func (s *Service) issueSession(ctx context.Context, user *iapiserver.IdentityUser, clientID, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	session := &iapiserver.IdentityAuthSession{ObjectMeta: imachinery.ObjectMeta{Name: clientID}, UserID: user.ID, ClientID: clientID, IPAddress: ip, UserAgent: userAgent, Status: "ACTIVE", LastActiveAt: pointerTime(imachinery.Now()), ExpiresAt: imachinery.NewTime(time.Now().Add(time.Duration(config.RefreshTokenLifetimeSeconds) * time.Second))}
	created, err := s.store.Identities().CreateSession(ctx, session)
	if err != nil {
		return nil, err
	}
	return s.issueSessionOnExisting(ctx, user, created, clientID, ip, userAgent)
}

func (s *Service) issueSessionOnExisting(ctx context.Context, user *iapiserver.IdentityUser, session *iapiserver.IdentityAuthSession, clientID, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	access, jti, err := identitymiddleware.IssueIdentityAccessToken(s.secret, "USER", user.ID, session.ID, user.SecurityVersion, user.SecurityVersion, time.Duration(config.AccessTokenLifetimeSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	credential := &iapiserver.IdentityTokenCredential{ObjectMeta: imachinery.ObjectMeta{Name: jti}, PrincipalType: "USER", PrincipalID: user.ID, AuthSessionID: session.ID, AccessTokenJTI: jti, SecurityVersion: user.SecurityVersion, CredentialVersion: user.SecurityVersion, Status: "ACTIVE", IssuedAt: imachinery.Now(), ExpiresAt: imachinery.NewTime(time.Now().Add(time.Duration(config.AccessTokenLifetimeSeconds) * time.Second))}
	if err := s.store.Identities().CreateTokenCredential(ctx, credential); err != nil {
		return nil, err
	}
	refresh := identitymiddleware.NewRefreshToken()
	refreshRecord := &iapiserver.IdentityRefreshToken{ObjectMeta: imachinery.ObjectMeta{Name: "refresh"}, SessionID: session.ID, TokenHash: identitymiddleware.HashRefreshToken(refresh), Status: "ACTIVE", IssuedAt: imachinery.Now(), ExpiresAt: imachinery.NewTime(time.Now().Add(time.Duration(config.RefreshTokenLifetimeSeconds) * time.Second))}
	if err := s.store.Identities().CreateRefreshToken(ctx, refreshRecord); err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	return &iapiserver.IdentityAuthUserResponse{User: user, AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: config.AccessTokenLifetimeSeconds, FirstLoginRequired: user.FirstLoginRequired}, nil
}

func normalize(value string) string                      { return strings.ToLower(strings.TrimSpace(value)) }
func pointerTime(value imachinery.Time) *imachinery.Time { return &value }
func redactEmail(email *string) *string {
	if email == nil {
		return nil
	}
	value := *email
	at := strings.IndexByte(value, '@')
	if at <= 1 {
		return email
	}
	masked := value[:1] + "***" + value[at:]
	return &masked
}

func validatePassword(password, username string, policy iapiserver.PlatformPasswordPolicy) error {
	minLength := policy.MinLength
	if minLength == 0 {
		minLength = 8
	}
	if len(password) < minLength || policy.MaxLength > 0 && len(password) > policy.MaxLength {
		return errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password policy failed")
	}
	if policy.RequireLowercase && !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") {
		return errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password policy failed")
	}
	if policy.RequireUppercase && !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password policy failed")
	}
	if policy.RequireDigit && !strings.ContainsAny(password, "0123456789") {
		return errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password policy failed")
	}
	if policy.RequireSpecialCharacter && !strings.ContainsAny(password, "!@#$%^&*()-_=+[]{};:,.?/") {
		return errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password policy failed")
	}
	if policy.DisallowUsername && username != "" && strings.Contains(strings.ToLower(password), strings.ToLower(username)) {
		return errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password policy failed")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, nil
	}
	var memory, iterations, parallelism uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, err
	}
	if memory < 8*1024 || memory > 256*1024 || iterations == 0 || iterations > 10 || parallelism == 0 || parallelism > 16 {
		return false, errors.New("argon2id parameters are outside supported bounds")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	if len(salt) < 8 || len(salt) > 64 {
		return false, errors.New("argon2id salt length is outside supported bounds")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	if len(expected) < 16 || len(expected) > 64 {
		return false, errors.New("argon2id key length is outside supported bounds")
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
