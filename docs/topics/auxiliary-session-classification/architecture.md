---
status: active
created: 2026-10-27
updated: 2026-10-27
---

# 辅助会话分类：架构与数据合同

## 1. 边界、输入与结论

落实 [requirements.md](requirements.md) 与四份 framework：[CLI](ux/cli.md)、
[菜单栏](ux/menubar.md)、[Settings](ux/settings.md)、[Widget](ux/widget.md)。
本文件规定拟实现合同，不声称产品已具有新字段/命令。产品源码保持只读。
用户 2026-10-27 明确允许只读检查已安装 claude-mem 源码，并进一步选择将
“只读插件会话映射元数据适配器”纳入产品架构；未授权本轮读取真实日志、数据库或凭据。

决定：Go 统一拥有分类与范围服务；用户覆盖/默认值存于 core，自动分类为可重建派生；
客户端只读取带版本的 DTO。维护来源适配器只读明确支持的映射元数据，不能用目录/模型猜用途。
不新增后台服务、网络请求、插件修改或另一套源日志。本主题不承担价格修复或原始事件去重重写。

输入版本在设计时核对：HEAD `cd1a98eb1c8546fe69f99f958bf89d97f1e854cf`；requirements blob
`3c14de201f3b0f87b16bdee4d4abde424b6b7136`，CLI blob 为 `6d1d8eed3e4b801a0534d33e72cb3e76e1cf19b9`。
菜单栏 `6589bae7e4a2bd11e3cc3731ec02a79815f0d912`、Settings `578976e5f1e00bbcc2cf753597cc798ed9934cd2`、
Widget `3b15e60bb77b3883dc3528e5dd5fd806657a2f25`。其他会话的状态勘误与评审修改保留，不纳入本轮改写。

## 2. 已核实的当前实现

| 源位置 | 当前事实及设计影响 |
| --- | --- |
| internal/session/session.go Metadata、meta、List | 无用途字段；List 按 client/session_id 与来源优先级选一条。不能通过 UI 隐藏替代统一分类 |
| internal/usage/usage.go rebuildSessions；usage_test.go TestScanCapturesClaudeSessionStartThroughToAttribution | 子代理来源可与主日志共用 session ID；不能假设每文件等于独立子会话 |
| internal/store/store.go Setting/SetSettings | 设置缺行与读取错误不同；提交后 secureFiles 错误仍可能已经落盘，不能所有 error 都回滚显示未保存 |
| internal/store/derived_generation.go | core 有 epoch/revision/dirty，session_index_generation 当前只有 epoch；新索引内容 revision 需要明确新增，不能假定已有 |
| internal/backup/backup.go Create/Restore | 默认复制 core；sessions 仅 include-sessions；恢复目标为空。新覆盖放 sessions 会在 purge 后丢失 |
| internal/store/store.go CurrentSchemaVersion | 本工作树为 core schema 30；其他未合并主题的 schema 不可视作当前能力 |
| internal/desktop/desktop.go sessionsSnapshot | 列表、total、periods 来自现有会话索引；新范围需独立投影，不能重解释 v1 旧 total |
| apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift | sessions 是 available/total 的聚合白名单；新 Widget 数据仍不能带身份或正文 |

## 3. 维护来源适配器与证据

### 3.1 源码核对结果（自包含记录）

检查的是安装包 claude-mem-plugin 15.2.1 的代码，不是用户数据。包中没有原始 TS，source map
只含源路径/mappings、不含 sourcesContent；通过固定 bundle 片段和映射核对下列事实：

- worker-service.cjs SHA-256 `aed0b798e04c40e2c3600e2e5976f4eb470b84c96bc1ea48fea9201157a556f4`。
- sqlite/SessionStore.js SHA-256 `4ed2bf28cfc01b37589e92f2ace8f243f25dbef0aa84df07d5d1b4140e1907c7`。
- SessionStore initializeSchema 建立 sdk_sessions，字段包括 `content_session_id TEXT NOT NULL`、
  `memory_session_id TEXT UNIQUE`、`platform_source TEXT NOT NULL DEFAULT 'claude'`、
  started_at/started_at_epoch，以及 user_prompt/project 等本适配器禁止读取的字段。
