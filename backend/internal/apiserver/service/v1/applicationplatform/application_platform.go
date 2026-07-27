package applicationplatform

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

const (
	applicationRunTaskDefinitionID = "application-platform.application-run"
	applicationRunTaskFunctionRef  = "application-platform.run"
)

// ApplicationPlatformSrv implements the public S2 Application Platform operations.
type ApplicationPlatformSrv interface {
	ListProviderCapabilities(context.Context, *iapiserver.ProviderCapabilityListRequest) (*iapiserver.ProviderCapabilityListResponse, error)
	GetProviderCapability(context.Context, string) (*iapiserver.AIAppProviderCapability, error)
	ListProviderCapabilityLoadResults(context.Context, *iapiserver.ProviderCapabilityLoadResultListRequest) (*iapiserver.ProviderCapabilityLoadResultListResponse, error)
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
	ListComfyUIWorkflows(context.Context, *iapiserver.ComfyUIWorkflowListRequest) (*iapiserver.ComfyUIWorkflowListResponse, error)
	ImportComfyUIWorkflow(context.Context, *iapiserver.ComfyUIWorkflowImportRequest) (*iapiserver.ComfyUIWorkflowImportResult, error)
	GetComfyUIWorkflow(context.Context, string) (*iapiserver.ComfyUIWorkflowDetail, error)
	UpdateComfyUIWorkflow(context.Context, *iapiserver.ComfyUIWorkflowUpdateRequest) (*iapiserver.ComfyUIWorkflowSummary, error)
	ListComfyUIWorkflowNodes(context.Context, string, *iapiserver.ComfyUIWorkflowDeriveRequest) (*iapiserver.ComfyUIWorkflowNodeListResponse, error)
	ListComfyUIWorkflowInputCandidates(context.Context, string, string) (*iapiserver.ComfyUIWorkflowInputCandidateListResponse, error)
	ListComfyUIWorkflowOutputCandidates(context.Context, string, string) (*iapiserver.ComfyUIWorkflowOutputCandidateListResponse, error)
	ListComfyUIWorkflowDependencies(context.Context, string, string) (*iapiserver.ComfyUIWorkflowDependencyListResponse, error)
	ListComfyUIWorkflowValidations(context.Context, *iapiserver.ComfyUIWorkflowValidationListRequest) (*iapiserver.ComfyUIWorkflowValidationListResponse, error)
	ValidateComfyUIWorkflow(context.Context, string, *iapiserver.ComfyUIWorkflowValidationCreateRequest) (*iapiserver.ComfyUIWorkflowValidation, error)
	GetComfyUIWorkflowValidation(context.Context, string) (*iapiserver.ComfyUIWorkflowValidation, error)
	ConvertComfyUIWorkflow(context.Context, string, *iapiserver.ComfyUIWorkflowConvertRequest) (*iapiserver.ComfyUIWorkflowConvertResult, error)
	ConvertComfyUIWorkflowToAPI(context.Context, string, *iapiserver.ComfyUIWorkflowAPIConversionRequest) (*iapiserver.ComfyUIWorkflowDetail, error)
	ListComfyUIWorkflowTestRuns(context.Context, *iapiserver.ComfyUIWorkflowTestRunListRequest) (*iapiserver.ComfyUIWorkflowTestRunListResponse, error)
	CreateComfyUIWorkflowTestRun(context.Context, string, *iapiserver.ComfyUIWorkflowTestRunCreateRequest) (*iapiserver.ComfyUIWorkflowTestRun, error)
	GetComfyUIWorkflowTestRun(context.Context, string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	CancelComfyUIWorkflowTestRun(context.Context, string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	GetComfyUIWorkflowTestOutputContent(context.Context, string, string) ([]byte, string, error)
	ListEngineBindings(context.Context, *iapiserver.EngineCapabilityBindingListRequest) (*iapiserver.EngineCapabilityBindingListResponse, error)
	CreateEngineBinding(context.Context, *iapiserver.EngineCapabilityBindingCreateRequest) (*iapiserver.EngineCapabilityBinding, error)
	UpdateEngineBinding(context.Context, *iapiserver.EngineCapabilityBindingUpdateRequest) (*iapiserver.EngineCapabilityBinding, error)
	DeleteEngineBinding(context.Context, string) (*iapiserver.DeleteResult, error)
	ListTemplates(context.Context, *iapiserver.ApplicationTemplateListRequest) (*iapiserver.ApplicationTemplateListResponse, error)
	CreateTemplate(context.Context, *iapiserver.ApplicationTemplateCreateRequest) (*iapiserver.ApplicationTemplate, error)
	GetTemplate(context.Context, string) (*iapiserver.ApplicationTemplate, error)
	ListTemplateVersions(context.Context, *iapiserver.ApplicationTemplateVersionListRequest) (*iapiserver.ApplicationTemplateVersionListResponse, error)
	CreateTemplateVersion(context.Context, string, *iapiserver.ApplicationTemplateVersionCreateRequest) (*iapiserver.ApplicationTemplateVersion, error)
	GetTemplateVersion(context.Context, string) (*iapiserver.ApplicationTemplateVersion, error)
	PublishTemplateVersion(context.Context, string) (*iapiserver.ApplicationTemplateVersion, error)
	ListApplications(context.Context, *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error)
	CreateApplication(context.Context, *iapiserver.ApplicationCreateRequest) (*iapiserver.Application, error)
	GetApplication(context.Context, string) (*iapiserver.Application, error)
	UpdateApplication(context.Context, *iapiserver.ApplicationUpdateRequest) (*iapiserver.Application, error)
	ListApplicationVersions(context.Context, *iapiserver.ApplicationVersionListRequest) (*iapiserver.ApplicationVersionListResponse, error)
	CreateApplicationVersion(context.Context, string, *iapiserver.ApplicationVersionCreateRequest) (*iapiserver.ApplicationVersion, error)
	GetApplicationVersion(context.Context, string) (*iapiserver.ApplicationVersion, error)
	PublishApplicationVersion(context.Context, string) (*iapiserver.ApplicationVersion, error)
	ResolveRuntimeForm(context.Context, string, *iapiserver.RuntimeFormResolveRequest) (*iapiserver.RuntimeFormSchema, error)
	CreateApplicationRun(context.Context, string, *iapiserver.ApplicationRunCreateRequest) (*iapiserver.ApplicationRun, error)
	GetApplicationRun(context.Context, string) (*iapiserver.ApplicationRun, error)
}

type Principal struct {
	UserID string
	Admin  bool
}

type PrincipalResolver interface {
	Resolve(context.Context) (Principal, error)
}
type EngineAdapter interface {
	ID() string
	Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error)
}
type ComfyUIObjectInfoReader interface {
	ReadObjectInfo(context.Context, *iapiserver.EngineInstance) (map[string]any, error)
}
type ComfyUIVersionReader interface {
	ReadComfyUIVersion(context.Context, *iapiserver.EngineInstance) (string, error)
}
type OperationExecutor interface {
	ID() string
	Execute(context.Context, *iapiserver.EngineInstance, *iapiserver.ApplicationRun) (map[string]any, error)
}
type AssetRegistrar interface {
	Register(context.Context, *iapiserver.ApplicationArtifact) (string, error)
}
type EventPublisher interface {
	Publish(context.Context, *iapiserver.ApplicationPlatformEvent) error
}
type WorkflowAuditor interface {
	Record(context.Context, WorkflowAuditRecord) error
}
type WorkflowAuditRecord struct {
	Action      string
	ActorUserID string
	OwnerUserID string
	WorkflowID  string
	Result      string
	OccurredAt  imachinery.Time
}

type Dependencies struct {
	Store          store.Factory
	Runtime        *appregistry.RuntimeRegistry
	Capabilities   *appregistry.ProviderCapabilityRegistry
	Principals     PrincipalResolver
	Adapters       map[string]EngineAdapter
	Executors      map[string]OperationExecutor
	Tasks          taskcenter.TaskCenterSrv
	Assets         AssetRegistrar
	Events         EventPublisher
	WorkflowAudit  WorkflowAuditor
	WorkflowParser ComfyWorkflowParser
}

type applicationPlatformService struct{ Dependencies }

func NewService(deps Dependencies) (*applicationPlatformService, error) {
	if deps.Store == nil || deps.Runtime == nil || deps.Capabilities == nil {
		return nil, fmt.Errorf("application platform store and registries are required")
	}
	if deps.Principals == nil {
		deps.Principals = &storePrincipalResolver{store: deps.Store}
	}
	if deps.Adapters == nil {
		deps.Adapters = map[string]EngineAdapter{}
	}
	if deps.Executors == nil {
		deps.Executors = map[string]OperationExecutor{}
	}
	if deps.Events == nil {
		deps.Events = NoopEventPublisher{}
	}
	if deps.Assets == nil {
		deps.Assets = NoopAssetRegistrar{}
	}
	if deps.WorkflowAudit == nil {
		deps.WorkflowAudit = StructuredWorkflowAuditor{}
	}
	if deps.WorkflowParser == nil {
		deps.WorkflowParser = comfy2GoWorkflowParser{}
	}
	return &applicationPlatformService{Dependencies: deps}, nil
}

type NoopEventPublisher struct{}

func (NoopEventPublisher) Publish(context.Context, *iapiserver.ApplicationPlatformEvent) error {
	return nil
}

type NoopAssetRegistrar struct{}

func (NoopAssetRegistrar) Register(context.Context, *iapiserver.ApplicationArtifact) (string, error) {
	return "", errors.New("asset registrar is not configured")
}

// StructuredWorkflowAuditor emits the mandatory managed-access audit fields for identity log ingestion.
type StructuredWorkflowAuditor struct{}

func (StructuredWorkflowAuditor) Record(_ context.Context, record WorkflowAuditRecord) error {
	log.Infof("ComfyUI workflow audit: action=%s actor_user_id=%s owner_user_id=%s workflow_id=%s result=%s occurred_at=%s", record.Action, record.ActorUserID, record.OwnerUserID, record.WorkflowID, record.Result, record.OccurredAt.String())
	return nil
}

type storePrincipalResolver struct{ store store.Factory }

func (r *storePrincipalResolver) Resolve(ctx context.Context) (Principal, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return Principal{}, errors.NewStatus(code.ErrAIAppPermissionDenied, "authenticated user is required")
	}
	principal := Principal{UserID: user.ID, Admin: user.ID == "system-admin"}
	if principal.Admin {
		return principal, nil
	}
	assignments, err := r.store.UserRoles().ListByUser(ctx, user.ID)
	if err != nil {
		return Principal{}, errors.WithStack(err)
	}
	roles, err := r.store.Roles().List(ctx)
	if err != nil {
		return Principal{}, errors.WithStack(err)
	}
	roleNames := make(map[string]string, len(roles))
	for _, role := range roles {
		roleNames[role.ID] = strings.ToUpper(role.Name)
	}
	for _, assignment := range assignments {
		if name := roleNames[assignment.RoleID]; name == "ADMIN" || name == "SUPER_ADMIN" {
			principal.Admin = true
			break
		}
	}
	return principal, nil
}

