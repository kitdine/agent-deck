---
status: historical
plan: provider-wrapper-routing
task: route-composition
---

# Review log — provider-wrapper-routing / route-composition

## Round 1 — 2026-07-27

- Reviewed state: base `00b6803`, uncommitted working tree. SHA-256 of
  `git diff -- internal/provider/service.go cmd/agentdeck/main.go internal/provider/service_test.go`
  = `f8614c9d4754cf76…`; SHA-256 of the untracked
  `internal/provider/route_composition_test.go` = `0f7e85f0962d3ef0…`.
  `docs/plans/provider-wrapper-routing.md` is also dirty (this task's Done note
  and its `Dev` tick). Tasks 1–3 are already committed and out of scope.
- Reviewer: Claude Opus 5 (read-only pass, independent session)
- Scope: `internal/provider/service.go:541-721` (`UseCredential`'s new `via`
  parameter, the official/custom endpoint composition, wrapper resolution, and
  the selection snapshot), `service.go:850-856` (`officialProvider()` now
  advertising Claude), `cmd/agentdeck/main.go:695` (call-site update),
  `route_composition_test.go` (5 new tests), and the nine mechanical
  `service_test.go` call-site updates. Contract checked against
  `docs/specs/cli-design.md` v15 "Provider Wrappers", "Owned Client
  Configuration Fields", and "Selecting the Built-in Provider", plus the plan's
  `## Invariants`. Reachability of the newly-enabled states traced through
  `cmd/agentdeck/main.go` and every consumer of `store.ProviderSnapshot`.

### Findings

- **[P1] `agentdeck doctor` reports permanent, unfixable config drift after
  `provider use official --client claude`.** `service.go:127-131`
  (`ConfigDrift`) branches on `snapshot.Official` alone and calls
  `ConfigMatchesOfficialCodex(path)` — a TOML parser — for *any* official
  selection. Before this task, official rejected `--client claude`
  (`service.go:556` old) so that branch only ever saw a Codex `config.toml`.
  This task removed that guard, so the branch now hands Claude's
  `settings.json` to `toml.Unmarshal`, which errors, so `checkErr != nil` →
  `drift++`.

  Reproduced end to end against a built binary with an isolated
  `HOME` and `--state-dir`:

  ```text
  $ agentdeck provider use official --client claude
  Completed provider.use for "official".            # settings.json correct:
                                                    # {"env":{"UNRELATED":true}}
  $ agentdeck doctor
  status: degraded
  provider_configuration: warning (provider_config_drift; count=1)
    recovery: agentdeck provider use
  ```

  The switch itself is correct — both owned keys removed, unowned keys kept.
  Only the read-back is wrong. The offered recovery (`agentdeck provider use`)
  can never clear it, so a user who follows the advice re-runs the switch
  forever. `internal/doctor/doctor.go:308` is the live consumer, so this is a
  user-visible degraded status on a documented, CLI-reachable invocation today.

  -> `ConfigDrift` must branch on client as well as `snapshot.Official`: an
  official Claude selection is "no `ANTHROPIC_BASE_URL` and no
  `ANTHROPIC_AUTH_TOKEN` under `env`", not "matches official Codex".

- **[P2] The same `ConfigDrift` branch also mis-reports every `--via` official
  selection as drift.** `ConfigMatchesOfficialCodex` requires `!hasBaseURL`
  (`config.go:74-76`), but `WriteCodexWrapperConfig` writes `base_url` by
  design, so `official --via` on Codex is drifted the moment it succeeds. Not
  reachable from the CLI yet (`--via` lands in `cli-route-surface`), which is
  the only reason this is P2 and not a second P1 — but it is the same root
  cause as the P1 and must not be left for task 5 to discover.

  Not a defect on the custom-provider route: `EndpointSnapshot` now records the
  endpoint actually written, so `ConfigMatchesEndpoint(client, path, wrapper)`
  matches for both clients on a `--via` custom switch. Verified by reading
  `config.go:28-56` against `service.go:693-697,712`. That half of the
  snapshot change is exactly right and is what makes the custom route work.

