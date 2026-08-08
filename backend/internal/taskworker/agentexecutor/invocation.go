package agentexecutor

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"golang.org/x/net/websocket"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentgrant"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
)

const (
	openCodeRequestTimeout = 10 * time.Minute
	openCodeCleanupTimeout = 10 * time.Second
)

type InvocationStore interface {
	GetAgent(context.Context, string, string) (*iapiserver.Agent, error)
	GetAgentSession(context.Context, string, string) (*iapiserver.AgentSession, error)
	GetAgentMessage(context.Context, string, string) (*iapiserver.AgentMessage, error)
	GetAgentInvocation(context.Context, string, string) (*iapiserver.AgentInvocation, error)
	GetAgentWorkspaceBinding(context.Context, string, string) (*iapiserver.AgentWorkspaceBinding, error)
	ListAgentModelBindings(context.Context, string, string) ([]*iapiserver.AgentModelBinding, error)
	GetAgentRuntimeByID(context.Context, string) (*iapiserver.AgentRuntimeBinding, error)
	AppendAgentOperationEvent(context.Context, *iapiserver.AgentOperationEvent) (*iapiserver.AgentOperationEvent, error)
	ListAgentOperationEvents(context.Context, string, int) ([]*iapiserver.AgentOperationEvent, error)
	CreateAgentAssistantMessage(context.Context, *iapiserver.AgentMessage) (*iapiserver.AgentMessage, error)
}

type InvocationEndpointResolver interface {
	ResolveEndpoint(context.Context, string, *iapiserver.InfraResolveEndpointRequest) (*iapiserver.InfraResolvedEndpoint, error)
}

type InvocationModelResolver interface {
	ResolveAgentModelAccessGrant(context.Context, string, string, string, string) (*modelgateway.UserModelExecutionContext, error)
}

type InvocationGrantResolver interface {
	Resolve(string, string, any) error
}

type InvocationExecutorDependencies struct {
	Store       InvocationStore
	Endpoints   InvocationEndpointResolver
	Models      InvocationModelResolver
	Credentials modelgateway.CredentialResolver
	Grants      InvocationGrantResolver
	Registry    *taskfunctionregistry.Registry
}

type InvocationExecutor struct {
	store       InvocationStore
	endpoints   InvocationEndpointResolver
	models      InvocationModelResolver
	credentials modelgateway.CredentialResolver
	grants      InvocationGrantResolver
	registry    *taskfunctionregistry.Registry
}

type invocationArguments struct {
	AgentID                    string  `json:"agent_id"`
	SessionID                  string  `json:"session_id"`
	InvocationID               string  `json:"invocation_id"`
	RuntimeBindingID           string  `json:"runtime_binding_id"`
	InvocationType             string  `json:"invocation_type"`
	AuthorizationRef           string  `json:"authorization_ref"`
	ExpectedResourceVersion    int64   `json:"expected_resource_version"`
	ResumeRuntimeSessionRef    *string `json:"resume_runtime_session_ref"`
	ResumeRuntimeInvocationRef *string `json:"resume_runtime_invocation_ref"`
	EventSequenceAfter         int     `json:"event_sequence_after"`
}

type invocationExecution struct {
	arguments    invocationArguments
	claims       agentgrant.InvocationClaims
	agent        *iapiserver.Agent
	session      *iapiserver.AgentSession
	message      *iapiserver.AgentMessage
	invocation   *iapiserver.AgentInvocation
	runtime      *iapiserver.AgentRuntimeBinding
	model        *modelgateway.UserModelExecutionContext
	credential   *modelgateway.ResolvedCredential
	endpointBase string
	sequence     int
}

type openCodeSession struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata"`
}

type openCodeMessage struct {
	Info struct {
		ID       string          `json:"id"`
		Role     string          `json:"role"`
		ParentID string          `json:"parentID"`
		Error    json.RawMessage `json:"error"`
	} `json:"info"`
	Parts []openCodePart `json:"parts"`
}

type openCodePart struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	MessageID string `json:"messageID"`
}

type hermesRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func NewInvocationExecutor(deps InvocationExecutorDependencies) (*InvocationExecutor, error) {
	if deps.Store == nil || deps.Endpoints == nil || deps.Models == nil || deps.Credentials == nil || deps.Grants == nil || deps.Registry == nil {
		return nil, fmt.Errorf("agent invocation executor dependencies are required")
	}
	return &InvocationExecutor{
		store: deps.Store, endpoints: deps.Endpoints, models: deps.Models,
		credentials: deps.Credentials, grants: deps.Grants, registry: deps.Registry,
	}, nil
}

