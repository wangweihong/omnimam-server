package iapiserver

import (
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"gorm.io/gorm"
)

const (
	ComfyUIWorkflowActive   = "active"
	ComfyUIWorkflowArchived = "archived"

	ComfyUIParseFullySupported              = "fully_supported"
	ComfyUIParsePartiallySupported          = "partially_supported"
	ComfyUIParseManualConfigurationRequired = "manual_configuration_required"
	ComfyUIParseUnsupported                 = "unsupported"

	ComfyUIValidationNotValidated = "not_validated"
	ComfyUIValidationCompatible   = "compatible"
	ComfyUIValidationIncompatible = "incompatible"
	ComfyUIValidationFailed       = "failed"
)

// ComfyUIWorkflow 保存用户导入的不可变执行源和服务端派生解析结果。
type ComfyUIWorkflow struct {
	imachinery.ObjectMeta
	// OwnerUserID 是唯一资源所有者，普通用户查询必须以此字段隔离。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// CreatedByUserID 和 UpdatedByUserID 记录实际操作者，支持管理员代管审计。
	CreatedByUserID string `json:"-" gorm:"column:created_by_user_id;type:text;not null"`
	UpdatedByUserID string `json:"-" gorm:"column:updated_by_user_id;type:text;not null"`
	// SourceEngineInstanceID 固定导入时读取 object_info 的 ComfyUI 实例。
	SourceEngineInstanceID string `json:"source_engine_instance_id" gorm:"column:source_engine_instance_id;type:text;not null;index"`
	// APIWorkflow 是执行事实；VisualWorkflow 只保存可选展示信息，二者导入后不可修改。
	APIWorkflow          map[string]any `json:"-" gorm:"-"`
	APIWorkflowShadow    string         `json:"-" gorm:"column:api_workflow_json;type:text;not null"`
	VisualWorkflow       map[string]any `json:"-" gorm:"-"`
	VisualWorkflowShadow *string        `json:"-" gorm:"column:visual_workflow_json;type:text"`
	// WorkflowChecksum 是 API Workflow 按 RFC 8785 规范化后的 SHA-256 摘要。
	WorkflowChecksum string `json:"workflow_checksum" gorm:"column:workflow_checksum;type:text;not null;index"`
	// ImportObjectInfo 是服务端从来源实例读取的能力快照，客户端不能提交。
	ImportObjectInfo         map[string]any `json:"-" gorm:"-"`
	ImportObjectInfoShadow   string         `json:"-" gorm:"column:import_object_info_json;type:text;not null"`
	ImportObjectInfoChecksum string         `json:"-" gorm:"column:import_object_info_checksum;type:text;not null"`
	// ParseStatus 和派生 JSON 列保存导入时的节点、候选项和依赖解析结果。
	ParseStatus            string                           `json:"parse_status" gorm:"column:parse_status;type:text;not null;index"`
	ParseSummary           ComfyUIWorkflowParseSummary      `json:"-" gorm:"-"`
	ParseSummaryShadow     string                           `json:"-" gorm:"column:parse_summary_json;type:text;not null"`
	ParsedNodes            []ComfyUIWorkflowNode            `json:"-" gorm:"-"`
	ParsedNodesShadow      string                           `json:"-" gorm:"column:parsed_nodes_json;type:text;not null"`
	InputCandidates        []ComfyUIWorkflowInputCandidate  `json:"-" gorm:"-"`
	InputCandidatesShadow  string                           `json:"-" gorm:"column:input_candidates_json;type:text;not null"`
	OutputCandidates       []ComfyUIWorkflowOutputCandidate `json:"-" gorm:"-"`
	OutputCandidatesShadow string                           `json:"-" gorm:"column:output_candidates_json;type:text;not null"`
	Dependencies           []ComfyUIWorkflowDependency      `json:"-" gorm:"-"`
	DependenciesShadow     string                           `json:"-" gorm:"column:dependencies_json;type:text;not null"`
	// LatestValidationStatus 是最近一次不可变兼容性校验的摘要状态。
	LatestValidationStatus string `json:"latest_validation_status" gorm:"column:latest_validation_status;type:text;not null;default:'not_validated';index"`
	// LifecycleStatus 控制归档和恢复；归档资源保留但不能校验或转换。
	LifecycleStatus  string           `json:"lifecycle_status" gorm:"column:lifecycle_status;type:text;not null;default:'active';index"`
	ArchivedAt       *imachinery.Time `json:"archived_at" gorm:"column:archived_at;type:timestamptz"`
	ArchivedByUserID *string          `json:"-" gorm:"column:archived_by_user_id;type:text"`
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
	unmarshalShadow(w.APIWorkflowShadow, &w.APIWorkflow)
	unmarshalOptional(w.VisualWorkflowShadow, &w.VisualWorkflow)
	unmarshalShadow(w.ImportObjectInfoShadow, &w.ImportObjectInfo)
	unmarshalShadow(w.ParseSummaryShadow, &w.ParseSummary)
	unmarshalShadow(w.ParsedNodesShadow, &w.ParsedNodes)
	unmarshalShadow(w.InputCandidatesShadow, &w.InputCandidates)
	unmarshalShadow(w.OutputCandidatesShadow, &w.OutputCandidates)
	unmarshalShadow(w.DependenciesShadow, &w.Dependencies)
	return nil
}
func (w *ComfyUIWorkflow) marshal() error {
	values := []struct {
		value    any
		target   *string
		fallback string
	}{
		{w.APIWorkflow, &w.APIWorkflowShadow, "{}"}, {w.ImportObjectInfo, &w.ImportObjectInfoShadow, "{}"},
		{w.ParseSummary, &w.ParseSummaryShadow, "{}"}, {w.ParsedNodes, &w.ParsedNodesShadow, "[]"},
		{w.InputCandidates, &w.InputCandidatesShadow, "[]"}, {w.OutputCandidates, &w.OutputCandidatesShadow, "[]"},
		{w.Dependencies, &w.DependenciesShadow, "[]"},
	}
	for _, item := range values {
		if err := marshalShadow(item.value, item.target, item.fallback); err != nil {
			return err
		}
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
	WorkflowChecksum               string           `json:"workflow_checksum"`
	LifecycleStatus                string           `json:"lifecycle_status"`
	ParseStatus                    string           `json:"parse_status"`
	LatestValidationStatus         string           `json:"latest_validation_status"`
	Converted                      bool             `json:"converted"`
	ConvertedApplicationTemplateID *string          `json:"converted_application_template_id"`
	ConvertedTemplateVersionID     *string          `json:"converted_template_version_id"`
	ConvertedAt                    *imachinery.Time `json:"converted_at"`
	ArchivedAt                     *imachinery.Time `json:"archived_at"`
	CreatedAt                      imachinery.Time  `json:"created_at"`
	UpdatedAt                      imachinery.Time  `json:"updated_at"`
	ResourceVersion                int64            `json:"resource_version"`
}

func (w *ComfyUIWorkflow) Summary() *ComfyUIWorkflowSummary {
	return &ComfyUIWorkflowSummary{ID: w.ID, Name: w.Name, Description: w.Description, OwnerUserID: w.OwnerUserID, SourceEngineInstanceID: w.SourceEngineInstanceID, WorkflowChecksum: w.WorkflowChecksum, LifecycleStatus: w.LifecycleStatus, ParseStatus: w.ParseStatus, LatestValidationStatus: w.LatestValidationStatus, Converted: w.Converted(), ConvertedApplicationTemplateID: w.ConvertedApplicationTemplateID, ConvertedTemplateVersionID: w.ConvertedTemplateVersionID, ConvertedAt: w.ConvertedAt, ArchivedAt: w.ArchivedAt, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt, ResourceVersion: w.ResourceVersion}
}

type ComfyUIWorkflowDetail struct {
	*ComfyUIWorkflowSummary
	APIWorkflow        map[string]any              `json:"api_workflow"`
	VisualWorkflow     map[string]any              `json:"visual_workflow"`
	ObjectInfoSnapshot map[string]any              `json:"object_info_snapshot"`
	ObjectInfoChecksum string                      `json:"object_info_checksum"`
	ParseSummary       ComfyUIWorkflowParseSummary `json:"parse_summary"`
	Dependencies       []ComfyUIWorkflowDependency `json:"dependencies"`
}

func (w *ComfyUIWorkflow) Detail() *ComfyUIWorkflowDetail {
	return &ComfyUIWorkflowDetail{ComfyUIWorkflowSummary: w.Summary(), APIWorkflow: w.APIWorkflow, VisualWorkflow: w.VisualWorkflow, ObjectInfoSnapshot: w.ImportObjectInfo, ObjectInfoChecksum: w.ImportObjectInfoChecksum, ParseSummary: w.ParseSummary, Dependencies: w.Dependencies}
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

// ComfyUIWorkflowValidation 保存一次不可变的目标实例兼容性快照。
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
	// ObjectInfo 是本次校验重新读取的独立能力快照；failed 记录允许为空。
	ObjectInfo         map[string]any `json:"object_info_snapshot" gorm:"-"`
	ObjectInfoShadow   *string        `json:"-" gorm:"column:object_info_json;type:text"`
	ObjectInfoChecksum *string        `json:"object_info_checksum" gorm:"column:object_info_checksum;type:text"`
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
	unmarshalOptional(v.ObjectInfoShadow, &v.ObjectInfo)
	unmarshalShadow(v.NodeSummaryShadow, &v.NodeSummary)
	unmarshalShadow(v.DependencySummaryShadow, &v.DependencySummary)
	unmarshalShadow(v.ErrorsShadow, &v.Errors)
	unmarshalShadow(v.WarningsShadow, &v.Warnings)
	return nil
}
func (v *ComfyUIWorkflowValidation) marshal() error {
	if err := marshalOptional(v.ObjectInfo, &v.ObjectInfoShadow); err != nil {
		return err
	}
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
type ComfyUIWorkflowConvertResult struct {
	WorkflowID                 string                      `json:"workflow_id"`
	ApplicationTemplate        *ApplicationTemplate        `json:"application_template"`
	ApplicationTemplateVersion *ApplicationTemplateVersion `json:"application_template_version"`
	WorkflowContractRevision   string                      `json:"workflow_contract_revision"`
}
