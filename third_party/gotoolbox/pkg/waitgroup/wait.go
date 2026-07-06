package waitgroup

import (
	"context"

	gerrors "errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/randutil"
)

type WaitGroupRoutineFunc struct {
	Name string
	//Ctx  context.Context
	// 异步函数体
	Call func(ctx context.Context) Result
}

func NewWaitGroupHandleFunc(name string, call func(ctx context.Context) Result) WaitGroupRoutineFunc {
	return NewFunc(name, call)
}

func NewFunc(name string, call func(ctx context.Context) Result) WaitGroupRoutineFunc {
	if name == "" {
		name = "async-routine"
	}
	name = name + "-" + randutil.RandNumSets(6)
	return WaitGroupRoutineFunc{
		Name: name,
		// Ctx:  ctx,
		Call: call,
	}
}

func NewWaitGroup(ctx context.Context) *Group {
	return &Group{
		ctx:     ctx,
		wg:      sync.WaitGroup{},
		results: make(map[string]Result),
		retLock: sync.Mutex{},
	}
}

// Group allows to start a group of goroutines and wait for their completion.
type Group struct {
	ctx         context.Context
	wg          sync.WaitGroup
	results     map[string]Result
	retLock     sync.Mutex
	debug       bool
	printReturn bool
}

func (g *Group) Debug() *Group {
	g.debug = true
	return g
}

func (g *Group) DebugReturn() *Group {
	g.printReturn = true
	return g
}

func (g *Group) Wait() {
	g.wg.Wait()
	g.PrintResults()
}

// Start starts f in a new goroutine in the group.
// func (g *Group) StartOld(f WaitGroupRoutineFunc) {
// 	g.wg.Add(1)
// 	go func() {
// 		ret := NewResult(nil, nil)
// 		start := time.Now()
// 		defer g.wg.Done()
// 		defer g.setResult(f.Name, &ret, start)
// 		defer g.handleWaitGroupCrash(&ret)
// 		ret = f.Call()
// 	}()
// }

func (g *Group) Start(f WaitGroupRoutineFunc, timeout ...time.Duration) {
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

		resultCh := make(chan Result, 1)
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
		var ret Result
		select {
		case res := <-resultCh:
			ret = res
		case err := <-panicCh:
			ret = NewResult(nil, err)
		case <-ctx.Done():
			// 区分全局超时和任务超时
			// FIXME: 不起作用, 全局超时或者单个超时都只能触发DeadlineExceed
			switch {
			case gerrors.Is(ctx.Err(), context.Canceled):
				ret = NewResult(nil, fmt.Errorf("task canceled by global context"))
			case gerrors.Is(ctx.Err(), context.DeadlineExceeded):
				ret = NewResult(nil, fmt.Errorf("task timed out after %v", time.Since(start).Round(time.Millisecond)))
			default:
				ret = NewResult(nil, fmt.Errorf("task context error: %w", ctx.Err()))
			}
		}
		g.setResult(f.Name, &ret, start)
	}()
}

func (g *Group) setResult(name string, ret *Result, startTime time.Time) {
	ret.Cost = time.Since(startTime)
	g.retLock.Lock()
	defer g.retLock.Unlock()
	g.results[name] = *ret
}

func (g *Group) GetResults() map[string]Result {
	g.retLock.Lock()
	defer g.retLock.Unlock()
	return g.results
}

func (g *Group) GetResultFast() ( /*total*/ int /*success*/, int /*fail*/, int) {
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

func (g *Group) PrintResults() {
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

func (g *Group) handleWaitGroupCrash(st *Result) {
	if x := recover(); x != nil {
		st.Error = errors.Errorf("runtime panic:%v, stack:%v", x, string(debug.Stack()))
	}
}

func (g *Group) ConvertResultToBatchOutput() BatchOutput {
	g.retLock.Lock()
	defer g.retLock.Unlock()

	var bo BatchOutput
	for _, v := range g.results {
		bo.Total += 1
		if v.Error != nil {
			bo.Fail += 1
		} else {
			bo.Success += 1
		}
		bo.Results = append(bo.Results, SetOutput(v.Data, v.Error))
	}
	return bo
}

type Result struct {
	Cost  time.Duration
	Error error
	Data  any
}

func NewResult(data any, err error) Result {
	return Result{
		Data:  data,
		Error: err,
	}
}

func GetResults[T any](wg *Group) []T {
	var metaList []T
	for _, v := range wg.GetResults() {
		if v.Data != nil {
			switch data := v.Data.(type) {
			case []T:
				metaList = append(metaList, data...)
			case T:
				metaList = append(metaList, data)
			}
		}
	}
	return metaList
}

func RunConcurrently[T any](ctx context.Context, inputs []T, task func(context.Context, T) Result, timeouts ...time.Duration) *Group {
	wg := NewWaitGroup(ctx)
	for _, input := range inputs {
		input := input
		wg.Start(NewFunc("", func(ctx context.Context) Result {
			return task(ctx, input)
		}), timeouts...)
	}
	wg.Wait()
	return wg
}

func RunConcurrentlyCondition[T any](ctx context.Context, inputs []T, condition func(T) bool, task func(context.Context, T) Result, timeouts ...time.Duration) *Group {
	wg := NewWaitGroup(ctx)
	for _, input := range inputs {
		if !condition(input) {
			continue
		}
		input := input
		wg.Start(NewFunc("", func(ctx context.Context) Result {
			return task(ctx, input)
		}), timeouts...)
	}
	wg.Wait()
	return wg
}