func (e *InvocationExecutor) Execute(
	ctx context.Context,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (result map[string]any, resultErr error) {
	contract, execution, err := e.prepare(ctx, workerTask, atomicTask)
	if err != nil {
		return nil, err
	}
	if execution.runtime.RuntimeProfileID == iapiserver.AgentProfileIDHermes {
		return e.executeHermes(ctx, workerTask, atomicTask, contract, execution)
	}
	if completed, result, err := e.completedResult(ctx, execution, contract); completed || err != nil {
		if completed {
			providerID := deterministicOpenCodeID("omnimam", execution.arguments.InvocationID)
			if cleanupErr := e.removeOpenCodeAuth(context.WithoutCancel(ctx), execution.endpointBase, providerID); cleanupErr != nil {
				return nil, fmt.Errorf("remove completed agent invocation runtime authentication: %w", cleanupErr)
			}
		}
		return result, err
	}

	execution.sequence, err = e.appendEvent(ctx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeInvocationStarted)
	if err != nil {
		return nil, fmt.Errorf("persist agent invocation start event: %w", err)
	}
	providerID := deterministicOpenCodeID("omnimam", execution.arguments.InvocationID)
	authConfigured, err := e.configureOpenCode(ctx, execution, providerID)
	if authConfigured {
		defer func() {
			if cleanupErr := e.removeOpenCodeAuth(context.WithoutCancel(ctx), execution.endpointBase, providerID); cleanupErr != nil {
				result = nil
				resultErr = stderrors.Join(resultErr, fmt.Errorf("remove agent invocation runtime authentication: %w", cleanupErr))
			}
		}()
	}
	if err != nil {
		return nil, err
	}

	sessionID, err := e.ensureOpenCodeSession(ctx, execution)
	if err != nil {
		return e.canceledResult(ctx, execution, contract, sessionID, err)
	}
	messageID := deterministicOpenCodeID("msg", execution.arguments.InvocationID)
	response, err := e.findOpenCodeResponse(ctx, execution.endpointBase, sessionID, messageID)
	if err != nil {
		return nil, err
	}
	if response == nil {
		response, err = e.promptOpenCode(ctx, execution, providerID, sessionID, messageID)
		if err != nil {
			return e.canceledResult(ctx, execution, contract, sessionID, err)
		}
	}
	content, err := assistantText(response)
	if err != nil {
		return nil, err
	}
	assistant, err := e.store.CreateAgentAssistantMessage(ctx, &iapiserver.AgentMessage{
		ObjectMeta:   imachinery.ObjectMeta{ID: deterministicUUID("agent-invocation-assistant", execution.arguments.InvocationID)},
		SessionID:    execution.session.ID,
		AgentID:      execution.agent.ID,
		InvocationID: execution.invocation.ID,
		Role:         iapiserver.AgentMessageRoleAssistant,
		Content:      content,
	})
	if err != nil {
		return nil, fmt.Errorf("persist agent assistant message: %w", err)
	}
	execution.sequence, err = e.appendEvent(ctx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeMessageCompleted)
	if err != nil {
		return nil, fmt.Errorf("persist agent message completion event: %w", err)
	}
	execution.sequence, err = e.appendEvent(ctx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeInvocationCompleted)
	if err != nil {
		return nil, fmt.Errorf("persist agent invocation completion event: %w", err)
	}
	result = invocationResult(execution, sessionID, response.Info.ID, iapiserver.AgentInvocationStatusSucceeded, assistant.ID, "", "")
	if err := e.registry.ValidateOutput(contract, result); err != nil {
		return nil, fmt.Errorf("validate agent invocation output: %w", err)
	}
	return result, nil
}

// executeHermes speaks the released Hermes JSON-RPC/WebSocket contract. The
// runtime owns the conversation state; this worker only translates terminal
// events into the canonical Invocation result and durable operation events.
func (e *InvocationExecutor) executeHermes(
	ctx context.Context,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
	contract *taskfunctionregistry.Contract,
	execution *invocationExecution,
) (map[string]any, error) {
	if completed, result, err := e.completedHermesResult(ctx, execution, contract); completed || err != nil {
		return result, err
	}

	var err error
	execution.sequence, err = e.appendEvent(ctx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeInvocationStarted)
	if err != nil {
		return nil, fmt.Errorf("persist agent invocation start event: %w", err)
	}

	conn, err := dialHermes(ctx, execution.endpointBase)
	if err != nil {
		return nil, fmt.Errorf("connect to Hermes runtime: %w", err)
	}
	defer conn.Close()

	sessionRef := ""
	if execution.arguments.ResumeRuntimeSessionRef != nil {
		sessionRef = strings.TrimSpace(*execution.arguments.ResumeRuntimeSessionRef)
	}
	if sessionRef == "" {
		response, callErr := hermesCall(ctx, conn, "session.create", map[string]any{
			"session_id": execution.session.ID,
			"metadata":   map[string]any{"omnimam_invocation_id": execution.arguments.InvocationID},
		})
		if callErr != nil {
			interruptHermes(ctx, conn, sessionRef, "")
			return e.hermesCanceledResult(ctx, execution, contract, sessionRef, "", callErr)
		}
		sessionRef = hermesString(response, "session_id", "session", "id")
		if sessionRef == "" {
			return nil, fmt.Errorf("Hermes session.create returned no session reference")
		}
	}

	invocationRef := ""
	if execution.arguments.ResumeRuntimeInvocationRef != nil {
		invocationRef = strings.TrimSpace(*execution.arguments.ResumeRuntimeInvocationRef)
	}
	if invocationRef == "" {
		response, callErr := hermesCall(ctx, conn, "prompt.submit", map[string]any{
			"session_id":    sessionRef,
			"invocation_id": execution.arguments.InvocationID,
			"prompt":        execution.message.Content,
		})
		if callErr != nil {
			interruptHermes(ctx, conn, sessionRef, invocationRef)
			return e.hermesCanceledResult(ctx, execution, contract, sessionRef, invocationRef, callErr)
		}
		invocationRef = hermesString(response, "invocation_id", "id", "invocation")
	}

	content, eventInvocationRef, readErr := readHermesCompletion(ctx, conn, sessionRef, invocationRef)
	if readErr != nil {
		interruptHermes(ctx, conn, sessionRef, invocationRef)
		return e.hermesCanceledResult(ctx, execution, contract, sessionRef, invocationRef, readErr)
	}
	if invocationRef == "" {
		invocationRef = eventInvocationRef
	}
	if invocationRef == "" {
		invocationRef = deterministicOpenCodeID("inv", execution.arguments.InvocationID)
	}
	assistant, err := e.store.CreateAgentAssistantMessage(ctx, &iapiserver.AgentMessage{
		ObjectMeta:   imachinery.ObjectMeta{ID: deterministicUUID("agent-invocation-assistant", execution.arguments.InvocationID)},
		SessionID:    execution.session.ID,
		AgentID:      execution.agent.ID,
		InvocationID: execution.invocation.ID,
		Role:         iapiserver.AgentMessageRoleAssistant,
		Content:      content,
	})
	if err != nil {
		return nil, fmt.Errorf("persist agent assistant message: %w", err)
	}
	execution.sequence, err = e.appendEvent(ctx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeMessageCompleted)
	if err != nil {
		return nil, fmt.Errorf("persist agent message completion event: %w", err)
	}
	execution.sequence, err = e.appendEvent(ctx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeInvocationCompleted)
	if err != nil {
		return nil, fmt.Errorf("persist agent invocation completion event: %w", err)
	}
	result := invocationResult(execution, sessionRef, invocationRef, iapiserver.AgentInvocationStatusSucceeded, assistant.ID, "", "")
	if err := e.registry.ValidateOutput(contract, result); err != nil {
		return nil, fmt.Errorf("validate agent invocation output: %w", err)
	}
	return result, nil
}

func dialHermes(ctx context.Context, baseURL string) (*websocket.Conn, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/ws")
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported Hermes endpoint scheme %q", u.Scheme)
	}
	origin := (&url.URL{Scheme: "http", Host: u.Host}).String()
	config, err := websocket.NewConfig(u.String(), origin)
	if err != nil {
		return nil, err
	}
	config.Header.Set("User-Agent", "omnimam-agent-executor")
	conn, err := websocket.DialConfig(config)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	return conn, nil
}

