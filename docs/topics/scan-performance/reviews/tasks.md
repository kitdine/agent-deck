---
status: active
topic: scan-performance
subject: tasks.md
---

> **证据引用处置 — 2026-10-22**：Neo4j 恢复后，按用户要求移除本 topic 期间以文件形式保存的
> 回退证据与实验记录（原 `docs/topics/scan-performance/evidence/`，以及 Task 1 的
> `docs/evidence/records/`）。本文原指向这些文件的链接已改为纯文本，文中出现的上述路径均不再是
> 可用引用；原件仅可从提交 `c6ea1ca6551f525cc8a757ccf2e7889fabf36402` 的 Git 历史追溯。
> 结构化观测、内容身份与门禁结果以 Neo4j CEv1 为准；历史观察、结论与评审裁决保持原义。

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

## Round 3 — 2026-10-18

## 📋 Task 2 验收职责与时间目标补充独立设计复核

Verdict: PASS

Completion gate: VERIFIED

2026-10-19元数据补记：本字段显式表达下文已记录的当轮VERIFIED及其原ContentState，未重判历史内容或改绑到当前版本。

- Reviewer: Codex；审查范围为 Task 2 的出口/阶段职责及与 requirements、performance-evaluation 补充的依赖一致性，无产品修改、无委派。
- Dev 负责实现、必要回归和关键场景；独立 Review 负责完整 E1、六条件每条件 ≥100、启动、真实压力及适用正确性验收。E1 仍是 Task 2 出口，没有移动到 Task 7 或成为 Task 2 自身的入口依赖。
- Task 3 继续依赖 Task 2 出口；Tasks 3–7 未启动。OTel 默认 off 不取消 Task 6，未改变原设计 Round 2 的分解边界。
- 设计补充 Review 标记恢复为已审；Task 2 整体 Review 标记保持未完成，避免把 Round 10 的 finding-disposition PASS 误读为整个任务通过。Round 10 历史结论及事实不变。
- 未发现新的设计 finding；启动、六条件百次、真实压力和整体实现审查仍由 Task 2 承接，不授权提交、推送或安装。最终 tasks.md 内容身份在同步后绑定，文档设计门与实现门分别查询。

本轮最终内容：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；blob `49c3cf92143be42dafca55c20c981a2f541303d7`；digest `d7538166d3375f3e3a2d04fc2e4a704090b95b4628d51110f9b6c103fd11b8ae`。
ContentState：`scan-serving-design-supplement-20261018-tasks-state-d7538166d337`。标准 Neo4j gate：**VERIFIED**，2/2 required，missing/invalidated/unresolved 均为空。
结构、空白及 diff 检查均通过；新增 6 项文档证据与 12 条关系完成预检、计数及精确回读。
Task checkpoint：设计补充已复核，当前未提交版本仍待单独授权交付；不覆盖整体 Task 2/E1 门禁。


## Round 4 — 2026-10-19

## 📋 任务分解及验收范围变更独立评审

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

最新范围决定明确优先于历史交接，Task2继续承担首击/启动/压力出口，Task3依赖该门；P03撤销与P04/P11例外没有转移其他任务义务。Review与完成门分离，Tasks3–7未启动。

### 📝 总结

- Reviewer/Method：Codex主会话，独立核对本次范围变更、依赖文档、实现接口及持久证据，无委派、无产品修改。
- Scope：2026-10-19的P03撤销与P04/P11有界接受，以及相应任务状态；此前Round3的未变设计结论保留。历史finding已关闭，无新增或回归finding。
- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；document blob `65175860c48aa9ba3ae8ff96196dbc690e76462b`；digest `31c747db2430143fdb97b9a0a0691e04382eacaf8f3272e9fa6a95d3f3361b38`；ContentState `scan-serving-design-r4-2V6mNrA-tasks-state`。文档身份按SHA256(`head=<HEAD>;document=<blob>`)；未变原型不重新验收。
- Evidence：本轮check-docs最初仅报告三份历史Round3缺可解析gate；以其已有VERIFIED事实补齐元数据，保留历史正文/身份。最终文档集、链接、review结构、空白及diff检查以修正后的状态执行；不以文档PASS替代产品性能验收。
- CEv1：当前文档两项required准则分别绑定结构检查及本轮语义审查，不覆盖旧观察；门禁以本轮精确文档状态查询。Task2整体门仍未完成。

Task checkpoint：ad-scan-performance-doc-tasks；content_state `31c747db2430143fdb97b9a0a0691e04382eacaf8f3272e9fa6a95d3f3361b38`；gate VERIFIED。
提交建议：经授权提交本次已审tasks.md变更及必要记录；不将未完成Task2的产品范围混入文档交付。
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

## 📋 全 topic 重新设计：任务矩阵评审

📊 总体评分：7/10

✅ 评审结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

[tasks.md:99] SP-TASKS-R5-F1（中）：删除 `prototype/tools/scan-performance-*` 与“UX 标本无变化”（line 42–43）相矛盾
- 行为风险：同 SP-ARCH-R3-F1。Task 1 若照此删除，会使 `ux/prototype/serving/manifest.json` 的 8 条固定身份失效，`prototype/README.md` 也会悬空，任务矩阵却没有任何承接项。
- 💡 有界修复：与 architecture 同步处置；如需删除，把 manifest/README 的更新和 UX Review 的重开写入 Task 1 的文件与准则。

[tasks.md:104–105] SP-TASKS-R5-F2（中）：E0 pilot 定义与 performance-evaluation 冲突。同 SP-PE-R5-F2：在性能评估中统一定义 pilot 表，本处改为引用。

[tasks.md:119,168] SP-TASKS-R5-F3（中）：P09 “≥20 次/对”没有固定。同 SP-PE-R5-F3。

### 🟡 建议改进 — 推荐

[tasks.md:115–117] SP-TASKS-R5-F4（低）：B0 被指派给 P08，但性能评估中没有对应方法。同 SP-PE-R5-F9。

[tasks.md:115,121,158,160] SP-TASKS-R5-F5（低）：“Task 1 原交付”“保留的 Task 4 候选”指旧任务，“Task 1 工具缺陷”“Task 2 关键路径”指新任务，同一编号在同一矩阵中指向两套任务。改用锚点名或“旧 `<anchor>`”。

[tasks.md:136] SP-TASKS-R5-F6（低）：“p95 >150 ms 则停止返回设计”沿用了“不可达”的前提。同 SP-PE-R5-F7。

### 🟢 优点

八项任务都有文件边界、删除清单、验证级别与命名准则。删除清单对照仓库核实，列出的脚本、DEBUG 采集与 `AGENTDECK_SCAN_PROFILE` 引用都存在，引用面也覆盖完整。依赖顺序与性能评估 E0–E7 一一对应；“正式批次失败即未完成、不可达则返回设计”把豁免路径关闭；WorkUnit 不复用旧 ID，符合已知的撞 ID 风险。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，冷读并核对删除目标、引用面、Beads 文档任务与 CEv1 WorkUnit；无委派，无产品修改。
- Scope：tasks.md 全文，以及它与 performance-evaluation、architecture、UX 标本的一致性。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `7de32b61236eaa11b8f0a2bde584c8815df105a4`；digest `72b35b43b27b890668ffe0f4e6350676c3308bda6d662cce0e1a8b0ece4837fb`。
- Evidence：`fd`/`rg` 核对删除目标与外部引用；Beads `ad-scan-performance-doc-*` 四项均为 `in_review`；`scripts/check-docs.py` 未报告本文问题；`git diff --check` 通过。
- CEv1：WorkUnit `scan-performance:tasks.md` 仍绑定旧 ContentState，当前 digest 无记录，门禁 NOT_VERIFIED。
- 结论：3 项中严重度、3 项低严重度问题未关闭，判 FAIL。

