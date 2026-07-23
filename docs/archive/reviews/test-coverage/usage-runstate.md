---
status: historical
plan: test-coverage
task: usage-runstate
retired: 2026-07-23
---

# Review log — test-coverage / usage-runstate

## Round 1 — 2026-07-22

- Reviewed state: `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae` plus the
  uncommitted `internal/usage/usage_test.go`
  (`6258eb651c8a79b6b5f19f81b9779fdb34149b1f84ee9e7b4f01ca1762e46e1d`).
- Reviewer: Codex.
- Scope: task 3 additions for `FailRun`, `SetRunPID`/`RunStatus`, `Diagnose`,
  `CheckSourceReadability`, `scanFile`, `ParseMultiplier`, and `ParseInt`,
  against `internal/usage/usage.go`. The independently reviewed
  `usage-scan-performance/line-slice` production change and its performance
  test were checked only to correct task ownership; they are not task 3.
- Findings:
  - [P2] `TestFailRunSetsFailureStateAndReason` proves only the transition of
    open runs. `FailRun` is also the cleanup path after `EndRun` returns an
    error (`cmd/agentdeck/main.go`), and its `ended_at IS NULL` predicate is
    what preserves a run that may already have reached its terminal exact
    state. A regression that removes that predicate can rewrite a completed
    run to estimated while this test remains green. Add a completed exact-run
    fixture, inject a fixed `Service.Now`, call `FailRun`, and assert its
    `ended_at`, `exact`, and `ambiguity_reason` are unchanged; retain the
    open-run/default-reason cases.
  - [P2] `TestScanFileReadFailurePreservesPriorState` replaces `Service.Open`
    with a function that immediately returns an error. It therefore establishes
    only the trivial pre-open early return and never exercises the existing
    `SourceFile` seam during `io.ReadFull`/`io.ReadAll` in `scanFileMode`.
    A regression in the opened-file read path can escape despite the extensive
    before/after database assertions. Wrap the real temporary file in a
    `SourceFile` whose `ReadAt` deterministically returns a synthetic error
    after open; assert the same preserved cursor, source rows, events, run
    sources, and bindings. Do not add production seams or use a real source.
  - [P2] Status wording in `docs/plans/test-coverage.md` and `docs/README.md`
    says task 3's performance test blocks the full suite. The performance test
    belongs to the separately reviewed `usage-scan-performance/line-slice`
    task, and current L2 commands pass. This misroutes future repair work and
    falsely reports a failing baseline. The wording is corrected below; task
    3 remains unchecked because of the two test-quality findings above, not a
    missing checkbox.
- Evidence:
  - `rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage -run TestScanCostScalesLinearlyNotQuadraticallyWithLineCount -count=20` — PASS (20 consecutive runs).
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS.
  - `rtk git diff --check` — PASS before this review-record update.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae` plus the
  uncommitted `internal/usage/usage_test.go`
  (`37f84b48fcd6921832e9526755892f6b74a33a6e995a22ee1456f3c19ea511eb`).
- Reviewer: Codex.
- Scope: Round 1 P2 repairs only: `TestFailRunSetsFailureStateAndReason` and
  `TestScanFileReadFailurePreservesPriorState`.
- Findings:
  - [P2] The terminal-run protection and fixed clock added to
    `TestFailRunSetsFailureStateAndReason` close the first Round 1 finding,
    but the prior open-run case for a supplied reason was replaced. The only
    open run now calls `FailRun(..., "")`; the supplied
    `"synthetic failure"` is passed only for an already-ended run, where the
    correct outcome is no mutation. An implementation that always records
    `wrapper_cleanup_failed` for every open run passes both current cases while
    breaking caller-supplied cleanup reasons such as `client_start_failed`.
    Keep the terminal no-mutation case and add a second open run that calls
    `FailRun` with a non-empty synthetic reason, asserting the fixed terminal
    timestamp, `exact=0`, and that exact supplied reason.
  - [P2] Resolved: the scan failure test now opens the temporary source then
    injects a deterministic `SourceFile.ReadAt` failure. It exercises the
    `io.ReadFull` source-read path and proves source, event, run-source, and
    binding state remain unchanged.
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS.
  - `rtk git diff --check` — PASS before this review-record update.
- Verdict: REOPEN

## Round 3 — 2026-07-22

- Reviewed state: `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae` plus the
  uncommitted `internal/usage/usage_test.go`
  (`1a5450f26ef4a8b7e37c9ed37d3630cc70eb80fd9bc894d2ebc3f9f100e38e70`).
- Reviewer: Codex.
- Scope: Round 2 P2 repair in `TestFailRunSetsFailureStateAndReason`, with a
  regression check of the previously repaired `ReadAt` source-read failure
  case.
- Findings:
  - Round 2 P2 resolved: the test now has distinct open runs for the default
    and caller-supplied failure reasons, plus an already-ended exact run. It
    asserts the injected terminal timestamp, `exact=0`, and the exact supplied
    reason for the open custom-reason path, while proving the closed run is not
    rewritten.
  - No new actionable findings in task 3 scope. The `ReadAt` failure seam and
    preserved-attribution assertions remain deterministic and isolated.
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS.
  - `rtk git diff --check` — PASS before this review-record update.
- Verdict: PASS
