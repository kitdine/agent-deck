---
status: active
created: 2026-10-09
updated: 2026-10-31
---

# Scan performance：设计集与任务矩阵

工作区 `agent-deck.scan-performance` / 分支 `feature/scan-performance`。
2026-10-22 用户决定全 topic 重新设计：验收指标恢复为 `c376984` 的 P01–P11 原值，撤回此后全部
豁免、撤销与按修订条件接受；先建立可自动复现的测量契约，再在现有代码上重新实现，
无用代码与脚本在实现中删除；证据只进入 Neo4j CEv1 与评审记录。
2026-10-27 用户决定：本 topic 不设任何具体性能指标，只做优化；取消全部数值目标、相对门、`k` 与首击窗口探测的
决策门。各任务按收益优化并报告对比，最终由 `cross-surface-acceptance` 给出优化前后的对比报告，用户判断是否足够。

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
| 1 | requirements.md | 2026-10-27：取消 P01–P11 的数值目标与相对门，“数值验收”改为“性能衡量与报告”（基线 S0 / B0、ABBA 对比、用户判断）；行为与正确性要求不变 |
| 2 | performance-evaluation.md | 测量契约不变；删除相对门、`k` 与 E2 下限/暖启动/面板/quiet 探测的判定出口；可测性表只报告；候选采用规则去掉数值阈值；决策门改为阶段 |
| 3 | architecture.md | 打开路径改为不设门的优化步骤（暖启动并入产品路径、再次打开不重建内容）；删除面板探测与 quiet 对照；启动争用削减保留为候选；短事务等处删除数值目标 |
| 4 | tasks.md | 各任务目标由“达标”改为“优化并报告对比”；`serving-open-path` 删除探测范围与探测工具入口；最终验收改为对比报告 |

UX 文档 `ux/menubar.md`、`ux/cli.md`、`ux/settings.md` 本次无行为、文案、状态或标本变化，
沿用原评审结论（见 `reviews/ux-menubar.md`、`reviews/ux-cli.md`、`reviews/ux-settings.md`）。
测量打点与驱动不新增用户可见表面；首帧内容延后构建不改变可见内容与交互。
`prototype/tools/scan-performance-*` 是 `ux/prototype/serving/manifest.json` 固定的标本 harness，
任何任务都不删除或修改它们，因此 UX 标本身份与上述评审保持有效。

评审按 1→2→3→4 依次进行，每份文档各自的记录、round 与内容身份；全部通过才进入实现。

2026-10-29 OTel 设计变更（依据：同日 0.165.0 隔离生产者实验，事实记录在 Bug
`ad-bug-codex-subagent-usage-attribution` 评论；用户同日决定：经同意写入 Codex 配置、采集不自动开启、后台会话按
辅助会话处理）。受影响文档的 Review 列置回待评审，按下表顺序复审：

| 文档 | 本次变化与评审重点 |
| --- | --- |
| requirements.md | OTel 留在本 topic 作为该 Bug 的修复载体；修复目标；P05 移出性能衡量与报告 |
| performance-evaluation.md | P05 与 E6 的测量移出；OTel 打点降为诊断 |
| architecture.md | 会话级源所有权（capture_only/pending/jsonl_owned/otel_owned）、settle、晚到 rollout 撤销、贡献落点与 `otel:` 来源、支持表 |
| ux/settings.md | Codex 记忆联动状态、征得同意写入的同意区、写入/关闭/恢复的结果表与文案；新标本 |
| ux/cli.md | `telemetry status` 的 Codex 事实与警告、`enable --write-codex-config`、`disable` 的移除与恢复不完整；新标本 |
| tasks.md | 任务 7 改为 Bug 修复与正确性验收、跨主题依赖；任务 8 不再分 OTel 开/关列 |

