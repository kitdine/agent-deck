---
status: active
created: 2026-09-21
updated: 2026-09-21
---

# Health Recovery — CLI Experience

## Goal, scope and non-goals

Give a terminal user enough truthful information to understand which AgentDeck
resource is unhealthy, what action is safe now, and whether that action only
diagnoses or can actually advance recovery.

This is a line-oriented CLI contract for the two independently testable tracks
in [`requirements.md`](../requirements.md):

- core-state and scan lock contention; and
- stale extension inventory after a native extension identity changes.

The affected entry points are ordinary command errors, `agentdeck doctor`,
`agentdeck doctor --full`, `agentdeck extension doctor`, and the existing
`agentdeck extension scan` recovery operation. The design preserves normal text
and JSON modes. It adds no full-screen mode, prompt, confirmation UI, cursor
control, progress animation, raw terminal input, daemon or new recovery command.

The CLI must not kill a process, delete a lock, mutate native client
configuration or manage extension installation. It does not redesign unrelated
doctor checks or extension release/enable/disable behavior; adoption of an
inventory row without a native fingerprint is refused until the source is
restored and rescanned.

## User jobs

1. After a `state_busy` failure, identify whether core state or scanning is
   busy and know the next safe step without guessing which lock file to delete.
2. Inspect both lock classes read-only, including the difference between live,
   automatically reclaimable, legacy and unknown ownership.
3. Understand why an extension warning exists and whether inventory
   synchronization can clear it.
4. Run one explicit recovery command, see exactly what AgentDeck-owned state it
   changed, and re-run diagnosis to confirm recovery.
5. Consume the same reason/action semantics from scripts through stable JSON
   without parsing localized or wrapped prose.

## Terminal matrix

| Context | Contract |
| --- | --- |
| macOS terminal, 80×24 or larger | One field or action per line; no table is required for recovery output. |
| Narrow terminal, 40–79 columns | Labels remain on their own logical lines; explanatory prose wraps naturally; IDs and executable commands are never truncated. |
| Very narrow terminal, below 40 columns | Same ordered line mode with natural wrapping. Decoration is absent, so there is no separate layout collapse or minimum-width rejection. |
| Tall or redirected output | Output order and content are unchanged; no viewport, paging or cursor movement exists. |
| `TERM=dumb`, `NO_COLOR`, `--no-color` | Identical plain labels and punctuation; meaning never depends on color, icons or Unicode. |
| `--format json`, piped stdout | Versioned JSON only; no ANSI, progress text, prompts or localized field names. |
| Non-TTY stderr | Errors remain complete newline-terminated records; no carriage-return replacement or transient status. |

Visible-width fitting is needed only for wrapping prose. Stable identifiers and
commands are copyable byte-for-byte even when the terminal wraps them visually.
Before text rendering, every externally sourced discovery diagnostic passes
through a shared single-line terminal sanitizer. The sanitizer normalizes
invalid UTF-8 to the replacement character and replaces each contiguous run of
CR, LF, tab, C0/C1, DEL, ESC, ANSI or OSC control content with one ASCII space,
then trims surrounding spaces. Text output never receives raw external control
bytes. JSON retains valid JSON escaping rather than applying this lossy text
normalization, while the stable diagnostic classification remains independent of
raw prose in both modes.

## Information hierarchy

Every health result follows this order:

1. **result** — success, warning or error and its stable code;
2. **resource/reason** — the affected lock class or extension condition;
3. **next step** — safe prose describing what must happen next;
4. **executable recovery** — present only when the exact command can advance
   recovery safely; and
5. **effect/safety boundary** — what that command changes and explicitly does
   not change when mutation could be misunderstood.

The labels have fixed meanings:

| Label | Meaning |
| --- | --- |
| `next:` | Human-readable prerequisite or sequence; never parsed as a command. |
| `diagnose:` | Optional read-only command that can refine the condition. |
| `recovery:` | Exact executable command that can safely advance recovery. It is omitted when no such command exists. |
| `effect:` | Bounded mutation performed by the recovery command. |
| `manual prerequisite:` | External confirmation or operator step required before an action that AgentDeck will not perform automatically. It is prose, never an executable command. |

`recovery:` is not used for upgrade prose, waiting, process inspection or an
unconditional destructive filesystem command. Existing JSON
`recovery_command` follows the same rule.

## Lock contention states and transitions

### Immediate command error

`state_busy` remains the stable error code and exit status `1`. The error must
identify the busy resource when the caller knows it:

