---
name: omnimam-server-backend
description: Guide OmniMAM Server backend implementation against the pinned, released SSOT with minimal layered context loading. Use for API, business logic, database migration, errors, permissions, events, scheduling, provider or runtime adapters, backend refactors, and backend tests in omnimam-server.
---

# OmniMAM Server Backend Skill

使用本 Skill 指导 `omnimam-server` 仓库中的后端任务，并以最小必要上下文消费已 release 的 SSOT。

## 1. Skill 目标与事实源

按以下职责使用规则来源：

```text
S1
  决定产品语义、业务规则、状态流转和验收标准。

S2
  决定 API、Schema、错误码、权限码、事件和模块契约。

Context
  负责领域定位和正式文件导航，不是正式实现依据。

backend/AGENTS.md
  决定不与 SSOT 冲突的后端实现方式。

当前实现
  用于确认已有结构、兼容要求和变更范围。
```

不得只读取 Context、OpenAPI、架构文档或现有代码后直接实现正式业务逻辑。

## 2. SSOT Submodule 与 Release 规则

必须通过 `ssot/` submodule 消费 `omnimam-ssot`，不得复制一套 SSOT 文档在本仓库长期维护。

禁止自动追踪 SSOT 主分支。必须 pin 到明确 commit 或 tag，且正式实现、合并、验收和发布只能依赖已 release 的 S1/S2。未 release 内容只能用于讨论、评估和隔离原型，不得作为正式实现依据。

不得直接修改 `ssot/` 内文件并作为 server 仓库变更提交。发现 SSOT 缺失或冲突时，必须回到 SSOT 仓库修正并 release，再更新本仓库 submodule。

执行正式任务前先运行或等价检查：

```text
git submodule status ssot
SSOT_VERSION
```

验证：

```text
SSOT_VERSION.commit == ssot submodule commit
当前 submodule commit 已 release
SSOT_VERSION 中的 release 信息与当前 commit 一致
```

如果版本不一致或 release 状态无法确认：

- 停止 API、业务逻辑、Schema、Migration、错误码、权限、事件、调度和 provider/runtime 等正式实现。
- 不生成或修改正式契约代码。
- 只允许继续执行经检查不改变产品语义、契约或运行行为的文档、测试、格式整理或内部重构。
- 明确记录实际 commit、声明 commit 和阻断原因。

## 3. SSOT Context Loading

对新的产品功能、业务流程或 Domain 任务按以下顺序读取：

```text
1. ssot/GLOBAL_CONTEXT.md
2. ssot/CONTEXT_MAP.md
3. CONTEXT_MAP 指向的主 Domain context.md
4. 任务所需的已 release S1/S2 章节或片段
5. backend/AGENTS.md 中相关章节
6. 当前功能已有代码和测试
```

### 3.1 Global Context

先读取 `ssot/GLOBAL_CONTEXT.md`，用于理解项目阶段、全局目标、领域边界和核心事实归属。不得在后端重新定义已有业务对象。

同一连续上下文已经读取且 submodule commit 未变化时，不要重复读取。

### 3.2 Context Map

读取 `ssot/CONTEXT_MAP.md`，根据“谁拥有将被修改的事实”确定主 Domain，并定位 Domain Context 和最小正式文件集合。

不得根据相似文件名猜测 Domain，不得在未查看 Context Map 时预防性加载多个 Domain。

### 3.3 Domain Context

读取 `CONTEXT_MAP.md` 给出的准确 `context.md` 路径。不得在本 Skill 中维护会随 SSOT 演进的 Domain 目录清单。

使用 Domain Context 确认领域职责、核心对象、规则、边界、上下游关系、正式事实源及非本领域内容。只有存在明确跨域读写、事件协作或事实归属依赖时，才读取第二个 Domain Context。

### 3.4 正式 S1/S2

Domain Context 只是摘要和导航。定位完成后，必须从当前 pin 且已 release 的正式文件中读取任务所需章节或片段。

如果工具无法按章节或结构片段读取，可以读取对应完整文件，但不得因此扩大到其他无关文件或 Domain。

