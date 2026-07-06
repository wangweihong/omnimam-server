# action — 任务执行框架

**功能**：提供基于管道的任务执行模型，支持任务编排、超时控制、错误处理。

| 函数/方法 | 说明 |
|---|---|
| `NewExecutor()` | 创建执行器 |
| `Executor.AddAction(action *Action)` | 添加执行动作 |
| `Executor.Run() error` | 运行所有动作 |
| `NewAction(name string, fn ActionFunc)` | 创建动作 |
| `Action.WithTimeout(d time.Duration)` | 设置超时 |
| `Action.WithRetry(n int)` | 设置重试次数 |
| `Action.Do() error` | 执行动作 |

[← 返回包列表](../../README.md)
