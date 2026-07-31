package engine

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sets"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/helpers"
)

// Adapter 对 EngineInstance 执行协议级健康探测。
type Adapter interface {
	ID() string
	Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error)
}

// ComfyUIObjectInfoReader 读取 ComfyUI 实例当前节点目录，不提交工作流。
type ComfyUIObjectInfoReader interface {
	ReadObjectInfo(context.Context, *iapiserver.EngineInstance) (map[string]any, error)
}

// ComfyUIVersionReader 读取 ComfyUI 实例版本元数据。
type ComfyUIVersionReader interface {
	ReadComfyUIVersion(context.Context, *iapiserver.EngineInstance) (string, error)
}

// OperationExecutor 将不可变 ApplicationRun 快照翻译为具体引擎协议调用。
type OperationExecutor interface {
	ID() string
	Execute(context.Context, *iapiserver.EngineInstance, *iapiserver.ApplicationRun) (map[string]any, error)
}

// Srv 提供 Model Gateway 引擎实例、能力绑定、健康状态与 ComfyUI 目录服务。
type Srv interface {
	ListApplicationEngineTypes(context.Context, *iapiserver.ApplicationEngineTypeListRequest) (*iapiserver.ApplicationEngineTypeListResponse, error)
	ListEngineInstances(context.Context, *iapiserver.EngineInstanceListRequest) (*iapiserver.EngineInstanceListResponse, error)
	CreateEngineInstance(context.Context, *iapiserver.EngineInstanceCreateRequest) (*iapiserver.EngineInstance, error)
	GetEngineInstance(context.Context, string) (*iapiserver.EngineInstance, error)
	UpdateEngineInstance(context.Context, *iapiserver.EngineInstanceUpdateRequest) (*iapiserver.EngineInstance, error)
	DeleteEngineInstance(context.Context, string) (*iapiserver.DeleteResult, error)
	CheckEngineInstanceHealth(context.Context, string) (*iapiserver.EngineHealthCheckResult, error)
	CheckEngineInstanceHealthInternal(context.Context, string) (*iapiserver.EngineHealthCheckResult, error)
	GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoResponse, error)
	RefreshComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error)
	RefreshComfyUIEngineObjectInfoInternal(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error)
	ListEngineBindings(context.Context, *iapiserver.EngineCapabilityBindingListRequest) (*iapiserver.EngineCapabilityBindingListResponse, error)
	CreateEngineBinding(context.Context, *iapiserver.EngineCapabilityBindingCreateRequest) (*iapiserver.EngineCapabilityBinding, error)
	UpdateEngineBinding(context.Context, *iapiserver.EngineCapabilityBindingUpdateRequest) (*iapiserver.EngineCapabilityBinding, error)
	DeleteEngineBinding(context.Context, string) (*iapiserver.DeleteResult, error)
	ReconcileRequiredEngineBindings(context.Context) error
	ResolveBindingStatus(*iapiserver.EngineCapabilityBinding)
	ResolveUsableComfyUIObjectInfo(context.Context, string) (*iapiserver.EngineInstance, *iapiserver.ComfyUIEngineObjectInfo, error)
}

// Dependencies 声明引擎服务所需的 store、注册表、身份与适配器边界。
type Dependencies struct {
	Store        store.Factory
	Runtime      *appregistry.RuntimeRegistry
	Capabilities *appregistry.ProviderCapabilityRegistry
	Principals   modelgateway.PrincipalResolver
	Adapters     map[string]Adapter
}

// Service 实现 Model Gateway 的引擎管理与运行时诊断服务。
type Service struct {
	store        store.Factory
	runtime      *appregistry.RuntimeRegistry
	capabilities *appregistry.ProviderCapabilityRegistry
	principals   modelgateway.PrincipalResolver
	adapters     map[string]Adapter
}

// NewService 构造引擎服务；注册表与 adapter 均由 bootstrap 显式注入。
func NewService(deps Dependencies) (*Service, error) {
	if deps.Store == nil || deps.Runtime == nil || deps.Capabilities == nil || deps.Principals == nil {
		return nil, fmt.Errorf("model gateway engine store, registries, and principal resolver are required")
	}
	if deps.Adapters == nil {
		deps.Adapters = map[string]Adapter{}
	}
	return &Service{
		store: deps.Store, runtime: deps.Runtime, capabilities: deps.Capabilities,
		principals: deps.Principals, adapters: deps.Adapters,
	}, nil
}

