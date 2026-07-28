---
status: active
plan: provider-wrapper-routing
task: cli-route-surface
---

# Review log — provider-wrapper-routing / cli-route-surface

## Round 1 — 2026-07-27 (summary, delivered in session)

- Reviewed state: base `382a358`, uncommitted working tree.
- Verdict: **PASS with improvements**. No blocking finding. All three named
  acceptance criteria hold, verified empirically: `--via` without a configured
  wrapper fails before any client file is touched (custom and `official` both
  checked against a built binary; config byte-identical, exit 2); no existing
  invocation changed behavior; `NormalizeWrapperURL` runs before any store
  write. Three independent revert checks confirmed the new tests were RED
  without their fix (`--via` flag wiring, URL normalization, `--quiet`
  suppression).
- Findings, all improvement-level:
  1. **Coverage.** The new JSON fields were structurally excluded from the
     golden: `provider set-wrapper` ran *after* `provider.list|status|show` in
     the e2e flow, and `provider.status` was only ever called before any
     switch, so `wrapper_url` could never appear and `active[]` was always an
     empty array. No test asserted `wrapper_url` in `list|show|status` JSON, and
     `ActiveSelection`'s route fields had no JSON assertion at all.
  2. **Spec drift.** `cli-design.md`'s `provider list` column enumeration
     omitted the wrapper URL while the same section states a wrapper appears in
     `provider list|show`.
  3. **Silent normalization.** `set-wrapper`'s text output does not echo the
     stored value, so `--url https://x/v1/` is silently rewritten to
     `https://x` with no visible confirmation.
- Recorded P3 (not to fix): the effective-route line reads the route back
  through `Service.Current` instead of having `UseCredential` return it, so a
  concurrent switch of the same client by another process could make the line
  describe that other selection.
- Checked and dismissed: `ftp://` accepted by `--url` (identical to
  `provider add --endpoint`, and the spec requires the same normalization);
  unknown provider → `runtime_error`/exit 1 (same as `provider show`); bad or
  empty `--url` → `invalid_provider`/exit 2; `--quiet` suppressing the stderr
  line does not change the JSON envelope.

## Round 2 — 2026-07-27 (re-review)

- Reviewed state: base `382a358`, uncommitted working tree. SHA-256 of
  `git diff -- cmd/agentdeck internal/provider` = `5029f84e3ef7bfc0…`; SHA-256
  of the untracked `cmd/agentdeck/provider_route_surface_test.go` =
  `1e2695de0e9fb69b…`. `docs/specs/cli-design.md` is newly dirty since Round 1
  (the column enumeration).
- Method: every claim re-derived from the current tree, not from the Fix round
  note. The golden was regenerated from scratch in a second out-of-tree copy
  and compared field-by-field; two of the new tests were reverted to confirm
  they fail. The repository itself was not modified by this pass.

### Finding-by-finding disposition

- **[1] Coverage — FIXED, and the fixture is now machine-derived rather than
  hand-written.** The e2e flow sets the wrapper before the definition reads,
  switches with `provider use phase7 --via`, and re-reads `provider status`
  after both switches. Independently regenerated the golden in a fresh copy
  (only patch: neutering the pre-existing stats assertion that aborts the flow)
  and compared against the repository fixture: `provider.list`, `provider.show`,
  `provider.status`, `provider.update`, `provider.current`, and
  `provider.set-wrapper` all **MATCH**, and the *only* differing entry in the
  whole file is `usage.stats`.
  - `usage.stats` is byte-identical to `HEAD` — confirmed by diffing the
    working fixture against `git show HEAD:...`. The regenerated form (empty
    `clients`, `models`, `providers`) was correctly **not** taken; taking it
    would have baked the pre-existing 0-events failure into the golden and
    silenced it.
  - `provider.status` now carries three element schemas, two of which have a
    populated `active[]` containing `via_wrapper` and `endpoint` — a real
    selection, not an empty array.
  - Two new tests, both confirmed RED against a reverted implementation:
    dropping the official wrapper read from `officialDefinition` fails
    `TestProviderDefinitionJSONCarriesWrapperURLForBothProviderKinds`
    (`"official":interface {}(nil)`), and dropping the route fields from
    `Status`'s `ActiveSelection` fails `TestProviderStatusJSONReportsSelectionRoute`
    (`via_wrapper:false`, no `endpoint`) as well as the text assertion in
    `TestProviderUseViaWrapperWritesWrapperEndpointAndReportsRoute`.
