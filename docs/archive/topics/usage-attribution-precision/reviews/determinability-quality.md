---
status: historical
topic: usage-attribution-precision
subject: determinability-quality
retired: 2026-09-01
---

# Review log — usage-attribution-precision / determinability-quality

## Round 1 — 2026-08-26

- Reviewed state: HEAD `1db02b8fd44b7f128d40fcc6aa7023b6cab0be7d` plus the
  uncommitted blobs `internal/usage/usage.go`
  `fdd8c4ea59f2e6e0688fa94e55163637cd44c609`, `internal/usage/routes.go`
  `1b93cbcacfca14747affc305516234c1141cc46f`,
  `internal/usage/presentation.go`
  `4dda906e4eb95e1fdb7a51a121c2596617436c57`,
  `internal/usage/usage_test.go`
  `8dbcea0c5e20fd9dcce62610de461317522701e2`,
  `internal/usage/routes_test.go`
  `c79b39c6c868730bdae5be07662351f4f514924c`,
  `internal/usage/presentation_test.go`
  `467398a2c86123f258376a38986e38fc6d41a889`, and the unchanged consumer
  `cmd/agentdeck/usage_family_text.go`
  `f0ee03fcb5d966f0da914a391ba9351e4635ea9f`.
- Reviewer: claude-code, independently reviewing the task 2 implementation
  authored by `codex`; this workflow turn kept production code, tests, and
  configuration read-only.
- Method: Formal REVIEW under `development-workflow`, no external skill. Each
  acceptance clause of `tasks.md` task 2 and each row of `architecture.md`'s
  "Legacy `ConfigChange` effect classification" table was checked against the
  source that implements it, then the change was traced outward to every
  consumer of the values it renames. Verification commands were re-run
  independently rather than taken from the implementation's report.
- Scope: `internal/usage/usage.go` (route metadata, `routeQuality`,
  `resolveSessionAttribution`, `exactRunRoute`, both `priceForEvent` paths,
  `summarizeEvents`), `internal/usage/routes.go` read side,
  `internal/usage/presentation.go` tier mapping and summary counts, the three
  changed test files, the regenerated `desktop/fixtures/v1/*.json`, the changed
  `cmd/agentdeck` golden and stable JSON contract, and every remaining reader of
  the `historical` quality key.
