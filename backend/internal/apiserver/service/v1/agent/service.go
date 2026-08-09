package agent

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentgrant"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

const (
	agentRuntimeIdleTimeout     = 30 * time.Minute
	agentRuntimeMaximumLifetime = 8 * time.Hour
)

// TaskClient 是 Agent 消费的 Task Center 小接口，只允许创建和操作受控任务。
type TaskClient interface {
	CreateDomainAtomicTask(context.Context, string, *iapiserver.AtomicTaskCreateRequest) (*iapiserver.AtomicTask, error)
	GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error)
	CancelAtomicTask(context.Context, string, *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error)
}

// WorkspaceBindingValidator 由 AppStudio 提供 Coding Agent 固定 Workspace 授权校验。
type WorkspaceBindingValidator interface {
	ValidateAgentWorkspaceBinding(context.Context, string, string) (*iapiserver.AgentAuthorizationSummary, error)
}

// ModelAccessResolver 将 Agent ModelBinding 转换为不含明文凭证的 ModelAccessSpec 引用。
type ModelAccessResolver interface {
	ResolveAgentModelAccess(context.Context, string, *iapiserver.AgentModelBinding) (string, error)
}

// WorkloadScopeResolver resolves the current AppStudio application and Coding
// Agent generation without allowing Agent to read AppStudio tables.
type WorkloadScopeResolver interface {
	ResolveAgentWorkloadScope(context.Context, string, string) (string, int64, error)
}

// Service 实现 released Agent API、固定 Workspace、交互持久化和 Runtime Task 编排。
type Service struct {
	store      store.AgentStore
	tasks      TaskClient
	workspaces WorkspaceBindingValidator
	models     ModelAccessResolver
	scopes     WorkloadScopeResolver
	grants     *agentgrant.Codec
	profiles   map[string]iapiserver.AgentProfile
}

// Dependencies 是 Agent service 显式消费方依赖。
type Dependencies struct {
	Store      store.AgentStore
	Tasks      TaskClient
	Workspaces WorkspaceBindingValidator
	Models     ModelAccessResolver
	Scopes     WorkloadScopeResolver
	Grants     *agentgrant.Codec
}

// New 构造 Agent service；缺失 Task Center 时 Runtime 操作 fail closed。
func New(deps Dependencies) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("agent store is required")
	}
	return &Service{store: deps.Store, tasks: deps.Tasks, workspaces: deps.Workspaces, models: deps.Models, scopes: deps.Scopes, grants: deps.Grants, profiles: defaultProfiles()}, nil
}

func defaultProfiles() map[string]iapiserver.AgentProfile {
	return map[string]iapiserver.AgentProfile{
		iapiserver.AgentProfileIDHermes: {ID: iapiserver.AgentProfileIDHermes, Name: "Hermes Agent", Revision: iapiserver.AgentProfileRevisionInitial, Status: iapiserver.AgentProfileStatusActive, SupportedAgentKinds: []string{iapiserver.AgentKindPlatform}, Description: "Platform Agent runtime profile."},
		iapiserver.AgentProfileIDCoding: {ID: iapiserver.AgentProfileIDCoding, Name: "Coding Agent", Revision: iapiserver.AgentProfileRevisionInitial, Status: iapiserver.AgentProfileStatusActive, SupportedAgentKinds: []string{iapiserver.AgentKindCoding}, Description: "OpenCode-compatible Coding Agent runtime profile."},
	}
}

// ListProfiles 返回当前启用的只读 AgentProfile。
func (s *Service) ListProfiles(context.Context) (*iapiserver.AgentProfileListResponse, error) {
	items := make([]*iapiserver.AgentProfile, 0, len(s.profiles))
	for _, id := range []string{iapiserver.AgentProfileIDHermes} {
		profile := s.profiles[id]
		copy := profile
		items = append(items, &copy)
	}
	return &iapiserver.AgentProfileListResponse{Total: len(items), Items: items}, nil
}

// ListAgents 返回当前认证用户拥有的 Agent。
func (s *Service) ListAgents(ctx context.Context, req *iapiserver.AgentListRequest) (*iapiserver.AgentListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = owner
	items, total, err := s.store.ListAgents(ctx, req)
	return &iapiserver.AgentListResponse{Total: total, Items: items}, err
}

// CreateAgent 原子创建 Agent、默认 Session、固定 Workspace Binding 和可选主模型绑定。
func (s *Service) CreateAgent(ctx context.Context, req *iapiserver.AgentCreateRequest) (*iapiserver.Agent, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	profile, ok := s.profiles[req.AgentProfileID]
	if !ok || profile.Status != iapiserver.AgentProfileStatusActive || !sets.NewString(profile.SupportedAgentKinds...).Has(iapiserver.AgentKindPlatform) {
		return nil, errors.NewStatus(code.ErrAgentProfileInvalid, "agent profile is unavailable for platform agents")
	}
	if req.AgentProfileRevision != "" && req.AgentProfileRevision != profile.Revision {
		return nil, errors.NewStatus(code.ErrAgentProfileInvalid, "agent profile revision is unavailable")
	}
	workspaceID := uuid.NewString()
	authorization := iapiserver.AgentAuthorizationSummary{Source: iapiserver.AgentAuthorizationSourceAgent, ValidatedAt: imachinery.Now()}
	agentID, sessionID := uuid.NewString(), uuid.NewString()
	agent := &iapiserver.Agent{
		ObjectMeta:  imachinery.ObjectMeta{ID: agentID, Name: req.Name, Description: req.Description},
		OwnerUserID: owner, Kind: iapiserver.AgentKindPlatform, AgentProfileID: profile.ID, AgentProfileRevision: profile.Revision,
		WorkspaceType: iapiserver.AgentWorkspaceTypeAgent, WorkspaceID: workspaceID, Status: iapiserver.AgentStatusReady, RuntimePolicy: req.RuntimePolicy,
	}
	session := &iapiserver.AgentSession{ObjectMeta: imachinery.ObjectMeta{ID: sessionID}, AgentID: agentID, OwnerUserID: owner, Title: req.Name, Status: iapiserver.AgentSessionStatusOpen}
	binding := &iapiserver.AgentWorkspaceBinding{
		ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AgentID: agentID, WorkspaceType: iapiserver.AgentWorkspaceTypeAgent,
		WorkspaceID: workspaceID, AccessMode: iapiserver.AgentWorkspaceAccessModeReadWrite, AuthorizationSummary: authorization,
	}
	var model *iapiserver.AgentModelBinding
	if req.ModelBinding != nil {
		model = modelBinding(agentID, "primary-model", req.ModelBinding)
	} else {
		model = modelBinding(agentID, "primary-model", &iapiserver.AgentModelBindingInput{
			SourceType: iapiserver.AgentModelBindingSourceTypeUserDefault, SourceRef: iapiserver.AgentModelBindingSourceRefUserDefault, Purpose: defaultPurpose(iapiserver.AgentKindPlatform),
		})
	}
	if err := s.store.CreateAgentAggregate(ctx, agent, session, binding, model); err != nil {
		return nil, err
	}
	return agent, nil
}

