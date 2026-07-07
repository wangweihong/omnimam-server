# Backend 开发规则

- 使用 Go；所有后端代码必须在 `backend/` 目录下。

## 架构原则

- API endpoint 统一使用 `/api/v1` 前缀；`API Server` 必须 stateless，
- 图片、视频、音频、PDF 等 heavy asset 列表必须使用 thumbnail、placeholder 或 derived preview，不能直接渲染原始文件。
- 凡是 provider、remote client、auth mode、storage、queue、cache、scheduler、policy、strategy 等可替换边界，必须采用“消费方 interface + adapter 实现 + bootstrap 注入/registry 选择”的结构。业务层不得直接依赖具体实现，不得在 service 中硬编码 provider switch。不要滥用 interface，DTO、稳定内部 helper、无替换需求的实现不需要抽象。新增边界时同时提供 fake/mock/noop 测试替身，并说明后续如何扩展第二种实现。

## 响应与错误码

- 普通 JSON DTO 请求只要成功到达并完成处理，HTTP status 必须为 `200`；业务成功直接返回业务对象，不强制包 `{code,data}`。
- JSON DTO 业务失败必须在响应体包含 business `code`，并通过 `message`、`messages`、`detail`、`causes`、`data` 等表达错误；客户端必须按 `code` 判断业务结果，不能按 HTTP status 判断业务错误。
- 批量接口 HTTP `200` 不代表全部成功；必须读取 `total`、`success`、`fail`、`results` 或接口定义的批量结果字段逐项判断，部分失败仍由批量结果字段表达。
- `404` 只能用于接口路由不存在，例如 `NoRoute`；provider、model、asset、task 等资源不存在必须返回明确 business error code 和 message。
- `500` 只能用于服务器异常、panic、不可恢复内部错误，或无法形成标准 JSON DTO 的内部错误；Provider、Storage、external API gateway、local model service 等外部调用失败必须按认证、权限、资源不存在、超时、上游不可用、响应解析失败等类型映射错误码，不能统一吞成 `500`。
- 新增或调整错误码时，按 `internal/pkg/code/base.go` 添加 `@HTTP`、`@CN`、`@EN` 注释；修改后优先运行 `make gen` 生成代码和文档，缺少 `codegen` 时先查 `Makefile`、`scripts/make-rules/`，优先用 `make tools` 或 `install.codegen`。
- 普通 JSON DTO 响应优先通过 `core.WriteResponse` 等统一入口返回；文件下载、图片、HTML、redirect、流式响应、SSE 可例外，但错误处理仍尽量保持标准错误 DTO 或记录例外原因；其他特殊情况先询问用户方案。

## Controller 与 API 入参

- Go backend API request struct 字段校验必须优先用 `binding` tag 触发 `pkg/validate` 或现有 `pkg/validator`；枚举字段必须用 `oneof` 等 tag 指定合法值。
- 跨字段、条件判断或 `binding` 难表达的校验，使用 `github.com/wangweihong/gotoolbox/pkg/validation.Validator` 风格实现 `Validate()`。
- 仅当绑定后需要参数归一化、派生字段填充、列表拆分等 post-bind 处理时，才实现 `imachinery.PostBinder`；零值默认值、字符串清理等用 `imachinery.DefaultSetter` 或 `PostBinder`，尽量在 controller 层通过 `core.DecodeParameter` / `core.Run` 完成，不放到 service。
- 无外部依赖、无 database 依赖、无特殊业务依赖的字段有效性检测，应尽可能在 Controller 层完成；入参必须校验合法性，避免过长字符串直接入库。
- 所有查询列表接口都必须有入参，入参必须嵌入 `imachinery.BasicQueryParam`；store 层通过 `ToQuery` / `ToStore` 转成 SQL/GORM 查询并接收自定义 filter。

## DB 与资源结构体

- 全局唯一必须由数据库约束保证，不能只靠业务逻辑；连续多个数据库操作必须用事务保证原子性，如检查同名再创建。
- PostgreSQL 新增函数前优先查 `helper.go` 是否可复用；需要通用能力时优先补到 helper。
- 元数据非 PostgreSQL 数据表支持类型时，采用 `Extend` / `ExtendShadow` 保存。
- 所有持久化资源结构体应优先嵌入 `imachinery.ObjectMeta`；不嵌入必须说明原因，如非标准资源、只读投影、关联表、纯 join/edge 表、外部系统映射表。
- 所有存储数据库的元数据结构体必须实现 `TableName`、`BeforeCreate`、`AfterCreate`、`BeforeUpdate`、`AfterUpdate`；无逻辑时可空实现预留。
- 持久化资源字段、状态、类型、枚举、常量必须补充中文注释，说明业务含义、适用场景、约束来源、是否终态、主要流转来源或特殊存储行为，禁止逐字复述名称或字面行为。

## 注释规则

- 实现逻辑注释仅用于复杂业务逻辑、非显而易见算法、跨模块/跨层关键流转、易误解实现、非常规库行为；`Magic Number`、复杂正则、锁/并发、类型强制转换、外部依赖降级策略默认先判断是否需要说明设计意图和风险。
- 修复 bug 时允许并要求用注释解释修改原因、防回归点、兼容性或历史背景；禁止解释基础语法或复述代码字面意思。
- 注释必须优先解释设计意图、业务原因、约束来源、兼容性背景；避免冗余堆叠，单行注释放在代码上方，行尾注释只允许极短对齐注记，平均每 50 行不超过 3 条，复杂算法例外。
- 新增或修改 HTTP API、service/store/provider adapter/worker task interface 时，必须补充中文功能注释，说明用途、主要 input/output、关键 side effect 或 async behavior。
- `backend/apis/iapiserver/meta_*.go` 元数据结构体字段和 `backend/apis/iapiserver/request_*.go` 请求参数结构体字段必须补充中文功能注释，说明字段业务含义、适用场景、主要约束、枚举/默认值/权限或 `FeatureFlag` 影响；禁止只复述字段名。Public API 注释需说明 permission 或 `FeatureFlag` 影响，以及 endpoint 是否返回原始 asset content、metadata/thumbnail，或是否创建 async `Task`。
- Internal helper function 不要求长注释，但 exported interface method 和 controller endpoint 必须有清晰功能说明。

## 公共函数复用

- 新增通用函数或者异步函数、slice、sets、fields、waitgroup、http请求等功能前必须先查复用：优先读取 `third_party/gotoolbox/README*` 采集 `github.com/wangweihong/gotoolbox` 功能包列表以及对应包的README.md的函数列表；如果有功能类似则优先复用。
- 确认 gotoolbox、仓库 `/pkg`、`backend/pkg`、`backend/internal/pkg` 都无合适能力后，才允许新增本地 helper，并说明原因。
- 多个外部服务健康检测或连接测试必须考虑网络延迟，默认并发检测，整体/单项超时时间限制为 5 秒；特殊情况实现前说明原因。等待、重试、超时控制优先用 `github.com/wangweihong/gotoolbox/pkg/wait`，必要时结合 gotoolbox waitgroup 等并发辅助包。
