---
status: active
topic: otel-version-tolerant-metering
subject: tasks.md
---

# tasks.md 评审记录

## Round 1 — 2026-11-15

## 📋 独立设计评审

📊 总分：7/10

✅ Verdict: FAIL

### 🔴 必须修复

OVM-D-R1-F1 — P2 — OPEN。

架构用途规则 v2 与诊断段及 tasks 的 identity-ownership-readmission 未定义来源状态迁移。
store.TelemetryBackgroundSessionCount 统计 telemetry_sessions 所有权 marker；
cmd/agentdeck/session_origin.go 的 readOriginStatusInputs 和 originSourceStatuses 使用该计数，
且仍报告 codex-otel-background-v1。

只有 none_fresh main 时会错误报告一个 background session；有效 jsonl_owned title 时会
漏计其辅助身份并报告 no_background_sessions。架构允许这两种状态，因此此路径必须修复。
关闭条件为明确 v2 rule_version、有效角色去重计数、main/旧版/冲突/JSONL-owned 口径、
schema 34 只读降级和对应回归。保留现有来源名称与表面。

主调度直接核实 telemetry.go:621 的查询及 session_origin.go 的读取与状态分支，采纳此发现。
架构与任务分解已补入修复候选，等待独立复评确认，当前发现保持 OPEN。

### 🟡 改进建议

无。

### 🟢 已确认优点

角色、请求用途与数字所有权分开；保留 legacy key、历史日期、JSONL 排他和前缀原子要求。
历史不可恢复证据保持保守，新的明确无持久化身份可前进。

### 📝 评审摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；tasks.md blob ba12dc98313d8939a84b48fa4b481ef67d75ec68。
依赖批次 requirements 990087bb6581f21ccd10d5b899e23f4add1a1bf5、
architecture da5ebf7d7f1fbedc61553b8e241503a765f3c12c、tasks ba12dc98313d8939a84b48fa4b481ef67d75ec68。
Codex pinned d32063f96c2dff1dc96af839b3952abb6acfeda4。

Reviewer: 全新上下文 Codex/gpt-6-astra，medium；主调度核实来源状态证据并记录结论。
Method: 独立只读源码追踪，跨仓库设计批次评审，无子委派。
Scope: 三份文档、受影响主合同、准入/结算/JSONL 前缀/origin 状态/schema/备份恢复，以及 pinned producer 初始化和用途路径。
Evidence: python3 scripts/check-docs.py 通过；复用本候选 check-topic-docs、make check-whitespace、git diff --check 通过。
Limitation: 发现决定性缺口后停止扩大验证；未证明全部 producer 生命周期及父 token 传播，实施验收仍须完成。
Completion gate: NOT_VERIFIED

修复范围为 OVM-D-R1-F1 的来源状态合同及任务覆盖。修复完成后独立复评，未开始产品实现。

## Round 2 — 2026-11-15

## 📋 独立设计复评

📊 总分：9/10

✅ Verdict: PASS

### 🔴 必须修复

无。OVM-D-R1-F1 -> CLOSED。架构来源状态 v2 段明确角色去重计数、main/旧版/非法及冲突
排除、JSONL-owned 辅助身份纳入，以及 ownership/override 独立口径。schema 34 只读
null/partial 降级及 receiver 三态完整，任务包含相应回归。此处关闭设计发现，产品尚未实现。

### 🟡 改进建议

无。

### 🟢 已确认优点

用途和数字所有权分离，legacy 身份缺口保守处理，前缀事务与发布恢复有明确约束。
Producer none_fresh 生命周期及无父 token 传播明确列为实施验收，未冒充已验证运行行为。