## Repair candidate — 2026-10-22

Repairer：Claude Code 主会话；范围仅为 Round 5 的 SP-TASKS-R5-F1..F6。Round 5 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS。各项均待独立复评确认。

- SP-TASKS-R5-F1 -> repaired in candidate：与 SP-ARCH-R3-F1 同步，Task 1 删除清单去掉 `prototype/tools/scan-performance-*`，
  并要求删除前的引用搜索排除这些标本与 README 说明；“重新设计的评审范围”写明任何任务都不删改它们，UX 标本身份保持有效。
- SP-TASKS-R5-F2 -> repaired in candidate：Task 1 的 E0 改为引用性能评估“样本与统计”中的工具 pilot 表，不再重复数值。
- SP-TASKS-R5-F3 -> repaired in candidate：P09 改为固定 20 对（`baseline-remeasure` 与 `parallel-source-pipeline` 两处）。
- SP-TASKS-R5-F4 -> repaired in candidate：B0 只用于 P09，并写明 P08 为绝对门、不与 B0 比较。
- SP-TASKS-R5-F5 -> repaired in candidate：旧任务改称旧 `baseline-and-open-path` / 旧 `bounded-source-pipeline`，
  新任务一律用锚点名（含第 172、184 行同类引用）；全文已无 “Task N”。
- SP-TASKS-R5-F6 -> repaired in candidate：下限探测改为与性能评估 E2 相同的两档表述；
  UX 变更条件改为“触发返回设计且用户选择替代窗口形态”。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `a717b54045f2eb396a33337a3a4ac08f53d8b0fa`；
digest `96def29caf3d7477b678ed43b658c1f93abd569c2eedb2274b52844aacfef0a1`。
验证：`python3 scripts/check-docs.py` exit 0。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 6 — 2026-10-22

## 📋 全 topic 重新设计：任务矩阵复评

📊 总体评分：9/10

✅ 复评结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[tasks.md:105] SP-TASKS-R6-F1（低，回归）：时钟同源测试仍写“三种时钟”
- Disposition：regressed。SP-PE-R5-F4 的修复把同源测试扩为四个来源（含 `ri_proc_start_abstime`），`measurement-harness` 的验证行没有同步。
- 行为风险：按任务矩阵实现时会漏测进程起点时钟，而 P02/P11 的另报项依赖它。
- 💡 有界修复：改为引用性能评估的时钟同源要求，或写明四个来源。

### 🟢 优点

- SP-TASKS-R5-F1 closed：删除清单已排除 `prototype/tools/scan-performance-*`；评审范围一节写明任何任务都不删改这些文件，引用搜索也排除了它们。
- SP-TASKS-R5-F2 closed：E0 改为引用性能评估的工具 pilot 表。
- SP-TASKS-R5-F3 closed：`baseline-remeasure` 与 `parallel-source-pipeline` 两处均固定为 20 对。
- SP-TASKS-R5-F4 closed：B0 只用于 P09。
- SP-TASKS-R5-F5 closed：旧任务均写成旧 `<anchor>`，新任务用锚点名，全文已无 “Task N”。
- SP-TASKS-R5-F6 closed：下限探测改为两档，UX 变更条件改为“触发返回设计且用户选择替代窗口形态”。

### 📝 总结

- Reviewer/Method：Claude Code 主会话（Round 5 评审者），核对当前正文及其与性能评估、架构的一致性；无委派，无产品修改。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `a717b54045f2eb396a33337a3a4ac08f53d8b0fa`；digest `96def29caf3d7477b678ed43b658c1f93abd569c2eedb2274b52844aacfef0a1`。本轮之后，状态同步把 requirements.md 的 Review 打勾，tasks.md 的 blob 随之改变，下一轮按新 blob 绑定。
- Evidence：`check-docs.py` exit 0；`rg` 核对。
- CEv1：FAIL，不写入，门禁 NOT_VERIFIED。
- 结论：6 项均已关闭，但出现 1 项回归，判 FAIL。

## Repair candidate — 2026-10-22（Round 6 后）

Repairer：Claude Code 主会话；范围仅为 Round 6 的 SP-TASKS-R6-F1。Round 6 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS，待独立复评确认。

- SP-TASKS-R6-F1 -> repaired in candidate：`measurement-harness` 验证行的时钟同源测试改为引用性能评估·原则第 2 条，
  并列出四个来源（Swift `clock_gettime_nsec_np`、Go `unix.ClockGettime`、`NSEvent.timestamp`、`ri_proc_start_abstime`），偏差 ≤1 ms；
  全文已无“三种时钟”。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `f17e9e8792bd13f05fa692ea06d6b1db347d3624`（含此前 requirements Review 勾选）；
digest `9f72e3b8fa75039ddf1d9e6f078805768867b372b72ec4f7df8effe036911d3c`。
验证：`python3 scripts/check-docs.py` exit 0；`git diff --check` 通过。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 7 — 2026-10-22

## 📋 全 topic 重新设计：任务矩阵复评

📊 总体评分：10/10

✅ 复评结论：PASS

完成门禁：BLOCKED（CEv1 provider 本会话不可达）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-TASKS-R6-F1 closed：`measurement-harness` 验证行引用性能评估·原则第 2 条，列出四个来源（Swift `clock_gettime_nsec_np`、Go `unix.ClockGettime`、`NSEvent.timestamp`、`ri_proc_start_abstime`），偏差 ≤1 ms，与性能评估一致；全文无“三种时钟”。
- SP-TASKS-R5-F1..F6 在当前内容中未回归。

### 📝 总结

- Reviewer/Method：Claude Code 主会话（Round 5/6 评审者），核对当前正文及其与性能评估、架构的一致性；无委派，无产品修改。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `f17e9e8792bd13f05fa692ea06d6b1db347d3624`；digest `9f72e3b8fa75039ddf1d9e6f078805768867b372b72ec4f7df8effe036911d3c`。本轮之后状态同步把 architecture.md、performance-evaluation.md、tasks.md 三行的 Review 打勾，tasks.md 的 blob 随之改变；该差异只有这三个复选框，证据记录须绑定同步后的 blob。
- Evidence：`scripts/check-docs.py` exit 0；`scripts/check-review-records.py` exit 0；`git diff --check` 通过。
- CEv1：Neo4j MCP 本会话连接失败，未查询、未写入；门禁 BLOCKED，待 provider 恢复后记录并重查。
- 结论：唯一发现关闭，判 PASS；完成门禁未 VERIFIED。

## Round 8 — 2026-10-23

## 📋 冻结真实语料夹具变更：任务矩阵评审

📊 总体评分：10/10

✅ Verdict: PASS

完成门禁：VERIFIED（CEv1 scan-perf-corpus-r-20261023-tasks-state，绑定状态同步后 blob 520a510c，两项 required criteria 均通过）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 文档矩阵把本轮改动的四份文档 Review 复位，评审顺序表第 2 行反映新夹具契约。
- 全局约束区分测试（合成夹具）与测量（冻结语料的平移工作副本），并把唯一一次读取真实日志限定在 `measurement-harness` 经批准的冻结中；`measurement-harness` 交付项列出 `corpus freeze`/`corpus shift`、`small` 仅供测试，验证项新增平移确定性测试；基线段改为同一冻结语料的同一工作副本。均与 performance-evaluation.md、requirements.md 一致。
- SP-TASKS-R5..R7 已关闭的发现在当前内容中未回归。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，核对本轮 diff 及其与需求、性能评估、架构的一致性；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `4df3e113ee80484bba3b21ca13e86d223738e875`；digest `fb6568b685b4653c93bb330fd385080819a7a1841971f0a06acbff9bbb4100a2`。本轮之后状态同步把 architecture.md、tasks.md 两行的 Review 打勾，tasks.md 的 blob 随之改变；差异只有这两个复选框，证据绑定同步后的 blob。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 依赖：performance-evaluation.md 本轮 FAIL；SP-PE-R8-F1/F2 的修复要求同步本文件 `measurement-harness` 验证项，届时本文件改动须重评，本结论只覆盖当前内容。
- 结论：无发现，判 PASS。

