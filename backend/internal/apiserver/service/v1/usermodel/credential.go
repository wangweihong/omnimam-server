package usermodel

import (
	"context"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const defaultCredentialHandleTTL = 30 * time.Second

type credentialHandle struct {
	providerType   string
	authType       string
	authentication map[string]any
	expiresAt      time.Time
}

// CredentialBroker issues short-lived opaque handles and resolves them only for the bound provider and auth type.
type CredentialBroker struct {
	mu      sync.Mutex
	ttl     time.Duration
	handles map[string]credentialHandle
}

func NewCredentialBroker(ttl time.Duration) *CredentialBroker {
	if ttl <= 0 {
		ttl = defaultCredentialHandleTTL
	}
	return &CredentialBroker{ttl: ttl, handles: make(map[string]credentialHandle)}
}

func (b *CredentialBroker) Issue(providerType, authType, credentialRef string) (string, error) {
	providerType = strings.TrimSpace(providerType)
	authType = strings.TrimSpace(authType)
	if providerType == "" || authType == "" {
		return "", errors.NewStatus(code.ErrModelProviderTestFailed, "provider credential scope is invalid")
	}
	if authType == iapiserver.EngineAuthNone {
		return "", nil
	}
	if authType != iapiserver.EngineAuthAPIKey {
		return "", errors.NewStatus(code.ErrModelProviderTestFailed, "provider authentication type is unsupported")
	}

	now := time.Now()
	handle := uuid.NewString()
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, entry := range b.handles {
		if !entry.expiresAt.After(now) {
			delete(b.handles, id)
		}
	}
	b.handles[handle] = credentialHandle{
		providerType:   providerType,
		authType:       authType,
		authentication: map[string]any{"api_key": credentialRef},
		expiresAt:      now.Add(b.ttl),
	}
	return handle, nil
}

func (b *CredentialBroker) ResolveCredential(_ context.Context, request modelgateway.CredentialResolveRequest) (*modelgateway.ResolvedCredential, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.handles[request.Handle]
	if !ok || !entry.expiresAt.After(time.Now()) {
		delete(b.handles, request.Handle)
		return nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "credential handle is invalid or expired")
	}
	if entry.providerType != request.ProviderType || entry.authType != request.AuthenticationType {
		return nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "credential handle scope does not match provider")
	}
	return &modelgateway.ResolvedCredential{Authentication: maps.Clone(entry.authentication)}, nil
}

var _ modelgateway.CredentialResolver = (*CredentialBroker)(nil)
