---
status: active
created: 2026-09-21
updated: 2026-09-22
---

# Health Recovery — Tasks

This is the topic-local execution and status authority for `health-recovery`.
Its planning carrier is `ad-health-recovery`; the retained defect origins are
`ad-bug-state-busy-recovery-guidance` and
`ad-bug-extension-stale-mcp-recovery`.

Workspace: `agent-deck.health-recovery`; branch `feature/health-recovery`;
creation base `a396c2f2158f579e70da3aaf9bd0bff084fc1f1a`. Design, implementation,
Git delivery, integration, retirement and release remain separate boundaries.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/cli-health-recovery.md | [x] | [x] |
| ux/menubar-health-recovery.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

The document set keeps CLI and menu-bar interaction contracts independently
reviewable while one architecture owns shared reason/action semantics and state
transitions. Both origin bugs remain independent acceptance tracks; neither is
closed merely because this topic or its requirements are approved.

## Design progression

```text
requirements.md
   ├─> ux/cli-health-recovery.md ─────┐
   └─> ux/menubar-health-recovery.md ─┼─> architecture.md
                                      ├─> final UX reconciliation
                                      └─> tasks.md decomposition/review
```

- `requirements.md` owns scope, safety, action taxonomy, surfaces, exclusions
  and acceptance scenarios. Passed independent Review Round 1 for its exact
  document blob; CEv1 document gate is VERIFIED; delivered by signed commit
  `7537f52`.
- `ux/cli-health-recovery.md` owns terminal text/JSON states, command semantics
  and the operator sequence for both lock and extension recovery. Passed
  Re-review Round 2 after five findings were closed; CEv1 document gate is
  VERIFIED; delivered by signed commit `bc6df88`.
- `ux/menubar-health-recovery.md` owns the native health-row hierarchy, labels,
  copied commands/manual prerequisites, bilingual copy and accessibility.
  Passed Re-review Round 3 after closing MBHR-R1-F1; CEv1 document gate is
  VERIFIED; delivered by signed commit `29694de` together with shared
  prototype specimens.
- `architecture.md` owns typed reason/action contracts, lock inspection and
  doctor read-only boundaries, extension inventory synchronization, ordering,
  compatibility and test seams. Passed Re-review Round 4 after closing all
  findings (ARCH-R1-F1..F11, ARCH-R2-F1..F2, ARCH-R3-F1..F2); CEv1 document
  gate is VERIFIED; delivered by signed commit `071fceb`.
- This file receives implementation tasks now that the documents above have
  passed their applicable reviews.

### Final UX reconciliation

The requested fields and behaviors from `ux/cli-health-recovery.md` and
`ux/menubar-health-recovery.md` are reconciled against `architecture.md` as
follows:

| Requested UX field / behavior | Owning producer in architecture | Disposition / reconciliation |
| --- | --- | --- |
| CLI `error.details.resource` | `internal/store` (`ErrLockContention`) | Sourced from `AcquireLock` (`state`), `AcquireScanLock` (`scan`), or fallback (`unknown`). |
| CLI `error.details.reason` | `internal/store` (`ClassifyLock`) | Evaluated at timeout: `lock_live`, `lock_legacy`, `lock_owner_unknown`. |
| CLI `error.details.action_kind` | `internal/store` | Mapped from reason: `retry`, `manual_prerequisite`, or `null`. |
| CLI `error.details.recovery_command` | `internal/store` | Strictly `null` for lock contention; no unsafe deletion command is provisioned. |
| CLI `error.details.inventory_committed` | `internal/extension` | Sourced from `ErrExtensionSyncIncomplete` when fingerprint write fails post-commit. |
| Doctor `resource`, `reason`, `action_kind` | `internal/doctor` | Added to `doctor.Check`; populated from `ClassifyLock` and 9-reason extension classification. |
| Doctor `recovery_command` | `internal/doctor` | `agentdeck extension scan` for stale inventory (and state missing if discovery succeeded); otherwise `null`. |
| Doctor `diagnostic_command` | `internal/doctor` | `agentdeck extension doctor` on aggregate check when `recovery_command` is `null`. |
| Doctor `manual_prerequisite` | `internal/doctor` | Populated exclusively from the closed set of 7 keys. |
| Wire `resource`, `reason`, `action_kind`, etc. | `internal/desktop` | Additive fields on `HealthCheck`; reuses existing `Recovery` and `Count` fields. |
| Swift wire model & fail-closed parsing | `DesktopWire.swift` | Additive Decodable fields on `DesktopHealthCheckV1`; unknown tokens fail closed to generic warning with nil action. |
| Menu-bar action button affordances | `AgentDeckApp` | `synchronize_inventory` -> "Copy sync command"; `diagnose` -> "Copy diagnostic command"; `manual_prerequisite` -> "Copy safety steps"; `retry` or null -> no button. |
| Menu-bar copy feedback | `AgentDeckApp` | 1.6s polite live region announcement "Copied" / "已复制"; focus does not move; no local execution or background mutation. |
| Menu-bar Effect (mutation scope) | `AgentDeckApp` localized derivation | Localized client-side derivation per `reason`: `extension_stale_inventory` exposes bounded effect ("Updates only AgentDeck's derived extension inventory and scan fingerprint; client configuration and installed extensions stay unchanged" / "只更新 AgentDeck 的派生扩展库存与扫描指纹；不修改客户端配置或已安装扩展"); `extension_fingerprint_update_failed` exposes partial commit effect ("Client configuration did not change, and inventory was not rolled back" / "客户端配置未改变，库存也没有回滚"). Wire conveys stable `reason` without duplicate text. |
| Menu-bar Partial commit (`inventory_committed`) | `internal/extension` & `AgentDeckApp` | In CLI, `error.details.inventory_committed: true` is emitted by `ErrExtensionSyncIncomplete`. In desktop app, `extension_fingerprint_update_failed` reason is produced exclusively when the inventory transaction committed but the fingerprint write failed; client derives the non-rollback presentation directly from this condition. Wire conveys the condition code; no separate wire boolean is required. |

All requested fields and interactions are fully provisioned without open gaps
or conflicting contracts. Reconciliation is complete and verified.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `lock-recovery-foundation` | [ ] | [ ] |
| 2. `extension-inventory-recovery` | [ ] | [ ] |
| 3. `desktop-wire-and-shared-contracts` | [ ] | [ ] |
| 4. `menubar-health-presentation` | [ ] | [ ] |
| 5. `health-recovery-acceptance` | [ ] | [ ] |

Implementation Beads tasks are created only after this document passes review.
Use one work-product task per anchor and the existing document task lifecycle;
do not create separate development/review tasks or revive the planning carrier
as an implementation task.

### Dependency graph

```text
1. lock-recovery-foundation
   └─> 2. extension-inventory-recovery
         └─> 3. desktop-wire-and-shared-contracts
               └─> 4. menubar-health-presentation
                     └─> 5. health-recovery-acceptance
```

Task 1 delivers both the lock classification substrate
(`ad-bug-state-busy-recovery-guidance`) and the shared Go contract pipeline
(`doctor.Check` additive fields, CLI `error.details` projection helper, and
aggregate doctor rendering). Task 2 depends on Task 1 and implements extension
inventory recovery (`ad-bug-extension-stale-mcp-recovery`), plugging its 9-reason
diagnostics and incomplete-sync error into Task 1's shared pipeline. Task 3
depends on Task 2 to serialize the complete health checks into Desktop Wire and
Swift models. Task 4 depends on Task 3 for decoded Swift wire types. Task 5
depends on Tasks 1–4 and owns topic-wide cross-surface acceptance, specification
reconciliation, and native residual-risk reporting.

### 1. `lock-recovery-foundation`

Deliver the typed lock contention error (`ErrLockContention`) wrapping
`ErrStateBusy`, the deterministic lock classifier (`ClassifyLock`), timeout
reclassification, doctor read-only lock checks for both `state.lock` and
`scan.lock`, CLI error details, and the shared doctor/CLI contract pipeline.

**Files and ownership**

- `internal/store/store.go`, `internal/store/process_darwin.go`, and
  `internal/store/store_test.go` for `ErrLockContention`, `ClassifyLock`,
  timeout classification, and `lockProcessCheck` test seams.
