
## sequential — 顺序容器

**功能**：记录插入顺序的 List 和 Map，支持索引查询和大小限制。

| 函数/方法 | 说明 |
|---|---|
| `NewSequentialList(datas...) *List` | 创建顺序列表 |
| `NewLimitSequentialList(len, datas...) *List` | 创建限制大小的顺序列表 |
| `List.Get(index) interface{}` | 按索引获取 |
| `List.Has(value) bool` | 检查是否存在 |
| `List.Indices(value) []int` | 获取值的所有索引 |
| `List.Inject(value)` | 注入元素 |
| `List.DeepCopy() *List` | 深拷贝 |
| `NewSequentialMap() *Map` | 创建顺序 Map |
| `NewLimitSequentialMap(len) *Map` | 创建限制大小的顺序 Map |
| `Map.Get(key) interface{}` | 获取值 |
| `Map.Has(key) bool` | 检查键是否存在 |
| `Map.HasValue(value) bool` | 检查值是否存在 |
| `Map.Inject(key, value)` | 注入键值 |
| `Map.DeepCopy() *Map` | 深拷贝 |

[← 返回包列表](../../README.md)
