# version — 版本信息

**功能**：编译时注入版本信息（GitVersion、GitCommit、BuildDate 等），支持格式化输出。

| 函数/方法 | 说明 |
|---|---|
| `Get() Info` | 获取版本信息 |
| `Info.String() string` | 字符串格式 |
| `Info.ToJSON() string` | JSON 格式 |
| `Info.Text() ([]byte, error)` | 表格格式 |
| `verflag` | 版本命令行标志 |

[← 返回包列表](../../README.md)
