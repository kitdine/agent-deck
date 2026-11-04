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

## Round 8 — 2026-10-23

## 📋 冻结真实语料夹具变更：性能评估与测量契约评审

📊 总体评分：6/10

✅ Verdict: FAIL

完成门禁：NOT_VERIFIED（评审未通过，不记录通过证据）

### 🔴 严重问题 — 必须修复

[performance-evaluation.md:154–158] SP-PE-R8-F1（高）：数据库模板与源文件树的绑定关系未定义，而产品的“未变”判定依赖 inode、ctime 与绝对路径
- 行为风险：本节规定“每个样本从模板复制”数据库，但没有规定每个样本使用哪一份源文件树、以什么方式得到、append 之后如何恢复。产品判定来源未变要求路径、`dev:ino`、size、mtime 与 ctime 全部等于入库时的值；任何复制（包括 APFS clone）都会产生新 inode 与新 ctime，路径不同的隔离 HOME 还会让所有来源变为“新增/移除”。按现文实现时，从模板起步的样本会把全部来源当作已变更重新读取：P08 的 no-op 扫描不再是 no-op（计数门必然失败或被测成全量），P02/P04/P06/P10 的起点混入一次全量重读。append（P08、P10、P07 的 100 次增量）还会永久改写工作副本，使后续条件的源文件与模板、与 CEv1 中记录的工作副本身份不再一致；P08 要求的“每场景 3 次结果完全相同”也需要每次从同一起点开始，文中没有给出恢复方法。
- 证据：`internal/ingest/ingest.go:973–978` 以 `fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)` 为 identity，`internal/ingest/stat_darwin.go:8` 以 `Ctimespec` 为 changedAt；`internal/ingest/ingest.go:945` 与 `internal/usage/usage.go:895–899` 要求 identity、size、mtime、changedAt 全等才算未变，否则记为 Mutated 并重读；`usage_source_files` 以 `path` 为主键（`internal/usage/usage.go:1387`）。现有 `cmd/agentdeck-perf/run.go:240–252` 每个样本以 `cp -cR` 克隆源文件到新的 `/private/tmp/agentdeck-perf.*` HOME，恰好触发上述全部失配；ctime 无法由用户态设置。
💡 有界修复：在“模板与增量”中规定源文件树与模板的生命周期：(a) 模板只对生成它的那一份源文件树（同路径、同 inode、同 ctime）有效，并把这一绑定写入模板身份；(b) 说明各条件的样本如何取得与模板匹配的起点（例如：只读条件共享同一份工作副本与固定 HOME 路径；会写入源文件的条件每个样本或每个场景重新生成工作副本并重建模板，或给出其他能保持起点一致的办法），以及由此产生的成本；(c) 规定 append 之后工作副本的处置，使 P08 三次结果可比、后续条件不受污染；(d) 同步 tasks.md `measurement-harness` 的验证项（例如：从模板起步的 no-op 扫描读取字节为 0）。

### 🟡 建议改进 — 推荐

[performance-evaluation.md:133–151] SP-PE-R8-F2（中）：日期平移的时间语义不完整，实现者须自行发明规则
- 行为风险：(1) `D_last` 是“最晚记录日期”，但未说明按 UTC 还是按批次时区取日期；记录时间戳为 UTC（`...Z`），晚间记录在两种口径下落在不同日期，直接决定 `d` 与“今天相关”集合。(2) 第 1 步“换算为本地时间、加 d 个日历日、再写回”没有规定平移后落在 DST 跳变处的本地时刻如何处理：春季跳过的时刻（如 02:30）不存在，秋季重复的时刻（01:00–02:00）有两个 UTC 值。语料跨越数月、批次在 2026-11-01 前后进行，这两类时刻几乎必然出现；Go `time.Date` 对重复时刻不保证取哪一个，这与第 3 步“逐字节一致”的身份承诺冲突，也可能让同一会话内记录的 UTC 先后颠倒。
- 证据：第 135 行只写“最早与最晚记录日期 `D_last`”；第 143–145 行只写“本地日期与时刻保持对应，跨 DST 不改变归属日”。产品以 `time.Parse(time.RFC3339Nano, …)` 后 `.UTC()` 归一（`internal/session/session.go:1463–1468`、`internal/usage/usage.go:1202–1204`）。
💡 有界修复：明确 `D_last` 按批次时区取本地日期；规定不存在与重复本地时刻的确定性处理（例如：不存在时刻按跳变量前移、重复时刻取与原记录相同的 UTC 偏移或取较早者），并要求平移后同一文件内记录的 UTC 顺序不变（违反则计数并记为工具无效）；tasks.md 的确定性测试同步覆盖这两类时刻。

[performance-evaluation.md:124–137] SP-PE-R8-F3（低）：重新冻结的授权与旧副本处置未规定
- 行为风险：第 124 行把冻结定为“一次性，经用户批准”，第 136–137 行又允许在代表性不足时“改日在当天使用之后重新冻结”，但没有说明重新冻结是否需要再次批准（它会再次读取真实日志），也没有说明旧副本是否保留、批次如何选择副本身份。tasks.md 第 96–97 行写“冻结一次……2026-10-23 已批准重新复制”，与“可能多次冻结”的读法不一致。
- 证据：上述行文；requirements.md 第 180–182 行只写“一次性冻结”“冻结之后测量不再读取真实日志”。
💡 有界修复：写明每次冻结（含重新冻结）都需用户批准；新副本以新的语料身份启用，旧副本保留至用户批准删除；批次只绑定一个语料身份。

### 🟢 优点

- 取消合成 `rep` 夹具的理由有代码事实支撑（会话内部结构未复现导致 O(turn²) 路径），并另有 Bug 承接产品缺陷。
- 来源根与产品发现一致（`internal/ingest/ingest.go:161–163`）；平移字段覆盖产品实际读取的时间字段（Codex/Claude 顶层 `timestamp`；Codex 会话开始时间不从 UUIDv7 读取，见 `internal/usage/usage.go:1199`）；`VACUUM INTO` 与 `cmd/agentdeck-perf/template.go:117` 一致。
- 隐私边界清晰：副本不进入仓库、评审记录、Beads 或 CEv1，只记录身份与聚合值；不存在“未来记录”与跨午夜两条约束都已写明。

### 📝 总结

- Reviewer/Method：Claude Code 主会话，逐段核对夹具一节与 P01–P11 表，对照 `internal/ingest`、`internal/usage`、`internal/session` 的来源判定与时间解析代码以及现有 `cmd/agentdeck-perf`；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `73f095177cd92fbfeecc734266c0ba5812c1e583`；digest `2af631041dadfa15b7a3066cec06f197ce4292d6b1def3b13593b45566f45ad2`。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 残余不确定性：“批次开始须晚于平移后最后一条记录”使每日可用的批次窗口从冻结时刻（本地）起算、止于午夜前 45 分钟；这是现文规则的直接推论，不单列发现，但修复 F1 时估算批次时长应考虑它。
- 结论：三项发现均为本文件缺陷，判 FAIL；修复范围为 SP-PE-R8-F1..F3。

## Repair candidate — 2026-10-23（Round 8 后）

Repairer：Claude Code 主会话；范围仅为 Round 8 的 SP-PE-R8-F1..F3。Round 8 的分数、Verdict 与 NOT_VERIFIED 门保留；本节不是 Review Round，也不自行声明 PASS，待独立复评确认。修复前已核对 F1 的代码事实：`internal/ingest/ingest.go` `fileGeneration` 以 `dev:ino` 为 identity，`stat_darwin.go` 以 ctime 为 changedAt，`internal/usage/usage.go` 只在 identity、size、mtime、changedAt 全等时视为未变。

