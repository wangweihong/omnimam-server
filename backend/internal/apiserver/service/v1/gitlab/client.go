package gitlab

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

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
	Name        string
	Path        string
	Description string
	NamespaceID int64
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
}

type Job struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
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
