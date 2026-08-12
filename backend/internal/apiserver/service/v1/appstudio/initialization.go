package appstudio

import (
	"context"
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const initializationTaskAttempts = 5

func studioInitializationDAG(id, applicationID, owner, idempotencyKey string) *iapiserver.DAGTaskGroupCreateRequest {
	arguments := map[string]any{
		"studio_application_id":  applicationID,
		"owner_user_id":          owner,
		"create_idempotency_key": idempotencyKey,
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
		IdempotencyScope: "appstudio-initialization", IdempotencyKey: idempotencyKey,
		CreatedBy: owner, TriggerType: iapiserver.DAGTriggerDomainEvent,
		TriggerSourceID: applicationID, TriggerSourceName: "AppStudio application creation",
	}
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
	if err != nil || project == nil || project.GitLabProjectID == "" || project.CommitSHA == "" {
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
	if err != nil || hookID <= 0 {
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
	if err != nil || commitSHA == "" {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit is unavailable")
	}
	committed, err := s.sourceProvider.ListFiles(ctx, projectID, commitSHA, "")
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, "appstudio starter commit is unavailable")
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
