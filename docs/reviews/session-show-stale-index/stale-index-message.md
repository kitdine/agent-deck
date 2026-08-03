---
status: active
plan: session-show-stale-index
task: stale-index-message
---

# Review log — session-show-stale-index / stale-index-message

## Round 1 — 2026-08-03

- Reviewed state: worktree on `e47ae9b1a08f1bc32bed5c83f961cd08e710eaa5`;
  `cmd/agentdeck/main.go` blob `c4963b2341793fc6245720455d710e998e78dc3c` and
  `cmd/agentdeck/main_test.go` blob
  `bd5f9431f290075b8817746152506b3c4310df48` (uncommitted), plus the plan's
  development-evidence and Status edits.
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: the `sessionShowNotFoundError` wrapper, `sessionShowNotFound`
  classification, both `session show` failure paths, the typed-code and
  exit-code surface, message content against the Decision's disclosure limits,
  the new regression tests, and the task's L2 verification contract.
- Findings:
  - **P2 — the classification lookup materializes WAL sidecars for the core
    database, which the Decision forbids in its own words.**
    `sessionShowNotFound` (`cmd/agentdeck/main.go:340-362`) calls
    `store.OpenReadOnly`, whose DSN is `mode=ro`
    (`internal/store/store.go:155`). The core database is created in WAL mode
    (`internal/store/store.go:37`), and opening a WAL database read-only
    materializes `-wal`/`-shm` sidecars — the behavior
    `internal/session/doctor_test.go:79-91` already pins for `sessions.sqlite3`
    under the same driver and the same DSN mode. The Decision at
    `docs/plans/session-show-stale-index.md:83-85` says the lookup "is
    read-only and non-creating" and that "a diagnostic message may not have the
    side effect of materializing state". Nothing here is a privacy or integrity
    problem — the sidecars are `0600` inside the `0700` state root and the
    committed bytes are untouched — but a failed `session show` now writes files
    into the state root, and neither the plan nor a test records it.
    Two acceptable resolutions: state the sidecar effect in the Decision (and
    ideally in a comment at the call site) the way `CheckHealth` now does, or
    pin it with an assertion so it cannot regress silently. Note the evidence
    limit: the sidecar creation is established for `sessions.sqlite3` by an
    existing test and inferred for `agentdeck.sqlite3` from the identical
    driver, DSN mode, and journal mode; no test in this task exercises it.
  - Nits (non-blocking):
    - `cmd/agentdeck/main.go:342-345`: the Decision also says the lookup "must
      not run when the file is absent", but the code calls `OpenReadOnly`
      unconditionally and relies on it failing. The observable behavior is
      correct and `TestSessionShowMissingCoreDoesNotCreateIt` pins it; an
      `os.Stat` guard would match the written contract and skip a pointless
      open.
    - `cmd/agentdeck/main.go:2047-2049`: the `errors.Is(err, sql.ErrNoRows)`
      test sits after `ShowWithActivity`, which today can only produce that
      error from its metadata lookup (`internal/session/session.go:963`). If
      `activity.ReadDetails` ever returns an error wrapping `sql.ErrNoRows`, a
      present session would be reported as missing.
    - `cmd/agentdeck/main.go:243-252`: `sessionShowNotFoundError.Unwrap` returns
      the bare `sql.ErrNoRows` rather than the original error. Equivalent today
      because the original *is* `sql.ErrNoRows`, but it drops the chain.
- Acceptance criteria: all six are met. Both paths classify (`main.go:2044` and
  `:2047`), the stale message names `agentdeck session scan`, the absent
  message carries no SQL text, `errorCode` still resolves `runtime_error`
  (no `sql.ErrNoRows` case exists at `main.go:300-330`) and `errorExitCode`
  still returns 1, the messages disclose only the session ID the user supplied,
  the successful path is untouched, and no shipped fixture pinned the old driver
  text.
