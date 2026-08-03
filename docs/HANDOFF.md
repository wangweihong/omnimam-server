# Project Handoff

## Current goal and status

继续实现 released `spec-v1.12.1` 中继承自 v1.12.0 的 Agent、AppStudio、Infrastructure 和 Task Center Function Registry。当前任务已完成：5 个 AppStudio registry executor 已注册；4 个 runtime executor 可执行，`appstudio.build.execute` 在 Artifact 注册边界缺失时保持 fail closed。

## Work completed in this session

- 已在 `backend/internal/taskworker/taskworker.go` 注册 5 个 AppStudio functionRef，并实现 `appstudio.preview.ensure`、`appstudio.preview.stop`、`appstudio.production.reconcile`、`appstudio.production.stop` 的 Infrastructure create/start/stop 映射、contract pin 校验、ready/stopped 结果校验和 registry output 校验。
- `appstudio.build.execute` 已实现 contract pin、输入/profile/source/config/authorization/resource 门禁；在 Infrastructure 调用前明确 fail closed，未把 Infrastructure output descriptor 的 `ArtifactID` 冒充 Asset Library Artifact，也未伪造 ready Artifact。
- 已在现有 `backend/internal/taskworker/taskworker_test.go` 增加 Infrastructure recording fake 和 5 个 executor 的直接测试，覆盖 create/start、DELETE stop、production artifact source、build fail-closed 及错误 contract pin。
- 已运行 `gofmt -w backend/internal/taskworker/taskworker.go backend/internal/taskworker/taskworker_test.go`，目标 package 测试通过。
- 本轮已从现有 checkpoint 恢复，确认继续范围仅为 5 个 AppStudio registry executor；已读取仓库后端 Skill、`backend/AGENTS.md` 及目标模块相关规则。
- 已从现有 checkpoint 接续剩余 5 个 registry executor，并重新读取 `backend/AGENTS.md`、`skills/omnimam-server-backend/SKILL.md` 及 Go 实现/测试规则；未重复扫描仓库或 SSOT。
- 已从本 handoff 恢复当前检查点，并确认本轮不启动 Subagent、不递归扫描、只读取入口直接引用规范及目标模块源码/测试。
- 已确认 `ssot` 当前 commit `23cadc158af8e3ec3a6debaa07f44dbd292f0265` 与 `SSOT_VERSION` 的 released `spec-v1.12.1` 声明一致。
- 已确认 `ssot` 固定到 released tag `spec-v1.12.1`、commit `23cadc158af8e3ec3a6debaa07f44dbd292f0265`，与 `SSOT_VERSION` 一致。
- 已复核仓库规则、后端 Skill 和 Go 实现规则；用户已授权新增 `backend/cmd/infraserver`。
- 已确认 `agent.runtime.ensure` / `agent.runtime.stop` 分别映射 Infrastructure `ENSURE_RUNTIME` / `STOP_RUNTIME`，worker 必须按 AtomicTask 固定的 contract version/digest 调用 `Registry.Resolve`，并用 `Registry.ValidateOutput` 校验返回值。
- 已确认 `AtomicTask` 提供 `FunctionContractVersion`、`FunctionContractDigest`、`Arguments`、`CreatedBy`，Infrastructure client 使用 `CommandRequest` / `CommandResponse`，create/result DTO 位于 `backend/apis/iapiserver/{request,response}_infrastructure.go`。
- 已在 `backend/internal/apiserver/options/options.go` 增加 Infrastructure client base URL/token 配置、flags、初始化和 Workflow Runtime 启用时的校验。
- 已在 `backend/internal/taskworker/taskworker.go` 注册并实现 `agent.runtime.ensure` / `agent.runtime.stop`；实现使用 AtomicTask 参数和创建人、固定 contract、零基重试次数转换、Infrastructure create/start/stop 协议，并对 runtime/status/endpoint/output fail closed。
- 已运行目标 package 测试和 Function Registry verifier，均通过；`git diff --check` 仍会报告既有 `AGENTS.md:184` 空白行。

此前已完成并保留在工作区：

- Function Registry：嵌入并验证 7 个 ACTIVE contract，支持 JSON Schema、JCS digest、retryable metadata，接入 API Server/taskworker 启动门禁。
- Task Center：AtomicTask 固定 `function_contract_version`/`function_contract_digest`，创建策略由 registry 驱动并增加数据库约束。
- Agent：released API/model/store/service/controller/permissions/SSE/bootstrap/terminal projector 主体实现，35 个 operation 已安装。
- AppStudio：released API/model/store/service/controller/permissions/bootstrap/terminal projector 主体实现，33 个 operation 已安装；本地不可变 Revision source store 已实现。
- Infrastructure：released model/store/service、Docker provider、内部 HTTP 协议/client/server 和独立 `backend/cmd/infraserver` 主体实现；Docker 安全默认和受控 profile image 配置已实现。
- 最近一次 `make gen.deepcopy`、registry verify 和 `go test -run '^$' ./backend/...` 已通过；本轮限定验证也已完成。

## Current in-progress work

