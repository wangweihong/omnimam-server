package iapiserver

import (
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"gorm.io/gorm"
)

const (
	ComfyUIWorkflowSourceVisual = "visual_workflow"
	ComfyUIWorkflowSourceAPI    = "api_workflow"
	ComfyUIAPIConversionPending = "pending"
	ComfyUIAPIConversionReady   = "ready"

	ComfyUIParseFullySupported              = "fully_supported"
	ComfyUIParsePartiallySupported          = "partially_supported"
	ComfyUIParseManualConfigurationRequired = "manual_configuration_required"
	ComfyUIParseUnsupported                 = "unsupported"

	ComfyUIValidationCompatible   = "compatible"
	ComfyUIValidationIncompatible = "incompatible"
	ComfyUIValidationFailed       = "failed"
)

// ComfyUIWorkflow 保存用户导入的不可变执行源；解析结果按目标实例当前目录即时计算。
type ComfyUIWorkflow struct {
	imachinery.ObjectMeta
	// OwnerUserID 是唯一资源所有者，普通用户查询必须以此字段隔离。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// CreatedByUserID 和 UpdatedByUserID 记录实际操作者，支持管理员代管审计。
	CreatedByUserID string `json:"-" gorm:"column:created_by_user_id;type:text;not null"`
	UpdatedByUserID string `json:"-" gorm:"column:updated_by_user_id;type:text;not null"`
	// SourceEngineInstanceID 固定导入时读取 object_info 的 ComfyUI 实例。
	SourceEngineInstanceID string  `json:"source_engine_instance_id" gorm:"column:source_engine_instance_id;type:text;not null;index"`
	SourceType             string  `json:"source_type" gorm:"column:source_type;type:text;not null;default:'api_workflow';index"`
	APIConversionStatus    string  `json:"api_conversion_status" gorm:"column:api_conversion_status;type:text;not null;default:'ready';index"`
	SourceChecksum         string  `json:"source_checksum" gorm:"column:source_checksum;type:text;not null;default:'';index"`
	APIWorkflowChecksum    *string `json:"api_workflow_checksum" gorm:"column:api_workflow_checksum;type:text;index"`
	// APIWorkflow 是执行事实；VisualWorkflow 只保存可选展示信息，二者导入后不可修改。
	APIWorkflow          map[string]any `json:"-" gorm:"-"`
	APIWorkflowShadow    *string        `json:"-" gorm:"column:api_workflow_json;type:text"`
	VisualWorkflow       map[string]any `json:"-" gorm:"-"`
	VisualWorkflowShadow *string        `json:"-" gorm:"column:visual_workflow_json;type:text"`
	// 以下字段仅承载单次即时解析结果，禁止映射为数据库列或 API 工作流详情字段。
	ParseStatus      string                           `json:"-" gorm:"-"`
	ParseSummary     ComfyUIWorkflowParseSummary      `json:"-" gorm:"-"`
	ParsedNodes      []ComfyUIWorkflowNode            `json:"-" gorm:"-"`
	InputCandidates  []ComfyUIWorkflowInputCandidate  `json:"-" gorm:"-"`
	OutputCandidates []ComfyUIWorkflowOutputCandidate `json:"-" gorm:"-"`
	Dependencies     []ComfyUIWorkflowDependency      `json:"-" gorm:"-"`
	// Converted* 字段原子固定一次性转换结果、幂等键、时间和操作者。
	ConvertedApplicationTemplateID *string          `json:"converted_application_template_id" gorm:"column:converted_application_template_id;type:text"`
	ConvertedTemplateVersionID     *string          `json:"converted_template_version_id" gorm:"column:converted_template_version_id;type:text"`
	ConversionIdempotencyKey       *string          `json:"-" gorm:"column:conversion_idempotency_key;type:text"`
	ConvertedAt                    *imachinery.Time `json:"converted_at" gorm:"column:converted_at;type:timestamptz"`
	ConvertedByUserID              *string          `json:"-" gorm:"column:converted_by_user_id;type:text"`
}

