package gitlab

import (
	"context"
	"io"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// RepositoryClient 是 AppStudio SourceProvider 消费的 GitLab Repository 最小边界。
type RepositoryClient interface {
	GetProject(context.Context, int64) (*RemoteProject, error)
	ListRepositoryTree(context.Context, int64, string, string) ([]RepositoryTreeEntry, error)
	GetRepositoryFile(context.Context, int64, string, string) (*RepositoryFile, error)
	GetRepositoryArchive(context.Context, int64, string) (io.ReadCloser, error)
	GetProjectByPath(context.Context, string) (*RemoteProject, error)
	GetBranchHead(context.Context, int64, string) (*BranchHead, error)
	CompareCommits(context.Context, int64, string, string) (*CommitComparison, error)
	CreateCommit(context.Context, int64, CreateCommitRequest) (*Commit, error)
}

// ProjectAccessTokenClient 是 Runtime Git access 的最小 token 生命周期边界。
type ProjectAccessTokenClient interface {
	CreateProjectAccessToken(context.Context, int64, CreateProjectAccessTokenRequest) (*ProjectAccessToken, error)
	RevokeProjectAccessToken(context.Context, int64, int64) error
}

type ProjectHookClient interface {
	CreateProjectHook(context.Context, int64, CreateProjectHookRequest) (*ProjectHook, error)
	GetProjectHook(context.Context, int64, int64) (*ProjectHook, error)
}

// Client 定义 GitLab Service 与 Pipeline Worker 消费的最小远端 API。
type Client interface {
	GetVersion(context.Context) (*Version, error)
	GetCurrentUser(context.Context) (*User, error)
	ResolveNamespace(context.Context, string) (*Namespace, error)
	CreateProject(context.Context, CreateProjectRequest) (*RemoteProject, error)
	GetProject(context.Context, int64) (*RemoteProject, error)
	DeleteProject(context.Context, int64) error
	CreatePipeline(context.Context, int64, CreatePipelineRequest) (*Pipeline, error)
	GetPipeline(context.Context, int64, int64) (*Pipeline, error)
	RetryPipeline(context.Context, int64, int64) (*Pipeline, error)
	CancelPipeline(context.Context, int64, int64) (*Pipeline, error)
	ListPipelineJobs(context.Context, int64, int64) ([]Job, error)
}

type PipelineArtifactClient interface {
	DownloadJobArtifact(context.Context, int64, int64, string) (io.ReadCloser, error)
}

// ClientFactory 从持久化 GitLabServer 构造不泄露 credential 的远端 Client。
type ClientFactory interface {
	NewClient(*iapiserver.GitLabServer) (Client, error)
}

type Version struct {
	Version string `json:"version"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Namespace struct {
	ID       int64  `json:"id"`
	FullPath string `json:"full_path"`
}

type CreateProjectRequest struct {
	Name                 string
	Path                 string
	Description          string
	NamespaceID          int64
	InitializeWithReadme bool
	DefaultBranch        string
}

type RemoteProject struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
	DefaultBranch     string `json:"default_branch"`
}

type CreatePipelineRequest struct {
	Ref       string
	Variables map[string]string
}

type Pipeline struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
	SHA    string `json:"sha"`
}

type Job struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CreateProjectHookRequest struct {
	URL            string
	Token          string
	PushEvents     bool
	PipelineEvents bool
}

type ProjectHook struct {
	ID             int64  `json:"id"`
	URL            string `json:"url"`
	PushEvents     bool   `json:"push_events"`
	PipelineEvents bool   `json:"pipeline_events"`
}

type RepositoryTreeEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Mode string `json:"mode"`
}

type RepositoryFile struct {
	FileName   string `json:"file_name"`
	FilePath   string `json:"file_path"`
	Size       int64  `json:"size"`
	Encoding   string `json:"encoding"`
	Content    string `json:"content"`
	ContentSHA string `json:"content_sha256"`
	BlobID     string `json:"blob_id"`
	CommitID   string `json:"commit_id"`
	LastCommit string `json:"last_commit_id"`
}

type BranchHead struct {
	Name   string `json:"name"`
	Commit struct {
		ID string `json:"id"`
	} `json:"commit"`
}

type CommitComparison struct {
	CompareSameRef bool     `json:"compare_same_ref"`
	Commits        []Commit `json:"commits"`
}

type Commit struct {
	ID        string   `json:"id"`
	ShortID   string   `json:"short_id"`
	Title     string   `json:"title"`
	Message   string   `json:"message"`
	ParentIDs []string `json:"parent_ids"`
}

type CommitAction struct {
	Action       string `json:"action"`
	FilePath     string `json:"file_path"`
	Content      string `json:"content,omitempty"`
	PreviousPath string `json:"previous_path,omitempty"`
}

type CreateCommitRequest struct {
	Branch        string
	StartBranch   string
	CommitMessage string
	BaseSHA       string
	Actions       []CommitAction
}

type CreateProjectAccessTokenRequest struct {
	Name        string
	Scopes      []string
	AccessLevel int
	ExpiresAt   string
}

type ProjectAccessToken struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Username  string   `json:"username"`
	Token     string   `json:"token"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at"`
}

// RemoteError 保留用于分支判断的 HTTP status，不保存或暴露 GitLab 原始响应正文。
type RemoteError struct {
	StatusCode int
	Operation  string
	Message    string
}

func (e *RemoteError) Error() string {
	if e.Message == "" {
		return e.Operation + " failed"
	}
	return e.Operation + " failed: " + e.Message
}
