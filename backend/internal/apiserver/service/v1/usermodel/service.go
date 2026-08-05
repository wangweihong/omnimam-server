package usermodel

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

const (
	CapabilityTextChatCompletion = "text.chat_completion"
	CapabilityTextTranslate      = "text.translate"
)

type Dependencies struct {
	Store       store.Factory
	Gateway     modelgateway.UserModelGateway
	Credentials *CredentialBroker
}

type Service struct {
	store       store.Factory
	gateway     modelgateway.UserModelGateway
	credentials *CredentialBroker
}

type ExecutionContextRequest struct {
	ModelID                         string
	CapabilityDefinitionID          string
	RequiredCapabilityDefinitionIDs []string
	DefaultUsage                    string
}

func New(deps Dependencies) (*Service, error) {
	if deps.Store == nil || deps.Gateway == nil || deps.Credentials == nil {
		return nil, errors.New("user model store, gateway, and credential broker are required")
	}
	return &Service{store: deps.Store, gateway: deps.Gateway, credentials: deps.Credentials}, nil
}

func (s *Service) ListProviderTypes(ctx context.Context) (*iapiserver.ProviderTypeListResponse, error) {
	items, err := s.gateway.ListProviderTypes(ctx)
	if err != nil {
		return nil, mapProviderGatewayError(err)
	}
	return &iapiserver.ProviderTypeListResponse{Total: len(items), Items: items}, nil
}

func (s *Service) ListProviders(ctx context.Context, req *iapiserver.ProviderListRequest) (*iapiserver.ProviderListResponse, error) {
	req.OwnerUserID = currentUserID(ctx)
	items, total, err := s.store.Providers().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ProviderListResponse{Total: total, Items: items}, nil
}

