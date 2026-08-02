package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

// V11Service implements the released local Identity contract and delegates persistence to consumer-side store interfaces.
type V11Service struct {
	identity store.IdentityV11Store
	platform store.PlatformManagementStore
	secret   []byte
}

func NewV11Service(identity store.IdentityV11Store, platform store.PlatformManagementStore, secret string) *V11Service {
	if secret == "" {
		secret = "dfVpOK8LZeJLZHYmHdb1VdyRrACKpqoo"
	}
	return &V11Service{identity: identity, platform: platform, secret: []byte(secret)}
}

func DefaultPermissions() []*iapiserver.IdentityPermissionDefinition {
	items := []struct{ code, domain, resource, action string }{
		{"identity.user.read", "identity", "user", "read"}, {"identity.user.manage", "identity", "user", "manage"},
		{"identity.auth.session", "identity", "auth", "session"}, {"identity.permission.read", "identity", "permission", "read"},
		{"identity.role.manage", "identity", "role", "manage"}, {"identity.group.manage", "identity", "group", "manage"},
		{"identity.resource_grant.read", "identity", "resource_grant", "read"}, {"identity.resource_grant.manage", "identity", "resource_grant", "manage"},
		{"identity.service_account.read", "identity", "service_account", "read"}, {"identity.service_account.manage", "identity", "service_account", "manage"},
		{"identity.authz.check", "identity", "authz", "check"}, {"platform.overview.read", "platform-management", "overview", "read"},
		{"platform.auth_config.read", "platform-management", "auth_config", "read"}, {"platform.auth_config.manage", "platform-management", "auth_config", "manage"},
		{"platform.auth_config.read_internal", "platform-management", "auth_config", "read_internal"}, {"platform.audit.read", "platform-management", "audit", "read"},
		{"platform.audit.record", "platform-management", "audit", "record"},
	}
	result := make([]*iapiserver.IdentityPermissionDefinition, 0, len(items))
	for _, item := range items {
		result = append(result, &iapiserver.IdentityPermissionDefinition{ObjectMeta: imachinery.ObjectMeta{Name: item.code}, Code: item.code, Domain: item.domain, Resource: item.resource, Action: item.action, RiskLevel: "NORMAL", Status: "ACTIVE"})
	}
	return result
}

func (s *V11Service) Register(ctx context.Context, req *iapiserver.IdentityRegisterRequest) (*iapiserver.IdentityAuthUserResponse, error) {
	if s.identity == nil || s.platform == nil {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity dependencies are unavailable")
	}
	config, err := s.platform.GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	if config.RegistrationMode != "OPEN" {
		return nil, errors.NewStatus(code.ErrIdentityUserStateInvalid, "registration is not open")
	}
	if err := validatePassword(req.Password, config.PasswordPolicy); err != nil {
		return nil, err
	}
	normalizedUsername := normalize(req.Username)
	normalizedEmail := normalize(req.Email)
	if existing, lookupErr := s.identity.GetUserByLogin(ctx, normalizedUsername); lookupErr == nil && existing != nil && existing.ID != "" {
		return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username already exists")
	}
	if existing, lookupErr := s.identity.GetUserByLogin(ctx, normalizedEmail); lookupErr == nil && existing != nil && existing.ID != "" {
		return nil, errors.NewStatus(code.ErrIdentityEmailAlreadyExists, "email already exists")
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password hashing failed")
	}
	email := strings.TrimSpace(req.Email)
	user := &iapiserver.IdentityUser{ObjectMeta: imachinery.ObjectMeta{Name: req.Username}, Username: strings.TrimSpace(req.Username), NormalizedUsername: normalizedUsername, DisplayName: strings.TrimSpace(req.DisplayName), Email: &email, NormalizedEmail: &normalizedEmail, PasswordHash: hash, Status: iapiserver.IdentityUserActive, SecurityVersion: 1, AuthorizationVersion: 1, PasswordChangedAt: pointerTime(imachinery.Now())}
	created, err := s.identity.CreateUser(ctx, user)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
		}
		return nil, err
	}
	return s.issueSession(ctx, created, "register", "", "")
}

