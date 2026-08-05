---
status: historical
plan: runtime-provider-attribution
task: v0-3-0-contract
retired: 2026-08-04
---

# Review log — runtime-provider-attribution / v0-3-0-contract

## Round 1 — 2026-08-04

- Reviewed state: worktree on `4698775`; `docs/specs/cli-design.md` blob
  `2bce453f1150cf38132b473f90b35cdb99aff32b`, `docs/specs/cli-manual.md` blob
  `03fe8401a09b067ff9484efa67a1cdc955995cda`, `docs/README.md` blob
  `c854638ca2efc20ca2e9adc7066979b877662f65`, plus the staged archive moves.
  All uncommitted.
- Reviewer: Claude Opus 5 (1M context)
- Scope: the task's ten acceptance criteria against the contract edits in
  `docs/specs/`, the documentation-index and retirement changes in `docs/`, and
  the shipped behavior those documents claim to describe.
- **Independence limitation, stated up front:** this reviewer performed the
  retirement and index half of the development pass. The contract edits in
  `docs/specs/` were already in the worktree and are genuinely independent
  input; the P1 below is in that pre-existing half. The development pass also
  signed off two acceptance criteria that this round finds unmet — that
  correction is recorded below rather than quietly reversed. A colder reviewer
  should still confirm the retirement half.

### Findings

**[P1] The specification version was raised without writing the contract the
changelog announces.** `docs/specs/cli-design.md` gained exactly two things: the
version-23 changelog row and a credential-key-compatibility paragraph. The
release's headline change — Hook-owned runtime attribution — has no normative
text anywhere in the document. `grep -i hook docs/specs/cli-design.md` returns
one line, the changelog row itself. Three concrete consequences:

- **The body still asserts what the changelog says was superseded.** `:1095`
  reads "Resuming a logical session through `agentdeck run` starts a new exact
  run with the provider active at resume time", with no mention of Hook
  boundaries — precisely the statement the changelog row supersedes with "Hook
  boundaries supersede client-wide `run` ownership for resumed sessions".
  `:1088` likewise still defines `exact` as "`agentdeck run` owns an
  unambiguous client process lifetime".
- **The table this release adds is undocumented.** `usage_session_routes`
  (`internal/store/migrations.go:107`) appears nowhere in the specification,
  although the document does name `usage_events`, `usage_runs`, and
  `provider_credentials` wherever behavior depends on them.
- **The comparable precedent was handled the opposite way.** When version 21
  added the `shell` group, the body gained roughly 29 lines of contract: the
  managed block, `shell env` as the supported resolver, `shell-init` as a
  hidden byte-equivalent alias that is "not a second contract", and the
  setup/remove semantics. The `usage hook` group received none of that.

This defeats three acceptance criteria simultaneously: "no specification
statement contradicts shipped behavior" (`:1095` does), "no living document
calls `agentdeck run` the primary resume path" (`:1095` effectively does, since
the document offers no other resume mechanism), and "the `usage hook` lifecycle
is described in the same terms as the `shell` lifecycle the specification
already defines".

*Correction to this task's own development pass:* those last two criteria were
marked satisfied on the strength of a string search for "primary resume" and
"resume path", which returned nothing. That was the wrong instrument — the
contradiction is in what `:1095` says, not in whether it uses that phrase.

**[P2] `cli-manual.md` is ahead of `cli-design.md`, inverting the two
documents' roles.** The manual correctly records that `run` is a compatible
low-level launcher and "不再是 resume 的主要归属机制", and carries a full
"Usage Hook 生命周期" section that explicitly compares itself to the `shell`
lifecycle and names the divergence (JSON merge versus text block, no shared
implementation). The contract the manual is supposed to implement says the
opposite. When P1 is fixed, `:1088` and `:1095` should be brought into
agreement with the manual, not the reverse — the manual's account matches
shipped behavior.

### What passed

