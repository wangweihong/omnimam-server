# Project Handoff

## Current goal and status

增加 Infrastructure 镜像构建和 Compose 服务，同时保留既有 Identity / Platform Manager 与 Compose 默认 JWT 工作区改动。

状态：已完成并验证完整 `make compose` 启动链路。`ssot/` 已固定到 `spec-v1.13.0` 并同步 `SSOT_VERSION`。

本轮已从 live checkpoint 恢复；注册申请查询、提交/重新申请、审批事务和 HTTP 分层已实现并完成目标模块验证。

## Work completed in this session

- 已复现 `make compose` 在 `scripts/gen_default_config.sh` 调用 `genconfig.sh` 时因 `APISERVER_AUTH_JWT_SECRET` 未设置退出。
- 已确认当前任务聚焦默认配置生成链路，不涉及新增 API、Schema、错误码、权限或事件。
- 已在 `scripts/install/environment.sh` 增加仅供本地默认配置生成的 46 字节开发 JWT 占位密钥，仍允许 `APISERVER_AUTH_JWT_SECRET` 显式覆盖。
- 已运行 `make configs`，生成 `_output/configs/apiserver.yaml` 成功且已替换 JWT 密钥占位符。
- 已运行目标脚本 `bash -n` 检查，并以测试密钥运行 `docker compose -f deployments/docker-compose.yaml config --quiet`，均通过。
- 用户明确要求 `make compose` 使用固定默认 JWT secret；直接执行 Compose 文件仍保留显式密钥门禁。
- 本轮任务分类为构建配置与产物诊断，不涉及 API、Schema、错误码、权限或事件变更。
- 已确认 `backend/cmd/infraserver/infraserver.go` 存在，`BINS` 自动发现包含 `infraserver`，但 `IMAGES` 自动发现仅包含 `apiserver`、`notificationworker`、`taskworker`。
- 已确认 `build/docker/` 没有 `infraserver` 镜像目录，Compose 也没有 `infraserver` 服务；因此 `make image` / `make compose` 不会构建或启动 Infrastructure。
- 已运行 `make go.build.linux_amd64.infraserver` 成功，生成 `_output/platforms/linux/amd64/infraserver`（41.6 MB），排除 Go 源码编译失败。
- 已确认 Infrastructure 进程需要共享 API Server 配置模型、PostgreSQL、Docker socket、服务 token、监听地址和非空 profile image JSON；TaskWorker 需要 `infrastructure-client.base-url/token`。
- 已新增 `build/docker/infraserver/Dockerfile.build` 与 `Dockerfile.gobuild`，并将 `infraserver` 纳入镜像自动发现和 `apiserver.yaml` 配置复用映射。
- 已在共享配置模板和三个复用配置的 Compose 服务中接入 Infrastructure client 地址/token；新增 `infraserver` 服务、Docker socket、健康检查、profile image 映射和 TaskWorker 健康依赖。
- 已为 `make compose` 增加 Infrastructure service token 与本地 profile image JSON 默认值，并允许通过同名 Make/环境变量覆盖。
- 已更新 `configs/README.md` 和 Compose 顶部部署说明。
- 用户反馈 `omnimam-infraserver` 容器在 `make compose` 后进入 Error；已完成容器退出状态、日志、健康检查和启动挂载排查。
- 已确认容器退出码为 1，日志失败点为 `platform_audit_logs.occurred_at` 非空列迁移。
- 已只读核对 PostgreSQL：`platform_audit_logs` 有 48 条旧记录，表中尚无 `occurred_at` 列；`AutoMigrate` 直接执行 `ADD occurred_at timestamptz NOT NULL`，触发 `SQLSTATE 23502`。
- 已停止并移除 `deployments` Compose 容器，删除 `deployments_omnimam_postgres_data` 成功；assets、Conductor queue、logs 和 debug 卷均保留。
- 已确认旧 PostgreSQL、API Server、Infrastructure、TaskWorker、NotificationWorker、Conductor 和 Redis 容器均已移除；默认网络仍被既有 `omnimam-frontend` 使用，因此未强制删除。
- 用户要求继续修复 `make compose`，直到完整 Compose 服务验证成功。
- 已确认容器启动命令、配置替换、镜像和 Docker socket 均正常；退出码为 1，日志失败点为 `platform_audit_logs.occurred_at` 非空列迁移。
- 已只读核对 PostgreSQL：`platform_audit_logs` 当前有 48 条旧记录，表中尚无 `occurred_at` 列；`AutoMigrate` 直接执行 `ADD occurred_at timestamptz NOT NULL`，触发 `SQLSTATE 23502`。
- 用户明确授权清理旧 PostgreSQL volume；仅删除 PostgreSQL 数据卷，不删除其他 Compose 数据卷。
- 已在 Makefile `compose` 目标的 `down` / `up` 命令中注入固定开发默认值，并保留 `OMNIMAM_AUTH_JWT_SECRET` 覆盖能力。
- 已验证无环境变量和显式覆盖两种 `make -n -o image compose` 命令展开结果。
- 已用固定默认值运行 `docker compose -f deployments/docker-compose.yaml config --quiet`，当前通过。

