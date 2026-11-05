---
status: active
created: 2026-10-26
updated: 2026-11-05
---

# 辅助会话识别与工作视图隔离：实现分解

工作区：`agent-deck.auxiliary-session-classification`；分支：`feature/auxiliary-session-classification`。
来源：`ad-auxiliary-session-classification`。本文件是待独立评审的实现计划，不是开发、提交或发布授权。
仅隔离后台维护，用户任务子代理保留在 Work；unknown 留在 Work，所有用量和费用完整保留。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| ux/menubar.md | [x] | [x] |
| ux/settings.md | [x] | [x] |
| ux/widget.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

2026-11-03 集成修订：scan-performance 已合入 `main` `6fff208`，并在 Beads 任务
`ad-auxiliary-session-classification-origin-adapter-reducer` 登记 `codex-otel-background` 分类接口请求。
据此修订 requirements（第二维护来源、记忆关联非目标、基线刷新前提）、architecture（§3.4 内建来源，
§4/§6 复用 serving generation 与分区失效）、CLI 与 Settings（内建来源的命令与状态行）及本文件任务 1/2/3/9。
上述五份的 Review 置回待评审，按 requirements → architecture → ux/cli → ux/settings → tasks 顺序复审；
requirements 已于 Round 3 PASS。

2026-11-04 修复（ASC-ARCH-R3-F1…F5、ASC-CLI-R4-F1/F2、ASC-SET-R5-F1、ASC-TASKS-R3-F1/F2）：规则覆盖支持表全部两条
（exec、TUI）；新增只读 store 接口 `TelemetrySessionOriginMarkers` 与 `TelemetryBackgroundSessionCount`（任务 1），
不改 scan-performance 已交付的 `TelemetrySessionSourceKinds`；分类 reason_code 为封闭枚举，Codex 来源复用
`maintenance_origin`、由 rule_version 区分；新增只读 `session origin-source status`（架构 §7）与 core
`origin_adapter_runs` 摘要；Settings 内建行定义五种状态并加三个原型场景。菜单栏与 Widget 的 UX 文档无需修改：
菜单栏已映射 maintenance_origin，其实现须按封闭枚举映射（含 `origin_metadata_missing`）并对未知码通用回退，列入任务 5；
Widget 只消费聚合与安全 availability reason，不受影响。两份原 PASS 保留。

修订前的历史输入判定（保留）：SUFFICIENT——需求 Round 2、架构 Round 2、CLI Round 3、菜单栏 Round 4、
Settings Round 4、Widget Round 2 均有适用 PASS。四份 UX 为最终表面，不沿用 framework 勾选冒充。
评审记录分别位于 reviews/requirements.md、reviews/architecture.md、reviews/ux-cli.md、
reviews/ux-menubar.md、reviews/ux-settings.md、reviews/ux-widget.md；历史失败和门禁勘误保留。
Widget 最终表面已提交于 `6818c682f96eaddcdab227ee96a0609bebbda4b9`；
该提交包含输入表所列 Widget blob。本计划的分解评审与实现验收仍独立进行。

## 分解输入身份

历史批准列为 Round 2 分解所读内容；当前列为 2026-11-04 工作树内容（未提交）。

输入判定 SUFFICIENT（2026-11-04 复评同步）：requirements Round 3、architecture Round 5、ux/cli Round 6、Settings Round 6、
菜单栏 Round 4、Widget Round 2 均对下表当前 blob 有适用 PASS。ux/cli.md 的 Round 5 PASS 绑定旧 blob `7b2d214…`，其第 211 行把
单项读取失败写成 available=false，与架构 §7 冲突；已按 ASC-TASKS-R5-F1 改为引用 §7 的 available/partial 规则，并于 Round 6 复评 PASS。
影响核对（更正）：§7 的 available/partial 收紧需要修改 CLI 文本（已改）；Settings 五态表本就按 §7 把“开关未知”定为可用，无需修改。
开发入口如有实质变化，先做影响核对，不能自动沿用批准。

| 文档 | 历史批准 blob | 当前 blob | 状态 |
| --- | --- | --- | --- |
| [requirements.md](requirements.md) | `3c14de201f3b0f87b16bdee4d4abde424b6b7136` | `50da36642b7db63fe8286893141a42b464a5559c` | Round 3 PASS |
| [architecture.md](architecture.md) | `5b5a3b5694827900a9380e3879058a6baca4919a` | `416d63c922b9ea445a53a9774388efa13776af1c` | Round 5 PASS |
| [ux/cli.md](ux/cli.md) | `e4651cca9dbdc40348a0624134778a327539eb3e` | `549477b1b3e47aae80b26756a27bde6cd44c1360` | Round 6 PASS |
| [ux/menubar.md](ux/menubar.md) | `770cb769987da4232789f629e079eec8a4aa8c98` | `770cb769987da4232789f629e079eec8a4aa8c98` | 未变，Round 4 PASS |
| [ux/settings.md](ux/settings.md) | `090ce4e2151722f3c89c08714e2501cc8eee41ce` | `bb96bb3d65d8ce625bf7250ee4e741f2286a9e03` | Round 6 PASS |
| [ux/widget.md](ux/widget.md) | `0e764dbee6e92d2a878508ac7151d54901ff06ca` | `0e764dbee6e92d2a878508ac7151d54901ff06ca` | 未变，Round 2 PASS |

