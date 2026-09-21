---
status: active
topic: desktop-refresh
subject: architecture.md
---

# Architecture Review

## Round 1 — 2026-09-19

## 📋 Desktop Refresh architecture 评审

📊 总体评分：7.8/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

**[AR-R1-F1] `architecture.md:55,190-201` quota-only 的并发仲裁没有覆盖 quota-first 与 quota-vs-quota。**

- 严重度：P1。
- 行为风险：A3 只声明一个 coordinator “serializes” quota-only，具体 transaction 只规定“background quota 到达已经拥有 quota probe 的 full generation”这一方向。它没有决定 quota-only 已在运行时 full/manual/provider-switch 如何处理，也没有决定 periodic/manual quota 彼此是 join、丢弃还是保留一个 follow-up。实现者可能排队每个 tick、并行 probe，或重复 deliver/ack alerts 与 whole-file publication，直接违反 requirements 的 single-flight/no-catch-up-storm 合同。
- 证据：当前 `refreshQuotaAlertsOnly` 是可重入 async 路径，probe、deliver/ack、fetch、publish 之间均有 suspension point；现有 generation guard 只阻止旧 subscription splice，并不阻止重复 probe 或 alert side effects。文档 `A4/A10` step 1 仅覆盖 full-first 情形，验证表也没有 quota-first / quota-vs-quota 的明确预期。
- 处置：OPEN。

💡 有界修复：补齐 full/quota 双向仲裁状态机，明确 periodic/manual quota 的 join/priority、最多一个 pending 规则、quota-first 后 full 到达时 probe 所有权、alert deliver/ack 的 exactly-once 边界、publication generation 以及每个终态如何唤醒 follow-up；在 coordinator table 中加入这些交叉组合和 catch-up 反例。

**[AR-R1-F2] `architecture.md:216,232-234,402-403` post-replace failure 的结果与恢复合同不闭合。**

- 严重度：P1。
- 行为风险：publisher 只返回 `published | superseded | failed`，步骤 4 却要求 post-replace failure 后恢复旧 entry，而验证合同允许“explicit unknown outcome”。若 rename 已成功、destination verify/fsync 失败且 rollback 也失败，磁盘可能是新、旧或不可读状态；当前类型没有表达 unknown，也没有定义 `last committed generation`、semantic comparison baseline、下一次 retry 和 UI publication state 应以哪个事实为准。实现者必须自行发明 durability 与恢复语义。
- 证据：当前 `AppGroupSnapshotStore.write` 在 rename 后才验证 destination，失败时旧文件已被替换且没有 rollback。新文档承认 after-replace unknown，但 component result 和 transaction state 没有对应分支。
- 处置：OPEN。

💡 有界修复：要么给出可证明的备份/交换/rollback 算法及其 crash、权限、清理和 generation 语义，要么在 publisher result 中增加明确的 post-replace indeterminate outcome；无论选择哪条，都定义磁盘 baseline、受影响 kind reload、`widgetPublication` 状态和下一成功如何恢复，并给出 before/after/rollback-failure 测试矩阵。

**[AR-R1-F3] `architecture.md:340-341` “oversized” 没有可实现的字节边界。**

- 严重度：P2。
- 行为风险：安全合同要求 reader 拒绝 oversized input，但未定义最大字节数、读取方式或错误映射。当前 `WidgetSnapshotReader` 直接 `Data(contentsOf:)`，会在判定前读取整个文件；实现者必须自行选择资源上限，测试也无法定义 N/N+1 边界。
- 证据：项目已有 helper snapshot 的 `maximumSnapshotBytes = 64 MiB`，但 App Group projection 是否复用或采用更小上限没有决定。typed outcome 只给出 `io | decode`，也未说明 oversized 属于哪一类固定诊断。
- 处置：OPEN。

💡 有界修复：决定 App Group projection 的固定最大字节数和有界读取/metadata preflight 顺序，定义 N、N+1、增长竞态、symlink/non-regular 与错误分类；说明 UI 仍合并为 unreadable、日志仅记录固定 code/size 而不泄漏路径或内容。

### 🟡 建议改进——推荐

无；除上述三项决定缺口外不记录偏好性重构。

### 🟢 优点

