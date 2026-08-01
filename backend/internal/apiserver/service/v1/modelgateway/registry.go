package modelgateway

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

const (
	localeChinese = "zh-CN"
	localeEnglish = "en-US"
)

// Registration 是一个具体提供商随构建交付的完整静态注册。
type Registration struct {
	CapabilityDefinitions []iapiserver.CapabilityDefinition
	EngineAdapter         iapiserver.EngineAdapterDefinition
	OperationExecutors    []iapiserver.OperationExecutorDefinition
	EngineType            iapiserver.ApplicationEngineType
	ProviderCapabilities  []iapiserver.AIAppProviderCapability
}

// RuntimeRegistry 是由全部提供商静态注册原子组装的不可变执行目录。
type RuntimeRegistry struct {
	capabilities map[string]iapiserver.CapabilityDefinition
	adapters     map[string]iapiserver.EngineAdapterDefinition
	executors    map[string]iapiserver.OperationExecutorDefinition
	engineTypes  map[string]iapiserver.ApplicationEngineType
	engineList   []*iapiserver.ApplicationEngineType
}

// AuthenticationConfigSchema 返回 EngineType 使用的严格鉴权字段结构。
func AuthenticationConfigSchema(authTypes ...string) map[string]map[string]any {
	schemas := make(map[string]map[string]any, len(authTypes))
	for _, authType := range authTypes {
		required, ok := authenticationFields(authType)
		if !ok {
			continue
		}
		schema := map[string]any{"type": "object", "additionalProperties": false, "required": required}
		if authType == iapiserver.EngineAuthNone {
			schema["forbidden"] = true
		}
		schemas[authType] = schema
	}
	return schemas
}

// NewRuntimeRegistry 显式合并提供商注册；任一冲突或缺失引用都会阻止启动。
func NewRuntimeRegistry(registrations []Registration) (*RuntimeRegistry, error) {
	registry := &RuntimeRegistry{
		capabilities: make(map[string]iapiserver.CapabilityDefinition),
		adapters:     make(map[string]iapiserver.EngineAdapterDefinition),
		executors:    make(map[string]iapiserver.OperationExecutorDefinition),
		engineTypes:  make(map[string]iapiserver.ApplicationEngineType),
		engineList:   make([]*iapiserver.ApplicationEngineType, 0, len(registrations)),
	}
	for _, registration := range registrations {
		for _, definition := range registration.CapabilityDefinitions {
			if err := registry.addCapabilityDefinition(definition); err != nil {
				return nil, err
			}
		}
		adapter := registration.EngineAdapter
		if strings.TrimSpace(adapter.ID) == "" {
			return nil, fmt.Errorf("static adapter registration has empty adapter id")
		}
		if _, exists := registry.adapters[adapter.ID]; exists {
			return nil, fmt.Errorf("duplicate static engine adapter %q", adapter.ID)
		}
		registry.adapters[adapter.ID] = adapter
	}
	for _, registration := range registrations {
		for _, executor := range registration.OperationExecutors {
			if err := registry.addExecutor(executor); err != nil {
				return nil, err
			}
		}
	}
	for _, registration := range registrations {
		if err := registry.addEngineType(registration.EngineType); err != nil {
			return nil, err
		}
	}
	sort.Slice(registry.engineList, func(i, j int) bool { return registry.engineList[i].ID < registry.engineList[j].ID })
	return registry, nil
}

func (r *RuntimeRegistry) addCapabilityDefinition(item iapiserver.CapabilityDefinition) error {
	if strings.TrimSpace(item.ID) == "" || !hasBilingualText(item.NameI18n) {
		return fmt.Errorf("invalid static capability definition %q", item.ID)
	}
	if existing, exists := r.capabilities[item.ID]; exists {
		if !reflect.DeepEqual(existing, item) {
			return fmt.Errorf("conflicting static capability definition %q", item.ID)
		}
		return nil
	}
	r.capabilities[item.ID] = item
	return nil
}

