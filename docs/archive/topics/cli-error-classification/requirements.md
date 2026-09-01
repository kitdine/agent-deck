---
status: historical
created: 2026-08-16
updated: 2026-08-17
retired: 2026-09-01
---

# CLI Error Classification — Requirements

`v0.5.0` has selected this topic. Version membership is decided by a
`vX-Y-Z-contract` topic's assembly list, not here; the
[`v0.5.0` contract topic](../../../topics/v0-5-0-contract/tasks.md#assembly-list) records the
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
| `backup inspect /tmp/nonexistent.adb`, passphrase supplied | `runtime_error` | `open /tmp/nonexistent.adb: no such file or directory` |
| `session show <unknown uuid>` | `runtime_error` | `no session "<id>" is known` |
| `extension show codex:skill:nonexistent` | `extension_not_found` | `extension_not_found: codex:skill:nonexistent` |

The `backup inspect` row was re-measured on 2026-08-17 against a build of
`1a205e2a`, because the original figure recorded a different failure. `inspect`
reads the passphrase before it opens anything, so with a character-device stdin the
command never reached the archive: it returned `operation not supported by device`,
the Darwin rendering of the ioctl error from reading a passphrase, and no archive
lookup occurred. Supplying a passphrase reaches the real absence path shown above,
which leaks the full caller-supplied path. The passphrase-input failure is a
separate defect in a different layer and is excluded below.

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
- No `error.message` for a command in the evidence table carries `database/sql`,
  driver, errno, or file-path text. The stable code lives in `error.code`; a
  message is human-readable text naming the missing thing, carrying at most the
  one caller-supplied identifier the Message identity table below permits for
  that row, and nothing else. Messages outside the evidence table are out of
  scope; a later topic may extend the rule.
- `docs/specs/cli-design.md` documents the complete error-code table, so
  `runtime_error` means an unclassified failure rather than an undocumented
  default.
- Regression coverage asserts the code, the absence of storage text, and that
  each message matches its Message identity row.

## Non-Goals

- No change to exit statuses. They are already correct: a failed command exits
  non-zero, and `provider use` exits `1` exactly as `session show` does.
- No change to where the envelope is written. Failures correctly go to stderr.
- No new command, flag, or output format.
- No localization of error messages. Codes are for machines; the desktop app
  supplies user-facing text from the code.
- No broad audit of unrelated `runtime_error` cases. Scope is exactly the
  not-found conditions in the evidence table and the storage text those rows
  leak; a genuinely unexpected runtime failure elsewhere may keep
  `runtime_error` and its current message.
- No renaming of `extension_not_found`, which is already correct and already
  consumed.
- No classification of `backup inspect`'s passphrase-input failure. Reading the
  passphrase happens before any archive lookup and fails for reasons that have
  nothing to do with a missing target, so it is not a not-found condition and gets
  no code here. It leaks an errno string today and still will; that is a real
  defect, in the CLI input layer rather than in any domain's lookup, and it needs
  its own topic. Every `backup inspect` scenario in this topic supplies a
  passphrase so it reaches the archive.

## Message identity

The stable code lives in `error.code`, and a message is not required to repeat
it. A machine consumer reads the code field; the message exists for a human
reader, and duplicating the code into it buys nothing while making the two fields
drift apart. That decision is what lets `session show` keep the text this document
already calls correct. `extension_not_found: <id>`, whose message happens to begin
with its code, stays valid and unchanged because consumers already read it, but it
is a preserved shape rather than the form new rows must copy.

"Names the missing thing" then needs a per-row decision, because for some commands
the only identifier the CLI receives is one the privacy rule forbids repeating:

| Command | `error.message` | Permitted caller-supplied identifier |
| --- | --- | --- |
| `provider show`, `provider use` | Names the missing provider | The provider name the caller supplied — a user-chosen label, not a path |
| `credential show` | Names the missing credential | The credential reference the caller supplied |
| `backup inspect` | Names the missing or unreadable backup archive by kind only | None. The caller supplies only a filesystem path, and its basename is still caller-supplied path text |
| `session show` | Unchanged: `no session "<id>" is known` | The session identifier |
| `extension show` | Unchanged: `extension_not_found: <id>` | The extension id |

So "and nothing else" means that naming plus at most this one identifier — never
a third element, and never storage, driver, errno, or path text. `session show`
satisfies the rule as written, which is why its already-correct text survives.
`backup inspect` satisfies it with the identifier omitted, which is the only
privacy-safe answer available when the caller named the target by path; a consumer
that needs to know which archive failed already holds the path it passed.

## Surfaces and contracts

This topic changes no interactive surface. The observable change is the
documented JSON error contract, specified in
[`architecture.md`](architecture.md). The document set itself is declared in
[`tasks.md`](tasks.md)'s Documents matrix, not here.

## Acceptance boundary

- Every command in the evidence table above reports a stable, documented code
  instead of `runtime_error`. The `backup inspect` scenario supplies a passphrase,
  so it exercises the archive lookup rather than the excluded passphrase input.
- No `error.message` in that table contains `sql:`, `no rows in result set`, an
  errno string, or a filesystem path, and a test asserts this rather than a
  reviewer reading it.
- Each message in that table matches its Message identity row: at most the
  permitted caller-supplied identifier, none for `backup inspect`, and no
  requirement that a message repeat its `error.code`.
- `docs/specs/cli-design.md` documents each code.
- Existing consumers of `extension_not_found` and `state_busy` are unaffected.

## Compatibility

A consumer that matched `runtime_error` for a missing target will now receive a
more specific code. That is the point of the change, but it is an observable JSON
contract change and belongs in the release notes of whichever version assembles
this topic.
