---
status: active
topic: auxiliary-session-classification
subject: settings-control
---

# settings-control 评审记录

## Round 1 — 2026-11-07

## 📋 settings-control 独立评审

📊 综合评分：7/10

✅ 结论：FAIL

- Reviewer：Codex，独立于本候选实现者 Claude Code。
- Method：专题工作区 CodeGraph 定位、受影响源码与测试审查、直接编译实际控制器的隔离异步调度复现、
  已有 XCTest 结果读回。未委派子代理。
- Scope：任务 6 的 11 个产品/测试路径，重点核对 App 持有的操作生命周期、确认值/草稿、
  CLI bridge、回执恢复与 Settings 接线。得到决定性阻断证据后停止广泛验证，
  本轮不声称剩余全部验收项已经通过。
- Reviewed state：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093`；
  scoped SHA-256 `891355b0152d64ea285772dfd8ed1d55f76ecc8b57375ebe54587f68e4b071fe`。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:891355b0152d64ea`。
- Completion gate：FAILED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification`，
  `feature/auxiliary-session-classification`。

### 🔴 严重问题 — 必须修复

#### ASC-SC-R1-F1 — P2：窗口重开期间的旧读取会覆盖刚保存的默认值

- 位置：[SessionOriginSettingsController.swift:270](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L270)、
  [同文件:494](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L494)。
  原生入口 [MenuBarItemController.swift:527](../../../../apps/macos/AgentDeckApp/MenuBarItemController.swift#L527)
  在每次显示 Settings 时异步调用 `open()`。
- 触发：已有确认值 Work/revision 0；关闭后重开窗口，新的只读请求取得旧值但尚未返回；
  此时本组仍允许保存。用户保存 All，写入回执确认 revision 1；随后旧读取返回。
- 行为风险：界面把已确认的 All/revision 1 和草稿退回 Work/revision 0，而持久值仍为 All/revision 1。
  “已保存”文案与实际持久值不一致，下一次操作还会使用旧 revision 发出请求。
  本复现未观察到持久值回滚；核心 CAS 仍保护持久写入，不能把 UI 失真描述成数据库丢更新。
- 证据：`readDefault()` 的 `valuesGeneration` 仅在另一读取启动时增加。
  `accept()` 接收成功的 scope 写入时直接更新确认值，却不使更早读取的代次失效。
  因此挂起的旧读取仍通过第 270 行检查，并在第 278 行覆盖新确认值。
  使用真实候选控制器、受控 transport 和 checked continuation 的运行输出：

  ```text
  F1 writesEnabledWhileReopenReadPending=true
  F1 beforeDelayedRead=all/1
  F1 afterDelayedRead=work/0 stored=all/1 oracle_pass=false
  ```

- 复现方法：
  1. 初次 `open()` 确认 Work/0，调用 `windowClosed()`。
  2. 让下一次 `readDefaultScope()` 捕获 Work/0，并用 continuation 挂起其返回；
     在主 actor 启动第二次 `open()`。
  3. 确认挂起期间 `writesEnabled == true`；选择 All 并执行 `saveDefault()`。
     transport 持久状态和成功回执均为 All/1，控制器此时也为 All/1。
  4. 放行旧读取，等待第二次 `open()` 结束；比较控制器与 transport 的持久状态，
     得到上述 Work/0 对 All/1 的不一致。
- 测试保护：现有测试覆盖保存中关闭/重开、unknown 重试、冲突和 no_change，
  但默认读取立即返回，没有覆盖“重开读取先开始、保存先完成、旧读取后返回”的交错。
- 合同：[Settings 默认范围与生命周期](../ux/settings.md#默认范围)要求展示最近确认值，
  关闭/重开不制造假成功；[任务 6](../tasks.md#6-settings-control)要求确认值与草稿分离及生命周期/并发验证。
- Disposition：OPEN；修复载体为同一任务 `ad-auxiliary-session-classification-settings-control`。

💡 修复建议：让成功写入使更早启动的默认读取失效，或在重新确认期间禁止竞争保存；
提交读取结果前检查它仍属于当前确认状态。加入可控挂起读取的回归用例，
断言成功保存后的确认值、草稿与下一次 expected_revision 不被旧响应退回。
修复范围仅为这条读写交错及相关生命周期保护。

### 🟡 改进建议 — 推荐

无独立建议项；本轮记录的唯一发现必须关闭后才能 PASS。

### 🟢 优点

- pending operation 由 App controller 持有，包含 request ID、负载和 expected revision，
  重试使用原操作身份；窗口关闭只清理局部草稿与查询。
- bridge 对已启动但结果无法确认的 helper 执行采用 unknown，
  读回回执和普通失败有不同的恢复入口。
- 既有测试与交接明确区分合成控制器/渲染证据和未执行的原生验收。

### 📝 总结

本轮为 FAIL：已在实际控制器上复现旧读取覆盖新保存值的竞态，
并非从函数命名或缺少测试推断缺陷。生产代码、测试、配置均保持只读。

#### 内容身份

指纹算法：UTF-8 `head=<HEAD>\n` 加按路径字节序排列的
`<path>=<git hash-object blob>\n`，末尾保留换行，再取 SHA-256。
本轮 11 个路径与开发交接完全一致：

- `apps/macos/AgentDeck.xcodeproj/project.pbxproj`
- `apps/macos/AgentDeckApp/AgentDeckApp.swift`
- `apps/macos/AgentDeckApp/DesktopCopy.swift`
- `apps/macos/AgentDeckApp/Localizable.xcstrings`
- `apps/macos/AgentDeckApp/MenuBarItemController.swift`
- `apps/macos/AgentDeckApp/SessionOriginSettingsController.swift`
- `apps/macos/AgentDeckApp/SettingsWindowView.swift`
- `apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift`
- `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift`
- `apps/macos/AgentDeckShared/SessionOriginTransport.swift`
- `apps/macos/AgentDeckTests/SessionOriginTransportTests.swift`

控制器 blob：`1128b7d06c0bdb08bab83fd1129d419a7cc24dba`。
任务状态与本评审记录不属于产品候选指纹；它们的同步不重标产品证据。

#### Evidence

- 独立复现直接编译候选 `SessionOriginSettingsController.swift`，保留实际
  Observation/MainActor 与状态机代码，链接候选构建的 `AgentDeckShared.framework`。
  仅文案函数使用无行为的 shim；传输使用合成 actor，读返回由 checked continuation 明确控制，
  不依赖 sleep 或随机调度，不读写真实偏好、会话或数据库。
- 工具链：Xcode 26.4 / 17E192，Apple Swift 6.3
  (`swiftlang-6.3.0.123.5 clang-2100.0.123.102`)，x86_64；
  XCTest 结果记录的主机为 macOS 26.7.3 / 25G416。
- 现有最终 macOS suite 通过 `xcrun xcresulttool get test-results summary --path <result-bundle>`
  直接读回：total 393，passed 392，skipped 1，failed 0，结果 Passed。
  该结果与开发交接的 App 209（含 1 跳过）、Shared 133、Widget 51 一致；
  候选指纹未变，未因进入评审重跑整套测试。
- 本轮没有把已有 suite 的通过解释为本次交错已被覆盖。
  临时探针与构建产物不是持久引用，必要输入、调度、观测和限制均已记录于上文。
- L0 结果：`python3 scripts/check-review-records.py --base HEAD docs/topics/auxiliary-session-classification/reviews/settings-control.md`、
  `python3 scripts/check-docs.py`（默认工作树模式，含 `git diff --check`）及 `make check-whitespace` 均通过。
  这些检查只验证结构/链接/格式，不颁发语义 PASS 或 CE VERIFIED。

#### 门禁与未完成范围

初次查询前，提供方可用但本 WorkUnit 不存在。依据已批准任务 6 建立
WorkUnit、候选 ContentState 和 10 条必需 criterion；12 个节点逐项读回一致，
10 条 requires 关系预检通过并由首次 gate 查询确认。入口 gate 为 NOT_VERIFIED，
没有历史证据被重新标记为本候选结果。

本轮追加 3 条 evidence，逐项读回一致；6 条关系预检通过，写入后 gate 确认全部适用于该目标。
最终 gate 为 FAILED：confirmed-default 与 review 有当前有效失败证据，validation 复用的测试结果通过；
其余 7 条 criterion 尚无完整验收证据。invalidated_evidence 与 unresolved_candidate_impacts 均为空。
证据 ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r1-20261107-f8gUd1:`，
后缀分别为 `confirmed-default`、`review`、`validation`。

本轮只记录有直接依据的失败和已核实的测试事实；其余未覆盖项保持开放。
原生 Tab/焦点、VoiceOver、Dynamic Type、应用重启读回及并发 CLI 原生交互仍缺独立验收，
不以此次隔离调度复现或现有 suite 补足这些要求。
任务 7–9 尚未完成，本轮不跨 Topic 完成边界。

下一步指令：`修复：auxiliary-session-classification / reviews/settings-control.md / ASC-SC-R1-F1`

## Repair 1 — 2026-11-07

- Repairer：claude-code（本任务实现者，非独立评审）。范围：仅 ASC-SC-R1-F1 及评审建议所指的相关生命周期保护；未处理未登记问题。
- 根因确认：成功写入只更新确认值，没有让写入之前已发出的读取失效。默认读取的代次只在另一次默认读取开始时推进，
  因此重开窗口时的旧读取仍通过检查。同一 `open()` 的覆盖列表读取完全没有代次检查。
  单会话查询只核对目标，不核对写入。三条读取路径属于同一类交错：读取先开始，写入先确认，旧读取后返回。

**ASC-SC-R1-F1 → 修复，待复评。**

- 变更（[SessionOriginSettingsController.swift](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift)）：
  - 改为按数据各自计代：默认值、覆盖列表、单会话查询各一个代次，来源状态沿用原有代次。
    读取开始时记下代次，返回时只有代次未变才应用。
  - 新确认的 scope 写入在采用回执值前推进默认代次，此前发出的默认读取全部作废。
  - 覆盖写入确认后照旧重新读取列表；列表读取现在有代次，较早的列表读取不能覆盖新列表。
  - 查询与 `findSession` 合并为 `refreshLookup`，编辑输入、新的查询或针对同一会话的已确认写入都会使旧答复作废。
    新确认的覆盖写入在用回执更新结果时推进查询代次；若该会话仍在查询中，则在写入后重新查询一次。
  - 重放或读回的回执照旧重新读取当前值。写入之后才开始的读取不受影响，仍会显示其他来源（如 CLI）作出的变更。
- 回归（[SessionOriginSettingsControllerTests.swift](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift)）：
  测试 transport 新增一次性挂起读取：读取在被调用时取答复，放行后才返回，不依赖 sleep 或调度时序。
  - `testAReopenReadThatPredatesASaveCannotRollTheConfirmedDefaultBack`：评审的复现步骤。重开读取取得 Work/0 后挂起，
    其间保存 All 得到 All/1，放行旧读取后确认值与草稿仍为 All/1，下一次保存以 revision 1 为基准；
    写入之后才开始的读取仍能显示外部改成的 Auxiliary/7。
  - `testAReopenListReadThatPredatesARemovalCannotRestoreTheRow`：重开时的列表读取早于撤销，返回后不会让已撤销的覆盖重新出现。
  - `testALookupThatPredatesASaveCannotRestoreTheOlderOverride`：撤销后的“查询当前分类”早于新的设置写入，
    返回后不会恢复撤销状态，下一次保存以 revision 4 为基准。
- 修复前复现：三项新测试在评审候选控制器（blob `1128b7d06c0bdb08bab83fd1129d419a7cc24dba`）上全部失败，且只有这三项失败。
  - 默认：确认值为 work/0 而非 all/1，草稿为 work，后续 expected revision 为 [0, 0]，与评审观测一致。
  - 列表：已撤销的 archived-1 重新出现。
  - 查询：覆盖为 nil、revision 为 3，后续 expected revision 为 [2, 3, 3]。
- 验证：`scripts/test-macos-app.sh`（隔离 HOME）TEST SUCCEEDED：App 212 项（含 3 项新回归，1 项为既有跳过）、
  Shared 133 项、Widget 51 项，0 失败。`make check-whitespace`、本记录检查与文档结构检查通过。
  本次只改控制器与其测试，Go、CLI bridge、视图与文案未改。
- 候选：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093` + 同一 11 个路径，摘要
  `21bdec01283376983147c1dc81532434641aaac8a5569ac5b7de6fad9354d4d4`（算法同 Round 1）。相对 Round 1 变化的 blob：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | `6ab07c75bac8efffe87e4cc6c916acc0735c2e2d` |
  | apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | `5d3b220d2dfe5e204654e7076b3be4798b40c48c` |

  其余 9 个路径 blob 与 Round 1 相同。
- CEv1：Repair 不宣称门禁完成，未写入新证据；Round 1 的 FAILED 证据绑定旧候选，新候选证据由复评按上述摘要绑定。
  原生 Tab/焦点、VoiceOver、Dynamic Type、应用重启读回及并发 CLI 原生交互仍未验收。

修复后状态：ASC-SC-R1-F1 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 2 — 2026-11-07

## 📋 settings-control 修复范围独立复评

📊 综合评分：9/10（本轮修复范围）

✅ 结论：PASS

- Reviewer：Codex，独立于 Repair 1 实现者 Claude Code。
- Method：复用本会话已加载的项目/证据规则；核对候选与修复交接，
  经本工作区 CodeGraph 定位后检查未返回的写入确认与测试源码，
  直接读回修复前后 XCTest 结果及三项指定测试的结果节点。未重跑同候选完整 suite。
- Scope：ASC-SC-R1-F1，以及 Repair 1 改动的默认值、覆盖列表、单会话查询读写交错。
  本轮 PASS 仅关闭该发现及受影响路径，不追认 Round 1 因阻断而停止的整任务剩余审查。
- Reviewed state：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093`；
  11 路径 scoped SHA-256 `21bdec01283376983147c1dc81532434641aaac8a5569ac5b7de6fad9354d4d4`，算法与 Round 1 相同。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:21bdec0128337698:acceptance:20f6c1bae556bddc`。
- Completion gate：NOT_VERIFIED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification`，
  `feature/auxiliary-session-classification`。
- Repair handoff：Beads comment `c2cf4e54-a2ed-5aed-bac5-d4ca6d6c7ffa`。

### 🔴 严重问题 — 必须修复

无。本轮范围内无仍开放、回归或新增的阻断发现。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

**ASC-SC-R1-F1 → CLOSED。**

- [控制器:516](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L516)
  在采用新确认的默认写入回执前推进 `defaultGeneration`；
  [读取端:285](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L285)
  丢弃代次不符的旧结果。原先 Work/0 读取不能再覆盖 All/1 的成功保存。
- 新增回归 [测试:436](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift#L436)
  覆盖原复现的完整交错，检查确认值、草稿以及下次写入的 expected revision 为 [0, 1]。
  随后启动的独立读取仍能接收外部 Auxiliary/7，避免通过冻结所有读取掩盖竞态。
- 同类保护覆盖列表与单会话查询：列表读取按代次丢弃旧页，写入后重新读取当前列表；
  查询同时检查代次和当前目标，确认写入会推进相应查询代次。
  两项新增回归检查撤销后旧列表不恢复记录、旧查询不恢复旧覆盖，
  后续覆盖写入使用 expected revision [2, 3, 4]。
- 测试 transport 在挂起前捕获答复，并用 checked continuation 放行，
  因而能明确控制“读取先启动、写入先确认、旧读取后返回”的顺序。

### 📝 总结

本轮复评 PASS，唯一历史发现 ASC-SC-R1-F1 已关闭。没有在本轮修改产品、测试或配置，
没有新增 Git 交付。整任务尚未完成；本轮限定 PASS 不代替剩余整体验收，
Tasks 矩阵的整任务 Review 仍未勾选，Beads 保持 in_review。

#### 用户明确调整的验收范围

用户在本轮明确指示：“原生键盘／辅助功能 不再验证 跳过”。

- 不再要求本任务的原生键盘导航、键盘焦点及辅助功能验收（含 VoiceOver/AX、Dynamic Type）。
- 这是用户明确排除的范围，不是测试通过、自动豁免或未记录的缺证据。
- 应用重启读回、并发 CLI 原生交互、分页功能、中英文/主题/窄布局，以及剩余完整候选审查继续保留。
- 原有 native-ux criterion 的适用范围按本决定排除上述部分；
  其他部分仍需证据，未将整个 criterion 标成通过。
- 以新 ContentState 的 acceptance_scope_json 与上述摘要绑定该决定；
  原候选 ContentState、Round 1 失败事实和原验收边界保持历史原样。
  摘要算法为本决定规范 JSON（UTF-8、键排序、无额外空白）的 SHA-256。

#### Evidence

- 11 路径指纹与 Repair 1 交接一致，仅两个 blob 改变：

  | 路径 | 本轮 blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | `6ab07c75bac8efffe87e4cc6c916acc0735c2e2d` |
  | apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | `5d3b220d2dfe5e204654e7076b3be4798b40c48c` |

  其余 9 路径与 Round 1 相同。
- 用 `xcrun xcresulttool get test-results summary --path <result-bundle>` 读回 Repair 1 的现有结果：
  旧控制器对应结果 total 396、passed 392、failed 3、skipped 1；
  失败恰为上述三个新回归，分别观察到 Work/0 覆盖 All/1、已撤销记录重现、查询恢复 nil 覆盖。
  修复候选结果 total 396、passed 395、failed 0、skipped 1。
- 用 `xcrun xcresulttool get test-results tests --path <result-bundle>` 逐项核实
  `testAReopenReadThatPredatesASaveCannotRollTheConfirmedDefaultBack`、
  `testAReopenListReadThatPredatesARemovalCannotRestoreTheRow`、
  `testALookupThatPredatesASaveCannotRestoreTheOlderOverride` 均为 Passed。
- 结果环境：macOS 26.7.3 / 25G416、x86_64。测试运行由 Repair 1 产生；
  本轮独立核对其事实、内容绑定和断言保护，未将修复者自述当作独立结论。
  这些是合成传输的实际控制器回归和现有 subsystem suite，
  不声称覆盖应用重启或真实 CLI 并发交互。
- 临时 XCTest bundle 不作为持久引用；必要命令、计数、失败信息、源码和内容身份已在本节记录。
- L0 检查通过：`python3 scripts/check-review-records.py --base HEAD docs/topics/auxiliary-session-classification/reviews/settings-control.md`、
  `python3 scripts/check-docs.py`（含 `git diff --check`）、`make check-whitespace`。
  这些仅检查结构、链接和格式，不认证整任务 Review 或 CE VERIFIED。

#### 完成门禁与精确剩余项

已登记 confirmed-default、validation 的当前候选通过证据；
review 记录本轮限定复评 PASS，但整任务审查尚未完成，该 criterion 为 not_verified。
native-ux 以 not_verified 记录用户排除决定及剩余范围，不把跳过的检查记作通过。
本轮新增带验收范围的 ContentState 与 4 条 evidence，5 个节点逐项读回一致；
8 条关系预检通过并写入，gate 确认适用于当前目标。
最终 gate 为 NOT_VERIFIED：2/10 条 criterion 有通过证据，剩余 8 条未完成；
没有适用于本目标的失败证据，invalidated_evidence 与 unresolved_candidate_impacts 均为空。
证据 ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r2-20261107-hQO_Lqq:`；
后缀为 `confirmed-default`、`validation`、`review`、`native-ux`。

剩余任务级 criterion：
`exact-query-privacy`、`override-list-clear`、`mutation-recovery`、
`publication-default`、`source-status`、`lifecycle-concurrency`、
`native-ux`（仅本轮决定后仍适用的部分）、`review`（完整候选剩余审查）。
后续仅补这些缺口并对精确候选查询 gate，不重做已关闭发现或相同内容的有效测试。

Topic 仍有任务 7–9，未到 containing-unit 完成边界。

Task checkpoint：ad-auxiliary-session-classification-settings-control；candidate=21bdec01283376983147c1dc81532434641aaac8a5569ac5b7de6fad9354d4d4；acceptance=20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1；gate=NOT_VERIFIED
提交建议：门禁 VERIFIED 后，在补齐上述剩余任务验收与完整候选审查的前提下，按任务 6 的 11 个产品/测试路径及 tasks.md、reviews/settings-control.md 形成提交边界；需另行提交授权。
推送建议：门禁 VERIFIED 后，先完成上述缺口、授权提交及提交对象/签名/必要检查核验，再单独授权推送至 origin 的 feature/auxiliary-session-classification；远端交付状态届时核对。

下一步指令：补齐 auxiliary-session-classification / settings-control 的上述 8 项剩余任务门禁证据与完整候选审查，按用户决定跳过原生键盘和辅助功能验收，再查询精确候选 Task 门禁。

## Round 3 — 2026-11-07

## 📋 settings-control 完整候选审查与剩余门禁验收

📊 综合评分：9/10

✅ 结论：PASS

- Reviewer：Codex，独立于本候选实现与 Repair 1。
- Authority：用户明确要求补齐剩余 8 项门禁证据和完整候选审查；沿用本任务原生键盘/辅助功能排除。
- Method：完整任务合同与 11 路径候选审查、已有精确内容测试复用、隔离真实 App/CLI 原生验收、
  临时副本 helper 边界故障注入、实际回执与只读存储核对。未委派代理，未修改产品/测试/配置。
- Scope：任务 6 的完整候选及全部适用验收；不只复查 ASC-SC-R1-F1。
- Reviewed state：HEAD `27a12fc3c811d176cbd17fdf8df1293b20eca093`；
  11 路径 SHA-256 `21bdec01283376983147c1dc81532434641aaac8a5569ac5b7de6fad9354d4d4`，与 Round 2 完全一致。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`，排除范围不变。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:21bdec0128337698:acceptance:20f6c1bae556bddc`。
- Completion gate：VERIFIED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。

### 🔴 严重问题 — 必须修复

无。ASC-SC-R1-F1 保持 CLOSED，本轮完整候选审查未发现新的阻断问题。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- 真实原生窗口与实际 helper 的并发冲突、重启读回和未知结果恢复均保持正确的目标、请求 ID 和版本。
- 排除目标仍可撤销其覆盖；查询维持不可见边界，clear 回执没有伪造当前分类。
- 已保存状态和派生发布等待分开；原生界面在 publication pending 时保留已确认值。

### 📝 总结

完整任务审查 PASS，原生验收已按批准的排除范围完成。
原生键盘导航/键盘焦点、VoiceOver/AX 和 Dynamic Type 是用户明确排除项，不计作测试通过。
Task 7 的 App Group 多组合 summary 与真实 WidgetKit 消费链、Task 8 的恢复/备份、
Task 9 的联合/性能验收仍属于各自任务，本轮不宣称这些后续任务完成。

#### 原生验收方法与可重复步骤

使用本候选 Debug App 的 `AGENTDECK_TEST_HOME`、`AGENTDECK_TEST_WINDOW=1`、
`AGENTDECK_TEST_SETTINGS_WINDOW=1` 入口，HOME 与 CFFIXED_USER_HOME 指向新建的
`agentdeck-menubar-acceptance.*` 临时目录；host 的自动采集启动在本隔离验收中关闭。
所有 UI 动作通过原生 App 控件执行；使用 UI 自动化的观察接口不等于执行被排除的辅助功能验收。

真实内嵌 helper 均显式使用 `--state-dir <isolated-home>/.agentdeck --format json`。
合成会话沿用 `cmd/agentdeck/session_origin_test.go:writeScopedSignalSession` 的 Claude JSONL 格式，
21 个独立 ID（含保留子命令同名 ID `list`），执行 `session scan` 得到 21 sources / 42 documents，
再逐条通过批准的 classify --set 接口创建 revision 1 的 Auxiliary 覆盖。

| 场景 | 直接观测 |
| --- | --- |
| 默认保存 | 原生 UI 保存 All/1，真实 `session scope get` 确认为 All/1；publication pending 不回滚默认。 |
| 并发 CLI | 窗口仍持有 All/1 时，CLI 以 expected revision 1 保存 Work/2。旧窗口保存被 origin_conflict 拒绝，竞争控件锁定；Read latest values 后 UI 确认 Work/2 并解锁，没有自动重放旧草稿。 |
| 应用重启 | 正常退出后重启同一候选，原生 UI 读回 Work/2 与 21 条持久覆盖。 |
| 分页 | 原生第 1 页 20 条，第 2 页 1 条；总页数和当前页一致。 |
| 排除边界与撤销 | CLI 排除 native-review-19 后，精确 classify 返回 session_not_found；原生列表撤销成功，回执 revision 2、classification_reason=not_queried，20 条剩余项回到第 1 页，分页控件消失。 |
| 精确查询 | 原生客户端 Claude、ID list 返回正确单会话，automatic=Unknown、effective=Auxiliary、override revision=1；未将 ID 解释成 list 子命令。 |
| 输入变更与隐私 | 改为被排除 ID 后旧详情/可保存目标立即清除；再次查询显示统一“未找到可见会话”，没有分类或来源正文。 |
| 源数据保留 | 验收结束逐字节核对全部 21 个合成源文件均与生成内容一致，包括被排除并撤销覆盖的目标。 |
| 文案、主题和窗口 | 目视检查英文浅色与中文深色原生窗口；常规 460pt 窗口纵向滚动、关闭入口可达、状态与只读来源组完整。320/460pt、中英、正常/未知/读取失败的 12 组原生渲染复用同候选通过的边界断言。 |

#### 原生未知结果恢复

临时 App 副本的主可执行文件与 debug library 均与原候选逐字节摘要相同，
只替换该临时副本的 helper 入口作一次性故障注入；原始构建和仓库不变。
正常路径调用实际内嵌 Go helper、真实临时 SQLite。以下是受控故障的原生恢复证据，
不是未经注入的随机崩溃或网络故障证据：

1. **未提交、无回执**：wrapper 在首次 scope set 调用真实 helper 前返回非零且不给 JSON。
   原生 UI 进入 unknown，保留 Work/2，锁住竞争写入。关闭并重开 Settings 后，
   pending 仍为 Auxiliary、expected revision 2、同一 request ID。
   Read request receipt 返回 unknown，UI 不把它当作未保存，而是提供同 ID/同操作重试。
   显式重试后真实 helper 保存 Auxiliary/3，UI 解锁。
   请求 `settings-c5a840cf-6a92-4309-9b83-443e3aec9317` 的真实回执为 saved/Auxiliary/3，
   origin_mutations 中此 ID 只有 1 行；首次故障捕获的参数与恢复目标/版本一致。
2. **已经提交、确认丢失**：wrapper 让实际 helper 保存 All/4，再丢弃其 stdout 并返回非零。
   原生 UI 先保持 unknown 与上次确认值，不声称保存完成；读取原请求回执后显示 All/4 并解锁。
   请求 `settings-3ad6d438-0853-4e1d-92db-d2be4d7857be` 的真实回执为 saved/All/4，
   该 ID 也仅有 1 行。后续 scope get 再次确认为 All/4。
3. 两条路径均没有产生替代请求 ID、盲目覆盖外部修改或把 publication pending 当作持久失败。
   原生 App 均已正常退出，隔离进程与材料在事实收录后清理；本记录不依赖临时原件。

#### 剩余 criterion 的核对

| Criterion | 本轮证据与结论 |
| --- | --- |
| exact-query-privacy | 原生保留字 ID、精确 client/ID、编辑清目标与排除边界；bridge 的 --id= 参数和值/目标校验；PASS。 |
| override-list-clear | 原生 20+1 分页与 excluded clear；真实 clear 回执 not_queried；未匹配撤销、旧回执不删除更新覆盖的同候选回归；PASS。 |
| mutation-recovery | 上述原生 before/after commit 故障恢复、同 ID 重试、回执唯一性和 CLI conflict；saved+权限警告/no_change 的同候选回归；PASS。 |
| publication-default | Settings 只请求发布、不写当前 Popover selection；默认发布 revision 检查、no_change 不触发发布、发布失败不回滚；每次 Popover 打开读取已确认默认、当前临时选择不被新发布重置的回归；PASS。Task 7 承接实际 Widget summary 链。 |
| source-status | 实际 origin-source status 的固定两行与原生显示一致；只读桥接验证 source/kind/数量，未知 reason 采用通用文案；Codex 五态、claude-mem 安全原因和 core 不可读/loadFailed fixture 检查；PASS。 |
| lifecycle-concurrency | 真实 App 重启、Settings 关闭/重开、并发 CLI、原生 unknown/conflict 恢复及重复请求；PASS。 |
| native-ux | 原生窗口/分页/文案/主题与同候选 12 组窄/常规渲染；按 Round 2 的用户决定排除键盘/辅助功能；PASS。 |
| review | 对完整任务范围核对状态机、UI 接线、DTO/CLI 边界、文案和 Xcode target 接入；无开放发现；PASS。 |

#### 复用的精确内容测试

修复候选 suite 仍为 395 passed / 1 existing skip / 0 failed，没有重新运行。
本轮通过 `xcrun xcresulttool get test-results tests --path <result-bundle>` 独立读回以下 9 项均为 Passed，
并检查其断言对应本轮 claim：两个 SessionScopeController 的默认打开/新发布测试，
以及 origin controller 的 publication pending、no_change/permission warning、clear/not_queried、
unmatched clear、旧回执、Codex 五态和 claude-mem 安全原因测试。
完整方法与结果事实保存在这里，不把临时 xcresult 包当作持久引用。

Round 2 已通过的 confirmed-default 与 validation 证据继续复用，
ASC-SC-R1-F1 的三个回归不因本轮补齐门禁而重跑。实际 helper 测试、bridge 错误分类和既有 renderer
均仍绑定相同 11 路径候选。

本轮未扩展 Go JSON 字段或来源管理入口。已批准接口没有项目标签字段，Settings 保持精确 client/ID，
不推断项目、不以来源绝对路径填充；未把缺失字段写成已实现。
常规原生宽度沿用既有 460pt，窄宽度保护由真实 native group 的 320pt 渲染断言验证。

#### 完成边界

本轮追加 8 条 evidence，逐项读回一致；35 条关系预检全部通过并写入，包括
observed_at/satisfies、对原有效测试的 depends_on、整任务 review 的 rolls_up 及两条明确 supersedes。
最终 Task gate 返回 VERIFIED：10/10 条 required criterion 有有效通过证据，missing_criteria=[]，
unresolved_candidate_impacts=[]。invalidated_evidence 中的两项为已被本轮替代的 Round 2
review/native-ux 的 not_verified 观察，不是未解决的失效证据。
Round 1 失败仍绑定其原候选，Round 2 限定 PASS 和当时未完成的事实保留。
新增证据 ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r3-20261107-gatefill:`，
后缀为上表 8 个 criterion 名；WorkUnit 的交付生命周期留待授权提交后完成。

Task checkpoint：ad-auxiliary-session-classification-settings-control；candidate=21bdec01283376983147c1dc81532434641aaac8a5569ac5b7de6fad9354d4d4；acceptance=20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1；gate=VERIFIED
提交建议：可按任务 6 的完整边界提交：11 个产品/测试路径，加 tasks.md 与本评审记录，共 13 路径；提交仍需用户单独授权。
推送建议：拟推送至 origin 的 feature/auxiliary-session-classification；先完成授权提交及实际提交对象、署名、SSH 签名和交付检查，再单独授权推送。

任务 7 的真实依赖仍包括本任务，须待本任务授权提交并关闭交付后解除；未领取任务 7。
Topic 尚有任务 7–9，未跨 Topic 完成边界。下一步交付建议为提交本 Task，不自动执行。

L0 收口：`python3 scripts/check-review-records.py --base HEAD docs/topics/auxiliary-session-classification/reviews/settings-control.md`、
`python3 scripts/check-docs.py`（含 `git diff --check`）、`make check-whitespace` 均通过。

收口时发现并行原型变更：`prototype/src/App.jsx`、`AuxiliarySettings.jsx`、`Settings.jsx`、
`auxiliarySettings.css`、`auxiliarySettingsModel.js` 与 `prototype/tools/check-auxiliary-settings.js`。
这些变更引入选中即保存、手动分类 sheet 等新交互，未包含在本轮已固定的 11 路径产品候选、
既有批准的需求/架构/Settings UX 输入或 13 路径提交建议中；未修改、回退或审查该原型候选。
本轮 PASS/VERIFIED 认证已列明的原生候选及既有批准合同，不认证与此新原型的一致性。
后续若采用该新交互，须在相应授权下对账合同、实现与证据，不能沿用本轮结果替代新设计验收。

## Round 4 — 2026-11-08

## 📋 settings-control 重设计实现独立评审

📊 综合评分：7/10

✅ 结论：FAIL

- Reviewer：Codex，独立于本轮实现者 Claude Code；未委派子代理。
- Method：验证专题绑定与实现交接，使用本工作区 CodeGraph 定位调用路径，核对当前 Settings 契约、
  控制器、视图、bridge 与相关测试；直接编译实际控制器进行隔离复现，独立读回已有 XCTest 结果。
- Scope：任务 6 的重设计实现候选。取得决定性阻断复现后停止广泛验证；本轮不声称其余验收全部通过。
- Reviewed state：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6`；
  11 路径 SHA-256 `28b23079ad43efd3058dcd1fac148545fb88d6dc0e34a4f918e31bf3dbaec3ac`。
  路径集合和摘要算法沿用 Round 1；与实现交接逐项核对一致。
- Contract：`ux/settings.md` blob `2b873cd65cb8e6c68cdd900f5ca41533de04e011`，设计文档 Round 8 PASS，已在当前 HEAD 交付。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`；
  沿用 Round 2 的真实用户决定，排除原生键盘导航、焦点及辅助功能（含 VoiceOver/AX、Dynamic Type），不记作测试通过。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:28b23079ad43efd3:acceptance:20f6c1bae556bddc`。
- Completion gate：FAILED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。
- Implementation handoff：Beads comment `610c2da3-e821-5f3f-bd6d-fc7e456b5b17`，实现者明确释放待独立评审。

### 🔴 严重问题 — 必须修复

#### ASC-SC-R4-F1 — P2：后返回的 core 不可读状态未使本组旧确认值失效

- 位置：[SessionOriginSettingsController.swift:385](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L385)，
  尤其第 391 行只更新 `sources`；[同文件:482](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L482)
  的 `writesEnabled` 只检查先前的默认值、覆盖列表和写入状态。
- 触发：一次打开或重新读取中，`readDefaultScope` 与 `listOverrides` 先成功；后发起的
  `readOriginSourceStatus` 返回 `core_unavailable`。这些是独立 helper 请求，core 可在它们之间变得不可读，
  不能假定三个结果永远同时成功或失败。
- 行为风险：来源行和说明已告知“本机用量库暂不可读，暂时不能修改”，
  默认控件却仍选中旧值，手动分类摘要仍显示旧数量，sheet 没有不可读提示，查找和写入继续启用。
  点击另一个默认范围实际会提交新的写请求。这违反 core 不可读时整组 `loadFailed` 的契约，
  把已无法确认的旧快照继续呈现为当前可操作状态。本复现没有访问真实数据库，不能据此声称发生了持久数据损坏。
- 契约：[自动识别（只读）](../ux/settings.md#自动识别只读)要求 core 不可读时不把旧覆盖当作仍然有效，
  默认范围与列表均不可读，sheet 禁用查找与修改；成功重新读取后才恢复。
  [任务 6](../tasks.md#6-settings-control)同样要求 core 不可读时本组进入 `loadFailed`。
- 证据：直接编译当前控制器，transport 先返回 Work/revision 0 和一条有效覆盖，
  再返回两行 `reason_code=core_unavailable` 的合法来源状态。设置非空查询 ID 后观测：

  ```text
  core_note=coreUnavailable group_status=ready writes_enabled=true lookup_enabled=true
  default=work manual_summary=originOverridesSummaryCount sheet_status=nil
  submitted_writes_after_core_unavailable=1
  ```

  其中摘要显示的是文案键：隔离探针仅对文案格式化使用 shim；状态机、Observation/MainActor、
  控制器与共享 DTO 均为真实实现。最后一行由 transport 的写入调用记录计数得出，
  对应 `selectDefault(.auxiliary)`；不是只根据按钮样式推断。
- 可重复步骤：复用 `SessionOriginSettingsControllerTests.swift` 的 `StubOriginTransport`，
  给 defaults 队列 `.confirmedWork(revision: 0)`、pages 队列一条 matched 覆盖、
  statuses 队列两行 core_unavailable（partial=true）；创建实际控制器并依次执行
  `await open()`、`setLookupID("existing-session")`、`await selectDefault(.auxiliary)`。
  断言应进入 loadFailed、默认无选中、列表不可读、查找与写入禁用且写调用数为 0；当前结果如上。
- 测试保护：现有 core 不可读 fixture 同时把默认值和列表设为 unavailable；来源说明测试只检查文案分支，
  没有覆盖“值读取成功，但后续来源状态明确报告 core 失败”的序列。
- Disposition：OPEN；修复载体为同一任务 `ad-auxiliary-session-classification-settings-control`。

💡 修复建议：把明确的 core_unavailable 作为本组数据不可用的输入，统一处理旧默认值、覆盖和查询结果的有效性，
禁止新查找与写入，成功重新读取后再恢复；同时保留已有 pending 操作及其原请求身份，
避免把单纯外部 adapter 不可用扩展成整组禁用。增加上述分步读取的确定性回归，并覆盖恢复与较早读取晚返回的情形。

### 🟡 改进建议 — 推荐

无独立建议项。

### 🟢 优点

- 历史发现 ASC-SC-R1-F1 保持 CLOSED：成功确认写入仍推进对应读取代次，重设计未移除旧响应保护。
- 选中即保存保留了确认值与请求身份的边界；撤销后的可见查询独立于 clear 回执，未把 not_queried 当成分类结果。
- 状态归属、来源失效位置与恢复入口已有明确模型及回归，当前缺陷集中于来源状态与整组可用性的衔接。

### 📝 总结

Round 4 FAIL，新增一项 P2 发现；旧 Round 3 PASS/VERIFIED 仍仅适用于旧候选。
任务 Dev 保持完成、Review 未勾选，产品代码、测试和配置保持只读；没有提交或推送。

#### Evidence 与适用范围

- 当前 11 路径指纹与实现交接一致。控制器 blob `2e6bc6024370a68bb1f87219d3d924f1283d8f68`，
  视图 blob `797b4fb668b8f573536abcffbe8f017aa24070a0`，测试 blob `0d2c60745016abab35b62f89573ae8738e066def`。
  任务状态与本评审记录不属于该产品指纹。
- 复现通过 `xcrun swiftc -parse-as-library -swift-version 5 -F <Debug-products> -framework AgentDeckShared`
  编译真实控制器及独立 transport，正常启动 Observation 宏。没有启动真实 App、helper、扫描器或真实状态写入。
  该证据验证控制器状态转换，不宣称完成原生窗口或真实存储故障验收。
- 使用 `xcrun xcresulttool get test-results summary --path <result-bundle>` 读回开发交接的正常结果：
  2026-11-08 01:36:51 -0500 开始的 suite，total 403、passed 402、skipped 1、failed 0，Passed；
  主机 macOS 26.7.3 / 25G416、x86_64。与 App 219（含既有跳过）、Shared 133、Widget 51 的交接一致。
- 后续 01:40:37 -0500 开始的 bundle 为开发交接已说明的五处变异检查：394 passed、8 failed、1 skipped。
  失败分别涉及自动重查、已选值不发请求、状态归属、core/adapter 说明与 Grid 高度，
  不把这份注入缺陷的结果当作当前候选失败。当前源码指纹与开发者恢复后的交接一致。
- 未重跑未变的完整 XCTest suite；其通过没有覆盖本轮复现。阻断后不追加原生重启、并发 CLI 或宽泛界面验收。
  这些旧候选结果未经本轮完整适用性对账，不自动重标为新候选 PASS。
- 临时探针、构建产物和 XCTest bundle 均不作为持久引用；必要输入、命令形态、结果与限制已记录在这里。

#### 门禁与交接

入口已确认现有 WorkUnit 及 10 条必需 criterion，新增本候选 ContentState 后逐项读回一致；
精确目标入口 gate 为 NOT_VERIFIED，旧状态的 PASS/FAIL 均未错误适用于本候选。
本轮追加 3 条 evidence，节点逐项读回一致；6 条关系预检全部通过并确认写入。
最终精确目标 gate 为 FAILED：source-status 与 review 各有有效失败证据，validation 复用的现有 suite 通过；
其余 7 项尚无本候选的完整适用性证据。没有未解决的 candidate impact；invalidated_evidence 中的两项
仍为已被历史 Round 3 替代的 Round 2 review/native-ux 观察，不是本轮新制造的缺口。
本轮证据 ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r4-20261108-Sci0l:`，
后缀为 `source-status`、`review`、`validation`。任务返回修复并释放评审负责人；不跨 Task 或 Topic 完成边界。

L0 收口：`python3 scripts/check-docs.py`（包含变更评审记录结构、链接及 `git diff --check`）与
`make check-whitespace` 均通过；这些检查不代表语义 PASS 或 CE VERIFIED。
收口复核 HEAD 与全部 11 个产品/测试 blob，指纹仍与本轮候选相同。

下一步指令：`修复：auxiliary-session-classification / reviews/settings-control.md / ASC-SC-R4-F1`

## Repair 2 — 2026-11-08

- Repairer：claude-code（本任务实现者，非独立评审）。范围：仅 ASC-SC-R4-F1，以及评审建议所指的恢复路径与较早读取晚返回；未处理未登记问题。
- 根因确认：默认值、覆盖列表与单会话查询都存在 core 中，但来源状态读取只更新 `sources`。
  `writesEnabled`、`status` 和视图只看先前取得的默认值与列表。
  因此，读取成功之后、来源状态明确报告 `core_unavailable` 时，旧快照仍被当作当前且可写的状态。
  原有的 core 不可读回归把默认值和列表都设成 unavailable，没有覆盖这种分步序列。

**ASC-SC-R4-F1 → 修复，待复评。**

- 变更（[SessionOriginSettingsController.swift](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift)）：
  - 来源状态读取开始时记下默认值、覆盖列表和单会话查询的代次。若答复中任一行为 `core_unavailable`，代次未变的值一律作废：
    默认值和列表改为 unavailable，查询结果清空。同时推进各自代次，此前发出、仍在途的读取返回时被丢弃。
    在来源读取开始之后才读取或确认的值比该报告新，予以保留。
    整组随之进入 `loadFailed`：默认无选中，摘要为覆盖列表不可读，sheet 显示 core 不可读状态条，查找与写入禁用。
  - pending 操作不受影响：saving、unknown、conflict 保留原请求 ID、负载、expected revision 和对应操作。
    只有外部 adapter 不可用（无 `core_unavailable`）时不作废任何值。
  - 写入或回执读回得到确定结果时，若来源仍报告 core 不可读，就重新读取默认值、列表和来源状态，使数值、说明与可用性一致。
  - 新增 `selectedDefault`：整组读取中或不可读时默认控件不选中任何项，视图改用它，避免局部作废后仍显示旧选择。
  - 抽出 `OriginSettingsCopy.reportsCoreUnavailable`，来源说明和作废判断共用同一判定。
- 回归（[SessionOriginSettingsControllerTests.swift](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift)）：
  测试 transport 的一次性挂起读取增加来源状态读取。新增 5 项：
  - `testACoreFailureReportedAfterSuccessfulReadsMakesTheGroupUnreadable`：评审的复现步骤。
    默认 Work/0 与一条覆盖读取成功，随后来源状态两行均为 `core_unavailable`。
    断言 loadFailed、默认无选中、列表不可读、摘要与 sheet 状态条、查找与写入禁用，且写入、移除、打开行、查找均为 0 次调用。
    重新读取成功后整组恢复。
  - `testAnAdapterOutageAloneLeavesTheGroupWritable`：只有 claude-mem 元数据不可读时仍为 ready 且可写。
  - `testReadsThatPredateACoreFailureCannotBringOldValuesBack`：conflict 期间读取最新值，默认读取被挂起；
    此时重开窗口只读来源状态，报告 core 不可读。放行后旧默认读取不恢复 Work，conflict 保留原请求与「读取最新值」；
    之后开始的读取恢复整组。另一段中，挂起的单会话查询在 core 报告后返回，不再显示会话。
  - `testALateCoreFailureKeepsOnlyValuesConfirmedAfterItBegan`：挂起的来源读取在一次新确认的默认写入之后返回 core 不可读。
    新确认值保留，早于报告的列表作废，整组不可读且默认无选中，直到重新读取。
  - `testAResultWhileTheCoreIsReportedUnreadableReadsEverythingAgain`：结果未知期间 core 被报告不可读，
    请求 ID 与「查询结果」保留；读回回执后重新读取全部，来源恢复后整组可写。
- 修复前复现：临时去掉作废调用和结果后的重读，在隔离套件中运行。
  `testAnAdapterOutageAloneLeavesTheGroupWritable` 以外的 4 项新回归失败，且只有这 4 项失败。
  首项断言与评审观测一致：status 为 ready、默认仍选中 Work、摘要显示「已手动分类 1 个会话」、sheet 状态条为 nil、
  写入与查找启用，并实际提交了 1 次 `saveDefault(.auxiliary)` 写入。
  之后恢复源码，控制器、视图与测试三个文件的 SHA-256 与下述候选逐字节一致，并重新编译以清除临时构建产物。
- 验证：`scripts/test-macos-app.sh`（隔离 HOME）在最终内容上 TEST SUCCEEDED：App 224 项（含 5 项新回归，1 项为既有 opt-in 跳过）、
  Shared 133 项、Widget 51 项，0 失败。`make check-whitespace`、`git diff --check` 通过。
  本次只改控制器、视图的一处选中绑定和测试；Go、CLI bridge、文案未改。
- 候选：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6` + 同一 11 个路径，摘要
  `ea76f255edf353328946b5be2e5756537baebc5d49e373cfe81527540e573139`（算法同 Round 1）。相对 Round 4 变化的 blob：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | `28f0743987fa651104ef9e35e4dbebcdd4c076f3` |
  | apps/macos/AgentDeckApp/SettingsWindowView.swift | `870b7a5da29837a7d781428a7a26fc62f908a3d7` |
  | apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | `04cf5526ab52e0de5977730c46b34307cc22e303` |

  其余 8 个路径 blob 与 Round 4 相同。
- CEv1：本会话 Neo4j MCP 连接失败（ENETUNREACH），未查询、未写入；Repair 不宣称门禁完成。
  Round 4 的 FAILED 证据绑定旧候选 `28b23079…`，新候选证据由复评按上述摘要绑定。
  原生键盘、焦点与辅助功能仍按 Round 2 的用户决定排除；应用重启读回与并发 CLI 原生交互仍未验收。

修复后状态：ASC-SC-R4-F1 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。


## Round 5 — 2026-11-08

## 📋 settings-control 修复范围独立复评

📊 综合评分：7/10

✅ 结论：FAIL

- Reviewer：Codex，独立于 Repair 2 实现者 Claude Code；未委派子代理。
- Method：复用已验证工作区与项目规则；核对修复交接、3 个变化的产品/测试 blob、调用路径和 5 项新回归，
  读回现有 XCTest 结果；直接编译实际控制器，用 checked continuation 确定性控制读取交错。
- Scope：ASC-SC-R4-F1 及受影响的失效、读回恢复、旧读取和 pending 身份路径；复核历史 ASC-SC-R1-F1 未被移除。
  取得阻断复现后停止广泛验证，未执行其余整任务原生验收。
- Reviewed state：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6`；
  11 路径 SHA-256 `ea76f255edf353328946b5be2e5756537baebc5d49e373cfe81527540e573139`，算法和路径集合沿用 Round 1。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`；
  原生键盘、焦点及辅助功能继续按 Round 2 的真实用户决定排除，不记作通过。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:ea76f255edf35332:acceptance:20f6c1bae556bddc`。
- Completion gate：FAILED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。
- Repair handoff：Beads comment `e678d847-179d-5793-8b5c-7f0d9fc1e718`。

### 🔴 严重问题 — 必须修复

#### ASC-SC-R4-F1 — P2，STILL OPEN：已发起但未返回的读取仍让旧确认值逃过失效

- 原复现处置：Round 4 的直线序列已修好，直接运行得到
  `status=loadFailed writes_enabled=false selected=nil`。但相同“core 已明确不可读，旧值仍可操作”行为
  在修复所覆盖的并发读取路径中仍可复现，因此此发现尚不能关闭；没有另造一个重复发现。
- 位置：[SessionOriginSettingsController.swift:388](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L388)
  捕获读取代次，[同文件:414](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L414)
  的 `invalidateValues` 仅在代次未变时使值失效。
  然而 `readDefault` 第 350 行与 `readOverrides` 第 367 行是在请求**开始**时推进代次，
  并非成功确认新值时；其间旧 `defaultState` 与 `overrides` 仍然保留。
- 触发：一次 Settings 打开已进入来源状态请求且未返回；再次关闭/重开会启动新的默认读取，
  当前页仍允许分页，另一个列表读取也可启动。两项新读取都尚未完成时，先前来源请求返回 core_unavailable。
  两个代次虽然已经变化，界面却还只有更早的旧确认值，`invalidateValues` 仍把它们当作“较新”的值而跳过。
- 行为风险：本组回到或保持 ready，旧 Work 仍选中、旧列表仍显示、sheet 不显示不可读条，查找与写入均可用。
  `selectDefault(.auxiliary)` 实际发出一个写请求。本次新读取最终都返回 unavailable，
  不存在可作为保留依据的新成功读回。该复现仅使用合成 transport，不声称发生真实持久数据损坏。
- 独立复现：真实控制器和 Observation/MainActor、当前共享 DTO，复用测试的 `StubOriginTransport`；
  所有挂起点均用 checked continuation，等待到达后才释放，不依赖 sleep 或随机调度。

  ```text
  original_sequence status=loadFailed writes_enabled=false selected=nil
  overlap core_note=coreUnavailable status=ready writes_enabled=true lookup_enabled=true selected=work sheet_status=nil
  pending_default=true pending_list=true
  submitted_writes_after_core_unavailable=1
  ```

- 可重复调度：
  1. 初次 `open()` 返回 Work/0、第一页 20 条覆盖（total=21）和健康来源状态。
  2. 再次 `open()` 的默认值、列表先成功；来源返回 core_unavailable，但在 transport 内挂起答复。
  3. 新一次 `open()` 启动默认读取并挂起；`showOverridePage(2)` 启动列表读取并挂起。
     两项答复均预设为 unavailable，尚未向控制器返回。
  4. 只放行第 2 步的来源答复并等待该 `open()` 结束，观察上述 overlap 行。
     输入非空 ID 后调用 `selectDefault(.auxiliary)`，transport 记录 1 次写调用；正确结果应为 0。
  5. 放行剩余两项读取并等待其结束，避免留下挂起任务。
- 测试保护：新增用例覆盖了“值读取先开始、来源读取后开始”以及“来源读取期间写入已成功确认”，
  未覆盖“来源读取先开始、新值读取随后启动但仍未返回”的差别。
- 契约仍为 [自动识别（只读）](../ux/settings.md#自动识别只读)与[任务 6](../tasks.md#6-settings-control)：
  core 不可读后不能继续把旧值作为当前可写状态，恢复需要成功读回。一个请求刚开始并不构成确认。
- Disposition：STILL OPEN；修复载体仍为 `ad-auxiliary-session-classification-settings-control`。

💡 修复建议：区分请求代次与成功确认值的时序，不能用“有更新的读取启动了”证明旧值仍有效。
使尚无新成功确认的旧默认值、列表与查询正确失效，并处理在途答复；保留真正较新的成功确认及 pending 请求身份。
新增上述交错的确定性回归，断言报告失败后、其他读取仍挂起期间就已禁止写入，而不只断言所有请求结束后的结果。

### 🟡 改进建议 — 推荐

无独立建议项。

### 🟢 优点

- ASC-SC-R1-F1 保持 CLOSED：写入确认推进代次和丢弃更早读取的保护仍在，未被本次修复删除。
- ASC-SC-R4-F1 的原始直线复现已通过；core 失败与纯 adapter 失败分开处理，pending 请求身份保留。
- 新增 5 项回归覆盖原复现、恢复、旧读取晚返回、较新的已确认写入及未知结果读回，能检出移除修复后的相关缺陷。

### 📝 总结

Round 5 FAIL：历史 ASC-SC-R1-F1 保持关闭，ASC-SC-R4-F1 部分修复但仍开放；本轮没有新增独立发现。
任务 Review 保持未勾选；本轮产品、测试和配置均只读，没有提交或推送。

#### Evidence 与内容身份

- 同一 11 路径候选与 Repair 2 交接一致，只有以下 3 个 blob 相对 Round 4 变化：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | `28f0743987fa651104ef9e35e4dbebcdd4c076f3` |
  | apps/macos/AgentDeckApp/SettingsWindowView.swift | `870b7a5da29837a7d781428a7a26fc62f908a3d7` |
  | apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | `04cf5526ab52e0de5977730c46b34307cc22e303` |

  视图的改动为默认控件改读 `selectedDefault`；其余 8 个产品/测试 blob 与 Round 4 一致。
- 用 `xcrun xcresulttool get test-results summary --path <result-bundle>` 读回 2026-11-08
  02:38:52 -0500 开始的正常 suite：total 408、passed 407、failed 0、skipped 1，Passed。
  与 App 224（含既有跳过）、Shared 133、Widget 51 一致；macOS 26.7.3 / 25G416、x86_64。
- 同日 02:42:52 -0500 开始的变异 suite 实际为 402 passed、5 failed、1 skipped。
  其中 4 项是 Repair 2 所列的新回归；另有未修改的
  `MenuBarViewModelTests.testDateEventsInvalidateRenderedSurfacesWithoutRefreshOrFilterChanges` 失败。
  此读回纠正 Repair 2“且只有这 4 项失败”的计数，不改写历史交接。额外失败不出现在正常候选 suite，
  本轮未诊断其根因，也不将变异 suite 计作当前源码候选的回归失败。
- 独立探针以 `xcrun swiftc -parse-as-library -swift-version 5 -F <Debug-products> -framework AgentDeckShared`
  编译当前控制器，保留实际 Observation 宏；只有文案格式化使用 shim，读取调度使用测试 transport。
  未读取或写入真实账户、数据库、配置；这属于控制器边界验证，不是原生窗口或实际数据库故障验收。
- 复用已有正常 suite，不因复评重跑。临时探针和 XCTest bundle 不作为持久引用；输入、调度、计数和限制已记录在此。

#### 门禁与交接

本候选 ContentState 已写入并逐项读回一致，入口 gate 为 NOT_VERIFIED。
新增 3 条 evidence 均逐项读回一致，6 条关系预检全部通过并确认写入。
最终精确目标 gate 为 FAILED：source-status、review 各有当前有效失败证据，validation 有现有正常 suite 的通过证据；
其余 7 项仍缺本候选的完整适用性证据。unresolved_candidate_impacts 为空；invalidated_evidence 中两项仍是
已被历史 Round 3 替代的 Round 2 review/native-ux 观察。旧候选观察未直接重标为新候选结果。
新增 evidence ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r5-20261108-Xin7pq:`，
后缀为 `source-status`、`review`、`validation`。任务返回修复并释放评审负责人。
任务 7–9 仍未完成，本轮不跨 Task 或 Topic 完成边界。

L0 收口：`python3 scripts/check-docs.py`（含评审记录结构、链接与 `git diff --check`）和
`make check-whitespace` 均通过；收口核验全部 11 个产品/测试 blob 与 HEAD，指纹仍与本轮候选一致。
这些检查不颁发语义 PASS 或 CE VERIFIED。

下一步指令：`修复：auxiliary-session-classification / reviews/settings-control.md / ASC-SC-R4-F1`

## Repair 3 — 2026-11-08

- Repairer：claude-code（本任务实现者，非独立评审）。范围：仅 Round 5 仍开放的 ASC-SC-R4-F1；未处理未登记问题。
- 根因确认：Repair 2 用「读取代次在来源读取开始后是否变化」判断旧值是否已被更新的结果取代。
  但默认值、列表和查询的代次都在请求**开始**时推进。新读取一旦发起，哪怕尚未返回，或最终返回失败，旧值都被当作「较新」而跳过作废。
  Round 5 的交错正是这样：来源读取先开始，随后发起的默认读取和分页读取都还挂起时，core 不可读的报告到达。

**ASC-SC-R4-F1 → 修复，待复评。**

- 变更（[SessionOriginSettingsController.swift](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift)）：
  - 每个值另设「已应用次数」：读取答复真正写入状态时才递增，写入回执确认新值时也递增；请求开始时不递增。
    读取代次仍只负责丢弃过期答复，两者职责分开。
  - 来源读取开始时记下已应用次数。答复为 `core_unavailable` 时，自那以后没有新答复或确认写入落地的值一律立即作废：
    默认值和列表改为 unavailable，查询清空。同时推进各自读取代次，报告到达时仍在途的读取，即使之后返回成功页，也被丢弃。
    只有报告之后才发起的读取能恢复。
  - 来源读取开始后真正应用过的新值，例如确认的写入，仍予保留。pending 操作的请求身份与可用操作不变；单纯 adapter 不可用不受影响。
  - 视图与文案未改。
- 回归（[SessionOriginSettingsControllerTests.swift](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift)）：
  新增 `testReadsStillInFlightWhenTheCoreIsReportedUnreadableDoNotKeepTheGroupWritable`，按 Round 5 的调度执行：
  1. 初次打开得到 Work/0、第 1 页 20 条（total 21）和健康来源。
  2. 再次打开的来源答复 `core_unavailable`，在 transport 内挂起。
  3. 第三次打开的默认读取和 `showOverridePage(2)` 的列表读取先后挂起。
  4. 只放行来源答复后，在两项读取仍挂起时断言：loadFailed、写入禁用、默认无选中、列表不可读、sheet 显示 core 不可读状态条；
     输入 ID 后查找禁用，`selectDefault(.auxiliary)` 的写调用为 0。
  5. 放行仍挂起的分页读取：它返回的有效第 2 页不会恢复列表。再放行默认读取，整组仍为 loadFailed。
- 修复前复现：临时把作废判据换回 Repair 2 的代次比较，在隔离套件中运行。整个套件只有这一项新回归失败。
  失败断言与 Round 5 的 overlap 行一致：status 为 ready、写入可用、默认选中 Work、列表仍显示、sheet 状态条为 nil、查找可用，
  且实际记录到 1 次 `saveDefault(.auxiliary)` 写调用。挂起的分页读取放行后还会把列表恢复。
  之后恢复源码，三个文件的 SHA-256 与下述候选逐字节一致，并重新编译以清除临时构建产物。
- 更正：Repair 2 写的「且只有这 4 项失败」只统计了本测试类。Round 5 读回的那次变异 suite 另有
  `MenuBarViewModelTests.testDateEventsInvalidateRenderedSurfacesWithoutRefreshOrFilterChanges` 失败，以 Round 5 的读回为准。
  本次的修复前复现运行中，该测试未失败；它与本发现无关，本次未诊断。
- 验证：`scripts/test-macos-app.sh`（隔离 HOME）在最终内容上 TEST SUCCEEDED：App 225 项（含新回归，1 项为既有 opt-in 跳过）、
  Shared 133 项、Widget 51 项，0 失败。Repair 2 的 5 项回归继续通过。`make check-whitespace`、`git diff --check` 通过。
- 候选：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6` + 同一 11 个路径，摘要
  `a9a263da0807ba96e920023422aa2600ba5aabe059a8fab108955eb2a8c7b9d2`（算法同 Round 1）。相对 Round 5 变化的 blob：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | `017eb2b8c2f50f6b2dd186065816ec0feaac21b5` |
  | apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | `8a088ab83f0dea0e51ae1ce89a9a021b5467e026` |

  其余 9 个路径 blob 与 Round 5 相同（视图仍为 `870b7a5da29837a7d781428a7a26fc62f908a3d7`）。
- CEv1：本会话 Neo4j MCP 连接失败（ENETUNREACH），未查询、未写入；Repair 不宣称门禁完成。
  Round 5 的 FAILED 证据绑定旧候选 `ea76f255…`，新候选证据由复评按上述摘要绑定。
  原生键盘、焦点与辅助功能仍按 Round 2 的用户决定排除；应用重启读回与并发 CLI 原生交互仍未验收。

修复后状态：ASC-SC-R4-F1 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 6 — 2026-11-08

## 📋 settings-control 修复范围独立复评

📊 综合评分：9/10（本轮修复范围）

✅ 结论：PASS

- Reviewer：Codex，独立于 Repair 3 实现者 Claude Code；未委派子代理。
- Method：核对当前候选与 Repair 3 交接，通过本工作区 CodeGraph 定位并检查读取代次、结果应用次数和写入确认路径，
  审查确定性回归的调度与断言，直接读回现有正常、变异 XCTest 结果及 9 项相关测试节点。
- Scope：ASC-SC-R4-F1、其直接影响的读取失效/恢复/pending 操作，以及 ASC-SC-R1-F1 的回归保护。
  本轮关闭记录中的全部历史发现；不追认 Round 4 因阻断而停止的整任务剩余审查和验收。
- Reviewed state：HEAD `5d47df828df4c76fd2d909e2981a4606ac9d87c6`；
  同一 11 路径 SHA-256 `a9a263da0807ba96e920023422aa2600ba5aabe059a8fab108955eb2a8c7b9d2`，算法沿用 Round 1。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`；
  原生键盘、焦点与辅助功能继续按 Round 2 的真实用户决定排除，不记作通过。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:a9a263da0807ba96:acceptance:20f6c1bae556bddc`。
- Completion gate：NOT_VERIFIED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。
- Repair handoff：Beads comment `b8830b79-d433-5a95-a595-f71937353219`。

### 🔴 严重问题 — 必须修复

无。本轮范围内无仍开放、回归或新增的阻断发现。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

**ASC-SC-R4-F1 → CLOSED。**

- [控制器:395](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L395)
  在来源读取开始时保存 `defaultApplied`、`overridesApplied`、`lookupApplied`。
  请求开始只推进读取代次；答复通过代次检查、真正应用到状态后才推进应用次数。
  [失效处理:426](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L426)
  因此不会再把“新读取仍挂起”误判成已有新确认值。
- core_unavailable 返回时，没有更新结果落地的旧值立即不可用，并推进读取代次，使仍在途的答复不能恢复旧状态。
  默认控件无选中、列表和 sheet 不可读、查找与写入禁用；pending 原请求身份不变。
- [新增回归:933](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift#L933)
  原样覆盖 Round 5 的交错：先挂起来源答复，再挂起新的默认读取和分页读取，只放行 core 失败答复。
  在另两项仍挂起时断言 loadFailed、无选中、查找/写入禁用，并检查写调用为 0。
  随后放行一份有效的第 2 页仍不能恢复列表，最后放行默认读取后也维持不可用。
- 应用次数也在已确认的默认写入与覆盖回执应用时推进，保留真正较新的成功结果；
  既有回归确认成功重新读取能够恢复，纯 adapter 不可用不锁组，未知结果仍可用原请求读回回执。

**ASC-SC-R1-F1 → 保持 CLOSED。**

- [确认路径:655](../../../../apps/macos/AgentDeckApp/SessionOriginSettingsController.swift#L655)
  仍推进默认/查询读取代次；新应用次数未替代原有防旧响应逻辑。
  默认、列表和单会话查询三个原竞态回归在本候选上均为 Passed。

### 📝 总结

Round 6 修复范围复评 PASS，记录中的两个历史发现均已关闭。本轮未发现新的范围内缺陷。
整任务完成门禁仍为 NOT_VERIFIED；Tasks 矩阵的整任务 Review 保持未勾选，Beads 保持 in_review。
本轮只更新评审、状态、证据与协调记录，没有修改产品、测试或配置，没有提交或推送。

#### 内容身份与现有验证

- 11 路径指纹与 Repair 3 交接一致；相对 Round 5 仅以下两个 blob 改变：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/SessionOriginSettingsController.swift | `017eb2b8c2f50f6b2dd186065816ec0feaac21b5` |
  | apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift | `8a088ab83f0dea0e51ae1ce89a9a021b5467e026` |

  其余 9 路径与 Round 5 一致；视图仍为 `870b7a5da29837a7d781428a7a26fc62f908a3d7`。
- 用 `xcrun xcresulttool get test-results summary --path <result-bundle>` 独立读回：
  2026-11-08 03:43:49 -0500 开始的正常 suite 为 total 409、passed 408、failed 0、skipped 1，Passed。
  与 App 225（含既有 opt-in 跳过）、Shared 133、Widget 51 一致；macOS 26.7.3 / 25G416、x86_64。
- 同日 03:47:16 -0500 开始的变异 suite 为 407 passed、1 failed、1 skipped。
  唯一失败是新增的 `testReadsStillInFlightWhenTheCoreIsReportedUnreadableDoNotKeepTheGroupWritable`，
  直接报告 `ready` 不等于 `loadFailed`；与将判据临时换回 Repair 2 的交接一致。
  当前源码已恢复到与正常候选交接匹配的指纹；没有把变异结果记作当前候选失败。
- 用 `xcrun xcresulttool get test-results tests --path <result-bundle>` 逐项读回下列 9 项，均为 Passed，非跳过：

  | 范围 | 测试 |
  | --- | --- |
  | Round 5 交错 | `testReadsStillInFlightWhenTheCoreIsReportedUnreadableDoNotKeepTheGroupWritable` |
  | Round 4 原序列与恢复 | `testACoreFailureReportedAfterSuccessfulReadsMakesTheGroupUnreadable` |
  | adapter 独立性 | `testAnAdapterOutageAloneLeavesTheGroupWritable` |
  | 报告前读取晚返回 | `testReadsThatPredateACoreFailureCannotBringOldValuesBack` |
  | 较新确认值 | `testALateCoreFailureKeepsOnlyValuesConfirmedAfterItBegan` |
  | 原请求读回恢复 | `testAResultWhileTheCoreIsReportedUnreadableReadsEverythingAgain` |
  | 原默认读取竞态 | `testAReopenReadThatPredatesASaveCannotRollTheConfirmedDefaultBack` |
  | 原列表读取竞态 | `testAReopenListReadThatPredatesARemovalCannotRestoreTheRow` |
  | 原查询读取竞态 | `testALookupThatPredatesASaveCannotRestoreTheOlderOverride` |

- 这些是实际控制器和合成 transport 的确定性调度回归；不宣称是原生窗口或真实数据库故障验收。
  未重跑已有同候选 suite，也未重建已由新增回归覆盖的临时探针。结果的必要输入、计数、测试名和限制已收录，
  临时 XCTest bundle 不作为持久引用。

#### 精确门禁与剩余范围

当前 WorkUnit 属于 task，containing_topic 为 auxiliary-session-classification；专题 Tasks 矩阵仍有任务 7–9，
本轮不跨 Topic 完成边界。入口确认 10 条 required criterion；本候选 ContentState 新建并读回一致，入口 gate 为 NOT_VERIFIED。
本轮 source-status 的修复与回归、validation 的现有套件可绑定当前候选；整任务 review 仅记录限定复评 PASS，
不能把完整候选审查标成通过。本轮新增 3 条 evidence（2 pass、1 not_verified），节点逐项读回一致；
6 条关系预检全部通过并确认写入。最终精确目标 gate 为 NOT_VERIFIED：source-status、validation 两项满足，
下表 8 项未满足，没有当前候选的失败证据或 unresolved candidate impact。
invalidated_evidence 中两项仍为已被历史 Round 3 替代的 Round 2 review/native-ux 观察，未错误阻断本轮通过事实。
新增 evidence ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r6-20261108-3l132Z:`，
后缀为 `source-status`、`validation`、`review`。任务保留 in_review，释放复评负责人等待后续证据对账。

以下 8 项仍需对当前候选完成适用性评估、复用或补齐证据；这是证据缺口，不是新增 8 个缺陷，
也不要求机械重跑已有套件：

| Criterion | 精确后续范围 |
| --- | --- |
| confirmed-default | 重设计后选中即保存、确认值与 CAS/no_change、不可读默认的完整契约证据对账。 |
| mutation-recovery | 当前控件和 sheet 下的 saved/failed/unknown/conflict、同 ID 重试与权限警告/待发布恢复；复用仍有效部分。 |
| publication-default | 新默认/覆盖的发布请求与确认 revision、当前 Popover 临时范围不变、no_change 不触发新发布。 |
| exact-query-privacy | 当前 sheet 的精确 client/ID、编辑清目标、exclude 边界及安全字段。 |
| override-list-clear | 当前 sheet 的 20 项分页、unmatched 撤销、clear 后独立查询及旧回执保护。 |
| lifecycle-concurrency | 当前候选的应用重启读回、关闭重开、真实 CLI 并发及原生恢复证据；不得把旧设计候选直接重标。 |
| native-ux | 重设计后的中英、主题、窄布局、sheet/分页行为；继续排除用户已明确取消的键盘/辅助功能验收。 |
| review | 完成 Round 4 因阻断而停止的整任务剩余候选审查，综合上述证据及本轮发现关闭结果。 |

L0 收口：`python3 scripts/check-docs.py`（含评审记录结构、链接与 `git diff --check`）和
`make check-whitespace` 均通过；收口核验全部 11 个产品/测试 blob 与 HEAD，指纹仍与本轮候选一致。
这些检查不替代整任务审查或 CEv1 门禁。

Task checkpoint：ad-auxiliary-session-classification-settings-control；content_state=a9a263da0807ba96e920023422aa2600ba5aabe059a8fab108955eb2a8c7b9d2；gate=NOT_VERIFIED
提交建议：门禁 VERIFIED 后（补齐上表 8 项证据及剩余整任务审查），按任务 6 的 11 个产品/测试路径、tasks.md 和本评审记录形成提交边界；提交需用户单独授权。
推送建议：门禁 VERIFIED 后（同一 8 项证据及剩余整任务审查完成），完成授权提交、实际提交对象/署名/SSH 签名检查，再单独授权推送至 origin 的 feature/auxiliary-session-classification；本轮只核实远端为 https://github.com/kitdine/agent-deck.git，没有执行交付。

下一步指令：补齐 auxiliary-session-classification / settings-control 当前候选上表 8 项未验证门禁证据及剩余整任务审查；复用仍有效的检查，不重复本轮已完成的修复复评。




## Round 7 — 2026-11-09

## 📋 settings-control 重做候选整任务独立评审

📊 综合评分：9/10

✅ 结论：PASS

- Reviewer：Codex（冷上下文子线程，独立于实现者 Claude Code；未再委派）。T3 `clientRequestId`
  `auxiliary-session-classification-settings-control-review-r1`；Provider `codex`，Model `gpt-6.1-sol`，请求的 `reasoningEffort` 为 `max`，
  `delegate_task` 与 `task_status` 均回显同一组值。评审报告由主 Agent Claude Code 对照指纹与自测结果核实后写入本记录。
- Method：核对工作区与指纹；审查合同与完整候选；检查两项历史发现是否回归；用 `scripts/test-macos-app.sh` 独立运行隔离 XCTest；
  导出本轮渲染附件，与原型截图逐组目视对照。
- Scope：任务 6 的全部 13 个产品/测试路径，按修订后的 [Settings UX](../ux/settings.md) 与[任务 6](../tasks.md#6-settings-control)审查。
- Reviewed state：HEAD `8e61b614b2ce9deeceee12a1cd33a50373bce4a2`；13 路径 SHA-256 `5bcad0423fe870c595b1f6a7b6b611cb615c442f3229438d950e86e0ab749c87`，
  算法同 Round 1。比此前 11 路径多出 `apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift` 与 `apps/macos/AgentDeckAppTests/QuotaSettingsControllerTests.swift`。
  评审者实测指纹与开发交接一致。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`，与 Round 2 起相同：原生键盘、焦点与辅助功能按用户决定排除，不记作通过。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:5bcad0423fe870c5:acceptance:20f6c1bae556bddc`。
- Completion gate：NOT_VERIFIED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。

### 🔴 严重问题 — 必须修复

无。本轮没有登记 `ASC-SC-R7-F<n>`。

- ASC-SC-R1-F1 保持 CLOSED：写入确认仍推进读取代次，默认、列表、查询三项旧读取回归本轮通过。
- ASC-SC-R4-F1 保持 CLOSED：控制器 blob 仍为 Round 6 的 `017eb2b8c2f50f6b2dd186065816ec0feaac21b5`。
  请求代次与结果应用次数仍分开计；「来源报告失败时新读取尚未返回」的确定性交错回归本轮通过。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- 四页的顺序、名称、图标，无会话控制器时隐藏「会话」页，hosting controller 常驻，记住上次的页并支持按页直达，都有实现和当前测试支撑。
- 尺寸适配处理了以下情形，标题、尺寸与通知警告增高的测试均通过：
  - 非有限的测量值；
  - 切页前预先测量；
  - 延后到布局完成后再适配；
  - 动画进行中的重入。
- 写入状态与请求身份由 App 级控制器持有，切页不会重新加载实时用量，也不会清除待处理的状态。
- bridge 保留以下语义：
  - 精确的 client/ID；
  - CAS 与同请求重试；
  - 未知结果与确定失败分开；
  - clear 后另发独立的可见查询。
- 分段控件用 `.settingsSegmented()`（surface tint）：评审者判定为可接受的平台适配，不登记缺陷，也无需为此回写 UX。
  依据是现行 UX 的直接措辞禁止的是 accent tint，且本轮渲染与目视对照满足原型要求的中性选中段。
  评审者没有独立复做「去掉 tint 后 macOS 26 显示系统蓝」的实验，所以该原因只记作交接说明，不作为本轮的独立实验结论。

### 📝 总结

13 路径完整候选的代码与测试评审 PASS，无开放发现。完成门禁为 NOT_VERIFIED：mutation-recovery、lifecycle-concurrency、native-ux 三项
仍缺当前候选的验收证据（见下表）。Review 勾选；Task 边界保持开放，Beads 保持 `in_review`，提交与推送建议都要等门禁通过。

#### 证据

- 独立隔离套件：`scripts/test-macos-app.sh` 共 415 项，通过 414、跳过 1、失败 0，结果 Passed。
  分项为 App 231（跳过的是既有 opt-in 用例 `SchemaSignalAcceptanceTests.testSchemaSignalNativeMatrix`）、Shared 133、Widget 51。
  环境为 macOS 26.7.3 / 25G416、x86_64。实现者在同一指纹上的运行结果计数一致。
- 首次运行时，沙箱拒绝了脚本里的 `/bin/ps`，退出 126，测试未启动；之后在沙箱外通过同一脚本完成。全程没有直接启动 App 或 helper。
- 20 张整页渲染（中英 × 深浅 × 四页加 sheet）中，评审者抽取 6 组与原型 `tabs-*.png` 并排目视核对。
- `Localizable.xcstrings` 新增 86 个 catalog key，与 `DesktopCopy` 的 86 个 origin 文案 key 对应，中英无缺失，格式参数一致。
- `git diff --name-only HEAD -- '*.go' go.mod go.sum` 与 `git diff --name-only 27a12fc..HEAD -- '*.go' go.mod go.sum vendor` 均无输出：
  Go 与依赖未改，Round 3 中依赖后端语义的部分可以复用，但旧窗口验收不能整体迁移过来。
- `make check-whitespace` 与 `git diff --check` 通过。
- 测试保护判断：
  - 读取竞态用例用 continuation 控制答复时序，并断言确认值、版本与实际写调用次数。
  - 切页往返用例先让出执行再检查读取计数。
  - 渲染用例比较自然高度与更高画布的顶部像素。
  - 渲染中的 sheet 没有展开查找卡片，六项窗口测试也没有覆盖全部恢复冲突态，所以不能当作完整状态矩阵验收。
  - 开发交接所述的五处变异，本轮既没有重新注入，也没有独立读回其结果，因此不作为独立证据。
- 原生键盘、焦点、VoiceOver/AX、Dynamic Type 按用户决定排除，不计作测试通过。

#### 精确门禁与剩余范围

本轮新建上述 ContentState，并追加 7 条 pass evidence：review、validation、source-status、confirmed-default、publication-default、override-list-clear、exact-query-privacy。
每条有 `satisfies` 与 `observed_at` 两条关系；exact-query-privacy 与 override-list-clear 另有一条 `depends_on`，指向 Round 3 中未被替代的同名后端证据。
16 条关系全部写入。evidence ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r7-20261109-codexr1:`，后缀为 criterion 名。
精确门禁查询结果为 NOT_VERIFIED：7/10 满足；missing 为下表三项；没有被拒绝的 evidence，没有 invalidated，也没有 unresolved candidate impact。

| Criterion | 仍缺的当前候选证据 |
| --- | --- |
| mutation-recovery | 当前候选在真实 helper 上的提交前故障与提交后确认丢失恢复（同一请求 ID、负载、expected revision，回执唯一）。Round 3 的旧窗口验收不能直接迁移。 |
| lifecycle-concurrency | 当前候选的应用重启读回、真实 CLI 与窗口并发写入、原生 unknown/conflict 恢复。 |
| native-ux | 活动窗口的切页动画与选中样式、sheet 打开后工具栏的系统表现、900 pt 实体屏幕目视。键盘与辅助功能仍按用户决定排除。 |

Task checkpoint：ad-auxiliary-session-classification-settings-control；content_state=5bcad0423fe870c595b1f6a7b6b611cb615c442f3229438d950e86e0ab749c87；gate=NOT_VERIFIED
提交建议：门禁 VERIFIED 后，按任务 6 的 13 个产品/测试路径，加 tasks.md 与本评审记录形成提交边界；提交以用户本轮对「通过门禁后本地提交」的授权为准。
推送建议：本轮未授权推送；门禁 VERIFIED 且完成本地提交后，推送至 origin 的 `feature/auxiliary-session-classification` 仍需另行授权。

## Round 8 — 2026-11-09

## 📋 settings-control 门禁补证复评

📊 综合评分：8/10

✅ 结论：FAIL

- Reviewer：Codex（冷上下文子线程，独立复评角色，未再委派）。T3 `clientRequestId` `auxiliary-session-classification-settings-control-rereview-r2`；
  Provider `codex`，Model `gpt-6.1-sol`，请求的 `reasoningEffort` 为 `max`，`delegate_task` 回显同一组值。复评报告由主 Agent Claude Code 核实后写入。
- Method：按 Re-review 规则核对历史发现、源码、UX 合同和测试断言。复评者用一份隔离脚本的临时副本运行 7 项定向测试：只固定仓库路径并追加 `-only-testing`，
  临时 HOME 与进程清理逻辑保持不变；另外读回了实现者全量运行的结果。
- Scope：Round 7 之后新增的验收测试（[tasks.md](../tasks.md#6-settings-control) 中「2026-11-09 任务 6 门禁补证」条目），以及三项仍缺 criterion 的证据是否充分。
- Reviewed state：HEAD `8e61b614b2ce9deeceee12a1cd33a50373bce4a2`；13 路径 SHA-256 `8aabcfb74bbd540150b5c9c63094658056cb4bde3dbb95e5c310e0f4aa671a7a`。
  复评者核实只有 `apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift` 由 `c6493e5866fdfa36e058370aafa9accb70e89dfc` 变为 `397358cbc2a099686a4765c392ac19c7768d83f9`；
  把该 blob 换回旧值后，重算结果正好是 Round 7 的 `5bcad0423fe870c595b1f6a7b6b611cb615c442f3229438d950e86e0ab749c87`，其余 12 个路径未变。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`（不变）。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:8aabcfb74bbd5401:acceptance:20f6c1bae556bddc`。
- Completion gate：FAILED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。

既有发现：
- ASC-SC-R1-F1 保持 CLOSED：写入确认仍使旧读取失效，默认、列表、单会话查询三项竞态回归本轮通过。
- ASC-SC-R4-F1 保持 CLOSED：读取代次与实际应用次数仍分开计；「core 已报告不可读、新读取仍在途」的回归本轮通过。

### 🔴 严重问题 — 必须修复

#### ASC-SC-R8-F1 — P2：同进程内重建实例不能完成 application restart readback 验收

- 位置：[SessionOriginSettingsControllerTests.swift:1900](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift#L1900)。
- 证据：测试关闭窗口后，在同一 XCTest/App 进程里新建 controller、window 和 runner。原实例仍被局部变量和 `opened` 持有；
  进程没有退出，也没有重新执行生产 App 的启动接线。
- 行为风险：能证明新实例通过新的 helper 请求从 SQLite 读回数据；但发现不了进程启动时选错 HOME、state root 或依赖接线的回归。
  注释里的「nothing in memory survives」超出了实际覆盖范围。
- 修复建议：保留现有的实例重建测试；另补两阶段隔离 hosted XCTest：第一阶段保存后结束宿主，第二阶段用同一临时 HOME 启动新的宿主 PID，
  经实际启动接线打开 Settings，断言持久值与 revision。可以通过隔离脚本完成，不需要启动已安装的 App，也不触碰真实 HOME。只重启 helper 不够。
- Disposition：OPEN；修复载体为同一任务 `ad-auxiliary-session-classification-settings-control`。

#### ASC-SC-R8-F2 — P2：native-ux 仍缺切页动画的直接行为证据

- 位置：[SessionOriginSettingsControllerTests.swift:1435](../../../../apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift#L1435)；
  依据 [窗口分页](../ux/settings.md#窗口分页)「窗口标题和高度由系统随页切换并带动画」。
- 证据：现有切页测试只检查布局稳定后的标题、尺寸和实例身份；新增 sheet 测试检查工具栏 action 被拒绝及之后的恢复。
  二者都没有观察窗口高度的动画过渡，也没有在宿主为活动应用时确认这一行为。
- 行为风险：即使去掉切页动画，这些断言仍可能全部通过。UX 允许由系统绘制的差异包括标题栏与工具栏材质、工具栏选中样式、系统控件的精确尺寸与字体渲染，
  但不包括切页动画这一要求。
- 修复建议：在隔离宿主中确认窗口可见、是 key window 且应用处于活动状态，通过工具栏 action 切页，记录高度的过渡过程以及最终标题与尺寸；
  如果无法观测，就继续保留此项为未验证。不要求系统工具栏与原型逐像素一致。
- Disposition：OPEN；修复载体为同一任务 `ad-auxiliary-session-classification-settings-control`。

### 🟡 改进建议 — 推荐

无。以上两项是验收保护与证据缺口，未复现新的产品功能缺陷。

### 🟢 优点

- 两项新增验收确实使用真实内嵌 Go helper 和真实 SQLite，另有共享同一隔离 state root 的独立 helper 请求。
- 提交前故障路径验证了原请求重试、revision 1→2 和原 ID 回执；提交后确认丢失路径验证了先保持 unknown、读回后才确认 revision 3。请求序列断言能发现重试另造 ID。
- CLI 冲突用例验证旧 revision 的保存被拒、持久值未被覆盖、读取最新值后解锁且不重放旧选择。
- sheet 用例确认真实的 `attachedSheet` 存在，并通过工具栏自身的 action 验证 sheet 打开期间禁止切页、关闭后恢复。
- 隔离链路完整：临时 HOME、`CFFIXED_USER_HOME`、测试环境传递和进程清理都保留；runner 明确绑定临时 HOME 与 `--state-dir`。

### 📝 总结

本轮 FAIL。两项发现都是验收证据缺口：mutation-recovery 判定满足；lifecycle-concurrency 仍缺 App 进程重启读回（R8-F1）；
native-ux 仍缺切页动画（R8-F2）。中英主题、460 pt、自然高度与原型对照可复用 Round 7。320 pt 已被现行固定 460 pt 的设计取代。
自然高度加标题栏与工具栏不超过 875 pt 的断言足以支撑 900 pt 屏幕的尺寸要求，不必再坚持实体屏幕目视。
键盘、焦点、VoiceOver/AX、Dynamic Type 按用户决定排除，不记作通过。

#### 证据

- 7 项定向测试 7 passed、0 skipped、0 failed，经 `xcrun xcresulttool get test-results tests` 逐项确认。7 项为：新增的两项验收、sheet 工具栏动作、
  R1 三项竞态、R4 在途读取。
- 读回实现者的全量结果：共 417 项，416 passed、1 skipped、0 failed（App 233，含 1 项既有跳过；Shared 133；Widget 51）。
- 交接所述两处控制器变异的杀伤，本轮没有独立复证；当前控制器 blob 与 Round 7 一致。

#### 精确门禁

为本候选新建上述 ContentState，并追加 2 条 evidence：`review` 记 fail（本轮 FAIL）；`mutation-recovery` 记 pass，依据是新增验收和复评者的判定。
4 条关系全部写入。evidence ID 前缀为 `auxiliary-session-classification:settings-control:evidence:r8-20261109-codexr2:`。
精确门禁查询结果为 FAILED：失败证据只有本轮的 `review`。Round 7 已满足的 7 项尚未上卷到本候选，待修复后的最终候选统一对账，不在本轮重标。

下一步指令：`修复：auxiliary-session-classification / reviews/settings-control.md / ASC-SC-R8-F1, ASC-SC-R8-F2`

## Repair 4 — 2026-11-09

- Repairer：Claude Code。范围是 Round 8 的 ASC-SC-R8-F1 与 ASC-SC-R8-F2，不扩展其他产品行为。
- ASC-SC-R8-F1 → repaired in candidate：
  - `scripts/test-macos-app.sh` 在主套件通过后，另起两次 `xcodebuild test-without-building` 宿主启动，共用测试根下新建的同一临时 HOME（`settings-restart-home`），
    隔离环境变量、helper 泄漏检查与清理均与主套件相同；任一阶段失败，脚本即失败。
  - 新增 `SettingsRestartAcceptanceTests`：没有脚本传入的阶段变量时两项都按 opt-in 跳过。
    - 第一阶段：取测试宿主里 App 自己的 `AgentDeckApplicationDelegate`，用 CLI 写入 Auxiliary/1；经 App 的设置菜单 action 打开 Settings，确认读到该值，
      再经 App 启动时构建的控制器保存 All/2，CLI 读回 All/2，并记录本进程 PID。
    - 第二阶段：断言 PID 与第一阶段不同（新进程），且读取前控制器为 `.loading`；经同一菜单 action 打开 Settings，读回 All/2，窗口为「通用」页。
  - 为让这两个阶段能观察 App 启动时构建的控制器，`AgentDeckApp.swift` 中 delegate 的 `sessionOrigin` 与 `settingsController` 由 `private let` 改为 internal `let`，
    不改变任何行为。
  - 同进程重建实例的用例保留，注释改为只声称它实际覆盖的范围。
- ASC-SC-R8-F2 → repaired in candidate：新增 `SettingsWindowPaneTests.testAToolbarSwitchAnimatesTheHeightInTheActiveWindow`。
  - 先让测试宿主成为活动应用、Settings 成为可见的 key window，前提不满足就直接失败，不跳过。
  - 通过工具栏按钮自身的 action 从「实时用量」切到「额度」，用 `NSWindow.didResizeNotification` 记录过程中的高度。
  - 断言出现严格介于起点与终点之间的中间值，最终高度等于目标页的自然高度，标题与选中页正确。
- 变异检查（均已恢复，两个产品文件 SHA-256 逐字节一致）：
  - 切页改为不带动画缩放：动画用例失败。
  - delegate 的 helper HOME 改指同一临时根下的另一目录：第一阶段读不到 CLI 写入的值而失败，脚本失败，第二阶段不再执行。
- 验证：`scripts/test-macos-app.sh` TEST SUCCEEDED。
  - App 236 项：跳过 3 项，为 1 项既有 opt-in 与主套件中按阶段门控的 2 项；Shared 133；Widget 51；0 失败。
  - 两个重启阶段各自单独执行，均通过。
  - `make check-whitespace` 与 `git diff --check` 通过。
- 候选：HEAD `8e61b614b2ce9deeceee12a1cd33a50373bce4a2` 加 14 个路径，即原 13 个加 `scripts/test-macos-app.sh`。
  SHA-256 `8e6d3ac3d00ba1bfd3f3137733f2176e676373c160857503edfdf123dc64ae81`，算法同 Round 1。相对 Round 8 的变化：
  - `apps/macos/AgentDeckApp/AgentDeckApp.swift`：`164fa9a0587eb87a796cf735d52df4e1ac9402af` → `88a0cfb8985c7cb2b387c351cb1ad7dc53851cac`。
  - `apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift`：`397358cbc2a099686a4765c392ac19c7768d83f9` → `3858bd1599b1f1a7409645a68602a8b26dd7676b`。
  - 新增 `scripts/test-macos-app.sh` = `4a980afdd9b1be2b1e01ad685bfcf7c1032eee29`。
- 待独立复评；本修复未写入 CEv1 证据。

## Round 9 — 2026-11-09

## 📋 settings-control Repair 4 复评

📊 综合评分：9/10

✅ 结论：PASS

- Reviewer：Codex，冷上下文独立复评，未再委派。T3 `clientRequestId` 为 `auxiliary-session-classification-settings-control-rereview-r3`；
  Provider `codex`，Model `gpt-6.1-sol`，请求 `reasoningEffort` 为 `max`，`delegate_task` 回显的值相同。
  复评报告由主 Agent Claude Code 核实后写入本记录。
- Method：只读核对历史发现、当前源码与候选指纹。在沙箱外运行完整隔离脚本 `scripts/test-macos-app.sh`，在沙箱内该脚本因 `/bin/ps` 被拒，测试未启动。
  另外，把 `AgentDeckApp.swift` 中新增的注释去掉、两处访问级别还原后重算 blob，与旧 blob 一致。
- Scope：Round 9 的范围限于 Repair 4、ASC-SC-R8-F1、ASC-SC-R8-F2，以及三项此前未满足的 criterion 的证据是否充分。
- Reviewed state：HEAD `8e61b614b2ce9deeceee12a1cd33a50373bce4a2`，加 14 路径，SHA-256 为 `8e6d3ac3d00ba1bfd3f3137733f2176e676373c160857503edfdf123dc64ae81`。
  该指纹在测试前、测试后均与交接一致。
- Acceptance scope SHA-256：`20f6c1bae556bddc33cff6b062221580f3da2acc4852f06ad9d76d5c3c1aefa1`，未变。
- WorkUnit：`auxiliary-session-classification:settings-control`。
- ContentState：`auxiliary-session-classification:settings-control:candidate:8e6d3ac3d00ba1bf:acceptance:20f6c1bae556bddc`。
- Completion gate：VERIFIED
- Dispatch：`ad-auxiliary-session-classification-settings-control`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。

### 🔴 严重问题 — 必须修复

无。本轮未新增 `ASC-SC-R9-F<n>`。

- ASC-SC-R1-F1 → CLOSED：控制器 blob 与 Round 8 相同，旧读取失效保护未受本次修改影响，完整套件通过。
- ASC-SC-R4-F1 → CLOSED：控制器仍为 `017eb2b8c2f50f6b2dd186065816ec0feaac21b5`；「core 不可读、新读取仍在途」的回归用例通过。
- ASC-SC-R8-F1 → CLOSED：两个重启阶段确为两个宿主进程，复评时观测到的 PID 为 75314 与 75401。
  两阶段都取 `NSApp.delegate`，经真实的 `openSettings` 菜单 action 打开窗口，使用 App 启动时构建的控制器。
  - 第一阶段确认读到 Auxiliary/1，保存后由 CLI 独立读回 All/2。
  - 第二阶段先确认 PID 已变、控制器处于 `.loading` 与 `.idle`，再读回 All/2，并停在「通用」页。
- ASC-SC-R8-F2 → CLOSED：活动窗口下的工具栏切页动画用例实际执行并通过，验证了中间高度与最终状态。

### 🟡 改进建议 — 推荐

无。

### 🟢 优点

- 隔离脚本的两个重启阶段共用测试根下新建的 HOME，并保留隔离环境变量的传递、helper 基线排除、泄漏检查与 EXIT 清理。任一阶段失败都会阻止后续阶段，并使脚本失败。
- 动画用例在触发 action 之前安装了只针对该窗口的 resize 观察器，并要求应用处于活动状态、窗口可见且为 key window。高度若直接跳到终点，过不了「严格中间值」断言。
- `private` 改为 internal 后，两项仍是 `let`，没有新增公开 setter、跨模块接口或外部写入口，没有扩大写入面。

### 📝 总结

本轮复评 PASS，四项历史发现全部关闭。三项此前未满足的 criterion 在当前候选上均有充分证据：

- mutation-recovery：满足，无回归。
- lifecycle-concurrency：满足。两次独立宿主启动经真实接线读回；关窗重开、CLI 并发冲突与 unknown 恢复用例均通过。
- native-ux：在用户排除范围之外的部分满足。活动窗口动画与 sheet 工具栏限制有新证据；中英主题、460 pt、自然高度与原型对照沿用 Round 7，相关产品 blob 未变。

键盘、焦点、VoiceOver/AX、Dynamic Type 按用户决定排除，未验证，不记作通过。复评者本轮未注入变异，交接中的变异结果不作为独立证据。

#### 证据

- 完整隔离 XCTest，运行于 macOS 26.7.3、x86_64、Xcode 26.4（17E192）：
  - 主套件 420 项：417 通过、3 跳过、0 失败（App 236、Shared 133、Widget 51）。3 项跳过为 1 项既有 opt-in，加 2 项在主套件中按阶段门控的重启用例。
  - 两个重启阶段分别执行，各 1 项通过。
  - 脚本退出码为 0。三个结果包的摘要因 TestReport 缓存写权限读取失败，以上数字依据 XCTest 日志与退出码。
- 实现者在同一指纹上的全量运行，计数与上述一致。
- 复评者核实的 blob 变化（其余 11 个 blob 未变）：
  - `apps/macos/AgentDeckApp/AgentDeckApp.swift`：`164fa9a0587eb87a796cf735d52df4e1ac9402af` → `88a0cfb8985c7cb2b387c351cb1ad7dc53851cac`。
  - `apps/macos/AgentDeckAppTests/SessionOriginSettingsControllerTests.swift`：`397358cbc2a099686a4765c392ac19c7768d83f9` → `3858bd1599b1f1a7409645a68602a8b26dd7676b`。
  - 新纳入清单的 `scripts/test-macos-app.sh` = `4a980afdd9b1be2b1e01ad685bfcf7c1032eee29`。
  - 去掉该脚本、并把上面两项换回旧 blob 后重算，正好得到 Round 8 的指纹 `8aabcfb74bbd540150b5c9c63094658056cb4bde3dbb95e5c310e0f4aa671a7a`。
- 以上检查只验证结构，不替代 CEv1 门禁。L0：`python3 scripts/check-review-records.py --base HEAD`、`python3 scripts/check-docs.py`、
  `make check-whitespace` 与 `git diff --check` 均通过。

#### 精确门禁

为本候选新建上述 ContentState，并写入 10 条 pass evidence（ID 前缀 `auxiliary-session-classification:settings-control:evidence:r9-20261109-codexr3:`）：

- lifecycle-concurrency、native-ux、validation：本候选的直接证据。
- mutation-recovery：`depends_on` Round 8 的同名证据。
- review：`depends_on` Round 7 的整任务评审证据。
- confirmed-default、publication-default、override-list-clear、exact-query-privacy、source-status：
  这些行为相关的产品 blob 未变，且在本候选的全量套件中再次通过，因此 `rolls_up` 到 Round 7 的同名证据。

27 条关系全部写入。精确门禁查询结果为 VERIFIED：10/10 项 required criterion 满足，missing、被拒绝的证据、invalidated 与 unresolved candidate impact 均为空。

Task checkpoint：ad-auxiliary-session-classification-settings-control；content_state=8e6d3ac3d00ba1bfd3f3137733f2176e676373c160857503edfdf123dc64ae81；gate=VERIFIED
提交建议：本地提交任务 6 的完整边界，共 16 个路径：14 个产品/测试/脚本路径，加 `docs/topics/auxiliary-session-classification/tasks.md` 与本评审记录。用户在本轮已授权「门禁通过后本地提交」，不推送。
推送建议：推送到 origin 的 `feature/auxiliary-session-classification` 需另行授权；推送前先核对提交对象、署名与 SSH 签名。
