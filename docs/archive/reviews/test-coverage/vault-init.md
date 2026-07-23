---
status: historical
plan: test-coverage
task: vault-init
retired: 2026-07-23
---

# Review log — test-coverage / vault-init

## Round 1 — 2026-07-23

- Reviewed state: base commit `edb774e` plus the uncommitted
  `internal/credentialvault/vault_test.go` (+166). `internal/credentialvault/vault.go`
  byte-identical to HEAD.
- Reviewer: independent cold-context test-reviewer subagent.
- Scope: task 5 additions for `InitializeNew` — non-overwrite, fail-without-key,
  post-creation preservation, permissions, and key-material absence.
- Findings:
  - [P1] `TestInitializeNewPreservesCreatedKeyForRecovery` proved "the vault
    works" (Seal/Open round-trip is self-consistent for any key), not "it
    recovered *this* key". A `deriveKey` that ignored its `seed` argument —
    deriving the credential key from machine identity alone, identical across
    installs and unbound from `credential.key`, violating the domain
    constraint — passed all 15 tests. The orchestrator independently
    reproduced this. Bind the recovered derivation to the persisted seed via a
    seed-dependent KeyID comparison.
  - [P2] The `missing machine identity` and `machine identity error` subtests
    asserted `created=true` but never `os.Stat`-ed the key file; an early
    `return true, ErrMachineIdentityMissing` before `createSeedExclusive` would
    report creation with no file, and `internal/backup/backup.go:337` takes
    ownership of `KeyPath()` on `created`. Add file-existence + mode checks.
  - [P2] Key-material leak assertions used only `hex.EncodeToString(seed)`; a
    `%v` leak on the byte slice was invisible. Also check `fmt.Sprintf("%v", seed)`.
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state: uncommitted `internal/credentialvault/vault_test.go` after the
  round-1 repairs.
- **Independence:** the repair was applied by the orchestrator in the same
  session as the review, because both subagents hit a session limit mid-fix.
  Recorded so the ticked `Review` is read with that context. The originating
  implementer subagent had already applied all three fixes to the test file
  before terminating; the orchestrator verified them independently.
- Round-1 findings, re-verified:
  - [closed] **Recovery test now binds to the seed.** After recovery it
    captures `recovered.InspectKey(ctx)` and compares against the KeyID of a
    second state root seeded with `syntheticSeedB` under the identical machine
    identity, requiring them to differ. RED: the constant-seed defect the P1
    described now fails with `recovered key ID matches a differently-seeded key
    under the same machine identity` — the orchestrator re-ran this injection
    itself and confirmed the previously-passing suite now fails at this exact
    assertion.
  - [closed] **File-existence checks added.** Both identity subtests now
    `os.Stat` the key path and assert `0600` after `created=true`.
  - [closed] **Leak checks cover a second encoding.** Both assertions now also
    reject `fmt.Sprintf("%v", seed)`.
- Evidence at this state: `gofmt -l internal/credentialvault` clean,
  `go test -mod=vendor -count=1 ./...` 548 ok across 16 packages,
  `go test -mod=vendor -race ./internal/credentialvault` ok,
  `go vet -mod=vendor ./...` clean, `git diff --check` clean.
  `internal/credentialvault/vault.go` byte-identical to HEAD.
- Verdict: **PASS.**
