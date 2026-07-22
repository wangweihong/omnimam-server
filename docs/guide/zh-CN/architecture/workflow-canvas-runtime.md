# Workflow Canvas v1.7 运行架构

## 目标与边界

Workflow Canvas 保存 NodeDefinition、Canvas 草稿、不可变 CanvasVersion、ExecutionPlan 和面向画布的运行投影。Task Center 与 Conductor 继续拥有 DAGTaskGroup、AtomicTask、TaskAttempt、调度、自动重试和取消事实；Asset Library 拥有 Artifact 正文与可用性；SSE 只投影 Canvas 可靠事件。

本实现参考了以下开源项目的成熟模式，但没有复制其私有领域对象：

- ComfyUI：节点能力目录、稳定输入输出端口、图快照和交互控制状态。
- n8n：partial execution、明确运行范围和历史结果复用语义。
- Dify：WorkflowRun/NodeRun 分层投影、进度和增量输出视图。
- Netflix Conductor：持久 DAG、Worker 分发、重试、取消和恢复；本仓库继续复用已有 Conductor adapter。

## 发布路径

1. Canvas 草稿以 `expected_draft_revision` 做乐观并发。
2. 发布前校验节点、边、流、无环、规模、NodeDefinition 可见性、JSON Schema 和受控执行绑定。
3. 规范化图使用 JCS + SHA256 生成内容摘要。
4. workflow definition 名称和整数版本从内容摘要确定，重复发布不会生成漂移定义。
5. CanvasVersion 冻结图、NodeDefinition 快照、输入输出 schema、编译摘要和 Task Center definition binding。

## 运行路径

`all`、`flows`、`only_nodes`、`until_nodes` 和 `from_nodes` 首先解析为确定性子图和 FlowRun execution keys。CanvasRun 在调用 Task Center 前持久化版本、输入、scope、策略、请求摘要和 ExecutionPlan；随后以稳定幂等键创建唯一 DAGTaskGroup。

每个选中节点形成 CanvasNodeRun。普通节点通常绑定一个 AtomicTask；expanded 节点允许绑定多个 AtomicTask；passive/client-generated/reused 节点允许没有新任务。TaskBinding 保存 Task Center 稳定 ID、child key、角色、shard 和已观察资源版本。OutputBinding 预先固定端口、producer key、required、shard 和 Artifact/结构化值槽位。

## 投影与事件

Task Center 状态事件先按 TaskBinding 的 `task_resource_version` 单调推进，再聚合 NodeRun、FlowRun 和 CanvasRun。每个业务聚合使用独立 `aggregate_version`，不能与 AtomicTask 或 Artifact 版本直接比较。

Canvas 事实、`workflow_canvas_outbox` 审计记录和 Watermill PostgreSQL 可靠消息在同一事务提交。SSE projector 消费 `canvas_run_*`、`canvas_node_run_status_changed` 和 `canvas_node_output_available`，映射为统一用户连接上的 `canvas.run.*` 与 `canvas.node.*`。

## 升级兼容

启动迁移会把旧 CanvasVersion 的 `compiled_definition_*` 回填到 v1.7 workflow definition 字段，把旧 CanvasNodeRun 的 `node_key/atomic_task_id` 回填为 `node_id/execution_key` 和 TaskBinding，并为旧运行补默认 `all + rerun_all + continue_independent_flows` 策略。旧历史记录保留，不重开 AtomicTask 或 TaskAttempt。

## 当前后续项

- `reuse_valid_outputs/reuse_required` 的跨运行 Artifact 权限、TTL 和输出完整性查询仍需接入 Asset Library 批量摘要后才能启用真实复用；当前 `reuse_required` 会明确失败，不会静默重跑。
- Artifact 事件驱动的 OutputBinding READY/FAILED 投影和持久 reconcile cursor 需要在 Asset Library 提供 Canvas consumer 后补齐。
- workflow-canvas 权限码已由 SSOT 定义；当前仓库统一认证中间件尚未提供细粒度 permission evaluator，后续需在共享授权层接入，不能在 service 内硬编码角色判断。