- 当前系统前提与源码一致，正确识别现有 30 秒 evaluator、五分钟 wire hint、generation guard、无条件 reloadAllTimelines、15–60 分钟 timeline 与丢失的 typed error。
- scheduler/coordinator/publisher/Widget reader 四层所有权总体清晰，menu-bar attempt 与 Widget publication 状态正确解耦。
- kind-specific semantic projection 排除了时间戳、编码顺序和 Widget 不可见字段，同时覆盖 usage availability、partial、schema recovery 和所有 intent 配置。
- privacy、schema-v1 compatibility、quota 独立时钟、native acceptance 与浏览器 evidence 边界均处理得当。

### 📝 总结

- Reviewed state：HEAD `8279718ce4cac9cd755cf974c9869719fed3a395`；`docs/topics/desktop-refresh/architecture.md` blob `bc972cb8f763797ed24a7a91b0eb8aa5463c1950`；内容指纹 `ccfe8a6645e6bd1bf3e5461090239947b4b342b708468d4635291807e2974a44`，配方为 `SHA-256(head=<HEAD>;document=<blob>)`。
- Reviewer：Codex。
- Method：设计/合同维度的证据驱动正式评审；使用 workspace-bound CodeGraph 核对 Go snapshot、App scheduler/coordinator、App Group store、Widget reader/timeline/presentation 与相关测试路径，并逐项走查时钟、双向并发、publication failure、semantic diff、compatibility、privacy 和 verification matrix。未委派。
- Scope：`docs/topics/desktop-refresh/architecture.md`；`tasks.md` 仅作为 Documents 状态权威。未评审实现、任务分解或 final-surface reconciliation。
- Findings：`AR-R1-F1`、`AR-R1-F2`、`AR-R1-F3` OPEN；无外带项。
- Evidence：`internal/desktop.Service.Build` 当前在 build start 生成 `generated_at`/五分钟 `next_refresh_at`；App 每 30 秒按 opt-in 和 hint 触发；coordinator 当前 quota/full 路径及 generation suspension points；store 当前 rename 后验证且全量 reload；Widget 当前无 typed load outcome、用 `Data(contentsOf:)` 无界读取并将 error 压成 nil；timeline 当前为 15–60 分钟。`make check-whitespace` 与 `git diff --check` 均通过。项目没有 architecture 语义 checker。
- Residual uncertainty：尚未实现的 actor/scheduler/native behavior 只能由后续测试与原生验收证明；本轮只判断合同是否足以实施，不把设计推演当 runtime PASS。
- Completion gate：FAILED。固定 CEv1 gate 对 `desktop-refresh:architecture.md` 和精确 ContentState `urn:agent-deck:content-state:desktop-refresh:architecture:8279718:bc972cb` 回查：document checks 与 handoff pass，contract-correct 由 `AR-R1-F1`、`AR-R1-F2`、`AR-R1-F3` 的 target-bound fail evidence 反证；无缺失、blocked、malformed、失效证据或未决 candidate impact。

整体分层方向可保留，但三处关键状态机/故障边界尚未决定，当前内容不能让实现者在不发明新合同的情况下安全开发。

### Repair candidate — 2026-09-19

- `AR-R1-F1 -> repaired in candidate.` 新增 full/quota 双 lane 仲裁表：quota-first
  时 full join 并等待；full quota phase 之后只保留一个 pending，manual 只提升 pending
  优先级；quota-vs-quota 全部 join 当前 operation。每个 `QuotaOperationID` 只有一个
  owner 执行一次 probe→deliver→ack→fetch→publish，joiner 不执行 side effect；文档也
  明确该边界不虚构 crash-proof OS notification exactly-once。
- `AR-R1-F2 -> repaired in candidate.` publisher result 现在区分
  `failedBeforeCommit` 与 `indeterminateAfterCommit`，以 rename 为 commit point；后者推进
  `highestAcceptedGeneration`、保持 `lastVerifiedGeneration`、baseline 置 unknown、禁止
  reload 和旧 generation 写入。下一成功强制 all-kind reload 并恢复 verified baseline；
  restart、orphan cleanup、前/后 commit failure 均有合同与测试矩阵。
- `AR-R1-F3 -> repaired in candidate.` App Group projection 固定上限为 8 MiB；reader
  使用 lstat → `O_NOFOLLOW` open → fstat → N+1 bounded read → post-fstat → header/full
  decode。N 进入 decode，N+1/增长竞态为 oversized；writer 在 temp/rename 前使用同一
  上限。UI 继续归入 unreadable，固定诊断允许 code、capped byte count 与 limit，禁止
  path/content。
