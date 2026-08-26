---
status: active
topic: usage-attribution-precision
subject: architecture.md
---

# Review log — usage-attribution-precision / architecture.md

## Round 1 — 2026-08-26

- Reviewed state: HEAD `5f2189550348a5a3f65fca42d6b92e8d07b2b5ac` plus the
  uncommitted document blob `fa92fc1d2b9d220855c69dd6b9b1ff97b380a964`
  (`requirements.md` `42bc63dc0d7890d2f2ffd1be57162fa0842dd46f`, `tasks.md`
  `46877d60171e1dd584743174f991626170360011` reviewed in the same pass).
- Reviewer: claude-code, independently reviewing the design authored by
  `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal document Review under `development-workflow`. The stated
  current resolution order was compared line by line against
  `readPriceResolver.priceForEvent`, and every named symbol was resolved in the
  source before its claim was accepted.
- Scope: the current and target resolution orders, the per-client time
  semantics, the ambiguity rule, the unattributed boundary, and the declared
  contract impact.
- Findings:
  - **[P1] A1-F1 — the target order promotes a route to `exact` without saying
    how, and the only stored value says otherwise.** Target step 2 yields
    "`exact` when the positioned route is unambiguous"
    (`architecture.md:43-48`). But route quality is persisted, not derived:
    `usage_session_routes.quality` is `TEXT NOT NULL`
    (`internal/store/migrations.go:107`), `recordSessionRouteConn` writes the
    literal `"estimated"` for every route it inserts
    (`internal/usage/routes.go:402,416`), and the resolver assigns that stored
    value directly (`internal/usage/usage.go:2620`). The design never chooses
    between the two available mechanisms, and they are not equivalent: deriving
    the promotion at read time leaves the persisted column unused by the very
    step that owns it and contradicts `requirements.md:63-64`'s claim that no
    such column exists; changing the write instead leaves every route row
    already stored as `estimated` — the entire existing corpus — unpromoted
    unless it is backfilled, which is the schema/data migration the same
    requirement forbids. Task 1 cannot be implemented from this document as
    written. The document's own note that step 2's quality is "hardcoded in
    `recordSessionRoute`" (`:21`) shows the fact was known; the target order
    just never resolves it.
  - **[P1] A1-F2 — a second implementation of the same resolution order is
    unaccounted for.** The document scopes itself to
    "`readPriceResolver.priceForEvent` (`internal/usage/usage.go`)" (`:16`),
    but a second, independent implementation of the identical four-step order
    exists at `internal/usage/usage.go:3187`, `func (s *Service)
    priceForEvent`, with its own `quality, mult := "historical", "1"` default,
    its own `usage_runs.exact` branch, and its own `sessionRouteAt` branch.
    Every rule this design states — the `exact` promotion, the ambiguity test,
    the `unattributed` split — must land in both, or the same event resolves to
    a different quality depending on which read path served the command. No
    document in the set mentions the duplication, so an implementer following
    the design will change one and silently diverge the other.
  - **[P1] A1-F3 — Contract impact omits the surface where this vocabulary
    already lives, and the change inverts one of its buckets.** The section
    limits observable impact to `usage summary`'s JSON `counts`, its two
    warnings, the CLI manual, the CLI design contract, and the release notes
    (`architecture.md:94-100`). It misses `addPresentationQuality`
    (`internal/usage/presentation.go:361-367`), which already classifies every
    event into `determinable` / `inferred` / `unattributed` by a different
    rule: `provider == "unknown"` becomes `unattributed`, and `quality ==
    "exact"` becomes `determinable`. Two consequences follow and neither is
    stated. First, this design defines an ambiguous route — one recording
    `provider = unknown` — as staying `estimated` (`architecture.md:76-79`),
    yet that existing mapping keys off the provider, so those events land in
    the `unattributed` bucket, which `requirements.md:68-70` requires be kept
    out of any real-spend total. An event the design calls merely estimated is
    thereby excluded from real spend. Second, this accumulator feeds
    `Presentation`, whose only consumer is `internal/desktop/desktop.go`, and
    the canonical desktop fixtures encode the result — `snapshot-complete.json`
    alone contains 18 occurrences each of `determinable`, `inferred`, and
    `unattributed`. Changing attribution quality therefore changes a checked-in
    producer-output contract. This session already saw that exact failure mode
    close a review as H3-F1 in `switch-effectiveness-boundary`, where a schema
    count moved and the canonical fixtures did not.
  - **[P2] A1-F4 — the ambiguity rule points at a symbol that no longer
    exists.** "which `RecordClaudeConfigChange` writes deliberately when the
    managed settings file did not match a completed selection" (`:77-79`)
    names a function with zero references anywhere in the repository. The
    behavior is still real, but it now lives in `classifyConfigChange` inside
    `RecordHookDelivery` (`internal/usage/routes.go`), where a mismatch sets
    `routeProvider = "unknown"`, `routeMultiplier = "1"`, and
    `effect = unknown`. `recordSessionRoute` (`:21`) is likewise now
    `recordSessionRouteConn`. Both were renamed by the upstream switch topic
    this design consumes, so the definition of the load-bearing ambiguity rule
    currently cannot be located from the document.
  - **[P2] A1-F5 — the unattributed split is mandated without an observable
    shape.** The table separates "Before adoption" from "Coverage gap" and
    marks both `Reportable`, calling the second "a defect signal worth
    surfacing" (`:86-92`), but Contract impact specifies only the rename of
    `historical` to `unattributed`. Whether the two states are two `counts`
    keys, one key plus a detail field, or a distinction visible only in
    `usage diagnose` is undefined, and the mechanism that tells them apart —
    presumably whether the client's timeline holds any entry at all, since
    `SnapshotAt` returns `sql.ErrNoRows` for both — is left to be inferred.
    See T1-F2 for how this lands on the task that must implement it.
- Evidence: the document's stated current order was verified correct against
  `usage.go:2604-2635` for all four steps, including that step 4 is the
  initial default rather than a terminal branch. `rg` finds no
  `RecordClaudeConfigChange` in the repository. `internal/desktop/desktop.go`
  is the sole caller of `Service.Presentation`. Fixture occurrence counts come
  from `desktop/fixtures/v1/snapshot-complete.json`. The claim that the
  provider timeline stays a session-start fallback for both clients is
  accurate (`usage.go:2623-2631`).
- Completion gate: NOT_REQUIRED — a document review round records no
  completion evidence; the document boundary is crossed only on `PASS`.
- Verdict: REOPEN

### Repair disposition — 2026-08-26

- A1-F1 closed: the target explicitly derives route quality at read time,
  ignores the persisted route-quality verdict, and requires no writer change,
  backfill, or migration.
- A1-F2 closed: both `readPriceResolver.priceForEvent` and
  `Service.priceForEvent` are named and bound to one policy plus parity cases.
- A1-F3 closed: the desktop Presentation mapping, spend-eligibility behavior,
  producer tests, and canonical fixture regeneration are part of the contract.
- A1-F4 closed: the ambiguity rule now locates `classifyConfigChange` inside
  `RecordHookDelivery` and the persistence helper `recordSessionRouteConn`.
- A1-F5 closed: the contract defines the `attribution_reasons` JSON/text shape,
  the timeline-existence test for `before_adoption` versus `coverage_gap`, and
  the real-spend versus unattributed catalog-base-cost boundary.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.