- `internal/doctor/doctor.go` and `internal/doctor/doctor_test.go` for the
  shared `Check` struct additive fields (`Resource`, `Reason`, `ActionKind`,
  `DiagnosticCommand`, `ManualPrerequisite`), the read-only lock check function
  (`checkLock`), retiring legacy codes (`stale_lock`, `state_busy`,
  `lock_unreadable`), and inserting `scan_lock` immediately after `state_lock`.
- `cmd/agentdeck/main.go` and `cmd/agentdeck/main_test.go` for the CLI
  `error.details` projection helper, CLI error presentation of
  `ErrLockContention`, aggregate doctor text/JSON rendering of `next:`,
  `diagnose:`, `recovery:`, `effect:`, and `manual prerequisite:` lines, and
  `schema_ahead` precedence.
- `cmd/agentdeck/quota.go` is touched only to document/verify that
  `runDesktopQuotaRefresh` continues to return plain `ErrStateBusy`, yielding
  `resource: unknown`.

**Required result**

- Define `ErrLockContention` in `internal/store`, carrying `Resource` (`string`),
  `Reason` (`string`), `ActionKind` (`*string`), and `RecoveryCommand`
  (`*string`, always `nil`).
- `ErrLockContention` implements `Unwrap() error { return ErrStateBusy }`,
  preserving `errors.Is(err, store.ErrStateBusy)` and exit code 1.
- Only `AcquireLock` (`state.lock` -> `resource: state`) and `AcquireScanLock`
  (`scan.lock` -> `resource: scan`) construct and return `ErrLockContention`.
  `AcquireQuotaRefreshLock` and `AcquireDerivedSnapshotCacheLock` keep returning
  plain `ErrStateBusy`.
- Export `ClassifyLock(path string, check lockProcessCheck) (LockReason, error)` on
  `internal/store`:
  1. If lock file does not exist, return not contended (callers stop before
     invoking classifier).
  2. If file exists but stat/open fails, classify as `lock_owner_unknown`.
  3. Content-shape test: read file, invoke `lockOwnerPID`. If `ok: false`
     (corrupted, non-`v1` token, or pre-`v1`), classify as `lock_legacy`.
  4. Liveness test: invoke `lockProcessAlive(pid)`:
     - `alive: true, known: true` -> `lock_live`.
     - `alive: false, known: true` -> `lock_reclaimable`.
     - `known: false` -> `lock_owner_unknown`.
  Age is never read or used in classification. PID reuse is bounded because
  `lock_live` only ever yields `action_kind: retry`.
- Timeout classification: on acquisition timeout in `AcquireLock` and
  `AcquireScanLock`, run `ClassifyLock` one more time to fill `Reason` and
  `ActionKind` (`lock_live` -> `retry`, `lock_legacy` -> `manual_prerequisite`,
  `lock_owner_unknown` -> `null`). If stat fails during timeout classification,
  fall back to `lock_owner_unknown` and `action_kind: null`.
- Shared `doctor.Check` fields: add additive fields `Resource` (`string`),
  `Reason` (`string`), `ActionKind` (`string`), `DiagnosticCommand` (`string`),
  and `ManualPrerequisite` (`string`) to `internal/doctor/doctor.go`, reusing
  existing `Recovery` (`recovery_command`) and `Count` (`count`).
- CLI `error.details` projection helper in `cmd/agentdeck/main.go`: projects
  `resource`, `reason`, `action_kind`, `recovery_command`, and
  `inventory_committed` into text and JSON `error.details`. Plain `ErrStateBusy`
  yields `resource: unknown` with `reason` and `action_kind` omitted.
- Aggregate doctor rendering in `cmd/agentdeck/main.go`: update doctor text and
  JSON rendering to support `next:`, `diagnose:`, `recovery:`, `effect:`, and
  `manual prerequisite:` sections.
- Precedence: `schema_ahead` remains highest priority command error over
  `ErrLockContention`.
- Doctor read-only lock check: `checkLock` independently classifies both
  `state.lock` and `scan.lock` with no side effects (no flock, no write, no
  delete). Retired codes (`stale_lock`, `state_busy`, `lock_unreadable`) are
  replaced one-to-one by classifier outcomes. `lock_reclaimable` is reported
  as `ok` with code `lock_reclaimable` and not counted toward problems or
  warnings.
