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


## Round 3 — 2026-10-29

## 📋 OTel 设计变更批量独立评审 — ux/cli.md

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

显式写配置参数、冲突退出 1、恢复不完整仍关闭 receiver 且退出 0、text/JSON 通道与 warning 字段彼此一致。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立文档差分、状态转换推演、现有源码/SQL 与标本核对；未参与本轮设计，未委派。
- Scope：新增 telemetry status/enable/disable 契约与 engineSamples、渲染截图；既有 scan/watch/envelope 兼容约束。
- Reviewed state：HEAD `5a57b26b15142ae980f5e19976555314a8759f2a`；document blob `38374626fbdf82f7c0e5f61de34f6cba2f6e2563`；digest `56f8814188682978e4b28d2a8c5e283ba78cf010be9cc6adf56ce95a6d4d79b1`。标本 manifest SHA256 `edc7c2012ae85c5f5679bfa978b8ae68738d0c2023e1ea53ed06f73bfd568013`。
- ContentState：`scan-perf-otel-docreview-20261029-ux-cli-0SDA62-state`；WorkUnit `scan-performance:ux/cli.md`；dispatch `ad-scan-performance-doc-ux-cli`。
- Evidence：本轮 `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace`、`git diff --check` 均通过。结构检查不替代语义评审。CEv1 入口查询两项 required criteria 均缺本目标证据，旧观察不重标；最终结果以本轮完成门禁字段为准。
- 直接核对 ServingSnapshot.jsx:102–145 的六个新增场景、commandId、warning、stdout/stderr 和退出码；目视核对写入冲突、恢复不完整截图。manifest 70/70 匹配，记录中的合成检查绑定当前原型源码；未执行真实 CLI 写配置。C1-F1/C1-F2 的原关闭处置保留；未变 scan/watch/公共 envelope 约束沿用。Settings 的禁用开关缺口不适用于 CLI 的显式 disable；架构写入/所有权算法的阻断问题由架构记录承接，CLI PASS 不批准其实现。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`。只更新评审、矩阵与协调/证据，保留现有产品改动；未提交、推送或运行真实配置变更。

Task checkpoint：ad-scan-performance-doc-ux-cli；content_state `scan-perf-otel-docreview-20261029-ux-cli-0SDA62-state`；gate VERIFIED。
提交建议：门禁已 VERIFIED；在独立授权下提交 ux/cli.md、本轮评审记录及对应矩阵状态；可与修复完成后的设计批次统一交付。
推送建议：门禁已 VERIFIED；核验提交范围、署名/SSH 签名与远端，再另行授权推送 origin/feature/scan-performance；当前远端未核验。

CEv1 收口：标准 gate-status 查询返回 VERIFIED；两项 required criteria 的结构证据通过，独立评审证据为 pass，无 unresolved candidate impact。6 个状态、12 个观察及 24 条关系已读回核验；关系批次预检 24/24 ok。未查询 Topic 门禁，因为仍有未完成设计与实现。


## Delivery impact assessment — 2026-10-29

范围：CLI 文档任务的独立授权提交，不产生新的语义评审轮。CLI 文档 blob 仍为 `38374626fbdf82f7c0e5f61de34f6cba2f6e2563`；`prototype/src/ServingSnapshot.jsx` 仍为 `ca45f6212bbf82d4b1b6f63f06ab879aa240db9e`。共享标本 manifest 从 Round 3 的身份变为 `6e9a9af054713b277976722035425a491deaf58c7b9e1ac6733640b11e3c716c`，变化由 Settings 修复、新增 Settings 截图及同一合成检查脚本补充引起。CLI 六场景的字段、通道与退出码未改变；当前 114 项合成检查仍含 40/80 列 CLI 断言，72 项 manifest 匹配，重新生成的冲突截图已目视核对，仍为 stdout 空、stderr error envelope、exit 1。故保持 Round 3 CLI 语义结论，显式绑定交付目标并关联原证据，不将旧 manifest 证据直接重标。架构 F2 仍未解决，不提交架构正文或其失败评审记录，不宣称整批设计可进入实现。

交付拆分更正：按用户要求一任务一提交。Settings 任务先提交其源码、截图、Settings 检查条目与 Settings-only harness；CLI 任务随后独立提交 CLI 文档/评审、EngineCli 场景、三张 CLI 截图，以及共享清单/检查结果/harness 的 CLI 增量。CLI 条目按同一内容保留，未改变字段、通道、退出码或断言。上述完整共享 manifest 为评审环境身份，不表示把其他任务纳入本提交。
