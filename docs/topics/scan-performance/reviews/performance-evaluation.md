---
status: active
topic: scan-performance
subject: performance-evaluation.md
---

## Round 1 — 2026-10-09

## 📋 性能评估方法与结论评审

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

- 撤回浏览器首帧加速比，明确两次 native UI 尝试无有效样本。
- 三种已存数据前置状态、12 对原生模型 A/B、3 对首采 pilot 与 2 对待入库 pilot 分开归因。
- 主动保留原位 15.545s、usage 12.045s、621.5MiB 负结果，不宣称实时或资源门已通过。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra；用户指定 actor `opencode`。
- Method: 独立主会话审查全部方法/决策门，核对三个脱敏结果 JSON、manifest 与 seven harness 源码的相关路径，重算摘要；不重新访问真实日志、数据库或重跑性能实验。
- Scope: 测量归因、摘要算术、适用边界及 E0–E4 计划。需求 P01–P11 是未来产品验收，并非本文件 PASS 的测量成果。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `c04dd0e71ba71e40b06eb1582306b93a04bf40bc`。
- ContentState: `scan-performance:state:01c36ea5f4d27c854c920d623a16e719686450440e619ded0a69b0fad0cad96c`；SHA256(`head=<HEAD>;document=<blob>`)。
- Supporting specimen/result identity: manifest SHA256 `739b67d01e67f7a5427946e41a80c2302ab64f9d8149901c9cef6ff109a428fe`，43 个列入文件逐项 SHA256 全匹配。此处记录依赖身份，不更改既定 document digest recipe。
- Evidence: `bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace` 退出 0；原生 A/B 摘要为 1919.1852805ms / 15.0885635ms；首采为 75437.596834ms / 1553.772468ms；与文中四舍五入一致。firstload harness 保留整文件解析与交错顺序；history harness 从 SQLite backup 分组；native harness `testNativeDataPreparationAB` 仅计 coordinator/decoder/model，未计 popover。
- Dependencies: requirements 的验收提案与本文件一致；tasks 对 E1 的入口/出口使用问题由 `reviews/tasks.md` 承接，不是实验结果缺陷。
- Limitations: 原始私有输入已按既有交接清理，本轮不能独立复现历史性能观察；只批准可复查摘要、方法及诚实限制，不认证真实 native frame、尾延迟、OTel 或资源预算。
- Completion gate: VERIFIED
- Gate evidence: Neo4j MCP 标准 gate-status 查询在上述精确 ContentState 上返回两项 required criteria 均通过，missing/invalidated/unresolved 均为空。
- 产品实现、安装、发布与整套设计批准仍未完成。

### Task checkpoint：ad-scan-performance-doc-perf

文档 blob `c04dd0e71ba71e40b06eb1582306b93a04bf40bc`，门禁 VERIFIED，待授权交付。

提交建议：性能评估文档、其引用的已审摘要/实验 harness 及本轮记录；共享标本依赖需保持本轮身份，可等待修复后的完整设计批次。

推送建议：仅考虑 `feature/scan-performance`；远端目标未核验，须先完成授权提交与依赖范围核验，再另行授权推送。

## Round 2 — 2026-10-09

## 📋 性能评估结论复用与依赖影响复核

📊 总体评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进问题 — 必须闭环

无。

### 🟢 优点

本记录原无开放 finding。Task 2 已正确区分 pilot 输入与 E1 产品出口，未改变历史实验或将其升级为产品验收。

### 📝 总结

