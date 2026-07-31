package applicationplatform

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/engine"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	applicationRunTaskDefinitionID = "application-platform.application-run"
	applicationRunTaskFunctionRef  = "application-platform.run"
)

// ApplicationPlatformSrv implements the public S2 Application Platform operations.
type ApplicationPlatformSrv interface {
	modelgateway.ProviderCapabilitySrv
	engine.Srv
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
	ListApplicationRuns(context.Context, *iapiserver.ApplicationRunListRequest) (*iapiserver.ApplicationRunListResponse, error)
	GetApplicationRun(context.Context, string) (*iapiserver.ApplicationRun, error)
	ResolveCanvasApplicationVersion(context.Context, string, map[string]any) (*CanvasApplicationVersion, error)
	ResolveCanvasApplicationVersions(context.Context, []CanvasApplicationVersionRequest) []CanvasApplicationVersionResult
	ListPublishedCanvasApplicationVersions(context.Context) ([]*CanvasApplicationVersion, error)
	EnsureCanvasApplicationRun(context.Context, *CanvasApplicationRunRequest) (*iapiserver.ApplicationRun, error)
}

// CanvasApplicationVersion 是 Workflow Canvas 消费的权限裁剪内部契约。
type CanvasApplicationVersion struct {
	Application *iapiserver.Application
	Version     *iapiserver.ApplicationVersion
	RuntimeForm *iapiserver.RuntimeFormSchema
}

// CanvasApplicationVersionRequest 是 Canvas 批量实时复核中的单项请求。
type CanvasApplicationVersionRequest struct {
	Key                  string
	ApplicationVersionID string
	Inputs               map[string]any
}

// CanvasApplicationVersionResult 保留单项身份，使调用方可区分权限/运行能力失败。
type CanvasApplicationVersionResult struct {
	Key      string
	Resolved *CanvasApplicationVersion
	Err      error
}

// CanvasApplicationRunRequest 携带 DAG Worker 已解析的最终输入和既有 AtomicTask 身份。
type CanvasApplicationRunRequest struct {
	AtomicTaskID         string
	CanvasRunID          string
	CanvasNodeRunID      string
	ExecutionKey         string
	ApplicationVersionID string
	OwnerUserID          string
	Inputs               map[string]any
	Arguments            map[string]any
}

type Principal = modelgateway.Principal
type PrincipalResolver = modelgateway.PrincipalResolver

// ArtifactLifecycle 是 ApplicationExecutor 消费的 Asset Library 写入边界。
// Application Platform 只交付受控字节流，不传递 Provider URL、凭证或原始响应。
type ArtifactLifecycle interface {
	Prepare(context.Context, *iapiserver.Artifact) (*iapiserver.Artifact, bool, error)
	StoreContent(context.Context, *iapiserver.Artifact, string, io.Reader) (*iapiserver.Artifact, error)
	Register(context.Context, *iapiserver.Artifact, string) (*iapiserver.Artifact, error)
	FailProcessing(context.Context, *iapiserver.Artifact, string, string) (*iapiserver.Artifact, error)
	FailRegistration(context.Context, *iapiserver.Artifact, string, string) (*iapiserver.Artifact, error)
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
	Adapters       map[string]engine.Adapter
	Tasks          taskcenter.TaskCenterSrv
	Assets         ArtifactLifecycle
	Events         EventPublisher
	WorkflowAudit  WorkflowAuditor
	WorkflowParser ComfyWorkflowParser
}

type applicationPlatformService struct {
	Dependencies
	modelgateway.ProviderCapabilitySrv
	engine.Srv
}

func NewService(deps Dependencies) (*applicationPlatformService, error) {
	if deps.Store == nil || deps.Runtime == nil || deps.Capabilities == nil {
		return nil, fmt.Errorf("application platform store and registries are required")
	}
	if deps.Principals == nil {
		deps.Principals = modelgateway.NewStorePrincipalResolver(deps.Store)
	}
	if deps.Adapters == nil {
		deps.Adapters = map[string]engine.Adapter{}
	}
	if deps.Events == nil {
		deps.Events = NoopEventPublisher{}
	}
	if deps.Assets == nil {
		deps.Assets = NoopArtifactLifecycle{}
	}
	if deps.WorkflowAudit == nil {
		deps.WorkflowAudit = StructuredWorkflowAuditor{}
	}
	if deps.WorkflowParser == nil {
		deps.WorkflowParser = comfy2GoWorkflowParser{}
	}
	providerCapabilityService, err := modelgateway.NewService(modelgateway.Dependencies{
		Capabilities: deps.Capabilities,
		Principals:   deps.Principals,
	})
	if err != nil {
		return nil, err
	}
	engineService, err := engine.NewService(engine.Dependencies{
		Store: deps.Store, Runtime: deps.Runtime, Capabilities: deps.Capabilities,
		Principals: deps.Principals, Adapters: deps.Adapters,
	})
	if err != nil {
		return nil, err
	}
	return &applicationPlatformService{
		Dependencies: deps, ProviderCapabilitySrv: providerCapabilityService, Srv: engineService,
	}, nil
}

