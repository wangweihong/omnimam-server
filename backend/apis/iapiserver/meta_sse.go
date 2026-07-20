package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	SSESourceDomainTaskCenter   = "task-center"
	SSESourceDomainAssetLibrary = "asset-library"

	UserEventAtomicTaskCreated         = "atomic_task.created"
	UserEventAtomicTaskBlocked         = "atomic_task.blocked"
	UserEventAtomicTaskReady           = "atomic_task.ready"
	UserEventAtomicTaskStarted         = "atomic_task.started"
	UserEventAtomicTaskProgressed      = "atomic_task.progressed"
	UserEventAtomicTaskRetrying        = "atomic_task.retrying"
	UserEventAtomicTaskCancelRequested = "atomic_task.cancel_requested"
	UserEventAtomicTaskSucceeded       = "atomic_task.succeeded"
	UserEventAtomicTaskFailed          = "atomic_task.failed"
	UserEventAtomicTaskCanceled        = "atomic_task.canceled"
	UserEventAtomicTaskTimedOut        = "atomic_task.timed_out"
	UserEventAtomicTaskSkipped         = "atomic_task.skipped"

	UserEventTaskAttemptCreated   = "task_attempt.created"
	UserEventTaskAttemptStarted   = "task_attempt.started"
	UserEventTaskAttemptSucceeded = "task_attempt.succeeded"
	UserEventTaskAttemptFailed    = "task_attempt.failed"
	UserEventTaskAttemptCanceled  = "task_attempt.canceled"
	UserEventTaskAttemptTimedOut  = "task_attempt.timed_out"

	UserEventTaskGroupCreated       = "task_group.created"
	UserEventTaskGroupStarted       = "task_group.started"
	UserEventTaskGroupProgressed    = "task_group.progressed"
	UserEventTaskGroupSucceeded     = "task_group.succeeded"
	UserEventTaskGroupFailed        = "task_group.failed"
	UserEventTaskGroupCanceled      = "task_group.canceled"
	UserEventDAGTaskGroupCreated    = "dag_task_group.created"
	UserEventDAGTaskGroupStarted    = "dag_task_group.started"
	UserEventDAGTaskGroupProgressed = "dag_task_group.progressed"
	UserEventDAGTaskGroupSucceeded  = "dag_task_group.succeeded"
	UserEventDAGTaskGroupFailed     = "dag_task_group.failed"
	UserEventDAGTaskGroupCanceled   = "dag_task_group.canceled"

	UserEventArtifactCreated               = "artifact.created"
	UserEventArtifactTransferring          = "artifact.transferring"
	UserEventArtifactProcessing            = "artifact.processing"
	UserEventArtifactPreviewReady          = "artifact.preview_ready"
	UserEventArtifactReady                 = "artifact.ready"
	UserEventArtifactProcessingFailed      = "artifact.processing_failed"
	UserEventArtifactRegistrationSucceeded = "artifact.registration_succeeded"
	UserEventArtifactRegistrationFailed    = "artifact.registration_failed"
	UserEventArtifactDeleted               = "artifact.deleted"

	UserEventAssetVersionProcessingStarted    = "asset_version.processing_started"
	UserEventAssetVersionProcessingProgressed = "asset_version.processing_progressed"
	UserEventAssetVersionReady                = "asset_version.ready"
	UserEventAssetVersionReadyWithWarnings    = "asset_version.ready_with_warnings"
	UserEventAssetVersionProcessingFailed     = "asset_version.processing_failed"
)

// UserEvent 是面向单个登录用户的短期可重放事件投影，不是任务或素材事实源。
type UserEvent struct {
	imachinery.ObjectMeta `json:"-"`
	EventSequence         int64           `json:"event_id" gorm:"column:event_sequence;type:bigserial;autoIncrement;uniqueIndex"`
	RecipientUserID       string          `json:"-" gorm:"column:recipient_user_id;type:text;not null;uniqueIndex:idx_sse_user_events_source,priority:1;index:idx_sse_recipient_sequence,priority:1;index:idx_sse_recipient_occurred,priority:1"`
	EventType             string          `json:"event_type" gorm:"column:event_type;type:text;not null;uniqueIndex:idx_sse_user_events_source,priority:4"`
	EventVersion          int             `json:"event_version" gorm:"column:event_version;not null;default:1"`
	AggregateType         string          `json:"aggregate_type" gorm:"column:aggregate_type;type:text;not null;index:idx_sse_aggregate,priority:1"`
	AggregateID           string          `json:"aggregate_id" gorm:"column:aggregate_id;type:text;not null;index:idx_sse_aggregate,priority:2"`
	AggregateVersion      int64           `json:"aggregate_version" gorm:"column:aggregate_version;not null;index:idx_sse_aggregate,priority:3"`
	CorrelationID         string          `json:"correlation_id,omitempty" gorm:"column:correlation_id;type:text"`
	CausationID           string          `json:"causation_id,omitempty" gorm:"column:causation_id;type:text"`
	ApplicationRunID      string          `json:"application_run_id,omitempty" gorm:"column:application_run_id;type:text"`
	TaskGroupID           string          `json:"task_group_id,omitempty" gorm:"column:task_group_id;type:text"`
	DAGTaskGroupID        string          `json:"dag_task_group_id,omitempty" gorm:"column:dag_task_group_id;type:text"`
	AtomicTaskID          string          `json:"atomic_task_id,omitempty" gorm:"column:atomic_task_id;type:text"`
	TaskAttemptID         string          `json:"task_attempt_id,omitempty" gorm:"column:task_attempt_id;type:text"`
	ArtifactID            string          `json:"artifact_id,omitempty" gorm:"column:artifact_id;type:text"`
	AssetID               string          `json:"asset_id,omitempty" gorm:"column:asset_id;type:text"`
	AssetVersionID        string          `json:"asset_version_id,omitempty" gorm:"column:asset_version_id;type:text"`
	Payload               map[string]any  `json:"payload" gorm:"-"`
	PayloadShadow         string          `json:"-" gorm:"column:payload_json;type:text;not null;default:'{}'"`
	SourceDomain          string          `json:"-" gorm:"column:source_domain;type:text;not null;uniqueIndex:idx_sse_user_events_source,priority:2"`
	SourceEventID         string          `json:"-" gorm:"column:source_event_id;type:text;not null;uniqueIndex:idx_sse_user_events_source,priority:3"`
	OccurredAt            imachinery.Time `json:"occurred_at" gorm:"column:occurred_at;type:timestamptz;not null;index:idx_sse_recipient_occurred,priority:2"`
	ExpiresAt             imachinery.Time `json:"-" gorm:"column:expires_at;type:timestamptz;not null;index"`
}

func (UserEvent) TableName() string { return "sse_user_events" }

func (e *UserEvent) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if e.EventVersion == 0 {
		e.EventVersion = 1
	}
	raw, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	e.PayloadShadow = string(raw)
	return nil
}

func (e *UserEvent) AfterCreate(*gorm.DB) error { return nil }

func (e *UserEvent) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	raw, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	e.PayloadShadow = string(raw)
	return nil
}

func (e *UserEvent) AfterUpdate(*gorm.DB) error { return nil }

func (e *UserEvent) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if e.PayloadShadow == "" {
		e.Payload = map[string]any{}
		return nil
	}
	return json.Unmarshal([]byte(e.PayloadShadow), &e.Payload)
}
