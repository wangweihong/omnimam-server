package appstudio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type webhookStoreStub struct {
	store.AppStudioStore
	scope        *store.StudioApplicationGitLabScope
	workspace    *iapiserver.StudioWorkspace
	revisionErr  error
	snapshotCall int
}

func (s *webhookStoreStub) GetStudioApplicationGitLabScope(context.Context, string) (*store.StudioApplicationGitLabScope, error) {
	return s.scope, nil
}

func (s *webhookStoreStub) GetStudioWorkspace(context.Context, string, string) (*iapiserver.StudioWorkspace, error) {
	return s.workspace, nil
}

func (s *webhookStoreStub) GetStudioWorkspaceRevisionByCommit(context.Context, string, string, string) (*iapiserver.StudioWorkspaceRevision, error) {
	return nil, s.revisionErr
}

func (s *webhookStoreStub) CreateStudioSourceSnapshot(context.Context, string, *iapiserver.StudioSourceSnapshot) (*iapiserver.StudioSourceSnapshot, error) {
	s.snapshotCall++
	return nil, nil
}

type webhookAuthStub struct {
	projectID string
	err       error
}

func (s webhookAuthStub) EnsureAppStudioWebhook(context.Context, string) (int64, error) {
	return 1, nil
}
func (s webhookAuthStub) AuthenticateAppStudioWebhook(context.Context, int64, string) (string, error) {
	return s.projectID, s.err
}

type webhookTaskStub struct {
	seen map[string]bool
	dags []*iapiserver.DAGTaskGroupCreateRequest
}

func (s *webhookTaskStub) CreateDomainAtomicTask(context.Context, string, *iapiserver.AtomicTaskCreateRequest) (*iapiserver.AtomicTask, error) {
	return nil, fmt.Errorf("unexpected atomic task")
}
func (s *webhookTaskStub) CancelAtomicTask(context.Context, string, *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error) {
	return nil, fmt.Errorf("unexpected cancel")
}
func (s *webhookTaskStub) DomainDAGTaskGroupExists(_ context.Context, _ string, id string) bool {
	return s.seen[id]
}
func (s *webhookTaskStub) CreateDomainDAGTaskGroup(_ context.Context, _ string, req *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error) {
	s.dags = append(s.dags, req)
	s.seen[req.ID] = true
	return &iapiserver.DAGTaskGroup{ObjectMeta: imachinery.ObjectMeta{ID: req.ID}}, nil
}

