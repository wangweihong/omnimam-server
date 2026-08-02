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
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

const (
	IdentityPrincipalContextKey   = "identity.principal"
	IdentityTokenClaimsContextKey = "identity.token_claims"
	IdentityAuditSourceDomain     = "identity"
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

// IdentityAuthentication validates the v1.11 JWT, JTI credential, session and current user state on every request.
func IdentityAuthentication(authOptions *options.AuthOptions, mode string, identity store.IdentityV11Store) gin.HandlerFunc {
	_ = mode
	secret := []byte("dfVpOK8LZeJLZHYmHdb1VdyRrACKpqoo")
	if authOptions != nil && authOptions.JWTSecret != "" {
		secret = []byte(authOptions.JWTSecret)
	}
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
		principal, user, err := resolveIdentityPrincipal(c.Request.Context(), c, identity, secret)
		if err != nil {
			core.WriteResponse(c, err, nil)
			c.Abort()
			return
		}
		c.Set(IdentityPrincipalContextKey, principal)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), IdentityPrincipalContextKey, principal))
		if user != nil {
			legacy := &iapiserver.User{}
			legacy.ID, legacy.Name, legacy.Mail, legacy.Phone = user.ID, user.Username, valueOrEmpty(user.Email), user.Phone
			SetUserContext(c, legacy)
		}
		c.Next()
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
	case "/api/v1/iam/auth/register", "/api/v1/iam/auth/login", "/api/v1/iam/auth/refresh":
		return true
	default:
		return false
	}
}

func resolveIdentityPrincipal(ctx context.Context, c *gin.Context, identity store.IdentityV11Store, secret []byte) (IdentityPrincipal, *iapiserver.IdentityUser, error) {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	if raw == "" {
		if token, err := c.Cookie(iapiserver.CookieKeyToken); err == nil {
			raw = "Bearer " + token
		}
	}
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
	c.Set(IdentityTokenClaimsContextKey, claims)
	return principal, user, nil
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
		action := c.Request.Method + " " + c.FullPath()
		if strings.HasSuffix(action, " ") {
			action = c.Request.Method + " " + c.Request.URL.Path
		}
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
	record := &iapiserver.PlatformAuditLog{SourceDomain: sourceDomain, SourceModule: "middleware", PrincipalType: principal.PrincipalType, PrincipalID: principal.PrincipalID, ActorUserID: principal.ActorUserID, Action: action, Result: result, ReasonCode: reason, RequestID: requestID, IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(), IdempotencyKey: hex.EncodeToString(idempotency[:]), Detail: []byte(`{}`)}
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