- Doctor row ordering: `state_lock` is row 3, `scan_lock` is row 4, preserving
  the complete 19-row doctor sequence.
- Automatic reclaim: inline `reclaimLockFromDeadProcess` remains unchanged.

**Verification — L3**

- Run affected packages via wrapper:
  `scripts/run-go-test.sh ./internal/store/... ./internal/doctor/... ./cmd/agentdeck/...`
- Run race detection on concurrency packages:
  `scripts/run-go-test.sh -race ./internal/store/... ./internal/doctor/...`
- Run full repository suite for Go core contract changes:
  `scripts/run-go-test.sh ./...`
- Assertions cover:
  - Injected `lockProcessCheck` for all four classifier outcomes.
  - Non-`v1` token fixture asserting deterministic `lock_legacy` regardless of age.
  - Stat failure fixture asserting `lock_owner_unknown`.
  - Timeout race simulation asserting `ErrLockContention` fills `Reason` and
    `ActionKind` at timeout, and falls back to `lock_owner_unknown` on stat race.
  - Doctor tests asserting `state_lock` and `scan_lock` reported independently
    in read-only mode under contention.
  - CLI command error tests asserting exit code 1, text/JSON details for `state`
    and `scan`, and `resource: unknown` for quota refresh.
  - Precedence test proving `schema_ahead` wins over `ErrLockContention`.

**Excluded:** extension inventory, desktop wire, Swift macOS app, changing
inline lock reclaim protocol.

### 2. `extension-inventory-recovery`

Deliver extension discovery resilience, typed classification of all nine
extension conditions, the read-only extension doctor entry, the
`extension.sync_incomplete` marker, and the aggregate doctor extension check.
Depends on Task 1.

**Files and ownership**

- `internal/extension/extension.go` and `internal/extension/extension_test.go`
  for `ExtensionDiscoverer` interface, per-item fingerprint failure resilience,
  read-only database access, 9-condition classification, and
  `ErrExtensionSyncIncomplete`.
- `internal/doctor/doctor.go` and `internal/doctor/doctor_test.go` only for the
  aggregate doctor `extensions` row using primary reason and mapped action/command,
  retiring `extension_diagnostics` and fixed doctor recovery.
- `cmd/agentdeck/main.go` and `cmd/agentdeck/main_test.go` for `extension doctor`
  (text/JSON), `extension scan` error handling, and wiring `extensions` check
  into Task 1's aggregate doctor.

**Required result**

- Introduce `ExtensionDiscoverer` interface wrapping `discover()` to allow
  injecting synthetic discovery results and scanner failures in tests.
- Per-item fingerprint failure resilience: a failure in `fingerprint(sourcePath)`
  (`source unavailable` or `source unreadable`) does not abort the entire
  discovery pass. `discover()` emits the candidate with its canonical identity,
  sets `Fingerprint = ""`, and records `source_unavailable` or
  `source_unreadable` in per-item `Diagnostics`.
- Read-only entry: `extension.Doctor` opens database via `store.OpenReadOnly()`.
  Outputs `extension_state_missing` if state root/db absent, and
  `extension_inventory_unreadable` if corrupted.
- Comparisons require successful discovery: when `discover()` returns an error,
  `discovery_status: "failed"`, collections (`stale_inventory`, `missing_paths`,
  `native_unavailable`, `duplicate_ids`, `drifted_ids`, `management_anomalies`)
  are JSON `null` (not `[]`), and no stored ID is reported as stale.
- Stale inventory and native unavailability are mutually exclusive: absent from
  discovery = stale; rediscovered with per-item diagnostic = native-unavailable.
- Non-fatal discovery diagnostics (invalid canonical IDs) are informational,
  sanitized, and not counted toward aggregate doctor problems (retiring
  `len(extensionReport.Diagnostics)` count term).
- 9-condition priority order for aggregate doctor:
  1. `extension_state_missing`
  2. `extension_inventory_unreadable`
  3. `extension_discovery_failed`
  4. `extension_duplicate_id`
  5. `extension_management_anomaly`
  6. `extension_managed_drift`
  7. `extension_stale_inventory`
  8. `extension_native_unavailable`
  9. `extension_fingerprint_update_failed`
