package infrastructure

import "github.com/wangweihong/omnimam/backend/apis/iapiserver"

type CommandRequest struct {
	Operation string                                `json:"operation"`
	RuntimeID string                                `json:"runtime_id,omitempty"`
	Delete    bool                                  `json:"delete,omitempty"`
	Create    *iapiserver.InfraCreateRuntimeRequest `json:"create,omitempty"`
}
type CommandResponse struct {
	Result *iapiserver.InfraOperationResult `json:"result,omitempty"`
	Error  string                           `json:"error,omitempty"`
}
