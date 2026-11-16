---
status: active
topic: scan-performance
subject: architecture.md
---

## Round 1 — 2026-10-09

## 📋 架构契约评审

📊 总体评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

**A1-F1 · P1 · OPEN — 默认有限模式仍把 publication 放在扫描终结之后。**

- 位置：`architecture.md:86–90`，对照 `architecture.md:30–46,299–301`。
- 行为风险：按“默认有限模式”实现，会继续等待整个历史 round 完成，首次当天优先的收益不能到达 App。
- 证据：同文前段要求安全提交点发布 partial、reader 不等全局 worker；默认模式却写“扫描终结后原子发布 serving 文件”。当前 `EmbeddedHelperRunner.swift:1920–1926` 也只在 host.refresh 返回后 publishSuccess，必须明确新通知路径如何避开此等待。
- 💡 修复：统一为默认有限模式的 source/domain 已提交检查点发布；明确发布触发、通知与 terminal 的区别、reader 不等待 worker 的入口，以及 CLI 完整终结仍按原范围等待。

**A1-F2 · P1 · OPEN — 首批 coverage 没有落到 stored-only CLI 的读取契约。**

- 位置：`architecture.md:198–217`，对照 `ux/cli.md:16–25`。
- 行为风险：App 首批只导入今天后，另一个 CLI 查询历史会把不完整库输出成 `partial:false`，即使 GUI 的 serving metadata 正确。
- 证据：coverage 供给表只规定 view/domain serving 与 receipt；没有规定任意 DB 查询如何获得该 inventory 的持久覆盖状态。当前 `cmd/agentdeck/main.go:3853–3870,3889–3916` 的 envelope partial 取决于本次 scanErr；`--no-scan` 跳过 scan 后该值为 false，不会自动继承 App 首批 coverage。已有 pricing warnings 不是来源覆盖证明。
- 💡 修复：定义与 source 提交一致的持久 coverage/watermark、范围查询判定及 text/JSON warning/partial 映射；将其提供给 stats/summary 的 stored-only 路径，且不引入扫描。明确 v1 消费者兼容和任务承接。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

全 inventory 保留、跨库 generation vector、shadow source 原子性、未知字段和精确请求去重的原则清楚；额外常驻进程有独立采用条件。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；用户指定 actor `opencode`。
- Method: 独立主会话设计评审；CodeGraph 绑定本 worktree，直接核对未被索引覆盖的候选与 CLI 分支；未委派。
- Scope: 全架构及 requirements、三份 UX、performance-evaluation、tasks 的接口与顺序依赖。未评审尚不存在的实现为 PASS。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `7426ed03980b7a41614a69649ef248d6ac32a342`。
- ContentState: `scan-performance:state:4039767e872fea306adb2d71d5c3fe7a42ac1858bd19087503c07e89ad0b1ede`；SHA256(`head=<HEAD>;document=<blob>`)。
- Evidence: `bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace` 退出 0；上述源码分支与文档矛盾为静态证据，不以新产品运行结果冒充证明。
- Completion gate: FAILED
- Gate evidence: Neo4j MCP 标准 gate-status 查询绑定上述 ContentState；document-integrity 通过，contracts-and-consumer-consistency 被本轮适用 fail evidence 否定；无 invalidated 或 unresolved evidence。
- 下一步指令：修复：scan-performance / reviews/architecture.md / A1-F1、A1-F2

## Repair candidate — 2026-10-09

Repairer: Codex；范围仅为上述Round 1的授权问题。原Round 1的分数、Verdict、ContentState与FAILED门保留。

- A1-F1：默认有限模式在source/domain提交点发布，独立通知/reader入口不等全局terminal/worker锁。 **REPAIRED in candidate，待独立复评确认**。
- A1-F2：持久inventory/domain coverage、任意stored-only范围判定、v1 partial/warnings与任务承接。 **REPAIRED in candidate，待独立复评确认**。

本候选不创建Review Round 2，不自行声明Review PASS。文档/标本的当前内容身份、
验证结果与修复目标证据状态在本节完成验证后绑定；旧观察不重新标成新状态。

修复目标绑定：HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；document blob `3cb0d75f50c41e7ba8f545fb33e58fd59635cfc9`；
repair_target_state: `scan-performance:state:4ac50212992e065f1265b6e4d50fd35529f0af94879668ee420b9d92a3841020`。
验证：topic文档集、local links、review-record parser、whitespace、diff检查通过；
prototype build通过；120项mixed/telemetry/CLI合成交互通过，17截图在900×1500视口生成。
真实规模性能harness/摘要未改，未重扫私人数据、未测browser/native性能；产品Go/Swift未改。
证据同步：15个精确目标/证据节点读回全匹配，20关系预检ok并写入；标准gate-status查询
在上述新目标返回 `NOT_VERIFIED`，仅缺独立复评准则 `scan-performance:architecture.md:contracts-and-consumer-consistency`。
document-integrity有当前pass；没有invalidated/unresolved impact。独立设计准则记not_verified，
作者不把修复当Review PASS，旧Round1 fail观察/关系未覆盖或重标。
依赖影响：requirements/performance-evaluation的blob保持原PASS身份，性能harness与三份数据摘要未改；
本次仅修复下游契约和合成UX，没有改需求目标或测量归因，复用其原结论。

## Round 2 — 2026-10-09

## 📋 架构契约复评

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

- A1-F1 CLOSED：`architecture.md:89–102` 明确默认有限模式在 source/domain 提交点发布，独立订阅不等待 scanIndexes、worker 释放或 host.refresh；terminal 仅结束请求/发布最终覆盖。
- A1-F2 CLOSED：`architecture.md:236–257` 定义 inventory、source/domain checkpoint、coverage heads，与事实同提交；未完成来源不能仅靠 mtime 排除，旧库 unknown，CLI warnings/partial 与原有状态取 OR。Task 2 承接落地，Task 5 验并发水位。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`。Method: 原独立评审者核对他方修复，逐 finding/消费者对照；未参与修复、未委派。
- Scope: 两项架构 finding、CLI 对应契约、任务承接与 Settings unknown 字段供给。Round 1 历史事实保持不变，无新增或回归问题。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`，blob `3cb0d75f50c41e7ba8f545fb33e58fd59635cfc9`；ContentState `scan-performance:state:4ac50212992e065f1265b6e4d50fd35529f0af94879668ee420b9d92a3841020`。
- Evidence: 入口标准 MCP gate 仅缺独立设计准则，当前 document-integrity pass 可复用且无失效/未决影响；59 项 manifest 哈希全匹配。当前源码事实仍沿用 Round 1 已核验的 HEAD，新增内容是拟实现契约，不宣称已运行。
- Completion gate: VERIFIED
- Gate evidence: 本轮标准 Neo4j MCP gate-status 在上述目标返回 VERIFIED；两项 required criteria 均有适用 pass，missing/invalidated/unresolved 均为空。结构证据复用修复轮，独立设计证据为 rereview-r2。
- Residual limits: 持久 coverage、并发、生命周期和 native 性能仍须实现后按 Task 2/5 验收。全部七份文档的通过不是产品实现完成。

Task checkpoint：ad-scan-performance-doc-arch；content_state `4ac50212992e065f1265b6e4d50fd35529f0af94879668ee420b9d92a3841020`；gate VERIFIED。

