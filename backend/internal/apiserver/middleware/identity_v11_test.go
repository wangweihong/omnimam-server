package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

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
