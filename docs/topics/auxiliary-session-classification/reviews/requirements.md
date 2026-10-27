---
status: active
topic: auxiliary-session-classification
subject: requirements.md
---

# requirements.md 评审

## 2026-10-27 证据范围与协调勘误

用户授权修正 Beads 状态后，确认先前将整个文档集缺稿归入本前置文档
document-integrity 的阻塞理由过宽。依据 docs/documentation-workflow.md 的文档集规则，
设计期间允许声明未编写文档；完整集合须在 tasks.md 文档集评审前补齐。
本勘误不改变语义 PASS、不撤销原发现、不豁免文档集/CI 检查，也不新建语义评审轮次。
下面各轮原 BLOCKED 及提交建议作为当时记录保留；当前协调以本勘误的交付对象及 CEv1 新观察为准。

- 交付对象：`1d6ab3a12c06ace582d6ee21f4109bf82988561c`；正文 blob `3c14de201f3b0f87b16bdee4d4abde424b6b7136`，与已评审正文一致。
- 新 ContentState：`auxiliary-session-classification:delivered-requirements.md-aaa32d98bdccd582`；指纹 `aaa32d98bdccd5821adc67643afcfc889605866063cf8560bb41442eff0c92d0`。
- 交付目标查询结果：VERIFIED（2026-10-27）；missing criteria 与 unresolved impacts 均为空。
  此结果仅绑定上述交付对象的需求文档，不宣称当前全主题或最终 UX 已完成。
- 复用来源：`auxiliary-session-classification:req-r2-376128eaa35844f7:independent-review`。新观察绑定交付对象并通过 rolls_up 显式引用；
  提交只固化已评审内容与记录，不引入新的正文决策。标本按各自交付 commit 核验，未用当前其他表面改动冒充旧 manifest。
- 本文检查范围：格式、真实本地链接、评审记录、空白和已绑定标本完整性；不包含尚未设计的另一文档。
  完整文档集缺稿仍由 tasks.md 承接。检查结果及门禁以新目标 CEv1 记录为准。
- 协调边界：完成该需求文档的交付协调；不关闭整个主题。

历史段落中的“尚未提交/不得提交”和旧 BLOCKED 不再用作当前待评审队列依据；
所有已有历史事实和旧目标证据保持不变。

## Round 1 — 2026-10-26

## 📋 需求边界评审

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 — 必须修复

**ASC-REQ-R1-F1 — P2 — JSON 显式辅助范围的过滤契约矛盾（OPEN）**

