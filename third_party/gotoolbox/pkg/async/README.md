# async — 异步并发工具

**功能**：提供协程管理、异步任务执行、panic 恢复、等待组等功能。

| 函数/方法 | 说明 |
|---|---|
| `Run(fn AsyncFunc) <-chan Result` | 异步执行函数，返回 channel 接收结果 |
| `GoRoutine(fn func())` | 安全启动 goroutine，带 panic 恢复 |
| `NewWaitGroup() *WaitGroup` | 创建等待组 |
| `WaitGroup.Wait()` | 等待所有任务完成 |
| `WaitGroup.Start(fn func())` | 启动任务 |
| `Handler.Run()` | 处理器执行 |
[← 返回包列表](../../README.md)