func (s *webhookTaskStub) GetDAGTaskGroupDetail(context.Context, string) (*iapiserver.DAGTaskGroupDetail, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestProjectInitializationReturnsFourSafeStages(t *testing.T) {
	now := imachinery.Now()
	failed := now
	secret := "glpat-must-not-leak"
	detail := &iapiserver.DAGTaskGroupDetail{
		DAGTaskGroup: &iapiserver.DAGTaskGroup{ObjectMeta: imachinery.ObjectMeta{ID: "dag-1", UpdatedAt: now}, Status: iapiserver.TaskGroupStatusFailed, Progress: 0.25},
		ExecutionNodes: []*iapiserver.DAGNodeExecutionSummary{
			{NodeKey: "project-ensure", Status: iapiserver.AtomicTaskStatusFailed, AttemptCount: 5, CompletedAt: &failed, LatestError: &iapiserver.TaskError{Message: "No READY GitLabServer is configured as the AppStudio default: " + secret, OccurredAt: now}},
		},
	}
	result := projectInitialization(&iapiserver.StudioApplication{ObjectMeta: imachinery.ObjectMeta{ID: "app-1"}}, detail)
	if result.Status != iapiserver.AppStudioInitializationStatusError || len(result.Stages) != 4 {
		t.Fatalf("initialization = %#v, want ERROR with four stages", result)
	}
	stage := result.Stages[0]
	if stage.Stage != iapiserver.AppStudioInitializationStageGitLabProject || stage.AttemptCount != 5 || stage.LatestError == nil || stage.LatestError.Code != "ERR_GITLAB_APPSTUDIO_DEFAULT_SERVER_UNAVAILABLE" {
		t.Fatalf("project stage = %#v", stage)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatalf("initialization response leaked secret: %s", payload)
	}
}

func TestReceiveGitLabWebhookRejectsInvalidTokenWithoutSubmittingDAG(t *testing.T) {
	tasks := &webhookTaskStub{seen: map[string]bool{}}
	service := &Service{store: &webhookStoreStub{}, tasks: tasks, webhooks: webhookAuthStub{err: fmt.Errorf("invalid")}}
	payload := pushWebhookPayload()
	if _, err := service.ReceiveGitLabWebhook(t.Context(), "Push Hook", "invalid-token", payload); err == nil {
		t.Fatal("ReceiveGitLabWebhook() error = nil")
	}
	if len(tasks.dags) != 0 {
		t.Fatalf("submitted DAGs = %d, want 0", len(tasks.dags))
	}
}

func TestReceiveGitLabWebhookDeduplicatesPushByProjectAndCommit(t *testing.T) {
	tasks := &webhookTaskStub{seen: map[string]bool{}}
	storage := &webhookStoreStub{scope: &store.StudioApplicationGitLabScope{ApplicationID: "app-1", OwnerUserID: "user-1", WorkspaceID: "workspace-1"}}
	service := &Service{store: storage, tasks: tasks, webhooks: webhookAuthStub{projectID: "project-1"}}
	payload := pushWebhookPayload()
	first, err := service.ReceiveGitLabWebhook(t.Context(), "Push Hook", "valid-token", payload)
	if err != nil {
		t.Fatalf("first ReceiveGitLabWebhook() error = %v", err)
	}
	second, err := service.ReceiveGitLabWebhook(t.Context(), "Push Hook", "valid-token", payload)
	if err != nil {
		t.Fatalf("second ReceiveGitLabWebhook() error = %v", err)
	}
	if first.Duplicate || !second.Duplicate || first.DAGTaskGroupID == "" || first.DAGTaskGroupID != second.DAGTaskGroupID {
		t.Fatalf("first = %#v, second = %#v", first, second)
	}
	if len(tasks.dags) != 2 || tasks.dags[0].ID != tasks.dags[1].ID {
		t.Fatalf("submitted DAGs = %#v", tasks.dags)
	}
}

func TestEnsureAutomationSnapshotWaitsForCanonicalRevision(t *testing.T) {
	storage := &webhookStoreStub{
		scope:       &store.StudioApplicationGitLabScope{ApplicationID: "app-1", OwnerUserID: "user-1", WorkspaceID: "workspace-1"},
		workspace:   &iapiserver.StudioWorkspace{ObjectMeta: imachinery.ObjectMeta{ID: "workspace-1"}},
		revisionErr: fmt.Errorf("not projected"),
	}
	service := &Service{store: storage}
	_, err := service.EnsureAutomationSnapshot(t.Context(), iapiserver.AppStudioAutomationTaskArguments{
		StudioApplicationID: "app-1", OwnerUserID: "user-1", GitLabProjectID: "project-1",
		CommitSHA: "0123456789abcdef0123456789abcdef01234567", GitRef: "refs/heads/main",
	})
	if err == nil {
		t.Fatal("EnsureAutomationSnapshot() error = nil")
	}
	if storage.snapshotCall != 0 {
		t.Fatalf("snapshot creates = %d, want 0", storage.snapshotCall)
	}
}

func TestLoadBlueprintIncludesRequiredDotfiles(t *testing.T) {
	blueprint, err := LoadBlueprint(BlueprintWebReactID, BlueprintWebReactVersion)
	if err != nil {
		t.Fatalf("LoadBlueprint() error = %v", err)
	}
	for _, name := range []string{".gitignore", ".gitlab-ci.yml"} {
		if len(blueprint.Files[name]) == 0 {
			t.Errorf("LoadBlueprint() file %q is empty", name)
		}
	}
}

func TestMatchesInitializationRevisionRequiresCanonicalRevisionZero(t *testing.T) {
	workspace := &iapiserver.StudioWorkspace{CurrentRevisionDigest: "sha256:starter"}
	revision := &iapiserver.StudioWorkspaceRevision{Revision: 0, CommitSHA: "commit-1", ContentDigest: "sha256:starter"}
	if !matchesInitializationRevision(revision, workspace, "commit-1") {
		t.Fatal("matchesInitializationRevision() rejected persisted initialization revision")
	}
	revision.Revision = 1
	if matchesInitializationRevision(revision, workspace, "commit-1") {
		t.Fatal("matchesInitializationRevision() accepted non-initial revision")
	}
}

func pushWebhookPayload() *iapiserver.AppStudioGitLabWebhookPayload {
	payload := &iapiserver.AppStudioGitLabWebhookPayload{Ref: "refs/heads/main", CheckoutSHA: "0123456789abcdef0123456789abcdef01234567"}
	payload.Project.ID = 42
	return payload
}
