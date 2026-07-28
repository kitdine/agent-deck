---
status: historical
plan: provider-wrapper-routing
task: wrapper-schema
---

# Review log — provider-wrapper-routing / wrapper-schema

## Round 1 — 2026-07-26

- Reviewed state: base `ea1df47`, uncommitted working tree; SHA-256 of
  `git diff -- cmd/agentdeck/main_test.go internal/` = `bd5dbe4113587e09…`.
  The plan and spec edits under `docs/` are design artifacts of the same
  change and were read but are not part of this task's code state.
- Reviewer: Claude Opus 5 (cold read of the diff, independent of the
  implementation session)
- Scope: `internal/store/migrations.go` (migration 15),
  `internal/store/providers.go` (`WrapperURL`/`ViaWrapper` fields,
  `SetProviderWrapper`, `OfficialWrapperURL`/`SetOfficialWrapperURL`,
  `boolToInt`, six `provider_selections` queries),
  `internal/store/store.go` (`CurrentSchemaVersion` 14→15),
  `internal/provider/service.go` (`NormalizeWrapperURL`),
  `internal/store/store_test.go`, `internal/provider/provider_test.go`,
  `cmd/agentdeck/main_test.go`. Behavior checked: migration on an existing
  database, selection column/scan alignment, credential-boundary isolation,
  wrapper storage round-trip, normalization contract.
- Findings:
  - [P2] `NormalizeWrapperURL` has zero production callers and neither
    `SetProviderWrapper` nor `SetOfficialWrapperURL` normalizes. Storage-layer
    purity matches the existing pattern (`provider.go:52` normalizes before
    persisting a credential endpoint), so this is a seam rather than a defect
    here — but the obligation is recorded nowhere, and `cli-route-surface` has
    no acceptance clause for it. An unnormalized `…:15021/v1` would make the
    Codex writer emit `…/v1/v1`, and no current test fails. -> Document the
    caller obligation on all three functions and add the acceptance clause to
    `cli-route-surface`.
  - [P2] `TestNormalizeWrapperURLReusesCodexAwareCredentialEndpointNormalization`
    compares only against `NormalizeCredentialEndpoint(url, true)`, while the
    task's acceptance says "for each client". The `codex=false` side is
    deliberately different (a Claude-only credential endpoint keeps a trailing
    `/v1`; a wrapper always strips it), and `cli-design.md`'s "normalized
    exactly like credential endpoints" therefore overstates the rule. -> Correct
    the spec sentence and add an assertion pinning the intended divergence, so a
    later reader does not "fix" it into client-aware behavior.
  - [P3] The v15 migration test asserts only `ViaWrapper == false` and an empty
    wrapper on the migrated provider. This change reorders six
    `provider_selections` SELECT/`Scan` pairs, so the values most at risk —
    `endpoint_snapshot`, `multiplier_snapshot`, `credential_name_snapshot` — are
    unpinned. -> Assert the concrete Name/Endpoint/Multiplier/Credential values
    from the fixture after migration.
  - [nit] `store.Provider.WrapperURL` carries `json:"wrapper_url,omitempty"`,
    but the output contract is `provider.Provider` via
    `service.go:814 storedProvider`, which does not copy the field. The tag is
    inert and implies an output contract that does not exist. -> Remove it; let
    `cli-route-surface` add the real output field.
  - [nit] `SetProviderWrapper(name, "")` means "clear", while
    `NormalizeWrapperURL("")` returns `ErrInvalidProvider` (pinned by test). The
    opposite empty-string semantics are undocumented. -> Note the `--clear`
    bypass in the `NormalizeWrapperURL` doc comment.
- Evidence:
  - `go test -mod=vendor ./internal/store/... ./internal/provider/...` → both
    packages `ok` (run by this review against the reviewed state).
  - Full `go test -mod=vendor ./...` (710 passed) and `go vet -mod=vendor ./...`
    reused from the implementation note; content state unchanged since, so not
    rerun.
  - Read-only checks: all six `provider_selections` queries verified for
    SELECT/`Scan` order; `LatestSelectionCredential` correctly left untouched;
    `SetSetting`/`DeleteSetting` reach `secureFiles()` through `s.Exec`;
    `service.go:140` already rejects a custom provider named `official`, so the
    two wrapper stores cannot collide; the portable archive snapshots the whole
    core database (`backup.go:102`), so the `settings`-held official wrapper URL
    travels with a device migration.
- Verdict: REOPEN — no correctness or security defect, but the two P2 findings
  become real defects when `cli-route-surface` lands and no current test would
  catch them. Task returns to `Dev` for the five items above; next pass is
  Round 2 in this file.

## Round 2 — 2026-07-27

