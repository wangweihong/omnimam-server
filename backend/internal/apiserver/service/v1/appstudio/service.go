package appstudio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

const (
	maxStudioFileBytes        = 2 << 20
	maxStudioContentReadBytes = 1 << 20
)

type TaskClient interface {
	CreateDomainAtomicTask(context.Context, string, *iapiserver.AtomicTaskCreateRequest) (*iapiserver.AtomicTask, error)
	CancelAtomicTask(context.Context, string, *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error)
}

type ArtifactReader interface {
	GetArtifact(context.Context, string, string) (*iapiserver.Artifact, error)
}

type CodingAgentCreator interface {
	PrepareCodingAgentForStudio(context.Context, string, string, string, string, string, *iapiserver.AgentModelBindingInput, *iapiserver.AgentAuthorizationSummary) (*iapiserver.Agent, *iapiserver.AgentSession, *iapiserver.AgentWorkspaceBinding, *iapiserver.AgentModelBinding, error)
	StartCodingInvocation(context.Context, string, string) (*iapiserver.AgentInvocation, error)
	GetCodingAgentForStudio(context.Context, string, string, string) (*iapiserver.Agent, *iapiserver.AgentSession, error)
	GetCodingModelBindingForStudio(context.Context, string, string, string) (*iapiserver.AgentModelBinding, error)
	SendMessage(context.Context, string, *iapiserver.AgentMessageRequest) (*iapiserver.AgentInvocation, error)
	ListInvocations(context.Context, string, *iapiserver.AgentInvocationListRequest) (*iapiserver.AgentInvocationListResponse, error)
	GetInvocation(context.Context, string) (*iapiserver.AgentInvocation, error)
	CancelInvocation(context.Context, string, *iapiserver.AgentActionRequest) (*iapiserver.AgentInvocation, error)
	ListInvocationEvents(context.Context, string, int) ([]*iapiserver.AgentOperationEvent, error)
	SuspendCodingAgentForStudio(context.Context, string, string, string, *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error)
	ResumeCodingAgentForStudio(context.Context, string, string, string, *iapiserver.AgentActionRequest) (*iapiserver.AgentRuntimeBinding, error)
	EnsurePlatformMCPBindingForCodingAgent(context.Context, string, string) error
}

type Service struct {
	store     store.AppStudioStore
	tasks     TaskClient
	sources   SourceContentStore
	artifacts ArtifactReader
	agents    CodingAgentCreator
}
type Dependencies struct {
	Store     store.AppStudioStore
	Tasks     TaskClient
	Sources   SourceContentStore
	Artifacts ArtifactReader
}

func New(deps Dependencies) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("appstudio store is required")
	}
	if deps.Sources == nil {
		return nil, fmt.Errorf("appstudio source content store is required")
	}
	return &Service{store: deps.Store, tasks: deps.Tasks, sources: deps.Sources, artifacts: deps.Artifacts}, nil
}

func (s *Service) SetCodingAgentCreator(creator CodingAgentCreator) {
	s.agents = creator
}

// ResolveAgentWorkloadScope resolves only the current Coding Agent generation
// for the owner-bound Studio application.
func (s *Service) ResolveAgentWorkloadScope(ctx context.Context, agentID, owner string) (string, int64, error) {
	app, err := s.store.GetStudioApplicationByCodingAgent(ctx, agentID, owner)
	if err != nil || app == nil || app.CodingAgentID != agentID || app.CodingAgentGeneration < 1 {
		return "", 0, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "coding agent workload scope is unavailable")
	}
	return app.ID, int64(app.CodingAgentGeneration), nil
}

// ResolveAgentWorkloadOwner revalidates that a workload still represents the
// current Coding Agent generation and returns only its owning user boundary.
func (s *Service) ResolveAgentWorkloadOwner(ctx context.Context, applicationID, agentID string, generation int64) (string, error) {
	app, err := s.store.GetStudioApplicationWorkloadScope(ctx, applicationID, agentID, generation)
	if err != nil || app == nil || app.OwnerUserID == "" {
		return "", errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "coding agent workload scope is unavailable")
	}
	return app.OwnerUserID, nil
}

