package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/internal/pkg/codec"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type authenticationUserStore struct {
	user *iapiserver.User
	err  error
}

func (s *authenticationUserStore) Get(context.Context, string) (*iapiserver.User, error) {
	return s.user, s.err
}
func (*authenticationUserStore) List(context.Context, *iapiserver.UserListRequest) ([]*iapiserver.User, int64, error) {
	return nil, 0, nil
}
func (*authenticationUserStore) GetByName(context.Context, string) (*iapiserver.User, error) {
	return nil, nil
}
func (*authenticationUserStore) Delete(context.Context, string) error { return nil }
func (*authenticationUserStore) Update(context.Context, *iapiserver.User) (*iapiserver.User, error) {
	return nil, nil
}
func (*authenticationUserStore) Sync(context.Context, []*iapiserver.User) error { return nil }
func (*authenticationUserStore) Add(context.Context, *iapiserver.User) (*iapiserver.User, error) {
	return nil, nil
}

func TestAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &iapiserver.User{}
	user.ID = "user-1"
	user.Name = "developer"
	token, err := codec.GenerateUserTokenStr(&iapiserver.TokenInfo{UserUUID: user.ID})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		mode       string
		allowAnon  bool
		header     string
		cookie     string
		wantStatus int
		wantUserID string
		storeErr   error
	}{
		{name: "bearer token", mode: "release", header: "Bearer " + token, wantStatus: http.StatusOK, wantUserID: user.ID},
		{name: "token cookie", mode: "release", cookie: token, wantStatus: http.StatusOK, wantUserID: user.ID},
		{name: "development anonymous", mode: "debug", allowAnon: true, wantStatus: http.StatusOK, wantUserID: "system-admin"},
		{name: "release anonymous rejected", mode: "release", allowAnon: true, wantStatus: http.StatusUnauthorized},
		{name: "invalid bearer format", mode: "debug", allowAnon: true, header: "Basic invalid", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", mode: "release", header: "Bearer invalid", wantStatus: http.StatusUnauthorized},
		{name: "token user not found", mode: "release", header: "Bearer " + token, storeErr: errors.New("not found"), wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Authentication(&options.AuthOptions{AllowAnonymousDevelopment: tt.allowAnon}, tt.mode, &authenticationUserStore{user: user, err: tt.storeErr}))
			router.GET("/test", func(c *gin.Context) {
				ginUser, ok := c.Get(iapiserver.GinContextKeyUser)
				requestUser, requestErr := ctxvalue.GetValue[*iapiserver.User](c.Request.Context(), iapiserver.GinContextKeyUser)
				if !ok || requestErr != nil || ginUser != requestUser {
					c.Status(http.StatusInternalServerError)
					return
				}
				c.String(http.StatusOK, requestUser.ID)
			})
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: iapiserver.CookieKeyToken, Value: tt.cookie})
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.wantUserID != "" && recorder.Body.String() != tt.wantUserID {
				t.Fatalf("user = %q, want %q", recorder.Body.String(), tt.wantUserID)
			}
		})
	}
}

func TestAuthenticationAllowsPublicAuthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Authentication(options.NewAuthOptions(), "release", &authenticationUserStore{}))
	router.GET("/api/v1/auth/sso/start", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/start", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
