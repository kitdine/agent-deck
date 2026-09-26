---
status: active
created: 2026-09-21
updated: 2026-09-21
---

# Health Recovery — Requirements

## Purpose and origin

Make AgentDeck health and lock failures lead to a truthful, cause-specific,
safe recovery path instead of repeating a diagnostic command that cannot change
the reported condition.

This topic is the `ad-health-recovery` planning entry selected for v0.6.0 in
the [roadmap](../../roadmap.md). It retains two independent defect origins:

- `ad-bug-state-busy-recovery-guidance`: `state_busy` says only that a lock
  timed out. It does not identify `state.lock` versus `scan.lock`, distinguish a
  live owner from a recoverable stale condition, or direct the user through a
  safe diagnosis and recovery sequence.
- `ad-bug-extension-stale-mcp-recovery`: an extension identity migration left
  the persisted `codex:mcp:user:computer-use` inventory row after the live
  capability moved to `unified-computer-use@openai-bundled` / `cua_repl`.
  Health reported a persistent warning while offering the read-only
  `agentdeck extension doctor` command as though it could repair the inventory.

The causes, data and acceptance evidence stay separate. They share one topic
because both violate the same product promise: when AgentDeck presents a health
problem and an action, that action must be safe, executable, and capable of
changing the diagnosed condition. One generic rescan or recovery message must
not hide the cause-specific contracts below.

## Supported premises

Current code establishes these premises for later design and review:

- `state.lock` serializes core state mutations. `scan.lock` separately owns the
  detached scan worker and maintenance ordering. Both currently surface the
  same `state_busy` error family even though their operators and recovery
  conditions differ.
- Modern lock tokens include owner identity and support fail-closed liveness
  checks. Unknown ownership, legacy tokens, PID reuse uncertainty and platform
  limitations are not proof that a lock is safe to delete.
- `agentdeck doctor` is contractually read-only. Its lock check currently
  inspects `state.lock`, uses age as the stale signal, and does not inspect
  `scan.lock`.
- `agentdeck extension doctor` discovers current native extensions and compares
  them with persisted inventory without replacing that inventory. A stored ID
  absent from live discovery enters `missing_paths`.
- `agentdeck extension scan` is the existing explicit inventory synchronization
  operation. It discovers native state first, then atomically replaces the
  AgentDeck-owned extension inventory; it does not modify Codex or Claude
  configuration or install, update, enable, disable or remove native content.
- The aggregate doctor maps every extension diagnostic to one
  `extension_diagnostics` warning and currently publishes
  `agentdeck extension doctor` as its `recovery_command`.
- The macOS health surface consumes safe doctor check fields, including
  `recovery_command`, and offers a copyable action. It does not own native
  extension management or lock deletion.

These are current-code observations, not blanket requirements to retain an
unsafe or misleading presentation.

## Goals

1. Preserve `state_busy` as the stable error family while identifying whether
   contention belongs to core state or scanning at every user-visible boundary
   that can know the owner.
2. Give users a bounded diagnosis and recovery sequence that distinguishes a
   live or uncertain owner, an automatically reclaimed modern stale lock, and a
   legacy condition requiring manual confirmation.
3. Keep all doctor modes read-only and available as diagnostics without making
   the user repeat an operation that is guaranteed to return the same warning.
4. Make extension inventory drift distinguish persisted stale identity from a
   genuinely unavailable native path or a discovery failure.
5. Offer an explicit extension inventory synchronization action only when live
   discovery succeeded and synchronization can address the reported stale
   inventory; use the existing `extension scan` ownership rather than a second
   mutation path.
6. Keep CLI text, structured output, desktop wire data and macOS action labels
   consistent about whether an action diagnoses, copies, retries or mutates
   AgentDeck-owned state.
7. Prove recovery from both origin defects without modifying client
   configuration, managing plugin installation, deleting live locks or exposing
   private configuration contents.

## Recovery contract

### Diagnostic and recovery actions