## Repair candidate — 2026-10-23（Round 8 后，承接 performance-evaluation Round 8–9）

Repairer：Claude Code 主会话；范围为 performance-evaluation.md Round 9 总结记录的跨目标承接项（SP-PE-R8-F1 (d) 与 SP-PE-R8-F2/F3 引起的本文件同步），以及与 SP-PE-R9-F1/F2 修复的一致性。Round 8 的 PASS 只覆盖其所评内容；本次修改使本文件需要重新评审。本节不是 Review Round，也不自行声明 PASS。

- 冻结措辞 -> repaired in candidate：`measurement-harness` 交付项把“冻结一次”改为“每次冻结（含重新冻结）都须用户事先批准”（2026-10-23 已批准一次重新复制），并写明批次按性能评估“工作副本、模板与样本起点”编排：固定 HOME 路径、模板记录源树绑定指纹、按运行单元核对与重置、写入条件前后重新生成工作副本。
- 验证项 -> repaired in candidate：夹具确定性测试增加平移后落入春季不存在与秋季重复时刻的确定性结果，以及逐文件回退为 UTC 平移且文件内先后不变；新增模板起点测试：运行单元起点指纹与模板一致、复制得到的源树指纹不同并判为工具无效、从模板起步的 no-op 扫描读取字节与 decode 记录数为 0、累积写入序列内不重置。
- 文档矩阵：tasks.md 的 Review 复位为未勾选。

修复目标：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `5fcc157c95dd20f7ed2f5dae469a6e8f3bdd2228`；
digest `018dcaef4d95024413fc68618b17fc45c99ce3ddd1c46647f81a8e96cfe70a67`。
验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` 仍为 exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件。
CEv1：修复不跨越完成边界，未写入；Round 8 记录的 VERIFIED 门绑定旧 blob，不覆盖当前内容，待复评。

## Round 9 — 2026-10-23

## 📋 冻结真实语料夹具变更：任务矩阵复评（承接 performance-evaluation Round 8–9）

📊 总体评分：9/10

✅ 复评结论：FAIL

完成门禁：NOT_VERIFIED（复评未通过；Round 8 的 VERIFIED 绑定旧 blob，不覆盖当前内容）

### 🔴 严重问题 — 必须修复

[tasks.md:79–80] SP-TASKS-R9-F1（低）：全局约束仍写“经用户批准的一次冻结”，与第 96–97 行及性能评估允许经批准重新冻结相矛盾
- 处置：仍开放（属于 performance-evaluation.md Round 9 承接到本文件的冻结措辞同步项；上轮承接只点名了第 96–97 行，第 80 行同一措辞未改）。
- 行为风险：第 80 行“除 `measurement-harness` 中经用户批准的一次冻结外，不读取真实日志或生产数据库”把读取真实日志限定为一次。第 96–97 行与性能评估第 127–128、138–141 行都允许在代表性不足时经批准重新冻结，requirements.md 第 181–183 行写的是“每次冻结（含重新冻结）”。重新冻结时，实现者按全局约束会认为它被禁止，按交付项又会认为它被允许，同一文件内两处相反。
- 证据：`rg -n '一次冻结|每次冻结' tasks.md` 命中第 80 行与第 96 行，两处措辞不同。
💡 有界修复：把第 80 行改为“除 `measurement-harness` 中经用户批准的冻结（含重新冻结）外”，其余不变。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 承接项中的冻结措辞（第 96–97 行）已改为“每次冻结（含重新冻结）都须用户事先批准”，并写明批次按性能评估“工作副本、模板与样本起点”编排（第 98–99 行）。
- 承接项中的验证项已补齐（第 113–116 行）：平移后落入春季不存在与秋季重复时刻的确定性结果；逐文件回退为 UTC 平移且文件内先后不变；模板起点测试覆盖运行单元起点指纹一致、复制得到的源树被判为工具无效、从模板起步的 no-op 扫描读取字节与 decode 记录数为 0、累积写入序列内不重置。与 performance-evaluation.md 第 155–161、180–189 行一致。
- 文档矩阵中 tasks.md 的 Review 已复位，符合内容变化后需重评的规则。

### 📝 总结

- 逐项处置：performance-evaluation Round 9 承接到本文件的同步项中，验证项 closed，第 96–97 行冻结措辞 closed，第 80 行同一措辞未同步，记为 SP-TASKS-R9-F1（低），仍开放。
- Reviewer/Method：Claude Code 主会话（Round 8 评审者），核对修复候选的当前内容及其与 performance-evaluation.md、requirements.md 的一致性；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `5fcc157c95dd20f7ed2f5dae469a6e8f3bdd2228`；digest `018dcaef4d95024413fc68618b17fc45c99ce3ddd1c46647f81a8e96cfe70a67`。本轮之后状态同步把 performance-evaluation.md 一行的 Review 打勾，tasks.md 的 blob 随之改变。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 结论：一项低严重度发现仍开放，判 FAIL；修复范围为 SP-TASKS-R9-F1。

## Repair candidate — 2026-10-23（Round 9 后）

Repairer：Claude Code 主会话；范围仅为 Round 9 的 SP-TASKS-R9-F1。Round 9 的分数、Verdict 与 NOT_VERIFIED 门保留；本节不是 Review Round，也不自行声明 PASS，待独立复评确认。

- SP-TASKS-R9-F1 -> repaired in candidate：第 80 行“除 `measurement-harness` 中经用户批准的一次冻结外”改为“除 `measurement-harness` 中经用户批准的冻结（含重新冻结）外”，其余不变；`rg -n '一次冻结'` 在本文件已无命中，与第 96–97 行、performance-evaluation.md 第 127–128、138–141 行及 requirements.md 第 181–183 行一致。

修复目标：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `e79c7dc919958ede728951dee2f61202f50e847d`；
digest `537d415756328645cb44be2a24750e46e58db3ec7a3fbe4dbad6989effb6f1d5`。
验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` 仍为 exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 10 — 2026-10-23

## 📋 冻结真实语料夹具变更：任务矩阵复评

📊 总体评分：10/10

✅ 复评结论：PASS

完成门禁：VERIFIED（CEv1 scan-perf-corpus-rr3-20261023-tasks-state，绑定状态同步后 blob f83ac6e，两项 required criteria 均通过）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-TASKS-R9-F1 closed：第 80 行改为“除 `measurement-harness` 中经用户批准的冻结（含重新冻结）外，不读取真实日志或生产数据库”；`rg -n '一次冻结'` 在本文件无命中，与第 96–97 行、performance-evaluation.md 第 127–128、138–141 行及 requirements.md 第 181–183 行一致。
- Round 9 已关闭的承接项（第 96–99 行冻结与批次编排措辞、第 113–116 行平移与模板起点验证项）在当前内容中未回归。

### 📝 总结