- Repair candidate：HEAD `8279718ce4cac9cd755cf974c9869719fed3a395`；
  `architecture.md` blob `d7e6f5e2b327ebfac49339df1e66502c67d5602d`；内容指纹
  `f47d47b6048f98034fb6be6c2c3673f13a6e9348e1c74213767001b7348c2fe0`。
- Verification：`bash scripts/check-topic-docs.sh`、`make check-whitespace`、
  `git diff --check` 均 PASS；三条 finding 的状态、结果类型、边界常量和交叉测试均可从
  修复后文档精确定位。生产代码、测试与配置未修改。

Round 1 结论仍为 FAIL，Review 矩阵仍未勾选；以上仅声明三条 finding 的返修候选已就绪，
是否 CLOSED、是否出现相邻回归以及新候选门禁结果由独立复评决定。

## Round 2 — 2026-09-19

## 📋 Desktop Refresh architecture 复评

📊 总体评分：8.2/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

**[AR-R2-F1] `architecture.md:135-137,293-326` pre-commit 失败未提升 generation 屏障，旧 projection 仍可能随后写入。**

- 严重度：P1。
- 处置：NEW。
- 行为风险：合同要求旧 full/quota publication 不能覆盖或替较新 accepted projection 报告结果，但 `highestAcceptedGeneration` 只在 rename 成功的 commit point 才前进；`failedBeforeCommit` 又明确保持两个 generation fact 不变。因此 generation 10 在 encode/temp/rename 前失败后，已经排队的 generation 9 仍高于旧屏障并可被接受、写回旧 projection，甚至以旧成功清除较新的 publication failure。
- 证据：`architecture.md:293-306` 仅以 `highestAcceptedGeneration` 拒绝旧请求并在 rename 后推进它；`:319-326` 对 pre-commit failure 保持该事实不变，只对 post-commit indeterminate 明确阻止旧写。serial actor 只保证执行互斥，不保证 actor mailbox 按 generation 排序。当前实现尚无该新 publisher，可由合同直接构造 `newer pre-commit fail → older queued publish` 反例。

💡 有界修复：把“已受理写入屏障”与“已验证磁盘 generation”分开；在任何可能 suspension/write 前原子推进前者。pre-commit failure 可保留旧磁盘和 semantic baseline，但不得回退受理屏障；所有更低 generation 必须返回 `superseded` 且不得写、reload 或改写用户状态。给 failure matrix 增加“较新 pre-commit 失败后较旧请求到达/已排队”的顺序反例。

**[AR-R2-F2] `architecture.md:278-303,387-420,470` host `readExisting()` 没有可实现的同界有界读取合同。**

- 严重度：P2。
- 处置：NEW。
- 行为风险：文档要求 Store/reader 都拒绝 symlink、non-regular 与 oversized 输入，但 A5 只把 8 MiB/regular/no-symlink 约束施加到新编码 temp；A7 的 descriptor/N+1 算法明确属于 Widget reader。实现者仍可为 publisher 的旧 projection 比较保留当前无界 host 读取，在判定 invalid/unknown baseline 前把任意大或路径竞态文件读入内存。
- 证据：CodeGraph 定位的当前 `AppGroupSnapshotStore.read()`（`apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift:264-267`）直接使用 `Data(contentsOf:)`。架构的 `readExisting() -> existing | missing | invalid` 没有 fixed category/读取顺序；Shared/App 验证矩阵也未列 oversized old projection 的 N/N+1 与 unsafe-file 分支。

💡 有界修复：明确 `readExisting()` 复用同一个安全 bounded-byte loader，或逐步写出等价的 `lstat → O_NOFOLLOW open → fstat → N+1 read → post-fstat` 合同。oversized/unsafe/malformed 旧内容统一进入 unknown semantic baseline、不得阻止有效新写；补充 host 侧 N、N+1、增长竞态、symlink/non-regular 和有界分配测试。

### 🟡 建议改进——推荐

无；本轮只记录阻止实现的 generation 与 host 读取边界。

### 🟢 优点