- SP-PE-R8-F1 -> repaired in candidate（tasks.md 部分未修，见下）：“模板与增量”改为“工作副本、模板与样本起点”六条：(1) 每批次工作副本生成在固定路径 `/private/tmp/agentdeck-perf.<batch-id>/home/`，作为全部样本的 HOME；(2) 模板另记录源树绑定指纹（排序后 `relative_path:dev:ino:size:mtime_ns:ctime_ns` 的 SHA-256）与 HOME 绝对路径；(3) 每个样本先重算指纹，不等即工具无效，再只重置 state root、App Group 隔离目录与 defaults，源文件树不复制，从模板起步的 no-op 扫描读取字节与 decode 记录数必须为 0；(4) 只读条件共享同一工作副本与模板；(5) 写入条件开始前在同一路径重新平移生成副本并重建模板，结束后丢弃，后续只读条件前再次重新生成；P08 append 每次运行各重新生成一次；P07 增量与 P10 写入在同一副本上累积；(6) 写明重新生成的成本与批次窗口约束。
  - 未修部分：F1 的第 (d) 项要求同步 tasks.md `measurement-harness` 的验证项（如“从模板起步的 no-op 扫描读取字节为 0”）。本次授权范围只有 requirements.md 与 performance-evaluation.md，tasks.md 未改；该同步需要另行授权的修复或下一轮任务矩阵修订。
- SP-PE-R8-F2 -> repaired in candidate：记录日期一律按批次时区取本地日期，`D_last` 为最晚可解析时间戳的本地日期；平移写明 `L` → `L'` 的确定性解析：唯一时刻取之，春季不存在的时刻按跳变前偏移解释，秋季重复的时刻取较早者；原为 `Z` 写回 UTC、带偏移写回同一偏移；平移后每个文件内 UTC 先后关系必须保持，违反对数随批次记录，非零即工作副本无效。tasks.md 确定性测试对这两类时刻的覆盖同属上面的未修部分。
- SP-PE-R8-F3 -> repaired in candidate：冻结标题改为“每次都须用户批准”，写明 2026-10-23 的批准只覆盖当日一次复制；重新冻结须经用户批准；新增“多个副本”条：新语料身份、旧副本保留至用户批准删除、每个批次只绑定一个语料身份、比较只在同一身份内进行。requirements.md 同步。tasks.md 第 96–97 行“冻结一次”的措辞未改，同属上面的未修部分。

修复目标：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `96777aeb8a98e3edb2e8712a79d0db5007c9a5ec`；
digest `e2d9870e8ec39de277af5dbb2d586ebde8c5d822c42eb3eac659d17858327965`。
验证：`git diff --check` 通过；`python3 scripts/check-docs.py` 仍为 exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，与 Round 8 相同，不属于本文件。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 9 — 2026-10-23

## 📋 冻结真实语料夹具变更：性能评估与测量契约复评

📊 总体评分：7/10

✅ 复评结论：FAIL

完成门禁：NOT_VERIFIED（复评未通过，不记录通过证据）

### 🔴 严重问题 — 必须修复

[performance-evaluation.md:175–184] SP-PE-R9-F1（中）：“样本起点”按每个样本重置并核对指纹，与累积写入条件及在同一进程内连续采样的条件冲突
- 处置：新（由 SP-PE-R8-F1 的修复引入）。
- 行为风险：第 3 条规定“每个样本开始前”重算源树绑定指纹、不等即工具无效，并把 state root、App Group 目录与 defaults 重置为模板。但第 5 条又规定 P07 的 100 次增量与 P10 的 100 次写入“在同一副本上依次累积”：第 1 次写入后源文件的 size/mtime/ctime 已变，从第 2 个样本起指纹必然与模板不等，按字面实现整批为工具无效；若仍按样本重置 state root，库会回到模板而源文件保留此前全部追加，每个样本都要补导入之前所有追加，与“每个样本只计本次写入”矛盾。另外 P01 重复层（10 个进程各 20 次开/关）、P03（300 次切换）、P10 都在同一个运行中的 App 内连续采样，运行中的 App 无法在样本之间重置 state root、App Group 目录与 defaults。重置与核对的单位因此没有定义。
- 证据：第 175–178 行“每个样本开始前重算源树绑定指纹……然后只把 state root……重置为模板副本”；第 183–184 行“P07 的 100 次增量与 P10 的 100 次写入在同一副本上依次累积，每个样本只计本次写入”；P01/P03/P10 的样本定义见第 256–266 行的可测性表。
💡 有界修复：把第 3 条的单位从“每个样本”改为“每个运行单元”（一次 App/CLI 进程启动，或一个条件的写入序列开始前），并分别写明：只读条件在每次进程启动前核对并重置；累积写入序列只在序列开始时与模板核对，序列内不重置，每个样本的期望起点为“模板 + 此前各次追加”；同一进程内的连续样本不重置。

[performance-evaluation.md:154–156] SP-PE-R9-F2（低）：“UTC 先后关系不变，否则工作副本无效”对原本就跨越秋季重复区间的文件没有出路
- 处置：新（由 SP-PE-R8-F2 的修复引入）。
- 行为风险：若冻结语料中某个文件在原始秋季回拨日的 01:00–02:00 两段都有记录（例如先 01:50 EDT、后 01:10 EST），按本地墙钟平移到非跳变日后两者必然变为 01:50、01:10，UTC 先后颠倒。按第 155–156 行这使工作副本无效，且同一语料在任何锚定日都会重现；重新冻结也不能去掉这些历史记录，于是该语料永久不可用，文中没有给出处理方式。语料覆盖 2025-11-02 时这一情形是否出现取决于真实使用时段，无法事先排除。
- 证据：第 147–156 行的平移与先后关系规则；第 153 行只规定平移后落在重复区间时取较早时刻，没有覆盖平移前就处在重复区间的记录。
💡 有界修复：为平移前处在重复区间的记录规定确定性处理（例如按其原 UTC 偏移选择平移后的偏移，或对这类记录改按 `d × 24 h` 的 UTC 平移），使先后关系可以保持；或在冻结时检查并把这类记录数写入清单，并写明出现时的处置。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R8-F1 closed（本文件部分）：第 165–187 行写明产品“未变”判定依赖路径、`dev:ino`、size、mtime 与 ctime；模板只对生成它的源树有效并记录源树绑定指纹与 HOME 路径；样本之间不复制源文件；只读与写入条件分开处理；append 后重新生成，P08 三次起点相同；成本与批次窗口已写明。已核实所引代码事实：Perf 构建接受 `/private/tmp/agentdeck-perf.` 前缀（`apps/macos/AgentDeckApp/AgentDeckApp.swift:11–27`），隔离 App Group 目录为 `testHome/app-group`，defaults 为 `com.kitdine.agentdeck.acceptance` suite（同文件第 160–164 行）。F1 第 (d) 项（同步 tasks.md 验证项）不属于本文件，见下方承接。
- SP-PE-R8-F2 closed：第 136–137 行规定记录日期与 `D_last` 按批次时区 `America/New_York` 取本地日期；第 149–153 行对平移后的唯一、不存在、重复三类本地时刻给出确定性规则，不依赖语言库默认值；第 154–156 行要求文件内 UTC 先后不变并计数。
- SP-PE-R8-F3 closed：第 124–128 行写明每次冻结（含重新冻结）须用户事先批准，2026-10-23 的批准只覆盖当日一次复制；第 140–141 行写明新语料身份、旧副本保留至批准删除、每批次只绑定一个语料身份、比较只在同一身份内进行；requirements.md 已同步。

### 📝 总结

- 逐项处置：SP-PE-R8-F1 closed（本文件部分）；SP-PE-R8-F2 closed；SP-PE-R8-F3 closed；SP-PE-R9-F1 new（中）；SP-PE-R9-F2 new（低）。
- 跨目标承接：SP-PE-R8-F1 (d) 与 SP-PE-R8-F2/F3 引起的 tasks.md 同步（`measurement-harness` 验证项：从模板起步的 no-op 扫描读取字节为 0、指纹核对、平移的不存在/重复时刻与 UTC 先后测试；第 96–97 行“冻结一次”措辞）属于 tasks.md，不是本文件的开放发现 -> open，承接者：Beads `ad-scan-performance-doc-tasks`。
- Reviewer/Method：Claude Code 主会话（Round 8 评审者），逐条核对 Round 8 发现与修复候选的当前内容，核实修复中引用的 macOS 隔离代码；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `96777aeb8a98e3edb2e8712a79d0db5007c9a5ec`；digest `e2d9870e8ec39de277af5dbb2d586ebde8c5d822c42eb3eac659d17858327965`。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 结论：Round 8 三项发现均已关闭，修复引入两项新发现，判 FAIL；修复范围为 SP-PE-R9-F1、F2。

## Repair candidate — 2026-10-23（Round 9 后）

Repairer：Claude Code 主会话；范围为 Round 9 的 SP-PE-R9-F1、F2，以及 Round 9 总结中承接到 tasks.md 的同步项（tasks.md 侧见其评审记录）。Round 9 的分数、Verdict 与 NOT_VERIFIED 门保留；本节不是 Review Round，也不自行声明 PASS，待独立复评确认。

- SP-PE-R9-F1 -> repaired in candidate：第 3 条改为“运行单元起点”。运行单元 = 一次 App 进程启动（含其内全部连续样本）、一次 CLI 进程或一个条件的累积写入序列；指纹核对与 state root、App Group 目录、defaults 的重置在运行单元开始前进行，不等则该运行单元全部样本工具无效。同一 App 进程内的连续样本之间不重置。P07 的 100 次增量与 P10 的 100 次写入只在序列开始时核对与重置一次；第 n 个样本的期望起点是“模板 + 前 n−1 次追加”，每个样本只计本次写入，P10 的正确性以本次写入后的期望值为准。
- SP-PE-R9-F2 -> repaired in candidate：新增“逐文件先后保护”。平移以文件为单位；若按本地日历规则平移后文件内 UTC 先后颠倒，整个文件改为 UTC 平移 `d × 24 h`，先后关系必然保持，不再使工作副本无效。复核时另发现，原规则在平移后落入春季跳过区间时也会越过后续记录（例如 02:30 与 03:10 平移到跳变日后为 03:30 与 03:10），同一回退一并覆盖。回退文件中距本地午夜不足 1 小时的记录可能改变归属日；回退文件数与归属日改变的记录数随批次记录，冻结清单另记录原始处在秋季重复区间的记录数。

修复目标：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `60a29725e96e2255bc1b54612f34bb6326e6e89d`；
digest `9932d41e9b62b92ec4b211581aeac3bb8495bef1e6b177d2e8df5d6814c3facd`。
验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` 仍为 exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，与 Round 9 相同，不属于本文件。
CEv1：修复不跨越完成边界，未写入；门禁仍为 NOT_VERIFIED，待复评。

## Round 10 — 2026-10-23

## 📋 冻结真实语料夹具变更：性能评估与测量契约复评

📊 总体评分：10/10

✅ 复评结论：PASS

完成门禁：VERIFIED（CEv1 scan-perf-corpus-rr2-20261023-perf-state，两项 required criteria 均通过）

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R9-F1 closed：第 180–189 行把重置与核对的单位定义为运行单元（一次 App 进程启动及其内全部连续样本、一次 CLI 进程、或一个条件的累积写入序列）；同一 App 进程内的连续样本（P01 重复层、P03、P10）之间不重置；P07 增量与 P10 写入只在序列开始时核对并重置，第 n 个样本的期望起点为“模板 + 前 n−1 次追加”，每个样本只计本次写入。逐条件核对：P02/P04/P11 每次启动一个运行单元，P09 每个 CLI 进程一个运行单元，P08 no-op 从模板起步，均能取得与模板一致的起点，不再出现按字面实现即整批无效的情形。
- SP-PE-R9-F2 closed：第 155–161 行规定逐文件先后保护——按本地日历规则平移后若文件内 UTC 先后关系改变，整个文件改为 `d × 24 h` 的 UTC 平移，先后关系必然保持，不再使工作副本无效；同时覆盖了原始处在秋季重复区间与平移后落入春季跳过区间两种情形。回退的代价（距午夜不足 1 小时的记录可能改变归属日）已写明，并按批次计数；冻结清单记录原始处在重复区间的记录数。规则只依赖确定性计算，与第 3 步工作副本身份逐字节一致的承诺相容。
- Round 8 的 SP-PE-R8-F1..F3 与更早已关闭的发现在当前内容中未回归（第 127–128、136–141、165–193 行）。

### 📝 总结

- 逐项处置：SP-PE-R9-F1 closed；SP-PE-R9-F2 closed；无新发现。跨目标承接的 tasks.md 同步由 `reviews/tasks.md` Round 9 处置。
- Reviewer/Method：Claude Code 主会话（Round 8/9 评审者），逐条核对修复候选的当前内容，并对照 P01–P11 可测性表逐条件核对运行单元划分；无委派，无产品修改。
- Reviewed state：HEAD `4048a2b2679d0ced61ec39e9d4b2618efce054c7`；document blob `60a29725e96e2255bc1b54612f34bb6326e6e89d`；digest `9932d41e9b62b92ec4b211581aeac3bb8495bef1e6b177d2e8df5d6814c3facd`。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` 通过；`python3 scripts/check-docs.py` exit 1，12 条均来自 `reviews/incremental-projections.md` 的失效脚本链接，不属于本文件（承接者：Beads `ad-scan-performance-measurement-harness`）。
- 残余不确定性：第 6 条成本把 P07 增量与 P10 写入分别计为一次重新生成；可测性表中 P07 的增量即“100 次 P10 增量”，实际编排若合并为同一序列则少一次重新生成，只影响成本估算，不影响口径。
- 结论：全部发现关闭，判 PASS。

## Round 11 — 2026-10-26

## 📋 E2 追加探测决策契约评审

📊 总体评分：7/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

[performance-evaluation.md:374–385] SP-PE-R11-F1（中）：暖启动不可行被归为工具无效，但后续决策既与整批作废规则冲突，也没有覆盖面板探测无效的出口。

