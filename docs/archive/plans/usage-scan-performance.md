---
status: historical
created: 2026-07-22
retired: 2026-07-22
---

# Usage Scan Performance and Progress Plan

**Specification:** `docs/specs/cli-design.md`
(usage collection and output sections)

**Goal:** Make a full usage re-read fast enough to stop reading as a hang, and
tell the user what is happening while it runs.

## Measured Baseline

An isolated fake `HOME` holding a copy of the real sources — 471 JSONL files,
622 MB, 221,661 lines, producing 35,234 usage events and 39,365 tool-call rows
— was scanned into a fresh state directory:

| Scenario | Time |
| --- | --- |
| Cold scan (first run, or any full re-read) | 108.7 s, no output at all |
| `usage stats` after a realistic 3-hour gap (3 new sessions, 6.07 MB) | 1.73 s |
| `usage stats` with nothing new | 0.57 s |

## Remeasurement (2026-07-22, controlled A/B)

The 2026-07-22 first-pass remeasurement (single-binary, single-fixture, no
paired baseline) was reopened in review
(`docs/archive/reviews/usage-scan-performance/remeasure.md`, Round 1) for comparing
against a different, undocumented-cache-state fixture instead of a same-input
paired baseline, and for not recording an auditable source/binary identity.
This section replaces it with a controlled paired A/B measurement.

**Fixture (frozen once, reused unchanged for every run below):** an isolated
fake `HOME` populated by a single copy of the real `~/.claude/projects` and
`~/.codex/sessions` + `~/.codex/archived_sessions` trees, never touched again
after that copy. Only privacy-safe aggregates and a content digest are
recorded — no paths, session IDs, or content:
- 479 JSONL files, 639,262,720 bytes (~610 MB), 227,765 lines.
- Fixture digest `7feac752ccf0baf45ce228beec1cde042a0fdb8d53144339230c6257c9127e55`
  — a path-insensitive content-multiset digest: SHA-256 over the sorted list of
  each file's individual SHA-256, so it identifies the multiset of file bytes
  (and changes if any file's bytes or the file set changes) without recording
  what any file contains. It is insensitive to file names, paths, or a pure
  rename/move, so it does not by itself prove path layout — path identity
  across the baseline and current runs comes from reusing the same frozen
  `$FAKE_HOME` directory tree for every sample below, not from the digest.

**Baseline binary (pre-optimization):** built from commit `c6c54c4`
(`c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae`, the repository `HEAD` at
remeasurement time) exported with `git archive HEAD | tar -x -C <tmpdir>` into
a temporary directory, leaving the working tree untouched. Confirmed
pre-optimization by inspection of the exported source:
`internal/usage/usage.go:852` still reads
`strings.IndexByte(string(line), '\n')` and `internal/store/store.go:20` still
reads `CurrentSchemaVersion = 13`. Built with
`env GOCACHE=/private/tmp/agent-deck-go-build go build -mod=vendor -o agentdeck-baseline ./cmd/agentdeck`
from that temporary export. Binary SHA-256:
`d62d845d956ae8a9bf1fc288619ce576e63127b9bc5cd9000f4bb3923f13952f`.

**Current binary:** built directly from the actual working tree (no copy) at
the same moment, i.e. `c6c54c4` plus the uncommitted working-tree diff, with
`env GOCACHE=/private/tmp/agent-deck-go-build go build -mod=vendor -o agentdeck-current ./cmd/agentdeck`.
Binary SHA-256:
`982507145d8f330d164ff9008d60faf4f2e3d3349db32913ef9efb3b65112630`.
Diff digest of the full uncommitted `git diff` at build time (`git diff |
shasum -a 256`):
`017c95b716c276c91cec68bba74736c3c53d303608709e2de290c3c50bcc5ef6`. Diff
digest restricted to the four tracked files that affect the built binary —
`cmd/agentdeck/main.go`, `internal/store/migrations.go`,
`internal/store/store.go`, `internal/usage/usage.go` (`git diff -- <those
files> | shasum -a 256`):
`1edf347061e5b61e7fe07000566b9b8e3375ce9ba42914d576c427dd902bf0fb`. (The
remaining diffed files at that moment were test files and documentation; they
do not affect the built binary.)

**Environment:** `go version go1.26.5 darwin/amd64`; `Darwin 25.6.0 x86_64`
(machine identity omitted as non-essential/sensitive).

**Method:** for each run, delete `$FAKE_HOME/.agentdeck` (forces a fresh
AgentDeck state — every run is a genuine cold scan against the frozen
fixture), then time the scan with `/usr/bin/time -p`, wrapping the target
invocation in a subshell so the child's own stdout/stderr are discarded and do
not interleave with `time`'s report (the current binary's stderr carries
progress-output lines that are otherwise not sensitive but are irrelevant
noise for a timing log):

