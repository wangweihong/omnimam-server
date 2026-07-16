package taskexecutor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestDispatcherStartsOnceAndRecoversQueuedRun(t *testing.T) {
	run := &iapiserver.TaskRun{DefinitionType: iapiserver.TaskDefinitionTypeAtomic, DefinitionID: "test-definition"}
	run.ID = "run-1"
	protocol := &fakeWorkerProtocol{run: run, completed: make(chan struct{})}
	factory := &dispatcherFactory{tasks: &dispatcherTaskStore{definition: &iapiserver.TaskDefinition{FunctionRef: "test.execute"}}}
	dispatcher := NewDispatcher(factory)
	dispatcher.taskCenter = protocol
	dispatcher.RegisterCapability("test.execute", "test", executorFunc(func(context.Context, *iapiserver.TaskRun) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}))

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("start dispatcher twice: %v", err)
	}
	select {
	case <-protocol.completed:
	case <-time.After(2 * time.Second):
		t.Fatal("queued task was not recovered on dispatcher start")
	}
	dispatcher.Close()
	dispatcher.Close()

	protocol.mu.Lock()
	defer protocol.mu.Unlock()
	if protocol.claimed != 1 {
		t.Fatalf("claimed = %d, want 1", protocol.claimed)
	}
	if protocol.progressUpdates != 1 || protocol.completions != 1 {
		t.Fatalf("progress updates = %d, completions = %d", protocol.progressUpdates, protocol.completions)
	}
}

func TestDispatcherLimitsConcurrentExecutions(t *testing.T) {
	const taskTotal = internalWorkerMaxConcurrency + 2
	protocol := newConcurrentWorkerProtocol(taskTotal)
	factory := &dispatcherFactory{tasks: &dispatcherTaskStore{definition: &iapiserver.TaskDefinition{FunctionRef: "test.execute"}}}
	dispatcher := NewDispatcher(factory)
	dispatcher.taskCenter = protocol
	release := make(chan struct{})
	var active atomic.Int64
	var maximum atomic.Int64
	dispatcher.RegisterCapability("test.execute", "test", executorFunc(func(context.Context, *iapiserver.TaskRun) (map[string]any, error) {
		current := active.Add(1)
		for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
		}
		protocol.started <- struct{}{}
		<-release
		active.Add(-1)
		return map[string]any{"ok": true}, nil
	}))

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	for range internalWorkerMaxConcurrency {
		select {
		case <-protocol.started:
		case <-time.After(2 * time.Second):
			t.Fatal("workers did not reach configured concurrency")
		}
	}
	select {
	case <-protocol.started:
		t.Fatal("dispatcher exceeded configured concurrency")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case <-protocol.completed:
	case <-time.After(2 * time.Second):
		t.Fatal("queued tasks did not complete after workers were released")
	}
	dispatcher.Close()
	if got := maximum.Load(); got != internalWorkerMaxConcurrency {
		t.Fatalf("maximum concurrency = %d, want %d", got, internalWorkerMaxConcurrency)
	}
}

type executorFunc func(context.Context, *iapiserver.TaskRun) (map[string]any, error)

func (f executorFunc) Execute(ctx context.Context, run *iapiserver.TaskRun) (map[string]any, error) {
	return f(ctx, run)
}

type dispatcherFactory struct {
	store.Factory
	tasks *dispatcherTaskStore
}

func (f *dispatcherFactory) TaskCenters() store.TaskCenterStore { return f.tasks }

type dispatcherTaskStore struct {
	store.TaskCenterStore
	definition *iapiserver.TaskDefinition
}

func (s *dispatcherTaskStore) GetDefinition(context.Context, string, string) (*iapiserver.TaskDefinition, error) {
	return s.definition, nil
}

type fakeWorkerProtocol struct {
	mu              sync.Mutex
	run             *iapiserver.TaskRun
	claimed         int
	progressUpdates int
	completions     int
	completed       chan struct{}
}

type concurrentWorkerProtocol struct {
	mu        sync.Mutex
	runs      []*iapiserver.TaskRun
	next      int
	done      int
	started   chan struct{}
	completed chan struct{}
}