// PrepareCodingAgentForStudio 仅校验并构造 AppStudio 初始化事务需要的 Agent 聚合。
func (s *Service) PrepareCodingAgentForStudio(
	ctx context.Context,
	studioApplicationID string,
	workspaceID string,
	ownerUserID string,
	idempotencyKey string,
	profileID string,
	modelBindingInput *iapiserver.AgentModelBindingInput,
	authorization *iapiserver.AgentAuthorizationSummary,
) (*iapiserver.Agent, *iapiserver.AgentSession, *iapiserver.AgentWorkspaceBinding, *iapiserver.AgentModelBinding, error) {
	if studioApplicationID == "" || workspaceID == "" || ownerUserID == "" || idempotencyKey == "" {
		return nil, nil, nil, nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent initialization context is incomplete")
	}
	if profileID == "" {
		profileID = iapiserver.AgentProfileIDCoding
	}
	profile, ok := s.profiles[profileID]
	if !ok || profile.Status != iapiserver.AgentProfileStatusActive || !sets.NewString(profile.SupportedAgentKinds...).Has(iapiserver.AgentKindCoding) {
		return nil, nil, nil, nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent profile is unavailable")
	}
	if authorization == nil || authorization.Source == "" {
		return nil, nil, nil, nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent workspace authorization is unavailable")
	}
	if modelBindingInput == nil || modelBindingInput.SourceType == "" || modelBindingInput.SourceRef == "" || modelBindingInput.Purpose != iapiserver.AgentModelBindingPurposeCoding {
		return nil, nil, nil, nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, "coding agent requires an explicit CODING model selection")
	}

	agentID := stableCodingAgentID(idempotencyKey)
	agent := &iapiserver.Agent{
		ObjectMeta: imachinery.ObjectMeta{
			ID:          agentID,
			Name:        "Studio Coding Agent",
			Description: "Coding Agent for StudioApplication " + studioApplicationID,
		},
		OwnerUserID:          ownerUserID,
		Kind:                 iapiserver.AgentKindCoding,
		AgentProfileID:       profile.ID,
		AgentProfileRevision: profile.Revision,
		WorkspaceType:        iapiserver.AgentWorkspaceTypeStudio,
		WorkspaceID:          workspaceID,
		Status:               iapiserver.AgentStatusReady,
	}
	session := &iapiserver.AgentSession{
		ObjectMeta: imachinery.ObjectMeta{ID: stableCodingAgentChildID(agentID, "session")},
		AgentID:    agentID, OwnerUserID: ownerUserID, Title: "Coding Session", Status: iapiserver.AgentSessionStatusOpen,
	}
	binding := &iapiserver.AgentWorkspaceBinding{
		ObjectMeta: imachinery.ObjectMeta{ID: stableCodingAgentChildID(agentID, "workspace-binding")},
		AgentID:    agentID, WorkspaceType: iapiserver.AgentWorkspaceTypeStudio, WorkspaceID: workspaceID,
		AccessMode: iapiserver.AgentWorkspaceAccessModeReadWrite, AuthorizationSummary: *authorization,
	}
	model := modelBinding(agentID, "primary-model", modelBindingInput)
	model.ID = stableCodingAgentChildID(agentID, "primary-model")
	return agent, session, binding, model, nil
}

// GetAgent 返回当前用户可见 Agent。
func (s *Service) GetAgent(ctx context.Context, id string) (*iapiserver.Agent, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	agent, err := s.store.GetAgent(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if agent.Kind != iapiserver.AgentKindPlatform {
		return nil, errors.NewStatus(code.ErrAgentNotVisible, "agent not visible")
	}
	return agent, nil
}

// GetCodingAgentForStudio 返回当前用户拥有且固定到指定 AppStudio Workspace 的内部 Coding Agent/Session。
func (s *Service) GetCodingAgentForStudio(ctx context.Context, agentID, sessionID, workspaceID string) (*iapiserver.Agent, *iapiserver.AgentSession, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, nil, err
	}
	agent, err := s.store.GetAgent(ctx, agentID, owner)
	if err != nil {
		return nil, nil, err
	}
	if !codingAgentMatchesStudio(agent, workspaceID) {
		return nil, nil, errors.NewStatus(code.ErrAgentNotVisible, "coding agent not visible")
	}
	session, err := s.store.GetAgentSession(ctx, sessionID, owner)
	if err != nil {
		return nil, nil, err
	}
	if session.AgentID != agent.ID {
		return nil, nil, errors.NewStatus(code.ErrAgentSessionNotVisible, "coding agent session not visible")
	}
	return agent, session, nil
}

// GetCodingModelBindingForStudio 返回替换 generation 时可继承的当前主 Coding ModelBinding。
func (s *Service) GetCodingModelBindingForStudio(ctx context.Context, agentID, sessionID, workspaceID string) (*iapiserver.AgentModelBinding, error) {
	agent, _, err := s.GetCodingAgentForStudio(ctx, agentID, sessionID, workspaceID)
	if err != nil {
		return nil, err
	}
	bindings, err := s.store.ListAgentModelBindings(ctx, agent.ID, agent.OwnerUserID)
	if err != nil {
		return nil, err
	}
	binding := primaryModel(bindings)
	if binding == nil || binding.Purpose != iapiserver.AgentModelBindingPurposeCoding {
		return nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, "an ACTIVE primary coding model binding is required")
	}
	return binding, nil
}

// UpdateAgent 更新非敏感配置并保持 Kind/Profile/Workspace 不变。
func (s *Service) UpdateAgent(ctx context.Context, id string, req *iapiserver.AgentUpdateRequest) (*iapiserver.Agent, error) {
	agent, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Description != nil {
		agent.Description = *req.Description
	}
	if len(req.RuntimePolicy) > 0 {
		agent.RuntimePolicy = req.RuntimePolicy
	}
	return s.store.UpdateAgent(ctx, agent, req.ResourceVersion)
}

// DeleteAgent 进入 DELETING，并请求停止当前 Runtime；Workspace 和历史事实不级联删除。
func (s *Service) DeleteAgent(ctx context.Context, id string) (*iapiserver.Agent, error) {
	agent, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime, runtimeErr := s.currentRuntime(ctx, agent); runtimeErr == nil && runtime != nil {
		if _, stopErr := s.stopRuntime(ctx, agent, runtime, iapiserver.AgentRuntimeActionDelete, "agent deletion"); stopErr != nil {
			return nil, stopErr
		}
	}
	agent.Status, agent.Disabled = iapiserver.AgentStatusDeleting, true
	return s.store.UpdateAgent(ctx, agent, agent.ResourceVersion)
}

// EnableAgent 恢复 DISABLED Agent 到 READY 或 SUSPENDED。
func (s *Service) EnableAgent(ctx context.Context, id string) (*iapiserver.Agent, error) {
	agent, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if !agent.Disabled && agent.Status != iapiserver.AgentStatusDisabled {
		return nil, errors.NewStatus(code.ErrAgentStateInvalid, "agent is not disabled")
	}
	agent.Disabled, agent.Status = false, iapiserver.AgentStatusReady
	return s.store.UpdateAgent(ctx, agent, agent.ResourceVersion)
}

// DisableAgent 阻止新 Invocation，并保持历史和 Workspace 不变。
func (s *Service) DisableAgent(ctx context.Context, id string, req *iapiserver.AgentActionRequest) (*iapiserver.Agent, error) {
	agent, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime, runtimeErr := s.currentRuntime(ctx, agent); runtimeErr == nil && runtime != nil {
		if _, stopErr := s.stopRuntime(ctx, agent, runtime, iapiserver.AgentRuntimeActionSuspend, req.Reason); stopErr != nil {
			return nil, stopErr
		}
	}
	agent.Disabled, agent.Status = true, iapiserver.AgentStatusDisabled
	return s.store.UpdateAgent(ctx, agent, agent.ResourceVersion)
}

// ListSessions 返回 Agent 的会话历史。
func (s *Service) ListSessions(ctx context.Context, agentID string, req *iapiserver.AgentSessionListRequest) (*iapiserver.AgentSessionListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgent(ctx, agentID, owner); err != nil {
		return nil, err
	}
	req.AgentID, req.OwnerUserID = agentID, owner
	items, total, err := s.store.ListAgentSessions(ctx, req)
	return &iapiserver.AgentSessionListResponse{Total: total, Items: items}, err
}

// CreateSession 创建 OPEN Session，不依赖 Runtime 是否存在。
func (s *Service) CreateSession(ctx context.Context, agentID string, req *iapiserver.AgentSessionCreateRequest) (*iapiserver.AgentSession, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgent(ctx, agentID, owner); err != nil {
		return nil, err
	}
	return s.store.CreateAgentSession(ctx, &iapiserver.AgentSession{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AgentID: agentID, OwnerUserID: owner, Title: req.Title, Status: iapiserver.AgentSessionStatusOpen})
}

func (s *Service) GetSession(ctx context.Context, id string) (*iapiserver.AgentSession, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetAgentSession(ctx, id, owner)
}

func (s *Service) UpdateSession(ctx context.Context, id string, req *iapiserver.AgentSessionUpdateRequest) (*iapiserver.AgentSession, error) {
	session, err := s.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.Status != iapiserver.AgentSessionStatusOpen {
		return nil, errors.NewStatus(code.ErrAgentSessionClosed, "closed session cannot be updated")
	}
	session.Title = req.Title
	return s.store.UpdateAgentSession(ctx, session, req.ResourceVersion)
}

func (s *Service) CloseSession(ctx context.Context, id string) (*iapiserver.AgentSession, error) {
	return s.setSessionStatus(ctx, id, iapiserver.AgentSessionStatusClosed)
}
func (s *Service) ArchiveSession(ctx context.Context, id string) (*iapiserver.AgentSession, error) {
	return s.setSessionStatus(ctx, id, iapiserver.AgentSessionStatusArchived)
}

