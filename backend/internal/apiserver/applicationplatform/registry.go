package applicationplatform

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

//go:embed assets/runtime-registry.yaml assets/provider-capability.schema.yaml
var registryAssets embed.FS

type runtimeRegistryDocument struct {
	SchemaVersion          string                                   `yaml:"schema_version"`
	CapabilityDefinitions  []iapiserver.CapabilityDefinition        `yaml:"capability_definitions"`
	EngineAdapters         []iapiserver.EngineAdapterDefinition     `yaml:"engine_adapters"`
	OperationExecutors     []iapiserver.OperationExecutorDefinition `yaml:"operation_executors"`
	ApplicationEngineTypes []iapiserver.ApplicationEngineType       `yaml:"application_engine_types"`
}

// RuntimeRegistry is the immutable set of executable types compiled into the server.
type RuntimeRegistry struct {
	capabilities map[string]iapiserver.CapabilityDefinition
	adapters     map[string]iapiserver.EngineAdapterDefinition
	executors    map[string]iapiserver.OperationExecutorDefinition
	engineTypes  map[string]iapiserver.ApplicationEngineType
	engineList   []*iapiserver.ApplicationEngineType
}

func LoadRuntimeRegistry() (*RuntimeRegistry, error) {
	raw, err := registryAssets.ReadFile("assets/runtime-registry.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded runtime registry: %w", err)
	}
	var document runtimeRegistryDocument
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse embedded runtime registry: %w", err)
	}
	r := &RuntimeRegistry{
		capabilities: make(map[string]iapiserver.CapabilityDefinition),
		adapters:     make(map[string]iapiserver.EngineAdapterDefinition),
		executors:    make(map[string]iapiserver.OperationExecutorDefinition),
		engineTypes:  make(map[string]iapiserver.ApplicationEngineType),
		engineList:   make([]*iapiserver.ApplicationEngineType, 0, len(document.ApplicationEngineTypes)),
	}
	for _, item := range document.CapabilityDefinitions {
		if item.ID == "" || r.capabilities[item.ID].ID != "" {
			return nil, fmt.Errorf("invalid or duplicate capability definition %q", item.ID)
		}
		r.capabilities[item.ID] = item
	}
	for _, item := range document.EngineAdapters {
		if item.ID == "" || r.adapters[item.ID].ID != "" {
			return nil, fmt.Errorf("invalid or duplicate engine adapter %q", item.ID)
		}
		r.adapters[item.ID] = item
	}
	for _, item := range document.OperationExecutors {
		if item.ID == "" || r.executors[item.ID].ID != "" {
			return nil, fmt.Errorf("invalid or duplicate operation executor %q", item.ID)
		}
		if r.adapters[item.EngineAdapterID].ID == "" {
			return nil, fmt.Errorf("executor %s references unknown adapter %s", item.ID, item.EngineAdapterID)
		}
		for _, capabilityID := range item.CapabilityDefinitionIDs {
			if r.capabilities[capabilityID].ID == "" {
				return nil, fmt.Errorf("executor %s references unknown capability %s", item.ID, capabilityID)
			}
		}
		r.executors[item.ID] = item
	}
	for i := range document.ApplicationEngineTypes {
		item := document.ApplicationEngineTypes[i]
		if item.ID == "" || r.engineTypes[item.ID].ID != "" {
			return nil, fmt.Errorf("invalid or duplicate application engine type %q", item.ID)
		}
		if r.adapters[item.EngineAdapterID].ID == "" {
			return nil, fmt.Errorf("engine type %s references unknown adapter %s", item.ID, item.EngineAdapterID)
		}
		for capabilityID, executorID := range item.OperationExecutors {
			executor := r.executors[executorID]
			if executor.ID == "" || !containsString(executor.CapabilityDefinitionIDs, capabilityID) {
				return nil, fmt.Errorf("engine type %s has invalid executor mapping %s=%s", item.ID, capabilityID, executorID)
			}
		}
		r.engineTypes[item.ID] = item
		copyItem := item
		r.engineList = append(r.engineList, &copyItem)
	}
	sort.Slice(r.engineList, func(i, j int) bool { return r.engineList[i].ID < r.engineList[j].ID })
	return r, nil
}

func (r *RuntimeRegistry) EngineTypes() []*iapiserver.ApplicationEngineType {
	items := make([]*iapiserver.ApplicationEngineType, 0, len(r.engineList))
	for _, item := range r.engineList {
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items
}

func (r *RuntimeRegistry) EngineType(id string) (*iapiserver.ApplicationEngineType, bool) {
	item, ok := r.engineTypes[id]
	if !ok {
		return nil, false
	}
	return &item, true
}

func (r *RuntimeRegistry) Capability(id string) (*iapiserver.CapabilityDefinition, bool) {
	item, ok := r.capabilities[id]
	return &item, ok
}

func (r *RuntimeRegistry) operationExecutor(engineTypeID, capabilityID string) (iapiserver.OperationExecutorDefinition, bool) {
	engineType, ok := r.engineTypes[engineTypeID]
	if !ok {
		return iapiserver.OperationExecutorDefinition{}, false
	}
	executorID, ok := engineType.OperationExecutors[capabilityID]
	if !ok {
		return iapiserver.OperationExecutorDefinition{}, false
	}
	executor, ok := r.executors[executorID]
	return executor, ok
}

// OperationExecutor resolves the immutable executor mapping for an engine type and standard capability.
func (r *RuntimeRegistry) OperationExecutor(engineTypeID, capabilityID string) (iapiserver.OperationExecutorDefinition, bool) {
	return r.operationExecutor(engineTypeID, capabilityID)
}

type capabilityEntry struct {
	capability *iapiserver.AIAppProviderCapability
	result     *iapiserver.ProviderCapabilityLoadResult
}

// ProviderCapabilityRegistry is frozen after startup and safe for concurrent reads.
type ProviderCapabilityRegistry struct {
	status       string
	capabilities map[string]*iapiserver.AIAppProviderCapability
	ordered      []*iapiserver.AIAppProviderCapability
	results      []*iapiserver.ProviderCapabilityLoadResult
}

func LoadProviderCapabilityRegistry(directory string, runtime *RuntimeRegistry) (*ProviderCapabilityRegistry, error) {
	if runtime == nil {
		return nil, fmt.Errorf("runtime registry is required")
	}
	registry := &ProviderCapabilityRegistry{
		status:       iapiserver.ProviderRegistryReady,
		capabilities: make(map[string]*iapiserver.AIAppProviderCapability),
		ordered:      []*iapiserver.AIAppProviderCapability{},
		results:      []*iapiserver.ProviderCapabilityLoadResult{},
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		registry.status = iapiserver.ProviderRegistryDegraded
		registry.results = append(registry.results, loadFailure(nil, directory,
			"ERR_AIAPP_PROVIDER_CAPABILITY_DIRECTORY_UNREADABLE",
			code.ErrAIAppProviderCapabilityDirectoryUnreadable, err.Error()))
		return registry, nil
	}
	schema, err := compileProviderCapabilitySchema()
	if err != nil {
		return nil, err
	}
	loaded := make([]*capabilityEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		loaded = append(loaded, loadCapabilityFile(filepath.Join(directory, entry.Name()), schema, runtime))
	}
	duplicates := make(map[string][]int)
	duplicateIDs := make(map[string]struct{})
	for i, entry := range loaded {
		if entry.capability != nil && entry.capability.ID != "" {
			duplicates[entry.capability.ID] = append(duplicates[entry.capability.ID], i)
		}
	}
	for id, indexes := range duplicates {
		if len(indexes) < 2 {
			continue
		}
		duplicateIDs[id] = struct{}{}
		for _, index := range indexes {
			entry := loaded[index]
			entry.capability.Availability = iapiserver.ProviderCapabilityUnavailable
			entry.capability.UnavailableCode = "ERR_AIAPP_PROVIDER_CAPABILITY_ID_DUPLICATED"
			entry.capability.UnavailableSummary = "duplicate ProviderCapability id: " + id
			entry.result.Result = "failed"
			entry.result.ErrorCode = entry.capability.UnavailableCode
			entry.result.ErrorValue = code.ErrAIAppProviderCapabilityIDDuplicated
			entry.result.FailureDetail = entry.capability.UnavailableSummary
		}
	}
	for _, entry := range loaded {
		registry.results = append(registry.results, entry.result)
		if entry.capability == nil || entry.capability.ID == "" {
			continue
		}
		if _, duplicated := duplicateIDs[entry.capability.ID]; duplicated {
			continue
		}
		registry.capabilities[entry.capability.ID] = entry.capability
		registry.ordered = append(registry.ordered, entry.capability)
	}
	sort.Slice(registry.ordered, func(i, j int) bool { return registry.ordered[i].ID < registry.ordered[j].ID })
	return registry, nil
}

func compileProviderCapabilitySchema() (*jsonschema.Schema, error) {
	raw, err := registryAssets.ReadFile("assets/provider-capability.schema.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded provider capability schema: %w", err)
	}
	var document any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse embedded provider capability schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const schemaURL = "https://omnimam.local/schemas/application-platform/provider-capability.schema.yaml"
	if err := compiler.AddResource(schemaURL, document); err != nil {
		return nil, fmt.Errorf("register provider capability schema: %w", err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile provider capability schema: %w", err)
	}
	return schema, nil
}

func loadCapabilityFile(path string, schema *jsonschema.Schema, runtime *RuntimeRegistry) *capabilityEntry {
	loadedAt := imachinery.NewTime(time.Now())
	result := &iapiserver.ProviderCapabilityLoadResult{SourceFile: path, Result: "failed", LoadedAt: loadedAt}
	raw, err := os.ReadFile(path)
	if err != nil {
		result.ErrorCode = "ERR_AIAPP_PROVIDER_CAPABILITY_YAML_INVALID"
		result.ErrorValue = code.ErrAIAppProviderCapabilityYAMLInvalid
		result.FailureDetail = err.Error()
		return &capabilityEntry{result: result}
	}
	var document any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		result.ErrorCode = "ERR_AIAPP_PROVIDER_CAPABILITY_YAML_INVALID"
		result.ErrorValue = code.ErrAIAppProviderCapabilityYAMLInvalid
		result.FailureDetail = err.Error()
		return &capabilityEntry{result: result}
	}
	var capability iapiserver.AIAppProviderCapability
	if err := yaml.Unmarshal(raw, &capability); err != nil {
		result.ErrorCode = "ERR_AIAPP_PROVIDER_CAPABILITY_YAML_INVALID"
		result.ErrorValue = code.ErrAIAppProviderCapabilityYAMLInvalid
		result.FailureDetail = err.Error()
		return &capabilityEntry{result: result}
	}
	capability.LoadedAt = loadedAt
	if capability.ID != "" {
		id := capability.ID
		result.ProviderCapabilityID = &id
	}
	if capability.SchemaVersion != "1.0" {
		return rejectedEntry(result, "ERR_AIAPP_PROVIDER_CAPABILITY_SCHEMA_VERSION_UNSUPPORTED", code.ErrAIAppProviderCapabilitySchemaVersionUnsupported, "unsupported schema_version")
	}
	if err := schema.Validate(document); err != nil {
		return rejectedEntry(result, "ERR_AIAPP_PROVIDER_CAPABILITY_SCHEMA_INVALID", code.ErrAIAppProviderCapabilitySchemaInvalid, err.Error())
	}
	if failure := validateCapabilitySemantics(&capability, runtime); failure != nil {
		return unavailableEntry(&capability, result, failure.name, failure.value, failure.detail)
	}
	if capability.Enabled {
		capability.Availability = iapiserver.ProviderCapabilityAvailable
		result.Result = "loaded"
	} else {
		capability.Availability = iapiserver.ProviderCapabilityDisabled
		result.Result = "disabled"
	}
	return &capabilityEntry{capability: &capability, result: result}
}

type validationFailure struct {
	name   string
	value  int
	detail string
}

func validateCapabilitySemantics(capability *iapiserver.AIAppProviderCapability, runtime *RuntimeRegistry) *validationFailure {
	engineType, ok := runtime.EngineType(capability.ApplicationEngineTypeID)
	if !ok {
		return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_ENGINE_TYPE_MISSING", code.ErrAIAppProviderCapabilityEngineTypeMissing, "application engine type is not registered"}
	}
	if runtime.adapters[engineType.EngineAdapterID].ID == "" {
		return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_ADAPTER_MISSING", code.ErrAIAppProviderCapabilityAdapterMissing, "engine adapter is not registered"}
	}
	models := make(map[string]struct{}, len(capability.Models))
	for _, model := range capability.Models {
		if _, exists := models[model.ID]; exists {
			return variantFailure("duplicate model " + model.ID)
		}
		models[model.ID] = struct{}{}
	}
	operations := make(map[string]iapiserver.ProviderCapabilityOperation, len(capability.Operations))
	for _, operation := range capability.Operations {
		if _, exists := operations[operation.ID]; exists {
			return variantFailure("duplicate operation " + operation.ID)
		}
		if _, ok := runtime.Capability(operation.CapabilityDefinitionID); !ok {
			return variantFailure("unknown capability definition " + operation.CapabilityDefinitionID)
		}
		if _, ok := runtime.operationExecutor(capability.ApplicationEngineTypeID, operation.CapabilityDefinitionID); !ok {
			return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_EXECUTOR_MISSING", code.ErrAIAppProviderCapabilityExecutorMissing, "operation executor is not registered for " + operation.CapabilityDefinitionID}
		}
		operations[operation.ID] = operation
	}
	variants := make(map[string]struct{}, len(capability.Variants))
	for _, variant := range capability.Variants {
		if _, exists := variants[variant.ID]; exists {
			return variantFailure("duplicate variant " + variant.ID)
		}
		if _, ok := models[variant.ModelID]; !ok {
			return variantFailure("variant references unknown model " + variant.ModelID)
		}
		if _, ok := operations[variant.OperationID]; !ok {
			return variantFailure("variant references unknown operation " + variant.OperationID)
		}
		variants[variant.ID] = struct{}{}
	}
	return nil
}

func variantFailure(detail string) *validationFailure {
	return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_VARIANT_INVALID", code.ErrAIAppProviderCapabilityVariantInvalid, detail}
}

func unavailableEntry(capability *iapiserver.AIAppProviderCapability, result *iapiserver.ProviderCapabilityLoadResult, name string, value int, detail string) *capabilityEntry {
	capability.Availability = iapiserver.ProviderCapabilityUnavailable
	capability.UnavailableCode = name
	capability.UnavailableSummary = detail
	result.Result = "failed"
	result.ErrorCode = name
	result.ErrorValue = value
	result.FailureDetail = detail
	return &capabilityEntry{capability: capability, result: result}
}

func rejectedEntry(result *iapiserver.ProviderCapabilityLoadResult, name string, value int, detail string) *capabilityEntry {
	result.Result = "failed"
	result.ErrorCode = name
	result.ErrorValue = value
	result.FailureDetail = detail
	return &capabilityEntry{result: result}
}

func loadFailure(id *string, source, name string, value int, detail string) *iapiserver.ProviderCapabilityLoadResult {
	return &iapiserver.ProviderCapabilityLoadResult{ProviderCapabilityID: id, SourceFile: source, Result: "failed", ErrorCode: name, ErrorValue: value, FailureDetail: detail, LoadedAt: imachinery.NewTime(time.Now())}
}

func (r *ProviderCapabilityRegistry) Status() string { return r.status }

func (r *ProviderCapabilityRegistry) Capabilities() []*iapiserver.AIAppProviderCapability {
	items := make([]*iapiserver.AIAppProviderCapability, 0, len(r.ordered))
	for _, item := range r.ordered {
		items = append(items, cloneCapability(item))
	}
	return items
}

func (r *ProviderCapabilityRegistry) Get(id string) (*iapiserver.AIAppProviderCapability, bool) {
	item, ok := r.capabilities[id]
	if !ok {
		return nil, false
	}
	return cloneCapability(item), true
}

func (r *ProviderCapabilityRegistry) Results() []*iapiserver.ProviderCapabilityLoadResult {
	items := make([]*iapiserver.ProviderCapabilityLoadResult, 0, len(r.results))
	for _, item := range r.results {
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items
}

func cloneCapability(source *iapiserver.AIAppProviderCapability) *iapiserver.AIAppProviderCapability {
	data, _ := json.Marshal(source)
	var target iapiserver.AIAppProviderCapability
	_ = json.Unmarshal(data, &target)
	return &target
}

func isYAMLFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".yaml" || extension == ".yml"
}
func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

var _ fs.FS = registryAssets