- 确认当前 `ssot/` 为 released commit `56907857c38992c16ae272b20aff957aae366490`（`spec-v1.13.0`）。
- 确认 `SSOT_VERSION` 与当前 submodule 一致，且声明为 released。
- 读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md` 和 Go 任务路由规则。
- 将任务分类为跨 Identity / Platform Manager 的 API、权限与可能的数据持久化契约变更；必须先通过 `spec-v1.13.0` release 门禁。
- 已确认 `spec-v1.13.0` 为正式 release tag，commit 为 `56907857c38992c16ae272b20aff957aae366490`，release 日期为 2026-08-03。
- 已将 `ssot/` pin 到该 commit，并同步根目录 `SSOT_VERSION`。
- 已核对 `ssot/RELEASE.md`：`spec-v1.13.0` 明确 `status: released`、`allowed_as_formal_implementation_basis: true`，覆盖 `identity` 与 `platform-management` 的 S1/S2。
- 已确认两个 Domain Context 仍残留“未 Release 草稿”描述；正式 release 记录优先，server 仓库不直接修改该 SSOT 文档不一致。
- 已提取 Identity v1.13.0 增量：注册审批、统一授权投影、用户删除依赖检查、服务账号直接角色/凭据管理/token exchange，以及对应权限、错误码和事件变化。
- 已提取 Platform Manager v1.13.0 增量：结构化认证配置校验、审计 `occurred_at`、服务端内容指纹、三元幂等作用域、敏感字段/JSON 边界校验、默认排序及配置更新原子审计要求。
- 已定位目标 API、service、controller、PostgreSQL store、route 与错误码文件。
- 已将认证配置密码/登录失败策略从 `json.RawMessage` 改为 SSOT 定义的强类型结构。
- 已为审计模型增加 `occurred_at`、服务端 `content_fingerprint`，并将幂等唯一约束改为 `(source_domain, source_module, idempotency_key)`。
- 已增加 Platform 审计受控列表 DTO、20/100 分页、允许的搜索/排序字段和 RFC3339 时间参数校验，概览摘要增加 `occurred_at`。
- 已注册 `230400`、`230401`、`230404` 三个 Platform 稳定错误码。
- 已为认证配置替换和内部审计追加实现严格 JSON 解码：拒绝未知字段、缺失必填字段、非对象和尾随 JSON，并映射到 Platform 稳定错误码。
- 已实现认证配置全部范围/跨字段校验，以及审计来源/action/长度/枚举/未来时间、16 KiB/32 属性/4 层 detail 和敏感内容校验。
- 已使用现有 `github.com/gowebpki/jcs` 依赖生成 RFC 8785 JCS + SHA-256 内容指纹。
- 已将概览限制为 `platform.auth_config.update` 最近 10 条并补充 `occurred_at`；审计详情不可见改用 `230400`。
- 已在 Platform 模型 hook 内定向落实配置单例 `id/name=default`、配置/审计/Outbox 初始 `resource_version=0`，并固定审计 `updated_at=created_at`、`extend_shadow=''`。
- 已将认证配置更新的委托用户 `actor_user_id` 从可信 PrincipalContext 透传为 store 瞬态字段，供配置审计和 Outbox 使用。
- 已新增 `backend/internal/pkg/platformaudit.CanonicalJSONDigest`，统一生成 RFC 8785 JCS + SHA-256 小写 64 位十六进制指纹（不带 `sha256:` 前缀）。
- 已实现认证配置单例读取与默认值、悲观锁 + 乐观版本替换，以及配置更新、配置审计、审计 Outbox、配置 Outbox 的单事务写入。
- 已实现审计 `(source_domain, source_module, idempotency_key)` 三元幂等：相同指纹回放原记录，不同指纹返回 `230404`，并处理并发唯一冲突后的事务外回读。
- 已实现审计追加与 `platform.audit.recorded` Outbox 原子写入，以及精确过滤、时间范围、允许的模糊搜索字段、允许的排序字段和稳定 ID 次级排序。
- 已运行 `go test ./internal/apiserver/store/postgresql`，当前通过。
- 已在 schema 初始化中删除旧版 `idx_platform_audit_logs_idempotency_key` 单列唯一索引，使不同来源可复用同一幂等键并由三元索引约束。
- 已修正 `identity_v11` 审计中间件：action 改为 dotted-lowercase，路由写入 target，补齐 `occurred_at` 并通过共享 helper 生成 `content_fingerprint`。
- 已运行 `go test ./internal/apiserver/middleware`，当前通过（包内尚无测试文件）。
- 已增加 Identity v1.13.0 契约骨架：注册申请、服务账号角色授权、用户删除检查模型，角色授权有效期与服务账号凭据生命周期字段，以及对应请求/响应 DTO。
- 已将新增 Identity 模型加入 schema 初始化，并将服务账号凭据轮换响应改为结构化凭据元数据 + 仅一次返回的 `client_secret`。
- 已按 `ssot/01_contracts/domains/identity/errors.yaml` 增加并注册 22 个 Identity 稳定错误码。
- 已格式化 Identity 契约骨架；`go test ./apis/iapiserver ./internal/pkg/code` 通过。
- 已将 Identity `validatePassword` 从旧 `json.RawMessage` 迁移到强类型 `PlatformPasswordPolicy`，并落实 `max_length`、大小写/数字/特殊字符及 `disallow_username`。
- 已重跑 `go test ./apis/iapiserver ./internal/pkg/code ./internal/apiserver`，当前全部通过。
- 已按 Identity Context 直接引用的 S1/S2 核对注册审批默认角色：稳定内置角色编码为 `USER`；批准事务必须通过 `Role.code=USER` 解析并创建用户角色授权，SSOT 未定义可硬编码的角色 ID。
- 已确认注册审批权限码为 `identity.registration.review`，覆盖列表、详情、批准与拒绝。
- 已扩展 Identity store 边界，实现注册申请列表、详情、ADMIN_APPROVAL 注册/拒绝后重新申请，以及按“申请 → 用户”锁顺序的批准/拒绝事务。
- 审批事务已原子写入用户状态、默认 `USER` 角色授权、Identity Outbox、Platform AuditLog 和 Platform Audit Outbox；相同决定幂等、相反决定返回稳定冲突错误，审计失败回滚全部变更。
- 已接入四个注册申请管理路由、权限 `identity.registration.review`，并阻止 PENDING/REJECTED 用户登录创建会话。
- OPEN 注册改为原子创建用户、默认 `USER` 授权、可靠事件和平台审计，同时保留首个本地用户 bootstrap `super_admin` 行为；ADMIN_APPROVAL 注册不创建角色、会话或 Token。
- 已在 Identity 初始化权限过程中确保活动内置 `USER`、`ADMIN`、`SUPER_ADMIN` 角色存在，并增加待审批申请唯一索引。
- 最新 Compose 失败根因为 `EnsureDefaultPermissions` 复用 `IdentityRole` 查询变量：最终查询 `super_admin` 时携带上一轮 `SUPER_ADMIN` 的主键，返回 `record not found` 并回滚初始化事务。
- 已在最终 bootstrap role 查询前清空复用模型，避免 GORM 自动叠加旧主键条件。
- 历史 Infrastructure 失败根因为 Docker Engine 最低 API 版本为 `1.44`，Compose 与 `NewDockerProvider` 默认仍使用 `v1.43`。
- 已将 Compose 和 `NewDockerProvider` 的默认 Docker API 版本统一改为 `v1.44`，仍允许 `OMNIMAM_DOCKER_API_VERSION` 覆盖。
- 第二次 Compose 暴露 PostgreSQL `deadlock detected (SQLSTATE 40P01)`：Infrastructure 仅等待 API Server TCP 端口打开，仍与 API Server 并发执行 `AutoMigrate`。
- 已将 Infrastructure、TaskWorker、NotificationWorker 的启动等待改为 API Server `/healthz` 返回 HTTP 200，确保共享 schema 初始化完成后再连接数据库。
- 已运行最新 `make compose` 并以退出码 0 完成配置生成、四个镜像构建、Compose 重启和服务启动。
- 已确认 API Server、Infrastructure、TaskWorker、NotificationWorker 均 `running`，Infrastructure 为 `healthy`，四个容器重启次数均为 0。
- 已确认宿主端 `http://127.0.0.1:8080/healthz` 与 `http://127.0.0.1:8082/healthz` 均返回 HTTP 200，最近容器日志无 deadlock、Docker API 版本错误、panic 或 fatal 错误。

