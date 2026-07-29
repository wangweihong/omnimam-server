package postgresql

import (
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestGateCanvasNodeRequiredOutputs(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		terminal      bool
		readyRequired int
		required      int
		wantStatus    string
		wantTerminal  bool
		wantReason    any
	}{
		{
			name:   "successful task waits for required artifact",
			status: iapiserver.AtomicTaskStatusSuccess, terminal: true,
			readyRequired: 0, required: 1,
			wantStatus: iapiserver.AtomicTaskStatusRunning, wantReason: "waiting_for_required_outputs",
		},
		{
			name:   "successful task completes after required artifact",
			status: iapiserver.AtomicTaskStatusSuccess, terminal: true,
			readyRequired: 1, required: 1,
			wantStatus: iapiserver.AtomicTaskStatusSuccess, wantTerminal: true,
		},
		{
			name:   "failed task remains failed",
			status: iapiserver.AtomicTaskStatusFailed, terminal: true,
			readyRequired: 0, required: 1,
			wantStatus: iapiserver.AtomicTaskStatusFailed, wantTerminal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, terminal, reason := gateCanvasNodeRequiredOutputs(
				tt.status,
				tt.terminal,
				tt.readyRequired,
				tt.required,
			)
			if status != tt.wantStatus || terminal != tt.wantTerminal || reason != tt.wantReason {
				t.Fatalf("gate = (%q,%t,%#v)", status, terminal, reason)
			}
		})
	}
}