### 3.5 Context 缺失门禁

如果以下任一条件成立，不得静默回退到按文件名扫描并开始正式实现：

```text
GLOBAL_CONTEXT.md 缺失
CONTEXT_MAP.md 缺失
Context Map 无法定位任务的主 Domain
目标 Domain context.md 缺失
Domain Context 引用的正式事实源缺失
```

此时停止依赖产品语义或契约的正式后端工作，记录缺失路径，并等待 pin 到包含 Context 层的已 release SSOT。只允许继续执行已经证明不改变语义、契约、持久化结构、模块边界或运行行为的文档、测试、格式整理和内部重构。

## 4. Minimum Context Set

普通单 Domain 后端任务默认控制在：

```text
ssot/GLOBAL_CONTEXT.md
ssot/CONTEXT_MAP.md
1 个主 Domain context.md
1 个 S1 相关章节
1～3 个必要 S2 片段
backend/AGENTS.md 的相关章节
当前实现和相关测试
```

禁止默认读取：

```text
整个 ssot/
全部 Domain Context
整个 product-spec.md
整个 openapi.yaml
整个 schema.sql
整个 errors.yaml、permissions.yaml 或 events.yaml
全部 module-contract 和 Architecture
完整 backend/ 目录
```

生成整个 Domain 契约代码、执行完整 Domain 契约审计或处理有大量共享定义的大规模迁移时，可以扩大到所需完整文件。必须说明扩大范围的原因，仍不得加载无关 Domain。

## 5. Task Classification

开始任务时先分类，再选择最小正式文件：

```text
产品与业务逻辑
API、Controller 或 DTO
数据库、Store 或 Migration
错误处理
权限与认证
事件、Outbox 或 SSE
任务调度、Provider 或 Runtime
跨模块或跨 Domain 功能
纯内部重构、测试、文档或格式整理
契约生成、全域审计或大规模迁移
```

执行流程：

```text
1. 检查 submodule commit、release 和 SSOT_VERSION。
2. 判断任务类型及是否影响正式语义或契约。
3. 读取或复用 Global Context 和 Context Map。
4. 确定事实拥有者和主 Domain。
5. 读取主 Domain Context。
6. 按任务类型选择最少必要的 S1/S2。
7. 读取 backend/AGENTS.md 相关章节。
8. 检查当前实现和测试。
9. 实现并验证修改。
10. 检查是否私自引入契约或跨越模块边界。
11. 更新相关文档和 docs/HANDOFF.md。
```

## 6. 按任务读取 S1/S2

### 6.1 产品与业务逻辑

必须读取主 Domain Context，以及 S1 中对应业务规则、状态流转、异常场景和验收标准。只在实际影响 API、数据、错误、权限、事件或模块协作时读取相应 S2 片段。

不得用现有实现或 S2 字段反推并替代 S1 产品语义。

### 6.2 API、Controller 或 DTO

必须读取：

```text
主 Domain Context
S1 中对应用户流程、业务规则和验收标准
openapi.yaml 中对应 operation
operation 直接使用的 request/response schema 和错误响应
```

优先按 `operationId`、path 或 schema 名定位。只有生成整个 Domain API、检查完整契约一致性或执行大规模 API 迁移时才读取完整 OpenAPI。

按需读取相关错误码、权限码和模块契约。禁止实现不存在的路径、字段、响应或错误分支。

### 6.3 数据库、Store 或 Migration

必须读取：

```text
主 Domain Context
S1 中对应生命周期、不变量和业务规则
schema.sql 中相关表、字段、约束、索引和枚举片段
现有 migration 链和 store 实现
```

跨模块访问或事务协作时再读取相关 `module-contract.md`、事件或架构片段。只有 API 同时变化时才读取对应 OpenAPI operation。

S2 schema 是设计态目标结构，本仓库 migration 是运行态演进过程。Migration 必须可追溯到已 release schema；不得私自新增表、字段、类型、枚举值、约束或索引。

### 6.4 错误处理

必须读取主 Domain Context、S1 中对应异常场景和 `errors.yaml` 中相关错误码。只有需要确认 HTTP 响应结构或 operation 错误响应时才读取对应 OpenAPI 片段。

