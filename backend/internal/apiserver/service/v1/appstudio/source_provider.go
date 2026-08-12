package appstudio

import (
	"context"
	"io"
	"time"
)

// SourceProvider 是 AppStudio 消费的源码正文边界；实现不得暴露远端 numeric ID、PAT 或凭据 URL。
type SourceProvider interface {
	ReadFile(context.Context, string, string, string) ([]byte, error)
	ListFiles(context.Context, string, string, string) ([]SourceFile, error)
	Commit(context.Context, string, string, string, string, []SourceAction) (*SourceCommit, error)
	BranchHead(context.Context, string, string) (string, error)
	Compare(context.Context, string, string, string) ([]SourceCommit, error)
	Archive(context.Context, string, string) (io.ReadCloser, error)
	CreateRuntimeGitAccess(context.Context, string, string, time.Time) (*RuntimeGitAccess, error)
	RevokeRuntimeGitAccess(context.Context, string, int64) error
}

// ProjectInitializer 是 AppStudio 初始化阶段创建 GitLab Project 和 Starter commit 的边界。
type ProjectInitializer interface {
	EnsureProject(context.Context, string, string, string, string, map[string][]byte) (*ProjectInitialization, error)
}

type ProjectInitialization struct {
	GitLabProjectID string
	DefaultBranch   string
	CommitSHA       string
}

type SourceFile struct {
	Path          string
	Content       []byte
	ContentDigest string
	SizeBytes     int64
}

type SourceAction struct {
	Operation  string
	Path       string
	Content    []byte
	TargetPath string
}

type SourceCommit struct {
	SHA       string
	ParentSHA string
	Message   string
}

// RuntimeGitAccess 只在当前进程内携带 Git clone 所需的最小信息，禁止持久化或记录日志。
type RuntimeGitAccess struct {
	CloneURL      string
	Username      string
	Token         string
	RemoteTokenID int64
}