```text
state_busy: AgentDeck state is busy.
  resource: state
  next: Wait for the current state operation to finish, then retry this command.
  diagnose: agentdeck doctor
```

```text
state_busy: AgentDeck scan is busy.
  resource: scan
  next: Wait for the current scan to finish, then retry this command.
  diagnose: agentdeck doctor
```

The error never prints a lock nonce, raw filesystem error, SQLite text or an
unverified owner PID. When a legacy caller cannot yet provide a lock class, the
resource is `unknown`, not guessed as `state`:

```text
state_busy: AgentDeck is busy.
  resource: unknown
  next: Run read-only diagnostics before taking recovery action.
  diagnose: agentdeck doctor
```

When a classified `state` or `scan` lock is legacy or has unknown ownership,
the immediate text uses the safe manual or fail-closed next step shown by doctor
below instead of the live-owner wait-and-retry sentence. It still offers
`agentdeck doctor` as read-only diagnosis and never prints an executable lock
deletion command.

### Doctor lock checks

Doctor reports `state_lock` and `scan_lock` as separate checks in stable order.
Absent locks are `ok` and have no recovery lines. A verified live owner is a
warning with a retry prerequisite, not a stale or removable lock:

```text
state_lock: warning (lock_live)
  next: Let the current state operation finish, then retry the failed command.
```

```text
scan_lock: warning (lock_live)
  next: Let the current scan finish, then retry the failed command.
```

A modern stale lock that the acquisition protocol can safely reclaim is
reported after recovery as an informational result or normal `ok`; doctor does
not advertise manual deletion for it. Architecture decides whether a distinct
non-warning `lock_reclaimable` observation is useful, but the CLI must not ask
the user to perform work the next acquisition already handles safely.

Unknown or legacy ownership fails closed:

```text
state_lock: warning (lock_legacy)
  next: Confirm that no AgentDeck process is using this state directory.
  manual prerequisite: Remove state.lock only after that confirmation.
```

```text
scan_lock: warning (lock_owner_unknown)
  next: Do not remove scan.lock. Stop the owning AgentDeck process through its normal lifecycle or wait and diagnose again.
```

There is no `recovery:` line for either example. Age alone does not change this
contract. If a definite future schema is found while state acquisition is busy,
the existing `schema_ahead` output and upgrade guidance take precedence; an
inconclusive probe returns the correctly classified `state_busy` output above.

### Lock transition model

```text
command
  ├─ acquired ───────────────────────────────> continue normally
  ├─ modern stale + safely reclaimed ────────> continue normally
  └─ busy
       ├─ state / live ──────> wait or normal owner shutdown ─> retry
       ├─ scan / live ───────> wait or normal owner shutdown ─> retry
       ├─ legacy ────────────> manual confirmation prerequisite
       └─ unknown ───────────> diagnose; no destructive command
```

Repeated diagnosis is stable and read-only. It does not change the lock,
advance timestamps, suppress the warning or manufacture an owner classification.

## Extension diagnosis and recovery states

### Read-only entry and unavailable state

The current implementation conflicts with this design: `extension doctor` uses
the shared `withExtensions` entry, which calls writable `store.Open`, acquires
`state.lock`, and may create, migrate or chmod core state. Architecture must
provision a distinct inventory-read entry for `extension doctor` and aggregate
doctor. That entry must not acquire `state.lock`, create a state root or database,
run migrations, enable WAL, repair permissions or write settings. A live
`state.lock` therefore cannot by itself prevent extension diagnosis.

The read-only entry has these observable outcomes:

| Stored-state condition | Text/JSON reason | Result |
| --- | --- | --- |
| Existing readable supported database | The applicable extension reason below, or no reason when healthy | Complete the diagnostic report without mutation. |
| Missing state root or core database | `extension_state_missing` | Complete with warning, mark persisted inventory comparisons unavailable, and offer `agentdeck extension scan` only after live discovery succeeds. |
| Definite future schema | existing `schema_ahead` | Exit `1` with existing upgrade guidance; emit no scan recovery command. |
| Unreadable, malformed or incompatible non-future database | `extension_inventory_unreadable` | Exit `1` with sanitized bounded prose, `action_kind: "manual_prerequisite"`, and no recovery command or raw SQLite/driver text. |

Discovery remains independently observable. A missing database does not turn a
discovery failure into a scan recommendation, and successful read-only opening
does not permit doctor to refresh inventory or its fingerprint.

### Healthy inventory

`agentdeck extension doctor` keeps a concise line-oriented success state and
exit `0`:

