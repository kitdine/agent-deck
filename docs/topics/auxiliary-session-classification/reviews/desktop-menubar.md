---
status: active
topic: auxiliary-session-classification
subject: desktop-menubar
created: 2026-11-06
updated: 2026-11-07
---

# desktop-menubar 评审记录

## Round 1 — 2026-11-06

### 📋 菜单栏实现独立评审

📊 总体评分：6/10

✅ 评审结论：FAIL

Reviewer：Codex。
Method：独立源码评审、已批准 UX/架构对照、实际控制器源码与共享 Swift DTO 的隔离异步反例。
Scope：任务 5 `desktop-menubar` 当前 32 个产品、测试及 fixture 路径；重点核对默认值、选择与发布代次、
Popover 生命周期及只读接口。发现确定的阻塞反例后停止扩大产品验证，未宣称其余路径已完整通过评审。
生产代码、测试、配置均未修改；本轮只维护评审、专题状态、CEv1 和 Beads。

### 🔴 严重问题 — 必须修复

#### ASC-DM-R1-F1 · P1 · OPEN — 返回已缓存选择没有废弃在途请求

位置：`apps/macos/AgentDeckApp/SessionScopeController.swift:94`（缓存命中直接回到 idle）；
同文件 `:163`（答复只核对 generation）、`:169`（接受答复）。

- 行为风险：用户从 All clients 切到 Codex，在读取结束前切回 All clients。第二次同步命中已接受的
  All 视图而直接返回，没有推进 generation。先前 Codex 请求随后仍通过代次检查，把当前会话区域替换成
  Codex。当前 client 控件为 All，控制器却以 idle 展示已被取代选择的数据；之后没有新的同步 key 变化来纠正。
- 证据：将当前生产控制器的完整 class 原样编译，使用真实 `DesktopSessionScopeViewV1` 解码的合成视图及
  continuation 控制答复时机。先接受 `work/all/today`，启动 `work/codex/today`，取消旧 Swift Task，
  再同步回 `work/all/today`，最后释放 Codex 成功答复。实际输出：

  ```text
  before late response: selected=all accepted=all request=idle
  after late response: selected=all accepted=codex request=idle
  FAIL: superseded Codex response replaced the All view
  ```

- 取消不是答复身份校验：Swift 取消是协作式的。`EmbeddedHelperRunner.swift:245` 在进程退出后检查取消，
  之后仍有输出收束、DTO 解码及回到 MainActor 的窗口；`sessionScopeView` 和控制器接受结果前没有另一次
  取消检查。探针的成功答复代表已产生但迟到交付的结果，不是实际 helper/SQLite 运行证据。
  `MenuBarPanelViews.swift` 的 `.task(id:)` 会取消旧任务，但不能替代 controller 的过期答复守卫。
- 合同：`ux/menubar.md` 的“临时选择、默认与生命周期”及“已定 DTO 与接收规则”要求后发选择覆盖先发请求，
  且 client/period/scope、列表和统计一致。

💡 修复建议：当新的目标选择命中缓存时，也使此前请求失效；在结果接受边界核对当前请求身份和目标选择。
补充 All → Codex → All 以及 period 往返的回归，覆盖取消后仍返回成功、迟到错误两种结果，确认最终 selection
始终等于当前目标。不得只靠底层进程通常响应取消来满足合同。

#### ASC-DM-R1-F2 · P2 · OPEN — publication 更新取消旧读取后，新同步被提前吞掉

位置：`apps/macos/AgentDeckApp/SessionScopeController.swift:91`（loading 只按 selection 去重）；
同文件 `:176`（取消后回到 idle）、`:355`（同步 key 包含 publication）；
`apps/macos/AgentDeckApp/MenuBarPanelViews.swift` 的 `.task(id:)`（key 变化取消并重建同步任务）。

- 行为风险：当前 Work/pub-1 可见，Auxiliary 请求尚未结束。刷新接受 pub-2 后，SwiftUI 取消旧任务并启动
  新同步；新同步发现同一个 selection 仍在 loading，直接返回。旧任务再以 cancelled 结束，控制器回到 idle，
  没有新请求、失败状态或重试入口。用户请求的 Auxiliary 没有生效，仍显示 Work/pub-1；点击已请求的
  Auxiliary 也不会改变 synchronizationKey，不能自行恢复。
- 证据：同一实际控制器，以取消感知的合成 reader 严格控制先后顺序：接受 `work/all/today`/pub-1，
  发起 Auxiliary，挂起答复；取消旧 Task；同步同一 Auxiliary/pub-2；最后返回
  `HelperExecutionError.cancelled`。实际输出：

  ```text
  after publication change + cancellation: requested=auxiliary accepted=work request=idle acceptedPublication=pub-1
  FAIL: new synchronization returned while old request was loading; cancellation left no request or retry
  ```

- 这与 F1 的“旧成功结果被接受”不同：即使 reader 正确返回取消错误，仍会丢失当前目标读取。
  现有 `testANewPublicationKeepsTheLastSuccessfulViewWhileItIsReread` 从先前请求已完成的状态开始，
  没覆盖 publication 在读取进行中变化的顺序。
- 合同：同一窗口持有请求与成功 selection；刷新、切 tab、关闭重开不得丢失正确恢复状态，也不能把未完成
  请求默认为成功。旧数据只能作为有明确状态与时间的完整旧视图保留。

💡 修复建议：请求去重身份包含 publication/请求代次，并协调取消与替换请求的所有权；取消旧请求不能清除或
  吞掉当前所需的新读取。补充“请求进行中 publication 改变”的确定性测试，覆盖旧取消先返回和新同步先进入
  两种顺序，断言当前目标继续读取或完成，最终数据属于当前目标且有正确状态。

### 🟡 改进建议 — 建议处理

无。以上两项均为本任务必须修复的行为缺陷。

### 🟢 做得好的地方

- 新视图和旧 `sessions` 字段分开，控制器没有把未确认默认静默回退为 Work。
- 临时范围通过显式只读接口读取，控制器自身不写偏好；来源原因码有受控映射和未知码回退。
- 已有测试覆盖普通乱序、关闭后重开、失败重试等场景，为上述边界补充回归提供了现成夹具。

### 📝 总结

Reviewed state：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1`；32 路径候选摘要
`4b617b19e721878b93d70ec6ecd0d17f13502c01d76140e98b3cc8499fa8bbea`。
摘要算法沿用开发交接：UTF-8 `head=<HEAD>\n`，随后按路径字节序拼接每条 `path=<git blob>\n`，对整体取 SHA-256。
专题 `tasks.md` 和评审记录不属于产品指纹；全部路径/blob 清单保存在下述 ContentState 的 `subjects_json`。
关键源码 blob：`SessionScopeController.swift` 为 `f512b487904b2253ab32ab8d3b8ca54341ee7727`，
`MenuBarPanelViews.swift` 为 `00b4426a0b86d6298bfb5808cfe62765b29be96b`，
`EmbeddedHelperRunner.swift` 为 `9b07c406652a991808a0414a1b56facaba28b227`。

Evidence：

1. 独立重算全部 32 路径的 blob 清单和摘要，与开发交接
   `d53db3b6-57bd-51d5-a6d3-7e66d75cece1` 及 `tasks.md` 一致。工作区
   `agent-deck.auxiliary-session-classification`、分支 `feature/auxiliary-session-classification`，绑定与 live Git 一致。
2. 两个隔离反例直接截取当前 `SessionScopeController.swift` 中 `// MARK: Presentation` 之前的完整 class，
   保留 Observation 宏，不修改控制流；通过 `xcrun swiftc -parse-as-library -framework AgentDeckShared`
   编译，链接既有 Debug 共享模块，使用 `CheckedContinuation` 明确安排上述时序。第二个反例的 reader
   返回取消错误；第一个返回已完成的迟到成功结果。初次沙箱编译因宏插件 `sandbox_apply: Operation not permitted`
   未产生业务结论；获准执行同一探针后，两次目标运行均 exit 1，且最终组合探针输出上述两个 FAIL。
3. 环境为 Xcode 26.4 / build 17E192，Apple Swift 6.3 / swiftlang-6.3.0.123.5，目标
   `x86_64-apple-macosx26.0`。所链接共享模块二进制 SHA-256 为
   `83a91526509a2590a29002542234dc01e8689c07873b8b337a894408dc0f204a`；本轮证明的是控制器时序，
   不是该二进制的完整构建溯源或原生 Popover 人工验收。探针不接触用户数据库、偏好、真实会话或运行中 App。
4. 开发交接记录 Go 全量/vet/race 和 macOS 测试通过，但未在 CEv1 登记对应的精确状态证据。本轮未把该文字
   交接升级为独立通过证据，也未在确定阻塞后重跑全量套件。键盘焦点往返和 VoiceOver 的原生验收仍未验证；
   它属于 `native-acceptance` 的证据缺口，本轮不虚构产品缺陷或通过结论。
5. 文档检查使用 `python3 scripts/check-docs.py` 的默认工作树模式；初次仅提供 `--base HEAD`，
   因缺少配对 head 参数而产生 NoneType 异常，不作为候选内容失败。另运行
   `python3 scripts/check-review-records.py docs/topics/auxiliary-session-classification/reviews/desktop-menubar.md`、
   `make check-whitespace` 和 `git diff --check`。最终重新核对全部 32 个产品路径，blob 和 HEAD 均未变。

CEv1 namespace：`github.com/kitdine/agent-deck`。
WorkUnit：`auxiliary-session-classification:desktop-menubar`。
ContentState：`asc-desktop-menubar-review-r1-20261106-4b617b19-state`。
完成门禁：FAILED

