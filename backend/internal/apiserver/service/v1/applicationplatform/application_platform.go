package applicationplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

const (
	applicationRunTaskDefinitionID = "application-platform.application-run"
	applicationRunTaskFunctionRef  = "application-platform.run"
)

type ApplicationPlatformSrv interface {
	ListProviderAdapters(context.Context, *iapiserver.ProviderAdapterListRequest) (*iapiserver.ProviderAdapterListResponse, error)
	ListProviderOperations(context.Context, string, *iapiserver.ProviderOperationListRequest) (*iapiserver.ProviderOperationListResponse, error)
	ListTemplates(context.Context, *iapiserver.AppTemplateListRequest) (*iapiserver.AppTemplateListResponse, error)
	CreateTemplate(context.Context, *iapiserver.AppTemplateCreateRequest) (*iapiserver.AppTemplate, error)
	GetTemplate(context.Context, string) (*iapiserver.AppTemplate, error)
	GetTemplateCapabilityGraph(context.Context, string) (*iapiserver.CapabilityGraphResponse, error)
	UpdateTemplate(context.Context, *iapiserver.AppTemplateUpdateRequest) (*iapiserver.AppTemplate, error)
	DeleteTemplate(context.Context, string) (*iapiserver.SuccessResponse, error)
	ListTemplateReferences(context.Context, string, *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error)
	ConvertTemplateToApplication(context.Context, string, *iapiserver.ApplicationFromTemplateRequest) (*iapiserver.Application, error)
	ListApplications(context.Context, *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error)
	CreateApplication(context.Context, *iapiserver.ApplicationCreateRequest) (*iapiserver.Application, error)
	GetApplication(context.Context, string) (*iapiserver.Application, error)
	UpdateApplication(context.Context, *iapiserver.ApplicationUpdateRequest) (*iapiserver.Application, error)
	DeleteApplication(context.Context, string) (*iapiserver.SuccessResponse, error)
	ListInputMappings(context.Context, string) (*iapiserver.InputMappingListResponse, error)
	SaveInputMappings(context.Context, string, *iapiserver.InputMappingSaveRequest) (*iapiserver.InputMappingListResponse, error)
	ListOutputMappings(context.Context, string) (*iapiserver.OutputMappingListResponse, error)
	SaveOutputMappings(context.Context, string, *iapiserver.OutputMappingSaveRequest) (*iapiserver.OutputMappingListResponse, error)
	ListAvailableAppEngines(context.Context, string) (*iapiserver.AvailableEngineListResponse, error)
	ListAppEngines(context.Context, *iapiserver.AppEngineListRequest) (*iapiserver.AppEngineListResponse, error)
	CreateAppEngine(context.Context, *iapiserver.AppEngineCreateRequest) (*iapiserver.AppEngine, error)
	GetAppEngine(context.Context, string) (*iapiserver.AppEngine, error)
	UpdateAppEngine(context.Context, *iapiserver.AppEngineUpdateRequest) (*iapiserver.AppEngine, error)
	CheckAppEngineHealth(context.Context, string) (*iapiserver.AppEngineHealthCheckResult, error)
	CheckAppEngineHealthByConfig(context.Context, *iapiserver.AppEngineHealthCheckRequest) (*iapiserver.AppEngineHealthCheckResult, error)
	DeleteAppEngine(context.Context, string) (*iapiserver.SuccessResponse, error)
	CreateApplicationRun(context.Context, string, *iapiserver.ApplicationRunCreateRequest) (*iapiserver.ApplicationRun, error)
	CreateApplicationTestRun(context.Context, string, *iapiserver.ApplicationRunCreateRequest) (*iapiserver.ApplicationRun, error)
	ListApplicationRuns(context.Context, *iapiserver.ApplicationRunListRequest) (*iapiserver.ApplicationRunListResponse, error)
	GetApplicationRun(context.Context, string) (*iapiserver.ApplicationRun, error)
}

type applicationPlatformService struct {
	store    store.Factory
	catalog  *providerCatalog
	checkers map[string]AppEngineHealthyChecker
}

func NewService(str store.Factory) *applicationPlatformService {
	return &applicationPlatformService{
		store:    str,
		catalog:  defaultProviderCatalog(),
		checkers: defaultAppEngineCheckers(),
	}
}

func (s *applicationPlatformService) ListProviderAdapters(
	_ context.Context,
	req *iapiserver.ProviderAdapterListRequest,
) (*iapiserver.ProviderAdapterListResponse, error) {
	items := s.catalog.listAdapters(req)
	total := int64(len(items))
	items = paginateAdapters(items, req.PageNum, req.PageSize)
	return &iapiserver.ProviderAdapterListResponse{Items: items, Page: pageInfo(req.PageNum, req.PageSize, total)}, nil
}

func (s *applicationPlatformService) ListProviderOperations(
	_ context.Context,
	adapterKey string,
	req *iapiserver.ProviderOperationListRequest,
) (*iapiserver.ProviderOperationListResponse, error) {
	return &iapiserver.ProviderOperationListResponse{Items: s.catalog.listOperations(adapterKey, req)}, nil
}

func (s *applicationPlatformService) ListTemplates(
	ctx context.Context,
	req *iapiserver.AppTemplateListRequest,
) (*iapiserver.AppTemplateListResponse, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID, req.IncludeAll = p.userID, p.admin
	items, total, err := s.store.ApplicationPlatforms().ListTemplates(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AppTemplateListResponse{Items: items, Page: pageInfo(req.PageNum, req.PageSize, total)}, nil
}

func (s *applicationPlatformService) CreateTemplate(
	ctx context.Context,
	req *iapiserver.AppTemplateCreateRequest,
) (*iapiserver.AppTemplate, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	op, err := s.catalog.requireOperation(req.AdapterKey, req.OperationKey, req.OperationVersion)
	if err != nil {
		return nil, err
	}
	if op.SourceMode == iapiserver.SourceModeDirect {
		return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "direct operation cannot create workflow template")
	}
	graph, requiredNodeTypes, requiredModelRefs, err := buildCapabilityGraphFromRaw(req.RawConfig, op)
	if err != nil {
		return nil, err
	}
	tpl := &iapiserver.AppTemplate{
		OwnerUserID:               p.userID,
		SourceKind:                req.SourceKind,
		AdapterKey:                req.AdapterKey,
		OperationKey:              req.OperationKey,
		OperationVersion:          req.OperationVersion,
		RawConfig:                 copyMap(req.RawConfig),
		CapabilityGraph:           graph,
		RequiredNodeTypes:         requiredNodeTypes,
		RequiredModelRefs:         requiredModelRefs,
		ReferenceApplicationCount: 0,
	}
	tpl.Name = req.Name
	tpl.Description = req.Description
	created, err := s.store.ApplicationPlatforms().AddTemplate(ctx, tpl)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *applicationPlatformService) GetTemplate(ctx context.Context, id string) (*iapiserver.AppTemplate, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	tpl, err := s.store.ApplicationPlatforms().GetTemplate(ctx, id)
	if err != nil || !p.canAccess(tpl.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "template not found or not visible")
	}
	return tpl, nil
}

func (s *applicationPlatformService) GetTemplateCapabilityGraph(
	ctx context.Context,
	id string,
) (*iapiserver.CapabilityGraphResponse, error) {
	tpl, err := s.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	return &iapiserver.CapabilityGraphResponse{Data: tpl.CapabilityGraph}, nil
}