func (s *applicationPlatformService) principal(ctx context.Context, admin bool) (Principal, error) {
	p, err := s.Principals.Resolve(ctx)
	if err != nil {
		return Principal{}, err
	}
	if admin && !p.Admin {
		return Principal{}, errors.NewStatus(code.ErrAIAppPermissionDenied, "administrator permission is required")
	}
	return p, nil
}

func (s *applicationPlatformService) ListProviderCapabilities(ctx context.Context, req *iapiserver.ProviderCapabilityListRequest) (*iapiserver.ProviderCapabilityListResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	items := s.Capabilities.Capabilities()
	filtered := items[:0]
	for _, item := range items {
		if req.ApplicationEngineTypeID != "" && item.ApplicationEngineTypeID != req.ApplicationEngineTypeID {
			continue
		}
		if req.Availability != "" && item.Availability != req.Availability {
			continue
		}
		if !matchesKeyword(req.Keyword, item.Name, item.Description, item.ID) {
			continue
		}
		filtered = append(filtered, item)
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, err
	}
	page := imachinery.PaginateSlice(filtered, window)
	return &iapiserver.ProviderCapabilityListResponse{Total: len(filtered), RegistryStatus: s.Capabilities.Status(), Items: page}, nil
}

func (s *applicationPlatformService) GetProviderCapability(ctx context.Context, id string) (*iapiserver.AIAppProviderCapability, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	item, ok := s.Capabilities.Get(id)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityNotFound, "provider capability not found")
	}
	return item, nil
}

func (s *applicationPlatformService) ListProviderCapabilityLoadResults(ctx context.Context, req *iapiserver.ProviderCapabilityLoadResultListRequest) (*iapiserver.ProviderCapabilityLoadResultListResponse, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	items := s.Capabilities.Results()
	filtered := items[:0]
	for _, item := range items {
		if req.Result == "" || item.Result == req.Result {
			filtered = append(filtered, item)
		}
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, err
	}
	return &iapiserver.ProviderCapabilityLoadResultListResponse{Total: len(filtered), RegistryStatus: s.Capabilities.Status(), Items: imachinery.PaginateSlice(filtered, window)}, nil
}

func (s *applicationPlatformService) ListApplicationEngineTypes(ctx context.Context, req *iapiserver.ApplicationEngineTypeListRequest) (*iapiserver.ApplicationEngineTypeListResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	items := s.Runtime.EngineTypes()
	filtered := items[:0]
	for _, item := range items {
		if matchesKeyword(req.Keyword, item.ID, item.Name) {
			filtered = append(filtered, item)
		}
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, err
	}
	return &iapiserver.ApplicationEngineTypeListResponse{Total: len(filtered), Items: imachinery.PaginateSlice(filtered, window)}, nil
}

func (s *applicationPlatformService) ListEngineInstances(ctx context.Context, req *iapiserver.EngineInstanceListRequest) (*iapiserver.EngineInstanceListResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	items, total, err := s.Store.ApplicationPlatforms().ListEngineInstances(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	summaries := make([]*iapiserver.EngineInstanceSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, item.Summary())
	}
	return &iapiserver.EngineInstanceListResponse{Total: total, Items: summaries}, nil
}

func (s *applicationPlatformService) CreateEngineInstance(ctx context.Context, req *iapiserver.EngineInstanceCreateRequest) (*iapiserver.EngineInstance, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	if req.Enabled == nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "enabled is required")
	}
	if err := s.validateEngineAuth(req.ApplicationEngineTypeID, req.AuthType, req.AuthConfig); err != nil {
		return nil, err
	}
	item := &iapiserver.EngineInstance{ApplicationEngineTypeID: req.ApplicationEngineTypeID, BaseURL: req.BaseURL, AuthType: req.AuthType, AuthConfig: req.AuthConfig, Enabled: *req.Enabled, HealthStatus: iapiserver.EngineHealthUnknown, Region: req.Region, MaxConcurrency: req.MaxConcurrency, RequestTimeoutSeconds: defaultInt(req.RequestTimeoutSeconds, 60), TaskTimeoutSeconds: defaultInt(req.TaskTimeoutSeconds, 1800)}
	item.Name, item.Description = req.Name, req.Description
	bindings := s.requiredBindingsForEngineType(req.ApplicationEngineTypeID)
	ret, err := s.Store.ApplicationPlatforms().AddEngineInstanceWithBindings(ctx, item, bindings)
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
func (s *applicationPlatformService) ReconcileRequiredEngineBindings(ctx context.Context) error {
	for _, capability := range s.Capabilities.Capabilities() {
		if capability.Kind != iapiserver.ProviderCapabilityKindEngineBinding || capability.Origin != iapiserver.ProviderCapabilityOriginBuiltin || capability.BindingPolicy != iapiserver.ProviderBindingPolicyRequiredImmutable || capability.Availability != iapiserver.ProviderCapabilityAvailable {
			continue
		}
		if err := s.Store.ApplicationPlatforms().EnsureRequiredEngineBindings(ctx, capability.ApplicationEngineTypeID, capability.ID, capability.Revision, capability.Name, "System-managed required capability binding"); err != nil {
			return fmt.Errorf("reconcile required engine binding %s: %w", capability.ID, err)
		}
	}
	return nil
}

