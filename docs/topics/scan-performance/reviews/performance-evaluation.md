---
status: active
topic: scan-performance
subject: performance-evaluation.md
---

> **证据引用处置 — 2026-10-22**：Neo4j 恢复后，按用户要求移除本 topic 期间以文件形式保存的
> 回退证据与实验记录（原 `docs/topics/scan-performance/evidence/`，以及 Task 1 的
> `docs/evidence/records/`）。本文原指向这些文件的链接已改为纯文本，文中出现的上述路径均不再是
> 可用引用；原件仅可从提交 `c6ea1ca6551f525cc8a757ccf2e7889fabf36402` 的 Git 历史追溯。
> 结构化观测、内容身份与门禁结果以 Neo4j CEv1 为准；历史观察、结论与评审裁决保持原义。

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

## Round 5 — 2026-10-22

## 📋 全 topic 重新设计：性能评估与测量契约评审

📊 总体评分：6/10

✅ 评审结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

[performance-evaluation.md:69,187] SP-PE-R5-F1（中）：P06 所需的度量量在打点契约中不存在
- 行为风险：P06 要求 30 分钟窗口内的 CPU 时间差、`phys_footprint` 稳定值和唤醒计数差，但“资源”行只在进程退出前自报 `ri_lifetime_max_phys_footprint`、`ru_maxrss` 等终生累计值或峰值。终生峰值包含启动和恢复阶段，不是空闲稳定值；常驻 App 在窗口结束时也不退出。实现者只能自行发明采样点，P06 的门结果因此无法复现。
- 证据：line 69 只定义退出时自报；line 187 使用“稳定值”和“计数差”。
- 💡 有界修复：为 P06 定义窗口起点与终点两次采样（`proc_pid_rusage` 当前值：CPU、`ri_phys_footprint`、wakeups），并写明“稳定 RSS”的统计口径（例如窗口末 N 分钟的采样中位数）和采样频率；同时说明采样本身的唤醒成本不计入预算。

[performance-evaluation.md:165,197,208] SP-PE-R5-F2（中）：pilot 规则自相矛盾，与 tasks.md 也不一致
- 行为风险：line 165 与 E0（line 208）要求“每个条件/每项 pilot 20 样本”；候选采用规则（line 197）是 3 对；tasks.md Task 1 写的是 P09 3 对、P06 1 × 5 min、P05 延至 Task 7。照字面执行，P06 需要 20 × 30 min，E0 判定条件也不确定。
- 证据：上述三行与 tasks.md:104–105。
- 💡 有界修复：在本文给出逐指标 pilot 表（含 P05 归属、P06 时长、P09 对数），E0 引用该表；tasks.md 只引用，不重复。

[performance-evaluation.md:190] SP-PE-R5-F3（中）：P09 正式样本数为“≥20 对”，没有固定
- 行为风险：开放上限允许在看到中间结果后决定何时停止，违反本文“口径冻结”和“正式批次只跑一次”。
- 证据：line 190；tasks.md:119、168 照抄“≥20”。
- 💡 有界修复：写成固定 n（例如 20 对），并说明 n 只能在批次开始前、经设计变更修改。

### 🟡 建议改进 — 推荐

[performance-evaluation.md:65,45–48] SP-PE-R5-F4（低）：`process.start` 不在 `CLOCK_UPTIME_RAW` 上
- 行为风险：`sysctl KERN_PROC_PID` 的 `p_starttime` 是墙钟 `timeval`，不能与 uptime 纳秒“直接相减”；时钟同源测试也只覆盖 Swift、Go 与 NSEvent。
- 证据：line 65 的来源定义；line 45–48 的同源声明。
- 💡 有界修复：定义转换方法（在同一时刻读取墙钟/uptime 偏移，并记录 NTP 调整风险）及其误差测试，或改用其他 uptime 同源的进程起点。

[performance-evaluation.md:69] SP-PE-R5-F5（低）：Go helper 自报 `proc_pid_rusage` 缺少无 CGO 实现路径
- 行为风险：需求非目标是“不默认引入 CGO”；vendored `golang.org/x/sys/unix` 的 darwin 文件里没有 `proc_pid_rusage`（检索 0 处），实现者必须自行选择 syscall 方案或改为父进程代读。
- 证据：`rg proc_pid_rusage vendor/golang.org/x/sys/unix/*darwin*` 0 命中；`ClockGettime` 存在。
- 💡 有界修复：写明 Go 侧取得 `ri_lifetime_max_phys_footprint` 的机制（如 `proc_info` 系统调用、父进程在 reap 前读取等），或声明退化为 `ru_maxrss` 时上界是否仍然成立。

