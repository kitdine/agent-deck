---
status: active
topic: scan-performance
subject: requirements.md
---

## Round 1 — 2026-10-09

## 📋 需求边界评审

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

- 区分空库、已入库当天、待入库当天和保存视图，不把恢复旧值等同于实时更新。
- P01–P11 明确原生绘制、资源、数据正确性、生命周期及各自前置状态；不把 pilot 当产品验收。
- OTel 默认关闭与来源身份能力门明确，禁止近似跨源去重和虚构历史覆盖。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；actor `opencode` 经用户明确选择。
- Method: 独立主会话冷作者上下文评审；本会话未参与设计。逐条核对需求、六份下游候选、既有 CLI 契约、当前 Swift 刷新与点击源码，以及三份实验摘要；未委派。
- Scope: 需求边界与验收提案。下游设计缺陷不反向改写清楚且可实施的需求；分别由对应评审记录承接。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；document blob `f4077fda7d6c0b7b1a510055964398e72d17634e`。
- ContentState: `scan-performance:state:73cc09ddca20d4f26917d59d06964e96e86f1d7f13cdd85b21b2aeecc25608b9`。Recipe：SHA256(`head=<HEAD>;document=<blob>`)。
- Evidence: `bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace` 退出 0。`MenuBarItemController.swift:115–128` 确认点击直接 show；`EmbeddedHelperRunner.swift:1773–1809,1902–1936` 确认初始空快照、额度先于 host.refresh、保留 previous；`cmd/agentdeck/main.go:3851–3927` 核对 CLI 同步和 stored-only 边界。
- Dependencies: `performance-evaluation.md` 的测量结论仅在其限定范围内使用。架构、UX 和分解的本轮 FAIL 不代表需求或产品实现已经失败；实施仍须等待整套设计闭环。
- Completion gate: VERIFIED
- Gate evidence: Neo4j MCP 标准 gate-status 查询在上述精确 ContentState 上返回两项 required criteria 均通过，missing/invalidated/unresolved 均为空。
- 结论仅批准需求文档，不批准产品性能、实现、提交或发布。

### Task checkpoint：ad-scan-performance-doc-req

文档 blob `f4077fda7d6c0b7b1a510055964398e72d17634e`，门禁 VERIFIED，待授权交付。

提交建议：仅需求文档及其本轮评审边界；可等待设计修复后在完整批次中交付。

推送建议：仅考虑 `feature/scan-performance`；远端目标尚未核验，须先授权提交并核验实际交付范围，再另行授权推送。

## Round 2 — 2026-10-09

## 📋 需求结论复用与依赖影响复核

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

