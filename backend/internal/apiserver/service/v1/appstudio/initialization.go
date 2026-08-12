package appstudio

import (
	"context"
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const initializationTaskAttempts = 5

func studioInitializationDAG(id, applicationID, owner, createIdempotencyKey, dagIdempotencyKey string) *iapiserver.DAGTaskGroupCreateRequest {
	arguments := map[string]any{
		"studio_application_id":  applicationID,
		"owner_user_id":          owner,
		"create_idempotency_key": createIdempotencyKey,
	}
	retry := iapiserver.RetryPolicy{
		MaxAttempts: initializationTaskAttempts, RetryDelaySeconds: 2,
		BackoffType: iapiserver.TaskWorkerRetryBackoffExponential, MaxRetryDelaySeconds: 30,
	}
	node := func(key, name, functionRef string) iapiserver.DAGNode {
		return iapiserver.DAGNode{Key: key, Task: iapiserver.AtomicTaskTemplate{
			Key: key, Name: name, FunctionRef: functionRef, Arguments: arguments, RetryPolicy: retry,
		}}
	}
	return &iapiserver.DAGTaskGroupCreateRequest{
		ID: id, Name: "Initialize AppStudio application",
		Nodes: []iapiserver.DAGNode{
			node("project-ensure", "Ensure GitLab project", iapiserver.AppStudioFunctionInitializationProjectEnsure),
			node("webhook-ensure", "Ensure GitLab webhook", iapiserver.AppStudioFunctionInitializationWebhookEnsure),
			node("finalize", "Finalize AppStudio application", iapiserver.AppStudioFunctionInitializationFinalize),
			node("invocation-start", "Start initial coding invocation", iapiserver.AppStudioFunctionInitializationInvocationStart),
		},
		Edges: []iapiserver.DAGEdge{
			{FromNode: "project-ensure", ToNode: "webhook-ensure"},
			{FromNode: "webhook-ensure", ToNode: "finalize"},
			{FromNode: "finalize", ToNode: "invocation-start"},
		},
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
		IdempotencyScope: "appstudio-initialization", IdempotencyKey: dagIdempotencyKey,
		CreatedBy: owner, TriggerType: iapiserver.DAGTriggerDomainEvent,
		TriggerSourceID: applicationID, TriggerSourceName: "AppStudio application creation",
	}
}

var initializationStageByNode = map[string]string{
	"project-ensure":   iapiserver.AppStudioInitializationStageGitLabProject,
	"webhook-ensure":   iapiserver.AppStudioInitializationStageGitLabWebhook,
	"finalize":         iapiserver.AppStudioInitializationStageApplication,
	"invocation-start": iapiserver.AppStudioInitializationStageFirstInvocation,
}

// GetInitialization 返回当前初始化 DAG 的固定四阶段安全投影。
func (s *Service) GetInitialization(ctx context.Context, appID string) (*iapiserver.StudioApplicationInitialization, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	app, err := s.store.GetStudioApplication(ctx, appID, owner)
	if err != nil {
		return nil, err
	}
	if s.tasks == nil || app.InitializationDAGTaskGroupID == "" {
		return nil, errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "studio application initialization is unavailable")
	}
	detail, err := s.tasks.GetDAGTaskGroupDetail(ctx, app.InitializationDAGTaskGroupID)
	if err != nil {
		return nil, err
	}
	return projectInitialization(app, detail), nil
}

// RetryInitialization 为 ERROR Application 原子切换并创建一轮幂等初始化 DAG。
func (s *Service) RetryInitialization(ctx context.Context, appID string, req *iapiserver.StudioApplicationInitializationRetryRequest) (*iapiserver.StudioApplicationInitialization, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	app, err := s.store.GetStudioApplication(ctx, appID, owner)
	if err != nil {
		return nil, err
	}
	dagID := stableStudioInitializationID(appID, req.IdempotencyKey, "initialization-retry-dag")
	if app.InitializationDAGTaskGroupID == dagID {
		return s.GetInitialization(ctx, appID)
	}
	if s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "task center is unavailable")
	}
	app, err = s.store.BeginStudioApplicationInitializationRetry(ctx, appID, owner, dagID)
	if err != nil {
		return nil, err
	}
	dag, err := s.tasks.CreateDomainDAGTaskGroup(ctx, iapiserver.AppStudioTaskDomain, studioInitializationDAG(dagID, app.ID, owner, app.CreateIdempotencyKey, req.IdempotencyKey))
	if err != nil {
		_ = s.store.RollbackStudioApplicationInitializationRetry(ctx, app.ID, owner, dagID)
		return nil, err
	}
	detail, err := s.tasks.GetDAGTaskGroupDetail(ctx, dag.ID)
	if err != nil {
		return nil, err
	}
	return projectInitialization(app, detail), nil
}

