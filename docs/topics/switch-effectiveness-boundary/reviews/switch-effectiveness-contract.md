---
status: active
topic: switch-effectiveness-boundary
subject: switch-effectiveness-contract
---

# Review log — switch-effectiveness-boundary / switch-effectiveness-contract

## Round 1 — 2026-08-26

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus
  scoped implementation diff fingerprint
  `b9e5ba5249bf6bc5e93ea806471cd6626dcf237a62a1476ca1480277a8acae99`.
  Final synchronized `tasks.md` blob is
  `27e14e0f9de4c13c6ea1215b9952baf5a6283ab8`; `docs/status.md` blob is
  `8639c6864524a57b9c2d456a01d7902e8a598dc0`.
- Reviewer: Codex, independently reviewing the implementation authored by
  `claude-code`; this workflow turn kept production code, tests, configuration,
  and living specifications read-only except for required review/status records.
- Method: Formal code/contract Review under `development-workflow`. CodeGraph
  traced `SwitchAdvisories`, the completed-switch call site, selection read-back,
  reconcile boundary comments, and affected tests. Focused source and contract
  comparison found a decisive completed-switch failure-path contradiction, so
  broad verification stopped. Developer-reported focused build/vet/tests remain
  implementation evidence but were not independently rerun in this round.
- Scope: Contract 1's direction-aware Claude advisories, Contract 2's
  file-versus-running-process boundary, conflict ordering/privacy, Codex
  compatibility, completed-switch success semantics, tests, and living specs.
- Findings:
  - **[P1] S1-F1 — a failed post-switch selection read emits the wrong Claude
    direction instead of dropping the advisory.** `reportSwitchAdvisories`
    (`cmd/agentdeck/main.go:1634-1649`) initializes `hasCredential := false`,
    calls `s.Current(ctx)`, and still invokes `SwitchAdvisories` when that read
    fails or returns no matching client. A successfully completed switch that
    actually wrote a credential can therefore print the credential-free
    “removing a key does not re-authenticate” message. The approved contract
    selects copy from the completed selection's real credential presence
    (`architecture.md:263-272`) and says advisory read failures are best-effort,
    not that unknown may be represented as credential-free. This path is also
    untested: current command tests cover written/free success, conflicts,
    JSON isolation, Codex, and quiet mode, but not selection read-back failure.
- Evidence: CodeGraph found one `reportSwitchAdvisories` caller immediately after
  successful `UseCredential`; its source at `main.go:1638-1647` proves the
  false-default branch. `provider.Service.Current` can return a store error at
  `internal/provider/service.go:695-700`; the helper suppresses that error but
  does not suppress or conservatively select the advisory.
- Completion gate: FAILED — CEv1 gate
  `switch-effectiveness-boundary:switch-effectiveness-contract` bound S1-F1 to
  exact content state
  `71ecc702d1411650c61bd57b4f4e255153d0c38b37b53f1895e05a4ba60cf8cb`;
  the direction-aware advisory and L2/scope criteria are disproved.
- Verdict: REOPEN

## 📋 Switch effectiveness contract 独立评审

📊 总体评分：7/10

✅ 评审结论：FAIL

### 🔴 严重问题——必须修复

[`cmd/agentdeck/main.go:1638`] S1-F1：成功 switch 后的 `s.Current` 读取失败会被
当作 `hasCredential=false`，继续打印 credential-free Claude advisory。
- 行为风险：实际写入 API key 的 selection 可能收到“removing a key”这一方向相反的
billing/restart 指引，破坏本 Task 要修复的核心用户合同。
- 证据：`hasCredential` 先默认为 false；只有 `Current` 成功且找到 client 才覆盖；错误和
缺失 client 都继续调用 `SwitchAdvisories`。
💡 有界修复：从成功 switch 已解析的 selection/config 直接传入 credential presence，
或在 read-back 失败/无匹配 selection 时静默丢弃 advisory；不得把 unknown 映射为
credential-free。增加一个失败注入回归，证明 completed switch 保持成功且不会打印错误
方向文案。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- 两条 Claude 文案与 architecture 的 credential-written/free 合同一致。
- Official conflict notes 保持在 restart note 之前且只输出 key name，不泄漏 value。
- Codex advisory、quiet suppression 和 JSON envelope 均有兼容性测试。
- `ClaudeConfigMatchesSnapshot` 与 reconcile caller 已明确 file fact 不等于 running-client
credential fact；两份 living specs 已同步该边界。

### 📝 总结

当前 Task 2 fingerprint
`b9e5ba5249bf6bc5e93ea806471cd6626dcf237a62a1476ca1480277a8acae99`
完成了主体 copy、comments、tests 和 specs，但 S1-F1 会在 post-switch read-back failure
上选择错误方向，直接违背方向化 advisory 的核心目标，因此不能 PASS。宽验证应在该
failure path 修复后的最终状态运行。

## Round 2 — 2026-08-26 (repair)

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus repaired
  implementation diff fingerprint
  `55c01c9d031bd441170252dc89bd25ebbd3fd8ad13540a086bac99d52a208535`.
- Author: claude-code (repair round — this is not an independent review; the
  `Review` cell stays unticked until an independent Re-review records a
  verdict).
