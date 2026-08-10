package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure/providers"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentmcp"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type Service struct {
	store       store.InfrastructureStore                  // 持久化存储层
	provider    providers.RuntimeProvider                  // 运行时提供者接口
	profiles    map[string]*iapiserver.InfraRuntimeProfile // 内存中的运行时配置模板
	stateMu     sync.RWMutex                               // 保护 endpoints/outputs 的读写锁
	endpoints   map[string]providers.ProviderEndpoint      // 内存中的端点缓存
	outputs     map[string]providers.ProviderOutputContent // 内存中的输出内容缓存
	mcpResolver agentmcp.Resolver
}

const resolvedEndpointTTL = time.Minute

func NewService(storage store.InfrastructureStore, provider providers.RuntimeProvider) (*Service, error) {
	if storage == nil {
		return nil, fmt.Errorf("infrastructure store is required")
	}
	if provider == nil {
		provider = providers.UnavailableProvider{}
	}
	profiles := defaultProfiles()
	service := &Service{store: storage, provider: provider, profiles: profiles, endpoints: make(map[string]providers.ProviderEndpoint), outputs: make(map[string]providers.ProviderOutputContent)}
	return service, nil
}

// SetMCPBindingResolver injects the Agent-domain resolver without coupling Infrastructure to Agent tables.
func (s *Service) SetMCPBindingResolver(resolver agentmcp.Resolver) { s.mcpResolver = resolver }

func (s *Service) getEndpoint(id string) (providers.ProviderEndpoint, bool) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	endpoint, ok := s.endpoints[id]
	return endpoint, ok
}

func (s *Service) getOutput(id string) (providers.ProviderOutputContent, bool) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	output, ok := s.outputs[id]
	return output, ok
}

func (s *Service) ReconcileCatalog(ctx context.Context) error {
	node, err := s.provider.Info(ctx)
	if err != nil {
		node = &iapiserver.InfraNode{ObjectMeta: imachinery.ObjectMeta{ID: iapiserver.InfraNodeIDDockerLocal, Name: iapiserver.InfraNodeNameDockerLocal}, ProviderType: iapiserver.InfraProviderTypeDocker, Status: iapiserver.InfraNodeStatusOffline}
	}
	items := make([]*iapiserver.InfraRuntimeProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		copy := profile.DeepCopy()
		items = append(items, copy)
	}
	return s.store.ReconcileInfraCatalog(ctx, items, node)
}
func defaultProfiles() map[string]*iapiserver.InfraRuntimeProfile {
	definitions := []struct {
		id, mode string
		caps     []string
	}{
		{iapiserver.InfraRuntimeProfileIDAgentHermes, iapiserver.InfraRuntimeModeService, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityNetwork, iapiserver.InfraRuntimeCapabilityPersistentWorkspace}},
		{iapiserver.InfraRuntimeProfileIDAgentCoding, iapiserver.InfraRuntimeModeService, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityNetwork, iapiserver.InfraRuntimeCapabilityWorkspaceTool}},
		{iapiserver.InfraRuntimeProfileIDAppStudioPreviewWeb, iapiserver.InfraRuntimeModeService, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityNetwork, iapiserver.InfraRuntimeCapabilityEndpoint}},
		{iapiserver.InfraRuntimeProfileIDAppStudioPreviewAPI, iapiserver.InfraRuntimeModeService, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityNetwork, iapiserver.InfraRuntimeCapabilityEndpoint}},
		{iapiserver.InfraRuntimeProfileIDAppStudioBuildWeb, iapiserver.InfraRuntimeModeJob, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityArtifactOutput}},
		{iapiserver.InfraRuntimeProfileIDAppStudioBuildAPI, iapiserver.InfraRuntimeModeJob, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityArtifactOutput}},
		{iapiserver.InfraRuntimeProfileIDAppStudioProductionWeb, iapiserver.InfraRuntimeModeService, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityNetwork, iapiserver.InfraRuntimeCapabilityEndpoint}},
		{iapiserver.InfraRuntimeProfileIDAppStudioProductionAPI, iapiserver.InfraRuntimeModeService, []string{iapiserver.InfraRuntimeCapabilityCPU, iapiserver.InfraRuntimeCapabilityNetwork, iapiserver.InfraRuntimeCapabilityEndpoint}}}
	result := make(map[string]*iapiserver.InfraRuntimeProfile, len(definitions))
	for _, item := range definitions {
		result[item.id] = &iapiserver.InfraRuntimeProfile{ObjectMeta: imachinery.ObjectMeta{ID: item.id, Name: item.id}, Revision: iapiserver.InfraRuntimeProfileRevisionInitial, RuntimeMode: item.mode, ProviderType: iapiserver.InfraProviderTypeDocker, Capabilities: item.caps, Status: iapiserver.InfraRuntimeProfileStatusActive}
	}
	return result
}
func (s *Service) ListProfiles(ctx context.Context, req *iapiserver.InfraBasicListRequest) (*iapiserver.InfraRuntimeProfileListResponse, error) {
	items, total, err := s.store.ListInfraRuntimeProfiles(ctx, req)
	return &iapiserver.InfraRuntimeProfileListResponse{Total: total, Items: items}, err
}
func (s *Service) GetProfile(ctx context.Context, id string) (*iapiserver.InfraRuntimeProfile, error) {
	return s.store.GetInfraRuntimeProfile(ctx, id)
}

