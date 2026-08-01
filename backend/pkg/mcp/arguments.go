package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

type CapabilitiesListArguments struct {
	Domain string `json:"domain,omitempty"`
	Query  string `json:"query,omitempty"`
	Status string `json:"status,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type CapabilitiesGetArguments struct {
	CapabilityID string `json:"capability_id"`
}

type ApplicationsListArguments struct {
	Query        string `json:"query,omitempty"`
	CapabilityID string `json:"capability_id,omitempty"`
	Cursor       string `json:"cursor,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type ApplicationsGetArguments struct {
	ApplicationID string `json:"application_id"`
}

type ApplicationsRunArguments struct {
	ApplicationID        string         `json:"application_id"`
	ApplicationVersionID string         `json:"application_version_id,omitempty"`
	Input                map[string]any `json:"input"`
	IdempotencyKey       string         `json:"idempotency_key"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type ApplicationRunsGetArguments struct {
	ApplicationRunID string `json:"application_run_id"`
}

type ApplicationRunsCancelArguments struct {
	ApplicationRunID string `json:"application_run_id"`
	Reason           string `json:"reason,omitempty"`
}

type AssetsSearchArguments struct {
	Query      string   `json:"query,omitempty"`
	MediaTypes []string `json:"media_types,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	Cursor     string   `json:"cursor,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

type AssetsGetArguments struct {
	AssetID                string `json:"asset_id"`
	IncludeRepresentations *bool  `json:"include_representations,omitempty"`
}

type Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type AssetsPrepareUploadArguments struct {
	Filename       string         `json:"filename"`
	MediaType      string         `json:"media_type"`
	SizeBytes      int64          `json:"size_bytes"`
	Checksum       Checksum       `json:"checksum"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type AssetsCompleteUploadArguments struct {
	UploadID       string `json:"upload_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// DecodeToolArguments strictly decodes and validates one of the eleven released Tool inputs.
func DecodeToolArguments(name string, raw json.RawMessage) (any, *RPCError) {
	var target any
	switch name {
	case ToolCapabilitiesList:
		target = &CapabilitiesListArguments{}
	case ToolCapabilitiesGet:
		target = &CapabilitiesGetArguments{}
	case ToolApplicationsList:
		target = &ApplicationsListArguments{}
	case ToolApplicationsGet:
		target = &ApplicationsGetArguments{}
	case ToolApplicationsRun:
		target = &ApplicationsRunArguments{}
	case ToolApplicationRunsGet:
		target = &ApplicationRunsGetArguments{}
	case ToolApplicationRunsCancel:
		target = &ApplicationRunsCancelArguments{}
	case ToolAssetsSearch:
		target = &AssetsSearchArguments{}
	case ToolAssetsGet:
		target = &AssetsGetArguments{}
	case ToolAssetsPrepareUpload:
		target = &AssetsPrepareUploadArguments{}
	case ToolAssetsCompleteUpload:
		target = &AssetsCompleteUploadArguments{}
	default:
		return nil, &RPCError{Code: JSONRPCMethodNotFound, Message: "MCP tool does not exist or is not visible"}
	}
	if err := decodeStrict(raw, target); err != nil {
		return nil, invalidToolArguments(err)
	}
	if err := validateToolArguments(target); err != nil {
		return nil, invalidToolArguments(err)
	}
	return target, nil
}

func validateToolArguments(arguments any) error {
	switch value := arguments.(type) {
	case *CapabilitiesListArguments:
		value.Status = defaultString(value.Status, "available")
		value.Limit = defaultLimit(value.Limit, 50)
		if value.Status != "available" || len(value.Domain) > 64 {
			return fmt.Errorf("status or domain is invalid")
		}
		return validateCommonList(value.Query, value.Cursor, value.Limit, 255)
	case *CapabilitiesGetArguments:
		return requiredID("capability_id", value.CapabilityID)
	case *ApplicationsListArguments:
		value.Limit = defaultLimit(value.Limit, 50)
		if err := validateCommonList(value.Query, value.Cursor, value.Limit, 255); err != nil {
			return err
		}
		if len(value.CapabilityID) > 255 {
			return fmt.Errorf("capability_id exceeds 255 characters")
		}
		return nil
	case *ApplicationsGetArguments:
		return requiredID("application_id", value.ApplicationID)
	case *ApplicationsRunArguments:
		if err := requiredID("application_id", value.ApplicationID); err != nil {
			return err
		}
		if value.ApplicationVersionID != "" && len(value.ApplicationVersionID) > 255 {
			return fmt.Errorf("application_version_id exceeds 255 characters")
		}
		if value.Input == nil || len(value.Input) > 256 {
			return fmt.Errorf("input must be an object with at most 256 properties")
		}
		if strings.TrimSpace(value.IdempotencyKey) == "" || len(value.IdempotencyKey) > 128 {
			return fmt.Errorf("idempotency_key is required and must not exceed 128 characters")
		}
		if len(value.Metadata) > 64 {
			return fmt.Errorf("metadata exceeds 64 properties")
		}
		for key, item := range value.Metadata {
			if key == "" || !isScalar(item) {
				return fmt.Errorf("metadata values must be scalar")
			}
		}
		return nil
	case *ApplicationRunsGetArguments:
		return requiredID("application_run_id", value.ApplicationRunID)
	case *ApplicationRunsCancelArguments:
		if err := requiredID("application_run_id", value.ApplicationRunID); err != nil {
			return err
		}
		if len(value.Reason) > 500 {
			return fmt.Errorf("reason exceeds 500 characters")
		}
		return nil
	case *AssetsSearchArguments:
		value.Sort = defaultString(value.Sort, "created_at_desc")
		value.Limit = defaultLimit(value.Limit, 20)
		if err := validateCommonList(value.Query, value.Cursor, value.Limit, 500); err != nil {
			return err
		}
		if len(value.MediaTypes) > 10 || hasDuplicates(value.MediaTypes) || len(value.Tags) > 50 || hasDuplicates(value.Tags) {
			return fmt.Errorf("media_types or tags exceed released limits or contain duplicates")
		}
		for _, mediaType := range value.MediaTypes {
			if !isMediaType(mediaType) {
				return fmt.Errorf("unsupported media type")
			}
		}
		for _, tag := range value.Tags {
			if strings.TrimSpace(tag) == "" || len(tag) > 128 {
				return fmt.Errorf("tag is empty or exceeds 128 characters")
			}
		}
		if !contains([]string{"created_at_asc", "created_at_desc", "updated_at_asc", "updated_at_desc"}, value.Sort) {
			return fmt.Errorf("unsupported sort")
		}
		return nil
	case *AssetsGetArguments:
		return requiredID("asset_id", value.AssetID)
	case *AssetsPrepareUploadArguments:
		if strings.TrimSpace(value.Filename) == "" || len(value.Filename) > 255 {
			return fmt.Errorf("filename is required and must not exceed 255 characters")
		}
		if strings.TrimSpace(value.MediaType) == "" || len(value.MediaType) > 255 || value.SizeBytes < 1 {
			return fmt.Errorf("media_type and positive size_bytes are required")
		}
		if value.Checksum.Algorithm != "sha256" || !sha256Pattern.MatchString(value.Checksum.Value) {
			return fmt.Errorf("checksum must contain a SHA-256 value")
		}
		if strings.TrimSpace(value.IdempotencyKey) == "" || len(value.IdempotencyKey) > 128 || len(value.Metadata) > 64 {
			return fmt.Errorf("idempotency_key or metadata violates released limits")
		}
		for _, item := range value.Metadata {
			if !isScalar(item) {
				return fmt.Errorf("metadata values must be scalar")
			}
		}
		return nil
	case *AssetsCompleteUploadArguments:
		if err := requiredID("upload_id", value.UploadID); err != nil {
			return err
		}
		if strings.TrimSpace(value.IdempotencyKey) == "" || len(value.IdempotencyKey) > 128 {
			return fmt.Errorf("idempotency_key is required and must not exceed 128 characters")
		}
		return nil
	default:
		return fmt.Errorf("unsupported tool arguments")
	}
}

func invalidToolArguments(err error) *RPCError {
	detail := "tool arguments do not conform to the published input schema"
	if err != nil {
		detail = err.Error()
	}
	return &RPCError{Code: JSONRPCInvalidParams, Message: "invalid MCP tool arguments", Data: &BusinessError{
		Code: "ERR_MCP_TOOL_ARGUMENT_INVALID", Value: 190401, Message: "Tool arguments do not conform to the published input schema.",
		Messages: map[string]string{"zh-CN": "Tool 参数不符合已发布输入 Schema。", "en-US": "Tool arguments do not conform to the published input schema."},
		Detail:   detail, Retryable: false, SourceDomain: "mcp",
	}}
}

func validateCommonList(query, cursor string, limit, maxQuery int) error {
	if len(query) > maxQuery || len(cursor) > 512 {
		return fmt.Errorf("query or cursor exceeds released limits")
	}
	if limit < 1 || limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	return nil
}

func requiredID(name, value string) error {
	if strings.TrimSpace(value) == "" || len(value) > 255 {
		return fmt.Errorf("%s is required and must not exceed 255 characters", name)
	}
	return nil
}

func defaultLimit(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, float64, bool:
		return true
	default:
		return false
	}
}

func hasDuplicates(items []string) bool {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			return true
		}
		seen[item] = struct{}{}
	}
	return false
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func isMediaType(value string) bool {
	return contains([]string{"image", "video", "audio", "text", "document", "model_3d", "prompt", "prompt_template", "pdf", "other"}, value)
}
