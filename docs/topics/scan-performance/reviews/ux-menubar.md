---
status: active
topic: scan-performance
subject: ux/menubar.md
---

## Round 1 — 2026-10-09

## 📋 菜单栏最终设计标本评审

📊 总体评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

**M1-F1 · P2 · OPEN — 首批 usage ready、其他字段或窗口 unavailable 的核心状态没有可评审标本。**

- 位置：`ux/menubar.md:33–37,93–104`。
- 行为风险：实现者只能从“全部有数字的 partial”推导混合 availability，无法验收“今天先可读但项目/会话/7d 未准备”，容易把未知计数呈现为普通可信数字。
- 证据：该文声称复用 first→partial→memory 标本。`partial-en-light-420.png` 显示 4 sessions / 3 projects；`prototype/src/Popover.jsx:1528–1535,1722–1733,1738–1749,1797–1802` 只对整体不可用/schema/midnight 隐藏数据，serving partial 没有逐域/逐窗口 availability 输入；切换 7d/30d 仍使用完整合成 scope。`ServingSnapshot.jsx:31–42` 只有全局 hasSnapshot 和 phase。不是要求原型真的调度，而是要求它能呈现设计承诺的状态。
- 💡 修复：在同一共享原型增加明确的混合 availability fixture，包含 today usage、项目/会话未知、其他窗口 unavailable/partial，以及该状态原子发布后的切换。补双语/窄宽标本与交互证据，绑定新 manifest。

### 🟢 优点

旧值年龄、跨天不可复用昨日 today、失败保留数据、默认额度独立和焦点保持要求明确；已有 first/midnight/disk 标本体现这些区别。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；actor `opencode`。
- Method: 独立主会话 final-surface 审查；直接查看存档 PNG、manifest 与候选源码，复用哈希匹配的作者交互证据；未新增浏览器或 native 实测。
- Scope: 全 menubar UX，依赖 requirements、architecture coverage、performance 边界与 tasks；本结论不批准其失败的架构依赖。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `d46fa8e8f67b1a3925072bdaf9c04b84bb95d728`；manifest SHA256 `739b67d01e67f7a5427946e41a80c2302ab64f9d8149901c9cef6ff109a428fe`。
- ContentState: `scan-performance:state:bf7657f6968d40b150bf079d2c4fba425a186196a2941d1e4a05b10ed890c399`；SHA256(`head=<HEAD>;document=<blob>;prototype=<manifest_sha256>`)。
- Evidence: 43 个 manifest 项哈希全匹配；文档集与 whitespace 检查通过；图像与源码支持上述缺失状态。历史 64 状态检查不能覆盖不存在的混合状态。
- Completion gate: FAILED
- Gate evidence: Neo4j MCP 标准 gate-status 查询绑定上述 ContentState；document-integrity 通过，surface-contract-and-specimens 被本轮适用 fail evidence 否定；无 invalidated 或 unresolved evidence。
- 下一步指令：修复：scan-performance / reviews/ux-menubar.md / M1-F1

## Repair candidate — 2026-10-09

Repairer: Codex；范围仅为上述Round 1的授权问题。原Round 1的分数、Verdict、ContentState与FAILED门保留。

- M1-F1：mixed synthetic fixture、today用量/未知session-project、7d unavailable/30d partial、原子发布保留选择；四语言主题组合/两宽度标本。 **REPAIRED in candidate，待独立复评确认**。

本候选不创建Review Round 2，不自行声明Review PASS。文档/标本的当前内容身份、
验证结果与修复目标证据状态在本节完成验证后绑定；旧观察不重新标成新状态。

修复目标绑定：HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；document blob `704518a57c97606ea76d156df6c09445a98c506f`；
repair_target_state: `scan-performance:state:538ae79ed36140a3b6f63af49ed765f8bdd5df795b2b1e2294e7ef5005468e37`。
标本manifest SHA256 `2ab9be34d6e69586182d905a8aba17a88533d69a635d6895aede220acc987ea7`，59项均按当前源码/合成截图绑定。
验证：topic文档集、local links、review-record parser、whitespace、diff检查通过；
prototype build通过；120项mixed/telemetry/CLI合成交互通过，17截图在900×1500视口生成。
真实规模性能harness/摘要未改，未重扫私人数据、未测browser/native性能；产品Go/Swift未改。
证据同步：15个精确目标/证据节点读回全匹配，20关系预检ok并写入；标准gate-status查询
在上述新目标返回 `NOT_VERIFIED`，仅缺独立复评准则 `scan-performance:ux/menubar.md:surface-contract-and-specimens`。
document-integrity有当前pass；没有invalidated/unresolved impact。独立设计准则记not_verified，
作者不把修复当Review PASS，旧Round1 fail观察/关系未覆盖或重标。
依赖影响：requirements/performance-evaluation的blob保持原PASS身份，性能harness与三份数据摘要未改；
本次仅修复下游契约和合成UX，没有改需求目标或测量归因，复用其原结论。

## Round 2 — 2026-10-09

## 📋 菜单栏混合可用性复评

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

M1-F1 CLOSED：`ux/menubar.md:33–38,108–119` 与 shared mixed fixture 一致。today 用量可读但 sessions/projects 为 —，7d unavailable，30d partial；sessions/额度域有独立 unavailable。发布后恢复完整数据并保留 client/period/panel。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`；Method: 独立复核修复文档、CodeGraph 定位的候选源码，以及 agent-browser 实际合成页面交互。未修改标本。
- Scope: M1-F1 全部闭环点及与 architecture coverage 的依赖；无新增或回归 finding。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `704518a57c97606ea76d156df6c09445a98c506f`；manifest SHA256 `2ab9be34d6e69586182d905a8aba17a88533d69a635d6895aede220acc987ea7`。
- ContentState: `scan-performance:state:538ae79ed36140a3b6f63af49ed765f8bdd5df795b2b1e2294e7ef5005468e37`；沿用 HEAD/blob/manifest recipe。
- Evidence: manifest 59 项全匹配；作者 120 项绑定检查作为补充复用。独立 browser 观察 today 为 75 events / — sessions / — projects，7d 为 —/unavailable；Codex/30D/Sessions 在 publication 1 时不可用，点击 Publish 后 publication 2、同样选择保留、55 sessions/5 projects 出现。未重跑整个作者矩阵或性能实验。
- Browser limit: 默认测试视口下舞台控件遮挡一次客户端点击；改用原标本规定的 900×1500 后继续同一用例成功。结论仅覆盖共享合成标本，不认证 native 布局/首帧。
- Completion gate: VERIFIED
- Gate evidence: 标准 Neo4j MCP gate-status 对上述目标返回 VERIFIED；两项 required criteria 通过，missing/invalidated/unresolved 均为空。

Task checkpoint：ad-scan-performance-doc-ux-menubar；content_state `538ae79ed36140a3b6f63af49ed765f8bdd5df795b2b1e2294e7ef5005468e37`；gate VERIFIED。

提交建议：将 UX、mixed 原型/标本及复评记录纳入完整设计批次的授权提交。

推送建议：feature/scan-performance；须核验远端及分别获得提交/推送授权。
