---
status: active
topic: usage-attribution-precision
subject: attribution-observability
---

# Review log — usage-attribution-precision / attribution-observability

## Round 1 — 2026-08-26

- Reviewed state: HEAD `7ce1c4f46e9555eb44ddbedc08d0a5d9b5a205c4` plus the scoped
  manifest `sha256:95e1ae124a3b7e21e0ed1ff58d5fc97e0132f8009d2aff9bfb791836a7c730a1`,
  recomputed independently from the working tree using the recipe and file order
  the implementation's content state records. It reproduced exactly, so the state
  judged below is the state the CEv1 gate binds.
- Reviewer: claude-code, independently reviewing the task 3 implementation
  authored by `codex`; this workflow turn kept production code, tests, and
  configuration read-only.
- Method: Formal REVIEW under `development-workflow`, no external skill. Each
  acceptance clause of `tasks.md` task 3 and each sentence of `architecture.md`'s
  Contract impact section was checked against the source implementing it, then
  every field whose meaning this task changes was traced to each of its
  consumers, including the Swift surfaces that read the desktop payload.
  Verification commands were re-run independently.
- Scope: `internal/usage/usage.go` (reason vocabulary, `resolveSessionAttribution`,
  `calculateAttributedEvent`, `summarizeEvents`, `statsAccumulator`),
  `internal/usage/presentation.go`, `internal/usage/session_usage.go`,
  `internal/store/providers.go`, `internal/desktop/desktop.go`,
  `cmd/agentdeck/usage_family_text.go`, the changed tests and goldens, the three
  regenerated canonical fixtures, `docs/specs/cli-{design,manual}.md`, the
  v0.5.0 release-note input, and the macOS consumers of every field whose
  semantics moved.
