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
