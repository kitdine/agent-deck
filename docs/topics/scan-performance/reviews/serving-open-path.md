---
status: active
topic: scan-performance
subject: serving-open-path
---

# serving-open-path 评审记录

## Round 1 — 2026-10-27

## 📋 实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

#### SP-SOP-R1-F1 — P1：释放 publication hold 会恢复已失效身份的数据

- 位置：`apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift:2037–2075`。
- 状态：OPEN。
- 行为风险：数据库替换、restore 或 epoch/schema 切换后，旧身份的 payload 可重新成为当前数据，并可能重新发布至 Widget；违反本任务的身份失效正确性要求。
- 证据：`installServingPublication` 在同身份且 hold 存在时将 publication 存入 `heldPublication`（2050–2056）；身份变化分支只清空 `servingHeader`、`latestSnapshot` 和可见状态，不清空该暂存值（2063–2075）。`releasePublications` 在计数归零后无条件把暂存值传回安装函数（2037–2041），此时没有 hold，也没有对暂存身份或接收顺序的校验。
- 可确定的调用序列：先安装当日完整 A0；调用 `holdPublications()`；通过 bootstrap publication 回调收到同身份 A1，使其进入暂存；随后收到当日完整 B0（不同 `stateIdentity` 或 `coreEpoch`），B0 绕过 hold 并立即替换 A；调用 `releasePublications()` 后 A1 被取出，再被当成另一轮身份变化安装。最终当前身份从 B 回退到 A。若 B 是跨天的不可用替换，身份清空后仍会被同一当日 A1 恢复。
- 可达性：`performFullRefresh` 的 publication 回调及最终返回值均调用安装函数（2201–2210）；同一刷新内允许多个 publication。测试辅助 `ControlledServingRefresher.publish` 已提供受控中途发布入口（`DesktopRefreshCoordinatorTests.swift:832`）。
- 测试保护：`testHeldPublicationIsInstalledOnRelease` 仅测试同身份暂存与嵌套释放；`testIdentityChangeIsNotHeld` 在身份切换前没有暂存 A1，且释放后未断言身份。因此已有全量通过不排除本缺陷。
- 💡 修复建议：身份失效必须同时废弃旧身份的暂存 publication；释放时确保暂存 publication 仍属于当前有效身份/发布序列。补充 A0 → hold A1 → B0 → release 的组合回归，并覆盖不可用 B、epoch/schema 变化与嵌套 hold；释放后不得恢复 A。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- restore-first 启动顺序、等值写入抑制与 presentation 缓存均针对已定位的展示路径成本。
- 六项已有验证证据绑定的候选指纹与当前产品内容一致；性能报告包含基线与候选统计，未以性能改善替代正确性要求。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra；本轮未参与该候选实现，单审查角色，无委派。
- Method：工作区与内容指纹核验、CodeGraph 调用路径、差异及相邻测试的静态审查、CEv1 精确状态查询。上述缺陷为源码控制流推导，未声称运行了新增回归或真实 App 复现。
- Scope：`scan-performance / serving-open-path` 当前候选；重点为首次打开/选择期间 publication hold 与 serving 身份切换。发现决定性正确性缺陷后结束扩展验证，本轮不宣称覆盖全部候选路径。
- Reviewed state：HEAD `910d76bf6feee20d1446e1633b8dfaae08f5b516`；scoped fingerprint `e366a11ef1333f89f78e05086edcf066fdda1d936003f7484c419c1b6692984b`；ContentState `scan-perf-sop-c11-20261027-state`。
- 指纹复算：取 `git diff --name-only HEAD` 与 `git ls-files --others --exclude-standard` 的路径并集，排除 `docs/`、`apps/macos/.build/`，按字节排序；每行 `path:<git hash-object>`，删除路径写 `path:deleted`；以 LF 连接且末尾无 LF，计算 SHA-256。结果与既有 ContentState 相同。
- Evidence：入口精确 gate 查询返回 VERIFIED，六项已有证据均可适用，ID 前缀为 `scan-perf-sop-c11-20261027-`，后缀为 `open-path-latency`、`startup-restore`、`selection-latency`、`stored-bootstrap`、`first-import`、`serving-correctness`。既有记录报告 Go 全量、相关 race、vet、native XCTest、双架构 Release 构建及 PerfDriver 符号隔离通过；本轮未重复运行这些未变化检查。该事实不覆盖新发现的组合顺序缺陷。
- 性能已有记录摘要（毫秒，S0 → c11，非本轮重测）：P01 首击 median 196.53 → 82.83、p95 363.89 → 103.08；重复 median 64.13 → 59.76；重建中 median 144.21 → 71.65；P02 median 409.98 → 363.06；P04 median 833.19 → 726.86；P11 首批 median 1317.44 → 1213.72，p99 1559.56 → 1594.83，记录判为相近。P03 各维 median 比值为 0.50/0.67/0.67。
- 当前任务矩阵为六项准则；本轮查询确认 `popover-floor` 已不在 requires 集合。本轮未修改准则或历史证据；旧准则描述中的数值门措辞不作为当前验收标准，现行契约以 `tasks.md:13–14,149–188` 为准。
- 完成门禁：FAILED。追加 `scan-perf-sop-r1-20261027-identity-hold-fail` 及其 `satisfies` / `observed_at` 关系后，对上述 WorkUnit/ContentState 重新执行 provider `gate-status.cypher`，`serving-correctness` 返回适用失败证据，其余五项保持通过。旧验证事实保留；本轮没有覆写它们。
- 未修改生产代码、测试或配置；未提交或推送。

