package iapiserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// +k8s:deepcopy-gen=true
type StudioApplicationListRequest struct {
	imachinery.BasicQueryParam
	Status      string `form:"status" binding:"omitempty,oneof=CREATING READY ARCHIVED ERROR"`
	OwnerUserID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationCreateRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=200"`
	Description string `json:"description,omitempty" binding:"omitempty,max=2000"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationUpdateRequest struct {
	Name            *string `json:"name,omitempty" binding:"omitempty,max=200"`
	Description     *string `json:"description,omitempty" binding:"omitempty,max=2000"`
	ResourceVersion int64   `json:"resource_version" binding:"required,min=0"`
}

func (r *StudioApplicationUpdateRequest) Validate() error {
	if r.Name == nil && r.Description == nil {
		return fmt.Errorf("studio application update has no fields")
	}
	return nil
}

// +k8s:deepcopy-gen=true
type StudioSourceFileListRequest struct {
	imachinery.BasicQueryParam
	SourceRevision int64  `form:"source_revision" binding:"min=0"`
	Prefix         string `form:"prefix" binding:"omitempty,max=1024"`
}

// +k8s:deepcopy-gen=true
type StudioFileContentRequest struct {
	Path           string `form:"path" binding:"required,max=1024"`
	SourceRevision int64  `form:"source_revision" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type StudioChangeSetRequest struct {
	BaseRevision      int64                   `json:"base_revision" binding:"min=0"`
	IdempotencyKey    string                  `json:"idempotency_key" binding:"required,max=200"`
	Operations        []StudioChangeOperation `json:"operations" binding:"required,min=1,max=200,dive"`
	Summary           string                  `json:"summary,omitempty" binding:"omitempty,max=2000"`
	AgentID           string                  `json:"-"`
	AgentSessionID    string                  `json:"-"`
	AgentInvocationID string                  `json:"-"`
}

func (r *StudioChangeSetRequest) Validate() error {
	for _, op := range r.Operations {
		if op.Operation != "create" && op.Operation != "update" && op.Operation != "delete" && op.Operation != "move" {
			return fmt.Errorf("unsupported change operation")
		}
		if op.Path == "" || strings.HasPrefix(op.Path, "/") || strings.Contains(op.Path, "..") {
			return fmt.Errorf("invalid source path")
		}
		if (op.Operation == "create" || op.Operation == "update") && op.Content == nil {
			return fmt.Errorf("source content is required")
		}
		if op.Operation == "move" && (op.TargetPath == nil || *op.TargetPath == "") {
			return fmt.Errorf("move target path is required")
		}
	}
	return nil
}

// +k8s:deepcopy-gen=true
type StudioSnapshotRequest struct {
	SourceRevision int64  `json:"source_revision" binding:"min=0"`
	IdempotencyKey string `json:"idempotency_key,omitempty" binding:"omitempty,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationVersionCreateRequest struct {
	SourceSnapshotID string `json:"source_snapshot_id" binding:"required,max=128"`
	Version          string `json:"version" binding:"required,min=1,max=100"`
	IdempotencyKey   string `json:"idempotency_key" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioBuildRequest struct {
	SourceSnapshotID           string `json:"source_snapshot_id" binding:"required,max=128"`
	IdempotencyKey             string `json:"idempotency_key" binding:"required,max=200"`
	StudioApplicationVersionID string `json:"studio_application_version_id,omitempty" binding:"omitempty,max=128"`
}

// +k8s:deepcopy-gen=true
type StudioPreviewRequest struct {
	SourceRevision int64  `json:"source_revision" binding:"min=0"`
	IdempotencyKey string `json:"idempotency_key,omitempty" binding:"omitempty,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioRestoreRevisionRequest struct {
	SourceRevision int64  `json:"source_revision" binding:"min=0"`
	BaseRevision   int64  `json:"base_revision" binding:"min=0"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioSourceSearchRequest struct {
	imachinery.BasicQueryParam
	Query          string `form:"query" binding:"required,min=1,max=500"`
	SourceRevision int64  `form:"source_revision" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeConfigRequest struct {
	PublicConfig          json.RawMessage `json:"public_config,omitempty"`
	SecretReferences      []string        `json:"secret_references,omitempty" binding:"omitempty,max=100,dive,max=1024"`
	IntegrationReferences []string        `json:"integration_references,omitempty" binding:"omitempty,max=100,dive,max=1024"`
	ResourceVersion       int64           `json:"resource_version" binding:"min=0"`
}

func (r *StudioRuntimeConfigRequest) Validate() error {
	if len(r.PublicConfig) > 64*1024 {
		return fmt.Errorf("runtime public config exceeds 64 KiB")
	}
	for _, ref := range r.SecretReferences {
		if !strings.HasPrefix(ref, "secret://") {
			return fmt.Errorf("invalid secret reference")
		}
	}
	for _, ref := range r.IntegrationReferences {
		if !strings.HasPrefix(ref, "integration://") {
			return fmt.Errorf("invalid integration reference")
		}
	}
	return nil
}

// +k8s:deepcopy-gen=true
type StudioReleaseRequest struct {
	StudioBuildID              string `json:"studio_build_id" binding:"required,max=128"`
	StudioApplicationVersionID string `json:"studio_application_version_id" binding:"required,max=128"`
	Environment                string `json:"environment" binding:"required,oneof=preview production"`
	RuntimeConfigID            string `json:"runtime_config_id" binding:"required,max=128"`
	IdempotencyKey             string `json:"idempotency_key" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioActionRequest struct {
	Reason          string `json:"reason,omitempty" binding:"omitempty,max=1000"`
	RequestID       string `json:"request_id,omitempty" binding:"omitempty,max=200"`
	ResourceVersion int64  `json:"resource_version" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type StudioBuildListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioReleaseListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationVersionListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeInstanceListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
	Environment         string `form:"environment" binding:"omitempty,oneof=preview production"`
	Status              string `form:"status" binding:"omitempty,oneof=CREATING READY DEGRADED STOPPED FAILED"`
}
