# Project Handoff

## Current goal and status

基于现有 Identity 重构完成统一认证优化，并按用户要求移除旧认证、OTP、SAML/OAuth SSO 及其配置面。当前状态：实现、验证和提交完成；全量 backend 仅保留既有 task-name 本地化断言失败。

## Work completed in this session

- 确认 `SSOT_VERSION.commit` 与 `ssot` submodule 均为 released `spec-v1.11.0` commit `926813505ce320f1450db2caf1e8f1ba3e258118`，`ssot/` 工作树干净。
- 将 Identity service/store 从 `V11` 临时命名统一为 `Service`、`IdentityStore` 和 `Factory.Identities()`；controller、middleware、route、启动权限初始化已同步。
- 保留并完善用户自停用/自删除保护：自删除返回 SSOT 已定义的 `ERR_IDENTITY_SELF_DELETE_FORBIDDEN=220405`，自停用返回 `ERR_IDENTITY_USER_STATE_INVALID=220403`；停用/删除递增 `security_version` 并撤销会话。
- 删除旧 `/api/v1/auth/*`、旧 Token middleware/codec、匿名开发管理员绕过和旧 user/OTP Identity service。
- 将普通 `/api/v1` 业务接口切换到 Identity JWT、JTI credential、AuthSession、用户状态和动态权限投影校验；仍向未迁移业务 service 注入兼容用户摘要。
- MCP Streamable HTTP 已改为同一 Identity JWT/credential/session 校验，只接受 `USER` 主体，不再读取 legacy `users` Token。
- 删除 `/api/v1/setting/sso/*` 及旧 SAML/OAuth/OTP 的 API DTO、controller、service、store interface 和 PostgreSQL adapter；停止 AutoMigrate `settings`、identity/service provider、one-time token 和 OTP 旧表，但没有执行 DROP TABLE 或修改运行数据库。
- 删除旧认证专用依赖：`dgrijalva/jwt-go`、`crewjam/saml`、`pquerna/otp`、`go-qrcode`、XML signature/validator 等，并执行 `go mod tidy`。
- JWT secret 不再内置固定默认值；启动配置要求至少 32 字节，并通过 `APISERVER_AUTH_JWT_SECRET`/`OMNIMAM_AUTH_JWT_SECRET` 注入 API Server、taskworker 和 notificationworker；resolver、token issuer、MCP 和备用路由安装入口均在密钥不足时 fail closed。
- Argon2id 校验改用常量时间比较，并限制 PHC 中 memory/time/parallelism、salt 和 key 长度，避免异常持久化参数导致资源耗尽。
- 按仓库规则运行 `make gen.deepcopy`；未手写 deepcopy，未新增 `backend/cmd/` 二进制。
- 已提交为 `refactor(identity): remove legacy authentication`，提交后工作区已核对。

## Current in-progress work

- 无。当前仅等待运行环境提供 JWT secret 并执行端到端回归。

## Files added, modified, renamed, or removed

- 新增统一实现：`backend/internal/apiserver/service/v1/identity/identity.go`。
- 修改 Identity/controller/middleware/route/store：`backend/internal/apiserver/service/v1/identity/0_service.go`、`backend/internal/apiserver/controller/v1/identity/v11.go`、`backend/internal/apiserver/controller/v1/mcp/controller.go`、`backend/internal/apiserver/middleware/identity_v11.go`、`backend/internal/apiserver/route.go`、`backend/internal/apiserver/server.go`、`backend/internal/apiserver/store/{factory.go,store.go}`、`backend/internal/apiserver/store/postgresql/{0_pg.go,identity_v11.go,identity_admin.go}`。
- 删除旧认证 controller/service/middleware/codec，以及 `backend/internal/apiserver/controller/v1/setting/`、`backend/internal/apiserver/service/v1/setting/`、旧 SSO/OTP API 和 PostgreSQL adapter 文件。
- 修改配置与依赖：`backend/internal/apiserver/options/options.go`、`configs/apiserver.yaml`、`deployments/docker-compose.yaml`、`go.mod`、`go.sum`。
- 修改错误码源和生成文档：`backend/internal/pkg/code/identity_platform.go`、`backend/internal/pkg/code/code_generated.go`、`docs/guide/zh-CN/api/error_code_generated.md`。
- 工作区另有本轮开始前已存在的 `backend/internal/apiserver/service/v1/applicationplatform/legacy_platform_test.go` 删除；本轮未恢复或改写该用户变更。

