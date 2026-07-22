---
status: historical
plan: usage-scan-performance
task: session-index
retired: 2026-07-22
---

# Review log — usage-scan-performance / session-index

## Round 1 — 2026-07-22

**This round's record was not written at the time it happened; it is filled in
here afterward from the reviewer's original report, reproduced below rather
than reconstructed from secondary evidence.** The review was produced under
the `进入评审并生成修复指令` workflow trigger, whose format (zh-code-reviewer:
🔴/🟡/🟢 plus a score) does not itself assign a `PASS`/`REOPEN` verdict — the
`Verdict` field below is assigned now, applying this plan's own convention
(see `line-slice.md` Round 1: findings with no 🔴 items pass even with open
🟡 nits). Unlike `line-slice`, where the 🟡 nits were left open after review,
here the user explicitly requested `根据评审修改 ... 全部采纳` — adopting all
three before re-review — which is why a Round 2 re-review follows in this
file rather than the task going straight to done.

- Reviewed state: the pre-fix worktree diff — `internal/store/migrations.go`
  (schema v14), `internal/store/store.go` (`CurrentSchemaVersion` 13 → 14),
  `internal/store/store_test.go` (`TestV14MigrationAddsUsageEventsClientSessionIndex`
  with 2 rows and the redundant `version < 14` condition),
  `internal/doctor/doctor_test.go` and `cmd/agentdeck/main_test.go` (the
  `CurrentSchemaVersion` literal/`DROP INDEX` drift fixes from `session-index`
  development), and `internal/usage/usage_test.go` (the `line-slice` task's
  min-of-3-samples hardening, done incidentally during `session-index` to
  unblock a clean full-suite run).
- Reviewer: Claude (zh-code-reviewer report format)
- Scope: correctness of the schema v14 migration and its regression test; the
  three drift fixes to pre-existing tests; the min-of-3 hardening to
  `line-slice`'s test.
- Findings:
  - 🔴 严重问题: none.
  - 🟡 [`internal/store/store_test.go`] Redundant `version < 14` condition in
    `TestV14MigrationAddsUsageEventsClientSessionIndex`, already implied by
    `version != CurrentSchemaVersion`. Non-blocking.
  - 🟡 [`internal/store/store_test.go`] Test used only 2 distinct-session rows;
    the `EXPLAIN QUERY PLAN` assertion's strength depends on the query
    planner choosing the index over a scan even for a trivially small table —
    a heuristic that could change. Suggested widening to make the index's
    benefit unambiguous. Non-blocking, but flagged as worth taking seriously
    since the same class of "passes now but rests on an unstated assumption"
    concern had already materialized once this session (the `line-slice`
    ratio-test flake).
  - 🟡 [`internal/doctor/doctor_test.go`] The sibling simulated-v12 test
    (`TestCheckOlderAndFutureSchemasAreSafeAndReadable`) used the same
    bootstrap-then-rewind technique as the one just fixed in
    `cmd/agentdeck/main_test.go`, but without the matching
    `DROP INDEX usage_events_client_session`. Not currently failing (`doctor`
    only reads the version, never replays migrations), but the simulated
    database was not an accurate v12 state. Non-blocking.
  - 🟢 亮点 (six items): index column order `(client, session_id)` matches
    `rebuildSessions`'s `WHERE`/`GROUP BY` exactly; `CurrentSchemaVersion` was
    bumped in the same change as the new migration entry (verified this
    coupling matters — a migration without the bump would let the binary
    reject its own freshly-migrated database on next open); the
    `EXPLAIN QUERY PLAN` test used the standard, empirically-verified 4-column
    format rather than a guess; the three drift fixes were each matched to
    their actual root cause rather than one fix pattern applied everywhere;
    the `line-slice` hardening note was transparent about touching an
    already-reviewed task's file without unilaterally reopening its gate;
    every step had RED/GREEN verification plus two independent uncached
    `-count=1 ./...` runs.
  - 总体评分: 9/10.
- Evidence (from the development pass this review covered): `go build`,
  targeted `go test` for `internal/store`, `internal/doctor`, `cmd/agentdeck`,
  two independent `go test -mod=vendor -count=1 ./...` runs, `go vet`,
  `git diff --check` — all pass.
- Verdict: **PASS**, no blocking findings. Three non-blocking nits were named;
  per this plan's convention that alone would not have blocked the `Review`
  tick (see `line-slice` Round 1), but the user chose to close them via
  `根据评审修改` before re-review, producing Round 2 below.

## Round 2 — 2026-07-22 (re-review)

- Reviewed state: uncommitted worktree. `internal/store/migrations.go` (+3,
  schema v14), `internal/store/store.go` (`CurrentSchemaVersion` 13 → 14),
  `internal/store/store_test.go`
  (`TestV14MigrationAddsUsageEventsClientSessionIndex` at :504,
  `TestV13MigrationAddsSafeToolActivityStorage` at :446),
  `internal/doctor/doctor_test.go` (:39, :76-77),
  `cmd/agentdeck/main_test.go` (:1106). Note: `store_test.go` and
  `usage_test.go` also carry unrelated changes from the concurrent
  `test-coverage` plan; those were excluded from this task's scope after
  confirming no overlap with the v14 work.