- `AR-R1-F1 -> CLOSED`：双 lane 表覆盖 quota-first、full-vs-quota、quota-vs-quota、manual priority、最多一个 pending、joiner 唤醒与每个 `QuotaOperationID` 的单 owner side-effect 边界。
- `AR-R1-F2 -> CLOSED`：publisher 已区分 `failedBeforeCommit` 与 `indeterminateAfterCommit`，定义 rename commit point、unknown baseline、restart、全 kind recovery 与禁止不可靠 rollback。
- `AR-R1-F3 -> CLOSED`：Widget reader 已固定 8 MiB，定义 N/N+1、descriptor bounded read、增长竞态、unsafe-file 分类、writer 上限与隐私诊断。
- surface provisioning、typed load outcome、timeline 时钟、semantic kind diff 与 native acceptance 边界保持完整，未发现对既有已审 UX 合同的回归。

### 📝 总结

- Finding disposition：`AR-R1-F1`、`AR-R1-F2`、`AR-R1-F3` CLOSED；`AR-R2-F1`、`AR-R2-F2` NEW/OPEN；无外带项。
- Reviewed state：HEAD `8279718ce4cac9cd755cf974c9869719fed3a395`；`docs/topics/desktop-refresh/architecture.md` blob `d7e6f5e2b327ebfac49339df1e66502c67d5602d`；内容指纹 `f47d47b6048f98034fb6be6c2c3673f13a6e9348e1c74213767001b7348c2fe0`，配方为 `SHA-256(head=<HEAD>;document=<blob>)`。
- Reviewer：Codex。
- Method：独立 finding-by-finding 复评；复用相同 HEAD 下未失效的 Round 1 源码与 L0 证据，逐项核对返修候选，并用 workspace-bound CodeGraph 定向确认 publisher/store 的当前消费者与测试边界。未委派。
- Scope：`docs/topics/desktop-refresh/architecture.md`；`tasks.md` 仅作为 Documents 状态权威。未修复文档，未评审实现、任务分解或 final-surface reconciliation。
- Evidence：三项 Round 1 finding 均可从修复后合同精确闭合；generation admission 与 commit fact 在 pre-commit failure 分支互相矛盾；当前 host store 仍以 `Data(contentsOf:)` 无界读取旧 projection，而架构只为 Widget reader 写出 bounded algorithm。
- Residual uncertainty：尚未实现的 actor/scheduler/native behavior 仍须后续测试与原生验收；本轮以可构造合同反例判定 FAIL，不把设计推演当 runtime 证据。
- Completion gate：FAILED。固定 CEv1 gate 对 `desktop-refresh:architecture.md` 和精确 ContentState `urn:agent-deck:content-state:desktop-refresh:architecture:8279718:d7e6f5e` 回查：document checks 与 handoff pass，contract-correct 由 `AR-R2-F1`、`AR-R2-F2` 的 target-bound fail evidence 反证；无缺失、blocked、malformed、失效证据或未决 candidate impact。

三项原发现已经关闭，但两个新缺口仍要求实现者自行发明 stale-write 屏障和 host 安全读取语义，因此当前 architecture Task 仍不能进入提交边界。

### Repair candidate — 2026-09-19

- `AR-R2-F1 -> repaired in candidate.` `highestAcceptedGeneration` 现在是独立的
  admission barrier：`publish()` 在任何 file work、allocation 或 suspension 前同步执行
  `guard generation > barrier` 并推进，且任何失败都不回退。每次 resume 与 rename 前
  必须复核；pre-commit failure 只保留旧 disk/baseline/`lastVerifiedGeneration`，所有旧
  queued/incoming generation 均 superseded，不能写、reload 或清除新失败。验证矩阵包含
  N+1 pre-commit 失败后 N 已排队/新到达，以及 N suspend 后 N+1 admission 的反例。
- `AR-R2-F2 -> repaired in candidate.` 新增 shared
  `AppGroupSnapshotBytes.readBounded`，host `readExisting()` 与 Widget reader 必须复用同一
  8 MiB、lstat→`O_NOFOLLOW`→fstat→N+1 read→post-fstat 合同。host 对 missing、unsafe、
  oversized、malformed、unsupported 旧内容只产生 typed unknown semantic baseline，不
  阻止安全新写；下一成功 all-kind reload。Shared/App 验证矩阵加入 N/N+1、增长竞态、
  path replacement、symlink/non-regular、有界分配和 repair-write 覆盖。