// ListNodes 返回符合状态筛选和分页条件的受管节点摘要。
func (s *Service) ListNodes(ctx context.Context, req *iapiserver.InfraBasicListRequest) (*iapiserver.InfraNodeListResponse, error) {
	items, total, err := s.store.ListInfraNodes(ctx, req)
	return &iapiserver.InfraNodeListResponse{Total: total, Items: items}, err
}
func (s *Service) GetNode(ctx context.Context, id string) (*iapiserver.InfraNode, error) {
	return s.store.GetInfraNode(ctx, id)
}
func (s *Service) ListRuntimes(ctx context.Context, req *iapiserver.InfraRuntimeListRequest) (*iapiserver.InfraRuntimeListResponse, error) {
	items, total, err := s.store.ListInfraRuntimes(ctx, req)
	return &iapiserver.InfraRuntimeListResponse{Total: total, Items: items}, err
}
func (s *Service) GetRuntime(ctx context.Context, id string) (*iapiserver.InfraRuntime, error) {
	return s.store.GetInfraRuntime(ctx, id)
}
func (s *Service) GetEndpoint(ctx context.Context, id string) (*iapiserver.InfraRuntimeEndpoint, error) {
	if _, err := s.store.GetInfraRuntime(ctx, id); err != nil {
		return nil, err
	}
	return s.store.GetInfraRuntimeEndpoint(ctx, id)
}