位置：[requirements.md:105](../requirements.md#用户控制与表面)，关联第 42、80–84、112 行。

- 行为风险：自动化调用显式选择 auxiliary 时，消费者不能确定返回辅助集合还是完整集合；实现者必须自行选择一个相反的契约，破坏三范围一致性。
- 证据：目标要求工作、辅助、全部三个范围，第 82 行将辅助定义为 auxiliary，第 84 行要求范围过滤先于分页；但第 105 行规定 JSON “显式工作范围才过滤”。具体判别场景：同一窗口含一条 work 和一条 auxiliary，JSON 显式选择辅助范围；前述集合定义要求只返回 auxiliary，后述限制却不允许该选择触发过滤。现有 cli-design.md 的 Session Search 只保证未分页 JSON 的完整集合兼容，并未要求新增显式辅助选择仍返回全部。
- 💡 修复建议：明确未指定范围时 JSON 保持原有集合与分页行为；显式 work 和 auxiliary 分别选择对应集合，显式 all 保持全部。统一第 105、112 行及 AC5/AC6 对该场景的表述，不改变文本默认或现有兼容边界。

### 🟡 建议改进 — 推荐

无。未把尚未编写的下游设计文档列为本需求正文的待修复 finding。

### 🟢 优点

- 明确维护用途与任务子代理是不同维度，unknown 保持可见，手工覆盖保留自动来源。
- 区分会话分区、项目集合并集及非可加统计，保留总消耗和 canonical 身份。
- 明确旧 JSON/wire、App Group 隐私、恢复、规则升级和扫描性能集成边界；至少一种实际维护来源的可识别性仍是架构必须兑现的条件。

### 📝 总结

- Reviewer: Codex；独立评审会话，未参与目标需求的设计写入。
- Method: 单评审者只读源代码/契约核对与反例推演；未委派。CodeGraph 定位 Metadata、List、sessionsSnapshot；核对 AppGroupSessionsSnapshotV1 和当前 CLI 契约。未读取私人日志或生产数据库，未运行产品测试。
- Scope: requirements.md；tasks.md 仅作文档集合和状态上下文，不评审实现分解。
- Reviewed state: HEAD `b450704bc43807487a51a777a90dc4f8793d30dc`；requirements blob `13a918736b86edd324b463a427a9073bb1c371c0`。
- Fingerprint: `206f2f4d6f6d68c02e86dc0e63c8ec99bebe4774d66db392ac836a7530073d57`，配方为 SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: `session.go:50–59,1494–1508` 确认当前元数据无用途字段且按 client/session_id、来源优先级去重；`desktop.go:638–650` 确认 total/periods 来自传入集合；`AppGroupSnapshotStore.swift:147–158` 确认会话仅投影 available/total；`cli-design.md` Session Search、desktop wire v1 与 Widget 段确认当前兼容和隐私约束。
- Evidence: `python3 scripts/check-docs.py` 报告文档集检查失败：ux/cli.md、ux/menubar.md、ux/settings.md、ux/widget.md、architecture.md 已声明但未编写。按文档生命周期，这是下游设计未完成导致的完整性门禁缺口，不要求本轮填占位文件或缩减文档集。
- Evidence: 记录及状态写入后，`python3 scripts/check-review-records.py docs/topics/auxiliary-session-classification/reviews/requirements.md`、`make check-whitespace` 与 `git diff --check` 均成功。
- WorkUnit: `auxiliary-session-classification:requirements.md`；ContentState: `auxiliary-session-classification:req-r1-206f2f4d6f6d68c0`。
- Completion gate: FAILED
- Gate scope: independent-review 因 ASC-REQ-R1-F1 为 fail；document-integrity 因声明的下游文档未完成为 blocked。需求 FAIL 与下游文档集缺口分别保留，不宣称产品验收或交付。
- Provider recovery: 从 ai-tools 的 completion-evidence/v1 PROFILE、schema 和固定 Neo4j 模板恢复读取；使用 ce_namespace 映射，不依赖有 sample: None 缺陷的 schema helper。按项目权威登记本文件的两个必需条件，无产品验收目标变更。
- Residual uncertainty: 实际维护来源识别与原生交互仍须后续架构/UX 和实现证据；本轮不将其当作已实现事实。

修复：auxiliary-session-classification / reviews/requirements.md / ASC-REQ-R1-F1

## Repair candidate — 2026-10-26（Round 1 后）

- Repairer：Codex 原设计会话；仅处理 ASC-REQ-R1-F1，未委派、未修改产品代码。
- ASC-REQ-R1-F1 → repaired in candidate，等待独立复评确认；上方 Round 1 的 OPEN、FAIL
  和原目标 FAILED 保留为历史，不在 Repair 中自行关闭 finding 或宣布 Review PASS。
- 修复：JSON 未指定范围保留既有集合和分页行为；显式 work 选择 work + unknown、
  auxiliary 仅选择 auxiliary、all 选择全部。统一表面表、参数优先级、AC5 和 AC6，
  明确先选集合再分页及 total；文本默认工作和旧 wire 字段含义不变。
- 判别场景核对：同窗口 work、auxiliary、unknown 各一条，显式三范围分别得到 2、1、3 条；
  未指定范围 JSON 得到全部三条，已有分页参数仍适用。Round 1 的两条记录反例中，
  显式 auxiliary 只得到辅助记录，不再与“仅工作范围才过滤”的限制冲突。
  这是需求文本一致性检查，不是实现运行结果。
- Candidate identity：HEAD `b450704bc43807487a51a777a90dc4f8793d30dc`；
  requirements blob `3c14de201f3b0f87b16bdee4d4abde424b6b7136`。
- 检查：`python3 scripts/check-docs.py` 仍退出 1，仅报告已声明未编写的四份 UX 与
  architecture.md 的文档集合缺口。缺口保持开放，不在单条需求修复中填充下游文档或豁免门禁。
- CEv1：未把原目标证据迁移为新候选通过结果；新候选的独立评审和完成门禁留待复评，
  不宣称 VERIFIED。tasks.md 的 Review 仍未勾选。

## Round 2 — 2026-10-26

## 📋 需求修复复评

📊 总体评分: 9/10

✅ 结论: PASS

### 🔴 严重问题 — 必须修复

无。无仍开放、回归或新增阻塞发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**ASC-REQ-R1-F1 — CLOSED**：requirements.md 第 105、112–114 行及 AC5/AC6 现已统一。
显式 work 选择 work + unknown，auxiliary 只选择 auxiliary，all 选择全部；未指定范围的
JSON 保留既有完整集合与分页契约。选集合先于分页与 total，文本仍默认工作范围。

原判别场景中 work、auxiliary 各一条，显式辅助只返回一条；加入 unknown 后，work、
auxiliary、all 分别为 2、1、3 条，未指定范围的未分页 JSON 为 3 条。
这是对需求集合定义的独立推演，不是产品运行结果。原矛盾已消除，无须改变兼容边界。

### 📝 总结

- Reviewer: Codex，原独立评审会话；未参与修复写入。
- Method: 单评审者逐项复核修复文本与原反例，检查全文受影响的一致性；不委派、不修复、不运行产品测试。
- Scope: requirements.md 的 ASC-REQ-R1-F1 及参数、分页、AC5/AC6。分类、消耗和隐私约束未变，复用 Round 1 的当前实现/CLI 契约核对；HEAD 未变且没有产品脏文件。
- Reviewed state: HEAD `b450704bc43807487a51a777a90dc4f8793d30dc`；requirements blob `3c14de201f3b0f87b16bdee4d4abde424b6b7136`。
- Fingerprint: `376128eaa35844f758d3b2c51084977e854d6a05b4f4a494f24cfbbf11302554`；沿用 SHA-256(`head=<HEAD>;document=<blob>`)。
- Evidence: 当前全文与原轮记录逐项核对，唯一发现 ASC-REQ-R1-F1 关闭。原 blob 未保存为 Git 对象，因此未声称执行旧 blob 对新 blob 的 Git diff；旧轮原文与本会话读取结果保留反例依据。
- Evidence: 本轮记录及矩阵同步后，`python3 scripts/check-docs.py` 退出 1，仅报告上述五份未编写文档；评审记录解析、`make check-whitespace` 和 `git diff --check` 均退出 0。
- WorkUnit: `auxiliary-session-classification:requirements.md`；ContentState: `auxiliary-session-classification:req-r2-376128eaa35844f7`。
- Completion gate: BLOCKED
- Gate scope: independent-review 为 pass；document-integrity 仍因四份 UX（cli、menubar、settings、widget）和 architecture.md 已声明未编写而 blocked。不以 PASS 豁免完整性检查，不把下游设计缺口重新列为需求 finding。
- Hierarchy: 当前 CEv1 WorkUnit 无 contains 父边；主题 Documents 矩阵仍有六个未完成文档，未到整个主题完成边界，不查询或声明 Topic VERIFIED。
- Residual uncertainty: 自动维护来源的可验证识别、UX 标本及实现验收仍由后续设计/实现完成，本轮只确认需求修复。

Task checkpoint：ad-auxiliary-session-classification-doc-req；content_state `376128eaa35844f758d3b2c51084977e854d6a05b4f4a494f24cfbbf11302554`；gate BLOCKED。
提交建议：门禁 VERIFIED 后，提交 requirements.md、对应评审记录及 tasks.md 状态；前置为完成已声明下游文档并使文档完整性检查通过，交付仍需用户授权。
推送建议：门禁 VERIFIED 后，经授权推送 feature/auxiliary-session-classification；前置同上，remote/upstream 尚未解析，提交对象与签名需在交付时核验。

完成已声明的 ux/cli.md、ux/menubar.md、ux/settings.md、ux/widget.md 和 architecture.md，并通过文档完整性检查后重算本 WorkUnit 门禁；保留本轮 PASS。
