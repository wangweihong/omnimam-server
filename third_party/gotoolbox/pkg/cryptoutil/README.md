# cryptoutil — 加密工具

**功能**：提供 MD5 加密和 AES/DES 加解密，支持多种模式（ECB/CBC/CTR/OFB/CFB）、填充（PKCS5/ZERO）和格式（BASE64/HEX）。

| 子包 | 函数/方法 | 说明 |
|---|---|---|
| `cryptoutil` | `Md5Encrypt(data) (string, error)` | MD5 加密 |
| `aes` | `NewCrypto(standard, mode, padding, format) Crypto` | 创建 AES/DES 加密器 |
| `aes` | `Crypto.Encrypt(src, key, vector) (string, error)` | 加密 |
| `aes` | `Crypto.Decrypt(src, key, vector) (string, error)` | 解密 |
| `aes` | `EncyptPassword(origin) (string, error)` | 密码加密（预设密钥） |
| `aes` | `DecryptPassword(origin) (string, error)` | 密码解密（预设密钥） |

[← 返回包列表](../../README.md)