func (s *Service) ResolveEndpoint(ctx context.Context, id string, req *iapiserver.InfraResolveEndpointRequest) (*iapiserver.InfraResolvedEndpoint, error) {
	endpoint, err := s.store.GetInfraRuntimeEndpointByID(ctx, id)
	if err != nil {
		return nil, err
	}
	runtime, err := s.store.GetInfraRuntime(ctx, endpoint.RuntimeID)
	if err != nil || runtime.OwnerReference != req.OwnerReference {
		return nil, errors.NewStatus(code.ErrInfraEndpointAccessDenied, "infra endpoint owner does not match")
	}
	now := time.Now()
	if endpoint.Status != iapiserver.InfraRuntimeEndpointStatusReady || runtime.Status != iapiserver.InfraRuntimeStatusRunning || !endpoint.RevokedAt.IsZero() || (!endpoint.ExpiresAt.IsZero() && !endpoint.ExpiresAt.Time.After(now)) {
		return nil, errors.NewStatus(code.ErrInfraEndpointNotReady, "infra endpoint is not ready")
	}
	target, ok := s.getEndpoint(id)
	if !ok || target.BaseURL == "" || (!target.ValidUntil.IsZero() && !target.ValidUntil.After(now)) {
		providerResult, inspectErr := s.provider.Inspect(ctx, runtime.ProviderRuntimeRef)
		if inspectErr != nil || providerResult == nil || providerResult.Status != iapiserver.InfraRuntimeStatusRunning || providerResult.Endpoint == nil {
			return nil, errors.NewStatus(code.ErrInfraEndpointNotReady, "infra endpoint target is unavailable")
		}
		s.rememberProviderState(endpoint, providerResult)
		target, ok = s.getEndpoint(id)
	}
	if !ok || target.BaseURL == "" || (!target.ValidUntil.IsZero() && !target.ValidUntil.After(now)) {
		return nil, errors.NewStatus(code.ErrInfraEndpointNotReady, "infra endpoint target is unavailable")
	}
	parsed, err := url.Parse(target.BaseURL)
	if err != nil || (parsed.Scheme != iapiserver.InfraProtocolHTTP && parsed.Scheme != iapiserver.InfraProtocolHTTPS) || parsed.Host == "" || parsed.Scheme != target.Protocol {
		return nil, errors.NewStatus(code.ErrInfraEndpointNotReady, "infra endpoint target is invalid")
	}
	// Provider 地址的生命周期和每次 resolve 返回的短时授权窗口不同；零值表示
	// Provider 未声明固有过期时间，不能把一个仍然可用的进程内地址判为已过期。
	resolvedValidUntil := now.Add(resolvedEndpointTTL)
	if !target.ValidUntil.IsZero() && target.ValidUntil.Before(resolvedValidUntil) {
		resolvedValidUntil = target.ValidUntil
	}
	if !endpoint.ExpiresAt.IsZero() && endpoint.ExpiresAt.Time.Before(resolvedValidUntil) {
		resolvedValidUntil = endpoint.ExpiresAt.Time
	}
	resolvedAt := imachinery.NewTime(now)
	validUntil := imachinery.NewTime(resolvedValidUntil)
	return &iapiserver.InfraResolvedEndpoint{EndpointRef: iapiserver.InfraRefPrefixEndpoint + endpoint.ID, RuntimeID: runtime.ID, Protocol: target.Protocol, BaseURL: target.BaseURL, ResolvedAt: resolvedAt, ValidUntil: validUntil}, nil
}

// ListOutputs 返回 Runtime 输出引用的分页结果。
func (s *Service) ListOutputs(ctx context.Context, id string, req *iapiserver.InfraBasicListRequest) (*iapiserver.InfraRuntimeOutputListResponse, error) {
	if _, err := s.store.GetInfraRuntime(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.store.ListInfraRuntimeOutputs(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichOutputs(items)
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRequestInvalid, err.Error())
	}
	return &iapiserver.InfraRuntimeOutputListResponse{Total: int64(len(items)), Items: imachinery.PaginateSlice(items, window)}, nil
}

type RuntimeOutputContent struct {
	Reader        io.ReadCloser
	MediaType     string
	SizeBytes     int64
	ContentDigest string
}

func (s *Service) ReadOutputContent(ctx context.Context, id string) (*RuntimeOutputContent, error) {
	output, err := s.store.GetInfraRuntimeOutput(ctx, id)
	if err != nil {
		return nil, err
	}
	content, ok := s.getOutput(id)
	if output.Status != iapiserver.InfraRuntimeOutputStatusCollected || !ok || content.Open == nil {
		return nil, errors.NewStatus(code.ErrInfraOutputContentUnavailable, "infra runtime output content is unavailable")
	}
	reader, err := content.Open(ctx)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraOutputContentUnavailable, "infra runtime output content cannot be opened")
	}
	return &RuntimeOutputContent{Reader: reader, MediaType: content.MediaType, SizeBytes: content.SizeBytes, ContentDigest: content.ContentDigest}, nil
}