本记录原无开放 finding。下游修复补足来源覆盖、表面未知状态及任务边界，没有改变需求目标或缩减 OTel 承诺；原结论有效。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`；Method: 同状态证据复用与依赖影响审查，不重复需求全文评审/产品测试。
- Scope: 全设计复评中的 requirements 结论适用性；无新增、未闭环或回归问题。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `f4077fda7d6c0b7b1a510055964398e72d17634e`，与 Round 1 完全一致。
- ContentState: `scan-performance:state:73cc09ddca20d4f26917d59d06964e96e86f1d7f13cdd85b21b2aeecc25608b9`。
- Evidence: 复用 Round 1 两项 required criteria 的有效证据；修复新增的契约均落实已有需求，没有改变 source premise、数值目标或 scope；其他五记录 Round 2 独立关闭其问题。
- Completion gate: VERIFIED
- 产品任务仍未实现；本结论不授予 Git 交付或开发权限。

Task checkpoint：ad-scan-performance-doc-req；content_state `73cc09ddca20d4f26917d59d06964e96e86f1d7f13cdd85b21b2aeecc25608b9`；gate VERIFIED。

提交建议：需求文档及评审记录，可纳入七文档设计批次的授权提交。

推送建议：feature/scan-performance；远端未核验，先完成授权提交，再另行授权推送。

## Round 3 — 2026-10-18

## 📋 时间目标补充独立设计复核

Verdict: PASS

Completion gate: VERIFIED

2026-10-19元数据补记：本字段显式表达下文已记录的当轮VERIFIED及其原ContentState，未重判历史内容或改绑到当前版本。

- Reviewer: Codex；仅审查设计补充，没有编写产品实现或改动本次被审需求。无委派。
- Scope: 2026-10-13 批准的 P01/P02/P04/P11 ×1.5 门槛及人工等待归属；沿用 Round 2 未变的需求边界。
- P01 为 300/600ms，P02 为 1.125s，P04 为 3s，P11 为 4.5s；其余目标、默认额度/全部面板、正确性、覆盖和最终百次要求没有降低。
- 内存快开、磁盘恢复、已提交库无缓存、真正首次导入继续分开；部分数据不补零，旧值不冒充新值。人工等待不能计为启动成绩，也不能从总时长倒减得到启动成绩。
- 联合核对 performance-evaluation 的独立模型/帧时钟、当前采集器的不足百次不报告尾分布，以及 tasks 的 Task 2 出口；没有新需求选择或开放 finding。
- 本结论只确认设计补充自洽，不确认目标已达成。当前 4,147 文件输入也不能替代 P09 的 ≥5,071 文件规模门。
- 内容身份、结构检查及 CEv1 结果在本轮最终证据中绑定；历史 Round 1/2 不改写。

本轮最终内容：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；blob `5b68f1f1c317fc5510333ef5fe6f070ed2147931`；digest `5b749723002ac47d944aeec8991102e2ca6aa36803847016eb372191aace8ed0`。
ContentState：`scan-serving-design-supplement-20261018-req-state-5b749723002a`。标准 Neo4j gate：**VERIFIED**，2/2 required，missing/invalidated/unresolved 均为空。
结构、空白及 diff 检查均通过；新增 6 项文档证据与 12 条关系完成预检、计数及精确回读。
Task checkpoint：设计补充已复核，当前未提交版本仍待单独授权交付；不覆盖整体 Task 2/E1 门禁。


## Round 4 — 2026-10-19

## 📋 需求范围变更独立评审

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

P03撤销保留展示路径不等待helper/扫描/OTel的约束；P04/P11接受有界历史证据，不宣称当前构建严格p95通过。P01/P02、压力、正确性及OTel承诺保持。

### 📝 总结

- Reviewer/Method：Codex主会话，独立核对本次范围变更、依赖文档、实现接口及持久证据，无委派、无产品修改。
- Scope：2026-10-19的P03撤销与P04/P11有界接受，以及相应任务状态；此前Round3的未变设计结论保留。历史finding已关闭，无新增或回归finding。
- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；document blob `2570ec1e469e20300d8410910f8de935a159d146`；digest `c5e705d5e7a69f78f780bceea539123e5189554acdaa59e5f01d5db6e8194d88`；ContentState `scan-serving-design-r4-2V6mNrA-req-state`。文档身份按SHA256(`head=<HEAD>;document=<blob>`)；未变原型不重新验收。
- Evidence：本轮check-docs最初仅报告三份历史Round3缺可解析gate；以其已有VERIFIED事实补齐元数据，保留历史正文/身份。最终文档集、链接、review结构、空白及diff检查以修正后的状态执行；不以文档PASS替代产品性能验收。
- CEv1：当前文档两项required准则分别绑定结构检查及本轮语义审查，不覆盖旧观察；门禁以本轮精确文档状态查询。Task2整体门仍未完成。

Task checkpoint：ad-scan-performance-doc-req；content_state `c5e705d5e7a69f78f780bceea539123e5189554acdaa59e5f01d5db6e8194d88`；gate VERIFIED。
提交建议：经授权提交本次已审requirements.md变更及必要记录；不将未完成Task2的产品范围混入文档交付。
推送建议：经授权推送origin/feature/scan-performance；前置为核验文档提交范围、贡献归属、trailer与SSH签名。


### Task2 用户批准的验收修订附记 — 2026-10-20

用户明确授权修改必要标准，并接受当前Task2以PASS完结。
本附记记录用户批准的验收豁免，不发布新的独立审查轮次；上述历史审查事实保留。
本次仅将open-path-latency、startup-bootstrap、e1-native-performance三项从Task2
必需完成条件收窄为用户豁免，其余十项仍必需。100次原生首击p95=536.842034ms、
p99=623.638814ms的原预算失败保留。当前验收结论由tasks.md顶端最终状态和
evidence/task2-acceptance-waiver-20261020-receipt.json的精确门禁/本地完成回执管理。
本次用户直接批准的文字调整无需另造独立Review PASS；无产品代码变更。
