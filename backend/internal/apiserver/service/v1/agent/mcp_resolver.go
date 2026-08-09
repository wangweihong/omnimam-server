package agent

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentmcp"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// MCPResolverDependencies are the trusted inputs needed to resolve a pinned
// Binding revision. JWTSecret and resolved credentials never leave memory.
type MCPResolverDependencies struct {
	Store              store.AgentStore
	JWTSecret          []byte
	PlatformMCPBaseURL string
	Credentials        MCPCredentialResolver
}

// MCPCredentialResolver resolves an opaque credential reference at the trusted
// Agent/Infrastructure boundary.
type MCPCredentialResolver interface {
	ResolveMCPCredential(context.Context, string) (string, error)
}

type MCPResolver struct {
	store            store.AgentStore
	jwtSecret        []byte
	platformEndpoint string
	credentials      MCPCredentialResolver
}

func NewMCPResolver(deps MCPResolverDependencies) (*MCPResolver, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("agent store is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(deps.PlatformMCPBaseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	endpoint := baseURL + "/mcp"
	if !approvedRemoteEndpoint(endpoint) {
		return nil, fmt.Errorf("platform MCP base URL is invalid")
	}
	return &MCPResolver{
		store: deps.Store, jwtSecret: append([]byte(nil), deps.JWTSecret...),
		platformEndpoint: endpoint, credentials: deps.Credentials,
	}, nil
}

// ResolveMCPBinding resolves only immutable revisions authorized by an active
// Runtime Grant. The returned secret must not be persisted or logged.
func (r *MCPResolver) ResolveMCPBinding(ctx context.Context, authorizationRef, ownerReference, reference string) (*agentmcp.Resolution, error) {
	grant, err := r.store.GetAgentRuntimeGrantByRequestID(ctx, authorizationRef)
	if err != nil || grant == nil || grant.RequestID != authorizationRef || grant.RuntimeBindingID != ownerReference ||
		grant.Status != iapiserver.AgentRuntimeGrantStatusActive || grant.RevokedAt != nil || !grant.ExpiresAt.Time.After(time.Now()) {
		return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "runtime grant is invalid or expired")
	}
	bindingID, revision, revisionRef, err := parseMCPRevisionReference(reference)
	if err != nil {
		return nil, err
	}
	if !containsString(grant.BindingRevisions, revisionRef) {
		return nil, errors.NewStatus(code.ErrAgentMCPBindingRevisionUnavailable, "MCP binding revision is not authorized")
	}
	snapshot, err := r.store.GetAgentMCPBindingRevision(ctx, bindingID, grant.AgentID, revision)
	if err != nil || snapshot == nil || !snapshot.Enabled {
		return nil, errors.NewStatus(code.ErrAgentMCPBindingRevisionUnavailable, "MCP binding revision is unavailable")
	}
	configuration, configurationErr := agentmcp.ParseConfiguration(snapshot.Configuration)
	if configurationErr != nil {
		return nil, errors.NewStatus(code.ErrAgentMCPBindingInvalid, "MCP configuration is invalid")
	}
	result := &agentmcp.Resolution{
		ServerKey: snapshot.BindingID, ServerType: snapshot.ServerType,
		AllowedTools: append([]string(nil), snapshot.AllowedTools...), Configuration: configuration,
	}
	switch snapshot.ServerType {
	case iapiserver.AgentMCPServerTypePlatform:
		if snapshot.EndpointRef != iapiserver.AgentMCPPlatformEndpointRefDefault || grant.StudioApplicationID == "" || grant.AgentGeneration == nil || *grant.AgentGeneration < 1 {
			return nil, errors.NewStatus(code.ErrAgentMCPBindingInvalid, "platform MCP scope is incomplete")
		}
		result.Endpoint = r.platformEndpoint
		result.Credential, err = middleware.IssueAgentWorkloadAccessToken(
			r.jwtSecret, grant.AgentID, *grant.AgentGeneration, grant.StudioApplicationID,
			grant.RuntimeBindingID, grant.RequestID, time.Until(grant.ExpiresAt.Time),
		)
		if err != nil {
			return nil, errors.NewStatus(code.ErrAgentRuntimeOperationFailed, "platform MCP workload credential is unavailable")
		}
	case iapiserver.AgentMCPServerTypeRemote:
		return nil, errors.NewStatus(code.ErrAgentMCPBindingInvalid, "remote MCP endpoint authorization is unavailable")
	case iapiserver.AgentMCPServerTypeRuntimeLocal:
		return nil, errors.NewStatus(code.ErrAgentMCPBindingInvalid, "runtime-local MCP profile resolution is unavailable")
	default:
		return nil, errors.NewStatus(code.ErrAgentMCPBindingInvalid, "MCP server type is invalid")
	}
	return result, nil
}

func parseMCPRevisionReference(reference string) (string, int64, string, error) {
	const prefix = "mcp-binding-revision://"
	if !strings.HasPrefix(reference, prefix) {
		return "", 0, "", errors.NewStatus(code.ErrAgentMCPBindingRevisionUnavailable, "MCP binding revision is unavailable")
	}
	parts := strings.Split(strings.TrimPrefix(reference, prefix), "/")
	if len(parts) != 2 || parts[0] == "" {
		return "", 0, "", errors.NewStatus(code.ErrAgentMCPBindingRevisionUnavailable, "MCP binding revision is unavailable")
	}
	revision, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || revision < 1 {
		return "", 0, "", errors.NewStatus(code.ErrAgentMCPBindingRevisionUnavailable, "MCP binding revision is invalid")
	}
	return parts[0], revision, parts[0] + "/" + parts[1], nil
}

func approvedRemoteEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

var _ agentmcp.Resolver = (*MCPResolver)(nil)
