package apiserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type recordingRepresentationTaskCreator struct {
	request *iapiserver.DAGTaskGroupCreateRequest
	err     error
}

func (f *recordingRepresentationTaskCreator) CreateDAGTaskGroup(_ context.Context, request *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error) {
	f.request = request
	return &iapiserver.DAGTaskGroup{}, f.err
}

func TestHealthCron(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     string
	}{
		{name: "seconds", interval: 30 * time.Second, want: "*/30 * * * * *"},
		{name: "minutes", interval: 5 * time.Minute, want: "0 */5 * * * *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthCron(tt.interval); got != tt.want {
				t.Fatalf("healthCron(%s) = %q, want %q", tt.interval, got, tt.want)
			}
		})
	}
}

func TestScheduleTargetOwnership(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{ProjectID: "project", Namespace: "namespace", CreatedBy: "user-1"}
	schedule.ID = "schedule-1"

	atomic := &iapiserver.AtomicTaskCreateRequest{}
	applyAtomicScheduleOwnership(atomic, schedule)
	if atomic.ProjectID != "project" || atomic.Namespace != "namespace" || atomic.CreatedBy != "user-1" || atomic.OwnerType != iapiserver.TaskOwnerTypeSchedule || atomic.OwnerID != "schedule-1" {
		t.Fatalf("atomic ownership = %#v", atomic)
	}
	group := &iapiserver.TaskGroupCreateRequest{}
	applyGroupScheduleOwnership(group, schedule)
	if group.ProjectID != "project" || group.Namespace != "namespace" || group.CreatedBy != "user-1" {
		t.Fatalf("group created_by = %q", group.CreatedBy)
	}
	dag := &iapiserver.DAGTaskGroupCreateRequest{}
	applyDAGScheduleOwnership(dag, schedule)
	if dag.ProjectID != "project" || dag.Namespace != "namespace" || dag.CreatedBy != "user-1" {
		t.Fatalf("dag created_by = %q", dag.CreatedBy)
	}
}

func TestScheduleTimeReadsConductorMilliseconds(t *testing.T) {
	want := time.Date(2026, time.July, 18, 1, 20, 30, 0, time.UTC)
	if got := scheduleTime(float64(want.UnixMilli())); !got.Equal(want) {
		t.Fatalf("scheduleTime = %s, want %s", got, want)
	}
}

func TestRepresentationDAGRequestCreatesImageThumbnailPlan(t *testing.T) {
	payload, err := json.Marshal(representationRequestedEvent{
		AssetID: "asset-1", AssetVersionID: "version-1", OwnerUserID: "user-1",
		ProjectID: "default", Namespace: "default", MediaType: "image", ProfileVersion: "default-v1",
		RequestedRepresentations: []requestedRepresentation{{RepresentationType: "thumbnail", Profile: "list-320"}},
		IdempotencyKey:           "asset-representations:version-1:default-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	creator := &recordingRepresentationTaskCreator{}
	if err := handleRepresentationRequested(context.Background(), creator, payload); err != nil {
		t.Fatalf("handleRepresentationRequested() error = %v", err)
	}
	request := creator.request
	if request == nil || request.ProjectID != "default" || request.Namespace != "default" || request.CreatedBy != "user-1" {
		t.Fatalf("request ownership = %#v", request)
	}
	if request.IdempotencyScope != "asset-representations" || request.IdempotencyKey != "asset-representations:version-1:default-v1" {
		t.Fatalf("request idempotency = %q/%q", request.IdempotencyScope, request.IdempotencyKey)
	}
	wantKeys := []string{"inspect", "thumbnail:list-320", "finalize"}
	if len(request.Nodes) != len(wantKeys) {
		t.Fatalf("node count = %d, want %d", len(request.Nodes), len(wantKeys))
	}
	for index, want := range wantKeys {
		if request.Nodes[index].Key != want {
			t.Fatalf("node[%d].key = %q, want %q", index, request.Nodes[index].Key, want)
		}
	}
	generate := request.Nodes[1].Task
	if generate.Arguments["media_type"] != "image" || generate.Arguments["max_attempts"] != 3 || generate.RetryPolicy.MaxAttempts != 3 || generate.RetryPolicy.BackoffType != "EXPONENTIAL_BACKOFF" {
		t.Fatalf("generate task = %#v", generate)
	}
	if len(request.Edges) != 2 || request.Edges[0].FromNode != "inspect" || request.Edges[0].ToNode != "thumbnail:list-320" || request.Edges[1].FromNode != "thumbnail:list-320" || request.Edges[1].ToNode != "finalize" {
		t.Fatalf("edges = %#v", request.Edges)
	}
}

func TestRepresentationDAGRequestRejectsLegacyEvent(t *testing.T) {
	_, err := representationDAGRequest([]byte(`{"asset_id":"asset-1","asset_version_id":"version-1","owner_user_id":"user-1","profile_version":"default-v1"}`))
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("representationDAGRequest() error = %v", err)
	}
}

func TestRepresentationOrchestratorConsumerGroupIsStable(t *testing.T) {
	if representationOrchestratorConsumerGroup != "task-center-representation-orchestrator" {
		t.Fatalf("consumer group = %q", representationOrchestratorConsumerGroup)
	}
}
