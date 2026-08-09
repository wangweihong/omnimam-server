package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestResolveIdentityBearerAcceptsScopedAgentWorkloadWithoutIdentityRows(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token, err := IssueAgentWorkloadAccessToken(secret, "agent-1", 2, "app-1", "runtime-1", "grant-1", time.Minute)
	if err != nil {
		t.Fatalf("issue workload token: %v", err)
	}
	principal, user, err := ResolveIdentityBearer(context.Background(), "Bearer "+token, nil, secret)
	if err != nil {
		t.Fatalf("resolve workload token: %v", err)
	}
	if user != nil {
		t.Fatalf("workload user = %#v, want nil", user)
	}
	if principal.PrincipalType != "AGENT_WORKLOAD" || principal.AgentID != "agent-1" || principal.AgentGeneration != 2 ||
		principal.ApplicationID != "app-1" || principal.RuntimeID != "runtime-1" || principal.GrantRef != "grant-1" {
		t.Fatalf("workload principal = %#v", principal)
	}
	if len(principal.Permissions) != 0 {
		t.Fatalf("workload permissions = %#v, want empty", principal.Permissions)
	}
}

func TestIssueAgentWorkloadAccessTokenRejectsIncompleteScope(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	if _, err := IssueAgentWorkloadAccessToken(secret, "agent-1", 1, "", "runtime-1", "grant-1", time.Minute); err == nil {
		t.Fatal("expected missing application scope to be rejected")
	}
}

func TestResolveIdentityBearerRejectsAgentWorkloadWithAdditionalAudience(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now()
	claims := IdentityTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "agent-1", ID: "token-1", Audience: jwt.ClaimStrings{"mcp", "other"},
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		PrincipalType: "AGENT_WORKLOAD", AgentID: "agent-1", AgentGeneration: 2,
		ApplicationID: "app-1", RuntimeID: "runtime-1", GrantRef: "grant-1",
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign workload token: %v", err)
	}
	if _, _, err := ResolveIdentityBearer(context.Background(), "Bearer "+token, nil, secret); err == nil {
		t.Fatal("expected additional workload audience to be rejected")
	}
}

type auditStore struct {
	records []*iapiserver.PlatformAuditLog
}

func (s *auditStore) GetSystemAuthConfig(context.Context) (*iapiserver.PlatformSystemAuthConfig, error) {
	return nil, nil
}

func (s *auditStore) ReplaceSystemAuthConfig(context.Context, *iapiserver.PlatformSystemAuthConfig, int64) (*iapiserver.PlatformSystemAuthConfig, error) {
	return nil, nil
}

func (s *auditStore) AppendAuditLog(_ context.Context, record *iapiserver.PlatformAuditLog) (*iapiserver.PlatformAuditLog, error) {
	copy := *record
	s.records = append(s.records, &copy)
	return &copy, nil
}

func (s *auditStore) GetAuditLog(context.Context, string) (*iapiserver.PlatformAuditLog, error) {
	return nil, nil
}

func (s *auditStore) ListAuditLogs(context.Context, *iapiserver.PlatformAuditLogListRequest) ([]*iapiserver.PlatformAuditLog, int64, error) {
	return nil, 0, nil
}

func TestAuditRecordsPrincipalEstablishedByInnerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	platform := &auditStore{}
	router := gin.New()
	router.Use(Audit(platform, IdentityAuditSourceDomain))
	router.Use(func(c *gin.Context) {
		SetIdentityContext(c, IdentityPrincipal{
			PrincipalType: "USER",
			PrincipalID:   "user-123",
			ActorUserID:   "user-123",
		}, nil)
		c.Next()
	})
	router.GET("/audit-test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/audit-test", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if len(platform.records) != 1 {
		t.Fatalf("audit record count = %d, want 1", len(platform.records))
	}
	record := platform.records[0]
	if record.PrincipalType != "USER" || record.PrincipalID != "user-123" || record.ActorUserID != "user-123" {
		t.Fatalf("audit principal = (%q, %q, %q), want (USER, user-123, user-123)", record.PrincipalType, record.PrincipalID, record.ActorUserID)
	}
}