func (s *Service) principal(ctx context.Context, admin bool) (modelgateway.Principal, error) {
	principal, err := s.principals.Resolve(ctx)
	if err != nil {
		return modelgateway.Principal{}, err
	}
	if admin && !principal.Admin {
		return modelgateway.Principal{}, errors.NewStatus(code.ErrAIAppPermissionDenied, "administrator permission is required")
	}
	return principal, nil
}

// ListApplicationEngineTypes 返回 Runtime Registry 中不可写的引擎类型定义。
func (s *Service) ListApplicationEngineTypes(ctx context.Context, req *iapiserver.ApplicationEngineTypeListRequest) (*iapiserver.ApplicationEngineTypeListResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	items := s.runtime.EngineTypes()
	filtered := items[:0]
	for _, item := range items {
		if helpers.MatchesKeyword(req.Keyword, item.ID, item.Name) {
			filtered = append(filtered, item)
		}
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, err
	}
	return &iapiserver.ApplicationEngineTypeListResponse{Total: len(filtered), Items: imachinery.PaginateSlice(filtered, window)}, nil
}

// ListEngineInstances 返回已脱敏的引擎实例摘要，不返回 AuthConfig。
func (s *Service) ListEngineInstances(ctx context.Context, req *iapiserver.EngineInstanceListRequest) (*iapiserver.EngineInstanceListResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	items, total, err := s.store.ApplicationPlatforms().ListEngineInstances(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	summaries := make([]*iapiserver.EngineInstanceSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, item.Summary())
	}
	return &iapiserver.EngineInstanceListResponse{Total: total, Items: summaries}, nil
}

// CreateEngineInstance 原子创建引擎实例及该类型全部系统必需的不可变能力绑定。
func (s *Service) CreateEngineInstance(ctx context.Context, req *iapiserver.EngineInstanceCreateRequest) (*iapiserver.EngineInstance, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	if req.Enabled == nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "enabled is required")
	}
	if err := s.validateAuth(req.ApplicationEngineTypeID, req.AuthType, req.AuthConfig); err != nil {
		return nil, err
	}
	item := &iapiserver.EngineInstance{
		ApplicationEngineTypeID: req.ApplicationEngineTypeID, BaseURL: req.BaseURL,
		AuthType: req.AuthType, AuthConfig: req.AuthConfig, Enabled: *req.Enabled,
		HealthStatus: iapiserver.EngineHealthUnknown, Region: req.Region,
		MaxConcurrency: req.MaxConcurrency, RequestTimeoutSeconds: defaultInt(req.RequestTimeoutSeconds, 60),
		TaskTimeoutSeconds: defaultInt(req.TaskTimeoutSeconds, 1800),
	}
	item.Name, item.Description = req.Name, req.Description
	ret, err := s.store.ApplicationPlatforms().AddEngineInstanceWithBindings(ctx, item, s.requiredBindingsForEngineType(req.ApplicationEngineTypeID))
	if err == nil {
		return ret, nil
	}
	if strings.Contains(err.Error(), "idx_aiapp_engine_instances_name") {
		return nil, errors.NewStatus(code.ErrAIAppEngineInstanceNameDuplicated, "engine instance name already exists")
	}
	if stderrors.Is(err, store.ErrRequiredEngineBindingFailed) {
		return nil, errors.NewStatus(code.ErrAIAppRequiredEngineBindingFailed, "required engine capability binding could not be created")
	}
	return nil, err
}

// ReconcileRequiredEngineBindings 在进程开始服务前恢复全部系统必需的不可变绑定。
func (s *Service) ReconcileRequiredEngineBindings(ctx context.Context) error {
	for _, capability := range s.capabilities.Capabilities() {
		if capability.Kind != iapiserver.ProviderCapabilityKindEngineBinding ||
			capability.Origin != iapiserver.ProviderCapabilityOriginBuiltin ||
			capability.BindingPolicy != iapiserver.ProviderBindingPolicyRequiredImmutable ||
			capability.Availability != iapiserver.ProviderCapabilityAvailable {
			continue
		}
		if err := s.store.ApplicationPlatforms().EnsureRequiredEngineBindings(
			ctx, capability.ApplicationEngineTypeID, capability.ID, capability.Revision,
			capability.Name, "System-managed required capability binding",
		); err != nil {
			return fmt.Errorf("reconcile required engine binding %s: %w", capability.ID, err)
		}
	}
	return nil
}