- Findings:
  - **[P1] O1-F1 — `session show` now reports a literal `0.000000000` as the
    cost of every fully priced unattributed invocation.** The real-spend rule is
    implemented by mutating each event's own `Result`:
    `calculateAttributedEvent` (`internal/usage/usage.go`) sets
    `result.ProviderCost = nil` and `result.KnownProviderCost = "0.000000000"`
    whenever the event is not spend-eligible or not fully priced. That is sound
    as a *summand* — a non-eligible event must contribute nothing to
    `known_provider_cost` — but the same mutated `Result` is what
    `SessionInvocations` (`internal/usage/session_usage.go`) hands back per
    invocation, and `SessionInvocation.KnownProviderCost` is a per-event field,
    not a sum. `sessionViewerInvocationCost`
    (`cmd/agentdeck/session_viewer_data.go`) then renders
    `invocation.KnownProviderCost + " (partial)"`, so the row's
    `IN … · OUT … · COST …` reads `COST 0.000000000 (partial)`.
    Zero is not that invocation's cost, and the row disproves itself: the task's
    own updated test (`cmd/agentdeck/session_viewer_data_test.go`) asserts that
    the same invocation still carries a non-empty `CATALOG BASE COST` detail
    field. So the viewer shows a known catalog amount beside a provider cost of
    zero for an event whose only defect is that its provider is unknown. An
    unattributed invocation has *no* real-spend figure; the honest rendering is
    the `unpriced`/unavailable branch that already exists two lines below, not a
    zero dressed as a partial amount. This is the failure mode the topic exists
    to remove — a cost claim that is not real — reintroduced on the surface where
    a user inspects individual invocations.
    The same mutation also silently drops a genuine partial for a *spend-eligible
    but partially priced* invocation, whose known provider subtotal was
    previously a real number. `docs/specs/cli-manual.md` used to guarantee
    exactly that ("不能把已计价工作隐藏掉"); this task replaced the sentence with
    one that restates the rule for the summary only, leaving the per-invocation
    contract unstated rather than reconciled.
    The repair is to keep the exclusion where it belongs — in the accumulators
    that build the totals — rather than in the per-event `Result` every
    per-event consumer also reads.
  - **[P2] O1-F2 — the desktop `pricing_complete` flag was widened to mean
    "provider cost is complete" while the field beside it and the copy that
    explains it still mean "unpriced components".** `presentationTotals`
    (`internal/usage/presentation.go`) now returns
    `PricingComplete: value.providerComplete` next to an unchanged
    `UnpricedComponents: len(value.missing)`. The flip itself is necessary and
    should stay: `WidgetFormat.cost(totals.providerCost, known:, incomplete:
    !totals.pricingComplete)` (`apps/macos/AgentDeckWidget/WidgetDomain.swift`,
    used at `WidgetViews.swift:414,839,852`) drops its `≈` prefix when the flag
    is true, so leaving it catalog-driven would print an unhedged real-spend
    figure that omits ineligible events — this topic's stated release blocker.
    What was not carried with it is the explanation.
    `apps/macos/AgentDeckApp/MenuBarViewModel.swift:438-440` renders
    `DesktopCopy.costIncomplete`, which is `"Cost incomplete · %lld unpriced"`,
    interpolated with `unpricedComponents`. For a store whose events are all
    fully priced but include one non-spend-eligible event — precisely the case
    `unattributed_catalog_base_cost` exists to report, since that field is
    non-null only when such events *are* priced — the menu bar reads
    **"Cost incomplete · 0 unpriced"**. The state is deterministic, not
    hypothetical: `value.missing` is fed only from `result.Unpriced`, while
    `providerComplete` is cleared by `result.ProviderCost == nil`, which
    `calculateAttributedEvent` forces for every ineligible event.
    No canonical fixture covers it — all three that carry unattributed events
    happen to also carry an unpriced component — so no producer test sees it.
    Either the copy needs the new cause, or the payload needs to distinguish the
    two causes; the field semantics reconciliation this task owns is what
    decides which.
  - **[P2] O1-F3 — `attribution_reasons` is emitted as JSON `null` in a
    canonical v1 fixture whose sibling collections are all empty objects.**
    `emptyUsageSnapshot` (`internal/desktop/desktop.go:391-397`) initializes
    `Tokens`, `Counts` and `Warnings` but not the new `AttributionReasons`, so
    `desktop/fixtures/v1/snapshot-partial.json` carries `"tokens": {}`,
    `"counts": {}`, `"warnings": []` and `"attribution_reasons": null`. The
    populated path is correct — `loadUsage` wraps it in `nonNilMap` — so this is
    the unavailable-usage branch only. It matters because the checked-in
    fixtures are the wire specimens: a Swift consumer declaring the field as a
    non-optional `[String: Int64]`, exactly as it declares `counts`, would fail
    to decode this snapshot. Nothing decodes it yet, which is why it is P2 and
    not a blocker, but shipping a v1 specimen with an inconsistent null is how
    that decoder gets written wrong.
  - **[nit] O1-F4 — the `ATTRIBUTION REASONS` section header prints
    unconditionally.** `cmd/agentdeck/usage_family_text.go` appends the section
    before building its rows, so an empty store renders a heading with nothing
    under it. Every other quality signal in that renderer degrades to a value
    rather than to an empty block.
  - **[nit] O1-F5 — `usage stats` cost completeness changed without a contract
    line.** `statsCost`, `statsMetricValue` and `statsDimension` moved from the
    catalog-driven flag to `providerComplete`, so any store with pre-adoption
    history now reports `usage stats --metric cost` totals as unavailable and
    suppresses dimension shares. That follows from the same real-spend rule and
    is defensible, but `cli-manual.md` and `cli-design.md` were reconciled only
    for `usage summary`, and this task owns "CLI contracts". One sentence in the
    `usage stats` section closes it.
- Verified, not findings:
  - The reason vocabulary is closed and total. `resolveSessionAttribution`
    assigns `effective_route`/`ambiguous_route` from the route quality,
    `timeline_snapshot` on a successful fallback, and
    `before_adoption`/`coverage_gap` on `sql.ErrNoRows`; both `priceForEvent`
    paths set `exact_run` on the exact-run branch. No path can leave the reason
    empty, so no `""` key can enter the JSON object.
    `TestAttributionResolversClassifyRouteEffectFromPriorState` now asserts
    quality *and* reason for eleven cases through both resolvers, including the
    two new `before_adoption` and `coverage_gap` sessions.
  - The two timeline-existence implementations agree.
    `ProviderTimeline.HasClient` scans operations loaded by
    `LoadProviderTimeline` under `kind='provider.use'`, which is the same filter
    `ProviderTimelineExists` applies in SQL, and both accept a selection whose
    `operation_id` is NULL or whose operation is completed.
    `internal/store/providers_test.go` exercises both on one fixture in both
    directions. One asymmetry is inherited rather than introduced: the SQL
    selection join carries no `kind` filter, so a selection pointing at a
    completed non-`provider.use` operation would count in SQL and not in Go. The
    existing query at `providers.go:948` has the same shape, and selections are
    only written by `provider use` flows, so it is recorded as an observation.
  - The real-spend accounting itself is right at the summary level.
    `provider_cost` is gated on a separate `providerComplete`,
    `known_provider_cost` excludes ineligible and partially priced events, and
    `unattributed_catalog_base_cost` accumulates only `before_adoption` and
    `coverage_gap` catalog base and goes null when any such event has an unpriced
    component. `TestSummarizeEventsReportsReasonsAndSeparatesRealSpend` and
    `TestPresentationSummaryExcludesNonSpendEligibleProviderCost` cover both
    builders, and the quality tier's `CostIncomplete` still derives from
    `statsCost`, so task 2's behaviour is preserved.
  - The living contracts, the v0.5.0 release-note input, and the command-contract
    golden do carry the six reason keys and the new cost field, and the three
    canonical fixtures are producer output.
