package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type Service struct {
	store    store.InfrastructureStore
	provider RuntimeProvider
	profiles map[string]*iapiserver.InfraRuntimeProfile
}

func NewService(storage store.InfrastructureStore, provider RuntimeProvider) (*Service, error) {
	if storage == nil {
		return nil, fmt.Errorf("infrastructure store is required")
	}
	if provider == nil {
		provider = UnavailableProvider{}
	}
	profiles := defaultProfiles()
	service := &Service{store: storage, provider: provider, profiles: profiles}
	return service, nil
}
func (s *Service) ReconcileCatalog(ctx context.Context) error {
	node, err := s.provider.Info(ctx)
	if err != nil {
		node = &iapiserver.InfraNode{ObjectMeta: imachinery.ObjectMeta{ID: "docker-local", Name: "Docker Local"}, ProviderType: "docker", Status: "OFFLINE"}
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
	}{{"agent.hermes", "SERVICE", []string{"cpu", "network", "persistent_workspace"}}, {"agent.coding", "SERVICE", []string{"cpu", "network", "workspace_tool"}}, {"appstudio.preview.static-web", "SERVICE", []string{"cpu", "network", "endpoint"}}, {"appstudio.preview.web-backend", "SERVICE", []string{"cpu", "network", "endpoint"}}, {"appstudio.build.static-web", "JOB", []string{"cpu", "artifact_output"}}, {"appstudio.build.web-backend", "JOB", []string{"cpu", "artifact_output"}}, {"appstudio.production.static-web", "SERVICE", []string{"cpu", "network", "endpoint"}}, {"appstudio.production.web-backend", "SERVICE", []string{"cpu", "network", "endpoint"}}}
	result := make(map[string]*iapiserver.InfraRuntimeProfile, len(definitions))
	for _, item := range definitions {
		result[item.id] = &iapiserver.InfraRuntimeProfile{ObjectMeta: imachinery.ObjectMeta{ID: item.id, Name: item.id}, Revision: "1.0", RuntimeMode: item.mode, ProviderType: "docker", Capabilities: item.caps, Status: "ACTIVE"}
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
func (s *Service) ListOutputs(ctx context.Context, id string) (*iapiserver.InfraRuntimeOutputListResponse, error) {
	if _, err := s.store.GetInfraRuntime(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.store.ListInfraRuntimeOutputs(ctx, id)
	return &iapiserver.InfraRuntimeOutputListResponse{Total: int64(len(items)), Items: items}, err
}
func (s *Service) Logs(ctx context.Context, id string) (*iapiserver.InfraRuntimeLogListResponse, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.ProviderRuntimeRef == "" {
		return &iapiserver.InfraRuntimeLogListResponse{Items: []*iapiserver.InfraRuntimeLogEntry{}}, nil
	}
	items, err := s.provider.Logs(ctx, runtime.ProviderRuntimeRef, 1000)
	return &iapiserver.InfraRuntimeLogListResponse{Total: int64(len(items)), Items: items}, err
}
func (s *Service) CreateRuntime(ctx context.Context, req *iapiserver.InfraCreateRuntimeRequest) (*iapiserver.InfraOperationResult, error) {
	profile, ok := s.profiles[req.RuntimeProfileID]
	if !ok || profile.Status != "ACTIVE" || profile.Revision != req.RuntimeProfileRevision {
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
	runtime := &iapiserver.InfraRuntime{ObjectMeta: imachinery.ObjectMeta{ID: runtimeID}, RuntimeMode: req.RuntimeMode, Status: "ACCEPTED", RequestingService: req.RequestingService, OwnerDomain: req.OwnerDomain, OwnerReference: req.OwnerReference, RequestUserID: req.RequestUserID, RequestID: req.RequestID, RequestFingerprint: fingerprint, RuntimeProfileID: req.RuntimeProfileID, RuntimeProfileRevision: req.RuntimeProfileRevision, ProviderType: "docker", SourceRef: req.SourceRef}
	timeout, _ := json.Marshal(req.TimeoutPolicy)
	runtime.TimeoutPolicy = timeout
	mounts := make([]*iapiserver.InfraRuntimeMount, 0, len(req.Mounts))
	for _, input := range req.Mounts {
		mounts = append(mounts, &iapiserver.InfraRuntimeMount{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, RuntimeID: runtimeID, SourceRef: input.SourceRef, MountKind: input.MountKind, TargetPath: input.TargetPath, ReadOnly: input.ReadOnly, AuthorizationRef: req.AuthorizationRef})
	}
	bindings := make([]*iapiserver.InfraRuntimeConfigBinding, 0, len(req.ConfigurationBindings))
	for _, input := range req.ConfigurationBindings {
		bindings = append(bindings, &iapiserver.InfraRuntimeConfigBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: input.Name}, RuntimeID: runtimeID, BindingType: input.BindingType, Reference: input.Reference, InjectionStatus: "PENDING"})
	}
	created, err := s.store.CreateInfraRuntimeAggregate(ctx, runtime, mounts, bindings)
	if err != nil {
		return nil, err
	}
	if created.ID != runtimeID {
		return s.result(ctx, created, nil)
	}
	created.Status = "PREPARING"
	_, _ = s.store.UpdateInfraRuntime(ctx, created, nil, nil, "")
	providerResult, err := s.provider.Ensure(ctx, ProviderRequest{RuntimeID: created.ID, Profile: profile, Request: req})
	if err != nil {
		created.Status = "FAILED"
		created.FailureCode = "ERR_INFRA_RUNTIME_OPERATION_FAILED"
		_, _ = s.store.UpdateInfraRuntime(ctx, created, nil, nil, "")
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	created.ProviderRuntimeRef = providerResult.ProviderRuntimeRef
	created.Status = providerResult.Status
	if created.Status == "" {
		created.Status = "RUNNING"
	}
	endpoint := endpointFromResult(created, req, providerResult)
	created.EndpointRef = ""
	if endpoint != nil {
		created.EndpointRef = "infra-endpoint://" + endpoint.ID
	}
	created, err = s.store.UpdateInfraRuntime(ctx, created, endpoint, providerResult.Outputs, "")
	if err != nil {
		return nil, err
	}
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
		runtime.Status = "RUNNING"
	}
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, "")
	if err != nil {
		return nil, err
	}
	return s.result(ctx, runtime, result)
}
func (s *Service) Stop(ctx context.Context, id string, deleteRuntime bool) (*iapiserver.InfraOperationResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.Status == "STOPPED" || runtime.Status == "DELETED" {
		return s.result(ctx, runtime, nil)
	}
	result, err := s.provider.Stop(ctx, runtime.ProviderRuntimeRef, deleteRuntime)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	if deleteRuntime {
		runtime.Status = "DELETED"
	} else {
		runtime.Status = "STOPPED"
	}
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, "")
	if err != nil {
		return nil, err
	}
	return s.result(ctx, runtime, result)
}
func (s *Service) Delete(ctx context.Context, id string) error {
	_, err := s.Stop(ctx, id, true)
	return err
}
func (s *Service) Cancel(ctx context.Context, id string) (*iapiserver.InfraOperationResult, error) {
	runtime, err := s.store.GetInfraRuntime(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.RuntimeMode != "JOB" {
		return nil, errors.NewStatus(code.ErrInfraRuntimeStateConflict, "only jobs can be canceled")
	}
	result, err := s.provider.Stop(ctx, runtime.ProviderRuntimeRef, false)
	if err != nil {
		return nil, errors.NewStatus(code.ErrInfraRuntimeOperationFailed, err.Error())
	}
	runtime.Status = "CANCELED"
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
	runtime, err = s.store.UpdateInfraRuntime(ctx, runtime, nil, result.Outputs, "infra_runtime_reconciled")
	if err != nil {
		return nil, err
	}
	return s.result(ctx, runtime, result)
}
func (s *Service) result(ctx context.Context, runtime *iapiserver.InfraRuntime, provider *ProviderResult) (*iapiserver.InfraOperationResult, error) {
	result := &iapiserver.InfraOperationResult{Runtime: runtime}
	if endpoint, err := s.store.GetInfraRuntimeEndpoint(ctx, runtime.ID); err == nil {
		result.Endpoint = endpoint
	}
	outputs, _ := s.store.ListInfraRuntimeOutputs(ctx, runtime.ID)
	result.Outputs = outputs
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
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func endpointFromResult(runtime *iapiserver.InfraRuntime, req *iapiserver.InfraCreateRuntimeRequest, result *ProviderResult) *iapiserver.InfraRuntimeEndpoint {
	if result.EndpointDisplayRef == "" {
		return nil
	}
	visibility := req.EndpointVisibility
	if visibility == "" {
		visibility = "INTERNAL"
	}
	return &iapiserver.InfraRuntimeEndpoint{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, RuntimeID: runtime.ID, Visibility: visibility, Status: "READY", DisplayRef: result.EndpointDisplayRef}
}

var _ = strings.TrimSpace
var _ = time.Second
