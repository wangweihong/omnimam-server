
# shutdown — 优雅关闭

**功能**：管理应用优雅关闭，注册回调函数和关闭管理器，支持 POSIX 信号触发。

| 函数/方法 | 说明 |
|---|---|
| `New() *GracefulShutdown` | 创建 GracefulShutdown |
| `GracefulShutdown.Start() error` | 启动关闭监听 |
| `GracefulShutdown.AddShutdownManager(manager)` | 添加关闭管理器 |
| `GracefulShutdown.AddShutdownCallback(callback)` | 添加关闭回调 |
| `GracefulShutdown.StartShutdown(sm)` | 触发关闭 |
| `GracefulShutdown.ReportError(err)` | 报告错误 |
| `GracefulShutdown.SetErrorHandler(handler)` | 设置错误处理器 |
[← 返回包列表](../../README.md)