```
rm -rf "$FAKE_HOME/.agentdeck"
/usr/bin/time -p sh -c 'env HOME="$0" "$1" usage scan >/dev/null 2>/dev/null' "$FAKE_HOME" "$BIN"
```

where `$BIN` is `agentdeck-baseline` or `agentdeck-current`. Order alternated
AB/BA/AB (baseline, current, current, baseline, baseline, current) so cache
warmth and background machine load do not systematically favor either binary.
This replaces the first controlled-A/B pass (`date +%s.%N` deltas around the
process, no retained raw output), reopened in review Round 2 for not recording
an auditable timer identity or genuine raw output; it reused the same still-
frozen fixture (digest reverified unchanged before rerunning) and the same two
binaries (SHA-256 reverified unchanged), rerun rather than reconstructed from
the earlier table.

Raw `/usr/bin/time -p` output, one block per run, unedited:

```
--- run 1 (baseline) ---
real 112.61
user 179.16
sys 48.71
--- run 2 (current) ---
real 19.86
user 19.74
sys 3.56
--- run 3 (current) ---
real 20.91
user 20.93
sys 3.85
--- run 4 (baseline) ---
real 106.40
user 176.32
sys 49.82
--- run 5 (baseline) ---
real 96.63
user 163.14
sys 43.84
--- run 6 (current) ---
real 17.73
user 18.33
sys 3.04
```

(`user` exceeds `real` for the baseline runs because the scan does concurrent
work across multiple goroutines/threads — expected, not a measurement error.)

| Run | Binary | Wall time (s, `real`) |
| --- | --- | --- |
| 1 | baseline | 112.61 |
| 2 | current | 19.86 |
| 3 | current | 20.91 |
| 4 | baseline | 106.40 |
| 5 | baseline | 96.63 |
| 6 | current | 17.73 |

| Binary | min | mean |
| --- | --- | --- |
| baseline | 96.63 s | 105.213 s |
| current | 17.73 s | 19.5 s |

**Direct A/B factor (this paired measurement, same fixture, same machine,
interleaved order):** min-to-min **5.45x**, mean-to-mean **5.40x**.

The original `108.7 s` baseline in `## Measured Baseline` above is kept only
as historical reference — its exact fixture bytes were not preserved, so it is
not used to compute any improvement factor here. It is, however, consistent
with this run's freshly-built pre-optimization baseline (96.6-112.6 s), which
is the expected result since both are the same unoptimized code path scanning
a similarly sized real-source fixture.

**Limitations:**
- Fixture is real machine session data, not a checked-in or byte-identical
  reproduction of the original 471-file fixture; it exists only in this
  session's scratch directory, was never committed, and contains no recorded
  paths, session IDs, or content beyond the aggregate counts and digest above.
- Disk/page cache state was not explicitly controlled (no `purge`/drop-caches
  step between runs), but the AB/BA/AB interleaving is the standard mitigation
  for that class of bias, and both binaries alternate through both prior-run
  ordinal positions.