func hermesCall(ctx context.Context, conn *websocket.Conn, method string, params map[string]any) (json.RawMessage, error) {
	id := deterministicOpenCodeID("rpc", uuid.NewString())
	payload, err := json.Marshal(hermesRPCMessage{JSONRPC: "2.0", ID: id, Method: method, Params: mustJSON(params)})
	if err != nil {
		return nil, err
	}
	if err := websocket.Message.Send(conn, string(payload)); err != nil {
		return nil, err
	}
	for {
		message, err := receiveHermesMessage(ctx, conn)
		if err != nil {
			return nil, err
		}
		if message.ID != id {
			continue
		}
		if len(message.Error) > 0 && string(message.Error) != "null" {
			return nil, fmt.Errorf("Hermes %s failed: %s", method, strings.TrimSpace(string(message.Error)))
		}
		return message.Result, nil
	}
}

func interruptHermes(ctx context.Context, conn *websocket.Conn, sessionRef, invocationRef string) {
	if conn == nil || (ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded) {
		return
	}
	interruptCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openCodeCleanupTimeout)
	defer cancel()
	_ = hermesNotify(interruptCtx, conn, "session.interrupt", map[string]any{
		"session_id": sessionRef, "invocation_id": invocationRef,
	})
}

func hermesNotify(ctx context.Context, conn *websocket.Conn, method string, params map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := json.Marshal(hermesRPCMessage{JSONRPC: "2.0", Method: method, Params: mustJSON(params)})
	if err != nil {
		return err
	}
	return websocket.Message.Send(conn, string(payload))
}

func receiveHermesMessage(ctx context.Context, conn *websocket.Conn) (*hermesRPCMessage, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var frame string
		if err := websocket.Message.Receive(conn, &frame); err != nil {
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}
			return nil, err
		}
		for _, line := range strings.Split(frame, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var message hermesRPCMessage
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				return nil, fmt.Errorf("decode Hermes message: %w", err)
			}
			return &message, nil
		}
	}
}

func readHermesCompletion(ctx context.Context, conn *websocket.Conn, sessionRef, invocationRef string) (string, string, error) {
	for {
		message, err := receiveHermesMessage(ctx, conn)
		if err != nil {
			return "", invocationRef, err
		}
		if message.Error != nil && len(message.Error) > 0 && string(message.Error) != "null" {
			return "", invocationRef, fmt.Errorf("Hermes invocation failed: %s", strings.TrimSpace(string(message.Error)))
		}
		if message.ID != "" && message.Method == "" {
			if ref := hermesString(message.Result, "invocation_id", "id", "invocation"); ref != "" {
				invocationRef = ref
			}
			continue
		}
		if message.Method == "error" {
			return "", invocationRef, fmt.Errorf("Hermes invocation failed: %s", strings.TrimSpace(string(message.Params)))
		}
		if message.Method != "message.complete" {
			if message.Method == "status.update" {
				status := strings.ToLower(hermesString(message.Params, "status", "state"))
				if status == "failed" || status == "error" {
					return "", invocationRef, fmt.Errorf("Hermes invocation status %s", status)
				}
			}
			continue
		}
		if ref := hermesString(message.Params, "invocation_id", "id", "invocation"); ref != "" {
			invocationRef = ref
		}
		content := hermesString(message.Params, "content", "text", "message")
		if content == "" {
			content = hermesString(message.Result, "content", "text", "message")
		}
		if content == "" {
			return "", invocationRef, fmt.Errorf("Hermes message.complete returned no assistant text")
		}
		_ = sessionRef
		return content, invocationRef, nil
	}
}

