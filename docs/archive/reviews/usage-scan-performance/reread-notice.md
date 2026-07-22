---
status: historical
plan: usage-scan-performance
task: reread-notice
retired: 2026-07-22
---

# Review log — usage-scan-performance / reread-notice

## Round 1 — 2026-07-22

- Reviewed state: current uncommitted worktree containing the
  `reread-notice` implementation in `internal/usage/usage.go`, renderer change
  in `cmd/agentdeck/main.go`, corresponding tests, and the task completion note
  in `docs/plans/usage-scan-performance.md`. Unrelated concurrent worktree
  changes were excluded from the findings.
- Reviewer: Codex
- Scope: parser-version-only inventory classification, reason propagation
  through scan progress, stderr renderer text, suppression for new/appended
  data and explicit rebuilds, and the regression value of the added tests.
- Overall score: 8.5/10; one test-contract repair is required.

### Findings

1. **[P2] The promised explicit-rebuild suppression has no protective test.**
   The completion note states that explicit rebuilds remain unlabelled, and the
   current production implementation satisfies that contract because
   `Service.Rebuild` never propagates `Inventory.ParserVersionReread` into its
   `ScanProgress` updates. However, the existing rebuild progress test runs
   only after all source rows already carry the current parser version. A
   future change that reads `inventory.ParserVersionReread` inside `Rebuild`
   and emits `re-reading after parser update` would therefore leave the suite
   green. Extend the service regression coverage so a stored source has an old
   parser version immediately before an explicit `Rebuild`, then assert every
   rebuild progress update has an empty `Reason`. The assertion must fail if
   parser-reread reason propagation is deliberately added to `Rebuild`.

### Behavior-to-test coverage

| Behavior | Current coverage | Assessment |
| --- | --- | --- |
| Unchanged old-parser source is rebuilt | State and aggregate assertions | Covered |
| Parser-only scan carries the reason on every update | Exact progress sequence | Covered |
| Appended/new data suppresses the parser-only reason | Inventory negative assertion | Covered |
| Renderer includes the explanatory text | Deterministic output assertion | Covered |
| Fast scans remain silent and quiet suppresses output | Existing progress-output tests | Covered |
| Explicit rebuild remains unlabelled despite old parser rows | None | Missing; finding 1 |

### Positive observations

- The classifier is conservative: it requires at least one metadata-identical
  parser mismatch and rejects added, appended, or other mutated sources.
- `ParserVersionReread` is excluded from inventory JSON, so the existing public
  inventory shape does not change.
- The reason is carried through every scan progress update rather than only the
  initial frame, preserving correct final redraw behavior.
- The renderer reuses the established stderr, TTY, non-TTY, quiet, and
  first-second-silence paths instead of creating a parallel output mechanism.

### Evidence and residual uncertainty

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageParserVersion ./internal/usage/...` — pass.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestInventoryWithNewData ./internal/usage/...` — pass.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestScanAndRebuildReport ./internal/usage/...` — pass.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageProgressOutputShowsParser ./cmd/agentdeck/...` — pass.
- `rtk git diff --check` — pass before this review record was added.
- The project-wide L2 suite was not repeated because the verdict is already
  reopened on a targeted regression gap; it is required after the repair.
- Verdict: **REOPEN**. Leave the plan's `Review` cell unchecked until finding 1
  is closed and independently re-reviewed.

### Scoped repair instruction

> **根据评审修改：usage-scan 性能与进度 / reread-notice**
>
> Read `AGENTS.md`, the `reread-notice` task in
> `docs/plans/usage-scan-performance.md`, and Round 1 in this review record.
> Add only regression coverage proving that an explicit `Service.Rebuild`
> remains unlabelled even when its inventory contains an unchanged source with
> an outdated parser version. Set that stored row's `parser_version` old before
> rebuild, capture every `ScanProgress` update, and assert each `Reason` is
> empty. The test must fail if parser-reread reason propagation is deliberately
> added to `Rebuild`. Do not modify production behavior unless the test exposes
> a real defect; do not implement `no-scan-flag` or `remeasure`, do not perform
> unrelated refactors, do not tick `Review`, and do not commit automatically.
> Run the targeted `internal/usage` and `cmd/agentdeck` tests, the current L2
> full suite, and `git diff --check`, then request
> `进入复评并生成后续指令：usage-scan 性能与进度 / reread-notice`.

## Round 2 — 2026-07-22 (re-review)

- Reviewed state: current uncommitted worktree after the Round 1 test-only
  repair. The only new task-scoped change was
  `TestRebuildDoesNotLabelUnchangedOldParserSource` in
  `internal/usage/usage_test.go`; production behavior was unchanged.
- Reviewer: Codex
- Scope: close Round 1's explicit-rebuild regression gap, assess whether the
  new test has failure value, check for new regressions, and rerun L2 evidence.
- Round 1 finding: **Closed.** The new test first creates a normal scanned
  source, changes its stored parser version to an old value, and independently
  confirms that `Inventory` classifies it as a parser-version reread. It then
  executes explicit `Service.Rebuild`, requires progress updates to exist, and
  asserts every update has an empty `Reason`. Propagating the parser-reread
  reason into `Rebuild` would make the test fail, so it directly protects the
  promised negative contract.
- New findings: none.
- Evidence:
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestRebuildDoesNotLabelUnchangedOldParserSource ./internal/usage/...`
  — pass;
  `rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
  — pass across all packages with exit code 0; and `rtk git diff --check` —
  pass before the review-status update.
- Verdict: **PASS**. The Round 1 finding is closed, no new findings were
  introduced, and the plan's `Review` cell may be ticked.