- Only the cold-scan scenario was remeasured, per this task's scope; the two
  warm `usage stats` rows in `## Measured Baseline` were not rerun.
- Six single-process wall-clock samples on one machine (three per binary); no
  statistical significance test was run beyond min/mean across three samples
  per side, consistent with how the existing `line-slice` micro-benchmark test
  measures the same class of cost.
- No production code was changed as a result of this remeasurement.

## What Is Not the Problem

The original backlog entry guessed the cause was redundant re-scanning or
re-aggregation. Profiling disproved that; it is recorded here so the same
direction is not re-investigated.

- Scanning is already correctly incremental. Per-source cursors plus append
  verification mean unchanged files are never re-read. Stat-ing the whole
  471-file inventory is only part of the 0.57 s floor.
- Reports are not slow. `Stats` and `Summary` aggregate in a five-query
  profile that a regression test already pins at exactly five queries for both
  3 events and 1003 events.
- JSON parsing is not a bottleneck: 5.7 s of the 108.7 s, even though 79% of
  lines are discarded as non-usage. A substring pre-filter before unmarshaling
  measured 2.3 s versus 5.7 s. **Do not implement it** — roughly 3 s of savings
  is not worth the complexity, and a hand-written pre-filter risks diverging
  from the parser's real acceptance rules.

## Profiled Root Causes

**1. `internal/usage/usage.go:852` is accidentally quadratic — 70.9 s, 65% of
the total.** The line loop calls `strings.IndexByte(string(line), '\n')`, and
`string(line)` copies the entire remaining file buffer on *every line
iteration*, so one file of size `S` with `L` lines copies about `L x S/2`
bytes. Replaying only this loop over the same 622 MB takes 70.9 s;
`bytes.IndexByte(line, '\n')` over identical input takes 0.32 s — a 218x
difference for a semantically identical operation.

Because the cost is quadratic in **individual file size**, it degrades
superlinearly as single sessions grow. That, not elapsed time since the last
scan, is the real reason the tool feels slower the longer it is used.

`internal/session` already uses `bytes.LastIndexByte` and `bytes.Split`, so it
does not share the defect. `usage.go:1786`'s `strings.Split(string(out), "\n")`
is a one-shot split outside any loop and is fine.

**2. `usage_events` has no index on `(client, session_id)` — about 13 s, 12%.**
`rebuildSessions` runs
`INSERT INTO usage_sessions ... SELECT ... WHERE client=? AND session_id=?`
once per affected session per file, and `EXPLAIN QUERY PLAN` reports
`SCAN usage_events` for it against a table that grows throughout the scan. The
sibling queries are already indexed: `usage_events_source` serves
`affectedSessions`, and the `event_key` autoindex serves `upsertTx`. Measured
by A/B — adding only this index took the same cold scan from 108.7 s to 95.6 s.

**3. The remaining ~19 s is legitimate work**: event and tool-call upserts,
per-file transactions, snapshot revalidation, and activity parsing.

Fixing 1 and 2 should bring a full re-read to roughly 25 s with no output
contract change.

## Why a Full Re-Read Is Not Rare

`scanFileMode` sets `reset` when `parserVersion != usageParserVersion`, so
every release that bumps the parser version silently re-reads every source on
the next scan. The parser has already gone v1 to v2 to v3, meaning users have
paid this cost after upgrades with no indication of why the command suddenly
took two minutes.

## Tasks

Task content lives here; per-gate status lives in the matrix below.

