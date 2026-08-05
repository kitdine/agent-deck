---
status: historical
plan: credential-key-and-cache-pricing
task: key-id-derivation
retired: 2026-08-04
---

# Review log — credential-key-and-cache-pricing / key-id-derivation

## Round 1 — 2026-08-04

- Reviewed state: `HEAD` `e42f5b941e30d44ca928eaefeede463e328206aa`; reviewed product/test file SHA-256 values: `internal/credentialvault/vault.go` `92e7ec5d1467695e59d0aaa0c7805fe2f42ac5a3cfea82a85d060898e95cb628`, `internal/credentialvault/vault_test.go` `97aca210048285eec5d753881cda55a444aa492a8450c0c6d546ceb27c41bea4`, `internal/doctor/doctor.go` `e5b27bac90b6d7d5a2b45ce2357af283eac6a7ce8371208a10dd63e2e0e9e281`, `internal/doctor/doctor_test.go` `5c486d280b56e81280eefeb291018bbc9e2ec644cd33d070b8614f20768fefc7`, `internal/provider/service.go` `1e9438b62445bc483854683d091d35d6f5608e2417b88cb78f54ec4051f93892`, `internal/provider/service_test.go` `022fbd17dfd7460fa10848133af685f26577b526057d45f693410ebc0616c409`, and `cmd/agentdeck/main_test.go` `c1a9f3cfca017abcf60fd9304e23a565a0012a2d8320d946f315638b3d1dd80a`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: key-file/sealed-version separation, unchanged AES derivation, legacy/current key IDs, version-aware open and mutation gates, doctor diagnostics/recovery, and regression coverage.

### Findings

**H1 — credential mutation and doctor rotation readiness discard the sealed key version before matching its key ID, so invalid version/ID pairs can pass the fail-closed gate.** `Store.CredentialSecretKeyIDs` returns only distinct `key_id` values. Both `provider.Service.sealCredential` and `doctor.Service.checkProviders` then accept a stored ID when it equals *any* live version's ID instead of the ID for that row's `key_version`. A row with `key_version=1` and the live v2 ID is therefore treated as write-compatible even though `Vault.Open` correctly reports `credential_key_machine_mismatch`. More critically, an unsupported `key_version=3` row carrying the live v2 ID also lets a normal credential update reach `UpdateProviderCredentialWithSecret`, replacing ciphertext without the explicit `--rotate` path. This violates the acceptance requirement that unsupported versions fail closed and never overwrite ciphertext, and the plan's explicit rule that `rotationReady` means every row matches the current machine's expected ID for its own version. In doctor, the same cross-version set match can leave global `rotationReady=true` and attach a misleading `--rotate` recovery to a separate ciphertext failure even while another row has an invalid version/ID pair.

**Required fix:** retain `(key_version, key_id)` pairs when inspecting stored secrets. For each supported row, compare `key_id` only with `liveKeyIDs[key_version]`; any unsupported version, empty ID, missing live version, or mismatched pair must make provider mutation fail closed and `rotationReady=false`. Preserve the existing per-row unsupported-version and machine-mismatch diagnostics.

**Required regression coverage:** add provider tests proving normal credential writes return the appropriate fail-closed error and do not alter or add ciphertext for (a) v1 paired with the v2 ID and (b) unsupported v3 paired with the live v2 ID. Add doctor coverage proving those invalid pairs cannot make rotation ready or produce a misleading `--rotate` recovery, while the valid mixed v1/v2 case remains healthy.

### Evidence

- Full-context diff and CodeGraph call-path review of `CredentialSecretKeyIDs -> provider.Service.sealCredential` and `CredentialSecretKeyIDs -> doctor.Service.checkProviders`.
- `internal/store/providers.go:721-735` removes version information with `SELECT DISTINCT key_id`; `internal/provider/service.go:1116-1142` and `internal/doctor/doctor.go:289-318` compare each stored ID against all live IDs.
- `internal/provider/service.go:521-525` shows a passing preflight reaches `UpdateProviderCredentialWithSecret`; `internal/doctor/doctor.go:325-326,352-353` gates `--rotate` recovery on the cross-version `rotationReady` result.
- Broad verification stopped after this high-severity finding had a decisive source reproducer. Recorded development test/race/vet evidence was not rerun.
- Verdict: REOPEN

## Fix response — 2026-08-04

- Replaced the lossy `CredentialSecretKeyIDs` query with `CredentialSecretKeys`, which preserves each distinct `(key_version, key_id)` pair.
- `provider.Service.sealCredential` now requires every stored pair to match the live ID for its own version before writing; a version-1 row carrying the version-2 ID is rejected without overwriting ciphertext.
- `doctor.Service.checkProviders` uses the same pairwise rule for `rotationReady`: a supported mismatched pair suppresses `--rotate`, while an unrelated unsupported-version row preserves the prior recovery behavior for a valid damaged row.
- Added provider and doctor regression coverage for the cross-version-ID case. Targeted `internal/store`, `internal/provider`, and `internal/doctor` tests, plus `go test -mod=vendor ./...`, `go test -mod=vendor -race ./...`, and `go vet -mod=vendor ./...`, passed with `GOCACHE=/private/tmp/agent-deck-go-build`.
- Status: awaiting independent re-review; Round 1 verdict remains `REOPEN`.

