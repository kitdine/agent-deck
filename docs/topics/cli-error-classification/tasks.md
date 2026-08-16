---
status: active
created: 2026-08-16
updated: 2026-08-16
---

# CLI Error Classification — Tasks

This file is the only status authority for this topic.

## Task breakdown

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

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [ ] |
| architecture.md | [x] | [ ] |
| tasks.md | [x] | [ ] |
| `ux/` | n/a | n/a |

The `ux/` row is stated rather than omitted so a reader can tell a decision from
an oversight: every command in scope keeps its current output shape, and the
observable change is a machine-read error code, which `architecture.md` owns.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `typed-not-found-errors` | [ ] | [ ] |
| 2. `stable-error-codes` | [ ] | [ ] |

Tasks are sequential. Commit boundaries follow task boundaries. This topic does
not authorize commits, pushes, release preparation, or assembly into any
version.

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
进入开发：`cli-error-classification` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the error and exit-code
contract in `docs/specs/cli-design.md`, and verification routing. Tick `Dev`
only after the task's selected verification passes. An independent reviewer
records a PASS round under `reviews/<task-anchor>.md` before ticking `Review`.
