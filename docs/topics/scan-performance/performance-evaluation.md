---
status: active
created: 2026-10-09
updated: 2026-10-09
---

# 先行评估：哪些等待能被减少

依赖 [需求](requirements.md) 和 [架构](architecture.md)。这是设计候选与有界实验，
不是产品实现验收或独立评审 PASS。

## 重新判断

当前 macOS App 已常驻，`togglePopover` 直接打开真实 NSPopover，不扫描。
因此不能用 scan 的 778 ms 与缓存读取的 3 ms 相除，宣称日常点击快了几百倍。
额外常驻 Go 进程也不能凭“消除点击扫描”论证收益，因为点击没有这段扫描。

实际有两个空缺：已有数据时coordinator没有启动恢复；库空时要先等大量原始日志整理。
首次取得数据还要经过额度、scan、snapshot helper；已有数据的后续刷新则保留旧内存快照。
快照恢复只解决前者。后者必须首批当天优先并在全局工作完成前发布。
已有库但无保存视图则先只读今天，不先准备90天的所有维度。
后台更新的旧实验仍约 9 秒；恢复只能改善等待已存数据的体验，不能将旧数据变成实时新数据。

采用顺序：首次当天优先/已有库当天准备/完整恢复 → 实测usage及统计热点 → 文件事件与CLI衔接。
并行预算、额外常驻引擎和多进程只在对应成本与收益证据成立后选入。
OTel 为可选持续输入，有自己的接收/身份/去重门；关闭时快开仍须成立。

## 首次加载与已有历史数据的对照

这里“有历史数据”指AgentDeck自己已有已整理数据库。只有本机Codex/Claude日志，
AgentDeck库仍空，属于首次导入；缓存恢复15ms不适用。

2026-10-09重新捕获冻结输入：5,336文件，3,457,009,079 bytes，
SHA256 `058ac968e0fbb1d7e097d205590a786fc891e3786ac8da2b46d6831faf8610c6`。
“今天”为America/New_York 2026-10-09，即UTC 10-09 04:00至10-10 04:00。
保留原mtime；双方同一private binary，普通路径与day-hint分支共用parser/SQLite/schema。
patch只安装在HEAD导出副本，产品Go/Swift不改。捕获/构建/fixture准备不计用户采集时间。

| 前置状态/数据准备方式 | 样本 | 中位/范围 | 验证范围 |
| --- | --- | --- | --- |
| 库空：当前全量scan + snapshot | 3 | 75.438 s；74.814–78.249 s | 全量数据准备 |
| 同一空库：优先今天相关来源 + snapshot | 3 | 1.554 s；1.426–1.650 s | 今天usage-period三客户端结果与全量完全一致 |
| 今天记录已入库，无有效派生缓存：全量desktop snapshot | 3 | 4.627 s；4.542–4.754 s | 完整snapshot准备，读所有现有维度 |
| 同一库：usage stats today --no-scan | 3 | 103.968 ms；82.204–104.465 ms | 今天基础用量，非完整popover envelope；totals/cost与全量一致 |
| 已有库、无新增来源、当前scan + snapshot | 5 | 1.147 s；0.921–3.108 s | 当前刷新路径 |
| 同条件、有效派生结果，只读snapshot | 5 | 544.122 ms；455.479–597.224 ms | 完整只读snapshot；没有完整展示保存文件 |
| 历史库已整理、今天112个来源尚未入库：当前完整刷新 | 2 | 14.502 s；13.601–15.402 s | 由同一历史库SQLite backup构造的两对pilot |
| 同历史库：等usage完成后只读今天，其他工作继续 | 2 | 5.664 s；5.395–5.933 s | 今天基础用量提前可用；其他域/统计不算已完成 |
| 历史库原位复核今天优先 | 1 | 15.545 s | 今天totals正确，但明显慢于backup组；差异未定位，不推广5.7s为稳定收益 |

另外，上轮有效完整展示快照恢复到原生模型的15.089ms仍只适用于已经保存了视图的条件，
见下方原生A/B。以上所有本轮数字是CLI/引擎采集或查询到有效JSON的时间，
不是点击icon到真实native绘制；最终原生联调和尾延迟门保持未完成。

当天优先选中112文件、38,255,070 bytes（约38.3MB），完整发现metadata的成本仍在scan里。
减少的是首批要解析和写入的输入，不是把3.46GB凭代码微调压到1秒。
源选择使用今天修改的文件、Codex目录的UTC日期范围提示，包含旧会话今天继续活动的文件。
选中文件仍完整解析保留上下文，统计按event时间归今天，不任意tail或清空累计基线。

