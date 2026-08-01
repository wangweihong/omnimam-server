package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

const (
	permissionProtocolAccess = "mcp.protocol.access"
	permissionDiscoveryRead  = "mcp.discovery.read"
	permissionResourceRead   = "mcp.resource.read"
	permissionTaskRead       = "mcp.task.read"
	permissionTaskCancel     = "mcp.task.cancel"
)

type Config struct {
	DiscoverTTL      time.Duration
	ResourceTTL      time.Duration
	TaskTTL          time.Duration
	TaskPollInterval time.Duration
	UploadTTL        time.Duration
	PublicBaseURL    string
	RequestRate      int
	RequestBurst     int
	ToolRate         int
	ToolBurst        int
	MaxUploadBytes   int64
	MaxLimiterScopes int
}

func DefaultConfig() Config {
	return Config{
		DiscoverTTL:      5 * time.Minute,
		ResourceTTL:      time.Minute,
		TaskTTL:          24 * time.Hour,
		TaskPollInterval: 2 * time.Second,
		UploadTTL:        time.Hour,
		RequestRate:      20,
		RequestBurst:     40,
		ToolRate:         10,
		ToolBurst:        20,
		MaxUploadBytes:   2 << 30,
		MaxLimiterScopes: 20000,
	}
}

type CapabilityCatalog interface {
	Capability(string) (*iapiserver.CapabilityDefinition, bool)
	CapabilityDefinitions() []*iapiserver.CapabilityDefinition
}

type ApplicationService interface {
	ListApplications(context.Context, *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error)
	GetApplication(context.Context, string) (*iapiserver.Application, error)
	GetApplicationVersion(context.Context, string) (*iapiserver.ApplicationVersion, error)
	CreateApplicationRun(context.Context, string, *iapiserver.ApplicationRunCreateRequest) (*iapiserver.ApplicationRun, error)
	GetApplicationRun(context.Context, string) (*iapiserver.ApplicationRun, error)
}

type TaskService interface {
	GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error)
	CancelAtomicTask(context.Context, string, *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error)
}

type AssetService interface {
	ListAssets(context.Context, *iapiserver.UserAssetListRequest) (*iapiserver.UserAssetListResponse, error)
	GetAsset(context.Context, string) (*iapiserver.AssetDetail, error)
	CreateUploads(context.Context, *iapiserver.CreateAssetUploadsRequest) (*iapiserver.CreateAssetUploadsResponse, error)
	CompleteUploadSession(context.Context, string, string) (*iapiserver.CompleteAssetUploadResponse, error)
	GetArtifact(context.Context, string) (*iapiserver.Artifact, error)
	GetRepresentation(context.Context, string) (*iapiserver.AssetRepresentation, error)
	RepresentationAccess(context.Context, string, string) (*iapiserver.RepresentationAccess, error)
}

type Authorizer interface {
	Allowed(context.Context, string) (bool, error)
}

type Auditor interface {
	Record(context.Context, AuditRecord) error
}

type AuditRecord struct {
	Phase            string
	RequestID        string
	TraceID          string
	Transport        string
	PrincipalID      string
	ClientName       string
	ProtocolVersion  string
	Method           string
	Name             string
	ApplicationRunID string
	MCPTaskID        string
	Result           string
	ErrorCode        string
	OccurredAt       time.Time
	Duration         time.Duration
}

type Dependencies struct {
	Capabilities CapabilityCatalog
	Applications ApplicationService
	Tasks        TaskService
	Assets       AssetService
	Bindings     store.MCPTaskBindingStore
	Authorizer   Authorizer
	Auditor      Auditor
	Admission    AdmissionController
	Config       Config
}

type Service struct {
	capabilities CapabilityCatalog
	applications ApplicationService
	tasks        TaskService
	assets       AssetService
	bindings     store.MCPTaskBindingStore
	authorizer   Authorizer
	auditor      Auditor
	admission    AdmissionController
	validator    *protocol.ResultValidator
	config       Config
}

type requestMetadataContextKey struct{}
type auditCorrelationContextKey struct{}

type auditCorrelation struct {
	ApplicationRunID string
	MCPTaskID        string
}

type RequestMetadata struct {
	RequestID string
	TraceID   string
	Transport string
}

// WithRequestMetadata carries sanitized HTTP correlation fields into MCP authorization, audit, and downstream calls.
func WithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataContextKey{}, metadata)
}

