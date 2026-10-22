---
status: active
topic: scan-performance
subject: incremental-projections
---

# incremental-projections 评审记录

## Round 1 — 2026-10-20

## 📋 Task 3 增量投影独立评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SP-IP-R1-F1 — P2 — 分批扫描移动源时漏报可见会话变更。**

位置：[session.go](../../../../internal/session/session.go#L307) 和
[affected_view.go](../../../../internal/session/affected_view.go#L124)。

- 行为风险：活动与归档源共有逻辑会话时，把当前可见源移动到归档、使另一归档源成为 winner，`Batch=true` 返回 `Documents=0`，完整视图差分却是 `Documents=1`。scanruntime 的首批路径确实调用该模式并累加这个计数，因此可能向扫描结果报告零文档变化。本轮没有发现或声称已存可见内容丢失。
- 证据：同一私有合成场景分别执行完整 inventory 与仅含移动后路径的 priority batch；两者的 before 为 active winner、after 为 archived alternative。完整 inventory 控制组报告 1/0（Documents/Removed），等于 oracle；batch 报告 0/0，oracle 为 1/0。Go overlay 独立复现的控制子例 PASS、batch 子例 FAIL。
- 原因：`fullSet=false` 排除未出现在 batch 的旧路径；随后 `prepareSourceUpdate` 才按 inode 识别 rename 并迁移旧路径。`oldKeys` 因而遗漏旧 owner，`affectedViewChanges` 把该会话当作新引入并从已经迁移后的“未变源”重建 before，读到的已是新 winner。
- 处置：OPEN，归属本 Task；不是验收豁免或外部跟踪项。

💡 有界修复：在收集受影响键和 before 之前将 identity 对应的 moved-from 源纳入集合；或在 batch 检出 rename 时使用完整 before/after oracle。增加覆盖 priority batch 中 owner 改变的回归，保留 append/rewrite/remove 的既有优化与正确性。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

完整 inventory 控制组与 oracle 相符；复现中的最终存储视图也正确。实现保留全量 oracle 与明确的 native 验收边界，使差分与性能结论可分别核对。

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；ContentState `scan-incremental-state-final-de3852a42e5e521e`；scoped fingerprint `de3852a42e5e521e8a30aa0a1b6f0ef0c1509661f85031ab6ff2adebb0bfd792`。原 ContentState 的 23 项 subject digest 全部匹配，Go/macOS 与交接记录一致；评审和状态行政记录不属于该摘要范围。
- Reviewer：Codex，冷上下文独立 reviewer；不是本候选的实现者，无子代理。
- Method：契约/源码路径检查、独立隔离 Go overlay 差分；产品、仓库测试和配置只读。CodeGraph 在当前工作区查询；未能定位新增 affected-view 文件时使用限定源码读取。图关系不作为运行时证据。
- Scope：Task 3 usage partition invalidation、session 受影响视图和 schema 兼容路径的有界审查。发现决定性阻断后停止扩大验证；不是剩余实现或原生性能的整体通过声明。
- Evidence：`scripts/run-go-test.sh -overlay=<fresh overlay JSON> ./internal/session -run '^TestReviewBatchRenameCountsAgainstFullView$'` 返回 FAIL；完整 inventory 子例 PASS、priority-batch 子例 FAIL。最小源代码、输入构造、精确结果与修复边界见[持久复现记录](../evidence/incremental-review-r1-20261020.json)，可用新的临时 overlay 重现，不依赖已删除日志。
- 复用边界：先前 full Go、相关 race/vet、双架构构建、native 302/1 skip/0 fail 和 E2 20 对结果保留为原范围事实；本轮不重复它们，不将其视为新发现通过。最初精确门为 NOT_VERIFIED（3/4 covered）；P10 原生绘制与 P07 完整资源/事务缺证不因此消失。
- Completion gate：FAILED
- 最终门禁：新增一项 `partition-equivalence` fail，绑定原精确 ContentState；Neo4j 已确认 1 个 evidence 节点和 2 条关系，固定 gate 模板返回 FAILED。此前 pass 观察保留，新的反例不改写旧观察；`update-latency` 仍缺证。完整 envelope 见[本轮门禁](../evidence/incremental-review-r1-gate-20261020.json)。
- 结论：存在一项可执行的 P2 正确性发现，故本轮 FAIL；Task 3 返回同一任务修复，Review 不勾选，无提交、推送或安装。

修复：scan-performance / incremental-projections / SP-IP-R1-F1

### SP-IP-R1-F1 修复附记 — 2026-10-20

- 状态：repaired in candidate，待独立复评；Round 1 FAIL 与原内容 FAILED 门禁保持历史结论。
- 修复：`scanMode` 在现有 registered-source 查询中收集 identity→旧路径；新路径按 identity 将 moved-from 路径纳入 changedPaths，再采集 oldKeys/before。priority batch 不再从迁移后的替代源重建错误 before。超过 128 路径/键仍用完整 oracle；append/rewrite/remove 路径保持原语义。
- 回归：新增 `TestAffectedSessionBatchRenameCountsAgainstFullView`，分别覆盖完整 inventory 与 priority batch。修复前控制组 PASS、batch 返回 0/0 而 oracle 为 1/0；修复后两组均要求 owner replacement 的完整差分 1/0。
- 新候选：`scan-incremental-repair-r1-97321292d5b8d565`；只有 `internal/session` 内容摘要变化，其余 22 项与原 reviewed state 相同。修复证据见 `evidence/incremental-repair-r1-20261020.json`。
- 本附记不是独立 Review PASS；Task 3 Review 保持未勾选，P10/P07 未验证范围保持开放。
- 验证完成：session/scanruntime 全包、全量 Go、两包 race（`-p 1`）、两包 vet 与 Darwin arm64/amd64 构建均 PASS；新增回归包含完整 inventory 控制。原生代码未变，复用原范围检查，不重跑原生或旧 E2 采样。
- 修复候选精确门查询结果为 NOT_VERIFIED（3/4 covered，仅 `update-latency` 缺证；无当前 fail/失效/未决影响）。5 个节点和 10 条关系已按模板写入并精确回读，见 `evidence/incremental-repair-r1-gate-20261020.json`。本附记不改变 Round 1 的历史评审结论或其原状态门禁。


## Round 2 — 2026-10-20

## 📋 Task 3 增量投影复评

📊 总体评分：8/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。SP-IP-R1-F1 已关闭，本轮没有新增阻断发现。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

**SP-IP-R1-F1 — CLOSED。** [session.go](../../../../internal/session/session.go#L323)
在采集 oldKeys/before 前按新路径的 identity 纳入 moved-from 路径，修复了 priority batch
漏掉旧 owner 的原因；超过 128 路径/键的 full-oracle fallback 保留。
新增回归显式确认真实 owner replacement 和 1/0 差分，不会因场景退化为无变更而假通过。

独立重放原 Round 1 的同一 reviewer overlay：完整 inventory 和 priority batch 两组
现在均报告 `Documents=1/Removed=0`，与完整可见视图 oracle 相等。两组均 PASS，
而原状态仅 batch 失败；这关闭原发现，未修改旧失败记录。

### 📝 总结

- Finding disposition：SP-IP-R1-F1 从 OPEN 经 repaired in candidate，本轮独立核验为 CLOSED；没有其他既有发现或新增发现。
- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；ContentState `scan-incremental-repair-r1-97321292d5b8d565`；scoped fingerprint `97321292d5b8d56591766b919c935a09a6306aa565d512c88c28d0d3f9f454f2`。23 项 subject digest 全部匹配，只有 internal/session 相比 Round 1 改变。Go 1.27.1 darwin/amd64、macOS 26.7.2 build 25G325 与修复输入一致。
- Reviewer：Codex，独立于修复 producer；复用 Round 1 reviewer 上下文，不把本轮称为新的冷上下文整体审查；无子代理。
- Method / Scope：同一任务既有发现及其修复影响，当前工作区 CodeGraph 同步后定位 scanMode，再检查新增回归及独立重放原反例。产品、仓库测试、配置只读；临时 overlay 不改仓库测试。
- Evidence：`scripts/run-go-test.sh -overlay=<fresh overlay JSON> ./internal/session -run '^TestReviewBatchRenameCountsAgainstFullView$'` 返回 PASS；两个子例报告与 oracle 同为 1/0。源码、场景、方法及精确结果见[本轮持久记录](../evidence/incremental-rereview-r2-20261020.json)，原反例源码仍在 Round 1 的持久 JSON 中；不引用临时日志。
- Evidence reuse：修复候选 full Go、session/scanruntime、相关 race/vet 与双架构构建沿用原精确状态证据，不因阶段变化重跑。其余 22 项未变，原生 wire/302 项检查只在原适用范围复用。原 E2 性能统计保留在父状态，不声明已在修复候选重测或达标。
- Completion gate：NOT_VERIFIED
- 本轮最终门禁：新增独立 pass evidence 的 1 个节点及 2 条关系已精确回读，固定模板再次确认 NOT_VERIFIED，3/4 required covered；没有当前 fail、失效或未决影响。完整 envelope 见[Round 2 门禁](../evidence/incremental-rereview-r2-gate-20261020.json)。
- Gate boundary：四项必需标准中三项技术准则已有当前目标证据，`update-latency` 仍缺 P10 原生 source-write→新值绘制和 P07 聚合资源/写事务 p95/max。复评 PASS 不等于 Task 3 验收完成；保留 in_review，不进入 awaiting_commit，不启动下一 Task。Topic 仍有 Tasks 4–7，无 containing-unit 完成边界。
- 结论：本轮所有既有发现均已关闭，复评 PASS；Task 3 Review 可勾选，完成门继续开放。没有提交、推送、安装或真实库迁移。

Task checkpoint：ad-scan-performance-incremental-projections；content_state=97321292d5b8d56591766b919c935a09a6306aa565d512c88c28d0d3f9f454f2；gate=NOT_VERIFIED。
提交建议：门禁 VERIFIED 后，补齐 update-latency 的 P10/P07 证据并明确 Task 2/3 累积未提交边界，再另行授权提交 Task 3 实现、回归、评审及证据。
推送建议：门禁 VERIFIED 后，在同一 P10/P07 前置闭环、提交范围确认、独立交付授权和提交对象/SSH 签名检查完成后，推送至 origin/feature/scan-performance；本轮不执行。

补齐验收：scan-performance / incremental-projections / update-latency（P10 原生新值绘制；P07 聚合资源与写事务 p95/max）

## Round 3 — 2026-10-21

## 📋 Task 3 P07 测量仪表评审

📊 总体评分：5/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SP-IP-R3-F1 — P1 — P07 事务分布遗漏增量投影路径中的隐式提交写入。**

位置：[overlay 事务选择](../../../../scripts/p07-overlay/main.go#L82)、[仪表边界说明](../../../../scripts/p07measure/README.md#L44)、[BeginUsagePartition 调用](../../../../internal/usage/projected_events.go#L109)、[PublishUsagePartition 调用](../../../../internal/usage/projected_events.go#L146)、[分区写入实现](../../../../internal/store/usage_partitions.go#L130)。

- 行为风险：overlay 只包装显式 Begin/BeginTx、Commit/Rollback 和 pinned Conn 的原始事务；summary 也只接受这些显式配对记录。Task 3 增量投影计算会调用 `BeginUsagePartition` 的 `DB.QueryRowContext(UPDATE ... RETURNING).Scan` 和 `PublishUsagePartition` 的 `DB.ExecContext(UPDATE ...)`。它们各自产生隐式 autocommit 写事务，却不会进入逐笔写分布。遗漏慢写可能让 P07 写事务 p95/max 看起来达标，导致验收错误通过。
- 证据：类型选择器 [main.go](../../../../scripts/p07-overlay/main.go#L80) 对 `*sql.DB` 仅匹配 `Begin`、`BeginTx`、`Conn`；README 明确声明 implicit autocommit SQL 不在分布中。当前产品路径 [projected_events.go](../../../../internal/usage/projected_events.go#L109) 和 [projected_events.go](../../../../internal/usage/projected_events.go#L146) 分别调用 [usage_partitions.go](../../../../internal/store/usage_partitions.go#L131) 的 `DB.QueryRowContext(...).Scan` 与 [usage_partitions.go](../../../../internal/store/usage_partitions.go#L139) 的 `DB.ExecContext`。这些调用不经过已包装的显式事务。当前仪表证据中的 19 笔显式合成事务通过，不覆盖这两个写边界。
- 有界修复：只扩展测量 overlay/collector，使上述 autocommit 写以完整单笔边界进入分布；`UPDATE ... RETURNING` 必须计到 `Scan` 完成，不能只计到 `QueryRowContext` 返回。对显式与隐式事务去重，保证未知/未观测写使 summary 不完整；用合成 SQLite 回归验证每种边界只计一次、计时定义正确且不改变产品 SQL/事务语义。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

仪表与产品源码保持隔离，通过临时 Go overlay 工作；事务日志不收集 SQL、参数或路径。唯一 PID 的 RSS 聚合集合、reparenting/PID reuse 处理、50 ms 固定间隔、采样缺口和观测开销均明确记录。当前工具哈希与实现证据中的独立身份逐项匹配。

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；仪表 ContentState `scan-p07-instrumentation-state-2b01c3f035473fff`，SHA-256 `2b01c3f035473fffb38e513e579fb59f4a3e8dabe88944f652e06baa201c3ae2`。7 个工具源/测试哈希与持久实施证据全部匹配；23 项产品输入仍绑定产品 ContentState `scan-incremental-repair-r1-97321292d5b8d565`，没有在本轮改写或重绑。
- Reviewer：Codex，当前任务为独立 Review；本轮未实现该仪表。Task 3 产品 Round 2 PASS 是另一内容状态的历史结论，继续保留。
- Method / Scope：通过 CodeGraph 定位事务写路径与调用关系，检查 overlay、采样器、摘要和合成验证记录；产品代码、测试与配置只读。仪表测试/race、Python 合成树测试、overlay 夹具、vet 和双架构构建按匹配哈希复用实施证据；发现决定性覆盖缺口后停止扩大验证。
- Evidence：`p07-instrumentation-20261020.json` 记录的 7 项工具身份逐项与当前 SHA-256 匹配。记录的合成显式事务汇总为 19 笔、无 rejected/unmatched，Go/race/Python/overlay/vet/Darwin 双架构构建通过；这些结果不包含 implicit autocommit 写，也不是 P07 性能样本。直接代码路径见上列位置。
- Completion gate：FAILED。Neo4j 对产品 ContentState `scan-incremental-repair-r1-97321292d5b8d565` 的精确门查询仍为 4 项中 3 项 PASS、`update-latency` FAILED；P10 失败事实不因本轮仪表 Review 改变，P07 有效性能样本仍为 0。
- 结论：P07 仪表未计入适用重建路径中的隐式写事务，故不能作为完整 P07 写事务分布的测量依据，本轮 FAIL。Round 2 产品 Review PASS 保留；Task 3 不完成、不进入 awaiting_commit，Review 保持未勾选。没有 P07 性能采样、真实数据访问、产品变更或 Git 交付。

修复：scan-performance / incremental-projections / SP-IP-R3-F1

### SP-IP-R3-F1 修复附记 — 2026-10-21

- Disposition：SP-IP-R3-F1 -> repaired in candidate，等待独立复评；保留 Round 3 FAIL，未自行宣布 CLOSED 或 Review PASS。
- 候选身份：`scan-p07-r3-repair-state-62eb7acb7f74a58d`，SHA-256 `62eb7acb7f74a58d7ba41415eefcf4a3b32466cf625159a8085b2d3e8be9f758`。10 项仪表源/测试/说明及依赖身份见[修复证据](../evidence/p07-r3-repair-20261021.json)。23 项产品输入逐项匹配 `scan-incremental-repair-r1-97321292d5b8d565`，没有修改产品 SQL、事务选项、超时、错误/重试语义或验收标准。
- 修复前复现：将保留的合成 store 回归模板注入私有 overlay，使用全新临时库和单个合成分区，实际调用 BeginUsagePartition、PublishUsagePartition；原仪表记录 0 笔，期望 2 笔，定向测试 FAIL。修复后重放同一模板，两个调用各产生一笔有效 autocommit 记录，发布 revision/payload 的原值校验通过。模板与可复现命令见[工具说明](../../../../scripts/p07measure/README.md)；事实和身份已直接保留在修复证据，不依赖已清理的临时输出。
- 修复：overlay 覆盖 DB/Conn 单语句隐式写；QueryRow 保留原对象和惰性求值，直到原有 Scan 返回才封口。嵌套 QueryRow.Scan 的类型驱动改写只执行每个调用一次。观察到的 raw/Go 显式事务内语句不重复计数；未知 SQL、复合语句、缺少 Scan、延迟 prepared 写及未观测终点的多行写明确产生不完整记录。日志/摘要升级 v2，拒绝旧显式事务 v1，以免旧日志被解释为完整覆盖。
- 验证：仪表 Go 单测 16 PASS / 1 fixture-entry skip；定向 race 14 PASS / 1 fixture-entry skip，入口 skip 已由两次受控子进程覆盖。Python ProcessSet/summary 7 PASS；同一 store overlay 回归 PASS；vet 及 Darwin amd64/arm64 overlay 构建 PASS，二进制未启动。RSS 源前缀 SHA-256 与原证据相同，复用原生合成 RSS 机制证据，不重复原生或代表性性能采样。
- 开销：保留全部三对单语句文件日志校准；Exec 每笔增加约 0.042–0.061 ms，RETURNING→Scan 增加约 0.071–0.366 ms，较慢一对未排除。仅是合成仪表校准，不代表 P07 或可忽略开销。未知/未观测写仍须在未来采样前解决，不能从失败或缺口推导性能通过。
- 交接：Task 3 保持开放，Review 未勾选，修复完成后释放给独立复评。产品门保留 P10 失败；P07 attempted=0、valid=0，仍需新的数据授权与代表性冻结输入。本轮没有真实来源/数据库访问、Tasks 4–7、提交或推送。

## Round 4 — 2026-10-21

## 📋 Task 3 P07 测量仪表复评

📊 总体评分：4/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**SP-IP-R3-F1 — P1 — P07 事务分布遗漏增量投影路径中的隐式提交写入。**

- Disposition：CLOSED。仅对该 finding 的修复结论关闭；保留 Round 3 FAIL 的历史记录。
- 证据：修复候选 `scan-p07-r3-repair-state-62eb7acb7f74a58d`，SHA-256 `62eb7acb7f74a58d7ba41415eefcf4a3b32466cf625159a8085b2d3e8be9f758`。持久修复证据记录同一合成分区回归修复前 0 笔、修复后 `BeginUsagePartition` 与 `PublishUsagePartition` 各 1 笔有效 autocommit，且发布 revision/payload 保持不变；修复候选的工具身份及回归结果继续匹配该记录。此次复评没有重跑不受影响的 Go/race/vet/native 检查。

**SP-IP-R4-F1 — P1 — P07 overlay 将 serving snapshot 的只读 CTE 查询记为未知写入，使事务汇总无法完整。**

位置：[SQL 分类器](../../../../scripts/p07measure/autocommit.go#L13)、[QueryContext overlay 映射](../../../../scripts/p07-overlay/main.go#L89)、[session.List 查询](../../../../internal/session/session.go#L1588)、[desktop snapshot 构建](../../../../internal/desktop/desktop.go#L259)、[serving 发布路径](../../../../internal/scanruntime/serving.go#L238)。

- Disposition：NEW，未修复。
- 行为风险：`statementKind` 仅把 `SELECT`、`VALUES` 和 `EXPLAIN` 判为只读；`WITH` 不在白名单中，因此只读 `WITH … SELECT` 返回 `unknown`。`QuerySQL` 经 `statementStart` 将该查询记录成无效的 `unknown_statement`。产品的 `session.List` 使用 `WITH visible AS (…) SELECT …`；`desktop.Service.Build` 会调用 `loadSessions`，而 scanruntime 的 serving snapshot 发布会调用该构建路径。于是正常 P07 重建会把只读 session 查询混入 autocommit 写观察并令完整覆盖条件失败，不能取得可信的写事务分布。
- 证据：当前实现分别位于上述源码位置；`statementStart` 对 `unknown` 设置 `unknown_statement`，`queryRows` 将它写入事务观察；`session.List` 的首词为 `WITH`，且通过 `*sql.DB.QueryContext` 执行。R3 修复的合成分区测试只证明两个目标隐式写入被计时，不覆盖这条只读 CTE 查询。P07 性能采样仍为 0。
- 有界修复：只扩展测量工具对可判定只读 CTE 的分类，使 `WITH … SELECT` 不污染写事务汇总；对数据修改型 CTE、复合语句及仍有歧义的语句继续 fail closed。新增覆盖 session 查询形状的合成回归，并验证其不生成写事务样本且不降低未知/潜在写入的完整性保护；不修改产品 SQL、业务行为或验收标准。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

R3 的隐式 `Exec` 与 `QueryRow…Scan` 边界现已被修复并有同一合成输入的前后对照；测量工具仍与产品代码隔离，历史校准也明确标注为合成仪表开销而非 P07 性能样本。Round 2 产品 Review PASS 与 Round 3 仪表 FAIL 均保留各自内容范围。

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；仪表候选 `scan-p07-r3-repair-state-62eb7acb7f74a58d` / SHA-256 `62eb7acb7f74a58d7ba41415eefcf4a3b32466cf625159a8085b2d3e8be9f758`；23 项产品输入仍匹配 `scan-incremental-repair-r1-97321292d5b8d565`，未重绑产品 ContentState。
- Reviewer：Codex，独立复评；产品代码、测试及配置只读。方法：复核 R3 持久身份和合成回归证据，结合当前 overlay、分类器、session 列表及 serving snapshot 调用路径做定向源码检查；未重复 Round 2 Review，也未执行 P07 性能采样。
- Completion gate：FAILED。复用未变化产品 ContentState 的精确门结果：4 项必需准则中 3 项 PASS、`update-latency` FAILED。P10 历史失败仍有效；P07 attempted=0、valid=0。R3 仪表修复或本轮 Review 结论都不能让复合准则变成 VERIFIED。
- 状态同步：默认沙箱首次读取 Beads gate lock 返回 `operation not permitted`；按用户要求重试后，actor-qualified 查询确认现有任务存在，初始为 `in_review`、`round-2`、未分配。使用 `--if-status in_review --if-assignee ''` 守卫更新为 `in_progress`、`round-3`、仍未分配；交接评论已读回，ID `b3e449c4-f688-521e-8479-fc973b2e6b06`。`tasks.md` 已追加 Round 4 指针；CEv1 未变，产品门继续保持 FAILED（3/4），没有依据 Beads 状态改写验收证据，也没有创建重复任务。
- 结论：SP-IP-R3-F1 CLOSED；SP-IP-R4-F1 NEW 且阻断仪表完整性，故本轮 FAIL。Task 3 Review 仍未勾选、任务保持开放；没有 P07 性能样本、真实数据访问、产品修改、Tasks 4–7 工作或 Git 交付。

修复：scan-performance / incremental-projections / SP-IP-R4-F1
WORKFLOW_WORKSPACE: agent-deck.scan-performance

### SP-IP-R4-F1 修复附记 — 2026-10-21

- Disposition：SP-IP-R4-F1 -> repaired in candidate，正式独立复评待执行；保留 Round 4 FAIL 和 R3 finding 的 CLOSED 历史，不自行宣布新的正式 PASS/CLOSED。
- 候选身份：`scan-p07-r4-repair-state-f1a100f58f67f932`，SHA-256 `f1a100f58f67f932bfb83e8de503e4aa1c7f2647873818b068f19e683a403da1`。13 项工具源/测试/说明和 4 项依赖身份见[修复证据](../evidence/p07-r4-repair-20261021.json)。23 项产品输入仍匹配 `scan-incremental-repair-r1-97321292d5b8d565`；产品 SQL、业务行为、事务语义、超时和验收标准未变。
- 修复前复现：实际 session.List 的合成 overlay 夹具在执行只读查询时增加 367 字节事务日志，期望为 0，因此 FAIL。修复后同一夹具增加 0 字节，优先级赢家仍为 synthetic/active；夹具中真实的 core/session 各一笔合成写入继续完整，collector 返回 complete=true、rejected=0、unmatched=0。
- 修复：仅为 WITH 分支增加有界结构识别；核对全部 CTE 声明、查询体及最终 SELECT/VALUES，区分引号、注释、括号，支持 recursive、多重/嵌套 CTE、列名及 materialization hints。修改型根/查询体、复合语句、歧义/未闭合声明与超过 64 查询层的输入继续 unknown，不把 WITH 前缀整体放行。该识别器不是完整 SQLite 语法验证器。
- 验证：Go 普通/定向 race 各 17 PASS / 1 fixture-entry skip；两次受控子进程覆盖该入口。实际 session.List 和 R3 分区回归均 PASS，后者仍是两笔有效 autocommit，revision/payload 未变。原 Python collector 从实际夹具 trace 重算通过；vet 和 Darwin amd64/arm64 overlay 构建通过，二进制未启动。RSS、既有边界时钟及日志/摘要 schema 未改，不重复原生或代表性性能采样。
- 用户要求的独立预审：`/root/p07_r4_sol_precheck`，模型 `gpt-6.1-sol`、effort `ultra`、`fork_turns=none` 冷上下文、只读建议角色。该 subagent 提供 advisory PASS、无 findings；主代理直接核对其 20 个主体哈希、7 项原始测试结果、25 对 overlay 身份，并从其原始 fixture trace 重算 collector。详细派发、方法、限制及结论直接保留在修复证据内，不引用临时原始输出。本次预审不替代正式复评或 Task 验收。
- 交接：预审通过后交回正式复评，Task 3 保持开放且 Review 未勾选。产品门继续受 P10 历史失败约束；P07 attempted=0、valid=0，仍需新的数据授权与可用代表性冻结输入。没有真实数据访问、Tasks 4–7、提交或推送。

## Round 5 — 2026-10-21

## 📋 Task 3 P07 只读 CTE 测量仪表复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **SP-IP-R4-F1 — CLOSED。** [分类入口](../../../../scripts/p07measure/autocommit.go#L76) 仅在 [CTE 结构识别](../../../../scripts/p07measure/cte.go#L138) 确认完整声明、查询体及最终 SELECT/VALUES 后排除该读操作。修改型根/查询体、复合及歧义输入继续 unknown；[合成测试](../../../../scripts/p07measure/cte_test.go) 还验证实际 CTE UPDATE 语义不变且保留无效观察，不会被读分类掩盖。
- 实际 `session.List` overlay 回归通过：查询新增日志为 0 字节，优先级赢家及 model 保持为 `synthetic/active`。直接从本轮原始 fixture trace 调用现有 collector 重算，得到 core/session 各 1 笔有效写、explicit/autocommit 各 1 笔，`complete=true`、rejected=0、unmatched=0。
- **SP-IP-R3-F1 — CLOSED 保持。** 同轮实际分区回归仍记录恰好两笔有效 autocommit，revision/payload 不变。SP-IP-R1-F1 的 Round 2 CLOSED 与产品 Review PASS 继续保留，本轮没有机械重开。

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；仪表 ContentState `scan-p07-r4-repair-state-f1a100f58f67f932`，SHA-256 `f1a100f58f67f932bfb83e8de503e4aa1c7f2647873818b068f19e683a403da1`。13 项工具文件和 4 项依赖逐项匹配，23 项产品主体也逐项匹配 `scan-incremental-repair-r1-97321292d5b8d565`；没有改绑产品证据。
- Reviewer / Method：Codex，正式复评角色，未参与 R4 修复，无本轮委派；先前 Sol ultra 预审仅作历史材料。定向源码核查后，新建私有 HOME/TMPDIR 和 Go overlay，使用项目 wrapper 重放 5 项 CTE/完整性单测及 2 项实际产品路径合成回归，7 PASS、0 skip、0 fail；再从 session fixture trace 独立重算 collector。事实、命令、身份和限制见[本轮复评证据](../evidence/p07-r4-rereview-r5-20261021.json)。
- Evidence reuse：相同工具/依赖身份及 Go 1.27.1、macOS 26.7.2 环境下的修复阶段 race、vet、Darwin 双架构构建继续复用；RSS 与既有事务边界实现未变，不重跑原生或性能采样。识别器是保守的读根识别器，不是完整 SQLite 语法验证器；现有合成观察开销也不代表正式 P07 输入上的开销。
- Completion gate：FAILED。复用[修复阶段精确目标 Neo4j 回执](../evidence/p07-r4-repair-gate-20261021.json)，产品目标仍为 `scan-incremental-repair-r1-97321292d5b8d565`，4 项中 3 项 PASS、`update-latency` FAILED；本轮没有新的 gate 查询或 CEv1 性能证据写入。P10 历史 attempted=1、failed=1、有效延迟样本=0；P07 attempted=0、valid=0。P10 自动通知至原生新值绘制、P07 新授权的代表性冻结输入及完整资源/事务/前台/切换约束仍是完成门前置。
- Disposition：SP-IP-R4-F1 CLOSED；既有已关闭项未回归，没有新发现，正式复评 PASS。Task 3 Review 可勾选，Task 3 仍开放并保持 in_review，不进入 awaiting_commit 或 Tasks 4–7。没有真实数据访问、产品修改或 Git 交付。

Task checkpoint：ad-scan-performance-incremental-projections；reviewed content_state=f1a100f58f67f932bfb83e8de503e4aa1c7f2647873818b068f19e683a403da1；产品 gate target=scan-incremental-repair-r1-97321292d5b8d565；gate=FAILED。
提交建议：门禁 VERIFIED 后，P10 自动通知至原生新值绘制及 P07 新授权代表性输入上的资源/完整事务/前台/切换验收闭环，再明确 Task 2/3 累积范围并另行授权提交。
推送建议：门禁 VERIFIED 后，同一 P10/P07 前置已闭环、提交另获授权且提交对象/SSH 签名已核对时，再另行授权推送至 origin/feature/scan-performance。

补齐验收：scan-performance / incremental-projections / update-latency（P10 自动通知至原生新值绘制；P07 先明确新的代表性输入授权及冻结起点，再完成聚合 RSS、完整写事务和前台/切换约束验收）
WORKFLOW_WORKSPACE: agent-deck.scan-performance

## update-latency 修复附记 — 2026-10-21

修复者：Codex；范围为当前验收已记录的 P10 通知缺失、WAL 初始化计量和 RSS 结束边界。
本轮无委派。新候选等待独立复评；Round 2 产品 PASS、Round 5 仪表局部 PASS 及原失败诊断按原身份保留。

### 候选身份与影响

- 产品：`scan-ip-update-repair-product-45f7482757f82715`，SHA-256
  `45f7482757f8271567909e80d93980b3678ab2c3f97895ef51afa44f04e67c45`。
- 仪表：`scan-ip-update-repair-instrumentation-d9db8dbe33f327da`，SHA-256
  `d9db8dbe33f327da0b246ef6818139fb349bcf4c233a06044ee7641bda00f03d`。
- HEAD：`14234ec4c96a3376d1072147eeb6e42b8768381a`；两类身份各自保存 scoped subjects 和摘要配方，
  见[内容身份](../evidence/incremental-update-repair-content-states-20261021.json)。
- 五个 Go 产品目录及依赖摘要与原产品身份一致；原 partition/nonadditive/generation 技术证据经
  Change、三个 MAY_INVALIDATE/PRESERVES 评估和新目标 roll-up 显式复用。旧观察未改绑。
  新增 App 接线、coordinator trigger、时钟及相关测试使用新身份和本轮直接证据。

### 修复映射及实际产品证据

1. **P10 通知链路。** 生命周期绑定的 FSEvents source monitor 观察内建 Codex/Claude JSONL roots，
   将变化交给现有 `DesktopRefreshCoordinator` 的 `sourceChange` trigger。
   在途通知最多合并为一次追赶刷新；停止 App 时关闭 stream，沿用原取消规则。
   此处没有新增扫描协调者、服务或固定周期全量任务；Task 5 的其余能力未启动。
   Debug 合成入口以正常 LaunchServices 启动私有 App，确认激活及 popover 已打开，随后实际写入完整
   JSONL 并 fsync。自动更新过程中没有显式 refresh、snapshot/model 赋值或最终通知注入。

   最终合成单例：写入至新值绘制 **841.069733ms**；fsync **0.469201ms**；落盘至通知
   **11.999401ms**；通知至排队 **0.203006ms**；排队至处理 **0.025408ms**；处理至首次
   serving publication 安装 **614.862884ms**。后续 publication 安装至绘制 **130.075644ms**。
   publication 时钟标记 helper 原子提交后 envelope 在 App 的安装端点；不将其称为 SQLite commit 时间。
   扫描在途的实际可见窗口 layout/display 为 **45.480933ms**，旧/新 publication ID 及完整时钟均已保留。
   新值为 36 input tokens / 2 events；新旧数值位图在内存中完成 OCR 核验，位图和识别文本未保存。
   此单例及前台操作仅证明合成链路可行，不能建立 P01/P10 的代表性分布或 p95 保证。

   与相同最终来源、空库起点的 full oracle 比较，完整归一化 snapshot 相等，partial=false；
   SHA-256 `2a4412b2f47f819c9f565e9a7256f978625c54e2095c2ea6d009ef62447931cd`。
   仅删除 generated_at / next_refresh_at；保留 health、数组和其余字段，并在两边 scan.lock 均释放后比较。
   早先 pilot 的七处 health 锁状态差异、无效 fixture 和失焦试样均保留为诊断。

2. **P07 WAL 初始化。** 两处实际调用均位于 `internal/store/store.go`：`OpenSessions:45` 和
   `openAtExistingRoot:405` 的 `ExecContext`。它们可以修改数据库 journal mode，计为
   `read_only=false` 的 autocommit 初始化写，标记 `operation=journal_mode_wal`。
   区间从原接口调用前至返回后，包含连接池/锁等待；vendored modernc Exec 将 ROW 消费到 DONE，
   并在返回前 reset/finalize。未修改 SQL、事务、错误、重试或超时，也未豁免该操作。
   未知、复合、潜在写且终点未观察的 SQL 继续 fail closed。

   实际 scan/bootstrap/helper 路径共 **478** 笔终结、**75** 个 Go process logs；
   core/session WAL 初始化分别 **50/20** 笔，汇总 complete=true、rejected=0、unmatched=0。
   RSS 两个 Go 进程日志与屏障 roster 精确一致。分案例及逐条记录均持久保存；合成汇总中的 p95/max
   不作为 P07 成绩。原 18 条 WAL 拒绝仍在旧证据中，不重新标为有效。
   真文件 DB 的连接池占用回归验证了 WAL 模式修改及完整等待区间。

3. **RSS 开始/结束。** overlay 在真实 main 前、main/Exit 清理后设置开始/结束屏障。
   collector 只在完整固定 50ms 帧见到同一 PID/birth 后确认；root 在所有已登记 worker 结束被观察、
   最终帧和 summary 写完后才释放。首次 root-end 不再提前结束 detached worker 的窗口。
   根为实际 bootstrap helper，另含实际 detached worker、collector 及递归发现的辅助进程。
   已观察的普通非 root 退出与采集中途的拓扑/identity/resident 缺口分别处理；未知缺口仍使 trace 无效。

   最终 trace **35 帧 / 1,711.226244ms**，无不完整帧、无 missed tick；采样期间共见到 7 组唯一
   PID/birth。sampled aggregate maximum 为 **103,563,264 bytes**；collector CPU **129.366ms**，
   poll 总计 **125.396464ms**。root 与必要 worker 的开始/结束样本及最后一帧可直接审计。
   原 12 帧缺口诊断保留；本轮早结束的 1 帧缺口 trace 也保留，未删帧或重分类。

### 比例验证与观察开销

- Go 计量/overlay：20 tests PASS；对应 race、vet PASS；当前 amd64 helper 构建及 arm64 overlay cross-build PASS。
  通过项目 `scripts/run-go-test.sh`，GOCACHE 固定为 `/private/tmp/agent-deck-go-build`。
- Python meter：10 tests PASS，覆盖 root-end 先到时等待 worker，并验证 summary 写完前 root 不获释放。
- 原生：79 tests PASS（coordinator 35、scheduler 6、log policy 3、popover chrome 35），零失败。
  包括实际文件写入/停止后的通知、8 次通知合并为一次 follow-up、重复事件及失败后旧 publication 保留。
  使用 scheme 中的 `AgentDeckSharedTests` 与 `AgentDeckAppTests`，全部为隔离 HOME。
- 25 组 overlay 原始产品文件摘要与当前工作树匹配；嵌入与独立 helper 均为
  `a8d91b9ca917754d0eeac067238cc697d40a7e6e1ba3533be2fd20ea27331e8f`。
- 两组交替 plain/overlay 的真实空库 scan 开销观察含明显冷启动差异，不据此估计代表性开销。
  文件日志校准约增加 23–45 microseconds/write；RSS collector CPU/wall 已如实报告，未称可忽略。
  启停屏障等待留在合成进程窗口内，不进入 SQL 区间。

[完整修复事实](../evidence/incremental-update-repair-20261021.json)、
[完整 RSS trace](../evidence/incremental-update-repair-20261021-rss.jsonl)、
[本轮早结束失败 trace](../evidence/incremental-update-repair-20261021-rss-failed-window.jsonl)、
[完整合成事务记录](../evidence/incremental-update-repair-20261021-transactions.jsonl) 均为持久证据。

### 精确门禁与交接边界

已从明确的 `ai-tools/protocols/completion-evidence/v1` source checkout 取得 PROFILE、schema、
provider 与四个固定模板，通过现有 Neo4j MCP 写入本轮事实并完成 relation preflight。
新增节点 11/11、关系 21/21；MCP 返回 creation metadata，所有 ID 均新建，计数与批次一致。
固定 gate 对新产品目标查得 **NOT_VERIFIED（3/4）**，仅 `update-latency` 缺验收，
invalidated/unresolved 均为空。旧目标重新查询仍为 FAILED（3/4）；两者分存于
[门禁回执](../evidence/incremental-update-repair-gate-20261021.json)，未用旧 FAILED 代替新查询。

代表性 P07 attempted/valid 仍为 **0/0**；P10 正式分布尚未建立，历史 attempted=1、failed=1、valid=0
保持原义。本轮修复及比例验证交回同一任务的独立复评，Task 3 保持开放/in_review。
本附记不产生正式 Review PASS、Task 完成或交付授权。

### 后续代表性只读冻结授权申请

合成通道和计量完整性已成立。独立复评后，申请仅为本 Task 3 进行一次新的只读冻结，准确来源为：

- `/Users/jobshen/.codex/sessions/**/*.jsonl`
- `/Users/jobshen/.codex/archived_sessions/**/*.jsonl`
- `/Users/jobshen/.claude/projects/**/*.jsonl`
- `/Users/jobshen/.agentdeck/agentdeck.sqlite3` 的只读 SQLite backup
- `/Users/jobshen/.agentdeck/sessions.sqlite3` 的只读 SQLite backup

本轮未读取这些位置，真实冻结身份尚未生成。所申请的授权仅用于生成不可变 manifest：
相对路径、字节数、逐文件 SHA-256、复制前后 identity/size/time 一致性，以及两份 backup 的摘要；
canonical manifest 的 SHA-256 作为唯一 cohort ID，生成后先回报该身份。
发现 symlink、读取中变化或身份漂移即停止，不自动重捕获其他输入。
此申请不包含代表性批采、Task 4–7、模型请求、安装或交付；批采需另行确定 exact cohort 与计划。

原文和数据库副本仅存放在本 Task 私有 0700 临时根，输入种子 0400，工作副本另建。
仅执行隔离本地产品链路；不读取认证文件，不输出认证/credential 字段，不向模型或图服务上传原文。
原文最多保留 24 小时，验收及独立复核完成或到期即删除；仅保留脱敏数量、摘要、计时和结论。
本轮没有取得或使用该授权。

### update-latency 复评前置阻塞 — 2026-10-21

本次正式复评尚未产生 PASS/FAIL。当前 HEAD、工作区和修复交接已核实；仪表身份
`scan-ip-update-repair-instrumentation-d9db8dbe33f327da` 的 8 项主体均匹配。
产品候选 `scan-ip-update-repair-product-45f7482757f82715` 的 31 项主体中，29 项可重现，
但以下两个目录的摘要无法按记录配方重现：

- `cmd/agentdeck`，记录值 `d617597f3acdac5ddce5ff738c351ad66878e69b8b6f4d20b5115ec9210520a4`。
- `apps/macos/AgentDeckApp`，记录值 `7e028946ce9a591b11896a1cb83e6aea3dcb50c0641cf93f84f67e3a087e8527`。

记录仅说明路径/SHA256 条目的排序和拼接，未保留目录文件筛选清单。按全文件及构建说明的运行源码
范围进行有界核对后仍不一致，故停止继续猜测配方。此结果不证明产品内容已漂移；它说明尚不能把
本次当前源码与已有 native/CLI 运行观察绑定。元数据整体摘要可重现也不能替代主体核对。
方法、明确边界和当前值见[前置核对记录](../evidence/incremental-update-rereview-preflight-20261021.json)。

前置：修复交接方补齐上述目录原始 per-file 清单/哈希及精确摘要配方；若确认内容已改变，登记新候选、
影响及适用验证，保留旧观察身份。解除此前置后再继续同一 update-latency 复评，不重复旧 Round 2/5。
Task 3 保持 in_review、Review 未勾选；没有新增代码、构建、原生试样、真实数据采集或性能结论。
门禁仅引用修复阶段对该记录目标的 NOT_VERIFIED（3/4）回执，本轮未新查门禁或写入 CEv1。

## Round 6 — 2026-10-21

## 📋 Task 3 update-latency 修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

| 已记录阻塞 | 本轮 disposition 与证据 |
| --- | --- |
| P10 缺自动来源通知 | CLOSED。生命周期绑定的 [source monitor](../../../../apps/macos/AgentDeckShared/DesktopRefreshScheduler.swift#L8) 接入现有 [sourceChange 协调入口](../../../../apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift#L2030)，在途通知合并为一次追赶，停止时关闭 stream。现有真实 App 合成试样经文件写入、实际 helper/worker、publication 到可见新值绘制，本轮核对原始时钟、数值帧及完整 oracle 记录。 |
| P07 WAL 初始化被拒绝 | CLOSED。[精确单语句分类](../../../../scripts/p07measure/autocommit.go#L83) 保留 write-capable autocommit，并计入 Exec 返回前的完整等待；真实文件 SQLite 的定向回归确认模式改变及连接池等待。未知/复合写仍 fail closed。 |
| P07 结束窗口不完整 | CLOSED。[进程屏障](../../../../scripts/p07measure/process.go#L17) 与 [collector 结束规则](../../../../scripts/scan_performance_p07.py#L162) 保留根进程至 worker 终点、完整最终帧及 summary 写完。定向回归与原始 RSS trace 的端点/身份核对均通过，旧失败 trace 保留。 |
| CEv1 协议包不可用 | RESOLVED。已使用明确的 ai-tools source checkout，复用其未修改固定模板，通过现有 Neo4j MCP 查询当前精确产品目标。 |
| 本轮候选身份前置阻塞 | RESOLVED。[只读复算](../evidence/incremental-update-repair-digest-replay-20261021.py) 重现 69 个 CLI 文件和 15 个 App 文件的全部条目及原目录摘要，新增/缺失/内容/排序差异均为空。清单是本次按原算法重建，不冒称修复时已独立留存。原候选、旧哈希和观察未改变。 |

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；产品 `scan-ip-update-repair-product-45f7482757f82715` / SHA-256 `45f7482757f8271567909e80d93980b3678ab2c3f97895ef51afa44f04e67c45`；独立仪表 `scan-ip-update-repair-instrumentation-d9db8dbe33f327da` / SHA-256 `d9db8dbe33f327da0b246ef6818139fb349bcf4c233a06044ee7641bda00f03d`。产品 31 项、仪表 8 项主体已完成当前核对。
- Reviewer / Method：Codex 独立复评，未参与本次修复，无委派。沿用上次已完成的定向源码检查；本轮新执行 Go 3 项、Python 9 项回归，全部 PASS。独立重算持久事务记录、核对 RSS 逐帧求和/去重及根/worker 屏障端点，并复算原生时钟和完整 snapshot/oracle。完整方法与结果见[复评证据](../evidence/incremental-update-rereview-r6-20261021.json)。
- Actual product channel：当前候选既有合成 App 试样的写入至绘制为 841.069733ms；按已落盘端点计算为 840.600532ms。旧/新 publication ID 不同，new numeric frame 有效；snapshot 与 full oracle 完整归一化结果相同、partial=false，SHA-256 `2a4412b2f47f819c9f565e9a7256f978625c54e2095c2ea6d009ef62447931cd`。首次点击未路由和后续 ready-click 的字段保持原义；本轮只复核已打开 popover 的更新端点，不将其当作 E1 首击成功。发布分段时钟为 helper 提交后 App 安装 envelope 的端点，不称 SQLite commit 时间。
- Meter channel：原始 478 笔终结记录/75 个 Go process logs 重算一致，core/session WAL 分别 50/20，complete=true、rejected=0、unmatched=0。原始 RSS 35 帧均完整；根进程保留至最后一帧，两个受管 Go 进程的起止 PID/birth 端点均匹配。103,563,264 bytes 为这条合成 trace 的 sampled maximum。跨场景事务汇总仅证明计量覆盖，不作为正式分布；旧拒绝和无效帧记录不删改。
- Evidence reuse：候选和工具链匹配后，复用修复阶段 native 79 项、Go meter 20 项、race/vet、Darwin 双架构构建及实际 App/CLI 运行证据；本轮没有重建或重跑 GUI。原五个 Go 产品目录未变，既有技术回归继续按 scope 复用。原 R1/R3/R4 CLOSED 和 Round 2/5 的正式结论保留其历史身份。
- Documentation routing：配方全文保留为[证据附件](../evidence/incremental-update-repair-digest-recipe-20261021.txt)，由 `.md` 改为 `.txt`；内容 SHA-256 仍为 `ea69cf7dbd20a880979370ea4f3167cf13e8a7fb863248f05424313b9cb07d35`。此举只纠正附件被主题审计识别为未登记文档的问题，不改产品身份。
- Completion gate：NOT_VERIFIED。固定模板对当前产品目标的新查询确认 4 项必需准则中 3 项已有有效证据，仅 `update-latency` 缺正式代表性验收；invalidated/unresolved 均为空。完整 envelope 与本轮记录回执见[精确门禁](../evidence/incremental-update-rereview-r6-gate-20261021.json)。旧目标的 FAILED 留在原身份上，没有被覆盖，也未用它替代新目标查询。
- Acceptance boundary：本轮确认修复和合成运行通道；P10 正式分布尚未建立，历史 attempted=1、failed=1、valid=0 原义保留；代表性 P07 attempted=0、valid=0。仍需新授权的只读冻结输入、固定 cohort/DB 起点和正式计划，随后完成 P10 p95、P07 RSS/完整事务及前台/切换约束。没有真实数据采集、产品/测试/配置修改、Tasks 4–7 或交付。
- 结论：记录的修复阻塞均已关闭，未发现新问题，复评 PASS。Task 3 Review 已通过；完成门仍开放，任务保持 in_review，不进入 awaiting_commit。

Task checkpoint：ad-scan-performance-incremental-projections；content_state=45f7482757f8271567909e80d93980b3678ab2c3f97895ef51afa44f04e67c45；gate=NOT_VERIFIED。
提交建议：门禁 VERIFIED 后，完成新授权代表性 cohort 上的 P10/P07 分布及前台/切换验收、确认累积变更范围，再另行授权提交。
推送建议：门禁 VERIFIED 后，同一代表性验收前置闭环、提交另获授权且提交对象/SSH 签名核对完成，再另行授权推送至 origin/feature/scan-performance。

验证：scan-performance / incremental-projections / update-latency（先明确新的私有只读冻结授权并固定 cohort/DB 起点，再执行 P10/P07 正式代表性验收）
WORKFLOW_WORKSPACE: agent-deck.scan-performance


## Round 7 — 2026-10-21

## 📋 update-latency 有界补充开发评审

📊 总体评分：7/10

✅ 评审结论：FAIL

评审内容状态：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；产品 `scan-ip-bounded-product-e14fca302e5e8572`（`e14fca302e5e85725a7642c210cde769ca0f170e7e66660b941bb5416df8fc72`）；仪表 `scan-ip-bounded-instrumentation-df7a0429f058d27f`（`df7a0429f058d27fcec37a6e70e1ebfecb6b66ab9b91ebdd4fc1cf6dbf02482f`）；编排 `scan-ip-bounded-orchestration-067e1bcc6aee7761`（`067e1bcc6aee7761744366ae826da931dba2b3a29a64955942e45799c1616921`）。三份文件清单的232/23/4项及候选摘要均复现；本轮24文件差异和patch摘要匹配。requirements/performance-evaluation亦在产品文件清单中；tasks状态更新不属于该产品指纹。

评审者：Codex，REVIEW `Au6g3ZcAf48iy2Z1`；未修改本候选产品、测试或配置，未委派。保留前序讨论背景，不将本轮称为另一次全新冷上下文面板。

方法与范围：以有界交接、精确源码差异和当前源码审查原生readiness、覆盖/孤儿清理、完整事务归因与RSS成员闭合；以独立确定性元数据反例检验新增RSS切面算法。发现决定性阻断后按项目规则停止广泛检查，不据此宣称余下代码无缺陷。复用已记录的适用Go/race/vet/native和最终差分/生成器检查，不重复Round6或性能批采。

适用合同：保留用户2026-10-21撤销P01点击/P10计时的决定；本轮不恢复点击分位或绘制时钟。P07聚合RSS与完整写事务50ms/250ms标准，以及P10自动更新/publication/full-oracle正确性仍适用。

### 🔴 严重问题 — 必须修复

**SP-IP-R7-F1 — P1 — OPEN，承载任务 `ad-scan-performance-incremental-projections`**

位置：`scripts/scan_performance_p07.py:203`、`:210`、`:233`、`:266`；关联 `scripts/p07_rss_support.py:119`。

问题：成员枚举结束后，非Go-ledger后代可在本帧驻留采集中fork，并在最终切面仍存活。ExitObserver已将该fork交付为`fork_observed`，但ProcessSet仅保存事件；补采循环只遍历ChildLedger.starts，没有因该事件重查父进程/后代，也没有把未闭合fork列为issue。因此在已知成员集合可能改变的情况下，仍可返回`complete=true`并少计RSS。

行为风险：缺失存活子进程的帧可能进入有效trace，低估engine+collector+后代聚合RSS，制造P07预算通过的错误证据。正常退出的证明并不能证明一个新fork在切面之前已被覆盖或已退出。

证据：一次确定性fake-metadata复现（不是原生RSS或性能样本）。根PID1/9先完成成员枚举；采集PID1驻留时创建PID2并保留至切面，随后交付父PID1的fork事件。切面成员应为`[1,2,9]`，合成RSS合计300000150 bytes；实际输出`[1,9]`、150 bytes、`complete=true`、`issues=[]`，同时lifecycle中确有fork_observed。源码SHA256 `4282cfb5560ff7bf5be2f5edb0fa5a4891aa7a4975899e511e3aa1e081217f64`匹配当前仪表。原始结果和可复制程序分别见[复现事实](../evidence/incremental-update-review-r7-membership-repro-20261021.json)与[复现程序](../evidence/incremental-update-review-r7-membership-repro-20261021.txt)。合成数值仅用于证明少计，不是实际RSS超预算结论。

💡 有界修复：将采集期间fork事件纳入切面成员闭合。对相关父进程/后代做同帧有界补采和出生身份验证；未能证明新后代已采集或在切面前退出时必须记录明确缺口并使帧/trace无效。不能仅忽略事件、填零、过滤子进程或修补历史trace。覆盖无Go-ledger子/孙进程在枚举后及补采期间出生、仍存活/已退出、PID复用和闭合预算耗尽等控制。固定50ms、完整窗口、观察开销和未知即无效规则保留。

### 🟡 建议改进 — 推荐

无独立建议项；本轮只登记已由决定性反例证实的阻断发现。

### 🟢 优点

- 新交接保留完整事务边界、三段归因及历史失败；没有把最终SQL微调前的诊断时间改绑为当前性能。
- 用户撤销决定和仍适用的资源/事务合同有单独记录；真实追加后的自动publication/full-oracle正确性与性能分布明确分开。

### 📝 总结

本轮有界候选评审FAIL，唯一新增阻断为SP-IP-R7-F1；Round6 PASS及其原范围保持历史原义。Task3 Review保持未勾选，返回同一任务修复。候选产品、仪表、编排及要求保持只读。

性能验收独立判断：精确产品门仍FAILED，三个技术准则有有效PASS；P07已记录core/session AcceptInventory完整事务384.921506/285.331956ms反例，代表性RSS仍无有效通过证明。本轮fake-metadata发现只说明仪表覆盖不成立，不新增或伪造性能分布，也不声称实际RSS超过256MiB。撤销的点击/P10计时不恢复，现有非计时真实追加正确性观察保留。

证据：范围/差异见`incremental-update-bounded-review-scope-20261021.json`及`incremental-update-bounded-review-delta-20261021.patch`；候选清单与指纹复算通过；独立复现执行一次并确认反例；未运行真实数据采集、GUI/原生性能或广泛回归。评审记录结构检查与限定文档git diff --check通过。make check-whitespace退出2：已有冻结差异patch的31个空上下文行（统一diff要求的单个空格）被通用扫描器判为trailing whitespace；当前源码未报该问题，未修改或重算该证据patch，不把此检查说成通过。完整结果见本轮评审证据。未进入Tasks4–7或Git交付。

完成门禁：FAILED

精确目标为`scan-ip-bounded-product-e14fca302e5e8572`，固定模板查询与新事实写入回执见[Round7证据](../evidence/incremental-update-review-r7-20261021.json)及[精确门禁](../evidence/incremental-update-review-r7-gate-20261021.json)。评审FAIL与该已有性能门失败分别记录。

### 后续指令

修复范围仅SP-IP-R7-F1。闭合后复评该发现并完成本轮剩余新增差异审查，复用已核对且未变的范围；不重复Round6，不把现存P07 max失败当作已修复。

```text
修复：scan-performance / incremental-projections / SP-IP-R7-F1
WORKFLOW_WORKSPACE: agent-deck.scan-performance

依据 reviews/incremental-projections.md Round7，仅修复已知fork事件下的RSS成员闭合及其回归。
未闭合的fork必须使帧和trace无效；不删除旧帧、不填零、不排除业务后代、不修改50ms及预算。
保留用户撤销决定、Round6 PASS及P07事务max FAIL。完成后给出复评指令。
不委派，不进入Tasks4–7，不提交、推送、安装、发布或部署。
```

### SP-IP-R7-F1 修复附记 — 2026-10-21

SP-IP-R7-F1 -> **repaired in candidate，待独立复评**；本附记不是新评审轮或 PASS。
原 Round7 FAIL 和原始复现均保留。修复者 REPAIR `xiFHGYjUjeE5MIrp`，未委派。

根因已按原程序复现：fork 事件只进入 lifecycle，晚成员补采却仅遍历 Go ledger。
修正使两类事件共同触发最多三次同帧子树重查；包含已选后代，核验父/子 birth，
新 resident 每身份只采一次。最后一次 drain 收到 fork 或耗尽闭合预算均明确无效。
NOTE_FORK 不提供子 PID 且可合并；发现某个子进程或证明其退出不能排除另一个已
重挂父进程的后代。因此未知 fork 不宣称闭合，保留明确 gap，并以首条事实作常量大小
跨帧标记，防止下一帧无新事件就恢复 complete。未缩短窗口、改50ms、填零、删除帧
或扣观察开销。此保守修正不建立含未知 fork 的可用正式 RSS 验收通道。

原确定性程序修后 reported_members 包含1/2/9，合成 RSS=300000150，complete=false，
issues 含 fork_membership_unresolved；不是实际 RSS 超预算的结论。
受影响两个 Python 模块共33项通过、0失败、0跳过，新增8项确定性控制与1项真实
原生 fork 控制；覆盖晚出生子/孙、消失/重挂、PID复用、最终drain、预算耗尽、
整 trace 无效、固定50ms及保留全部帧。完整私有 stdout/stderr/退出码已保存。

新仪表为 `scan-ip-bounded-instrumentation-a1f70d08e822de14`。产品232项、编排4项
逐项及完整状态均不变，现有 Go/SQL/native UI 和三个技术准则证据显式复用，未重跑。
CEv1 新增5节点/6关系并精确读回；旧整仪表校准不复用于新 RSS 完整性，事务反例保留。
精确产品门仍 FAILED、4项 required 中3项有有效证据、候选影响均已解决；不是修复通过
即可完成 Task3。详见 [修复结果](../evidence/incremental-update-repair-r7-result-20261021.json)、
[门禁](../evidence/incremental-update-repair-r7-gate-20261021.json) 与
[复评交接](../evidence/incremental-update-repair-r7-handoff-20261021.md)。

独立复评确认本发现后，按 Round7 原定范围完成尚未覆盖的新增差异审查；不重复 Round6，
不恢复点击/P10计时，不重新采集真实 HOME，不进入 Tasks4–7 或 Git 交付。

本轮L0：新增源码 whitespace、158个本地链接及限定 git diff --check 通过。
全局 make check-whitespace 仍退出2，仅报告原冻结差异 patch 的31个空上下文行；
其 SHA256 与原交接完全一致。本轮不修改历史证据 patch，新差异使用 gzip 保存，
不把该全局检查记为通过。验证收尾见修复完成回执。

## Round 8 — 2026-10-22

## 📋 Task 3 R7-F1 修复与剩余有界差异复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

| 既有发现或阻塞 | 本轮处置及证据 |
| --- | --- |
| SP-IP-R7-F1 | **CLOSED**。`ProcessSet.sample` 将 kernel fork 和 Go-ledger 晚登记一起纳入最多三轮同帧子树重查；父/子 birth 校验和已采身份去重保留。原 Round7 程序在当前源码上独立重放，成员由原 `[1,9]` 变为 `[1,9,2]`，合成 RSS 从150变为300000150 bytes，且 `complete=false`、`issues=["fork_membership_unresolved"]`。因此不再出现已知 fork 漏计却报告完整的原反例。 |
| 未知/coalesced fork、最终 drain 和闭合预算 | CLOSED，作为同一发现的边界核对。`scripts/scan_performance_p07.py:229` 起执行有界补采，`:279` 起保留首条未知 fork 的跨帧标记；最终 drain 的晚事件及预算耗尽均列为 issue。`sample_rss` 保留全部帧，以 incomplete/missed/drop/error 判整条 trace 无效。稳定枚举或某个已知子进程退出均未被当作全部出生已闭合的证明。 |
| SP-IP-R1-F1、SP-IP-R3-F1、SP-IP-R4-F1 | 保持 **CLOSED**。当前有界差异未改变 moved-from affected-view 修复、autocommit 分类或只读 CTE 判定；原 Round2/4/5 的对应证据不因本次阶段变化失效。事务新增 caller/phase 字段没有缩短原始完整区间；新的 exec 观察保留 Start/Wait/Output 的错误和输出语义。 |
| Round6 的通知、WAL 初始化计量及结束窗口修复 | 保持原范围 PASS。真实来源通知和 WAL 分类未改变；本轮检查了新增 business-child 结束约束、异步日志 drain 和 root release 顺序，没有用本次合成反例替代历史原生观察。 |

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`。产品 `scan-ip-bounded-product-e14fca302e5e8572` / SHA-256 `e14fca302e5e85725a7642c210cde769ca0f170e7e66660b941bb5416df8fc72`；仪表 `scan-ip-bounded-instrumentation-a1f70d08e822de14` / `a1f70d08e822de14e0971ceea980d1fa2181aeeb6414d98b64f88ced864b12fe`；编排 `scan-ip-bounded-orchestration-067e1bcc6aee7761` / `067e1bcc6aee7761744366ae826da931dba2b3a29a64955942e45799c1616921`。用原 `scan-performance-update-bounded-state.py` 复算232/23/4项文件、subject digests和完整指纹，全部一致，无增删或内容差异。Go1.27.1、Xcode26.4/17E192、macOS26.7.2/25G325、Python3.15.0及x86_64均与记录相同。
- Reviewer / Method：Codex 独立复评，本会话未产出该修复，未委派。接手已释放的同一任务；基于原 Round7 反例、四文件修复差异和原24文件有界差异进行定向源码/SQL/回归断言审查，没有另建评审面板或将背景资料当作运行证据。CodeGraph 未返回新增脚本的目标源码后，使用精确文件清单定向读取。原冻结有界 patch 的 SHA-256 `97489d68e6bb934674b163c4c1fb1370bd056f101bc3b5cbaafcf9f4f21e1911` 和 R7 修复 gzip 差异 `f1888e021d561fa3503af0833b63c98315eb57231795fd42e39e8a536717112e` 均匹配。
- 新执行证据：执行一次原 `incremental-update-review-r7-membership-repro-20261021.txt` 中的完整程序，退出0，结果如上；程序及历史结果没有修改。它使用确定性元数据，300000150只是用于证明成员求和的合成数值，不是实际 RSS、预算反例或正式性能样本。新源码 SHA-256 `28f57d61291f9fdb6610d0c92f0ad90e29f8fdf58d7fe2c06faffc3484d556bb` 与修复清单一致。
- 余下新增差异：已完成 Round7 因首个阻断而停止的审查。孤儿路径 UNION 同时覆盖 event/tool-only 来源，发现、删除及 session 重建仍在原事务内；prepared coverage 保留 identity/end/RowsAffected 和整批 rollback。readiness 仍限 DEBUG 完整性入口。事务 Begin/body/terminal 三段相加等于原完整区间，未豁免池/锁等待；typed exec-only 包、方法逃逸拒绝、Start/Wait 出生/退出绑定及 stderr 行为均有对应回归。RSS end-cut、未知退出、跨帧成员、writer overflow/error、完整结束窗口和三轮 fork 补采按同一 fail-closed 规则审查。编排保留固定 manifest、期限前置及不延长保留期的要求；未执行旧 cohort 或任何实际采集动作。三个文档仅复核本次用户撤销决定及有界说明，不重做旧设计集评审。
- Evidence reuse：当前产品和编排主体及工具链未变，复用已记录 full Go1355 PASS/7 skip、race377 PASS/1 skip、meter Go21 PASS/1 skip、最终孤儿差分5 PASS和generator7 PASS，以及适用 vet、双架构构建、原生 readiness/编译和自动追加/full-oracle观察。当前 RSS 仪表23项均匹配修复清单，复用其33项 Python/受控原生回归（0失败、0跳过，含9项新增控制）。未重跑这些套件，也未把旧整仪表校准改绑为新 RSS 完整性证据。
- 完成门禁：FAILED。对 `scan-performance:incremental-projections` 和上述精确产品 ContentState 使用当前固定 `gate-status.cypher` 查询；四项准则均 required，generation-switch、partition-equivalence、nonadditive-statistics 三项有效 PASS，update-latency 的有效失败证据仍为 `scan-ip-bounded-product-e14fca302e5e8572:p07-max-fail`。invalidated列表是旧仪表校准及旧冻结阻塞两项，unresolved impacts为空。新复评事实按 acceptance `not_verified` 登记并依赖当前仪表/编排，不能冒充 update-latency PASS；写入后重新查询同一精确门。Neo4j schema 辅助工具的 `sample: None` 缺陷未影响普通 Cypher/固定模板。
- 验收与残余不确定性：保留 core/session完整事务384.921506/285.331956ms超过250ms的反例。代表性聚合 RSS 尚无有效通过证明；未知 NOTE_FORK 使帧及整条 trace 无效，这次修复没有建立可用的代表性 RSS 验收通道。P07 RSS≤256MiB、完整事务p95≤50ms/max≤250ms仍适用。P10只保留真实文件自动更新、publication/partial及full-oracle正确性；点击/P10计时撤销决定、旧失败、无效trace和Round6原范围PASS均保留。本轮没有真实HOME/cohort读取、GUI、点击、原生绘制计时、性能采样、产品/测试/配置修改或Git交付。
- 状态与边界：SP-IP-R7-F1已关闭，无 still-open/regressed/new 发现，复评PASS。Task3 Review勾选，但完成边界保持开放/in_review，不进入awaiting_commit。CEv1当前Task无子WorkUnit或声明的父contains边；仓库Task4–7仍未完成，本轮不跨Topic边界。全局whitespace历史阻断仍为冻结patch的31个单空格上下文行，原patch未修改；本轮仅更新评审、证据和topic状态，局部结构/链接/diff与新增内容检查单独记录，不将全局检查报为通过。
- 记录收尾：Neo4j新增1节点/4关系，创建metadata与提交数量一致，关系预检4/4为ok；节点attributes及四条关系端点/属性均精确读回。MCP未返回processed结果行，不虚构该计数。写入后的固定精确门仍FAILED，三条技术证据可复用、unresolved为空，新复评事实为applicable/not_verified。当前155个本地链接及新增JSON/JSONL、局部whitespace检查通过。全局`make check-whitespace`本轮退出2，仍仅上述31行；原patch不修改。记录结构和限定diff检查随本轮最终附记核对。

Task checkpoint：ad-scan-performance-incremental-projections；content_state=e14fca302e5e85725a7642c210cde769ca0f170e7e66660b941bb5416df8fc72；gate=FAILED。
提交建议：门禁 VERIFIED 后，先闭合 P07 完整事务 max 超限并取得有效聚合 RSS/完整事务证据，再确认 Task3 累积候选范围并另获提交授权；本轮复评不执行提交。
推送建议：门禁 VERIFIED 后，同一 P07 前置闭环且授权提交的范围、提交对象、trailer及SSH签名核对完成，再另获授权推送至 origin/feature/scan-performance。

补齐验收：scan-performance / incremental-projections / P07（闭合完整事务 max 超限及有效聚合 RSS 缺证，保持现行预算与历史反例）
WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 9 — 2026-10-22

## 📋 P07 有界事务修复与 RSS 验证条件修订独立评审

📊 综合评分：9/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无本轮已确认的变更所致代码缺陷。四包 race 的一次 publication 失败仍未解释，
按下文保留为验证限制，不能用隔离重跑通过将其改写为通过。

### 🟡 改进建议 — 推荐

无代码修改建议。代表性完整事务分布仍缺证；本轮不进行真实输入捕获。

### 🟢 优点

- 每批128行、512参数，语句形状和参数缓冲有界。校验及形状构建先于 Begin，
  全部 inventory、checkpoints、head 和行级 revision 仍在同一事务内提交或回滚。
- core/session 覆盖0、1、127、128、129、4717源；逐行核对路径、identity、captured_end、
  committed_end、state。后批重复、触发器失败、非法来源和 head 更新失败均不留下部分状态。
- 原 publication、epoch、full/partial 和有限 inventory 回归独立通过。未改变
  durability、WAL、timeout、外部格式、依赖或事务终端边界。
- RSS 不可验证已如实记录并依用户决定取消该项强制量化验证；没有把枚举、ledger 或
  已知子进程退出当作完整未知 fork 证明，没有修改旧无效 trace 或声称预算达标。

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；
  产品 `scan-ip-bounded-product-92c37854995da877`，fingerprint
  `92c37854995da8777f3eda52de8444bdec1bfae542274bc5fa269525c0ea962a`。
  [内容清单](../evidence/incremental-p07-repair-content-states-20261022.json)包含234项；
  相对 Round8 仅 source_coverage.go、requirements/evaluation 两文档变化，新增批次测试和
  RSS 决定快照。仪表 `a1f70d08e822de14` 与编排 `067e1bcc6aee7761` 完全未变。
- Reviewer / Method：用户明确要求同配置 subagent 独立评审；以 `fork_turns=none`
  创建冷上下文只读评审角色，继承主会话模型/推理配置，没有覆盖模型或降级。
  运行时未暴露可核实的具体后端模型标识，不编造模型名。主代理核对源码 Git blobs、
  独立9项原始结果和最终文档摘要后记录结论。子代理不修改任何工作产品、状态或证据。
- Scope：仅本轮 AcceptInventory 批量 INSERT、其新回归，以及按用户最新决定修改的
  requirements/evaluation RSS 段落和 P07 行。没有重评 Round6/8 已覆盖的未变实现，
  也没有将 Documents matrix 中仍待整体评审的文档改为整体 PASS。
  source blob=`827ab015d87ad1bd851bbb0ca4fca823e3905bb2`；
  test blob=`fb435c50035fbbb07459102011bd03dbdd3cc6fe`。
- 独立 Evidence：5项 store 回归和4项 desktop/scanruntime publication 回归通过，
  0跳过、0失败。检查成功/回滚的完整 rows、revision=count+1、旧 head、旧 publication、
  epoch 拒绝及 full publication 保留。单次 wall 只作为诊断，不当作完整事务分布。
  没有新增中途取消注入实验；检查同一 context、BeginTx 契约和 deferred rollback。
- 主代理验证：full Go1358 PASS/7skip，vet及Darwin arm64/amd64构建通过。
  用最终新 overlay 运行合成库存回归，12笔 explicit Begin→Commit返回端点及
  Begin/body/terminal 相加均正确；4717源 core35.875382ms/session37.961969ms。
  这些仅为校准；没有代表性新候选分布，不能据此宣称正式 p95/max 或 Task3 接受。
  全部必要事实和时钟直接保留在[开发记录](../evidence/incremental-p07-repair-20261022.json)。
- race 验证限制：四包组合391 PASS/1 FAIL；唯一顶层失败为
  `TestServingEmptyInventoryReconcilesLikeOrdinaryScan/seeded=true/serving=true`，
  `internal/scanruntime/serving_test.go:466` 报 `seed not published`，无 DATA RACE 报告。
  旧源 overlay 和候选分别单独执行同一 race 子用例各1次，均 PASS；没有放宽 timeout、
  改断言或重复整套。根因未确认，不能称为已排除的环境问题或已修复。
  子代理在收到此新事实后维持有界代码 PASS，同时明确不能据此排除回归。
- 用户决定：收到用户本机 `proc:::create` 被 SIP 拒绝的结果后，用户明确指示
  “无法验证就写明后修改验证条件取消该条目验证 不要继续花费时间”。
  [决定快照](../evidence/incremental-p07-rss-validation-withdrawal-20261022.json)
  保存旧 criterion/门禁及修订。仅 Task3 aggregate RSS 强制量化验证取消；
  256MiB工程预算保留且未验证，完整事务 p95≤50ms/max≤250ms、原子性及其他任务独立资源门不变。
  后补的 canonical criterion/读回字段由主代理直接核对，不改变已独立审查的用户决定。
- 完成门禁：NOT_VERIFIED。三个未变技术准则经当前事务回归和范围评估显式 roll-up；
  update-latency 只有本轮校准/评审事实，代表性完整事务分布与上述 race 风险尚未闭环。
  [精确门禁](../evidence/incremental-p07-repair-gate-20261022.json)绑定当前产品。
  原候选 max384.921506/285.331956ms、原 FAILED 门、旧无效RSS和全部历史评审保持原义。
- 状态：独立有界评审 PASS，Task3 Review 勾选；任务仍开放/in_review，不进入 awaiting_commit。
  本轮没有重新读取真实 HOME/cohort、捕获输入、GUI、点击、RSS/P10计时、安装或Git交付。
  Topic 还有Tasks4–7，本轮不跨Topic完成边界。
- 文档限制：topic 文档审计报告两个既有 evidence handoff Markdown 缺 Documents 行，
  本轮不调整旧声明。全局 whitespace 的既有冻结 patch 问题与本轮局部检查分别记录。

Task checkpoint：ad-scan-performance-incremental-projections；上述产品内容状态；gate=NOT_VERIFIED。
提交建议：等待剩余完成门通过，再另获授权提交 Task3 的明确候选边界；本轮不交付。
推送建议：等待完成门、授权提交对象/范围/trailer/SSH签名核对及单独推送授权。

## Round 10 — 2026-10-22

## 📋 P07 剩余事务证据与 race 未决项有界评审

📊 综合评分：8/10

✅ Verdict: PASS

### 🔴 严重问题 — 必须修复

无新增已证实的变更所致缺陷。原 race 失败未解决，作为验收风险保留；
没有因本次诊断未复现而把原失败关闭或改写为通过。

### 🟡 改进建议 — 推荐

无新增代码建议。200ms reader deadline 目前仅是候选归因路径，不能据此放宽预算、
调整断言或修复产品。剩余门禁前置见下文，不能以合成校准替代。

### 🟢 优点

完整复用未变的代码、仪表、编排、独立评审与适用验证；只执行一次有界诊断，
保留调用原返回值、原15s context、200ms预算及断言。RSS验证取消决定保持，
没有恢复采集、捕获真实输入、扩大套件或修改产品/测试/配置。

### 📝 总结

- Reviewed state：HEAD `14234ec4c96a3376d1072147eeb6e42b8768381a`；
  `scan-ip-bounded-product-92c37854995da877` /
  `92c37854995da8777f3eda52de8444bdec1bfae542274bc5fa269525c0ea962a`。
  对[既有清单](../evidence/incremental-p07-repair-content-states-20261022.json)
  复算产品234、仪表23、编排4项，全部匹配；没有新的产品内容状态。
- Reviewer / Method / Scope：Codex主代理进行剩余证据与gate审查；沿用用户已授权的
  同配置、冷上下文独立评审角色，仅读收集race材料。主代理直接核对私有overlay差异、
  原始日志及命令/退出码后记录；子代理不拥有正式判定、状态或CEv1写入。
  复用Round9独立代码及本次RSS条件修订PASS，不重复其审查。
- 完整事务证据：保留12笔唯一完整span，phase方程逐笔相等。core/session各6笔合成
  库存用例，最近秩p95/max分别35.875382/37.961969ms。这些数值与原记录一致，
  但不是代表性重建数据库分布；p95≤50ms/max≤250ms准则仍未满足证据门。
  已有获准私有输入缺失的记录保留，本轮没有重捕获、读真实HOME或将旧候选结果改绑。
- race源码路径：store/serving_generation.go:106有200ms私有context；
  desktop/serving_cache.go:167、203在publication前后读generation并返回错误；
  scanruntime/serving.go:238、264只在publication错误为nil时安装结果，不传播该错误；
  serving_test.go:465的旧断言同时涵盖publication=nil与tokens≠28。仅凭源码路径不足以定因。
- 唯一新实验：私有诊断状态 `scan-ip-p07-r10-diagnostic-00a995e7e3a1e155`，
  在三个私有overlay文件中只增加调用返回后的观测及最多4次独立fixture；
  仓库对应源文件及断言/超时未修改。4/4 fixture PASS，24次generation read、
  12次publication调用均error=nil，4次seed均publication_nil=false、tokens=28。
  原始日志SHA256 `13e4bf8637f69b51b3dc992c70f700fbb789ba6d84b807df402adcfb6bd29bf7`。
  一次generation read外部wall为207.379439ms但err=nil，直接反驳
  “外部wall>200ms即可证明reader超时”的推断。私有诊断不能充当正式产品性能观察。
- 根因分类：unresolved。原四包race391 PASS/1 FAIL及两次旧/新隔离PASS均保留，
  这4次诊断PASS不构成修复、全race通过或排除回归的证明。缺失的是原失败窗口的
  publication实际错误/parent context，以及seed究竟为nil还是totals错误。
  已完成本轮唯一实验，不再追加尝试。必要事实已直接固化于
  [本轮证据](../evidence/incremental-p07-review-r10-20261022.json)，不依赖临时路径。
- Evidence reuse：复用full Go1358 PASS/7skip、vet、Darwin双架构及独立9项回归；
  三项技术criterion仍有精确目标roll-up，代码未变，无新增候选影响或重新运行义务。
  既有全局whitespace31项冻结patch问题和两个旧handoff的topic矩阵问题不改写为通过。
- 完成门禁：VERIFIED。用户随后按下文接受附记批准两项验收修订，当前接受目标为
  `scan-ip-bounded-product-22d321fe2ab870af`。原R10首次query在p92目标为
  `NOT_VERIFIED`，其[原始回执](../evidence/incremental-p07-review-r10-gate-20261022.json)
  保留不改；[新准则精确回执](../evidence/task3-p07-acceptance-gate-20261022.json)
  覆盖四项必需criterion。WorkUnit仍为Task，Topic还有Tasks4–7，不跨Topic完成边界。
- 原R10判定范围：证据审查PASS确认复用和未解决结论有据；当时不是P07接受、
  代表性事务达标或race关闭，当时任务保持in_review。后续接受仅依据下文用户批准修订，
  不改变原观察、历史失败、RSS无效trace或Round9/10独立审查范围。

历史 finding 标识处置（沿用 Round8，不是新增修复或接受）：

- SP-IP-R7-F1 CLOSED：Round8 已独立核验原 fork 成员漏计反例关闭；本轮保留该处置。
- IP-R7-F1 CLOSED：同一历史 finding 的缩写，非独立缺陷；沿用 Round8 已核验的关闭处置。
- R7-F1 CLOSED：同一历史 finding 的缩写，非独立缺陷；沿用 Round8 已核验的关闭处置。

上述历史标识补记当时仅消除歧义，不关闭剩余事务证据或race未决项；后续接受条件和当前门禁以下文用户批准修订为准。

### 用户批准的 P07 接受条件修订 — 2026-10-22

用户明确批准两项修订：取消代表性事务分布强制验证（50/250ms工程预算保留且未验证），
并保留原race391 PASS/1 FAIL、接受为本次Task3验收例外。根因仍unresolved，
不是race修复、全race通过或新的一轮独立实现评审；RSS取消决定保持，256MiB仍未验证。
[批准审计](../evidence/task3-p07-acceptance-revision-20261022.json)保存此前criterion和
NOT_VERIFIED门禁，原R10观察、历史max失败、旧无效RSS及复合not_verified事实均不改写。

本次只改两份验收文档并新增批准记录；正常产品/测试/config与已独立评审的p92候选完全相同。
相对此前功能观察的public产品状态e14，仅inventory同事务批量INSERT及对应测试有变化；
自动来源通知、publication、统计/full-oracle代码不变，字段/revision/head/原子回滚已核验。
保留的非计时fsync19-token观察中自动更新、publication/App Group推进与完整wire-v1
oracle相等、双方partial=false，含sessions/非可加/All。按新准则重新评估这个已PASS功能
子集，未把旧复合not_verified整条观察改成PASS，也未把新私有诊断当正式产品样本。

当前验收目标为 `scan-ip-bounded-product-22d321fe2ab870af` /
`22d321fe2ab870aff91e83e5265dfc4fae1a3dca305116b7556624bdc51c6efb`；
固定模板精确门VERIFIED，四项必需criterion均覆盖、missing/unresolved为空。
12个新节点/24条关系均预检和精确读回；criterion的当前声明也已精确读回。
[最终回执](../evidence/task3-p07-acceptance-gate-20261022.json)、
[最终接受结果](../evidence/task3-p07-acceptance-result-20261022.json)与
[内容清单](../evidence/task3-p07-acceptance-content-states-20261022.json)给出独立身份和复用依据。
Task3达到按用户修订条件的验收检查点，Beads为awaiting_commit，尚未closed；
默认Task3交付边界仍要求授权提交，不套用Task2的local acceptance/no Git例外。
未提交、推送、安装、发布，未启动Tasks4–7或宣称Topic完成。


Task checkpoint：ad-scan-performance-incremental-projections；content_state=22d321fe2ab870aff91e83e5265dfc4fae1a3dca305116b7556624bdc51c6efb；gate=VERIFIED。
提交建议：Task3当前验收边界与本次修订进入交付检查点；核对累计diff及必要依赖边界后，另获授权提交，保留无关脏工作。
推送建议：授权提交的对象/tree/trailer/SSH签名核对通过且另获推送授权后，目标origin/feature/scan-performance；交付时核验remote/ref。

原R10补齐前置已由本次用户批准的验收修订替代，不再要求提供冻结输入或原失败现场记录。
WORKFLOW_WORKSPACE: agent-deck.scan-performance