提交建议：将该文档、关联契约修复和复评记录纳入完整设计批次的授权提交。

推送建议：feature/scan-performance；须核验远端及分别获得提交/推送授权。

## Round 3 — 2026-10-22

## 📋 全 topic 重新设计：架构评审

📊 总体评分：8/10

✅ 评审结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

[architecture.md:33] SP-ARCH-R3-F1（中）：删除 `prototype/tools/scan-performance-*` 会破坏已评审的 UX 标本身份
- 行为风险：`ux/prototype/serving/manifest.json` 以路径和 SHA-256 固定了 8 个 `prototype/tools/scan-performance-*` 文件，`prototype/README.md:22–29,327` 也把它们写成标本 harness。tasks.md 又声明 UX 标本“无变化、沿用原评审”。按本表删除后，manifest 指向不存在的文件，三份 UX 评审所依据的标本身份随之失效。
- 证据：manifest.json:206–234；`fd scan-performance prototype/tools` 共 8 个文件。
- 💡 有界修复：明确处置——要么把 prototype tools 留作标本历史，从删除范围中排除；要么在删除的同时更新 manifest/README，并按规则重开受影响 UX 文档的 Review。此处与 tasks.md 同步修改。

### 🟡 建议改进 — 推荐

[architecture.md:101–102] SP-ARCH-R3-F2（低）：下限探测阈值被表述为“判定不可达”。问题同 performance-evaluation SP-PE-R5-F7，按同一修复同步本处。

[architecture.md:530] SP-ARCH-R3-F3（低）：Go `perftrace` 自报 `ri_lifetime_max_phys_footprint` 缺少无 CGO 的取数机制。问题同 SP-PE-R5-F5，需在产品侧接口一并写明。

### 🟢 优点

当前代码表中的事实已逐条核实：`togglePopover`（MenuBarItemController.swift:178）、`popoverPreparation`、FSEvents latency 0.1、`parallelDecodeWorkers = 4` 及 1 MiB 阈值（64 MiB / 64）、`temp_store` 内存模式、`scanInventory → scanFileMode` 串行、`session.scanMode/scanOne`、`AcceptInventory` 单事务循环。并行 reduce 只做纯计算、跨 source 判定留在有序写者内，这一边界让 worker=1 等价性可以检验；短事务对 append 与 rewrite 分别处理，没有让查询看到半个重建。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，冷读并直接核对 Swift/Go 源码；无委派，无产品修改。
- Scope：architecture.md 中本次改动的章节（当前代码表、系统结构、打开路径、启动顺序、流水线、短事务、测量打点），以及它们与其余已评审契约的一致性。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `28f1686684e08d0756adb6f6d549a16864b4bcb1`；digest `93ff018b15136f95cbddd94442a2abce901222e169de347b0f99736248fcd73c`。
- Evidence：上述源码位置的 rg 检索；manifest/README 引用检索；`scripts/check-docs.py` 未报告本文问题。
- CEv1：WorkUnit `scan-performance:architecture.md` 仍绑定旧 ContentState，当前 digest 无记录，门禁 NOT_VERIFIED。
- 结论：一项中严重度、两项低严重度问题未关闭，判 FAIL。

## Repair candidate — 2026-10-22

Repairer：Claude Code 主会话；范围仅为 Round 3 的 SP-ARCH-R3-F1..F3。Round 3 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS。各项均待独立复评确认。

- SP-ARCH-R3-F1 -> repaired in candidate：选择评审给出的第一种处置。`prototype/tools/scan-performance-*` 从删除行拆出，
  单列为“保留为已评审 UX 标本的一部分，不删除、不修改，也不作为测量工具”；manifest 与 UX 评审身份因此不变，
  不需要重开 UX Review。tasks.md 同步。
- SP-ARCH-R3-F2 -> repaired in candidate：打开路径第 2 步改为与性能评估 E2 相同的两档表述，并链接 E2；
  其后“若第 2 步判定不可达”改为“第 2 步停止后用户选择改用其他窗口形态”。
- SP-ARCH-R3-F3 -> repaired in candidate：Go `perftrace` 改为以标准库 `syscall.Getrusage(RUSAGE_SELF)` 自报 `ru_maxrss`
  与 CPU，不需要 CGO，不读取 `phys_footprint`；口径与性能评估 P07 一致。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `10903444fa5815be3634e25f689e059127b37574`；
digest `e886369d0dd1f67b3f72af153ba10e2b17fbd5a1b2cf4cebb1bc87ba6cefde9d`。
验证：`python3 scripts/check-docs.py` exit 0。
范围外观察（未修改，交复评判断）：architecture.md 第 17 行与第 331 行仍以“Task 4 候选”指旧 `bounded-source-pipeline`
候选，与 SP-TASKS-R5-F5/SP-PE-R5-F8 同类，但不在 Round 3 已记录的发现内。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 4 — 2026-10-22

## 📋 全 topic 重新设计：架构复评

📊 总体评分：9/10

✅ 复评结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[architecture.md:17,331] SP-ARCH-R4-F1（低，新）：“Task 4 候选”仍指旧 `bounded-source-pipeline`
- Disposition：new。这是 SP-PE-R5-F8、SP-TASKS-R5-F5 同类的问题，Round 3 漏记；修复者已在附记中指出，但没有修改。
- 行为风险：新矩阵中 Task 4 是尚未开始的 `parallel-source-pipeline`；“已保留的 Task 4 候选”会被读成新任务的产物。
- 💡 有界修复：两处都改为旧 `bounded-source-pipeline` 任务的候选。

### 🟢 优点

- SP-ARCH-R3-F1 closed：`prototype/tools/scan-performance-*` 单列为保留的 UX 标本，不删除、不修改，manifest 固定身份因此保持有效；tasks.md 同步排除。
- SP-ARCH-R3-F2 closed：打开路径第 2 步改为两档表述并链接 E2，UX 变更条件与之一致。
- SP-ARCH-R3-F3 closed：Go `perftrace` 以 `syscall.Getrusage` 自报 `ru_maxrss` 与 CPU，不需要 CGO，与性能评估 P07 口径一致。

### 📝 总结

- Reviewer/Method：Claude Code 主会话（Round 3 评审者），核对当前正文；无委派，无产品修改。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `10903444fa5815be3634e25f689e059127b37574`；digest `e886369d0dd1f67b3f72af153ba10e2b17fbd5a1b2cf4cebb1bc87ba6cefde9d`。
- Evidence：`check-docs.py` exit 0；`rg "Task ?[0-9]"` 命中第 17、331 行。
- CEv1：FAIL，不写入，门禁 NOT_VERIFIED。
- 结论：3 项均已关闭，但新增 1 项低严重度问题，判 FAIL。

## Repair candidate — 2026-10-22（Round 4 后）

Repairer：Claude Code 主会话；范围仅为 Round 4 的 SP-ARCH-R4-F1。Round 4 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS，待独立复评确认。

- SP-ARCH-R4-F1 -> repaired in candidate：“当前代码与改变的位置”与“流水线、重读与发布”两处的“Task 4 候选”均改为旧
  `bounded-source-pipeline` 任务的候选；全文 `rg "Task ?[0-9]"` 已无命中。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `44a2b088364b56f9098c7a6b658ecc902832b9ee`；
