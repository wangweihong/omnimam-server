package appstudio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type AutomationArtifactLifecycle interface {
	Prepare(context.Context, *iapiserver.Artifact) (*iapiserver.Artifact, bool, error)
	StoreContent(context.Context, *iapiserver.Artifact, string, io.Reader) (*iapiserver.Artifact, error)
}

func (s *Service) EnsureAutomationSnapshot(ctx context.Context, arguments iapiserver.AppStudioAutomationTaskArguments) (map[string]any, error) {
	scope, workspace, err := s.automationScope(ctx, arguments)
	if err != nil {
		return nil, err
	}
	revision, err := s.store.GetStudioWorkspaceRevisionByCommit(ctx, workspace.ID, arguments.CommitSHA, arguments.OwnerUserID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceRevisionConflict, "canonical source revision is not available yet")
	}
	if err := s.requireNonEmptySourceRevision(ctx, workspace.ID, revision.Revision, arguments.OwnerUserID); err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotInvalid, "canonical source revision is empty")
	}
	snapshot := &iapiserver.StudioSourceSnapshot{
		ObjectMeta:          imachinery.ObjectMeta{ID: automationID(arguments.GitLabProjectID, arguments.CommitSHA, "snapshot")},
		StudioApplicationID: scope.ApplicationID, WorkspaceID: workspace.ID, WorkspaceRevision: revision.Revision,
		CommitSHA: arguments.CommitSHA, GitRef: arguments.GitRef, ContentDigest: revision.ContentDigest,
		ManifestDigest: revision.ContentDigest, Status: iapiserver.AppStudioSnapshotStatusReady, CreatedBy: arguments.OwnerUserID,
	}
	created, err := s.store.CreateStudioSourceSnapshot(ctx, arguments.OwnerUserID, snapshot)
	if err != nil {
		return nil, err
	}
	return map[string]any{"source_snapshot_id": created.ID, "workspace_revision": created.WorkspaceRevision, "commit_sha": created.CommitSHA}, nil
}

func (s *Service) EnsureAutomationBuild(ctx context.Context, arguments iapiserver.AppStudioAutomationTaskArguments) (map[string]any, error) {
	if _, _, err := s.automationScope(ctx, arguments); err != nil {
		return nil, err
	}
	snapshotID := automationID(arguments.GitLabProjectID, arguments.CommitSHA, "snapshot")
	snapshot, err := s.store.GetStudioSourceSnapshot(ctx, snapshotID, arguments.OwnerUserID)
	if err != nil || snapshot.CommitSHA != arguments.CommitSHA {
		return nil, errors.NewStatus(code.ErrAppStudioSnapshotInvalid, "automation source snapshot is unavailable")
	}
	buildID := automationID(arguments.GitLabProjectID, arguments.CommitSHA, "build")
	build := &iapiserver.StudioBuild{
		ObjectMeta: imachinery.ObjectMeta{ID: buildID, Name: "Build " + buildID}, OwnerUserID: arguments.OwnerUserID,
		StudioApplicationID: arguments.StudioApplicationID, SourceSnapshotID: snapshot.ID, CommitSHA: arguments.CommitSHA,
		Status: iapiserver.AppStudioBuildStatusPending, IdempotencyKey: "push-" + arguments.GitLabProjectID + "-" + arguments.CommitSHA,
	}
	created, err := s.store.CreateStudioBuild(ctx, arguments.OwnerUserID, build)
	if err != nil {
		return nil, err
	}
	return map[string]any{"studio_build_id": created.ID, "source_snapshot_id": snapshot.ID, "commit_sha": created.CommitSHA}, nil
}

