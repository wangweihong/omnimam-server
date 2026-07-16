package taskexecutor

import (
	"context"
	stderrors "errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const (
	internalWorkerID                = "api-internal-worker-v3"
	internalWorkerMaxConcurrency    = 4
	internalDispatcherPollInterval  = time.Second
	internalWorkerHeartbeatInterval = 30 * time.Second
)

// TaskFunctionExecutor 定义领域执行器的最小契约，输入 TaskRun 并返回引用型结果。
type TaskFunctionExecutor interface {
	// Execute 执行一个已领取的 TaskRun；实现必须响应 ctx 取消且不得自行推进 Task Center 状态。
	Execute(ctx context.Context, run *iapiserver.TaskRun) (map[string]any, error)
}

type taskCompletionObserver interface {
	Completed(ctx context.Context, task *iapiserver.TaskRun) error
}

type taskCenterWorkerProtocol interface {
	GetRun(ctx context.Context, id string) (*iapiserver.TaskRun, error)
	HeartbeatWorker(ctx context.Context, req *iapiserver.WorkerHeartbeatRequest) (*iapiserver.Worker, error)
	ClaimRun(ctx context.Context, req *iapiserver.ClaimTaskRunRequest) (*iapiserver.ClaimTaskRunResponse, error)
	UpdateProgress(ctx context.Context, req *iapiserver.ProgressUpdateRequest) (*iapiserver.TaskRun, error)
	CompleteRun(ctx context.Context, req *iapiserver.TaskRunCompleteRequest) (*iapiserver.TaskRun, error)
	FailRun(ctx context.Context, req *iapiserver.TaskRunFailRequest) (*iapiserver.TaskRun, error)
	RenewLease(ctx context.Context, req *iapiserver.LeaseRenewRequest) (*iapiserver.ExecutionLease, error)
}

type Dispatcher struct {
	store        store.Factory
	taskCenter   taskCenterWorkerProtocol
	executors    map[string]TaskFunctionExecutor
	capabilities map[string]struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	wake         chan struct{}
	wg           sync.WaitGroup
	lifecycleMu  sync.Mutex
	started      bool
	closing      bool
	running      atomic.Int64
}

func NewDispatcher(store store.Factory) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		store: store, taskCenter: taskcenter.NewService(store), executors: map[string]TaskFunctionExecutor{},
		capabilities: map[string]struct{}{}, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1),
	}
}

// RegisterCapability 在 Dispatcher 启动前注册领域执行器及领取任务所需能力。
func (d *Dispatcher) RegisterCapability(functionRef, capability string, executor TaskFunctionExecutor) {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()
	if d.started {
		panic("task executors must be registered before dispatcher start")
	}
	if d.executors == nil {
		d.executors = map[string]TaskFunctionExecutor{}
	}
	d.executors[functionRef] = executor
	if capability != "" {
		d.capabilities[capability] = struct{}{}
	}
}

func (d *Dispatcher) Register(functionRef string, executor TaskFunctionExecutor) {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()
	if d.started {
		panic("task executors must be registered before dispatcher start")
	}
	if d.executors == nil {
		d.executors = map[string]TaskFunctionExecutor{}
	}
	d.executors[functionRef] = executor
}

// Start 注册唯一的进程内 Worker，并持续消费新任务和服务重启前遗留的可恢复 TaskRun。
func (d *Dispatcher) Start() error {
	if d == nil || d.store == nil {
		return errors.Errorf("task dispatcher store is required")
	}
	d.lifecycleMu.Lock()
	if d.started {
		d.lifecycleMu.Unlock()
		return nil
	}
	if d.closing {
		d.lifecycleMu.Unlock()
		return errors.Errorf("task dispatcher is closed")
	}
	if len(d.executors) == 0 {
		d.lifecycleMu.Unlock()
		return errors.Errorf("task dispatcher has no executors")
	}
	d.started = true
	d.lifecycleMu.Unlock()

	if err := d.ensureWorker(d.ctx); err != nil {
		d.lifecycleMu.Lock()
		d.started = false
		d.lifecycleMu.Unlock()
		return err
	}
	for range internalWorkerMaxConcurrency {
		d.wg.Add(1)
		go d.runWorker()
	}
	d.wg.Add(1)
	go d.runHeartbeat()
	d.signal()
	return nil
}

