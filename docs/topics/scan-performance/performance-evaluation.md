---
status: active
created: 2026-10-09
updated: 2026-10-22
---

# 先行评估：哪些等待能被减少

## Task 3 P07 最终接受依据 — 2026-10-22

按用户明确批准的[最终验收修订](requirements.md#task-3-p07-用户批准的最终验收修订--2026-10-22)，
本次取消代表性完整事务分布的强制验证，并接受原四包race失败为Task3验收例外。
50ms p95/250ms max及256MiB工程预算保留且未验证；不报告正式量化达标。
原391 PASS/1 FAIL、根因unresolved、历史事务max失败和无效RSS trace均保留。
不继续捕获或实验；这不是修复race、全race通过或一次新的独立实现评审。

继续要求完整事务边界、原子性、durability和自动更新/full oracle正确性。
已核对保留的非计时真实fsync追加19-token观察：自动更新、publication/App Group推进、
完整wire-v1 full oracle相等且双方partial=false，含sessions、非可加统计与All。
对原复合criterion的not_verified观察不改写，只按新条件对其已通过的功能子集做有据复用。
其后正常产品代码仅有已独立评审的同事务inventory批量写入，逐行内容、revision、head及
回滚回归和现有full Go结果继续复用。本次仅改验收条件与记录，不新增性能主张。
原criterion、原gate及失败保存在[决定审计](evidence/task3-p07-acceptance-revision-20261022.json)。

<a id="task-3-p07-rss-验证项取消--2026-10-22"></a>

## Task 3 P07 RSS 验证项取消（历史范围决定）— 2026-10-22

按[用户修订的范围](requirements.md#task-3-p07-rss-验证项取消--2026-10-22)，
本次不再要求完整进程树 RSS 的量化验证，也不继续花时间探测替代通道。
用户本机运行 `sudo /usr/sbin/dtrace -l -n 'proc:::create'` 后返回
`dtrace: failed to match proc:::create: System Integrity Protection is on`。
该权限前置已实证；不把稳定枚举、匹配 ledger 或可见子进程退出当完整 fork 证明。
256 MiB 工程预算未改，但其验证项已取消，不能声称观测达标；历史无效 RSS trace 保留。
本节优先于下文关于 Task 3 RSS 必需验收的历史表述，其他任务独立资源门不变。

当时完整事务仍要求 p95 ≤50 ms、max ≤250 ms 的验证，后续按上节用户最终决定修订；
保留完整边界和原子性。既有 core 384.921506 ms/session 285.331956 ms 失败仍属于
原候选；新实现只在原事务内批量 INSERT，不拆提交或放宽预算。
本轮仅用隔离合成数据校准和验证修复，不重新捕获真实输入；代表性新候选分布未采集。
最终代码、逐笔时钟及验证结果见 Task 3 当前交接，不将合成结果称为代表性性能接受。

## Task 2 最终接受决定 — 2026-10-20

按[用户批准的最终验收标准](requirements.md#task-2-用户批准的最终验收标准--2026-10-20)，
本次Task2的首击延迟、启动恢复剩余范围及完整E1矩阵不再作为完成阻断条件。
100次同cohort原生首击完整采集，未因尾部耗时早停：p95=536.842034ms、
p99=623.638814ms、最大797.622640ms；100次超过300ms，3次超过600ms；
100次PID、有效首帧与清理检查均通过。原数值门的失败结论保持真实。

本次接受基于用户明确的风险接受和标准修订，保留既有机制/正确性/回归/独立审查PASS。
不再继续Task2性能优化、压力补采或新测量工具；本节优先于下文历史实验计划中仍要求
补齐这些项目的表述。其他任务和后续发布要求不受影响。
详见[最终接受审计](evidence/task2-acceptance-waiver-20261020.json)。

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

## 点击计时停止决定 — 2026-10-21

按用户决定停止无法精确测量的点击耗时工作；当前标准以
[要求修订](requirements.md#点击计时要求撤销--2026-10-21) 为准。
P01、P02 的点击子区间和 P07/P09 的点击分位依赖不再是必需门，不再要求百次
点击或切换采样。现有打开/关闭、数据正确、可交互及重建期间前台可用仍保留。

本轮与点击入口直接相关的执行为 1 次独立 App 试样及 3 次 hosted XCTest；
前两次 XCTest 失败，最后一次采用仓库既有自有窗口锚点后通过，但它只证明合成
测试中的回调和旧数值绘制，不能支持真实图标点击耗时。所引历史代表性记录另有
2 次原生准备失败。构建和纯 readiness 判定测试不计入这 6 次相关尝试；本轮没有
新的正式独立评审。失败结果不删除，也不通过更多变体替换。

下列计时定义和历史实验保留供审计，不再据此触发点击计时开发或补采。
用户进一步选择 P10 一并撤掉计时，只保留自动更新、publication 和 full oracle
正确性。后续不再按原 2 秒分位或原生绘制时钟补采；事务/RSS 仍独立判定。

同 cohort 的一次无点击正确性试样已完成：正常隔离 Debug App 自动建立完整基线，
对既有测试来源追加完整 19-token 记录并 fsync，没有显式 refresh、模型赋值或通知注入。
随后 publication 自动前进，App Group 数值一致；按实际 publication 的
America/New_York 时区，增量与同输入空库 full oracle 的完整 wire-v1 输出一致，
两边 partial=false。比较包含非可加统计和 All，仅忽略 generated_at/next_refresh_at。
此试样单列，不计入性能分布，也不证明绘制耗时或正式性能通过。
记录见 [有界补充结果](evidence/incremental-update-bounded-result-20261021.json)。

## “首屏”的确切含义

| 场景 | 前置状态和起点 | 终点 | 前后比较 |
| --- | --- | --- | --- |
| 日常点击 | App 已运行，双方均有同一份有效内存数据；原生 icon action | 真实 popover 显示选定 scope/window 的有效数字与日期，原生绘制完成，控件可用 | 当前内存展示 vs 候选内存展示；不能让基线先 scan |
| 启动恢复 | 双方内存为空、同一已提交数据库；完整 OS launch 和 App-ready 后数据准备分开记录 | 恢复/查询所得正确数据完成原生绘制 | 当前首次刷新 vs 完整缓存恢复；同时报告缓存是否已存在、年龄及窗口 |
| 自动更新 | 一条已知 JSONL 完整记录写入隔离来源，不显式刷新 | 自动推进 publication，完整统计与同输入 full oracle 一致 | 同 delta 与数据起点；仅功能核对，不采 P10 绘制耗时；OTel 的 P05 独立保留 |

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
失败的样本不计入成功分布。保留量化门的 native 条件最终每条件 ≥100 次；
P04/P11按2026-10-19用户决定接受已有有界证据，不再强制百次补测。
慢额度 30 s、扫描 >120 s 下旧有效数据仍可显示。没有cache但有DB另测只读bootstrap。
E1覆盖首次day优先与有库今天查询，不能只测已保存视图。首批发布不等worker全局终结。
E1失败先处理视图/主线程/恢复，不能增加worker掩盖问题；warm无改善就明报无改善。

#### 人工点击预检 — 2026-10-13

用户已批准用物理鼠标单击隔离测试 App，不再把自动 CGEvent 授权作为测量入口的必要前置。
`scripts/scan-performance-serving.py --click-dispatch manual` 只修改私有导出的探针；
菜单栏 `TEST A` / `TEST B` 分别标识基线和候选，保持正常按钮 target/action 和真实 NSPopover。
等待操作员的时段独立计量，点击区间从按钮收到真实鼠标 action 的入口到有效数值 bitmap
draw/readback 完成；这个起点不冒充物理 mouse-down，终点不冒充 compositor/photon。
等待指示在点击前写入，OCR 校验在帧计时结束后执行，原始图像和识别文本仍只留在内存。
计时前分别确认 A/B 的测试图标对操作员可见；菜单栏管理器可能分别收起不同私有 App。
操作员未见图标与产品耗时分开诊断，不能把等待操作超时直接归因为操作员未点击。
已完成的 arm 不要求重做：`--arms B --reuse-seed-from-report <prior-report>` 可作单臂续接，
但必须同时复核冻结输入、App/probe 身份、环境、点击控制与两份起始数据库的精确哈希。
任一漂移或原 seed 缺失即拒绝复用；单臂报告本身不冒充完整交错 A/B。

首次点击时数据是否已准备必须单列。人工等待期间引擎可能已经完成准备，所以含人工等待的
`launchToDataNanoseconds` 和整个进程时长不能用于判定 P02/P04/P11 启动预算，也不能通过减去
等待时间来伪造无等待启动。无人工等待的模型/启动准备和相应首批数据条件仍须另证。
丢失点击、操作员超时、缺少数值帧均记为失败，不自动注入、不重试成首次成功、不填零延迟。

先每个关键条件人工 5–10 次作测量入口和可行性预检，报告逐次值、中位数、最大值及失败数。
这不替代最终 native 每条件 ≥100 次，也不据小样本声称 p95/p99 已通过。
P01/P02/P04/P11 的用户批准 ×1.5 门槛由 [需求](requirements.md#数值验收提案) 统一维护；
慢额度/超长扫描、默认额度/全部面板、输入冻结、正确性及原始 A/B 身份要求不变。

#### 可复现 E1 条件调度与独立时钟 — 2026-10-14

用户批准 Task 2 验收工具收尾。`scripts/scan-performance-serving.py --matrix` 与
`scripts/scan_performance_serving_matrix.py` 分开处理 warm、restore、
committed-today-no-cache、first-import、rebuilding、slow-quota，不把单一无缓存条件
或同一进程的五个面板合并成完整矩阵/一百次首次点击。

- 人工首击在同一进程中等待图标可见，最长 600s；可见确认以请求 instance 绑定，
  然后才启用原操作等待预算。直接真实点击也可立即接受，无重启、自动注入或重试。
  `--confirm-visible --root <本轮私有根>` 只确认当前请求，不开始另一轮实验。
- `--measurement model` 是无需人工操作的模型准备预检。独立 main-actor 观察从进程入口、
  AppKit didFinishLaunching 入口到首个可用用量模型，10ms 轮询成本包含在观察内；
  它不打开窗口、不点击、不用总 launch 减去人工等待，也不叫首个有效数值帧。
  P02 的模型子路径可独立报告；P04/P11 的原生绘制有界证据按下节用户决定接受，不强制补测。
- `--measurement startup-frame` 是单列的就绪态自动展示控制：独立等待真实模型后调用
  controller 原 toggle 路径，并按既有 baseline 的 show 后 activation 顺序捕获可见
  NSPopover 数值 bitmap，终点先于 OCR。提前自动 show 的不稳定观察保留，不重试重标；
  就绪态控制不是对其根因已经修复的声明，也不覆盖“加载中立即点击”的行为。
  进程入口/App-ready 到帧不含人工等待；它不是鼠标事件、真实 icon action 或 P01 样本，
  不据此绕过 warm/压力条件的图标输入要求。每次控制会短暂展示隔离测试 popover。
- `--first-click-only` 每次真实 launch 只收一个首次 action/frame；后续面板不得混入首次样本。
  P03 独立验收已于 2026-10-19 撤销，不再要求另建切换样本集。各 arm/condition 分组，失败、重试、无效帧和 model-only 不填入
  首击分布。每组不足 100 个有效首次帧不输出 p95/p99，不自发宣布 E1 VERIFIED。
- 恢复缓存由真实 candidate finite bootstrap 产生；每组还原准确起始数据库/缓存，
  已接受 worker 退出之前不重置私有状态。A/B 的 schema/恢复能力差异不能被默默
  绕过；旧 A 不能读取相同缓存起点时保留失败，不从候选单臂计算 A/B 加速比。
  复制 state 会改变绑定的目录/DB inode，因此最后一次复制之后必须在原位重新由
  真实 producer 生成缓存，按 App 使用的根拼写读取并核对 publication，而非改写 header。
  私有 Swift runner 的 state 根与 worker 同为 canonical 路径，HOME/source 路径不变，
  避免 `/tmp`/`/private/tmp` 别名把 restore 暗中变成只读 today fallback；不改产品源码。
  普通 committed-today-no-cache 使用已提交的当前 candidate schema，仅去除保存文件；
  `upgrade-no-cache` 是额外的旧库升级诊断，不混入普通 P04。warm 从共同起始库完成
  非计时准备后确认 usage/session 内存数据均可用，再请求首次物理点击。
- 慢额度是私有探针可取消的 30s 依赖延迟，不冒充真实网络。首击是否发生在延迟
  未结束时及实际延迟长度单列。重建使用私有真实 `usage rebuild`，不是 sleep；
  CLI 整体 wall 与实际 processing/queue 不混称，未确认所需压力即不计通过。
- 可将已批准冻结 JSONL 克隆到全新私有根，不重采 live HOME、不改变旧捕获。
  如旧捕获已无当前日事件，`--today-sentinel` 明示添加一条可核对的合成当天来源，
  单列其 bytes/hash/mtime。该受控今天条件不推广为更大真实当天输入的 P11 保证。

重现需要操作员批准准备的冻结输入；私有语料与构建目录不是持久证据引用。新结果保留
源码/探针/四项 artifact 身份及脱敏事实到本 topic evidence；工具测试/构建和模型预检
本身不完成 Task 2、设计补充复核或独立实现评审。
首轮 15 次模型预检与旧库/克隆口径更正见
[模型预检](evidence/serving-matrix-model-pilot-20261014.json)；当前库无缓存及启动绘制另作新观察。
该首轮 restore 标签仅表示存在保存文件，未证实缓存命中；不能拿它当真实磁盘恢复的成绩。

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

### R7 首帧捕获与准备成本归属

2026-10-16 的 R7 修复保留正常 status-item action 起点及
AppKit layout/display/cacheDisplay 数值 bitmap 终点。若 action 时已有有效用量数据，
探针在原 toggle/show/窗口激活返回后立即保留首个可用 bitmap；OCR 仍在计时终点之后，
并与捕获时的 hero 绑定。未就绪的点击仍走既有数据就绪等待路径，不能套用 warm 时钟。
记录 `firstFrameCaptureOrigin` 区分这两条路径，另报
`probeResumeAfterActionNanoseconds`；探针 Task 稍后被调度的等待不再冒充首个帧的绘制成本。
该修正不把 action 入口改成物理 mouse-down，也不把 bitmap 改称 compositor/photon。
历史分布保持原测量口径，不倒减等待、拼接样本或重标旧 A/B 成绩；新候选须独立复验。

产品的初始准备成本继续计入独立 `popoverPreparation`：在未显示窗口中准备同一 hosting
层级及显示链路，首次点击仍由原 NSPopover 显示并接管该视图。准备窗口不 order front、
不请求焦点、不启动 helper；显示链路不加入 run loop。首次显示后关闭准备窗口，控制器
销毁时释放剩余准备资源。准备工作是实际启动成本，不能据此宣称整体启动更快。

## P03 测量工作退役 — 2026-10-19

依据 [需求中的用户决定](requirements.md#p03-独立验收项撤销--2026-10-19)，
停止 P03 专用采集器开发/修复、权限诊断、精密计时、轻量体验验收及百次采样。
现有采集脚本和历史报告只作历史诊断材料，不再作为本主题完成前必须修复或运行的入口；
本次不删除脚本或改产品代码。其他 native 条件的样本和性能门不受影响。
callback-to-bitmap 的历史数值不升级为产品性能结果；无新的性能 PASS。
保留展示路径无 helper/扫描/OTel 等待的架构约束，按实际改动或观察到的回归选择检查。

## P04/P11 有界证据接受 — 2026-10-19

按 [用户批准的启动验收范围](requirements.md#p04p11-有界启动验收--2026-10-19)，
接受已留存的有库无缓存5次原生位图帧与首次导入有界样本，不继续补百次或开发测量工具。
本节优先于历史“六条件各百次”“P04/P11原生出口仍须另证”的未完成表述。
不修改原报告、原失败、原二进制身份和采集方法；App-ready模型、位图帧、真实首击
及系统合成呈现仍严格区分，不计算或声称小样本p95通过。
历史方法说明仍用于解释旧记录，P01/P02及压力等未调整条件不受此例外影响。
