---
status: historical
plan: usage-scan-performance
task: line-slice
retired: 2026-07-22
---

# Review log — usage-scan-performance / line-slice

## Round 1 — 2026-07-22
- Reviewed state: `internal/usage/usage.go` one-line diff
  (`strings.IndexByte(string(line), '\n')` → `bytes.IndexByte(line, '\n')`),
  plus `TestScanCostScalesLinearlyNotQuadraticallyWithLineCount` in
  `internal/usage/usage_test.go` (uncommitted worktree changes).
- Reviewer: Claude (zh-code-reviewer format)
- Scope: correctness and semantic equivalence of the fix; isolation and
  robustness of the new ratio-based regression test; RED/GREEN evidence
  supplied by the implementer.
- Findings:
  - [nit] Ratio-based timing assertion still carries residual wall-clock noise
    risk under `go test ./...` cross-package parallelism -> not closed,
    optional, no flakiness observed.
  - [nit] `baseLines`/`padLen` constants lack an inline rationale comment for
    the specific values chosen -> not closed, optional.
  - [nit] No `testing.Short()` gate despite a multi-second worst-case runtime
    if the bug regresses -> not closed, optional.
- Evidence:
  `go build -mod=vendor ./...`,
  `go test -mod=vendor ./internal/usage/...`,
  `go test -mod=vendor ./...`,
  `go vet -mod=vendor ./internal/usage/...`,
  `git diff --check` — all pass.
- Verdict: PASS

## Round 2 — 2026-07-22 (re-review)
- Reviewed state: identical to Round 1 (`internal/usage/usage.go` one-line
  diff unchanged; `TestScanCostScalesLinearlyNotQuadraticallyWithLineCount`
  body byte-for-byte unchanged, re-run confirms 0.08s / PASS). Noted that
  `internal/usage/usage_test.go` grew from ~50 to ~352 changed lines in the
  interim due to the concurrent test-coverage plan's task 3
  (`usage-runstate`) touching the same file; confirmed no overlap or
  interference with this task's diff before proceeding.
- Reviewer: Claude
- Scope: re-verify the three Round 1 nits remain the only open items, confirm
  no new issues, confirm no drift in the reviewed content.
- Findings: the three Round 1 nits remain open (optional, not requested to be
  closed); no new findings.
- Evidence: re-ran `go build -mod=vendor ./...`,
  `go test -mod=vendor -run TestScanCostScalesLinearlyNotQuadraticallyWithLineCount -v ./internal/usage/...`,
  `go test -mod=vendor ./internal/usage/...`,
  `go vet -mod=vendor ./internal/usage/...` — all pass.
- Verdict: PASS

## Round 3 — 2026-07-22 (test hardening note, not a reopened task)
- Reviewed state: `TestScanCostScalesLinearlyNotQuadraticallyWithLineCount` in
  `internal/usage/usage_test.go`, hardened during the unrelated `session-index`
  task. `internal/usage/usage.go`'s one-line fix is unchanged.
- Reviewer: Claude
- Trigger: the test flaked once under a full `go test -mod=vendor ./...` run
  (measured ratio 10.1x, just above the 8x threshold), traced to cross-package
  CPU contention with the concurrently running `internal/backup` package tests
  — exactly the residual risk Round 1 already named as an open, non-blocking
  nit. Isolated re-runs of the test alone passed reliably (0.08s, 3x), and a
  second full-suite run immediately after also passed cleanly, confirming
  scheduling noise rather than a real regression.
- Change: `measure()` now runs 3 samples per file size (fresh state directory
  and fresh `store.Open` each attempt, so every sample is a genuine cold scan)
  and keeps the minimum — standard practice for wall-clock microbenchmark
  noise, sound because contention can only inflate a sample, never deflate it
  below true cost.
- RED/GREEN re-verified against the hardened version: reverting the production
  fix reproduced a reliable failure (10.2x, base=97ms/scaled=991ms); restoring
  it passed consistently across four repeated runs (cached and uncached).
  Two independent `go test -mod=vendor -count=1 ./...` full-suite runs after
  the change were both clean.
- Assessment: test-only stability hardening, not a behavior or scope change —
  the algorithmic fix in `usage.go` is untouched, and the regression test still
  fails on the same bug it always did (re-confirmed above). Does not reopen
  this task's gate.
- Evidence:
  `go build -mod=vendor ./...`,
  `go test -mod=vendor -run TestScanCostScalesLinearlyNotQuadraticallyWithLineCount -v ./internal/usage/...` (RED then GREEN),
  `go test -mod=vendor -count=1 ./...` (run twice, both clean),
  `go vet -mod=vendor ./...`,
  `git diff --check` — all pass.
- Verdict: PASS (unchanged; Round 2's `Review` tick for this task stands)