完整文档集为上述六份输入加本 tasks.md，共七份；无 n/a 表面或未编写占位。
本文件独立评审负责文档集完整性、最终 UX/架构一致性、任务覆盖和依赖可执行性。
文档 PASS、CEv1、Beads 协调与 Git 交付分别记录，不将前置文档的历史门禁状态镜像到本任务。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `origin-store` | [x] | [x] |
| 2. `origin-adapter-reducer` | [x] | [x] |
| 3. `scope-projections` | [ ] | [ ] |
| 4. `cli-contract` | [ ] | [ ] |
| 5. `desktop-menubar` | [ ] | [ ] |
| 6. `settings-control` | [ ] | [ ] |
| 7. `widget-summary` | [ ] | [ ] |
| 8. `durability` | [ ] | [ ] |
| 9. `integrated-acceptance` | [ ] | [ ] |

顺序：1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 可串行执行；最低依赖见各任务。
共享文件变更须顺序协调；此处依赖图不授权并行代理、分支创建或跨工作树合并。
实现任务已按 Round 2 PASS 进入 Beads（各一载体、open）；认领开发须在本文件当前版本评审通过及适用门禁/检查点满足后进行，
不按阶段建重复项。
每个任务完成后独立评审并解决其证据边界；后续集成不能替代前序任务的必要检查。

### 1. origin-store

- 依赖：本计划批准；架构 §4–5。范围：`internal/store/store.go`、`internal/store/derived_generation.go`、
  core append-only migrations 与 store 测试；新增 `internal/sessionorigin/` 的类型、存储接口及写入服务。
  新文件按包职责命名，实施时按当时 main 的 migration 清单分配号（main 已为 33），不固定沿用基线 schema 30。
  基线：分支已于 2026-11-03 经授权合入 `main` `660c815`（合并提交 `3ec8709`），包含 `6fff208`。
- 实现：preferences、overrides/tombstone、mutations、origins/evidence；不新增 scope_data_revision，
  为 preferences/overrides/origins/evidence 与 telemetry_sessions 安装 serving revision trigger（mutations 不装）；
  confirmed/value/revision/reason 默认 DTO；同 ID/同负载先返回回执，再 CAS，再 no-change；
  value 与 binding 一起比较。只读路径不建库/迁移；显式写路径使用既有锁与短事务。
  本任务提供接收已验证 binding 的存储接口，可见目标解析与来源绑定由任务 2 接入。
  另含 core `origin_adapter_runs`（架构 §4）及 `internal/store/telemetry.go` 新增的两个只读接口：
  `TelemetrySessionOriginMarkers(ctx, client, ids)` 返回 source_kind/support_entry/settled_at（200 一批、空候选不查、
  不建表不写行）与 `TelemetryBackgroundSessionCount(ctx)`；不修改既有 `TelemetrySessionSourceKinds`。
- 验收：无配置成功读才 Work/0；读取错误为 unconfirmed/null；新请求 stale revision 即使同值也冲突；
  archived-1/expected4 clear→tombstone5、never-present expected0→no_change/0、旧请求不能清除后来的 revision6；
  request ID 负载冲突拒绝。提交前失败、提交后权限警告、commit 边界未知分别返回；回执和数据同事务。
  新读取接口：0/1/200/201 个候选的批次边界、无标记与跨 client 同 ID 不返回、只读连接下可执行；计数只计
  `codex-otel-background` 行；既有 `TelemetrySessionSourceKinds` 的测试与行为不变。
  clear 不打开 sessions/源日志，automatic/effective=null、not_queried；回执不自动过期。
- 验证 L3：store/sessionorigin 定向及迁移/事务故障注入；并发 CAS/race；权限 0600/0700；
  schema-ahead 拒绝、回滚不推进可见 revision；Go core L2 全量与相关 vet。所有数据库为隔离 fixture。

### 2. origin-adapter-reducer

- 依赖：1。范围：新增 `internal/sessionorigin/` adapter/reducer/binding，`internal/session/session.go`、
  `internal/usage/usage.go` 的现有采集接点及相关 parser/fixture 测试；本地 source registry 接点。
- 实现：内建 `codex-otel-background-v1` 规则（架构 §3.4）经任务 1 的 `store.TelemetrySessionOriginMarkers` 批量读取，
  按 support_entry 白名单（`codex_exec/0.165.0/codex_exec/sse`、`codex-app-server/0.165.0/codex-tui/sse`）产生
  auxiliary/`maintenance_origin` 证据，support_entry+settled_at 作 source token；白名单外为 unknown/`origin_rule_unsupported`；
  标记撤销即证据失效；不读 Codex 文件、memories 状态库或接收器配置。reason_code 按架构 §3.3 封闭枚举产出。
  claude-mem 每次协调阶段结束时与 origin_evidence 同事务写 origin_adapter_runs 摘要。
  只读、显式配置、版本/schema 白名单的 claude-mem 映射适配器；仅必要 ID 元数据，
  精确 memory_session_id 与 Claude canonical ID 匹配，绝不用 content_session_id、路径、模型或前缀推断。
  在现有 scan/rebuild 阶段有界批次读取，UI 切范围不访问外部库。自动结果统一归约、叠加用户覆盖，
  stable birth binding 与共享 origin_input_token 编码固定；证据撤销/禁用/漂移使自动结果 unknown，覆盖保留。