```text
status: ok
diagnostics: none
stale inventory: none
duplicate ids: none
drifted ids: none
management anomalies: none
```

The exact section order remains stable in text. `stale inventory` replaces the
misleading text label `missing paths` for persisted IDs absent after successful
live discovery. JSON compatibility for the former field is an architecture
decision; the surface requires an additive, explicitly named stale-inventory
classification rather than silently changing the old field's meaning.

### Stale identity after successful discovery

```text
status: warning
diagnostics: none
stale inventory (1):
  - codex:mcp:user:computer-use
duplicate ids: none
drifted ids: none
management anomalies: none
next: Synchronize AgentDeck's extension inventory with current native discovery.
recovery: agentdeck extension scan
effect: Updates AgentDeck's derived extension inventory and extension scan fingerprint only; does not modify Codex or Claude configuration or installed extensions.
```

The stable extension ID may be shown because it is the resource being repaired;
native configuration contents, environment values and discovery paths are not
shown. The recovery command is on its own line and is never truncated.

### Discovery failure

When live discovery fails, the command preserves prior inventory and withholds a
guaranteed recovery command:

```text
status: warning
diagnostics (1):
  - Native extension discovery failed.
stale inventory: unavailable
duplicate ids: unavailable
drifted ids: unavailable
management anomalies: unavailable
next: Resolve the discovery error, then run agentdeck extension doctor again.
```

There is no `recovery: agentdeck extension scan` line because that command uses
the same failed discovery prerequisite. Raw native configuration, sensitive
paths and driver errors remain redacted.

### Other extension conditions

Duplicate live IDs, managed drift and management anomalies remain separate
sections. Inventory scan is not advertised as a universal repair:

- duplicate discovery requires resolving the duplicate native definitions;
- managed drift uses the existing explicit adopt/release ownership contract;
- management anomalies require their cause-specific prerequisite; and
- combinations list all conditions, but present `extension scan` only when
  stale inventory is independently present and discovery succeeded.

### Synchronization result and recovery confirmation

`agentdeck extension scan` keeps its existing success counters and exit `0`.
For the stale-identity scenario, its observable result names removal and current
discovery without exposing content:

```text
found: 1
added: 1
updated: 0
removed: 1
unchanged: 0
codex:mcp:user: 1
```

The user confirms recovery by running `agentdeck extension doctor` or
`agentdeck doctor --full` again. Successful synchronization does not print
“fixed” before the follow-up diagnostic actually returns healthy. Cancellation
or discovery failure before atomic replacement preserves the previous inventory
and returns exit `1` on stderr/JSON error output.

The inventory transaction is the commit point, but the existing command also
updates AgentDeck's `watch.fingerprint.extension` setting afterward. A successful
scan reports exit `0` only after both AgentDeck-owned derived-state steps finish.
Cancellation observed after inventory commit does not claim rollback: the command
finishes the bounded fingerprint step when possible and reports the committed
result. If that post-commit step fails or cannot finish, text and JSON exit `1`
with code `extension_sync_incomplete`, reason
`extension_fingerprint_update_failed`, `action_kind: "diagnose"`,
`recovery_command: null`, and an explicit `inventory_committed: true` detail.
Text says that inventory changed but its scan fingerprint did not, then uses
`diagnose: agentdeck extension doctor`; follow-up diagnosis reads the committed
inventory and determines health before the user decides whether to retry scan.
Neither mode says that native client configuration changed or that inventory was
rolled back.

### Extension transition model

```text
doctor
  ├─ discovery succeeds
  │    ├─ inventory agrees ─────────────────> healthy
  │    ├─ stale inventory ─> extension scan ─> doctor again
  │    └─ other diagnostic ─> cause-specific next step
  └─ discovery fails ───────> preserve inventory; resolve prerequisite
```

## Aggregate doctor presentation

`agentdeck doctor` and `agentdeck doctor --full` retain their existing header,
check ordering, warning counts and exit `0` for a completed diagnostic report.
Their health rows use the action rules above:

```text
status: warning
mode: full
warnings: 1
errors: 0
extensions: warning (extension_stale_inventory; count=1)
  recovery: agentdeck extension scan
  effect: Updates AgentDeck's derived extension inventory and extension scan fingerprint only; does not modify native client configuration.
```

If extension diagnostics contain only discovery failure or a condition that
scan cannot repair, `recovery_command` is absent and text shows `next:` prose.
The aggregate report may direct the user to `agentdeck extension doctor` with a
`diagnose:` label for details, but it must not present that read-only command as
the repair itself.