func (s *applicationPlatformService) requiredBindingsForEngineType(engineTypeID string) []*iapiserver.EngineCapabilityBinding {
	capabilities := s.Capabilities.RequiredBindingsForEngineType(engineTypeID)
	bindings := make([]*iapiserver.EngineCapabilityBinding, 0, len(capabilities))
	for _, capability := range capabilities {
		binding := &iapiserver.EngineCapabilityBinding{
			ProviderCapabilityID:       capability.ID,
			ProviderCapabilityRevision: capability.Revision,
			Enabled:                    true,
			Restrictions:               map[string]any{},
			EffectiveStatus:            iapiserver.BindingEffectiveAvailable,
			SystemManaged:              true,
		}
		binding.Name = capability.Name
		binding.Description = "System-managed required capability binding"
		bindings = append(bindings, binding)
	}
	return bindings
}

func (s *applicationPlatformService) GetEngineInstance(ctx context.Context, id string) (*iapiserver.EngineInstance, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, id)
	return item, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
}

func (s *applicationPlatformService) UpdateEngineInstance(ctx context.Context, req *iapiserver.EngineInstanceUpdateRequest) (*iapiserver.EngineInstance, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, req.ID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	applyEngineUpdate(item, req)
	if err := s.validateEngineAuth(item.ApplicationEngineTypeID, item.AuthType, item.AuthConfig); err != nil {
		return nil, err
	}
	ret, err := s.Store.ApplicationPlatforms().UpdateEngineInstance(ctx, item, req.ResourceVersion)
	return ret, mapUnique(err, "idx_aiapp_engine_instances_name", code.ErrAIAppEngineInstanceNameDuplicated, "engine instance name already exists")
}

func (s *applicationPlatformService) DeleteEngineInstance(ctx context.Context, id string) (*iapiserver.DeleteResult, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	count, err := s.Store.ApplicationPlatforms().CountRunsByEngineInstance(ctx, id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.NewStatus(code.ErrAIAppEngineReferenceBlocked, "engine instance has run references")
	}
	if err := s.Store.ApplicationPlatforms().DeleteEngineInstance(ctx, id); err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	return &iapiserver.DeleteResult{ID: id, Deleted: true}, nil
}

func (s *applicationPlatformService) CheckEngineInstanceHealth(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	return s.checkEngineInstanceHealth(ctx, id)
}

// CheckEngineInstanceHealthInternal 为受信任的 TaskWorker 执行健康探测，不经过 HTTP 用户鉴权。
func (s *applicationPlatformService) CheckEngineInstanceHealthInternal(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	return s.checkEngineInstanceHealth(ctx, id)
}

func (s *applicationPlatformService) checkEngineInstanceHealth(ctx context.Context, id string) (*iapiserver.EngineHealthCheckResult, error) {
	item, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	now := imachinery.Now()
	var result *iapiserver.EngineHealthCheckResult
	typeDef, ok := s.Runtime.EngineType(item.ApplicationEngineTypeID)
	if !ok {
		result = degradedEngineHealth(id, now, "engine type is not registered")
	} else if adapter := s.Adapters[typeDef.EngineAdapterID]; adapter == nil {
		result = degradedEngineHealth(id, now, "engine adapter is not registered")
	} else {
		result, err = adapter.Check(ctx, item)
		if err != nil {
			// Worker shutdown must leave the current chunk retryable; a bounded detection deadline is a valid offline observation.
			if stderrors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}
			result = engineHealthFromError(id, now, err)
		}
	}
	result = normalizeEngineHealthResult(id, now, result)
	old := item.HealthStatus
	item.LastHealthCheckAt = &now
	item.HealthStatus = result.HealthStatus
	item.UnhealthyReason = result.FailureSummary
	var event *iapiserver.ApplicationPlatformEvent
	if old != item.HealthStatus {
		payload := map[string]any{"engine_instance_id": id, "application_engine_type_id": item.ApplicationEngineTypeID, "health_status": item.HealthStatus, "checked_at": now, "failure_summary": item.UnhealthyReason}
		event = &iapiserver.ApplicationPlatformEvent{Type: "engine_instance_health_changed", IdempotencyKey: id + ":" + now.String(), Payload: payload, OccurredAt: now}
	}
	_, err = s.Store.ApplicationPlatforms().UpdateEngineInstanceHealth(ctx, item, item.ResourceVersion, event)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func engineHealthFromError(id string, checkedAt imachinery.Time, err error) *iapiserver.EngineHealthCheckResult {
	status := errors.ToStatus(err)
	switch status.Code {
	case code.ErrAIAppEngineAuthConfigInvalid:
		return degradedEngineHealth(id, checkedAt, "provider authentication failed")
	case code.ErrAIAppProviderRuntimeCapabilityMismatch:
		return degradedEngineHealth(id, checkedAt, "provider health protocol is incompatible")
	case code.ErrAIAppEngineUnavailable:
		switch {
		case strings.Contains(status.Desc, "timed out"):
			return offlineEngineHealth(id, checkedAt, "provider request timed out")
		case strings.Contains(status.Desc, "base URL"), strings.Contains(status.Desc, "could not be parsed"):
			return degradedEngineHealth(id, checkedAt, "provider health protocol is incompatible")
		default:
			return offlineEngineHealth(id, checkedAt, "provider is unavailable")
		}
	default:
		return degradedEngineHealth(id, checkedAt, "engine health adapter failed")
	}
}

func normalizeEngineHealthResult(id string, checkedAt imachinery.Time, result *iapiserver.EngineHealthCheckResult) *iapiserver.EngineHealthCheckResult {
	if result == nil {
		return degradedEngineHealth(id, checkedAt, "engine health adapter failed")
	}
	result.EngineInstanceID, result.CheckedAt = id, checkedAt
	switch result.HealthStatus {
	case iapiserver.EngineHealthOnline:
		result.FailureSummary = ""
	case iapiserver.EngineHealthOffline:
		result.FailureSummary = safeEngineHealthSummary(result.FailureSummary, "provider is unavailable")
	case iapiserver.EngineHealthDegraded:
		result.FailureSummary = safeEngineHealthSummary(result.FailureSummary, "engine health protocol is degraded")
	default:
		return degradedEngineHealth(id, checkedAt, "engine health adapter failed")
	}
	return result
}

func safeEngineHealthSummary(summary, fallback string) string {
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

func degradedEngineHealth(id string, checkedAt imachinery.Time, summary string) *iapiserver.EngineHealthCheckResult {
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: id, HealthStatus: iapiserver.EngineHealthDegraded, CheckedAt: checkedAt, FailureSummary: summary}
}

func offlineEngineHealth(id string, checkedAt imachinery.Time, summary string) *iapiserver.EngineHealthCheckResult {
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: id, HealthStatus: iapiserver.EngineHealthOffline, CheckedAt: checkedAt, FailureSummary: summary}
}

func (s *applicationPlatformService) ListEngineBindings(ctx context.Context, req *iapiserver.EngineCapabilityBindingListRequest) (*iapiserver.EngineCapabilityBindingListResponse, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	items, total, err := s.Store.ApplicationPlatforms().ListEngineBindings(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		s.resolveBindingStatus(item)
	}
	return &iapiserver.EngineCapabilityBindingListResponse{Total: total, Items: items}, nil
}

