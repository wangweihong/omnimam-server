package providers

import (
	"context"
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

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
func (p UnavailableProvider) Health(context.Context, string) (*iapiserver.InfraRuntimeHealthResult, error) {
	return nil, p.err()
}

var _ RuntimeProvider = UnavailableProvider{}
