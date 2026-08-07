---
status: active
plan: session-experience
task: interactive-session-viewer
---

# Review log — session-experience / interactive-session-viewer

## 📋 Round 1 评审 — session-experience / interactive-session-viewer

### 📊 总体评分：5/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`cmd/agentdeck/session_viewer_terminal.go:36`] 输入读取无法随取消退出，单独 Escape 会阻塞

- 行为风险：`readSessionViewerKey` 在读到 ESC 后同步等待下一个字节；真实 TTY 上单独
  按 Escape 不会产生 EOF，因此合同规定的 Escape 退出会挂起。输入 goroutine 也只在
  `ReadByte` 返回后检查 `done`；context 取消时主循环会恢复终端并返回，但 goroutine
  仍阻塞在 stdin，随后可能吞掉用户在已恢复终端输入的第一个字节才退出。
- 证据：`session_viewer_terminal.go:36-47` 的 `done` 只保护 channel send，不能中断
  `readSessionViewerKey`；`session_viewer_terminal.go:103-107` 对 ESC 立即执行第二次
  `ReadByte`。当前测试用 `strings.Reader`，EOF 会让 ESC 假装成功，无法代表保持打开的
  TTY。

💡 有界修复：使用可取消的终端输入读取，并为 ESC 与 escape sequence 的歧义设置短暂
等待/超时；退出前确保 reader goroutine 已停止或不再拥有 stdin。用 PTY 覆盖单独
Escape、context cancellation、Ctrl-C/正常退出以及恢复后输入不被吞掉。

#### [`cmd/agentdeck/session_viewer_terminal.go:56`] resize 和 renderer 完全忽略终端高度

- 行为风险：`term.GetSize` 的行数被丢弃，renderer 总是输出当前 page 的全部行、header
  和 footer。较矮终端会滚屏，选中行或帮助 footer 可能离开可见区域；SIGWINCH 只重新
  计算宽度，没有实现设计要求的 viewport calculation，也不能在高度变化时保持一个
  可见 selection。
- 证据：`session_viewer_terminal.go:56-60` 只传递 columns；
  `session_viewer.go:112-136` 遍历全部 `current.Lines`，没有 height、viewport start 或
  clipping。计划明确要求 resize 重算 layout，并把 viewport calculation 列为可测试
  组件。

💡 有界修复：把 columns 和 rows 一并传给 renderer，扣除 header/status/help 行后计算
可见 viewport，使 selection 始终位于窗口内；resize 只重算 viewport，不改变 section
page 或 selection。补充短高度和 SIGWINCH PTY 断言。

#### [`cmd/agentdeck/session_viewer_test.go:10`] 缺少计划要求的 PTY 和终端生命周期验收

- 行为风险：当前 green race 测试不能证明 TTY 检测、raw mode、cursor cleanup、
  cancellation、interrupt、resize 或 reader goroutine 生命周期。上述真实终端缺陷可以
  在全部现有测试通过时进入发布候选。
- 证据：仓库测试中没有 `runSessionViewer`、`MakeRaw`、`Restore` 或 SIGWINCH 的调用；
  `go test -mod=vendor -race ./cmd/agentdeck -run SessionViewer -count=1` 通过，但仅运行
  state/key/render 的非 PTY fixture。计划要求 targeted PTY acceptance，并因 terminal
  concurrency/signal handling 路由到 L3 race gate。

💡 有界修复：增加真实 PTY integration tests，至少覆盖 non-TTY 拒绝、初始 render、
section/page keys、单独 Escape、resize、取消/错误/正常退出后的 raw mode 与 cursor 恢复，
以及 goroutine 退出；修复后运行 targeted PTY、相关 race 和最终 L3 所需验证。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- `--interactive` 保持显式 opt-in，并与 JSON、分页和现有 `--activity/--tokens` 路径分离，
  普通 `session show` 行为未被默认改为 TUI。
- AgentDeck-owned state machine 没有引入新 TUI 依赖；四个 section 保存独立 page，只有
  section/page 变化时触发 lazy load。
- Documents、Activity、Tokens 继续复用已评审的 bounded/safe readers，没有扩大
  activity allowlist 或持久化 raw source content。