func (ComfyUIWorkflow) TableName() string { return "aiapp_comfyui_workflows" }
func (w *ComfyUIWorkflow) BeforeCreate(tx *gorm.DB) error {
	if err := w.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return w.marshal()
}
func (*ComfyUIWorkflow) AfterCreate(*gorm.DB) error { return nil }
func (w *ComfyUIWorkflow) BeforeUpdate(tx *gorm.DB) error {
	if err := w.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return w.marshal()
}
func (*ComfyUIWorkflow) AfterUpdate(*gorm.DB) error { return nil }
func (w *ComfyUIWorkflow) AfterFind(*gorm.DB) error {
	unmarshalOptional(w.APIWorkflowShadow, &w.APIWorkflow)
	unmarshalOptional(w.VisualWorkflowShadow, &w.VisualWorkflow)
	return nil
}
func (w *ComfyUIWorkflow) marshal() error {
	if err := marshalOptional(w.APIWorkflow, &w.APIWorkflowShadow); err != nil {
		return err
	}
	return marshalOptional(w.VisualWorkflow, &w.VisualWorkflowShadow)
}
func (w *ComfyUIWorkflow) Converted() bool { return w.ConvertedApplicationTemplateID != nil }

// ComfyUIWorkflowSummary 是列表和资源变更接口返回的安全摘要。
type ComfyUIWorkflowSummary struct {
	ID                             string           `json:"id"`
	Name                           string           `json:"name"`
	Description                    string           `json:"description"`
	OwnerUserID                    string           `json:"owner_user_id"`
	SourceEngineInstanceID         string           `json:"source_engine_instance_id"`
	SourceType                     string           `json:"source_type"`
	APIConversionStatus            string           `json:"api_conversion_status"`
	SourceChecksum                 string           `json:"source_checksum"`
	APIWorkflowChecksum            *string          `json:"api_workflow_checksum"`
	Converted                      bool             `json:"converted"`
	ConvertedApplicationTemplateID *string          `json:"converted_application_template_id"`
	ConvertedTemplateVersionID     *string          `json:"converted_template_version_id"`
	ConvertedAt                    *imachinery.Time `json:"converted_at"`
	CreatedAt                      imachinery.Time  `json:"created_at"`
	UpdatedAt                      imachinery.Time  `json:"updated_at"`
	ResourceVersion                int64            `json:"resource_version"`
}

func (w *ComfyUIWorkflow) Summary() *ComfyUIWorkflowSummary {
	return &ComfyUIWorkflowSummary{ID: w.ID, Name: w.Name, Description: w.Description, OwnerUserID: w.OwnerUserID, SourceEngineInstanceID: w.SourceEngineInstanceID, SourceType: w.SourceType, APIConversionStatus: w.APIConversionStatus, SourceChecksum: w.SourceChecksum, APIWorkflowChecksum: w.APIWorkflowChecksum, Converted: w.Converted(), ConvertedApplicationTemplateID: w.ConvertedApplicationTemplateID, ConvertedTemplateVersionID: w.ConvertedTemplateVersionID, ConvertedAt: w.ConvertedAt, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt, ResourceVersion: w.ResourceVersion}
}

type ComfyUIWorkflowDetail struct {
	*ComfyUIWorkflowSummary
	APIWorkflow    map[string]any `json:"api_workflow"`
	VisualWorkflow map[string]any `json:"visual_workflow"`
}

func (w *ComfyUIWorkflow) Detail() *ComfyUIWorkflowDetail {
	apiWorkflow := w.APIWorkflow
	if w.APIConversionStatus != ComfyUIAPIConversionReady {
		apiWorkflow = nil
	}
	return &ComfyUIWorkflowDetail{ComfyUIWorkflowSummary: w.Summary(), APIWorkflow: apiWorkflow, VisualWorkflow: w.VisualWorkflow}
}