func projectInitialization(app *iapiserver.StudioApplication, detail *iapiserver.DAGTaskGroupDetail) *iapiserver.StudioApplicationInitialization {
	result := &iapiserver.StudioApplicationInitialization{
		StudioApplicationID: app.ID, DAGTaskGroupID: detail.ID, Status: initializationStatus(detail.Status),
		Progress: detail.Progress, UpdatedAt: detail.UpdatedAt,
	}
	nodes := make(map[string]*iapiserver.DAGNodeExecutionSummary, len(detail.ExecutionNodes))
	for _, node := range detail.ExecutionNodes {
		if node != nil {
			nodes[node.NodeKey] = node
		}
	}
	for _, key := range []string{"project-ensure", "webhook-ensure", "finalize", "invocation-start"} {
		node := nodes[key]
		stage := &iapiserver.StudioApplicationInitializationStage{Stage: initializationStageByNode[key], Status: iapiserver.AppStudioInitializationStatusPending, UpdatedAt: detail.UpdatedAt}
		if node != nil {
			stage.Status, stage.AttemptCount = initializationStatus(node.Status), node.AttemptCount
			if node.CompletedAt != nil {
				stage.UpdatedAt = *node.CompletedAt
			} else if node.StartedAt != nil {
				stage.UpdatedAt = *node.StartedAt
			}
			if stage.Status == iapiserver.AppStudioInitializationStatusError {
				stage.FailedAt = node.CompletedAt
				stage.LatestError = safeInitializationError(node.LatestError, stage.UpdatedAt)
			}
		}
		result.Stages = append(result.Stages, stage)
	}
	return result
}

func initializationStatus(status string) string {
	switch status {
	case iapiserver.AtomicTaskStatusSuccess:
		return iapiserver.AppStudioInitializationStatusSuccess
	case iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusCanceled, iapiserver.AtomicTaskStatusTimeout:
		return iapiserver.AppStudioInitializationStatusError
	case iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusRetrying, iapiserver.AtomicTaskStatusCancelRequested:
		return iapiserver.AppStudioInitializationStatusRunning
	default:
		return iapiserver.AppStudioInitializationStatusPending
	}
}

func safeInitializationError(taskError *iapiserver.TaskError, occurredAt imachinery.Time) *iapiserver.StudioApplicationInitializationSafeError {
	if taskError != nil && !taskError.OccurredAt.IsZero() {
		occurredAt = taskError.OccurredAt
	}
	message := "AppStudio initialization failed."
	codeName := "ERR_APPSTUDIO_APPLICATION_INVALID_STATE"
	cn := "AppStudio 初始化失败，请检查配置后重试。"
	if taskError != nil {
		raw := taskError.Code + " " + taskError.Message
		switch {
		case strings.Contains(raw, "ERR_GITLAB_APPSTUDIO_DEFAULT_SERVER_UNAVAILABLE"), strings.Contains(raw, "No READY GitLabServer is configured as the AppStudio default"):
			codeName, cn, message = "ERR_GITLAB_APPSTUDIO_DEFAULT_SERVER_UNAVAILABLE", "没有 READY 且标记为 AppStudio 默认的 GitLabServer，请先完成代码仓库配置和连接检测。", "No READY GitLabServer is configured as the AppStudio default. Configure and test a repository connection first."
		case strings.Contains(raw, "GitLabServer connectivity"):
			codeName, cn, message = "ERR_GITLAB_SERVER_CONNECTION_FAILED", "GitLabServer 连接、credential 或 Namespace 检测失败。", "GitLabServer connectivity, credential, or Namespace validation failed."
		case strings.Contains(raw, "remote GitLab Project"):
			codeName, cn, message = "ERR_GITLAB_PROJECT_REMOTE_FAILED", "GitLab 远端 Project 操作失败。", "The remote GitLab Project operation failed."
		case strings.Contains(raw, "local GitLabProject projection"):
			codeName, cn, message = "ERR_GITLAB_PROJECT_PROJECTION_FAILED", "GitLabProject 本地投影写入失败。", "The local GitLabProject projection could not be persisted."
		}
	}
	return &iapiserver.StudioApplicationInitializationSafeError{Code: codeName, Message: message, Messages: iapiserver.StudioApplicationInitializationMessages{ZhCN: cn, EnUS: message}, OccurredAt: occurredAt}
}