`ux/menubar.md` 无变化：后台用量进入既有总量与模型构成，漏统计提示复用既有完整性 / Health 文案。
标本影响评估：本次改动的原型共享文件 `prototype/src/styles.css` 只在末尾追加 `.telemetry-consent*` 与
`.telemetry-details .tone-text-warn` 规则，`prototype/src/ServingSnapshot.jsx` 只新增 CLI 场景并让错误消息按场景取值
（既有场景输出不变），因此 `ux/menubar.md` 与既有 CLI 标本的评审结论沿用。Settings 不沿用：开启开关现在先经过
同意区，`scan-performance-repair-check.mjs` 中“直接点击开关进入 unknown”的 Settings 步骤已不能用当前原型复现
（该工具按约定不修改，也未重跑，以免覆盖已评审的截图）；开启路径与开启后的状态由
`scan-performance-otel-linkage.mjs` 覆盖，`ux/settings.md` 整体复审。
[manifest](ux/prototype/serving/manifest.json) 已更新这些文件与新增标本的身份。
文档 dispatch 为 `ad-scan-performance-doc-req`、`ad-scan-performance-doc-perf`、
`ad-scan-performance-doc-arch`、`ad-scan-performance-doc-tasks`。

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `measurement-harness` | [x] | [x] |
| 2. `baseline-remeasure` | [x] | [x] |
| 3. `serving-open-path` | [x] | [x] |
| 4. `parallel-source-pipeline` | [x] | [x] |
| 5. `incremental-update` | [x] | [x] |
| 6. `event-updates-and-cli` | [x] | [x] |
| 7. `otel-capture-and-reconciliation` | [x] | [x] |
| 8. `cross-surface-acceptance` | [x] | [x] |

`measurement-harness` 的当前评审与修复交接见 [评审记录](reviews/measurement-harness.md)。
`baseline-remeasure` 的当前评审与修复交接见 [评审记录](reviews/baseline-remeasure.md)。
`serving-open-path` 的当前评审与修复交接见 [评审记录](reviews/serving-open-path.md)；Round 2 PASS，精确状态门禁 VERIFIED；交付状态由对应 Beads 任务记录。
`parallel-source-pipeline` 当前评审与交接见 [评审记录](reviews/parallel-source-pipeline.md)；Round 3 PASS，候选 `scan-perf-psp-r2f-20261028-state` 完成门禁 VERIFIED；交付状态由对应 Beads 任务记录。
`incremental-update` 当前评审与交接见 [评审记录](reviews/incremental-update.md)；Round 2 PASS，候选 `scan-perf-iu-r1f-20261028-state` 门禁 VERIFIED；交付状态由 Beads 任务 `ad-scan-performance-incremental-update` 记录。
`event-updates-and-cli` 当前评审与交接见 [评审记录](reviews/event-updates-and-cli.md)；Round 4 PASS，候选 `scan-perf-euc-r2f-20261029-state` 门禁 VERIFIED；交付状态由 Beads 任务 `ad-scan-performance-event-updates-and-cli` 记录。
`otel-capture-and-reconciliation` 当前评审与交接见 [评审记录](reviews/otel-capture-and-reconciliation.md)；Round 2 PASS，候选 `scan-perf-otel-r1f-20261029-state` 门禁 VERIFIED；交付状态由 Beads 任务 `ad-scan-performance-otel-capture-and-reconciliation` 记录。

依赖：1 → 2 → 3、4；4 的前台准则在 3 通过后验证；3、4 → 5 → 6 → 7 → 8。
额外常驻 Go 引擎与多进程 parser 不是预建任务，只在性能评估的候选采用规则满足后经设计增补选入。

每个任务的 CEv1 WorkUnit 为 `scan-performance:<anchor>`，required criteria 为各节“准则”所列；
创建前按具体 ID 查询存在性，不复用旧任务的 WorkUnit。每个任务都需独立实现评审与精确
content-state 门禁 `VERIFIED`；测量类准则的证据按性能评估·记录与证据写入 CEv1。

通用约束（每个任务适用）：