type NoopEventPublisher struct{}

func (NoopEventPublisher) Publish(context.Context, *iapiserver.ApplicationPlatformEvent) error {
	return nil
}

type NoopArtifactLifecycle struct{}

func (NoopArtifactLifecycle) Prepare(context.Context, *iapiserver.Artifact) (*iapiserver.Artifact, bool, error) {
	return nil, false, errors.New("artifact lifecycle is not configured")
}
func (NoopArtifactLifecycle) StoreContent(context.Context, *iapiserver.Artifact, string, io.Reader) (*iapiserver.Artifact, error) {
	return nil, errors.New("artifact lifecycle is not configured")
}
func (NoopArtifactLifecycle) Register(context.Context, *iapiserver.Artifact, string) (*iapiserver.Artifact, error) {
	return nil, errors.New("artifact lifecycle is not configured")
}
func (NoopArtifactLifecycle) FailProcessing(context.Context, *iapiserver.Artifact, string, string) (*iapiserver.Artifact, error) {
	return nil, errors.New("artifact lifecycle is not configured")
}
func (NoopArtifactLifecycle) FailRegistration(context.Context, *iapiserver.Artifact, string, string) (*iapiserver.Artifact, error) {
	return nil, errors.New("artifact lifecycle is not configured")
}

// StructuredWorkflowAuditor emits the mandatory managed-access audit fields for identity log ingestion.
type StructuredWorkflowAuditor struct{}

func (StructuredWorkflowAuditor) Record(_ context.Context, record WorkflowAuditRecord) error {
	log.Infof("ComfyUI workflow audit: action=%s actor_user_id=%s owner_user_id=%s workflow_id=%s result=%s occurred_at=%s", record.Action, record.ActorUserID, record.OwnerUserID, record.WorkflowID, record.Result, record.OccurredAt.String())
	return nil
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
	result, err := s.retryTaskBinding(ctx, created)
	if err != nil {
		return nil, err
	}
	s.attachApplicationRunRelations(ctx, result)
	return result, nil
}

// ResolveCanvasApplicationVersion 返回当前主体可用于 Canvas 的版本、schema 与实时运行能力。
func (s *applicationPlatformService) ResolveCanvasApplicationVersion(
	ctx context.Context,
	versionID string,
	inputs map[string]any,
) (*CanvasApplicationVersion, error) {
	results := s.ResolveCanvasApplicationVersions(ctx, []CanvasApplicationVersionRequest{{
		Key:                  versionID,
		ApplicationVersionID: versionID,
		Inputs:               inputs,
	}})
	if len(results) != 1 {
		return nil, errors.NewStatus(code.ErrAIAppApplicationVersionNotFound, "published application version not found")
	}
	return results[0].Resolved, results[0].Err
}

