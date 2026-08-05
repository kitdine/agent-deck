---
status: historical
created: 2026-07-29
retired: 2026-08-03
---

# Credential and Pricing Hardening

Target release: `v0.2.2`.

Three known defects promoted out of the `docs/README.md` Backlog. Each was
recorded there with its own evidence and left for "the next time that file is
opened"; this plan opens those files once, deliberately, instead of letting the
entries age further.

The batch is deliberately narrow: no new command, flag, database table, column,
error code, or output change. Every task here is PATCH-safe under
[the release versioning contract](release-versioning-contract.md) — the shipped
tree stays safe to downgrade to `v0.2.1`.

**Scope reduction, 2026-08-02.** This plan was designed on 2026-07-29 with six
tasks. Two of them — `key-id-derivation` and `cache-creation-ttl-default` — are
MINOR under the versioning contract adopted on 2026-08-02: the first makes newly
sealed rows unreadable by an earlier release, and the second changes user-visible
cost numbers. They moved, with their evidence, to
[the credential key and cache pricing plan](credential-key-and-cache-pricing.md)
targeting `v0.3.0`, together with the contract task that recorded them. Tasks 1,
3, and 4 below never depended on them.

## Goal

- Close the credential-key durability window that can strand ciphertext with no
  recoverable key.
- Make an oversized 5xx price response retryable, as the contract already
  promises.
- Stop claiming that read-only session diagnostics create no WAL sidecars when
  they do.

## Non-Goals

- No credential re-encryption sweep, no forced key rotation, and no automatic
  migration of existing ciphertext.
- No change to the credential key file format, its seed size, its HKDF salt or
  info string, the derived AES key bytes, or the persisted key ID.
- No change to the price catalog, its generator, or its pinned LiteLLM commit.
- No model-name special-casing anywhere in usage parsing or pricing.
- No new `doctor` check, recovery command, or `--fix` behavior.
- No specification version increment. Nothing here rewrites a promised behavior;
  the wording upgrade that states the durable directory entry rides with the
  `v0.3.0` contract task rather than forcing a spec bump into a patch release.

## Evidence Baseline

Gathered on 2026-07-29 at `2db056b`, before any task started.

**Credential key.** `internal/credentialvault/vault.go:244` links the temporary
seed file into place with `os.Link` and returns. The file's contents were synced
at line 236, but the parent directory entry never is.

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
- The derived AES key bytes and the persisted key ID are unchanged, so a vault
  written by this build opens under `v0.2.1` and the reverse.

Verification: L3. Targeted `internal/credentialvault` tests, then the full
vendor suite and `go vet`.

### 2. `price-retry-ordering`

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

### 3. `session-health-doc-accuracy`

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

Development evidence (2026-08-03):

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/session`
  -> PASS (`1.255s`; rerun after Round 1 P1 correction).
- `git diff --check` -> PASS.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `key-file-durability` | [x] | [x] |
| 2. `price-retry-ordering` | [x] | [x] |
| 3. `session-health-doc-accuracy` | [x] | [x] |

All three tasks are independent of each other and of every other `v0.2.2` plan.
Task 1 must land before `key-id-derivation` in the `v0.3.0` plan, because both
edit `internal/credentialvault/vault.go`.

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