func (s *Service) GetProvider(ctx context.Context, id string) (*iapiserver.Provider, error) {
	provider, err := s.store.Providers().GetOwned(ctx, currentUserID(ctx), id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	return provider, nil
}

func (s *Service) CreateProvider(ctx context.Context, req *iapiserver.ProviderCreateRequest) (*iapiserver.Provider, error) {
	if err := s.validateProviderType(ctx, req.Type, req.AuthType); err != nil {
		return nil, err
	}
	provider := &iapiserver.Provider{
		OwnerUserID: currentUserID(ctx), Type: strings.TrimSpace(req.Type), Enabled: valueOr(req.Enabled, true),
		BaseURL: strings.TrimSpace(req.BaseURL), AuthType: strings.TrimSpace(req.AuthType),
		CredentialRef: req.CredentialRef, Config: req.Config,
	}
	provider.Name = strings.TrimSpace(req.Name)
	provider.Description = strings.TrimSpace(req.Description)
	created, err := s.store.Providers().Add(ctx, provider)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *Service) UpdateProvider(ctx context.Context, id string, req *iapiserver.ProviderUpdateRequest) (*iapiserver.Provider, error) {
	provider, err := s.GetProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	nextType := provider.Type
	nextAuthType := provider.AuthType
	if req.Type != nil {
		nextType = strings.TrimSpace(*req.Type)
	}
	if req.AuthType != nil {
		nextAuthType = strings.TrimSpace(*req.AuthType)
	}
	if err := s.validateProviderType(ctx, nextType, nextAuthType); err != nil {
		return nil, err
	}
	if req.Name != nil {
		provider.Name = strings.TrimSpace(*req.Name)
	}
	provider.Type = nextType
	provider.AuthType = nextAuthType
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if req.BaseURL != nil {
		provider.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.CredentialRef != nil {
		provider.CredentialRef = *req.CredentialRef
	}
	if req.Config != nil {
		provider.Config = *req.Config
	}
	if req.Description != nil {
		provider.Description = strings.TrimSpace(*req.Description)
	}
	updated, err := s.store.Providers().Update(ctx, provider)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	owner := currentUserID(ctx)
	if _, err := s.store.Providers().GetOwned(ctx, owner, id); err != nil {
		return errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	if err := s.store.Providers().DeleteOwnedCascade(ctx, owner, id); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Service) TestUnsavedProvider(ctx context.Context, req *iapiserver.ProviderTestRequest) (*iapiserver.ProviderTestResponse, error) {
	provider := &iapiserver.Provider{
		Type: strings.TrimSpace(req.Type), BaseURL: strings.TrimSpace(req.BaseURL), AuthType: strings.TrimSpace(req.AuthType),
		CredentialRef: req.CredentialRef, Config: req.Config,
	}
	if err := s.testProviderConnection(ctx, provider); err != nil {
		return nil, err
	}
	return providerTestResponse(provider.ID), nil
}

func (s *Service) TestProvider(ctx context.Context, id string) (*iapiserver.ProviderTestResponse, error) {
	provider, err := s.GetProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	now := imachinery.NewTime(time.Now().UTC())
	if err := s.testProviderConnection(ctx, provider); err != nil {
		s.persistHealthCheck(ctx, &iapiserver.ModelHealthCheck{
			OwnerUserID: provider.OwnerUserID, TargetType: "provider", ProviderID: provider.ID,
			Success: false, HealthStatus: iapiserver.ProviderModelHealthUnhealthy,
			Message: gatewayErrorCategory(err), CheckedAt: now,
		})
		return nil, err
	}
	s.persistHealthCheck(ctx, &iapiserver.ModelHealthCheck{
		OwnerUserID: provider.OwnerUserID, TargetType: "provider", ProviderID: provider.ID,
		Success: true, HealthStatus: iapiserver.ProviderModelHealthHealthy,
		Message: "provider connection ok", CheckedAt: now,
	})
	response := providerTestResponse(provider.ID)
	response.CheckedAt = now
	return response, nil
}

func (s *Service) ListProviderModels(ctx context.Context, providerID string, req *iapiserver.ProviderModelListRequest) (*iapiserver.ProviderModelListResponse, error) {
	owner := currentUserID(ctx)
	provider, err := s.store.Providers().GetOwned(ctx, owner, providerID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	capability := req.Capability
	req.OwnerUserID = owner
	req.ProviderID = providerID
	req.Capability = ""
	items, _, err := s.store.ProviderModels().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	items = s.projectModels(provider, items)
	if capability != "" {
		items = filterModelsByCapability(items, capability)
	}
	return &iapiserver.ProviderModelListResponse{Total: int64(len(items)), Items: items}, nil
}

func (s *Service) CreateProviderModel(ctx context.Context, providerID string, req *iapiserver.ProviderModelCreateRequest) (*iapiserver.ProviderModel, error) {
	owner := currentUserID(ctx)
	provider, err := s.store.Providers().GetOwned(ctx, owner, providerID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	if err := s.ensureModelUnique(ctx, owner, providerID, req.Name, req.Model, ""); err != nil {
		return nil, err
	}
	model := &iapiserver.ProviderModel{
		OwnerUserID: owner, ProviderID: providerID, Model: strings.TrimSpace(req.Model),
		DisplayName: strings.TrimSpace(req.Name), GroupName: strings.TrimSpace(req.GroupName),
		FeatureLabels: append([]string(nil), req.FeatureLabels...), Enabled: valueOr(req.Enabled, true),
		HealthStatus: iapiserver.ProviderModelHealthUnknown,
	}
	model.Name = model.DisplayName
	created, err := s.store.ProviderModels().Add(ctx, model)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.projectModel(provider, created)
	return created, nil
}

func (s *Service) UpdateProviderModel(ctx context.Context, id string, req *iapiserver.ProviderModelUpdateRequest) (*iapiserver.ProviderModel, error) {
	owner := currentUserID(ctx)
	model, err := s.store.ProviderModels().GetOwned(ctx, owner, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrProviderModelNotFound, "provider model not found")
	}
	provider, err := s.store.Providers().GetOwned(ctx, owner, model.ProviderID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	nextName := model.DisplayName
	if req.Name != nil {
		nextName = strings.TrimSpace(*req.Name)
	}
	if err := s.ensureModelUnique(ctx, owner, model.ProviderID, nextName, model.Model, model.ID); err != nil {
		return nil, err
	}
	if req.DisabledCapabilityDefinitionIDs != nil {
		if model.HealthStatus != iapiserver.ProviderModelHealthHealthy {
			return nil, errors.NewStatus(code.ErrModelHealthCheckFailed, "provider model capabilities have not been verified")
		}
		if _, err := s.gateway.ResolveUserModelCapabilities(provider.Type, verifiedProbe(model), *req.DisabledCapabilityDefinitionIDs); err != nil {
			return nil, mapModelGatewayError(err)
		}
		model.DisabledCapabilityDefinitionIDs = append([]string(nil), (*req.DisabledCapabilityDefinitionIDs)...)
	}
	model.Name = nextName
	model.DisplayName = nextName
	if req.GroupName != nil {
		model.GroupName = strings.TrimSpace(*req.GroupName)
	}
	if req.FeatureLabels != nil {
		model.FeatureLabels = append([]string(nil), (*req.FeatureLabels)...)
	}
	if req.Enabled != nil {
		model.Enabled = *req.Enabled
	}
	updated, err := s.store.ProviderModels().Update(ctx, model)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.projectModel(provider, updated)
	return updated, nil
}

func (s *Service) DeleteProviderModel(ctx context.Context, id string) error {
	owner := currentUserID(ctx)
	if _, err := s.store.ProviderModels().GetOwned(ctx, owner, id); err != nil {
		return errors.NewStatus(code.ErrProviderModelNotFound, "provider model not found")
	}
	if err := s.store.ProviderModels().DeleteOwnedCascade(ctx, owner, id); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Service) SyncProviderModels(ctx context.Context, providerID string) (*iapiserver.ProviderModelSyncResponse, error) {
	owner := currentUserID(ctx)
	provider, err := s.store.Providers().GetOwned(ctx, owner, providerID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	request, err := s.connectionRequest(provider)
	if err != nil {
		return nil, err
	}
	discovered, err := s.gateway.DiscoverProviderModels(ctx, request)
	if err != nil {
		return nil, mapProviderGatewayError(err)
	}
	existing, _, err := s.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{OwnerUserID: owner, ProviderID: providerID})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	byRemote := make(map[string]struct{}, len(existing))
	for _, model := range existing {
		byRemote[model.Model] = struct{}{}
	}
	created := make([]*iapiserver.ProviderModel, 0)
	skipped := 0
	for _, remote := range discovered {
		if _, ok := byRemote[remote.RemoteModel]; ok {
			skipped++
			continue
		}
		model := &iapiserver.ProviderModel{
			OwnerUserID: owner, ProviderID: providerID, Model: remote.RemoteModel,
			DisplayName: remote.DisplayName, Enabled: true, HealthStatus: iapiserver.ProviderModelHealthUnknown,
		}
		model.Name = model.DisplayName
		item, addErr := s.store.ProviderModels().Add(ctx, model)
		if addErr != nil {
			return nil, errors.WithStack(addErr)
		}
		s.projectModel(provider, item)
		created = append(created, item)
	}
	return &iapiserver.ProviderModelSyncResponse{
		Total: len(discovered), Created: len(created), Updated: 0, Skipped: skipped, Models: created,
	}, nil
}

func (s *Service) TestProviderModel(ctx context.Context, id string) (*iapiserver.ProviderModelHealthCheckResponse, error) {
	return s.testProviderModelOwned(ctx, currentUserID(ctx), id)
}

func (s *Service) StartProviderModelHealthChecks(stopCh <-chan struct{}, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	run := func() {
		enabled := true
		models, _, err := s.store.ProviderModels().List(context.Background(), &iapiserver.ProviderModelListRequest{Enabled: &enabled})
		if err != nil {
			log.Errorf("user model health list failed: %v", err)
			return
		}
		for _, model := range models {
			checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if _, err := s.testProviderModelOwned(checkCtx, model.OwnerUserID, model.ID); err != nil {
				log.Errorf("user model health check failed: %v", err)
			}
			cancel()
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-stopCh:
				return
			}
		}
	}()
}

func (s *Service) testProviderModelOwned(ctx context.Context, owner, id string) (*iapiserver.ProviderModelHealthCheckResponse, error) {
	model, err := s.store.ProviderModels().GetOwned(ctx, owner, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrProviderModelNotFound, "provider model not found")
	}
	provider, err := s.store.Providers().GetOwned(ctx, owner, model.ProviderID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	request, err := s.connectionRequest(provider)
	if err != nil {
		return nil, err
	}
	now := imachinery.NewTime(time.Now().UTC())
	probe, probeErr := s.gateway.ProbeProviderModel(ctx, request, model.Model)
	if probeErr != nil {
		model.HealthStatus = iapiserver.ProviderModelHealthUnhealthy
		model.HealthReason = gatewayErrorCategory(probeErr)
		model.HealthCheckedAt = &now
		_, _ = s.store.ProviderModels().Update(context.WithoutCancel(ctx), model)
		s.persistHealthCheck(ctx, &iapiserver.ModelHealthCheck{
			OwnerUserID: owner, TargetType: "model", ProviderID: provider.ID, ModelID: model.ID,
			Success: false, HealthStatus: model.HealthStatus, Message: model.HealthReason, CheckedAt: now,
		})
		return nil, mapModelGatewayError(probeErr)
	}
	resolution, err := s.gateway.ResolveUserModelCapabilities(provider.Type, probe, model.DisabledCapabilityDefinitionIDs)
	if err != nil {
		return nil, mapModelGatewayError(err)
	}
	model.HealthStatus = iapiserver.ProviderModelHealthHealthy
	model.HealthReason = ""
	model.HealthCheckedAt = &now
	applyResolution(model, resolution)
	if _, err := s.store.ProviderModels().Update(context.WithoutCancel(ctx), model); err != nil {
		return nil, errors.WithStack(err)
	}
	s.persistHealthCheck(ctx, &iapiserver.ModelHealthCheck{
		OwnerUserID: owner, TargetType: "model", ProviderID: provider.ID, ModelID: model.ID,
		Success: true, HealthStatus: model.HealthStatus, Message: "provider model is available", CheckedAt: now,
	})
	return &iapiserver.ProviderModelHealthCheckResponse{
		TargetType: "model", ProviderID: provider.ID, ModelID: model.ID, Success: true,
		HealthStatus: model.HealthStatus, Message: "provider model is available", CheckedAt: now,
	}, nil
}

func (s *Service) GetDefaultModel(ctx context.Context, usage string) (*iapiserver.SystemLLMConfig, error) {
	if capabilityForUsage(usage) == "" {
		return nil, errors.NewStatus(code.ErrDefaultModelMissing, "default model usage is invalid")
	}
	owner := currentUserID(ctx)
	config, err := s.store.SystemLLMConfigs().GetOwned(ctx, owner, usage)
	if err != nil {
		return nil, errors.NewStatus(code.ErrDefaultModelMissing, "default model is missing")
	}
	model, err := s.store.ProviderModels().GetOwned(ctx, owner, config.ModelID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrDefaultModelMissing, "default model is missing")
	}
	provider, err := s.store.Providers().GetOwned(ctx, owner, config.ProviderID)
	if err != nil || model.ProviderID != config.ProviderID {
		return nil, errors.NewStatus(code.ErrDefaultModelMissing, "default model is missing")
	}
	s.projectModel(provider, model)
	config.ModelDetail = model
	return config, nil
}

func (s *Service) SaveDefaultModel(ctx context.Context, usage string, req *iapiserver.DefaultModelSaveRequest) (*iapiserver.SystemLLMConfig, error) {
	capability := capabilityForUsage(usage)
	if capability == "" {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "default model usage is invalid")
	}
	owner := currentUserID(ctx)
	provider, err := s.store.Providers().GetOwned(ctx, owner, req.ProviderID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "default model provider is invalid")
	}
	model, err := s.store.ProviderModels().GetOwned(ctx, owner, req.ModelID)
	if err != nil || model.ProviderID != provider.ID {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "default model candidate is invalid")
	}
	s.projectModel(provider, model)
	if !provider.Enabled || !model.Enabled || model.HealthStatus != iapiserver.ProviderModelHealthHealthy ||
		model.CapabilityResolutionStatus != modelgateway.CapabilityResolutionResolved || !model.Executable ||
		!slices.Contains(model.Capabilities, capability) {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "default model candidate is unavailable")
	}
	config := &iapiserver.SystemLLMConfig{
		OwnerUserID: owner, Purpose: usage, ProviderID: provider.ID, ModelID: model.ID, ModelDetail: model,
	}
	config.Name = usage
	saved, err := s.store.SystemLLMConfigs().Upsert(ctx, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	saved.ModelDetail = model
	return saved, nil
}

