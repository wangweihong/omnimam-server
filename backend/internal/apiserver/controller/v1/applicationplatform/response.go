package applicationplatform

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type errorDefinition struct {
	name      string
	retryable bool
}

var applicationPlatformErrors = map[int]errorDefinition{
	code.ErrAIAppProviderCapabilityUnavailable:           {name: "ERR_AIAPP_PROVIDER_CAPABILITY_UNAVAILABLE"},
	code.ErrAIAppProviderCapabilityNotFound:              {name: "ERR_AIAPP_PROVIDER_CAPABILITY_NOT_FOUND"},
	code.ErrAIAppEngineInstanceNotFound:                  {name: "ERR_AIAPP_ENGINE_INSTANCE_NOT_FOUND"},
	code.ErrAIAppEngineAuthConfigInvalid:                 {name: "ERR_AIAPP_ENGINE_AUTH_CONFIG_INVALID"},
	code.ErrAIAppEngineBindingIncompatible:               {name: "ERR_AIAPP_ENGINE_BINDING_INCOMPATIBLE"},
	code.ErrAIAppEngineBindingRestrictionExpands:         {name: "ERR_AIAPP_ENGINE_BINDING_RESTRICTION_EXPANDS"},
	code.ErrAIAppEngineUnavailable:                       {name: "ERR_AIAPP_ENGINE_UNAVAILABLE", retryable: true},
	code.ErrAIAppEngineReferenceBlocked:                  {name: "ERR_AIAPP_ENGINE_REFERENCE_BLOCKED"},
	code.ErrAIAppEngineBindingNotFound:                   {name: "ERR_AIAPP_ENGINE_BINDING_NOT_FOUND"},
	code.ErrAIAppSystemEngineBindingImmutable:            {name: "ERR_AIAPP_SYSTEM_ENGINE_BINDING_IMMUTABLE"},
	code.ErrAIAppRequiredEngineBindingFailed:             {name: "ERR_AIAPP_REQUIRED_ENGINE_BINDING_FAILED", retryable: true},
	code.ErrAIAppEngineInstanceNameDuplicated:            {name: "ERR_AIAPP_ENGINE_INSTANCE_NAME_DUPLICATED"},
	code.ErrAIAppTemplateSourceInvalid:                   {name: "ERR_AIAPP_TEMPLATE_SOURCE_INVALID"},
	code.ErrAIAppApplicationVersionImmutable:             {name: "ERR_AIAPP_APPLICATION_VERSION_IMMUTABLE"},
	code.ErrAIAppRuntimeFormNoValidVariant:               {name: "ERR_AIAPP_RUNTIME_FORM_NO_VALID_VARIANT"},
	code.ErrAIAppApplicationInputInvalid:                 {name: "ERR_AIAPP_APPLICATION_INPUT_INVALID"},
	code.ErrAIAppTemplateNotFound:                        {name: "ERR_AIAPP_TEMPLATE_NOT_FOUND"},
	code.ErrAIAppTemplateVersionNotFound:                 {name: "ERR_AIAPP_TEMPLATE_VERSION_NOT_FOUND"},
	code.ErrAIAppApplicationNotFound:                     {name: "ERR_AIAPP_APPLICATION_NOT_FOUND"},
	code.ErrAIAppApplicationVersionNotFound:              {name: "ERR_AIAPP_APPLICATION_VERSION_NOT_FOUND"},
	code.ErrAIAppTemplateVersionNotPublishable:           {name: "ERR_AIAPP_TEMPLATE_VERSION_NOT_PUBLISHABLE"},
	code.ErrAIAppApplicationVersionNotPublishable:        {name: "ERR_AIAPP_APPLICATION_VERSION_NOT_PUBLISHABLE"},
	code.ErrAIAppApplicationSemanticVersionDuplicated:    {name: "ERR_AIAPP_APPLICATION_SEMANTIC_VERSION_DUPLICATED"},
	code.ErrAIAppResourceVersionConflict:                 {name: "ERR_AIAPP_RESOURCE_VERSION_CONFLICT", retryable: true},
	code.ErrAIAppApplicationRunCreateFailed:              {name: "ERR_AIAPP_APPLICATION_RUN_CREATE_FAILED", retryable: true},
	code.ErrAIAppTaskProjectionStale:                     {name: "ERR_AIAPP_TASK_PROJECTION_STALE"},
	code.ErrAIAppProviderRuntimeCapabilityMismatch:       {name: "ERR_AIAPP_PROVIDER_RUNTIME_CAPABILITY_MISMATCH"},
	code.ErrAIAppTaskRunCreateFailed:                     {name: "ERR_AIAPP_TASK_RUN_CREATE_FAILED", retryable: true},
	code.ErrAIAppAtomicTaskCreateFailed:                  {name: "ERR_AIAPP_ATOMIC_TASK_CREATE_FAILED", retryable: true},
	code.ErrAIAppAtomicTaskProjectionStale:               {name: "ERR_AIAPP_ATOMIC_TASK_PROJECTION_STALE"},
	code.ErrAIAppArtifactRegistrationAtomicTaskUnchanged: {name: "ERR_AIAPP_ARTIFACT_REGISTRATION_ATOMIC_TASK_UNCHANGED"},
	code.ErrAIAppArtifactProcessingFailed:                {name: "ERR_AIAPP_ARTIFACT_PROCESSING_FAILED", retryable: true},
	code.ErrAIAppArtifactStateInvalid:                    {name: "ERR_AIAPP_ARTIFACT_STATE_INVALID"},
	code.ErrAIAppArtifactRegistrationFailed:              {name: "ERR_AIAPP_ARTIFACT_REGISTRATION_FAILED", retryable: true},
	code.ErrAIAppApplicationRunNotFound:                  {name: "ERR_AIAPP_APPLICATION_RUN_NOT_FOUND"},
	code.ErrAIAppPermissionDenied:                        {name: "ERR_AIAPP_PERMISSION_DENIED"},
	code.ErrAIAppComfyUIWorkflowFileInvalid:              {name: "ERR_AIAPP_COMFYUI_WORKFLOW_FILE_INVALID"},
	code.ErrAIAppComfyUIEngineTypeInvalid:                {name: "ERR_AIAPP_COMFYUI_ENGINE_TYPE_INVALID"},
	code.ErrAIAppComfyUIObjectInfoUnavailable:            {name: "ERR_AIAPP_COMFYUI_OBJECT_INFO_UNAVAILABLE", retryable: true},
	code.ErrAIAppComfyUIWorkflowReferenceInvalid:         {name: "ERR_AIAPP_COMFYUI_WORKFLOW_REFERENCE_INVALID"},
	code.ErrAIAppComfyUIWorkflowIncompatible:             {name: "ERR_AIAPP_COMFYUI_WORKFLOW_INCOMPATIBLE"},
	code.ErrAIAppComfyUIWorkflowNotFound:                 {name: "ERR_AIAPP_COMFYUI_WORKFLOW_NOT_FOUND"},
	code.ErrAIAppComfyUIWorkflowArchived:                 {name: "ERR_AIAPP_COMFYUI_WORKFLOW_ARCHIVED"},
	code.ErrAIAppComfyUIValidationNotFound:               {name: "ERR_AIAPP_COMFYUI_VALIDATION_NOT_FOUND"},
	code.ErrAIAppComfyUIValidationNotCompatible:          {name: "ERR_AIAPP_COMFYUI_VALIDATION_NOT_COMPATIBLE"},
	code.ErrAIAppComfyUITemplateContractInvalid:          {name: "ERR_AIAPP_COMFYUI_TEMPLATE_CONTRACT_INVALID"},
	code.ErrAIAppComfyUIWorkflowAlreadyConverted:         {name: "ERR_AIAPP_COMFYUI_WORKFLOW_ALREADY_CONVERTED"},
	code.ErrAIAppComfyUIConversionIdempotencyConflict:    {name: "ERR_AIAPP_COMFYUI_CONVERSION_IDEMPOTENCY_CONFLICT"},
	code.ErrAIAppComfyUIResourceVersionConflict:          {name: "ERR_AIAPP_COMFYUI_RESOURCE_VERSION_CONFLICT", retryable: true},
	code.ErrAIAppComfyUIWorkflowAccessDenied:             {name: "ERR_AIAPP_COMFYUI_WORKFLOW_ACCESS_DENIED"},
	code.ErrAIAppComfyUIWorkflowSourceInvalid:            {name: "ERR_AIAPP_COMFYUI_WORKFLOW_SOURCE_INVALID"},
	code.ErrAIAppComfyUIAPIConversionBlocked:             {name: "ERR_AIAPP_COMFYUI_API_CONVERSION_BLOCKED"},
	code.ErrAIAppComfyUIAPINotReady:                      {name: "ERR_AIAPP_COMFYUI_API_NOT_READY"},
	code.ErrAIAppComfyUITestEngineUnavailable:            {name: "ERR_AIAPP_COMFYUI_TEST_ENGINE_UNAVAILABLE", retryable: true},
	code.ErrAIAppComfyUITestIncompatible:                 {name: "ERR_AIAPP_COMFYUI_TEST_INCOMPATIBLE"},
	code.ErrAIAppComfyUITestParameterInvalid:             {name: "ERR_AIAPP_COMFYUI_TEST_PARAMETER_INVALID"},
	code.ErrAIAppComfyUITestRunNotFound:                  {name: "ERR_AIAPP_COMFYUI_TEST_RUN_NOT_FOUND"},
	code.ErrAIAppComfyUITestPreviewUnavailable:           {name: "ERR_AIAPP_COMFYUI_TEST_PREVIEW_UNAVAILABLE", retryable: true},
	code.ErrAIAppComfyUITestRunStateBlocked:              {name: "ERR_AIAPP_COMFYUI_TEST_RUN_STATE_BLOCKED"},
	code.ErrAIAppComfyUIObjectInfoRefreshNotAllowed:      {name: "ERR_AIAPP_COMFYUI_OBJECT_INFO_REFRESH_NOT_ALLOWED"},
	code.ErrAIAppComfyUIObjectInfoRefreshFailed:          {name: "ERR_AIAPP_COMFYUI_OBJECT_INFO_REFRESH_FAILED", retryable: true},
}

