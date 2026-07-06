# urlutil — URL 工具

**功能**：URL 构建、解析、拆分。

| 函数/方法 | 说明 |
|---|---|
| `BuildURL(protocol, host, port, paths...) string` | 构建 URL |
| `SplitURL(rawURL) (scheme, host, port, error)` | 拆分为 scheme/host/port |
| `SplitURLV2(rawURL) (scheme, host, port, path, error)` | 拆分为 scheme/host/port/path |
| `Scheme(rawURL) string` | 获取协议 |
| `Domain(rawURL) string` | 获取域名 |
| `Port(rawURL) int` | 获取端口 |
| `Path(rawURL) string` | 获取路径 |
| `TrimScheme(rawURL) string` | 移除协议前缀 |
[← 返回包列表](../../README.md)
