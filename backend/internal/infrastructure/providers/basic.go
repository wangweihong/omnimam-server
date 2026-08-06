package providers

import (
	"context"
	"io"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type ProviderRequest struct {
	RuntimeID string
	Profile   *iapiserver.InfraRuntimeProfile
	Request   *iapiserver.InfraCreateRuntimeRequest
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
}
