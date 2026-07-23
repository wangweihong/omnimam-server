# 服务器存量素材批量导入方案

本文使用的 Asset、AssetVersion、AssetRepresentation、Blob 和 StorageBackend 分层语义，详见 [Asset、Representation 与 Blob 机制](asset-library-content-model.md)。

## 1. 目标

将服务器上已有的大量图片、文档、视频、音频等文件导入 OmniMAM 素材库，避免用户逐个通过浏览器上传，同时满足：

- 可处理数十万到数百万文件，单个大文件不占满内存。
- 导入过程可暂停、恢复、取消和重试，服务重启后可继续。
- 每个文件独立成功或失败，坏文件不阻塞整批任务。
- 保留文件来源、相对目录和校验结果，便于审计与重跑。
- 不绕过现有 `Asset -> AssetVersion -> AssetRepresentation -> Blob` 模型。
- 导入后继续使用现有 Representation 异步处理链生成缩略图、预览和播放格式。

## 2. 推荐结论

新增一个“服务端受控批量导入”能力。管理员先把待导入目录以只读方式挂载给专用 Import Worker，再创建导入作业。系统分为盘点、预检、传输、登记和派生处理五个阶段。

```text
只读源目录
  -> 扫描并生成清单
  -> 逐文件预检与 SHA256
  -> 流式写入受管 StorageBackend
  -> 事务登记 Asset/Version/original Representation
  -> 现有 Task Center 生成 thumbnail/preview/playback
```

不要让 API Server 递归扫描目录，也不要把任意服务器绝对路径提交给通用 HTTP API。扫描和大文件 I/O 都由独立 Worker 执行；API Server 只负责作业配置、权限、查询和控制。

## 3. 为什么不能直接“登记路径”

不建议把现有文件绝对路径直接写入 Blob，原因如下：

- 原文件可能被移动、覆盖或删除，AssetVersion 的不可变性无法保证。
- 路径可能逃逸允许目录，形成任意文件读取风险。
- 文件权限、符号链接和挂载变化会让素材状态不可预测。
- 以后迁移到 S3、MinIO 等 StorageBackend 时无法保持一致语义。
- 无法可靠判断文件在校验后、登记前是否被修改。

默认策略应为“复制后接管”：源文件保持只读，Worker 将内容流式复制到受管存储，计算并复核 SHA256，成功登记后再由管理员决定是否清理源目录。

可在后续增加“同一受管存储内接管”优化，但只能针对由系统管理员预先配置的可信根目录，并需要原子移动或等价的不可变保证。硬链接不作为默认方案，因为源端修改仍可能改变已登记内容。

## 4. 作业模型

### 4.1 Import Source

Import Source 是管理员预配置的可信导入源，不接受用户任意输入绝对路径。建议配置：

- `source_id`：稳定标识。
- `root_path`：Worker 本地可见的规范化根目录。
- `allowed_extensions` / `allowed_media_types`：允许范围。
- `follow_symlinks=false`：默认禁止跟随符号链接。
- `read_only=true`：部署层强制只读挂载。
- 可选默认 owner、Collection、Labels、Tags 和处理 profile。

所有待导入路径都以 `source_id + relative_path` 表达。Worker 对路径做 `clean`、求值和根目录边界校验，拒绝 `..`、设备文件、FIFO、socket、符号链接逃逸及不规则文件。

### 4.2 Import Job

一次 Import Job 固定以下信息：

- 导入源和相对起始目录。
- owner；不能从文件名或清单中越权指定其他用户。
- 扫描规则：递归、包含/排除 glob、隐藏文件策略、最大文件大小。
- 组织规则：是否按目录创建 Collection、公共 Labels/Tags。
- 冲突策略、传输策略和 Representation profile version。
- 创建时配置快照，运行中修改默认配置不影响本次作业。

建议状态：

```text
created -> scanning -> ready -> importing -> completed
                    \-> failed
importing <-> paused
created/scanning/ready/importing/paused -> cancelled
completed 可包含逐项失败，即 completed_with_errors
```

作业保存稳定 checkpoint 和计数：`discovered`、`eligible`、`skipped`、`running`、`succeeded`、`failed`、`bytes_total`、`bytes_completed`。不得把百万条文件清单塞进一个 JSON 字段或一个 Task 输入。

### 4.3 Import Item

每个普通文件对应一个 Import Item，保存至少：

- 规范化相对路径、文件名、大小、mtime 和发现时文件标识。
- 探测出的 MIME/media type、SHA256 和预检结果。
- 当前阶段、尝试次数、错误摘要和是否可重试。
- 最终 `asset_id`、`asset_version_id`、`blob_id` 和 original Representation ID。
- 稳定幂等键，建议由 `job_id + relative_path + discovered_size + discovered_mtime` 派生。

文件在传输前后必须复核大小、mtime；内容 SHA256 必须与写入受管存储的内容一致。源文件在处理中变化时，该项失败并标记为 `source_changed`，不能登记一个不确定版本。

