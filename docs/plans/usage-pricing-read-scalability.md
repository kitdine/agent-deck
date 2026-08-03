---
status: active
created: 2026-08-02
---

# Usage Pricing Read Scalability

Target release: `v0.2.2`. This plan records the deeper pricing-read work found
while validating `v0.2.1-rc.2`. Nothing in this plan is implemented by the
quick-doctor fix.

Confirmed patch-level on 2026-08-02 under
[the release versioning contract](release-versioning-contract.md): the Non-Goals
below already forbid every MINOR trigger, so this plan is the largest `v0.2.2`
work and still leaves the release safe to downgrade from.

It is also the first thing to land in the release. Three `v0.3.0` tasks —
`cache-creation-ttl-default`, `codex-auto-review-classification`'s
`classification-behavior`, and `hook-boundary-storage` — all change how a stored
event resolves to a provider or a price, and each is specified to build on the
resolver this plan delivers rather than on the per-event path it replaces.

## Goal

- Make full doctor price diagnostics scale with events and catalog rows without
  per-event database queries or repeated full price-table scans.
- Make `usage sessions` reuse one bounded pricing/attribution snapshot while
  preserving its current ordering and output contracts.
- Reuse the batch resolver and provider timeline already proven by `usage stats`
  instead of maintaining two pricing algorithms.

## Non-Goals

- No additional change to quick `doctor`; that is owned by
  `doctor-quick-diagnostics.md`.
- No `usage sessions` pagination, new flags, progress output, or deadline.
- No schema, stored-event interpretation, pricing rule, or output-shape change.
- No work in `v0.2.1` after the quick boundary is fixed.

## Evidence Baseline

- `usage.PriceDiagnostics` loads all events and calls the legacy
  `priceForEvent` for each one.
- Legacy `priceForEvent` performs attribution queries and calls
  `mergedPriceAt` twice; each call scans, parses, and sorts all model-price rows.
- `usage.Service.Sessions` first opens every session row, then queries each
  session's events and runs the same per-event summary path. The CLI exposes no
  limit or page flag.
- Existing coverage uses one session and one or two events; it proves values but
  not query count or large-store behavior.
- `usage stats` already loads model prices once into `statsPriceResolver` and
  loads the provider timeline once, demonstrating the intended bounded pattern.

## Tasks

### 1. `shared-read-resolver`

Extract or generalize the batch price resolver and provider-timeline lookup so
stats, deep diagnostics, and session summaries share one price interpretation.
Preserve historical/current component merge behavior and attribution quality.

**Dev evidence, 2026-08-02.** Renamed the stats-only resolver to the internal
`readPriceResolver`, made it load and retain the provider timeline with the
price rows, and kept `Stats` on that one snapshot for event attribution and
provider reporting. `PriceDiagnostics` and `Sessions` intentionally remain
unchanged for Tasks 2 and 3. Verified
`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage`
and `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`.

### 2. `full-price-diagnostics`

Rewrite `PriceDiagnostics` to load pricing inputs once and calculate unpriced
model coverage without per-event database calls. Full `doctor` check names,
codes, counts, and recovery commands remain byte-compatible.

**Dev evidence, 2026-08-02.** `PriceDiagnostics` now loads one
`readPriceResolver` after loading its events and resolves every event through
that snapshot, eliminating its legacy per-event attribution and price reads.
The aggregate provenance count, distinct unpriced-model count, `Calculate`
semantics, and full-doctor check contract remain unchanged. Added a regression
test for historical/current component merging, then verified
`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage ./internal/doctor`
and `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`.

### 3. `usage-sessions-batch`

Load session metadata and events in bounded queries, group events by session in
memory, and summarize through the shared resolver. Preserve ordering, token
totals, known subtotals, nil completeness, local-time text rendering, and JSON.

**Dev evidence, 2026-08-02.** Closed the prior review's P2 by making
`readPriceResolver` fall back to an event's `EventAt` when no session-start is
available, matching legacy estimated provider attribution. `Sessions` now loads
ordered metadata once, all events once, and one shared resolver; it groups events
in memory and uses the shared summary path without changing its ordering or
output fields. The event query now also selects exact-run provider and start
metadata, so every resolver caller has complete attribution inputs. Added
regression coverage for the fallback and the visible session-start provider cost
and attribution warning. Constant query-count assertions remain Task 4 scope.
Verified `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage`,
`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck ./internal/doctor`,
and `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`.

### 4. `scalability-acceptance`

Add deterministic large-fixture and query-count coverage. Prefer constant-query
assertions over wall-clock thresholds; retain one real-data measurement against
the validation database as acceptance evidence, not a portable unit-test gate.
Update living documentation only if implementation changes an observable
contract.

**Dev evidence, 2026-08-02.** `TestPriceReadPathsKeepQueryCountConstantForLargeFixture`
seeds 1,003 events and proves both `PriceDiagnostics` and `Sessions` use exactly
five SELECTs, independent of event count: one caller-specific read plus the
resolver's operation, selection, and price snapshots. No wall-clock assertion is
used. The real 93,982,720-byte state database remained SHA-256
`2763ef03f3fff49f79955fe78eb19e31c96323fcb2b88a5fc15ffc024dd8624b` before and
after acceptance. Since the CLI's `usage stats --no-scan` still opens a writable
store, current and `HEAD` binaries compared byte-identical isolated copies over
the fixed `2000-01-01` through `2026-08-02` range; after removing only envelope
`generated_at`, their JSON SHA-256 was
`20199dcad3ac1fd89d78f748af9588b70d61f2c88d306803669b0ba817b452c2`.
Direct read-only real-state doctor checks both exited 0: quick reported 12 checks
and full 18, including full-only price provenance and unpriced-model checks.
This clears the v0.2.1 real-state acceptance debt in `docs/README.md`.

Verification: L2 for shared usage pricing and CLI report behavior. Each task
needs targeted `internal/usage` or `cmd/agentdeck` tests; final acceptance needs
`go test -mod=vendor ./...`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `shared-read-resolver` | [x] | [x] |
| 2. `full-price-diagnostics` | [x] | [x] |
| 3. `usage-sessions-batch` | [x] | [x] |
| 4. `scalability-acceptance` | [x] | [x] |

## Starting Task

> 进入开发：`usage-pricing-read-scalability` / `<task-anchor>`

Read `AGENTS.md`, this plan's evidence and selected task, the named source
symbols, and the L2 verification route. Implement only that task, tick `Dev`
after its targeted evidence passes, and leave `Review` for an independent round
recorded under `docs/reviews/usage-pricing-read-scalability/`.
