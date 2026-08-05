package modelgateway

import (
	"context"
	"maps"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	CapabilityResolutionUnknown     = "unknown"
	CapabilityResolutionResolved    = "resolved"
	CapabilityResolutionUnavailable = "unavailable"
	CapabilityResolutionStale       = "stale"
	UserModelExecutionContextIssuer = "user-model"
)

// UserModelExecutionContext is a short-lived, in-process execution grant issued by User Model.
type UserModelExecutionContext struct {
	OwnerUserID             string
	ProviderID              string
	ModelID                 string
	RemoteModel             string
	ProviderType            string
	CapabilityDefinitionID  string
	CapabilityDefinitionIDs []string
	ConfigVersion           int64
	CredentialHandle        string
	IssuedAt                time.Time
	ExpiresAt               time.Time
	Issuer                  string
	ModelSnapshot           map[string]any
	Endpoint                string
	AuthenticationType      string
	ProviderConfiguration   map[string]any
}

type UserModelTarget struct {
	ExecutionContext UserModelExecutionContext
}

type OperationExecutionRequest struct {
	PrincipalUserID        string
	Target                 UserModelTarget
	CapabilityDefinitionID string
	Input                  map[string]any
	ExecutionOptions       map[string]any
}

type OperationExecutionResult struct {
	Output map[string]any
}

// CredentialResolveRequest 将不透明句柄绑定到当前 ProviderType 和认证方式后解析。
type CredentialResolveRequest struct {
	Handle             string
	ProviderType       string
	AuthenticationType string
}

// ResolvedCredential 只在单次 Gateway 请求内保存受信任 resolver 返回的鉴权字段。
type ResolvedCredential struct {
	Authentication map[string]any
}

// CredentialResolver 解析 User Model 签发的短期凭证句柄；实现不得向调用结果回传凭证明文。
type CredentialResolver interface {
	ResolveCredential(context.Context, CredentialResolveRequest) (*ResolvedCredential, error)
}

// ProviderConnectionRequest 是 User Model 调用 Gateway 连接、发现和探测能力的受控输入。
type ProviderConnectionRequest struct {
	ProviderType     string
	Endpoint         string
	AuthType         string
	CredentialHandle string
	Config           map[string]any
}

// ProviderProbeResult 返回协议无关的 Provider 连接事实。
type ProviderProbeResult struct {
	Success      bool
	HealthStatus string
}

// DiscoveredModel 是 Gateway 从远端目录返回的协议无关模型标识。
type DiscoveredModel struct {
	RemoteModel string
	DisplayName string
}

// ModelProbeResult 是 Adapter 对单个远端模型的协议无关探测事实。
type ModelProbeResult struct {
	RemoteModel             string
	Available               bool
	CapabilityDefinitionIDs []string
	StreamSupported         bool
}

// CapabilityResolution 是 Gateway 对 Adapter 能力、探测事实和用户关闭范围的交集结果。
type CapabilityResolution struct {
	CapabilityDefinitionIDs []string
	StreamSupported         bool
	Executable              bool
	UnavailableReason       string
	Status                  string
}

// UserModelAdapter 扩展协议 Adapter 的模型发现与无生成探测能力。
type UserModelAdapter interface {
	Adapter
	DiscoverProviderModels(context.Context, *iapiserver.EngineInstance) ([]DiscoveredModel, error)
	ProbeProviderModel(context.Context, *iapiserver.EngineInstance, string) (*ModelProbeResult, error)
}

// UserModelGateway 提供 user-model -> modelgateway 的受控模块接口，不持久化输入或结果。
type UserModelGateway interface {
	ListProviderTypes(context.Context) ([]*iapiserver.ProviderType, error)
	TestProviderConnection(context.Context, ProviderConnectionRequest) (*ProviderProbeResult, error)
	DiscoverProviderModels(context.Context, ProviderConnectionRequest) ([]DiscoveredModel, error)
	ProbeProviderModel(context.Context, ProviderConnectionRequest, string) (*ModelProbeResult, error)
	ResolveUserModelCapabilities(string, *ModelProbeResult, []string) (*CapabilityResolution, error)
	ExecuteOperation(context.Context, OperationExecutionRequest) (*OperationExecutionResult, error)
}