func (s *Service) CompleteAutomationArtifact(ctx context.Context, arguments iapiserver.AppStudioAutomationTaskArguments, pipelineID int64, pipelineURL, atomicTaskID, attemptID string, lifecycle AutomationArtifactLifecycle) (map[string]any, error) {
	_, workspace, err := s.automationScope(ctx, arguments)
	if err != nil {
		return nil, err
	}
	if pipelineID <= 0 || lifecycle == nil || s.pipelineArtifacts == nil || s.tasks == nil {
		return nil, errors.NewStatus(code.ErrAppStudioBuildFailed, "pipeline artifact completion is unavailable")
	}
	buildID := automationID(arguments.GitLabProjectID, arguments.CommitSHA, "build")
	build, err := s.store.GetStudioBuild(ctx, buildID, arguments.OwnerUserID)
	if err != nil || build.CommitSHA != arguments.CommitSHA {
		return nil, errors.NewStatus(code.ErrAppStudioBuildFailed, "automation build is unavailable")
	}
	if build.Status == iapiserver.AppStudioBuildStatusSucceeded && build.ArtifactID != "" && build.ArtifactDigest != "" {
		return map[string]any{"studio_build_id": build.ID, "artifact_id": build.ArtifactID, "artifact_digest": build.ArtifactDigest}, nil
	}
	bundle, err := s.pipelineArtifacts.DownloadAppStudioBundle(ctx, arguments.GitLabProjectID, pipelineID)
	if err != nil || bundle == nil || bundle.ContentDigest == "" || len(bundle.Content) == 0 {
		return nil, errors.NewStatus(code.ErrAppStudioBuildFailed, "gitlab pipeline bundle is unavailable")
	}
	artifact, _, err := lifecycle.Prepare(ctx, &iapiserver.Artifact{
		OwnerUserID: arguments.OwnerUserID, ProducerType: iapiserver.TaskWorkerArtifactProducerTypeStudioBuild,
		ProducerID: build.ID, ProducerIdempotencyKey: iapiserver.TaskWorkerBuildIdempotencyPrefix + build.ID + ":bundle",
		AtomicTaskID: atomicTaskID, TaskAttemptID: attemptID, OutputKey: "bundle",
		ArtifactType: iapiserver.TaskWorkerArtifactTypeBuildBundle, MediaType: iapiserver.AssetMediaTypeOther,
		SavePolicy: iapiserver.ArtifactSaveAutomatic, ProcessingProfileVersion: iapiserver.TaskWorkerArtifactProcessingProfileBuildBundle,
		Metadata: map[string]any{iapiserver.TaskWorkerKeyContentType: bundle.MediaType},
	})
	if err != nil {
		return nil, err
	}
	if artifact.ProcessingStatus != iapiserver.ArtifactProcessingReady {
		artifact, err = lifecycle.StoreContent(ctx, artifact, bundle.MediaType, bytes.NewReader(bundle.Content))
		if err != nil {
			return nil, err
		}
	}
	digest, _ := artifact.Metadata[iapiserver.TaskWorkerKeySHA256].(string)
	if artifact.ProcessingStatus != iapiserver.ArtifactProcessingReady || "sha256:"+strings.ToLower(digest) != bundle.ContentDigest {
		return nil, errors.NewStatus(code.ErrAppStudioBuildFailed, "gitlab pipeline bundle digest does not match artifact")
	}
	build.AtomicTaskID, build.PipelineID, build.PipelineURL = atomicTaskID, pipelineID, pipelineURL
	build.ArtifactID, build.ArtifactDigest, build.Status = artifact.ID, bundle.ContentDigest, iapiserver.AppStudioBuildStatusSucceeded
	build, err = s.store.UpdateStudioBuild(ctx, build)
	if err != nil {
		return nil, err
	}
	revision, err := s.store.GetStudioWorkspaceRevisionByCommit(ctx, workspace.ID, arguments.CommitSHA, arguments.OwnerUserID)
	if err != nil {
		return nil, err
	}
	previewID := automationID(arguments.GitLabProjectID, arguments.CommitSHA, "preview")
	preview, err := s.store.CreateStudioPreviewRuntime(ctx, arguments.OwnerUserID, &iapiserver.StudioPreviewRuntime{
		ObjectMeta: imachinery.ObjectMeta{ID: previewID}, StudioApplicationID: arguments.StudioApplicationID,
		WorkspaceID: workspace.ID, WorkspaceRevision: revision.Revision, Status: iapiserver.AppStudioPreviewStatusPending,
		ExpiresAt: imachinery.Time{Time: time.Now().Add(24 * time.Hour)},
	})
	if err != nil {
		return nil, err
	}
	previewArguments := (iapiserver.AppStudioPreviewTaskArguments{
		StudioApplicationID: arguments.StudioApplicationID, PreviewRuntimeID: preview.ID, WorkspaceID: workspace.ID,
		WorkspaceRevision: revision.Revision, WorkspaceRevisionSourceRef: iapiserver.AppStudioRefPrefixWorkspaceRevision + workspace.ID + "/" + fmt.Sprint(revision.Revision),
		RuntimeProfileID: iapiserver.AppStudioPreviewProfileStaticWeb, RuntimeProfileRevision: iapiserver.AppStudioRuntimeProfileRevision,
		EndpointVisibility:      iapiserver.AppStudioEndpointVisibilityUser,
		AuthorizationRef:        fmt.Sprintf(iapiserver.AppStudioRefPrefixPreviewGrant+"%s/%s/%d", workspace.ID, preview.ID, preview.ResourceVersion),
		ExpectedResourceVersion: preview.ResourceVersion,
	}).AtomicTaskArguments()
	if _, err := s.tasks.CreateDomainAtomicTask(ctx, iapiserver.AppStudioTaskDomain, &iapiserver.AtomicTaskCreateRequest{
		Key: "preview-" + preview.ID, Name: "AppStudio automatic preview", FunctionRef: iapiserver.AppStudioFunctionPreviewEnsure,
		Arguments: previewArguments, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"studio_build_id": build.ID, "artifact_id": artifact.ID, "artifact_digest": bundle.ContentDigest, "preview_runtime_id": preview.ID}, nil
}

func (s *Service) automationScope(ctx context.Context, arguments iapiserver.AppStudioAutomationTaskArguments) (*store.StudioApplicationGitLabScope, *iapiserver.StudioWorkspace, error) {
	if arguments.StudioApplicationID == "" || arguments.OwnerUserID == "" || arguments.GitLabProjectID == "" || !gitCommitSHA.MatchString(arguments.CommitSHA) || !strings.HasPrefix(arguments.GitRef, "refs/heads/") {
		return nil, nil, fmt.Errorf("appstudio automation arguments are invalid")
	}
	scope, err := s.store.GetStudioApplicationGitLabScope(ctx, arguments.GitLabProjectID)
	if err != nil || scope == nil || scope.ApplicationID != arguments.StudioApplicationID || scope.OwnerUserID != arguments.OwnerUserID {
		return nil, nil, errors.NewStatus(code.ErrAppStudioAccessDenied, "appstudio automation scope is invalid")
	}
	workspace, err := s.store.GetStudioWorkspace(ctx, scope.WorkspaceID, arguments.OwnerUserID)
	return scope, workspace, err
}

func automationID(projectID, commitSHA, resource string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("appstudio:push:"+projectID+":"+commitSHA+":"+resource)).String()
}