- **Credential key compatibility paragraph (`:2039-2047`) — verified clause by
  clause** against `internal/credentialvault/vault.go:199-224`. The HKDF stream
  under info `agentdeck/credential-key/v1` yields bytes 0..32 as the AES key and
  32..48 as the version-2 ID, exactly as written; the legacy ID is
  `hex(sha256(key)[:16])`; `KeyFileVersion = 1`; `IsSupportedKeyVersion` admits
  only 1 and 2; `seal` always stamps `KeyVersion` (2) so new seals and rewrites
  use version 2; `Open` accepts both versions and fails closed with
  `ErrKeyVersionUnsupported = "credential_key_version_unsupported"`; nothing
  rewrites version-1 rows automatically. Every clause holds, including the
  claim that the AES key bytes are unchanged.
- **Cache-creation TTL paragraph (`:1048-1053`) — verified** against
  `internal/usage/usage.go:480-497`. The condition
  (`cacheCreation > 0 && cacheWrite5m == 0 && cacheWrite1h == 0`), the
  five-minute default, the exact marker string `defaulted 5m cache creation
  TTL` (`:487`), the partial-breakdown remainder landing in
  `missing_components` (`:494-496`), and "never redistributes that remainder"
  all match. Stored token fields are untouched, as this is read-time pricing.
- **Manual command surface** — `usage hook setup|status|remove` with
  `--client codex|claude|all` matches `cmd/agentdeck/main.go:601-603` and the
  `hook` command at `:2555`; the four reported states match
  `internal/usagehook/config.go:36-39` exactly; the hidden
  `usage hook event` is described only as the installed handler's wire
  mechanism and stays out of the command table, consistent with how the manual
  treats the hidden `shell-init`.
- **Changelog row 23 content** — names the hook lifecycle, the run/boundary
  relationship, sealed key version 2, the cache-creation default, and the
  unresolved `codex-auto-review` classification. Correctly omits the durable
  key-file directory entry (shipped in `v0.2.2`) and the unchanged
  project-attribution shell-function scope (not a contract change).
- **Retirement mechanics** — all three plans and six review records carry
  `status: historical` plus `retired: 2026-08-04`; the archive index holds one
  retirement entry naming what each plan delivered; `docs/README.md` no longer
  re-lists archived files and its roadmap table was removed in favour of the
  prose form that commit `0894830` established for the `v0.2.2` batch; two
  pre-existing broken links exposed by the move
  (`docs/archive/plans/credential-and-pricing-hardening.md:26`,
  `docs/archive/plans/shell-integration.md:1214`) were repaired.
- **Status agreement** — the three archived plans' matrices, the review
  records, and the documentation index agree, with task 4's `Review` correctly
  still unticked.

### Deferred, not a finding

The acceptance item "the release notes state both downgrade consequences"
cannot be met in this task: the repository has no CHANGELOG or release-notes
file, and release notes are produced when the release is authorized. The
requirement was carried into `docs/README.md`'s `v0.3.0` section so it survives
the plan's retirement, and it lands for real at tag time.

### Evidence

- `grep -i hook docs/specs/cli-design.md` -> one match, the changelog row.
- `grep -n "session_route\|usage_session_routes" docs/specs/cli-design.md` -> no
  match; `internal/store/migrations.go:107` creates the table.
- Read of `docs/specs/cli-design.md:1086-1098` for the verbatim `exact` and
  resume statements.
- `internal/credentialvault/vault.go:23-35, 78-120, 199-224` for the key
  version, derivation, seal, and open behavior.
- `internal/usage/usage.go:480-497` for the cache-creation pricing branch.
- `cmd/agentdeck/main.go:601-603, 2555`; `internal/usagehook/config.go:36-39`.
- `git diff --check` (staged and unstaged) -> PASS.
- Full relative-link sweep across `docs/` -> 0 broken.
- `find docs/archive -name '*.md' ! -name README.md -exec sed -n 2p` ->
  0 `status: active`, 104 `status: historical`.

### Verdict

**REOPEN.** The credential and pricing halves of the contract are precise and
verifiable — better than the bar, since every clause survives line-by-line
comparison with the implementation. The retirement half is complete. But the
task exists to make the specification tell the truth about the release, and on
the release's largest change the specification is silent while still asserting
the superseded model. A changelog row is a record of a contract change, not the
change itself. `Review` stays unticked.

## Round 2 — 2026-08-04

