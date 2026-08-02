# Project Handoff

## Current goal and status

提交当前工作区全部未提交变更。当前状态：已完成暂存、验证和统一提交；现有 Identity/Platform Management、workflow canvas、SSOT pin 及交接文档变更已纳入提交。

## Work completed in this session

- 已读取根目录、`backend/AGENTS.md` 和 `skills/omnimam-server-backend/SKILL.md`；确认本任务属于数据库运行数据清理，删除前必须先确认候选数据范围。
- 已确认 `SSOT_VERSION.commit` 与 `git -C ssot rev-parse HEAD` 均为 `926813505ce320f1450db2caf1e8f1ba3e258118`，对应 released `spec-v1.11.0`。
- 已确认工作区存在用户/前序任务未提交修改；本任务不回滚、不覆盖这些修改。
- 本次提交任务确认当前分支为 `feature/task-center`，工作区包含已修改、已重命名和未跟踪文件；`ssot` 工作树干净且固定在 released `spec-v1.11.0`。
- 已创建统一提交 `feat(identity): add v1.11 identity and platform APIs`，包含当前工作区全部 37 个文件变更；提交后工作区无未提交文件。

- 将 `ssot` pin 到 `926813505ce320f1450db2caf1e8f1ba3e258118`，对应已 release 的 `spec-v1.11.0`；`SSOT_VERSION` 已保持一致。
- 将旧 platform service、thumbnail executor 及测试移动到 `backend/internal/apiserver/service/v1/applicationplatform/`，旧构造函数改为 `NewLegacyService`；仓库中已无 `service/v1/platform` import。
- 新增 v1.11 Identity/Platform DTO、持久化模型、store capability、service、controller 和路由，使用独立 `identity_*`、`platform_*` 表模型。
- Identity 已接入注册、登录、Refresh Token、当前用户、会话、登出、改密、用户/RBAC/组/资源授权及 ServiceAccount 管理接口。
- Platform Management 已接入 overview、SystemAuthConfig、AuditLog 查询和内部审计写入接口。
- `IdentityAuthentication` 每请求校验 HS256 JWT、JTI credential、session、用户状态/security version，并动态读取权限；`RequireIdentityPermission` 和 `RequireServicePrincipal` 在路由 middleware 中判权。
- `Audit` middleware 对敏感请求执行审计 preflight 和完成记录；Identity/Platform 路由已调整为先进入审计、再执行 Token 验证，使无效 Token 的敏感请求也能进入审计边界。敏感审计边界不可用时 fail-closed。
- 密码使用 Argon2id PHC；Access Token 仅持久化 JTI；Refresh Token 仅持久化 SHA-256 hash；审计 detail 不接受敏感凭据字段。
- 修复 ServiceAccount 更新构造：`Description` 正确写入嵌入的 `ObjectMeta`，不再引用 middleware 私有 helper。
- 未新增 `backend/cmd/` 二进制；已保留用户原有 workflowcanvas/task-center 相关未提交修改。
- 定位审计列表报错：`CountAndFindPage` 的查询来源是裸 `s.ds.db`，未设置 GORM Model，计数阶段触发 `Table not set`。
- `ListAuditLogs` 显式使用 `Model(&iapiserver.PlatformAuditLog{})`；同批 Identity 列表显式使用各自的 User、PermissionDefinition、AuthSession、Role、Group、ResourceAccessGrant 和 ServiceAccount 模型。

## Current in-progress work

本次提交相关工作已完成；后续仅保留 Identity ServiceAccount 权限契约缺口和既有 task-name 本地化测试差异等风险。

## Files added, modified, renamed, or removed

- 新增 API/model：
  - `backend/apis/iapiserver/meta_identity_v11.go`
  - `backend/apis/iapiserver/meta_platform_management.go`
  - `backend/apis/iapiserver/request_identity_v11.go`
  - `backend/apis/iapiserver/request_platform_management.go`
