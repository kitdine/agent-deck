---
status: historical
topic: usage-attribution-precision
subject: architecture.md
retired: 2026-09-01
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

## Round 2 — 2026-08-26

- Reviewed state: HEAD `2b9dd2bd43d130ea31c6065df11275756e38605f` plus committed
  document blob `2174860af3646359e86f7cfaa8ace1611dbd1b14` (`requirements.md`
  `0abe112cccb025e40e69ac6c531e93a3621f10be`, `tasks.md`
  `1651a86c465f42b049bd75992726c873ea247862` re-reviewed in the same pass).
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; finding-by-finding
  inspection against the source each claim names, followed by a check for
  consequences the repair itself introduces.
- Scope: A1-F1 through A1-F5, and any newly blocking repair regression.
- Findings:
  - **A1-F1 — CLOSED.** The new "Read-time quality derivation" section
    (`architecture.md:95-103`) makes the choice the document previously lacked:
    step 2 derives quality from the positioned route, does not read the
    persisted `usage_session_routes.quality` as its verdict, and leaves the
    column, `recordSessionRouteConn`, and existing rows untouched.
  - **A1-F2 — CLOSED.** Both `readPriceResolver.priceForEvent` and
    `Service.priceForEvent` are named (`:16-19`) and required to share one
    policy helper or prove equivalence through a shared case table (`:97-98`).
  - **A1-F3 — CLOSED.** Contract impact now carries the desktop `Presentation`
    consumer, makes its tier mapping quality-driven, states that provider
    identity does not override it — which was the inversion Round 1 found — and
    requires canonical fixtures to be regenerated with the producer command
    rather than hand-edited (`:147-153`).
  - **A1-F4 — CLOSED.** The ambiguity rule now names `classifyConfigChange`
    inside `RecordHookDelivery`, and `recordSessionRouteConn` replaces the old
    `recordSessionRoute` reference.
  - **A1-F5 — CLOSED.** The split has both a mechanism (a client-wide timeline
    existence check after `sql.ErrNoRows`, explicitly not inferred from the
    event timestamp) and an observable shape (the six-key
    `attribution_reasons` object).
  - **[P1] A2-F1 — step 2 treats "provider is known" as "provider was in
    effect", so it promotes to `exact` the one class of route this project has
    already ruled does not take effect.** Target step 2 grades a positioned
    route `exact` / `effective_route` whenever its provider is known
    (`:47-54`), and justifies that grade by asserting such a route "is an
    observation of the provider and multiplier **actually in effect**, not a
    guess" (`:67-68`). The assertion does not hold for every route the resolver
    will position. `RecordClaudeConfigChange`, the writer for every Claude
    `ConfigChange` route captured before `7db5618`, recorded a route carrying
    the **new** selection for any matched change:
    `recordSessionRoute(ctx, route, runtimeProviderName(snapshot.Name),
    snapshot.Multiplier, snapshot.ViaWrapper)`. `switch-effectiveness-boundary`
    task 3 replaced that with `retain` precisely because an already-keyed
    running session does not adopt key rotation or key removal. A route written
    for one of those transitions therefore names a provider the session never
    used, and this design grades it `exact` — the value the topic offers as its
    answer to whether a recorded cost is real.
    The subset is narrow and must not be overstated. Of 166 route rows in the
    operator's store, 135 are `SessionStart` and are correct under both the old
    and the new capture policy. Of the 31 Claude `ConfigChange` rows, 9 name the
    same provider as the prior route, 15 have no prior route, and 3 are
    `official -> sssaicode`, which is the no-key-to-first-key transition this
    topic says *does* apply live. Only 4 rows — one `cubence -> official` and
    three `sssaicode -> official`, all credential removals from an already-keyed
    state — record a transition that provably did not take effect. They position
    534 of 12,929 Claude events, or 4.13%.
    Two things this finding does **not** claim. It does not claim the repair
    changes any amount: those events are already priced through the same row at
    the same multiplier by today's build, the capture defect predates this topic,
    and route capture is an explicit Non-Goal. And it does not justify a blanket
    rule such as excluding every pre-cutoff `ConfigChange` route, which would
    hold 2,976 events at `estimated` to avoid mislabelling 534 — a coarse
    provenance test standing in for the classification the design is missing.
    What does change is the claim attached to those 534 events: `estimated`
    becomes `exact`, and the desktop tier moves from `inferred` to
    `determinable` (`:147-149`), so a hedge becomes an assertion on a shipped
    v0.5.0 surface, and the independent `v0.5.0` candidates that consume this
    output as ground truth (`requirements.md:13-15`) inherit it.
    The distinction is derivable at read time from evidence the resolver already
    loads. Whether the prior effective state was keyed, and whether this change
    moves away from it, follows from the session's own earlier route and from
    the provider timeline at session start — the same two sources task 3's
    prior-state classifier uses, and the same two this document already reads in
    steps 2 and 3. The gap is the rule, not the data: step 2 needs to say that a
    known provider alone does not establish effect for a `ConfigChange` route,
    and that a route recording a transition away from an already-keyed prior
    state stays `estimated`.
  - **[P2] A2-F2 — the resolver the contract change targets is described as
    legacy.** `:16-19` assigns `Service.priceForEvent` to "session and legacy
    summary paths". It reaches `summarize` from `Summary`
    (`internal/usage/usage.go:2108`) and `SummaryRange` (`:2116`), which are the
    two code paths behind the `usage summary` command
    (`cmd/agentdeck/main.go:3143,3154`) — the surface whose JSON `counts` and
    warnings this document's Contract impact section changes. Calling it legacy
    invites an implementer to deprioritize the resolver that serves the contract.
    `readPriceResolver` is likewise described as "bounded range and desktop
    presentation" while it also serves `Stats`, `Sessions`, and
    `PriceDiagnostics`. The shared-policy requirement at `:97-98` keeps this from
    affecting correctness, which is why it is P2 and not a blocker.