func (s *Service) ListModelOptions(ctx context.Context, req *iapiserver.ProviderModelListRequest) (*iapiserver.ModelOptionListResponse, error) {
	owner := currentUserID(ctx)
	capability := req.Capability
	if capability == "" && req.Usage != "" {
		capability = capabilityForUsage(req.Usage)
	}
	enabled := true
	req.OwnerUserID = owner
	req.Enabled = &enabled
	req.Capability = ""
	models, _, err := s.store.ProviderModels().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	providerIDs := make([]string, 0, len(models))
	for _, model := range models {
		providerIDs = append(providerIDs, model.ProviderID)
	}
	providers, err := s.store.Providers().GetByIDs(ctx, owner, providerIDs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	byID := make(map[string]*iapiserver.Provider, len(providers))
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	items := make([]*iapiserver.ProviderModel, 0, len(models))
	for _, model := range models {
		provider := byID[model.ProviderID]
		if provider == nil || !provider.Enabled {
			continue
		}
		s.projectModel(provider, model)
		if model.HealthStatus != iapiserver.ProviderModelHealthHealthy || !model.Executable {
			continue
		}
		if capability != "" && !slices.Contains(model.Capabilities, capability) {
			continue
		}
		items = append(items, model)
	}
	return &iapiserver.ModelOptionListResponse{Total: int64(len(items)), Items: items}, nil
}

// ResolveUserModelExecutionContext validates the current User Model facts and issues a short-lived execution grant.
func (s *Service) ResolveUserModelExecutionContext(ctx context.Context, req ExecutionContextRequest) (*modelgateway.UserModelExecutionContext, error) {
	owner := currentUserID(ctx)
	capabilityID := strings.TrimSpace(req.CapabilityDefinitionID)
	if capabilityID == "" {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "execution capability is required")
	}
	modelID := strings.TrimSpace(req.ModelID)
	var defaultConfig *iapiserver.SystemLLMConfig
	if modelID == "" {
		if capabilityForUsage(req.DefaultUsage) != capabilityID {
			return nil, errors.NewStatus(code.ErrDefaultModelMissing, "default model usage does not match the requested capability")
		}
		var err error
		defaultConfig, err = s.store.SystemLLMConfigs().GetOwned(ctx, owner, req.DefaultUsage)
		if err != nil {
			return nil, errors.NewStatus(code.ErrDefaultModelMissing, "default model is missing")
		}
		modelID = defaultConfig.ModelID
	}
	model, err := s.store.ProviderModels().GetOwned(ctx, owner, modelID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrProviderModelNotFound, "provider model not found")
	}
	provider, err := s.store.Providers().GetOwned(ctx, owner, model.ProviderID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrModelProviderNotFound, "model provider not found")
	}
	if defaultConfig != nil && (defaultConfig.ProviderID != provider.ID || defaultConfig.ModelID != model.ID) {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "default model configuration is inconsistent")
	}
	s.projectModel(provider, model)
	if !provider.Enabled || !model.Enabled || model.HealthStatus != iapiserver.ProviderModelHealthHealthy ||
		model.CapabilityResolutionStatus != modelgateway.CapabilityResolutionResolved || !model.Executable {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "provider model is not execution eligible")
	}
	required := append([]string{capabilityID}, req.RequiredCapabilityDefinitionIDs...)
	for _, id := range required {
		id = strings.TrimSpace(id)
		if id == "" || !slices.Contains(model.Capabilities, id) {
			return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "provider model does not satisfy the requested capabilities")
		}
	}
	connection, err := s.connectionRequest(provider)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	configVersion := model.ResourceVersion
	if configVersion <= 0 {
		return nil, errors.NewStatus(code.ErrDefaultModelInvalid, "provider model configuration version is invalid")
	}
	capabilities := append([]string(nil), model.Capabilities...)
	return &modelgateway.UserModelExecutionContext{
		OwnerUserID: owner, ProviderID: provider.ID, ModelID: model.ID, RemoteModel: model.Model,
		ProviderType: provider.Type, CapabilityDefinitionID: capabilityID,
		CapabilityDefinitionIDs: capabilities, ConfigVersion: configVersion,
		CredentialHandle: connection.CredentialHandle, IssuedAt: now,
		ExpiresAt: now.Add(s.credentials.ttl), Issuer: modelgateway.UserModelExecutionContextIssuer,
		ModelSnapshot: map[string]any{
			"model_id": model.ID, "display_name": model.DisplayName, "remote_model": model.Model,
			"provider_id": provider.ID, "provider_name": provider.Name, "provider_type": provider.Type,
			"capability_definition_ids": capabilities, "config_version": configVersion,
		},
		Endpoint: provider.BaseURL, AuthenticationType: provider.AuthType,
		ProviderConfiguration: maps.Clone(provider.Config),
	}, nil
}

