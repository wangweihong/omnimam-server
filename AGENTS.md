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