- Evidence: `git show 08a713b:internal/usage/routes.go:45-53` is the
  pre-repair `RecordClaudeConfigChange`. `rg` finds no `cutoff`, `provenance`,
  `observed_at <`, or `delivery_id` in any of the three documents. Caller
  mapping was resolved through `loadReadPriceResolver`
  (`session_usage.go:103`, `usage.go:1791,2676,3003`, `presentation.go:257`)
  and `summarize` (`usage.go:2113,2121`, `session_usage.go:53`).
  The A2-F1 counts were measured on a read-only copy of the operator's live
  store, whose routes were all written by the installed `v0.4.1` binary and so
  all predate `7db5618`: 166 route rows total, 104 `SessionStart`/codex,
  31 `SessionStart`/claude, 31 `ConfigChange`/claude with no `unknown` provider
  among them. Classifying each `ConfigChange` row against the session's own
  preceding route gives 9 same-provider, 15 with no prior route, 3
  `official -> sssaicode`, 1 `cubence -> official`, and 3
  `sssaicode -> official`. Positioning events at or after the last four rows
  yields 534 of 12,929 Claude events. The store copy was inspected read-only
  and was not modified.
- Acceptance validation data for A2-F1: the classification below was measured on
  the operator's live store on 2026-08-26 and is the reference fixture for
  accepting the repair. Every row predates `7db5618`, so it exercises exactly the
  legacy-capture case. An implementation that satisfies A2-F1 must reproduce the
  `expected quality` column; a run that promotes the last group, or that holds
  any other group at `estimated`, has not met the finding.

  | Route group | Rows | Expected quality | Why |
  | --- | --- | --- | --- |
  | `SessionStart` / codex | 104 | `exact` | Configuration is loaded at process start and kept for the process lifetime |
  | `SessionStart` / claude | 31 | `exact` | Same; the route is what the process loaded |
  | `ConfigChange` / claude, prior route names the same provider | 9 | `exact` | No transition occurred; the session is on that provider either way |
  | `ConfigChange` / claude, no prior route in the session | 15 | resolve from the provider timeline at session start, then apply the two rules below | The prior effective state is determinable from evidence steps 2 and 3 already read |
  | `ConfigChange` / claude, `official -> sssaicode` | 3 | `exact` | No-key to first-key is the one transition a running Claude session adopts |
  | `ConfigChange` / claude, `cubence -> official` and `sssaicode -> official` | 4 | **`estimated`** | Credential removal from an already-keyed state; task 3 rules it does not take effect, so the recorded provider was never in effect |

  Totals to check against: 166 route rows; 12,929 Claude events; 534 Claude
  events (4.13%) positioned at or after the final group; 2,976 Claude events
  positioned at or after any `ConfigChange` route, which is the number a blanket
  pre-cutoff exclusion would wrongly hold at `estimated`.

- Completion gate: NOT_REQUIRED — a document review round records no completion
  evidence; the document boundary is crossed only on `PASS`.
- Verdict: REOPEN

### Repair disposition — Round 2 — 2026-08-26