- 行为风险：同批次中 popover 暖启动若夺焦点一次，374 行要求 pilot 工具无效为零，241 行通则要求整批作废、修工具后重测；384–385 行却要求按第 2 条继续面板探测。实现者必须自行决定是否保留或废弃该批次。如果面板暖启动也夺焦点，两个条件都没有有效 p95，377–382 行三个数值分支全部无法适用；不能把缺失 p95 当成大于 150 ms 并据此建议修改目标。
- 证据：371–372 行两个条件共享可见帧／焦点变化即工具无效的定义；374 行要求零无效；377–382 行只按有效 p95 分支；384–385 行仅指定暖启动无效后进入第 2 条，没有定义面板无效的处置。架构 122–131 行也将不可见、不夺焦点列为候选本身必须满足的约束，不是已经证明可靠的测量工具前提。
- 💡 有界修复：区分候选不满足不可见／焦点约束与真正的测量工具故障；写清适用范围是单条件还是整批，以及 popover 有效／不可行、panel 有效／不可行／工具故障的决策出口。没有有效测量时不得进入数值失败或目标不可达分支。同步必要的架构／任务交接措辞，不预先选择 UX 或修改目标。
- 处置：open，待修复；本轮未修改被评文档。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 已通过 Neo4j 回读核实 floor 证据的 100 次、零失败／零工具无效、p95 198.49 ms 与 blocked 结果，正文没有把决策档称为产品通过。
- 保留真实数据帧的正式 P01 门和 k，窗口形态变更仍须 UX 评审。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立文档决策表推演、上游需求与架构交叉核对、CEv1 原观察回读；无委派，无产品修改，未重跑性能批次。
- Scope：当前性能评估契约，重点为返回设计后新增 E2 探测与失败出口；历史 Round 10 已关闭发现不重新打开。其通过身份不覆盖后续相对门、样本编排和本次 E2 增补。
- Reviewed state：HEAD `7e7e7e340c238c3991877e160cbc59f2a19da005`；document blob `47c46ed9721fc9f424432bbbd71c820a6915a528`；SHA256(`head=<HEAD>;document=<blob>`) = `4e5871dac99320eaece4412ed2f96e2475f0ac06eae8198eaba0a9832c9dbe83`。
- Dependencies：requirements blob `315b3305c39c42d894be91b69310b434bc683630`；architecture blob `729ce59a0cdf71253ea7300e26cfc8f16e9bacb7`。工作区 `agent-deck.scan-performance`，分支 `feature/scan-performance`。
- Evidence：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0。`python3 scripts/check-docs.py` 在 `make check-whitespace` 阶段失败：已有未跟踪的 `apps/macos/.build/index-build` 中生成 JSON／lock 缺尾换行、生成 Swift header 含尾空白；未执行到后续链接检查。此检查限制不归因为被评文档缺陷，也不清理他人的构建输出。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-doc-r11-20261026-state`；本轮失败证据 `scan-perf-doc-r11-20261026-review`，结构检查限制 `scan-perf-doc-r11-20261026-integrity`。历史观察保持不变。
- 结论：一个确定的决策契约缺口足以阻止通过，按评审规则停止扩大验证；文档矩阵 Review 保持未勾选，dispatch 返回修复。

修复：scan-performance / reviews/performance-evaluation.md / SP-PE-R11-F1

## Repair candidate — 2026-10-26（Round 11 后）

Repairer：Claude Code 主会话（本次 E2 增补的设计者）；范围为 SP-PE-R11-F1。Round 11 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- SP-PE-R11-F1 -> repaired in candidate：
  - **结果分类：** [performance-evaluation.md](../performance-evaluation.md) “E2 下限探测结果与追加探测”把每个条件的正式批次
    结论分为三类：工具无效（沿用样本类别中的原因，按通则只作废该条件批次并整批重测；工具修复触及另一条件所用代码时
    该条件也重测；修复后仍无工具有效批次即“无有效结果”）、候选不可行（暖启动产生可见帧或改变前台应用/key 窗口，
    是被测候选违反约束，批次保持有效、照常报告 p95，但该形态不得采用）、有效且可行（按 p95 与 150 ms 判定）。
    pilot 的“工具无效为 0”只针对第一类，因此候选违反约束不再与整批作废通则冲突。
  - **判定出口：** 改为 warm × panel 的出口表，覆盖 warm 达标、warm 不达标或不可行后 panel 达标、两者都有有效结论且
    都不可采用、以及任一需要的探测无有效结果。无有效结果时停止并报告阻塞，不进入 UX 变更或目标变更分支；
    明确缺失的 p95 不按 “>150 ms” 处理，只有两种形态都有有效结论时才可进入目标变更分支。
  - **交接同步：** [architecture.md](../architecture.md) 首次窗口暖启动一节把不可见/不夺焦点写为候选约束（违反即候选不可行，
    不是工具无效），面板探测受同一约束，任一探测无工具有效结果时停止报告阻塞；`tasks.md` 的 serving-open-path
    范围同步三类结论与阻塞出口。未预先选择 UX 或修改目标。

修复目标：HEAD `7e7e7e340c238c3991877e160cbc59f2a19da005`；performance-evaluation blob
`b0c0e345f8c47998189a7ad2d21e93c2bc7d2694`；architecture blob `5ddf672966cd8aac77e57f0b9270733dd4191497`；
tasks blob `87bd4dd43e2907ef8f547a183c3bcd197e2004a0`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0。`check-docs.py` 仍被既有未跟踪
`apps/macos/.build/index-build` 生成内容阻塞在 whitespace 前置检查（同 Round 11 所述），未清理他人构建输出，不声称该检查通过。

#### 下一步指令

复评：scan-performance / performance-evaluation.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 12 — 2026-10-26

## 📋 E2 追加探测决策契约复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R11-F1 closed：工具故障仅使受影响条件的批次作废；共享工具修复触及另一条件时，该条件重测。候选违反不可见／焦点约束属于有效的不可行结论，不能采用，不再与工具无效混同。
- 已逐格核对 warm 达标、warm 超时延阈值／不可行后 panel 达标、两者有有效结论但不可采用、warm 无有效结果、panel 无有效结果五种出口。缺失 p95 明确不能当作大于 150 ms；工具故障未解决时停止报告阻塞。
- architecture 的候选约束和 tasks 的 serving-open-path 交接均已同步；本轮不批准具体窗口形态或目标调整。

### 📝 总结

- Reviewer：Codex 主会话，原 Round 11 评审者；Method：对照独立修复者 Claude Code 的候选，推演决策表并检查相关契约；无委派、无产品修改。
- Scope：SP-PE-R11-F1 及其修复引入的分支；历史已关闭发现维持原处置。未发现新增或回归问题。本轮不代替 architecture.md、tasks.md 的独立整份文档评审。
- Reviewed state：HEAD `7e7e7e340c238c3991877e160cbc59f2a19da005`；document blob `b0c0e345f8c47998189a7ad2d21e93c2bc7d2694`；SHA256(`head=<HEAD>;document=<blob>`) = `09d8a9bddb429ef8dea8747911d8627aed8d2be958e6eeac68e81ebe0da9f7c8`。
- Dependencies：requirements blob `315b3305c39c42d894be91b69310b434bc683630` 未变；architecture blob `5ddf672966cd8aac77e57f0b9270733dd4191497`；tasks 修复输入 blob `87bd4dd43e2907ef8f547a183c3bcd197e2004a0`，本轮仅把 performance-evaluation 的 Review 勾选。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。
- Evidence：当前修复状态运行 `python3 scripts/check-docs.py` exit 1，仍在 `make check-whitespace` 因已有未跟踪 `apps/macos/.build/index-build` 生成 JSON／lock 缺尾换行、Swift header 尾空白而停止，链接检查未执行。该工作区检查限制不是本文件的新语义发现；未清理或改写他人的构建输出。最终记录结构、topic 文档集与 scoped diff hygiene 分别检查，不冒充完整 L0 通过。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-doc-r12-20261026-state`；新观察 `scan-perf-doc-r12-20261026-review` 为 pass，`scan-perf-doc-r12-20261026-integrity` 为 blocked。Round 11 的失败证据继续绑定旧内容，不覆盖或删除；当前状态不复用其失败结果。
- 结论：唯一开放发现关闭，复评 PASS；document-integrity 未完成，任务保持 in_review，不进入 awaiting_commit。topic 还有其他文档与实现任务，未跨越 topic 完成边界。

Task checkpoint：ad-scan-performance-doc-perf；content_state `09d8a9bddb429ef8dea8747911d8627aed8d2be958e6eeac68e81ebe0da9f7c8`；gate BLOCKED。
提交建议：门禁 VERIFIED 后，授权提交本次性能评估修复、评审记录与必要矩阵变更；前提是处理既有构建产物引起的 L0 阻塞并补齐 document-integrity 证据，不混入未评产品改动。
推送建议：门禁 VERIFIED 后，再按独立授权推送 origin/feature/scan-performance；前提同上，并核验实际提交范围、贡献归属、trailer 与 SSH 签名，远端当前状态尚未核验。

后续前提：处理 apps/macos/.build/index-build 生成产物引起的 L0 阻塞，完成 check-docs.py 并补齐当前内容的 document-integrity 证据与门禁查询；不重复已完成的复评。

### L0 与门禁同步 — 2026-10-26