func (r *RuntimeRegistry) addExecutor(item iapiserver.OperationExecutorDefinition) error {
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("static operation executor has empty id")
	}
	if _, exists := r.executors[item.ID]; exists {
		return fmt.Errorf("duplicate static operation executor %q", item.ID)
	}
	if _, ok := r.adapters[item.EngineAdapterID]; !ok {
		return fmt.Errorf("executor %s references unknown adapter %s", item.ID, item.EngineAdapterID)
	}
	for _, capabilityID := range item.CapabilityDefinitionIDs {
		if _, ok := r.capabilities[capabilityID]; !ok {
			return fmt.Errorf("executor %s references unknown capability %s", item.ID, capabilityID)
		}
	}
	r.executors[item.ID] = item
	return nil
}

func (r *RuntimeRegistry) addEngineType(item iapiserver.ApplicationEngineType) error {
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("static application engine type has empty id")
	}
	if _, exists := r.engineTypes[item.ID]; exists {
		return fmt.Errorf("duplicate static application engine type %q", item.ID)
	}
	if !hasBilingualText(item.NameI18n) || !hasBilingualText(item.DescriptionI18n) {
		return fmt.Errorf("engine type %s is missing zh-CN or en-US text", item.ID)
	}
	for field, value := range map[string]string{
		"official_website_url":       item.OfficialWebsiteURL,
		"official_documentation_url": item.OfficialDocumentationURL,
		"default_api_base_url":       item.DefaultAPIBaseURL,
	} {
		if !validAbsoluteURL(value) {
			return fmt.Errorf("engine type %s has invalid %s", item.ID, field)
		}
	}
	if _, ok := r.adapters[item.EngineAdapterID]; !ok {
		return fmt.Errorf("engine type %s references unknown adapter %s", item.ID, item.EngineAdapterID)
	}
	seenAuthTypes := make(map[string]struct{}, len(item.AuthenticationTypes))
	for _, authType := range item.AuthenticationTypes {
		expectedFields, supported := authenticationFields(authType)
		if !supported {
			return fmt.Errorf("engine type %s has unsupported authentication type %s", item.ID, authType)
		}
		if _, duplicate := seenAuthTypes[authType]; duplicate {
			return fmt.Errorf("engine type %s has duplicate authentication type %s", item.ID, authType)
		}
		seenAuthTypes[authType] = struct{}{}
		schema, exists := item.AuthenticationConfigSchema[authType]
		if !exists || !validAuthenticationSchema(schema, expectedFields, authType == iapiserver.EngineAuthNone) {
			return fmt.Errorf("engine type %s has invalid authentication schema for %s", item.ID, authType)
		}
	}
	for authType := range item.AuthenticationConfigSchema {
		if _, exists := seenAuthTypes[authType]; !exists {
			return fmt.Errorf("engine type %s has authentication schema for unsupported type %s", item.ID, authType)
		}
	}
	capabilityIDs := make([]string, 0, len(item.OperationExecutors))
	for capabilityID, executorID := range item.OperationExecutors {
		executor, ok := r.executors[executorID]
		if !ok || !containsString(executor.CapabilityDefinitionIDs, capabilityID) {
			return fmt.Errorf("engine type %s has invalid executor mapping %s=%s", item.ID, capabilityID, executorID)
		}
		capabilityIDs = append(capabilityIDs, capabilityID)
	}
	sort.Strings(capabilityIDs)
	item.CapabilityDefinitions = map[string][]string{localeChinese: {}, localeEnglish: {}}
	for _, capabilityID := range capabilityIDs {
		definition := r.capabilities[capabilityID]
		item.CapabilityDefinitions[localeChinese] = append(item.CapabilityDefinitions[localeChinese], definition.NameI18n[localeChinese])
		item.CapabilityDefinitions[localeEnglish] = append(item.CapabilityDefinitions[localeEnglish], definition.NameI18n[localeEnglish])
	}
	r.engineTypes[item.ID] = item
	r.engineList = append(r.engineList, item.DeepCopy())
	return nil
}