Every surfaced action has one truthful class:

- **diagnose** reads and classifies current state but does not repair it;
- **retry** repeats the failed operation only after a condition may have changed;
- **synchronize inventory** updates only AgentDeck's derived extension inventory
  from a successful live discovery; or
- **manual prerequisite** explains the external confirmation required before a
  destructive filesystem action that AgentDeck will not perform automatically.

`recovery_command` is present only when running that exact command can safely
advance recovery. Prose that says to upgrade, wait, inspect a process or confirm
ownership is not encoded as an executable recovery command. UI copy must not
label a diagnostic command as Fix, Repair or an equivalent mutating promise.

### Lock contention and liveness

The product distinguishes at least `state.lock` and `scan.lock`. A
user-visible `state_busy` response identifies the lock class in text and in a
stable machine-readable form without exposing the private lock token or raw
internal errors.

Recovery follows these rules:

- A verified live owner tells the user to let the named class of work finish or
  stop that owning process through its normal lifecycle, then retry. AgentDeck
  does not kill it or recommend deleting its lock.
- Unknown liveness, a legacy token, PID reuse uncertainty, an unreadable lock,
  or an unsupported platform fails closed. Age alone never turns such a lock
  into an executable removal command.
- A modern stale lock that the existing lock protocol can safely reclaim stays
  an automatic internal recovery. The user is not told to delete a file the
  next acquisition can safely reclaim itself.
- A legacy lock may require a documented manual prerequisite: first establish
  that no relevant AgentDeck process is alive, then remove only the identified
  legacy lock. Architecture and UX must state how that confirmation is made and
  must not offer a one-click or copyable `rm` command without it.
- Diagnostics report `state.lock` and `scan.lock` independently. A clear state
  lock does not imply the scan owner is absent, and a scan owner does not make
  ordinary read-only health checks a state-lock failure.

The design may add structured detail to `state_busy`, but it must retain the
stable error code and exit status. Unsupported or inconclusive schema probing
continues to preserve the original lock error; a definite future schema retains
the existing `schema_ahead` precedence.

### Extension inventory recovery

Extension health separates these conditions:

- live discovery failed, so persisted-versus-live conclusions are unavailable;
- a persisted identity is absent from successful live discovery and may be
  stale inventory;
- a managed extension drifted from its adopted fingerprint;
- duplicate live identities or management metadata are inconsistent; and
- a native path or capability is genuinely unavailable under the applicable
  adapter contract.

The stale-identity origin is recoverable by explicit inventory synchronization.
That operation:

- uses the same discovery and replacement implementation as
  `agentdeck extension scan`;
- replaces AgentDeck's derived inventory only after discovery succeeds;
- removes inventory rows no longer present and admits current identities;
- never rewrites Codex or Claude configuration, plugin manifests or native
  extension files; and
- returns a result that lets the user verify the warning cleared or understand
  a remaining, differently classified condition.

`agentdeck extension doctor` remains read-only. It may recommend
`agentdeck extension scan` only for conditions that synchronization can address.
Discovery failure, managed drift and other conditions retain their own truthful
diagnosis or prerequisite instead of receiving a blanket scan command.

## Affected surfaces and document set

This topic requires:

- [`ux/cli-health-recovery.md`](ux/cli-health-recovery.md) — `state_busy`,
  doctor and extension diagnostic/recovery text and structured-output states,
  including command naming and operator sequence;
- [`ux/menubar-health-recovery.md`](ux/menubar-health-recovery.md) — health-row
  severity, cause summary, action label, copied command/prerequisite, recovery
  confirmation and accessibility states in English and Chinese;
- [`architecture.md`](architecture.md) — typed lock identity/liveness,
  read-only doctor access, extension diagnostic classification, inventory-sync
  ownership, CLI/desktop wire compatibility and failure ordering; and
- [`tasks.md`](tasks.md) — the reviewed implementation decomposition after the
  documents above settle their contracts.