- Evidence:
  - Full-context source and diff review against the Decision and acceptance
    list -> one P2.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./cmd/agentdeck -run TestSessionShow`
    -> PASS (`1.242s`).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
    -> PASS.
  - `git diff --check` -> PASS.
- Verdict: REOPEN (one P2; nits non-blocking)

## Round 2 — 2026-08-03 (re-review)

- Reviewed state: worktree on `e47ae9b1a08f1bc32bed5c83f961cd08e710eaa5`;
  `cmd/agentdeck/main.go` blob `c62543cabe5e37755f6dd24c7bb3db8043356c37`,
  `cmd/agentdeck/main_test.go` blob
  `09ec07ea95c9d381af2b3e4e984d88b426f3ab39`, and the newly touched
  `internal/store/store.go` blob
  `10f1dcea5f48a6bfd95f72c25a30ef396b80da3a` (all uncommitted).
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: closure of the Round 1 P2, the new `OpenReadOnlyImmutable` seam, its
  single call site, the added sidecar assertions, and whether the fix introduced
  new failure modes.
- Finding closure:
  - **P2 (WAL sidecars materialized by the classification lookup) — closed as
    observable behavior, but by a means that introduces a P1.** The lookup now
    calls `store.OpenReadOnlyImmutable` (`cmd/agentdeck/main.go:344`), and
    `TestSessionShowClassifiesMissingIndexEntries` asserts that neither
    `agentdeck.sqlite3-wal` nor `-shm` exists after the command. The sidecars
    are indeed gone.
  - Round 1 nits: the `os.Stat` guard nit is now moot in a different way — the
    open still happens unconditionally, but no longer writes. The
    `ShowWithActivity` ordering nit and the `Unwrap` chain nit are unchanged and
    remain non-blocking.
- New findings:
  - **P1 — `immutable=1` silently reads a stale snapshot, defeating the very
    classification this task adds.** `openReadOnly(..., immutable=true)`
    (`internal/store/store.go:164-172`) tells SQLite the file cannot change, so
    it skips locking *and the WAL*. Whenever another process holds the core
    database open with unmerged commits — the watcher, a concurrent
    `usage import`, or a leftover `-wal` from an unclean exit — the lookup reads
    the main file only.
    Measured directly, with a writer connection held open after a committed
    insert: `file:core.sqlite3?mode=ro` returns `exists=1`, while
    `file:core.sqlite3?mode=ro&immutable=1` fails with
    `no such table: usage_sessions`. So a session that *is* in `usage_sessions`
    gets reported as "no session is known" — the exact misreport this task
    exists to remove — and it degrades silently, because a failed lookup falls
    through to the not-found branch by design (Decision case 3).
    This is also the option the repository rejected three commits ago:
    `internal/session/doctor.go:22-23` now records that `immutable=1` "assumes
    the file cannot change and can yield incorrect results or SQLITE_CORRUPT
    during concurrent watcher or scanner writes".
    The new test passes because `core.Close()` checkpoints and removes the
    `-wal` before the command runs, so the stale-snapshot window is never
    exercised.
  - **P1 (same defect, contract view) — the documented precondition is violated
    at the only call site.** `internal/store/store.go:157-158` states "Callers
    must hold the state lock while it is open." `sessionShowNotFound` runs
    inside `withSessions`, which opens the session index, not the core state
    lock, and takes no core lock of its own.
- Recommendation: revert to the Round 1 options — record the sidecar effect in
  the Decision (and a comment at the call site) the way `CheckHealth` does, or
  pin it with an assertion — and keep `OpenReadOnly`. Eliminating the sidecar is
  not worth a lookup that can be wrong exactly when the index is behind, which
  is the only situation this feature addresses. If `OpenReadOnlyImmutable` is
  kept for other callers, it needs a caller that actually holds the state lock.
- Evidence:
  - Full-context source and diff review of the three changed files -> two P1
    findings.
  - Concurrent-writer experiment (`python3` + system SQLite, scratch directory):
    plain `mode=ro` sees the committed row; `mode=ro&immutable=1` cannot see the
    table at all.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./cmd/agentdeck -run TestSessionShow`
    -> PASS (`0.936s`).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
    -> PASS.
  - `git diff --check` -> PASS.
