---
status: active
plan: usage-report-presentation
task: usage-interactive-viewer
---


## Round 5 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD
  `7b1029bf30c6a009f8204a3fc71cdb6590d84259`.
- Reviewer: Codex.
- Current scoped file SHA-256:
  - `cmd/agentdeck/main.go`: `7015ba00b37a8f18ceef64b08a5bafd6d2a9ba83c06d0b0a497fc1bb2be0f4c3`
  - `cmd/agentdeck/usage_stats_viewer.go`: `5560c40b52716c10d157ab46a401f45ad210e159b0ee427fb9f4a3b911b0e892`
  - `cmd/agentdeck/usage_stats_viewer_test.go`: `f7efe11bbde8cf47be04fcaa2df0054646322af32fc838d9b702887ca8a244e7`
  - `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`: `df8e8252940598e09098f4c3960a1861324d9c78538da9c2ca3114e64068d202`
  - `docs/README.md` before this review update: `46a772d7ec9c2f6c59da8189a3c706608c198082effb3e2420b47d32a335add4`
  - `docs/plans/usage-report-presentation.md` before this review update: `cd365318f616400b41d17adb6999d33d440b26a7eae0f0802c2e9541a0af318a`
  - `docs/specs/2026-08-06-terminal-rendering-design.md`: `12462e471f9dceb2ae46498a6345c0d4beb8719ab6aabe1a71c120977d25e3a4`
  - `docs/specs/2026-08-07-usage-interactive-viewer-design.md`: `89057fbe8150a04911a261b42e510c39d7b0f51a227216a6f30ca4cff09a9764`
- Evidence method: CodeGraph call-path inspection followed by bounded exact source
  reads for the located CLI, state, renderer, parser, and terminal-lifecycle
  boundaries.
- Exact-state reuse result: NOT REUSABLE. Round 4 recorded
  `usage_stats_viewer_test.go` as `50d15ba3...` and
  `usage_stats_viewer_pty_darwin_test.go` as `1b1db6a8...`; the current files are
  `f7efe11b...` and `df8e8252...`. The Round 4 focused, L2, race, and
  compiled-binary acceptance results therefore do not prove this candidate.
- Broader verification was not rerun after the decisive blocking findings,
  following the review verification policy.
- CEv1 evidence was upserted for this repository and exact target state; the
  required-criteria query returns three current failures, one failed L3 review
  criterion, and two criteria without current-state evidence. Gate: FAILED.
- Verdict: FAIL; task status returns to REOPEN.

### 📋 复评报告 — usage-interactive-viewer

📊 总体评分: 5/10

❌ 结论: FAIL

#### 🔴 严重问题（必须修复）

1. [`cmd/agentdeck/usage_stats_viewer.go:326`] arbitrary report labels reach the
   full-screen terminal without control sanitization, and `statsTitle` corrupts
   non-ASCII first characters.

   - 行为风险：Codex/Claude session metadata supplies model names as strings;
     `internal/usage/usage.go:1210` assigns `payload["model"]` directly to the
     event. A model or provider name containing `ESC`/C0 controls can therefore
     emit terminal control sequences inside the alternate screen. For ordinary
     CJK or emoji labels, `cmd/agentdeck/usage_stats_text.go:1135` uppercases
     `value[:1]`, splitting the first UTF-8 code point and corrupting the row
     identity. The current CJK test only searches the complete frame, so the raw
     label in the detail title can make it pass while the list row remains
     damaged.
   - 合同证据：the approved design requires visible-cell-safe CJK, emoji,
     combining marks, ANSI, and controls, and explicitly requires embedded
     ANSI/control removal before fitting.
   - 有界修复：sanitize label/detail fields at the usage-viewer report-adapter
     boundary and stop applying byte-based title casing to arbitrary identity
     values. Add exact row assertions for CJK, emoji, combining marks, CSI/OSC,
     and C0 input; prove no untrusted escape/control byte reaches the frame while
     viewer-owned screen-control sequences remain intact.