三对今天usage-period投影严格一致；补充复核today summary、session periods、
hourly/quality/pricing一致。首次扩展比较遇到真实时钟跨小时，through_hour和桶数变化；
同一小时重算双方后全部通过，不把时间差改记成数据范围收益。没有核验全量其他时段/工作信号。
Hints不是完整性证明；产品必须保留全inventory，并把初始来源覆盖标partial/unknown。
实验过滤不能直接应用已有库，否则遗漏来源可能被解释成删除。

历史未采集today的负结果很重要。原位样本usage阶段12.045s；一对backup的当前全刷新
discovery261ms、usage5.853s、session7.572s、derive5.872s、total14.520s，usage/session重叠。
其worker-lifetime peak RSS651,640,832 bytes（约621.5MiB），仍不满足256MiB预算。
今天usage域先可用会减少等session/derive的时间，但没有消除usage本身的长等待。
备用副本与原位差异尚未定位，不能声称新增数据实时问题解决，不能猜是SQL/WAL或加线程可治。

因此本设计采用“库空先今天、有库先读已提交今天、上次视图可恢复”，
把新日志入库瓶颈保留为独立必须解决的门。历史全部补齐仍约分钟级；首批快不代表全量导入更快。
这是第一次可用数据的策略，不改变用户目标为“今天与历史二选一”。

完整脱敏样本/内容身份/阶段与限制见[首次加载结果](ux/prototype/serving/first-loading-results.json)。
复测脚本为[firstload](../../../prototype/tools/scan-performance-firstload.mjs)、
[private patch](../../../prototype/tools/scan-performance-firstload-patch.py)、
[已有库bootstrap](../../../prototype/tools/scan-performance-db-bootstrap.mjs)、
[历史待采集today](../../../prototype/tools/scan-performance-history-today.mjs)。
第一轮harness误读usage.changes.files后停止，源接口核对后修正；该轮不计时。
源文件/数据库/私有视图原件最终清理，只保存耗时、资源、来源数/bytes和校验摘要。

## “首屏”的确切含义

| 场景 | 前置状态和起点 | 终点 | 前后比较 |
| --- | --- | --- | --- |
| 日常点击 | App 已运行，双方均有同一份有效内存数据；原生 icon action | 真实 popover 显示选定 scope/window 的有效数字与日期，原生绘制完成，控件可用 | 当前内存展示 vs 候选内存展示；不能让基线先 scan |
| 启动恢复 | 双方内存为空、同一已提交数据库；完整 OS launch 和 App-ready 后数据准备分开记录 | 恢复/查询所得正确数据完成原生绘制 | 当前首次刷新 vs 完整缓存恢复；同时报告缓存是否已存在、年龄及窗口 |
| 自动更新 | popover 已打开；一条已知 JSONL 记录完整写入来源；OTel 则另从 producer/receiver 起点计时 | 相应新数值完成原生绘制 | 同一 delta、同一数据库起点、同一触发与可见验证方法 |

窗口出现、spinner、空卡片、coordinator 有值或 CLI 返回都不能独自作为终点。
保存的旧值可以是可用数据，但其日期、年龄和 coverage 必须真实；这不等同于最新数据。
“先显示旧数据”的候选与“等待更新后显示”的基线还须另报最新数据到达时间。

本轮原生实验的实际终点是 AppKit layout/display/cacheDisplay 完成，并在内存 bitmap 上
核验可见数字；它不是 WindowServer/物理显示器真正呈现的时间。缓存暖态、Debug build、
XCTest 及输入事件注入的限制必须明说，不能扩写为完整 release App 冷启动。

## 原生 A/B 实验

使用 HEAD `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e` 的私有导出副本，
生产 Go/Swift、原日志、配置与生产数据库保持不变。
重新捕获的真实副本为 5,328 个 JSONL，3,453,693,095 bytes，冻结后供双方复用。
content multiset digest 为 `80ec082591648c5d3f5a104ab1dc23caae4630c81403e1b44708961423d65246`，
算法是排序 `size:content-SHA256`，用 LF 连接且无尾 LF，再计算 SHA256。
同一 stock helper SHA256 `8e5baf400c8460ee213f591a21164ec15af8ec6944edeacff14a993ae1d07172`。
缓存 payload 93,813 bytes，SHA256 `cb695a00e54bd6d06e015806ddf465a0346df96f2a7a558e4d5e94c46dba747c`。
输入捕获不是 live HOME 原子备份；双方使用捕获完成后相同的冻结副本。