func (s *Service) ListApplications(ctx context.Context, req *iapiserver.StudioApplicationListRequest) (*iapiserver.StudioApplicationListResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = owner
	items, total, err := s.store.ListStudioApplications(ctx, req)
	return &iapiserver.StudioApplicationListResponse{Total: total, Items: items}, err
}
func (s *Service) CreateApplication(ctx context.Context, req *iapiserver.StudioApplicationCreateRequest) (*iapiserver.StudioApplicationCreateResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	applicationType := req.ApplicationType
	if applicationType == "" {
		applicationType = iapiserver.AppStudioApplicationTypeStaticWeb
	}
	if applicationType != iapiserver.AppStudioApplicationTypeStaticWeb || req.BackendRequired {
		return nil, errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "only STATIC_WEB applications are supported")
	}
	if s.agents == nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent initialization is unavailable")
	}

	appID := stableStudioInitializationID(owner, req.IdempotencyKey, "application")
	repoID := stableStudioInitializationID(owner, req.IdempotencyKey, "repository")
	workspaceID := stableStudioInitializationID(owner, req.IdempotencyKey, "workspace")
	app := &iapiserver.StudioApplication{
		ObjectMeta:  imachinery.ObjectMeta{ID: appID, Name: req.Name, Description: req.Description},
		OwnerUserID: owner, Status: iapiserver.AppStudioApplicationStatusReady, DefaultWorkspaceID: workspaceID,
		CodingAgentGeneration: 1, CreateIdempotencyKey: req.IdempotencyKey,
	}
	repository := &iapiserver.StudioSourceRepository{ObjectMeta: imachinery.ObjectMeta{ID: repoID, Name: req.Name + " source"}, StudioApplicationID: appID, ProviderType: iapiserver.AppStudioSourceProviderBuiltIn, Status: iapiserver.AppStudioRepositoryStatusReady}
	workspace := &iapiserver.StudioWorkspace{ObjectMeta: imachinery.ObjectMeta{ID: workspaceID, Name: iapiserver.AppStudioDefaultWorkspaceName}, StudioApplicationID: appID, RepositoryID: repoID, Status: iapiserver.AppStudioWorkspaceStatusReady, CurrentRevisionDigest: emptyTreeDigest()}
	revision := &iapiserver.StudioWorkspaceRevision{ObjectMeta: imachinery.ObjectMeta{ID: stableStudioInitializationID(owner, req.IdempotencyKey, "revision-0")}, WorkspaceID: workspaceID, Revision: 0, ContentDigest: emptyTreeDigest(), CreatedBy: owner}
	if err := s.sources.WriteRevision(ctx, workspaceID, 0, map[string][]byte{}); err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, err.Error())
	}
	modelInput := &iapiserver.AgentModelBindingInput{SourceType: req.CodingModelSelection.SourceType, SourceRef: req.CodingModelSelection.SourceRef, Purpose: iapiserver.AgentModelBindingPurposeCoding}
	authorization := &iapiserver.AgentAuthorizationSummary{Source: iapiserver.AppStudioTaskDomain, ValidatedAt: imachinery.Now()}
	agent, session, workspaceBinding, modelBinding, err := s.agents.PrepareCodingAgentForStudio(ctx, appID, workspaceID, owner, appID, req.CodingAgentProfile, modelInput, authorization)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}
	mcpBinding := studioPlatformMCPBinding(agent, appID)
	app.CodingAgentID, app.CodingSessionID = agent.ID, session.ID
	messageID := stableStudioInitializationID(owner, req.IdempotencyKey, "initial-message")
	invocationID := stableStudioInitializationID(owner, req.IdempotencyKey, "initial-invocation")
	attachments := make([]iapiserver.AgentReference, 0, len(req.Attachments))
	for _, attachment := range req.Attachments {
		attachments = append(attachments, iapiserver.AgentReference{ReferenceType: attachment.Type, ReferenceID: attachment.ReferenceID})
	}
	message := &iapiserver.AgentMessage{ObjectMeta: imachinery.ObjectMeta{ID: messageID}, SessionID: session.ID, AgentID: agent.ID, InvocationID: invocationID, Role: iapiserver.AgentMessageRoleUser, Content: req.InitialRequirement, Attachments: attachments}
	invocation := &iapiserver.AgentInvocation{ObjectMeta: imachinery.ObjectMeta{ID: invocationID}, AgentID: agent.ID, SessionID: session.ID, Type: iapiserver.AgentInvocationTypeCoding, Status: iapiserver.AgentInvocationStatusQueued, UserMessageID: messageID, IdempotencyKey: "studio-create:" + req.IdempotencyKey}
	initialization := &store.StudioApplicationInitialization{Application: app, Repository: repository, Workspace: workspace, Revision: revision, Agent: agent, Session: session, WorkspaceBinding: workspaceBinding, ModelBinding: modelBinding, MCPBinding: mcpBinding, UserMessage: message, InitialInvocation: invocation}
	if _, err := s.store.CreateStudioApplicationInitialization(ctx, initialization); err != nil {
		return nil, err
	}
	canonical, err := s.store.GetStudioApplicationInitialization(ctx, owner, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if err := s.agents.EnsurePlatformMCPBindingForCodingAgent(ctx, canonical.Agent.ID, canonical.Application.ID); err != nil {
		return nil, err
	}
	if _, err := s.agents.StartCodingInvocation(ctx, canonical.Agent.ID, canonical.InitialInvocation.ID); err != nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}
	canonical, err = s.store.GetStudioApplicationInitialization(ctx, owner, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return studioApplicationCreateResponse(canonical), nil
}
func (s *Service) GetApplication(ctx context.Context, id string) (*iapiserver.StudioApplication, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetStudioApplication(ctx, id, owner)
}
func (s *Service) UpdateApplication(ctx context.Context, id string, req *iapiserver.StudioApplicationUpdateRequest) (*iapiserver.StudioApplication, error) {
	app, err := s.GetApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status == iapiserver.AppStudioApplicationStatusArchived {
		return nil, errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "archived studio application is immutable")
	}
	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Description != nil {
		app.Description = *req.Description
	}
	return s.store.UpdateStudioApplication(ctx, app, req.ResourceVersion)
}
func (s *Service) ArchiveApplication(ctx context.Context, id string) (*iapiserver.StudioApplication, error) {
	app, err := s.GetApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status == iapiserver.AppStudioApplicationStatusArchived {
		return app, nil
	}
	app.Status = iapiserver.AppStudioApplicationStatusArchived
	return s.store.UpdateStudioApplication(ctx, app, app.ResourceVersion)
}

// GetAgentStatus 返回当前 generation 的脱敏 Coding Agent/Session 状态。
func (s *Service) GetAgentStatus(ctx context.Context, appID string) (*iapiserver.StudioAgentStatus, error) {
	app, agent, _, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	return studioAgentStatus(app, agent), nil
}

