# Project Handoff

## Current goal and status

- Goal: 排查用户注册后登出，再用相同密码登录失败的问题；重点核对 Identity 模块 OPAQUE registration record 的生成/保存，以及登录 challenge 使用的 server setup、user identifier 和前端默认配置是否一致。
- Status: 排查中，仅分析，不修改业务代码。

## Work completed in this session

- 已读取 `skills/omnimam-server-backend/SKILL.md` 与 `backend/AGENTS.md`。
- 已沿用既有复现结论：用户为 `ACTIVE`、registration record 存在，`login/start` 返回有效 `ke2`，前端 `finishLogin` 返回空值，因此未调用 `login/finish`。
- 当前最高可疑点是 Go 后端使用自定义 OPAQUE Context `omnimam/identity/opaque/v1`，而前端 `@serenity-kit/opaque@1.1.0` 调用未显式传递该 Context；仍需读取库默认值和双方 wire/config 以确认。

## Current in-progress work

- 对比 Go OPAQUE configuration、前端 OPAQUE 封装及依赖库默认 suite/context/record 编码。

## Files added, modified, renamed, or removed

- Modified: `docs/HANDOFF.md`（仅更新本次排查状态）。

## Key architectural or design decisions

- 本轮只做只读排查与可复现实验，不清理数据库、不旋转 OPAQUE setup、不修改 API 或业务代码。
- 只读取 Identity 目标模块、其直接相关测试和前端 OPAQUE 封装；不递归扫描整个 SSOT 或无关模块。

## API, schema, dependency, or configuration changes

- None.

## Verification performed and remaining checks

- 已复现 `login/start` 成功但浏览器端 `finishLogin` 返回 `null`。
- Remaining: 确认 Go 与 JS 的 OPAQUE context、suite、identifier 字节编码和 registration record 编解码是否完全一致；必要时用相同 record 做最小跨语言验证。

## Outstanding tasks

- 完成配置差异定位并给出证据链与不修改前提下的修复建议。

## Known issues and risks

- 当前不能仅凭 `login/start` 的 HTTP 200 判断密码校验通过；真正失败发生在浏览器端 OPAQUE client 处理 `ke2` 阶段。

## Exact recommended next step

读取 `backend/internal/apiserver/service/v1/identity/opaque.go`、`opaque_service.go` 及前端 `shared/auth/opaque`，再检查 `@serenity-kit/opaque` 1.1.0 的默认配置和测试向量。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
