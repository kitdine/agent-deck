---
status: active
created: 2026-08-02
---

# Session Show Stale Index Reporting

Target release: `v0.2.2`.

Promoted out of the `docs/README.md` Backlog on 2026-08-02. `agentdeck session
show` returns raw SQL driver text when the separately purgeable session index
does not know a session that the core usage database does — including for the
very command `usage stats` generates and tells the user to copy.

## Goal

- Replace the leaked `sql: no rows in result set` with a message that says which
  of the two possible causes occurred and what to run next.
- Keep the change PATCH-safe under
  [the release versioning contract](release-versioning-contract.md): no new
  typed error code, no exit-code change, no schema change, no successful-output
  change.

## Non-Goals

- **No automatic index synchronization.** Making `session show` scan or repair
  `sessions.sqlite3` on a failed lookup would add behavior to a read command and
  is out of scope for a patch release; it is recorded in Future Ideas below.
- No resolution of activity detail through core usage state. The Backlog entry
  listed that as one of three candidate designs; this plan chooses the third.
- No new typed error code. `session_index_stale` and `session_not_found` would
  be MINOR triggers; see Future Ideas.
- No change to `session show`'s successful text, JSON, pagination, or activity
  privacy boundary.
- No change to `usage stats`, including the command string it emits.

## Evidence Baseline

Gathered on 2026-08-02 at `308feb0`.

Reproduced on `v0.2.0-rc.2`: the selected Claude session, its usage events, safe
tool calls, and source ownership were present in `agentdeck.sqlite3`, while
`sessions.sqlite3` held no metadata, documents, or source row for that session
because its last scan predated the source log.

Two code paths reach the same failure:

| Path | Site | Trigger |
| --- | --- | --- |
| Explicit `--client` | `internal/session/session.go:963` | The `session_metadata`/`session_sources` join returns no row, and `ShowWithActivity` returns `sql.ErrNoRows` unwrapped |
| Inferred client | `cmd/agentdeck/main.go:2008-2010` | `session.List` yields no matching session ID, and the handler returns `sql.ErrNoRows` directly |

The command that `usage stats` generates carries `--client`, so the first path is
the one users actually hit.

`sql.ErrNoRows` matches no case in `errorCode` (`cmd/agentdeck/main.go:288-318`)
and falls to `default`, producing `runtime_error`; `errorExitCode`
(line 321) leaves it at 1. The JSON error envelope is built at line 253 from
`errorCode(err)` and `err.Error()`, so today the `message` field carries the
driver string verbatim.

The specification's "Output and Errors" section pins **typed error codes** and
exit codes through stable fixtures; it does not pin error message text. That is
what makes a message-only correction a patch-level change.

## Decision

`session show` classifies a missing session before reporting it.

1. **Stale index.** The session ID (and client, when given) exists in the core
   usage database's `usage_sessions`. Report that the session index has not been
   scanned since this session was written, and name `agentdeck session scan` as
   the recovery command.
2. **Genuinely absent.** The session is in neither database. Report that no such
   session is known, without SQL text.
3. **Classification unavailable.** The core database cannot be consulted. Report
   case 2's message. A failed hint lookup must never replace or mask the original
   error, and must never change the exit code.

Three constraints on the classification lookup, all of them safety rather than
style:

- It is **read-only and does not create the core database**. It must not open
  the core database in a mode that creates or migrates it, and must not run when
  the file is absent. The ordinary read-only SQLite connection may materialize
  `agentdeck.sqlite3-wal` and `agentdeck.sqlite3-shm` with mode `0600` inside
  the `0700` state root. This is required to observe current WAL-backed usage
  rows accurately; the lookup does not change committed database contents.
- It is **bounded**. One indexed lookup, no scan of usage events, no pricing.
- It **emits no new data**. The message may name the client and the session ID
  the user already supplied, and nothing else — no source path, project, model,
  timestamp, or session content.

Both paths in the Evidence Baseline route through this classification, so the
inferred-client path reports the same thing as the explicit one.

The error code stays `runtime_error` and the exit code stays 1. Scripts that
branch on the typed code are unaffected; only the human-readable message and the
envelope's `message` field change.

## Tasks

### 1. `stale-index-message`

Implement the Decision.

- Introduce the classification at the CLI boundary, where both failure paths
  already converge on returning an error, rather than inside
  `internal/session`: the session package has no access to the core database and
  should not gain one for a message.
- Wrap, do not replace: the returned error must still satisfy
  `errors.Is(err, sql.ErrNoRows)` so any existing caller or test that checks for
  it keeps working, while `Error()` returns the new text.
- Cover both the explicit-`--client` and inferred-client paths.
- If any shipped fixture pins the current driver message, updating that fixture
  is in scope; a fixture that pins the `code` must remain unchanged and must
  still pass.

Acceptance:

- With a session present in `usage_sessions` and absent from the session index,
  both paths report the stale-index message naming `agentdeck session scan`,
  with code `runtime_error` and exit code 1.
- With a session absent from both databases, both paths report the not-found
  message, with the same code and exit code.
- With the core database file absent, the not-found message is reported and no
  core database file is created.
- No output contains `sql: no rows in result set`, a source path, a project, a
  model, a timestamp, or any session text.
- `session show` for an existing session is byte-identical to `v0.2.1` in text
  and JSON.
- The stable error-code fixtures pass unchanged.

Verification: L2. Targeted `cmd/agentdeck` tests covering the three
classification outcomes and both paths, then `go test -mod=vendor ./...`,
because the change touches the shared CLI error surface.

Development evidence (2026-08-03):

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run=TestSessionShow`
  -> PASS (`1.263s`; final run after Round 2 WAL correction).
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` -> PASS
  (final run after Round 2 WAL correction).
- `git diff --check` -> PASS.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `stale-index-message` | [x] | [x] |

Independent of every other `v0.2.2` plan.

## Future Ideas

Not scoped here; each needs its own plan and a MINOR release.

- Dedicated typed error codes (`session_index_stale`, `session_not_found`) so
  scripts can branch on the cause rather than on message text.
- Having `session show --activity` resolve activity detail through core usage
  state when the session index is behind, which would make the copied command
  succeed instead of explaining why it cannot.
- Having `usage stats` verify or refresh the session index before emitting a
  copyable command.

## Starting a task

> 进入开发：`session-show-stale-index` / `stale-index-message`

Read `AGENTS.md`, this plan's Evidence Baseline and Decision,
`cmd/agentdeck/main.go:1990-2015` and `:288-325`,
`internal/session/session.go:957-988`, and the L2 verification route in
`AGENTS.md`. Tick `Dev` after the targeted tests and the full vendored suite
pass; an independent reviewer records a PASS round under
`docs/reviews/session-show-stale-index/stale-index-message.md` before ticking
`Review`.
