---
status: active
created: 2026-08-16
---

# CLI Error Classification

Target release: not yet assigned. This plan is deliberately unassigned, because
its changes must not enter the `v0.5.0` tag — they are not part of that version's
scope. Assembly happens when a version contract plan selects this plan; see
`.agent-instructions/branching.md`.

Surfaced while specifying the desktop menu-bar switch contract, recorded in
`docs/reviews/desktop-app/menubar-experience.md` Round 2. The menu-bar design
works around the defect without hiding it, which is why the workaround is not the
fix.

## Problem

`errorCode` in `cmd/agentdeck/main.go` maps a fixed list of typed errors to
stable codes and sends everything else to `runtime_error`. Not-found conditions
are absent from that list, so a consumer cannot distinguish "the thing you named
does not exist" from any other runtime failure, and some of those failures carry
raw storage text into a documented JSON contract.

Measured on 2026-08-16 with the released `v0.4.1` binary, using
`agentdeck --format json <command> 2>&1 >/dev/null`:

| Command with a missing target | `error.code` | `error.message` |
| --- | --- | --- |
| `provider show nonexistent-xyz` | `runtime_error` | `sql: no rows in result set` |
| `provider use nonexistent-xyz --client codex` | `runtime_error` | `sql: no rows in result set` |
| `credential show nonexistent-xyz` | `runtime_error` | `sql: no rows in result set` |
| `backup inspect /tmp/nonexistent.adb` | `runtime_error` | `operation not supported by device` |
| `session show <unknown uuid>` | `runtime_error` | `no session "<id>" is known` |
| `extension show codex:skill:nonexistent` | `extension_not_found` | `extension_not_found: codex:skill:nonexistent` |

The last row is the target shape and already exists, which makes this a
consistency defect rather than a new capability: `extension_not_found` is a stable
code whose message names the missing thing and leaks nothing.

Three distinct problems:

1. **No stable not-found code.** Every case above except extensions reports
   `runtime_error`, which appears zero times in `docs/specs/cli-design.md` and
   `docs/specs/cli-manual.md`. It is an implementation fallback that consumers
   have no documented way to interpret.
2. **Storage text reaches the contract.** `sql: no rows in result set` is a Go
   `database/sql` sentinel string. It exposes the persistence layer through a
   documented interface, cannot be localized, and means nothing to a user.
   `backup inspect` leaks a filesystem errno string the same way.
3. **`session show` proves message quality is not enough.** Its message is
   already good, yet its code is still `runtime_error`, so a machine consumer
   must string-match the message to recognize a missing session.

Root cause: `internal/store` returns bare `sql.ErrNoRows` for missing rows, and
`internal/provider/service.go` passes it through unwrapped
(`UseCredential` returns `lookupErr` directly). Nothing in the chain converts it
into a domain error, so `errorCode` has nothing to match.

## Scope

- Introduce typed not-found errors at the boundary that owns each concept, so the
  code is decided by the domain rather than by string inspection in the CLI layer.
- Map them in `errorCode`, following the existing `extension_not_found` pattern
  where the error text is the stable code.
- Ensure no `error.message` carries `database/sql`, driver, errno, or file-path
  text. A message names the missing thing and nothing else.
- Document the error codes in `docs/specs/cli-design.md`, which currently
  documents none of them, and reconcile `docs/specs/cli-manual.md` where it shows
  affected commands.
- Add regression coverage asserting both the code and the absence of storage text.

## Non-goals

- No change to exit statuses. They are already correct: a failed command exits
  non-zero, and `provider use` exits `1` exactly as `session show` does.
- No change to where the envelope is written. Failures correctly go to stderr.
- No new command, flag, or output format.
- No localization of error messages. Codes are for machines; the desktop app
  supplies user-facing text from the code.
- No broad audit of unrelated `runtime_error` cases. Only not-found conditions
  and leaked storage text are in scope; a genuinely unexpected runtime failure
  may keep `runtime_error`.
- No renaming of `extension_not_found`, which is already correct and already
  consumed.

## Tasks

### 1. `typed-not-found-errors`

Add typed not-found errors for provider, credential, and backup archive lookups
at their owning boundaries, and wrap the store's bare `sql.ErrNoRows` so callers
receive a domain error. Make `session show`'s existing not-found condition typed
as well, keeping its current message text.

- Files: `internal/store/providers.go`, `internal/store/store.go`,
  `internal/provider/service.go`, `internal/backup/backup.go`,
  `internal/session/session.go`, tests.
- Verification level: L2. The change alters a persisted-adjacent error path
  consumed by a documented JSON contract.

### 2. `stable-error-codes`

Map the new errors in `cmd/agentdeck/main.go`'s `errorCode`, assert that no
mapped message contains `sql:`, `no rows in result set`, an errno string, or a
filesystem path, and record the complete error-code table in
`docs/specs/cli-design.md`, reconciling `docs/specs/cli-manual.md` for the
affected commands.

- Depends on task 1.
- Files: `cmd/agentdeck/main.go`, `docs/specs/cli-design.md`,
  `docs/specs/cli-manual.md`, tests.
- Verification level: L2.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `typed-not-found-errors` | [ ] | [ ] |
| 2. `stable-error-codes` | [ ] | [ ] |

Tasks are sequential. Commit boundaries follow task boundaries. This plan does
not authorize commits, pushes, release preparation, or assembly into any version.

## Acceptance

- Every command in the evidence table above reports a stable, documented code
  instead of `runtime_error`.
- No `error.message` in that table contains `sql:`, `no rows in result set`, an
  errno string, or a filesystem path, and a test asserts this rather than a
  reviewer reading it.
- `docs/specs/cli-design.md` documents each code, so `runtime_error` means an
  unclassified failure rather than an undocumented default.
- Existing consumers of `extension_not_found` and `state_busy` are unaffected.

## Compatibility

A consumer that matched `runtime_error` for a missing target will now receive a
more specific code. That is the point of the change, but it is an observable JSON
contract change and belongs in the release notes of whichever version assembles
this plan.

## Starting Task

Turn a Status row into scoped development by naming its anchor:

```text
进入开发：`cli-error-classification` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's Problem and named task, the error and exit-code
contract in `docs/specs/cli-design.md`, and verification routing. Tick `Dev` only
after the task's selected verification passes. An independent reviewer records a
PASS round under `docs/reviews/cli-error-classification/<task-anchor>.md` before
ticking `Review`.