func (e *InvocationExecutor) hermesCanceledResult(ctx context.Context, execution *invocationExecution, contract *taskfunctionregistry.Contract, sessionRef, invocationRef string, cause error) (map[string]any, error) {
	if stderrors.Is(cause, context.Canceled) || stderrors.Is(ctx.Err(), context.Canceled) {
		result := invocationResult(execution, sessionRef, invocationRef, iapiserver.AgentInvocationStatusCanceled, "", "", "")
		execution.sequence, _ = e.appendEvent(context.WithoutCancel(ctx), execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeInvocationCanceled)
		if err := e.registry.ValidateOutput(contract, result); err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, cause
}

func (e *InvocationExecutor) completedHermesResult(ctx context.Context, execution *invocationExecution, contract *taskfunctionregistry.Contract) (bool, map[string]any, error) {
	events, err := e.store.ListAgentOperationEvents(ctx, execution.arguments.InvocationID, execution.arguments.EventSequenceAfter)
	if err != nil {
		return false, nil, fmt.Errorf("load agent invocation operation events: %w", err)
	}
	completed := false
	for _, event := range events {
		if event.SequenceNo > execution.sequence {
			execution.sequence = event.SequenceNo
		}
		completed = completed || event.EventType == iapiserver.AgentOperationEventTypeInvocationCompleted
	}
	if !completed {
		return false, nil, nil
	}
	assistant, err := e.store.GetAgentMessage(ctx, deterministicUUID("agent-invocation-assistant", execution.arguments.InvocationID), execution.claims.OwnerUserID)
	if err != nil {
		return false, nil, fmt.Errorf("load completed Hermes assistant message: %w", err)
	}
	sessionRef := execution.invocation.RuntimeSessionRef
	if sessionRef == "" && execution.arguments.ResumeRuntimeSessionRef != nil {
		sessionRef = *execution.arguments.ResumeRuntimeSessionRef
	}
	invocationRef := execution.invocation.RuntimeInvocationRef
	if invocationRef == "" && execution.arguments.ResumeRuntimeInvocationRef != nil {
		invocationRef = *execution.arguments.ResumeRuntimeInvocationRef
	}
	result := invocationResult(execution, sessionRef, invocationRef, iapiserver.AgentInvocationStatusSucceeded, assistant.ID, "", "")
	if err := e.registry.ValidateOutput(contract, result); err != nil {
		return false, nil, err
	}
	return true, result, nil
}

func hermesString(raw json.RawMessage, keys ...string) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return findHermesString(value, keys...)
}

func findHermesString(value any, keys ...string) string {
	if object, ok := value.(map[string]any); ok {
		for _, key := range keys {
			if candidate, ok := object[key].(string); ok && strings.TrimSpace(candidate) != "" {
				return strings.TrimSpace(candidate)
			}
		}
		for _, child := range object {
			if found := findHermesString(child, keys...); found != "" {
				return found
			}
		}
	}
	if list, ok := value.([]any); ok {
		for _, child := range list {
			if found := findHermesString(child, keys...); found != "" {
				return found
			}
		}
	}
	return ""
}

func mustJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func (e *InvocationExecutor) prepare(
	ctx context.Context,
	workerTask workflowruntime.WorkerTask,
	atomicTask *iapiserver.AtomicTask,
) (*taskfunctionregistry.Contract, *invocationExecution, error) {
	if atomicTask == nil || workerTask.AtomicTaskID == "" || workerTask.AtomicTaskID != atomicTask.ID {
		return nil, nil, fmt.Errorf("agent invocation task identity does not match")
	}
	if workerTask.FunctionRef != "" && workerTask.FunctionRef != iapiserver.TaskWorkerFunctionAgentInvocationExecute {
		return nil, nil, fmt.Errorf("agent invocation worker function does not match")
	}
	if atomicTask.FunctionRef != iapiserver.TaskWorkerFunctionAgentInvocationExecute || atomicTask.FunctionContractVersion == "" || atomicTask.FunctionContractDigest == "" {
		return nil, nil, fmt.Errorf("agent invocation task contract pin is invalid")
	}
	if !equivalentArguments(workerTask.Arguments, atomicTask.Arguments) {
		return nil, nil, fmt.Errorf("agent invocation worker arguments do not match the atomic task")
	}
	contract, err := e.registry.Resolve(atomicTask.FunctionRef, atomicTask.FunctionContractVersion, atomicTask.FunctionContractDigest)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve agent invocation contract: %w", err)
	}
	raw, err := json.Marshal(atomicTask.Arguments)
	if err != nil {
		return nil, nil, fmt.Errorf("encode agent invocation arguments: %w", err)
	}
	var arguments invocationArguments
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, nil, fmt.Errorf("decode agent invocation arguments: %w", err)
	}
	if arguments.AgentID == "" || arguments.SessionID == "" || arguments.InvocationID == "" || arguments.RuntimeBindingID == "" ||
		arguments.ExpectedResourceVersion < 0 || arguments.EventSequenceAfter < 0 {
		return nil, nil, fmt.Errorf("agent invocation arguments are incomplete")
	}
	if arguments.InvocationType != iapiserver.AgentInvocationTypeChat && arguments.InvocationType != iapiserver.AgentInvocationTypeCoding {
		return nil, nil, fmt.Errorf("agent invocation type is invalid")
	}
	var claims agentgrant.InvocationClaims
	if err := e.grants.Resolve(arguments.AuthorizationRef, iapiserver.TaskWorkerRefPrefixAgentInvocationGrant, &claims); err != nil {
		return nil, nil, fmt.Errorf("resolve agent invocation authorization: %w", err)
	}
	if err := agentgrant.ValidateWindow(claims.IssuedAt, claims.ExpiresAt); err != nil {
		return nil, nil, fmt.Errorf("validate agent invocation authorization: %w", err)
	}
	if claims.AgentID != arguments.AgentID || claims.SessionID != arguments.SessionID || claims.InvocationID != arguments.InvocationID ||
		claims.RuntimeBindingID != arguments.RuntimeBindingID || claims.InvocationType != arguments.InvocationType ||
		claims.ExpectedResourceVersion != arguments.ExpectedResourceVersion || claims.OwnerUserID != atomicTask.CreatedBy {
		return nil, nil, fmt.Errorf("agent invocation authorization scope does not match the task")
	}

	execution := &invocationExecution{arguments: arguments, claims: claims, sequence: arguments.EventSequenceAfter}
	if execution.agent, err = e.store.GetAgent(ctx, arguments.AgentID, claims.OwnerUserID); err != nil {
		return nil, nil, fmt.Errorf("load agent invocation agent: %w", err)
	}
	if execution.agent.Disabled || execution.agent.Kind != iapiserver.AgentKindCoding || execution.agent.WorkspaceType != iapiserver.AgentWorkspaceTypeStudio ||
		(execution.agent.Status != iapiserver.AgentStatusReady && execution.agent.Status != iapiserver.AgentStatusIdle) ||
		execution.agent.WorkspaceID != claims.WorkspaceID {
		return nil, nil, fmt.Errorf("agent invocation agent fence is invalid")
	}
	if arguments.InvocationType != iapiserver.AgentInvocationTypeChat && arguments.InvocationType != iapiserver.AgentInvocationTypeCoding {
		return nil, nil, fmt.Errorf("agent invocation type is unsupported")
	}
	if execution.session, err = e.store.GetAgentSession(ctx, arguments.SessionID, claims.OwnerUserID); err != nil {
		return nil, nil, fmt.Errorf("load agent invocation session: %w", err)
	}
	if execution.session.AgentID != arguments.AgentID || execution.session.Status != iapiserver.AgentSessionStatusOpen {
		return nil, nil, fmt.Errorf("agent invocation session fence is invalid")
	}
	if execution.invocation, err = e.store.GetAgentInvocation(ctx, arguments.InvocationID, claims.OwnerUserID); err != nil {
		return nil, nil, fmt.Errorf("load agent invocation: %w", err)
	}
	if execution.invocation.AgentID != arguments.AgentID || execution.invocation.SessionID != arguments.SessionID ||
		execution.invocation.Type != arguments.InvocationType || execution.invocation.AtomicTaskID == nil || *execution.invocation.AtomicTaskID != atomicTask.ID ||
		execution.invocation.TaskExpectedResourceVersion == nil || *execution.invocation.TaskExpectedResourceVersion != arguments.ExpectedResourceVersion ||
		execution.invocation.ResourceVersion != arguments.ExpectedResourceVersion || execution.invocation.RuntimeBindingID != arguments.RuntimeBindingID {
		return nil, nil, fmt.Errorf("agent invocation resource fence is invalid")
	}
	if execution.message, err = e.store.GetAgentMessage(ctx, execution.invocation.UserMessageID, claims.OwnerUserID); err != nil {
		return nil, nil, fmt.Errorf("load agent invocation user message: %w", err)
	}
	if execution.message.AgentID != arguments.AgentID || execution.message.SessionID != arguments.SessionID ||
		execution.message.InvocationID != arguments.InvocationID || execution.message.Role != iapiserver.AgentMessageRoleUser {
		return nil, nil, fmt.Errorf("agent invocation message fence is invalid")
	}
	workspace, err := e.store.GetAgentWorkspaceBinding(ctx, arguments.AgentID, claims.OwnerUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("load agent invocation workspace binding: %w", err)
	}
	expectedWorkspaceType := iapiserver.AgentWorkspaceTypeAgent
	if arguments.InvocationType == iapiserver.AgentInvocationTypeCoding {
		expectedWorkspaceType = iapiserver.AgentWorkspaceTypeStudio
	}
	if workspace.AgentID != arguments.AgentID || workspace.WorkspaceType != expectedWorkspaceType ||
		workspace.WorkspaceID != claims.WorkspaceID || workspace.AccessMode != iapiserver.AgentWorkspaceAccessModeReadWrite {
		return nil, nil, fmt.Errorf("agent invocation workspace fence is invalid")
	}
	if execution.runtime, err = e.store.GetAgentRuntimeByID(ctx, arguments.RuntimeBindingID); err != nil {
		return nil, nil, fmt.Errorf("load agent invocation runtime: %w", err)
	}
	expectedProfile := iapiserver.AgentProfileIDHermes
	if arguments.InvocationType == iapiserver.AgentInvocationTypeCoding {
		expectedProfile = iapiserver.AgentProfileIDCoding
	}
	if execution.runtime.AgentID != arguments.AgentID || execution.runtime.RuntimeProfileID != expectedProfile ||
		execution.runtime.State != iapiserver.AgentRuntimeStateReady || execution.runtime.HealthStatus != iapiserver.AgentRuntimeHealthHealthy ||
		execution.runtime.ActivityState == iapiserver.AgentRuntimeActivitySuspended || execution.runtime.InfraRuntimeID == "" ||
		!strings.HasPrefix(execution.runtime.EndpointRef, iapiserver.TaskWorkerRefPrefixInfraEndpoint) {
		return nil, nil, fmt.Errorf("agent invocation runtime fence is invalid")
	}
	binding, err := primaryInvocationModelBinding(ctx, e.store, execution.agent, claims.OwnerUserID, arguments.InvocationType)
	if err != nil {
		return nil, nil, err
	}
	usage := "agent.chat"
	if arguments.InvocationType == iapiserver.AgentInvocationTypeCoding {
		usage = "agent.coding"
	}
	execution.model, err = e.models.ResolveAgentModelAccessGrant(ctx, claims.ModelAccessGrantRef, claims.OwnerUserID, claims.AgentID, usage)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve agent invocation model authorization: %w", err)
	}
	if execution.model.OwnerUserID != claims.OwnerUserID || execution.model.ModelID == "" || execution.model.RemoteModel == "" ||
		execution.model.ProviderType == "" || execution.model.Endpoint == "" {
		return nil, nil, fmt.Errorf("agent invocation model fence is invalid")
	}
	switch binding.SourceType {
	case iapiserver.AgentModelBindingSourceTypeUserDefault:
		if binding.SourceRef != iapiserver.AgentModelBindingSourceRefUserDefault {
			return nil, nil, fmt.Errorf("agent invocation default model binding is invalid")
		}
	case iapiserver.AgentModelBindingSourceTypeUserProvider:
		if binding.SourceRef == "" || execution.model.ModelID != binding.SourceRef {
			return nil, nil, fmt.Errorf("agent invocation provider model binding is invalid")
		}
	case iapiserver.AgentModelBindingSourceTypePlatform:
		return nil, nil, fmt.Errorf("agent invocation platform model binding is unsupported")
	default:
		return nil, nil, fmt.Errorf("agent invocation model binding source is invalid")
	}
	if execution.model.AuthenticationType != iapiserver.EngineAuthAPIKey {
		return nil, nil, fmt.Errorf("agent invocation model authentication is unsupported")
	}
	execution.credential, err = e.credentials.ResolveCredential(ctx, modelgateway.CredentialResolveRequest{
		Handle: execution.model.CredentialHandle, ProviderType: execution.model.ProviderType,
		AuthenticationType: execution.model.AuthenticationType,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("resolve agent invocation model credential: %w", err)
	}
	apiKey, _ := execution.credential.Authentication["api_key"].(string)
	if strings.TrimSpace(apiKey) == "" {
		return nil, nil, fmt.Errorf("agent invocation model credential is invalid")
	}
	endpointID := strings.TrimPrefix(execution.runtime.EndpointRef, iapiserver.TaskWorkerRefPrefixInfraEndpoint)
	resolved, err := e.endpoints.ResolveEndpoint(ctx, endpointID, &iapiserver.InfraResolveEndpointRequest{
		OwnerReference:     execution.runtime.ID,
		Purpose:            iapiserver.InfraResolveEndpointPurposeAgentRuntimeAdapter,
		AuditCorrelationID: arguments.InvocationID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("resolve agent invocation runtime endpoint: %w", err)
	}
	if resolved.EndpointRef != execution.runtime.EndpointRef || resolved.RuntimeID != execution.runtime.InfraRuntimeID {
		return nil, nil, fmt.Errorf("agent invocation runtime endpoint fence is invalid")
	}
	execution.endpointBase = strings.TrimRight(resolved.BaseURL, "/")
	return contract, execution, nil
}

func primaryInvocationModelBinding(ctx context.Context, store InvocationStore, agent *iapiserver.Agent, owner, invocationType string) (*iapiserver.AgentModelBinding, error) {
	bindings, err := store.ListAgentModelBindings(ctx, agent.ID, owner)
	if err != nil {
		return nil, fmt.Errorf("load agent invocation model bindings: %w", err)
	}
	var primary *iapiserver.AgentModelBinding
	for _, binding := range bindings {
		if binding != nil && binding.Status == iapiserver.AgentModelBindingStatusActive && binding.IsPrimary {
			if primary != nil {
				return nil, fmt.Errorf("agent invocation has multiple active primary model bindings")
			}
			primary = binding
		}
	}
	expectedPurpose := iapiserver.AgentModelBindingPurposeChat
	if invocationType == iapiserver.AgentInvocationTypeCoding {
		expectedPurpose = iapiserver.AgentModelBindingPurposeCoding
	}
	if primary == nil || primary.AgentID != agent.ID || primary.Purpose != expectedPurpose {
		return nil, fmt.Errorf("agent invocation primary model binding is invalid")
	}
	return primary, nil
}

func (e *InvocationExecutor) configureOpenCode(ctx context.Context, execution *invocationExecution, providerID string) (bool, error) {
	apiKey, _ := execution.credential.Authentication["api_key"].(string)
	if err := invokeOpenCode(ctx, execution.endpointBase, http.MethodPut, "/auth/"+url.PathEscape(providerID), map[string]any{
		"type": "api", "key": apiKey,
	}, nil); err != nil {
		return false, fmt.Errorf("configure agent invocation runtime authentication: %w", err)
	}
	config := map[string]any{"provider": map[string]any{providerID: map[string]any{
		"id": providerID, "name": "OmniMAM invocation", "npm": "@ai-sdk/openai-compatible",
		"options": map[string]any{"baseURL": execution.model.Endpoint},
		"models": map[string]any{execution.model.RemoteModel: map[string]any{
			"id": execution.model.RemoteModel, "name": execution.model.RemoteModel,
		}},
	}}}
	if err := invokeOpenCode(ctx, execution.endpointBase, http.MethodPatch, "/global/config", config, nil); err != nil {
		return true, fmt.Errorf("configure agent invocation runtime model: %w", err)
	}
	return true, nil
}

func (e *InvocationExecutor) ensureOpenCodeSession(ctx context.Context, execution *invocationExecution) (string, error) {
	if execution.arguments.ResumeRuntimeSessionRef != nil && strings.HasPrefix(*execution.arguments.ResumeRuntimeSessionRef, "ses") {
		return *execution.arguments.ResumeRuntimeSessionRef, nil
	}
	sessionID, found, err := e.findOpenCodeSession(ctx, execution)
	if err != nil {
		return "", err
	}
	if found {
		return sessionID, nil
	}
	var session openCodeSession
	if err := invokeOpenCode(ctx, execution.endpointBase, http.MethodPost, "/session", map[string]any{
		"title": "OmniMAM invocation", "metadata": map[string]any{"omnimam_invocation_id": execution.arguments.InvocationID},
	}, &session); err != nil {
		return "", fmt.Errorf("create agent invocation runtime session: %w", err)
	}
	if !strings.HasPrefix(session.ID, "ses") {
		return "", fmt.Errorf("agent invocation runtime returned an invalid session")
	}
	return session.ID, nil
}

func (e *InvocationExecutor) findOpenCodeSession(ctx context.Context, execution *invocationExecution) (string, bool, error) {
	var sessions []openCodeSession
	if err := invokeOpenCode(ctx, execution.endpointBase, http.MethodGet, "/session?limit=200", nil, &sessions); err != nil {
		return "", false, fmt.Errorf("list agent invocation runtime sessions: %w", err)
	}
	for _, session := range sessions {
		if value, _ := session.Metadata["omnimam_invocation_id"].(string); value == execution.arguments.InvocationID && strings.HasPrefix(session.ID, "ses") {
			return session.ID, true, nil
		}
	}
	return "", false, nil
}

func (e *InvocationExecutor) findOpenCodeResponse(ctx context.Context, baseURL, sessionID, userMessageID string) (*openCodeMessage, error) {
	var messages []openCodeMessage
	if err := invokeOpenCode(ctx, baseURL, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/message?limit=200", nil, &messages); err != nil {
		return nil, fmt.Errorf("list agent invocation runtime messages: %w", err)
	}
	for index := range messages {
		message := &messages[index]
		if message.Info.Role == "assistant" && message.Info.ParentID == userMessageID {
			if len(message.Info.Error) > 0 && string(message.Info.Error) != "null" {
				return nil, fmt.Errorf("agent invocation runtime message failed")
			}
			if _, err := assistantText(message); err == nil {
				return message, nil
			}
		}
	}
	return nil, nil
}

func (e *InvocationExecutor) promptOpenCode(ctx context.Context, execution *invocationExecution, providerID, sessionID, messageID string) (*openCodeMessage, error) {
	var response openCodeMessage
	err := invokeOpenCode(ctx, execution.endpointBase, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/message", map[string]any{
		"messageID": messageID,
		"model":     map[string]any{"providerID": providerID, "modelID": execution.model.RemoteModel},
		"parts":     []map[string]any{{"type": "text", "text": execution.message.Content}},
	}, &response)
	if err != nil {
		return nil, fmt.Errorf("execute agent invocation runtime message: %w", err)
	}
	if response.Info.Role != "assistant" || response.Info.ParentID != messageID || !strings.HasPrefix(response.Info.ID, "msg") {
		return nil, fmt.Errorf("agent invocation runtime returned an invalid message")
	}
	if len(response.Info.Error) > 0 && string(response.Info.Error) != "null" {
		return nil, fmt.Errorf("agent invocation runtime message failed")
	}
	return &response, nil
}