func (s *applicationPlatformService) UpdateTemplate(
	ctx context.Context,
	req *iapiserver.AppTemplateUpdateRequest,
) (*iapiserver.AppTemplate, error) {
	tpl, err := s.GetTemplate(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		tpl.Name = *req.Name
	}
	if req.Description != nil {
		tpl.Description = *req.Description
	}
	return s.store.ApplicationPlatforms().UpdateTemplate(ctx, tpl)
}

func (s *applicationPlatformService) DeleteTemplate(ctx context.Context, id string) (*iapiserver.SuccessResponse, error) {
	if _, err := s.GetTemplate(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.ApplicationPlatforms().DeleteTemplate(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *applicationPlatformService) ListTemplateReferences(
	ctx context.Context,
	templateID string,
	req *iapiserver.ApplicationListRequest,
) (*iapiserver.ApplicationListResponse, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	req.TemplateID, req.OwnerUserID, req.IncludeAll = templateID, p.userID, p.admin
	return s.listApplications(ctx, req)
}

func (s *applicationPlatformService) ConvertTemplateToApplication(
	ctx context.Context,
	templateID string,
	req *iapiserver.ApplicationFromTemplateRequest,
) (*iapiserver.Application, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	tpl, err := s.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	op, err := s.catalog.requireOperation(tpl.AdapterKey, tpl.OperationKey, tpl.OperationVersion)
	if err != nil {
		return nil, err
	}
	inputs, outputs, err := buildMappingsFromGraph("", tpl.CapabilityGraph, req.InputMappings.Items, req.OutputMappings.Items)
	if err != nil {
		return nil, err
	}
	if err := validateFixedParameters(req.FixedParameters, inputs); err != nil {
		return nil, err
	}
	app := newApplicationFromTemplate(p.userID, tpl, op, req)
	created, err := s.store.ApplicationPlatforms().AddApplication(ctx, app, inputs, outputs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetApplication(ctx, created.ID)
}

func (s *applicationPlatformService) ListApplications(
	ctx context.Context,
	req *iapiserver.ApplicationListRequest,
) (*iapiserver.ApplicationListResponse, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID, req.IncludeAll = p.userID, p.admin
	return s.listApplications(ctx, req)
}

func (s *applicationPlatformService) listApplications(
	ctx context.Context,
	req *iapiserver.ApplicationListRequest,
) (*iapiserver.ApplicationListResponse, error) {
	items, total, err := s.store.ApplicationPlatforms().ListApplications(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ApplicationListResponse{Items: items, Page: pageInfo(req.PageNum, req.PageSize, total)}, nil
}

func (s *applicationPlatformService) CreateApplication(
	ctx context.Context,
	req *iapiserver.ApplicationCreateRequest,
) (*iapiserver.Application, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	op, err := s.catalog.requireOperation(req.AdapterKey, req.OperationKey, req.OperationVersion)
	if err != nil {
		return nil, err
	}
	if op.SourceMode != iapiserver.SourceModeDirect {
		return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "operation is not direct")
	}
	graph := graphFromOperation(op)
	inputs, outputs, err := buildMappingsFromGraph("", graph, req.InputMappings.Items, req.OutputMappings.Items)
	if err != nil {
		return nil, err
	}
	if err := validateFixedParameters(req.FixedParameters, inputs); err != nil {
		return nil, err
	}
	app := &iapiserver.Application{
		OwnerUserID:      p.userID,
		SourceType:       iapiserver.ApplicationSourceTypeProviderOperation,
		AdapterKey:       req.AdapterKey,
		OperationKey:     req.OperationKey,
		OperationVersion: req.OperationVersion,
		CapabilityType:   op.CapabilityType,
		FixedParameters:  copyMap(req.FixedParameters),
	}
	app.Name = req.Name
	app.Description = req.Description
	created, err := s.store.ApplicationPlatforms().AddApplication(ctx, app, inputs, outputs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetApplication(ctx, created.ID)
}

func (s *applicationPlatformService) GetApplication(ctx context.Context, id string) (*iapiserver.Application, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	app, err := s.store.ApplicationPlatforms().GetApplication(ctx, id)
	if err != nil || !p.canAccess(app.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "application not found or not visible")
	}
	if err := s.hydrateApplicationMappings(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *applicationPlatformService) UpdateApplication(
	ctx context.Context,
	req *iapiserver.ApplicationUpdateRequest,
) (*iapiserver.Application, error) {
	app, err := s.GetApplication(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Description != nil {
		app.Description = *req.Description
	}
	if req.FixedParameters != nil {
		if err := validateFixedParameters(*req.FixedParameters, app.InputMappings); err != nil {
			return nil, err
		}
		app.FixedParameters = copyMap(*req.FixedParameters)
	}
	updated, err := s.store.ApplicationPlatforms().UpdateApplication(ctx, app)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetApplication(ctx, updated.ID)
}

func (s *applicationPlatformService) DeleteApplication(ctx context.Context, id string) (*iapiserver.SuccessResponse, error) {
	if _, err := s.GetApplication(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.ApplicationPlatforms().DeleteApplication(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *applicationPlatformService) ListInputMappings(
	ctx context.Context,
	applicationID string,
) (*iapiserver.InputMappingListResponse, error) {
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return &iapiserver.InputMappingListResponse{Items: app.InputMappings}, nil
}

func (s *applicationPlatformService) SaveInputMappings(
	ctx context.Context,
	applicationID string,
	req *iapiserver.InputMappingSaveRequest,
) (*iapiserver.InputMappingListResponse, error) {
	app, graph, err := s.applicationGraph(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	inputs, err := buildInputMappings(app.ID, graph, req.Items)
	if err != nil {
		return nil, err
	}
	if err := validateFixedParameters(app.FixedParameters, inputs); err != nil {
		return nil, err
	}
	items, err := s.store.ApplicationPlatforms().ReplaceInputMappings(ctx, app.ID, inputs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.InputMappingListResponse{Items: items}, nil
}

func (s *applicationPlatformService) ListOutputMappings(
	ctx context.Context,
	applicationID string,
) (*iapiserver.OutputMappingListResponse, error) {
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return &iapiserver.OutputMappingListResponse{Items: app.OutputMappings}, nil
}

func (s *applicationPlatformService) SaveOutputMappings(
	ctx context.Context,
	applicationID string,
	req *iapiserver.OutputMappingSaveRequest,
) (*iapiserver.OutputMappingListResponse, error) {
	app, graph, err := s.applicationGraph(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	outputs, err := buildOutputMappings(app.ID, graph, req.Items)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ApplicationPlatforms().ReplaceOutputMappings(ctx, app.ID, outputs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.OutputMappingListResponse{Items: items}, nil
}

func (s *applicationPlatformService) ListAvailableAppEngines(
	ctx context.Context,
	applicationID string,
) (*iapiserver.AvailableEngineListResponse, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	engines, err := s.store.ApplicationPlatforms().ListAvailableAppEngines(ctx, app, p.userID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	items := make([]*iapiserver.AvailableEngine, 0, len(engines))
	for _, engine := range engines {
		items = append(items, availableEngine(engine))
	}
	return &iapiserver.AvailableEngineListResponse{Items: items}, nil
}

func (s *applicationPlatformService) ListAppEngines(
	ctx context.Context,
	req *iapiserver.AppEngineListRequest,
) (*iapiserver.AppEngineListResponse, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID, req.IncludeAll = p.userID, p.admin
	items, total, err := s.store.ApplicationPlatforms().ListAppEngines(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AppEngineListResponse{Items: items, Page: pageInfo(req.PageNum, req.PageSize, total)}, nil
}

func (s *applicationPlatformService) CreateAppEngine(
	ctx context.Context,
	req *iapiserver.AppEngineCreateRequest,
) (*iapiserver.AppEngine, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateAuthConfig(req.AuthType, req.AuthConfig); err != nil {
		return nil, err
	}
	engine := &iapiserver.AppEngine{
		OwnerUserID:         p.userID,
		AdapterKey:          req.AdapterKey,
		Endpoint:            req.Endpoint,
		AuthType:            req.AuthType,
		AuthConfig:          req.AuthConfig,
		SupportedOperations: req.SupportedOperations,
		NodeTypes:           req.NodeTypes,
		ModelRefs:           req.ModelRefs,
		Priority:            req.Priority,
		MaxConcurrency:      req.MaxConcurrency,
		Status:              iapiserver.AppEngineStatusActive,
		HealthStatus:        iapiserver.AppEngineHealthUnknown,
	}
	engine.Name = req.Name
	engine.Description = req.Description
	return s.store.ApplicationPlatforms().AddAppEngine(ctx, engine)
}

func (s *applicationPlatformService) GetAppEngine(ctx context.Context, id string) (*iapiserver.AppEngine, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	engine, err := s.store.ApplicationPlatforms().GetAppEngine(ctx, id)
	if err != nil || !p.canAccess(engine.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "app engine not found or not visible")
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
		engine.Name = *req.Name
	}
	if req.Description != nil {
		engine.Description = *req.Description
	}
	if req.Endpoint != nil {
		engine.Endpoint = *req.Endpoint
	}
	if req.AuthType != nil {
		engine.AuthType = *req.AuthType
	}
	if req.AuthConfig != nil {
		engine.AuthConfig = *req.AuthConfig
	}
	if err := validateAuthConfig(engine.AuthType, engine.AuthConfig); err != nil {
		return nil, err
	}
	if req.Status != nil {
		engine.Status = *req.Status
	}
	if req.SupportedOperations != nil {
		engine.SupportedOperations = *req.SupportedOperations
	}
	if req.NodeTypes != nil {
		engine.NodeTypes = *req.NodeTypes
	}
	if req.ModelRefs != nil {
		engine.ModelRefs = *req.ModelRefs
	}
	if req.Priority != nil {
		engine.Priority = *req.Priority
	}
	if req.MaxConcurrency != nil {
		engine.MaxConcurrency = *req.MaxConcurrency
	}
	return s.store.ApplicationPlatforms().UpdateAppEngine(ctx, engine)
}

func (s *applicationPlatformService) CheckAppEngineHealth(
	ctx context.Context,
	id string,
) (*iapiserver.AppEngineHealthCheckResult, error) {
	engine, err := s.GetAppEngine(ctx, id)
	if err != nil {
		return nil, err
	}
	result, err := s.checkEngine(ctx, engine.AdapterKey, engine.Endpoint, engine.AuthType, engine.AuthConfig)
	if err != nil {
		return nil, err
	}
	engine.HealthStatus = result.HealthStatus
	engine.LastHealthCheckAt = &result.CheckedAt
	engine.UnhealthyReason = result.UnhealthyReason
	if _, err := s.store.ApplicationPlatforms().UpdateAppEngine(ctx, engine); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *applicationPlatformService) CheckAppEngineHealthByConfig(
	ctx context.Context,
	req *iapiserver.AppEngineHealthCheckRequest,
) (*iapiserver.AppEngineHealthCheckResult, error) {
	return s.checkEngine(ctx, req.AdapterKey, req.Endpoint, req.AuthType, req.AuthConfig)
}

func (s *applicationPlatformService) DeleteAppEngine(ctx context.Context, id string) (*iapiserver.SuccessResponse, error) {
	if _, err := s.GetAppEngine(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.ApplicationPlatforms().DeleteAppEngine(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *applicationPlatformService) CreateApplicationRun(
	ctx context.Context,
	applicationID string,
	req *iapiserver.ApplicationRunCreateRequest,
) (*iapiserver.ApplicationRun, error) {
	return s.createApplicationRun(ctx, applicationID, iapiserver.RunModeNormal, req)
}

func (s *applicationPlatformService) CreateApplicationTestRun(
	ctx context.Context,
	applicationID string,
	req *iapiserver.ApplicationRunCreateRequest,
) (*iapiserver.ApplicationRun, error) {
	return s.createApplicationRun(ctx, applicationID, iapiserver.RunModeTest, req)
}

func (s *applicationPlatformService) createApplicationRun(
	ctx context.Context,
	applicationID, runMode string,
	req *iapiserver.ApplicationRunCreateRequest,
) (*iapiserver.ApplicationRun, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	engine, err := s.resolveAndReserveEngine(ctx, app, p.userID, req.AppEngineID)
	if err != nil {
		return nil, err
	}
	rendered, err := renderApplicationPayload(app, req.Input)
	if err != nil {
		_ = s.store.ApplicationPlatforms().ReleaseAppEngine(ctx, engine.ID)
		return nil, err
	}
	run := &iapiserver.ApplicationRun{
		OwnerUserID:             p.userID,
		ApplicationID:           app.ID,
		RunMode:                 runMode,
		RequestedEngineID:       req.AppEngineID,
		ResolvedEngineID:        engine.ID,
		AdapterKey:              app.AdapterKey,
		OperationKey:            app.OperationKey,
		OperationVersion:        app.OperationVersion,
		InputSnapshot:           copyMap(req.Input),
		RenderedPayloadSnapshot: rendered,
		OutputMappingSnapshot:   outputMappingValues(app.OutputMappings),
		TaskStatusProjection:    iapiserver.TaskRunStatusReady,
		OutputValues:            []iapiserver.ApplicationOutputValue{},
	}
	run.Name = app.Name + "-run"
	taskRun := &iapiserver.TaskRun{
		DefinitionType:    iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:      applicationRunTaskDefinitionID,
		Status:            iapiserver.TaskRunStatusReady,
		AdapterKey:        app.AdapterKey,
		OperationKey:      app.OperationKey,
		OperationVersion:  app.OperationVersion,
		RequestedEngineID: req.AppEngineID,
		ResolvedEngineID:  engine.ID,
		Input:             applicationTaskInput(app, run, rendered),
		ProjectID:         iapiserver.DefaultTaskCenterProjectID,
		Namespace:         iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:         p.userID,
		MaxAttempts:       1,
	}
	created, err := s.store.ApplicationPlatforms().CreateApplicationRun(ctx, run, applicationRunTaskDefinition(), taskRun)
	if err != nil {
		_ = s.store.ApplicationPlatforms().ReleaseAppEngine(ctx, engine.ID)
		return nil, errors.NewStatusF(code.ErrAdapterInvocationFailed, "application run create failed")
	}
	if runMode == iapiserver.RunModeTest {
		now := imachinery.Now()
		app.LatestTestStatus = iapiserver.LatestTestStatusRunning
		app.LastTestedAt = &now
		_, _ = s.store.ApplicationPlatforms().UpdateApplication(ctx, app)
	}
	return created, nil
}

func (s *applicationPlatformService) ListApplicationRuns(
	ctx context.Context,
	req *iapiserver.ApplicationRunListRequest,
) (*iapiserver.ApplicationRunListResponse, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID, req.IncludeAll = p.userID, p.admin
	items, total, err := s.store.ApplicationPlatforms().ListApplicationRuns(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ApplicationRunListResponse{Items: items, Page: pageInfo(req.PageNum, req.PageSize, total)}, nil
}

func (s *applicationPlatformService) GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error) {
	p, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.store.ApplicationPlatforms().GetApplicationRun(ctx, id)
	if err != nil || !p.canAccess(run.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "application run not found or not visible")
	}
	return run, nil
}

func (s *applicationPlatformService) hydrateApplicationMappings(ctx context.Context, app *iapiserver.Application) error {
	inputs, err := s.store.ApplicationPlatforms().ListInputMappings(ctx, app.ID)
	if err != nil {
		return errors.WithStack(err)
	}
	outputs, err := s.store.ApplicationPlatforms().ListOutputMappings(ctx, app.ID)
	if err != nil {
		return errors.WithStack(err)
	}
	app.InputMappings = inputs
	app.OutputMappings = outputs
	return nil
}

func (s *applicationPlatformService) applicationGraph(
	ctx context.Context,
	applicationID string,
) (*iapiserver.Application, iapiserver.CapabilityGraph, error) {
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, iapiserver.CapabilityGraph{}, err
	}
	if app.SourceType == iapiserver.ApplicationSourceTypeTemplate {
		tpl, err := s.GetTemplate(ctx, app.TemplateID)
		if err != nil {
			return nil, iapiserver.CapabilityGraph{}, err
		}
		return app, tpl.CapabilityGraph, nil
	}
	op, err := s.catalog.requireOperation(app.AdapterKey, app.OperationKey, app.OperationVersion)
	if err != nil {
		return nil, iapiserver.CapabilityGraph{}, err
	}
	return app, graphFromOperation(op), nil
}

func (s *applicationPlatformService) resolveAndReserveEngine(
	ctx context.Context,
	app *iapiserver.Application,
	ownerUserID, requestedID string,
) (*iapiserver.AppEngine, error) {
	if requestedID != "" {
		engine, err := s.GetAppEngine(ctx, requestedID)
		if err != nil || engine.OwnerUserID != ownerUserID || !engineSupportsApp(engine, app) {
			return nil, errors.NewStatusF(code.ErrSelectedEngineUnavailable, "selected app engine is unavailable")
		}
		return s.store.ApplicationPlatforms().ReserveAppEngine(ctx, engine.ID)
	}
	items, err := s.store.ApplicationPlatforms().ListAvailableAppEngines(ctx, app, ownerUserID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.NewStatusF(code.ErrEngineUnavailable, "no available app engine")
	}
	return s.store.ApplicationPlatforms().ReserveAppEngine(ctx, items[0].ID)
}

func (s *applicationPlatformService) checkEngine(
	ctx context.Context,
	adapterKey, endpoint, authType string,
	authConfig iapiserver.AppEngineAuthConfig,
) (*iapiserver.AppEngineHealthCheckResult, error) {
	if err := validateAuthConfig(authType, authConfig); err != nil {
		return nil, err
	}
	checker := s.checkers[adapterKey]
	if checker == nil {
		checker = s.checkers["default"]
	}
	started := time.Now()
	healthy, reason := checker.Check(ctx, &iapiserver.AppEngine{
		AdapterKey: adapterKey,
		Endpoint:   endpoint,
		AuthType:   authType,
		AuthConfig: authConfig,
	})
	status := iapiserver.AppEngineHealthHealthy
	if !healthy {
		status = iapiserver.AppEngineHealthUnhealthy
	}
	return &iapiserver.AppEngineHealthCheckResult{
		HealthStatus:    status,
		CheckedAt:       imachinery.Now(),
		UnhealthyReason: reason,
		LatencyMS:       time.Since(started).Milliseconds(),
	}, nil
}

type AppEngineHealthyChecker interface {
	Check(context.Context, *iapiserver.AppEngine) (bool, string)
}

type httpAppEngineHealthyChecker struct {
	client *http.Client
}

func (c *httpAppEngineHealthyChecker) Check(
	ctx context.Context,
	engine *iapiserver.AppEngine,
) (bool, string) {
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := strings.TrimRight(engine.Endpoint, "/")
	method := http.MethodGet
	if engine.HealthCheckConfig.Path != "" {
		endpoint += engine.HealthCheckConfig.Path
	}
	if engine.HealthCheckConfig.Method != "" {
		method = engine.HealthCheckConfig.Method
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return false, err.Error()
	}
	applyAuthHeader(req, engine.AuthType, engine.AuthConfig)
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	expected := engine.HealthCheckConfig.ExpectedStatus
	if expected == 0 {
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return true, ""
		}
		return false, fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	if resp.StatusCode == expected {
		return true, ""
	}
	return false, fmt.Sprintf("unexpected status %d", resp.StatusCode)
}

type comfyUIHealthyChecker struct {
	httpAppEngineHealthyChecker
}

func (c *comfyUIHealthyChecker) Check(ctx context.Context, engine *iapiserver.AppEngine) (bool, string) {
	copied := *engine
	if copied.HealthCheckConfig.Path == "" {
		copied.HealthCheckConfig.Path = "/system_stats"
	}
	return c.httpAppEngineHealthyChecker.Check(ctx, &copied)
}

func defaultAppEngineCheckers() map[string]AppEngineHealthyChecker {
	defaultChecker := &httpAppEngineHealthyChecker{client: &http.Client{Timeout: 5 * time.Second}}
	return map[string]AppEngineHealthyChecker{
		"default":                            defaultChecker,
		iapiserver.ProviderAdapterKeyComfyUI: &comfyUIHealthyChecker{httpAppEngineHealthyChecker: *defaultChecker},
	}
}

type providerCatalog struct {
	adapters   []*iapiserver.ProviderAdapter
	operations map[string][]*iapiserver.ProviderOperation
}

func defaultProviderCatalog() *providerCatalog {
	ops := map[string][]*iapiserver.ProviderOperation{
		iapiserver.ProviderAdapterKeyComfyUI: {
			operation(iapiserver.ProviderAdapterKeyComfyUI, iapiserver.ProviderOperationComfyUIRun, "v1", "ComfyUI Workflow Run", "image_generation", iapiserver.SourceModeWorkflow, iapiserver.ExecutionModeAsynchronous),
		},
		iapiserver.ProviderAdapterKeyByteDance: {
			operation(iapiserver.ProviderAdapterKeyByteDance, iapiserver.ProviderOperationSeedanceText2V, "v1", "Seedance Text to Video", "video_generation", iapiserver.SourceModeDirect, iapiserver.ExecutionModeAsynchronous),
		},
		iapiserver.ProviderAdapterKeyOpenAI: {
			operation(iapiserver.ProviderAdapterKeyOpenAI, iapiserver.ProviderOperationOpenAIImageGen, "v1", "GPT Image Generate", "image_generation", iapiserver.SourceModeDirect, iapiserver.ExecutionModeAsynchronous),
			operation(iapiserver.ProviderAdapterKeyOpenAI, iapiserver.ProviderOperationOpenAIImageEdit, "v1", "GPT Image Edit", "image_editing", iapiserver.SourceModeDirect, iapiserver.ExecutionModeAsynchronous),
		},
	}
	adapters := make([]*iapiserver.ProviderAdapter, 0, len(ops))
	for adapterKey, operations := range ops {
		keys := make([]string, 0, len(operations))
		for _, op := range operations {
			keys = append(keys, op.OperationKey)
		}
		sort.Strings(keys)
		adapters = append(adapters, &iapiserver.ProviderAdapter{
			AdapterKey:    adapterKey,
			Name:          adapterDisplayName(adapterKey),
			PlatformType:  adapterKey,
			Version:       "v1",
			OperationKeys: keys,
			Enabled:       true,
		})
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].AdapterKey < adapters[j].AdapterKey })
	return &providerCatalog{adapters: adapters, operations: ops}
}

func operation(
	adapterKey, operationKey, version, name, capabilityType, sourceMode, executionMode string,
) *iapiserver.ProviderOperation {
	return &iapiserver.ProviderOperation{
		AdapterKey:           adapterKey,
		OperationKey:         operationKey,
		OperationVersion:     version,
		Name:                 name,
		CapabilityType:       capabilityType,
		SourceMode:           sourceMode,
		ExecutionMode:        executionMode,
		InputSchema:          map[string]any{"ports": []any{}},
		OutputSchema:         map[string]any{"ports": []any{}},
		ProgressSupported:    true,
		CancelSupported:      true,
		IdempotencySupported: true,
	}
}

func adapterDisplayName(adapterKey string) string {
	switch adapterKey {
	case iapiserver.ProviderAdapterKeyComfyUI:
		return "ComfyUI"
	case iapiserver.ProviderAdapterKeyByteDance:
		return "ByteDance Seedance"
	case iapiserver.ProviderAdapterKeyOpenAI:
		return "OpenAI"
	default:
		return adapterKey
	}
}

func (c *providerCatalog) listAdapters(req *iapiserver.ProviderAdapterListRequest) []*iapiserver.ProviderAdapter {
	items := make([]*iapiserver.ProviderAdapter, 0, len(c.adapters))
	for _, adapter := range c.adapters {
		if req.PlatformType != "" && adapter.PlatformType != req.PlatformType {
			continue
		}
		if req.Enabled != nil && adapter.Enabled != *req.Enabled {
			continue
		}
		if req.CapabilityType != "" && !c.adapterHasCapability(adapter.AdapterKey, req.CapabilityType) {
			continue
		}
		items = append(items, adapter)
	}
	return items
}

func (c *providerCatalog) adapterHasCapability(adapterKey, capabilityType string) bool {
	for _, op := range c.operations[adapterKey] {
		if op.CapabilityType == capabilityType {
			return true
		}
	}
	return false
}

func (c *providerCatalog) listOperations(
	adapterKey string,
	req *iapiserver.ProviderOperationListRequest,
) []*iapiserver.ProviderOperation {
	items := make([]*iapiserver.ProviderOperation, 0)
	for _, op := range c.operations[adapterKey] {
		if req.SourceMode != "" && op.SourceMode != req.SourceMode {
			continue
		}
		if req.CapabilityType != "" && op.CapabilityType != req.CapabilityType {
			continue
		}
		items = append(items, op)
	}
	return items
}

func (c *providerCatalog) requireOperation(adapterKey, operationKey, version string) (*iapiserver.ProviderOperation, error) {
	for _, op := range c.operations[adapterKey] {
		if op.OperationKey == operationKey && op.OperationVersion == version {
			return op, nil
		}
	}
	return nil, errors.NewStatusF(code.ErrOperationVersionIncompatible, "provider operation is not registered")
}

func graphFromOperation(op *iapiserver.ProviderOperation) iapiserver.CapabilityGraph {
	node := iapiserver.CapabilityNode{
		NodeKey:          op.OperationKey,
		NodeType:         op.OperationKey,
		Name:             op.Name,
		ResolutionStatus: "resolved",
		InputPorts: []iapiserver.PortDefinition{
			{PortKey: "input", NodeKey: op.OperationKey, Direction: iapiserver.PortDirectionInput, DataType: iapiserver.PortDataTypeJSON, Required: true, Cardinality: iapiserver.CardinalitySingle, SourcePath: "input", Candidate: false},
		},
		OutputPorts: []iapiserver.PortDefinition{
			{PortKey: "output", NodeKey: op.OperationKey, Direction: iapiserver.PortDirectionOutput, DataType: outputDataType(op.CapabilityType), Required: true, Cardinality: iapiserver.CardinalitySingle, SourcePath: "output", SemanticRole: "primary_output", Candidate: false},
		},
	}
	return iapiserver.CapabilityGraph{Nodes: []iapiserver.CapabilityNode{node}, Edges: []iapiserver.CapabilityEdge{}, GraphVersion: "v1", UnresolvedNodeCount: 0}
}

func buildCapabilityGraphFromRaw(
	raw map[string]any,
	op *iapiserver.ProviderOperation,
) (iapiserver.CapabilityGraph, []string, []string, error) {
	if len(raw) == 0 {
		return iapiserver.CapabilityGraph{}, nil, nil, errors.NewStatusF(code.ErrTemplateParseFailed, "raw_config is required")
	}
	nodes := make([]iapiserver.CapabilityNode, 0)
	requiredNodeTypes := map[string]struct{}{}
	if op.AdapterKey == iapiserver.ProviderAdapterKeyComfyUI {
		nodeKeys := sortedKeys(raw)
		for _, nodeKey := range nodeKeys {
			nodeRaw, _ := raw[nodeKey].(map[string]any)
			nodeType, _ := nodeRaw["class_type"].(string)
			if nodeType == "" {
				nodeType = "unknown"
			}
			requiredNodeTypes[nodeType] = struct{}{}
			inputs, _ := nodeRaw["inputs"].(map[string]any)
			nodes = append(nodes, iapiserver.CapabilityNode{
				NodeKey:          nodeKey,
				NodeType:         nodeType,
				Name:             nodeType,
				InputPorts:       portsFromMap(nodeKey, inputs),
				OutputPorts:      defaultOutputPorts(nodeKey, nodeType, op.CapabilityType),
				ResolutionStatus: "resolved",
				RawReference:     map[string]any{"node_key": nodeKey},
			})
		}
	} else {
		graph := graphFromOperation(op)
		return graph, []string{op.OperationKey}, nil, nil
	}
	if len(nodes) == 0 {
		return iapiserver.CapabilityGraph{}, nil, nil, errors.NewStatusF(code.ErrTemplateParseFailed, "workflow has no nodes")
	}
	return iapiserver.CapabilityGraph{
		Nodes:               nodes,
		Edges:               []iapiserver.CapabilityEdge{},
		GraphVersion:        "v1",
		UnresolvedNodeCount: 0,
	}, setKeys(requiredNodeTypes), nil, nil
}

func portsFromMap(nodeKey string, inputs map[string]any) []iapiserver.PortDefinition {
	keys := sortedKeys(inputs)
	ports := make([]iapiserver.PortDefinition, 0, len(keys))
	for _, key := range keys {
		value := inputs[key]
		if isComfyUILink(value) {
			continue
		}
		ports = append(ports, iapiserver.PortDefinition{
			PortKey:     key,
			NodeKey:     nodeKey,
			Direction:   iapiserver.PortDirectionInput,
			DataType:    inferPortDataType(value),
			Required:    true,
			Cardinality: iapiserver.CardinalitySingle,
			SourcePath:  nodeKey + ".inputs." + key,
			Candidate:   false,
		})
	}
	return ports
}

func defaultOutputPorts(nodeKey, nodeType, capabilityType string) []iapiserver.PortDefinition {
	return []iapiserver.PortDefinition{{
		PortKey:      "output",
		NodeKey:      nodeKey,
		Direction:    iapiserver.PortDirectionOutput,
		DataType:     outputDataType(capabilityType),
		Required:     true,
		Cardinality:  iapiserver.CardinalitySingle,
		SourcePath:   nodeKey + ".outputs.output",
		SemanticRole: "primary_output",
		Candidate:    false,
		MediaType:    "",
	}}
}

func buildMappingsFromGraph(
	applicationID string,
	graph iapiserver.CapabilityGraph,
	inputReqs []iapiserver.InputMappingInput,
	outputReqs []iapiserver.OutputMappingInput,
) ([]*iapiserver.InputMapping, []*iapiserver.OutputMapping, error) {
	inputs, err := buildInputMappings(applicationID, graph, inputReqs)
	if err != nil {
		return nil, nil, err
	}
	outputs, err := buildOutputMappings(applicationID, graph, outputReqs)
	if err != nil {
		return nil, nil, err
	}
	return inputs, outputs, nil
}

func buildInputMappings(
	applicationID string,
	graph iapiserver.CapabilityGraph,
	reqs []iapiserver.InputMappingInput,
) ([]*iapiserver.InputMapping, error) {
	ports := graphInputPorts(graph)
	seenKeys, seenPaths := map[string]struct{}{}, map[string]struct{}{}
	items := make([]*iapiserver.InputMapping, 0, len(reqs))
	for _, req := range reqs {
		port, ok := ports[portRef(req.SourcePortKey, req.SourcePath)]
		if !ok {
			return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "input source port is invalid")
		}
		if port.Candidate || port.Direction != iapiserver.PortDirectionInput {
			return nil, errors.NewStatusF(code.ErrPortUnresolved, "input port is not resolved")
		}
		if req.DataType != port.DataType {
			return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "input data_type is invalid")
		}
		if err := markUnique(seenKeys, req.InputKey, code.ErrInputMappingInvalid, "input key duplicated"); err != nil {
			return nil, err
		}
		if err := markUnique(seenPaths, req.SourcePath, code.ErrInputMappingInvalid, "input source path duplicated"); err != nil {
			return nil, err
		}
		item := &iapiserver.InputMapping{
			ApplicationID: applicationID,
			InputKey:      req.InputKey,
			InputLabel:    req.InputLabel,
			SourcePortKey: req.SourcePortKey,
			SourcePath:    req.SourcePath,
			DataType:      req.DataType,
			Required:      port.Required,
			DefaultValue:  req.DefaultValue,
			SortOrder:     req.SortOrder,
		}
		item.Name = req.InputKey
		items = append(items, item)
	}
	return items, nil
}

func graphInputPorts(graph iapiserver.CapabilityGraph) map[string]iapiserver.PortDefinition {
	ret := map[string]iapiserver.PortDefinition{}
	for _, node := range graph.Nodes {
		for _, port := range node.InputPorts {
			ret[portRef(port.PortKey, port.SourcePath)] = port
		}
	}
	return ret
}

func graphOutputPorts(graph iapiserver.CapabilityGraph) map[string]iapiserver.PortDefinition {
	ret := map[string]iapiserver.PortDefinition{}
	for _, node := range graph.Nodes {
		for _, port := range node.OutputPorts {
			ret[portRef(port.PortKey, port.SourcePath)] = port
		}
	}
	return ret
}

func portRef(portKey, sourcePath string) string {
	return portKey + "\x00" + sourcePath
}

func validateFixedParameters(fixed map[string]any, inputs []*iapiserver.InputMapping) error {
	if len(fixed) == 0 {
		return nil
	}
	openPaths := map[string]struct{}{}
	for _, input := range inputs {
		openPaths[input.SourcePath] = struct{}{}
	}
	for path := range flattenPaths("", fixed) {
		if _, ok := openPaths[path]; ok {
			return errors.NewStatusF(code.ErrFixedParameterConflicted, "fixed parameter conflicts with input mapping")
		}
	}
	return nil
}

func flattenPaths(prefix string, value any) map[string]struct{} {
	ret := map[string]struct{}{}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			for path := range flattenPaths(joinPath(prefix, key), child) {
				ret[path] = struct{}{}
			}
		}
	default:
		if prefix != "" {
			ret[prefix] = struct{}{}
		}
	}
	return ret
}

func renderApplicationPayload(app *iapiserver.Application, input map[string]any) (map[string]any, error) {
	payload := copyMap(app.FixedParameters)
	if len(app.InputMappings) == 0 && len(app.FieldMappings) > 0 {
		for _, mapping := range app.FieldMappings {
			value, ok := input[mapping.FieldKey]
			if !ok || value == nil {
				value = mapping.DefaultValue
			}
			if mapping.Required && value == nil {
				return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "required input %s is missing", mapping.FieldKey)
			}
			if value != nil {
				payload[mapping.SourcePath] = value
			}
		}
		return payload, nil
	}
	for _, mapping := range app.InputMappings {
		value, ok := input[mapping.InputKey]
		if !ok || value == nil {
			value = mapping.DefaultValue
		}
		if mapping.Required && value == nil {
			return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "required input %s is missing", mapping.InputKey)
		}
		if value != nil {
			payload[mapping.SourcePath] = value
		}
	}
	return payload, nil
}

func newApplicationFromTemplate(
	ownerUserID string,
	tpl *iapiserver.AppTemplate,
	op *iapiserver.ProviderOperation,
	req *iapiserver.ApplicationFromTemplateRequest,
) *iapiserver.Application {
	app := &iapiserver.Application{
		OwnerUserID:      ownerUserID,
		SourceType:       iapiserver.ApplicationSourceTypeTemplate,
		TemplateID:       tpl.ID,
		AdapterKey:       tpl.AdapterKey,
		OperationKey:     tpl.OperationKey,
		OperationVersion: tpl.OperationVersion,
		CapabilityType:   op.CapabilityType,
		FixedParameters:  copyMap(req.FixedParameters),
	}
	app.Name = req.Name
	app.Description = req.Description
	return app
}

func applicationRunTaskDefinition() *iapiserver.TaskDefinition {
	definition := &iapiserver.TaskDefinition{
		DefinitionType:       iapiserver.TaskDefinitionTypeAtomic,
		FunctionRef:          applicationRunTaskFunctionRef,
		RequiredCapabilities: "application-platform",
		ProjectID:            iapiserver.DefaultTaskCenterProjectID,
		Namespace:            iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:            iapiserver.DefaultTaskCenterCreatedBy,
	}
	definition.ID = applicationRunTaskDefinitionID
	definition.Name = "Application Platform Run"
	definition.Description = "Application Platform delegated AppRun execution"
	return definition
}

func applicationTaskInput(
	app *iapiserver.Application,
	run *iapiserver.ApplicationRun,
	rendered map[string]any,
) map[string]any {
	return map[string]any{
		"application_id":      run.ApplicationID,
		"application_run_id":  run.ID,
		"adapter_key":         app.AdapterKey,
		"operation_key":       app.OperationKey,
		"operation_version":   app.OperationVersion,
		"requested_engine_id": run.RequestedEngineID,
		"resolved_engine_id":  run.ResolvedEngineID,
		"rendered_payload":    rendered,
	}
}

func outputMappingValues(items []*iapiserver.OutputMapping) []iapiserver.OutputMapping {
	ret := make([]iapiserver.OutputMapping, 0, len(items))
	for _, item := range items {
		if item != nil {
			ret = append(ret, *item)
		}
	}
	return ret
}

func availableEngine(engine *iapiserver.AppEngine) *iapiserver.AvailableEngine {
	return &iapiserver.AvailableEngine{
		ID:                  engine.ID,
		Name:                engine.Name,
		AdapterKey:          engine.AdapterKey,
		SupportedOperations: engine.SupportedOperations,
		HealthStatus:        engine.HealthStatus,
		RuntimeVersion:      engine.RuntimeVersion,
		MaxConcurrency:      engine.MaxConcurrency,
		CurrentInflight:     engine.CurrentInflight,
		CapabilitySummary: map[string]any{
			"node_types": len(engine.NodeTypes),
			"model_refs": len(engine.ModelRefs),
		},
	}
}

func engineSupportsApp(engine *iapiserver.AppEngine, app *iapiserver.Application) bool {
	if engine.AdapterKey != app.AdapterKey ||
		engine.Status != iapiserver.AppEngineStatusActive ||
		engine.HealthStatus != iapiserver.AppEngineHealthHealthy ||
		engine.CurrentInflight >= engine.MaxConcurrency {
		return false
	}
	for _, op := range engine.SupportedOperations {
		if op.OperationKey != app.OperationKey {
			continue
		}
		if op.MinVersion != "" && app.OperationVersion < op.MinVersion {
			continue
		}
		if op.MaxVersion != "" && app.OperationVersion > op.MaxVersion {
			continue
		}
		return true
	}
	return false
}

func validateAuthConfig(authType string, cfg iapiserver.AppEngineAuthConfig) error {
	switch authType {
	case iapiserver.AppEngineAuthNone:
		return nil
	case iapiserver.AppEngineAuthBearerToken:
		if cfg.BearerToken != "" || cfg.Token != "" {
			return nil
		}
	case iapiserver.AppEngineAuthAPIKey:
		if cfg.APIKey != "" {
			return nil
		}
	case iapiserver.AppEngineAuthAKSK:
		if cfg.AccessKey != "" && cfg.SecretKey != "" {
			return nil
		}
	}
	return errors.NewStatusF(code.ErrAppEngineAuthConfigInvalid, "app engine auth config invalid")
}

func applyAuthHeader(req *http.Request, authType string, cfg iapiserver.AppEngineAuthConfig) {
	switch authType {
	case iapiserver.AppEngineAuthBearerToken:
		token := cfg.BearerToken
		if token == "" {
			token = cfg.Token
		}
		req.Header.Set("Authorization", "Bearer "+token)
	case iapiserver.AppEngineAuthAPIKey:
		req.Header.Set("X-API-Key", cfg.APIKey)
	case iapiserver.AppEngineAuthAKSK:
		req.Header.Set("X-Access-Key", cfg.AccessKey)
		req.Header.Set("X-Secret-Key", cfg.SecretKey)
	}
}

func validateAppEngineAuthConfig(authType string, cfg iapiserver.AppEngineAuthConfig) error {
	return validateAuthConfig(authType, cfg)
}

func validateAppEngineSaaSConfig(engineType, platformType string, capabilities []string) error {
	if engineType != iapiserver.AppEngineTypeSaaSAPI {
		return nil
	}
	if platformType == "" {
		return errors.NewStatusF(code.ErrAppEngineAuthConfigInvalid, "saas platform type is required")
	}
	for _, capability := range capabilities {
		switch capability {
		case iapiserver.CapabilityImageGeneration,
			iapiserver.CapabilityImageEditing,
			iapiserver.CapabilityVideoGeneration:
		default:
			return errors.NewStatusF(code.ErrOperationVersionIncompatible, "capability unsupported")
		}
	}
	return nil
}

func parseTemplateFields(kind string, config map[string]any) ([]iapiserver.ParsedField, error) {
	if len(config) == 0 {
		return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template config is required")
	}
	var root any = config
	switch kind {
	case iapiserver.AppTemplateKindComfyUI:
		if raw, ok := config["raw"].(string); ok {
			if err := json.Unmarshal([]byte(raw), &root); err != nil {
				return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui raw json parse failed")
			}
		}
		workflow, ok := root.(map[string]any)
		if !ok || len(workflow) == 0 {
			return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui workflow must be an object")
		}
		if _, ok := workflow["nodes"].([]any); ok {
			return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui workflow must use api format")
		}
		fields := []iapiserver.ParsedField{}
		for _, nodeID := range sortedKeys(workflow) {
			node, ok := workflow[nodeID].(map[string]any)
			if !ok {
				return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui node must be an object")
			}
			classType, _ := node["class_type"].(string)
			if classType == "" {
				return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui node misses class_type")
			}
			inputs, _ := node["inputs"].(map[string]any)
			label := classType
			if meta, ok := node["_meta"].(map[string]any); ok {
				if title, ok := meta["title"].(string); ok && title != "" {
					label = title
				}
			}
			appendParsedInputFields(nodeID, "", label, inputs, &fields)
		}
		if len(fields) == 0 {
			return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template has no primitive leaf fields")
		}
		sortParsedFields(fields)
		return fields, nil
	case iapiserver.AppTemplateKindSaaSAPI:
		if requestTemplate, ok := config["requestTemplate"]; ok {
			root = requestTemplate
		}
		fields := []iapiserver.ParsedField{}
		appendPrimitiveFields("", root, &fields)
		if len(fields) == 0 {
			return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template has no primitive leaf fields")
		}
		sortParsedFields(fields)
		return fields, nil
	default:
		return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template kind is unsupported")
	}
}

func appendParsedInputFields(nodeID, prefix, label string, inputs map[string]any, out *[]iapiserver.ParsedField) {
	for _, key := range sortedKeys(inputs) {
		value := inputs[key]
		if isComfyUILink(value) {
			continue
		}
		path := joinPath(prefix, key)
		switch typed := value.(type) {
		case map[string]any:
			appendParsedInputFields(nodeID, path, label, typed, out)
		default:
			*out = append(*out, iapiserver.ParsedField{
				SourcePath: nodeID + ".inputs." + path,
				FieldType:  legacyFieldType(value),
				Required:   true,
				LabelHint:  label + "." + lastPathPart(path),
			})
		}
	}
}

func appendPrimitiveFields(prefix string, value any, out *[]iapiserver.ParsedField) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			appendPrimitiveFields(joinPath(prefix, key), typed[key], out)
		}
	default:
		if prefix != "" {
			*out = append(*out, iapiserver.ParsedField{
				SourcePath: prefix,
				FieldType:  legacyFieldType(value),
				Required:   true,
				LabelHint:  lastPathPart(prefix),
			})
		}
	}
}

func buildFieldMappings(
	applicationID, templateID string,
	parsed []iapiserver.ParsedField,
	inputs []iapiserver.FieldMappingInput,
) ([]*iapiserver.FieldMapping, error) {
	parsedByPath := map[string]iapiserver.ParsedField{}
	for _, item := range parsed {
		parsedByPath[item.SourcePath] = item
	}
	seen := map[string]struct{}{}
	ret := make([]*iapiserver.FieldMapping, 0, len(inputs))
	for _, input := range inputs {
		if err := markUnique(seen, input.FieldKey, code.ErrInputMappingInvalid, "field key duplicated"); err != nil {
			return nil, err
		}
		field, ok := parsedByPath[input.SourcePath]
		if !ok {
			return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "source path invalid")
		}
		if field.FieldType != input.FieldType {
			return nil, errors.NewStatusF(code.ErrInputMappingInvalid, "field type invalid")
		}
		item := &iapiserver.FieldMapping{
			ApplicationID: applicationID,
			TemplateID:    templateID,
			FieldKey:      input.FieldKey,
			FieldLabel:    input.FieldLabel,
			FieldType:     input.FieldType,
			SourcePath:    input.SourcePath,
			DefaultValue:  input.DefaultValue,
			Required:      field.Required,
			SortOrder:     input.SortOrder,
		}
		ret = append(ret, item)
	}
	return ret, nil
}

