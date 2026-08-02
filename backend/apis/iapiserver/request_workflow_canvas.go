package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

type WorkflowNodeDefinitionListRequest struct {
	imachinery.BasicQueryParam
	Category          string `form:"category"           binding:"omitempty,max=100"`
	NodeKind          string `form:"node_kind"          binding:"omitempty,oneof=data processor generator controller orchestrator viewer"`
	ExecutionMode     string `form:"execution_mode"     binding:"omitempty,oneof=passive compile_time atomic expanded"`
	IncludeDeprecated bool   `form:"include_deprecated"`
}
type WorkflowNodeDefinitionRegisterRequest struct {
	NodeType                string                      `json:"node_type"                 binding:"required,max=200"`
	DefinitionVersion       string                      `json:"definition_version"        binding:"required,max=64"`
	Title                   string                      `json:"title"                     binding:"required,max=200"`
	Description             string                      `json:"description"               binding:"omitempty,max=2000"`
	Category                string                      `json:"category"                  binding:"required,max=100"`
	NodeKind                string                      `json:"node_kind"                 binding:"required,oneof=data processor generator controller orchestrator viewer"`
	Ports                   []WorkflowPortDefinition    `json:"ports"                     binding:"required,max=200,dive"`
	ConfigSchema            map[string]any              `json:"config_schema"             binding:"required"`
	ControllerStateSchema   map[string]any              `json:"controller_state_schema"`
	ControllerSchemaVersion *string                     `json:"controller_schema_version" binding:"omitempty,max=64"`
	ExecutionBinding        WorkflowExecutionBinding    `json:"execution_binding"         binding:"required"`
	Renderer                *WorkflowRendererCapability `json:"renderer"`
	CacheAllowed            bool                        `json:"cache_allowed"`
	ReuseTTLSeconds         *int                        `json:"reuse_ttl_seconds"         binding:"omitempty,min=1"`
	AvailabilityScope       string                      `json:"availability_scope"        binding:"required,oneof=SYSTEM PROJECT"`
	ProjectID               *string                     `json:"project_id"                binding:"omitempty,max=128"`
	Namespace               *string                     `json:"namespace"                 binding:"omitempty,max=128"`
}
type WorkflowNodeDefinitionDeprecateRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=1000"`
}