// UserModelGatewayDependencies 声明 User Model Gateway 所需的 Registry、Adapter 与凭证解析边界。
type UserModelGatewayDependencies struct {
	Runtime     *RuntimeRegistry
	Adapters    map[string]Adapter
	Executors   map[string]OperationExecutor
	Credentials CredentialResolver
}

// UserModelGatewayService 实现 User Model 使用的无持久化 Provider 操作。
type UserModelGatewayService struct {
	runtime     *RuntimeRegistry
	adapters    map[string]Adapter
	executors   map[string]OperationExecutor
	credentials CredentialResolver
}

// NewUserModelGatewayService 构造受控 Gateway；全部实现必须由 bootstrap 显式注入。
func NewUserModelGatewayService(deps UserModelGatewayDependencies) (*UserModelGatewayService, error) {
	if deps.Runtime == nil || deps.Adapters == nil || deps.Executors == nil || deps.Credentials == nil {
		return nil, errors.New("model gateway runtime, adapters, executors, and credential resolver are required")
	}
	return &UserModelGatewayService{runtime: deps.Runtime, adapters: deps.Adapters, executors: deps.Executors, credentials: deps.Credentials}, nil
}

// ListProviderTypes 返回 Registry 的稳定脱敏投影，不暴露 Adapter 或 Executor ID。
func (s *UserModelGatewayService) ListProviderTypes(context.Context) ([]*iapiserver.ProviderType, error) {
	return s.runtime.ProviderTypes(), nil
}

// TestProviderConnection 验证临时连接，不创建 Provider、EngineInstance、Binding 或健康事实。
func (s *UserModelGatewayService) TestProviderConnection(ctx context.Context, request ProviderConnectionRequest) (*ProviderProbeResult, error) {
	_, adapter, engine, err := s.resolveConnection(ctx, request)
	if err != nil {
		return nil, err
	}
	if _, err := adapter.Check(ctx, engine); err != nil {
		return nil, err
	}
	return &ProviderProbeResult{Success: true, HealthStatus: iapiserver.ProviderModelHealthHealthy}, nil
}

// DiscoverProviderModels 读取远端模型目录并返回排序、去重后的稳定模型标识。
func (s *UserModelGatewayService) DiscoverProviderModels(ctx context.Context, request ProviderConnectionRequest) ([]DiscoveredModel, error) {
	registered, adapter, engine, err := s.resolveConnection(ctx, request)
	if err != nil {
		return nil, err
	}
	if !registered.public.SupportsModelDiscovery {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type does not support model discovery")
	}
	discoverer, ok := adapter.(UserModelAdapter)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider model discovery adapter is unavailable")
	}
	items, err := discoverer.DiscoverProviderModels(ctx, engine)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]DiscoveredModel, len(items))
	for _, item := range items {
		item.RemoteModel = strings.TrimSpace(item.RemoteModel)
		if item.RemoteModel == "" {
			continue
		}
		if strings.TrimSpace(item.DisplayName) == "" {
			item.DisplayName = item.RemoteModel
		}
		byID[item.RemoteModel] = item
	}
	models := make([]DiscoveredModel, 0, len(byID))
	for _, item := range byID {
		models = append(models, item)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].RemoteModel < models[j].RemoteModel })
	return models, nil
}

// ProbeProviderModel 验证远端模型存在性，并由 Registry 补充 Adapter 可执行能力范围。
func (s *UserModelGatewayService) ProbeProviderModel(ctx context.Context, request ProviderConnectionRequest, remoteModel string) (*ModelProbeResult, error) {
	registered, adapter, engine, err := s.resolveConnection(ctx, request)
	if err != nil {
		return nil, err
	}
	if !registered.public.SupportsModelProbe {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type does not support model probe")
	}
	remoteModel = strings.TrimSpace(remoteModel)
	if remoteModel == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "remote model is required")
	}
	prober, ok := adapter.(UserModelAdapter)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider model probe adapter is unavailable")
	}
	result, err := prober.ProbeProviderModel(ctx, engine, remoteModel)
	if err != nil {
		return nil, err
	}
	result.RemoteModel = remoteModel
	result.CapabilityDefinitionIDs = intersectCapabilities(registered.operationExecutors, result.CapabilityDefinitionIDs)
	return result, nil
}