func (e *InvocationExecutor) canceledResult(ctx context.Context, execution *invocationExecution, contract *taskfunctionregistry.Contract, sessionID string, cause error) (map[string]any, error) {
	if !stderrors.Is(cause, context.Canceled) && !stderrors.Is(ctx.Err(), context.Canceled) {
		return nil, cause
	}
	var cleanupErrors []error
	if sessionID != "" {
		abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openCodeCleanupTimeout)
		if err := invokeOpenCode(abortCtx, execution.endpointBase, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/abort", nil, nil); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("abort canceled agent invocation runtime session: %w", err))
		}
		cancel()
	}
	eventCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openCodeCleanupTimeout)
	sequence, err := e.appendEvent(eventCtx, execution.arguments.InvocationID, execution.sequence, iapiserver.AgentOperationEventTypeInvocationCanceled)
	cancel()
	if err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("persist agent invocation cancellation event: %w", err))
	} else {
		execution.sequence = sequence
	}
	result := invocationResult(execution, sessionID, "", iapiserver.AgentInvocationStatusCanceled, "", "", "")
	if err := e.registry.ValidateOutput(contract, result); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("validate canceled agent invocation output: %w", err))
	}
	if err := stderrors.Join(cleanupErrors...); err != nil {
		return nil, stderrors.Join(cause, err)
	}
	return result, nil
}