## 5. 五阶段流程

### 5.1 扫描

Worker 使用流式目录遍历，分批写入 Import Item，例如每批 500 到 2000 条；通过稳定的规范化相对路径排序和 checkpoint 恢复。扫描阶段只收集轻量元数据，不读取整个文件内容。

扫描需要明确跳过：

- 不规则文件、损坏链接和越界符号链接。
- 临时文件、隐藏文件或排除规则命中的文件。
- 超过配置上限或零字节且类型不允许的文件。
- 无读取权限的文件。

扫描完成后给出预估文件数、总字节数和按媒体类型分布。建议默认先进入 `ready`，由管理员确认后开始实际传输；也可显式选择扫描后自动开始。

### 5.2 预检

逐文件流式计算 SHA256，并使用内容探测而非仅凭扩展名判断 MIME。文件名只用于展示，不能决定安全处理方式。

SHA256 用于两类判断：

1. 当前 owner 已有同内容素材时，按冲突策略跳过并返回已有 Asset。
2. 同一作业内重复内容时，只传输和登记一次，其余项记录为重复项；是否把重复路径作为额外 Collection 归属或标签，由组织规则决定。

预检不生成缩略图或视频转码，重媒体处理仍交给现有 Representation 流程。

### 5.3 传输

Worker 以固定大小缓冲区流式读取源文件并写入 StorageAdapter 的临时对象，严禁 `ReadAll`。写入成功后校验：

- 实际字节数等于扫描记录。
- 写入内容 SHA256 等于预检 SHA256。
- StorageAdapter 完成临时对象到最终对象的原子提交或等价提交。

传输失败时保留 Import Item 和可重试状态；临时对象由定时清理机制按 TTL 回收。不要让导入作业直接写 StorageBackend 表或拼接本地对象路径。

### 5.4 登记

每个文件独立使用短事务登记，避免一个坏文件回滚整批导入。事务语义应与普通上传完成一致：

1. 创建或命中 Blob。
2. 创建 Asset。
3. 创建不可变 AssetVersion，来源标记为 `external_import`。
4. 创建 `original` Representation。
5. 更新 Import Item 的结果引用。
6. 同事务写 `asset_version_representation_requested` outbox。

事务提交前发生错误时不得发布 Representation 请求。重复执行同一 Item 必须返回同一登记结果，不能创建重复 AssetVersion。

### 5.5 派生处理

登记成功即表示原始素材已安全进入素材库，不等待缩略图、文档预览或视频播放格式完成。现有 Task Center 按每个 AssetVersion 的幂等键创建 Representation build DAG，并由 SSE/查询展示处理状态。

批量导入和 Representation 生成应使用不同并发池或队列限额。否则大量视频转码会挤占目录扫描、文件复制和普通用户任务。

## 6. 目录与素材库组织

服务器目录不应变成素材库的物理目录模型。推荐映射如下：

- 每个普通文件创建一个 Asset。
- 原文件相对路径写入导入来源审计信息，而不是成为 Blob 绝对路径。
- 默认使用文件名去扩展名作为 `display_name`，保留完整文件名为 `original_name`。
- 可选择按一级目录或完整目录树创建 Collection；Collection 是逻辑分组，不影响存储对象键。
- 公共 Labels/Tags 可由作业配置附加，例如 `source=legacy-nas`、`import_batch=2026-07`。
- 同一内容出现在多个目录时，默认只创建一个 Asset，并可加入多个 Collection；需要保留多个独立业务身份时，必须显式选择“按路径保留独立 Asset”。

目录到 Collection 的映射需要限制最大深度为现有契约允许的 8 层。超过部分可折叠到第 8 层，并在报告中提示。

## 7. 冲突策略

建议提供以下作业级策略，并把默认值设为安全选项：

| 场景 | 默认策略 | 可选策略 |
| --- | --- | --- |
| 相同 SHA256 已存在 | 跳过并引用既有 Asset | 按路径保留独立 Asset |
| 同名但内容不同 | 创建不同 Asset，允许重名 | 按规则追加名称后缀 |
| 同一路径再次扫描且内容未变 | 幂等跳过 | 无 |
| 同一路径内容已变化 | 创建新导入项；默认创建新 Asset | 显式映射到已有 Asset 的新版本 |
| 不支持或无法探测的类型 | 作为 `other` 或跳过，由白名单决定 | 管理员修正规则后重试 |

不要自动把“同名文件”识别为同一 Asset 的新版本。文件名不是稳定业务身份，自动追加版本容易污染历史。只有提供明确映射清单或人工确认时，才向已有 Asset 追加 AssetVersion。

## 8. 并发、限流与容量

建议把各阶段并发分别配置，并从保守值开始：

