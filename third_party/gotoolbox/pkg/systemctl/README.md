
# systemctl — 系统服务管理

**功能**：封装 systemctl 命令，支持服务重启和 daemon-reload。

| 函数/方法 | 说明 |
|---|---|
| `NewCommand() Cmd` | 创建 systemctl 命令 |
| `Cmd.Restart(svc, reload) error` | 重启服务（可选 daemon-reload） |

[← 返回包列表](../../README.md)
