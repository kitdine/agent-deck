---
status: active
created: 2026-07-22
plan: test-coverage
task: store-boundaries
---

# Review log — test-coverage / store-boundaries

## Round 1 — 2026-07-22

- Reviewed state: `b6a13632b50a6119194b351fbb48b04b1a2dc44a` plus the
  uncommitted `internal/store/store_test.go`
  (`474f8b827560e5112621a7b72fbedc724f267d5142194bacfe2e4dc01ff84148`).
- Reviewer: Codex.
- Scope: task 1 test additions in `internal/store/store_test.go` for
  `OpenReadOnly`, `BackupSQLiteFile`, `IntegrityCheck`, and `AcquireScanLock`,
  against their implementations in `internal/store/store.go`. The concurrent
  task 2 changes in `internal/store/providers_test.go` were out of scope.
- Findings:
  - [P2] `TestOpenReadOnlyCompatibleDBSkipsMigrationAndDoesNotCreateWriteSidecars`
    does not assert that `OpenReadOnly` preserves the existing database file
    mode. The documented contract at `internal/store/store.go:151-152` says a
    read-only open does not change permissions. A regression that calls
    `secureFiles` or otherwise chmods the database would still pass the added
    no-migration and no-sidecar assertions. Extend the temporary v6 fixture
    test to set and record a readable non-default mode, call and close
    `OpenReadOnly`, then assert the exact original mode is retained. No
    production change is required.
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store -count=1` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./internal/store -count=1` — PASS.
  - `rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./internal/store` — PASS.
- The reviewer did not rerun a full-repository suite because task 2 has
  separate, unreviewed changes in the same working tree.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: `b6a13632b50a6119194b351fbb48b04b1a2dc44a` plus the
  uncommitted `internal/store/store_test.go`
  (`93621e3dfcb6ac94093026349b7b6ca0f64f5c78d8ce2accd66769cda6741511`).
- Reviewer: Codex.
- Scope: Round 1 P2 finding in `TestOpenReadOnlyCompatibleDBSkipsMigrationAndDoesNotCreateWriteSidecars`.
- Findings:
  - [P2] Resolved: the test now records and enforces the pre-open file mode for
    the v6 fixture database, then closes `Store` and asserts the mode remains
    unchanged.
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...` — PASS.
  - `rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` — PASS.
  - `rtk git diff --check` — PASS.
- Verdict: PASS