- Findings:
  - **[P1] D1-F1 — the `usage summary` text surface now prints a permanently
    zero attribution count, and its three attribution rows no longer sum to
    `EVENTS`.** This task renames the fallback quality from `historical` to
    `unattributed` in every producer: `summarizeEvents`
    (`internal/usage/usage.go:3282`), `summarizeWithReadPriceResolver` (`:3279`)
    and `newPresentationSummaryBuilder` (`internal/usage/presentation.go:737`)
    initialize `Counts` with `unattributed`, and the only writes into that map
    are `out.Counts[quality]++` (`usage.go:3304`) and
    `b.summary.Counts[attribution.quality]++` (`presentation.go:745`), where
    `quality` is now one of `exact`, `estimated`, `unattributed`. No code path
    can produce a `historical` key any more. The text renderer was not moved
    with them: `cmd/agentdeck/usage_family_text.go:57` still reads
    `value.Counts["historical"]`, a missing map key, so
    `renderUsageFamilySummary` — reached from `cmd/agentdeck/main.go:4367`, the
    `usage summary` text path — emits `HISTORICAL ATTRIBUTION 0` for every store,
    including one whose events are entirely unattributed.
    The result is not merely a stale label. `EVENTS` still counts every event
    while `EXACT + ESTIMATED + HISTORICAL` now omits the unattributed ones, so
    the printed rows contradict each other, and the `WARNINGS` row directly
    beneath them already reads `unattributed attribution` — the warning string is
    derived from the same quality value (`usage.go:3306`,
    `presentation.go:747`, `session_usage.go:119`) and so did move. A user is
    shown a warning about unattributed attribution next to a count asserting
    there is none. That is the class of silently wrong number this topic exists
    to remove, on the surface `architecture.md` names as the contract consumer.
    The deferral to task 3 does not cover this. `architecture.md`'s closing
    sentence defers the *new* observability shape — the `attribution_reasons`
    object, the `ATTRIBUTION REASONS` section, the manual and the design
    contract — to "the same task that introduces observability". It does not
    license this task to leave an existing, already-correct count reading a dead
    key. The distinction is between a surface that does not yet show something
    new and a surface that now shows a wrong value; only the first is a
    deferral. The same reasoning is why this task correctly did update
    `cmd/agentdeck/testdata/phase7/gui-json-contract.json` and
    `cmd/agentdeck/session_viewer_data_test.go`, both outside its `Files` list:
    the rename's blast radius is the boundary, not the list.
    No test covers the row, which is why the full suite passes over it.
    Repair is bounded: point the row at `Counts["unattributed"]`, rename its
    label to match the vocabulary the JSON and warnings already use, and add a
    renderer assertion that a store with an unattributed event prints a non-zero
    count. Task 3 keeps the `ATTRIBUTION REASONS` section.
  - **[P2] D1-F2 — the regression does not prove that a blanket pre-cutoff
    exclusion would be wrong; it restates the number.** `tasks.md` task 2 and
    the Beads acceptance criteria both require the focused regression to prove
    that claim, and `reviews/architecture.md` Round 2 fixes 2,976 as the count a
    coarse provenance test would wrongly hold at `estimated`.
    `TestRouteQualityMatchesLegacySixGroupReference`
    (`internal/usage/usage_test.go`) accumulates `configChangeEvents` from the
    `configChange` boolean the test itself sets on each group, outside the
    `got == "exact"` branch, and then asserts it equals 2,976. That sum is
    independent of `routeQuality`'s return value: the assertion holds identically
    if the classifier grades every `ConfigChange` row `estimated`, which is
    exactly the blanket exclusion it claims to refute. The refutation lives only
    in the failure message.
    The rest of the test is sound and this finding does not touch it — the six
    `routeQuality` calls are real, and `exactRows`/`estimatedRows` and the
    12,395/534/12,929 event totals *are* coupled to the verdicts, because a
    misclassified group moves its rows and events into the other bucket and
    breaks them. What is missing is one assertion of the same kind: the events
    positioned by `ConfigChange` rows the classifier grades `exact` must be
    2,442, so that demoting them wholesale is a measurable failure rather than a
    comment.
  - **[nit] D1-F3 — the no-prior-route branch is covered only where it resolves
    to `exact`.** `resolveSessionAttribution` falls back to the session-start
    timeline snapshot when a positioned `ConfigChange` has no preceding route
    (`usage.go`), and `reviews/architecture.md` Round 3 resolved those fifteen
    live rows as ten `official -> official`, three `official -> sssaicode` and
    two `cubence -> cubence`. `TestAttributionResolversClassifyRouteEffectFromPriorState`
    exercises the branch once, with an `official` timeline and a keyed route.
    Neither of the two branches that keep a row `estimated` through this path is
    covered: a timeline that is already keyed while the route moves away from it,
    and a client with no timeline entry at all, where `snapshotAt` returns
    `sql.ErrNoRows` and `priorFound` stays false. Both are one table row each in
    the existing test.
  - **[nit] D1-F4 — `routeQuality` adds a client restriction the normative table
    does not state.** `usage.go:2680` returns `estimated` for any non-Claude
    `ConfigChange` before the four-row rule is applied. The rule in
    `architecture.md` is written per transition, not per client, so under the
    document a Codex `ConfigChange` naming the same provider as its predecessor
    is `exact` while this code makes it `estimated`. The deviation is
    conservative and currently unreachable — the operator's store holds no Codex
    `ConfigChange` row, and the post-`7db5618` writer records none — so it is not
    a defect. It is unexplained: the reason (Codex activates configuration only
    at process start, so a mid-session `ConfigChange` is never adopted) is real
    and belongs in a comment on that line, or the restriction should go.
  - **[nit] D1-F5 — dead condition.** In `readPriceResolver.priceForEvent`, the
    exact-run branch sets `attribution.spendEligible = attribution.provider !=
    "unknown"` immediately after `exactRunRoute` has already rejected an unknown
    provider, so the right-hand side is always true.
