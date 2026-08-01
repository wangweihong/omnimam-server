# MCP Server 运行与安全边界

OmniMAM MCP Server 实现已发布 SSOT `spec-v1.9.2`，固定支持 MCP
`2026-07-28`。业务入口安装在 API Server 的 `POST /mcp`；仅支持
stdio 的本地 Agent 通过 `omnimam-mcp-proxy` 转发到该 HTTP 入口。

## 1. 部署形态

`POST /mcp` 与现有 API Server 共用 Identity JWT、Application Platform、
Task Center、Asset Library 和 Model Gateway 服务。MCP 只持久化
`McpTaskBinding`；ApplicationRun、AtomicTask、Artifact、Asset 和
Representation 仍由源领域拥有。

本机部署可以使用 loopback HTTP，例如
`http://127.0.0.1:8080/mcp`。任何非 loopback MCP Proxy endpoint、
`mcp.public-base-url` 或浏览器 Origin 都必须使用 HTTPS，避免 Bearer
Token 和受控内容链接通过明文网络传输。

## 2. API Server 配置

`configs/apiserver.yaml` 的 `mcp` 段控制端点：

- `enabled`：是否安装 `POST /mcp`。
- `allowed-origins`：浏览器额外 Origin 白名单；同源请求自动允许。
- `max-request-bytes`、`request-timeout`：单请求大小和执行超时。
- `discover-ttl`、`resource-ttl`：私有发现与 Resource 投影缓存建议。
- `task-ttl`、`task-poll-interval`：Task Binding 生命周期和建议轮询间隔。
- `public-base-url`：生成上传与 Representation 受控 URL 的可信 Origin；
  不从客户端 `Host` Header 派生。
- `request-rate-per-second`、`request-burst`：每 Principal 请求令牌桶。
- `tool-rate-per-second`、`tool-burst`：每 Principal/Tool 令牌桶。
- `max-upload-bytes`：MCP 创建 UploadSession 前的单文件大小门禁。
- `max-limiter-scopes`：单实例内存限流键数量上限。

生产环境应把 `APISERVER_MCP_PUBLIC_BASE_URL` 设置为 API Server 的公开
HTTPS Origin。过期 `McpTaskBinding` 会在 API Server 启动时及之后每小时
物理清理；该清理不会修改任何源领域对象。

## 3. Streamable HTTP

每个请求都必须携带：

- `Authorization: Bearer <Identity JWT>`
- `MCP-Protocol-Version: 2026-07-28`
- 与 JSON-RPC Body 一致的 `Mcp-Method`
- `tools/call` 或 `resources/read` 时与参数一致的 `Mcp-Name`
- `Content-Type: application/json`
- 接受 `application/json` 或 `text/event-stream` 的 `Accept`

非法 Origin 返回 HTTP 403；传输结构错误返回 HTTP 400。已进入 JSON-RPC
调度的业务失败使用 HTTP 200，并通过 JSON-RPC error 或 Tool Result
`isError=true` 表达。

## 4. stdio Proxy

构建：

```bash
go build ./backend/cmd/omnimam-mcp-proxy
```

运行时由 Agent 启动配置或 Secret Manager 向进程环境注入
`OMNIMAM_MCP_TOKEN`，不要把 Token 放入命令行参数、Tool 参数或日志。
`OMNIMAM_MCP_ENDPOINT` 可设置默认 HTTP endpoint，也可以使用
`--endpoint` 指定不含凭证的 URL：

```bash
omnimam-mcp-proxy --endpoint https://omnimam.example/mcp
```

Proxy 从 stdin 逐行读取 JSON-RPC，并向 stdout 逐行写回 JSON-RPC；诊断只
写 stderr。Proxy 不跟随 HTTP redirect，限制单条请求/响应大小，并校验
响应媒体类型与 JSON-RPC request ID。

## 5. 验证

提交前至少执行：

```bash
make gen
make gen.deepcopy
go test ./backend/pkg/mcp ./backend/pkg/mcpproxy
go test -race ./backend/pkg/mcp ./backend/pkg/mcpproxy
go vet ./backend/pkg/mcp ./backend/pkg/mcpproxy
go build ./backend/cmd/apiserver ./backend/cmd/omnimam-mcp-proxy
```

协议烟测应同时覆盖合法 JSON/请求绑定 SSE、非法 Origin、无效 JWT、Header
与 Body 不一致，以及 stdio Proxy 的单行输出和 redirect 拒绝。
