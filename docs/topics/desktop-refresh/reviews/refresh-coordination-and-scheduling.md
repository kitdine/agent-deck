---
status: active
topic: desktop-refresh
subject: refresh-coordination-and-scheduling
---

# Refresh Coordination and Scheduling Review

## Round 1 — 2026-09-20

## 📋 Refresh coordination and scheduling 实现评审

📊 总体评分：6/10

✅ 评审结论：FAIL

### 🔴 严重问题 — 必须修复

**RCS-R1-F1 — 高：periodic evaluator 在检查 full deadline 前等待 quota transaction，破坏 ≤30 秒调度延迟合同。**

位置：`apps/macos/AgentDeckApp/AgentDeckApp.swift:265`–`:281`；关联
`apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift:459`、`:1288`–`:1305`；
测试缺口位于 `apps/macos/AgentDeckTests/DesktopRefreshSchedulerTests.swift:5`–`:66`。

- 行为风险：每轮 loop 先 sleep 30 秒，再 `await requestQuotaRefresh`，quota 完成后
  才调用 `refreshScheduler.evaluate(.periodic)`。quota probe 自身允许 40 秒超时，
  随后还可能执行 alert delivery/ack 和 subscription fetch，因此 periodic
  evaluator 的相邻调用间隔可以超过 70 秒。若 manual/provider/startup full attempt
  在前一次 evaluator 之后完成，其 `completion + 60s` deadline 可能刚好落在这个
  长窗口内，自动 full refresh 会晚 30–70 秒以上才被请求，违反 reviewed
  “bounded 30-second evaluator / application-controlled delay ≤30s”，也让 quota
  transaction 实际阻塞独立的 full cadence。
- 证据：`startPeriodicRefresh` 的单个 Task 在 `evaluate(.periodic)` 前直接 await
  coordinator quota 请求；源码没有并行 task、deadline-first evaluation 或独立
  evaluator。`EmbeddedHelperRunner.quotaRefreshTimeout` 固定为 40 秒，而 fetch
  是同一 owner sequence 的后续 await。现有 scheduler tests 只用 fake clock
  直接调用 `evaluate`，没有把慢/悬挂 quota 操作接入 App loop，因此十轮 no-replay
  PASS 不能证明真实 wiring 的 30 秒上界。

💡 修复范围：让 full-deadline evaluation 不等待 quota transaction；可在每个
30 秒 tick 先同步 evaluate，再通过独立、仍受 coordinator single-flight 约束的
任务请求 quota，或拆成两个生命周期受控的循环。增加确定性 App/scheduler wiring
回归，悬挂 quota 超过 30 秒时仍证明 due full request 在界内只触发一次；同时
验证取消/termination 不遗留无主任务。不要通过缩短业务 timeout 或放宽 30 秒
合同掩盖串行等待。

Disposition：OPEN，交回同一任务
`ad-dr-refresh-coordination-and-scheduling-dev` 修复。

### 🟡 建议改进 — 推荐

无。决定性阻塞 finding 成立后停止扩大广泛验证。

### 🟢 优点

- Coordinator 将 full/quota lanes、follow-up/pending state 和 Widget publication
  state 明确分离，现有确定性测试覆盖主要 join、priority 与 superseding 路径。
- one-minute Go wire hint、periodic 默认关闭、reviewed 双语 Settings copy 和
  monotonic scheduler 的局部逻辑均有聚焦自动化证据。
- Task 1 publisher 被复用，没有重新引入 direct store/reload publication path。

### 📝 总结

- Reviewer：Codex 主代理；Method：代码与测试正式评审，先用 workspace-bound
  CodeGraph 定位候选，再核对 scheduler/App wiring、coordinator 仲裁、Task 2
  合同及已有 CEv1/测试证据；未委派，也不声称完全冷上下文。
- Scope：Task 2 的 22 个 source/test/fixture/project/status 路径；生产代码、
  测试和配置保持只读。Task 1 已交付基础与 Task 3 Widget loading 不在本轮。
- Reviewed state：workspace `agent-deck.desktop-refresh`，branch
  `feature/desktop-refresh`；HEAD
  `e549d99b2a9972e197ecd2af19e0adb2e6341a5f`；22 路径有序
  `path<TAB>git-blob` manifest SHA-256
  `9692455abcf5c0e915f6b1547dcc38d32d3976ce152bdfa1915d2dc8107e0161`；
  ContentState
  `urn:ce:agent-deck:content-state:desktop-refresh:refresh-coordination-and-scheduling:implement:9692455abcf5c0e915f6b1547dcc38d32d3976ce152bdfa1915d2dc8107e0161`。
- Evidence：实现交接的 `internal/desktop` package/race、AgentDeckShared 74/74、
  hosted App copy/preferences 14/14、universal App build 与格式检查均绑定同一候选；
  本轮复用未变化证据。RCS-R1-F1 是这些测试未覆盖的 App-level slow-quota
  wiring 反例；有决定性源码路径后未重复全套件。
- Completion gate：FAILED。`one-minute-completion-scheduler` 与
  `l3-coordination-verification` 被本轮 exact-state failure evidence 否定；
  其余四项历史 PASS 保留但不构成 Task gate PASS。
- Residual uncertainty：阻塞 finding 后未完成其他 phase-crossing 的全面复核，
  也未运行 native timing；修复最终态应重跑受影响的 App/scheduler wiring 与
  相关 Shared regression。

### 下一步指令

修复：desktop-refresh / reviews/refresh-coordination-and-scheduling.md / RCS-R1-F1

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh

### Repair candidate — 2026-09-20

- `RCS-R1-F1 -> repaired in candidate.` `DesktopRefreshPeriodicDriver.tick`
  now evaluates the full deadline synchronously before starting quota work.
  Quota runs in one driver-owned child task, repeated ticks do not start a
  second request, and `AgentDeckApplicationDelegate` cancels that owned task at
  termination. The coordinator's existing quota single-flight remains the
  product-side owner boundary.
- The deterministic regression suspends quota past the due full deadline. The
  pre-fix serial driver failed because no full trigger existed while quota was
  suspended; the repaired driver requests exactly one periodic full across
  repeated ticks, runs exactly one quota owner, retains ownership after cancel,
  and clears the task after the operation exits. No sleep, shortened business
  timeout, or relaxed 30-second contract is used.
- Repair candidate：HEAD
  `e549d99b2a9972e197ecd2af19e0adb2e6341a5f`；Round 1 相同 22 路径有序
  `path<TAB>git-blob` manifest SHA-256
  `80a7f48a268058990ae1d78e898df948bdb6ef6e8c0af8f1a7b03fc578668abc`。
- Verification：同一 reproducer 修复前 1/1 FAIL、修复后 1/1 PASS；完整
  AgentDeckShared SwiftPM suite 75/75 PASS；官方 macOS App build PASS；
  `scripts/check-topic-docs.sh`、JSON、whitespace 与 `git diff --check` PASS。
  本 finding 未改变 Go producer/fixtures，当前 Task 2 Go package/race 证据可复用。

Round 1 结论仍为 FAIL，Task Review 仍未勾选；以上只声明返修候选就绪，
`RCS-R1-F1` 是否 CLOSED 与新候选 completion gate 由独立复评决定。

## Round 2 — 2026-09-20

## 📋 Refresh coordination and scheduling 修复复评

📊 总体评分：9/10

✅ 复评结论：PASS

### 🔴 严重问题 — 必须修复

无。

### 🟡 建议改进 — 推荐

无。

### 🟢 优点

- **RCS-R1-F1 → CLOSED。** `DesktopRefreshPeriodicDriver.tick` 在启动 quota
  child task 前同步执行 full deadline evaluation；慢/悬挂 quota 不再延迟
  30 秒 evaluator。
- Driver 持有至多一个 quota task，重复 tick 只重复安全的 scheduler evaluation，
  不重复 quota side effects；termination cancellation 后仍保留 ownership，直到
  operation 实际退出并清空 task。
- 回归测试用 continuation 确定性悬挂 quota，验证 due full 只触发一次、quota
  只启动一次、取消时 ownership 不丢失，无 sleep 或业务 timeout 改写。

### 📝 总结

- Finding disposition：`RCS-R1-F1` CLOSED；无 still-open、regressed 或新增
  finding。Round 1 的唯一 finding 已完整处置，因此本轮 PASS。
- Reviewer：Codex 主代理；Method：finding-scoped 源码复评与独立聚焦 Swift
  运行，核对 driver、App lifecycle wiring、确定性 regression 及同内容态的完整
  Shared/App build 证据；未修改生产代码、测试或配置，未委派。
- Scope：`DesktopRefreshPeriodicDriver`、`AgentDeckApplicationDelegate` 的
  tick/termination wiring、scheduler regression，以及 Task 2 相同 22 路径内容态；
  未进入 Task 3/4 实现。
- Reviewed state：workspace `agent-deck.desktop-refresh`，branch
  `feature/desktop-refresh`；HEAD
  `e549d99b2a9972e197ecd2af19e0adb2e6341a5f`。修复候选代码态为
  `80a7f48a268058990ae1d78e898df948bdb6ef6e8c0af8f1a7b03fc578668abc`；
  Task Review 同步后的最终 22 路径有序 `path<TAB>git-blob` manifest SHA-256 为
  `0cff465a25bc7f9f92350aa8c8af67ce583d5461d0c6972f36408b6552725208`。
- Evidence：`swift test --package-path apps/macos --filter DesktopRefreshSchedulerTests`
  在最终代码态执行 4 项、0 failure；slow-quota regression PASS。修复交接中同一
  `80a7f48a…` 代码态的完整 AgentDeckShared 75/75 与官方 macOS App build 可复用；
  Go producer/fixture 未受 finding 修复影响，原 package/race 证据保持适用。
- Completion gate：VERIFIED。六项 required criteria 均有最终 ContentState 的
  适用 PASS evidence，missing/invalidated/unresolved 为空；Round 1 FAIL 仍只
  绑定旧 `9692455a…` ContentState。
- Residual uncertainty：native timing/installed acceptance 仍由 Task 5 承担；
  本 PASS 不把该后续边界声明为已完成。

Task checkpoint：`ad-dr-refresh-coordination-and-scheduling-dev`；
content_state=`0cff465a25bc7f9f92350aa8c8af67ce583d5461d0c6972f36408b6552725208`；
gate=`VERIFIED`。

提交建议：授权后按 Task 2 边界提交 22 个 source/test/fixture/project/status 路径
及本评审记录；排除其他 task、全局 `docs/status.md` 和派生文件。

推送建议：仅在提交对象完整消息、贡献归属、Codex trailer、SSH 签名与分支范围
核验通过后，另行授权推送 `feature/desktop-refresh` 到确认的远端同名分支；
不推送 `main`、不合并。

### 下一步指令

开发：desktop-refresh / widget-loading-and-timeline

WORKFLOW_WORKSPACE: agent-deck.desktop-refresh