// SendAgentMessage 持久化应用开发指令并返回当前 generation 的 CODING Invocation 投影。
func (s *Service) SendAgentMessage(ctx context.Context, appID string, req *iapiserver.StudioAgentMessageRequest) (*iapiserver.StudioAgentInvocation, error) {
	app, _, session, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	attachments := make([]iapiserver.AgentReference, 0, len(req.Attachments))
	for _, attachment := range req.Attachments {
		attachments = append(attachments, iapiserver.AgentReference{ReferenceType: attachment.Type, ReferenceID: attachment.ReferenceID})
	}
	invocation, err := s.agents.SendMessage(ctx, session.ID, &iapiserver.AgentMessageRequest{Content: req.Instruction, Attachments: attachments, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	if err := validateStudioInvocationBinding(app, invocation); err != nil {
		return nil, err
	}
	return s.projectStudioAgentInvocation(ctx, app, invocation)
}

// ListAgentInvocations 返回当前 generation 的 CODING Invocation 列表。
func (s *Service) ListAgentInvocations(ctx context.Context, appID string, req *iapiserver.AgentInvocationListRequest) (*iapiserver.StudioAgentInvocationListResponse, error) {
	app, _, session, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	result, err := s.agents.ListInvocations(ctx, session.ID, req)
	if err != nil {
		return nil, err
	}
	for _, invocation := range result.Items {
		if err := validateStudioInvocationBinding(app, invocation); err != nil {
			return nil, err
		}
	}
	items, err := s.projectStudioAgentInvocations(ctx, app, result.Items)
	if err != nil {
		return nil, err
	}
	return &iapiserver.StudioAgentInvocationListResponse{Total: result.Total, Items: items}, nil
}

// GetAgentInvocation 返回当前 generation 的单个 CODING Invocation。
func (s *Service) GetAgentInvocation(ctx context.Context, appID, invocationID string) (*iapiserver.StudioAgentInvocation, error) {
	app, _, _, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	invocation, err := s.agents.GetInvocation(ctx, invocationID)
	if err != nil {
		return nil, err
	}
	if err := validateStudioInvocationBinding(app, invocation); err != nil {
		return nil, err
	}
	return s.projectStudioAgentInvocation(ctx, app, invocation)
}

// CancelAgentInvocation 取消当前 generation 的 Invocation，不允许跨应用或访问旧 generation。
func (s *Service) CancelAgentInvocation(ctx context.Context, appID, invocationID string, req *iapiserver.AgentActionRequest) (*iapiserver.StudioAgentInvocation, error) {
	if _, err := s.GetAgentInvocation(ctx, appID, invocationID); err != nil {
		return nil, err
	}
	invocation, err := s.agents.CancelInvocation(ctx, invocationID, req)
	if err != nil {
		return nil, err
	}
	app, err := s.GetApplication(ctx, appID)
	if err != nil {
		return nil, err
	}
	if err := validateStudioInvocationBinding(app, invocation); err != nil {
		return nil, err
	}
	return s.projectStudioAgentInvocation(ctx, app, invocation)
}

// ListAgentInvocationEvents 在应用绑定校验后返回可恢复的持久化 Invocation 事件。
func (s *Service) ListAgentInvocationEvents(ctx context.Context, appID, invocationID string, afterSequence int) ([]*iapiserver.AgentOperationEvent, error) {
	if _, err := s.GetAgentInvocation(ctx, appID, invocationID); err != nil {
		return nil, err
	}
	return s.agents.ListInvocationEvents(ctx, invocationID, afterSequence)
}

// SuspendAgent 挂起当前 generation 的 Coding Agent Runtime。
func (s *Service) SuspendAgent(ctx context.Context, appID string, req *iapiserver.AgentActionRequest) (*iapiserver.StudioAgentStatus, error) {
	app, _, _, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	if _, err := s.agents.SuspendCodingAgentForStudio(ctx, app.CodingAgentID, app.CodingSessionID, app.DefaultWorkspaceID, req); err != nil {
		return nil, err
	}
	return s.GetAgentStatus(ctx, appID)
}

// ResumeAgent 恢复当前 generation 的 Coding Agent Runtime。
func (s *Service) ResumeAgent(ctx context.Context, appID string, req *iapiserver.AgentActionRequest) (*iapiserver.StudioAgentStatus, error) {
	app, _, _, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	if _, err := s.agents.ResumeCodingAgentForStudio(ctx, app.CodingAgentID, app.CodingSessionID, app.DefaultWorkspaceID, req); err != nil {
		return nil, err
	}
	return s.GetAgentStatus(ctx, appID)
}

// ReplaceAgent 原子创建新的 Coding Agent/Session 并切换应用 generation，旧历史保持不变。
func (s *Service) ReplaceAgent(ctx context.Context, appID string, req *iapiserver.StudioAgentReplaceRequest) (*iapiserver.StudioAgentStatus, error) {
	app, currentAgent, _, err := s.currentCodingAgent(ctx, appID)
	if err != nil {
		return nil, err
	}
	currentModel, err := s.agents.GetCodingModelBindingForStudio(ctx, app.CodingAgentID, app.CodingSessionID, app.DefaultWorkspaceID)
	if err != nil {
		return nil, err
	}
	profileID := req.CodingAgentProfile
	if profileID == "" {
		profileID = currentAgent.AgentProfileID
	}
	modelInput := &iapiserver.AgentModelBindingInput{SourceType: currentModel.SourceType, SourceRef: currentModel.SourceRef, Purpose: iapiserver.AgentModelBindingPurposeCoding}
	if req.CodingModelSelection != "" {
		modelInput.SourceRef = req.CodingModelSelection
	}
	replacementKey := app.ID + ":replace:" + req.IdempotencyKey
	authorization := &iapiserver.AgentAuthorizationSummary{Source: iapiserver.AppStudioTaskDomain, ValidatedAt: imachinery.Now()}
	agent, session, workspaceBinding, modelBinding, err := s.agents.PrepareCodingAgentForStudio(ctx, app.ID, app.DefaultWorkspaceID, app.OwnerUserID, replacementKey, profileID, modelInput, authorization)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}
	updated, err := s.store.ReplaceStudioCodingAgent(ctx, app.ID, app.OwnerUserID, &store.StudioCodingAgentReplacement{Agent: agent, Session: session, WorkspaceBinding: workspaceBinding, ModelBinding: modelBinding, MCPBinding: studioPlatformMCPBinding(agent, app.ID)})
	if err != nil {
		return nil, err
	}
	current, _, err := s.agents.GetCodingAgentForStudio(ctx, updated.CodingAgentID, updated.CodingSessionID, updated.DefaultWorkspaceID)
	if err != nil {
		return nil, err
	}
	return studioAgentStatus(updated, current), nil
}

func (s *Service) GetSource(ctx context.Context, appID string) (*iapiserver.StudioSourceState, error) {
	_, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	status := workspace.Status
	if status == iapiserver.AppStudioApplicationStatusCreating {
		status = iapiserver.AppStudioSourceStatusInitializing
	}
	return &iapiserver.StudioSourceState{StudioApplicationID: appID, CurrentRevision: workspace.CurrentRevision, Status: status, UpdatedAt: workspace.UpdatedAt}, nil
}

func (s *Service) ListFiles(ctx context.Context, appID string, req *iapiserver.StudioSourceFileListRequest) (*iapiserver.StudioSourceFileListResponse, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	revision := req.SourceRevision
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	items, err := s.store.ListStudioSourceFiles(ctx, workspace.ID, revision, req.Prefix, owner)
	return &iapiserver.StudioSourceFileListResponse{Total: int64(len(items)), Items: items}, err
}

func (s *Service) GetFileContent(ctx context.Context, appID string, req *iapiserver.StudioFileContentRequest) (*iapiserver.StudioFileContent, error) {
	_, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	revision := req.SourceRevision
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	content, err := s.sources.ReadFile(ctx, workspace.ID, revision, req.Path)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceNotVisible, "studio source file not visible")
	}
	truncated := len(content) > maxStudioContentReadBytes
	if truncated {
		content = content[:maxStudioContentReadBytes]
	}
	return &iapiserver.StudioFileContent{Path: req.Path, Content: string(content), Truncated: truncated, SourceRevision: revision}, nil
}

func (s *Service) ApplyChangeSet(ctx context.Context, appID string, req *iapiserver.StudioChangeSetRequest) (*iapiserver.StudioChangeSet, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	if workspace.Status != iapiserver.AppStudioWorkspaceStatusReady {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "studio source is not ready")
	}
	if workspace.CurrentRevision != req.BaseRevision {
		return nil, errors.NewStatus(code.ErrAppStudioSourceRevisionConflict, "source base revision conflicts")
	}
	files, err := s.loadRevision(ctx, workspace.ID, req.BaseRevision, owner)
	if err != nil {
		return nil, err
	}
	if err := applyOperations(files, req.Operations); err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, err.Error())
	}
	target := req.BaseRevision + 1
	digest, rows := revisionRows(workspace.ID, target, files)
	if err := s.sources.WriteRevision(ctx, workspace.ID, target, files); err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, err.Error())
	}
	targetPtr := target
	changeSet := &iapiserver.StudioChangeSet{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Description: req.Summary}, StudioApplicationID: appID, WorkspaceID: workspace.ID, BaseRevision: req.BaseRevision, TargetRevision: &targetPtr, ActorID: owner, AgentID: req.AgentID, AgentSessionID: req.AgentSessionID, AgentInvocationID: req.AgentInvocationID, Operations: req.Operations, Status: iapiserver.AppStudioChangeSetStatusApplied, IdempotencyKey: req.IdempotencyKey}
	parent := req.BaseRevision
	revision := &iapiserver.StudioWorkspaceRevision{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, WorkspaceID: workspace.ID, Revision: target, ContentDigest: digest, ParentRevision: &parent, CreatedBy: owner, ChangeSetID: changeSet.ID}
	result, err := s.store.ApplyStudioChangeSet(ctx, owner, changeSet, revision, rows)
	if result != nil {
		result.StudioApplicationID = appID
	}
	return result, err
}

