package syncx

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestNewRateLimitGroup(t *testing.T) {
	rg := NewRateLimitGroup(2)

	start := time.Unix(time.Now().Unix(), 0)
	for i := 0; i < 10; i++ {
		rg.Go(func() {
			time.Sleep(time.Second)
		})
	}

	rg.Wait()
	end := time.Unix(time.Now().Unix(), 0)
	if end.Sub(start) != time.Second*5 {
		t.Errorf("rate limit failed: %v", end.Sub(start).String())
	}
}

func TestRateLimitPanic(t *testing.T) {
	rg := NewRateLimitGroup(2)

	for i := 0; i < 10; i++ {
		i := i
		rg.SafeGo(func() {
			time.Sleep(time.Second)
			panic(fmt.Errorf("this is %v panic", i))
		})
	}

	if err := rg.WaitError(); err != nil {
		t.Logf("error: %v", err)
	} else {
		t.Errorf("error is nil")
	}
}

func TestRateLimitError(t *testing.T) {
	rg := NewRateLimitGroup(2)

	for i := 0; i < 10; i++ {
		i := i
		rg.SafeGoError(func() error {
			time.Sleep(time.Second)
			return fmt.Errorf("this is %v error", i)
		})
	}

	if err := rg.WaitError(); err != nil {
		t.Logf("error: %v", err)
	} else {
		t.Errorf("error is nil")
	}
}

func TestRateLimitResult(t *testing.T) {
	rg := NewRateLimitGroup(2)

	for i := 0; i < 10; i++ {
		i := i
		rg.SafeGoResult(func() (interface{}, error) {
			time.Sleep(time.Second)
			return i, nil
		})
		rg.SafeGoResult(func() (interface{}, error) {
			time.Sleep(time.Second)
			return nil, fmt.Errorf("this is %v error", i)
		})
		rg.SafeGoResult(func() (interface{}, error) {
			time.Sleep(time.Second)
			panic(fmt.Errorf("this is %v panic", i))
		})
	}

	results := rg.WaitResult()

	bytes, _ := json.Marshal(results)
	t.Logf("results: %v", string(bytes))
}