func (s *Service) setSessionStatus(ctx context.Context, id, status string) (*iapiserver.AgentSession, error) {
	session, err := s.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.Status == status {
		return session, nil
	}
	if session.Status != iapiserver.AgentSessionStatusOpen && status == iapiserver.AgentSessionStatusClosed {
		return nil, errors.NewStatus(code.ErrAgentSessionClosed, "session is not open")
	}
	session.Status = status
	return s.store.UpdateAgentSession(ctx, session, session.ResourceVersion)
}

// SendMessage 持久化消息和 Invocation，并提交到 Task Center 执行。
func (s *Service) SendMessage(ctx context.Context, sessionID string, req *iapiserver.AgentMessageRequest) (*iapiserver.AgentInvocation, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	session, err := s.store.GetAgentSession(ctx, sessionID, owner)
	if err != nil {
		return nil, err
	}
	if session.Status != iapiserver.AgentSessionStatusOpen {
		return nil, errors.NewStatus(code.ErrAgentSessionClosed, "session cannot accept messages")
	}
	agent, err := s.store.GetAgent(ctx, session.AgentID, owner)
	if err != nil {
		return nil, err
	}
	if agent.Disabled || agent.Status == iapiserver.AgentStatusDisabled || agent.Status == iapiserver.AgentStatusDeleting {
		return nil, errors.NewStatus(code.ErrAgentStateInvalid, "agent cannot accept messages")
	}
	idempotency := req.IdempotencyKey
	if idempotency == "" {
		idempotency = uuid.NewString()
	}
	typeName := iapiserver.AgentInvocationTypeChat
	if agent.Kind == iapiserver.AgentKindCoding {
		typeName = iapiserver.AgentInvocationTypeCoding
	}
	bindings, err := s.store.ListAgentModelBindings(ctx, agent.ID, owner)
	if err != nil {
		return nil, err
	}
	if primaryModel(bindings) == nil {
		return nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, "an ACTIVE primary model binding is required")
	}
	messageID, invocationID := uuid.NewString(), uuid.NewString()
	message := &iapiserver.AgentMessage{ObjectMeta: imachinery.ObjectMeta{ID: messageID}, SessionID: session.ID, AgentID: agent.ID, InvocationID: invocationID, Role: iapiserver.AgentMessageRoleUser, Content: req.Content, Attachments: req.Attachments}
	invocation := &iapiserver.AgentInvocation{
		ObjectMeta: imachinery.ObjectMeta{ID: invocationID}, AgentID: agent.ID, SessionID: session.ID,
		Type: typeName, Status: iapiserver.AgentInvocationStatusQueued, UserMessageID: messageID,
		AssistantMessageID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("agent-invocation-assistant:"+invocationID)).String(),
		IdempotencyKey:     idempotency,
	}
	created, err := s.store.CreateAgentInvocation(ctx, message, invocation)
	if err != nil {
		return nil, err
	}
	if created.ID != invocationID {
		if created.Status != iapiserver.AgentInvocationStatusFailed ||
			created.FailureCode != iapiserver.AgentInvocationFailureCodeTaskUnavailable || created.AtomicTaskID != nil {
			return created, nil
		}
		created, _, err = s.store.RetryAgentInvocationSubmission(ctx, created.ID, created.ResourceVersion, created.SubmissionGeneration)
		if err != nil {
			return nil, err
		}
		if created.Status != iapiserver.AgentInvocationStatusQueued || created.AtomicTaskID != nil {
			return created, nil
		}
	}
	if s.tasks == nil {
		return s.failInvocationSubmission(ctx, created, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "task center is unavailable"))
	}
	runtime, runtimeErr := s.currentRuntime(ctx, agent)
	if runtimeErr != nil {
		return s.failInvocationSubmission(ctx, created, runtimeErr)
	}
	if runtime == nil || runtime.State != iapiserver.AgentRuntimeStateReady {
		runtime, runtimeErr = s.ensureRuntimeForAgent(ctx, agent, iapiserver.AgentRuntimeOperationStart, &iapiserver.AgentRuntimeActionRequest{})
		if runtimeErr != nil {
			return s.failInvocationSubmission(ctx, created, runtimeErr)
		}
		created.RuntimeBindingID = runtime.ID
		unbound := created.DeepCopy()
		if _, err := s.store.UpdateAgentInvocation(ctx, created); err != nil {
			return s.failInvocationSubmission(ctx, unbound, err)
		}
		return created, nil
	}
	return s.submitInvocationTask(ctx, agent, created, runtime)
}

// StartCodingInvocation 在 AppStudio 初始化事务提交后启动或排队指定 Coding Invocation。
func (s *Service) StartCodingInvocation(ctx context.Context, agentID, invocationID string) (*iapiserver.AgentInvocation, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	agent, err := s.store.GetAgent(ctx, agentID, owner)
	if err != nil {
		return nil, err
	}
	if agent.Kind != iapiserver.AgentKindCoding {
		return nil, errors.NewStatus(code.ErrAgentStateInvalid, "agent is not a coding agent")
	}
	invocation, err := s.store.GetAgentInvocation(ctx, invocationID, owner)
	if err != nil {
		return nil, err
	}
	if invocation.AgentID != agent.ID {
		return invocation, nil
	}
	if invocation.Status == iapiserver.AgentInvocationStatusFailed &&
		invocation.FailureCode == iapiserver.AgentInvocationFailureCodeTaskUnavailable && invocation.AtomicTaskID == nil {
		invocation, _, err = s.store.RetryAgentInvocationSubmission(ctx, invocation.ID, invocation.ResourceVersion, invocation.SubmissionGeneration)
		if err != nil {
			return nil, err
		}
	}
	if invocation.Status != iapiserver.AgentInvocationStatusQueued || invocation.AtomicTaskID != nil {
		return invocation, nil
	}
	runtime, err := s.currentRuntime(ctx, agent)
	if err != nil {
		return s.failInvocationSubmission(ctx, invocation, err)
	}
	if runtime == nil || runtime.State != iapiserver.AgentRuntimeStateReady {
		runtime, err = s.ensureRuntimeForAgent(ctx, agent, iapiserver.AgentRuntimeOperationStart, &iapiserver.AgentRuntimeActionRequest{})
		if err != nil {
			return s.failInvocationSubmission(ctx, invocation, err)
		}
		invocation.RuntimeBindingID = runtime.ID
		unbound := invocation.DeepCopy()
		updated, updateErr := s.store.UpdateAgentInvocation(ctx, invocation)
		if updateErr != nil {
			return s.failInvocationSubmission(ctx, unbound, updateErr)
		}
		return updated, nil
	}
	return s.submitInvocationTask(ctx, agent, invocation, runtime)
}