func (s *V11Service) Login(ctx context.Context, req *iapiserver.IdentityLoginRequest, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	user, err := s.identity.GetUserByLogin(ctx, normalize(req.Login))
	if err != nil || user == nil {
		return nil, errors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	if user.Status == iapiserver.IdentityUserDisabled || user.Status == iapiserver.IdentityUserDeleted {
		return nil, errors.NewStatus(code.ErrIdentityAccountDisabled, "account is disabled")
	}
	if user.Status == iapiserver.IdentityUserLocked && user.LockedUntil != nil && user.LockedUntil.Time.After(time.Now()) {
		return nil, errors.NewStatus(code.ErrIdentityAccountLocked, "account is locked")
	}
	valid, verifyErr := verifyPassword(user.PasswordHash, req.Password)
	if verifyErr != nil || !valid {
		return nil, errors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	user.FailedLoginCount = 0
	user.LastLoginAt = pointerTime(imachinery.Now())
	user.Status = iapiserver.IdentityUserActive
	updated, err := s.identity.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, updated, "login", ip, userAgent)
}

func (s *V11Service) Refresh(ctx context.Context, req *iapiserver.IdentityRefreshRequest) (*iapiserver.IdentityAuthUserResponse, error) {
	refresh, err := s.identity.GetRefreshTokenByHash(ctx, identitymiddleware.HashRefreshToken(req.RefreshToken))
	if err != nil || refresh == nil {
		return nil, errors.NewStatus(code.ErrIdentityRefreshTokenInvalid, "refresh token is invalid")
	}
	if refresh.Status != "ACTIVE" || refresh.ExpiresAt.Time.Before(time.Now()) {
		if refresh.Status == "USED" {
			_ = s.identity.RevokeSession(ctx, refresh.SessionID, "TOKEN_REUSE")
			_ = s.identity.RevokeSessionRefreshTokens(ctx, refresh.SessionID, "TOKEN_REUSE")
			return nil, errors.NewStatus(code.ErrIdentityRefreshTokenReused, "refresh token reuse detected")
		}
		return nil, errors.NewStatus(code.ErrIdentityRefreshTokenInvalid, "refresh token is invalid")
	}
	if err := s.identity.MarkRefreshTokenUsed(ctx, refresh.ID); err != nil {
		_ = s.identity.RevokeSession(ctx, refresh.SessionID, "TOKEN_REUSE")
		_ = s.identity.RevokeSessionRefreshTokens(ctx, refresh.SessionID, "TOKEN_REUSE")
		return nil, errors.NewStatus(code.ErrIdentityRefreshTokenReused, "refresh token reuse detected")
	}
	session, err := s.identity.GetSession(ctx, refresh.SessionID)
	if err != nil || session.Status != "ACTIVE" {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "session is revoked")
	}
	user, err := s.identity.GetUser(ctx, session.UserID)
	if err != nil || user.Status != iapiserver.IdentityUserActive {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "user is not active")
	}
	return s.issueSessionOnExisting(ctx, user, session, "refresh", "", "")
}

func (s *V11Service) Me(ctx context.Context) (*iapiserver.IdentityUser, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity principal context is missing")
	}
	user, err := s.identity.GetUser(ctx, p.PrincipalID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	user.Email = redactEmail(user.Email)
	return user, nil
}

func (s *V11Service) Heartbeat(ctx context.Context) (*iapiserver.IdentityPresenceHeartbeatResponse, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.SessionID == "" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "session context is missing")
	}
	session, err := s.identity.TouchSession(ctx, p.SessionID, imachinery.Now())
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "session is revoked")
	}
	return &iapiserver.IdentityPresenceHeartbeatResponse{Online: true, LastActiveAt: *session.LastActiveAt}, nil
}

func (s *V11Service) Sessions(ctx context.Context, req *iapiserver.IdentitySessionListRequest) (*iapiserver.IdentitySessionListResponse, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	items, total, err := s.identity.ListSessions(ctx, p.PrincipalID, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentitySessionListResponse{Total: total, Items: items}, nil
}

func (s *V11Service) RevokeSession(ctx context.Context, id string) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	session, err := s.identity.GetSession(ctx, id)
	if err != nil || session.UserID != p.PrincipalID {
		return nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "session is not visible")
	}
	if err := s.identity.RevokeSession(ctx, id, "LOGOUT"); err != nil {
		return nil, err
	}
	_ = s.identity.RevokeSessionRefreshTokens(ctx, id, "LOGOUT")
	return &iapiserver.IdentityActionResult{Success: true, Message: "session revoked"}, nil
}