- 验收：OTel 来源用 scan-performance 的隔离 ownership fixture：两条支持条目各自 settle 后的带标记会话
  （exec 的 Memories 整理、TUI 的标题生成）均为 auxiliary/maintenance_origin、rule_version=codex-otel-background-v1；
  晚到 rollout 撤销后不再 auxiliary；支持表外条目 unknown/origin_rule_unsupported；跨 client 同 ID 不命中；
  OTel 贡献与金额不变。origin_adapter_runs 的 ok/partial/unavailable/unsupported 与证据同事务、失败回滚不留摘要。
  至少一种与受支持 SDK 版本对应的脱敏或合成忠实 observer fixture，通过隔离端到端得到 auxiliary；
  content-only/synthetic/跨 client/伪目录/任务子代理不能误隐藏。同 canonical 多副本、混合用途、冲突、
  父关系缺失/环、source append/rewrite/move、迟到元数据及 token mismatch 均按架构处理。
  普通来源无已证实用途字段时 unknown，不虚构 work；增量与全量归约一致。
- 验证 L3：query allowlist 不含正文列；只读 WAL、权限拒绝、超时/锁、版本漂移、关闭/重启适配器；
  源文件及 usage 事件身份/token/金额前后不变。无法建立可验证真实格式匹配则阻止适配器交付，
  不以全 unknown 通过 AC1。真实私人数据读取不由本计划授权，优先受支持版本的隔离 fixture。

### 3. scope-projections

- 依赖：1、2。范围：`internal/session/` 查询和索引迁移、`internal/usage/` 统计/信号、
  `internal/store/derived_generation.go` 与新增 sessionorigin selection/publication 服务及测试。
- 实现：exclusions/canonical 去重 → scope → 排序/分页/聚合；work 包含 unknown，auxiliary 为确定辅助。
  复用 main 的 ServingGeneration 向量与读者，不另建时钟；对照最终 SQL 的全部读取依赖（含直接 SQL I/U/D、
  FTS 输入）与既有+新增 trigger 形成可审计依赖清单；不以 dirty 位代替 revision。重分类经
  usage_derived_invalidations(kind='session') 只 dirty 受影响分区，范围为分区 payload 内的 scoped membership；
  session_scope_view 随 desktop serving cache 同代发布。双库只读向量核对，最多重试 2 次，
  失败回旧完整快照+stale 或 unavailable；同 epoch 拒绝迟到结果，恢复 epoch 不作大小比较。
- 验收：token 不一致不能沿用旧 auxiliary；usage-only 不制造文本会话。列表/总量/信号同代；
  work+aux=all 会话数，项目并集、时长区间/平均值按独立 oracle；跨天/DST、并发与缺失时间。
  classification 缺元数据与整库不可读区分；全部 usage/token/费用/未知价语义不变。
  覆盖只 dirty 受影响分区；切换零额外采集、不新增扫描、不强制全量重建。
- 验证 L3：两库交错事务、直接 SQL 依赖时钟、回滚、token 过渡窗口、重试上限、迟到发布与 epoch 切换；
  全量/增量 oracle、分页与非可加聚合回归；记录触发器写放大，最终预算由任务 9 在集成内容核验。

### 4. cli-contract

- 依赖：1–3。范围：`cmd/agentdeck/` session/usage 命令、文本 renderer、browser/viewer 与对应 command/PTY 测试；
  具体新命令文件可新增，复用既有 envelope/错误协议，不建立第二分类实现。
- 实现：list/search/interactive、usage stats/signals 的 --session-scope；classify 查询/set/clear/list/--request，
  scope get/set、origin-source set/disable/status；--id 消解保留字；expected_revision/request_id 与读-CAS。
  `origin-source status`（`session.origin_source.status`）按架构 §7 字段白名单与来源状态 reason 枚举，
  只读 core、本机来源配置与 `telemetry.LoadConfig` 的 enabled；内建来源 set/disable 为 exit2 且不改状态。
  clear 必须 client、仅 core；覆盖枚举含 unmatched，默认20/最大200，client/session_id 排序。
- 验收：文本默认 work，省略 scope JSON 默认 all，显式三范围及分页 total 正确；--all 只取消分页，
  show 精确 ID 不加 scope。数组/对象旧形状不变，session_selection/origin additive，搜索文档 total
  与 selected_session_count 分开。scan 默认/--no-scan/signals 原合同不变；失败保持 partial/scan_incomplete。
  exit2 参数错误、exit1 冲突/写失败/身份不可用按 CLI 最终合同；clear 回执不伪造当前分类。
  unknown 先读回执，重试保留同 ID/负载/expected_revision；source 输出不泄漏配置路径。
  status：两项固定顺序；claude-mem 六种原因与判定顺序；Codex 有记录/三种无记录/core_unavailable；
  claude-mem 每种原因下 available 取值，含“先有 ok 运行摘要后 disable”与“有摘要但配置不可读”两例必须 available=false；
  partial 仅在 core 不可读、来源配置不可读、receiver.json 不可读或无效（含有记录时 receiver=unknown）时为 true，
  未配置/停用/未读取/来源不可用/版本不支持/无记录为 partial=false；各情形均 exit0；
  命令前后 core、配置文件、receiver.json 与 token 字节不变，不启动监听器，输出无路径/端点/token。