## Current in-progress work

- 本轮无进行中的部署修复；完整 Compose 已验证成功。

- 注册审批闭环、目标包验证和 handoff 已完成；不属于本次配置问题的既有工作。
- 事务实现前置约束：先锁定 RegistrationApplication，再锁定关联 User；`USER` 角色缺失或审计写入失败时整笔回滚，不在审批路径临时创造未明确约定的角色事实。
- 授权投影、用户删除依赖、服务账号角色/凭据与 token exchange 仍是后续增量；其中服务账号 Token exchange 需先解决 SSOT 中 `AuthSession.user_id` 非空约束与服务主体会话模型的一致性，当前不扩大本轮实现。

## Files changed

- `docs/HANDOFF.md`：切换为本次任务的 live checkpoint。
- `scripts/install/environment.sh`：补齐 `APISERVER_AUTH_JWT_SECRET` 的本地默认值。
- `Makefile`：为 `compose` 目标提供并导出固定开发 JWT secret 默认值。
- `configs/apiserver.yaml`：增加共享 `infrastructure-client` 配置段。
- `scripts/install/environment.sh`：增加 Infrastructure client 默认地址/token。
- `scripts/make-rules/image.mk`：让 `infraserver` 镜像复用 `apiserver` 配置生成物。
- `build/docker/infraserver/Dockerfile.build`、`Dockerfile.gobuild`：新增 Infrastructure 镜像构建定义。
- `deployments/docker-compose.yaml`：新增 `infraserver` 服务、健康检查、Docker socket、token/profile image 配置及 TaskWorker 依赖。
- `configs/README.md`：记录 Infrastructure 配置复用方式。
- `_output/platforms/linux/amd64/infraserver`：通过单目标构建生成的本地诊断产物（构建输出，不纳入仓库提交）。
- `ssot`：submodule gitlink 当前固定为 released `56907857c38992c16ae272b20aff957aae366490`（`spec-v1.13.0`）。
- `SSOT_VERSION`：同步为 `spec-v1.13.0` release commit。
- `backend/apis/iapiserver/meta_platform_management.go`：强类型认证策略；审计时间、内容指纹和组合幂等索引；Platform 专用创建元数据 hook。
- `backend/apis/iapiserver/request_platform_management.go`：强类型请求/响应、审计查询约束和概览时间。
- `backend/internal/pkg/code/identity_platform.go`：新增 Platform 审计错误码常量。
- `backend/internal/pkg/code/code_generated.go`：注册新增 Platform 审计错误码。
- `backend/internal/apiserver/controller/v1/platformmanagement/platform.go`：Platform 专用严格 JSON 解码和查询错误映射。
- `backend/internal/apiserver/service/v1/platformmanagement/service.go`：认证配置与审计语义校验、JCS 指纹、概览过滤及配置 actor 透传。
- `backend/internal/pkg/platformaudit/fingerprint.go`：共享 RFC 8785 JCS + SHA-256 小写十六进制指纹。
- `backend/internal/apiserver/store/postgresql/platform_management.go`：配置/审计原子事务、Outbox、三元幂等与受控列表查询。
- `backend/internal/apiserver/store/postgresql/0_pg.go`：Platform 旧审计单列唯一索引兼容清理。
- `backend/internal/apiserver/middleware/identity_v11.go`：合规 action、审计时间、路由 target 与内容指纹。
- `backend/apis/iapiserver/meta_identity_v11.go`：Identity v1.13.0 注册申请、服务账号角色授权、删除检查模型及角色/凭据增量字段。
- `backend/apis/iapiserver/request_identity_v11.go`：Identity v1.13.0 请求、响应与授权投影 DTO。
- `backend/internal/apiserver/server.go`：注册新增 Identity 持久化模型。
- `backend/internal/apiserver/store/postgresql/identity_admin.go`：注册申请查询、提交/重提、批准/拒绝事务、Identity Outbox 与平台审计原子写入。
- `backend/internal/apiserver/store/postgresql/identity_v11.go`：初始化活动内置 `USER`、`ADMIN`、`SUPER_ADMIN` 角色。
- `backend/internal/apiserver/store/postgresql/identity_v11.go`：修复 GORM 复用模型残留主键导致的 bootstrap role 查询失败。
- `backend/internal/infrastructure/docker_provider.go`：将未配置时的 Docker API 默认版本提升为 `v1.44`。
- `deployments/docker-compose.yaml`：将 Infrastructure Docker API 默认版本提升为 `v1.44`。
- `deployments/docker-compose.yaml`：三个依赖 API Server 的进程改为等待 `/healthz` HTTP 200，避免并发数据库迁移。
- `backend/internal/apiserver/store/postgresql/0_pg.go`：注册申请 PENDING 唯一索引和状态查询索引。
- `backend/internal/apiserver/store/store.go`：Identity 注册申请视图与管理事务接口。
- `backend/internal/apiserver/controller/v1/identity/v11.go`、`backend/internal/apiserver/route.go`：注册申请管理 HTTP 接口和权限路由。
- `backend/internal/pkg/code/identity_platform.go`、`backend/internal/pkg/code/code_generated.go`：新增并注册 Identity v1.13.0 稳定错误码。
- `backend/internal/apiserver/service/v1/identity/identity.go`：消费强类型密码策略并执行完整密码规则。

## Key decisions

- 不递归扫描 `ssot/` 或整个仓库。
- 在 release commit、tag 和 `SSOT_VERSION` 未一致前，不修改 API、Schema、权限、错误码或运行行为。
- 不在 `backend/cmd/` 新增二进制。
- `make compose` 使用固定开发占位密钥；直接执行 `docker compose` 仍要求显式注入 `OMNIMAM_AUTH_JWT_SECRET`。
- Infrastructure 使用 Docker socket 是其 Docker provider 的必要权限边界；服务通过独立 token 对 TaskWorker 提供内部 HTTP 接口，不直接复用 Identity JWT。

## API, schema, dependency, and configuration changes

- 配置：`SSOT_VERSION.commit` 和 `contract_version` 已更新。
- 配置生成：`scripts/install/environment.sh` 为 `APISERVER_AUTH_JWT_SECRET` 提供本地开发默认值，不改变 Compose 的运行时密钥要求。
- Compose：`make compose` 为 `OMNIMAM_AUTH_JWT_SECRET` 提供固定默认值，并允许命令行或环境变量覆盖。
- Compose：`infraserver` 使用 `omnimam/infraserver` 镜像监听容器端口 `8082`，挂载 `/var/run/docker.sock`，并由 PostgreSQL 健康状态启动；TaskWorker 等待其健康。
- Identity 待实现 API：管理员初始密码重置与用户角色替换、删除依赖检查、目录用户/组、服务账号角色/凭据撤销及 token exchange。
- Identity 待实现 schema：注册申请、服务账号角色授权、凭据生命周期字段、用户删除检查及明细；用户状态增加 `REJECTED`。
- Platform Manager schema 模型已增加 `occurred_at`、`content_fingerprint`，唯一性已改为 `(source_domain, source_module, idempotency_key)`；数据库事务、双 Outbox 和 `occurred_at DESC, id DESC` 查询已实现。

## Verification performed and remaining checks

- 已运行 `git status --short`、`git submodule status` 并读取当前 `SSOT_VERSION`。
- 已运行两个候选正式上游的 `git ls-remote --heads --tags` release 门禁检查。
- 已完成 release tag、submodule commit 与 `SSOT_VERSION` 一致性校验、目标模块测试和格式检查。
- 已运行 `go test ./internal/apiserver/controller/v1/platformmanagement ./internal/apiserver/service/v1/platformmanagement`，当前通过（两个包尚无测试文件）。
- 已运行 `go test ./internal/apiserver/store/postgresql`，当前通过。
- 已运行 `go test ./apis/iapiserver ./internal/pkg/code ./internal/apiserver`，当前通过。
- 已运行 `go test ./internal/apiserver/service/v1/identity ./internal/apiserver/controller/v1/identity`，当前通过（目标包无测试文件）。
- 已运行 `go test ./internal/apiserver/controller/v1/platformmanagement ./internal/apiserver/service/v1/platformmanagement ./internal/apiserver/middleware`，当前通过（目标包无测试文件）。
- 已运行 `git diff --check`，当前通过。
- 已运行 `make configs`，当前通过。
- 已运行 `bash -n scripts/install/environment.sh scripts/genconfig.sh scripts/gen_default_config.sh`，当前通过。
- 已运行固定默认值的 `docker compose ... config --quiet`，当前通过。
- 已运行 `make -s --eval=...` 确认 `IMAGES` 包含 `infraserver`，且 `make image.build` 计划包含其二进制和 Docker image 构建步骤。
- 已运行 `make image.build.linux_amd64.infraserver`，镜像 `omnimam/infraserver:bbf9382-amd64` 构建成功。
- 已运行 `docker run --rm --entrypoint cat omnimam/infraserver:bbf9382-amd64 /etc/omnimam/infraserver.yaml`，确认镜像内配置包含 `infrastructure-client` 和 JWT 配置。
- 已运行 `go test ./backend/internal/infrastructure`，当前通过（包内无测试文件）。
- 已解析包含 `infraserver` 的 Compose service 列表并运行 `docker compose ... config --quiet`，当前通过。
- 已运行最新 `make compose`，命令退出码为 0；Compose 状态确认 API Server、Infrastructure、TaskWorker、NotificationWorker 均运行，Infrastructure 健康检查通过且重启次数为 0。
- 已通过宿主端 HTTP 检查确认 API Server 和 Infrastructure `/healthz` 均返回 200，并检查最近日志无 deadlock、Docker API 版本错误、panic 或 fatal 错误。
- 历史启动失败已定位为 Infrastructure 与 API Server 并发 `AutoMigrate` 触发 PostgreSQL deadlock，现已通过等待 API Server `/healthz` 解决。
- 未连接外部 PostgreSQL 执行审批集成测试；当前验证覆盖编译、目标包测试、格式和 SSOT pin 一致性。

