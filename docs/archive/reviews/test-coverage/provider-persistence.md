---
status: historical
created: 2026-07-22
plan: test-coverage
task: provider-persistence
retired: 2026-07-23
---

# Review log — test-coverage / provider-persistence

## Round 1 — 2026-07-22

- Reviewed state: `b6a13632b50a6119194b351fbb48b04b1a2dc44a` plus the
  uncommitted `internal/store/providers_test.go`
  (`16fb7d4fa58afa7b3cac962a80e2cedfcf07cd691f0cdbaf867c1482f4d790b3`).
- Reviewer: Codex.
- Scope: task 2 test additions in `internal/store/providers_test.go` for
  `CompleteProviderUse`, `UpdateProviderCredentialWithSecret`,
  `PendingOperations`, and `UpdateOperationDetails`, against
  `internal/store/providers.go`. Task 1 and unrelated worktree changes were
  out of scope.
- Findings:
  - [P2] `TestCompleteProviderUseCompletesOperationAndPersistsSelection`
    asserts only three of the immutable selection snapshot fields and submits a
    nil credential ID. `CompleteProviderUse` persists provider ID, client,
    multiplier, credential ID, and selected time as well. The provider contract
    relies on the completed operation for credential and multiplier attribution;
    an implementation that drops those fields can still pass. Create a
    credential fixture, use non-default multiplier/credential/selected-time
    values, and assert every contract-relevant persisted snapshot field.
  - [P2] `TestUpdateProviderCredentialWithSecretFailureDoesNotPartiallyPersist`
    checks `provider_credential_clients` but not the derived
    `provider_clients` aggregate. `UpdateProviderCredentialWithSecret` calls
    `syncProviderClients` in the same transaction. A reordered or non-atomic
    implementation can remove/add an aggregate client before the secret write
    fails while every current assertion remains green. Capture the provider's
    aggregate client mappings before and after the injected failure and assert
    exact equality.
  - [P2] `TestUpdateOperationDetailsPersistsAndRollsBackOnWriteFailure`
    verifies only successful `details_json`. The production statement also
    persists `state` and `error_code`; an implementation that leaves either
    stale would pass this test. Assert the successful row's `state`,
    `error_code`, and `details_json` together (and retain the current
    failure-rollback assertion).
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store -count=1` — PASS.
  - `rtk git diff --check` — PASS before creating this review record.
  - No full-suite or race result is claimed for this review round; task 2 is
    reopened on assertion quality, not a runtime failure.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae` plus the
  uncommitted `internal/store/providers_test.go`
  (`f3da6aac8f54576c90d1d69e6e0d77f75876dac38b68577114a4089040bd5424`).
- Reviewer: Codex.
- Scope: task 2 only (`internal/store/providers_test.go`) and review-close evidence:
  `internal/store/providers.go`.
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store` — PASS.
  - `rtk git diff --check` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` fails only at the unreviewed task 3 test `internal/usage: TestScanCostScalesLinearlyNotQuadraticallyWithLineCount`; it is not evidence against task 2 and has not been diagnosed in this review.
- Findings:
  - All three `Round 1` P2 findings are addressed by the updated tests:
    - full provider selection snapshot persistence assertions in
      `TestCompleteProviderUseCompletesOperationAndPersistsSelection`;
    - pre/post `provider_clients` aggregate mapping equality in
      `TestUpdateProviderCredentialWithSecretFailureDoesNotPartiallyPersist`;
    - successful state/error/details assertions in
      `TestUpdateOperationDetailsPersistsAndRollsBackOnWriteFailure`.
- Verdict: PASS