func (s *applicationPlatformService) CreateEngineBinding(ctx context.Context, req *iapiserver.EngineCapabilityBindingCreateRequest) (*iapiserver.EngineCapabilityBinding, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	if req.Enabled == nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineBindingIncompatible, "enabled is required")
	}
	engine, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, req.EngineInstanceID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	capability, ok := s.Capabilities.Get(req.ProviderCapabilityID)
	if !ok || capability.Availability != iapiserver.ProviderCapabilityAvailable {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability is unavailable")
	}
	if capability.BindingPolicy == iapiserver.ProviderBindingPolicyRequiredImmutable {
		return nil, errors.NewStatus(code.ErrAIAppSystemEngineBindingImmutable, "system-managed engine binding cannot be created")
	}
	if capability.ApplicationEngineTypeID != engine.ApplicationEngineTypeID {
		return nil, errors.NewStatus(code.ErrAIAppEngineBindingIncompatible, "engine type does not match capability")
	}
	if err := validateRestrictions(capability, req.Restrictions); err != nil {
		return nil, err
	}
	item := &iapiserver.EngineCapabilityBinding{EngineInstanceID: engine.ID, ProviderCapabilityID: capability.ID, ProviderCapabilityRevision: capability.Revision, Enabled: *req.Enabled, Restrictions: req.Restrictions, EffectiveStatus: iapiserver.BindingEffectiveAvailable}
	item.Name, item.Description = req.Name, req.Description
	ret, err := s.Store.ApplicationPlatforms().AddEngineBinding(ctx, item)
	return ret, mapUnique(err, "idx_aiapp_binding_engine_capability", code.ErrAIAppEngineBindingIncompatible, "binding already exists")
}

func (s *applicationPlatformService) UpdateEngineBinding(ctx context.Context, req *iapiserver.EngineCapabilityBindingUpdateRequest) (*iapiserver.EngineCapabilityBinding, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetEngineBinding(ctx, req.ID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineBindingNotFound, "engine binding not found")
	}
	if s.isSystemManagedBinding(item) {
		return nil, errors.NewStatus(code.ErrAIAppSystemEngineBindingImmutable, "system-managed engine binding cannot be updated")
	}
	capability, ok := s.Capabilities.Get(item.ProviderCapabilityID)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability is unavailable")
	}
	if req.Restrictions != nil {
		if err := validateRestrictions(capability, req.Restrictions); err != nil {
			return nil, err
		}
		item.Restrictions = req.Restrictions
	}
	applyString(&item.Name, req.Name)
	applyString(&item.Description, req.Description)
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	ret, err := s.Store.ApplicationPlatforms().UpdateEngineBinding(ctx, item, req.ResourceVersion)
	if ret != nil {
		s.resolveBindingStatus(ret)
	}
	return ret, err
}

func (s *applicationPlatformService) DeleteEngineBinding(ctx context.Context, id string) (*iapiserver.DeleteResult, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetEngineBinding(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineBindingNotFound, "engine binding not found")
	}
	if s.isSystemManagedBinding(item) {
		return nil, errors.NewStatus(code.ErrAIAppSystemEngineBindingImmutable, "system-managed engine binding cannot be deleted")
	}
	if err := s.Store.ApplicationPlatforms().DeleteEngineBinding(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.DeleteResult{ID: id, Deleted: true}, nil
}

func (s *applicationPlatformService) ListTemplates(ctx context.Context, req *iapiserver.ApplicationTemplateListRequest) (*iapiserver.ApplicationTemplateListResponse, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	if !p.Admin {
		req.OwnerUserID = p.UserID
	}
	items, total, err := s.Store.ApplicationPlatforms().ListTemplates(ctx, req)
	return &iapiserver.ApplicationTemplateListResponse{Total: total, Items: items}, err
}
func (s *applicationPlatformService) GetTemplate(ctx context.Context, id string) (*iapiserver.ApplicationTemplate, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetTemplate(ctx, id)
	if err != nil || (!p.Admin && item.OwnerUserID != p.UserID) {
		return nil, errors.NewStatus(code.ErrAIAppTemplateNotFound, "application template not found")
	}
	return item, nil
}

func (s *applicationPlatformService) CreateTemplate(ctx context.Context, req *iapiserver.ApplicationTemplateCreateRequest) (*iapiserver.ApplicationTemplate, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	version, err := s.templateVersionFromRequest(req.CapabilityDefinitionID, req.CapabilitySourceType, req.ProviderCapabilityID, req.ProviderOperationID, nil, req.TemplateContract)
	if err != nil {
		return nil, err
	}
	template := &iapiserver.ApplicationTemplate{OwnerUserID: p.UserID, CapabilitySourceType: req.CapabilitySourceType, CapabilityDefinitionID: req.CapabilityDefinitionID}
	template.Name, template.Description = req.Name, req.Description
	version.Version = 1
	version.Status = iapiserver.VersionStatusDraft
	version.Name = req.Name + " v1"
	return s.Store.ApplicationPlatforms().AddTemplateWithVersion(ctx, template, version)
}

func (s *applicationPlatformService) ListTemplateVersions(ctx context.Context, req *iapiserver.ApplicationTemplateVersionListRequest) (*iapiserver.ApplicationTemplateVersionListResponse, error) {
	if _, err := s.GetTemplate(ctx, req.ApplicationTemplateID); err != nil {
		return nil, err
	}
	items, total, err := s.Store.ApplicationPlatforms().ListTemplateVersions(ctx, req)
	return &iapiserver.ApplicationTemplateVersionListResponse{Total: total, Items: items}, err
}

func (s *applicationPlatformService) CreateTemplateVersion(ctx context.Context, templateID string, req *iapiserver.ApplicationTemplateVersionCreateRequest) (*iapiserver.ApplicationTemplateVersion, error) {
	template, err := s.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	version, err := s.templateVersionFromRequest(template.CapabilityDefinitionID, req.CapabilitySourceType, req.ProviderCapabilityID, req.ProviderOperationID, req.ComfyUIAPIWorkflow, req.TemplateContract)
	if err != nil {
		return nil, err
	}
	version.ApplicationTemplateID = templateID
	version.Status = iapiserver.VersionStatusDraft
	version.Name = req.Name
	version.Description = req.Description
	return s.Store.ApplicationPlatforms().AddTemplateVersion(ctx, version)
}

func (s *applicationPlatformService) GetTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error) {
	item, err := s.Store.ApplicationPlatforms().GetTemplateVersion(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppTemplateVersionNotFound, "template version not found")
	}
	if _, err = s.GetTemplate(ctx, item.ApplicationTemplateID); err != nil {
		return nil, err
	}
	return item, nil
}
func (s *applicationPlatformService) PublishTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error) {
	item, err := s.GetTemplateVersion(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.validateTemplateVersion(ctx, item); err != nil {
		return nil, err
	}
	ret, err := s.Store.ApplicationPlatforms().PublishTemplateVersion(ctx, id)
	return ret, mapNotFound(err, code.ErrAIAppTemplateVersionNotFound, "template version not found")
}

func (s *applicationPlatformService) ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	if !p.Admin {
		req.OwnerUserID = p.UserID
		req.IncludeGlobal = true
	}
	items, total, err := s.Store.ApplicationPlatforms().ListApplications(ctx, req)
	return &iapiserver.ApplicationListResponse{Total: total, Items: items}, err
}
func (s *applicationPlatformService) CreateApplication(ctx context.Context, req *iapiserver.ApplicationCreateRequest) (*iapiserver.Application, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, ok := s.Runtime.Capability(req.CapabilityDefinitionID); !ok {
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "capability definition is not registered")
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = iapiserver.ApplicationVisibilityPrivate
	}
	if visibility == iapiserver.ApplicationVisibilityGlobal && !p.Admin {
		return nil, errors.NewStatus(code.ErrAIAppPermissionDenied, "only administrators can create global applications")
	}
	item := &iapiserver.Application{OwnerUserID: p.UserID, CapabilityDefinitionID: req.CapabilityDefinitionID, Visibility: visibility, RunEnabled: defaultBool(req.RunEnabled, true), CanvasEnabled: defaultBool(req.CanvasEnabled, true), CopyEnabled: defaultBool(req.CopyEnabled, false), PresetEnabled: defaultBool(req.PresetEnabled, false)}
	item.Name, item.Description = req.Name, req.Description
	ret, err := s.Store.ApplicationPlatforms().AddApplication(ctx, item)
	return ret, mapUnique(err, "idx_aiapp_applications_owner_name", code.ErrAIAppResourceVersionConflict, "application name already exists")
}
func (s *applicationPlatformService) GetApplication(ctx context.Context, id string) (*iapiserver.Application, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetApplication(ctx, id)
	if err != nil || (!p.Admin && item.OwnerUserID != p.UserID && item.Visibility != iapiserver.ApplicationVisibilityGlobal) {
		return nil, errors.NewStatus(code.ErrAIAppApplicationNotFound, "application not found")
	}
	return item, nil
}
func (s *applicationPlatformService) UpdateApplication(ctx context.Context, req *iapiserver.ApplicationUpdateRequest) (*iapiserver.Application, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetApplication(ctx, req.ID)
	if err != nil || (!p.Admin && item.OwnerUserID != p.UserID) {
		return nil, errors.NewStatus(code.ErrAIAppApplicationNotFound, "application not found")
	}
	if req.Visibility != nil && *req.Visibility == iapiserver.ApplicationVisibilityGlobal && !p.Admin {
		return nil, errors.NewStatus(code.ErrAIAppPermissionDenied, "only administrators can set global visibility")
	}
	applyApplicationUpdate(item, req)
	return s.Store.ApplicationPlatforms().UpdateApplication(ctx, item, req.ResourceVersion)
}