- 逐项处置：SP-TASKS-R9-F1 closed；无新发现。
- Reviewer/Method：Claude Code 主会话（Round 8/9 评审者），核对修复候选的当前内容及其与 performance-evaluation.md、requirements.md 的一致性；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `e79c7dc919958ede728951dee2f61202f50e847d`；digest `537d415756328645cb44be2a24750e46e58db3ec7a3fbe4dbad6989effb6f1d5`。本轮之后状态同步把 tasks.md 一行的 Review 打勾，blob 随之改变；差异只有这一个复选框，证据绑定同步后的 blob。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 本记录引用的其他记录发现均已在其所属记录中关闭，此处逐项注明以免被读作本文件的开放项（不改变本轮结论）：
  - A1-F1 closed：见 `reviews/architecture.md` Round 2（`A1-F1 CLOSED`）。
  - M1-F1 closed：见 `reviews/ux-menubar.md` Round 2。
  - C1-F1 closed：见 `reviews/ux-cli.md` Round 2。
  - S1-F1 closed：见 `reviews/ux-settings.md` Round 2。
  - SP-ARCH-R3-F1 closed：见 `reviews/architecture.md` Round 4，Round 5 确认未回归。
  - SP-PE-R5-F2 closed：见 `reviews/performance-evaluation.md` Round 6。
  - SP-PE-R5-F3 closed：见 `reviews/performance-evaluation.md` Round 6。
  - SP-PE-R5-F4 closed：见 `reviews/performance-evaluation.md` Round 6；本文件同步项 SP-TASKS-R6-F1 已于本记录 Round 7 关闭。
  - SP-PE-R5-F7 closed：见 `reviews/performance-evaluation.md` Round 6。
  - SP-PE-R5-F9 closed：见 `reviews/performance-evaluation.md` Round 7（Round 5 的 F1..F5、F7..F10 未回归）。
  - SP-PE-R8-F1 closed：见 `reviews/performance-evaluation.md` Round 9；第 (d) 项本文件同步已于本记录 Round 9 关闭。
  - SP-PE-R8-F2 closed：见 `reviews/performance-evaluation.md` Round 9；本文件同步已于本记录 Round 9 关闭。
  - SP-PE-R9-F1 closed：见 `reviews/performance-evaluation.md` Round 10。
- 结论：唯一发现关闭，判 PASS。

## Round 11 — 2026-10-26

## 📋 E2 追加探测任务分解评审

📊 总体评分：8/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

[tasks.md:151–166] SP-TASKS-R11-F1（中）：新增两个 Perf 探测条件，但 Task 3 的文件边界没有覆盖实现它们所需的测量工具入口。

- 行为风险：范围要求实现并最终删除或采用 `P01-floor-warm`／`P01-floor-panel`，文件清单却只列产品展示／恢复路径，未包含 `PerfDriver.swift`、`cmd/agentdeck-perf` 条件编排和统计。实现者必须越过列明的文件边界，或只能在 App 内实现一个无法由既定批次工具运行和统计的探测。不能把这项新工作默认为已经完成的 measurement-harness 任务负责。
- 证据：Task 3 文件清单位于 151–153 行，追加条件与退出／清理职责位于 157–166 行。当前 `cmd/agentdeck-perf/run.go:116–119` 从 `conditionPlans()` 查找条件，不存在即返回 unknown condition；778 行只注册现有 `P01-floor`。`stats.go:274–279` 有独立起止点条件表，653 行拒绝未知统计条件。`PerfDriver.swift:10–20` 持有 floor 开关与静态视图，24–69 行定义驱动脚本协议。新增条件不是只改 MenuBarItemController 就能接入的既有工具能力。
- 💡 有界修复：在 Task 3 文件／验证边界中明确纳入为这两个探测所需的 Perf 驱动、条件编排／统计及对应定向回归；保留 Perf-only、pilot／正式样本、候选不可行与工具无效分流和结束清理约束。不扩大为重写测量工具，不要求先实现后再评文档。
- 处置：open，待修复。本轮不修改任务定义或产品源码。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- Task 3 的决策出口与已评性能契约／架构一致：缺少有效结果时阻塞，不把缺失 p95 当数值失败，UX／目标变更仍须用户评审。
- 非窗口工作可并行推进，探测结束需清理或正式采用；原 correctness、P01 各层及 P02/P03/P04/P11 验收未被撤回。
- SP-TASKS-R9-F1 的冻结措辞修复保留，未回归；文档集检查通过。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立任务分解评审、依赖契约对照、CodeGraph 定位与条件入口源码核实；无委派，无产品／测试／配置修改。
- Scope：当前 E2 追加探测增补及其文件边界、验证、依赖和文档矩阵；历史已关闭发现维持原处置。发现确定边界缺口后停止扩大验证。
- Reviewed state：HEAD `5e8c98cbcfefc6366fb5b1fa579dcee119d8276e`；document blob `7411ff26fde01b285ef07fc484978cdcd02c53ec`；SHA256(`head=<HEAD>;document=<blob>`) = `2c0ddcafbc3e21405eadc0f65192e24231871b49f9ff88d4c6345d388cd82a99`。
- Dependencies：performance-evaluation blob `b0c0e345f8c47998189a7ad2d21e93c2bc7d2694`、architecture blob `5ddf672966cd8aac77e57f0b9270733dd4191497` 已在当前 HEAD 的交付门 VERIFIED；requirements 的 Review 仍未勾选，本轮不替它恢复批准，也不授权进入实现。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`python3 scripts/check-docs.py` exit 0；追加本轮后的完整文档检查亦通过。文档集／结构成功不消除任务边界缺口；未运行产品测试或私有语料测量。
- CEv1：WorkUnit `scan-performance:tasks.md`；ContentState `scan-perf-tasks-r11-20261026-state`；语义失败证据 `scan-perf-tasks-r11-20261026-review`，结构通过证据 `scan-perf-tasks-r11-20261026-integrity`。历史 PASS 继续绑定原状态。
- 结论：一个开放 finding，FAIL；tasks.md Review 保持未勾选，Beads 返回修复。没有 Git 交付或其他任务认领。

修复：scan-performance / reviews/tasks.md / SP-TASKS-R11-F1

## Repair candidate — 2026-10-26（Round 11 后）

Repairer：Claude Code 主会话（本次 E2 增补的设计者）；范围为 SP-TASKS-R11-F1。Round 11 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- SP-TASKS-R11-F1 -> repaired in candidate：
  - **文件边界：** [tasks.md](../tasks.md) Task 3 文件清单后新增 E2 探测的测量工具入口，限定在 Perf 配置与
    `cmd/agentdeck-perf`，不扩大为重写测量工具：`PerfDriver`（探测开关、静态内容、popover 暖启动与面板探测窗口、
    探测所需的驱动步骤）、`PerfFrameProbe`（面板窗口的帧终点与暖启动可见帧/前台/key 变化记录）、`run.go` 三个探测
    条件的 pilot 与正式编排、`stats.go` 起止点与结论分流，以及定向测试 `oracle_test.go`、`stats_test.go`。
  - **验证边界：** 验证段新增 E2 探测项：计划表（pilot 20、正式 100）、无 oracle 的起止统计、候选不可行与工具无效的
    分流回归；Perf 构建通过且 Release 产物不含探测开关与面板探测符号；每个运行的探测条件 pilot 工具无效为 0。
    结束清理写明探测条件、开关与面板代码删除（或暖启动按判定并入产品路径）时定向回归随之移除。
  - 保留 Perf-only、候选不可行与工具无效分流、结束清理约束；决策出口与性能评估、架构一致，未改。

修复目标：HEAD `5e8c98cbcfefc6366fb5b1fa579dcee119d8276e`；tasks blob `87650934896ee170a6991ec1ac643f5eb6d5d755`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`python3 scripts/check-docs.py` exit 0；`git diff --check` exit 0。

#### 下一步指令

复评：scan-performance / tasks.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 12 — 2026-10-26

## 📋 E2 追加探测任务边界复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-TASKS-R11-F1 closed：Task 3 明确纳入 PerfDriver、PerfFrameProbe、run.go、stats.go 及 oracle_test.go／stats_test.go，限定为 E2 探测所需修改，没有把新工作隐含交给已完成的 measurement-harness。
- 验证覆盖三个条件的 pilot 20／正式 100 编排、无 oracle 统计、工具无效与候选不可行分流、Perf 构建与 Release 隔离，以及清理后的探测测试处置。与已评性能契约、架构一致。
- 新探测仍不授权 UX／目标变更；无有效测量时停止，原产品正确性与性能验收保留。

