# remote — SSH 远程操作

**功能**：SSH 连接构建、命令执行、文件上传下载（SFTP）、SCP 传输。

| 函数/方法 | 说明 |
|---|---|
| `NewSSHBuilder() *SSHBuilder` | 创建 SSH 构建器 |
| `SSHBuilder.WithEndpoint(host) *SSHBuilder` | 设置远程地址 |
| `SSHBuilder.WithUser(user) *SSHBuilder` | 设置用户 |
| `SSHBuilder.AddAuthFromPassword(password) *SSHBuilder` | 密码认证 |
| `SSHBuilder.AddAuthFromPrivateKeyFile(path, password) *SSHBuilder` | 私钥文件认证 |
| `SSHBuilder.AddAuthFromPrivateKeyData(data, password) *SSHBuilder` | 私钥数据认证 |
| `SSHBuilder.AddHostKey(knownHostsPath) *SSHBuilder` | 添加主机密钥 |
| `SSHBuilder.Build() (*SSHClient, error)` | 构建连接 |
| `SSHCommand.Output(command) (string, error)` | 执行命令获取输出 |
| `SSHCommand.Tree(condition, dir) ([]string, error)` | 列出目录树 |
| `SSHFile.Upload(remote, local) error` | 上传文件 |
| `SSHFile.Download(remote, local) error` | 下载文件 |
| `SSHFile.ListDirectory(remoteDir) ([]os.FileInfo, error)` | 列出目录 |
| `SSHFile.ReadFile(remoteFilePath) (string, error)` | 读取远程文件内容 |
| `SSHSession.Exec(command) (string, error)` | 会话中执行命令 |
[← 返回包列表](../../README.md)