用户授权处理文档检查阻塞。本轮最初的 BLOCKED 与原始失败原因保留为历史；上方单一完成门禁字段更新为本次最终查询结果，语义 PASS 未重新评审。

- `.gitignore` 新增精确的 `apps/macos/.build/` 规则；目录没有被跟踪的文件，未删除或改写构建产物。
- 仅修正 performance-evaluation 与 baseline-remeasure 评审记录各一处检查器不支持的缩进续行链接，改为明确文件名／章节文本；历史结论、观察与 finding 含义未变。
- 最终 `python3 scripts/check-docs.py` 退出 0，覆盖空白、diff、文档集、链接与评审结构；产品代码／测试／配置未改（忽略规则除外）。
- 当前 HEAD `67ac8b7da27f581a58d4014a7b218321fcd1042f`；正文 blob 与本轮已评版本一致。新 digest `423e90b93bdd1562dd82e0e7cadf02ad7b1476e37ab70a919eef7ea47e050f07`，ContentState `scan-perf-doc-l0-20261026-perf-state`；新 L0 证据与显式目标绑定的语义 roll-up 分别为 `scan-perf-doc-l0-20261026-perf-integrity`、`scan-perf-doc-l0-20261026-perf-review`。
- 原观察保留原身份；只评估本次 HEAD、忽略规则与记录格式变更对语义结论没有影响。旧 blocked 观察不改写，新状态不继承旧环境阻塞。
- 本段取代上方 checkpoint 的阻塞前提：门禁 VERIFIED；文档本体已在此前授权提交中交付，本次忽略规则及记录同步尚未提交，因此协调转 awaiting_commit，暂不关闭任务。不执行提交或推送。

## Round 13 — 2026-10-26

## 📋 锁屏有效性规则定向评审

📊 总体评分：8/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

[performance-evaluation.md:222–235] SP-PE-R13-F1（中）：环境段与样本类别表对锁屏的处理冲突。

- 处置：new，承接 requirements.md Round 10 的 SP-REQ-R10-X1；该跨文档发现由本 finding 正式承接，后续以本记录闭环。
- 行为风险：同一次运行期间锁屏，222–223行的无条件工具无效规则会使整批作废；235行却要求留档计数后重跑同一单元，第5次仍被打断才无效。运行前已经锁屏的情况也没有在环境段明确等待与不产生样本，和已评需求不一致。
- 证据：222行“只有以下情形使运行单元的样本为工具无效：屏幕锁定”；235行外部打断表将“运行单元期间屏幕锁定”定义为“不使批次无效”；requirements.md:185–190 已区分运行前等待、运行中打断及第5次仍打断。
- 💡 有界修复：同步环境段为运行前准入／运行中打断两种范围，明确运行前锁屏等待解锁、不产生样本；运行中沿用样本表，第5次仍被打断才工具无效。保留另有实例、负载、供电与降频规则，不改统计阈值或样本数。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 样本类别表已有留档、计数、有限重试与失败出口；无需重设计测量方案。
- E2 结果分类与候选约束的前轮修复未变，SP-PE-R11-F1 维持关闭。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立定向契约评审，推演锁屏前／中场景并对照已评需求；无委派，无产品或性能正文修改。
- Scope：SP-REQ-R10-X1 所指环境有效性矛盾；确认一个阻断问题后停止扩大验证。历史结论保留原状态，新 finding 不重开此前已关闭问题。
- Reviewed state：HEAD `48712967713facb5df1ed2c3539ce0387fc7aa99`；document blob `b0c0e345f8c47998189a7ad2d21e93c2bc7d2694`；SHA256(`head=<HEAD>;document=<blob>`) = `abb4b060ee3822d11915b44c7672f94452e0b0bae10e3f60f6730e526ef40005`。
- Dependency：requirements blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。
- Evidence：最终 `python3 scripts/check-docs.py` exit 0，结构／文档集／链接／空白检查通过，不消除语义矛盾；无产品测试或性能批次重跑。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-pe-r13-20261026-state`；语义 fail 与结构 pass 分开记录。此前通过观察保留原内容绑定。
- 结论：FAIL；矩阵 performance-evaluation Review 复位，Beads 返回修复。需求修复提交及其 PASS 不因此撤回，本记录尚未提交。

修复：scan-performance / reviews/performance-evaluation.md / SP-PE-R13-F1

## Repair candidate — 2026-10-26（Round 13 后）

Repairer：Claude Code 主会话；范围为 SP-PE-R13-F1（承接 requirements.md 的 SP-REQ-R10-X1）。Round 13 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- SP-PE-R13-F1 -> repaired in candidate：
  - 环境段删除“只有以下情形使运行单元的样本为工具无效：屏幕锁定、另有 AgentDeck 实例运行”的无条件陈述，改为两种范围：
    运行单元开始前的环境准入——屏幕锁定时等待解锁后再开始，不产生样本，也不算无效；另有 AgentDeck 实例运行时该运行单元为
    工具无效；运行单元期间的屏幕锁定按样本类别表“外部打断”处理，同一单元第 5 次仍被打断才记工具无效。
  - 与需求“无效样本”条（requirements blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`）及 `cmd/agentdeck-perf/run.go` 的
    `waitUnlocked`、`sampleEnvironment`、`unitInterruption` 现行行为一致（只读核对）。
  - 另有实例、负载、供电与降频规则保留；样本类别表、统计阈值与样本数未改。

修复目标：HEAD `48712967713facb5df1ed2c3539ce0387fc7aa99`；performance-evaluation blob `fc6927338aa7878b549152c83b712ef25d3a4692`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / performance-evaluation.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 14 — 2026-10-26

## 📋 锁屏有效性规则复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R13-F1 closed：环境段已区分运行前等待解锁、不产生样本，与运行中外部打断重试；第5次仍打断才工具无效。与本文件样本表和已评需求一致。
- SP-REQ-R10-X1 closed：跨文档承接的同一矛盾随本 finding 闭环；另有实例、负载、供电、降频、样本数和统计阈值未改。

### 📝 总结

- Reviewer：Codex 主会话，Round 13 评审者；Method：独立核对 Claude Code 修复候选，对照运行前锁屏、运行中锁屏及第5次打断三个场景与需求规则；无委派，无产品修改。
- Scope：SP-PE-R13-F1 及其修复影响；唯一开放发现关闭，无新发现或回归。SP-PE-R11-F1 等历史已关闭发现维持原处置；不重跑产品实验或重新认证实现。
- Reviewed state：HEAD `48712967713facb5df1ed2c3539ce0387fc7aa99`；document blob `fc6927338aa7878b549152c83b712ef25d3a4692`；SHA256(`head=<HEAD>;document=<blob>`) = `025c4a39fa581586d8d028d0de54cb17473213380a907a3bb7b7e39a6ddbdc8f`。
- Dependency：requirements blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`；仅恢复 tasks.md 中性能评估 Review 勾选，不改变其他任务范围。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。
- Evidence：最终 `python3 scripts/check-docs.py` exit 0，覆盖空白、diff、文档集、链接和评审结构；原结构证据不替代当前内容检查。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-pe-r14-20261026-state`；语义及 document-integrity 均 pass，精确门 VERIFIED；旧 Round 13 fail 保留原内容绑定。
- 结论：复评 PASS，矩阵同步，Beads 转 awaiting_commit。设计文档本轮跟进已闭环，后续为 serving-open-path；topic 实现尚未完成，不查询 topic 完成门。未提交／推送。

Task checkpoint：ad-scan-performance-doc-perf；content_state `025c4a39fa581586d8d028d0de54cb17473213380a907a3bb7b7e39a6ddbdc8f`；gate VERIFIED。
提交建议：授权提交本次性能契约修复及 Round 13/14 评审记录；矩阵勾选若与 HEAD 一致则无需另行提交，不混入产品改动。
推送建议：实际提交范围、贡献归属、trailer 与 SSH 签名核验后，另行授权推送 origin/feature/scan-performance；远端当前状态未核验。

