package providers

import (
	"context"
	"io"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type ProviderRequest struct {
	RuntimeID   string
	Profile     *iapiserver.InfraRuntimeProfile
	Request     *iapiserver.InfraCreateRuntimeRequest
	MCPBindings []ResolvedMCPBinding
	// SourceArchive 只在当前 Ensure 调用内存中存在，不得持久化或写入日志。
	SourceArchive       []byte
	SourceContentDigest string
	// RuntimeGitAccess exists only during Coding Runtime startup and is never persisted or logged.
	RuntimeGitAccess *RuntimeGitAccess
}

// RuntimeGitAccess is the transient Git clone credential resolved from a SECRET_REF.
type RuntimeGitAccess struct {
	CloneURL string
	Username string
	Token    string
}

// ResolvedMCPBinding is an in-memory result of the Agent authorization resolver.
// It must never be persisted or logged.
type ResolvedMCPBinding struct {
	ServerKey     string
	ServerType    string
	Endpoint      string
	Credential    string
	AllowedTools  []string
	Configuration map[string]any
}

type ProviderResult struct {
	ProviderRuntimeRef string
	Status             string
	EndpointDisplayRef string
	Endpoint           *ProviderEndpoint
	ArtifactDigest     string
	Outputs            []*iapiserver.InfraRuntimeOutput
	OutputContents     map[string]ProviderOutputContent
}

type ProviderEndpoint struct {
	Protocol   string
	BaseURL    string
	ValidUntil time.Time
}

type ProviderOutputContent struct {
	MediaType     string
	SizeBytes     int64
	ContentDigest string
	CollectedAt   time.Time
	Open          func(context.Context) (io.ReadCloser, error)
}

type RuntimeProvider interface {
	Info(context.Context) (*iapiserver.InfraNode, error)
	Ensure(context.Context, ProviderRequest) (*ProviderResult, error)
	Start(context.Context, string) (*ProviderResult, error)
	Stop(context.Context, string, bool) (*ProviderResult, error)
	Delete(context.Context, string) error
	Inspect(context.Context, string) (*ProviderResult, error)
	Logs(context.Context, string, int) ([]*iapiserver.InfraRuntimeLogEntry, error)
	Health(context.Context, string) (*iapiserver.InfraRuntimeHealthResult, error)
}
