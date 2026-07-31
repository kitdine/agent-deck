---
status: active
created: 2026-07-29
---

# Credential and Pricing Hardening

Target release: `v0.2.2`.

Five known defects promoted out of the `docs/README.md` Backlog. Each was
recorded there with its own evidence and left for "the next time that file is
opened"; this plan opens those files once, deliberately, instead of letting the
entries age further.

The batch is deliberately narrow: no new command, flag, database table, or
column. Two tasks change promised behavior, so the batch's contract task must
increment `docs/specs/cli-design.md` once from whatever version is current at
delivery; the rest are internal robustness or documentation accuracy.

## Goal

- Close the credential-key durability window that can strand ciphertext with no
  recoverable key.
- Stop publishing a hash of the live AES key as the persisted key ID, without
  breaking any credential already sealed under key version 1.
- Make an oversized 5xx price response retryable, as the contract already
  promises.
- Either stop creating session-index WAL sidecars during read-only diagnostics
  or stop claiming that they are not created.
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

**Credential key.** `internal/credentialvault/vault.go:244` links the temporary
seed file into place with `os.Link` and returns. The file's contents were synced
at line 236, but the parent directory entry never is. `KeyVersion = 1` at line 25
is used for two different things: the version byte inside the key file
(line 223, checked at line 197) and the `key_version` recorded on every sealed
row (line 98, checked at line 106). `deriveKey` returns
`hex(sha256(key)[:16])` as the key ID (lines 181-182).

Consumers of that key ID are wider than the vault:

| Site | Current logic | Why it matters here |
| --- | --- | --- |
| `internal/doctor/doctor.go:246` | `sealed.KeyVersion != credentialvault.KeyVersion` reports `credential_key_version_unsupported` | A second supported version must not be reported as unsupported |
| `internal/doctor/doctor.go:285` | `sealed.KeyID != keyID` reports `credential_key_machine_mismatch` | Comparing a version-1 row against a version-2 key ID would be a false machine mismatch |
| `internal/doctor/doctor.go:266-270` | `rotationReady` requires exactly one distinct stored key ID equal to the live one | A mixed-version store has two distinct IDs, which would silently drop the `--rotate` recovery hint |
| `internal/provider/service.go:1091-1098` | same single-key-ID comparison | same |

**Price retrieval.** `internal/usage/price_update.go:140-153` reads the body
under a `LimitReader`, then checks the byte cap at line 148 and only afterwards
checks `resp.StatusCode` at line 151. An oversized 5xx body therefore returns
`retryable=false` with `response exceeds N bytes`. The contract at
`docs/specs/cli-design.md:1170-1173` already promises up to three attempts for
HTTP 408/429/5xx, so this is a defect against the existing contract, not a
contract change.

**Session diagnostics.** `internal/session/doctor.go:20-21` documents
`CheckHealth` as inspecting the index "without creating, migrating, or changing
it", while opening a WAL-mode database `mode=ro` materializes `-wal` and `-shm`.
`internal/session/doctor_test.go:80-93` already pins the observed sidecar
creation, with a comment stating the pin is test-only. Nothing in
`docs/specs/cli-design.md` promises the absence of sidecars; the design-level
promises for `doctor` (lines 1677-1679) are no network, no credentials, no
session text.

**Claude cache creation.** `usage.go:1224-1232` copies
`usage.cache_creation_input_tokens` into `cache_creation_tokens` and, when the
`cache_creation` object is present, copies its two ephemeral fields into
`cache_write_5m_tokens` and `cache_write_1h_tokens`. `usage.go:475-477` then
marks `cache_creation_tokens` unpriced whenever the total is positive and both
TTL buckets are zero, which suppresses `CatalogBaseCost`/`ProviderCost` for the
whole event (lines 487-490). `docs/specs/cli-design.md:903-904` states this as
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

Three conclusions follow, and they correct the Backlog entry's framing:

1. The `cache_creation` object is never absent in observed data. The affected
   events carry the object with both ephemeral fields at zero.
2. Dotted spelling is a coincidence, not the cause: dotted
   `claude-haiku-4-5-20251001` reports a normal breakdown, and hyphenated
   `claude-opus-4-8` does too. Any implementation that branches on a model name
   is wrong.
3. Wherever a breakdown exists, `5m + 1h` equals the reported total exactly.
   The two `claude-sonnet-5` events with a zero total and a non-zero breakdown
   (370 tokens) are the only observed inconsistency in the other direction.

## Tasks

### 1. `key-file-durability`

Close the durability window at `vault.go:244`.

- After `os.Link` succeeds, open the state root and `Sync()` it before
  returning, so the key file's directory entry is durable before any caller can
  commit ciphertext that depends on it.
- Introduce the smallest testable seam for this. `Vault` currently calls `os`
  directly, so add one replaceable directory-sync function field on `Vault`
  (unexported, defaulting to the real implementation) rather than threading the
  `platform.FileSystem` abstraction into the vault.
- A directory-sync failure must return an error and must **not** remove the
  already-linked key file: the file may already be the only copy of the seed,
  and `createSeed` recovers from `fs.ErrExist` by loading it.
- `InitializeNew` inherits the same guarantee because it calls
  `createSeedExclusive`.

Acceptance:

- The sync happens after the link, exactly once, on the state root.
- A failing directory sync surfaces as an error from `createSeedExclusive`,
  `createSeed`, and `InitializeNew`, and the key file still exists afterwards
  with mode `0600`.
- A subsequent call recovers by loading the existing seed and derives the same
  key ID.

Verification: L3. Targeted `internal/credentialvault` tests, then the full
vendor suite and `go vet`.

### 2. `key-id-derivation`

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

- `doctor.go:246` must accept any supported version rather than only the
  current one.
