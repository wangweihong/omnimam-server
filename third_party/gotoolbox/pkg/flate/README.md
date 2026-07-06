# flate — 安全 Deflate 解压

**功能**：防止解压炸弹攻击的安全 Deflate 读取器，限制最大解压 10MB。

| 函数/方法 | 说明 |
|---|---|
| `NewSaferFlateReader(r io.Reader) io.ReadCloser` | 创建安全 Flate 读取器 |

[← 返回包列表](../../README.md)