- Reviewer: OpenCode / GPT-6 Astra，actor `opencode`；Method: 原内容身份/测量依赖核对与结论复用，未重复真实规模或 native 实验。
- Scope: 全设计复评中的测量结论适用性；无新增、未闭环或回归问题。
- Reviewed state: HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`；blob `c04dd0e71ba71e40b06eb1582306b93a04bf40bc`，与 Round 1 完全一致。
- ContentState: `scan-performance:state:01c36ea5f4d27c854c920d623a16e719686450440e619ded0a69b0fad0cad96c`。
- Evidence reuse: 三份性能结果及七份原实验 harness 的哈希仍与 Round 1 manifest 对应值一致。新 manifest `2ab9be34d6e69586182d905a8aba17a88533d69a635d6895aede220acc987ea7` 变更的是合成 UX/输出及检查标本，不改变性能测量归因；复用原两项 required criteria 的证据与限制。
- Completion gate: VERIFIED
- 原始私有输入未留存、native first-frame 未通过、实时/资源负结果仍保留；设计 PASS 不改变这些限制。

Task checkpoint：ad-scan-performance-doc-perf；content_state `01c36ea5f4d27c854c920d623a16e719686450440e619ded0a69b0fad0cad96c`；gate VERIFIED。

提交建议：评估文档、已审结果/harness 与评审记录，可随完整设计批次授权交付。

推送建议：feature/scan-performance；远端未核验，先完成授权提交，再另行授权推送。

## Round 3 — 2026-10-18

## 📋 E1 测量补充独立设计复核

Verdict: PASS

Completion gate: VERIFIED

2026-10-19元数据补记：本字段显式表达下文已记录的当轮VERIFIED及其原ContentState，未重判历史内容或改绑到当前版本。

- Reviewer: Codex；方法为补充差异、真实采集器/汇总器与既有修复证据的联合核对，无产品修改、无委派。
- Scope: 人工可见性入口、六条件调度、独立启动时钟、原位缓存校验、真实压力准入及 R7 首帧归属；沿用 Round 2 未变的历史实验结论和限制。
- 模型时钟不当作帧；AppKit 数值 bitmap 不当作 compositor/photon；operator wait 不当作启动；后续面板不合并为首次点击。六条件、A/B、数据库种子、语料和探针身份分别分组，百次不足不输出正式 p95/p99。
- 实际外部 CGEvent 必须单列输入 provenance，不能使用探针原 manual 标签声称物理鼠标测量。AX 屏幕范围及命中 PID 在事件前检查，拒绝事件不能补零或重试伪装成功。
- cache 文件存在不等于 restore；最后一次状态复制后由真实 producer 原位生成，并由真实 reader 核对 publication/session_prepared。schema-ahead 基线失败保留，候选单臂不能给出 A/B 加速比。
- rebuilding 必须有一致请求/worker/round 身份、成功 processing >120s 且 action 在该区间内；排队、CLI wall、sleep 或 pending 标记均不代替它。30s quota delay 是受控依赖延迟，不冒充网络。
- R7 的 action 起点、正常 show 后捕获、OCR 在终点后、准备窗口成本与资源释放均有明确归属；旧分布不倒减或重标。
- 没有新设计 finding。新启动帧 pilot 只证明测量入口可运行，不把一次结果或尚缺的六条件矩阵升级为产品 PASS。内容身份与 CEv1 单独绑定。

本轮最终内容：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；blob `e9b39794d8839a344fc781334d6599ca38e3771d`；digest `79869e104e52e9669517c6b25b80564713f4fbb1ee5d97419ce8261cef7715ca`。
ContentState：`scan-serving-design-supplement-20261018-perf-state-79869e104e52`。标准 Neo4j gate：**VERIFIED**，2/2 required，missing/invalidated/unresolved 均为空。
结构、空白及 diff 检查均通过；新增 6 项文档证据与 12 条关系完成预检、计数及精确回读。
Task checkpoint：设计补充已复核，当前未提交版本仍待单独授权交付；不覆盖整体 Task 2/E1 门禁。

本轮设计 PASS 不认证采集器实现。随后 E1 执行发现统计起点未校验，见 [Task 2 Round 11](serving-snapshot-and-bootstrap.md#round-11--2026-10-18) 的 SP-SB-R11-F1；实现须按已声明的固定/等价起点契约修复，不改变设计门槛。


## Round 4 — 2026-10-19

## 📋 性能评估范围变更独立评审

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

P03工具退役、P04/P11样本例外与需求一致。已核对持久pilot的5次无缓存位图帧899.982896–989.126545ms，以及R18单次模型2066.408569ms/帧2222.636098ms；样本、构建、cohort与模型/位图/真实首击限制未混用。

### 📝 总结

- Reviewer/Method：Codex主会话，独立核对本次范围变更、依赖文档、实现接口及持久证据，无委派、无产品修改。
- Scope：2026-10-19的P03撤销与P04/P11有界接受，以及相应任务状态；此前Round3的未变设计结论保留。历史finding已关闭，无新增或回归finding。
- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；document blob `dc5dbca3e6af2650c042d4893c025056aad36740`；digest `6db5653c982b3b3bb519f8fc87557751e7ab4f6a47451069b60168b24fa2948b`；ContentState `scan-serving-design-r4-2V6mNrA-perf-state`。文档身份按SHA256(`head=<HEAD>;document=<blob>`)；未变原型不重新验收。
- Evidence：本轮check-docs最初仅报告三份历史Round3缺可解析gate；以其已有VERIFIED事实补齐元数据，保留历史正文/身份。最终文档集、链接、review结构、空白及diff检查以修正后的状态执行；不以文档PASS替代产品性能验收。
- CEv1：当前文档两项required准则分别绑定结构检查及本轮语义审查，不覆盖旧观察；门禁以本轮精确文档状态查询。Task2整体门仍未完成。

Task checkpoint：ad-scan-performance-doc-perf；content_state `6db5653c982b3b3bb519f8fc87557751e7ab4f6a47451069b60168b24fa2948b`；gate VERIFIED。
提交建议：经授权提交本次已审performance-evaluation.md变更及必要记录；不将未完成Task2的产品范围混入文档交付。
推送建议：经授权推送origin/feature/scan-performance；前置为核验文档提交范围、贡献归属、trailer与SSH签名。


### Task2 用户批准的验收修订附记 — 2026-10-20

用户明确授权修改必要标准，并接受当前Task2以PASS完结。
本附记记录用户批准的验收豁免，不发布新的独立审查轮次；上述历史审查事实保留。
本次仅将open-path-latency、startup-bootstrap、e1-native-performance三项从Task2
必需完成条件收窄为用户豁免，其余十项仍必需。100次原生首击p95=536.842034ms、
p99=623.638814ms的原预算失败保留。当前验收结论由tasks.md顶端最终状态和
evidence/task2-acceptance-waiver-20261020-receipt.json的精确门禁/本地完成回执管理。
本次用户直接批准的文字调整无需另造独立Review PASS；无产品代码变更。
