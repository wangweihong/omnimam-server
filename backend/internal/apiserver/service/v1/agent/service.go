package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/generic"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
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

// Service 实现 released Agent API、固定 Workspace、交互持久化和 Runtime Task 编排。
type Service struct {
	store      store.AgentStore
	tasks      TaskClient
	workspaces WorkspaceBindingValidator
	models     ModelAccessResolver
	profiles   map[string]iapiserver.AgentProfile
}

// Dependencies 是 Agent service 显式消费方依赖。
type Dependencies struct {
	Store      store.AgentStore
	Tasks      TaskClient
	Workspaces WorkspaceBindingValidator
	Models     ModelAccessResolver
}

// New 构造 Agent service；缺失 Task Center 时 Runtime 操作 fail closed。
func New(deps Dependencies) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("agent store is required")
	}
	return &Service{store: deps.Store, tasks: deps.Tasks, workspaces: deps.Workspaces, models: deps.Models, profiles: defaultProfiles()}, nil
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

// CreateCodingAgentForStudio 仅供 AppStudio 在应用初始化期间创建固定 Coding Agent。
func (s *Service) CreateCodingAgentForStudio(
	ctx context.Context,
	studioApplicationID string,
	workspaceID string,
	ownerUserID string,
	idempotencyKey string,
) (*iapiserver.Agent, error) {
	if studioApplicationID == "" || workspaceID == "" || ownerUserID == "" || idempotencyKey == "" {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent initialization context is incomplete")
	}
	if s.workspaces == nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "appstudio workspace validation is unavailable")
	}
	profile, ok := s.profiles[iapiserver.AgentProfileIDCoding]
	if !ok || profile.Status != iapiserver.AgentProfileStatusActive || !sets.NewString(profile.SupportedAgentKinds...).Has(iapiserver.AgentKindCoding) {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent profile is unavailable")
	}
	authorization, err := generic.GetValueOrZero(s.workspaces.ValidateAgentWorkspaceBinding(ctx, ownerUserID, workspaceID))
	if err != nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}

	agentID := stableCodingAgentID(idempotencyKey)
	if existing, getErr := s.store.GetAgent(ctx, agentID, ownerUserID); getErr == nil {
		if codingAgentMatchesStudio(existing, workspaceID) {
			return existing, nil
		}
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent idempotency key conflicts")
	}

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
		AccessMode: iapiserver.AgentWorkspaceAccessModeReadWrite, AuthorizationSummary: authorization,
	}
	model := &iapiserver.AgentModelBinding{
		ObjectMeta: imachinery.ObjectMeta{ID: stableCodingAgentChildID(agentID, "primary-model"), Name: "primary-model"},
		AgentID:    agentID, SourceType: iapiserver.AgentModelBindingSourceTypeUserDefault, SourceRef: iapiserver.AgentModelBindingSourceRefUserDefault,
		Purpose: defaultPurpose(iapiserver.AgentKindCoding), Status: iapiserver.AgentModelBindingStatusActive, IsPrimary: true,
	}
	if err := s.store.CreateAgentAggregate(ctx, agent, session, binding, model); err != nil {
		existing, getErr := s.store.GetAgent(ctx, agentID, ownerUserID)
		if getErr == nil && codingAgentMatchesStudio(existing, workspaceID) {
			return existing, nil
		}
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}
	return agent, nil
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

