# waitgroup — 泛型等待组

**功能**：泛型等待组，支持为每个任务返回类型化结果。

| 函数/方法 | 说明 |
|---|---|
| `NewGenericGroup[T](ctx) *GenericGroup[T]` | 创建泛型等待组 |
| `GenericGroup[T].Start(fn, timeout...)` | 启动任务 |
| `GenericGroup[T].Wait() map[string]GenericResult[T]` | 等待所有任务并返回结果 |
| `GenericGroup[T].Results() map[string]GenericResult[T]` | 获取结果 |
| `NewWaitGroup() *WaitGroup` | 创建简单等待组 |
[← 返回包列表](../../README.md)
