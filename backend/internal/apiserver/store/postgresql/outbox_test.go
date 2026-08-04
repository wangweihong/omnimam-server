package postgresql

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestNewOutboxMessageSeparatesUUIDAndIdempotencyKey(t *testing.T) {
	idempotencyKey := uuid.NewString() + ":uploaded"
	payload := []byte(`{"asset_id":"asset-1"}`)

	msg := newOutboxMessage(idempotencyKey, payload)

	if _, err := uuid.Parse(msg.UUID); err != nil {
		t.Fatalf("message UUID %q is invalid: %v", msg.UUID, err)
	}
	if len(msg.UUID) != 36 {
		t.Fatalf("message UUID length = %d, want 36", len(msg.UUID))
	}
	if got := msg.Metadata.Get(outboxIdempotencyKeyMetadata); got != idempotencyKey {
		t.Fatalf("idempotency key = %q, want %q", got, idempotencyKey)
	}
	if got := string(msg.Payload); got != string(payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}

func TestAppStudioEventKeyFormats(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		components []any
		want       string
	}{
		{"application lifecycle", studioApplicationLifecycleChangedEvent, []any{"app-1", int64(1)}, "studio_application_lifecycle_changed:app-1:1"},
		{"source revision", studioSourceRevisionChangedEvent, []any{"app-1", int64(1)}, "studio_source_revision_changed:app-1:1"},
		{"source snapshot", studioSourceSnapshotCreatedEvent, []any{"snapshot-1", int64(2)}, "studio_source_snapshot_created:snapshot-1:2"},
		{"build projection", studioBuildProjectionChangedEvent, []any{"build-1", int64(3)}, "studio_build_projection_changed:build-1:3"},
		{"preview runtime", studioPreviewRuntimeChangedEvent, []any{"preview-1", int64(4)}, "studio_preview_runtime_status_changed:preview-1:4"},
		{"release status", studioReleaseStatusChangedEvent, []any{"release-1", int64(5)}, "studio_release_status_changed:release-1:5"},
		{"runtime instance", studioRuntimeInstanceChangedEvent, []any{"runtime-1", int64(6)}, "studio_runtime_instance_status_changed:runtime-1:6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appStudioEventKey(tt.eventType, tt.components...); got != tt.want {
				t.Fatalf("appStudioEventKey() = %q, want %q", got, tt.want)
			}
		})
	}

	lifecycleKey := appStudioEventKey(studioApplicationLifecycleChangedEvent, "app-1", int64(1))
	firstRevisionKey := appStudioEventKey(studioSourceRevisionChangedEvent, "app-1", int64(1))
	if lifecycleKey == firstRevisionKey {
		t.Fatalf("lifecycle key %q conflicts with first revision key", lifecycleKey)
	}
	if repeated := appStudioEventKey(studioSourceRevisionChangedEvent, "app-1", int64(1)); repeated != firstRevisionKey {
		t.Fatalf("repeated revision key = %q, want stable key %q", repeated, firstRevisionKey)
	}
	if next := appStudioEventKey(studioSourceRevisionChangedEvent, "app-1", int64(2)); next == firstRevisionKey {
		t.Fatalf("consecutive revision key %q must differ from %q", next, firstRevisionKey)
	}
}

