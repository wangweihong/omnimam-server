package gitlab

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appstudio "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"gorm.io/gorm"
)

const (
	maxRepositoryArchiveBytes     = 32 << 20
	maxRepositoryArchiveFileBytes = 2 << 20
	maxRepositoryContentBytes     = 64 << 20
	maxRepositoryFileCount        = 10000
)

// SourceProvider 是 GitLab domain 提供给 AppStudio 的 Repository adapter。
type SourceProvider struct {
	store   store.GitLabStore
	clients ClientFactory
}

// NewSourceProvider 构造 GitLab-only AppStudio SourceProvider。
func NewSourceProvider(store store.GitLabStore, clients ClientFactory) (*SourceProvider, error) {
	if store == nil || clients == nil {
		return nil, fmt.Errorf("gitlab source provider dependencies are required")
	}
	return &SourceProvider{store: store, clients: clients}, nil
}

func (p *SourceProvider) EnsureProject(ctx context.Context, localProjectID, name, projectPath, description string, files map[string][]byte) (*appstudio.ProjectInitialization, error) {
	project, projectErr := p.store.GetGitLabProject(ctx, localProjectID)
	if projectErr != nil && !stderrors.Is(projectErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load appstudio gitlab project projection: %w", projectErr)
	}
	var server *iapiserver.GitLabServer
	if projectErr != nil {
		server, projectErr = p.store.GetDefaultGitLabServer(ctx)
		if projectErr != nil || server == nil {
			return nil, fmt.Errorf("appstudio default gitlab server is unavailable")
		}
		project = &iapiserver.GitLabProject{ObjectMeta: imachinery.ObjectMeta{ID: localProjectID, Name: name, Description: description}, GitLabServerID: server.ID, Status: iapiserver.GitLabProjectStatusCreating, Path: projectPath, PathWithNamespace: strings.Trim(server.NamespacePath, "/") + "/" + strings.Trim(projectPath, "/"), DefaultBranch: "main"}
		if project, projectErr = p.store.CreateGitLabProject(ctx, project); projectErr != nil {
			project, projectErr = p.store.GetGitLabProject(ctx, localProjectID)
			if projectErr != nil {
				return nil, projectErr
			}
		}
	} else {
		// A retry must remain on the Server and path captured by its reservation.
		// The current default may have changed after the first durable reservation.
		server, projectErr = p.store.GetGitLabServer(ctx, project.GitLabServerID)
		if projectErr != nil || server == nil || server.Status != iapiserver.GitLabServerStatusReady {
			return nil, fmt.Errorf("reserved appstudio gitlab server is unavailable")
		}
	}
	client, err := p.clients.NewClient(server)
	if err != nil {
		return nil, p.reservationFailure(ctx, project, err)
	}
	repository, ok := client.(RepositoryClient)
	if !ok {
		return nil, p.reservationFailure(ctx, project, fmt.Errorf("gitlab client does not support repository operations"))
	}
	var remote *RemoteProject
	createdRemote := false
	if project.Status == iapiserver.GitLabProjectStatusReady {
		if project.ExternalProjectID <= 0 {
			return nil, fmt.Errorf("ready appstudio gitlab project projection is incomplete")
		}
		remote, err = repository.GetProject(ctx, project.ExternalProjectID)
	} else {
		remote, err = repository.GetProjectByPath(ctx, project.PathWithNamespace)
		if err != nil {
			if remoteErr, ok := err.(*RemoteError); !ok || remoteErr.StatusCode != 404 {
				return nil, p.reservationFailure(ctx, project, err)
			}
			namespace, resolveErr := client.ResolveNamespace(ctx, server.NamespacePath)
			if resolveErr != nil {
				return nil, p.reservationFailure(ctx, project, resolveErr)
			}
			remote, err = client.CreateProject(ctx, CreateProjectRequest{Name: project.Name, Path: project.Path, Description: project.Description, NamespaceID: namespace.ID, DefaultBranch: "main"})
			createdRemote = err == nil && remote != nil
		}
	}
	if err != nil || remote == nil {
		return nil, p.reservationFailure(ctx, project, fmt.Errorf("ensure appstudio gitlab project: %w", err))
	}
	branch := remote.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	starterActions := make([]appstudio.SourceAction, 0, len(files))
	for _, filePath := range sortedPaths(files) {
		starterActions = append(starterActions, appstudio.SourceAction{Operation: iapiserver.AppStudioChangeOperationCreate, Path: filePath, Content: files[filePath]})
	}
	head, headErr := repository.GetBranchHead(ctx, remote.ID, branch)
	commitSHA := ""
	if headErr != nil {
		remoteErr, ok := headErr.(*RemoteError)
		if !ok || remoteErr.StatusCode != 404 {
			return nil, p.reservationFailure(ctx, project, headErr)
		}
		commit, commitErr := repository.CreateCommit(ctx, remote.ID, CreateCommitRequest{Branch: branch, CommitMessage: "chore(appstudio): initialize web-react@v1", Actions: toRemoteActions(starterActions)})
		if commitErr != nil {
			return nil, p.reservationFailure(ctx, project, commitErr)
		}
		if commit == nil || commit.ID == "" {
			return nil, p.reservationFailure(ctx, project, fmt.Errorf("gitlab starter commit is unavailable"))
		}
		commitSHA = commit.ID
	} else {
		commitSHA = head.Commit.ID
		if commitSHA == "" {
			return nil, p.reservationFailure(ctx, project, fmt.Errorf("gitlab project branch head is empty"))
		}
		for _, file := range starterActions {
			current, readErr := repository.GetRepositoryFile(ctx, remote.ID, commitSHA, file.Path)
			if readErr != nil {
				return nil, p.reservationFailure(ctx, project, fmt.Errorf("appstudio starter template is missing %s", file.Path))
			}
			content, decodeErr := decodeRepositoryFileContent(current)
			if decodeErr != nil || string(content) != string(file.Content) {
				return nil, p.reservationFailure(ctx, project, fmt.Errorf("appstudio starter template conflicts with existing project"))
			}
		}
	}
	projection := projectProjection(server.ID, project.Description, remote)
	projection.ID, projection.CreatedAt, projection.ResourceVersion, projection.Status = project.ID, project.CreatedAt, project.ResourceVersion, iapiserver.GitLabProjectStatusReady
	project, err = p.store.UpdateGitLabProject(ctx, projection)
	if err != nil {
		if createdRemote {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			cleanupErr := client.DeleteProject(cleanupCtx, remote.ID)
			cancel()
			if cleanupErr != nil {
				return nil, p.reservationFailure(ctx, project, fmt.Errorf("compensate appstudio gitlab project: %w", cleanupErr))
			}
		}
		return nil, p.reservationFailure(ctx, project, fmt.Errorf("persist appstudio gitlab project projection: %w", err))
	}
	return &appstudio.ProjectInitialization{GitLabProjectID: project.ID, DefaultBranch: branch, CommitSHA: commitSHA}, nil
}

