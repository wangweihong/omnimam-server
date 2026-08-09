package invocationsse

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestParseLastEventID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "absent", value: "", want: 0},
		{name: "zero", value: "0", want: 0},
		{name: "positive", value: "42", want: 42},
		{name: "leading zero", value: "01", wantErr: true},
		{name: "explicit plus", value: "+1", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "space", value: " 1", wantErr: true},
		{name: "decimal", value: "1.0", wantErr: true},
		{name: "overflow", value: strings.Repeat("9", strconv.IntSize), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseLastEventID(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseLastEventID(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("ParseLastEventID(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestWriteEvent(t *testing.T) {
	t.Parallel()
	occurredAt := imachinery.NewTime(time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC))
	event := &iapiserver.AgentOperationEvent{
		ObjectMeta:   imachinery.ObjectMeta{ID: "event-1", Name: "internal", CreatedAt: occurredAt},
		InvocationID: "invocation-1",
		EventType:    iapiserver.AgentOperationEventTypeInvocationStarted,
		SequenceNo:   1,
		Payload:      json.RawMessage(`{"status":"RUNNING"}`),
	}
	var output bytes.Buffer
	if err := WriteEvent(&output, event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 || lines[0] != "id: 1" || lines[1] != "event: invocation.started" || !strings.HasPrefix(lines[2], "data: ") {
		t.Fatalf("unexpected SSE frame: %q", output.String())
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[2], "data: ")), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	for _, key := range []string{"invocation_id", "sequence_no", "occurred_at", "event", "payload"} {
		if _, exists := envelope[key]; !exists {
			t.Fatalf("envelope is missing %q: %s", key, lines[2])
		}
	}
	for _, key := range []string{"id", "name", "created_at", "event_type"} {
		if _, exists := envelope[key]; exists {
			t.Fatalf("envelope exposes persistence field %q: %s", key, lines[2])
		}
	}
}

func TestTerminalClassifiers(t *testing.T) {
	t.Parallel()
	if !IsTerminalEventType(iapiserver.AgentOperationEventTypeInvocationFailed) ||
		IsTerminalEventType(iapiserver.AgentOperationEventTypeMessageCompleted) {
		t.Fatal("terminal event classification does not match the Invocation contract")
	}
	if !IsTerminalInvocationStatus(iapiserver.AgentInvocationStatusCanceled) ||
		IsTerminalInvocationStatus(iapiserver.AgentInvocationStatusRunning) {
		t.Fatal("terminal status classification does not match the Invocation contract")
	}
}
