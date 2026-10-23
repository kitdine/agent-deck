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