func New(deps Dependencies) (*Service, error) {
	if deps.Capabilities == nil || deps.Applications == nil || deps.Tasks == nil || deps.Assets == nil || deps.Bindings == nil {
		return nil, fmt.Errorf("MCP domain services and task binding store are required")
	}
	if deps.Authorizer == nil {
		deps.Authorizer = AuthenticatedAuthorizer{}
	}
	if deps.Auditor == nil {
		deps.Auditor = StructuredAuditor{}
	}
	defaults := DefaultConfig()
	if deps.Config.DiscoverTTL <= 0 {
		deps.Config.DiscoverTTL = defaults.DiscoverTTL
	}
	if deps.Config.ResourceTTL <= 0 {
		deps.Config.ResourceTTL = defaults.ResourceTTL
	}
	if deps.Config.TaskTTL <= 0 {
		deps.Config.TaskTTL = defaults.TaskTTL
	}
	if deps.Config.TaskPollInterval <= 0 {
		deps.Config.TaskPollInterval = defaults.TaskPollInterval
	}
	if deps.Config.UploadTTL <= 0 {
		deps.Config.UploadTTL = defaults.UploadTTL
	}
	if deps.Config.RequestRate <= 0 {
		deps.Config.RequestRate = defaults.RequestRate
	}
	if deps.Config.RequestBurst <= 0 {
		deps.Config.RequestBurst = defaults.RequestBurst
	}
	if deps.Config.ToolRate <= 0 {
		deps.Config.ToolRate = defaults.ToolRate
	}
	if deps.Config.ToolBurst <= 0 {
		deps.Config.ToolBurst = defaults.ToolBurst
	}
	if deps.Config.MaxUploadBytes <= 0 {
		deps.Config.MaxUploadBytes = defaults.MaxUploadBytes
	}
	if deps.Config.MaxLimiterScopes <= 0 {
		deps.Config.MaxLimiterScopes = defaults.MaxLimiterScopes
	}
	if deps.Admission == nil {
		var err error
		deps.Admission, err = NewLocalAdmissionController(LocalAdmissionConfig{
			RequestRatePerSecond: deps.Config.RequestRate,
			RequestBurst:         deps.Config.RequestBurst,
			ToolRatePerSecond:    deps.Config.ToolRate,
			ToolBurst:            deps.Config.ToolBurst,
			MaxUploadBytes:       deps.Config.MaxUploadBytes,
			MaxTrackedScopes:     deps.Config.MaxLimiterScopes,
		})
		if err != nil {
			return nil, err
		}
	}
	validator, err := protocol.NewResultValidator()
	if err != nil {
		return nil, err
	}
	return &Service{
		capabilities: deps.Capabilities, applications: deps.Applications, tasks: deps.Tasks,
		assets: deps.Assets, bindings: deps.Bindings, authorizer: deps.Authorizer,
		auditor: deps.Auditor, admission: deps.Admission, validator: validator, config: deps.Config,
	}, nil
}