func (s *Service) RestoreRevision(ctx context.Context, appID string, req *iapiserver.StudioRestoreRevisionRequest) (*iapiserver.StudioChangeSet, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	if workspace.CurrentRevision != req.BaseRevision {
		return nil, errors.NewStatus(code.ErrAppStudioSourceRevisionConflict, "source base revision conflicts")
	}
	source, err := s.loadRevision(ctx, workspace.ID, req.SourceRevision, owner)
	if err != nil {
		return nil, err
	}
	current, err := s.loadRevision(ctx, workspace.ID, req.BaseRevision, owner)
	if err != nil {
		return nil, err
	}
	operations := diffOperations(current, source)
	return s.ApplyChangeSet(ctx, appID, &iapiserver.StudioChangeSetRequest{BaseRevision: req.BaseRevision, IdempotencyKey: req.IdempotencyKey, Operations: operations, Summary: fmt.Sprintf("Restore revision %d", req.SourceRevision)})
}

func (s *Service) SearchSource(ctx context.Context, appID string, req *iapiserver.StudioSourceSearchRequest) (*iapiserver.StudioSourceSearchResponse, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	revision := req.SourceRevision
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	files, err := s.loadRevision(ctx, workspace.ID, revision, owner)
	if err != nil {
		return nil, err
	}
	items := make([]*iapiserver.StudioSourceSearchHit, 0)
	needle := strings.ToLower(req.Query)
	paths := sortedPaths(files)
	for _, path := range paths {
		for index, line := range strings.Split(string(files[path]), "\n") {
			if strings.Contains(strings.ToLower(line), needle) {
				items = append(items, &iapiserver.StudioSourceSearchHit{Path: path, LineNumber: index + 1, Snippet: truncateText(line, 500), SourceRevision: revision})
				if len(items) >= 500 {
					break
				}
			}
		}
		if len(items) >= 500 {
			break
		}
	}
	return &iapiserver.StudioSourceSearchResponse{Total: int64(len(items)), Items: items}, nil
}

func (s *Service) CreateSnapshot(ctx context.Context, appID string, req *iapiserver.StudioSnapshotRequest) (*iapiserver.StudioSourceSnapshot, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	revision := req.SourceRevision
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	record, err := s.store.GetStudioWorkspaceRevision(ctx, workspace.ID, revision, owner)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotInvalid, "source revision is unavailable")
	}
	files, err := s.store.ListStudioSourceFiles(ctx, workspace.ID, revision, "", owner)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotInvalid, "source revision is empty")
	}
	snapshot := &iapiserver.StudioSourceSnapshot{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, StudioApplicationID: appID, WorkspaceID: workspace.ID, WorkspaceRevision: revision, ContentDigest: record.ContentDigest, ManifestDigest: record.ContentDigest, Status: iapiserver.AppStudioSnapshotStatusReady, CreatedBy: owner}
	return s.store.CreateStudioSourceSnapshot(ctx, owner, snapshot)
}
func (s *Service) GetSnapshot(ctx context.Context, appID, id string) (*iapiserver.StudioSourceSnapshot, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.store.GetStudioSourceSnapshot(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if snapshot.StudioApplicationID != appID {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotNotVisible, "studio source snapshot not visible")
	}
	return snapshot, nil
}
func (s *Service) CreateVersion(ctx context.Context, appID string, req *iapiserver.StudioApplicationVersionCreateRequest) (*iapiserver.StudioApplicationVersion, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.store.GetStudioSourceSnapshot(ctx, req.SourceSnapshotID, owner)
	if err != nil || snapshot.Status != iapiserver.AppStudioSnapshotStatusReady || snapshot.StudioApplicationID != appID {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotInvalid, "source snapshot is invalid for application")
	}
	version := &iapiserver.StudioApplicationVersion{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: req.Version}, StudioApplicationID: appID, SourceSnapshotID: req.SourceSnapshotID, Version: req.Version, IdempotencyKey: req.IdempotencyKey, Status: iapiserver.AppStudioVersionStatusDraft}
	return s.store.CreateStudioApplicationVersion(ctx, owner, version)
}
func (s *Service) ListVersions(ctx context.Context, appID string, req *iapiserver.StudioApplicationVersionListRequest) (*iapiserver.StudioApplicationVersionListResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListStudioApplicationVersions(ctx, appID, owner, req)
	return &iapiserver.StudioApplicationVersionListResponse{Total: total, Items: items}, err
}

func (s *Service) ListBuilds(ctx context.Context, appID string, req *iapiserver.StudioBuildListRequest) (*iapiserver.StudioBuildListResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListStudioBuilds(ctx, appID, owner, req)
	return &iapiserver.StudioBuildListResponse{Total: total, Items: items}, err
}
func (s *Service) BatchBuildSummaries(ctx context.Context, req *iapiserver.StudioBuildBatchSummaryRequest) (*iapiserver.StudioBuildBatchSummaryResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		ids = append(ids, item.ID)
	}
	summaries, err := s.store.ResolveStudioBuildSummaries(ctx, owner, ids)
	if err != nil {
		return nil, err
	}
	items := make([]*iapiserver.StudioBuildBatchSummaryItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, &iapiserver.StudioBuildBatchSummaryItem{ID: item.ID, StudioBuild: summaries[item.ID]})
	}
	return &iapiserver.StudioBuildBatchSummaryResponse{Total: len(items), Items: items}, nil
}
func (s *Service) CreateBuild(ctx context.Context, appID string, req *iapiserver.StudioBuildRequest) (*iapiserver.StudioBuild, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.store.GetStudioSourceSnapshot(ctx, req.SourceSnapshotID, owner)
	if err != nil || snapshot.Status != iapiserver.AppStudioSnapshotStatusReady || snapshot.StudioApplicationID != appID {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotInvalid, "source snapshot is invalid")
	}
	if s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioBuildFailed, "task center is unavailable")
	}
	buildID := uuid.NewString()
	build := &iapiserver.StudioBuild{ObjectMeta: imachinery.ObjectMeta{ID: buildID, Name: "Build " + buildID}, OwnerUserID: owner, StudioApplicationID: appID, SourceSnapshotID: snapshot.ID, StudioApplicationVersionID: req.StudioApplicationVersionID, Status: iapiserver.AppStudioBuildStatusPending, IdempotencyKey: req.IdempotencyKey}
	created, err := s.store.CreateStudioBuild(ctx, owner, build)
	if err != nil {
		return nil, err
	}
	if created.AtomicTaskID != "" {
		return created, nil
	}
	profile := buildProfile(snapshot)
	arguments := (iapiserver.AppStudioBuildTaskArguments{StudioApplicationID: appID, StudioBuildID: created.ID, SourceSnapshotID: snapshot.ID, SourceSnapshotDigest: snapshot.ContentDigest, SourceSnapshotSourceRef: iapiserver.AppStudioRefPrefixStudioSnapshot + snapshot.ID, StudioApplicationVersionID: nullableString(req.StudioApplicationVersionID), RuntimeProfileID: profile, RuntimeProfileRevision: iapiserver.AppStudioRuntimeProfileRevision, BuildConfigRef: iapiserver.AppStudioRefPrefixBuildConfig + created.ID, DependencyLockDigest: snapshot.ManifestDigest, AuthorizationRef: fmt.Sprintf(iapiserver.AppStudioRefPrefixBuildGrant+"%s/%s/%d", appID, created.ID, created.ResourceVersion), ExpectedResourceVersion: created.ResourceVersion}).AtomicTaskArguments()
	task, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AppStudioTaskDomain, &iapiserver.AtomicTaskCreateRequest{Key: "build-" + created.ID, Name: "AppStudio build", FunctionRef: iapiserver.AppStudioFunctionBuildExecute, Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace})
	if err != nil {
		created.Status = iapiserver.AppStudioBuildStatusFailed
		_, _ = s.store.UpdateStudioBuild(ctx, created)
		return nil, err
	}
	created.AtomicTaskID, created.Status = task.ID, iapiserver.AppStudioBuildStatusRunning
	return s.store.UpdateStudioBuild(ctx, created)
}
func (s *Service) GetBuild(ctx context.Context, id string) (*iapiserver.StudioBuild, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetStudioBuild(ctx, id, owner)
}
func (s *Service) CancelBuild(ctx context.Context, id string, req *iapiserver.StudioActionRequest) (*iapiserver.StudioBuild, error) {
	build, err := s.GetBuild(ctx, id)
	if err != nil {
		return nil, err
	}
	if build.Status == iapiserver.AppStudioBuildStatusSucceeded || build.Status == iapiserver.AppStudioBuildStatusFailed || build.Status == iapiserver.AppStudioBuildStatusCanceled {
		return build, nil
	}
	if build.AtomicTaskID == "" || s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioBuildFailed, "build task is unavailable")
	}
	if _, err := s.tasks.CancelAtomicTask(ctx, build.AtomicTaskID, &iapiserver.ActionReasonRequest{Reason: req.Reason}); err != nil {
		return nil, err
	}
	build.Status = iapiserver.AppStudioBuildStatusCanceled
	return s.store.UpdateStudioBuild(ctx, build)
}

