package applicationplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type AppEngineHealthyChecker interface {
	Check(ctx context.Context, engine *iapiserver.AppEngine) (healthy bool, reason string)
}

type httpAppEngineHealthyChecker struct {
	client *http.Client
}

type comfyUIHealthyChecker struct {
	httpAppEngineHealthyChecker
}

func defaultAppEngineCheckers() map[string]AppEngineHealthyChecker {
	client := &http.Client{Timeout: 5 * time.Second}
	return map[string]AppEngineHealthyChecker{
		iapiserver.AppEngineTypeComfyUI: &comfyUIHealthyChecker{
			httpAppEngineHealthyChecker: httpAppEngineHealthyChecker{client: client},
		},
		iapiserver.AppEngineTypeSaaSAPI: &httpAppEngineHealthyChecker{client: client},
	}
}

func (s *applicationPlatformService) ListAppEngines(
	ctx context.Context,
	req *iapiserver.AppEngineListRequest,
) (*iapiserver.AppEngineListResponse, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = principal.userID
	req.IncludeAll = principal.admin
	items, total, err := s.store.ApplicationPlatforms().ListAppEngines(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AppEngineListResponse{Total: total, Items: items}, nil
}

func (s *applicationPlatformService) CreateAppEngine(
	ctx context.Context,
	req *iapiserver.AppEngineCreateRequest,
) (*iapiserver.AppEngine, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateAppEngineAuthConfig(req.AuthType, req.AuthConfig); err != nil {
		return nil, err
	}
	if err := validateAppEngineSaaSConfig(req.EngineType, req.SaaSPlatformType, req.SupportedCapabilityTypes); err != nil {
		return nil, err
	}
	if err := s.ensureAppEngineNameUnique(ctx, principal.userID, req.Name, ""); err != nil {
		return nil, err
	}
	engine := &iapiserver.AppEngine{
		OwnerUserID:              principal.userID,
		EngineType:               req.EngineType,
		SaaSPlatformType:         req.SaaSPlatformType,
		Endpoint:                 req.Endpoint,
		AuthType:                 req.AuthType,
		AuthConfig:               req.AuthConfig,
		Status:                   iapiserver.AppEngineStatusActive,
		HealthStatus:             iapiserver.AppEngineHealthUnknown,
		SupportedCapabilityTypes: req.SupportedCapabilityTypes,
		HealthCheckConfig:        req.HealthCheckConfig,
		CapabilityTags:           req.CapabilityTags,
	}
	engine.Name = req.Name
	engine.Description = req.Description
	created, err := s.store.ApplicationPlatforms().AddAppEngine(ctx, engine)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *applicationPlatformService) GetAppEngine(ctx context.Context, id string) (*iapiserver.AppEngine, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	engine, err := s.store.ApplicationPlatforms().GetAppEngine(ctx, id)
	if err != nil {
		return nil, errors.NewStatusF(code.ErrAppEngineNotVisible, "app engine not found or not visible")
	}
	if !principal.canAccess(engine.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAppEngineNotVisible, "app engine not found or not visible")
	}
	return engine, nil
}