// Dispatch enforces per-request MCP and target-domain permissions before invoking a controlled boundary.
func (s *Service) Dispatch(ctx context.Context, invocation protocol.Invocation) (result any, returnedErr error) {
	principalID, err := currentPrincipalID(ctx)
	if err != nil {
		return nil, rpcBusiness(mcpError(code.ErrMCPAuthenticationRequired, "ERR_MCP_AUTHENTICATION_REQUIRED", false, "identity"))
	}
	if err := s.require(ctx, permissionProtocolAccess); err != nil {
		return nil, err
	}
	correlation := &auditCorrelation{}
	ctx = context.WithValue(ctx, auditCorrelationContextKey{}, correlation)
	name := invocation.Name
	if name == "" {
		name = invocation.URI
	}
	record := AuditRecord{
		Phase: "attempt", PrincipalID: principalID, ProtocolVersion: invocation.Meta.ProtocolVersion,
		Method: invocation.Method, Name: name, MCPTaskID: invocation.TaskID, OccurredAt: time.Now(),
	}
	startedAt := record.OccurredAt
	if metadata, ok := ctx.Value(requestMetadataContextKey{}).(RequestMetadata); ok {
		record.RequestID, record.TraceID, record.Transport = metadata.RequestID, metadata.TraceID, metadata.Transport
	}
	if invocation.Meta.ClientInfo != nil {
		record.ClientName = invocation.Meta.ClientInfo.Name
	}
	write := invocation.Method == protocol.MethodTasksCancel ||
		(invocation.Method == protocol.MethodToolsCall && isWriteTool(invocation.Name))
	if err := s.auditor.Record(ctx, record); err != nil && write {
		return nil, rpcBusiness(mcpError(code.ErrMCPAuditUnavailable, "ERR_MCP_AUDIT_UNAVAILABLE", true, "mcp"))
	}
	defer func() {
		completed := record
		completed.Phase = "result"
		completed.OccurredAt = time.Now()
		completed.Duration = completed.OccurredAt.Sub(startedAt)
		completed.Result = "success"
		populateAuditResult(&completed, result)
		if correlation.ApplicationRunID != "" {
			completed.ApplicationRunID = correlation.ApplicationRunID
		}
		if correlation.MCPTaskID != "" {
			completed.MCPTaskID = correlation.MCPTaskID
		}
		if returnedErr != nil {
			completed.Result = "failed"
			if rpcErr, ok := returnedErr.(*protocol.RPCError); ok && rpcErr.Data != nil {
				completed.ErrorCode = rpcErr.Data.Code
			}
		}
		_ = s.auditor.Record(ctx, completed)
	}()
	admissionRequest := AdmissionRequest{
		PrincipalID: principalID,
		Method:      invocation.Method,
		Name:        invocation.Name,
	}
	if invocation.Meta.ClientInfo != nil {
		admissionRequest.ClientName = invocation.Meta.ClientInfo.Name
	}
	if err := s.admission.AdmitRequest(ctx, admissionRequest); err != nil {
		return nil, rpcBusiness(mcpError(code.ErrMCPRateLimited, "ERR_MCP_RATE_LIMITED", true, "mcp"))
	}

	switch invocation.Method {
	case protocol.MethodDiscover:
		if err := s.require(ctx, permissionDiscoveryRead); err != nil {
			return nil, err
		}
		return s.discover(), nil
	case protocol.MethodToolsList:
		if err := s.require(ctx, permissionDiscoveryRead); err != nil {
			return nil, err
		}
		return s.listTools(ctx), nil
	case protocol.MethodToolsCall:
		return s.dispatchTool(ctx, invocation)
	case protocol.MethodResourcesList:
		if err := s.require(ctx, permissionDiscoveryRead); err != nil {
			return nil, err
		}
		return protocol.ResourcesListResult{ResultType: "complete", Resources: []protocol.Resource{}}, nil
	case protocol.MethodResourceTemplatesList:
		if err := s.require(ctx, permissionDiscoveryRead); err != nil {
			return nil, err
		}
		return protocol.ResourceTemplatesListResult{ResultType: "complete", ResourceTemplates: protocol.ResourceTemplates()}, nil
	case protocol.MethodResourcesRead:
		if err := s.require(ctx, permissionResourceRead); err != nil {
			return nil, err
		}
		return s.readResource(ctx, invocation.URI)
	case protocol.MethodTasksGet:
		return s.getTask(ctx, invocation)
	case protocol.MethodTasksCancel:
		return s.cancelTask(ctx, invocation)
	default:
		return nil, &protocol.RPCError{Code: protocol.JSONRPCMethodNotFound, Message: "MCP method is unsupported"}
	}
}

func (s *Service) discover() protocol.DiscoverResult {
	return protocol.DiscoverResult{
		ResultType: "complete", SupportedVersions: []string{protocol.ProtocolVersion},
		Capabilities: protocol.ServerCapabilities{
			Tools:      protocol.ToolsCapability{ListChanged: false},
			Resources:  protocol.ResourcesCapability{ListChanged: false, Subscribe: false},
			Extensions: map[string]json.RawMessage{protocol.TasksExtension: json.RawMessage(`{}`)},
		},
		Instructions: "Use published Applications for execution; Capability definitions are read-only.",
		CacheScope:   "private", TTLMS: s.config.DiscoverTTL.Milliseconds(),
	}
}

func (s *Service) listTools(ctx context.Context) protocol.ToolsListResult {
	definitions := protocol.ToolDefinitions(func(name string) bool {
		for _, permission := range toolPermissions[name] {
			allowed, err := s.authorizer.Allowed(ctx, permission)
			if err != nil || !allowed {
				return false
			}
		}
		return true
	})
	return protocol.ToolsListResult{ResultType: "complete", Tools: definitions}
}

