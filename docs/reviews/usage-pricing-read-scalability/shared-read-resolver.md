---
status: active
plan: usage-pricing-read-scalability
task: shared-read-resolver
---

# Review log — usage-pricing-read-scalability / shared-read-resolver

## Round 1 — 2026-08-02
- Reviewed state: worktree on `308feb0`, `internal/usage/usage.go` blob
  `594dd8e17abc7b8018c6305f1369faf7f2cc7a29` (uncommitted).
- Reviewer: Claude Opus 5 (independent round, no product code changed).
- Scope: the whole `internal/usage/usage.go` diff — `statsPriceRow` →
  `readPriceRow`, `statsPriceResolver` → `readPriceResolver`,
  `loadStatsPriceResolver` → `loadReadPriceResolver`, `statsPriceModelKey` →
  `readPriceModelKey`, the new `timeline store.ProviderTimeline` field, the
  `priceForEvent` signature change, and the `Stats` call site. Checked against
  the plan's Task 1 statement, its Non-Goals, and the patch-level claim under
  the release versioning contract.
- Findings:
  - No P1/P2. `priceAt` and `priceForEvent` bodies are unchanged apart from
    reading the timeline off the receiver, so the historical/current component
    merge and the exact/estimated/historical attribution ladder are preserved.
    Every renamed symbol is unexported, no exported API, output shape, schema,
    stored-event interpretation, or pricing rule changes — the Non-Goals and the
    PATCH claim hold.
  - [nit] Error precedence in `Stats` flipped: the provider-timeline load now
    runs before the price-row query, so if both fail the timeline error surfaces
    instead of the pricing error. No contract or test pins this; noted only.
  - [nit] `readPriceResolver.priceForEvent` now coexists with the legacy
    `(*Service).priceForEvent` (`usage.go:3053`). Pre-existing since the stats
    resolver landed, and Tasks 2/3 are specified to retire the legacy path;
    no action in this task.
  - [nit] The resolver is still exercised only through `Stats`; there is no
    test asserting it is reusable/bounded as a shared component. That is Task 4
    (`scalability-acceptance`) by the plan's own decomposition, not a gap here.
  - `PriceDiagnostics` and `Sessions` intentionally untouched, matching the plan
    Task 2/3 split. Verified no `statsPrice*` identifier remains anywhere.
- Evidence:
  - `grep -rn 'statsPriceResolver\|statsPriceRow\|statsPriceModelKey\|loadStatsPriceResolver' --include=*.go .` → no matches.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage/...` → ok (7.2s).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` → all packages ok (L2 route).
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./internal/usage` → clean.
- Verdict: PASS
