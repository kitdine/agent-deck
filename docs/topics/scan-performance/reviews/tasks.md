---
status: active
topic: scan-performance
subject: tasks.md
---

## Round 1 — 2026-10-09

## 📋 整套设计分解与依赖评审

📊 总体评分：7/10

✅ Verdict: FAIL

### 🔴 严重问题 — 必须修复

**T1-F1 · P1 · OPEN — Task 2 的 E1 前置条件包含它自身的实现产物。**

- 位置：`tasks.md:99–114`，对照 `performance-evaluation.md:227–233`。
- 行为风险：实施入口要求先通过尚未实现的恢复/首批发布/只读 bootstrap/native 验收，形成循环；或者实施者只能自行把 E1 降为现有 pilot，从而跳过明确要求。
- 证据：Task 2 写“前置Task1/E1”；E1 要求完整 restore、无缓存已有 DB 的 bootstrap、首次 day 优先和不等 worker 终结发布，并验证真实 native 路径。Task 1 仅基线与计量；这些能力全部列在 Task 2 范围，现有 pilot 又明确没有 native frame 证据。
- 💡 修复：分清 Task 2 的入口可行性证据与出口产品 E1 门；保留现有 pilot 的适用限制，将候选实现后的验收放在出口。不得把未过的产品门静默改记为已过。

### 🟡 改进问题 — 必须闭环

**T1-F2 · P2 · OPEN — 任务选择规则允许无授权地省略已列入范围的 OTel 能力。**

- 位置：`tasks.md:82–84,156–179`，对照 `requirements.md:79–92,140–150,186`。
- 行为风险：Task 6 被标为“独立可选能力”，Task 7 只要求“所有选入实现任务”与“适用”P 门；可以不交付 Settings/CLI receiver、capture-only/去重基础设施便结束专题，而需求仍把它们列在范围内。
- 证据：需求中的默认 off 是用户运行时开关，不是批准移除功能；缺精确身份时降为 capture_only 也不等于省略接收能力。任务分解没有规定谁选择排除、排除后如何调整需求/文档及完成边界。
- 💡 修复：将承诺的 OTel 能力纳入完成条件，或写出需用户批准的显式延期边界与承接位置；不要把运行时可选误当交付可省略。同步 Task 7 前置条件与对应门。

### 🟢 优点

七份 Documents 均有实际候选且无 stub，Widget n/a 有理由；Task 2 已承接 App-only 退出暂停，未将其拖到 Task 5；任务均列出文件范围与验证等级。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；actor `opencode`。
- Method: 独立主会话综合评审，按 requirements → UX → architecture/evaluation → tasks 联合核对；各文档保留单独 Round 1 和结论。
- Scope: 七份 Documents 声明、全部任务范围/前置/验收及依赖；同时覆盖 cli-design/manual、共享原型 manifest 与三份实验结果。
- Reviewed state at semantic review: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `daf6a95544c3c81ed4326f92c2eaf0571fcd2501`。
- ContentState at semantic review: `scan-performance:state:cc44a7933b3a4aa75d9d29e826ba8dce88b366d9e50a329b5afb9a5ebc65cc72`；SHA256(`head=<HEAD>;document=<blob>`)。本轮后续仅同步矩阵与记录指针；最终证据绑定同步后的状态，原始审查身份保留。
- Evidence: `bash scripts/check-topic-docs.sh scan-performance` 退出 0（不是仅目测完整性）；`make check-whitespace` 退出 0；manifest 43 项 SHA256 全匹配。分解中的门和文档依赖按原文追踪，不运行产品全量测试来评价未实现设计。
- Dependency coverage: requirements 与 performance-evaluation 独立 PASS；architecture、menubar、CLI、Settings 仍有本轮开放问题，因此本分解也尚不具备实施批准条件。问题详情以各自 carrier 为准，不复制其 findings。
- Final reviewed state after status synchronization: blob `ce562cddf78dffdde22aafb218bf7779890ac297`；ContentState `scan-performance:state:8971de03c4dd12551c69dbbdfdd597915115b271023f957f976b98254556a91e`。变化仅为两项 PASS 勾选与评审入口/历史状态说明；原有设计、任务行号与本轮问题未变。
- Completion gate: FAILED
- Gate evidence: Neo4j MCP 标准 gate-status 查询绑定最终 ContentState；document-integrity 通过，decomposition-and-dependency-coverage 被本轮适用 fail evidence 否定；无 invalidated 或 unresolved evidence。
- 下一步指令：修复：scan-performance / 全部设计评审记录的未闭环问题

