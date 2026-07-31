package applicationplatform

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/deepcopy"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type runtimeCandidate struct {
	engineID string
	variants []iapiserver.ProviderCapabilityVariant
}

func (s *applicationPlatformService) resolveRuntimeForm(ctx context.Context, app *iapiserver.Application, version *iapiserver.ApplicationVersion, req *iapiserver.RuntimeFormResolveRequest) (*iapiserver.RuntimeFormSchema, error) {
	templateVersion, err := s.Store.ApplicationPlatforms().GetTemplateVersion(ctx, version.ApplicationTemplateVersionID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppTemplateVersionNotFound, "application template version not found")
	}
	form := &iapiserver.RuntimeFormSchema{
		ApplicationVersionID: version.ID,
		CapabilitySourceType: templateVersion.CapabilitySourceType,
		SourceRevision:       templateVersion.SourceRevision,
		Changes:              []iapiserver.RuntimeFormChange{},
		Violations:           []iapiserver.RuntimeFormViolation{},
		ResolvedAt:           imachinery.Now(),
	}
	var properties map[string]any
	var required []string
	if raw, ok := version.InputSchema["properties"].(map[string]any); ok {
		properties = deepcopy.AnyMapClone(raw)
	} else {
		properties = map[string]any{}
	}
	required = typeutil.SliceAs[string](version.InputSchema["required"])

	switch templateVersion.CapabilitySourceType {
	case iapiserver.CapabilitySourceProviderCapability:
		if templateVersion.ProviderCapabilityID == nil || templateVersion.ProviderOperationID == nil {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "provider capability source is incomplete")
		}
		capability, ok := s.Capabilities.Get(*templateVersion.ProviderCapabilityID)
		if !ok || capability.Kind != iapiserver.ProviderCapabilityKindCatalog || capability.Availability != iapiserver.ProviderCapabilityAvailable {
			return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability is unavailable")
		}
		form.SourceRevision = capability.Revision
		form.ProviderCapabilityID = stringPtr(capability.ID)
		form.ProviderCapabilityRevision = stringPtr(capability.Revision)
		candidates, err := s.providerRuntimeCandidates(ctx, capability, *templateVersion.ProviderOperationID, req.EngineInstanceID)
		if err != nil {
			return nil, err
		}
		variants := selectRuntimeVariants(candidates, req.CurrentValues)
		if len(variants) == 0 {
			return nil, errors.NewStatus(code.ErrAIAppRuntimeFormNoValidVariant, "no executable capability variant")
		}
		for _, candidate := range candidates {
			form.CompatibleEngineInstanceIDs = append(form.CompatibleEngineInstanceIDs, candidate.engineID)
		}
		mergeVariantProperties(properties, &required, variants)
	case iapiserver.CapabilitySourceComfyUIWorkflow:
		if templateVersion.WorkflowContractRevision == nil {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI workflow revision is missing")
		}
		form.WorkflowContractRevision = stringPtr(*templateVersion.WorkflowContractRevision)
		restrictions := typeutil.As[map[string]any](templateVersion.TemplateContract["engine_restrictions"])
		requireEnabled := true
		if value, ok := restrictions["require_enabled"].(bool); ok {
			requireEnabled = value
		}
		var enabledFilter *bool
		if requireEnabled {
			enabledFilter = typeutil.Bool(true)
		}
		allowedIDs := commaSet(typeutil.As[string](restrictions["allowed_engine_instance_ids"]))
		allowedRegions := commaSet(typeutil.As[string](restrictions["allowed_regions"]))
		requiredHealth := typeutil.As[string](restrictions["required_health_status"])
		if requiredHealth == "" {
			requiredHealth = iapiserver.EngineHealthOnline
		}
		engines, _, err := s.Store.ApplicationPlatforms().ListEngineInstances(ctx, &iapiserver.EngineInstanceListRequest{ApplicationEngineTypeID: "comfyui", Enabled: enabledFilter})
		if err != nil {
			return nil, err
		}
		candidates := make([]*iapiserver.EngineInstance, 0, len(engines))
		for _, engine := range engines {
			if engine.HealthStatus != requiredHealth || (req.EngineInstanceID != "" && engine.ID != req.EngineInstanceID) || (len(allowedIDs) > 0 && !allowedIDs[engine.ID]) || (len(allowedRegions) > 0 && !allowedRegions[engine.Region]) {
				continue
			}
			candidates = append(candidates, engine)
		}
		compatible, err := s.compatibleComfyUIEngineIDs(ctx, templateVersion, candidates)
		if err != nil {
			return nil, err
		}
		form.CompatibleEngineInstanceIDs = compatible
		if len(form.CompatibleEngineInstanceIDs) == 0 {
			return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "no compatible ComfyUI engine")
		}
	default:
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "unknown capability source type")
	}
	sort.Strings(form.CompatibleEngineInstanceIDs)
	form.Fields, form.Changes, form.Violations = buildRuntimeFields(properties, required, version.ParameterPolicies, req.CurrentValues)
	return form, nil
}