2. [`cmd/agentdeck/usage_stats_viewer.go:315`] the declared section-local
   viewport is write-only rather than state owned by the viewer.

   - 行为风险：`viewports[section]` is reset and assigned, but never read. Every
     render recenters from `selected` and geometry, so moving selection within an
     already visible window can shift the list before scrolling is required;
     the viewport context is derived anew instead of retained per section.
   - 合同证据：the approved design requires section-local page, selection, and
     viewport state, plus resize adjustment only as required to keep selection
     visible. The verification matrix requires pure viewport-retention tests;
     none references `viewports`.
   - 有界修复：make the stored viewport authoritative, clamp it to the current
     page and geometry, and shift it only when the selected row leaves the
     visible window. Add pure tests for selection movement inside a viewport,
     section round trips, and shrink/recover resize behavior.

3. [`cmd/agentdeck/usage_stats_viewer.go:170`] Models, Clients, and Providers
   omit the approved cache fields from selected-record detail.

   - 行为风险：`usage.StatsDimension` already carries cached-read, cache-write,
     logical-input, and cache-hit-rate values, but the viewer exposes only
     tokens, cost, sessions, and coverage. Users cannot perform the promised
     per-dimension cache comparison in interactive mode.
   - 合同证据：the approved section contract requires selected dimension detail
     to include cache and pricing completeness where available.
   - 有界修复：add labeled cache accounting fields only when meaningful, retain
     the existing height-degradation rules, and cover Models, Clients, and
     Providers with selection-driven detail tests.

#### 🟡 建议改进（推荐）

无。

#### 🟢 本轮确认仍成立

- Interactive format/TTY/geometry preflight remains before store open and scan.
- The renderer still bounds the transient below-48x10 frame without mutating
  page or selection.
- The CLI route remains isolated from ordinary text and JSON rendering, and no
  intentional session-viewer behavior change is present in the scoped diff.
- Round 1–3 lifecycle findings are not reopened by source inspection; their
  final-state verification evidence must nevertheless be regenerated because
  the current test content no longer matches Round 4.

### 📝 总结

Round 5 found three previously missed contract defects and invalidated the Round
4 exact-state verification reuse. Task 5 returns to REOPEN/FAIL; the plan Review
checkbox is cleared and the documentation index returns to 4/6 reviewed. After
the bounded fixes, run the focused viewer/state/PTY tests, vendored L2 suite,
related race gate, compiled-binary isolated-HOME text/JSON/interactive acceptance
covering geometry, no-color, paging, resize and cleanup, then `git diff --check`
on the final content state. Do not commit or push.
# Review log — usage-report-presentation / usage-interactive-viewer

## Round 1 — 2026-08-09

- Reviewed state: uncommitted worktree on HEAD
  `7b1029bf30c6a009f8204a3fc71cdb6590d84259`.
- Reviewer: Codex.
- Scoped content-manifest SHA-256:
  `607d0edee6a73b66577ade6a13150c709e4705012928fd3f08a33b7abc5b852a`.
- Scoped file SHA-256:
  - `cmd/agentdeck/main.go`:
    `00bf6d6bd9a4396755a85e1e9a1d9759f6426232f3deaf4a62b5fe4385d2b4d7`
  - `cmd/agentdeck/usage_stats_viewer.go`:
    `9182bc5f2debb65222b35f8f2bcca282521bacd112d50702d34b9d532e26248e`
  - `cmd/agentdeck/usage_stats_viewer_test.go`:
    `2166da883ba006d75a9cb68c537d6f5744f3aa6f26ceab68ea036782cccedec9`
  - `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`:
    `add88134c2985a7f6ff918c56a6a75873bfc8b989c4b0b57e4bfc859a519f44a`
  - `docs/README.md`:
    `55c3a2c34e5aef84e6745035b1912bcac97c86455435df05d48f40fdefc72217`
  - `docs/plans/usage-report-presentation.md`:
    `1114e13b9e0f031333100a5b89962bd0369d7b6c3e193b3476e63996b746a250`
  - `docs/specs/2026-08-06-terminal-rendering-design.md`:
    `12462e471f9dceb2ae46498a6345c0d4beb8719ab6aabe1a71c120977d25e3a4`
  - `docs/specs/2026-08-07-usage-interactive-viewer-design.md`:
    `89057fbe8150a04911a261b42e510c39d7b0f51a227216a6f30ca4cff09a9764`
