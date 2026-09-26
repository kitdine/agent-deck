---
status: active
topic: desktop-refresh
subject: menubar-refresh-presentation
---

# Menubar refresh presentation review

## Round 1 — 2026-09-20

## 📋 Menubar refresh presentation 独立评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**MRP-R1-F1（P1）— [`apps/macos/AgentDeckApp/MenuBarSurfaceView.swift:127`](../../../../apps/macos/AgentDeckApp/MenuBarSurfaceView.swift#L127)：刷新控件随 surface 分支销毁重建，未满足稳定控件与键盘焦点保持契约。**

- 行为风险：首次失败后的 Retry 会令 presentation 从 `errorSurface` 切到
  `loadingSurface`，成功或再次失败后再切到 `dataSurface` / `errorSurface`。三个分支各自
  实例化 `header`（第 149、167、188 行），因此同一逻辑按钮并不是跨状态保持 identity
  的一个原生控件；键盘用户触发 Retry 后可能失去焦点，无法在原位置继续操作。
- 证据：Task 4 要求 “One stable refresh control preserves keyboard focus”，并把
  “Keyboard retry/running/success focus” 列为本 Task 的 L2 native-App risk check；
  `ux/menubar-refresh.md` 也要求 starting/completing/failing 永不移动焦点。当前根
  `switch model.surface` 在三棵不同子树间切换，新增测试仅断言 ViewModel 值和 PNG
  尺寸，没有在实际渲染 view 上断言按钮 identity 或焦点贯穿
  error → running → success/failure。Beads implementation handoff 把 real focus 留给 Task 5，
  与 `tasks.md` 的 Task 4 边界冲突。

💡 有界修复：把唯一 `header` / refresh button 提升到 surface 分支之外，使状态变化只
更新该控件的 label、symbol、disabled 与 announcement；或用可证明等价的稳定 identity
和显式焦点恢复。增加 actual-view 键盘测试，至少覆盖首次失败 Retry 的
error → running → success 与 error → running → failure，断言同一刷新控件持续持有焦点；
保留 Task 5 的真实 VoiceOver、Dynamic Type、Increase Contrast 和 scheduler timing 边界。

