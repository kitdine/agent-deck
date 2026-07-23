---
status: active
created: 2026-07-22
---

# AgentDeck High-Value Test Implementation Plan

**Parent baseline:** [documentation and execution baseline](../README.md)

**Purpose:** Add focused unit and package-integration regression tests for the
highest-risk, evidenced gaps. This is an implementation queue, not a discovery
exercise: a later agent starts at one unchecked task below and does not repeat
coverage collection or repository-wide test-gap reconnaissance.

## Execution contract

- Before one task, read `AGENTS.md`, this plan, the named production file, and
  the named existing test file(s). Copy their fixture and assertion conventions.
- Add tests and synthetic fixtures only. Do not change production code,
  schemas, dependencies, generated files, or real-user state.
- Use temporary directories and isolated SQLite files; fake clocks, machine
  identities, random sources, and client configuration files where named. Do
  not read real HOME, Codex/Claude configuration, session logs, auth files,
  Keychain, credential keys, or backups.
- If a proposed test needs a production seam that does not already exist, or
  exposes a production defect, stop that task. Preserve the failing test and
  hand off the observed/expected behavior; do not broaden this test-only plan
  into a production change.
- A task's **Dev** gate is earned once its targeted command and its listed
  L2/L3 evidence pass against the final task content. Then tick that task's Dev
  cell in the Status matrix and record the command/result briefly in its
  completion note. Its **Review** gate is ticked separately by an independent
  reviewer once findings are closed; a reviewer reopens a task rather than
  ticking Review when it finds problems. A task is *done* only when Review is
  ticked. Do not create a second plan or re-run a repository-wide coverage scan
  merely to update this plan.

## Scan evidence (not an execution phase)

The following was collected once on 2026-07-22 with vendored dependencies:

```bash
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -covermode=atomic -coverprofile=/private/tmp/agent-deck-test-gap-20260722.out ./...
```

The command passed. The aggregate profile was **74.1%**; package-local results
were `internal/store` 53.8%, `internal/usage` 79.2%, `internal/provider`
75.0%, and `internal/credentialvault` 79.1%. These numbers prioritize
behavior; they are neither goals nor completion criteria. No CI coverage
threshold was found.

The queue deliberately excludes generated/vendor code, accessors and simple
configuration wrappers, plus behavior already covered by adjacent regression
tests. In particular, do not duplicate the existing store migration/lock tests,
usage exact-run/rebuild tests, provider switching/recovery tests, TOML/JSON
unmanaged-field preservation tests, or vault round-trip/fail-closed tests.

## Status

Task content lives in the queue below; per-gate status lives here.

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | store-boundaries | ✅ | ✅ |
| 2 | provider-persistence | ✅ | ✅ |
| 3 | usage-runstate | ✅ | ✅ |
| 4 | provider-backup | ✅ | ✅ |
| 5 | vault-init | ⬜ | ⬜ |
| 6 | session-health | ⬜ | ⬜ |

Done: 4/6. Tasks 1 through 3 passed independent review; task 4 passed a same-session re-review on 2026-07-23 after its round-1 assertion repairs, with that caveat recorded in its review log. The implementer ticks
**Dev** once a task's targeted L2/L3 evidence passes; a reviewer ticks
**Review** once findings are closed. A task is done only when Review is ticked.

## Direct implementation queue

### 1. Store open, backup, and scan-lock boundaries (P1)

- **Target files:** add cases to `internal/store/store_test.go` (or the
  existing focused store test file that already owns the helper); inspect
  `internal/store/store.go` and its existing migration/lock tests.
- **Behavior at risk:** inspection commands must not create or migrate state;
  a SQLite backup must contain a consistent private snapshot; scan locking must
  remain exclusive without colliding with the ordinary state lock.
- **Verified gap evidence:** function coverage showed no direct execution of
  `OpenReadOnly`, `BackupSQLiteFile`, `IntegrityCheck`, and `AcquireScanLock`.
  Adjacent tests already prove migration-failure preservation, state-lock
  privacy/exclusivity, lock ownership replacement, migration-lock blocking,
  and lock-release errors, so these are distinct observable boundaries.
- **Add these cases:**
  1. `OpenReadOnly` opens an existing compatible database without creating a
     missing database, migrating an older one, or enabling a write-side WAL
     change; it rejects a future schema.
  2. `BackupSQLiteFile` copies committed WAL-backed data to the destination;
     the backup can be opened and its file mode is private.
  3. `IntegrityCheck` returns success for a valid database and returns its
     observable error for a deliberately invalid/corrupt input using the
     existing test seam.
  4. `AcquireScanLock` is independent of the state lock, returns busy while a
     scan owner is live, respects context deadline/cancellation, and releases
     only its own scan ownership.
