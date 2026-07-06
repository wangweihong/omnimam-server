# wait — 等待/定时工具

**功能**：定时循环执行、Jitter 抖动、panic 恢复、并发组、退避管理器、时钟抽象。

| 函数/方法 | 说明 |
|---|---|
| `Forever(fn, period)` | 永久循环执行 |
| `Until(fn, period, stopCh)` | 直到停止信号循环执行 |
| `UntilWithContext(ctx, fn, period)` | 直到 context 取消循环执行 |
| `NonSlidingUntil(fn, period, stopCh)` | 非滑动窗口循环 |
| `NonSlidingUntilWithContext(ctx, fn, period)` | 非滑动窗口 + context |
| `JitterUntil(fn, period, jitterFactor, sliding, stopCh)` | 带抖动的循环执行 |
| `JitterUntilWithContext(ctx, fn, period, jitterFactor, sliding)` | 带抖动 + context |
| `Group.Start(fn)` | 启动 goroutine |
| `Group.StartWithChannel(stopCh, fn)` | 带 channel 启动 |
| `Group.StartWithContext(ctx, fn)` | 带 context 启动 |
| `Group.Wait()` | 等待所有 goroutine |
| `HandleCrash(handlers...)` | Panic 恢复处理 |
| `BackoffManager` | 退避管理器 |
| `ExponentialBackoff(...)` | 指数退避 |
| `NewFakeClock(t)` / `NewFakePassiveClock(t)` | 假时钟（测试用） |
[← 返回包列表](../../README.md)