type WorkflowCanvasListRequest struct {
	imachinery.BasicQueryParam
	Visibility string `form:"visibility" binding:"omitempty,oneof=PRIVATE PROJECT"`
	ProjectID  string `form:"project_id" binding:"omitempty,max=128"`
	Namespace  string `form:"namespace"  binding:"omitempty,max=128"`
}
type WorkflowCanvasCreateRequest struct {
	Name        string              `json:"name"        binding:"required,max=200"`
	Description string              `json:"description" binding:"omitempty,max=2000"`
	Visibility  string              `json:"visibility"  binding:"required,oneof=PRIVATE PROJECT"`
	DraftGraph  WorkflowCanvasGraph `json:"draft_graph"`
}
type WorkflowCanvasUpdateRequest struct {
	ID                    string               `json:"-"`
	ExpectedDraftRevision int64                `json:"expected_draft_revision" binding:"required,min=1"`
	Name                  *string              `json:"name"                    binding:"omitempty,min=1,max=200"`
	Description           *string              `json:"description"             binding:"omitempty,max=2000"`
	Visibility            *string              `json:"visibility"              binding:"omitempty,oneof=PRIVATE PROJECT"`
	DraftGraph            *WorkflowCanvasGraph `json:"draft_graph"`
}
type WorkflowCanvasPublishRequest struct {
	ExpectedDraftRevision int64 `json:"expected_draft_revision" binding:"required,min=1"`
	// DraftRevision is retained for source compatibility and is not serialized.
	DraftRevision int64 `json:"-"`
}
type WorkflowCanvasValidateRequest struct {
	ExpectedDraftRevision int64 `json:"expected_draft_revision" binding:"required,min=1"`
}
type WorkflowValidationIssue struct {
	Code     string         `json:"code"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	NodeID   *string        `json:"node_id,omitempty"`
	EdgeID   *string        `json:"edge_id,omitempty"`
	PortKey  *string        `json:"port_key,omitempty"`
	FlowID   *string        `json:"flow_id,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}
type WorkflowValidationResult struct {
	Valid         bool                      `json:"valid"`
	Issues        []WorkflowValidationIssue `json:"issues"`
	NodeCount     int                       `json:"node_count"`
	EdgeCount     int                       `json:"edge_count"`
	ContentDigest *string                   `json:"content_digest,omitempty"`
}
type CanvasVersionListRequest struct {
	imachinery.BasicQueryParam
	CanvasID string `form:"-"`
}
type WorkflowCanvasRunListRequest struct {
	imachinery.BasicQueryParam
	CanvasID           string `form:"canvas_id"`
	CanvasVersionID    string `form:"canvas_version_id"`
	Status             string `form:"status"                 binding:"omitempty,oneof=PENDING RUNNING SUCCESS FAILED CANCELED TIMEOUT"`
	RetryOfCanvasRunID string `form:"retry_of_canvas_run_id"`
	ProjectID          string `form:"project_id"`
	Namespace          string `form:"namespace"`
}
type WorkflowCanvasRunCreateRequest struct {
	CanvasVersionID string            `json:"canvas_version_id" binding:"required,max=64"`
	IdempotencyKey  string            `json:"idempotency_key"   binding:"required,max=200"`
	Scope           WorkflowRunScope  `json:"scope"             binding:"required"`
	RunPolicy       WorkflowRunPolicy `json:"run_policy"        binding:"required"`
	RuntimeInputs   map[string]any    `json:"runtime_inputs"    binding:"required"`
	// Input is retained for source compatibility and is not serialized.
	Input map[string]any `json:"-"`
}
type WorkflowCanvasRunValidateRequest struct {
	Scope         WorkflowRunScope  `json:"scope"          binding:"required"`
	RunPolicy     WorkflowRunPolicy `json:"run_policy"     binding:"required"`
	RuntimeInputs map[string]any    `json:"runtime_inputs" binding:"required"`
}
type WorkflowCanvasRunValidationResult struct {
	WorkflowValidationResult
	SelectedNodeCount   int     `json:"selected_node_count"`
	ExecutableTaskCount int     `json:"executable_task_count"`
	ReusedNodeCount     int     `json:"reused_node_count"`
	FlowCount           int     `json:"flow_count"`
	ExecutionPlanDigest *string `json:"execution_plan_digest,omitempty"`
}
type WorkflowCanvasRunCancelRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=1000"`
}
type WorkflowCanvasRunRetryRequest struct {
	IdempotencyKey string   `json:"idempotency_key" binding:"required,max=200"`
	Intent         string   `json:"intent"          binding:"required,oneof=retry_failed retry_node retry_from_node retry_flow rerun_all"`
	NodeIDs        []string `json:"node_ids"        binding:"omitempty,max=1000,dive,max=200"`
	FlowIDs        []string `json:"flow_ids"        binding:"omitempty,max=1000,dive,max=200"`
	ReusePolicy    string   `json:"reuse_policy"    binding:"omitempty,oneof=rerun_all reuse_valid_outputs reuse_required"`
}
type CanvasFlowRunListRequest struct {
	imachinery.BasicQueryParam
	CanvasRunID string `form:"-"`
	Status      string `form:"status" binding:"omitempty,oneof=PENDING RUNNING SUCCESS PARTIAL_SUCCESS FAILED CANCELED TIMEOUT"`
}
type CanvasNodeRunListRequest struct {
	imachinery.BasicQueryParam
	CanvasRunID string `form:"-"`
	Status      string `form:"status"  binding:"omitempty,oneof=PENDING BLOCKED READY RUNNING RETRYING SUCCESS PARTIAL_SUCCESS FAILED CANCELED TIMEOUT SKIPPED REUSED"`
	FlowID      string `form:"flow_id"`
	NodeID      string `form:"node_id"`
}

type WorkflowCanvasListResponse struct {
	Total int64                     `json:"total"`
	Items []*WorkflowCanvasListItem `json:"items"`
}
type CanvasVersionListResponse struct {
	Total int64                    `json:"total"`
	Items []*CanvasVersionListItem `json:"items"`
}
type WorkflowCanvasRunListResponse struct {
	Total int64                        `json:"total"`
	Items []*WorkflowCanvasRunListItem `json:"items"`
}
type CanvasNodeRunListResponse struct {
	Total int64            `json:"total"`
	Items []*CanvasNodeRun `json:"items"`
}
type WorkflowNodeDefinitionListResponse struct {
	Total int64                             `json:"total"`
	Items []*WorkflowNodeDefinitionListItem `json:"items"`
}
type CanvasFlowRunListResponse struct {
	Total int64            `json:"total"`
	Items []*CanvasFlowRun `json:"items"`
}