入口查询确认 9 项 required criteria 及关系完整，尚无绑定该候选的通过证据；无失效证据或未决候选影响。
本轮失败反例及独立评审结论登记至 `selection-consistency` 和 `independent-review`；最终门禁确认两项失败证据
均匹配当前 target，applicable=true，malformed/invalidated/unresolved=false；2 个 evidence 节点和 4 条关系
均已预检、写入并读回核对数量。其余判据保留未验证边界，
不以开发者文字交接冒充完成门禁。两项发现均 OPEN，任务 Review 保持未勾选，不到达提交检查点。

下一步指令：`修复：auxiliary-session-classification / desktop-menubar / ASC-DM-R1-F1、ASC-DM-R1-F2`

WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Repair 1 — 2026-11-06

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-DM-R1-F1、ASC-DM-R1-F2 及其请求生命周期回归；未处理未登记问题。
- 复现：把 Round 1 评审的 `SessionScopeController.swift` 按 blob `f512b487904b2253ab32ab8d3b8ca54341ee7727` 逐字节还原（重算一致），
  与下述新回归一起跑隔离套件：两项新测试全部失败，共 27 处断言，其余测试无失败，与记录的两个反例一致。
  F1：迟到的成功答复把 All/today 换成 Codex/7d；迟到错误使当前选择显示 failed 和重试。
  F2：新同步先进入时 request 停在 idle、只发出 1 次读取、Auxiliary 不生效。
  旧取消先返回的顺序在评审内容上本就通过，与记录相符。

**ASC-DM-R1-F1 → 修复，待复评。**

- 变更：[SessionScopeController.swift](../../../../apps/macos/AgentDeckApp/SessionScopeController.swift) 的 `synchronize`
  命中已接受视图时先推进 `generation` 再回到 idle，离开该选择时发出的读取随即失效；`load` 的成功与失败两条接受路径改为
  `isCurrent(id, selection)`，要求代次仍为当前且 `request == .loading(selection)`，不再仅凭代次接受答复，也不依赖 Swift 取消。
- 回归：[SessionScopeControllerTests.swift](../../../../apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift)
  `testReturningToTheShownSelectionDiscardsTheReadThatLeftIt`：client（All → Codex → All）与 period（today → 7d → today）往返，
  各覆盖取消后成功、取消后错误、未取消成功、未取消错误四种迟到答复。断言最终 accepted selection 与面板 selection 都是 `work|all|today`，
  request 为 idle，无失败原因或重试入口，按钮可用，往返本身不发出新读取。

**ASC-DM-R1-F2 → 修复，待复评。**

- 变更：同文件 `synchronize` 去掉对进行中读取的 `.loading(pending) where pending == desired` 去重。
  每次同步都代表当前目标，需读取时一律发起新读取并推进代次，取代任何在途读取，包括同一 selection 的读取。
  在途读取属于上一次同步，SwiftUI 在 key 变化或区域离开时已取消它；再对它去重只会等一个可能永不被接受的答复。
  旧读取以取消结束时代次已过期，不能再把 request 清回 idle。失败选择仍只等待显式重试，未改动。
- 回归：同文件 `testACancelledReadNeverSwallowsTheSynchronizationThatReplacesIt`。
  覆盖 publication 变化与离开后返回（publication 不变）两种来源，每种都测旧取消先返回和新同步先进入两种顺序。
  旧读取以 `HelperExecutionError.cancelled` 结束，断言当前目标仍在读取，旧视图保留原标签；
  完成后显示 `auxiliary|all|today`、acceptedPublication 为当前 publication、request 为 idle、无重试，且恰好两次读取。
  为此 [AppTestFixtures.swift](../../../../apps/macos/AgentDeckAppTests/AppTestFixtures.swift) 的 `StubSessionScopeReader`
  增加按请求序号覆盖答复的 `answer(request:with:)`。
- 验证：`scripts/test-macos-app.sh`（隔离 HOME）跑两次修复后候选。
  第一次 185 项中 2 处失败，均在既有 `MenuBarChromeTests.testInitialPopoverPreparationDoesNotShowAWindowOrMoveFocus`：
  它断言全局 `NSApp.windows` 集合，前一测试的状态栏场景仍在断开，多出一个可见窗口。
  第二次在相同内容上 TEST SUCCEEDED（185 项，1 skipped，0 failures），该测试通过，判为测试间时序抖动，不属本次范围，未修改。
  两项新回归在两次运行中都通过。`make check-whitespace` 与 `git diff --check` 通过。Go 侧无改动，未重跑。
- 候选：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1` + 同一 32 个路径，摘要
  `18dffc5cc5db2db0219fe6c909e1355b09dcd7964285dc753e30ffa0694ae7d7`（算法同 Round 1；以同法重算 Round 1 清单仍得 `4b617b19…`）。
  变化的 blob：SessionScopeController.swift `c5b4ccc2a932c3d8efaca767eab36754a65fd1be`、
  AppTestFixtures.swift `c54b1aede33cf9fcb19b75c25f530aec39b3e3b7`、
  SessionScopeControllerTests.swift `4882be9d995062fd37d5823025a3453667cbf070`；其余 29 个路径 blob 与 Round 1 相同。
- CEv1：Repair 不宣称门禁完成，未写入新证据；Round 1 的 FAILED 证据绑定旧 target，新候选的证据由复评按上述摘要绑定。
  `native-acceptance`（原生键盘焦点往返与 VoiceOver）仍未验证，不属本次两项发现。

修复后状态：ASC-DM-R1-F1、ASC-DM-R1-F2 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 2 — 2026-11-06

### 📋 desktop-menubar 任务复评

📊 总体评分：7/10

✅ 复评结论：FAIL

Reviewer：Codex。
Method：独立源码与修复差异核对；当前生产控制器原样编译的确定性异步探针；隔离数据库的 Go producer 反例；
实际 XCTest 结果读取。Scope：原两项发现及其影响路径，并继续 Round 1 因阻塞而未完成的任务级 wire、发布和会话区检查。
新阻塞问题取得确定证据后停止扩大验证。未修改生产代码、测试或配置，也未执行 Git 交付。

### 🔴 严重问题 — 必须修复

#### ASC-DM-R2-F1 · P2 · OPEN — 文本会话数为零会隐藏仍然有效的工作信号

位置：`apps/macos/AgentDeckApp/MenuBarViewModel.swift:1054`；
显示入口：`apps/macos/AgentDeckApp/MenuBarPanelViews.swift:780`。

- 处置：NEW。该问题属于本任务的范围视图接入，不回开已修复的两个请求时序发现。
- 行为风险：Work 的文本索引集合可以为空，同时仍有未关联文本会话的 usage-only 工作信号。
  `workSignalPanel` 在查看三族的 available/items 之前，仅凭 `counts.sessions == 0` 返回 `.empty`、
  清空全部 summaries/rows；SessionsPanelView 又完全隐藏 `.empty` 的信号区。用户因此看不到真实的
  Activity、Workflow、Tooling，也看不到关联覆盖不足的说明，容易把“没有可见文本会话”理解成“没有工作活动”。
- 合同：[requirements.md](../requirements.md) 的范围规则明确要求未关联 canonical 会话的信号属于 unknown，
  同时不伪造文本会话；[ux/menubar.md](../ux/menubar.md) 要求三族独立表达 availability。
  会话索引集合与 usage 信号的数据域不同，不能用前者是否为空决定后者是否存在。
- 生产路径证据：通过外部 Go overlay 增加一个仅用于本轮的诊断测试，复用 `internal/desktop/session_scope_test.go`
  的隔离 fixture helpers。创建一条已索引 auxiliary 会话 `indexed-maintenance`，再创建一条没有文本索引的
  Codex usage 会话 `usage-only-work`，含 coding/feature 事件和一次 edit `main.go`。
  时间为 `2026-08-13T08:00:00Z`，沿用 fixture 的 `2026-08-13T10:00:00Z` 时钟；调用真实
  `Service.SessionScopeView` 的 work/all/today 与 all/all/today，并调用 `Service.Build` 获取旧字段。
  测试前提断言全部通过，实际输出：

  ```text
  legacy sessions=1; all scoped sessions=1; work scoped sessions=0; work tooling available=true calls=1
  ```

  Work DTO 同时包含 `available=true`、`counts.sessions=0`、`recent=[]`；Activity available=true 且
  coding/feature events=1，Workflow available=true 且 files_touched=1，Tooling available=true 且
  calls=1、edit share=100。三族 selection 都是 all/today。Swift 当前上述 guard 必然先返回 `.empty`，
  这些有效字段不会进入后续呈现分支。这里的运行证据来自真实 Go producer，消费者结论来自明确的源码分支；
  未声称已经运行原生 Popover 端到端反例。
- 变更归属：旧实现也有空集合 guard，但输入为未加 session scope 的 `statistics.sessions`；同一 fixture
  该值为 1。此次接入改为传入 Work 的 `counts.sessions=0`，触发了新范围下对仍应保留的 unknown 信号的隐藏。
  这不是要求本轮修复所有旧的零索引场景。

💡 修复建议：三族信号的呈现由各自的 availability/items 和有效内容决定，独立于文本会话数；
零文本会话可以保留真实会话空态，同时显示仍可用的信号及适用覆盖说明。不要制造假会话或修改用量归属来绕过 guard。
补充上述“全部已索引会话均为辅助 + Work 仍有 usage-only 信号”的 Go/Swift 连通回归，覆盖独立不可用族，
并确认切换范围仍不改变总用量、费用和额度。

### 🟡 改进建议 — 建议处理

无。XCTest 宿主启动失败与原生人工验收缺口单独记录为证据限制，未冒充本轮已定位的产品缺陷。

### 🟢 已关闭发现与改进

- **ASC-DM-R1-F1 → CLOSED**：`SessionScopeController.swift:95` 的缓存返回分支先推进 generation；
  成功和失败路径均经 `isCurrent(id, selection)` 核对 generation 与 `.loading(selection)`。
  独立探针覆盖 client/period 往返 × 取消/未取消 × 迟到成功/错误，**8/8 PASS**；迟到错误也不能污染当前状态。
- **ASC-DM-R1-F2 → CLOSED**：`synchronize` 不再把旧的同 selection 在途请求当作可等待的当前读取；
  新读取拥有新的 generation，旧取消不能覆盖它。独立探针覆盖 publication 改变/不变 × 旧取消先返回/新同步先进入，
  **4/4 PASS**，始终恰好两次读取，最终为 auxiliary/all/today、当前 publication、idle。
- 新增两项回归测试的断言与上述行为一致；`StubSessionScopeReader.answer(request:with:)` 能明确控制每个请求的
  成功或失败，不依赖取消通常足够快的假设。

### 📝 总结

| 发现 | 当前处置 | 依据 |
| --- | --- | --- |
| ASC-DM-R1-F1 | CLOSED | 当前源码 + 8 个确定性反例通过 |
| ASC-DM-R1-F2 | CLOSED | 当前源码 + 4 个取消/替换顺序通过 |
| ASC-DM-R2-F1 | OPEN | 真实 producer 输出零文本会话与非空信号；Swift 明确丢弃该合法状态 |

Reviewed state：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1`；同一 32 路径候选摘要
`18dffc5cc5db2db0219fe6c909e1355b09dcd7964285dc753e30ffa0694ae7d7`，算法沿用 Round 1。
独立重算与 Repair 1 一致，仅以下三个 blob 相对 Round 1 改变，其余 29 个未变：