- Evidence: the manifest recomputed to
  `95e1ae124a3b7e21e0ed1ff58d5fc97e0132f8009d2aff9bfb791836a7c730a1` from the
  twenty-one declared files in the declared order.
  `./scripts/run-go-test.sh ./...` passes, as do
  `go test ./internal/usage/... ./internal/desktop/... ./cmd/agentdeck/...` and
  `go vet ./internal/... ./cmd/...`. `shasum` of `desktop/fixtures/v1/*.json` is
  unchanged after `AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop`.
  `bash scripts/check-topic-docs.sh` and `bash scripts/check-whitespace.sh` pass.
  `gofmt -l` reports only `cmd/agentdeck/usage_stats_viewer_test.go`, unmodified
  by this task and already unformatted at HEAD, as recorded for tasks 1 and 2.
  O1-F1 was traced from `calculateAttributedEvent` through
  `SessionInvocations` to `sessionViewerInvocationCost`; O1-F2 from
  `presentationTotals` through `DesktopPresentationTotalsV1` to
  `MenuBarViewModel.hero` and `WidgetFormat.cost`; O1-F3 read directly from the
  checked-in `snapshot-partial.json`. XCTest is unavailable under Command Line
  Tools, so the macOS consumers were read rather than executed; both findings
  that name them rest on source, not on a test run.
- Completion gate: VERIFIED — the WorkUnit
  `usage-attribution-precision:attribution-observability` was queried
  independently and returns `pass` on all seven criteria
  (`reason-vocabulary`, `timeline-gap-distinction`, `real-spend-boundary`,
  `summary-json-and-text`, `desktop-payload-and-fixtures`,
  `living-contracts-and-release-input`, `task3-boundary-and-l2`) against content
  state `95e1ae124a3b7e21e0ed1ff58d5fc97e0132f8009d2aff9bfb791836a7c730a1`,
  which reproduces from the working tree. A VERIFIED gate is not a review
  verdict: `real-spend-boundary` passed over the per-invocation leak O1-F1
  names, and no criterion reaches the desktop consumers of the fields this task
  redefined. The repair re-records evidence against the new state.
- Verdict: REOPEN

## Round 1 repair — 2026-08-27

