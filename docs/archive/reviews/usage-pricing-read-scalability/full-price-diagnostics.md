---
status: historical
plan: usage-pricing-read-scalability
task: full-price-diagnostics
retired: 2026-08-03
---

# Review log — usage-pricing-read-scalability / full-price-diagnostics

## Round 1 — 2026-08-02
- Reviewed state: worktree on `24b9e56`; `internal/usage/usage.go` blob
  `367fe5109e0f43f5dd56c7e8b7a66e4618844c5f`, `internal/usage/usage_test.go`
  blob `e9d5c645e5eb55b7a9935eb351ee2bf2756985a3` (uncommitted).
- Reviewer: Claude Opus 5 (independent round, no product code changed).
- Scope: the `PriceDiagnostics` switch from `(*Service).priceForEvent` to
  `readPriceResolver.priceForEvent`, and the added
  `TestPriceDiagnosticsUsesCurrentPricesOnlyForMissingHistoricalComponents`.
  Because the task promises byte-compatible full-`doctor` counts, the review
  compared the legacy and resolver paths clause by clause rather than reading
  the diff alone: `mergedPriceAt` vs `priceAt`, `usageModelMatches` vs
  `readPriceModelKey`, the historical/current merge, and the `s.events()` column
  set that now feeds the resolver.
- Findings:
  - No defect in the delivered behavior. The two returned counts are unchanged:
    - **Model matching is equivalent.** `usageModelMatches` reduces to exact
      equality except when both catalog and event model carry the `claude-`
      prefix, where it compares dot-to-dash normalized names — exactly what
      `readPriceModelKey` bakes into the map key on both the catalog and lookup
      side. No catalog row that matched before stops matching.
    - **The merge is equivalent.** Both take historical-at-event-time, fall back
      to current when historical is absent, and otherwise let current fill only
      components historical lacks. Neither propagates `EffectiveFrom` into the
      merged value.
    - **Provider scoping is equivalent** — the legacy `Provider != expected`
      filter is carried by the resolver's `provider\x00model` key.
    - `Calculate` derives `Unpriced` and therefore `CatalogBaseCost` purely from
      component coverage; the multiplier only reaches `ProviderCost`. So the
      distinct-unpriced-model count cannot move even where attribution differs.
    - The aggregate provenance count above the loop is untouched, and no
      `doctor` check name, code, or recovery command changed.
    - Resolving `s.now()` once into the resolver instead of per event also makes
      the "current" side of the merge a single consistent instant.
  - [P2 — not a Task 2 defect; blocks Task 3] The resolver drops the legacy
    session-start fallback. `(*Service).priceForEvent` used `e.EventAt` when
    `usage_sessions.first_at` was absent and could still resolve an
    `estimated` provider snapshot; `readPriceResolver.priceForEvent` skips the
    lookup entirely and keeps `quality: historical`, `multiplier: "1"`. Harmless
    here — `PriceDiagnostics` reads neither field — but `usage sessions` reports
    provider cost and completeness, so moving it onto the resolver in
    `usage-sessions-batch` under this behavior would change user-visible cost
    numbers. Under the release versioning contract that is a MINOR trigger and
    would break the plan's PATCH classification. Task 3 must either restore the
    fallback in the resolver or record why the change is correct and how
    `usage sessions` output stays byte-identical.
  - [nit] `PriceDiagnostics` reads events through `s.events()`, whose SELECT
    omits `runProvider` and `runStart`. The resolver therefore always computes
    `provider: "unknown"` and `viaWrapper: false` on this path. Unused by this
    caller, but it means the resolver's attribution fields are only fully
    populated via `eventsRange`. Task 3 should confirm which query it feeds the
    resolver from.
  - [nit] No automated evidence yet for the task's actual claim — that the
    per-event database calls are gone. The new test proves the pricing values,
    not the query count. `scalability-acceptance` owns that assertion; until it
    lands, the elimination is verified by reading the code only.
  - The new test is a real regression gate: with the current-fills-missing-
    components branch removed, the fixture's `output` component would be
    unpriced and the assertion `unpriced != 0` would fail. It also exercises the
    no-`usage_sessions`-row path, which is where the P2 difference lives.
- Evidence:
  - `go test -mod=vendor ./internal/usage ./internal/doctor` → ok.
  - `go test -mod=vendor ./...` → all packages ok (L2 route).
  - `go test -mod=vendor -count=1 ./internal/doctor ./cmd/agentdeck` → ok
    (uncached, to bind the full-`doctor` surface to this content state).
  - `go test -mod=vendor -count=1 -run TestPriceDiagnostics -v ./internal/usage`
    → both diagnostics tests PASS.
  - `go vet -mod=vendor ./internal/usage ./internal/doctor` → clean.
  - All commands run with `GOCACHE=/private/tmp/agent-deck-go-build`.
- Verdict: PASS