- 同一源码的 updateMemorySessionId 按内部行 ID 更新 memory_session_id，并同步 tool_uses 的关联。
  observations 表的 memory_session_id 外键指向 sdk_sessions.memory_session_id；它是记忆侧身份。
- worker bundle 的 startSession 为部分 provider 生成 `syntheticIdPrefix-contentSessionId-timestamp`
  形式的 memorySessionId。此值不能无条件当作 Claude 会话 ID，也不能把 platform_source 当 observer backend。
- hardened-options 的 Observer 路径设置无工具、无 MCP、无客户端 settingSources 的 SDK 选项；
  默认 observer cwd 来自插件目录。cwd 只能做诊断，不用于本分类规则。
- observer-audit 可能保存 tool_input，本适配器不读取该文件；user_prompt、observations、
  tool_uses 的内容也不属于许可查询面。当前 bundle 中没有 claude-mem-cpa 字符串，旧模型线索不提升为规则。

以上事实直接写在本文件，不将可清理的插件缓存路径作为必须永久存在的引用。
源码证实了可用映射合同，但**没有**证明本机实际启用 SDK observer、存在对应行或覆盖率；未读运行库。

### 3.2 支持范围与查询

新增 `internal/sessionorigin/adapters/claudemem`，首版只支持上述 15.2.1 格式的本地 SDK observer 映射。
识别安装版本与 schema 指纹；更高/未知版本默认不适配，需新的源码与 fixture 评审，不猜兼容。
插件位置由扩展定位提供候选并校验包版本；首版数据根只接受明确配置的绝对目录，不猜测默认根。
配置覆盖通过 AgentDeck 本机设置/CLI `session origin-source set claude-mem --data-dir <absolute>`，
不修改插件配置；该本机路径不进入 portable backup、输出或 Widget。禁用用 `session origin-source disable claude-mem`。
没有可确认根、多个冲突根或无权限时返回 unavailable，不扫描整个 HOME、不询问凭据。

查询仅打开已存在数据库：mode=ro、query_only，禁止插件初始化/migration/checkpoint/VACUUM、全库复制及
immutable=1 忽略活跃 WAL 的捷径。必须用隔离 WAL fixture 证明不会创建/修改源库及缺失 sidecar；
不能满足只读访问时适配器 unavailable。允许正常读取锁协调，不调整文件权限或启动插件进程。

允许列严格为 sdk_sessions 的 id、memory_session_id，以及格式校验所需的 schema_versions.version；
不 SELECT *，不读 content_session_id、platform_source、project、user_prompt 或其他表内容。
以已索引的 Claude canonical ID 为候选，每次最多 200 个 UUID，使用参数化 `IN` 查询 memory_session_id；
空候选不查。使用该版本提供的 memory_session_id 唯一索引；单次读超时 250 ms，整个轮次 2 s，
超限返回 coverage partial 并保留尚未处理候选为 unknown，不阻塞前台范围切换。
ID 格式限制为规范 UUID；非 UUID/合成 provider 前缀不参与此适配器，不能扩大解释为 Codex 会话。

规则 `claude-mem-sdk-map-v1`：已确认支持版本/来源根 + 精确 memory_session_id 映射 +
当前 Claude 日志的 canonical ID 一致 → 维护来源证据。只命中 content_session_id **不算**辅助，
因为那可能是用户工作。只匹配路径、模型、时间邻近、类似 ID 或 hash 前缀均不算。
来源根可信性是本地用户授权的信任边界，不声称能抵御用户伪造自己的数据库。

接收数据先暂存为 adapter evidence，持有版本、映射行 ID、输入 token 和成功读取时间；
不建立新 usage 事件、不导入 observer 报告的 token。元数据适配器不改变价格或重复计量结果。
新增原始消耗去重必须由既有事件身份合同另行证明，不能因映射命中就减费。

