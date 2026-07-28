package applicationplatform

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type engineHealthStore struct {
	executorApplicationStore
	updated          *iapiserver.EngineInstance
	event            *iapiserver.ApplicationPlatformEvent
	updateContextErr error
}

func (s *engineHealthStore) UpdateEngineInstanceHealth(ctx context.Context, data *iapiserver.EngineInstance, _ int64, event *iapiserver.ApplicationPlatformEvent) (*iapiserver.EngineInstance, error) {
	s.updateContextErr = ctx.Err()
	if s.updateContextErr != nil {
		return nil, s.updateContextErr
	}
	copyData := *data
	s.updated, s.event = &copyData, event
	return &copyData, nil
}

type engineHealthAdapter struct {
	result *iapiserver.EngineHealthCheckResult
	err    error
}

func (engineHealthAdapter) ID() string { return "deepseek_official" }
func (a engineHealthAdapter) Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	return a.result, a.err
}

type deadlineEngineHealthAdapter struct{}

func (deadlineEngineHealthAdapter) ID() string { return "deepseek_official" }
func (deadlineEngineHealthAdapter) Check(ctx context.Context, _ *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	<-ctx.Done()
	return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider request timed out")
}

func TestEngineHealthClassificationAndSafeSummary(t *testing.T) {
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		adapter     EngineAdapter
		wantStatus  string
		wantSummary string
	}{
		{name: "network unavailable", adapter: engineHealthAdapter{err: errors.NewStatus(code.ErrAIAppEngineUnavailable, "https://user:secret@example.invalid failed with token=secret")}, wantStatus: iapiserver.EngineHealthOffline, wantSummary: "provider is unavailable"},
		{name: "timeout", adapter: engineHealthAdapter{err: errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider request timed out")}, wantStatus: iapiserver.EngineHealthOffline, wantSummary: "provider request timed out"},
		{name: "authentication", adapter: engineHealthAdapter{err: errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "bearer secret rejected")}, wantStatus: iapiserver.EngineHealthDegraded, wantSummary: "provider authentication failed"},
		{name: "protocol", adapter: engineHealthAdapter{err: errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "raw upstream payload")}, wantStatus: iapiserver.EngineHealthDegraded, wantSummary: "provider health protocol is incompatible"},
		{name: "adapter payload is redacted", adapter: engineHealthAdapter{result: &iapiserver.EngineHealthCheckResult{HealthStatus: iapiserver.EngineHealthDegraded, FailureSummary: "https://example.invalid?api_key=secret"}}, wantStatus: iapiserver.EngineHealthDegraded, wantSummary: "engine health protocol is degraded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", HealthStatus: iapiserver.EngineHealthOnline}
			engine.ID = "engine-1"
			storage := &engineHealthStore{executorApplicationStore: executorApplicationStore{engine: engine}}
			service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: storage}, Runtime: runtimeRegistry, Adapters: map[string]EngineAdapter{"deepseek_official": tt.adapter}}}
			result, err := service.CheckEngineInstanceHealthInternal(context.Background(), engine.ID)
			if err != nil {
				t.Fatal(err)
			}
			if result.HealthStatus != tt.wantStatus || result.FailureSummary != tt.wantSummary {
				t.Fatalf("result = %#v", result)
			}
			if storage.updated == nil || storage.updated.HealthStatus != tt.wantStatus || storage.updated.UnhealthyReason != tt.wantSummary {
				t.Fatalf("updated = %#v", storage.updated)
			}
			if strings.Contains(storage.updated.UnhealthyReason, "secret") || strings.Contains(storage.updated.UnhealthyReason, "http") {
				t.Fatalf("unsafe health summary persisted: %q", storage.updated.UnhealthyReason)
			}
			if storage.event == nil || storage.event.Payload["failure_summary"] != tt.wantSummary {
				t.Fatalf("event = %#v", storage.event)
			}
		})
	}
}

func TestEngineHealthMissingAdapterIsPersistedAsDegraded(t *testing.T) {
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	storage := &engineHealthStore{executorApplicationStore: executorApplicationStore{engine: engine}}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: storage}, Runtime: runtimeRegistry, Adapters: map[string]EngineAdapter{}}}
	result, err := service.CheckEngineInstanceHealthInternal(context.Background(), engine.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.HealthStatus != iapiserver.EngineHealthDegraded || result.FailureSummary != "engine adapter is not registered" || storage.updated == nil {
		t.Fatalf("result = %#v, updated = %#v", result, storage.updated)
	}
}

func TestEngineHealthCancellationKeepsChunkRetryable(t *testing.T) {
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	storage := &engineHealthStore{executorApplicationStore: executorApplicationStore{engine: engine}}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: storage}, Runtime: runtimeRegistry, Adapters: map[string]EngineAdapter{"deepseek_official": engineHealthAdapter{err: context.Canceled}}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.CheckEngineInstanceHealthInternal(ctx, engine.ID); err == nil {
		t.Fatal("expected cancellation")
	}
	if storage.updated != nil {
		t.Fatalf("canceled detection updated engine: %#v", storage.updated)
	}
}

func TestEngineHealthDeadlinePersistsOfflineWithFreshContext(t *testing.T) {
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", HealthStatus: iapiserver.EngineHealthUnknown}
	engine.ID = "engine-1"
	storage := &engineHealthStore{executorApplicationStore: executorApplicationStore{engine: engine}}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: storage}, Runtime: runtimeRegistry, Adapters: map[string]EngineAdapter{"deepseek_official": deadlineEngineHealthAdapter{}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := service.CheckEngineInstanceHealthInternal(ctx, engine.ID)
	if err != nil {
		t.Fatalf("deadline health check failed: %v", err)
	}
	if storage.updateContextErr != nil {
		t.Fatalf("health fact used expired persistence context: %v", storage.updateContextErr)
	}
	if result.HealthStatus != iapiserver.EngineHealthOffline || result.FailureSummary != "provider request timed out" {
		t.Fatalf("result = %#v", result)
	}
	if storage.updated == nil || storage.updated.HealthStatus != iapiserver.EngineHealthOffline || storage.updated.LastHealthCheckAt == nil {
		t.Fatalf("updated = %#v", storage.updated)
	}
}