实验脚本：[私有原生 instrumentation](../../../prototype/tools/scan-performance-native-ab.py)、
[固定输入和缓存准备](../../../prototype/tools/scan-performance-native-input.mjs)。
脚本仅接受私有 native experiment root，额外 accessors/test 只安装在导出副本。
真实 `MenuBarItemController`、NSStatusItem、NSPopover、NSHostingController、完整
`MenuBarSurfaceView` 与 `EmbeddedHelperRunner` 均保留；没有浏览器或简化卡片替代。
默认 all/today；quota 关闭，实际展示 usage；未测 quota 网络和其他 tab 的最终性能门。

- A 空内存：点击真实 status item 后，真实 coordinator/runner 执行当前 scan + snapshot。
- B 空内存：相同点击后，读取保存文件、只读核验 core/session epochs、校验 SHA256，
  用现有 Swift wire decoder 验证，再安装完整 envelope。
- A/B warm：双方事先安装相同 envelope，点击只走当前原生展示逻辑。
- 每组一次 warm-up 不计，之后交错 AB/BA；不 purge OS page cache。
- 校验 usage presentation、session periods、所选 scope/window 一致，popover/window 可见，
  对内存绘制结果做 OCR，确认 hero 实际显示期望 token/amount；OCR 在计时终点之后运行。
  不保存真实数据截图、OCR 文本、私有路径或会话内容。

候选 restore 是最小机制实验，尚未完成产品的 owner/mode、state UUID/schema、跨天、损坏、
旧版本和后台异步读/解码等全部门。它在测试中直接读小文件，正式实现必须在后台读取。
空 coordinator 的测试排除进程创建、App 生命周期初始化及 OS 启动，不称为完整 App 重启。

结果直接保存于 [脱敏原生摘要](ux/prototype/serving/native-ab-results.json)。
首次 `performClick` 试跑因 XCTest 当前非鼠标事件触发 NSEvent.clickCount 断言而中止，
没有可用测量，不能作为产品延迟样本。第二次改为向 AppKit 注入标准 mouseDown/mouseUp，
不改原产品 handleClick/togglePopover。两次若仍无有效样本，停止这条实验方法并记录原因。

实际第二次检测为 `proof-popover-not-visible`。两次界面试跑都没有有效样本，已经停止；
未证明根因是锁屏或产品缺陷，不再切回浏览器补“首屏”数值。
另运行独立的 `testNativeDataPreparationAB`，只测真实 Swift coordinator/runner、
wire decoder 和 MenuBarViewModel 到有效模型；没有 icon、popover 或绘制计时。
同一冻结输入，先排除一对 warm-up，交错 12 对；24 个样本均通过用量 presentation、
session periods 等价以及 all/today hero 可用检查。XCTest 通过，测试耗时 28.773 s。

| 原生数据准备；空内存、有已提交数据库 | A：当前 scan + snapshot + 模型 | B：保存文件/epoch/hash/decode + 模型 |
| --- | --- | --- |
| 有效样本 | 12 | 12 |
| 中位数 | 1,919.185 ms | 15.089 ms |
| 最小值 | 1,837.533 ms | 13.015 ms |
| p95 nearest-rank / 最大值 | 2,683.757 ms | 25.795 ms |

中位数据准备等待减少约 99.2%；这不是产品首屏加速比。小样本 p95 等于最大值，
只用于先行筛选，不能当最终尾延迟验收。双方未改来源、schema/parser或统计内容，
B恢复的是相同已存数据；这轮没有新delta，不能推导最新数据处理更快。
与旧CLI约778ms也不计算速度比：原生bridge计时范围、输入和helper内容身份不同。

结论：完整恢复可以明显减少原生模型准备等待，值得进入第一阶段设计；
真实日常点击、原生数据帧和完整App重启仍未验证，P01–P04保持未完成。
未测JSONL事件更新A/B、OTel真实producer及资源预算；~9秒更新的优化仍须独立验证。

## 撤回浏览器性能口径