- 新增 middleware/service/controller/store：
  - `backend/internal/apiserver/middleware/identity_v11.go`
  - `backend/internal/apiserver/service/v1/identity/v11_service.go`
  - `backend/internal/apiserver/service/v1/platformmanagement/service.go`
  - `backend/internal/apiserver/controller/v1/identity/v11.go`
  - `backend/internal/apiserver/controller/v1/platformmanagement/platform.go`
  - `backend/internal/apiserver/store/postgresql/identity_v11.go`
  - `backend/internal/apiserver/store/postgresql/identity_admin.go`
  - `backend/internal/apiserver/store/postgresql/platform_management.go`
- 重命名旧实现：
  - `backend/internal/apiserver/service/v1/platform/service.go` -> `backend/internal/apiserver/service/v1/applicationplatform/legacy_platform.go`
  - `backend/internal/apiserver/service/v1/platform/thumbnail_executor.go` -> `backend/internal/apiserver/service/v1/applicationplatform/legacy_thumbnail_executor.go`
  - 对应测试文件一并移动并更新 package/import。
- 修改路由、server AutoMigrate/default permission 初始化、store interfaces、旧认证旁路、错误码生成物和 API 错误码文档。
- 修改：`backend/internal/apiserver/store/postgresql/platform_management.go`、`identity_v11.go`、`identity_admin.go` 的列表查询模型声明。
- 本次提交任务不新增业务实现；`docs/HANDOFF.md` 已更新为提交前状态记录。
- `git add -A` 已完成；暂存区包含当前工作区全部 37 个文件变更，包括新增、修改、重命名、SSOT gitlink 和本交接文件。
- 用户已有且未由本任务回滚的修改：`backend/apis/iapiserver/meta_workflow_canvas.go`、`meta_workflow_canvas_v17.go`、`request_workflow_canvas.go`、`backend/internal/apiserver/server.go`、`backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas.go`、`backend/internal/apiserver/store/postgresql/workflow_canvas.go`、`backend/internal/apiserver/service/v1/workflowcanvas/builtin_catalog.go`。

## Key architectural or design decisions

- 正式实现只基于 pinned `spec-v1.11.0`；Identity 拥有用户、会话、Token、RBAC、资源授权和 ServiceAccount，Platform Management 拥有 SystemAuthConfig 与 AuditLog。
- 新 v1.11 链路不继承旧 debug 匿名管理员旁路；旧 `/api/v1` 业务路由保持原兼容链路。
- `/api/v1/iam/*` 与 `/api/v1/platform/*` 使用独立 IdentityAuthentication、permission middleware 和 Audit middleware；旧 `installPlatformApis` 仍注册 `/api/v1/me`、`/api/v1/model-providers`、`/api/v1/assets` 等顶层旧入口，两者没有同一路径冲突。
- 不在本仓库修改 `ssot/` 内容，也未新增 SSOT 未定义的表、错误码、权限码或事件类型。

## API, schema, dependency, or configuration changes

- 新增 v1.11 路由：`/api/v1/iam/*`、`/api/v1/platform/*`，普通业务成功/失败继续使用 HTTP 200 + business code 约定。
- 新增 GORM AutoMigrate 模型：Identity 用户/RBAC/组/资源授权/会话/Token/Refresh/ServiceAccount/Outbox，以及 Platform SystemAuthConfig/AuditLog/Outbox。
- 使用仓库已有 `golang-jwt/jwt/v4`、`x/crypto/argon2` 和 `google/uuid`，未引入新外部依赖。
- `SSOT_VERSION.commit` 与 `git -C ssot rev-parse HEAD` 均为 `926813505ce320f1450db2caf1e8f1ba3e258118`。

## Verification performed and remaining checks

- 已通过：工作区状态、最近提交、SSOT 子模块状态和 `SSOT_VERSION` 一致性核对；未发现 `ssot/` 工作树内文件修改。
- 已通过：`git diff --cached --check`。
- 已通过：涉及 API、Identity、Platform Management、旧 application platform、workflow canvas、PostgreSQL store、middleware 和 taskworker 的定向 `go test`。
- 已通过：所有已暂存 Go 文件的 `gofmt -l` 检查；`service/v1/service.go` 和 `taskworker/taskworker.go` 已按标准格式化并重新暂存。
- 已通过：统一提交后 `git status --short --branch` 确认工作区干净。

