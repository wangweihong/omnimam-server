package waitgroup

import (
	"context"
	gerrors "errors"
	"fmt"
	"maps"
	"runtime/debug"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/randutil"
)

type GenericFunc[T any] struct {
	Name string
	// 异步函数体
	Call func(context.Context) GenericResult[T]
}

func NewGenericFunc[T any](name string, call func(ctx context.Context) GenericResult[T]) GenericFunc[T] {
	if name == "" {
		name = "async-routine" + "-" + randutil.RandNumSets(6)
	}
	return GenericFunc[T]{
		Name: name,
		Call: call,
	}
}

type GenericResult[T any] struct {
	Cost  time.Duration
	Data  T
	Error error
}

type GenericGroup[T any] struct {
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	results     map[string]GenericResult[T]
	retLock     sync.Mutex
	debug       bool
	printReturn bool
}

func NewGenericGroup[T any](ctx context.Context) *GenericGroup[T] {
	return &GenericGroup[T]{
		ctx:     ctx,
		results: make(map[string]GenericResult[T]),
	}
}

func (g *GenericGroup[T]) WithGlobalTimeout(timeout time.Duration) {
	g.retLock.Lock()
	defer g.retLock.Unlock()

	if g.cancel != nil {
		g.cancel()
	}

	g.ctx, g.cancel = context.WithTimeout(g.ctx, timeout)
}

func (g *GenericGroup[T]) Debug() *GenericGroup[T] {
	g.debug = true
	return g
}

func (g *GenericGroup[T]) DebugReturn() *GenericGroup[T] {
	g.printReturn = true
	return g
}

func (g *GenericGroup[T]) Wait() {
	g.wg.Wait()
	g.PrintResults()

	// 全局超时后释放资源
	if g.cancel != nil {
		g.cancel()
	}
	//return g.GetResults()
}

func NewGenericResult[T any](data T, err error) GenericResult[T] {
	return GenericResult[T]{
		Data:  data,
		Error: err,
	}
}

// // Start starts f in a new goroutine in the group.
// func (g *GenericGroup[T]) Start(f GenericFunc[T]) {
// 	g.wg.Add(1)
// 	go func() {
// 		var zero T
// 		ret := NewGenericResult[T](zero, nil)
// 		start := time.Now()
// 		defer g.wg.Done()
// 		defer g.setResult(f.Name, &ret, start)
// 		defer g.handleWaitGroupCrash(&ret)
// 		ret = f.Call()
// 	}()
// }

func (g *GenericGroup[T]) Start(f GenericFunc[T], timeout ...time.Duration) {
	g.wg.Add(1)
	go func() {
		var cancel context.CancelFunc
		var ctx context.Context
		if len(timeout) > 0 && timeout[0] > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), timeout[0])
		} else {
			ctx, cancel = context.WithCancel(g.ctx)
		}
		defer cancel()

		resultCh := make(chan GenericResult[T], 1)
		panicCh := make(chan error, 1)
		start := time.Now()
		defer g.wg.Done()

		go func() {
			defer func() {
				if x := recover(); x != nil {
					err := errors.Errorf("runtime panic:%v, stack:%v", x, string(debug.Stack()))
					panicCh <- err
				}
			}()

			resultCh <- f.Call(ctx)
		}()
		var ret GenericResult[T]
		select {
		case res := <-resultCh:
			ret = res
		case err := <-panicCh:
			var zero T
			ret = NewGenericResult[T](zero, err)
		case <-ctx.Done():
			var zero T
			// 区分全局超时和任务超时
			// FIXME: 不起作用, 全局超时或者单个超时都只能触发DeadlineExceed
			switch {
			case gerrors.Is(ctx.Err(), context.Canceled):
				ret = NewGenericResult(zero, errors.Errorf("task canceled by global context"))
			case gerrors.Is(ctx.Err(), context.DeadlineExceeded):
				ret = NewGenericResult(zero, errors.Errorf("task timed out after %v", time.Since(start).Round(time.Millisecond)))
			default:
				ret = NewGenericResult(zero, errors.Errorf("task context error: %w", ctx.Err()))
			}
		}
		g.setResult(f.Name, &ret, start)
	}()
}