### 下一步指令

修复：scan-performance / serving-open-path / SP-SOP-R1-F1

## Repair candidate — 2026-10-27（Round 1 后）

Repairer：Claude Code 主会话（Round 1 前的实现者）；范围为 SP-SOP-R1-F1。Round 1 的评分、结论与 FAILED 门保留；
本节不是 Review Round，不自行声明 PASS，待独立复评确认。未委派。

- SP-SOP-R1-F1 -> repaired in candidate：
  - **复现：** 修复前逻辑下按评审给出的序列（A0 已装 → hold → 同身份 A1 进入暂存 → 不同身份 B0 装入 → release），
    release 后当前身份回到 A1（`held-full`）；B0 为跨天不可用替换时，身份清空后同样被 A1 恢复。
  - **生产：** `EmbeddedHelperRunner.installServingPublication` 在未被暂存、即将真正安装的分支开头清空
    `heldPublication`。任何走到安装的 publication 都晚于暂存值到达并取代它；暂存只发生在同身份且 hold 存在时，
    所以 release 时若仍有暂存值，它必然属于当前身份、且是最新一次同身份 publication。release 逻辑未改。
  - **回归：** `DesktopRefreshCoordinatorTests` 新增三例，共用 A0 → hold → A1 → B0 → release 序列，宿主不重复回调
    final（`publishesFinal: false`），使暂存值确为旧 A1：`core_epoch` 变化；`core_schema` 31 → 30（不兼容）且两层
    嵌套 hold；不同 `state_identity` 且 B0 为前一日（不可用）。断言 release 前后头部均为 B0（不可用时为 nil，
    payload 亦为 nil）。临时去掉修复行时三例都以 `held-full` 被恢复而失败；恢复后通过。`session_schema` 变化
    不可构造：线格式校验把它固定为 1，故改用 core schema。
  - **附带测试修正（验证阻塞，非新发现的产品缺陷）：** 本机现接 3 块显示器，status item 获得内容后会在另外两块
    显示器各建一个 `NSStatusBarWindow`，使实现期新增的
    `testWindowWarmUpRunsOnceWithoutShowingAWindowOrMovingFocus` 的可见窗口集合比较在修复前后都稳定失败
    （诊断确认新增的两个窗口均为 `NSStatusBarWindow`，非 popover 窗口）。该比较改为排除 `NSStatusBarWindow`；
    “暖启动不显示窗口、不移走 key window / first responder”的断言不变。

受影响证据处理（用户 2026-10-27 选择“影响评估沿用”，不重测）：

- **内容状态：** 修复改变 `EmbeddedHelperRunner.swift`、`DesktopRefreshCoordinatorTests.swift`、
  `MenuBarChromeTests.swift` 三个 blob，其余 18 个条目不变。按既有配方复算 scoped fingerprint
  `63f0d3aa99d676f73795ab4b9d636c45cb6c21368d01ab3e395618d32279df08`（HEAD 不变，
  `910d76bf6feee20d1446e1633b8dfaae08f5b516`）；CEv1 ContentState `scan-perf-sop-r1f1-20261027-state`。