- Repaired state: HEAD `7ce1c4f46e9555eb44ddbedc08d0a5d9b5a205c4` plus scoped
  manifest `sha256:fc510be10d640f66e74ef30d6c80b0c03bafde6aeb093cec8071500e1fba475a`.
  The manifest is SHA-256 over newline-terminated
  `<repository-relative-path>\t<git-blob-hash>\n` rows in exact lexicographic
  order across these 27 paths: `apps/macos/AgentDeckApp/DesktopCopy.swift`,
  `apps/macos/AgentDeckApp/Localizable.xcstrings`,
  `apps/macos/AgentDeckApp/MenuBarViewModel.swift`,
  `apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift`,
  `cmd/agentdeck/main_test.go`, `cmd/agentdeck/session_viewer_data_test.go`,
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json`,
  `cmd/agentdeck/usage_family_text.go`,
  `cmd/agentdeck/usage_family_text_test.go`, the three
  `desktop/fixtures/v1/snapshot-{complete,empty-client,partial}.json` files,
  `docs/specs/cli-design.md`, `docs/specs/cli-manual.md`, `docs/status.md`,
  `docs/topics/usage-attribution-precision/tasks.md`,
  `docs/topics/v0-5-0-contract/tasks.md`, `internal/desktop/desktop.go`,
  `internal/desktop/desktop_test.go`, `internal/store/providers.go`,
  `internal/store/providers_test.go`, `internal/usage/presentation.go`,
  `internal/usage/presentation_test.go`, `internal/usage/session_usage.go`,
  `internal/usage/session_usage_test.go`, `internal/usage/usage.go`, and
  `internal/usage/usage_test.go`.
- Repairer: Codex, single-agent bounded Repair; no external implementation or
  scoring tool was used.
- Authorized scope: O1-F1, O1-F2, O1-F3, O1-F4, and O1-F5 only.
- Disposition:
  - O1-F1 — CLOSED. `calculateAttributedEvent` again returns the honest
    per-event calculation. Aggregate-only copies exclude ineligible and partial
    provider summands; `SessionInvocation` instead makes an unattributed
    provider cost unavailable while retaining catalog base, and preserves a
    genuine known provider subtotal for spend-eligible partial pricing.
  - O1-F2 — CLOSED. Desktop keeps provider-cost completeness. When it is false
    with zero unpriced components, the menu bar now uses the separately
    localized attribution-unavailable copy rather than `0 unpriced`; a Go
    producer regression and Swift view-model regression cover that state.
  - O1-F3 — CLOSED. `emptyUsageSnapshot` initializes `AttributionReasons` to an
    empty map, and the producer-regenerated `snapshot-partial.json` now carries
    `"attribution_reasons": {}`.
  - O1-F4 — CLOSED. `usage summary` appends the `ATTRIBUTION REASONS` section
    only when at least one reason count is non-zero; an empty-summary regression
    rejects the empty header.
  - O1-F5 — CLOSED. Both living CLI contracts now state that `usage stats
    --metric cost` complete values and shares require every applicable event to
    be fully priced and spend-eligible while catalog coverage remains separate.
- Out of scope, not changed: the six-value reason classifier, timeline
  existence policy, task 2 quality semantics, wire version, unrelated desktop
  copy or layout, release actions, and `output/`.
- Verification: all finding-focused Go tests pass; affected usage/desktop/CLI
  packages pass; `scripts/run-go-test.sh ./...` passes; affected-package
  `go vet` passes; Swift repair files pass `swiftc -parse`; the localization
  catalog passes `jq empty`; canonical fixtures pass their producer and normal
  reproducibility paths; `bash scripts/check-topic-docs.sh`,
  `make check-whitespace`, and `git diff --check` pass on the final repair
  content. XCTest remains unavailable under Command Line Tools, so the new
  Swift assertion is syntax-checked but awaits independent runtime Review.
- Verdict: REOPEN — O1-F1 through O1-F5 repair complete, awaiting independent
  Re-review.

## Round 2 — 2026-08-27

- Reviewed state: HEAD `7ce1c4f46e9555eb44ddbedc08d0a5d9b5a205c4` plus the scoped
  manifest the repair declared,
  `sha256:fc510be10d640f66e74ef30d6c80b0c03bafde6aeb093cec8071500e1fba475a`,
  recomputed independently from the working tree over the twenty-seven declared
  paths in the declared order. It reproduced exactly.
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`, no external skill. Each
  finding was checked against the source its disposition names rather than
  against the disposition text, then the repair was traced outward for
  consequences of its own — in particular to the second `session show` renderer
  the repair does not mention. Verification commands were re-run rather than
  taken from the repair's report.