- **[P3] The carried `claude-writer-routes` P3 is only half closed.** The task
  brief requires that *every* call into `WriteClaudeConfig` decide its
  endpoint/credential intent explicitly, because `""` is the writer's
  "remove this key" sentinel. `TestUseOfficialClaudeDirectAndViaWrapperDecide
  IntentExplicitly` covers the official route only. The custom route
  (`service.go:697`) still passes `selectedCredential.Endpoint` and the
  decrypted `credential` straight through, and both are lookups that *could*
  return `""` for a reason other than "this field should be absent". They are
  in fact non-empty for every live row — `AddProviderWithCredential` rejects an
  empty secret (`service.go:233`), `UpdateNamedCredential` rejects one
  (`service.go:419`) and re-runs `Validate` on the endpoint (`service.go:409`),
  and portable restore restores a database written under those rules — so this
  is latent, not live. But the invariant now lives three call layers away from
  the sentinel it protects, is documented nowhere at the call site, and no test
  pins it.

  -> Either assert the two values are non-empty before composing `ClientConfig`
  (turning a silent key-deletion into an error), or state the upstream
  invariant in a comment at `service.go:692-697` and add a test that a
  custom-provider Claude switch writes both owned keys. A comment alone does
  not satisfy "cover this with a test".

- **[P3] `err` is tested outside the branch that assigns it.**
  `service.go:621-634`: `wrapperURL, err = s.Store.OfficialWrapperURL(ctx)` in
  one arm, plain assignment in the other, then a shared `if err != nil`. On the
  custom arm `err` still holds whatever `s.Vault.Open` left (nil, since a
  non-nil value returned at `service.go:614`), so this is benign today and
  purely a fragility: any future early assignment to `err` above this block
  becomes a spurious failure on the custom route.
  -> Scope the check to the official arm, or use a fresh `wrapperErr`.

- **[P3] `official` still reports `authentication: codex_existing_login` while
  now serving Claude.** `service.go:855` was left unchanged when `Clients`
  gained Claude at `service.go:854`. `cmd/agentdeck/main.go:2382-2383` prints
  it in `provider show`, so a user selecting official for Claude is told their
  authentication mode is a Codex login. The spec (cli-design.md:446-452) says
  authentication is decided by which provider is selected and describes the
  built-in provider as leaving *each* client on its own login. The string is
  not part of any documented JSON contract I can find, and
  `service_test.go:210` pins the old value.
  -> Rename to a client-neutral value (e.g. `client_existing_login`) and update
  the test, or record why the Codex-specific name is kept.

- **[nit] The plan's Done note misdescribes the Claude official branch.** It
  says direct calls `WriteClaudeConfig{}` "(both fields removed)" and `--via`
  calls `WriteClaudeConfig{Endpoint: wrapper}`, implying two call sites naming
  two intents. `service.go:688` is a single call passing
  `ClientConfig{Endpoint: writtenEndpoint}`, where `writtenEndpoint` is `""`
  by omission on the direct route — which is the zero-value-decides-intent
  shape the carried P3 asked to avoid, not two explicit branches.
  -> Correct the note, or split the call to match what it claims.

### Not defects (checked and dismissed)

- Replacing the codex-only guard with a `credentialName != ""` guard for
  official: correct per cli-design.md:683 ("takes no `--credential`"), and
  `cmd/agentdeck/main.go:665-667` already rejects it earlier with its own
  message, so no user-facing error text changed.
- `provider use official` with no `--client` still defaults to Codex.
  `main.go:664-670` special-cases official *before* the multi-client inference
  at `main.go:672-687`, so adding Claude to `officialProvider().Clients` does
  not turn the documented bare invocation (`cli-manual.md:130`) into a
  "`--client` is required" error. Verified empirically: exit 0, unchanged text.
- Custom `--via` credential identity: both routes call the same
  `WriteCodexConfig`/`WriteClaudeConfig` with only `Endpoint` differing, so the
  acceptance criterion holds structurally, not just by test. The deferred
  `WriteCodexConfig`→`rewriteCodexCustomTable` migration is correctly recorded
  as an open decision rather than silently dropped.
- `rejectingCredentialVault` assertions: both official tests prove the vault is
  never touched, satisfying cli-design.md:442-444.
- Snapshot: `ViaWrapper` and `EndpointSnapshot` are now set unconditionally
  before the `definition != nil` block, so official selections carry the route
  too, per cli-design.md:690-692. Multiplier stays `1` for official.
- The nine `service_test.go` call-site edits are mechanical `, false` additions
  with no assertion changes.

### Evidence

- `go test -mod=vendor ./internal/provider/... ./internal/store/...` → `ok`
  (both packages).
- `gofmt -l internal/provider cmd/agentdeck` → clean.
- Built `./cmd/agentdeck` and ran the P1 reproduction above under an isolated
  `HOME` and `--state-dir`; scratch tree only, repository untouched.
- P2 assessed by reading `config.go:59-77` against `config.go:249-306`; not
  reproduced through the CLI because `--via` has no flag yet.

### Verdict

**REOPEN.** One P1 (user-visible `doctor` regression on a CLI-reachable
invocation this task newly enabled) plus one P2 of the same root cause. `Dev`
unticked until both close. The three P3s and the nit may be closed in the same
fix round or explicitly deferred with a recorded decision. The task's three
named acceptance criteria are met on the write path; the gap is entirely on the
read-back path that this task's new states made reachable.

