package taskfunctionregistry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gowebpki/jcs"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const (
	StatusActive   = "ACTIVE"
	StatusRetained = "RETAINED"
	StatusDisabled = "DISABLED"

	HandlerInfraAdapter = "task-center.infra-adapter"
)

var (
	ErrFunctionNotRegistered = errors.New("function is not registered")
	ErrCallerNotAllowed      = errors.New("function caller is not allowed")
	ErrContractUnavailable   = errors.New("function contract is unavailable")
	ErrInputInvalid          = errors.New("function input is invalid")
	ErrOutputInvalid         = errors.New("function output is invalid")
)

// Registry 是从已 release SSOT 生成的只读函数合同集合；构造成功后可并发读取。
type Registry struct {
	document     Document
	active       map[string]*Contract
	versions     map[string]*Contract
	input        map[string]*jsonschema.Schema
	output       map[string]*jsonschema.Schema
	retryability map[string]bool
}

// Document 对应 function-registry.yaml 的顶层只读结构。
type Document struct {
	SchemaVersion  string                    `json:"schema_version" yaml:"schema_version"`
	RegistryID     string                    `json:"registry_id" yaml:"registry_id"`
	Scope          string                    `json:"scope" yaml:"scope"`
	Description    string                    `json:"description" yaml:"description"`
	ContractDigest DigestRule                `json:"contract_digest" yaml:"contract_digest"`
	S1Refs         S1Refs                    `json:"x-s1-refs" yaml:"x-s1-refs"`
	Schemas        map[string]map[string]any `json:"schemas" yaml:"schemas"`
	Functions      []Contract                `json:"functions" yaml:"functions"`
}

type DigestRule struct {
	Algorithm        string `json:"algorithm" yaml:"algorithm"`
	Canonicalization string `json:"canonicalization" yaml:"canonicalization"`
	Payload          string `json:"payload" yaml:"payload"`
}

type S1Refs struct {
	UserStories   []string `json:"user_stories" yaml:"user_stories"`
	BusinessRules []string `json:"business_rules" yaml:"business_rules"`
}

// Contract 描述一个固定 functionRef 版本的 I/O、执行策略、Infra 映射和安全边界。
type Contract struct {
	FunctionRef          string                `json:"function_ref" yaml:"function_ref"`
	ContractVersion      string                `json:"contract_version" yaml:"contract_version"`
	ContractDigest       string                `json:"contract_digest" yaml:"contract_digest"`
	Status               string                `json:"status" yaml:"status"`
	OwningDomain         string                `json:"owning_domain" yaml:"owning_domain"`
	Handler              string                `json:"handler" yaml:"handler"`
	ExecutionMode        string                `json:"execution_mode" yaml:"execution_mode"`
	RequiredCapabilities []string              `json:"required_capabilities" yaml:"required_capabilities"`
	InputSchemaRef       string                `json:"input_schema_ref" yaml:"input_schema_ref"`
	OutputSchemaRef      string                `json:"output_schema_ref" yaml:"output_schema_ref"`
	TaskIdempotency      TaskIdempotency       `json:"task_idempotency" yaml:"task_idempotency"`
	RetryPolicy          RetryPolicy           `json:"retry_policy" yaml:"retry_policy"`
	CancelPolicy         CancelPolicy          `json:"cancel_policy" yaml:"cancel_policy"`
	TimeoutPolicy        TimeoutPolicy         `json:"timeout_policy" yaml:"timeout_policy"`
	InfraAdapter         InfraAdapter          `json:"infra_adapter" yaml:"infra_adapter"`
	ArtifactRegistration *ArtifactRegistration `json:"artifact_registration,omitempty" yaml:"artifact_registration,omitempty"`
	ResultProjection     ResultProjection      `json:"result_projection" yaml:"result_projection"`
	Security             Security              `json:"security" yaml:"security"`
	S1Refs               S1Refs                `json:"x-s1-refs" yaml:"x-s1-refs"`
}

type TaskIdempotency struct {
	ScopeTemplate         string `json:"scope_template" yaml:"scope_template"`
	KeyTemplate           string `json:"key_template" yaml:"key_template"`
	ManualRetryUsesNewKey bool   `json:"manual_retry_uses_new_key" yaml:"manual_retry_uses_new_key"`
}

