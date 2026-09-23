---
status: active
created: 2026-09-22
updated: 2026-09-22
---

# Health Recovery — Architecture

## Purpose

This document resolves the provisioning contracts for AgentDeck health and lock recovery, fulfilling the data requirements specified in the surface frameworks (`cli-health-recovery.md`, `menubar-health-recovery.md`). It defines the wire schemas, execution boundaries, lock inspection algorithms, and failure ordering required to implement cause-specific recovery.

## Data Contracts and Provisioning

The architecture provisions all requested UX fields via stable, additive additions to the structured JSON health check output and the Desktop wire format.

### Health Check Schema (Desktop Wire & CLI JSON)

To support the surface requirements while keeping older clients compatible, the health status array elements will include additive fields. Missing additive fields will be safely ignored by older desktop clients, which will continue to render the default generic health warning.

Additive fields for a `Check` object, and their mappings to Desktop Wire (`internal/desktop/desktop.go` -> `desktop/fixtures/v1` -> `apps/macos/AgentDeckShared/DesktopWire.swift`):

- `resource` (string): Stable identifier of the failing subsystem (e.g., `state`, `scan`, `extension_inventory`).
- `reason` (string): Stable error code or condition class.
- `action_kind` (string, nullable): Typified classification of the recovery action (`diagnose`, `retry`, `synchronize_inventory`, `manual_prerequisite`).
- `recovery_command` (string, nullable): Executable shell command strictly reserved for safe, mutating recovery.
- `diagnostic_command` (string, nullable): Executable shell command strictly reserved for diagnosis.
- `manual_prerequisite` (string, nullable): A stable localization key explaining external confirmation steps. The client resolves this key to localized prose; an unrecognized key is not emitted.
- `count` (integer, optional): Bounded count of affected entities. Reuses the existing `count` field.

New clients encountering an unknown token in `resource`, `reason`, or `action_kind` must fail closed: render a generic warning, treat `action_kind` as null, and do not expose any recovery command.

**`manual_prerequisite` key set.** Each `reason` whose `action_kind` is `manual_prerequisite` has exactly one key; `extension_state_missing` takes that action only when discovery failed, and then reuses the discovery key because discovery is the blocking prerequisite. No other reason emits this field. `lock_owner_unknown` has `action_kind: null` (fail-closed diagnosis, not a manual step) and therefore has no key.

| Key | Reason it covers | Localized meaning |
| --- | --- | --- |
| `prereq_legacy_lock_removal` | `lock_legacy` | Confirm no AgentDeck process is using the state directory, then remove only the identified legacy lock file. |
| `prereq_extension_discovery_failed` | `extension_discovery_failed`; also `extension_state_missing` when discovery failed (see the reason -> action mapping) | Resolve the native discovery error (unreadable config, permissions) before diagnosis or synchronization can proceed. |
| `prereq_extension_inventory_unreadable` | `extension_inventory_unreadable` | Resolve the unreadable or malformed AgentDeck database before extension diagnosis can run. |
| `prereq_extension_duplicate_id` | `extension_duplicate_id` | Resolve the duplicate native definitions that produce the same canonical identity before synchronizing. |
| `prereq_extension_managed_drift` | `extension_managed_drift` | Use the existing `agentdeck extension adopt`/`release` ownership commands to reconcile the drifted fingerprint. |
| `prereq_extension_management_anomaly` | `extension_management_anomaly` | Re-adopt or release the extension to repair inconsistent management metadata. |
| `prereq_extension_native_unavailable` | `extension_native_unavailable` | Restore the native path or capability outside AgentDeck; synchronization cannot repair an unavailable native resource. |

### Typed Command Error (`state_busy` and `extension_sync_incomplete`)

To provide typed data for CLI commands that fail during lock acquisition or scanning:

- **`ErrLockContention`**: A new typed error wrapping the existing `ErrStateBusy` (which retains exit code 1 and `Unwrap() -> ErrStateBusy`, so `errors.Is(err, store.ErrStateBusy)` remains true). It carries `Resource`, `Reason`, `ActionKind`, and `RecoveryCommand` (always `nil`; no lock-contention state has a safe executable command).
  - Only `AcquireLock` (`state.lock`, `resource: state`) and `AcquireScanLock` (`scan.lock`, `resource: scan`) construct `ErrLockContention`. Each wraps the `ErrStateBusy` returned by `acquireNamedLockWithChecks` before returning it to its caller. `AcquireQuotaRefreshLock` and `AcquireDerivedSnapshotCacheLock` keep returning the plain `ErrStateBusy`. At least one of them reaches a command error path: `runDesktopQuotaRefresh` (`cmd/agentdeck/quota.go:378`, invoked by `agentdeck desktop quota refresh`, including the user-initiated `--manual` form) returns the plain error on quota-refresh lock contention. That caller is a known `resource: unknown` producer by design: the schema-version-1 `resource` vocabulary frozen by the CLI JSON contract (`state`, `scan`, `extension_inventory`, `unknown`) has no quota or cache value, and adding one requires reviewed versioning. It therefore yields the CLI's existing `resource: unknown` specimen (`next: Run read-only diagnostics before taking recovery action.`, `diagnose: agentdeck doctor`) rather than being mislabelled as `state` or `scan`.
  - `Reason` is filled by running the shared classifier defined under [Lock Contention and Liveness](#lock-contention-and-liveness) against the lock file one more time at the moment of timeout. `ActionKind` follows deterministically from `Reason` via the same reason -> action table used by doctor (`lock_live` -> `retry`, `lock_legacy` -> `manual_prerequisite`, `lock_owner_unknown` -> `null`, `lock_reclaimable` -> `null`, the last being unreachable here in practice because a reclaimable lock is reclaimed inline by the acquisition loop before timeout can be reached, but the classifier still returns it defensively if the lock file changes between the reclaim attempt and the timeout check).
  - If the classifier itself cannot open or stat the lock file at timeout (e.g., it was removed and recreated mid-check), `Resource` keeps its known value (`state` or `scan`) but `Reason`/`ActionKind` fall back to `lock_owner_unknown`/`null` rather than failing the command differently.
  - The CLI projects `Resource`/`Reason`/`ActionKind`/`RecoveryCommand` into text/JSON `error.details` via `errors.As(err, &lockContention)`. An error that is a plain `ErrStateBusy` rather than the wrapped type — the quota-refresh and derived-snapshot-cache lock domains above, or any future unclassified caller — yields `resource: unknown` with `reason` and `action_kind` omitted.
- **`ErrExtensionSyncIncomplete`**: A new typed error (exit code 1) returned by `agentdeck extension scan` when the inventory commits but the subsequent fingerprint write fails. It yields `reason: extension_fingerprint_update_failed`, `action_kind: diagnose`, `recovery_command: nil`, and `inventory_committed: true`.
  - **Persisted marker.** Doctor has no other way to learn that a prior scan's post-commit fingerprint write failed, so the scan command persists that fact as an AgentDeck-owned setting, `extension.sync_incomplete` (stored the same way as the existing `watch.fingerprint.extension` setting, via `database.SetSetting`). When the fingerprint write fails after a successful inventory commit, the scan command additionally writes `extension.sync_incomplete = "true"` (best-effort; if this second write also fails, `ErrExtensionSyncIncomplete` is still returned — the marker is an enhancement to doctor visibility, not a precondition of the command's own typed error). The next `agentdeck extension scan` invocation that successfully writes `watch.fingerprint.extension` clears the marker (`database.SetSetting(ctx, "extension.sync_incomplete", "")`) in the same request, so a later fully successful scan always retires a stale flag.
  - The read-only extension doctor entry (see "Read-Only Extension Doctor" below) reads this setting alongside its discovery-based classification. When set, it reports `extension_fingerprint_update_failed` as an additional condition independent of live discovery — this reason is never produced from the discovery comparison itself, only from this marker.

## Execution and Ownership Boundaries