func validateTemplateSaaSConfig(req *iapiserver.AppTemplateCreateRequest) error {
	if req.Kind != iapiserver.AppTemplateKindSaaSAPI {
		return nil
	}
	return validateAppEngineSaaSConfig(iapiserver.AppEngineTypeSaaSAPI, req.SaaSPlatformType, []string{req.CapabilityType})
}

func validateApplicationRunEngine(app *iapiserver.Application, engine *iapiserver.AppEngine) error {
	if engine.Status != iapiserver.AppEngineStatusActive || engine.HealthStatus != iapiserver.AppEngineHealthHealthy {
		return errors.NewStatusF(code.ErrSelectedEngineUnavailable, "app engine is not available")
	}
	if app.Kind == iapiserver.AppTemplateKindSaaSAPI {
		if engine.EngineType != iapiserver.AppEngineTypeSaaSAPI || app.SaaSPlatformType != engine.SaaSPlatformType {
			return errors.NewStatusF(code.ErrSelectedEngineUnavailable, "saas platform mismatched")
		}
		for _, capability := range engine.SupportedCapabilityTypes {
			if capability == app.CapabilityType {
				return nil
			}
		}
		return errors.NewStatusF(code.ErrSelectedEngineUnavailable, "capability unsupported")
	}
	return nil
}

