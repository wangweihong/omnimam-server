# Application Platform 能力注册与系统绑定

本文说明 Application Platform 中 `ProviderCapability` 的用途、加载来源和 Engine 绑定策略。正式产品语义和接口契约以当前 `SSOT_VERSION` 指向的 application-platform S1/S2 为准。

## 三个正交字段

三个字段分别回答不同问题，不能互相替代：

| 字段 | 取值 | 含义 |
| --- | --- | --- |
| `kind` | `catalog` | 完整 Provider 目录，包含 model、operation、variant 与参数 schema，可用于 Provider ApplicationTemplate 和 RuntimeForm。 |
| `kind` | `engine_binding` | 只标识 EngineType 的基础运行时身份，不声明模型、operation、variant 或参数能力。 |
| `origin` | `builtin` | 清单编译进服务；字段由加载器派生，内置清单无效时服务拒绝启动。 |
| `origin` | `directory` | 清单来自 `provider_capability_directory`；目录失败时 registry degraded，但已加载的 builtin 能力保留。 |
| `binding_policy` | `manual` | 管理员可以创建、更新、禁用、收紧或删除兼容绑定。 |
| `binding_policy` | `required_immutable` | 系统为相同 EngineType 的所有实例维护唯一绑定，管理员不能创建、修改、禁用或删除。 |

当前固定组合：

| ProviderCapability | kind | origin | binding_policy |
| --- | --- | --- | --- |
| `deepseek-official` | `catalog` | `directory` | `manual` |
| `seedance-byteplus` | `catalog` | `directory` | `manual` |
| `comfyui-workflow-runtime` | `engine_binding` | `builtin` | `required_immutable` |

`origin` 不属于 YAML 可写字段。外部清单不能声明自己是 builtin，也不能使用 `comfyui-workflow-runtime` 等内置保留 ID。保留 ID 冲突只隔离外部文件，不会覆盖或禁用内置能力。

## 启动加载

服务先加载 embedded ProviderCapability，再扫描配置目录第一层的 `.yaml` 与 `.yml` 文件。内置清单执行严格校验，失败会阻止进程启动；目录不可读或单文件无效不会阻止启动，只产生 degraded 状态或文件级诊断。

Registry 在进程运行期间不可变。目录 catalog 只有替换 YAML 并重启后才会加载新 revision。内置能力随服务版本交付，不能由部署目录修改。

## ComfyUI 系统绑定

创建 `application_engine_type_id=comfyui` 的 EngineInstance 时，服务在同一 PostgreSQL 事务中创建 `comfyui-workflow-runtime` 绑定。绑定写入失败时整个事务回滚，不能留下无系统绑定的实例。

API Server 和 TaskWorker 启动时都会执行幂等 reconcile，为既有 ComfyUI 实例补齐绑定，并将系统绑定恢复为：

```text
provider_capability_revision = 当前内置 revision
enabled = true
restrictions = {}
system_managed = true（只读派生）
```

多副本依靠 `(engine_instance_id, provider_capability_id)` 唯一索引与条件 upsert 收敛。没有字段漂移时不会增加 `resource_version`。删除无历史运行引用的 EngineInstance 时，绑定通过外键 `ON DELETE CASCADE` 清理。

系统绑定的创建、PATCH、禁用和 DELETE 请求返回 `ERR_AIAPP_SYSTEM_ENGINE_BINDING_IMMUTABLE`。新建实例无法原子创建系统绑定时返回可重试的 `ERR_AIAPP_REQUIRED_ENGINE_BINDING_FAILED`。

## Workflow Contract 边界

`comfyui-workflow-runtime` 不是 ComfyUI 模型目录，也不能作为 Provider ApplicationTemplate 来源。它只证明实例属于受支持的 ComfyUI runtime 类型。

具体 ComfyUI 模板和运行能力仍由以下事实共同决定：

```text
API Workflow
∩ workflow contract 与人工映射
∩ 目标 EngineInstance 当前 object_info
∩ enabled / health / stale / template engine restrictions
```

因此，系统绑定不会绕过 Visual-to-API 转换、兼容性校验、输出节点限制、RuntimeForm 或运行前 object_info 校验。