[performance-evaluation.md:116–126] SP-PE-R5-F6（低）：`rep` 夹具的分布参数不足以确定生成算法
- 行为风险：对数正态中位 64 KiB、均值约 644 KB（3.48 GB / 5,400）意味着 σ≈2.15；128 MiB 截断约去掉 8% 的质量，且总量由少数大文件主导，按种子抽样无法稳定落在 ±1% 内。实现者必须自行发明缩放或重抽规则，而该规则会改变夹具形态。`small` 的“同形态缩放”也没有说明大文件下限和最大值。
- 证据：line 118–120 与 line 128 的参数。
- 💡 有界修复：规定满足总量与大文件约束的确定性算法（例如固定大文件清单 + 其余按截断分布抽样后线性缩放），并给出 `small` 的具体数值。

[performance-evaluation.md:217–220] SP-PE-R5-F7（低）：下限探测 p95 >150 ms 被表述为“不可达”
- 行为风险：静态内容下限在 150–200 ms 之间时，P01 并未被证明不可达，文中却据此“判定不可达”并停止实现。这一前提不成立，会把一个设计取舍伪装成事实判断。
- 证据：line 218–219；architecture.md:101–102、tasks.md:136 照抄。
- 💡 有界修复：改成预先声明的决策阈值，并写明 50 ms 余量的依据（数据内容的增量成本）；或把阈值改为 200 ms。三处同步修改。

[performance-evaluation.md:48,105–109] SP-PE-R5-F8（低）：“Task 1”同时指旧、新两个任务
- 行为风险：line 105 的“Task 1 已用…”指旧 `baseline-and-open-path`；line 48、106、109 的 “Task 1” 指新 `measurement-harness`。可行性依据会被误读为新任务的成果。
- 💡 有界修复：一律使用锚点名。

[performance-evaluation.md:182–190] SP-PE-R5-F9（低）：tasks.md 把 B0 用于 P08，但本文没有 B0 的 P08 测量方法
- 行为风险：P08 依赖新的 ingest 计数器，B0 没有这些计数器（只有 `AGENTDECK_SCAN_PROFILE` 的 `ingest.Metrics`）；而且 P08 是绝对门，不需要基线。
- 证据：本文只在 P09 行说明 B0 的测量方式；tasks.md:115 写的是“B0 … 用于 P08、P09”。
- 💡 有界修复：在本文说明 P08 的 B0 用途与取数方式，或从 tasks.md 去掉 B0 对 P08 的用途。

[prototype/README.md:27] SP-PE-R5-F10（低）：本次重写删除了 `#首次加载与已有历史数据的对照`，入链锚点失效
- 证据：`scripts/check-docs.py` 报告 `missing Markdown anchor: ../docs/topics/scan-performance/performance-evaluation.md#首次加载与已有历史数据的对照`。
- 💡 有界修复：改为指向固定提交 `c6ea1ca` 中该节的纯文本说明。

### 🟢 优点

