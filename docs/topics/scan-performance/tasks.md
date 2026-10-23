---
status: active
created: 2026-10-09
updated: 2026-10-22
---

# Scan performance：设计集与任务矩阵

工作区 `agent-deck.scan-performance` / 分支 `feature/scan-performance`。
2026-10-22 用户决定全 topic 重新设计：验收指标恢复为 `c376984` 的 P01–P11 原值，撤回此后全部
豁免、撤销与按修订条件接受；先建立可自动复现的测量契约，再在现有代码上重新实现，
无用代码与脚本在实现中删除；证据只进入 Neo4j CEv1 与评审记录。

此前的任务矩阵、实施交接与验收决定保留在 Git 历史（提交 `c6ea1ca` 的本文件）与
`reviews/` 下各评审记录中，作为历史与设计输入，不构成本矩阵任何任务的完成依据。
旧任务的 PASS 不代表按原标准完成。

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

Settings 含用户要求的 OTel 开关；Widget 不改表面，保留既有兼容性。

## 重新设计的评审范围

| 顺序 | 文档 | 本次变化与评审重点 |
| --- | --- | --- |
| 1 | requirements.md | P01–P11 恢复原值、删除全部豁免/撤销段；已测未达标事实作为输入；测量能力要求；不可达时返回设计而非豁免 |
| 2 | performance-evaluation.md | 测量契约：产品内起止打点、单一时钟、自动刺激、固定种子夹具、环境与机器、样本与统计、无效样本处理；逐项可测性判定；候选采用规则与决策门 |
| 3 | architecture.md | 打开路径（首击分段与 NSPopover 下限探测）、启动恢复优先、usage 并行 reduce + 单一有序写者、短事务、测量打点接口；其余契约不变 |
| 4 | tasks.md | 本矩阵：首项为测量工具、基线按新契约重测；文件边界、删除项、验证级别、准则与依赖 |

UX 文档 `ux/menubar.md`、`ux/cli.md`、`ux/settings.md` 本次无行为、文案、状态或标本变化，
沿用原评审结论（见 `reviews/ux-menubar.md`、`reviews/ux-cli.md`、`reviews/ux-settings.md`）。
测量打点与驱动不新增用户可见表面；首帧内容延后构建不改变可见内容与交互。
若 `serving-open-path` 的 NSPopover 下限探测触发返回设计，且用户选择替代窗口形态，这属于 UX 变更，
届时清除 `ux/menubar.md` 的 Review 并按用户决定重新设计与评审。
`prototype/tools/scan-performance-*` 是 `ux/prototype/serving/manifest.json` 固定的标本 harness，
任何任务都不删除或修改它们，因此 UX 标本身份与上述评审保持有效。

评审按 1→2→3→4 依次进行，每份文档各自的记录、round 与内容身份；全部通过才进入实现。
文档 dispatch 为 `ad-scan-performance-doc-req`、`ad-scan-performance-doc-perf`、
`ad-scan-performance-doc-arch`、`ad-scan-performance-doc-tasks`。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `measurement-harness` | [ ] | [ ] |
| 2. `baseline-remeasure` | [ ] | [ ] |
| 3. `serving-open-path` | [ ] | [ ] |
| 4. `parallel-source-pipeline` | [ ] | [ ] |
| 5. `incremental-update` | [ ] | [ ] |
| 6. `event-updates-and-cli` | [ ] | [ ] |
| 7. `otel-capture-and-reconciliation` | [ ] | [ ] |
| 8. `cross-surface-acceptance` | [ ] | [ ] |

依赖：1 → 2 → 3、4；4 的前台准则在 3 通过后验证；3、4 → 5 → 6 → 7 → 8。
额外常驻 Go 引擎与多进程 parser 不是预建任务，只在性能评估的候选采用规则满足后经设计增补选入。

每个任务的 CEv1 WorkUnit 为 `scan-performance:<anchor>`，required criteria 为各节“准则”所列；
创建前按具体 ID 查询存在性，不复用旧任务的 WorkUnit。每个任务都需独立实现评审与精确
content-state 门禁 `VERIFIED`；测量类准则的证据按性能评估·记录与证据写入 CEv1。

通用约束（每个任务适用）：

- 在现有代码上修改；任务内确认不再使用的代码、测试与脚本随任务删除，删除前以引用搜索确认。
- 性能门只用测量契约规定的方法、样本数与统计；正式批次失败即任务未完成，不能改口径或豁免。
  若证明目标在既定约束下不可达，停止并返回设计。
