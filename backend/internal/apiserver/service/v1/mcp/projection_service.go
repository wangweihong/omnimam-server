package mcp

import (
	"context"
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func (s *Service) applicationRunProjection(ctx context.Context, id string) (ApplicationRunProjection, error) {
	run, err := s.applications.GetApplicationRun(ctx, id)
	if err != nil {
		return ApplicationRunProjection{}, err
	}
	correlateAudit(ctx, run.ID, "")
	application, err := s.applications.GetApplication(ctx, run.ApplicationID)
	if err != nil {
		return ApplicationRunProjection{}, err
	}
	version, err := s.applications.GetApplicationVersion(ctx, run.ApplicationVersionID)
	if err != nil {
		return ApplicationRunProjection{}, err
	}
	projection := ApplicationRunProjection{
		ApplicationRunID: run.ID, Application: applicationSummary(application),
		ApplicationVersion: applicationVersionSummary(version), Status: "queued", Progress: 0,
		Outputs: make([]ArtifactProjection, 0, len(run.Artifacts)), CreatedAt: run.CreatedAt,
		ResourceVersion: run.ResourceVersion,
	}
	for _, artifact := range run.Artifacts {
		projection.Outputs = append(projection.Outputs, artifactProjection(artifact))
	}
	if run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
		return projection, nil
	}
	task, err := s.tasks.GetAtomicTask(ctx, *run.AtomicTaskID)
	if err != nil {
		return ApplicationRunProjection{}, err
	}
	projection.AtomicTask = &AtomicTaskProjection{
		AtomicTaskID: task.ID, Status: task.Status, Progress: normalizeProgress(task.Progress),
		ResourceVersion: task.ResourceVersion,
	}
	projection.Status = runStatus(task.Status)
	projection.Progress = normalizeProgress(task.Progress)
	if !task.StartedAt.IsZero() {
		projection.StartedAt = copyTime(task.StartedAt)
	}
	if !task.CompletedAt.IsZero() {
		projection.CompletedAt = copyTime(task.CompletedAt)
	} else if task.Status == iapiserver.AtomicTaskStatusCanceled && !task.CanceledAt.IsZero() {
		projection.CompletedAt = copyTime(task.CanceledAt)
	}
	return projection, nil
}

func (s *Service) assetDetail(ctx context.Context, id string, includeRepresentations bool) (AssetDetailProjection, error) {
	detail, err := s.assets.GetAsset(ctx, id)
	if err != nil {
		return AssetDetailProjection{}, err
	}
	if detail == nil || detail.Asset == nil || detail.CurrentVersion == nil {
		return AssetDetailProjection{}, internalMCPError(code.ErrMCPResourceNotVisible, "ERR_MCP_RESOURCE_NOT_VISIBLE", false, "Asset has no visible current version")
	}
	result := AssetDetailProjection{
		AssetSummary: assetSummary(detail.Asset), CurrentVersion: assetVersionProjection(detail.CurrentVersion),
		Representations: []RepresentationProjection{}, Tags: append([]string(nil), detail.Asset.Tags...),
	}
	if includeRepresentations {
		for _, representation := range detail.Representations {
			if representation == nil || representation.Status != "ready" {
				continue
			}
			result.Representations = append(result.Representations, representationProjection(detail.Asset.ID, representation))
		}
	}
	return result, nil
}

func artifactResourceProjection(artifact *iapiserver.Artifact) ArtifactProjection {
	return ArtifactProjection{
		ArtifactID: artifact.ID, OutputKey: artifact.OutputKey, Sequence: artifact.Sequence,
		MediaType: artifact.MediaType, ProcessingStatus: artifact.ProcessingStatus,
		RegistrationStatus: artifact.RegistrationStatus, AssetID: artifact.AssetID,
		URI: "omnimam://artifacts/" + artifact.ID,
	}
}

func representationResourceProjection(
	assetID string,
	representation *iapiserver.AssetRepresentation,
	access *iapiserver.RepresentationAccess,
) RepresentationProjection {
	result := representationProjection(assetID, representation)
	if access != nil {
		result.AccessURL = access.AccessURL
	}
	return result
}

func normalizeProgress(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func copyTime(value imachinery.Time) *imachinery.Time {
	copyValue := value
	return &copyValue
}

func (p ApplicationRunProjection) String() string {
	return fmt.Sprintf("ApplicationRun %s is %s.", p.ApplicationRunID, p.Status)
}