## JSON and automation contract

JSON never relies on text wrapping or parsing the human message. Existing
envelope fields, command names, `schema_version: 1`, exit codes and
`error.code: state_busy` remain stable. The surface requests additive typed data
owned by architecture:

```json
{
  "schema_version": 1,
  "command": "session.rebuild",
  "data": null,
  "warnings": [],
  "partial": false,
  "error": {
    "code": "state_busy",
    "message": "AgentDeck scan is busy.",
    "details": {
      "resource": "scan",
      "reason": "lock_live",
      "action_kind": "retry",
      "recovery_command": null
    }
  }
}
```

The field container and additive versioning mechanism belong to architecture,
but schema version 1 freezes this complete machine vocabulary:

| Field | Exact values |
| --- | --- |
| `resource` | `state`, `scan`, `extension_inventory`, `unknown` |
| lock `reason` | `lock_live`, `lock_legacy`, `lock_owner_unknown`, `lock_reclaimable` |
| extension `reason` | `extension_state_missing`, `extension_discovery_failed`, `extension_stale_inventory`, `extension_duplicate_id`, `extension_managed_drift`, `extension_management_anomaly`, `extension_native_unavailable`, `extension_inventory_unreadable`, `extension_fingerprint_update_failed` |
| `action_kind` | `diagnose`, `retry`, `synchronize_inventory`, `manual_prerequisite`, or JSON `null` when no safe action applies |

`schema_ahead` remains the existing top-level error code rather than a new reason
alias. `unknown` is only a resource value for a caller that cannot identify the
lock class; it is not a wildcard reason or action. Healthy results omit or null
the reason and action fields. `recovery_command` is nullable and is present only
when the named command safely advances recovery.

Producers must not emit another version-1 token without reviewed versioning.
Consumers that receive a missing or unrecognized non-healthy resource, reason or
action value fail closed: show a generic warning, treat `action_kind` as null,
and expose no executable recovery command. They must not map an unknown value to
retry, synchronization or a manual filesystem action. Text labels are not
machine tokens; every text/JSON specimen and architecture data requirement uses
the vocabulary above.

Doctor check JSON similarly needs cause-specific reason/action data. Extension
doctor needs discovery status and a separate stale-inventory collection so
automation never infers recovery from prose or the historical `missing_paths`
name. Missing additive fields remain safe for older desktop clients and scripts.

JSON success remains on stdout; JSON failure remains on stderr. Human errors
remain on stderr. Doctor and extension-doctor reports remain on stdout. No
progress, prompt or human prose is mixed into JSON.

## Input lifecycle, cancellation and signals

- All commands remain non-interactive. They do not read stdin, enter raw mode,
  hide the cursor, install terminal key handlers or require confirmation.
- Ctrl-C or context cancellation stops diagnosis or discovery through existing
  cancellation paths and exits non-zero. Before extension inventory replacement,
  cancellation preserves the previous inventory; after the atomic commit, the
  command reports the committed result rather than pretending cancellation
  rolled it back.
- EOF and redirected stdin have no effect because these commands accept no
  input. Piped stdout is ordinary line or JSON output.
- A diagnostic command, including `extension doctor`, never acquires the
  exclusive state lock and performs no mutation while opening state, discovering
  native extensions or printing recovery guidance.
- Retrying is always a new explicit command invocation. No hidden polling,
  countdown, automatic retry or timing-only transition exists.

## Fallback and accessibility behavior

- Fixed labels and stable order make text usable with screen readers and copied
  transcripts. Status is always written as a word, never color alone.
- ASCII punctuation is sufficient; no icons, box drawing or animation is
  required. `NO_COLOR` and `TERM=dumb` therefore preserve full meaning.
- Commands and resource IDs are on dedicated lines so wrapping does not merge
  them with prose. No ellipsis hides an executable command or affected ID.
- English is the CLI contract language. Machine-readable fields remain stable
  English identifiers and are never localized.
- Sensitive external diagnostics are summarized into stable safe categories;
  screen-reader accessibility does not justify printing raw configuration or
  paths.

## Alternatives and decisions

