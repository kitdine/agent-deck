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

### 4. `scalability-acceptance`

Add deterministic large-fixture and query-count coverage. Prefer constant-query
assertions over wall-clock thresholds; retain one real-data measurement against
the validation database as acceptance evidence, not a portable unit-test gate.
Update living documentation only if implementation changes an observable
contract.

This task also clears one verification debt carried over from `v0.2.1`. Real-state
acceptance of the quick `doctor` boundary fix in `e722be8` is recorded in
`docs/README.md` as unverified, because the managed approval reviewer blocked the
command with its own request-format error. The same real 93,982,720-byte usage
database and the same `doctor` surface are already required here, so the run
covers both: record quick-mode and full-mode `doctor` behavior against that
database as acceptance evidence, and update the `docs/README.md` statement when
it passes. If it cannot be run, the debt stays recorded as unverified rather than
being quietly dropped.

Verification: L2 for shared usage pricing and CLI report behavior. Each task
needs targeted `internal/usage` or `cmd/agentdeck` tests; final acceptance needs
`go test -mod=vendor ./...`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `shared-read-resolver` | [x] | [x] |
| 2. `full-price-diagnostics` | [x] | [x] |
| 3. `usage-sessions-batch` | [ ] | [ ] |
| 4. `scalability-acceptance` | [ ] | [ ] |

## Starting Task

> 进入开发：`usage-pricing-read-scalability` / `<task-anchor>`

Read `AGENTS.md`, this plan's evidence and selected task, the named source
symbols, and the L2 verification route. Implement only that task, tick `Dev`
after its targeted evidence passes, and leave `Review` for an independent round
recorded under `docs/reviews/usage-pricing-read-scalability/`.