- 验证 L2，读写隐私/并发部分 L3：命令 golden/JSON 向后兼容、非交互与 PTY 键盘/窄终端/退出恢复，
  与任务 1 的 clear 七场景联合检查，CLI 与服务独立 oracle 一致。

### 5. desktop-menubar

- 依赖：3、4。范围：`internal/desktop/desktop.go` 与 desktop 测试；`apps/macos/AgentDeckApp/` 的
  snapshot DTO、读取控制器与会话 Popover，配套 `AgentDeckTests/`；按现有结构定位，不新建并行 App。
- 实现：wire v1 新 session_scope_view；旧 sessions.total/items/periods/work_signals 原义保留。
  selection/client/period/publication、counts/recent/work_signals 一致；新消费者缺 view 时仅会话区域 unavailable。
  临时三范围不写偏好；下次打开使用已确认默认；未知默认不能 fallback Work。
  来源详情的原因文案按架构 §3.3 封闭枚举映射（含 `origin_metadata_missing`；Codex OTel 会话显示 maintenance_origin
  文案并带 rule_version），未知码显示通用文案、不推断 auxiliary。
  向任务 7 提供可按显式 Selection 请求的 Go session_scope_view 与 Swift 解码接口；
  不将 Popover 控制器的当前 selection 作为 Widget 数据来源。任务 7 承担 Widget 调用与聚合生产链。
- 验收：快速切换、旧请求迟到、刷新/关闭重开、partial/stale/no-store、unknown 及旧 producer，
  不拼接不同 selection/generation；任务子代理留 Work。金额/额度始终全量，来源详情仅安全字段。
  七个已知原因码各有文案，注入未知码时走通用回退且分类显示不变。
  用户 UTC/本地时区显示按最终菜单栏合同，标本时间不当运行期证据。
- 验证 L3：Go/Swift DTO 兼容与控制器时序、受影响 macOS build/tests；原生 Popover 键盘、
  VoiceOver、窗口/分辨率/主题、session/client/period 联动验收，不以 React 断言替代。

### 6. settings-control

- 依赖：4、5。范围：`apps/macos/AgentDeckApp/SettingsWindowView.swift`、App 级设置请求控制器（新增）、
  CLI bridge 与 `AgentDeckTests/`；Go 只复用已批准接口，Swift 不直连 SQLite。
- 实现：默认范围、精确目标分类、有效覆盖列表（含 unmatched）、只读来源状态；source 定位仅 CLI。
  来源状态经 CLI bridge 读取任务 4 的 `session origin-source status`，固定两行：claude-mem 与内建 Codex background（OTel），
  后者按 ux/settings.md 五种状态（有记录、三种无记录、core 不可读）展示，无控件；未知 reason 通用文案。
  App 持有 pending/request ID；view 关闭不丢失操作；确认值与草稿分离，冲突读最新值后重新选择。
- 验收：saved+权限警告与未保存分离；publication_pending 不回滚默认。unknown 回执缺失后可显式
  同 ID/负载/expected_revision 重试；确定结果结束 pending 并解锁，旧 clear 回执不能移除较新覆盖。
  clear 后 not_queried，独立可见查询才显示分类；撤销不可见/excluded 覆盖不查源。
  当前 Popover 临时范围不变，后续打开及 Widget 新发布采用默认；loading/loadFailed 不生成 Work。
  来源状态两行与 status JSON 逐字段一致；“无记录”三态不显示为错误，core 不可读时两行均不可用且本组 loadFailed。
- 验证 L3：原生关闭重开/应用重启后的读回、并发 CLI、重复提交与 unknown/conflict 恢复；
  列表分页/删除焦点、键盘/AX、中英/主题。覆盖 ASC-SET-R1-F1、ASC-SET-R3-F1 已批准回归场景。

### 7. widget-summary

- 依赖：3 的同代 Selection/已提交投影、5 的 session_scope_view producer/decoder、6 的已确认默认读取。
  范围：`internal/desktop/desktop.go` 及 desktop 测试、desktop 命令的显式 Selection 读取接点与测试，
  `apps/macos/AgentDeckShared/DesktopWire.swift` 及 wire 解码测试、宿主 Widget 发布协调器与测试；
  `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift`、
  `WidgetSnapshotPublisher.swift`、`WidgetSemanticProjection.swift`；`AgentDeckWidget/WidgetSnapshot.swift`、
  `WidgetDomain.swift`、`WidgetTimeline.swift`、`WidgetViews.swift` 与共享/Widget 测试。