- Verdict: REOPEN (one defect, recorded from two angles; nits unchanged)

## Round 3 — 2026-08-03 (re-review)

- Reviewed state: worktree on `e47ae9b1a08f1bc32bed5c83f961cd08e710eaa5`;
  `cmd/agentdeck/main.go` blob `53723f2aea4d69719419052bac480e7685175b02`,
  `cmd/agentdeck/main_test.go` blob
  `2c7f56b2d5c4798c3cb3c866ac0ea1c9581ab481`, and
  `docs/plans/session-show-stale-index.md` blob
  `11d9a640ca9334bd0e32c0588ad5d180acd7e8ad` (uncommitted).
  `internal/store/store.go` is byte-identical to `HEAD` again.
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: closure of both Round 2 P1 angles, closure of the Round 1 P2 by the
  route this log recommended, and whether the reverted seam left anything
  behind.
- Finding closure:
  - **P1 (stale snapshot from `immutable=1`) — CLOSED.** The call site is back
    to `store.OpenReadOnly` (`cmd/agentdeck/main.go:343`), so the lookup reads
    through the WAL again. `TestSessionShowClassifiesSessionFromLiveCoreWAL`
    (`cmd/agentdeck/main_test.go:2061-2077`) holds the core writer open with
    `defer core.Close()`, inserts a `usage_sessions` row that therefore stays in
    an unmerged `-wal`, and asserts `session show` still reports the
    stale-index message. That is exactly the window Round 2 measured; under
    `immutable=1` this test fails.
  - **P1 (precondition violated at the only call site) — CLOSED.**
    `OpenReadOnlyImmutable` and the `immutable` parameter are gone;
    `internal/store/store.go` matches `HEAD` byte for byte, so no unlocked
    caller and no unused seam remain.
  - **P2 (undocumented sidecar materialization) — CLOSED.** The Decision
    (`docs/plans/session-show-stale-index.md:83-88`) now states that the
    read-only connection may materialize `agentdeck.sqlite3-wal` and `-shm` at
    mode `0600` inside the `0700` state root, and says why that is required. The
    call site carries the same reasoning in a comment
    (`cmd/agentdeck/main.go:340-342`), and
    `TestSessionShowClassifiesMissingIndexEntries` now pins both the presence of
    the sidecars and their `0600` mode across all four path/outcome
    combinations.
- Remaining nits (unchanged, non-blocking):
  - The Decision still says the lookup "must not run when the file is absent"
    while the code calls `OpenReadOnly` unconditionally and relies on it
    failing. Behavior is correct and pinned by
    `TestSessionShowMissingCoreDoesNotCreateIt`.
  - `errors.Is(err, sql.ErrNoRows)` still sits after `ShowWithActivity`.
  - `sessionShowNotFoundError.Unwrap` still returns the bare sentinel.
  - New, minor: the sidecar assertions now *require* the sidecars to exist, so a
    future SQLite or driver that stops creating them would fail the test for a
    harmless reason. This matches the existing convention in
    `internal/session/doctor_test.go:79-91`, so it is left as is.
- No new findings. All six acceptance criteria still hold; the classification
  logic, messages, typed code, and exit code are unchanged from Round 1's
  assessment.
- Evidence:
  - Full-context source and diff review of the three changed files, plus
    confirmation that `internal/store/store.go` carries no diff -> no findings.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./cmd/agentdeck -run TestSessionShow`
    -> PASS (`0.853s`).
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
    -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./cmd/agentdeck ./internal/store`
    -> PASS.
  - `git diff --check` -> PASS.
- Verdict: PASS
