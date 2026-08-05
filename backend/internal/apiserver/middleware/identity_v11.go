package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/platformaudit"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

const (
	IdentityPrincipalContextKey = "identity.principal"
	IdentityAuditSourceDomain   = "identity"
)

// IdentityPrincipal is the verified one-hop context passed from middleware to handlers and services.
type IdentityPrincipal struct {
	PrincipalType        string
	PrincipalID          string
	ActorUserID          string
	SessionID            string
	CredentialVersion    int64
	AuthorizationVersion int64
	Permissions          map[string]struct{}
}

type IdentityTokenClaims struct {
	jwt.RegisteredClaims
	PrincipalType     string `json:"principal_type"`
	SessionID         string `json:"session_id"`
	SecurityVersion   int64  `json:"security_version"`
	CredentialVersion int64  `json:"credential_version"`
}

// IdentityAuthentication 在每个请求中校验 JWT、JTI 凭据、会话和当前用户状态。
func IdentityAuthentication(authOptions *options.AuthOptions, identity store.IdentityStore) gin.HandlerFunc {
	secret := IdentityJWTSecret(authOptions)
	return func(c *gin.Context) {
		if isIdentityPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		if identity == nil {
			core.WriteResponse(c, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity store is not configured"), nil)
			c.Abort()
			return
		}
		principal, user, err := ResolveIdentityBearer(c.Request.Context(), bearerAuthorization(c), identity, secret)
		if err != nil {
			core.WriteResponse(c, err, nil)
			c.Abort()
			return
		}
		SetIdentityContext(c, principal, user)
		c.Next()
	}
}

// IdentityJWTSecret 返回 Identity JWT 的当前签名密钥，并保持与 service 默认配置一致。
func IdentityJWTSecret(authOptions *options.AuthOptions) []byte {
	if authOptions == nil {
		return nil
	}
	return []byte(authOptions.JWTSecret)
}

// IdentityAuthenticationForAPIs 为旧业务路径启用新 Identity 认证，并让 IAM/Platform 路径保留审计优先的局部 middleware 链。
func IdentityAuthenticationForAPIs(authOptions *options.AuthOptions, identity store.IdentityStore) gin.HandlerFunc {
	authenticate := IdentityAuthentication(authOptions, identity)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/iam/") || strings.HasPrefix(path, "/api/v1/platform/") {
			c.Next()
			return
		}
		authenticate(c)
	}
}

// PrincipalFromContext returns the middleware-verified principal for service-layer authorization and ownership checks.
func PrincipalFromContext(ctx context.Context) (IdentityPrincipal, bool) {
	value := ctx.Value(IdentityPrincipalContextKey)
	principal, ok := value.(IdentityPrincipal)
	return principal, ok
}

func isIdentityPublicPath(path string) bool {
	switch path {
	case "/api/v1/iam/auth/register/start", "/api/v1/iam/auth/register/finish", "/api/v1/iam/auth/login/start", "/api/v1/iam/auth/login/finish", "/api/v1/iam/auth/refresh":
		return true
	default:
		return false
	}
}

