# template — 模板处理

**功能**：Go 模板渲染、文件模板处理、目录模板处理、代码生成（将文件嵌入 Go 代码）。

| 函数/方法 | 说明 |
|---|---|
| `FileProcessor.Parse() (string, error)` | 解析模板 |
| `FileProcessor.SetContext(key, value)` / `SetContexts(context)` | 设置上下文 |
| `FileProcessor.SetFilePath(path)` / `SetFileMode(mode)` | 设置输出路径/权限 |
| `FileProcessor.WriteFile() error` | 写入文件 |
| `DirectoryProcessor` | 目录模板处理器 |
| `GenerateDataPath(packageName, nameVar, rootPath) (*bytes.Buffer, error)` | 生成嵌入数据路径代码 |
| `GenerateDataPathToFile(packageName, nameVar, rootPath, outputPath) error` | 生成嵌入数据文件 |
| `WalkRecordFileDataPath(fp, rootPath, prefix) ([]string, error)` | 遍历记录文件数据路径 |
[← 返回包列表](../../README.md)