func (r *RuntimeRegistry) EngineTypes() []*iapiserver.ApplicationEngineType {
	items := make([]*iapiserver.ApplicationEngineType, 0, len(r.engineList))
	for _, item := range r.engineList {
		items = append(items, item.DeepCopy())
	}
	return items
}

func (r *RuntimeRegistry) EngineType(id string) (*iapiserver.ApplicationEngineType, bool) {
	item, ok := r.engineTypes[id]
	if !ok {
		return nil, false
	}
	return item.DeepCopy(), true
}

func (r *RuntimeRegistry) Capability(id string) (*iapiserver.CapabilityDefinition, bool) {
	item, ok := r.capabilities[id]
	if !ok {
		return nil, false
	}
	copyItem := item
	copyItem.NameI18n = maputil.StringString(item.NameI18n).DeepCopy()
	copyItem.InputMediaTypes = append([]string(nil), item.InputMediaTypes...)
	copyItem.OutputMediaTypes = append([]string(nil), item.OutputMediaTypes...)
	return &copyItem, true
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

// OperationExecutor 解析 EngineType 与标准能力的不可变执行器映射。
func (r *RuntimeRegistry) OperationExecutor(engineTypeID, capabilityID string) (iapiserver.OperationExecutorDefinition, bool) {
	return r.operationExecutor(engineTypeID, capabilityID)
}

// ProviderCapabilityRegistry 是进程启动后冻结的静态能力目录。
type ProviderCapabilityRegistry struct {
	capabilities map[string]*iapiserver.AIAppProviderCapability
	ordered      []*iapiserver.AIAppProviderCapability
	schemas      map[capabilitySchemaKey]compiledCapabilitySchema
	validators   map[string]CapabilityValidator
}

// NewProviderCapabilityRegistry 校验全部提供商能力并原子构造只读 Registry。
func NewProviderCapabilityRegistry(registrations []Registration, runtime *RuntimeRegistry, validatorImplementations ...CapabilityValidator) (*ProviderCapabilityRegistry, error) {
	if runtime == nil {
		return nil, fmt.Errorf("runtime registry is required")
	}
	registry := &ProviderCapabilityRegistry{
		capabilities: make(map[string]*iapiserver.AIAppProviderCapability),
		ordered:      []*iapiserver.AIAppProviderCapability{},
		schemas:      make(map[capabilitySchemaKey]compiledCapabilitySchema),
		validators:   make(map[string]CapabilityValidator, len(validatorImplementations)),
	}
	for _, validator := range validatorImplementations {
		if validator == nil || strings.TrimSpace(validator.ID()) == "" {
			return nil, fmt.Errorf("capability validator id is required")
		}
		if _, exists := registry.validators[validator.ID()]; exists {
			return nil, fmt.Errorf("duplicate capability validator %q", validator.ID())
		}
		registry.validators[validator.ID()] = validator
	}
	for _, registration := range registrations {
		for index := range registration.ProviderCapabilities {
			capability := registration.ProviderCapabilities[index].DeepCopy()
			if _, exists := registry.capabilities[capability.ID]; exists {
				return nil, fmt.Errorf("duplicate static provider capability %q", capability.ID)
			}
			if err := validateCapability(capability, runtime, registration.EngineAdapter.ID); err != nil {
				return nil, fmt.Errorf("static provider capability %s: %w", capability.ID, err)
			}
			if err := registry.addCapabilitySchemas(capability); err != nil {
				return nil, fmt.Errorf("static provider capability %s: %w", capability.ID, err)
			}
			registry.capabilities[capability.ID] = capability
			registry.ordered = append(registry.ordered, capability)
		}
	}
	sort.Slice(registry.ordered, func(i, j int) bool { return registry.ordered[i].ID < registry.ordered[j].ID })
	return registry, nil
}

func validateCapability(capability *iapiserver.AIAppProviderCapability, runtime *RuntimeRegistry, adapterID string) error {
	if capability == nil || strings.TrimSpace(capability.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if capability.SchemaVersion != "1.0" || capability.Origin != iapiserver.ProviderCapabilityOriginStatic {
		return fmt.Errorf("schema_version=1.0 and origin=static are required")
	}
	if !hasBilingualText(capability.NameI18n) || !hasBilingualText(capability.DescriptionI18n) {
		return fmt.Errorf("zh-CN and en-US name/description are required")
	}
	if strings.TrimSpace(capability.Revision) == "" {
		return fmt.Errorf("revision is required")
	}
	if len(capability.Provider) == 0 || len(capability.Sources) == 0 {
		return fmt.Errorf("provider and official sources are required")
	}
	if website, _ := capability.Provider["official_website"].(string); !validAbsoluteURL(website) {
		return fmt.Errorf("provider official_website is invalid")
	}
	for index, source := range capability.Sources {
		sourceURL, _ := source["url"].(string)
		if !validAbsoluteURL(sourceURL) {
			return fmt.Errorf("source %d has invalid url", index)
		}
	}
	engineType, ok := runtime.EngineType(capability.ApplicationEngineTypeID)
	if !ok || engineType.EngineAdapterID != adapterID {
		return fmt.Errorf("engine type %q does not resolve to adapter %q", capability.ApplicationEngineTypeID, adapterID)
	}
	if capability.Kind == iapiserver.ProviderCapabilityKindEngineBinding {
		if capability.BindingPolicy != iapiserver.ProviderBindingPolicyRequiredImmutable || len(capability.Models) != 0 || len(capability.Operations) != 0 || len(capability.Variants) != 0 {
			return fmt.Errorf("engine_binding must be required_immutable and have no model catalog")
		}
	} else if capability.Kind != iapiserver.ProviderCapabilityKindCatalog || capability.BindingPolicy != iapiserver.ProviderBindingPolicyManual {
		return fmt.Errorf("catalog must use manual binding policy")
	}
	models := make(map[string]struct{}, len(capability.Models))
	for _, model := range capability.Models {
		if strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.ProviderModelID) == "" || strings.TrimSpace(model.Family) == "" || strings.TrimSpace(model.Variant) == "" || !hasBilingualText(model.DisplayNameI18n) || !hasBilingualText(model.DescriptionI18n) || !validLifecycleStatus(model.Lifecycle.Status) {
			return fmt.Errorf("model %q is incomplete", model.ID)
		}
		if _, exists := models[model.ID]; exists {
			return fmt.Errorf("duplicate model %q", model.ID)
		}
		models[model.ID] = struct{}{}
	}
	operations := make(map[string]iapiserver.ProviderCapabilityOperation, len(capability.Operations))
	for _, operation := range capability.Operations {
		if strings.TrimSpace(operation.ID) == "" || strings.TrimSpace(operation.CapabilityDefinitionID) == "" || !hasBilingualText(operation.NameI18n) || !hasBilingualText(operation.DescriptionI18n) || !validExecutionMode(operation.ExecutionMode) || operation.InputMediaTypes == nil || operation.OutputMediaTypes == nil {
			return fmt.Errorf("operation %q is incomplete", operation.ID)
		}
		if _, exists := operations[operation.ID]; exists {
			return fmt.Errorf("duplicate operation %q", operation.ID)
		}
		if _, ok := runtime.Capability(operation.CapabilityDefinitionID); !ok {
			return fmt.Errorf("operation %s references unknown capability %s", operation.ID, operation.CapabilityDefinitionID)
		}
		if _, ok := runtime.operationExecutor(capability.ApplicationEngineTypeID, operation.CapabilityDefinitionID); !ok {
			return fmt.Errorf("operation %s has no registered executor", operation.ID)
		}
		operations[operation.ID] = operation
	}
	variants := make(map[string]struct{}, len(capability.Variants))
	for _, variant := range capability.Variants {
		if strings.TrimSpace(variant.ID) == "" {
			return fmt.Errorf("variant id is required")
		}
		if _, exists := variants[variant.ID]; exists {
			return fmt.Errorf("duplicate variant %q", variant.ID)
		}
		if _, ok := models[variant.ModelID]; !ok {
			return fmt.Errorf("variant %s references unknown model %s", variant.ID, variant.ModelID)
		}
		operation, ok := operations[variant.OperationID]
		if !ok {
			return fmt.Errorf("variant %s references unknown operation %s", variant.ID, variant.OperationID)
		}
		if !validLifecycleStatus(variant.Lifecycle.Status) || (variant.InputSchema == nil && operation.InputSchema == nil) || (variant.OutputSchema == nil && operation.OutputSchema == nil) {
			return fmt.Errorf("variant %s is incomplete", variant.ID)
		}
		variants[variant.ID] = struct{}{}
	}
	if capability.Enabled {
		capability.Availability = iapiserver.ProviderCapabilityAvailable
	} else {
		capability.Availability = iapiserver.ProviderCapabilityDisabled
	}
	return nil
}

func (r *ProviderCapabilityRegistry) Capabilities() []*iapiserver.AIAppProviderCapability {
	items := make([]*iapiserver.AIAppProviderCapability, 0, len(r.ordered))
	for _, item := range r.ordered {
		items = append(items, item.DeepCopy())
	}
	return items
}

func (r *ProviderCapabilityRegistry) Get(id string) (*iapiserver.AIAppProviderCapability, bool) {
	item, ok := r.capabilities[id]
	if !ok {
		return nil, false
	}
	return item.DeepCopy(), true
}

// RequiredBindingsForEngineType 返回适用于 EngineType 的系统不可变绑定能力。
func (r *ProviderCapabilityRegistry) RequiredBindingsForEngineType(engineTypeID string) []*iapiserver.AIAppProviderCapability {
	items := make([]*iapiserver.AIAppProviderCapability, 0)
	for _, item := range r.ordered {
		if item.ApplicationEngineTypeID != engineTypeID || item.Kind != iapiserver.ProviderCapabilityKindEngineBinding || item.Origin != iapiserver.ProviderCapabilityOriginStatic || item.BindingPolicy != iapiserver.ProviderBindingPolicyRequiredImmutable || item.Availability != iapiserver.ProviderCapabilityAvailable {
			continue
		}
		items = append(items, item.DeepCopy())
	}
	return items
}

func hasBilingualText(values map[string]string) bool {
	return strings.TrimSpace(values[localeChinese]) != "" && strings.TrimSpace(values[localeEnglish]) != ""
}

func validAbsoluteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.IsAbs() && parsed.Host != ""
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func authenticationFields(authType string) ([]string, bool) {
	switch authType {
	case iapiserver.EngineAuthNone:
		return []string{}, true
	case iapiserver.EngineAuthAPIKey:
		return []string{"api_key"}, true
	case iapiserver.EngineAuthBearerToken:
		return []string{"bearer_token"}, true
	case iapiserver.EngineAuthAKSK:
		return []string{"access_key", "secret_key"}, true
	default:
		return nil, false
	}
}

func sameRequiredFields(raw any, expected []string) bool {
	actual, ok := raw.([]string)
	if !ok || len(actual) != len(expected) {
		return false
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, field := range actual {
		actualSet[field] = struct{}{}
	}
	if len(actualSet) != len(expected) {
		return false
	}
	for _, field := range expected {
		if _, exists := actualSet[field]; !exists {
			return false
		}
	}
	return true
}

func validAuthenticationSchema(schema map[string]any, expectedFields []string, forbidden bool) bool {
	if schema["type"] != "object" || schema["additionalProperties"] != false || !sameRequiredFields(schema["required"], expectedFields) {
		return false
	}
	configuredForbidden, hasForbidden := schema["forbidden"].(bool)
	if forbidden {
		return hasForbidden && configuredForbidden
	}
	return !hasForbidden || !configuredForbidden
}

func validLifecycleStatus(status string) bool {
	switch status {
	case "active", "preview", "deprecated", "retired":
		return true
	default:
		return false
	}
}

func validExecutionMode(mode string) bool {
	return mode == "synchronous" || mode == "asynchronous"
}