- Evidence: `rg 'Counts\['` over `internal/usage` returns seven writes, none of
  them `historical`; `rg '"historical"'` over `internal`, `cmd`, `desktop`,
  `apps` finds one live reader, `cmd/agentdeck/usage_family_text.go:57`, the rest
  being test literals and unrelated price-layer locals.
  `go test ./internal/usage/... ./internal/desktop/... ./cmd/agentdeck/...`
  passes, and so does `./scripts/run-go-test.sh ./...`. Canonical fixture
  reproducibility was confirmed independently: `shasum` of
  `desktop/fixtures/v1/*.json` is unchanged after
  `AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop`, so the four payloads
  are producer output and not hand edits. `gofmt -l` reports only
  `cmd/agentdeck/usage_stats_viewer_test.go`, which is unmodified by this task
  and already unformatted at HEAD. `go vet` on the changed packages is clean.
  The macOS synthetic verifier failure the implementation disclosed is
  pre-existing and unrelated: `apps/macos/AgentDeckVerification/main.swift`
  requires three helper invocations (`usage scan`, `session scan`, snapshot)
  while `EmbeddedHelperRunner` performs two (`desktop refresh-indexes`,
  snapshot), and neither file is touched by this task.
  Classification was checked row by row against the six-group table:
  `routeQuality` grades `SessionStart` `exact` for both clients, a same-provider
  `ConfigChange` `exact`, `official -> keyed` `exact`, `keyed -> official`
  `estimated`, an unresolvable prior state `estimated`, and `provider = unknown`
  `estimated`; `TestAttributionResolversClassifyRouteEffectFromPriorState` drives
  all seven through both real resolvers against a live store, which is the
  substantive proof the task needed. `positionedSessionRoute` returns
  `routes[index-2]` as the predecessor, matching the document's "immediately
  preceding route in the same session", and the timeline fallback for the prior
  state is positioned at `sessionStartAt`, not at event time.
  Presentation tiers map strictly from quality (`presentation.go:360-366`), so a
  known-provider `estimated` event lands in `inferred` and provider identity no
  longer overrides the tier, and `tier.stats` is a pointer, so the
  `spendEligible` incompleteness propagation persists.
  `Service.priceForEvent` now reads `runMultiplier`/`runProvider` from the event
  instead of re-querying `usage_runs`; all three loaders
  (`usage.go:3223`, `usage.go:3254`, `session_usage.go:169`) select
  `r.multiplier,r.provider` through the same join, so the substitution is
  equivalent.
- Completion gate: VERIFIED — the WorkUnit
  `usage-attribution-precision:determinability-quality` was queried
  independently and returns `pass` on all six criteria
  (`route-effect-classification`, `legacy-six-group-regression`,
  `quality-vocabulary`, `desktop-quality-and-fixtures`, `task2-boundary`,
  `l2-verification`) against content state
  `10518e7f71d874868825385f2f432643a44578bff5ccbd4f3ff3e7c084ddc4fc`. Recorded
  for the round as queried, with two limits stated rather than assumed. The
  state binds `head=<HEAD-SHA>;manifest=<sha256 of ordered task file blob
  hashes>`, and that recipe does not name the file order or the joining
  separator, so the digest could not be reproduced from the working tree and the
  gate cannot be confirmed to cover exactly the blobs reviewed above. And a
  VERIFIED gate is not a review verdict: the `legacy-six-group-regression`
  criterion passed over the regression D1-F2 finds incomplete, and no criterion
  covers the consumer D1-F1 breaks. The repair re-records evidence against the
  new state.
- Verdict: REOPEN

## Round 1 repair — 2026-08-26

