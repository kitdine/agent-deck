---
status: active
topic: desktop-refresh
subject: widget-loading-and-timeline
---

# Widget Loading and Timeline Review

## Round 1 — 2026-09-20

**Checklist: 54/54 complete**<br>
**Incomplete: None**

## 📋 Widget loading and timeline 实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**WLT-R1-F1 — 高：typed failure 的必需 L3 语言、无障碍与环境矩阵没有实现，且候选 gate 的 passing evidence 全部 malformed。**

位置：`apps/macos/AgentDeckWidgetTests/WidgetPresentationTests.swift:367`–`:398`；
`apps/macos/AgentDeckWidgetTests/WidgetCopyTests.swift:3`–`:16`；合同位于
`docs/topics/desktop-refresh/tasks.md:214`–`:222`。

- 行为风险：四类读取失败会进入所有五种 Widget kind 和三种 family，但当前
  rendering test 只切换 light/dark、生成 PNG 并断言字节数大于 1000。它没有切换
  English/简体中文、没有断言 failure title/footer 的精确本地化值、没有读取 AX
  label/value/order，也没有在 failure surface 上执行 Dynamic Type 或 reduced-motion
  环境。标题、footer、VoiceOver 顺序或大字体降级若回归，现有 43/43 Widget tests
  仍可全部通过，违反 Task 3 明列的 L3 验收边界。
- 证据：`testTypedFailureRenderingsCoverEveryKindFamilyAndTheme` 的循环维度只有
  failure/kind/family/colorScheme，最终 oracle 只有 `surface == .unavailable` 与 PNG
  非空；`WidgetCopyTests.testEveryKeyResolvesInBothShippedLanguages` 只证明 key 存在且
  非空，不证明 reviewed copy 值或实际 failure view 使用对应语言。全目录检索没有
  failure matrix 的 `testLanguage`/locale、AX tree/order 或 reduced-motion 断言。
  官方 gate 对 ContentState `238a97c6…` 返回 `NOT_VERIFIED`：五条 pass evidence
  全部 `malformed`；其中 L3 evidence 的 `work_unit_id` 是逻辑短 ID
  `desktop-refresh:widget-loading-and-timeline`，不等于 WorkUnit 稳定节点 ID
  `urn:ce:agent-deck:work-unit:desktop-refresh-widget-loading-and-timeline`。