digest `152ed9217ab91842fbcc69e45a2b0b46706acc971d4ab3faf42041a78b97955b`。
验证：`python3 scripts/check-docs.py` exit 0；`git diff --check` 通过。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 5 — 2026-10-22

## 📋 全 topic 重新设计：架构复评

📊 总体评分：10/10

✅ 复评结论：PASS

完成门禁：BLOCKED（CEv1 provider 本会话不可达）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-ARCH-R4-F1 closed：第 17、331 行均改为旧 `bounded-source-pipeline` 任务的候选；全文 `rg "Task ?[0-9]"` 无命中。
- SP-ARCH-R3-F1..F3 在当前内容中未回归。

### 📝 总结

- Reviewer/Method：Claude Code 主会话（Round 3/4 评审者），核对当前正文；无委派，无产品修改。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `44a2b088364b56f9098c7a6b658ecc902832b9ee`；digest `152ed9217ab91842fbcc69e45a2b0b46706acc971d4ab3faf42041a78b97955b`。
- Evidence：`scripts/check-docs.py` exit 0；`scripts/check-review-records.py` exit 0；`git diff --check` 通过。
- CEv1：Neo4j MCP 本会话连接失败，未查询、未写入；门禁 BLOCKED，待 provider 恢复后按上述 digest 记录并重查。
- 结论：唯一发现关闭，判 PASS；完成门禁未 VERIFIED。

## Round 6 — 2026-10-23

## 📋 冻结真实语料夹具变更：架构评审

📊 总体评分：10/10

✅ Verdict: PASS

完成门禁：VERIFIED（CEv1 scan-perf-corpus-r-20261023-arch-state，两项 required criteria 均通过）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 本轮唯一改动（第 537 行）把 `cmd/agentdeck-perf` 职责改为“冻结真实语料与按锚定日平移（测量夹具）、合成夹具（仅测试）”，与 performance-evaluation.md 夹具一节、tasks.md `measurement-harness` 交付项一致；仍声明不进入发布打包。
- 模板拷贝方式与现有实现一致：`cmd/agentdeck-perf/template.go:117` 使用 `VACUUM INTO`。
- SP-ARCH-R3/R4 已关闭的发现在当前内容中未回归。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，核对本轮 diff 及其与性能评估、任务矩阵、现有 `cmd/agentdeck-perf` 代码的一致性；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `a2b7c6e2b37845573f3701b6c9bd2ad6807a2f46`；digest `f7333dc22ec7d2526b5b427b0beaafe6af0ee03f71c6035727d0de50de55d56e`。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 依赖：performance-evaluation.md 本轮 FAIL（SP-PE-R8-F1..F3），其修复只涉及测量契约，不改变本文件的职责划分；若修复改动本文件，本结论失效并须重评。
- 结论：无发现，判 PASS。

## Round 7 — 2026-10-26

## 📋 E2 暖启动与追加探测架构评审

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 首次窗口暖启动保持候选身份，明确不闪现、不夺焦点、不触发 helper／数据读取，并规定暖启动期间的点击先结束暖启动再走正常打开路径；不会仅凭静态文本下限宣布产品验收通过。
- popover 与 panel 使用相同候选约束；候选不可行与工具无效分开，缺少有效测量时停止，与 performance-evaluation.md Round 12 已评决策出口一致。面板仅用于探测，采用仍须菜单栏 UX 评审。
- 保留同一 hosting 层级、关闭不销毁 content；启动恢复、内容最小化与切换优化的实施边界明确。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立增量架构评审，CodeGraph 定位后核对当前源码、依赖契约及失败／点击交错场景；无委派，无产品修改。
- Scope：新增“首次窗口暖启动与追加探测”及其与打开路径、启动顺序、性能契约和任务范围的衔接。与 Round 6 对比，实质新增为本节；冻结真实语料工具职责沿用 Round 6 结论，其他未变契约不重做全面审计。SP-ARCH-R3-F1..F3、SP-ARCH-R4-F1 保持关闭，无新增发现。
- Reviewed state：HEAD `a427d97bb760d775e9762fc8c0d14522aec63888`；document blob `5ddf672966cd8aac77e57f0b9270733dd4191497`；SHA256(`head=<HEAD>;document=<blob>`) = `b61ea1ccc9693e8b30f191d9e1cf2b1633cad627a8dbfc553ab95b87f1aab690`。
- Dependencies：performance-evaluation blob `b0c0e345f8c47998189a7ad2d21e93c2bc7d2694`（Round 12 PASS，独立 L0 门仍阻塞）；requirements blob `315b3305c39c42d894be91b69310b434bc683630`。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。
- Source evidence：`MenuBarItemController.swift:232–256` 在未上屏借用窗口中布局，未调用 popover.show；`194–215` 在首次打开调用 show 后才激活窗口，`52–60` 保持 transient／无动画且 Perf 下可替换静态内容；`264–265` 关闭仅调用 performClose。以上支持设计对现有路径的描述，不证明 AppKit 内部资源创建时刻或暖启动性能。
- Runtime limits：不可见暖启动能否成立、同步 show 是否满足原有单片 ≤8 ms 预算、关闭后窗口资源保留、真实数据 content 的副作用、点击与暖启动结束的交错，以及 helper 调度是否改善争用，均需任务实现与实验验证。设计未给这些实验结果签发 PASS，也不豁免原有预算、生命周期或候选采用规则。
- Evidence：当前 `python3 scripts/check-docs.py` exit 1，在 `make check-whitespace` 因既有未跟踪 `apps/macos/.build/index-build` 生成文件缺尾换行／尾空白停止，链接阶段未执行。最终 topic 文档集、评审记录结构及 scoped diff hygiene 单独检查；不把这些检查代替完整 L0。未清理他人的构建产物。
- CEv1：WorkUnit `scan-performance:architecture.md`；ContentState `scan-perf-arch-r7-20261026-state`；语义观察 `scan-perf-arch-r7-20261026-review` 为 pass，结构前置限制 `scan-perf-arch-r7-20261026-integrity` 为 blocked。旧状态的通过证据不直接改绑。其他文档／实现任务仍未完成，不查询 topic 完成门。
- 结论：本次架构增补 PASS；矩阵 architecture Review 勾选；document-integrity 仍阻塞，Beads 保持 in_review，不进入 awaiting_commit。

Task checkpoint：ad-scan-performance-doc-arch；content_state `b61ea1ccc9693e8b30f191d9e1cf2b1633cad627a8dbfc553ab95b87f1aab690`；gate BLOCKED。
提交建议：处理构建产物引起的 L0 阻塞、补齐 document-integrity 且门禁 VERIFIED 后，授权提交架构增补、其评审记录与必要矩阵变更；不混入产品实现。
推送建议：同一 L0 前提解决、门禁 VERIFIED 后，经提交范围、贡献归属、trailer、SSH 签名核验和独立授权，推送 origin/feature/scan-performance；远端当前状态未核验。

后续前提：处理 apps/macos/.build/index-build 引起的 L0 阻塞，完成 check-docs.py 并补齐本内容的 document-integrity 证据与门禁；不重复已完成的架构评审。

### L0 与门禁同步 — 2026-10-26

用户授权处理文档检查阻塞。本轮最初的 BLOCKED 与原始失败原因保留为历史；上方单一完成门禁字段更新为本次最终查询结果，语义 PASS 未重新评审。

