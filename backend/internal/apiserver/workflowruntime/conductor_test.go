package workflowruntime

import (
	"strings"
	"testing"
)

func TestConductorTaskMapsRetryPolicy(t *testing.T) {
	task := conductorTask(Task{Name: "asset-library.representation.generate", ReferenceName: "thumbnail", Type: "SIMPLE", Retry: RetryPolicy{MaxAttempts: 3, RetryDelaySeconds: 5, BackoffType: "EXPONENTIAL_BACKOFF"}})
	if task.TaskDefinition == nil || task.TaskDefinition.RetryCount != 2 || task.TaskDefinition.RetryDelaySeconds != 5 || task.TaskDefinition.RetryLogic != "EXPONENTIAL_BACKOFF" || task.TaskDefinition.BackoffScaleFactor != 2 {
		t.Fatalf("task definition = %#v", task.TaskDefinition)
	}
}

func TestPrepareDynamicForkOutputAddsDeterministicBusinessIdentity(t *testing.T) {
	output := map[string]any{
		"dynamic_tasks": []any{
			map[string]any{"name": "image.generate", "taskReferenceName": "image_0", "type": "SIMPLE"},
			map[string]any{"name": "image.generate", "taskReferenceName": "image_1", "type": "SIMPLE"},
		},
		"dynamic_inputs": map[string]any{
			"image_0": map[string]any{"prompt": "first"},
			"image_1": map[string]any{"arguments": map[string]any{"prompt": "second"}},
		},
	}
	input := workerInput{AtomicTaskID: "planner-1", DAGNodeKey: "images", FunctionRef: "image.plan", MaxDynamicTasks: 2}
	registered := func(functionRef string) bool { return functionRef == "image.generate" }
	if err := prepareDynamicForkOutput(output, input, registered); err != nil {
		t.Fatal(err)
	}
	first := output["dynamic_inputs"].(map[string]any)["image_0"].(map[string]any)
	second := output["dynamic_inputs"].(map[string]any)["image_1"].(map[string]any)
	if first["atomic_task_id"] == "" || first["atomic_task_id"] == second["atomic_task_id"] {
		t.Fatalf("dynamic identities = %#v / %#v", first, second)
	}
	if first["dag_node_key"] != "images" || first["function_ref"] != "image.generate" {
		t.Fatalf("first identity envelope = %#v", first)
	}
	if first["arguments"].(map[string]any)["prompt"] != "first" || second["arguments"].(map[string]any)["prompt"] != "second" {
		t.Fatalf("dynamic arguments = %#v / %#v", first, second)
	}
	originalID := first["atomic_task_id"]
	if err := prepareDynamicForkOutput(output, input, registered); err != nil {
		t.Fatal(err)
	}
	if first["atomic_task_id"] != originalID {
		t.Fatalf("identity changed across replay: %v -> %v", originalID, first["atomic_task_id"])
	}
}

func TestPrepareDynamicForkOutputRejectsInvalidPlannerOutput(t *testing.T) {
	validOutput := func() map[string]any {
		return map[string]any{
			"dynamic_tasks": []any{
				map[string]any{"name": "image.generate", "taskReferenceName": "image_0", "type": "SIMPLE"},
				map[string]any{"name": "image.generate", "taskReferenceName": "image_1", "type": "SIMPLE"},
			},
			"dynamic_inputs": map[string]any{
				"image_0": map[string]any{"prompt": "first"},
				"image_1": map[string]any{"prompt": "second"},
			},
		}
	}
	input := workerInput{AtomicTaskID: "planner-1", DAGNodeKey: "images", MaxDynamicTasks: 2}
	registered := func(functionRef string) bool { return functionRef == "image.generate" }
	tests := []struct {
		name       string
		mutate     func(map[string]any)
		registered func(string) bool
		want       string
	}{
		{name: "over limit", mutate: func(map[string]any) { input.MaxDynamicTasks = 1 }, registered: registered, want: "exceeds configured maximum"},
		{name: "duplicate reference", mutate: func(output map[string]any) {
			output["dynamic_tasks"].([]any)[1].(map[string]any)["taskReferenceName"] = "image_0"
		}, registered: registered, want: "is duplicated"},
		{name: "unregistered function", mutate: func(output map[string]any) {
			output["dynamic_tasks"].([]any)[1].(map[string]any)["name"] = "unknown.run"
		}, registered: registered, want: "is not registered"},
		{name: "malformed arguments", mutate: func(output map[string]any) {
			output["dynamic_inputs"].(map[string]any)["image_1"].(map[string]any)["arguments"] = "invalid"
		}, registered: registered, want: "arguments for dynamic task reference"},
		{name: "malformed tasks", mutate: func(output map[string]any) { output["dynamic_tasks"] = "invalid" }, registered: registered, want: "must be an array"},
		{name: "missing paired inputs", mutate: func(output map[string]any) { delete(output, "dynamic_inputs") }, registered: registered, want: "must be returned together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input.MaxDynamicTasks = 2
			output := validOutput()
			tt.mutate(output)
			err := prepareDynamicForkOutput(output, input, tt.registered)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