### 📝 评审摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；tasks.md blob ee19f6018cc2c0727180ccf2f86056fd093436ce。
依赖 requirements 990087bb6581f21ccd10d5b899e23f4add1a1bf5，architecture 844f472e44f154481cac02712bd33c39d0c419ad，tasks ee19f6018cc2c0727180ccf2f86056fd093436ce。
Reviewer: 全新上下文 Codex/gpt-6-astra，medium；主调度复核修复段及已核实源码并采纳。
Method: 冷上下文只读合同审查和 pinned Codex 源码核对。
Scope: F1 关闭与剩余 producer 身份、legacy key、JSONL 前缀/历史恢复、协调入口、用途 API 等设计边界。
Evidence: 文件 blob 匹配；复用修复候选的 check-docs、check-topic-docs、check-review-records、whitespace、diff 检查通过。
Limitation: 未运行新 producer 或 Rust 构建，完整生命周期与无父 token 传播留给实现验收。
Completion gate: VERIFIED

Task checkpoint

本设计文档评审通过。WorkUnit otel-version-tolerant-metering:tasks.md 在 ContentState otel-version-tolerant-metering:tasks.md:state:1a637d52220e8604 的三个 required criteria 全部 VERIFIED，无失效或未解决影响。提交建议可按文档任务准备，等待提交授权；推送建议等待已验证提交及单独授权。
Tasks 矩阵 Review 勾选及状态说明为本轮结果同步，不改变需求、接口、实施范围或验收标准。

## 范围修正 — 2026-11-15

用户在 epoch8 撤回 Codex producer 改造，并在 epoch9 要求内置自动规则、普通可读会话
自动归类和可选增强中性呈现。Fable5.2 重评估指出旧方案引入了超出目标的上游依赖。
旧 Round 的 PASS 与 VERIFIED 仅适用于当时的精确候选，不作为当前替代稿的审批。
当前文档已被 AgentDeck-only 版本替换，Review 清空，等待新的独立评审。
旧历史、用户override与事实均保留。此段是授权/适用性记录，不是新评审 verdict。

## Round 3 — 2026-11-15

## 📋 AgentDeck-only 自动规则设计评审

📊 总分：8/10

✅ Verdict: FAIL

### 🔴 必须修复

OVM-AUTO-R1-F1 — 高 — OPEN。每批最多64个身份补填来源，不能兑现首次结算消除全部
存量重复。反例为65个可读改名子线程来源，payload.id是子ID、accounting是父ID，且各有
otel_owned贡献。第65个仍NULL，原结算未命中JSONL并可ensureContributions恢复重复。
源码依据 usage.go的parse/checkpoint、telemetry.go的JSONL查询及贡献恢复。
主调度核对当前文档批预算和上述查询/恢复路径，采纳发现。

修复候选明确首次成功结算前自动连续分批、失败/取消未完成、事务内完成检查阻止晋升和
恢复，并新增65来源、批间崩溃和混合读取失败测试。待独立复评，当前保持OPEN。

### 🟡 改进建议

无额外偏好项。

### 🟢 已确认优点

AgentDeck-only、默认work、TUI/exec有界推断符合epoch9要求，未重新引入上游改造。
publishFacts事务守卫和历史readmission范围有源码依据。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；tasks.md blob f15d03f607d7247172c5b6903cd616213b9aece9。
Reviewer: 冷上下文Codex/gpt-6-astra medium，主调度裁定并记录。
Method: 只读文档—源码对照；独立审阅，无委派。
Scope: epoch9三文档，准入、历史来源与前缀事务等已检查路径。
Evidence: 候选blob匹配，复用check-docs、check-topic-docs、whitespace、diff检查通过。
Limitation: 决定性反例后停止扩大验证。版本边界、client/reset/cleanup、迁移触发器/消费者、
分类失败恢复与原生表面/UX n/a尚待复评完成；评分不代表这些范围通过。
Completion gate: NOT_VERIFIED

本候选不能进入实现。下一步关闭OVM-AUTO-R1-F1并完成剩余设计审阅。

## Round 4 — 2026-11-15

## 📋 自动规则设计复评

