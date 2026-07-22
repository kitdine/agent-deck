---
status: historical
created: 2026-07-22
retired: 2026-07-22
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

## Remeasurement (2026-07-22, controlled A/B)

The first remeasurement pass (single current binary against the live
database at whatever size it had grown to) was reopened in review
(`docs/archive/reviews/usage-stats-readability/remeasure.md`, Round 1) for comparing
against different underlying data than the original 140/822 baseline and for
attributing the difference to tasks 1-4 without a paired same-content
comparison. This section replaces it with a controlled paired A/B: a
pre-usage-stats-readability binary and the current binary, both run against
one frozen database snapshot.

**Snapshot (frozen once, a fresh disposable copy made from it before each of
the four runs below, never mutated itself):** a copy of the real
`~/.agentdeck/agentdeck.sqlite3` (51,941,376 bytes). Content identity —
SHA-256 `2434b8cad72915de1415f2f0ea0bc585f1499f2b164321058e9b011486a66467` —
was reverified unchanged after all four runs. A same-file copy was tried
first without per-run refresh and its hash changed after a single
`--no-scan` read (`usage stats` still writes through some metadata on open
even without scanning), so each run instead got a fresh copy from this
frozen reference, verified byte-identical (same SHA-256) immediately before
that run — the reference itself was never opened by either binary directly.
The real `~/.agentdeck/agentdeck.sqlite3` was only ever read from to make
this copy; its mtime was confirmed unchanged (predates this session)
throughout, and no real Codex/Claude session source was read or scanned by
either binary (`--no-scan` on every invocation).

**Baseline binary (pre-usage-stats-readability):** none of this plan's five
tasks have been committed yet — the repository `HEAD` at remeasurement time,
`5bf3356` (`5bf3356aa26f45e170b625930831a809a328ad17`), is therefore already
the correct pre-change revision; confirmed by grepping the exported source
for `statsTopN`/`statsCompactDetail`/`statsTwoColumnFits` (all absent).
Exported via `git archive HEAD | tar -x -C <tmpdir>` into a temporary
directory — no branch or worktree was created — and built with
`env GOCACHE=/private/tmp/agent-deck-go-build go build -mod=vendor -o agentdeck-baseline ./cmd/agentdeck`.
Binary SHA-256:
`4b6a4525ea52eb82b5f91e0b4f491ff8e4b61aa695d3a1afd29f91474f269186`.

**Current binary:** built directly from the actual working tree (no copy),
i.e. `5bf3356` plus the uncommitted tasks 1-5 diff, with the same build
command (`-o agentdeck-current`). Binary SHA-256:
`5138d0401d8a35bef499b77d373547e5985c4a2d6a1b496a89b74a410f815d23`. Diff
digest of the full uncommitted `git diff` at build time (`git diff | shasum
-a 256`): `5face1073e2bc551c7221abd019ae0b5f81ce2bdbe5a768387a732808593ea95`.
Diff digest restricted to the two tracked files that affect the built
binary's `usage stats` behavior — `cmd/agentdeck/main.go`,
`cmd/agentdeck/usage_stats_text.go` (`git diff -- <those files> | shasum -a
256`): `d0e68ffc45f707b14e3dd5bd5df31652a15a3fa22981c1a92be51944615dc08c`.

**Environment:** `go version go1.26.5 darwin/amd64`; `Darwin 25.6.0 x86_64`.

**Method:** for each of the four runs, copy the frozen snapshot into a fresh
`--state-dir`, verify its SHA-256 matches the frozen reference, then run
`env NO_COLOR=1 COLUMNS=100 <binary> --state-dir <fresh-copy-dir> usage
stats <period-args> --no-scan`, counting output lines with `awk
'END{print NR}'`.

| Binary | Invocation | Command | Lines |
| --- | --- | --- | --- |
| baseline | `--period all` | `agentdeck-baseline --state-dir <copy> usage stats --period all --no-scan` | 139 |
| current | `--period all` | `agentdeck-current --state-dir <copy> usage stats --period all --no-scan` | 120 |
| baseline | `--period 30d --group-by hour` | `agentdeck-baseline --state-dir <copy> usage stats --period 30d --group-by hour --no-scan` | 832 |
| current | `--period 30d --group-by hour` | `agentdeck-current --state-dir <copy> usage stats --period 30d --group-by hour --no-scan` | 142 |

**Direct A/B result (same snapshot, same machine):**
`--period all`: 139 to 120, **-13.7%**.
`--period 30d --group-by hour`: 832 to 142, **-82.9%**.