// ResolveCanvasApplicationVersions 批量执行权限、开关、schema 与 Engine/runtime 实时复核。
func (s *applicationPlatformService) ResolveCanvasApplicationVersions(
	ctx context.Context,
	requests []CanvasApplicationVersionRequest,
) []CanvasApplicationVersionResult {
	results := make([]CanvasApplicationVersionResult, len(requests))
	for index, request := range requests {
		results[index].Key = request.Key
	}
	if len(requests) == 0 {
		return results
	}
	principal, err := s.principal(ctx, false)
	if err != nil {
		for index := range results {
			results[index].Err = err
		}
		return results
	}
	versionIDs := make([]string, 0, len(requests))
	seenVersionIDs := make(map[string]struct{}, len(requests))
	for _, request := range requests {
		if _, exists := seenVersionIDs[request.ApplicationVersionID]; !exists {
			seenVersionIDs[request.ApplicationVersionID] = struct{}{}
			versionIDs = append(versionIDs, request.ApplicationVersionID)
		}
	}
	versions, err := s.Store.ApplicationPlatforms().GetApplicationVersionsByIDs(ctx, versionIDs)
	if err != nil {
		for index := range results {
			results[index].Err = err
		}
		return results
	}
	versionByID := make(map[string]*iapiserver.ApplicationVersion, len(versions))
	applicationIDs := make([]string, 0, len(versions))
	seenApplicationIDs := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		versionByID[version.ID] = version
		if _, exists := seenApplicationIDs[version.ApplicationID]; !exists {
			seenApplicationIDs[version.ApplicationID] = struct{}{}
			applicationIDs = append(applicationIDs, version.ApplicationID)
		}
	}
	applications, err := s.Store.ApplicationPlatforms().GetApplicationsByIDs(ctx, applicationIDs)
	if err != nil {
		for index := range results {
			results[index].Err = err
		}
		return results
	}
	applicationByID := make(map[string]*iapiserver.Application, len(applications))
	for _, application := range applications {
		applicationByID[application.ID] = application
	}
	for index, request := range requests {
		version := versionByID[request.ApplicationVersionID]
		if version == nil || version.Status != iapiserver.VersionStatusPublished {
			results[index].Err = errors.NewStatus(code.ErrAIAppApplicationVersionNotFound, "published application version not found")
			continue
		}
		application := applicationByID[version.ApplicationID]
		if application == nil || (!principal.Admin && application.OwnerUserID != principal.UserID &&
			application.Visibility != iapiserver.ApplicationVisibilityGlobal) {
			results[index].Err = errors.NewStatus(code.ErrAIAppApplicationNotFound, "application not found")
			continue
		}
		if !application.CanvasEnabled || !application.RunEnabled {
			results[index].Err = errors.NewStatus(code.ErrAIAppPermissionDenied, "application is unavailable to canvas")
			continue
		}
		form, resolveErr := s.resolveRuntimeForm(ctx, application, version, &iapiserver.RuntimeFormResolveRequest{
			ApplicationVersionID: version.ID,
			CurrentValues:        request.Inputs,
		})
		if resolveErr != nil {
			results[index].Err = resolveErr
			continue
		}
		if len(form.CompatibleEngineInstanceIDs) == 0 {
			results[index].Err = errors.NewStatus(code.ErrAIAppEngineUnavailable, "application has no executable engine runtime")
			continue
		}
		results[index].Resolved = &CanvasApplicationVersion{
			Application: application,
			Version:     version,
			RuntimeForm: form,
		}
	}
	return results
}

// ListPublishedCanvasApplicationVersions 返回启动修复所需的已发布版本事实，不替代调用方实时权限校验。
func (s *applicationPlatformService) ListPublishedCanvasApplicationVersions(ctx context.Context) ([]*CanvasApplicationVersion, error) {
	versions, err := s.Store.ApplicationPlatforms().ListPublishedApplicationVersions(ctx)
	if err != nil {
		return nil, err
	}
	applicationIDs := make([]string, 0, len(versions))
	for _, version := range versions {
		applicationIDs = append(applicationIDs, version.ApplicationID)
	}
	applications, err := s.Store.ApplicationPlatforms().GetApplicationsByIDs(ctx, applicationIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*iapiserver.Application, len(applications))
	for _, application := range applications {
		byID[application.ID] = application
	}
	result := make([]*CanvasApplicationVersion, 0, len(versions))
	for _, version := range versions {
		if application := byID[version.ApplicationID]; application != nil {
			result = append(result, &CanvasApplicationVersion{Application: application, Version: version})
		}
	}
	return result, nil
}

