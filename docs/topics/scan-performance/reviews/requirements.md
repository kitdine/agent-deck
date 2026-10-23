---
status: active
topic: scan-performance
subject: requirements.md
---

> **证据引用处置 — 2026-10-22**：Neo4j 恢复后，按用户要求移除本 topic 期间以文件形式保存的
> 回退证据与实验记录（原 `docs/topics/scan-performance/evidence/`，以及 Task 1 的
> `docs/evidence/records/`）。本文原指向这些文件的链接已改为纯文本，文中出现的上述路径均不再是
> 可用引用；原件仅可从提交 `c6ea1ca6551f525cc8a757ccf2e7889fabf36402` 的 Git 历史追溯。
> 结构化观测、内容身份与门禁结果以 Neo4j CEv1 为准；历史观察、结论与评审裁决保持原义。

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

## Round 5 — 2026-10-22

## 📋 全 topic 重新设计：需求文档评审

📊 总体评分：8/10

✅ 评审结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[requirements.md:76] SP-REQ-R5-F1（低）：“Task 4 保留的候选”沿用旧编号
- 行为风险：新矩阵中 Task 4 是尚未开始的 `parallel-source-pipeline`，旧编号指 `bounded-source-pipeline`；读者会把未实施任务的结果当作已测事实。
- 证据：tasks.md 的 Tasks 矩阵把 4 定义为 `parallel-source-pipeline`；26.8% pilot 来自旧 bounded 候选（c6ea1ca 的 tasks.md）。
- 💡 有界修复：改用锚点名或“旧 `bounded-source-pipeline` 候选”。
- Disposition：OPEN，由下一轮 Repair 关闭。

[reviews/serving-snapshot-and-bootstrap.md:1879] SP-REQ-R5-F2（低）：本次重写删除了 `#p03-独立验收项撤销--2026-10-19` 一节，历史记录的入链锚点失效
- 行为风险：历史决定的引用落空，读者无法找到用户 10-19 决定的原文。
- 证据：`python3 scripts/check-docs.py` 报告 `missing Markdown anchor: ../requirements.md#p03-独立验收项撤销--2026-10-19`。
- 💡 有界修复：参照本次证据链接的处置方式，在该历史记录中改为纯文本，并指明原文所在固定提交 `c6ea1ca` 的 requirements.md；历史裁决不改。
- Disposition：OPEN，由下一轮 Repair 关闭。

### 🟢 优点

P01–P11 与 `c376984` 逐字一致（已比对原表）；豁免与撤销已完整撤回；“不可达则返回设计、由用户决定”把豁免路径从实现与评审阶段移出。已测未达标事实都有可追溯来源，并且标明只作设计输入。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，冷读四份重设计文档并逐条核对源码、Git 历史（`c376984`、`14234ec`、`c6ea1ca`）、评审记录与 Neo4j；无委派，无产品修改。
- Scope：requirements.md 全文，以及它与 performance-evaluation/architecture/tasks 的一致性。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `a631df60f5a5af78e83860610cc91defa27670e2`；digest `ffff14ee95635598dc4f115be51d5a7c855bdf6e7bc9aca3ea12e49342ad8f9a`（SHA256(`head=<HEAD>;document=<blob>`)）。
- Evidence：`git show c376984:…/requirements.md` 的 P01–P11 行与当前全等；`scripts/check-docs.py` 报告上述两条失效锚点之一（另一条归 performance-evaluation）；`check-review-records.py --base HEAD` exit 0；`git diff --check` 通过。
- CEv1：WorkUnit `scan-performance:requirements.md` 仍绑定 `c376984` 时期的 ContentState；当前 digest 无对应记录，门禁 NOT_VERIFIED。FAIL 轮次不写入评审证据。
- 结论：两项低严重度问题都未关闭，按规则判 FAIL；需求实质内容没有阻断性问题。

## Repair candidate — 2026-10-22

Repairer：Claude Code 主会话；范围仅为 Round 5 的 SP-REQ-R5-F1..F2。Round 5 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS。

- SP-REQ-R5-F1 -> repaired in candidate：requirements.md 的“已测未达标事实”改称“旧 `bounded-source-pipeline` 任务保留的候选”，
  对照点改为“该任务开始前的 HEAD”；同文件来源段（第 60 行）同类的“Task 4 候选改动”一并改名。待独立复评确认。
- SP-REQ-R5-F2 -> repaired in candidate：`reviews/serving-snapshot-and-bootstrap.md` 的 2026-10-19 用户范围决定改为纯文本，
  指向固定提交 `c6ea1ca` 中 requirements.md 的“P03 独立验收项撤销 — 2026-10-19”一节；该历史裁决与其处置未改。待独立复评确认。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `cace39f60f38674b5e4830eb86e28be6ae66c70c`；
digest `7d243492e937b85e7b5bc578cadc57de62513b6fd228769f05c595bfb96be52f`（SHA256(`head=<HEAD>;document=<blob>`)）。
验证：`python3 scripts/check-docs.py` exit 0，F2 所报失效锚点已消失。P01–P11 表未改。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 6 — 2026-10-22

## 📋 全 topic 重新设计：需求文档复评

📊 总体评分：10/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-REQ-R5-F1 closed：第 60、76 行改称旧 `bounded-source-pipeline` 任务的候选，对照点改为“该任务开始前的 HEAD”；全文已无指代新矩阵的 “Task N”。
- SP-REQ-R5-F2 closed：`reviews/serving-snapshot-and-bootstrap.md:1875–1880` 改为纯文本，指向固定提交 `c6ea1ca` 中 requirements.md 的“P03 独立验收项撤销 — 2026-10-19”一节。已核实该提交中这一节存在（`### P03 独立验收项撤销 — 2026-10-19`，第 229 行）、c6ea1ca 已在 `origin/feature/scan-performance` 上，历史裁决原文未改。
- P01–P11 仍与 `c376984` 逐字一致；本轮修复未引入新问题。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，即 Round 5 评审者，与修复者是不同角色；逐条核对当前正文、固定提交与检查工具，不依据修复附记的自述。无委派，无产品修改。
- Scope：Round 5 的两项问题，以及修复引入回归的可能。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `cace39f60f38674b5e4830eb86e28be6ae66c70c`；digest `7d243492e937b85e7b5bc578cadc57de62513b6fd228769f05c595bfb96be52f`；ContentState `scan-perf-redesign-rr1-20261022-req-state`。
- Evidence：`python3 scripts/check-docs.py` exit 0，无缺失锚点；`check-review-records.py --base HEAD` exit 0；`git diff --check` 通过。
- CEv1：新增 ContentState 1 个、evidence 2 个（`…-integrity`、`…-semantic`，outcome `pass`）、关系 4 条，写入前按 ID 与指纹核实均不存在。门禁按 WorkUnit `scan-performance:requirements.md` 的 required criteria 查询：`document-integrity`、`boundary-and-acceptance` 在目标状态各有 1 条有效通过证据，均未被 invalidate 或 supersede，结果 **VERIFIED**（2/2）。
- 结论：两项问题均已关闭，无新问题，判 PASS。

Task checkpoint：ad-scan-performance-doc-req；content_state `7d243492e937b85e7b5bc578cadc57de62513b6fd228769f05c595bfb96be52f`；gate VERIFIED。
提交建议：经授权后提交 requirements.md 及其评审记录（含 Round 5/6），以及 SP-REQ-R5-F2 对 `reviews/serving-snapshot-and-bootstrap.md` 的改动；其余三份尚未通过，不要混入。
推送建议：经授权后推送到 `origin/feature/scan-performance`；推送前核验提交范围、贡献归属、trailer 与签名。
