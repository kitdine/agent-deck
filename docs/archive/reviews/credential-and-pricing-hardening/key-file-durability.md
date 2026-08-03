---
status: historical
plan: credential-and-pricing-hardening
task: key-file-durability
retired: 2026-08-03
---

# Review log — credential-and-pricing-hardening / key-file-durability

## Round 1 — 2026-08-03

- Reviewed state: worktree on `49bff2b`; `internal/credentialvault/vault.go`
  blob `a396c071a0d64470f1064de0fa3dc11f3439d5c2` and
  `internal/credentialvault/vault_test.go` blob
  `7da9ca498d41237013c936aae42c3127fd6b0297` (uncommitted).
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: the complete `key-file-durability` diff, including the unexported
  directory-sync seam, the post-link state-root sync, error propagation and
  recovery tests, existing concurrent initialization behavior, and the task's
  L3 verification contract.
- Findings:
  - [P2] A concurrent `createSeed` caller can return before any state-root
    directory sync completes. The successful linker waits in
    `syncStateRoot`, but another caller whose `os.Link` receives `fs.ErrExist`
    follows `createSeed` directly to `loadSeed` and can derive a key and return
    ciphertext while the winning caller is still blocked before directory
    durability. A crash in that window can therefore preserve ciphertext that
    depends on a key-directory entry that was never made durable. This violates
    the task requirement that the directory entry be durable before any caller
    can commit dependent ciphertext. The new tests cover the successful-link
    and sync-error paths, but not this existing-key concurrency path.
- Evidence:
  - `codegraph explore` confirmed the `Seal` -> `key` -> `createSeed` call path
    and the `fs.ErrExist` recovery branch.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./internal/credentialvault`
    -> PASS (`2.284s`).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./...`
    -> PASS (all packages).
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`
    -> PASS.
  - A temporary `/private/tmp` Go overlay test blocked the successful linker
    inside `syncStateRoot`, then started a second vault on the same state root.
    It failed deterministically with
    `concurrent createSeed() returned before any directory sync completed`.
    The overlay files were removed after the run.
  - `git diff --check` -> PASS before review-artifact finalization.
- Verdict: REOPEN

## Round 2 — 2026-08-03

- Reviewed state: worktree on `49bff2b`; `internal/credentialvault/vault.go`
  blob `42566656e68921a7a10e0b70edc3f6b65e047aef` and
  `internal/credentialvault/vault_test.go` blob
  `2f59f5554df93c8fd1b6b8c7af2d55a845e1d282` (uncommitted).
- Reviewer: Codex (re-review only; no product code or tests changed).
- Scope: closure of Round 1's concurrent directory-durability finding, both
  `loadSeed`-success and `fs.ErrExist` race paths, the new deterministic
  concurrency regression test, and the complete current task diff.
- Findings:
  - Round 1 [P2] closed. A caller that observes an already-linked key now syncs
    the state root before deriving the key, while a caller that loses the link
    race syncs before loading the winner's seed. Neither path can return
    dependent ciphertext before its own successful directory sync.
  - No new P1/P2 findings. The successful-link path still syncs exactly once;
    sync failures remain visible without deleting the recoverable key; key
    bytes, key IDs, file format, and file mode are unchanged.
- Evidence:
  - `codegraph explore` confirmed all `Seal`, `SealExisting`, `key`,
    `createSeed`, `createSeedExclusive`, and `loadSeed` paths against the
    updated sync points.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=50 -mod=vendor -run TestVaultConcurrentSealWaitsForDirectorySync ./internal/credentialvault`
    -> PASS (`1.622s`).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -race -mod=vendor ./internal/credentialvault`
    -> PASS (`2.818s`).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./...`
    -> PASS (all packages).
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`
    -> PASS.
  - `git diff --check` -> PASS before review-artifact finalization.
- Verdict: PASS
