# stage — 阶段执行

**功能**：定义和执行分阶段任务，支持超时、追踪、导出。

| 函数/方法 | 说明 |
|---|---|
| `NewExecuteStage(name, nameCN, run) Stage` | 创建执行阶段 |
| `Stage` (struct) | 阶段结构体（Name, StartTime, EndTime, Success, ErrorMessage 等） |
| `NewStageController() *StageController` | 创建阶段控制器 |
| `StageController.Run(ctx) error` | 运行阶段 |
| `NewExporter() *Exporter` | 创建导出器 |

[← 返回包列表](../../README.md)
