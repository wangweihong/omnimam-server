# validation — 验证框架

**功能**：基于 go-playground/validator 的验证器，支持中英文翻译、自定义验证规则。包含子包 `field`。

| 函数/方法 | 说明 |
|---|---|
| `NewValidator() *CustomValidator` | 创建验证器 |
| `CustomValidator.Validate(obj) error` | 执行验证 |
| `CustomValidator.SetLanguage(lang)` | 设置语言（en/zh） |
| `IsQualifiedName(value) []string` | 验证合格名称 |
| `IsValidLabelValue(value) []string` | 验证标签值 |
| `IsDNS1123Subdomain(value) []string` | 验证 DNS 子域名 |
| `IsDNS1123Label(value) []string` | 验证 DNS 标签 |
| `ValidateDir(fl) bool` | 验证目录存在 |
| `ValidateFile(fl) bool` | 验证文件存在 |
| `ValidateDescription(fl) bool` | 验证描述长度 |
| `ValidateName(fl) bool` | 验证名称 |
| `field.Path` | 字段路径 |
| `field.Errors` | 字段错误列表 |
[← 返回包列表](../../README.md)
