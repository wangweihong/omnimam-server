#  httpcli — HTTP 客户端

**功能**：功能丰富的 HTTP 客户端，支持拦截器链、多部分上传、URL 编码、日志、限流、链路追踪。包含子包 `decode`、`def`、`httpconfig`、`httphandler`、`httpresponse`、`interceptorcli`。

| 子包 | 函数/方法 | 说明 |
|---|---|---|
| `httpcli` | `NewHttpRequestBuilder() *Builder` | 创建请求构建器 |
| `httpcli` | `Builder.POST() / GET() / PUT() / DELETE()` | HTTP 方法 |
| `httpcli` | `Builder.WithEndpoint(url)` | 设置端点 |
| `httpcli` | `Builder.WithBody(contentType, body)` | 设置请求体 |
| `httpcli` | `Builder.WithHeader(key, value)` | 设置请求头 |
| `httpcli` | `Builder.Build() *Request` | 构建请求 |
| `httpcli` | `Request.Invoke() (*Response, error)` | 发送请求 |
| `httpcli` | `Response.Decode(v) error` | 解码响应 |
| `httpcli` | `NewTransport(cfg) *Transport` | 创建传输层 |
| `httpcli` | `NewInterceptor()` | 创建拦截器 |
| `httpcli` | `CallOption` | 调用选项 |
| `interceptorcli` | 提供 `decode`、`logging`、`ratelimiter`、`statuscode`、`trace` 拦截器 |
| `httphandler` | `NewHandler()` | HTTP 处理器 |
| `multihandler` | `NewMultiHandler()` | 多处理器 |
| `loghandler` | 日志处理器 |
| `httpresponse` | 响应处理工具 |
| `httpconfig` | HTTP 配置 |
| `def` | `multipart`、`urlencode` | 请求体编码 |
[← 返回包列表](../../README.md)