func (s *Service) GetPreview(ctx context.Context, appID string) (*iapiserver.StudioPreviewRuntime, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	return s.store.GetStudioPreviewRuntime(ctx, workspace.ID, owner)
}
func (s *Service) RefreshPreview(ctx context.Context, appID string, req *iapiserver.StudioPreviewRequest) (*iapiserver.StudioPreviewRuntime, error) {
	owner, workspace, err := s.sourceWorkspace(ctx, appID)
	if err != nil {
		return nil, err
	}
	if s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioRuntimeDeployFailed, "task center is unavailable")
	}
	revision := req.SourceRevision
	if revision == 0 {
		revision = workspace.CurrentRevision
	}
	revisionRecord, err := s.store.GetStudioWorkspaceRevision(ctx, workspace.ID, revision, owner)
	if err != nil {
		return nil, err
	}
	runtime := &iapiserver.StudioPreviewRuntime{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, StudioApplicationID: appID, WorkspaceID: workspace.ID, WorkspaceRevision: revision, Status: iapiserver.AppStudioPreviewStatusPending, ExpiresAt: imachinery.Time{Time: time.Now().Add(24 * time.Hour)}}
	runtime, err = s.store.CreateStudioPreviewRuntime(ctx, owner, runtime)
	if err != nil {
		return nil, err
	}
	arguments := (iapiserver.AppStudioPreviewTaskArguments{StudioApplicationID: appID, PreviewRuntimeID: runtime.ID, WorkspaceID: workspace.ID, WorkspaceRevision: revision, WorkspaceRevisionSourceRef: iapiserver.AppStudioRefPrefixWorkspaceRevision + workspace.ID + "/" + fmt.Sprint(revision), RuntimeProfileID: iapiserver.AppStudioPreviewProfileStaticWeb, RuntimeProfileRevision: iapiserver.AppStudioRuntimeProfileRevision, EndpointVisibility: iapiserver.AppStudioEndpointVisibilityUser, AuthorizationRef: fmt.Sprintf(iapiserver.AppStudioRefPrefixPreviewGrant+"%s/%s/%d", workspace.ID, runtime.ID, runtime.ResourceVersion), ExpectedResourceVersion: runtime.ResourceVersion}).AtomicTaskArguments()
	_ = revisionRecord
	_, err = s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AppStudioTaskDomain, &iapiserver.AtomicTaskCreateRequest{Key: "preview-" + runtime.ID, Name: "AppStudio preview", FunctionRef: iapiserver.AppStudioFunctionPreviewEnsure, Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace})
	if err != nil {
		runtime.Status = iapiserver.AppStudioPreviewStatusFailed
		_, _ = s.store.UpdateStudioPreviewRuntime(ctx, runtime)
		return nil, err
	}
	return runtime, nil
}
func (s *Service) StopPreview(ctx context.Context, appID string, req *iapiserver.StudioActionRequest) (*iapiserver.StudioPreviewRuntime, error) {
	runtime, err := s.GetPreview(ctx, appID)
	if err != nil {
		return nil, err
	}
	if runtime == nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceNotVisible, "studio preview runtime not visible")
	}
	if runtime.Status == iapiserver.AppStudioPreviewStatusStopped || runtime.Status == iapiserver.AppStudioPreviewStatusExpired {
		return runtime, nil
	}
	if runtime.InfraRuntimeID == "" || s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioRuntimeDeployFailed, "preview infra runtime is unavailable")
	}
	arguments := (iapiserver.AppStudioStopTaskArguments{StudioApplicationID: runtime.StudioApplicationID, PreviewRuntimeID: runtime.ID, InfraRuntimeID: runtime.InfraRuntimeID, Action: iapiserver.AppStudioTaskActionStop, Reason: req.Reason, AuthorizationRef: fmt.Sprintf(iapiserver.AppStudioRefPrefixPreviewGrant+"%s/%s/%d", runtime.WorkspaceID, runtime.ID, runtime.ResourceVersion), ExpectedResourceVersion: runtime.ResourceVersion}).AtomicTaskArguments()
	_, err = s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AppStudioTaskDomain, &iapiserver.AtomicTaskCreateRequest{Key: "preview-stop-" + runtime.ID, Name: "AppStudio preview stop", FunctionRef: iapiserver.AppStudioFunctionPreviewStop, Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace})
	return runtime, err
}

func (s *Service) GetRuntimeConfig(ctx context.Context, versionID, environment string) (*iapiserver.StudioRuntimeConfig, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetStudioRuntimeConfig(ctx, versionID, environment, owner)
}
func (s *Service) ReplaceRuntimeConfig(ctx context.Context, versionID, environment string, req *iapiserver.StudioRuntimeConfigRequest) (*iapiserver.StudioRuntimeConfig, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	config := &iapiserver.StudioRuntimeConfig{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: environment}, StudioApplicationVersionID: versionID, Environment: environment, PublicConfig: req.PublicConfig, SecretReferences: req.SecretReferences, IntegrationReferences: req.IntegrationReferences, ValidationStatus: iapiserver.AppStudioRuntimeConfigStatusValid}
	return s.store.ReplaceStudioRuntimeConfig(ctx, owner, config, req.ResourceVersion)
}

