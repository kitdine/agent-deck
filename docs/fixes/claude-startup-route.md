---
status: active
created: 2026-09-01
---

# 缺陷：Claude startup 会话丢失归因 route

## 现象

Claude 的 `SessionStart` Hook 在 `source=startup` 时可能早于 transcript
文件创建。修复前，AgentDeck 对这种正常时序静默 fail-open，不写入
`usage_session_routes`，导致该会话只能落入推断归因。

聚焦回归测试在旧实现上稳定复现：

```text
scripts/run-go-test.sh ./cmd/agentdeck \
  -run '^TestUsageHookEventAcceptsClaudeStartupBeforeTranscriptExists$'

hook_boundary_test.go:185: startup before transcript wrote 0 routes, want 1
go test failed with status 1
```

## 根因

[`validHookTranscript`](../../cmd/agentdeck/main.go)（`cmd/agentdeck/main.go:3080`）
位于 `SessionStart` route 处理之前。旧实现对 Codex 和 Claude 一律先执行
`os.Lstat(event.TranscriptPath)`，并要求目标已经是普通文件。Claude startup
payload 指向的 transcript 尚不存在时，该函数返回 `false`；调用者在
`cmd/agentdeck/main.go:2971` 随即静默返回，因此 provider selection 和 route
持久化逻辑都不会执行。

## 修复边界

- 仅当 client 为 Claude、`source=startup`，且 `Lstat` 明确返回
  `os.ErrNotExist` 时，允许 transcript 最终文件尚不存在。
- 例外路径仍须解析 `~/.claude/projects` 和 transcript 已存在父目录的真实
  路径，并通过同一 root containment 检查；root 外路径和父目录 symlink
  逃逸仍被拒绝。
- 已存在 transcript 继续要求普通文件且自身不是 symlink。Claude `resume`、
  Codex、`compact` route 排除、ConfigChange、SessionEnd 和 Hook fail-open
  行为均不改变。
- 不修改 SQLite schema、归因优先级、Hook 注册、用户配置或真实会话文件。

## 验证

- RED：聚焦测试在生产代码修改前按上面的命令失败，实际为 `0 routes`，
  预期为 `1`。
- GREEN：同一聚焦测试在修复后通过；随后增加 root 外与父目录 symlink
  逃逸断言，再次通过。
- 相关测试：
  `scripts/run-go-test.sh ./cmd/agentdeck -run '^TestUsageHookEvent'`，PASS。
- L2：`scripts/run-go-test.sh ./...`，PASS。

## Review — Round 1 — 2026-09-02

- Reviewed state: HEAD `5a33522980644a009d3b163d644a008a1bf7943d` 加三个未提交
  blob——`cmd/agentdeck/main.go` `7f10b6dd`、
  `cmd/agentdeck/hook_boundary_test.go` `90b3eaae`、
  `docs/fixes/claude-startup-route.md` `526400a2`。
- Reviewer: `claude-code`。实现由 `codex` 完成，本轮为独立冷上下文评审。
- Method: 通读生产代码与测试的完整 diff；独立执行聚焦测试与相邻包回归；
  对 RED 状态改用逻辑推演而非回退工作区（理由见 Evidence）；按 `ParseEvent`
  接受的 source 全集与 `filepath` 调用链逐条推演未覆盖边界。
- Scope: `validHookTranscript`（`cmd/agentdeck/main.go:3080`）的准入判定及其在
  `runUsageHookEvent`（`:2971`）的调用点，新增用例
  `TestUsageHookEventAcceptsClaudeStartupBeforeTranscriptExists`，以及本记录
  文档自身。未涉及读时归因判定、schema、Hook 注册与用户配置。
- Findings:
  - [P1] **R1-F1** 修复只放行 `event.Source == "startup"`，而 `ParseEvent` 对 Claude
    `SessionStart` 接受 `startup|resume|clear|compact|fork`。若 `clear` 与
    `fork` 同样产生新的 session_id 与新 transcript，它们命中完全相同的时序，
    仍会被静默丢弃。本机 `usage_session_routes` 中 Claude `SessionStart` 只有
    `resume` 一种 source，无法据此区分「从未发生」与「发生了但被拦」，因此该
    缺口既未被证实也未被排除。 -> `ad-bug-hook-transcript-admission-edges`
  - [P1] **R1-F2** 例外分支要求 transcript 的**父目录**已经存在——
    `filepath.EvalSymlinks(filepath.Dir(event.TranscriptPath))` 在目录不存在时
    返回错误，随后 `return false`。全新项目的第一个 Claude 会话，若
    `~/.claude/projects/<project>/` 在 SessionStart 之后才被创建，则仍然丢
    route。新增用例先 `os.MkdirAll(projectDir, 0o700)` 再投递，未覆盖该情形。
    -> `ad-bug-hook-transcript-admission-edges`
  - [nit] **R1-F3** `resolvedPath, err = EvalSymlinks(dir)` 之后无条件执行
    `resolvedPath = filepath.Join(resolvedPath, base)`，`EvalSymlinks` 失败时会
    先用空串拼出一个无意义路径，再由其后的 `if err != nil` 拦下。行为正确，
    但失败处理与路径拼接交错，读者需多看一眼。 -> 不要求修改。
- Evidence:
  - `scripts/run-go-test.sh ./cmd/agentdeck -run '^TestUsageHookEvent'` —— PASS
  - `scripts/run-go-test.sh ./cmd/agentdeck ./internal/usage ./internal/usagehook`
    —— PASS
  - RED 未由本轮独立复现。复现需回退 `cmd/agentdeck/main.go`，而该工作区同时
    承载其他在办改动，回退存在干扰风险。改用逻辑推演：修复前的
    `if err != nil || !info.Mode().IsRegular() …` 在 `Lstat` 返回
    `os.ErrNotExist` 时必然 `return false`，新用例断言 `want 1` 因而必然得到
    `0`，与实现者记录的 `startup before transcript wrote 0 routes, want 1`
    一致。该推演是确定的，但它不等价于一次真实的失败观测，故在此标注。
  - 安全边界未退化：新增用例对 root 外路径与父目录 symlink 逃逸两种情形均
    断言 route 数不变，且 `resolvedRoot` 的解析被前移后仍作用于同一
    containment 检查。
- Completion gate: NOT_REQUIRED —— `.agent-instructions/evidence.md` 定义的四个
  CEv1 边界是 document、task、topic、release，`work_unit_id` 形如
  `<topic>:<task-anchor>`。Lane A 缺陷修复不属于任何 topic，现行 CEv1 模型
  未为其定义门禁。这是 Bug lane 引入时未一并规定的流程缺口，承载于
  `ad-chore-cev1-lane-a-boundary`，不影响本轮判定。
- Verdict: PASS

### Follow-up 承载体

本轮的三条 finding 都不阻塞交付，但都已进入可查询的承载体，不依赖本文件被
再次读到：

| finding | 承载 |
| --- | --- |
| R1-F1 `clear` / `fork` 命中同一时序 | `ad-bug-hook-transcript-admission-edges` |
| R1-F2 全新项目首个会话的父目录时序 | 同上 |
| R1-F3 EvalSymlinks 失败与路径拼接交错 | 不要求修改，本轮关闭 |
| Completion gate 无 CEv1 边界 | `ad-chore-cev1-lane-a-boundary` |
