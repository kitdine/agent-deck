---
status: active
plan: test-coverage
task: provider-backup
---

# Review log — test-coverage / provider-backup

## Round 1 — 2026-07-22

- Reviewed state: `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae` plus the
  uncommitted `internal/provider/service_test.go`
  (`b5e9345124189452cc85de767db9847a89ca8cc3c56cf22335c7bb7e2ae3d4f8`)
  and `internal/provider/config_test.go`
  (`3acac048cfbe7ce560e58b64d5d7d04f8f6c9d09b594d0c59dbf38db284ca132`).
- Reviewer: Codex.
- Scope: task 4 additions for `UpdateDefinition`, `ResolveCredentialName`,
  `ShowCredential`, and `WriteRedactedBackup`, against
  `internal/provider/service.go` and `internal/provider/config.go`.
- Findings:
  - [P2] `TestUpdateDefinitionResolvesCredentialAndRejectsAmbiguityOrBuiltInProvider`
    verifies only the `extra` credential after an ambiguous no-name update.
    `UpdateDefinition` resolves the credential before mutation; that ordering
    is the protection against accidentally updating any existing credential.
    An implementation that updates the default or `work` credential before
    returning the ambiguity error still leaves `extra` unchanged and passes.
    Capture endpoint, multiplier, client mappings, and credential reference
    for every existing credential before the ambiguous call, then assert exact
    equality after it fails.
  - [P2] `TestWriteRedactedBackupForCodexAndClaudeCreatesParentAndRedactsCredential`
    proves the synthetic secret is absent but does not prove safe configuration
    is retained. A redactor that serializes an empty document passes every
    current redaction and mode assertion while silently discarding usable
    backup configuration. Parse each backup and assert representative
    non-secret fields remain (for example Codex custom-provider metadata and
    Claude `env.OTHER`), while retaining the plaintext-absence checks.
- Evidence:
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/provider` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS.
  - `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...` — PASS.
  - `rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` — PASS.
  - `rtk git diff --check` — PASS before this review-record update.
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state: uncommitted `internal/provider/service_test.go` and
  `internal/provider/config_test.go` after the round-1 repairs.
- **Independence:** this round was performed in the same session as the repair,
  under an instruction to run fix and re-review automatically. Recorded so the
  ticked `Review` is read with that context.
- Round-1 findings, re-verified:
  - [closed] **Ambiguous update assertion was too weak.** The test now
    snapshots every credential of the provider — endpoint, multiplier, sorted
    client mapping, and credential reference — immediately before the ambiguous
    call and requires exact equality after it fails, via a new
    `credentialSnapshot` helper. RED: injecting the exact defect the finding
    described (mutate the `default` credential, *then* discover the ambiguity)
    fails with a before/after diff naming `default` moving to
    `https://ambiguous.example`. The previous assertion passed that same
    injected defect, which is what made this finding real rather than
    theoretical.
  - [closed] **Redaction proved absence but not retention.** Both backups are
    now parsed. The Codex source carries representative non-secret
    configuration (`model_provider`, `model`, the custom provider's `name` and
    `wire_api`, and a `[features]` table) and the backup must still contain it;
    the Claude backup must still carry `env.OTHER`. The plaintext-absence and
    file-mode checks are retained, and each side additionally asserts the
    secret key itself is gone rather than merely its value being absent.
    RED, both halves independently: a redactor that serializes an empty TOML
    document fails with `codex backup dropped the custom provider entirely`,
    and one that deletes the whole `env` map instead of the token fails with
    `claude backup dropped restorable env configuration`. Both defects satisfy
    every pre-repair assertion.
- No new defects found. The snapshot helper sorts client lists so ordering
  noise cannot cause a false failure, and it compares only fields a mutation
  could touch.
- Evidence at this state: `gofmt -l` clean, `go vet -mod=vendor ./...` clean,
  `go test -mod=vendor -count=1 ./...` all 15 packages ok,
  `go test -mod=vendor -race ./internal/provider` ok, `git diff --check` clean.
- Verdict: **PASS.**
