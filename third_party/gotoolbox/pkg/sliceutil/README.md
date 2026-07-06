# sliceutil — 切片工具

**功能**：泛型切片操作 + 类型化切片操作 + 固定大小切片。

### 泛型函数

| 函数/方法 | 说明 |
|---|---|
| `Unique[T](slice) []T` | 去重 |
| `Push[T](slice, element) []T` | 向切片添加元素 |
| `Pop[T](slice) ([]T, T, bool)` | 移除并返回最后一个元素 |
| `Len[T](slice) int` | 返回切片长度 |
| `Delete[T](slice, element) []T` | 删除指定元素 |
| `DeleteIf[T](slice, condition) []T` | 条件删除 |
| `Filter[T](slice, fn) []T` | 条件过滤 |
| `Has[T](slice, element) bool` | 检查是否包含 |
| `ElemNum[T](slice, elem) int` | 元素计数 |
| `ElemNumIf[T](slice, fn) int` | 条件计数 |
| `Find[T](slice, fn) (T, bool)` | 查找元素 |

### 类型化函数

| 函数/方法 | 说明 |
|---|---|
| `Strings(ips) []string` | IP 转字符串切片 |
| `IsSliceOfStructs(si) bool` | 判断是否为结构体切片 |
| `ToInterfaceSlice(si) []interface{}` | 转为 interface 切片 |

### 固定大小切片

| 函数/方法 | 说明 |
|---|---|
| `NewFixedSlice[T](capacity) *FixedSlice[T]` | 创建固定大小切片 |
| `FixedSlice[T].Append(item)` | 追加元素（超过容量则移除最旧） |
| `FixedSlice[T].GetAll() []T` | 获取所有元素 |
[← 返回包列表](../../README.md)
