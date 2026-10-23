---
status: active
created: 2026-10-09
updated: 2026-10-22
---

# 性能评估与测量契约

依赖 [需求](requirements.md) 和 [架构](architecture.md)。本文规定 P01–P11 的唯一测量方法、
设计阶段的可测性判定、候选采用规则与决策门。它不是产品验收结果；验收结果只记录在
Neo4j CEv1 与各任务评审记录中。

2026-10-22 全 topic 重新设计后，本文取代此前所有测量口径、停止决定、撤销与有界接受。
历史实验、私有 A/B 与撤回口径保留在 Git 历史（`c376984`、`c6ea1ca` 中的本文件及 `tasks.md`）
和各评审记录中；它们只作为设计输入，不复用为任何指标的验收样本。

## 已有实测：设计输入

下列数字来自不同输入、构建与方法，只用于判断瓶颈与选择方案，不跨行计算加速比，
也不作为新契约下的基线或通过证据。

| 前置状态 | 观察 | 对设计的含义 |
| --- | --- | --- |
| App 已运行，每进程首次点击（100 次） | p95 536.842 ms，p99 623.639 ms，max 797.623 ms | P01 首击层远未达标；首击是 P01 的必测层 |
| 首击分段（单次诊断） | 无预布局首次 show 302–339 ms；预布局后 show 110–124 ms，但预布局本身 211–239 ms；一次候选首击 331.664 ms，其中 `popoverOpen` 154.591 ms | 成本在首次窗口/布局/提交，不在数据准备；必须细分到提交与 vsync |
| 已准备面板的后续打开（5 次） | 中位 97.714 ms | 重复点击层接近预算，需正式分布 |
| 有效 serving 缓存，App-ready → 可用模型（固定 100 次） | p95 1,104.791 ms | P02 未达标；启动期与 quota/bootstrap/helper 争用待分段 |
| 无缓存、已提交库，App-ready → 数据帧（5 次） | 900–989 ms | P04 预算内，需正式分布 |
| 空库当天首批，App-ready → 数据帧（5 次）；一次 4,299 文件首次导入 | 2,543 ms 中位；2,223 ms | P11 接近 3 s 上限 |
| 完整写事务（合成校准） | max：core 384.922 ms，session 285.332 ms | P07 写事务未达标；AcceptInventory 与 session 写入需切分 |
| 全量重建 CPU profile（单次） | `usage.scanFileMode` 累计约 27% CPU；parse、`activity.ScanMessage`、`rebuildSessions`、冷发布为可见子路径 | usage 单 goroutine 逐 source 串行消费是全量扫描瓶颈 |
| 旧 `bounded-source-pipeline` 任务保留候选 vs 该任务开始前 HEAD（5 对交替 pilot） | wall 中位 40,105 → 29,347 ms（−26.8%），CPU 70,032 → 55,341 ms，单进程峰值 RSS 163.1 → 202.0 MiB | 距 P09 的 30% 仍差；并行化 usage 消费需同时守住 P07 RSS |
| 历史文件事件更新（3 次，旧语料） | 通知到 publication 9.2–9.5 s；usage 2.3–2.5 s、session 3.9–4.3 s、derive 4.4 s | P10 需分段测；增量投影已实现但未按 2 s 门验证 |

历史测量失败的原因决定了新契约的约束：物理点击依赖操作员可见性，全局事件注入依赖
系统权限且首击路由不可控，OCR 进入计时窗口，DTrace 进程树追踪被 SIP 拒绝，私有真实语料
清理后无法重现，样本在出错后被逐轮改口径。新契约对这些依赖一律不采用。

## 测量契约

### 原则

1. **起止点在产品内。** 每个指标的起点和终点都是产品代码在固定位置记录的打点；
   外部脚本只负责准备条件、触发和收集，不用外部观察时间替代产品打点。
2. **单一时钟。** Swift 与 Go 都读取 `CLOCK_UPTIME_RAW`（纳秒）。Swift 用
   `clock_gettime_nsec_np(CLOCK_UPTIME_RAW)`，Go 用 `golang.org/x/sys/unix.ClockGettime`；
   `NSEvent.timestamp` 与 `rusage_info_v4.ri_proc_start_abstime` 与之同源（mach absolute time，
   后者按 `mach_timebase_info` 换算为纳秒）。同机跨进程打点直接相减；不使用任何墙钟时间。
   `measurement-harness` 以单元测试核对这四个来源同源，偏差 >1 ms 则该时钟方案作废并返回设计。
