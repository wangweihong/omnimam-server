# rate — 令牌桶限流
**功能**：基于令牌桶算法的速率限制器，支持 Allow、Reserve、Wait 三种消费模式。

| 函数/方法 | 说明 |
|---|---|
| `NewLimiter(r Limit, b int) *Limiter` | 创建限流器 |
| `Every(interval) Limit` | 间隔转速率 |
| `Limiter.Allow() bool` | 非阻塞消费 1 个令牌 |
| `Limiter.AllowN(now, n) bool` | 非阻塞消费 n 个令牌 |
| `Limiter.Reserve() *Reservation` | 预留令牌 |
| `Limiter.ReserveN(now, n) *Reservation` | 预留 n 个令牌 |
| `Limiter.Wait(ctx) error` | 阻塞等待令牌 |
| `Limiter.WaitN(ctx, n) error` | 阻塞等待 n 个令牌 |
| `Limiter.Limit() Limit` | 获取速率 |
| `Limiter.Burst() int` | 获取突发容量 |

[← 返回包列表](../../README.md)