// ResolveIdentityBearer 校验 Bearer JWT 及其 credential、session、用户状态和动态权限投影。
func ResolveIdentityBearer(ctx context.Context, raw string, identity store.IdentityStore, secret []byte) (IdentityPrincipal, *iapiserver.IdentityUser, error) {
	if len(secret) < 32 {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity JWT secret is not configured")
	}
	raw = strings.TrimSpace(raw)
	parts := strings.Fields(raw)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrMissingHeader, "a Bearer Identity JWT is required")
	}
	claims := &IdentityTokenClaims{}
	token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return secret, nil
	})
	if err != nil || token == nil || !token.Valid {
		if validationErr, ok := err.(*jwt.ValidationError); ok && validationErr.Errors&jwt.ValidationErrorExpired != 0 {
			return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityTokenExpired, "access token expired")
		}
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrTokenInvalid, "identity token is invalid")
	}
	if claims.Subject == "" || claims.ID == "" || claims.SessionID == "" || claims.PrincipalType == "" {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrTokenInvalid, "identity token claims are incomplete")
	}
	credential, err := identity.GetTokenCredentialByJTI(ctx, claims.ID)
	if err != nil || credential.Status != "ACTIVE" || credential.ExpiresAt.Time.Before(time.Now()) {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "access token credential is revoked")
	}
	session, err := identity.GetSession(ctx, claims.SessionID)
	if err != nil || session.Status != "ACTIVE" || session.ExpiresAt.Time.Before(time.Now()) {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "identity session is revoked")
	}
	principal := IdentityPrincipal{PrincipalType: claims.PrincipalType, PrincipalID: claims.Subject, SessionID: claims.SessionID, CredentialVersion: claims.CredentialVersion}
	var user *iapiserver.IdentityUser
	if claims.PrincipalType == "USER" {
		user, err = identity.GetUser(ctx, claims.Subject)
		if err != nil || user.Status != iapiserver.IdentityUserActive {
			return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityAccountDisabled, "identity user is not active")
		}
		if claims.SecurityVersion != user.SecurityVersion || credential.SecurityVersion != user.SecurityVersion {
			return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityTokenRevoked, "identity security version changed")
		}
		principal.AuthorizationVersion = user.AuthorizationVersion
	} else if claims.PrincipalType != "SERVICE_ACCOUNT" {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityPrincipalContextInvalid, "identity principal type is invalid")
	}
	permissionCodes, version, err := identity.PermissionCodes(ctx, claims.PrincipalType, claims.Subject)
	if err != nil {
		return IdentityPrincipal{}, nil, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity authorization context is unavailable")
	}
	principal.AuthorizationVersion = version
	principal.Permissions = make(map[string]struct{}, len(permissionCodes))
	for _, permission := range permissionCodes {
		principal.Permissions[permission] = struct{}{}
	}
	return principal, user, nil
}

func bearerAuthorization(c *gin.Context) string {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	if raw == "" {
		if token, err := c.Cookie(iapiserver.CookieKeyToken); err == nil {
			return "Bearer " + token
		}
	}
	return raw
}

// SetIdentityContext 将已校验的主体写入 Gin 和 request context，并提供旧业务 service 仍消费的用户摘要。
func SetIdentityContext(c *gin.Context, principal IdentityPrincipal, user *iapiserver.IdentityUser) {
	c.Set(IdentityPrincipalContextKey, principal)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), IdentityPrincipalContextKey, principal))
	if user == nil {
		return
	}
	legacy := &iapiserver.User{}
	legacy.ID, legacy.Name, legacy.Mail, legacy.Phone = user.ID, user.Username, valueOrEmpty(user.Email), user.Phone
	SetUserContext(c, legacy)
}

// SetUserContext 注入已校验用户摘要，兼容尚未迁移到 PrincipalContext 的业务 service。
func SetUserContext(c *gin.Context, user *iapiserver.User) {
	c.Set(iapiserver.GinContextKeyUser, user)
	ctx := context.WithValue(c.Request.Context(), iapiserver.GinContextKeyUser, user)
	c.Request = c.Request.WithContext(ctx)
}