func (s *V11Service) Permissions(ctx context.Context) (*iapiserver.IdentityPermissionProjection, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "principal context is missing")
	}
	codes, version, err := s.identity.PermissionCodes(ctx, p.PrincipalType, p.PrincipalID)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentityPermissionProjection{PrincipalType: p.PrincipalType, PrincipalID: p.PrincipalID, ActorUserID: p.ActorUserID, AuthorizationVersion: version, PermissionCodes: codes}, nil
}

func (s *V11Service) ListUsers(ctx context.Context, req *iapiserver.IdentityUserListRequest) (any, error) {
	items, total, err := s.identity.ListUsers(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.PasswordHash = ""
		item.NormalizedEmail = nil
	}
	return &iapiserver.IdentityUserListResponse{Total: total, Items: items}, nil
}

func (s *V11Service) GetUser(ctx context.Context, id string) (*iapiserver.IdentityUser, error) {
	user, err := s.identity.GetUser(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	return user, nil
}

func (s *V11Service) AdminCreateUser(ctx context.Context, req *iapiserver.IdentityAdminUserCreateRequest) (*iapiserver.IdentityUser, error) {
	config, err := s.platform.GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	password := req.InitialPassword
	if password == "" {
		password = "ChangeMe!123"
	}
	if err := validatePassword(password, config.PasswordPolicy); err != nil {
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
	created, err := s.identity.CreateUser(ctx, user)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
	}
	created.PasswordHash = ""
	created.NormalizedEmail = nil
	return created, nil
}

func (s *V11Service) UpdateUser(ctx context.Context, id string, req *iapiserver.IdentityAdminUserUpdateRequest) (*iapiserver.IdentityUser, error) {
	user, err := s.identity.GetUser(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
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
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		normalized := normalize(email)
		user.Email = &email
		user.NormalizedEmail = &normalized
	}
	updated, err := s.identity.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	updated.PasswordHash = ""
	updated.NormalizedEmail = nil
	return updated, nil
}

func (s *V11Service) SetUserStatus(ctx context.Context, id, status string) (*iapiserver.IdentityUser, error) {
	user, err := s.identity.GetUser(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.Status = status
	if status == iapiserver.IdentityUserActive {
		user.LockedUntil = nil
		user.FailedLoginCount = 0
	}
	updated, err := s.identity.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	updated.PasswordHash = ""
	updated.NormalizedEmail = nil
	return updated, nil
}

func (s *V11Service) ListPermissionDefinitions(ctx context.Context, req *iapiserver.IdentityPermissionListRequest) (any, error) {
	items, total, err := s.identity.ListPermissionDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentityPermissionListResponse{Total: total, Items: items}, nil
}

func (s *V11Service) adminStore() (store.IdentityAdminStore, error) {
	admin, ok := s.identity.(store.IdentityAdminStore)
	if !ok || admin == nil {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity admin store is unavailable")
	}
	return admin, nil
}

func (s *V11Service) ListRoles(ctx context.Context, req *iapiserver.IdentityRoleListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListRoles(ctx, req)
	return &iapiserver.IdentityRoleListResponse{Total: total, Items: items}, err
}
func (s *V11Service) GetRole(ctx context.Context, id string) (*iapiserver.IdentityRole, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.GetRole(ctx, id)
}
func (s *V11Service) CreateRole(ctx context.Context, req *iapiserver.IdentityRoleWriteRequest) (*iapiserver.IdentityRole, error) {
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
func (s *V11Service) UpdateRole(ctx context.Context, id string, req *iapiserver.IdentityRoleWriteRequest) (*iapiserver.IdentityRole, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	role := &iapiserver.IdentityRole{ObjectMeta: imachinery.ObjectMeta{ID: id, Name: req.Name, Description: req.Description}, Code: req.Code, Status: req.Status}
	return admin.UpdateRole(ctx, role)
}
func (s *V11Service) ReplaceRolePermissions(ctx context.Context, id string, req *iapiserver.IdentityPermissionReplaceRequest) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.ReplaceRolePermissions(ctx, id, req.PermissionCodes); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}

func (s *V11Service) ListGroups(ctx context.Context, req *iapiserver.IdentityGroupListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListGroups(ctx, req)
	return &iapiserver.IdentityGroupListResponse{Total: total, Items: items}, err
}
func (s *V11Service) GetGroup(ctx context.Context, id string) (*iapiserver.IdentityGroup, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.GetGroup(ctx, id)
}
func (s *V11Service) CreateGroup(ctx context.Context, req *iapiserver.IdentityGroupWriteRequest) (*iapiserver.IdentityGroup, error) {
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
func (s *V11Service) UpdateGroup(ctx context.Context, id string, req *iapiserver.IdentityGroupWriteRequest) (*iapiserver.IdentityGroup, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	group := &iapiserver.IdentityGroup{ObjectMeta: imachinery.ObjectMeta{ID: id, Name: req.Name, Description: req.Description}, Code: req.Code, Status: req.Status}
	return admin.UpdateGroup(ctx, group)
}
func (s *V11Service) ReplaceGroupMembers(ctx context.Context, id string, req *iapiserver.IdentityGroupMembersReplaceRequest) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.ReplaceGroupMembers(ctx, id, req.Items); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}
func (s *V11Service) ReplaceGroupRoles(ctx context.Context, id string, req *iapiserver.IdentityRoleIDsReplaceRequest) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.ReplaceGroupRoles(ctx, id, req.Items); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}

func (s *V11Service) ListResourceGrants(ctx context.Context, resourceType, resourceID string, req *iapiserver.IdentityResourceGrantListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListResourceGrants(ctx, resourceType, resourceID, req)
	return &iapiserver.IdentityResourceGrantListResponse{Total: total, Items: items}, err
}
func (s *V11Service) CreateResourceGrant(ctx context.Context, resourceType, resourceID string, req *iapiserver.IdentityResourceGrantCreateRequest) (*iapiserver.IdentityResourceAccessGrant, error) {
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
func (s *V11Service) UpdateResourceGrant(ctx context.Context, id string, req *iapiserver.IdentityResourceGrantUpdateRequest) (*iapiserver.IdentityResourceAccessGrant, error) {
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
func (s *V11Service) RevokeResourceGrant(ctx context.Context, id string) (*iapiserver.IdentityActionResult, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	if err := admin.RevokeResourceGrant(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true}, nil
}

func (s *V11Service) ListServiceAccounts(ctx context.Context, req *iapiserver.IdentityServiceAccountListRequest) (any, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	items, total, err := admin.ListServiceAccounts(ctx, req)
	return &iapiserver.IdentityServiceAccountListResponse{Total: total, Items: items}, err
}
func (s *V11Service) GetServiceAccount(ctx context.Context, id string) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.GetServiceAccount(ctx, id)
}
func (s *V11Service) CreateServiceAccount(ctx context.Context, req *iapiserver.IdentityServiceAccountCreateRequest) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	p, _ := identitymiddleware.PrincipalFromContext(ctx)
	account := &iapiserver.IdentityServiceAccount{ObjectMeta: imachinery.ObjectMeta{Name: req.Name, Description: req.Description}, Code: req.Code, OwnerType: req.OwnerType, OwnerID: req.OwnerID, Status: "ACTIVE", CreatedBy: p.PrincipalID, SecurityVersion: 1, AuthorizationVersion: 1}
	return admin.CreateServiceAccount(ctx, account, req.PermissionCodes)
}
func (s *V11Service) UpdateServiceAccount(ctx context.Context, id string, req *iapiserver.IdentityServiceAccountUpdateRequest) (*iapiserver.IdentityServiceAccount, error) {
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
func (s *V11Service) SetServiceAccountStatus(ctx context.Context, id, status string) (*iapiserver.IdentityServiceAccount, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.SetServiceAccountStatus(ctx, id, status)
}
func (s *V11Service) RotateServiceAccountCredential(ctx context.Context, id string) (*iapiserver.IdentityServiceAccountCredentialResponse, error) {
	admin, err := s.adminStore()
	if err != nil {
		return nil, err
	}
	return admin.RotateServiceAccountCredential(ctx, id)
}

func (s *V11Service) Logout(ctx context.Context) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "session context is missing")
	}
	if err := s.identity.RevokeSession(ctx, p.SessionID, "LOGOUT"); err != nil {
		return nil, err
	}
	_ = s.identity.RevokeSessionRefreshTokens(ctx, p.SessionID, "LOGOUT")
	return &iapiserver.IdentityActionResult{Success: true, Message: "logged out"}, nil
}