type ComfyUIWorkflowParseSummary struct {
	TotalNodes       int `json:"total_nodes"`
	SupportedNodes   int `json:"supported_nodes"`
	WarningNodes     int `json:"warning_nodes"`
	UnsupportedNodes int `json:"unsupported_nodes"`
}
type ComfyUIWorkflowDiagnostic struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	NodeID    *string        `json:"node_id"`
	FieldName *string        `json:"field_name"`
	Detail    map[string]any `json:"detail,omitempty"`
}
type ComfyUIWorkflowNode struct {
	NodeID      string                      `json:"node_id"`
	ClassType   string                      `json:"class_type"`
	Title       string                      `json:"title,omitempty"`
	DisplayName string                      `json:"display_name,omitempty"`
	Position    map[string]any              `json:"position,omitempty"`
	Inputs      []map[string]any            `json:"inputs"`
	Outputs     []map[string]any            `json:"outputs"`
	ParseStatus string                      `json:"parse_status"`
	Errors      []ComfyUIWorkflowDiagnostic `json:"errors,omitempty"`
	Warnings    []ComfyUIWorkflowDiagnostic `json:"warnings,omitempty"`
}
type ComfyUIWorkflowInputCandidate struct {
	NodeID            string   `json:"node_id"`
	InputName         string   `json:"input_name"`
	Classification    string   `json:"classification"`
	DataType          string   `json:"data_type"`
	CurrentValue      any      `json:"current_value"`
	Required          bool     `json:"required,omitempty"`
	Options           []any    `json:"options,omitempty"`
	Minimum           *float64 `json:"minimum,omitempty"`
	Maximum           *float64 `json:"maximum,omitempty"`
	Step              *float64 `json:"step,omitempty"`
	SourceNodeID      string   `json:"source_node_id,omitempty"`
	SourceOutputIndex *int     `json:"source_output_index,omitempty"`
}
type ComfyUIWorkflowOutputCandidate struct {
	NodeID      string `json:"node_id"`
	OutputIndex int    `json:"output_index"`
	OutputName  string `json:"output_name,omitempty"`
	DataType    string `json:"data_type"`
	Extractable bool   `json:"extractable"`
	MediaType   string `json:"media_type,omitempty"`
}
type ComfyUIWorkflowDependency struct {
	DependencyType string   `json:"dependency_type"`
	Identifier     string   `json:"identifier"`
	DisplayName    string   `json:"display_name,omitempty"`
	Required       bool     `json:"required"`
	SourceNodeIDs  []string `json:"source_node_ids,omitempty"`
}

// ComfyUIWorkflowValidation 保存一次不含 object_info 正文的不可变兼容性结果。
type ComfyUIWorkflowValidation struct {
	imachinery.ObjectMeta
	// WorkflowID、OwnerUserID 和 RequestedByUserID 同时记录目标资源、所有者与实际操作者。
	WorkflowID        string `json:"workflow_id" gorm:"column:workflow_id;type:text;not null;index"`
	OwnerUserID       string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null"`
	RequestedByUserID string `json:"-" gorm:"column:requested_by_user_id;type:text;not null"`
	// EngineInstanceID 固定本次校验的目标 ComfyUI 实例。
	EngineInstanceID string `json:"engine_instance_id" gorm:"column:engine_instance_id;type:text;not null;index"`
	// Status 是 compatible、incompatible 或读取失败后的 failed 终态。
	Status         string `json:"status" gorm:"column:status;type:text;not null;index"`
	ComfyUIVersion string `json:"comfyui_version,omitempty" gorm:"column:comfyui_version;type:text;default:''"`
	// NodeSummary、DependencySummary、Errors 和 Warnings 保存稳定诊断结果。
	NodeSummary             map[string]any              `json:"node_summary" gorm:"-"`
	NodeSummaryShadow       string                      `json:"-" gorm:"column:node_summary_json;type:text;not null"`
	DependencySummary       map[string]any              `json:"dependency_summary" gorm:"-"`
	DependencySummaryShadow string                      `json:"-" gorm:"column:dependency_summary_json;type:text;not null"`
	Errors                  []ComfyUIWorkflowDiagnostic `json:"errors" gorm:"-"`
	ErrorsShadow            string                      `json:"-" gorm:"column:errors_json;type:text;not null;default:'[]'"`
	Warnings                []ComfyUIWorkflowDiagnostic `json:"warnings" gorm:"-"`
	WarningsShadow          string                      `json:"-" gorm:"column:warnings_json;type:text;not null;default:'[]'"`
	// ValidatedAt 是目标能力快照完成或读取失败的时间。
	ValidatedAt imachinery.Time `json:"validated_at" gorm:"column:validated_at;type:timestamptz;not null"`
}