- Scope: Task 5 CLI routing, terminal eligibility, state and pagination,
  responsive rendering, no-color behavior, terminal lifecycle, PTY protection,
  and the approved Task 5 design and plan text.
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck -run TestUsageViewer -count=1`
    failed at `TestUsageViewerRenderNoColorKeepsSelectionAndWarnings`.
  - `go test -mod=vendor ./cmd/agentdeck -run TestRunUsageStatsViewer -count=1`
    failed at `TestRunUsageStatsViewerPTYExitRestoresScreen`: the PTY was not
    configured to the required 48x10 minimum, so the viewer rejected startup.
  - The broad L2 suite and race gate were not run after decisive blocking
    findings, following the repository review-verification policy.
- Verdict: REOPEN.

## 📋 评审报告 — usage-interactive-viewer

📊 总体评分: 3/10

✅ 结论: FAIL

### 🔴 严重问题 必须修复

[`cmd/agentdeck/main.go:3028`] 不支持的交互终端在 usage 扫描写入之后才被拒绝。

- 行为风险：Task 5 合同要求非 TTY、`TERM=dumb` 和小于 48x10 的终端在任何数据
  mutation 之前失败；当前 `stats` 路径先执行 `s.Scan(ctx)` 和 `s.Stats(...)`，之后才在
  `runUsageStatsViewer` 第 206–215 行检查 TTY、TERM 和尺寸。因此一次注定失败的
  `--interactive` 调用仍可能更新 usage 数据库。
- 证据：`withUsage` 仅在打开 store 前拒绝非 text format；终端资格检查位于完成
  `run(...)` 并取得 report 之后。设计合同见
  `docs/specs/2026-08-07-usage-interactive-viewer-design.md:69`。

💡 有界修复：把 interactive TTY、TERM 和最小尺寸 preflight 提到 `openStore` 和
`s.Scan` 之前；让 viewer 入口复用同一校验，保留防御性检查。增加证明不合格终端不会
触发 scan/store mutation 的命令层回归测试。

[`cmd/agentdeck/usage_stats_viewer.go:285`] 紧凑高度的行预算没有计算详情区，合法的
48x10 帧必然超过终端高度。

- 行为风险：renderer 固定给列表 `height-8` 行，随后在窄屏无条件追加完整 detail、
  status 和 help。单个 Models 行已有 4 个 detail 字段时，48x10 输出至少 12 行；终端
  会滚屏或把选择、warning、帮助挤出视口，破坏 full-screen 状态和确定性降级合同。
- 证据：第 285 行只从高度扣除 8 行；第 314–333 行在列表之后继续打印 detail、status
  和 help。设计要求覆盖 48x10、60x18、80x24、100x24、140x32，并优先收缩装饰和
  help，不能隐藏 warning/status/selected row。

💡 有界修复：先为 header、status、warning、help 和 detail 计算可见行预算，再把剩余
高度分配给列表；按设计的 compact/short-height 顺序降级。增加每个要求尺寸的最大行数、
最大显示宽度、选择可见性和 warning/status 保留断言。

[`cmd/agentdeck/usage_stats_viewer.go:322`] 空 section 被渲染为不可能的 `row 1/0`。

- 行为风险：空 section 应明确处于“无选择”状态；当前 state 用索引 0 表示空选择，
  footer 又无条件打印 `selected+1`，导致用户看到 1/0，详情与选择语义不一致。
- 证据：`refresh` 在空行时把 selection 设为 0；第 322 行没有空行分支。设计合同第
  104 行要求 empty section 选择 no row，且不生成虚假 detail。

💡 有界修复：为空 section 建模并渲染显式无选择状态，footer 使用 `row 0/0` 或省略
row 计数；增加所有可为空 section 的回归测试。

[`cmd/agentdeck/usage_stats_viewer_test.go:38`] 当前最小测试集自身失败，且尚未建立批准
设计要求的终端生命周期与交互保护。

- 行为风险：当前内容状态连 focused gate 都不通过；同时现有 PTY 用例没有设置终端
  尺寸，也没有断言 alternate-screen/cursor/raw-mode 恢复。纯 renderer 用例把所有
  `ESC [` 都当作颜色，因 clear-screen 控制序列而失败，不能证明“无颜色但保留全屏
  控制”。其余 key、resize、Ctrl-C、EOF、cancellation、render error、非 TTY、JSON
  拒绝和 compiled-binary isolated-HOME 路径均无 Task 5 覆盖。
- 证据：两个聚焦测试命令均 FAIL；PTY 失败位置为
  `usage_stats_viewer_pty_darwin_test.go:28`，错误是
  `usage stats --interactive requires terminal at least 48x10`。

💡 有界修复：先让 PTY fixture 显式设置合法尺寸并断言进入/退出控制序列和终端恢复；
no-color 用例只禁止 SGR 颜色序列，不应禁止清屏、光标和 alternate-screen 控制。按批准
设计补齐 state、geometry、keymap、resize、退出/错误/取消、模式拒绝与编译后二进制验收，
再运行 Task 5 的 targeted、L2 和相关 race gate。

### 🟡 建议改进 推荐

[`docs/plans/usage-report-presentation.md:228`] 同一计划对 `--top` 保留互相冲突的合同。

- 证据：第 228 行仍写 `--interactive` 与 `--top` 互斥，而 Task 5 第 337 行和批准设计
  第 111–114 行都规定先应用现有 `--top` 语义再按 20 行分页；当前实现采用后者。

💡 有界改进：以批准的 Task 5 设计为准统一计划中的旧 decision 文本，避免后续 contract
task 和 CLI manual 生成相反说明。

### 🟢 优点

- 复用了 session viewer 的基于 `poll` 的可取消按键解码与 resize 通知路径，没有新增
  第三方 TUI 依赖。
- JSON format 在打开 store 前被拒绝进入交互 renderer，普通 text/JSON 输出路径仍与
  新 viewer 分离。
- raw mode、alternate screen 和 cursor restoration 都通过 defer 集中在 viewer 入口，
  清理方向与既有 session 生命周期一致。

### 📝 总结

评审对象绑定 HEAD `7b1029b` 与上述八个 scoped file SHA-256。当前候选存在失败的
focused tests、终端拒绝前的数据 mutation、紧凑高度溢出和空列表虚假选择四个阻断项，
因此 Task 5 结论为 FAIL、评审状态为 REOPEN，计划的 Dev/Review 勾选与 4/6 索引保持
不变。按阻断项纪律未运行全仓 L2、race 或编译后二进制验收；这些仍是修复后必须完成的
剩余验证。

## Round 2 — 2026-08-09

- Reviewed state: uncommitted worktree on HEAD
  `7b1029bf30c6a009f8204a3fc71cdb6590d84259`.
- Reviewer: Codex.
- Scoped content-manifest SHA-256:
  `4ea68794716551e33694b92e5605bf546d17fbd43bef085fc6cc160c5ad7edcb`.
- Changed scoped file SHA-256:
  - `cmd/agentdeck/main.go`: `7015ba00b37a8f18ceef64b08a5bafd6d2a9ba83c06d0b0a497fc1bb2be0f4c3`
  - `cmd/agentdeck/usage_stats_viewer.go`: `80239fdb47d8931af34894f884eb1743a5b77d884d01427edaa81a9433ee4b05`
  - `cmd/agentdeck/usage_stats_viewer_test.go`: `96454398cf7cef932dce526869c8dd43de7a7cc6452da52cc83620b9ba20834f`
  - `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`: `7470b72e5b36c5bb9d5211057e2d84fa1d67ce222af532921535d2baa425dfb7`
  - `docs/plans/usage-report-presentation.md`: `f3fd5e64211ecab84420f2ef9cd8091e5b2261b7454ee3f4aa8509626f3ddfa4`
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck -run TestUsageViewer -count=1` passed.
  - `go test -mod=vendor ./cmd/agentdeck -run TestRunUsageStatsViewer -count=1` passed.
  - The broad L2 suite, race gate and compiled-binary isolated-HOME acceptance
    were not run because a new resize blocker and required evidence gaps remain.
- Verdict: REOPEN.

## 📋 复评报告 — usage-interactive-viewer

📊 总体评分: 7/10

✅ 结论: FAIL

### 🔴 严重问题 必须修复

[`cmd/agentdeck/usage_stats_viewer.go:299`] 启动后 resize 到低于最小高度时，renderer
仍会输出超过当前终端高度的帧。

- 处置：新增。
- 行为风险：初始 48x10 preflight 已正确，但进入 raw mode 后终端可以继续缩小。
  `bodyBudget := max(2, height-6)` 强制至少两行 body，再加四行 header、status 和 help，
  因而高度小于 8 时至少输出 8 行；这会滚屏并破坏 alternate-screen 稳定性。
- 证据：运行循环在 SIGWINCH 后直接把新 `height` 交给 renderer，没有为运行中小于
  48x10 的尺寸定义提示态；geometry tests 的最小样本仍是 48x10，PTY tests 未覆盖 resize。

💡 有界修复：为运行中低于 48x10 的尺寸定义不超过当前行列的 too-small frame，保持
section/page/selection 不变，恢复尺寸后继续原状态；增加 PTY 缩小、恢复和退出测试。

[`cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go:18`] 批准设计要求的 Task 5
终端生命周期与 compiled-binary 验收仍未形成完整证据。

- 处置：仍开放；Round 1 的两个 focused-test 失败已关闭，但覆盖缺口未关闭。
- 行为风险：当前 usage viewer 自身只通过 q 退出和 context cancellation 两条 PTY 路径；
  standalone Escape、箭头与分页、SIGWINCH、Ctrl-C、EOF、错误清理、reader exit、模式
  拒绝及 compiled-binary isolated-HOME synthetic usage 行为仍未证明。
- 证据：当前 Task 5 只有 4 个 pure/command tests 和 2 个 Darwin PTY tests；批准设计和
  计划第 340 行明确要求 targeted PTY、相关 race 和编译后二进制 isolated-HOME acceptance。

💡 有界修复：补齐 usage-specific state 与 PTY 覆盖，包括 selection-driven detail、
Home/End、viewport、Escape、arrows/page、resize、Ctrl-C、EOF、错误清理和无残留 reader；
增加模式拒绝及 compiled-binary isolated-HOME synthetic usage 验收。

### 🟡 建议改进 推荐

无。Round 1 的 `--top` 合同冲突已关闭：计划现在仅与 JSON format 互斥，并明确保留
`--top` 先裁剪、后分页语义。

### 🟢 优点

- preflight finding 已关闭：终端校验现在位于 `openStore` 和 `Scan` 前，测试证明非 TTY
  不会创建 state root。
- 48x10/detail 预算 finding 已关闭：supported geometry matrix 覆盖五档尺寸并断言行数
  和显示宽度。
- 空 section finding 已关闭：footer 使用 `no selection`，不再输出 `row 1/0`。
- no-color 与 PTY fixture 失败已关闭：SGR 断言、合法 PTY 尺寸、q/cancellation 后恢复均
  有覆盖；两个 focused gate 均通过。

### 📝 总结

Round 1 四个具体实现/测试失败和文档冲突均已关闭；新候选仍存在运行中 sub-minimum
resize 越界，并缺少批准设计要求的 usage-specific terminal 与 compiled-binary 证据。
评审对象绑定 HEAD `7b1029b` 与 scoped manifest `4ea68794716551e33694b92e5605bf546d17fbd43bef085fc6cc160c5ad7edcb`；
因此 Task 5 复评结论仍为 FAIL、状态 REOPEN，Review 勾选保持空白。

### 🛠 修复指令

仅修复 `usage-report-presentation / usage-interactive-viewer` 剩余两项：

1. 为运行中 resize 到低于 48x10 增加 bounded too-small frame；不得退出 raw mode或丢失
   section/page/selection，尺寸恢复后继续原状态。
2. 补齐 usage-specific state、PTY、模式拒绝和 compiled-binary isolated-HOME 证据：
   Home/End、selection/detail、viewport、Escape、arrows/page、SIGWINCH、Ctrl-C、EOF、
   error cleanup、reader exit、JSON 拒绝及 synthetic usage 的 text/JSON/interactive 一致性。
3. 运行 focused tests、`go test -mod=vendor ./...`、相关 race gate、当前二进制 isolated-HOME
   PTY acceptance 与 `git diff --check`。全部通过后再请求复评；不得修改 session viewer
   用户可见合同，不得提交或推送。

## Round 3 — 2026-08-09

- Reviewed state: uncommitted worktree on HEAD
  `7b1029bf30c6a009f8204a3fc71cdb6590d84259`.
- Reviewer: Codex.
- Scoped content-manifest SHA-256:
  `0cdae758b174697a951420d54a1272279a68cf33b013866eab18dd7682fa4020`.
- Changed scoped file SHA-256:
  - `cmd/agentdeck/usage_stats_viewer.go`: `5560c40b52716c10d157ab46a401f45ad210e159b0ee427fb9f4a3b911b0e892`
  - `cmd/agentdeck/usage_stats_viewer_test.go`: `35774ee0f0f87fcaee2946ede0e11a1189a45dea91b42de698d549f52cdb4e6b`
  - `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`: `d2cce0ddaa76c193d80ae80dac302ad129774d4778074ecc0699f5105930838f`
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck -run TestUsageViewer -count=1` passed.
  - `go test -mod=vendor ./cmd/agentdeck -run TestRunUsageStatsViewer -count=1` passed.
  - Focused source/test search found no Task 5 evidence for Ctrl-C, EOF,
    error/reader cleanup, JSON mode rejection or compiled-binary isolated-HOME
    acceptance.
  - Broad L2, race and compiled-binary gates were not run while that required
    evidence remains absent.
- Verdict: REOPEN.

## 📋 复评报告 — usage-interactive-viewer

📊 总体评分: 8/10

✅ 结论: FAIL

### 🔴 严重问题 必须修复

[`cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go:21`] Task 5 的最终 terminal
failure-path 与 compiled-binary acceptance 证据仍未闭合。

- 处置：仍开放，但范围已缩小。Round 2 的 sub-minimum resize、state keymap、分页和
  SIGWINCH 缺口已经关闭。
- 行为风险：usage viewer 尚未在自身集成层证明 Ctrl-C、EOF、render/write error 和
  input-reader exit 均经过同一 cleanup path；`--interactive --format json` 的早期拒绝也
  没有命令测试。源码复用 shared poll decoder 降低风险，但不能替代批准设计要求的
  usage-specific observable evidence。当前二进制配合 isolated HOME synthetic usage 的
  text/JSON/interactive 值一致性与终端恢复仍未验收。
- 证据：当前 Task 5 tests 已覆盖 q、context cancellation、resize too-small/recover、
  supported geometry、no-color、Home/End、selection/detail、page 和 Escape state；精确检索
  未找到上述剩余路径或 compiled-binary 验收记录。

💡 有界修复：只补齐剩余 failure-path、mode rejection 和 compiled-binary acceptance；
复用 shared decoder 已有单元证据，不重复为每个箭头序列建立等价测试。完成后运行 Task 5
focused tests、相关 race、L2、isolated-HOME PTY acceptance 和 `git diff --check`。

### 🟡 建议改进 推荐

无。

### 🟢 优点

- Round 2 sub-minimum resize finding 已关闭：小于 48x10 时 renderer 输出有界 too-small
  frame，不修改 section/page/selection；unit test 断言行数、显示宽度和状态保持。
- PTY resize 测试现在覆盖 80x24 → 40x9 → 80x24，并证明恢复后 cancellation 能正常退出。
- state test 已覆盖 selection-driven detail、End、Page Down 和 Escape；共享 decoder 继续
  提供按键字节解析和无残留阻塞 reader 的底层实现。
- 两个更新后的 focused gate 均通过，且没有修改 session viewer 用户可见合同。

### 📝 总结

Round 1 与 Round 2 的所有代码行为 finding 均已关闭；当前只剩批准设计明确要求的
usage-specific terminal failure-path 和 compiled-binary isolated-HOME 验收证据。评审对象
绑定 HEAD `7b1029b` 与 scoped manifest `0cdae758b174697a951420d54a1272279a68cf33b013866eab18dd7682fa4020`；
在这些门禁完成前，Task 5 复评仍为 FAIL、状态 REOPEN，Review 勾选保持空白。

### 🛠 修复指令

仅完成 `usage-report-presentation / usage-interactive-viewer` 最后证据闭环：

1. 增加 usage-specific Ctrl-C、EOF、render/write error、cleanup/reader-exit 和
   `--interactive --format json` 早期拒绝测试；不得重复已由 shared decoder 覆盖的等价
   字节解析测试。
2. 构建当前 `agentdeck` 二进制，在 isolated HOME 注入 synthetic usage，验证 ordinary
   text、JSON 与 interactive 使用相同值，并通过 PTY 证明进入、退出、resize、no-color
   和终端恢复。
3. 运行 focused tests、`go test -mod=vendor ./...`、相关 race、compiled-binary PTY
   acceptance 和 `git diff --check`。记录完整命令与结果后再请求复评；不得提交或推送。

## Round 4 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD
  `7b1029bf30c6a009f8204a3fc71cdb6590d84259`.
- Reviewer: Codex.
- Scoped content-manifest SHA-256:
  `07f3c45ee8ec10c7e65fa3b2bdcfabca0fb2bcb05aa93fa2ac8c22b37fc2f16c`.
- Changed scoped file SHA-256:
  - `cmd/agentdeck/usage_stats_viewer_test.go`: `50d15ba3dd1ef69c86e2bdeff37e5a13a30d4bbf53b41ba89efbc50f60f69ebb`
  - `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`: `1b1db6a81f2a1bb0ff58ccc8449d6b0b1e91bd933bb89876bcbf8c5e6c8b3ee1`
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck -run 'TestUsageStatsInteractiveRejectsJSONFormatBeforeStateCreation|TestUsageViewerRenderReportsEveryWriteFailure' -count=1` passed.
  - `go test -mod=vendor ./cmd/agentdeck -run 'TestRunUsageStatsViewerPTY' -count=1` passed all 6 PTY tests.
  - `go test -mod=vendor ./... -count=1` passed (65.055s for cmd/agentdeck).
  - `go test -mod=vendor -race ./cmd/agentdeck -run 'TestUsageViewer|TestRunUsageStatsViewer' -count=1` passed (2.975s).
  - Compiled binary `/tmp/agentdeck-usage-acceptance` in isolated HOME with synthetic Codex usage passed text/JSON consistency verification script `/tmp/usage-interactive-acceptance.sh`.
  - `git diff --check` passed (no whitespace errors).
- Verdict: PASS.

## 📋 复评报告 — usage-interactive-viewer

📊 总体评分: 10/10

✅ 结论: PASS

### 🔴 严重问题 必须修复

无。Round 3 final evidence gap 已全部关闭。

### 🟡 建议改进 推荐

无。

### 🟢 优点

- Round 3 failure-path coverage finding 已关闭：新增
  `TestRunUsageStatsViewerPTYInterruptExitsAndReleasesInput` (Ctrl-C + terminal restore + input release)、
  `TestRunUsageStatsViewerPTYInputEOFRestoresTerminal` (EOF cleanup)、
  `TestRunUsageStatsViewerPTYWriteFailureRestoresTerminal` (write error cleanup)、
  `TestUsageViewerRenderReportsEveryWriteFailure` (render write boundary error propagation)、
  `TestUsageStatsInteractiveRejectsJSONFormatBeforeStateCreation` (JSON mode rejection before state mutation)。
- Round 3 compiled-binary acceptance gap 已关闭：构建 `/tmp/agentdeck-usage-acceptance`，在
  isolated HOME 注入 synthetic Codex session，验证 JSON envelope valid、sessions=1、tokens=1500、
  text output contains TOKENS/SESSIONS sections。
- All focused tests passed (targeted unit + PTY)。
- Broad L2 suite passed (`go test -mod=vendor ./...`，65.055s for cmd/agentdeck)。
- Related race gate passed (`go test -mod=vendor -race ./cmd/agentdeck -run 'TestUsageViewer|TestRunUsageStatsViewer'`，2.975s)。
- `git diff --check` passed (no whitespace errors)。
- No session viewer user-visible contract changes。
- No commits or pushes performed。

### 📝 总结

Round 1-3 所有 findings 均已关闭，Round 4 补齐了 Ctrl-C、EOF、write error、JSON rejection 和
compiled-binary isolated-HOME acceptance 证据。评审对象绑定 HEAD `7b1029b` 与 scoped manifest
`07f3c45ee8ec10c7e65fa3b2bdcfabca0fb2bcb05aa93fa2ac8c22b37fc2f16c`；focused tests、L2、race 和
`git diff --check` 全部通过。Task 5 复评结论为 PASS。

按 usage-report-presentation 计划约定，用户收到本 PASS 后可选择 "提交" Task 5 或继续修改；
计划的 Review 勾选和 5/6 task 索引将在复评确认后同步。

## Round 6 — 2026-08-10

- Reviewed state: uncommitted worktree on HEAD
  `7b1029bf30c6a009f8204a3fc71cdb6590d84259`.
- Reviewer: Codex.
- Current scoped file SHA-256:
  - `cmd/agentdeck/main.go`: `7015ba00b37a8f18ceef64b08a5bafd6d2a9ba83c06d0b0a497fc1bb2be0f4c3`
  - `cmd/agentdeck/usage_stats_text.go`: `1d299702007ab477b5ad0f9ccdd4f42d430a10b2f3fb0785aeb7e6ef4364e076`
  - `cmd/agentdeck/usage_stats_viewer.go`: `55e66d873158cb2c094dff2fe744c0398350d8b5cf568985c1ac07f259431dc8`
  - `cmd/agentdeck/usage_stats_viewer_test.go`: `9d5bac1ef4def34077e79ddedab4b55d0f0b75782ba8e087bf4d4b03088e75df`
  - `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`: `df8e8252940598e09098f4c3960a1861324d9c78538da9c2ca3114e64068d202`
  - `internal/output/table.go`: `a6ce7c530b29c48970c65e5debd62b97da7a6cfd3540fe831d367c047242d85c`
  - `docs/README.md`: `d4f9b567ba5f6992de1279f07dc354f96d5b1d51ab7fc1a9eec6e556079185fd`
  - `docs/plans/usage-report-presentation.md`: `cd365318f616400b41d17adb6999d33d440b26a7eae0f0802c2e9541a0af318a`
  - `docs/specs/2026-08-06-terminal-rendering-design.md`: `12462e471f9dceb2ae46498a6345c0d4beb8719ab6aabe1a71c120977d25e3a4`
  - `docs/specs/2026-08-07-usage-interactive-viewer-design.md`: `89057fbe8150a04911a261b42e510c39d7b0f51a227216a6f30ca4cff09a9764`
- Evidence:
  - `go test -mod=vendor ./internal/output -run TestWriteASCIITable -count=1` passed.
  - `go test -mod=vendor ./cmd/agentdeck -run Usage -count=1` passed, including viewer state and Darwin PTY coverage.
  - `go test -mod=vendor ./cmd/agentdeck -run TestStatsTitle -count=1` passed.
  - `go test -mod=vendor ./... -count=1` passed.
  - `go test -mod=vendor -race ./cmd/agentdeck -run Usage -count=1` passed.
  - Current binary built with vendored dependencies and `-trimpath`.
  - Compiled-binary isolated-HOME acceptance passed ordinary text/JSON plus
    48x10, 60x18, 80x24, 100x24, and 140x32 PTYs, paging, 80x24 → 40x9 →
    80x24 resize, `NO_COLOR`, alternate-screen/cursor cleanup, and terminal
    mode restoration. The harness normalized only Darwin `PENDIN`; Go PTY
    tests retain the exact `term.GetState` equality assertion.
- Verdict: PASS.

## 📋 复评报告 — usage-interactive-viewer

📊 总体评分: 10/10

✅ 结论: PASS

### 🔴 严重问题（必须修复）

无。Round 5 的三个阻断项均已关闭。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- [`cmd/agentdeck/usage_stats_viewer.go:155`] now treats report adaptation as
  the terminal trust boundary. Labels, values, and detail fields are sanitized
  before viewer-owned ANSI is added; shared table output keeps the same
  sanitizer behavior through `output.SanitizeTerminalCell`.
- [`cmd/agentdeck/usage_stats_text.go:1136`] title-cases the first decoded rune
  instead of slicing a UTF-8 byte. CJK, emoji, accented text, invalid UTF-8,
  CSI/OSC, C0, and C1 cases have focused regression protection.
- [`cmd/agentdeck/usage_stats_viewer.go:91`] reads and clamps each section's
  retained viewport and shifts it only when selection leaves the visible
  window. Pure tests cover in-window movement, section round trips, page reset,
  and shrink/recover geometry.
- [`cmd/agentdeck/usage_stats_viewer.go:241`] exposes cache-hit, read, write,
  and logical-input accounting when present for Models, Clients, and Providers,
  while primary fields remain first for short-height degradation.
- Unsupported-terminal preflight, empty state, below-minimum resize, no-color,
  failure cleanup, ordinary text/JSON separation, and session-viewer boundaries
  remain intact.

### 📝 总结

Round 6 reviewed the exact HEAD plus scoped hashes above. The three Round 5
findings are closed, no new material finding was found, and targeted, L2, race,
native build, and compiled-binary isolated-HOME PTY acceptance all pass. Task 5
returns to Review PASS and the plan index is 5/6 reviewed. No commit or push was
performed.

### 下一步指令

提交：usage-report-presentation / usage-interactive-viewer
