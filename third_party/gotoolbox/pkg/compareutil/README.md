# compareutil — 比较工具

**功能**：提供泛型和反射比较函数，支持结构体字段比较。

| 函数/方法 | 说明 |
|---|---|
| `EqualCompare[T cmp.Ordered](a, b T) int` | 泛型相等比较（返回 -1/0/1） |
| `Compare[T cmp.Ordered](a, b T, asc bool) bool` | 泛型比较（升序/降序） |
| `StructReflectValueCompare(v1, v2, fieldName) int` | 反射结构体字段比较 |
| `ReflectCompare(fi, fj reflect.Value) int` | 反射值比较 |
| `CompareInterface(a, b any, ascending bool) int` | 接口值比较 |
| `IsStructReflectValueCanCompare(v1, v2, field) (bool)` | 检查结构体字段是否可比较 |
[← 返回包列表](../../README.md)
