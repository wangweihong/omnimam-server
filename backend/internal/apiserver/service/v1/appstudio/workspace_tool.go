package appstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentgrant"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

const (
	workspaceToolActionStatus = "SOURCE_STATUS"
	workspaceToolActionList   = "SOURCE_LIST"
	workspaceToolActionRead   = "SOURCE_READ"
	workspaceToolActionApply  = "CHANGESET_APPLY"
)

var workspaceToolActions = []string{
	workspaceToolActionStatus,
	workspaceToolActionList,
	workspaceToolActionRead,
	workspaceToolActionApply,
}

type workspaceToolClaimsContextKey struct{}

type workspaceToolDispatcher struct{ service *Service }

type workspaceSourceStatus struct {
	StudioApplicationID     string   `json:"studio_application_id"`
	InvocationID            string   `json:"invocation_id"`
	InitialRevision         int64    `json:"initial_revision"`
	CurrentRevision         int64    `json:"current_revision"`
	AllowedPathScopes       []string `json:"allowed_path_scopes"`
	ResultingChangeSetID    *string  `json:"resulting_change_set_id,omitempty"`
	ResultingSourceRevision *int64   `json:"resulting_source_revision,omitempty"`
}

// IssueAgentWorkspaceToolGrant 由 AppStudio 重新校验 canonical Workspace，并签发当前 Invocation 专用授权。
func (s *Service) IssueAgentWorkspaceToolGrant(
	ctx context.Context,
	owner, agentID, sessionID, invocationID, workspaceID string,
) (string, *agentgrant.WorkspaceToolClaims, error) {
	if s == nil || s.grants == nil || s.workspaceToolProcessor == nil {
		return "", nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "workspace tool authorization is unavailable")
	}
	workspace, err := s.store.GetStudioWorkspace(ctx, workspaceID, owner)
	if err != nil || workspace == nil || workspace.Status != iapiserver.AppStudioWorkspaceStatusReady ||
		agentID == "" || sessionID == "" || invocationID == "" {
		return "", nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "workspace tool authorization scope is invalid")
	}
	issuedAt, expiresAt := s.grants.Window()
	claims := &agentgrant.WorkspaceToolClaims{
		OwnerUserID: owner, StudioApplicationID: workspace.StudioApplicationID, WorkspaceID: workspace.ID,
		AgentID: agentID, SessionID: sessionID, InvocationID: invocationID,
		InitialRevision: workspace.CurrentRevision, AllowedActions: append([]string(nil), workspaceToolActions...),
		AllowedPathScopes: []string{"."}, IssuedAt: issuedAt, ExpiresAt: expiresAt,
	}
	reference, err := s.grants.Issue(iapiserver.AppStudioRefPrefixWorkspaceToolGrant, claims)
	if err != nil {
		return "", nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "workspace tool authorization could not be issued")
	}
	return reference, claims, nil
}

// ProcessWorkspaceToolRequest 验证短期 grant 后处理一次内部 MCP 请求；grant 不进入业务日志或 Task 投影。
func (s *Service) ProcessWorkspaceToolRequest(
	ctx context.Context,
	authorization string,
	headers protocol.Headers,
	body []byte,
) (any, int) {
	requestID := protocol.RequestIDOrNull(body)
	claims, err := s.resolveWorkspaceToolGrant(ctx, authorization)
	if err != nil {
		return workspaceToolRPCFailure(requestID, code.ErrAppStudioSourceAccessInvalid, "ERR_APPSTUDIO_SOURCE_ACCESS_INVALID", "workspace tool authorization is invalid"), 200
	}
	if headers.Method == "" && headers.Name == "" {
		requestCtx := context.WithValue(ctx, workspaceToolClaimsContextKey{}, claims)
		if response, status, handled := s.workspaceToolProcessor.ProcessInitializeProtocol(requestCtx, body); handled {
			return response, status
		}
	}
	if transportErr := protocol.ValidateHeaders(headers); transportErr != nil {
		return protocol.Response{JSONRPC: "2.0", ID: requestID, Error: transportErr.RPC}, transportErr.HTTPStatus
	}
	requestCtx := context.WithValue(ctx, workspaceToolClaimsContextKey{}, claims)
	return s.workspaceToolProcessor.Process(requestCtx, headers, body)
}