### 📝 总结

- Reviewer：Codex 主会话，Round 11 评审者；Method：对照 Claude Code 修复交接逐项核对文件／验证边界和依赖契约；复用上一轮已核实且未变的条件入口源码事实，无委派、无产品修改。
- Scope：SP-TASKS-R11-F1 及修复影响；唯一开放 finding 关闭，无新增或回归发现。历史已关闭发现维持原处置。此为分解文档 PASS，不是实现通过或整套设计全部完成。
- Reviewed state：HEAD `5e8c98cbcfefc6366fb5b1fa579dcee119d8276e`；修复输入 blob `87650934896ee170a6991ec1ac643f5eb6d5d755`；仅同步本文件 Review 勾选后，最终 blob `070b1709af301ac4c73835d8396a7b748bcbb0df`；SHA256(`head=<HEAD>;document=<blob>`) = `befc42cf91d4576d45653496080e06991209c2b6ae7c4b63049510bf96fa2436`。
- Dependencies：performance-evaluation blob `b0c0e345f8c47998189a7ad2d21e93c2bc7d2694`、architecture blob `5ddf672966cd8aac77e57f0b9270733dd4191497` 及上一轮依赖核对结果未变。requirements Review 未勾选，Beads `ad-scan-performance-doc-req` 仍 in_review；本轮不恢复其批准，不绕过 tasks.md 所声明的全部文档通过前提。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；最终 `python3 scripts/check-docs.py` exit 0，涵盖空白、diff、文档集、链接与评审结构。不重复运行产品测试或私有语料实验。
- CEv1：WorkUnit `scan-performance:tasks.md`；ContentState `scan-perf-tasks-r12-20261026-state`；当前目标的 `decomposition-and-dependency-coverage`、`document-integrity` 均通过。旧 Round 11 FAIL 保留原内容绑定；未更改其观察。
- 结论：任务矩阵复评 PASS，Review 勾选，Beads 转 awaiting_commit；未提交／推送。topic 仍有需求评审和实现任务，不跨越 topic 完成门。

Task checkpoint：ad-scan-performance-doc-tasks；content_state `befc42cf91d4576d45653496080e06991209c2b6ae7c4b63049510bf96fa2436`；gate VERIFIED。
提交建议：经用户授权提交 tasks.md 的 E2 分解修复、对应 Review 勾选与本记录，保留其他产品改动。
推送建议：经独立授权推送 origin/feature/scan-performance；先核验实际提交范围、贡献归属、trailer 与 SSH 签名，远端当前状态未核验。

评审：scan-performance / requirements.md

## Round 13 — 2026-10-27

## 📋 启动争用对照与削减任务边界评审

📊 总体评分：7/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

[tasks.md:151–154] SP-TASKS-R13-F1（中）：Task 3 承接架构“启动争用削减”候选 1，但文件边界未覆盖消除无效写入所需的写入路径，且这些路径属于后续任务的文件范围。

- 行为风险：架构候选 1 的第一类处理是“无事实变化的写入把派生 generation 置 dirty 或推进 revision；在不改 schema 与 revision 触发器的前提下消除这类无效写入”。触发器挂在事实表上，消除只能改发出写入的代码。Task 3 只列出 `internal/desktop` 的 `derived_cache.go`、`serving_cache.go` 与 `internal/scanruntime` 发布段／worker 优先级。实现者确认原因属于这一类时，必须越过列明边界去改 Task 4（`internal/ingest`、`internal/usage`、`internal/session`、`internal/store` 写事务封装）或 Task 5（`internal/store` dirty/revision）拥有的文件；依赖顺序 3 → 4、5 意味着 Task 3 先改动后续任务的文件而无约定，或只能放弃一条架构已批准的处理。
- 证据：`internal/store/migrations.go:431–446` 对 `derivedSnapshotGenerationTables` 的每张表安装 INSERT/UPDATE/DELETE 触发器，执行 `revision=revision+1,dirty=1`；`internal/store/usage_partitions.go:199–206` 复用同一触发器族。无变化扫描中的写入来自 `internal/scanruntime/serving.go` derive 前的 `service.FinalizeInventory`（`internal/usage`）、`session.ScanWithOptions`（`internal/session`）与 `finishAcceptedInventory`（store）。Task 4 文件见 tasks.md:193–194，Task 5 见 215–216。
- 💡 有界修复：在 Task 3 文件边界中以“必要时”形式纳入候选 1 第一类处理所需的写入方（限定为消除无事实变化的写入，不改 schema、触发器与写事务结构），并写明与 Task 4／5 的边界：Task 3 只做无效写入消除，并行 reduce、短事务与分区重算仍归 4／5；或者明确把第一类处理移交 Task 4／5 并从 Task 3 范围删去。二选一，与架构候选 1 的表述保持一致。

[tasks.md:155–159] SP-TASKS-R13-F2（低）：quiet 对照依赖新打点 `refresh.startup.done`，Task 3 的 E2 测量工具入口未列出打点注册文件。

- 行为风险：架构与性能评估要求 App 在 coordinator 的启动全量刷新返回时打点 `refresh.startup.done`（Perf 构建专用），`P01-floor-warm-quiet` 以它为等待起点。打点名在 `PerfTrace.swift` 的枚举中注册，该文件是 Task 1 的交付物，不在 Task 3 列表中；`DesktopRefreshCoordinator` 位于已列出的 `EmbeddedHelperRunner.swift`，但注册点缺失使边界不完整。
- 证据：`apps/macos/AgentDeckShared/PerfTrace.swift`（HEAD）第 7–36 行以 `case xxx = "<name>"` 枚举全部打点，无 `refresh.startup.done`；`DesktopRefreshCoordinator` 定义于 `EmbeddedHelperRunner.swift:1888`；architecture.md:136–137、performance-evaluation.md:420 规定该打点。
- 💡 有界修复：在 E2 测量工具入口中加入 `PerfTrace`（仅新增 `refresh.startup.done` 打点）及其定向测试 `PerfTraceTests`，保留 Perf-only 与结束清理约束。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- Task 3 范围准确承接性能评估“追加探测结果与争用对照”的出口：quiet 对照只用于归因，争用主导才逐个实施候选，不改窗口形态与目标；与 performance-evaluation.md:420–457 一致。
- 文件边界对候选 1 的描述与架构 Round 9 已评口径一致（经已有派生快照缓存复用、不新增复用键、不跳过 `Build`）；候选 2 的 `EmbeddedHelperRunner` QoS 与 scanruntime worker 优先级已列入。
- 正确性回归补入同一注入时钟下的逐字节等价与各时间边界重算；验证段补入 quiet 条件 helper 存活即工具无效、ABBA 交错，与性能评估契约一致。
- 历史 SP-TASKS-R11-F1 等已关闭发现未回归。

### 📝 总结

- Reviewer：Claude Code 新会话（冷上下文；非本次 tasks.md 增补的设计或修复会话）；Method：独立增量任务分解评审，对照已评 architecture（Round 9）与 performance-evaluation（Round 16）核对文件边界、验证与跨任务所有权，并核实触发器、写入方与打点注册源码；无委派，无产品修改。
- Scope：自 HEAD 起 tasks.md 的增补（矩阵、Task 3 文件边界、范围、正确性回归与 E2 验证）。发现边界缺口后未扩大到其他任务全面审计。
- Reviewed state：HEAD `615d124453b99c6cb504d534f04fb4a019a4a741`；document blob `cd605929316983c557f02041be66182badae812d`；SHA256(`head=<HEAD>;document=<blob>`) = `27b9f90f66c8de7602df757a24366d944bab44b73a40487dccfa9b94afeac89f`。
- Dependencies：architecture blob `b960650b007556750d6389681035173da110a182`（Round 9 PASS，已交付于 615d124）；performance-evaluation blob `7a17d7fa58017793a01c090145ec20051a70906a`（Round 16 PASS）。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。工作区内另有未提交的 Swift／`cmd/agentdeck-perf` 探测实现改动，不属于本评审对象。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0（文档集检查）；`python3 scripts/check-docs.py` exit 0；`git diff --check` exit 0。结构检查通过不消除边界缺口。
- CEv1：WorkUnit `scan-performance:tasks.md`；ContentState `scan-perf-tasks-r13-20261027-state`；`scan-perf-tasks-r13-20261027-integrity` pass、`scan-perf-tasks-r13-20261027-review` fail。
- 结论：两个开放 finding，FAIL；tasks.md Review 保持未勾选，Beads `ad-scan-performance-doc-tasks` 由 closed 重开评审后退回 in_progress。无 Git 交付。

