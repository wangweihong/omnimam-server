# executil — 命令执行

**功能**：执行系统命令，支持 Linux/Windows 平台。

| 函数/方法 | 说明 |
|---|---|
| `Execute(command, args) (string, error)` | 执行系统命令并返回输出 |
| `NewCommander() *Commander` | 创建命令执行器 |
| `Commander.Execute(command, args...) (string, error)` | 链式执行命令 |
[← 返回包列表](../../README.md)
