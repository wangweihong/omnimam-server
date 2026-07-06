# callerutil — 调用栈信息

**功能**：获取调用栈帧信息，支持格式化输出。

| 函数/方法 | 说明 |
|---|---|
| `Frame.File() string` | 获取文件路径 |
| `Frame.Line() int` | 获取行号 |
| `Frame.Name() string` | 获取函数名 |
| `Frame.Format(s fmt.State, verb rune)` | 格式化输出 |
| `Frame.MarshalText() ([]byte, error)` | 序列化为文本 |
| `Frame.String() string` | 字符串表示 |
[← 返回包列表](../../README.md)
