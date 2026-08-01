# Application Platform 能力注册与系统绑定

本文说明 Application Platform 中 `ProviderCapability` 的用途、静态注册来源和 Engine 绑定策略。正式产品语义和接口契约以当前 `SSOT_VERSION` 指向的 application-platform S1/S2 为准。

## 三个正交字段

三个字段分别回答不同问题，不能互相替代：

| 字段 | 取值 | 含义 |
| --- | --- | --- |
| `kind` | `catalog` | 完整 Provider 目录，包含 model、operation、variant 与参数 schema，可用于 Provider ApplicationTemplate 和 RuntimeForm。 |
| `kind` | `engine_binding` | 只标识 EngineType 的基础运行时身份，不声明模型、operation、variant 或参数能力。 |
| `origin` | `static` | 能力由具体协议适配器以不可变 Go 注册随构建交付。 |
| `binding_policy` | `manual` | 管理员可以创建、更新、禁用、收紧或删除兼容绑定。 |
| `binding_policy` | `required_immutable` | 系统为相同 EngineType 的所有实例维护唯一绑定，管理员不能创建、修改、禁用或删除。 |

当前固定组合：

| ProviderCapability | kind | origin | binding_policy |
| --- | --- | --- | --- |
| DeepSeek、ModelArk、OpenAI、xAI、Google | `catalog` | `static` | `manual` |
| Ollama 协议能力 | `catalog` | `static` | `manual` |
| `comfyui-workflow-runtime` | `engine_binding` | `static` | `required_immutable` |
| `runninghub-workflow-runtime` | `engine_binding` | `static` | `required_immutable` |

`origin` 是只读字段。平台不接受 YAML/JSON 清单、目录覆盖、数据库资源、远程目录或运行时编辑；能力变化只能通过适配器代码修改、评审、构建和部署完成。

## 启动组装

API Server 和 TaskWorker 的 bootstrap 显式收集各适配器导出的 CapabilityDefinition、ApplicationEngineType、EngineAdapter、OperationExecutor 和 ProviderCapability。组装依次检查全局 ID、共享能力定义、双语字段、官方 URL、鉴权 schema、模型/operation/variant 关系以及 Adapter/Executor 实现引用。

任一注册不合法都会阻止进程启动，不建立部分 Registry，也不存在 degraded 注册表或文件级加载诊断。Registry 在进程运行期间保持不可变。

## 系统不可变绑定

创建 `application_engine_type_id=comfyui` 或 `runninghub_workflow` 的 EngineInstance 时，服务在同一 PostgreSQL 事务中创建对应的 `comfyui-workflow-runtime` 或 `runninghub-workflow-runtime` 绑定。绑定写入失败时整个事务回滚，不能留下缺失系统绑定的实例。

API Server 和 TaskWorker 启动时都会执行幂等 reconcile，为既有实例补齐适用于其 EngineType 的系统绑定，并将绑定恢复为：

```text
provider_capability_revision = 当前内置 revision
enabled = true
restrictions = {}
system_managed = true（只读派生）
```

多副本依靠 `(engine_instance_id, provider_capability_id)` 唯一索引与条件 upsert 收敛。没有字段漂移时不会增加 `resource_version`。删除无历史运行引用的 EngineInstance 时，绑定通过外键 `ON DELETE CASCADE` 清理。

系统绑定的创建、PATCH、禁用和 DELETE 请求返回 `ERR_AIAPP_SYSTEM_ENGINE_BINDING_IMMUTABLE`。新建实例无法原子创建系统绑定时返回可重试的 `ERR_AIAPP_REQUIRED_ENGINE_BINDING_FAILED`。

## Workflow Contract 边界

`comfyui-workflow-runtime` 和 `runninghub-workflow-runtime` 都不是模型目录，也不能作为 Provider ApplicationTemplate 来源。前者只证明实例属于受支持的 ComfyUI runtime 类型；后者只标识 RunningHub 工作流执行环境，实际 `workflowId`、节点映射和输入输出契约由模板版本与运行快照固定。

具体 ComfyUI 模板和运行能力仍由以下事实共同决定：

```text
API Workflow
∩ workflow contract 与人工映射
∩ 目标 EngineInstance 当前 object_info
∩ enabled / health / stale / template engine restrictions
```

因此，系统绑定不会绕过 Visual-to-API 转换、兼容性校验、输出节点限制、RuntimeForm 或运行前 object_info 校验。
