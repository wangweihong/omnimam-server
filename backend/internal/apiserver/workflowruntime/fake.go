package workflowruntime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Fake struct {
	mu          sync.Mutex
	definitions map[string]Definition
	executions  map[string]Execution
	schedules   map[string]Schedule
	handlers    map[string]Handler
	nextID      int64
}

func NewFake() *Fake {
	return &Fake{
		definitions: make(map[string]Definition),
		executions:  make(map[string]Execution),
		schedules:   make(map[string]Schedule),
		handlers:    make(map[string]Handler),
	}
}

func (f *Fake) RegisterDefinition(_ context.Context, definition Definition) (Binding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.definitions[definitionKey(definition.Name, definition.Version)] = definition
	return Binding{DefinitionName: definition.Name, DefinitionVersion: definition.Version, Revision: definitionKey(definition.Name, definition.Version)}, nil
}

func (f *Fake) StartExecution(_ context.Context, request StartRequest) (Execution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.definitions[definitionKey(request.DefinitionName, request.DefinitionVersion)]; !ok {
		return Execution{}, fmt.Errorf("workflow definition is not registered")
	}
	for _, execution := range f.executions {
		if request.IdempotencyKey != "" && execution.Output["idempotency_key"] == request.IdempotencyKey {
			return execution, nil
		}
	}
	f.nextID++
	now := time.Now()
	execution := Execution{ID: fmt.Sprintf("fake-execution-%d", f.nextID), Status: "RUNNING", StartedAt: now, Output: map[string]any{"idempotency_key": request.IdempotencyKey}}
	f.executions[execution.ID] = execution
	return execution, nil
}

func (f *Fake) GetExecution(_ context.Context, id string) (Execution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	execution, ok := f.executions[id]
	if !ok {
		return Execution{}, fmt.Errorf("workflow execution not found")
	}
	return execution, nil
}

func (f *Fake) CancelExecution(_ context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	execution, ok := f.executions[id]
	if !ok {
		return fmt.Errorf("workflow execution not found")
	}
	execution.Status = "TERMINATED"
	execution.FailureReason = reason
	execution.CompletedAt = time.Now()
	f.executions[id] = execution
	return nil
}

func (f *Fake) RetryExecution(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	execution, ok := f.executions[id]
	if !ok {
		return fmt.Errorf("workflow execution not found")
	}
	execution.Status = "RUNNING"
	execution.FailureReason = ""
	execution.CompletedAt = time.Time{}
	f.executions[id] = execution
	return nil
}

func (f *Fake) ListNonTerminalExecutions(_ context.Context, limit int) ([]Execution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]Execution, 0)
	for _, execution := range f.executions {
		if execution.Status != "RUNNING" && execution.Status != "PAUSED" {
			continue
		}
		items = append(items, execution)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (f *Fake) SaveSchedule(_ context.Context, schedule Schedule) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.schedules[schedule.Name] = schedule
	return nil
}
func (f *Fake) PauseSchedule(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	schedule, ok := f.schedules[name]
	if !ok {
		return fmt.Errorf("workflow schedule not found")
	}
	schedule.Paused = true
	f.schedules[name] = schedule
	return nil
}
func (f *Fake) ResumeSchedule(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	schedule, ok := f.schedules[name]
	if !ok {
		return fmt.Errorf("workflow schedule not found")
	}
	schedule.Paused = false
	f.schedules[name] = schedule
	return nil
}
func (f *Fake) DeleteSchedule(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.schedules, name)
	return nil
}
func (f *Fake) RegisterHandler(functionRef string, _ int, handler Handler) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if functionRef == "" || handler == nil {
		return fmt.Errorf("function ref and handler are required")
	}
	f.handlers[functionRef] = handler
	return nil
}
func (f *Fake) Close() error { return nil }

func definitionKey(name string, version int) string { return fmt.Sprintf("%s:%d", name, version) }

var _ WorkflowRuntime = (*Fake)(nil)