func (s *Service) EnsureInitializationProject(ctx context.Context, arguments iapiserver.AppStudioInitializationTaskArguments) (map[string]any, error) {
	initialization, err := s.initialization(ctx, arguments)
	if err != nil {
		return nil, err
	}
	if s.projectInitializer == nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio source provider is unavailable")
	}
	blueprint, err := LoadBlueprint(initialization.Application.BlueprintID, initialization.Application.BlueprintVersion)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "application blueprint is unavailable")
	}
	files := make(map[string][]byte, len(blueprint.Files))
	for path, content := range blueprint.Files {
		files[path] = append([]byte(nil), content...)
	}
	projectID := stableStudioInitializationID(arguments.OwnerUserID, arguments.CreateIdempotencyKey, "gitlab-project")
	project, err := s.projectInitializer.EnsureProject(ctx, projectID, initialization.Application.Name,
		deterministicGitLabProjectPath(arguments.OwnerUserID, arguments.CreateIdempotencyKey, initialization.Application.Name),
		initialization.Application.Description, files)
	if err != nil {
		return nil, err
	}
	if project == nil || project.GitLabProjectID == "" || project.CommitSHA == "" {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio gitlab project initialization failed")
	}
	return map[string]any{"gitlab_project_id": project.GitLabProjectID, "commit_sha": project.CommitSHA}, nil
}

func (s *Service) EnsureInitializationWebhook(ctx context.Context, arguments iapiserver.AppStudioInitializationTaskArguments) (map[string]any, error) {
	if _, err := s.initialization(ctx, arguments); err != nil {
		return nil, err
	}
	if s.webhooks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio webhook provider is unavailable")
	}
	projectID := stableStudioInitializationID(arguments.OwnerUserID, arguments.CreateIdempotencyKey, "gitlab-project")
	hookID, err := s.webhooks.EnsureAppStudioWebhook(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if hookID <= 0 {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio gitlab webhook initialization failed")
	}
	return map[string]any{"gitlab_project_id": projectID, "webhook_id": hookID}, nil
}

