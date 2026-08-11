package gitlab

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type Dependencies struct {
	Store   store.GitLabStore
	Clients ClientFactory
}

// Service 编排 GitLabServer、GitLabProject 和远端 GitLab API；它不依赖 AppStudio。
type Service struct {
	store   store.GitLabStore
	clients ClientFactory
}

func New(deps Dependencies) (*Service, error) {
	if deps.Store == nil || deps.Clients == nil {
		return nil, fmt.Errorf("gitlab store and client factory are required")
	}
	return &Service{store: deps.Store, clients: deps.Clients}, nil
}

func (s *Service) ListServers(ctx context.Context, req *iapiserver.GitLabServerListRequest) (*iapiserver.GitLabServerListResponse, error) {
	items, total, err := s.store.ListGitLabServers(ctx, req)
	return &iapiserver.GitLabServerListResponse{Total: total, Items: items}, err
}

func (s *Service) GetServer(ctx context.Context, id string) (*iapiserver.GitLabServer, error) {
	item, err := s.store.GetGitLabServer(ctx, id)
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.NewStatus(code.ErrGitLabServerNotFound, "gitlab server is not visible")
	}
	return item, err
}

func (s *Service) CreateServer(ctx context.Context, req *iapiserver.GitLabServerCreateRequest) (*iapiserver.GitLabServer, error) {
	item := &iapiserver.GitLabServer{ObjectMeta: imachinery.ObjectMeta{Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description)}, APIURL: strings.TrimRight(req.APIURL, "/"), ExternalURL: strings.TrimRight(req.ExternalURL, "/"), NamespacePath: strings.Trim(req.NamespacePath, "/ "), Credential: strings.TrimSpace(req.Credential), Status: iapiserver.GitLabServerStatusUnknown}
	created, err := s.store.CreateGitLabServer(ctx, item)
	if stderrors.Is(err, store.ErrGitLabServerNameConflict) {
		return nil, errors.NewStatus(code.ErrGitLabServerNameConflict, "gitlab server name already exists")
	}
	return created, err
}

func (s *Service) UpdateServer(ctx context.Context, id string, req *iapiserver.GitLabServerUpdateRequest) (*iapiserver.GitLabServer, error) {
	current, err := s.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	connectionChanged := false
	if req.Name != nil {
		current.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		current.Description = strings.TrimSpace(*req.Description)
	}
	if req.APIURL != nil {
		value := strings.TrimRight(*req.APIURL, "/")
		connectionChanged = connectionChanged || value != current.APIURL
		current.APIURL = value
	}
	if req.ExternalURL != nil {
		current.ExternalURL = strings.TrimRight(*req.ExternalURL, "/")
	}
	if req.NamespacePath != nil {
		value := strings.Trim(*req.NamespacePath, "/ ")
		connectionChanged = connectionChanged || value != current.NamespacePath
		current.NamespacePath = value
	}
	if req.Credential != nil {
		value := strings.TrimSpace(*req.Credential)
		connectionChanged = connectionChanged || value != current.Credential
		current.Credential = value
	}
	if connectionChanged {
		current.Status, current.LastCheckedAt, current.LastError = iapiserver.GitLabServerStatusUnknown, nil, ""
	}
	expectedVersion := int64(0)
	if req.ResourceVersion != nil {
		expectedVersion = *req.ResourceVersion
	}
	updated, err := s.store.UpdateGitLabServer(ctx, current, expectedVersion)
	switch {
	case stderrors.Is(err, store.ErrGitLabServerNameConflict):
		return nil, errors.NewStatus(code.ErrGitLabServerNameConflict, "gitlab server name already exists")
	case stderrors.Is(err, store.ErrGitLabResourceVersionConflict):
		return nil, errors.NewStatus(code.ErrValidation, "gitlab server resource version conflict")
	default:
		return updated, err
	}
}

func (s *Service) TestServer(ctx context.Context, id string) (*iapiserver.GitLabServer, error) {
	server, err := s.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	client, err := s.clients.NewClient(server)
	if err == nil {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err = client.GetVersion(probeCtx); err == nil {
			if _, err = client.GetCurrentUser(probeCtx); err == nil {
				_, err = client.ResolveNamespace(probeCtx, server.NamespacePath)
			}
		}
	}
	now := imachinery.Now()
	server.LastCheckedAt = &now
	if err != nil {
		server.Status = iapiserver.GitLabServerStatusError
		server.LastError = sanitizeRemoteError(err, server.Credential)
	} else {
		server.Status = iapiserver.GitLabServerStatusReady
		server.LastError = ""
	}
	return s.store.UpdateGitLabServer(ctx, server, server.ResourceVersion)
}

func (s *Service) DeleteServer(ctx context.Context, id string) error {
	err := s.store.DeleteGitLabServer(ctx, id)
	switch {
	case stderrors.Is(err, store.ErrGitLabServerHasProjects):
		return errors.NewStatus(code.ErrGitLabServerHasProjects, "gitlab server still has projects")
	case stderrors.Is(err, gorm.ErrRecordNotFound):
		return errors.NewStatus(code.ErrGitLabServerNotFound, "gitlab server is not visible")
	default:
		return err
	}
}

