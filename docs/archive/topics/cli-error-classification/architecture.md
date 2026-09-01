---
status: historical
created: 2026-08-16
updated: 2026-08-17
retired: 2026-09-01
---

# CLI Error Classification — Architecture

Specifies where not-found conditions become typed errors, how they reach a
stable code, and what an `error.message` may contain. The defect and its
evidence are in [`requirements.md`](requirements.md), whose Message identity
table is the normative input to the construction rules below.

## Root cause

`internal/store` returns bare `sql.ErrNoRows` for missing rows, and
`internal/provider/service.go` passes it through unwrapped (`UseCredential`
returns `lookupErr` directly). Nothing in the chain converts it into a domain
error, so `errorCode` in `cmd/agentdeck/main.go` has nothing to match and falls
through to `runtime_error`.

The chain therefore has two defects at different layers, and both must be fixed:
the store loses the domain meaning of "absent", and the CLI has no way to
recover it.

The existing `store.Error` wrapper cannot be reused for this. Its `Error()`
renders `Code + ": " + Err`, so wrapping `sql.ErrNoRows` in it would put the
driver sentinel back into the JSON message under a new code — the same defect
with a better label.

## Ownership boundary

The code is decided by the domain that owns the concept, not by string
inspection in the CLI layer. Each lookup boundary converts absence into a typed
error before returning:

| Concept | Owning boundary | Condition | Construction site |
| --- | --- | --- | --- |
| Provider | `internal/store/providers.go`, `internal/provider/service.go` | Named provider row absent | Store lookup, at the `sql.ErrNoRows` return |
| Credential | `internal/store/providers.go` | Named credential row absent | Store lookup, at the `sql.ErrNoRows` return |
| Backup archive | `internal/backup/backup.go` | See the backup classification matrix | `readEncrypted`, at the `os.Open` result |
| Session | `internal/session` owns the code; `cmd/agentdeck/main.go` keeps the construction | Named session unknown to the index | Existing `sessionShowNotFound` |

Session is the one split row, and deliberately. Its message is chosen by
consulting both the session index and the `usage_sessions` table, so the
construction needs data the `internal/session` package does not hold. Moving that
query is a larger refactor than this topic authorizes. The *code* is still a
domain constant declared in `internal/session`; only the message construction
stays where it already works.

## Error catalogue

One carrier type expresses every row, in a new leaf package `internal/errdefs`
that imports nothing beyond the standard library. `internal/backup` and
`internal/session` do not currently import `internal/store`, and this contract
must not make them:

```go
package errdefs

// NotFound carries a stable error.code and an already-redacted message.
// Error() never renders the cause: redaction is structural, not a review habit.
type NotFound struct {
    Code    string // the stable error.code
    Message string // redacted, per the Message identity table
    cause   error  // optional; matched by errors.Is, never rendered
}

func NewNotFound(code, message string, cause error) *NotFound
func (e *NotFound) Error() string { return e.Message }
func (e *NotFound) Unwrap() error { return e.cause }
```

`cause` is unexported on purpose: `NewNotFound` is the only way to attach one, so
every construction passes through the boundary that decides the redacted message,
and no caller can assemble a `NotFound` whose text and cause disagree.

Each owning package declares its code as an exported constant, so the value has
one definition and the CLI never spells it:

| Concept | Code constant | `error.code` | Cause preserved | Matching rule |
| --- | --- | --- | --- | --- |
| Provider | `store.CodeProviderNotFound` | `provider_not_found` | `sql.ErrNoRows` | `errors.As(err, &notFound)` with `var notFound *errdefs.NotFound`; `errors.Is(err, sql.ErrNoRows)` still holds |
| Credential | `store.CodeCredentialNotFound` | `credential_not_found` | `sql.ErrNoRows` | same |
| Backup archive absent | `backup.CodeArchiveNotFound` | `backup_not_found` | `fs.ErrNotExist` | same |
| Backup archive unreadable | `backup.CodeArchiveUnreadable` | `backup_unreadable` | the underlying `os.Open` error | same |
| Session | `session.CodeSessionNotFound` | `session_not_found` | `sql.ErrNoRows` | same |

The cause stays in the chain because `errors.Is(err, sql.ErrNoRows)` is already
asserted by existing `session show` tests, and keeping it costs nothing: leakage
is a property of the rendered text, and `NotFound.Error()` returns only
`Message`. No call site may render the cause with `%v` or `%w` into a message.

## Backup classification matrix

`backup inspect`'s failures are not one condition. `readEncrypted` currently
returns the raw `os.Open` error, and maps authentication and archive-structure
failures to the already stable `invalid_backup`. Splitting them is required, and
`invalid_backup` keeps its current meaning and text:

| Condition | `error.code` | `error.message` | Change |
| --- | --- | --- | --- |
| `os.Open` fails, `errors.Is(err, fs.ErrNotExist)` | `backup_not_found` | Names the missing backup archive by kind, with no path | New |
| `os.Open` fails otherwise — permission, any other errno | `backup_unreadable` | Names the unreadable backup archive by kind, with no path or errno | New |
| `age` passphrase or decrypt failure | `invalid_backup` | Unchanged | None |
| tar entry, duplicate entry, entry authentication, manifest failure | `invalid_backup` | Unchanged | None |

Both new rows are needed because absence is not the only way this command leaks.
Reproduced against a build of HEAD `1a205e2a`, each with a passphrase supplied:

```text
$ printf 'pass\n' | agentdeck --format json backup inspect /tmp/nonexistent-xyz.adb
"code":"runtime_error","message":"open /tmp/nonexistent-xyz.adb: no such file or directory"

$ printf 'pass\n' | agentdeck --format json backup inspect <mode-000 file>
"code":"runtime_error","message":"open <path>: permission denied"
```

Both leak the full caller-supplied path today, and only the absence branch would
otherwise be classified. A directory argument is not one of these rows: `os.Open`
succeeds on it and the failure surfaces later as the unchanged
`invalid_backup: authentication`.

**Every `backup inspect` condition here is reached with a passphrase supplied.**
`inspect` reads the passphrase before it opens anything, and that read has its own
failure mode — with a character-device stdin, `term.ReadPassword`'s ioctl fails
with ENODEV, rendered on Darwin as `operation not supported by device`. That is
the figure the requirements evidence table originally recorded, and no archive
lookup had occurred when it was produced. The requirements now record the
re-measured absence path instead and exclude the passphrase-input failure by name:
it is not a not-found condition, it belongs to the CLI input layer rather than to
any domain's lookup, and it needs its own topic. This architecture therefore gives
it no code, no carrier, and no mapping. It still leaks an errno string after this
topic ships, which the requirements state as a known residual.

## Message construction

The requirements Message identity table is normative. The stable code lives in
`error.code`; **a new `error.message` does not repeat it**. Per row:

| Command | `error.code` | `NotFound.Message` construction |
| --- | --- | --- |
| `provider show`, `provider use` | `provider_not_found` | Names the missing provider using the caller-supplied name — a user-chosen label, never a path |
| `credential show` | `credential_not_found` | Names the missing credential using the caller-supplied reference |
| `backup inspect`, absent | `backup_not_found` | Names the missing backup archive by kind only, with no caller-supplied identifier |
| `backup inspect`, unreadable | `backup_unreadable` | Names the unreadable backup archive by kind only |
| `session show` | `session_not_found` | Unchanged. Both existing texts are preserved: `no session "<id>" is known`, and the stale-index variant naming `agentdeck session scan`. Neither contains storage text |
| `extension show` | `extension_not_found` | Unchanged `extension_not_found: <id>`. Its leading code is a preserved compatibility shape, not the form the new rows copy |

`extension_not_found` therefore stops being the construction pattern for new
errors while remaining exactly as consumers read it today; the earlier "the error
text is the stable code" reading is what would have made every new message repeat
its code, against the approved contract.

A message must not contain:

- `sql:` or `no rows in result set`, or any other `database/sql` or driver
  sentinel;
- an errno string such as `operation not supported by device`;
- a filesystem path.

This is asserted by a test over the mapped messages rather than checked by a
reviewer reading output. The assertion is per row: the code, the presence of the
row's permitted identifier, and the absence of all three forbidden classes.

## Code mapping

`errorCode` gains one case, not five, because the code travels on the error:

```go
var notFound *errdefs.NotFound
case errors.As(err, &notFound):
    return notFound.Code
```

It is placed before the `store.ErrStateBusy` case and after the existing sentinel
cases, so no currently mapped code changes. Nothing else is added to `errorCode`:
the excluded passphrase-input failure keeps falling through to `runtime_error`.

After the change, `runtime_error` means an unclassified failure. It is a
documented residual category, not a default.

## Contract impact

`docs/specs/cli-design.md` gains the error-code table: the five new codes above —
`provider_not_found`, `credential_not_found`, `backup_not_found`,
`backup_unreadable` and `session_not_found` — plus every code already in use, none
of which it currently documents. `runtime_error` is documented as the residual
category, which is what the excluded passphrase-input failure keeps returning.
`docs/specs/cli-manual.md` is reconciled where it shows the affected commands.
Both are updated in the task that changes the values, not afterwards.