type RetryPolicy struct {
	MaxAttempts         int      `json:"max_attempts" yaml:"max_attempts"`
	Backoff             string   `json:"backoff" yaml:"backoff"`
	InitialDelaySeconds int      `json:"initial_delay_seconds" yaml:"initial_delay_seconds"`
	MaxDelaySeconds     int      `json:"max_delay_seconds" yaml:"max_delay_seconds"`
	RetryableErrorCodes []string `json:"retryable_error_codes" yaml:"retryable_error_codes"`
}

type CancelPolicy struct {
	Mode               string `json:"mode" yaml:"mode"`
	GracePeriodSeconds int    `json:"grace_period_seconds" yaml:"grace_period_seconds"`
	TerminalResult     string `json:"terminal_result" yaml:"terminal_result"`
}

type TimeoutPolicy struct {
	StartupTimeoutSeconds    int `json:"startup_timeout_seconds" yaml:"startup_timeout_seconds"`
	PerAttemptTimeoutSeconds int `json:"per_attempt_timeout_seconds" yaml:"per_attempt_timeout_seconds"`
	OverallTimeoutSeconds    int `json:"overall_timeout_seconds" yaml:"overall_timeout_seconds"`
}

type InfraAdapter struct {
	Operation                  string         `json:"operation" yaml:"operation"`
	RuntimeMode                string         `json:"runtime_mode" yaml:"runtime_mode"`
	RequestingService          string         `json:"requesting_service" yaml:"requesting_service"`
	RequestIDTemplate          string         `json:"request_id_template" yaml:"request_id_template"`
	OwnerDomain                string         `json:"owner_domain" yaml:"owner_domain"`
	OwnerReferencePath         string         `json:"owner_reference_path" yaml:"owner_reference_path"`
	AuthorizationRefPath       string         `json:"authorization_ref_path" yaml:"authorization_ref_path"`
	ExistingRuntimeIDPath      string         `json:"existing_runtime_id_path,omitempty" yaml:"existing_runtime_id_path,omitempty"`
	RuntimeProfileIDPath       string         `json:"runtime_profile_id_path,omitempty" yaml:"runtime_profile_id_path,omitempty"`
	RuntimeProfileRevisionPath string         `json:"runtime_profile_revision_path,omitempty" yaml:"runtime_profile_revision_path,omitempty"`
	SourceRefPath              string         `json:"source_ref_path,omitempty" yaml:"source_ref_path,omitempty"`
	SourcePolicy               string         `json:"source_policy,omitempty" yaml:"source_policy,omitempty"`
	Constants                  map[string]any `json:"constants" yaml:"constants"`
}

type ArtifactRegistration struct {
	Enabled                        bool   `json:"enabled" yaml:"enabled"`
	ProducerType                   string `json:"producer_type" yaml:"producer_type"`
	ProducerIDPath                 string `json:"producer_id_path" yaml:"producer_id_path"`
	ProducerIdempotencyKeyTemplate string `json:"producer_idempotency_key_template" yaml:"producer_idempotency_key_template"`
	OutputKey                      string `json:"output_key" yaml:"output_key"`
	DigestSource                   string `json:"digest_source" yaml:"digest_source"`
}

type ResultProjection struct {
	ConsumerDomain  string         `json:"consumer_domain" yaml:"consumer_domain"`
	AggregateType   string         `json:"aggregate_type" yaml:"aggregate_type"`
	AggregateIDPath string         `json:"aggregate_id_path" yaml:"aggregate_id_path"`
	UpdateMode      string         `json:"update_mode" yaml:"update_mode"`
	FieldMappings   []FieldMapping `json:"field_mappings" yaml:"field_mappings"`
}

type FieldMapping struct {
	From      string `json:"from" yaml:"from"`
	To        string `json:"to" yaml:"to"`
	Transform string `json:"transform,omitempty" yaml:"transform,omitempty"`
}

type Security struct {
	AllowedCallers          []string `json:"allowed_callers" yaml:"allowed_callers"`
	ForbiddenArgumentFields []string `json:"forbidden_argument_fields" yaml:"forbidden_argument_fields"`
	ForbiddenOutputFields   []string `json:"forbidden_output_fields" yaml:"forbidden_output_fields"`
}

// TaskPolicySnapshot 是创建 AtomicTask 时从固定函数合同派生的不可变执行快照。
type TaskPolicySnapshot struct {
	ContractVersion      string
	ContractDigest       string
	RequiredCapabilities string
	IdempotencyScope     string
	IdempotencyKey       string
	MaxAttempts          int
	RetryDelaySeconds    int
	BackoffType          string
	MaxRetryDelaySeconds int
	PerAttemptTimeout    int
	OverallTimeout       int
	CancelMode           string
	CancelGraceSeconds   int
	CancelTerminalResult string
	StartupTimeout       int
}