- 生产路径（本任务端到端负责）：宿主读取 confirmed/value/revision 默认，按受支持的
  `client ∈ all/codex/claude × period ∈ today/7d/30d` 组合发起显式 Selection 只读请求，scope 固定为
  本次已确认持久默认，复用架构 §6 Selection 和 §7 单选择 session_scope_view；Go 从任务 3 的
  已提交投影提供各组合 counts.sessions、origin_coverage 及 publication/time，Swift 转换为
  App Group 各组合的 session_scope_summary。此处选定“宿主按组合请求”路径，不新增多组合 wire 字段，
  不重解释旧 usage.presentation.scopes 的 sessions，也不复制 Popover 当前 view。
  source 读取/分类/聚合全部留在 Go；宿主只做选择、身份校验和白名单转换，不按 recent 列表自行计数。
- 组合一致性：各请求使用同一已确认默认版本与同一 publication 身份，复用架构 Selection 的
  publication_id 约束；Go 必须从该完整已提交代返回组合，不能以最新代静默替代指定代。
  指定代不可用、组合缺失、默认版本变化或身份/时间不一致即丢弃整批候选；按架构有界重试最多 2 次，
  仍失败保留旧完整 App Group 快照并标 stale，无旧结果则 unavailable，不发布拼接代次。
  任务 3/5 的接口接入和对应 Go/Swift 回归在本任务范围内完成，不能留给渲染任务猜测。
  默认未确认不启动新分类批次；Popover 切换不会改变批次 scope 或触发本链重新采集。
- 消费实现：session_scope_summary 白名单 codec，只有聚合与安全 publication/time；Magnitude small/large
  消费实际 entry client/period，中号不加 count，其他四种 kind 保持原义。旧 total 不改名。
- 验收：missing/invalid/unknown enum/client-period mismatch 只降级会话行，外层读失败整卡 typed failure；
  null unknown_count 与缺字段不同，Aux unknown=0。保存新默认/发布失败/WidgetKit 延迟保持旧整份
  scope/count/time，新接受投影一起替换；不读取设置库或探测宿主，不新增刷新预算/服务。
  ID/项目/模型/关系/正文/路径/覆盖列表/request receipts 不进入 App Group。
- 验证 L3：新旧 codec、序列化白名单、跨进程原子发布与失败恢复；所有真实 client/period 和三 family，
  大计数本地化/精度、VoiceOver 完整数字、Dynamic Type、WidgetKit 时间线及点击原动作。
  不以合成 all/today/30d 固定窗口代表真实配置覆盖。
  增加 Go→DesktopWire→宿主→App Group→Widget 联合 fixture：每个 client 的 today/7d/30d 数字刻意不同，
  已确认默认 Auxiliary、Popover 临时 Work 时仍发布各组合的 Auxiliary；仅切换 Popover 输出不变。
  默认改 All 后确认新批次才整体替换；未确认默认、跨组合插入写入、指定代丢失/迟到响应和任一组合失败
  都不得发布混合快照。对每个组合与同 publication 的 Go selection oracle 对照，费用仍全量。
  批次查询和转换成本归入任务 9 性能预算，不新增扫描、轮询或 WidgetKit 刷新预算。

### 8. durability

- 依赖：1–4；联合客户端恢复在 9 汇合。范围：`internal/backup/backup.go`、backup/store/session 测试、
  purge-index/restore 接点；仅隔离状态根和 fixture，不操作用户真实库。
- 实现：backup 私有 core snapshot 去除派生、receipts 与本机 adapter 定位，保留 preferences/overrides
  再算 digest；live core 不改，include-sessions 仍可选。restore 校验 schema/枚举/整数/重复身份，
  mint 新 epoch，自动证据与回执丢弃，覆盖先 unmatched，稳定出生 binding 一致才重激活。
- 验收：binding_limited 不凭同 ID/路径跨设备激活；显式 set 可重新绑定；purge-index 保留 core 覆盖/默认。
  空目标/加密/凭据合同不变；升级失败、损坏 archive、规则变化保留可读旧快照并正确 stale/partial。
  恢复不自动访问外部插件库，禁用来源不删除用量或用户覆盖。
- 验证 L3：backup/restore roundtrip、无 include-sessions、跨根/同 ID 不同 birth、schema-ahead、
  崩溃/权限拒绝和摘要验证；检查源字节与 live core 未被导出清洗逻辑修改。

### 9. integrated-acceptance

- 依赖：1–8 的适用独立评审/证据；最终扫描与工作信号/价格范围的集成候选可用。
  范围：跨子系统验收 fixture/harness、相关测试与本主题验收记录；不得借验收扩张产品定义。
- 实现验收：统一 canonical cohort 对照 CLI、菜单栏、Settings、Widget，AC1–AC8 逐项绑定实际内容。
  覆盖 work/auxiliary/unknown、维护适配器真实格式、切换/覆盖/clear/重扫/恢复链；
  事件身份/token/全部费用不变，unknown price 不补零；synthetic 重复计量需有证据或显式未确认状态。
