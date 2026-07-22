---
status: historical
plan: usage-stats-readability
task: top-flag
retired: 2026-07-22
---

# Review log — usage-stats-readability / top-flag

## Round 1 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `d9b633e526b8651ad549edd37ab1767da59493ed3bf1973ee91796bb1d86ea78`
  over the relevant diff in `cmd/agentdeck/main.go`,
  `cmd/agentdeck/main_test.go`, `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, `docs/specs/cli-manual.md`, and
  `docs/plans/usage-stats-readability.md`.
- Reviewer: Codex
- Scope: Cobra flag parsing and validation; omitted versus explicit-zero
  semantics; positive overrides across the five shared-topn lists; TREND and
  CLIENTS independence; JSON completeness; CLI documentation and regression
  test strength. Previously reviewed tasks and unrelated worktree changes were
  excluded.
- Findings:
  - [P2] The new renderer and CLI tests do not assert the documented negative
    contract that `--top` leaves TREND and CLIENTS unchanged. The positive test
    uses `--top 3` while the fixture has only two clients, and it never checks
    trend rows; the existing trend-cap test runs with no top override. An
    implementation that incorrectly applies `capFor` to the trend window or to
    CLIENTS can therefore remain green. Add a fixture with more than N clients
    and more than 48 buckets, render with a small positive top, and require all
    clients plus the independent 48-bucket recent trend and its original
    overflow footer. RED-check the two protections with temporary erroneous
    cap wiring.
- Evidence: read-only inspection of the CLI wiring, renderer cap resolution,
  renderer tests, end-to-end CLI test, manual, and plan diff. Omitted/default,
  positive, explicit-zero, negative rejection, all five shared-topn lists, and
  JSON completeness are otherwise protected. No tests were rerun in this
  review round; development evidence recorded in the plan was not independently
  revalidated here.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `5db167f50a2fbb2564406796cb8e1d2d8a2bf8f0ed8dccfbfba85b378ed20f8a`
  over the relevant diff in `cmd/agentdeck/main.go`,
  `cmd/agentdeck/main_test.go`, `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, `docs/specs/cli-manual.md`, and
  `docs/plans/usage-stats-readability.md` before this verdict update.
- Reviewer: Codex
- Scope: closure of the Round 1 TREND/CLIENTS independence finding, mutation
  strength, final production-code identity, and newly introduced problems in
  the top-flag scope.
- Findings:
  - [closed] `TestUsageStatsTopFlagDoesNotAffectTrendOrClients` renders five
    clients and sixty hourly buckets with `--top 3`; it requires all clients,
    the independent recent 48-bucket window, the original overflow footer,
    omission of earlier buckets, and strictly increasing retained positions.
  - [closed] Temporarily applying `capFor` to CLIENTS failed the client
    assertions, and separately applying it to the trend window failed the trend
    assertions. Both mutations were reverted; no production defect or final
    production-code change resulted from the repair.
  - New findings: none.
- Evidence: the repair's `./cmd/agentdeck` package test, full `./...` suite,
  two independent RED mutations, and `git diff --check` results are recorded in
  the plan and bind to the reviewed content. A fresh relevant-diff digest and
  `git diff --check` passed in this re-review; product tests were not
  mechanically rerun.
- Verdict: PASS
