package agentexecutor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
	"github.com/wangweihong/omnimam/backend/internal/taskworker/contracts"
)

type runtimeEnsureArguments struct {
	MCPBindingRefs          []mcpBindingRef                     `json:"mcp_binding_refs"`
	AgentID                 string                              `json:"agent_id"`
	AgentRuntimeID          string                              `json:"agent_runtime_id"`
	ExistingInfraRuntimeID  *string                             `json:"existing_infra_runtime_id"`
	Operation               string                              `json:"operation"`
	AgentKind               string                              `json:"agent_kind"`
	WorkspaceType           string                              `json:"workspace_type"`
	WorkspaceID             string                              `json:"workspace_id"`
	WorkspaceSourceRef      *string                             `json:"workspace_source_ref"`
	RuntimeProfileID        string                              `json:"runtime_profile_id"`
	RuntimeProfileRevision  string                              `json:"runtime_profile_revision"`
	ModelAccessGrantRef     string                              `json:"model_access_grant_ref"`
	RuntimeConfigurationRef string                              `json:"runtime_configuration_ref"`
	RuntimeGitAccessRef     string                              `json:"runtime_git_access_ref"`
	AuthorizationRef        string                              `json:"authorization_ref"`
	ExpectedResourceVersion int64                               `json:"expected_resource_version"`
	ResourceRequirement     iapiserver.InfraResourceRequirement `json:"resource_requirement"`
	LifecyclePolicy         runtimeLifecyclePolicy              `json:"lifecycle_policy"`
}

type mcpBindingRef struct {
	BindingID       string `json:"binding_id"`
	BindingRevision string `json:"binding_revision"`
}

type runtimeLifecyclePolicy struct {
	RestartPolicy          string `json:"restart_policy"`
	IdleTimeoutSeconds     *int   `json:"idle_timeout_seconds"`
	MaximumLifetimeSeconds *int   `json:"maximum_lifetime_seconds"`
}