- 性能：采用 scan-performance 最终批准的 requirements、architecture、performance-evaluation 与验收矩阵，
  在实际合并后的内容上测量有界适配器读取、trigger 写放大、受影响分区重算、范围切换及发布成本。
  不从本主题复制未稳定的数值或降低其预算；记录采用的文档 blob、候选 SHA、工具链、输入身份和有效样本。
  同输入 full-view 等价先通过，按该矩阵测 scan/无变化/增量/恢复和前台延迟；不以孤立 microbenchmark 代替。
  scan-performance 已合入 `main` `6fff208`，本分支已刷新包含它，在实际内容上测量；缺最终集成对象时
  仅该联合验收保持未完成，不反向撤销可独立通过的前序任务，不标记 AC8 完成。
- 验证 L3：适用 Go 全量/race/vet 与 macOS 集成/原生检查，复用精确内容可复用证据；
  不运行仅发布需要的 L4，不授予合并/安装/发布。需要超出现有权限的数据或安装时另取得相应授权。
  最后任务通过后按项目要求完成 Topic 证据边界与稳定 specs 同步；不把 Beads closed 当产品通过。

## 覆盖与验证责任

| 需求 | 主任务 | 必需交叉证据 |
| --- | --- | --- |
| AC1 分类 | 2 | 9 支持 SDK 格式的隔离端到端；不能用全 unknown 替代 |
| AC2 身份 | 2、3 | 8 恢复绑定；9 增量/全量等价 |
| AC3 统计 | 3 | 4、5、7 各客户端/窗口消费同一事实，9 独立 oracle |
| AC4 消耗 | 2、3 | 4–7 展示全量；8、9 操作前后事件/金额身份 |
| AC5 控制 | 1、4、5、6 | 7 下一发布默认；8 重启/恢复；9 联合链 |
| AC6 兼容 | 4、5、7 | 1 schema 边界；9 新旧 producer/consumer |
| AC7 恢复 | 1、8 | 3 发布回退、6 未知写入、7 原子快照；9 联合故障 |
| AC8 集成 | 3、9 | 2 采集边界、5–7 前台/发布成本，最终扫描/信号/价格预算 |

Go 测试统一 `scripts/run-go-test.sh`；Go core L2 变更要求 `scripts/run-go-test.sh ./...`，
L3 追加受影响包 race、vet、迁移/隐私/跨进程测试，不以小包通过免除必需全量。
macOS 使用当时仓库支持的构建/测试入口与原生验收，不写虚构命令。每任务记录准确命令、结果、
HEAD+相关 blob/指纹、依赖/工具链/环境身份。相同内容有效证据可复用，阶段切换本身不要求重跑。
失败/不可执行必须记录实际限制；没有证据不得写 PASS 或改验收条件。运行时不可用不降低需求。

本次只做 L0 文档设计；生产代码、数据库和系统配置保持只读。已用本工作树 CodeGraph 定位现有
store/desktop/backup/Widget/controller 接点，图只用于范围发现，不作 SQL、原生或运行期证明。
具体代码文件新增/拆分允许在任务边界内决定；跨任务合同改变必须先核对本计划与已批准架构。

## 当前交接

分解与文档集曾独立复评 PASS（Round 2），见 [reviews/tasks.md](reviews/tasks.md)；2026-11-03 集成修订后，
五份文档回到待评审（见 Documents），开发须等复审通过。所有实现 Dev/Review 未勾选。
基线已刷新到 `main` `660c815`（合并提交 `3ec8709`）；Codex 记忆提取与来源会话关联已登记为 roadmap
Backlog 与 deferred 协调项 `ad-codex-memory-source-linkage`。Settings 原型已补内建来源行及三个非默认状态场景。
九个实现 Beads 任务（`ad-auxiliary-session-classification-origin-store` … `-integrated-acceptance`）已于 2026-10-27
依据 Round 2 PASS 矩阵创建，当前均为 open、未认领；本轮修订改变了任务 1/2/4/5/6 的范围，开发认领前以本文件复审通过的版本为准。
评审 PASS 不授权开发或提交。2026-11-04 起七份文档对当前内容均已复评 PASS（tasks.md Round 6），文档集重新 ratified；
各文档尚未提交，提交须单独授权。实现任务的认领与开发须经单独的开发指令。
未合并主题状态在本文件及 Beads 维护，不平行修改 main 的 docs/status.md。