不得因为处理一个错误而默认读取完整错误码文件，不得临时发明或复用语义不匹配的错误码。

### 6.5 权限与认证

必须读取主 Domain Context、S1 中对应角色与授权行为，以及 `permissions.yaml` 中相关权限码。按需读取 identity Domain Context、对应认证接口或模块契约。

不得自行新增权限码，也不得因认证相关对象出现而加载全部 identity 契约。

### 6.6 事件、Outbox 或 SSE

必须读取主 Domain Context、S1 中对应状态变化和业务语义，以及 `events.yaml` 中相关事件定义。生产者与消费者跨 Domain 时，读取直接相关的第二个 Domain Context、双方 module contract 和必要 schema。

按需读取持久化、重放、幂等或投影涉及的 schema 和架构片段。不得私自新增事件类型、payload 字段或事实归属。

### 6.7 任务调度、Provider 或 Runtime

必须读取主 Domain Context、S1 中对应执行生命周期和失败语义，以及 Domain Context 指向的相关 module contract、事件、schema 或 runtime 契约。只有涉及全局依赖或跨模块运行链路时才读取相关 Architecture 片段。

同时读取 `backend/AGENTS.md` 中 Task Center、WorkflowRuntime、可替换边界、依赖注入、恢复和测试相关章节。业务层必须依赖消费方接口，不得把具体 provider/runtime 写死在核心逻辑中。

### 6.8 纯内部重构、测试、文档或格式整理

先读取当前实现、相关测试和 `backend/AGENTS.md` 对应章节。如果能够证明不改变产品行为、API、持久化、错误、权限、事件、调度或模块边界，可以不重新读取完整 S1/S2。

必须保持外部行为和契约不变。存在语义不确定性或发现当前实现可能与 SSOT 冲突时，立即回到完整 Context 定位流程；Context 层缺失时停止该部分工作。

### 6.9 契约生成、全域审计或大规模迁移

可以读取任务覆盖范围内的完整 S1/S2 文件，例如完整 OpenAPI、schema、错误码、权限码或事件文件。先明确目标 Domain、生成物、共享依赖和审计边界，不得把“全域”扩展为整个 SSOT。

## 7. S1/S2 与契约边界

不得私自新增：

```text
API 路径、请求字段或响应字段
数据库表、字段、类型、枚举值、约束或索引
错误码或权限码
事件类型或事件字段
核心业务状态、对象关系或流程
跨模块访问路径
```

使用以下正式来源：

```text
产品语义和验收：product-spec.md
HTTP 接口：openapi.yaml
设计态数据结构：schema.sql
错误码：errors.yaml
权限码：permissions.yaml
事件：events.yaml
模块边界：module-contract.md
架构关系：对应 Architecture 片段
```

不得把 Architecture 当作覆盖 S1/S2 的合同。不得因为 Context 摘要已经描述某项规则而跳过正式文件。

## 8. Context 缓存规则

同一连续任务中，如果 submodule commit 未变化：

- 复用已经读取的 `GLOBAL_CONTEXT.md` 和 `CONTEXT_MAP.md`。
- 同一 Domain 不重复读取未变化的 `context.md`。
- 复用已读取且仍覆盖当前任务的 S1/S2 章节。
- 切换 Domain 时只新增目标 Domain Context 和直接需要的正式片段。

如果 submodule commit 变化：

- 重新检查 release 和 `SSOT_VERSION`。
- 重新读取 Global Context、Context Map 和当前 Domain Context。
- 重新定位已使用的 S1/S2，并检查相关契约差异。
- 不得用旧缓存覆盖新的 SSOT release。

## 9. 跨 Domain 读取规则

只有任务明确改变跨域协作、读写另一事实源、生产或消费跨域事件，或依赖另一个领域的正式业务规则时，才读取多个 Domain Context。

以事实拥有者作为主 Domain。关联 ID、只读摘要、投影字段或理论上的上下游关系不自动触发加载关联 Domain 的全部文件。

