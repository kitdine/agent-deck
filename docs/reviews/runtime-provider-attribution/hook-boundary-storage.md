---
status: active
plan: runtime-provider-attribution
task: hook-boundary-storage
---

# Review log — runtime-provider-attribution / hook-boundary-storage

## Round 1 — 2026-08-03

- Reviewed state: worktree on `192e969ef689a7d7d5e68dc0bfb7c8b43f5b274e`; reviewed product/test file aggregate SHA-256 `595888a49dfea938595ff5255a5e0bc70e5c634cb71e21c27e2b86ac4d969a04` for `cmd/agentdeck/main.go`, `internal/store/migrations.go`, `internal/store/store.go`, `internal/usage/usage.go`, `internal/usage/routes.go`, `internal/usagehook/event.go`, and `internal/store/routes_test.go`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: schema 17 migration, managed same-client run overlap, strict Hook event parsing, idempotent session-route persistence, route lookup during usage pricing, CLI fail-open event delivery, and task acceptance coverage.

### Findings

**P1 — schema 12 migration fails because migration 17 unconditionally drops an index absent from the supported fixture.**

`internal/store/migrations.go:106` executes `DROP INDEX one_active_usage_run_per_client`. The existing `TestStateMigrateTextAndJSONUpgradeSchema12` fixture does not contain that index, so the supported state migration exits with `migration 17: SQL logic error: no such index: one_active_usage_run_per_client (1)`. This blocks the required full suite and can block upgrade for a database whose earlier schema is otherwise accepted by `state migrate`. Reconcile the migration with supported historical states (for example, use an existence-safe drop if the missing-index state is valid) and retain the schema-12 regression test.

**P1 — global semantic-key uniqueness drops a later legitimate switch back to a previous provider.**

`internal/usage/routes.go:37-38` builds the key from client, session, event name, source, provider, multiplier, and wrapper state, while `internal/store/migrations.go:107` makes that key globally `UNIQUE` and the writer uses `INSERT OR IGNORE`. For one resumed session routed `official -> provider-b -> official`, the second `official` resume has the same key as the first and is ignored. Events after that resume still resolve the newer `provider-b` row in `sessionRouteAt`, producing wrong attribution. Idempotence must suppress duplicate delivery of one boundary without suppressing a later equal boundary after an intervening route.

**P1 — every Claude `ConfigChange` currently creates a provider boundary without path or settings reconciliation.**

`internal/usagehook/event.go:80-82` accepts any Claude `ConfigChange`; `cmd/agentdeck/main.go:2695-2708` forwards only its client/session/name/source; and `RecordSessionRoute` skips only `compact` and `SessionEnd`. Therefore project, local, policy, skills, and unrelated user-setting changes all snapshot the current AgentDeck selection as a new route. This violates the plan rule that only the managed user settings path may form this boundary and preempts task 3's required matched/unmatched reconciliation. Until that logic exists, task 2 must not persist generic `ConfigChange` events.

**P1 — the strict parser never validates `transcript_path`, so arbitrary session IDs can receive routes.**

The plan requires `transcript_path` to be used only to validate that an event belongs to the declared client session. `ParseEvent` reads only `session_id`, `hook_event_name`, and `source`, and `runUsageHookEvent` writes the resulting session ID directly. A malformed or manually invoked hidden handler can therefore poison attribution for another logical session while still passing validation. Validate the client/session relationship without persisting or reporting the path, and cover mismatches as write-nothing cases.

**P2 — acceptance tests exercise schema shape, not the new behavior.**

`internal/store/routes_test.go` directly inserts one route and two already-estimated `usage_runs` rows. No test calls `RecordSessionRoute`, proves duplicate-delivery idempotence versus a later equal boundary, proves pre/post-boundary pricing, verifies invalid Hook input writes nothing end to end, or starts a second active same-client run through `StartRun` and checks both rows are downgraded. The task's four acceptance criteria therefore remain unprotected; the defects above pass all three currently green target packages.

### Evidence

