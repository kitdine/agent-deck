---
status: historical
plan: provider-wrapper-routing
task: usage-route-metadata
---

# Review log — provider-wrapper-routing / usage-route-metadata

## Round 1 — 2026-07-27 (summary, delivered in session)

- Reviewed state: base `23453be`, uncommitted working tree.
- Verdict: **PASS with improvements**, no blocking finding. The route reaches
  the report as one additive count and nothing else: the grouping key
  (`client + runtimeProvider`), the `--provider` filter, the provider sort
  (`KnownMetricValue`/client/name), and every cost path were checked
  individually and none consult it. Three revert checks confirmed the tests
  were RED without their fix (the name guard, the aggregation increment, the
  text annotation).
- Findings:
  1. **An exact event's route was read from the session start while its
     provider came from the run**, so a session spanning a route change on one
     provider **over-reported** — reproduced with a probe: session wrapped +
     run direct yielded `wrapper_events=1`. That contradicted the "under-report
     only" guarantee stated in the code comment, `cli-manual.md`, and the plan.
  2. **The route cost a second `SnapshotAt` per event** — a linear scan over
     operations and selections — doubling the timeline work for estimated
     events and adding it to exact events that previously did none, in the
     block `cli-design.md:955` calls a single in-memory aggregation.
- Also recorded: the plan's timezone diagnosis of the two long-standing
  `./cmd/agentdeck` failures is evidence-backed and not over-extrapolated, and
  is in fact stronger than stated — passing under `TZ=UTC` includes
  `assertCommandContracts`.
- Checked and dismissed: `unknown`/missing-snapshot/parse-failure handling;
  `wrapperEvents` living on the shared `statsAccumulator` (comment plus
  `omitempty`, and only the provider accumulator increments it); text width
  (`statsCompactDetail` appends last and drops on overflow, so the annotation
  can never displace an existing secondary).

## Round 2 — 2026-07-27 (re-review)

- Reviewed state: base `23453be`, uncommitted working tree, five modified files
  plus two new test files. The repository was not modified by this pass.

### Finding-by-finding disposition

- **[1] Route precision — FIXED via the option (a) route.** `storedEvent`
  gained `runStart`, `eventsRange` selects `r.started_at`, and an exact
  run-bound event now reads its route at the instant the run pinned its
  provider. Both directions assert correctly
  (`TestStatsReadsAnExactEventsRouteFromItsRunStartNotItsSessionStart`: session
  direct + run wrapped → 1, session wrapped + run direct → 0) and both subcases
  were independently reproduced RED by pointing the lookup back at
  `event.sessionStart`. The provider-name guard survives and is load-bearing:
  removing `runtimeProviderName(snapshot.Name) == provider` fails
  `TestStatsLeavesTheRouteUnreportedWhenTheRunAndSnapshotDisagree`.
- **[1a] The added column did not disturb the other query.** `events()` still
  selects 19 columns and scans 19 targets ending at `&e.sessionStart`; it never
  selected `r.provider` and does not select `r.started_at`, so its
  `runProvider`/`runStart` stay invalid, which is right for a path that reports
  no routes. `eventsRange()` is 21 and 21, ending
  `runProvider, runStart, sessionStart`, matching its `SELECT` order. Both are
  runtime-verified: the `./internal/usage/` suite and the `TZ=UTC` end-to-end
  flow exercise both queries, and a misaligned `Scan` would fail at runtime,
  not compile time.
- **[2] One lookup per event — FIXED.** `priceForEvent` returns a single
  `eventAttribution`; the exact branch spends its one `SnapshotAt` at the run
  start, the estimated branch reuses the snapshot that already chose the
  provider (`attribution.viaWrapper = provider != "unknown" && snapshot.ViaWrapper`,
  no second call), and the aggregation site reads `attribution.viaWrapper`
  rather than resolving anything. The only other `SnapshotAt` in the file is
  `runtimeProviderAt`, used by the pre-existing tool-call activity loop, which
  this task did not touch.
- **[2a] The `eventAttribution` refactor is semantics-preserving.** The
  defaults are the same three values in the same order
  (`historical`/`1`/`unknown`); the exact and estimated branches are unchanged
  line for line apart from writing into fields; every error path still returns
  the error with an unused zero value. The `if`-chain to `switch` conversion is
  equivalent: the two guard cases (`!historical && !current`, then
  `!historical`) come first in the same order, so the default branch is reached
  exactly when `historicalFound` is true, which is when the original merge loop
  ran. `historicalFound && !currentFound` still merges from an empty
  `current.Prices`, a no-op in both shapes. The package's price, quality, and
  multiplier tests pass unchanged.

### Requested cross-checks

- **The golden gained exactly one key.** `git diff` on the fixture is a single
  hunk adding `"wrapper_events": "number"` to the `usage.stats` providers
  element (2 insertions, 1 deletion — the second line is the comma the previous
  last key needed). The file still parses and still holds 51 contracts, so the
  Python round-trip that wrote it swallowed nothing else.
- **`TZ=UTC` passing is reproducible.** Re-ran with `-count=1` to defeat the
  test cache: `./cmd/agentdeck` passes as a whole, contract comparison
  included. Under the machine's own zone the package still fails exactly the
  two diagnosed timezone-dependent tests.

### Nits (no fix required)

- The "unknown provider carries no route" rule now lives in two places: inside
  `runtimeRouteWasWrapped` for the exact branch and inline as
  `attribution.provider != "unknown"` for the estimated one. Both are correct
  and each is one expression, but a future change to that rule has to find both.
- "At most one `SnapshotAt` per event" is true of token events. The tool-call
  activity loop still resolves its own snapshot per row under `--provider`;
  that is pre-existing behavior and outside this task, but the plan's wording
  could say "per token event" to be exact.

### Verdict

**PASS.** Both Round 1 findings are closed at the root: the route and the
provider now come from the same instant, and the extra timeline scan is gone
rather than merely amortized. Every new assertion was independently reproduced
in its RED state, the added column left the sibling query's scan alignment
intact, the refactor preserves attribution semantics, and the one golden change
is exactly the field this task adds — which is itself end-to-end evidence that
the fix reaches real events. `Review` ticked for `usage-route-metadata`; no
further round follows.
