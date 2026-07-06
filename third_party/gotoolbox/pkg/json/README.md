
#  json — JSON 工具

**功能**：JSON 编解码、格式化输出、RawMessage 支持。

| 函数/方法 | 说明 |
|---|---|
| `Marshal(v) ([]byte, error)` | JSON 编码 |
| `Unmarshal(data, v) error` | JSON 解码 |
| `MarshalIndent(v, prefix, indent) ([]byte, error)` | 格式化编码 |
| `PrintStructObject(data)` | 打印结构体（格式化 JSON） |
| `PrettyPrint(b)` | 美化打印 JSON 字节 |
| `ToString(obj) string` | 转为 JSON 字符串 |
| `Encode(o) ([]byte, error)` | 编码 |
| `Decode(y, o) error` | 解码 |
| `ShouldEncode(params) string` | 编码（忽略错误） |
| `ShouldDecode(b) map[string]any` | 解码为 map（忽略错误） |
| `ShouldMap(params) map[string]any` | 结构体转 map（忽略错误） |
| `RawMarshal(v) ([]byte, error)` | 原始 JSON 编码（不转义） |
[← 返回包列表](../../README.md)