1. **line-slice** — Replace `strings.IndexByte(string(line), '\n')` at
   `internal/usage/usage.go:852` with `bytes.IndexByte(line, '\n')`. The two are
   semantically identical, so no output contract changes. Add coverage that
   would actually fail on a regression — assert that scan cost scales roughly
   linearly with file size rather than asserting a wall-clock threshold, which
   would be flaky in CI.
   **Completion note (2026-07-22):** Replaced the line at `usage.go:852`.
   Added `TestScanCostScalesLinearlyNotQuadraticallyWithLineCount` in
   `internal/usage/usage_test.go`, which measures `Service.Scan` over 2,000 vs
   8,000 non-usage lines (isolating the split-loop cost from JSON/DB work) and
   fails if the ratio exceeds 8x (linear predicts ~4x). Verified RED against
   the reverted line (14.9x–17.2x measured, fails as expected) and GREEN
   against the fix (ratio well under threshold, 0.08s). Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage/...`,
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`,
   `env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`,
   and `git diff --check` — all pass.
   **Post-review update (2026-07-22, during session-index):** the ratio test
   flaked once under `go test ./...` (10.1x, just above the 8x threshold) due
   to cross-package scheduling contention — the residual risk already named as
   a nit in Round 1 review. Hardened `measure()` to take the minimum of 3
   samples per size rather than a single sample (a contended run can only
   inflate a sample, never deflate it below true cost). Re-verified RED/GREEN
   and two independent `-count=1 ./...` runs, both clean. This touches an
   already-reviewed task's test file without reopening its Review gate here —
   flagged for the maintainer to decide whether it needs its own re-review
   pass.
2. **session-index** — Add schema v14 with
   `CREATE INDEX usage_events_client_session ON usage_events(client, session_id)`
   and confirm with `EXPLAIN QUERY PLAN` that `rebuildSessions` no longer reports
   `SCAN usage_events`. Migration only; no data rewrite.
   **Completion note (2026-07-22):** Added schema v14
   (`internal/store/migrations.go`) and bumped `CurrentSchemaVersion` to 14
   (`internal/store/store.go`). Added
   `TestV14MigrationAddsUsageEventsClientSessionIndex`
   (`internal/store/store_test.go`), which runs
   `EXPLAIN QUERY PLAN` on the exact query `rebuildSessions` executes and
   asserts no plan row contains `SCAN usage_events` while one reports using
   `usage_events_client_session`. RED/GREEN verified (reverting the migration
   makes the test fail on the version check). Rollback/failure behavior is
   already covered generically by the existing, version-agnostic
   `TestMigrationFailurePreservesLastUsableSchema`; no v14-specific rollback
   test was needed. Fixed three pre-existing tests whose literals assumed
   `CurrentSchemaVersion == 13`:
   `internal/store/store_test.go` (`TestV13MigrationAddsSafeToolActivityStorage`,
   now compares against the `CurrentSchemaVersion` symbol),
   `internal/doctor/doctor_test.go` (two hardcoded `13` rows, same fix, matching
   the sibling test in `cmd/agentdeck/main_test.go` that already used the
   symbol), and `cmd/agentdeck/main_test.go`
   (`TestStateMigrateTextAndJSONUpgradeSchema12`, whose "simulate a v12
   database" technique — bootstrap fully, then drop the newest table and
   rewind the version — did not also drop the new index on the pre-existing
   `usage_events` table, so replaying migration 14 hit `index already exists`;
   fixed by dropping the index alongside the table before the rewind).
   Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go build -mod=vendor ./...`,
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store/...`,
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/doctor/...`,
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck/...`,
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...` (run twice, both clean),
   `env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`,
   and `git diff --check` — all pass.
   **Round 1 review fixes (2026-07-22), all adopted:** removed a redundant
   `version < 14` condition in `TestV14MigrationAddsUsageEventsClientSessionIndex`
   (already implied by `version != CurrentSchemaVersion`); widened that test
   from 2 to 60 distinct-session rows so the `EXPLAIN QUERY PLAN` assertion
   does not depend on the query planner's tiny-table heuristics still choosing
   the index; and added the same `DROP INDEX usage_events_client_session` fix
   to the sibling simulated-v12 test in
   `internal/doctor/doctor_test.go` (`TestCheckOlderAndFutureSchemasAreSafeAndReadable`)
   for consistency, even though it does not call `state migrate` and was not
   failing. Re-verified: targeted test, `./internal/store/...`,
   `./internal/doctor/...`, two independent `-count=1 ./...` runs, `go vet`,
   `git diff --check` — all pass.
3. **progress-output** — Report progress for long scans on **stderr**, never
   stdout, so JSON and NDJSON stay machine-parseable. Emit nothing for the first
   second so the common 0.6-2 s path stays silent, then report processed and
   total source files. Honor `--quiet` by suppressing entirely, emit no ANSI
   escapes when stderr is not a TTY, and cover the implicit pre-scan inside
   `usage stats` and `usage summary`, not just explicit `usage scan` and
   `usage rebuild`.
   **Completion note (2026-07-22):** Added scanner/rebuild progress lifecycle
   hooks and a CLI reporter that starts after one second, writes only to stderr,
   uses ANSI redraws only for a TTY, and becomes a no-op for `--quiet`. The
   shared service hook covers `usage scan`, `usage rebuild`, and the synchronous
   pre-scans in `usage stats` and `usage summary`. Added service, command-path,
   and output tests for processed/total reporting, the implicit paths, first
   second silence, non-TTY output, and quiet suppression. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage/... ./cmd/agentdeck/...`
   (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass); and `git diff --check` (pass).
   **Post-review repair (2026-07-22):** `Stop` now waits for the refresh
   goroutine and then redraws the final processed/total state only if progress
   was already visible. Replaced the real-time reporter test with manual
   timer/ticker coverage, added TTY final-redraw/newline and JSON stderr/stdout
   separation assertions, and retained the no-output quiet path. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck/... ./internal/usage/...`
   (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass); and `git diff --check` (pass).
4. **reread-notice** — When a scan is triggered by a parser version change
   rather than by new data, say so in that progress output. A two-minute wait
   after an upgrade is acceptable if it is explained; an unexplained one reads
   as a hang.
   **Completion note (2026-07-22):** Added parser-version-only detection to the
   inventory and propagated `re-reading after parser update` through every scan
   progress update. The reason is withheld when source metadata also indicates
   new or appended data, and explicit rebuilds remain unlabelled. Added service
   and deterministic renderer coverage. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage/... ./cmd/agentdeck/...`
   (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass); and `git diff --check` (pass).