// EnsureCanvasApplicationRun 在 Provider 调用前把最终 DAG 输入固化为 ApplicationRun，并绑定既有 AtomicTask。
func (s *applicationPlatformService) EnsureCanvasApplicationRun(
	ctx context.Context,
	req *CanvasApplicationRunRequest,
) (*iapiserver.ApplicationRun, error) {
	if req == nil || req.AtomicTaskID == "" || req.CanvasRunID == "" || req.CanvasNodeRunID == "" ||
		req.ExecutionKey == "" || req.ApplicationVersionID == "" || req.OwnerUserID == "" || req.Arguments == nil {
		return nil, errors.NewStatus(code.ErrAIAppApplicationRunCreateFailed, "canvas application run identity is incomplete")
	}
	version, err := s.Store.ApplicationPlatforms().GetApplicationVersion(ctx, req.ApplicationVersionID)
	if err != nil || version.Status != iapiserver.VersionStatusPublished {
		return nil, errors.NewStatus(code.ErrAIAppApplicationVersionNotFound, "published application version not found")
	}
	app, err := s.Store.ApplicationPlatforms().GetApplication(ctx, version.ApplicationID)
	if err != nil || (app.OwnerUserID != req.OwnerUserID && app.Visibility != iapiserver.ApplicationVisibilityGlobal) {
		return nil, errors.NewStatus(code.ErrAIAppApplicationNotFound, "application not found")
	}
	if !app.CanvasEnabled || !app.RunEnabled {
		return nil, errors.NewStatus(code.ErrAIAppPermissionDenied, "application is unavailable to canvas")
	}
	form, err := s.resolveRuntimeForm(ctx, app, version, &iapiserver.RuntimeFormResolveRequest{
		ApplicationVersionID: version.ID,
		CurrentValues:        req.Inputs,
	})
	if err != nil {
		return nil, err
	}
	if len(form.Violations) > 0 {
		return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "canvas application input has unresolved violations")
	}
	if len(form.CompatibleEngineInstanceIDs) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "no compatible engine instance")
	}
	template, err := s.Store.ApplicationPlatforms().GetTemplateVersion(ctx, version.ApplicationTemplateVersionID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppTemplateVersionNotFound, "application template version not found")
	}
	idempotencyKey := "canvas:" + req.CanvasRunID + ":" + req.ExecutionKey
	resolved := &iapiserver.ApplicationRunCreateRequest{
		ApplicationVersionID: version.ID,
		Inputs:               resolvedRuntimeInputs(req.Inputs, form.Fields),
		IdempotencyKey:       idempotencyKey,
	}
	original := &iapiserver.ApplicationRunCreateRequest{
		ApplicationVersionID: version.ID,
		Inputs:               req.Inputs,
		IdempotencyKey:       idempotencyKey,
	}
	run, lookupErr := s.Store.ApplicationPlatforms().GetApplicationRunByIdempotency(ctx, req.OwnerUserID, idempotencyKey)
	if lookupErr != nil {
		if !stderrors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return nil, lookupErr
		}
		resolved.EngineInstanceID = form.CompatibleEngineInstanceIDs[0]
		engine, engineErr := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, resolved.EngineInstanceID)
		if engineErr != nil {
			return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine instance is unavailable")
		}
		run = newApplicationRun(req.OwnerUserID, app, version, template, engine, resolved, original, form)
		run.ExecutionSnapshot["origin_type"] = "canvas"
		run.ExecutionSnapshot["canvas_run_id"] = req.CanvasRunID
		run.ExecutionSnapshot["canvas_node_run_id"] = req.CanvasNodeRunID
		run.ExecutionSnapshot["execution_key"] = req.ExecutionKey
		run, err = s.Store.ApplicationPlatforms().AddApplicationRun(ctx, run)
		if err != nil {
			run, lookupErr = s.Store.ApplicationPlatforms().GetApplicationRunByIdempotency(ctx, req.OwnerUserID, idempotencyKey)
			if lookupErr != nil {
				return nil, mapUnique(err, "idx_aiapp_runs_owner_idempotency", code.ErrAIAppApplicationRunCreateFailed, "canvas application run idempotency conflict")
			}
		}
	}
	if !contains(form.CompatibleEngineInstanceIDs, run.EngineInstanceID) {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "application run engine instance is no longer executable")
	}
	if !sameCanvasApplicationRun(run, req, version.ID, resolved.Inputs) {
		return nil, errors.NewStatus(code.ErrAIAppApplicationRunCreateFailed, "canvas application run idempotency conflict")
	}
	task, err := s.Tasks.BindApplicationRun(
		ctx,
		req.AtomicTaskID,
		run.ID,
		req.CanvasRunID,
		req.CanvasNodeRunID,
		req.ExecutionKey,
		req.Arguments,
	)
	if err != nil {
		return nil, err
	}
	if run.AtomicTaskID != nil && *run.AtomicTaskID != task.ID {
		return nil, errors.NewStatus(code.ErrAIAppApplicationRunCreateFailed, "application run is already bound to another atomic task")
	}
	if run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
		run, err = s.Store.ApplicationPlatforms().BindApplicationRunTask(
			ctx, run.ID, task.ID, iapiserver.TaskCreationCreated, task.Status, task.ResourceVersion, "",
		)
		if err != nil {
			return nil, err
		}
	}
	return run, nil
}

