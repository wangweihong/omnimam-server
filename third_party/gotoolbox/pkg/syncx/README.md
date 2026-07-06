# syncx — 同步扩展

**功能**：原子布尔操作、互斥锁封装、可重试互斥锁、速率限制并发组、切片并发遍历。

| 函数/方法 | 说明 |
|---|---|
| `IsTrue(v) bool` / `SetTrue(v)` / `SetFalse(v)` | 原子布尔操作 |
| `TrySetTrue(v) bool` | CAS 设置 true |
| `NewMutex() *Mutex` | 创建互斥锁封装 |
| `Mutex.Do(fn)` / `Mutex.DoError(fn) error` | 锁内执行 |
| `LockDo(mu, fn)` | 便捷锁操作 |
| `NewRetryMutex() *RetryMutex` | 创建可重试互斥锁 |
| `RetryMutex.Lock() error` / `Unlock()` | 可重试加锁/解锁 |
| `NewRateLimitGroup(rate) *RateLimitGroup` | 创建速率限制并发组 |
| `RateLimitGroup.Go(fn)` / `SafeGo(fn)` | 启动并发任务 |
| `RateLimitGroup.SafeGoError(fn) error` | 启动并发任务（带错误） |
| `RateLimitGroup.Wait() map[string]*RateGroupResult` | 等待所有结果 |
| `RateLimitGroup.WaitError() error` | 等待并返回首个错误 |
| `Range[T](s, limit, fn) error` | 切片并发遍历 |
[← 返回包列表](../../README.md)