func (s *V11Service) LogoutAll(ctx context.Context) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "principal context is missing")
	}
	if err := s.identity.RevokeUserSessions(ctx, p.PrincipalID, "LOGOUT_ALL"); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true, Message: "all sessions logged out"}, nil
}

// ChangePassword verifies the current password, writes a new Argon2id hash, and revokes all old sessions.
func (s *V11Service) ChangePassword(ctx context.Context, req *iapiserver.IdentityChangePasswordRequest) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	if req.NewPassword != req.ConfirmPassword {
		return nil, errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password confirmation does not match")
	}
	user, err := s.identity.GetUser(ctx, p.PrincipalID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	valid, verifyErr := verifyPassword(user.PasswordHash, req.OldPassword)
	if verifyErr != nil || !valid {
		return nil, errors.NewStatus(code.ErrIdentityInvalidCredentials, "current password is invalid")
	}
	config, err := s.platform.GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	if err := validatePassword(req.NewPassword, config.PasswordPolicy); err != nil {
		return nil, err
	}
	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		return nil, errors.NewStatus(code.ErrIdentityPasswordPolicyFailed, "password hashing failed")
	}
	user.PasswordHash = hash
	user.PasswordChangedAt = pointerTime(imachinery.Now())
	user.SecurityVersion++
	if _, err := s.identity.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	if err := s.identity.RevokeUserSessions(ctx, p.PrincipalID, "PASSWORD_CHANGED"); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true, Message: "password changed"}, nil
}