func (g *GenericGroup[T]) handleWaitGroupCrash(st *GenericResult[T]) {
	if x := recover(); x != nil {
		st.Error = errors.Errorf("runtime panic:%v, stack:%v", x, string(debug.Stack()))
	}
}

func (g *GenericGroup[T]) PrintResults() {
	g.retLock.Lock()
	defer g.retLock.Unlock()
	if g.debug {
		fmt.Printf("has start %v waitgroup routines\n", len(g.results))
		for k, v := range g.results {
			if g.printReturn {
				fmt.Printf("waitgroup routine %v, results:%v, cost:%vs \n", k, v, v.Cost.Seconds())
			} else {
				fmt.Printf("waitgroup routine %v, cost:%vs \n", k, v.Cost.Seconds())
			}
		}
	}
}

func (g *GenericGroup[T]) setResult(name string, ret *GenericResult[T], startTime time.Time) {
	ret.Cost = time.Since(startTime)
	g.retLock.Lock()
	defer g.retLock.Unlock()
	g.results[name] = *ret
}

func (g *GenericGroup[T]) GetResults() map[string]GenericResult[T] {
	g.retLock.Lock()
	defer g.retLock.Unlock()
	return maps.Clone(g.results)
}

func (g *GenericGroup[T]) GetSuccessResultList() []T {
	g.retLock.Lock()
	defer g.retLock.Unlock()

	list := make([]T, 0, len(g.results))
	for _, v := range g.results {
		if v.Error == nil {
			list = append(list, v.Data)
		}
	}
	return list
}

func (g *GenericGroup[T]) GetErrorList() errors.Aggregate {
	g.retLock.Lock()
	defer g.retLock.Unlock()

	list := make([]error, 0, len(g.results))
	for _, v := range g.results {
		if v.Error != nil {
			list = append(list, v.Error)
		}
	}
	return errors.NewAggregate(list...)
}

func (g *GenericGroup[T]) GetResultFast() ( /*total*/ int /*success*/, int /*fail*/, int) {
	g.retLock.Lock()
	defer g.retLock.Unlock()

	total := len(g.results)
	var fail int
	var success int
	for _, v := range g.results {
		if v.Error != nil {
			fail += 1
		} else {
			success += 1
		}
	}
	return total, success, fail
}

func (g *GenericGroup[T]) BatchGenericOutput() BatchGenericOutput[T] {
	g.retLock.Lock()
	defer g.retLock.Unlock()

	var b BatchGenericOutput[T]
	for _, v := range g.results {
		SetBatchGenericOutput(b, v.Data, v.Error)
	}
	return b
}

func RunGenericConcurrently[T any, R any](ctx context.Context, inputs []T, task func(context.Context, T) GenericResult[R], timeouts ...time.Duration) *GenericGroup[R] {
	wg := NewGenericGroup[R](ctx)
	for _, input := range inputs {
		input := input
		wg.Start(NewGenericFunc("", func(ctx context.Context) GenericResult[R] {
			return task(ctx, input)
		}), timeouts...)
	}
	wg.Wait()
	return wg
}

func RunGenericConcurrentlyCondition[T any, R any](ctx context.Context, inputs []T, condition func(T) bool, task func(context.Context, T) GenericResult[R], timeouts ...time.Duration) *GenericGroup[R] {
	wg := NewGenericGroup[R](ctx)
	for _, input := range inputs {
		if !condition(input) {
			continue
		}
		input := input
		wg.Start(NewGenericFunc("", func(ctx context.Context) GenericResult[R] {
			return task(ctx, input)
		}), timeouts...)
	}
	wg.Wait()
	return wg
}