func (s *Service) AttachOutputArtifact(ctx context.Context, id string, req *iapiserver.InfraAttachArtifactRequest) (*iapiserver.InfraRuntimeOutput, error) {
	output, err := s.store.GetInfraRuntimeOutput(ctx, id)
	if err != nil {
		return nil, err
	}
	content, ok := s.getOutput(id)
	if output.Status != iapiserver.InfraRuntimeOutputStatusCollected || !ok {
		return nil, errors.NewStatus(code.ErrInfraOutputContentUnavailable, "infra runtime output content is unavailable")
	}
	if content.SizeBytes != req.SizeBytes || content.ContentDigest != req.ContentDigest {
		return nil, errors.NewStatus(code.ErrInfraOutputIntegrityMismatch, "infra runtime output artifact metadata does not match")
	}
	attached, err := s.store.AttachInfraRuntimeOutputArtifact(ctx, id, req.ArtifactID)
	if err != nil {
		return nil, err
	}
	s.enrichOutput(attached)
	return attached, nil
}

// Logs 返回经 Provider 脱敏后的 Runtime 日志分页结果。
func (s *Service) Logs(ctx context.Context, id string, req *iapiserver.InfraBasicListRequest) (*iapiserver.InfraRuntimeLogListResponse, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.OwnerReference == "" || runtime.OwnerDomain != iapiserver.TaskWorkerOwnerDomainAgent || runtime.OwnerReference != req.OwnerReference {
		return nil, errors.NewStatus(code.ErrInfraEndpointAccessDenied, "infra runtime owner does not match")
	}
	if runtime.ProviderRuntimeRef == "" {
		return &iapiserver.InfraRuntimeLogListResponse{Items: []*iapiserver.InfraRuntimeLogEntry{}}, nil
	}
	items, err := s.provider.Logs(ctx, runtime.ProviderRuntimeRef, 5000)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRequestInvalid, err.Error())
	}
	return &iapiserver.InfraRuntimeLogListResponse{Total: int64(len(items)), Items: imachinery.PaginateSlice(items, window)}, nil
}