func (s *Service) ListReleases(ctx context.Context, appID string, req *iapiserver.StudioReleaseListRequest) (*iapiserver.StudioReleaseListResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListStudioReleases(ctx, appID, owner, req)
	return &iapiserver.StudioReleaseListResponse{Total: total, Items: items}, err
}
func (s *Service) CreateRelease(ctx context.Context, appID string, req *iapiserver.StudioReleaseRequest) (*iapiserver.StudioRelease, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	build, err := s.store.GetStudioBuild(ctx, req.StudioBuildID, owner)
	if err != nil || build.Status != iapiserver.AppStudioBuildStatusSucceeded || build.StudioApplicationID != appID {
		return nil, errors.NewStatus(code.ErrAppStudioReleaseInvalid, "studio build is not releasable")
	}
	version, err := s.store.GetStudioApplicationVersion(ctx, req.StudioApplicationVersionID, owner)
	if err != nil || version.StudioApplicationID != appID || version.SourceSnapshotID != build.SourceSnapshotID {
		return nil, errors.NewStatus(code.ErrAppStudioReleaseInvalid, "studio application version does not match build")
	}
	config, err := s.store.GetStudioRuntimeConfig(ctx, version.ID, req.Environment, owner)
	if err != nil || config == nil || config.ID != req.RuntimeConfigID || config.ValidationStatus != iapiserver.AppStudioRuntimeConfigStatusValid {
		return nil, errors.NewStatus(code.ErrAppStudioReleaseInvalid, "runtime config is invalid")
	}
	if s.artifacts != nil {
		artifact, readErr := s.artifacts.GetArtifact(ctx, owner, build.ArtifactID)
		if readErr != nil || artifact == nil || strings.ToLower(artifact.ProcessingStatus) != iapiserver.AppStudioArtifactProcessingReady || artifact.BlobID == "" {
			return nil, errors.NewStatus(code.ErrAppStudioBuildArtifactNotReady, "build artifact is not ready")
		}
	}
	return s.createReleaseTask(ctx, owner, appID, build, version, config, req.Environment, req.IdempotencyKey, "")
}
func (s *Service) GetRelease(ctx context.Context, id string) (*iapiserver.StudioRelease, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetStudioRelease(ctx, id, owner)
}
func (s *Service) RollbackRelease(ctx context.Context, id string, req *iapiserver.StudioActionRequest) (*iapiserver.StudioRelease, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.store.GetStudioRelease(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	build, err := s.store.GetStudioBuild(ctx, target.StudioBuildID, owner)
	if err != nil {
		return nil, err
	}
	version, err := s.store.GetStudioApplicationVersion(ctx, target.StudioApplicationVersionID, owner)
	if err != nil {
		return nil, err
	}
	config, err := s.store.GetStudioRuntimeConfig(ctx, version.ID, target.Environment, owner)
	if err != nil || config == nil {
		return nil, errors.NewStatus(code.ErrAppStudioReleaseInvalid, "rollback runtime config is unavailable")
	}
	key := req.RequestID
	if key == "" {
		key = uuid.NewString()
	}
	return s.createReleaseTask(ctx, owner, target.StudioApplicationID, build, version, config, target.Environment, key, target.ID)
}
func (s *Service) createReleaseTask(ctx context.Context, owner, appID string, build *iapiserver.StudioBuild, version *iapiserver.StudioApplicationVersion, config *iapiserver.StudioRuntimeConfig, environment, idempotency, rollbackID string) (*iapiserver.StudioRelease, error) {
	if s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioRuntimeDeployFailed, "task center is unavailable")
	}
	releaseID, runtimeID := uuid.NewString(), uuid.NewString()
	release := &iapiserver.StudioRelease{ObjectMeta: imachinery.ObjectMeta{ID: releaseID}, OwnerUserID: owner, StudioApplicationID: appID, StudioApplicationVersionID: version.ID, StudioBuildID: build.ID, RuntimeConfigID: config.ID, ArtifactID: build.ArtifactID, ArtifactDigest: build.ArtifactDigest, Environment: environment, Status: iapiserver.AppStudioReleaseStatusPending, RollbackOfReleaseID: rollbackID, IdempotencyKey: idempotency}
	runtime := &iapiserver.StudioRuntimeInstance{ObjectMeta: imachinery.ObjectMeta{ID: runtimeID}, StudioApplicationID: appID, StudioReleaseID: releaseID, Environment: environment, Status: iapiserver.AppStudioRuntimeStatusCreating, HealthStatus: iapiserver.AppStudioRuntimeHealthUnknown}
	release, err := s.store.CreateStudioReleaseAggregate(ctx, owner, release, runtime)
	if err != nil {
		return nil, err
	}
	arguments := (iapiserver.AppStudioProductionTaskArguments{StudioApplicationID: appID, StudioReleaseID: release.ID, StudioRuntimeInstanceID: runtime.ID, StudioApplicationVersionID: version.ID, RuntimeConfigID: config.ID, ArtifactID: build.ArtifactID, ArtifactDigest: build.ArtifactDigest, ArtifactSourceRef: iapiserver.AppStudioRefPrefixArtifact + build.ArtifactID + "@" + build.ArtifactDigest, Environment: environment, DeploymentReason: deploymentReason(rollbackID), RuntimeProfileID: iapiserver.AppStudioProductionProfileStaticWeb, RuntimeProfileRevision: iapiserver.AppStudioRuntimeProfileRevision, HealthCheckRef: iapiserver.AppStudioRefPrefixHealthCheck + release.ID, EndpointVisibility: iapiserver.AppStudioEndpointVisibilityUser, AuthorizationRef: fmt.Sprintf(iapiserver.AppStudioRefPrefixProductionGrant+"%s/%s/%d", release.ID, runtime.ID, runtime.ResourceVersion), ExpectedResourceVersion: runtime.ResourceVersion}).AtomicTaskArguments()
	task, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AppStudioTaskDomain, &iapiserver.AtomicTaskCreateRequest{Key: "production-" + runtime.ID, Name: "AppStudio production reconcile", FunctionRef: iapiserver.AppStudioFunctionProductionEnsure, Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace})
	if err != nil {
		release.Status = iapiserver.AppStudioReleaseStatusFailed
		_, _ = s.store.UpdateStudioRelease(ctx, release)
		return nil, err
	}
	runtime.AtomicTaskID = task.ID
	_, err = s.store.UpdateStudioRuntimeInstance(ctx, runtime)
	if err != nil {
		return nil, err
	}
	release.Status = iapiserver.AppStudioReleaseStatusDeploying
	return s.store.UpdateStudioRelease(ctx, release)
}
func (s *Service) ListRuntimeInstances(ctx context.Context, appID string, req *iapiserver.StudioRuntimeInstanceListRequest) (*iapiserver.StudioRuntimeInstanceListResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListStudioRuntimeInstances(ctx, appID, owner, req)
	return &iapiserver.StudioRuntimeInstanceListResponse{Total: total, Items: items}, err
}
func (s *Service) GetRuntimeInstance(ctx context.Context, id string) (*iapiserver.StudioRuntimeInstance, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetStudioRuntimeInstance(ctx, id, owner)
}
func (s *Service) StopRuntimeInstance(ctx context.Context, id string, req *iapiserver.StudioActionRequest) (*iapiserver.StudioRuntimeInstance, error) {
	runtime, err := s.GetRuntimeInstance(ctx, id)
	if err != nil {
		return nil, err
	}
	if runtime.Status == iapiserver.AppStudioRuntimeStatusStopped {
		return runtime, nil
	}
	if runtime.InfraRuntimeID == "" || s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioRuntimeDeployFailed, "studio infra runtime is unavailable")
	}
	arguments := (iapiserver.AppStudioStopTaskArguments{StudioApplicationID: runtime.StudioApplicationID, StudioReleaseID: runtime.StudioReleaseID, StudioRuntimeInstanceID: runtime.ID, InfraRuntimeID: runtime.InfraRuntimeID, Action: iapiserver.AppStudioTaskActionStop, Reason: req.Reason, AuthorizationRef: fmt.Sprintf(iapiserver.AppStudioRefPrefixProductionGrant+"%s/%s/%d", runtime.StudioReleaseID, runtime.ID, runtime.ResourceVersion), ExpectedResourceVersion: runtime.ResourceVersion}).AtomicTaskArguments()
	_, err = s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AppStudioTaskDomain, &iapiserver.AtomicTaskCreateRequest{Key: "production-stop-" + runtime.ID, Name: "AppStudio production stop", FunctionRef: iapiserver.AppStudioFunctionProductionStop, Arguments: arguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace})
	return runtime, err
}
func (s *Service) BuildLogs(ctx context.Context, id string) (*iapiserver.StudioRuntimeLogListResponse, error) {
	if _, err := s.GetBuild(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.StudioRuntimeLogListResponse{Items: []*iapiserver.StudioRuntimeLogEntry{}}, nil
}
func (s *Service) RuntimeLogs(ctx context.Context, id string) (*iapiserver.StudioRuntimeLogListResponse, error) {
	if _, err := s.GetRuntimeInstance(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.StudioRuntimeLogListResponse{Items: []*iapiserver.StudioRuntimeLogEntry{}}, nil
}

// ValidateAgentWorkspaceBinding 实现 Agent 的固定 Coding Workspace 消费方合同。
func (s *Service) ValidateAgentWorkspaceBinding(ctx context.Context, owner, workspaceID string) (*iapiserver.AgentAuthorizationSummary, error) {
	workspace, err := s.store.GetStudioWorkspace(ctx, workspaceID, owner)
	if err != nil {
		return nil, err
	}
	if workspace.Status != iapiserver.AppStudioWorkspaceStatusReady {
		return nil, errors.NewStatus(code.ErrAppStudioSourceNotVisible, "studio source is not ready")
	}
	return &iapiserver.AgentAuthorizationSummary{Source: iapiserver.AppStudioTaskDomain, ValidatedAt: imachinery.Now()}, nil
}

func (s *Service) sourceWorkspace(ctx context.Context, appID string) (string, *iapiserver.StudioWorkspace, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return "", nil, err
	}
	workspace, err := s.store.GetStudioWorkspaceByApplication(ctx, appID, owner)
	if err != nil {
		return "", nil, err
	}
	return owner, workspace, nil
}

func studioUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "", errors.NewStatus(code.ErrAppStudioAccessDenied, "authenticated user is required")
	}
	return user.ID, nil
}
func (s *Service) loadRevision(ctx context.Context, workspaceID string, revision int64, owner string) (map[string][]byte, error) {
	rows, err := s.store.ListStudioSourceFiles(ctx, workspaceID, revision, "", owner)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(rows))
	for _, row := range rows {
		content, readErr := s.sources.ReadFile(ctx, workspaceID, revision, row.Path)
		if readErr != nil {
			return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "source content is unavailable")
		}
		result[row.Path] = content
	}
	return result, nil
}