// reservationFailure preserves a retryable but unavailable remote initialization.
func (p *SourceProvider) reservationFailure(ctx context.Context, project *iapiserver.GitLabProject, cause error) error {
	if project == nil || project.Status == iapiserver.GitLabProjectStatusReady {
		return cause
	}
	if _, err := p.store.MarkGitLabProjectError(ctx, project.ID); err != nil {
		return fmt.Errorf("record appstudio gitlab project reservation failure: %w", cause)
	}
	return cause
}

func (p *SourceProvider) client(ctx context.Context, projectID string) (RepositoryClient, *iapiserver.GitLabProject, error) {
	project, err := p.store.GetGitLabProject(ctx, projectID)
	if err != nil || project == nil || project.Status != iapiserver.GitLabProjectStatusReady || project.ExternalProjectID <= 0 {
		return nil, nil, fmt.Errorf("gitlab project is unavailable")
	}
	server, err := p.store.GetGitLabServer(ctx, project.GitLabServerID)
	if err != nil || server == nil || server.Status != iapiserver.GitLabServerStatusReady {
		return nil, nil, fmt.Errorf("gitlab server is unavailable")
	}
	client, err := p.clients.NewClient(server)
	if err != nil {
		return nil, nil, err
	}
	repository, ok := client.(RepositoryClient)
	if !ok {
		return nil, nil, fmt.Errorf("gitlab client does not support repository operations")
	}
	return repository, project, nil
}

