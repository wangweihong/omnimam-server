
# filereceiver — 文件分片接收

**功能**：支持大文件分片接收和写入，适用于网络传输大文件场景。

| 函数/方法 | 说明 |
|---|---|
| `NewFileReceiver(dir, filename, totalSize, partitionSize) (FileReceiver, error)` | 创建文件接收器 |
| `FileReceiver.Receive(data, index) error` | 接收分片数据并写入指定偏移 |

[← 返回包列表](../../README.md)