Next pass: Round 2 in this file.

## Round 2 — 2026-07-27 (re-review)

- Reviewed state: base `00b6803`, uncommitted working tree. SHA-256 of
  `git diff -- internal/provider/service.go internal/provider/config.go
  cmd/agentdeck/main.go internal/provider/service_test.go` =
  `b36df5ed2a3f9cc6…`; SHA-256 of the untracked
  `internal/provider/route_composition_test.go` = `dc856f549c2c0983…`.
  `internal/provider/config.go` is newly dirty since Round 1 (the two added
  matchers). Tasks 1–3 remain committed and out of scope.
- Reviewer: Claude Opus 5 (read-only pass; every command below ran against an
  out-of-tree copy of the working tree, whose `internal/provider` was verified
  byte-identical to the repository before and after. The repository itself was
  not modified by this pass — `git status --short` is unchanged from the entry
  state.)
- Method: each Round 1 finding re-derived from the current source rather than
  from the Fix round note, then the claimed RED state reproduced independently
  by reverting the fix *in the copy* and rerunning the new tests.

### Finding-by-finding disposition

- **[P1] official Claude drift — FIXED.** `ConfigDrift`
  (`service.go:115-147`) now switches on route and client, and
  `ConfigMatchesOfficialClaude` (`config.go:85-98`) proves the Claude
  no-owned-key state directly instead of parsing `settings.json` as TOML.
  Independently reproduced RED: with the switch reverted to the Round 1
  two-way `if snapshot.Official`, `TestOfficialClaudeSelectionIsNotReported
  AsConfigDrift` fails with `drift = 1, want 0`. The end-to-end Round 1
  reproduction was re-run against a binary built from this tree under an
  isolated `HOME`/`--state-dir`: `provider use official --client claude`
  leaves `settings.json` as `{"env":{"UNRELATED":true}}` and `doctor` now
  reports `provider_configuration: ok`. Selecting official for *both* clients
  in the same home also holds at `ok`, so the Codex arm did not regress.
- **[P2] `--via` official drift — FIXED.** `ConfigMatchesOfficialWrapper`
  (`config.go:106-138`) requires the wrapper endpoint (`+ "/v1"` on Codex,
  bare on Claude, matching what the writers produce) *and* the absence of a
  credential. Independently reproduced RED on the reverted branch: both
  `TestOfficialCodexViaWrapperSelectionIsNotReportedAsConfigDrift` and
  `TestOfficialClaudeViaWrapperSelectionIsNotReportedAsConfigDrift` fail with
  `drift = 1, want 0`. Both tests also assert `drift = 1` after a credential is
  injected into the wrapper-routed file, so neither can be satisfied by a
  permissive matcher.
- **[P3] carried `claude-writer-routes` intent gap — CLOSED.** The custom route
  no longer feeds a possibly-empty lookup into a writer whose empty field means
  "delete this owned key": `service.go:653-655` rejects an empty written
  endpoint or empty secret for a custom provider, sited before `operationID()`,
  the redacted backup, and any client write. Independently reproduced RED: with
  the guard short-circuited, `TestUseCustomProviderWithEmptyStoredEndpointFails
  BeforeTouchingClientFile` fails with `= <nil>, want ErrInvalidProvider`. The
  test forces the otherwise-unreachable row through SQL and asserts the error
  kind, a byte-identical client file, and `sql.ErrNoRows` for the snapshot, so
  it pins the fail-fast *position*, not just the error. Checked against
  cli-design.md:644-652: a custom `--via` selection is specified to keep writing
  its secret, so requiring both fields on that route rejects no valid state.
- **[P3] `err` tested outside its assigning branch — CLOSED.** The official arm
  now uses a locally-scoped `wrapperErr` returned inside the arm
  (`service.go:630-636`); no shared `if err != nil` spans the two arms.
- **[P3] Codex-specific authentication label — CLOSED.**
  `officialProvider()` now returns `client_existing_login` with a doc comment
  explaining why the value is client-neutral, and `service_test.go:210` was
  updated to match (also loosening the `Clients` assertion to a set of both
  clients rather than a length-1 slice). No output contract broke: a repo-wide
  grep finds `codex_existing_login` only in this review file and the plan's own
  Round 1 quote; the sole JSON fixture that mentions the field
  (`cmd/agentdeck/testdata/phase7/gui-json-contract.json`) declares the type
  `"string"`, not a value; `docs/specs/` documents authentication as a concept
  and never enumerates the token. Confirmed live: `provider show official`
  prints `authentication: client_existing_login` and `clients: codex,claude`,
  and its `--format json` envelope carries the same value plus both client
  mappings — an additive `clients` entry and a changed unenumerated string.