💡 修复范围：在现有 Widget tests 中增加最小、参数化的 failure-surface matrix：
对四种 failure 覆盖五 kind/三 family，并分别加载 English/简体中文 bundle，精确断言
reviewed title/footer；对共享 failure frame 断言 header → typed body → footer 的 AX
语义与顺序；覆盖 accessibility Dynamic Type 和 reduced-motion 环境，使用真实几何/
语义 oracle，而不是只检查 PNG 非空。复用当前 Xcode Widget test target 和稳定语义
helper，不新增并行测试框架。重新通过 canonical CEv1 provider 写入当前候选证据，
使 `work_unit_id`/`criterion_id`/`observed_at` 与图关系一致。参考 Apple
[XCTest documentation](https://developer.apple.com/documentation/xctest)
（2026-09-20 核验）：XCTest 是当前原生 UI/行为断言的项目既有机制；等价、可证伪的
原生断言实现也可接受。

Disposition：OPEN，交回同一任务
`ad-dr-widget-loading-and-timeline-dev` 修复。

### 🟡 建议改进 — 推荐

无。已有决定性阻塞 finding，按项目规则停止扩大广泛验证。

### 🟢 优点

- Widget reader 已复用 Task 1 的 8 MiB bounded bytes，保留 missing/container/
  unsafe/oversized/decode/schema 的 typed outcome，没有重新引入 `Data(contentsOf:)`。
- 三类 provider 都以注入的实际 invocation time 生成一个 entry，3/4/5 分钟 clamp
  与 14:59、15:00、6:00:00、>6h/sleep-jump 边界已有确定性单元保护。
- failure path 通过 `WidgetLoadOutcome.failed` 进入 typed unavailable frame，不再用
  optional snapshot 或 `try?` 吞掉原因，静态路径不会渲染 retained/zero data。

### 📝 总结

- Reviewer：Codex 主代理；Method：正式代码/测试 delivery review，使用
  workspace-bound CodeGraph 和直接源码核对；Independent review panel：Blue-only，
  subagent rounds consumed：0（项目规则禁止未请求委派）。
- Scope：Task 3 的九个 Widget source/test/localization/status 路径；Task 1 shared
  bounded reader 仅作为依赖核对，Task 2 host coordinator 和 Task 5 native timing
  明确排除。现有 surface 的唯一授权变化是 reviewed typed failure copy 和 3/4/5
  minute timeline。
- Reviewed state：workspace `agent-deck.desktop-refresh`，branch
  `feature/desktop-refresh`；HEAD
  `cdb6ca1e5922093f4ba189d8ad15063416faf1d0`；九路径有序
  `path<TAB>git-blob` manifest SHA-256
  `238a97c69f8dbe8afbc7ab7cc61d9dd92baf1f730bc9d77018b8712a8af60a6c`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:widget-loading-and-timeline:implement:238a97c69f8dbe8afbc7ab7cc61d9dd92baf1f730bc9d77018b8712a8af60a6c`。
- Acceptance matrix：typed bounded loading、timeline/clock、privacy/compatibility、
  typed presentation implementation 为 COMPLETE；Task 3 必需 L3 failure matrix 为
  OMITTED，因此总体验收 FAIL。旧 `Data(contentsOf:)`、optional/`try?` runtime path、
  15–60 minute policy 均已移除；兼容 initializer 仅保留测试/既有构造便利，不是生产
  loader 双路径。
- Test/document actions：`WidgetTimelineTests` KEEP；现有 failure rendering test
  UPDATE；`WidgetCopyTests` UPDATE；新增参数化语言/AX/Dynamic Type/reduced-motion
  oracle 为 ADD；无 DELETE/MERGE。`tasks.md` 与 `ux/widget-refresh.md` 为 KEEP，未发现
  需要改写的产品合同。
- Evidence：实现交接报告官方 `make test-macos-app` 为 App 116 tests（1 skip）、
  Widget 43/43、0 failures；本轮在候选未变化时复用该结果。它证明 build/test 执行，
  但因上述 oracle 缺失不能证明 L3 matrix。官方 CEv1 gate 查询为
  `NOT_VERIFIED`，不是 handoff 所称 `VERIFIED 5/5`。
- Reuse/custom code：共享 bounded reader、WidgetKit/AppIntent provider 和现有 XCTest
  target 均为 REUSE_EXISTING；本变更未引入第三方依赖。typed outcome 与小型 timeline
  policy 是紧贴 repository contract 的 KEEP_CUSTOM 域逻辑。
- Residual uncertainty：决定性 blocker 后未重复 full macOS suite，也未执行 Task 5
  拥有的真实 WidgetKit requested/actual timing。后者不属于本 finding，不能替代缺失的
  Task 3 native failure-surface oracle。

### 下一步指令

修复：desktop-refresh / reviews/widget-loading-and-timeline.md / WLT-R1-F1

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate — 2026-09-20

- `WLT-R1-F1 -> repaired in candidate.` Typed failure rendering now consumes
  `WidgetFailureSurfaceSemantics`, which emits the ordered header → body → footer
  accessibility descriptors and localized title/footer used by the real view.
  `WidgetHeader` consumes the same header presentation helper, so the semantic
  oracle is not a parallel test-only copy.
- The parameterized matrix covers four failures × five kinds × three families ×
  English/Simplified Chinese with exact reviewed title/footer values and exact AX
  role/order/label/value assertions. Real SwiftUI rendering covers light/default
  Dynamic Type and dark/accessibility5, uses a reduce-motion override that disables
  transaction animation, and verifies logical point geometry independent of Retina
  backing scale.
- Round 1's old candidate is the reproducer: it has no localized semantic/AX matrix,
  no writable reduce-motion seam, and only a nonempty-PNG oracle. The first repaired
  geometry run failed deterministically when it compared 2× Retina pixels to logical
  points; the final scale-independent point oracle passes without weakening coverage.
- Repair candidate：HEAD
  `cdb6ca1e5922093f4ba189d8ad15063416faf1d0`；Task 3 原九路径加 finding 授权的
  `WidgetCopyTests.swift`，十路径有序 `path<TAB>git-blob` manifest SHA-256
  `e01451d658c0025d407d1dda8563e9e36fc75d27dc8c68d448ff4a9d960754ad`。
- Verification：最终 `make test-macos-app` PASS；AgentDeckAppTests 116 项
  （1 skip、0 failure），AgentDeckWidgetTests 45/45 PASS；官方 App/Widget build、
  双语资源编译均通过。topic-docs、JSON、whitespace 与 diff checks 待最终内容态
  同步后执行。

Round 1 结论仍为 FAIL，Task Review 仍未勾选；以上只声明返修候选就绪，
`WLT-R1-F1` 是否 CLOSED 与新候选 completion gate 由独立复评决定。

## Round 2 — 2026-09-20

**Checklist: 54/54 complete**<br>
**Incomplete: None**

## 📋 Widget loading and timeline 独立复评

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**WLT-R1-F1 — 高：STILL OPEN。返修增加了平行语义数组与 canvas-size 检查，但仍未验证真实 failure view 的 AX 顺序和元素几何。**

位置：`apps/macos/AgentDeckWidget/WidgetViews.swift:279`–`:289`、`:501`–`:570`；
`apps/macos/AgentDeckWidgetTests/WidgetPresentationTests.swift:367`–`:448`。

- Disposition：`WLT-R1-F1` 部分修复但仍开放。双语精确 copy、light/dark、
  accessibility5、reduce-motion seam 和 canonical evidence ID 已补齐；真实 view oracle
  仍缺失。
- 行为风险：`WidgetFailureSurfaceSemantics.accessibilityNodes` 声明
  header → body → footer，但 production view 只把数组的 body descriptor 用在
  `TypedUnavailableWidget`。真实 `WidgetHeader` 与 `WidgetFooter` 各自重新计算语义，
  它们的顺序由 `WidgetFrame` 的 `VStack` 决定。若 production 把 footer 移到 header
  前、漏掉 footer AX modifier、或 failure title/footer 被裁切，semantic-array test 仍
  全绿。`NSBitmapImageRep.image.size == WidgetLayoutContract.canvas` 只证明输出 bitmap
  canvas 是请求尺寸，不证明 header/body/footer 的 frame、可见性、无溢出或顺序。
- 证据：`testTypedFailureSemanticMatrixUsesExactLocalizedAXOrderAndNoMotion` 只检查
  `semantics.accessibilityNodes.map(\.role)`，没有读取实际 SwiftUI/AppKit accessibility
  tree；`testTypedFailureRenderingsCoverEveryKindFamilyAndTheme` 只检查整个 bitmap 的
  logical size，没有捕获任何 failure 元素 frame。生产 `TypedUnavailableWidget` 只消费
  `accessibilityNodes[1]`；header/footer 不消费 `[0]`/`[2]`。因此原 Round 1 的
  mutation（实际 view order/AX/element geometry 改坏而 helper 不变）仍不能被测试检出。

💡 修复范围：让同一 production semantic contract 真正驱动 header、body、footer
三处 AX descriptor，或从实际渲染 view 提取 AX tree，断言真实顺序、label 和 value；
使用真实 element frame/capture seam 断言 failure title/footer 在三种 family 与
accessibility5 下可见、位于 canvas 内且不互相覆盖，而不是只比较整张 bitmap 尺寸。
保留当前双语精确 copy、reduce-motion 和 canonical CEv1 修复。参考 Round 1 已核验的
Apple [XCTest documentation](https://developer.apple.com/documentation/xctest)；
等价且能杀死上述实际-view mutation 的原生 oracle 可接受。

### 🟡 建议改进 — 推荐

无。本轮只复核 `WLT-R1-F1`，没有扩大范围。

### 🟢 优点

- 双语 failure title/footer 现在逐值断言，不再只检查 key 非空。
- light/default 与 dark/accessibility5 渲染、reduce-motion override 以及十路径 repair
  fingerprint 均已明确；官方 macOS suite 报告 Widget 45/45、0 failure。
- repair ContentState 的五条 evidence 使用稳定 WorkUnit URN，canonical gate 在复评
  前能正确识别为 5/5 applicable；旧 malformed evidence 保留为历史而未被覆写。

### 📝 总结

- Finding disposition：`WLT-R1-F1 STILL OPEN`；无 CLOSED、REGRESSED、SUPERSEDED
  或 NEW finding。
- Reviewed state：HEAD `cdb6ca1e5922093f4ba189d8ad15063416faf1d0`；十路径
  manifest SHA-256
  `e01451d658c0025d407d1dda8563e9e36fc75d27dc8c68d448ff4a9d960754ad`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:widget-loading-and-timeline:repair:e01451d658c0025d407d1dda8563e9e36fc75d27dc8c68d448ff4a9d960754ad`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，Blue-only，
  subagent rounds consumed：0；项目规则禁止未请求委派。
- Evidence：manifest 与 repair state 精确匹配；复用相同 content state 的最终
  `make test-macos-app`（App 116、1 skip；Widget 45/45；0 failure）和五条 canonical
  evidence。源码级 mutation-sensitivity 核对证明这些 pass tests 仍不能检测实际
  view AX order/element geometry 回归，因此 `l3-widget-verification` 当前态失败，
  completion gate 为 FAILED。
- Residual uncertainty：未重复全套测试，因为 content state 未变且已有结果可复用；
  Task 5 的真实 WidgetKit timing 继续排除，它不能关闭当前 actual-view oracle 缺口。

### 下一步指令

修复：desktop-refresh / reviews/widget-loading-and-timeline.md / WLT-R1-F1

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate after Round 2 — 2026-09-20

- `WLT-R1-F1 -> repaired again in candidate.` `WidgetFrame` now passes the same
  `WidgetFailureAccessibilityNode` values into production `WidgetHeader`, failure
  body and `WidgetFooter`. Each component's real AX modifier consumes its node and
  emits that same node through `WidgetFailureAccessibilityPreferenceKey`; tests no
  longer validate a disconnected array.
- The three production elements also emit their actual frames through
  `WidgetFailureGeometryPreferenceKey` in the shared `WidgetFailureFrame` coordinate
  space. The real-view matrix asserts captured AX nodes equal the expected ordered
  header → body → footer contract, all three frames are non-empty and contained in
  the logical canvas, header/body/footer ordering holds, and header/footer do not
  overlap. Moving a production element, dropping its AX modifier, or removing its
  geometry capture now fails the test.
- The matrix retains four failures × five kinds × three families, exact English/
  Simplified Chinese copy, light/default and dark/accessibility5 rendering, explicit
  reduce-motion behavior and Retina-independent logical geometry.
- Repair candidate：HEAD
  `cdb6ca1e5922093f4ba189d8ad15063416faf1d0`；同一十路径有序
  `path<TAB>git-blob` manifest SHA-256
  `130d7809ff9365eebe331a9f1992904f6d03910e9b012743a762c2636b7b11f0`。
- Verification：最终 `make test-macos-app` PASS；AgentDeckAppTests 116 项
  （1 skip、0 failure），AgentDeckWidgetTests 45/45 PASS；官方 App/Widget build
  与双语资源编译通过。

Round 2 结论仍为 FAIL，Task Review 仍未勾选；以上只声明第二次返修候选就绪，
`WLT-R1-F1` 是否 CLOSED 与新 ContentState gate 由独立复评决定。

## Round 3 — 2026-09-20

**Checklist: 54/54 complete**<br>
**Incomplete: None**

## 📋 Widget loading and timeline 独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- `WLT-R1-F1 CLOSED`：production `WidgetFrame` 把同一
  `WidgetFailureSurfaceSemantics.accessibilityNodes` 分别传给真实
  `WidgetHeader`、failure body 和 `WidgetFooter`；三处真实 AX modifier 同时通过
  `WidgetFailureAccessibilityPreferenceKey` 发出所消费的 node，测试不再验证平行数组。
- production 使用命名坐标空间 `WidgetFailureFrame` 捕获 header/body/footer 实际
  frame；finding-scoped matrix 断言三个 role 齐全、frame 非空且在 canvas 内、
  header → body → footer 顺序成立并且 header/footer 不重叠。移动元素、删除 AX
  modifier 或删除 geometry capture 都会让当前 oracle 失败。
- 四 failure × 五 kind × 三 family、English/简体中文精确 copy、light/default、
  dark/accessibility5、reduce-motion 和 Retina-independent logical geometry 均保留。

### 📝 总结

- Finding disposition：`WLT-R1-F1 CLOSED`；无 still-open、regressed、superseded
  或 new finding。
- Reviewed state：HEAD `cdb6ca1e5922093f4ba189d8ad15063416faf1d0`；Review
  状态同步后的十路径 manifest SHA-256
  `055acdf9cb5929f2adeca80bca473951f89d470dd7edb3b007ab8e2d03260f0f`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:widget-loading-and-timeline:rereview-r3:055acdf9cb5929f2adeca80bca473951f89d470dd7edb3b007ab8e2d03260f0f`。
- Reviewer：Codex 主代理；Method：finding-scoped selective follow-up，Blue-only，
  subagent rounds consumed：0；项目规则禁止未请求委派。
- Evidence：第二次 repair state `130d7809…` 的 `make test-macos-app` 结果保持可复用：
  AgentDeckAppTests 116（1 skip）、AgentDeckWidgetTests 45/45、0 failure，官方
  App/Widget build 与双语资源编译通过。当前态仅增加 tasks.md Review 勾选和本轮记录，
  通过 scope-aware assessment 与 target-bound roll-up 后 Task gate 为 VERIFIED 5/5。
- Residual uncertainty：Task 5 拥有的真实 WidgetKit requested/actual timing 继续明确
  排除，不是 Task 3 的验收条件，也不影响本轮 PASS。

### Task checkpoint

Task checkpoint：`ad-dr-widget-loading-and-timeline-dev`；ContentState
`055acdf9cb5929f2adeca80bca473951f89d470dd7edb3b007ab8e2d03260f0f`；gate
`VERIFIED`。

提交建议：提交 Task 3 的十路径 source/test/localization/status 候选及
`docs/topics/desktop-refresh/reviews/widget-loading-and-timeline.md` 完整 Round 1–3
历史；建议范围不授权提交。

推送建议：提交完成并验证签名、message、trailers 与 Task 3 实际交付边界后，推送
`feature/desktop-refresh`；当前没有推送授权。

### 下一步指令

开发：desktop-refresh / menubar-refresh-presentation

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh
