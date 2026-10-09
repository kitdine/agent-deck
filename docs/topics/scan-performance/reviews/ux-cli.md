---
status: active
topic: scan-performance
subject: ux/cli.md
---

## Round 1 — 2026-10-09

## 📋 CLI 契约与逐字符标本评审

📊 总体评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

**C1-F1 · P1 · OPEN — 将 stored-only 来源不完整提示误写成已有行为。**

- 位置：`ux/cli.md:16–25`。
- 行为风险：首次后台补历史时，`usage summary --no-scan` / `stats --no-scan` 不会因扫描被跳过而自动声明历史覆盖不完整；调用方可能把少量已提交结果当成完整范围。
- 证据：`cmd/agentdeck/main.go:3853–3870,3889–3916` 使用本次 scanErr 计算 partial；no-scan 分支无 scanErr。`docs/specs/cli-design.md:1251–1258` 保证 stored-only 读取，但没有所声称的来源覆盖传播。与 `reviews/architecture.md` A1-F2 是同一跨层缺口，分别需关闭契约与表面承诺。
- 💡 修复：写出拟新增的覆盖读取行为，而非“沿用”；定义 no-scan text/JSON warning、partial、退出码及完整性恢复条件，增加首批导入中 CLI 标本，接到架构持久 coverage 契约。

### 🟡 改进问题 — 必须闭环

**C1-F2 · P2 · OPEN — 声称沿用的 JSON envelope 与逐字符标本不一致。**

- 位置：`ux/cli.md:76–80,130–142`；`prototype/src/ServingSnapshot.jsx:75–109`。
- 行为风险：照标本实现会形成与现有自动化契约不兼容的 command ID 和顶层字段；错误 JSON 的输出通道也不明确。
- 证据：现有 `docs/specs/cli-design.md:2275–2285` 包含 schema_version、generated_at、warnings 和点分 command；`main.go:394–396` 将 JSON error 输出至 stderr。标本用 `engine status`/`telemetry status` 等空格 command，且生成的成功/错误 envelope 缺少公共字段，独立 JSON 区未标出 stderr。图像 `cli-mismatch-en-dark.png` 可直接看到该差异。
- 💡 修复：使 proposed 命令的完整 text/JSON 样本遵循公共 envelope、command 命名和 stdout/stderr 契约；若仅为摘录需明确标注，不能再作为完整逐字符标本使用。

### 🟢 优点

有限请求与 App 生命周期分开，Ctrl-C detach 和旧 v1 保持要求清楚；条件性的 engine 命令与当前命令有明确区分。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；actor `opencode`。
- Method: 独立主会话文档/代码/图像对照；未启动真实 receiver 或修改用户配置。
- Scope: 完整 CLI 候选、requirements、architecture、既有 cli-design/manual 及共享 CLI 标本。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `df8ca55432cca52a546a4d45e40e9dd329a5f211`；manifest SHA256 `739b67d01e67f7a5427946e41a80c2302ab64f9d8149901c9cef6ff109a428fe`。
- ContentState: `scan-performance:state:4f2e2312a5f4293940af3344663f70609d275aec75fae721cb623f829d6a7970`；SHA256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 文档集/whitespace 检查通过，43 项 manifest 全匹配；静态分支与完整输出契约提供直接反例。未把历史浏览器列宽通过当成 CLI runtime 兼容证明。
- Completion gate: FAILED
- Gate evidence: Neo4j MCP 标准 gate-status 查询绑定上述 ContentState；document-integrity 通过，command-contract-and-specimens 被本轮适用 fail evidence 否定；无 invalidated 或 unresolved evidence。
- 下一步指令：修复：scan-performance / reviews/ux-cli.md / C1-F1、C1-F2

## Repair candidate — 2026-10-09

Repairer: Codex；范围仅为上述Round 1的授权问题。原Round 1的分数、Verdict、ContentState与FAILED门保留。

- C1-F1：明确是新增持久coverage读取；daily --no-scan text/JSON warning、partial与exit0标本，完整覆盖后清除。 **REPAIRED in candidate，待独立复评确认**。
- C1-F2：完整schema_version/点分command/generated_at/data/warnings/partial；成功stdout、错误stderr/data:null；40/80列九场景。 **REPAIRED in candidate，待独立复评确认**。

本候选不创建Review Round 2，不自行声明Review PASS。文档/标本的当前内容身份、
验证结果与修复目标证据状态在本节完成验证后绑定；旧观察不重新标成新状态。

修复目标绑定：HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；document blob `24250e33e864046239473aab1c8663231f71986e`；
repair_target_state: `scan-performance:state:752d9eb4874fa2408c70418ecc77e27d0ef15b8c00c9b6b1146643e2659ee0aa`。
标本manifest SHA256 `2ab9be34d6e69586182d905a8aba17a88533d69a635d6895aede220acc987ea7`，59项均按当前源码/合成截图绑定。
验证：topic文档集、local links、review-record parser、whitespace、diff检查通过；
prototype build通过；120项mixed/telemetry/CLI合成交互通过，17截图在900×1500视口生成。
真实规模性能harness/摘要未改，未重扫私人数据、未测browser/native性能；产品Go/Swift未改。
证据同步：15个精确目标/证据节点读回全匹配，20关系预检ok并写入；标准gate-status查询
在上述新目标返回 `NOT_VERIFIED`，仅缺独立复评准则 `scan-performance:ux/cli.md:command-contract-and-specimens`。
document-integrity有当前pass；没有invalidated/unresolved impact。独立设计准则记not_verified，
作者不把修复当Review PASS，旧Round1 fail观察/关系未覆盖或重标。
依赖影响：requirements/performance-evaluation的blob保持原PASS身份，性能harness与三份数据摘要未改；
本次仅修复下游契约和合成UX，没有改需求目标或测量归因，复用其原结论。

## Round 2 — 2026-10-09

## 📋 CLI 覆盖提示与完整 envelope 复评

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

- C1-F1 CLOSED：`ux/cli.md:16–19,89–106` 明确是新增持久 coverage 读取，不再声称当前 no-scan 自动拥有该行为；范围、warning/partial、exit0、清除条件及读取失败定义齐全，与 architecture 和 Task 2 对应。
- C1-F2 CLOSED：`ux/cli.md:78–87`、`ServingSnapshot.jsx:111–130` 提供 schema_version、点分 command、UTC generated_at、data、warnings、partial；成功 JSON 在 stdout、错误 JSON 在 stderr，data:null。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`；Method: 独立修复复核与 browser 逐字符输出抽查；复用原 HEAD 上已核验的 CLI 公共契约和修复作者 40/80 列矩阵。
- Scope: 两项 CLI finding、架构来源覆盖、Settings unknown 同步；无新增或回归 finding。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `24250e33e864046239473aab1c8663231f71986e`；manifest `2ab9be34d6e69586182d905a8aba17a88533d69a635d6895aede220acc987ea7`。
- ContentState: `scan-performance:state:752d9eb4874fa2408c70418ecc77e27d0ef15b8c00c9b6b1146643e2659ee0aa`。
- Evidence: agent-browser 在 cols=40 核对 stored_partial 为 usage.summary、source_coverage_partial、partial:true、exit0，stderr 无 JSON；mismatch 为 stdout 空、stderr 完整 error envelope、data:null、exit1；telemetry_unknown 保留 producer_configured:unknown、last_received_at:null。59 项 manifest 全匹配；入口 document-integrity 证据有效。
- Completion gate: VERIFIED
- Gate evidence: 标准 Neo4j MCP gate-status 对上述目标返回 VERIFIED；两项 required criteria 通过，missing/invalidated/unresolved 均为空。
- Limits: 合成标本不执行真实 CLI/IPC/OTel；运行时契约验证属于后续实现，不把输出样本当已实现功能。

Task checkpoint：ad-scan-performance-doc-ux-cli；content_state `752d9eb4874fa2408c70418ecc77e27d0ef15b8c00c9b6b1146643e2659ee0aa`；gate VERIFIED。

提交建议：将 CLI UX、完整输出标本与复评记录纳入完整设计批次的授权提交。

推送建议：feature/scan-performance；须核验远端及分别获得提交/推送授权。