- 已通过：
  - `go test ./backend/internal/apiserver/service/v1/identity ./backend/internal/apiserver/controller/v1/identity ./backend/internal/apiserver/store/postgresql ./backend/internal/apiserver/middleware ./backend/internal/apiserver`
  - middleware 顺序调整后的 `go test ./backend/internal/apiserver ./backend/internal/apiserver/middleware ./backend/internal/apiserver/service/v1/identity ./backend/internal/apiserver/service/v1/platformmanagement ./backend/internal/apiserver/service/v1/applicationplatform ./backend/internal/apiserver/store/postgresql ./backend/apis/iapiserver`
  - 之前已通过 applicationplatform、platformmanagement、iapiserver、taskworker 等相关定向测试。
  - `git diff --check`
  - `rg 'service/v1/platform["/]' backend` 无结果。
  - 路由检查确认旧顶层 platform 入口与新 `/api/v1/platform/*` 无重复路径。
- 本次问题的静态根因检查已确认 `ListAuditLogs` 使用裸 `s.ds.db`；修复后已确认对应查询和同批 Identity 列表均设置 Model。
- 本次已通过：`gofmt -w internal/apiserver/store/postgresql/platform_management.go internal/apiserver/store/postgresql/identity_v11.go internal/apiserver/store/postgresql/identity_admin.go`。
- 本次已通过：`go test ./internal/apiserver/store/postgresql`。
- 本次已通过：`go test ./internal/apiserver/service/v1/platformmanagement ./internal/apiserver/controller/v1/platformmanagement ./internal/apiserver/middleware ./apis/iapiserver`。
- 本次已通过：`git diff --check`，以及 `SSOT_VERSION.commit == git -C ssot rev-parse HEAD` 检查。
- 本次运行 `go test ./internal/apiserver/...` 时，除已有 task-name 本地化测试外其余相关包均编译/测试通过；失败仍为 `service/v1/taskcenter/TestAssignSystemName` 与 `taskname/TestResolve/parameterized_system_name` 的既有期望差异，未涉及本次改动文件。
- `go test ./backend/...` 编译通过本任务新增包，但失败于既有 task-name 本地化断言：
  - `backend/internal/apiserver/service/v1/taskcenter/TestAssignSystemName`
  - `backend/internal/apiserver/taskname/TestResolve/parameterized_system_name`
  - 实际值为 `生成 thumbnail视图`，测试期望为 `生成 thumbnail 表现形式`；未将该无关行为混入本任务。

## Outstanding tasks

- 若要宣称 v1.11 全域验收，需要先由 SSOT/产品侧明确 ServiceAccount 请求中的 `permission_codes` 如何持久化：当前 v1.11 `schema.sql` 没有 ServiceAccount 权限关联表，现有 PostgreSQL store 不持久化该输入，以避免私自新增 SSOT 未定义表或字段。
- 需要产品或 SSOT 维护者决定是否修正上述契约缺口后，再补 ServiceAccount 权限读写与 SERVICE_ACCOUNT 权限判定集成。
- 与本任务无关的 task-name 本地化测试失败仍需由对应维护者处理。

## Known issues and risks

- 当前 ServiceAccount 管理接口可以创建、更新、禁用和轮换凭据，但其 `permission_codes` 在 released schema 中没有事实存储位置；在契约澄清前不能把该字段描述为已持久化。
- middleware 完成审计写入失败时无法回滚已经执行的业务事务；敏感操作通过执行前审计 preflight fail-closed，若需要审计与业务事实原子提交，应由具体业务事务接入 outbox/审计写入边界。
- 根工作区包含用户已有 workflowcanvas/task-center 修改，后续提交或回滚必须继续按文件区分。
- 本次用户明确要求提交全部未提交代码，因此将按工作区现状整体提交，不拆分或回滚其中的既有修改。

## Exact recommended next step

后续任务先读取本文件，针对 Outstanding tasks 中的契约缺口或既有测试差异继续处理；不要重复本次已完成的提交工作。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
