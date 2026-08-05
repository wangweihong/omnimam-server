package infrastructure

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type CommandCreateContext struct {
	AuthorizationRef   string          `json:"authorization_ref,omitempty"`
	EndpointVisibility string          `json:"endpoint_visibility,omitempty"`
	FunctionRef        string          `json:"function_ref,omitempty"`
	FunctionArguments  json.RawMessage `json:"function_arguments,omitempty"`
}

type CommandRequest struct {
	Operation     string                                `json:"operation"`
	RuntimeID     string                                `json:"runtime_id,omitempty"`
	Delete        bool                                  `json:"delete,omitempty"`
	Create        *iapiserver.InfraCreateRuntimeRequest `json:"create,omitempty"`
	CreateContext *CommandCreateContext                 `json:"create_context,omitempty"`
}
type CommandResponse struct {
	Result *iapiserver.InfraOperationResult `json:"result,omitempty"`
	Error  string                           `json:"error,omitempty"`
}
