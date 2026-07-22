package workflowruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingTaskLogManager struct{ appendCalls int }

func (m *failingTaskLogManager) AppendTaskLog(context.Context, string, TaskLogEntry) error {
	m.appendCalls++
	return ErrUnavailable
}

func (*failingTaskLogManager) ListTaskLogs(context.Context, string) ([]TaskLogEntry, error) {
	return nil, ErrUnavailable
}

func TestSanitizeTaskLogMessage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		forbidden []string
	}{
		{name: "credentials and url", input: "Authorization: Bearer secret-token api_key=topsecret https://user:pass@example.invalid/path", forbidden: []string{"secret-token", "topsecret", "example.invalid", "user:pass"}},
		{name: "basic and json credentials", input: `Authorization: Basic dXNlcjpwYXNz {"api key":"json-secret","password":"json-password"}`, forbidden: []string{"dXNlcjpwYXNz", "json-secret", "json-password"}},
		{name: "log injection", input: "first\nsecond\tthird", forbidden: []string{"\n", "\t"}},
		{name: "invalid utf8", input: string([]byte{'o', 'k', 0xff}), forbidden: []string{string([]byte{0xff})}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTaskLogMessage(tt.input)
			for _, forbidden := range tt.forbidden {
				if strings.Contains(got, forbidden) {
					t.Fatalf("sanitizeTaskLogMessage() = %q contains %q", got, forbidden)
				}
			}
		})
	}
	long := sanitizeTaskLogMessage(strings.Repeat("界", 2000))
	if len(long) > maxTaskLogMessageBytes || !strings.Contains(long, "界") {
		t.Fatalf("long message bytes = %d", len(long))
	}
}

func TestWorkerTaskLogFailureDoesNotChangeHandlerResult(t *testing.T) {
	manager := &failingTaskLogManager{}
	task := WorkerTask{RuntimeTaskID: "runtime-task-1", Logger: newBoundTaskLogger("test", "runtime-task-1", manager)}
	handler := Handler(func(ctx context.Context, task WorkerTask) (map[string]any, error) {
		task.Log(ctx, WorkerLog("phase.started", TaskLogLevelInfo, "started"))
		return map[string]any{"status": "ok"}, nil
	})

	output, err := handler(context.Background(), task)
	if err != nil || output["status"] != "ok" {
		t.Fatalf("output = %#v, err = %v", output, err)
	}
	if manager.appendCalls != 1 {
		t.Fatalf("append calls = %d", manager.appendCalls)
	}
}

func TestNormalizeTaskLogsSortsAndDeduplicatesEventKeys(t *testing.T) {
	base := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)
	worker := WorkerLog("provider.completed", TaskLogLevelInfo, "done")
	worker.OccurredAt = base.Add(2 * time.Second)
	logs := normalizeTaskLogs([]TaskLogEntry{
		worker,
		{Source: TaskLogSourceLifecycle, Level: TaskLogLevelInfo, EventKey: "attempt.started", Message: "duplicate", OccurredAt: base.Add(time.Second)},
		{Source: TaskLogSourceLifecycle, Level: TaskLogLevelInfo, EventKey: "attempt.started", Message: "started", OccurredAt: base},
	})
	if len(logs) != 2 {
		t.Fatalf("logs = %#v, want two deduplicated entries", logs)
	}
	if logs[0].Message != "started" || logs[0].Sequence != 1 || logs[1].Sequence != 2 {
		t.Fatalf("logs are not stably sorted and sequenced: %#v", logs)
	}
}

func TestConductorTaskLogsRoundTripAndLegacyCompatibility(t *testing.T) {
	var written taskLogEnvelope
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/runtime-task-1/log" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&written); err != nil {
				t.Fatalf("decode log envelope: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"log": `plain token=secret https://example.invalid`, "taskId": "runtime-task-1", "createdTime": int64(1000)},
				{"log": `{"v":1,"source":"LIFECYCLE","level":"INFO","event_key":"attempt.started","message":"started"}`, "taskId": "runtime-task-1", "createdTime": int64(2000)},
			})
		default:
			t.Fatalf("method = %s", r.Method)
		}
	}))
	defer server.Close()
	runtime, err := NewConductor(ConductorConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AppendTaskLog(context.Background(), "runtime-task-1", WorkerLog("provider.ready", TaskLogLevelInfo, "ready token=secret")); err != nil {
		t.Fatal(err)
	}
	if written.Version != 1 || strings.Contains(written.Message, "secret") {
		t.Fatalf("written envelope = %#v", written)
	}
	logs, err := runtime.ListTaskLogs(context.Background(), "runtime-task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].Source != TaskLogSourceWorker || strings.Contains(logs[0].Message, "secret") || strings.Contains(logs[0].Message, "example.invalid") {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestConductorTaskLogsMapMissingRuntimeTask(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	runtime, err := NewConductor(ConductorConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ListTaskLogs(context.Background(), "missing"); !strings.Contains(err.Error(), ErrTaskLogNotFound.Error()) {
		t.Fatalf("error = %v, want ErrTaskLogNotFound", err)
	}
}

func TestFakeTaskLogsAreIsolatedByRuntimeTask(t *testing.T) {
	runtime := NewFake()
	if err := runtime.AppendTaskLog(context.Background(), "task-1", WorkerLog("phase.one", TaskLogLevelInfo, "one")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AppendTaskLog(context.Background(), "task-2", WorkerLog("phase.two", TaskLogLevelWarn, "two")); err != nil {
		t.Fatal(err)
	}
	logs, err := runtime.ListTaskLogs(context.Background(), "task-1")
	if err != nil || len(logs) != 1 || logs[0].Message != "one" {
		t.Fatalf("logs = %#v, err = %v", logs, err)
	}
}