func legacyFieldType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return "number"
	default:
		return "null"
	}
}

func sortParsedFields(fields []iapiserver.ParsedField) {
	sort.Slice(fields, func(i, j int) bool { return fields[i].SourcePath < fields[j].SourcePath })
}

func lastPathPart(path string) string {
	parts := strings.Split(path, ".")
	return strings.Trim(parts[len(parts)-1], "[]")
}

func markUnique(seen map[string]struct{}, value string, errCode int, msg string) error {
	if strings.TrimSpace(value) == "" {
		return errors.NewStatus(errCode, msg)
	}
	if _, ok := seen[value]; ok {
		return errors.NewStatus(errCode, msg)
	}
	seen[value] = struct{}{}
	return nil
}

func inferPortDataType(value any) string {
	switch value.(type) {
	case string:
		return iapiserver.PortDataTypeText
	case bool:
		return iapiserver.PortDataTypeBoolean
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return iapiserver.PortDataTypeNumber
	case []any:
		return iapiserver.PortDataTypeArray
	case map[string]any:
		return iapiserver.PortDataTypeJSON
	default:
		return iapiserver.PortDataTypeJSON
	}
}

func outputDataType(capabilityType string) string {
	switch capabilityType {
	case "video_generation":
		return iapiserver.PortDataTypeVideo
	case "image_generation", "image_editing":
		return iapiserver.PortDataTypeImage
	default:
		return iapiserver.PortDataTypeJSON
	}
}

func isComfyUILink(value any) bool {
	items, ok := value.([]any)
	if !ok || len(items) != 2 {
		return false
	}
	_, firstIsString := items[0].(string)
	return firstIsString
}

func sortedKeys(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func setKeys(data map[string]struct{}) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func copyMap(src map[string]any) map[string]any {
	ret := make(map[string]any, len(src))
	for key, value := range src {
		ret[key] = value
	}
	return ret
}

func paginateAdapters(items []*iapiserver.ProviderAdapter, pageNum, pageSize int) []*iapiserver.ProviderAdapter {
	if pageNum <= 0 || pageSize <= 0 {
		return items
	}
	start := (pageNum - 1) * pageSize
	if start >= len(items) {
		return []*iapiserver.ProviderAdapter{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func pageInfo(pageNum, pageSize int, total int64) iapiserver.PageInfo {
	return iapiserver.PageInfo{PageNum: pageNum, PageSize: pageSize, Total: total}
}

type principal struct {
	userID string
	admin  bool
}

func (p principal) canAccess(ownerUserID string) bool {
	return p.admin || ownerUserID == "" || ownerUserID == p.userID
}

func (s *applicationPlatformService) currentPrincipal(ctx context.Context) (principal, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return principal{userID: "system-admin", admin: true}, nil
	}
	return principal{userID: user.ID, admin: false}, nil
}

func dumpJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func buildOutputMappings(
	applicationID string,
	graph iapiserver.CapabilityGraph,
	reqs []iapiserver.OutputMappingInput,
) ([]*iapiserver.OutputMapping, error) {
	ports := graphOutputPorts(graph)
	seenKeys, seenPaths := map[string]struct{}{}, map[string]struct{}{}
	primaryCount := 0
	items := make([]*iapiserver.OutputMapping, 0, len(reqs))
	for _, req := range reqs {
		port, ok := ports[portRef(req.SourcePortKey, req.SourcePath)]
		if !ok {
			return nil, errors.NewStatusF(code.ErrOutputMappingInvalid, "output source port is invalid")
		}
		if port.Candidate {
			return nil, errors.NewStatusF(code.ErrCandidateOutputUnconfirmed, "candidate output is not confirmed")
		}
		if port.Direction != iapiserver.PortDirectionOutput || req.DataType != port.DataType {
			return nil, errors.NewStatusF(code.ErrOutputMappingInvalid, "output mapping is invalid")
		}
		if req.Primary {
			primaryCount++
		}
		if err := markUnique(seenKeys, req.OutputKey, code.ErrOutputMappingInvalid, "output key duplicated"); err != nil {
			return nil, err
		}
		if err := markUnique(seenPaths, req.SourcePath, code.ErrOutputMappingInvalid, "output source path duplicated"); err != nil {
			return nil, err
		}
		item := &iapiserver.OutputMapping{
			ApplicationID:   applicationID,
			OutputKey:       req.OutputKey,
			OutputLabel:     req.OutputLabel,
			SourcePortKey:   req.SourcePortKey,
			SourcePath:      req.SourcePath,
			DataType:        req.DataType,
			Cardinality:     req.Cardinality,
			Primary:         req.Primary,
			Materialization: req.Materialization,
			SortOrder:       req.SortOrder,
		}
		item.Name = req.OutputKey
		items = append(items, item)
	}
	if primaryCount > 1 {
		return nil, errors.NewStatusF(code.ErrOutputMappingInvalid, "only one primary output is allowed")
	}
	return items, nil
}
