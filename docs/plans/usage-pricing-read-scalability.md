---
status: active
created: 2026-08-02
---

# Usage Pricing Read Scalability

Target release: `v0.2.2`. This plan records the deeper pricing-read work found
while validating `v0.2.1-rc.2`. Nothing in this plan is implemented by the
quick-doctor fix.

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

### 2. `full-price-diagnostics`

Rewrite `PriceDiagnostics` to load pricing inputs once and calculate unpriced
model coverage without per-event database calls. Full `doctor` check names,
codes, counts, and recovery commands remain byte-compatible.

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

Verification: L2 for shared usage pricing and CLI report behavior. Each task
needs targeted `internal/usage` or `cmd/agentdeck` tests; final acceptance needs
`go test -mod=vendor ./...`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `shared-read-resolver` | [ ] | [ ] |
| 2. `full-price-diagnostics` | [ ] | [ ] |
| 3. `usage-sessions-batch` | [ ] | [ ] |
| 4. `scalability-acceptance` | [ ] | [ ] |

## Starting Task

> 进入开发：`usage-pricing-read-scalability` / `<task-anchor>`

Read `AGENTS.md`, this plan's evidence and selected task, the named source
symbols, and the L2 verification route. Implement only that task, tick `Dev`
after its targeted evidence passes, and leave `Review` for an independent round
recorded under `docs/reviews/usage-pricing-read-scalability/`.