- Reason -> action mapping:
  - `extension_stale_inventory` -> `action_kind: synchronize_inventory`,
    `recovery_command: agentdeck extension scan`.
  - `extension_state_missing` -> `action_kind: synchronize_inventory` only when
    discovery succeeded; otherwise `manual_prerequisite`.
  - Other seven reasons map to `manual_prerequisite` or `diagnose` per reviewed
    mapping table.
- Aggregate doctor: replaces fixed `Code: "extension_diagnostics"` and
  `Recovery: "agentdeck extension doctor"` with the primary reason and its row
  from the mapping table. Sets `diagnostic_command: agentdeck extension doctor`
  only when `recovery_command` is `null`.
- `agentdeck extension scan`: on fingerprint write failure after inventory commit,
  writes setting `extension.sync_incomplete = "true"` (best-effort) and returns
  `ErrExtensionSyncIncomplete` (exit code 1, `inventory_committed: true`). Next
  successful scan clears setting (`""`). Read-only doctor surfaces
  `extension_fingerprint_update_failed` exclusively from this marker.
- Terminal single-line sanitizer normalizes and sanitizes all external discovery
  diagnostics before text rendering.
- Compatibility: `DoctorReport` preserves `missing_paths` with identical contents
  to `stale_inventory`.

**Verification — L3**

- Run affected packages via wrapper:
  `scripts/run-go-test.sh ./internal/extension/... ./internal/doctor/... ./cmd/agentdeck/...`
- Run race detection on concurrency packages:
  `scripts/run-go-test.sh -race ./internal/extension/...`
- Run full repository suite for Go core contract changes:
  `scripts/run-go-test.sh ./...`
- Assertions cover:
  - Synthetic discovery via `ExtensionDiscoverer` interface.
  - Stale MCP identity fixture asserting `extension_stale_inventory`,
    `action_kind: synchronize_inventory`, `recovery_command: agentdeck extension scan`,
    and absence of `diagnostic_command` on aggregate check.
  - Unresolvable native path fixture asserting candidate is marked
    `source_unavailable`, classified as `extension_native_unavailable`, and kept
    by scan without drift warning.
  - Scanner-level failure fixture asserting `discovery_status: "failed"`, null
    collections, and no stale reported.
  - Invalid canonical ID candidate asserting informational section, `discovery_status: "ok"`,
    no reason token, and no problem count.
  - Incomplete sync fixture asserting failed fingerprint write returns
    `ErrExtensionSyncIncomplete`, writes marker, doctor reports
    `extension_fingerprint_update_failed` with `action_kind: diagnose`, and
    subsequent successful scan clears marker.
  - Aggregate doctor tests asserting complete 9-condition priority ordering.
  - CLI `extension doctor` (text and JSON) and `extension scan` tests.

**Excluded:** lock acquisition/classification, wire serialization, desktop Swift UI.

### 3. `desktop-wire-and-shared-contracts`

Deliver additive `HealthCheck` fields in Go desktop wire format, update schema v1
fixtures, and provide fail-closed Swift Decodable models in `DesktopWire.swift`.
Depends on Task 2.

**Files and ownership**

- `internal/desktop/desktop.go` and `internal/desktop/desktop_test.go` for
  wire struct additive fields, mapping from doctor checks, and serialization.
- `desktop/fixtures/v1` fixture files for schema v1 wire test baseline.
- `apps/macos/AgentDeckShared/DesktopWire.swift` and
  `apps/macos/AgentDeckTests/DesktopWireTests.swift` for Swift wire types,
  decoding, and fail-closed validation.

**Required result**

- Add additive fields to `HealthCheck` in `internal/desktop/desktop.go`:
  - `Resource *string `json:"resource,omitempty"``
  - `Reason *string `json:"reason,omitempty"``
  - `ActionKind *string `json:"action_kind,omitempty"``
  - `DiagnosticCommand *string `json:"diagnostic_command,omitempty"``
  - `ManualPrerequisite *string `json:"manual_prerequisite,omitempty"``
  - Preserves existing `Recovery string `json:"recovery_command,omitempty"``
    and `Count int `json:"count,omitempty"``.