- 当前无未完成的本轮代码变更。
- 已确认 `InfraOperationResult` 返回 `Runtime`、`Outputs`、`ArtifactDigest`、`LogsRef`；`InfraRuntimeOutput.ArtifactID` 仍只是 Infrastructure output descriptor 字段。现有 Asset Library producer enum 不含 `studio_build`，且生命周期注册要求 Artifact 已 ready/有 Blob，因此 build executor 不能完成正式 Artifact 注册。

## Files added, modified, renamed, or removed

- 本轮修改：`backend/internal/apiserver/options/options.go`。
- 本轮修改：`backend/internal/taskworker/taskworker.go`。
- 本轮修改：`backend/internal/taskworker/taskworker_test.go`。
- 本轮修改：`docs/HANDOFF.md`。
- 本轮未新增、重命名或删除文件。

## Key architectural or design decisions

- Function contract 只由 pinned released SSOT 驱动；任务创建固定 version/digest，worker 执行前 fail closed。
- Task Center 保持 AtomicTask/Attempt 事实源，Conductor 保持调度事实边界；业务 handler 只执行 AtomicTask。
- Infrastructure 是独立进程；API Server 与 taskworker 通过消费方 interface + client adapter 调用，不直接依赖 Docker。
- Docker image 必须来自 `OMNIMAM_INFRA_PROFILE_IMAGES`；Source/Secret resolver 缺失时 fail closed。
- `WorkerTask.RetryCount` 为零基计数，Infrastructure create request 的 attempt number 使用 `RetryCount + 1`。
- Infrastructure wire operation 使用 `create`、`start`、`stop`；ensure 在无既有 runtime 时 create，有既有 runtime 时 start；stop 的 `DELETE` 设置 `Delete:true`。
- Infrastructure 没有独立 health 字段；仅在返回 `RUNNING` 且 endpoint 为匹配的 `READY` endpoint 时投影 `health_status: HEALTHY`。
- 不新增 SSOT 未定义的 API、表字段、错误码、权限码、事件或 functionRef。

## API, schema, dependency, or configuration changes

- AtomicTask 新增 function contract version/digest 字段与约束。
- 新增 Agent/AppStudio/Infrastructure released API DTO 与持久化实体。
- 新增 `backend/cmd/infraserver`。配置包括 `OMNIMAM_INFRA_PROFILE_IMAGES`、`OMNIMAM_INFRA_SERVICE_TOKEN`、`OMNIMAM_INFRA_LISTEN_ADDRESS`、`OMNIMAM_DOCKER_SOCKET`、`OMNIMAM_DOCKER_API_VERSION`。
- API Server options 新增 `infrastructure-client.base-url` 和 `infrastructure-client.token`；Workflow Runtime 启用时 base URL 必填，token 至少 32 bytes。

## Verification performed and remaining checks

- 已通过：`go test ./backend/internal/apiserver/options ./backend/internal/taskworker`（options 无测试文件，taskworker `ok`）。
- 已通过：实现 5 个 AppStudio handler 后执行 `go test ./backend/internal/taskworker`。
- 已通过：`go run ./backend/internal/taskfunctionregistry/internal/verify`，验证 7 个 ACTIVE contracts，SSOT commit 为 `23cadc158af8e3ec3a6debaa07f44dbd292f0265`。
- `gofmt`、`go test ./backend/internal/taskworker` 和 registry verifier 已在补测试后重新执行并通过。
- `git diff --check` 仅报告既有 `AGENTS.md:184` 空白行，未修改该无关文件。
- 未运行全仓库测试，符合当前限定验证范围。

## Outstanding tasks

- 解决 `appstudio.build.execute` 所需的正式 `studio_build` Asset Library Artifact 注册链路；在 released SSOT 或现有模块接口提供可验证身份前保持 fail closed。
- Infrastructure 15 个 released user-facing API/controller/routes、9 个 permissions、显式 schema CHECK/FK/index。
- Infrastructure SourceRef/Secret resolver 和真实 runtime output/log 恢复。
- AppStudio Build Artifact 经 Asset Library 注册，Build/Runtime logs 接真实脱敏日志，显式 schema constraints。
- Agent RuntimeAdapter/CHAT 实际执行和流式事件；非 CHAT Invocation 因 SSOT 无 canonical functionRef 保持 fail closed。
- 补足允许范围内的验证、启动检查和发布文档。

## Known issues and risks

- Released registry 缺少 Agent 非 CHAT Invocation 的 canonical functionRef，禁止私自新增；该路径只能 fail closed 并记录上游缺口。
- AgentWorkspace 在 released schema/OpenAPI 中无 canonical 表/API，禁止私自新增。
- Asset Library readable summary 不暴露 Artifact digest；AppStudio 只能信任 Task Worker 注册时固定 digest，无法跨域读取 Blob 私表。
- RuntimeProfile 未提供 released image 地址，部署必须显式配置 profile image。
- Docker provider 当前对 mounts/config bindings 的解析不完整，多数真实执行仍会 fail closed。
- 工作区含大量未提交变更，必须保留并在现状上继续。

## Exact recommended next step

下一步读取并评估 released SSOT 对 `studio_build` Artifact 注册链路的定义；在契约或现有 Asset Library 接口补齐前，继续保持 `appstudio.build.execute` fail closed，不新增未定义 API 或字段。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