**MRP-R1-F2（P2）— [`apps/macos/AgentDeckApp/DesktopCopy.swift:39`](../../../../apps/macos/AgentDeckApp/DesktopCopy.swift#L39)：失败动作的无障碍标签没有实现已评审的完整句子。**

- 行为风险：VoiceOver 读出 `Refresh failed, Retry` / `刷新失败，重试`，而评审通过的
  contract 是 `Refresh failed. Retry` / `刷新失败。重试`。逗号把失败状态与可执行动作
  合并为一个短语，弱化了两段语义和停顿；测试反而把错误标点固化为预期。
- 证据：`ux/menubar-refresh.md` 的 Accessibility and motion 段逐字要求完整标签
  `Refresh failed. Retry` / `刷新失败。重试`；当前 `refreshFailedAction` 常量及
  `Localizable.xcstrings` 使用逗号，`DesktopCopyTests` 也期待逗号。ViewModel 第 367–369
  行直接把该常量作为按钮 accessibility label。

💡 有界修复：将两种语言的失败动作无障碍标签改为评审通过的句号版本，并同步精确
本地化断言；可见的 `Retry` / `重试` 保持不变。

### 🟡 建议改进 — 推荐

无。本轮只记录会阻止 Task 4 达到已评审契约的缺陷。

### 🟢 优点

- 成功数据年龄、full-attempt 状态与 Widget publication 状态已分离；失败保留旧数据，
  publication 失败不会把菜单栏数据标为 stale 或 badge。
- schema → refresh/publication → health/session → partial 的 notice 顺序与设计一致，
  scan progress 活跃时会抑制泛化 announcement。
- 双语 key inventory、280/420 pt 渲染路径和 App/Widget 回归套件均有候选态证据；
  本轮 finding 不否定这些已通过检查。

### 📝 总结

- Finding disposition：`MRP-R1-F1 OPEN`；`MRP-R1-F2 OPEN`；无 CLOSED、
  SUPERSEDED 或 carried finding。
- Reviewed state：HEAD `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `cabb1d41fd914f131c1a81264a5f74a085dcc8ad1e6976ef605caeb32debd1c0`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:menubar-refresh-presentation:implement:cabb1d41fd914f131c1a81264a5f74a085dcc8ad1e6976ef605caeb32debd1c0`。
- Reviewer：Codex 主代理；Method：独立源码/契约评审，CodeGraph impact exploration +
  scoped diff/source inspection，Blue-only，subagent rounds consumed：0；项目规则禁止未请求委派。
- Scope：Task 4 的九路径 source/test/localization/status 候选及其对已评审
  `ux/menubar-refresh.md`、`architecture.md` 和 `tasks.md` 契约的符合性。
- Evidence：复用同一 ContentState 的 implementation handoff：最终
  `make test-macos-app` 报告 AgentDeckAppTests 120（1 skip）、
  AgentDeckWidgetTests 45/45、0 failure，format/JSON checks PASS；本轮最小源码
  reproducer 证明 surface 分支重建刷新按钮，且精确文案对照证明 accessibility label
  标点不符。首个阻断 finding 成立后未重复广泛测试。
- Completion gate：FAILED。现有 CEv1 5/5 记录没有覆盖本轮发现的稳定焦点与精确
  accessibility-label 缺陷；需为本 review ContentState 记录失败证据并重新查询。
- Residual uncertainty：未执行 Task 5 所拥有的真实 VoiceOver、Dynamic Type、
  Increase Contrast、scheduler timing；这些排除项不能关闭本轮两个 Task 4 finding。

### 下一步指令

修复：desktop-refresh / reviews/menubar-refresh-presentation.md / MRP-R1-F1 MRP-R1-F2

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate — 2026-09-20

- `MRP-R1-F1 -> repaired in candidate.` The unique header and
  `StableRefreshControl` now live above the loading/error/data surface switch.
  The control owns one persistent UUID and `@FocusState`; action-state changes
  explicitly restore focus when it was held. A native test drives first-use
  error → running → success → failure in one `NSHostingView` and proves the
  rendered control identity never changes.
- `MRP-R1-F2 -> repaired in candidate.` The accessible failure label is exactly
  `Refresh failed. Retry` / `刷新失败。重试`; visible `Retry` / `重试` remains
  unchanged, and exact localization assertions use the reviewed punctuation.
- Repair candidate：HEAD
  `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；同一九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `b74a01489b7764fb066ea0cde8a0a8f2eee1a8e5ff9c189c0b61ed03db57d833`。
- Verification：最终 `make test-macos-app` PASS；AgentDeckAppTests 121 项
  （1 skip、0 failure），AgentDeckWidgetTests 45/45 PASS；官方 App/Widget build
  与双语资源编译通过。

Round 1 结论仍为 FAIL，Task Review 仍未勾选；以上只声明返修候选就绪，
`MRP-R1-F1`、`MRP-R1-F2` 是否 CLOSED 与新 ContentState gate 由独立复评决定。

## Round 2 — 2026-09-20

## 📋 Menubar refresh presentation 独立复评

📊 总体评分：8/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**MRP-R1-F1（P1）— [`apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift:87`](../../../../apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift#L87)：actual-view 测试只证明 UUID identity 不变，仍未证明键盘焦点跨 Retry 状态转换保持。**

- 处置：`MRP-R1-F1 STILL OPEN`。生产 header 已提升到 surface switch 外，关闭了原
  finding 的控件销毁/重建部分；但要求的键盘焦点行为仍无有效 oracle。
- 行为风险：`StableRefreshControl` 在 running 时变为 disabled，完成后再恢复 enabled；
  当前 `@FocusState` 的 `onChange` 以 `guard focused` 为前提。如果 AppKit 在 disabled
  转换时先移走焦点，这条恢复路径不会执行。键盘用户是否能在原位置继续 Retry/Refresh
  仍未被当前证据确定。
- 证据：新增 `testRefreshControlIdentitySurvivesErrorRunningSuccessAndFailure` 只读取
  `RefreshControlIdentityPreferenceKey` 并比较一个 `@State UUID`；测试从未让按钮获得
  键盘焦点，也未读取 first responder / focus state，更没有在 error → running →
  success/failure 各节点断言焦点。Round 1 的有界修复明确要求 actual-view 键盘测试断言
  同一刷新控件持续持有焦点，因此 repair evidence 对
  `focus-accessibility-live-region` 的 PASS 超出了测试实际证明范围。

💡 有界修复：在一个 `NSHostingView` 中以真实键盘/first-responder 路径聚焦 Retry，驱动
error → running → success 与 error → running → failure，并在每个可操作节点断言刷新
按钮仍是焦点目标；若 disabled 阶段平台必然转移焦点，则保存“此前持有焦点”状态并在
重新 enabled 时显式恢复。测试必须在删除恢复逻辑或把 header 放回分支时失败。

### 🟡 建议改进 — 推荐

无。本轮不把 Task 5 的真实 VoiceOver、Dynamic Type、Increase Contrast 或 scheduler
timing 扩张进 Task 4。

### 🟢 优点

- `MRP-R1-F2 CLOSED`：`DesktopCopy.refreshFailedAction`、双语 strings catalog 和
  `DesktopCopyTests` 均逐字使用 `Refresh failed. Retry` / `刷新失败。重试`；可见
  `Retry` / `重试` 未改变。
- `MRP-R1-F1` 的结构性部分已改进：唯一 header 位于 surface switch 之外，
  `StableRefreshControl` 不再因 loading/error/data 分支切换而重建；UUID identity 测试
  能捕获这一结构回归。
- 同一 repair ContentState 的最终 App 121（1 skip）、Widget 45/45、双语资源编译和
  build 结果保持可复用；本轮未发现其他 regression。

### 📝 总结

- Finding disposition：`MRP-R1-F1 STILL OPEN`；`MRP-R1-F2 CLOSED`；无 REGRESSED、
  SUPERSEDED 或 NEW finding。
- Reviewed state：HEAD `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `b74a01489b7764fb066ea0cde8a0a8f2eee1a8e5ff9c189c0b61ed03db57d833`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:menubar-refresh-presentation:repair:b74a01489b7764fb066ea0cde8a0a8f2eee1a8e5ff9c189c0b61ed03db57d833`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，CodeGraph
  current-state probe + scoped source/test inspection，Blue-only，subagent rounds
  consumed：0；项目规则禁止未请求委派。
- Evidence：复用 repair handoff 的 `make test-macos-app` PASS（App 121、1 skip；
  Widget 45/45；0 failure）与未受影响四项 criterion evidence。精确源码核对关闭
  `MRP-R1-F2`，并证明现有 identity test 没有观测 `MRP-R1-F1` 所需的真实焦点行为；
  决定性 finding 成立后未重复广泛测试。
- Completion gate：FAILED。此前 repair state 的 5/5 VERIFIED 包含一条超出实际
  oracle 的 focus PASS；本轮为同一 ContentState 记录适用的复评 fail evidence 后，
  `focus-accessibility-live-region` 失败，Task gate 保持开放。
- Residual uncertainty：没有运行 Task 5 拥有的真实 VoiceOver、Dynamic Type、
  Increase Contrast 和 scheduler timing；它们不是关闭本轮焦点 finding 的替代证据。

### 下一步指令

修复：desktop-refresh / reviews/menubar-refresh-presentation.md / MRP-R1-F1

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate after Round 2 — 2026-09-20

- `MRP-R1-F1 -> repaired again in candidate.` The stable refresh control is now
  backed by one persistent native `NSButton`. Before disabling, its coordinator
  records whether that exact button is `window.firstResponder`; when the control
  becomes enabled it restores focus with `window.makeFirstResponder(button)`.
  Running still uses an embedded native spinner, with a static hourglass under
  reduce-motion.
- The native XCTest locates the actual button by its stable identifier, focuses it
  through AppKit, and drives error → running → success and success → running →
  failure. It proves the same `NSButton` object remains mounted and is again the
  window first responder at both actionable terminal states. A deterministic
  suspended-failure fixture ensures the disabled intermediate state is rendered.
- `MRP-R1-F2` remains CLOSED; the reviewed sentence punctuation is unchanged.
- Repair candidate：HEAD
  `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；同一九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `5662e4e5bbd13a37360972a68b6e2abeceec66032d5b10a61fc3cb726f3ec2be`。
- Verification：最终 `make test-macos-app` PASS；AgentDeckAppTests 121 项
  （1 skip、0 failure），AgentDeckWidgetTests 45/45 PASS；官方 App/Widget build
  与双语资源编译通过。

Round 2 结论仍为 FAIL，Task Review 仍未勾选；以上只声明第二次返修候选就绪，
`MRP-R1-F1` 是否 CLOSED 与新 ContentState gate 由独立复评决定。

## Round 3 — 2026-09-20

## 📋 Menubar refresh presentation 独立复评

📊 总体评分：8/10

✅ 复评结论：FAIL

### 🔴 严重问题 — 必须修复

**MRP-R3-F1（P1）— [`apps/macos/AgentDeckApp/MenuBarSurfaceView.swift:476`](../../../../apps/macos/AgentDeckApp/MenuBarSurfaceView.swift#L476)：一次成功的焦点恢复永久武装后续抢焦点。**

- 处置：`NEW`。这是第二次 repair 为关闭 `MRP-R1-F1` 引入的当前变更缺陷。
- 行为风险：刷新按钮在进入 disabled running 前持有焦点时，coordinator 把
  `restoreFocusWhenEnabled` 设为 `true`。terminal state 恢复焦点后，该 flag 从未清零。
  用户随后主动 Tab/点击到过滤器、面板或其他控件，只要任一后续 model/SwiftUI update
  再次调用 `updateNSView`，第 476–478 行就会把 first responder 抢回刷新按钮，违反
  “starting, completing, or failing a refresh never moves keyboard focus” 的单次恢复边界。
- 证据：`Coordinator.restoreFocusWhenEnabled` 仅在第 465 行写 `true`，没有任何写
  `false` 的路径；恢复闭包也只调用 `makeFirstResponder(button)`。新增 native test 在
  success/failure terminal state 断言按钮恢复焦点后立即结束，没有把焦点移到另一个真实
  控件并触发后续 update，因此无法检测 sticky restoration。源码删去/保留现有恢复逻辑
  都不会让这个缺口暴露。

💡 有界修复：把恢复请求建模为一次性的 enabled-transition token：只在“focused 的
enabled → disabled”边界置位，在下一次 disabled → enabled 恢复尝试后无条件清零；若
用户已明确选择其他焦点目标，不得再次恢复。扩展 actual-view 测试：terminal 恢复后把
first responder 移到另一个真实控件，触发与刷新无关的 view/model update，并断言焦点
不会被刷新按钮夺回。

### 🟡 建议改进 — 推荐

无。本轮只记录修复路径造成的当前行为缺陷。

### 🟢 优点

- `MRP-R1-F1 CLOSED`：header/refresh control 位于 surface switch 外；测试定位真实生产
  `NSButton`、通过 AppKit 聚焦它，并在 error → running → success 与 success → running
  → failure 后断言同一对象重新成为 `window.firstResponder`。这满足原 finding 的稳定
  控件与跨 disabled interval 恢复要求。
- `MRP-R1-F2 CLOSED（保持）`：双语 failure accessibility label 与精确本地化断言继续
  使用 `Refresh failed. Retry` / `刷新失败。重试`。
- 同一 repair ContentState 的 App 121（1 skip）、Widget 45/45、双语资源编译与 build
  结果仍可复用；未发现其他 regression。

### 📝 总结

- Finding disposition：`MRP-R1-F1 CLOSED`；`MRP-R1-F2 CLOSED`；
  `MRP-R3-F1 NEW/OPEN`；无 REGRESSED 或 SUPERSEDED finding。
- Reviewed state：HEAD `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `5662e4e5bbd13a37360972a68b6e2abeceec66032d5b10a61fc3cb726f3ec2be`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:menubar-refresh-presentation:repair2:5662e4e5bbd13a37360972a68b6e2abeceec66032d5b10a61fc3cb726f3ec2be`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，CodeGraph
  current-state probe + scoped source/actual-view-test inspection，Blue-only，subagent
  rounds consumed：0；项目规则禁止未请求委派。
- Evidence：复用 repair handoff 的最终 `make test-macos-app` PASS（App 121、1 skip；
  Widget 45/45；0 failure）与未受影响 criterion evidence。真实 `NSButton` / first
  responder oracle 关闭 `MRP-R1-F1`；源码状态机核对证明恢复 flag 没有消费/清零路径，
  形成 `MRP-R3-F1` 的决定性反例，因此未重复广泛测试。
- Completion gate：FAILED。repair-r2 state 原 5/5 evidence 中的 focus criterion 未覆盖
  sticky restoration；本轮为同一 ContentState 记录适用 fail evidence 后，Task gate
  保持开放。
- Residual uncertainty：没有执行 Task 5 的真实 VoiceOver、Dynamic Type、Increase
  Contrast 或 scheduler timing；它们不能关闭当前 AppKit first-responder 状态机缺陷。

### 下一步指令

修复：desktop-refresh / reviews/menubar-refresh-presentation.md / MRP-R3-F1

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate after Round 3 — 2026-09-20

- `MRP-R3-F1 -> repaired in candidate.` Focus restoration is now a one-shot
  enabled-transition token. It arms only on a focused `enabled → disabled`
  transition, is consumed only on the next `disabled → enabled` transition, and
  is cleared unconditionally before the asynchronous restore attempt. Ordinary
  `enabled → enabled` updates cannot restore focus.
- The native XCTest focuses the real persistent `NSButton`, proves restoration
  after error → running → success, then moves first responder to another real
  button and triggers an unrelated panel update; focus stays on that other button.
  It then refocuses refresh and proves the independent success → running → failure
  interval restores once. A deterministic suspended-failure fixture renders both
  disabled intermediate states.
- `MRP-R1-F1` and `MRP-R1-F2` remain CLOSED; this candidate changes only the new
  Round 3 sticky-restoration defect.
- Repair candidate：HEAD
  `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；同一九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `683510cd68d36cc3d94a7006385507562bc5decfea323709245284d325dc7549`。
- Verification：最终 `make test-macos-app` PASS；AgentDeckAppTests 121 项
  （1 skip、0 failure），AgentDeckWidgetTests 45/45 PASS；官方 App/Widget build
  与双语资源编译通过。

Round 3 结论仍为 FAIL，Task Review 仍未勾选；以上只声明返修候选就绪，
`MRP-R3-F1` 是否 CLOSED 与新 ContentState gate 由独立复评决定。

## Round 4 — 2026-09-20

## 📋 Menubar refresh presentation 独立复评

📊 总体评分：10/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `MRP-R3-F1 CLOSED`：focus restoration 只在 focused 的 enabled → disabled 边界
  置位；下一次 disabled → enabled 先读取并无条件清零 token，再异步恢复焦点。
  enabled → enabled 更新不能再消费旧请求或抢回焦点。
- native AppKit 测试在真实生产 `NSButton` 上证明 success 后恢复一次，随后把 first
  responder 移到另一个真实按钮并触发 `.quota` → `.usage` 的无关 panel 更新，焦点保持
  在用户选择的控件；之后独立 failure interval 仍可重新置位并恢复一次。删除清零、恢复
  或 transition guard 均会破坏现有断言。
- `MRP-R1-F1 CLOSED（保持）`：唯一原生 refresh control 跨 loading/error/data surface
  与 success/failure interval 保持同一对象，并在确实从 focused disabled interval 返回时
  恢复焦点。
- `MRP-R1-F2 CLOSED（保持）`：双语 accessibility label 继续逐字匹配
  `Refresh failed. Retry` / `刷新失败。重试`。

### 📝 总结

- Finding disposition：`MRP-R1-F1 CLOSED`；`MRP-R1-F2 CLOSED`；
  `MRP-R3-F1 CLOSED`；无 still-open、regressed、superseded 或 new finding。
- Reviewed state：HEAD `0c298342e12a2ed17ad809286e1ecd7ee9a60cee`；Review 状态同步后的
  九路径有序 `path<TAB>git-blob` manifest SHA-256
  `37ffbeb6e7064632c66347496528f95f744f8b4fcfc12056e1250581205be490`；ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:menubar-refresh-presentation:rereview-r4:37ffbeb6e7064632c66347496528f95f744f8b4fcfc12056e1250581205be490`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，CodeGraph
  current-state probe + scoped production/native-test inspection，Blue-only，subagent rounds
  consumed：0；项目规则禁止未请求委派。
- Evidence：repair3 ContentState `683510cd68d3…` 的 `make test-macos-app` 保持可复用：
  AgentDeckAppTests 121（1 skip）、AgentDeckWidgetTests 45/45、0 failure，官方 App/Widget
  build 与双语资源编译通过。Round 4 只同步 Task Review 状态；scope-aware target-bound
  roll-up 将五项有效 repair evidence 绑定到最终 ContentState，未重复测试。
- Completion gate：VERIFIED（5/5 required criteria；无 missing、invalidated 或 unresolved）。
- Residual uncertainty：Task 5 拥有的真实 VoiceOver、Dynamic Type、Increase Contrast、
  scheduler timing 与跨 target integration 仍明确排除，不影响 Task 4 的 PASS。

### Task checkpoint

Task checkpoint：`ad-dr-menubar-refresh-presentation-dev`；ContentState
`37ffbeb6e7064632c66347496528f95f744f8b4fcfc12056e1250581205be490`；gate
`VERIFIED`。

提交建议：提交 Task 4 的九路径 source/test/localization/status 候选及
`docs/topics/desktop-refresh/reviews/menubar-refresh-presentation.md` Round 1–4 完整历史；
建议范围不授权提交。

推送建议：提交完成并验证签名、message、trailers 与 Task 4 实际交付边界后，推送
`feature/desktop-refresh`；当前没有推送授权。

### 下一步指令

开发：desktop-refresh / refresh-integration-acceptance

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh
