package applicationplatform

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	enginegateway "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/engine"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// ApplicationRunExecutor executes Task Center claims and projects terminal facts back to ApplicationRun.
type ApplicationRunExecutor struct {
	store        store.Factory
	runtime      *appregistry.RuntimeRegistry
	capabilities *appregistry.ProviderCapabilityRegistry
	adapters     map[string]enginegateway.Adapter
	executors    map[string]enginegateway.OperationExecutor
	assets       ArtifactLifecycle
	events       EventPublisher
	limitsMu     sync.Mutex
	activity     map[string]*engineActivity
}

type engineActivity struct {
	active  int
	changed chan struct{}
}

func NewApplicationRunExecutor(str store.Factory, runtime *appregistry.RuntimeRegistry, capabilities *appregistry.ProviderCapabilityRegistry, adapters map[string]enginegateway.Adapter, executors map[string]enginegateway.OperationExecutor, assets ArtifactLifecycle, events EventPublisher) (*ApplicationRunExecutor, error) {
	if str == nil || runtime == nil {
		return nil, fmt.Errorf("application run executor store and runtime registry are required")
	}
	if assets == nil {
		assets = NoopArtifactLifecycle{}
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
	capabilityID := maputil.FirstString(run.ExecutionSnapshot, "capability_definition_id")
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
		workflow := typeutil.As[map[string]any](run.CapabilitySourceSnapshot["comfyui_api_workflow"])
		contract := typeutil.As[map[string]any](run.CapabilitySourceSnapshot["template_contract"])
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
		selectedModel := maputil.FirstString(run.InputSnapshot, "model", "model_id")
		if !restrictionAllows(binding.Restrictions, "model_ids", selectedModel) ||
			!restrictionAllows(binding.Restrictions, "operation_ids", *run.ProviderOperationID) ||
			!variantRestrictionAllows(binding.Restrictions, capability, selectedModel, *run.ProviderOperationID, maputil.FirstString(run.InputSnapshot, "variant", "variant_id")) {
			return errors.NewStatus(code.ErrAIAppEngineBindingIncompatible, "engine binding restrictions reject the immutable run snapshot")
		}
		return nil
	}
	return errors.NewStatus(code.ErrAIAppEngineUnavailable, "current engine binding is unavailable")
}

func variantRestrictionAllows(restrictions map[string]any, capability *iapiserver.AIAppProviderCapability, modelID, operationID, selectedVariant string) bool {
	allowed := typeutil.SliceAs[string](restrictions["variant_ids"])
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
	allowed := typeutil.SliceAs[string](restrictions[key])
	if len(allowed) == 0 {
		return true
	}
	if selected == "" {
		return false
	}
	return contains(allowed, selected)
}