func (s *applicationPlatformService) ListApplicationVersions(ctx context.Context, req *iapiserver.ApplicationVersionListRequest) (*iapiserver.ApplicationVersionListResponse, error) {
	if _, err := s.GetApplication(ctx, req.ApplicationID); err != nil {
		return nil, err
	}
	items, total, err := s.Store.ApplicationPlatforms().ListApplicationVersions(ctx, req)
	return &iapiserver.ApplicationVersionListResponse{Total: total, Items: items}, err
}
func (s *applicationPlatformService) CreateApplicationVersion(ctx context.Context, applicationID string, req *iapiserver.ApplicationVersionCreateRequest) (*iapiserver.ApplicationVersion, error) {
	app, err := s.managedApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	templateVersion, err := s.GetTemplateVersion(ctx, req.ApplicationTemplateVersionID)
	if err != nil {
		return nil, err
	}
	if templateVersion.Status != iapiserver.VersionStatusPublished {
		return nil, errors.NewStatus(code.ErrAIAppApplicationVersionNotPublishable, "template version is not published")
	}
	template, err := s.GetTemplate(ctx, templateVersion.ApplicationTemplateID)
	if err != nil {
		return nil, err
	}
	if template.CapabilityDefinitionID != app.CapabilityDefinitionID {
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "template capability does not match application")
	}
	item := &iapiserver.ApplicationVersion{ApplicationID: applicationID, SemanticVersion: req.SemanticVersion, Status: iapiserver.VersionStatusDraft, ApplicationTemplateVersionID: req.ApplicationTemplateVersionID, InputSchema: req.InputSchema, OutputSchema: req.OutputSchema, ParameterPolicies: req.ParameterPolicies}
	item.Name = app.Name + " " + req.SemanticVersion
	ret, err := s.Store.ApplicationPlatforms().AddApplicationVersion(ctx, item)
	return ret, mapUnique(err, "idx_aiapp_application_versions_semver", code.ErrAIAppApplicationSemanticVersionDuplicated, "semantic version already exists")
}
func (s *applicationPlatformService) GetApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error) {
	item, err := s.Store.ApplicationPlatforms().GetApplicationVersion(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppApplicationVersionNotFound, "application version not found")
	}
	if _, err = s.GetApplication(ctx, item.ApplicationID); err != nil {
		return nil, err
	}
	return item, nil
}
func (s *applicationPlatformService) PublishApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error) {
	item, err := s.GetApplicationVersion(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.managedApplication(ctx, item.ApplicationID); err != nil {
		return nil, err
	}
	template, err := s.GetTemplateVersion(ctx, item.ApplicationTemplateVersionID)
	if err != nil || template.Status != iapiserver.VersionStatusPublished {
		return nil, errors.NewStatus(code.ErrAIAppApplicationVersionNotPublishable, "referenced template version is not published")
	}
	ret, err := s.Store.ApplicationPlatforms().PublishApplicationVersion(ctx, id)
	if err == nil {
		s.publish(ctx, "application_version_published", ret.ID+":"+ret.SemanticVersion, map[string]any{"application_id": ret.ApplicationID, "application_version_id": ret.ID, "application_template_version_id": ret.ApplicationTemplateVersionID, "semantic_version": ret.SemanticVersion, "published_at": ret.PublishedAt})
	}
	return ret, err
}

func (s *applicationPlatformService) managedApplication(ctx context.Context, id string) (*iapiserver.Application, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	item, err := s.Store.ApplicationPlatforms().GetApplication(ctx, id)
	if err != nil || (!p.Admin && item.OwnerUserID != p.UserID) {
		return nil, errors.NewStatus(code.ErrAIAppApplicationNotFound, "application not found")
	}
	return item, nil
}

func (s *applicationPlatformService) ResolveRuntimeForm(ctx context.Context, applicationID string, req *iapiserver.RuntimeFormResolveRequest) (*iapiserver.RuntimeFormSchema, error) {
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	version, err := s.GetApplicationVersion(ctx, req.ApplicationVersionID)
	if err != nil {
		return nil, err
	}
	if version.ApplicationID != app.ID || version.Status != iapiserver.VersionStatusPublished {
		return nil, errors.NewStatus(code.ErrAIAppApplicationVersionNotFound, "published application version not found")
	}
	return s.resolveRuntimeForm(ctx, app, version, req)
}

func (s *applicationPlatformService) CreateApplicationRun(ctx context.Context, applicationID string, req *iapiserver.ApplicationRunCreateRequest) (*iapiserver.ApplicationRun, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	if !app.RunEnabled {
		return nil, errors.NewStatus(code.ErrAIAppPermissionDenied, "application run is disabled")
	}
	if existing, lookupErr := s.Store.ApplicationPlatforms().GetApplicationRunByIdempotency(ctx, p.UserID, req.IdempotencyKey); lookupErr == nil {
		if !sameApplicationRunRequest(existing, applicationID, req) {
			return nil, errors.NewStatus(code.ErrAIAppApplicationRunCreateFailed, "idempotency key was used with a different application run request")
		}
		result, retryErr := s.retryTaskBinding(ctx, existing)
		if retryErr != nil {
			return nil, retryErr
		}
		s.attachApplicationRunRelations(ctx, result)
		return result, nil
	} else if !stderrors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}
	form, err := s.ResolveRuntimeForm(ctx, applicationID, &iapiserver.RuntimeFormResolveRequest{ApplicationVersionID: req.ApplicationVersionID, EngineInstanceID: req.EngineInstanceID, CurrentValues: req.Inputs})
	if err != nil {
		return nil, err
	}
	if len(form.Violations) > 0 {
		return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "application input has unresolved violations")
	}
	engineID := req.EngineInstanceID
	if engineID == "" && len(form.CompatibleEngineInstanceIDs) > 0 {
		engineID = form.CompatibleEngineInstanceIDs[0]
	}
	if engineID == "" {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "no compatible engine instance")
	}
	resolvedInputs := resolvedRuntimeInputs(req.Inputs, form.Fields)
	resolvedRequest := *req
	resolvedRequest.Inputs = resolvedInputs
	version, err := s.Store.ApplicationPlatforms().GetApplicationVersion(ctx, req.ApplicationVersionID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppApplicationVersionNotFound, "application version not found")
	}
	templateVersion, err := s.Store.ApplicationPlatforms().GetTemplateVersion(ctx, version.ApplicationTemplateVersionID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppTemplateVersionNotFound, "template version not found")
	}
	engine, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, engineID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineUnavailable, "engine instance is unavailable")
	}
	run := newApplicationRun(p.UserID, app, version, templateVersion, engine, &resolvedRequest, req, form)
	if run.ProviderCapabilityID != nil {
		if capability, ok := s.Capabilities.Get(*run.ProviderCapabilityID); ok {
			run.CapabilitySourceSnapshot["provider_capability"] = capability
		}
	}
	created, err := s.Store.ApplicationPlatforms().AddApplicationRun(ctx, run)
	if err != nil {
		return nil, mapUnique(err, "idx_aiapp_runs_owner_idempotency", code.ErrAIAppApplicationRunCreateFailed, "application run idempotency conflict")
	}
	s.publish(ctx, "application_run_created", created.ID+":created", applicationRunEventPayload(created))
	result, err := s.retryTaskBinding(ctx, created)
	if err != nil {
		return nil, err
	}
	s.attachApplicationRunRelations(ctx, result)
	return result, nil
}

