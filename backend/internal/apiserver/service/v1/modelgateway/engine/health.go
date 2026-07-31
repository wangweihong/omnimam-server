package engine

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const healthPersistenceTimeout = time.Second

// CheckEngineInstanceHealth 执行管理员触发的单实例实时健康探测并持久化结果。
func (s *Service) CheckEngineInstanceHealth(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	return s.checkHealth(ctx, id)
}

// CheckEngineInstanceHealthInternal 为受信任的 TaskWorker 执行健康探测，不经过 HTTP 用户鉴权。
func (s *Service) CheckEngineInstanceHealthInternal(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	return s.checkHealth(ctx, id)
}

func (s *Service) checkHealth(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	item, err := s.store.ApplicationPlatforms().GetEngineInstance(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	now := imachinery.Now()
	var result *iapiserver.EngineHealthCheckResult
	typeDef, ok := s.runtime.EngineType(item.ApplicationEngineTypeID)
	if !ok {
		result = degradedHealth(id, now, "engine type is not registered")
	} else if adapter := s.adapters[typeDef.EngineAdapterID]; adapter == nil {
		result = degradedHealth(id, now, "engine adapter is not registered")
	} else {
		result, err = adapter.Check(ctx, item)
		if err != nil {
			// Worker shutdown must leave the current chunk retryable; a bounded detection deadline is a valid offline observation.
			if stderrors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}
			result = healthFromError(id, now, err)
		}
	}
	result = normalizeHealthResult(id, now, result)
	old := item.HealthStatus
	item.LastHealthCheckAt = &now
	item.HealthStatus = result.HealthStatus
	item.UnhealthyReason = result.FailureSummary
	var event *iapiserver.ApplicationPlatformEvent
	if old != item.HealthStatus {
		payload := map[string]any{
			"engine_instance_id": id, "application_engine_type_id": item.ApplicationEngineTypeID,
			"health_status": item.HealthStatus, "checked_at": now, "failure_summary": item.UnhealthyReason,
		}
		event = &iapiserver.ApplicationPlatformEvent{
			Type: "engine_instance_health_changed", IdempotencyKey: id + ":" + now.String(),
			Payload: payload, OccurredAt: now,
		}
	}
	persistCtx := ctx
	persistCancel := func() {}
	if stderrors.Is(ctx.Err(), context.DeadlineExceeded) {
		// 单项探测超时是需要落库的 offline 事实，不能继续复用已过期的探测 context。
		persistCtx, persistCancel = context.WithTimeout(context.WithoutCancel(ctx), healthPersistenceTimeout)
	}
	defer persistCancel()
	if _, err := s.store.ApplicationPlatforms().UpdateEngineInstanceHealth(persistCtx, item, item.ResourceVersion, event); err != nil {
		return nil, err
	}
	return result, nil
}

func healthFromError(id string, checkedAt imachinery.Time, err error) *iapiserver.EngineHealthCheckResult {
	status := errors.ToStatus(err)
	switch status.Code {
	case code.ErrAIAppEngineAuthConfigInvalid:
		return degradedHealth(id, checkedAt, "provider authentication failed")
	case code.ErrAIAppProviderRuntimeCapabilityMismatch:
		return degradedHealth(id, checkedAt, "provider health protocol is incompatible")
	case code.ErrAIAppEngineUnavailable:
		switch {
		case strings.Contains(status.Desc, "timed out"):
			return offlineHealth(id, checkedAt, "provider request timed out")
		case strings.Contains(status.Desc, "base URL"), strings.Contains(status.Desc, "could not be parsed"):
			return degradedHealth(id, checkedAt, "provider health protocol is incompatible")
		default:
			return offlineHealth(id, checkedAt, "provider is unavailable")
		}
	default:
		return degradedHealth(id, checkedAt, "engine health adapter failed")
	}
}

func normalizeHealthResult(id string, checkedAt imachinery.Time, result *iapiserver.EngineHealthCheckResult) *iapiserver.EngineHealthCheckResult {
	if result == nil {
		return degradedHealth(id, checkedAt, "engine health adapter failed")
	}
	result.EngineInstanceID, result.CheckedAt = id, checkedAt
	switch result.HealthStatus {
	case iapiserver.EngineHealthOnline:
		result.FailureSummary = ""
	case iapiserver.EngineHealthOffline:
		result.FailureSummary = safeHealthSummary(result.FailureSummary, "provider is unavailable")
	case iapiserver.EngineHealthDegraded:
		result.FailureSummary = safeHealthSummary(result.FailureSummary, "engine health protocol is degraded")
	default:
		return degradedHealth(id, checkedAt, "engine health adapter failed")
	}
	return result
}

func safeHealthSummary(summary, fallback string) string {
	switch summary {
	case "engine type is not registered",
		"engine adapter is not registered",
		"provider authentication failed",
		"provider health protocol is incompatible",
		"provider request timed out",
		"provider is unavailable",
		"engine health adapter failed",
		"engine health protocol is degraded":
		return summary
	default:
		return fallback
	}
}

func degradedHealth(id string, checkedAt imachinery.Time, summary string) *iapiserver.EngineHealthCheckResult {
	return &iapiserver.EngineHealthCheckResult{
		EngineInstanceID: id, HealthStatus: iapiserver.EngineHealthDegraded,
		CheckedAt: checkedAt, FailureSummary: summary,
	}
}

func offlineHealth(id string, checkedAt imachinery.Time, summary string) *iapiserver.EngineHealthCheckResult {
	return &iapiserver.EngineHealthCheckResult{
		EngineInstanceID: id, HealthStatus: iapiserver.EngineHealthOffline,
		CheckedAt: checkedAt, FailureSummary: summary,
	}
}