func (s *Service) validateProviderType(ctx context.Context, providerType, authType string) error {
	items, err := s.gateway.ListProviderTypes(ctx)
	if err != nil {
		return mapProviderGatewayError(err)
	}
	for _, item := range items {
		if item.ID == strings.TrimSpace(providerType) && slices.Contains(item.AuthenticationTypes, strings.TrimSpace(authType)) {
			return nil
		}
	}
	return errors.NewStatus(code.ErrValidation, "provider type or authentication type is unsupported")
}

func (s *Service) testProviderConnection(ctx context.Context, provider *iapiserver.Provider) error {
	request, err := s.connectionRequest(provider)
	if err != nil {
		return err
	}
	if _, err := s.gateway.TestProviderConnection(ctx, request); err != nil {
		return mapProviderGatewayError(err)
	}
	return nil
}

func (s *Service) connectionRequest(provider *iapiserver.Provider) (modelgateway.ProviderConnectionRequest, error) {
	handle, err := s.credentials.Issue(provider.Type, provider.AuthType, provider.CredentialRef)
	if err != nil {
		return modelgateway.ProviderConnectionRequest{}, err
	}
	return modelgateway.ProviderConnectionRequest{
		ProviderType: provider.Type, Endpoint: provider.BaseURL, AuthType: provider.AuthType,
		CredentialHandle: handle, Config: provider.Config,
	}, nil
}

