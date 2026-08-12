package appstudio

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

var gitCommitSHA = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

func (s *Service) ReceiveGitLabWebhook(ctx context.Context, event, token string, payload *iapiserver.AppStudioGitLabWebhookPayload) (*iapiserver.AppStudioGitLabWebhookAccepted, error) {
	if s.webhooks == nil || payload == nil || payload.Project.ID <= 0 || token == "" {
		return nil, errors.NewStatus(code.ErrAppStudioWebhookUnauthorized, "appstudio webhook token is invalid or project is unavailable")
	}
	projectID, err := s.webhooks.AuthenticateAppStudioWebhook(ctx, payload.Project.ID, token)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioWebhookUnauthorized, "appstudio webhook token is invalid or project is unavailable")
	}
	scope, err := s.store.GetStudioApplicationGitLabScope(ctx, projectID)
	if err != nil || scope == nil {
		return nil, errors.NewStatus(code.ErrAppStudioWebhookUnauthorized, "appstudio webhook token is invalid or project is unavailable")
	}
	switch event {
	case "Push Hook":
		commitSHA := strings.ToLower(strings.TrimSpace(payload.CheckoutSHA))
		gitRef := strings.TrimSpace(payload.Ref)
		if !gitCommitSHA.MatchString(commitSHA) || !strings.HasPrefix(gitRef, "refs/heads/") || len(gitRef) > 255 {
			return nil, errors.NewStatus(code.ErrAppStudioWebhookPayloadInvalid, "appstudio push webhook payload is invalid")
		}
		if s.tasks == nil {
			return nil, errors.NewStatus(code.ErrAppStudioApplicationInvalidState, "task center is unavailable")
		}
		dag := studioPushDAG(scope, projectID, commitSHA, gitRef)
		duplicate := s.tasks.DomainDAGTaskGroupExists(ctx, iapiserver.AppStudioTaskDomain, dag.ID)
		created, err := s.tasks.CreateDomainDAGTaskGroup(ctx, iapiserver.AppStudioTaskDomain, dag)
		if err != nil {
			return nil, err
		}
		return &iapiserver.AppStudioGitLabWebhookAccepted{Accepted: true, Duplicate: duplicate, DAGTaskGroupID: created.ID}, nil
	case "Pipeline Hook":
		commitSHA := strings.ToLower(strings.TrimSpace(payload.ObjectAttributes.SHA))
		status := strings.ToLower(strings.TrimSpace(payload.ObjectAttributes.Status))
		if payload.ObjectAttributes.ID <= 0 || !gitCommitSHA.MatchString(commitSHA) || status == "" {
			return nil, errors.NewStatus(code.ErrAppStudioWebhookPayloadInvalid, "appstudio pipeline webhook payload is invalid")
		}
		if _, err := s.store.ProjectStudioBuildPipeline(ctx, scope.ApplicationID, commitSHA, payload.ObjectAttributes.ID, strings.TrimSpace(payload.ObjectAttributes.URL), status); err != nil {
			return nil, errors.NewStatus(code.ErrAppStudioWebhookPayloadInvalid, "appstudio pipeline webhook does not match an automation build")
		}
		return &iapiserver.AppStudioGitLabWebhookAccepted{Accepted: true}, nil
	default:
		return nil, errors.NewStatus(code.ErrAppStudioWebhookPayloadInvalid, "appstudio webhook event type is invalid")
	}
}

func studioPushDAG(scope *store.StudioApplicationGitLabScope, projectID, commitSHA, gitRef string) *iapiserver.DAGTaskGroupCreateRequest {
	pushKey := "push-" + projectID + "-" + commitSHA
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("appstudio:"+pushKey+":dag")).String()
	arguments := (iapiserver.AppStudioAutomationTaskArguments{
		StudioApplicationID: scope.ApplicationID, OwnerUserID: scope.OwnerUserID,
		GitLabProjectID: projectID, CommitSHA: commitSHA, GitRef: gitRef,
	}).AtomicTaskArguments()
	retry := iapiserver.RetryPolicy{MaxAttempts: 12, RetryDelaySeconds: 5, BackoffType: iapiserver.TaskWorkerRetryBackoffExponential, MaxRetryDelaySeconds: 30}
	internal := func(key, name, ref string) iapiserver.DAGNode {
		return iapiserver.DAGNode{Key: key, Task: iapiserver.AtomicTaskTemplate{Key: key, Name: name, FunctionRef: ref, Arguments: arguments, RetryPolicy: retry}}
	}
	branch := strings.TrimPrefix(gitRef, "refs/heads/")
	return &iapiserver.DAGTaskGroupCreateRequest{
		ID: id, Name: "Build and preview AppStudio push",
		Nodes: []iapiserver.DAGNode{
			internal("snapshot", "Ensure canonical source snapshot", iapiserver.AppStudioFunctionAutomationSnapshotEnsure),
			internal("build", "Ensure Studio build", iapiserver.AppStudioFunctionAutomationBuildEnsure),
			{Key: "pipeline", Task: iapiserver.AtomicTaskTemplate{Key: "pipeline", Name: "Run GitLab pipeline", FunctionRef: iapiserver.GitLabFunctionPipelineRun, Arguments: (iapiserver.GitLabPipelineRunTaskArguments{GitLabProjectID: projectID, Ref: branch, Variables: map[string]string{"APPSTUDIO_COMMIT_SHA": commitSHA}}).AtomicTaskArguments()}},
			{Key: "artifact", Task: iapiserver.AtomicTaskTemplate{Key: "artifact", Name: "Complete Bundle artifact and preview", FunctionRef: iapiserver.AppStudioFunctionAutomationArtifactComplete, Arguments: arguments, RetryPolicy: retry}, InputMapping: map[string]any{"pipeline_id": "pipeline.pipeline_id", "pipeline_url": "pipeline.web_url"}},
		},
		Edges:     []iapiserver.DAGEdge{{FromNode: "snapshot", ToNode: "build"}, {FromNode: "build", ToNode: "pipeline"}, {FromNode: "pipeline", ToNode: "artifact"}},
		ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
		IdempotencyScope: "appstudio-push", IdempotencyKey: pushKey, CreatedBy: scope.OwnerUserID,
		TriggerType: iapiserver.DAGTriggerDomainEvent, TriggerSourceID: scope.ApplicationID,
		TriggerSourceName: fmt.Sprintf("GitLab push %s", commitSHA),
	}
}
