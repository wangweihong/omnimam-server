package appstudioexecutor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/contracts"
)

// BuildArtifactLifecycle 定义 AppStudio Worker 持久化和读取构建产物所需的生命周期接口。
type BuildArtifactLifecycle interface {
	Prepare(context.Context, *iapiserver.Artifact) (*iapiserver.Artifact, bool, error)
	StoreContent(context.Context, *iapiserver.Artifact, string, io.Reader) (*iapiserver.Artifact, error)
}

type previewEnsureArguments struct {
	StudioApplicationID        string                              `json:"studio_application_id"`
	PreviewRuntimeID           string                              `json:"preview_runtime_id"`
	ExistingInfraRuntimeID     *string                             `json:"existing_infra_runtime_id"`
	WorkspaceID                string                              `json:"workspace_id"`
	WorkspaceRevision          int64                               `json:"workspace_revision"`
	WorkspaceRevisionSourceRef string                              `json:"workspace_revision_source_ref"`
	RuntimeProfileID           string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision     string                              `json:"runtime_profile_revision"`
	EndpointVisibility         string                              `json:"endpoint_visibility"`
	AuthorizationRef           string                              `json:"authorization_ref"`
	ExpectedResourceVersion    int64                               `json:"expected_resource_version"`
	ResourceRequirement        iapiserver.InfraResourceRequirement `json:"resource_requirement"`
}

