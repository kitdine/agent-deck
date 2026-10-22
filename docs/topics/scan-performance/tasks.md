---
status: active
created: 2026-10-09
updated: 2026-10-20
---

# Scan performance：设计集与渐进实施候选

## Task 2 最终状态：PASS / 完成 — 2026-10-20

用户明确授权修改必要标准和记录，接受当前Task2并要求全部事项完结。
本节优先于下方所有历史的`in_review`、缺证、失败后继续采样/优化等交接建议。
最终依据为[用户批准的验收修订](requirements.md#task-2-用户批准的最终验收标准--2026-10-20)：
`open-path-latency`、`startup-bootstrap`、`e1-native-performance`三项豁免；
其余十项仍必需，沿用适用PASS；整体实现审查R33 PASS、R30/R32有界PASS保留。
标准修订本身由用户直接批准，未伪造新的独立审查轮次或重写旧审查结论。

完整100次原生首击均有效：p95=536.842034ms、p99=623.638814ms，
原300ms/600ms数值门未达标，历史失败证据保留。最终Task2 PASS表示按修订标准获接受。
终态证据绑定新ContentState，十项PASS以显式roll-up复用，三项豁免前后定义可审计；
精确门禁及Beads收尾回执见[最终接受回执](evidence/task2-acceptance-waiver-20261020-receipt.json)。

Task2本地工作区验收完成即为本次授权边界，Beads可按专属本地验收例外关闭；
无Git提交、推送、集成或发布，Topic与Tasks3–7继续开放。本决定不启动下一任务。
Documents矩阵保留既有独立审查事实；本次仅验收豁免文字由用户直接批准。

工作区 `agent-deck.scan-performance` / `feature/scan-performance`，创建基线
`32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`。当前评审状态见 Documents；详细结论见各文档的 reviews 记录。
本次重新评估移除“额外常驻引擎是快开前置依赖”，区分新安装当天优先、已有库当天读取、完整恢复，
再按实测热点调整更新。该段为设计阶段历史；Task 1 的交付与 E0 记录见下方，Task 2 按上述用户批准的最终标准完成；Tasks 3–7 未启动。

## 当前验收范围调整 — 2026-10-19

用户已明确批准 [撤销 P03 独立验收](requirements.md#p03-独立验收项撤销--2026-10-19)。
本节优先于下方历史交接中的 P03 要求：不再要求 P03 100ms 分位门、transition 百次、
专用采集器修复、轻量体验验收或权限排查；不因撤销而重测既有切换功能。
切换不等待 helper/扫描/OTel 仍为架构约束，实际影响路径或发现卡顿时针对性检查。

SP-SB-R25-F1、SP-SB-R28-F1 的 P03 采集器工作因用户撤销范围关闭，非修复通过；
不再按历史末尾修复指令继续，也不重新计算修复轮次。P02 R24 有界通过保留。
Task 2 仍未完成，Review 不勾选；其他 E1/startup/压力、适用正确性及整体审查要求保留。
本次仅变更需求/评估/任务文档及发现处置，不是新的独立评审；三份变更文档 Review
取消勾选以免把旧评审绑定到新内容，其余设计不机械重开。

CEv1 同步核对更正：此前空结果源于查询误用了 `namespace`，实际映射字段是
`ce_namespace`；不再将其解释为图谱不可用。2026-10-19 已通过 MCP 读到本 Task 的
13 个 required 准则及 R24/R28 原始证据，没有独立 P03 criterion，不能删除整个
open-path-latency、e1-native-performance 或 independent-review 来实现范围撤销。
当前未定位到 provider 的 PROFILE/schema/参数化写入模板，按 evidence.md 的写入前提，
本轮未自行构造写操作、未覆盖历史证据、未发布新 gate 结论。图谱范围同步仍待完成；
旧 P03 失败不授权重新启动采集器修复。

### 去掉 P03 后的剩余项核对 — 2026-10-19

本轮仅核对现有记录和源码身份，没有测试、采样或新的独立评审。
R24 持久证据所列353个源码输入SHA-256全部匹配当前内容；其固定100次完整恢复模型
p95=1104.790923ms≤1125ms，5个慢样本保留。P02仅此有界范围通过，不外推到真实首击。
Native300/0失败/1既有skip及最终focused10保留；其他9项机制/L3准则在R24/R28图谱
已有pass记录，不因P03撤销重跑。当前OS不同于原测量环境，原结论保留在原cohort，
本轮不宣称当前OS已重新验收或把历史记录改绑到新内容状态。

| 剩余事项 | 已有事实 | 真正缺口与后续边界 |
| --- | --- | --- |
| P01真实首次打开与open-path-latency | 存在历史warm/首击与pilot，R24/R28该准则仍not_verified | 还没有覆盖最终适用条件的完整有效首击结论；先复核已有报告适用性，不自动重跑 |
| startup-bootstrap：P02其余范围 | P02完整恢复模型固定100通过；P04/P11已按用户决定接受有界启动证据 | P04/P11不再要求百次或严格p95补测；其他未覆盖首击/启动范围仍不能用模型通过替代 |
| E1真实压力及适用正确性 | 机制与回归已有pass；e1-native-performance仍not_verified | slow quota 30s、实际scan>120s重叠等最终矩阵证据不足；不包含已撤销P03，不机械重测旧切换 |
| 整体独立审查 | R24为有界PASS；R28 FAIL由P03采集器缺陷触发，现已按用户决定关闭该发现 | 关闭发现不等于完成其余实现审查；需完成实际剩余范围，不重开P03 |
| 文档与完成门收尾 | 需求/评估/任务已更新，Beads已记范围撤销 | 三份变更文档Review未勾选；历史Round3结构错误及CEv1新范围同步尚未解决，不能伪造PASS |

### P04/P11 范围决定 — 2026-10-19

用户批准 [P04/P11有界启动验收](requirements.md#p04p11-有界启动验收--2026-10-19)。
两项接受现有持久启动证据，不再以样本不足百次、未测严格p95或首批partial要求补验。
具体数值、输入/构建/环境范围及不可外推限制见需求；原报告和评审历史保持不变。
本决定不是新的产品性能PASS或独立评审；Task2 Review保持未勾选，P02有界结论保留。
CEv1新范围同步仍受上述profile/schema/模板缺口限制，未改写原gate或宣称同步完成。

优先后续：剩余整体代码审查；复用适用证据，不重开P03，不启动P04/P11百次补测。
P01/P02其余范围、真实压力及适用正确性要求继续保留，不由本决定自动豁免。

### 当前评审交接 — 2026-10-19

剩余实现审查已形成新一轮 FAIL，详见
[Task 2 Round 29](reviews/serving-snapshot-and-bootstrap.md#round-29--2026-10-19)。
Task 2 返回实施/修复，Review 保持未勾选；发现决定性阻断后停止扩大审查，
未运行产品测试、构建或性能采样。既有P02有界证据和P03撤销、P04/P11接受决定不变。
其余整体审查、P01/P02剩余范围、压力/适用正确性及三份变更文档评审仍待完成。
CEv1权威协议/schema/模板现已定位并加载，取代上方历史“未定位”前置；
新范围及本轮结论绑定 `scan-serving-r29-bNkQeWSb-state`，保留13项复合准则和历史观察，
整体门禁 FAILED，不表示Task2通过或允许交付。


### R29 修复交接 — 2026-10-19

SP-SB-R29-F1 repaired in candidate，见评审记录R29修复附记。
CLI/App在inventory封口前共享能力选择，首批发布不等待历史扫描终结；两顺序、
App退出保留CLI义务、pending升级和本地封口拒绝均有回归。Go全量24包及race36项通过，
相关vet和macOS双架构构建通过；不重跑原生性能。Task2转in_review，Review仍未勾选。
CEv1目标scan-serving-r29-repair-1aImqWny-state保持NOT_VERIFIED，等待独立复评及剩余门。
不重启P03或P04/P11百次，不进入Tasks3–7或交付。

### R30 有界复评交接 — 2026-10-19

R29修复范围独立复评PASS，详见[Round 30](reviews/serving-snapshot-and-bootstrap.md#round-30--2026-10-19)。
Task2保持in_review、整体Review仍未勾选；剩余整体实现审查及三份变更文档评审尚未完成。
CEv1目标 `scan-serving-r30-xkicsHmW-state` 为NOT_VERIFIED：旧证据需按修复影响显式复用，
首击/启动剩余范围及真实压力/适用正确性仍缺证。本轮仅针对性race复评，无性能采样。
下一前置为剩余整体/文档审查，不重复已完成的有界复评，不重开P03或P04/P11百次要求。

### R31 剩余审查交接 — 2026-10-19

剩余实现审查FAIL，详见[Round 31](reviews/serving-snapshot-and-bootstrap.md#round-31--2026-10-19)。
Task2返回in_progress、整体Review未勾选；R30有界PASS保持。
单一隔离差分实验确认阻断后停止扩大审查，三份变更文档尚未形成各自结论，Documents保持未勾选。
CEv1目标 `scan-serving-r31-LhtOnMbd-state` 门禁FAILED；无新性能采样、产品修改或交付。


### R31 修复进展 — 2026-10-19

空inventory删除对账修复已落地；最终focused race四组合通过，Go全量24包及相关vet/双架构构建通过。
整包race仍有既有首批发布测试的200ms身份读取超时，根因未定；修复验证尚未完成。
详情见评审R31修复进展及持久验证JSON。Task2保持in_progress/Review未勾选，
不发布修复完成或复评指令，不改权限/超时、不继续盲目重跑；先有界诊断该验证阻断。


### R31 修复交接 — 2026-10-19

此前race验证阻断已通过单次plain/race对照归因：插桩身份读取耗尽200ms预算，
不是空inventory删除错误。保留整包race失败限制，不增大产品预算、不重复重跑。
最终定向race四组合、Go全量24包及vet/双架构构建支持本次修复，SP-SB-R31-F1
repaired in candidate，Task2交回in_review，Review仍未勾选。详见评审R31验证阻断归因。
新CEv1目标scan-serving-r31-repair-Y5Xrko-state保持NOT_VERIFIED；等待独立复评及剩余门。


### R31 交接纠正 — 2026-10-19

用户质疑后撤回上节R31修复完成/交回in_review的结论：代码修复已完成，整包race
验证尚未闭环。Task2恢复in_progress、Review未勾选。前述诊断不构成验证豁免，
不发布复评指令；以评审记录“R31完成声明撤回”为当前状态。


### R31 扩展修复最终交接 — 2026-10-19

用户授权身份读取超时一并修复后，request-local只读连接复用已落地，200ms预算和
现有断言保留；generation/inode每次重验。最终三个完整包race（-p1）181项通过，
普通Go全量、相关vet及双架构构建通过，详见R31扩展修复与最终验证。
此次以实际整包通过闭环，不沿用曾撤回的“定向race替代”结论。
SP-SB-R31-F1 repaired in candidate，Task2 in_review、Review未勾选。
CEv1目标scan-serving-r31-reader-final-state保持NOT_VERIFIED，待独立复评及剩余门。

### R32 有界复评交接 — 2026-10-19

R31删除对账及用户纳入的身份读取修复独立复评PASS，详见[Round 32](reviews/serving-snapshot-and-bootstrap.md#round-32--2026-10-19)。
Task2保持in_review、整体Review未勾选。CEv1目标 `scan-serving-r32-S6uUPqb9-state` 为NOT_VERIFIED；
剩余整体/文档审查、适用证据复用及首击/启动/压力缺证仍需闭环，不重评已完成的有界范围。
最终完整包race与全量Go事实保留，本轮新增独立定向race检查；无新原生性能结论或交付。

### R33 整体审查与设计变更收尾 — 2026-10-19

剩余整体代码审查PASS，见[Round 33](reviews/serving-snapshot-and-bootstrap.md#round-33--2026-10-19)。
需求、性能评估及任务文档变更分别通过各自Round4；Documents及Task2 Review据此勾选。
Review标记只表示审查通过：Task2仍in_review，完成门NOT_VERIFIED，尚未达到交付或Task3入口。
后续仅处理适用证据roll-up及首击/启动/真实压力与正确性矩阵的剩余缺证，先复用后补验；
不重复R30/R32或本轮审查，不恢复P03或P04/P11百次要求，无新的原生性能结论。


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
原七份设计文档已通过独立评审及复评，历史结论与证据门保留在上述 reviews 记录。
2026-10-13 起的 requirements/performance-evaluation/tasks 时间目标、测量归属与阶段职责补充
已于 2026-10-18 完成设计复核，见各自 Round 3；其余未变文档不机械重开。
设计通过不等于 E1 或 Task 2 产品验收通过，产品任务仍须各自的授权与验收。

文档dispatch仍为`ad-scan-performance-doc-req`、`ad-scan-performance-doc-arch`、
`ad-scan-performance-doc-perf`、`ad-scan-performance-doc-ux-menubar`、
`ad-scan-performance-doc-ux-cli`、`ad-scan-performance-doc-ux-settings`、`ad-scan-performance-doc-tasks`。
原Bug不因设计或实验被关闭；本矩阵不授权提交、推送、PR、安装或发布。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `baseline-and-open-path` | [x] | [x] |
| 2. `serving-snapshot-and-bootstrap` | [x] | [x] |
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
独立复评 Round 2 为 PASS；历史离线证据门禁及交付检查点见
[baseline-and-open-path 复评](reviews/baseline-and-open-path.md#round-2--2026-10-12)。
Dev/Review 已完成；Task 1 已交付，见 Beads `ad-scan-performance-baseline-and-open-path`。Task 2 当前状态见本任务的实施小节。
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
验收：P01/P02/P04/P11；epoch/state/schema/owner/mode/size/损坏/跨天/DST，旧有效数据保留，
默认quota/全tab/筛选/焦点，slowquota30s与scan>120s；分别报旧值及新值到达时间。
不强求warm路径比当前更快，要求无退化。恢复收益必须真实native对照，cache miss不能漏测。
验证：L3相关desktop/store/CLI/native回归、full Go、race/vet、macOS双架构；
保留量化门的native条件最终≥100次；P04/P11接受上述有界证据，P03已撤销。仅已变化的影响范围重跑。
出口：E1产品验证及`serving-identity`、`atomic-publication`、`open-path-latency`、
`startup-bootstrap`、`wire-compatibility`、`stored-only-coverage`；Task3依赖此出口，不以pilot替代。

### Task 2 当前阶段职责 — 2026-10-14

本节按用户本次 Review 指令机械同步，优先于下方保留的历史进展和交接建议。
开发负责功能实现、必要回归与关键真实场景验证；具备代码检查条件即可交接独立评审。
评审负责独立代码审查及完整 E1、每条件 ≥100 次、性能分布、压力矩阵和适用正确性验证。
完整验证仍属于 Task 2，不转交 Task 7，不改变 Tasks 3–7 的范围或依赖。
最终性能目标、必需准则及完成门不变；未完成 E1/百次不是代码评审入口阻碍。
评审发现产品或测量入口问题即交修复，不在评审中修改产品、测试、配置或扩建工具。

入口核对：HEAD 为 Task 1 交付 `14234ec4c96a3376d1072147eeb6e42b8768381a`；
`scan-serving-state-matrix-20261014-ec6be3271` 所列 275 个输入与交接时全部相同；
macOS 26.7.1、Go 1.27.1、Xcode 26.4/17E192 与回归记录一致。
已检查 serving/bootstrap、coverage、原子发布及 Native 接入实现存在；沿用绑定的基本 Go/native
回归及保留的关键真实点击/启动预检，足以交代码评审，不能据此宣称正式性能通过。
Task 2 Dev交接已核对；用户续授三轮后的首轮有界复评及P02完整恢复固定100通过，整体Review仍未勾选。用户重新授权3轮有效修复候选/独立评审，当前1/3；真实切换候选的面板帧验证仍需修复，整体Task完成门FAILED。R24 P02固定100与适用L3证据继续复用。
完整记录及完成门见 [Task 2 评审](reviews/serving-snapshot-and-bootstrap.md)。
Task 2 整体未完成，不启动后续任务或进入交付。

### Task 2 R11 修复候选 — 2026-10-18

Round 19 有界复评 PASS 保留；Round 24有界复评PASS保留；Round27采集器复评FAIL保留；重新授权的有效修复轮次当前1/3，Round28候选复评FAIL，Task2完成门禁FAILED，整体 Review 保持未勾选。
当前结论与验证边界见 [Round 28](reviews/serving-snapshot-and-bootstrap.md#round-28--2026-10-19)；
Round 17 有界复评 PASS 与 [Round 18](reviews/serving-snapshot-and-bootstrap.md#round-18--2026-10-19) 的历史 FAIL 保留；
Round 15 有界复评 PASS 与 [Round 16](reviews/serving-snapshot-and-bootstrap.md#round-16--2026-10-19) 的历史 FAIL 保留；
Round 13 有界复评 PASS 与 [Round 14](reviews/serving-snapshot-and-bootstrap.md#round-14--2026-10-19) 的历史 FAIL 保留；
[Round 12](reviews/serving-snapshot-and-bootstrap.md#round-12--2026-10-18) 保留复评历史；
[R11 修复附记](reviews/serving-snapshot-and-bootstrap.md#r11-修复附记--2026-10-18) 保留原交接事实。
完整 E1 六条件百次、startup、真实压力、P03/全tab及剩余整体实现审查仍未完成；L3回归按Round19的准确状态复用；
不启动 Tasks 3–7 或交付。

### Task 2 Round 1 修复候选 — 2026-10-14

SP-SB-R1-F1 已在候选中修复，待独立复评；Round 1 的 FAIL 和原始证据保留。
Native publication 安装先比较 state identity、core/session epoch 和 schema，身份变化时
先清除旧 header、快照及可见 previous，再检查日期和 full/partial 优先级。
当天有效的新 partial 可接替旧 full；新身份日期不可用时保持 unavailable，后续失败不能
恢复失效数字。同身份仅 revision 推进时仍保留有效的完整旧视图。

新增五项原生回归覆盖 core epoch、session epoch、state identity、core schema 变化及
新身份日期不可用，均检查失败前后 header/payload/state；既有保留用例增加同身份 revision 变化。
`bash scripts/test-macos-app.sh` 成功：Shared 98、App 143、Widget 51，共 292 项，
1 项既有 opt-in schema matrix 跳过，0 失败；coordinator 31 项全部通过。
相对 Round 1 的 275 个输入，仅本次两个 Swift 文件变化；未变 Go、工具及构建输入复用原证据。
修复状态和 gate 见 [Task 2 评审记录](reviews/serving-snapshot-and-bootstrap.md) 的修复附记。
交接同一 Beads task 为 `in_review`；不自判 Review PASS，不勾选 Review，不进入交付。
完整 E1、每条件 ≥100、性能分布、压力及剩余独立审查仍属于 Task 2。

### Task 2 Round 2 复评 — 2026-10-14

独立复评 FAIL；原问题已关闭，本轮新增阻断问题和证据详见
[Task 2 评审记录 Round 2](reviews/serving-snapshot-and-bootstrap.md#round-2--2026-10-14)。
Dev 保持就绪，Review 未勾选；同一任务返回 `in_progress` 等待修复。
Task 2 完成门为 FAILED，完整 E1 和其余验收边界仍开放；不进入交付或 Tasks 3–7。

### Task 2 Round 2 修复候选 — 2026-10-14

SP-SB-R2-F1 已在候选中修复，待独立复评；Round 2 FAIL 及原始证据保留。
正常 bootstrap 返回后若安装层没有可用快照，明确进入 `.failed` / `.degraded(previous: nil)`
及 unavailable 状态并清理进度；不恢复失效身份数据。原身份失效和同身份 full 优先规则保留。
新增两项 Native 回归分别覆盖无最终 publication callback 的正常返回，以及 callback 加挂起 quota；
等待完整作业结束后检查失败终态、空 header/payload/progress、终结回调恰好一次，并保留原抛错用例。
本次原生测试 Shared 100、App 143、Widget 51，共 294 项，1 项既有跳过、0 失败；
coordinator 33 项通过。未变 Go、工具、依赖及构建配置按原范围复用证据。
修复证据和当前 gate 见 [Task 2 评审记录](reviews/serving-snapshot-and-bootstrap.md) 的 Round 2 修复附记。
同一任务交接 `in_review`，Review 未勾选；完整 E1、性能/压力及剩余独立审查仍属于 Task 2，
不进入交付或后续任务。

### Task 2 Round 3 复评 — 2026-10-14

历史问题复评 PASS，处置及边界见
[Task 2 评审记录 Round 3](reviews/serving-snapshot-and-bootstrap.md#round-3--2026-10-14)。
Review 勾选表示本轮 finding 复评通过，不表示完整 Task 2 验收完成。
完成门仍为 NOT_VERIFIED：完整 E1 的 open-path-latency、startup-bootstrap、
e1-native-performance，以及前两轮因阻断问题停止的剩余独立审查仍待补齐。
同一任务保持 `in_review`；门禁 VERIFIED 前不进入交付，不启动 Tasks 3–7。

### Task 2 验收续接审查 — 2026-10-14

Round 3 的历史问题复评 PASS 保留。剩余验收工具审查发现新的阻断问题，
见 [Task 2 评审记录 Round 4](reviews/serving-snapshot-and-bootstrap.md#round-4--2026-10-14)。
最新整体审查为 FAIL，Review 取消勾选，同一任务交回 `in_progress`；完成门 FAILED。
在修复验收入口前停止扩大验证，尚未启动本轮原生采样；操作员无需点击。
剩余整体审查、设计补充完整复核和六条件每条件 ≥100 验收仍待完成，不重复 Round 3 已关闭问题。

### Task 2 Round 4 修复候选 — 2026-10-15

SP-SB-R4-F1 已在候选中修复，待独立复评；Round 3 PASS、R1/R2 CLOSED 及 Round 4 FAIL 保留。
rebuilding 压力改由私有导出的 A/B helper 在真实 maintenance 重建中记录：请求/执行身份、
锁排队、准备、处理开始/结束及 CLI wall 分开；首击保存同钟时间和当时的 processing 标本。
仅实际处理 >120s、首击在该处理区间且身份匹配时入组；run_matrix 和 summarize 均检查准入。
无 worker 证据、仅排队超时、首击不重叠或旧 pending 标记均不能补足压力条件。
私有 Go 插桩只存在于导出副本，生产源码未改。27 项 matrix 加 19 项入口回归通过；
A/B 单源合成 fixture 的真实短重建均被排除，时钟对照及最终 A/B 私有 Debug App 构建通过。
首击 processing 标本使用独立的可编码字段，保留原 nativeEnvironment 的 Bool 契约。
未变 Go/Native 294 项等原证据按范围复用；修复证据与当前 gate 见
[Task 2 评审记录](reviews/serving-snapshot-and-bootstrap.md) 的 Round 4 修复附记。
同一任务交接 `in_review`，Review 未勾选；剩余整体/设计审查和完整 E1/百次/压力验收仍开放，
不进入交付或后续任务。

### Task 2 Round 5 复评 — 2026-10-15

SP-SB-R4-F1 的有界修复已核对关闭；继续实际入口验证后，本轮整体复评 FAIL，
见 [Task 2 评审记录 Round 5](reviews/serving-snapshot-and-bootstrap.md#round-5--2026-10-15)。
2026-10-15 用户明确重申：剩余整体独立审查、设计补充复核及完整 E1 六条件每条件 ≥100、
压力和正确性验收由评审者完成；不能用 finding 关闭代替 Task 2 评审通过。
本轮过早勾选 Review 已撤回；新阻断问题交回同一任务 `in_progress` 修复。
Round 3 历史 PASS 保留为当时的有界问题结论，不代表整个 Task 2 通过。
中间证据 scan-serving-rereview-r5-XHUHsB6b-* 保留审计历史，其状态段已被本次纠正取代，
不得用其中的 Round5 PASS 表述宣称本轮通过；最终证据和 FAILED 门禁见 Round 5。

### Task 2 修复交接 — 2026-10-15

本次测量入口修复候选已就绪，交接独立复评；当前 Review 仍未勾选。
真实导出 A/B helper 在实际 matrix 环境中均生成匹配请求与 PID 的处理 trace；
28 项 matrix 回归和 19 项入口回归通过。范围、验证与证据门见
[Task 2 评审记录](reviews/serving-snapshot-and-bootstrap.md) 的 Round 5 修复附记。
剩余整体审查、设计补充复核及完整 E1 六条件每条件 ≥100、压力和正确性验收仍由评审者完成。

### Task 2 Round 6 复评 — 2026-10-15

最新复评 FAIL，见 [Task 2 评审记录 Round 6](reviews/serving-snapshot-and-bootstrap.md#round-6--2026-10-15)。
路径别名修复已核对；继续实际验收入口检查发现新的工具集成阻断，同一任务返回 `in_progress`。
Review 保持未勾选，完整 E1/百次及剩余审查仍由评审者在修复后继续，不以 finding 关闭代替通过。
历史 Round 3 PASS 保留；没有新增物理点击样本，不交付、不启动 Tasks 3–7。

### Task 2 复用入口修复交接 — 2026-10-15

复用入口的修复候选已就绪，同一任务交接独立复评，Review 保持未勾选。
23 项入口测试及 27 项 matrix 契约测试通过；实际复用函数已覆盖 A/B 正例和漂移拒绝。
验证范围与当前证据门见 [Task 2 评审记录](reviews/serving-snapshot-and-bootstrap.md) 的 Round 6 修复附记。
完整 E1/百次、压力、正确性及剩余整体/设计补充审查仍由评审者继续。

### Task 2 Round 7 复评 — 2026-10-15

自动 warm 首击已完成 A/B 各100个有效样本；最新复评 FAIL，完成门 FAILED。
完整结果及边界见 [Task 2 评审记录 Round 7](reviews/serving-snapshot-and-bootstrap.md#round-7--2026-10-15)
和 [自动百次证据](evidence/serving-warm-auto100-20261015-r7.json)。
Review 保持未勾选，同一任务交回 `in_progress`。已记录问题的修复关闭不代表任务通过。
因 warm 验收出现决定性阻断，未扩大其余五条件的百次采样；完整 E1、剩余整体及设计补充审查仍开放。
没有产品修复、Git交付或 Tasks 3–7。历史 Round 3 的有界 PASS 保留。

### Task 2 首击修复候选交接 — 2026-10-16

当前候选已完成同起点 warm 百次自动首击复验及原生回归，交接独立复评，Review 保持未勾选。
完整修改、诊断、测量口径、结果和当前证据门见
[Task 2 评审记录](reviews/serving-snapshot-and-bootstrap.md) 的 Round 7 修复完成附记。
其余五条件、真实重建压力、启动、完整正确性及剩余整体/设计补充审查仍由评审者继续；
本次交接不表示 Task 2 已通过或允许交付，不启动 Tasks 3–7。

### Task 2 Round 9 复评 — 2026-10-17

最新复评 FAIL，完成门 FAILED；完整结论和有界证据见
[Task 2 评审记录 Round 9](reviews/serving-snapshot-and-bootstrap.md#round-9--2026-10-17)。
Review 保持未勾选，同一任务返回 `in_progress` 并释放评审者。
本轮未修改产品、测试或配置；发现确定阻断后停止扩大验收。
完整 E1、六条件百次、启动、真实压力及剩余整体审查仍属于 Task 2；不启动 Tasks 3–7。

R9 修复候选增加可观察日期失效、独立日界定时器及日历日/系统时钟/时区/唤醒通知，
继续复用统一 today guard。日期事件不启动扫描，不依赖 full refresh 或 quota 开关。
原生回归检查未交互的实际菜单栏标题及挂载表面更新；候选证据与验证见评审记录 R9 修复附记。
Review 保持未勾选，待独立复评。

### Task 2 Round 10 复评 — 2026-10-17

既有 findings 的本轮复评 PASS，完整 Task 2 完成门 BLOCKED，任务保持 `in_review`。
Review 勾选只对应最新 finding-disposition 结论，不表示 E1 或 Task 2 完成。
完整范围、当前内容身份、原生采样及外部输入前置见
[Task 2 评审记录 Round 10](reviews/serving-snapshot-and-bootstrap.md#round-10--2026-10-17)。
下一步先确保隔离 TEST 图标无遮挡且 AX 命中 PID 与测试 App 一致，再继续本任务 E1：
完整六条件百次、启动/真实压力及剩余整体和设计补充审查。不得绕过输入目标保护。
门禁 VERIFIED 前不进入提交边界、不启动 Tasks 3–7；无 Git 交付。

2026-10-17：证据权威已切换到 Neo4j/CEv1。本次仅清理规则和历史本地引用；
当前治理同步目标为 `scan-serving-neo4j-authority-20261017-state`，
WorkUnit 为 `scan-performance:serving-snapshot-and-bootstrap`。原 R10 PASS 与验收阻碍保持。

以下为历史实施记录，历史 Dev/交接判断保留，当前阶段边界以上述批准为准。

### Task 2 实施中 — 2026-10-12

工作区 `agent-deck.scan-performance` / `feature/scan-performance`，起点
`14234ec4c96a3376d1072147eeb6e42b8768381a`。Beads
`ad-scan-performance-serving-snapshot-and-bootstrap` 为 `in_progress`；没有独立实现评审或交付授权。

- 新增独立 serving generation、原子快照文件、只读身份检查和当天投影；core schema 31
  只增加协调/coverage 数据，不替换已有 facts 或 parser。
- 全 inventory 先持久化，再按日期/mtime 提示分批；首批提交后发布 partial，不等待历史终结。
  来源提示不构成覆盖证明，未完成/旧库/已知待更新由持久 coverage 和稳定 warnings 表达。
- 私有 worker protocol 3、独立 bootstrap stream 2 与外部 scan NDJSON v1 分开；App 最后一个
  owner 断开后暂停 App-only 工作，已接受的 CLI 义务不被取消。
- Native 启动恢复、quota 与 bootstrap 并行；后台 partial 不覆盖仍有效的完整旧视图。
  未准备项目数显示 `—`，跨天 today 不沿用昨日数字。点击仍只读取内存。
- 针对性 store/desktop/scanruntime/CLI 回归通过。原生 XCTest 281 项执行，1 项跳过，0 失败；
  这不是 E1 首帧/尾延迟 PASS。首轮全 Go 回归暴露新增 hidden command help、旧 coverage 断言、
  network import 边界及 schema 30 golden 需要适配；这些定位后的修正及 internal-runtime annotation
  的定向回归已通过。后续 no-op 修复后的全 Go、相关 race、vet、darwin arm64/amd64 build
  均已通过；续接时对照候选记录的 175 个 Go/构建输入没有变化，因此复用这些结果。
- 用户已明确批准只读复制三个 JSONL 根及两个 SQLite 库用于私有原生 A/B；不读取认证文件、
  不改真实数据、不发送模型请求。E1 harness 目前先做冻结输入和 no-cache pilot；其余条件、
  slow-quota/scan>120s、各条件 ≥100 与正式性能证据门仍未完成，Dev/Review 均不勾选。

首轮实际 E1 输入为 5,372 文件 / 3,477,971,661 bytes，corpus SHA256
`5cb6d2dbfd2936a4552b158e92b77672ed992966041bda0774c8d44d118c5117`；原观察保存在
[no-cache pilot](evidence/serving-snapshot-bootstrap-pilot.json)。A 的可用数据帧 7.708s，初次点击未路由、
使用了明确记录的 ready click；不能当作首次 click 成功。B 在 150s 内没有可用数字帧，不能填 0
或放入成功分布。该冻结输入当天没有事件；旧库没有 coverage，candidate 不能伪造当天 zero。
检查发现分批 session no-op 仍使用全 source-set 证明，落入重复 FTS 物化，coverage 只推进到部分来源。
已增加 subset 证明、成批 unchanged coverage 提交和 metadata-only full no-op 快路径；相关定向回归通过。
替换候选的独立新 pilot 正在执行，不能重标首轮结果为修复后证据。

后续 [R2 no-cache pilot](evidence/serving-snapshot-bootstrap-pilot-r2.json) 的 3 对 / 6 样本都有有效 bitmap，
候选进程入口到帧中位 2.985s、warm 中位 431.6ms；三次 candidate 初始点击均未路由，不能当作
首次 click 或 P04 成功。按 E1 的前台优先规则，新增 hosting hierarchy 复用而未增加 worker。
[R3 pilot](evidence/serving-snapshot-bootstrap-pilot-r3.json) 仍只有 3 对：candidate warm 中位 219.1ms、
最大 642.7ms，仍不足以通过 P01 或 ≥100 条件门；初始 click 仍未路由，必须另处置测量入口。
schema-ahead fixture 的旧 supported-count 30 断言已按 schema31 标本适配并复验通过。
续接进一步定位每次 presentation/当天窗口/展示日期访问重复创建 ICU 日期解析器的成本；
隔离的 1,000 次解析为新建 formatter 632.3ms、复用 formatter 144.8ms、value-style 2.1ms。
该微基准只支持移除重复解析器构造，不作原生首屏收益。新增无共享可变缓存的日期解析器，
保留超长 fractional 的旧兼容路径；精确年龄边界、offset、fraction 与 DST 回归保留。
最终原生 XCTest 283 项执行、1 项跳过、0 失败；未改变 Go，因此不重复已相容的 Go 验证。

新 [R4 激活对照](evidence/serving-snapshot-bootstrap-pilot-r4.json) 复用同一冻结输入，在 A 的
`native_activation_unavailable` 前置检查失败，B 没有执行，无有效数据帧，未补出任何新延迟。
该控制在 App 激活后才注入点击，不是 inactive 用户首次物理点击；R4 的 `initialClickRouted`
保留旧默认值，失败发生在注入之前，不能把该布尔值当作路由观察。后续 harness 已改为
只有实际原始 click 已路由才置 true，同时只为未重试首次 click 填单独耗时。
harness 拒绝覆盖旧输出，失败退出非零，并绑定实际导出源码、App/helper、工具链和
每个 A/B 样本的起始库身份。最新日期优化的 native 首帧收益仍未测得。
实施仍未完成，不交接 Review。

续接的 [R5 窗口直送对照](evidence/serving-snapshot-bootstrap-pilot-r5.json) 在 A 超时，
没有 native 结果或有效帧；同步直送 mouse-down 后才直送 mouse-up 的方法未取得有效观察，
不据此宣称产品变慢或变快。该轮执行中 harness 文件有后续编辑，未冻结的脚本哈希也不用于
精确性能证据。已按实际执行 App 的二进制哈希、唯一 bundle ID 和 executable 路径定位并停止
本轮遗留 App；没有终止其他 App。后续窗口控制改为先排入匹配的 mouse-up，再同步分派
mouse-down，避免后者跟踪循环等待尚未分派的 mouse-up。

[R6 WindowServer 对照](evidence/serving-snapshot-bootstrap-pilot-r6.json) 在首次注入前确认：
A 的 status button 已有可见 window，但 `firstEventPostingAllowed=false`、
`firstActionReached=false`、`initialClickRouted=false`；未重试，无有效帧，B 未执行。
CLI 预检的允许状态不能代表独立私有 App；不能把本轮记作事件已送出或首击延迟失败。
真实全局输入需操作员为明确的测试 App 提供权限，当前未自动请求或修改 macOS 权限。
E1 harness 默认丢失首击即失败；ready-click 只允许作为明确选择的独立诊断。
启动时冻结执行脚本哈希，观察期脚本变化即拒绝；绑定 App/helper/shared framework/widget
和源码，失败后校验精确私有 App 已停止。六项无真实数据的 harness 回归及六种私有 Swift
源码变体语法检查通过。Go、Swift 产品和构建输入共 228 项与上一验证状态相同，复用原
full Go/race/vet/双架构和 283 项 native 回归；这些不是新性能或真实首击证据。

以上为实施进展，不是 Review PASS、性能保证或 Task 3 的入口证据。

### Task 2 人工测量续接 — 2026-10-13

用户明确在当前会话继续，不恢复故障会话；只续接本 Task，不启动 Tasks 3–7。
已批准 P01/P02/P04/P11 时间门槛统一原值 ×1.5，具体数值由 requirements 维护；
其他性能指标、正确性/coverage/旧值保护和最终样本要求不变。
隔离 harness 增加人工等待真实鼠标 action 的入口及仅准备 App 的边界；测试菜单栏
标为 `TEST A` / `TEST B`，不修改产品源码的正常点击行为、不请求或修改系统权限。
人工等待与 action-to-bitmap 耗时分开，首次点击数据是否已就绪单列；含人工等待的
launch-to-frame 不用于启动预算。先 5–10 次预检，不能代替正式 E1 或尾延迟门。
人工入口及单臂续接的 17 项探针回归通过；此前 A/B Swift 语法及私有 Debug 构建通过，
此次私有 Swift 探针未变，已复核精确探针和
App/helper/framework/widget 身份。对照既有 L3 状态的 228 项产品/测试/构建输入没有变化，
原 full Go/race/vet/双架构及 283 项 native 结果按未变范围复用，不重新宣称性能通过。
首轮因操作员未见测试图标而在 A 首击等待超时，0 有效帧，不算产品性能失败。
操作员确认 A 可见后的
[人工 R2](evidence/serving-snapshot-bootstrap-manual-pilot-r2-20261013.json) 中，
A 收到 6 次真实按钮 action，6 个数值 bitmap 均有效；首次 action-to-frame 859.469ms，
五个已准备面板的 warm 570.103–672.771ms（中位 632.115ms）。这是旧版基线的小样本，
不是候选性能或 p95/p99；首次点击前数据已就绪，launch 含人工等待，不能作为启动预算。
B 等待首次点击超时、0 有效帧；操作员明确未见 B，不能记作候选慢或补零。
操作员确认 B 可见后，使用与 R2 精确一致的原起始数据库和原 App/probe，仅补
[B 单臂观察](evidence/serving-snapshot-bootstrap-manual-b-resume-20261013.json)，没有重做 A。
B 收到 6 次真实按钮 action、6 个数值 bitmap 均有效；首次 action-to-frame 698.327ms，
五个已准备面板 warm 70.109–110.242ms（中位 97.714ms）。已直接核对冻结输入、
原始 App/probe 身份、点击控制、环境及两份起始数据库均与 A 一致；续接的 host
脚本哈希变化保留在各自观察里，不改写 R2。五个不同面板的描述性 warm 中位耗时减少
84.542%，这不是每面板重复采样的尾延迟结论。
B 首击前数据同样已准备，launch 含人工等待，不能用它通过 P02/P04/P11。
首击仍明显慢于后续打开：B 的首个 popoverOpen 区间 526.244ms，后续 10.800–45.151ms；
该区间覆盖 show/activation，不是 CPU 采样，也尚未细分到具体 SwiftUI/AppKit 子调用。
下一处需要定位的是首次展示路径，不把剩余耗时猜成扫描、OCR 或某个未测子调用。
正式 E1、完整矩阵与本次设计补充复核保持未完成。
Dev/Review 不勾选，不交接实现评审。

#### 首次展示归因与预布局候选

[正序诊断](evidence/serving-snapshot-bootstrap-first-ui-attribution-20261013.json) 和
[反序对照](evidence/serving-snapshot-bootstrap-first-ui-attribution-reverse-20261013.json)
复用同一私有诊断 App/probe 与原 R2 精确起始数据库，数据已准备后直接调用原 toggle 路径。
四次运行的 24 个数值 bitmap 均有效；没有新增物理点击，不重建反序对照，不改写人工结果。
所有首击前 view 都已 loaded；配置小于 0.12ms，activation 4.652–5.673ms。
没有预布局时首个 show 302.350–339.080ms；先布局/显示现有 view 后 show 110.005–123.909ms，
但准备本身需要 210.952–238.652ms。准备后的首次显示分段为 260.051–286.440ms，
包含准备的总分段为 471.003–525.092ms：这是把工作移到点击前，不是整体启动加速。
两种顺序结果一致，支持处理首次布局/初始显示准备时机，而非猜测首次 loadView 或 activation 主导。

候选在初始化已 ready 或首份 dataSurface 到达后，只准备一次已有 hosting view；
保留同一控制器/控件身份和屏幕几何逻辑，不提前 show 弹窗、不夺焦点、不启动 helper，
点击仍只展示已有内存视图。新增固定 `popoverPreparation` 阶段单独记录真实准备成本。
对应原生准备时机/幂等/焦点/可见窗口/不刷新和计量隐私回归已补并通过。
2026-10-14 `make test-macos-app`：Shared 93、App 143、Widget 51，合计 287 执行，
1 项既有 skip，0 失败；不是旧 283 项结果的重标记。
17 项私有探针回归通过；Go 未改，不重复 Go 广域验证。此前未变产品的 283 项 native
结果不能替代此次改变过的 Controller/Shared 计量回归，也不能复用旧候选二进制作为新实现。
新候选首击、启动预算和正式 E1 仍未验证。
全新隔离 TEST C 已按改变后的 source/probe/App/helper/framework/widget 身份准备，
仅请求一次首次物理点击，不重做已有 A/B 面板测量。已跨到 2026-10-14，旧 10-13
观察保持其原始日期/窗口，不将跨日新结果冒充正式同条件加速比；准备时长继续单列。
首个 C 因操作员未见重启后的图标而等待超时，0 点击/0 有效帧，原观察和原 C
App/probe 已保留。私有探针改为同一进程跨可见性确认，独立 600s 可见性等待及
确认后的 120s 操作等待；产品/helper/数据准备超时及实际点击起点不变。
[持久 C 首击](evidence/serving-snapshot-bootstrap-ui-visible-first-click-r2-20261014.json)
成功收到 1 次真实 action、1 个有效数值 bitmap：action-to-frame 331.664ms，
独立 `popoverPreparation` 194.399ms，`popoverOpen` 154.591ms。点击前数据已就绪，
23.763s 操作等待没有进入首击耗时；含人工等待的 launch 不可判定启动预算，精确私有 C 已停止。
这是新候选单次首击，仍略高于 300ms，不能从 1 个样本声称 p95/p99 达标，也不将
跨日旧 B 的 698.327ms 重标为本次严格 A/B 基线。正式每条件 ≥100、完整启动/压力矩阵、
实现独立评审和设计补充复核仍未完成，Task 2 保持实施中。

#### E1 验收工具收尾 — 2026-10-14

用户批准在本会话执行有界收尾，不恢复旧会话、不启动 Tasks 3–7、不改变权限或安装。
产品 Go/Swift 候选保持原样；本轮改变隔离 harness、其回归和测量方法说明。
已有 302 个观察源码输入在续接核对时全等，旧 287 项 native / 未变 Go 范围保留原身份，
不把新探针或模型计时重标成既有原生性能结果。

新增六条件调度、请求绑定的同进程可见性等待、独立模型准备时钟、首次点击单采样，
并区分有效缓存、当前 schema 已提交库无缓存、额外旧库升级诊断与空库。
19 项原入口回归及 20 项新矩阵回归通过；初版六项 Swift 语法检查及私有 A/B
完整构建通过。这是工具验证，不是完整原生矩阵 PASS。
[模型预检](evidence/serving-matrix-model-pilot-20261014.json) 已完成 15 次、0 模型失败：
有保存文件条件的模型中位 509.6ms（缓存命中未证实），受控当天输入的空库首批 1,675.5ms；旧库无缓存组
2,902.7ms 包含 schema30→31 升级，保留原标签并更正适用边界，不作为普通 P04。
缓存 fixture 进一步改为最后一次 state 复制后的真实原位生成/同 reader 核验，并统一
私有 state 根拼写，避免 inode/路径别名导致隐式 fallback。原观察不重标为修正后的命中。
[提前展示控制](evidence/serving-matrix-startup-controls-20261014.json) 保留当前库无缓存
一个有效数值帧（App-ready 1,191.6ms）及 restore 的 popover_invisible；第一批缓存路径
URL API 仍返回 /tmp 的问题另以私有前缀对齐处理。同二进制 on/off 诊断的 shown 差异
在 activation 前已发生，未证实其根因，不把控制方案变化说成产品修复。
[就绪后展示预检](evidence/serving-matrix-ready-startup-pilot-20261014.json) 完成三条件各五次，
15 个有效数值帧、0 失败。当前 schema31 无缓存 App-ready→帧中位 923.5ms/max989.1ms；
真实原位 cache 恢复的模型中位 1,024.7ms/max1,080.9ms（单列帧中位1,912.4ms）；
空库受控当天首批帧中位2,543.4ms/max2,608.9ms。所有起点均不含人工等待，
真实 cache reader 与 worker 拼写匹配；这不是 P01 的 icon 或 p95/p99/≥100 通过。
人工入口进一步拒绝首次等待请求之前的任何未计时输入，不将后续点击当 original first。
压力归属、
各条件 ≥100、P03/全表面正确性、三份设计补充复核与独立实现评审仍保留为出口要求。
详细方法及输入/终点限制由 [性能评估](performance-evaluation.md#可复现-e1-条件调度与独立时钟--2026-10-14) 维护。
本轮 历史离线目标为 `scan-serving-state-matrix-20261014-ec6be3271`，请求
`scan-serving-gate-matrix-20261014-ec6be3271`；技术机制按已核对的未变回归和新工具验证
分别归属，pilot 不填为正式 E1、startup/open-path 性能或独立 Review 的通过证据。
下一项是集中人工 warm/首击和压力预检，然后再决定正式采样；Dev/Review 均不勾选。

### Task 2 Round 8 复评 — 2026-10-16

独立复评 FAIL，Review 保持未勾选，返回同一 Task 2 修复。
本轮结论、逐项 disposition、准确内容身份、门禁及采样停止边界见
[Task 2 评审记录 Round 8](reviews/serving-snapshot-and-bootstrap.md#round-8--2026-10-16)。
完整 E1、剩余条件百次与正确性验收仍属于 Task 2；不启动 Tasks 3–7 或进入交付。

R8 修复候选：SP-SB-R8-F1 统一 today 日期可用性，覆盖客户端金额、全部日期面板、
会话/工作信号与菜单栏 cost/tokens；历史窗口保留原值。新增同一 publication 跨午夜的
多表面回归。验证与候选身份见评审记录的 R8 修复附记；Review 保持未勾选，待独立复评。

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