- 在现有代码上修改；任务内确认不再使用的代码、测试与脚本随任务删除，删除前以引用搜索确认。
- 性能只按测量契约规定的方法、样本数与统计测量和报告，不设通过门；改动保留与否按候选采用规则的实测收益决定，
  某项变差时在任务评审记录中写明取舍。工具无效的批次整批重测，不能改口径。
- 测试只用临时 HOME/隔离 state root 与合成夹具；测量只用隔离 state root 与冻结语料的平移工作副本。
  除 `measurement-harness` 中经用户批准的冻结（含重新冻结）外，不读取真实日志或生产数据库。
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
- 新增 `cmd/agentdeck-perf`：`corpus freeze`/`corpus shift`（冻结真实语料与按锚定日平移）、`fixture`
  （仅 `small` 合成 profile，供测试）、`append`、`template`、`run`、`stats`、`gate`。每次冻结（含重新
  冻结）都须用户事先批准，在本任务中执行（2026-10-23 已批准一次重新复制）；语料身份与聚合清单写入
  CEv1。批次按性能评估“工作副本、模板与样本起点”编排：固定 HOME 路径、模板记录源树绑定指纹、
  按运行单元核对与重置、写入条件前后重新生成工作副本。`run` 支持基线与候选两臂按运行单元 ABBA 交错；
  负载每个运行单元记录、不作无效条件（供电与 `CPU_Speed_Limit` 不检查，2026-10-25 用户决定）；`stats`/`gate` 输出两臂比值与配对 bootstrap
  95% 区间，并按 `k` 判定通过、不确定或未通过（按 `k` 判定的部分是 2026-10-27 之前的历史契约，本任务的验证事实
  不改写；无 `k` 对比报告的迁移由 `serving-open-path` 承接）。
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
记录隐私测试（输出不含路径、会话 ID、正文）；关闭时零记录测试；夹具确定性测试（合成夹具同参数两次生成身份相同；同一语料与锚定日两次平移得到相同工作副本身份，
时间字段长度与其余字节不变，跨 DST 归属日不变；平移后落入春季不存在与秋季重复时刻的确定性结果；
原始处在秋季重复区间或平移后越过后续记录的文件回退为 UTC 平移且文件内 UTC 先后不变）；
模板起点测试（运行单元起点的源树绑定指纹与模板一致；复制得到的源树指纹不同并被判为工具无效；
从模板起步的 no-op 扫描读取字节与 decode 记录数为 0；累积写入序列内不重置）；
Release 产物不含 `PerfDriver` 符号。E0：按[性能评估的工具 pilot 表](performance-evaluation.md#样本与统计)
运行其中标为 `measurement-harness` 的条件，工具无效为 0；P05 的 pilot 原归 `otel-capture-and-reconciliation`（2026-10-29 P05 移出，不再执行）。
若点击刺激出现未路由样本，按性能评估在本任务评审中确认替代刺激。

准则：`clock-source`、`marker-coverage`、`driver-isolation`、`fixture-determinism`、`measurement-privacy`、
`pilot-validity`、`legacy-harness-removal`。

## 2. baseline-remeasure

目标：按新契约测得起点基线并定位主导成本（E1），作为后续任务的比较对象与方案选择依据。

基线：B0 = 提交 `14234ec`（旧 `baseline-and-open-path` 任务的原交付，主题产品行为改动之前）的构建，
只用于 P09；P08 是确定性计数，不与 B0 比较。S0 = 本任务开始时的内容状态（含 `measurement-harness` 打点与
旧 `bounded-source-pipeline` 任务保留的候选），用于 P01–P04、P06 基线臂、P07、P08、P09、P10、P11。两者均在同一机器、同一冻结语料的同一工作副本与模板上测量。

范围：按性能评估的正式样本数跑完整批次（P09 为 B0/S0 20 对 ABBA 交错）；给出分段：P01 首击超出部分所在段、
P02 启动期争用来源、P07 事务按 label 的 top 分布与 RSS 上界构成、P09 关键路径（usage/session/derive）、
P10 各段；更新来源与版本影响图。按需求“数值验收”的换算规则，以本任务测得的同口径 S0 统计量固定各
时延类指标与分层的 `k`（绝对目标 ÷ S0 值；S0 已满足者 `k = 1`），写入 CEv1（2026-10-27 起 `k` 失效，S0 统计量
保留为对比基线）。
不改变产品行为；本任务只允许修正 `measurement-harness` 的工具缺陷，修正后受影响批次整批重测。

验证（L1）：工具回归；基线批次本身即证据。

准则：`baseline-identity`、`baseline-matrix`、`cost-attribution`。

## 3. serving-open-path

目标：优化展示路径——P01（首击、重复点击、重建中三层）、P02、P03、P04、P11——并报告相对 S0 的对比（E2）；
保持 serving 正确性契约。

文件：`MenuBarItemController`、`MenuBarSurfaceView`、`MenuBarPanelViews`、`MenuBarViewModel`、
`AgentDeckApp`、`EmbeddedHelperRunner`、`DesktopWire`；必要时 `internal/desktop` serving 读取与
`internal/scanruntime` bootstrap 调度；架构“启动争用削减”的两个候选：`internal/desktop` 发布段经由已有派生快照缓存复用投影
（`derived_cache.go`、`serving_cache.go`，不新增复用键、不跳过 `Build`）、`internal/scanruntime` 发布段调用与扫描 worker 调度优先级，以及 `EmbeddedHelperRunner` 的 helper QoS。
候选 1 第一类处理（无事实变化的写入把派生 generation 置 dirty 或推进 revision）在实现第一步确认属于该类时，必要时修改发出这些写入的调用方：`internal/usage` 的 `FinalizeInventory`、`internal/session` 的 `ScanWithOptions`、`internal/scanruntime` 的 `finishAcceptedInventory` 及其调用的 `internal/store` inventory 完成写入。只限于让无事实变化时不发出写入；不改 schema、revision 触发器与写事务结构。并行 reduce、短事务切分与写事务封装仍归 Task 4，dirty/revision 机制与分区重算仍归 Task 5；若消除需要改动这些机制，停止并返回设计。不改 UX 文档声明的布局、文案与交互。
测量工具：删除 E2 探测留下的 Perf 专用代码——`PerfDriver` 的探测开关、静态内容视图、面板探测窗口与
`wait_startup_refresh` 步骤，`PerfTrace` 的 `refresh.startup.done`，`MenuBarItemController` 中的探测分支，
`cmd/agentdeck-perf` 的 `P01-floor*` 条件、`-pair`、`warmViolation`/`quietViolation`/`helpersAliveAtClick` 及其定向测试；
暖启动进入产品路径后，`popover.warm` 打点与其约束记录保留为产品打点。
对比报告工具迁移（本任务的对比批次首先需要它，后续任务复用）：`cmd/agentdeck-perf` 的 `gate.go` 改为不带 `--k` 的
`compare` 子命令，`run.go` 的两臂交错按性能评估“对比报告”覆盖各条件，`stats.go` 提供两臂所需的汇总；定向测试
`gate_test.go`（随之改名为 `compare_test.go`）与 `stats_test.go`。验收：不接受也不需要 `k`；输出不含通过/不确定/失败等
性能判定；ABBA 条件输出两臂 median、p95、p99（P03 每维、P09 中位 wall）、比值与固定种子的 bootstrap 95% 区间，组不完整时
拒绝出区间并报工具无效；P06 输出逐对差值与中位数；P07 与 P11 完整历史输出两臂并列值与中位数之比；P08 输出两臂计数并列；
P05 原为单臂统计，该输出义务自 2026-10-29 随 P05 移出而退役；已交付的本任务不因此重测，
`cmd/agentdeck-perf otel` 与 `otel.*` 打点只保留诊断用途。

范围：按架构“打开路径”实施，每步以分段打点衡量并按候选采用规则保留或撤回：分段定位 → 首次窗口暖启动
（产品路径，满足不产生可见帧、不改变前台应用与 key 窗口、不触发 helper 的约束）→ ready 后空闲分片预热 →
首帧内容最小化 → 保持 hosting 层级，重复打开先以分段与视图/控制器身份观察确认是否存在重复构建，确认后才实施
“未变时不重算、不重建” → 启动争用削减候选 1、2；按“启动顺序”的恢复优先
（quota/bootstrap/预热/watcher 在 `model.usable` 后开始，750 ms 兜底）保留；切换按 `(publication_id, selection)` memoize。
无缓存与空库沿用当天优先 bootstrap 与持久 coverage。删除被新预热机制取代的预布局代码及不再使用的 presentation 路径。

正确性回归：身份/epoch/schema/owner/mode/size/损坏/跨天/DST、同一注入时钟下缓存命中与强制重算的 serving payload 逐字节相等、同日额度跨 stale 阈值与跨整点/价格生效/本地日/dirty/revision 时重算、旧有效数据保留、默认 quota、全部 tab、
筛选/焦点、慢额度 30 s、扫描 >120 s 与首批发布、partial/unavailable 不补零、`--no-scan` v1 映射；
暖启动不产生可见帧、不改变前台应用与 key 窗口、启动期间点击先完成关闭再打开。

验证（L3）：相关 Go/desktop/store/scanruntime 回归、Go 全量、race/vet、macOS 双架构构建、native 全量 XCTest；
Release 产物不含 Perf 驱动与已删除的探测符号；`agentdeck-perf` 包测试随删除的条件同步更新。
对比批次：P01 三层（重建中层用当时的全量重建实现）、P02、P03、P04、P11 首批按契约样本数，本任务构建与 S0 构建按运行单元
配对交错，报告方法按性能评估“对比报告”的分类（这些条件均为 ABBA 比值与区间）；P11 完整历史本任务不跑对比，
只在 Task 8 按“两臂并列”报告。报告记入评审记录与 CEv1。

准则：`open-path-latency`、`startup-restore`、`selection-latency`、`stored-bootstrap`、
`first-import`、`serving-correctness`。

## 4. parallel-source-pipeline

目标：优化采集与资源——P07（RSS 上界与写事务）、P08、P09——并报告相对 S0（P09 另对 B0）的对比（E3）；重建期间 P01 重建层一并报告。

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
对比批次：P07（按契约正式样本：3 次全量重建 + 100 次增量，增量只在第 3 个运行单元的重建之后，事务取该臂全部 `tx.*`）、P08、P09（20 对，对比 B0 与 S0），以及 P01 重建层复测（在 `serving-open-path` 通过后）。

准则：`reduce-equivalence`、`bounded-accounting`、`short-transactions`、`aggregate-rss`、
`checkpoint-correctness`、`noop-append-bytes`、`rebuild-throughput`、`foreground-under-rebuild`。

## 5. incremental-update

目标：优化更新路径 P10 并报告相对 S0 的对比（E4）；增量投影写事务一并报告。

文件：`internal/store` dirty/revision 与 derived partitions、`internal/usage` presentation/signals、
`internal/session` 受影响集合、`internal/desktop` projector 与 serving 发布、App 发布绑定。

范围：在现有增量投影上，按 `baseline-remeasure` 的 P10 分段处理主导段：受影响分区重算、projector 只重建受影响视图、
serving 写入与 App 绑定不等待全局终结；shadow 分区与短事务 swap。若分段证明 helper 启动/身份重建主导，
记录证据并按候选采用规则提出常驻扩展的设计增补，不在本任务内自行引入常驻引擎。
删除被取代的投影路径。

正确性回归：分区与 full oracle 等价、late/跨天/remove/rewrite、非可加统计、All 不重复、旧 publication 可读、
partial 不误报。

验证（L3）：相关 Go/store/usage/session/desktop 回归、Go 全量、race/vet、macOS 双架构构建、native wire/筛选；
对比批次：P10（100 次）及增量期间的事务分布。

准则：`partition-equivalence`、`nonadditive-statistics`、`generation-switch`、`update-latency`。

## 6. event-updates-and-cli

目标：事件合并与 CLI 衔接完成，P06 与 P10 报告相对 S0 的对比（E5）。

文件：`DesktopRefreshScheduler`、App 生命周期、`EmbeddedHelperRunner`、`internal/scanruntime` 有限任务、
CLI contract tests。

范围：文件事件有界合并 → 单协调者 → 有限任务 → 原子 publication → App 通知；启动/唤醒/明确漏事件有限校对；
无固定刷新周期；CLI 与 App 共用 facts 与 publication，任意 CLI 查询读已提交库；保留 scan/watch/`--no-scan`/
v1 stdout/stderr/Ctrl-C 契约；有限 helper 退出；App quit 停 watcher 并在 checkpoint 暂停 App-only 工作，
CLI 已接受工作按契约完成。

验证（L3）：相关 Go/race/vet/build、native/CLI 隔离集成；对比批次：P06（每臂 3 × 30 min，基线臂为 S0）、
P10 复测；连续写入、App/CLI 同时请求、Ctrl-C、App quit、崩溃/唤醒/version mismatch、publication 失败保留旧结果。

准则：`event-delivery`、`finite-request-compatibility`、`cli-app-sharing`、`idle-resource-budget`。

## 7. otel-capture-and-reconciliation

目标：修复 Bug `ad-bug-codex-subagent-usage-attribution`——支持表内 Codex producer 的后台请求用量计入 All 与
模型构成，并以各自会话记录可追溯；已由 JSONL 计量的会话不重复计入；Codex 记忆联动与征得同意的配置写入按 UX 交付。
2026-10-29 起本任务不产生性能批次（P05 移出，E6 改为正确性验收）。

已有实现（2026-10-29 开发轮，未提交，本设计复审通过后在其上修改）：迁移 33 的 observation/cursor/coverage 表，
`internal/telemetry` 接收端、journal、reconciler 与状态，CLI `telemetry status|enable|disable|config-preview|import`
及隐藏的 `serve`，Settings“实时用量”组与 App 随行 listener，`cmd/agentdeck-perf otel` mock producer。

文件：上述实现；`internal/store` 贡献、会话记录、`telemetry_sessions` 来源标记与撤销；`internal/usage` orphan 清理与
rebuild 对 `otel:` 来源的处理；`internal/session` 的会话索引投影（`otel:` 来源与元数据的写入/删除/重新生成）、文件
inventory 全集对账对 `otel:` 来源的豁免、会话排除对投影的约束，以及 `cmd/agentdeck` `session show` 对无正文会话的读取；Codex 配置读取与受管写入（新文件，使用已依赖的 go-toml/v2）；status 与 App 解码；
Settings 同意区与结果提示；原型 `TelemetrySettings.jsx` 与 CLI 标本已在设计中更新，实现以其为准。

范围：

- 事件识别按架构修正：事件名取 `event.name` 属性，白名单加入 originator，transport 由事件名判定；
  删除依赖 producer instance + response id 的 canonical key 路径。
- 会话所有权（JSONL 证据只来自非 `otel:` 事件或 rollout 文件）、settle、晚到 rollout 撤销、贡献写入、会话记录与
  来源标记、会话索引投影、`otel:` 来源排除、rebuild 后重派生，按[架构·去重与源所有权](architecture.md#去重与源所有权)。
  来源标记查询只是分类接口；后台会话必须经现有会话读取面（`session list`、`session show`、popover 会话面板）可见。
- Codex 配置读取、联动规则与受管写入/移除，按[架构·Codex 配置读取与受管写入](architecture.md#codex-配置读取与受管写入)；
  Settings 与 CLI 的状态、文案、通道与退出码按 [Settings](ux/settings.md#codex-记忆联动与配置写入) 与
  [CLI](ux/cli.md#codex-记忆联动2026-10-29)，`telemetry status` 的 JSON 字段名改为原型 `engineSamples` 的命名。
  已实现的 Settings 文案与原型不一致处（如隐私说明）按原型修正。
- 支持表：0.165.0 SSE `codex_exec` 入表；WebSocket、Memories stage1、guardian、交互式 TUI 与 Desktop、重试/失败/中断、
  主会话压缩请求按架构做同类隔离实验（隔离 CODEX_HOME 与 HOME、本地模拟模型服务、不改真实配置、不发真实请求），
  通过者入表，未通过者保持 `capture_only`；guardian 或压缩若确认产生无 JSONL 用量，加入联动的“需要采集的功能”。
  评审记录写明每个组合与结论。
- 默认 off、不自动开启、loopback 与 token、durable ACK、journal cap/retention、coverage/gap 保持已实现契约。

跨主题依赖：`auxiliary-session-classification` 采用 `codex-otel-background` 来源标记，把这些会话判为辅助会话。
本任务交付标记与只读查询接口，并在该专题的 Beads 任务上登记接口请求；v0.7.0 版本装配需两者同在才算该 Bug
修复完整。逐会话记忆提取与来源会话的关联作为该专题或后续专题的待办，不在本任务实现。

验证（L3）：相关 Go/race/vet/build、迁移与 rebuild、隐私（无正文/headers/凭据落盘，token 只在受管段与 0600 文件）；
会话所有权测试：JSONL 会话不计入、无 rollout 会话 settle 后恰计入一次并生成会话记录与标记、settle 前不计入、
晚到 rollout 撤销贡献/会话记录/标记后只剩 JSONL、replay 与重复导入幂等、rebuild 后重派生且不重复、orphan 清理
不删 `otel:` 来源、conflict 不计入、支持表外 `capture_only`；自身写入不成为证据：同一 `otel_owned` 会话经连续多次
reconciliation、App/CLI 重启、journal 清理与全量 rebuild 后仍为 `otel_owned` 且贡献恰一份；读取面断言：后台会话出现在
`session list` 与 popover 会话面板（项目未知、有会话 id），`session show` 返回元数据与用量明细且转录为空、不提示运行
session scan，`session search` 不命中；会话索引 purge/rebuild 后投影重新生成，文件 inventory 全集对账不删除 `otel:` 来源，
被排除的会话不投影，晚到 JSONL 后读取面只显示 JSONL 来源且 `otel:` 来源被删除；配置写入测试（只用临时 HOME）：absent 追加后其余字节
不变且解析为 managed、other 拒写且不改、不可写与写入期间被改动均不改、移除只删未改动的受管段、标记缺失或被改为
恢复不完整且不改文件；联动测试：记忆 off 时 listener 不启动、运行中变 off 即停止、偏好保留；native：Settings 同意区
与全部结果状态、开关真实状态；真实 producer 端到端：隔离 Codex 实验经 App 随行 listener 计入后台请求，App 与 CLI
总量一致、会话记录带标记。不跑性能批次。

准则：`durable-ingress`、`session-ownership`、`background-metering`、`late-jsonl-retraction`、
`producer-support-table`、`codex-config-consent`、`memories-linkage`、`coverage-truth`、`real-producer-e2e`。

## 8. cross-surface-acceptance

2026-10-31 独立复评 Round 2 PASS；Task 精确状态门禁 VERIFIED，待授权提交。当前结论与证据状态见
[评审记录](reviews/cross-surface-acceptance.md)。不代表任务已交付或专题整体完成。

- 批次脚本支持输入身份封存、失败即停、受控续跑与验收域布尔偏好恢复；工具补齐 P09 的
  S0 traced 基线、P06 配对中位数及 P08/P09 分目录检查。native serving 已支持 schema 33，
  保留对未知未来 schema 的拒绝；helper 的 SIGTERM 启动竞态和 SIGPIPE trace 丢失已修复。
- 最终矩阵的 14 个正式条件及对应 pilot 已完成并核对封存身份。P02 和 P01 三层/P03
  使用必要的最终状态补测，早期数值保留为历史，不充当最终结果。重建中点击改为双向握手：
  App 可用且静置后请求重建，CLI 进程成功启动后才放行点击；原 CLI start→exit 区间校验保留。
  两臂采用相同 Perf-only 驱动补丁，产品 helper 字节身份不变；未受影响条件不重复运行。
- 验证包括 Go 全量串行回归、最终受影响包回归、信号与握手 race、双架构构建、vet、
  native 全量及新增 CLI wire → Widget 逐值对照、原型与文档检查。既有 opt-in schema
  手工矩阵未运行，不据此声称安装态 Widget、手工 UI 或 L4 验收通过。
- CLI 设计/手册、topic 实现对照及原型默认自动更新已同步。独立评审须覆盖这些变更；
  Round 2 已完成来源修复的独立复评，沿用未变部分的首轮检查。
- 历史失败保留：默认并发 Go 回归超时（根因未定）、schema 不兼容、SIGTERM/SIGPIPE
  空 trace、显示器切换、午夜准入拒绝、分目录误判及重建同步失效。外部打断与产品/工具失败
  分开记录，不选择性剔除产品失败，不以重新采样挑选更好的数值。
- 初次 P06 的采样在偏好恢复错误之前已完成。脚本已修复数字布尔值并先持久保存原值；
  该次原偏好缺少备份，隔离验收域仍保持关闭，不能宣称已恢复。按用户要求不重开安装态 App。
- 完成证据：WorkUnit `scan-performance:cross-surface-acceptance`，目标状态
  `scan-perf-csa-rereview-r2-20261031-state`；完整数值、置信区间、逐臂身份、模板身份、失败处置及
  工具来源补全见 `scan-perf-csa-repair-r1-20261030:full-metric-matrix`。四个工具均以恢复源码
  重建并匹配原二进制 SHA-256；有效样本复用，未重跑测量。旧完成观察和评审否定观察保留。
  2026-10-31 用户明确接受当前整体收益及 P04、RSS 退化取舍。验收状态
  `scan-perf-topic-accepted-20261031-state` 的 Task/Topic 门禁 VERIFIED；尚待授权 Git 交付，未归档专题。

目标：在最终内容状态上对 P01–P11（不含已移出的 P05）全矩阵做相对 S0（P09 另对 B0）的对比（E7），各项按性能评估“对比报告”的分类方法报告：
ABBA 比值与区间（P01 三层、P02、P03、P04、P09、P10、P11 首批）、P06 配对差值、P07 与 P11 完整历史两臂并列、
P08 计数并列；给出逐项基线、最终值与变化幅度的报告交用户判断整体提升是否足够；并完成跨表面一致性与生命周期验收。

范围：全部指标按契约样本数在同一内容状态重测，只复用与该内容状态精确绑定的证据；
性能矩阵不分 OTel 开/关列；默认 quota/全部 tab、cache miss/跨天/升级/restore、CLI 与 Widget 逻辑对照、App quit 生命周期。
同步最终契约到 `docs/specs/cli-design.md`、`docs/specs/cli-manual.md` 与原型；topic 本地矩阵与评审记录收尾。
不执行未授权的 L4 release、安装或发布，不更新 `main` 全局状态，不关闭未交付的 Bug。

验证：L3 全量回归与全矩阵对比批次。

准则：`full-metric-matrix`、`cli-native-equivalence`、`upgrade-and-restore`、`lifecycle-failure-matrix`、
`contract-reconciliation`。

## Dispatch

文档任务在设计阶段推进至 `in_review`。实现任务在本矩阵评审通过后按锚点创建（标题 `任务：<anchor>`）；
旧锚点任务的处置在同一时点处理：已关闭的旧任务保留为历史，`ad-scan-performance-bounded-source-pipeline`
由 `parallel-source-pipeline` 取代并在新任务创建时关闭其协调。原 Bug 不因设计或实现进展而关闭；
本矩阵不授权提交、推送、PR、安装或发布。
