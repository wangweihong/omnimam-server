package mcp

import (
	"maps"
	"strconv"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

type CapabilityListResult struct {
	Total      int                 `json:"total"`
	Items      []CapabilitySummary `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type CapabilitySummary struct {
	CapabilityID         string                         `json:"capability_id"`
	Name                 string                         `json:"name"`
	Description          string                         `json:"description"`
	Status               string                         `json:"status"`
	SupportsDirectInvoke bool                           `json:"supports_direct_invoke"`
	SchemaURI            string                         `json:"schema_uri"`
	Applications         []ApplicationNavigationSummary `json:"applications"`
}

type CapabilityDetail struct {
	CapabilitySummary
	InputSchema  map[string]any `json:"input_schema"`
	OutputSchema map[string]any `json:"output_schema"`
}

type ApplicationNavigationSummary struct {
	ApplicationID      string `json:"application_id"`
	Name               string `json:"name"`
	PublishedVersionID string `json:"published_version_id"`
	RunEnabled         bool   `json:"run_enabled"`
}

type ApplicationListResult struct {
	Total      int                  `json:"total"`
	Items      []ApplicationSummary `json:"items"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

type ApplicationSummary struct {
	ApplicationID      string `json:"application_id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	PublishedVersionID string `json:"published_version_id"`
	RunEnabled         bool   `json:"run_enabled"`
	Availability       string `json:"availability,omitempty"`
	SchemaURI          string `json:"schema_uri"`
}

type ApplicationDetail struct {
	ApplicationSummary
	PublishedVersion ApplicationVersionSummary `json:"published_version"`
	InputSchema      map[string]any            `json:"input_schema"`
	OutputSchema     map[string]any            `json:"output_schema"`
}

type ApplicationVersionSummary struct {
	ApplicationVersionID string `json:"application_version_id"`
	Version              int    `json:"version"`
}

type ApplicationRunAccepted struct {
	ApplicationRunID     string `json:"application_run_id"`
	ApplicationID        string `json:"application_id"`
	ApplicationVersionID string `json:"application_version_id"`
	Status               string `json:"status"`
	StatusURI            string `json:"status_uri"`
	MCPTaskID            string `json:"mcp_task_id,omitempty"`
}

type ApplicationRunProjection struct {
	ApplicationRunID   string                    `json:"application_run_id"`
	Application        ApplicationSummary        `json:"application"`
	ApplicationVersion ApplicationVersionSummary `json:"application_version"`
	AtomicTask         *AtomicTaskProjection     `json:"atomic_task,omitempty"`
	Status             string                    `json:"status"`
	Progress           float64                   `json:"progress"`
	Outputs            []ArtifactProjection      `json:"outputs"`
	Error              *protocol.BusinessError   `json:"error,omitempty"`
	CreatedAt          imachinery.Time           `json:"created_at"`
	StartedAt          *imachinery.Time          `json:"started_at,omitempty"`
	CompletedAt        *imachinery.Time          `json:"completed_at,omitempty"`
	ResourceVersion    int64                     `json:"resource_version"`
}

type AtomicTaskProjection struct {
	AtomicTaskID    string  `json:"atomic_task_id"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	Phase           string  `json:"phase,omitempty"`
	ResourceVersion int64   `json:"resource_version"`
}

type ApplicationRunCancelResult struct {
	ApplicationRunID string `json:"application_run_id"`
	Accepted         bool   `json:"accepted"`
	Status           string `json:"status"`
}

type ArtifactProjection struct {
	ArtifactID         string `json:"artifact_id"`
	OutputKey          string `json:"output_key"`
	Sequence           int    `json:"sequence"`
	MediaType          string `json:"media_type"`
	ProcessingStatus   string `json:"processing_status"`
	RegistrationStatus string `json:"registration_status"`
	AssetID            string `json:"asset_id,omitempty"`
	URI                string `json:"uri"`
}

type AssetListResult struct {
	Total      int            `json:"total"`
	Items      []AssetSummary `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type AssetSummary struct {
	AssetID         string          `json:"asset_id"`
	DisplayName     string          `json:"display_name"`
	MediaType       string          `json:"media_type"`
	Format          string          `json:"format,omitempty"`
	SizeBytes       int64           `json:"size_bytes,omitempty"`
	Width           int             `json:"width,omitempty"`
	Height          int             `json:"height,omitempty"`
	DurationSeconds float64         `json:"duration_seconds,omitempty"`
	Status          string          `json:"status"`
	ThumbnailStatus string          `json:"thumbnail_status"`
	URI             string          `json:"uri"`
	ThumbnailURL    string          `json:"thumbnail_url,omitempty"`
	CreatedAt       imachinery.Time `json:"created_at"`
}

type AssetDetailProjection struct {
	AssetSummary
	CurrentVersion  AssetVersionProjection     `json:"current_version"`
	Representations []RepresentationProjection `json:"representations"`
	Tags            []string                   `json:"tags"`
}

type AssetVersionProjection struct {
	AssetVersionID string `json:"asset_version_id"`
	Version        int    `json:"version"`
	Status         string `json:"status"`
}

type RepresentationProjection struct {
	RepresentationID string `json:"representation_id"`
	Type             string `json:"type"`
	MediaType        string `json:"media_type"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	URI              string `json:"uri"`
	AccessURL        string `json:"access_url,omitempty"`
}

type AssetUploadPrepared struct {
	UploadID              string            `json:"upload_id"`
	Method                string            `json:"method"`
	ContentUploadURL      string            `json:"content_upload_url"`
	RequiredHeaders       map[string]string `json:"required_headers"`
	AuthorizationRequired bool              `json:"authorization_required"`
	ExpiresAt             imachinery.Time   `json:"expires_at"`
}

type AssetUploadCompleted struct {
	UploadID string       `json:"upload_id"`
	Asset    AssetSummary `json:"asset"`
	Status   string       `json:"status"`
	URI      string       `json:"uri"`
}

func applicationSummary(application *iapiserver.Application) ApplicationSummary {
	versionID := ""
	if application != nil && application.CurrentVersionID != nil {
		versionID = *application.CurrentVersionID
	}
	availability := "available"
	if application == nil || !application.RunEnabled || versionID == "" {
		availability = "unavailable"
	}
	return ApplicationSummary{
		ApplicationID: application.ID, Name: application.Name, Description: application.Description,
		PublishedVersionID: versionID, RunEnabled: application.RunEnabled,
		Availability: availability, SchemaURI: "omnimam://applications/" + application.ID,
	}
}

func applicationNavigation(application *iapiserver.Application) ApplicationNavigationSummary {
	summary := applicationSummary(application)
	return ApplicationNavigationSummary{
		ApplicationID: summary.ApplicationID, Name: summary.Name,
		PublishedVersionID: summary.PublishedVersionID, RunEnabled: summary.RunEnabled,
	}
}

func applicationVersionSummary(version *iapiserver.ApplicationVersion) ApplicationVersionSummary {
	return ApplicationVersionSummary{ApplicationVersionID: version.ID, Version: versionNumber(version)}
}

func versionNumber(version *iapiserver.ApplicationVersion) int {
	if version == nil {
		return 1
	}
	major, _, _ := strings.Cut(version.SemanticVersion, ".")
	value, err := strconv.Atoi(major)
	if err == nil && value > 0 {
		return value
	}
	return 1
}

func publishedSchema(input map[string]any) map[string]any {
	result := maps.Clone(input)
	if result == nil {
		result = map[string]any{}
	}
	result["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	if _, ok := result["type"]; !ok {
		result["type"] = "object"
	}
	return result
}

func runStatus(taskStatus string) string {
	switch taskStatus {
	case iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusRetrying:
		return "running"
	case iapiserver.AtomicTaskStatusCancelRequested:
		return "cancel_requested"
	case iapiserver.AtomicTaskStatusSuccess:
		return "succeeded"
	case iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusTimeout, iapiserver.AtomicTaskStatusSkipped:
		return "failed"
	case iapiserver.AtomicTaskStatusCanceled:
		//nolint:misspell // MCP 2026-07-28 fixes this status to the British spelling.
		return "cancelled"
	default:
		return "queued"
	}
}

func mcpTaskStatus(taskStatus string) string {
	switch taskStatus {
	case iapiserver.AtomicTaskStatusSuccess:
		return "completed"
	case iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusTimeout, iapiserver.AtomicTaskStatusSkipped:
		return "failed"
	case iapiserver.AtomicTaskStatusCanceled:
		//nolint:misspell // MCP 2026-07-28 fixes this status to the British spelling.
		return "cancelled"
	default:
		return "working"
	}
}

func artifactProjection(item *iapiserver.ApplicationArtifactRef) ArtifactProjection {
	assetID := ""
	if item.AssetID != nil {
		assetID = *item.AssetID
	}
	return ArtifactProjection{
		ArtifactID: item.ArtifactID, OutputKey: item.OutputKey, Sequence: item.Sequence,
		MediaType: item.MediaType, ProcessingStatus: item.ArtifactProcessingStatus,
		RegistrationStatus: item.ArtifactRegistrationStatus, AssetID: assetID,
		URI: "omnimam://artifacts/" + item.ArtifactID,
	}
}

func assetSummary(item *iapiserver.UserAsset) AssetSummary {
	thumbnailStatus := item.ThumbnailStatus
	if thumbnailStatus == "" {
		thumbnailStatus = "none"
	}
	return AssetSummary{
		AssetID: item.ID, DisplayName: item.DisplayName, MediaType: item.MediaType, Format: item.Format,
		SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height, DurationSeconds: item.DurationSeconds,
		Status: item.Status, ThumbnailStatus: thumbnailStatus, URI: "omnimam://assets/" + item.ID,
		CreatedAt: item.CreatedAt,
	}
}

func assetVersionProjection(version *iapiserver.AssetVersion) AssetVersionProjection {
	if version == nil {
		return AssetVersionProjection{}
	}
	status := version.Status
	if status == iapiserver.AssetVersionStatusUploading {
		status = iapiserver.AssetVersionStatusProcessing
	}
	return AssetVersionProjection{AssetVersionID: version.ID, Version: version.VersionNo, Status: status}
}

func representationProjection(assetID string, item *iapiserver.AssetRepresentation) RepresentationProjection {
	representationType := item.RepresentationType
	if representationType == iapiserver.AssetRepresentationCanonical {
		representationType = iapiserver.AssetRepresentationOriginal
	}
	return RepresentationProjection{
		RepresentationID: item.ID, Type: representationType, MediaType: item.MIMEType,
		URI: "omnimam://assets/" + assetID + "/representations/" + item.ID,
	}
}
