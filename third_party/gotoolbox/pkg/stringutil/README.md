# stringutil — 字符串工具

**功能**：字符串前缀/后缀判断、修剪、转换。

| 函数/方法 | 说明 |
|---|---|
| `BothEmptyOrNone(str1, str2) bool` | 两个字符串同时为空或非空 |
| `HasAnyPrefix(str, prefixes...) bool` | 包含任意前缀 |
| `HasAnySuffix(str, suffixes...) bool` | 包含任意后缀 |
| `ContainsAny(str, substrings...) bool` | 包含任意子串 |
| `PointerToString(p) string` | 指针转字符串 |
| `PrintUnescape(p)` | 打印不转义字符串 |
| `TrimAnyPrefix(str, prefixes...) string` | 移除任意前缀 |
| `TrimAnyPrefixAndReturn(str, prefixes...) ([]string, string)` | 移除前缀并返回被移除的前缀 |
| `TrimAnySuffix(str, suffixes...) string` | 移除任意后缀 |
| `UnderscoreToCamel(name) string` | 下划线转驼峰 |
[← 返回包列表](../../README.md)
