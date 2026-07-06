# workqueue — 工作队列

**功能**：带去重的工作队列，支持延迟队列、速率限制队列、指标采集。灵感来源于 Kubernetes client-go。

| 函数/方法 | 说明 |
|---|---|
| `NewQueue(name) *Type` | 创建基础工作队列 |
| `NewDelayingQueue(name) *delayingType` | 创建延迟工作队列 |
| `NewRateLimitingQueue(r) *rateLimitingType` | 创建速率限制工作队列 |
| `Interface.Add(item)` | 添加工作项 |
| `Interface.Get() (item, shutdown)` | 获取工作项 |
| `Interface.Done(item)` | 标记完成 |
| `Interface.Len() int` | 队列长度 |
| `Interface.ShutDown()` | 关闭队列 |
| `Interface.ShuttingDown() bool` | 是否正在关闭 |
| `DelayingInterface.AddAfter(item, duration)` | 延迟添加 |
| `RateLimitingInterface.AddRateLimited(item)` | 限速添加 |
| `RateLimitingInterface.Forget(item)` | 忘记重试次数 |
| `RateLimitingInterface.NumRequeues(item) int` | 获取重试次数 |
| `NewDefaultRateLimiter() *DefaultRateLimiter` | 创建默认速率限制器 |

[← 返回包列表](../../README.md)
