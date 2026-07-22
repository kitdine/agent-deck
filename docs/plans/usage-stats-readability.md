---
status: active
created: 2026-07-22
---

# Usage Stats Display Readability Plan

**Specification:** `docs/specs/cli-design.md`
(usage text output contract, "Default `text` output is a human-facing
interactive contract, not a serialized representation of internal DTOs")

**Goal:** Keep `usage stats` text scannable as the underlying data grows.
Sections that today expand without bound — the trend, the model/provider/unpriced
lists, and the cache block — should stay a fixed, glanceable size, while
`--format json` keeps carrying the complete set.

## Measured Baseline

Rendered from the real state database (48.5 MB), `NO_COLOR=1`, `COLUMNS=100`.

| Invocation | Total lines | Notes |
| --- | --- | --- |
| `usage stats --period all` | 140 | one screen-and-a-half |
| `usage stats --period 30d --group-by hour` | **822** | 731 of them are bar rows |

Per-section line spans and the row counts behind them, for `--period all`:

| Section | Lines | Underlying rows (JSON) | Capped today? |
| --- | --- | --- | --- |
| CACHE HIT RATE | 49 | 240 cache sessions | sessions only, at 10 |
| 🤖 MODELS | 32 | 15 models | no |
| 🗓 TREND | 19 | 17 buckets | no |
| PROVIDERS | 12 | 5 providers | no |
| ▦ ACTIVITY (heatmap) | 11 | 168 cells → fixed 7×24 grid | already a summary |
| UNPRICED MODELS | 5 | 4 models | no |
| CLIENTS | 4 | 2 clients | no |

The activity heatmap already collapses 168 cells into a fixed 7×24 grid and is
not part of this problem. Everything else scales with row count.

## What Is Not the Problem

- JSON output. `--format json` is the machine contract and must keep every row;
  this plan changes only the text presentation. The spec already separates the
  two: text is "a human-facing interactive contract", and the JSON envelope
  "remains stable independently of text presentation".
- Computation. The report already carries everything needed; this is a rendering
  change in `cmd/agentdeck/usage_stats_text.go`, not in `internal/usage`.
- The heatmap. It is already a bounded summary.

## Root Causes

**1. There is no shared truncation rule; exactly one section caps.** Cache
*sessions* stop at 10 with a `+K more cache sessions in JSON` footer
(`usage_stats_text.go:357-373`, pinned by
`TestUsageStatsCacheSessionsAreCappedOnlyInText`). Every other list renders all
rows: MODELS (`limit := len(r.report.Models)`, line 226), PROVIDERS (line 301),
UNPRICED (line 409), per-model CACHE (line 347), CLIENTS (line 275). The
precedent for "cap the text, keep JSON whole" already exists and is applied in
exactly one place.

**2. The trend has no bucket ceiling.** `trendLines` (line 202) emits one bar
per bucket. Auto grouping stays bounded (17 buckets for `--period all`), but an
explicit `--group-by hour` over 30 days produces 709 buckets and an 822-line
wall. A trend is a time series, so it cannot be top-N-ranked without breaking the
timeline — it needs a contiguous-window cap instead.

**3. Dense detail lines wrap and double the height.** Each model and provider
emits a bar row plus a wide middle-dot-joined detail line, e.g.
`84.9M tokens · 1.6% · unavailable · UNPRICED · 69 sessions · 57 tools · 88.14% hit`.
Below roughly width 104 this wraps to two lines (`statsWrap`, line 258/320), so a
15-model list is ~30 rows at common terminal widths. This is the "wide … hard to
scan" half of the backlog note.

## Tasks

Task content lives here; per-gate status lives in the matrix below.

1. **shared-topn** — Extract the cache-session cap into one helper (top-N rows in
   the section's existing rank order, then a `+K more in JSON` footer) and apply
   it to MODELS, PROVIDERS, UNPRICED, and the per-model CACHE list. Keep JSON
   complete. Defaults chosen for a glance, not exhaustiveness (starting point:
   models 8, providers 8, unpriced 12, per-model cache 8; CLIENTS stays uncapped
   — it is inherently tiny). Generalize, do not duplicate, the existing
   `TestUsageStatsCacheSessionsAreCappedOnlyInText` pattern across each newly
   capped section, asserting the footer text and that JSON still holds every row.
2. **trend-cap** — Bound the TREND section to a fixed number of buckets (starting
   point: 48). Because the trend is a time series, keep the most recent
   contiguous window and add a `+K earlier buckets in JSON` note rather than
   ranking by value; state in the note that older buckets are omitted. JSON keeps
   all buckets. Add a test for the `--group-by hour --period 30d` case asserting
   a bounded bar-row count and the overflow note.
3. **top-flag** — Add `--top N` to `usage stats` (0 means "all", restoring
   today's behavior) so a caller can widen the text caps without switching to
   JSON. Default keeps the scannable caps from task 1. Document it in
   `docs/specs/cli-manual.md`. This is the explicit escape hatch that makes the
   caps safe to add.
4. **detail-compaction** — Keep each model/provider detail on one line at width
   ≥ 80 by holding the high-value fields inline (tokens · share · cost ·
   sessions) and showing the secondary fields (tools, cache-hit) only when they
   fit, with the full set always present in JSON. This changes the text output
   shape the most, so it carries the golden-file updates and must clear review
   before its Review gate is ticked.
5. **remeasure** — Re-render the same two fixtures (`--period all` and
   `--period 30d --group-by hour`) and record the new line counts in this
   document.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | shared-topn | ⬜ | ⬜ |
| 2 | trend-cap | ⬜ | ⬜ |
| 3 | top-flag | ⬜ | ⬜ |
| 4 | detail-compaction | ⬜ | ⬜ |
| 5 | remeasure | ⬜ | ⬜ |

Done: 0/5. The implementer ticks **Dev** once a task is built and its targeted
verification passes; an independent reviewer ticks **Review** once findings are
closed, recording the round in `docs/reviews/usage-stats-readability/`. A task
is done only when Review is ticked.

## Formatting invariants

- **Cache-hit percentages render as `NN.NN%` — two decimals — at every site**
  (model detail, provider detail, and the `MODEL`/`SESSION` cache lines). They
  already arrive that way from `percentPointer` (`internal/usage/usage.go:2182`,
  `FloatString(2)`), so the renderer must pass them through unrounded.
  `detail-compaction` must not shorten `91.85% hit` to `92% hit`; an early
  mockup did, and that is wrong. Share percentages are a different quantity and
  keep their existing one-decimal `formatPercent` form.

## Starting a task

Turn any row of the Status matrix into a scoped development instruction through
its anchor — no fresh prompt needs to be written by hand:

> **进入开发:usage-stats 展示可读性 / `<task-anchor>`**
> 阅读 `AGENTS.md`、本 plan `## Tasks` 中 `<task-anchor>` 一条及它命名的文件、本
> plan 的 `## Formatting invariants` 与 `## Required Verification`,以及
> `docs/README.md` 的验证路由。只在该 task 的范围内实现并自测,不改 JSON 输出或
> `internal/usage` 计算。完成后在 `## Status` 勾上该行的 `Dev`,把命令与结果记进
> 该 task 的完成注记;评审留痕到
> `docs/reviews/usage-stats-readability/<task-anchor>.md`。

Example — `shared-topn`: 阅读 `cmd/agentdeck/usage_stats_text.go` 的
`rankingLines`、`cacheLines`、`unpricedLines`,以及既有
`TestUsageStatsCacheSessionsAreCappedOnlyInText`;产出把会话封顶推广为一个共享
top-N + `+K more in JSON` helper 的实现,并为每个新封顶的区块加对应文本测试。

## Required Verification

L2: this is a shared CLI text-output contract change. Targeted
`go test ./cmd/agentdeck` for the renderer and golden files, then
`go test -mod=vendor ./...` once after the final edit. No schema, concurrency,
or credential surface is touched, so L3 checks are not required. Commands are
listed in `AGENTS.md` under "Testing and Verification".