func (s *Service) dispatchTool(ctx context.Context, invocation protocol.Invocation) (any, error) {
	permissions, exists := toolPermissions[invocation.Name]
	if !exists {
		return nil, rpcBusiness(mcpError(code.ErrMCPToolNotVisible, "ERR_MCP_TOOL_NOT_VISIBLE", false, "mcp"))
	}
	for _, permission := range permissions {
		allowed, err := s.authorizer.Allowed(ctx, permission)
		if err != nil || !allowed {
			failure := mcpError(code.ErrMCPPermissionDenied, "ERR_MCP_PERMISSION_DENIED", false, "mcp")
			return toolErrorResult(failure), nil
		}
	}
	arguments, rpcErr := protocol.DecodeToolArguments(invocation.Name, invocation.Arguments)
	if rpcErr != nil {
		return nil, rpcErr
	}
	admissionRequest := ToolAdmissionRequest{AdmissionRequest: AdmissionRequest{
		Method: invocation.Method,
		Name:   invocation.Name,
	}, Arguments: arguments}
	admissionRequest.PrincipalID, _ = currentPrincipalID(ctx)
	if invocation.Meta.ClientInfo != nil {
		admissionRequest.ClientName = invocation.Meta.ClientInfo.Name
	}
	if err := s.admission.AdmitTool(ctx, admissionRequest); err != nil {
		return toolErrorResult(mcpError(code.ErrMCPRateLimited, "ERR_MCP_RATE_LIMITED", true, "mcp")), nil
	}
	execution, err := s.callTool(ctx, invocation, arguments)
	if err != nil {
		return toolErrorResult(sourceBusinessError(err)), nil
	}
	if err := s.validator.Validate(invocation.Name, execution.Structured); err != nil {
		return toolErrorResult(mcpError(code.ErrMCPToolResultInvalid, "ERR_MCP_TOOL_RESULT_INVALID", true, "mcp")), nil
	}
	if execution.Task != nil {
		return protocol.ToolTaskResult{
			ResultType: "task", Task: *execution.Task, StructuredContent: execution.Structured, Content: execution.Content,
		}, nil
	}
	return protocol.ToolCompleteResult{
		ResultType: "complete", Content: execution.Content, StructuredContent: execution.Structured, IsError: false,
	}, nil
}

type toolExecution struct {
	Structured any
	Content    []protocol.ContentBlock
	Task       *protocol.Task
}

func toolErrorResult(failure *protocol.BusinessError) protocol.ToolCompleteResult {
	return protocol.ToolCompleteResult{
		ResultType:        "complete",
		Content:           []protocol.ContentBlock{{Type: "text", Text: failure.Message}},
		StructuredContent: failure,
		IsError:           true,
	}
}

func (s *Service) require(ctx context.Context, permission string) error {
	allowed, err := s.authorizer.Allowed(ctx, permission)
	if err != nil || !allowed {
		return rpcBusiness(mcpError(code.ErrMCPPermissionDenied, "ERR_MCP_PERMISSION_DENIED", false, "mcp"))
	}
	return nil
}

// AuthenticatedAuthorizer mirrors the current default-role permission behavior while Identity S2 lacks role-permission storage.
type AuthenticatedAuthorizer struct{}

func (AuthenticatedAuthorizer) Allowed(ctx context.Context, permission string) (bool, error) {
	if _, err := currentPrincipalID(ctx); err != nil {
		return false, err
	}
	_, known := releasedPermissions[permission]
	return known, nil
}

// StructuredAuditor emits only low-cardinality protocol metadata for the existing Identity log ingestion boundary.
type StructuredAuditor struct{}

