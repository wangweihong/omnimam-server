# tokenutil — Token 工具

**功能**：JWT 编解码，支持 RSA、HMAC、ECDSA 签名算法。一次性令牌（OTT）生成与验证。

| 函数/方法 | 说明 |
|---|---|
| `DefaultJWTTrackedRequestCodec(opts) JWTTrackedRequestCodec` | 创建默认 JWT 编解码器 |
| `NewRSAJWTCodec(key, maxAge) JWTTrackedRequestCodec` | 创建 RSA JWT 编解码器 |
| `NewHMACJWTCodec(key, maxAge) JWTTrackedRequestCodec` | 创建 HMAC JWT 编解码器 |
| `NewECDSAJWTCodec(key, maxAge) JWTTrackedRequestCodec` | 创建 ECDSA JWT 编解码器 |
| `JWTTrackedRequestCodec.Encode(req) (string, error)` | 编码 JWT |
| `JWTTrackedRequestCodec.Decode(signed) (*TrackedRequest, error)` | 解码 JWT |
| `NewOTT() *OTT` | 创建一次性令牌 |
| `OTT.Generate() (string, error)` | 生成一次性令牌 |
| `OTT.Validate(token) error` | 验证一次性令牌 |
[← 返回包列表](../../README.md)