func newConcurrentWorkerProtocol(total int) *concurrentWorkerProtocol {
	protocol := &concurrentWorkerProtocol{
		runs: make([]*iapiserver.TaskRun, total), started: make(chan struct{}, total), completed: make(chan struct{}),
	}
	for i := range total {
		run := &iapiserver.TaskRun{DefinitionType: iapiserver.TaskDefinitionTypeAtomic, DefinitionID: "test-definition"}
		run.ID = "run-" + string(rune('a'+i))
		protocol.runs[i] = run
	}
	return protocol
}

func (p *concurrentWorkerProtocol) GetRun(context.Context, string) (*iapiserver.TaskRun, error) {
	return nil, nil
}

func (p *concurrentWorkerProtocol) HeartbeatWorker(context.Context, *iapiserver.WorkerHeartbeatRequest) (*iapiserver.Worker, error) {
	return &iapiserver.Worker{}, nil
}

func (p *concurrentWorkerProtocol) ClaimRun(context.Context, *iapiserver.ClaimTaskRunRequest) (*iapiserver.ClaimTaskRunResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.next >= len(p.runs) {
		return &iapiserver.ClaimTaskRunResponse{}, nil
	}
	run := p.runs[p.next]
	p.next++
	attempt := &iapiserver.TaskAttempt{}
	attempt.ID = run.ID + "-attempt"
	lease := &iapiserver.ExecutionLease{}
	lease.ID = run.ID + "-lease"
	return &iapiserver.ClaimTaskRunResponse{TaskRun: run, Attempt: attempt, Lease: lease}, nil
}

func (p *concurrentWorkerProtocol) UpdateProgress(context.Context, *iapiserver.ProgressUpdateRequest) (*iapiserver.TaskRun, error) {
	return &iapiserver.TaskRun{}, nil
}

func (p *concurrentWorkerProtocol) CompleteRun(context.Context, *iapiserver.TaskRunCompleteRequest) (*iapiserver.TaskRun, error) {
	p.mu.Lock()
	p.done++
	if p.done == len(p.runs) {
		close(p.completed)
	}
	p.mu.Unlock()
	return &iapiserver.TaskRun{}, nil
}

func (p *concurrentWorkerProtocol) FailRun(context.Context, *iapiserver.TaskRunFailRequest) (*iapiserver.TaskRun, error) {
	return &iapiserver.TaskRun{}, nil
}

func (p *concurrentWorkerProtocol) RenewLease(context.Context, *iapiserver.LeaseRenewRequest) (*iapiserver.ExecutionLease, error) {
	return &iapiserver.ExecutionLease{}, nil
}

func (p *fakeWorkerProtocol) GetRun(context.Context, string) (*iapiserver.TaskRun, error) {
	return p.run, nil
}

func (p *fakeWorkerProtocol) HeartbeatWorker(context.Context, *iapiserver.WorkerHeartbeatRequest) (*iapiserver.Worker, error) {
	return &iapiserver.Worker{}, nil
}

func (p *fakeWorkerProtocol) ClaimRun(context.Context, *iapiserver.ClaimTaskRunRequest) (*iapiserver.ClaimTaskRunResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.claimed > 0 {
		return &iapiserver.ClaimTaskRunResponse{}, nil
	}
	p.claimed++
	attempt := &iapiserver.TaskAttempt{}
	attempt.ID = "attempt-1"
	lease := &iapiserver.ExecutionLease{}
	lease.ID = "lease-1"
	return &iapiserver.ClaimTaskRunResponse{TaskRun: p.run, Attempt: attempt, Lease: lease}, nil
}

func (p *fakeWorkerProtocol) UpdateProgress(context.Context, *iapiserver.ProgressUpdateRequest) (*iapiserver.TaskRun, error) {
	p.mu.Lock()
	p.progressUpdates++
	p.mu.Unlock()
	return p.run, nil
}

func (p *fakeWorkerProtocol) CompleteRun(context.Context, *iapiserver.TaskRunCompleteRequest) (*iapiserver.TaskRun, error) {
	p.mu.Lock()
	p.completions++
	if p.completions == 1 {
		close(p.completed)
	}
	p.mu.Unlock()
	return p.run, nil
}

func (p *fakeWorkerProtocol) FailRun(context.Context, *iapiserver.TaskRunFailRequest) (*iapiserver.TaskRun, error) {
	return p.run, nil
}

func (p *fakeWorkerProtocol) RenewLease(context.Context, *iapiserver.LeaseRenewRequest) (*iapiserver.ExecutionLease, error) {
	return &iapiserver.ExecutionLease{}, nil
}