每增加一个 Domain，必须能够指出：

```text
本次修改涉及的事实
该 Domain 对事实的所有权
必须读取的具体正式文件或片段
```

## 10. 与 backend/AGENTS.md 的关系

本 Skill 负责 SSOT、Context、release 和契约消费流程；`backend/AGENTS.md` 负责 Go 后端实现规则。按标题和关键词读取与当前任务相关的章节，不要为了普通单 Domain 任务默认扩展到无关实现规则。

如果两者或当前实现与 SSOT 冲突：

```text
产品语义以已 release S1 为准。
实现契约以已 release S2 为准。
Context 只负责定位。
backend/AGENTS.md 必须在不冲突的范围内执行。
```

未经用户许可，不得在 `backend/cmd/` 引入新二进制。

## 11. 冲突与 SSOT 更新流程

发现缺失或冲突时：

```text
1. 记录冲突、事实拥有者和受影响正式文件。
2. 停止依赖该不确定契约的正式实现。
3. 在 omnimam-ssot 仓库修正 S1/S2 和必要 Context。
4. 完成 release。
5. 更新本仓库 ssot submodule 到明确 commit/tag。
6. 更新 SSOT_VERSION。
7. 重新加载 Context 并核对受影响 S1/S2。
8. 再继续正式实现、生成、验收或发布。
```

如果 S2 变化影响产品语义，必须同步修正 S1。不得在本仓库直接修改 submodule 内容规避 release 流程。

## 12. Before Implementation

- [ ] 已确认 submodule commit、release 状态和 `SSOT_VERSION` 一致。
- [ ] 已确定任务类型及是否改变正式语义或契约。
- [ ] 对依赖正式语义或契约的任务，已读取或有效复用 `GLOBAL_CONTEXT.md`。
- [ ] 对依赖正式语义或契约的任务，已通过 `CONTEXT_MAP.md` 确定主 Domain。
- [ ] 对依赖正式语义或契约的任务，已读取 Context Map 指向的主 Domain Context。
- [ ] 对依赖正式语义或契约的任务，已读取必要的 S1 章节和 S2 片段。
- [ ] 对不读取 Context 的例外任务，已证明其不改变语义、契约、持久化、模块边界或运行行为。
- [ ] 已读取 `backend/AGENTS.md` 相关章节。
- [ ] 已检查当前实现、测试和兼容边界。
- [ ] 已说明任何跨 Domain 或扩大读取范围的必要性。
- [ ] 未加载明显无关的 Domain 或完整契约文件。

## 13. Before Completion

- [ ] 产品行为、状态和异常流程符合已 release S1。
- [ ] API 与相关 OpenAPI operation 和 schema 一致。
- [ ] Migration 可追溯到已 release `schema.sql`。
- [ ] 错误码、权限码和事件来自对应 S2 文件。
- [ ] 模块边界符合 `module-contract.md` 和相关架构约束。
- [ ] 未新增 SSOT 未定义的契约、业务状态或跨域关系。
- [ ] 相关格式化、生成、测试、构建或静态检查已通过，或已记录明确原因。
- [ ] 已记录使用的 SSOT release 和 commit。
- [ ] 未修改 `ssot/` 内部文件；必要的 gitlink 更新经过明确任务授权。
- [ ] 已更新相关文档和 `docs/HANDOFF.md`。

如果只修改文档或 Skill，不要求执行完整后端构建，但必须验证 Markdown、路径引用、Skill 结构、变更范围和 submodule 完整性。

## 14. 最终规则

```text
omnimam-server 只能消费已 release 的 omnimam-ssot，
不得替代或私自扩展 omnimam-ssot。

默认读取顺序：
GLOBAL_CONTEXT
-> CONTEXT_MAP
-> Domain Context
-> 必要的 S1/S2
-> backend/AGENTS.md 相关章节
-> 当前实现

Context 只负责定位，不是正式契约。
普通单 Domain 任务不得默认读取整个 SSOT。
S1 决定产品语义。
S2 决定实现契约。
backend/AGENTS.md 决定不与 SSOT 冲突的后端实现细节。
```
