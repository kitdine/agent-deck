---
status: active
created: 2026-08-16
updated: 2026-08-18
---

# CLI Error Classification — Tasks

This file is the only status authority for this topic.

## Task breakdown

Both tasks touch `cmd/agentdeck/main.go`, and the split is by hunk, not by file:
task 1 owns the `sessionShowNotFoundError` constructor, task 2 owns the `errorCode`
mapping. Neither may stage the other's hunk.

### 1. `typed-not-found-errors`

Deliver the approved carrier and every conversion boundary, so that each lookup
returns a domain error whose rendered text is already redacted.

- Add `internal/errdefs` with the `NotFound` carrier and `NewNotFound`, per
  `architecture.md`'s Error catalogue. It imports nothing beyond the standard
  library, because `internal/backup` and `internal/session` must not start
  importing `internal/store`.
- Declare each code as a constant in its owning package: `store.CodeProviderNotFound`,
  `store.CodeCredentialNotFound`, `backup.CodeArchiveNotFound`,
  `backup.CodeArchiveUnreadable`, `session.CodeSessionNotFound`.
- Convert provider and credential lookups where they currently return bare
  `sql.ErrNoRows`, keeping that sentinel as the preserved cause.
- Split `readEncrypted`'s `os.Open` failure into the absent branch
  (`errors.Is(err, fs.ErrNotExist)`) and the unreadable branch. Leave every
  `ErrInvalidArchive` path untouched.
- Retype `session show`'s existing not-found condition in
  `cmd/agentdeck/main.go`'s `sessionShowNotFound`, which the architecture keeps
  there because the message needs the `usage_sessions` query. Both existing texts
  stay byte-identical.

- Files: `internal/errdefs/errdefs.go` (new), `internal/store/providers.go`,
  `internal/store/store.go`, `internal/provider/service.go`,
  `internal/backup/backup.go`, `internal/session/session.go`,
  `cmd/agentdeck/main.go` (constructor hunk only), and the tests below.
- Tests this task owns:
  - `internal/errdefs`: `Error()` returns `Message` and never the cause's text;
    a cause is reachable by `errors.Is`; `errors.As` recovers `*NotFound` and its
    `Code`; a cause can only be attached through `NewNotFound`.
  - `internal/store`: a missing provider and a missing credential each return a
    `*errdefs.NotFound` carrying the right `Code` and the caller-supplied
    identifier, contain neither `sql:` nor `no rows in result set`, and still
    satisfy `errors.Is(err, sql.ErrNoRows)`.
  - `internal/provider`: `UseCredential` propagates the typed error rather than the
    bare sentinel.
  - `internal/backup`: an absent path yields `backup_not_found` and a mode-`000`
    file yields `backup_unreadable`, neither message containing a path or errno;
    the existing wrong-passphrase and tampered-archive assertions still yield
    `invalid_backup` unchanged.
  - `cmd/agentdeck`: `sessionShowNotFound` returns a `*errdefs.NotFound` carrying
    `session_not_found`, both existing texts unchanged, and the existing
    `errors.Is(err, sql.ErrNoRows)` assertions in `main_test.go` stay green.
- Verification level: L2. The change alters a persisted-adjacent error path
  consumed by a documented JSON contract.

### 2. `stable-error-codes`

Map the carriers to `error.code`, prove the whole envelope contract end to end,
and document it.

- Add the single `errors.As` case to `errorCode`, positioned so no currently
  mapped code changes.
- Record the complete error-code table in `docs/specs/cli-design.md` and reconcile
  `docs/specs/cli-manual.md` for the affected commands.

Acceptance matrix — every row asserted against a real command envelope:

| Command | `error.code` | `error.message` | Exit |
| --- | --- | --- | --- |
| `provider show <missing>` | `provider_not_found` | Names the provider; carries the caller-supplied name | 1 |
| `provider use <missing> --client codex` | `provider_not_found` | Same | 1 |
| `credential show <missing>` | `credential_not_found` | Names the credential; carries the caller-supplied reference | 1 |
| `backup inspect <absent>`, passphrase supplied | `backup_not_found` | Names the archive by kind; no identifier | 1 |
| `backup inspect <mode-000>`, passphrase supplied | `backup_unreadable` | Names the archive by kind; no identifier | 1 |
| `session show <unknown>` | `session_not_found` | Unchanged `no session "<id>" is known` | 1 |
| `extension show <missing>` | `extension_not_found` | Unchanged `extension_not_found: <id>` | 1 |

No message in the matrix may contain `sql:`, `no rows in result set`, an errno
string, or a filesystem path. Exit statuses are unchanged, and the existing
`state_busy`, `invalid_backup`, `extension_not_found` and `invalid_argument`
mappings must still resolve exactly as they do today.

- Depends on task 1.
- Files: `cmd/agentdeck/main.go` (`errorCode` hunk only),
  `cmd/agentdeck/error_code_test.go`, `cmd/agentdeck/main_test.go`,
  `docs/specs/cli-design.md`, `docs/specs/cli-manual.md`.
- Tests this task owns:
  - `error_code_test.go`: extend the mapping matrix with the five new codes and
    assert the existing rows are unchanged.
  - `main_test.go`: the JSON `session show` case currently expects
    `"code":"runtime_error"`; it becomes `session_not_found`, keeping its exit-1
    and no-`sql:` assertions.
  - A command-level envelope test covering every row of the acceptance matrix
    above, supplying a passphrase for both `backup inspect` rows.
- Out of scope, by requirements Non-Goal: `backup inspect`'s passphrase-input
  failure keeps returning `runtime_error` with its current errno text. Do not add
  a code for it, and do not write a scenario that reaches it.
- Verification level: L2.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |
| `ux/` | n/a | n/a |

The `ux/` row is stated rather than omitted so a reader can tell a decision from
an oversight. The change is observable in more than the machine code: four of the
seven matrix rows get a new `error.message`, because today's text is a driver
sentinel or a filesystem path. What no row changes is the interaction — same
commands, same flags, same envelope shape, same stream, same exit status — and the
replacement text is fixed per row by `requirements.md`'s Message identity table
rather than being designed here. There is no interaction to specify, so no `ux/`
document is required.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `typed-not-found-errors` | [ ] | [ ] |
| 2. `stable-error-codes` | [ ] | [ ] |

Tasks are sequential. Commit boundaries follow task boundaries, and because both
tasks touch `cmd/agentdeck/main.go`, each commit must be staged by hunk rather
than by file. This topic does not authorize commits, pushes, release preparation,
or assembly into any version.

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
开发：`cli-error-classification` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the error and exit-code
contract in `docs/specs/cli-design.md`, and verification routing. Tick `Dev`
only after the task's selected verification passes. An independent reviewer
records a PASS round under `reviews/<task-anchor>.md` before ticking `Review`.