func (s *Service) ListProjects(ctx context.Context, req *iapiserver.GitLabProjectListRequest) (*iapiserver.GitLabProjectListResponse, error) {
	items, total, err := s.store.ListGitLabProjects(ctx, req)
	return &iapiserver.GitLabProjectListResponse{Total: total, Items: items}, err
}

func (s *Service) GetProject(ctx context.Context, id string) (*iapiserver.GitLabProject, error) {
	item, err := s.store.GetGitLabProject(ctx, id)
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.NewStatus(code.ErrGitLabProjectNotFound, "gitlab project is not visible")
	}
	return item, err
}

func (s *Service) CreateProject(ctx context.Context, req *iapiserver.GitLabProjectCreateRequest) (*iapiserver.GitLabProject, error) {
	server, err := s.GetServer(ctx, req.GitLabServerID)
	if err != nil {
		return nil, err
	}
	if server.Status != iapiserver.GitLabServerStatusReady {
		return nil, errors.NewStatus(code.ErrGitLabProjectServerNotReady, "gitlab server is not ready")
	}
	client, err := s.clients.NewClient(server)
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabProjectRemoteFailed, sanitizeRemoteError(err, server.Credential))
	}
	namespace, err := client.ResolveNamespace(ctx, server.NamespacePath)
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabProjectRemoteFailed, sanitizeRemoteError(err, server.Credential))
	}
	remote, err := client.CreateProject(ctx, CreateProjectRequest{Name: strings.TrimSpace(req.Name), Path: req.Path, Description: strings.TrimSpace(req.Description), NamespaceID: namespace.ID})
	if err != nil {
		return nil, errors.NewStatus(code.ErrGitLabProjectRemoteFailed, sanitizeRemoteError(err, server.Credential))
	}
	item := projectProjection(server.ID, req.Description, remote)
	created, err := s.store.CreateGitLabProject(ctx, item)
	if err == nil {
		return created, nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if cleanupErr := client.DeleteProject(cleanupCtx, remote.ID); cleanupErr != nil {
		log.Warnf("gitlab project compensation failed: project_id=%d error=%s", remote.ID, sanitizeRemoteError(cleanupErr, server.Credential))
	}
	return nil, errors.NewStatus(code.ErrGitLabProjectProjectionFailed, "gitlab project projection could not be persisted")
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	project, err := s.GetProject(ctx, id)
	if err != nil {
		return err
	}
	server, err := s.GetServer(ctx, project.GitLabServerID)
	if err != nil {
		return err
	}
	client, err := s.clients.NewClient(server)
	if err == nil {
		err = client.DeleteProject(ctx, project.ExternalProjectID)
	}
	var remoteErr *RemoteError
	if err != nil && !(stderrors.As(err, &remoteErr) && remoteErr.StatusCode == http.StatusNotFound) {
		return errors.NewStatus(code.ErrGitLabProjectRemoteFailed, sanitizeRemoteError(err, server.Credential))
	}
	if err := s.store.DeleteGitLabProject(ctx, project.ID); err != nil {
		return err
	}
	return nil
}

// CancelTask 使用 Task Center 提供的 runtime checkpoint 尽力取消同一 GitLab Pipeline，不修改 AtomicTask。
func (s *Service) CancelTask(ctx context.Context, task *iapiserver.AtomicTask, checkpoint map[string]any) error {
	if task == nil || task.FunctionRef != iapiserver.GitLabFunctionPipelineRun {
		return fmt.Errorf("gitlab pipeline task is invalid")
	}
	projectID, _ := task.Arguments["gitlab_project_id"].(string)
	externalJobID, _ := checkpoint[iapiserver.TaskWorkerKeyExternalJobID].(string)
	pipelineID, err := strconv.ParseInt(externalJobID, 10, 64)
	if projectID == "" || externalJobID == "" {
		return nil
	}
	if err != nil || pipelineID <= 0 {
		return fmt.Errorf("gitlab pipeline checkpoint is invalid")
	}
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("gitlab project is unavailable")
	}
	server, err := s.GetServer(ctx, project.GitLabServerID)
	if err != nil {
		return fmt.Errorf("gitlab server is unavailable")
	}
	client, err := s.clients.NewClient(server)
	if err != nil {
		return fmt.Errorf("gitlab client is unavailable")
	}
	cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = client.CancelPipeline(cancelCtx, project.ExternalProjectID, pipelineID)
	var remoteErr *RemoteError
	if err != nil && !(stderrors.As(err, &remoteErr) && remoteErr.StatusCode == http.StatusNotFound) {
		return fmt.Errorf("gitlab pipeline cancellation failed")
	}
	return nil
}

func projectProjection(serverID, description string, remote *RemoteProject) *iapiserver.GitLabProject {
	defaultBranch := remote.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	return &iapiserver.GitLabProject{ObjectMeta: imachinery.ObjectMeta{Name: remote.Name, Description: description}, GitLabServerID: serverID, ExternalProjectID: remote.ID, Path: remote.Path, PathWithNamespace: remote.PathWithNamespace, WebURL: remote.WebURL, HTTPURLToRepo: remote.HTTPURLToRepo, SSHURLToRepo: remote.SSHURLToRepo, DefaultBranch: defaultBranch}
}

func sanitizeRemoteError(err error, credential string) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if credential != "" {
		message = strings.ReplaceAll(message, credential, "[redacted]")
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	return message
}