- **[2] Spec drift — FIXED.** `cli-design.md:706-709` now lists the provider's
  optional wrapper URL among `provider list`'s columns, consistent with the same
  section's `provider list|show` statement two sentences later and with the
  implemented `WRAPPER` column. No other column enumeration in either spec
  document is stale: `provider status`'s `CODEX ACTIVE`/`CLAUDE ACTIVE`
  (`cli-design.md:738`) is unchanged by this task, and `provider current`'s
  field list (`cli-design.md:743-746`) already named the route and the written
  endpoint.
- **[3] Silent normalization — CLOSED as a documented trade-off, and the
  reasoning holds.** Verified the premise empirically: `provider add` prints
  `Completed provider.add.` and `provider update` prints
  `Completed provider.update.`, both normalizing their `--endpoint` just as
  silently. Echoing only in `set-wrapper` would indeed have made it the single
  exception among three commands sharing one normalization rule. The manual now
  states the rewrite and points at `provider show` and `--format json`, both of
  which do return the stored value.

### Requested cross-checks

- **e2e switching to `--via` does not disturb any later assertion.** Usage
  attribution resolves a provider through `runtimeProviderAt` →
  `timeline.SnapshotAt(...).Name` (`internal/usage/usage.go:2430-2445`); the
  endpoint snapshot is never an input, and `--via` leaves
  `MultiplierSnapshot` and the credential untouched, so the provider-filtered
  stats assertion keeps exactly the meaning it had. The restore assertions
  check the decrypted credential value, provider names, and event counts —
  none of them read an endpoint or a route. The `provider.use` contract is
  `data: null` on both routes, confirmed unchanged by the regeneration.
- **The pre-existing e2e failure is unchanged in kind.** It still aborts at the
  same stats assertion (`e2e_test.go:148`; it was `:141` before, moved only by
  the added lines), and the package still fails exactly two tests by name.

### Not defects (checked and dismissed)

- The golden's `provider.list` official element carries no `wrapper_url`,
  because `official` has no wrapper in the e2e flow. Official's wrapper
  reporting is covered by `TestProviderDefinitionJSONCarriesWrapperURLForBothProviderKinds`
  instead. Setting an official wrapper in the e2e as well would add golden
  coverage but also permanently route the flow's `official` reads through a
  wrapper; the current split is reasonable.
- The golden records types, not values, so it cannot distinguish a wrapped from
  a direct selection. That distinction is what the two value-asserting tests
  exist for; expecting it from a schema fixture would be a category error.

### Nits (no fix required)

- `docs/specs/cli-manual.md` says the text output is "与 `provider add`/`provider
  update` 一样只说明完成的动作和资源名". The shared property it relies on — no
  stored value echoed — is accurate, but `add`/`update` in fact print no
  resource name at all (`Completed provider.add.`), while `set-wrapper` prints
  `Completed provider.set-wrapper for "p1".`. The sentence is slightly more
  precise than the behavior it compares against.
- `set-wrapper --url` and `set-wrapper --clear` produce byte-identical text
  output, so text-mode users cannot tell which one ran without a follow-up
  `provider show`. This is a direct consequence of the accepted
  documentation-only disposition of finding 3, recorded here so the trade-off
  is visible rather than rediscovered.
- `cli-design.md:706-709` has an awkward line wrap after the edit ("rather than
  a single / endpoint or multiplier. Credential / readiness belongs...").
  Renders identically; only a source-formatting wrinkle.
- `provider status <name>` text prints `credentials: N` twice (once from
  `renderProviderDetail`, once from `renderProviderStatus`). Pre-existing on
  `main`, not introduced here; noted because this task's new `ROUTE`/`ENDPOINT`
  columns sit in the same output.

### Evidence

- Independent golden regeneration in a fresh out-of-tree copy → only
  `usage.stats` differs; the six `provider.*` entries all MATCH.
- `git show HEAD:cmd/agentdeck/testdata/phase7/gui-json-contract.json` vs. the
  working file → `usage.stats` identical; six `provider.*` entries changed.
- Two revert runs (copy only) producing the RED states quoted above.
- `go test -mod=vendor ./cmd/agentdeck/ -run TestProvider` → `ok`.
- Full-suite, `go vet`, `gofmt -l`, and `git diff --check` results from the fix
  round are reused: the tree is byte-identical to the state they were bound to
  (hashes above; `git status` unchanged), and this pass modified nothing.

### Verdict

**PASS.** All three Round 1 findings are closed — two by code and test changes,
one by an explicitly reasoned documentation disposition — and the coverage fix
is stronger than asked: the fixture is now derived from real output rather than
hand-written, and the one entry that could not be regenerated honestly was left
untouched so the pre-existing failure keeps its signal. Four nits, none
requiring action. `Review` ticked for `cli-route-surface`; no fix round follows.
