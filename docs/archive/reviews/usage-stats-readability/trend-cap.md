---
status: historical
plan: usage-stats-readability
task: trend-cap
retired: 2026-07-22
---

# Review log — usage-stats-readability / trend-cap

## Round 1 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `edf1a1e8771b485ec3ae2234c7d9a2058457014cb65fc254f01ec30ac00eb395`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md`.
- Reviewer: Codex
- Scope: the 48-bucket text cap, selection of the most recent contiguous
  window, chronological rendering, overflow wording, JSON completeness, and
  regression-test strength. Previously reviewed shared-topn changes and
  unrelated worktree changes were excluded.
- Findings:
  - [P2] `TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText` checks
    that every retained label exists, but never checks their relative output
    positions. Reversing or otherwise reordering the correct 48-bucket window
    would still pass, despite the plan and completion note promising the
    existing chronological order. Record each retained label's index in the
    rendered text and require strictly increasing positions; RED-check with a
    temporary reversal or swap of the windowed buckets.
- Evidence: read-only inspection of the trend renderer, its regression test,
  and the plan diff. The implementation correctly selects
  `r.report.Buckets[omitted:]`, emits the exact earlier-bucket footer, and
  leaves the report slice available to the actual JSON envelope path. No tests
  were rerun in this review round; development evidence recorded in the plan
  was not independently revalidated here.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `e2e023372ea9184329d6c55e4d2d7ffd5ae45d9bdf6c59916dec9aad93554db3`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md` before this verdict update.
- Reviewer: Codex
- Scope: closure of the Round 1 chronological-order test finding, RED/GREEN
  strength, final production-code identity, and newly introduced problems in
  the trend-cap scope.
- Findings:
  - [closed] The retained-window loop now obtains each expected label's
    `strings.Index` position and requires every position to be strictly greater
    than the previous one, so presence without chronological order no longer
    passes.
  - [closed] A temporary copy of the correct window with its first two buckets
    swapped produced the intended RED failure (`Jun 21 13:00` at position 1204
    after position 1386). Restoring the original tail slice returned the test
    to GREEN without a final production-code change.
  - New findings: none.
- Evidence: targeted trend-cap test, `./cmd/agentdeck` package tests, and the
  full `./...` suite passed with `-count=1`; `git diff --check` passed after the
  final completion-note edit. These results were produced in the immediately
  preceding repair workflow and bind to the unchanged reviewed product and
  test content, so this round did not mechanically rerun them.
- Verdict: PASS