type ComfyUIWorkflowTestStep struct {
	Key           string  `json:"key"`
	Label         string  `json:"label"`
	AtomicTaskID  *string `json:"atomic_task_id"`
	Status        string  `json:"status"`
	Progress      int     `json:"progress"`
	ExternalJobID *string `json:"external_job_id"`
	ProviderState *string `json:"provider_state"`
	QueuePosition *int    `json:"queue_position"`
	Error         *string `json:"error"`
}
type ComfyUIWorkflowTestOutput struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	NodeID      string  `json:"node_id"`
	Filename    *string `json:"filename"`
	Subfolder   *string `json:"-"`
	StorageType *string `json:"-"`
	MimeType    *string `json:"mime_type"`
	Text        *string `json:"text"`
	ContentURL  *string `json:"content_url" gorm:"-"`
}

// ComfyUIWorkflowTestEngineSnapshot 保存试运行创建时可公开的实例身份，不包含地址或鉴权信息。
type ComfyUIWorkflowTestEngineSnapshot struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Region *string `json:"region"`
}
type ComfyUIWorkflowTestRun struct {
	imachinery.ObjectMeta
	WorkflowID             string                               `json:"workflow_id" gorm:"column:workflow_id;type:text;not null;index"`
	OwnerUserID            string                               `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index;uniqueIndex:uk_comfy_test_owner_key"`
	RequestedByUserID      string                               `json:"-" gorm:"column:requested_by_user_id;type:text;not null"`
	EngineInstanceID       string                               `json:"engine_instance_id" gorm:"column:engine_instance_id;type:text;not null;index"`
	EngineInstanceSnapshot ComfyUIWorkflowTestEngineSnapshot    `json:"engine_instance_snapshot" gorm:"-"`
	EngineSnapshotShadow   string                               `json:"-" gorm:"column:engine_instance_snapshot_json;type:text;not null"`
	ParameterOverrideCount int                                  `json:"parameter_override_count" gorm:"-"`
	WorkflowValidationID   string                               `json:"workflow_validation_id" gorm:"column:workflow_validation_id;type:text;not null"`
	DAGTaskGroupID         *string                              `json:"dag_task_group_id" gorm:"column:dag_task_group_id;type:text;index"`
	ExternalJobID          *string                              `json:"external_job_id" gorm:"column:external_job_id;type:text;index"`
	IdempotencyKey         string                               `json:"idempotency_key" gorm:"column:idempotency_key;type:text;not null;uniqueIndex:uk_comfy_test_owner_key"`
	TaskCreationStatus     string                               `json:"task_creation_status" gorm:"column:task_creation_status;type:text;not null;default:'pending'"`
	TaskCreationFailure    *string                              `json:"task_creation_failure" gorm:"column:task_creation_failure;type:text"`
	WorkflowSnapshot       map[string]any                       `json:"-" gorm:"-"`
	WorkflowSnapshotShadow string                               `json:"-" gorm:"column:workflow_snapshot_json;type:text;not null"`
	Parameters             []ComfyUIWorkflowTestParameter       `json:"parameter_snapshot" gorm:"-"`
	ParametersShadow       string                               `json:"-" gorm:"column:parameter_snapshot_json;type:text;not null;default:'[]'"`
	OutputSelections       []ComfyUIWorkflowTestOutputSelection `json:"output_snapshot" gorm:"-"`
	OutputSelectionsShadow string                               `json:"-" gorm:"column:output_snapshot_json;type:text;not null;default:'[]'"`
	Steps                  []ComfyUIWorkflowTestStep            `json:"steps" gorm:"-"`
	StepsShadow            string                               `json:"-" gorm:"column:steps_json;type:text;not null;default:'[]'"`
	Outputs                []ComfyUIWorkflowTestOutput          `json:"outputs" gorm:"-"`
	OutputsShadow          string                               `json:"-" gorm:"column:outputs_json;type:text;not null;default:'[]'"`
	Status                 string                               `json:"status" gorm:"column:status;type:text;not null;default:'PENDING';index"`
	Progress               int                                  `json:"progress" gorm:"column:progress;not null;default:0"`
	CurrentStep            *string                              `json:"current_step" gorm:"column:current_step;type:text"`
	FailureSummary         *string                              `json:"failure_summary" gorm:"column:failure_summary;type:text"`
}