// SendMessage 持久化消息和 Invocation；执行适配器不可用时立即进入失败终态。
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
	if typeName != iapiserver.AgentInvocationTypeChat {
		return nil, errors.NewStatus(code.ErrAgentInvocationTaskUnavailable, "Released SSOT has no canonical non-runtime Agent Invocation functionRef.")
	}
	messageID, invocationID := uuid.NewString(), uuid.NewString()
	message := &iapiserver.AgentMessage{ObjectMeta: imachinery.ObjectMeta{ID: messageID}, SessionID: session.ID, AgentID: agent.ID, InvocationID: invocationID, Role: iapiserver.AgentMessageRoleUser, Content: req.Content, Attachments: req.Attachments}
	invocation := &iapiserver.AgentInvocation{ObjectMeta: imachinery.ObjectMeta{ID: invocationID}, AgentID: agent.ID, SessionID: session.ID, Type: typeName, Status: iapiserver.AgentInvocationStatusQueued, UserMessageID: messageID, IdempotencyKey: idempotency}
	created, err := s.store.CreateAgentInvocation(ctx, message, invocation)
	if err != nil {
		return nil, err
	}
	if created.ID != invocationID {
		return created, nil
	}
	created.Status = iapiserver.AgentInvocationStatusFailed
	created.FailureCode = iapiserver.AgentInvocationFailureCodeTaskUnavailable
	created.FailureMessage = "Agent execution adapter is unavailable for " + typeName + " invocation."
	created.CompletedAt = imachinery.Now()
	if _, err := s.store.UpdateAgentInvocation(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
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
	invocation.Status, invocation.CompletedAt = iapiserver.AgentInvocationStatusCanceled, imachinery.Now()
	return s.store.UpdateAgentInvocation(ctx, invocation)
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
	items, err := s.store.ListAgentMCPBindings(ctx, agentID, owner)
	return &iapiserver.AgentMCPBindingListResponse{Total: int64(len(items)), Items: items}, err
}
func (s *Service) CreateMCPBinding(ctx context.Context, agentID string, req *iapiserver.AgentMCPBindingRequest) (*iapiserver.AgentMCPBinding, error) {
	owner, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	binding := &iapiserver.AgentMCPBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: req.Name}, AgentID: agentID, ServerType: req.ServerType, EndpointRef: req.EndpointRef, CredentialRef: req.CredentialRef, AllowedTools: req.AllowedTools, Configuration: req.Configuration, Enabled: true}
	return s.store.CreateAgentMCPBinding(ctx, owner, binding)
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
	if runtime == nil {
		runtime = &iapiserver.AgentRuntimeBinding{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AgentID: agent.ID, RuntimeProfileID: agent.AgentProfileID, RuntimeProfileRevision: agent.AgentProfileRevision, State: iapiserver.AgentRuntimeStateStarting, ActivityState: iapiserver.AgentRuntimeActivityIdle, HealthStatus: iapiserver.AgentRuntimeHealthUnknown}
		if _, err := s.store.CreateAgentRuntime(ctx, agent.OwnerUserID, runtime); err != nil {
			return nil, err
		}
	} else {
		runtime.State = iapiserver.AgentRuntimeStateStarting
		if _, err := s.store.UpdateAgentRuntime(ctx, runtime); err != nil {
			return nil, err
		}
	}
	bindings, err := s.store.ListAgentModelBindings(ctx, agent.ID, agent.OwnerUserID)
	if err != nil {
		return nil, err
	}
	primary := primaryModel(bindings)
	modelAccessRef := "model-access://user-default/" + strings.ToLower(defaultPurpose(agent.Kind))
	if s.models != nil {
		modelAccessRef, err = s.models.ResolveAgentModelAccess(ctx, agent.OwnerUserID, primary)
		if err != nil {
			return nil, errors.NewStatus(code.ErrAgentModelBindingInvalid, err.Error())
		}
	} else if primary != nil {
		modelAccessRef = "model-access://" + primary.SourceType + "/" + primary.SourceRef
	}
	workspaceSource := any(nil)
	if agent.Kind == iapiserver.AgentKindPlatform {
		workspaceSource = "agent-workspace://" + agent.WorkspaceID
	}
	arguments := map[string]any{
		"agent_id": agent.ID, "agent_runtime_id": runtime.ID, "operation": operation, "agent_kind": agent.Kind,
		"workspace_type": agent.WorkspaceType, "workspace_id": agent.WorkspaceID, "workspace_source_ref": workspaceSource,
		"runtime_profile_id": runtime.RuntimeProfileID, "runtime_profile_revision": runtime.RuntimeProfileRevision,
		"model_access_spec_ref": modelAccessRef, "runtime_configuration_ref": "agent-runtime-config://" + agent.ID,
		"authorization_ref":         fmt.Sprintf("agent-runtime-grant://%s/%s/%d", agent.ID, runtime.ID, agent.ResourceVersion),
		"expected_resource_version": runtime.ResourceVersion,
	}
	if runtime.InfraRuntimeID != "" {
		arguments["existing_infra_runtime_id"] = runtime.InfraRuntimeID
	}
	task, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AgentTaskDomain, &iapiserver.AtomicTaskCreateRequest{
		Key: "runtime-" + runtime.ID, Name: "Agent runtime " + strings.ToLower(operation), FunctionRef: iapiserver.AgentRuntimeFunctionEnsure,
		Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
	})
	if err != nil {
		runtime.State = iapiserver.AgentRuntimeStateFailed
		_, _ = s.store.UpdateAgentRuntime(ctx, runtime)
		return nil, err
	}
	agent.Status = iapiserver.AgentStatusStarting
	_, _ = s.store.UpdateAgent(ctx, agent, agent.ResourceVersion)
	_ = task
	return runtime, nil
}

func (s *Service) SuspendRuntime(ctx context.Context, agentID string, req *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error) {
	return s.stopRuntimeAction(ctx, agentID, iapiserver.AgentRuntimeActionSuspend, req)
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
	_, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AgentTaskDomain, &iapiserver.AtomicTaskCreateRequest{
		Key: "runtime-stop-" + runtime.ID, Name: "Agent runtime stop", FunctionRef: iapiserver.AgentRuntimeFunctionStop,
		Arguments: map[string]any{"agent_id": agent.ID, "agent_runtime_id": runtime.ID, "infra_runtime_id": runtime.InfraRuntimeID, "action": action, "reason": reason, "authorization_ref": fmt.Sprintf("agent-runtime-grant://%s/%s/%d", agent.ID, runtime.ID, agent.ResourceVersion), "expected_resource_version": runtime.ResourceVersion},
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
	})
	if err != nil {
		return nil, err
	}
	runtime.State = iapiserver.AgentRuntimeStateStopping
	return s.store.UpdateAgentRuntime(ctx, runtime)
}

func (s *Service) currentRuntime(ctx context.Context, agent *iapiserver.Agent) (*iapiserver.AgentRuntimeBinding, error) {
	return s.store.GetCurrentAgentRuntime(ctx, agent.ID, agent.OwnerUserID)
}

// ProjectTaskTerminal 按固定 registry output 单调投影 AgentRuntime，不写 Infra 或 Task 私表。
func (s *Service) ProjectTaskTerminal(ctx context.Context, task *iapiserver.AtomicTask) error {
	if task == nil || (task.FunctionRef != iapiserver.AgentRuntimeFunctionEnsure && task.FunctionRef != iapiserver.AgentRuntimeFunctionStop) {
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
	_, err = s.store.ProjectAgentRuntime(ctx, runtime, agentStatus)
	return err
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