func (s *Service) Health(ctx context.Context, id, ownerReference string) (*iapiserver.InfraRuntimeHealthResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if ownerReference == "" || runtime.OwnerDomain != iapiserver.TaskWorkerOwnerDomainAgent || runtime.OwnerReference != ownerReference {
		return nil, errors.NewStatus(code.ErrInfraEndpointAccessDenied, "infra runtime owner does not match")
	}
	if runtime.ProviderRuntimeRef == "" {
		return &iapiserver.InfraRuntimeHealthResult{Status: iapiserver.AgentRuntimeHealthUnknown, CheckedAt: imachinery.Now(), Reason: iapiserver.AgentRuntimeHealthReasonNotProvisioned}, nil
	}
	return s.provider.Health(ctx, runtime.ProviderRuntimeRef)
}
func (s *Service) CreateRuntime(ctx context.Context, req *iapiserver.InfraCreateRuntimeRequest) (*iapiserver.InfraOperationResult, error) {
	profile, ok := s.profiles[req.RuntimeProfileID]
	if !ok || profile.Status != iapiserver.InfraRuntimeProfileStatusActive || profile.Revision != req.RuntimeProfileRevision {
		return nil, errors.NewStatus(code.ErrInfraRuntimeProfileNotFound, "infra runtime profile is unavailable")
	}
	if profile.RuntimeMode != req.RuntimeMode {
		return nil, errors.NewStatus(code.ErrInfraUnsupportedRuntimeMode, "runtime mode does not match profile")
	}
	fingerprint, err := requestFingerprint(req)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRequestInvalid, err.Error())
	}
	runtimeID := uuid.NewString()
	runtime := &iapiserver.InfraRuntime{ObjectMeta: imachinery.ObjectMeta{ID: runtimeID}, RuntimeMode: req.RuntimeMode, Status: iapiserver.InfraRuntimeStatusAccepted, RequestingService: req.RequestingService, OwnerDomain: req.OwnerDomain, OwnerReference: req.OwnerReference, RequestUserID: req.RequestUserID, RequestID: req.RequestID, RequestFingerprint: fingerprint, RuntimeProfileID: req.RuntimeProfileID, RuntimeProfileRevision: req.RuntimeProfileRevision, ProviderType: iapiserver.InfraProviderTypeDocker, SourceRef: req.SourceRef}
	timeout, _ := json.Marshal(req.TimeoutPolicy)
	runtime.TimeoutPolicy = timeout
	mounts := make([]*iapiserver.InfraRuntimeMount, 0, len(req.Mounts))
	for _, input := range req.Mounts {
		mounts = append(mounts, &iapiserver.InfraRuntimeMount{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, RuntimeID: runtimeID, SourceRef: input.SourceRef, MountKind: input.MountKind, TargetPath: input.TargetPath, ReadOnly: input.ReadOnly, AuthorizationRef: req.AuthorizationRef})
	}
	bindings := make([]*iapiserver.InfraRuntimeConfigBinding, 0, len(req.ConfigurationBindings))
	for _, input := range req.ConfigurationBindings {
		bindings = append(bindings, &iapiserver.InfraRuntimeConfigBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: input.Name}, RuntimeID: runtimeID, BindingType: input.BindingType, Reference: input.Reference, InjectionStatus: iapiserver.InfraRuntimeConfigBindingStatusPending})
	}
	created, err := s.store.CreateInfraRuntimeAggregate(ctx, runtime, mounts, bindings)
	if err != nil {
		return nil, err
	}
	if created.ID != runtimeID {
		return s.result(ctx, created, nil)
	}
	created.Status = iapiserver.InfraRuntimeStatusPreparing
	_, _ = s.store.UpdateInfraRuntime(ctx, created, nil, nil, "")
	markRuntimeFailed := func(failureCode string) {
		created.Status = iapiserver.InfraRuntimeStatusFailed
		created.FailureCode = failureCode
		_, _ = s.store.UpdateInfraRuntime(ctx, created, nil, nil, "")
	}
	providerRequest := providers.ProviderRequest{RuntimeID: created.ID, Profile: profile, Request: req}
	for _, binding := range req.ConfigurationBindings {
		if binding.BindingType != iapiserver.InfraConfigBindingTypeMCPServerRef {
			continue
		}
		if s.mcpResolver == nil {
			markRuntimeFailed("ERR_INFRA_RUNTIME_OPERATION_FAILED")
			return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, "MCP binding resolver is unavailable")
		}
		resolved, resolveErr := s.mcpResolver.ResolveMCPBinding(ctx, req.AuthorizationRef, req.OwnerReference, binding.Reference)
		if resolveErr != nil {
			markRuntimeFailed("ERR_INFRA_RUNTIME_OPERATION_FAILED")
			return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, "MCP binding resolution failed")
		}
		if resolved == nil || resolved.ServerKey == "" || resolved.Endpoint == "" {
			markRuntimeFailed("ERR_INFRA_RUNTIME_OPERATION_FAILED")
			return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, "MCP binding resolver returned an invalid result")
		}
		providerRequest.MCPBindings = append(providerRequest.MCPBindings, providers.ResolvedMCPBinding{
			ServerKey: resolved.ServerKey, ServerType: resolved.ServerType, Endpoint: resolved.Endpoint,
			Credential: resolved.Credential, AllowedTools: append([]string(nil), resolved.AllowedTools...), Configuration: resolved.Configuration,
		})
	}
	providerResult, err := s.provider.Ensure(ctx, providerRequest)
	if err != nil {
		markRuntimeFailed("ERR_INFRA_RUNTIME_OPERATION_FAILED")
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	if providerResult == nil {
		markRuntimeFailed("ERR_INFRA_RUNTIME_OPERATION_FAILED")
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, "provider returned no runtime result")
	}
	created.ProviderRuntimeRef = providerResult.ProviderRuntimeRef
	created.Status = providerResult.Status
	if created.Status == "" {
		created.Status = iapiserver.InfraRuntimeStatusRunning
	}
	if err := prepareProviderOutputs(created.ID, providerResult, req.OutputDeclarations); err != nil {
		markRuntimeFailed("ERR_INFRA_OUTPUT_COLLECTION_FAILED")
		return nil, errors.NewStatus(code.ErrInfraOutputCollectionFailed, err.Error())
	}
	endpoint := endpointFromResult(created, req, providerResult)
	created.EndpointRef = ""
	if endpoint != nil {
		created.EndpointRef = iapiserver.InfraRefPrefixEndpoint + endpoint.ID
	}
	created, err = s.store.UpdateInfraRuntime(ctx, created, endpoint, providerResult.Outputs, "")
	if err != nil {
		return nil, err
	}
	s.rememberProviderState(endpoint, providerResult)
	return s.result(ctx, created, providerResult)
}
func (s *Service) Start(ctx context.Context, id string) (*iapiserver.InfraOperationResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.ProviderRuntimeRef == "" {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, "provider runtime reference is unavailable")
	}
	result, err := s.provider.Start(ctx, runtime.ProviderRuntimeRef)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	runtime.Status = result.Status
	if runtime.Status == "" {
		runtime.Status = iapiserver.InfraRuntimeStatusRunning
	}
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, "")
	if err != nil {
		return nil, err
	}
	if endpoint, endpointErr := s.store.GetInfraRuntimeEndpoint(ctx, runtime.ID); endpointErr == nil {
		s.rememberProviderState(endpoint, result)
	}
	return s.result(ctx, runtime, result)
}