// submitInvocationTask 只向 Task Center 传递 released registry 允许的引用和恢复游标。
func (s *Service) submitInvocationTask(
	ctx context.Context,
	agent *iapiserver.Agent,
	invocation *iapiserver.AgentInvocation,
	runtime *iapiserver.AgentRuntimeBinding,
) (*iapiserver.AgentInvocation, error) {
	if invocation == nil || runtime == nil || runtime.State != iapiserver.AgentRuntimeStateReady || runtime.AgentID != agent.ID {
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "invocation runtime binding is not ready"))
	}
	if s.tasks == nil {
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "task center is unavailable"))
	}
	if s.models == nil || s.grants == nil {
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "agent invocation authorization is unavailable"))
	}
	bindings, err := s.store.ListAgentModelBindings(ctx, agent.ID, agent.OwnerUserID)
	if err != nil {
		return s.failInvocationSubmission(ctx, invocation, err)
	}
	primary := primaryModel(bindings)
	if primary == nil || primary.Purpose != defaultPurpose(agent.Kind) {
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentModelBindingInvalid, "an ACTIVE primary model binding is required before invocation submission"))
	}
	modelAccessRef, err := s.models.ResolveAgentModelAccess(ctx, agent.OwnerUserID, primary)
	if err != nil {
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentModelBindingInvalid, err.Error()))
	}
	expectedVersion := invocation.ResourceVersion + 1
	issuedAt, expiresAt := s.grants.Window()
	authorizationRef, err := s.grants.Issue(iapiserver.TaskWorkerRefPrefixAgentInvocationGrant, agentgrant.InvocationClaims{
		OwnerUserID: agent.OwnerUserID, AgentID: agent.ID, SessionID: invocation.SessionID,
		InvocationID: invocation.ID, RuntimeBindingID: runtime.ID, InvocationType: invocation.Type,
		ExpectedResourceVersion: expectedVersion, WorkspaceID: agent.WorkspaceID,
		ModelAccessGrantRef: modelAccessRef, IssuedAt: issuedAt, ExpiresAt: expiresAt,
	})
	if err != nil {
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, err.Error()))
	}
	task, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AgentTaskDomain, &iapiserver.AtomicTaskCreateRequest{
		Key:  "agent-invocation-" + invocation.ID + "-" + fmt.Sprint(invocation.SubmissionGeneration),
		Name: "Agent " + invocation.Type + " Invocation", FunctionRef: iapiserver.AgentInvocationFunctionExecute,
		Arguments: map[string]any{
			iapiserver.AgentInvocationTaskKeyAgentID:                    agent.ID,
			iapiserver.AgentInvocationTaskKeySessionID:                  invocation.SessionID,
			iapiserver.AgentInvocationTaskKeyInvocationID:               invocation.ID,
			iapiserver.AgentInvocationTaskKeyRuntimeBindingID:           runtime.ID,
			iapiserver.AgentInvocationTaskKeyInvocationType:             invocation.Type,
			iapiserver.AgentInvocationTaskKeyAuthorizationRef:           authorizationRef,
			iapiserver.AgentInvocationTaskKeyExpectedResourceVersion:    expectedVersion,
			iapiserver.AgentInvocationTaskKeyResumeRuntimeSessionRef:    nullableString(invocation.RuntimeSessionRef),
			iapiserver.AgentInvocationTaskKeyResumeRuntimeInvocationRef: nullableString(invocation.RuntimeInvocationRef),
			iapiserver.AgentInvocationTaskKeyEventSequenceAfter:         invocation.LastEventSequence,
		},
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: agent.OwnerUserID,
	})
	if err != nil && (task == nil || errors.ToStatus(err).Code != code.ErrAtomicTaskIdempotencyConflict) {
		return s.failInvocationSubmission(ctx, invocation, err)
	}
	if task == nil || !sameInvocationTask(task, agent, invocation, runtime, expectedVersion) {
		if err != nil {
			return s.failInvocationSubmission(ctx, invocation, err)
		}
		return s.failInvocationSubmission(ctx, invocation, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "invocation task identity does not match"))
	}
	bound, applied, err := s.store.BindAgentInvocationTask(
		ctx,
		invocation.ID,
		invocation.ResourceVersion,
		invocation.SubmissionGeneration,
		runtime.ID,
		task.ID,
		expectedVersion,
	)
	if err != nil {
		return s.failInvocationSubmission(ctx, invocation, err)
	}
	if !applied {
		return s.failInvocationSubmission(ctx, bound, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "invocation task lost its resource-version fence"))
	}
	return bound, nil
}

func (s *Service) failInvocationSubmission(
	ctx context.Context,
	invocation *iapiserver.AgentInvocation,
	cause error,
) (*iapiserver.AgentInvocation, error) {
	if invocation == nil {
		return nil, cause
	}
	failed, _, err := s.store.FailAgentInvocationSubmission(
		ctx,
		invocation.ID,
		invocation.ResourceVersion,
		invocation.SubmissionGeneration,
		invocationSubmissionFailureMessage(cause),
	)
	if err != nil {
		return nil, stderrors.Join(cause, errors.Wrap(err, "project invocation submission failure"))
	}
	return failed, cause
}

func invocationSubmissionFailureMessage(cause error) string {
	switch errors.ToStatus(cause).Code {
	case code.ErrAgentRuntimeOperationFailed, code.ErrAgentRuntimeNotVisible:
		return "Agent runtime is unavailable for invocation submission."
	case code.ErrAgentModelBindingInvalid:
		return "Agent model authorization is unavailable for invocation submission."
	default:
		return "Agent invocation task submission failed before task binding."
	}
}

func sameInvocationTask(task *iapiserver.AtomicTask, agent *iapiserver.Agent, invocation *iapiserver.AgentInvocation, runtime *iapiserver.AgentRuntimeBinding, expectedVersion int64) bool {
	if task == nil || agent == nil || invocation == nil || runtime == nil || task.FunctionRef != iapiserver.AgentInvocationFunctionExecute ||
		task.ProjectID != iapiserver.DefaultTaskCenterProjectID || task.Namespace != iapiserver.DefaultTaskCenterNamespace ||
		task.CreatedBy != agent.OwnerUserID || task.IdempotencyScope != "agent-invocation:"+invocation.ID ||
		task.IdempotencyKey != fmt.Sprintf("%d:0", expectedVersion) {
		return false
	}
	stringArgument := func(key, expected string) bool {
		value, ok := task.Arguments[key].(string)
		return ok && value == expected
	}
	if !stringArgument(iapiserver.AgentInvocationTaskKeyAgentID, agent.ID) ||
		!stringArgument(iapiserver.AgentInvocationTaskKeySessionID, invocation.SessionID) ||
		!stringArgument(iapiserver.AgentInvocationTaskKeyInvocationID, invocation.ID) ||
		!stringArgument(iapiserver.AgentInvocationTaskKeyRuntimeBindingID, runtime.ID) ||
		!stringArgument(iapiserver.AgentInvocationTaskKeyInvocationType, invocation.Type) {
		return false
	}
	actualVersion, ok := runtimeTaskExpectedVersion(task.Arguments[iapiserver.AgentInvocationTaskKeyExpectedResourceVersion])
	return ok && actualVersion == expectedVersion
}

func (s *Service) ListMessages(ctx context.Context, sessionID string, req *iapiserver.AgentMessageListRequest) (*iapiserver.AgentMessageListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgentSession(ctx, sessionID, owner); err != nil {
		return nil, err
	}
	req.SessionID = sessionID
	items, total, err := s.store.ListAgentMessages(ctx, req, owner)
	return &iapiserver.AgentMessageListResponse{Total: total, Items: items}, err
}

func (s *Service) ListInvocations(ctx context.Context, sessionID string, req *iapiserver.AgentInvocationListRequest) (*iapiserver.AgentInvocationListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgentSession(ctx, sessionID, owner); err != nil {
		return nil, err
	}
	req.SessionID = sessionID
	items, total, err := s.store.ListAgentInvocations(ctx, req, owner)
	return &iapiserver.AgentInvocationListResponse{Total: total, Items: items}, err
}

func (s *Service) GetInvocation(ctx context.Context, id string) (*iapiserver.AgentInvocation, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetAgentInvocation(ctx, id, owner)
}

func (s *Service) CancelInvocation(ctx context.Context, id string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentInvocation, error) {
	invocation, err := s.GetInvocation(ctx, id)
	if err != nil {
		return nil, err
	}
	if isInvocationTerminal(invocation.Status) {
		return invocation, nil
	}
	invocation.Status = iapiserver.AgentInvocationStatusCanceling
	if _, err := s.store.UpdateAgentInvocation(ctx, invocation); err != nil {
		return nil, err
	}
	if invocation.AtomicTaskID != nil && *invocation.AtomicTaskID != "" {
		if s.tasks == nil {
			return nil, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "task center is unavailable")
		}
		if _, err := s.tasks.CancelAtomicTask(ctx, *invocation.AtomicTaskID, &iapiserver.ActionReasonRequest{Reason: req.Reason}); err != nil {
			return nil, err
		}
	}
	return invocation, nil
}

func (s *Service) ListInvocationEvents(ctx context.Context, id string, afterSequence int) ([]*iapiserver.AgentOperationEvent, error) {
	if _, err := s.GetInvocation(ctx, id); err != nil {
		return nil, err
	}
	return s.store.ListAgentOperationEvents(ctx, id, afterSequence)
}

func (s *Service) ListMemories(ctx context.Context, agentID string, req *iapiserver.AgentMemoryListRequest) (*iapiserver.AgentMemoryListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgent(ctx, agentID, owner); err != nil {
		return nil, err
	}
	req.AgentID = agentID
	items, total, err := s.store.ListAgentMemories(ctx, req, owner)
	return &iapiserver.AgentMemoryListResponse{Total: total, Items: items}, err
}

func (s *Service) CreateMemory(ctx context.Context, agentID string, req *iapiserver.AgentMemoryCreateRequest) (*iapiserver.AgentMemory, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgent(ctx, agentID, owner); err != nil {
		return nil, err
	}
	if req.SessionID != "" {
		session, err := s.store.GetAgentSession(ctx, req.SessionID, owner)
		if err != nil || session.AgentID != agentID {
			return nil, errors.NewStatus(code.ErrAgentMemoryInvalid, "memory session is invalid")
		}
	}
	var sessionID *string
	if req.SessionID != "" {
		value := req.SessionID
		sessionID = &value
	}
	return s.store.CreateAgentMemory(ctx, &iapiserver.AgentMemory{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AgentID: agentID, SessionID: sessionID, Scope: req.Scope, Type: req.Type, Content: req.Content, SourceMessageID: req.SourceMessageID})
}

