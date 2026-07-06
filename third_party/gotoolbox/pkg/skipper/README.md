# skipper — 路由跳过

**功能**：路由/方法跳过判定，支持前缀匹配。

| 函数/方法 | 说明 |
|---|---|
| `AllowPathPrefixSkipper(prefixes...) SkipperFunc` | 允许指定前缀跳过 |
| `AllowPathPrefixNoSkipper(prefixes...) SkipperFunc` | 非指定前缀跳过 |
| `Skip(method, skippers...) bool` | 判定是否跳过 |
[← 返回包列表](../../README.md)