### Lock Contention and Liveness

The `internal/store` package natively owns all named locks via `acquireNamedLock*`. A modern lock token has the form `v1:<pid>:<32-hex-random>`, written by `newLockToken` and parsed by the existing `lockOwnerPID(token) (pid int, ok bool)`; `ok` is `false` for anything that does not match that exact shape, which is the deterministic legacy test below. Liveness itself is answered by the existing platform-specific `lockProcessAlive(pid) (alive, known bool)` (`syscall.Kill(pid, 0)` on Darwin: `nil`/`EPERM` -> `(true, true)`, `ESRCH` -> `(false, true)`, anything else, and every call on an unsupported platform, -> `(false, false)`).

**Classifier (`ClassifyLock`, new exported function on `internal/store`, reused by both the acquisition timeout path and doctor).** Given a lock file's path:

1. If the file does not exist, there is no contention; callers stop before invoking the classifier.
2. If the file exists but cannot be opened or stat'd (permission error, race with concurrent removal that is not a clean not-exist), the file is treated as **`lock_owner_unknown`**.
3. Read the file contents and call `lockOwnerPID`. If it returns `ok: false` (legacy, pre-`v1` token, or corrupted content), the result is **`lock_legacy`**. This is a content-shape test, not an age test, so a freshly written malformed token and a five-year-old one classify identically.
4. Otherwise call `lockProcessAlive(pid)`:
   - `known: true, alive: true` -> **`lock_live`**.
   - `known: true, alive: false` -> **`lock_reclaimable`** (the existing acquisition-time reclaim protocol, `reclaimLockFromDeadProcess`, would remove this file the next time any process attempts to acquire this lock name).
   - `known: false` (unsupported platform, or an unexpected `syscall.Kill` error other than `ESRCH`/`EPERM`) -> **`lock_owner_unknown`**.

No step reads the file's modification time or any other age signal; age never participates in this classification, satisfying the requirement that age alone never turns a lock into a removal path.

**PID reuse is an accepted, bounded risk, not an open contradiction.** `lock_live` only ever yields `action_kind: retry` (wait and retry the command); it never yields a manual-removal or recovery step. If the PID that owned a now-dead lock is reused by an unrelated live process before classification runs, the worst observable outcome is a false "wait and retry" message that eventually times out again on retry — never an unsafe deletion, because deletion is only ever offered from the `lock_legacy` path, which is independent of `lockProcessAlive` entirely (step 3 above resolves before step 4 is reached). `lock_reclaimable` carries the same PID-confirmed-dead guarantee the existing reclaim protocol already relies on for its inline (non-doctor) reclaim, so this classifier introduces no new risk beyond what `reclaimLockFromDeadProcess` already accepts.

- **Automatic Reclaim**: Unchanged from the current protocol — a modern lock whose owner is confirmed dead (`lock_reclaimable` under this classifier) is reclaimed inline during normal lock acquisition via the existing `reclaimLockFromDeadProcess`, guarded by `flock` against concurrent reclaimers and re-verified by `os.SameFile`/content comparison before removal.
- **Doctor Read-Only**: `agentdeck doctor` independently classifies `state.lock` and `scan.lock` using the same `ClassifyLock` function, with no side effects (it never opens the file for writing, flocks it, or removes it):

  | Reason | Doctor status | Code | `action_kind` |
  | --- | --- | --- | --- |
  | (lock file absent) | `ok` | *(none)* | *(none)* |
  | `lock_live` | `warning` | `lock_live` | `retry` |
  | `lock_legacy` | `warning` | `lock_legacy` | `manual_prerequisite` (`prereq_legacy_lock_removal`) |
  | `lock_owner_unknown` | `warning` | `lock_owner_unknown` | `null` |
  | `lock_reclaimable` | `ok` | `lock_reclaimable` | *(none — informational only; not counted toward `Problems`/`Warnings`)* |

  `lock_reclaimable` is reported as `ok` with an informational code, matching the requirement that the CLI must not ask the user to perform work the next acquisition already handles safely.

