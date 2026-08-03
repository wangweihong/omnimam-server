# Project Handoff

## Current goal and status

- Goal: 将 `ssot` submodule 更新到已 release 的 `spec-v1.15.0` 并对齐后端；按用户最新要求移除 PostgreSQL 启动阶段的一次性旧 schema/旧数据迁移，改为直接在数据库容器中清理历史对象。
- Status: 已完成；v1.15.0 权限与实现已对齐，PostgreSQL 一次性历史迁移已从代码移除，容器内旧 RBAC 表及 Workflow Canvas 旧列已清理，限定验证通过。
- Current SSOT: tag `spec-v1.15.0`, gitlink `1445e30d800c7ae4598bed42811c787bf7d39fbd`; release 内容 commit `0d5954609215da7bce01fe82b351e7d57e8018da` 是该 tag 的直接父提交。

## Work completed in this session

- 已读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md` 和 Go 任务编排规则。
- 已核对当前 submodule、`SSOT_VERSION` 和工作区状态；发现已有未提交的 Identity 修复，不属于本次 SSOT 更新，必须保留。
- 已建立本次 v1.15.0 对齐任务的起始检查点。
- 已 fetch 并验证 annotated tag `spec-v1.15.0`，tag 指向正式 release commit `1445e30d...`，且包含 release 记录声明的内容 commit `0d595460...`。
- 已将 `ssot` gitlink pin 到 `1445e30d...`，并同步根目录 `SSOT_VERSION` 为 `spec-v1.15.0`。
- 已读取 `RELEASE.md` 的 v1.15.0 入口和其直接引用的权限/OpenAPI/S1 变更片段；未扫描无关 SSOT。
- 已结构化比对 v1.15.0 权限文件与 `identity.DefaultPermissions`：当前权限目录缺少 31 个已发布 code，且 `DefaultRolePermissions` 未向 `USER` 提供页面基础权限。
- 已更新 `backend/internal/apiserver/service/v1/identity/identity.go`：登记缺失的 31 个已发布权限码，补充 `USER` 默认页面权限，并让 `ADMIN`/`SUPER_ADMIN` 继承用户权限后叠加管理员权限；内部服务权限未授予用户角色。该阶段尚未格式化和测试。
- 已格式化 Identity 权限实现，并确认 v1.15.0 直接涉及的 12 个权限文件共定义 96 个权限码。
- 已按 OpenAPI operation 为 AI Chat、Model Management 和受条件权限影响的 Application Platform 路由补充 `RequireIdentityPermission`。
- 已补齐 Application Platform OpenAPI 中模板、应用、版本、运行表单和 Application Run 等全部既有 operation 的 `aiapp.application.read` / `manage` / `run` 路由权限；未扩大到该 OpenAPI 未定义的旧 engine/provider 路由。
- 已扩展共享 `modelgateway.Principal`，从 Identity 请求上下文复制已验证权限集合；Application Platform 创建/修改 global Application 显式校验 `aiapp.application.manage_global`，跨 owner ComfyUI 工作流及 test-run 显式校验 `aiapp.comfyui_workflow.manage_all` 并复用现有审计。
- 已更新现有 `comfyui_workflow_test.go`，覆盖管理员持有 `manage_all` 时的跨 owner 审计，以及仅有管理员角色但无权限码时拒绝访问。
- 已结构化确认 v1.15.0 直接涉及的 12 个权限文件共 96 个权限码，代码无缺失；`USER`、`ADMIN`、`SUPER_ADMIN` 的默认角色映射与 release 完全一致，代码额外保留 3 个 Identity 前端基础权限。
- 已发现 22 个既有权限目录项的 `resource/action` 仍为旧粗粒度值；当前 `EnsureDefaultPermissions` 只创建缺失项，不会刷新已有目录元数据，已有环境的前端权限目录无法完整获得 v1.15.0 元数据。
- 已将上述 22 个权限目录项的 `resource/action` 对齐 v1.15.0，并修改 `EnsureDefaultPermissions`，在已有权限记录的 `name/domain/resource/action/risk_level/status` 与发布目录不一致时执行幂等更新。
- Identity service 目录没有既有 `_test.go`；`backend/AGENTS.md` 禁止在非公共 `pkg/` 私自新增测试文件，因此默认权限与默认角色改用结构化脚本核对，不新增 Identity 测试文件。
- 已为 `IdentityStore` 增加 `UserHasAnyRole` 只读能力，覆盖用户直接角色与组角色；`StorePrincipalResolver` 现仅从 Identity RBAC 识别 `ADMIN`/`SUPER_ADMIN`，不再依赖旧 Platform `roles/user_roles`。
- 已在现有 `engine_system_binding_test.go` 中补充 resolver 测试，验证管理员角色查询与 middleware 权限投影传递。
- 已按用户“不兼容历史”要求将首账号 bootstrap、Model Gateway 和 Asset Library 管理员判断统一收紧为 uppercase `ADMIN`/`SUPER_ADMIN`。
- `EnsureDefaultPermissions` 现在删除 lowercase `super_admin` 的角色权限、用户角色、组角色和服务账号角色授权，再硬删除该旧角色；不迁移历史授权。
- 三个 uppercase 内置角色的 `name/builtin/status` 会幂等刷新，角色权限会按 `DefaultRolePermissions` 精确替换，既补齐前端页面权限，也移除历史多余授权。
- Asset Library 物理存储管理员鉴权已从旧 Platform `roles/user_roles` 迁移为 `IdentityStore.UserHasAnyRole`，同时保留现有 `system-admin` 内部主体路径。
- 已删除旧 `RoleStore`/`UserRoleStore` factory 接口、PostgreSQL 实现和 `Role`/`UserRole` 持久化模型。
- 已从 `0_pg.go` 移除启动时旧 RBAC 删表、旧 Application Platform schema reset、Identity OPAQUE 补列、Task Center attempt logs/DAG observability/schedule ownership 数据 backfill；DAG 当前索引和 CHECK 约束保留为 `taskCenterDAGObservabilityConstraintsSQL`。
- 已移除启动时旧索引/旧约束替换逻辑：Agent 旧 model-binding 索引、Platform audit idempotency 索引、Task Center owner-child 索引和 Application Platform 旧 source/binding 约束均不再执行 `DROP`；只幂等创建当前定义。
- 已将 `ensureWorkflowCanvasScheme` 收敛为当前 `idx_canvas_node_run_execution` 唯一索引创建，移除 v1.0 到 v1.7 的列/数据 backfill。
- 已删除只验证上述历史迁移的测试；现有业务行为、索引与约束测试保留，并改为验证干净 schema 上的幂等创建。
- 已只读确认容器 `omnimam-postgres` 的目标为数据库 `omnimam`、schema `public`、用户 `omnimam`；历史 backfill 待处理行均为 0，旧 Application Platform 标记列不存在，OPAQUE 当前列存在。
- 已在容器中事务执行 `DROP TABLE IF EXISTS public.user_roles; DROP TABLE IF EXISTS public.roles;`；复查两表均不存在，仍被 `/me` 使用的 `public.permissions` 保留。
- 已在容器中确认 Workflow Canvas 历史 backfill 待处理行均为 0、相关表无数据且旧列无数据库依赖；事务删除 `canvas_versions.compiled_definition_name` / `compiled_definition_version` 及 `canvas_node_runs.node_key` / `atomic_task_id` / `output_json` / `task_resource_version`，复查遗留列数为 0。

## Current in-progress work

- 当前无进行中实现。
- 已确认管理员 OPAQUE 创建用户不在 v1.15.0 release 入口的 Identity 契约范围内，本轮保持不变。

## Files added, modified, renamed, or removed

- 本次已修改：`ssot` gitlink、`SSOT_VERSION`、`docs/HANDOFF.md`、`backend/apis/iapiserver/meta_platform.go`、`backend/internal/apiserver/route.go`、`server.go`、`store/factory.go`、`store/store.go`、`store/postgresql/0_pg.go`、`store/postgresql/application_platform_test.go`、`task_center_schedule_test.go`、`task_center_test.go`、`identity_v11.go`、`platform.go`、`service/v1/identity/identity.go`、`service/v1/modelgateway/service.go`、`engine_system_binding_test.go`、`service/v1/applicationplatform/application_platform.go`、`comfyui_workflow.go`、`comfyui_workflow_test_run.go`、`comfyui_workflow_test.go`、`service/v1/assetlibrary/service.go`、`storage_inspection.go`、`storage_inspection_test.go`。
- 本次已修改的 PostgreSQL 直接相关测试还包括 `application_platform_system_binding_integration_test.go`；本次已删除：`backend/internal/apiserver/store/postgresql/application_platform_reset_integration_test.go`、`task_center_observability_migration_integration_test.go`、`workflow_canvas_migration_integration_test.go`。
- 任务开始前已有修改：`backend/internal/apiserver/service/v1/identity/opaque.go`、`backend/internal/apiserver/store/postgresql/0_pg.go`、`docs/HANDOFF.md`；不得回退或覆盖其既有内容。

## Key architectural or design decisions

- 只读取 v1.15.0 release 入口、入口直接引用的 S1/S2、目标模块源码及直接相关测试；不递归扫描整个 SSOT 或仓库。
- 正式实现仅以已 release 的 `spec-v1.15.0` commit 为依据，不自行扩展 API、Schema、错误码、权限码或事件。
- 不在 `backend/cmd/` 新增二进制。
- AI Chat 当前已有实现，因此只补 release 新增的 operation 权限中间件；不新增 API。
- Application Platform 保留 owner/visibility 校验，并在跨 owner/global 条件上叠加显式权限码；管理员角色不替代权限码。
- 路由层负责每个 OpenAPI operation 的基础权限，服务层只校验必须读取资源后才能判断的条件权限；Application Platform 复用认证中间件已经写入请求上下文的动态权限投影，不另建授权来源。
- Application Platform 的管理员身份必须仅来自 Identity 内置角色 `ADMIN`/`SUPER_ADMIN`；不保留历史 lowercase `super_admin` 兼容。旧 Platform `roles/user_roles` 不再作为该条件鉴权的事实源。
- 应用启动只建立当前模型、索引和约束，不再承担历史行 backfill 或旧表删除；环境升级所需的破坏性清理由运维在明确目标数据库中一次性执行。

## API, schema, dependency, or configuration changes

- SSOT pin 从 `spec-v1.14.2`/`102a447...` 更新为 `spec-v1.15.0`/`1445e30...`。
- 正式契约变化涉及权限目录、默认角色映射和已有 operation 的权限 enforcement；实现层另按用户明确授权删除非 SSOT 的旧 Platform `roles/user_roles` 表，并移除启动阶段的一次性历史 schema/data migration；无错误码、事件或依赖变化。

## Verification performed and remaining checks

- 已验证 `spec-v1.15.0` tag、release 状态和 commit 祖先关系。
- 已验证 `ssot` gitlink 与 `SSOT_VERSION.commit` 均为 `1445e30d800c7ae4598bed42811c787bf7d39fbd`，且该 commit 对应 tag `spec-v1.15.0`。
- 已用 `yaml.v3` 结构化核对 12 份 release 权限文件：96 个发布权限全部存在，`domain/resource/action` 一致；实现总目录为 114 项。
- 已结构化核对默认角色：`USER` 80 个发布权限，`ADMIN`/`SUPER_ADMIN` 各 91 个发布权限，三者均额外保留 3 个 Identity 前端基础权限。
- 已逐项核对 AI Chat、Model Management、Application Platform OpenAPI 中现有 operation 的 `x-permission` 与路由中间件。
- 已通过 `go test ./backend/internal/apiserver/service/v1/identity`（无测试文件，编译通过）。
- 已通过 `go test ./backend/internal/apiserver/service/v1/modelgateway`。
- 已通过 `go test ./backend/internal/apiserver/service/v1/applicationplatform`。
- 已通过 `go test ./backend/internal/apiserver/service/v1/assetlibrary`。
- 已通过 `go test ./backend/internal/apiserver/store/postgresql`。
- 已通过 `go test ./backend/internal/apiserver`（无测试文件，编译通过）。
- 已通过 `gofmt` 和 `git diff --check`；未运行全仓库测试。
- 本轮已验证容器目标为 `omnimam-postgres` / `omnimam` / `public`；旧表清理后 `to_regclass('public.user_roles')`、`to_regclass('public.roles')` 均为 `NULL`，`public.permissions` 仍存在；6 个 Workflow Canvas 旧列均不存在。
- 已确认 `0_pg.go` 和 `workflow_canvas.go` 的启动 schema SQL 不再包含 `DROP TABLE` / `DROP INDEX` / `DROP CONSTRAINT`、历史数据 `UPDATE` 或旧列探测。

## Outstanding tasks

- 当前 PostgreSQL 历史迁移清理无剩余任务。
- 上一版 Identity OPAQUE 管理员创建用户的角色应用与真实端到端回归仍需单独继续。

## Known issues and risks

- 工作区存在上一任务留下的 Identity 修改；本任务必须与其共存，不能将其误判为 v1.15.0 对齐改动。
- 上一任务尚有真实 OPAQUE `register/start` 与 `register/finish` 端到端回归未完成；除非 v1.15.0 直接涉及该链路，否则不扩大本任务范围。
- 管理员 OPAQUE 创建用户请求中的 `RoleGrants` 当前未在 `AdminRegisterStart` 路径应用，非首个账号也不会自动绑定 `USER`；这是上一版 Identity 链路的既有未完成项，本轮 v1.15.0 release 未包含 Identity 契约，故不在此无依据修改。
- Identity 默认权限没有可复用的现有单测文件；受仓库测试文件规则限制，本次依赖结构化契约比对和 package 编译测试覆盖该静态目录。
- lowercase `super_admin` Identity 授权仍由 `EnsureDefaultPermissions` 清理；旧表及其他一次性 schema 清理将改为本次容器内直接执行，不再由应用启动逻辑承担。
- 应用不再兼容旧 PostgreSQL schema；其他仍停留在旧列/旧约束定义的环境必须先由运维显式清理或重建数据库，再启动新版本。

## Exact recommended next step

单独处理 Identity OPAQUE 管理员创建用户的 `RoleGrants`/默认 `USER` 授权，并完成真实注册登录端到端回归。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