此前实验是一个浏览器小面板，在点击后绑定数字并等待两次 rAF。
其约 33 ms 主要受 rAF 定义影响；基线组还强制先 scan，而当前 native warm 点击不会 scan。
因此它既没有量到真实 App 的首屏，也没有提供同前置状态的点击 A/B。
该浏览器结果从 P01–P04 的性能支持证据中撤回，不据此计算产品加速比。
浏览器仅保留共享原型的状态、文案、层级、布局与开关交互验收作用。
旧数值原样保留在 [历史实验摘要](ux/prototype/serving/feasibility-results.json)，
作为实验历史而非 native 性能证据；不重新运行真实快照 HTTP server。

## 旧真实规模实验：可复用的有限事实

旧捕获为 5,314 文件、3,446,641,930 bytes，与本轮新捕获不同，不能跨轮算加速比。
旧 baseline 首次导入 85.117 s（n=1）；unchanged scan + snapshot 中位 778.505 ms（n=3）。
预备快照 HTTP read p95 3.115 ms，磁盘 epoch/hash/decode p95 4.627 ms。
这些仅说明准备好的小 payload 可短时间读取，没有证明 native 点击、冷启动或最新数据更快。
旧 binary SHA256 `4e1b83f0687bfdd0b6d38490f0e910bd31e85dc3645d8ddf152f624d927c0734`；
Go 1.27.1 darwin/amd64，Node 26.11.1，Darwin x86_64。

首个 fs notification 在 2 秒内漏收，随后显式 signal 的 11.567 s 单列，不能记作自动成功。
后两次真实通知到 publication 为 9.197/9.526 s；只有 publication，没有 native 绘制终点。
三条合成追加合计 399 tokens，计量精确、无重复；不代表真实 producer OTel 已验证。

三个 append worker 的 discovery 41–49 ms、usage 2,256–2,485 ms、
session 3,888–4,310 ms、derived-cache 4,369–4,439 ms，total 8,793–9,053 ms。
usage/session 重叠，阶段 wall 不能直接相加。会话处理与派生统计是下一步 profiling 的
明确候选；尚未证明 SQL/reducer 更细的根因。事件触发代替定时触发本身不会去掉这些成本。
worker process-lifetime peak RSS 676–688 MiB，超出 proposed 256 MiB；不能称为单次 delta RSS。
实验 Node process peak 139 MiB，3 秒 idle CPU 0.256 ms；不是最终 Go 的 30 分钟验收。

旧私有原件已经清理。保留的数值是历史观察，本轮只复用因果限制，不把它绑定成新实现 PASS。

## 之前优化的经验