func applyOperations(files map[string][]byte, operations []iapiserver.StudioChangeOperation) error {
	for _, op := range operations {
		path, err := cleanSourcePath(op.Path)
		if err != nil {
			return err
		}
		switch op.Operation {
		case iapiserver.AppStudioChangeOperationCreate:
			if _, exists := files[path]; exists {
				return fmt.Errorf("source file already exists")
			}
			if op.Content == nil {
				return fmt.Errorf("source content is required")
			}
			if len(*op.Content) > maxStudioFileBytes {
				return fmt.Errorf("source file exceeds size limit")
			}
			files[path] = []byte(*op.Content)
		case iapiserver.AppStudioChangeOperationUpdate:
			if _, exists := files[path]; !exists {
				return fmt.Errorf("source file does not exist")
			}
			if op.Content == nil {
				return fmt.Errorf("source content is required")
			}
			if len(*op.Content) > maxStudioFileBytes {
				return fmt.Errorf("source file exceeds size limit")
			}
			files[path] = []byte(*op.Content)
		case iapiserver.AppStudioChangeOperationDelete:
			if _, exists := files[path]; !exists {
				return fmt.Errorf("source file does not exist")
			}
			delete(files, path)
		case iapiserver.AppStudioChangeOperationMove:
			if _, exists := files[path]; !exists {
				return fmt.Errorf("source file does not exist")
			}
			if op.TargetPath == nil {
				return fmt.Errorf("move target is required")
			}
			target, err := cleanSourcePath(*op.TargetPath)
			if err != nil {
				return err
			}
			if _, exists := files[target]; exists {
				return fmt.Errorf("move target already exists")
			}
			files[target] = files[path]
			delete(files, path)
		default:
			return fmt.Errorf("unsupported source operation")
		}
	}
	return nil
}
func revisionRows(workspaceID string, revision int64, files map[string][]byte) (string, []*iapiserver.StudioSourceFile) {
	paths := sortedPaths(files)
	manifest := sha256.New()
	rows := make([]*iapiserver.StudioSourceFile, 0, len(paths))
	for _, path := range paths {
		sum := sha256.Sum256(files[path])
		digest := "sha256:" + hex.EncodeToString(sum[:])
		_, _ = fmt.Fprintf(manifest, "%s\x00%s\x00%d\n", path, digest, len(files[path]))
		rows = append(rows, &iapiserver.StudioSourceFile{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, WorkspaceID: workspaceID, Revision: revision, Path: path, ContentDigest: digest, SizeBytes: int64(len(files[path]))})
	}
	return "sha256:" + hex.EncodeToString(manifest.Sum(nil)), rows
}
func emptyTreeDigest() string {
	sum := sha256.Sum256(nil)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func stableStudioInitializationID(owner, idempotencyKey, resource string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("appstudio:"+owner+":"+idempotencyKey+":"+resource)).String()
}

func studioApplicationCreateResponse(initialization *store.StudioApplicationInitialization) *iapiserver.StudioApplicationCreateResponse {
	app, agent, invocation := initialization.Application, initialization.Agent, initialization.InitialInvocation
	return &iapiserver.StudioApplicationCreateResponse{
		Application: app,
		CodingAgent: &iapiserver.StudioAgentStatus{
			StudioApplicationID: app.ID,
			AgentID:             app.CodingAgentID,
			SessionID:           app.CodingSessionID,
			Generation:          app.CodingAgentGeneration,
			Status:              agent.Status,
		},
		InitialInvocation: &iapiserver.StudioAgentInvocation{
			ID:                   invocation.ID,
			AgentID:              invocation.AgentID,
			SessionID:            invocation.SessionID,
			Generation:           app.CodingAgentGeneration,
			Type:                 invocation.Type,
			Status:               invocation.Status,
			AtomicTaskID:         invocation.AtomicTaskID,
			RuntimeBindingID:     invocation.RuntimeBindingID,
			RuntimeSessionRef:    invocation.RuntimeSessionRef,
			RuntimeInvocationRef: invocation.RuntimeInvocationRef,
			LastEventSequence:    invocation.LastEventSequence,
			FailureCode:          invocation.FailureCode,
			FailureMessage:       invocation.FailureMessage,
			CompletedAt:          invocation.CompletedAt,
			CreatedAt:            invocation.CreatedAt,
			UpdatedAt:            invocation.UpdatedAt,
		},
	}
}

