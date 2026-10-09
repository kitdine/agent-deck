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
