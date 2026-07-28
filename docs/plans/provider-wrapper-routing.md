---
status: active
created: 2026-07-26
---

# Provider Wrapper Routing Plan

Let any provider carry an optional wrapper URL and let one switch choose whether
to route through it, so a compression proxy can sit in front of a relay or in
front of the vendor without duplicating credentials or inventing provider names.
Contract: `docs/specs/cli-design.md` v15, sections "Provider Wrappers", "Owned
Client Configuration Fields", and "Selecting the Built-in Provider".

## Why

Two capabilities are missing, and they are the same shape.

A user running a local or LAN proxy in front of a third-party relay has to point
the client at the proxy while still sending the relay's own token, because the
proxy forwards that token verbatim to the upstream that issued it. Today the
endpoint and the credential are written together from one credential record, so
the only way to express the proxied route is a second credential holding a
duplicate of the same secret — and, since the multiplier is credential-owned, a
second multiplier that can silently drift away from the first for what is one
billed account.

A user with a subscription has the same need against the vendor itself: point
the client at the proxy, write no credential, let the client's own login ride
through. AgentDeck cannot express that either. Its built-in `official` provider
is Codex-only, and its Claude writer always writes a token, which per Anthropic's
documentation replaces the subscription and moves billing to whoever owns that
token.

Both are one missing dimension: **where the request is sent** is independent of
**who is billed**. A wrapper changes the first and nothing else.

## Evidence

Verified on `main` at `ea1df47`, and against the proxy this is designed for:

- `internal/provider/config.go:229-230` — `WriteClaudeConfig` writes
  `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` unconditionally.
- `internal/provider/service.go:579-581` — every switch resolves a credential.
- `internal/provider/service.go:535-538`, `service.go:795-802` — `official` is
  hard-coded to Codex, so Claude has no path back to its own login.
- Headroom's `headroom/backends/litellm.py` forwards the client's credential to
  the upstream (`Authorization: Bearer` matched first, `x-api-key` as fallback),
  and skips forwarding only for the env-authenticated backends (Bedrock, Vertex,
  SageMaker) that hold their own upstream credentials. Its
  `headroom/proxy/cc_switch_reconciler.py` states the same rule in prose: "The
  token rides in the request … passed through verbatim; Headroom never reads or
  stores it."
- Headroom's proxy serves `/v1/messages` and `/v1/responses` plus the Codex
  alias paths on one address, so one wrapper URL covers both clients.
- Headroom's upstream is fixed at startup by `--anthropic-api-url` /
  `--openai-api-url` / `--backend`, so one instance fronts one upstream — which
  is why the URL belongs to a provider rather than to a credential or a client.
- `internal/store/migrations.go` — versioned migrations exist, so the new column
  lands through the normal sequence.

## Tasks

### `wrapper-schema`

Add the persisted model in one migration: a nullable `providers.wrapper_url`, a
place to hold the built-in provider's wrapper URL without giving it a provider
definition row, credential, or ciphertext, and the route field on the selection
snapshot. Wrapper URLs reuse the existing endpoint parser, validation, and
client-aware `/v1` normalization rather than a second implementation.

Files: `internal/store/migrations.go`, `internal/store/providers.go`,
`internal/provider/service.go` (validation only).

Acceptance: an existing database opens with every provider reading back with no
wrapper and unchanged behavior; the same URL normalizes identically whether
stored as an endpoint or as a wrapper, for each client; setting a wrapper on
`official` creates no credential, secret row, or key file.

Done (2026-07-26): migration 15 adds nullable `providers.wrapper_url` and
`provider_selections.via_wrapper` (route snapshot, default `0`); the built-in
`official` wrapper URL reuses the existing generic `settings` table via new
`Store.OfficialWrapperURL`/`SetOfficialWrapperURL` rather than a provider
definition row. `Store.SetProviderWrapper` is pure storage (no normalization,
no vault access). `provider.NormalizeWrapperURL` in `service.go` is the one
validation addition, delegating to the existing `NormalizeCredentialEndpoint`
with `codex=true` so a wrapper normalizes exactly like a Codex-bound
credential endpoint. `CurrentSchemaVersion` bumped 14→15 in `store.go` (not
separately listed in Files, but required for the migration to take effect).
`cmd/agentdeck/main_test.go`'s `TestStateMigrateTextAndJSONUpgradeSchema12`
fixture needed a matching rollback of the two new columns before re-running
migrations from a simulated v12 state; updated alongside.

Verification: `go test -mod=vendor ./internal/store/... ./internal/provider/...
-run 'Wrapper|V15'` (new targeted tests, all pass) plus
`go test -mod=vendor ./...` (710 passed, 0 failed) and `go vet -mod=vendor
./...` (clean) — both run once after the final edit. Migration-execution
check (L3): `TestV15MigrationAddsProviderWrapperURLAndSelectionRouteWithoutSideEffects`
replays `testdata/agentdeck-v6.sql` through `Open`, asserting unchanged
provider/snapshot reads, then exercises `SetProviderWrapper` and
`Set/OfficialWrapperURL` round-trips with `provider_credentials`/
`credential_secrets` row counts and `credential.key` absence checked before
and after.

Review Round 1 (2026-07-26): `REOPEN`, five findings, no correctness or
security defect. `Dev` unticked until they close.
Review Round 2 (2026-07-27): `PASS` — all five closed with tests that fail on
regression; two non-blocking nits carried forward (`json:"-"` on
`store.Provider.WrapperURL`, folded into `cli-route-surface`; a stale
`docs/README.md` sentence, folded into the next status-doc sync). See
`docs/reviews/provider-wrapper-routing/wrapper-schema.md`.

Fix round (2026-07-26), closing all five Round 1 findings:
- [P2] Documented the normalization obligation directly on
  `SetProviderWrapper`, `SetOfficialWrapperURL` (`providers.go`), and
  `NormalizeWrapperURL` (`service.go`): all three now say in the doc comment
  that the store setters are pure storage with no normalization of their own,
  so every caller must normalize a non-empty `url` through
  `NormalizeWrapperURL` first. Added the matching acceptance clause to
  `cli-route-surface`.
- [P2] Corrected `cli-design.md`'s "Provider Wrappers" section and its v15
  changelog row from "normalized exactly like credential endpoints" to
  "always normalized like a Codex-bound credential endpoint, regardless of
  which clients the provider actually serves" (v15 is unreleased, so the row
  was revised in place rather than adding a new version). Added
  `TestNormalizeWrapperURLDiffersFromClaudeOnlyCredentialEndpointOnV1Input` in
  `provider_test.go`, pinning that a `/v1`-suffixed wrapper URL diverges from
  `NormalizeCredentialEndpoint(url, false)` (the Claude-only form, which keeps
  the trailing `/v1`).