- **为何沿用性能证据：** 唯一的产品改动是安装路径上一次指针清空。它只在“hold 期间暂存了同身份 publication、
  随后又安装了另一身份”时改变行为；c11 各批次使用同一冻结语料、同一 state identity，无 epoch/schema 变化，
  不出现该组合，在被测的打开、选择、启动、首批导入路径上只多一次赋值。另两个文件是测试，不进入产物。构建输入、
  语料、S0 基线与环境不变。这是模型评估，交由复评裁定。
- **CEv1：** `change` `scan-perf-sop-r1f1-20261027-change`（c11 状态 → 新状态）对六条
  `scan-perf-sop-c11-20261027-*` 证据各记 `may_invalidate`；五项性能证据各记一条 `impact_assessment`
  （outcome `preserves`）并在新状态下记 roll-up 证据 `scan-perf-sop-r1f1-20261027-{open-path-latency,
  startup-restore,selection-latency,stored-bootstrap,first-import}`；`serving-correctness` 在新状态下以本次全量
  XCTest 直接记为 pass（`scan-perf-sop-r1f1-20261027-serving-correctness`）。原证据与评审的失败证据
  `scan-perf-sop-r1-20261027-identity-hold-fail` 保持原状态不改写。

验证：

- `bash scripts/test-macos-app.sh` 全量隔离 XCTest：TEST SUCCEEDED（含三例新回归与暖启动测试）。Go 源码与 c11
  状态相同，未重跑 Go 测试；未重跑性能批次与 Release 构建。
- **CEv1 gate：** 对 `scan-performance:serving-open-path` / `scan-perf-sop-r1f1-20261027-state` 逐项查询 requires
  集合（六项）：每项均有绑定新状态的 pass 证据，新状态下无失败证据。本地没有规范 `gate-status.cypher`
  文件，使用等价的直接查询。这是证据状态，不代替复评结论。

#### 下一步指令

复评：scan-performance / serving-open-path

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## Round 2 — 2026-10-27

## 📋 SP-SOP-R1-F1 修复复评

📊 总体评分：8/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。SP-SOP-R1-F1 CLOSED：当前安装分支在任何身份失效、日期检查和可见数据更新之前清空 `heldPublication`，原 A0 → hold A1 → B0 → release 的回退路径已关闭。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `EmbeddedHelperRunner.swift:2049–2061` 在同身份暂存分支之外统一清空旧暂存；B0 不可用时也先废弃 A1，避免日期 guard 提前返回后恢复旧身份。`releasePublications` 的嵌套计数与同身份最新值释放行为保持不变。
- `DesktopRefreshCoordinatorTests.swift:94–133` 的三例组合回归覆盖 core epoch、core schema 与嵌套 hold、不同 state identity 且日期不可用；使用 `publishesFinal: false` 防止重复 B 回调掩盖问题，并在释放后断言头部与 payload。修复证据记录了移除修复行后失败、恢复后通过。
- 暖启动测试只排除系统 `NSStatusBarWindow`，仍保留 popover 关闭、key window、first responder、内容窗口集合与单次 preparation 的断言；没有放宽产品窗口检查。

### 📝 总结

- Reviewer：OpenCode / GPT-6 Astra，原独立审查角色；未参与本轮修复，无委派。
- Method / Scope：逐项核验 Round 1 唯一发现及三文件修复增量；检查生产安装/释放路径、组合测试与暖启动测试差异；核验内容指纹、CEv1 影响评估和精确门禁。没有新增阻断发现。
- Finding disposition：SP-SOP-R1-F1 CLOSED；无仍开放、回归或新增发现。Round 1 的历史 FAIL 与原状态失败证据保留。
- Reviewed state：HEAD `910d76bf6feee20d1446e1633b8dfaae08f5b516`；scoped fingerprint `63f0d3aa99d676f73795ab4b9d636c45cb6c21368d01ab3e395618d32279df08`；ContentState `scan-perf-sop-r1f1-20261027-state`。依照 Round 1 配方现场复算，逐路径核对确认只有修复说明中的三个 blob 变化，其余 18 个条目相同。状态文档不参与该产品指纹。
- Evidence reuse：新状态的 `scan-perf-sop-r1f1-20261027-serving-correctness` 记录 `bash scripts/test-macos-app.sh` 隔离全量 XCTest 成功及三例红/绿回归。Go 内容未变化，沿用 c11 的 Go test/race/vet。此次不新增导入、依赖、平台条件或构建符号，沿用既有双架构/Release 隔离检查的适用结论；未声称重新构建了修复后的 Release 产物。
- Performance impact：接受五项 `scan-perf-sop-r1f1-20261027-impact-*` 的 preserves 判断及目标状态 roll-up。修复仅在安装路径增加暂存清空；普通同身份 hold 路径仍提前返回，释放已先清空暂存，批次无身份/epoch/schema 切换，故原测量行为可沿用。测试变化不进入产品。性能数字是 c11 测量经影响评估复用，不是修复后重测结果。
- 完成门禁：VERIFIED。使用已加载的规范 provider `gate-status.cypher`，参数 namespace `github.com/kitdine/agent-deck`、work_unit_id `scan-performance:serving-open-path`、target_content_state `scan-perf-sop-r1f1-20261027-state`。六项准则均有适用 pass，reason 为 null，missing_criteria 与 invalidated_evidence 为空。五项性能证据沿 roll-up 依赖有效，新 correctness 证据直接绑定本状态。
- Gate diagnostics：`unresolved_candidate_impacts` 列出旧 `scan-perf-sop-c11-20261027-serving-correctness`；其 target_matches=false、applicable=false，未被本状态 correctness 证据依赖或复用，故不阻断本次 gate。旧 R1 fail 同样只绑定 c11。本轮保留这些历史事实，没有借简化查询忽略适用失败。
- 验证限制：本轮复用规范 gate 返回的既有验证，没有重复运行 XCTest、Go 或性能批次；静态复评不能等同于新一轮真实 App 测量。
- 层级：`tasks.md` 确认本任务属于未完成的 scan-performance topic，Tasks 4–8 尚未完成；CEv1 当前未建该任务的 contains 父边，不据此宣称 topic 完成或生成父级门禁。下一任务 `ad-scan-performance-parallel-source-pipeline` 为 open，前置 baseline-remeasure 已 closed。

Task checkpoint：`ad-scan-performance-serving-open-path`；content_state `scan-perf-sop-r1f1-20261027-state`；fingerprint `63f0d3aa99d676f73795ab4b9d636c45cb6c21368d01ab3e395618d32279df08`；gate VERIFIED。

提交建议：经用户授权后，将 serving-open-path 实现、测试、compare 工具迁移及本任务的 tasks.md / 评审记录作为一个任务边界提交；提交前按路径和 hunk 核验范围与贡献者，保留其他工作。

推送建议：目标 `origin/feature/scan-performance`；需用户另行授权、任务提交完成，并核对待推送完整提交范围及签名。当前分支另有 14 个本地领先提交，不能将本任务推送建议视为对整个范围的授权。

### 下一步指令

开发：scan-performance / parallel-source-pipeline

WORKFLOW_WORKSPACE: agent-deck.scan-performance

## PR #61 修复交接 — 2026-10-31