// Completed applies only a newer AtomicTask resource version, then delivers output bytes to Asset Library and projects Artifact references.
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
	items := taskArtifacts(task.Output)
	if len(items) == 0 {
		return nil
	}
	engine, err := e.store.ApplicationPlatforms().GetEngineInstance(ctx, projected.EngineInstanceID)
	if err != nil {
		return err
	}
	legacy, err := e.store.ApplicationPlatforms().ListArtifactsByRun(ctx, projected.ID)
	if err != nil {
		return err
	}
	legacyAssets := make(map[string]string, len(legacy))
	for _, item := range legacy {
		if item != nil && item.AssetID != nil {
			legacyAssets[item.OutputKey] = *item.AssetID
		}
	}
	sequences := map[string]int{}
	for _, item := range items {
		outputKey := maputil.FirstString(item, "output_key")
		contentRef := maputil.FirstString(item, "content_ref")
		mediaType := maputil.FirstString(item, "media_type")
		sequence := artifactSequence(item, sequences[outputKey])
		if sequence >= sequences[outputKey] {
			sequences[outputKey] = sequence + 1
		}
		if outputKey == "" || contentRef == "" || mediaType == "" {
			continue
		}
		artifact := &iapiserver.Artifact{
			OwnerUserID: projected.OwnerUserID, ProducerType: "application_run", ProducerID: projected.ID,
			ProducerIdempotencyKey: projected.ID + ":" + outputKey + ":" + strconv.Itoa(sequence),
			AtomicTaskID:           task.ID, ApplicationRunID: projected.ID, OutputKey: outputKey, Sequence: sequence,
			ArtifactType: mediaType, MediaType: mediaType, SavePolicy: iapiserver.ArtifactSaveAutomatic,
			ProcessingProfileVersion: "default-v1", Metadata: map[string]any{},
		}
		current, _, prepareErr := e.assets.Prepare(ctx, artifact)
		if prepareErr != nil {
			return prepareErr
		}
		if err := e.projectArtifact(ctx, current); err != nil {
			return err
		}
		if current.ProcessingStatus != iapiserver.ArtifactProcessingReady {
			reader, mimeType, openErr := e.openArtifactContent(ctx, engine, contentRef, mediaType)
			if openErr != nil {
				failed, failErr := e.assets.FailProcessing(ctx, current, artifactProcessingErrorCode(openErr), openErr.Error())
				if failErr != nil {
					return failErr
				}
				if err := e.projectArtifact(ctx, failed); err != nil {
					return err
				}
				continue
			}
			stored, storeErr := e.assets.StoreContent(ctx, current, mimeType, reader)
			closeErr := reader.Close()
			if storeErr == nil {
				storeErr = closeErr
			}
			if storeErr != nil {
				failed, failErr := e.assets.FailProcessing(ctx, current, "artifact_content_unavailable", storeErr.Error())
				if failErr != nil {
					return failErr
				}
				if err := e.projectArtifact(ctx, failed); err != nil {
					return err
				}
				continue
			}
			current = stored
			if err := e.projectArtifact(ctx, current); err != nil {
				return err
			}
		}
		if current.RegistrationStatus == iapiserver.ArtifactRegistrationRegistered {
			continue
		}
		registered, registerErr := e.assets.Register(ctx, current, legacyAssets[outputKey])
		if registerErr != nil {
			failed, failErr := e.assets.FailRegistration(ctx, current, "artifact_registration_invalid", registerErr.Error())
			if failErr != nil {
				return failErr
			}
			if err := e.projectArtifact(ctx, failed); err != nil {
				return err
			}
			continue
		}
		if err := e.projectArtifact(ctx, registered); err != nil {
			return err
		}
	}
	return nil
}

func artifactProcessingErrorCode(err error) string {
	switch errors.ToStatus(err).Code {
	case code.ErrArtifactSourceForbidden:
		return "artifact_source_forbidden"
	case code.ErrArtifactMediaInvalid:
		return "artifact_media_invalid"
	default:
		return "artifact_content_unavailable"
	}
}

