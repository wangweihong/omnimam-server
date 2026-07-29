package workflowcanvas

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	applicationsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const applicationArtifactReadyTimeoutSeconds = 1800

// ApplicationVersionPublication 是 Application Platform 发布到 Canvas 节点目录的可靠内部事件。
type ApplicationVersionPublication struct {
	ApplicationID                string         `json:"application_id"`
	ApplicationVersionID         string         `json:"application_version_id"`
	ApplicationTemplateVersionID string         `json:"application_template_version_id"`
	SemanticVersion              string         `json:"semantic_version"`
	ApplicationName              string         `json:"application_name"`
	OwnerUserID                  string         `json:"owner_user_id"`
	Visibility                   string         `json:"visibility"`
	CanvasEnabled                bool           `json:"canvas_enabled"`
	RunEnabled                   bool           `json:"run_enabled"`
	InputSchema                  map[string]any `json:"input_schema"`
	OutputSchema                 map[string]any `json:"output_schema"`
}

// ApplicationCatalogProjector 将发布事件幂等转换为不可变 Application NodeDefinition。
type ApplicationCatalogProjector struct {
	store store.WorkflowCanvasStore
}

// ApplicationCatalogDiagnosticError 表示发布版本无法无损转换，应记录诊断并确认事件。
type ApplicationCatalogDiagnosticError struct {
	err error
}

func (e *ApplicationCatalogDiagnosticError) Error() string { return e.err.Error() }
func (e *ApplicationCatalogDiagnosticError) Unwrap() error { return e.err }

func NewApplicationCatalogProjector(target store.WorkflowCanvasStore) *ApplicationCatalogProjector {
	return &ApplicationCatalogProjector{store: target}
}

// Project 解码发布事实并幂等登记节点定义；事件重放不得产生第二个定义版本。
func (p *ApplicationCatalogProjector) Project(ctx context.Context, payload []byte) error {
	var event ApplicationVersionPublication
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "decode application version published event")
	}
	return p.ProjectPublication(ctx, event)
}

// ProjectPublication 用于启动修复和可靠事件消费共享同一登记语义。
func (p *ApplicationCatalogProjector) ProjectPublication(ctx context.Context, event ApplicationVersionPublication) error {
	definition, err := applicationNodeDefinition(event)
	if err != nil {
		return &ApplicationCatalogDiagnosticError{err: err}
	}
	_, _, err = p.store.AddWorkflowNodeDefinitionIdempotent(ctx, definition)
	return err
}

func applicationNodeDefinition(event ApplicationVersionPublication) (*iapiserver.WorkflowNodeDefinition, error) {
	if event.ApplicationID == "" || event.ApplicationVersionID == "" || event.SemanticVersion == "" ||
		event.ApplicationName == "" || event.OwnerUserID == "" {
		return nil, fmt.Errorf("application version publication is incomplete")
	}
	inputPorts, err := applicationSchemaPorts(event.InputSchema, "input")
	if err != nil {
		return nil, fmt.Errorf("convert application input schema: %w", err)
	}
	outputPorts, err := applicationSchemaPorts(event.OutputSchema, "output")
	if err != nil {
		return nil, fmt.Errorf("convert application output schema: %w", err)
	}
	ports := append(inputPorts, outputPorts...)
	scope := iapiserver.CanvasAvailabilityProject
	var projectID, namespace *string
	if event.Visibility == iapiserver.ApplicationVisibilityGlobal {
		scope = iapiserver.CanvasAvailabilitySystem
	} else {
		project, ns := iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace
		projectID, namespace = &project, &ns
	}
	versionID := event.ApplicationVersionID
	definition := &iapiserver.WorkflowNodeDefinition{
		NodeType:          "application." + strings.ToLower(event.ApplicationID),
		DefinitionVersion: event.SemanticVersion,
		Title:             event.ApplicationName,
		Category:          "application",
		NodeKind:          "processor",
		Ports:             ports,
		ConfigSchema:      map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
		ExecutionBinding: iapiserver.WorkflowExecutionBinding{
			Mode:                 iapiserver.CanvasExecutionAtomic,
			ApplicationVersionID: &versionID,
			BindingVersion:       event.SemanticVersion,
		},
		AvailabilityScope: scope,
		ProjectID:         projectID,
		Namespace:         namespace,
		RegisteredBy:      event.OwnerUserID,
	}
	definition.Name = event.ApplicationName + " " + event.SemanticVersion
	return definition, nil
}

