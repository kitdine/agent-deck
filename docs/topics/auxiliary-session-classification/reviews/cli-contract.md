---
status: active
topic: auxiliary-session-classification
subject: cli-contract
---

# cli-contract 评审记录

## Round 1 — 2026-11-05

### 📋 cli-contract 实现评审

📊 综合评分：7/10

✅ 评审结论：FAIL

- Reviewer：Codex。
- Method：单代理独立代码审查，CodeGraph 与当前源码核验，真实 macOS PTY 加受控阻塞 loader 的隔离反例；产品、测试与配置只读。
- Scope：任务 4 的命令入口、参数与回执路径、来源管理及交互范围加载；出现决定性 PTY 反例后停止扩大验证。
- Authority：[任务 4](../tasks.md#4-cli-contract)、[CLI 最终表面 §4/§6](../ux/cli.md) 的 loading 可退出、任一层 q 退出及终端输入生命周期要求。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`；开发交接评论 `9dc2d5db-f72f-54ea-aa6e-5656819e5d2e`。

### 🔴 严重问题（必须修复）

**ASC-CLI-R1-F1 · P2 · OPEN — 范围加载阻塞按键循环，loading 期间无法执行 q 退出。**

位置：[cmd/agentdeck/session_browser.go:223](../../../../cmd/agentdeck/session_browser.go#L223)。
`runSessionBrowser` 在同一交互循环里调用 `scopeLoad(requested)`；调用返回前不会再次执行 `readSessionViewerKey`。
加载画面虽然显示 `q quit`，但输入只能留在 PTY 队列里等待读取完成。

- 行为风险：索引读取慢或受阻时，用户无法按界面提示退出，终端仍被 raw/alternate-screen 交互会话占用。
  这违反已批准的 loading 可退出及任一层 q 退出合同；不是要求增加新交互能力。
- 证据：80×24 的真实 PTY，使用现有终端 owner/input 代码，进入 s 菜单选择 auxiliary；scope loader 在通道上受控阻塞。
  已观察到 `Loading committed sessions` 后输入 q，2 秒内 owner 未退出；释放 loader 后才退出。
  探针失败：`q was ignored throughout a blocked scope read; browser exited only after the read was released`。
  该实验隔离了输入调度缺陷，未读取私人数据，也没有声称测量真实数据库的最坏加载时长。
- 对照：现有 `TestSessionBrowserPTYSwitchesScopeWithoutWritingAndRestoresTerminal` PASS（0.11s），
  说明正常加载的切换与终端恢复测试未覆盖受阻加载期间的退出。
- 💡 修复建议：使范围读取可取消，并在加载期间继续处理退出和 resize；退出时取消并收束读取工作后恢复终端。
  以请求身份拒绝迟到加载结果，保留已有失败回退/选择语义。增加受控阻塞 loader 的 PTY 回归，
  证明 q 不依赖读取自然完成，且退出后的终端资源与后台读取均收束；不要用更长等待掩盖同步阻塞。
- 处置：OPEN，回交本任务修复。

### 🟡 建议改进（推荐）

无独立建议项。其余尚未完成的验收维度不视为已通过，也不凭推测登记发现。

### 🟢 优点

- 命令参数集中处理，并在共享 envelope 中以可加字段承载 session_selection。
- 正常范围切换的现有 PTY 测试验证了只读状态和终端恢复，本次对照仍通过。
- 任务复用共享 selection 和 mutation 服务，未建立另一套分类归约实现。

### 📝 总结

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`，入口 14 路径指纹
`8aa38dfdf208c24a221c5f45b51c93f2708f23e680ed34648a486e070d19671e`。
配方为 SHA-256(`head=<HEAD>` 行与 LC_ALL=C 路径排序的 `path=<git blob>` 行，均含末尾 LF)。
范围为下表 13 个产品/测试/fixture 路径及入口 tasks.md（blob `2684ad126a5eec684d65be426a2c5c4f7977136a`）；
新评审记录不纳入自身指纹。最终 14 路径指纹为
`0304d84a76cd25472d887c5fc3b86fdc104c6b9bad1858b99a3332ba6185ca14`；
仅 tasks.md blob 变为 `8551c3d7089685c54b3aa828ba826d52c79b2735`，13 个产品/测试/fixture blob 与入口一致，
反例因此适用于最终状态，未因交接文字变化重跑产品检查。

| 路径 | blob |
| --- | --- |
| cmd/agentdeck/e2e_test.go | `d27690e0459cfec8b2cee03c7524328ece260fac` |
| cmd/agentdeck/main.go | `b655be6c65cad5e2bd6559f4036591b25b15748d` |
| cmd/agentdeck/session_browser.go | `36d6eb650f63e0c9e67db2e99d436d690ae1d3d1` |
| cmd/agentdeck/session_browser_pty_darwin_test.go | `291043b0818aff953239eb59f2e59158f7e7686a` |
| cmd/agentdeck/session_origin.go | `23bc785c029e101d7d7ea7ba4a1a03cf77a214e5` |
| cmd/agentdeck/session_origin_test.go | `2c01aff9bef6c84225593fbfe622e929055c9911` |
| cmd/agentdeck/session_viewer_terminal.go | `5f762579721593dac070351c98ba546308dc3f9f` |
| cmd/agentdeck/testdata/phase7/gui-json-contract.json | `9f6188283ffe1138949b22b3b73c5d3c5519bf16` |
| internal/output/output.go | `6f84ada2865e369316a8d046374cc6882f7157d0` |
| internal/sessionorigin/selection.go | `640974d07732047161e21558a0010b45579e86ec` |
| internal/sessionorigin/selection_test.go | `812ce55186d031ffeebc2e788f1160171b5b9527` |
| internal/sessionorigin/sessionorigin.go | `ad41f206acb3d84c0437f027ee03f3cbc1365777` |
| internal/sessionorigin/sessionorigin_test.go | `27d74a6a706bf66163b6508658b837b9463dd444` |

Evidence：Go 1.27.3 darwin/amd64，显式 GO_TEST_BIN 与 GOROOT 指向同一安装；
go.mod `bc41e2c6481bba3582c8e5f208d4339cdc8ee0fd`、go.sum `b037abba0aebb3609441e94cb6db5035b013cbfe`、
vendor/modules.txt `35c60c6060305c34e187087aef6a98f82e9beea0`。
一次执行：

```sh
scripts/run-go-test.sh -overlay <overlay.json> ./cmd/agentdeck -run '^(TestReviewScopeLoadingCanQuitBeforeReadCompletes|TestSessionBrowserPTYSwitchesScopeWithoutWritingAndRestoresTerminal)$'
```

既有对照测试 PASS；新探针 FAIL（2.60s）；包耗时 4.098s，退出码 1。
overlay 将以下源码映射为该包虚拟 `zz_review_loading_test.go`，未写入工作树测试。
完整反例保留在本记录中，不依赖临时输出的留存；它复用同包现有 PTY fixture 和 owner helper：

```go
//go:build darwin

package main

import (
    "context"
    "sync"
    "testing"
    "time"

    "golang.org/x/sys/unix"
)

func TestReviewScopeLoadingCanQuitBeforeReadCompletes(t *testing.T) {
    t.Setenv("TERM", "xterm-256color")
    master, slave := openSessionViewerPTY(t)
    defer master.Close()
    defer slave.Close()
    if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
        t.Fatal(err)
    }
    started := make(chan struct{})
    releaseRead := make(chan struct{})
    var once sync.Once
    release := func() { once.Do(func() { close(releaseRead) }) }
    load := func(scope string) (sessionBrowserPage, error) {
        close(started)
        <-releaseRead
        return sessionBrowserPage{scope: scope}, nil
    }
    done, stop := startSessionBrowserPTYOwner(func(ctx context.Context) error {
        return runSessionBrowser(ctx, slave, slave, sessionBrowserPage{scope: "work"}, load, nil)
    })
    defer func() { release(); stop() }()
    readSessionBrowserPTYUntil(t, master, "s scope")
    if _, err := master.WriteString("s"); err != nil { t.Fatal(err) }
    readSessionBrowserPTYUntil(t, master, "esc cancel")
    if _, err := master.WriteString("\x1b[B\r"); err != nil { t.Fatal(err) }
    readSessionBrowserPTYUntil(t, master, "Loading committed sessions")
    select {
    case <-started:
    case <-time.After(time.Second): t.Fatal("scope loader did not start")
    }
    if _, err := master.WriteString("q"); err != nil { t.Fatal(err) }
    select {
    case err := <-done:
        if err != nil { t.Fatal(err) }
    case <-time.After(2 * time.Second):
        release()
        select {
        case err := <-done:
            if err != nil { t.Fatal(err) }
        case <-time.After(2 * time.Second): t.Fatal("browser did not stop even after read release")
        }
        t.Fatal("q was ignored throughout a blocked scope read; browser exited only after the read was released")
    }
}
```

按项目规则在决定性反例后停止扩大验证；未重跑开发者报告的全量/vet/race，也未将其计为本轮独立通过证据。
入口规范门禁为 NOT_VERIFIED（新目标尚未登记、八项尚无证据）。最终 ContentState 为
`auxiliary-session-classification:cli-contract:review-20261105-Ilvmxd-final-state`。
已登记 `interactive-pty` 与 `independent-review` 两条失败证据；3 个新增节点精确读回一致，
4 条关系预检 ok、写入数量 4。规范门禁返回 FAILED，两条失败证据适用于最终目标，
invalidated/unresolved 均为空，其余六项未因本轮提前停止而获得通过声明。

完成门禁：FAILED

记录检查：`make check-whitespace`、`python3 scripts/check-docs.py`、`git diff --check` 均通过。
Beads 已回交 in_progress、解除指派并加 round-1；交接评论 `1f91d426-89cf-508e-ba63-ce52e3aae087`。
CLI 的 workspace gate lock 权限警告仍存在，主操作以 ungated 模式成功，返回结果已核实。

修复范围：ASC-CLI-R1-F1 与其加载/退出回归。任务 Review 保持未勾选，无提交检查点；未提交、推送或修改全局状态。

## Repair 1 — 2026-11-05

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-CLI-R1-F1 及其加载/退出回归；未处理未登记问题。
- 复现：将 Round 1 记录中的探针原样经 overlay 运行于修复前内容，FAIL（2.60s），与记录一致。

**ASC-CLI-R1-F1 → 修复，待复评。**

- 变更：[cmd/agentdeck/session_browser.go](../../../../cmd/agentdeck/session_browser.go) 中范围读取改为在独立 goroutine 执行，
  `sessionBrowserScopeLoad` 增加 `context.Context` 参数；加载期间键盘循环继续运行：`q`/Ctrl-C/EOF 立即退出，
  Escape 只取消加载并保留旧列表与旧 Scope，resize 重绘 loading 帧；加载完成通过取消按键读取的子 context 唤醒循环。
  每次加载为一个 `sessionBrowserLoad`，只有仍为当前加载时才应用结果，已取消请求的迟到结果随之丢弃；
  失败回退（failedScope）与选择保留语义不变。退出路径先取消读取并有界等待其返回（`sessionBrowserLoadDrain` 500ms），
  再按 defer 顺序恢复终端；遵守 context 的 loader 在该界限内收束，不遵守的 loader 不能挟持终端。
  [cmd/agentdeck/main.go](../../../../cmd/agentdeck/main.go) 的 loader 改用传入的加载 context，SQLite 读取随之可取消。
  loading 帧帮助改为 `q quit · esc cancel`，小于 48×10 时显示精简 loading 帧而非不重绘。
- 回归（[session_browser_pty_darwin_test.go](../../../../cmd/agentdeck/session_browser_pty_darwin_test.go)，真实 80×24 PTY）：
  `TestSessionBrowserPTYQuitsDuringABlockedScopeRead`：loader 忽略取消并永久阻塞，q 后观察到终端离开 alternate screen 且 owner 返回；
  `TestSessionBrowserPTYQuitCancelsAndCollectsTheScopeRead`：遵守 context 的阻塞 loader，owner 返回时 loader 已观察到取消并返回；
  `TestSessionBrowserPTYEscapeCancelsLoadAndDiscardsTheLateResult`：Escape 后旧列表与 `Scope: work` 恢复，
  随后释放被取消的 loader，其迟到结果不出现在下一帧。
- 验证：Round 1 探针按新签名（仅 loader 增加 context 参数，行为不变）经 overlay 运行 PASS；
  浏览器 PTY 测试与该探针连续 10 轮全部通过；`-race` 浏览器测试无竞争；`go vet ./cmd/... ./internal/...` 通过；
  `scripts/run-go-test.sh ./cmd/agentdeck ./internal/sessionorigin` 通过。
  修复过程中发现并修正了新测试自身的一处读取假设（同一帧被一次读完时第二次读取等待不存在的文本），产品代码无相应缺陷。
- 候选 blob：session_browser.go `891190ce63e4ad62a63036476b4198a49b7c00ef`、
  session_browser_pty_darwin_test.go `7b0ea2a680e0c805794327916019f94c29b77381`、
  main.go `de5f272394188df7d8aed70b0f088abb91d1ddc3`；其余 Round 1 路径未变。

修复后状态：ASC-CLI-R1-F1 待复评关闭；任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 2 — 2026-11-05

### 📋 ASC-CLI-R1-F1 限定复评

📊 综合评分：9/10（仅本轮限定范围）

✅ 复评结论：PASS

- Reviewer：Codex。
- Scope：真实用户明确选择 `reviews/cli-contract.md / ASC-CLI-R1-F1`；仅该发现及受影响的加载、取消、迟到结果与终端恢复路径。
- Method：单代理独立源码核验，按新 loader 签名重放原反例并运行 5 个相关真实 macOS PTY 回归；产品、测试和配置保持只读。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`；Repair 1 交接评论 `e1a4a60f-24a5-5509-a95a-b1f8c15a9314`。

### 🔴 严重问题（必须修复）

无。ASC-CLI-R1-F1 CLOSED；本轮限定范围内无新增 finding。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- **ASC-CLI-R1-F1 CLOSED**：范围读取在独立 goroutine 内执行并接收 context；加载期间继续读取按键。
  q 在阻塞读取期间可以退出，遵守 context 的读取在 owner 返回前已取消并返回；非合作 loader 的等待以 500ms 为界。
- Escape 保留旧 Scope/列表，已取消请求的迟到结果不再应用；正常范围切换、状态只读与终端恢复回归通过。
- main.go 的实际 scope loader 将加载 context 传入共享 selection 读取；旧反例只调整了新增 context 参数，原阻塞行为与退出断言不变。

### 📝 总结

Finding disposition：ASC-CLI-R1-F1 CLOSED。上述 PASS 只回答该 finding，不升级为整个 cli-contract 任务的独立评审 PASS。
Round 1 曾在决定性失败后停止其他维度；本轮没有扩大用户明确选择的范围，任务 Review 因此仍未勾选。
历史失败、修复说明和原反例完整保留。

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`；入口 14 路径指纹
`6f0bd851b6fb6c267e035c3aafede1e021a3fdfb7a52530494e3e93154fa3ab5`，配方与范围同 Round 1，评审记录自身不参与指纹。
产品修复 blob 与交接一致：main.go `de5f272394188df7d8aed70b0f088abb91d1ddc3`、
session_browser.go `891190ce63e4ad62a63036476b4198a49b7c00ef`、
session_browser_pty_darwin_test.go `7b0ea2a680e0c805794327916019f94c29b77381`。
其余 10 个产品/测试/fixture blob 与 Round 1 表一致；入口 tasks.md blob 为 `9c9094e3842d39a8061ceb9a39175c1c0f083559`。
限定交接同步后的最终 14 路径指纹为 `28201a165bf55dede4e2237ef2704b31cd89209c6d5ae13a71058e972bb31494`；
仅 tasks.md blob 变为 `ec99321cea64b020dd00a8408e0543341e727678`，13 个产品/测试/fixture blob 与入口一致。
限定通过观察适用于最终状态，未因交接文字变化重跑测试。

Evidence：Go 1.27.3 darwin/amd64，显式 GO_TEST_BIN/GOROOT；go.mod/go.sum/vendor/modules.txt blob 与 Round 1 相同。
原文探针仅将 `load := func(scope string)` 改为 `load := func(_ context.Context, scope string)` 以适应接口，
仍忽略取消并保持通道阻塞；经外部 overlay 注入，未更改工作树测试。一次运行：

```sh
scripts/run-go-test.sh -overlay <overlay.json> ./cmd/agentdeck -run '^(TestReviewScopeLoadingCanQuitBeforeReadCompletes|TestSessionBrowserPTYQuitsDuringABlockedScopeRead|TestSessionBrowserPTYQuitCancelsAndCollectsTheScopeRead|TestSessionBrowserPTYEscapeCancelsLoadAndDiscardsTheLateResult|TestSessionBrowserPTYSwitchesScopeWithoutWritingAndRestoresTerminal|TestSessionBrowserPTYOwnerCanBeStoppedBeforeTheFirstFrame)$'
```

6 个顶层测试全部 PASS，0 FAIL、0 SKIP，退出码 0，包耗时 4.142s：

| 测试 | 用时 | 可观察结果 |
| --- | --- | --- |
| TestReviewScopeLoadingCanQuitBeforeReadCompletes | 1.10s | 原反例不再需要释放读取才能退出 |
| TestSessionBrowserPTYQuitsDuringABlockedScopeRead | 0.50s | 不遵守取消的阻塞 loader 不能挟持终端 |
| TestSessionBrowserPTYQuitCancelsAndCollectsTheScopeRead | 0.00s | 合作 loader 在 owner 返回前收到取消并返回 |
| TestSessionBrowserPTYEscapeCancelsLoadAndDiscardsTheLateResult | 0.54s | Escape 恢复旧列表，迟到结果不出现 |
| TestSessionBrowserPTYSwitchesScopeWithoutWritingAndRestoresTerminal | 0.13s | 正常切换、状态字节与屏幕恢复保持 |
| TestSessionBrowserPTYOwnerCanBeStoppedBeforeTheFirstFrame | 0.60s | 首帧前的 context 取消可结束 owner |

边界：这证明真实 PTY 的调度、取消与恢复，阻塞源为受控 fixture，不是私人数据库的延迟测量。
Go 无法强制终止忽略 context 的任意回调；实现对该防御场景有界等待后恢复终端，正常生产 loader 负责遵守传入 context。
本轮未重跑开发者已报告的全量/vet/race，也没有借此声明任务的其他验收准则已完成。

任务级缺口（不是本 finding 的未关闭问题）：

- session-scope-reads、classify-mutations、scope-preference、origin-source-commands、json-backward-compat、l3-privacy-concurrency 的完整任务验收与精确状态证据。
- interactive-pty 的完整任务矩阵；本轮只覆盖本 finding 及影响路径，未重新判定全部宽度、键盘和入口要求。
- independent-review 的整个任务候选覆盖；本轮限定 PASS 不能替代该准则。

本任务之后仍有 desktop-menubar 等后续任务，本轮不跨 Topic 边界、不启动后续开发。

完成门禁：NOT_VERIFIED

最终 ContentState：`auxiliary-session-classification:cli-contract:rereview-20261105-X7Njip-final-state`。
3 个新增节点精确读回一致，4 条关系预检 ok、写入数量 4。规范门禁返回 NOT_VERIFIED，八项任务准则仍未完整满足，
invalidated/unresolved 均为空。当前两条观察明确记载 finding_disposition=closed、scoped_check_outcome=pass，
对完整 interactive-pty/independent-review 准则的 outcome 仍为 not_verified，避免将限定证据误算为整体通过。
Round 1 失败仍绑定历史内容，不适用于新 target，也未被改写或删除。

Task checkpoint：`ad-auxiliary-session-classification-cli-contract`；content_state `28201a165bf55dede4e2237ef2704b31cd89209c6d5ae13a71058e972bb31494`；gate NOT_VERIFIED，限定 finding 已关闭、完整任务边界仍开放。
提交建议：门禁 VERIFIED 后再提交 cli-contract 的完整任务边界；前置为上述八项任务级准则补齐完整覆盖与精确状态证据，当前限定 PASS 不构成提交就绪。
推送建议：门禁 VERIFIED 后，经单独授权推送 `origin/feature/auxiliary-session-classification`；同样先补齐上述任务级验收与完整候选评审证据，再核验提交与远端差异。

记录检查：`make check-whitespace`、`python3 scripts/check-docs.py`、`git diff --check` 均通过。
Beads 保持 in_review、解除指派并更新 round-2；交接评论 `61bc8cb0-053d-5338-b5a0-77c3b4b27f34` 明确限定范围与任务级缺口。
workspace gate lock 权限警告仍存在，主操作以 ungated 模式成功并已核实返回状态。未提交、推送或开始后续任务。

## Round 3 — 2026-11-05

### 📋 cli-contract 完整候选评审

📊 综合评分：6/10

✅ 评审结论：FAIL

- Reviewer：Codex，单代理独立评审；实现作者为另一会话的 claude-code，未委派。
- Scope：真实用户在原会话 `01a1ab82-3875-73a0-9125-17d640f4deb5` 明确要求补齐范围读取、分类写入、默认偏好、来源命令、JSON 兼容、隐私/并发、完整 PTY 矩阵及完整候选评审；本次按用户续接请求完成中断的记录。
- Method：沿用原会话的 CodeGraph/源码、隔离命令、服务 oracle 和真实 macOS PTY 证据；续接时逐一比较 14 路径 blob、HEAD 和依赖身份。未因换会话重跑相同产品验证。
- Authority：[任务 4](../tasks.md#4-cli-contract)、[CLI 最终表面](../ux/cli.md) §2–§7；判定只覆盖本任务。
- Workspace：`agent-deck.auxiliary-session-classification`，`feature/auxiliary-session-classification`；Git common dir 和实际工作区均与已建立的绑定一致。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`。本轮不修复产品、不执行 Git 交付、不启动后续任务。

### 🔴 严重问题（必须修复）

**ASC-CLI-R3-F1 · P2 · OPEN — usage 选择元数据未纳入报告时间窗。**

位置：[session_origin.go:1480](../../../../cmd/agentdeck/session_origin.go#L1480)。
`scopedSignals` 给 `Reader.Select` 的 Selection 只有 Scope 和 Client，报告的 From/To 仅传给内层 Signals。
因此信号报告按时间过滤，而 unknown_count 与 publication_id 仍代表全时间的选择。

- 证据：`TestReviewUsageSelectionMetadataFollowsTimeWindow` 先建立仅在 2026-08-29 活动的 Claude 会话，
  以 `usage stats --no-scan --from <day> --to <day> --client claude --session-scope work --format json`
  分别读取 2026-08-29 和 2026-08-30。前置断言确认 sessions 为 1/0；实际 unknown_count 为 1/1，
  read_vector 相同且 publication_id 相同。两条断言失败，探针 0.77s。
- 违反 CLI §7 的 unknown_count 分母为分页前选定会话集合，以及架构中包含时间窗的选择身份；
  用户会在空时间窗看到窗外 unknown 数，消费者不能用 publication_id 区分两个不同选择。
- 💡 修复：将已经解析的报告时间窗传入共享 Selection，与信号读取使用同一选择身份。
  保持 usage-only 和消耗块 all 的既有语义；以空窗/有数据窗及两条 usage 入口回归。
  依据：[现有 CLI 元数据合同](../ux/cli.md#7-已定字段与-verification-design)和共享 `sessionorigin.Selection`，不另建时间过滤器。

**ASC-CLI-R3-F2 · P2 · OPEN — 分类输出的可复制命令把原始终端控制字符送到终端。**

位置：[session_origin.go:780](../../../../cmd/agentdeck/session_origin.go#L780)、
[session_origin.go:852](../../../../cmd/agentdeck/session_origin.go#L852)。
显示字段经过 safeCell，但 Change/Undo 命令只经过 shellQuote；shell 引号不阻止终端解释控制码。

- 证据：`TestReviewClassificationCommandsEscapeTerminalControls` 向隔离 sessions 库放入
  session ID `control-\x1b[31m-id`，分别运行 classify 查询和 `--set work`，均带 `--no-color`。
  Session/Target 字段显示为安全文本，但 Change/Undo 含原始 ESC `[31m`，两个子用例均失败。
  JSON 对照保留完整 identity 并合法转义，未输出原始 ESC；不应为修复文本而改变 JSON identity。
- 违反 CLI §5/§7 的控制字符消毒与安全命令输出要求；本实验实证颜色控制注入，不推断未测试的任意代码执行。
- 💡 修复：为可复制命令采用终端安全且能还原精确参数的编码，或在身份无法安全复制时提供明确替代操作；
  不得仅删除字符后指向另一个会话。查询和保存/撤销提示共用此边界，回归 C0/ESC 与原始 JSON identity。
  依据：[CLI 布局与安全输出合同](../ux/cli.md#5-布局宽高与退化)；外部终端参考页原会话返回 403，
  未把该页当作已核验来源，具体 shell 编码机制须在修复时验证。

**ASC-CLI-R3-F3 · P2 · OPEN — 重放旧 clear 操作时文本误示当前覆盖已清除。**

位置：[session_origin.go:678](../../../../cmd/agentdeck/session_origin.go#L678)。
普通 clear 路径没有把 `Receipt.Replayed` 传给 renderer 的 historical 标记；只有 `--request` 查询路径启用该标记。

- 证据：`TestReviewCLIClearSevenScenariosAgainstService` 在无 sessions 库的隔离 core 中建立 revision 4 覆盖；
  requestA 清除得到 saved/revision 5，随后 requestSet 写入 work/revision 6；以原 requestA/expected_revision=4 重试。
  服务与 CLI JSON 均正确重放 revision 5，数据库仍保留 work/revision 6；文本却显示
  `CLASSIFICATION CLEARED / Outcome saved / Revision 5 / Override none / Views: pending publication`，没有
  `RECORDED RECEIPT` 或“并非当前状态”的说明。该断言失败，探针 4.08s。
- 违反 CLI §3 明确要求重放回执标为历史操作结果。持久化幂等正确，但用户可能将旧结果当成当前清除成功。
- 💡 修复：普通重放路径使用已存在的历史回执呈现，明确旧 operation 与当前状态的区别，
  不对重放回执重新声称当前 publication pending；核对 set/scope 的同类重放入口。
  依据：[已有 read-back 分支](../../../../cmd/agentdeck/session_origin.go#L716)及 [CLI 回执合同](../ux/cli.md#3-命令与可观察输出)。

**ASC-CLI-R3-F4 · P2 · OPEN — 首次浏览器读取失败直接退出，未呈现可退出错误页。**

位置：[main.go:2685](../../../../cmd/agentdeck/main.go#L2685)、[main.go:2759](../../../../cmd/agentdeck/main.go#L2759)。
交互入口在进入终端 owner 前打开 core 并同步读取初始范围；错误直接返回，只有已有列表后的切换错误能留在浏览器内。

- 证据：`TestReviewInitialBrowserFailureShowsAnExitableErrorPage` 建立可读 sessions 索引，在 core 文件位置建立目录，
  先断言 `OpenReadOnly` 确实失败；用 80×24 PTY 启动 `session --interactive`。
  在输入任何退出键前命令即返回 `agentdeck.sqlite3: is a directory`，测试 0.02s FAIL，未进入错误页。
  原较弱夹具被旧 WAL 掩盖所得的 PASS 已撤回；只采用强化前置条件后的失败结果。
- 违反 CLI §4“没有旧结果则错误页，q/Escape 退出”。开发记录已承认这一剩余边界，但没有用户豁免，不能作为通过依据。
- 💡 修复：让初次可用性失败进入同一个可取消终端 owner/error 状态，保留无可信结果语义与安全诊断，
  验证 q/Escape、初次阻塞取消和终端恢复；不要伪造空列表。
  依据：[CLI 状态转换合同](../ux/cli.md#4-状态与转换)及已修复的浏览器加载生命周期。

### 🟡 建议改进（推荐）

无另行建议项。扫描失败探针的证据限制列于矩阵，不虚构第五项产品缺陷。

### 🟢 优点

- **ASC-CLI-R1-F1 CLOSED**：Round 2 的原反例和五项加载/取消/恢复回归继续有效，相关三个 blob 未变。
- 默认偏好并发 CAS、相同 request ID 幂等，以及 clear 的七个持久化场景与独立服务 oracle 一致。
- 正常 PTY 切换、尺寸退化、resize、q/Escape/Ctrl-C/EOF 和终端恢复均有通过记录。

### 📝 总结

四项新发现均 OPEN，原 ASC-CLI-R1-F1 保持 CLOSED。完整任务评审为 FAIL；不改变 Round 2 的限定 PASS。

#### 精确内容状态与复用

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`。
续接入口 14 路径指纹：`28201a165bf55dede4e2237ef2704b31cd89209c6d5ae13a71058e972bb31494`，
配方、路径集合与 Round 1/2 一致；逐 blob 比较没有差异。
13 个产品/测试/fixture blob 与 Round 2 一致；tasks.md 入口 blob 为 `ec99321cea64b020dd00a8408e0543341e727678`。
Go 1.27.3 darwin/amd64；原验证显式绑定相同 GO_TEST_BIN/GOROOT。
go.mod、go.sum、vendor/modules.txt 三个 blob 续接时均与 Round 1/2 相同。
本次仅追加任务和评审记录；产品验证复用，最终内容指纹与门禁在下方补记。

#### 任务级证据矩阵

| 准则 | 证据与独立判定 | 结果 |
| --- | --- | --- |
| session-scope-reads | 列表/搜索、默认范围、缺 core/index、信号/消耗隔离的既有命令测试通过；空时间窗反例 F1 失败 | FAIL |
| classify-mutations | 查询、set、clear、list、read-back、保留字 ID、冲突命令测试通过；clear 七场景持久化与服务 oracle 一致，但历史文本 F3 失败 | FAIL |
| scope-preference | get/set/读取失败回归通过；双写不同 ID 仅一个 saved/另一个 conflict，相同 ID 两个 saved 仅一个 replay，最终 revision=1 | PASS |
| origin-source-commands | 状态与 decision table 回归通过；补充只读探针在外部根不可读、无效 token、监听端口已占用时成功返回，core/config/token 字节不变，输出无路径/端点/token | PASS |
| json-backward-compat | 列表/搜索旧形状与 GUI fixture、signals envelope/共享派生回归通过；扫描失败的部分结果契约仍未证实 | NOT_VERIFIED |
| interactive-pty | 三档尺寸、40 列退化恢复、正常切换/详情、q/Escape/Ctrl-C/EOF、取消/恢复通过；初次失败错误页 F4 失败 | FAIL |
| l3-privacy-concurrency | `-race` 的默认偏好 CAS、clear 服务 oracle、PTY 状态字节检查通过；分类文本控制字符 F2 失败 | FAIL |
| independent-review | 精确候选完整审查已形成本轮 FAIL，不能满足 required PASS | FAIL |

扫描探针边界：`TestReviewScopedUsagePreservesScanFailureAndNoScan` 先追加源文件再 chmod 000，并断言直接 ReadFile 失败；
signals/stats 实际均返回 partial=false、warnings=[]，四条断言失败（0.79s），`--no-scan` 分支与 core generation 不变断言通过。
但这只证明源文件不可读，没有证明扫描入口把该条件归为本合同所说的 scanErr；当前 diff 保留了 scanErr 到
partial/scan_incomplete 的映射。未建立对基线的差异因果，不登记新缺陷，也不把 JSON 准则标 PASS。
后续补证应在扫描边界建立确定的 scanErr，再检查两个 envelope；本轮不继续扩大诊断。

#### 已执行检查及测试处置

所有验证均使用 `scripts/run-go-test.sh`，Go overlay 仅注入工作树外诊断探针；生产代码、测试和配置未改。
本记录保存重建步骤、断言、输出与内容身份，不将临时文件路径当作证据引用。

| 批次 | 结果 | 处置 |
| --- | --- | --- |
| 14 个现有范围/分类/偏好/来源/信号命令测试，加 F1/F2 两个独立探针 | 14 PASS、2 FAIL，包 4.946s，exit 1 | 既有 KEEP；F1/F2 修复时 ADD 回归 |
| `-race`：并发 scope CAS、clear 七场景、浏览器几何/退出矩阵及四项既有详情/恢复测试 | 6 个顶层 PASS、clear 历史文本 1 FAIL，包 21.797s，exit 1 | KEEP；F3 修复时 ADD 回归；clear 其余持久化断言通过 |
| GUI JSON fixture、signals envelope/共享派生、origin status 隐私与外部 I/O 探针 | 4 PASS | KEEP；只读来源补充探针可在修复交付时择要持久化 |
| 强化故障前置条件后的 scan/初次浏览器两个探针 | 2 FAIL，包 2.697s，exit 1 | scan 为 UNPROVEN；F4 修复时 ADD 回归；不复用旧弱夹具 PASS |
| Round 2 加载取消与原反例 6 项 | 复用原 PASS，相关代码未变 | KEEP，无重复执行 |

PTY 几何/退出子矩阵：48×10/q、80×24/Escape、120×40/Ctrl-C、80×24/EOF 均 PASS；
80×24→40×10→48×10 的 resize/too-small 恢复 PASS。该矩阵验证既定组合，不宣称全部尺寸与退出键的笛卡尔积。
无跳过项，无产品全量测试重跑；开发者全量/vet/race 记录作为历史背景，不冒充本轮新证据。
状态文件前后字节及 term state 比较验证只读与恢复，不用测试数量代替契约证明。

#### 审查方法闭合与剩余边界

`ln-12-delivery-reviewer` Checklist：46/54 complete。这是审查方法覆盖数，不是产品验收通过率。
按编号闭合，未保留 PENDING；没有委派，CLOSE-1 的 panel 条件为 CLEARED。

| 条目 | 处置与证据 |
| --- | --- |
| SCOPE-1–8 | PROVEN：用户范围、任务 4/CLI 合同、精确候选/只读边界、隔离状态与本轮方法 |
| TRACE-1–2、5–9 | PROVEN：八准则矩阵、改动入口与回执/选择调用链、命令和并发/PTY 记录 |
| TRACE-3–4 | UNPROVEN：F1/F3/F4 证明完整用户结果及必需行为未实现；修复后复核对应入口 |
| DESIGN-1–3、5–6、8–9、11 | PROVEN：安全输出、core-only clear、共享选择/写服务、取消资源收束与差异核对；本轮不增加机制 |
| DESIGN-4、10 | CLEARED：本候选未新增 schema migration 或独立通用基础设施，复用已交付服务与现有 context/终端机制 |
| DESIGN-7 | UNPROVEN：四项发现阻止声称根因边界完整；修复后验证全部已列受影响入口 |
| VERIFY-1–3、5–6、10–13、15 | PROVEN：上述风险/测试处置与命令结果、无跳过、文档 SSOT、来源隐私及恢复证据 |
| VERIFY-4 | UNPROVEN：现有 PTY 探针按终端文本等待帧，未具备该辅助方法要求的独立稳定交互定位证据；仅作为方法限制，不另造产品 finding |
| VERIFY-7 | UNPROVEN：本轮在决定性失败下未重新执行完整构建/vet/全量；修复后依最终影响及可复用证据选择必需门禁 |
| VERIFY-8 | UNPROVEN：scan 探针尚未确立 scanErr 与改动因果；以明确扫描边界失败补证，不能声称该项通过 |
| VERIFY-9 | UNPROVEN：初次失败用户流程 F4；修复后验证错误页与退出恢复 |
| VERIFY-14 | UNPROVEN：stable manual/design 在主题收尾对账，当前不声称已完成该后续范围 |
| CLOSE-1–11 | CLOSED：CLOSE-1 CLEARED；其余 PROVEN，按本轮事实完成去重/因果/严重度/结论核对。外部参考不可读的编码方案明确未验证，保留合同要求而不指定未经核验的机制 |

最终 14 路径指纹：`2d2d7808184aad9b3d87af2d88c699d3e8b1c2441c5452aeddb6377dfd6aef69`。
仅 tasks.md blob 变为 `b3bbb6b1f1b2094a1dc8ffa76f090a90ecd65d9e`；13 个产品/测试/fixture blob 均未变。
CEv1 ContentState：`auxiliary-session-classification:cli-contract:r3-20261105-resume-01a1ab82:state`。
新写 9 个节点并按完整 attributes_json 精确读回；24 条关系全部预检 ok，创建数量 24。
八项准则分别得到 2 PASS、5 FAIL、1 NOT_VERIFIED；规范门禁返回 FAILED，reason=null，
invalidated_evidence 和 unresolved_candidate_impacts 均为空。历史 Round 1/2 状态与观察保持原样；
新观察通过 depends_on 保留旧候选身份，并明确仅状态文档变化、原产品检查可复用的影响判断。

完成门禁：FAILED

Beads 已追加交接评论 `a3673811-3d97-552a-b908-304660c464dc`，并在 assignee=codex、status=in_review
双守卫下更新为 in_progress、解除指派、round-3；两次操作均 exit 0，返回对象确认目标状态与评论内容。
首次沙箱内读取发生 Dolt socket 超时，改用获准的本机访问后成功；未修改服务、权限或部署配置。

修复范围为 F1–F4；扫描部分结果与其余未证实项保留在任务验收矩阵，不能降低标准或借局部 PASS 关闭任务。
后续 desktop-menubar 等任务依赖不解除，本轮不跨 Topic 完成边界。

修复：auxiliary-session-classification / reviews/cli-contract.md / ASC-CLI-R3-F1、ASC-CLI-R3-F2、ASC-CLI-R3-F3、ASC-CLI-R3-F4
WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Repair 2 — 2026-11-06

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-CLI-R3-F1–F4 及其直接回归；未处理未登记问题。
- Workspace：`agent-deck.auxiliary-session-classification`，`feature/auxiliary-session-classification`，HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`。
- 复现：逐项核对 Round 3 指出的源码位置，四项根因均与记录一致；F1 新回归在去除修复的同一工作树上失败
  （`stats unknown_count = 1/1, want 1/0`），与 Round 3 观察相同，恢复修复后通过。

**ASC-CLI-R3-F1 → 修复，待复评。**

- 变更：`scopedSignals` 新增 `window sessionorigin.Selection` 参数，把已解析的报告 `[from, to)` 与 period
  （显式 `--from/--to` 时 period 为空）传入共享 `Reader.Select`；`usage stats` 与 `usage signals` 都经新的
  `usageSelectionWindow` 传入。时间窗因此进入 unknown_count 分母与 Selection key/publication_id。
- 信号过滤不变的依据：`assemble` 在应用 `Selection.includes`（client/时间/范围）之前就收集 auxiliary 集合，
  `Snapshot.SessionFilter()` 只由该集合派生，所以加入时间窗不会丢弃最后活动在窗后、但窗内有事件的会话；
  消耗块仍为 all，usage-only 语义未改。
- 回归 `TestUsageSelectionMetadataFollowsTheReportWindow`：仅在 2026-08-29 活动的 unknown 会话，
  stats `--from/--to` 为 08-29/08-30 时 totals.sessions 1/0（窗口前置）、unknown_count 1/0、publication_id 不同；
  signals `--period all/today` 的 unknown_count 1/0、publication_id 不同。

**ASC-CLI-R3-F2 → 修复，待复评。**

- 变更：新增 `terminalSafeShellArg`。值不含 C0/DEL/C1 控制字符或非法 UTF-8 时保持原单引号形式（既有输出不变）；
  否则用 ANSI-C 引号 `$'…'`，控制字节和非法字节写为 `\xHH`，`\` 与 `'` 加反斜杠，其余字符原样保留，打印形式不含控制字符。
  含 NUL 的值无法作为一个 argv 元素，返回不可用，输出 `Command unavailable: … Read the exact target with --format json.`，
  而不是删除字符后指向另一个会话。查询的 Change 与保存后的 Undo 共用此边界。
- 已验证的 shell 机制：在 `/bin/bash` 与 `/bin/zsh` 中执行 `printf %s <编码>`，回读字节与原值逐字节一致，覆盖 ESC/CSI、
  BEL/TAB/LF、C1 U+009B、DEL、非法 UTF-8、引号、反斜杠、CJK 与 `$`/反引号。`$'…'` 不是 POSIX sh/fish 语法：
  在不支持的 shell 中会报语法错误，而不会把参数解释为另一个会话；纯可打印 ID 仍是三种 shell 通用的单引号形式。
- 回归：`TestTerminalSafeShellArgRestoresExactBytesWithoutControls`（真实 shell 往返与 NUL 拒绝）；
  `TestClassificationCommandsEscapeTerminalControls` 以 `control-\x1b[31m-id` 运行查询与 `--set work`，
  `--no-color` 文本不含 ESC/CSI，Change/Undo 为 `$'control-\x1b[31m-id'`；JSON target 仍是完整原始 ID。

**ASC-CLI-R3-F3 → 修复，待复评。**

- 变更：`runClassifySet`、`runClassifyClear`、`runScopeSet` 将 `Receipt.Replayed` 作为 renderer 的 historical 标记，
  重放回执呈现为 `· RECORDED RECEIPT` 并说明并非当前状态，不再显示 `Views: pending publication`。
  `renderScopeReceipt` 的历史分支补上同一说明与 `Current default scope: agentdeck session scope get`。
  同类入口核对的结果：历史分类回执不再显示 Undo，因为其 `--clear` 采用当前 revision 的隐式 CAS，
  会清除此后较新的覆盖，违反 CLI §3“不再次撤销”；`--request` 读回路径因此同样不显示 Undo。
  JSON 回执（含 replayed、publication_pending）未改。
- 回归 `TestReplayedMutationsRenderAsRecordedReceipts`：auxiliary/rev1 → clear requestA/rev2 → set work requestB/rev3；
  重放 requestA 的 JSON 为 replayed/rev2，文本为 `CLASSIFICATION CLEARED · RECORDED RECEIPT`，无 pending；
  重放 requestB 为 `CLASSIFICATION SAVED · RECORDED RECEIPT`，无 Undo/pending；当前覆盖仍为 rev3；
  scope set 重放同样为历史回执。新保存仍显示 pending 与 Undo（既有 `TestSessionClassifyQuerySetAndConflictContracts` 通过）。

**ASC-CLI-R3-F4 → 修复，待复评。**

- 变更：新增 `runSessionBrowserLoadingFirst`，交互入口不再在 terminal owner 外同步读取初始范围；
  初次读取复用 R1-F1 的可取消后台加载。状态新增 `untrusted/unavailableScope`：从未有结果时失败进入错误页
  （`Sessions for scope <scope> could not be read.`、`No trusted result is available. Diagnose: agentdeck doctor`、
  `s scope · q/esc quit`），不显示计数、空列表或伪造零值；q/Escape 退出，s 可重新选择范围；初次加载中 Escape 无旧列表可回，直接退出。
  错误页退出后在终端恢复之后返回原读取错误，诊断经既有路径写 stderr，退出码不声称成功。
  `session --interactive` 打开 core 用于详情 usage 服务失败时不再提前返回，由浏览器错误页报告；详情页沿用已有的 usage unavailable 分支。
  `session show --interactive` 与非交互命令的 core 打开错误语义不变。
- 回归（真实 80×24 PTY）：`TestSessionBrowserPTYInitialReadFailureShowsAnExitableErrorPage` 沿用 Round 3 前置条件
  （删除 core 及 WAL/SHM 后在原位置建目录，先断言 `OpenReadOnly` 失败），经 `newRootCommand` 运行 `session --interactive`；
  q、Escape 两个子用例均观察到 alternate screen 内错误页、退出键前 owner 未返回、无伪造结果，按键后观察到终端恢复且返回错误；
  `TestSessionBrowserPTYInitialBlockedReadCanQuit` 在遵守 context 的阻塞初次读取期间以 q/Escape 退出，owner 返回前读取已取消并收束。

### 验证

Go 1.27.3 darwin/amd64，均使用 `scripts/run-go-test.sh`：

| 批次 | 结果 |
| --- | --- |
| 六个新回归（F1–F4，含 4 个 PTY 子用例） | 全部 PASS |
| F1 回归在去除修复的工作树上 | FAIL，`unknown_count = 1/1`；修复恢复后 PASS |
| `./cmd/agentdeck ./internal/sessionorigin ./internal/usage ./internal/output` 全包 | ok（173.9s / 9.0s / 42.6s / 3.2s） |
| `-race ./cmd/agentdeck -run 'SessionBrowser\|Replayed\|ClassificationCommands\|UsageSelectionMetadata\|TerminalSafeShellArg\|SessionClassify\|SessionScope'` | ok |
| `go vet ./cmd/... ./internal/...`、`go build ./cmd/...`、改动文件 `gofmt -l`、`git diff --check` | 通过 |

F2–F4 未逐项做去除修复的变异运行；它们的断言直接检查 Round 3 观察到的失败输出（原始 ESC、缺少 RECORDED RECEIPT、
在错误页前返回）。Round 3 中尚未证实的 scan 部分结果项（json-backward-compat 的 NOT_VERIFIED）不是本轮 finding，未处理。

候选 blob：main.go `ac5d94908fc30e2cbd1262e8a56fd2ce011badb4`、session_browser.go `739db6cef615fc300444d01a2042fa050a302916`、
session_origin.go `52f310e428801846a573ff0e2fd32fe8c04934e5`；新增 session_origin_repair_test.go `af22fad2c41fe6f4ee1d0c4244a26c88960c30e3`、
session_browser_initial_pty_darwin_test.go `8088d71943370a9cbb6db36aead5d29800bb6085`。其余 Round 3 路径未变。
新增的两个测试路径加入候选路径集合，复评时按新集合计算指纹。

修复后状态：ASC-CLI-R3-F1–F4 待复评关闭；R1-F1 保持 CLOSED。任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 4 — 2026-11-06

### 📋 ASC-CLI-R3-F1–F4 限定复评

📊 综合评分：7/10

✅ 复评结论：FAIL

- Reviewer：Codex；Repair 2 由另一会话的 claude-code 实施，复评未委派。
- Scope：真实用户明确选择 ASC-CLI-R3-F1/F2/F3/F4 及修复影响路径；不扩展为新的全任务审计。
- Method：复用既有合同与历史证据，核对 Repair 2 blob、工作区独立 CodeGraph、实际调用链；
  单批 `-race` 定向回归加一个真实 PTY 反例；仅对两个失败测试做一次有界诊断对照。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`；Repair 2 交接 `f85dea99-d3d2-587f-bc77-873ef1bba338`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`；correlation `tdhmbuDjm1Uwo3od`。
- 生产代码、仓库测试、配置只读；无提交、推送、环境修复或后续任务开发。

### 🔴 严重问题（必须修复）

**ASC-CLI-R3-F4 · P2 · 部分修复，仍 OPEN — sessions 库打开失败仍绕过初始错误页。**

位置：[main.go:2675](../../../../cmd/agentdeck/main.go#L2675)。
`withSessions` 仍在 terminal owner 启动前调用 `store.OpenSessions`，失败直接返回；
Repair 2 只让 core 打开失败和 owner 内的 selection 读取失败进入错误页。

- 证据：`TestReviewInitialSessionsFailureShowsAnExitableErrorPage` 在隔离状态中保留可读 core，
  删除 sessions 主库/WAL/SHM 并在 `sessions.sqlite3` 位置建立目录，先断言 `OpenSessionsReadOnly` 失败。
  通过真实 80×24 PTY 和 `newRootCommand` 启动 `session --interactive`，未输入退出键即返回
  `sessions.sqlite3: is a directory`；反例 2.45s FAIL。失败发生在 250ms 等待窗口之前，
  探针总耗时包含 fixture 初始化，不能把 2.45s 当作退出延迟。
- 对照：新增的 core 不可读错误页测试 q/Escape 两个子用例 PASS（共 3.40s），证明修复有效但不完整。
- 合同仍为 [CLI §4](../ux/cli.md#4-状态与转换)“没有旧结果则错误页，q/Escape 退出”；
  同一初始可用性失败因发生在 sessions 打开阶段就直接退出，用户没有已批准的错误页和恢复入口。
- 💡 修复：将交互浏览器初始化所需的 sessions 打开/读取失败纳入同一个 owner/error 状态，
  在有效终端内保持 q/Escape 可退出且无伪造零值；非交互命令的失败语义保持其合同。
  以 core 与 sessions 分别不可读的端到端 PTY 对照证明边界覆盖，不只修内部 loader 的模拟路径。
- 处置：保留原 finding ID；不是新增第五项产品需求，也不重新打开已通过的 core 分支。

**ASC-CLI-R4-F1 · P2 · 新发现 OPEN — 初始阻塞测试误把合法标题识别为伪造计数，尚未执行退出断言就失败。**

位置：[session_browser_initial_pty_darwin_test.go:105](../../../../cmd/agentdeck/session_browser_initial_pty_darwin_test.go#L105)。
测试将 `strings.Contains(frame, "SESSIONS ·")` 作为失败条件，但正常标题本来就是
`AGENTDECK · SESSIONS · READ ONLY`。q/Escape 两个子用例均在第 105 行失败，尚未发送退出键。

- 原始日志同时含 `Requested scope: work`、`Loading committed sessions…` 和 `q quit · esc cancel`；
  没有计数或列表，不能据此认定产品伪造结果。该测试失败是确定的文本匹配错误，不是取消机制的运行反例。
- 有界对照仅在外部 overlay 中把误匹配条件改为 `strings.Contains(frame, "0 SESSIONS")`，
  其余启动、阻塞、q/Escape、取消收束和退出断言原样保留，两个子用例通过。
  该对照用于区分失败归属，不把更窄的诊断断言当成最终测试设计或已修复的仓库测试。
- 风险：新增回归在正确加载画面上恒失败，Repair 2 的“六个新回归全部 PASS”不适用于当前 blob，
  也不能证明原声称的初次阻塞取消覆盖。
- 💡 修复测试的行为断言，使其区分合法标题与真实列表/计数；保留输入退出键、取消收束和终端恢复断言，
  在最终仓库内容上重跑原测试，不得跳过或仅删除有价值的断言。

**ASC-CLI-R4-F2 · P2 · 新发现 OPEN — 时间窗回归缺少 usage 初始化，未到达目标断言即失败。**

位置：[session_origin_repair_test.go:32](../../../../cmd/agentdeck/session_origin_repair_test.go#L32)。
Round 3 原反例在 `session scan` 之后执行过 `usage signals --period all --client claude`，
以建立可读 usage/serving 状态；新回归省略该前置，直接运行 `usage stats --no-scan`。

- 原始执行在 helper 第 37 行得到 exit 1、`runtime_error`、`session scope work is unavailable: generation_unavailable; run agentdeck doctor`，
  因而未到达 sessions=1/0、unknown_count=1/0 或 publication_id 断言。不能将它解释为 F1 的旧元数据缺陷再次复现。
- 有界对照仅在外部 overlay 中恢复上述原反例的 usage 初始化命令，保持两个 stats 窗口及 signals all/today
  的全部断言不变；测试通过。当前产品确实得到 unknown_count=1/0 和不同 publication_id。
- 分类：对本时间窗实验而言，测试缺少能到达所选行为的 fixture 前置；没有据此断言一般的
  generation_unavailable 行为都正确，也未扩展调查另一个生产问题。
- 💡 修复测试前置，使所需的已提交 usage 状态在 `--no-scan` 前建立并可确认，再保留原窗口分离断言。
  不用宽泛重试或移除 `--no-scan` 掩盖前置问题；最终仓库测试必须真实通过。

### 🟡 建议改进（推荐）

无。上述两个测试问题属于本轮修复直接新增的验证路径，不是无关的测试质量审计。

### 🟢 优点

- **ASC-CLI-R3-F1 CLOSED**：两个 usage 入口均把已解析的 Period/From/To 交给共享 Reader；
  保留原反例初始化前置后，stats 的两个日期窗口和 signals all/today 均得到正确 unknown_count 与不同 publication_id。
  工作分析/消耗隔离与 usage-only 现有回归均 PASS。测试前置缺陷单列为 R4-F2，不用它维持已经消失的产品 finding。
- **ASC-CLI-R3-F2 CLOSED**：查询 Change 与保存 Undo 采用同一终端安全参数编码；
  bash/zsh 实际字节往返覆盖 C0/C1/DEL、非法 UTF-8、引号、反斜杠和中文，NUL 拒绝；
  命令输出无原始 ESC，JSON identity 保持原值，两项回归 PASS。
  本结论限于已测 bash/zsh，不重复 Repair 2 对其他 shell 失败方式的未验证推断。
- **ASC-CLI-R3-F3 CLOSED**：set/clear/scope 重放均显示 RECORDED RECEIPT 与历史说明；
  不再显示当前 pending 或历史 Undo，较新覆盖 revision 保留，新保存的既有回执回归 PASS。
- **ASC-CLI-R1-F1 保持 CLOSED**：五项既有加载取消/迟到丢弃/正常切换/owner 收束测试全部 PASS，
  没有把新增测试的错误断言当成旧产品缺陷复发。

### 📝 总结

Finding disposition：R3-F1/F2/F3 CLOSED；R3-F4 部分修复、仍 OPEN；R4-F1/R4-F2 新增测试发现 OPEN；
R1-F1 保持 CLOSED。本轮 FAIL，尚未到达任务提交检查点。Round 3 的 scan 部分结果缺口未在本轮授权范围内补证，
继续保持未验证；不因其他修复通过而升级整个 cli-contract 任务。

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`；入口 16 路径指纹
`b961319c1e584940a33767d8bcfd69059c3979a9bdbb006a6f368bc2803596b6`。
配方为 SHA-256(`head=<HEAD>` 行加 LC_ALL=C 排序的 `path=<git blob>` 行，末尾 LF)；
在 Round 3 的 14 路径集合上加入两个新增测试文件，评审记录自身不参与指纹。
产品修复 blob 与 Repair 2 完全一致：main.go `ac5d94908fc30e2cbd1262e8a56fd2ce011badb4`、
session_browser.go `739db6cef615fc300444d01a2042fa050a302916`、
session_origin.go `52f310e428801846a573ff0e2fd32fe8c04934e5`；新增测试 blob 分别为
session_origin_repair_test.go `af22fad2c41fe6f4ee1d0c4244a26c88960c30e3`、
session_browser_initial_pty_darwin_test.go `8088d71943370a9cbb6db36aead5d29800bb6085`。
其余 10 个产品/测试/fixture blob 与 Round 3 不变；入口 tasks.md blob 为 `ae15b5e53f57c6dc6148c9e6c97f5225421de37e`。
go.mod/go.sum/vendor/modules.txt 与前三轮一致；实测工具链 Go 1.27.3 darwin/amd64，
命令显式绑定相同 GO_TEST_BIN/GOROOT。

验证只执行两批，均经 `scripts/run-go-test.sh -race`，第二批是改变诊断变量后的有界对照，不是原命令重试：

| 批次 | 范围与结果 | 含义 |
| --- | --- | --- |
| 原始候选 + sessions 不可读 PTY 外部探针 | 16 个顶层测试：13 PASS、3 FAIL、0 SKIP；包 32.972s，exit 1 | F2/F3 与既有影响路径通过；F4 原始反例失败；两个新测试各自提前失败 |
| 仅替换外部测试副本的两个诊断变量 | 时间窗测试加原 usage 初始化；初始阻塞测试仅消除标题误匹配；2 个顶层 PASS、0 FAIL、0 SKIP，包 6.378s，exit 0 | 区分产品行为与新测试缺陷，不能冲销原始失败或声称仓库测试已修复 |

第一批六项 Repair 2 新测试、五项既有加载/取消测试、四项既有分类/偏好/usage 测试，
再加本轮 `TestReviewInitialSessionsFailureShowsAnExitableErrorPage`。完整过滤集合可按上述测试名在当前代码定位；
新增反例重建方法：同包复用 `seedOriginSessions`、`openSessionBrowserPTY80x24`、`startSessionBrowserPTYOwner`，
按 F4 描述建立 sessions 不可读前置；启动后 250ms 内若 owner 返回即 FAIL，否则确认错误页后发送 q，
验证 terminalLeaveScreen 与有错误的 owner 退出。工作树外 overlay 只用于诊断，不写入产品测试。

证据边界：没有运行全量构建/vet/全包或扩展审计；决定性 F4 反例后只完成已选发现的失败归属核对。
Repair 2 声称的全量/vet/race 通过保留为历史交接，当前两个新增测试的原始 FAIL 必须保留，不能借历史记录覆盖。
最终 16 路径指纹：`28bba54a3973c79901f6910876c2b7ea8952ad1e85514881c37c5935a31717f4`；
仅 tasks.md blob 变为 `e03a1e5da259e067828300d9e5bf167da0866a35`，其余 15 路径与入口逐 blob 相同。
ContentState：`auxiliary-session-classification:cli-contract:r4-20261106-tdhmbu-final`。
本轮 6 个最终状态/观察节点均完整精确读回，15 条关系预检全为 ok、实际创建 15。
规范门禁返回 FAILED、reason=null、invalidated/unresolved 均为空；当前反证落在 interactive-pty 与 independent-review。
F1/F2/F3 的限定通过观察明确记录其关闭事实，对完整任务 criterion 使用 not_verified，避免将指定 finding 的复评
冒充整个任务的再次验收。其他准则的历史记录不删除、不改写，也不未经核验就换绑到新候选。

完成门禁：FAILED

Beads 交接评论 `d2c49f79-aacc-5257-a739-7959b044d6e2` 已写入；在 codex/in_review 双守卫下
回交 in_progress、解除指派并标记 round-4。原始失败与诊断通过分别保留，后续任务依赖不解除。

修复：auxiliary-session-classification / reviews/cli-contract.md / ASC-CLI-R3-F4、ASC-CLI-R4-F1、ASC-CLI-R4-F2
WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Repair 3 — 2026-11-06

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-CLI-R3-F4、ASC-CLI-R4-F1、ASC-CLI-R4-F2；未处理未登记问题。
- Workspace：`agent-deck.auxiliary-session-classification`，`feature/auxiliary-session-classification`，HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`。
- 复现：R4-F1 在 `NO_COLOR=1` 下原样复现（两个子用例在旧第 105 行失败，帧内容与 Round 4 一致）；
  Repair 2 在有色输出下通过，是因为标题被 ANSI 样式拆开，误匹配条件未被触发，所以那次 PASS 依赖环境。
  R4-F2 在本机含 `-race` 时未复现 `generation_unavailable`；按 Round 4 的诊断对照修复前置条件，并在记录中注明未在本机复现。
  R3-F4 sessions 分支经代码核对与变异运行确认（见下）。

**ASC-CLI-R3-F4 → 修复，待复评。**

- 变更：`withSessions` 打开 sessions 索引失败时，仅 `session --interactive` 路径不再提前返回：
  sessions 置空，打开错误经 `sessionIndexOpenErrorKey` 进入浏览器的 `sessionSelectionReader.sessionsErr`，
  作为 `ErrSessionsUnavailable` 的原因随初次读取失败进入同一错误页；退出后的 stderr 诊断仍为
  `session scope work is unavailable: …; run agentdeck doctor`。其他 session 子命令（含 `show --interactive`）的打开失败语义不变。
  Repair 2 的 core 分支保持不变。
- 回归 `TestSessionBrowserPTYInitialReadFailureShowsAnExitableErrorPage` 改为 core/sessions × q/Escape 四个端到端子用例：
  各自删除对应主库及 WAL/SHM 并在原位置建目录，先断言 `OpenReadOnly`/`OpenSessionsReadOnly` 失败；
  经 `newRootCommand` 在真实 80×24 PTY 运行 `session --interactive`，250ms 内 owner 不得返回，随后错误页可见、无伪造结果，
  按键后终端恢复，owner 返回不可用选择错误。
- 测试设计修正：owner 是否提前返回的检查放在读取画面之前。变异运行发现，命令在 owner 启动前失败时没有任何输出，
  PTY master 的读取停在 read 系统调用，不响应读取截止时间（10 分钟测试超时的栈位于 `readSessionBrowserPTYUntil`）。
  调整后，同一变异下 sessions 两个子用例在 0.15s 内以 `browser returned before an exit key: open …sessions.sqlite3…` 失败，core 两个子用例通过；
  恢复修复后四个子用例全部通过。

**ASC-CLI-R4-F1 → 修复，待复评。**

- 变更：删除 `strings.Contains(frame, "SESSIONS ·")`，改用 `assertNoSessionBrowserResult`：先用 `stripStatsANSI` 去除样式，
  再匹配只有已加载选择才会出现的内容（`<n> SESSIONS`、`<n> INDEXED SESSIONS`、`row n/m`、`No indexed sessions.`、
  `… sessions in this selection.`）；标题 `AGENTDECK · SESSIONS` 不匹配。错误页测试也使用同一检查。
- `TestSessionBrowserPTYInitialBlockedReadCanQuit` 增加有色/`NO_COLOR=1` 两种输出模式，每种都跑 q/Escape，
  保留 loading 帧、loader 启动、输入退出键、终端恢复和取消收束断言。
- 新增 `TestSessionBrowserInventedResultCheckSeparatesTitleFromResults`：在有色和无色写法下，
  检查都能识别真实计数、行位置和空结果，同时不把加载页和错误页的标题当成结果。

**ASC-CLI-R4-F2 → 修复，待复评。**

- 变更：`session scan` 之后先运行 `usage signals --period all --client claude`（同步执行自身扫描），
  并断言 exit 0、`partial=false`、`session_selection.availability=available`，确认 usage 状态在 `--no-scan` 前已建立并可读；
  其后的两个 stats 日期窗口和 signals all/today 断言不变。显示时区改用现有 `useUTCDisplayClock`。
  不用重试，也不移除 `--no-scan`。

### 验证

Go 1.27.3 darwin/amd64，均使用 `scripts/run-go-test.sh`：

| 批次 | 结果 |
| --- | --- |
| R4-F1 旧测试，`NO_COLOR=1` | FAIL（复现），两个子用例在第 105 行失败 |
| `-race` 定向：新结果检查单测、错误页 4 子用例、初始阻塞 4 子用例、时间窗回归 | 有色 PASS；`NO_COLOR=1` 下重跑 PASS |
| 变异：仅撤回 sessions 分支 | sessions/q、sessions/escape 快速 FAIL，core 两项 PASS；恢复后 PASS |
| `./cmd/agentdeck ./internal/sessionorigin ./internal/usage ./internal/output` 全包 | ok（168.7s / 4.2s / 36.2s / 1.5s） |
| `NO_COLOR=1 -race ./cmd/agentdeck -run 'SessionBrowser\|Replayed\|ClassificationCommands\|UsageSelectionMetadata\|TerminalSafeShellArg\|SessionClassify\|SessionScope\|UsageSignals'` | ok（43.2s） |
| `go vet ./cmd/... ./internal/...`、改动文件 `gofmt -l`、`git diff --check` | 通过 |

R4-F2 的原失败（`generation_unavailable`）在本机未复现，因此本轮没有“修复前 FAIL / 修复后 PASS”的本机对照；
修复依据是 Round 4 的诊断对照，以及测试中新增的可确认前置断言。

候选 blob：main.go `e8507ab3d9e816e8597399cf3c6a7974bf19980d`、session_origin.go `fe330e9b05f782d4e286a088c325c845b7b1577f`、
session_origin_repair_test.go `abab91906e70a7e842290dda75c8b63e04d1e5b0`、
session_browser_initial_pty_darwin_test.go `e3757275a883e2ba1dca839a6333095709a299e4`；
session_browser.go 仍为 `739db6cef615fc300444d01a2042fa050a302916`。其余 Round 4 的 16 路径未变。

修复后状态：ASC-CLI-R3-F4、ASC-CLI-R4-F1、ASC-CLI-R4-F2 待复评关闭；R3-F1/F2/F3 与 R1-F1 保持 CLOSED。
任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 5 — 2026-11-06

### 📋 ASC-CLI-R3-F4、ASC-CLI-R4-F1/F2 限定复评

📊 综合评分：9/10（本轮限定范围）

✅ 复评结论：PASS

- Reviewer：Codex，独立于另一会话的 claude-code 修复作者；未委派。
- Scope：真实用户指定 ASC-CLI-R3-F4、ASC-CLI-R4-F1、ASC-CLI-R4-F2 及直接影响路径。
- Method：核对 Repair 3 交接与候选 blob，使用当前工作区 CodeGraph 和源码追踪错误传播及测试前置，
  一次 `NO_COLOR=1 -race` 定向验证；仓库产品、测试与配置只读。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`；Repair 3 交接 `29f01559-174a-5b0f-b366-50f0e67b703d`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`；correlation `iw_nNfEU9i3POxjL`。

### 🔴 严重问题（必须修复）

无。本轮三项 finding 均 CLOSED，影响路径内无新增 finding。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

| Finding | 处置及直接证据 |
| --- | --- |
| ASC-CLI-R3-F4 | CLOSED。`withSessions` 仅对 `session --interactive` 保留 sessions 打开错误；通过 context 交给 reader.sessionsErr，nil sessions 在访问数据库前返回 ErrSessionsUnavailable，进入同一个初始错误页。成功打开时才 defer Close；非交互及 show 路径仍走原错误返回。core/sessions × q/Escape 四个真实 PTY 子用例全部 PASS，覆盖不可读前置、退出键前 owner 存活、错误页/doctor 提示、无伪造结果、alternate-screen 退出和不可用错误返回。 |
| ASC-CLI-R4-F1 | CLOSED。结果检查先剥离 ANSI，再识别数字计数、行位置及空结果，不再匹配合法标题；独立正负 fixture 验证有色/无色的真实结果可检出、标题可放行。初始阻塞测试在有色/NO_COLOR=1 × q/Escape 四个子用例中均执行到退出和取消收束断言并 PASS，不靠跳过或删掉退出断言通过。 |
| ASC-CLI-R4-F2 | CLOSED。时间窗测试在 --no-scan 前同步初始化 usage，并明确断言 partial=false 和 session_selection.availability=available；原 stats 日期窗口与 signals all/today 的 unknown_count、publication_id 断言保留且通过。没有增加重试或移除 --no-scan。 |

ASC-CLI-R1-F1 的五项既有 owner/切换/取消/迟到丢弃回归再次 PASS，关闭结论保持。
ASC-CLI-R3-F1 的时间窗断言随本轮测试通过；R3-F2/F3 保留 Round 4 的既有关闭处置，本轮未将其列为新的独立验收范围。
没有以开发者声称的 PASS 替代本轮运行，也没有改写前三轮原始失败与诊断边界。

### 📝 总结

Finding disposition：本轮 R3-F4、R4-F1、R4-F2 CLOSED；此前 R1-F1、R3-F1/F2/F3 保持 CLOSED。
限定复评 PASS 不表示任务整体已完成。Round 3 的 scan_incomplete/partial 验收仍未验证；
完整任务评审及其他可复用准则需要按当前候选做明确的内容影响判断和证据对账，不能自动换绑历史观察。

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`；入口 16 路径指纹
`f97edfa504c741be2d02a7581752600c05a7465d0c8a4e80f347178d582c2a08`。
配方、路径集合沿用 Round 4：SHA-256(`head=<HEAD>` 行加 LC_ALL=C 排序的 `path=<git blob>` 行，末尾 LF)，
评审记录自身不纳入指纹。与 Repair 3 交接一致的变化为：

| 路径 | blob |
| --- | --- |
| cmd/agentdeck/main.go | `e8507ab3d9e816e8597399cf3c6a7974bf19980d` |
| cmd/agentdeck/session_origin.go | `fe330e9b05f782d4e286a088c325c845b7b1577f` |
| cmd/agentdeck/session_origin_repair_test.go | `abab91906e70a7e842290dda75c8b63e04d1e5b0` |
| cmd/agentdeck/session_browser_initial_pty_darwin_test.go | `e3757275a883e2ba1dca839a6333095709a299e4` |

其余 11 个产品/测试/fixture blob 与 Round 4 一致；入口 tasks.md blob 为 `0c9fffb23cd139fda1f5a92c183323b48c7e65b4`。
go.mod/go.sum/vendor/modules.txt 与前轮完全一致。工具链实测为 Go 1.27.3 darwin/amd64，
显式 GO_TEST_BIN/GOROOT 指向同一安装；没有新建测试 overlay、修改产品或调整运行环境配置。

#### 验证

仅运行一次以下定向批次；测试自身切换 NO_COLOR，验证初始阻塞的两种输出模式：

```sh
NO_COLOR=1 scripts/run-go-test.sh -race ./cmd/agentdeck -run '^(TestSessionBrowserInventedResultCheckSeparatesTitleFromResults|TestSessionBrowserPTYInitialReadFailureShowsAnExitableErrorPage|TestSessionBrowserPTYInitialBlockedReadCanQuit|TestUsageSelectionMetadataFollowsTheReportWindow|TestSessionBrowserPTYOwnerCanBeStoppedBeforeTheFirstFrame|TestSessionBrowserPTYSwitchesScopeWithoutWritingAndRestoresTerminal|TestSessionBrowserPTYQuitsDuringABlockedScopeRead|TestSessionBrowserPTYQuitCancelsAndCollectsTheScopeRead|TestSessionBrowserPTYEscapeCancelsLoadAndDiscardsTheLateResult|TestSessionListScopesDefaultsAndJSONCompatibility|TestSessionSearchSeparatesDocumentTotalFromSelectedSessions|TestSessionListWithoutCoreKeepsEverySessionUnknownInWork|TestSessionBrowserPTYOpensNavigatesAndReturnsFromStructuredDetail)$'
```

**13 个顶层测试 PASS，0 FAIL、0 SKIP，exit 0，包耗时 30.860s**；无 race 报告。

| 检查 | 观察 |
| --- | --- |
| 初次不可读错误页 | core/q 4.18s、core/Escape 3.14s、sessions/q 2.41s、sessions/Escape 2.47s；合计 12.21s，包含 fixture 开销，不解释为退出延迟 |
| 初次阻塞取消 | color/q、color/Escape、no-color/q、no-color/Escape 全部 PASS，合计 0.08s；返回前合作 loader 已取消并收束 |
| 结果检查正负 fixture | 合法标题不误报，ANSI 不能掩盖真实计数/行位置/空结果 |
| usage 时间窗 | 3.61s PASS；初始化前置可确认，stats 两日期窗口与 signals all/today 均保留原断言 |
| 其余影响路径 | 五项加载/取消回归、结构化详情往返、列表/搜索合同和缺 core 的 unknown 回退均 PASS |

边界：这是本轮所选失败及影响路径的验证；不宣称全量构建/vet/全包重跑，也不把此次 PASS 当作 scan 部分结果的运行证据。
Repair 3 的开发者变异/全量记录仅保留为历史交接，不伪装成本轮新证据。
主题仍有 desktop-menubar 等后续任务；Beads 明确本任务所属主题，当前 CEv1 未登记 contains 边，
本轮不创建或跨越 Topic 完成边界，不启动后续开发。

最终 16 路径指纹：`bb7196e0aaa8020adea50087a22180ac6ec90c996e7ab1dcdca15dd2362d5470`；
仅 tasks.md blob 变为 `7d2f02a14160d939ba51b0f0b1c9a3945e7949bd`，15 个产品/测试/fixture blob 与入口相同。
CEv1 ContentState：`auxiliary-session-classification:cli-contract:r5-20261106-iw_nNf-final`。
4 个最终状态/观察节点精确读回一致，9 条关系预检均为 ok、实际创建 9。
三条限定观察明确记录 scoped_check_outcome=pass、finding_disposition=closed；完整任务准则仍为 not_verified，
不会把所选 finding 的通过误算为整个任务验收通过。规范门禁返回 NOT_VERIFIED，reason=null，
invalidated/unresolved/当前适用 failed evidence 均为空；历史失败和既有通过观察保留原身份。
八项任务准则尚未全部具备当前目标的完整适用证据；这不表示历史每项检查都失败，也不要求无差别重跑。
下一项工作应先按内容影响复用已有事实，再补齐真正缺少的扫描失败 partial/scan_incomplete 验收及完整任务评审/对账。

完成门禁：NOT_VERIFIED

Beads 交接评论 `3cc74a88-1dd8-53ec-a66d-1e3a5234685b` 已写入；codex/in_review 双守卫更新成功，
任务保持 in_review、解除指派并标记 round-5。主操作返回对象确认该状态；workspace gate lock 警告未影响此次读写成功。

Task checkpoint：ad-auxiliary-session-classification-cli-contract；content_state=bb7196e0aaa8020adea50087a22180ac6ec90c996e7ab1dcdca15dd2362d5470；gate=NOT_VERIFIED。
提交建议：门禁 VERIFIED 后，经用户提交授权，提交 cli-contract 完整任务边界（15 个产品/测试/fixture 路径、tasks.md、本评审记录，共 17 路径）；前置为补齐扫描失败验收、完整任务评审及当前候选证据对账。
推送建议：门禁 VERIFIED 后，补齐扫描失败验收与完整任务证据对账，完成授权提交及提交对象核验，并取得单独推送授权后推送主题分支；远端 ref 待交付时核实。

补齐 cli-contract 任务级门禁：验证扫描失败的 partial/scan_incomplete，并将完整任务评审及可复用证据绑定当前候选。
WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Round 6 — 2026-11-06

### 📋 cli-contract 任务级验收与完整候选证据对账

📊 综合评分：8/10

✅ 评审结论：FAIL

- Reviewer：Codex。真实用户明确要求验证扫描失败的 partial/scan_incomplete，并完成完整任务评审与可复用证据的当前候选绑定。
- Method：确认当前 16 路径指纹与 Round 5 最终状态完全一致，复用 Round 3–5 已执行的独立验证及修复关闭事实；
  沿实际 scanruntime/文本输出调用链增加一个确定的扫描失败实验，不重跑未变的验证矩阵。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`；correlation `cli-gatefill-20261106`。
- 产品、仓库测试与配置只读；本轮只更新评审、状态和证据，不授权修复或 Git 交付。

### 🔴 严重问题（必须修复）

**ASC-CLI-R6-F1 · P2 · OPEN — 扫描失败时 signals/stats 文本仍展示普通结果，未披露 PARTIAL 和旧已提交数据。**

位置：[main.go:4602](../../../../cmd/agentdeck/main.go#L4602) 的 `writeUsageEnvelope` 文本分支。
JSON 分支写入 `partial/warnings`，文本分支只把 data 交给 renderer，随后仅为 `usage.rebuild` 处理 warnings；
`usage.signals` 和 `usage.stats` 没有显示扫描失败状态的分支。scope 提示不能替代扫描完整性提示。

- 合同：[CLI 扫描与范围过滤矩阵](../ux/cli.md#扫描与范围过滤矩阵) 明确要求 signals 扫描失败后继续读已提交派生，
  JSON 为 partial=true、warnings 含 scan_incomplete，文本明确标 PARTIAL 和旧已提交数据；stats 的 warnings/partial
  同样须保留。任务 4 将该最终 CLI 合同列为验收要求。
- 证据：隔离状态先导入一个 Claude usage session，正常 signals/stats 基线均成功；再通过已有测试入口
  `scanRuntimeForceLocal=false` 和 `scanRuntimeExecutable` 指向一个确定不存在的临时 worker 路径。
  先直接调用 `runUsageScanRound`，断言错误确实包含该 fixture 名，确认失败来自真实 worker 启动边界，
  不是把源文件不可读猜作 scanErr；不替换 renderer、返回 envelope 或扫描函数实现。
- signals/stats × work/all 四个 JSON 子用例全部得到 exit=0、partial=true、scan_incomplete、可用 selection，
  每次只有一次 scan request，旧 workflow/totals 与成功基线一致。
- 同一失败条件下两个文本入口 exit=0，stdout 仅显示正常 Scope 和报告，既没有 PARTIAL，也没有 scan_incomplete；
  stderr 为空。signals/text 与 stats/text 均 FAIL。用户无法知道此次扫描失败、显示的是旧提交状态。
- 基线归属：直接检查 HEAD `6ddbe40` 中同一个函数，文本分支已有相同遗漏。
  本记录不声称它由当前 scope 代码引入；它仍是本任务已批准验收明确要求、当前尚未满足的行为，不能因基线存在而标 PASS。
- 💡 修复：在 signals/stats 的文本呈现边界传达 partial/warnings，明确 PARTIAL、scan_incomplete 和已提交旧数据语义；
  保持已通过的 JSON、单次扫描、旧数据保留、--no-scan 与退出码行为。以同一确定扫描失败条件验证文本，
  不只测试 JSON，也不把修复扩展到本任务之外的其他命令。
- 处置：OPEN，承载于本评审记录和现有 cli-contract 任务。此前七项历史发现均保持 CLOSED。

### 🟡 建议改进（推荐）

无。没有把原 Round 3 的 chmod 源文件探针重新解释为有效 scanErr；该历史证据限制继续保留。

### 🟢 优点

- 新实验确认 JSON partial/scan_incomplete 传播正确，失败后的已提交数据可读且与基线一致。
- `stats --no-scan` 不调用扫描边界，partial=false，已提交 totals 与 core serving generation 保持不变。
- Round 5 的 13 项 race 回归及所有历史 finding 的关闭证据在未变的产品内容上继续有效。

### 📝 总结

完整候选评审为 FAIL，原因仅为新确认的 ASC-CLI-R6-F1；Round 5 的限定 REREVIEW PASS 不撤回，
R1-F1、R3-F1/F2/F3/F4、R4-F1/F2 的关闭处置均保留。没有把此处旧缺口另行推给无关任务以获得通过。

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`；入口 16 路径指纹
`bb7196e0aaa8020adea50087a22180ac6ec90c996e7ab1dcdca15dd2362d5470`，与 Round 5 最终状态完全相同。
配方与路径集合沿用 Round 4/5；go.mod、go.sum、vendor/modules.txt 的 blob 亦与前轮相同。
本次测试显式使用 Go 1.27.3 的 GO_TEST_BIN/GOROOT、NO_COLOR=1；所有复用判断均基于该内容状态。

#### 新增扫描失败证据

```sh
NO_COLOR=1 scripts/run-go-test.sh -race -overlay <external-overlay.json> ./cmd/agentdeck -run '^TestGateScanFailureKeepsCommittedUsageAndNoScan$'
```

一个外部诊断测试，包耗时 5.817s、exit 1、无 race 报告；四个 JSON 子用例 PASS、两个文本子用例 FAIL，
测试末尾的 --no-scan 对照仍执行并 PASS。顶层用时 3.84s；子用例分别为 signals/work 0.15s、signals/all 0.14s、
signals/text 0.13s、stats/work 0.17s、stats/all 0.20s、stats/text 0.17s。不存在跳过项。

重建步骤：同包复用 `withUsageSignalsHome`、`writeScopedSignalSession`、`runOriginCLI`，先 session scan 和 usage signals
建立可读状态，分别缓存 signals/stats 在 work/all 下的成功 JSON 基线，并断言 stats 的 sessions=1。
保存并在 Cleanup 恢复两个 scanruntime 测试入口，只对这个隔离 root 使用不存在的 worker 路径；
用 2s context 直接确认 `runUsageScanRound` 返回预期启动错误，再从实际 CLI execute 入口执行上述六个用例。
JSON 逐项检查 scan request 次数、partial、warnings、selection.availability 和旧数据等值；
文本检查 PARTIAL；最后检查 --no-scan 的请求计数与 serving generation 不变。
该方法验证扫描启动失败后的结果呈现合同，不声称覆盖每种采集器失败，也不操作用户真实日志或数据库。

#### 当前候选复用矩阵

| Required criterion | 当前结论 | 支撑与影响判断 |
| --- | --- | --- |
| session-scope-reads | PASS | Round 3 的范围默认/显式选择、List 服务 oracle、搜索分页、缺 core/index、usage-only 和消耗隔离；Round 4 关闭时间窗 F1，Round 5 在最终产品 blob 上重验列表/搜索/缺 core 与时间窗。修复只影响明确列出的错误/呈现路径，既有通过行为保持。 |
| classify-mutations | PASS | Round 3 命令及 core-only clear 七场景与服务 oracle 的持久化结果；Round 4 重放测试关闭历史呈现 F3，旧请求不撤销较新覆盖，read-back 语义保留。服务、store 和对应测试 blob 后续未变；Repair 3 的 reader nil-index 防御不改写 CAS/回执处理。 |
| scope-preference | PASS | 复用 Round 3 已登记的 get/set/read-failure 与并发 CAS/同 ID 重放通过证据；Round 4 的默认偏好回归和历史 scope 回执检查通过。偏好持久化服务未改变；新增历史说明不改变 revision、确认或并发规则。 |
| origin-source-commands | PASS | 复用 Round 3 已登记的状态 decision table、隐私和只读外部 I/O 证据；后续修复为 selection/分类文本/浏览器及其测试，来源状态命令不经过这些新增交互或参数编码分支，来源存储与配置合同未变。 |
| json-backward-compat | FAIL | 既有数组/对象形状与 GUI fixture 通过事实保留；本轮四项扫描失败 JSON 与 --no-scan 对照通过。但此准则声明的 scan/partial 合同包含最终 CLI 呈现要求，文本 F1 未满足，不能只因名称含 JSON 就忽略任务明定的文本要求。 |
| interactive-pty | PASS | 复用 Round 3 三档尺寸、40 列退化恢复、q/Escape/Ctrl-C/EOF 与详情检查；Round 4/5 的当前路径补证覆盖初次 core/sessions 失败、两种颜色模式、加载取消、迟到丢弃与恢复。Repair 3 不修改 browser 主循环/渲染 blob；既有几何与退出证明未因新错误传递分支失效。 |
| l3-privacy-concurrency | PASS | 复用 Round 3 race CAS、clear 服务 oracle、状态字节与来源隐私检查；Round 4 bash/zsh 字节往返和分类命令检查关闭终端控制字符 F2；Round 5 当前 reader/PTY 路径 race 通过。既有核心 mutation、token/config 只读路径未变。 |
| independent-review | FAIL | 本轮把完整任务的历史独立审查、关闭事实及缺口实验归并到当前候选；新验收 finding R6-F1 OPEN，完整任务不能给 PASS。 |

复用是本轮显式的内容影响判断，不把旧 observation 改写到新状态。
来源命令与默认偏好的既有 PASS 采用新目标绑定的 roll-up；其余准则以本记录中的跨轮直接检查事实及修复关闭结果形成新的综合观察，
保留各来源 ContentState 作为依赖。历史 FAIL 和限定 not_verified 观察均不删除，也不作为虚构的 passing lineage。
正式记录中的验证步骤与关键结果足以重建本次判断，不引用临时日志的留存位置。

最终 16 路径指纹：`be462f873186a7bffe624e9960588d844d9a94cdf536db5b2c82dc1798cac54b`；
仅 tasks.md blob 变为 `9d064b6aa0941f23a1da99a1ac4e7360ea701f88`，15 个产品/测试/fixture blob 与入口一致。
CEv1 ContentState：`auxiliary-session-classification:cli-contract:review-20261106-gatefill-r6-state`。
9 个新节点精确读回一致，41 条关系预检全为 ok、实际创建 41；规范门禁返回 FAILED，reason=null，
invalidated_evidence/unresolved_candidate_impacts 均为空。
八项准则均有当前目标绑定的结论：6 项具有适用 PASS，json-backward-compat 与 independent-review 为适用 FAIL。
当前差距已从“缺乏确定的扫描失败证据”收敛为一个明确的文本反馈缺陷；不需要因这次阶段变化重做已证明的六项准则。

完成门禁：FAILED

Beads 交接评论 `68bfdb02-676b-59cc-b729-ba2cd193bd27` 已写入；在 codex/in_review 双守卫下
回交 in_progress、解除指派并标记 round-6，返回对象确认成功。任务未进入提交或 Topic 完成边界。

修复：auxiliary-session-classification / reviews/cli-contract.md / ASC-CLI-R6-F1
WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Repair 4 — 2026-11-06

- Repairer：claude-code（开发者本人，非独立评审）。范围：仅 ASC-CLI-R6-F1；未处理未登记问题，也未扩展到 signals/stats 之外的命令。
- Workspace：`agent-deck.auxiliary-session-classification`，`feature/auxiliary-session-classification`，HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`。
- 复现：沿用 Round 6 的确定失败条件（`scanRuntimeForceLocal=false`，`scanRuntimeExecutable` 指向不存在的 worker），
  新回归在去除修复的代码上 signals、stats 两个子用例均以 `text has no PARTIAL notice` 失败，与 Round 6 文本观察一致。

**ASC-CLI-R6-F1 → 修复，待复评。**

- 依据：[CLI 扫描与范围过滤矩阵](../ux/cli.md#扫描与范围过滤矩阵)，以及原型标本 `signalsScanIncomplete`
  （stdout `PARTIAL: scan_incomplete` / `Showing last committed data.`，stderr `Warning: scan_incomplete`）。
- 变更（`cmd/agentdeck/main.go`）：
  - 新增 `usageScanIncomplete(command, partial, warnings)`。只有 `usage.signals`、`usage.stats` 在 partial 且 warnings 含
    `scan_incomplete` 时成立，与 JSON envelope 的 partial/warnings 同源。
  - `writeUsageEnvelope` 文本分支在 scope 块之后、报告数据之前写出 `PARTIAL: scan_incomplete` 和 `Showing last committed data.`；
    报告本身不变。
  - `withUsage` 在 text 格式下向 stderr 写 `Warning: scan_incomplete`，与同函数已有的 text 覆盖警告写法一致；
    `usage stats --interactive` 不在本 finding 范围内，未改动。
  - `--quiet` 不隐藏该提示，依据 CLI Manual：quiet 只能静默没有 warning 的完整文本结果。
- 保持不变：JSON envelope、单次扫描请求、失败后读取已提交数据、`--no-scan` 不扫描、退出码 0。

回归 `TestUsageTextDisclosesAnIncompleteScan`（`cmd/agentdeck/session_origin_repair_test.go`）：

1. 隔离 home/state 中导入一个 Claude 会话，`session scan` 后用 `usage signals` 同步建立 usage 状态。
2. 在扫描正常时记录 signals、stats 的文本基线，确认其中没有 PARTIAL。
3. 切换到不存在的 worker，先直接调用 `runUsageScanRound`，确认错误来自该 worker 路径。
4. 对 signals、stats，在默认与 `--quiet` 下分别检查：
   - exit 0；
   - PARTIAL 两行出现在 scope 块之后、任何报告数据之前；
   - 去掉这两行后，stdout 与基线逐字节相同；
   - stderr 含 `Warning: scan_incomplete`；
   - JSON 仍为 partial=true、warnings 以 `scan_incomplete` 开头。
5. `stats --no-scan` 不出现 PARTIAL，stderr 也没有 scan_incomplete。

### 验证

Go 1.27.3 darwin/amd64，均使用 `scripts/run-go-test.sh`：

| 批次 | 结果 |
| --- | --- |
| 新回归，`NO_COLOR=1` | PASS（signals、stats 两个子用例） |
| 变异：`usageScanIncomplete` 恒为 false | signals、stats 均 FAIL（`text has no PARTIAL notice`）；恢复后 PASS |
| `./cmd/agentdeck ./internal/sessionorigin ./internal/usage ./internal/output` 全包 | ok（186.9s / 8.1s / 40.5s / 4.0s） |
| `NO_COLOR=1 -race ./cmd/agentdeck -run 'UsageTextDisclosesAnIncompleteScan\|UsageSignals\|UsageStats\|UsageSelectionMetadata\|SessionBrowser\|Replayed\|ClassificationCommands\|SessionClassify\|SessionScope'` | ok（63.1s），无 race 报告 |
| `go vet ./cmd/... ./internal/...`、改动文件 `gofmt -l`、`git diff --check` | 通过 |

边界：只覆盖扫描 worker 无法启动这一种失败，不声称覆盖每种采集器失败。

候选 blob：main.go `5ac26555bf23a8bc6dde34495c5e5d9c2858ebed`、
session_origin_repair_test.go `cc69969ce49d2e450e2d9088c685c966beabac9d`；其余 Round 6 的 16 路径未变。

修复后状态：ASC-CLI-R6-F1 待复评关闭；此前七项发现保持 CLOSED。任务 Review 保持未勾选。未提交、推送或修改全局状态。

## Round 7 — 2026-11-06

### 📋 ASC-CLI-R6-F1 复评与任务检查点

📊 综合评分：9/10

✅ 复评结论：PASS

- Reviewer：Codex，独立于另一会话的 claude-code 修复作者；未委派。
- Scope：真实用户选择 ASC-CLI-R6-F1；复核修复影响路径，并依项目要求复用未变证据完成同一任务的当前候选门禁。
- Method：核对 Repair 4 交接、实际 blob、CodeGraph/源码及输出分支；一次 NO_COLOR=1 的定向 race 验证。
- Dispatch：`ad-auxiliary-session-classification-cli-contract`；修复交接 `d9576167-4d2d-5ff7-847d-efe6bc5cb842`。
- Workspace：`agent-deck.auxiliary-session-classification` / `feature/auxiliary-session-classification`；correlation `KXgVGh3n-8T7hiH8`。
- 产品、仓库测试、配置只读；未提交、推送或启动后续任务。

### 🔴 严重问题（必须修复）

无。**ASC-CLI-R6-F1 CLOSED**；本轮影响范围内无新增 finding。

### 🟡 建议改进（推荐）

无。

### 🟢 优点

- `usageScanIncomplete` 仅在 signals/stats、partial=true 且 warnings 含 scan_incomplete 时触发。
  文本输出在 scope 块之后、报告数据之前显示 `PARTIAL: scan_incomplete` 与 `Showing last committed data.`，
  `withUsage` 的 text 非交互分支向 stderr 写 `Warning: scan_incomplete`；JSON 分支不经过这些文本写入。
- 新回归使用与 Round 6 相同的真实扫描 worker 启动失败边界，先确认缺失 worker 的错误再验证呈现，
  默认与 --quiet 均有提示；去掉新增两行后，stdout 与成功基线逐字节相同，证明仍展示原已提交数据。
- JSON partial/warnings 与 --no-scan 对照通过；共享输出函数的其他正常 usage 和 GUI JSON 合同回归通过。
- 修复只改动 main.go 的警告呈现和一个测试文件，未改扫描请求、mutation、选择归约、来源配置或浏览器循环，
  因此 Round 6 已通过的六项准则可按下述内容影响判断复用。

### 📝 总结

Finding disposition：ASC-CLI-R6-F1 CLOSED。此前 R1-F1、R3-F1/F2/F3/F4、R4-F1/F2 七项保持 CLOSED，
本任务八项已登记发现全部闭合。Round 6 已完成的完整候选评审只剩该项验收失败；本轮关闭它并复用其余证明，
任务 Review 因此勾选，不再重复建立“限定通过但完整评审未做”的缺口。
历史 FAIL 与各限定 PASS 的原始范围仍完整保留。

Reviewed state：HEAD `6ddbe4078beefe0f8981f58119e26ad86d05111b`；入口 16 路径指纹
`a85a928bbb5ef52dba5565a124372f2fbc4ca8c794b2061e234b23c3ecc773b5`。
配方与 Round 4–6 相同：SHA-256(`head=<HEAD>` 行加 LC_ALL=C 排序的 `path=<git blob>` 行，末尾 LF)，
评审记录自身不纳入指纹。Repair 4 两个变化 blob 已实际核对：
main.go `5ac26555bf23a8bc6dde34495c5e5d9c2858ebed`、
session_origin_repair_test.go `cc69969ce49d2e450e2d9088c685c966beabac9d`。
其余 13 个产品/测试/fixture blob 与 Round 6 一致；入口 tasks.md blob 为 `a3b346b649a1f8a9982eacc89b12c28080a35d9d`。
go.mod/go.sum/vendor/modules.txt 的 blob 与前轮一致；实测工具链 Go 1.27.3 darwin/amd64，
命令显式绑定相同 GO_TEST_BIN/GOROOT。

#### 本轮直接验证

```sh
NO_COLOR=1 scripts/run-go-test.sh -race ./cmd/agentdeck -run '^(TestUsageTextDisclosesAnIncompleteScan|TestUsageSignalsCommandUsesEnvelopeAndHasNoTopFlag|TestUsageCommandTextAndJSONContracts|TestUsageSummaryShortcutsAndStatsJSONContract|TestGUIJSONContractFixture)$'
```

**5 个顶层测试 PASS，0 FAIL、0 SKIP，exit 0，包耗时 16.581s，无 race 报告。**

| 检查 | 结果 |
| --- | --- |
| TestUsageTextDisclosesAnIncompleteScan | PASS 4.05s；signals/stats 子用例各覆盖默认和 quiet，提示位置、原报告等值、stderr 警告及 JSON；末尾 --no-scan 对照通过 |
| TestUsageCommandTextAndJSONContracts | PASS 3.57s，共享文本/JSON 正常合同保持 |
| TestUsageSummaryShortcutsAndStatsJSONContract | PASS 3.26s，summary/stats 相关合同保持 |
| TestUsageSignalsCommandUsesEnvelopeAndHasNoTopFlag | PASS 2.88s，signals envelope 与参数边界保持 |
| TestGUIJSONContractFixture | PASS 0.03s，GUI JSON 兼容夹具保持 |

该实验验证 worker 无法启动后、已提交存储仍可读时的失败呈现；不宣称覆盖所有采集器故障。
Round 6 已通过的 work/all JSON、单次扫描、--no-scan 请求计数与 serving generation 证明继续复用：
本次新增谓词与输出写入不调用扫描、不修改数据，相关代码/依赖未变。
不重跑无关全量测试；Repair 4 的开发者全量/vet/race 记录仅作为历史交接，不冒充本轮新运行。

#### 任务级证据复用

| 准则 | 当前判断 |
| --- | --- |
| session-scope-reads | 复用 Round 6 的通过观察；只追加已批准的失败提示，不修改范围、计数、分页、选择身份或消耗隔离，正常输出对照通过 |
| classify-mutations | 复用 Round 6 通过观察；分类查询/写入/read-back、CAS 和回执代码未改 |
| scope-preference | 复用 Round 6 通过观察；确认、revision、并发和重放语义未改 |
| origin-source-commands | 复用 Round 6 通过观察；来源读取、状态判定、隐私和配置持久化路径未改 |
| interactive-pty | 复用 Round 6 通过观察；browser/terminal/PTY 路径及 blob 未改，新增警告明确排除 statsInteractive |
| l3-privacy-concurrency | 复用 Round 6 通过观察；mutation/来源隐私/终端控制字符/并发路径未改；本轮新增输出路径 race 通过 |
| json-backward-compat | 新观察：保留既有 JSON/scan/no-scan 证明，以本轮真实失败边界验证关闭文本缺口，共享格式回归通过 |
| independent-review | 新观察：Round 6 完整任务评审及其六项通过证据保持，唯一未关闭 finding 经独立复核关闭，完整当前候选 PASS |

六项通过使用新目标绑定的 roll-up，保留 Round 6 原观察与内容身份；不把历史失败节点改写为通过。
本轮新 JSON/扫描合同与完整评审观察绑定最终候选，并保留适用的内容依赖。
Beads 当前确认下一任务 `ad-auxiliary-session-classification-desktop-menubar` 为 open/unassigned；
tasks.md 明确其依赖任务 3、4。当前仅完成任务 4 的评审/证据检查点，仍须本任务授权提交交付后解除依赖，
不跨 Topic 完成边界、不创建主题层证据结构。

最终 16 路径指纹：`3d11cbe405ed7145d754202044f0e1d8f47be5c716d65f21eb6eadf2d828e52a`；
仅 tasks.md blob 变为 `4afb7dc631c567e262c4d2250238487a0d2bacbd`，15 个产品/测试/fixture blob 与入口逐一相同。
CEv1 ContentState：`auxiliary-session-classification:cli-contract:r7-20261106-KXgVGh-final`。
9 个新节点精确读回一致，37 条关系预检全为 ok、实际创建 37；规范门禁返回 VERIFIED（8/8），
reason=null，missing_criteria、invalidated_evidence、unresolved_candidate_impacts 和当前适用失败证据均为空。
六项复用与两个新结论已明确绑定该目标；原失败观察保留在其历史内容状态。

完成门禁：VERIFIED

Beads 交接评论 `9a63c6a2-73d8-51a7-8216-de55e8a1db1c` 已写入；codex/in_review 双守卫更新成功，
任务转为 awaiting_commit、解除指派并标记 round-7。该状态不代表已提交或推送。

Task checkpoint：ad-auxiliary-session-classification-cli-contract；content_state=3d11cbe405ed7145d754202044f0e1d8f47be5c716d65f21eb6eadf2d828e52a；gate=VERIFIED。
提交建议：经用户授权后，提交 cli-contract 完整任务边界，共 17 路径（15 个产品/测试/fixture、tasks.md、本评审记录）；不拆分任务记录与实现。
推送建议：完成授权提交、提交对象/共同作者/SSH 签名核验并获得单独推送授权后，推送主题分支；远端 ref 待交付时核实。

后续 desktop-menubar 须先满足本任务授权提交交付的依赖；下列指令不启动该任务，也不授予提交或推送权限。

开发：auxiliary-session-classification / desktop-menubar
WORKFLOW_WORKSPACE: agent-deck.auxiliary-session-classification

## Local delivery — 2026-11-06

用户在 Round 7 检查点后明确授权本地提交。范围为本任务 15 个产品/测试/fixture 路径及 tasks.md、
本评审记录，共 17 路径；不含其他主题或全局状态。产品内容与 Round 7 VERIFIED 候选逐 blob 一致，
仅追加授权交付说明，因此复用原产品验证，仅运行适用的文档检查。

贡献者依据：claude-code 的开发交接 `9dc2d5db-f72f-54ea-aa6e-5656819e5d2e` 及四轮修复交接，
末轮为 `d9576167-4d2d-5ff7-847d-efe6bc5cb842`；Codex 产生本次提交中的评审与状态内容，
末轮交接为 `9a63c6a2-73d8-51a7-8216-de55e8a1db1c`。使用已建立的 Claude 和 Codex trailers，无未决归属。
实际 Git 对象、签名、提交态证据门禁及任务关闭结果在 Beads/CEv1 记录，避免将临时输出当作交付原件。
本授权不包含推送、PR、合并或启动后续开发。