来源：Codex 对 `3e07c892f34d775e60919774f49ae7c85afb755f` 的
[凭据检查发现](https://github.com/kitdine/agent-deck/pull/61#discussion_r4431001383)。
用户授权本 PR 的修复、提交与推送；全部 Actions 成功后再请求 Codex Code Review。

完整 serving 发布原先没有给 `desktop.Service` 传 Vault，导致已有加密凭据时
doctor 报告 `credential_key_missing`。普通轮次与分批 serving 轮次现统一使用带
生产凭据 Vault 的 builder；测试注入合成机器身份与临时密钥。
`TestServingPublicationsUseTheCredentialVault` 覆盖普通/serving 两条路径各两轮，
验证发布中的 `provider_credential_key` 为 `ok`，定向 Go 回归通过。

原 CI 的文档空行失败使 documentation 与 verify 同时失败，已删除
`incremental-projections.md` 的尾部空行。日期表面 XCTest 的等待断言补充调用点位置，
单独运行原失败用例通过，保留原条件与时限。组合验证以本轮 CI 结果继续核对。

完整 `make test-macos-app` 隔离验证通过：Shared 121、App 165（1 项既有跳过）、
Widget 51 项，0 failure；原日期用例在整套测试中通过。文档结构、document set、
review-record 与 whitespace 检查通过；远端以修复提交的 Actions 结果继续核对。

新候选等待 PR Code Review；此前各轮 PASS 与精确门禁只对应其原内容身份。

## sql.DB 连接池修复交接 — 2026-10-31

用户要求继续修复本机完整 scanruntime race 的 200 ms 读取超时，并将 sql.DB 处理单独
提交，以便独立评审。此前 PR 修复提交为 `c12cd86f984c6a91ae461996da1b33175fbe5d81`，
保留该提交，不改写历史。

### 根因与改动

`OpenServingGenerationReader` 每次新建 `sql.DB`，`PingContext` 初始化首个连接，
请求结束就销毁池；普通只读 Store 也逐次建池。分段诊断（同机 Go 1.27.3、race、合成库）
测得 core 连接打开 100–209 ms，而 generation 查询各约 1 ms，重复连接初始化耗尽
200 ms 期限。该期限及原失败断言保留。

现按物理数据库文件身份与配置复用进程只读池，最多驻留 32 个；请求持有独立 lease
和 context。首次初始化合并，等待者取消不影响创建者，创建者取消也不污染其他等待者。
20 ms identity 与 5000 ms 普通只读 busy timeout 分开；文件替换、空闲淘汰与进程退出
均按租借生命周期释放连接。每次仍查当前 schema/epoch/revision，并校验文件身份。
写 Store 完成初始化时预热首个 identity 连接，不把冷连接成本逐次带入读取期限。

常驻只读连接会改变 SQLite 的最后连接关闭行为，故 writer 关闭时显式 PASSIVE
checkpoint，保留主库副本可见性；重复关闭保持幂等。CLI 正常退出释放进程池。
Snapshot 合成夹具在最后 writer 关闭前排空池，以建立原用例要求的完全关闭输入；
主库 hash 不变、sidecar 权限与前后状态的原断言全部保留。

### 已取得的直接验证

- 连接复用、活跃旧读者与文件替换、冷初始化合并、双向取消隔离、容量/淘汰/退出回收，
  及独立读取 context 的定向 race 回归通过。五次 primed 双库 identity 读取为
  1.37–2.77 ms；这是上述合成库的诊断对照，不替代 topic 的完整性能报告。
- 连接池候选下 `scripts/run-go-test.sh -race ./internal/scanruntime` 全包通过，
  原先超时的用例通过。200 ms 期限及不续期/取消检查通过。
- 新增主库副本回归在缺少关闭 checkpoint 时确定失败（副本缺少 schema_metadata）；
  修复后该回归与原 IntegrityCheck 用例在 race 下连续三次通过。
- Snapshot 的完全关闭输入夹具修正后，原完整快照用例在 race 下连续三次通过；
  没有删除或放宽原有断言。最终全量及跨包 race 结果随本修复提交收口。

本 SQL 候选等待独立 Code Review；此前历史失败及旧内容门禁不改写。

最终本机收口：Go 1.27.3，`scripts/run-go-test.sh -p 1 ./...` 全量通过；
`scripts/run-go-test.sh -race -p 1 ./internal/store ./internal/telemetry ./internal/scanruntime ./internal/desktop`
四包完整通过（含 scanruntime 全包与原超时用例），无 race 报告；`make vet` 通过。
原始失败观察保留为根因与修复输入，不将其改写为通过。SQL 修复按用户要求独立提交，
远端 Actions 与新 Code Review 继续针对最终提交核对。

### CI 日期用例定位补充

PR CI [41189217902](https://github.com/kitdine/agent-deck/actions/runs/41189217902) 的
desktop job 在 `MenuBarViewModelTests.swift:168` 失败：五项日期事件均是 re-arm today
后的正向显示条件不满足，原过期后的检查没有对应失败。本机单独及全套 XCTest 的通过
不能替代此 runner 的失败。后续只补充该断言的合成金额、菜单栏/模型值、AX 文本和窗口
状态诊断，保留原条件与 2 秒期限，以区分显示恢复与测试获取内容的差异。

第二次 CI [41190042429](https://github.com/kitdine/agent-deck/actions/runs/41190042429)
对 `7aaefa2` 给出五次一致诊断：expected、button、hero 都是 `$1.44`，
active/key/visible/unoccluded 均为 true，只有 accessibility 数组为空。
辅助函数要求节点声明完整 `NSAccessibility` 协议；AppKit 还允许 role-specific 和
informal AX 节点，旧筛选会丢弃这些合法节点。现按节点支持的公开 AX getter/attribute
读取文本，并保留 attributed text 的字符串内容；仍读取实际 hosting view，原金额与
日期失效条件、2 秒期限均保留。新增不声明完整协议的 AX 节点回归，连同原日期渲染
用例在本机通过；远端继续验证该兼容性修复。
