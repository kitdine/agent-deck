---
status: active
created: 2026-08-16
updated: 2026-08-16
---

# CLI Error Classification — Requirements

`v0.5.0` has selected this topic. Version membership is decided by a
`vX-Y-Z-contract` topic's assembly list, not here; the
[`v0.5.0` contract topic](../v0-5-0-contract/tasks.md#assembly-list) records the
selection and the reason. See `.agent-instructions/branching.md` for how a
selected topic's branch reaches a tag.

Surfaced while specifying the desktop menu-bar switch contract, recorded in
[`menubar-experience.md`](../desktop-app/reviews/menubar-experience.md) Round 2.
The menu-bar design works around the defect without hiding it, which is why the
workaround is not the fix.

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

## Goals

- Every not-found condition in the evidence table reports a stable, documented
  error code decided by the domain that owns the concept.
- No `error.message` in a documented JSON contract carries `database/sql`,
  driver, errno, or file-path text. A message names the missing thing and
  nothing else.
- `docs/specs/cli-design.md` documents the complete error-code table, so
  `runtime_error` means an unclassified failure rather than an undocumented
  default.
- Regression coverage asserts both the code and the absence of storage text.

## Non-Goals

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

## Surfaces and contracts

This topic changes no interactive surface. The observable change is the
documented JSON error contract, specified in
[`architecture.md`](architecture.md). The document set itself is declared in
[`tasks.md`](tasks.md)'s Documents matrix, not here.

## Acceptance boundary

- Every command in the evidence table above reports a stable, documented code
  instead of `runtime_error`.
- No `error.message` in that table contains `sql:`, `no rows in result set`, an
  errno string, or a filesystem path, and a test asserts this rather than a
  reviewer reading it.
- `docs/specs/cli-design.md` documents each code.
- Existing consumers of `extension_not_found` and `state_busy` are unaffected.

## Compatibility

A consumer that matched `runtime_error` for a missing target will now receive a
more specific code. That is the point of the change, but it is an observable JSON
contract change and belongs in the release notes of whichever version assembles
this topic.