## Key architectural or design decisions

- `spec-v1.11.0` Identity 是唯一认证事实源；普通业务接口通过兼容用户摘要逐步迁移，但不再接受旧 JWT 或匿名管理员。
- Identity/Platform Management 路径保留局部 `Audit -> IdentityAuthentication` 顺序，确保无效 Token 的敏感请求也进入审计边界；其他 `/api/v1` 路径由统一 Identity middleware 认证。
- SSO/OAuth/MFA 属于当前 released SSOT 明确不支持能力，因此移除旧入口和实现，不以隐藏路由或孤立配置保留。
- 不执行破坏性数据库清理；停止管理的旧表可在单独 migration 任务中确认数据和回滚方案后再删除。

## API, schema, dependency, or configuration changes

- 删除旧 `/api/v1/auth/otp/*`、`/api/v1/auth/sso/*` 和 `/api/v1/setting/sso/*` 路由；正式认证入口仅保留 `/api/v1/iam/*`。
- 非 IAM 业务 API 和 `/mcp` 现在要求新 Identity Access Token；旧 Token 不再兼容。
- 未新增数据库表、字段、错误码、权限码或事件；仅停止 AutoMigrate SSOT 不支持的旧认证表。
- 新增必填配置 `auth.jwt-secret`，要求至少 32 字节；compose 必须设置 `OMNIMAM_AUTH_JWT_SECRET`。

## Verification performed and remaining checks

- 通过 `make gen.deepcopy`。
- 通过 Identity、MCP、middleware、options、PostgreSQL store、API Server、taskworker、notificationworker 定向 `go test`。
- 通过 `go vet ./backend/internal/apiserver/... ./backend/internal/taskworker ./backend/internal/notificationworker`。
- 通过 `go mod tidy`、`go mod verify`、`git diff --check`。
- 通过 `OMNIMAM_AUTH_JWT_SECRET=0123456789abcdef0123456789abcdef docker compose -f deployments/docker-compose.yaml config --quiet`。
- `go test ./backend/...` 除以下既有失败外均通过：
  - `backend/internal/apiserver/service/v1/taskcenter/TestAssignSystemName`
  - `backend/internal/apiserver/taskname/TestResolve/parameterized_system_name`
  - 实际 `zh-CN` 为 `生成 thumbnail视图`，测试期望 `生成 thumbnail 表现形式`，与 Identity 变更无关。
- 未启动完整 compose 或执行真实注册/登录/MCP 手工回归；运行前必须提供稳定 `OMNIMAM_AUTH_JWT_SECRET`。

## Outstanding tasks

- 在可用 PostgreSQL 环境中执行注册、登录、Refresh、登出、普通业务 API 和 MCP 的端到端手工回归。
- 单独处理既有 task-name 本地化测试失败。
- 若要删除数据库内遗留 SSO/OTP 表，必须另行盘点数据、制定 migration 和回滚方案；本任务未执行。
- released S1 要求注册/管理员创建用户分配默认 `USER` 角色、显式初始化管理员和最后一个 SUPER_ADMIN 保护；现有实现仍未完整覆盖，应作为独立 Identity 合同补齐任务处理。
- ServiceAccount `permission_codes` 在 released schema 中仍无持久化事实位置；契约澄清前不得私自新增表或字段。

## Known issues and risks

- 旧 Token、旧 OTP/SSO 路由是明确 breaking change；客户端必须迁移到 `/api/v1/iam/*`。
- JWT secret 变更会使已有 Access Token 失效；多实例和 worker 必须使用相同、稳定且保密的值。
- 旧 SSO/OTP 表仍可能存在于部署数据库，当前代码不会访问或删除它们。
- middleware 完成审计失败仍无法回滚已提交业务事务；敏感请求只在执行前通过审计 preflight fail closed。

## Exact recommended next step

设置稳定的 `OMNIMAM_AUTH_JWT_SECRET`，启动 compose，依次验证注册、登录、权限投影、普通业务 API 和 MCP。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