2026-11-04 任务 1 `origin-store` 开发（claude-code，未提交）：core migration 34（基于 origin/main 的 33；本地 `main` ref 落后于
origin/main，非本主题问题）新增 origin_preferences、origin_overrides（tombstone、binding_state matched/unmatched、binding_limited）、
origin_mutations、session_origins（reason_code 封闭枚举 CHECK）、origin_evidence、origin_adapter_runs；除 origin_mutations 外
各表与 telemetry_sessions 以单一 trigger 族推进 serving revision、置 derived generation dirty，键控表另写
`usage_derived_invalidations(kind='session')`。store 写路径为 `BEGIN IMMEDIATE` 短事务（回执→负载→CAS→no-change→写→回执→commit），
新增 `TelemetrySessionOriginMarkers`/`TelemetryBackgroundSessionCount`，`TelemetrySessionSourceKinds` 未改；
`internal/sessionorigin` 提供默认 DTO、覆盖列表、set/clear/scope 写服务与错误映射。schema 升 34 的连带修改：
desktop 与原生 `DesktopWire.swift` 的 31/32/33→34 兼容规则及其测试、三份 desktop 金标与 snapshot-performance 金标（差异仅 schema 数字）、
schema12 迁移测试。CEv1 WorkUnit `auxiliary-session-classification:origin-store`（7 项 required criteria）已建，证据待评审时按候选内容绑定。
验证：store/sessionorigin 定向、`-race` store/sessionorigin/desktop、`scripts/run-go-test.sh ./...`、`make vet`、`make check-whitespace` 通过，
`make test-macos-app`（隔离 HOME）：`DesktopWireTests` 的 supported schema 断言已随金标改为 34 并通过；
`MenuBarViewModelTests.testDateEventsInvalidateRenderedSurfacesWithoutRefreshOrFilterChanges` 两次运行均失败——它以设备 OCR 识别渲染文本，
识别结果为乱码，fixture 固定 core_schema 31，不经过本任务改动；未在基线上复现，作为未解决的环境相关失败记录，不计为通过。
`AgentDeckVerification/main.swift` 断言 maximumSupportedCoreSchema == 32，早于本任务已失效（当前为 33），未改动。

2026-11-04 任务 1 `origin-store` 独立评审未通过，Review 保持未勾选；完整结论、候选身份与验证边界见
[reviews/origin-store.md](reviews/origin-store.md)。本轮只更新评审和交接记录，产品代码、测试与配置未修改。
Beads 连接曾失败；用户要求处理后读写恢复，任务状态、指派、轮次标签及评审指针已完成失败回交同步。

2026-11-04 任务 1 `origin-store` 修复 ASC-OS-R1-F1（claude-code，未提交）：缺偏好行按已确认默认 Work/revision0
参与 CAS 后同值判断，首次保存 Work 为 no_change；回归测试与验证见 [reviews/origin-store.md](reviews/origin-store.md)
Repair 1。Review 保持未勾选，待复评。

2026-11-04 任务 1 `origin-store` 独立复评 Round 2 PASS，Review 已勾选；完整处置与候选身份见
[reviews/origin-store.md](reviews/origin-store.md)。必需 L3 回归仍未通过，任务证据门禁为 FAILED，
Beads 保持 in_review，提交与推送建议均以门禁 VERIFIED 为前置；不进入 Topic 完成边界。

2026-11-05 任务 1 `origin-store` L3 补齐（claude-code，未提交）：empty trace 确认为性能测试工具在 worker 不可见时
提前读取未刷盘 trace，已在本分支修复（等待 `resource.lifetime`）；LaunchServices 超时未复现；新候选全量、全库 vet 与
相关 race 通过。详见 [reviews/origin-store.md](reviews/origin-store.md) L3 补齐；工具改动待复评。

2026-11-05 任务 1 `origin-store` 新候选独立复评 Round 3 FAIL，Review 恢复未勾选；
完整处置、内容身份及门禁结果见 [reviews/origin-store.md](reviews/origin-store.md)。
本轮回交工具变更的修复，任务尚未到达提交边界。

2026-11-05 任务 1 `origin-store` 修复 ASC-OS-R3-F1（claude-code，未提交）：trace 完成屏障改为生产者 flock 释放 +
lifetime，回归测试、全量/vet/相关 race 见 [reviews/origin-store.md](reviews/origin-store.md) Repair 2。Review 保持未勾选，待复评。

2026-11-05 任务 1 `origin-store` 独立复评 Round 4 PASS，Review 已勾选；完整处置、候选身份与证据复用见
[reviews/origin-store.md](reviews/origin-store.md)。当前候选 CEv1 已 VERIFIED（7/7），到达任务提交检查点；
未执行 Git 交付，下一任务仍以本任务完成授权提交交付为前置。

2026-11-05 本次本地提交交付任务 1 `origin-store`：包含 Round 4 已评审的 23 个候选文件及本文件、
[评审记录](reviews/origin-store.md)，候选门禁 VERIFIED（7/7）。本提交的 Git 对象为交付原件；
完整提交 ID、签名及提交态证据对账在对应 Beads 任务和 CEv1 保存。用户授权范围为本地提交，未推送。
下一任务为 `origin-adapter-reducer`，本次提交核验并完成任务关闭后解除其前置依赖。

