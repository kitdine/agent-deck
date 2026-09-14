---
status: active
topic: subscription-quota
subject: desktop-surfaces
---

# Desktop Surfaces Review

## Round 1 — 2026-09-14

## 📋 Desktop Surfaces 实现评审

📊 总体评分：5/10

✅ 评审结论：FAIL

**Reviewed state：** `urn:agent-deck:content-state:subscription-quota:desktop-surfaces:cfcc395:a9cc1235cf92`（HEAD `cfcc3952003778d66ef70d50ae2db91b36812150`，Task 7 scoped candidate digest `a9cc1235cf9254440cb347c0fa7d60964ecc9622a2e1bdb31eb33b569ff8bba3`）。

**Reviewer：** Codex，独立 Review。

**Method：** 对照 `tasks.md` Task 7、已批准的三个 UX 文档与当前未提交候选进行限定源码审查；CodeGraph 明确警告其索引属于主工作树、缺少本 topic 的未提交符号，因此未把图谱结果作为证据，改用 topic worktree 的直接源码。发现决定性契约违背后按项目规则停止 broad verification，并复用已绑定本 ContentState 的隔离 Xcode 构建和 Shared/App/Widget 测试证据。

**Scope：** Task 7 的 menu-bar、Widget、Settings、App Group 投影、quota refresh caller、本地化与 CLI specification 候选；本轮重点走查 Widget 的三尺寸投影和 Settings 写入状态机。Task 1–6 的已交付 Go/domain/wire 行为不重开。

### 🔴 严重问题 — 必须修复

**DS-R1-F1 — 中等；`apps/macos/AgentDeckWidget/WidgetViews.swift:260`：大号 Quota Widget 把两个客户端横向排列，而批准契约要求上下等高两半。**

- **行为风险：** systemLarge 会把 Codex/Claude 压进左右窄列，既不满足原型的上下信息层级，也无法满足“各自居中、等高并填满卡片正文”的 Task 7 验收条件。
- **证据：** 当前 `QuotaWidgetView.body` 无条件使用 `HStack(alignment: .top, spacing: 10)`；`ux/widget-quota.md:261,299,323-329` 明确要求“两端上下等分”、等高且填满正文，并记录了能区分“仅等高”与“真正填满”的负控断言。现有 `WidgetLayoutContract.sections` 测试只比较字符串清单，没有验证真实 SwiftUI 轴向、等高或填充。
- **处置：** OPEN。

💡 **限定修复：** systemLarge 改为上下分块，并把剩余正文高度等分给两个客户端；单客户端时不保留空半区或 divider。添加针对真实 `QuotaWidgetView` 结构/渲染结果的回归断言，覆盖双端等高、填满以及单端不留空区。

**DS-R1-F2 — 中等；`apps/macos/AgentDeckWidget/WidgetViews.swift:330-337`：Quota Widget 页脚使用快照生成时间，而不是所显示额度的最旧观测时间。**

- **行为风险：** Widget 每 3–5 分钟刷新一次但额度探测间隔更长时，页脚会把旧额度误标为刚更新；用户无法判断数值实际陈旧程度，直接破坏额度表面的 freshness 语义。
- **证据：** `WidgetFooter.relativeTime` 只读取 `entry.snapshot.generatedAt`，且 `WidgetFrame` 没有接收额度窗口 `observedAt` 的入口；`ux/widget-quota.md:198-205` 明确要求页脚显示 quota read time，并取当前卡片所显示观测中的最旧值，而不是 Widget 刷新时钟。当前 Widget 测试未覆盖这两个时钟分离的场景。
- **处置：** OPEN。

💡 **限定修复：** 为 quota kind 从实际呈现的客户端/窗口推导最旧 `observedAt`，将该时刻交给 footer；无成功观测时显示契约规定的原因。增加“snapshot 刚生成但 quota 较旧”及多窗口取最旧观测的回归测试。

**DS-R1-F3 — 中等；`apps/macos/AgentDeckApp/QuotaSettingsController.swift:132-145,183-190`：设置写入进行中时，后续用户更改被静默丢弃。**

