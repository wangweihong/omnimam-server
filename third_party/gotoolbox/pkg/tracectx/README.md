# tracectx — 链路追踪上下文

**功能**：在 context 中存储和传递 Trace ID，支持自动注入。

| 函数/方法 | 说明 |
|---|---|
| `NewTraceID() string` | 生成 Trace ID（格式：trace-id-{pid}-{time}-{seq}） |
| `NewTraceIDContext(ctx, traceID) context.Context` | 创建带 Trace ID 的 context |
| `FromTraceIDContext(ctx) string` | 从 context 中获取 Trace ID |
| `WithTraceIDContext(ctx) context.Context` | 自动注入 Trace ID（如果不存在） |
[← 返回包列表](../../README.md)