// Stop 停止 Service；对 Job 按契约执行取消语义。
func (s *Service) Stop(ctx context.Context, id string, deleteRuntime bool) (*iapiserver.InfraOperationResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.Status == iapiserver.InfraRuntimeStatusDeleted {
		return s.result(ctx, runtime, nil)
	}
	if !deleteRuntime {
		if runtime.RuntimeMode == iapiserver.InfraRuntimeModeJob && runtime.Status == iapiserver.InfraRuntimeStatusCanceled {
			return s.result(ctx, runtime, nil)
		}
		if runtime.RuntimeMode == iapiserver.InfraRuntimeModeService && runtime.Status == iapiserver.InfraRuntimeStatusStopped {
			return s.result(ctx, runtime, nil)
		}
	}
	result, err := s.provider.Stop(ctx, runtime.ProviderRuntimeRef, deleteRuntime)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	if deleteRuntime {
		runtime.Status = iapiserver.InfraRuntimeStatusDeleted
	} else if runtime.RuntimeMode == iapiserver.InfraRuntimeModeJob {
		runtime.Status = iapiserver.InfraRuntimeStatusCanceled
	} else {
		runtime.Status = iapiserver.InfraRuntimeStatusStopped
	}
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, "")
	if err != nil {
		return nil, err
	}
	return s.result(ctx, runtime, result)
}