- `.gitignore` 新增精确的 `apps/macos/.build/` 规则；目录没有被跟踪的文件，未删除或改写构建产物。
- 仅修正 performance-evaluation 与 baseline-remeasure 评审记录各一处检查器不支持的缩进续行链接，改为明确文件名／章节文本；历史结论、观察与 finding 含义未变。
- 最终 `python3 scripts/check-docs.py` 退出 0，覆盖空白、diff、文档集、链接与评审结构；产品代码／测试／配置未改（忽略规则除外）。
- 当前 HEAD `67ac8b7da27f581a58d4014a7b218321fcd1042f`；正文 blob 与本轮已评版本一致。新 digest `4bcfaf2ccda7fb00a4347b99106fba3b085930c6664b4752c11c6827c02d85fd`，ContentState `scan-perf-doc-l0-20261026-arch-state`；新 L0 证据与显式目标绑定的语义 roll-up 分别为 `scan-perf-doc-l0-20261026-arch-integrity`、`scan-perf-doc-l0-20261026-arch-review`。
- 原观察保留原身份；只评估本次 HEAD、忽略规则与记录格式变更对语义结论没有影响。旧 blocked 观察不改写，新状态不继承旧环境阻塞。
- 本段取代上方 checkpoint 的阻塞前提：门禁 VERIFIED；文档本体已在此前授权提交中交付，本次忽略规则及记录同步尚未提交，因此协调转 awaiting_commit，暂不关闭任务。不执行提交或推送。

## Round 8 — 2026-10-27

## 📋 启动争用削减架构评审

📊 总体评分：7/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

[architecture.md:146–154] SP-ARCH-R8-F1（高）：复用键不足以支持整份 serving payload 的复用和逐字节等价承诺。

- 处置：new，open；属于新增候选 1，不重开历史已关闭发现。
- 行为风险：同一天且数据库 generation 不变时，复用旧 payload 会保留已过期的额度新鲜度或时间相关投影；更新 coverage 头不能更新 payload 中的状态。
- 证据：`internal/desktop/desktop.go:259–297` 的 Build 每次写入新的 GeneratedAt、NextRefreshAt，并加载 subscription、sessions、health；不是只读 generation 决定的纯函数。`internal/desktop/subscription.go:263–265` 根据 now 计算 Stale；`internal/quota/model.go:107–148` 的阈值为 max(窗口长度的 1/10, 两倍 probe interval)，跨阈值不要求数据库写入。比如同日 300 分钟窗口、5 分钟 probe interval，观察后第 29 分钟与第 31 分钟的 generation 可相同，但完整重建的 Stale 分别为 false 和 true，旧 payload 复用无法等价。
- 相关边界：`internal/desktop/derived_cache.go:117–143,158–183` 已声明 ValidUntil，取下一小时、本地日和未来价格生效时间的最早值；新增候选只核对本地日，且跳过 Build，不能借用原派生缓存读取时的有效性检查。`internal/desktop/serving_cache.go:232–238` 将完整 Snapshot 和生成时间写进 payload，故即使业务值不变，跨时刻“逐字节相等”也不成立。
- 💡 有界修复：列出可复用的稳定投影与仍需刷新／重新校验的时间、额度、健康等字段，定义复用的有效期及失效出口；明确完整比较 oracle 的同一时钟／请求条件或精确的归一化字段范围，不能仅以 generation 与本地日相同替代全 payload 等价。补上同日跨额度 stale 阈值、小时和价格生效边界的验证要求；保持不改 schema/revision 触发器的范围约束。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- quiet 对照用于确认争用后才启动优化，正式 P01 口径仍由已评性能契约约束。
- 无变化扫描若改变 generation，要求先调查并在需要改 schema／触发器时返回设计，避免无边界扩大优化。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话，非架构增补作者；Method：独立增量设计评审，以当前工作区 CodeGraph 的逐行源码核对复用输入、时间有效性及发布路径，无委派。
- Scope：2026-10-27 争用对照与两个削减候选及受影响契约。发现候选 1 的确定性阻断后停止扩大验证；候选 2 的跨进程调度、运行时收益与实现正确性未完成认证。本轮不运行性能批次、不修改产品。
- Reviewed state：HEAD `78ad48dcfaffa65a4f116f443897a668957764be`；architecture blob `6d3d0f16a3013abca3b306899916fe23674746f5`；SHA256(`head=<HEAD>;document=<blob>`) = `597a5cf79ddc84bc931b69c43abd997eeef46b03c3ebe267870c502cf7752cff`。
- Dependencies：performance-evaluation blob `7a17d7fa58017793a01c090145ec20051a70906a`（Round 16 PASS、交付门 VERIFIED）；requirements blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`。工作区 `agent-deck.scan-performance` / `feature/scan-performance`；相关源码为本工作区当前内容。
- Evidence：上述源码静态场景构成反例；`python3 scripts/check-docs.py` exit 0，覆盖空白、diff、链接、受影响文档集与评审结构。历史 SP-ARCH-R3-F1..F3、SP-ARCH-R4-F1 保持原关闭处置。
- CEv1：WorkUnit `scan-performance:architecture.md`；本轮目标 `scan-perf-arch-r8-20261027-state`。required criteria 为 document-integrity 与 contracts-and-consumer-consistency；新观察 `scan-perf-arch-r8-20261027-integrity` pass、`scan-perf-arch-r8-20261027-review` fail；关系预检 4/4 ok，写入 4 条关系，精确目标门禁 FAILED。旧观察不直接改绑。
- 结论：FAIL，architecture 的 Documents Review 保持未勾选，同一 dispatch 退回修复；不跨越 topic 完成边界。

修复：scan-performance / reviews/architecture.md / SP-ARCH-R8-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-27（Round 8 后）

Repairer：Claude Code 主会话；范围为 SP-ARCH-R8-F1。Round 8 的评分、结论与 FAILED 门保留；本节不是 Review Round，
不自行声明 PASS，待独立复评确认。未委派。

- SP-ARCH-R8-F1 -> repaired in candidate：
  - 候选 1 从“generation 与本地日相同则不调用 `Build`、复用整份 payload”改为“发布段每次调用 `Build`，只经由已有
    派生快照缓存复用 usage 摘要/presentation 与 work signals”。不新增复用键，不扩大有效期，不跳过 `Build`。
    源码依据（只读核对）：`internal/desktop/desktop.go` 的 `Build` 每次按时钟写 `GeneratedAt`/`NextRefreshAt`，并加载
    provider、subscription、sessions、health；`derived_cache.go` 的 `nextDerivedSnapshotValidityBoundary` 与键比较。
  - 明列每次按当次时钟重建、不复用的字段：生成与下次刷新时间、provider、subscription/额度新鲜度、sessions、health、
    warnings、coverage 头。可复用投影沿用缓存已评审的键与 `ValidUntil`（下一整点、下一本地日、未来价格生效时间
    三者取最早），任一不符或到期即重算。
  - 本候选只处理两类重复计算：无事实变化的写入把派生 generation 置 dirty 或推进 revision（只在不改 schema 与
    触发器时消除，否则返回设计）；同一 derive 段内 `PublishDerivedSnapshotCache` 与 `Build` 重复计算同一投影。
    合法失效照常重算；若重算全部来自合法失效，本候选无余量，转候选 2。
  - 正确性 oracle 改为：同一数据库、同一注入时钟、同一请求下，缓存命中与强制重算的 serving payload 逐字节相等；
    不跨时刻比较。
  - 补齐边界验证：同 generation、同日额度跨 stale 阈值前后（300 分钟窗口、5 分钟 probe，第 29 与第 31 分钟）
    `Stale` 分别为 false 和 true；跨整点、跨未来价格生效、跨本地日、dirty 或 revision 变化时重算；sessions 与 health
    每次重新加载。
  - tasks.md 第 3 项的文件边界与正确性回归中引用旧复用说法的两处同步改为上述口径；任务范围、准则与样本数未改。

修复目标：HEAD `78ad48dcfaffa65a4f116f443897a668957764be`；architecture blob `b960650b007556750d6389681035173da110a182`；
tasks blob `b0e274fca0474e226199ac2a40d37f46f30b2cce`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / architecture.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 9 — 2026-10-27

## 📋 启动争用削减架构复评

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED（CEv1 scan-perf-arch-r9-20261027-state，两项 required criteria 均通过）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-ARCH-R8-F1 -> closed。候选 1 不再以 generation 与本地日相同复用整份 payload：发布段每次调用 `Build`，
  逐项列出按当次时钟重建的字段（生成与下次刷新时间、provider、subscription／额度新鲜度、sessions、health、
  warnings、coverage 头），可复用部分只经由已有派生快照缓存。源码核对：`internal/desktop/derived_cache.go`
  的 `derivedSnapshotCachePayload` 只含 presentation、summary、work signals 与价格可用性；
  `derivedSnapshotCacheHeaderMatches` 比较的键与 `nextDerivedSnapshotValidityBoundary`（下一整点、下一本地日、
  未来价格生效时间取最早）与文中列举一致；`desktop.go` 的 `Build` 在缓存命中时仍执行 `loadProvider`、
  `loadSubscription`、`loadSessions`、`loadHealth`。因此 Round 8 的同日跨 stale 阈值反例不再成立：`Stale`
  属于每次重建的 subscription，不在缓存载荷中。
- 等价 oracle 收窄为同一数据库、同一注入时钟、同一请求下缓存命中与强制重算的 serving payload 逐字节相等，
  不跨时刻比较；`serving_cache.go` 中随机 `PublicationID` 位于 Header 而非 Payload，固定 `Service.Now` 时
  Payload 的 `GeneratedAt` 一致，oracle 可执行。边界验证覆盖 stale 阈值（29／31 分钟）、整点、价格生效、
  本地日、dirty／revision。
- 范围受控：不新增复用键、不扩大有效期、不改 schema／revision 触发器（需要时返回设计）；若重算全部来自
  合法失效即无余量并转候选 2，出口完整。tasks.md 第 3 项文件边界与正确性回归的引用同步为新口径，与本文件一致。
- 候选 2 的落点与现有代码对应：`EmbeddedHelperRunner` 以 `executableURL` 启动 helper（可设
  `Process.qualityOfService`），扫描 worker 由 `internal/scanruntime/scanruntime.go` 以独立 `scan-worker`
  子进程拉起；无缓存／空库回合保持默认优先级，P02/P04/P11 不回归作为判据。其运行时收益仍待实施与批次验证，
  本设计不对其签发性能结论。

### 📝 总结

- Reviewer：Claude Code 新会话（冷上下文，非 Round 8 修复会话，未继承其上下文）；Method：独立增量复评，逐条核对
  SP-ARCH-R8-F1 的有界修复要求与当前源码（`derived_cache.go`、`desktop.go`、`serving_cache.go`、
  `scanruntime/serving.go` 与 `scanruntime.go` 的 derive 段、`EmbeddedHelperRunner`），并复查候选 2 的落点；
  无委派，无产品修改。
- 处置：SP-ARCH-R8-F1 closed；历史 SP-ARCH-R3-F1..F3、SP-ARCH-R4-F1 保持关闭，未回归；无新增发现。
- Reviewed state：HEAD `78ad48dcfaffa65a4f116f443897a668957764be`；architecture blob
  `b960650b007556750d6389681035173da110a182`；SHA256(`head=<HEAD>;document=<blob>`) =
  `8a80d3605f2a7ae6d38a55a2886f86ab3157ed610bcc7bf6761e69531d946c5b`。
- Dependencies：tasks blob `b0e274fca0474e226199ac2a40d37f46f30b2cce`（本轮只核对与本文件的一致性，tasks.md
  自身的评审另行进行）；performance-evaluation blob `7a17d7fa58017793a01c090145ec20051a70906a`；requirements
  blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check -- docs` exit 0；
  `python3 scripts/check-docs.py` exit 0（含本轮记录）。
