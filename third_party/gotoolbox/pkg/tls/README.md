# tls — TLS 证书配置

**功能**：TLS/MTLS 证书配置管理，支持 HTTP/gRPC 服务端和客户端。包含子包 `grpctls`、`httptls`。

| 子包 | 函数/方法 | 说明 |
|---|---|---|
| `tls` | `LoadDataFromFile(certPath, keyPath) (string, string, error)` | 从文件加载证书 |
| `tls` | `GeneratableKeyCert.Validate() error` | 验证证书配置 |
| `tls` | `GeneratableKeyCert.CopyAndHide() *GeneratableKeyCert` | 深拷贝并隐藏敏感信息 |
| `tls` | `GeneratableKeyCert.DeepCopy() *GeneratableKeyCert` | 深拷贝 |
| `tls` | `MTLSCert.CopyAndHide() *MTLSCert` | MTLS 深拷贝并隐藏 |
| `httptls` | `NewServer()` / `NewClient()` | HTTP TLS 服务端/客户端 |
| `grpctls` | `NewServer()` / `NewClient()` | gRPC TLS 服务端/客户端 |
[← 返回包列表](../../README.md)
