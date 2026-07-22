---
status: historical
plan: usage-scan-performance
task: progress-output
retired: 2026-07-22
---

# Review log — usage-scan-performance / progress-output

## Round 1 — 2026-07-22
- Reviewed state: progress-output development tree before the repair.
- Reviewer: not identified in the supplied review conclusion.
- Scope: CLI progress rendering, command-path output routing, and reporter
  test determinism.
- Findings:
  - [P2] `Stop` could leave a visible stale processed/total value -> repair
    requires one final redraw after the reporter has stopped.
  - [P2] Command-path tests did not prove that progress receives stderr and
    leaves JSON stdout parseable -> add an injected stderr probe.
  - [P2] Reporter tests used real sleeps and read `bytes.Buffer` concurrently
    with the refresh goroutine -> replace with controlled timer/ticker and a
    synchronized writer; cover TTY, non-TTY, final newline, and quiet output.
- Evidence: review conclusions supplied with the repair instruction.
- Verdict: REOPEN

## Round 2 — 2026-07-22 (re-review)

- Reviewed state: current uncommitted worktree after the Round 1 repair in
  `cmd/agentdeck/main.go` and `cmd/agentdeck/main_test.go`. Unrelated concurrent
  changes in the worktree were excluded from the review findings, although the
  current full-suite result necessarily exercised the complete tree.
- Reviewer: Codex
- Scope: verify each Round 1 P2 finding, inspect the repair for new regressions,
  and independently run the task's L2 evidence.

### Round 1 findings — verification

1. **Closed in production behavior; regression coverage is incomplete.**
   `usageProgressOutput.Stop` now closes the reporter, waits for the refresh
   goroutine, and redraws the latest state only when `emitted` is already true.
   The deterministic non-TTY and TTY tests prove that an already-visible
   `1/N` line is finalized as `N/N`, with the TTY newline preserved. However,
   no test calls `Stop` before firing the manual initial timer. A regression
   that removes the `if !p.emitted { return }` guard and prints on every stop
   would still leave the suite green, violating the required silent fast-scan
   path. Add one deterministic fast-stop test that starts the reporter, updates
   a non-zero total, stops without firing the timer, and asserts empty output
   (and preferably that no ticker was created).
2. **Closed.** `TestUsageProgressReporterUsesStderrAndPreservesJSONStdout`
   injects a reporter that writes through the factory-supplied writer, asserts
   that writer is the command's stderr, verifies the probe is absent from
   stdout, and successfully decodes stdout as a `usage.scan` JSON envelope.
3. **Partially closed.** The real 900 ms/300 ms sleeps and concurrent
   `bytes.Buffer` access are gone. The manual timer/ticker and synchronized
   writer deterministically cover periodic non-TTY lines, TTY redraw and final
   newline, no ANSI in non-TTY output, and quiet suppression without creating a
   timer. The missing stop-before-initial-timer assertion described in finding
   1 is the only remaining scenario from Round 1.

### New findings

- None beyond the incomplete fast-stop regression coverage above.

### Evidence

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageProgress ./cmd/agentdeck/...` — pass.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestScanAndRebuildReportProcessedAndTotalSourceFiles ./internal/usage/...` — pass.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...` — pass across all packages.
- `rtk git diff --check` — pass.
- An earlier combined targeted command was invalid because `rtk test` parsed
  the `|` characters in its `-run` regular expression as shell pipelines; it
  produced `command not found` errors and is not counted as evidence. The two
  successful targeted commands above replace it.

- Verdict: **REOPEN**. The implementation behavior is correct in the reviewed
  tree, but the plan's `Review` cell remains unchecked until the explicit
  stop-before-first-frame regression test is added and re-reviewed.

### Scoped repair instruction

> **根据评审修改：usage-scan 性能与进度 / progress-output**
>
> Read `AGENTS.md`, the `progress-output` task in
> `docs/plans/usage-scan-performance.md`, and Round 2 in this review record.
> Add only a deterministic regression test for the remaining fast-stop case:
> construct `usageProgressOutput` with the manual clock and synchronized
> writer, call `Start`, update a non-zero processed/total value, call `Stop`
> without firing the initial timer, and assert that output remains empty and no
> refresh ticker was created. Do not change production behavior unless that
> new test exposes a real defect; do not implement `reread-notice` or
> `no-scan-flag`, do not perform unrelated refactors, do not tick `Review`, and
> do not commit automatically. Run the targeted `cmd/agentdeck` test, the
> current L2 full suite, and `git diff --check`, then request
> `进入复评并生成后续指令：usage-scan 性能与进度 / progress-output`.

## Round 3 — 2026-07-22 (re-review)

- Reviewed state: current uncommitted worktree after the Round 2 test-only
  repair. The only new task-scoped change was
  `TestUsageProgressOutputFastStopBeforeInitialTimerStaysSilent` in
  `cmd/agentdeck/main_test.go`; production progress behavior was unchanged.
- Reviewer: Codex
- Scope: close Round 2's sole remaining fast-stop coverage gap, check the new
  test's regression value, and rerun the task's L2 evidence.
- Round 2 finding: **Closed.** The new deterministic test starts the reporter,
  supplies a non-zero processed/total state, and calls `Stop` without firing
  the manual initial timer. It asserts that output remains empty and that no
  refresh ticker was created. Removing the production `!emitted` guard would
  make this test fail, so it protects the required silent fast-scan behavior
  rather than merely exercising the path.
- New findings: none.
- Evidence:
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageProgress ./cmd/agentdeck/...`
  — pass;
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
  — pass across all packages with exit code 0; and `rtk git diff --check` —
  pass before the review-status update.
- Verdict: **PASS**. All Round 1 and Round 2 findings are closed, no new
  findings were introduced, and the plan's `Review` cell may be ticked.