- **行为风险：** 用户快速切换 alerts、thresholds、reset notice、reading 或 interval 时，第二个 `Task` 在第一个 transport await 期间进入 `applySettings`，命中 `guard !isApplyingSettings else { return }` 后无排队、无合并、无错误反馈；界面随后又被首个响应覆盖，用户选择没有落入 core state。
- **证据：** 五个 setter 都以完整快照调用同一个 `applySettings`，而该方法以布尔 guard 丢弃重叠请求。`QuotaSettingsControllerTests.swift` 只覆盖串行 await 的单次写入；transport stub 没有 suspended response，因此无法反证并发交互。Settings 契约要求 core state 为权威且控件写入通过 `desktop quota-settings` 落盘，静默丢弃不满足该行为。
- **处置：** OPEN。

💡 **限定修复：** 用串行队列或 last-write-wins coalescing 保存 pending desired state，确保前一请求完成后提交最新完整设置；对 transport failure 保留可重试的用户意图与明确反馈。增加可暂停 transport 的两次重叠更改测试，断言最终 core/controller/preferences 三者一致。

### 🟡 建议改进 — 推荐

无；本轮记录的三个问题都属于必须关闭的在范围缺陷。

### 🟢 优点

- Task 7 已把 quota-first refresh、App Group subscription 投影、第五种 Widget、设置写入 transport、本地化和 focused tests 串接到同一候选，并为 hosted XCTest 增加真实 HOME 的 fail-closed 隔离保护。
- 手动原生验收豁免被明确记录为 waived，而不是伪装成 performed；绑定本 ContentState 的隔离 Xcode build/test 证据仍可复用。
- small Widget 的最高 used share 选择已集中到 `WidgetSurfaceModel.quotaWindows`，并有针对性回归测试。

### 📝 总结

评审对象是 Task 7 `desktop-surfaces` 的精确未提交候选 `cfcc395 + a9cc1235cf92`。隔离构建与既有测试均为通过证据，但它们没有覆盖大号 Widget 的真实轴向/等分布局、quota observation clock 与 snapshot clock 的分离，或设置 transport await 期间的重叠输入。三个中等缺陷均在本变更内且仍 OPEN，因此 Round 1 为 FAIL；完成门禁为 FAILED，Task 保持未完成并返回修复。

**Evidence：**

- CEv1 既有 `verification-r1`：隔离 `build-for-testing`、Shared/App/Widget 相关套件、最终 Widget 22/22 与 focused quota-refresh command test 均通过，绑定上述 ContentState；不因阶段变化重跑。
- 源码决定性复现：`WidgetViews.swift:260` 的 `HStack` 对照 `ux/widget-quota.md:261,299,323-329`；`WidgetViews.swift:330-337` 的 `generatedAt` 对照 `ux/widget-quota.md:198-205`；`QuotaSettingsController.swift:184` 的重叠请求早退与现有串行测试集。
- `git diff --check`：通过。
- Task 7 operator waiver：2026-09-14 对本轮所有 Task 7 native manual-acceptance 行显式豁免，未将任何一项记为 performed。

**完成门禁：** FAILED。`surface-contract` 与 `review` 两项在该 Target ContentState 上由本轮反证失败；`verification` 与 `manual-acceptance` 的既有 pass 证据仍适用且不改写。

## Round 2 — 2026-09-14

## 📋 Desktop Surfaces 修复复评

📊 总体评分：7/10

✅ 复评结论：FAIL

**Reviewed state：** `urn:agent-deck:content-state:subscription-quota:desktop-surfaces:cfcc395:6b307372d62b`（HEAD `cfcc3952003778d66ef70d50ae2db91b36812150`，Task 7 scoped candidate digest `6b307372d62b322b99d511043d9f97f2eb8f1818328d4b58a3b3669b2b858319`）。

**Reviewer：** Codex，独立 Re-review。

**Method：** 针对 Round 1 的三项 finding 逐项复核当前源码和新增测试；复用绑定本 ContentState 的隔离 Xcode build-for-testing、App/Widget 回归套件与 suspended-transport 测试证据。CodeGraph 的 worktree 绑定限制未改变，因此继续使用 topic worktree 的直接源码。确认一项 finding 仍开后停止更广验证。