// RequireIdentityPermission enforces a stable v1.11 permission code after authentication middleware.
func RequireIdentityPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(IdentityPrincipalContextKey)
		principal, valid := value.(IdentityPrincipal)
		if !ok || !valid {
			core.WriteResponse(c, errors.NewStatus(code.ErrIdentityAuthzContextInvalid, "identity principal context is missing"), nil)
			c.Abort()
			return
		}
		if _, allowed := principal.Permissions[permission]; !allowed {
			if _, wildcard := principal.Permissions["*"]; !wildcard {
				core.WriteResponse(c, errors.NewStatus(code.ErrIdentityAuthzDenied, "permission denied"), nil)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// RequireServicePrincipal restricts platform internal boundaries to verified service principals.
func RequireServicePrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(IdentityPrincipalContextKey)
		principal, valid := value.(IdentityPrincipal)
		if !ok || !valid || principal.PrincipalType != "SERVICE_ACCOUNT" {
			core.WriteResponse(c, errors.NewStatus(code.ErrIdentityAuthzDenied, "service principal is required"), nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// Audit records request authorization and completion without serializing request bodies, credentials or secrets.
func Audit(platform store.PlatformManagementStore, sourceDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		principal := IdentityPrincipal{PrincipalType: "ANONYMOUS"}
		if value, ok := c.Get(IdentityPrincipalContextKey); ok {
			if p, valid := value.(IdentityPrincipal); valid {
				principal = p
			}
		}
		action := sourceDomain + ".http." + strings.ToLower(c.Request.Method)
		if isSensitiveRequest(c.Request.Method, c.Request.URL.Path) {
			if platform == nil {
				core.WriteResponse(c, errors.NewStatus(code.ErrPlatformAuditWriteUnavailable, "audit boundary is unavailable"), nil)
				c.Abort()
				return
			}
			if _, err := appendMiddlewareAudit(c, platform, sourceDomain, action+".authorize", "SUCCESS", "", requestID, principal); err != nil {
				core.WriteResponse(c, errors.NewStatus(code.ErrPlatformAuditWriteUnavailable, "audit boundary is unavailable"), nil)
				c.Abort()
				return
			}
		}
		c.Next()
		if platform == nil {
			return
		}
		if value, ok := c.Get(IdentityPrincipalContextKey); ok {
			if p, valid := value.(IdentityPrincipal); valid {
				principal = p
			}
		}
		result := "SUCCESS"
		if c.Writer.Status() >= 400 {
			result = "FAILED"
		}
		_, _ = appendMiddlewareAudit(c, platform, sourceDomain, action, result, "", requestID, principal)
	}
}

func isSensitiveRequest(method, path string) bool {
	return method != "GET" || strings.Contains(path, "/auth/") || strings.Contains(path, "/admin/") || strings.Contains(path, "/internal/")
}

func appendMiddlewareAudit(c *gin.Context, platform store.PlatformManagementStore, sourceDomain, action, result, reason, requestID string, principal IdentityPrincipal) (*iapiserver.PlatformAuditLog, error) {
	idempotency := sha256.Sum256([]byte(requestID + ":" + action + ":" + result))
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	record := &iapiserver.PlatformAuditLog{SourceDomain: sourceDomain, SourceModule: "middleware", PrincipalType: principal.PrincipalType, PrincipalID: principal.PrincipalID, ActorUserID: principal.ActorUserID, Action: action, TargetType: "http_route", TargetID: route, Result: result, ReasonCode: reason, RequestID: requestID, IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), OccurredAt: imachinery.Now(), IdempotencyKey: hex.EncodeToString(idempotency[:]), Detail: []byte(`{}`)}
	fingerprint, err := platformaudit.CanonicalJSONDigest(iapiserver.PlatformAuditRecordRequest{SourceDomain: record.SourceDomain, SourceModule: record.SourceModule, PrincipalType: record.PrincipalType, PrincipalID: record.PrincipalID, ActorUserID: record.ActorUserID, Action: record.Action, TargetType: record.TargetType, TargetID: record.TargetID, Result: record.Result, ReasonCode: record.ReasonCode, RequestID: record.RequestID, IPAddress: record.IPAddress, UserAgent: record.UserAgent, OccurredAt: record.OccurredAt, Detail: record.Detail, IdempotencyKey: record.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	record.ContentFingerprint = fingerprint
	return platform.AppendAuditLog(c.Request.Context(), record)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// IssueIdentityAccessToken signs an access token whose JTI is checked against the credential store by middleware.
func IssueIdentityAccessToken(secret []byte, principalType, principalID, sessionID string, securityVersion, credentialVersion int64, lifetime time.Duration) (string, string, error) {
	if len(secret) < 32 {
		return "", "", errors.New("identity JWT secret must be at least 32 bytes")
	}
	jti := uuid.NewString()
	now := time.Now()
	claims := IdentityTokenClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: principalID, ID: jti, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(lifetime))}, PrincipalType: principalType, SessionID: sessionID, SecurityVersion: securityVersion, CredentialVersion: credentialVersion}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	return token, jti, err
}

func HashRefreshToken(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
func NewRefreshToken() string { return uuid.NewString() + "." + uuid.NewString() }
