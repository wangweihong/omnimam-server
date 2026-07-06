# sortutil — 排序工具

**功能**：拼音排序、多从属排序、结构体字段排序。

| 函数/方法 | 说明 |
|---|---|
| `UTF82GBK(src) ([]byte, error)` | UTF8 转 GBK |
| `GBK2UTF8(src) (string, error)` | GBK 转 UTF8 |
| `ByPinyin` | 拼音排序器（按 GBK 编码排序） |
| `NewMasterBasedSorter(master, comparer) MasterBasedSorter` | 基于主序列排序从属序列 |
| `MasterBasedSorter.Sort(dps ...*[]any)` | 执行排序 |
| `FieldTagSort(si, tag, asc, defaultComparer, condition) error` | 按标签字段排序 |
| `StructSliceSort[T](slice, target, sortAsc)` | 泛型结构体切片排序 |
| `GetSortComparator[T](slice, target, sortAsc) func(int,int) bool` | 获取排序比较器 |
[← 返回包列表](../../README.md)
