# errors — 增强错误处理

**功能**：提供带堆栈跟踪、错误码、聚合错误、状态码的增强错误处理。灵感来源于 `github.com/pkg/errors`。

| 函数/方法 | 说明 |
|---|---|
| `New(message) error` | 创建带堆栈的错误 |
| `Errorf(format, args...) error` | 格式化创建带堆栈的错误 |
| `WithStack(err) error` | 为错误附加堆栈 |
| `Wrap(err, message) error` | 包装错误并附加堆栈 |
| `Wrapf(err, format, args...) error` | 格式化包装错误 |
| `NewAggregate(errlist) Aggregate` | 创建聚合错误 |
| `NewCode(code, message) *Code` | 创建带错误码的错误 |
| `NewMessage(message) *Message` | 创建带消息的错误 |
| `NewStatus(code, message) *Status` | 创建带状态码的错误 |
| `Status.Code() int` | 获取状态码 |
| `Status.Message() string` | 获取状态消息 |
| `Trim(err) error` | 截断错误信息（保留顶层堆栈） |
[← 返回包列表](../../README.md)
