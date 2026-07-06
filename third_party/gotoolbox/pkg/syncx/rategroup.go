package syncx

import (
	"encoding/json"
	"runtime/debug"
	"sync"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

type (
	RateLimitGroup struct {
		wg   sync.WaitGroup
		mu   sync.Mutex
		rate chan struct{}

		results map[string]*RateGroupResult
	}
	RateGroupResult struct {
		Result interface{}
		Error  error
	}
)

func (r *RateGroupResult) MarshalJSON() ([]byte, error) {
	type marshaller struct {
		Result interface{}
		Error  string
	}
	rr := marshaller{Result: r.Result}
	if r.Error != nil {
		rr.Error = r.Error.Error()
	}
	return json.Marshal(rr)
}

func newUUID() string {
	uid := uuid.New()
	return uid.String()
}

func NewRateLimitGroup(rate int) *RateLimitGroup {
	return &RateLimitGroup{
		wg:      sync.WaitGroup{},
		rate:    make(chan struct{}, rate),
		results: make(map[string]*RateGroupResult),
	}
}

func (r *RateLimitGroup) goresult(id string, safe bool, fn func() (interface{}, error)) {
	r.wg.Add(1)
	r.rate <- struct{}{}

	go func() {
		defer func() {
			r.wg.Done()
			<-r.rate
		}()

		if safe {
			defer r.recover(id)
		}

		result, err := fn()
		r.setresult(id, result, err)
	}()
}

func (r *RateLimitGroup) recover(id string) {
	if re := recover(); re != nil {
		var err error
		log.Errorf("error group panic: %v; stack: %s", re, string(debug.Stack()))
		e, ok := re.(error)
		if ok {
			err = e
		} else {
			err = errors.Errorf("%v", re)
		}

		r.setresult(id, nil, err)
	}
}

func (r *RateLimitGroup) setresult(id string, result interface{}, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.results[id] = &RateGroupResult{
		Result: result,
		Error:  err,
	}
}

func (r *RateLimitGroup) Go(fn func()) {
	r.goresult(newUUID(), false, func() (interface{}, error) {
		fn()
		return nil, nil
	})
}

func (r *RateLimitGroup) SafeGo(fn func()) {
	r.goresult(newUUID(), true, func() (interface{}, error) {
		fn()
		return nil, nil
	})
}

func (r *RateLimitGroup) GoError(fn func() error) {
	r.goresult(newUUID(), false, func() (interface{}, error) {
		return nil, fn()
	})
}

func (r *RateLimitGroup) SafeGoError(fn func() error) {
	r.goresult(newUUID(), true, func() (interface{}, error) {
		return nil, fn()
	})
}

func (r *RateLimitGroup) GoResult(fn func() (interface{}, error)) {
	r.goresult(newUUID(), false, fn)
}

func (r *RateLimitGroup) SafeGoResult(fn func() (interface{}, error)) {
	r.goresult(newUUID(), true, fn)
}

func (r *RateLimitGroup) Wait() {
	r.wg.Wait()
}

func (r *RateLimitGroup) WaitError() error {
	r.wg.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, result := range r.results {
		if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

func (r *RateLimitGroup) WaitResult() []*RateGroupResult {
	r.wg.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()
	results := make([]*RateGroupResult, 0, len(r.results))
	for _, result := range r.results {
		results = append(results, result)
	}
	return results
}

func (r *RateLimitGroup) LockDoErr(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return fn()
}

func (r *RateLimitGroup) LockDo(fn func()) {
	_ = r.LockDoErr(func() error {
		fn()
		return nil
	})
}
