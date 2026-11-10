---
status: active
topic: installed-state-ownership
subject: app-state-location
---

# app-state-location Review

## Round 1 — 2026-11-10

## 📋 Task 6 独立实现评审

📊 总体评分：7/10

✅ Verdict: FAIL

- Reviewer: Claude Code，独立评审会话；未参与本候选实现，未修改产品代码、测试或配置。
- Method: 对照已批准的 [Task 6](../tasks.md#6-app-state-location)、[架构「App 与小组件」](../architecture.md#app-与小组件) 与 [CLI 契约](../ux/cli.md)，逐段检查工作区差异；核对 Swift 会话与 Go `AcquireOwnershipLock` 的锁路径、打开标志与 flock 语义；沿 desktop 线路从 `doctor` Check 到 `healthSnapshot` 再到 App 文案查表追踪 Health 文案；检查原生夹具与 harness 的隔离边界，以及刷新协调器中各取消路径能否触达所有权会话。未重跑原生套件：唯一发现由源码与仓库 fixture 直接证实，复用开发 CEv1 的原生回归证据。
- Scope: `EmbeddedHelperRunner.swift`（状态位置、`NativeOwnershipSession`）、`AgentDeckApp.swift` 生命周期装配、`TelemetrySettingsController.swift` receiver 监视、`DesktopCopy.swift`/`Localizable.xcstrings` Health 文案、对应 Swift 测试、`AgentDeckVerification/main.swift`、`scripts/test-macos-app.sh`、`internal/installation/native_fixture_test.go`。Task 8 的真实安装器与完整 App 联合 A5 验收不在本轮范围。
- Reviewed state: HEAD `a4a5d684bc1e2d067f8e92113aa1c7f064bd4ba5`；开发候选 `installed-state-ownership:app-state-location:dev-20261110-3Xphhahl`；SHA-256 `bea3480cd160c4c4cfa2c285aad5f9b77836b077261febfae81151cb35af19a3`。本轮按 ContentState 记录的配方（`head=<HEAD>` 换行，随后按路径排序的 26 个 `<path>=<git-blob>` 行）重新计算，结果一致。
- Completion gate: NOT_VERIFIED（independent-review 本轮为 fail）

### 🔴 严重问题 — 必须修复

**ISO-APP-R1-F1 — P2。** [apps/macos/AgentDeckApp/DesktopCopy.swift:221](../../../../apps/macos/AgentDeckApp/DesktopCopy.swift) 把 `not_installed`、`managed_command_owner_mismatch` 加进 `healthReasonKeys`，但这两个值在契约和实现中都是 `code`，不是 `reason`，新增说明永远不会显示。

- 行为风险：未安装副本（例如 Debug App）的 Installation 行、以及接管后 Hook 归属不匹配的 Hook command owner 行，只显示通用警告文案；任务要求的中英文说明成为死文案。归属不匹配正是架构要求提醒用户重新登记 Hook 的场景，用户看不到原因。
- 证据：
  - App 按 `check.reason` 查表：`apps/macos/AgentDeckApp/MenuBarViewModel.swift:1620`（`healthReasonCopy`）及 `:1644`（`healthAction`）。
  - Go 侧：`internal/doctor/installation.go:13` 生成 `Code: "not_installed", Reason: identity.Reason`；`internal/doctor/managed_command.go:65-66` 生成 `Code: "managed_command_owner_mismatch"`，`Reason` 为受影响客户端列表（如 `claude,codex`）。`internal/desktop/desktop.go` 的 `healthSnapshot` 原样映射 `Code`/`Reason`。
  - 契约：[ux/cli.md](../ux/cli.md) 第 97、119 行把两者列在 code 列，reason 分别为原因码与客户端列表；[tasks.md](../tasks.md) 第 337 行确认 fixture 中 installation 行为 `code: not_installed`、`reason: receipt_missing`。
  - 仓库 fixture `desktop/fixtures/v1/snapshot-complete.json` 第 7852–7855 行即该形状；按现有查表逻辑，`receipt_missing` 不在 `healthReasonKeys`，渲染落到 `healthStatusWarning`。
  - `rg 'not_installed|managed_command_owner' apps/macos` 除 `DesktopCopy.swift` 与 xcstrings 外无命中：没有测试渲染这两类 Health 行，因此 352 项全绿未能发现。

💡 修复范围：让这两条说明按 `code` 生效（例如新增按 code 查表的说明，或在 reason 查表未命中时对这两个检查回退到 code），不改变线路字段、布局和其他检查行为；补一个 App 测试，用 fixture 形状（`installation/warning/not_installed/receipt_missing` 和 `managed_command_owner/warning/managed_command_owner_mismatch/claude,codex`）断言 Health 行显示新增中英文说明。

### 🟡 改进建议 — 建议处理

无。

### 🟢 优点

- 状态路径：`snapshot`、serving 恢复、bootstrap、quota/telemetry 请求各自重新执行 `desktop state-location`，删除了 `HOME/.agentdeck` 推算与全部 `--state-dir` 注入；查询失败或畸形输出不回退默认路径。`NativeStateLocation.decode` 拒绝 partial、错误包、非绝对路径和含 NUL 的路径，不接受线路上的锁路径或账户。
- 所有权会话与 Go 完全对齐：同一 `<account>/Library/Application Support/AgentDeck/ownership.lock`，同样的 `O_CREAT|O_RDWR|O_CLOEXEC|O_NOFOLLOW|O_NONBLOCK`、常规文件检查、`0600`、`LOCK_NB` 轮询与 5 秒上限；锁内复核、资格丢失即释放、`close()` 同步关闭且之后不能重新持锁（包括正在复核的临时描述符）。
- 物理路径比较按最近存在祖先 `realpath` 加缺失分量，拒绝悬空符号链接，覆盖首次启动 `.agentdeck` 尚不存在时的 `/tmp` 别名；开发交接如实保留了由真实 helper 测试暴露并修复的这一缺陷。
- 取消路径核查：`activeRefresh`/`activeRestore` 只在 `stopAppWork`（App 退出）时取消；quota 外层任务的取消不会传入内部非结构化任务。因此"失败即释放"不会在正常刷新中让已安装 App 丢锁。
- 原生夹具隔离严格：根目录强制 `/private/tmp/agentdeck-macos-xctest.` 前缀并以 `EvalSymlinks` 校验，账户与 helper 路径逐一核对；驱动只编进测试二进制，缺参数时 XCTest 失败而非跳过；正常 Debug/Release 构建与打包脚本未改。
- Telemetry 每轮重新定位 `receiver.json` 监视器，查询失败时撤销旧监视器并停止 listener，不会把直接文件读取挂在上一任拥有者的状态上。

### 📝 总结

被审内容为 HEAD `a4a5d684…` 加上述 26 输入候选（digest `bea3480c…`）。状态位置、原生所有权会话、生命周期和夹具隔离都符合 Task 6 与架构；唯一发现 ISO-APP-R1-F1 使任务要求的 Health 说明不可见，是本变更内的缺陷，故 FAIL。残余不确定性：架构写的是锁"确认后持有到 App 退出"，本实现在查询失败时也释放共享锁；因 App 不在未确认时访问真实状态、开发 CEv1 准则已声明该行为，本轮判定为保守且可接受的解释，不记为发现。修复后只需复评 ISO-APP-R1-F1 及其测试，其余证据可复用。

Evidence:

1. 本轮重算候选 digest，与开发 ContentState 一致；`git diff --stat` 共 11 个已跟踪文件改动及新增 `native_fixture_test.go`，与开发交接范围一致。
2. ISO-APP-R1-F1 的证据链见上文，全部来自仓库源码与 fixture，无需运行时复现。
3. 复用开发 CEv1 的原生回归、夹具、会话、状态位置证据（`dev-20261110-3Xphhahl:*`）；本轮未重复 `make test-macos-app`。修复会改变 App 源码与测试，届时原 native-regression 证据需针对修复后状态重新取证或做影响评估。
4. 未执行安装、启动 App、账号操作或 Git 交付。

下一步：修复 ISO-APP-R1-F1 后复评。

## Repair 1 — 2026-11-10

- Scope: 仅 ISO-APP-R1-F1；修复者 Codex，用户授权 `修复：installed-state-ownership / app-state-location / ISO-APP-R1-F1`。未改变 Round 1 的独立评审结论，未自行签发 PASS；该发现的实现修复已完成，最终关闭待独立复评确认。
- 确认：doctor 的 installation 检查输出 `code=not_installed, reason=receipt_missing`；managed_command_owner 输出 `code=managed_command_owner_mismatch, reason=claude,codex`。既有 App 只按 reason 查找这两条新增说明，确实不命中。以源码及仓库 fixture 为直接复现证据，不需先重跑无覆盖的旧套件。
- 修改：将两项从 `DesktopCopy.healthReasonKeys` 移到 `healthCodeCauseKeys`，保留在 `allKeys` 本地化清单中；`MenuBarViewModel.healthCause` 在既有 reason cause 未命中时按 `check.code` 取说明。文本资源、线路字段、布局、其他 reason 说明和 recovery/action 判定均不变。原 Health 行因 cause 非空具有 disclosure，默认展开即可显示说明，无需新布局。
- 回归：新增 `MenuBarViewModelTests.testInstallationHealthDescriptionsUseWireCodesInBothLanguages`，通过 WireFixture 解码两种真实形状；分别断言英文/中文检查名、cause 的完整字面值、hasDisclosure 和展开后的无障碍文本，并确认没有重复 reasonLabel 或虚构 action。附带既有 lock_live 原因说明和 unknown code/reason 负例，防止改成无条件文案或继续误读 reason。
- 验证：原始 `make test-macos-app`（单次进程 PATH 使用 Go 1.27.3，GOCACHE 为隔离构建缓存）exit 0、TEST SUCCEEDED；Shared 133、App 169、Widget 51，合计 353 项、0 failure、1 项既有 schema opt-in skip。新增测试通过，耗时 0.160 秒；harness 未报告 helper 泄漏。未增加重试、超时或跳过。记录结构、局部链接、空白与 `git diff --check` 通过。
- 影响评估：相对开发候选，产品/测试修复仅涉及 DesktopCopy.swift、MenuBarViewModel.swift、MenuBarViewModelTests.swift；后两文件为本次受影响消费路径，加入原指纹范围，配方仍为 HEAD 与排序路径/blob 行的 SHA-256。其余原 26 输入中除 tasks.md 状态和 DesktopCopy.swift 外的 24 个 blob 完全相同。状态路径、所有权会话、确定性锁测试、真实 Go helper 夹具的证据显式复用；health-copy 和 native-regression 使用本次新证据。Foundation verifier 的 Shared/Verification 输入未变，复用其原通过结果。
- 候选：HEAD `a4a5d684bc1e2d067f8e92113aa1c7f064bd4ba5`；ContentState `installed-state-ownership:app-state-location:repair-r1-f1-20261110`，共 28 个范围输入。修复后精确状态查询为 NOT_VERIFIED，仅 independent-review 缺失，无失效证据或未解决候选影响；不复用旧候选的失败评审作为新候选的 PASS。
- 交接：Task 6 保持 Dev 完成、Review 未勾选；Beads 回到 in_review 并释放负责人。无提交、推送、安装或账号操作。下一步只复评 ISO-APP-R1-F1 及受影响的 Health 文案行为，其余任务范围不扩张。

## Round 2 — 2026-11-10

## 📋 Task 6 复评：ISO-APP-R1-F1

📊 总体评分：9/10

✅ Verdict: PASS

- Reviewer: Claude Code，独立复评会话；未参与 Repair 1，未修改产品代码、测试或配置。
- Method: 对照 Round 1 发现与 Repair 1 记录，检查修复差异、Health 文案查找顺序和新增回归测试断言是否会因原缺陷失败；按既定配方复算修复候选指纹；核对 CEv1 中修复态各准则的显式复用评估。未重跑原生套件：修复态 native-regression 证据为同一精确状态的新取证。
- Reviewed state: HEAD `a4a5d684bc1e2d067f8e92113aa1c7f064bd4ba5`；修复候选 `installed-state-ownership:app-state-location:repair-r1-f1-20261110`；28 输入 SHA-256 `6e5da36e2499dc8699f0163373138e626399222af676c5627eb0c1daf08ade9e`，本轮复算一致。
- Completion gate: 见文末收口状态。

### 🔴 严重问题 — 必须修复

无。

### 🟡 改进建议 — 建议处理

无。

### 🟢 优点

- **ISO-APP-R1-F1 closed:** 两条说明从 `healthReasonKeys` 移入新的 `healthCodeCauseKeys`（`apps/macos/AgentDeckApp/DesktopCopy.swift:222`）；`MenuBarViewModel.healthCause`（`apps/macos/AgentDeckApp/MenuBarViewModel.swift:1590`）在既有 reason cause 未命中后按 `check.code` 取说明，与 doctor/CLI 契约中 code/reason 的分工一致。`Localizable.xcstrings` 键值未变，`allKeys` 本地化清单仍包含两条文本；线路字段、布局、其他 reason 说明与 action/recovery 判定未改。
- 回归测试 `testInstallationHealthDescriptionsUseWireCodesInBothLanguages`（`apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift:1067`）使用真实形状 `installation/warning/not_installed/receipt_missing` 与 `managed_command_owner/warning/managed_command_owner_mismatch/claude,codex`，经 `WireFixture` 解码与 `readyModel` 走完整视图模型路径；中英文分别断言检查名、cause 字面值、disclosure 与展开后的无障碍文本，并断言不生成 reasonLabel 或 action。在 Round 1 的代码上 cause 为 nil，断言必然失败，因此能防住原缺陷。
- 负例同样有效：`future_check` 的 `reason: not_installed` 不再产生说明，证明不会退回按 reason 误读；既有 `lock_live` 说明与标签保持不变。
- 修复范围克制，只触及 Health 文案消费路径；修复记录如实说明 Round 1 结论不由修复者改签，并把受影响的两个文件加入指纹范围。

### 📝 总结

ISO-APP-R1-F1：closed（修复候选中已更正查表依据并有能失败的回归测试）。Round 1 无其他发现。被审为修复候选 `repair-r1-f1-20261110`（digest `6e5da36e…`），全部发现关闭，PASS。残余不确定性维持 Round 1 所述：查询失败即释放共享锁属保守解释，不构成发现；Task 8 的真实安装器与完整 App 联合 A5 验收不在本任务范围。

Evidence:

1. 本轮复算 28 输入 digest，与修复 ContentState 一致；`git status` 显示的改动集合与 Repair 1 声明一致。
2. 复用修复态 CEv1：health-copy 与 native-regression 为该精确状态新取证（`make test-macos-app` exit 0，353 项 0 failure，1 项既有 opt-in skip，无 helper 泄漏）；state-location、ownership-session、deterministic-tests、native-helper-fixture 为显式目标态复用，复用依据为 Shared runner/session、App 生命周期、telemetry、Go 夹具、harness 与依赖 blob 未变——本轮逐项核对 subject_blobs，确认属实。
3. 未执行安装、启动 App、账号操作或 Git 交付。

收口状态：Review 栏勾选与状态句更新只改变 `tasks.md`，得到 ContentState `installed-state-ownership:app-state-location:review-r2-20261110`（28 输入 SHA-256 `94a891bee1f02367096759036c8ab5783ac8bff01dae61b23449812c2c583bbe`；其余 27 个 blob 与修复候选相同）。独立复评证据记录于修复候选，七项必需准则以显式目标态 roll-up 指向修复态证据；精确状态门禁 VERIFIED 7/7，无失效证据或未解决影响。Beads 转 `awaiting_commit`，未提交或推送。

Task checkpoint：`ad-installed-state-ownership-app-state-location`；ContentState `installed-state-ownership:app-state-location:review-r2-20261110`；VERIFIED。

提交建议：取得明确授权后，单独提交 Task 6 的状态路径、原生所有权会话、Health 文案及其修复、Swift/Go 夹具与 harness、tasks.md 状态和本评审记录；保留 Codex 与 Claude 贡献者署名并核验 SSH 签名。

推送建议：取得独立推送授权后，核验提交范围、正文、trailer 和签名，再推送 `origin/feature/installed-state-ownership`；未执行推送。