func (s *Service) ensureModelUnique(ctx context.Context, owner, providerID, displayName, remoteModel, excludeID string) error {
	displayName = strings.TrimSpace(displayName)
	remoteModel = strings.TrimSpace(remoteModel)
	if displayName == "" || remoteModel == "" {
		return errors.NewStatus(code.ErrProviderModelIdentifierInvalid, "provider model and display name are required")
	}
	models, _, err := s.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{OwnerUserID: owner, ProviderID: providerID})
	if err != nil {
		return errors.WithStack(err)
	}
	for _, model := range models {
		if model.ID == excludeID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(model.DisplayName), displayName) || strings.EqualFold(strings.TrimSpace(model.Model), remoteModel) {
			return errors.NewStatus(code.ErrProviderModelDuplicated, "provider model or display name already exists")
		}
	}
	return nil
}

func (s *Service) projectModels(provider *iapiserver.Provider, models []*iapiserver.ProviderModel) []*iapiserver.ProviderModel {
	for _, model := range models {
		s.projectModel(provider, model)
	}
	return models
}

func (s *Service) projectModel(provider *iapiserver.Provider, model *iapiserver.ProviderModel) {
	model.ProviderName = provider.Name
	model.ConfigVersion = model.ResourceVersion
	model.Capabilities = nil
	model.StreamSupported = false
	model.Executable = false
	model.UnavailableReason = ""
	model.CapabilityResolutionStatus = modelgateway.CapabilityResolutionUnknown
	if !provider.Enabled || !model.Enabled {
		model.CapabilityResolutionStatus = modelgateway.CapabilityResolutionUnavailable
		model.UnavailableReason = "provider or model is disabled"
		return
	}
	if model.HealthStatus == iapiserver.ProviderModelHealthUnhealthy {
		model.CapabilityResolutionStatus = modelgateway.CapabilityResolutionUnavailable
		model.UnavailableReason = model.HealthReason
		return
	}
	if model.HealthStatus != iapiserver.ProviderModelHealthHealthy || model.HealthCheckedAt == nil {
		return
	}
	if provider.UpdatedAt.After(model.HealthCheckedAt.Time) {
		model.CapabilityResolutionStatus = modelgateway.CapabilityResolutionStale
		model.UnavailableReason = "provider configuration changed after the last model check"
		return
	}
	resolution, err := s.gateway.ResolveUserModelCapabilities(provider.Type, verifiedProbe(model), model.DisabledCapabilityDefinitionIDs)
	if err != nil {
		model.CapabilityResolutionStatus = modelgateway.CapabilityResolutionUnavailable
		model.UnavailableReason = "provider model capabilities are unavailable"
		return
	}
	applyResolution(model, resolution)
}