func (s *applicationPlatformService) UpdateAppEngine(
	ctx context.Context,
	req *iapiserver.AppEngineUpdateRequest,
) (*iapiserver.AppEngine, error) {
	engine, err := s.GetAppEngine(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if err := s.ensureAppEngineNameUnique(ctx, engine.OwnerUserID, *req.Name, engine.ID); err != nil {
			return nil, err
		}
		engine.Name = *req.Name
	}
	if req.Description != nil {
		engine.Description = *req.Description
	}
	if req.Endpoint != nil {
		engine.Endpoint = *req.Endpoint
	}
	if req.SaaSPlatformType != nil {
		engine.SaaSPlatformType = *req.SaaSPlatformType
	}
	if req.AuthType != nil {
		engine.AuthType = *req.AuthType
	}
	if req.AuthConfig != nil {
		engine.AuthConfig = *req.AuthConfig
	}
	if req.Status != nil {
		engine.Status = *req.Status
	}
	if req.SupportedCapabilityTypes != nil {
		engine.SupportedCapabilityTypes = *req.SupportedCapabilityTypes
	}
	if req.HealthCheckConfig != nil {
		engine.HealthCheckConfig = *req.HealthCheckConfig
	}
	if req.CapabilityTags != nil {
		engine.CapabilityTags = *req.CapabilityTags
	}
	if err := validateAppEngineAuthConfig(engine.AuthType, engine.AuthConfig); err != nil {
		return nil, err
	}
	if err := validateAppEngineSaaSConfig(engine.EngineType, engine.SaaSPlatformType, engine.SupportedCapabilityTypes); err != nil {
		return nil, err
	}
	if engine.EngineType == iapiserver.AppEngineTypeComfyUI {
		engine.SaaSPlatformType = ""
		engine.SupportedCapabilityTypes = nil
	}
	updated, err := s.store.ApplicationPlatforms().UpdateAppEngine(ctx, engine)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *applicationPlatformService) CheckAppEngineHealth(
	ctx context.Context,
	id string,
) (*iapiserver.AppEngine, error) {
	engine, err := s.GetAppEngine(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateAppEngineAuthConfig(engine.AuthType, engine.AuthConfig); err != nil {
		return nil, err
	}
	result, err := s.checkAppEngine(ctx, engine, engine.Status == iapiserver.AppEngineStatusDisabled)
	if err != nil {
		return nil, err
	}
	engine.HealthStatus = result.HealthStatus
	engine.LastHealthCheckAt = &result.CheckedAt
	engine.UnhealthyReason = result.UnhealthyReason
	updated, err := s.store.ApplicationPlatforms().UpdateAppEngine(ctx, engine)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *applicationPlatformService) CheckAppEngineHealthByConfig(
	ctx context.Context,
	req *iapiserver.AppEngineHealthCheckRequest,
) (*iapiserver.AppEngineHealthCheckResult, error) {
	if err := validateAppEngineAuthConfig(req.AuthType, req.AuthConfig); err != nil {
		return nil, err
	}
	if err := validateAppEngineSaaSConfig(req.EngineType, req.SaaSPlatformType, nil); err != nil {
		return nil, err
	}
	engine := &iapiserver.AppEngine{
		EngineType:        req.EngineType,
		SaaSPlatformType:  req.SaaSPlatformType,
		Endpoint:          req.Endpoint,
		AuthType:          req.AuthType,
		AuthConfig:        req.AuthConfig,
		Status:            iapiserver.AppEngineStatusActive,
		HealthCheckConfig: req.HealthCheckConfig,
	}
	return s.checkAppEngine(ctx, engine, false)
}

func (s *applicationPlatformService) DeleteAppEngine(
	ctx context.Context,
	id string,
) (*iapiserver.SuccessResponse, error) {
	if _, err := s.GetAppEngine(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.ApplicationPlatforms().DeleteAppEngine(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *applicationPlatformService) checkAppEngine(
	ctx context.Context,
	engine *iapiserver.AppEngine,
	disabled bool,
) (*iapiserver.AppEngineHealthCheckResult, error) {
	start := time.Now()
	checkedAt := imachinery.NewTime(start)
	if disabled {
		return &iapiserver.AppEngineHealthCheckResult{
			HealthStatus:    iapiserver.AppEngineHealthUnhealthy,
			CheckedAt:       checkedAt,
			UnhealthyReason: "app engine is disabled",
			LatencyMs:       0,
			RawSummary:      map[string]any{},
		}, nil
	}
	mode := strings.TrimSpace(engine.HealthCheckConfig.Mode)
	if mode == "none" {
		return &iapiserver.AppEngineHealthCheckResult{
			HealthStatus: iapiserver.AppEngineHealthHealthy,
			CheckedAt:    checkedAt,
			LatencyMs:    0,
			RawSummary:   map[string]any{"mode": "none"},
		}, nil
	}
	if mode != "" && mode != "http_ping" && mode != "api_call" {
		return nil, errors.NewStatusF(code.ErrAppEngineHealthCheckConfigInvalid, "health check mode is unsupported")
	}
	checker, ok := s.checkers[engine.EngineType]
	if !ok {
		return nil, errors.NewStatusF(code.ErrAppEngineTypeMismatched, "app engine type %s is unsupported", engine.EngineType)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	healthy, reason := checker.Check(checkCtx, engine)
	result := &iapiserver.AppEngineHealthCheckResult{
		CheckedAt:  checkedAt,
		LatencyMs:  time.Since(start).Milliseconds(),
		RawSummary: map[string]any{},
	}
	if healthy {
		result.HealthStatus = iapiserver.AppEngineHealthHealthy
	} else {
		result.HealthStatus = iapiserver.AppEngineHealthUnhealthy
		result.UnhealthyReason = reason
	}
	return result, nil
}

func (s *applicationPlatformService) ensureAppEngineNameUnique(
	ctx context.Context,
	ownerUserID, name, excludeID string,
) error {
	item, err := s.store.ApplicationPlatforms().GetAppEngineByOwnerName(ctx, ownerUserID, name)
	if err != nil {
		return nil
	}
	if item.ID != excludeID {
		return errors.NewStatusF(code.ErrAppEngineNameDuplicated, "app engine name %s already exists", name)
	}
	return nil
}

func validateAppEngineSaaSConfig(engineType, platformType string, capabilities []string) error {
	if engineType == iapiserver.AppEngineTypeSaaSAPI && strings.TrimSpace(platformType) == "" {
		return errors.NewStatusF(code.ErrAppEngineHealthCheckConfigInvalid, "saas platform type is required")
	}
	if strings.TrimSpace(platformType) != "" && !validSaaSPlatformType(platformType) {
		return errors.NewStatusF(code.ErrAppEngineHealthCheckConfigInvalid, "saas platform type is unsupported")
	}
	for _, capability := range capabilities {
		if !validCapabilityType(capability) {
			return errors.NewStatusF(code.ErrAppEngineHealthCheckConfigInvalid, "capability type %s is unsupported", capability)
		}
	}
	return nil
}

func validateAppEngineAuthConfig(authType string, authConfig iapiserver.AppEngineAuthConfig) error {
	switch authType {
	case iapiserver.AppEngineAuthBearerToken:
		if strings.TrimSpace(authConfig.Token) == "" {
			return errors.NewStatusF(code.ErrAppEngineAuthConfigInvalid, "bearer token is required")
		}
	case iapiserver.AppEngineAuthAPIKey:
		if strings.TrimSpace(authConfig.APIKey) == "" {
			return errors.NewStatusF(code.ErrAppEngineAuthConfigInvalid, "api key is required")
		}
	case iapiserver.AppEngineAuthAKSK:
		if strings.TrimSpace(authConfig.AccessKey) == "" || strings.TrimSpace(authConfig.SecretKey) == "" {
			return errors.NewStatusF(code.ErrAppEngineAuthConfigInvalid, "access key and secret key are required")
		}
	case iapiserver.AppEngineAuthNone:
		return nil
	default:
		return errors.NewStatusF(code.ErrAppEngineAuthConfigInvalid, "auth type %s is unsupported", authType)
	}
	return nil
}

func (c *comfyUIHealthyChecker) Check(ctx context.Context, engine *iapiserver.AppEngine) (bool, string) {
	probe, err := appendEndpointPath(engine.Endpoint, "system_stats")
	if err != nil {
		return false, err.Error()
	}
	return c.checkURL(ctx, probe, engine)
}

func (c *httpAppEngineHealthyChecker) Check(ctx context.Context, engine *iapiserver.AppEngine) (bool, string) {
	target := engine.Endpoint
	if strings.TrimSpace(engine.HealthCheckConfig.Path) != "" {
		probe, err := appendEndpointPath(engine.Endpoint, engine.HealthCheckConfig.Path)
		if err != nil {
			return false, err.Error()
		}
		target = probe
	}
	return c.checkURL(ctx, target, engine)
}

func (c *httpAppEngineHealthyChecker) checkURL(
	ctx context.Context,
	target string,
	engine *iapiserver.AppEngine,
) (bool, string) {
	method := strings.ToUpper(strings.TrimSpace(engine.HealthCheckConfig.Method))
	if method == "" {
		method = http.MethodGet
	}
	body := bytes.NewReader(nil)
	if len(engine.HealthCheckConfig.Payload) > 0 {
		payload, err := json.Marshal(engine.HealthCheckConfig.Payload)
		if err != nil {
			return false, err.Error()
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return false, err.Error()
	}
	if len(engine.HealthCheckConfig.Payload) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	applyAppEngineAuth(req, engine)
	client := c.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if engine.HealthCheckConfig.ExpectedStatus > 0 {
		if resp.StatusCode == engine.HealthCheckConfig.ExpectedStatus {
			return true, ""
		}
		return false, resp.Status
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, ""
	}
	return false, resp.Status
}

func applyAppEngineAuth(req *http.Request, engine *iapiserver.AppEngine) {
	switch engine.AuthType {
	case iapiserver.AppEngineAuthBearerToken:
		req.Header.Set("Authorization", "Bearer "+engine.AuthConfig.Token)
	case iapiserver.AppEngineAuthAPIKey:
		req.Header.Set("X-API-Key", engine.AuthConfig.APIKey)
	case iapiserver.AppEngineAuthAKSK:
		req.Header.Set("X-Access-Key", engine.AuthConfig.AccessKey)
		req.Header.Set("X-Secret-Key", engine.AuthConfig.SecretKey)
	}
}

func appendEndpointPath(endpoint, suffix string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.Errorf("endpoint must be an absolute url")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(suffix, "/")
	return parsed.String(), nil
}