3. **全自动、可复现。** 不需要人工点击、不请求辅助功能/输入监控权限、不读取真实日志。
   给定同一工具版本、构建、夹具种子与机器条件，任何人可重跑同一批次。
4. **正确性在计时之外核对。** 每个样本的数据正确性在终点之后核对；失败不计时，但计入失败。
5. **口径冻结。** 批次开始后工具、构建、夹具与条件不变；中途变化使该批次作废。
   不允许事后剔除、改阈值或改统计量。

### 打点

产品内新增一个记录器（Swift `PerfTrace`，Go `internal/perftrace`），取代现有
`DesktopPerformanceTrace`、`AGENTDECK_SCAN_PROFILE` 摘要及其分散计数，阶段名为固定枚举：

| 层 | 打点 | 位置 |
| --- | --- | --- |
| 输入 | `input.event`（事件 timestamp）、`input.action` | 测量驱动投递的事件；`handleClick` 入口 |
| 打开 | `popover.show.begin/end`、`popover.layout.data`、`frame.commit`、`frame.vsync` | `togglePopover`；数据视图布局标记；提交后观察者；窗口 display link |
| 选择 | `select.intent`、`select.layout.data` | view model 切换意图入口；新选择的数据视图布局 |
| 启动 | `process.start`（内核记录的进程启动时间）、`app.ready`、`restore.identity`、`restore.decode`、`model.usable` | App 在 `applicationDidFinishLaunching` 中读 `proc_pid_rusage(getpid(), RUSAGE_INFO_V4)` 的 `ri_proc_start_abstime`（mach absolute time，不用 `sysctl KERN_PROC_PID` 的墙钟 `p_starttime`）；`applicationDidFinishLaunching`；恢复各段；model 首次可用 |
| 更新 | `source.fsevent`、`job.enqueue`、`job.start`、`ingest.read/reduce/write`、`projection.done`、`publication.commit`、`publication.bound` | watcher 回调；scheduler；helper 各阶段；serving 写入；App 绑定 |
| OTel | `otel.ack`（journal fsync 后）、`otel.normalized` | receiver；reconciler |
| 事务 | `tx.<domain>.<label>` 的 begin→commit/rollback 返回耗时 | store 写事务封装 |
| 资源（终生） | Go helper/collector 退出前：`ru_maxrss`（字节）、user/sys CPU；App 另报 `ri_lifetime_max_phys_footprint`、`ri_pkg_idle_wkups`、`ri_interrupt_wkups` | Go 用标准库 `syscall.Getrusage(RUSAGE_SELF)`，不需要 CGO；App 用 `proc_pid_rusage(getpid(), RUSAGE_INFO_V4)` |
| 资源（窗口） | `idle.window.begin/end` 时 App 的 user/sys CPU、`ri_resident_size`、`ri_pkg_idle_wkups + ri_interrupt_wkups` 当前值 | `PerfDriver` 在窗口两端各读一次 `proc_pid_rusage(getpid())`；用于 P06，见下 |

记录默认关闭；仅当 `AGENTDECK_PERF_OUT` 指向一个 0700 私有目录时启用，关闭时只有一次分支判断。
记录内容只有阶段名、uptime 纳秒、整数计数、进程角色、固定错误码与环境布尔值；
不含路径、会话 ID、日志正文、模型输入输出或凭据。App 与 helper 各写自己的 0600 文件，
helper 继承同一目录；进程以角色和启动序号区分，不以路径区分。

#### 数据帧终点

“首个可读且可交互的数据帧”按以下顺序判定，任一条件不满足则继续等待：

1. 数据视图（不是 loading、空卡片或占位）携带 `perfDataFrame(key:)` 修饰器；
   它在布局完成时记录 `popover.layout.data`，key 为 `(publication_id, client, period, panel)`，
   并携带视图实际渲染的 hero 文本（token 数、金额、日期标签）的摘要。
