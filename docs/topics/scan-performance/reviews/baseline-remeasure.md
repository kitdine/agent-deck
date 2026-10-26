---
status: active
topic: scan-performance
subject: baseline-remeasure
---

# baseline-remeasure 评审记录

## Round 1 — 2026-10-25

### 📋 E1 起点基线交付评审

📊 综合评分：6/10

✅ 评审结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra。
- Method：独立冷上下文单人评审；核对当前契约、CEv1 原始结构化事实、候选指纹和工具源码；未参与本次实现，无委派。
- Scope：Task 2 `baseline-remeasure` 的 E1 基线矩阵、身份、成本归因及四个打点修正文件。
- Reviewed state：HEAD `34a5ca1bfc8f9150ecdaacbff704ed6b66ebdde3`；
  scoped fingerprint `09dfcc83744d2ba3bd52d0e6d0b2b1cf870e7324db523eb77076d8975484629b`；
  CEv1 ContentState `scan-perf-br-s0-20261025-state`。
- WorkUnit：`scan-performance:baseline-remeasure`；Beads：`ad-scan-performance-baseline-remeasure`。
- 完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### BR-R1-F1 — P1：P09 语料规模未满足当前明示要求

- 位置：[requirements.md:214](../requirements.md#L214)、[tasks.md:132–140](../tasks.md#L132-L140)。
- 证据：`scan-perf-br-s0-20261025-baseline-identity` 的 `batch.working_copy_files = 2232`，
  而需求要求 ≥5,071 文件；`scan-perf-br-s0-20261025-baseline-matrix` 将该批列为正式 P09 基线。
  [测量契约](../performance-evaluation.md#夹具冻结的真实语料) 的冻结语料决定规定真实 JSONL 来源与当天代表性下限，
  但当前需求仍保留总文件数下限。现有记录没有明确解决两者的差异。
- 风险：小于指定规模的测量不能证明指定规模的基线或支撑其后续比较；不能由工具无效数为零推导契约满足。
- 💡 修复：先解决语料与规模要求的权威冲突；按现行要求取得合格冻结语料后重测受影响基线，或取得用户明确决定并通过相应设计变更。
  不允许复制文件凑数、静默降下限或将现有 2,232 文件结果当作 ≥5,071 文件证据。
  若需重新冻结真实日志，必须另获批准；本轮不执行冻结。
- Disposition：OPEN；本任务修复，carrier `ad-scan-performance-baseline-remeasure`。

#### BR-R1-F2 — P1：P09 资源与分段证据被其他条件替代

- 位置：[performance-evaluation.md:295](../performance-evaluation.md#L295)、
  [run.go:525–543](../../../../cmd/agentdeck-perf/run.go#L525-L543)、
  [run.go:841–856](../../../../cmd/agentdeck-perf/run.go#L841-L856)。
- 证据：P09 调用 `runCLI(..., "", ..., arm, "scan")`，未启用 trace；`runCLI` 只取前台命令的 `ProcessState.SysUsage`，
  源码明确说明扫描在 detached worker 中。矩阵给出的 B0/S0 前台 CPU 中位仅 39.051/36.894 ms；
  `scan-perf-br-s0-20261025-cost-attribution` 明确以三个 P07 的 S0 全量重建替代 P09 的 CPU/bytes/RSS/分段。
  P07 的 collector CPU 为 25,752–27,246 ms，与前台 CPU 不是同一测量对象。
- 风险：20 对 ABBA P09 没有自身完整重建的资源与分段数据，也缺 B0 对应数据；其他条件的三个 S0 样本不能代表这 20 对，
  不能支撑 P09 声称的 wall/CPU/bytes/RSS 全部报告及同批资源判定。这里只否定替代关系，不否定已有 wall 数值。
- 💡 修复：补齐 P09 两臂自身的 worker/helper 资源与阶段采集；B0 按契约使用其原有 profile 与适用资源方法，
  若实际不能满足则先解决测量契约，不用前台 rusage 或 P07 样本冒充。工具修正后按契约重跑受影响 pilot/正式 P09 批次，
  将各臂分布、资源与分段绑定该批内容状态。
- Disposition：OPEN；本任务修复，carrier `ad-scan-performance-baseline-remeasure`。

#### BR-R1-F3 — P2：P08 缺少可判断的 append 字节上界

- 位置：[requirements.md:213](../requirements.md#L213)、[performance-evaluation.md:294](../performance-evaluation.md#L294)。
- 证据：`scan-perf-br-s0-20261025-baseline-matrix.statistics_ms.P08` 报告三次相同的 append 读取量
  4,184,133 bytes、989 decoded records、111 session documents、120 usage imported，
  同时明确注明 `append bound (delta + declared anchors) not computed by the tool; counts reported`。
  因此没有本批 delta 字节、声明 anchors 合计上界及不等式比较。
- 风险：无法区分必要追加/anchor 读取与历史重读，也无法给出后续优化所需的 P08 起点是否超界。
  E1 不要求 S0 已达到产品目标，但必须能测出并明确报告该绝对计数条件的结果。
- 💡 修复：为本批报告实际追加字节、每来源有限 anchors 的声明规则与合计上界、实读字节及比较结果；
  不能从实读量反推一个恰好通过的上界。若现有保留事实不足以重建，补齐工具并按契约整批重测 P08；
  如实记录基线超界即可，不在此任务中引入产品优化。
- Disposition：OPEN；本任务修复，carrier `ad-scan-performance-baseline-remeasure`。

### 🟡 建议改进 — 推荐

无。以上均为交付缺口，不能延期到 PASS 之后。

### 🟢 优点

- HEAD、四文件候选 blob 与 CEv1 S0 指纹完全吻合，身份可复算。
- 正式样本数、失败/无效计数、分层统计与固定 k 表已结构化记录；没有将 S0 未达性能目标等同于 E1 失败。
- 交接明确披露语料、append 上界及 P09 替代测量的局限，保留了可审计事实。

### 📝 总结

本轮 FAIL 的原因是测量交付不满足既定输入规模与报告口径，而非要求基线提前达到后续任务的性能目标。
未解决三项 finding 前，Review 保持未勾选，任务返回修复；不提供提交/推送 checkpoint。

#### Evidence

- 当前工作区：`agent-deck.scan-performance`，`feature/scan-performance`；位于主仓库 `.worktrees/scan-performance`，
  Git common directory 与主仓库一致。实际 HEAD 与记录一致。
- 按 ContentState 既有配方复算：`git diff --name-only HEAD` 与 `git ls-files --others --exclude-standard` 的并集，
  排除 `docs/`、`apps/macos/.build/`，按路径字节排序；`path:git hash-object`（删除记 `deleted`）以 LF 连接，无尾 LF，SHA-256。
  得到上述 `09dfcc83…4629b`。四路径为 `internal/desktop/serving_cache.go`、
  `internal/scanruntime/profile_test.go`、`internal/scanruntime/scanruntime.go`、`internal/scanruntime/serving.go`。
- 查阅 CEv1：`scan-perf-br-s0-20261025-state` 及同前缀 `baseline-identity`、`baseline-matrix`、`cost-attribution`。
  复用其测量事实，未读取真实日志、副本正文或生产数据库。
- 规范 `gate-status.cypher` 首次查询返回 `NOT_VERIFIED / no_required_criteria`：原三条 `requires` 缺少
  `ce_namespace` 和稳定 ID。本轮在阶段授权下按已声明准则补充三条规范关系；预检全部 `ok`，回读数量 3/3。
  保留原关系与历史观察，不将交接所称 3/3 pass 当作规范门结果。
- 追加反证 `scan-perf-br-r1-20261025-baseline-matrix-fail` 与
  `scan-perf-br-r1-20261025-cost-attribution-fail`，绑定原 S0；保留原 pass 观察。
  节点 2/2、关系 4/4，关系预检全部 `ok`；规范门复查返回 `FAILED`，三个 required criteria 齐全，
  两条反证均 `target_matches=true`、`applicable=true`、`malformed=false`。
- 工具源码只读核验后已有决定性反证，停止扩大验证；未运行构建、Go 回归、E0 或正式性能批次。
  该局限不影响已确定的三项 FAIL；本轮不对其余工具实现作全面无缺陷保证。
- `python3 scripts/check-review-records.py docs/topics/scan-performance/reviews/baseline-remeasure.md`、
  `bash scripts/check-topic-docs.sh scan-performance`、`git diff --check` 均通过。
- `make check-whitespace` 失败，且使 `python3 scripts/check-docs.py` 在前置 whitespace 步骤退出：
  既有未跟踪 `apps/macos/.build/index-build/` 生成文件存在缺尾换行与尾空白；本轮未修改这些产物。
  单独调用 `check-docs.py` 的 `scoped_errors` 检查当前 Markdown 的链接、结构与文档集，范围检查通过；
  不将此结果表述为全局 L0 通过。

#### 下一步指令

修复：scan-performance / baseline-remeasure / BR-R1-F1、BR-R1-F2、BR-R1-F3

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-25（Round 1 后）

Repairer：Claude Code 主会话（Round 1 前的实现者）；范围为 BR-R1-F1–F3。Round 1 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- BR-R1-F1 -> repaired in candidate（用户决定）：
  - **用户决定（2026-10-25，修复指令）：** "对于文件数要求 需要变更"。
  - **需求变更：** [requirements.md](../requirements.md) P09 行改为“批次绑定的冻结真实语料的全量重建”；“数值验收”增加
    P09 输入规模规则：不设 ≥5,071 文件下限，语料须满足[性能评估·夹具](../performance-evaluation.md#夹具冻结的真实语料)
    的冻结范围与当天代表性下限，文件数与总字节随批次语料身份记录。5,071 保留为原始缺陷背景。
  - **本批语料：** 身份 `1d9ee56f…57e3f4`，2,232 个文件，未复制或改动语料。
- BR-R1-F2 -> repaired in candidate：
  - **工具：** P09 两臂各自采集本臂的完整重建成本，不再用 P07 替代。S0 臂把 trace 写入 `<sample>/S0`，取 worker
    （collector）生命周期的 CPU 与 RSS、`scan.*` 阶段、`scan.end` 读取字节；B0 臂设 `AGENTDECK_SCAN_PROFILE=1`，
    取其原有 `scan_performance` 记录中的 worker CPU、worker 峰值 RSS（加前台峰值作上界）、ingestion `read_bytes` 与各阶段。
    任一臂缺本臂数据即工具无效。见 `cmd/agentdeck-perf/run.go` P09 计划与 `stats.go` `p09ArmCost`。
  - **契约：** [performance-evaluation.md](../performance-evaluation.md) P09 行写明上述口径。
  - **回归：** `TestP09ReportsEachArmsOwnRebuildCost`（两臂成本、RSS 上界构成；删去 S0 trace 即工具无效）。
- BR-R1-F3 -> repaired in candidate：
  - **工具：** append 命令按实际写入报告 `delta_bytes`，并按产品声明规则报告 `anchor_bytes`：每个被追加文件
    2 个 consumer（usage、session）× min(4096, 追加前大小)，即 `ingest.Coordinator.Plan` 的校验后缀。不从实读量反推。
    P08 统计输出实读字节、上界、差值与 `append_within_bound`。
  - **契约：** P08 行写明上界口径。
  - **回归：** `TestAppendDrivesExactIngestCounters` 断言报告的 delta 与 anchors 等于文件实际增长与声明规则；
    `TestP08ReportsTheAppendBound` 覆盖超界报告与缺少 append 结果即工具无效。

随修复发现并修正的工具缺陷（均属 `measurement-harness` 工具，修正后受影响批次整批重测）：

- **`scan.end` 晚于结果发布：** `internal/scanruntime/scanruntime.go` 先发布结果再记 `scan.end`，进程内调用方可能在打点前
  停止 trace（全量测试负载下 `TestAppendDrivesExactIngestCounters` 复现）。改为先记再发布；仅影响打点顺序。
  回归 `TestScanEndIsRecordedBeforeTheResultIsPublished`（旧顺序下失败，修复后通过；scanruntime race 通过）。
- **外部打断被记成产品失败：** P03、P10 的 pilot 中，另一程序夺走前台后系统关闭 popover，样本被记为产品失败
  （trace `frame.blocked visible=0 app_active=0`）。按 2026-10-25 用户决定，驱动在 popover 显示期间检测到前台被夺
  （`willResignActive`）、或开 popover 前状态栏图标因他人占前台不在屏上时，记 `driver.interrupted`；`agentdeck-perf run`
  把该次尝试移到 `interrupted/` 留档，等待解锁后重跑同一运行单元，最多 5 次；`stats` 报告 `interrupted_reruns`。
  锁屏同样按打断重跑。契约见 requirements“无效样本”条与 performance-evaluation 样本类别表。回归
  `TestInterruptedAttemptsAreRerunNotCounted`。驱动只在 Perf 构建编译，无单元测试，由 Perf 构建与 E0 验证。
- **供电与降频不作条件：** 按 2026-10-25 用户决定，删除交流电/低电量模式与 `CPU_Speed_Limit` 的检查与记录。

修复目标：HEAD `34a5ca1bfc8f9150ecdaacbff704ed6b66ebdde3`；scoped fingerprint
`45b151a71a73060df23b1822f1f0e105a4064f9c3ce674a60be585ffcf5efa63`（既有配方，11 个非 docs 路径，全部存在）；
CEv1 ContentState `scan-perf-br-repair-20261025-state`。批次后复算指纹未变。

E0/E1（同一内容状态、同一冻结语料与锚定日 2026-10-25）：
- **E0：** 12:12–12:47 EDT，14 个条件工具无效 0、产品失败 0、打断重跑 0。
- **E1：** 12:47–16:19 EDT，13 个条件按正式样本数各跑一次，工具无效 0、产品失败 0、打断重跑 0。
- **固定 k（p95；P01 另含 p99；P07 另含 max）：** P01 首击 0.4256/0.8046，重复 0.3906/0.6807，重建中 0.3965/0.5287；
  P02 0.6712；P03 client 0.7453、period 0.6011、panel 0.6027；P04 1；P07 写事务 p95 1、max 0.6612；P10 0.2830；P11 首批 1。
- **P08：** no-op 计数全 0；append 实读 4,184,133 bytes，上界 1,040,696（delta 65,064 + anchors 975,632），超出 3,143,437，
  三次一致。这是 S0 基线事实，不是 E1 缺陷。
- **P09：** 20 对，S0/B0 中位 wall 0.869（B0 15,300 ms，S0 13,296 ms）。worker CPU 中位 B0 25,906 / S0 26,655 ms；
  RSS 上界最大 B0 120.1 / S0 175.2 MiB；读取字节两臂均 501,391,380。session 是两臂的较长分支；S0 的 derive 1,470 ms（B0 685 ms）。
- **作废且不引用的运行：** 原 S0 批次（`scan-perf-br-s0-20261025-state`）已被本次取代；修复期间因上述工具缺陷停止或作废的
  pilot/正式批次未产生证据。

验证：
- **测试：** `go vet ./...`、`scripts/run-go-test.sh ./...` 通过（E0 前由测量链执行）；`agentdeck-perf` 包测试与
  scanruntime race 测试通过；Perf App 构建通过。
- **CEv1：** 新证据 `scan-perf-br-repair-20261025-{baseline-identity, baseline-matrix, cost-attribution}` 记为 pass，绑定上述
  ContentState，并 `supersedes` 原 `scan-perf-br-s0-20261025-*` 通过证据；评审追加的两条反证仍绑定原 S0 状态，保留不动。
  规范 `gate-status.cypher` 查询目标状态：VERIFIED，missing 为空，unresolved 为空；invalidated 只列出被取代的原 S0 证据。
  这是证据状态，不代替复评结论。

## Round 2 — 2026-10-25

### 📋 baseline-remeasure 修复复评

📊 综合评分：8/10

✅ 复评结论：FAIL

- Reviewer：OpenCode / GPT-6 Astra。
- Method：独立冷上下文单人复评；逐项核对 Round 1、修复候选、源码与 CEv1，并以隔离合成输入验证新发现；未委派。
- Scope：BR-R1-F1、BR-R1-F2、BR-R1-F3 及其修复引入的 P09 数据完整性判断。
- Reviewed state：HEAD `34a5ca1bfc8f9150ecdaacbff704ed6b66ebdde3`；11 个非 docs 路径的 scoped fingerprint
  `45b151a71a73060df23b1822f1f0e105a4064f9c3ce674a60be585ffcf5efa63`；
  CEv1 ContentState `scan-perf-br-repair-20261025-state`。
- WorkUnit：`scan-performance:baseline-remeasure`；Beads：`ad-scan-performance-baseline-remeasure`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

#### BR-R2-F1 — P2：P09 缺失读取量打点仍被当作有效零值

- 位置：[stats.go:779–796](../../../../cmd/agentdeck-perf/stats.go#L779-L796)、
  [performance-evaluation.md:296](../performance-evaluation.md#L296)。
- Disposition：NEW / OPEN；本任务修复，carrier `ad-scan-performance-baseline-remeasure`。
- 证据：`p09ArmCost` 仅在遇到 collector 的 `scan.end` 时累加 `record.N["read_bytes"]`，
  但没有检查该事件或字段是否存在；返回前仅检查 collector 生命周期以及 usage/session/derive 的非零耗时。
  `readSample` 只保证 trace 可解析、有生命周期且没有显式 invalid，不保证 `scan.end` 存在。
- 独立复现：在临时目录生成一个 P09 B0/S0 配对，B0 有完整 profile，S0 有合法 header、
  usage/session/derive spans 和 collector `resource.lifetime`，刻意不写 `scan.end`。
  对当前源码执行 `go run -mod=vendor ./cmd/agentdeck-perf stats <合成批次目录> P09`，退出码 0，
  输出 `tool_invalid=0`、`S0_rebuild.runs=1`、`S0_rebuild.read_bytes=[0]`。
  输入完全合成，未读取真实语料或数据库；临时输入已清理，复现必要事实均在此保留。
- 风险：缺失重建读取量时会制造一个有效的零读取结果，违反“任一臂缺本臂数据即工具无效”，
  使工具无效数为零不能排除这一类缺失数据。现有回归只删除整个 collector 文件，未覆盖局部打点缺失。
- 💡 修复：对 S0 的必需 `scan.end` 及 `read_bytes` 做存在性校验，区分合法零值和缺失；
  补充分别缺事件、缺字段及正常完整输入的回归。按测量契约处理受影响工具证据与批次，不能以默认零值填补未知值。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- BR-R1-F1 → CLOSED：修复交接记录用户 2026-10-25“对于文件数要求 需要变更”的决定；当前需求已明确取消
  ≥5,071 文件下限，使用批次绑定的冻结真实语料并保留代表性约束。CEv1 身份记录绑定 2,232 文件语料，未以复制文件凑数。
- BR-R1-F2 → CLOSED：本次正式 20 对 P09 已分别记录 B0 原有 profile 和 S0 worker trace 的 CPU、RSS、读取量和阶段分布。
  B0/S0 worker CPU 中位分别 25,906/26,654.758 ms，读取量均为 501,391,380 bytes，RSS 上界最大 120.1/175.2 MiB；
  不再使用 P07 的 S0 样本替代。新问题 BR-R2-F1 是缺失数据防护缺口，不否定已报告的非零测量事实。
- BR-R1-F3 → CLOSED：append 按实际写入计算 delta，按两个 consumer × min(4096, 追加前大小) 声明 anchors；
  与 `ingest.Coordinator.Plan` 的校验后缀口径一致。三次实读 4,184,133 bytes，上界 1,040,696
  （65,064 + 975,632），明确报告超出 3,143,437 bytes；没有把基线超界掩饰成达标。

### 📝 总结

三项原 finding 均关闭；新增 BR-R2-F1 尚未关闭，因此复评 FAIL，Task Review 保持未勾选。
E1 基线无需提前满足后续优化目标，本轮也不因 P08 超界或 P09 未达到 30% 优化而判失败。

#### Evidence

- 已接受 Beads 修复交接 `81ee72c2-b443-5faf-835e-71342a7fedca`，复评期间 assignee 为 `opencode`、状态 `in_review`。
  工作区 `agent-deck.scan-performance` / `feature/scan-performance` 位于主仓库 `.worktrees/scan-performance`，
  common Git directory 与主仓库一致；未改变分支或迁移工作。
- 按既有配方复算 11 路径指纹，逐路径 blob 与 CEv1 ContentState 完全一致。
- 使用共享 `gate-status.cypher` 模板查询精确 WorkUnit/ContentState：三个 required criteria 均有
  `target_matches=true`、`applicable=true`、`malformed=false` 的 passing evidence；missing 和 unresolved 均为空。
  复用 `scan-perf-br-repair-20261025-{baseline-identity,baseline-matrix,cost-attribution}`。
  原 S0 的三个 passing evidence 已 superseded，Round 1 两条反证仍保留且不匹配本次目标。
- 门禁 VERIFIED 表示所记录的当前批次三项准则已有证据，不代替修复代码评审。合成缺失事件证明工具防护缺陷，
  不证明真实批次发生了同类丢失；因此未伪造真实批次失败或修改既有测量观察。
- 本轮复用当前状态的 E0/E1 结构化事实和修复交接验证；没有重跑性能批次、全量 Go 或 macOS 构建。
  新增验证仅为上述 stats 命令的隔离合成输入复现。源码、测试、配置保持只读。
- 文档验证：`check-review-records.py` 与 `git diff --check` 通过。`check-docs.py` 的 whitespace 前置检查
  被既有 `apps/macos/.build/index-build/` 生成内容阻塞；独立执行其 `scoped_errors` 后，仅报告本记录既有
  Repair candidate 中第 126 行缩进续行链接语法不受支持。本轮未改写历史修复交接，不声称全局 L0 或范围链接检查通过。

#### 下一步指令

修复：scan-performance / baseline-remeasure / BR-R2-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-26（Round 2 后）

Repairer：Claude Code 主会话；范围为 BR-R2-F1。Round 2 的评分、结论与 FAIL 保留；本节不是 Review Round，
不自行声明 PASS，待独立复评确认。未委派。

- BR-R2-F1 -> repaired in candidate：
  - **复现：** 修复前逻辑下，S0 collector 缺 `scan.end`、或 `scan.end` 缺 `read_bytes` 时 `p09ArmCost` 返回成功，
    读取量记为 0（下述回归在旧逻辑上这两例失败）。
  - **工具：** `cmd/agentdeck-perf/stats.go` `p09ArmCost` 对 S0 collector 的每个 `scan.end` 要求带 `read_bytes`，缺字段即
    返回错误；没有任何 collector `scan.end` 也判为不完整重建。两者都使该样本工具无效，不以零填补；记录的零值仍有效。
    B0 臂原已要求 profile 含 `ingestion.read_bytes`，未改。
  - **契约：** [performance-evaluation.md](../performance-evaluation.md) P09 行补充：S0 臂缺 collector 的 `scan.end`
    或该事件缺 `read_bytes` 即属缺本臂数据，不以零填补，记录的零值仍有效。
  - **回归：** `TestP09S0ReadBytesMustBeRecorded` 四例：缺事件、缺字段均为工具无效 1；记录的零值、完整输入均为
    工具无效 0 且读取量分别为 0、400。临时把判断退回修复前逻辑时，前两例失败；恢复后通过。

受影响证据处理（用户 2026-10-26 选择“影响评估沿用”，不重测）：

- **内容状态：** 修复只改变 `stats.go`、`stats_test.go` 两个 blob，其余 9 个路径 blob 不变。按既有配方复算
  scoped fingerprint `fcbe95303bad1814aa1b0d058bbf7c4dddb4ac37268a23179087db8bb1b8cbd6`（HEAD 不变，
  `34a5ca1bfc8f9150ecdaacbff704ed6b66ebdde3`）；CEv1 ContentState `scan-perf-br-r2f1-20261026-state`。
- **为何沿用：** 改动只收紧离线 `stats` 的 P09 S0 校验，不改测量执行（run、驱动、产品）。r5 原始 trace 已随临时目录
  清理，不能用修复后的工具重算；依据是已记录的 E1 事实：20 个 S0 运行的读取量均为 501,391,380 bytes，与独立的
  B0 profile `read_bytes` 完全相等，缺 collector `scan.end` 或 `read_bytes` 只会让和变小，所以该批次已满足新校验，
  没有样本会转为工具无效。这是基于聚合事实的推断，不是用新工具对原始数据的重算，交由复评裁定。
- **CEv1：** `change` `scan-perf-br-r2f1-20261026-change`（旧状态 → 新状态，touched 两个文件），对三条
  `scan-perf-br-repair-20261025-*` 证据各记 `may_invalidate` 与一条 `impact_assessment`（outcome `preserves`，
  assessor 为本修复会话的模型评估）；新状态下三条 roll-up 证据 `scan-perf-br-r2f1-20261026-{baseline-identity,
  baseline-matrix,cost-attribution}` 记为 pass 并 `rolls_up` 原证据。原证据保持原状态不改写。

验证：

- `go vet ./cmd/agentdeck-perf` 通过；`scripts/run-go-test.sh ./cmd/agentdeck-perf` 通过（含新回归与
  `TestP09ReportsEachArmsOwnRebuildCost`）；`gofmt -l` 无输出。未重跑性能批次、全量 Go 测试或 macOS 构建：
  改动限于 `cmd/agentdeck-perf` 的离线统计。
- **CEv1 gate：** 规范 `gate-status.cypher` 查询 `scan-performance:baseline-remeasure` /
  `scan-perf-br-r2f1-20261026-state`：VERIFIED；missing 为空，unresolved 为空；reused 为三条 roll-up 证据；
  invalidated 只列出早已被取代的原 S0 证据。这是证据状态，不代替复评结论。

#### 下一步指令

复评：scan-performance / baseline-remeasure

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 3 — 2026-10-26

### 📋 baseline-remeasure 修复复评

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer：OpenCode / GPT-6 Astra。
- Method：独立冷上下文单人复评；接受修复交接，核对当前源码、旧内容指纹、CEv1 证据沿用与定向回归；未参与修复，未委派。
- Scope：BR-R2-F1 的 P09 S0 数据存在性校验、回归和受影响证据；复核 BR-R1-F1–F3 的关闭状态。
- Reviewed state：HEAD `34a5ca1bfc8f9150ecdaacbff704ed6b66ebdde3`；11 个非 docs 路径的 scoped fingerprint
  `fcbe95303bad1814aa1b0d058bbf7c4dddb4ac37268a23179087db8bb1b8cbd6`；
  CEv1 ContentState `scan-perf-br-r2f1-20261026-state`。
- WorkUnit：`scan-performance:baseline-remeasure`；Beads：`ad-scan-performance-baseline-remeasure`。
- 完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- BR-R2-F1 → CLOSED：[p09ArmCost](../../../../cmd/agentdeck-perf/stats.go#L779-L804) 对 collector 的每个
  `scan.end` 使用 map 存在性检查，缺 `read_bytes` 返回错误；没有 `scan.end` 也返回错误。
  `runStats` 将错误计入工具无效并跳过整对样本，合法零值仍保留。
- [四例回归](../../../../cmd/agentdeck-perf/stats_test.go#L260-L306) 分别覆盖缺事件、缺字段、记录零值和完整输入，
  并经本轮独立执行通过；既有两臂自身重建成本回归也通过。
- BR-R1-F1 → CLOSED（保持）：当前 P09 输入仍按用户决定使用批次绑定的冻结真实语料，不再设 5,071 文件下限。
- BR-R1-F2 → CLOSED（保持）：两臂自身资源与阶段数据沿用原 E1 观察，未回退到 P07 替代或前台 CPU 冒充 worker。
- BR-R1-F3 → CLOSED（保持）：P08 仍如实报告读取量 4,184,133、上界 1,040,696、超界 3,143,437 bytes。

### 📝 总结

全部四项历史 finding 已关闭，未发现本轮修复引入的阻塞问题。接受用户选择的影响评估沿用路径，
Review 勾选，任务进入待提交。该结论确认 E1 起点基线交付，不表示 S0 已达到后续性能优化目标。

#### Evidence

- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`，Git common directory 与主仓库一致。
  已接受修复交接 `2e4c6025-0c53-53d9-b7e5-04d2c6c61a53`；复评期间 Beads assignee 为 `opencode`、状态 `in_review`。
- 按本记录 Round 1 的既有配方复算候选，11 个路径 blob 与当前 ContentState 逐项一致。
  其余 9 个路径与上一轮完全一致；旧未提交 blob 未保存在 Git 对象库中，因此未声称成功读取旧 Git 对象。
  在内存中撤去当前 `stats.go` 的存在性校验得到旧 blob `b9e947ce50380e62add4d6c2694474f2c36ddb13`；
  撤去新测试及其注释得到旧测试 blob `b93b941c627e63e51fd9d747dd813fc06ca583fa`，均匹配上一 ContentState。
  这确认此次代码差异仅为所述校验与测试，未写回产品文件。
- 独立执行 `scripts/run-go-test.sh ./cmd/agentdeck-perf -run 'TestP09(S0ReadBytesMustBeRecorded|ReportsEachArmsOwnRebuildCost)$'`：
  两个测试及新增测试四个子例全部通过。复用修复交接记录的包级测试与 vet；未重复全量 Go、macOS 构建或性能批次。
- 沿用判断：P09 每臂从独立空状态运行一次 scan，见 [run.go](../../../../cmd/agentdeck-perf/run.go#L899-L938)；
  `round.executeWith` 在 once 内调用一次 `markEnd`，且启用 trace 时初始化 metrics；`markEnd` 输出其 `read_bytes`。
  该执行路径及其 blob 未变。原 E1 记录两臂读取量为 501,391,380 bytes，修复交接确认 20 个 S0 运行一致，
  因而不是上一轮复现的“缺失而默认零”情形；新的存在性检查不改变这些已记录计数或阶段统计。
  此判断限定于该批次和已核验的单轮执行路径，不把任意聚合非零值当成任意多事件 trace 完整性的通用证明。
- 原始 trace 已清理，未用新工具重算。沿用依据为持久化测量事实、未变的执行路径和明确影响评估，
  不是原始样本重新检查；保留这一可复核性局限。不存在实际批次丢失事件的新证据，不伪造批次失败。
- 读取三条 `scan-perf-br-r2f1-20261026-impact-*` 及原 `scan-perf-br-repair-20261025-*` 测量事实；
  使用共享 `gate-status.cypher` 查询精确 WorkUnit/ContentState，结果 VERIFIED。
  `baseline-identity`、`baseline-matrix`、`cost-attribution` 三项 required criteria 齐全；三条当前 roll-up 均
  `target_matches=true`、`applicable=true`、`malformed=false`；missing/unresolved 为空。
  invalidated 仅列原始 S0 已被取代的三条观察。未改写历史证据，也未新增重复测量记录。
- CEv1 当前没有该 Task 的 `contains` 父关系；任务矩阵明确后续仍有 Task 3–8，因此不跨越或声明 Topic 完成边界。
- 文档验证：评审记录结构检查与 `git diff --check` 通过。`check-docs.py` 的 whitespace 前置检查仍受既有
  `apps/macos/.build/index-build/` 生成文件阻塞；单独调用 `scoped_errors` 检查本轮两份文档，仅报告历史 Repair candidate
  第 126 行的缩进续行链接语法不受支持，未报告本轮新增内容错误。不声称全局 L0 或整个历史记录链接检查通过。

#### Task checkpoint：baseline-remeasure

- 内容状态：`scan-perf-br-r2f1-20261026-state`；完成门禁 VERIFIED。
- 提交建议：仅本 Task 的 11 个产品/工具及测试路径、需求/测量契约修订、任务矩阵与本评审记录；提交前核对实际暂存边界。
- 推送建议：已配置上游 `origin/feature/scan-performance`；待用户明确授权提交与推送，并核验待推送提交范围及交付前提。
  本轮未执行 Git 交付。

#### 下一步指令

开发：scan-performance / serving-open-path

WORKFLOW_WORKSPACE: agent-deck.scan-performance