func commaSet(value string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = true
		}
	}
	return result
}

func (s *applicationPlatformService) compatibleComfyUIEngineIDs(ctx context.Context, version *iapiserver.ApplicationTemplateVersion, engines []*iapiserver.EngineInstance) ([]string, error) {
	if len(engines) == 0 {
		return []string{}, nil
	}
	ids := make([]string, 0, len(engines))
	for _, engine := range engines {
		if !engineMatchesComfyUITemplateRestrictions(engine, version.TemplateContract) {
			continue
		}
		catalog, err := s.Store.ApplicationPlatforms().GetComfyUIEngineObjectInfo(ctx, engine.ID)
		if err != nil || catalog.Stale(time.Now()) {
			continue
		}
		if err := validateComfyUITemplateSnapshot(version.ComfyUIAPIWorkflow, catalog.ObjectInfo, version.TemplateContract); err != nil {
			continue
		}
		ids = append(ids, engine.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

func engineMatchesComfyUITemplateRestrictions(engine *iapiserver.EngineInstance, contract map[string]any) bool {
	if engine == nil || engine.ApplicationEngineTypeID != "comfyui" || !engine.Enabled || engine.HealthStatus != iapiserver.EngineHealthOnline {
		return false
	}
	restrictions := typeutil.As[map[string]any](contract["engine_restrictions"])
	allowedIDs := commaSet(typeutil.As[string](restrictions["allowed_engine_instance_ids"]))
	if len(allowedIDs) > 0 && !allowedIDs[engine.ID] {
		return false
	}
	allowedRegions := commaSet(typeutil.As[string](restrictions["allowed_regions"]))
	return len(allowedRegions) == 0 || allowedRegions[engine.Region]
}

func (s *applicationPlatformService) providerRuntimeCandidates(ctx context.Context, capability *iapiserver.AIAppProviderCapability, operationID, selectedEngineID string) ([]runtimeCandidate, error) {
	enabled := true
	bindings, _, err := s.Store.ApplicationPlatforms().ListEngineBindings(ctx, &iapiserver.EngineCapabilityBindingListRequest{ProviderCapabilityID: capability.ID, Enabled: &enabled})
	if err != nil {
		return nil, err
	}
	baseVariants := make([]iapiserver.ProviderCapabilityVariant, 0)
	for _, variant := range capability.Variants {
		if variant.OperationID == operationID && (variant.Lifecycle.Status == "active" || variant.Lifecycle.Status == "preview") {
			baseVariants = append(baseVariants, variant)
		}
	}
	candidates := make([]runtimeCandidate, 0, len(bindings))
	for _, binding := range bindings {
		s.ResolveBindingStatus(binding)
		if binding.EffectiveStatus != iapiserver.BindingEffectiveAvailable || (selectedEngineID != "" && binding.EngineInstanceID != selectedEngineID) {
			continue
		}
		engine, engineErr := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, binding.EngineInstanceID)
		if engineErr != nil || !runtimeHealthy(engine) || engine.ApplicationEngineTypeID != capability.ApplicationEngineTypeID {
			continue
		}
		variants := restrictVariants(baseVariants, binding.Restrictions)
		if len(variants) > 0 {
			candidates = append(candidates, runtimeCandidate{engineID: engine.ID, variants: variants})
		}
	}
	if len(candidates) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "no enabled and healthy engine binding")
	}
	return candidates, nil
}

