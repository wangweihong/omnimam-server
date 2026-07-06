package statemachine

import (
	"fmt"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
)

// 状态类型
type State string

// 状态转移规则映射
type TransitionRules map[State]map[State]bool

// 状态机结构体
type StateMachine struct {
	mutex        sync.RWMutex
	currentState State
	rules        TransitionRules
	stateHistory *sliceutil.FixedSlice[State]
}

// 创建新状态机
func New(initial State) *StateMachine {
	return &StateMachine{
		currentState: initial,
		rules:        make(TransitionRules),
		stateHistory: sliceutil.NewFixedSlice[State](100),
	}
}

// 添加状态转移规则
func (sm *StateMachine) AddRule(from, to State) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.rules[from] == nil {
		sm.rules[from] = make(map[State]bool)
	}
	sm.rules[from][to] = true
}

// 状态转移
func (sm *StateMachine) Transition(to State) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 检查转移是否允许
	if allowed, exists := sm.rules[sm.currentState][to]; !exists || !allowed {
		return fmt.Errorf("invalid transition: %s → %s", sm.currentState, to)
	}
	// 执行状态转移
	sm.stateHistory.Append(sm.currentState)
	sm.currentState = to
	return nil
}

// func (sm *StateMachine) CanTransition(to State) error {
// 	sm.mutex.Lock()
// 	defer sm.mutex.Unlock()

// 	// 检查转移是否允许
// 	if allowed, exists := sm.rules[sm.currentState][to]; !exists || !allowed {
// 		return fmt.Errorf("invalid transition: %s → %s", sm.currentState, to)
// 	}
// 	// 执行状态转移
// 	sm.stateHistory.Append(sm.currentState)
// 	sm.currentState = to
// 	return nil
// }

// 获取当前状态
func (sm *StateMachine) CurrentState() State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.currentState
}

// 获取状态历史
func (sm *StateMachine) History() []State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.stateHistory.GetAll()
}

// 重置状态机
func (sm *StateMachine) Reset(to State) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.currentState = to
	sm.stateHistory = nil
}