- Residual：派生缓存命中后剩余的发布段耗时（sessions／health／subscription 加载）是否仍构成争用，以及候选 2
  的调度收益，只能由实施与 `P01-floor-warm` 批次判定；设计已为两者给出出口。
- CEv1：WorkUnit `scan-performance:architecture.md`；目标 `scan-perf-arch-r9-20261027-state`；新观察
  `scan-perf-arch-r9-20261027-integrity`、`scan-perf-arch-r9-20261027-review`，均为 pass；关系预检 4/4 ok，写入 4 条
  关系；精确目标门禁 VERIFIED。Round 8 的 fail 观察保留原身份，绑定旧状态，不改写。
- 结论：PASS；矩阵 architecture Review 勾选。
- Coordination：Beads `ad-scan-performance-doc-arch` 转 awaiting_commit；不执行提交或推送。tasks.md 的 Review 仍未勾选，
  其 Beads 任务 `ad-scan-performance-doc-tasks` 当前为 closed，需按其自身评审另行处理，本轮不代为流转。

Task checkpoint：ad-scan-performance-doc-arch；content_state `8a80d3605f2a7ae6d38a55a2886f86ab3157ed610bcc7bf6761e69531d946c5b`（ContentState `scan-perf-arch-r9-20261027-state`）；gate VERIFIED。
提交建议：经独立授权后，提交 architecture.md 启动争用削减修订、reviews/architecture.md Round 8–9 记录与 tasks.md 中 architecture 行的矩阵变更；tasks.md 正文修订待其自身评审 PASS 后同批或单独提交；不混入工作区中的 Swift／`cmd/agentdeck-perf` 探测代码改动。
推送建议：提交后，经提交范围、贡献归属、trailer、SSH 签名核验和独立授权，推送 origin/feature/scan-performance；远端当前状态未核验。

## Round 10 — 2026-10-27

## 📋 无门槛打开路径与优化归因评审

📊 总体评分：8/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

无高严重度发现。

### 🟡 建议改进 — 推荐

- **SP-ARCH-R10-F1（中，OPEN，必须关闭后才可 PASS）** — `architecture.md:97–103`：新文字以重复打开中位 351 ms 直接断言“每次打开仍在重建内容”。端到端耗时不能区分 SwiftUI 重建、重新布局、提交/刷新等待或后台争用，且代码已经保留同一 hosting controller，只有高度变化才替换 root。
  - 行为风险：把未证明的根因当成已有事实，可能让第 5 步围绕不存在的每次销毁/重建路径实施；第 1 步尚待分段定位与此前的肯定归因互相冲突。
  - 证据：当前 `MenuBarItemController.swift:308–319` 的 `configurePopoverGeometry` 仅在 `height != preparedHeight` 时写 `rootView`，369–373 行关闭只执行 `performClose`；S0 提交 `7e7e7e3` 同文件 211–223、257–259 行同样如此。源码不能排除框架内部布局工作，但也不能用总体中位数证明每次重建。
  - 💡 有界修复：将其限定为待验证假设，明确用分段与视图/控制器身份观察区分重复构建和布局/提交成本，再依候选规则决定是否实施；保留“不重建身份”的行为约束，无需为修正文案运行新的性能批次。