- 测试与测量只用临时 HOME/隔离 state root 与生成夹具，不读取真实日志或生产数据库。
- 提交、推送、PR、安装、发布不在任务授权内。

## 1. measurement-harness

目标：实现测量契约的全部产品侧打点与工具，使 P01–P11 均可自动、可复现地测量（E0）。

文件：

- 新增 `apps/macos/AgentDeckShared/PerfTrace.swift`、数据帧探针（视图修饰器、提交后观察者、display link）、
  `apps/macos/AgentDeckApp/PerfDriver.swift`（`#if AGENTDECK_PERF`）与 Xcode `Perf` 构建配置；
  在 `MenuBarItemController`、`MenuBarViewModel`、`MenuBarSurfaceView`/`MenuBarPanelViews`、
  `AgentDeckApp`、`EmbeddedHelperRunner`、`DesktopRefreshScheduler` 接入打点。
- 新增 `internal/perftrace`；在 `internal/store` 写事务封装、`internal/ingest`、`internal/usage`、
  `internal/session`、`internal/scanruntime`、`internal/desktop` serving 写入、`cmd/agentdeck` 接入打点与进程自报资源。
- 新增 `cmd/agentdeck-perf`：`fixture`（`rep`/`small` profile、`append`）、`template`、`run`、`stats`、`gate`。
- 发布打包与 Release 配置保持只构建 `./cmd/agentdeck`，不含驱动。

删除：`DesktopPerformanceStage`/`DesktopPerformanceTrace` 及其测试、`MenuBarItemController` 的 DEBUG 基线采集
（`captureNativeBaseline`、`baselineClick*`、Vision/OCR）及 `AgentDeckApp` 中的入口、
`AGENTDECK_SCAN_PROFILE` 摘要路径（`scanruntime` profile 与其测试改为 `perftrace`）、
`scripts/scan-performance-*`、`scripts/scan_performance_*`、`scripts/p07_rss_support*`、
`scripts/p07measure/`、`scripts/p07-overlay/`。不删除 `prototype/tools/scan-performance-*`
（UX 标本，见上文“重新设计的评审范围”）；删除前的引用搜索把它们与 `prototype/README.md` 中的标本说明排除在外。