// Delete 删除 Runtime 并返回删除后的脱敏摘要。
func (s *Service) Delete(ctx context.Context, id string) (*iapiserver.InfraRuntime, error) {
	result, err := s.Stop(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Runtime == nil {
		return nil, fmt.Errorf("infrastructure delete returned no runtime")
	}
	return result.Runtime, nil
}
func (s *Service) Cancel(ctx context.Context, id string) (*iapiserver.InfraOperationResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.RuntimeMode != iapiserver.InfraRuntimeModeJob {
		return nil, errors.NewStatus(code.ErrInfraRuntimeStateConflict, "only jobs can be canceled")
	}
	if runtime.Status == iapiserver.InfraRuntimeStatusCanceled {
		return s.result(ctx, runtime, nil)
	}
	result, err := s.provider.Stop(ctx, runtime.ProviderRuntimeRef, false)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	runtime.Status = iapiserver.InfraRuntimeStatusCanceled
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, "")
	if err != nil {
		return nil, err
	}
	return s.result(ctx, runtime, result)
}
func (s *Service) Reconcile(ctx context.Context, id string) (*iapiserver.InfraOperationResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	result, err := s.provider.Inspect(ctx, runtime.ProviderRuntimeRef)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	runtime.Status = result.Status
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, iapiserver.InfraRuntimeEventReasonReconciled)
	if err != nil {
		return nil, err
	}
	if endpoint, endpointErr := s.store.GetInfraRuntimeEndpoint(ctx, runtime.ID); endpointErr == nil {
		s.rememberProviderState(endpoint, result)
	}
	return s.result(ctx, runtime, result)
}
func (s *Service) result(ctx context.Context, runtime *iapiserver.InfraRuntime, provider *providers.ProviderResult) (*iapiserver.InfraOperationResult, error) {
	result := &iapiserver.InfraOperationResult{Runtime: runtime}
	if endpoint, err := s.store.GetInfraRuntimeEndpoint(ctx, runtime.ID); err == nil {
		result.Endpoint = endpoint
	}
	if provider != nil && len(provider.Outputs) > 0 {
		result.Outputs = provider.Outputs
	} else {
		outputs, _ := s.store.ListInfraRuntimeOutputs(ctx, runtime.ID)
		s.enrichOutputs(outputs)
		result.Outputs = outputs
	}
	if provider != nil {
		result.ArtifactDigest = provider.ArtifactDigest
	}
	return result, nil
}
func requestFingerprint(req *iapiserver.InfraCreateRuntimeRequest) (string, error) {
	copy := *req
	copy.RequestID = ""
	copy.FunctionArguments = nil
	raw, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return iapiserver.InfraContentDigestSHA256Prefix + hex.EncodeToString(sum[:]), nil
}
func endpointFromResult(runtime *iapiserver.InfraRuntime, req *iapiserver.InfraCreateRuntimeRequest, result *providers.ProviderResult) *iapiserver.InfraRuntimeEndpoint {
	if result.EndpointDisplayRef == "" && result.Endpoint == nil {
		return nil
	}
	visibility := req.EndpointVisibility
	if visibility == "" {
		visibility = iapiserver.InfraEndpointVisibilityInternal
	}
	endpointName := ""
	if req.EndpointRequest != nil {
		endpointName = req.EndpointRequest.EndpointName
	}
	return &iapiserver.InfraRuntimeEndpoint{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, RuntimeID: runtime.ID, EndpointName: endpointName, Visibility: visibility, Status: iapiserver.InfraRuntimeEndpointStatusReady, DisplayRef: result.EndpointDisplayRef}
}