// ValidateWorkspaceToolGrant 仅验证内部 Runtime 的短期 Workspace Tool grant，不执行 MCP 方法。
func (s *Service) ValidateWorkspaceToolGrant(ctx context.Context, authorization string) error {
	_, err := s.resolveWorkspaceToolGrant(ctx, authorization)
	return err
}

func (s *Service) resolveWorkspaceToolGrant(ctx context.Context, authorization string) (*agentgrant.WorkspaceToolClaims, error) {
	if s == nil || s.grants == nil || s.workspaceToolProcessor == nil {
		return nil, fmt.Errorf("workspace tool is unavailable")
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authorization, bearerPrefix) || strings.Contains(strings.TrimPrefix(authorization, bearerPrefix), " ") {
		return nil, fmt.Errorf("workspace tool bearer authorization is invalid")
	}
	reference := strings.TrimSpace(strings.TrimPrefix(authorization, bearerPrefix))
	var claims agentgrant.WorkspaceToolClaims
	if err := s.grants.Resolve(reference, iapiserver.AppStudioRefPrefixWorkspaceToolGrant, &claims); err != nil {
		return nil, err
	}
	if err := agentgrant.ValidateWindow(claims.IssuedAt, claims.ExpiresAt); err != nil {
		return nil, err
	}
	workspace, err := s.store.GetStudioWorkspace(ctx, claims.WorkspaceID, claims.OwnerUserID)
	if err != nil || workspace == nil || workspace.StudioApplicationID != claims.StudioApplicationID ||
		workspace.Status != iapiserver.AppStudioWorkspaceStatusReady || claims.AgentID == "" || claims.SessionID == "" || claims.InvocationID == "" ||
		!sameStringSet(claims.AllowedActions, workspaceToolActions) || len(claims.AllowedPathScopes) != 1 || claims.AllowedPathScopes[0] != "." {
		return nil, fmt.Errorf("workspace tool authorization scope is invalid")
	}
	return &claims, nil
}

func (d workspaceToolDispatcher) Dispatch(ctx context.Context, invocation protocol.Invocation) (any, error) {
	claims, ok := ctx.Value(workspaceToolClaimsContextKey{}).(*agentgrant.WorkspaceToolClaims)
	if !ok || claims == nil || d.service == nil {
		return nil, &protocol.RPCError{Code: protocol.JSONRPCBusinessError, Message: "workspace tool authorization is unavailable"}
	}
	switch invocation.Method {
	case protocol.MethodDiscover:
		return protocol.DiscoverResult{
			ResultType: "complete", SupportedVersions: []string{protocol.ProtocolVersion},
			Capabilities: protocol.ServerCapabilities{Tools: protocol.ToolsCapability{}, Resources: protocol.ResourcesCapability{}, Extensions: map[string]json.RawMessage{}},
			Instructions: "Use only application-relative paths. Apply all writes atomically with base_revision and an idempotency key.",
			CacheScope:   "private", TTLMS: 0,
		}, nil
	case protocol.MethodToolsList:
		return protocol.ToolsListResult{ResultType: "complete", Tools: workspaceToolDefinitions()}, nil
	case protocol.MethodToolsCall:
		return d.service.callWorkspaceTool(ctx, claims, invocation)
	default:
		return nil, &protocol.RPCError{Code: protocol.JSONRPCMethodNotFound, Message: "workspace tool method is unsupported"}
	}
}

