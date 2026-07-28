package apiserver

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type recordingRepresentationTaskCreator struct {
	request *iapiserver.DAGTaskGroupCreateRequest
	err     error
}

func (f *recordingRepresentationTaskCreator) CreateDAGTaskGroup(_ context.Context, request *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error) {
	f.request = request
	return &iapiserver.DAGTaskGroup{}, f.err
}

type terminalProjectionTaskStore struct {
	store.TaskCenterStore
	task  *iapiserver.AtomicTask
	err   error
	calls int
}

func (s *terminalProjectionTaskStore) GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error) {
	s.calls++
	return s.task, s.err
}

type recordingTerminalProjector struct {
	task  *iapiserver.AtomicTask
	err   error
	calls int
}

func (p *recordingTerminalProjector) Completed(_ context.Context, task *iapiserver.AtomicTask) error {
	p.calls++
	p.task = task
	return p.err
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

func TestApplicationRunTerminalProjectionConsumerGroupIsStable(t *testing.T) {
	if applicationRunTerminalProjectionConsumerGroup != "application-platform-terminal-projection" {
		t.Fatalf("consumer group = %q", applicationRunTerminalProjectionConsumerGroup)
	}
}

func TestHandleApplicationRunTerminalProjection(t *testing.T) {
	terminalTask := &iapiserver.AtomicTask{
		ApplicationRunID: "run-1",
		Status:           iapiserver.AtomicTaskStatusSuccess,
	}
	terminalTask.ID = "task-1"

	tests := []struct {
		name               string
		payload            string
		task               *iapiserver.AtomicTask
		wantStoreCalls     int
		wantProjectorCalls int
		wantError          bool
	}{
		{
			name:               "terminal application run projects current task",
			payload:            `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"SUCCESS"}`,
			task:               terminalTask,
			wantStoreCalls:     1,
			wantProjectorCalls: 1,
		},
		{
			name:    "non-terminal event is acknowledged without lookup",
			payload: `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"RUNNING"}`,
		},
		{
			name:    "unrelated terminal task is acknowledged without lookup",
			payload: `{"atomic_task_id":"task-1","application_run_id":null,"to_status":"SUCCESS"}`,
		},
		{
			name:      "incomplete event is retried",
			payload:   `{"application_run_id":"run-1","to_status":"SUCCESS"}`,
			wantError: true,
		},
		{
			name:           "missing current task is retried",
			payload:        `{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"SUCCESS"}`,
			wantStoreCalls: 1,
			wantError:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &terminalProjectionTaskStore{task: tt.task}
			projector := &recordingTerminalProjector{}
			err := handleApplicationRunTerminalProjection(
				context.Background(),
				tasks,
				projector,
				[]byte(tt.payload),
			)
			if (err != nil) != tt.wantError {
				t.Fatalf("handleApplicationRunTerminalProjection() error = %v, wantError %t", err, tt.wantError)
			}
			if tasks.calls != tt.wantStoreCalls {
				t.Fatalf("task store calls = %d, want %d", tasks.calls, tt.wantStoreCalls)
			}
			if projector.calls != tt.wantProjectorCalls {
				t.Fatalf("projector calls = %d, want %d", projector.calls, tt.wantProjectorCalls)
			}
		})
	}
}

func TestConsumeApplicationRunTerminalProjectionAcknowledgement(t *testing.T) {
	task := &iapiserver.AtomicTask{
		ApplicationRunID: "run-1",
		Status:           iapiserver.AtomicTaskStatusSuccess,
	}
	task.ID = "task-1"
	payload := []byte(`{"atomic_task_id":"task-1","application_run_id":"run-1","to_status":"SUCCESS"}`)

	t.Run("ack after projection", func(t *testing.T) {
		msg := message.NewMessage("message-1", payload)
		messages := make(chan *message.Message, 1)
		messages <- msg
		close(messages)
		consumeApplicationRunTerminalProjections(
			context.Background(),
			messages,
			&terminalProjectionTaskStore{task: task},
			&recordingTerminalProjector{},
		)
		select {
		case <-msg.Acked():
		default:
			t.Fatal("terminal projection message was not acknowledged")
		}
	})

	t.Run("nack after projection failure", func(t *testing.T) {
		msg := message.NewMessage("message-2", payload)
		messages := make(chan *message.Message, 1)
		messages <- msg
		close(messages)
		consumeApplicationRunTerminalProjections(
			context.Background(),
			messages,
			&terminalProjectionTaskStore{task: task},
			&recordingTerminalProjector{err: stderrors.New("temporary failure")},
		)
		select {
		case <-msg.Nacked():
		default:
			t.Fatal("failed terminal projection message was not negatively acknowledged")
		}
	})
}
