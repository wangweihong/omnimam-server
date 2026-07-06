# mathutil — 数学工具

**功能**：数值运算、浮点比较、数字解析、取整、单位转换。

| 函数/方法 | 说明 |
|---|---|
| `Divide[T](a, b T) T` | 安全除法（除零返回 0） |
| `Add[T](a, b T) T` | 加法 |
| `Min[T](a, b T) T` / `Max[T](a, b T) T` | 最小值/最大值 |
| `FloatEqual(a, b, epsilon) bool` | 浮点相等比较（带精度） |
| `FloatBiggerThan(a, b) bool` | 浮点大于比较 |
| `FloatDivide(a, b) float64` | 浮点除法 |
| `FloatToString(a, digitNum) string` | 浮点转字符串 |
| `FloatRoundToInt(a) int` | 浮点四舍五入 |
| `IntCompare[T](a, b T) int` | 整数比较 |
| `IntMax[T](a, b T) T` / `IntMin[T](a, b T) T` | 整数最大/最小 |
| `ParseInt(size) (int, error)` | 字符串解析为 int |
| `ParseInt64(size) (int64, error)` | 字符串解析为 int64 |
| `ParseNum[T](a, d) (T, error)` | 泛型数字解析 |
| `MustParseNum[T](a, d) T` | 泛型数字解析（忽略错误） |
| `ParseNumExplicitType[T](d) (T, error)` | 显式类型数字解析 |
| `RoundUp(length, roundSize) int64` | 向上取整 |
| `RoundDown(length, roundSize) int64` | 向下取整 |
| `RoundUp4K(length) int64` / `RoundDown4K(length) int64` | 4K 对齐取整 |
| `ConvertMBToByte(sizeInMB) int64` | MB 转字节 |
| `ConvertByteToMB(sizeInByte) int64` | 字节转 MB |
| `ConvertGBToByte(sizeInGB) int64` | GB 转字节 |
| `ParseSizeByteToStr(size) string` | 字节大小转可读字符串 |
| `ParseSizeBitToStr(size) (string, error)` | 比特大小转可读字符串 |
| `ParseSizeInMb(size) (int64, error)` | 可读大小解析为 MB |
[← 返回包列表](../../README.md)