// New 校验并构造当前编译进二进制的 released Function Registry。
func New() (*Registry, error) {
	registryDocument, err := yamlDocument(registryYAML)
	if err != nil {
		return nil, fmt.Errorf("decode function registry: %w", err)
	}
	registrySchema, err := yamlDocument(registrySchemaYAML)
	if err != nil {
		return nil, fmt.Errorf("decode function registry schema: %w", err)
	}
	if err := validateDocument(registrySchema, registryDocument); err != nil {
		return nil, fmt.Errorf("validate function registry document: %w", err)
	}

	var document Document
	if err := yaml.Unmarshal(registryYAML, &document); err != nil {
		return nil, fmt.Errorf("decode typed function registry: %w", err)
	}
	retryability := make(map[string]bool)
	if err := json.Unmarshal(errorRetryabilityJSON, &retryability); err != nil {
		return nil, fmt.Errorf("decode function registry error retryability: %w", err)
	}
	registry := &Registry{
		document: document, active: make(map[string]*Contract), versions: make(map[string]*Contract),
		input: make(map[string]*jsonschema.Schema), output: make(map[string]*jsonschema.Schema), retryability: retryability,
	}
	if err := registry.compile(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) compile() error {
	activeCounts := make(map[string]int)
	for index := range r.document.Functions {
		contract := &r.document.Functions[index]
		key := contractKey(contract.FunctionRef, contract.ContractVersion)
		if _, exists := r.versions[key]; exists {
			return fmt.Errorf("duplicate function contract %s", key)
		}
		inputDocument, err := r.schema(contract.InputSchemaRef)
		if err != nil {
			return fmt.Errorf("resolve input schema for %s: %w", key, err)
		}
		outputDocument, err := r.schema(contract.OutputSchemaRef)
		if err != nil {
			return fmt.Errorf("resolve output schema for %s: %w", key, err)
		}
		input, err := compileSchema("urn:omnimam:task-function:"+key+":input", inputDocument)
		if err != nil {
			return fmt.Errorf("compile input schema for %s: %w", key, err)
		}
		output, err := compileSchema("urn:omnimam:task-function:"+key+":output", outputDocument)
		if err != nil {
			return fmt.Errorf("compile output schema for %s: %w", key, err)
		}
		digest, err := contractDigest(contract, inputDocument, outputDocument)
		if err != nil {
			return fmt.Errorf("calculate contract digest for %s: %w", key, err)
		}
		if digest != contract.ContractDigest {
			return fmt.Errorf("function contract %s digest mismatch: calculated %s, declared %s", key, digest, contract.ContractDigest)
		}
		for _, code := range contract.RetryPolicy.RetryableErrorCodes {
			retryable, exists := r.retryability[code]
			if !exists {
				return fmt.Errorf("function contract %s references unknown error code %s", key, code)
			}
			if !retryable {
				return fmt.Errorf("function contract %s references non-retryable error code %s", key, code)
			}
		}
		r.versions[key], r.input[key], r.output[key] = contract, input, output
		if contract.Status == StatusActive {
			activeCounts[contract.FunctionRef]++
			r.active[contract.FunctionRef] = contract
		}
	}
	for functionRef, count := range activeCounts {
		if count != 1 {
			return fmt.Errorf("function %s has %d ACTIVE contracts", functionRef, count)
		}
	}
	for _, contract := range r.document.Functions {
		if activeCounts[contract.FunctionRef] != 1 {
			return fmt.Errorf("function %s must have exactly one ACTIVE contract", contract.FunctionRef)
		}
	}
	return nil
}

// Active 返回调用方当前可创建任务使用的唯一 ACTIVE 合同并校验 arguments。
func (r *Registry) Active(functionRef, caller string, arguments map[string]any) (*Contract, error) {
	contract := r.active[functionRef]
	if contract == nil {
		return nil, fmt.Errorf("%w: %s", ErrFunctionNotRegistered, functionRef)
	}
	if !contains(contract.Security.AllowedCallers, caller) {
		return nil, fmt.Errorf("%w: %s cannot call %s", ErrCallerNotAllowed, caller, functionRef)
	}
	if err := r.validate(r.input[contractKey(contract.FunctionRef, contract.ContractVersion)], arguments); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInputInvalid, err)
	}
	for _, field := range contract.Security.ForbiddenArgumentFields {
		if _, exists := arguments[field]; exists {
			return nil, fmt.Errorf("%w: forbidden argument field %s", ErrInputInvalid, field)
		}
	}
	return contract, nil
}