修复：scan-performance / reviews/tasks.md / SP-TASKS-R13-F1、SP-TASKS-R13-F2

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-27（Round 13 后）

Repairer：Claude Code 主会话；范围为 SP-TASKS-R13-F1、SP-TASKS-R13-F2。Round 13 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- SP-TASKS-R13-F1 -> repaired in candidate（选择“必要时纳入 Task 3”，与架构候选 1 保持一致）：
  - Task 3 文件边界新增一段：实现第一步确认重算原因属于候选 1 第一类（无事实变化的写入把派生 generation 置 dirty
    或推进 revision）时，必要时修改发出这些写入的调用方：`internal/usage` 的 `FinalizeInventory`、`internal/session` 的
    `ScanWithOptions`、`internal/scanruntime` 的 `finishAcceptedInventory` 及其调用的 `internal/store` inventory 完成写入。
    函数位置已只读核对（`usage.go:774`、`session.go:228`、`scanruntime/serving.go:48`）。
  - 只限于让无事实变化时不发出写入；不改 schema、revision 触发器与写事务结构。
  - 写明与后续任务的边界：并行 reduce、短事务切分与写事务封装仍归 Task 4，dirty/revision 机制与分区重算仍归 Task 5；
    若消除需要改动这些机制，停止并返回设计。
- SP-TASKS-R13-F2 -> repaired in candidate：
  - E2 测量工具入口加入 `PerfTrace`，只新增探测所需的 `popover.warm` 与 `refresh.startup.done` 打点；定向测试加入
    `PerfTraceTests`（`apps/macos/AgentDeckTests/PerfTraceTests.swift`）。
  - 结束清理约束同步为“探测条件、开关、探测打点与面板探测代码在任务结束前删除，或判定为采用的暖启动并入产品路径”。
- 目标、范围、准则与样本数未改。

修复目标：HEAD `615d124453b99c6cb504d534f04fb4a019a4a741`；tasks blob `995e6aeb403595abc6cc3480f1e1e45448a590d3`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / tasks.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 14 — 2026-10-27

## 📋 启动争用削减任务边界复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED（CEv1 scan-perf-tasks-r14-20261027-state，两项 required criteria 均通过）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-TASKS-R13-F1 -> closed。Task 3 选择“必要时纳入”，与架构候选 1 一致：确认属于第一类时可改发出写入的调用方，
  且只限于无事实变化时不写；不改 schema、触发器与写事务结构；并行 reduce、短事务与写事务封装归 Task 4，dirty/revision
  与分区重算归 Task 5，需要改这些机制即停止返回设计。所列调用方经源码核对：`internal/usage/usage.go:774`
  `FinalizeInventory`、`internal/session/session.go:228` `ScanWithOptions`、`internal/scanruntime/serving.go:48`
  `finishAcceptedInventory`（调用 `store.FinishInventory`），正是 Round 13 指出的无变化扫描写入来源。
- SP-TASKS-R13-F2 -> closed。E2 工具入口加入 `PerfTrace`（只新增 `popover.warm` 与 `refresh.startup.done`）及
  `PerfTraceTests`；`refresh.startup.done` 与 architecture.md、performance-evaluation.md:420 一致，`popover.warm` 已存在于
  工作区未提交的探测实现 `PerfTrace.swift:52`，列入边界与实际需要相符。结束清理同步覆盖探测打点。
- 修复未改目标、范围、准则与样本数；Round 13 认可的承接与回归项保持不变。

### 📝 总结

- Reviewer：Claude Code 会话，Round 13 评审者（非修复会话）；Method：逐条核对两项 finding 的有界修复与源码，复用
  Round 13 已核实的触发器与依赖契约事实；无委派，无产品修改。
- 处置：SP-TASKS-R13-F1、SP-TASKS-R13-F2 closed；历史发现保持关闭，无新增发现。
- Reviewed state：HEAD `615d124453b99c6cb504d534f04fb4a019a4a741`；修复输入 blob `995e6aeb403595abc6cc3480f1e1e45448a590d3`；
  仅同步本文件 Review 勾选后最终 blob `ddd58cc56bf500b454b94eb20b176e0cfff9ea9c`；SHA256(`head=<HEAD>;document=<blob>`) =
  `c8c5908529e14f4cc3c9db77e6f972dcc836d0576b27efb5556f9ca1832b9ad5`。
- Dependencies：architecture blob `b960650b007556750d6389681035173da110a182`（Round 9 PASS，已交付于 615d124）；
  performance-evaluation blob `7a17d7fa58017793a01c090145ec20051a70906a`（Round 16 PASS）。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`python3 scripts/check-docs.py` exit 0（含本轮记录）；
  `git diff --check` exit 0。
- CEv1：WorkUnit `scan-performance:tasks.md`；目标 `scan-perf-tasks-r14-20261027-state`；`scan-perf-tasks-r14-20261027-integrity`、
  `scan-perf-tasks-r14-20261027-review` 均为 pass；关系预检 4/4 ok；精确目标门禁 VERIFIED。Round 13 的 fail 观察保留原绑定。
- 结论：PASS；tasks.md Review 勾选，Beads `ad-scan-performance-doc-tasks` 转 awaiting_commit；未提交／推送。topic 仍有实现任务，
  不跨越 topic 完成边界。

Task checkpoint：ad-scan-performance-doc-tasks；content_state `c8c5908529e14f4cc3c9db77e6f972dcc836d0576b27efb5556f9ca1832b9ad5`；gate VERIFIED。
提交建议：经用户授权后，提交 tasks.md 的启动争用对照／削减分解与 Review 勾选、reviews/tasks.md Round 13–14 及修复记录；不混入工作区的 Swift／`cmd/agentdeck-perf` 探测实现改动。
推送建议：提交后，经提交范围、贡献归属、trailer、SSH 签名核验和独立授权，推送 origin/feature/scan-performance；远端当前状态未核验。

## Round 15 — 2026-10-27

## 📋 无数值门槛任务覆盖评审

📊 总体评分：7/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

- **SP-TASKS-R15-F1（高，OPEN）** — `tasks.md:98–104, 153–176, 257–269`：Task 1 仍要求 `stats`/`gate` 按 `k` 判定通过/不确定/失败，与本轮需求取消 `k` 的契约直接相反；Task 1 已标完成，剩余任务只安排删除探测工具代码，没有任何任务承接“无 k、仅输出两臂统计量/比值/区间”的报告工具迁移。
  - 行为风险：按现有分解完成所有优化后，E7 要么仍须输入已失效的 k 并产生误导 verdict，要么临时扩大任务范围改工具；文档所说“测量契约不变”掩盖了真实接口变更。
  - 证据：当前 `cmd/agentdeck-perf/gate.go:264–300` 的 `runGate` 强制解析正数 `--k`，缺省即报错，并只取 p95（P09 取 median）；`stats.go:447–685` 为单条件/单臂汇总，不提供文档 243–248 行要求的通用双臂 median/p95/p99 区间报告。任务 3 工具范围只列探测移除，任务 8 只列最终批次及文档收尾。
  - 💡 有界修复：给既有适当任务明确报告工具/测试文件边界与验收（无 k 可报告、无性能 pass/fail、规定统计量/区间、无效 ABBA 拒绝）；把 Task 1 已完成的旧 gate 契约标为历史并说明迁移承接。不要直接重写历史 Task 1 验证事实或在本轮修改产品。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

