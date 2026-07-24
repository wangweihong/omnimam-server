package iapiserver

import (
	"errors"
	"mime/multipart"
	"unicode/utf8"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type ComfyUIWorkflowListRequest struct {
	imachinery.BasicQueryParam
	// Converted 用于筛选是否已经转换为应用模板。
	Converted *bool `form:"converted"`
	// OwnerUserID 仅管理员代管查询可用，普通用户会由服务端覆盖为本人。
	OwnerUserID string `form:"owner_user_id"`
}

// ComfyUIWorkflowDeriveRequest 指定即时解析使用的目标实例；分页仅用于节点列表。
type ComfyUIWorkflowDeriveRequest struct {
	imachinery.BasicQueryParam
	EngineInstanceID string `form:"engine_instance_id" binding:"required"`
}

// ComfyUIWorkflowImportRequest 是不依赖 EngineInstance 或 object_info 的 multipart 导入请求。
type ComfyUIWorkflowImportRequest struct {
	// Name 和 Description 是导入后唯一允许修改的业务元数据。
	Name         string                `form:"name" binding:"required,max=255"`
	Description  string                `form:"description"`
	WorkflowFile *multipart.FileHeader `form:"workflow_file"`
	// APIWorkflowFile 是必填执行文件；VisualWorkflowFile 仅提供展示位置和标题。
	APIWorkflowFile    *multipart.FileHeader `form:"api_workflow_file"`
	VisualWorkflowFile *multipart.FileHeader `form:"visual_workflow_file"`
	// APIWorkflow、原始字节和 VisualWorkflow 是 Controller 安全解析后的内部传递字段。
	APIWorkflow       map[string]any `json:"-"`
	APIWorkflowRaw    []byte         `json:"-"`
	VisualWorkflow    map[string]any `json:"-"`
	SourceWorkflow    map[string]any `json:"-"`
	SourceWorkflowRaw []byte         `json:"-"`
}

func (r *ComfyUIWorkflowImportRequest) Validate() error {
	if r.Name == "" || utf8.RuneCountInString(r.Name) > 255 {
		return errors.New("name is required and must not exceed 255 characters")
	}
	if r.WorkflowFile == nil && r.APIWorkflowFile == nil && r.SourceWorkflow == nil && r.APIWorkflow == nil {
		return errors.New("workflow_file or api_workflow_file is required")
	}
	return nil
}

type ComfyUIWorkflowUpdateRequest struct {
	// ID 来自 path，不接受客户端 JSON 覆盖。
	ID string `json:"-"`
	// Name 和 Description 至少提交一个，不能修改执行事实或来源实例。
	Name        *string `json:"name" binding:"omitempty,max=255"`
	Description *string `json:"description"`
	// ResourceVersion 用于元数据乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"required"`
}

func (r *ComfyUIWorkflowUpdateRequest) Validate() error {
	if r.Name == nil && r.Description == nil {
		return errors.New("name or description is required")
	}
	return nil
}

// ComfyUIWorkflowAPIConversionRequest 指定 Visual Workflow 转换使用的当前 ComfyUI 实例。
type ComfyUIWorkflowAPIConversionRequest struct {
	// EngineInstanceID 必须引用 enabled、online 且 object_info 未过期的 ComfyUI 实例。
	EngineInstanceID string `json:"engine_instance_id" binding:"required"`
	// ResourceVersion 对工作流状态变更执行乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"required"`
}
type ComfyUIWorkflowValidationListRequest struct {
	imachinery.BasicQueryParam
	WorkflowID string `form:"-"`
}
type ComfyUIWorkflowValidationCreateRequest struct {
	EngineInstanceID string `json:"engine_instance_id" binding:"required"`
}
type ComfyUIWorkflowTestRunListRequest struct {
	imachinery.BasicQueryParam
	WorkflowID  string `form:"-"`
	OwnerUserID string `form:"-"`
	// Detail 控制列表是否返回参数快照、任务步骤和输出；默认 false 返回轻量投影。
	Detail bool `json:"detail" form:"detail"`
}
type ComfyUIWorkflowTestParameter struct {
	NodeID    string `json:"node_id" binding:"required"`
	InputName string `json:"input_name" binding:"required"`
	Value     any    `json:"value"`
}

// ComfyUIWorkflowTestOutputSelection 标识试运行需要收集临时预览的工作流输出候选。
type ComfyUIWorkflowTestOutputSelection struct {
	NodeID      string `json:"node_id" binding:"required"`
	OutputIndex int    `json:"output_index" binding:"min=0"`
}

type ComfyUIWorkflowTestRunCreateRequest struct {
	EngineInstanceID string                               `json:"engine_instance_id" binding:"required"`
	Parameters       []ComfyUIWorkflowTestParameter       `json:"parameters" binding:"max=256,dive"`
	Outputs          []ComfyUIWorkflowTestOutputSelection `json:"outputs" binding:"required,min=1,max=256,dive"`
	IdempotencyKey   string                               `json:"idempotency_key" binding:"required,max=256"`
}
type ComfyUIWorkflowConvertRequest struct {
	Name                   string         `json:"name" binding:"required"`
	Description            string         `json:"description"`
	CapabilityDefinitionID string         `json:"capability_definition_id" binding:"required"`
	WorkflowValidationID   string         `json:"workflow_validation_id" binding:"required"`
	TemplateContract       map[string]any `json:"template_contract" binding:"required"`
	IdempotencyKey         string         `json:"idempotency_key" binding:"required"`
}