func applicationDefinitionMatches(
	definition *iapiserver.WorkflowNodeDefinition,
	resolved *applicationsvc.CanvasApplicationVersion,
) bool {
	if definition == nil || resolved == nil || resolved.Application == nil || resolved.Version == nil ||
		definition.ApplicationVersionID == nil || *definition.ApplicationVersionID != resolved.Version.ID ||
		definition.NodeType != "application."+strings.ToLower(resolved.Application.ID) ||
		definition.DefinitionVersion != resolved.Version.SemanticVersion {
		return false
	}
	inputs, err := applicationSchemaPorts(resolved.Version.InputSchema, "input")
	if err != nil {
		return false
	}
	outputs, err := applicationSchemaPorts(resolved.Version.OutputSchema, "output")
	if err != nil {
		return false
	}
	return reflect.DeepEqual(definition.Ports, append(inputs, outputs...))
}

func applicationSchemaPorts(schema map[string]any, direction string) ([]iapiserver.WorkflowPortDefinition, error) {
	if schema == nil || schemaString(schema["type"]) != "object" {
		return nil, fmt.Errorf("root schema type must be object")
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("root schema properties are required")
	}
	required := make(map[string]struct{})
	for _, key := range schemaStrings(schema["required"]) {
		required[key] = struct{}{}
	}
	ports := make([]iapiserver.WorkflowPortDefinition, 0, len(properties))
	for key, raw := range properties {
		if !nodeKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("property %q cannot be represented as a port key", key)
		}
		property, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("property %q schema must be an object", key)
		}
		dataType, cardinality, err := applicationPortType(property)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", key, err)
		}
		_, isRequired := required[key]
		port := iapiserver.WorkflowPortDefinition{
			Key: key, Label: schemaLabel(property, key), Direction: direction, DataType: dataType,
			Required: isRequired, Cardinality: cardinality, ConnectionType: "data",
			AllowsLiteral: direction == "input", RequiredForCompletion: direction == "output" && isRequired,
		}
		if value, exists := property["default"]; exists {
			port.DefaultValue = value
		}
		if direction == "output" && isRequired && isArtifactDataType(dataType) {
			timeout := applicationArtifactReadyTimeoutSeconds
			port.ArtifactReadyTimeoutSeconds = &timeout
		}
		ports = append(ports, port)
	}
	sortWorkflowPorts(ports)
	return ports, nil
}

func applicationPortType(property map[string]any) (string, string, error) {
	rawType := schemaString(property["type"])
	cardinality := "single"
	if rawType == "array" {
		cardinality = "multiple"
		items, ok := property["items"].(map[string]any)
		if !ok {
			return "", "", fmt.Errorf("array items schema is required")
		}
		property, rawType = items, schemaString(items["type"])
	}
	if explicit := schemaString(property["x-omnimam-data-type"]); explicit != "" {
		return explicit, cardinality, nil
	}
	format := schemaString(property["format"])
	if strings.HasPrefix(format, "asset_") {
		return strings.TrimPrefix(format, "asset_"), cardinality, nil
	}
	if strings.HasPrefix(rawType, "asset.") {
		return strings.TrimPrefix(rawType, "asset."), cardinality, nil
	}
	switch rawType {
	case "string", "integer", "number", "boolean", "object":
		return rawType, cardinality, nil
	default:
		return "", "", fmt.Errorf("unsupported schema type %q", rawType)
	}
}

func schemaString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "null" {
				return text
			}
		}
	}
	return ""
}

func schemaStrings(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	if typed, ok := value.([]string); ok {
		return typed
	}
	return result
}

func schemaLabel(property map[string]any, fallback string) string {
	if title := schemaString(property["title"]); title != "" {
		return title
	}
	return fallback
}

func isArtifactDataType(value string) bool {
	switch value {
	case "image", "video", "audio", "document", "model_3d", "pdf":
		return true
	default:
		return false
	}
}

func sortWorkflowPorts(ports []iapiserver.WorkflowPortDefinition) {
	for i := 1; i < len(ports); i++ {
		for j := i; j > 0 && ports[j].Key < ports[j-1].Key; j-- {
			ports[j], ports[j-1] = ports[j-1], ports[j]
		}
	}
}
