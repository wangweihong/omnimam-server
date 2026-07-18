package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

type WorkflowCanvasListRequest struct{ imachinery.BasicQueryParam }
type WorkflowCanvasCreateRequest struct {
	Name        string              `json:"name" binding:"required,max=200"`
	Description string              `json:"description" binding:"omitempty,max=2000"`
	Visibility  string              `json:"visibility" binding:"required,oneof=PRIVATE PROJECT"`
	DraftGraph  WorkflowCanvasGraph `json:"draft_graph"`
}
type WorkflowCanvasUpdateRequest struct {
	ID                    string               `json:"-"`
	ExpectedDraftRevision int64                `json:"expected_draft_revision" binding:"required,min=1"`
	Name                  *string              `json:"name" binding:"omitempty,min=1,max=200"`
	Description           *string              `json:"description" binding:"omitempty,max=2000"`
	Visibility            *string              `json:"visibility" binding:"omitempty,oneof=PRIVATE PROJECT"`
	DraftGraph            *WorkflowCanvasGraph `json:"draft_graph"`
}
type WorkflowCanvasPublishRequest struct {
	DraftRevision int64 `json:"draft_revision" binding:"required,min=1"`
}
type CanvasVersionListRequest struct {
	imachinery.BasicQueryParam
	CanvasID string `form:"-"`
}
type WorkflowCanvasRunListRequest struct {
	imachinery.BasicQueryParam
	CanvasID        string `form:"canvas_id"`
	CanvasVersionID string `form:"canvas_version_id"`
	Status          string `form:"status" binding:"omitempty,oneof=PENDING RUNNING SUCCESS FAILED CANCELED TIMEOUT"`
}
type WorkflowCanvasRunCreateRequest struct {
	CanvasVersionID string         `json:"canvas_version_id" binding:"required,max=64"`
	IdempotencyKey  string         `json:"idempotency_key" binding:"required,max=200"`
	Input           map[string]any `json:"input" binding:"required"`
}
type WorkflowCanvasRunRetryRequest struct {
	IdempotencyKey string `json:"idempotency_key" binding:"required,max=200"`
}
type CanvasNodeRunListRequest struct {
	imachinery.BasicQueryParam
	CanvasRunID string `form:"-"`
	Status      string `form:"status"`
}

type WorkflowCanvasListResponse struct {
	Total int64             `json:"total"`
	Items []*WorkflowCanvas `json:"items"`
}
type CanvasVersionListResponse struct {
	Total int64            `json:"total"`
	Items []*CanvasVersion `json:"items"`
}
type WorkflowCanvasRunListResponse struct {
	Total int64                `json:"total"`
	Items []*WorkflowCanvasRun `json:"items"`
}
type CanvasNodeRunListResponse struct {
	Total int64            `json:"total"`
	Items []*CanvasNodeRun `json:"items"`
}