| 路径 | 当前 blob |
| --- | --- |
| apps/macos/AgentDeckApp/SessionScopeController.swift | `c5b4ccc2a932c3d8efaca767eab36754a65fd1be` |
| apps/macos/AgentDeckAppTests/AppTestFixtures.swift | `c54b1aede33cf9fcb19b75c25f530aec39b3e3b7` |
| apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift | `4882be9d995062fd37d5823025a3453667cbf070` |

Evidence：

1. 当前控制器在 `// MARK: Presentation` 之前的完整 class 原样编译，保留 Observation 宏，链接真实
   AgentDeckShared DTO；使用 `CheckedContinuation` 控制 12 个顺序，执行 exit 0。Swift 6.3 / Xcode 26.4，
   共享模块 SHA-256 仍为 `83a91526509a2590a29002542234dc01e8689c07873b8b337a894408dc0f204a`。
   这是控制器证据，不是 App 宿主、实际 helper 或原生辅助功能验收。
2. Go 反例通过必用 `scripts/run-go-test.sh`，参数为 `-overlay <本轮生成的隔离映射> ./internal/desktop
   -run '^TestReviewWorkSignalsWithoutTextSessions$'`，用合成状态验证上述输出，exit 0、测试耗时 0.08s。
   首次执行尚未进入测试：PATH 的 `/usr/local/bin/go` 为 1.27.1，GOROOT 指向已安装 1.27.3，出现编译器版本不匹配。
   仅本次调用设置 `GO_TEST_BIN=/Users/jobshen/.local/share/mise/installs/go/1.27.3/bin/go` 后同一探针通过；
   没有修改系统配置。此编译失败归类为已确认的工具链选择不一致，不是产品失败。
3. 直接读取既有 XCTest 结果：07:43:20 开始的一份为总数 362、361 passed、1 skipped、0 failed；
   07:46:16 的一份为 359 passed、1 skipped、2 failed，失败的两项恰是新增回归，符合 Repair 1 的旧源码对照描述。
   结果包没有足够的精确源码身份，不能按文件时间把最后一份当成当前候选，也未据此重开两项发现。
4. 为取得当前源码的定向 XCTest 证据，在隔离 HOME 下执行 `xcodebuild -project apps/macos/AgentDeck.xcodeproj
   -scheme AgentDeck -configuration Debug -derivedDataPath apps/macos/build/DerivedData CODE_SIGNING_ALLOWED=NO
   CODE_SIGNING_REQUIRED=NO CODE_SIGN_IDENTITY= -only-testing:AgentDeckAppTests/SessionScopeControllerTests test`。
   构建完成后 exit 65：`Early unexpected exit ... Test crashed with signal ill before establishing connection`。
   摘要为 0 passed，仅一个宿主错误记录，没有到达测试断言。
   诊断分类为 **Unresolved**：失败位置确定在测试宿主 bootstrap，不能在缺少 crash stack 的情况下认定为测试、
   产品或环境根因；“两项新回归断言失败”已被本次运行未进入测试的事实排除。未修复环境或重试同一宿主；
   下一项有区分力的诊断是核对宿主 crash stack 与测试启动/构建条件。Xcode 在该构建中执行了默认
   `RegisterWithLaunchServices`，目标为本工作树的 Debug AgentDeck.app；没有另行运行安装、卸载或注册修复命令。
5. 原生键盘焦点往返与 VoiceOver 仍未验收；上述宿主失败也未被独立控制器探针掩盖。
   未改变的 Go 全量/vet/race 未重复运行。新增范围反例确定后停止扩大验证，不声称剩余任务验收已完整覆盖。
6. 最后逐 blob 核对全部 32 个产品路径和 HEAD，均保持本轮候选；文档与记录结构、空白及 diff 检查在交接前执行。
   必要输入、观察和输出保留在本记录中，不依赖可清理的诊断日志、结果包或 overlay 路径。

CEv1 namespace：`github.com/kitdine/agent-deck`。
WorkUnit：`auxiliary-session-classification:desktop-menubar`。
ContentState：`asc-desktop-menubar-rereview-r2-20261106-18dffc5c-state`。
完成门禁：FAILED

入口门禁为 NOT_VERIFIED，9 项 required criteria 完整；Round 1 的两条失败证据 target_matches=false、applicable=false，
保留其历史意义。两项原发现的通过只证明此次修复边界；新发现阻止本任务完整候选通过。原生验收与宿主诊断限制独立保留。
最终查询确认新 `independent-review` 失败证据、原发现关闭的有限范围证据和 `native-acceptance` blocked 证据均匹配当前
target，applicable=true、malformed=false；无失效证据或未决候选影响。3 个 evidence 节点与 6 条关系均已写入并读回核对，
关系写入前的全部预检为 ok。有限范围通过未冒充整个 `selection-consistency` 判据的验收通过。
Review 保持未勾选，任务回交修复，不到达提交检查点。

下一步指令：`修复：auxiliary-session-classification / desktop-menubar / ASC-DM-R2-F1`

WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Repair 2 — 2026-11-06

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-DM-R2-F1 及其信号区呈现回归；未处理未登记问题，ASC-DM-R1-F1/F2 的修复未改动。
- 复现：把 Round 2 的三个生产文件按 blob 逐字节还原（重算一致）：
  MenuBarViewModel.swift `cc9bc138…`、MenuBarPanelViews.swift `00b4426a…`、SessionScopeController.swift `c5b4ccc2…`。
  与下述最终测试一起跑隔离套件：TEST FAILED（186 项，6 处失败），失败全部落在两项目标测试内，
  signal state 为 `empty`、summaries 为 `[]`，与记录一致。
  首次复现所用测试版本在状态断言失败后按下标读取空 summaries，致测试宿主崩溃重启。
  已在测试中加 count guard 后重跑，上述数字取自重跑，无宿主重启。

**ASC-DM-R2-F1 → 修复，待复评。**

- 变更：[MenuBarViewModel.swift](../../../../apps/macos/AgentDeckApp/MenuBarViewModel.swift) 的 `workSignalPanel` 去掉 `sessions` 参数和
  `(sessions ?? 0) > 0` guard，信号区只由三族各自的 available/item 决定：
  - 不可用族进入 `uncapturedSections`，显示既有“尚未采集”标记；
  - 可用而无本选择 item 的族显示“—”，沿用既有 `testCapturedWorkSignalScopeKeepsEachMissingFamilyHonest` 的规则；
  - 无 signals 为 `.uncaptured`。
  `WorkSignalPresentationState.empty` 已无产生方，随之删除。
  [MenuBarPanelViews.swift](../../../../apps/macos/AgentDeckApp/MenuBarPanelViews.swift) 的信号区只要求 `scope.state == .ready`，
  删除仅为 `.empty` 关闭信号详情的 `onChange`；选择变化关闭详情的既有 handler 不变。
  [SessionScopeController.swift](../../../../apps/macos/AgentDeckApp/SessionScopeController.swift) 的 6 处调用随签名去掉 `sessions:`。
  会话列表自身的空态（`sessionScopeEmpty` 提示与“没有匹配会话”）不变；不伪造会话，不改用量归属。
- 依据：本专题原型 `prototype/src/AuxiliarySessions.jsx` 在包括 empty 的每个状态都渲染三族信号，各带自身状态；
  [ux/menubar.md](../ux/menubar.md) 要求各信号族独立降级。`desktop-app` 旧原型的 empty 整页留白属于未分范围的单一数据域，
  不再适用于会话索引与 usage 信号分属不同数据域的范围视图。
- 覆盖说明的解释：架构与 wire 没有“未关联 canonical 会话的信号”专用字段，原型也没有对应文案。
  本次把“适用覆盖说明”实现为既有的逐族可用性标记（不可用族的“尚未采集”），
  与会话列表的空态提示并列显示，没有新增文案。若复评认为需要专门的数据域说明行，那是新的 UX 文案决定，需另行确认。
