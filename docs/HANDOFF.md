# Project Handoff

## Current goal and status

- Goal: 更新已发布 SSOT 到 `spec-v1.15.2`，并按该 release 补充新增权限。
- Status: 已完成；submodule、`SSOT_VERSION` 和 Identity/Platform 内置角色权限基线均已同步到 `spec-v1.15.2`。

- Goal: 确认前端未显示平台管理和 identity 入口是否因为后端未返回权限。
- Status: 已完成只读核查；确认权限投影字段会返回，但默认角色未授予平台管理及大部分 Identity 管理权限。

- Goal: 明确用户登录后前端判定角色/权限的正式契约。
- Status: 已完成 SSOT 与当前后端路由核查；本次为只读答复，无代码行为变更。

- Goal: 修复首个注册用户登录授权投影中 `effective_roles: null`，按已发布 Identity S2 返回有效角色来源。
- Status: 已完成实现并通过 Identity 目标包验证，已提交当前 HEAD。

## Work completed in this session

- 已确认 `spec-v1.15.2` 是 release tag，commit 为 `7eacab00bf4b505f69582e2ccc390ebc24feb774`，主题为默认角色权限规范。
- 已将 `ssot` submodule 与 `SSOT_VERSION` 更新到 `spec-v1.15.2`。
- 已核对 v1.15.2 直接变更的 Identity/Platform S1、Identity schema/module contract 及两域 `permissions.yaml`：权限码未新增，新增的是必须物化的 `default_roles` 授权基线。
- 已更新 `DefaultRolePermissions()`：USER 增加 `identity.resource_grant.read`；ADMIN 增加 Identity 用户/注册/组/服务账号读取和 Platform 概览/认证配置读取/审计读取权限；SUPER_ADMIN 再增加角色、服务账号和认证配置管理权限。

- 核对 `backend/internal/apiserver/route.go`：平台管理与 Identity 管理路由均存在并使用 `RequireIdentityPermission`。
- 核对 `backend/internal/apiserver/service/v1/identity/identity.go`：登录、刷新和 `/api/v1/iam/auth/permissions` 均生成 `authorization.permission_codes`、`effective_roles`、`allowed_actions`。
- 前序核查确认的默认角色权限缺口已由本次 `DefaultRolePermissions()` 更新修复。

- 已读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md` 和 v1.15.1 的 AppStudio Context/S2 变更。
- 已将 `ssot` submodule 从 `spec-v1.15.0` 更新到 `spec-v1.15.1` commit `2d15f36c8c911373034b04d7861e8add5e54f48a`。
- 已同步 `SSOT_VERSION` 的 commit、contract version、日期和 release note。
- 已从 `StudioApplicationCreateRequest`、`StudioApplication` 和创建服务中移除 `template_id`、`technology_stack`。
- 已运行 `make gen.deepcopy`；生成文件没有变化。
- 已新增 Identity Store 有效角色查询，覆盖直接角色与用户组角色，并过滤无效角色/授权期。
- 登录、Refresh、`GET /api/v1/iam/auth/permissions` 现在统一返回非空 `effective_roles`、`permission_codes`、`allowed_actions` 和 `session_mode`。

## Current in-progress work

- None.

## This session

- 已核对 `ssot/01_contracts/domains/identity/openapi.yaml` 的 `login_finish`、`refresh_token`、`get_current_permissions`、`get_current_user`。
- 已核对 `ssot/00_product/domains/identity/product-spec.md` 的授权投影规则：菜单/按钮/动作只使用 `permission_codes` 与 `allowed_actions`，不得按角色名授权；JWT 不包含权限码。
- 当前后端路由通过 `RequireIdentityPermission("<permission>")` 强制服务端授权，前端判定仅用于 UX，不能替代后端校验。

## Files added, modified, renamed, or removed

- Modified: `SSOT_VERSION`。
- Modified: `backend/apis/iapiserver/request_appstudio.go`。
- Modified: `backend/apis/iapiserver/meta_appstudio.go`。
- Modified: `backend/internal/apiserver/service/v1/appstudio/service.go`。
- Modified: `docs/HANDOFF.md`。
- Modified: `backend/internal/apiserver/store/store.go`。
- Modified: `backend/internal/apiserver/store/postgresql/identity_v11.go`。
- Modified: `backend/internal/apiserver/service/v1/identity/identity.go`。
- Modified: `ssot` submodule pointer。

## Key architectural or design decisions

- 只实现 v1.15.1 已 release 的 AppStudio S2 字段清理，不新增模板、模板发现或其他能力。
- 按用户要求不增加旧表处理，不执行 `DROP COLUMN` 或额外 migration；当前模型、API 和新建数据路径不再使用废弃字段。
- `application-platform` 的独立 ApplicationTemplate 合同不在本次范围内，保持不变。

## API, schema, dependency, or configuration changes

- SSOT pin 更新为 `spec-v1.15.2`；本次不新增 API、数据库字段、错误码、权限码、事件类型或依赖。
- AppStudio 创建请求现在仅声明必填 `name` 和可选 `description`。
- AppStudio 当前持久化模型不再映射 `template_id`、`technology_stack`。
- 无依赖、错误码、权限码、事件或环境变量变更。

## Verification performed and remaining checks

- `gofmt -w backend/internal/apiserver/service/v1/identity/identity.go`：通过。
- `go test ./backend/internal/apiserver/service/v1/identity ./backend/internal/apiserver/store/postgresql`：通过；Identity service 无测试文件，PostgreSQL store 测试通过。
- `git diff --check`：通过。
- 已确认 `git submodule status ssot`、exact tag 和 `SSOT_VERSION.commit` 均为 `spec-v1.15.2` / `7eacab00bf4b505f69582e2ccc390ebc24feb774`。
- 未运行全仓库测试，符合目标模块验证约束。

- `make gen.deepcopy`：通过，生成文件无差异。
- `go test ./backend/apis/iapiserver ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/controller/v1/appstudio ./backend/internal/apiserver/store/postgresql`：通过。
- `git diff --check`：通过。
- 已确认 `git submodule status ssot`、exact tag 和 `SSOT_VERSION.commit` 均为 `spec-v1.15.1` / `2d15f36c8c911373034b04d7861e8add5e54f48a`。
- 未运行全仓库测试，符合目标模块验证约束。

## Outstanding tasks

- None.

## Known issues and risks

- 登录实现使用 OPAQUE 两阶段流程；登录完成和 refresh 响应包含完整 `authorization` 投影，不能只解析 token。
- 授权版本变化后必须整体替换投影并重新请求 `/api/v1/iam/auth/permissions`，不能在客户端增量拼装角色/权限。
- `effective_roles` 现在由数据库角色授权关系实时构造；若历史数据确实没有任何角色授权，返回空数组而不是 `null`，需通过角色管理/数据修复补齐授权事实。

- 按用户要求，已有数据库中的旧列不会被本次变更主动删除；运行时代码不再读写这些列。

## Exact recommended next step

重启 API Server 或重新执行 store 初始化，使 `EnsureDefaultPermissions` 将新基线对账到现有数据库；随后重新登录或刷新授权投影确认管理入口可见。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
