package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

// +k8s:deepcopy-gen=true
type InfraRuntimeListResponse struct {
	Total int64           `json:"total"`
	Items []*InfraRuntime `json:"items"`
}

// +k8s:deepcopy-gen=true
type InfraNodeListResponse struct {
	Total int64        `json:"total"`
	Items []*InfraNode `json:"items"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeProfileListResponse struct {
	Total int64                  `json:"total"`
	Items []*InfraRuntimeProfile `json:"items"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeOutputListResponse struct {
	Total int64                 `json:"total"`
	Items []*InfraRuntimeOutput `json:"items"`
}

// +k8s:deepcopy-gen=true
type InfraResolvedEndpoint struct {
	EndpointRef string          `json:"endpoint_ref"`
	RuntimeID   string          `json:"runtime_id"`
	Protocol    string          `json:"protocol"`
	BaseURL     string          `json:"base_url"`
	ResolvedAt  imachinery.Time `json:"resolved_at"`
	ValidUntil  imachinery.Time `json:"valid_until"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeLogListResponse struct {
	Total int64                   `json:"total"`
	Items []*InfraRuntimeLogEntry `json:"items"`
}

// +k8s:deepcopy-gen=true
type InfraOperationResult struct {
	Runtime        *InfraRuntime         `json:"runtime"`
	Endpoint       *InfraRuntimeEndpoint `json:"endpoint,omitempty"`
	Outputs        []*InfraRuntimeOutput `json:"outputs,omitempty"`
	ArtifactDigest string                `json:"artifact_digest,omitempty"`
	LogsRef        string                `json:"logs_ref,omitempty"`
}