type previewStopArguments struct {
	StudioApplicationID     string `json:"studio_application_id"`
	PreviewRuntimeID        string `json:"preview_runtime_id"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	Action                  string `json:"action"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

type buildArguments struct {
	StudioApplicationID     string                              `json:"studio_application_id"`
	StudioBuildID           string                              `json:"studio_build_id"`
	SourceSnapshotID        string                              `json:"source_snapshot_id"`
	SourceSnapshotDigest    string                              `json:"source_snapshot_digest"`
	SourceSnapshotSourceRef string                              `json:"source_snapshot_source_ref"`
	RuntimeProfileID        string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision  string                              `json:"runtime_profile_revision"`
	BuildConfigRef          string                              `json:"build_config_ref"`
	DependencyLockDigest    string                              `json:"dependency_lock_digest"`
	AuthorizationRef        string                              `json:"authorization_ref"`
	ExpectedResourceVersion int64                               `json:"expected_resource_version"`
	ResourceRequirement     iapiserver.InfraResourceRequirement `json:"resource_requirement"`
}

type productionReconcileArguments struct {
	StudioApplicationID        string                              `json:"studio_application_id"`
	StudioReleaseID            string                              `json:"studio_release_id"`
	StudioRuntimeInstanceID    string                              `json:"studio_runtime_instance_id"`
	ExistingInfraRuntimeID     *string                             `json:"existing_infra_runtime_id"`
	StudioApplicationVersionID string                              `json:"studio_application_version_id"`
	RuntimeConfigID            string                              `json:"runtime_config_id"`
	ArtifactID                 string                              `json:"artifact_id"`
	ArtifactDigest             string                              `json:"artifact_digest"`
	ArtifactSourceRef          string                              `json:"artifact_source_ref"`
	Environment                string                              `json:"environment"`
	DeploymentReason           string                              `json:"deployment_reason"`
	RuntimeProfileID           string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision     string                              `json:"runtime_profile_revision"`
	HealthCheckRef             string                              `json:"health_check_ref"`
	EndpointVisibility         string                              `json:"endpoint_visibility"`
	AuthorizationRef           string                              `json:"authorization_ref"`
	ExpectedResourceVersion    int64                               `json:"expected_resource_version"`
	ResourceRequirement        iapiserver.InfraResourceRequirement `json:"resource_requirement"`
}

type productionStopArguments struct {
	StudioApplicationID     string `json:"studio_application_id"`
	StudioReleaseID         string `json:"studio_release_id"`
	StudioRuntimeInstanceID string `json:"studio_runtime_instance_id"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

// ExecutePreviewEnsure starts an existing AppStudio preview runtime or creates one.
func ExecutePreviewEnsure(
	ctx context.Context,
	client contracts.InfrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, raw, err := resolveArguments(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAppStudioPreviewEnsure)
	if err != nil {
		return nil, err
	}
	arguments, err := decodeArguments[previewEnsureArguments](atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "decode appstudio preview ensure arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.PreviewRuntimeID == "" || arguments.WorkspaceID == "" || arguments.WorkspaceRevision < 0 || arguments.RuntimeProfileRevision == "" || arguments.ExpectedResourceVersion < 0 {
		return nil, fmt.Errorf("appstudio preview ensure arguments are incomplete")
	}
	if !strings.HasPrefix(arguments.WorkspaceRevisionSourceRef, iapiserver.TaskWorkerRefPrefixStudioWorkspaceRevision) || !strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAppStudioPreviewGrant) {
		return nil, fmt.Errorf("appstudio preview ensure references are invalid")
	}
	if arguments.RuntimeProfileID != iapiserver.TaskWorkerAppStudioPreviewProfileStaticWeb && arguments.RuntimeProfileID != iapiserver.TaskWorkerAppStudioPreviewProfileWebBackend {
		return nil, fmt.Errorf("appstudio preview ensure profile is invalid")
	}
	if arguments.EndpointVisibility != iapiserver.TaskWorkerEndpointVisibilityUserAccessible || invalidResourceRequirement(arguments.ResourceRequirement) {
		return nil, fmt.Errorf("appstudio preview ensure runtime configuration is invalid")
	}
	command := ensureCommand(atomicTask, workerTask, contract, raw, arguments.ExistingInfraRuntimeID, arguments.PreviewRuntimeID, arguments.RuntimeProfileID, arguments.RuntimeProfileRevision, arguments.WorkspaceRevisionSourceRef, arguments.AuthorizationRef, arguments.EndpointVisibility, arguments.ResourceRequirement)
	return executeReady(ctx, client, registry, contract, command, arguments.ExistingInfraRuntimeID, "preview ensure")
}

// ExecutePreviewStop stops or deletes an AppStudio preview runtime.
func ExecutePreviewStop(
	ctx context.Context,
	client contracts.InfrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, _, err := resolveArguments(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAppStudioPreviewStop)
	if err != nil {
		return nil, err
	}
	arguments, err := decodeArguments[previewStopArguments](atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "decode appstudio preview stop arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.PreviewRuntimeID == "" || arguments.InfraRuntimeID == "" || arguments.ExpectedResourceVersion < 0 || !strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAppStudioPreviewGrant) {
		return nil, fmt.Errorf("appstudio preview stop arguments are invalid")
	}
	if arguments.Action != iapiserver.TaskWorkerActionStop && arguments.Action != iapiserver.TaskWorkerActionDelete {
		return nil, fmt.Errorf("appstudio preview stop action is invalid")
	}
	return executeStop(ctx, client, registry, contract, arguments.InfraRuntimeID, arguments.Action, arguments.Action == iapiserver.TaskWorkerActionDelete, "preview stop")
}

// ExecuteBuild executes an AppStudio build and attaches its collected artifact.
func ExecuteBuild(
	ctx context.Context,
	client contracts.InfrastructureBuildExecutor,
	lifecycle BuildArtifactLifecycle,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, raw, err := resolveArguments(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAppStudioBuildExecute)
	if err != nil {
		return nil, err
	}
	arguments, err := decodeArguments[buildArguments](atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "decode appstudio build arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.StudioBuildID == "" || arguments.SourceSnapshotID == "" || arguments.SourceSnapshotDigest == "" || arguments.RuntimeProfileRevision == "" || arguments.DependencyLockDigest == "" || arguments.ExpectedResourceVersion < 0 {
		return nil, fmt.Errorf("appstudio build arguments are incomplete")
	}
	if !strings.HasPrefix(arguments.SourceSnapshotSourceRef, iapiserver.TaskWorkerRefPrefixStudioSnapshot) || !strings.HasPrefix(arguments.BuildConfigRef, iapiserver.TaskWorkerRefPrefixAppStudioBuildConfig) || !strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAppStudioBuildGrant) {
		return nil, fmt.Errorf("appstudio build references are invalid")
	}
	if arguments.RuntimeProfileID != iapiserver.TaskWorkerAppStudioBuildProfileStaticWeb && arguments.RuntimeProfileID != iapiserver.TaskWorkerAppStudioBuildProfileWebBackend {
		return nil, fmt.Errorf("appstudio build profile is invalid")
	}
	if invalidResourceRequirement(arguments.ResourceRequirement) {
		return nil, fmt.Errorf("appstudio build resource requirement is invalid")
	}
	if client == nil || lifecycle == nil {
		return nil, fmt.Errorf("appstudio build delivery dependencies are required")
	}
	registration := contract.ArtifactRegistration
	if registration == nil || !registration.Enabled || registration.ProducerType != iapiserver.TaskWorkerArtifactProducerTypeStudioBuild || registration.OutputKey == "" || registration.DeliveryPolicy == nil || !registration.DeliveryPolicy.AttachAfterComplete {
		return nil, fmt.Errorf("appstudio build artifact delivery contract is invalid")
	}
	declarations := make([]iapiserver.InfraRuntimeOutputDeclaration, 0, len(contract.InfraAdapter.OutputDeclarations))
	for _, declaration := range contract.InfraAdapter.OutputDeclarations {
		declarations = append(declarations, iapiserver.InfraRuntimeOutputDeclaration{
			OutputKey:    declaration.OutputKey,
			RelativePath: declaration.RelativePath,
			MediaType:    declaration.MediaType,
		})
	}
	command := &infrastructure.CommandRequest{
		Operation: iapiserver.TaskWorkerInfrastructureOperationCreate,
		Create: &iapiserver.InfraCreateRuntimeRequest{
			RequestID:              fmt.Sprintf("%s:%d", atomicTask.ID, workerTask.RetryCount+1),
			RequestingService:      contract.InfraAdapter.RequestingService,
			OwnerDomain:            contract.InfraAdapter.OwnerDomain,
			OwnerReference:         arguments.StudioBuildID,
			RequestUserID:          atomicTask.CreatedBy,
			RuntimeMode:            contract.InfraAdapter.RuntimeMode,
			RuntimeProfileID:       arguments.RuntimeProfileID,
			RuntimeProfileRevision: arguments.RuntimeProfileRevision,
			SourceRef:              arguments.SourceSnapshotSourceRef,
			OutputDeclarations:     declarations,
			ResourceRequirement:    arguments.ResourceRequirement,
			AuthorizationRef:       arguments.AuthorizationRef,
			FunctionRef:            contract.FunctionRef,
			FunctionArguments:      raw,
		},
	}
	response, err := client.Execute(ctx, command)
	if err != nil {
		return nil, errors.Wrap(err, "execute appstudio build infrastructure command")
	}
	runtime, err := contracts.RequireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate appstudio build infrastructure response")
	}
	if runtime.Status != iapiserver.TaskWorkerRuntimeStatusSucceeded {
		return nil, fmt.Errorf("appstudio build infrastructure runtime did not succeed")
	}
	output, err := requireCollectedBuildOutput(response, registration.OutputKey)
	if err != nil {
		return nil, err
	}
	artifact, _, err := lifecycle.Prepare(ctx, &iapiserver.Artifact{
		OwnerUserID:              atomicTask.CreatedBy,
		ProducerType:             registration.ProducerType,
		ProducerID:               arguments.StudioBuildID,
		ProducerIdempotencyKey:   iapiserver.TaskWorkerBuildIdempotencyPrefix + arguments.StudioBuildID + ":" + registration.OutputKey,
		AtomicTaskID:             atomicTask.ID,
		TaskAttemptID:            workerTask.RuntimeTaskID,
		OutputKey:                registration.OutputKey,
		ArtifactType:             iapiserver.TaskWorkerArtifactTypeBuildBundle,
		MediaType:                iapiserver.AssetMediaTypeOther,
		SavePolicy:               iapiserver.ArtifactSaveAutomatic,
		ProcessingProfileVersion: iapiserver.TaskWorkerArtifactProcessingProfileBuildBundle,
		Metadata:                 map[string]any{iapiserver.TaskWorkerKeyContentType: output.MediaType},
	})
	if err != nil {
		return nil, errors.Wrap(err, "prepare appstudio build artifact")
	}
	if artifact == nil || artifact.ID == "" {
		return nil, fmt.Errorf("appstudio build artifact preparation returned no artifact")
	}
	if artifact.ProcessingStatus != iapiserver.ArtifactProcessingReady {
		artifact, err = deliverBuildOutputContent(ctx, client, lifecycle, output, artifact)
		if err != nil {
			return nil, err
		}
	}
	if !artifactMatchesBuildOutput(artifact, output) {
		return nil, errors.NewStatus(code.ErrInfraOutputIntegrityMismatch, "completed artifact does not match infra runtime output")
	}
	attached, err := client.AttachOutputArtifact(ctx, output.ID, &iapiserver.InfraAttachArtifactRequest{
		ArtifactID:    artifact.ID,
		SizeBytes:     output.SizeBytes,
		ContentDigest: output.ContentDigest,
	})
	if err != nil {
		return nil, errors.Wrap(err, "attach appstudio build artifact to infra output")
	}
	if attached == nil || attached.ArtifactID != artifact.ID {
		return nil, fmt.Errorf("appstudio build output artifact attachment was not confirmed")
	}
	if workerTask.RuntimeTaskID == "" {
		return nil, fmt.Errorf("appstudio build task attempt log identity is unavailable")
	}
	result := map[string]any{
		iapiserver.TaskWorkerKeyArtifactID:       artifact.ID,
		iapiserver.TaskWorkerKeyArtifactDigest:   output.ContentDigest,
		iapiserver.TaskWorkerKeyProcessingStatus: artifact.ProcessingStatus,
		iapiserver.TaskWorkerKeyValidationStatus: iapiserver.TaskWorkerBuildValidationStatusPassed,
		iapiserver.TaskWorkerKeyLogsRef:          iapiserver.TaskWorkerTaskAttemptLogRefPrefix + workerTask.RuntimeTaskID,
	}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate appstudio build output")
	}
	return result, nil
}

// ExecuteProductionReconcile starts an AppStudio production runtime or creates one from an artifact.
func ExecuteProductionReconcile(
	ctx context.Context,
	client contracts.InfrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, raw, err := resolveArguments(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAppStudioProductionReconcile)
	if err != nil {
		return nil, err
	}
	arguments, err := decodeArguments[productionReconcileArguments](atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "decode appstudio production reconcile arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.StudioReleaseID == "" || arguments.StudioRuntimeInstanceID == "" || arguments.StudioApplicationVersionID == "" || arguments.RuntimeConfigID == "" || arguments.ArtifactID == "" || arguments.ArtifactDigest == "" || arguments.RuntimeProfileRevision == "" || arguments.ExpectedResourceVersion < 0 {
		return nil, fmt.Errorf("appstudio production reconcile arguments are incomplete")
	}
	if !strings.HasPrefix(arguments.ArtifactSourceRef, iapiserver.TaskWorkerRefPrefixArtifact) || !strings.HasPrefix(arguments.HealthCheckRef, iapiserver.TaskWorkerRefPrefixAppStudioHealthCheck) || !strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAppStudioReleaseGrant) {
		return nil, fmt.Errorf("appstudio production reconcile references are invalid")
	}
	if arguments.Environment != iapiserver.TaskWorkerEnvironmentPreview && arguments.Environment != iapiserver.TaskWorkerEnvironmentProduction {
		return nil, fmt.Errorf("appstudio production environment is invalid")
	}
	if arguments.DeploymentReason != iapiserver.TaskWorkerDeploymentReasonDeploy && arguments.DeploymentReason != iapiserver.TaskWorkerDeploymentReasonUpgrade && arguments.DeploymentReason != iapiserver.TaskWorkerDeploymentReasonRollback {
		return nil, fmt.Errorf("appstudio production deployment reason is invalid")
	}
	if arguments.RuntimeProfileID != iapiserver.TaskWorkerAppStudioProductionProfileStaticWeb && arguments.RuntimeProfileID != iapiserver.TaskWorkerAppStudioProductionProfileWebBackend {
		return nil, fmt.Errorf("appstudio production profile is invalid")
	}
	if arguments.EndpointVisibility != iapiserver.TaskWorkerEndpointVisibilityInternal && arguments.EndpointVisibility != iapiserver.TaskWorkerEndpointVisibilityUserAccessible {
		return nil, fmt.Errorf("appstudio production endpoint visibility is invalid")
	}
	if invalidResourceRequirement(arguments.ResourceRequirement) {
		return nil, fmt.Errorf("appstudio production resource requirement is invalid")
	}
	command := ensureCommand(atomicTask, workerTask, contract, raw, arguments.ExistingInfraRuntimeID, arguments.StudioRuntimeInstanceID, arguments.RuntimeProfileID, arguments.RuntimeProfileRevision, arguments.ArtifactSourceRef, arguments.AuthorizationRef, arguments.EndpointVisibility, arguments.ResourceRequirement)
	return executeReady(ctx, client, registry, contract, command, arguments.ExistingInfraRuntimeID, "production reconcile")
}

// ExecuteProductionStop stops an AppStudio production runtime without deleting its record.
func ExecuteProductionStop(
	ctx context.Context,
	client contracts.InfrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, _, err := resolveArguments(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAppStudioProductionStop)
	if err != nil {
		return nil, err
	}
	arguments, err := decodeArguments[productionStopArguments](atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "decode appstudio production stop arguments")
	}
	if arguments.StudioApplicationID == "" || arguments.StudioReleaseID == "" || arguments.StudioRuntimeInstanceID == "" || arguments.InfraRuntimeID == "" || arguments.ExpectedResourceVersion < 0 || !strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAppStudioReleaseGrant) {
		return nil, fmt.Errorf("appstudio production stop arguments are invalid")
	}
	return executeStop(ctx, client, registry, contract, arguments.InfraRuntimeID, iapiserver.TaskWorkerActionStop, false, "production stop")
}

func resolveArguments(registry *taskfunctionregistry.Registry, workerTask workflowruntime.WorkerTask, atomicTask *iapiserver.AtomicTask, expectedFunctionRef string) (*taskfunctionregistry.Contract, []byte, error) {
	if registry == nil {
		return nil, nil, fmt.Errorf("task function registry is required")
	}
	if atomicTask == nil || workerTask.AtomicTaskID == "" || atomicTask.ID != workerTask.AtomicTaskID {
		return nil, nil, fmt.Errorf("appstudio atomic task identity is invalid")
	}
	if workerTask.RetryCount < 0 || atomicTask.FunctionRef != expectedFunctionRef || atomicTask.FunctionContractVersion == "" || atomicTask.FunctionContractDigest == "" || atomicTask.Arguments == nil {
		return nil, nil, fmt.Errorf("appstudio atomic task contract pin is invalid")
	}
	contract, err := registry.Resolve(atomicTask.FunctionRef, atomicTask.FunctionContractVersion, atomicTask.FunctionContractDigest)
	if err != nil {
		return nil, nil, errors.Wrap(err, "resolve appstudio atomic task contract")
	}
	raw, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, nil, errors.Wrap(err, "encode appstudio arguments")
	}
	return contract, raw, nil
}

func decodeArguments[T any](arguments map[string]any) (T, error) {
	var result T
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &result, TagName: "json"})
	if err != nil {
		return result, err
	}
	return result, decoder.Decode(arguments)
}

func ensureCommand(atomicTask *iapiserver.AtomicTask, workerTask workflowruntime.WorkerTask, contract *taskfunctionregistry.Contract, raw []byte, existing *string, ownerReference, profileID, profileRevision, sourceRef, authorizationRef, endpointVisibility string, resource iapiserver.InfraResourceRequirement) *infrastructure.CommandRequest {
	if existing != nil {
		return &infrastructure.CommandRequest{Operation: iapiserver.TaskWorkerInfrastructureOperationStart, RuntimeID: *existing}
	}
	return &infrastructure.CommandRequest{
		Operation: iapiserver.TaskWorkerInfrastructureOperationCreate,
		Create: &iapiserver.InfraCreateRuntimeRequest{
			RequestID:              fmt.Sprintf("%s:%d", atomicTask.ID, workerTask.RetryCount+1),
			RequestingService:      iapiserver.TaskWorkerRequestingServiceTaskCenter,
			OwnerDomain:            iapiserver.TaskWorkerOwnerDomainAppStudio,
			OwnerReference:         ownerReference,
			RequestUserID:          atomicTask.CreatedBy,
			RuntimeMode:            iapiserver.TaskWorkerRuntimeModeService,
			RuntimeProfileID:       profileID,
			RuntimeProfileRevision: profileRevision,
			SourceRef:              sourceRef,
			ResourceRequirement:    resource,
			AuthorizationRef:       authorizationRef,
			EndpointVisibility:     endpointVisibility,
			FunctionRef:            contract.FunctionRef,
			FunctionArguments:      raw,
		},
	}
}

func executeReady(ctx context.Context, client contracts.InfrastructureCommandExecutor, registry *taskfunctionregistry.Registry, contract *taskfunctionregistry.Contract, command *infrastructure.CommandRequest, existing *string, operation string) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}
	response, err := client.Execute(ctx, command)
	if err != nil {
		return nil, errors.Wrap(err, "execute appstudio "+operation+" infrastructure command")
	}
	runtime, err := contracts.RequireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" response")
	}
	if existing != nil && runtime.ID != *existing {
		return nil, fmt.Errorf("appstudio %s returned unexpected infrastructure runtime %q", operation, runtime.ID)
	}
	if runtime.Status != iapiserver.TaskWorkerRuntimeStatusRunning || !contracts.HasReadyInfrastructureEndpoint(runtime, response.Result.Endpoint) {
		return nil, fmt.Errorf("appstudio %s returned an invalid ready runtime", operation)
	}
	result := map[string]any{
		iapiserver.TaskWorkerKeyInfraRuntimeID:     runtime.ID,
		iapiserver.TaskWorkerKeyRuntimeStatus:      iapiserver.TaskWorkerRuntimeStatusRunning,
		iapiserver.TaskWorkerKeyHealthStatus:       iapiserver.TaskWorkerRuntimeHealthStatusHealthy,
		iapiserver.TaskWorkerKeyEndpointRef:        runtime.EndpointRef,
		iapiserver.TaskWorkerKeyDiagnosticsSummary: map[string]any{},
	}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" output")
	}
	return result, nil
}

func executeStop(ctx context.Context, client contracts.InfrastructureCommandExecutor, registry *taskfunctionregistry.Registry, contract *taskfunctionregistry.Contract, runtimeID, action string, deleteRuntime bool, operation string) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}
	response, err := client.Execute(ctx, &infrastructure.CommandRequest{
		Operation: iapiserver.TaskWorkerInfrastructureOperationStop,
		RuntimeID: runtimeID,
		Delete:    deleteRuntime,
	})
	if err != nil {
		return nil, errors.Wrap(err, "execute appstudio "+operation+" infrastructure command")
	}
	runtime, err := contracts.RequireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" response")
	}
	wantStatus := iapiserver.TaskWorkerRuntimeStatusStopped
	if deleteRuntime {
		wantStatus = iapiserver.TaskWorkerRuntimeStatusDeleted
	}
	if runtime.ID != runtimeID || runtime.Status != wantStatus {
		return nil, fmt.Errorf("appstudio %s returned unexpected runtime result", operation)
	}
	result := map[string]any{iapiserver.TaskWorkerKeyInfraRuntimeID: runtime.ID, iapiserver.TaskWorkerKeyRuntimeStatus: runtime.Status, iapiserver.TaskWorkerKeyCompletedAction: action}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate appstudio "+operation+" output")
	}
	return result, nil
}

func requireCollectedBuildOutput(response *infrastructure.CommandResponse, outputKey string) (*iapiserver.InfraRuntimeOutput, error) {
	if response == nil || response.Result == nil {
		return nil, fmt.Errorf("appstudio build infrastructure response is empty")
	}
	for _, output := range response.Result.Outputs {
		if output == nil || output.OutputKey != outputKey {
			continue
		}
		if output.ID == "" || output.Status != iapiserver.TaskWorkerRuntimeOutputStatusCollected || output.MediaType == "" || output.SizeBytes < 0 || !validBuildDigest(output.ContentDigest) || output.ContentRef != iapiserver.TaskWorkerRefPrefixInfraOutput+output.ID || output.CollectedAt == nil {
			return nil, errors.NewStatus(code.ErrInfraOutputContentUnavailable, "appstudio build output descriptor is incomplete")
		}
		return output, nil
	}
	return nil, errors.NewStatus(code.ErrInfraOutputCollectionFailed, "appstudio build bundle output was not collected")
}

func deliverBuildOutputContent(ctx context.Context, client contracts.InfrastructureBuildExecutor, lifecycle BuildArtifactLifecycle, output *iapiserver.InfraRuntimeOutput, artifact *iapiserver.Artifact) (*iapiserver.Artifact, error) {
	content, err := client.ReadOutputContent(ctx, output.ID)
	if err != nil {
		return nil, errors.Wrap(err, "read appstudio build output content")
	}
	if content == nil || content.Body == nil {
		return nil, errors.NewStatus(code.ErrInfraOutputContentUnavailable, "appstudio build output content is unavailable")
	}
	defer content.Body.Close()
	if content.MediaType != output.MediaType || content.SizeBytes != output.SizeBytes || content.ContentDigest != output.ContentDigest {
		return nil, errors.NewStatus(code.ErrInfraOutputIntegrityMismatch, "infra output stream metadata does not match descriptor")
	}
	temporary, err := os.CreateTemp("", "omnimam-build-output-*")
	if err != nil {
		return nil, errors.Wrap(err, "create appstudio build output staging file")
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	defer temporary.Close()
	digest := sha256.New()
	size, err := io.Copy(io.MultiWriter(temporary, digest), content.Body)
	if err != nil {
		return nil, errors.Wrap(err, "stage appstudio build output content")
	}
	actualDigest := iapiserver.TaskWorkerContentDigestSHA256Prefix + hex.EncodeToString(digest.Sum(nil))
	if size != output.SizeBytes || actualDigest != output.ContentDigest {
		return nil, errors.NewStatus(code.ErrInfraOutputIntegrityMismatch, "infra output stream bytes do not match descriptor")
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "rewind appstudio build output content")
	}
	completed, err := lifecycle.StoreContent(ctx, artifact, output.MediaType, temporary)
	if err != nil {
		return nil, errors.Wrap(err, "store appstudio build artifact content")
	}
	if !artifactMatchesBuildOutput(completed, output) {
		return nil, errors.NewStatus(code.ErrInfraOutputIntegrityMismatch, "stored artifact content does not match infra output")
	}
	return completed, nil
}

func artifactMatchesBuildOutput(artifact *iapiserver.Artifact, output *iapiserver.InfraRuntimeOutput) bool {
	if artifact == nil || output == nil || artifact.ProcessingStatus != iapiserver.ArtifactProcessingReady {
		return false
	}
	digest, _ := artifact.Metadata[iapiserver.TaskWorkerKeySHA256].(string)
	size, ok := artifactMetadataSize(artifact.Metadata[iapiserver.TaskWorkerKeySizeBytes])
	mimeType, _ := artifact.Metadata[iapiserver.TaskWorkerKeyMIMEType].(string)
	return ok && iapiserver.TaskWorkerContentDigestSHA256Prefix+strings.ToLower(digest) == output.ContentDigest && size == output.SizeBytes && mimeType == output.MediaType
}

func artifactMetadataSize(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if typed < 0 || typed != float64(int64(typed)) {
			return 0, false
		}
		return int64(typed), true
	default:
		return 0, false
	}
}

func validBuildDigest(value string) bool {
	if len(value) != len(iapiserver.TaskWorkerContentDigestSHA256Prefix)+64 || !strings.HasPrefix(value, iapiserver.TaskWorkerContentDigestSHA256Prefix) {
		return false
	}
	for _, char := range value[len(iapiserver.TaskWorkerContentDigestSHA256Prefix):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func invalidResourceRequirement(resource iapiserver.InfraResourceRequirement) bool {
	return resource.CPUCores < 0 || resource.MemoryMB < 0 || resource.DiskMB < 0 || resource.GPUCount < 0 || resource.GPUMemoryMB < 0
}