### 🟢 优点

暖启动继续受不可见、不夺焦点、不启动 helper 的正确性约束；未变化扫描仅复用已有派生缓存，仍每次 Build，未回归 SP-ARCH-R8-F1 的跨时刻 payload 复用问题。合法缓存失效继续重算。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话；Method：独立增量文档评审，CodeGraph 定位当前 native 打开路径并以 Git 对照 S0；无委派。
- Scope：本轮打开路径、停止探测、调度/短事务去阈值及测量接口变更；未变化的 payload 修复契约沿用 Round 9，SP-ARCH-R8-F1 保持关闭。统计组问题归 performance-evaluation Round 17。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；blob `b1c93a9e1c22bb31973ba776915773dae60915dd`；digest `1b1d1a1bcc64a7a659188d93933bad17aa5c652824512079dcaae64e390f6c6e`，配方 SHA256(`head=<HEAD>;document=<blob>`)。
- Evidence：`python3 scripts/check-docs.py` exit 0；当前源码与 `git show 7e7e7e3:apps/macos/AgentDeckApp/MenuBarItemController.swift` 对照。未执行性能批次，不声称证明真正时延根因或暖启动产品收益。
- CEv1：WorkUnit `scan-performance:architecture.md`；目标 `scan-perf-docbatch-20261027-arch-state`。最终门禁见本轮单一完成门禁字段。
- 结论：FAIL，Review 未勾选；工作区 `agent-deck.scan-performance` / `feature/scan-performance`。无产品修改或 Git 交付。

下一步指令：修复：scan-performance / reviews/architecture.md / SP-ARCH-R10-F1

## Repair candidate — 2026-10-27（Round 10 后）

Repairer：Claude Code 主会话；范围为 SP-ARCH-R10-F1。Round 10 的结论与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS。未委派。

- SP-ARCH-R10-F1 -> repaired in candidate：
  - 删除“重复点击中位说明每次打开仍在重建内容”的断言，改为写明重复点击成本来源尚未分段。
    现有代码已保留同一 hosting controller，仅在高度变化时替换 root，关闭只 `performClose`，S0 相同（只读核对
    `MenuBarItemController.configurePopoverGeometry` 与关闭路径）。端到端中位不能区分视图重算、重新布局、提交/刷新
    等待或后台争用。
  - 第 5 步中，“保持 hosting 层级与控件身份”保留为行为约束。“再次打开重新计算 presentation 或重建视图树”改为待验证
    假设：先用重复点击分段与 Perf 构建的视图/控制器身份观察（`body` 求值次数、hosting controller 与 root 身份）区分
    重复构建与布局/提交成本，确认存在重复构建后才实施，并按候选采用规则衡量；否则按实测主导段处理。
- 同步：tasks.md 第 3 项范围中的对应表述改为同一口径。
- 不运行新的性能批次。

修复目标：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；architecture blob `31e7ae55d347b4870e09180ef846a6b85fd23414`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / architecture.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 11 — 2026-10-27

## 📋 重复打开成本归因复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

SP-ARCH-R10-F1 closed：97–105 行区分端到端耗时与未定位的成本，第 5 步用分段、body 求值及 hosting/root 身份观察验证假设，确认重复构建后才实施；tasks.md:172–174 同步此边界。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话，非修复作者；Method：独立定向文档复评，复用 Round 10 的当前/S0 源码观察，无委派。
- 处置：SP-ARCH-R10-F1 关闭；SP-ARCH-R8-F1 的派生缓存/Build 约束未变，既有关闭处置保留；无新发现。修复不再把耗时当作框架内部重建的证明。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；blob `31e7ae55d347b4870e09180ef846a6b85fd23414`；SHA256(`head=<HEAD>;document=<blob>`) = `4ed215db32354821bc88b7cb2a36d9fa166120176c0bfe1bb3de381037d81a1b`。
- Scope：归因修复及任务联动；不宣称真正根因、暖启动收益或 native 正确性已经实测。性能评估与任务分解的开放发现由各自记录承接。
- CEv1：WorkUnit `scan-performance:architecture.md`；target `scan-perf-docrecheck-20261027-arch-state`；精确 gate VERIFIED，2/2 required criteria 通过，missing/invalidated/unresolved 为空。`python3 scripts/check-docs.py` 与 `git diff --check` exit 0；新证据为该前缀的 `arch-review`、`arch-integrity`。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`；correlation `QtgKmOTPe89-k69i`。整批未完成，不触发 Topic 门禁。

Task checkpoint：ad-scan-performance-doc-arch；content_state `4ed215db32354821bc88b7cb2a36d9fa166120176c0bfe1bb3de381037d81a1b`；gate VERIFIED。
提交建议：另行授权提交架构、对应评审记录及 Review 勾选，可等待整批修复完成后统一交付。
推送建议：提交范围、署名、签名及远端核验后，另行授权推送 origin/feature/scan-performance；远端未核验。


## Round 12 — 2026-10-29

## 📋 OTel 设计变更批量独立评审 — architecture.md

📊 总体评分：5/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### SP-ARCH-R12-F1 — P1：OTel 会话被自身写入误判为 JSONL 所有

位置：`architecture.md:583,602–605`。
- 行为风险：第一次 settle 后写入 OTel 贡献与 usage_sessions；下一轮因 usage_sessions 已存在该 id 而判 jsonl_owned，撤销或停止计入本来没有 rollout 的后台用量。
- 证据：583 行把已提交 usage_sessions 作为 JSONL 会话依据，602 行又要求 OTel 会话写入同一表。当前 `internal/store/migrations.go:39` 的该表只有 client、session_id、first_at、last_at，没有 JSONL 来源列。按两条合同执行“无 rollout → settle → 写入 → 再 reconciliation”即可导出矛盾，无需运行旧实现证明新设计缺陷。
- 处置：OPEN。
💡 修复：给出来源可证明的 JSONL 所有权谓词，明确排除仅由 OTel 建立的 usage_sessions，保留真正晚到 rollout 的优先权；补入连续 reconciliation/restart/rebuild 场景。

#### SP-ARCH-R12-F2 — P1：会话写入位置未连接现有会话读取面

位置：`architecture.md:602–608`。
- 行为风险：只写 usage_sessions 和 telemetry_sessions 不能让后台会话出现在现有 session list/show 或其消费者中，无法兑现“各自会话记录可追溯”；“采用分类标记前仍显示在工作视图”缺乏数据路径。
- 证据：`internal/session/session.go:1635–1665` 的 List/ShowMetadata 从 session_metadata JOIN session_sources 读取，不读 usage_sessions；`internal/store/migrations.go:39` 的 usage_sessions 也没有该处声称保持为空的路径、项目、标题字段。原有 session list/show 是 `docs/specs/cli-design.md:1832` 起的产品表面。
- 处置：OPEN。
💡 修复：明确后台会话的持久化位置、现有列表/详情的读取适配与无正文/无文件路径行为，以及晚到 JSONL 后的替换边界；与 Task 7 对齐，不把来源标记查询接口当成完整会话读取接口。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

不再假设 OTel 存在 response id；支持表、settle、晚到 rollout 撤销和受管配置边界均已明确列出。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立文档差分、状态转换推演、现有源码/SQL 与标本核对；未参与本轮设计，未委派。
- Scope：会话所有权、canonical 贡献、会话消费端及配置/联动依赖；不是未提交实现的产品评审。
- Reviewed state：HEAD `5a57b26b15142ae980f5e19976555314a8759f2a`；document blob `6d52fb7dda1dda378d7dadaf4028bbb6325fe240`；digest `3bc3dea9685d9780e92edb349c4cfe4d50d5902e4cfb3d08e5e0d26b13625743`。
- ContentState：`scan-perf-otel-docreview-20261029-arch-0SDA62-state`；WorkUnit `scan-performance:architecture.md`；dispatch `ad-scan-performance-doc-arch`。
- Evidence：本轮 `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace`、`git diff --check` 均通过。结构检查不替代语义评审。CEv1 入口查询两项 required criteria 均缺本目标证据，旧观察不重标；最终结果以本轮完成门禁字段为准。
- 以上是设计合同与现有消费者的差异，不是要求未完成实现提前通过。既有 Round 11 的归因修复仍关闭；本轮出现两个新的阻断问题后停止扩大运行验证。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`。只更新评审、矩阵与协调/证据，保留现有产品改动；未提交、推送或运行真实配置变更。

