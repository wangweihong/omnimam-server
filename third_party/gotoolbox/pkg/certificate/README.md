# certificate — 证书管理

**功能**：提供 X509 和国密(GM)证书的生成、签名、验证功能。包含子包 `x509`、`gm`、`helper`。

| 子包 | 函数/方法 | 说明 |
|---|---|---|
| `certificate` | `NewCertificateCrypto(t CertificateType)` | 创建证书加密器 |
| `certificate` | `CertificateCrypto.PrivateSign(msg)` | 私钥签名 |
| `certificate` | `CertificateCrypto.PublicVerify(msg, sig)` | 公钥验证 |
| `certificate` | `CertificateCrypto.PublicSign(msg)` | 公钥加密 |
| `certificate` | `CertificateCrypto.PrivateVerify(msg, sig)` | 私钥解密 |
| `x509` | `NewPrivateKey()` | 生成 X509 私钥 |
| `x509` | `NewSelfSignedCert(cfg, key, IsCa)` | 生成自签名证书 |
| `x509` | `NewCertificateCrypto(privateKey)` | 创建 X509 加密器 |
| `gm` | `NewPrivateKey()` | 生成国密私钥 |
| `gm` | `NewSelfSignedCert(cfg, key, IsCa)` | 生成国密自签名证书 |
| `gm` | `NewCertificateCrypto(privateKey)` | 创建国密加密器 |
[← 返回包列表](../../README.md)