验证（L3）：Go 全量 `scripts/run-go-test.sh ./...`、`perftrace`/store 封装 race、vet、
macOS 双架构构建（Debug/Release/Perf 三配置）、native 全量 XCTest；时钟同源测试（按[性能评估·原则](performance-evaluation.md#原则)第 2 条核对四个来源：Swift
`clock_gettime_nsec_np`、Go `unix.ClockGettime`、`NSEvent.timestamp`、`ri_proc_start_abstime`，偏差 ≤1 ms）；
记录隐私测试（输出不含路径、会话 ID、正文）；关闭时零记录测试；夹具确定性测试（同参数两次生成身份相同）；
Release 产物不含 `PerfDriver` 符号。E0：按[性能评估的工具 pilot 表](performance-evaluation.md#样本与统计)
运行其中标为 `measurement-harness` 的条件，工具无效为 0；P05 的 pilot 归 `otel-capture-and-reconciliation`。
若点击刺激出现未路由样本，按性能评估在本任务评审中确认替代刺激。

准则：`clock-source`、`marker-coverage`、`driver-isolation`、`fixture-determinism`、`measurement-privacy`、
`pilot-validity`、`legacy-harness-removal`。

## 2. baseline-remeasure

目标：按新契约测得起点基线并定位主导成本（E1），作为后续任务的比较对象与方案选择依据。

基线：B0 = 提交 `14234ec`（旧 `baseline-and-open-path` 任务的原交付，主题产品行为改动之前）的构建，
只用于 P09；P08 是绝对门，不与 B0 比较。S0 = 本任务开始时的内容状态（含 `measurement-harness` 打点与
旧 `bounded-source-pipeline` 任务保留的候选），用于 P01–P04、P06 基线臂、P07、P08、P09、P10、P11。两者均在同一机器、同一 `rep` 夹具与模板上测量。

范围：按性能评估的正式样本数跑完整批次（P09 为 B0/S0 20 对 ABBA 交错）；给出分段：P01 首击超出部分所在段、
P02 启动期争用来源、P07 事务按 label 的 top 分布与 RSS 上界构成、P09 关键路径（usage/session/derive）、
P10 各段；更新来源与版本影响图。不改变产品行为；本任务只允许修正 `measurement-harness` 的工具缺陷，
修正后受影响批次整批重测。

验证（L1）：工具回归；基线批次本身即证据。

准则：`baseline-identity`、`baseline-matrix`、`cost-attribution`。

## 3. serving-open-path

目标：P01（首击、重复点击、重建中三层）、P02、P03、P04、P11 达标（E2），并保持 serving 正确性契约。

文件：`MenuBarItemController`、`MenuBarSurfaceView`、`MenuBarPanelViews`、`MenuBarViewModel`、
`AgentDeckApp`、`EmbeddedHelperRunner`、`DesktopWire`；必要时 `internal/desktop` serving 读取与
`internal/scanruntime` bootstrap 调度。不改 UX 文档声明的布局、文案与交互。

范围：按架构“打开路径”顺序执行：分段定位 → NSPopover 下限探测（记录结果；p95 >200 ms 为不可达、(150, 200] ms 为预先声明的决策阈值，
两者都停止返回设计，见性能评估 E2）
→ ready 后空闲分片预热 → 首帧内容最小化 → 保持 hosting 层级；按“启动顺序”实现恢复优先，
quota/bootstrap/预热/watcher 在 `model.usable` 后开始（750 ms 兜底）；切换按 `(publication_id, selection)` memoize。
无缓存与空库沿用当天优先 bootstrap 与持久 coverage。
删除被新预热机制取代的预布局代码及不再使用的 presentation 路径。

正确性回归：身份/epoch/schema/owner/mode/size/损坏/跨天/DST、旧有效数据保留、默认 quota、全部 tab、
筛选/焦点、慢额度 30 s、扫描 >120 s 与首批发布、partial/unavailable 不补零、`--no-scan` v1 映射。

验证（L3）：相关 Go/desktop/store/scanruntime 回归、Go 全量、race/vet、macOS 双架构构建、native 全量 XCTest；
正式批次：P01 三层（重建中层用当时的全量重建实现）、P02、P03、P04、P11 按契约样本数。

准则：`popover-floor`、`open-path-latency`、`startup-restore`、`selection-latency`、`stored-bootstrap`、
`first-import`、`serving-correctness`。

## 4. parallel-source-pipeline

目标：P07（RSS 上界与写事务）、P08、P09 达标（E3），重建期间 P01 重建层不回归。

文件：`internal/ingest`、`internal/usage`、`internal/session`、`internal/store`（写事务封装、
inventory staging/head、checkpoint）、`internal/scanruntime` 调度；不替换 SQLite driver、不引入 CGO。

范围：按架构“并行 source reduce 与单一有序写者”拆分 usage consumer；按 `baseline-remeasure` 的关键路径证据决定
session consumer 是否同样拆分；按“短事务”切分写事务与 inventory 接受；统一预算覆盖 reduce 结果与写者队列；
worker 数按 1/2/4/8 完整周期选择。旧 `bounded-source-pipeline` 任务保留的候选（有序并行解码、`ScanMessage` 单遍扫描、
`temp_store=MEMORY`）逐项按候选采用规则复核，未带来收益或 race 不稳定者删除。

正确性回归：worker=1 与现状逐字节一致；并行与串行 full oracle 逐逻辑行/视图哈希等价；
append/rewrite/truncate/move/parser bump/超大行/崩溃点/磁盘满；归档副本所有权、orphan 恢复、
会话重建；consumer Plan/Seal；FTS、backup/restore、直接 SQL 影响清单。

验证（L3）：相关回归、Go 全量、race（含新并行路径，多次运行无失败）、vet、macOS 双架构构建、迁移测试；
正式批次：P07（3 次全量重建）、P08、P09（20 对，对比 B0），以及 P01 重建层复测（在 `serving-open-path` 通过后）。

准则：`reduce-equivalence`、`bounded-accounting`、`short-transactions`、`aggregate-rss`、
`checkpoint-correctness`、`noop-append-bytes`、`rebuild-throughput`、`foreground-under-rebuild`。

## 5. incremental-update

目标：P10 达标（E4），增量投影写事务满足 P07 事务门。

文件：`internal/store` dirty/revision 与 derived partitions、`internal/usage` presentation/signals、
`internal/session` 受影响集合、`internal/desktop` projector 与 serving 发布、App 发布绑定。

范围：在现有增量投影上，按 `baseline-remeasure` 的 P10 分段处理主导段：受影响分区重算、projector 只重建受影响视图、
serving 写入与 App 绑定不等待全局终结；shadow 分区与短事务 swap。若分段证明 helper 启动/身份重建主导，
记录证据并按候选采用规则提出常驻扩展的设计增补，不在本任务内自行引入常驻引擎。
删除被取代的投影路径。

正确性回归：分区与 full oracle 等价、late/跨天/remove/rewrite、非可加统计、All 不重复、旧 publication 可读、
partial 不误报。

验证（L3）：相关 Go/store/usage/session/desktop 回归、Go 全量、race/vet、macOS 双架构构建、native wire/筛选；
正式批次：P10（100 次）及增量期间的事务分布。

准则：`partition-equivalence`、`nonadditive-statistics`、`generation-switch`、`update-latency`。

## 6. event-updates-and-cli

目标：P06 达标（E5），事件合并与 CLI 衔接完成，P10 不回归。

文件：`DesktopRefreshScheduler`、App 生命周期、`EmbeddedHelperRunner`、`internal/scanruntime` 有限任务、
CLI contract tests。

范围：文件事件有界合并 → 单协调者 → 有限任务 → 原子 publication → App 通知；启动/唤醒/明确漏事件有限校对；
无固定刷新周期；CLI 与 App 共用 facts 与 publication，任意 CLI 查询读已提交库；保留 scan/watch/`--no-scan`/
v1 stdout/stderr/Ctrl-C 契约；有限 helper 退出；App quit 停 watcher 并在 checkpoint 暂停 App-only 工作，
CLI 已接受工作按契约完成。

验证（L3）：相关 Go/race/vet/build、native/CLI 隔离集成；正式批次：P06（每臂 3 × 30 min，基线臂为 S0）、
P10 复测；连续写入、App/CLI 同时请求、Ctrl-C、App quit、崩溃/唤醒/version mismatch、publication 失败保留旧结果。

准则：`event-delivery`、`finite-request-compatibility`、`cli-app-sharing`、`idle-resource-budget`。

## 7. otel-capture-and-reconciliation

目标：P05 达标（E6），OTel 接收、持久化、身份与去重能力门按架构交付；OTel 开启时 P07、P10 不回归。

文件：telemetry receiver/journal/reconciler、store observations 与 usage 桥接、Settings/CLI 偏好与状态、
`cmd/agentdeck-perf otel` mock producer。

范围：默认 off；配置 preview 不自动应用；loopback 白名单、durable ACK、journal cap/retention、
精确 JSONL 跨源身份、coverage/gap；未证明身份时 `capture_only`，接收/持久化/状态仍须交付。
持续 listener 随 App 生命周期；独立后台模式显式另启。

验证（L3）：相关 Go/race/vet/build、隐私（无正文/headers/凭据落盘）、mock 与真实受支持版本 producer 分列；
正式批次：P05（100 个事件）、OTel 开启下的 P07 RSS 上界与 P10。真实 producer 端到端 freshness 另报。

准则：`durable-ingress`、`request-identity`、`transport-coverage`、`cross-source-idempotency`、`coverage-truth`、
`otel-publication-latency`。

## 8. cross-surface-acceptance

目标：在最终内容状态上重测 P01–P11 全矩阵（E7），并完成跨表面一致性与生命周期验收。

范围：全部指标按契约样本数在同一内容状态重测，只复用与该内容状态精确绑定的证据；
OTel on/off 分列；默认 quota/全部 tab、cache miss/跨天/升级/restore、CLI 与 Widget 逻辑对照、App quit 生命周期。
同步最终契约到 `docs/specs/cli-design.md`、`docs/specs/cli-manual.md` 与原型；topic 本地矩阵与评审记录收尾。
不执行未授权的 L4 release、安装或发布，不更新 `main` 全局状态，不关闭未交付的 Bug。

验证：L3 全量回归与全矩阵正式批次。

准则：`full-metric-matrix`、`cli-native-equivalence`、`upgrade-and-restore`、`lifecycle-failure-matrix`、
`contract-reconciliation`。

## Dispatch

文档任务在设计阶段推进至 `in_review`。实现任务在本矩阵评审通过后按锚点创建（标题 `任务：<anchor>`）；
旧锚点任务的处置在同一时点处理：已关闭的旧任务保留为历史，`ad-scan-performance-bounded-source-pipeline`
由 `parallel-source-pipeline` 取代并在新任务创建时关闭其协调。原 Bug 不因设计或实现进展而关闭；
本矩阵不授权提交、推送、PR、安装或发布。