开发：scan-performance / serving-open-path

## Round 15 — 2026-10-27

## 📋 启动争用对照决策契约评审

📊 总体评分：8/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

[performance-evaluation.md:426–432] SP-PE-R15-F1（中）：新增 quiet 对照出口没有完整保留工具无效、候选不可行与有效可行的区分。

- 处置：new，open。本轮新增决策表的缺陷；Round 12 对旧表的修复和历史 PASS 保留，不改写旧结论。
- 行为风险：在缺少可判定的面板测量时提出放宽 P01 首击目标，或由实现者临时决定未覆盖组合的处理，违反冻结决策口径。
- 证据与场景：
  - quiet warm 有效可行且 p95 160 ms，panel quiet 因打点缺失、修复后仍无有效结果：430 行“有效可行且 ≤150 ms……否则……提出目标变更”会命中目标变更；432 行只覆盖 quiet warm 自身无有效结果，不能拦截 panel quiet 无效。431 行继承同一出口。
  - quiet warm 有效可行且 p95 140 ms，而交错普通 warm 无有效结果或违反暖启动约束：428–429 行都要求普通 warm 有效可行，430–432 行又要求 quiet warm 超阈值、不可行或无效，没有一行定义该组合。
  - 379–397 行已明确工具无效与候选不可行不同；需要的探测无有效结果必须报告阻塞，缺失 p95 不按大于 150 ms 处理。新增表与这项既有约束不一致。
- 💡 有界修复：补全 quiet warm × 普通 warm 与后续 panel quiet 的结果分类及出口；需要的探测无有效结果时明确停止报告阻塞，只有得到有效且不可采用的结论才允许提出目标变更。为 quiet warm 达标但普通 warm 无效／不可行规定明确处理，消除兜底“否则”的歧义；保留目标、样本数与正式 P01 口径。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- 明确原有两组探测没有争用对照，不把 200 次恒定 helper 数当作因果归因；要求新批次 ABBA 交错。
- quiet 探测与正式 P01 分开，保留真实数据帧及原定点击时机的验收口径。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话，非本次增补作者。Method：独立文档契约评审、决策表场景推演及 CEv1 记录核对；无委派。
- Scope：2026-10-27 启动争用对照增补及其上游测量规则、相关任务范围。确认上述阻断问题后停止扩大验证；不重新认证历史测量、产品实现或整个设计集。
- Reviewed state：HEAD `2a1369b23308c8fcfda16c296bad14348b965806`；document blob `ebf6686c4c27df0ddec5c87f96eb5c679fb3437e`；SHA256(`head=<HEAD>;document=<blob>`) = `04a9815b2b10b3c94db137006523cfd24dd16f78e742ac09d36ef808c710fa8f`。
- Dependencies：requirements blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`；architecture blob `6d3d0f16a3013abca3b306899916fe23674746f5`；tasks.md 第 3 项将 quiet 探测纳入 serving-open-path。工作区 `agent-deck.scan-performance` / `feature/scan-performance`。
- Evidence：CEv1 原始观察 `scan-perf-sop-probes-20261027-popover-floor` 确认两批各 100 次、p95 176.52/165.97 ms、约束违反与工具无效为 0、每次点击 helper 数为 2；本轮没有重新运行性能批次。`python3 scripts/check-docs.py` exit 0，覆盖空白、diff、受影响文档集、链接与评审结构。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-pe-r15-20261027-state`。入口查询 NOT_VERIFIED：当前新内容尚无两项 required criteria 的证据，历史观察均不匹配当前目标。新增 `scan-perf-pe-r15-20261027-review`（fail）与 `scan-perf-pe-r15-20261027-integrity`（pass），关系预检 4/4 ok；写入后精确目标查询 FAILED，语义失败与结构通过均适用，无 invalidated 或 unresolved evidence。
- 结论：FAIL；Documents 中 performance-evaluation Review 已是未勾选，保持该状态。同一文档 dispatch 退回修复，其他文档与实现任务由各自评审闭环。

修复：scan-performance / reviews/performance-evaluation.md / SP-PE-R15-F1

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Repair candidate — 2026-10-27（Round 15 后）

Repairer：Claude Code 主会话；范围为 SP-PE-R15-F1。Round 15 的评分、结论与 FAILED 门保留；本节不是 Review Round，
不自行声明 PASS，待独立复评确认。未委派。

- SP-PE-R15-F1 -> repaired in candidate：
  - 先把每个条件的正式批次明确分为四类：无有效结果、候选不可行、有效且可行 p95 ≤150 ms、有效且可行 p95 >150 ms。
  - 原单表拆为“第一步：暖启动对照”（quiet warm × 交错普通 warm，按行序首个匹配）、“争用削减”与“第二步：面板对照”
    （`P01-floor-panel-quiet` 单条件），删除兜底“否则”。
  - 评审场景一：quiet warm 有效可行 160 ms、panel quiet 无有效结果，进入第二步后命中“无有效结果 → 停止并报告阻塞”，
    不再命中目标变更。
  - 评审场景二：quiet warm 有效可行 140 ms 而普通 warm 无有效结果，命中第一步第 2 行（阻塞）；普通 warm 候选不可行，
    命中第 4 行（暖启动不可采用，进入第二步）。
  - quiet warm 有效可行 >150 ms 而普通 warm 有效可行 ≤150 ms 归入第 5 行：按正式口径采用暖启动，quiet 只作归因记录。
  - 争用削减中候选批次无有效结果或候选不可行时停止并报告阻塞；候选用尽仍 >150 ms 时交用户决定，不自动进入 UX 或目标变更。
  - 目标变更只在暖启动与面板两种形态都得到有效且不可采用的结论时提出；缺失 p95 不按 >150 ms 处理。
    目标、样本数与正式 P01 口径未改。

修复目标：HEAD `2a1369b23308c8fcfda16c296bad14348b965806`；performance-evaluation blob `7a17d7fa58017793a01c090145ec20051a70906a`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / performance-evaluation.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 16 — 2026-10-27