func (e *InvocationExecutor) completedResult(ctx context.Context, execution *invocationExecution, contract *taskfunctionregistry.Contract) (bool, map[string]any, error) {
	events, err := e.store.ListAgentOperationEvents(ctx, execution.arguments.InvocationID, execution.arguments.EventSequenceAfter)
	if err != nil {
		return false, nil, fmt.Errorf("load agent invocation operation events: %w", err)
	}
	completed := false
	for _, event := range events {
		if event.SequenceNo > execution.sequence {
			execution.sequence = event.SequenceNo
		}
		if event.EventType == iapiserver.AgentOperationEventTypeInvocationCompleted {
			completed = true
		}
	}
	if !completed {
		return false, nil, nil
	}
	assistantID := deterministicUUID("agent-invocation-assistant", execution.arguments.InvocationID)
	assistant, err := e.store.GetAgentMessage(ctx, assistantID, execution.claims.OwnerUserID)
	if err != nil {
		return false, nil, fmt.Errorf("load completed agent assistant message: %w", err)
	}
	if assistant.AgentID != execution.agent.ID || assistant.SessionID != execution.session.ID || assistant.InvocationID != execution.invocation.ID || assistant.Role != iapiserver.AgentMessageRoleAssistant {
		return false, nil, fmt.Errorf("completed agent assistant message fence is invalid")
	}
	sessionRef := execution.invocation.RuntimeSessionRef
	invocationRef := execution.invocation.RuntimeInvocationRef
	if sessionRef == "" {
		var found bool
		sessionRef, found, err = e.findOpenCodeSession(ctx, execution)
		if err != nil {
			return false, nil, fmt.Errorf("recover completed agent runtime session: %w", err)
		}
		if !found {
			return false, nil, fmt.Errorf("completed agent runtime session is unavailable")
		}
	}
	if invocationRef == "" {
		messageID := deterministicOpenCodeID("msg", execution.arguments.InvocationID)
		response, responseErr := e.findOpenCodeResponse(ctx, execution.endpointBase, sessionRef, messageID)
		if responseErr != nil {
			return false, nil, fmt.Errorf("recover completed agent runtime response: %w", responseErr)
		}
		if response == nil {
			return false, nil, fmt.Errorf("completed agent runtime response is unavailable")
		}
		invocationRef = response.Info.ID
	}
	result := invocationResult(execution, sessionRef, invocationRef, iapiserver.AgentInvocationStatusSucceeded, assistant.ID, "", "")
	if err := e.registry.ValidateOutput(contract, result); err != nil {
		return false, nil, fmt.Errorf("validate completed agent invocation output: %w", err)
	}
	return true, result, nil
}