func (s *Service) FinalizeInitialization(ctx context.Context, arguments iapiserver.AppStudioInitializationTaskArguments) (map[string]any, error) {
	initialization, err := s.initialization(ctx, arguments)
	if err != nil {
		return nil, err
	}
	if initialization.Application.Status == iapiserver.AppStudioApplicationStatusReady {
		return map[string]any{"studio_application_id": initialization.Application.ID}, nil
	}
	workspace, err := s.store.GetStudioWorkspaceByApplication(ctx, initialization.Application.ID, arguments.OwnerUserID)
	if err != nil {
		return nil, err
	}
	repository, err := s.store.GetStudioSourceRepository(ctx, workspace.ID, arguments.OwnerUserID)
	if err != nil {
		return nil, err
	}
	projectID := stableStudioInitializationID(arguments.OwnerUserID, arguments.CreateIdempotencyKey, "gitlab-project")
	commitSHA, err := s.sourceProvider.BranchHead(ctx, projectID, "main")
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit is unavailable: "+err.Error())
	}
	if commitSHA == "" {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit is unavailable: branch head is empty")
	}
	existingRevision, existingErr := s.store.GetStudioWorkspaceRevisionByCommit(ctx, workspace.ID, commitSHA, arguments.OwnerUserID)
	if existingErr == nil {
		if !matchesInitializationRevision(existingRevision, workspace, commitSHA) {
			return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit conflicts with initialized workspace")
		}
		return map[string]any{"studio_application_id": initialization.Application.ID, "commit_sha": commitSHA}, nil
	}
	if errors.ToStatus(existingErr).Code != code.ErrAppStudioSourceRevisionConflict {
		return nil, existingErr
	}
	committed, err := s.sourceProvider.ListFiles(ctx, projectID, commitSHA, "")
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit is unavailable: "+err.Error())
	}
	files, err := sourceFileContents(committed)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit is invalid")
	}
	digest, rows := revisionRows(workspace.ID, 0, files)
	if digest != workspace.CurrentRevisionDigest {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit conflicts with blueprint")
	}
	revision := &iapiserver.StudioWorkspaceRevision{
		ObjectMeta:  imachinery.ObjectMeta{ID: stableStudioInitializationID(arguments.OwnerUserID, arguments.CreateIdempotencyKey, "revision-0")},
		WorkspaceID: workspace.ID, Revision: 0, CommitSHA: commitSHA, ContentDigest: digest, CreatedBy: arguments.OwnerUserID,
	}
	repository.GitLabProjectID, repository.Status = projectID, iapiserver.AppStudioRepositoryStatusReady
	workspace.Status, workspace.CurrentRevisionDigest = iapiserver.AppStudioWorkspaceStatusReady, digest
	initialization.Repository, initialization.Workspace = repository, workspace
	initialization.Revision, initialization.SourceFiles = revision, rows
	if _, err := s.store.CreateStudioApplicationInitialization(ctx, initialization); err != nil {
		return nil, err
	}
	return map[string]any{"studio_application_id": initialization.Application.ID, "commit_sha": commitSHA}, nil
}

func matchesInitializationRevision(revision *iapiserver.StudioWorkspaceRevision, workspace *iapiserver.StudioWorkspace, commitSHA string) bool {
	return revision != nil && workspace != nil && revision.Revision == 0 && revision.CommitSHA == commitSHA && revision.ContentDigest == workspace.CurrentRevisionDigest
}

func (s *Service) StartInitializationInvocation(ctx context.Context, arguments iapiserver.AppStudioInitializationTaskArguments) (map[string]any, error) {
	initialization, err := s.initialization(ctx, arguments)
	if err != nil {
		return nil, err
	}
	if initialization.Application.Status == iapiserver.AppStudioApplicationStatusReady {
		return map[string]any{"studio_application_id": initialization.Application.ID}, nil
	}
	if s.agents == nil || initialization.Agent == nil || initialization.InitialInvocation == nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent initialization is incomplete")
	}
	if err := s.agents.EnsurePlatformMCPBindingForCodingAgent(ctx, initialization.Agent.ID, initialization.Application.ID); err != nil {
		return nil, err
	}
	invocation, err := s.agents.StartCodingInvocation(ctx, initialization.Agent.ID, initialization.InitialInvocation.ID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}
	return map[string]any{"studio_application_id": initialization.Application.ID, "initial_invocation_id": invocation.ID}, nil
}

func (s *Service) initialization(ctx context.Context, arguments iapiserver.AppStudioInitializationTaskArguments) (*store.StudioApplicationInitialization, error) {
	if arguments.StudioApplicationID == "" || arguments.OwnerUserID == "" || arguments.CreateIdempotencyKey == "" {
		return nil, fmt.Errorf("appstudio initialization task arguments are incomplete")
	}
	initialization, err := s.store.GetStudioApplicationInitialization(ctx, arguments.OwnerUserID, arguments.CreateIdempotencyKey)
	if err != nil {
		return nil, err
	}
	if initialization.Application == nil || initialization.Application.ID != arguments.StudioApplicationID || initialization.Application.OwnerUserID != arguments.OwnerUserID {
		return nil, errors.NewStatus(code.ErrAppStudioAccessDenied, "appstudio initialization scope is invalid")
	}
	return initialization, nil
}