### 📝 摘要

- Reviewed content identity：HEAD
  `56c57dc701037f3336f5cb11899b7ce78d72ebbb`；scoped candidate fingerprint
  `776b7706251381fc4aaf648432b1317eb35dbcda73800380f8f99b47947127c4`。
- Verdict rationale：状态机和 lazy paging 方向正确，但 Escape/cancellation 输入生命周期、
  vertical viewport 与强制 PTY acceptance 均未满足，结论为 `FAIL`。
- Evidence：CodeGraph 调用路径、当前源码/测试检查，以及 targeted viewer race test。
  发现决定性阻断后未运行全仓 L3。
- Residual uncertainty：修复后仍需验证真实 PTY 下的 signal/resize/cleanup、CLI 非 TTY/JSON
  拒绝，以及最终 L3；当前纯单测不能替代这些证据。
- 状态同步：Dev `[x]`、Review `[ ]`；`docs/README.md` 保持 `4/6 done`。

### 🛠 修复指令

```text
根据评审修改：session-experience / interactive-session-viewer
```

只修复本轮三个 terminal/PTY 阻断：可取消且正确区分单独 Escape 的输入生命周期、按
终端高度计算的 viewport，以及真实 PTY/cleanup/resize integration coverage；保持已
通过的 lazy independent paging、safe reader、普通非交互输出和依赖边界不变。

## 📋 Round 2 复评 — session-experience / interactive-session-viewer

### 📊 总体评分：9/10

### ✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- 已关闭 `terminal-input-lifecycle`：`readSessionViewerByte` 使用短轮询检查
  context、resize 与输入就绪，不再遗留阻塞 reader goroutine；单独 Escape
  通过 35ms 歧义窗口可靠退出。
- 已关闭 `responsive-terminal-viewport`：渲染器同时接收终端宽高，按保留的
  header/status/help 行计算可见窗口，并在 resize 后重新渲染且保持选中行可见。
- 已关闭 `pty-terminal-lifecycle-verification`：Darwin PTY 验收覆盖独立 Escape、
  SIGWINCH resize、取消、退出以及 raw mode 恢复，定向 race 运行通过。

### 📝 总结

Round 1 的三项阻断均已关闭，没有仍开放、回归或新增的阻断 finding。复评内容身份为
`HEAD 56c57dc` 加代码/测试 SHA-256 manifest
`cc6c4bc7abcbb6e0c485324772c0a8510640c72a6d5ec57b78ed04baf92b2cbb`。
验证证据为
`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./cmd/agentdeck -run=SessionViewer -count=1`
以及
`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`，两者均通过。
PTY 验收受当前 macOS-first 支持边界约束，Darwin 之外未在本轮执行真实 PTY；这不影响
当前任务合同。综合判定 `PASS`。

## 📋 Round 3 评审 — session-experience / interactive-session-viewer

### 📊 总体评分：7/10

### ✅ 结论：FAIL

### 🔴 严重问题（必须修复）

#### [`cmd/agentdeck/session_viewer_data.go:60`] TOKENS 页面读取了未标准化的汇总键，真实会话显示为 0/0

- 行为风险：`session show --tokens` 已返回 `input_tokens=120`、`output_tokens=30`，但交互 TOKENS 页面读取 `summary.Tokens["input"]` 和 `["output"]`。这两个键在当前标准化汇总中不存在，导致用户在交互模式看到 `input: 0 · output: 0`，与同一会话的非交互输出相矛盾。
- 证据：真实 Codex JSONL sandbox 验收显示 JSON `session show --tokens` 为 120/30；真实 PTY 的 TOKENS 页面为 0/0。当前源码第 60 行直接访问旧键。对 `newSessionViewerLoad` / `viewerTokens` / `input: ` / `SessionUsageSummary` 的现有 `cmd/agentdeck/*test.go` 检索未发现覆盖这个适配路径的回归断言。