// ResolveUserModelCapabilities 计算最终能力交集；未知关闭项不能扩张或伪造 Registry 能力。
func (s *UserModelGatewayService) ResolveUserModelCapabilities(providerType string, probe *ModelProbeResult, disabled []string) (*CapabilityResolution, error) {
	registered, ok := s.runtime.providerType(strings.TrimSpace(providerType))
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type is not registered")
	}
	if probe == nil || !probe.Available {
		return &CapabilityResolution{Status: CapabilityResolutionUnavailable, UnavailableReason: "provider model is unavailable"}, nil
	}
	capabilities := intersectCapabilities(registered.operationExecutors, probe.CapabilityDefinitionIDs)
	available := make(map[string]struct{}, len(capabilities))
	for _, id := range capabilities {
		available[id] = struct{}{}
	}
	for _, id := range disabled {
		if _, ok := available[id]; !ok {
			return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "disabled capability is not available for provider model")
		}
		delete(available, id)
	}
	resolved := make([]string, 0, len(available))
	for id := range available {
		resolved = append(resolved, id)
	}
	sort.Strings(resolved)
	result := &CapabilityResolution{
		CapabilityDefinitionIDs: resolved,
		StreamSupported:         probe.StreamSupported,
		Executable:              len(resolved) > 0,
		Status:                  CapabilityResolutionResolved,
	}
	if !result.Executable {
		result.Status = CapabilityResolutionUnavailable
		result.UnavailableReason = "all provider model capabilities are disabled"
	}
	return result, nil
}

// ExecuteOperation validates a User Model grant and dispatches it through the immutable Registry mapping.
func (s *UserModelGatewayService) ExecuteOperation(ctx context.Context, request OperationExecutionRequest) (*OperationExecutionResult, error) {
	grant := request.Target.ExecutionContext
	now := time.Now()
	if grant.Issuer != UserModelExecutionContextIssuer || grant.IssuedAt.IsZero() || grant.ExpiresAt.IsZero() ||
		grant.IssuedAt.After(now) || !grant.ExpiresAt.After(now) {
		return nil, errors.NewStatus(code.ErrAIAppPermissionDenied, "user model execution context is invalid or expired")
	}
	if strings.TrimSpace(request.PrincipalUserID) == "" || request.PrincipalUserID != grant.OwnerUserID {
		return nil, errors.NewStatus(code.ErrAIAppPermissionDenied, "user model execution context is outside the principal scope")
	}
	capabilityID := strings.TrimSpace(request.CapabilityDefinitionID)
	if capabilityID == "" || capabilityID != grant.CapabilityDefinitionID || !containsString(grant.CapabilityDefinitionIDs, capabilityID) {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "user model execution capability is not granted")
	}
	if strings.TrimSpace(grant.ProviderID) == "" || strings.TrimSpace(grant.ModelID) == "" ||
		strings.TrimSpace(grant.RemoteModel) == "" || grant.ConfigVersion <= 0 {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "user model execution configuration is invalid")
	}
	registered, ok := s.runtime.providerType(strings.TrimSpace(grant.ProviderType))
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type is not registered")
	}
	executorID, ok := registered.operationExecutors[capabilityID]
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type does not execute the requested capability")
	}
	executor := s.executors[executorID]
	if executor == nil || executor.ID() != executorID {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider operation executor is unavailable")
	}
	_, _, engine, err := s.resolveConnection(ctx, ProviderConnectionRequest{
		ProviderType: grant.ProviderType, Endpoint: grant.Endpoint, AuthType: grant.AuthenticationType,
		CredentialHandle: grant.CredentialHandle, Config: grant.ProviderConfiguration,
	})
	if err != nil {
		return nil, err
	}
	input := maps.Clone(request.Input)
	if input == nil {
		input = map[string]any{}
	}
	for key, value := range request.ExecutionOptions {
		input[key] = value
	}
	input["model"] = grant.RemoteModel
	run := &iapiserver.ApplicationRun{
		OwnerUserID:   grant.OwnerUserID,
		InputSnapshot: input,
		CapabilitySourceSnapshot: map[string]any{
			"user_model": maps.Clone(grant.ModelSnapshot),
		},
		ExecutionSnapshot: map[string]any{
			"capability_definition_id": capabilityID,
			"model_config_version":     grant.ConfigVersion,
		},
	}
	output, err := executor.Execute(ctx, engine, run)
	if err != nil {
		return nil, err
	}
	return &OperationExecutionResult{Output: output}, nil
}

