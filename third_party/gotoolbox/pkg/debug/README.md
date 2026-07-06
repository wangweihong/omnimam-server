# debug — 调试工具

**功能**：提供 pprof 性能分析（CPU/Mem/Block/Goroutine）、运行时信号处理、动态调试标志。

| 函数/方法 | 说明 |
|---|---|
| `StartProf(dir) error` | 启动性能分析，采集所有 profile |
| `SetupRuntimeDebugSignalHandler(outputDir)` | 安装调试信号处理器 |
| `Dynamic` (var) | 动态控制标志 |

[← 返回包列表](../../README.md)