type runtimeStopArguments struct {
	AgentID                 string `json:"agent_id"`
	AgentRuntimeID          string `json:"agent_runtime_id"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	Action                  string `json:"action"`
	AgentKind               string `json:"agent_kind"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

// ExecuteRuntimeEnsure starts an existing Agent runtime or creates a new one.
func ExecuteRuntimeEnsure(
	ctx context.Context,
	client contracts.InfrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, err := resolveRuntimeContract(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAgentRuntimeEnsure)
	if err != nil {
		return nil, err
	}
	rawArguments, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "encode agent runtime ensure arguments")
	}
	var arguments runtimeEnsureArguments
	if err := json.Unmarshal(rawArguments, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode agent runtime ensure arguments")
	}
	if err := validateRuntimeEnsureArguments(arguments); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}

	command := &infrastructure.CommandRequest{}
	if arguments.ExistingInfraRuntimeID != nil {
		command.Operation = iapiserver.TaskWorkerInfrastructureOperationStart
		command.RuntimeID = *arguments.ExistingInfraRuntimeID
	} else {
		sourceRef := ""
		if arguments.WorkspaceSourceRef != nil {
			sourceRef = *arguments.WorkspaceSourceRef
		}
		timeoutPolicy := iapiserver.InfraTimeoutPolicy{}
		if arguments.LifecyclePolicy.IdleTimeoutSeconds != nil {
			timeoutPolicy.IdleTimeoutSeconds = *arguments.LifecyclePolicy.IdleTimeoutSeconds
		}
		if arguments.LifecyclePolicy.MaximumLifetimeSeconds != nil {
			timeoutPolicy.MaximumLifetimeSeconds = *arguments.LifecyclePolicy.MaximumLifetimeSeconds
		}
		command.Operation = iapiserver.TaskWorkerInfrastructureOperationCreate
		command.Create = &iapiserver.InfraCreateRuntimeRequest{
			RequestID:              fmt.Sprintf("%s:%d", atomicTask.ID, workerTask.RetryCount+1),
			RequestingService:      iapiserver.TaskWorkerRequestingServiceTaskCenter,
			OwnerDomain:            iapiserver.TaskWorkerOwnerDomainAgent,
			OwnerReference:         arguments.AgentRuntimeID,
			RequestUserID:          atomicTask.CreatedBy,
			RuntimeMode:            iapiserver.TaskWorkerRuntimeModeService,
			RuntimeProfileID:       arguments.RuntimeProfileID,
			RuntimeProfileRevision: arguments.RuntimeProfileRevision,
			SourceRef:              sourceRef,
			ResourceRequirement:    arguments.ResourceRequirement,
			TimeoutPolicy:          timeoutPolicy,
			AuthorizationRef:       arguments.AuthorizationRef,
			EndpointVisibility:     iapiserver.TaskWorkerEndpointVisibilityInternal,
			FunctionRef:            contract.FunctionRef,
			FunctionArguments:      rawArguments,
		}
		for _, ref := range arguments.MCPBindingRefs {
			command.Create.ConfigurationBindings = append(command.Create.ConfigurationBindings, iapiserver.InfraRuntimeConfigBindingInput{
				Name: "mcp-" + ref.BindingID, BindingType: iapiserver.InfraConfigBindingTypeMCPServerRef,
				Reference: "mcp-binding-revision://" + ref.BindingID + "/" + ref.BindingRevision,
			})
		}
		if arguments.RuntimeGitAccessRef != "" {
			command.Create.ConfigurationBindings = append(command.Create.ConfigurationBindings, iapiserver.InfraRuntimeConfigBindingInput{
				Name: "coding-runtime-git", BindingType: iapiserver.InfraConfigBindingTypeSecretRef,
				Reference: arguments.RuntimeGitAccessRef,
			})
		}
	}
	response, err := client.Execute(ctx, command)
	if err != nil {
		return nil, errors.Wrap(err, "execute agent runtime ensure infrastructure command")
	}
	runtime, err := contracts.RequireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate agent runtime ensure response")
	}
	if arguments.ExistingInfraRuntimeID != nil && runtime.ID != *arguments.ExistingInfraRuntimeID {
		return nil, fmt.Errorf("agent runtime ensure returned unexpected infrastructure runtime %q", runtime.ID)
	}
	if runtime.Status != iapiserver.TaskWorkerRuntimeStatusRunning {
		return nil, fmt.Errorf("agent runtime ensure returned runtime status %q", runtime.Status)
	}
	if !contracts.HasReadyInfrastructureEndpoint(runtime, response.Result.Endpoint) {
		return nil, fmt.Errorf("agent runtime ensure returned an invalid ready endpoint")
	}
	result := map[string]any{
		iapiserver.TaskWorkerKeyInfraRuntimeID:     runtime.ID,
		iapiserver.TaskWorkerKeyRuntimeStatus:      iapiserver.TaskWorkerRuntimeStatusRunning,
		iapiserver.TaskWorkerKeyHealthStatus:       iapiserver.TaskWorkerRuntimeHealthStatusHealthy,
		iapiserver.TaskWorkerKeyEndpointRef:        runtime.EndpointRef,
		iapiserver.TaskWorkerKeyDiagnosticsSummary: map[string]any{},
	}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate agent runtime ensure output")
	}
	return result, nil
}

// ExecuteRuntimeStop stops or deletes an Agent runtime according to the task action.
func ExecuteRuntimeStop(
	ctx context.Context,
	client contracts.InfrastructureCommandExecutor,
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (map[string]any, error) {
	contract, err := resolveRuntimeContract(registry, workerTask, atomicTask, iapiserver.TaskWorkerFunctionAgentRuntimeStop)
	if err != nil {
		return nil, err
	}
	rawArguments, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, errors.Wrap(err, "encode agent runtime stop arguments")
	}
	var arguments runtimeStopArguments
	if err := json.Unmarshal(rawArguments, &arguments); err != nil {
		return nil, errors.Wrap(err, "decode agent runtime stop arguments")
	}
	if err := validateRuntimeStopArguments(arguments); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("infrastructure client is required")
	}
	response, err := client.Execute(ctx, &infrastructure.CommandRequest{
		Operation: iapiserver.TaskWorkerInfrastructureOperationStop,
		RuntimeID: arguments.InfraRuntimeID,
		Delete:    arguments.Action == iapiserver.TaskWorkerActionDelete || arguments.AgentKind == iapiserver.TaskWorkerAgentKindCoding,
	})
	if err != nil {
		return nil, errors.Wrap(err, "execute agent runtime stop infrastructure command")
	}
	runtime, err := contracts.RequireInfrastructureRuntime(response)
	if err != nil {
		return nil, errors.Wrap(err, "validate agent runtime stop response")
	}
	if runtime.ID != arguments.InfraRuntimeID {
		return nil, fmt.Errorf("agent runtime stop returned unexpected infrastructure runtime %q", runtime.ID)
	}
	expectedStatus := iapiserver.TaskWorkerRuntimeStatusStopped
	if arguments.Action == iapiserver.TaskWorkerActionDelete {
		expectedStatus = iapiserver.TaskWorkerRuntimeStatusDeleted
	}
	if runtime.Status != expectedStatus {
		return nil, fmt.Errorf("agent runtime stop returned runtime status %q, want %q", runtime.Status, expectedStatus)
	}
	result := map[string]any{
		iapiserver.TaskWorkerKeyInfraRuntimeID:  runtime.ID,
		iapiserver.TaskWorkerKeyRuntimeStatus:   runtime.Status,
		iapiserver.TaskWorkerKeyCompletedAction: arguments.Action,
	}
	if err := registry.ValidateOutput(contract, result); err != nil {
		return nil, errors.Wrap(err, "validate agent runtime stop output")
	}
	return result, nil
}