func (s *Service) GetMemory(ctx context.Context, id string) (*iapiserver.AgentMemory, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetAgentMemory(ctx, id, owner)
}
func (s *Service) UpdateMemory(ctx context.Context, id string, req *iapiserver.AgentMemoryUpdateRequest) (*iapiserver.AgentMemory, error) {
	memory, err := s.GetMemory(ctx, id)
	if err != nil {
		return nil, err
	}
	memory.Content = req.Content
	return s.store.UpdateAgentMemory(ctx, memory, req.ResourceVersion)
}
func (s *Service) DeleteMemory(ctx context.Context, id string) error {
	owner, err := currentUserID(ctx)
	if err != nil {
		return err
	}
	return s.store.DeleteAgentMemory(ctx, id, owner)
}

func (s *Service) GetWorkspaceBinding(ctx context.Context, agentID string) (*iapiserver.AgentWorkspaceBinding, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetAgentWorkspaceBinding(ctx, agentID, owner)
}
func (s *Service) ListModelBindings(ctx context.Context, agentID string) (*iapiserver.AgentModelBindingListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListAgentModelBindings(ctx, agentID, owner)
	return &iapiserver.AgentModelBindingListResponse{Total: int64(len(items)), Items: items}, err
}
func (s *Service) ReplaceModelBinding(ctx context.Context, agentID string, req *iapiserver.AgentModelBindingInput) (*iapiserver.AgentModelBinding, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.ReplaceAgentModelBinding(ctx, agentID, owner, modelBinding(agentID, "primary-model", req))
}
func (s *Service) ListSkillBindings(ctx context.Context, agentID string) (*iapiserver.AgentSkillBindingListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListAgentSkillBindings(ctx, agentID, owner)
	return &iapiserver.AgentSkillBindingListResponse{Total: int64(len(items)), Items: items}, err
}
func (s *Service) ListMCPBindings(ctx context.Context, agentID string) (*iapiserver.AgentMCPBindingListResponse, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetAgent(ctx, agentID, owner); err != nil {
		return nil, err
	}
	items, err := s.store.ListAgentMCPBindings(ctx, agentID, owner)
	return &iapiserver.AgentMCPBindingListResponse{Total: int64(len(items)), Items: items}, err
}
func (s *Service) CreateMCPBinding(ctx context.Context, agentID string, req *iapiserver.AgentMCPBindingRequest) (*iapiserver.AgentMCPBinding, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	binding := &iapiserver.AgentMCPBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: req.Name}, AgentID: agentID, ServerType: req.ServerType, EndpointRef: req.EndpointRef, CredentialRef: req.CredentialRef, AllowedTools: req.AllowedTools, Configuration: req.Configuration, Enabled: enabled}
	return s.store.CreateAgentMCPBinding(ctx, owner, binding)
}

func (s *Service) UpdateMCPBinding(ctx context.Context, agentID, bindingID string, req *iapiserver.AgentMCPBindingUpdateRequest) (*iapiserver.AgentMCPBinding, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	credential := ""
	if req.CredentialRefMode == iapiserver.AgentMCPBindingCredentialRefModeSet && req.CredentialRef != nil {
		credential = *req.CredentialRef
	}
	if req.CredentialRefMode == iapiserver.AgentMCPBindingCredentialRefModeKeep {
		// Store resolves KEEP from the current row; an empty value is a sentinel here.
		credential = "__KEEP__"
	}
	binding := &iapiserver.AgentMCPBinding{ObjectMeta: imachinery.ObjectMeta{ID: bindingID, Name: req.Name}, AgentID: agentID, ServerType: req.ServerType, EndpointRef: req.EndpointRef, CredentialRef: credential, AllowedTools: req.AllowedTools, Configuration: req.Configuration, Enabled: req.Enabled}
	if req.CredentialRefMode == iapiserver.AgentMCPBindingCredentialRefModeClear {
		binding.CredentialRef = ""
	}
	return s.store.UpdateAgentMCPBinding(ctx, agentID, owner, binding, req.ResourceVersion)
}

func (s *Service) DeleteMCPBinding(ctx context.Context, agentID, bindingID string) error {
	owner, err := currentUserID(ctx)
	if err != nil {
		return err
	}
	if _, err := s.store.GetAgent(ctx, agentID, owner); err != nil {
		return err
	}
	return s.store.DeleteAgentMCPBinding(ctx, agentID, bindingID, owner)
}

// EnsurePlatformMCPBindingForCodingAgent 为当前 Coding Agent 代际幂等补齐平台 MCP Binding。
func (s *Service) EnsurePlatformMCPBindingForCodingAgent(ctx context.Context, agentID, applicationID string) error {
	owner, err := currentUserID(ctx)
	if err != nil {
		return err
	}
	agent, err := s.store.GetAgent(ctx, agentID, owner)
	if err != nil {
		return err
	}
	if agent.Kind != iapiserver.AgentKindCoding || agent.WorkspaceType != iapiserver.AgentWorkspaceTypeStudio {
		return nil
	}
	if item, lookupErr := s.store.GetAgentMCPBindingByName(ctx, agentID, agent.OwnerUserID, "omnimam-platform"); lookupErr == nil {
		if item.DeletedAt != nil {
			return nil
		}
		return nil
	}
	_, err = s.store.CreateAgentMCPBinding(ctx, agent.OwnerUserID, studioPlatformBinding(agentID, agent.OwnerUserID))
	return err
}

func studioPlatformBinding(agentID, owner string) *iapiserver.AgentMCPBinding {
	return &iapiserver.AgentMCPBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("agent-platform-mcp:"+agentID)).String(), Name: "omnimam-platform", Description: "Platform MCP for " + owner}, AgentID: agentID, ServerType: iapiserver.AgentMCPServerTypePlatform, EndpointRef: iapiserver.AgentMCPPlatformEndpointRefDefault, Enabled: true, AllowedTools: []string{"omnimam.capabilities.list", "omnimam.capabilities.get", "omnimam.applications.list", "omnimam.applications.get", "omnimam.applications.run", "omnimam.application_runs.get", "omnimam.assets.search", "omnimam.assets.get"}}
}

func (s *Service) StartRuntime(ctx context.Context, agentID string, req *iapiserver.AgentRuntimeActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	return s.ensureRuntime(ctx, agentID, iapiserver.AgentRuntimeOperationStart, req)
}
func (s *Service) RecoverRuntime(ctx context.Context, agentID string, req *iapiserver.AgentRuntimeActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	return s.ensureRuntime(ctx, agentID, iapiserver.AgentRuntimeOperationRecover, req)
}

func (s *Service) ensureRuntime(ctx context.Context, agentID, operation string, req *iapiserver.AgentRuntimeActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return s.ensureRuntimeForAgent(ctx, agent, operation, req)
}