- Scope: O1-F1 through O1-F5, and any newly blocking repair regression.
- Findings:
  - **O1-F1 — CLOSED.** The exclusion moved out of the per-event calculation:
    `calculateAttributedEvent` is now a plain `Calculate`, and a new
    `aggregateAttributedResult` applies the provider-spend rule at the four
    aggregate sites only (`summarizeEvents`, `Stats`, `Presentation`,
    `PriceDiagnostics`). `SessionInvocations` instead makes the absence explicit
    per invocation — `ProviderCost = nil`, `KnownProviderCost = ""` — so
    `sessionViewerInvocationCost` falls through to its unavailable branch and the
    row no longer renders `0.000000000 (partial)`. The regression asserts that
    string is absent by name, which is the negative form the finding needed, and
    `internal/usage/session_usage_test.go` pins the shape directly: catalog base
    present, provider cost nil, known provider subtotal empty. The second half of
    the finding is closed too — a spend-eligible but partially priced invocation
    keeps its genuine known subtotal, because only `!spendEligible` clears it
    on that path.
  - **O1-F2 — CLOSED.** The flag keeps its provider-cost meaning, so the
    Widget's `≈` hedge is preserved, and the explanation now branches:
    `MenuBarViewModel.hero` uses `DesktopCopy.costIncomplete` only when
    `unpricedComponents > 0` and the new `costIncompleteAttribution`
    ("Cost incomplete · attribution unavailable") otherwise. The new key is
    registered in the localized-string list and carries `en` and `zh-Hans` units
    in `Localizable.xcstrings`. The branch is exhaustive: `providerComplete` can
    only be false with zero unpriced components when `!spendEligible` caused it,
    since `CatalogBaseCost == nil` always contributes an unpriced component, so
    the new copy never claims attribution for a pricing gap.
  - **O1-F3 — CLOSED.** `emptyUsageSnapshot` now initializes
    `AttributionReasons`, `TestEmptyUsageSnapshotUsesNonNilCollections` covers all
    four collections, and the producer-regenerated `snapshot-partial.json` carries
    `"attribution_reasons": {}` beside its `{}` siblings.
  - **O1-F4 — CLOSED.** The `ATTRIBUTION REASONS` section is appended only when a
    reason row exists, and `TestUsageSummaryOmitsEmptyAttributionReasonsSection`
    rejects the bare header.
  - **O1-F5 — CLOSED.** Both living contracts now state the `usage stats
    --metric cost` boundary, and `cli-manual.md` additionally gained the
    per-invocation sentence O1-F1 left unstated: an unattributable invocation
    exposes no provider cost rather than `0`, while a spend-eligible partial one
    keeps its real known subtotal.
- Verified, not findings:
  - The two `session show` renderings now use different words for the same
    unattributed invocation: the viewer's `sessionViewerInvocationCost` returns
    `unpriced`, while `sessionShowInvocationPricing` still answers `partial`,
    because `KnownCatalogBaseCost` is non-empty. Recorded as an observation
    rather than a finding. Neither states a false amount, which was the substance
    of O1-F1; the divergence is a status word, and "partial" is if anything the
    more accurate of the two for an event that is priced but unattributable.
    Reopening on it would be re-litigating the branch the finding itself named as
    the honest one. It belongs to a vocabulary pass over the pricing-status
    words, not to this task.
  - `SessionInvocation.KnownProviderCost` expresses absence as `""` while its
    `*string` siblings use `null`. The field has always been a non-pointer
    string, the command-contract golden still types it `string`, and
    `cli-manual.md` now states the semantics, so the alternative is changing a
    stable field's JSON type on a shipped contract — a larger change than the
    inconsistency justifies, and not one this review has grounds to require.
  - The `CATALOG BASE COST` detail field keeps its warning role for an
    unattributed invocation, because `sessionViewerPricingDetailRole` keys off
    the provider-cost status. That coupling predates this topic and was not
    introduced by the repair.
- Evidence: the manifest recomputed to
  `fc510be10d640f66e74ef30d6c80b0c03bafde6aeb093cec8071500e1fba475a`.
  `./scripts/run-go-test.sh ./...` passes, as does `go vet ./internal/...
  ./cmd/...`. `shasum` of `desktop/fixtures/v1/*.json` is unchanged after
  `AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop`.
  `swiftc -parse` succeeds on both changed Swift sources and
  `Localizable.xcstrings` parses as JSON. `bash scripts/check-topic-docs.sh` and
  `bash scripts/check-whitespace.sh` pass. `gofmt -l` reports only
  `cmd/agentdeck/usage_stats_viewer_test.go`, unmodified by this task and already
  unformatted at HEAD, as recorded for tasks 1 and 2.
  XCTest remains unavailable under Command Line Tools, so
  `MenuBarViewModelTests.testAttribution…` was read and syntax-checked rather
  than executed; the O1-F2 closure rests on the `MenuBarViewModel.hero` branch,
  the registered copy, and the exhaustiveness argument above, not on that test
  having run.
- Completion gate: VERIFIED — the WorkUnit
  `usage-attribution-precision:attribution-observability` was queried
  independently and returns `pass` on all nine criteria, the seven from Round 1
  plus the per-invocation and desktop-consumer criteria the repair added, against
  content state
  `fc510be10d640f66e74ef30d6c80b0c03bafde6aeb093cec8071500e1fba475a`, which
  reproduces from the working tree. Evidence for this round is re-recorded
  against this round's post-synchronization state and supersedes the repair-round
  records, so one live record describes the state the PASS was given on.
- Verdict: PASS