func prepareProviderOutputs(runtimeID string, result *providers.ProviderResult, declarations []iapiserver.InfraRuntimeOutputDeclaration) error {
	if result == nil {
		return fmt.Errorf("provider returned no runtime result")
	}
	declared := make(map[string]iapiserver.InfraRuntimeOutputDeclaration, len(declarations))
	for _, declaration := range declarations {
		declared[declaration.OutputKey] = declaration
	}
	byKey := make(map[string]*iapiserver.InfraRuntimeOutput, len(result.Outputs)+len(declarations))
	for _, output := range result.Outputs {
		if output == nil || output.OutputKey == "" {
			return fmt.Errorf("provider returned an invalid output descriptor")
		}
		if _, exists := byKey[output.OutputKey]; exists {
			return fmt.Errorf("provider returned duplicate output %q", output.OutputKey)
		}
		if len(declared) > 0 {
			declaration, ok := declared[output.OutputKey]
			if !ok {
				return fmt.Errorf("provider returned undeclared output %q", output.OutputKey)
			}
			if output.MediaType != "" && output.MediaType != declaration.MediaType {
				return fmt.Errorf("collected output %q media type does not match", output.OutputKey)
			}
		}
		if output.ID == "" {
			output.ID = uuid.NewString()
		}
		output.RuntimeID = runtimeID
		if output.Status == "" {
			output.Status = iapiserver.InfraRuntimeOutputStatusPending
		}
		byKey[output.OutputKey] = output
	}
	for _, declaration := range declarations {
		if _, exists := byKey[declaration.OutputKey]; exists {
			continue
		}
		output := &iapiserver.InfraRuntimeOutput{
			ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, RuntimeID: runtimeID,
			OutputKey: declaration.OutputKey, Status: iapiserver.InfraRuntimeOutputStatusPending, MediaType: declaration.MediaType,
		}
		result.Outputs = append(result.Outputs, output)
		byKey[declaration.OutputKey] = output
	}
	for key, content := range result.OutputContents {
		output := byKey[key]
		if output == nil || content.Open == nil || content.SizeBytes < 0 || !validSHA256(content.ContentDigest) || content.MediaType == "" {
			return fmt.Errorf("collected output %q is invalid", key)
		}
		if output.MediaType != "" && output.MediaType != content.MediaType {
			return fmt.Errorf("collected output %q media type does not match", key)
		}
		if content.CollectedAt.IsZero() {
			content.CollectedAt = time.Now()
			result.OutputContents[key] = content
		}
		collectedAt := imachinery.NewTime(content.CollectedAt)
		output.Status = iapiserver.InfraRuntimeOutputStatusCollected
		output.MediaType = content.MediaType
		output.SizeBytes = content.SizeBytes
		output.ContentDigest = content.ContentDigest
		output.CollectedAt = &collectedAt
	}
	if result.Status == iapiserver.InfraRuntimeStatusSucceeded {
		for _, declaration := range declarations {
			if _, ok := result.OutputContents[declaration.OutputKey]; !ok {
				return fmt.Errorf("declared output %q was not collected", declaration.OutputKey)
			}
		}
	}
	return nil
}

func (s *Service) rememberProviderState(endpoint *iapiserver.InfraRuntimeEndpoint, result *providers.ProviderResult) {
	if result == nil {
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if endpoint != nil && result.Endpoint != nil {
		s.endpoints[endpoint.ID] = *result.Endpoint
	}
	for _, output := range result.Outputs {
		if output == nil || output.ID == "" {
			continue
		}
		content, ok := result.OutputContents[output.OutputKey]
		if !ok {
			continue
		}
		output.ContentRef = iapiserver.InfraRefPrefixOutput + output.ID
		s.outputs[output.ID] = content
	}
}

func (s *Service) enrichOutputs(outputs []*iapiserver.InfraRuntimeOutput) {
	for _, output := range outputs {
		s.enrichOutput(output)
	}
}

func (s *Service) enrichOutput(output *iapiserver.InfraRuntimeOutput) {
	if output == nil {
		return
	}
	content, ok := s.getOutput(output.ID)
	if !ok {
		return
	}
	output.MediaType = content.MediaType
	output.SizeBytes = content.SizeBytes
	output.ContentDigest = content.ContentDigest
	output.ContentRef = iapiserver.InfraRefPrefixOutput + output.ID
	if !content.CollectedAt.IsZero() {
		collectedAt := imachinery.NewTime(content.CollectedAt)
		output.CollectedAt = &collectedAt
	}
}

func validSHA256(value string) bool {
	if len(value) != len(iapiserver.InfraContentDigestSHA256Prefix)+64 || !strings.HasPrefix(value, iapiserver.InfraContentDigestSHA256Prefix) {
		return false
	}
	for _, char := range value[len(iapiserver.InfraContentDigestSHA256Prefix):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