| Decision | Alternative rejected | Reason |
| --- | --- | --- |
| Preserve `state_busy` and add typed resource/action detail | New error codes for every lock state | The stable code already means a valid command could not obtain local state; additive detail preserves consumers while distinguishing recovery. |
| Cause-specific next step | Always say “run doctor” | Repeating diagnosis does not recover stale inventory and is insufficient guidance for live/legacy lock cases. |
| No unconditional lock-removal command | Print `rm state.lock` after an age threshold | Age does not prove owner death; unknown and legacy states must fail closed. |
| Reuse `extension scan` | Add `extension repair` | Scan already owns successful-discovery inventory replacement. A wrapper would duplicate mutation semantics and failure handling. |
| Recommend scan only for stale inventory | Recommend scan for every extension warning | Duplicate IDs, discovery failure and managed drift require different actions; a blanket command remains misleading. |
| Ordered line output | Add an interactive recovery wizard | The jobs are short, scriptable and safety-sensitive; prompts would weaken composability without resolving ownership. |

## Verification design

| Risk | Observable oracle |
| --- | --- |
| State and scan contention become indistinguishable | Golden text and JSON cases for `state`, `scan` and `unknown`; exit remains 1 and code remains `state_busy`. |
| Unsafe lock deletion is suggested | Table-driven live, legacy, unreadable, unknown, PID-reuse and non-Darwin cases assert no executable removal command. |
| Schema probing masks lock semantics | Contract tests prove definite future schema yields `schema_ahead`; missing/malformed/changing probes retain classified `state_busy`. |
| Doctor mutates state | Before/after hashes, modes and directory inventory for quick/full doctor with both locks and missing state. |
| Extension doctor reuses the writable store entry | Tests with live `state.lock`, missing state, future schema and unreadable database assert the read-only outcome table and no create, migrate, chmod, WAL or setting writes. |
| Stale identity remains unrecoverable | Fixture with stored old MCP ID and successful discovery of the new identity: doctor recommends scan, scan reports removed/added, follow-up doctor is healthy. |
| Failed discovery truncates inventory | Cancellation/error fixtures assert prior rows, managed flags and fingerprints are byte/logically unchanged and no recovery command is emitted. |
| Fingerprint update fails after inventory commit | Text/JSON fixtures assert exit `1`, `extension_sync_incomplete`, `inventory_committed: true`, no rollback claim and a diagnostic follow-up; post-commit cancellation has the same truthful boundary. |
| Scan is advertised for the wrong cause | Matrix across stale, duplicate, drift, management anomaly and combinations checks action kind and command presence. |
| Narrow output loses commands or IDs | Visible-cell tests at 32, 40, 80 and 120 columns; commands/IDs remain complete and ordered. |
| Terminal mode leaks control sequences | TTY, non-TTY, `TERM=dumb`, `NO_COLOR` and piped-output assertions contain no ANSI or carriage-return progress. |
| External diagnostics break line hierarchy | Extension-doctor text/error fixtures cover ANSI CSI, OSC, C0/C1, CR, LF, tab, DEL and invalid UTF-8; text contains one sanitized line, while JSON remains validly escaped and preserves the stable category. |
| JSON drifts from text semantics | Stable fixture matrix asserts reason/action enums, nullable command, stdout/stderr placement and backward-compatible omission handling. |
| Cancellation leaves terminal or state dirty | Process/PTY cancellation tests verify ordinary terminal mode, exit status and unchanged inventory before commit. No raw-mode restoration test is needed because the design never enters raw mode. |

Rendered text specimens establish hierarchy, copyability and narrow-terminal
wrapping. They do not prove lock liveness, atomic inventory replacement,
cancellation or privacy; those require source-level and integration oracles.

## Assumptions and unresolved implementation questions

The product decisions in this framework are complete. Architecture must still
decide, without changing the observable contract:

- the typed Go owner for lock resource/reason/action detail;
- how the required read-only doctor and extension-inventory entries provision
  the outcomes above without acquiring the exclusive boundary they diagnose;
- the additive JSON and desktop-wire representation and older-consumer default;
- whether modern reclaimable locks appear as an informational doctor result or
  only as normal post-reclaim success; and
- the canonical extension diagnostic type replacing the current conflated
  `missing_paths` result while retaining compatibility.

These are provisioning decisions, not open UX choices. Final CLI reconciliation
must confirm the settled architecture supplies every field and preserves the
specimens and action taxonomy above.

## Review boundary

Framework review decides whether the line-oriented mode, user jobs, terminal
matrix, hierarchy, lock and extension state transitions, text/JSON specimens,
command semantics, cancellation, accessibility and verification oracles are
complete and consistent with reviewed requirements. It does not approve Go
types, lock inspection algorithms, desktop presentation or implementation
tasks. A later final-surface reconciliation checks this document against the
reviewed architecture before decomposition.