📊 总分：9.4/10（独立报告为对应95或94/100，按相同比例记录）

✅ Verdict: PASS

### 🔴 必须修复

无。OVM-AUTO-R1-F1 -> CLOSED。有限集合连续分批、成功批次持久化、失败维持NULL、
事务内完成检查以及未完成诊断关闭65来源反例，取消和重启不会恢复未确认重复贡献。
该关闭是设计判断，实施仍须证明所有列出的恢复行为。

### 🟡 改进建议

无新增发现。

### 🟢 已确认优点

AgentDeck-only与自动work/auxiliary符合epoch9用户要求。兜底不冒充证据，版本无关
准入与真实数字校验分离。历史补填、前缀、迁移触发器、原因码/原生消费者和发布恢复
都有明确实现/验收边界。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；tasks.md blob 8c8cbcbff648b0d25ddfdeb9c831ecbb519e27b2。
批次要求 requirements abf5eb016a32bdc380f60ef4272aa8d9fa1e1fd6、architecture d66f58faa7e6a2b57992ac5221e691367eec5da3、tasks 8c8cbcbff648b0d25ddfdeb9c831ecbb519e27b2。
Reviewer: 冷上下文Codex/gpt-6-astra medium；主调度核对当前修复与直接源码证据并采纳。
Method: 独立只读合同和调用路径核查，未委派。
Scope: 全部声明范围，补齐准入字节边界、client识别、来源生命周期、schema35/恢复/只读/Swift、
分类规则/回调重试和CLI/native状态及UX复用合同。没有因新发现而停止的剩余审查。
Evidence: 独立计算文件blob一致，复用修复候选的check-docs、check-review-records、whitespace、diff检查通过。
Limitation: 产品实现、Go/SQL实际等价、并发迁移、原生渲染仍待实施验收；设计评分不证明运行结果。
Completion gate: VERIFIED

Task checkpoint

本设计文档评审通过。WorkUnit otel-version-tolerant-metering:tasks.md 在 ContentState otel-version-tolerant-metering:tasks.md:state:595a0f16ef9e34a7 的4项必需证据 VERIFIED，无缺失、失效或未解决影响。提交建议可按当前任务准备，等待提交授权；推送建议等待已验证提交及单独授权。
Tasks Review勾选与状态句同步为机械结果记录，不改变实现范围和验收标准。
累计设计修复周期2/3，历史保持。

## Round 5 — 2026-11-15

## 📋 实现收口的合同影响评审

📊 总分：9.3/10（Task3独立批次整体评分）

✅ Verdict: PASS

### 🔴 必须修复

无。Task3 R3已独立确认本文件相关合同与实现及恢复复现一致。

### 🟡 改进建议

无。

### 🟢 已确认优点

自动默认、pending分类/重试、用量独立发布与规则版本检查点覆盖了普通失败路径。

### 📝 摘要

Reviewed state: HEAD 1b2faeb93a222cf5d2c0b04ec11c5bfa20ff4692；当前tasks.md blob 90f70e0674ba15ab73db92f189fb5e4231af654b。
Reviewer: 独立Fable5.2 medium，主调度记录，详见builtin-rule-set.md Round3。
Method: 代码及六合同一致性核查，复用设计和实现证据。
Scope: 本次实现收口的相关澄清和任务验证映射，Task3 Review勾选及末尾实现完成说明为机械状态同步，无验收或合同变化。
Evidence: Task3 R3明确逐份评估六合同通过；docs结构检查与实现测试证据按其记录复用。
Completion gate: VERIFIED

提交建议等待精确状态门禁及提交授权；推送建议等待已验证提交及单独授权。

CEv1最终查询：WorkUnit `otel-version-tolerant-metering:tasks.md`，ContentState `otel-version-tolerant-metering:tasks.md:state:cbd181ab17ca7138`，VERIFIED 4/4；missing、invalidated、unresolved均为空。
