package applicationplatform

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// ApplicationRunExecutor executes Task Center claims and projects terminal facts back to ApplicationRun.
type ApplicationRunExecutor struct {
	store        store.Factory
	runtime      *appregistry.RuntimeRegistry
	capabilities *appregistry.ProviderCapabilityRegistry
	adapters     map[string]EngineAdapter
	executors    map[string]OperationExecutor
	assets       AssetRegistrar
	events       EventPublisher
	limitsMu     sync.Mutex
	activity     map[string]*engineActivity
}

type engineActivity struct {
	active  int
	changed chan struct{}
}

func NewApplicationRunExecutor(str store.Factory, runtime *appregistry.RuntimeRegistry, capabilities *appregistry.ProviderCapabilityRegistry, adapters map[string]EngineAdapter, executors map[string]OperationExecutor, assets AssetRegistrar, events EventPublisher) (*ApplicationRunExecutor, error) {
	if str == nil || runtime == nil {
		return nil, fmt.Errorf("application run executor store and runtime registry are required")
	}
	if assets == nil {
		assets = NoopAssetRegistrar{}
	}
	if events == nil {
		events = NoopEventPublisher{}
	}
	return &ApplicationRunExecutor{store: str, runtime: runtime, capabilities: capabilities, adapters: adapters, executors: executors, assets: assets, events: events, activity: map[string]*engineActivity{}}, nil
}

// Execute resolves the engine adapter from Runtime Registry and honors engine task timeout/concurrency.
func (e *ApplicationRunExecutor) Execute(ctx context.Context, task *iapiserver.AtomicTask) (map[string]any, error) {
	if task == nil || task.ApplicationRunID == "" {
		return nil, errors.NewStatus(code.ErrAIAppApplicationRunNotFound, "application run id is required")
	}
	run, err := e.store.ApplicationPlatforms().GetApplicationRun(ctx, task.ApplicationRunID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppApplicationRunNotFound, "application run not found")
	}
	engine, err := e.store.ApplicationPlatforms().GetEngineInstance(ctx, run.EngineInstanceID)
	if err != nil || !engine.Enabled || engine.HealthStatus == iapiserver.EngineHealthOffline {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine instance is unavailable")
	}
	engineType, ok := e.runtime.EngineType(engine.ApplicationEngineTypeID)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine type is not registered")
	}
	if e.adapters[engineType.EngineAdapterID] == nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine adapter is not registered")
	}
	capabilityID := firstString(run.ExecutionSnapshot, "capability_definition_id")
	if err := e.validateCurrentProviderBinding(ctx, run, engine, capabilityID); err != nil {
		return nil, err
	}
	if run.CapabilitySourceType == iapiserver.CapabilitySourceComfyUIWorkflow {
		if engine.HealthStatus != iapiserver.EngineHealthOnline {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "ComfyUI engine is not online")
		}
		catalog, catalogErr := e.store.ApplicationPlatforms().GetComfyUIEngineObjectInfo(ctx, engine.ID)
		if catalogErr != nil || catalog.Stale(time.Now()) {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "current object_info is missing or stale")
		}
		workflow := mapValue(run.CapabilitySourceSnapshot["comfyui_api_workflow"])
		contract := mapValue(run.CapabilitySourceSnapshot["template_contract"])
		if validateErr := validateComfyUITemplateSnapshot(workflow, catalog.ObjectInfo, contract); validateErr != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowIncompatible, "workflow is incompatible with the current object_info")
		}
	}
	executorDefinition, ok := e.runtime.OperationExecutor(engine.ApplicationEngineTypeID, capabilityID)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "operation executor mapping is unavailable")
	}
	executor := e.executors[executorDefinition.ID]
	if executor == nil {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityExecutorMissing, "operation executor is not registered")
	}
	release, err := e.acquire(ctx, engine.ID, engine.MaxConcurrency)
	if err != nil {
		return nil, err
	}
	defer release()
	timeout := time.Duration(engine.TaskTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	executeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := executor.Execute(executeCtx, engine, run)
	if executeCtx.Err() != nil {
		return nil, executeCtx.Err()
	}
	if err != nil && run.ProviderCapabilityID != nil && errors.ToStatus(err).Code == code.ErrAIAppProviderRuntimeCapabilityMismatch {
		e.publishCorrection(ctx, run, err)
	}
	return output, err
}

func (e *ApplicationRunExecutor) validateCurrentProviderBinding(ctx context.Context, run *iapiserver.ApplicationRun, engine *iapiserver.EngineInstance, capabilityDefinitionID string) error {
	if run.ProviderCapabilityID == nil {
		return nil
	}
	if e.capabilities == nil || run.ProviderCapabilityRevision == nil || run.ProviderOperationID == nil {
		return errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability snapshot is incomplete")
	}
	capability, ok := e.capabilities.Get(*run.ProviderCapabilityID)
	if !ok || capability.Kind != iapiserver.ProviderCapabilityKindCatalog || capability.Availability != iapiserver.ProviderCapabilityAvailable || capability.Revision != *run.ProviderCapabilityRevision || capability.ApplicationEngineTypeID != engine.ApplicationEngineTypeID {
		return errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability revision is no longer executable")
	}
	operation, ok := findOperation(capability, *run.ProviderOperationID)
	if !ok || operation.CapabilityDefinitionID != capabilityDefinitionID {
		return errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider operation no longer matches the application capability")
	}
	enabled := true
	bindings, _, err := e.store.ApplicationPlatforms().ListEngineBindings(ctx, &iapiserver.EngineCapabilityBindingListRequest{
		EngineInstanceID: engine.ID, ProviderCapabilityID: capability.ID, Enabled: &enabled,
	})
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.EngineInstanceID != engine.ID || binding.ProviderCapabilityRevision != capability.Revision {
			continue
		}
		selectedModel := firstString(run.InputSnapshot, "model", "model_id")
		if !restrictionAllows(binding.Restrictions, "model_ids", selectedModel) ||
			!restrictionAllows(binding.Restrictions, "operation_ids", *run.ProviderOperationID) ||
			!variantRestrictionAllows(binding.Restrictions, capability, selectedModel, *run.ProviderOperationID, firstString(run.InputSnapshot, "variant", "variant_id")) {
			return errors.NewStatus(code.ErrAIAppEngineBindingIncompatible, "engine binding restrictions reject the immutable run snapshot")
		}
		return nil
	}
	return errors.NewStatus(code.ErrAIAppEngineUnavailable, "current engine binding is unavailable")
}

func variantRestrictionAllows(restrictions map[string]any, capability *iapiserver.AIAppProviderCapability, modelID, operationID, selectedVariant string) bool {
	allowed := anyStrings(restrictions["variant_ids"])
	if len(allowed) == 0 {
		return true
	}
	if selectedVariant != "" {
		return contains(allowed, selectedVariant)
	}
	for _, variant := range capability.Variants {
		if variant.ModelID == modelID && variant.OperationID == operationID && contains(allowed, variant.ID) {
			return true
		}
	}
	return false
}

func restrictionAllows(restrictions map[string]any, key, selected string) bool {
	allowed := anyStrings(restrictions[key])
	if len(allowed) == 0 {
		return true
	}
	if selected == "" {
		return false
	}
	return contains(allowed, selected)
}