- `doctor.go:285` must compare each row against the expected key ID for its own
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

Verification: L3. Targeted `internal/credentialvault`, `internal/doctor`, and
`internal/provider` tests, then the full vendor suite, `-race`, and `go vet`.
Contract text is task 6.

### 3. `price-retry-ordering`

Check the HTTP status before the byte cap in `fetchPriceBody`.

- A non-200 response reports `retryablePriceStatus(status)` regardless of body
  size, so an oversized 502 is retried and an oversized 404 is not.
- A 200 response whose body exceeds the cap still fails non-retryably with the
  existing `response exceeds %d bytes` message.
- No change to attempt counts, backoff, the cap value, or the error text of
  either branch.

Acceptance: an oversized 5xx is retryable; an oversized 200 is not; a
within-cap 5xx keeps its current behavior; the successful path is unchanged.

Verification: L1. Targeted `internal/usage` tests plus `go vet`.

### 4. `session-health-doc-accuracy`

Resolve the `CheckHealth` doc-versus-behavior contradiction.

The decision is to **correct the documentation, not the behavior**, for reasons
that must be recorded in the comment itself rather than left implicit:

- `immutable=1` asserts that no other process is writing, which is false
  whenever the optional watcher or a concurrent scan is running, and can
  surface as a spurious integrity failure.
- `nolock=1` skips the locking protocol, which in WAL mode can read a stale
  snapshot and would make `doctor` report on content the index no longer has.
- The committed database bytes are unchanged either way, and the sidecars are
  `0600` inside the `0700` state root, so no privacy or integrity boundary is
  involved.

Rewrite the `CheckHealth` comment to promise what it actually does — reads the
rebuildable index without migrating it or changing its committed contents, and
may materialize `0600` WAL sidecars as a side effect of opening a WAL database
read-only — and note why the two alternatives were rejected. Update the
`doctor_test.go:80-93` comment so it no longer describes the pin as recording
an unintended behavior; the pin stays.

Acceptance: the comment and the shipped test agree with observed behavior, and
no production behavior changes.

Verification: L0. `git diff --check` plus the targeted `internal/session`
tests, since the pinned test's comment is edited alongside.

### 5. `cache-creation-ttl-default`

Price a Claude cache-creation total that arrives without a TTL breakdown.

This task changes promised behavior. `docs/specs/cli-design.md:903-904`
currently says such totals stay unpriced and that the scanner never guesses.
The replacement rule must be a disclosed default, not a silent guess:

- **Storage stays faithful.** The parser keeps writing what the source said:
  `cache_creation_tokens` as reported, and the two TTL buckets exactly as the
  `cache_creation` object gave them, including zero. No rebuild is required for
  users to benefit, and no historical event is reinterpreted on disk.
- **Pricing applies the default TTL.** When `cache_creation_tokens > 0` and both
  TTL buckets are zero, price the total at the five-minute cache-write rate,
  which is the rate the importer already maps
  `cache_creation_input_token_cost` to (`cli-design.md:1182`) and the default
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
- Recomputed coverage figures are recorded in the plan's evidence when the task
  completes.

Verification: L2. Targeted `internal/usage` and `cmd/agentdeck` tests, then the
full vendor suite. Contract text is task 6.

**Scope switch.** This is the only task in the batch that changes user-visible
cost numbers. If `v0.2.2` should ship as robustness-only, drop this task and
task 6's specification row for it, and move both to `v0.3.0`; tasks 1-4 do not
depend on it.

### 6. `hardening-contract`

Record the shipped behavior in the contract and the manual, mirroring the
`attribution-contract` pattern.

- Increment `docs/specs/cli-design.md` once from whatever version is current at
  delivery, with one changelog row covering this batch.
- Rewrite the aggregate-cache-creation rule at lines 903-904 to state the
  disclosed five-minute default and the unchanged conservative handling of
  partial breakdowns.
- Update numbered rules 36 and 37 so the credential key's supported sealed key
  versions and the derived (not hashed) key ID are part of the promise, and so
  the durable directory entry is stated rather than implied.
- Update `docs/specs/cli-manual.md` wherever it shows `missing_components`,
  pricing completeness, or credential key diagnostics.
- Update `docs/README.md`: move this plan's rollup into Current State when the
  batch retires, and keep the Backlog free of the five promoted entries.

Acceptance: no specification statement contradicts shipped behavior, and the
changelog row names every behavior change in the batch.

Verification: L0. Documentation checks and `git diff --check`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `key-file-durability` | [ ] | [ ] |
| 2. `key-id-derivation` | [ ] | [ ] |
| 3. `price-retry-ordering` | [ ] | [ ] |
| 4. `session-health-doc-accuracy` | [ ] | [ ] |
| 5. `cache-creation-ttl-default` | [ ] | [ ] |
| 6. `hardening-contract` | [ ] | [ ] |

Order: tasks 1 and 2 are sequential because both edit
`internal/credentialvault/vault.go`. Tasks 3, 4, and 5 are independent of each
other and of 1-2. Task 6 runs last and requires 2 and 5 to be reviewed, since
it records what they shipped.

Commit boundaries follow task boundaries: one commit per task.

## Starting a task

Turn any Status row into a scoped instruction by naming its anchor:

> 进入开发：`credential-and-pricing-hardening` / `<task-anchor>`

Read, in order: `AGENTS.md`; this plan's Evidence Baseline and the task's own
section; every file the task names; and the verification routing in `AGENTS.md`
for the level the task declares. Implement only that task's scope. When the
task's own targeted verification passes, tick `Dev`, record the evidence
(commands and results) under the task, and leave the review trail for an
independent reviewer, who ticks `Review` only after a `Verdict: PASS` round in
`docs/reviews/credential-and-pricing-hardening/<task-anchor>.md`.