- 回归：
  - Go：[session_scope_test.go](../../../../internal/desktop/session_scope_test.go) `TestSessionScopeViewKeepsUsageOnlySignalsWhenNoIndexedSessionIsInScope`
    固化 Round 2 的 producer 反例：已索引 `indexed-maintenance` 归辅助，无文本索引的 `usage-only-work` 含 coding/feature 事件与一次 edit `main.go`。
    断言旧 sessions total=1、All 会话数 1、Work `counts.sessions=0`、projects=0、`recent=[]`；Work 三族 available，
    coding/feature events=1/1、files_touched=1、tooling calls=1 且 edit share=100，信号项均为 today/all；
    Auxiliary 会话数 1，且其 tooling 不含该 usage-only 信号。
  - Swift：[SessionScopeControllerTests.swift](../../../../apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift)
    `testWorkSignalsStayVisibleWhenTheSelectionListsNoSession` 用与上述 Go 输出同形的 Work 视图
    （0 会话、空列表、三族信号），分两种情况：三族可用，以及 Activity 独立不可用。
    断言会话区为 ready 且保留空态提示，信号区为 captured。三族全可用时值来自真实项；Activity 不可用时只有该族进入 `uncapturedSections`、
    其余两族照常显示。依次切换 Auxiliary/All/Work 时 hero、usagePanel、quotaClients、客户端标签都不变，回到 Work 后信号与切换前相同。
  - 同时更新既有测试：[MenuBarViewModelTests.swift](../../../../apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift)
    原 `testWorkSignalStateDistinguishesEmptyScopeFromLegacyUncapturedPayload` 改名为
    `testAnEmptySessionSelectionKeepsItsSignalFamiliesAndALegacyPayloadStaysUncaptured`。
    Claude/today 无会话、信号族可用但无项，原期望 `.empty` 正是本发现指出的耦合；
    现期望会话区为空态、信号区 captured 且三族为“—”。legacy 与整族不可用仍为 `.uncaptured`，不变。
- 验证：
  - `scripts/test-macos-app.sh`（隔离 HOME）在修复后候选上运行两次，均 TEST SUCCEEDED：第一次为加 guard 之前的测试版本，
    第二次为最终版本（186 项，1 skipped，0 failures）。
  - `scripts/run-go-test.sh ./internal/desktop` 全包通过，新 Go 测试单独以 `-v` 确认已执行并通过；`go vet ./internal/desktop` 通过，
    `gofmt -l` 对改动的测试文件无输出。Go 生产代码无改动，未重跑全量。
  - `make check-whitespace`、`git diff --check`、评审记录与文档结构检查在交接前执行。
- 候选：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1` + 同一 32 个路径，摘要
  `f0dc2376dda30994c83fec4527e17fb8408b61c51ef143748b69d49a139aeb99`（算法同 Round 1；全部 32 个 live blob 已逐一核对）。
  相对 Round 2 变化的 blob：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/MenuBarPanelViews.swift | `5a02e49b93eb933f58720060831b14ba06f71929` |
  | apps/macos/AgentDeckApp/MenuBarViewModel.swift | `763e7eb1c22cbabdc7c46e8682a058b69a77a79c` |
  | apps/macos/AgentDeckApp/SessionScopeController.swift | `bfc695007dce1677b43f3cc4bdd58fe53881345e` |
  | apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift | `3a85161ebca1be6a31576a99ee8504f02111c613` |
  | apps/macos/AgentDeckAppTests/SessionScopeControllerTests.swift | `13eb284ee9614b9dba127d47f878baab7c1e1dd9` |
  | internal/desktop/session_scope_test.go | `7278809a8b513a85fa4a5030f44a48afe855f22a` |

  其余 26 个路径 blob 与 Round 2 相同。
- CEv1：Repair 不宣称门禁完成，未写入新证据；Round 2 证据绑定旧 target，新候选证据由复评按上述摘要绑定。
  `native-acceptance`（原生键盘焦点往返与 VoiceOver）仍未验证，不属本次发现。

修复后状态：ASC-DM-R2-F1 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 3 — 2026-11-06

### 📋 desktop-menubar 任务复评

📊 总体评分：8/10

✅ 复评结论：FAIL

Reviewer：Codex。
Method：独立核对 Repair 2 的六个变更路径、原有请求时序守卫、批准的范围文案合同、Go producer 回归及实际 XCTest 结果。
Scope：ASC-DM-R2-F1 及影响路径，并继续任务级合同与证据核对。新合同缺口确认后停止扩大验证。
本轮未修改生产代码、测试或配置，未提交、推送或启动下一任务。

### 🔴 严重问题 — 必须修复

#### ASC-DM-R3-F1 · P2 · OPEN — 范围说明仍缺少已批准的数据域区别

位置：`apps/macos/AgentDeckApp/DesktopCopy.swift:155` 的 `sessionScopeNote`，
及 `Localizable.xcstrings` 对应中英文条目；`MenuBarPanelViews.swift` 的 scopeHeader 同时把该文案用于可见说明和 accessibilityHint。

- 处置：NEW。ASC-DM-R2-F1 的信号隐藏缺陷已关闭；本项是独立的、尚未落实的呈现合同。
- 合同依据：[ux/menubar.md](../ux/menubar.md) 第 57 行明确要求“工作页数量来自会话索引，usage 事件聚合的会话数可能不同；
  范围说明保留数据域差异”。[requirements.md](../requirements.md) 第 109–111 行也要求未可靠关联 canonical 会话的信号
  保持 unknown 并说明覆盖边界。该要求已经批准，不能因为现有 wire 没有单独的“未关联信号”字段而作为新 UX 决策推迟。
- 当前事实：实际英文是 `Affects sessions and work analysis only. Usage and costs include all sessions.`；
  zh-Hans 是 `仅影响会话与工作分析。总用量和费用保留全部。`。两者只说明范围对用量的影响，没有交代文本会话索引
  与 usage/work-analysis 信号的来源不同。项目数注释也只说明 distinct project 可能重叠。
- 行为风险及场景：已被新 Go/Swift 回归确认的合法状态是 0 个文本会话、空列表，但有 1 次工具调用和 1 个文件活动。
  当前修复会正确展示这些数字，却没有合同要求的解释，用户无法从面板判断它们为何并存，或区分文本会话的 unknown 数与信号关联覆盖。
  Repair 2 所述“尚未采集”仅对应某族 available=false；该场景三族 available=true，因而不会出现这种标记，
  它也不能说明已存在信号与文本会话未关联的边界。
- 证据：逐项读取当前 `sessionScopeNote`、真实 en/zh-Hans catalog 值及 scopeHeader 的可见/AX 绑定；
  `TestSessionScopeViewKeepsUsageOnlySignalsWhenNoIndexedSessionIsInScope` 在本轮全量测试中通过；
  留存 XCTest 的 `testWorkSignalsStayVisibleWhenTheSelectionListsNoSession()` 单例结果为 Passed，0.076s。
  本项是明确文本合同与静态绑定的差异，不以未执行的截图或 VoiceOver 播报作证。

💡 修复建议：在已有范围说明中用中英文解释“会话数量来自文本会话索引；工作分析可能包含未关联文本会话的用量记录”，
保留现有总用量/费用不变的说明，并同步可见文本与无障碍 hint。无需新增 unknown 数值、wire 字段、数据库接口或重新设计布局。
复用既有原生渲染/AX 检查核对中英及 280/420pt；本轮已经通过的 Go L3 证据不因纯文案变化重复运行，按实际影响复用。

### 🟡 改进建议 — 建议处理

无。原生键盘与 VoiceOver 验收仍是独立证据缺口，不把未执行的检查写成已定位产品缺陷。

### 🟢 已关闭发现与改进

- **ASC-DM-R2-F1 → CLOSED**：`workSignalPanel` 已移除 sessions 参数及零会话 guard，三族按自身 available/item 呈现；
  `.empty` 信号状态和隐藏入口已移除。会话列表自身仍显示真实空态，既不制造文本会话，也不改用量归属。
  Go 回归锁定 usage-only producer 反例；Swift 回归覆盖三族可用、Activity 独立不可用、范围往返和 hero/usage/quota/client 标签不变。
- **ASC-DM-R1-F1、ASC-DM-R1-F2 → 保持 CLOSED**：本轮检查当前 `synchronize`、`load`、`isCurrent`，
  缓存返回推进 generation、每次替换读取取得新代次及接受边界检查均保持；当前控制器变更限于信号模型调用参数。
  复用 Round 2 的 12 个确定性时序反例，不因换轮次重复执行。
- 本轮 Go 全量、全量 vet、desktop/sessionorigin race 均通过；新增 producer 回归实际执行并通过。

### 📝 总结

| 发现 | 当前处置 |
| --- | --- |
| ASC-DM-R1-F1 | CLOSED，保持 |
| ASC-DM-R1-F2 | CLOSED，保持 |
| ASC-DM-R2-F1 | CLOSED |
| ASC-DM-R3-F1 | OPEN，仅补齐既定范围说明 |

Reviewed state：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1`；32 路径候选摘要
`f0dc2376dda30994c83fec4527e17fb8408b61c51ef143748b69d49a139aeb99`，算法沿用 Round 1。
独立重算与 Repair 2 完全一致：仅该节列出的六个 blob 相对 Round 2 改变，其余 26 个未变。
工作区 `agent-deck.auxiliary-session-classification`、分支及 Git common directory 已核对；产品指纹不包含评审/状态记录。

Evidence：

1. 在 current candidate 上通过必用 wrapper 运行 `scripts/run-go-test.sh ./...`，随后 `go vet ./...`，
   再运行 `scripts/run-go-test.sh -race ./internal/desktop ./internal/sessionorigin`，全部 exit 0。
   runner 的 `GO_TEST_BIN` 与 vet 均明确使用现有 Go 1.27.3，与 GOROOT 匹配；GOCACHE 为项目规定的隔离路径，
   未修改系统配置。新增 producer 回归耗时 0.34s、PASS；全量 cmd/agentdeck 包 209.596s，race sessionorigin 包 52.126s。
   这是此前尚未绑定当前候选的 Go L3 门禁证据；没有再叠加执行同一组定向 Go 测试。
