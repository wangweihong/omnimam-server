#  excel — Excel 导入导出

**功能**：提供泛型 Excel 文件导入导出，支持自定义字段解析器和导出器。

| 函数/方法 | 说明 |
|---|---|
| `ImportFromFile[T](ctx, pr, sheet, fileName, failFields...) ([]T, error)` | 从文件导入 Excel |
| `Import[T](ctx, pr, xlsx, sheet, failFields...) ([]T, error)` | 从 Excel 对象导入 |
| `Export[T](ctx, pr, f, sheet, rawData, hideFields...) error` | 导出到 Excel 对象 |
| `ExportToFile[T](ctx, pr, fileName, rawData, sheet, hideFields...) error` | 导出到文件 |
| `NewExportRegistry() *ExportRegistry` | 创建导出注册中心 |
| `NewimportParserRegistry() *importParserRegistry` | 创建导入解析器注册中心 |
| `ExportRegistry.RegisterDefault(kind, exporter)` | 注册默认导出器 |
| `ExportRegistry.RegisterTypeExporter(typ, exporter)` | 注册类型导出器 |
| `ExportRegistry.RegisterFieldExporter(key, exporter)` | 注册字段导出器 |
[← 返回包列表](../../README.md)
