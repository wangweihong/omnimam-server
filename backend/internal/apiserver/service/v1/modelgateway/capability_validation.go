package modelgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

const validatorIDsExtension = "x-omnimam-validator-ids"

// CapabilityValidationRequest 是命名复杂校验器接收的当前能力、实例与值快照。
type CapabilityValidationRequest struct {
	Capability *iapiserver.AIAppProviderCapability
	Operation  *iapiserver.ProviderCapabilityOperation
	Variant    *iapiserver.ProviderCapabilityVariant
	Engine     *iapiserver.EngineInstance
	Run        *iapiserver.ApplicationRun
	Value      map[string]any
}

// CapabilityValidator 实现 JSON Schema 无法表达的 provider 复杂约束。
type CapabilityValidator interface {
	ID() string
	Validate(context.Context, CapabilityValidationRequest) error
}

type capabilitySchemaKey struct {
	capabilityID string
	operationID  string
	modelID      string
}

type compiledCapabilitySchema struct {
	input            *jsonschema.Schema
	output           *jsonschema.Schema
	inputValidators  []CapabilityValidator
	outputValidators []CapabilityValidator
	operation        *iapiserver.ProviderCapabilityOperation
	variant          *iapiserver.ProviderCapabilityVariant
}

func compileCapabilitySchema(location string, document map[string]any) (*jsonschema.Schema, error) {
	if len(document) == 0 {
		return nil, nil
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	for _, name := range []string{"asset_image", "asset_video", "asset_audio", "asset_pdf"} {
		formatName := name
		compiler.RegisterFormat(&jsonschema.Format{Name: formatName, Validate: func(value any) error {
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return fmt.Errorf("%s must be a non-empty asset reference", formatName)
			}
			return nil
		}})
	}
	raw, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	var canonical any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		return nil, err
	}
	if err := compiler.AddResource(location, canonical); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}

func composeSchema(base, narrowing map[string]any) map[string]any {
	if len(base) == 0 {
		return narrowing
	}
	if len(narrowing) == 0 {
		return base
	}
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"allOf":   []any{base, narrowing},
	}
}

func schemaValidatorIDs(schema map[string]any) ([]string, error) {
	raw, exists := schema[validatorIDsExtension]
	if !exists {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		if stringsList, stringsOK := raw.([]string); stringsOK {
			return stringsList, nil
		}
		return nil, fmt.Errorf("%s must be an array of strings", validatorIDsExtension)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		id, ok := value.(string)
		if !ok || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("%s must contain non-empty strings", validatorIDsExtension)
		}
		result = append(result, id)
	}
	return result, nil
}

func resolveValidators(schemas []map[string]any, implementations map[string]CapabilityValidator) ([]CapabilityValidator, error) {
	seen := map[string]struct{}{}
	result := []CapabilityValidator{}
	for _, schema := range schemas {
		ids, err := schemaValidatorIDs(schema)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if _, exists := seen[id]; exists {
				continue
			}
			implementation := implementations[id]
			if implementation == nil {
				return nil, fmt.Errorf("validator %q has no implementation", id)
			}
			seen[id] = struct{}{}
			result = append(result, implementation)
		}
	}
	return result, nil
}