CEv1 收口：标准 gate-status 查询返回 FAILED；两项 required criteria 的结构证据通过，独立评审证据为 fail，无 unresolved candidate impact。6 个状态、12 个观察及 24 条关系已读回核验；关系批次预检 24/24 ok。未查询 Topic 门禁，因为仍有未完成设计与实现。

## Repair candidate — 2026-10-29（Round 12 后）

修复方：Claude Code 主会话（`修复：scan-performance / architecture.md、ux/settings.md、tasks.md / …`）；未提交，工作区
`agent-deck.scan-performance`。只改本记录两项发现所在段落，不改动其他设计。

- SP-ARCH-R12-F1 -> repaired in candidate：“去重与源所有权”表中 `jsonl_owned` 改为引用新定义的 **JSONL 证据**：只来自
  `usage_events` 中该会话非 `otel:` 来源的事件，或最近一次有限 inventory 中同 id 的 Codex rollout 文件；明确
  `usage_sessions`、会话索引与 `telemetry_sessions` 中由 OTel 建立的行不是证据。所有权每次 reconciliation 按同一谓词
  重新求值，`otel_owned` 在连续 reconciliation、App/CLI 重启、journal 清理与全量 rebuild 后保持，直到出现 JSONL 证据；
  晚到 rollout 仍优先，撤销贡献、OTel 建立的会话首末时间与来源标记。
- SP-ARCH-R12-F2 -> repaired in candidate：新增“会话记录、读取面与来源标记”。真相源在 core（`usage_sessions` +
  `telemetry_sessions`，同事务写入/撤销）；读取面经会话索引投影：`session_sources` 写 `otel:codex/<conversation.id>`
  （优先级低于任何文件来源）、`session_metadata` 写空项目与路径、最后计量模型、首末时间，不写 `session_documents`。
  由此现有 `session list`/`ShowMetadata`/popover `loadSessions`（核对 `internal/session/session.go:1635–1665`、
  `internal/desktop/desktop.go:616–635`）可见；`session show` 用量明细来自按 client/session_id 读取的 `usage_events`
  （`internal/usage/session_usage.go:166`），不再落入 `cmd/agentdeck/main.go:673–695` 的“请运行 session scan”分支；
  文件 inventory 全集对账（`session.go:~975–995` 删除缺失来源）豁免 `otel:` 来源；会话排除约束投影；purge/rebuild 后
  从 `telemetry_sessions` 重新生成；晚到 JSONL 后删除 `otel:` 来源。来源标记查询明确只作分类接口。
- 验证：`git diff --check` 干净；锚点 `architecture.md#去重与源所有权` 未变；与 Task 7 的对齐见 `reviews/tasks.md`
  同日修复候选。设计推演按评审给出的“无 rollout → settle → 写入 → 再 reconciliation”序列复核：第二轮 reconciliation
  不再把自身写入当证据。


## Round 13 — 2026-10-29

## 📋 OTel 设计缺陷定向复评 — architecture.md

📊 总体评分：7/10

✅ 复评结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

#### SP-ARCH-R12-F2 — P1：仍未关闭；会话索引关联键被“路径类字段为空”覆盖

位置：`architecture.md:615–620`。
- 处置：STILL OPEN。修复已补上索引投影，但没有把内部来源键与可展示的文件路径区分。
- 行为风险：617 行要求 session_sources.source_path 为 `otel:codex/<id>`，618 行要求 session_metadata 的路径类字段为空。按字面写入后，现有 List/ShowMetadata/ShowWithActivity 的 JOIN 无法匹配，会话仍不可追溯；启用外键时写入本身不满足引用约束。
- 证据：`internal/store/store.go:61,65` 定义 source_path 为来源主键及 metadata 非空外键；`internal/session/session.go:1635–1665,1710–1713` 按 `s.source_path=m.source_path` 连接。本轮 Python sqlite3 内存最小对照：sources 一行 `otel:codex/background-1`，metadata 对应会话的 source_path 为空，执行现有等值连接得到 []；只把 metadata.source_path 改为同一合成键，相同查询返回 [('background-1',)]。不读写产品数据库。
💡 修复：明确两表使用同一非空合成来源键；“无文件路径”只约束用户展示与文件 I/O，不清空关联键。明确读取面如何抑制把该键当真实文件路径。Task 7 已有列表/详情读取断言，保留即可。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点与已关闭问题

SP-ARCH-R12-F1 CLOSED：586–591 行只接受非 otel: usage_events 或 rollout 证据，明确自身写入不构成 JSONL 证据，连续 reconciliation、重启、journal 清理与 rebuild 的不变量已补齐。

### 📝 总结