func sameCanvasApplicationRun(
	run *iapiserver.ApplicationRun,
	req *CanvasApplicationRunRequest,
	applicationVersionID string,
	resolvedInputs map[string]any,
) bool {
	return run != nil && req != nil &&
		run.ApplicationVersionID == applicationVersionID &&
		maputil.FirstString(run.ExecutionSnapshot, "origin_type") == "canvas" &&
		maputil.FirstString(run.ExecutionSnapshot, "canvas_run_id") == req.CanvasRunID &&
		maputil.FirstString(run.ExecutionSnapshot, "canvas_node_run_id") == req.CanvasNodeRunID &&
		maputil.FirstString(run.ExecutionSnapshot, "execution_key") == req.ExecutionKey &&
		reflect.DeepEqual(run.InputSnapshot, resolvedInputs) &&
		reflect.DeepEqual(run.ExecutionSnapshot["idempotency_inputs"], req.Inputs)
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
	engine, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, run.EngineInstanceID)
	if err != nil {
		return s.failTaskBinding(ctx, run, err)
	}
	operation := ""
	if templateVersion.ProviderOperationID != nil {
		operation = *templateVersion.ProviderOperationID
	}
	taskTimeoutSeconds := applicationRunTaskTimeoutSeconds(engine)
	task, err := s.Tasks.CreateAtomicTask(ctx, &iapiserver.AtomicTaskCreateRequest{
		Key: applicationRunTaskDefinitionID, Name: "Application Platform Run", Description: "Execute an immutable ApplicationRun snapshot",
		FunctionRef: applicationRunTaskFunctionRef, Arguments: map[string]any{"application_run_id": run.ID, "operation": operation, "execution_snapshot": run.ExecutionSnapshot},
		RequiredCapabilities: "application-platform", ApplicationRunID: run.ID,
		IdempotencyScope: "application-run", IdempotencyKey: run.IdempotencyKey,
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
		TimeoutPolicy: iapiserver.TimeoutPolicy{OverallTimeoutSeconds: taskTimeoutSeconds},
		SystemName:    iapiserver.SystemNameSpec{Key: taskname.ApplicationRun},
	})
	if err != nil {
		return s.failTaskBinding(ctx, run, err)
	}
	bound, err := s.Store.ApplicationPlatforms().BindApplicationRunTask(ctx, run.ID, task.ID, iapiserver.TaskCreationCreated, task.Status, task.ResourceVersion, "")
	if err != nil {
		return nil, err
	}
	return bound, nil
}
func (s *applicationPlatformService) failTaskBinding(ctx context.Context, run *iapiserver.ApplicationRun, cause error) (*iapiserver.ApplicationRun, error) {
	failed, storeErr := s.Store.ApplicationPlatforms().BindApplicationRunTask(ctx, run.ID, "", iapiserver.TaskCreationFailed, "", 0, cause.Error())
	if storeErr != nil {
		return nil, storeErr
	}
	return failed, errors.NewStatus(code.ErrAIAppAtomicTaskCreateFailed, cause.Error())
}

func applicationRunTaskTimeoutSeconds(engine *iapiserver.EngineInstance) int {
	if engine != nil && engine.TaskTimeoutSeconds > 0 {
		return engine.TaskTimeoutSeconds
	}
	return 30 * 60
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

func (s *applicationPlatformService) ListApplicationRuns(ctx context.Context, req *iapiserver.ApplicationRunListRequest) (*iapiserver.ApplicationRunListResponse, error) {
	if _, err := s.GetApplication(ctx, req.ApplicationID); err != nil {
		return nil, err
	}
	items, total, err := s.Store.ApplicationPlatforms().ListApplicationRuns(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		s.attachApplicationRunRelations(ctx, item)
	}
	return &iapiserver.ApplicationRunListResponse{Total: total, Items: items}, nil
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
	case "application_run_created", "application_run_atomic_task_bound":
		payload["occurred_at"] = occurredAt
	}
	if err := s.Events.Publish(context.WithoutCancel(ctx), &iapiserver.ApplicationPlatformEvent{Type: eventType, IdempotencyKey: key, Payload: payload, OccurredAt: occurredAt}); err != nil {
		log.Errorf("application platform event publish failed: type=%s key=%s error=%v", eventType, key, err)
	}
}