保留调用方写入优化与 dirty/revision/短事务任务之间的边界；探测删除范围、暖启动正确性、正式对比与最终用户判断均有明确落点。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话；Method：独立分解增量评审、上游合同与现有 CLI 接口核对；无委派。
- Scope：Documents 完整集合、Task 1–8 对无数值门报告的覆盖、探测删除、验证边界；SP-TASKS-R13-F1/F2 沿用 Round 14 关闭处置，无回归；上游统计单元与架构归因发现分别由其当前记录承接。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；输入 blob `12df8fa00c741d821026d9a789fba5b29e4be537`。本轮只勾选 requirements Review；最终 blob `8a8dfe3465fa82c6ee537d147fd60057cae98964`，SHA256(`head=<HEAD>;document=<blob>`) = `74d14f10884313a33c329b3fefab5494fc81c3563b3e8c53093895b3a1899c75`。
- Dependencies：本轮 requirements、performance-evaluation、architecture 候选；UX 本轮未改变表面及标本要求，保留既有 Review。需求 PASS 不覆盖另两份上游文档的 FAIL。
- Evidence：`python3 scripts/check-docs.py` exit 0（含文档集检查）；CodeGraph 返回 `runGate` 与 `runStats` 当前源码。未运行产品测试或测量，工具迁移仍属将来的实现任务。
- CEv1：WorkUnit `scan-performance:tasks.md`；目标 `scan-perf-docbatch-20261027-tasks-state`。最终门禁见本轮单一完成门禁字段。
- 结论：FAIL；自身及 performance/architecture Review 保持未勾选。工作区 `agent-deck.scan-performance` / `feature/scan-performance`；无 Git 交付。

下一步指令：修复：scan-performance / reviews/tasks.md / SP-TASKS-R15-F1

## Repair candidate — 2026-10-27（Round 15 后）

Repairer：Claude Code 主会话；范围为 SP-TASKS-R15-F1。Round 15 的结论与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS。未委派。

- SP-TASKS-R15-F1 -> repaired in candidate：
  - Task 1 中“按 `k` 判定通过、不确定或未通过”标为 2026-10-27 之前的历史契约，不改写其验证事实；并注明迁移由
    `serving-open-path` 承接。
  - Task 3 新增“对比报告工具迁移”，因为本任务的对比批次最先需要它，后续任务复用。文件边界：`cmd/agentdeck-perf` 的
    `gate.go` 改为不带 `--k` 的 `compare` 子命令，`run.go` 的两臂交错覆盖各条件，`stats.go` 提供两臂汇总；定向测试为
    `gate_test.go`（改名 `compare_test.go`）与 `stats_test.go`。现状已只读核对：`runGate` 强制 `--k`，`stats.go` 为单臂汇总。
  - 验收：
    - 不接受、也不需要 `k`；输出不含任何性能判定。
    - ABBA 条件输出两臂 median、p95、p99、比值与固定种子的 bootstrap 区间；组不完整时拒绝出区间并报工具无效。
    - P06 输出逐对差值；P07 与 P11 完整历史输出两臂并列；P08 输出计数并列；P05 只输出单臂统计。
    - 以上与 performance-evaluation“对比报告”方法表一致。
- 另按 SP-ARCH-R10-F1 同步第 3 项范围：重复打开先以分段与身份观察确认存在重复构建，再实施“未变时不重算、不重建”。

修复目标：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；tasks blob `551b78a9dffbe247ce76dfa9f6d4ec71a4703c07`（含清除 requirements Review 勾选）。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / tasks.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 16 — 2026-10-27

## 📋 报告工具迁移承接与最终验收复评

📊 总体评分：8/10

✅ 复评结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

无高严重度发现。

### 🟡 建议改进 — 推荐

- **SP-TASKS-R16-F1（中，new，必须修复）** — `tasks.md:183–184, 267–270`：Task 3 仍将 P11 整项要求为 ABBA，Task 8 的明确目标仍是“P01–P11 全矩阵……ABBA 对比”，与本轮 163–168 行新增的五类工具验收冲突。
  - 行为风险：按 Task 3 工具合同实现后，Task 8 仍会要求对只有 3/5 单元的条件及无基线的 P05 生成 ABBA 对比；最终验收与工具验收给出不同完成条件。
  - 证据：上游 performance-evaluation:250–254、requirements:205–207 明确只在指定偶数条件使用 ABBA，P05 只报绝对值；Task 8 明确写全矩阵，并非单纯简称交错顺序。
  - 💡 有界修复：Task 3 与 Task 8 的批次/目标直接引用分类方法，限定 ABBA 仅用于适用条件；P11 首批与完整历史分列，P05 保持单臂；保留全部测量项与原正式样本要求。

### 🟢 优点