func (e *ApplicationRunExecutor) projectArtifact(ctx context.Context, artifact *iapiserver.Artifact) error {
	if artifact == nil || artifact.ApplicationRunID == "" {
		return nil
	}
	var assetID, assetVersionID, lastErrorCode *string
	if artifact.AssetID != "" {
		value := artifact.AssetID
		assetID = &value
	}
	if artifact.AssetVersionID != "" {
		value := artifact.AssetVersionID
		assetVersionID = &value
	}
	errorCode := artifact.ProcessingErrorCode
	if errorCode == "" {
		errorCode = artifact.RegistrationErrorCode
	}
	if errorCode != "" {
		lastErrorCode = &errorCode
	}
	ref := &iapiserver.ApplicationArtifactRef{
		ApplicationRunID: artifact.ApplicationRunID, ArtifactID: artifact.ID, OutputKey: artifact.OutputKey,
		Sequence: artifact.Sequence, MediaType: artifact.MediaType, ArtifactProcessingStatus: artifact.ProcessingStatus,
		ArtifactRegistrationStatus: artifact.RegistrationStatus, AssetID: assetID, AssetVersionID: assetVersionID,
		ArtifactResourceVersion: artifact.ResourceVersion, LastErrorCode: lastErrorCode,
	}
	projected, applied, err := e.store.ApplicationPlatforms().ProjectApplicationArtifactRef(ctx, ref)
	if err != nil || !applied {
		return err
	}
	_ = projected
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
		"operation_id": run.ProviderOperationID, "model_id": maputil.FirstString(run.InputSnapshot, "model", "model_id"),
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

const maximumArtifactContentBytes = int64(10 << 30)

func (e *ApplicationRunExecutor) openArtifactContent(ctx context.Context, engine *iapiserver.EngineInstance, rawURL, mediaType string) (io.ReadCloser, string, error) {
	target, err := url.Parse(rawURL)
	if err != nil || target.Host == "" || target.User != nil || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, "", errors.NewStatus(code.ErrArtifactSourceForbidden, "artifact source URL is invalid")
	}
	engineURL, engineErr := url.Parse(engine.BaseURL)
	trustedHost := ""
	trustedOrigin := engineErr == nil && engineURL.Host != "" &&
		strings.EqualFold(target.Scheme, engineURL.Scheme) && strings.EqualFold(target.Host, engineURL.Host)
	if trustedOrigin {
		trustedHost = strings.ToLower(target.Hostname())
	} else if target.Scheme != "https" {
		return nil, "", errors.NewStatus(code.ErrArtifactSourceForbidden, "external artifact source must use HTTPS")
	}
	transport := artifactDownloadTransport(trustedHost)
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(target.String()).WithMethod(http.MethodGet).AddHeaderParam("Accept", "*/*")
	if trustedOrigin {
		if err := enginegateway.ApplyProviderAuthentication(builder, engine, http.MethodGet, target.RequestURI(), nil); err != nil {
			return nil, "", err
		}
	}
	response, err := builder.Build().InvokeWithContext(ctx, httpcli.TimeoutCallOption(5*time.Minute), httpcli.CallOptionTransport(transport))
	if err != nil {
		return nil, "", errors.NewStatus(code.ErrArtifactContentUnavailable, "artifact content download failed")
	}
	if response.Response == nil || response.Response.Body == nil {
		return nil, "", errors.NewStatus(code.ErrArtifactContentUnavailable, "artifact content response is empty")
	}
	if response.GetStatusCode() < http.StatusOK || response.GetStatusCode() >= http.StatusMultipleChoices {
		_ = response.Response.Body.Close()
		return nil, "", errors.NewStatus(code.ErrArtifactContentUnavailable, "artifact content download was rejected")
	}
	if response.Response.ContentLength > maximumArtifactContentBytes {
		_ = response.Response.Body.Close()
		return nil, "", errors.NewStatus(code.ErrArtifactMediaInvalid, "artifact content exceeds the supported size")
	}
	contentType, _, _ := mime.ParseMediaType(response.GetHeader("Content-Type"))
	if contentType == "" {
		contentType = defaultArtifactMIMEType(mediaType)
	}
	return &boundedArtifactReadCloser{
		Reader: io.LimitReader(response.Response.Body, maximumArtifactContentBytes+1),
		body:   response.Response.Body,
		limit:  maximumArtifactContentBytes,
	}, contentType, nil
}

func artifactDownloadTransport(trustedHost string) *http.Transport {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if strings.EqualFold(host, trustedHost) {
				return dialer.DialContext(ctx, network, address)
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, errors.NewStatus(code.ErrArtifactSourceForbidden, "artifact source host could not be resolved")
			}
			for _, ip := range ips {
				if forbiddenArtifactIP(ip) {
					return nil, errors.NewStatus(code.ErrArtifactSourceForbidden, "artifact source resolves to a private address")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
}

func forbiddenArtifactIP(ip net.IP) bool {
	return ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

type boundedArtifactReadCloser struct {
	io.Reader
	body  io.Closer
	read  int64
	limit int64
}

func (r *boundedArtifactReadCloser) Read(buffer []byte) (int, error) {
	n, err := r.Reader.Read(buffer)
	r.read += int64(n)
	if r.read > r.limit {
		return n, errors.NewStatus(code.ErrArtifactMediaInvalid, "artifact content exceeds the supported size")
	}
	return n, err
}

func (r *boundedArtifactReadCloser) Close() error { return r.body.Close() }

func defaultArtifactMIMEType(mediaType string) string {
	switch mediaType {
	case "image":
		return "image/png"
	case "video":
		return "video/mp4"
	case "audio":
		return "audio/mpeg"
	case "text", "prompt", "prompt_template":
		return "text/plain"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func artifactSequence(item map[string]any, fallback int) int {
	for _, key := range []string{"sequence", "index"} {
		switch value := item[key].(type) {
		case int:
			if value >= 0 {
				return value
			}
		case float64:
			if value >= 0 && value == float64(int(value)) {
				return int(value)
			}
		}
	}
	return fallback
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
	for _, raw := range sliceutil.ToInterfaceSlice(output["artifacts"]) {
		if item, ok := raw.(map[string]any); ok {
			items = append(items, item)
		}
	}
	return items
}