func (s *Service) ensureRuntimeForAgent(ctx context.Context, agent *iapiserver.Agent, operation string, req *iapiserver.AgentRuntimeActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	if agent.Disabled || agent.Status == iapiserver.AgentStatusDisabled || agent.Status == iapiserver.AgentStatusDeleting {
		return nil, errors.NewStatus(code.ErrAgentStateInvalid, "agent runtime operation is blocked")
	}
	if s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "task center is unavailable")
	}
	runtime, err := s.currentRuntime(ctx, agent)
	if err != nil {
		return nil, err
	}
	if runtime != nil {
		if runtime.State == iapiserver.AgentRuntimeStateReady && operation == iapiserver.AgentRuntimeOperationStart {
			return runtime, nil
		}
		if runtime.CurrentTaskID != nil && *runtime.CurrentTaskID != "" {
			return runtime, nil
		}
	}
	bindings, err := s.store.ListAgentModelBindings(ctx, agent.ID, agent.OwnerUserID)
	if err != nil {
		return nil, err
	}
	primary := primaryModel(bindings)
	if primary == nil {
		return nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, "an ACTIVE primary model binding is required before runtime startup")
	}
	if s.models == nil {
		return nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, "agent model grant resolver is unavailable")
	}
	modelAccessRef, err := s.models.ResolveAgentModelAccess(ctx, agent.OwnerUserID, primary)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, err.Error())
	}
	var studioApplicationID string
	var agentGeneration *int64
	if agent.Kind == iapiserver.AgentKindCoding {
		if s.scopes == nil {
			return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "coding agent workload scope resolver is unavailable")
		}
		applicationID, generation, scopeErr := s.scopes.ResolveAgentWorkloadScope(ctx, agent.ID, agent.OwnerUserID)
		if scopeErr != nil || applicationID == "" || generation < 1 {
			return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "coding agent workload scope is unavailable")
		}
		studioApplicationID = applicationID
		agentGeneration = &generation
	}
	mcpBindings, err := s.store.ListAgentMCPBindings(ctx, agent.ID, agent.OwnerUserID)
	if err != nil {
		return nil, err
	}
	mcpRefs := make([]map[string]string, 0, len(mcpBindings))
	for _, binding := range mcpBindings {
		if !binding.Enabled || binding.DeletedAt != nil {
			continue
		}
		mcpRefs = append(mcpRefs, map[string]string{"binding_id": binding.ID, "binding_revision": fmt.Sprintf("%d", binding.ResourceVersion)})
	}
	if len(mcpRefs) > iapiserver.AgentRuntimeMaxMCPBindings {
		return nil, errors.NewStatus(code.ErrAgentMCPBindingInvalid, "at most 50 active MCP bindings may be attached")
	}
	if runtime == nil {
		runtime = &iapiserver.AgentRuntimeBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AgentID: agent.ID, RuntimeProfileID: agent.AgentProfileID, RuntimeProfileRevision: agent.AgentProfileRevision, State: iapiserver.AgentRuntimeStateStarting, ActivityState: iapiserver.AgentRuntimeActivityIdle, HealthStatus: iapiserver.AgentRuntimeHealthUnknown}
		if _, err := s.store.CreateAgentRuntime(ctx, agent.OwnerUserID, runtime); err != nil {
			return nil, err
		}
	}
	workspaceSource := any(nil)
	if agent.Kind == iapiserver.AgentKindPlatform {
		workspaceSource = "agent-workspace://" + agent.WorkspaceID
	}
	expectedVersion := runtime.ResourceVersion + 1
	authRef := runtimeGrantAuthorizationRef(agent.ID, runtime.ID, operation, req, expectedVersion)
	arguments := map[string]any{
		"agent_id": agent.ID, "agent_runtime_id": runtime.ID, "operation": operation, "agent_kind": agent.Kind,
		"workspace_type": agent.WorkspaceType, "workspace_id": agent.WorkspaceID, "workspace_source_ref": workspaceSource,
		"runtime_profile_id": runtime.RuntimeProfileID, "runtime_profile_revision": runtime.RuntimeProfileRevision,
		"model_access_grant_ref": modelAccessRef, "runtime_configuration_ref": "agent-runtime-config://" + agent.ID,
		"authorization_ref":         authRef,
		"expected_resource_version": expectedVersion,
		"mcp_binding_refs":          mcpRefs,
		"lifecycle_policy": map[string]any{
			"restart_policy":           iapiserver.TaskWorkerAgentRuntimeRestartPolicyOnFailure,
			"idle_timeout_seconds":     int(agentRuntimeIdleTimeout / time.Second),
			"maximum_lifetime_seconds": int(agentRuntimeMaximumLifetime / time.Second),
		},
	}
	expiresAt := time.Now().UTC().Add(agentRuntimeMaximumLifetime)
	grant := &iapiserver.AgentRuntimeGrant{
		ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: "runtime-grant-" + runtime.ID},
		AgentID:    agent.ID, RuntimeBindingID: runtime.ID, StudioApplicationID: studioApplicationID, AgentGeneration: agentGeneration, RequestID: authRef,
		BindingRevisions: make([]string, 0, len(mcpRefs)), Status: iapiserver.AgentRuntimeGrantStatusActive,
		ExpiresAt: imachinery.Time{Time: expiresAt},
	}
	for _, ref := range mcpRefs {
		grant.BindingRevisions = append(grant.BindingRevisions, ref["binding_id"]+"/"+ref["binding_revision"])
	}
	if err := s.store.CreateAgentRuntimeGrant(ctx, grant); err != nil {
		return nil, err
	}
	if runtime.InfraRuntimeID != "" {
		arguments["existing_infra_runtime_id"] = runtime.InfraRuntimeID
	}
	task, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AgentTaskDomain, &iapiserver.AtomicTaskCreateRequest{
		Key: "runtime-" + runtime.ID, Name: "Agent runtime " + strings.ToLower(operation), FunctionRef: iapiserver.AgentRuntimeFunctionEnsure,
		Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: agent.OwnerUserID,
	})
	if err != nil && (task == nil || errors.ToStatus(err).Code != code.ErrAtomicTaskIdempotencyConflict || !sameRuntimeEnsureTask(task, agent, runtime, operation, expectedVersion, arguments)) {
		return nil, s.revokeRuntimeGrantAfterFailure(ctx, authRef, err)
	}
	if task == nil || !sameRuntimeEnsureTask(task, agent, runtime, operation, expectedVersion, arguments) {
		cause := errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "runtime ensure task identity does not match")
		return nil, s.revokeRuntimeGrantAfterFailure(ctx, authRef, cause)
	}
	bound, applied, bindErr := s.store.BindAgentRuntimeTask(ctx, runtime.ID, runtime.ResourceVersion, task.ID, operation, iapiserver.AgentRuntimeStateStarting)
	if bindErr != nil {
		return nil, s.revokeRuntimeGrantAfterFailure(ctx, authRef, bindErr)
	}
	if !applied {
		cause := errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "runtime lifecycle task lost its resource-version fence")
		return nil, s.revokeRuntimeGrantAfterFailure(ctx, authRef, cause)
	}
	agent.Status = iapiserver.AgentStatusStarting
	if _, err := s.store.UpdateAgent(ctx, agent, agent.ResourceVersion); err != nil {
		return nil, err
	}
	return bound, nil
}

func runtimeGrantAuthorizationRef(agentID, runtimeID, operation string, req *iapiserver.AgentRuntimeActionRequest, expectedVersion int64) string {
	requestIdentity := "version:" + fmt.Sprint(expectedVersion)
	if req != nil && strings.TrimSpace(req.RequestID) != "" {
		requestIdentity = "request:" + strings.TrimSpace(req.RequestID)
	}
	identity := strings.Join([]string{"agent-runtime-grant", agentID, runtimeID, operation, requestIdentity}, "\x00")
	return "agent-runtime-grant://" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)).String()
}

func sameRuntimeEnsureTask(task *iapiserver.AtomicTask, agent *iapiserver.Agent, runtime *iapiserver.AgentRuntimeBinding, operation string, expectedVersion int64, arguments map[string]any) bool {
	if task == nil || agent == nil || runtime == nil || task.FunctionRef != iapiserver.AgentRuntimeFunctionEnsure ||
		task.ProjectID != iapiserver.DefaultTaskCenterProjectID || task.Namespace != iapiserver.DefaultTaskCenterNamespace ||
		task.CreatedBy != agent.OwnerUserID || task.IdempotencyScope != "agent-runtime:"+runtime.ID ||
		task.IdempotencyKey != fmt.Sprintf("%s:%d:0", operation, expectedVersion) {
		return false
	}
	actual, actualErr := json.Marshal(task.Arguments)
	expected, expectedErr := json.Marshal(arguments)
	return actualErr == nil && expectedErr == nil && string(actual) == string(expected)
}

func (s *Service) revokeRuntimeGrantAfterFailure(ctx context.Context, authorizationRef string, cause error) error {
	if err := s.store.RevokeAgentRuntimeGrant(ctx, authorizationRef); err != nil {
		return stderrors.Join(cause, errors.Wrap(err, "revoke runtime grant after lifecycle submission failure"))
	}
	return cause
}

func (s *Service) SuspendRuntime(ctx context.Context, agentID string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	return s.stopRuntimeAction(ctx, agentID, iapiserver.AgentRuntimeActionSuspend, req)
}

// SuspendCodingAgentForStudio 挂起指定 AppStudio 当前 generation 的 Coding Agent Runtime。
func (s *Service) SuspendCodingAgentForStudio(ctx context.Context, agentID, sessionID, workspaceID string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	agent, _, err := s.GetCodingAgentForStudio(ctx, agentID, sessionID, workspaceID)
	if err != nil {
		return nil, err
	}
	runtime, err := s.currentRuntime(ctx, agent)
	if err != nil {
		return nil, err
	}
	if runtime == nil {
		return nil, errors.NewStatus(code.ErrAgentRuntimeNotVisible, "agent runtime is not active")
	}
	return s.stopRuntime(ctx, agent, runtime, iapiserver.AgentRuntimeActionSuspend, req.Reason)
}

