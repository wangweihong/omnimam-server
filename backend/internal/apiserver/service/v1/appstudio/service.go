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
	CreateCodingAgentForStudio(context.Context, string, string, string, string) (*iapiserver.Agent, error)
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

func (s *Service) ListApplications(ctx context.Context, req *iapiserver.StudioApplicationListRequest) (*iapiserver.StudioApplicationListResponse, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = owner
	items, total, err := s.store.ListStudioApplications(ctx, req)
	return &iapiserver.StudioApplicationListResponse{Total: total, Items: items}, err
}
func (s *Service) CreateApplication(ctx context.Context, req *iapiserver.StudioApplicationCreateRequest) (*iapiserver.StudioApplication, error) {
	owner, err := studioUserID(ctx)
	if err != nil {
		return nil, err
	}
	appID, repoID, workspaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	app := &iapiserver.StudioApplication{ObjectMeta: imachinery.ObjectMeta{ID: appID, Name: req.Name, Description: req.Description}, OwnerUserID: owner, Status: iapiserver.AppStudioApplicationStatusCreating, DefaultWorkspaceID: workspaceID}
	repository := &iapiserver.StudioSourceRepository{ObjectMeta: imachinery.ObjectMeta{ID: repoID, Name: req.Name + " source"}, StudioApplicationID: appID, ProviderType: iapiserver.AppStudioSourceProviderBuiltIn, Status: iapiserver.AppStudioRepositoryStatusReady}
	workspace := &iapiserver.StudioWorkspace{ObjectMeta: imachinery.ObjectMeta{ID: workspaceID, Name: iapiserver.AppStudioDefaultWorkspaceName}, StudioApplicationID: appID, RepositoryID: repoID, Status: iapiserver.AppStudioWorkspaceStatusReady, CurrentRevisionDigest: emptyTreeDigest()}
	revision := &iapiserver.StudioWorkspaceRevision{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, WorkspaceID: workspaceID, Revision: 0, ContentDigest: emptyTreeDigest(), CreatedBy: owner}
	if err := s.sources.WriteRevision(ctx, workspaceID, 0, map[string][]byte{}); err != nil {
		return nil, errors.NewStatus(code.ErrAppStudioSourceChangeRejected, err.Error())
	}
	if err := s.store.CreateStudioApplicationAggregate(ctx, app, repository, workspace, revision); err != nil {
		return nil, err
	}
	if s.agents == nil {
		app.Status = iapiserver.AppStudioApplicationStatusError
		_, _ = s.store.UpdateStudioApplication(ctx, app, app.ResourceVersion)
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, "coding agent initialization is unavailable")
	}
	if _, err := s.agents.CreateCodingAgentForStudio(ctx, appID, workspaceID, owner, appID); err != nil {
		app.Status = iapiserver.AppStudioApplicationStatusError
		_, _ = s.store.UpdateStudioApplication(ctx, app, app.ResourceVersion)
		return nil, errors.NewStatus(code.ErrAgentInitializationFailed, err.Error())
	}
	app.Status = iapiserver.AppStudioApplicationStatusReady
	if _, err := s.store.UpdateStudioApplication(ctx, app, app.ResourceVersion); err != nil {
		return nil, err
	}
	return app, nil
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