| 来源 | 已有事实 | 本轮使用边界 |
| --- | --- | --- |
| [2026-07 A/B](../../archive/plans/usage-scan-performance.md#remeasurement-2026-07-22-controlled-ab) | 479 文件/639 MB；同输入交错 A/B 均值 105.213 → 19.5 s | 特定代码优化曾有效；不外推到新规模或 UI |
| [2026-09 Task 2](../snapshot-performance/tasks.md) | 2,200 文件/2.34 GB；cold 59.054 s、recompute 6.181 s、unchanged 5.791 s | 缓存与摄取之外仍有成本，不用跨 corpus 比值 |
| [旧最终边界](../snapshot-performance/tasks.md#task-3-current-state-completion-decision--2026-09-13) | pure-Go cold median 37.170 s（n=3），未达 10 s；CGO 未证明完整周期收益 | 不重复按 driver/微优化猜根因；旧延期不等于本主题 PASS |
| `ad-bug-menubar-long-scan-timeout` | 5,071 文件，一轮 1,540,148 ms，之后约 9–10 s | 长任务与前台 timeout 有问题；没有证实 parser bump 是全量重读根因 |
| 当前源码 | App 已常驻；click 不扫描；初始内存为空；已有 workers/shared decode | 先验证展示恢复；线程/额外常驻的收益需另证 |

## 候选比较与决定

| 候选 | 直接针对哪段成本 | 采用条件 |
| --- | --- | --- |
| 现有 App + 首批当天/已有库当天读/完整恢复 | 无库/有库/有保存视图三种初始等待 | 第一阶段；快照恢复不能替代无库首批采集；真实native最终门尚未通过 |
| 受影响 session 集合/dirty 统计分区 | ~9 s 更新中的 session/derive | 首先 profiling；同一 delta/full oracle 等价后测整个更新周期 |
| 文件事件合并 | 更新开始前的周期等待和重复请求 | 无固定周期；通知遗漏/唤醒有限校对；不称为处理耗时优化 |
| 当天/近期优先 | 无旧数据时较早提供已采集部分 | 不遗漏旧活跃/归档文件，不把 partial 当“当天已完整” |
| 有界 chunk/checkpoint | 重读、长事务、内存峰值 | bytes/资源证据成立；每 consumer 上下文和 rewrite 原子性通过 |
| 1/2/4/8 workers | CPU 可并行热点 | 整周期改善 ≥15%，UI 不退化，CPU 增量 ≤20%，满足内存门 |
| 额外常驻 Go 引擎 | helper 启动/状态重建，持续接收/订阅 | 量出真实进程开销或持续接收必要性；闲置资源达标；不阻挡快照恢复交付 |
| 多进程 parser | 可隔离的 CPU/GC 热点 | 相对最佳单进程多线程整周期改善 ≥20%，满足聚合 RSS/IPC/崩溃门 |
| OTel | 新调用的来源覆盖和 freshness | 默认 off；真实字段/transport/精确跨源身份通过，否则 capture_only |

排除仅按今天目录/mtime tail、增大 timeout、削弱 durability、无据的 CGO 替换。
Go 已能多线程；先明确串行比例和实际 admission/SQL/GC 成本，不能按核心数预测加速比。

## 实验顺序与决策门

### E0：冻结输入、定位首屏路径

绑定 Git HEAD/相关 blobs、instrumentation/helper hashes、工具链/OS、source/parser/schema、
timezone/window/preferences；冻结 corpus、DB起点和合成 delta。双方同输入，交错 AB/BA。
区分默认 quota、OTel off/on、warm click、空内存、有库无cache、首次import、重建压力。
记录 queue/processing/publication/UI 各段；不把并行 wall 相加，不保存正文/源路径。

### E1：先证伪展示方案

保持旧有限 pipeline，先完整 restore。比较同前置条件的真实 native 路径，正确性验证
失败的样本不计入成功分布。小样本只作可行性筛选，最终 native 每条件 ≥100 次。
慢额度 30 s、扫描 >120 s 下旧有效数据仍可显示。没有cache但有DB另测只读bootstrap。
E1覆盖首次day优先与有库今天查询，不能只测已保存视图。首批发布不等worker全局终结。
E1失败先处理视图/主线程/恢复，不能增加worker掩盖问题；warm无改善就明报无改善。

### E2：热点、分段与资源预算

先定位usage原位12秒与backup差异，再测dirty session/derive；一次仅改变一个机制，
比较相同delta的全周期与最新数据帧。已提交today只读快不说明新数据入库快。
之后有必要才测1/2/4/8workers、chunk与短事务；相同schema/parser和DB起点。
报 wall/CPU/bytes/rows/transaction/queue/GC/RSS，复用已通过且内容未变的正确性证据。
至少3对pilot决定值不值得继续，最终heavy≥20样本；不以单阶段微基准替代整周期收益。

### E3：是否需要额外常驻或多进程

记录实际 spawn/connect/identity/decode 开销占比，先比较现有有限worker与复用worker。
若进程开销小，拒绝把常驻作为速度必选项。OTel连续listener可独立论证其必要性。
多进程按worker/IPC/collector聚合CPU与RSS；达不到比较门就保留单进程。

### E4：来源等价、事件与 OTel

对partition/delta与full oracle逐逻辑行/聚合比较；覆盖late事件、跨天、重写、移除、版本重建。
同一JSONL追加前后测真实popover新值，而非只读HTTP快照。文件watcher遗漏必须失败或单列。
OTel单测producer导出→durable ingress→exact reconciliation→新数据帧；off不影响E1。
未知身份capture_only，不拿相似token/time/model去重，不声称历史未导出部分已补齐。

## 当前完成边界

用户已明确接受首次加载/历史对照的口径，并认可其中可见的性能收益。
这支持将已验证的空库当天优先、已有库当天读取和保存视图恢复带入综合设计评审；
不等于原生首帧、新记录实时入库、资源预算或未实施架构已通过。
完整评审批次、依赖顺序与各文档carrier由[tasks.md](tasks.md#综合设计评审范围)声明。

设计文档与共享原型不构成产品性能保证。只有本文件明确列出的对照和正确性范围可作结论。
原生实验也不替代全部 P01–P11、最终 Release、资源/跨天/升级/CLI/OTel/独立评审门。
没有测出的收益保持未验证；没有改善的场景直接报告，不追加不相关优化。