2. 主 run loop 上注册的 `beforeWaiting` 观察者（排序在 Core Animation 提交之后）
   在窗口可见、`occlusionState` 含 visible、窗口为 key、且第 1 步 key 等于期望 key 时记录 `frame.commit`。
3. 该窗口 `NSView.displayLink` 在 `frame.commit` 之后的第一次回调记录 `frame.vsync`。
   **终点 = `frame.vsync`**，即包含该数据的提交之后的第一次屏幕刷新。

它不是 WindowServer 合成或物理发光时间；与之相差不超过一个刷新周期，已含在终点中。
交互性以第 2 步的 key window 与数据视图控件可用为准。

#### 刺激

测量驱动 `PerfDriver` 只编入专用 `Perf` 构建配置（Release 优化 + 驱动），从
`AGENTDECK_PERF_SCRIPT` 读取脚本，在主线程执行：

- **点击**：构造与状态栏按钮窗口匹配的 `leftMouseDown`/`leftMouseUp`（timestamp = 当前 uptime），
  先后经 `NSApp.postEvent(_:atStart:false)` 投入本进程事件队列，由 AppKit 正常路由到
  `NSStatusBarButton` 与 `handleClick`。起点 = `input.event`。
  不使用 `CGEvent`、全局事件或窗口直送，因此不需要任何系统权限。
  1 s 内未出现 `input.action` 即为“未路由”，属工具无效。
- **切换**：调用 view model 公开的切换意图（与控件 action 调用的是同一入口），起点 = `select.intent`。
  它不含 SwiftUI 命中测试（亚毫秒级），此排除在记录中声明。
- **写入**：外部 writer 追加一条完整 JSONL 记录，`fsync` 文件与目录后取 uptime 记 `source.written`。

点击刺激的可行性依据：旧 `baseline-and-open-path` 任务的修复候选已用本进程投递的鼠标事件
经正常路由得到有效数值帧；此前失败的是全局注入、窗口直送与人工点击方案。
`measurement-harness` 的工具 pilot 若出现任何未路由样本，
采用**预先声明的替代刺激**：在同一位置构造事件后直接 `sendAction` 到按钮 target，
起点改为 `input.action`，并以 pilot 中两种刺激的配对差值（`input.action − input.event`）
报告被排除的路由段。替代方案只能在 `measurement-harness` 评审中确认采用，不能在后续任务中切换。

### 夹具

新增 Go 工具 `cmd/agentdeck-perf`（不进入发布产物；发布打包只构建 `./cmd/agentdeck`），
`fixture` 子命令以固定种子生成 Codex 与 Claude JSONL 树：

| 参数 | `rep`（验收） | `small`（回归/pilot） |
| --- | --- | --- |
| 文件数 | 5,400 | 400 |
| 总字节（精确） | 3,480,000,000 | 67,108,864（64 MiB） |
| 固定大文件 | 8 个：128、96、80、64、48、40、36、32 MiB | 2 个：8、4 MiB |
| 其余文件大小 | 5,392 个，分位对数正态，中位 64 KiB，夹在 [512 B, 31 MiB] | 398 个，分位对数正态，中位 16 KiB，夹在 [512 B, 3 MiB] |
| 客户端 | Codex 60% / Claude 40%（按文件） | 同 |
| Codex 归档 | 15% 位于 `archived_sessions` | 同 |
| 时间跨度 | 锚定日前 120 天 | 30 天 |
| 今天相关 | 112 个文件，目标 38,000,000 B（按下文第 4 步选出，实际 38,019,460 B）；其中 24 个为旧会话今天继续 | 12 个文件，目标 786,432 B（实际 790,846 B），其中 3 个旧会话 |
| 记录形态 | Codex 累计 token、turn_id、模型切换、工具调用；Claude 嵌套 model、工具使用、子目录 | 同 |
| 异常 | 0.1% 畸形行；1 个超长行（8 MiB） | 同 |

文件大小不随机抽样，按以下确定性算法生成（`rep` 与 `small` 相同，只换上表参数）：

1. 固定大文件按上表取值；其余 n 个文件的目标总量 T = 总字节 − 固定大文件之和。
2. 第 i 个（i = 1…n）大小为 `clamp(exp(ln(中位) + σ·Φ⁻¹((i − 0.5)/n)), 下限, 上限)`，
   向下取整为字节。中间分位的 z = 0，因此中位数不随 σ 变化。