func selectRuntimeVariants(candidates []runtimeCandidate, values map[string]any) []iapiserver.ProviderCapabilityVariant {
	seen := map[string]struct{}{}
	result := []iapiserver.ProviderCapabilityVariant{}
	model := typeutil.As[string](values["model"])
	for _, candidate := range candidates {
		for _, variant := range candidate.variants {
			if model != "" && variant.ModelID != model {
				continue
			}
			if _, ok := seen[variant.ID]; ok {
				continue
			}
			seen[variant.ID] = struct{}{}
			result = append(result, variant)
		}
	}
	return result
}

func restrictVariants(variants []iapiserver.ProviderCapabilityVariant, restrictions map[string]any) []iapiserver.ProviderCapabilityVariant {
	models, operations, variantIDs := stringSet(typeutil.SliceAs[string](restrictions["model_ids"])), stringSet(typeutil.SliceAs[string](restrictions["operation_ids"])), stringSet(typeutil.SliceAs[string](restrictions["variant_ids"]))
	result := make([]iapiserver.ProviderCapabilityVariant, 0, len(variants))
	for _, variant := range variants {
		if len(models) > 0 && !models[variant.ModelID] {
			continue
		}
		if len(operations) > 0 && !operations[variant.OperationID] {
			continue
		}
		if len(variantIDs) > 0 && !variantIDs[variant.ID] {
			continue
		}
		result = append(result, variant)
	}
	return result
}

func mergeVariantProperties(properties map[string]any, required *[]string, variants []iapiserver.ProviderCapabilityVariant) {
	properties["model"] = map[string]any{"type": "string", "enum": variantModelIDs(variants)}
	for _, variant := range variants {
		variantProps, _ := variant.InputSchema["properties"].(map[string]any)
		for name, raw := range variantProps {
			schema, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			existing, _ := properties[name].(map[string]any)
			if existing == nil {
				existing = deepcopy.AnyMapClone(schema)
				properties[name] = existing
			}
			if values := anyValues(schema["enum"]); len(values) > 0 {
				existing["enum"] = uniqueValues(append(anyValues(existing["enum"]), values...))
			}
		}
		for _, name := range typeutil.SliceAs[string](variant.InputSchema["required"]) {
			if !contains(*required, name) {
				*required = append(*required, name)
			}
		}
	}
}