func (s *Service) callWorkspaceTool(ctx context.Context, claims *agentgrant.WorkspaceToolClaims, invocation protocol.Invocation) (any, error) {
	action, ok := workspaceToolAction(invocation.Name)
	if !ok || !containsString(claims.AllowedActions, action) {
		return workspaceToolError(toolboxerrors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "workspace tool action is not authorized")), nil
	}
	var result any
	var err error
	switch invocation.Name {
	case protocol.ToolAppStudioSourceStatus:
		result, err = s.workspaceSourceStatus(ctx, claims)
	case protocol.ToolAppStudioSourceList:
		var request struct {
			Prefix         string `json:"prefix,omitempty"`
			SourceRevision int64  `json:"source_revision,omitempty"`
		}
		if err = decodeWorkspaceToolArguments(invocation.Arguments, &request); err == nil {
			result, err = s.workspaceSourceList(ctx, claims, request.Prefix, request.SourceRevision)
		}
	case protocol.ToolAppStudioSourceRead:
		var request struct {
			Path           string `json:"path"`
			SourceRevision int64  `json:"source_revision,omitempty"`
		}
		if err = decodeWorkspaceToolArguments(invocation.Arguments, &request); err == nil {
			result, err = s.workspaceSourceRead(ctx, claims, request.Path, request.SourceRevision)
		}
	case protocol.ToolAppStudioChangeSetApply:
		var request iapiserver.StudioChangeSetRequest
		if err = decodeWorkspaceToolArguments(invocation.Arguments, &request); err == nil {
			err = request.Validate()
		}
		if err == nil {
			request.AgentID, request.AgentSessionID, request.AgentInvocationID = claims.AgentID, claims.SessionID, claims.InvocationID
			ownerCtx := context.WithValue(ctx, iapiserver.GinContextKeyUser, &iapiserver.User{ObjectMeta: imachinery.ObjectMeta{ID: claims.OwnerUserID}})
			result, err = s.ApplyChangeSet(ownerCtx, claims.StudioApplicationID, &request)
		}
	}
	if err != nil {
		return workspaceToolError(err), nil
	}
	return workspaceToolSuccess(result), nil
}

func (s *Service) workspaceSourceStatus(ctx context.Context, claims *agentgrant.WorkspaceToolClaims) (*workspaceSourceStatus, error) {
	workspace, err := s.store.GetStudioWorkspace(ctx, claims.WorkspaceID, claims.OwnerUserID)
	if err != nil {
		return nil, err
	}
	status := &workspaceSourceStatus{
		StudioApplicationID: claims.StudioApplicationID, InvocationID: claims.InvocationID,
		InitialRevision: claims.InitialRevision, CurrentRevision: workspace.CurrentRevision,
		AllowedPathScopes: append([]string(nil), claims.AllowedPathScopes...),
	}
	changeSets, err := s.store.ResolveStudioInvocationChangeSets(ctx, claims.StudioApplicationID, claims.OwnerUserID, []string{claims.InvocationID})
	if err != nil {
		return nil, err
	}
	if changeSet := changeSets[claims.InvocationID]; changeSet != nil && changeSet.TargetRevision != nil &&
		changeSet.WorkspaceID == claims.WorkspaceID && changeSet.AgentID == claims.AgentID &&
		changeSet.AgentSessionID == claims.SessionID && changeSet.AgentInvocationID == claims.InvocationID {
		changeSetID, revision := changeSet.ID, *changeSet.TargetRevision
		status.ResultingChangeSetID, status.ResultingSourceRevision = &changeSetID, &revision
	}
	return status, nil
}

func (s *Service) workspaceSourceList(ctx context.Context, claims *agentgrant.WorkspaceToolClaims, prefix string, revision int64) (map[string]any, error) {
	workspace, err := s.store.GetStudioWorkspace(ctx, claims.WorkspaceID, claims.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	if _, err := s.store.GetStudioWorkspaceRevision(ctx, claims.WorkspaceID, revision, claims.OwnerUserID); err != nil {
		return nil, err
	}
	if prefix != "" {
		prefix, err = cleanSourcePath(prefix)
		if err != nil {
			return nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "source prefix is outside the authorized application root")
		}
	}
	files, err := s.store.ListStudioSourceFiles(ctx, claims.WorkspaceID, revision, prefix, claims.OwnerUserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"source_revision": revision, "total": len(files), "files": files}, nil
}

