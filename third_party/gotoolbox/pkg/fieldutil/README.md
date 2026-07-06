# fieldutil — 结构体字段工具

**功能**：反射解析结构体字段、标签、值，支持结构体转 map。

| 函数/方法 | 说明 |
|---|---|
| `ParseStructFields(s any) Fields` | 解析结构体字段 |
| `ParseStructFieldValues(s any) FieldValues` | 解析结构体字段和值 |
| `ParseStructFieldTags(s any, tagName, hideFields...) FieldTags` | 解析结构体字段和标签 |
| `GetFieldTagMapping(s any) map[string]string` | 获取字段标签映射 |
| `StructToMap(obj, hideKeys...) map[string]any` | 结构体转 map |
| `Fields.ToMap() map[string]reflect.StructField` | 字段转 map |
| `FieldTags.ToMap() map[string]FieldTag` | 标签转 map |
| `FieldTags.ToTagMap() map[string]FieldTag` | 标签名映射 |
| `FieldTags.Tags() []string` | 获取所有标签值 |

[← 返回包列表](../../README.md)
