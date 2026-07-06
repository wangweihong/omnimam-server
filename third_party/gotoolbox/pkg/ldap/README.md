# ldap — LDAP 客户端

**功能**：LDAP 目录服务操作，支持用户搜索、认证、同步。

| 函数/方法 | 说明 |
|---|---|
| `NewLDAPClient(cfg) *LDAPClient` | 创建 LDAP 客户端 |
| `LDAPClient.Search(req) (*SearchResult, error)` | 搜索条目 |
| `LDAPClient.Authenticate(username, password) error` | 认证 |
| `LDAPClient.Add(req) error` | 添加条目 |
| `LDAPClient.Modify(req) error` | 修改条目 |
| `LDAPClient.Delete(req) error` | 删除条目 |

[← 返回包列表](../../README.md)
