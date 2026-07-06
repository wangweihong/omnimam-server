## hash — 哈希工具

**功能**：支持 MD5、SHA256、SHA512、BLAKE3 哈希计算，支持文件流哈希和 HMAC 签名。

| 函数/方法 | 说明 |
|---|---|
| `NewMd5() Hasher` | 创建 MD5 哈希器 |
| `NewSha256() sha256Hasher` | 创建 SHA256 哈希器 |
| `NewSha512() Hasher` | 创建 SHA512 哈希器 |
| `NewFileStream(hasher) Hasher` | 创建文件流哈希器（自适应缓冲区） |
| `NewFileBufferStream(bufSize, hasher) Hasher` | 创建带缓冲的文件流哈希器 |
| `sha256Hasher.HmacSum(data, secret) (string, error)` | HMAC-SHA256 签名 |
| `Hasher.Sum(data) (string, error)` | 计算哈希值 |
[← 返回包列表](../../README.md)