type applicationPlatformErrorResponse struct {
	Code      string            `json:"code"`
	Value     int               `json:"value"`
	Message   string            `json:"message"`
	Messages  map[string]string `json:"messages"`
	Detail    string            `json:"detail,omitempty"`
	Retryable bool              `json:"retryable"`
	Data      any               `json:"data,omitempty"`
}

func run[T any](ctx *gin.Context, request T, action func(T) (any, error)) {
	if err := core.DecodeParameter(ctx, request); err != nil {
		writeResponse(ctx, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, err.Error()), nil)
		return
	}
	result, err := action(request)
	if err != nil {
		writeResponse(ctx, err, nil)
		return
	}
	if postRun, ok := result.(imachinery.PostRun); ok {
		result = postRun.Transform()
	}
	writeResponse(ctx, nil, result)
}

func writeResponse(ctx *gin.Context, err error, data any) {
	if err == nil {
		ctx.JSON(http.StatusOK, data)
		return
	}
	log.Errorf("%#+v", err)
	status := errors.ToStatus(err)
	definition, ok := applicationPlatformErrors[status.Code]
	if !ok {
		originalDetail := status.Desc
		status = errors.ToStatus(errors.NewStatus(code.ErrAIAppApplicationInputInvalid, originalDetail))
		definition = applicationPlatformErrors[code.ErrAIAppApplicationInputInvalid]
	}
	ctx.JSON(http.StatusOK, applicationPlatformErrorResponse{
		Code: definition.name, Value: status.Code,
		Message: status.Message[errors.MessageLangENKey],
		Messages: map[string]string{
			"zh-CN": status.Message[errors.MessageLangCNKey],
			"en-US": status.Message[errors.MessageLangENKey],
		},
		Detail: status.Desc, Retryable: definition.retryable, Data: data,
	})
}
