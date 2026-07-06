package syncx

import (
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

type Mutex struct {
	mu sync.Mutex
}

func NewMutex() *Mutex {
	return &Mutex{}
}

func (m *Mutex) DoError(fn func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return fn()
}

func (m *Mutex) Do(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn()
}

func LockDo(mu *sync.Mutex, fn func()) {
	mu.Lock()
	defer mu.Unlock()

	fn()
}

type RetryMutex struct {
	locking int32
	mu      sync.Mutex
}

func NewRetryMutex() *RetryMutex {
	return &RetryMutex{}
}

func (m *RetryMutex) Lock() error {
	if !TrySetTrue(&m.locking) {
		return errors.Errorf("mutex is locking")
	}

	m.mu.Lock()
	return nil
}

func (m *RetryMutex) Unlock() {
	SetFalse(&m.locking)
	m.mu.Unlock()
}

func (m *RetryMutex) LockCantWait() {
	SetTrue(&m.locking)
	m.mu.Lock()
}