### 3.3 分类归约

规则输出 automatic=`work|auxiliary|unknown`，reason_code 与 rule_version；effective 另叠加用户覆盖。
首版维护适配器可产生 auxiliary；当前原生 parser 没有已核实的显式用户用途字段，普通来源保守 unknown，
仍在 Work 视图。不会宣称普通日志已被证明为 work。显式用户覆盖可将其设为 work。
任务子代理与主会话同 ID 时保持同一 canonical 会话；有独立 ID 但无可验证用途关系则 unknown，仍可见。
后续支持的显式用途/父子字段必须带真实格式 fixture 和规则版本，不在此凭字段名虚构客户端能力。

同 canonical 多来源先按现有身份规则去重，归约保留所有有效证据的冲突状态；不以低优先级副本
覆盖主源身份。精确维护映射与明确工作证据冲突、混合用途或循环关联 → unknown/conflicting_origin。
缺映射不证明 work；映射读取失败/版本未知 → unknown/origin_source_unavailable。
移除或禁用适配器使受影响自动证据失效，重分类为 unknown；用户覆盖不随之删除。
覆盖优先但不抹掉 automatic 的冲突/缺失原因。证据与会话源 generation 不匹配时只能 unknown，不能旧 auxiliary 隐藏新来源。

实现验收必须包含至少一种真实 SDK observer 格式的脱敏或合成忠实 fixture：匹配 memory ID 为辅助，
content ID-only、synthetic ID、跨 client、重复副本、用户任务子代理与冲突均不误隐藏。
源码核对不能替代支持版本的隔离端到端验收；无法得到可验证匹配则停止该适配器交付，不降低 AC1 为全 unknown。

## 4. 持久化、身份与迁移

新增 Go 包 `internal/sessionorigin`，store 只持有 SQL/事务；parser 与 adapter 产出证据，服务负责统一归约。
新增内容不落入 source logs；core 文件与 sidecar 沿用 0600、目录 0700。

| 存储 | 拟新增对象 | 权威/生命周期 |
| --- | --- | --- |
| core | origin_preferences：singleton、scope、revision | 用户持久默认；缺行且读取成功=work/revision0；读取错误=unconfirmed/null |
| core | origin_overrides：client/session_id、binding_digest、value、revision、updated_at | 用户覆盖；value 为 work/auxiliary 或 NULL；clear 置 NULL 并保留版本 tombstone，列表只列有效覆盖，防止删除重建后的 ABA |
| core | origin_mutations：request_id、payload_hash、result、revision、target key | 幂等回执；不含正文/路径；明确响应可按 request_id 查询 |
| core | session_origins：canonical key、automatic、reason、rule_version、input_token、evidence_revision | 可重建派生；不复制项目名或正文 |
| core | origin_evidence：adapter/rule、canonical key、source token、purpose、validated_version、observed_at | 可重建，失效时保守 unknown；本地源路径只通过现有 source registry 定位，不新增可导出路径列 |
| core | scope_data_revision：singleton、epoch、revision | 新范围读取依赖的事务时钟，提交变化才可见；不能复用会合并 dirty 写入的旧 revision 作为每次写入序号 |
| sessions | 元数据的 origin_input_token；session_index_generation 新增 revision | 同一源解析事务更新；revision 与现有 epoch 组合识别内容，不以 epoch 单独冒充每次更新版本 |
| 本机非导出配置 | origin-source 的明确数据根与禁用状态 | 只定位来源，不保存凭据；restore 后需重新确认 |

表/索引通过 core 与 session 各自 append-only migration 注册。schema 30 只是本基线；
具体下一个版本号在实施入口依据当时集成 migration 清单分配，不与 scan-performance 尚未合并的 schema 抢号。
禁止把语义变更伪装成无版本的 CREATE IF NOT EXISTS；旧二进制保持 schema-ahead 拒绝，不提供静默降级写。
rollback 使用升级前备份，不删除新增表让旧版继续写。

