package workflowruntime

import (
	"context"
	"testing"
	"time"
)

func TestFakeStartExecutionIsIdempotent(t *testing.T) {
	runtime := NewFake()
	ctx := context.Background()
	if _, err := runtime.RegisterDefinition(ctx, Definition{Name: "test", Version: 1, Tasks: []Task{{Name: "test.run", ReferenceName: "run", Type: "SIMPLE"}}}); err != nil {
		t.Fatal(err)
	}
	first, err := runtime.StartExecution(ctx, StartRequest{DefinitionName: "test", DefinitionVersion: 1, IdempotencyKey: "same"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.StartExecution(ctx, StartRequest{DefinitionName: "test", DefinitionVersion: 1, IdempotencyKey: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("execution ids differ: %s != %s", first.ID, second.ID)
	}
}

func TestFakeDeletesOnlyTerminalExecution(t *testing.T) {
	runtime := NewFake()
	ctx := context.Background()
	_, _ = runtime.RegisterDefinition(ctx, Definition{Name: "task_center_reconcile_controller", Version: 1})
	running, _ := runtime.StartExecution(ctx, StartRequest{DefinitionName: "task_center_reconcile_controller", DefinitionVersion: 1})
	if err := runtime.DeleteTerminalExecution(ctx, running.ID); err == nil {
		t.Fatal("running execution was deleted")
	}
	if err := runtime.CancelExecution(ctx, running.ID, "done"); err != nil {
		t.Fatal(err)
	}
	items, err := runtime.ListTerminalExecutions(ctx, "task_center_reconcile_controller", time.Now().Add(time.Second), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %#v, err = %v", items, err)
	}
	if err := runtime.DeleteTerminalExecution(ctx, running.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.GetExecution(ctx, running.ID); err == nil {
		t.Fatal("terminal execution still exists")
	}
}

func TestTerminalExecutionStatusUsesAllowlist(t *testing.T) {
	for _, status := range []string{"COMPLETED", "FAILED", "TERMINATED", "TIMED_OUT", "CANCELED"} {
		if !isTerminalExecutionStatus(status) {
			t.Fatalf("status %s must be terminal", status)
		}
	}
	for _, status := range []string{"", "RUNNING", "PAUSED", "SCHEDULED"} {
		if isTerminalExecutionStatus(status) {
			t.Fatalf("status %s must not be terminal", status)
		}
	}
}
func TestFakeCancelAndRetry(t *testing.T) {
	runtime := NewFake()
	ctx := context.Background()
	_, _ = runtime.RegisterDefinition(ctx, Definition{Name: "test", Version: 1})
	execution, _ := runtime.StartExecution(ctx, StartRequest{DefinitionName: "test", DefinitionVersion: 1})
	if err := runtime.CancelExecution(ctx, execution.ID, "requested"); err != nil {
		t.Fatal(err)
	}
	canceled, _ := runtime.GetExecution(ctx, execution.ID)
	if canceled.Status != "TERMINATED" {
		t.Fatalf("status = %s", canceled.Status)
	}
	if err := runtime.RetryExecution(ctx, execution.ID); err != nil {
		t.Fatal(err)
	}
	retried, _ := runtime.GetExecution(ctx, execution.ID)
	if retried.Status != "RUNNING" {
		t.Fatalf("status = %s", retried.Status)
	}
}