- Scope: S1-F1 only, as named in the repair command.
- Repair (`cmd/agentdeck/main.go`):
  - `reportSwitchAdvisories` no longer defaults `hasCredential` to `false` and
    falls through on a read-back failure or a missing client selection. For
    `ClientClaude` it now returns without printing any advisory when
    `s.Current(ctx)` errors or the returned selections carry no entry for the
    switching client — "unknown" is never mapped to "credential-free". Codex
    is unaffected: its advisory does not depend on credential presence.
  - Extracted the pure helper `claudeCredentialPresence(selections, client)
    (hasCredential, found bool)`, which distinguishes "selection found, no
    credential" from "selection not found" as two different return shapes,
    so the caller cannot collapse them the way S1-F1 did.
  - Added the test seam `currentSelectionsForAdvisories` (a package-level
    function variable over `provider.Service.Current`, matching the existing
    `userHomeDir` / `sleepForHookReconciliation` injection pattern in this
    file) so a read-back failure can be injected without a fault-injecting
    store.
- Regression tests (`cmd/agentdeck/provider_switch_advisories_test.go`):
  - `TestClaudeCredentialPresenceDistinguishesUnknownFromCredentialFree` pins
    the helper's four cases: keyed, keyless, wrong-client-only, and nil.
  - `TestSwitchAdvisoryDropsRatherThanGuessesOnReadBackFailure` forces
    `currentSelectionsForAdvisories` to return an error after a real
    credential-writing Claude switch and asserts the switch still exits 0 and
    stderr carries no `restart running Claude sessions` line at all — proving
    the completed switch stays successful and prints neither direction rather
    than the wrong one.
  - `TestSwitchAdvisoryDropsRatherThanGuessesOnMissingSelection` covers the
    other half: a successful read that has no entry for `claude` (only
    `codex`) is treated the same as a failure, not as credential-free.
- Verification: `go build ./...`, `go vet ./...` clean;
  `go test ./internal/provider/... ./cmd/agentdeck/... -count=1` all pass,
  including the three new tests above; `gofmt -l .`, `make check-whitespace`,
  and `git diff --check` clean on the repaired files.
- Completion gate: NOT_VERIFIED — a repair round cannot record its own
  completion evidence; the Task WorkUnit stays open until an independent
  Re-review passes this blob.
- Verdict: REPAIRED — awaiting independent Re-review.

## Round 3 — 2026-08-26

- Reviewed state: HEAD `08a713bda1602c9b0430398c9f4ae78423e2dd43` plus
  independently reproduced full Task 2 scope fingerprint
  `c2ceed380b7bb639564c0ac2f2086d0b47ec102bc1f9ecb523b739a8f065949f`.
  Final synchronized `tasks.md` blob is
  `78c27c1ec401384ff5764cccedfc12a94918cd91`; `docs/status.md` blob is
  `eee90ce31527b98bfea539a5805884ed43e8e32d`.
- Reviewer: Codex, independently re-reviewing the Round 2 repair; this workflow
  turn kept production code, tests, configuration, and living specs read-only.
- Method: Formal REREVIEW under `development-workflow`; S1-F1 was checked
  against the current helper/call path and its failure regressions with
  CodeGraph and full scoped diff inspection, followed by exact-state full L2
  tests and vet. Round 2's recorded `55c01c9d...` is a repair task-slice
  fingerprint; this round binds the complete shared-file Task 2 state to the
  reproducible `c2ceed38...` fingerprint above.
- Scope: S1-F1, all original Task 2 contracts for regression, living specs, and
  the final L2 gate.
- Findings:
  - **S1-F1 — CLOSED.** Claude advisory reporting now distinguishes keyed,
    keyless, missing-selection, and read-error outcomes. A missing/error outcome
    returns without printing a Claude restart advisory; it cannot fall through
    to credential-free copy, while the completed switch remains successful.
  - No new finding.
- Evidence: `cmd/agentdeck/main.go:1627-1675` defines the injectable selection
  read, two-result credential-presence helper, and fail-closed-to-silence Claude
  branch; `provider_switch_advisories_test.go:173-236` covers keyed/keyless,
  wrong-client/nil, read failure, and missing selection with successful switch
  exit. The existing provider and command tests preserve conflict ordering,
  privacy, Codex, quiet, JSON, and both normal Claude directions. Current-state
  `scripts/run-go-test.sh ./...` -> PASS (log
  `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.K8uB1q`);
  `go vet -mod=vendor ./...` -> PASS.
- Completion gate: VERIFIED — CEv1 gate
  `switch-effectiveness-boundary:switch-effectiveness-contract` verified all
  five required criteria for exact content state
  `3df46cdb4321e780cb1c0d4f5a17539d714c4100df5d7d66afd8b9cb6dc4de7b`.
- Verdict: PASS

## 📋 Switch effectiveness contract 独立复评

📊 总体评分：10/10

✅ 评审结论：PASS

### 🔴 严重问题——必须修复

无。

### 🟡 建议改进——推荐

无。

### 🟢 优点

- S1-F1 通过显式 found bit 和 read-error silence 关闭，unknown 不再伪装成
  credential-free。
- 成功 switch 的 exit status 保持不变，失败读回不会输出任一错误方向。
- 两条正常 Claude 文案、Official conflict ordering/privacy、Codex、quiet 与 JSON
  compatibility 均保持覆盖。
- File-vs-running-process comments 和两份 living specs 与实现一致。

### 📝 总结

当前完整 Task 2 fingerprint
`c2ceed380b7bb639564c0ac2f2086d0b47ec102bc1f9ecb523b739a8f065949f`
关闭了 S1-F1，未发现新的 blocker；方向化 advisory、失败语义、comments、tests、specs
与全仓 L2 形成一致候选状态，可以 PASS。剩余交付仅是单独授权的 commit/push
checkpoint。