3. Σ 大小在该区间对 σ 单调不减（生成器测试核对）。σ 在 [0.1, 4.0] 内二分求解
   （60 次迭代，float64），取使 Σ 大小 ≤ T 的最大 σ；剩余 T − Σ 字节加到未触及上限的
   最大文件上，仍超过上限则依次加到下一个，直至分配完；无法分配则生成失败（工具缺陷，
   不是夹具变体）。按上表参数，`rep` 的 σ ≈ 2.12、`small` 的 σ ≈ 2.23。
4. 今天相关子集只从第 2–3 步的 n 个文件中选，不含固定大文件：把它们按大小升序排列
   （等大按 i 升序），取连续 k 个（`rep` k = 112，`small` k = 12）、总和与上表目标之差的
   绝对值最小的窗口；并列取起点最小者。该子集的大小集合因此与种子无关。按上表参数，
   `rep` 取第 4,157–4,268 名（每个 315,204–365,388 B，合计 38,019,460 B），
   `small` 取第 287–298 名（合计 790,846 B）；生成器测试核对这两个合计。
5. 种子只决定大小到文件路径、客户端与归档位置的分配（Fisher–Yates，PCG 由种子初始化），
   以及今天相关子集内哪些文件是旧会话今天继续，和记录内容；今天相关子集只在其内部
   打乱，其余文件在其余集合内打乱。种子不影响全集与今天相关子集的大小集合。

总字节、今天相关子集字节与 ≥32 MiB（`small` 为 ≥4 MiB）文件数因此精确成立；生成器在夹具身份中同时记录
σ 与实际中位数。参数修改属于新 profile 版本，按下文流程评审后启用。

规模参数来自已记录的脱敏聚合（5,336–5,372 文件、约 3.46–3.48 GB、今天 112 文件/38 MB）。
真实日志不被读取。若需要校准分布形态，只能在用户明确批准下读取文件大小与记录类型计数的
脱敏聚合，结果作为新 profile 版本经设计评审后启用，不在实现中私自修改。

确定性：给定 `(generator_version, profile, seed, anchor_date, timezone)`，输出逐字节一致；
mtime/ctime 由锚定日确定。夹具身份 = 排序后 `relative_path:size:sha256` 以 LF 连接（无尾 LF）
的 SHA-256。锚定日取批次开始时 `America/New_York` 的本地日期，App 与 helper 以 `TZ` 环境变量
运行在同一时区。批次开始距本地午夜不足 45 分钟时不启动；任何样本跨越午夜则该批次作废。

数据库起点由产品自身在隔离 state root 上从夹具生成（`agentdeck scan`），再以 SQLite backup
拷贝为只读模板；每个样本从模板复制，身份为两库文件的 SHA-256。有效 serving 缓存同理由产品生成。
增量写入使用 `fixture append` 子命令按种子生成下一条完整记录，并给出期望的 hero 值。

### 环境与机器

验收机为当前开发机：Intel Core i7-9750H（6 核 12 线程）、32 GiB、macOS 26.7.x、
Go 1.27.x、Xcode 26.4。换机、换主版本或硬件变化即重测基线，不跨机器比较。

每个批次要求并记录：交流电源、低电量模式关闭、`caffeinate -dimsu` 保持唤醒、屏幕已解锁；
无其他 AgentDeck 实例（测量 App 使用独立 bundle ID 与隔离 HOME/state root）；夹具目录含
`.metadata_never_index`；每个样本开始前 `ProcessInfo.thermalState == .nominal` 且 1 分钟
load average < 1.5，不满足则最多等待 120 s，仍不满足的样本为工具无效。
A/B 双方使用同一工具版本、同一夹具与数据库模板，交错执行（ABBA）。

### 样本与统计

百分位用 nearest-rank：`P(q) = sorted[ceil(q·n) − 1]`。每批报告 n、min、median、p95、p99、max、
失败数、工具无效数及各阶段中位数。

| 样本类别 | 定义 | 处理 |
| --- | --- | --- |
| 有效 | 终点到达、正确性核对通过 | 计入分布 |
| 产品失败 | 终点超时、正确性不符、数据帧不可交互、漏通知 | 以 +∞ 计入分布，必然影响尾分位；不剔除 |
| 工具无效 | 刺激未路由、环境条件不满足、oracle 失败、跨午夜、打点缺失 | 整批作废；修复工具后整批重测，原批保留在 CEv1 作为无效记录 |