覆盖绑定：canonical key + 来源提供的稳定出生身份（例如可信根 ID 与固定起始时间）形成 binding_digest，
不含路径、正文或 inode。仅凭同名目录/同 ID 不自动跨设备匹配。重复副本、移动与重建不改变稳定出生证据。
出生身份不完整时允许本机显式覆盖，但标记 binding_limited；恢复到另一 state root 后未确认前为 unmatched/inactive。
恢复后的显式 set 对当前可见会话重新绑定；same value 但 binding 不同不算 no-change。
未知/冲突身份不能自动继承另一个实例的覆盖，旧覆盖保留供查看与逐条撤销。

## 5. 写入、幂等与前台读

默认与单条覆盖分别事务保存；不混入其他设置组。命令携带 expected_revision 与 request_id。
查询成功返回当前版本；交互 App 使用该版本保存；CLI 按下表读取相应目标，再用同样 CAS，不能绕过并发检查。

| 操作 | 目标解析与可见性 |
| --- | --- |
| 查询会话分类 / --set | 定位可见 canonical 会话并核对绑定；不可见沿用 session_not_found，不读取被 exclude 的源内容 |
| --clear | 仅以明确 client + session_id 定位 core 的用户覆盖记录或 tombstone；不要求 sessions 中存在/可见会话，不验证当前源绑定，也不打开源日志 |

因此 unmatched、会话已删除/索引已清除、被 exclude 的会话均可撤销其**已有用户覆盖**。
这不是查询或解除排除：只修改调用者指定的 core 覆盖键，不返回项目、正文、路径或隐含的会话存在性。
clear 必须明确 --client；省略时为输入错误，不通过探测源会话推断 client。
Settings 从覆盖列表取得键和 revision 后直接 clear；CLI 未传 expected_revision 时只读 core 覆盖版本，
不先调用可见会话查询。core 读取失败不能解释为没有覆盖。

事务内顺序：检查 request_id → 核对 payload hash → CAS revision → 修改值/绑定 → 写回执 →
标记 origin 及现有 derived generation dirty → commit。相同 request_id/相同负载返回原结果；
相同 ID 不同负载拒绝 invalid_request，不重做。新 ID 同值同绑定返回 no_change，revision 不增。
clear 的同值比较是“此键已无有效覆盖”，不拿旧 binding 与当前源比较：有效记录变为 value=NULL 的
tombstone，revision 加一，保留绑定和版本以防 ABA；已有 tombstone 返回 no_change、原 revision。
从未存在的键的基准 revision=0；在写事务中确认仍不存在且 expected_revision=0，返回 no_change/0，
只记录请求回执，不创建可见覆盖或源会话。CAS 总在新请求的 no-change 判定之前；不匹配即 origin_conflict。
同 request_id/同负载的重放优先返回原回执，不再次执行 CAS/clear；之后若别人重新设置了覆盖，重放
旧请求也不能清掉新值。回执描述该次操作，不保证仍是当前状态；需要当前状态时重新读取覆盖列表。
保留回执于本机 core，默认不导出；不按任意时间自动遗忘仍可能由客户端读回的未知结果。
后续有界清理须单独明确保留/确认合同，不能借本任务设置过期后自动重放。

错误：参数错误 exit2；版本冲突 `origin_conflict` exit1；会话分类查询/--set 的目标不可见沿用 session_not_found；
未确认身份 `origin_identity_unavailable` exit1；来源不可用是读取结果的 warning/partial，不改成伪成功分类。
提交前失败 `origin_write_failed`；连接在 commit 边界失联 `origin_write_unknown`，返回 request_id，
禁止自动重试新 ID。已有 SetSettings 的提交后权限加固错误必须归类 saved + warning，不能谎称未保存。
发布失败返回 saved/publication_pending，不撤销持久覆盖；Settings 状态机以回执与读回为准。

