---
status: active
created: 2026-08-02
---

# Credential Key Derivation and Cache Pricing

Target release: `v0.3.0`.

Two tasks split out of
[the credential and pricing hardening plan](../archive/plans/credential-and-pricing-hardening.md)
on 2026-08-02, when
[the release versioning contract](../archive/plans/release-versioning-contract.md) classified
both as MINOR. They were designed on 2026-07-29 and their evidence is carried
over unchanged; only the release target and the contract ownership moved.

Why each is MINOR rather than PATCH:

- `key-id-derivation` writes newly sealed rows at key version 2. Per the
  evidence below, an earlier release compares `sealed.KeyVersion` against its
  own single supported constant, so `v0.2.1` would report those rows as
  `credential_key_version_unsupported`. The tree is therefore not safe to
  downgrade from, which is MINOR trigger 6.
- `cache-creation-ttl-default` changes reported cost and coverage numbers for
  unchanged input, which is MINOR trigger 4, and rewrites a promised rule in
  `docs/specs/cli-design.md`, which is trigger 7.

Contract text for both tasks is **not** owned here. It is recorded by the single
`v0.3.0` contract task in
[the runtime provider attribution plan](runtime-provider-attribution.md), so the
release increments the specification version exactly once.

## Goal

- Stop publishing a hash of the live AES key as the persisted key ID, without
  breaking any credential already sealed under key version 1.
- Decide what a Claude cache-creation total means when the provider supplies no
  TTL breakdown, and price it accordingly.

## Non-Goals

- No credential re-encryption sweep, no forced key rotation, and no automatic
  migration of existing ciphertext.
- No change to the credential key file format, its seed size, its HKDF salt or
  info string, or the derived AES key bytes.
- No change to the price catalog, its generator, or its pinned LiteLLM commit.
- No model-name special-casing anywhere in usage parsing or pricing.
- No new `doctor` check, recovery command, or `--fix` behavior.

## Evidence Baseline

Gathered on 2026-07-29 at `2db056b`, before any task started.

**Credential key.** `KeyVersion = 1` at `internal/credentialvault/vault.go:25`
is used for two different things: the version byte inside the key file
(line 235, checked at line 208) and the `key_version` recorded on every sealed
row (line 104, checked at line 112). `deriveKey` returns
`hex(sha256(key)[:16])` as the key ID (lines 192-193).

Consumers of that key ID are wider than the vault:

| Site | Current logic | Why it matters here |
| --- | --- | --- |
| `internal/doctor/doctor.go:281` | `sealed.KeyVersion != credentialvault.KeyVersion` reports `credential_key_version_unsupported` | A second supported version must not be reported as unsupported |
| `internal/doctor/doctor.go:320` | `sealed.KeyID != keyID` reports `credential_key_machine_mismatch` | Comparing a version-1 row against a version-2 key ID would be a false machine mismatch |
| `internal/doctor/doctor.go:289-305` | `rotationReady` requires exactly one distinct stored key ID equal to the live one | A mixed-version store has two distinct IDs, which would silently drop the `--rotate` recovery hint |
| `internal/provider/service.go:1116-1127` | same single-key-ID comparison | same |

`doctor.go:281` is also the reason this task is not downgrade-safe: that exact
comparison exists in `v0.2.1` with `KeyVersion = 1`.

**Claude cache creation.** `usage.go:1224-1232` copies
`usage.cache_creation_input_tokens` into `cache_creation_tokens` and, when the
`cache_creation` object is present, copies its two ephemeral fields into
`cache_write_5m_tokens` and `cache_write_1h_tokens`. `usage.go:475-477` then
marks `cache_creation_tokens` unpriced whenever the total is positive and both
TTL buckets are zero, which suppresses `CatalogBaseCost`/`ProviderCost` for the
whole event (lines 487-490). `docs/specs/cli-design.md:1048-1049` states this as
an intentional rule.

An aggregate-only probe of the real local Claude logs (counts only; no session
text, paths, or arguments were emitted) shows what the affected events actually
look like:

| Model | `creation>0` events | object absent | object present, both TTLs zero | object with a non-zero TTL | Σ creation | Σ 5m | Σ 1h |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `claude-opus-4-8` | 9115 | 0 | 0 | 9115 | 75,620,547 | 48,372,753 | 27,247,794 |
| `claude-sonnet-5` | 7566 | 0 | 0 | 7566 | 36,018,777 | 11,852,178 | 24,166,969 |
| `claude-haiku-4.5` | 2664 | 0 | 2664 | 0 | 7,716,156 | 0 | 0 |
| `claude-opus-5` | 2538 | 0 | 0 | 2538 | 16,121,970 | 0 | 16,121,970 |
| `claude-fable-5` | 1151 | 0 | 0 | 1151 | 5,343,867 | 5,343,867 | 0 |
| `claude-haiku-4-5-20251001` | 250 | 0 | 0 | 250 | 832,469 | 241,789 | 590,680 |
| `claude-sonnet-4-6` | 22 | 0 | 0 | 22 | 434,912 | 434,912 | 0 |
| `claude-opus-4.8` | 1 | 0 | 1 | 0 | 59,152 | 0 | 0 |

Three conclusions follow, and they correct the original Backlog entry's framing:

1. The `cache_creation` object is never absent in observed data. The affected
   events carry the object with both ephemeral fields at zero.
2. Dotted spelling is a coincidence, not the cause: dotted
   `claude-haiku-4-5-20251001` reports a normal breakdown, and hyphenated
   `claude-opus-4-8` does too. Any implementation that branches on a model name
   is wrong.
3. Wherever a breakdown exists, `5m + 1h` equals the reported total exactly.
   The two `claude-sonnet-5` events with a zero total and a non-zero breakdown
   (370 tokens) are the only observed inconsistency in the other direction.

**Cross-plan ordering, added 2026-08-02.** `v0.2.2` delivers
[the usage pricing read scalability plan](../archive/plans/usage-pricing-read-scalability.md),
whose `shared-read-resolver` task unifies the pricing read path used by stats,
deep diagnostics, and session summaries. Task 2 below must be implemented on top
of that resolver, not before it: changing the cache-creation rule first would
mean changing it once in the legacy per-event path and again when that path is
replaced, and recomputing the coverage assertions twice.

## Tasks

### 1. `key-id-derivation`

Derive the persisted key ID alongside the AES key instead of hashing it, and
add key version 2 without invalidating version 1.

Design constraints, in order of importance:

- **The AES key bytes must not change.** Keep the HKDF salt (machine identity)
  and info string (`agentdeck/credential-key/v1`) exactly as they are, and read
  48 bytes from the same reader: bytes 0..32 stay the key, bytes 32..48 become
  the version-2 key ID. Because HKDF-Expand is a stream, the first 32 bytes are
  bit-identical to today's key, so every existing ciphertext still decrypts.
- **The key file format must not change.** Split the overloaded `KeyVersion`
  constant into a key-file format version (stays 1, still the byte written and
  checked in the file) and a sealed key version, whose current value becomes 2
  and whose supported set is {1, 2}. An existing key file must keep loading
  untouched.
- **Version-aware key IDs.** `Open` accepts sealed rows at version 1 or 2 and
  compares against the key ID for *that* row's version; version 1 keeps
  `hex(sha256(key)[:16])`. `Seal` and `SealExisting` always write version 2.
- **No migration sweep.** Existing rows stay at version 1 until they are
  rewritten by a normal credential write or an explicit
  `credential update --rotate`. No command silently re-encrypts.

Consumers to update, all four sites listed in the Evidence Baseline:

- `doctor.go:281` must accept any supported version rather than only the
  current one.
- `doctor.go:320` must compare each row against the expected key ID for its own
  version.
- `rotationReady` in both `doctor.go` and `provider/service.go` must mean "every
  stored row matches this machine's expected key ID for its own version",
  because a mixed-version store legitimately holds two distinct IDs. Whatever
  shape that takes, `credential update --rotate` must still be offered as
  recovery in exactly the situations it is offered today.

Acceptance:

- A vault whose ciphertext was sealed before this change still opens, and
  `doctor` reports it healthy with no version or mismatch problem.
- A store holding both a version-1 and a version-2 row reports no problem, and
  the `--rotate` recovery hint behaves as it does for a single-version store.
- A version-2 key ID is not derivable from the AES key by SHA-256, and the AES
  key derived for a fixed seed and machine identity is unchanged from version 1.
- An unsupported version (0, 3, or absent) still fails closed with
  `credential_key_version_unsupported` and never overwrites ciphertext.
- The downgrade consequence is stated in the release notes: once a credential is
  written by this build, `v0.2.1` and earlier report it as an unsupported key
  version until it is rewritten.