- **Fixtures and boundary:** temporary SQLite roots/files and the package's
  existing lock/migration helpers; use contexts with short deterministic
  deadlines. Do not introduce sleep-based timing or alter lock SQL.
- **Completion note (2026-07-22):** Added `internal/store/store_test.go` cases
   for `OpenReadOnly`, `BackupSQLiteFile`, `IntegrityCheck`, and `AcquireScanLock`;
   then passed:
   `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store`,
   `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...`,
   `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...`,
   `rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...`,
   and `rtk git diff --check`.
- **Stop rule:** stop if the current package offers no deterministic way to
  construct the corrupt/lock condition without changing production code.
- **Verification:** L3 because this adds concurrency-boundary evidence.

  ```bash
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
  rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
  ```

### 2. Store provider-operation and credential persistence (P1)

- **Target files:** add cases to `internal/store/providers_test.go` and
  the existing credential persistence test file; inspect
  `internal/store/providers.go` and the corresponding store credential code.
- **Behavior at risk:** provider selection journals and encrypted-credential
  metadata must be atomically recoverable. A failed write must not leave a
  completed operation, mismatched provider/client mapping, or partially updated
  credential state.
- **Verified gap evidence:** direct function coverage was absent for
  `CompleteProviderUse`, `UpdateProviderCredentialWithSecret`,
  `PendingOperations`, and `UpdateOperationDetails`. Existing provider
  migration, selection-timeline, and credential-transaction tests cover nearby
  happy paths, not these failure and ordering contracts.
- **Add these cases:**
  1. complete an operation successfully and assert the durable completed state;
     completing a missing operation returns the package's not-found/error
     contract.
  2. inject a transactional write failure into credential update and verify the
     secret metadata and provider/client mapping remain at their prior values.
  3. verify pending operations exclude completed entries and use the documented
     stable ordering.
  4. update operation details, then verify a write failure leaves the original
     details intact.
- **Fixtures and boundary:** package temp database plus existing trigger/error
  injection style and fake ciphertext/metadata values. Assert only metadata or
  ciphertext presence; never put plaintext secrets in fixture names, failures,
  or assertions.
- **Stop rule:** stop if the operation ordering or error type is not a
  documented/current observable contract; report the ambiguity instead of
  encoding a guessed ordering.
- **Verification:** L2 shared persisted-state contract.

  ```bash
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/store
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
  ```

### 3. Usage run-state wrappers and diagnostics (P1)

- Completion note (2026-07-22): `FailRun` now preserves closed exact-run state while
  asserting open-run default and custom failure reasons; scan-file `ReadAt` failure
  path still preserves prior attribution state.

- **Target files:** add cases to `internal/usage/usage_test.go`; inspect
  `internal/usage/usage.go` and the adjacent exact-run/rebuild tests.
- **Behavior at risk:** usage attribution must expose a truthful terminal run
  state, report diagnostics without leaking source contents, and retain prior
  stored state when scanning/reading a source fails.
- **Verified gap evidence:** function coverage was zero for `scanFile`,
  `Diagnose`, `CheckSourceReadability`, `FailRun`, `SetRunPID`, and `RunStatus`.
  Existing tests cover exact time/byte-range attribution, stale recovery,
  source rebuild, concurrent append, rollback, prices, and stats—none assert
  these wrapper and diagnostic contracts directly.
- **Add these cases:**
  1. `FailRun` records both the default and supplied failure reason and closes
     the run in the expected estimated/failed terminal state.
  2. `SetRunPID` followed by `RunStatus` returns the expected PID/status;
     querying a missing run returns the existing absence contract.
  3. `Diagnose` returns the documented table/count summary and propagates a
     database failure rather than presenting a false healthy result.
  4. `CheckSourceReadability` distinguishes missing, unreadable, and readable
     synthetic sources; errors/counts must not contain path or file-content
     leakage.
  5. Exercise `scanFile` through its current package seam for malformed/read
     failure and assert prior scan state remains unchanged. Cover
     `ParseMultiplier` and `ParseInt` only at their accepted/error boundaries
     that are consumed by this path.
- **Fixtures and boundary:** temporary usage database and source files, the
  package's injected `Now`/client-process interfaces, and existing SQLite
  failure fixture technique. Never invoke real `ps` or inspect real logs.
- **Stop rule:** stop if the output is intentionally non-contractual diagnostic
  text and no stable structured result exists; hand off the ambiguity.
- **Verification:** L2 shared usage and SQLite contract.

  ```bash
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/usage
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
  ```

