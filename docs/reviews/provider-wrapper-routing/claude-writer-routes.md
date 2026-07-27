---
status: active
plan: provider-wrapper-routing
task: claude-writer-routes
---

# Review log — provider-wrapper-routing / claude-writer-routes

## Round 1 — 2026-07-27

- Reviewed state: base `ea1df47`, uncommitted working tree; SHA-256 of
  `git diff -- internal/provider/config.go internal/provider/config_test.go` =
  `290425d700a1ab41…`. The same working tree also carries the reviewed-and-
  passed `wrapper-schema` change and the `codex-writer-routes` change; only the
  `WriteClaudeConfig` behavior and its tests are in scope here.
- Reviewer: Claude Opus 5 (read-only pass, independent of the implementation
  session)
- Scope: `internal/provider/config.go:309-339` (`WriteClaudeConfig`'s new
  empty-string-means-remove semantics) and the three new `config_test.go`
  Claude cases. Contract checked against `docs/specs/cli-design.md` "Owned
  Client Configuration Fields". Reachability of the new semantics checked
  through `internal/provider/service.go:662-664`, the only caller.
- Findings:
  - [P2] `config.go:318-322` unconditionally materializes `document["env"]` as
    `{}` when the source has no `env` key, and that block now runs in the
    "neither field" intent, where AgentDeck writes no owned key at all.
    Verified empirically: `{"keep":true,"model":"opus"}` +
    `WriteClaudeConfig(path, ClientConfig{})` →
    `{"env":{},"keep":true,"model":"opus"}`. The spec says AgentDeck "owns
    exactly two transport fields per client and never writes, clears, or
    reorders any other field"; `env` is a container it does not own. Before
    this task the creation was a necessary consequence of always writing an
    owned key into it — now it is a gratuitous addition to a file that never
    had one. Harmless to Claude at runtime, but it is a diff in a user's
    settings file that AgentDeck was not asked to make, and no test covers a
    source without `env`. -> Only create `env` when an owned key is actually
    being written; add a test pinning "no `env` key + neither intent → file
    unchanged apart from re-serialization".
  - [P2] `config.go:309` has no doc comment, while all three Codex writers
    (`WriteCodexConfig` aside, `WriteOfficialCodexConfig:198`,
    `WriteCodexWrapperConfig:243`) document their intent. `WriteClaudeConfig`
    now encodes three distinct intents through empty-string sentinels on
    `ClientConfig.Endpoint`/`.Credential`, and nothing at the signature states
    that `""` means "delete this owned key" rather than "write an empty
    string". This is the same class of gap that Round 1 of `wrapper-schema`
    reopened for the store setters. -> Document the sentinel on the function.
  - [P3] Design asymmetry between the two writer tasks: Codex expresses the
    wrapper intent as a separate function (`WriteCodexWrapperConfig`), so the
    call site names its intent; Claude overloads one function on whether a
    string happens to be empty. Verified that no live caller can currently
    reach the new semantics by accident — `service.go:663` passes
    `selectedCredential.Endpoint`, which went through
    `NormalizeCredentialEndpoint` (rejects `""`), and the decrypted credential
    cannot be empty because `service.go:234` rejects an empty credential at
    creation — so this is latent, not a live defect. It becomes reachable in
    `route-composition`, where a lookup that returns `""` would silently
    downgrade "write the token" to "delete the token". -> Not a blocker for
    this task; `route-composition` should decide the intent explicitly rather
    than letting a zero value decide it, and should carry a test for that.
  - [P3] Pre-existing but newly consequential: a non-object `env` is
    destroyed. `{"keep":true,"env":"user-string-value"}` + the neither-field
    intent → `{"env":{},"keep":true}`. The line is unchanged by this task, but
    this task's stated acceptance is "the task that protects against a
    whole-`env` rewrite", and the new intent makes the loss total — an unowned
    value is discarded while zero owned fields are written in exchange. ->
    Either leave a non-`map` `env` untouched, or record the decision in the
    task notes so it is a choice rather than an oversight.
  - [nit] `TestWriteClaudeConfigNeitherFieldKeepsEnvObjectWhenLastOwnedKeyGoesAndEnvWasEmpty`
    is misnamed: the fixture's `env` holds two owned keys and no unowned key,
    so `env` was not empty — it *becomes* empty. -> Rename to
    `...WhenEnvHeldOnlyOwnedKeys`.
- Not defects (checked and dismissed):
  - `json.MarshalIndent` re-serializes the whole document and sorts keys —
    pre-existing for every Claude write, not introduced here.
  - `delete` on an absent key is a no-op, satisfying the spec's "removing a
    field already absent is a successful no-op" for both owned keys.
  - The pre-existing `TestWriteClaudeConfigPreservesUnmanagedFields` still
    covers the unchanged endpoint+credential intent, so the acceptance's
    "across every intent" has all three intents covered for unowned-key
    survival.
  - Trailing-slash trimming still applies in the new endpoint-only intent
    (`https://wrapper.example/` → `https://wrapper.example`), asserted.
