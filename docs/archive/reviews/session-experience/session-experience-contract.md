---
status: historical
plan: session-experience
task: session-experience-contract
retired: 2026-08-06
---

# Review log — session-experience / session-experience-contract

## 📋 Round 1 评审 — session-experience / session-experience-contract

### 📊 总体评分：7/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`docs/specs/cli-manual.md:645`] usage warning 的 DTO 层级与实际 JSON 合同不一致

- 行为风险：手册声称 usage warning 与 partial state 位于标准 JSON envelope，但
  `session show --tokens` 实际把 pricing/attribution warning 放在
  `data.usage.warnings`；顶层 `warnings` / `partial` 表达命令级 warning 与部分结果。
  v0.5.0 desktop client 若按当前手册只读取顶层 envelope，会静默漏掉 usage warning，
  破坏本 task 要确认的 desktop-facing DTO 边界。
- 证据：`sessionShowPage` 通过 `json:"usage,omitempty"` 将完整
  `usage.SessionSummary` 嵌入 `data.usage`，其中 `SessionSummary.Warnings` 随 summary
  序列化；真实 sandbox 的 `session show --tokens --format json` 已显示顶层
  `warnings: []` / `partial: false`，同时 `data.usage.warnings` 包含
  `historical attribution`。当前手册第 645 行却将 usage warning 与 envelope 状态合并
  描述。

💡 有界修复：明确区分 `data.usage.warnings` 与顶层 envelope
`warnings` / `partial`；如需说明 invocation warning，也应写出其位于各
`data.invocations[].warnings`。不要改变 JSON 实现或扩展本 task 范围。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- 设计文档覆盖了 search `event_at`、scan/rebuild stderr progress、sectioned
  `session show`、interactive viewer 与 desktop DTO 消费边界，并明确本 task 不提升
  specification version。
- interactive viewer 的 text/TTY 限制、互斥 flags 与非 DTO 定位符合当前实现。
- Plan 保持 Dev `[x]`、Review `[ ]`，README 明确合同文档已开发且等待评审，没有提前
  宣称 6/6 完成。

### 📝 摘要

- Reviewed content identity：HEAD `9000c127438919987315f1610e8aea02b515a82e` 加
  scoped documentation SHA-256 manifest
  `e46c74e77435cb8f8a522a604f7dfc7b70ec0d2ffe30c73773589d00253b18d7`。
- Verdict rationale：desktop-facing DTO warning 层级是本 task 的核心合同，当前手册
  存在会导致消费者漏读 warning 的错误陈述，因此本轮为 `FAIL`。
- Evidence：当前文档 diff、session DTO/usage summary 生产代码、现有 session usage JSON
  测试结构，以及已完成的真实 sandbox JSON 证据。找到决定性阻断后未重复全仓 L2。
- Residual uncertainty：除该字段层级错误外，本轮未发现其他阻断；修复后需定向核对
  living design/manual 与 JSON DTO 字段位置，并执行文档格式与相关合同测试。
- 状态同步：Task 6 保持 Dev `[x]`、Review `[ ]`；`docs/README.md` 继续显示
  `5/6 reviewed` 且合同文档等待评审。

## 复评 Round 2 — 2026-08-06

### 📊 总体评分：6/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`cmd/agentdeck/session_viewer_terminal.go:47`] resize 可丢弃有效的 standalone Escape 退出键

- 处置：新发现（newly blocking）。
- 行为风险：`session show --interactive` 在 resize 与 standalone Escape 接近发生时可能
  无法退出，用户只能继续输入或取消进程；这违反本 task 要收口的只读 interactive
  viewer 键盘与 resize 合同。
- 证据：`readSessionViewerKey` 在 Escape 的 35ms lookahead 内收到 resize 时会返回有效
  `"escape"` 并同时设置 `resizedDuringRead=true`；`runSessionViewer` 在调用点先对该标记
  `continue`，未把有效 key 交给 `viewer.apply`。L2 全量命令
  `go test -mod=vendor ./...` 因
  `TestRunSessionViewerPTYExitResizeAndRestore` 超时失败；同一最小复现连续执行 20 次，
  20 次均在约 2.6 秒后报告 `viewer did not exit after standalone PTY Escape`，排除了偶发
  资源争用。