- Reviewer：Codex 主会话，非修复作者；Method：逐 finding 文档/源码核对、最小 SQL 对照及标本身份与截图检查，无委派。
- Scope：本文件上一轮两项 finding 及修复影响。SP-ARCH-R12-F2 的修复方向正确，但关联键合同仍矛盾，因此本文件 FAIL。没有新增独立 finding；修复范围仅限原 F2 的剩余字段/消费约定。最小反例明确后未扩大产品验证。
- Reviewed state：HEAD `5a57b26b15142ae980f5e19976555314a8759f2a`；blob `8c40117296fe7877d4080d640763f32ef3b5ee4c`；digest `4f068c3ff4df9186086aef0caed320b96ceee61a15a1d394b14fecb28e27b9ec`。
- ContentState：`scan-perf-otel-rereview-20261029-arch-lziiHa-state`；WorkUnit `scan-performance:architecture.md`；dispatch `ad-scan-performance-doc-arch`。
- Evidence：`python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace`、`git diff --check` 通过。CEv1 入口精确目标 NOT_VERIFIED，缺本轮独立复评及结构证据；旧失败观察保持原身份，最终门禁见本轮单一字段。
- 依赖影响：requirements/performance-evaluation/ux-cli 文档 blob 分别仍为 812d3f4c206101a62812b232e5c34e262b9287de、913a03fa724daca96e2e628ef6b4f9e35269fc08、38374626fbdf82f7c0e5f61de34f6cba2f6e2563；保留其已有语义结论，不宣称变动后的共享 manifest 已被旧门禁认证。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`；correlation `lziiHa3A3s6UgvCO`。生产代码、测试和配置只读；未提交或推送。

CEv1 收口：标准 gate-status 返回 FAILED；本轮 3 个状态、6 个观察与 12 条关系均已精确读回，关系预检 12/12 ok。结构准则 pass，语义准则 fail；无 unresolved candidate impact。旧失败观察保留，不重标。

## Repair candidate — 2026-10-29（Round 13 后）

修复方：Claude Code 主会话（`修复：scan-performance / architecture.md / SP-ARCH-R12-F2`）；未提交，工作区
`agent-deck.scan-performance`。只改“会话记录、读取面与来源标记”中读取面一条，不改其他设计。

- SP-ARCH-R12-F2 -> repaired in candidate：`session_sources.source_path` 与 `session_metadata.source_path` 明确为
  **同一个非空合成来源键** `otel:codex/<conversation.id>`，满足 `internal/store/store.go:61,65` 的主键与非空外键及
  `internal/session/session.go:1635–1665,1710–1713` 的等值连接；`session_sources` 的 identity 同为该键、size/mtime/cursor
  为 0、`priority=-1`（低于 `internal/ingest/ingest.go:161–163` 的文件来源 0/1）；与现有非文件合成来源
  `agentdeck://replace-documents`（`session.go:29,1608`）同类。删除“路径类字段为空”的写法，改为：“没有文件路径”只约束
  展示与文件 I/O——`ShowWithActivity` 的 `activity.ReadDetails`（`session.go:1731`）、会话查看器的 `os.Stat` 与
  `activity.ReadDetailsPage`（`cmd/agentdeck/session_viewer_data.go:30,39`）识别 `otel:` 前缀并返回“无活动明细”，不报来源
  缺失；文本输出省略该来源的路径；JSON `source_path` 保留键原值；按 `path` 的排除（`session.go:1794`）按该键字面匹配。
- 验证：内存 SQLite 使用 `store.go` 的真实建表语句与现有 `List` 查询（不读写产品数据库）：只有合成来源时返回
  `('codex','background-1','','otel:codex/background-1','gpt-5.6-terra')`；再加入同 id 的 priority 1 文件来源后只返回文件来源；
  metadata `source_path` 为空时外键拒绝写入（复现评审的最小反例）。`git diff --check` 干净。Task 7 现有列表/详情读取断言
  覆盖此合同，未改 tasks.md。


## Round 14 — 2026-10-29

## 📋 合成会话来源键与读取契约复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点与已关闭问题

SP-ARCH-R12-F2 CLOSED：architecture.md:613–630 明确 session_sources.source_path、identity 与 session_metadata.source_path 使用同一个非空合成键 `otel:codex/<conversation.id>`；priority=-1 低于文件来源 0/1。“无文件路径”仅约束展示与文件 I/O：ShowWithActivity、viewer os.Stat/ReadDetailsPage 识别 otel: 并返回无明细，文本省略路径，JSON 保留来源键，path 排除按键匹配。已消除 Round 13 的空键/等值连接矛盾。

SP-ARCH-R12-F1 保持 CLOSED：JSONL 证据仍仅由非 otel: usage_events 或 rollout 提供，自身 usage_sessions/索引写入不构成证据。无新发现或回归。

### 📝 总结

- Reviewer：Codex 主会话，非修复作者；Method：定向文档与现有 DDL/SQL/消费者核对、隔离内存 SQLite 对照；无委派。
- Scope：SP-ARCH-R12-F2 的来源键与展示/文件 I/O 边界；不评审或修复未提交产品实现。
- Reviewed state：HEAD `bd72d138fe280ec84ecb822176bd27de8e635cf4`；blob `7744862383b013a4b446f4727083d933d69c4e6c`；digest `e922edc2b72ccf18c0cd32e99c6c160e94857ba0ab3a0920298ab5ad313ff4d6`；ContentState `scan-perf-arch-r14-20261029-tVcY3H-state`。
- Evidence：使用 internal/store/store.go 中 session_sources/session_metadata 的真实 CREATE TABLE 语句，启用 foreign_keys，提取 internal/session/session.go 的原 List SQL。只插入合成来源时返回该 Codex 会话；再插入同 id、priority=1 的文件来源时只返回文件来源；metadata 空来源键被外键拒绝。整个实验在 :memory:，不读取产品数据库。核对 internal/ingest/ingest.go:161–163 的 0/1、session.go:29 的既有合成来源，以及 session_viewer_data.go:30,39 的文件访问点。前轮已核验的 List/ShowMetadata/ShowWithActivity 连接事实继续有效。
- 验证边界：设计声明的是待实施分支，未把现有产品已支持 otel: 当成事实。Task 7 现有列表/详情、晚到 JSONL 与 rebuild 断言仍承接实现验收。
- 本轮恢复 tasks.md Documents 中 architecture.md 的 Review 勾选；这是状态投影，不改变任务正文或重开 tasks.md 语义评审。待提交的 tasks.md 仍需在架构章节交付后核验提交树依赖，旧目标证据不能直接重标其新矩阵 blob。
- CEv1 入口精确目标 NOT_VERIFIED，缺本轮独立复评和结构证据；保留 Round 12/13 失败观察与历史。最终门禁见本轮单一字段。
- 工作区 agent-deck.scan-performance / feature/scan-performance；correlation tVcY3HTfsxjZ5nZ7。未提交、推送或修改生产代码/测试/配置；Topic 与实现任务尚未完成。

Task checkpoint：ad-scan-performance-doc-arch；content_state `scan-perf-arch-r14-20261029-tVcY3H-state`；gate VERIFIED。
提交建议：门禁已 VERIFIED；独立提交 architecture.md、本记录与仅属于本任务的 Review 格状态。不得合并其他文档任务或产品实现。
推送建议：门禁已 VERIFIED；核验单任务提交范围、正文/trailers、SSH 签名及远端，再另行授权推送 origin/feature/scan-performance；远端未核验。

收口证据：上述四项 L0 命令通过；1 个 ContentState、2 个 evidence 节点和 4 条关系精确读回，关系预检 4/4 ok；标准 gate-status 返回 VERIFIED，两个 required criteria 均 pass，missing/unresolved 为空。原失败记录未重标。

## sql.DB 实现修订交接 — 2026-10-31

用户要求修复反复建立连接池导致的读取超时，并将 sql.DB 改动单独提交。
`architecture.md` 的“恢复与 MainActor”同步进程只读池的身份绑定、配置隔离、有限驻留、
预热、lease/退出释放及 writer checkpoint；200 ms identity 期限与每次读取当前
schema/epoch/revision 的约束保留。根因、修复和直接验证统一记录于
[serving-open-path 的 SQL 修复交接](serving-open-path.md#sqldb-连接池修复交接--2026-10-31)。
此修订待新候选评审；上述历史 PASS 和精确证据仅对应原文档身份。