clear 成功回执包含 request_id、target={client,session_id}、outcome=saved|no_change、override=null、
revision、publication_pending。其 automatic/effective 固定为 null，classification_available=false、
classification_reason=not_queried，因为该操作不访问会话索引；不得将“覆盖已撤销”伪造成自动类别为 work/unknown。
Settings 显示撤销结果后，只有对仍可见的会话另行成功查询才显示其当前自动/有效分类；未匹配或不可见时
保持分类不可用并从有效覆盖列表移除该项。原始源未被删除，未知提交仍按 request_id 读回，未确认时不移除列表项。
应用回执时仍须尊重版本：旧回执不能删除界面已知的更高 revision 覆盖；重放回执后刷新覆盖列表，
以当前已确认记录决定是否移除，而不是把历史 saved 当成“现在仍无覆盖”。no_change 不触发新的派生发布工作。

新只读入口使用 OpenReadOnly 与 session read-only 连接，不迁移、不建库、不扫描。
显式写命令才通过现有 state lock 开启必要迁移及短事务；不在 UI 等待输入时持有锁。
Settings 请求生命周期由 App controller 持有，关闭窗口只释放视图，读回/回执状态不随 view 重建丢失。
默认 DTO 为 `{confirmed, value, revision, reason}`；confirmed=false 时 value=null。
值与确认状态一起传到所有消费者，禁止 `value || work`；只有成功确认缺配置才能初始化 Work。

## 6. 查询范围、跨数据库一致性与发布

新增只读 Selection：session_scope、client、period/from/to、publication_id。
work=work+unknown，auxiliary=auxiliary，all=并集；缺失关联归 unknown，不创建假会话。
先应用原有 exclusions 和 canonical 去重，再 scope，再排序/分页/聚合。
会话数可相加；项目用集合并集；均值/区间并集/比例从所选集合重新算。usage 总事件与金额始终全量。

core 分类与 sessions 索引不假定跨 WAL 数据库原子提交。定义读向量
`(core_epoch, scope_data_revision, session_epoch, session_revision)`，另加规则版本和默认/覆盖版本。
scope_data_revision 在 core 的新增 triggers 中覆盖分类/覆盖/偏好，以及被范围信号查询读取的
usage_events、usage_sessions、usage_work_signals 和其查询依赖；按实际最终 SQL 形成依赖清单，
每个依赖的 INSERT/UPDATE/DELETE（含直接 SQL）都须推进时钟。旧 dirty 位不能替代这一保障。
session revision 的 triggers 覆盖 session_metadata、session_sources、session_exclusions 与 FTS 输入；
递增在同一事务内提交，回滚不产生可见新版本。额外写放大纳入性能验收，不跳过交易计时。
origin_input_token 是共享归约器对 canonical key、当前有效来源身份/代次、用途证据和 parser/rule 版本
排序后编码的 SHA-256；两端使用同一规范编码，不从路径名或未确认内容猜测 token。
解析事务先提交带 input_token 的 sessions 元数据，分类事务再提交对应 token 的证据；
中间窗口 token 不一致时该会话 unknown/partial，绝不使用过期 auxiliary 隐藏它。
usage-only 来源可有分类证据，但不能因此生成不存在的可见文本索引记录。

组装器在各库只读事务取一致快照并捕获向量，完成全部所选数据后用新只读连接核对当前向量。
期间变化则放弃候选、最多重试 2 次；仍变化返回已有完整 published snapshot + stale，若无旧结果则 unavailable。
不得把不同读向量的列表、total、信号拼到一张卡。最后核对后发生的新写入只让已完成快照变旧，
不破坏该快照内部一致性；后续 dirty 触发下一轮。发布拒绝用较旧向量覆盖同 epoch 的较新结果；
epoch 变化（恢复/重建）先使旧 identity 失效，再构建新结果，不作跨 epoch 数值大小比较。

不可用分类元数据与整库不可读不同：前者 unknown，后者 affected section unavailable。
scope 切换只查已提交分区/缓存，不读外部元数据库、不重读日志、不新增扫描。
适配器在现有 scan/rebuild 协调阶段按候选 ID 批次刷新；CLI 原有 usage stats 默认扫描、--no-scan
和 usage signals 始终先尝试扫描的合同保留，失败仍 scan_incomplete/partial。
重分类触发受影响 canonical 分区与服务投影 dirty，不因一次覆盖强制全量重建。
必须在与 scan-performance 合并后的最终内容下满足其预算；本设计不自设豁免。