起止点都在产品内，时钟选择与 NSEvent 同源；刺激改为本进程投递事件，不需要系统权限。这一可行性已对照旧 Task 1 的代码（`MenuBarItemController.swift:284,489–493`）和评审记录 SP-R1-F3 核实。“工具无效则整批作废”与“产品失败按 +∞ 计入”把剔除样本的空间完全关闭。P07 的上界求和法去掉了对进程树追踪的依赖。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，冷读并逐条核对源码、vendor 依赖、Git 历史和评审记录；无委派，无产品修改。
- Scope：performance-evaluation.md 全文，以及它与 tasks.md、architecture.md 的交叉一致性。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `717050241cfa26a1aff35d199a11be36c48c75c5`；digest `a04f86e6e710e2b369bf64e50df4ed79121698097ef0d8ca7b47f96caf097db9`。
- Evidence：`scripts/check-docs.py`（报告 F10 及 requirements 的一条锚点）；vendor 检索（F5）；`git show 14234ec:internal/scanruntime/scanruntime.go`（B0 只有 `ingest.Metrics`）；`git diff --check` 通过。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md` 仍绑定旧 ContentState，当前 digest 无记录，门禁 NOT_VERIFIED。
- 结论：3 项中严重度、7 项低严重度问题均未关闭，判 FAIL。契约方向正确，修复集中在口径定义，不需要推翻设计。

## Repair candidate — 2026-10-22

Repairer：Claude Code 主会话；范围仅为 Round 5 的 SP-PE-R5-F1..F10。Round 5 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS。各项均待独立复评确认。

- SP-PE-R5-F1 -> repaired in candidate：新增“P06 空闲窗口”小节：起点 = 首次 `model.usable` 后 120 s 且无 helper 存活
  （7 min 内仍有 helper 存活记为产品失败），时长 1,800 s；CPU 为 App 两端 `proc_pid_rusage` 读数差加窗口内 helper 自报；
  稳定 RSS 为外部 `ps -o rss=` 每 60 s 采样、窗口末 10 min 中位数，并与终点 `ri_resident_size` 交叉核对；
  门用维护激活数（`job.enqueue` + helper 启动，≤30），OS 唤醒差只另报；测量自身成本只在窗口边界。
  打点表新增“资源（窗口）”行，P06 行改为引用该小节。
- SP-PE-R5-F2 -> repaired in candidate：新增逐条件“工具 pilot”表（含 P05 归 `otel-capture-and-reconciliation`、
  P06 1 × 5 min 不作判定、P09 3 对），与候选采用规则的“候选 pilot”明确区分；E0 改为引用该表。
- SP-PE-R5-F3 -> repaired in candidate：P09 正式样本固定为 20 对 ABBA；样本数只能在批次开始前经设计变更修改，
  禁止看到中间结果后提前停止或追加。
- SP-PE-R5-F4 -> repaired in candidate：`process.start` 改取 `rusage_info_v4.ri_proc_start_abstime`（mach absolute time，
  按 `mach_timebase_info` 换算），不再使用墙钟 `p_starttime`；时钟同源测试扩为四个来源。
- SP-PE-R5-F5 -> repaired in candidate：Go helper/collector 以标准库 `syscall.Getrusage(RUSAGE_SELF)` 自报 `ru_maxrss` 与 CPU，
  不需要 CGO；P07 上界只对 `ru_maxrss` 求和，并写明“每进程峰值之和 ≥ 聚合峰值”的论证；需求 P07 为 RSS 口径，
  `phys_footprint` 只由 App 另报、不入门。
- SP-PE-R5-F6 -> repaired in candidate：夹具大小改为确定性分位对数正态算法（固定大文件清单、σ 二分、余量分配规则、
  失败条件），总字节精确；`small` 给出具体参数。按表中参数离线核算：`rep` σ ≈ 2.118、`small` σ ≈ 2.227，
  余量 0，Σ 对 σ 在 [0.1, 4.0] 以 0.01 步长单调不减。
- SP-PE-R5-F7 -> repaired in candidate：下限探测改为两档：p95 >200 ms 为不可达（下限已超预算）；(150, 200] ms
  为预先声明的决策阈值，写明 50 ms 余量的依据与其未证明部分，由用户决定；≤150 ms 时以 `popover.layout.data` 段核对假设。
  architecture.md 与 tasks.md 同步。
- SP-PE-R5-F8 -> repaired in candidate：“Task 1”按所指改为 `measurement-harness` 或旧 `baseline-and-open-path`；
  设计输入表中的“Task 4 保留候选”同类改为旧 `bounded-source-pipeline`。
- SP-PE-R5-F9 -> repaired in candidate：P08 行写明是绝对门、不与 B0 比较；tasks.md 的 B0 只用于 P09。
- SP-PE-R5-F10 -> repaired in candidate：`prototype/README.md` 的失效链接改为指向固定提交 `c6ea1ca` 中该节的纯文本说明；
  README 不在 serving manifest 的固定清单内，标本身份不受影响。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `d99397e918bebd1fda926cf2f859331bc707064f`；
digest `5804186654b2e6da88f4bc7703e47d355e0cc8738934c8ae9418c5e87c830352`。`prototype/README.md` blob `8950af7cabde933a91aabb8662d4d49f29969635`。
验证：`python3 scripts/check-docs.py` exit 0；文中已无 “Task N” 与 “≥20 对”；F6 算法以独立脚本按文中参数复算。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 6 — 2026-10-22

## 📋 全 topic 重新设计：性能评估与测量契约复评

📊 总体评分：8/10

✅ 复评结论：FAIL

完成门禁：NOT_VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

[performance-evaluation.md:128,141] SP-PE-R5-F6（低）：仍未关闭，今天相关子集的大小与确定性算法矛盾
- Disposition：still open（部分修复）。全集大小的算法已经确定，σ 也已复算核实；但第 4 步规定由种子把大小随机分配到文件路径，而第 128 行要求“今天相关 112 个文件、约 38 MB”。
- 行为风险：随机分配下，今天相关子集的期望大小约为 112 × 543,500 B ≈ 61 MB（T = 2,930,546,176 B 分给 5,392 个文件）；每个固定大文件也有约 2% 的概率落入今天子集，单个 128 MiB 文件就足以让子集大小翻几倍。P11 首批与 P08 append 的工作量都取决于这个子集，按种子不同会大幅偏离 38 MB，规模参数失去意义。
- 证据：`python3 -I` 按文中参数复算，rep σ = 2.1176、small σ = 2.2272，余量均为 0，与修复声明一致；子集大小在算法中没有约束。
- 💡 有界修复：为今天相关子集规定确定性的大小选择（例如从大小集合中按固定规则选出总和最接近 38 MB 的 112 个、排除固定大文件），再由种子分配其余部分；`small` 的 12 个文件同样给出目标总量。

[performance-evaluation.md:238–241] SP-PE-R6-F1（低，新）：P06 的交叉核对把 App 单值与“App + helper 求和的中位数”比较
- 行为风险：“稳定值”是窗口末 10 分钟、App 加存活 helper 的 RSS 求和取中位数，而终点只读 App 自身的 `ri_resident_size`。只要终点时有 helper 存活（窗口内允许最多 30 次维护激活），或末段 RSS 自然波动，差异就可能超过 10%，于是整批被判工具无效，产品行为正常时也会出现。
- 💡 有界修复：改为把终点 `ri_resident_size` 与同一时刻 App 自身的 `ps` 读数比较，或明确只在终点没有存活 helper 时核对。

### 🟢 优点

- SP-PE-R5-F1 closed：新增“P06 空闲窗口”与“资源（窗口）”行；CPU 由窗口两端读数差加 helper 自报得出，稳定 RSS 由外部每 60 s 采样，门改为产品维护激活数，OS 唤醒只另报，并说明了理由。
- SP-PE-R5-F2 closed：新增逐条件工具 pilot 表（P05 归 OTel 任务、P06 1 × 5 min 不作判定、P09 3 对），与候选 pilot 明确区分，E0 改为引用该表。
- SP-PE-R5-F3 closed：P09 正式样本固定为 20 对 ABBA，禁止提前停止或追加。
- SP-PE-R5-F4 closed：改用 `rusage_info_v4.ri_proc_start_abstime`（mach absolute time，与 `CLOCK_UPTIME_RAW` 同源），时钟测试覆盖四个来源。
- SP-PE-R5-F5 closed：Go 侧改用标准库 `syscall.Getrusage`，darwin 下 `ru_maxrss` 以字节为单位，表述正确；“每进程峰值之和 ≥ 聚合峰值”的上界论证成立。
- SP-PE-R5-F7 closed：两档阈值；(150, 200] ms 明确是决策阈值，并给出 50 ms 余量的依据及其未证明的部分。
- SP-PE-R5-F8、F9、F10 closed：编号已改为锚点名；P08 写明是绝对门；`prototype/README.md` 改为纯文本，指向 c6ea1ca 中的“首次加载与已有历史数据的对照”（已核实该节存在，在第 75 行）。README 不在任何 serving manifest 的固定清单中。

### 📝 总结

- Reviewer/Method：Claude Code 主会话（Round 5 评审者），逐条核对当前正文、固定提交、vendor 依赖与独立数值复算；无委派，无产品修改。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `d99397e918bebd1fda926cf2f859331bc707064f`；digest `5804186654b2e6da88f4bc7703e47d355e0cc8738934c8ae9418c5e87c830352`；`prototype/README.md` blob 以修复附记为准。
- Evidence：`check-docs.py` exit 0；`check-review-records.py` exit 0；σ 复算见 F6。
- CEv1：FAIL，不写入；当前状态没有记录，门禁 NOT_VERIFIED。
- 结论：10 项中 9 项关闭；SP-PE-R5-F6 仍未关闭，另新增 1 项低严重度问题，判 FAIL。

## Repair candidate — 2026-10-22（Round 6 后）

Repairer：Claude Code 主会话；范围仅为 Round 6 的 SP-PE-R5-F6、SP-PE-R6-F1。Round 6 的分数、Verdict 与 NOT_VERIFIED 门保留；
本节不是 Review Round，也不自行声明 PASS。各项均待独立复评确认。

- SP-PE-R5-F6 -> repaired in candidate：夹具算法新增第 4 步：今天相关子集只从非固定大文件中选，按大小升序取连续 k 个、
  与目标总量之差最小的窗口（并列取起点最小）；`rep` 目标 38,000,000 B、`small` 目标 786,432 B（768 KiB 整数值；按 rep 今天/全集字节比 1.09% 折算约为 732,800 B，取接近的整 KiB 值）。
  原第 4 步改为第 5 步：种子只在子集内部与其余集合内部分别打乱，并决定子集内哪些是旧会话继续，不影响两者的大小集合。
  夹具表的“今天相关”行同步给出目标与实际值。
- SP-PE-R6-F1 -> repaired in candidate：P06 交叉核对改为只比较 App 自身：终点自读 `ri_resident_size` 后立即追加一次 App pid 的
  `ps -o rss=` 读数（不计入稳定值），差异 >10% 为工具无效；helper 读数与稳定中位数不参与核对。

修复目标：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `ca8d56fdc754aed7123d50d1d2ab5a1f6f7cc21e`；
digest `3b84023e63c903edd44c30f9b6e203ec446148834b802d1b80acec996f277d93`。
验证：`python3 scripts/check-docs.py` exit 0；`git diff --check` 通过；F6 以 `python3 -I` 独立脚本按文中算法复算：
σ 仍为 rep 2.1176、small 2.2272，余量 0；rep 窗口为第 4,157–4,268 名（315,204–365,388 B，合计 38,019,460 B，偏差 +19,460 B），
small 窗口为第 287–298 名（合计 790,846 B，偏差 +4,414 B）。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 7 — 2026-10-22

## 📋 全 topic 重新设计：性能评估与测量契约复评

📊 总体评分：10/10

✅ 复评结论：PASS

完成门禁：BLOCKED（CEv1 provider 本会话不可达）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R5-F6 closed：夹具算法第 4 步规定今天相关子集只从非固定大文件中按大小升序取连续 k 个、与目标差最小的窗口，第 5 步让种子只在子集内部与其余集合内部打乱；子集的大小集合与种子无关，固定大文件不会落入子集。`python3 -I` 按文中参数独立复算：rep σ = 2.1176、small σ = 2.2272，总字节精确、余量 0；rep 窗口第 4,157–4,268 名（315,204–365,388 B，合计 38,019,460 B），small 窗口第 287–298 名（合计 790,846 B），与正文和夹具表一致。
- SP-PE-R6-F1 closed：P06 交叉核对只比较 App 自身——终点自读 `ri_resident_size` 后立即追加一次 App pid 的 `ps -o rss=` 读数，不计入稳定值；helper 读数与稳定中位数不参与核对，终点有 helper 存活不再导致误判工具无效。
- Round 5 已关闭的 SP-PE-R5-F1..F5、F7..F10 在当前内容中未回归（`rg` 无 “Task N”、“≥20 对”；P08 仍写明绝对门）。

### 📝 总结

- Reviewer/Method：Claude Code 主会话（Round 5/6 评审者），逐条核对当前正文并独立复算夹具数值；无委派，无产品修改。
- Reviewed state：HEAD `c6ea1ca6551f525cc8a757ccf2e7889fabf36402`；document blob `ca8d56fdc754aed7123d50d1d2ab5a1f6f7cc21e`；digest `3b84023e63c903edd44c30f9b6e203ec446148834b802d1b80acec996f277d93`。
- Evidence：`scripts/check-docs.py` exit 0；`scripts/check-review-records.py` exit 0；`git diff --check` 通过；夹具复算如上。
- CEv1：Neo4j MCP（`neo4j`）本会话连接失败，未查询、未写入；无已配置的本地镜像，按 Evidence 规则门禁为 BLOCKED，待 provider 恢复后按上述 digest 记录证据并重查门禁。
- 残余不确定性：`ps -o rss=` 以 KiB 报告，与字节单位的 `ri_resident_size` 比较时须换算，属实现常识，不构成发现。
- 结论：全部发现关闭，判 PASS；完成门禁未 VERIFIED。