func sameApplicationRunRequest(existing *iapiserver.ApplicationRun, applicationID string, request *iapiserver.ApplicationRunCreateRequest) bool {
	if existing == nil || request == nil || existing.ApplicationID != applicationID || existing.ApplicationVersionID != request.ApplicationVersionID {
		return false
	}
	storedEngine, hasStoredEngine := existing.ExecutionSnapshot["idempotency_engine_instance_id"].(string)
	if hasStoredEngine && storedEngine != request.EngineInstanceID {
		return false
	}
	if !hasStoredEngine && request.EngineInstanceID != "" && existing.EngineInstanceID != request.EngineInstanceID {
		return false
	}
	storedInputs, hasStoredInputs := existing.ExecutionSnapshot["idempotency_inputs"].(map[string]any)
	if !hasStoredInputs {
		storedInputs = existing.InputSnapshot
	}
	return reflect.DeepEqual(storedInputs, request.Inputs)
}

func (s *applicationPlatformService) retryTaskBinding(ctx context.Context, run *iapiserver.ApplicationRun) (*iapiserver.ApplicationRun, error) {
	if run.TaskCreationStatus == iapiserver.TaskCreationCreated {
		return s.Store.ApplicationPlatforms().GetApplicationRun(ctx, run.ID)
	}
	if s.Tasks == nil {
		return s.failTaskBinding(ctx, run, errors.New("task center is unavailable"))
	}
	version, err := s.Store.ApplicationPlatforms().GetApplicationVersion(ctx, run.ApplicationVersionID)
	if err != nil {
		return s.failTaskBinding(ctx, run, err)
	}
	templateVersion, err := s.Store.ApplicationPlatforms().GetTemplateVersion(ctx, version.ApplicationTemplateVersionID)
	if err != nil {
		return s.failTaskBinding(ctx, run, err)
	}
	operation := ""
	if templateVersion.ProviderOperationID != nil {
		operation = *templateVersion.ProviderOperationID
	}
	task, err := s.Tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{
		Key: applicationRunTaskDefinitionID, Name: "Application Platform Run", Description: "Execute an immutable ApplicationRun snapshot",
		FunctionRef: applicationRunTaskFunctionRef, Arguments: map[string]any{"application_run_id": run.ID, "operation": operation, "execution_snapshot": run.ExecutionSnapshot},
		RequiredCapabilities: "application-platform", ApplicationRunID: run.ID,
		IdempotencyScope: "application-run", IdempotencyKey: run.IdempotencyKey,
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
		SystemName: iapiserver.SystemNameSpec{Key: taskname.ApplicationRun},
	})
	if err != nil {
		return s.failTaskBinding(ctx, run, err)
	}
	bound, err := s.Store.ApplicationPlatforms().BindApplicationRunTask(ctx, run.ID, task.ID, iapiserver.TaskCreationCreated, task.ResourceVersion, "")
	if err != nil {
		return nil, err
	}
	s.publish(ctx, "application_run_task_bound", run.ID+":"+task.ID, map[string]any{"application_run_id": run.ID, "atomic_task_id": task.ID, "task_creation_status": iapiserver.TaskCreationCreated, "task_resource_version": task.ResourceVersion})
	return bound, nil
}
func (s *applicationPlatformService) failTaskBinding(ctx context.Context, run *iapiserver.ApplicationRun, cause error) (*iapiserver.ApplicationRun, error) {
	failed, storeErr := s.Store.ApplicationPlatforms().BindApplicationRunTask(ctx, run.ID, "", iapiserver.TaskCreationFailed, 0, cause.Error())
	if storeErr != nil {
		return nil, storeErr
	}
	return failed, errors.NewStatus(code.ErrAIAppAtomicTaskCreateFailed, cause.Error())
}
func (s *applicationPlatformService) GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	run, err := s.Store.ApplicationPlatforms().GetApplicationRun(ctx, id)
	if err != nil || (!p.Admin && run.OwnerUserID != p.UserID) {
		return nil, errors.NewStatus(code.ErrAIAppApplicationRunNotFound, "application run not found")
	}
	s.attachApplicationRunRelations(ctx, run)
	return run, nil
}

// attachApplicationRunRelations 优先读取运行快照，并以固定上限回查旧数据缺失的同域摘要。
// AtomicTask 始终通过 Task Center 权限边界读取；任一关联缺失不影响父运行响应。
func (s *applicationPlatformService) attachApplicationRunRelations(ctx context.Context, run *iapiserver.ApplicationRun) {
	if run == nil {
		return
	}
	run.Application = snapshotValue[iapiserver.ApplicationSummary](run.CapabilitySourceSnapshot, "application")
	run.ApplicationVersion = snapshotValue[iapiserver.ApplicationVersionSummary](run.CapabilitySourceSnapshot, "application_version")
	run.ApplicationTemplateVersion = snapshotValue[iapiserver.ApplicationTemplateVersionSummary](run.CapabilitySourceSnapshot, "application_template_version")
	run.EngineInstance = snapshotValue[iapiserver.EngineInstanceRefSummary](run.CapabilitySourceSnapshot, "engine_instance")

	if run.Application == nil {
		if item, err := s.GetApplication(ctx, run.ApplicationID); err == nil {
			run.Application = applicationSummary(item)
		}
	}
	if run.ApplicationVersion == nil {
		if item, err := s.GetApplicationVersion(ctx, run.ApplicationVersionID); err == nil {
			run.ApplicationVersion = applicationVersionSummary(item)
		}
	}
	if run.ApplicationTemplateVersion == nil {
		if item, err := s.GetTemplateVersion(ctx, run.ApplicationTemplateVersionID); err == nil {
			run.ApplicationTemplateVersion = applicationTemplateVersionSummary(item)
		}
	}
	if run.EngineInstance == nil {
		if item, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, run.EngineInstanceID); err == nil {
			run.EngineInstance = engineInstanceRefSummary(item)
		}
	}
	if run.ProviderCapabilityID != nil {
		if capability := snapshotValue[iapiserver.AIAppProviderCapability](run.CapabilitySourceSnapshot, "provider_capability"); capability != nil {
			run.ProviderCapability = providerCapabilityRefSummary(capability, run.ProviderOperationID)
		} else if s.Capabilities != nil {
			capability, ok := s.Capabilities.Get(*run.ProviderCapabilityID)
			if ok {
				run.ProviderCapability = providerCapabilityRefSummary(capability, run.ProviderOperationID)
			}
		}
	}
	if run.AtomicTaskID != nil && s.Tasks != nil {
		if task, err := s.Tasks.GetAtomicTask(ctx, *run.AtomicTaskID); err == nil {
			run.AtomicTask = &iapiserver.AtomicTaskRefSummary{ID: task.ID, Name: task.Name, Status: task.Status, Progress: task.Progress, FunctionRef: task.FunctionRef}
		}
	}
}