## 📋 启动争用对照决策契约复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R15-F1 closed：必需暖启动对照任一无有效结果时，前两行优先停止并报告阻塞；候选不可行单独进入面板对照；面板无有效结果也明确阻塞，不再落入目标变更。
- 按行序推演四类结果的 16 种暖启动组合：7 种无有效结果组合报告阻塞，5 种有结果但至少一臂不可行的组合进入面板对照，2 种普通 warm 达标组合继续既定打开路径，1 种进入争用削减，1 种两臂超阈值进入面板对照。所有组合都有唯一的首匹配出口。
- 面板四类结果分别进入阻塞、UX 变更建议、目标变更建议（超阈值或不可行）。争用削减阶段的无效／不可行及候选用尽也有停止出口；缺失 p95 不再被视为超阈值。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话，Round 15 评审者；Method：独立核对 Claude Code 修复候选，对决策表作穷举场景推演并核验前后契约；无委派。
- Scope：SP-PE-R15-F1 与修复涉及的暖启动、争用削减和面板出口。唯一开放发现关闭，无新增或回归发现；历史已关闭发现保持其原处置。未改的需求与架构 blob 和本会话前轮一致，复用其相关上下文；不重新认证性能批次或产品实现。
- Reviewed state：HEAD `2a1369b23308c8fcfda16c296bad14348b965806`；document blob `7a17d7fa58017793a01c090145ec20051a70906a`；SHA256(`head=<HEAD>;document=<blob>`) = `db04a7e01d9fb0294e3036cef80c86d16c37cd2c93528a62904544ec872b34cb`。
- Dependencies：requirements blob `33dee36077c29b565e7c4a7aa8ce3e7f3957207d`；architecture blob `6d3d0f16a3013abca3b306899916fe23674746f5`；工作区 `agent-deck.scan-performance` / `feature/scan-performance`。本轮只恢复 performance-evaluation 的 Documents Review 勾选。
- Evidence：决策表的 16 种暖启动组合、4 种面板结果及争用削减出口完成静态核对；正式 P01 点击时机、数据帧、阈值与样本数保留。`python3 scripts/check-docs.py` exit 0，覆盖空白、diff、受影响文档集、链接与评审结构。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-pe-r16-20261027-state`。入口查询 NOT_VERIFIED；新增 `scan-perf-pe-r16-20261027-review` 与 `scan-perf-pe-r16-20261027-integrity` 两项 pass，关系预检 4/4 ok、创建关系 4/4，最终精确目标查询 VERIFIED，missing／invalidated／unresolved 均为空。旧 Round 15 fail 仍绑定旧内容，不覆盖其观察。
- 结论：复评 PASS。此文档属于 dispatch `ad-scan-performance-doc-perf`，不是无任务的文档。主题仍有 architecture.md、tasks.md 待评及后续实现任务，未到 topic 完成边界。

Task checkpoint：ad-scan-performance-doc-perf；content_state `db04a7e01d9fb0294e3036cef80c86d16c37cd2c93528a62904544ec872b34cb`；gate VERIFIED。
提交建议：另行授权提交性能评估增补与修复、Round 15/16 及修复记录、对应 Documents 勾选。
推送建议：核验实际提交范围、贡献归属、签名及远端状态后，另行授权推送 origin/feature/scan-performance；远端当前状态未核验。

评审：scan-performance / architecture.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 17 — 2026-10-27

## 📋 无数值门槛的对比报告契约评审

📊 总体评分：7/10

✅ 评审结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

- **SP-PE-R17-F1（高，OPEN）** — `performance-evaluation.md:243–248, 290–296`：新的通用规则要求所有对比批次由完整 ABBA 组组成、每臂偶数个运行单元，不完整结构不得产生对比结果；但 P06 仍是每臂 3 次独立 30 分钟窗口，P07 全量重建为 3 次、P08 每场景 3 次、P11 完整历史为 5 次，均不能组成该规则要求的完整组。P06 虽另定义同序号差值，未声明豁免新的全批次 ABBA 规则。
  - 行为风险：按表完成规定样本仍无法签发 E5/E7 所需对比报告；实现者必须擅自增样本、改变运行单元或放弃 ABBA 才能继续。
  - 证据：ABBA 每组每臂恰有两个单元，3/5 个不能整分；样本起点 186–202 行明确单元为进程/累积序列，不能事后把一次完整窗口拆成独立单元。269 行又禁止批次后改变正式样本数。
  - 💡 有界修复：逐项明确哪些报告使用 ABBA 比值/区间、哪些使用配对差值或确定性计数；在设计中预先统一相应样本数与运行单元，并同步任务要求，不在已运行批次后补样本或宣称不完整组有效。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

保留正式样本有效性、正确性核对、机器/语料身份与失败计数；历史首击探测明确停止，旧 `k` 不再充当性能验收门。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话；Method：独立增量评审与运行单元计数反例；无委派，未修改产品或测量契约。
- Scope：从数值门改为报告的统计契约、候选采用、E0–E7 与依赖需求；历史 Round 16 已关闭问题保持关闭，旧探测决策表已依本轮需求撤销。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；blob `107137504689fffedd5e10d45e9558e2a87bad63`；digest `29f3aba8bc9a72b305a1455e38f9d77087b3931393295535d86fa848c15e8648`，配方 SHA256(`head=<HEAD>;document=<blob>`)。
- Evidence：`python3 scripts/check-docs.py` exit 0；上述完整组计数反例为静态合同证据，不需要运行数小时性能批次。结构检查不能证明统计契约可执行。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；目标 `scan-perf-docbatch-20261027-perf-state`。最终门禁见本轮单一完成门禁字段。
- 结论：FAIL，Documents Review 保持未勾选。工作区 `agent-deck.scan-performance` / `feature/scan-performance`；未提交或推送。

下一步指令：修复：scan-performance / reviews/performance-evaluation.md / SP-PE-R17-F1

## Repair candidate — 2026-10-27（Round 17 后）

Repairer：Claude Code 主会话；范围为 SP-PE-R17-F1。Round 17 的结论与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS。未委派。

- SP-PE-R17-F1 -> repaired in candidate：“对比报告”改为按样本结构预先固定的五类方法表，样本数与运行单元不变：
  - ABBA 比值与区间：P01 三层、P02、P04、P11 首批、P03、P10、P09。每臂运行单元数均为偶数。连续两个配对序号组成
    A B B A 组作为重抽单位；组不完整时不出区间，按工具无效处理。
  - 配对差值：P06 的 3 对逐对“被比较 − 基线”及中位数，不出区间。
  - 两臂并列：P07 的 3 次全量重建（各含 100 次增量）与 P11 完整历史的 5 次，给两臂值与中位数之比，不出区间。
  - 确定性计数：P08 两臂计数并列。
  - 仅绝对值：P05，没有基线臂。
  - 运行单元一律按序号配对交错（奇数序号先基线），不在批次后增减样本。
- 同步：requirements.md“对比方法”一句改为引用上述分类（requirements blob `ca2fafe1a7eb816266650c60c723bb65355621f5`）。

修复目标：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；performance-evaluation blob `eaa1bd16de35014f8b53746366bba123da519805`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / performance-evaluation.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 18 — 2026-10-27

## 📋 分类报告修复与正式样本一致性复评

📊 总体评分：8/10

✅ 复评结论：FAIL

完成门禁：FAILED

### 🔴 严重问题 — 必须修复

- **SP-PE-R18-F1（高，new）** — `performance-evaluation.md:252, 274–275, 297`：分类报告表把 P07 定为“3 次全量重建，各含 100 次增量”（每臂 300 次增量），正式样本表仍为“3 次全量重建 + 100 次增量”，且禁止批次后改变样本数。
  - 行为风险：相同合规批次可能按 100 或 300 次增量汇总，改变全量/增量事务的权重及 p95；报告工具与正式采集无法共享唯一预先固定的口径。
  - 证据：252 与 297 行的直接数量反例；本轮 Repair candidate 同样写“各含 100 次增量”，不是旧记录偶发笔误。两臂并列不消除采样总数矛盾。
  - 💡 有界修复：选定唯一 P07 正式样本数、增量附属哪个运行单元以及全部事务的汇总范围，统一两表及任务引用；若保持既有 100 次总数，不应无意扩成 300。仅修订合同，不事后给已完成批次补样本。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

SP-PE-R17-F1 closed：243–254 行明确划分五类报告；P06、P07、P08、P11 完整历史不再受完整 ABBA bootstrap 约束，P05 单臂。原奇数单元无法形成完整组的问题已解决，不机械保留为开放发现。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra 主会话，非修复作者；Method：独立定向复评、3×100 与 100 的数量推演及上下游核对，无委派。
- 处置：SP-PE-R17-F1 关闭；新增 SP-PE-R18-F1；历史锁屏及探测相关已关闭发现无回归。通用“ABBA”文字按新增明确分类表限定为适用条件，不单独据此重开原发现；任务中明确要求全矩阵 ABBA 的执行目标另归 tasks Round 16。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；blob `eaa1bd16de35014f8b53746366bba123da519805`；SHA256(`head=<HEAD>;document=<blob>`) = `de09b945d0a40d475c7b86e62d4690ef7dae6494cf1d1cb77f1993f4d4faa0ee`。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；target `scan-perf-docrecheck-20261027-perf-state`；精确 gate FAILED：`perf-review` 为 fail，`perf-integrity` 为 pass，均使用该前缀；无 unresolved impact。`python3 scripts/check-docs.py` 与 `git diff --check` exit 0。不运行性能批次来裁定合同内部矛盾。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`；correlation `QtgKmOTPe89-k69i`。本轮只更新记录/状态，不修正文案或产品。