**Scope：** `DS-R1-F1`、`DS-R1-F2`、`DS-R1-F3` 的修复处置及其回归保护；不重开 Task 7 其他已评审路径或 Task 1–6。

### 🔴 严重问题 — 必须修复

**DS-R1-F1 — 中等；`apps/macos/AgentDeckWidget/WidgetViews.swift:273`：大号 Widget 已改为纵向等高槽位，但每个客户端块仍固定顶端对齐，没有按批准契约在各自半区居中。**

- **处置：** STILL OPEN；纵向轴、双端等高槽和单端不留空区已修复，但“各自居中”这一原 finding 的组成条件未关闭。
- **行为风险：** 双端内容会聚集在每个半区顶部，留下不对称空白；这正是 UX 文档要求等高并各自居中所避免的错误构图。
- **证据：** `QuotaWidgetView` 对双端使用 `VStack`，但每个 `clientBlock` 明确 `.frame(maxHeight: .infinity, alignment: .top)`。`ux/widget-quota.md:261,299` 要求“两端上下等分，各自居中”。新增 `testLargeQuotaWidgetUsesTwoVerticalEqualHeightSlotsAndSingleClientUsesNoEmptySlot` 仅断言 `QuotaWidgetLayoutContract(axis: .vertical, equalHeightSlots: 2)`，没有渲染或结构断言能覆盖 `.top` 对齐，因而不能关闭该条件。

💡 **限定修复：** 双客户端 large 布局把每个 `clientBlock` 在其等高槽内垂直居中，同时保留单客户端不创建空半区；增加能在 `.top` 回归时失败的真实布局/渲染断言，而不是只测试与 View 对齐无关的枚举值。

### 🟡 建议改进 — 推荐

无；唯一未关闭项仍属于必须修复的原 finding。

### 🟢 优点

- **DS-R1-F2 → CLOSED。** `WidgetSurfaceModel.quotaFooterObservedAt(family:)` 从实际呈现的客户端与窗口集合中选择最旧有效 `observedAt`，quota 的 aging qualifier 与 footer 都使用该时钟；新增测试明确构造 fresh snapshot / older quota，并断言选中两端中更旧的观测。
- **DS-R1-F3 → CLOSED。** Settings 完整 desired state 现在先乐观 stage，再由 `pendingSettings` 执行 last-write-wins 合并；旧响应不会覆盖更新的 pending intent，失败 desired state 会留给下一次操作重试。可暂停 transport 测试覆盖了重叠 alerts/thresholds 写入和失败后 reset-notice 重试。
- **DS-R1-F1 部分修复有效。** systemLarge 已从横向 `HStack` 改为纵向结构，双端使用两个弹性槽，单端不创建空槽。

### 📝 总结

处置矩阵为：`DS-R1-F1` STILL OPEN，`DS-R1-F2` CLOSED，`DS-R1-F3` CLOSED。复评对象是新的精确候选 `cfcc395 + 6b307372d62b`。绑定该状态的隔离构建与回归测试均通过，但 F1 新测试只证明布局枚举为纵向双槽，没有证明真实 SwiftUI 客户端块在各自半区居中；当前源码反而明确使用 `.top`。因此 Round 2 仍为 FAIL，残余不确定性仅限修复后的真实 WidgetKit 几何表现，而 Task 7 的原生验收已由 operator 显式豁免。

**Evidence：**

- CEv1 `verification-r2`：隔离 build-for-testing、App/Widget 回归套件、最终 Widget 套件以及 F3 suspended-transport overlap/retry 测试均通过，绑定本轮 Target ContentState；不重复运行。
- `WidgetViews.swift:267-277`：纵向分支已存在，但 `clientBlock` 的等高 frame 使用 `alignment: .top`。
- `WidgetPresentationTests.swift:63-81`：只验证 `QuotaWidgetLayoutContract` 的 axis/slot count 与单端槽数，不观察 View 的实际 alignment。
- `WidgetDomain.swift:111-124,165-181` 与 `WidgetPresentationTests.swift:83-97`：F2 的 quota 时钟和 older-of-displayed-clients 断言。
- `QuotaSettingsController.swift:183-225` 与 `QuotaSettingsControllerTests.swift:181-235`：F3 的 pending/coalescing/retry 实现和可暂停 transport 回归。