2. 直接读取 Repair 2 的最终 XCTest 摘要：2026-11-06 08:43:12 EST 开始的运行共 363 项，362 passed、1 skipped、0 failed。
   Repair 2 所记的 186 项不能代替全 scheme 计数；此处保留工具直接返回的数据。另读新增 usage-only 单例的 test-details，确认实际 Passed、
   运行 1 次、0.076s。结合修复交接、当前六个 blob 与源码断言核对，支持关闭信号隐藏发现；没有重跑整个 macOS 套件。
   结果包的摘要不单独证明所有 build/runtime 输入，也不替代原生键盘与 VoiceOver 验收。
3. 静态对照批准 UX 第 57 行、requirements 第 110 行、实际两种语言的 catalog 和 SwiftUI 可见/AX 绑定，
   确定本轮新发现；确认后未再启动计划中的标准 macOS 构建/测试或验收 App。此前测试宿主崩溃仍作为 Round 2 历史观察保留，
   不直接套用到本轮候选，也不宣称已查明其根因。
4. 收口时核对全部 32 个产品 blob 与 HEAD；记录结构、专题文档、空白和 diff 检查在交接前执行。
   必要结果保存在本记录和 CEv1，不引用临时日志或可清理的结果包路径。

CEv1 namespace：`github.com/kitdine/agent-deck`。
WorkUnit：`auxiliary-session-classification:desktop-menubar`。
ContentState：`asc-desktop-menubar-rereview-r3-20261106-f0dc2376-state`。
完成门禁：FAILED

入口查询确认九项 required criteria，旧状态证据对新 target 均不适用；本轮单独登记 Go L3、复评结论和原生验收边界。
最终查询确认 Go L3 有当前 target 的有效通过证据，独立评审有当前 target 的失败证据；原生验收未验证。
三条新证据均 target_matches=true、applicable=true、malformed=false；无失效证据或未决候选影响。
3 个 evidence 节点和 6 条关系均读回核对数量，关系写入前全部预检为 ok。
Review 保持未勾选，任务回交修复，没有 Task 提交检查点。

下一步指令：`修复：auxiliary-session-classification / desktop-menubar / ASC-DM-R3-F1`

WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Repair 3 — 2026-11-06

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-DM-R3-F1；未处理未登记问题，已关闭的三项发现未改动。
- 确认：直接读取当前 `DesktopCopy.sessionScopeNote` 与 catalog 中英值，与 Round 3 记录一致，只说明范围不影响用量，没有数据域区别。
  scopeHeader 的可见说明（`note(...)`）与 `accessibilityHint` 均读同一个 key（`MenuBarPanelViews.swift` 第 860–861 行）。

**ASC-DM-R3-F1 → 修复，待复评。**

- 变更：[DesktopCopy.swift](../../../../apps/macos/AgentDeckApp/DesktopCopy.swift) 的 `sessionScopeNote` 和
  [Localizable.xcstrings](../../../../apps/macos/AgentDeckApp/Localizable.xcstrings) 对应条目（英文 key 及 en/zh-Hans 值）在原两句之间加入数据域说明：
  - en：`Affects sessions and work analysis only. Session counts come from the session index; work analysis may include usage not linked to an indexed session. Usage and costs include all sessions.`
  - zh-Hans：`仅影响会话与工作分析。会话数来自会话索引；工作分析可能包含未关联到已索引会话的用量记录。总用量和费用保留全部。`

  “会话索引 / session index”沿用 catalog 已有译名。可见说明与无障碍 hint 共用该 key，一处修改同时生效；
  没有新增 unknown 数值、wire 字段、数据库接口或布局变化。catalog 仍为 436 个 key（一个 key 替换）。
- 回归：
  - [DesktopCopyTests.swift](../../../../apps/macos/AgentDeckAppTests/DesktopCopyTests.swift)
    `testSessionScopeNoteNamesBothDataDomainsInBothLanguages` 从 en 与 zh-Hans 的 lproj 读取该 key 并逐字比对上面两段文本。
  - [MenuBarChromeTests.swift](../../../../apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift)
    `testSessionScopeNoteFitsNativeWidthsInBothLanguages` 在 en/zh-Hans × 280/420pt 下渲染真实 `MenuBarSurfaceView` 的会话页，
    状态为 0 个会话加三族信号（即该说明要解释的情形），断言 `fittingSize.width` 不超过窗宽并附 PNG。
- 原生渲染核对：从结果包导出该测试的四张图逐张查看。
  - 说明在会话区内完整换行：en 280pt 四行、420pt 三行；zh-Hans 280pt 三行、420pt 两行。
  - 下方同时可见“此筛选下没有会话”与三族信号卡片。
- 范围外观察：zh-Hans 280pt 渲染中 Popover 顶部区域（客户端标签、金额、用量总览行、时段与面板标签）整体向左被截。
  同一次运行里既有的 `testRefreshFailureAndFirstUseRenderAtNativeWidthsInBothLanguagesAndThemes` 的 zh-Hans 280pt 图（额度面板）
  呈现相同截断，说明与会话页及本次文案无关。从图上看源于 zh-Hans 金额格式（`US$`）使客户端标签行宽于 280pt；
  本任务新增的用量总览行可纵向换行，不是成因。未进一步诊断，也未修改；需另行分诊决定归属。
- 原型差异：本专题原型 `prototype/src/AuxiliarySessions.jsx` 的范围说明仍是旧的两句，不在本发现位置内，未修改。
- 验证：`scripts/test-macos-app.sh`（隔离 HOME）TEST SUCCEEDED（188 项，1 skipped，0 failures），两项新测试均通过。
  纯文案与 Swift 测试变化，Go 侧无改动，按 Round 3 建议复用其 Go L3 证据，未重跑。
  `make check-whitespace`、`git diff --check`、`check-review-records.py`（本记录）、`check-docs.py` 与 `check-topic-docs.sh` 均通过。
  同一修复指令再次下达时复核：33 个 live blob 与下方摘要逐一一致，HEAD 未变；上述检查重跑通过，
  唯 `git diff --check` 本次被 worktree 守卫拦截未重跑（内容逐字节未变，沿用前次通过结果）。
- 候选：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1` + 33 个路径（新增本次首次修改的 `DesktopCopyTests.swift`），摘要
  `98f58af127182621bc2cedddab1bbad234ff31607efe69a355058fc2dae39e3d`（算法同 Round 1，路径按字节序；全部 33 个 live blob 已逐一核对）。
  相对 Round 3 变化或新增的 blob：

  | 路径 | blob |
  | --- | --- |
  | apps/macos/AgentDeckApp/DesktopCopy.swift | `5f6eb7e310eda23eed8a5e383a59f003dc616b8d` |
  | apps/macos/AgentDeckApp/Localizable.xcstrings | `e2afa40d9cbe8d01b8cff5f27124b33fc105fe15` |
  | apps/macos/AgentDeckAppTests/DesktopCopyTests.swift（新增入候选） | `bff88c77853bbe7083737bc668055c7db6adafb8` |
  | apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift | `d6bc6db88c165361ac3fb01ce3a88fec20876851` |

  其余 29 个路径 blob 与 Round 3 相同。
- CEv1：Repair 不宣称门禁完成，未写入新证据；Round 3 证据绑定旧 target，新候选证据由复评按上述摘要绑定。
  `native-acceptance`（原生键盘焦点往返与 VoiceOver）仍未验证。

修复后状态：ASC-DM-R3-F1 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 4 — 2026-11-06

### 📋 desktop-menubar 任务复评

📊 总体评分：9/10

✅ 复评结论：PASS

Reviewer：Codex。
Method：独立核对精确候选、批准合同、中英文 catalog、可见/AX 绑定、实际 XCTest 执行结果和五张原生合成图；
按变更影响复用已有 Go L3 证据，逐项对账任务判据。Scope：四项历史发现及 desktop-menubar 任务实现；
公共顶部截断作为另行分诊的共享表面问题保留载体，原生验收门禁单独判定。
生产代码、测试和配置保持只读；未执行 Git 交付或范围外修复。

### 🔴 严重问题 — 本次变更必须修复

无。本任务四项已登记发现均已关闭。PASS 不代表 native-acceptance 或整个 Task 已完成。

### 🟡 范围外问题与后续处置

#### ASC-DM-R4-F1 · P2 · 范围外 → open；carrier：`ad-bug-menubar-zh-narrow-header-clipping`

位置：公共顶部 `MenuBarSurfaceView.swift` 的 clientTabs/hero/filter 区域，及 `MenuBarViewModel.swift` 的 clientTabs 数据构造。
实际现象：zh-Hans、280pt 原生合成渲染中，客户端选择、日期、金额、时段/面板标签的左侧被裁切；
同次运行的 Sessions 页面和原有 Quota/refresh-retained 页面均有。英文 280pt、中文 420pt 的对照中未见同一截断。
新范围说明自身在四张图中均完整换行，且可见文本和 AX hint 使用同一 key。

- 归属证据：两个 clientTabs 段均逐字节等于 HEAD；其 SHA-256 分别为
  `2f9cc24b2776b8bec731956320fbc503533fa9f8d7e821519290891ff716cbb4` 和
  `97b0b80299d6628f09124c13b1fec42dedd8afa9ab2c6e57aa833a3225637e2f`。
  相同症状在不含新会话范围说明的额度面板也存在。本轮不把该共享表面观察归因为 ASC-DM-R3-F1 修复；
  具体根因和首次引入点尚未完成分诊，`US$` 较长只是假设，不是根因结论。
- 行为风险：窄窗口下共享筛选与金额标签不可完整读出。hosting.fittingSize.width 不超过 280 的断言已通过，
  仍不能证明其后代内容没有裁切；因此任务所要求的原生宽度验收不能据此通过。
- 证据：目视检查本次 note-render 的 zh-Hans 280pt 与同次运行的 `Refresh retained — zh-Hans — 280` 原生附件；
  再与 en 280pt、en/zh-Hans 420pt 对照。都是合成数据、真实 NSHostingView 渲染，不是 React 标本或实际用户会话。

💡 后续处置：共享桌面顶部的分诊已落到上述 Bug，状态 open、未认领，Lane 尚未由用户裁定；本轮未修复。
在 Task 准出前完成修复后的原生验收，或取得明确的基线处置决定。建立载体不表示问题解决，也不豁免 native-acceptance。

### 🟢 已关闭发现与验证

- **ASC-DM-R3-F1 → CLOSED**：`DesktopCopy.sessionScopeNote` 及 en/zh-Hans catalog 均已加入“会话数来自会话索引；
  工作分析可能包含未关联到已索引会话的用量记录”，并保留总用量/费用全部计入的说明。
  `scopeHeader` 的可见 note 与 accessibilityHint 共用该 key，没有新增协议字段或伪造 unknown 数。
- **ASC-DM-R1-F1、ASC-DM-R1-F2、ASC-DM-R2-F1 → 保持 CLOSED**：本轮四个变更/新增路径不触及请求状态机、
  信号呈现逻辑或 Go producer；旧修复保留。实际 XCTest 中相应乱序、取消替换、失败重试、零会话信号等回归均 Passed。
- 四张原生说明图已目视检查：en 280/420pt 分别约四行/三行，zh-Hans 280/420pt 分别约三行/两行，说明完整可读；
  零会话空态与独立信号卡片同时存在。AX 绑定同源已静态核实，但没有把它当成真实 VoiceOver 播报验收。

### 📝 总结

| 发现 | 当前处置 |
| --- | --- |
| ASC-DM-R1-F1 | CLOSED，保持 |
| ASC-DM-R1-F2 | CLOSED，保持 |
| ASC-DM-R2-F1 | CLOSED，保持 |
| ASC-DM-R3-F1 | CLOSED |
| ASC-DM-R4-F1 | 范围外 open → `ad-bug-menubar-zh-narrow-header-clipping`；仍影响原生准出 |

Reviewed state：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1`；33 路径候选摘要
`98f58af127182621bc2cedddab1bbad234ff31607efe69a355058fc2dae39e3d`，算法沿用 Round 1。
独立重算与 Repair 3 一致：变更只有 DesktopCopy.swift、Localizable.xcstrings、MenuBarChromeTests.swift，
新增入候选的路径是 DesktopCopyTests.swift；其余 29 个路径与 Round 3 逐 blob 相同。
全部 Go 源码、测试、依赖、配置未变，复用时确认同一已安装 Go 1.27.3；不重新运行全量、vet 或 race。