- Repair candidate：HEAD `8279718ce4cac9cd755cf974c9869719fed3a395`；
  `architecture.md` blob `d5ff801efa8b0d15fb50e939d2726dd6284ca58e`；内容指纹
  `ead94b4b6049d07563b6f64c8c0c777eb158a64837a7c60049b0bffb33398f5b`。
- Verification：`bash scripts/check-topic-docs.sh`、`make check-whitespace`、
  `git diff --check` 均 PASS；admission barrier 的同步推进/resume 复核与 shared host/Widget
  bounded reader、typed baseline、交叉测试均可从修复后文档精确定位。生产代码、测试与
  配置未修改。

Round 2 结论仍为 FAIL，Review 矩阵仍未勾选；以上只声明 `AR-R2-F1`、`AR-R2-F2`
返修候选就绪，是否 CLOSED、是否出现相邻回归及新候选门禁由独立复评决定。

## Round 3 — 2026-09-19

## 📋 Desktop Refresh architecture 复评

📊 总体评分：9.4/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无；本轮没有可继续外带的 in-scope finding。

### 🟢 优点

- `AR-R2-F1 -> CLOSED`：`highestAcceptedGeneration` 已成为独立 admission barrier，在任何 file work、allocation、suspension 或 write 前同步推进且永不回退；resume/rename 前复核与 N/N+1 顺序反例共同封闭旧 generation 倒灌和清除较新失败的路径。
- `AR-R2-F2 -> CLOSED`：host `readExisting()` 与 Widget reader 现在被合同强制复用 shared `AppGroupSnapshotBytes.readBounded`，共用 8 MiB、`lstat → O_NOFOLLOW → fstat → N+1 read → post-fstat` 边界；host typed unknown baseline、repair write 和 all-kind recovery 均已定义。
- Round 1 的 `AR-R1-F1`、`AR-R1-F2`、`AR-R1-F3` 闭合状态保持有效；双 lane quota 仲裁、post-commit indeterminate recovery 与 bounded Widget loading 未被本轮修复回归。
- verification matrix 覆盖 admission-order、host/Widget N/N+1、增长与 path replacement 竞态、unsafe file、有界分配、repair write 和 targeted reload，足以指导后续 L3 实现与验收。

### 📝 总结

- Finding disposition：`AR-R1-F1`、`AR-R1-F2`、`AR-R1-F3`、`AR-R2-F1`、`AR-R2-F2` 全部 CLOSED；无 OPEN、regressed、new 或外带项。
- Reviewed state：HEAD `8279718ce4cac9cd755cf974c9869719fed3a395`；`docs/topics/desktop-refresh/architecture.md` blob `d5ff801efa8b0d15fb50e939d2726dd6284ca58e`；内容指纹 `ead94b4b6049d07563b6f64c8c0c777eb158a64837a7c60049b0bffb33398f5b`，配方为 `SHA-256(head=<HEAD>;document=<blob>)`。
- Reviewer：Codex。
- Method：独立 finding-by-finding 复评；复用相同 HEAD 下未失效的源码基线与既有 L0 证据，核对两项返修及相邻 publisher/store/reader/failure/recovery/verification 合同。未委派。
- Scope：`docs/topics/desktop-refresh/architecture.md`；`tasks.md` 仅作为 Documents 状态权威。未评审实现、任务分解或 final-surface reconciliation。
- Evidence：admission barrier 同步推进且 failure 不回退，resume 与 rename 前都有 stale check；shared bounded reader 同时约束 host 与 Widget，typed baseline 与验证矩阵覆盖原反例。`bash scripts/check-topic-docs.sh`、`make check-whitespace`、`git diff --check` 均 PASS。
- Residual uncertainty：尚未实现的 actor、scheduler、storage 与 WidgetKit runtime 行为仍须后续测试和原生验收；本轮 PASS 只表示架构合同可实施且既有 findings 全部闭合。
- Completion gate：VERIFIED。固定 CEv1 gate 对 `desktop-refresh:architecture.md` 和精确 ContentState `urn:agent-deck:content-state:desktop-refresh:architecture:8279718:d5ff801` 回查为 3/3；无缺失、反证、blocked、malformed、失效证据或未决 candidate impact。

架构合同已完整决定 scheduling、coalescing、publication、bounded read、Widget timeline、privacy 与 verification ownership，可进入 final-surface reconciliation；这不批准实现、提交或推送。