// PrepareTask 校验调用方策略覆盖并派生合同固定的 AtomicTask 执行快照。
func (r *Registry) PrepareTask(
	contract *Contract,
	arguments map[string]any,
	retryGeneration int,
	requestedMaxAttempts, requestedRetryDelay int,
	requestedBackoff string,
	requestedMaxRetryDelay, requestedPerAttemptTimeout, requestedOverallTimeout int,
) (TaskPolicySnapshot, error) {
	if contract == nil {
		return TaskPolicySnapshot{}, fmt.Errorf("%w: contract is nil", ErrContractUnavailable)
	}
	maxAttempts := requestedMaxAttempts
	if maxAttempts == 0 {
		maxAttempts = contract.RetryPolicy.MaxAttempts
	}
	if maxAttempts < 1 || maxAttempts > contract.RetryPolicy.MaxAttempts {
		return TaskPolicySnapshot{}, fmt.Errorf("retry max_attempts exceeds function contract")
	}
	retryDelay := requestedRetryDelay
	if retryDelay == 0 {
		retryDelay = contract.RetryPolicy.InitialDelaySeconds
	}
	if retryDelay < 0 || retryDelay > contract.RetryPolicy.MaxDelaySeconds {
		return TaskPolicySnapshot{}, fmt.Errorf("retry delay exceeds function contract")
	}
	maxRetryDelay := requestedMaxRetryDelay
	if maxRetryDelay == 0 {
		maxRetryDelay = contract.RetryPolicy.MaxDelaySeconds
	}
	if maxRetryDelay < retryDelay || maxRetryDelay > contract.RetryPolicy.MaxDelaySeconds {
		return TaskPolicySnapshot{}, fmt.Errorf("maximum retry delay exceeds function contract")
	}
	backoff := requestedBackoff
	if backoff == "" {
		backoff = contract.RetryPolicy.Backoff
	}
	if backoff != contract.RetryPolicy.Backoff {
		return TaskPolicySnapshot{}, fmt.Errorf("retry backoff differs from function contract")
	}
	perAttemptTimeout := requestedPerAttemptTimeout
	if perAttemptTimeout == 0 {
		perAttemptTimeout = contract.TimeoutPolicy.PerAttemptTimeoutSeconds
	}
	if perAttemptTimeout < 1 || perAttemptTimeout > contract.TimeoutPolicy.PerAttemptTimeoutSeconds {
		return TaskPolicySnapshot{}, fmt.Errorf("per-attempt timeout exceeds function contract")
	}
	overallTimeout := requestedOverallTimeout
	if overallTimeout == 0 {
		overallTimeout = contract.TimeoutPolicy.OverallTimeoutSeconds
	}
	if overallTimeout < perAttemptTimeout || overallTimeout > contract.TimeoutPolicy.OverallTimeoutSeconds {
		return TaskPolicySnapshot{}, fmt.Errorf("overall timeout exceeds function contract")
	}
	templateValues := map[string]string{"retry_generation": fmt.Sprintf("%d", retryGeneration)}
	for key, value := range arguments {
		templateValues["arguments."+key] = fmt.Sprint(value)
	}
	scope, err := expandTemplate(contract.TaskIdempotency.ScopeTemplate, templateValues)
	if err != nil {
		return TaskPolicySnapshot{}, err
	}
	key, err := expandTemplate(contract.TaskIdempotency.KeyTemplate, templateValues)
	if err != nil {
		return TaskPolicySnapshot{}, err
	}
	return TaskPolicySnapshot{
		ContractVersion: contract.ContractVersion, ContractDigest: contract.ContractDigest,
		RequiredCapabilities: strings.Join(contract.RequiredCapabilities, ","),
		IdempotencyScope:     scope, IdempotencyKey: key, MaxAttempts: maxAttempts,
		RetryDelaySeconds: retryDelay, BackoffType: backoff, MaxRetryDelaySeconds: maxRetryDelay,
		PerAttemptTimeout: perAttemptTimeout, OverallTimeout: overallTimeout,
		CancelMode: contract.CancelPolicy.Mode, CancelGraceSeconds: contract.CancelPolicy.GracePeriodSeconds,
		CancelTerminalResult: contract.CancelPolicy.TerminalResult, StartupTimeout: contract.TimeoutPolicy.StartupTimeoutSeconds,
	}, nil
}