- Full-context review of the tracked diff plus complete reads of new `internal/usage/routes.go` and `internal/store/routes_test.go`; CodeGraph call-path review for `ParseEvent`, `RecordSessionRoute`, `sessionRouteAt`, `priceForEvent`, `StartRun`, and `CurrentProviderSnapshot`.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/store ./internal/usage ./internal/usagehook ./cmd/agentdeck` -> **FAIL**: `TestStateMigrateTextAndJSONUpgradeSchema12`; `internal/store`, `internal/usage`, and `internal/usagehook` pass.
- Broader suite, race, and vet intentionally not run after decisive blocking findings.

- Verdict: REOPEN

## Round 2 — 2026-08-04 (re-review)

- Reviewed state: worktree on `192e969ef689a7d7d5e68dc0bfb7c8b43f5b274e`; reviewed product/test file aggregate SHA-256 `5df4da8c98316d2dfc8fc7d6b726262d6c8b1fc5aab17b1f8079486010fe5a39` for `cmd/agentdeck/main.go`, `cmd/agentdeck/hook_boundary_test.go`, `internal/store/migrations.go`, `internal/store/store.go`, `internal/store/routes_test.go`, `internal/usage/usage.go`, `internal/usage/routes.go`, `internal/usage/routes_test.go`, and `internal/usagehook/event.go`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: closure of all Round 1 findings, new route-resolution read path, regression-test value, and the plan's concurrent-delivery/idempotence boundary.

### Finding closure

- **P1 (schema 12 migration failure) — CLOSED.** Migration 17 now uses `DROP INDEX IF EXISTS`; the target `cmd/agentdeck` package, including `TestStateMigrateTextAndJSONUpgradeSchema12`, passes in the re-review target run.
- **P1 (later switch back to a previous provider ignored) — PARTIALLY CLOSED.** The global unique constraint and `INSERT OR IGNORE` were removed, and `TestSessionRoutesAreIdempotentButKeepLaterReturnBoundary` proves sequential `official -> b -> official` persistence. The replacement deduplication has the two P1 defects below.
- **P1 (generic Claude `ConfigChange` writes a route) — CLOSED for task 2.** `RecordSessionRoute` now persists only `SessionStart`; task 3 remains responsible for validated and reconciled `ConfigChange` boundaries.
- **P1 (`transcript_path` not validated) — CLOSED for the current client filename/root contract.** Session-start delivery now requires a regular non-symlink transcript under the client's resolved session root whose basename contains the declared session ID. CLI coverage proves outside-root, mismatched-name, and symlink inputs write nothing while a valid input writes one route.
- **P2 (acceptance tests only exercise schema shape) — PARTIALLY CLOSED.** New tests cover sequential duplicate delivery, provider switch-back, managed overlap through `StartRun`, pre/post-boundary resolver behavior, invalid Hook delivery, and generic non-boundary events. They do not cover concurrent duplicate delivery or wrapper-only route changes.

### Findings

**P1 — concurrent duplicate Hook delivery is no longer idempotent.**

`internal/usage/routes.go:39-47` performs an autocommit `SELECT` of the latest route followed by a separate `INSERT`. Migration 17 no longer has any unique constraint on `semantic_key`, and `Store.Open` releases its state lock before returning while `Store.Exec` protects only one statement. Two Hook processes can therefore both observe the same previous row (or no row) and then insert the same semantic boundary serially. This violates the plan's explicit failure-safety rule, “Concurrent duplicate Hook delivery is idempotent.” The test at `internal/usage/routes_test.go:13-31` invokes the writer twice sequentially and cannot expose the race. Make the latest-row comparison and insert one atomic persistence decision, and add a deterministic barrier test that forces both deliveries through the vulnerable window.

**P1 — wrapper-only provider route changes are silently discarded.**

The snapshot key includes `ViaWrapper`, but `internal/usage/routes.go:39-41` reads and compares only provider, multiplier, hook event, and source. If a session resumes after the same provider/multiplier changes from direct to wrapped or wrapped to direct, `RecordSessionRoute` returns without inserting the boundary. Later events retain the prior `via_wrapper` attribution even though the new runtime route differs. Include `via_wrapper` in the adjacent-boundary equality decision and add direct-to-wrapper and wrapper-to-direct coverage.

**P1 — the required targeted verification is still red.**

`go test` for the four affected packages exits 1 in both the normal RTK run and the single unfiltered fallback. The unfiltered stream confirms `cmd/agentdeck` passes, but the outer CCR layer replaced the earlier non-command failure details with an opaque reference, so the failing package/test cannot be truthfully identified from retained output. A PASS verdict requires a fresh green targeted run after the product findings are fixed; capture structured/raw output on that run if failure detail is needed.

### Evidence

- Full-context review of the current tracked diff and complete reads of the three new test files plus `internal/usage/routes.go`; CodeGraph inspection of route attribution callers and store lock lifetime.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/store ./internal/usage ./internal/usagehook ./cmd/agentdeck` -> **FAIL**; `cmd/agentdeck` explicitly reports PASS in the unfiltered fallback, while the outer output layer hid the earlier failing test details.
- Broader suite, race, and vet intentionally not run after decisive P1 findings.

- Verdict: REOPEN

## Round 3 — 2026-08-04 (re-review)

- Reviewed state: worktree on `192e969ef689a7d7d5e68dc0bfb7c8b43f5b274e`; reviewed product/test file aggregate SHA-256 `35b29a2476d1e8dffe1b29fcd3732ff1993013420e170bfe91a295403daf94a6` for `cmd/agentdeck/main.go`, `cmd/agentdeck/hook_boundary_test.go`, `internal/store/migrations.go`, `internal/store/store.go`, `internal/store/routes_test.go`, `internal/usage/usage.go`, `internal/usage/routes.go`, `internal/usage/routes_test.go`, and `internal/usagehook/event.go`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: closure of the two Round 2 persistence findings and the required targeted verification gate.

### Finding closure

- **P1 (concurrent duplicate Hook delivery not idempotent) — CLOSED by source and regression coverage.** `RecordSessionRoute` now performs the latest-boundary equality check and conditional insert in one SQLite statement. `TestConcurrentDuplicateSessionRoutesInsertOneBoundary` uses two stores and a deterministic pre-write barrier so both deliveries reach the persistence decision together, then requires one stored boundary.
- **P1 (wrapper-only provider route changes discarded) — CLOSED.** The atomic equality decision now selects and compares `via_wrapper`. `TestSessionRoutesKeepWrapperOnlyChanges` pins the direct -> wrapped -> direct sequence as three boundaries.
- **P1 (targeted verification red) — NOT CLOSED.** The affected-package command still exits 1 in the current content state.

### Findings

**P1 — the required targeted verification gate remains red.**

`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/store ./internal/usage ./internal/usagehook ./cmd/agentdeck` exits 1. A single bounded fallback changed the reporter to `go test -json` and selected only fail/error events, but the outer CCR layer replaced the resulting block with an opaque reference; the retained output therefore does not support naming the failing package or test. Do not infer PASS from source closure or guess the failure. Diagnose the current target failure with one captured raw/structured producer run, fix the actual cause, and present a green target result before re-review.

### Evidence

- Full-context current diff review, complete reads of `internal/usage/routes.go` and its behavioral tests, and CodeGraph inspection of the persistence and attribution paths.
- Affected-package compact run -> **FAIL** (exit 1; RTK returned an opaque compressed reference).
- One `go test -json` bounded fallback selecting failure events -> producer still reports failure; outer CCR compressed the selected block, so exact failing-test text is unavailable.
- Full vendored suite, race, and vet intentionally not run because the targeted gate did not pass.

- Verdict: REOPEN

## Round 4 — 2026-08-04 (re-review)

- Reviewed state: worktree on `192e969ef689a7d7d5e68dc0bfb7c8b43f5b274e`; reviewed product/test file aggregate SHA-256 `1c935a16440ec48e438c2e76204ccda171177fda51401a7771a2c0eca81a4d4d` for `cmd/agentdeck/main.go`, `cmd/agentdeck/hook_boundary_test.go`, `internal/store/migrations.go`, `internal/store/store.go`, `internal/store/routes_test.go`, `internal/usage/usage.go`, `internal/usage/usage_test.go`, `internal/usage/routes.go`, `internal/usage/routes_test.go`, and `internal/usagehook/event.go`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: closure of the Round 3 verification failure, exact query-budget adjustment, all prior findings, and task acceptance criteria.

### Finding closure

- **P1 (targeted verification red) — CLOSED.** The new route resolver intentionally adds one bulk route query to each read path. The constant-query tests now expect six rather than five queries for both small and 1003-event fixtures, preserving the bounded-query contract while accounting for the new route source.
- All Round 1-3 product findings remain closed: schema-12 migration succeeds; provider return and wrapper-only transitions persist; concurrent duplicate delivery is idempotent; generic `ConfigChange` remains deferred to task 3; transcript validation rejects mismatched, outside-root, and symlink inputs; managed overlap downgrades both open runs; and pre/post-boundary attribution is covered.

### Findings

- No blocking, medium, or new regression finding.

### Evidence

- Full-context review of the current changed attribution, persistence, migration, parser, CLI, and behavioral-test paths; focused review of the query-budget change that closed Round 3.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/store ./internal/usage ./internal/usagehook ./cmd/agentdeck` -> PASS (`internal/store` 2.973s, `internal/usage` 7.876s, `internal/usagehook` 2.137s, `cmd/agentdeck` 33.544s).
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...` -> PASS (exit 0).

- Verdict: PASS
