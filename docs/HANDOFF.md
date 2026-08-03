# Project Handoff

## Current goal and status

- Goal: 修复共享参数绑定错误被降级为 `code: 1 / Unknown Error Code` 的问题，使其返回既有 `ErrValidation=100004`。
- Status: 已完成；共享 binding error 现在保留既有 `ErrValidation=100004` 和 HTTP `400`，定向测试通过。
- Current SSOT: released tag `spec-v1.15.0`, gitlink `1445e30d800c7ae4598bed42811c787bf7d39fbd`；`SSOT_VERSION` 与 submodule 一致。

## Work completed in this session

- 已读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md`、Go 故障排查与错误处理规则。
- 已确认 `ssot` gitlink 与 `SSOT_VERSION.commit` 一致，且声明为 released `spec-v1.15.0`。
- 已从用户提供的响应确认两个独立症状：创建请求缺少三个 required 字段；该校验错误未映射到已知业务错误码而降级为 `code: 1`。
- 已发现工作区进入本任务前 `docs/HANDOFF.md` 存在上一任务修改；本次在其基础上刷新状态。
- 已确认 `AgentCreateRequest` 要求顶层 JSON 字段 `agent_profile_id`、`workspace_type`、`workspace_id`；platform 组合为 `agent.hermes/platform/agent`，coding 组合为 `agent.coding/coding/studio`。
- 已确认请求已通过 `agent.manage` 权限中间件，但在 `core.Run -> DecodeParameter -> ShouldBindJSON` 阶段失败，未调用 Agent service 或数据库。
- 已确认 `DecodeParameter` 对 JSON binding 错误调用 `errors.WrapCode(..., code.ErrValidation)`，而 `WriteResponse` 调用的 `errors.ToStatus` 只从 `*status` 复制 code，不从 `*withCode` 复制，导致 code 降级为 gotoolbox 保留值 `1`，HTTP status 降级为 `500`。
- 已确认预期已注册错误为 `ErrValidation=100004`、HTTP `400`、`Validation failed.`。
- 未找到 Agent controller/service 的直接创建测试；`backend/pkg/core/run_test.go` 只打印包装错误，不断言响应 code/status，因此未覆盖该回归。
- 用户已明确要求修复错误码问题；本次只复用既有 `ErrValidation`，不新增或修改 SSOT 错误码契约。
- 已将 `DecodeParameter` 的自定义 Decoder、JSON、multipart 和 query binding 错误从 `errors.WrapCode` 统一改为 `errors.WrapStatus`，使 `WriteResponse -> errors.ToStatus` 能保留业务错误码。
- 已将原先只打印错误的 `backend/pkg/core/run_test.go` 改为 HTTP 回归测试，断言 required JSON 校验失败返回 HTTP `400`、业务码 `100004`、英文消息 `Validation failed.`，且业务 action 不执行。

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Modified: `backend/pkg/core/request.go`、`backend/pkg/core/run_test.go`、`docs/HANDOFF.md`。

## Key architectural or design decisions

- 不修改 agent 创建接口、请求字段或错误码定义，只修复共享 binding error 的包装类型。
- 用户未指定 SSOT 入口文件，因此不自行扫描 `ssot/`；只读取当前 agent 创建模块及直接相关测试。
- 不修改 `ssot/`，不在 `backend/cmd/` 新增二进制。
- 在 `backend/pkg/core/request.go` 将 binding 分支统一为可被 `ToStatus` 识别的 status 包装，并在既有公共 `backend/pkg/core` 测试中覆盖 `100004/400`；不扩大修改 third-party errors 行为。
- 选择修改调用方包装类型而非 `third_party/gotoolbox/pkg/errors.ToStatus`，避免扩大所有 gotoolbox 消费方的行为变更范围。

## API, schema, dependency, or configuration changes

- None.

## Verification performed and remaining checks

- 已验证 `git submodule status ssot` 与 `SSOT_VERSION.commit` 一致且状态为 released。
- 已通过 `go test ./backend/pkg/core ./backend/internal/pkg/code`。
- 已通过 gotoolbox module 内 `go test ./pkg/errors -run 'TestWrapCode|TestErrorStack'`；现有测试不包含 `WrapCode -> ToStatus` code 保留断言。
- 已通过 `go test ./backend/pkg/core`，新增回归测试实际覆盖 Agent 创建共用的 JSON binding 响应路径。
- 已通过 `gofmt` 和 `git diff --check`。
- 未运行全仓库测试，符合目标 package 验证约束。
- Remaining: 获取调用方实际 POST body，确定三个字段是未发送、字段名不匹配还是被错误嵌套。

## Outstanding tasks

- 调用方补齐并按正确组合发送 `agent_profile_id`、`workspace_type`、`workspace_id`。
- 错误码修复无剩余实现任务。

## Known issues and risks

- 尚未取得实际 POST 请求体，当前只能确定服务端绑定后的三个字段为空，不能仅凭响应断言前端完全未发送字段，也可能存在字段命名或嵌套结构不匹配。
- 上述共享降级风险已在 `DecodeParameter` 内修复；其他绕过该入口并直接把 `WrapCode` 交给 `WriteResponse` 的调用点不在本任务扫描范围内。
- 上一任务的 SSE Bearer header 问题和 Identity OPAQUE `RoleGrants`/默认 `USER` 授权回归仍是独立遗留项，本次不扩大处理范围。

## Exact recommended next step

重新部署 API Server 后，用缺少 required 字段的 `POST /api/v1/agents` 请求确认运行实例返回 `code: 100004`；随后补齐 Agent 创建请求的三个必填字段。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