The original `140`/`822` baseline figures in `## Measured Baseline` above are
kept only as historical reference — they were captured against the database
at an earlier, smaller size (48.5 MB vs. this snapshot's ~49.5 MB) — and are
not used to compute any change here; the close resemblance to this run's own
baseline (139/832) is incidental data drift, not evidence of anything.

Content was also inspected directly, not just line-counted: the baseline
`--period all` output lists all 15 models with no footer (confirming the
exported `HEAD` binary genuinely predates `shared-topn`), while the current
output caps MODELS at 8 with a `+7 more models in JSON` footer.

Because this is now a genuine paired same-content comparison, the causal
attributions are direct read-offs of what changed in the diff between the
two binaries, not inferred from mismatched inputs:
- `--period all` (-13.7%): `shared-topn`'s caps (MODELS 15 to 8 rows plus a
  footer; cache sessions still capped at 10, now via the shared helper;
  per-model CACHE similarly capped) reduce rows, partly offset by
  `detail-compaction` making every model/provider detail reliably one line
  instead of sometimes shorter through accidental wrap-driven truncation.
- `--period 30d --group-by hour` (-82.9%): dominated by `trend-cap` — the
  baseline binary's trend section rendered all of this snapshot's hourly
  buckets uncapped; the current binary caps it to the 48-bucket window plus
  a `+K earlier buckets in JSON` footer — with the same `shared-topn` caps
  from the first fixture applying to its MODELS/PROVIDERS/etc. sections too.

**Limitations:**
- Only the two required fixtures were remeasured, per this task's scope; the
  finer per-section line-span table in `## Measured Baseline` was not
  reproduced, since the dataset's model/provider/bucket mix has changed
  shape since that table was captured, making a like-for-like per-section
  comparison less meaningful than the paired top-line totals above.
- Single-machine, single-process wall-independent line counts (no repeated
  sampling was needed here — line count is deterministic given identical
  binary and identical input, unlike a timing measurement).
