---
status: active
plan: usage-pricing-read-scalability
task: usage-sessions-batch
---

# Review log — usage-pricing-read-scalability / usage-sessions-batch

## Round 1 — 2026-08-02
- Reviewed state: worktree on `24b9e56`; `internal/usage/usage.go` blob
  `5386a2dc3b5634651af134ae8d84e6ff68581ff0`, `internal/usage/usage_test.go`
  blob `65daeea89395d56a153c42cdee56c731e092d57b` (uncommitted, still carrying
  the reviewed Task 2 content).
- Reviewer: Claude Opus 5 (independent round, no product code changed).
- Scope: reviewed as the delta from the Task 2 blobs — the `sessionStartAt`
  fallback, the `Sessions` rewrite, the `s.events()` column additions, the
  `summarizeEvents`/`eventPricing` extraction, and the two new tests. Checked
  ordering, grouping, output-field preservation, and which callers the changed
  attribution semantics actually reach.
- Findings:
  - **Correction to the Task 2 review.** That round's P2 asserted that moving
    `Sessions` onto the resolver without the session-start fallback would change
    `usage sessions` cost numbers. That premise was wrong: `Sessions` derives its
    rows from `usage_sessions`, so every event it summarizes necessarily joins a
    session row and `us.first_at` is never empty on that path. The fallback was
    therefore not required to keep `usage sessions` byte-identical. This changes
    the justification recorded in the plan's Dev evidence, not the code.
  - The `Sessions` rewrite is correct. Metadata keeps its
    `first_at DESC, client, session_id` ordering, events keep their
    `event_at, event_key` order inside each in-memory group, the summary fields
    and the nil-vs-known cost distinction are copied through unchanged, and the
    per-session query and the nested cursor it held open are both gone.
  - [P2 — accepted, unreachable on normal data] The fallback's real effect is on
    `Stats` and `PriceDiagnostics`, not `Sessions`, because those read events
    that are not constrained to `usage_sessions` rows. Where a session row is
    missing, an event that previously resolved as `historical` / `1` /
    `unknown` now resolves through the provider snapshot at its own `EventAt` —
    a user-visible provider grouping and cost change in `usage stats`, which
    would be a MINOR trigger. It is judged safe because `rebuildSessions`
    deletes and reinserts `usage_sessions` from `usage_events` inside the same
    transaction as the events themselves, in the same database, so the missing
    row is not a reachable state for imported data. Two points make it worth
    keeping on record anyway: it is a runtime invariant rather than an enforced
    one, and no test pins it. Task 4's real-database acceptance run is the
    natural place to confirm `usage stats` output is unchanged.
  - Aligning the resolver with legacy `(*Service).priceForEvent` is still the
    right call on its own merits — the two pricing paths now agree clause for
    clause, which is what lets the legacy one retire later.
  - [nit] The plan's Task 3 Dev evidence states the change "closed the prior
    review's P2". Per the correction above, the accurate statement is that it
    aligned the resolver with legacy attribution semantics; `usage sessions` was
    never exposed to the difference.
  - [nit] Bounded queries, unbounded memory: `Sessions` now holds every event in
    the store at once. That matches what `PriceDiagnostics` and `Summary`
    already do, and the plan's Non-Goals forbid pagination, so it is in scope —
    but the 93,982,720-byte acceptance database is where it should be measured.
  - [nit] `Summary` and `SummaryRange` still call `s.summarize` on the legacy
    per-event path, so two pricing algorithms remain in the tree against the
    plan's third Goal. Out of this task's scope; nothing in the plan currently
    owns retiring them.
  - The `s.events()` additions of `r.provider` and `r.started_at` close the
    Task 2 nit — every resolver caller now supplies complete attribution inputs.
  - Both new tests are real regression gates.
    `TestReadPriceResolverFallsBackToEventAtWithoutSessionStart` fails with
    `provider: unknown` / `quality: historical` if the fallback is removed.
    `TestSessionsRetainsSessionStartProviderCost` pins the estimated multiplier
    reaching provider cost (`5.000000000`) and the `estimated attribution`
    warning, so it fails if `Sessions` loses attribution when grouping in
    memory.
- Evidence:
  - `go test -mod=vendor -count=1 ./internal/usage ./internal/doctor ./cmd/agentdeck` → ok.
  - `go test -mod=vendor ./...` → all packages ok (L2 route).
  - `go vet -mod=vendor ./...` → clean.
  - All commands run with `GOCACHE=/private/tmp/agent-deck-go-build`.
- Verdict: PASS
