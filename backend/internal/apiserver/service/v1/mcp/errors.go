package mcp

import (
	stderrors "errors"
	"fmt"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func rpcBusiness(failure *protocol.BusinessError) *protocol.RPCError {
	return &protocol.RPCError{Code: protocol.JSONRPCBusinessError, Message: failure.Message, Data: failure}
}

func mcpError(value int, name string, retryable bool, sourceDomain string) *protocol.BusinessError {
	status := toolerrors.ToStatus(toolerrors.NewStatus(value, ""))
	english := status.Message[toolerrors.MessageLangENKey]
	chinese := status.Message[toolerrors.MessageLangCNKey]
	if english == "" {
		english = name
	}
	if chinese == "" {
		chinese = english
	}
	return &protocol.BusinessError{
		Code: name, Value: value, Message: english,
		Messages:  map[string]string{"zh-CN": chinese, "en-US": english},
		Retryable: retryable, SourceDomain: sourceDomain,
	}
}

func sourceBusinessError(err error) *protocol.BusinessError {
	var rpcErr *protocol.RPCError
	if stderrors.As(err, &rpcErr) && rpcErr.Data != nil {
		return rpcErr.Data
	}
	status := toolerrors.ToStatus(err)
	domain := sourceDomain(status.Code)
	name := sourceErrorNames[status.Code]
	if name == "" {
		return mcpError(code.ErrMCPToolResultInvalid, "ERR_MCP_TOOL_RESULT_INVALID", true, "mcp")
	}
	english := status.Message[toolerrors.MessageLangENKey]
	chinese := status.Message[toolerrors.MessageLangCNKey]
	if english == "" {
		english = "The requested operation failed."
	}
	if chinese == "" {
		chinese = english
	}
	return &protocol.BusinessError{
		Code: name, Value: status.Code, Message: english,
		Messages:  map[string]string{"zh-CN": chinese, "en-US": english},
		Retryable: retryableSourceErrors[status.Code], SourceDomain: domain,
	}
}

func sourceDomain(value int) string {
	switch {
	case value >= 190000 && value <= 190999:
		return "mcp"
	case value >= 130000 && value <= 139999:
		return "application-platform"
	case value >= 140000 && value <= 149999:
		return "task-center"
	case value >= 150000 && value <= 159999:
		return "asset-library"
	case value == code.ErrTokenInvalid || value == code.ErrMissingHeader || value == code.ErrInvalidAuthHeader || value == code.ErrPermissionDenied:
		return "identity"
	default:
		return "mcp"
	}
}

var sourceErrorNames = map[int]string{
	code.ErrTokenInvalid:                            "ERR_MCP_AUTHENTICATION_REQUIRED",
	code.ErrMissingHeader:                           "ERR_MCP_AUTHENTICATION_REQUIRED",
	code.ErrInvalidAuthHeader:                       "ERR_MCP_AUTHENTICATION_REQUIRED",
	code.ErrPermissionDenied:                        "ERR_MCP_PERMISSION_DENIED",
	code.ErrAIAppProviderCapabilityUnavailable:      "ERR_AIAPP_PROVIDER_CAPABILITY_UNAVAILABLE",
	code.ErrAIAppProviderCapabilityNotFound:         "ERR_AIAPP_PROVIDER_CAPABILITY_NOT_FOUND",
	code.ErrAIAppEngineUnavailable:                  "ERR_AIAPP_ENGINE_UNAVAILABLE",
	code.ErrAIAppApplicationInputInvalid:            "ERR_AIAPP_APPLICATION_INPUT_INVALID",
	code.ErrAIAppApplicationNotFound:                "ERR_AIAPP_APPLICATION_NOT_FOUND",
	code.ErrAIAppApplicationVersionNotFound:         "ERR_AIAPP_APPLICATION_VERSION_NOT_FOUND",
	code.ErrAIAppApplicationRunCreateFailed:         "ERR_AIAPP_APPLICATION_RUN_CREATE_FAILED",
	code.ErrAIAppAtomicTaskCreateFailed:             "ERR_AIAPP_ATOMIC_TASK_CREATE_FAILED",
	code.ErrAIAppApplicationRunNotFound:             "ERR_AIAPP_APPLICATION_RUN_NOT_FOUND",
	code.ErrAIAppPermissionDenied:                   "ERR_AIAPP_PERMISSION_DENIED",
	code.ErrAIAppProviderRuntimeCapabilityMismatch:  "ERR_AIAPP_PROVIDER_RUNTIME_CAPABILITY_MISMATCH",
	code.ErrAtomicTaskNotFound:                      "ERR_ATOMIC_TASK_NOT_FOUND",
	code.ErrAtomicTaskStateBlocked:                  "ERR_ATOMIC_TASK_STATE_BLOCKED",
	code.ErrTaskPermissionDenied:                    "ERR_TASK_PERMISSION_DENIED",
	code.ErrTaskWorkerNotAvailable:                  "ERR_TASK_WORKER_NOT_AVAILABLE",
	code.ErrAssetListFailed:                         "ERR_ASSET_LIST_FAILED",
	code.ErrAssetQueryParametersInvalid:             "ERR_ASSET_QUERY_PARAMETERS_INVALID",
	code.ErrAssetNotFoundOrNotVisible:               "ERR_ASSET_NOT_FOUND_OR_NOT_VISIBLE",
	code.ErrAssetNotFoundOrNotWritable:              "ERR_ASSET_NOT_FOUND_OR_NOT_WRITABLE",
	code.ErrAssetRepresentationNotFoundOrNotVisible: "ERR_ASSET_REPRESENTATION_NOT_FOUND_OR_NOT_VISIBLE",
	code.ErrAssetContentUnavailable:                 "ERR_ASSET_CONTENT_UNAVAILABLE",
	code.ErrAssetUploadRequestInvalid:               "ERR_ASSET_UPLOAD_REQUEST_INVALID",
	code.ErrAssetUploadNotFoundOrNotVisible:         "ERR_ASSET_UPLOAD_NOT_FOUND_OR_NOT_VISIBLE",
	code.ErrAssetUploadStateInvalid:                 "ERR_ASSET_UPLOAD_STATE_INVALID",
	code.ErrAssetUploadPartInvalid:                  "ERR_ASSET_UPLOAD_PART_INVALID",
	code.ErrAssetUploadChecksumMismatch:             "ERR_ASSET_UPLOAD_CHECKSUM_MISMATCH",
	code.ErrAssetUploadStorageFailed:                "ERR_ASSET_UPLOAD_STORAGE_FAILED",
	code.ErrArtifactOwnerMismatch:                   "ERR_ARTIFACT_OWNER_MISMATCH",
	code.ErrArtifactContentUnavailable:              "ERR_ARTIFACT_CONTENT_UNAVAILABLE",
}

var retryableSourceErrors = map[int]bool{
	code.ErrAIAppEngineUnavailable:          true,
	code.ErrAIAppApplicationRunCreateFailed: true,
	code.ErrAIAppAtomicTaskCreateFailed:     true,
	code.ErrTaskWorkerNotAvailable:          true,
	code.ErrAssetListFailed:                 true,
	code.ErrAssetUploadStorageFailed:        true,
	code.ErrAssetContentUnavailable:         true,
}

func internalMCPError(value int, name string, retryable bool, detail string) error {
	failure := mcpError(value, name, retryable, "mcp")
	failure.Detail = detail
	return rpcBusiness(failure)
}

func invalidCursorError(err error) error {
	detail := "invalid MCP cursor"
	if err != nil {
		detail = fmt.Sprintf("invalid MCP cursor: %v", err)
	}
	return internalMCPError(code.ErrMCPCursorInvalid, "ERR_MCP_CURSOR_INVALID", false, detail)
}