func TestAppStudioEventPayloadFields(t *testing.T) {
	app := &iapiserver.StudioApplication{ObjectMeta: imachinery.ObjectMeta{ID: "app-1"}, OwnerUserID: "user-1", Status: "ACTIVE"}
	changeSet := &iapiserver.StudioChangeSet{ObjectMeta: imachinery.ObjectMeta{ID: "change-1"}, BaseRevision: 0, AgentID: "agent-1", AgentInvocationID: "invocation-1"}
	snapshot := &iapiserver.StudioSourceSnapshot{ObjectMeta: imachinery.ObjectMeta{ID: "snapshot-1"}, StudioApplicationID: "app-1", WorkspaceRevision: 1, ContentDigest: "content-digest", ManifestDigest: "manifest-digest", CreatedBy: "user-1"}
	build := &iapiserver.StudioBuild{ObjectMeta: imachinery.ObjectMeta{ID: "build-1"}, StudioApplicationID: "app-1", SourceSnapshotID: "snapshot-1", AtomicTaskID: "task-1", ArtifactID: "artifact-1", ArtifactDigest: "artifact-digest", Status: "SUCCEEDED"}
	preview := &iapiserver.StudioPreviewRuntime{ObjectMeta: imachinery.ObjectMeta{ID: "preview-1"}, StudioApplicationID: "app-1", WorkspaceRevision: 1, Status: "RUNNING"}
	release := &iapiserver.StudioRelease{ObjectMeta: imachinery.ObjectMeta{ID: "release-1"}, StudioApplicationID: "app-1", StudioApplicationVersionID: "version-1", StudioBuildID: "build-1", RuntimeConfigID: "config-1", ArtifactID: "artifact-1", ArtifactDigest: "artifact-digest", Environment: "production", RuntimeInstanceID: "runtime-1", RollbackOfReleaseID: "release-0", Status: "READY"}
	runtime := &iapiserver.StudioRuntimeInstance{ObjectMeta: imachinery.ObjectMeta{ID: "runtime-1"}, StudioApplicationID: "app-1", StudioReleaseID: "release-1", Environment: "production", AtomicTaskID: "task-2", InfraRuntimeID: "infra-1", Status: "READY", HealthStatus: "HEALTHY", ErrorCode: "RUNTIME_FAILED", IsCurrent: true}

	tests := []struct {
		name    string
		payload map[string]any
		fields  []string
	}{
		{"application lifecycle", studioApplicationLifecyclePayload(app, nil), []string{"studio_application_id", "owner_user_id", "from_status", "to_status", "resource_version", "occurred_at"}},
		{"initial revision", studioSourceRevisionPayload("app-1", 0, nil), []string{"studio_application_id", "previous_revision", "current_revision", "change_set_id", "agent_id", "agent_invocation_id", "resource_version", "occurred_at"}},
		{"first revision", studioSourceRevisionPayload("app-1", 1, changeSet), []string{"studio_application_id", "previous_revision", "current_revision", "change_set_id", "agent_id", "agent_invocation_id", "resource_version", "occurred_at"}},
		{"snapshot", studioSourceSnapshotPayload(snapshot), []string{"source_snapshot_id", "studio_application_id", "source_revision", "content_digest", "manifest_digest", "created_by", "resource_version", "occurred_at"}},
		{"build", studioBuildPayload(build, "RUNNING"), []string{"studio_build_id", "studio_application_id", "source_snapshot_id", "atomic_task_id", "artifact_id", "artifact_digest", "from_status", "to_status", "error_code", "resource_version", "occurred_at"}},
		{"preview", studioPreviewPayload(preview, "STARTING"), []string{"preview_runtime_id", "studio_application_id", "source_revision", "from_status", "to_status", "diagnostics_summary", "error_code", "resource_version", "occurred_at"}},
		{"release", studioReleasePayload(release, "DEPLOYING"), []string{"studio_release_id", "studio_application_id", "studio_application_version_id", "studio_build_id", "runtime_config_id", "artifact_id", "artifact_digest", "environment", "runtime_instance_id", "rollback_of_release_id", "from_status", "to_status", "error_code", "resource_version", "occurred_at"}},
		{"runtime instance", studioRuntimePayload(runtime, "DEPLOYING"), []string{"runtime_instance_id", "studio_release_id", "studio_application_id", "environment", "atomic_task_id", "infra_runtime_id", "from_status", "to_status", "health_status", "is_current", "error_code", "resource_version", "occurred_at"}},
	}

	decoded := make(map[string]map[string]any, len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := decodeAppStudioEventPayload(t, tt.payload)
			assertExactPayloadFields(t, payload, tt.fields)
			decoded[tt.name] = payload
		})
	}

	if payload := decoded["initial revision"]; payload["previous_revision"] != nil || payload["current_revision"] != float64(0) || payload["change_set_id"] != nil || payload["agent_id"] != nil || payload["agent_invocation_id"] != nil {
		t.Fatalf("initial revision payload = %#v", payload)
	}
	if payload := decoded["first revision"]; payload["previous_revision"] != float64(0) || payload["current_revision"] != float64(1) || payload["change_set_id"] != "change-1" || payload["agent_id"] != "agent-1" || payload["agent_invocation_id"] != "invocation-1" {
		t.Fatalf("first revision payload = %#v", payload)
	}
	if diagnostics, ok := decoded["preview"]["diagnostics_summary"].(map[string]any); !ok || diagnostics == nil {
		t.Fatalf("preview diagnostics_summary = %#v, want object", decoded["preview"]["diagnostics_summary"])
	}
	for _, name := range []string{"build", "preview", "release"} {
		if decoded[name]["error_code"] != nil {
			t.Fatalf("%s error_code = %#v, want nil", name, decoded[name]["error_code"])
		}
	}
	if got := decoded["release"]["runtime_instance_id"]; got != "runtime-1" {
		t.Fatalf("release runtime_instance_id = %#v, want runtime-1", got)
	}
	if got := decoded["runtime instance"]["error_code"]; got != "RUNTIME_FAILED" {
		t.Fatalf("runtime error_code = %#v, want persisted error code", got)
	}
	if repeated := studioSourceRevisionPayload("app-1", 1, changeSet); !reflect.DeepEqual(repeated, studioSourceRevisionPayload("app-1", 1, changeSet)) {
		t.Fatalf("repeated ChangeSet payload is not stable: %#v", repeated)
	}
}

func decodeAppStudioEventPayload(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	raw, err := marshalAppStudioEventPayload(payload, 7, time.Date(2026, time.August, 4, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)))
	if err != nil {
		t.Fatalf("marshalAppStudioEventPayload() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["resource_version"] != float64(7) || decoded["occurred_at"] != "2026-08-04T02:00:00Z" {
		t.Fatalf("common event fields = resource_version:%#v occurred_at:%#v", decoded["resource_version"], decoded["occurred_at"])
	}
	return decoded
}

func assertExactPayloadFields(t *testing.T, payload map[string]any, want []string) {
	t.Helper()
	got := make([]string, 0, len(payload))
	for field := range payload {
		got = append(got, field)
	}
	sort.Strings(got)
	want = append([]string(nil), want...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload fields = %v, want %v", got, want)
	}
}