- A2-F1 closed: step 2 now loads `hook_event`, resolves prior effective state
  from the preceding session route or session-start timeline, and classifies
  legacy `ConfigChange` rows by actual effect. It preserves the Round 2
  reference totals, uses no provenance cutoff, and changes confidence rather
  than historical amounts.
- A2-F2 closed: the document names the current callers of both resolvers and
  identifies `Service.priceForEvent` as the path behind `usage summary`, not a
  legacy summary path.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 3 — 2026-08-26

- Reviewed state: HEAD `2b9dd2bd43d130ea31c6065df11275756e38605f` plus the
  uncommitted document blob `6a43f5f3b4cf6e71042390265e542a647919dda2`
  (`requirements.md` `647c0963d5a012b0bb513645042747461d921080`, `tasks.md`
  `9f08908761ae0cc63a593190a5cbdbe94aa17fff` re-reviewed in the same pass);
  subject digest `dc7acd8355b47c8182d4cdd9d44a56c8d374d882fd909887d06f9c493f7e296f`.
- Reviewer: claude-code, independently re-reviewing the Round 2 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; each finding checked
  against the repaired text, the source its claims name, and — for the
  classification A2-F1 turns on — a re-measurement of the operator's live store,
  including the one group Round 2 left unresolved.
- Scope: A2-F1 and A2-F2, and any newly blocking repair regression.
- Findings:
  - **A2-F1 — CLOSED.** Step 2 no longer grades on provider presence. It
    branches on `hook_event` and, for a `ConfigChange`, on the prior effective
    state resolved from the session's preceding route or the provider timeline
    at session start — the two sources Round 2 identified as already loaded. The
    new "Legacy `ConfigChange` effect classification" section states the closed
    four-row rule, and its rows correspond exactly to the measured groups:
    same-provider and no-key-to-first-key reach `exact`, rotation or removal
    from an already-keyed state and an unresolvable prior state stay
    `estimated`. The section also rejects by name the substitution Round 2
    rejected — "it does not use commit time, `observed_at`, delivery ID, or
    another provenance cutoff" — and states that held rows keep their existing
    amount and spend eligibility, so the repair changes the confidence claim
    only, which is the boundary Round 2 drew.
  - **A2-F2 — CLOSED.** The consumer mapping is now accurate:
    `Service.priceForEvent` serves `Summary` and `SummaryRange`, named as the
    two `usage summary` paths, plus session-summary aggregation, while
    `readPriceResolver.priceForEvent` serves Stats, Sessions, PriceDiagnostics,
    bounded range reads, and desktop `Presentation`. The "legacy" label is gone.
- Verified, not findings:
  - The document's acceptance claim that 27 `ConfigChange` rows reach `exact` is
    correct. Round 2 deliberately left the fifteen rows with no preceding
    session route unresolved, so this round resolved them: through the provider
    timeline at session start they are ten `official -> official`, three
    `official -> sssaicode`, and two `cubence -> cubence`, all of which the
    four-row rule grades `exact`. With the nine same-provider and three
    first-key rows, 27 reach `exact` and the four keyed-to-official removals
    stay `estimated`, positioning 534 events.
  - `sessionRouteAt` currently selects only `provider,multiplier,quality`
    (`internal/usage/routes.go:422`), so "Both resolver paths must load
    [`hook_event`]" names a real and necessary read-side change, and confining
    `routes.go` to its read side keeps `recordSessionRouteConn` and the Non-Goal
    on the writer intact.
  - Reusing `ambiguous_route` for both an unknown provider and an unproven
    effect is a deliberate, documented choice ("the ambiguity is whether that
    route took effect"), and the two cases stay separable because the
    attribution carries `spend_eligible` independently. Recorded as accepted
    rather than as a finding.
- Evidence: the repaired policy is at `architecture.md:46-60`, the legacy
  classification section and its rule table follow the step-2 narrative, and the
  consumer mapping is at `:14-21`. `internal/usage/routes.go:422` is the current
  `sessionRouteAt` projection. The fifteen-row resolution was measured on a
  read-only copy of the live store, joining each row's session to
  `provider_selections` at `usage_sessions.first_at`; the copy was deleted after
  inspection.
- Completion gate: VERIFIED — CEv1 gate
  `usage-attribution-precision:architecture.md` records this PASS against exact
  content state
  `dc7acd8355b47c8182d4cdd9d44a56c8d374d882fd909887d06f9c493f7e296f`.
- Verdict: PASS