func (s *Service) requiredBindingsForEngineType(engineTypeID string) []*iapiserver.EngineCapabilityBinding {
	capabilities := s.capabilities.RequiredBindingsForEngineType(engineTypeID)
	bindings := make([]*iapiserver.EngineCapabilityBinding, 0, len(capabilities))
	for _, capability := range capabilities {
		binding := &iapiserver.EngineCapabilityBinding{
			ProviderCapabilityID: capability.ID, ProviderCapabilityRevision: capability.Revision,
			Enabled: true, Restrictions: map[string]any{}, EffectiveStatus: iapiserver.BindingEffectiveAvailable,
			SystemManaged: true,
		}
		binding.Name = capability.Name
		binding.Description = "System-managed required capability binding"
		bindings = append(bindings, binding)
	}
	return bindings
}

// GetEngineInstance 返回管理员可见的完整引擎实例配置。
func (s *Service) GetEngineInstance(ctx context.Context, id string) (*iapiserver.EngineInstance, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.store.ApplicationPlatforms().GetEngineInstance(ctx, id)
	return item, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
}

// UpdateEngineInstance 按 resourceVersion 更新管理员维护的引擎连接与执行限制。
func (s *Service) UpdateEngineInstance(ctx context.Context, req *iapiserver.EngineInstanceUpdateRequest) (*iapiserver.EngineInstance, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.store.ApplicationPlatforms().GetEngineInstance(ctx, req.ID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	applyEngineUpdate(item, req)
	if err := s.validateAuth(item.ApplicationEngineTypeID, item.AuthType, item.AuthConfig); err != nil {
		return nil, err
	}
	ret, err := s.store.ApplicationPlatforms().UpdateEngineInstance(ctx, item, req.ResourceVersion)
	return ret, mapUnique(err, "idx_aiapp_engine_instances_name", code.ErrAIAppEngineInstanceNameDuplicated, "engine instance name already exists")
}

// DeleteEngineInstance 删除没有 ApplicationRun 引用的引擎实例。
func (s *Service) DeleteEngineInstance(ctx context.Context, id string) (*iapiserver.DeleteResult, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	count, err := s.store.ApplicationPlatforms().CountRunsByEngineInstance(ctx, id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.NewStatus(code.ErrAIAppEngineReferenceBlocked, "engine instance has run references")
	}
	if err := s.store.ApplicationPlatforms().DeleteEngineInstance(ctx, id); err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	return &iapiserver.DeleteResult{ID: id, Deleted: true}, nil
}

func (s *Service) validateAuth(typeID, authType string, config map[string]any) error {
	engineType, ok := s.runtime.EngineType(typeID)
	if !ok || !sets.NewGenericSet[string](engineType.AuthenticationTypes...).Has(authType) {
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "unsupported engine authentication type")
	}
	schema := engineType.AuthenticationConfigSchema[authType]
	if authType == iapiserver.EngineAuthNone {
		if len(config) != 0 {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "auth_config must be omitted when auth_type is none")
		}
		return nil
	}
	allowed := map[string]struct{}{}
	required, _ := schema["required"].([]any)
	if required == nil {
		if stringsList, ok := schema["required"].([]string); ok {
			for _, key := range stringsList {
				allowed[key] = struct{}{}
				if typeutil.As[string](config[key]) == "" {
					return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "missing authentication field "+key)
				}
			}
		}
	} else {
		for _, raw := range required {
			key, _ := raw.(string)
			allowed[key] = struct{}{}
			if typeutil.As[string](config[key]) == "" {
				return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "missing authentication field "+key)
			}
		}
	}
	for key := range config {
		if _, ok := allowed[key]; !ok {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "unknown authentication field "+key)
		}
	}
	return nil
}

var _ Srv = (*Service)(nil)
