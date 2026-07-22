---
status: historical
plan: usage-stats-readability
task: detail-compaction
retired: 2026-07-22
---

# Review log — usage-stats-readability / detail-compaction

## Round 1 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `eda8deaba8cd9f6e1f98ffe893e720e2bbdd858b5ea1e789e217abf54459298a`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md`.
- Reviewer: Codex
- Scope: one-line model/provider detail behavior at supported widths,
  high-value and secondary field retention, two-decimal cache-hit formatting,
  responsive two-column interaction, JSON completeness, golden impact, and
  regression-test strength. Previously reviewed tasks and unrelated worktree
  changes were excluded.
- Findings:
  - [P1] The one-line contract is enforced only in the 80/100 single-column
    tests. At terminal width 104 or greater, `render` switches to two columns
    and passes only the right-column width to `rankingLines`: for example,
    width 104 yields a 40-column ranking block and width 140 yields 55 columns.
    `statsCompactDetail` never compacts an already-overwide high-value base;
    it returns that base and the following `statsWrap` splits it across lines.
    Model/provider detail can therefore remain multi-line on wide terminals,
    contradicting the task's width-greater-than-or-equal-to-80 contract. Add
    representative 104/140/160 responsive-layout tests that require each
    model/provider detail block to be exactly one line, then adjust either the
    high-value representation or the column/layout strategy so the invariant
    holds at the actual available width without dropping tokens, share, cost,
    or sessions. RED-check against the current two-column behavior and update
    golden output only where the chosen fix intentionally changes it.
- Evidence: read-only inspection of the renderer's two-column width formula,
  the compaction helper/call sites, the new 80/100 tests, and the completion
  note. The secondary-field priority, cache-hit precision, JSON preservation,
  and single-column test paths are otherwise protected. No tests were rerun in
  this review round; development evidence recorded in the plan was not
  independently revalidated here.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `0d1cf942407664f42c53ed83d98bd25b22ef36f7f32d566dbc9e6886a5808c7b`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md` before this review update.
- Reviewer: Codex
- Scope: closure of the Round 1 two-column detail finding, responsive-layout
  side effects on the previously reviewed TREND contract, mutation strength,
  final production behavior, and newly introduced problems.
- Findings:
  - [closed] `statsTwoColumnWidths` now gives the ranking column 80 columns at
    terminal widths 104/140/160. The new test requires contiguous one-line
    model/provider high-value detail plus the appropriate secondary fields,
    and the recorded pre-fix RED confirms the original wrapping defect is
    closed.
  - [P1] Flooring the ranking column at 80 leaves TREND only 20 columns at
    terminal width 104 (`inner=100`, `right=80`, `left=20`). A minimum trend
    data row needs about 28 columns (7 label + separators + 8 bar + 9 value),
    and `joinStatsColumns` sends the left line through `statsPad`/`statsFit`,
    which truncates it to 20 columns. The fix therefore makes bucket values
    disappear at the lower end of two-column mode, regressing the already
    reviewed trend display. The new responsive test checks detail substrings
    and total line width but never requires trend labels and metric values to
    remain visible. Switch to stacked layout until both columns' minimum widths
    fit, or otherwise preserve the trend's semantic content; add a width-104
    regression assertion that a representative bucket label and value both
    survive. RED-check against the current forced-80 split.
- Evidence: read-only inspection of `statsTwoColumnWidths`, `trendLines`,
  `joinStatsColumns`, `statsPad`/`statsFit`, the 104/140/160 test, and the repair
  note. The repair's package/full tests and pre-fix RED are recorded in the
  plan, but no tests were rerun in this review round.
- Verdict: REOPEN

## Round 3 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `09e89ce4ecc40a8a5309d59dc9b0abb0ef93e28afb849ef93d0fb174a2e8b735`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md` before this review update.
- Reviewer: Codex
- Scope: closure of the Round 2 trend-truncation finding at the new layout
  threshold, adequacy of the static trend minimum across supported group/metric
  formats, responsive boundary tests, and newly introduced problems.
- Findings:
  - [partially closed] Width 104 now correctly stacks, preserving both TREND
    semantic content and full-width detail compaction. Widths 140/160 retain
    the two-column behavior and one-line details.
  - [P1] `statsTrendMinWidth = 28` derives from a 7-column label and 9-column
    value, but supported output can be wider: known cost values such as
    `$13.4M KNOWN` need 12 columns, and multi-date hour labels such as
    `Jan 02 15:04` also need 12. At width 112, `statsTwoColumnFits` activates
    the split with exactly 28 columns on the left, so `trendLines` produces a
    row wider than the column and `joinStatsColumns` truncates it. The new test
    covers 104 and then jumps to 140/160, missing the activation boundary.
    Add boundary cases at 112 and the first width that truly fits representative
    day/cost and multi-date-hour labels/values. Derive a conservative minimum
    from all supported compact formats or make the layout decision depend on
    the rendered report's actual semantic width; do not enable two columns
    while label or value would be truncated. RED-check against the current
    static-28 threshold.
- Evidence: read-only inspection of `statsTwoColumnFits`, the 28-column
  derivation, `trendLines`' dynamic `labelWidth`/`valueWidth`, the supported
  label and known-cost formats, and the updated 104/140/160 test. No tests were
  rerun in this review round.
- Verdict: REOPEN

## Round 4 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `13eb2eab393f3020bcde2c97f59ab14de8bb0c9062072fd9617d65a68647d8d9`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md` before this review update.
- Reviewer: Codex
- Scope: closure of the Round 3 dynamic trend-width finding, consistency
  between layout selection and trend rendering, boundary/mutation-test
  strength, preservation of one-line ranking detail and JSON completeness,
  and newly introduced problems.
- Findings:
  - [closed] `statsTrendLabelValueWidths` now measures the actual retained
    trend window using the same `compactBucketLabels`, metric selection,
    `compactCost`, and default width floors consumed by `trendLines`.
    `statsTwoColumnFits` derives the required trend width from those measured
    label/value widths plus the existing separators and minimum bar width, so
    the layout decision no longer relies on the incomplete static 28-column
    assumption.
  - [closed] `TestUsageStatsTwoColumnThresholdCoversWidestSupportedTrendFormats`
    combines multi-date hour labels with known-partial cost values and checks
    both sides of the exact boundary: widths 112 and 119 remain stacked, while
    width 120 enters two-column mode. At every width it also requires the
    trend label/value and one-line model detail to remain visible, enforces the
    terminal-width bound, and checks JSON completeness. The recorded RED
    mutation back to the static 28-column condition failed at 112/119 on the
    truncated label, so the test distinguishes the repaired behavior from the
    reopened implementation.
  - No new blocking or medium-severity findings.
- Evidence: read-only inspection of the dynamic width helper, `render`,
  `trendLines`, the new threshold test, and the round-3 repair record; fresh
  relevant-diff identity and `git diff --check` (clean). The repair record's
  package test, full `go test -mod=vendor ./...`, focused vet, and RED/GREEN
  evidence were reused because the fresh relevant-diff identity matches the
  reviewed content state; product tests were not mechanically rerun in this
  review round.
- Verdict: PASS
