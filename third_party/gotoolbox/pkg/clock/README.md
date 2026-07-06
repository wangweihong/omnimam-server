
# clock — 时钟抽象

**功能**：提供可注入的时钟接口，支持真实时钟和假时钟（用于测试）。

| 函数/方法 | 说明 |
|---|---|
| `RealClock.Now() time.Time` | 获取当前时间 |
| `RealClock.Since(ts) time.Duration` | 计算时间间隔 |
| `RealClock.After(d) <-chan time.Time` | 定时通道 |
| `RealClock.NewTimer(d) Timer` | 创建定时器 |
| `RealClock.NewTicker(d) Ticker` | 创建周期定时器 |
| `RealClock.Sleep(d)` | 休眠 |
| `NewFakePassiveClock(t) *FakePassiveClock` | 创建假被动时钟 |
| `NewFakeClock(t) *FakeClock` | 创建假时钟 |
| `FakePassiveClock.SetTime(t)` | 设置假时钟时间 |

[← 返回包列表](../../README.md)