下一步指令：修复：scan-performance / reviews/performance-evaluation.md / SP-PE-R18-F1

## Repair candidate — 2026-10-27（Round 18 后）

Repairer：Claude Code 主会话；范围为 SP-PE-R18-F1。Round 18 的结论与 FAILED 门保留；本节不是 Review Round，不自行声明 PASS。未委派。

- SP-PE-R18-F1 -> repaired in candidate：P07 的唯一样本口径定为“3 个运行单元，每单元一次全量重建；100 次增量只在第 3 个
  运行单元的重建之后执行”，与正式样本表“3 次全量重建 + 100 次增量”一致，也与工具现行编排一致（只读核对
  `cmd/agentdeck-perf/run.go` P07 计划：`formal: schedule{Units: 3, Inner: 100}`，增量循环条件 `index == s.Units`）。
  事务汇总范围定为该臂 3 次重建与 100 次增量中的全部 `tx.*`。上一轮 Repair candidate 中“各含 100 次增量”的写法以本节为准；
  样本数未改。

修复目标：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；performance-evaluation blob `9411045950822bd13701f63ab0cce5953baf5606`。

验证：`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0；`python3 scripts/check-docs.py` exit 0。

#### 下一步指令

复评：scan-performance / performance-evaluation.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 19 — 2026-10-27

## 📋 P07 样本口径修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- SP-PE-R18-F1 closed：`performance-evaluation.md:252` 把 P07 定为“3 个运行单元，每单元一次全量重建；100 次增量只在第 3 个运行单元的重建之后执行”，与 297 行正式样本表“3 次全量重建 + 100 次增量”数量一致；事务汇总范围明确为该臂 3 次重建与 100 次增量中的全部 `tx.*`，与 297 行“事务取全部”一致。192、201、204 行的累积序列与写入副本重新生成次数（P07 1 次）同样与“增量只在一个单元内累积”相容。
- 合同与工具现行编排一致：只读核对 `cmd/agentdeck-perf/run.go:971–978`，P07 `formal: schedule{Units: 3, Inner: 100}`，增量循环条件 `index == s.Units`，即仅第 3 单元执行 100 次。
- topic 文档中不再有“各含 100 次增量”或 300 次的表述；Round 17 后 Repair candidate 中的旧写法已由 Round 18 后 Repair candidate 明确取代。
- SP-PE-R17-F1 保持关闭：五类报告方法表除 P07 单元格外未变，奇数单元条件仍不受完整 ABBA 组约束；历史已关闭发现无回归。

### 📝 总结

- Reviewer：Claude Code 主会话（Opus 5.5），非 Round 18 评审者；Method：定向复评，逐行核对 P07 在分类表、正式样本表、样本起点与写入条件中的数量与 `run.go` 编排；无委派，未修改被评文档或产品。
- 处置：SP-PE-R18-F1 closed；无仍开放、回归或新增发现。
- Reviewed state：HEAD `c778fef45ed7490020c0fea710582ac1f17bab94`；document blob `9411045950822bd13701f63ab0cce5953baf5606`；SHA256(`head=<HEAD>;document=<blob>`) = `3a3f141f9ebaedd5abc08c60f4d4d20103613d3af0923cdebe5afb53cd25fc99`。
- Evidence：`python3 scripts/check-docs.py` exit 0；`bash scripts/check-topic-docs.sh scan-performance` exit 0；`git diff --check` exit 0。不运行性能批次来裁定合同文字一致性。
- CEv1：WorkUnit `scan-performance:performance-evaluation.md`；ContentState `scan-perf-pe-r19-20261027-state`。新增 `scan-perf-pe-r19-20261027-review`（measurement-attribution）与 `scan-perf-pe-r19-20261027-integrity`（document-integrity）两项 pass，关系 4/4 创建；精确目标门禁查询 VERIFIED，两项准则均有本状态证据。Round 18 fail 仍绑定旧内容，不覆盖其观察。
- 结论：复评 PASS，恢复 tasks.md Documents 中 performance-evaluation.md 的 Review 勾选。此文档属于 dispatch `ad-scan-performance-doc-perf`。工作区 `agent-deck.scan-performance` / `feature/scan-performance`；未提交或推送。

Task checkpoint：ad-scan-performance-doc-perf；content_state `3a3f141f9ebaedd5abc08c60f4d4d20103613d3af0923cdebe5afb53cd25fc99`；gate VERIFIED。
提交建议：另行授权提交 performance-evaluation.md 的无数值门槛重设计与 R17/R18 修复、Round 17–19 及修复记录、Documents 勾选。
推送建议：核验实际提交范围、签名与远端状态后，另行授权推送 origin/feature/scan-performance；远端当前状态未核验。

下一步指令：复评：scan-performance / tasks.md

WORKFLOW_WORKSPACE: agent-deck.scan-performance


## Round 20 — 2026-10-29

## 📋 OTel 设计变更批量独立评审 — performance-evaluation.md

📊 总体评分：9/10

✅ 评审结论：PASS

完成门禁：VERIFIED

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

P05 从正式样本、pilot、超时与统计分类表移出；E6 明确为正确性验收，OTel 打点保留为诊断，其余十项方法不变。

### 📝 总结

- Reviewer：Codex 主会话；Method：独立文档差分、状态转换推演、现有源码/SQL 与标本核对；未参与本轮设计，未委派。
- Scope：P05/E6 移出及需求、Task 7/8、测量方法的交叉核对。
- Reviewed state：HEAD `5a57b26b15142ae980f5e19976555314a8759f2a`；document blob `913a03fa724daca96e2e628ef6b4f9e35269fc08`；digest `9f4e5606b388544944cfa15562a690a7d3c0b5133d529d3c94d553e13487df8e`。
- ContentState：`scan-perf-otel-docreview-20261029-perf-0SDA62-state`；WorkUnit `scan-performance:performance-evaluation.md`；dispatch `ad-scan-performance-doc-perf`。
- Evidence：本轮 `python3 scripts/check-docs.py`、`bash scripts/check-topic-docs.sh scan-performance`、`make check-whitespace`、`git diff --check` 均通过。结构检查不替代语义评审。CEv1 入口查询两项 required criteria 均缺本目标证据，旧观察不重标；最终结果以本轮完成门禁字段为准。
- 变更未改变 P01–P04/P06–P11 的采样、ABBA/配对/两臂并列分类。既有 Round 19 的 SP-PE-R18-F1 关闭处置保留；未变方法复用既有语义评审，不重跑性能批次。tasks.md:194 残留的 P05 单臂统计义务属于任务分解，已由其本轮 SP-TASKS-R19-F2 承接，不是本文件缺陷。
- 工作区 `agent-deck.scan-performance` / `feature/scan-performance`。只更新评审、矩阵与协调/证据，保留现有产品改动；未提交、推送或运行真实配置变更。

Task checkpoint：ad-scan-performance-doc-perf；content_state `scan-perf-otel-docreview-20261029-perf-0SDA62-state`；gate VERIFIED。
提交建议：门禁已 VERIFIED；在独立授权下提交 performance-evaluation.md、本轮评审记录及对应矩阵状态；可与修复完成后的设计批次统一交付。
推送建议：门禁已 VERIFIED；核验提交范围、署名/SSH 签名与远端，再另行授权推送 origin/feature/scan-performance；当前远端未核验。

CEv1 收口：标准 gate-status 查询返回 VERIFIED；两项 required criteria 的结构证据通过，独立评审证据为 pass，无 unresolved candidate impact。6 个状态、12 个观察及 24 条关系已读回核验；关系批次预检 24/24 ok。未查询 Topic 门禁，因为仍有未完成设计与实现。
