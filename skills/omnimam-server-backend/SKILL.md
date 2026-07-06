# OmniMAM Server Backend Skill

本 skill 用于指导 `omnimam-server` 仓库中的后端实现任务。

适用任务：

```text
API 实现
业务逻辑实现
数据库 migration
权限校验
错误码返回
事件生产与消费
任务调度
模型调用
存储访问
provider / runtime 适配
后端测试
```

---

## 1. 必须遵循的上游规则

执行任何后端任务前，必须遵循：

```text
1. ssot/ 中已 release 的 S1/S2
2. backend/AGENTS.md
3. 本 skill
```

其中：

```text
S1 = 产品语义事实源
S2 = 实现契约事实源
backend/AGENTS.md = 后端代码实现规则
```

---

## 2. 必须读取的 SSOT 文件

实现某个 domain 前，必须读取：

```text
ssot/00_product/domains/<domain_id>/product-spec.md
ssot/01_contracts/domains/<domain_id>/openapi.yaml
ssot/01_contracts/domains/<domain_id>/schema.sql
ssot/01_contracts/domains/<domain_id>/errors.yaml
ssot/01_contracts/domains/<domain_id>/permissions.yaml
ssot/01_contracts/domains/<domain_id>/events.yaml
ssot/01_contracts/domains/<domain_id>/module-contract.md
ssot/02_architecture/domains/<domain_id>.md
```

不能只读取 S2。
业务语义、状态流转、规则和验收标准必须参考 S1。
接口、数据库、错误码、权限码、事件和模块边界必须参考 S2。

---

## 3. backend/AGENTS.md 约束

后端实现必须遵循：

```text
backend/AGENTS.md
```

包括但不限于：

```text
项目结构规则
编码规则
接口实现规则
数据库访问规则
migration 规则
错误码规则
权限校验规则
事件规则
模块边界规则
provider / runtime 抽象规则
测试规则
```

如果 `backend/AGENTS.md` 与 SSOT 冲突：

```text
产品语义以 SSOT S1 为准。
实现契约以 SSOT S2 为准。
backend/AGENTS.md 需要修正。
```

---

## 4. 实现边界

本仓库可以实现：

```text
后端 API
业务逻辑
数据库 migration
权限校验
错误码返回
事件生产与消费
任务调度
模型调用
存储访问
provider / runtime 适配
```

不得私自新增：

```text
SSOT 未定义的 API
SSOT 未定义的请求字段
SSOT 未定义的响应字段
SSOT 未定义的数据库表
SSOT 未定义的数据库字段
SSOT 未定义的错误码
SSOT 未定义的权限码
SSOT 未定义的事件类型
SSOT 未定义的核心业务状态
```

发现 SSOT 缺失时，必须先修改并 release `omnimam-ssot`，再更新 submodule。

---

## 5. OpenAPI 规则

后端 API 必须匹配：

```text
ssot/01_contracts/domains/<domain_id>/openapi.yaml
```

禁止：

```text
实现 OpenAPI 中不存在的接口
返回 OpenAPI 中不存在的字段
修改请求结构但不更新 SSOT
修改响应结构但不更新 SSOT
忽略 OpenAPI 定义的错误响应
```

---

## 6. Migration 规则

SSOT schema 是设计态目标结构：

```text
ssot/01_contracts/domains/<domain_id>/schema.sql
```

本仓库 migration 是运行态演进过程。

migration 必须能追溯到 SSOT schema。

禁止：

```text
私自新增表
私自新增字段
私自修改字段类型
私自新增枚举值
私自新增与 SSOT 无关的约束或索引
```

如 schema 不足，先更新 SSOT，再写 migration。

---

## 7. 错误码、权限码、事件规则

错误码只能来自：

```text
ssot/01_contracts/domains/<domain_id>/errors.yaml
```

权限码只能来自：

```text
ssot/01_contracts/domains/<domain_id>/permissions.yaml
```

事件只能来自：

```text
ssot/01_contracts/domains/<domain_id>/events.yaml
```

禁止在代码中临时发明错误码、权限码或事件类型。

---

## 8. 模块边界规则

模块边界必须参考：

```text
ssot/01_contracts/domains/<domain_id>/module-contract.md
ssot/02_architecture/domains/<domain_id>.md
backend/AGENTS.md
```

禁止为了快速实现而跨模块穿透。

如果模块契约不足，先补充 SSOT 的 `module-contract.md`。

---

## 9. Provider / Runtime 抽象规则

涉及以下场景时，应按 `backend/AGENTS.md` 使用接口抽象：

```text
模型 provider
对象存储 provider
向量数据库 provider
TTS / image / video 任务执行器
OAuth / API key / 本地授权模式
Workflow runtime
Agent runtime
GPU worker 调度器
外部 HTTP/RPC client
```

禁止把具体 provider 实现直接写死在核心业务逻辑中。

---

## 10. 冲突处理

发现冲突时按以下规则处理：

```text
实现与 SSOT S2 冲突：
  实现错则修改实现。
  S2 漏则修改 SSOT，release 后更新 submodule。

S2 变化影响产品语义：
  必须同步修改 S1。

backend/AGENTS.md 与 SSOT 冲突：
  以 SSOT 为准，修正 backend/AGENTS.md。

ssot/ 中内容需要修改：
  不得在本仓库直接改。
  必须回到 omnimam-ssot 仓库修改并 release。
```

---

## 11. 提交前检查

提交前必须确认：

```text
ssot submodule 指向已 release commit 或 tag
SSOT_VERSION.commit 与 submodule commit 一致
实现已读取对应 S1/S2
API 与 openapi.yaml 一致
migration 可追溯到 schema.sql
错误码来自 errors.yaml
权限码来自 permissions.yaml
事件来自 events.yaml
模块边界符合 module-contract.md
实现规则符合 backend/AGENTS.md
```