func resolveRuntimeContract(
	registry *taskfunctionregistry.Registry,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
	expectedFunctionRef string,
) (*taskfunctionregistry.Contract, error) {
	if registry == nil {
		return nil, fmt.Errorf("task function registry is required")
	}
	if atomicTask == nil {
		return nil, fmt.Errorf("agent runtime atomic task is missing")
	}
	if workerTask.AtomicTaskID == "" || atomicTask.ID != workerTask.AtomicTaskID {
		return nil, fmt.Errorf("agent runtime atomic task identity does not match worker task")
	}
	if workerTask.RetryCount < 0 {
		return nil, fmt.Errorf("agent runtime worker retry count is invalid")
	}
	if atomicTask.FunctionRef != expectedFunctionRef {
		return nil, fmt.Errorf("agent runtime atomic task function is %q, want %q", atomicTask.FunctionRef, expectedFunctionRef)
	}
	if atomicTask.FunctionContractVersion == "" || atomicTask.FunctionContractDigest == "" {
		return nil, fmt.Errorf("agent runtime atomic task contract pin is missing")
	}
	if atomicTask.Arguments == nil {
		return nil, fmt.Errorf("agent runtime atomic task arguments are missing")
	}
	contract, err := registry.Resolve(atomicTask.FunctionRef, atomicTask.FunctionContractVersion, atomicTask.FunctionContractDigest)
	if err != nil {
		return nil, errors.Wrap(err, "resolve agent runtime atomic task contract")
	}
	return contract, nil
}