Evidence：

- 当前 en/zh-Hans 实际 key/value、scopeHeader 的两处绑定及新增测试源码已核对。
  `testSessionScopeNoteNamesBothDataDomainsInBothLanguages` 从 lproj 读取，两种语言均通过；
  `testSessionScopeNoteFitsNativeWidthsInBothLanguages` 生成四张真实 MenuBarSurfaceView 渲染图。
- 直接读取 2026-11-06 09:24:08 EST 开始的 XCTest：全 scheme 365 项，364 passed、1 skipped、0 failed。
  逐项读取 test tree，确认 wire 的 canonical/legacy/invalid/explicit-envelope、默认确认与每次打开、
  请求乱序/取消/往返/失败、原因码与覆盖、范围切换的全量用量/任务子代理，以及零会话信号用例均 Passed。
  交接的 188 项不是此工具返回的全 scheme 总数；此处保留直接观察到的计数。
- 本轮复用的 Go 原始证据为 `asc-desktop-menubar-r3-20261106-f0dc2376-go-l3`：full suite、vet、
  desktop/sessionorigin race 全部通过；包含只读 27 组合、旧字段不改义、默认/不可用、usage-only、同代发布与截断行数回归。
  对 R3→R4 四路径变化做 scope-aware preserves 评估，并创建绑定本 target 的 roll-up；没有重标旧 observation。
- 原生图检查来自上述本次运行的新说明测试与既有 refresh-retained 测试，观测事实完整保存在本记录和 Bug，
  不把临时导出图片或可清理结果包作为持久引用。真实键盘焦点往返、VoiceOver、实际 Popover 生命周期仍未完成验收。
- 最终检查产品 HEAD/blob 未变；执行文档结构、记录结构、空白与 diff 检查。只更新本专题记录，不改 main 的全局状态。

任务证据对账：

| Required criterion | 支持证据及边界 |
| --- | --- |
| wire-scope-view | Go 同代发布/旧字段/trim 回归；Swift canonical、legacy、invalid 和 envelope 用例及源码核对 |
| explicit-selection-interface | 现有 Go 显式参数和只读接口、27 组合无状态写入回归；Swift runner 明确传入 selection 并拒绝错选答复 |
| selection-consistency | 当前请求守卫；晚到、缓存往返、取消替换、失败/重试、stale/无旧数据等 Passed 用例；Go 同代读取检查 |
| scope-default-lifecycle | confirmed default、unconfirmed 不回退、每次打开恢复、临时选择不持久化用例及调用边界 |
| origin-reason-copy | 七个原因码、未知码不改分类、override/coverage 用例；安全 DTO 与详情映射 |
| full-usage-and-subagents | 作用域切换前后 hero/usage/quota/client 标签不变、任务子代理留 Work；Go 原始事件/旧 totals 保持 |
| go-l3-verification | 原始 R3 通过证据，经明确 preserves 与本 target roll-up 复用 |
| independent-review | 四项历史发现均关闭；本轮 PASS；共享顶部观察已有具体载体 |
| native-acceptance | 宽度部分不通过：zh-Hans 280pt 公共顶部裁切；键盘/VoiceOver 等实际验收尚缺 |

CEv1 namespace：`github.com/kitdine/agent-deck`。
WorkUnit：`auxiliary-session-classification:desktop-menubar`。
ContentState：`asc-desktop-menubar-rereview-r4-20261106-98f58af1-state`。
完成门禁：FAILED

最终查询：8/9 required criteria 有当前 target 的有效通过证据；唯一未通过的是 `native-acceptance`，
其当前失败证据来自已验证的 280pt 原生裁切。全部当前 evidence 的 target_matches/applicable=true，
malformed/invalidated/unresolved=false；缺失集合仅含该 native criterion。
本轮新增 11 个节点（9 条 evidence、1 个 change、1 个 preserves assessment），26 条关系全部预检为 ok 并读回核对数量；
Go 通过证据以明确评估和 target-bound roll-up 复用，原始 R3 observation 未改写。

层级：Beads 与 tasks.md 明确本 subject 属任务 5，后续任务仍未完成。当前 CEv1 WorkUnit 没有已登记的 contains 父关系；
本轮不创建或镜像层级、不宣称 Topic 完成，也不查询并未跨越的 Topic gate。
Review 按 PASS 勾选；任务仍在 in_review，证据和交付边界开放，不进入 awaiting_commit。

Task checkpoint：`ad-auxiliary-session-classification-desktop-menubar`；content_state=`98f58af127182621bc2cedddab1bbad234ff31607efe69a355058fc2dae39e3d`；gate=`FAILED`（8/9，native-acceptance 未通过）。
提交建议：门禁 VERIFIED 后，提交本任务 33 个产品/测试/fixture 路径及 tasks.md、reviews/desktop-menubar.md；前提是处置公共顶部截断并完成 native-acceptance，取得独立提交授权。
推送建议：门禁 VERIFIED 后，目标 `origin/feature/auxiliary-session-classification`（origin 已核实为 `https://github.com/kitdine/agent-deck.git`）；前提为同一原生验收问题已处置、授权提交完成并核验，再取得推送授权。

下一步指令：完成 auxiliary-session-classification / desktop-menubar 的 native-acceptance：处置 `ad-bug-menubar-zh-narrow-header-clipping`，并补齐原生键盘、VoiceOver 与窗口生命周期验收后重查任务门禁。

WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## 原生验收工作 — 2026-11-06

用户明确要求处置 `ad-bug-menubar-zh-narrow-header-clipping` 并完成 native-acceptance；以现有专题任务为修复和验收载体。
执行者 Codex；入口为 Round 4 的 33 路径候选 `98f58af127182621bc2cedddab1bbad234ff31607efe69a355058fc2dae39e3d`。
本段记录修复/验收事实，不产生新的独立评审结论；Round 4 及其原始证据保持历史身份。

根因已确认：直接使用生产 `clientTabs` Picker 内容进行离屏 NSHostingView 尺寸测量，精确分离 label/value。
280pt 中文标签时控件 frame 为 x=-27、width=334，原生 intrinsic width=354；去掉用量总览说明仍为相同水平溢出。
420pt 时 x=12、width=354，正常；只把 US$ 改成窄货币符号仍有 x=-1、width=282，不能完整恢复边界；大金额会扩大到 width=416。
这确认是原生 segmented Picker 对长标签的宽度要求撑开共享顶部，排除了新增范围/用量说明造成水平裁切的解释。

修复边界：MenuBarSurfaceView 的客户端选择器在等宽列内按空间将名称/金额分行，保留全部客户端、完整金额、原 selectClient 路径、
选中/禁用状态、方向键和可访问名称/值。MenuBarChromeTests 增加实际目标矩形边界检查，避免只断言外层 fittingSize 的漏检。
以下收尾记录给出本次限定验收结果；不把这段实现自行标记为独立评审通过。

Round 2 测试宿主崩溃原因补证：传入的隔离 HOME 使用了 `asc-desktop-rereview-` 目录前缀，
不满足生产 `debugTestHome` 的隔离前缀白名单。原样纯函数重放返回 unsafeHome，使用已允许的 agentdeck-menubar-acceptance 前缀则返回 isolated。
结合该轮命令和启动前 preconditionFailure 路径，确定为本轮审查者当时的测试启动目录选择错误，不是那两项请求时序修复的失败。
本次验收使用既有允许前缀，不放宽保护规则。

### 验收收尾 — 2026-11-07