终点超时：P01/P03 5 s，P02/P04 30 s，P05/P10 30 s，P11 60 s。

**工具 pilot** 只验证工具与环境能产出完整、有效的样本，不产生门结果。每个条件在正式批次前
按下表运行，工具无效必须为 0，否则先修工具。E0 在 `measurement-harness` 中跑完全部条件；
此后若工具源码摘要或构建配置与该条件上次通过的 pilot 不同，正式批次前重跑该条件的 pilot。

| 条件 | 工具 pilot | 运行于 |
| --- | --- | --- |
| P01 首击 / 重复 / 重建中 | 20 / 20 / 20 + 20 | `measurement-harness` |
| P02、P04 | 各 20 次启动 | `measurement-harness` |
| P03 | 每维 20，共 60 | `measurement-harness` |
| P05 | 20 个事件 | `otel-capture-and-reconciliation`，在其正式批次前（receiver 在该任务前不存在） |
| P06 | 1 次 × 5 min 窗口，核对窗口两端读数与 RSS 序列完整；不按 30 min 判定 | `measurement-harness` |
| P07 | 1 次全量重建 + 20 次增量 | `measurement-harness` |
| P08 | 每场景 2 次，计数必须相同 | `measurement-harness` |
| P09 | 3 对 B0/S0 交错 | `measurement-harness` |
| P10 | 20 次 | `measurement-harness` |
| P11 | 首批 20；完整历史 1 次 | `measurement-harness` |

工具 pilot 与下文“候选采用规则”中的**候选 pilot** 是两件事：后者比较两个产品版本，
不验证工具。正式批次样本数见下表，为固定值；只能在批次开始前经设计变更修改。
一个条件的正式批次只跑一次，不允许“多跑几批取最好”或看到中间结果后提前停止、追加样本。
若需要重测（产品改动后），新批次绑定新的 content state，旧批次保留。

### 记录与证据

运行目录位于私有临时路径（0700），保存原始打点直至 CEv1 写入并回读确认后删除。
CEv1 证据记录：工具源码内容摘要、App/helper/framework 二进制 SHA-256、构建配置、
夹具五元组与身份、数据库模板身份、机器与环境记录、样本数、统计量、门结果、
失败与无效计数。topic 下不新建 evidence 文件，评审记录只引用 CEv1 证据 ID 与关键数值。

## 逐项可测性判定

所有十一项在设计阶段判定为**可测**。下表即实现时必须使用的方法。