- 扫描器：每个源 1 个，批量落库 1000 条左右。
- SHA256：HDD 每磁盘 1 到 2 个，SSD/NVMe 视吞吐提高；避免随机读拖垮源盘。
- 文件复制：默认每个源 2 到 4 个，总并发受源盘、目标盘和网络带宽限制。
- 登记事务：可高于复制并发，但需要限制数据库连接和 outbox 写入速率。
- 视频/文档派生：独立 Worker 并发，按 CPU、内存和 GPU 容量设置。

所有队列必须有界。按字节和文件数同时限流，因为十万个小文件与十个超大视频对系统的压力不同。建议提供全局暂停开关、磁盘剩余空间低水位保护和 API/普通上传保留容量。

容量规划至少计算：

```text
目标存储需求 = 原始文件总量 + 临时写入峰值 + thumbnail/preview/playback 预估 + 安全余量
```

采用复制接管时，在源目录清理前需要同时容纳源数据和素材库存储数据。

## 9. 恢复、重试与取消

- checkpoint 和 Item 状态必须持久化，API Server、Worker 或数据库短暂重启后可恢复。
- 自动重试只处理可重试错误，如临时 I/O、存储超时和数据库瞬断；权限拒绝、类型不支持、源文件变化等需要人工处理或重新扫描。
- 重试从安全阶段恢复：未完成临时对象重新传输；已完成 Blob 但未登记的项复用校验结果；已登记项按幂等键返回原结果。
- 暂停不启动新 Item，允许正在写入的 bounded Item 安全结束或在超时后中止。
- 取消停止新工作并清理未提交临时对象，不删除已成功登记的 Asset。
- “重跑失败项”创建新的尝试记录，但保持同一 Item 和幂等身份。

## 10. 可观测性与验收

作业页面或运维查询至少展示：

- 总文件数、已处理数、成功、跳过、失败和剩余字节。
- 当前吞吐量、估算剩余时间和各阶段排队量。
- 按稳定错误原因聚合的失败数，并可导出逐项报告。
- 源目录、规则快照、发起人、开始/结束时间和暂停/取消记录。
- 已登记但 Representation 尚未 ready 的数量，避免把“导入完成”和“派生完成”混为一谈。

上线前应使用小目录、十万小文件、超大视频、重复文件、源文件变化、无权限文件、磁盘写满以及 Worker/数据库重启等场景验收。最终应核对：成功 Item 数与 Asset/AssetVersion/original Representation 数一致，所有登记事务都有 outbox，失败重试不产生重复素材。

## 11. 分阶段落地

### 阶段 A：一次性迁移工具

先提供仅管理员使用的离线或 CLI 导入器，读取只读目录，按 100 个文件一批复用现有上传初始化、内容写入和完成语义。它解决近期迁移需求，但进度、恢复和审计能力有限；适合几千到几万文件，不适合作为长期百万级能力。

即使源文件与服务在同一台机器，也应由导入器直接调用后端内部受控能力或通过本机网络流式传输，不能绕过登记事务直接写数据库。

### 阶段 B：正式 Import Job

在 SSOT 中补齐 Import Source、Import Job、Import Item 的产品语义、权限、错误码、状态、表结构和受控接口，再实现独立 Import Worker、暂停/恢复/失败重跑及作业进度。

这是推荐的长期方案，适用于持续从 NAS、共享盘、迁移盘或历史目录导入。

### 阶段 C：增量同步

在全量导入稳定后，再增加定期扫描。以相对路径、大小、mtime 和 SHA256 识别新增或变化文件；默认只导入新增内容，不把源端删除自动同步成素材删除。文件系统 watcher 只能作为加速提示，定期全量校验仍是事实来源，避免丢事件。

## 12. 当前契约边界

已 release 的 Asset Library 契约可直接复用：

- 一次最多 100 个文件初始化上传会话。
- 单文件/分片上传、完整与分片 SHA256 校验。
- 当前 owner 范围去重。
- 上传完成事务创建 Asset、AssetVersion、Blob、original Representation 和 outbox。
- Representation 通过 Task Center 异步生成。
- 批量加入 Collection 和批量 Labels/Tags。

当前契约未定义 Import Source、Import Job、Import Item、服务器目录扫描权限、导入状态及导入事件。因此：

- 阶段 A 只能复用现有正式上传能力，不新增 server API、表、权限码、错误码或事件。
- 阶段 B/C 实现前必须先在 `omnimam-ssot` 完成 S1/S2 设计并 release，再更新本仓库 submodule。
- 不得直接修改本仓库 `ssot/` 目录来规避 release 流程。

## 13. 最终建议

如果当前目标是尽快导入一次历史库存，先做阶段 A，并用小批次验证媒体探测、去重、Collection 映射和 Representation 容量。

如果这是长期运营能力，直接规划阶段 B：只读 Import Source + 持久化 Job/Item + 独立 Worker + 受管复制接管。阶段 A 的扫描、校验和流式传输组件应按可迁移到阶段 B 的方式设计，避免变成一次性脚本债务。
