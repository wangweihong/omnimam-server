# netutil — 网络工具

**功能**：IP 地址获取、CIDR 操作、ARP 查询、端口检测、网络接口管理。

| 函数/方法 | 说明 |
|---|---|
| `IsIpv4Addr(s) bool` | 判断是否 IPv4 地址 |
| `GetLocalIPs(wantIpv6, condition) ([]net.IP, error)` | 获取本地 IP 列表 |
| `GetLocalIPsV2(wantIpv6, skipCondition) ([]string, error)` | 获取本地 IP（字符串） |
| `GetIPAddrs(wantIpv6) ([]string, error)` | 获取 IP 地址 |
| `GetIPAddr(wantIpv6, ifacePrefix) (string, error)` | 获取单个 IP |
| `ValidateCIDR(cidr) (*net.IPNet, error)` | 验证 CIDR |
| `GenerateIPs(ipnet) []string` | 生成 CIDR 内所有 IP |
| `AddIPOffset(base, offset) net.IP` | IP 地址偏移 |
| `GetIndexedIP(subnet, index) (net.IP, error)` | 获取指定索引 IP |
| `IpBetween(from, to, test) bool` | 判断 IP 是否在范围内 |
| `CheckPortUsed(port) (bool, error)` | 检查端口是否被占用 |
| `IsValidPort(port) bool` | 验证端口合法性 |
| `GetMacIPFromARPCache(mac) (string, error)` | 从 ARP 缓存获取 MAC 对应 IP |
| `GetMacIPFromARPBroadcast(mac) (string, error)` | 通过 ARP 广播获取 MAC 对应 IP |
| `GetDefaultInterface() (*net.Interface, error)` | 获取默认网络接口 |
| `GetInterfaceAndIP() (string, net.IP, error)` | 获取接口和 IP |
[← 返回包列表](../../README.md)
