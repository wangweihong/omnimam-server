package workflowruntime

import (
	"context"
	"testing"
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