- Reviewed state: worktree on `4698775`; `docs/specs/cli-design.md` blob
  `b9983904d8cf0a4efe1d5337a74d45409d33cf27`,
  `docs/archive/plans/runtime-provider-attribution.md` blob
  `ad1c6e828469e7403f6ea3ef7a8646e5a1ffcea6`. Uncommitted.
- Reviewer: Claude Opus 5 (1M context)
- Scope: closure of Round 1's P1 and P2, and independent verification of every
  statement the fix added, read back from disk and checked against the
  implementation rather than against the fix author's notes.
- **Independence limitation:** this reviewer wrote the Round 1 fix. The new
  finding below is in that fix, which is the exact failure mode flagged when the
  fix was handed off. It was found by re-deriving each claim from the code; a
  cold reviewer would still add value.

### Round 1 findings

**[P1] Closed.** `docs/specs/cli-design.md` now carries the Hook attribution
contract in the body of `## Usage Collection and Attribution`, matching that
section's existing flat-prose shape rather than introducing a heading level it
does not use. Verified present and correct against the implementation:

- the fixed resolution order at `:1096-1105` matches
  `internal/usage/usage.go:2599-2630` — exact run binding, then the most recent
  boundary at or before the event, then the session-start fallback;
- the non-blocking overlap rule at `:1108-1111` matches `:1877-1886`, where a
  second run is created with `exact=0` and existing active exact runs are
  updated to `exact=0, ambiguity_reason='managed_client_overlap'`; nothing
  refuses the launch;
- the lifecycle, owned-entry form, registered events, and the four states at
  `:1119-1136` match `internal/usagehook/config.go:803-853`;
- the fail-open handler at `:1142-1150` matches
  `cmd/agentdeck/main.go:2689-2721`, where every failure path returns nil;
- duplicate suppression at `:1156` matches the `WHERE NOT EXISTS` guard in
  `internal/usage/routes.go:63-78`;
- `Schema v17 creates usage_session_routes … and drops the single-active-run
  index` matches `internal/store/migrations.go:105-110`.

Both contradicting statements were rewritten rather than supplemented: the
resume paragraph no longer presents `agentdeck run` as the way to re-attribute a
resumed session, and the runtime provider dimension at `:1259-1264` no longer
claims an estimated event unconditionally uses the session-start snapshot. The
version-23 changelog row was also tightened to state the resolution order
instead of the ambiguous "supersede client-wide `run` ownership", which could
have been read as overriding exact runs.

**[P2] Closed.** `cli-design.md` was brought up to `cli-manual.md` and the
implementation, not the reverse. The manual is unchanged.

### New finding

**[P2] The fix's own boundary paragraph states the wrong rule and contradicts
itself two sentences later.** `:1157` says "Only `SessionStart` establishes a
boundary". That is false: Claude `ConfigChange` also establishes one.
`RecordClaudeConfigChange` (`internal/usage/routes.go:45-53`) calls
`recordSessionRoute` directly with `HookEvent: "ConfigChange"`, bypassing the
`route.HookEvent != "SessionStart"` filter that guards `RecordSessionRoute`
(`:29`). The same paragraph then says `ConfigChange` "is honored", so the
document asserts both that only `SessionStart` creates boundaries and that
`ConfigChange` is acted on.

The precondition attached to that sentence is also mis-scoped. "only when its
transcript validates and a completed provider selection exists" holds for
`SessionStart` — `validHookTranscript` is gated on `event.Name == "SessionStart"`
(`cmd/agentdeck/main.go:2716`) and `RecordSessionRoute` returns early on
`sql.ErrNoRows` (`internal/usage/routes.go:33-35`). Neither applies to
`ConfigChange`: `reconcileClaudeConfigChange` (`cmd/agentdeck/main.go:2727-2757`)
does not return on `sql.ErrNoRows`, and after its attempts are exhausted it
records a boundary with provider `unknown` and multiplier `1`. A `ConfigChange`
boundary can therefore exist where no completed selection exists at all.

