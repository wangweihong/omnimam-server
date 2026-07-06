# maputil — Map 工具

**功能**：泛型 map 操作 + 类型化 map 封装，支持嵌套 map 数据提取。

### 泛型函数

| 函数/方法 | 说明 |
|---|---|
| `Get[K, V](m, key) V` | 安全获取值（返回零值） |
| `Has[K, V](m, key) bool` | 检查键是否存在 |
| `Delete[K, V](m, keys...)` | 删除键 |
| `Insert[K, V](m, key, v) map[K]V` | 插入键值 |
| `Equal[K, V](m, n) bool` | 比较两个 map |
| `Clone[K, V](m) map[K]V` | 克隆 map |
| `Copy[K, V](src, dst) map[K]V` | 拷贝 src 到 dst |
| `DeleteIfKey[K, V](m, condition)` | 条件删除（按键） |
| `DeleteIfValue[K, V](m, condition)` | 条件删除（按值） |
| `ToString[K, V](m) string` | 转为排序的 key=value 字符串 |
| `Keys[K, V](m) []K` | 获取排序后的所有键 |
| `TypedGet[K, T](m, key) T` | 类型安全的获取 |

### 嵌套 Map 提取

| 函数/方法 | 说明 |
|---|---|
| `GetStringFromMapInterface(param, key, parents...) string` | 从嵌套 map 提取字符串 |
| `GetFloat64FromMapInterface(param, key, parents...) float64` | 从嵌套 map 提取浮点数 |
| `GetMapSliceFromMapInterface(param, key, parents...) []map[string]any` | 从嵌套 map 提取 map 切片 |
| `GetMapFromMapInterface(param, key, parents...) map[string]any` | 从嵌套 map 提取子 map |
| `GetStringSliceFromMapInterface(param, key, parents...) []string` | 从嵌套 map 提取字符串切片 |

### 类型化 Map

| 类型 | 主要方法 |
|---|---|
| `StringAny` | `Has()`, `Delete()`, `DeepCopy()`, `Init()`, `DeleteIfKey()`, `DeleteIfValue()`, `HasKeyAndValue()`, `IsSuperSet()` |
| `StringString` | `Get()`, `Set()`, `Keys()`, `Merge()`, `Has()`, `Delete()`, `DeepCopy()` |
| `StringInt` | `Get()`, `Set()`, `Keys()`, `AddKeys()`, `Has()`, `Delete()`, `DeepCopy()` |
| `StringBool` | `Get()`, `Set()`, `Keys()`, `Has()`, `Delete()`, `DeepCopy()` |

[← 返回包列表](../../README.md)