func (s *Service) workspaceSourceRead(ctx context.Context, claims *agentgrant.WorkspaceToolClaims, path string, revision int64) (map[string]any, error) {
	clean, err := cleanSourcePath(path)
	if err != nil {
		return nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceAccessInvalid, "source path is outside the authorized application root")
	}
	workspace, err := s.store.GetStudioWorkspace(ctx, claims.WorkspaceID, claims.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	content, err := s.sources.ReadFile(ctx, claims.WorkspaceID, revision, clean)
	if err != nil {
		return nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceNotVisible, "studio source file not visible")
	}
	if len(content) > maxStudioContentReadBytes {
		return nil, toolboxerrors.NewStatus(code.ErrAppStudioSourceChangeRejected, "source file exceeds workspace tool read limit")
	}
	return map[string]any{"path": clean, "content": string(content), "source_revision": revision}, nil
}

func workspaceToolDefinitions() []protocol.ToolDefinition {
	object := func(required []string, properties map[string]any) map[string]any {
		schema := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	integer := map[string]any{"type": "integer", "minimum": 0}
	path := map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
	statusOutput := object([]string{"studio_application_id", "invocation_id", "initial_revision", "current_revision", "allowed_path_scopes"}, map[string]any{
		"studio_application_id": map[string]any{"type": "string"}, "invocation_id": map[string]any{"type": "string"},
		"initial_revision": integer, "current_revision": integer,
		"allowed_path_scopes":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"resulting_change_set_id": map[string]any{"type": "string"}, "resulting_source_revision": integer,
	})
	return []protocol.ToolDefinition{
		{Name: protocol.ToolAppStudioSourceStatus, Title: "Get AppStudio source status", Description: "Read the current source Revision and this Invocation's applied ChangeSet result.", InputSchema: object(nil, map[string]any{}), OutputSchema: statusOutput},
		{Name: protocol.ToolAppStudioSourceList, Title: "List AppStudio source files", Description: "List application-relative files at an authorized source Revision.", InputSchema: object(nil, map[string]any{"prefix": map[string]any{"type": "string", "maxLength": 1024}, "source_revision": integer}), OutputSchema: map[string]any{"type": "object"}},
		{Name: protocol.ToolAppStudioSourceRead, Title: "Read an AppStudio source file", Description: "Read one application-relative file at an authorized source Revision.", InputSchema: object([]string{"path"}, map[string]any{"path": path, "source_revision": integer}), OutputSchema: map[string]any{"type": "object"}},
		{Name: protocol.ToolAppStudioChangeSetApply, Title: "Apply an AppStudio ChangeSet", Description: "Atomically apply CREATE, UPDATE, DELETE, or MOVE operations using base_revision and an idempotency key.", InputSchema: object([]string{"base_revision", "idempotency_key", "operations"}, map[string]any{
			"base_revision": integer, "idempotency_key": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
			"summary": map[string]any{"type": "string", "maxLength": 2000},
			"operations": map[string]any{"type": "array", "minItems": 1, "maxItems": 200, "items": object([]string{"operation", "path"}, map[string]any{
				"operation": map[string]any{"type": "string", "enum": []string{iapiserver.AppStudioChangeOperationCreate, iapiserver.AppStudioChangeOperationUpdate, iapiserver.AppStudioChangeOperationDelete, iapiserver.AppStudioChangeOperationMove}},
				"path":      path, "content": map[string]any{"type": "string", "maxLength": maxStudioFileBytes}, "target_path": path,
			})},
		}), OutputSchema: map[string]any{"type": "object"}},
	}
}

func workspaceToolAction(name string) (string, bool) {
	switch name {
	case protocol.ToolAppStudioSourceStatus:
		return workspaceToolActionStatus, true
	case protocol.ToolAppStudioSourceList:
		return workspaceToolActionList, true
	case protocol.ToolAppStudioSourceRead:
		return workspaceToolActionRead, true
	case protocol.ToolAppStudioChangeSetApply:
		return workspaceToolActionApply, true
	default:
		return "", false
	}
}

func workspaceToolSuccess(value any) protocol.ToolCompleteResult {
	raw, err := json.Marshal(value)
	if err != nil {
		return workspaceToolEncodingError()
	}
	return protocol.ToolCompleteResult{ResultType: "complete", Content: []protocol.ContentBlock{{Type: "text", Text: string(raw)}}, StructuredContent: value}
}

func workspaceToolError(err error) protocol.ToolCompleteResult {
	status := toolboxerrors.ToStatus(err)
	name := workspaceToolErrorName(status.Code)
	message := status.Message[toolboxerrors.MessageLangENKey]
	if message == "" {
		message = "The AppStudio Workspace Tool request failed."
	}
	value := map[string]any{"code": name, "value": status.Code, "message": message}
	raw, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return workspaceToolEncodingError()
	}
	return protocol.ToolCompleteResult{ResultType: "complete", Content: []protocol.ContentBlock{{Type: "text", Text: string(raw)}}, StructuredContent: value, IsError: true}
}