**完成门禁：** FAILED。`review` 与 `surface-contract` 在本轮 Target ContentState 上仍有适用 fail 反证；`verification` 与 `manual-acceptance` 的 pass 证据仍适用。

## Round 3 — 2026-09-14

## 📋 Desktop Surfaces 最终修复复评

📊 总体评分：10/10

✅ 复评结论：PASS

**Reviewed state：** `urn:agent-deck:content-state:subscription-quota:desktop-surfaces:cfcc395:745dfd8f33c8`（HEAD `cfcc3952003778d66ef70d50ae2db91b36812150`，Task 7 scoped candidate digest `745dfd8f33c87d5860b02f4e0a44609a873b7f486ff462e12253064683f18f25`）。

**Reviewer：** Codex，独立 Re-review。

**Method：** 复核 Round 2 唯一仍开放的 `DS-R1-F1`，直接检查生产 SwiftUI 对齐结构与新增的真实 View 几何测试；复用绑定本 ContentState 的隔离 build-for-testing、focused geometry test 和完整 Widget suite 证据。`DS-R1-F2`、`DS-R1-F3` 的修复路径未改变，沿用 Round 2 已关闭处置及其精确状态验证链。

**Scope：** `DS-R1-F1` 的剩余“等高半区内居中”条件及防回归测试；同时确认 Round 1 三项 finding 的最终处置矩阵完整。Task 7 其他已评审路径和 Task 1–6 不重开。

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **DS-R1-F1 → CLOSED。** `QuotaWidgetView` 的双客户端 large 分支在每个弹性纵向槽内使用 `ZStack(alignment: .center)`，保留两槽等高；单客户端仍走单槽分支，不创建空半区。
- 新增测试不再只验证布局枚举：它渲染真实 `AgentDeckWidgetView`，通过 `QuotaWidgetGeometryPreferenceKey` 捕获两端 slot/content frame，断言槽位等高、内容高度小于槽位且 content/slot 的 `midY` 在容差内相等；`.top` 回归会直接失败。
- **DS-R1-F2 保持 CLOSED。** quota footer/aging 继续使用实际显示窗口中最旧的额度观测时间。
- **DS-R1-F3 保持 CLOSED。** Settings 重叠写入继续采用 last-write-wins 合并，并保留失败意图供下一次操作重试。

### 📝 总结

最终处置矩阵为：`DS-R1-F1` CLOSED，`DS-R1-F2` CLOSED，`DS-R1-F3` CLOSED。复评对象是精确候选 `cfcc395 + 745dfd8f33c8`。生产布局、真实 View 几何断言和绑定该状态的隔离 Widget 验证共同关闭了上一轮唯一残留条件；本轮未发现新增 finding，因此 Round 3 为 PASS。Task 7 原生人工验收仍按 operator 明确豁免处理，没有任何一项被表示为 performed。

**Evidence：**

- `WidgetViews.swift:267-284`：双客户端 large 使用纵向弹性槽与 `ZStack(alignment: .center)`；单客户端保持单块布局。
- `WidgetPresentationTests.swift:63-101`：渲染真实 View，捕获双端 slot/content frame，并断言等高、余量和 midpoint 居中。
- CEv1 `verification-r3`：隔离 build-for-testing、focused geometry test 与完整 Widget suite 全部通过，绑定本轮 Target ContentState；Round 2 未受影响的 App/settings、F2/F3 证据被明确复用。
- CEv1 `manual-waiver-r3`：operator 的 Task 7 native manual-acceptance 豁免继续绑定该候选，无项目被记为 performed。

**完成门禁：** VERIFIED。`surface-contract`、`verification`、`manual-acceptance` 与本轮独立 `review` 四项 required criterion 均有绑定本 Target ContentState 的适用 pass 证据，无缺失、失效或未决影响。