**用户范围决定**：用户随后明确指示“不验证键盘与 VoiceOver 操作”。本次仅排除这两类操作验收，
不把未执行事项记为已验证，不删除产品已有交互要求，也不扩大为其他任务的豁免。
实际没有执行键盘或 VoiceOver 操作验收，没有修改这两类系统设置。
电脑控制服务在取得隔离 App 时返回 `-10005: timeoutReached`；该工具超时不作为产品失败证据。
已按启动记录与精确可执行路径核对并停止本次隔离进程，后续以原生 XCTest 完成保留的窗口生命周期检查。

**修复与回归**：

- `MenuBarClientFilters` 在三个等宽客户端选项内保留完整 label/value，空间不足时自然分行；
  未更改金额计算、客户端顺序或 `selectClient` 路径。方向键、选中/禁用及可访问名称/值实现保留，操作效果不在本次验证范围内。
- 原生尺寸检查覆盖 280/420pt（扣除两侧 padding 后可用宽度 256/396）、en/zh-Hans、普通/大金额、
  默认/放大字号共 16 组，三个目标矩形均在可用宽度内，且保留最小目标高度。
  独立离屏对照的 en、zh、大金额 × 两个宽度共六组也通过；修复后 zh-Hans 280pt 原生图已目视核对，公共顶部不再左裁切。
- 窗口准备回归原先枚举全局 NSApp.windows，既把延迟出现的状态栏窗口算成新 Popover，也把其他测试状态栏窗口的退出算作异常。
  排除单个状态栏窗口的尝试虽在三项定向检查中通过，后续组合仍观察到旧窗口消失引发的误报。
  最终改为核对控制器实际持有的 preparation window，保留创建次数、不可见、非 key、宿主挂载、原 responder 及 key window 不变检查，
  并确认 Popover 未显示。生产控制器仅将该已有字段改为模块内只读，不改变其创建/显示行为。
- 扩展真实 NSPopover 测试：首次显示时移交同一 hosting view、退休准备窗口；本次临时 All 选择在确认新默认 Auxiliary 后仍保持；
  两次关闭重开均恢复 Auxiliary，宿主仍可见，准备次数保持 1。这些是原生窗口和合成数据测试，不是实际键盘或 VoiceOver 操作。

**实际验证结果**：

- 首次完整 macOS 套件：365 项非跳过检查中，窗口准备测试出现三处断言失败；其余通过。失败归属及修正过程如上，未以直接重跑掩盖。
- 窗口生命周期测试新增时有一次参数顺序编译错误；修正后再验证。一次四项定向运行仅全局窗口集合误报，
  关闭重开/默认恢复、预热及布局检查均通过；随后完成按拥有者核对窗口的测试修正。
- 最终 `scripts/test-macos-app.sh`（标准隔离 HOME，构建 helper 时使用匹配的 Go 1.27.3）exit 0、TEST SUCCEEDED。
  2026-11-06 23:50:53 EST 开始的全 scheme 运行：366 项，365 passed、1 skipped、0 failed；AppTests 189 项、1 skipped、0 failures。
  直接读取结果树确认 `testClientFiltersKeepEveryTargetInsideTheAvailableWidth`、
  `testFirstOpenRetiresPreparationWindowWithoutRemovingPopoverContent`、
  `testInitialPopoverPreparationDoesNotShowAWindowOrMoveFocus`、`testWindowWarmUpRunsOnceWithoutShowingAWindowOrMovingFocus` 全部 Passed。
- Go 代码、测试和依赖未改变，沿用 R4 已绑定的 Go L3 证据，不重复全量/vet/race。文档和改动边界检查在最终交接前完成。

**最终内容身份**：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1`，同一 33 路径、同一摘要算法，产品指纹
`c5f19968600e590cf6999cb35b11c6db0125ddf46cfff6f2c3f636b58d535552`。
相对 R4 仅三个 blob 改变：

| 路径 | 当前 blob |
| --- | --- |
| apps/macos/AgentDeckApp/MenuBarSurfaceView.swift | `39711e6845094f4a0ce5e27a4eadb139b078b37a` |
| apps/macos/AgentDeckApp/MenuBarItemController.swift | `1ed924ba62b6c2a1ee9ec6356497f7e0ff8a7ea2` |
| apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift | `fd30cd9924cc14f81b2e16976864712d24383ca0` |

本次验收范围决定另有摘要 `e93b69461d26c8d79320e130462635f78bf691b8b4174cb8dff02508ffbb32c5`：
SHA-256 输入为 `work_unit=auxiliary-session-classification:desktop-menubar;excluded=keyboard_operations,voiceover_operations;user_quote=不验证键盘与 VoiceOver 操作`。
它与产品指纹分开记录，避免把用户排除项默认为已测试。

本次限定的 native-acceptance 检查已满足；新控件代码由本轮执行者实现，不能沿用旧 R4 的独立评审结果。
当前产品候选仍需未参与实现的评审者复评；Task 与关联 Bug 保持交付边界开放，没有提交或推送。

证据收口：新目标 `asc-desktop-menubar-native-20261107-c5f19968-e93b6946-state` 同时记录产品指纹及本次验收范围摘要。
用户排除决定另存为 `asc-desktop-menubar-scope-e93b6946-state`，native 通过证据明确依赖该范围决定，
未把键盘或 VoiceOver 操作伪装成已执行。七项未受影响的证据通过逐项 preserves 与 target-bound roll-up 复用；
R4 独立评审证据因新生产控件变更而不再可复用于此目标，原始观察保留。
最终查询返回 `NOT_VERIFIED`（8/9），唯一缺项为 `independent-review`；native 判据已有当前限定范围的通过证据，
没有未决 candidate impact。2 个 ContentState、18 个变更/评估/证据节点及 42 条关系均写入并核对数量，关系全部先预检为 ok。

后续由未参与本次实现的独立评审会话执行 `复评：auxiliary-session-classification / desktop-menubar`。

隔离环境已清理：确认验收进程退出，并按 bundle identifier 与运行时 SHA-256 核验后，仅注销和删除本次私有验收 App；
合成 HOME、离屏探针和本次手工 XCTest 临时目录已删除。原仓库构建产物与持久验收记录保留，键盘/VoiceOver 系统设置未修改。

## Round 5 — 2026-11-07

### 📋 desktop-menubar 独立复评

📊 总体评分：9/10

✅ 复评结论：PASS

Reviewer：Codex；本会话未参与产品实现，独立接收原生验收交接。
Method：当前源码与批准合同对照、33 路径内容身份重算、既有原生 XCTest 结果树及四张渲染附件的直接检查。
CodeGraph 的本工作区查询没有覆盖新增 Swift 控件，相关判断采用限定路径源码检查。
Scope：五项历史发现，原生验收交接中的客户端控件、准备窗口访问及回归变更；复用未变更范围的已有验证。
生产代码、测试及配置保持只读；只更新本专题评审、状态和协调记录。

### 🔴 严重问题 — 必须修复

无。未发现本轮修复引入的阻塞缺陷。

### 🟡 改进建议 — 建议处理

无未关闭的本任务发现。首次收尾的 CEv1 连接故障及随后完成的证据收口记于下文，产品复评结论保持不变。

### 🟢 已关闭发现与验证

- **ASC-DM-R4-F1 → CLOSED**：用户已将共享顶部裁切纳入本任务处理。
  `MenuBarSurfaceView.swift:437` 的 `MenuBarClientFilters` 用三个等宽按钮承载客户端名称和金额，
  空间不足时由横向排布切换为纵向排布；`clientTabs` 仍将 `model.clientTabs`、`model.selectedClient` 和
  `model.selectClient` 原样传入，并保留父级禁用条件。没有重新计算金额或写入会话默认。
  直接检查 en/zh-Hans × 280/420pt 四张真实 NSHostingView 附件：三个客户端名称及金额均完整，
  中文 280pt 原有左侧整体裁切已消失。关联载体 `ad-bug-menubar-zh-narrow-header-clipping` 保留交付边界，
  本次关闭的是发现，不宣称 Bug 已交付。
- **ASC-DM-R1-F1 → 保持 CLOSED**：`SessionScopeController.swift:90` 的缓存返回仍推进 generation；
  `load` 的成功和错误分支均经 `isCurrent` 核对 generation 与 `.loading(selection)`。
  当前结果包中的 `testReturningToTheShownSelectionDiscardsTheReadThatLeftIt` 为 Passed。
- **ASC-DM-R1-F2 → 保持 CLOSED**：`synchronize` 不对仍在途的相同 selection 提前返回；
  新同步通过 `load` 推进 generation，旧取消不能清除替代请求。
  当前结果包中的 `testACancelledReadNeverSwallowsTheSynchronizationThatReplacesIt` 为 Passed。
- **ASC-DM-R2-F1 → 保持 CLOSED**：`MenuBarViewModel.swift:1038` 的 `workSignalPanel` 按各信号族 availability
  与 selection 读取，不以文本会话数为 guard；
  `testWorkSignalsStayVisibleWhenTheSelectionListsNoSession` 为 Passed。
  四张原生图中零会话列表空态与三族工作信号同时存在。
- **ASC-DM-R3-F1 → 保持 CLOSED**：当前 `DesktopCopy.sessionScopeNote`、en/zh-Hans catalog 及
  `MenuBarPanelViews.swift:826` 的可见说明/AX hint 绑定一致；两种语言均明确会话索引与可能未关联的用量记录，
  并保留总消耗说明。文案测试和四种宽度/语言原生渲染测试为 Passed，目视检查说明完整换行。
- 准备窗口改动只开放 `presentationPreparationWindow` 的模块内读取，setter 仍为 private。
  隐藏准备测试直接核对本控制器窗口的不可见、非 key、宿主挂载、原 responder/key window 和准备次数。
  首次显示回归检查同一 hosting view 转移、准备窗口退休、当前临时 All 在新默认发布后保留，
  两次关闭重开恢复已确认 Auxiliary。相应原生用例及预热用例均为 Passed。

### 📝 总结

| 发现 | 当前处置 |
| --- | --- |
| ASC-DM-R1-F1 | CLOSED，保持 |
| ASC-DM-R1-F2 | CLOSED，保持 |
| ASC-DM-R2-F1 | CLOSED，保持 |
| ASC-DM-R3-F1 | CLOSED，保持 |
| ASC-DM-R4-F1 | CLOSED，修复及原生图已独立复核；关联 Bug 尚待授权交付 |

Reviewed state：HEAD `de9d6337a29a195c93cbabc727d99a9486bb55d1`；
33 路径指纹 `c5f19968600e590cf6999cb35b11c6db0125ddf46cfff6f2c3f636b58d535552`。
采用 Round 1 的同一算法，独立核对 CEv1 当前 ContentState 的全部路径/blob，差异数为 0。
三个交接变更文件的 blob 与上方“验收收尾”表一致。review/status 文件不属于该产品指纹。
已验证工作区 `agent-deck.auxiliary-session-classification`、分支 `feature/auxiliary-session-classification`、
Git common directory 与执行根包含关系；没有切换或创建工作区。

Evidence：

1. 直接读取 2026-11-06 23:50:53 EST 开始的原生 XCTest 结果：366 项，365 passed、1 skipped、0 failed；
   result 为 Passed，testFailures 为空。环境为 macOS 26.7.3 / 25G416、x86_64、Xcode 26.4 / 17E192、
   Swift 6.3 / swiftlang-6.3.0.123.5；本轮核对的 Xcode/Swift 与其一致。
2. 结果树直接确认客户端边界、隐藏准备、预热、首次宿主转移/关闭重开、请求往返、取消替换、零会话信号、
   范围切换不改总用量，以及中英文说明用例通过。本轮读取原结果，没有重跑未变的全套测试。
   `testClientFiltersKeepEveryTargetInsideTheAvailableWidth` 的源码包含宽度、语言、普通/大金额及
   `.large`/`.accessibility3` 环境共 16 组，检查每个客户端目标的实际边界与最小高度。
3. 通过 `xcresulttool export attachments --test-id 'MenuBarChromeTests/testSessionScopeNoteFitsNativeWidthsInBothLanguages()'`
   取得并目视检查四张原生合成图。必要观察已直接记入本轮；导出副本仅供本轮检查，不作为持久引用。
4. Go 产品、测试和依赖没有本轮改动；不重跑已登记的 Go L3 检查。此复用判断不替代最终 CEv1 gate。
   已读取的 WorkUnit 具有九项 required criteria；后续任务 6–9 仍未完成，不跨越 Topic 完成边界。
5. 验收范围摘要为 `e93b69461d26c8d79320e130462635f78bf691b8b4174cb8dff02508ffbb32c5`。
   用户明确决定“不验证键盘与 VoiceOver 操作”，其来源在原生验收段、任务 5 验收范围及
   Beads 评论 `9a2a4335-9d12-5982-8049-558db6548273` 中一致。本轮保留该决定，未执行或宣称这些操作通过。

CEv1 namespace：`github.com/kitdine/agent-deck`。
WorkUnit：`auxiliary-session-classification:desktop-menubar`。
ContentState：`asc-desktop-menubar-native-20261107-c5f19968-e93b6946-state`。
完成门禁：VERIFIED

**首次收尾记录（连接故障期间的历史事实；下文证据收口已完成其待办）**：

当前 WorkUnit、九项 criteria 和上述 ContentState 已通过 MCP 读取，并完成产品指纹核对；
随后用共享 provider 的原始 `gate-status.cypher` 查询时发生 `Transport send error / HTTP request failed`，
层级读取也遇到同一传输故障。没有得到本轮正式门禁结果，没有把错误或空返回解释为缺失图数据。
既有验收记录的 NOT_VERIFIED 8/9 仅保留为上一轮事实，本轮不将其宣称为当前查询结果。
没有绕过 MCP 直写后端，也没有创建空本地 mirror。

收尾待办：恢复 Neo4j MCP 连接后，先读取同一 target 的门禁及当前证据，再以新稳定 ID
`asc-desktop-menubar-r5-20261107-c5f19968-independent-review` 登记本轮独立复评 pass，
criterion 为 `auxiliary-session-classification:desktop-menubar:independent-review`，
observed_state_id 为上述 ContentState，producer 为本轮独立 Codex reviewer，check 以本 Round 5 的
源码、指纹和原生结果直接复核事实为依据。遵循节点写入、关系完整预检、satisfies/observed_at 关系写入及数量读回，
最后重新查询同一门禁。该 evidence 尚未写入；本轮记录不是另一个 evidence provider。
若本轮未改动的产品及验收范围仍相同，保留本次 PASS，不重复复评或无关测试。

**证据收口 — 2026-11-07**：

用户明确要求恢复 MCP 连接并完成本轮证据登记。当前客户端已有工具连接仍报传输错误，而主机可访问同一已配置端点；
通过该端点建立新的标准 Streamable HTTP MCP 会话，`initialize`、`notifications/initialized`、`tools/list` 成功，
服务器自报 `mcp-neo4j-cypher` 2.13.3。后续读写均经其已公布的 `read_neo4j_cypher` / `write_neo4j_cypher` 工具，
沿用共享 provider 原始参数化模板，没有直连数据库、改写服务器配置或替换证据 provider。
这证实本次新 MCP 会话可用，不宣称当前客户端的原连接已修复。

恢复入口再次验证工作区绑定、HEAD 及全部 33 个产品 blob，指纹与本轮 reviewed state 相同。
正式入口 gate 返回 NOT_VERIFIED，八项 required criteria 有可复用通过证据，唯一缺项为 independent-review；
没有未决 candidate impact。新稳定 ID 在写入前不存在，随后登记
`asc-desktop-menubar-r5-20261107-c5f19968-independent-review`，outcome=pass，
并绑定本轮 ContentState 与 independent-review criterion。
四条关系为 satisfies、observed_at、depends_on（本 target 的 native-acceptance 证据）及
supersedes（本 target 原有的 not_verified 独立复评占位观察）；旧观察未覆盖。
关系预检全部为 ok；节点属性逐字读回一致，节点数 1/1，关系端点与 kind 读回 4/4。

最终使用原始 `gate-status.cypher` 及同一 namespace/work_unit_id/target_content_state 查询，返回 VERIFIED 9/9；
reason=null，missing_criteria=[]，unresolved_candidate_impacts=[]。
九项有效通过证据均 target_matches/applicable=true，malformed/invalidated/unresolved=false。
诊断集合中的两个 invalidated evidence 分别是已被本轮 supersede 的待复评占位观察，以及历史 R4 独立评审；
两者均不再用于本 target 准出，不影响当前九项有效证据。
其余八项证据直接复用，未重跑测试或重复本轮产品复评。键盘与 VoiceOver 操作排除决定保持原范围。

WorkUnit 当前无 contains 父关系；tasks.md 与 Beads 中任务 6–9 仍未完成，未跨越 Topic gate。
本任务及关联裁切 Bug 的证据/评审边界已满足，协调状态进入 awaiting_commit；未执行提交或推送。
后续 settings-control 仍依赖本任务完成授权交付。

Task checkpoint：`ad-auxiliary-session-classification-desktop-menubar`；content_state=`c5f19968600e590cf6999cb35b11c6db0125ddf46cfff6f2c3f636b58d535552`；gate=`VERIFIED`（9/9）。
提交建议：取得明确提交授权后，将本任务 33 个产品/测试/fixture 路径及 tasks.md、reviews/desktop-menubar.md 作为同一任务范围提交，并核验贡献者、实际提交对象和 SSH 签名。
推送建议：目标 `origin/feature/auxiliary-session-classification`；完成授权提交及范围/签名核验后，再取得单独推送授权。

下一步指令：开发：auxiliary-session-classification / settings-control

WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## 本地交付边界 — 2026-11-07

用户明确授权 Round 5 检查点的本地提交；未授权推送、PR、合并或发布。
交付包含同一 33 个产品/测试/fixture 路径及 tasks.md、本评审记录，共 35 个路径。
产品 blob 与候选 `c5f19968600e590cf6999cb35b11c6db0125ddf46cfff6f2c3f636b58d535552` 一致，
候选正式门禁九项全部通过；本段和 tasks.md 的追加只记录授权交付边界，不改变产品或验收范围。
复用本轮已有产品验证，执行必要的暂存范围、消息、贡献者、提交对象及 SSH 签名检查。
实际提交 ID、tree、签名结果及绑定该提交的 CEv1 门禁结果保存于 Beads 与 CEv1，避免在提交文件内自引用提交 ID。

Commit checkpoint contributors

- staged_tasks：`ad-auxiliary-session-classification-desktop-menubar`；关联修复载体 `ad-bug-menubar-zh-narrow-header-clipping`。
- included：claude-code — 实现及 Repairs 1–3；交接 `d53db3b6-57bd-51d5-a6d3-7e66d75cece1`、
  `9c341dfe-ec1d-5498-a862-2a81183c2165`、`1f0d393c-f851-585f-b064-b7b05f25930b`、`b62c356c-f466-5574-8770-b73d30d73e6f`。
- included：codex — 共享顶部修复、原生回归及提交范围内的评审/状态记录；交接
  `77996f31-ae9b-5774-af2e-d0bce43d0fd4`、`e839b60b-69ad-5513-b99e-052f19b23724`、`3323bb48-a39f-55a6-bbbe-ed6636963fb0`。
- excluded：无其他已证实的产品或文档贡献者；纯证据查询、协调和评审判断不单独产生额外署名。
- trailers：`Co-Authored-By: Claude <noreply@anthropic.com>`；`Co-Authored-By: Codex <noreply@openai.com>`。
- unresolved：无。