| ID | 前置状态与刺激 | 起点 → 终点 | 正式样本 | 判定与依据 |
| --- | --- | --- | --- | --- |
| P01 | App 已运行、有效内存 publication；投递点击。三层：首击（每进程首次点击，进程在 `model.usable` 后静置 2 s）；重复点击（10 个进程各 20 次开/关）；重建中（后台对 `rep` 做全量重建时的首击与重复点击各 100） | `input.event` → `frame.vsync`；正确性：hero 摘要 = oracle | 首击 100；重复 200；重建中 100+100 | 可测。每层分别满足 p95 ≤200 ms、p99 ≤400 ms。刺激依据见上，替代刺激预先声明 |
| P02 | 进程退出后以 `open -n` 启动 Perf 构建，有效 serving 缓存，`rep` 数据库 | 门：`app.ready` → `model.usable`；另报 `process.start` → `app.ready`，以及 `model.usable` 后首次点击的 `input.event` → `frame.vsync`（计入 P01 首击层） | 100 次启动 | 可测。启动时间为暖页缓存口径（不清 OS 缓存，需 root 的 purge 不使用），在记录中声明 |
| P03 | 已恢复 publication、popover 打开；按固定轮转切换 client / period / panel | `select.intent` → 新 key 的 `frame.vsync` | 每维 100，共 300 | 可测。每维分别满足 p95 ≤100 ms；期间 helper/扫描/OTel 打点必须为零 |
| P04 | 无 serving 缓存、`rep` 已提交库；启动后驱动在 `app.ready` 立即打开 popover | `app.ready` → 首个数据帧 `frame.vsync` | 100 次启动 | 可测。与 P11 分开批次 |
| P05 | OTel 已启用、App 运行、popover 打开；`cmd/agentdeck-perf otel` 以种子向 loopback receiver 发送可计量事件 | `otel.ack` → 含该事件的 `frame.vsync`；另报 producer 发送 → `otel.ack` | 100 个事件 | 可测（mock producer）。真实 Codex producer 的端到端 freshness 另报，不作门 |
| P06 | App 运行、自动更新开、无文件变化/无 OTel、`rep` 已提交；窗口定义见下 | CPU、稳定 RSS、维护激活数，口径见下 | 每臂 3 × 30 min | 可测。“新增”= 候选 − 同条件基线臂（同一 run 序号配对）；每次运行都须达标。另报 App 总量与 OS 唤醒差 |
| P07 | `rep` 空库全量重建，期间同时执行 P01 重建层；以及 100 次 P10 增量 | RSS：所有时间上重叠的 helper/collector 进程自报的 `ru_maxrss` 求和（保守上界）；事务：全部 `tx.*` 耗时 | 3 次全量重建 + 100 次增量；事务取全部 | 可测。每个进程的峰值 RSS ≥ 它在任一时刻的 RSS，故求和 ≥ 真实聚合 RSS 峰值；求和 ≤256 MiB 即证明真实聚合 ≤256 MiB，不需要进程树追踪。门按需求的 RSS 口径；`phys_footprint` 只由 App 另报，不进入门。事务 p95 ≤50 ms、max ≤250 ms 按全部事务计 |
| P08 | `rep` 已提交库：no-op 扫描；一次 append（每个今天相关文件追加一条） | ingest 计数器：decode 记录数、读取字节、导入行数 | 每场景 3 次（结果须完全相同） | 可测。确定性计数，门为精确等式/不等式，是绝对门，不与 B0 比较，B0 也没有这些计数器 |
| P09 | `rep` 空库，`agentdeck scan`（CLI，同步完成）；基线 B0 = `14234ec` 构建 | 进程 wall（启动到退出）、user+sys CPU、读取字节、各阶段打点、RSS 上界 | 正式 20 对，ABBA 交错 | 可测。门：候选中位 wall ≤ 0.70 × B0 中位，且同批满足 P07。B0 无新打点，用外部 wall/rusage 与其原有 profile 摘要 |
| P10 | App 运行、自动更新开、popover 打开、`rep` 已提交；`fixture append` 写入一条完整记录并 fsync | `source.written` → hero 摘要等于新期望值的 `frame.vsync`；另报 `source.fsevent`、`job.*`、`publication.*` 各段 | 100 次，间隔 ≥5 s | 可测。30 s 内未更新记为产品失败（漏通知不改记成功） |
| P11 | 空 state root、`rep` 夹具；启动后驱动在 `app.ready` 立即打开 popover | `app.ready` → 显示今天用量（partial 标记明示）的 `frame.vsync`；另报来源选择、读取、统计、绘制各段与 `process.start` 起点 | 首批 100；完整历史完成时间 5 次（`publication` 的 coverage 达 complete） | 可测。首批样本在首个数据帧后结束进程；完整历史另跑 |

### P06 空闲窗口

- **窗口：** 首次 `model.usable` 后 120 s 且没有存活的 helper 时为起点 `idle.window.begin`，
  起点后 1,800 s 为终点 `idle.window.end`。`model.usable` 后 7 min 内仍有 helper 存活，
  记为产品失败（有限 helper 应退出），不延长等待。
- **CPU：** App 两端读数之差，加窗口内启动的每个 helper 自报的终生 user+sys CPU；
  除以 1,800 s 得平均单核占比。
- **稳定 RSS：** `cmd/agentdeck-perf` 每 60 s 以 `ps -o rss= -p <pid>` 读取 App 与当时存活 helper
  的 RSS 并求和；稳定值 = 窗口最后 10 min 的 10 个读数的中位数（nearest-rank，偶数取下中位）。
  该读取由外部进程完成，不唤醒 App。交叉核对只比较 App 自身：终点自读 App 的
  `ri_resident_size` 后，`cmd/agentdeck-perf` 立即对 App pid 追加一次 `ps -o rss=` 读数
  （不计入稳定值的 10 个读数），两者差异 >10% 为工具无效；helper 的读数与稳定值的中位数
  不参与该核对。