💡 有界修复：保证 Escape lookahead 已识别出的 `"escape"` 不会被同时到达的 resize
事件丢弃，并增加确定性的 resize/Escape 交错覆盖；不要改变其他 viewer 键位、分页或
DTO 行为。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- Round 1 的 DTO warning 层级 finding 已关闭：`docs/specs/cli-manual.md:645-647`
  现已分别写明 `data.usage.warnings`、`data.invocations[].warnings` 与顶层
  command-level `warnings` / `partial`。
- 修正文档与当前 `sessionShowPage`、`usage.SessionSummary`、
  `usage.SessionInvocation` 和标准 envelope 序列化边界一致。
- 定向测试
  `TestSessionShowTokensUsesEventTimeUsageAndInvocationPagination` 与
  `TestSessionUsageSummaryAndInvocationsUseStoredEventDeltasWithoutPrivateMetadata`
  均通过。

### 📝 摘要

- Finding disposition：Round 1 的 desktop DTO finding 已关闭；新增一个 interactive
  PTY exit/resize 阻断项，无仍开放或回归的旧 finding。
- Reviewed content identity：HEAD
  `9000c127438919987315f1610e8aea02b515a82e` 加 scoped documentation SHA-256
  manifest `f8322483eeae11aa655be085d90b875bda71f92cee6f338a17d605d2e07fbb25`。
- Verdict rationale：文档修复正确，但 Task 6 明确要求的 L2 contract state 出现稳定
  可复现的 interactive viewer 退出失败，因此复评仍为 `FAIL`。
- Residual uncertainty：根因已定位到有效 Escape 与 resize 标记的处理优先级；本轮
  评审权限不允许修改生产代码或测试，修复后必须重跑最小复现与原 L2 命令。
- 状态同步：Task 6 保持 Dev `[x]`、Review `[ ]`；父 Plan 保持进行中，README 继续
  显示 `5/6 reviewed`。

### 🛠 修复指令

修复 `runSessionViewer` / `readSessionViewerKey` 的 resize 与 standalone Escape 交错：
不得丢弃已识别的 `"escape"`；补充确定性的回归覆盖，然后运行该 PTY 最小复现、session
viewer 相关测试及 `go test -mod=vendor ./...`。保持本 task 的 JSON DTO、其他键位、
分页、文档版本和交付边界不变。

## 复评 Round 3 — 2026-08-06

### 📊 总体评分：10/10

### ✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- Round 1 的 desktop DTO warning 层级 finding 保持关闭；living design/manual 继续
  准确区分 usage、invocation 与 command-level warning/partial state。
- Round 2 的 PTY exit/resize finding 已关闭：`runSessionViewer` 只在 resize 且没有
  已识别 Escape 时 redraw，使有效的 standalone Escape 可以继续进入
  `viewer.apply`，同时保留普通 resize redraw 行为。
- 新增 `TestSessionViewerEscapeWinsOverResize` 固定该优先级；原
  `TestRunSessionViewerPTYExitResizeAndRestore` 仍从真实 PTY、signal、raw-mode 与
  terminal restore 路径验证端到端行为。

### 📝 摘要

- Finding disposition：Round 1 与 Round 2 的两个阻断 finding 均已关闭；无仍开放、
  回归或新增阻断项。
- Reviewed content identity：HEAD
  `9000c127438919987315f1610e8aea02b515a82e` 加 scoped candidate SHA-256 manifest
  `39555fde57efee0ac29796faceff6eeaf7f565be696d1f4c7fcbb0dd71a43eee`。
- Verification：上一轮 20/20 失败的 PTY 最小复现现为 20/20 通过；新增判定测试和
  standalone Escape 单测通过；`go test -mod=vendor ./...` 通过。
- Verdict rationale：所有记录 finding 均关闭，相关定向证据与 Task 6 要求的 L2
  contract state 均通过，因此复评为 `PASS`。
- Residual uncertainty：无已知阻断；本 task 不包含 release、desktop implementation、
  commit 或 push authority。
- 状态同步：Task 6 Dev/Review 均完成；六个 task 全部 Review PASS，plan 与 review
  记录按仓库生命周期转为 historical 并归档。
