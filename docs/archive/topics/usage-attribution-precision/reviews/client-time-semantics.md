---
status: historical
topic: usage-attribution-precision
subject: client-time-semantics
retired: 2026-09-01
---

# Review log — usage-attribution-precision / client-time-semantics

## Round 1 — 2026-08-26

- Reviewed state: HEAD `944bc86481660b824da8c0094cbfd11770599ace` plus
  implementation diff fingerprint
  `eb447b5eec720263a112aa6a36a289f5d03bdaecb6efa772befef8ad5fcfc94f`, taken as
  `git diff HEAD -- internal/usage/usage.go internal/usage/usage_test.go` piped
  to `shasum -a 256`. Working `tasks.md` blob is
  `5d643bd8596bf07ff4f1b9a5d2b2096ce01bd6e6`; `docs/status.md` blob is
  `29f26458417d5c2d9a4e39733428e388d6ae912b`.
- Reviewer: claude-code, independently reviewing the implementation authored by
  `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal code-and-tests Review under `development-workflow`. The shared
  helper was compared against both prior branch bodies statement by statement,
  the two route lookups it delegates to were read in full, and the ordering
  claim the new code documents was verified empirically rather than by
  inspection alone.
- Scope: the shared time-positioning policy across both read paths, Codex
  restart-only semantics, Claude effective-route positioning by event time, the
  session-start fallback, and the task's required parity coverage.
- Findings:
  - **[P1] C1-F1 — the ordering hazard is fixed on one read path and left in
    place on the other, which is the divergence this task exists to remove.**
    `storedSessionRouteAt` (`internal/usage/usage.go:2612-2650`) is introduced
    with an explicit rationale: "RFC3339Nano strings are not lexically ordered
    when one timestamp has fractional seconds and the other does not, so compare
    parsed instants and use id only as the equal-instant tie breaker." That is
    correct, and the `Service.priceForEvent` path now positions routes by parsed
    instant. The `readPriceResolver` path does not. Its routes are loaded by
    `loadReadPriceResolver` with `ORDER BY client,session_id,observed_at,id`
    (`:2409`) — a SQL string comparison of exactly the values the new comment
    says must not be compared as strings — appended to the per-session slice in
    that order, and never re-sorted. `readPriceResolver.sessionRouteAt` (`:2504`)
    then runs `sort.Search` over that slice, which requires sorted input.
    The two paths therefore disagree under the triggering condition. With routes
    at `…:01Z` (provider A) and `…:01.5Z` (provider B) in one session, the
    loaded slice is `[01.5Z(B), 01Z(A)]`; for an event at `…:02Z` no element is
    `After` the event, so `sort.Search` returns `len`, and `routes[index-1]`
    yields A, while the correct latest-route-at-or-before is B — which is what
    `storedSessionRouteAt` returns. The same event resolves to a different
    provider and multiplier depending on whether the command went through
    `usage summary` or through Stats/Sessions/desktop `Presentation`.
    Trigger conditions, stated honestly: the mis-order needs one fractional
    string to be a prefix of another, such as `.1` against `.15`, or a
    whole-second value against a fractional one in the same session. Production
    timestamps come from `s.now().Format(time.RFC3339Nano)`, so a uniform clock
    granularity produces uniform-length fractions and is safe; the hazard needs
    mixed lengths, which round values and fixtures produce readily. The
    probability is low, but `sort.Search` over unsorted input is undefined
    regardless of how the input got that way, and this task's single deliverable
    is that both paths follow one policy.
    The bounded fix is on the loading side, not the query: sort each session's
    slice by parsed `observedAt` with `id` as the equal-instant tie breaker after
    the rows are read, so the in-memory path matches the tie-breaking
    `storedSessionRouteAt` already implements.
  - **[P2] C1-F2 — the superseded string-comparison lookup is left in place with
    no callers.** `(s *Service) sessionRouteAt` (`internal/usage/routes.go:421`)
    still selects `ORDER BY observed_at DESC,id DESC LIMIT 1` over the same
    column, and now has zero callers anywhere in the repository, tests included.
    `go vet` does not report an unused method, so nothing prevents a later change
    from reaching for it and reintroducing the defect C1-F1 documents. Task 1's
    `Files` list does not include `routes.go` and the architecture assigns that
    file's read side to task 2, so removing it may belong there; either way the
    dead function should not be left as an attractive nuisance without a note.
- Verified, not findings:
  - The task's required behaviors are covered and asserted against both
    resolvers by `TestAttributionResolversShareClientTimeSemantics`
    (`internal/usage/usage_test.go:1469`): all three Claude transitions
    (`no key -> first key`, `key A -> key B`, `key -> no key`), restart, a Codex
    session spanning a global switch, and a timeline fallback that stays at
    session start. Every case asserts the read resolver and the service resolver
    against the same expectation, which is the right shape for a parity test.
  - Replacing the Service path's direct `usage_sessions.first_at` query with
    `sessionStartAt(event)` preserves behavior and removes a per-event query:
    `sessionStartAt` (`:2583-2589`) falls back to `event.EventAt` exactly as the
    removed code did, and both `events` and `eventsRange` (`:3129`, `:3164`)
    already `LEFT JOIN usage_sessions us` and select `us.first_at`, so
    `storedEvent.sessionStart` is populated on that path.
  - The prerequisite holds: `switch-effectiveness-boundary` tasks 1 and 3 are
    committed at `8703fed` and `7db5618` with Review PASS.
  - The persisted route-quality writer is untouched; `recordSessionRouteConn`
    and `usage_session_routes.quality` are unchanged, and the helper preserves
    the stored quality rather than promoting it, correctly leaving that to
    task 2.
- Evidence: the ordering claim was verified rather than reasoned about —
  `sqlite3 :memory:` ordering `'2026-08-04T00:00:00.5Z'`,
  `'2026-08-04T00:00:01Z'`, `'2026-08-04T00:00:01.5Z'` by `observed_at, id`
  returns `00.5Z, 01.5Z, 01Z`, placing `01.5Z` before `01Z`. The parity test's
  own route timestamps are whole seconds within the Claude session (`t0`, `+1s`,
  `+2s`, `+3s`, `+4s`) and the Codex session holds one route at `+100ms` plus one
  at `+4s`, whose strings happen to order correctly, so the suite does not
  exercise the hazard and passes with the defect present.
  `go build -mod=vendor ./...` and `go vet -mod=vendor ./...` are clean;
  `go test -mod=vendor ./internal/usage/... -count=1` passes, including the new
  test. Broad verification stopped after the decisive parity blocker; the full
  repository suite should be run on the final repaired state.
- Completion gate: FAILED — the task's parity criterion is disproved by C1-F1;
  no completion evidence is recorded for a failing round.
- Verdict: REOPEN

### Repair disposition — 2026-08-26

- C1-F1 closed: `loadReadPriceResolver` now retains route IDs and sorts every
  session slice by parsed `observedAt`, then ID for equal instants, before
  `sort.Search`. A focused regression mixes whole-second and unequal fractional
  RFC3339 values and asserts both resolvers choose the same route, including the
  equal-instant later-ID rule.
- C1-F2 closed: the zero-caller string-comparison helper is explicitly marked
  deprecated and forbidden for pricing reuse; task 2's already-approved
  read-side route metadata work remains the owner of its removal or replacement.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

### Repair disposition — 2026-08-26

- C1-F1 repaired: `loadReadPriceResolver` no longer orders routes by the
  `observed_at` string. It selects `id` as well, orders only by
  `client,session_id,id` for grouping, and sorts each session's slice by parsed
  `observedAt` with `id` ascending as the equal-instant tie breaker.
- C1-F2 repaired: `(s *Service) sessionRouteAt` carries a `Deprecated:` note
  stating that it compares raw RFC3339 strings, has no callers, and that task 2
  owns its removal.
- New regression `TestAttributionResolversSortMixedRFC3339RouteInstants` covers
  the mixed-precision ordering and the equal-instant tie break against both
  resolvers.

## Round 2 — 2026-08-26

- Reviewed state: HEAD `944bc86481660b824da8c0094cbfd11770599ace` plus repaired
  implementation diff fingerprint
  `b4b65deb401a67eedf2f15342a141e367eb863e9491a4ab10a81ec2a80650c1d`,
  reproducible as `git diff HEAD -- internal/usage/usage.go
  internal/usage/usage_test.go internal/usage/routes.go` piped to
  `shasum -a 256`.
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; each finding checked
  against the repaired source, the tie-breaking of the two lookups compared
  directly, and the new regression checked for discriminating power rather than
  for merely passing.
- Scope: C1-F1 and C1-F2, and any newly blocking repair regression.
- Findings:
  - **C1-F1 — CLOSED.** The ordering hazard is now handled on the loading side,
    which is where the finding placed it. `loadReadPriceResolver` selects `id`,
    drops `observed_at` from the SQL `ORDER BY` so no string comparison decides
    order, and sorts each session's slice by parsed `observedAt` with `id`
    ascending on equal instants. Tie-breaking now agrees across both paths:
    `readPriceResolver.sessionRouteAt` returns `routes[index-1]`, the last
    element not after the event, which under an ascending `id` sort is the
    highest `id` among equal instants — the same row `storedSessionRouteAt`
    selects through `id > selectedID`.
  - **C1-F2 — CLOSED.** The superseded lookup is documented rather than left
    bare: the `Deprecated:` note states that it compares raw RFC3339 strings,
    that it has no callers, that task 2's read-side work owns its removal, and
    that it must not be reused. Round 1 asked for removal or a note, and the
    note is the correct half here, since `routes.go` is outside this task's
    file boundary.
- Verified, not findings:
  - The new regression discriminates rather than merely passing. Its fixture
    holds routes at `00.5Z`, `01Z`, `01.5Z` and two at `02Z`. For an event at
    `01.25Z` the correct answer is the `01Z` route; under the Round 1 loading
    order the slice would have been `[00.5Z, 01.5Z, 01Z]`, so `sort.Search`
    would find `01.5Z` as the first element after the event, return index 1, and
    yield `routes[0]` — the `00.5Z` route. The test asserts the `01Z` provider,
    so it fails on the pre-repair code. The second case pins the equal-instant
    tie break to the higher `id`. Both cases assert the read resolver and the
    service resolver against the same expectation.
  - The Round 1 parity coverage is retained unchanged: all three Claude
    transitions, restart, a Codex session spanning a global switch, and the
    timeline fallback positioned at session start.
  - The persisted route-quality writer is still untouched, and the helper still
    preserves stored quality rather than promoting it, leaving that to task 2.
- Evidence: the broad verification Round 1 deferred was run on this final state.
  `go build -mod=vendor ./...` and `go vet -mod=vendor ./...` are clean;
  `scripts/run-go-test.sh ./...` passes for the whole repository (exit 0; log
  `/var/folders/x1/pbx8jlln5lb46wtp8_nq0khh0000gn/T/agentdeck-go-test.fX0Y5R`),
  which is this task's L2 gate. `go test -mod=vendor -race ./internal/usage/...`
  passes in 52.4s. `gofmt -l internal/usage/` reports nothing,
  `make check-whitespace` and `git diff --check` are clean.
- Completion gate: VERIFIED — CEv1 gate
  `usage-attribution-precision:client-time-semantics` records this PASS against
  exact content state
  `b4b65deb401a67eedf2f15342a141e367eb863e9491a4ab10a81ec2a80650c1d`.
- Verdict: PASS