## 7. CLI 与桌面协议

CLI framework 的 session list/search/interactive、usage signals/stats 接受 --session-scope；
文本默认 work、JSON 默认 all；--all 仍仅取消分页。session show 按精确 ID 不加 scope 过滤。
`session classify <id> --client <client>` 查询；`--set work|auxiliary`、`--clear` 互斥。
其中 --clear 使用 §5 的 core 覆盖记录解析例外，不能复用查询/--set 的可见会话前置条件。
增加 `--expected-revision`、`--request-id` 供并发/重试；省略时 CLI 生成请求 ID 并执行读-CAS，不能盲写。
读回命令 `session classify --request <request-id>` 只查本机回执；没有确认结果返回 unknown，不返回 saved。
默认设置入口 `session scope get` / `session scope set work|auxiliary|all`，使用同一 CAS/回执机制。
覆盖枚举 `session classify list --page N --limit N`，每页默认20、最大200，排序 client/session_id。
只返回有效用户覆盖，包括 unmatched；每项带 client/session_id、value、binding_status、revision，
不从被排除来源补项目/正文。tombstone 不出现在有效列表，但内部版本读取仍能取得其 revision。
精确会话 ID 与保留子命令冲突时使用 `session classify --id <id> --client ...`，不重新解释合法身份。

旧 JSON data 的数组/对象形状不变。新顶层可加字段 `session_selection` 携带 requested/effective scope、
source_default（text_default/json_compat/explicit）、availability、unknown_count 和 read_vector/publication_id。
旧 data 数组元素可加 `origin` 对象（automatic/effective/reason_code/rule_version/override/revision）；
不把数组包成对象。搜索 pagination 的 total 是匹配文档数，另有 selected_session_count，不能混用。
未传 scope 的 JSON 旧集合及 envelope warnings/partial、UTC 时间、decimal 金额合同保持。

桌面 wire v1 旧 sessions.total/items/periods/work_signals 不变，新加 `session_scope_view`：
`available, selection, publication_id, generated_at, origin_coverage, counts, recent, work_signals`。
selection 中 scope/client/period 必填且同代；counts 含 sessions/projects/known_duration_ms/missing_duration_count。
recent 仅旧安全字段加 origin，禁止完整路径；新 App 使用新 view，旧 App 忽略它，不给旧 total 偷换语义。
新 view 缺失→分类范围 unavailable，不能把旧 all total 当 Work；独立 usage 仍可读。

Settings 通过 Go JSON 命令读/写，Swift 不直连 SQLite；origin-source 管理仅 CLI 首版入口，
Settings 显示适配器状态/安全原因与只读查询提示，不展示/编辑本机绝对路径。
此状态字段需 final-surface 明确加入说明区；不在 framework 已通过的截图中假称它已实现。

App Group 新增 `session_scope_summary`，只含 available、scope、count、unknown_count、client/period、
publication identity/time、安全 reason；绝不含 ID、project、model、父子关系、override 列表或源路径。
既有 sessions.total 不改名。Magnitude small/large 消费各自窗口的 summary，中号不新增 count。
旧快照无新字段仅会话行 unavailable；整份读取失败沿用 typed failure，优先于 local pending/stale。
保存新默认不修改旧快照标签；新 scope/count/time 一起发布。Widget 不读真实默认或探测宿主进程。

## 8. 升级、purge 与备份恢复

自动派生、缓存、producer 定位状态与 mutation receipts 不作可移植事实。
backup.Create 在其私有临时 core snapshot 上删除这些新派生表的行及本机 source path 配置；
保留 origin_preferences 与 overrides（不含路径/正文），再计算 archive digest。原 live core 不受影响。
不改变加密、凭据、空目标恢复合同；默认 backup 不额外包含 sessions，include-sessions 保持独立选择。