- **Old-code migration**: `checkLock`'s previous age-based heuristic (`stale_lock` at >10 minutes, `state_busy` at ≤10 minutes) and the separate `lock_unreadable` code are retired and replaced one-to-one by the table above, not by a parallel code:

  | Retired code | Trigger it replaced | New source of truth |
  | --- | --- | --- |
  | `stale_lock` | file age > 10 minutes | `ClassifyLock`'s outcome for that file — typically `lock_reclaimable` when the owner is actually dead, but `lock_live`/`lock_legacy`/`lock_owner_unknown` when it is not, since age never correlated reliably with liveness |
  | `state_busy` (doctor's per-lock code, distinct from the top-level `store.ErrStateBusy` command error) | file age ≤ 10 minutes | same `ClassifyLock` outcome; most often `lock_live` |
  | `lock_unreadable` | `os.Stat` error opening the lock file itself | `lock_owner_unknown` (classifier step 2) |

### Extension Diagnostics Classification

The `internal/extension` package owns extension discovery and inventory diagnosis.

**Distinguishing stale inventory from native unavailability.** `extension.Doctor` compares the persisted inventory (`db.ListExtensions`) against a fresh `discover()` pass, keyed by canonical ID:

**Comparisons require successful discovery.** Stale inventory, native unavailability and managed drift are persisted-versus-live comparisons. They are computed only when `discover()` returned `discoveryErr == nil`, preserving the existing `if discoveryErr != nil { continue }` guard in `extension.Doctor` (`internal/extension/extension.go:173-175`). When discovery fails, `discovery_status` is `"failed"`, the `stale_inventory`, `missing_paths`, `native_unavailable`, `duplicate_ids` and `drifted_ids` collections are reported as unavailable rather than empty (text prints `unavailable`, matching the CLI discovery-failure specimen), and no stored ID is ever reported as stale. `extension_management_anomaly` is the only condition that does not depend on discovery: it reads stored rows alone (`Managed && AdoptedFingerprint == ""`) and is still computed, but it is also reported as unavailable in this case so that the discovery-failure output stays exactly as the CLI specimen shows.

- **`extension_stale_inventory`**: discovery succeeded and a persisted ID is entirely absent from `currentByID` — the identity itself was not rediscovered at all (today's `MissingPaths` logic, which already runs only under `discoveryErr == nil`). This is the shape of the reported bug: an identity migrated (e.g., `codex:mcp:user:computer-use` -> a different canonical ID) and the old row simply never reappears.
- **`extension_native_unavailable`**: discovery succeeded and a persisted ID *is* rediscovered under the same canonical ID (so it is not stale), but that item's native source is unavailable. The signal is the item's own `fingerprint(sourcePath)` result.

**Per-item fingerprint policy.** Today `discover()` aborts the whole pass when any one candidate's `fingerprint(sourcePath)` fails (`extension.go:269-272`), so the most common form of native unavailability — a source path that no longer resolves or cannot be read (`fingerprint` returns `source unavailable` or `source unreadable`) — currently surfaces as `extension_discovery_failed`. This changes: a per-candidate fingerprint failure no longer aborts discovery. `discover()` still emits that candidate's `store.Extension` row with its canonical identity, sets `Fingerprint` to `""`, and records the safe category (`source_unavailable` or `source_unreadable`, never the raw path or OS error) in the row's existing per-item `Diagnostics []string` field, which is otherwise the presently-always-empty `[]string{}`. `extension.Doctor` collects every rediscovered ID whose `Diagnostics` is non-empty into a new `NativeUnavailable []string` field on `DoctorReport`. Consequences:

- `extension_discovery_failed` is reserved for scanner-level failures (a scanner in `discover` returning an error, such as an unparseable native configuration file), after which no per-item conclusion is trustworthy.
- Managed-drift comparison skips an item with an empty fingerprint: an unavailable source is reported as `extension_native_unavailable`, never also as `extension_managed_drift`.
- `agentdeck extension scan` persists the row with its empty fingerprint and per-item diagnostic (the `diagnostics_json` column already exists); the identity is kept rather than removed. Synchronization therefore does not clear `extension_native_unavailable`, which matches its `manual_prerequisite` action: only restoring the native source clears it.

Stale inventory and native unavailability are mutually exclusive by construction: an ID is either absent from `currentByID` (stale) or present with a diagnostic (native-unavailable); it cannot be both.

**Non-fatal discovery diagnostics are informational, not health problems.** A successful `discover()` still returns non-fatal `diagnostics` for candidates skipped because their canonical ID is invalid (`extension.go:264-267`). They receive no reason from the frozen vocabulary: no AgentDeck action can change a native definition AgentDeck cannot represent, and a new reason token would require reviewed versioning. `extension doctor` keeps listing them, sanitized, in its `diagnostics` section with `discovery_status: "ok"`; they do not set a `reason`, do not change the report's status, and are no longer added to the aggregate doctor's problem count (retiring the `len(extensionReport.Diagnostics)` term summed at `internal/doctor/doctor.go:168`).

The remaining seven reasons keep their existing producers:

1. `extension_state_missing`: the read-only entry's underlying `store.OpenReadOnly` fails with "state root or database absent."
2. `extension_inventory_unreadable`: the read-only entry's open fails for any other non-future-schema reason (malformed or incompatible database).
3. `extension_discovery_failed`: a scanner-level failure makes `discover()` return a non-nil `err` (today's `discoveryErr`); `report.Diagnostics` carries its sanitized message.

Reasons 1 and 2 are produced by `agentdeck extension doctor` only. The aggregate doctor opens the same database first and returns early with its existing `state` (`state_missing`) or `database` row when that open fails (`internal/doctor/doctor.go:55-83`), so its `extensions` row is never emitted in those cases and these two reasons never become its primary reason.
4. `extension_stale_inventory`: as above.
5. `extension_duplicate_id`: `DuplicateIDs` (unchanged — duplicate canonical IDs within one `discover()` pass).
6. `extension_managed_drift`: `DriftedIDs` (unchanged — `Managed && AdoptedFingerprint != live.Fingerprint`).
7. `extension_management_anomaly`: `ManagementAnomalies` (unchanged — `Managed && AdoptedFingerprint == ""`).
8. `extension_native_unavailable`: as above.
9. `extension_fingerprint_update_failed`: produced exclusively from the `extension.sync_incomplete` persisted marker described under [Typed Command Error](#typed-command-error-state_busy-and-extension_sync_incomplete); `extension.Doctor` never derives it from discovery, since a doctor pass with no prior failed scan has no way to observe it.

**Aggregation and priority.** `extensionReport, err := extension.Doctor(...)` still yields one aggregate `extensions` check for `agentdeck doctor`. When more than one of the nine conditions is present, the aggregate `Code` is the highest-priority condition present, in this order (most severe/blocking first, so a condition that invalidates every other comparison always wins; then integrity/identity problems that risk acting on wrong data; then conditions with a safe, targeted recovery; then the orthogonal, informational marker last):

1. `extension_state_missing`
2. `extension_inventory_unreadable`
3. `extension_discovery_failed`
4. `extension_duplicate_id`
5. `extension_management_anomaly`
6. `extension_managed_drift`
7. `extension_stale_inventory`
8. `extension_native_unavailable`
9. `extension_fingerprint_update_failed`

All present conditions remain enumerated in their own JSON arrays regardless of which one is chosen as the primary `reason`; the priority order only selects the single top-level `extensions` check code and `count` in the aggregate doctor, matching `extensions: warning (extension_stale_inventory; count=1)` in the CLI framework's worked example.

**Reason -> action mapping.** Every extension reason has exactly one row. The same values populate the typed fields of `extension doctor` JSON, the aggregate doctor's `extensions` check (for its primary reason), and the desktop wire copy of that check:

| Reason | Status | `action_kind` | `recovery_command` | `manual_prerequisite` | Aggregate `diagnostic_command` |
| --- | --- | --- | --- | --- | --- |
| `extension_state_missing` | `warning` | `synchronize_inventory` when discovery succeeded; otherwise `manual_prerequisite` | `agentdeck extension scan` only when discovery succeeded; otherwise `null` | `prereq_extension_discovery_failed` only when discovery failed | not applicable (`extension doctor` only) |
| `extension_inventory_unreadable` | `error` (exit 1) | `manual_prerequisite` | `null` | `prereq_extension_inventory_unreadable` | not applicable (`extension doctor` only) |
| `extension_discovery_failed` | `warning` | `manual_prerequisite` | `null` | `prereq_extension_discovery_failed` | `agentdeck extension doctor` |
| `extension_duplicate_id` | `warning` | `manual_prerequisite` | `null` | `prereq_extension_duplicate_id` | `agentdeck extension doctor` |
| `extension_management_anomaly` | `warning` | `manual_prerequisite` | `null` | `prereq_extension_management_anomaly` | `agentdeck extension doctor` |
| `extension_managed_drift` | `warning` | `manual_prerequisite` | `null` | `prereq_extension_managed_drift` | `agentdeck extension doctor` |
| `extension_stale_inventory` | `warning` | `synchronize_inventory` | `agentdeck extension scan` | `null` | `null` |
| `extension_native_unavailable` | `warning` | `manual_prerequisite` | `null` | `prereq_extension_native_unavailable` | `agentdeck extension doctor` |
| `extension_fingerprint_update_failed` | `warning` | `diagnose` | `null` | `null` | `agentdeck extension doctor` |

Rules that apply across the table:

- `recovery_command` is `agentdeck extension scan` only for `extension_stale_inventory`, and for `extension_state_missing` after successful discovery, because only then does synchronization change the diagnosed condition. `synchronize_inventory` is the only `action_kind` that carries a recovery command in this domain.
- The aggregate `extensions` row is built from its primary reason. It carries `diagnostic_command: agentdeck extension doctor` exactly when its `recovery_command` is `null`, so the read-only command is always labelled `diagnose:` and never presented as the repair. It is `null` for stale inventory, whose row carries the scan command and the effect line instead.
- Within `agentdeck extension doctor` itself, `diagnostic_command` is always `null` (the user is already running it). Its text keeps the separate per-condition sections from the CLI framework; it prints `recovery: agentdeck extension scan` whenever stale inventory is present and discovery succeeded, even when another condition is present too, and otherwise prints the `next:` prose for the primary reason.
- The aggregate doctor replaces its current fixed `Code: "extension_diagnostics"` and `Recovery: "agentdeck extension doctor"` (`internal/doctor/doctor.go:168-170`) with the primary reason and the row above. `extension_diagnostics` is retired; `count` becomes the number of affected IDs for the primary reason (or `1` for the marker-only `extension_fingerprint_update_failed`).

**JSON fields (`DoctorReport`, both `extension doctor` and the aggregate doctor's `extensions` check detail).** The historical `missing_paths` name is preserved for backward compatibility and is populated from the same data as the new, correctly named field:

| Field | Meaning | Compatibility |
| --- | --- | --- |
| `discovery_status` (new) | `"ok"` or `"failed"`, mirrors whether `discoveryErr` was non-nil | new field |
| `stale_inventory` (new) | canonical IDs absent from rediscovery | new, additive |
| `missing_paths` | identical contents to `stale_inventory` | retained; existing consumers keep working unmodified |
| `duplicate_ids` | unchanged | unchanged |
| `drifted_ids` | unchanged | unchanged |
| `management_anomalies` | unchanged | unchanged |
| `native_unavailable` (new) | canonical IDs rediscovered with a non-empty per-item `Diagnostics` | new field |
| `fingerprint_sync_incomplete` (new, bool) | value of the `extension.sync_incomplete` marker | new field; `extension doctor` also surfaces `reason: extension_fingerprint_update_failed`, `action_kind: diagnose` when true |
| `reason`, `action_kind`, `recovery_command`, `manual_prerequisite` (new) | the primary reason and its row from the reason -> action mapping | new fields; omitted or `null` when healthy |

When `discovery_status` is `"failed"`, `stale_inventory`, `missing_paths`, `native_unavailable`, `duplicate_ids`, `drifted_ids` and `management_anomalies` are JSON `null` (unavailable), never `[]`, so automation cannot read a failed comparison as "no problems". `diagnostics` always remains an array.

**Read-Only Extension Doctor**: To avoid lock contention, the extension doctor (and aggregate doctor calling `extension.Doctor`) opens the database using `store.OpenReadOnly()`. If the state is missing, schema is future, or DB is unreadable, it outputs the respective reasons without mutating state.

## Ordering and Prioritization

### Command Error Precedence

1. `schema_ahead` remains the highest precedence command error; an unresolved future-schema probe always wins over a concurrently busy lock.
2. `ErrLockContention` (`state_busy`, carrying `Resource`/`Reason`/`ActionKind`) or other initialization errors.

### Doctor Row Ordering

`agentdeck doctor` and `agentdeck doctor --full` retain the existing implementation's exact row order; only `scan_lock` is a new insertion, placed immediately after `state_lock` since both are read by the same lock-classification step. This is the complete, implementable order from `Service.Check`, `checkLock`, `checkProviders`, and `checkUsage`:

1. `state` (only emitted on the early-return path when the state root itself is missing)
2. `state_permissions`
3. `state_lock`
4. `scan_lock` *(new — `checkLock` is extended to classify `scan.lock` the same way it already classifies `state.lock`)*
5. `hook_deliveries`
6. `database` (only emitted on the early-return path when the database itself cannot be opened)
7. `schema`
8. `database_integrity` (full mode only)
9. `pending_operations`
10. `provider_operation_state` (zero or more rows, only when `pending_operations` found operations)
11. `provider_credentials`
12. `provider_credential_key`
13. `provider_credential_authentication` (full mode only)
14. `provider_configuration`
15. `project_attribution_gate`
16. Usage group, taking one of these forms:
    - `database` (`schema_incompatible`), when the read-only catalog probe for `usage_tool_calls` failed; or
    - the `checkUsage` rows in order: `usage`, `usage_sources` (full mode only), `prices`, `price_provenance` (full mode only), `unpriced_models` (full mode only); or
    - a prefix of those `checkUsage` rows followed by one `usage` row (`schema_incompatible`), when `checkUsage` returns an error partway through.
17. `sessions`
18. `session_sources` (full mode only)
19. `extensions`

This guarantees that a locked state database does not mask an independent stale extension inventory, and vice versa.

## Desktop Refresh Semantics

A copied external action (e.g., running `agentdeck extension scan` in a terminal) does not synchronously notify the desktop app. The menu-bar app relies on its existing polling interval or a manual user click on "Refresh". Only a subsequently fetched snapshot proving the condition has cleared will remove the notice and health row. The act of clicking "Copy" triggers a 1.6-second `Copied` feedback but mutates no health state.

## Security and Privacy Refusals

- **No Raw Paths/Tokens**: The wire format explicitly refuses to provision raw SQLite errors, native config paths, lock tokens, or full extension manifest contents.
- **No Destructive Commands**: Any condition requiring manual cleanup (e.g., `lock_legacy`) is refused a `recovery_command` and is instead provisioned with a `manual_prerequisite` localization key.
- **No Silent Mutators**: The desktop app and read-only diagnostics (`doctor`) are refused any capability or endpoint to silently delete locks or update inventory.

## Test Seams

To satisfy the verification requirements for both origins without mocking the filesystem or SQLite CGO layer, the architecture defines these test seams:

- **Lock Liveness Override**: A function injection seam via `acquireNamedLockWithChecks(processAlive, tryReclaimLock)` allows tests to simulate live/dead processes without real dummy processes. `ClassifyLock` accepts the same `lockProcessCheck` injection so doctor tests can assert each of the four reasons deterministically.
- **Mock Extension Discovery**: An interface seam for `ExtensionDiscoverer` (introduced to wrap the package-level `discover` function) allows tests to inject synthetic discovery failures, duplicate identities, or per-item native-unavailable diagnostics, ensuring the inventory synchronization logic can be tested in isolation.
- **Wire Format Verification**: JSON serializers for the desktop wire format include strict validation test assertions to ensure no private fields leak into the additive fields.

## Acceptance Boundary

| Requirement Scenario | Responsible Component | Test Seam / Verification |
| --- | --- | --- |
| Live `state.lock` owner | `internal/store` | `ClassifyLock` injection simulating `known:true, alive:true`; assert `lock_live` with `action_kind: retry` and no `recovery_command`. |
| Live `scan.lock` owner | `internal/store`, `internal/doctor` | Doctor output asserts `scan_lock` reason distinct from `state_lock`. |
| Legacy lock token | `internal/store` | Injection of a non-`v1` token; assert `lock_legacy`, `action_kind: manual_prerequisite`, `manual_prerequisite: prereq_legacy_lock_removal`. |
| Unknown or unreadable lock ownership | `internal/store` | Injection of `known:false` (or unsupported platform) and of a stat failure; assert `lock_owner_unknown` with `action_kind: null`. |
| Safely reclaimable modern stale lock | `internal/store`, `internal/doctor` | Normal acquisition reclaims via `reclaimLockFromDeadProcess`; a read-only doctor pass before reclaim asserts `lock_reclaimable` as `ok`, not a warning. |
| Doctor under contention | `internal/doctor` | Assert both lock classes reported correctly under read-only mode without mutating. |
| Future schema plus contention | CLI command initialization | `schema_ahead` precedence assertion over `ErrLockContention`. |
| Stale persisted MCP identity | `internal/extension` | `ExtensionDiscoverer` interface injecting a rediscovery set missing the old ID; assert `extension_stale_inventory`, `action_kind: synchronize_inventory`, `recovery_command: agentdeck extension scan`, and no `diagnostic_command` on the aggregate row. |
| Native path genuinely unavailable | `internal/extension` | Fixture whose rediscovered candidate has an unresolvable `sourcePath`; assert discovery still succeeds, the row carries an empty fingerprint and a `source_unavailable` diagnostic, the result is `extension_native_unavailable` (not `extension_discovery_failed`, `extension_stale_inventory` or `extension_managed_drift`), and a following scan keeps the row. |
| Extension synchronization | `internal/extension` | Sync command completes and removes stale IDs; verify unchanged client config. |
| Discovery failure | `internal/extension` | `ExtensionDiscoverer` interface injecting a scanner-level failure; assert existing inventory preserved, `discovery_status: "failed"`, discovery-dependent collections `null`, and no stored ID reported as stale. |
| Invalid canonical ID candidate | `internal/extension`, `internal/doctor` | Fixture with a skipped invalid candidate; assert a sanitized `diagnostics` entry, `discovery_status: "ok"`, no reason, and no aggregate problem count. |
| Other extension diagnostics | `internal/extension` | Assert duplicate, managed drift, and management anomaly conditions are reported with the priority order's chosen primary reason. |
| Fingerprint write fails after inventory commit | `internal/extension`, `internal/doctor` | Scan fixture that fails the post-commit fingerprint write asserts `ErrExtensionSyncIncomplete`, `inventory_committed: true`, and the `extension.sync_incomplete` marker; a subsequent doctor pass asserts `extension_fingerprint_update_failed` with `action_kind: diagnose`; a later fully successful scan clears the marker. |
| Structured output and desktop wire | `internal/desktop` | Wire fixture tests asserting `Resource`, `Reason`, `ActionKind` serialization and fallback. |
| macOS recovery presentation | `apps/macos/.../DesktopWire.swift` | UI layer tests for fail-closed decoding and label distinctions. |
