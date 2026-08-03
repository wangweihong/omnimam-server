# Project Handoff

## Current goal and status

- Goal: 完成 Identity OPAQUE 改造，并确保 `make compose` 不要求用户额外传入 `APISERVER_OPAQUE_SERVER_SETUP` 才能启动。
- Status: OPAQUE Server/Web 主流程及 Compose 本地默认 setup 注入已完成；目标测试、类型检查、构建和跨语言 registration/login 已通过。服务端 setup 必须稳定，生产可覆盖，setup 变更会使已有 OPAQUE registration record 失效。
- SSOT: `spec-v1.14.2`, commit `102a4477672f05b223e3bcff19b7df61e9cdc8e8`; 根目录 `SSOT_VERSION.commit` 与 `ssot` gitlink 当前一致。

## Work completed in this session

- 已读取 `skills/omnimam-server-backend/SKILL.md`、`backend/AGENTS.md`，并确认本任务属于后端配置与 Compose 运行链路修复。
- 已定位并修复问题：`configs/apiserver.yaml` 使用 `${APISERVER_OPAQUE_SERVER_SETUP}`，但 Compose 的 API Server 环境和渲染循环都没有注入该值；现已提供固定本地 setup，并接入 API Server、Infrastructure、TaskWorker、NotificationWorker 的模板渲染。
- 已确认本任务不新增 API、Schema、错误码、权限码、事件或 `backend/cmd/` 二进制。
- 已生成固定 `ServerKeyMaterial.Hex()` setup，长度 274；默认配置和四个 Compose 服务使用同一值。
- 已删除 setup 生成和跨语言互操作临时文件。

## Current in-progress work

- 完成 OPAQUE 错误密码、篡改消息、过期/重复/并发 finish 的专门验证。
- 检查管理员初始密码重置 UI 与平台认证配置残留字段。
- 完成发布前目标测试并复核文档。

## Files changed or pending

- 已有 OPAQUE 改造工作区改动：`backend/apis/iapiserver/meta_identity_v11.go`、`request_identity_v11.go`、Identity controller/service/store、`backend/internal/apiserver/options/options.go`、`configs/apiserver.yaml`、`scripts/install/environment.sh`、`SSOT_VERSION`、`ssot`、`go.mod`、`go.sum`。
- 本次重点修改：`Makefile`、`scripts/install/environment.sh`、`deployments/docker-compose.yaml`、`docs/HANDOFF.md`。
- 本轮未保留临时文件。

## Key decisions

- 本地 Compose 使用固定开发 setup 默认值，不要求用户传入 `APISERVER_OPAQUE_SERVER_SETUP` 或 `OMNIMAM_OPAQUE_SERVER_SETUP`。
- 仍允许通过 `OMNIMAM_OPAQUE_SERVER_SETUP` 覆盖 Compose 的本地默认值；该覆盖只适用于部署配置，不改变协议消息或数据库结构。
- OPAQUE setup 不写入日志、审计、事件或请求体；HTTPS 仍是必需的传输保护。

## Verification and risks

- 已通过目标 Go Identity/API Server 测试、Web Identity/Login 测试和 TypeScript 类型检查（此前 OPAQUE 改造阶段）。
- 已验证：`make configs`、`bash -n`、无 OPAQUE 外部变量的 `docker compose config --quiet`、Compose 四个服务 setup 长度均为 274、`go test ./backend/internal/apiserver/service/v1/identity ./backend/internal/apiserver`、`git diff --check`。
- 已验证：Web/Go 使用固定 setup 的 registration/login 互操作；Web Identity/Login 测试 15/15、TypeScript 类型检查和 production build 通过。
- 当前尚未专门验证：错误密码、篡改消息、过期/重复/并发 finish；这些仍需接入真实 API/store 测试环境覆盖。
- 当前工作区存在其他 OPAQUE 改造未提交修改；不得回退或覆盖无关用户改动。
- setup 默认值仅供本地开发；生产必须提供稳定且受保护的部署密钥，并在部署周期内保持不变。

## Outstanding tasks

- 在可用数据库/API 环境验证真实 OPAQUE exchange 的错误密码、篡改消息、过期、重复和并发 finish。
- 检查管理员初始密码重置 UI 与 `password_hash_policy` 残留表达，确认不再进入 OPAQUE 主流程。
- 最终复核 server/web 两仓库 SSOT pin 和发布文件。
- 完成剩余 OPAQUE 互操作和发布前验证，并再次刷新本 handoff。

## Exact next step

在真实 API/store 环境运行 OPAQUE exchange 错误场景验证，并检查管理员重置 UI 与平台认证配置残留字段。

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