func (ComfyUIWorkflowTestRun) TableName() string { return "aiapp_comfyui_workflow_test_runs" }
func (r *ComfyUIWorkflowTestRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (*ComfyUIWorkflowTestRun) AfterCreate(*gorm.DB) error { return nil }
func (r *ComfyUIWorkflowTestRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (*ComfyUIWorkflowTestRun) AfterUpdate(*gorm.DB) error { return nil }
func (r *ComfyUIWorkflowTestRun) AfterFind(*gorm.DB) error {
	unmarshalShadow(r.WorkflowSnapshotShadow, &r.WorkflowSnapshot)
	unmarshalShadow(r.EngineSnapshotShadow, &r.EngineInstanceSnapshot)
	unmarshalShadow(r.ParametersShadow, &r.Parameters)
	r.ParameterOverrideCount = len(r.Parameters)
	unmarshalShadow(r.OutputSelectionsShadow, &r.OutputSelections)
	unmarshalShadow(r.StepsShadow, &r.Steps)
	var storedOutputs []storedComfyUIWorkflowTestOutput
	unmarshalShadow(r.OutputsShadow, &storedOutputs)
	r.Outputs = make([]ComfyUIWorkflowTestOutput, 0, len(storedOutputs))
	for _, output := range storedOutputs {
		r.Outputs = append(r.Outputs, output.public())
	}
	return nil
}
func (r *ComfyUIWorkflowTestRun) marshal() error {
	r.ParameterOverrideCount = len(r.Parameters)
	storedOutputs := make([]storedComfyUIWorkflowTestOutput, 0, len(r.Outputs))
	for _, output := range r.Outputs {
		storedOutputs = append(storedOutputs, newStoredComfyUIWorkflowTestOutput(output))
	}
	for _, item := range []struct {
		value    any
		target   *string
		fallback string
	}{{r.WorkflowSnapshot, &r.WorkflowSnapshotShadow, "{}"}, {r.EngineInstanceSnapshot, &r.EngineSnapshotShadow, "{}"}, {r.Parameters, &r.ParametersShadow, "[]"}, {r.OutputSelections, &r.OutputSelectionsShadow, "[]"}, {r.Steps, &r.StepsShadow, "[]"}, {storedOutputs, &r.OutputsShadow, "[]"}} {
		if err := marshalShadow(item.value, item.target, item.fallback); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComfyUIWorkflowTestRun) MarshalShadows() error { return r.marshal() }

type storedComfyUIWorkflowTestOutput struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	NodeID      string  `json:"node_id"`
	Filename    *string `json:"filename"`
	Subfolder   *string `json:"subfolder"`
	StorageType *string `json:"storage_type"`
	MimeType    *string `json:"mime_type"`
	Text        *string `json:"text"`
}

func newStoredComfyUIWorkflowTestOutput(output ComfyUIWorkflowTestOutput) storedComfyUIWorkflowTestOutput {
	return storedComfyUIWorkflowTestOutput{ID: output.ID, Kind: output.Kind, NodeID: output.NodeID, Filename: output.Filename, Subfolder: output.Subfolder, StorageType: output.StorageType, MimeType: output.MimeType, Text: output.Text}
}

func (output storedComfyUIWorkflowTestOutput) public() ComfyUIWorkflowTestOutput {
	return ComfyUIWorkflowTestOutput{ID: output.ID, Kind: output.Kind, NodeID: output.NodeID, Filename: output.Filename, Subfolder: output.Subfolder, StorageType: output.StorageType, MimeType: output.MimeType, Text: output.Text}
}

func (ComfyUIWorkflowValidation) TableName() string { return "aiapp_comfyui_workflow_validations" }
func (v *ComfyUIWorkflowValidation) BeforeCreate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*ComfyUIWorkflowValidation) AfterCreate(*gorm.DB) error { return nil }
func (v *ComfyUIWorkflowValidation) BeforeUpdate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*ComfyUIWorkflowValidation) AfterUpdate(*gorm.DB) error { return nil }
func (v *ComfyUIWorkflowValidation) AfterFind(*gorm.DB) error {
	unmarshalShadow(v.NodeSummaryShadow, &v.NodeSummary)
	unmarshalShadow(v.DependencySummaryShadow, &v.DependencySummary)
	unmarshalShadow(v.ErrorsShadow, &v.Errors)
	unmarshalShadow(v.WarningsShadow, &v.Warnings)
	return nil
}
func (v *ComfyUIWorkflowValidation) marshal() error {
	values := []struct {
		value    any
		target   *string
		fallback string
	}{{v.NodeSummary, &v.NodeSummaryShadow, "{}"}, {v.DependencySummary, &v.DependencySummaryShadow, "{}"}, {v.Errors, &v.ErrorsShadow, "[]"}, {v.Warnings, &v.WarningsShadow, "[]"}}
	for _, item := range values {
		if err := marshalShadow(item.value, item.target, item.fallback); err != nil {
			return err
		}
	}
	return nil
}

