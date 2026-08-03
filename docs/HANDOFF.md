# Project Handoff

## Current goal and status

- Goal: 提供测试脚本，删除 `deployments_omnimam_postgres_data` 中 OmniMAM PostgreSQL 数据库的所有业务表。
- Status: 已完成；脚本、使用文档和静态验证均已完成。
- Current SSOT: released tag `spec-v1.15.0`, gitlink `1445e30d800c7ae4598bed42811c787bf7d39fbd`；`SSOT_VERSION` 与 submodule 一致。

## Work completed in this session

- 已读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md`、相关 Compose 配置和部署文档。
- 已确认 `deployments_omnimam_postgres_data` 挂载到 `omnimam-postgres:/var/lib/postgresql/data`，默认业务数据库为 `omnimam`。
- 已新增 `deployments/postgres/drop-all-tables.sh`，要求显式传入 `--force`，并检查目标 volume、容器挂载和运行状态。
- 脚本只删除 `$POSTGRES_DB` 的 `public` schema 内所有普通表，不删除 volume、数据库或独立 `conductor` 数据库。
- 已在 `deployments/README.md` 添加测试数据库清理说明。

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Added: `deployments/postgres/drop-all-tables.sh`。
- Modified: `deployments/README.md`、`docs/HANDOFF.md`。

## Key architectural or design decisions

- 使用现有 PostgreSQL 容器内的 `psql`，不引入本地主机 PostgreSQL client 或新二进制。
- 删除范围限定为业务数据库 `public` schema 中的表；通过 `CASCADE` 处理外键和依赖关系。
- 在执行破坏性 SQL 前验证容器的数据目录确实挂载指定 volume，避免误清其他 Compose 实例。
- 本任务不改变产品语义、API、Schema 定义、migration 或运行时代码，因此不读取 SSOT 业务规范。

## API, schema, dependency, or configuration changes

- None.

## Verification performed and remaining checks

- 已确认 SSOT gitlink、release 状态与 `SSOT_VERSION.commit` 一致。
- 已通过 `sh -n deployments/postgres/drop-all-tables.sh`。
- 已验证未传 `--force` 时脚本返回状态码 `2`，且在调用 Docker 前退出。
- 已通过 `git diff --check`，并确认脚本具备可执行权限。
- 当前环境未安装 `shellcheck`，因此未运行该项检查。
- 不会在未获明确授权时对当前 PostgreSQL 数据执行真实清表测试。

## Outstanding tasks

- None.

## Known issues and risks

- 清表是不可逆操作；脚本要求 `--force`，但执行前仍应确认目标是测试环境。
- `CASCADE` 可能同时删除依赖这些表的 `public` schema 对象，这是完整删除关联表所必需的 PostgreSQL 行为。

## Exact recommended next step

在确认数据可丢弃的测试环境中执行 `deployments/postgres/drop-all-tables.sh --force`。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