func (StructuredAuditor) Record(_ context.Context, record AuditRecord) error {
	log.Infof(
		"MCP audit: phase=%s request_id=%s trace_id=%s transport=%s principal_id=%s client_name=%s protocol=%s method=%s name=%s application_run_id=%s mcp_task_id=%s result=%s error_code=%s duration_ms=%d",
		safeAuditValue(record.Phase, 32), safeAuditValue(record.RequestID, 128), safeAuditValue(record.TraceID, 128),
		safeAuditValue(record.Transport, 32), safeAuditValue(record.PrincipalID, 255), safeAuditValue(record.ClientName, 128),
		safeAuditValue(record.ProtocolVersion, 32), safeAuditValue(record.Method, 128), safeAuditValue(record.Name, 2048),
		safeAuditValue(record.ApplicationRunID, 255), safeAuditValue(record.MCPTaskID, 255), safeAuditValue(record.Result, 32),
		safeAuditValue(record.ErrorCode, 128), record.Duration.Milliseconds(),
	)
	return nil
}

func safeAuditValue(value string, maximum int) string {
	value = strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return ' '
		}
		return character
	}, value)
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}

func populateAuditResult(record *AuditRecord, result any) {
	if record == nil {
		return
	}
	var structured any
	switch value := result.(type) {
	case protocol.ToolCompleteResult:
		structured = value.StructuredContent
		if value.IsError {
			record.Result = "failed"
		}
	case protocol.ToolTaskResult:
		structured = value.StructuredContent
		record.MCPTaskID = value.Task.TaskID
	case protocol.Task:
		record.MCPTaskID = value.TaskID
	}
	switch value := structured.(type) {
	case *protocol.BusinessError:
		record.ErrorCode = value.Code
	case protocol.BusinessError:
		record.ErrorCode = value.Code
	case ApplicationRunAccepted:
		record.ApplicationRunID = value.ApplicationRunID
	case ApplicationRunProjection:
		record.ApplicationRunID = value.ApplicationRunID
	case ApplicationRunCancelResult:
		record.ApplicationRunID = value.ApplicationRunID
	}
}

func correlateAudit(ctx context.Context, applicationRunID, mcpTaskID string) {
	correlation, _ := ctx.Value(auditCorrelationContextKey{}).(*auditCorrelation)
	if correlation == nil {
		return
	}
	if applicationRunID != "" {
		correlation.ApplicationRunID = applicationRunID
	}
	if mcpTaskID != "" {
		correlation.MCPTaskID = mcpTaskID
	}
}

func currentPrincipalID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || strings.TrimSpace(user.ID) == "" {
		return "", errors.NewStatus(code.ErrMCPAuthenticationRequired, "authenticated user is required")
	}
	return user.ID, nil
}

func isWriteTool(name string) bool {
	return name == protocol.ToolApplicationsRun || name == protocol.ToolApplicationRunsCancel ||
		name == protocol.ToolAssetsPrepareUpload || name == protocol.ToolAssetsCompleteUpload
}

func (s *Service) publicURL(path string) string {
	return strings.TrimRight(s.config.PublicBaseURL, "/") + path
}

var toolPermissions = map[string][]string{
	protocol.ToolCapabilitiesList:      {iapiserver.AIAppProviderCapabilityRead},
	protocol.ToolCapabilitiesGet:       {iapiserver.AIAppProviderCapabilityRead},
	protocol.ToolApplicationsList:      {iapiserver.AIAppApplicationRead},
	protocol.ToolApplicationsGet:       {iapiserver.AIAppApplicationRead},
	protocol.ToolApplicationsRun:       {iapiserver.AIAppApplicationRead, iapiserver.AIAppApplicationRun},
	protocol.ToolApplicationRunsGet:    {iapiserver.AIAppApplicationRun},
	protocol.ToolApplicationRunsCancel: {iapiserver.AIAppApplicationRun, "task.atomic.operate"},
	protocol.ToolAssetsSearch:          {"asset.read"},
	protocol.ToolAssetsGet:             {"asset.read"},
	protocol.ToolAssetsPrepareUpload:   {"asset.upload"},
	protocol.ToolAssetsCompleteUpload:  {"asset.upload"},
}

var releasedPermissions = func() map[string]struct{} {
	permissions := []string{
		permissionProtocolAccess, permissionDiscoveryRead, permissionResourceRead, permissionTaskRead, permissionTaskCancel,
		iapiserver.AIAppProviderCapabilityRead, iapiserver.AIAppApplicationRead, iapiserver.AIAppApplicationRun,
		"task.atomic.operate", "asset.read", "asset.upload", "asset.content.read", "asset.artifact.read",
		"asset.representation.read",
	}
	result := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		result[permission] = struct{}{}
	}
	return result
}()

var _ protocol.Dispatcher = (*Service)(nil)
