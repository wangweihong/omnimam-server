package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func (s *Service) readResource(ctx context.Context, rawURI string) (any, error) {
	resource, err := parseResourceURI(rawURI)
	if err != nil {
		return nil, internalMCPError(code.ErrMCPResourceURIInvalid, "ERR_MCP_RESOURCE_URI_INVALID", false, err.Error())
	}
	var projection any
	switch resource.kind {
	case "capabilities":
		if err := s.require(ctx, "aiapp.provider_capability.read"); err != nil {
			return nil, resourceNotVisible()
		}
		execution, err := s.getCapability(ctx, &protocol.CapabilitiesGetArguments{CapabilityID: resource.id})
		if err != nil {
			return nil, resourceNotVisible()
		}
		projection = execution.Structured
	case "applications":
		if err := s.require(ctx, "aiapp.application.read"); err != nil {
			return nil, resourceNotVisible()
		}
		detail, err := s.applicationDetail(ctx, resource.id)
		if err != nil {
			return nil, resourceNotVisible()
		}
		projection = detail
	case "application-runs":
		if err := s.require(ctx, "aiapp.application.run"); err != nil {
			return nil, resourceNotVisible()
		}
		run, err := s.applicationRunProjection(ctx, resource.id)
		if err != nil {
			return nil, resourceNotVisible()
		}
		projection = run
	case "assets":
		if err := s.require(ctx, "asset.read"); err != nil {
			return nil, resourceNotVisible()
		}
		if resource.representationID == "" {
			asset, err := s.assetDetail(ctx, resource.id, true)
			if err != nil {
				return nil, resourceNotVisible()
			}
			projection = asset
			break
		}
		for _, permission := range []string{"asset.content.read", "asset.representation.read"} {
			if err := s.require(ctx, permission); err != nil {
				return nil, resourceNotVisible()
			}
		}
		asset, err := s.assetDetail(ctx, resource.id, true)
		if err != nil {
			return nil, resourceNotVisible()
		}
		var found bool
		for _, item := range asset.Representations {
			if item.RepresentationID == resource.representationID {
				found = true
				break
			}
		}
		if !found {
			return nil, resourceNotVisible()
		}
		representation, err := s.assets.GetRepresentation(ctx, resource.representationID)
		if err != nil {
			return nil, resourceNotVisible()
		}
		access, err := s.assets.RepresentationAccess(ctx, representation.ID, "inline")
		if err != nil {
			return nil, resourceNotVisible()
		}
		access.AccessURL = s.publicURL(access.AccessURL)
		projection = representationResourceProjection(resource.id, representation, access)
	case "artifacts":
		if err := s.require(ctx, "asset.artifact.read"); err != nil {
			return nil, resourceNotVisible()
		}
		artifact, err := s.assets.GetArtifact(ctx, resource.id)
		if err != nil {
			return nil, resourceNotVisible()
		}
		projection = artifactResourceProjection(artifact)
	default:
		return nil, internalMCPError(code.ErrMCPResourceTypeUnsupported, "ERR_MCP_RESOURCE_TYPE_UNSUPPORTED", false, "unsupported Resource type")
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		return nil, internalMCPError(code.ErrMCPToolResultInvalid, "ERR_MCP_TOOL_RESULT_INVALID", true, "Resource projection encoding failed")
	}
	return protocol.ResourceReadResult{
		ResultType: "complete",
		Contents:   []protocol.TextResourceContents{{URI: rawURI, MIMEType: "application/json", Text: string(raw)}},
		CacheScope: "private", TTLMS: s.config.ResourceTTL.Milliseconds(),
	}, nil
}

type resourceURI struct {
	kind             string
	id               string
	representationID string
}

func parseResourceURI(raw string) (resourceURI, error) {
	if len(raw) > 2048 {
		return resourceURI{}, fmt.Errorf("Resource URI exceeds 2048 characters")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "omnimam" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return resourceURI{}, fmt.Errorf("Resource URI must use the omnimam scheme without query or fragment")
	}
	if parsed.Host == "" || strings.Contains(parsed.Path, "..") || strings.Contains(parsed.EscapedPath(), "%2f") || strings.Contains(parsed.EscapedPath(), "%2F") {
		return resourceURI{}, fmt.Errorf("Resource URI host or path is invalid")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 1 && segments[0] != "" {
		switch parsed.Host {
		case "capabilities", "applications", "application-runs", "assets", "artifacts":
			return resourceURI{kind: parsed.Host, id: segments[0]}, nil
		default:
			return resourceURI{}, fmt.Errorf("Resource type is unsupported")
		}
	}
	if parsed.Host == "assets" && len(segments) == 3 && segments[0] != "" && segments[1] == "representations" && segments[2] != "" {
		return resourceURI{kind: parsed.Host, id: segments[0], representationID: segments[2]}, nil
	}
	return resourceURI{}, fmt.Errorf("Resource URI does not match a released template")
}

func resourceNotVisible() error {
	return internalMCPError(code.ErrMCPResourceNotVisible, "ERR_MCP_RESOURCE_NOT_VISIBLE", false, "Resource does not exist or is not visible")
}