### 4. Provider definition, credential lookup, and redacted backups (P1)

- **Target files:** add cases to `internal/provider/service_test.go` and
  the existing config/recovery test file; inspect
  `internal/provider/service.go` and `internal/provider/config.go`.
- **Behavior at risk:** provider edits must resolve the intended credential and
  preserve client configuration; user-visible credential lookup must reject
  ambiguity without exposing secret values; backup writes must redact secrets
  and remain private/atomic.
- **Verified gap evidence:** function coverage was absent for `UpdateDefinition`,
  `ResolveCredentialName`, and `ShowCredential`; `WriteRedactedBackup` and
  `atomicPrivateReplace` had only partial direct coverage. Existing tests
  already protect switching, official-provider behavior, journal recovery,
  ambiguous named credentials, unmanaged TOML/JSON fields, and basic redaction.
- **Add these cases:**
  1. update a definition with one resolved credential, explicitly selected
     credential, and ambiguous credentials; assert the official provider's
     rejection contract and subsequent metadata reads.
  2. assert `ResolveCredentialName`/`ShowCredential` for missing, unique, and
     multiple credentials; every assertion/failure must prove that plaintext is
     absent.
  3. write redacted Codex and Claude backups, including destination-parent
     creation, unsupported client, write failure, and rename failure; assert
     preservation/redaction plus `0600` output mode.
- **Round 1 repair note (2026-07-22):** task 4 is reopened because the
  ambiguous-update case does not prove every existing credential remains
  unchanged, and redacted-backup cases do not prove safe configuration fields
  are retained. See `docs/reviews/test-coverage/provider-backup.md`.
- **Round 1 repair (2026-07-23):** both closed.
  The ambiguous-update case now snapshots *every* credential of the provider —
  endpoint, multiplier, sorted client mapping, credential reference — right
  before the ambiguous call and requires exact equality after it fails, through
  a new `credentialSnapshot` helper. The point is ordering: `UpdateDefinition`
  resolves the credential before mutating anything, so an implementation that
  wrote first and detected the ambiguity second would leave the one credential
  the old test inspected untouched. RED confirms that reading — injecting
  exactly that defect fails with a before/after diff showing `default` moving
  to the ambiguous endpoint, and the pre-repair assertion passed the same
  injected defect.
  The redacted-backup case now parses both outputs and asserts what survives,
  not only what is absent: the Codex source carries `model_provider`, `model`,
  the custom provider's `name` and `wire_api`, and a `[features]` table, all of
  which must still be present; the Claude backup must still carry `env.OTHER`.
  Plaintext-absence and file-mode checks are retained, and each side also
  asserts the secret *key* is gone rather than just its value. RED on both
  halves independently: an empty-document redactor fails with `codex backup
  dropped the custom provider entirely`, and one deleting the whole `env` map
  fails with `claude backup dropped restorable env configuration` — both of
  which satisfied every pre-repair assertion.
  L3 verification at the repaired state: `gofmt -l` clean,
  `go vet -mod=vendor ./...` clean, `go test -mod=vendor -count=1 ./...` all 15
  packages ok, `go test -mod=vendor -race ./internal/provider` ok,
  `git diff --check` clean. Re-reviewed the same day (round 2, PASS) in the same
  session as the repair; that caveat is recorded in the review log.
- **Fixtures and boundary:** fake credential-vault implementation, temporary
  TOML/JSON files and destinations, existing config mutation helpers. Use
  synthetic credential strings only to prove absence from output.
- **Stop rule:** stop if a test would need to change real Codex/Claude config
  or a real vault/key file, or if an error is only host-filesystem dependent.
- **Verification:** L3 because credential privacy and atomic file replacement
  are security-sensitive boundaries.

  ```bash
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/provider
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
  rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
  ```

### 5. Credential-vault initialization failures (P1)

- **Target files:** add cases to `internal/credentialvault/vault_test.go`;
  inspect `internal/credentialvault/vault.go` and its existing round-trip,
  fail-closed, and concurrent-initialization tests.
- **Behavior at risk:** first key initialization must fail closed and accurately
  report whether a key file was created, so callers never overwrite a
  machine-bound credential key or mistake a failed initialization for a usable
  vault.
- **Verified gap evidence:** `InitializeNew` had no direct function coverage.
  Existing tests cover successful round trip, fail-closed reads, and
  same-machine concurrent initialization, leaving the creation/error semantics
  unprotected.
- **Add these cases:**
  1. an existing key is not overwritten;
  2. random-source failure and missing/error machine identity fail without a
     usable key;
  3. a failure after key-file creation returns the documented `created=true`
     result while preserving the created private file for recovery handling;
  4. assert key-file permissions and that errors never contain key material.