// Resolve 返回历史 AtomicTask 固定的 ACTIVE/RETAINED 合同；不会漂移到新版本。
func (r *Registry) Resolve(functionRef, version, digest string) (*Contract, error) {
	contract := r.versions[contractKey(functionRef, version)]
	if contract == nil || contract.Status == StatusDisabled || contract.ContractDigest != digest {
		return nil, fmt.Errorf("%w: %s@%s %s", ErrContractUnavailable, functionRef, version, digest)
	}
	return contract, nil
}

// ValidateOutput 校验 Worker 的小型 result 是否符合 AtomicTask 固定的输出合同。
func (r *Registry) ValidateOutput(contract *Contract, result map[string]any) error {
	if contract == nil {
		return fmt.Errorf("%w: contract is nil", ErrContractUnavailable)
	}
	key := contractKey(contract.FunctionRef, contract.ContractVersion)
	if err := r.validate(r.output[key], result); err != nil {
		return fmt.Errorf("%w: %v", ErrOutputInvalid, err)
	}
	for _, field := range contract.Security.ForbiddenOutputFields {
		if _, exists := result[field]; exists {
			return fmt.Errorf("%w: forbidden output field %s", ErrOutputInvalid, field)
		}
	}
	return nil
}

// IsManaged 表示 functionRef 是否属于本次 Infra-backed registry。
func (r *Registry) IsManaged(functionRef string) bool {
	if r == nil {
		return false
	}
	for _, contract := range r.document.Functions {
		if contract.FunctionRef == functionRef {
			return true
		}
	}
	return false
}

// ActiveFunctionRefs 返回稳定排序的 ACTIVE functionRef 集合。
func (r *Registry) ActiveFunctionRefs() []string {
	result := make([]string, 0, len(r.active))
	for functionRef := range r.active {
		result = append(result, functionRef)
	}
	sort.Strings(result)
	return result
}

// SourceMetadata 返回生成资产对应的 SSOT commit 与源文件摘要。
func SourceMetadata() string { return strings.TrimSpace(string(sourceMetadata)) }

func (r *Registry) schema(ref string) (map[string]any, error) {
	const prefix = "#/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported schema ref %q", ref)
	}
	document := r.document.Schemas[strings.TrimPrefix(ref, prefix)]
	if document == nil {
		return nil, fmt.Errorf("schema ref %q is unresolved", ref)
	}
	return document, nil
}

func (r *Registry) validate(schema *jsonschema.Schema, value any) error {
	if schema == nil {
		return fmt.Errorf("compiled schema is unavailable")
	}
	normalized, err := normalizeJSON(value)
	if err != nil {
		return err
	}
	return schema.Validate(normalized)
}

func validateDocument(schemaDocument, registryDocument any) error {
	schema, err := compileSchema("https://omnimam.local/schemas/task-center/function-registry.schema.yaml", schemaDocument)
	if err != nil {
		return err
	}
	return schema.Validate(registryDocument)
}

func compileSchema(location string, document any) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}

func contractDigest(contract *Contract, inputSchema, outputSchema map[string]any) (string, error) {
	raw, err := json.Marshal(contract)
	if err != nil {
		return "", err
	}
	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		return "", err
	}
	delete(entry, "contract_digest")
	delete(entry, "x-s1-refs")
	payload := map[string]any{"function_entry": entry, "input_schema": inputSchema, "output_schema": outputSchema}
	raw, err = json.Marshal(payload)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func yamlDocument(raw []byte) (any, error) {
	var yamlValue any
	if err := yaml.Unmarshal(raw, &yamlValue); err != nil {
		return nil, err
	}
	return normalizeJSON(yamlValue)
}

func normalizeJSON(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func contractKey(functionRef, version string) string { return functionRef + "@" + version }

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func expandTemplate(template string, values map[string]string) (string, error) {
	result := template
	for {
		start := strings.IndexByte(result, '{')
		if start < 0 {
			return result, nil
		}
		endOffset := strings.IndexByte(result[start+1:], '}')
		if endOffset < 0 {
			return "", fmt.Errorf("invalid template %q", template)
		}
		end := start + 1 + endOffset
		name := result[start+1 : end]
		value, ok := values[name]
		if !ok || value == "" || value == "<nil>" {
			return "", fmt.Errorf("template %q requires %s", template, name)
		}
		result = result[:start] + value + result[end+1:]
	}
}