func buildRuntimeFields(properties map[string]any, required []string, policies, current map[string]any) ([]iapiserver.RuntimeFormField, []iapiserver.RuntimeFormChange, []iapiserver.RuntimeFormViolation) {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	fields := make([]iapiserver.RuntimeFormField, 0, len(names))
	changes := []iapiserver.RuntimeFormChange{}
	violations := []iapiserver.RuntimeFormViolation{}
	for _, name := range names {
		schema, _ := properties[name].(map[string]any)
		policy, _ := policies[name].(map[string]any)
		field := iapiserver.RuntimeFormField{Name: name, Type: typeutil.As[string](schema["type"]), Required: contains(required, name), Value: current[name], Options: anyValues(schema["enum"]), Connectable: boolValue(schema["x-omnimam-connectable"]), Dynamic: false, DependsOn: typeutil.SliceAs[string](policy["depends_on"]), OnInvalid: typeutil.As[string](policy["on_invalid"]), UI: typeutil.As[map[string]any](policy["ui"])}
		if field.Type == "" {
			field.Type = "string"
		}
		if field.OnInvalid == "" {
			if field.Type == "integer" || field.Type == "number" {
				field.OnInvalid = iapiserver.RuntimeInvalidClamp
			} else {
				field.OnInvalid = iapiserver.RuntimeInvalidReset
			}
		}
		if exposure := typeutil.As[string](policy["exposure"]); exposure == "fixed" {
			field.Value = policy["value"]
			field.Options = []any{policy["value"]}
		}
		if allowlist := anyValues(policy["values"]); len(allowlist) > 0 {
			field.Options = intersectValues(field.Options, allowlist)
			if len(field.Options) == 0 {
				violations = append(violations, iapiserver.RuntimeFormViolation{Field: name, Code: "NO_VALID_OPTION", Message: "application policy has no value allowed by the current capability"})
			}
		}
		field.Dynamic = len(field.DependsOn) > 0
		if field.Value == nil {
			if defaultValue, ok := schema["default"]; ok {
				field.Value = defaultValue
			}
		}
		if field.Value != nil && !valueAllowed(field.Value, field.Options, schema) {
			previous := field.Value
			switch field.OnInvalid {
			case iapiserver.RuntimeInvalidReset:
				field.Value = nil
				changes = append(changes, iapiserver.RuntimeFormChange{Field: name, Reason: "VALUE_NOT_SUPPORTED", Strategy: field.OnInvalid, PreviousValue: previous})
			case iapiserver.RuntimeInvalidFallback:
				if len(field.Options) > 0 {
					field.Value = field.Options[0]
				}
				changes = append(changes, iapiserver.RuntimeFormChange{Field: name, Reason: "VALUE_NOT_SUPPORTED", Strategy: field.OnInvalid, PreviousValue: previous, CurrentValue: field.Value})
			case iapiserver.RuntimeInvalidClamp:
				field.Value = clampValue(field.Value, schema)
				changes = append(changes, iapiserver.RuntimeFormChange{Field: name, Reason: "VALUE_OUT_OF_RANGE", Strategy: field.OnInvalid, PreviousValue: previous, CurrentValue: field.Value})
			default:
				violations = append(violations, iapiserver.RuntimeFormViolation{Field: name, Code: "VALUE_NOT_SUPPORTED", Message: "value is not allowed by the current capability", CurrentValue: previous})
			}
		}
		if field.Required && field.Value == nil {
			violations = append(violations, iapiserver.RuntimeFormViolation{Field: name, Code: "REQUIRED", Message: "required field is missing"})
		}
		fields = append(fields, field)
	}
	return fields, changes, violations
}

func runtimeHealthy(engine *iapiserver.EngineInstance) bool {
	return engine.Enabled && (engine.HealthStatus == iapiserver.EngineHealthOnline || engine.HealthStatus == iapiserver.EngineHealthDegraded)
}
func variantModelIDs(variants []iapiserver.ProviderCapabilityVariant) []any {
	values := []any{}
	for _, variant := range variants {
		values = append(values, variant.ModelID)
	}
	return uniqueValues(values)
}
func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
func anyValues(value any) []any {
	switch typed := value.(type) {
	case []any:
		return append([]any(nil), typed...)
	case []string:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = typed[i]
		}
		return result
	default:
		return nil
	}
}
func uniqueValues(items []any) []any {
	seen := map[string]struct{}{}
	result := []any{}
	for _, item := range items {
		key := fmt.Sprint(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
func intersectValues(left, right []any) []any {
	if len(left) == 0 {
		return uniqueValues(right)
	}
	allowed := map[string]bool{}
	for _, item := range right {
		allowed[fmt.Sprint(item)] = true
	}
	result := []any{}
	for _, item := range left {
		if allowed[fmt.Sprint(item)] {
			result = append(result, item)
		}
	}
	return result
}
func valueAllowed(value any, options []any, schema map[string]any) bool {
	if len(options) > 0 {
		for _, option := range options {
			if fmt.Sprint(option) == fmt.Sprint(value) {
				return true
			}
		}
		return false
	}
	number, ok := numberValue(value)
	if !ok {
		return true
	}
	if min, ok := numberValue(schema["minimum"]); ok && number < min {
		return false
	}
	if max, ok := numberValue(schema["maximum"]); ok && number > max {
		return false
	}
	return true
}
func clampValue(value any, schema map[string]any) any {
	number, ok := numberValue(value)
	if !ok {
		return value
	}
	if min, ok := numberValue(schema["minimum"]); ok && number < min {
		number = min
	}
	if max, ok := numberValue(schema["maximum"]); ok && number > max {
		number = max
	}
	if typeutil.As[string](schema["type"]) == "integer" {
		return int(number)
	}
	return number
}
func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}
func boolValue(value any) bool { ret, _ := value.(bool); return ret }
func boolPtr(value bool) *bool { return &value }