func newApplicationRun(owner string, app *iapiserver.Application, version *iapiserver.ApplicationVersion, template *iapiserver.ApplicationTemplateVersion, engine *iapiserver.EngineInstance, resolved, original *iapiserver.ApplicationRunCreateRequest, form *iapiserver.RuntimeFormSchema) *iapiserver.ApplicationRun {
	run := &iapiserver.ApplicationRun{OwnerUserID: owner, ApplicationID: app.ID, ApplicationVersionID: version.ID, ApplicationTemplateVersionID: template.ID, EngineInstanceID: engine.ID, CapabilitySourceType: template.CapabilitySourceType, SourceRevision: template.SourceRevision, ProviderCapabilityID: template.ProviderCapabilityID, ProviderCapabilityRevision: template.ProviderCapabilityRevision, ProviderOperationID: template.ProviderOperationID, WorkflowContractRevision: template.WorkflowContractRevision, CapabilitySourceSnapshot: map[string]any{"source_revision": template.SourceRevision, "template_contract": template.TemplateContract, "comfyui_api_workflow": template.ComfyUIAPIWorkflow, "application": applicationSummary(app), "application_version": applicationVersionSummary(version), "application_template_version": applicationTemplateVersionSummary(template), "engine_instance": engineInstanceRefSummary(engine)}, InputSnapshot: resolved.Inputs, ExecutionSnapshot: map[string]any{"inputs": resolved.Inputs, "application_version_id": version.ID, "template_version_id": template.ID, "engine_instance_id": engine.ID, "capability_definition_id": app.CapabilityDefinitionID, "idempotency_inputs": original.Inputs, "idempotency_engine_instance_id": original.EngineInstanceID}, OutputMappingSnapshot: version.OutputSchema, TaskCreationStatus: iapiserver.TaskCreationPending, OutputValues: []map[string]any{}, IdempotencyKey: original.IdempotencyKey, Artifacts: []*iapiserver.ApplicationArtifactRef{}}
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
	originType := maputil.FirstString(run.ExecutionSnapshot, "origin_type")
	if originType == "" {
		originType = "api"
	}
	return map[string]any{
		"application_run_id": run.ID, "application_id": run.ApplicationID, "application_version_id": run.ApplicationVersionID,
		"application_template_version_id": run.ApplicationTemplateVersionID, "engine_instance_id": run.EngineInstanceID,
		"capability_source_type": run.CapabilitySourceType, "source_revision": run.SourceRevision,
		"provider_capability_id": run.ProviderCapabilityID, "provider_capability_revision": run.ProviderCapabilityRevision,
		"provider_operation_id": run.ProviderOperationID, "workflow_contract_revision": run.WorkflowContractRevision,
		"execution_snapshot": run.ExecutionSnapshot, "origin_type": originType,
		"canvas_run_id":        maputil.FirstString(run.ExecutionSnapshot, "canvas_run_id"),
		"canvas_node_run_id":   maputil.FirstString(run.ExecutionSnapshot, "canvas_node_run_id"),
		"execution_key":        maputil.FirstString(run.ExecutionSnapshot, "execution_key"),
		"task_creation_status": run.TaskCreationStatus,
	}
}

func resolvedRuntimeInputs(inputs map[string]any, fields []iapiserver.RuntimeFormField) map[string]any {
	resolved := maputil.Clone(inputs)
	for _, field := range fields {
		if field.Value == nil {
			delete(resolved, field.Name)
			continue
		}
		resolved[field.Name] = field.Value
	}
	return resolved
}
func validateComfyUIWorkflow(workflow, objectInfo map[string]any) error {
	for nodeID, raw := range workflow {
		node, ok := raw.(map[string]any)
		if !ok {
			return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI node "+nodeID+" is invalid")
		}
		classType := typeutil.As[string](node["class_type"])
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
	mappings := typeutil.As[map[string]any](contract["request_mapping"])
	outputs := typeutil.As[map[string]any](contract["outputs"])
	if len(mappings) == 0 || len(outputs) == 0 {
		return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI contract requires request_mapping and outputs")
	}
	dummyInputs := make(map[string]any, len(mappings))
	for field := range mappings {
		dummyInputs[field] = "validation"
	}
	for field, raw := range typeutil.As[map[string]any](contract["parameters"]) {
		definition := typeutil.As[map[string]any](raw)
		if boolValue(definition["required"]) {
			if _, exists := mappings[field]; !exists {
				return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "required ComfyUI parameter has no request mapping: "+field)
			}
		}
	}
	_, err := engine.ApplyComfyInputs(workflow, dummyInputs, contract)
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
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func stringPtr(value string) *string { return &value }