- **维护激活数：** 窗口内 `job.enqueue` 与 helper 进程启动打点的总数；门为总数 ≤30
  （平均 ≤1 次/分钟）。OS 唤醒差（`ri_pkg_idle_wkups + ri_interrupt_wkups`）只另报，不作门，
  因为它包含 AppKit/WindowServer 等产品无法控制的唤醒。
- **测量自身成本：** 窗口两端的两次自读发生在边界上，不计入窗口；`PerfDriver` 在窗口内
  只设一个触发终点自读的定时器，不产生打点之外的工作。

## 候选采用规则

1. 一次只改变一个机制；同一夹具、同一数据库模板、同一构建配置下比较完整周期，不用单阶段微基准代替。
2. 先跑 3 对交错的**候选 pilot**（与工具 pilot 无关）：输出（逻辑行、完整视图哈希）必须一致；
   三对均改善才保留，否则撤回改动。
3. worker 数在 1/2/4/8 中按完整周期选择：增大 worker 须带来 ≥15% 整周期收益且满足 P07，否则保留较小值。
4. 多进程 parser 须相对最佳单进程多线程整周期改善 ≥20%，并满足聚合 RSS/IPC/崩溃门。
5. 额外常驻 Go 引擎只在 P10/P05 分段证明 helper 启动/身份重建占主导，或持续 OTel 接收需要时选入；
   选入需设计增补与评审。
6. 采用后由正式批次给出门结果；pilot 不作为门结果。

## 决策门

| 门 | 内容 | 承接任务 |
| --- | --- | --- |
| E0 | 打点、驱动、夹具、统计与环境检查就绪；样本与统计一节工具 pilot 表中运行于 `measurement-harness` 的条件全部工具无效为 0；时钟同源测试通过 | `measurement-harness` |
| E1 | 新契约下的起点基线（B0 与当前状态 S0），分段定位 P01/P02/P07/P09/P10 主导成本 | `baseline-remeasure` |
| E2 | 展示路径：P01–P04、P11 达标；首击分段与 NSPopover 首次显示下限探测 | `serving-open-path` |
| E3 | 采集与资源：P07–P09 达标；并行 reduce 与短事务的等价性 | `parallel-source-pipeline` |
| E4 | 更新路径：P10 达标；增量与 full oracle 等价 | `incremental-update` |
| E5 | 生命周期与空闲：P06；事件合并、CLI 衔接；常驻扩展是否需要 | `event-updates-and-cli` |
| E6 | OTel：P05 及其对 P07/P10 的影响；身份与去重能力门 | `otel-capture-and-reconciliation` |
| E7 | 最终内容状态上 P01–P11 全矩阵重测 | `cross-surface-acceptance` |

E2 的 NSPopover 下限探测：在同一 Perf 构建中以只含一个静态文本的 content controller 测首击
`input.event → frame.vsync`（100 次）。结果按两档处理，两档都停止实现并返回设计：

- **下限 p95 >200 ms：** 下限本身已超出 P01 首击预算，在既定 UX（复用既有 popover）下首击层不可达。
- **下限 p95 在 (150, 200] ms：** 这不是不可达的证明，而是预先声明的决策阈值。剩余余量
  不足 50 ms，而数据帧相对静态文本还要承担数据视图的布局与提交；已测重复打开中位
  97.714 ms 与预布局后 show 110–124 ms 只能说明这部分增量远小于首次窗口成本，不能证明它小于
  50 ms。继续实现的风险由用户在继续尝试、改 UX 与改目标之间决定。
- **下限 p95 ≤150 ms：** 按架构“打开路径”继续；首击分段中 `popover.layout.data` 段的实测
  增量用于核对上述 50 ms 假设，核对结果写入该任务评审。

不得在此之前或之后以豁免代替该决定。

## 不作为证据的内容

浏览器原型的 rAF 计时、hosted XCTest 的空 coordinator、OCR 识别耗时、人工点击、
未冻结脚本的观察、跨输入/跨机器的比值，均不作为任何指标的证据。设计文档与共享原型不构成产品性能保证。