- **[nit] plan Done note vs. code — CLOSED, by changing the code.** The official
  Claude branch is now two explicit calls (`WriteClaudeConfig(ClientConfig{})`
  direct, `WriteClaudeConfig(ClientConfig{Endpoint: writtenEndpoint})` for
  `--via`) at `service.go:706-710`, which is what the note already described.
  The Fix-round bullet says the note was "corrected"; what actually changed is
  the code — the note's text is unchanged and is now accurate. Wording only.

### Branch-coverage check (requested explicitly)

The four `ConfigDrift` arms map one-to-one onto the four valid combinations in
cli-design.md:646-649, with no overlap and no gap: official+`--via` (either
client) → `ConfigMatchesOfficialWrapper`; official+direct+Claude →
`ConfigMatchesOfficialClaude`; official+direct+Codex →
`ConfigMatchesOfficialCodex` (unchanged); custom (either route) →
`ConfigMatchesEndpoint` against `snapshot.Endpoint`, which now records the
endpoint actually written, so the wrapped and direct custom routes both compare
against the right value. The loop still visits only `codex` and `claude`, and
`sql.ErrNoRows` still skips an unselected client, so no new client state is
reachable. No new false positive found: each new matcher accepts exactly the
shape its writer produces (`WriteClaudeConfig` with both fields empty, and
`WriteCodexWrapperConfig`/`WriteClaudeConfig`+endpoint respectively), verified
by reading writer against matcher and by the four drift tests, three of which
also assert a positive drift case.

### Not defects (checked and dismissed in this round)

- Asymmetric strength between the two direct official matchers: the Codex one
  requires AgentDeck's `name = "official"` marker, the Claude one only proves
  both owned keys are absent, so a Claude settings file that never went through
  AgentDeck also reads as "no drift". That is inherent to the route — a direct
  official Claude selection owns no field, so there is no marker to check — and
  the matcher's doc comment says so. Detecting unowned overrides
  (`ANTHROPIC_API_KEY`, `apiKeyHelper`) is `switch-advisories`, not this task.
- `TestCustomProviderViaWrapperSelectionIsNotReportedAsConfigDrift` passes on
  the pre-fix branch too. It is a pin for a path that already worked, not a
  regression test, and its own comment says exactly that. Keeping it is right;
  it is the only one of the four whose RED state does not exist.
- Bare `provider use official` with no `--client` still defaults to Codex after
  `Clients` gained a second mapping: `main.go:664-671` special-cases official
  before the multi-client inference at `main.go:672-687`. Re-verified live,
  exit 0.
- `Service.Use` and the single CLI call site both pass `via=false`; no other
  caller of `UseCredential` exists outside tests, so the new parameter changes
  no existing behavior.
- Snapshot now always carries `EndpointSnapshot`/`ViaWrapper`, including `""`
  and `false` for a direct official selection. Consistent with
  cli-design.md:689-692, and the drift default branch is only reached for
  non-official selections, so the empty official endpoint is never compared.

### Evidence

- `go test -mod=vendor ./internal/provider/...` → `ok` on the unmodified copy.
- Reverted-fix runs (copy only): drift switch reverted → 3 FAIL / 1 PASS as
  described; empty-endpoint guard short-circuited → that test FAILs. Both
  reverts discarded afterwards; `internal/provider` re-verified byte-identical
  to the repository.
- `go test -mod=vendor ./...` → all packages `ok` except
  `./cmd/agentdeck` (`TestIsolatedEndToEndFlow`,
  `TestSessionShowActivityReadsOnlySafeMetadataOnDemand`). Independently
  confirmed pre-existing: `git stash -u` in the copy reproduces exactly those
  two failures at `00b6803`.
- `go vet -mod=vendor ./...` clean; `gofmt -l internal/provider cmd/agentdeck`
  clean.
- Built `./cmd/agentdeck` from the copy and ran the P1 reproduction, the
  both-clients drift case, bare `provider use official`, and
  `provider show official` in text and JSON under an isolated `HOME` and
  `--state-dir`; scratch tree only.

### Verdict

**PASS.** The P1 and P2 are closed at the root cause, all three P3s and the nit
are closed rather than deferred, each new test was independently confirmed to
fail against the reverted fix (except the one that is a pin by design and says
so), the four drift branches cover the full route matrix with no new
misjudgment, and the `authentication` rename breaks no output contract in code,
fixtures, or specs. No new finding at or above nit level. `Review` ticked for
`route-composition`; no fix round follows.