## Round 2 — 2026-08-04

- Reviewed state: `HEAD` `e42f5b941e30d44ca928eaefeede463e328206aa`; changed since Round 1: `internal/store/providers.go` SHA-256 `41fb073f9599bac6c41bd5686fc19e6c9b35117e9bc8f7d07f09dba7073b2e07`, `internal/provider/service.go` `3308fd4c7dbaeea8808c49de8511b5c4b47e82672b18aaa799977096415f95d7`, `internal/provider/service_test.go` `2bffaa4de58b8f44c158047f620eec0fc1cc3810b4e3a3b6381ee2ee21cf44bf`, `internal/doctor/doctor.go` `ab092b465776055889bb523b525d6eecc5bb2b6dcd6ac4fd7c7e216f90f7db07`, and `internal/doctor/doctor_test.go` `4f0bf48b82bb064500553bfe563dfc2f432a3524c883cc9e04fb250d40643647`. The vault and CLI seam files retain their Round 1 hashes.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: H1 pair preservation, provider mutation fail-closed behavior, doctor rotation recovery, and required regression coverage.
- H1 closure: the store now preserves distinct `(key_version, key_id)` pairs. Supported cross-version IDs fail provider mutation without changing ciphertext; doctor reports the pair mismatch and suppresses `--rotate`. Valid mixed v1/v2 rows remain healthy. Round 1's blanket requirement that an unrelated unsupported row always make `rotationReady=false` was overbroad: the plan also requires recovery availability to remain identical to prior behavior, under which a separately valid damaged row retains its explicit rotate recovery.

### Remaining finding

**M1 — provider mutation reports an unsupported stored key version as a machine mismatch instead of the required unsupported-version error.** In `provider.Service.sealCredential`, `!supported`, an empty/mismatched ID, and a different machine all return `credentialvault.ErrKeyMachineMismatch`. Therefore a stored `key_version=0` or `3` correctly blocks the write but surfaces `credential_key_machine_mismatch`, while the task acceptance requires unsupported versions to fail closed with `credential_key_version_unsupported`. These errors are distinct CLI typed error codes, so the conflation changes the user-visible diagnosis and recovery contract. The Round 1 required provider regression for an unsupported version paired with the live v2 ID is also absent.

**Required closure:** split the provider preflight branches: return `ErrKeyVersionUnsupported` when `liveKeyIDs` has no entry for the stored version; return `ErrKeyMachineMismatch` only for an empty or nonmatching ID of a supported version. Add deterministic provider mutation tests for versions 0 and 3 paired with the live v2 ID, asserting the exact unsupported-version error and byte-for-byte unchanged stored ciphertext.

### Evidence

- `internal/provider/service.go:1128-1132` combines `!supported` with ID mismatch and returns only `ErrKeyMachineMismatch`.
- `cmd/agentdeck/main.go:318-321` maps machine mismatch and unsupported version to distinct typed error codes.
- Existing provider coverage proves supported v1/v2 cross-version mismatch, but no provider mutation test covers stored version 0 or 3.
- Broad verification stopped after the medium finding had a decisive source reproducer. Recorded repair test/race/vet evidence was not rerun.
- Verdict: REOPEN

## Fix response — 2026-08-04 (Round 2)

- `provider.Service.sealCredential` now returns `credential_key_version_unsupported` before comparing IDs when a stored key version is outside the vault's supported set; supported versions with an empty or mismatched ID still return `credential_key_machine_mismatch`.
- Added deterministic provider mutation coverage for stored versions 0 and 3 paired with the live version-2 ID. Each case asserts the exact unsupported-version error and byte-for-byte unchanged stored secret.
- Targeted `internal/store`, `internal/provider`, and `internal/doctor` tests, plus `go test -mod=vendor ./...`, `go test -mod=vendor -race ./...`, and `go vet -mod=vendor ./...`, passed with `GOCACHE=/private/tmp/agent-deck-go-build`.
- Status: awaiting independent re-review; Round 2 verdict remains `REOPEN`.

## Round 3 — 2026-08-04

- Reviewed state: `HEAD` `e42f5b941e30d44ca928eaefeede463e328206aa`; changed since Round 2: `internal/provider/service.go` SHA-256 `8b718b508ea378d2ae0c25b5459188a793832830115d10bace3afd43a0d6b395` and `internal/provider/service_test.go` `73d8f1a5a1736821ebf0b8205ef25a2083755ead5afc8bdd446ae44a54216125`. The other reviewed product/test files retain their Round 2 hashes.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: Round 2 unsupported-version typed error, versions 0/3 mutation regression coverage, and complete H1 closure.
- Finding closure: `provider.Service.sealCredential` now returns `ErrKeyVersionUnsupported` before ID comparison when a stored version is outside the supported set. Supported versions with empty, cross-version, or machine-mismatched IDs continue to return `ErrKeyMachineMismatch`. Deterministic version 0 and 3 tests assert the exact unsupported-version error and byte-for-byte unchanged stored secret.
- New findings: none.
- Verification:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/credentialvault ./internal/store ./internal/provider ./internal/doctor` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -race ./...` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` -> PASS.
- Verdict: PASS
