# statemachine — 状态机

**功能**：通用状态机，支持状态转移规则、历史记录、重置。

| 函数/方法 | 说明 |
|---|---|
| `New(initial State) *StateMachine` | 创建状态机 |
| `StateMachine.AddRule(from, to State)` | 添加转移规则 |
| `StateMachine.Transition(to State) error` | 执行状态转移 |
| `StateMachine.CurrentState() State` | 获取当前状态 |
| `StateMachine.History() []State` | 获取状态历史 |
| `StateMachine.Reset(to State)` | 重置状态机 |

[← 返回包列表](../../README.md)