func (s *UserModelGatewayService) resolveConnection(ctx context.Context, request ProviderConnectionRequest) (registeredProviderType, Adapter, *iapiserver.EngineInstance, error) {
	registered, ok := s.runtime.providerType(strings.TrimSpace(request.ProviderType))
	if !ok {
		return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type is not registered")
	}
	adapter := s.adapters[registered.adapterID]
	if adapter == nil || adapter.ID() != registered.adapterID {
		return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider type adapter is unavailable")
	}
	endpoint := strings.TrimSpace(request.Endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider endpoint is invalid")
	}
	if !containsString(registered.public.AuthenticationTypes, request.AuthType) {
		return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "provider authentication type is unsupported")
	}
	config := request.Config
	if config == nil {
		config = map[string]any{}
	}
	if err := registered.configurationSchema.Validate(config); err != nil {
		return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "provider configuration is invalid")
	}
	authentication := map[string]any{}
	if request.AuthType != iapiserver.EngineAuthNone {
		if strings.TrimSpace(request.CredentialHandle) == "" {
			return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "credential handle is required")
		}
		credential, err := s.credentials.ResolveCredential(ctx, CredentialResolveRequest{
			Handle: request.CredentialHandle, ProviderType: request.ProviderType, AuthenticationType: request.AuthType,
		})
		if err != nil {
			return registeredProviderType{}, nil, nil, err
		}
		if credential == nil {
			return registeredProviderType{}, nil, nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "credential handle could not be resolved")
		}
		authentication = maps.Clone(credential.Authentication)
		if err := validateProviderAuthentication(request.AuthType, authentication); err != nil {
			return registeredProviderType{}, nil, nil, err
		}
	}
	for key, value := range config {
		authentication[key] = value
	}
	return registered, adapter, &iapiserver.EngineInstance{
		ApplicationEngineTypeID: request.ProviderType,
		BaseURL:                 endpoint, AuthType: request.AuthType, AuthConfig: authentication,
		Enabled: true, HealthStatus: iapiserver.EngineHealthOnline, RequestTimeoutSeconds: 5,
	}, nil
}

func validateProviderAuthentication(authType string, authentication map[string]any) error {
	fields, ok := authenticationFields(authType)
	if !ok {
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "provider authentication type is unsupported")
	}
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
		value, _ := authentication[field].(string)
		if strings.TrimSpace(value) == "" {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "credential handle is missing required authentication fields")
		}
	}
	for field := range authentication {
		if _, ok := allowed[field]; !ok {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "credential handle contains unsupported authentication fields")
		}
	}
	return nil
}

func intersectCapabilities(registered map[string]string, probed []string) []string {
	allowed := make(map[string]struct{}, len(probed))
	for _, id := range probed {
		allowed[id] = struct{}{}
	}
	result := make([]string, 0, len(registered))
	for id := range registered {
		if len(probed) == 0 {
			result = append(result, id)
			continue
		}
		if _, ok := allowed[id]; ok {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

var _ UserModelGateway = (*UserModelGatewayService)(nil)