func (p *SourceProvider) ReadFile(ctx context.Context, projectID, commitSHA, filePath string) ([]byte, error) {
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return nil, err
	}
	file, err := client.GetRepositoryFile(ctx, project.ExternalProjectID, commitSHA, filePath)
	if err != nil {
		return nil, err
	}
	return decodeRepositoryFileContent(file)
}

func (p *SourceProvider) ListFiles(ctx context.Context, projectID, commitSHA, prefix string) ([]appstudio.SourceFile, error) {
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return nil, err
	}
	archive, err := client.GetRepositoryArchive(ctx, project.ExternalProjectID, commitSHA)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(io.LimitReader(archive, maxRepositoryArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("gitlab repository archive is invalid")
	}
	defer gzipReader.Close()
	tree := tar.NewReader(gzipReader)
	files := make([]appstudio.SourceFile, 0)
	total := int64(0)
	seen := make(map[string]struct{})
	for {
		header, nextErr := tree.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("gitlab repository archive is truncated")
		}
		name := strings.TrimPrefix(strings.ReplaceAll(header.Name, "\\", "/"), "./")
		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			if header.Typeflag == tar.TypeDir {
				continue
			}
			return nil, fmt.Errorf("gitlab repository archive entry has no project root")
		}
		filePath := strings.Join(parts[1:], "/")
		clean := path.Clean(filePath)
		if clean == "." || clean != filePath || path.IsAbs(clean) || strings.HasPrefix(clean, "../") || strings.ContainsRune(clean, '\x00') {
			return nil, fmt.Errorf("gitlab repository archive path is unsafe")
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("gitlab repository archive contains unsupported entry type")
		}
		if prefix != "" && !strings.HasPrefix(clean, prefix) {
			continue
		}
		if header.Size < 0 || header.Size > maxRepositoryArchiveFileBytes || total+header.Size > maxRepositoryContentBytes || len(files) >= maxRepositoryFileCount {
			return nil, fmt.Errorf("gitlab repository archive exceeds source limits")
		}
		if _, exists := seen[clean]; exists {
			return nil, fmt.Errorf("gitlab repository archive contains duplicate path")
		}
		content, readErr := io.ReadAll(io.LimitReader(tree, maxRepositoryArchiveFileBytes+1))
		if readErr != nil || int64(len(content)) != header.Size {
			return nil, fmt.Errorf("gitlab repository archive entry is truncated")
		}
		seen[clean] = struct{}{}
		total += int64(len(content))
		digest := sha256.Sum256(content)
		files = append(files, appstudio.SourceFile{Path: clean, Content: content, ContentDigest: "sha256:" + hex.EncodeToString(digest[:]), SizeBytes: int64(len(content))})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func (p *SourceProvider) Commit(ctx context.Context, projectID, branch, baseSHA, message string, actions []appstudio.SourceAction) (*appstudio.SourceCommit, error) {
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return nil, err
	}
	head, err := client.GetBranchHead(ctx, project.ExternalProjectID, branch)
	if err != nil {
		return nil, err
	}
	if head.Commit.ID != baseSHA {
		comparison, compareErr := client.CompareCommits(ctx, project.ExternalProjectID, baseSHA, head.Commit.ID)
		if compareErr == nil && len(comparison.Commits) == 1 && strings.Contains(comparison.Commits[0].Message, message) {
			return &appstudio.SourceCommit{SHA: comparison.Commits[0].ID, ParentSHA: baseSHA, Message: comparison.Commits[0].Message}, nil
		}
		return nil, fmt.Errorf("gitlab branch head does not match base commit")
	}
	remoteActions := make([]CommitAction, 0, len(actions))
	for _, action := range actions {
		content := string(action.Content)
		remote := CommitAction{Action: action.Operation, FilePath: action.Path, Content: content, PreviousPath: action.TargetPath}
		remoteActions = append(remoteActions, remote)
	}
	commit, err := client.CreateCommit(ctx, project.ExternalProjectID, CreateCommitRequest{Branch: branch, BaseSHA: baseSHA, CommitMessage: message, Actions: remoteActions})
	if err != nil {
		return nil, err
	}
	return &appstudio.SourceCommit{SHA: commit.ID, ParentSHA: baseSHA, Message: strings.TrimSpace(commit.Message)}, nil
}