This matters beyond wording. A reader implementing against this contract would
conclude that a Claude settings change never re-routes a session — the exact
opposite of what the delivered `claude-reload-boundary` task provides, and the
reason that task exists. The rest of the paragraph is accurate; only the
"Only `SessionStart`" sentence and its trailing precondition need rewriting so
that `SessionStart` and Claude `ConfigChange` are both named as boundary
sources, with `compact` and `SessionEnd` named as recording none, and the
transcript/selection preconditions scoped to `SessionStart`.

### Evidence

- Read of `docs/specs/cli-design.md:1086-1164` and `:2121` from disk.
- `internal/usage/routes.go:22-40, 45-53, 56-80`;
  `internal/usage/usage.go:1847-1886, 2599-2630`;
  `internal/usagehook/config.go:803-853`;
  `cmd/agentdeck/main.go:2689-2758`;
  `internal/store/migrations.go:102-110`.
- `git diff --check` (staged and unstaged) -> PASS.
- Full relative-link sweep across `docs/` -> 0 broken.

### Verdict

**REOPEN.** Round 1's P1 and P2 are genuinely closed: the contract exists, it is
in the right place, and every load-bearing statement in it survives comparison
with the implementation. The task is held open for one wrong sentence in that
new contract — but a wrong sentence about which client events change
attribution is not cosmetic in the document whose job is to state exactly that.
The fix is confined to `:1157-1164`. `Review` stays unticked; `Dev` reverts.

## Round 3 — 2026-08-04

- Reviewed state: worktree on `4698775`; `docs/specs/cli-design.md` blob
  `1b7aeb8cdf29b769b79ffa678416a3bbc18bae89`,
  `docs/specs/cli-manual.md` blob
  `03fe8401a09b067ff9484efa67a1cdc955995cda`, and
  `docs/archive/plans/runtime-provider-attribution.md` pre-review-artifact blob
  `4f6d16878a6ee491f23c62141d39468881a36b26`. Product code is unchanged from
  the Round 2 reviewed state. All changes remain uncommitted.
- Reviewer: Codex (GPT-5)
- Scope: independent closure of Round 2's boundary-source finding, plus a
  contradiction check against the adjacent contract and manual text. Review
  artifact and status updates below are not substantive review input.

### Round 2 finding

**[P2] Closed.** The corrected contract now names both boundary-producing
events: `SessionStart` for Codex and Claude, and `ConfigChange` for Claude.
It scopes transcript validation and the completed-provider-selection
precondition to `SessionStart`; it separately states that a user-settings
`ConfigChange` can record `unknown` even when `SessionStart` would write
nothing. It also retains the boundary-free rules for `compact` and
`SessionEnd`.

This matches the current implementation:

- `usage.Service.RecordSessionRoute` accepts only non-`compact`
  `SessionStart`, requires `CurrentProviderSnapshot`, and returns without a
  boundary on `sql.ErrNoRows`;
- `reconcileClaudeConfigChange` gates on the managed user settings file,
  records the matched snapshot when available, and otherwise calls
  `RecordClaudeConfigChange` with `matched=false`;
- `RecordClaudeConfigChange` writes an estimated `ConfigChange` boundary with
  provider `unknown` and multiplier `1` for an observed mismatch or absent
  selection;
- the adjacent lifecycle, duplicate-suppression, fallback, and schema-v17
  statements remain consistent with the Round 2 verified implementation.

### Findings

No new blocking, high-, or medium-priority findings.

### Evidence

- CodeGraph call-path and verbatim-source inspection of
  `RecordSessionRoute`, `RecordClaudeConfigChange`,
  `reconcileClaudeConfigChange`, and the hook event gating.
- Focused read of `docs/specs/cli-design.md:1086-1172` and the adjacent
  `docs/specs/cli-manual.md` Usage Hook lifecycle section.
- Current content identity differs from Round 2 only in the reviewed contract
  paragraph and its task-local evidence/status artifacts; relevant product
  code is unchanged in `git status`.
- `git diff --check` after the review-artifact and status updates -> PASS.
- Full relative-link sweep across `docs/`, excluding fenced code blocks ->
  0 broken.

### Verdict

**PASS.** Round 2's sole finding is closed, no new regression was found, and
the task's `Review` gate is checked. The `v0.3.0` batch is now complete; the
next workflow step is the separately authorized release-candidate preparation
described in `docs/README.md`.