The CLI and menu-bar surfaces may share stable reason/action codes, but their
interaction designs remain separately reviewable. The architecture must map
each requested surface field to an owning producer or explicitly refuse it.

## Security, privacy and compatibility

- Never print lock nonces, credential material, native configuration contents,
  environment secrets or raw SQLite/driver errors.
- Process identity or paths appear only to the minimum degree needed for safe
  local recovery and follow existing redaction rules.
- Doctor, including full doctor, never deletes, migrates, chmods, synchronizes
  inventory or changes native client state.
- Inventory synchronization affects only AgentDeck's derived core-database rows
  and uses existing state/scan ordering; partial discovery never truncates a
  previously valid inventory.
- Existing `state_busy` and `extension_diagnostics` consumers remain compatible.
  Additive structured detail must have safe behavior for older desktop clients
  and missing fields.

## Non-goals

- Automatically deleting any lock, killing a process or weakening fail-closed
  lock acquisition.
- Adding a force-unlock command, daemon, LaunchAgent, privileged helper or
  remote health service.
- Making doctor, extension doctor or the macOS health view mutate state merely
  by opening or refreshing.
- Installing, updating, enabling, disabling, adopting, releasing or removing
  Codex/Claude plugins, MCP servers or skills as a recovery shortcut.
- Treating every missing extension identity as a migration or automatically
  mapping unrelated old and new IDs.
- Redesigning unrelated provider, credential, schema, session, quota or Widget
  health states.
- Delivering full extension observability, which remains outside v0.6.0.

## Acceptance boundary

| Scenario | Required result |
| --- | --- |
| Live `state.lock` owner | `state_busy` identifies core-state contention, recommends wait/normal owner shutdown and retry, and never suggests deletion. |
| Live `scan.lock` owner | The response identifies scan contention separately and does not claim that `state.lock` is busy. |
| Unknown or legacy lock | Diagnosis fails closed, names the manual confirmation prerequisite and provides no unconditional destructive command. |
| Safely reclaimable modern stale lock | Normal acquisition reclaims it under the existing token/inode protocol; no manual cleanup is required or advertised. |
| Doctor under contention | Read-only diagnosis reports both lock classes without migrating state, deleting locks or hiding one class behind the other. |
| Future schema plus contention | A definite `schema_ahead` retains precedence; inconclusive probing preserves the correctly classified lock error. |
| Stale persisted MCP identity | Extension diagnosis identifies stale inventory after successful discovery and recommends explicit inventory synchronization. |
| Extension synchronization | The stale ID is removed, the current live identity is recorded, the warning clears, and no native client configuration or extension content changes. |
| Discovery failure | Existing inventory is preserved and no synchronization command is presented as guaranteed recovery. |
| Other extension diagnostics | Duplicate, managed drift and management anomalies keep cause-specific actions; they are not silently collapsed into stale inventory. |
| Structured output and desktop wire | Stable reason/action semantics survive text, JSON and wire encoding; missing additive fields remain safe for older consumers. |
| macOS recovery presentation | The label accurately distinguishes Diagnose, Copy Command, Retry and manual prerequisite; focus, accessibility and bilingual copy remain truthful through failure and recovery. |

Verification must include deterministic lock-owner/liveness fixtures for both
lock names, legacy/unknown/modern-stale cases, state-busy/schema-ahead ordering,
read-only doctor assertions, stale-identity migration fixtures, discovery-failure
preservation, inventory synchronization and CLI/desktop contract tests. Native
macOS acceptance records keyboard, VoiceOver and copied-command behavior as
performed, blocked or explicitly waived.

## Review boundary

Requirements review decides whether the two origin defects remain independently
testable, the action taxonomy is truthful, lock recovery is fail-closed,
extension synchronization has the correct ownership, affected surfaces are
complete, and exclusions prevent unintended client or plugin mutation. It does
not approve final copy, UI layout, Go type shapes, wire fields, lock inspection
algorithms or implementation tasks; those belong to the declared documents and
their later review boundaries.