💡 修复建议：仅将交互汇总改为读取与 `session show --tokens` 相同的标准化 `input_tokens` 和 `output_tokens` 键；增加一个由真实标准化 summary 驱动的 `newSessionViewerLoad(..., viewerTokens, ...)` 测试，断言首行显示非零 input/output totals，且缺失 token 数据仍保持现有的 partial/warning 行为。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- Round 2 关闭的终端输入、viewport、resize 和 raw-mode cleanup 保护仍与本轮 finding 无关；本轮未发现这些行为回退的证据。
- TOKENS 页面继续通过既有 bounded usage service 获取数据，修复可以局限在展示层和其定向测试。

### 📝 摘要

- Reviewed content identity：HEAD `319e665e033646aa897ec9d9532c3e10db78188c`（`feat: add interactive session viewer`），工作树在评审开始时干净。
- Verdict rationale：真实场景已经证实 TOKENS 汇总与 `session show --tokens` 的规范化 totals 不一致；这是 task 明确承诺的 TOKENS summary 行为，故本轮为 `FAIL`。
- Evidence：当前源码、真实 Codex JSONL sandbox 的 JSON 与 PTY 验收，以及 test-path 检索。该 finding 已有决定性 reproducer，未重复无关的全仓 L3 验证。
- Residual uncertainty：修复后仍须在同一真实 JSONL/PTY 路径确认 120/30（或等价 fixture totals）一致，并执行新增定向测试；Round 2 的终端生命周期证据可在未改动相关文件时复用。
- 状态同步：Dev `[x]`、Review `[ ]`；`docs/README.md` 回退为 `4/6 done`。

### 🛠 修复指令

```text
根据评审修改：session-experience / interactive-session-viewer
```

只修复 TOKENS 汇总适配：在 `cmd/agentdeck/session_viewer_data.go` 中改用标准化 `input_tokens` / `output_tokens` totals，并为 `newSessionViewerLoad` 的 `viewerTokens` 分支添加回归覆盖。保持 `--interactive` 的 TTY/JSON 拒绝、terminal/PTY 生命周期、分页、usage reader、JSON 输出契约和依赖边界不变。完成后以真实 Codex JSONL sandbox 同时核对 `session show --tokens` 与 PTY TOKENS 页面，并运行受影响的 Go 测试；不要提交、推送或变更其他 task。

## 📋 Round 4 复评 — session-experience / interactive-session-viewer

### 📊 总体评分：9/10

### ✅ 结论：PASS

### 🔴 严重问题（必须修复）

无。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- 已关闭 Round 3 `token-summary-contract`：TOKENS 汇总改用标准化
  `input_tokens` / `output_tokens`，与非交互 `session show --tokens` 共用同一
  `SessionUsageSummary` 合同。
- 新增回归测试通过真实临时 store、bundled price catalog 和 usage service 注入
  `120/30`，直接断言 `viewerTokens` 首行显示 `input: 120 · output: 30`。
- 编译后的候选二进制在隔离 HOME/state-dir 中扫描真实 Codex JSONL 协议形状，JSON
  totals 与真实 PTY TOKENS 页面均为 `120/30`；独立 Escape 以 0 退出并恢复光标。

### 📝 摘要

- Finding disposition：Round 3 唯一阻断已关闭；没有仍开放、回归或新增的阻断 finding。
- Reviewed content identity：HEAD `319e665e033646aa897ec9d9532c3e10db78188c` 加代码/测试
  SHA-256 manifest `9b195ba8a62ba41c86287a09cdcf75127bb3e0d8d7d146e61ce051d3084f2088`。
- Evidence：`TestSessionViewerTokensUseNormalizedSummaryTotals` 定向测试通过；修复阶段的
  `SessionViewer` 普通与 race 定向测试继续有效；本轮编译后二进制 sandbox 的 session/usage
  scan、JSON `session show --tokens`、真实 PTY TOKENS 与独立 Escape 验收均通过。
- Residual uncertainty：真实场景使用完全隔离的合成 Codex 数据，未读取真实用户会话，且
  未另建 Claude fixture；两客户端经过同一标准化 summary/viewer 适配路径，因此不构成当前
  task 的阻断。
- Verdict rationale：用户可见的汇总与明细现已一致，Round 3 reproducer 无法复现；综合判定
  `PASS`。
