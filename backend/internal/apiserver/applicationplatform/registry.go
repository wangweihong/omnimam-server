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
	"github.com/wangweihong/gotoolbox/pkg/deepcopy"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"gopkg.in/yaml.v3"
)

//go:embed assets/runtime-registry.yaml assets/provider-capability.schema.yaml assets/builtin-provider-capabilities/*.yaml
var registryAssets embed.FS

const builtinProviderCapabilityDirectory = "assets/builtin-provider-capabilities"

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
		if item.ID == "" || r.capabilities[item.ID].ID != "" || item.NameI18n["zh-CN"] == "" || item.NameI18n["en-US"] == "" {
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
		capabilityIDs := make([]string, 0, len(item.OperationExecutors))
		for capabilityID, executorID := range item.OperationExecutors {
			executor := r.executors[executorID]
			if executor.ID == "" || !containsString(executor.CapabilityDefinitionIDs, capabilityID) {
				return nil, fmt.Errorf("engine type %s has invalid executor mapping %s=%s", item.ID, capabilityID, executorID)
			}
			capabilityIDs = append(capabilityIDs, capabilityID)
		}
		sort.Strings(capabilityIDs)
		item.CapabilityDefinitions = map[string][]string{"zh-CN": {}, "en-US": {}}
		for _, capabilityID := range capabilityIDs {
			definition := r.capabilities[capabilityID]
			item.CapabilityDefinitions["zh-CN"] = append(item.CapabilityDefinitions["zh-CN"], definition.NameI18n["zh-CN"])
			item.CapabilityDefinitions["en-US"] = append(item.CapabilityDefinitions["en-US"], definition.NameI18n["en-US"])
		}
		r.engineTypes[item.ID] = item
		r.engineList = append(r.engineList, cloneApplicationEngineType(item))
	}
	sort.Slice(r.engineList, func(i, j int) bool { return r.engineList[i].ID < r.engineList[j].ID })
	return r, nil
}

func (r *RuntimeRegistry) EngineTypes() []*iapiserver.ApplicationEngineType {
	items := make([]*iapiserver.ApplicationEngineType, 0, len(r.engineList))
	for _, item := range r.engineList {
		items = append(items, cloneApplicationEngineType(*item))
	}
	return items
}

func (r *RuntimeRegistry) EngineType(id string) (*iapiserver.ApplicationEngineType, bool) {
	item, ok := r.engineTypes[id]
	if !ok {
		return nil, false
	}
	return cloneApplicationEngineType(item), true
}

func (r *RuntimeRegistry) Capability(id string) (*iapiserver.CapabilityDefinition, bool) {
	item, ok := r.capabilities[id]
	if !ok {
		return nil, false
	}
	copyItem := item
	copyItem.NameI18n = cloneStringMap(item.NameI18n)
	copyItem.InputMediaTypes = append([]string(nil), item.InputMediaTypes...)
	copyItem.OutputMediaTypes = append([]string(nil), item.OutputMediaTypes...)
	return &copyItem, true
}

func cloneApplicationEngineType(item iapiserver.ApplicationEngineType) *iapiserver.ApplicationEngineType {
	copyItem := item
	copyItem.AuthenticationTypes = append([]string(nil), item.AuthenticationTypes...)
	copyItem.OperationExecutors = cloneStringMap(item.OperationExecutors)
	copyItem.AuthenticationConfigSchema = make(map[string]map[string]any, len(item.AuthenticationConfigSchema))
	for key, value := range item.AuthenticationConfigSchema {
		copyItem.AuthenticationConfigSchema[key] = deepcopy.AnyMapClone(value)
	}
	copyItem.CapabilityDefinitions = make(map[string][]string, len(item.CapabilityDefinitions))
	for language, names := range item.CapabilityDefinitions {
		copyItem.CapabilityDefinitions[language] = append([]string(nil), names...)
	}
	return &copyItem
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
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
	schema, err := compileProviderCapabilitySchema()
	if err != nil {
		return nil, err
	}
	builtinEntries, err := fs.ReadDir(registryAssets, builtinProviderCapabilityDirectory)
	if err != nil {
		return nil, fmt.Errorf("read embedded provider capabilities: %w", err)
	}
	for _, entry := range builtinEntries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		assetPath := builtinProviderCapabilityDirectory + "/" + entry.Name()
		raw, readErr := registryAssets.ReadFile(assetPath)
		if readErr != nil {
			return nil, fmt.Errorf("read embedded provider capability %s: %w", entry.Name(), readErr)
		}
		result := loadCapabilityBytes("embedded://provider-capabilities/"+entry.Name(), raw, iapiserver.ProviderCapabilityOriginBuiltin, schema, runtime)
		if result.capability == nil || result.result.Result != "loaded" {
			return nil, fmt.Errorf("invalid embedded provider capability %s: %s", entry.Name(), result.result.FailureDetail)
		}
		if _, exists := registry.capabilities[result.capability.ID]; exists {
			return nil, fmt.Errorf("duplicate embedded provider capability %q", result.capability.ID)
		}
		registry.capabilities[result.capability.ID] = result.capability
		registry.ordered = append(registry.ordered, result.capability)
		registry.results = append(registry.results, result.result)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		registry.status = iapiserver.ProviderRegistryDegraded
		registry.results = append(registry.results, loadFailure(nil, directory,
			"ERR_AIAPP_PROVIDER_CAPABILITY_DIRECTORY_UNREADABLE",
			code.ErrAIAppProviderCapabilityDirectoryUnreadable, err.Error()))
		sort.Slice(registry.ordered, func(i, j int) bool { return registry.ordered[i].ID < registry.ordered[j].ID })
		return registry, nil
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
	reservedIndexes := make(map[int]struct{})
	for i, entry := range loaded {
		if entry.capability != nil && entry.capability.ID != "" {
			if _, reserved := registry.capabilities[entry.capability.ID]; reserved {
				reservedIndexes[i] = struct{}{}
				entry.capability.Availability = iapiserver.ProviderCapabilityUnavailable
				entry.capability.UnavailableCode = "ERR_AIAPP_PROVIDER_CAPABILITY_ID_RESERVED"
				entry.capability.UnavailableSummary = "reserved builtin ProviderCapability id: " + entry.capability.ID
				entry.result.Result = "failed"
				entry.result.ErrorCode = entry.capability.UnavailableCode
				entry.result.ErrorValue = code.ErrAIAppProviderCapabilityIDReserved
				entry.result.FailureDetail = entry.capability.UnavailableSummary
				continue
			}
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
	for index, entry := range loaded {
		registry.results = append(registry.results, entry.result)
		if entry.capability == nil || entry.capability.ID == "" {
			continue
		}
		if _, duplicated := duplicateIDs[entry.capability.ID]; duplicated {
			continue
		}
		if _, reserved := reservedIndexes[index]; reserved {
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
	raw, err := os.ReadFile(path)
	if err != nil {
		loadedAt := imachinery.NewTime(time.Now())
		result := &iapiserver.ProviderCapabilityLoadResult{SourceFile: path, Result: "failed", LoadedAt: loadedAt}
		result.ErrorCode = "ERR_AIAPP_PROVIDER_CAPABILITY_YAML_INVALID"
		result.ErrorValue = code.ErrAIAppProviderCapabilityYAMLInvalid
		result.FailureDetail = err.Error()
		return &capabilityEntry{result: result}
	}
	return loadCapabilityBytes(path, raw, iapiserver.ProviderCapabilityOriginDirectory, schema, runtime)
}

func loadCapabilityBytes(source string, raw []byte, origin string, schema *jsonschema.Schema, runtime *RuntimeRegistry) *capabilityEntry {
	loadedAt := imachinery.NewTime(time.Now())
	result := &iapiserver.ProviderCapabilityLoadResult{SourceFile: source, Result: "failed", LoadedAt: loadedAt}
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
	capability.Origin = origin
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
	if failure := validateCapabilitySemantics(&capability, runtime, origin); failure != nil {
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

func validateCapabilitySemantics(capability *iapiserver.AIAppProviderCapability, runtime *RuntimeRegistry, origin string) *validationFailure {
	if origin == iapiserver.ProviderCapabilityOriginDirectory && capability.BindingPolicy != iapiserver.ProviderBindingPolicyManual {
		return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_SCHEMA_INVALID", code.ErrAIAppProviderCapabilitySchemaInvalid, "directory capabilities must use manual binding_policy"}
	}
	engineType, ok := runtime.EngineType(capability.ApplicationEngineTypeID)
	if !ok {
		return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_ENGINE_TYPE_MISSING", code.ErrAIAppProviderCapabilityEngineTypeMissing, "application engine type is not registered"}
	}
	if runtime.adapters[engineType.EngineAdapterID].ID == "" {
		return &validationFailure{"ERR_AIAPP_PROVIDER_CAPABILITY_ADAPTER_MISSING", code.ErrAIAppProviderCapabilityAdapterMissing, "engine adapter is not registered"}
	}
	if capability.Kind == iapiserver.ProviderCapabilityKindEngineBinding {
		return nil
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

// RequiredBindingsForEngineType 返回指定 EngineType 必须具备的内置不可变绑定。
func (r *ProviderCapabilityRegistry) RequiredBindingsForEngineType(engineTypeID string) []*iapiserver.AIAppProviderCapability {
	items := make([]*iapiserver.AIAppProviderCapability, 0)
	for _, item := range r.ordered {
		if item.ApplicationEngineTypeID != engineTypeID || item.Kind != iapiserver.ProviderCapabilityKindEngineBinding || item.Origin != iapiserver.ProviderCapabilityOriginBuiltin || item.BindingPolicy != iapiserver.ProviderBindingPolicyRequiredImmutable || item.Availability != iapiserver.ProviderCapabilityAvailable {
			continue
		}
		items = append(items, cloneCapability(item))
	}
	return items
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
