# Project Handoff

## Current goal and status

- Goal: 拉取并 pin 已发布的 SSOT `spec-v1.15.1`，按该版本实现后端 AppStudio 合同变更。
- Status: 已完成并通过目标包验证，尚未提交。

## Work completed in this session

- 已读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md` 和 v1.15.1 的 AppStudio Context/S2 变更。
- 已将 `ssot` submodule 从 `spec-v1.15.0` 更新到 `spec-v1.15.1` commit `2d15f36c8c911373034b04d7861e8add5e54f48a`。
- 已同步 `SSOT_VERSION` 的 commit、contract version、日期和 release note。
- 已从 `StudioApplicationCreateRequest`、`StudioApplication` 和创建服务中移除 `template_id`、`technology_stack`。
- 已运行 `make gen.deepcopy`；生成文件没有变化。

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Modified: `SSOT_VERSION`。
- Modified: `backend/apis/iapiserver/request_appstudio.go`。
- Modified: `backend/apis/iapiserver/meta_appstudio.go`。
- Modified: `backend/internal/apiserver/service/v1/appstudio/service.go`。
- Modified: `docs/HANDOFF.md`。
- Modified: `ssot` submodule pointer。

## Key architectural or design decisions

- 只实现 v1.15.1 已 release 的 AppStudio S2 字段清理，不新增模板、模板发现或其他能力。
- 按用户要求不增加旧表处理，不执行 `DROP COLUMN` 或额外 migration；当前模型、API 和新建数据路径不再使用废弃字段。
- `application-platform` 的独立 ApplicationTemplate 合同不在本次范围内，保持不变。

## API, schema, dependency, or configuration changes

- AppStudio 创建请求现在仅声明必填 `name` 和可选 `description`。
- AppStudio 当前持久化模型不再映射 `template_id`、`technology_stack`。
- 无依赖、错误码、权限码、事件或环境变量变更。

## Verification performed and remaining checks

- `make gen.deepcopy`：通过，生成文件无差异。
- `go test ./backend/apis/iapiserver ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/controller/v1/appstudio ./backend/internal/apiserver/store/postgresql`：通过。
- `git diff --check`：通过。
- 已确认 `git submodule status ssot`、exact tag 和 `SSOT_VERSION.commit` 均为 `spec-v1.15.1` / `2d15f36c8c911373034b04d7861e8add5e54f48a`。
- 未运行全仓库测试，符合目标模块验证约束。

## Outstanding tasks

- None.

## Known issues and risks

- 按用户要求，已有数据库中的旧列不会被本次变更主动删除；运行时代码不再读写这些列。

## Exact recommended next step

检查当前 diff 后提交 `ssot` pin、`SSOT_VERSION` 和 AppStudio 最小实现变更。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
