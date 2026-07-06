
# randutil — 随机工具

**功能**：随机字符串、数字、布尔值、加密安全随机字节生成。

| 函数/方法 | 说明 |
|---|---|
| `RandBool() bool` | 随机布尔值 |
| `RandString(runes, size) string` | 随机字符串（可指定字符集） |
| `RandStringSlice(runes, size, n) []string` | 随机字符串切片 |
| `RandNumSets(n) string` | 随机数字串（默认 6 位） |
| `RandNumRange(min, absDelta) int` | 范围内随机数 |
| `RandBytes(n) []byte` | 加密安全随机字节 |
[← 返回包列表](../../README.md)
