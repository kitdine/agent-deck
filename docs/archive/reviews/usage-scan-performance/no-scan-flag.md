---
status: historical
plan: usage-scan-performance
task: no-scan-flag
retired: 2026-07-22
---

# Review log — usage-scan-performance / no-scan-flag

## Round 1 — 2026-07-22

- Reviewed state: current uncommitted worktree containing the command-local
  flags in `cmd/agentdeck/main.go`, command-path coverage in
  `cmd/agentdeck/main_test.go`, the CLI manual update, and the task completion
  note in `docs/plans/usage-scan-performance.md`. Unrelated concurrent worktree
  changes were excluded from the findings.
- Reviewer: Codex
- Scope: flag availability and isolation, default synchronous pre-scan,
  no-scan stored-aggregate behavior, progress suppression, partial/warning
  semantics, output contracts, documentation, and regression-test value.
- Overall score: 8.5/10; one behavioral-test repair is required.

### Findings

1. **[P2] The core stored-aggregate output contract is not protected.**
   `TestUsageCommandsUseProgressForExplicitAndImplicitScans` proves that the
   progress reporter does not start for `usage stats --no-scan` and
   `usage summary --no-scan`, but it does not inspect either command's result.
   Both commands run against an empty state and success alone cannot prove that
   they returned stored aggregates, ignored newer source data, or avoided a
   false `scan_incomplete` partial warning. An implementation that skipped the
   reporter but returned empty data, discarded stored data, or marked the
   intentional skip as partial could leave all current tests green. Add a
   command-level JSON regression for both commands: seed one stored usage event,
   add a second source event without scanning it, run `--no-scan`, and assert
   the response contains only the stored event with no partial/warning state.
   Then run the default command path and assert the newly added event becomes
   visible, proving that default synchronous scanning remains active. Use an
   all-history range for stats so the fixture is independent of wall-clock
   dates. The assertions must fail if `--no-scan` scans the new source, loses
   the stored aggregate, or reports `scan_incomplete`.

### Behavior-to-test coverage

| Behavior | Current coverage | Assessment |
| --- | --- | --- |
| Flags exist only on stats and summary | Cobra command construction | Correct by inspection |
| Default stats and summary start a scan | Reporter lifecycle assertions | Covered |
| `--no-scan` does not start progress | Reporter lifecycle assertions | Covered |
| `--no-scan` returns the stored aggregate | None | Missing; finding 1 |
| New source data remains invisible until a default scan | None | Missing; finding 1 |
| Intentional skip is not `scan_incomplete` | None | Missing; finding 1 |
| JSON/text command surface is documented | CLI manual and design contract | Covered |

### Positive observations

- The flags are local to `usage stats` and `usage summary`; scan, rebuild,
  sessions, and diagnose do not inherit them.
- The implementation skips only `Service.Scan` and still executes
  `Summary`, `SummaryRange`, or `Stats`, so the current production path does
  query persisted aggregates rather than moving work to a background goroutine.
- Default paths retain their synchronous scan and existing `scanErr` to
  `scan_incomplete` mapping.
- The CLI manual clearly distinguishes the default fresh behavior from the
  caller-selected stale behavior and supplies examples for both commands.

### Evidence and residual uncertainty

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageCommandsUseProgressForExplicitAndImplicitScans ./cmd/agentdeck/...` — pass.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageCommandTextAndJSONContracts ./cmd/agentdeck/...` — pass.
- `rtk git diff --check` — pass before this review record was added.
- The project-wide L2 suite was not repeated because the verdict is already
  reopened on a targeted behavioral gap; it is required after the repair.
- Verdict: **REOPEN**. Leave the plan's `Review` cell unchecked until finding 1
  is closed and independently re-reviewed.

### Scoped repair instruction

> **根据评审修改：usage-scan 性能与进度 / no-scan-flag**
>
> Read `AGENTS.md`, the `no-scan-flag` task in
> `docs/plans/usage-scan-performance.md`, and Round 1 in this review record.
> Add command-level JSON regression coverage for both `usage stats --no-scan`
> and `usage summary --no-scan`: create isolated state and source fixtures,
> scan one usage event into storage, append or add a second event without
> scanning it, and prove each no-scan response contains only the stored data,
> remains non-partial, and has no `scan_incomplete` warning. Then execute the
> corresponding default command and prove the second event becomes visible.
> Use `--period all` for stats to avoid wall-clock-dependent fixtures. The test
> must fail if no-scan reads the new source, loses stored data, or reports the
> intentional skip as incomplete. Do not change production behavior unless the
> test exposes a real defect; do not implement `remeasure`, do not perform
> unrelated refactors, do not tick `Review`, and do not commit automatically.
> Run the targeted `cmd/agentdeck` and `internal/usage` tests, the current L2
> full suite, and `git diff --check`, then request
> `进入复评并生成后续指令：usage-scan 性能与进度 / no-scan-flag`.

## Round 2 — 2026-07-22 (re-review)

- Reviewed state: current uncommitted worktree after the Round 1 test-only
  repair. The only new task-scoped change was
  `TestUsageNoScanUsesStoredAggregateUntilDefaultScan` in
  `cmd/agentdeck/main_test.go`; production behavior was unchanged.
- Reviewer: Codex
- Scope: close Round 1's stored-aggregate behavior gap, assess regression-test
  failure value for both commands, check for new problems, and rerun L2
  evidence.
- Round 1 finding: **Closed.** The new table-driven command test gives stats
  and summary separate isolated state roots. It scans one event into storage,
  appends a second event without scanning, and verifies each JSON `--no-scan`
  response remains at one event and 10 input tokens with `partial=false` and no
  warnings. It then executes the corresponding default command and requires two
  events and 30 input tokens. Stats uses `--period all`, so the fixture is not
  dependent on the current date. Scanning during no-scan, losing persisted
  data, marking the intentional skip incomplete, or failing to scan on the
  default path would each make the test fail.
- New findings: none.
- Evidence:
  `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageNoScanUsesStoredAggregateUntilDefaultScan ./cmd/agentdeck/...`
  — pass;
  `rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
  — pass across all packages with exit code 0; and `rtk git diff --check` —
  pass before the review-status update.
- Verdict: **PASS**. The Round 1 finding is closed, no new findings were
  introduced, and the plan's `Review` cell may be ticked.