func validateRuntimeEnsureArguments(arguments runtimeEnsureArguments) error {
	if arguments.AgentID == "" || arguments.AgentRuntimeID == "" || arguments.WorkspaceID == "" ||
		arguments.RuntimeProfileRevision == "" || arguments.ExpectedResourceVersion < 0 {
		return fmt.Errorf("agent runtime ensure arguments are incomplete")
	}
	if arguments.Operation != iapiserver.TaskWorkerAgentRuntimeOperationStart && arguments.Operation != iapiserver.TaskWorkerAgentRuntimeOperationRecover {
		return fmt.Errorf("agent runtime ensure operation %q is invalid", arguments.Operation)
	}
	if arguments.AgentKind != iapiserver.TaskWorkerAgentKindPlatform && arguments.AgentKind != iapiserver.TaskWorkerAgentKindCoding {
		return fmt.Errorf("agent runtime ensure agent kind %q is invalid", arguments.AgentKind)
	}
	if arguments.WorkspaceType != iapiserver.TaskWorkerWorkspaceTypeAgent && arguments.WorkspaceType != iapiserver.TaskWorkerWorkspaceTypeStudio {
		return fmt.Errorf("agent runtime ensure workspace type %q is invalid", arguments.WorkspaceType)
	}
	if arguments.RuntimeProfileID != iapiserver.TaskWorkerAgentRuntimeProfileHermes && arguments.RuntimeProfileID != iapiserver.TaskWorkerAgentRuntimeProfileCoding {
		return fmt.Errorf("agent runtime ensure profile %q is invalid", arguments.RuntimeProfileID)
	}
	if !strings.HasPrefix(arguments.ModelAccessGrantRef, iapiserver.TaskWorkerRefPrefixAgentModelAccessGrant) ||
		!strings.HasPrefix(arguments.RuntimeConfigurationRef, iapiserver.TaskWorkerRefPrefixAgentRuntimeConfig) ||
		!strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAgentRuntimeGrant) {
		return fmt.Errorf("agent runtime ensure references are invalid")
	}
	if arguments.ExistingInfraRuntimeID != nil && strings.TrimSpace(*arguments.ExistingInfraRuntimeID) == "" {
		return fmt.Errorf("agent runtime ensure existing infrastructure runtime id is invalid")
	}
	if arguments.WorkspaceSourceRef != nil && !strings.HasPrefix(*arguments.WorkspaceSourceRef, iapiserver.TaskWorkerRefPrefixAgentWorkspace) {
		return fmt.Errorf("agent runtime ensure workspace source reference is invalid")
	}
	if arguments.AgentKind == iapiserver.TaskWorkerAgentKindCoding {
		if arguments.WorkspaceType != iapiserver.TaskWorkerWorkspaceTypeStudio || arguments.RuntimeProfileID != iapiserver.TaskWorkerAgentRuntimeProfileCoding || arguments.WorkspaceSourceRef != nil || arguments.ExistingInfraRuntimeID != nil || !strings.HasPrefix(arguments.RuntimeGitAccessRef, iapiserver.AppStudioRefPrefixRuntimeGitAccess) {
			return fmt.Errorf("coding runtime git access arguments are invalid")
		}
	} else if arguments.RuntimeGitAccessRef != "" {
		return fmt.Errorf("only coding runtimes may carry git access")
	}
	if len(arguments.MCPBindingRefs) > 50 {
		return fmt.Errorf("agent runtime ensure has too many MCP bindings")
	}
	for _, ref := range arguments.MCPBindingRefs {
		if ref.BindingID == "" || ref.BindingRevision == "" {
			return fmt.Errorf("agent runtime ensure MCP binding reference is invalid")
		}
	}
	resource := arguments.ResourceRequirement
	if resource.CPUCores < 0 || resource.MemoryMB < 0 || resource.DiskMB < 0 || resource.GPUCount < 0 || resource.GPUMemoryMB < 0 {
		return fmt.Errorf("agent runtime ensure resource requirement is invalid")
	}
	lifecycle := arguments.LifecyclePolicy
	if lifecycle.RestartPolicy != "" && lifecycle.RestartPolicy != iapiserver.TaskWorkerAgentRuntimeRestartPolicyNever && lifecycle.RestartPolicy != iapiserver.TaskWorkerAgentRuntimeRestartPolicyOnFailure && lifecycle.RestartPolicy != iapiserver.TaskWorkerAgentRuntimeRestartPolicyAlways {
		return fmt.Errorf("agent runtime ensure restart policy %q is invalid", lifecycle.RestartPolicy)
	}
	if lifecycle.IdleTimeoutSeconds != nil && *lifecycle.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("agent runtime ensure idle timeout is invalid")
	}
	if lifecycle.MaximumLifetimeSeconds != nil && *lifecycle.MaximumLifetimeSeconds < 1 {
		return fmt.Errorf("agent runtime ensure maximum lifetime is invalid")
	}
	return nil
}

func validateRuntimeStopArguments(arguments runtimeStopArguments) error {
	if arguments.AgentID == "" || arguments.AgentRuntimeID == "" || arguments.InfraRuntimeID == "" || arguments.ExpectedResourceVersion < 0 {
		return fmt.Errorf("agent runtime stop arguments are incomplete")
	}
	if arguments.Action != iapiserver.TaskWorkerActionSuspend && arguments.Action != iapiserver.TaskWorkerActionStop && arguments.Action != iapiserver.TaskWorkerActionDelete {
		return fmt.Errorf("agent runtime stop action %q is invalid", arguments.Action)
	}
	if arguments.AgentKind != iapiserver.TaskWorkerAgentKindPlatform && arguments.AgentKind != iapiserver.TaskWorkerAgentKindCoding {
		return fmt.Errorf("agent runtime stop agent kind %q is invalid", arguments.AgentKind)
	}
	if !strings.HasPrefix(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAgentRuntimeGrant) {
		return fmt.Errorf("agent runtime stop authorization reference is invalid")
	}
	return nil
}