// Completed applies only a newer AtomicTask resource version, then creates and registers output Artifacts idempotently.
func (e *ApplicationRunExecutor) Completed(ctx context.Context, task *iapiserver.AtomicTask) error {
	if task == nil || task.ApplicationRunID == "" {
		return nil
	}
	outputValues := taskOutputValues(task.Output)
	projected, err := e.store.ApplicationPlatforms().ProjectApplicationRun(ctx, task.ApplicationRunID, task.ResourceVersion, task.Status, task.LastError.Message, outputValues)
	if err != nil {
		if errors.ToStatus(err).Code == code.ErrAIAppTaskProjectionStale {
			projected, err = e.store.ApplicationPlatforms().GetApplicationRun(ctx, task.ApplicationRunID)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	e.publish(ctx, "application_run_projection_changed", task.ID+":"+fmt.Sprint(task.ResourceVersion), map[string]any{
		"application_run_id":    projected.ID,
		"atomic_task_id":        task.ID,
		"task_resource_version": task.ResourceVersion,
		"task_status":           task.Status,
		"progress":              map[string]any{"value": task.Progress},
		"output_values":         outputValues,
		"failure_summary":       task.LastError.Message,
	})
	if task.Status != iapiserver.AtomicTaskStatusSuccess {
		return nil
	}
	for _, item := range taskArtifacts(task.Output) {
		artifact := &iapiserver.ApplicationArtifact{
			OwnerUserID:        projected.OwnerUserID,
			ApplicationRunID:   projected.ID,
			OutputKey:          firstString(item, "output_key"),
			MediaType:          firstString(item, "media_type"),
			ContentRef:         firstString(item, "content_ref"),
			RegistrationStatus: iapiserver.ArtifactRegistrationPending,
		}
		if artifact.OutputKey == "" || artifact.ContentRef == "" || artifact.MediaType == "" {
			continue
		}
		created, upsertErr := e.store.ApplicationPlatforms().UpsertArtifact(ctx, artifact)
		if upsertErr != nil {
			return upsertErr
		}
		if created.RegistrationStatus != iapiserver.ArtifactRegistrationPending {
			continue
		}
		assetID, registerErr := e.assets.Register(ctx, created)
		status, errorCode, detail := iapiserver.ArtifactRegistrationRegistered, "", ""
		if registerErr != nil {
			status, errorCode, detail = iapiserver.ArtifactRegistrationFailed, "ERR_AIAPP_ARTIFACT_REGISTRATION_FAILED", registerErr.Error()
		}
		updated, updateErr := e.store.ApplicationPlatforms().UpdateArtifactRegistration(ctx, created.ID, status, assetID, errorCode, detail, created.ResourceVersion)
		if updateErr != nil {
			return updateErr
		}
		e.publish(ctx, "application_artifact_registration_changed", updated.ID+":"+status+":"+fmt.Sprint(updated.ResourceVersion), map[string]any{
			"artifact_id": updated.ID, "application_run_id": updated.ApplicationRunID, "output_key": updated.OutputKey,
			"registration_status": updated.RegistrationStatus, "asset_id": updated.AssetID,
			"registration_error_code": updated.RegistrationErrorCode, "resource_version": updated.ResourceVersion,
		})
	}
	return nil
}

// ReconcileTerminalProjections repairs existing runs and retries idempotent artifact projection after transient failures.
func (e *ApplicationRunExecutor) ReconcileTerminalProjections(ctx context.Context, limit int) error {
	runs, err := e.store.ApplicationPlatforms().ListApplicationRunProjectionCandidates(ctx, limit)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
			continue
		}
		task, taskErr := e.store.TaskCenters().GetAtomicTask(ctx, *run.AtomicTaskID)
		if taskErr != nil || !iapiserver.IsAtomicTaskTerminal(task.Status) {
			continue
		}
		if err := e.Completed(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (e *ApplicationRunExecutor) acquire(ctx context.Context, engineID string, maximum int) (func(), error) {
	if maximum <= 0 {
		maximum = 1
	}
	for {
		e.limitsMu.Lock()
		activity := e.activity[engineID]
		if activity == nil {
			activity = &engineActivity{changed: make(chan struct{})}
			e.activity[engineID] = activity
		}
		if activity.active < maximum {
			activity.active++
			e.limitsMu.Unlock()
			return func() {
				e.limitsMu.Lock()
				activity.active--
				close(activity.changed)
				activity.changed = make(chan struct{})
				e.limitsMu.Unlock()
			}, nil
		}
		changed := activity.changed
		e.limitsMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (e *ApplicationRunExecutor) publishCorrection(ctx context.Context, run *iapiserver.ApplicationRun, cause error) {
	occurredAt := imachinery.Now()
	payload := map[string]any{
		"provider_capability_id": run.ProviderCapabilityID, "provider_capability_revision": run.ProviderCapabilityRevision,
		"operation_id": run.ProviderOperationID, "model_id": firstString(run.InputSnapshot, "model", "model_id"),
		"field": "provider_request", "rejected_value": run.InputSnapshot,
		"provider_error": map[string]any{"code": code.ErrAIAppProviderRuntimeCapabilityMismatch, "message": cause.Error()},
		"occurred_at":    occurredAt,
	}
	key := strings.Join([]string{dereference(run.ProviderCapabilityID), dereference(run.ProviderCapabilityRevision), dereference(run.ProviderOperationID), occurredAt.String()}, ":")
	e.publish(ctx, "provider_capability_correction_required", key, payload)
}

func (e *ApplicationRunExecutor) publish(ctx context.Context, eventType, key string, payload map[string]any) {
	occurredAt := imachinery.Now()
	if _, exists := payload["occurred_at"]; !exists {
		payload["occurred_at"] = occurredAt
	}
	if err := e.events.Publish(context.WithoutCancel(ctx), &iapiserver.ApplicationPlatformEvent{Type: eventType, IdempotencyKey: key, Payload: payload, OccurredAt: occurredAt}); err != nil {
		log.Errorf("application platform event publish failed: type=%s key=%s error=%v", eventType, key, err)
	}
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func taskOutputValues(output map[string]any) []map[string]any {
	if output == nil {
		return []map[string]any{}
	}
	if values, ok := output["values"].(map[string]any); ok {
		return []map[string]any{values}
	}
	return []map[string]any{output}
}

func taskArtifacts(output map[string]any) []map[string]any {
	items := []map[string]any{}
	for _, raw := range anySlice(output["artifacts"]) {
		if item, ok := raw.(map[string]any); ok {
			items = append(items, item)
		}
	}
	return items
}

func anySlice(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []map[string]any:
		result := make([]any, len(items))
		for index := range items {
			result[index] = items[index]
		}
		return result
	default:
		return nil
	}
}
