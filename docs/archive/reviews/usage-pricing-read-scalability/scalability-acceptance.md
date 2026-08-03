---
status: historical
plan: usage-pricing-read-scalability
task: scalability-acceptance
retired: 2026-08-03
---

# Review log — usage-pricing-read-scalability / scalability-acceptance

## Round 1 — 2026-08-02

- Reviewed state: worktree on `24b9e56`; `internal/usage/usage.go` blob
  `5386a2dc3b5634651af134ae8d84e6ff68581ff0`,
  `internal/usage/usage_test.go` blob
  `a9d1e682f714f02ca2a0178f35a2e14527cdcd77` (uncommitted, carrying the
  already reviewed Tasks 2 and 3 content).
- Reviewer: Codex (independent round, no product code changed).
- Scope: `TestPriceReadPathsKeepQueryCountConstantForLargeFixture`, the shared
  query-counting driver it uses, the current `PriceDiagnostics` and `Sessions`
  call paths, and the Task 4 acceptance evidence in the plan and
  `docs/README.md`.
- Findings:
  - No P1/P2. The 1,003-event, two-session fixture asserts the exact five-query
    bound for both read paths and validates returned diagnostic counts, session
    count, ordering, and token totals. The counting driver covers direct and
    prepared queries, while `QueryRowContext` reaches the driver's query path.
  - The five-query result matches the implementation: each caller performs its
    own metadata/provenance query and the shared event query, then the resolver
    loads operation, selection, and price snapshots once. Event and session
    cardinality cannot add database reads inside the in-memory loops.
  - [nit] The plan's Dev evidence explains five SELECTs as “one caller-specific
    read plus the resolver's operation, selection, and price snapshots,” which
    names only four reads and omits the shared event SELECT. The asserted count
    and implementation are correct; this is only an explanatory omission.
  - Real-state evidence remains bound to the named database: current size is
    93,982,720 bytes, mode is `0600`, and SHA-256 remains
    `2763ef03f3fff49f79955fe78eb19e31c96323fcb2b88a5fc15ffc024dd8624b`.
    Independent current-source read-only runs returned exit 0 for quick doctor
    with 12 checks and full doctor with 18 checks; full included
    `price_provenance` and `unpriced_models`. Existing environment warnings made
    both reports degraded but produced no errors and do not contradict the
    acceptance claim.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run '^TestPriceReadPathsKeepQueryCountConstantForLargeFixture$' ./internal/usage`
    → PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./...`
    → PASS (L2 route).
  - Current-source `doctor` and `doctor --full` against
    `/Users/jobshen/.agentdeck` through the CLI's read-only doctor path → both
    exit 0, with 12 and 18 checks respectively.
  - `git diff --check` → PASS.
- Verdict: PASS
