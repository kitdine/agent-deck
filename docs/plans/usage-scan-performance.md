---
status: active
created: 2026-07-22
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
2. **session-index** — Add schema v14 with
   `CREATE INDEX usage_events_client_session ON usage_events(client, session_id)`
   and confirm with `EXPLAIN QUERY PLAN` that `rebuildSessions` no longer reports
   `SCAN usage_events`. Migration only; no data rewrite.
3. **progress-output** — Report progress for long scans on **stderr**, never
   stdout, so JSON and NDJSON stay machine-parseable. Emit nothing for the first
   second so the common 0.6-2 s path stays silent, then report processed and
   total source files. Honor `--quiet` by suppressing entirely, emit no ANSI
   escapes when stderr is not a TTY, and cover the implicit pre-scan inside
   `usage stats` and `usage summary`, not just explicit `usage scan` and
   `usage rebuild`.
4. **reread-notice** — When a scan is triggered by a parser version change
   rather than by new data, say so in that progress output. A two-minute wait
   after an upgrade is acceptable if it is explained; an unexplained one reads
   as a hang.
5. **no-scan-flag** — Add `--no-scan` to `usage stats` and `usage summary` for
   callers who want the stored aggregate immediately. Keep the synchronous
   pre-scan as the default: reporting silently stale numbers is worse than
   waiting, and the existing `scan_incomplete` partial-warning contract assumes
   a scan was attempted. Do not move the scan to a background goroutine — that
   would make report output depend on a race.
6. **remeasure** — Re-measure the cold scan against the same fixture and record
   the result in this document.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | line-slice | ⬜ | ⬜ |
| 2 | session-index | ⬜ | ⬜ |
| 3 | progress-output | ⬜ | ⬜ |
| 4 | reread-notice | ⬜ | ⬜ |
| 5 | no-scan-flag | ⬜ | ⬜ |
| 6 | remeasure | ⬜ | ⬜ |

Done: 0/6. The implementer ticks **Dev** once a task is built and its targeted
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
> `docs/reviews/usage-scan-performance/<task-anchor>.md`。

Example — `line-slice`: 阅读 `internal/usage/usage.go:852` 附近的行循环与既有
usage 扫描测试;把 `strings.IndexByte(string(line), '\n')` 换成
`bytes.IndexByte(line, '\n')`,并加一个断言扫描成本随文件大小近似线性的回归测试。

## Required Verification

L2 for the scan path and output contract. The schema v14 migration
additionally requires the migration and rollback tests. Commands are listed in
`AGENTS.md` under "Testing and Verification".