- Evidence (run by this review against the reviewed state):
  - `go test -mod=vendor ./internal/provider/ -run Claude -v` → all four
    `WriteClaudeConfig` tests pass.
  - `go test -mod=vendor ./internal/provider/` → `ok`.
  - `go vet -mod=vendor ./...` → clean; `gofmt -l` clean on both changed
    files; `git diff --check` clean.
  - Two throwaway probe tests (added, run, removed; working tree confirmed
    unchanged afterwards) produced the two concrete before/after strings
    quoted in the P2 and P3 findings above.
- Verdict: REOPEN — no correctness or security defect on any currently
  reachable path, and the named acceptance is met. Returned to `Dev` for the
  two P2 items (gratuitous `env` creation contradicts the spec's owned-field
  rule and is untested; the sentinel semantics are undocumented). The two P3s
  and the nit may be closed here or explicitly deferred with a recorded
  decision. Next pass is Round 2 in this file.

## Round 2 — 2026-07-27

- Reviewed state: base `ea1df47`, uncommitted working tree; SHA-256 of
  `git diff -- internal/provider/config.go internal/provider/config_test.go` =
  `3360722430769fe2…` (Round 1 reviewed `290425d700a1ab41…`). The same file
  pair still carries `codex-writer-routes`, which is separately REOPEN and out
  of scope here.
- Reviewer: Claude Opus 5 (re-review pass; each Round 1 finding independently
  re-verified by probe, plus a regression sweep over every `env` shape)
- Round 1 finding status:
  - [P2] gratuitous `env: {}` creation — **fixed**. `config.go:335-337` now
    guards the whole write on `isMap || writesOwnedKey`, so the neither-field
    intent on a source with no `env` key takes the block not at all. Probed:
    `{"keep":true,"model":"opus"}` + `ClientConfig{}` →
    `{"keep":true,"model":"opus"}`, no `env` key.
    `TestWriteClaudeConfigNeitherFieldWithoutExistingEnvLeavesDocumentUnchanged`
    asserts exactly that and would fail on a revert.
  - [P2] undocumented empty-string sentinel — **fixed**. `config.go:314-325`
    now names all three intents, states that an empty field removes the key,
    and documents both the env-creation guard and the one remaining
    overwrite case.
  - [P3] non-object `env` destroyed — **closed**, and slightly better than the
    finding asked for. Probed: a string `env`, a `null` `env`, and an array
    `env` all survive the neither-field intent untouched; only a write that
    must set an owned key still replaces a non-map `env`, which the doc
    comment now calls out explicitly.
  - [P3] Codex/Claude design asymmetry — **deferred as agreed**, and properly
    landed: `route-composition` (plan `:294-299`) now carries the obligation
    that every `WriteClaudeConfig` call decide its intent explicitly rather
    than passing through a lookup that can return `""` for another reason,
    with a test required. This is the right owner; nothing further is due
    here.
  - [nit] misnamed test — **fixed**; renamed to
    `…KeepsEnvObjectWhenEnvHeldOnlyOwnedKeys`, and no stale reference to the
    old name survives in code.
- Regression sweep (probe over all nine `env` shapes × intents, run against
  the re-reviewed state):
  - `env` map + neither intent → both owned keys removed, `OTHER` survives.
  - `env` map holding only owned keys + neither intent → survives as `{}`,
    i.e. the original design requirement Round 1 confirmed is still met.
  - `env` absent + writes a key → `env` created, as it must be.
  - both fields written → both owned keys set, unowned `OTHER` survives,
    trailing slash still trimmed.
  - No behavior reachable today changed except the two the fix targeted.
- New findings (all nits, none blocking):
  - [nit] The one documented destructive case — a non-map `env` replaced when
    an owned key must be written — has no test, while every neighbouring case
    now does. It is documented and was confirmed pre-existing, so this is
    coverage symmetry, not a defect. -> Optional one-liner.
  - [nit] `config.go:321-322` reads "is left byte-for-byte unowned apart from
    re-serialization"; "unowned" appears to be a slip for "untouched". ->
    Wording only.
  - [nit] The task's original `Verification` paragraph (plan `:225`) still
    names the pre-rename test, so that line now points at a test that does not
    exist. It is a historical record of a past run, so this is a judgment
    call, but a reader checking the evidence will not find it. -> Either
    update the name or mark the paragraph as superseded by the fix round's
    own verification block.
- Evidence (run by this round against the re-reviewed state):
  - `go test -mod=vendor ./internal/provider/ -run Claude -v` → all seven
    `WriteClaudeConfig` tests pass, including both new ones.
  - `go test -mod=vendor ./...` → every package `ok`.
  - `go vet -mod=vendor ./...` clean; `gofmt -l` clean on both changed files;
    `git diff --check` clean.
  - One throwaway probe test (added, run, removed; working tree confirmed
    unchanged afterwards) produced the nine before/after pairs summarized
    above.
- Verdict: PASS — both P2 findings are closed with tests that fail on
  regression, the closed P3 goes slightly beyond what was asked, the deferred
  P3 has a real owner and a required test in `route-composition`, and the
  nit is fixed. The three new nits are wording and coverage symmetry, not
  reasons to hold this task.