type ComfyUIWorkflowListResponse struct {
	Total int64                     `json:"total"`
	Items []*ComfyUIWorkflowSummary `json:"items"`
}
type ComfyUIWorkflowImportResult struct {
	Workflow             *ComfyUIWorkflowSummary `json:"workflow"`
	DuplicateContent     bool                    `json:"duplicate_content"`
	DuplicateWorkflowIDs []string                `json:"duplicate_workflow_ids"`
}
type ComfyUIWorkflowNodeListResponse struct {
	Total int                   `json:"total"`
	Items []ComfyUIWorkflowNode `json:"items"`
}
type ComfyUIWorkflowInputCandidateListResponse struct {
	Total int                             `json:"total"`
	Items []ComfyUIWorkflowInputCandidate `json:"items"`
}
type ComfyUIWorkflowOutputCandidateListResponse struct {
	Total int                              `json:"total"`
	Items []ComfyUIWorkflowOutputCandidate `json:"items"`
}
type ComfyUIWorkflowDependencyListResponse struct {
	Total int                         `json:"total"`
	Items []ComfyUIWorkflowDependency `json:"items"`
}
type ComfyUIWorkflowValidationListResponse struct {
	Total int64                        `json:"total"`
	Items []*ComfyUIWorkflowValidation `json:"items"`
}
type ComfyUIWorkflowTestRunListResponse struct {
	Total int64                     `json:"total"`
	Items []*ComfyUIWorkflowTestRun `json:"items"`
}
type ComfyUIWorkflowConvertResult struct {
	WorkflowID                 string                      `json:"workflow_id"`
	ApplicationTemplate        *ApplicationTemplate        `json:"application_template"`
	ApplicationTemplateVersion *ApplicationTemplateVersion `json:"application_template_version"`
	WorkflowContractRevision   string                      `json:"workflow_contract_revision"`
}