func (s *applicationPlatformService) validateEngineAuth(typeID, authType string, config map[string]any) error {
	engineType, ok := s.Runtime.EngineType(typeID)
	if !ok || !contains(engineType.AuthenticationTypes, authType) {
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
				if stringValue(config[key]) == "" {
					return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "missing authentication field "+key)
				}
			}
		}
	} else {
		for _, raw := range required {
			key, _ := raw.(string)
			allowed[key] = struct{}{}
			if stringValue(config[key]) == "" {
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

func (s *applicationPlatformService) resolveBindingStatus(binding *iapiserver.EngineCapabilityBinding) {
	capability, ok := s.Capabilities.Get(binding.ProviderCapabilityID)
	binding.SystemManaged = ok && capability.Origin == iapiserver.ProviderCapabilityOriginBuiltin && capability.BindingPolicy == iapiserver.ProviderBindingPolicyRequiredImmutable
	switch {
	case !binding.Enabled:
		binding.EffectiveStatus = iapiserver.BindingEffectiveDisabled
	case !ok || capability.Availability != iapiserver.ProviderCapabilityAvailable || capability.Revision != binding.ProviderCapabilityRevision:
		binding.EffectiveStatus = iapiserver.BindingEffectiveUnavailable
	default:
		binding.EffectiveStatus = iapiserver.BindingEffectiveAvailable
	}
}

func (s *applicationPlatformService) isSystemManagedBinding(binding *iapiserver.EngineCapabilityBinding) bool {
	if binding == nil {
		return false
	}
	capability, ok := s.Capabilities.Get(binding.ProviderCapabilityID)
	return ok && capability.Origin == iapiserver.ProviderCapabilityOriginBuiltin && capability.BindingPolicy == iapiserver.ProviderBindingPolicyRequiredImmutable
}

func (s *applicationPlatformService) templateVersionFromRequest(capabilityDefinitionID, sourceType, providerID, operationID string, workflow, contract map[string]any) (*iapiserver.ApplicationTemplateVersion, error) {
	version := &iapiserver.ApplicationTemplateVersion{CapabilitySourceType: sourceType, TemplateContract: contract, ComfyUIAPIWorkflow: workflow}
	switch sourceType {
	case iapiserver.CapabilitySourceProviderCapability:
		capability, ok := s.Capabilities.Get(providerID)
		if !ok || capability.Availability != iapiserver.ProviderCapabilityAvailable {
			return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability is unavailable")
		}
		if capability.Kind != iapiserver.ProviderCapabilityKindCatalog {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "binding-only capability cannot be used as a template source")
		}
		operation, ok := findOperation(capability, operationID)
		if !ok || operation.CapabilityDefinitionID != capabilityDefinitionID {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "provider operation does not match capability")
		}
		version.SourceRevision = capability.Revision
		version.ProviderCapabilityID = stringPtr(capability.ID)
		version.ProviderCapabilityRevision = stringPtr(capability.Revision)
		version.ProviderOperationID = stringPtr(operationID)
	case iapiserver.CapabilitySourceComfyUIWorkflow:
		if len(workflow) == 0 {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI workflow contract is incomplete")
		}
		revision, err := canonicalJSONDigest(map[string]any{"api_workflow": workflow, "template_contract": contract})
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, err.Error())
		}
		version.SourceRevision = revision
		version.WorkflowContractRevision = stringPtr(revision)
	default:
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "unknown capability source type")
	}
	return version, nil
}
func (s *applicationPlatformService) validateTemplateVersion(ctx context.Context, version *iapiserver.ApplicationTemplateVersion) error {
	if version.Status != iapiserver.VersionStatusDraft {
		return errors.NewStatus(code.ErrAIAppTemplateVersionNotPublishable, "template version is not draft")
	}
	switch version.CapabilitySourceType {
	case iapiserver.CapabilitySourceProviderCapability:
		if version.ProviderCapabilityID == nil {
			return errors.NewStatus(code.ErrAIAppTemplateVersionNotPublishable, "provider capability is missing")
		}
		capability, ok := s.Capabilities.Get(*version.ProviderCapabilityID)
		if !ok || capability.Kind != iapiserver.ProviderCapabilityKindCatalog || capability.Availability != iapiserver.ProviderCapabilityAvailable || version.ProviderCapabilityRevision == nil || capability.Revision != *version.ProviderCapabilityRevision {
			return errors.NewStatus(code.ErrAIAppTemplateVersionNotPublishable, "provider capability revision is unavailable")
		}
	case iapiserver.CapabilitySourceComfyUIWorkflow:
		engines, _, err := s.Store.ApplicationPlatforms().ListEngineInstances(ctx, &iapiserver.EngineInstanceListRequest{ApplicationEngineTypeID: "comfyui", Enabled: boolPtr(true)})
		if err != nil {
			return err
		}
		compatible, err := s.compatibleComfyUIEngineIDs(ctx, version, engines)
		if err != nil {
			return err
		}
		if len(compatible) == 0 {
			return errors.NewStatus(code.ErrAIAppTemplateVersionNotPublishable, "no current ComfyUI object_info can execute this template")
		}
		return nil
	default:
		return errors.NewStatus(code.ErrAIAppTemplateVersionNotPublishable, "invalid capability source")
	}
	return nil
}
func (s *applicationPlatformService) publish(ctx context.Context, eventType, key string, payload map[string]any) {
	occurredAt := imachinery.Now()
	switch eventType {
	case "application_run_created", "application_run_task_bound":
		payload["occurred_at"] = occurredAt
	}
	if err := s.Events.Publish(context.WithoutCancel(ctx), &iapiserver.ApplicationPlatformEvent{Type: eventType, IdempotencyKey: key, Payload: payload, OccurredAt: occurredAt}); err != nil {
		log.Errorf("application platform event publish failed: type=%s key=%s error=%v", eventType, key, err)
	}
}

func newApplicationRun(owner string, app *iapiserver.Application, version *iapiserver.ApplicationVersion, template *iapiserver.ApplicationTemplateVersion, engine *iapiserver.EngineInstance, resolved, original *iapiserver.ApplicationRunCreateRequest, form *iapiserver.RuntimeFormSchema) *iapiserver.ApplicationRun {
	run := &iapiserver.ApplicationRun{OwnerUserID: owner, ApplicationID: app.ID, ApplicationVersionID: version.ID, ApplicationTemplateVersionID: template.ID, EngineInstanceID: engine.ID, CapabilitySourceType: template.CapabilitySourceType, SourceRevision: template.SourceRevision, ProviderCapabilityID: template.ProviderCapabilityID, ProviderCapabilityRevision: template.ProviderCapabilityRevision, ProviderOperationID: template.ProviderOperationID, WorkflowContractRevision: template.WorkflowContractRevision, CapabilitySourceSnapshot: map[string]any{"source_revision": template.SourceRevision, "template_contract": template.TemplateContract, "comfyui_api_workflow": template.ComfyUIAPIWorkflow, "application": applicationSummary(app), "application_version": applicationVersionSummary(version), "application_template_version": applicationTemplateVersionSummary(template), "engine_instance": engineInstanceRefSummary(engine)}, InputSnapshot: resolved.Inputs, ExecutionSnapshot: map[string]any{"inputs": resolved.Inputs, "application_version_id": version.ID, "template_version_id": template.ID, "engine_instance_id": engine.ID, "capability_definition_id": app.CapabilityDefinitionID, "idempotency_inputs": original.Inputs, "idempotency_engine_instance_id": original.EngineInstanceID}, OutputMappingSnapshot: version.OutputSchema, TaskCreationStatus: iapiserver.TaskCreationPending, OutputValues: []map[string]any{}, IdempotencyKey: original.IdempotencyKey, Artifacts: []*iapiserver.ApplicationArtifact{}}
	run.Name = app.Name + " run"
	if form.ProviderCapabilityID != nil {
		run.CapabilitySourceSnapshot["provider_capability_id"] = *form.ProviderCapabilityID
		run.CapabilitySourceSnapshot["provider_capability_revision"] = *form.ProviderCapabilityRevision
	}
	return run
}