- No production code, test, JSON output, or `internal/usage` computation was
  changed to produce this result.

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
   **Completion note (2026-07-22):** Added a generic `statsTopN[T any](items
   []T, limit int) []T` helper plus a `(statsTextRenderer).topNFooterLine`
   method in `cmd/agentdeck/usage_stats_text.go`, and applied both to MODELS,
   PROVIDERS, UNPRICED MODELS, the per-model CACHE list, and refactored the
   existing CacheSessions cap onto the same helper (its cap value and footer
   text are unchanged, now via `statsCacheSessionsCap = 10`). New caps:
   `statsModelsCap = 8`, `statsProvidersCap = 8`, `statsUnpricedCap = 12`,
   `statsModelCacheCap = 8`. CLIENTS was left uncapped, as scoped. Existing
   rank order is preserved because capping takes a prefix of the
   already-rank-ordered slice (models/providers arrive sorted by share
   descending, so the bar-scaling `maximum` computed only from the shown
   prefix is still the true maximum). The per-model CACHE list caps the
   already-filtered (has cache data) subset of `Models`, independent of the
   MODELS section's own cap. Only `cmd/agentdeck/usage_stats_text.go` and its
   test file changed; `internal/usage` and the JSON envelope are untouched.
   Generalized `TestUsageStatsCacheSessionsAreCappedOnlyInText`'s pattern into
   four new tests — `TestUsageStatsModelsAreCappedOnlyInText`,
   `TestUsageStatsProvidersAreCappedOnlyInText`,
   `TestUsageStatsUnpricedModelsAreCappedOnlyInText`,
   `TestUsageStatsModelCacheRowsAreCappedOnlyInText` — each building one more
   fixture row than that section's cap, asserting the exact rendered-row count,
   the `+K more <label> in JSON` footer text, and that the underlying report
   slice (the JSON-equivalent data) still holds every row. Cache-hit percentage
   formatting (`FloatString(2)`-sourced, unrounded) was not touched. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass, includes golden-file and new cap tests);
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build go vet
   -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check` (clean).
   **Repair (2026-07-22):** addressed both Round 1 findings
   (`docs/archive/reviews/usage-stats-readability/shared-topn.md`). For the four newly
   capped sections (models, providers, unpriced models, per-model cache),
   added explicit retained-order and boundary assertions — each test now
   checks that every one of the first N (capped) rows is present by name and
   that the row immediately past the cap boundary is absent from the text, not
   just that the total count matches. Added `usageStatsJSONReport`, a test
   helper that exercises the real `--format json` path
   (`writeUsageEnvelope(..., "json", "usage.stats", ...)` +
   `json.NewEncoder`) instead of trusting the caller-side Go slice length as a
   completeness proxy, and applied it to all five `...CappedOnlyInText` tests
   (including the pre-existing cache-sessions one, since the finding referred
   to "the `OnlyInText` tests" collectively and this test's cap was refactored
   under this task); each asserts the decoded JSON row count and that the
   specific text-omitted tail row is present by name/ID. RED-verified the new
   boundary assertions by temporarily shifting `statsTopN` to return
   `items[1:limit+1]` instead of `items[:limit]`: all four newly-strengthened
   tests failed as expected (the pre-existing cache-sessions test, which has
   no boundary assertion by scope, still passed), confirming the assertions
   catch a wrong-window bug that a count-only check would miss; reverted and
   re-verified GREEN. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
   ./...` (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build
   go vet -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check`
   (clean). Only `cmd/agentdeck/usage_stats_text_test.go` changed in this
   repair; `usage_stats_text.go` is unchanged from the round-1 implementation
   (confirmed by restoring it from the RED-check mutation to the identical
   pre-mutation content).
2. **trend-cap** — Bound the TREND section to a fixed number of buckets (starting
   point: 48). Because the trend is a time series, keep the most recent
   contiguous window and add a `+K earlier buckets in JSON` note rather than
   ranking by value; state in the note that older buckets are omitted. JSON keeps
   all buckets. Add a test for the `--group-by hour --period 30d` case asserting
   a bounded bar-row count and the overflow note.
   **Completion note (2026-07-22):** Added `statsTrendCap = 48` and bounded
   `trendLines` in `cmd/agentdeck/usage_stats_text.go`: when
   `len(r.report.Buckets) > statsTrendCap`, it takes the tail slice
   `r.report.Buckets[omitted:]` (the most recent contiguous window, buckets
   are chronologically ascending) instead of ranking by value, and appends a
   `+%d earlier buckets in JSON` footer line (dim-styled, matching the other
   sections' footer style) whose wording itself states the omitted rows are
   the earlier ones. Label/value-width computation and the bar loop now run
   over the windowed slice only, so an unbounded hour-grouped trend no longer
   inflates line width or count. JSON is untouched since `r.report.Buckets`
   itself is never mutated. Only `cmd/agentdeck/usage_stats_text.go` and its
   test file changed; `internal/usage` untouched. Added
   `TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText`, modeling the
   `--group-by hour --period 30d` shape with 60 hourly buckets spanning
   multiple days (`GroupBy: "hour"`, so `compactBucketLabels`' multi-date
   `"Jan 02 15:04"` label path is exercised, the same path that produced the
   822-line baseline). It asserts: the `+12 earlier buckets in JSON` footer;
   exactly `statsTrendCap` (48) bar rows via `strings.Count(text, "Jun ")`
   (only trend labels carry that dated format); that each of the 12 earliest
   buckets' labels is absent from text (windowed out, not just uncounted);
   that each of the 48 most recent buckets' labels is present, in their
   existing chronological order (no reordering by value); and, via the same
   `usageStatsJSONReport` helper added under `shared-topn`, that the decoded
   `--format json` output still carries all 60 buckets, including the
   text-omitted earliest one. RED-verified by temporarily changing the
   windowing to `r.report.Buckets[:statsTrendCap]` (keep-oldest instead of
   keep-most-recent): the new test failed as expected; reverted and
   re-verified GREEN. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass, includes golden-file, shared-topn, and new trend-cap tests);
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build go vet
   -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check` (clean).
   **Repair (2026-07-22, Round 1):** Strengthened
   `TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText` so every
   retained label's `strings.Index` position must be strictly greater than the
   previous label's position, proving the 48-row window keeps its original
   chronological order rather than merely containing the right labels. RED
   verification temporarily copied the selected window and swapped its first
   two buckets; the targeted test failed at the new assertion with
   `Jun 21 13:00` at position 1204 after position 1386. Restored the original
   production slice unchanged and re-verified GREEN. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1
   -run '^TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText$'
   ./cmd/agentdeck` (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go
   test -mod=vendor -count=1 ./cmd/agentdeck` (pass); `env
   GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
   (pass, all packages); `git diff --check` (clean). Only the test file changed
   in the repair; production code remains identical to the Round 1 reviewed
   implementation.
3. **top-flag** — Add `--top N` to `usage stats` (0 means "all", restoring
   today's behavior) so a caller can widen the text caps without switching to
   JSON. Default keeps the scannable caps from task 1. Document it in
   `docs/specs/cli-manual.md`. This is the explicit escape hatch that makes the
   caps safe to add.
   **Completion note (2026-07-22):** Added `stats.Flags().IntVar(&statsTop,
   "top", 0, ...)` to the `usage stats` command in `cmd/agentdeck/main.go`,
   validated non-negative (`--top -1` is an `inputError`), and threaded it
   into the shared `withUsage` wrapper: only when the invoked command is
   `stats` and `command.Flags().Changed("top")` is true does the wrapper set
   `renderOptions.top = &value` (via `command.Flags().GetInt("top")`) before
   calling `writeUsageEnvelope`; every other `usage` subcommand, and `stats`
   itself when `--top` is omitted, gets a zero-value `usageTextRenderOptions`
   (`top == nil`). Added a `top *int` field to `usageTextRenderOptions` and
   `statsTextRenderer`, plus a `capFor(defaultCap int) int` method: `nil`
   keeps the section's own default, otherwise `*top` wins outright (0 falls
   through `statsTopN`'s existing `limit <= 0` "no cap" branch, so explicit
   `--top 0` naturally restores the full list with no special-casing). Wired
   `capFor` into all five shared-topn cap sites — MODELS, PROVIDERS, UNPRICED
   MODELS, per-model CACHE, and cache sessions (`statsCacheSessionsCap`) —
   since the task's "task 1's caps" phrasing covers all of them, not only the
   four named explicitly. `trendLines`' independent 48-bucket window and
   CLIENTS remain completely untouched by `top`; JSON is unaffected since
   `capFor` only gates what `trendLines`/`rankingLines`/`cacheLines`/
   `unpricedLines` render, never the underlying report slices serialized by
   `--format json`. Documented the flag, its default/`0`/positive-N semantics,
   and TREND's independence from it in `docs/specs/cli-manual.md` (the `usage
   stats` table row and the Balanced-report prose), including a wording fix
   to a stale "完整模型列表"/"完整 MODELS" description left over from tasks 1
   and 2, which never touched that doc. Tests: renderer-level
   `TestUsageStatsTopFlagOmittedKeepsSharedTopNDefaults`,
   `TestUsageStatsTopFlagPositiveOverridesAllListedCaps`,
   `TestUsageStatsTopFlagExplicitZeroRestoresFullText` in
   `usage_stats_text_test.go` cover all five capped sections independently
   (via disjoint name prefixes; per-model CACHE gets its own fixture,
   `topFlagModelCacheFixture`, so a model that is both ranked in MODELS and
   listed in per-model CACHE can't double-count) plus JSON completeness under
   each `--top` value. CLI-level
   `TestUsageStatsTopFlagOverridesTextCapsButNotJSON` in `main_test.go` seeds
   9 real models directly into the store and drives the actual `usage stats`
   command line (proving the `Changed`/`GetInt` wiring itself, not just
   `capFor`), covering default/positive/explicit-zero/negative-rejected/JSON.
   RED-verified twice: reverting `capFor` to always return `defaultCap` failed
   the positive-N and explicit-zero renderer tests as expected; renaming the
   wrapper's `command.Name() == "stats"` check to a non-matching string failed
   the CLI test as expected (proving it tests the wiring, not just the
   renderer). Both reverted and re-verified GREEN. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
   ./...` (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build
   go vet -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check`
   (clean).
   **Repair (2026-07-22):** addressed the Round 1 finding
   (`docs/archive/reviews/usage-stats-readability/top-flag.md`): none of the prior
   tests combined a positive `--top` with more than `statsTrendCap` buckets or
   more than a couple of clients, so `capFor` leaking into TREND or CLIENTS
   could have stayed green. Added
   `TestUsageStatsTopFlagDoesNotAffectTrendOrClients` in
   `usage_stats_text_test.go`, reusing
   `TestUsageStatsTrendBucketsAreCappedToRecentWindowOnlyInText`'s exact
   60-bucket construction and assertions (overflow note, bounded bar-row
   count, earlier buckets absent, retained buckets present in strictly
   increasing chronological order) plus a 5-client fixture, all rendered
   together with `--top 3`. RED-verified both protections separately:
   temporarily routing the CLIENTS loop through
   `statsTopN(r.report.Clients, r.capFor(statsModelsCap))` failed the new
   client-retention assertions as expected; temporarily routing `trendLines`'
   window size through `r.capFor(statsTrendCap)` instead of the bare constant
   failed the new trend assertions as expected. Both mutations were reverted
   (confirmed via `git diff --stat` showing the same line counts as before,
   and a grep for the mutation strings finding nothing) and re-verified
   GREEN. No production defect was exposed, so `usage_stats_text.go` is
   unchanged from before this repair; only the test file changed. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass); `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
   ./...` (pass, all packages); `git diff --check` (clean).
4. **detail-compaction** — Keep each model/provider detail on one line at width
   ≥ 80 by holding the high-value fields inline (tokens · share · cost ·
   sessions) and showing the secondary fields (tools, cache-hit) only when they
   fit, with the full set always present in JSON. This changes the text output
   shape the most, so it carries the golden-file updates and must clear review
   before its Review gate is ticked.
   **Completion note (2026-07-22):** Added a shared
   `statsCompactDetail(base string, width int, secondaries ...string) string`
   helper in `cmd/agentdeck/usage_stats_text.go`: it appends secondary fields
   to the always-present high-value base string, in priority order, keeping
   only as many as still fit on one line at `width`, and stops at the first
   one that doesn't (predictable, no skip-ahead). Applied to both MODELS
   (base: tokens/share/cost/pricing-status/sessions; secondaries in priority
   order: tools, then cache-hit) and PROVIDERS (same base, no tools field to
   begin with; secondary: cache-hit only). `modelPricingStatus` (PRICED/
   PARTIAL/UNPRICED) was kept in the always-present base alongside tokens/
   share/cost/sessions — the plan's four-field list names the representative
   high-value fields, and pricing status travels with cost as one unit; only
   `tools` and `cache-hit` were named as the fields becoming conditional, so
   those are the only two gated by `statsCompactDetail`. `statsWrap` remains
   the final step after compaction as a fallback for widths below 80 (48/72,
   already covered by `TestUsageStatsBalancedTextLayout`), where the
   single-line invariant does not apply and multi-line wrapping is still
   acceptable. Cache-hit text is never reformatted — it's included whole
   (`*model.CacheHitRate + "% hit"`) or omitted whole, so `NN.NN%` never
   rounds. JSON is untouched: compaction only affects what gets appended to
   the local `detail` string inside the text renderer; the underlying
   `usage.StatsDimension`/`Activity` values serialized by `--format json` are
   never touched. `docs/reviews/test-coverage`, other parallel-task changes,
   and `internal/usage` were not touched.
   Golden file: `cmd/agentdeck/testdata/usage-stats-balanced.txt` was checked
   and found byte-identical after the change (`git diff --stat` empty for
   `cmd/agentdeck/testdata/`) — the fixture's field values are all short
   enough that compaction never triggers for it at width 100, so there was
   nothing to update; the task's golden-file-update expectation was verified,
   not skipped.
   Tests: `TestUsageStatsModelDetailCompactsSecondaryFieldsByWidth` (width 80
   vs 100, using the plan's own root-cause example values — `84.9M tokens ·
   1.6% · unavailable · UNPRICED · 69 sessions · 57 tools · 88.14% hit` —
   where the full line is 82 chars: at width 80 only cache-hit is dropped
   (82 > 80, but high-value+tools = 69 <= 80); at width 100 everything is
   kept), `TestUsageStatsModelDetailDropsToolsWhenHighValueAloneFillsTheLine`
   (width 80 with inflated share/cost/sessions values where high-value alone
   is 75 chars and high-value+tools is 87, so tools is dropped too, leaving
   only the always-present fields), and
   `TestUsageStatsProviderDetailCompactsCacheHitByWidth` (the provider
   equivalent of the first test, since providers have no tools field). All
   three assert every high-value field is present, assert the exact secondary
   field composition (via a new `modelDetailBlock`/`modelDetailLine` helper
   that requires the detail be *exactly one line* — not just that some line
   fits within width, which `statsWrap`'s own greedy word-wrap can otherwise
   coincidentally satisfy for a two-line wrap's first fragment), assert
   `88.14% hit` is never rounded when present, assert
   `assertUsageStatsWidth`, and assert JSON completeness via
   `usageStatsJSONReport`. RED-verified by temporarily making
   `statsCompactDetail` append every secondary unconditionally (no width
   check): all three width-80 subtests failed as expected — with the
   pre-strengthening version of `modelDetailLine` (checking only that the
   first line fits within width) this RED check did **not** fail, because
   `statsWrap`'s own wrap coincidentally produced a first line that looked
   compacted; `modelDetailLine`/`modelDetailBlock` were hardened to require
   exactly one line in the block before trusting the RED result. Reverted and
   re-verified GREEN. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass, includes golden-file and all prior tasks' tests);
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build go vet
   -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check` (clean).
   **Repair (2026-07-22):** addressed the Round 1 [P1] finding
   (`docs/archive/reviews/usage-stats-readability/detail-compaction.md`): the one-line
   contract was only ever proven in single-column mode. At terminal width
   >= 104, `render()` switches to two columns and previously gave the ranking
   column (MODELS/PROVIDERS) just `(width-4)*2/5` — 40 columns at width 104,
   55 at 140, 63 at 160 — well under the 80-column single-line contract, even
   though the terminal itself was far past 80; `statsCompactDetail` never
   compacted an already-overwide high-value base, so `statsWrap` split it
   across lines exactly as the finding described. Fixed via the "adjust the
   column/layout strategy" option (not by further compacting the field
   representation, and without dropping tokens/share/cost/sessions): added
   `statsRankingMinWidth = 80` and a `statsTwoColumnWidths(width int)
   (left, right int)` helper that floors the ranking column at that width —
   `rightWidth = max(statsRankingMinWidth, inner*2/5)`, `leftWidth = inner -
   rightWidth` — and wired it into `render()`'s two-column branch in place of
   the old inline `(width-4)*3/5` split. Within the supported width range
   (`statsMaxWidth` = 160), the natural `2/5` share never reaches 80 on its
   own, so the ranking column is pinned at exactly 80 throughout 104-160 and
   the trend column takes the rest (20 columns at 104, growing to 76 at 160);
   `trendLines` already clamps its own label/bar widths internally, so it
   degrades gracefully rather than breaking. This reuses the already-proven
   width-80 single-column compaction behavior for the ranking column's
   content instead of inventing a new one, since the column now genuinely
   receives that width.
   Added `TestUsageStatsModelProviderDetailStaysOneLineInTwoColumnLayout`,
   covering terminal widths 104/140/160 with one model and one provider sized
   so the full detail does not fit an 80-column budget (model: high-value +
   tools fits, + cache-hit does not, so cache-hit alone is dropped; provider:
   no tools field, so its shorter high-value + cache-hit fits outright and is
   kept — proving the two sections compact independently). Row/column-based
   assertions (the `modelDetailLine` helper from the single-column tests)
   cannot isolate a two-column joined line's ranking-side content — a wrapped
   continuation does not start at column 0, and it is not globally blank
   while the trend column still has rows on the same line — so this test
   instead asserts that the exact contiguous substring spanning the
   high-value fields through the last kept secondary field appears unbroken
   in the full output (`statsWrap` only ever breaks at a space between two
   such fields, so an unbroken run proves no wrap occurred), plus
   `assertUsageStatsWidth` and JSON completeness via `usageStatsJSONReport`.
   RED-verified against the pre-fix two-column split: all three widths
   failed as expected (confirmed the continuation fragment "sessions" split
   onto its own line, exactly matching the finding). Reverted the fix,
   confirmed the RED failure, then restored it and re-verified GREEN.
   Golden impact: `cmd/agentdeck/testdata/usage-stats-balanced.txt` (rendered
   at width 100, single-column, unaffected by the two-column split change)
   remained byte-identical (`git diff --stat` empty for
   `cmd/agentdeck/testdata/`) — no golden diff was created, since none of the
   changed output shapes are exercised by that fixture/width. Only
   `cmd/agentdeck/usage_stats_text.go` and its test file changed in this
   repair; `shared-topn`, `trend-cap`, `top-flag`, and `internal/usage` were
   not touched. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass, includes golden-file and all prior tasks' tests);
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build go vet
   -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check` (clean).
   **Repair (2026-07-22, round 2):** addressed the Round 2 [P1] finding
   (`docs/archive/reviews/usage-stats-readability/detail-compaction.md`): flooring the
   ranking column at 80 in the round-1 repair left the trend column only 20
   columns at terminal width 104 (`inner=100`, `right=80`, `left=20`), below
   the ~28 columns `trendLines` needs for a bucket row's label+bar+value
   (7 label + 2 gap + 8 min-bar + 2 gap + 9 value), so `joinStatsColumns`'
   `statsFit` truncated the whole left column — cutting off bucket values, not
   just narrowing the bar — regressing the already-reviewed trend display.
   Fixed by making two-column layout conditional on both columns clearing
   their minimums, not just on the terminal width: added
   `statsTrendMinWidth = 28` (documented as the exact 7+2+8+2+9 derivation)
   and a `statsTwoColumnFits(width int) bool` returning
   `width-4 >= statsRankingMinWidth+statsTrendMinWidth` (i.e. `width >= 112`).
   `render()`'s layout switch now uses `statsTwoColumnFits(r.width)` in place
   of the old bare `r.width >= 104` check — width 104-111 now stacks (falls
   through to the existing single-column branch, which already gives both
   trendLines and rankingLines the full terminal width), width >= 112 stays
   two-column with the round-1 80-column ranking floor unchanged. No field
   representation was touched and no field was dropped to make width fit;
   this is purely the "adjust the column/layout strategy" option, applied a
   second time to the layout's activation condition rather than its split
   ratio.
   Updated `TestUsageStatsModelProviderDetailStaysOneLineInTwoColumnLayout`
   (round-1's new test) to also assert the fixture's first trend bucket
   ("Jul 14" label, "$13.4M KNOWN" value under metric=cost) is visible at
   every tested width, and to reflect that width 104 now stacks: stacked mode
   gives the ranking column the full 104-column width, so the model's full
   89-column detail (cache-hit included) now fits and is kept there — a
   correct, expected behavior change from round 1's forced-80 split at that
   same width, not a new defect (140/160 remain two-column with the ranking
   floor, so the model still drops cache-hit there, unchanged from round 1).
   RED-verified by temporarily reducing `statsTwoColumnFits` to the old bare
   `width >= 104` check: the width-104 subtest failed as expected, with the
   rendered output showing literal `…` truncation cutting off trend bar rows
   (visible proof of the exact defect described), while 140/160 still passed
   (already unaffected, confirming the fix's blast radius matches the
   finding). Reverted to the two-minimum check and re-verified GREEN.
   `TestUsageStatsBalancedTextLayout` (48/72/100/140) and
   `TestUsageStatsWideLayoutAndColorAreDeterministic` (140) were re-run and
   remain unaffected, since neither exercises the 104-111 stacking band.
   Golden impact: `cmd/agentdeck/testdata/usage-stats-balanced.txt` (width
   100, single-column, never reaches the two-column branch either way)
   remained byte-identical (`git diff --stat` empty for
   `cmd/agentdeck/testdata/`) — no golden diff created. Only
   `cmd/agentdeck/usage_stats_text.go` and its test file changed in this
   repair; `shared-topn`, `trend-cap`, `top-flag`, and `internal/usage` were
   not touched. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass, includes golden-file and all prior tasks' tests);
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build go vet
   -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check` (clean).
   **Repair (2026-07-22, round 3):** addressed the Round 3 [P1] finding
   (`docs/archive/reviews/usage-stats-readability/detail-compaction.md`): round 2's
   `statsTrendMinWidth = 28` assumed trendLines' 7/9-column *defaults*, but
   compact formats the report can actually produce are wider — a
   known-but-partial cost value like `$13.4M KNOWN` is 12 columns, and so is
   the multi-date hour label `compactBucketLabels` switches to
   (`Jan 02 15:04`) whenever buckets span more than one calendar date under
   hour grouping. At width 112 (round 2's exact activation point), a report
   using both wider formats still needed 36 trend columns
   (12 label + 2 + 8 min-bar + 2 + 12 value), not 28, so two-column mode still
   truncated. Fixed via the review's second option — deriving the minimum
   from the report's actual rendered content rather than a static per-format
   guess, since a static number had already been reopened twice for
   undercounting a format nobody had enumerated yet. Extracted
   `(statsTextRenderer).statsTrendLabelValueWidths()`, which scans this
   report's actual (cap-windowed) bucket labels and values the same way
   `trendLines` itself does, starting from the same
   `statsTrendDefaultLabelWidth`/`statsTrendDefaultValueWidth` (7/9)
   `trendLines` always used; `trendLines` was refactored to call this helper
   instead of duplicating the scan, so the two can no longer disagree.
   `statsTwoColumnFits` became a `statsTextRenderer` method — necessarily,
   since the fit decision now depends on report content, not just terminal
   width — computing `trendMinWidth = labelWidth + 2 + statsTrendMinBarWidth
   + 2 + valueWidth` from that helper and requiring
   `width-4 >= statsRankingMinWidth + trendMinWidth`. No field representation
   was touched and no field was dropped or truncated to make width fit.
   Added `TestUsageStatsTwoColumnThresholdCoversWidestSupportedTrendFormats`
   with a report combining hour-grouped buckets spanning two calendar dates
   (forcing the 12-column multi-date label format) and metric=cost with a
   12-column known-partial value, plus the same wide-model/wide-provider
   detail fixture used by the round-1/round-2 tests. Covers width 112 (round
   2's old activation point — must now stack), 119 (one column short of this
   report's real 36-column trend need — must still stack), and 120 (the
   first width that actually fits ranking(80) + trend(36) + gap(4) = 120 —
   must be two-column). Asserts, at every width: the trend label and value
   both appear verbatim (not truncated), model detail stays a contiguous
   single line, `assertUsageStatsWidth`, and JSON completeness. Layout mode
   (stacked vs two-column) is detected via
   `isTwoColumnStatsLayout` — whether the TREND and MODELS section titles
   were joined onto the same output line — reusing the same signal
   `TestUsageStatsWideLayoutAndColorAreDeterministic` already relies on.
   RED-verified by temporarily reverting `statsTwoColumnFits` to the old
   static `width-4 >= statsRankingMinWidth+28` check: widths 112 and 119
   failed as expected, with the rendered output showing literal `…`
   truncation on `"Jul 14 09:…"` (the exact defect described); width 120
   still passed under the old check too, since 120 already clears both the
   old and new thresholds for this fixture — consistent, not a gap in the
   RED check. Reverted to the dynamic implementation and re-verified GREEN.
   Also re-ran the round-1/round-2 test
   (`TestUsageStatsModelProviderDetailStaysOneLineInTwoColumnLayout`, widths
   104/140/160, day-grouped labels) to confirm the dynamic recalculation
   doesn't regress it. That fixture's own dynamic trend minimum turned out
   wider than a first hand estimate assumed: its 7 buckets' cost values
   include `$379243.00 KNOWN` (16 columns — under-1M values fall through
   `compactDecimal` to two-decimal formatting with no K/M/B suffix, which is
   wider than the M-suffixed values), not just the 12-column `$13.4M KNOWN`
   used elsewhere in this file, giving a real minimum of
   7 (label) + 2 + 8 + 2 + 16 (value) = 35 columns. A temporary scratch test
   swept widths 108-122 against this exact fixture to find the actual
   activation point empirically rather than trust a second hand calculation:
   two-column starts at width 119 (matching 35), not the 115/116 the first
   estimate produced. This is still well below the tested 140/160, so those
   subtests are unaffected, and the 104 subtest already expected stacking
   regardless of the exact boundary — the fixture's own tests all still pass
   with the correct dynamic code; only this note's arithmetic needed
   correcting. (This mismatch is itself further evidence for the
   review-directed dynamic-measurement fix over a static "conservative
   minimum across all supported formats" one: a static minimum would have
   needed this under-1M formatting branch enumerated too, and neither round 2
   nor this note's first draft did.) The scratch test was removed after
   verification; it is not part of the delivered diff.
   Golden impact: `cmd/agentdeck/testdata/usage-stats-balanced.txt` (width
   100, single-column, never reaches the two-column branch) remained
   byte-identical (`git diff --stat` empty for `cmd/agentdeck/testdata/`) —
   no golden diff created. Only `cmd/agentdeck/usage_stats_text.go` and its
   test file changed in this repair; `shared-topn`, `trend-cap`, `top-flag`,
   and `internal/usage` were not touched. Evidence:
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck`
   (pass, includes golden-file and all prior tasks' tests);
   `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`
   (pass, all packages); `env GOCACHE=/private/tmp/agent-deck-go-build go vet
   -mod=vendor ./cmd/agentdeck/...` (clean); `git diff --check` (clean).
5. **remeasure** — Re-render the same two fixtures (`--period all` and
   `--period 30d --group-by hour`) and record the new line counts in this
   document.
   **Completion note (2026-07-22):** First pass measured a single current
   binary against the live database at whatever size it had grown to,
   compared against the historical 140/822 baseline; reopened in review for
   not being a same-content paired comparison, so the causal attributions to
   tasks 1-4 were not established
   (`docs/archive/reviews/usage-stats-readability/remeasure.md`, Round 1).
   **Repair (2026-07-22):** replaced it with a controlled paired A/B in `##
   Remeasurement (2026-07-22, controlled A/B)`: a pre-usage-stats-readability
   baseline binary built from the exported `HEAD` (`5bf3356`, confirmed
   pre-change — none of this plan's tasks are committed yet) and the current
   binary built from the actual working tree, both run against fresh copies
   of one frozen database snapshot (identity recorded only as a SHA-256 file
   digest, reverified unchanged after all four runs; discovered mid-repair
   that `usage stats --no-scan` still mutates the file it opens, so each run
   needed its own fresh copy from the frozen reference rather than sharing
   one mutable copy across runs). Direct result: `--period all` 139 to 120
   (**-13.7%**), `--period 30d --group-by hour` 832 to 142 (**-82.9%**) — both
   recomputed from this paired data, not the original 140/822, which are
   kept only as historical reference. Content was inspected directly (not
   just counted): the baseline output lists all 15 models with no cap or
   footer, confirming the exported binary genuinely predates `shared-topn`.
   No production code, test, JSON output, or `internal/usage` computation was
   changed; only this plan document changed. Evidence: baseline/current
   binary SHA-256, diff digests, snapshot SHA-256 (reverified matching before
   every one of the four runs and unchanged at the end), `go version`, exact
   build and measurement commands, and all four raw line counts are recorded
   above; real `~/.agentdeck/agentdeck.sqlite3` mtime confirmed unchanged
   throughout (never opened directly, only copied from); `rtk git diff
   --check` passes on the documentation-only change.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | shared-topn | ✅ | ✅ |
| 2 | trend-cap | ✅ | ✅ |
| 3 | top-flag | ✅ | ✅ |
| 4 | detail-compaction | ✅ | ✅ |
| 5 | remeasure | ✅ | ✅ |

Done: 5/5. The implementer ticks **Dev** once a task is built and its targeted
verification passes; an independent reviewer ticks **Review** once findings are
closed, recording the round in `docs/archive/reviews/usage-stats-readability/`. A task
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
> `docs/archive/reviews/usage-stats-readability/<task-anchor>.md`。

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
