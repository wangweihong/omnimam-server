
# log — 日志框架

**功能**：多级别日志记录，支持 logrus、klog、分布式日志、上下文日志、动态级别切换。包含子包 `cronlog`、`distribution`、`klog`、`logrus`。

| 函数/方法 | 说明 |
|---|---|
| `Info(args...)` / `Infof(format, args...)` | Info 级别日志 |
| `Debug(args...)` / `Debugf(format, args...)` | Debug 级别日志 |
| `Warn(args...)` / `Warnf(format, args...)` | Warn 级别日志 |
| `Error(args...)` / `Errorf(format, args...)` | Error 级别日志 |
| `Fatal(args...)` / `Fatalf(format, args...)` | Fatal 级别日志 |
| `WithContext(ctx) *Logger` | 带上下文的日志 |
| `NewLogger(opts) *Logger` | 创建日志器 |
| `SetLevel(level)` | 设置日志级别 |
| `NewOptions() *Options` | 创建日志选项 |
| `FromContext(ctx) *Logger` | 从 context 获取日志器 |
[← 返回包列表](../../README.md)