func applicationSummary(item *iapiserver.Application) *iapiserver.ApplicationSummary {
	return &iapiserver.ApplicationSummary{ID: item.ID, Name: item.Name, Visibility: item.Visibility}
}

func applicationVersionSummary(item *iapiserver.ApplicationVersion) *iapiserver.ApplicationVersionSummary {
	return &iapiserver.ApplicationVersionSummary{ID: item.ID, SemanticVersion: item.SemanticVersion, Status: item.Status}
}

func applicationTemplateVersionSummary(item *iapiserver.ApplicationTemplateVersion) *iapiserver.ApplicationTemplateVersionSummary {
	return &iapiserver.ApplicationTemplateVersionSummary{ID: item.ID, Version: item.Version, Status: item.Status, CapabilitySourceType: item.CapabilitySourceType, SourceRevision: item.SourceRevision}
}

func engineInstanceRefSummary(item *iapiserver.EngineInstance) *iapiserver.EngineInstanceRefSummary {
	return &iapiserver.EngineInstanceRefSummary{ID: item.ID, Name: item.Name, ApplicationEngineTypeID: item.ApplicationEngineTypeID, Enabled: item.Enabled, HealthStatus: item.HealthStatus}
}

func providerCapabilityRefSummary(capability *iapiserver.AIAppProviderCapability, operationID *string) *iapiserver.ProviderCapabilityRefSummary {
	summary := &iapiserver.ProviderCapabilityRefSummary{ID: capability.ID, Name: capability.Name, Revision: capability.Revision, Availability: capability.Availability, OperationID: operationID}
	if operationID == nil {
		return summary
	}
	for _, operation := range capability.Operations {
		if operation.ID != *operationID {
			continue
		}
		name := operation.Description
		if name == "" {
			name = operation.ID
		}
		summary.OperationName = &name
		break
	}
	return summary
}

func snapshotValue[T any](snapshot map[string]any, key string) *T {
	value, ok := snapshot[key]
	if !ok || value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return &result
}
func applicationRunEventPayload(run *iapiserver.ApplicationRun) map[string]any {
	return map[string]any{"application_run_id": run.ID, "application_id": run.ApplicationID, "application_version_id": run.ApplicationVersionID, "application_template_version_id": run.ApplicationTemplateVersionID, "engine_instance_id": run.EngineInstanceID, "capability_source_type": run.CapabilitySourceType, "source_revision": run.SourceRevision, "provider_capability_id": run.ProviderCapabilityID, "provider_capability_revision": run.ProviderCapabilityRevision, "provider_operation_id": run.ProviderOperationID, "workflow_contract_revision": run.WorkflowContractRevision, "execution_snapshot": run.ExecutionSnapshot, "task_creation_status": run.TaskCreationStatus}
}

func resolvedRuntimeInputs(inputs map[string]any, fields []iapiserver.RuntimeFormField) map[string]any {
	resolved := copyMap(inputs)
	for _, field := range fields {
		if field.Value == nil {
			delete(resolved, field.Name)
			continue
		}
		resolved[field.Name] = field.Value
	}
	return resolved
}

func validateRestrictions(capability *iapiserver.AIAppProviderCapability, restrictions map[string]any) error {
	if len(restrictions) == 0 {
		return nil
	}
	allowed := map[string]map[string]struct{}{"model_ids": {}, "operation_ids": {}, "variant_ids": {}}
	for _, item := range capability.Models {
		allowed["model_ids"][item.ID] = struct{}{}
	}
	for _, item := range capability.Operations {
		allowed["operation_ids"][item.ID] = struct{}{}
	}
	for _, item := range capability.Variants {
		allowed["variant_ids"][item.ID] = struct{}{}
	}
	for key, value := range restrictions {
		set, ok := allowed[key]
		if !ok {
			return errors.NewStatus(code.ErrAIAppEngineBindingRestrictionExpands, "unknown restriction key "+key)
		}
		for _, item := range anyStrings(value) {
			if _, ok := set[item]; !ok {
				return errors.NewStatus(code.ErrAIAppEngineBindingRestrictionExpands, "restriction adds unknown value "+item)
			}
		}
	}
	return nil
}
func validateComfyUIWorkflow(workflow, objectInfo map[string]any) error {
	for nodeID, raw := range workflow {
		node, ok := raw.(map[string]any)
		if !ok {
			return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI node "+nodeID+" is invalid")
		}
		classType := stringValue(node["class_type"])
		if classType == "" {
			return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI node is missing class_type")
		}
		if _, ok := objectInfo[classType]; !ok {
			return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI object_info is missing "+classType)
		}
	}
	return nil
}

func validateComfyUIContract(workflow, objectInfo, contract map[string]any) error {
	if err := validateComfyUIWorkflow(workflow, objectInfo); err != nil {
		return err
	}
	mappings := mapValue(contract["request_mapping"])
	outputs := mapValue(contract["outputs"])
	if len(mappings) == 0 || len(outputs) == 0 {
		return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI contract requires request_mapping and outputs")
	}
	dummyInputs := make(map[string]any, len(mappings))
	for field := range mappings {
		dummyInputs[field] = "validation"
	}
	for field, raw := range mapValue(contract["parameters"]) {
		definition := mapValue(raw)
		if boolValue(definition["required"]) {
			if _, exists := mappings[field]; !exists {
				return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "required ComfyUI parameter has no request mapping: "+field)
			}
		}
	}
	_, err := applyComfyInputs(workflow, dummyInputs, contract)
	return err
}
func findOperation(capability *iapiserver.AIAppProviderCapability, id string) (iapiserver.ProviderCapabilityOperation, bool) {
	for _, item := range capability.Operations {
		if item.ID == id {
			return item, true
		}
	}
	return iapiserver.ProviderCapabilityOperation{}, false
}
func applyEngineUpdate(item *iapiserver.EngineInstance, req *iapiserver.EngineInstanceUpdateRequest) {
	applyString(&item.Name, req.Name)
	applyString(&item.Description, req.Description)
	applyString(&item.BaseURL, req.BaseURL)
	applyString(&item.AuthType, req.AuthType)
	if req.AuthConfig != nil {
		item.AuthConfig = req.AuthConfig
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	applyString(&item.Region, req.Region)
	applyInt(&item.MaxConcurrency, req.MaxConcurrency)
	applyInt(&item.RequestTimeoutSeconds, req.RequestTimeoutSeconds)
	applyInt(&item.TaskTimeoutSeconds, req.TaskTimeoutSeconds)
}
func applyApplicationUpdate(item *iapiserver.Application, req *iapiserver.ApplicationUpdateRequest) {
	applyString(&item.Name, req.Name)
	applyString(&item.Description, req.Description)
	applyString(&item.Visibility, req.Visibility)
	if req.RunEnabled != nil {
		item.RunEnabled = *req.RunEnabled
	}
	if req.CanvasEnabled != nil {
		item.CanvasEnabled = *req.CanvasEnabled
	}
	if req.CopyEnabled != nil {
		item.CopyEnabled = *req.CopyEnabled
	}
	if req.PresetEnabled != nil {
		item.PresetEnabled = *req.PresetEnabled
	}
}
func mapNotFound(err error, businessCode int, message string) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(businessCode, message)
	}
	return err
}
func mapUnique(err error, constraint string, businessCode int, message string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), constraint) || strings.Contains(err.Error(), "duplicate key") {
		return errors.NewStatus(businessCode, message)
	}
	return err
}
func matchesKeyword(keyword string, values ...string) bool {
	if keyword == "" {
		return true
	}
	keyword = strings.ToLower(keyword)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}
func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
func defaultBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
func applyString(target *string, value *string) {
	if value != nil {
		*target = *value
	}
}
func applyInt(target *int, value *int) {
	if value != nil {
		*target = *value
	}
}
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func stringPtr(value string) *string { return &value }
func stringValue(value any) string   { ret, _ := value.(string); return ret }
func anyStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		ret := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				ret = append(ret, value)
			}
		}
		return ret
	default:
		return nil
	}
}
