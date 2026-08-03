package infrastructure

import (
	"context"
	"fmt"

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
	ArtifactDigest     string
	Outputs            []*iapiserver.InfraRuntimeOutput
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

type UnavailableProvider struct{ Reason string }

func (p UnavailableProvider) err() error {
	if p.Reason == "" {
		return fmt.Errorf("runtime provider is unavailable")
	}
	return fmt.Errorf("runtime provider is unavailable: %s", p.Reason)
}
func (p UnavailableProvider) Info(context.Context) (*iapiserver.InfraNode, error) {
	return nil, p.err()
}
func (p UnavailableProvider) Ensure(context.Context, ProviderRequest) (*ProviderResult, error) {
	return nil, p.err()
}
func (p UnavailableProvider) Start(context.Context, string) (*ProviderResult, error) {
	return nil, p.err()
}
func (p UnavailableProvider) Stop(context.Context, string, bool) (*ProviderResult, error) {
	return nil, p.err()
}
func (p UnavailableProvider) Delete(context.Context, string) error { return p.err() }
func (p UnavailableProvider) Inspect(context.Context, string) (*ProviderResult, error) {
	return nil, p.err()
}
func (p UnavailableProvider) Logs(context.Context, string, int) ([]*iapiserver.InfraRuntimeLogEntry, error) {
	return nil, p.err()
}

var _ RuntimeProvider = UnavailableProvider{}