// ResumeCodingAgentForStudio 恢复指定 AppStudio 当前 generation 的 Coding Agent Runtime。
func (s *Service) ResumeCodingAgentForStudio(ctx context.Context, agentID, sessionID, workspaceID string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	agent, _, err := s.GetCodingAgentForStudio(ctx, agentID, sessionID, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.ensureRuntimeForAgent(ctx, agent, iapiserver.AgentRuntimeOperationRecover, &iapiserver.AgentRuntimeActionRequest{RequestID: req.RequestID, ResourceVersion: req.ResourceVersion})
}
func (s *Service) StopRuntime(ctx context.Context, agentID string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	return s.stopRuntimeAction(ctx, agentID, iapiserver.AgentRuntimeActionStop, req)
}

func (s *Service) stopRuntimeAction(ctx context.Context, agentID, action string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	runtime, err := s.currentRuntime(ctx, agent)
	if err != nil {
		return nil, err
	}
	if runtime == nil {
		return nil, errors.NewStatus(code.ErrAgentRuntimeNotVisible, "agent runtime is not active")
	}
	return s.stopRuntime(ctx, agent, runtime, action, req.Reason)
}

func (s *Service) stopRuntime(ctx context.Context, agent *iapiserver.Agent, runtime *iapiserver.AgentRuntimeBinding, action, reason string) (*iapiserver.AgentRuntimeBinding, error) {
	if s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "task center is unavailable")
	}
	if runtime.InfraRuntimeID == "" {
		return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "infra runtime reference is unavailable")
	}
	expectedVersion := runtime.ResourceVersion + 1
	task, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AgentTaskDomain, &iapiserver.AtomicTaskCreateRequest{
		Key: "runtime-stop-" + runtime.ID, Name: "Agent runtime stop", FunctionRef: iapiserver.AgentRuntimeFunctionStop,
		Arguments: map[string]any{"agent_id": agent.ID, "agent_runtime_id": runtime.ID, "infra_runtime_id": runtime.InfraRuntimeID, "action": action, "reason": reason, "authorization_ref": fmt.Sprintf("agent-runtime-grant://%s/%s/%d", agent.ID, runtime.ID, expectedVersion), "expected_resource_version": expectedVersion},
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
	})
	if err != nil {
		return nil, err
	}
	bound, applied, bindErr := s.store.BindAgentRuntimeTask(ctx, runtime.ID, runtime.ResourceVersion, task.ID, action, iapiserver.AgentRuntimeStateStopping)
	if bindErr != nil {
		return nil, bindErr
	}
	if !applied {
		return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "runtime stop task lost its resource-version fence")
	}
	return bound, nil
}

func (s *Service) currentRuntime(ctx context.Context, agent *iapiserver.Agent) (*iapiserver.AgentRuntimeBinding, error) {
	return s.store.GetCurrentAgentRuntime(ctx, agent.ID, agent.OwnerUserID)
}

// ProjectTaskTerminal 按固定 registry output 单调投影 Agent Runtime 或 Invocation，不写 Infra 或 Task 私表。
func (s *Service) ProjectTaskTerminal(ctx context.Context, task *iapiserver.AtomicTask) error {
	if task == nil {
		return nil
	}
	if task.FunctionRef == iapiserver.AgentInvocationFunctionExecute {
		return s.projectInvocationTaskTerminal(ctx, task)
	}
	if task.FunctionRef != iapiserver.AgentRuntimeFunctionEnsure && task.FunctionRef != iapiserver.AgentRuntimeFunctionStop {
		return nil
	}
	runtimeID, _ := task.Arguments["agent_runtime_id"].(string)
	if runtimeID == "" {
		return errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "agent runtime task has no aggregate reference")
	}
	runtime, err := s.store.GetAgentRuntimeByID(ctx, runtimeID)
	if err != nil {
		return err
	}
	operation, _ := task.Arguments["operation"].(string)
	if operation == "" {
		operation, _ = task.Arguments["action"].(string)
	}
	expectedVersion, ok := runtimeTaskExpectedVersion(task.Arguments["expected_resource_version"])
	if operation == "" || !ok {
		return errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "agent runtime task has invalid lifecycle fence")
	}
	if !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil
	}
	agentStatus := iapiserver.AgentStatusError
	if task.Status == iapiserver.AtomicTaskStatusSuccess {
		runtime.InfraRuntimeID, _ = task.Output["infra_runtime_id"].(string)
		runtime.EndpointRef, _ = task.Output["endpoint_ref"].(string)
		if task.FunctionRef == iapiserver.AgentRuntimeFunctionEnsure {
			runtime.State, runtime.ActivityState, runtime.HealthStatus, runtime.StartedAt = iapiserver.AgentRuntimeStateReady, iapiserver.AgentRuntimeActivityIdle, iapiserver.AgentRuntimeHealthHealthy, imachinery.Now()
			agentStatus = iapiserver.AgentStatusIdle
		} else {
			runtime.State, runtime.ActivityState, runtime.HealthStatus, runtime.StoppedAt = iapiserver.AgentRuntimeStateStopped, iapiserver.AgentRuntimeActivitySuspended, iapiserver.AgentRuntimeHealthUnknown, imachinery.Now()
			action, _ := task.Arguments["action"].(string)
			if action == iapiserver.AgentRuntimeActionDelete {
				agentStatus = iapiserver.AgentStatusDeleting
			} else {
				agentStatus = iapiserver.AgentStatusSuspended
			}
		}
	} else if iapiserver.IsAtomicTaskTerminal(task.Status) {
		runtime.State, runtime.HealthStatus = iapiserver.AgentRuntimeStateFailed, iapiserver.AgentRuntimeHealthUnhealthy
	}
	projectedRuntime, applied, err := s.store.ProjectAgentRuntimeTerminal(ctx, store.AgentRuntimeTerminalProjection{
		TaskID: task.ID, Operation: operation, ExpectedResourceVersion: expectedVersion, Runtime: runtime, AgentStatus: agentStatus,
	})
	if err != nil {
		return err
	}
	if task.Status != iapiserver.AtomicTaskStatusSuccess {
		if applied && task.FunctionRef == iapiserver.AgentRuntimeFunctionEnsure {
			authorizationRef, _ := task.Arguments["authorization_ref"].(string)
			var revokeErr error
			if authorizationRef == "" {
				revokeErr = errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "agent runtime ensure task has no authorization reference")
			} else {
				revokeErr = s.store.RevokeAgentRuntimeGrant(ctx, authorizationRef)
			}
			return stderrors.Join(revokeErr, s.failQueuedInvocationSubmissions(ctx, runtime.AgentID))
		}
		return nil
	}
	if applied && task.FunctionRef == iapiserver.AgentRuntimeFunctionStop {
		if err := s.store.RevokeActiveAgentRuntimeGrants(ctx, runtime.ID); err != nil {
			return err
		}
	}
	if task.FunctionRef != iapiserver.AgentRuntimeFunctionEnsure ||
		projectedRuntime == nil || projectedRuntime.State != iapiserver.AgentRuntimeStateReady {
		return nil
	}
	// 重复终态投影仍需恢复 READY Runtime 的队列：首次投影可能已提交，随后 Task Center 提交失败。
	// AtomicTask 幂等键和 Invocation resource-version fence 保证该扫描不会重复绑定执行任务。
	return s.submitQueuedInvocations(ctx, projectedRuntime, task.CreatedBy)
}

func (s *Service) failQueuedInvocationSubmissions(ctx context.Context, agentID string) error {
	invocations, err := s.store.ListQueuedAgentInvocationsByAgent(ctx, agentID)
	if err != nil {
		return err
	}
	var errs []error
	for _, invocation := range invocations {
		if _, _, failErr := s.store.FailAgentInvocationSubmission(
			ctx,
			invocation.ID,
			invocation.ResourceVersion,
			invocation.SubmissionGeneration,
			"Agent runtime failed before invocation task submission.",
		); failErr != nil {
			errs = append(errs, failErr)
		}
	}
	return stderrors.Join(errs...)
}