- [P3] `TestV15MigrationAddsProviderWrapperURLAndSelectionRouteWithoutSideEffects`
  now asserts the migrated `codex` snapshot's concrete
  `Name`/`Endpoint`/`Multiplier`/`Credential` (`legacy` /
  `https://legacy.example` / `1.5` / `default`) alongside `ViaWrapper`, so the
  six reordered `provider_selections` SELECT/`Scan` pairs are pinned, not just
  the new column.
- [nit] Removed the inert `json:"wrapper_url,omitempty"` tag from
  `store.Provider.WrapperURL`; `cli-route-surface` owns the real output field
  on `provider.Provider`.
- [nit] `NormalizeWrapperURL`'s doc comment now states the `--clear` bypass
  explicitly: a clearing caller must skip normalization and pass `""`
  straight through, since `""` is not a valid endpoint.

Verification: `go test -mod=vendor ./internal/store/... ./internal/provider/...`
(all pass, including the two new tests) plus `go test -mod=vendor ./...` (711
passed, 0 failed — one more than the prior round's 710, from the new
divergence test) and `go vet -mod=vendor ./...` (clean), run once after the
final edit. Doc changes: `git diff --check` clean on `cli-design.md` and this
plan file; manual link check on every `Provider Wrappers` / wrapper-routing
cross-reference in `docs/` found no broken relative path or stale anchor.

### `codex-writer-routes`

Teach the Codex writer the endpoint-only intent: write `base_url`, remove
`experimental_bearer_token`, keep `requires_openai_auth` and `wire_api`, and
leave every other TOML field, comment, and ordering untouched. Removing an
already-absent field is a successful no-op.

Files: `internal/provider/config.go`.

Acceptance: a config that had a bearer token loses exactly that key; a config
that never had one is byte-identical apart from the fields the intent names.

Done (2026-07-27): added `WriteCodexWrapperConfig(path, name, endpoint string)
error`, the endpoint-only intent: it writes `base_url`, removes
`experimental_bearer_token`, and leaves `requires_openai_auth`, `wire_api`,
and every other TOML field, comment, and ordering untouched — matching the
general "Codex keeps `model_provider = custom`, sets `.name`" rule from
`docs/specs/cli-design.md`'s "Owned Client Configuration Fields" section.
Extracted the line-preserving rewrite loop shared with
`WriteOfficialCodexConfig` into `rewriteCodexCustomTable`, parameterized by
per-line rewrite/drop, end-of-table flush, and missing-table creation
callbacks, so the two writers share one tested traversal instead of
duplicating ~90 lines of comment-preserving TOML editing.
`WriteOfficialCodexConfig`'s own behavior and tests are unchanged — the
extraction is a pure refactor. Fixed a bug caught by the pre-existing
`WriteOfficialCodexConfig` tests during that refactor: the generic line loop
initially appended a line's trailing newline even when the per-line callback
dropped the line outright, inserting a spurious blank line after
`[model_providers.custom]`; fixed by only appending the ending when the
callback returns a non-nil replacement.

Verification: `go test -mod=vendor ./internal/provider/... -run Codex -v`
(all `WriteOfficialCodexConfig`/`WriteCodexConfig`/`WriteCodexWrapperConfig`
tests pass, including five new `WriteCodexWrapperConfig` tests covering
comment/ordering preservation, idempotency, bearer-token-only removal,
byte-identical output apart from named fields when no bearer token was
present, custom-table creation, and atomic-replace failure leaving original
bytes) plus `go test -mod=vendor ./...` (all packages pass) and
`go vet -mod=vendor ./...` (clean), run once after the final edit. `gofmt -l`
clean on the changed files.

Review Round 1 (2026-07-27): `REOPEN`, one P2 + one P3 + one nit. The new
wrapper writer is correct and well covered, but the accompanying extraction is
not the "pure refactor" this note claims: the old loop's per-occurrence
owed-`name` reset was dropped, so on an array-of-tables source
`WriteOfficialCodexConfig` now leaves the second `[[model_providers.custom]]`
without a `name` where it previously wrote one — and the helper's own doc
comment states the contract that was dropped. `Dev` unticked until that
closes. See `docs/reviews/provider-wrapper-routing/codex-writer-routes.md`.

Fix round (2026-07-27), closing the Round 1 findings:
- [P2] Restored the dropped per-occurrence reset: `rewriteCodexCustomTable`
  now takes an `onEnter func()` callback, invoked once per occurrence of
  `[model_providers.custom]` (including each element of an array-of-tables
  source) before any of that occurrence's lines are processed. Both callers'
  `onEnter` closures reset their owed-field flags (`nameSeen` in
  `WriteOfficialCodexConfig`; `nameSeen` and `baseURLSeen` in
  `WriteCodexWrapperConfig`), matching the pre-refactor behavior the task
  note incorrectly claimed was already preserved. Added
  `TestWriteOfficialCodexConfigResetsOwedNameAcrossArrayOfTablesOccurrences`
  and
  `TestWriteCodexWrapperConfigResetsOwedFieldsAcrossArrayOfTablesOccurrences`,
  both reproducing the reviewer's two-element array-of-tables probe and
  asserting every occurrence gets its own owed field.
- [P3] Deferred, as the reviewer allowed: recorded here rather than fixed in
  this task. `WriteCodexConfig` (the direct, credentialed path) still
  rewrites the whole file through `toml.Marshal`, so a provider switched
  between direct and `--via` produces very different diffs in `config.toml`
  even though both paths are supposed to leave unrelated fields, comments,
  and ordering untouched per `docs/specs/cli-design.md`'s "Owned Client
  Configuration Fields". `route-composition` (task 4) is the next task that
  touches this call path and should decide whether to move `WriteCodexConfig`
  onto `rewriteCodexCustomTable` or record a further-deferred decision.
- [nit] Corrected `rewriteCodexCustomTable`'s doc comment: `flush` does not
  run for a table that was never present (`ensureTable` does), and the
  comment now describes `onEnter`'s role instead of the wrong "or because the
  table was absent" clause on `flush`.

Verification: `go test -mod=vendor ./internal/provider/... -run Codex -v`
(all 17 Codex tests pass, including the two new array-of-tables regression
tests and the full pre-existing `WriteOfficialCodexConfig` suite) plus
`go test -mod=vendor ./...` (all packages pass) and `go vet -mod=vendor
./...` (clean), run once after the final edit. `gofmt -l` clean on the
changed files.

