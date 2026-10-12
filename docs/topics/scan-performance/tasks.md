---
status: active
created: 2026-10-09
updated: 2026-10-09
---

# Scan performance：设计集与渐进实施候选

工作区 `agent-deck.scan-performance` / `feature/scan-performance`，创建基线
`32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`。当前评审状态见 Documents；详细结论见各文档的 reviews 记录。
本次重新评估移除“额外常驻引擎是快开前置依赖”，区分新安装当天优先、已有库当天读取、完整恢复，
再按实测热点调整更新。该段为设计阶段历史；Task 1 的当前实施状态与 E0 记录见下方，其他产品任务未启动。

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar.md | [x] | [x] |
| ux/cli.md | [x] | [x] |
| architecture.md | [x] | [x] |
| performance-evaluation.md | [x] | [x] |
| tasks.md | [x] | [x] |
| ux/settings.md | [x] | [x] |
| ux/widget.md | n/a | n/a |

Settings含用户要求的OTel开关；Widget不改表面，保留既有兼容性。
requirements → UX → architecture/evaluation → tasks按依赖一批评审，
每文档有自己的内容身份和结论。Draft不等于Review/CEv1/产品验收。

## 综合设计评审范围

评审对象是本文件Documents矩阵的七份设计候选，作为同一批进行一次独立综合评审。
依据[Documentation Workflow](../../documentation-workflow.md#document-lifecycle)和
[Review Records](../../../.agent-instructions/review-records.md#structure)，合批不合并各文档的结论或历史。

| 顺序 | 文档 | 与整套设计联合核对的内容 |
| --- | --- | --- |
| 1 | requirements.md | 首次空库/已有库/新记录待入库/已有保存视图；可用数据定义、范围和验收目标 |
| 2 | ux/menubar.md、ux/cli.md、ux/settings.md | first/partial/ready、日期与缺失字段、CLI语义、OTel开关、App退出；与共享原型对应 |
| 3 | architecture.md、performance-evaluation.md | 首批提交和发布、全inventory保留、只读今天、身份/coverage、所有权、真实对照与采用/停止条件 |
| 4 | tasks.md | 文件范围与前置依赖能实现上述行为；全部门和风险有明确任务承接 |

同时纳入受影响的cli-design/cli-manual契约、共享prototype源码/标本manifest、
feasibility-results.json、native-ab-results.json、first-loading-results.json与对应harness。
关联源码只用于核对现状、接口和可实现性；本批不评审尚未实施的产品改动，不预造产品PASS。

联合重点：

- 空库75.438s→当天优先1.554s、已入库今天约104ms的收益适用条件是否讲清，
  原生模型15ms恢复不能替代首次导入，数据准备不能冒充真实popover首帧。
- 今天优先是调度策略，不能遗漏旧活跃会话、伪造全部覆盖，或把未准备字段写成0。
  首批发布不等全局worker退出，后台补历史与CLI全范围义务保持一致。
- 新记录入库原位15.545s、usage12.045s及621.5MiB资源负结果有明确定位和验收门；
  用户认可收益对照不代表这条路径已经实时，也不代表全部架构/产品门通过。
- App-only工作退出后的暂停与CLI已接受义务的边界、跨天/恢复/失效、quota解耦、
  OTel关闭和capture-only、跨源去重，以及额外常驻/多进程的有条件采用相互一致。

评审结果逐文档落到reviews/requirements.md、ux-menubar.md、ux-cli.md、ux-settings.md、
architecture.md、performance-evaluation.md、tasks.md；首次实际评审才创建记录。
一个独立报告可被各carrier精确引用，但每份保留自己的内容身份、依赖覆盖、结论和round。
全部七份通过且适用设计证据门成立，才具备批准实施分解的条件。
七份设计文档已通过独立评审及复评；当前结论与证据门见上述七份 reviews 记录。设计通过不启动实施，产品任务仍须各自的开发授权与验收。

文档dispatch仍为`ad-scan-performance-doc-req`、`ad-scan-performance-doc-arch`、
`ad-scan-performance-doc-perf`、`ad-scan-performance-doc-ux-menubar`、
`ad-scan-performance-doc-ux-cli`、`ad-scan-performance-doc-ux-settings`、`ad-scan-performance-doc-tasks`。
原Bug不因设计或实验被关闭；本矩阵不授权提交、推送、PR、安装或发布。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `baseline-and-open-path` | [x] | [x] |
| 2. `serving-snapshot-and-bootstrap` | [ ] | [ ] |
| 3. `incremental-projections` | [ ] | [ ] |
| 4. `bounded-source-pipeline` | [ ] | [ ] |
| 5. `event-updates-and-cli` | [ ] | [ ] |
| 6. `otel-capture-and-reconciliation` | [ ] | [ ] |
| 7. `cross-surface-acceptance` | [ ] | [ ] |

默认主线1→2→3/4→5→7。Task4只有source profiling显示必要才选入；
Task6是需求承诺的交付能力，默认off仅是用户运行时选择；off时Task2仍须成立。
额外常驻Go/multiprocess不是必选Task，
需要E3证据和明确的设计增补，不预创建它们的实施任务。

## 1. baseline-and-open-path

产物：E0精确输入/内容/工具链身份、真实native warm/empty-memory/full launch/update基线，
scanruntime/usage/session/derive和主线程成本、re-read/版本触发证据。保留可解释的前后结果。
必须分别覆盖：库空但本机日志巨大；今天已入库；已有历史但今天未入库；有效上次展示。
本轮前两项有pilot收益，第三项原位复核仍15.5s；不把移位SQLite副本的5.7s推广为已解决。
文件边界：App MenuBarItemController/ViewModel、Shared runner/coordinator logging，
scanruntime/ingest/desktop阶段计量、隔离harness；不改变统计结果或parser。
正确性：真实popover、默认quota及所有tab；旧数据与最新数据时间分开；无数字不记为成功。
同一个实验刺激/终点，交错A/B；hosted空coordinator不叫完整冷启动。
验证：L2影响包/native focused；并发计量新增则相关L3。隐私日志、计时归属和内容绑定。
门：`baseline-identity`、`measurement-attribution`、`source-impact-map`、`privacy`。

### Task 1 实施与 E0 记录


工作区 `agent-deck.scan-performance`，分支 `feature/scan-performance`。
此记录承接 `tasks.md / baseline-and-open-path` 的实现与测量，不是独立 Review PASS。
计量只增加诊断；没有改变 parser、facts、统计结果或 v1 stdout。

### 计量边界

- 新增计量用 uptime/monotonic 时钟；snapshot 的 generated_at 是数据时间，不是打开耗时。
- 正常点击仍只构建/显示已准备的内存视图。`popoverOpen` 在 show/activation 后结束，不能冒充首个有效数据帧。
- 独立 Debug App 经 LaunchServices 启动，真实 coordinator、quota、helper、scan 和 publication 保留；不使用 hosted 空 coordinator 代替完整启动。
- AppKit 鼠标 down/up 经本 App 的正常事件路由；这不是全局物理鼠标输入。
- 数值帧要求真实 NSPopover、可见 window、非零 window number、bitmap 中的 token 和金额匹配模型。未匹配或不可见的样本记失败，不填 0 延迟。
- 帧终点是 AppKit bitmap draw/readback 完成；修复候选先捕获更新的旧/新两个 bitmap 及终点，再执行任何 OCR 验证，避免旧帧验证阻塞 refresh 或进入最新帧耗时。它不是 compositor/photon 时间，不能据此宣称 P01 已通过。
- native trace 最多 1,024 个计时样本、16 个 helper 失败分类；只保留固定 stage、数值、机器错误代码和环境布尔值。
- Go `AGENTDECK_SCAN_PROFILE=1` 启用共享 ingestion 原子计数，在 stderr 输出聚合阶段摘要；默认不输出该摘要，v1 stdout 保持原结构。
- usage/session worker wall 重叠，不能相加；read/decode/send/source-lifetime 是累计 worker 时间。RSS 是 worker lifetime peak。

### 来源与版本影响图

| 路径/触发 | 实际影响 | 直接验证 |
| --- | --- | --- |
| `MenuBarItemController.togglePopover` → `MenuBarSurfaceView` → `MenuBarViewModel` | 只读取 coordinator 内存，重新创建 hosting view；不扫描、不查询 SQLite | native 路由/数值 bitmap；MenuBarChromeTests |
| coordinator full refresh | quota → global scan → worker lock handoff → snapshot helper/decode → state/widget publication | 分段 trace；DesktopRefreshCoordinatorTests |
| `scanruntime.executeProduction` | 全 inventory discovery/consumer plans → 共享 reader → usage/session → derived-cache publication | StageProfile、ingestion counters、scanruntime/ingest 回归与 race |
| 未变且完整的 source checkpoint | identity/size/mtime/ctime/parser 等相容时不重读正文；不意味着 derive 被跳过 | E0 read_bytes=0；TestScanSkipsUnchangedSourceWithoutWritingState、TestPreparedIngestionSkipsUnchangedWritesAndPreservesOwnership |
| append | 保留已提交 cursor、partial record 和累计 token/model/turn context；只处理需要的来源 | TestSharedAppendPlanIncludesStoredPartialRecord、TestCodexCumulativeUsageSurvivesAppendRestartResetAndArchiveCopy |
| rewrite/truncate/保留 mtime 的改写 | 验证 identity/anchors，重建必要来源，不能假装 append | ingest rewrite/growth 和 usage/session preserved-mtime 回归 |
| usage/session parser version 变化 | consumer checkpoint 不再相容，必须重读相应来源 | TestUsageParserVersionRebuildsUnchangedSource、TestScanClaudeIndexesNestedModelAndRereadsLegacyParserVersion |
| core/schema/epoch/revision、timezone/hour/day、价格与 cache algorithm 变化 | derived snapshot cache identity/validity 失效；没有 source 重读也可能重新 derive | derived-cache header/validity 源码与完整 Go 回归 |
| 新 fixture token 事件 | Codex 必须含 session、turn_id、model；缺 turn_id 的 token 事件被现有 parser 跳过 | fixture 初始 imported=1、追加 imported=1 断言；未改 parser |

### 已确认的测量事实

真实语料按每文件初始长度截取，保留原 mtime；仅包含三个批准的 JSONL 根。
它是冻结的每文件 prefix 集，不是全文件系统同一瞬间快照。报告只导出 corpus/DB/源码/二进制哈希、耗时和计数。
原始 JSONL、数据库、helper 输出和识别文本不进入本记录。

首轮真实引擎 pilot：5,365 文件、3,473,828,548 bytes，corpus SHA256
`94ffd266985c64f6025e9a2c2975e417d90d9ade33d673391f886e40cc272ee7`。
其添加的合成 token source 缺 turn_id；真实日志的引擎测量保留原身份，不能重标成修正后 fixture 的结果。
该版 synthetic native 数值帧不用于有效数据结论；后续版本增加实际 token 入库与变化 oracle。

- 空库全 scan + snapshot：139.043、142.658、146.296 秒；每次 read_bytes=3,473,828,548。
- 已导入、未变来源：8.120、8.959、13.987 秒；read_bytes=0，derive=6.940、7.598、12.191 秒。
- 历史已导入、当日提示来源待入库：15.355、14.966、28.966 秒；read_bytes=3,879,096。
  usage=4.390、3.916、8.163 秒，session=6.884、6.359、13.511 秒，与 usage 重叠。
  worker peak RSS=650,874,880、713,994,240、704,868,352 bytes。
- CLI-only 的第四组只是已导入 engine warm control；没有 native restore，不能称为 saved-display 首帧。
- 上述 pilot 与旧 75.438 秒实验不是相同输入/计量条件，不计算加速比。

### 验证与交付边界

完整 Go 回归在最终 production Go 状态通过；首次并行回归的 LaunchServices 一秒超时在串行回归未复现。
受影响 scanruntime/ingest race、Go vet、darwin arm64/amd64 build 通过。
原生 shared/coordinator/chrome focused XCTest 共 53 项通过；计量/fixture 后续变化仅追加其受影响验证。
所有测试通过只支持对应代码/输入，不等于性能预算或独立 Review PASS。

Task 1 实施已交接独立 Review；完整脱敏事实见 [E0 执行记录](evidence/baseline-and-open-path.json)。
独立复评 Round 2 为 PASS；Git 文本证据门禁及交付检查点见
[baseline-and-open-path 复评](reviews/baseline-and-open-path.md#round-2--2026-10-12)。
Dev/Review 已完成；交付状态见 Beads `ad-scan-performance-baseline-and-open-path`，Task 2 尚未启动。
修复候选的实际四条件测量见 [修复 E0](evidence/baseline-and-open-path-repair-r1.json)；
6 个样本完整成功，2 个空库样本更新捕获失败并从成功分布排除；不等于最终性能或 native 验收通过。
历史最终真实规模 verifier 保留 default quota 和五个 tab；old/new 数值帧匹配，latestDataChanged=true。
其 update_latest 不复用为排除 OCR 的更新基线；超时后的样本隔离与刺激标识适用性更正保留在 E0 记录中，不改写原观察。
空库完整路径在现有 helper 120s deadline 超时；没有有效数据帧，不填 0、不计成功分布。
最终 trace 八个阶段都有完整聚合，0 dropped；today 已入库组 presentation=5,021 次、累计4.825s，
popoverOpen=6次/5.638s，publication=2次/4.199s（后者为含异步 widget write 的 wall，不能称主线程 CPU）。
旧逐条 trace 被 presentation 挤满的阶段归属不复用；其数据正确性、绑定的有效帧和超时事实按记录里的影响评估保留。
16.65MB单源 profiling off/on 三对交错，六份提交 tokens 全等；scan中位703.669→749.628ms，
只是并发开发负载下的合成校准，不推广为真实语料开销或性能收益。
复现实验用 scripts/scan-performance-baseline.py；默认四条件，--conditions 只用于有明确变更的 scoped 复核。
AppKit 帧含 bitmap readback；这些不是最终用户首帧/尾延迟门，也不证明已经解决扫描性能问题。
后续 serving/bootstrap、incremental projection、watch/OTel、最终 ≥100 native / ≥20 heavy / idle 门仍属于各自任务。

## 2. serving-snapshot-and-bootstrap

入口是Task1的E0基线/归属/影响清单及有限pilot可行性输入；1.554s/104ms/15ms只用于方案选择，
没有native frame或完整E1通过证据。E1是Task2实现后的出口门，不是入口前置。
文件：internal/desktop serving-cache与只读generation接口，Shared wire/refresh，
App startup/model、native fixtures，scanruntime首批调度/提交点通知、ingest inventory分批接口。
仅扩展必需接口，不全盘重写scanruntime或facts schema。
范围：首次today优先批次与partial发布、全inventory/coverage保留；今天已入库时按窗口只读bootstrap；
完整envelope保存/restore、身份、0700/0600/fsync/rename，额度解耦；点击只读已准备内存。
首批不能等待全局scan终结/worker退出，其他字段/时段按需准备；未知project等不可补0。
App-only bootstrap作业的owner/暂停/已提交source checkpoint也在本任务落地，
不等Task5才满足退出App停止其历史补齐；未提交source回滚，CLI义务保持原契约。
本任务落地持久inventory/domain coverage及stats/summary --no-scan的v1 warnings/partial映射，
partial/unknown/update_pending不启动扫描，不把未声明覆盖读成完整。
验收：P01–P04/P11；epoch/state/schema/owner/mode/size/损坏/跨天/DST，旧有效数据保留，
默认quota/全tab/筛选/焦点，slowquota30s与scan>120s；分别报旧值及新值到达时间。
不强求warm路径比当前更快，要求无退化。恢复收益必须真实native对照，cache miss不能漏测。
验证：L3相关desktop/store/CLI/native回归、full Go、race/vet、macOS双架构；
最终≥100次各native条件。仅已变化的影响范围重跑。
出口：E1产品验证及`serving-identity`、`atomic-publication`、`open-path-latency`、
`startup-bootstrap`、`wire-compatibility`、`stored-only-coverage`；Task3依赖此出口，不以pilot替代。

## 3. incremental-projections

前置Task1热点证据和Task2展示保护。补上原位usage域12秒的SQL/归属/reconciliation定位，
与现有derive/session成本一起评估；仅改显示范围不会消除新日志入库的等待。
文件：store dirty/revision、usage presentation/signals、session受影响集合、desktop projector。
范围：仅更新受影响时间/client/model/session分区；distinct/median/peak/top等独立状态，
价表/算法revision失效、shadow derivation和active切换。先保持现有facts/外部ID。
验收：P07/P10和E2/E4；同delta、同DB起点的整个更新周期A/B；full oracle等价；
late/cross-day/remove/rewrite、非可加统计、All不重复、旧publication可读。
如果profiling推翻热点判断，先改设计范围，不全盘迁移数据库。
验证：L3相关Go/store/usage/session/desktop、race/vet/macOS双build及nativewire/筛选。
门：`partition-equivalence`、`nonadditive-statistics`、`generation-switch`、`update-latency`。

## 4. bounded-source-pipeline

有条件：Task1确认重读/内存/事务成本，或Task3后P07/P08/P10仍被source处理限制。
文件：ingest/usage/session/store checkpoint与scanruntime调度；不替换SQLite driver。
范围：consumer上下文、boundedchunk/实际decoded记账、短事务与backpressure；
1/2/4/8workers只按完整周期证据选；shadow source事实/新schema仅在rewrite原子性确需时引入。
验收：P07–P09；append/rewrite/truncate/move/parserbump/超大行/crashpoint/磁盘满，
consumerPlan/Seal、FTS/backup/restore/directSQL影响清单；incremental/full精确等价。
若更大worker无≥15%整周期收益或资源失败，保留小worker；多进程须另过E3。
验证：L3相关回归、fullGo/race/vet/macOS双build、迁移/资源测试及最终≥20heavy样本。
门：`bounded-accounting`、`source-atomicity`、`checkpoint-correctness`、`replay-equivalence`、`migration-preservation`。

## 5. event-updates-and-cli

前置Task2；实时处理目标依赖Task3/适用Task4。文件：App watcher/lifecycle/Shared刷新，
现有scanruntime finite任务与desktop serving发布、CLI contract tests。
范围：文件事件有界合并→现有单协调者→有限任务→原子publication→App通知；
启动/唤醒/明确漏事件有限校对；无固定刷新周期。CLI/app共用facts与cache，
任意CLI查询仍读已提交DB；保留scan/watch/--no-scan/v1stdout/stderr/Ctrl-C契约。
默认有限helper退出，Appquit停watcher并在checkpoint暂停App-only历史补齐；
有限作业需owner/暂停策略，CLI已接受工作按既有契约完成。
验收：P06/P10；通知遗漏单列、连续写入、App/CLI同时请求、Ctrl-C、Appquit、
崩溃/唤醒/versionmismatch、publication失败旧结果保留。事件触发快不等于统计处理快。
如持续OTel或测得spawn/IPC主导，按E3补充常驻扩展的lease/capability/退出资源验收。
验证：L3相关Go/race/vet/build、真实native/CLI隔离集成与idle30min。
门：`event-delivery`、`finite-request-compatibility`、`cli-app-sharing`、`idle-resource-budget`。

## 6. otel-capture-and-reconciliation

前置Task2/5；producer identity/transport先验证。文件：telemetry receiver/journal/reconciler，
store observations/usage桥接、Settings/CLI偏好与状态。off不能影响快开。
范围：用户开关/defaultoff、配置preview不自动应用，loopback白名单/durableACK，
journalcap/retention、exactJSONL跨源身份、coverage/gap。没有证明就capture_only。
持续listener随App生命周期；独立后台模式显式另启，不因开关默认启用独立daemon。
unknown默认、脱敏配置检查/有效event/明确失败的观察判定需GUI/CLI一致，静默不推断断连。
capture_only是身份未验证时的能力状态，接收/持久化/状态仍须交付；
完整reconciliation依承诺的能力门验收，不以defaultoff/capture_only省略Task6。
验收：P05/P07/P10适用链路；正常/memory/guardian、SSE/WebSocket/retry/失败/中断，
重复delivery/import、Appquit/restart/diskfull/crashpoint；无正文/headers/凭据持久化，
tokencomponents不双加，conflict不overwrite，历史unknown不伪补齐。
验证：L3相关Go/race/vet/build、隐私/实际支持版本隔离producerintegration；
mock与actual分别列，未测真实producer不宣称完整实时补采已实现。
门：`durable-ingress`、`request-identity`、`transport-coverage`、`cross-source-idempotency`、`coverage-truth`。

## 7. cross-surface-acceptance

前置Tasks1/2/3/5/6及适用Task4独立PASS/requiredgates；OTel接收、Settings/CLI、身份与去重门纳入完成边界。
延期承诺的OTel范围须先取得用户明确批准，同步requirements/UX/architecture/tasks完成边界，
并在现有Task6/Beads记录承接延期范围和门；实现者不可自行排除。当前没有延期批准。
范围：native各条件≥100次、heavy≥20次、idle30min；全launch/首次click/update三指标，
默认quota/全部tab、cachemiss/跨天/升级/restore、CLI/Widgetlogical对照、Appquit生命周期。
验收P01–P11适用边界及E1–E4，OTelon/off/disconnect分列；旧值与最新值分列。
只复用未变且可target-bind证据；规模输入不足或native失败明确未验证。
同步最终批准contract到cli-design/manual和原型；topic-local矩阵/reviews。
不执行未授权的L4release/安装/发布，不更新main全局状态或关闭未交付Bug。
门：`native-open-path`、`representative-performance`、`cli-native-equivalence`、`upgrade-and-restore`、`lifecycle-failure-matrix`。

## 2026-10-09 设计阶段的历史验证边界

产品worktree的Go/Swift保持只读。原生试验只修改私有HEAD导出；结果保留耗时与校验摘要。
浏览器原型仍用于文案/状态/布局，旧浏览器首帧不用于产品性能门或加速比。
设计实验结束时七项Review均未勾选；当前状态见 Documents。实验结果见[先行评估](performance-evaluation.md)，不替代Development/Review/CEv1。
本轮原生数据准备12对A/B为1,919.185→15.089ms（中位），24样本逻辑一致；
真实popover两次未得到有效样本，因此没有native首帧或warm点击加速结论。
持久[脱敏原生摘要](ux/prototype/serving/native-ab-results.json)保留对照和失败边界，
不能把数据准备的99.2%减少写成P01/P02已经通过。
本次首次安装/历史矩阵另见[脱敏结果](ux/prototype/serving/first-loading-results.json)：
相同5,336文件/3.457GB空库全量75.438s→当日优先1.554s（3对），today投影一致；
已入库today只读约104ms；尚未入库today的backup组5.7s，原位15.5s，差异未定位。
首批采集/数据库查询/保存视图恢复是三个前置状态，不能混成一个首屏比值。
这些不勾选产品Dev/Review，不能证明nativefirstframe或实时新数据门。
本轮L0为topicdocs/local links/whitespace和harness语法；原型源码未再变化，不重复已有build/68项交互。