5. **no-scan-flag** — Add `--no-scan` to `usage stats` and `usage summary` for
   callers who want the stored aggregate immediately. Keep the synchronous
   pre-scan as the default: reporting silently stale numbers is worse than
   waiting, and the existing `scan_incomplete` partial-warning contract assumes
   a scan was attempted. Do not move the scan to a background goroutine — that
   would make report output depend on a race.
   **Completion note (2026-07-22):** Added command-local `--no-scan` flags that
   skip only the synchronous pre-scan and query the stored aggregates directly;
   the default command paths retain their pre-scan and partial-warning behavior.
   Command-path coverage verifies both default reporters and no-scan suppression.
   Evidence: `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
   ./cmd/agentdeck/... ./internal/usage/...` (pass); `env
   GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` (pass);
   and `git diff --check` (pass).
6. **remeasure** — Re-measure the cold scan against the same fixture and record
   the result in this document.
   **Completion note (2026-07-22):** First pass measured a single current
   binary against a freshly-copied fixture with no paired baseline; reopened
   in review for lacking a same-fixture paired comparison and an auditable
   source/binary identity
   (`docs/archive/reviews/usage-scan-performance/remeasure.md`, Round 1).
   **Repair (2026-07-22):** replaced it with a controlled paired A/B
   measurement in `## Remeasurement (2026-07-22, controlled A/B)`: a
   pre-optimization baseline binary built from commit `c6c54c4` in a temporary
   `git archive` export (working tree untouched) and the current binary built
   from the actual working tree, both run against one frozen fixture (identity
   recorded only as aggregate counts plus a digest-of-digests, no paths,
   session IDs, or content), fresh AgentDeck state per run, AB/BA/AB
   interleaving. Reopened again in Round 2
   (`docs/archive/reviews/usage-scan-performance/remeasure.md`) for using
   `date +%s.%N` deltas with no retained raw timer output, and for
   overstating the fixture digest's path sensitivity.
   **Repair (2026-07-22, round 2):** kept the same frozen fixture (digest
   reverified unchanged) and the same two binaries (SHA-256 reverified
   unchanged) and reran the A/B comparison with `/usr/bin/time -p`, recording
   the six genuine raw output blocks verbatim in
   `## Remeasurement (2026-07-22, controlled A/B)`. Reworded the fixture
   digest as a path-insensitive content-multiset digest and noted that path
   identity comes from reusing the same frozen directory, not the digest.
   Updated result: baseline min 96.63 s / mean 105.213 s, current min 17.73 s
   / mean 19.5 s — **5.45x (min) / 5.40x (mean)** direct improvement (values
   shifted from the round-1 repair's 5.27x/5.11x because these are genuine
   reruns, not the same samples). No defect surfaced, so no production code
   was changed. Evidence: baseline/current binary SHA-256, diff digests,
   fixture digest, `go version`, exact `/usr/bin/time -p` invocation, and the
   six raw output blocks are recorded above; `rtk git diff --check` passes on
   the documentation-only change.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | line-slice | ✅ | ✅ |