- **Fixtures and boundary:** temporary state root plus the existing injectable
  random/machine-identity seams. Use known synthetic byte sequences only; do
  not inspect real machine identity or key paths.
- **Stop rule:** stop if a required post-creation failure cannot be injected
  through an existing test seam without production changes; hand off that
  missing seam rather than adding one.
- **Verification:** L3 for key material and concurrent initialization.

  ```bash
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/credentialvault
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
  rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
  ```

### 6. Session-index health is read-only and privacy-safe (P1)

- **Target files:** add cases to `internal/session/session_test.go` (or a
  focused `internal/session/doctor_test.go` if that is the local convention);
  inspect `internal/session/doctor.go` and the existing doctor integration
  coverage in `internal/doctor/doctor_test.go`.
- **Behavior at risk:** `agentdeck doctor` must accurately report whether the
  separately purgeable session index is usable without creating, migrating, or
  changing that database. In full mode it must count inaccessible source files
  without reading or returning their contents or paths.
- **Verified gap evidence:** the follow-up atomic coverage profile collected
  on 2026-07-22 shows `session.CheckHealth` at 0.0% direct function coverage.
  `TestFullCheckReportsProblemsWithoutChangingDatabases` exercises its caller
  with a missing source, but does not assert the session database's own
  read-only/no-sidecar behavior or the direct missing/FTS/full-mode contracts.
  This is distinct from the already-covered session scanning, privacy allowlist,
  source-move, exclusion, and pagination behavior.
- **Add these cases:**
  1. a missing `sessions.sqlite3` returns `Present=false` and
     `Integrity="not_requested"` without creating any database, sidecar, or
     directory entry;
  2. an existing compatible temporary session index returns `Present=true`,
     `FTSAvailable=true`, and `Integrity="ok"` in non-full mode while its
     file mode, digest, and absence of new `-wal`/`-shm` sidecars are preserved;
  3. a temporary SQLite file with no `session_documents` table reports
     `FTSAvailable=false` rather than a healthy index;
  4. full mode returns SQLite integrity status and counts one missing and one
     permission-denied synthetic source alongside a readable source. Assert
     only the count and health fields: no test failure, result, or helper may
     include source contents or paths.
- **Fixtures and boundary:** create temporary session databases via the
  existing `store.OpenSessions` helper, plus a raw temporary SQLite fixture
  only for the missing-FTS-table case. Use fake source text and local temporary
  paths. Do not open the real session index, real Codex/Claude logs, or HOME;
  do not change `CheckHealth` or add an OS/filesystem seam.
- **Stop rule:** stop if a permission-denied fixture is not portable on the
  supported test runner (for example, a privileged runner can still open a
  `0000` file). Report that environment limitation instead of adding sleeps,
  platform-specific production logic, or a fake filesystem abstraction.
- **Verification:** L3 because the diagnostic reads privacy-sensitive local
  session state and its no-mutation contract is security-relevant.

  ```bash
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./internal/session
  rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
  rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
  ```

## Completion and follow-up

When every task's Review gate is ticked (all six done), run one final L3
evidence set against that unchanged content state. Record any remaining high-risk behavior as a newly
scoped backlog item; do not turn low coverage alone into another plan.

```bash
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
rtk git diff --check
```

`make release-verify` is L4 and is not required for this test-only queue unless
a later change reaches release-artifact scope or the maintainer requests it.

## Sources and confidence

**Sources inspected for the scan:** `AGENTS.md`; `docs/README.md`; the parent
backlog plan; `go.mod`, vendored dependencies, Makefile/CI test commands;
existing representative store, usage, provider, and credential-vault tests;
and focused implementation/function-coverage inspection of those three module
groups.

**Confidence:** high for the six direct tasks: each is anchored in observed
uncovered function behavior and differentiated from nearby existing tests.

**Follow-up scan (2026-07-22):** a fresh atomic profile and focused inspection
of `internal/session`, `internal/backup`, and `internal/watch` added task 6.
`session.CheckHealth` was selected over the zero-covered `backup.List` because
the latter is a low-risk sorted directory listing, and over `watch.Run` because
the current timer-only loop has no deterministic timing seam for a test-only
task. Confidence is high for task 6's direct read-only and health-state cases;
the permission-denied fixture remains runner-dependent as documented above.

**Open questions deliberately left out of this queue:** lower-coverage watch,
backup, CLI, and platform packages were not promoted because this scan did not
inspect enough behavior to specify tests without guessing. Reassess them only
through a separately scoped gap analysis after this queue is complete.
