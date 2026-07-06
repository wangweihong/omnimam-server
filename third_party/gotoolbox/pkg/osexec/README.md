# osexec — 系统文件操作

**功能**：文件压缩/解压、校验和计算、文件复制/移动、tar 打包（Linux）。

| 函数/方法 | 说明 |
|---|---|
| `GetFileChecksum(filePath) (string, error)` | 获取文件 SHA512 校验和 |
| `CompressFile(filePath) error` | gzip 压缩文件 |
| `DecompressFile(filePath) error` | gunzip 解压文件 |
| `CompressDir(sourceDir, targetFile) error` | 压缩目录为 tar.gz |
| `DecompressDir(sourceFile, targetDir) error` | 解压 tar.gz 到目录 |
| `Copy(src, dst) error` | 复制文件/目录 |
| `MoveFile(src, dst) error` | 移动文件 |
| `Tar(src, dst) error` | 创建 tar 包 |
[← 返回包列表](../../README.md)