- Reviewer: Claude
- Scope: verify each Round 1 finding is closed, check for newly introduced
  problems, and independently confirm the regression test actually protects the
  index rather than passing incidentally.

### Round 1 findings — verification

1. **Closed.** `store_test.go:516` reads
   `if err != nil || version != CurrentSchemaVersion`; the redundant
   `version < 14` condition is gone.
2. **Closed.** `store_test.go:525-531` inserts 60 rows across 60 distinct
   sessions (`session-%02d`), with an inline comment explaining why.
3. **Closed.** `doctor_test.go:39` now reads
   `DROP TABLE usage_tool_calls; DROP INDEX usage_events_client_session; UPDATE schema_metadata SET version=12`,
   matching the sibling fix in `cmd/agentdeck/main_test.go:1106`.

### Independent verification

- **The test's assertions have teeth.** The implementer's recorded RED was
  observed "on the version check", which would not prove the
  `EXPLAIN QUERY PLAN` assertions can fail. Replayed the exact query against the
  real table DDL with and without the v14 index (sqlite3 CLI 3.51.0):
  without it the plan reports `SCAN usage_events` (which the test treats as a
  hard failure); with it, `SEARCH usage_events USING INDEX
  usage_events_client_session (client=? AND session_id=?)` (which sets
  `usesIndex`). Both assertions are therefore protective. Method caveat: the
  no-index case was demonstrated with the sqlite3 CLI rather than the
  `modernc.org/sqlite v1.53.0` driver; the with-index case is already proven by
  the passing Go test.
- **The test query faithfully mirrors production.** `rebuildSessions`
  (`internal/usage/usage.go:1265`) executes
  `INSERT INTO usage_sessions(...) SELECT client,session_id,MIN(event_at),MAX(event_at) FROM usage_events WHERE client=? AND session_id=? GROUP BY client,session_id`.
  The test's `SELECT` is character-for-character identical to that inner select.
- **The index is not redundant.** `usage_events` previously carried only
  `usage_events_source(source_path)` (`migrations.go:40`) and
  `usage_events_event_at(event_at)` (`migrations.go:82`); nothing covered
  `(client, session_id)`.
- **No other simulated-downgrade site was missed.** The remaining
  `UPDATE schema_metadata SET version=?` sites
  (`cmd/agentdeck/main_test.go:1037`, `internal/doctor/doctor_test.go:94`) only
  read the schema version through `doctor`; they never reopen the store, so
  migration 14 is not replayed and no `DROP INDEX` is needed. Correctly left
  untouched.
- **Migration style is consistent** with the existing plain `CREATE INDEX`
  statements at `migrations.go:40` and `:82`; no `IF NOT EXISTS` divergence.

### New findings

- **[nit] Stale subtest names.** `internal/doctor/doctor_test.go:76-77` keep the
  names `schema13` and `schema13_missing_tool_calls` while now asserting
  `store.CurrentSchemaVersion` (= 14). The names became misleading as a direct
  result of this change, so a failure message would report "schema13" for a
  schema-14 case. The same staleness pre-exists at
  `cmd/agentdeck/main_test.go:1018-1019`. Cosmetic; fold into the next change
  that touches these files.
- **[nit] Completion note overstates the test.** The plan says the test runs
  `EXPLAIN QUERY PLAN` on "the exact query `rebuildSessions` executes"; it runs
  the exact inner `SELECT`, not the full `INSERT ... SELECT`. A faithful proxy
  for planner purposes, but the wording is imprecise, and the recorded RED
  evidence does not demonstrate the index assertion (see above — independently
  confirmed instead).

### Audit-trail findings (block the Review tick, not the code)

- Round 1's record was never written; it is reconstructed above and its verdict
  is inferred. `docs/reviews/README.md` requires a `Verdict: PASS` round in this
  file before the plan's `Review` cell may be ticked.
- `docs/reviews/usage-scan-performance/line-slice.md`'s Round 2 states the test
  body is "byte-for-byte unchanged", which no longer describes the tree:
  `TestScanCostScalesLinearlyNotQuadraticallyWithLineCount` was hardened to a
  minimum-of-3-samples measurement during this task, after that round passed.
  The plan explicitly flags this for a maintainer decision. Assessed here as
  test-only stability hardening that preserves the RED/GREEN property — each
  sample uses a fresh state directory (`state-%d`), so every attempt performs a
  real cold scan rather than a no-op incremental one, and taking the minimum is
  sound because contention can only inflate a sample. Recommendation: record it
  as a Round 3 note in `line-slice.md` rather than reopening that task's gate.

- Evidence:
  `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run 'TestV14Migration|TestV13Migration|TestMigrationFailure|TestStateMigrate|TestDoctorCLIReportsExactSchemaMatrix|TestCheckOlderAndFutureSchemas|TestCheckUsageSchemaMatrix' ./internal/store/... ./internal/doctor/... ./cmd/agentdeck/...`
  — 15 passed across 3 packages. Plus the independent `EXPLAIN QUERY PLAN`
  replay described above. A full `./...` run was not repeated: the worktree
  carries unrelated in-flight `test-coverage` work, so a full-suite result would
  not bind to this task's content state; the implementer's two clean `-count=1
  ./...` runs remain the L2 evidence for the v14 change itself.
- Verdict: **PASS** on the code. The `Review` cell may be ticked only once the
  audit-trail items above are recorded.