Review Round 2 (2026-07-27): `PASS`. The `onEnter` reset is invoked in the
right place relative to the previous occurrence's `flush`, both callers reset
their owed-field state, and the two new tests fail if the reset is dropped
again; the doc-comment nit is closed; the P3 is deferred onto
`route-composition` with both outcomes named. A nine-shape probe (multiple
occurrences, mixed owed states, CRLF, no trailing newline, no custom table)
matched pre-refactor behavior throughout. Two out-of-scope observations
recorded in the review log, neither introduced here: two *singular*
`[model_providers.custom]` tables are rejected by the upfront `toml.Unmarshal`
guard before the rewrite runs, and a prepended `model_provider = "custom"`
lands above a leading comment. See
`docs/reviews/provider-wrapper-routing/codex-writer-routes.md`.

### `claude-writer-routes`

Teach the Claude writer the two intents it lacks: endpoint without credential,
and neither field. The `env` object survives as an empty object when its last
owned key goes, and every unowned key inside and outside `env` is carried
through unchanged.

Files: `internal/provider/config.go`.

Acceptance: a settings file carrying unrelated `env` entries and unrelated
top-level settings keeps all of them across every intent. This is the task that
protects against a whole-`env` rewrite.

Done (2026-07-27): `WriteClaudeConfig` now treats an empty `ClientConfig.Endpoint`
or `ClientConfig.Credential` as "remove this owned key" rather than writing an
empty string, so the same function expresses all three intents (endpoint +
credential, endpoint without credential, neither field) purely from the fields
the caller passes. `document["env"]` is still assigned by reference before any
key is deleted, so the `env` object survives as `{}` when its last owned key is
removed, and every unowned key inside and outside `env` is untouched because
only the two owned keys are ever deleted or set.

Verification: `go test -mod=vendor ./internal/provider/... -run Claude` (new
`TestWriteClaudeConfigEndpointWithoutCredentialRemovesTokenKeepsUnrelated`,
`TestWriteClaudeConfigNeitherFieldRemovesBothKeepsEnvObjectAndUnrelated`,
`TestWriteClaudeConfigNeitherFieldKeepsEnvObjectWhenLastOwnedKeyGoesAndEnvWasEmpty`,
plus the existing `TestWriteClaudeConfigPreservesUnmanagedFields`, all pass)
plus `go test -mod=vendor ./...` (all packages pass) and `go vet -mod=vendor
./...` (clean), run once after the final edit. `gofmt -l` clean on the changed
files.

