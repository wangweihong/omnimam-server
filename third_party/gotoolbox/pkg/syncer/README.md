# syncer — 同步器

**功能**：定时同步任务框架，支持简单同步、单工作同步、工作队列同步、日期同步。

| 函数/方法 | 说明 |
|---|---|
| `Service.Run(stop <-chan struct{})` | 启动同步服务 |
| `Service.Trigger(arg any, auto bool) bool` | 触发同步 |
| `Service.GetRecords() []SyncInfo` | 获取同步记录 |
| `NewSimpleSyncer(interval, action) *SimpleSyncer` | 创建简单定时同步器 |
| `NewOneWorkerSyncer(...) *OneWorkerSyncer` | 创建单工作同步器 |
| `NewWorkequeueSyncer(...) *WorkequeueSyncer` | 创建工作队列同步器 |
| `NewDateSyncer(...) *DateSyncer` | 创建日期同步器 |

[← 返回包列表](../../README.md)
