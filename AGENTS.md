# OmniMAM Server Agents 规则

本仓库是 OmniMAM 后端实现仓库。

所有后端实现必须遵循两个上游约束：

```text
1. ssot/ 中已 release 的 omnimam-ssot S1/S2
2. backend/AGENTS.md 中定义的后端实现规则
```

---

## 1. SSOT Submodule 约束

本仓库必须通过 submodule 引用 `omnimam-ssot`。

推荐路径：

```text
ssot/
```

禁止自动跟随 `omnimam-ssot/main`。
必须 pin 到明确 commit 或 tag。

本仓库不得直接修改 `ssot/` 目录内文件并作为 server 仓库提交。

如需修改 S1/S2，必须回到 `omnimam-ssot` 仓库修改、release 后，再更新本仓库 submodule commit。

---

## 2. SSOT_VERSION 约束

仓库根目录必须包含：

```text
SSOT_VERSION
```

示例：

```text
repo: github.com/<org>/omnimam-ssot
commit: abc123456789
contract_version: ssot-v0.1.0
updated_at: YYYY-MM-DD
```

要求：

```text
SSOT_VERSION.commit 必须与 ssot submodule 当前 commit 一致。
正式实现、合并、验收和发布只能基于已 release 的 SSOT commit 或 tag。
未 release 的 SSOT 只能用于讨论、探索和评估。
```

---

## 3. 后端实现路由

当任务涉及后端实现、API、数据库、migration、错误码、权限、事件、任务调度、provider/runtime 适配时，必须读取：

```text
skills/omnimam-server-backend/SKILL.md
```

并同时遵循：

```text
backend/AGENTS.md
```

---

## 4. 冲突优先级

```text
产品语义以 SSOT S1 为准。
实现契约以 SSOT S2 为准。
后端实现规则以 backend/AGENTS.md 为准。
若 backend/AGENTS.md 与 SSOT 冲突，以 SSOT 为准，并修正 backend/AGENTS.md。
```

---

## 5. 禁止事项

禁止：

```text
直接修改 ssot/ 后作为本仓库提交
自动跟随 ssot/main
基于未 release SSOT 做正式合并或验收
私自新增 SSOT 未定义的 API
私自新增 SSOT 未定义的数据库表或字段
私自新增 SSOT 未定义的错误码
私自新增 SSOT 未定义的权限码
私自新增 SSOT 未定义的事件类型
绕过 backend/AGENTS.md 实现后端逻辑
```
# Completion & Handoff Rules

These rules are mandatory for every task.

## 1. Handoff Is a Live Checkpoint

`docs/HANDOFF.md` is not only a final summary. It must be updated throughout the task so work can resume after context compression, interruption, or a new session.

Update it:

* At the start of every non-trivial task.
* After each meaningful implementation milestone.
* After important design, API, schema, configuration, or file changes.
* When a blocker, failed approach, risk, or technical debt is discovered.
* Before large or high-risk changes.
* Whenever the context is becoming large or may be compressed.
* Before declaring the task complete.

Do not allow significant completed work to remain undocumented.

## 2. Required Content

Keep `docs/HANDOFF.md` concise and reflect the current project state.

It must contain:

* Current goal and status.
* Work completed in this session.
* Current in-progress work.
* Files added, modified, renamed, or removed.
* Key architectural or design decisions.
* API, schema, dependency, or configuration changes.
* Verification performed and remaining checks.
* Outstanding tasks.
* Known issues and risks.
* Exact recommended next step.

For unfinished work, record the last successful action and the exact next file, command, or implementation step.

Never describe unverified or partial work as completed.

## 3. Task Completion

Before declaring a task complete:

1. Finish the implementation.
2. Run relevant tests, builds, linting, or manual verification.
3. Update affected project documentation.
4. Refresh `docs/HANDOFF.md`.
5. Confirm the handoff matches the actual repository state.

## 4. Handoff Maintenance

Keep the handoff actionable and free of obsolete history:

* Remove outdated information.
* Move finished items out of the in-progress section.
* Preserve unresolved blockers and risks.
* Use exact file paths, function names, commands, and API names.
* Do not claim changes or verification that were not actually performed.

Always end `docs/HANDOFF.md` with:

```text
Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
```


## 引入新二进制限制
未经用户许可，禁止自行在backend/cmd/ 目录下引入新的二进制文件。

# 任务范围
【任务范围】
不再递归扫描整个 ssot-spec 或整个仓库。
只允许读取：当前任务指定的 SSOT 入口文件；
入口文件直接引用的规范；
当前目标模块的源码；
当前目标模块直接相关的测试。

不读取历史版本、归档目录、生成产物和无关模块。
 
【读取约束】
大文件禁止整文件读取。
先使用 rg 定位符号或章节，再用 sed 读取必要区间。
命令输出最多保留相关部分，不输出完整日志。
同一信息已经读取后，不重复读取。
【实现约束】
只实现 SSOT 明确要求的最小变更。
优先复用当前代码，不新增无必要的抽象和基础设施。
不修改与本任务无关的公共接口。
发现规范冲突时停止，不自行选择或扩大设计。
【验证约束】
只运行目标 package 或目标模块测试。
不运行全仓库测试。
同一失败测试最多修复并重试 2 次。
禁止为了消除无关测试失败而修改其他模块。