- Reviewed state: base `ea1df47`, uncommitted working tree; SHA-256 of
  `git diff -- cmd/agentdeck/main_test.go internal/` =
  `495fb3ba203ce8c4…` (Round 1 reviewed `bd5dbe4113587e09…`).
- Reviewer: Claude Opus 5 (re-review pass; independent re-verification of each
  Round 1 finding plus a regression sweep)
- Scope: the Round 1 finding set, the fix-round edits to
  `internal/store/providers.go`, `internal/provider/service.go`,
  `internal/store/store_test.go`, `internal/provider/provider_test.go`,
  `docs/specs/cli-design.md`, `docs/plans/provider-wrapper-routing.md`, and a
  fresh sweep of every `provider_selections` / `providers` SQL site for
  column-list drift.
- Round 1 finding status:
  - [P2] normalization obligation undocumented — **fixed**. All three doc
    comments (`SetProviderWrapper`, `SetOfficialWrapperURL`,
    `NormalizeWrapperURL`) now state that the store setters are pure storage
    and that callers must normalize first, and `cli-route-surface`'s
    Acceptance carries the matching clause naming `--clear` as the one
    normalization-skipping path.
  - [P2] `codex=false` divergence unpinned and spec overstated — **fixed**.
    `cli-design.md`'s "Provider Wrappers" paragraph and the v15 changelog row
    both now say "always normalized like a Codex-bound credential endpoint,
    regardless of which clients the provider actually serves", and
    `TestNormalizeWrapperURLDiffersFromClaudeOnlyCredentialEndpointOnV1Input`
    asserts both the inequality and the two concrete values
    (`…/api` vs `…/api/v1`), so it fails on either a silent normalization
    change or a "fix" toward client-aware behavior.
  - [P3] v15 migration test asserted only the new column — **fixed**. It now
    pins `Name`/`Endpoint`/`Multiplier`/`Credential` on the migrated `codex`
    snapshot, which is what a mis-ordered `Scan` in the reordered selection
    queries would corrupt.
  - [nit] inert `json:"wrapper_url,omitempty"` tag — **fixed** (tag removed).
    See the new nit below for the residue.
  - [nit] `--clear` / `NormalizeWrapperURL("")` opposite empty-string
    semantics — **fixed**; documented on all three functions, and
    `TestNormalizeWrapperURLRejectsInvalidEndpoints` pins `""` →
    `ErrInvalidProvider` so the documented bypass is the only way to clear.
- New findings (neither blocking):
  - [nit] `store.Provider.WrapperURL` now carries no struct tag at all. Every
    sibling field is tagged, and absence does not mean "excluded" — if a
    `store.Provider` ever reaches `writeResult`'s `json.NewEncoder`, the field
    marshals as `"WrapperURL"`, un-omitempty and off the project's snake_case
    convention. Verified no live path does: `cmd/agentdeck/main.go:724,757`
    are the only CLI consumers of `AddProviderWithCredential`/
    `UpdateDefinition` and both discard the returned value, and
    `service.go:822 storedProvider` does not copy the field. `json:"-"` would
    state the intent the tag's absence only implies. -> Optional; fold into
    `cli-route-surface` when it adds the real `wrapper_url` output field.
  - [nit] `docs/README.md`'s new paragraph still says the plan was opened
    "with no task started", which was true when written and is now stale —
    `wrapper-schema` is built and, as of this round, reviewed. -> Refresh with
    the status-doc sync pass, together with the `0/7 done` index row.
- Regression sweep (read-only, independent of Round 1):
  - All five `provider_selections` SELECT/`Scan` pairs re-verified for
    positional alignment; both INSERTs are 10 columns / 10 placeholders /
    10 args; `LatestSelectionCredential` correctly still selects one column.
  - No `SELECT *` and no other column-enumerating access to
    `provider_selections` or `providers` exists outside
    `internal/store/providers.go` and `migrations.go`, so no reader was left
    behind by the new columns.
  - A fresh database reaches v15 through the same sequential migration list —
    there is no separate current-schema snapshot to keep in sync.
- Evidence (run by this round against the reviewed state):
  - `go test -mod=vendor ./internal/store/... ./internal/provider/...` → both
    packages `ok`.
  - `go test -mod=vendor ./cmd/... -run TestStateMigrateTextAndJSONUpgradeSchema12`
    → `ok` (the v12-replay fixture that the two new columns forced to change).
  - `go vet -mod=vendor ./...` → clean.
- Verdict: PASS — all five Round 1 findings are closed with tests that would
  fail if the behavior regressed, and no new defect was introduced. The two
  nits above are documentation/consistency follow-ups for `cli-route-surface`
  and the next status-doc sync, not reasons to hold this task.