2026-11-05 任务 2 `origin-adapter-reducer` 开发（claude-code，未提交）：`internal/sessionorigin` 新增共享归约器
（§3.3 封闭枚举固定顺序；只有绑定 matched 的覆盖参与 effective；稳定出生绑定 = client/session_id/最早起始时间，
无起始时间为 binding_limited；`origin_input_token` 规范编码覆盖 key、各副本 identity/prefix_hash/parser_version、
解析器证据及 reducer/规则版本，追加与移动不变、重写改变，供任务 3 的 sessions 侧共用）、内建
`codex-otel-background-v1`（两条支持条目精确匹配，其余标记 origin_rule_unsupported）、只读 claude-mem 适配器
`adapters/claudemem`（显式绝对根；mode=ro + query_only；仅 schema_versions 最大版本 68、sdk_sessions 列/唯一索引元数据
与 id/memory_session_id；WAL 缺 sidecar 即 unavailable 而不创建；规范 UUID、200 一批、单读 250 ms、整轮 2 s，
超限 partial 且未读候选 unknown）、本机非导出来源配置 `<state>/session-origin/sources.json`（0700/0600，
set/disable 命令属任务 4）与 `Reconcile`：在三处现有 SettleAndProject 之后（scanruntime 两条路径、CLI watch）
读索引身份列与 core 标记，只写分类或证据变化的行，claude-mem 运行摘要与证据同事务，离开索引的会话删除派生行、
覆盖保留。store 新增只读 `SessionOriginSources(For)` 与 `OriginDerivedState`。
测试为隔离 fixture：真实 OTel 导入→settle→投影→归约（exec/TUI 两条目 auxiliary、晚到 rollout 撤销、
支持表外 unsupported、跨 client 不命中、usage 事件与 usage_sessions 前后一致）；按 claude-mem 15.2.1 源码
（SessionStore.js SHA-256 已复核与架构 §3.1 一致）合成的忠实 WAL fixture 端到端（memory ID 命中、content-only/
synthetic/任务子代理/跨 client 不隐藏、源库与 WAL 字节不变、查询白名单、0/1/200/201 批次、版本 67/69、无唯一索引、
伪根、权限拒绝、超时 partial）；禁用/重启用、配置不可读、未配置、无变化不推进 serving revision、增量=全量。
变异：去掉 TUI 条目、去掉“无变化不写”各使对应测试失败，已恢复。
验证：`scripts/run-go-test.sh ./...`、`go vet ./...`、`-race` sessionorigin/scanruntime/store 通过。
候选：HEAD `55dcf41` + 14 个路径，摘要 `a9e50e716877f4e843ad079e03bbb5a450857f72677b3923a7e12965704744de`
（sha256 of `head=<HEAD>` 加按路径排序的 `path=blob` 行）。CEv1 WorkUnit
`auxiliary-session-classification:origin-adapter-reducer`（7 项 required criteria）已建，证据待评审时绑定。
残余：sessions 侧 origin_input_token 列与 token 不一致投影属任务 3；Reconcile 读与写分属两个事务，
期间的标记撤销由下一协调阶段与任务 3 的 token 核对收敛；claude-mem 配置时每个协调阶段写一次运行摘要，
会推进 serving revision，写放大由任务 9 按集成预算测量。

2026-11-05 任务 2 `origin-adapter-reducer` 独立评审 Round 1 FAIL，Review 保持未勾选；
候选身份、反例与修复边界见 [reviews/origin-adapter-reducer.md](reviews/origin-adapter-reducer.md)。
本轮只更新评审、证据与交接记录；尚未到达任务提交边界，后续任务保持依赖约束。

2026-11-05 任务 2 `origin-adapter-reducer` 修复 ASC-OAR-R1-F1（claude-code，未提交）：claude-mem 适配器先经扩展定位确认
已安装 `claude-mem-plugin` 为已审查的 15.2.1，未知/更高/不能确认的版本 unsupported、不产生证据，已有自动证据失效、覆盖保留；
回归、变异与全量/vet/race 见 [reviews/origin-adapter-reducer.md](reviews/origin-adapter-reducer.md) Repair 1。Review 保持未勾选，待复评。

2026-11-05 任务 2 `origin-adapter-reducer` 独立复评 Round 2 FAIL，Review 保持未勾选；
原发现处置、当前候选及验证边界见 [reviews/origin-adapter-reducer.md](reviews/origin-adapter-reducer.md)。
本轮只更新复评、证据与交接记录，任务尚未到达提交边界。

2026-11-05 任务 2 `origin-adapter-reducer` 修复 Round 2 仍 OPEN 的 ASC-OAR-R1-F1（claude-code，未提交）：插件确认改为统计全部
作用域/marketplace 的 claude-mem 安装，仅唯一且为 user 15.2.1 时适配，混合安装保守拒绝并使已有证据失效、覆盖保留；
回归、评审反例重放、变异与全量/vet/race 见 [reviews/origin-adapter-reducer.md](reviews/origin-adapter-reducer.md) Repair 2。Review 保持未勾选，待复评。

2026-11-05 任务 2 `origin-adapter-reducer` 独立复评 Round 3 PASS，Review 已勾选；
发现处置、精确候选、验证结果及任务提交检查点见 [reviews/origin-adapter-reducer.md](reviews/origin-adapter-reducer.md)。
本候选 CEv1 VERIFIED（7/7），进入任务提交检查点，尚未提交或推送。
后续 `scope-projections` 仍以前置任务完成授权提交交付为条件，未启动该任务或进入 Topic 完成边界。

2026-11-05 本次本地提交交付任务 2 `origin-adapter-reducer`：包含 Round 3 已评审的 15 个候选文件及本文件、
[评审记录](reviews/origin-adapter-reducer.md)，候选门禁 VERIFIED（7/7）。本提交的 Git 对象为交付原件；
实际提交 ID、签名和提交态证据对账在对应 Beads 任务及 CEv1 保存。用户仅授权本地提交，未推送。
提交核验并关闭本任务后解除 `scope-projections` 的依赖；后续开发仍需单独授权。