SP-TASKS-R15-F1 closed：Task 1 标注旧 k 契约为历史，Task 3 承接 compare、run、stats 与定向测试，明确无 k、无性能 verdict 及各类输出；归因修复也已同步。报告工具迁移已有负责人任务及可验证边界。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话，非修复作者；Method：独立定向复评、工具验收与最终任务目标一致性核对，无委派。
- 处置：SP-TASKS-R15-F1 关闭；新增 SP-TASKS-R16-F1。SP-TASKS-R13-F1/F2 既有关闭处置不变。P07 样本总数问题归 performance-evaluation Round 18，不重复建发现。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；输入 blob `551b78a9dffbe247ce76dfa9f6d4ec71a4703c07`；仅同步 requirements/architecture Review 勾选后的最终 blob `cd812585326c93124f6711a4055276889bd7af93`；SHA256(`head=<HEAD>;document=<blob>`) = `69f83256d16bf00e6185e98da7bf5d9742b8d8a44e36a99e32c20431903d2a0c`。
- CEv1：WorkUnit `scan-performance:tasks.md`；target `scan-perf-docrecheck-20261027-tasks-state`；精确 gate FAILED：`tasks-review` 为 fail，`tasks-integrity` 为 pass，均使用该前缀；无 unresolved impact。返回的四项旧 corpus evidence 已失效且不匹配本目标，不用于本轮通过声明。`python3 scripts/check-docs.py`（含文档集）与 `git diff --check` exit 0；不运行产品测试或性能批次。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`；correlation `QtgKmOTPe89-k69i`。仅需求/架构 Review 勾选，性能评估/任务 Review 保持未勾选；Topic 边界仍未到达。

下一步指令：修复：scan-performance / reviews/tasks.md / SP-TASKS-R16-F1

## Repair candidate — 2026-10-27（Round 16 后）

Repairer：Claude Code 主会话；范围为 SP-TASKS-R16-F1。Round 16 的结论与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS。未委派。

- SP-TASKS-R16-F1 -> repaired in candidate：
  - Task 3 对比批次限定为 P01 三层、P02、P03、P04、P11 首批，按运行单元配对交错，引用性能评估“对比报告”分类（这些均为
    ABBA 比值与区间）；P11 完整历史不在本任务对比，只在 Task 8 按“两臂并列”报告。
  - Task 8 目标改为按分类方法报告全部测量项：ABBA 比值与区间（P01 三层、P02、P03、P04、P09、P10、P11 首批）、P06 配对差值、
    P07 与 P11 完整历史两臂并列、P08 计数并列、P05 只报单臂绝对值。
  - 全部测量项与正式样本要求保留。

修复目标：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；tasks blob `15cc4b49ed36bfdf39b3b0074dda0a1dec82915d`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / tasks.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 17 — 2026-10-27

## 📋 分类报告承接与 P07 任务引用复评

📊 总体评分：8/10

✅ 复评结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

- **SP-TASKS-R17-F1（中，new，必须修复）** — `tasks.md:207`：Task 4 `parallel-source-pipeline` 的对比批次写为“P07（3 次全量重建）”，未包含 100 次增量；而本任务目标（192 行）要报告 P07 的写事务对比，performance-evaluation 现行合同（blob `9411045…`，252、297 行）把 P07 正式样本定为“3 次全量重建 + 100 次增量（只在第 3 个运行单元重建之后）”，事务 p95/max 取该臂全部 `tx.*`。
  - 行为风险：按任务文字只跑 3 次重建时，P07 事务统计缺失 100 次增量，事务权重与 p95 口径和 S0 基线（Task 2 按正式样本数）不同，E3 的写事务对比不可比；这正是 SP-PE-R18-F1 要排除的口径分歧，其有界修复明确要求“统一两表及任务引用”，本处任务引用未同步。
  - 证据：207 行字面；performance-evaluation 252、297 行；`cmd/agentdeck-perf/run.go:971–978` 的 P07 formal 计划 `schedule{Units: 3, Inner: 100}`，增量在 `index == s.Units` 时执行。
  - 💡 有界修复：207 行 P07 改为引用性能评估正式样本（3 次全量重建 + 100 次增量，事务取全部），或直接写“按契约样本数”；不改其他条件、样本数或任务范围。

### 🟢 优点

- SP-TASKS-R16-F1 closed：Task 3（183–185 行）对比批次限定为 P01 三层、P02、P03、P04、P11 首批，并明确均属 ABBA 比值与区间；P11 完整历史移交 Task 8。Task 8（268–270 行）按五类方法逐项列出：ABBA（P01 三层、P02、P03、P04、P09、P10、P11 首批）、P06 配对差值、P07 与 P11 完整历史两臂并列、P08 计数并列、P05 单臂绝对值，与 performance-evaluation 250–254 行分类完全一致；Task 3 工具验收（166–168 行）与之对应。
- SP-TASKS-R13-F1/F2、R15-F1 的既有关闭处置无回归；Task 5 P10（100 次）、Task 6 P05/P07 RSS 引用与性能评估一致。

### 📝 总结

- Reviewer：Claude Code 主会话（Opus 5.5），本文档 Round 16 后 Repair candidate 的作者为同一客户端的另一会话；本轮为定向复评，未修改 tasks.md 正文。Method：逐项核对 Task 3/4/5/6/8 的批次与目标对性能评估分类表、正式样本表及 `run.go` 编排；无委派。
- 处置：SP-TASKS-R16-F1 closed；新增 SP-TASKS-R17-F1（中）。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；tasks blob `b6718788f5535693b35e279213fed69a126d438e`（Repair candidate blob `15cc4b4…` 之后仅 Documents 表中 performance-evaluation.md Review 勾选因其 Round 19 PASS 而恢复）；SHA256(`head=<HEAD>;document=<blob>`) = `5f55ff99c18673464051f3b071c4159122d79c49dfd8a56df8f61de2e075f8eb`。
- Evidence：`python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh scan-performance`、`git diff --check` exit 0；不运行产品测试或性能批次。
- CEv1：WorkUnit `scan-performance:tasks.md`；ContentState `scan-perf-tasks-r17-20261027-state`；`scan-perf-tasks-r17-20261027-review` fail、`scan-perf-tasks-r17-20261027-integrity` pass；精确门禁 FAILED。
- 结论：FAIL，tasks.md Review 保持未勾选。工作区 `agent-deck.scan-performance` / `feature/scan-performance`；未提交或推送。

下一步指令：修复：scan-performance / reviews/tasks.md / SP-TASKS-R17-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-27（Round 17 后）

Repairer：Claude Code 主会话；范围为 SP-TASKS-R17-F1。Round 17 的结论与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS。未委派。

- SP-TASKS-R17-F1 -> repaired in candidate：Task 4 对比批次中的 P07 改为引用性能评估正式样本——3 次全量重建 + 100 次增量，
  增量只在第 3 个运行单元的重建之后，事务取该臂全部 `tx.*`——与 performance-evaluation 现行合同及
  `cmd/agentdeck-perf/run.go` P07 计划一致；其他条件、样本数与任务范围未改。

修复目标：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；tasks blob `048d84ec38579e4103875e38920c2e15ac5f3202`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / tasks.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 18 — 2026-10-27

## 📋 Task 4 P07 正式样本引用修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-TASKS-R17-F1 closed：`tasks.md:207` Task 4 对比批次的 P07 改为“按契约正式样本：3 次全量重建 + 100 次增量，增量只在第 3 个运行单元的重建之后，事务取该臂全部 `tx.*`”，与 performance-evaluation 252、297 行及 `cmd/agentdeck-perf/run.go:971–978` 的 formal 计划一致；E3 写事务对比与 S0 基线口径相同。其他条件（P08、P09 20 对、P01 重建层复测）未变。
- SP-TASKS-R16-F1 保持关闭：Task 3（183–185 行）与 Task 8（268–270 行）的五类分类引用未变；R13-F1/F2、R15-F1 无回归。其余 P07 引用（136、139、167、192、251、261 行）均与合同相容。

### 📝 总结

- Reviewer：Claude Code 主会话（Opus 5.5）；Method：定向复评，核对修复行与性能评估合同、工具编排及全文 P07 引用；无委派，未修改 tasks.md 正文。
- 处置：SP-TASKS-R17-F1 closed；无仍开放、回归或新增发现。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；评审 blob `048d84ec38579e4103875e38920c2e15ac5f3202`（与 Repair candidate 目标一致）；本轮恢复 Documents 表中 tasks.md 的 Review 勾选后最终 blob `aeb7c0d0d3e18325cb93929d4c105de7cbfaae4c`；SHA256(`head=<HEAD>;document=<最终 blob>`) = `d869c465bb29195a561d968256188fe5468c2f5e024272b263213018f2a756a2`。
- Evidence：`python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh scan-performance`、`git diff --check` exit 0；不运行产品测试或性能批次。
- CEv1：WorkUnit `scan-performance:tasks.md`；ContentState `scan-perf-tasks-r18-20261027-state`；`scan-perf-tasks-r18-20261027-review` 与 `-integrity` 两项 pass，关系 4/4；精确门禁 VERIFIED。Round 17 fail 仍绑定旧内容。
- 结论：PASS。dispatch `ad-scan-performance-doc-tasks`。四份重设计文档均已通过；实现任务 `serving-open-path` 已由 claude-code 认领。工作区 `agent-deck.scan-performance` / `feature/scan-performance`；未提交或推送。
- 跨记录与简写引用的处置（本记录内无其他未闭环发现）：
  - SP-ARCH-R10-F1 closed（architecture 记录 Round 11，复核于本轮）
  - SP-PE-R18-F1 closed（performance-evaluation 记录 Round 19）
  - R13-F1 closed（即 SP-TASKS-R13-F1 的简写）
  - R15-F1 closed（即 SP-TASKS-R15-F1 的简写）

Task checkpoint：ad-scan-performance-doc-tasks；content_state `d869c465bb29195a561d968256188fe5468c2f5e024272b263213018f2a756a2`；gate VERIFIED。
提交建议：另行授权提交 tasks.md 的无数值门槛重设计与 R15–R17 修复、Round 15–18 及修复记录、Documents 勾选。
推送建议：核验实际提交范围、签名与远端状态后，另行授权推送 origin/feature/scan-performance；远端当前状态未核验。

下一步指令：开发：scan-performance / serving-open-path

WORKFLOW_WORKSPACE: agent-deck.scan-performance
