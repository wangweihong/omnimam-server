# sets — 集合工具

**功能**：泛型集合 + 类型化集合（String、Int、Object），支持交集、并集、差集、前缀匹配。

| 函数/方法 | 说明 |
|---|---|
| `NewGenericSet[T](items...) GenericSet[T]` | 创建泛型集合 |
| `NewString(items...) String` | 创建字符串集合 |
| `NewInt(items...) Int` | 创建整数集合 |
| `GenericSet[T].Insert(items...)` | 插入元素 |
| `GenericSet[T].Delete(items...)` | 删除元素 |
| `GenericSet[T].Has(item) bool` | 检查是否包含 |
| `GenericSet[T].HasAll(items...) bool` | 检查是否包含全部 |
| `GenericSet[T].InsertIf(condition, items...)` | 条件插入 |
| `GenericSet[T].DeleteIf(condition)` | 条件删除 |
| `GenericSet[T].Match(condition, item) bool` | 匹配任意元素 |
| `GenericSet[T].MatchAny(condition, items...) bool` | 匹配任意项 |
| `GenericSet[T].FindMatch(condition) GenericSet[T]` | 查找匹配子集 |
| `GenericSet[T].Union(other) GenericSet[T]` | 并集 |
| `GenericSet[T].Intersection(other) GenericSet[T]` | 交集 |
| `GenericSet[T].Difference(other) GenericSet[T]` | 差集 |
| `GenericSet[T].IsPrefixOf(str) bool` | 前缀匹配 |
| `String.Has()`, `Insert()`, `Delete()`, `List()`, `Len()` | 字符串集合方法 |
[← 返回包列表](../../README.md)