Verification: L3. Targeted `internal/credentialvault`, `internal/doctor`, and
`internal/provider` tests, then the full vendor suite, `-race`, and `go vet`.

Prerequisite: `key-file-durability` in the `v0.2.2` hardening plan, which edits
the same file. **Satisfied on 2026-08-03** - and it is what moved the `vault.go`
line numbers cited above, so re-verify them before relying on any of them.

#### Development Evidence (2026-08-04)

- New seals write sealed key version 2 and the next 16 bytes of the unchanged HKDF stream as their key ID; the first 32 AES-key bytes, key-file format version, salt, and info string remain unchanged.
- Version-1 sealed rows validate with the legacy SHA-256 key ID; `doctor` and credential writes accept authenticated version-1/version-2 mixtures, while unknown versions remain fail-closed without a misleading machine-mismatch diagnostic.
- Regression tests cover the fixed HKDF boundary, v2 ID derivation, version-1 decryption, versions 0 and 3 rejection, normal credential rewrite from legacy to current, and full `doctor` mixed-version diagnostics.
- RED: `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/credentialvault -run '^TestVaultSealsWithDerivedVersionTwoKeyID$'` failed before implementation with `sealed key version = 1, want 2`.
- GREEN: targeted `internal/credentialvault`, `internal/doctor`, and `internal/provider` tests; `go test -mod=vendor ./...`; `go test -mod=vendor -race ./...`; and `go vet -mod=vendor ./...` all passed with `GOCACHE=/private/tmp/agent-deck-go-build`.
- Review Round 1 repair preserves `(key_version, key_id)` pairs in the store preflight. Cross-version IDs now reject credential writes without changing ciphertext and suppress `--rotate` recovery, while unrelated unsupported versions retain the prior recovery behavior for valid damaged rows. Targeted store/provider/doctor tests and the L3 command set passed again after the repair.
- Review Round 2 repair classifies stored key versions 0 and 3 as `credential_key_version_unsupported` before ID comparison. Regression coverage confirms both block normal credential writes without changing the stored secret; targeted store/provider/doctor tests and the L3 command set passed again.

### 2. `cache-creation-ttl-default`

Price a Claude cache-creation total that arrives without a TTL breakdown.

`docs/specs/cli-design.md:1048-1049` currently says such totals stay unpriced
and that the scanner never guesses. The replacement rule must be a disclosed
default, not a silent guess:

- **Storage stays faithful.** The parser keeps writing what the source said:
  `cache_creation_tokens` as reported, and the two TTL buckets exactly as the
  `cache_creation` object gave them, including zero. No rebuild is required for
  users to benefit, and no historical event is reinterpreted on disk.
- **Pricing applies the default TTL.** When `cache_creation_tokens > 0` and both
  TTL buckets are zero, price the total at the five-minute cache-write rate,
  which is the rate the importer already maps
  `cache_creation_input_token_cost` to (`cli-design.md:1327`) and the default
  TTL Anthropic applies when no longer TTL is requested.
- **The estimate is disclosed.** A cost that rests on this default must be
  distinguishable from one derived from a reported breakdown. Follow the
  existing disclosed-estimate precedent for prices rather than inventing a new
  vocabulary, and keep the disclosure out of any grouping key.
- **The branch is data-driven.** The condition is "positive total, both buckets
  zero". It must never test a model name, a client version, or a spelling.
- **Partial and contradictory shapes stay conservative.** If a breakdown is
  present and non-zero but does not sum to the total, price the reported
  buckets and leave the remainder unpriced as today; do not redistribute. The
  observed zero-total-with-breakdown case keeps its current behavior.

Consequences to carry through in the same task:

- `missing_components` no longer reports `cache_creation_tokens` for the
  defaulted shape, and affected events gain non-null cost fields.
- Cold-start coverage assertions move: the bundled-coverage expectation of
  95.1% fully priced must be recomputed, not adjusted by hand.
- `usage stats` coverage, cost, and `unpriced_models` output changes for
  affected data. The release notes must state that costs previously reported as
  incomplete will now be priced.

Acceptance:

- An event with a positive total and a zero breakdown is priced at the
  five-minute rate, carries no `cache_creation_tokens` in
  `missing_components`, and is marked as resting on the default TTL.
- An event with a reported breakdown is priced exactly as it is today, with no
  disclosure marker.
- An event whose breakdown is non-zero but short of the total keeps the
  remainder unpriced.