- Repaired state: HEAD `1db02b8fd44b7f128d40fcc6aa7023b6cab0be7d` plus
  scoped manifest `sha256:283657c991ef4d643e8bec1e87841433d483b51b34ea19c1e8e843e912ee16a9`.
  The manifest is SHA-256 over newline-terminated
  `<repository-relative-path>\t<git-blob-hash>\n` rows, in this exact
  lexicographic order: `cmd/agentdeck/main_test.go`,
  `cmd/agentdeck/session_viewer_data_test.go`,
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json`,
  `cmd/agentdeck/usage_family_text.go`,
  `desktop/fixtures/v1/snapshot-complete.json`,
  `desktop/fixtures/v1/snapshot-empty-client.json`, `docs/status.md`,
  `docs/topics/usage-attribution-precision/tasks.md`,
  `internal/usage/presentation.go`, `internal/usage/presentation_test.go`,
  `internal/usage/routes.go`, `internal/usage/routes_test.go`,
  `internal/usage/usage.go`, and `internal/usage/usage_test.go`.
- Repairer: Codex, single-agent bounded Repair; no external implementation or
  scoring skill was used.
- Authorized scope: D1-F1, D1-F2, D1-F3, D1-F4, and D1-F5 only.
- Disposition:
  - D1-F1 — CLOSED. `usage summary` now renders
    `UNATTRIBUTED ATTRIBUTION` from `Counts["unattributed"]`; the focused CLI
    regression supplies one unattributed event and asserts that the rendered
    row ends in the non-zero count `1`.
  - D1-F2 — CLOSED. The six-group regression derives both the total and exact
    `ConfigChange` event counts from each real `routeQuality` verdict, and
    asserts 2,442 classifier-exact events. A blanket downgrade now fails the
    proof instead of only contradicting its message.
  - D1-F3 — CLOSED. The dual-resolver live-store table adds a keyed timeline
    whose no-prior route moves away from that key, plus a client with no
    timeline row; both read paths retain `estimated`.
  - D1-F4 — CLOSED. The conservative Codex restriction remains unchanged and
    now states why: Codex applies configuration only at process start, so a
    mid-session `ConfigChange` cannot be an adopted boundary.
  - D1-F5 — CLOSED. The exact-run branch assigns `spendEligible = true` after
    `exactRunRoute` has already rejected an unknown provider.
- Out of scope, not changed: task 3's attribution-reason object/text section,
  real-spend total separation, living CLI specifications, persisted route
  writers or schema, canonical desktop fixture contents beyond the existing
  task candidate, and unrelated dirty worktree changes.
- Verification: focused CLI renderer and three attribution classifier/exact-run
  tests pass; `scripts/run-go-test.sh ./...` passes; `go vet -mod=vendor
  ./internal/usage ./cmd/agentdeck` passes; `bash scripts/check-topic-docs.sh`,
  `make check-whitespace`, and `git diff --check` pass on the final repair
  content.
- Verdict: REOPEN — D1-F1 through D1-F5 repair complete, awaiting independent
  Re-review.

## Round 2 — 2026-08-26

- Reviewed state: HEAD `1db02b8fd44b7f128d40fcc6aa7023b6cab0be7d` plus the
  scoped manifest the repair declared,
  `sha256:283657c991ef4d643e8bec1e87841433d483b51b34ea19c1e8e843e912ee16a9`,
  recomputed independently from the working tree with the recipe and file order
  the repair states. It reproduced exactly, so the state judged below is the
  state the repair bound its evidence to — the limitation Round 1 recorded
  against the previous, under-specified recipe does not carry forward.
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`, no external skill. Each
  finding was checked against the source its disposition names rather than
  against the disposition text, then the repair was checked for consequences it
  introduces of its own. Every verification command was re-run rather than
  taken from the repair's report.
