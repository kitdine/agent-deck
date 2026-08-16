---
status: active
created: 2026-08-16
updated: 2026-08-16
---

# CLI Error Classification — Architecture

Specifies where not-found conditions become typed errors, how they reach a
stable code, and what an `error.message` may contain. The defect and its
evidence are in [`requirements.md`](requirements.md).

## Root cause

`internal/store` returns bare `sql.ErrNoRows` for missing rows, and
`internal/provider/service.go` passes it through unwrapped (`UseCredential`
returns `lookupErr` directly). Nothing in the chain converts it into a domain
error, so `errorCode` in `cmd/agentdeck/main.go` has nothing to match and falls
through to `runtime_error`.

The chain therefore has two defects at different layers, and both must be fixed:
the store loses the domain meaning of "absent", and the CLI has no way to
recover it.

## Ownership boundary

The code is decided by the domain that owns the concept, not by string
inspection in the CLI layer. Each lookup boundary converts absence into a typed
error before returning:

| Concept | Owning boundary | Condition |
| --- | --- | --- |
| Provider | `internal/store/providers.go`, `internal/provider/service.go` | Named provider row absent |
| Credential | `internal/store` | Named credential row absent |
| Backup archive | `internal/backup/backup.go` | Archive path absent or not a readable archive |
| Session | `internal/session/session.go` | Named session unknown to the index |

`internal/store` wraps bare `sql.ErrNoRows` so callers receive a domain error
rather than a driver sentinel. `session show`'s existing not-found condition
becomes typed as well, keeping its current message text, which is already
correct.

## Code mapping

`errorCode` maps the new typed errors following the existing
`extension_not_found` pattern, where the error text is the stable code. The
complete table — new codes plus the codes already in use — is recorded in
`docs/specs/cli-design.md`, which currently documents none of them.

After the change, `runtime_error` means an unclassified failure. It is a
documented residual category, not a default.

## Message rule

An `error.message` names the missing thing and nothing else. It must not
contain:

- `sql:` or `no rows in result set`, or any other `database/sql` or driver
  sentinel;
- an errno string such as `operation not supported by device`;
- a filesystem path.

This is asserted by a test over the mapped messages rather than checked by a
reviewer reading output.

## Contract impact

`docs/specs/cli-design.md` gains the error-code table.
`docs/specs/cli-manual.md` is reconciled where it shows the affected commands.
Both are updated in the task that changes the values, not afterwards.