func (p *SourceProvider) BranchHead(ctx context.Context, projectID, branch string) (string, error) {
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return "", err
	}
	head, err := client.GetBranchHead(ctx, project.ExternalProjectID, branch)
	if err != nil || head == nil || head.Commit.ID == "" {
		return "", fmt.Errorf("resolve gitlab branch head: %w", err)
	}
	return head.Commit.ID, nil
}

func (p *SourceProvider) Compare(ctx context.Context, projectID, baseSHA, headSHA string) ([]appstudio.SourceCommit, error) {
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return nil, err
	}
	comparison, err := client.CompareCommits(ctx, project.ExternalProjectID, baseSHA, headSHA)
	if err != nil || comparison == nil {
		return nil, fmt.Errorf("compare gitlab commits: %w", err)
	}
	commits := make([]appstudio.SourceCommit, 0, len(comparison.Commits))
	for _, commit := range comparison.Commits {
		parent := ""
		if len(commit.ParentIDs) == 1 {
			parent = commit.ParentIDs[0]
		}
		commits = append(commits, appstudio.SourceCommit{SHA: commit.ID, ParentSHA: parent, Message: strings.TrimSpace(commit.Message)})
	}
	return commits, nil
}

func (p *SourceProvider) Archive(ctx context.Context, projectID, commitSHA string) (io.ReadCloser, error) {
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return client.GetRepositoryArchive(ctx, project.ExternalProjectID, commitSHA)
}

func (p *SourceProvider) CreateRuntimeGitAccess(ctx context.Context, projectID, runtimeID string, expiresAt time.Time) (*appstudio.RuntimeGitAccess, error) {
	if runtimeID == "" || expiresAt.IsZero() || !expiresAt.After(time.Now()) {
		return nil, fmt.Errorf("runtime git access scope is invalid")
	}
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return nil, err
	}
	tokenClient, ok := client.(ProjectAccessTokenClient)
	if !ok {
		return nil, fmt.Errorf("gitlab client does not support project access tokens")
	}
	remote, err := client.GetProject(ctx, project.ExternalProjectID)
	if err != nil || remote == nil || remote.HTTPURLToRepo == "" {
		return nil, fmt.Errorf("gitlab project clone URL is unavailable")
	}
	// GitLab accepts a date-only expiry; Agent Runtime's shorter grant window still
	// controls use, and the next date provides a revocation-failure safety bound.
	date := expiresAt.UTC().Add(24 * time.Hour).Format("2006-01-02")
	token, err := tokenClient.CreateProjectAccessToken(ctx, project.ExternalProjectID, CreateProjectAccessTokenRequest{
		Name: "omnimam-runtime-" + runtimeID, Scopes: []string{"write_repository"}, AccessLevel: 30, ExpiresAt: date,
	})
	if err != nil {
		return nil, fmt.Errorf("create gitlab runtime access token: %w", err)
	}
	if token == nil || token.ID <= 0 || token.Username == "" || token.Token == "" {
		return nil, fmt.Errorf("create gitlab runtime access token returned an invalid response")
	}
	return &appstudio.RuntimeGitAccess{CloneURL: remote.HTTPURLToRepo, Username: token.Username, Token: token.Token, RemoteTokenID: token.ID}, nil
}

func (p *SourceProvider) RevokeRuntimeGitAccess(ctx context.Context, projectID string, remoteTokenID int64) error {
	if remoteTokenID <= 0 {
		return fmt.Errorf("gitlab runtime token id is invalid")
	}
	client, project, err := p.client(ctx, projectID)
	if err != nil {
		return err
	}
	tokenClient, ok := client.(ProjectAccessTokenClient)
	if !ok {
		return fmt.Errorf("gitlab client does not support project access tokens")
	}
	return tokenClient.RevokeProjectAccessToken(ctx, project.ExternalProjectID, remoteTokenID)
}

func sortedPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func toRemoteActions(actions []appstudio.SourceAction) []CommitAction {
	result := make([]CommitAction, 0, len(actions))
	for _, action := range actions {
		result = append(result, CommitAction{Action: action.Operation, FilePath: action.Path, Content: string(action.Content), PreviousPath: action.TargetPath})
	}
	return result
}

var _ appstudio.SourceProvider = (*SourceProvider)(nil)
var _ appstudio.ProjectInitializer = (*SourceProvider)(nil)