- `HealthCheck` serialization populates additive fields from doctor checks for
  both lock checks and extension check.
- Update `desktop/fixtures/v1` fixtures with additive fields, proving backward
  compatibility: missing additive fields safely decode as nil.
- Update `apps/macos/AgentDeckShared/DesktopWire.swift` with Decodable additive
  fields on `DesktopHealthCheckV1`:
  `resource: String?`, `reason: String?`, `actionKind: HealthActionKind?`,
  `diagnosticCommand: String?`, `manualPrerequisite: String?`.
  Preserves existing `recoveryCommand: String?` and `count: Int?`.
- Fail-closed rules in Swift decoding:
  - If `action_kind`, `reason`, or `resource` contains an unrecognized token,
    fail closed: decode check as generic warning, treat `actionKind` as `nil`,
    and do not expose `recoveryCommand`.
- Verify closed set of 7 `manual_prerequisite` keys:
  `prereq_legacy_lock_removal`, `prereq_extension_discovery_failed`,
  `prereq_extension_inventory_unreadable`, `prereq_extension_duplicate_id`,
  `prereq_extension_managed_drift`, `prereq_extension_management_anomaly`,
  `prereq_extension_native_unavailable`.

**Verification — L2**

- Run affected package tests via wrapper:
  `scripts/run-go-test.sh ./internal/desktop/...`
- Run full repository suite for wire contract changes:
  `scripts/run-go-test.sh ./...`
- Wire fixture validation asserting additive fields serialize correctly and
  older fixtures decode without error.
- Swift XCTest in `DesktopWireTests.swift` via `scripts/test-macos-app.sh`:
  - Decoding lock checks (`lock_live`, `lock_legacy`, `lock_owner_unknown`,
    `lock_reclaimable`).
  - Decoding extension checks (all nine reasons).
  - Unknown token decoding asserting fail-closed fallback (`actionKind: nil`,
    `recoveryCommand: nil`).
  - Privacy assertions: verify no private paths, nonces, or tokens are serialized.

**Excluded:** SwiftUI views, String Catalog translations, Go lock/extension logic.

### 4. `menubar-health-presentation`

Deliver the native macOS menu-bar health row hierarchy, severity, localized
labels, copyable command button with 1.6-second transient feedback, manual
prerequisite resolution, bilingual localization (en/zh-Hans), and accessibility.
Depends on Task 3.

**Files and ownership**

- `apps/macos/AgentDeckApp/MenuBarViewModel.swift` for health check presentation
  and copy action state machine.
- `apps/macos/AgentDeckApp/MenuBarSurfaceView.swift` for rendering severity, cause
  summary, action button, and feedback.
- `apps/macos/AgentDeckApp/DesktopCopy.swift` for health-related copy constants and
  action labels.
- `apps/macos/AgentDeckApp/Localizable.xcstrings` for bilingual English and
  Chinese strings.
- `apps/macos/AgentDeckAppTests/MenuBarViewModelTests.swift` for view model and
  action contract assertions.
- `apps/macos/AgentDeckAppTests/DesktopCopyTests.swift` for copy and localization
  assertions.
- Reuses reviewed prototype specimens in `prototype/src/healthRecovery.js` as
  specimen authority.

**Required result**

- Health row UI in menu bar popover:
  - Severity badge/icon (`warning`, `failed`).
  - Localized cause summary and reason label.
  - Count badge when `count` > 0.
- Action presentation strictly adheres to the reviewed Action contract:
  - The menu-bar app never executes these commands; it only copies classified
    content to the clipboard.
  - `synchronize_inventory`: displays button "Copy sync command" / "复制同步命令",
    copying exact `recovery_command` (`agentdeck extension scan`).
  - `diagnose`: displays button "Copy diagnostic command" / "复制诊断命令",
    copying exact diagnostic command (`agentdeck doctor` or `agentdeck extension doctor`).
    Never labeled Fix, Repair, or Recovery.
  - `manual_prerequisite`: displays button "Copy safety steps" / "复制安全步骤",
    copying localized prose from the 7 closed keys. Must not contain a copyable
    destructive command. Unknown key fails closed (no button).
  - `retry` or null: no action button is rendered.
