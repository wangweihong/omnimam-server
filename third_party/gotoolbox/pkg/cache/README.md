# cache — 线程安全缓存

**功能**：提供带索引的线程安全内存缓存，支持深拷贝、索引查询。

| 函数/方法 | 说明 |
|---|---|
| `NewThreadSafeStore(indexers, indices)` | 创建线程安全缓存 |
| `ThreadSafeStore.Add(key, obj)` | 添加缓存项 |
| `ThreadSafeStore.Update(key, obj)` | 更新缓存项 |
| `ThreadSafeStore.Delete(key)` | 删除缓存项 |
| `ThreadSafeStore.Get(key)` | 获取缓存项 |
| `ThreadSafeStore.List()` | 列出所有缓存项 |
| `ThreadSafeStore.ListKeys()` | 列出所有键 |
| `ThreadSafeStore.GetByKey(key)` | 通过键获取 |
| `ThreadSafeStore.Index(indexName, obj)` | 添加索引 |
| `ThreadSafeStore.ByIndex(indexName, key)` | 通过索引查询 |
| `ThreadSafeStore.IndexKeys(indexName)` | 获取索引键列表 |

[← 返回包列表](../../README.md)