Review Round 1 (2026-07-27): `REOPEN`, two P2 + two P3 + one nit, no
correctness or security defect on any currently reachable path. `Dev` unticked
until the two P2 items close: `WriteClaudeConfig` still creates `env: {}` in a
settings file that never had an `env` key (contradicts the spec's "never writes
any other field" and is untested), and the empty-string-means-remove sentinel is
undocumented on the function.
Review Round 2 (2026-07-27): `PASS` — both P2 findings closed with tests that
fail on regression, the closed P3 goes beyond what was asked (a `null` and an
array `env` also survive now), and the deferred P3 landed on
`route-composition` with a required test. Three non-blocking nits carried
forward: the one documented destructive case (non-map `env` replaced when an
owned key must be written) has no test, `config.go:321` says "unowned" where it
means "untouched", and this task's original Verification paragraph still names
the pre-rename test. See
`docs/reviews/provider-wrapper-routing/claude-writer-routes.md`.

Fix round (2026-07-27), closing both Round 1 P2 findings and disposing the two
P3s and the nit:
- [P2] `WriteClaudeConfig` no longer creates `document["env"]` when it is
  absent and neither owned key is being written: the write now only touches
  `env` when it is already a `map[string]any` or an owned key must be set,
  so the "neither field" intent on a source with no `env` key leaves the
  document unchanged apart from re-serialization. Added
  `TestWriteClaudeConfigNeitherFieldWithoutExistingEnvLeavesDocumentUnchanged`
  pinning that no `env` key is created and unrelated top-level keys survive.
- [P2] Documented the empty-string-means-remove sentinel directly on
  `WriteClaudeConfig`'s doc comment, naming all three intents it expresses
  and matching the documentation style already used on
  `WriteOfficialCodexConfig`/`WriteCodexWrapperConfig`.
- [P3, closed] The non-`map` `env` value is no longer destroyed by a
  no-owned-key write: the same absent-or-non-map guard that fixes the P2
  above also leaves a non-object `env` (e.g. a string) untouched when
  neither owned key is being written, closing the "loss is now total" gap
  Round 1 flagged. A write that does set an owned key still overwrites a
  non-object `env` with a fresh map, unchanged from the pre-existing
  behavior Round 1 confirmed was not newly introduced. Added
  `TestWriteClaudeConfigNeitherFieldLeavesNonObjectEnvUntouched`.
- [P3, deferred] Design asymmetry between the two writer tasks (Codex names
  its wrapper intent as a separate function; Claude overloads one function
  on empty-string sentinels) is not closed here — Round 1 confirmed no live
  caller can reach the new semantics by accident today, so this stays a
  `route-composition` decision: that task must decide each write's intent
  explicitly rather than letting a zero-value lookup decide it, and must
  carry a test for that. Recorded as a `route-composition` acceptance
  consideration, not a `claude-writer-routes` defect.
- [nit] Renamed
  `TestWriteClaudeConfigNeitherFieldKeepsEnvObjectWhenLastOwnedKeyGoesAndEnvWasEmpty`
  to `TestWriteClaudeConfigNeitherFieldKeepsEnvObjectWhenEnvHeldOnlyOwnedKeys`,
  matching what the fixture actually holds.

Verification: `go test -mod=vendor ./internal/provider/... -run Claude -v`
(all seven `WriteClaudeConfig` tests pass, including the two new ones) plus
`go test -mod=vendor ./...` (all packages pass) and `go vet -mod=vendor
./...` (clean), run once after the final edit. `gofmt -l` and
`git diff --check` clean on both changed files.

### `route-composition`

Compose the two rules at switch time: the credential field comes from the
selected provider alone, the endpoint field from the route. `--via` writes the
provider's wrapper URL; without it the switch is direct. `official` dispatches
to both clients' restore paths and skips credential resolution. The selection
snapshot records the route taken and the endpoint actually written.

Files: `internal/provider/service.go`, `internal/provider/config.go`.

Acceptance: switching a custom provider between direct and `--via` changes only
the endpoint field and leaves the written credential byte-identical;
`provider use official --client claude` completes and records a snapshot;
`--client codex` behavior is unchanged when no wrapper is set.

Carried from `claude-writer-routes` Round 1 (P3, deferred, not closed there):
every call into `WriteClaudeConfig` must decide its endpoint/credential intent
explicitly — an empty `ClientConfig` field is the writer's "remove this key"
sentinel, so a credential or endpoint lookup that can return `""` for a reason
other than "this field should be absent" must not be passed through directly.
Cover this with a test.

Done (2026-07-27): `Service.UseCredential` gained a trailing `via bool`
parameter (`Service.Use` still defaults to `false`, so its one caller in
`cmd/agentdeck/main.go` is unaffected until `cli-route-surface` adds the
flag). `official` no longer rejects `--client claude`; `officialProvider()`'s
`Clients` now lists both `codex` and `claude`, and the codex-only guard was
replaced with a guard that rejects a credential name for `official` instead
(official never resolves one). Endpoint composition is one small block per
name/client/route combination, each naming its own intent at its own call:
for `official`, direct calls the unchanged `WriteOfficialCodexConfig` or
`WriteClaudeConfig(ClientConfig{})` (both fields removed) and `--via` calls
`WriteCodexWrapperConfig` or `WriteClaudeConfig(ClientConfig{Endpoint: wrapper})`
(endpoint only, credential vault never touched — asserted with
`rejectingCredentialVault`); for a custom provider, both routes call the
existing `WriteCodexConfig`/`WriteClaudeConfig` with the selected credential
unchanged and only the `Endpoint` argument switched between the credential's
own endpoint and the provider's `wrapper_url`, so credential and endpoint
never come from two different code paths. `via` is validated before any
config read: requesting it with no wrapper configured on the resolved route
(the provider's `wrapper_url` for a custom provider, `Store.OfficialWrapperURL`
for `official`) fails with `ErrInvalidProvider` before the fingerprint check or
redacted backup, so no client file is touched. The completed selection
snapshot now always carries `ViaWrapper` and `EndpointSnapshot` from the
endpoint actually written (previously `EndpointSnapshot` was only set for a
non-`official` selection and there was no route field at all), covering
`official --via`'s snapshot too.

Decision on the P3 carried from `codex-writer-routes` (whether `WriteCodexConfig`
should move onto `rewriteCodexCustomTable`): deferred again, not migrated.
The stated acceptance — direct vs. `--via` on a custom provider changes only
the endpoint and leaves the credential byte-identical — is met structurally,
because both routes now call the exact same `WriteCodexConfig` with only the
`Endpoint` argument differing, which trivially keeps `experimental_bearer_token`
identical without needing comment/ordering preservation. `WriteCodexConfig`
still reserializes the whole file via `toml.Marshal` (unlike the line-preserving
`rewriteCodexCustomTable` path `WriteCodexWrapperConfig`/`WriteOfficialCodexConfig`
use), so a custom-provider switch still produces a different `config.toml` diff
shape than an `official` switch touching the same file. Migrating it is a
larger refactor (it currently forces five fields — `name`, `base_url`,
`requires_openai_auth`, `experimental_bearer_token`, `wire_api` — where the
shared helper's existing callers only ever force two) and isn't required by
this task's acceptance or invariants, so it is left as an explicitly-recorded
open decision rather than attempted here.

Verification: `go test -mod=vendor ./internal/provider/... -run
'RouteComposition|UseCustomProvider|UseOfficial|UseViaWrapper' -v` (new
`TestUseCustomProviderCodexViaWrapperChangesOnlyEndpointKeepsCredentialByteIdentical`,
`TestUseCustomProviderClaudeViaWrapperChangesOnlyEndpointKeepsCredentialByteIdentical`,
`TestUseOfficialCodexViaWrapperWritesEndpointRemovesCredentialAndRecordsSnapshot`,
`TestUseOfficialClaudeDirectAndViaWrapperDecideIntentExplicitly` (closes the
carried Claude-intent P3),
`TestUseViaWrapperWithoutConfiguredWrapperFailsBeforeWritingClientFile`, all
pass) plus `go test -mod=vendor ./internal/provider/...` (all pass, including
the updated `TestOfficialProviderIsBuiltInAndDefinitionReadsDoNotAccessSecrets`
and the nine existing `UseCredential` call sites updated for the new trailing
argument) plus `go test -mod=vendor ./...` and `go vet -mod=vendor ./...`, run
once after the final edit. `go test -mod=vendor ./cmd/agentdeck/...` has two
pre-existing failures unrelated to this change
(`TestIsolatedEndToEndFlow`, `TestSessionShowActivityReadsOnlySafeMetadataOnDemand`),
confirmed present on the unmodified tree via `git stash`/rerun before this
task began. `gofmt -l` and `git diff --check` clean on every changed file.

Review Round 1 (2026-07-27): `REOPEN`, one P1 + one P2 + three P3 + one nit.
`Dev` unticked. The three named acceptance criteria are met on the write path;
the gap is on the read-back path this task's new states made reachable.
`Service.ConfigDrift` (`service.go:127-131`) branches on `snapshot.Official`
alone and calls the TOML-parsing `ConfigMatchesOfficialCodex` for every
official selection, so `provider use official --client claude` — reachable
from the CLI today and correct in the file it writes — makes `agentdeck doctor`
report permanent `provider_config_drift` whose offered recovery cannot clear
it (reproduced end to end), and `official --via` on Codex will drift the same
way once task 5 exposes the flag, because that matcher requires no `base_url`.
See `docs/reviews/provider-wrapper-routing/route-composition.md`.

Fix round (2026-07-27), closing the P1 and P2 and disposing the three P3s and
the nit:
- [P1 + P2] `ConfigDrift` now branches on client and route, not on
  `snapshot.Official` alone. Two matchers were added next to the existing
  `ConfigMatchesOfficialCodex`: `ConfigMatchesOfficialClaude` (neither owned
  `env` key present, which is what a direct official Claude switch writes) and
  `ConfigMatchesOfficialWrapper` (the wrapper endpoint written and no
  credential, for either client). They are separate functions rather than one
  matcher taking a possibly-empty endpoint, because "no endpoint at all" and
  "exactly this wrapper endpoint" are two different states to prove. The four
  routes now map one-to-one onto the four branches of one `switch`, and the
  custom branch is unchanged. Added
  `TestOfficialClaudeSelectionIsNotReportedAsConfigDrift`,
  `TestOfficialCodexViaWrapperSelectionIsNotReportedAsConfigDrift`,
  `TestOfficialClaudeViaWrapperSelectionIsNotReportedAsConfigDrift`, and
  `TestCustomProviderViaWrapperSelectionIsNotReportedAsConfigDrift`. Each of
  the first three asserts drift `0` after the switch and drift `1` after an
  external edit, so none can be satisfied by a matcher that accepts anything;
  all three were confirmed to fail with `drift = 1, want 0` against the
  pre-fix branch.
- [P3, closed] The custom route no longer passes a possibly-empty lookup into a
  writer whose empty field means "remove this owned key": `UseCredential` now
  rejects an empty written endpoint or empty decrypted secret for a custom
  provider, before the operation row, the redacted backup, and any client
  write. Added `TestUseCustomProviderWithEmptyStoredEndpointFailsBeforeTouching
  ClientFile`, which forces the unreachable row directly through SQL and
  asserts `ErrInvalidProvider`, a byte-identical client file, and no recorded
  selection; without the guard it fails with a silent success.
- [P3, closed] The wrapper lookup no longer tests `err` outside the branch that
  assigns it. Official wrapper resolution uses its own `wrapperErr`, and the
  written endpoint is now resolved once, before the operation begins, instead
  of being recomputed inside the write block.
- [P3, closed] `officialProvider()` now reports
  `authentication: client_existing_login` instead of `codex_existing_login`,
  with a doc comment recording why the built-in provider's authentication mode
  is client-neutral. This changes one user-visible field of
  `provider show official` — deliberately, because the same task made that
  provider selectable for Claude, and the plan's "default behavior is
  unchanged" invariant covers unchanged providers and invocations, not the
  built-in provider's own description. No spec or manual text names the value.
- [nit] Corrected this task's Done note: the Claude official branch is now two
  explicit calls (`ClientConfig{}` for direct, `ClientConfig{Endpoint: wrapper}`
  for `--via`) rather than one call whose intent came from a zero value, which
  is what the note already claimed.

Verification: `go test -mod=vendor ./internal/provider/... -run Drift -v`
(four new drift tests pass) and `-run
TestUseCustomProviderWithEmptyStoredEndpointFailsBeforeTouchingClientFile`
(passes), each also run against a temporarily reverted fix to confirm it fails
without it; plus `go test -mod=vendor ./...` and `go vet -mod=vendor ./...`,
run once after the final edit. The two pre-existing `./cmd/agentdeck/...`
failures recorded above (`TestIsolatedEndToEndFlow`,
`TestSessionShowActivityReadsOnlySafeMetadataOnDemand`) are still present and
still unrelated. `gofmt -l` and `git diff --check` clean on every changed file.
The P1 reproduction from Round 1 was re-run against a rebuilt binary: the same
`provider use official --client claude` now leaves `agentdeck doctor` reporting
`provider_configuration: ok`.

Review Round 2 (2026-07-27): `PASS`, `Review` ticked. The P1 and P2 are closed
at the root cause, all three P3s and the nit are closed rather than deferred,
and no new finding at or above nit level was found. The re-review re-derived
each finding from source rather than from this note, reproduced the RED state
of every new test by reverting the corresponding fix in an out-of-tree copy
(three drift tests and the empty-endpoint test all fail without their fix; the
custom `--via` drift test passes either way and is a pin by design, as its own
comment states), confirmed the four `ConfigDrift` branches map one-to-one onto
the four route combinations in `cli-design.md:646-649` with no gap and no new
false positive, and confirmed the `client_existing_login` rename breaks no
output contract — the value appears in no spec, manual, or fixture, and the one
JSON fixture naming the field declares only its type. The two `./cmd/agentdeck`
failures were independently confirmed pre-existing at `00b6803` via `git stash`
in the copy. See `docs/reviews/provider-wrapper-routing/route-composition.md`.

### `cli-route-surface`

Surface it on the existing nouns: `provider set-wrapper <provider>
--url <url>|--clear`, `provider use --via`, the wrapper URL in
`provider list|show` and their JSON as an additive `wrapper_url`, the route and
written endpoint in `provider current` and `provider status`, and the effective
route line on every successful switch. Update `docs/specs/cli-manual.md` in the
same task, since it documents the implemented surface.

Files: `cmd/agentdeck/main.go` provider commands, `internal/provider/service.go`,
`docs/specs/cli-manual.md`. (There is no `internal/cli` package; the provider
subcommands live in `cmd/agentdeck/main.go`.)

Acceptance: `--via` without a configured wrapper fails before any client file is
touched; every existing invocation without a new flag behaves exactly as before;
`provider set-wrapper --url` must normalize through `NormalizeWrapperURL`
before any write reaches `SetProviderWrapper`/`SetOfficialWrapperURL`, since
those store methods perform no normalization themselves — `--clear` is the
one path that skips `NormalizeWrapperURL` and writes `""` straight through.

Done (2026-07-27): `provider set-wrapper <name> --url|--clear` and
`provider use --via` are on `providerCmd` in `cmd/agentdeck/main.go`. The
command layer decides the intent (`--url` and `--clear` are mutually exclusive
and one is required, both rejected as `inputError`/exit 2) and
`Service.SetWrapper(ctx, name, url, clear)` performs the write, taking `clear`
as its own argument rather than overloading an empty url: `NormalizeWrapperURL`
runs on the `--url` path before any store call, and `--clear` writes `""`
straight through, which is the one value normalization would reject.
`official` routes to `SetOfficialWrapperURL`, every other name to
`SetProviderWrapper`. Reporting is additive: `Provider.WrapperURL`
(`wrapper_url`, omitempty) is filled from the stored row for custom providers
and from `Store.OfficialWrapperURL` for the built-in one, so `provider
list|show|status` all carry it; `CurrentSelection` and `ActiveSelection` gained
`via_wrapper` and `endpoint` (omitempty) straight from the selection snapshot,
which is the only place the route survives a write. Text output follows the
existing renderers: a `WRAPPER` column on `provider list`, a `wrapper:` line on
`provider show` only when one is configured, and `ROUTE`/`ENDPOINT` columns
appended after `SELECTED AT` on `provider current` and the per-client table of
`provider status <name>` (appended, so the existing header-prefix assertions
still hold). A successful switch prints one `effective route: <client>
direct|via wrapper, endpoint <url>` line — `no endpoint written` for a direct
official switch — to stderr, read back from the recorded selection rather than
from the requested flag, suppressed by `--quiet`, absent from the JSON envelope,
and dropped rather than failing the command if the read-back errors.

Verification: new `cmd/agentdeck/provider_route_surface_test.go` (7 tests:
normalize/store/clear, the two rejected intent combinations, official's separate
storage, `--via` endpoint-only switching with route read-back in JSON and text,
the acceptance criterion that `--via` without a wrapper fails before touching
the client file, stderr/`--quiet`/JSON boundaries, and the official-Claude
`no endpoint written` line) plus `go test -mod=vendor ./...` and `go vet
-mod=vendor ./...`. `cmd/agentdeck/testdata/phase7/gui-json-contract.json`
gained the `provider.set-wrapper` leaf command and contract and the two new
`provider.current` fields; both hand-written schemas were verified against real
command output, because `TestIsolatedEndToEndFlow` fails earlier for an
unrelated pre-existing reason (`e2e_test.go:141`, provider-filtered stats return
0 events) and never reaches its contract comparison, so
`UPDATE_AGENTDECK_GOLDEN=1` cannot regenerate the fixture today. The e2e flow
does now call `provider set-wrapper`, so regeneration will cover it once that
failure is fixed. The two known pre-existing `./cmd/agentdeck` failures are
unchanged.

Review Round 1 (2026-07-27): no blocking finding; the three named acceptance
criteria hold and three independent revert checks confirmed the new tests fail
without their fix. Three improvement findings (coverage of the new JSON fields,
a stale column enumeration in `cli-design.md`, and silent normalization in
`set-wrapper`'s text output) plus one recorded P3.

Fix round (2026-07-27):
- [coverage] The golden flow now sets the wrapper *before* the definition reads
  and re-reads `provider status` after both switches, so `wrapper_url` and the
  selection's `via_wrapper`/`endpoint` are captured rather than structurally
  excluded; the switch it captures is now `provider use phase7 --via`. Added
  `TestProviderDefinitionJSONCarriesWrapperURLForBothProviderKinds` (custom and
  built-in wrapper reporting across `show`/`list`/`status` JSON, plus the field
  disappearing on `--clear` without touching the other provider) and
  `TestProviderStatusJSONReportsSelectionRoute` (the active-selection struct,
  which carries the route through different code than `provider current`).
  `gui-json-contract.json` was regenerated with `UPDATE_AGENTDECK_GOLDEN=1` in
  an out-of-tree copy whose only patch was neutering the pre-existing
  `e2e_test.go` stats assertion that aborts the flow early; only the four
  changed `provider.*` entries were merged back. The regenerated `usage.stats`
  entry was deliberately **not** taken: it collapses to empty `clients`,
  `models`, and `providers` arrays because of that same pre-existing failure,
  and taking it would bake the breakage into the golden. Re-running the golden
  comparison against the merged fixture leaves exactly one differing entry,
  `usage.stats`, which is that known failure and not this task's.
- [spec] `cli-design.md`'s `provider list` column enumeration now includes the
  wrapper URL, matching the same section's existing statement that a wrapper
  appears in `provider list|show`.
- [text output] Kept the mutation text as-is instead of echoing the stored
  wrapper. `provider add --endpoint` and `provider update --endpoint` normalize
  exactly as silently and also print only the action and resource name, so
  echoing here would make `set-wrapper` the single exception to the documented
  mutation-text shape for a rule that applies to three commands. Documented the
  rewrite in `cli-manual.md` instead, pointing at `provider show` and
  `--format json`, both of which return the stored value.
- [P3, recorded not fixed] The effective-route line reads the completed
  selection back through `Service.Current` rather than having `UseCredential`
  return the route it wrote, so a concurrent switch of the same client by
  another process could make the line describe that other selection. The window
  is tiny for a single-user local CLI, the line is informational, and closing it
  means changing a `UseCredential` signature that `route-composition` already
  passed re-review, so it is left as a known trade-off.

Review Round 2 (2026-07-27): `PASS`, `Review` ticked. All three findings closed;
the golden was independently regenerated and matches real output on every
`provider.*` entry, with `usage.stats` confirmed byte-identical to `HEAD` so the
pre-existing failure keeps its signal, and two of the new tests were confirmed
RED against a reverted implementation. Four nits recorded, none requiring
action. See `docs/reviews/provider-wrapper-routing/cli-route-surface.md`.

### `switch-advisories`

Two stderr advisories on a successful Claude switch: running Claude sessions
should be restarted, and any conflicting credential source AgentDeck does not
own (`env.ANTHROPIC_API_KEY`, `apiKeyHelper`) that would override an `official`
selection. Advisory only — no exit-status or JSON-envelope change, and no
removal of unowned fields.

Files: `internal/provider/service.go`, `cmd/agentdeck/main.go`.

Acceptance: the advisory appears on stderr, the JSON envelope is byte-identical
to a run without it, and no unowned field is written or deleted.

Done (2026-07-27): `provider.ClaudeCredentialConflicts(path)` names the unowned
sources present in a Claude settings file — `env.ANTHROPIC_API_KEY` and
`apiKeyHelper`, in that fixed order — returning key names only, because one of
the two holds a credential. A key set to `null` or to a blank string configures
no credential and is not reported, so the advisory never fires on something
that overrides nothing. `Service.SwitchAdvisories(client, name, configPath)`
composes what a completed switch carries: nothing for Codex, the restart note
for every Claude switch, and the conflict notes ahead of it only when the
selection is `official`, which is the selection an unowned source overrides. It
resolves the managed settings path itself when the CLI passed no
`--config-path`, and it never fails a switch that already succeeded — an
unreadable, unparsable, or unresolvable settings file drops the conflict note
and keeps the restart note. `cmd/agentdeck/main.go` prints them through
`reportSwitchAdvisories` with an `advisory: ` prefix under exactly the rules
the effective-route line already follows: stderr only, never the JSON envelope,
no exit-status effect, suppressed by `--quiet`. Nothing removes or rewrites an
unowned field; the writer behavior from `claude-writer-routes` is untouched.

Review Round 1 (2026-07-27): no blocking finding; the three acceptance criteria
hold and three independent revert checks confirmed the new tests fail without
their fix. One code finding (the value test reported malformed non-string
values) and two scope findings, fixed and recorded below.

Fix round (2026-07-27):
- [value test] `configuresCredential` now reports exactly one shape: a
  non-empty string. Both keys are string-valued to Claude — an env value and a
  helper command line — so `null`, `""`, a bool, a number, an object, and an
  array either configure nothing or are malformed for the key they sit on, and
  Claude can derive no credential from any of them; the previous
  `default: true` contradicted the function's own stated rule. The blank
  string `" "` is now **reported** rather than trimmed away: it is non-empty to
  Claude, which will use it and fail to authenticate, which is exactly the
  confusing state the advisory explains. Twelve table cases cover the silent
  shapes and one test covers the blank-but-present one; all of them were
  confirmed to fail against the pre-fix `default: true`/`TrimSpace` shape.
- [docs] `cli-manual.md` now states the detection boundary explicitly, so a
  missing advisory is not read as "no conflict".

Scope decisions (recorded, deliberately not implemented this round):
- **A custom-provider selection is not checked for `env.ANTHROPIC_API_KEY`.**
  `cli-design.md:665-670` scopes the conflict advisory to a switch that selects
  `official`, and this task implements that scope exactly. The risk on the
  custom route is real but different: AgentDeck writes `ANTHROPIC_AUTH_TOKEN`
  (`Authorization: Bearer`) while a leftover `ANTHROPIC_API_KEY` travels as
  `x-api-key`, so both reach the upstream and which one wins is the upstream's
  decision — see `## Out of Scope`, which already records that this design
  matches Bearer first. Advising on it would mean claiming an override that
  AgentDeck cannot actually predict. Extending it would be a spec change to
  that paragraph first, not an implementation change here.
- **A credential exported into the shell environment is not detected.**
  `export ANTHROPIC_API_KEY=...` overrides an `official` selection just as a
  settings entry does, but the spec sentence names the settings key, and the
  environment AgentDeck's own process sees is not necessarily the environment
  the user's Claude client will run under, so reading `os.Getenv` here would
  produce both false positives and false negatives. Documented as a boundary in
  `cli-manual.md` instead.
- **Only the settings file AgentDeck manages is inspected.** Claude resolves
  settings from more than the one user-level file this project models, so a
  credential parked in another scope is invisible to the advisory for the same
  reason the shell environment is: AgentDeck writes exactly one Claude file and
  reads exactly that file back. Widening detection would mean adopting Claude's
  whole settings-resolution order, which is a client behavior this project does
  not track and which would go stale. Documented as a boundary in
  `cli-manual.md`, worded without pinning a file list.

Decision on `--quiet`: advisories are suppressed, like the effective-route
line. They are informational by specification, the JSON envelope a script reads
is unchanged either way, and a script can detect the same conflict itself. A
`--quiet` run that stayed noisy for one of the two stderr lines would be the
odd one out.

Verification: new `internal/provider/switch_advisories_test.go` (detector
naming both sources without revealing either value, the four
"configures nothing" shapes, Claude-only scope with conflicts scoped to
`official`, survival of an unreadable/unparsable/unresolvable settings file,
and default-path resolution) and
`cmd/agentdeck/provider_switch_advisories_test.go` (the acceptance case:
advisories on stderr while both unowned sources and both unrelated fields
survive byte-for-byte and both owned keys are gone; the JSON envelope of a
conflicted switch compared field-for-field against a clean one, differing only
in `generated_at`; and advisory scope plus `--quiet` suppression across custom
Claude, Codex, and official switches). Plus `go test -mod=vendor ./...` and
`go vet -mod=vendor ./...` after the final edit; the two known pre-existing
`./cmd/agentdeck` failures are unchanged.

Review Rounds 2-3 (2026-07-27): Round 2 closed all three Round 1 findings and
opened one documentation finding (the boundary list named the shell environment
as the only unchecked source); Round 3 closed it and ticked `Review`. The
converged detection rule was independently reproduced in its RED state, and
neither the Claude writers nor `ConfigMatchesOfficialClaude` was disturbed —
they test disjoint keys with deliberately different predicates. See
`docs/reviews/provider-wrapper-routing/switch-advisories.md`.

### `usage-route-metadata`

Carry the route into attribution as reported metadata. The provider dimension
keys on provider name only, so `--provider <name>` selects an account's events
whether the route was wrapped or direct, and subscription traffic through a
proxy stays under `official` at multiplier `1`.

Files: `internal/store/providers.go`, `internal/usage/usage.go`.

Acceptance: every existing stats and summary contract is unchanged for
selections with no wrapper.

Done (2026-07-27): the route reaches attribution as one additive count on the
provider dimension, `StatsDimension.WrapperEvents` (`wrapper_events`,
omitempty), and nothing else. The route and the provider always come from the
same instant: an estimated event takes both from the session-start snapshot
that already chose its provider, and an exact run-bound event — whose run
records a provider name but no route — takes its route from the snapshot at
the **run start**, the moment the run pinned that provider. If the snapshot at
that instant names a different provider (the run spanned a provider switch),
the route is left unreported instead of guessed: under-reporting is
recoverable, mis-attributing a route is not. The grouping
key is untouched: the provider dimension still keys on client plus provider
name, so wrapped and direct events share one row, `--provider <name>` selects
both, and wrapped `official` traffic stays under `official` at multiplier `1`.
`internal/store/providers.go` needed no change — `ProviderSnapshot.ViaWrapper`
and `SnapshotAt` already carried the route from `wrapper-schema`. Text follows
the same additive rule: `PROVIDERS` rows gain a `N via wrapper` secondary only
when a wrapper carried events, so a report with no wrapper renders exactly as
before. `Summary` has no provider dimension and is unchanged.

Review Round 1 (2026-07-27) and its fix round: the review reproduced a case the
first cut got wrong — a session spanning a route change on one provider. The
route was read from the session start while the provider came from the run, so
a session that opened wrapped and ran direct **over-reported**, contradicting
the "under-report only" guarantee the code claimed. Fixed by selecting
`usage_runs.started_at` into `storedEvent.runStart` and reading an exact
event's route there. The review also found the route cost a second
`SnapshotAt` per event (a linear scan over operations and selections), doubling
the timeline work and adding it to exact events that previously did none;
`priceForEvent` now returns one `eventAttribution` carrying price, multiplier,
quality, provider, and route, so the aggregation reuses what it already
resolved and every event costs at most one lookup.

Verification: new `internal/usage/route_metadata_test.go` (one row spanning
both routes with only the count distinguishing them; `--provider` selecting
both routes; wrapped `official` staying one row at multiplier `1`;
`wrapper_events` absent from every dimension's JSON when no wrapper is in play;
both directions of a session that spans a route change, each confirmed to fail
when the route is read from the session start again; and the deliberate
under-report when a run and the snapshot at its start name different providers)
plus `cmd/agentdeck/usage_route_text_test.go`, which
asserts the text annotation appears only with wrapped events and that removing
it recovers the original rendering byte for byte. Plus
`go test -mod=vendor ./...` and `go vet -mod=vendor ./...` after the final edit.

The golden's `usage.stats` provider element gained `wrapper_events: "number"`.
That is the fix confirming itself end to end: the flow's `provider use phase7
--via` now reaches an exact run-bound event through the run-start snapshot,
where the first cut — reading the session start, which predates the switch —
reported nothing. `TZ=UTC go test ./cmd/agentdeck/` passes with that one entry
updated, contract comparison included.

Review Rounds 1-2 (2026-07-27): Round 1 found the route-precision defect and
the duplicated timeline scan recorded above; Round 2 confirmed both closed,
re-derived the scan alignment of both event queries, checked the
`eventAttribution` refactor for semantic drift, and reproduced the `TZ=UTC`
pass with `-count=1`. `Review` ticked. Two nits recorded, neither actionable.
See `docs/reviews/provider-wrapper-routing/usage-route-metadata.md`.

Diagnosis of the two long-standing `./cmd/agentdeck` failures (recorded here
because they sit in this task's area and were repeatedly excluded as
"pre-existing"): they are **timezone-dependent test fixtures, not product
bugs**. `usage sessions` in the end-to-end flow reports the synthetic event at
`2026-07-14T00:00:01Z`, but `usage stats --from 2026-07-14` resolves the range
in the machine's local zone, so on a `America/Los_Angeles` host the window
starts at `2026-07-14T07:00:00Z` and excludes it, leaving every dimension
empty. Running `TZ=UTC go test ./cmd/agentdeck/ -run
'TestIsolatedEndToEndFlow|TestSessionShowActivityReadsOnlySafeMetadataOnDemand'`
passes both. The provider dimension and the `--provider` filter are not
implicated. The `TZ=UTC` run is stronger evidence than "the fixture could be
regenerated": `TestIsolatedEndToEndFlow` ends in `assertCommandContracts`, so
passing under `TZ=UTC` proves the committed golden — including the `provider.*`
entries merged by hand during `cli-route-surface` — already matches observed
output exactly, and that the earlier refusal to bake the empty `usage.stats`
entry into it was right. Fixing the fixtures is a separate task, not this one.

## Out of Scope

- Account, plan, or subscription switching, and any OAuth token handling. The
  backlog item about switching a Claude account stays open and is not addressed
  here; selecting `official` returns the client to whatever login it already
  holds.
- Validating that a wrapper reaches the upstream its provider names, or that a
  relay forwards what a client's own authentication needs. AgentDeck writes
  configuration; it does not probe endpoints.
- A chain of proxies, a wrapper record shared across providers, or a per-client
  wrapper URL. The client holds one address, one instance fronts one upstream,
  and one instance serves both client protocols.
- Client settings beyond the two owned transport fields per client. A proxy
  backend that needs extra client variables, such as the Bedrock recipe's
  `CLAUDE_CODE_USE_BEDROCK=0`, is outside this plan.
- A credential written to `x-api-key` instead of `Authorization: Bearer`. The
  proxy this is designed for matches Bearer first.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | wrapper-schema | ✓ | ✓ |
| 2 | codex-writer-routes | ✓ | ✓ |
| 3 | claude-writer-routes | ✓ | ✓ |
| 4 | route-composition | ✓ | ✓ |
| 5 | cli-route-surface | ✓ | ✓ |
| 6 | switch-advisories | ✓ | ✓ |
| 7 | usage-route-metadata | ✓ | ✓ |

Done: **3/7 reviewed.** The implementer ticks **Dev** once a task is built and
its targeted verification passes; an independent reviewer ticks **Review** once
findings are closed, recording the round in
`docs/reviews/provider-wrapper-routing/<task-anchor>.md`. A task is done only
when Review is ticked.

Sequencing: task 1 gates everything. Tasks 2 and 3 are independent writer tasks
and both gate task 4. Task 5 lands after task 4 so no flag can expose an
unreachable state. Tasks 6 and 7 are independent tails; task 7 is the only one
that may be deferred without leaving the feature half-usable.

A usable vertical slice for the reported deployment is tasks 1, 3, 4, 5: Claude,
with a wrapper in front of either the subscription or an existing relay
credential. Task 2 extends the same model to Codex.

## Invariants

- **AgentDeck owns exactly two fields per client.** Codex
  `[model_providers.custom].base_url` and `.experimental_bearer_token`; Claude
  `env.ANTHROPIC_BASE_URL` and `env.ANTHROPIC_AUTH_TOKEN`. No task may write,
  clear, or reorder anything else — including `env.ANTHROPIC_API_KEY`, which
  overrides an `official` selection but is the user's field, to be reported and
  never removed.
- **A wrapper overrides the endpoint field and nothing else.** It never changes
  which credential is written, which multiplier applies, or which provider name
  an event is attributed to. A proxy that needs its own credential is a
  provider, not a wrapper.
- **The route is per switch, never stored as an attachment.** A configured
  wrapper must not route a switch that did not pass `--via`. Do not add a sticky
  attachment, a default-on rule, or a client-level toggle.
- **The wrapper stays provider-owned.** One nullable column. Do not promote it
  to a shared record, a chain, an ordered list, or a per-client value.
- **Default behavior is unchanged.** Every existing provider has no wrapper, and
  every existing invocation without a new flag keeps its exact current behavior,
  output, and error text.

## Starting a task

Turn any row of the Status matrix into a scoped development instruction through
its anchor:

> **进入开发:provider wrapper routing / `<task-anchor>`**
> 阅读 `AGENTS.md`、本 plan `## Tasks` 中 `<task-anchor>` 一条及它命名的文件、本
> plan 的 `## Invariants` 与 `## Required Verification`,以及
> `docs/specs/cli-design.md` v15 的 "Provider Wrappers"、"Owned Client
> Configuration Fields"、"Selecting the Built-in Provider" 三节。只在该 task 的范围
> 内实现并自测。完成后在 `## Status` 勾上该行的 `Dev`,把命令与结果记进该 task 的
> 完成注记;评审留痕到 `docs/reviews/provider-wrapper-routing/<task-anchor>.md`。

Example — `claude-writer-routes`: 阅读 `internal/provider/config.go` 的
`WriteClaudeConfig`/`WriteRedactedBackup`/`atomicPrivateReplace`,先写一个持有无关
`env` 键与无关顶层键的 fixture,再实现三种意图,确保未拥有的键在三种意图下逐字保留。

## Required Verification

L2 for tasks 1-5 and 7: they change SQLite schema, persisted provider state, or
a JSON/text output contract. Run the affected targeted tests plus
`go test -mod=vendor ./...` once after the final edit of each task.

L1 for task 6: stderr advisory only, no persisted or envelope contract change.
Targeted tests are sufficient.

Task 1 executes a migration against existing databases, which is an L3 trigger
under `AGENTS.md`; it adds the migration-execution check on an existing-database
fixture on top of L2.

No task touches concurrency, credential ciphertext formats, or the build and
installer path, so no race, cross-build, size, or `release-verify` gate is
required by this plan. Commands are listed in `AGENTS.md` under "Testing and
Verification".

## External constraints

Facts about the surrounding clients and proxy, recorded so they are not
misdiagnosed as AgentDeck defects. None of them is something AgentDeck
validates:

- A proxy carrying subscription traffic must forward `anthropic-beta` verbatim;
  it carries the OAuth capability the upstream requires, and stripping it fails
  those requests with `401`.
- With `ANTHROPIC_BASE_URL` set to a non-first-party host, Claude Code disables
  MCP tool search by default; `ENABLE_TOOL_SEARCH=true` re-enables it only if the
  proxy forwards `tool_reference` blocks.
- From Claude Code v2.1.196, a custom `ANTHROPIC_BASE_URL` disables Remote
  Control.
- Claude Code re-reads `~/.claude/settings.json` while running. Changing owned
  keys mid-session can invalidate capabilities already negotiated in that
  session, which is why task 6 exists.
- Headroom ships an opt-in watcher (`HEADROOM_CC_SWITCH_RECONCILE=1`, off by
  default) that rewrites `ANTHROPIC_BASE_URL` back to itself after another
  switcher writes the file, capturing the written endpoint as its own upstream.
  With it enabled, two tools own the same field; AgentDeck does not coordinate
  with it.