- Clipboard copy feedback:
  - Clicking the copy button exposes `Copied` / `已复制` through one polite live
    region for 1.6 seconds.
  - Focus does not move after clicking Copy.
  - Health row, notice, and health status do not change on copy.
- Desktop refresh semantics:
  - Copying an action mutates no health state; the health row is removed only
    when a subsequent refresh snapshot proves the condition has cleared.
- Localization:
  - Complete English and Chinese (zh-Hans) strings in `Localizable.xcstrings` and
    constants in `DesktopCopy.swift` for all seven manual prerequisite keys, the
    three action button labels, "Copied" feedback, health reasons, and the effect /
    partial commit (non-rollback) descriptions.
- Effect and partial commit presentation:
  - For `extension_stale_inventory`, renders bounded effect text explaining
    mutation scope ("Updates only AgentDeck's derived extension inventory and scan
    fingerprint; client configuration and installed extensions stay unchanged" /
    "只更新 AgentDeck 的派生扩展库存与扫描指纹；不修改客户端配置或已安装扩展").
  - For `extension_fingerprint_update_failed`, renders partial commit / non-rollback
    explanation ("Client configuration did not change, and inventory was not
    rolled back" / "客户端配置未改变，库存也没有回滚"), satisfying the requirement
    that partial commit is never presented as a rollback.
- Accessibility & Layout:
  - Keyboard navigation and VoiceOver accessible labels and traits.
  - At 420 pt, command and button share a row; at 280 pt, they stack, the button
    fills the row, and commands wrap at safe boundaries without truncation.

**Verification — L2 with native-App risk checks**

- Run full macOS app test suite in isolated HOME:
  `scripts/test-macos-app.sh`
- Unit tests in `MenuBarViewModelTests.swift` asserting correct button, action,
  prerequisite mapping, effect text for stale inventory, and partial commit
  (non-rollback) text for incomplete sync.
- Unit tests in `DesktopCopyTests.swift` asserting String Catalog and
  `DesktopCopy.swift` completeness for all keys, effect strings, and
  non-rollback text in both `en` and `zh-Hans`.
- UI snapshot / layout tests at 280 pt and 420 pt widths.
- Focus retention and 1.6-second timer tests.

**Excluded:** Go backend logic, Widget target changes, background polling changes.

### 5. `health-recovery-acceptance`

Deliver cross-surface automated integration acceptance across all 16 architecture
scenarios and 12 requirement scenarios, CLI design specification reconciliation
in `docs/specs/cli-design.md`, and residual native risk documentation.
Depends on Tasks 1–4.

**Files and ownership**

- `cmd/agentdeck/acceptance_test.go` (new integration test file) for end-to-end
  CLI integration tests.
- `docs/specs/cli-design.md` for stable CLI contract reconciliation.
- Topic status updates in `tasks.md`.

**Required result**

- Full automated regression across all 16 acceptance rows in `architecture.md`
  (and 12 requirement scenarios in `requirements.md`):
  1. Live `state.lock` owner: `state_busy` identifies core state, recommends retry,
     no deletion.
  2. Live `scan.lock` owner: identifies scan contention separately.
  3. Legacy lock token: `lock_legacy`, `manual_prerequisite: prereq_legacy_lock_removal`,
     no destructive command.
  4. Unknown lock ownership: `lock_owner_unknown`, `action_kind: null`.
  5. Safely reclaimable modern stale lock: normal acquisition reclaims; read-only
     doctor reports `lock_reclaimable` as `ok`.
  6. Doctor under contention: reports both lock classes read-only without mutating.
  7. Future schema plus contention: `schema_ahead` wins over `ErrLockContention`.
  8. Stale persisted MCP identity: `extension_stale_inventory`,
     `action_kind: synchronize_inventory`, `recovery_command: agentdeck extension scan`.
  9. Native path unavailable: candidate marked `source_unavailable`, reported as
     `extension_native_unavailable`, scan keeps row.
  10. Extension synchronization: removes stale row, records current identity,
      warning clears.
  11. Discovery failure: inventory preserved, collections null, no stale reported.
  12. Invalid canonical ID: skipped as informational, no reason, no problem count.
  13. Other extension conditions: duplicate, drift, anomaly reported with priority order.
  14. Incomplete sync marker: failed fingerprint write sets marker, doctor reports
      `extension_fingerprint_update_failed`, subsequent scan clears it.
  15. Structured output and desktop wire: additive fields survive text, JSON, and
      wire encoding.
  16. macOS recovery presentation: label distinguishes action copies, no execution,
      Retry has no button, focus and bilingual copy verified.
- Reconcile `docs/specs/cli-design.md` with final delivered CLI contracts for
  health recovery.
- Populated Task 5 Acceptance Evidence table: record PERFORMED / PASS, SIMULATED,
  or BLOCKED for each native and automated boundary.
- Document residual risks and native limitations.

**Verification — L3 integration / native acceptance**

- Run full Go test suite via wrapper:
  `scripts/run-go-test.sh ./...`
- Run full Go race suite via wrapper:
  `scripts/run-go-test.sh -race ./...`
- Run full macOS app test suite in isolated HOME:
  `scripts/test-macos-app.sh`
- Audit topic documentation:
  `bash scripts/check-topic-docs.sh`
- Audit code formatting and whitespace:
  `make check-whitespace` and `git diff --check`

**Excluded:** release, tag, push, deployment, out-of-scope refactorings.

## Current handoff

The requirements boundary passed Review Round 1 and its signed delivered
document gate is VERIFIED (`7537f52`). CLI framework Re-review Round 2 passed
for content state
`urn:ce:agent-deck:content-state:health-recovery:ux-cli-health-recovery:7537f52:1d95ea77b6fff6ac58ad65424f1b546a4d9f262e7bd70a7e53978fc49a2fa7de`;
all five Round 1 findings are closed and the document gate is VERIFIED;
delivered by signed commit `bc6df88`. `ux/menubar-health-recovery.md` framework
Re-review Round 3 passed for content state
`urn:ce:agent-deck:content-state:health-recovery:ux-menubar-health-recovery:bc6df88:6a83f9435c75f17b4ebfb6769855e0be0a06099a8b51d73f222f709bad08a9c0`
and closed MBHR-R1-F1; document gate is VERIFIED; delivered by signed commit
`29694de` together with its shared prototype changes. `architecture.md`
Re-review Round 4 passed for content state
`urn:ce:agent-deck:content-state:health-recovery:architecture:29694de:87b6094fbb40f55784ebd5a2f9deb2872613e2e9a22096a35afed7a43b9ceef7`;
every finding recorded in [`reviews/architecture.md`](reviews/architecture.md)
is closed, document gate is VERIFIED, and it was delivered by signed commit
`071fceb` on `feature/health-recovery`.

Final UX reconciliation confirms that `architecture.md` provisions all fields
requested by both UX surfaces without data gaps, unmapped fields, or contradictory
contracts. Round 1 and Round 2 review findings (`TASKS-R1-F1` through `TASKS-R1-F6`)
have been fully repaired: Task 4 action contract is aligned with menu-bar UX;
Task 1 owns the shared doctor/CLI contract pipeline, serializing Task 2's
dependency; file paths are corrected to real repository locations; Task 3 wire
fields reuse existing fields; verification commands follow the project L0–L4
wrapper matrix; and the final UX reconciliation explicitly records Effect and
Partial commit derivations, with matching copy and test requirements added to
Task 4. `tasks.md` decomposition Re-review Round 3 passed with every finding
recorded in [`reviews/tasks.md`](reviews/tasks.md) closed; its document gate is
VERIFIED and it awaits separately authorized Git delivery. The five
implementation Beads tasks and their Development authorization Gates may now be
created.

Implementation remains blocked on its declared dependencies and implementation
gate approval. Git delivery, integration, retirement and release remain separate
boundaries.

## Review boundary

`tasks.md` review decides whether these five anchors cover every approved
requirement, surface contract, architecture invariant, dependency, file owner
and verification level without overlapping responsibility or adding scope. Run
`bash scripts/check-topic-docs.sh`; a PASS allows creation of exactly these
five implementation Beads tasks and their Development authorization Gates. It
does not authorize implementation, commit, push, release, installation or
native waivers.