- Scope: D1-F1 through D1-F5, and any newly blocking repair regression.
- Findings:
  - **D1-F1 — CLOSED.** `cmd/agentdeck/usage_family_text.go:57` now reads
    `value.Counts["unattributed"]` under the label `UNATTRIBUTED ATTRIBUTION`,
    and `rg '"historical"'` over `internal`, `cmd`, `desktop` and `apps` returns
    no live reader of the retired key. The row is now covered:
    `TestUsageSummaryAndSessionsUseSharedTerminalPrimitives`
    (`cmd/agentdeck/main_test.go`) renders a summary carrying one unattributed
    event, requires the label to appear, and then splits that rendered line and
    asserts its last field is `1`. The assertion is real in both directions — it
    fails on the label if the row is dropped, and on the count if the row is
    wired back to a dead key. The three attribution rows sum to `EVENTS` again.
  - **D1-F2 — CLOSED.** `TestRouteQualityMatchesLegacySixGroupReference` now
    accumulates `exactConfigChangeEvents` inside the `got == "exact"` branch and
    asserts it equals 2,442, so the count is a function of the classifier's
    verdicts rather than of the test's own constants. The blanket downgrade the
    finding named is now a measurable failure: grading every `ConfigChange` row
    `estimated` drives that sum to zero and fails the assertion, where before it
    left every assertion intact. The group table also stopped carrying a
    separate `configChange` flag and derives the denominator from
    `group.hookEvent`, removing the second place that fact was stated.
  - **D1-F3 — CLOSED.** `TestAttributionResolversClassifyRouteEffectFromPriorState`
    gains a second Claude selection at `t0+10s` and a `no-prior-keyed` session
    whose only route is a `ConfigChange` to `official` positioned after it. Its
    session start resolves the timeline to the keyed `custom` provider, so the
    route records a move away from an already-keyed prior state and both
    resolvers hold it at `estimated`. That is the branch Round 1 named, driven
    through the real timeline fallback rather than through an injected prior.
  - **D1-F4 — CLOSED.** The Codex restriction now carries its reason on the
    line that implements it: configuration is applied only when a process
    starts, so a mid-session `ConfigChange` cannot be an adopted boundary.
  - **D1-F5 — CLOSED.** The exact-run branch assigns `spendEligible = true`
    directly, `exactRunRoute` having already rejected an unknown provider.
- Verified, not findings:
  - The `no-timeline` case the repair added alongside `no-prior-keyed` is a
    Codex session, where `routeQuality` returns `estimated` from the client
    restriction before the prior state is consulted, so its verdict is
    over-determined and it does not distinguish the unresolvable-prior rule. It
    is not empty: the case does drive a `ConfigChange` with no prior route
    through `snapshotAt` returning `sql.ErrNoRows`, proving the resolver
    swallows that error instead of failing the read. What remains uncovered is
    the same rule on a Claude route, and it is doubly guarded there — `priorFound`
    is false exactly when `priorProvider` is still `"unknown"`, and either
    condition alone yields `estimated`. Recorded as accepted rather than as a
    finding: D1-F3 was a nit, its named branch is closed, and the residue cannot
    change a verdict.
  - `cmd/agentdeck/session_show_text_test.go:118,122,132` still uses the string
    `historical attribution` as a session warning. It is not a D1-F1 recurrence:
    that finding was a producer-coupled read of a key no producer writes, while
    this is an opaque literal in a hand-built fixture that the renderer passes
    through unchanged, so it asserts nothing about the quality vocabulary and
    produces no wrong number. It is stale wording, and belongs with task 3's CLI
    vocabulary reconciliation rather than here.
  - The repair introduces no production change beyond the renderer key, the
    `routeQuality` comment, and the `spendEligible` assignment; `git diff` on
    `internal/usage` and `cmd/agentdeck` shows nothing else outside tests.
- Evidence: the repair manifest recomputed to
  `283657c991ef4d643e8bec1e87841433d483b51b34ea19c1e8e843e912ee16a9` from the
  fourteen declared files. `go test ./internal/usage/... ./internal/desktop/...
  ./cmd/agentdeck/...` and `./scripts/run-go-test.sh ./...` pass. Canonical
  fixture reproducibility re-confirmed: `shasum` of `desktop/fixtures/v1/*.json`
  is unchanged after `AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop`.
  `go vet` on `./internal/usage/...` and `./cmd/agentdeck/...` is clean.
  `bash scripts/check-topic-docs.sh` and `bash scripts/check-whitespace.sh` pass.
  `gofmt -l` reports only `cmd/agentdeck/usage_stats_viewer_test.go`, unmodified
  by this task and already unformatted at HEAD, as recorded in Round 1.
  The Round 1 classification checks were re-run unchanged and still hold.
- Completion gate: VERIFIED — the WorkUnit
  `usage-attribution-precision:determinability-quality` was queried
  independently and returns `pass` on all seven criteria, the six from Round 1
  plus `summary-text-quality-vocabulary`, which the repair added for D1-F1.
  Evidence for this round is re-recorded against the post-synchronization state
  of this round and supersedes the repair-round records, so one live record
  describes the state the PASS was given on.
- Verdict: PASS