## Repair candidate — 2026-10-09

Repairer: Codex；范围仅为上述Round 1的授权问题。原Round 1的分数、Verdict、ContentState与FAILED门保留。

- T1-F1：Task1/E0/pilot为入口，E1是Task2实现后的出口；产品门不以pilot代替。 **REPAIRED in candidate，待独立复评确认**。
- T1-F2：Task6为承诺交付能力，运行时off不等于省略；Task7含OTel门，延期须用户批准并明确承接。 **REPAIRED in candidate，待独立复评确认**。

本候选不创建Review Round 2，不自行声明Review PASS。文档/标本的当前内容身份、
验证结果与修复目标证据状态在本节完成验证后绑定；旧观察不重新标成新状态。

修复目标绑定：HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；document blob `4361ff953ac57e25ac8571ea4b8be68fed52e183`；
repair_target_state: `scan-performance:state:845156143c7df32734e39a705f7b091e2b8833409784dd00a143e26fbb01298b`。
验证：topic文档集、local links、review-record parser、whitespace、diff检查通过；
prototype build通过；120项mixed/telemetry/CLI合成交互通过，17截图在900×1500视口生成。
真实规模性能harness/摘要未改，未重扫私人数据、未测browser/native性能；产品Go/Swift未改。
证据同步：15个精确目标/证据节点读回全匹配，20关系预检ok并写入；标准gate-status查询
在上述新目标返回 `NOT_VERIFIED`，仅缺独立复评准则 `scan-performance:tasks.md:decomposition-and-dependency-coverage`。
document-integrity有当前pass；没有invalidated/unresolved impact。独立设计准则记not_verified，
作者不把修复当Review PASS，旧Round1 fail观察/关系未覆盖或重标。
依赖影响：requirements/performance-evaluation的blob保持原PASS身份，性能harness与三份数据摘要未改；
本次仅修复下游契约和合成UX，没有改需求目标或测量归因，复用其原结论。

## Round 2 — 2026-10-09

## 📋 任务分解与整套设计复评

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

- T1-F1 CLOSED：`tasks.md:102–120` 将 E0/有限 pilot 作为入口，完整 E1 作为 Task 2 实现后的出口；不再形成自身依赖，也未把 pilot 重新命名为产品 PASS。
- T1-F2 CLOSED：`tasks.md:83,164–183` 将 Task 6 列为承诺交付，明确 default off/capture_only 不能省略功能；Task 7 纳入该门，延期需用户批准和承接记录。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`；Method: 逐 finding 独立复核他方修复，联合核对七个记录及依赖；未修复或委派。
- Scope: 本记录两项 finding 及全设计批次依赖。A1-F1/F2、M1-F1、C1-F1/F2、S1-F1 均由相应 Round 2 关闭；requirements、performance-evaluation 保持原内容身份并复用通过结论。
- Reviewed candidate: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `4361ff953ac57e25ac8571ea4b8be68fed52e183`；ContentState `scan-performance:state:845156143c7df32734e39a705f7b091e2b8833409784dd00a143e26fbb01298b`。
- Final target: blob `3cfeae7250562c93ebd8a5bff9f7b3dae78499b8`；ContentState `scan-performance:state:3a27ec1e16750f1b78453a799f153d7011345027040d5e6ef8477d975ccb8108`。本轮仅将 Documents 五项 Review 勾选并更新就绪说明，任务设计内容不变；新证据绑定此最终状态，不重标旧观察。
- Evidence: 入口标准 MCP gate 仅缺独立分解准则，其余当前结构证据有效；59 项 manifest 哈希全匹配。原型关键交互独立抽查支持 UX 闭环；最终文档集及记录结构另作本轮收口检查。
- Completion gate: VERIFIED
- Gate evidence: 标准 Neo4j MCP gate-status 对最终目标返回 VERIFIED；最终结构与独立分解两项 required criteria 均通过，missing/invalidated/unresolved 均为空。原 candidate 的观察保留。
- Boundary: 仅设计批次通过，七个产品任务均未 Dev/Review；没有到达整个 topic 完成边界，不查询或宣称 Topic/Release VERIFIED。下一实施对象为 baseline-and-open-path，须用户明确开发授权。

Task checkpoint：ad-scan-performance-doc-tasks；content_state `3a27ec1e16750f1b78453a799f153d7011345027040d5e6ef8477d975ccb8108`；gate VERIFIED。

提交建议：将已审七文档、标本/实验依赖、矩阵与评审记录作为连贯设计批次授权提交。

推送建议：feature/scan-performance；须核验远端及分别获得提交/推送授权。
