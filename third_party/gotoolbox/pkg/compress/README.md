
# compress — 压缩解压

**功能**：支持 tar.gz、zip、rar、7z 格式的压缩包解压和文件查找。

| 函数/方法 | 说明 |
|---|---|
| `NewExtractor(fileName) Extractor` | 根据文件名创建解压器 |
| `Extractor.FindDirPathInTar(archivePath, targets...) (string, error)` | 在压缩包中查找目录路径 |
| `Extractor.FindFileFormatPathInTar(archivePath, formats...) (string, error)` | 按文件格式后缀查找路径 |
| `Extractor.ExtractTarGZDirectory(archivePath, targetDir, destPath) error` | 解压压缩包中指定目录 |
[← 返回包列表](../../README.md)