func (e *InvocationExecutor) appendEvent(ctx context.Context, invocationID string, after int, eventType string) (int, error) {
	sequence := after + 1
	created, err := e.store.AppendAgentOperationEvent(ctx, &iapiserver.AgentOperationEvent{
		ObjectMeta:   imachinery.ObjectMeta{ID: deterministicUUID("agent-operation-event", fmt.Sprintf("%s:%d", invocationID, sequence))},
		InvocationID: invocationID,
		EventType:    eventType,
		SequenceNo:   sequence,
		Payload:      json.RawMessage("{}"),
	})
	if err != nil {
		return after, err
	}
	if created.EventType != eventType {
		return after, fmt.Errorf("agent invocation event sequence conflicts")
	}
	return created.SequenceNo, nil
}

func (e *InvocationExecutor) removeOpenCodeAuth(ctx context.Context, baseURL, providerID string) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, openCodeCleanupTimeout)
	defer cancel()
	if err := invokeOpenCode(cleanupCtx, baseURL, http.MethodDelete, "/auth/"+url.PathEscape(providerID), nil, nil, http.StatusNotFound); err != nil {
		return fmt.Errorf("remove runtime authentication: %w", err)
	}
	return nil
}

func invokeOpenCode(ctx context.Context, baseURL, method, path string, payload, output any, acceptedStatuses ...int) error {
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(strings.TrimRight(baseURL, "/")+path).WithMethod(method).
		AddHeaderParam("Accept", "application/json")
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode runtime request: %w", err)
		}
		builder.WithBody("json", json.RawMessage(raw)).AddHeaderParam("Content-Type", "application/json")
	}
	response, err := builder.Build().InvokeWithContext(ctx, httpcli.TimeoutCallOption(openCodeRequestTimeout))
	if err != nil {
		return fmt.Errorf("runtime request failed: %w", err)
	}
	statusCode := response.GetStatusCode()
	statusAccepted := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
	for _, accepted := range acceptedStatuses {
		statusAccepted = statusAccepted || statusCode == accepted
	}
	if !statusAccepted {
		body := strings.TrimSpace(response.GetBody())
		if len(body) > 1024 {
			body = body[:1024] + "..."
		}
		if body == "" {
			return fmt.Errorf("runtime request returned status %d", statusCode)
		}
		return fmt.Errorf("runtime request returned status %d: %s", statusCode, body)
	}
	if output == nil || strings.TrimSpace(response.GetBody()) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(response.GetBody()), output); err != nil {
		return fmt.Errorf("decode runtime response: %w", err)
	}
	return nil
}

func assistantText(message *openCodeMessage) (string, error) {
	if message == nil {
		return "", fmt.Errorf("agent invocation runtime response is missing")
	}
	var parts []string
	for _, part := range message.Parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			parts = append(parts, part.Text)
		}
	}
	content := strings.TrimSpace(strings.Join(parts, "\n"))
	if content == "" {
		return "", fmt.Errorf("agent invocation runtime response has no assistant text")
	}
	return content, nil
}

func invocationResult(execution *invocationExecution, sessionRef, invocationRef, status, assistantID, failureCode, failureMessage string) map[string]any {
	result := map[string]any{
		"invocation_id": execution.arguments.InvocationID, "invocation_status": status,
		"last_event_sequence": execution.sequence,
	}
	if sessionRef != "" {
		result["runtime_session_ref"] = sessionRef
	}
	if invocationRef != "" {
		result["runtime_invocation_ref"] = invocationRef
	}
	if assistantID != "" {
		result["assistant_message_id"] = assistantID
	}
	if failureCode != "" {
		result["failure_code"] = failureCode
	}
	if failureMessage != "" {
		result["failure_message"] = failureMessage
	}
	return result
}

func equivalentArguments(left, right map[string]any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && (bytes.Equal(leftRaw, rightRaw) || reflect.DeepEqual(left, right))
}

func deterministicOpenCodeID(prefix, value string) string {
	id := strings.ReplaceAll(deterministicUUID(prefix, value), "-", "")
	return prefix + "_" + id
}

func deterministicUUID(scope, value string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(scope+":"+value)).String()
}
