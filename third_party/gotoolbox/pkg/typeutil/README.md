
# typeutil — 类型转换工具

**功能**：指针/值互转、interface 类型转换、切片类型转换。

### 泛型函数

| 函数/方法 | 说明 |
|---|---|
| `GenericIndirectValue[T](p) T` | 泛型指针解引用（返回零值） |
| `GenericIndirectPrefined[T](p, prefine) T` | 指针解引用（带默认值） |
| `MustAtoi(d) int` | 字符串转 int（忽略错误） |
| `MustAtoi64(d) int64` | 字符串转 int64（忽略错误） |


### Interface 转换

| 函数/方法 | 说明 |
|---|---|
| `ConvertInterfaceToString(value) string` | interface 转字符串 |
| `InterfaceToInt(di) int` | interface 转 int |
| `InterfaceToString(di) string` | interface 转 string |
| `InterfaceToMapStringInterface(di) map[string]any` | interface 转 map |
| `SliceInterfaceToIntType(di...) []int` | interface 切片转 int 切片 |
| `SliceIntToInterfaceType(di...) []any` | int 切片转 interface 切片 |
| `SliceInterfaceToStringType(di...) []string` | interface 切片转 string 切片 |

### 指针/值互转

| 函数/方法 | 说明 |
|---|---|
| `String(v) *string` / `StringValue(v) string` | 字符串指针/值互转 |
| `StringSlice(src) []*string` / `StringValueSlice(src) []string` | 字符串切片指针/值互转 |
| `StringMap(src) map[string]*string` / `StringValueMap(src) map[string]string` | 字符串 map 指针/值互转 |
| `Bool(v) *bool` / `BoolValue(v) bool` | 布尔指针/值互转 |
| `BoolSlice(src) []*bool` / `BoolValueSlice(src) []bool` | 布尔切片指针/值互转 |
| `Int(v) *int` / `IntValue(v) int` | 整数指针/值互转 |
| `Int64(v) *int64` / `Int64Value(v) int64` | int64 指针/值互转 |
| `Float64(v) *float64` / `Float64Value(v) float64` | float64 指针/值互转 |
| `Time(v) *time.Time` / `TimeValue(v) time.Time` | 时间指针/值互转 |
| `Duration(v) *time.Duration` / `DurationValue(v) time.Duration` | 时间段指针/值互转 |

[← 返回包列表](../../README.md)