func (s *V11Service) issueSession(ctx context.Context, user *iapiserver.IdentityUser, clientID, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	config, err := s.platform.GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	session := &iapiserver.IdentityAuthSession{ObjectMeta: imachinery.ObjectMeta{Name: clientID}, UserID: user.ID, ClientID: clientID, IPAddress: ip, UserAgent: userAgent, Status: "ACTIVE", LastActiveAt: pointerTime(imachinery.Now()), ExpiresAt: imachinery.NewTime(time.Now().Add(time.Duration(config.RefreshTokenLifetimeSeconds) * time.Second))}
	created, err := s.identity.CreateSession(ctx, session)
	if err != nil {
		return nil, err
	}
	return s.issueSessionOnExisting(ctx, user, created, clientID, ip, userAgent)
}

func (s *V11Service) issueSessionOnExisting(ctx context.Context, user *iapiserver.IdentityUser, session *iapiserver.IdentityAuthSession, clientID, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	config, err := s.platform.GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	access, jti, err := identitymiddleware.IssueIdentityAccessToken(s.secret, "USER", user.ID, session.ID, user.SecurityVersion, user.SecurityVersion, time.Duration(config.AccessTokenLifetimeSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	credential := &iapiserver.IdentityTokenCredential{ObjectMeta: imachinery.ObjectMeta{Name: jti}, PrincipalType: "USER", PrincipalID: user.ID, AuthSessionID: session.ID, AccessTokenJTI: jti, SecurityVersion: user.SecurityVersion, CredentialVersion: user.SecurityVersion, Status: "ACTIVE", IssuedAt: imachinery.Now(), ExpiresAt: imachinery.NewTime(time.Now().Add(time.Duration(config.AccessTokenLifetimeSeconds) * time.Second))}
	if err := s.identity.CreateTokenCredential(ctx, credential); err != nil {
		return nil, err
	}
	refresh := identitymiddleware.NewRefreshToken()
	refreshRecord := &iapiserver.IdentityRefreshToken{ObjectMeta: imachinery.ObjectMeta{Name: "refresh"}, SessionID: session.ID, TokenHash: identitymiddleware.HashRefreshToken(refresh), Status: "ACTIVE", IssuedAt: imachinery.Now(), ExpiresAt: imachinery.NewTime(time.Now().Add(time.Duration(config.RefreshTokenLifetimeSeconds) * time.Second))}
	if err := s.identity.CreateRefreshToken(ctx, refreshRecord); err != nil {
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

func validatePassword(password string, policy json.RawMessage) error {
	var values struct {
		MinLength     int  `json:"min_length"`
		RequireLower  bool `json:"require_lower"`
		RequireDigit  bool `json:"require_digit"`
		RequireUpper  bool `json:"require_upper"`
		RequireSymbol bool `json:"require_symbol"`
	}
	_ = json.Unmarshal(policy, &values)
	if values.MinLength == 0 {
		values.MinLength = 8
	}
	if len(password) < values.MinLength || (values.RequireLower && !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")) || (values.RequireUpper && !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")) || (values.RequireDigit && !strings.ContainsAny(password, "0123456789")) || (values.RequireSymbol && !strings.ContainsAny(password, "!@#$%^&*()-_=+[]{};:,.?/")) {
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
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expected)))
	return string(actual) == string(expected), nil
}