func workspaceToolEncodingError() protocol.ToolCompleteResult {
	const payload = `{"code":"ERR_APPSTUDIO_SOURCE_CHANGE_REJECTED","message":"The AppStudio Workspace Tool response could not be encoded."}`
	value := map[string]any{"code": "ERR_APPSTUDIO_SOURCE_CHANGE_REJECTED", "message": "The AppStudio Workspace Tool response could not be encoded."}
	return protocol.ToolCompleteResult{ResultType: "complete", Content: []protocol.ContentBlock{{Type: "text", Text: payload}}, StructuredContent: value, IsError: true}
}

func workspaceToolRPCFailure(id json.RawMessage, value int, name, message string) protocol.Response {
	return protocol.Response{JSONRPC: "2.0", ID: id, Error: &protocol.RPCError{Code: protocol.JSONRPCBusinessError, Message: message, Data: &protocol.BusinessError{
		Code: name, Value: value, Message: message, Messages: map[string]string{"zh-CN": "AppStudio 源码访问授权无效。", "en-US": message}, SourceDomain: "appstudio",
	}}}
}

func workspaceToolErrorName(value int) string {
	switch value {
	case code.ErrAppStudioSourceNotVisible:
		return "ERR_APPSTUDIO_SOURCE_NOT_VISIBLE"
	case code.ErrAppStudioSourceRevisionConflict:
		return "ERR_APPSTUDIO_SOURCE_REVISION_CONFLICT"
	case code.ErrAppStudioSourceChangeRejected:
		return "ERR_APPSTUDIO_SOURCE_CHANGE_REJECTED"
	case code.ErrAppStudioSourceAccessInvalid:
		return "ERR_APPSTUDIO_SOURCE_ACCESS_INVALID"
	default:
		return "ERR_APPSTUDIO_SOURCE_CHANGE_REJECTED"
	}
}

func decodeWorkspaceToolArguments(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return toolboxerrors.NewStatus(code.ErrAppStudioSourceChangeRejected, "workspace tool arguments are invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil || err != io.EOF {
		return toolboxerrors.NewStatus(code.ErrAppStudioSourceChangeRejected, "workspace tool arguments are invalid")
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for _, value := range right {
		if !containsString(left, value) {
			return false
		}
	}
	return true
}

var _ protocol.Dispatcher = workspaceToolDispatcher{}