func verifiedProbe(model *iapiserver.ProviderModel) *modelgateway.ModelProbeResult {
	return &modelgateway.ModelProbeResult{RemoteModel: model.Model, Available: true, StreamSupported: true}
}

func applyResolution(model *iapiserver.ProviderModel, resolution *modelgateway.CapabilityResolution) {
	if resolution == nil {
		return
	}
	model.Capabilities = append([]string(nil), resolution.CapabilityDefinitionIDs...)
	model.StreamSupported = resolution.StreamSupported
	model.Executable = resolution.Executable
	model.UnavailableReason = resolution.UnavailableReason
	model.CapabilityResolutionStatus = resolution.Status
}

func filterModelsByCapability(items []*iapiserver.ProviderModel, capability string) []*iapiserver.ProviderModel {
	filtered := make([]*iapiserver.ProviderModel, 0, len(items))
	for _, item := range items {
		if slices.Contains(item.Capabilities, capability) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *Service) persistHealthCheck(ctx context.Context, check *iapiserver.ModelHealthCheck) {
	_, _ = s.store.ModelHealthChecks().Add(context.WithoutCancel(ctx), check)
}

func providerTestResponse(providerID string) *iapiserver.ProviderTestResponse {
	return &iapiserver.ProviderTestResponse{
		TargetType: "provider", ProviderID: providerID, Success: true,
		HealthStatus: iapiserver.ProviderModelHealthHealthy, Message: "provider connection ok",
		CheckedAt: imachinery.NewTime(time.Now().UTC()),
	}
}

func capabilityForUsage(usage string) string {
	switch usage {
	case "assistant.default", "quick":
		return CapabilityTextChatCompletion
	case "translation":
		return CapabilityTextTranslate
	default:
		return ""
	}
}

func currentUserID(ctx context.Context) string {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err == nil && user != nil && strings.TrimSpace(user.ID) != "" {
		return user.ID
	}
	return "system-admin"
}

func valueOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func gatewayErrorCategory(err error) string {
	switch errors.ToStatus(err).Code {
	case code.ErrAIAppEngineAuthConfigInvalid:
		return "authentication"
	case code.ErrAIAppProviderRuntimeCapabilityMismatch:
		return "capability_mismatch"
	case code.ErrAIAppProviderResponseInvalid:
		return "invalid_response"
	default:
		return "unavailable"
	}
}

func mapProviderGatewayError(err error) error {
	return errors.NewStatus(code.ErrModelProviderTestFailed, "provider connection failed: "+gatewayErrorCategory(err))
}

func mapModelGatewayError(err error) error {
	return errors.NewStatus(code.ErrModelHealthCheckFailed, "provider model check failed: "+gatewayErrorCategory(err))
}