- Two models that differ only in spelling receive identical treatment for
  identical token shapes.
- Recomputed coverage figures are recorded in this plan's evidence when the task
  completes.

Verification: L2. Targeted `internal/usage` and `cmd/agentdeck` tests, then the
full vendor suite.

Prerequisite: `shared-read-resolver` in the `v0.2.2` scalability plan, for the
reason recorded in the Evidence Baseline. **Satisfied on 2026-08-03.**

#### Development Evidence (2026-08-04)

- Claude events with a positive cache-creation total and two zero TTL buckets
  now use the catalog's five-minute cache-write price. They are fully priced
  and disclose `defaulted 5m cache creation TTL` through the existing summary
  warning output, including JSON.
- A non-zero TTL breakdown that does not equal its positive total prices only
  the reported buckets and retains `cache_creation_tokens` as unpriced. A
  zero-total event with a reported breakdown keeps its existing behavior.
- The resolver branch examines only the three token fields; it contains no
  model-name, spelling, or client-version condition.
- RED: `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor
  ./internal/usage -run '^TestCalculateSeparatesCachedAndClaudeTTLComponents$'`
  failed before the change: the defaulted shape had no complete cost and left
  `cache_creation_tokens` unpriced.
- GREEN: targeted `internal/usage` default/contradictory-shape and summary
  disclosure tests; then `GOCACHE=/private/tmp/agent-deck-go-build go test
  -mod=vendor ./internal/usage ./cmd/agentdeck` and `GOCACHE=/private/tmp/
  agent-deck-go-build go test -mod=vendor ./...` passed.
- Review Round 1 repair: `usage stats` now retains the same deduplicated warning
  in its report, text section, and JSON data/envelope. Regression coverage
  exercises that end-to-end route, dotted and hyphenated model spellings, and
  the zero-total-with-reported-breakdown invariant; the bundled cold-start
  fixture now exercises both observed default-TTL model shapes.
- Recomputed on the frozen aggregate-only baseline: the prior fully priced
  numerator `5,000,000,097` gains `7,775,308` defaulted cache-creation tokens
  (`7,716,156` for `claude-haiku-4.5` plus `59,152` for `claude-opus-4.8`),
  yielding `5,007,775,405 / 5,259,503,075 = 95.213851%`, reported as **95.2%**.
- Repair verification: `GOCACHE=/private/tmp/agent-deck-go-build go test
  -mod=vendor ./internal/usage ./cmd/agentdeck` and `GOCACHE=/private/tmp/
  agent-deck-go-build go test -mod=vendor ./...` passed.
- Review Round 2 repair: the exact dotted/hyphenated pair `claude-opus-4.8`
  and `claude-opus-4-8` now receives the same positive creation total and zero
  TTL buckets in both direct and bundled-cold-start regression coverage.
- Review Round 3 repair: the real CLI disclosure test pins the existing UTC
  display clock so its selected local day always contains the synthetic event.
  The documented `UPDATE_AGENTDECK_GOLDEN=1` path regenerated the GUI JSON
  contract, whose only change is `usage.stats.data.warnings: []`; the focused
  CLI and isolated end-to-end tests pass without the update environment.
  Uncached targeted `internal/usage` and `cmd/agentdeck` tests plus the full
  vendored suite also pass.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `key-id-derivation` | [x] | [x] |
| 2. `cache-creation-ttl-default` | [x] | [x] |

The two tasks are independent of each other. Both had a `v0.2.2` prerequisite,
recorded in their own sections; both were satisfied on 2026-08-03, when that
release's plans retired. Contract text for both is owned by the `v0.3.0`
contract task in the runtime provider attribution plan, which cannot start until
both are reviewed.

Commit boundaries follow task boundaries: one commit per task.

## Starting a task

> 进入开发：`credential-key-and-cache-pricing` / `<task-anchor>`

Read, in order: `AGENTS.md`; this plan's Evidence Baseline and the task's own
section; every file the task names; and the verification routing in `AGENTS.md`
for the level the task declares. Confirm the task's `v0.2.2` prerequisite has
landed before starting. Implement only that task's scope. When the task's own
targeted verification passes, tick `Dev`, record the evidence under the task,
and leave the review trail for an independent reviewer, who ticks `Review` only
after a `Verdict: PASS` round in
`docs/reviews/credential-key-and-cache-pricing/<task-anchor>.md`.