## Outstanding tasks

- 本轮无阻塞任务。后续 Identity v1.13.0 增量仍包括服务账号 token exchange、删除依赖检查等未纳入本轮的功能。

## Known issues and risks

- 必须限制实现为 `spec-v1.13.0` 明确的 Identity / Platform Manager 变化，避免顺带实现无关内容。
- `docs/HANDOFF.md` 在任务开始前已有未提交修改；本轮在其现有内容上维护，不回退其他用户工作。
- `ssot/domains/identity/context.md` 与 `ssot/domains/platform-management/context.md` 的“当前状态”仍称 S1/S2 未 Release，与 `ssot/RELEASE.md` 的 `spec-v1.13.0` 正式 release 记录不一致；本仓库只消费 release，不修改 submodule 内容。
- 全局 `ObjectMeta.BeforeCreate` 默认把 `resource_version` 设为 1 且分别生成创建/更新时间；Platform schema 要求配置 singleton `id = default`、审计和 Outbox 初始版本为 0、审计 `updated_at = created_at`，必须在 Platform 模型 hook 内定向覆盖，不能修改全局行为。
- 旧版审计表可能保留 `idempotency_key` 单列唯一索引；仅靠 AutoMigrate 新增三元唯一索引不足以允许不同来源复用同一 key，需要在 Platform schema 初始化阶段做窄范围兼容清理。
- `make compose` 的固定默认值仅适合本地开发，不应作为生产环境 JWT secret；生产环境应通过 Make 变量或环境变量覆盖。
- Infrastructure Compose 服务挂载 Docker socket，拥有宿主 Docker 控制能力；仅适合受信任的本地开发环境，生产环境需单独评估隔离和授权方案。
- `make compose` 使用固定开发 JWT 和 Infrastructure token；生产环境必须通过 Make 变量或环境变量覆盖。
- Infrastructure Compose 服务挂载 Docker socket，拥有宿主 Docker 控制能力；仅适合受信任的本地开发环境。

## Exact recommended next step

本轮已完成；后续任务先读取本文件并从未实现的 Identity 增量继续。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