| 2 | session-index | ✅ | ✅ |
| 3 | progress-output | ✅ | ✅ |
| 4 | reread-notice | ✅ | ✅ |
| 5 | no-scan-flag | ✅ | ✅ |
| 6 | remeasure | ✅ | ✅ |

Done: 6/6. Task 1 passed Round 2 review (see
`docs/archive/reviews/usage-scan-performance/line-slice.md`, extended with a Round 3
test-hardening note that does not reopen its gate). Task 2 passed Round 2
review (see `docs/archive/reviews/usage-scan-performance/session-index.md`). Task 3
passed Round 3 review (see
`docs/archive/reviews/usage-scan-performance/progress-output.md`). Task 4 passed Round
2 review (see `docs/archive/reviews/usage-scan-performance/reread-notice.md`). Task 5
passed Round 2 review (see
`docs/archive/reviews/usage-scan-performance/no-scan-flag.md`). Task 6 passed
Round 3 review (see
`docs/archive/reviews/usage-scan-performance/remeasure.md`). The
implementer ticks **Dev** once a task is built and its targeted
verification passes; an independent reviewer ticks **Review** once findings are
closed, and reopens the task rather than ticking it when review finds problems.
A task is done only when Review is ticked.

## Starting a task

Turn any row of the Status matrix into a scoped development instruction through
its anchor — no fresh prompt needs to be written by hand:

> **进入开发:usage-scan 性能与进度 / `<task-anchor>`**
> 阅读 `AGENTS.md`、本 plan `## Tasks` 中 `<task-anchor>` 一条及它命名的文件、本
> plan 的 `## Profiled Root Causes` 与 `## Required Verification`,以及
> `docs/README.md` 的验证路由。只在该 task 的范围内实现并自测。完成后在
> `## Status` 勾上该行的 `Dev`,把命令与结果记进该 task 的完成注记;评审留痕到
> `docs/archive/reviews/usage-scan-performance/<task-anchor>.md`。

Example — `line-slice`: 阅读 `internal/usage/usage.go:852` 附近的行循环与既有
usage 扫描测试;把 `strings.IndexByte(string(line), '\n')` 换成
`bytes.IndexByte(line, '\n')`,并加一个断言扫描成本随文件大小近似线性的回归测试。

## Required Verification

L2 for the scan path and output contract. The schema v14 migration
additionally requires the migration and rollback tests. Commands are listed in
`AGENTS.md` under "Testing and Verification".