restore 先验证新增表/schema/枚举/整数边界、重复请求/身份冲突；迁移完成后 mint 新 epoch，
丢弃自动分类与回执，覆盖标为 unmatched，待当前源出生身份与 binding_digest 精确重建后再激活。
缺稳定出生证据则保持未应用，用户显式 set 才绑定当前会话。绝不按路径或同一个 ID 自动误套。
purge-index 仅删除派生 sessions 与相应 checkpoint；core 用户默认/覆盖仍在；重扫后按上述绑定恢复。
重建/升级失败保留旧可读 projection 并标 stale/partial；不能混用新 schema 与旧 codec 输出。
清除源 adapter 配置不删除原始事件或覆盖，恢复流程不自动访问外部插件库。

## 9. 验证与最终 UX 交接

| 准则 | 可观察证据 |
| --- | --- |
| origin-adapter | 支持版本 schema fixture、真实 SDK ID 与 synthetic/content ID 反例；只读 WAL/权限/超时/版本漂移；查询白名单无正文列 |
| classification | 同 ID 跨 client、共 ID 子代理、多副本、混合/冲突、迟到元数据、source rewrite/move；全量与增量结果一致 |
| mutation | CAS 冲突、幂等/no-change、commit 前后故障、unknown 回执读回、Settings 关闭重开、CLI 并发 |
| selection | text/JSON 默认与显式三范围、分页顺序、非可加指标、unknown/no-store 分离、费用/事件身份不变 |
| publication | 两库交错写入、token mismatch、向量重试、迟到响应、epoch 切换；禁止旧辅助分类误隐藏 |
| compatibility | 旧 JSON 数组和 wire v1 不变，新消费者缺字段安全降级，App Group 字段白名单与三个 Widget 尺寸 |
| durability | schema-ahead、backup/include-sessions、purge-index、恢复绑定与未匹配覆盖；没有源数据修改 |
| performance | 适配器有界查询、范围切换零额外采集；按最终 scan-performance 矩阵验证实际分区/投影成本 |

L0 验文档；L2 覆盖规则/查询/协议；持久化、迁移、并发、恢复和跨进程发布走 L3，Go 测试使用项目 wrapper。
覆盖撤销的 Settings/CLI 联合验收必须包含以下同一目标链：

| 场景 | 必须结果 |
| --- | --- |
| core 有 codex/archived-1、auxiliary、revision4、unmatched；sessions 无该会话 | list 返回覆盖；clear expected4/requestA 成功写 tombstone5；automatic/effective 为 null、不查询源，列表随后移除该项 |
| 相同覆盖对应已被 exclude 的会话 | 同样能 clear；不得解除排除、展示会话内容或为其生成可见索引 |
| tombstone5 上新 requestB/expected5 再 clear | no_change/revision5；不增版本、不制造会话 |
| 从未存在的键上 expected0 clear | no_change/revision0；core 读取错误则失败，不能冒充此结果 |
| 另一操作在读取4后把记录改为5 | 原 expected4 请求 origin_conflict；不能删除新覆盖，即使当前已是 tombstone 也不跳过 CAS |
| requestA 成功后别人重新 set 为 revision6，再重放 requestA | 返回原操作回执 revision5，不修改 revision6；用新 request/expected6 才能撤销新覆盖 |
| clear 提交边界响应丢失 | 显示结果未知；按原 request_id 读回 saved/no_change 后再更新列表，不能用 session_not_found 当作撤销成功 |

真实 SDK 维护来源的隔离验收必须与源码/版本对应；本轮不拿合成 UI、源码或未访问的真实库冒充该证据。

四份 UX 必须做 final-surface reconciliation：CLI 补回执/来源管理命令与错误；Settings 补适配器状态、
并发版本和未匹配绑定原因；菜单栏落实同代 selection/默认确认/partial；Widget 核对 summary 与 codec。
本架构通过后才可移交这些最终核对及任务分解；不创建实现任务、提交、安装或发布。