func (r *ProviderCapabilityRegistry) addCapabilitySchemas(capability *iapiserver.AIAppProviderCapability) error {
	operations := make(map[string]*iapiserver.ProviderCapabilityOperation, len(capability.Operations))
	for index := range capability.Operations {
		operation := &capability.Operations[index]
		operations[operation.ID] = operation
		if err := r.addCompiledSchema(capability, operation, nil); err != nil {
			return err
		}
	}
	for index := range capability.Variants {
		variant := &capability.Variants[index]
		if err := r.addCompiledSchema(capability, operations[variant.OperationID], variant); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProviderCapabilityRegistry) addCompiledSchema(capability *iapiserver.AIAppProviderCapability, operation *iapiserver.ProviderCapabilityOperation, variant *iapiserver.ProviderCapabilityVariant) error {
	if operation == nil {
		return fmt.Errorf("operation is required")
	}
	modelID := ""
	var variantInput, variantOutput map[string]any
	if variant != nil {
		modelID = variant.ModelID
		variantInput, variantOutput = variant.InputSchema, variant.OutputSchema
	}
	inputDocument := composeSchema(operation.InputSchema, variantInput)
	outputDocument := composeSchema(operation.OutputSchema, variantOutput)
	if variant == nil && len(inputDocument) == 0 && len(outputDocument) == 0 {
		return nil
	}
	key := capabilitySchemaKey{capabilityID: capability.ID, operationID: operation.ID, modelID: modelID}
	location := "https://omnimam.local/provider-capabilities/" + capability.ID + "/" + operation.ID
	if modelID != "" {
		location += "/" + modelID
	}
	input, err := compileCapabilitySchema(location+"/input.schema.json", inputDocument)
	if err != nil {
		return fmt.Errorf("operation %s model %s input schema: %w", operation.ID, modelID, err)
	}
	output, err := compileCapabilitySchema(location+"/output.schema.json", outputDocument)
	if err != nil {
		return fmt.Errorf("operation %s model %s output schema: %w", operation.ID, modelID, err)
	}
	inputValidators, err := resolveValidators([]map[string]any{operation.InputSchema, variantInput}, r.validators)
	if err != nil {
		return fmt.Errorf("operation %s model %s input schema: %w", operation.ID, modelID, err)
	}
	outputValidators, err := resolveValidators([]map[string]any{operation.OutputSchema, variantOutput}, r.validators)
	if err != nil {
		return fmt.Errorf("operation %s model %s output schema: %w", operation.ID, modelID, err)
	}
	r.schemas[key] = compiledCapabilitySchema{input: input, output: output, inputValidators: inputValidators, outputValidators: outputValidators, operation: operation, variant: variant}
	return nil
}

func (r *ProviderCapabilityRegistry) validationSchema(capabilityID, operationID, modelID string) (*iapiserver.AIAppProviderCapability, compiledCapabilitySchema, error) {
	capability := r.capabilities[capabilityID]
	if capability == nil {
		return nil, compiledCapabilitySchema{}, fmt.Errorf("provider capability %q is not registered", capabilityID)
	}
	if schema, ok := r.schemas[capabilitySchemaKey{capabilityID: capabilityID, operationID: operationID, modelID: modelID}]; ok {
		return capability, schema, nil
	}
	if modelID != "" {
		for _, variant := range capability.Variants {
			if variant.OperationID == operationID {
				return nil, compiledCapabilitySchema{}, fmt.Errorf("provider capability %s operation %s does not register model %s", capabilityID, operationID, modelID)
			}
		}
	}
	if schema, ok := r.schemas[capabilitySchemaKey{capabilityID: capabilityID, operationID: operationID}]; ok {
		return capability, schema, nil
	}
	return nil, compiledCapabilitySchema{}, fmt.Errorf("provider capability %s operation %s model %s has no schema", capabilityID, operationID, modelID)
}

// ValidateInput 校验即将提交给 provider 的完整标准输入和复杂规则。
func (r *ProviderCapabilityRegistry) ValidateInput(ctx context.Context, capabilityID, operationID, modelID string, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun, value map[string]any) error {
	capability, schema, err := r.validationSchema(capabilityID, operationID, modelID)
	if err != nil {
		return err
	}
	if schema.input == nil {
		return fmt.Errorf("provider capability input schema is missing")
	}
	if err := schema.input.Validate(value); err != nil {
		return err
	}
	request := CapabilityValidationRequest{Capability: capability, Operation: schema.operation, Variant: schema.variant, Engine: engine, Run: run, Value: value}
	for _, validator := range schema.inputValidators {
		if err := validator.Validate(ctx, request); err != nil {
			return fmt.Errorf("validator %s: %w", validator.ID(), err)
		}
	}
	return nil
}

// ValidateOutput 校验归一化 values 的必需结构，同时允许 schema 声明的兼容扩展。
func (r *ProviderCapabilityRegistry) ValidateOutput(ctx context.Context, capabilityID, operationID, modelID string, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun, value map[string]any) error {
	capability, schema, err := r.validationSchema(capabilityID, operationID, modelID)
	if err != nil {
		return err
	}
	if schema.output == nil {
		return fmt.Errorf("provider capability output schema is missing")
	}
	if err := schema.output.Validate(value); err != nil {
		return err
	}
	request := CapabilityValidationRequest{Capability: capability, Operation: schema.operation, Variant: schema.variant, Engine: engine, Run: run, Value: value}
	for _, validator := range schema.outputValidators {
		if err := validator.Validate(ctx, request); err != nil {
			return fmt.Errorf("validator %s: %w", validator.ID(), err)
		}
	}
	return nil
}