// DispatchAsync 在生产者创建 TaskRun 后唤醒常驻 Worker；runID 仅用于拒绝无效通知。
func (d *Dispatcher) DispatchAsync(_ context.Context, runID string) {
	if d == nil || d.store == nil || runID == "" {
		return
	}
	d.signal()
}

// Close 取消进程内执行并等待 Dispatcher 的全部 goroutine 退出。
func (d *Dispatcher) Close() {
	if d == nil {
		return
	}
	d.lifecycleMu.Lock()
	if d.closing {
		d.lifecycleMu.Unlock()
		return
	}
	d.closing = true
	d.cancel()
	d.lifecycleMu.Unlock()
	d.wg.Wait()
}

func (d *Dispatcher) runWorker() {
	defer d.wg.Done()
	ticker := time.NewTicker(internalDispatcherPollInterval)
	defer ticker.Stop()
	for {
		if err := d.claimAndExecute(d.ctx); err != nil && !stderrors.Is(err, context.Canceled) {
			log.Errorf("task worker iteration failed: error=%v", err)
		}
		select {
		case <-d.ctx.Done():
			return
		case <-d.wake:
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) claimAndExecute(ctx context.Context) error {
	for {
		claim, err := d.taskCenter.ClaimRun(ctx, &iapiserver.ClaimTaskRunRequest{
			WorkerID:     internalWorkerID,
			Capabilities: d.workerCapabilities(),
			MaxCount:     1,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if claim.TaskRun == nil {
			return nil
		}
		d.running.Add(1)
		err = d.executeClaim(ctx, claim)
		d.running.Add(-1)
		if err != nil {
			log.Errorf("task execution failed: run_id=%s error=%v", claim.TaskRun.ID, err)
		}
	}
}

func (d *Dispatcher) runHeartbeat() {
	defer d.wg.Done()
	ticker := time.NewTicker(internalWorkerHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if _, err := d.taskCenter.HeartbeatWorker(d.ctx, &iapiserver.WorkerHeartbeatRequest{
				WorkerID: internalWorkerID, Status: iapiserver.WorkerStatusOnline, RunningCount: int(d.running.Load()),
			}); err != nil && !stderrors.Is(err, context.Canceled) {
				log.Errorf("task worker heartbeat failed: error=%v", err)
			}
		}
	}
}

func (d *Dispatcher) signal() {
	d.lifecycleMu.Lock()
	active := d.started && !d.closing
	d.lifecycleMu.Unlock()
	if !active {
		return
	}
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *Dispatcher) ensureWorker(ctx context.Context) error {
	_, err := d.taskCenter.HeartbeatWorker(ctx, &iapiserver.WorkerHeartbeatRequest{
		WorkerID:     internalWorkerID,
		Status:       iapiserver.WorkerStatusOnline,
		RunningCount: 0,
	})
	if err == nil {
		return nil
	}
	worker := &iapiserver.Worker{
		WorkerType:     "api-internal",
		Status:         iapiserver.WorkerStatusOnline,
		Capabilities:   d.workerCapabilities(),
		MaxConcurrency: internalWorkerMaxConcurrency,
	}
	worker.ID = internalWorkerID
	worker.Name = "api-internal"
	if _, registerErr := d.store.TaskCenters().RegisterWorker(ctx, worker); registerErr != nil {
		_, heartbeatErr := d.taskCenter.HeartbeatWorker(ctx, &iapiserver.WorkerHeartbeatRequest{
			WorkerID:     internalWorkerID,
			Status:       iapiserver.WorkerStatusOnline,
			RunningCount: 0,
		})
		return errors.WithStack(heartbeatErr)
	}
	return nil
}

func (d *Dispatcher) executeClaim(ctx context.Context, claim *iapiserver.ClaimTaskRunResponse) error {
	run := claim.TaskRun
	definition, err := d.store.TaskCenters().GetDefinition(ctx, run.DefinitionType, run.DefinitionID)
	if err != nil {
		return errors.WithStack(err)
	}
	executor := d.executors[definition.FunctionRef]
	if executor == nil {
		err = errors.Errorf("task function %s has no internal executor", definition.FunctionRef)
		if _, failErr := d.failRun(ctx, claim, err); failErr != nil {
			return failErr
		}
		return err
	}
	if _, err := d.taskCenter.UpdateProgress(ctx, &iapiserver.ProgressUpdateRequest{
		RunID:         run.ID,
		AttemptID:     claim.Attempt.ID,
		LeaseID:       claim.Lease.ID,
		WorkerID:      internalWorkerID,
		Progress:      0.1,
		ExternalJobID: "api-internal",
	}); err != nil {
		if _, failErr := d.failRun(ctx, claim, err); failErr != nil {
			return failErr
		}
		return errors.WithStack(err)
	}
	executeCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go d.renewLeaseAndWatchCancellation(executeCtx, cancel, claim, done)
	output, err := executor.Execute(executeCtx, run)
	cancel()
	<-done
	if err != nil {
		failed, failErr := d.failRun(ctx, claim, err)
		if failErr != nil {
			return failErr
		}
		if observer, ok := executor.(taskCompletionObserver); ok {
			if observerErr := observer.Completed(ctx, failed); observerErr != nil {
				return errors.WithStack(observerErr)
			}
		}
		return err
	}
	completed, err := d.taskCenter.CompleteRun(ctx, &iapiserver.TaskRunCompleteRequest{
		RunID:         run.ID,
		AttemptID:     claim.Attempt.ID,
		LeaseID:       claim.Lease.ID,
		WorkerID:      internalWorkerID,
		Output:        output,
		ExternalJobID: "api-internal",
	})
	if err != nil {
		return errors.WithStack(err)
	}
	if observer, ok := executor.(taskCompletionObserver); ok {
		return errors.WithStack(observer.Completed(ctx, completed))
	}
	return nil
}

func (d *Dispatcher) workerCapabilities() string {
	items := make([]string, 0, len(d.capabilities))
	for capability := range d.capabilities {
		items = append(items, capability)
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func (d *Dispatcher) renewLeaseAndWatchCancellation(ctx context.Context, cancel context.CancelFunc, claim *iapiserver.ClaimTaskRunResponse, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(iapiserver.DefaultTaskCenterLeaseDuration / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run, err := d.taskCenter.GetRun(ctx, claim.TaskRun.ID)
			if err != nil || run.Status == iapiserver.TaskRunStatusCancelRequested || run.Status == iapiserver.TaskRunStatusCanceled {
				cancel()
				return
			}
			if _, err := d.taskCenter.RenewLease(ctx, &iapiserver.LeaseRenewRequest{LeaseID: claim.Lease.ID, WorkerID: internalWorkerID, AttemptID: claim.Attempt.ID, RunID: claim.TaskRun.ID}); err != nil {
				cancel()
				return
			}
		}
	}
}

func (d *Dispatcher) failRun(ctx context.Context, claim *iapiserver.ClaimTaskRunResponse, cause error) (*iapiserver.TaskRun, error) {
	failureType := iapiserver.FailureTypeFunctionError
	errorCode := "task_execution_failed"
	switch {
	case stderrors.Is(cause, context.Canceled):
		failureType, errorCode = iapiserver.FailureTypeCanceled, "task_execution_canceled"
	case stderrors.Is(cause, context.DeadlineExceeded):
		failureType, errorCode = iapiserver.FailureTypeTimeout, "task_execution_timeout"
	}
	failed, err := d.taskCenter.FailRun(ctx, &iapiserver.TaskRunFailRequest{
		RunID:     claim.TaskRun.ID,
		AttemptID: claim.Attempt.ID,
		LeaseID:   claim.Lease.ID,
		WorkerID:  internalWorkerID,
		Error: iapiserver.TaskError{
			Code:        errorCode,
			Message:     cause.Error(),
			FailureType: failureType,
			Retryable:   false,
			OccurredAt:  imachinery.NewTime(time.Now()),
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return failed, nil
}
