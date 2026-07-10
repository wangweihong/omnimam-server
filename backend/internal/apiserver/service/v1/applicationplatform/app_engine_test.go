package applicationplatform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestValidateAppEngineAuthConfig(t *testing.T) {
	tests := []struct {
		name       string
		authType   string
		authConfig iapiserver.AppEngineAuthConfig
		wantErr    bool
	}{
		{name: "none", authType: iapiserver.AppEngineAuthNone},
		{
			name:       "bearer token",
			authType:   iapiserver.AppEngineAuthBearerToken,
			authConfig: iapiserver.AppEngineAuthConfig{Token: "token"},
		},
		{
			name:       "api key",
			authType:   iapiserver.AppEngineAuthAPIKey,
			authConfig: iapiserver.AppEngineAuthConfig{APIKey: "key"},
		},
		{
			name:       "ak sk",
			authType:   iapiserver.AppEngineAuthAKSK,
			authConfig: iapiserver.AppEngineAuthConfig{AccessKey: "ak", SecretKey: "sk"},
		},
		{name: "missing bearer token", authType: iapiserver.AppEngineAuthBearerToken, wantErr: true},
		{name: "missing api key", authType: iapiserver.AppEngineAuthAPIKey, wantErr: true},
		{
			name:       "missing secret key",
			authType:   iapiserver.AppEngineAuthAKSK,
			authConfig: iapiserver.AppEngineAuthConfig{AccessKey: "ak"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppEngineAuthConfig(tt.authType, tt.authConfig)
			if tt.wantErr && err == nil {
				t.Fatalf("validateAppEngineAuthConfig() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateAppEngineAuthConfig() error = %v", err)
			}
		})
	}
}

func TestHTTPAppEngineHealthyChecker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "secret" {
			t.Fatalf("X-API-Key = %q, want secret", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	checker := &httpAppEngineHealthyChecker{client: server.Client()}
	healthy, reason := checker.Check(context.Background(), &iapiserver.AppEngine{
		Endpoint:   server.URL,
		AuthType:   iapiserver.AppEngineAuthAPIKey,
		AuthConfig: iapiserver.AppEngineAuthConfig{APIKey: "secret"},
	})
	if !healthy {
		t.Fatalf("healthy = false, reason = %q", reason)
	}
}

func TestHTTPAppEngineHealthyCheckerUsesHealthCheckConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("path = %q, want /healthz", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	checker := &httpAppEngineHealthyChecker{client: server.Client()}
	healthy, reason := checker.Check(context.Background(), &iapiserver.AppEngine{
		Endpoint: server.URL,
		AuthType: iapiserver.AppEngineAuthNone,
		HealthCheckConfig: iapiserver.HealthCheckConfig{
			Path:           "/healthz",
			Method:         http.MethodPost,
			ExpectedStatus: http.StatusAccepted,
			Payload:        map[string]any{"ping": true},
		},
	})
	if !healthy {
		t.Fatalf("healthy = false, reason = %q", reason)
	}
}

func TestValidateAppEngineSaaSConfig(t *testing.T) {
	if err := validateAppEngineSaaSConfig(iapiserver.AppEngineTypeSaaSAPI, "", nil); err == nil {
		t.Fatalf("validateAppEngineSaaSConfig() error = nil, want platform required")
	}
	if err := validateAppEngineSaaSConfig(
		iapiserver.AppEngineTypeSaaSAPI,
		iapiserver.SaaSPlatformModelScope,
		[]string{iapiserver.CapabilityImageGeneration},
	); err != nil {
		t.Fatalf("validateAppEngineSaaSConfig() error = %v", err)
	}
	if err := validateAppEngineSaaSConfig(
		iapiserver.AppEngineTypeSaaSAPI,
		iapiserver.SaaSPlatformModelScope,
		[]string{"text_generation"},
	); err == nil {
		t.Fatalf("validateAppEngineSaaSConfig() error = nil, want unsupported capability")
	}
}

func TestComfyUIHealthyCheckerUsesSystemStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/system_stats" {
			t.Fatalf("path = %q, want /system_stats", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := &comfyUIHealthyChecker{httpAppEngineHealthyChecker: httpAppEngineHealthyChecker{client: server.Client()}}
	healthy, reason := checker.Check(context.Background(), &iapiserver.AppEngine{
		Endpoint: server.URL,
		AuthType: iapiserver.AppEngineAuthNone,
	})
	if !healthy {
		t.Fatalf("healthy = false, reason = %q", reason)
	}
}