func (s *Service) projectInvocationTaskTerminal(ctx context.Context, task *iapiserver.AtomicTask) error {
	if !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil
	}
	invocationID, _ := task.Arguments[iapiserver.AgentInvocationTaskKeyInvocationID].(string)
	expectedVersion, ok := runtimeTaskExpectedVersion(task.Arguments[iapiserver.AgentInvocationTaskKeyExpectedResourceVersion])
	if invocationID == "" || !ok {
		return errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "agent invocation task has invalid aggregate fence")
	}

	projection := store.AgentInvocationTerminalProjection{
		TaskID: task.ID, ExpectedResourceVersion: expectedVersion,
		RuntimeSessionRef:    taskOutputString(task.Output, "runtime_session_ref"),
		RuntimeInvocationRef: taskOutputString(task.Output, "runtime_invocation_ref"),
		AssistantMessageID:   taskOutputString(task.Output, "assistant_message_id"),
		FailureCode:          taskOutputString(task.Output, "failure_code"),
		FailureMessage:       taskOutputString(task.Output, "failure_message"),
	}
	if sequence, sequenceOK := runtimeTaskExpectedVersion(task.Output["last_event_sequence"]); sequenceOK && sequence >= 0 {
		projection.LastEventSequence = int(sequence)
	}

	switch task.Status {
	case iapiserver.AtomicTaskStatusSuccess:
		outputInvocationID := taskOutputString(task.Output, "invocation_id")
		projection.Status = taskOutputString(task.Output, "invocation_status")
		if outputInvocationID != invocationID || !agentInvocationTerminalStatus(projection.Status) {
			return errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "agent invocation task returned an invalid terminal result")
		}
	case iapiserver.AtomicTaskStatusCanceled:
		projection.Status = iapiserver.AgentInvocationStatusCanceled
	default:
		projection.Status = iapiserver.AgentInvocationStatusFailed
		if projection.FailureCode == "" {
			projection.FailureCode = iapiserver.AgentInvocationFailureCodeTaskUnavailable
		}
		if projection.FailureMessage == "" {
			projection.FailureMessage = task.LastError.Message
		}
	}
	projected, _, err := s.store.ProjectAgentInvocationTerminal(ctx, invocationID, projection)
	if err != nil || projected == nil || !agentInvocationTerminalStatus(projected.Status) {
		return err
	}
	return s.ensureInvocationTerminalEvent(ctx, projected)
}

func (s *Service) ensureInvocationTerminalEvent(ctx context.Context, invocation *iapiserver.AgentInvocation) error {
	afterSequence := invocation.LastEventSequence - 1
	if afterSequence < 0 {
		afterSequence = 0
	}
	events, err := s.store.ListAgentOperationEvents(ctx, invocation.ID, afterSequence)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.EventType == iapiserver.AgentOperationEventTypeInvocationCompleted ||
			event.EventType == iapiserver.AgentOperationEventTypeInvocationFailed ||
			event.EventType == iapiserver.AgentOperationEventTypeInvocationCanceled {
			return nil
		}
		if event.SequenceNo > invocation.LastEventSequence {
			invocation.LastEventSequence = event.SequenceNo
		}
	}

	eventType := iapiserver.AgentOperationEventTypeInvocationFailed
	payload := any(&iapiserver.AgentInvocationFailedEventPayload{
		Status:         iapiserver.AgentInvocationStatusFailed,
		FailureCode:    invocation.FailureCode,
		FailureMessage: invocation.FailureMessage,
	})
	switch invocation.Status {
	case iapiserver.AgentInvocationStatusSucceeded:
		eventType = iapiserver.AgentOperationEventTypeInvocationCompleted
		payload = &iapiserver.AgentInvocationCompletedEventPayload{Status: invocation.Status, AssistantMessageID: invocation.AssistantMessageID}
	case iapiserver.AgentInvocationStatusCanceled:
		eventType = iapiserver.AgentOperationEventTypeInvocationCanceled
		payload = &iapiserver.AgentInvocationCanceledEventPayload{Status: invocation.Status}
	default:
		if invocation.FailureCode == "" {
			invocation.FailureCode = iapiserver.AgentInvocationFailureCodeInvocationFailed
		}
		if invocation.FailureMessage == "" {
			invocation.FailureMessage = "Agent invocation execution failed."
		}
		payload = &iapiserver.AgentInvocationFailedEventPayload{
			Status: invocation.Status, FailureCode: invocation.FailureCode, FailureMessage: invocation.FailureMessage,
		}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal agent invocation terminal event payload: %w", err)
	}
	sequence := invocation.LastEventSequence + 1
	_, err = s.store.AppendAgentOperationEvent(ctx, &iapiserver.AgentOperationEvent{
		ObjectMeta: imachinery.ObjectMeta{
			ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("agent-operation-event:%s:%d", invocation.ID, sequence))).String(),
		},
		InvocationID: invocation.ID,
		EventType:    eventType,
		SequenceNo:   sequence,
		Payload:      payloadJSON,
	})
	return err
}

func taskOutputString(output map[string]any, key string) string {
	value, _ := output[key].(string)
	return value
}

func agentInvocationTerminalStatus(status string) bool {
	return status == iapiserver.AgentInvocationStatusSucceeded ||
		status == iapiserver.AgentInvocationStatusFailed ||
		status == iapiserver.AgentInvocationStatusCanceled
}

func (s *Service) submitQueuedInvocations(ctx context.Context, runtime *iapiserver.AgentRuntimeBinding, ownerUserID string) error {
	agent, err := s.store.GetAgent(ctx, runtime.AgentID, ownerUserID)
	if err != nil {
		return err
	}
	if agent.Disabled || agent.Status == iapiserver.AgentStatusDisabled || agent.Status == iapiserver.AgentStatusDeleting || agent.Status == iapiserver.AgentStatusSuspended {
		return nil
	}
	invocations, err := s.store.ListQueuedAgentInvocationsByAgent(ctx, agent.ID)
	if err != nil {
		return err
	}
	for _, invocation := range invocations {
		if _, err := s.submitInvocationTask(ctx, agent, invocation, runtime); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileQueuedInvocations 为 READY Runtime 恢复尚未绑定 Task 的 Invocation；Task 幂等键和资源版本栅栏防止重复提交。
func (s *Service) ReconcileQueuedInvocations(ctx context.Context) error {
	candidates, err := s.store.ListAgentRuntimeQueueCandidates(ctx, 200)
	if err != nil {
		return err
	}
	var errs []error
	for _, candidate := range candidates {
		runtime, runtimeErr := s.store.GetAgentRuntimeByID(ctx, candidate.RuntimeID)
		if runtimeErr != nil {
			errs = append(errs, runtimeErr)
			continue
		}
		if runtime == nil || runtime.AgentID != candidate.AgentID || runtime.State != iapiserver.AgentRuntimeStateReady {
			continue
		}
		if submitErr := s.submitQueuedInvocations(ctx, runtime, candidate.OwnerUserID); submitErr != nil {
			errs = append(errs, submitErr)
		}
	}
	return stderrors.Join(errs...)
}

func runtimeTaskExpectedVersion(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		converted := int64(typed)
		return converted, typed == float64(converted)
	default:
		return 0, false
	}
}

func currentUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "", errors.NewStatus(code.ErrAgentAccessDenied, "authenticated user is required")
	}
	return user.ID, nil
}

func modelBinding(agentID, name string, input *iapiserver.AgentModelBindingInput) *iapiserver.AgentModelBinding {
	return &iapiserver.AgentModelBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: name}, AgentID: agentID, SourceType: input.SourceType, SourceRef: input.SourceRef, Purpose: input.Purpose, Status: iapiserver.AgentModelBindingStatusActive, IsPrimary: true}
}

func primaryModel(bindings []*iapiserver.AgentModelBinding) *iapiserver.AgentModelBinding {
	for _, binding := range bindings {
		if binding != nil && binding.IsPrimary && binding.Status == iapiserver.AgentModelBindingStatusActive {
			return binding
		}
	}
	return nil
}
func defaultPurpose(kind string) string {
	if kind == iapiserver.AgentKindCoding {
		return iapiserver.AgentModelBindingPurposeCoding
	}
	return iapiserver.AgentModelBindingPurposeChat
}

func stableCodingAgentID(idempotencyKey string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("appstudio:coding-agent:"+idempotencyKey)).String()
}

func stableCodingAgentChildID(agentID, child string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("agent:"+agentID+":"+child)).String()
}

func codingAgentMatchesStudio(agent *iapiserver.Agent, workspaceID string) bool {
	return agent != nil && agent.Kind == iapiserver.AgentKindCoding &&
		agent.WorkspaceType == iapiserver.AgentWorkspaceTypeStudio && agent.WorkspaceID == workspaceID
}

func isInvocationTerminal(status string) bool {
	return status == iapiserver.AgentInvocationStatusSucceeded || status == iapiserver.AgentInvocationStatusFailed || status == iapiserver.AgentInvocationStatusCanceled
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