func (s *Service) currentCodingAgent(ctx context.Context, appID string) (*iapiserver.StudioApplication, *iapiserver.Agent, *iapiserver.AgentSession, error) {
	if s.agents == nil {
		return nil, nil, nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent facade is unavailable")
	}
	app, err := s.GetApplication(ctx, appID)
	if err != nil {
		return nil, nil, nil, err
	}
	if app.CodingAgentID == "" || app.CodingSessionID == "" || app.CodingAgentGeneration < 1 || app.DefaultWorkspaceID == "" {
		return nil, nil, nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent binding is incomplete")
	}
	agent, session, err := s.agents.GetCodingAgentForStudio(ctx, app.CodingAgentID, app.CodingSessionID, app.DefaultWorkspaceID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.agents.EnsurePlatformMCPBindingForCodingAgent(ctx, agent.ID, app.ID); err != nil {
		return nil, nil, nil, err
	}
	return app, agent, session, nil
}

func studioPlatformMCPBinding(agent *iapiserver.Agent, applicationID string) *iapiserver.AgentMCPBinding {
	return &iapiserver.AgentMCPBinding{
		ObjectMeta: imachinery.ObjectMeta{ID: stableStudioInitializationID(agent.OwnerUserID, agent.ID, "platform-mcp"), Name: "omnimam-platform"},
		AgentID:    agent.ID, ServerType: iapiserver.AgentMCPServerTypePlatform,
		EndpointRef: iapiserver.AgentMCPPlatformEndpointRefDefault, Enabled: true,
		AllowedTools: []string{"omnimam.capabilities.list", "omnimam.capabilities.get", "omnimam.applications.list", "omnimam.applications.get", "omnimam.applications.run", "omnimam.application_runs.get", "omnimam.assets.search", "omnimam.assets.get"},
	}
}

func studioAgentStatus(app *iapiserver.StudioApplication, agent *iapiserver.Agent) *iapiserver.StudioAgentStatus {
	return &iapiserver.StudioAgentStatus{
		StudioApplicationID: app.ID,
		AgentID:             app.CodingAgentID,
		SessionID:           app.CodingSessionID,
		Generation:          app.CodingAgentGeneration,
		Status:              agent.Status,
	}
}

func validateStudioInvocationBinding(app *iapiserver.StudioApplication, invocation *iapiserver.AgentInvocation) error {
	if invocation == nil || invocation.AgentID != app.CodingAgentID || invocation.SessionID != app.CodingSessionID || invocation.Type != iapiserver.AgentInvocationTypeCoding {
		return errors.NewStatus(code.ErrAgentSessionNotVisible, "coding invocation not visible")
	}
	return nil
}

func (s *Service) projectStudioAgentInvocation(ctx context.Context, app *iapiserver.StudioApplication, invocation *iapiserver.AgentInvocation) (*iapiserver.StudioAgentInvocation, error) {
	items, err := s.projectStudioAgentInvocations(ctx, app, []*iapiserver.AgentInvocation{invocation})
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

func (s *Service) projectStudioAgentInvocations(ctx context.Context, app *iapiserver.StudioApplication, invocations []*iapiserver.AgentInvocation) ([]*iapiserver.StudioAgentInvocation, error) {
	invocationIDs := make([]string, 0, len(invocations))
	for _, invocation := range invocations {
		invocationIDs = append(invocationIDs, invocation.ID)
	}
	changeSets, err := s.store.ResolveStudioInvocationChangeSets(ctx, app.ID, app.OwnerUserID, invocationIDs)
	if err != nil {
		return nil, err
	}
	items := make([]*iapiserver.StudioAgentInvocation, 0, len(invocations))
	for _, invocation := range invocations {
		items = append(items, studioAgentInvocationProjection(app, invocation, changeSets[invocation.ID]))
	}
	return items, nil
}

func studioAgentInvocationProjection(app *iapiserver.StudioApplication, invocation *iapiserver.AgentInvocation, changeSet *iapiserver.StudioChangeSet) *iapiserver.StudioAgentInvocation {
	projection := &iapiserver.StudioAgentInvocation{
		ID:                   invocation.ID,
		AgentID:              invocation.AgentID,
		SessionID:            invocation.SessionID,
		Generation:           app.CodingAgentGeneration,
		Type:                 invocation.Type,
		Status:               invocation.Status,
		AtomicTaskID:         invocation.AtomicTaskID,
		RuntimeBindingID:     invocation.RuntimeBindingID,
		RuntimeSessionRef:    invocation.RuntimeSessionRef,
		RuntimeInvocationRef: invocation.RuntimeInvocationRef,
		LastEventSequence:    invocation.LastEventSequence,
		FailureCode:          invocation.FailureCode,
		FailureMessage:       invocation.FailureMessage,
		CompletedAt:          invocation.CompletedAt,
		CreatedAt:            invocation.CreatedAt,
		UpdatedAt:            invocation.UpdatedAt,
	}
	if changeSet != nil && changeSet.TargetRevision != nil {
		changeSetID := changeSet.ID
		resultingRevision := *changeSet.TargetRevision
		projection.ResultingChangeSetID = &changeSetID
		projection.ResultingSourceRevision = &resultingRevision
	}
	return projection
}

func sortedPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
func diffOperations(current, target map[string][]byte) []iapiserver.StudioChangeOperation {
	operations := make([]iapiserver.StudioChangeOperation, 0)
	for _, path := range sortedPaths(current) {
		if _, ok := target[path]; !ok {
			operations = append(operations, iapiserver.StudioChangeOperation{Operation: iapiserver.AppStudioChangeOperationDelete, Path: path})
		}
	}
	for _, path := range sortedPaths(target) {
		content := string(target[path])
		currentContent, ok := current[path]
		if !ok {
			operations = append(operations, iapiserver.StudioChangeOperation{Operation: iapiserver.AppStudioChangeOperationCreate, Path: path, Content: &content})
		} else if string(currentContent) != content {
			operations = append(operations, iapiserver.StudioChangeOperation{Operation: iapiserver.AppStudioChangeOperationUpdate, Path: path, Content: &content})
		}
	}
	if len(operations) == 0 {
		content := ""
		operations = append(operations, iapiserver.StudioChangeOperation{Operation: iapiserver.AppStudioChangeOperationCreate, Path: ".restore-marker", Content: &content})
	}
	return operations
}
func truncateText(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func buildProfile(*iapiserver.StudioSourceSnapshot) string {
	return iapiserver.AppStudioBuildProfileStaticWeb
}
func deploymentReason(rollback string) string {
	if rollback != "" {
		return iapiserver.AppStudioDeploymentReasonRollback
	}
	return iapiserver.AppStudioDeploymentReasonRelease
}

var _ = json.Valid
