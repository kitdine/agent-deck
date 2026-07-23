---
status: historical
plan: test-coverage
task: session-health
retired: 2026-07-23
---

# Review log — test-coverage / session-health

## Round 1 — 2026-07-23

- Reviewed state: base commit `edb774e` plus the untracked
  `internal/session/doctor_test.go` (136 lines). `internal/session/doctor.go`
  byte-identical to HEAD.
- Reviewer: independent cold-context test-reviewer subagent.
- Scope: task 6 additions for `CheckHealth` — missing index, compatible index
  (non-full), missing-FTS-table, and full-mode unreadable-source counting.
- Findings:
  - [P1] Spec case 2 explicitly required asserting the absence of new
    `-wal`/`-shm` sidecars; the test asserted only main-file mode and digest.
    The reviewer showed — and the orchestrator independently reproduced — that
    `CheckHealth` opening the WAL-mode index with `mode=ro` in fact creates
    both sidecars in the state root and cannot clean them up on close. So the
    contract the spec named is violated by current production code. Committed
    bytes unchanged; sidecars `0600` inside a `0700` root, `mode=ro` genuinely
    enforced (write attempt fails with `attempt to write a readonly database
    (8)`), so no privacy or data boundary breaks.
  - [P2] Case 4's "no leakage" property was structurally guaranteed by
    `Health` having only scalar fields; no assertion would fail if a future
    field carried a source path, despite the test name promising otherwise.
  - Four independent mutations of `doctor.go` (count-all-unreadable,
    never-increment, force-FTS-true, constant integrity query, mkdir-before-stat)
    each produced a targeted failure — cases 1, 3, and 4 are genuinely
    discriminating. Non-finding flagged: `-run TestCheckHealth` silently
    excludes `TestFull...`; use `-run CheckHealth` or the whole package.
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state: uncommitted `internal/session/doctor_test.go` after the
  round-1 repairs.
- **Independence:** the repair was applied by the orchestrator in the same
  session as the review, because both subagents hit a session limit before the
  fix round; the task-6 implementer subagent terminated before applying any
  fix. Recorded so the ticked `Review` is read with that context.
- Maintainer decision on the P1: keep the queue test-only. The sidecar
  assertion now pins the OBSERVED behavior (sidecars present after the check,
  `0600`) with an explanatory comment, and the production question is handed off
  as a backlog item in the plan rather than fixed here.
- Round-1 findings, re-verified:
  - [closed] **Sidecar assertion added.** Case 2 now asserts `-wal` and `-shm`
    exist and are `0600` after a non-full check, retaining the main-file mode
    and digest assertions. RED: pointing `CheckHealth` at an `immutable=1` DSN
    (which suppresses sidecars) fails with `expected observed -wal sidecar
    after CheckHealth`. Restored.
  - [closed] **Leakage guard added.** Case 4 now reflects over every `Health`
    field and fails if any rendered value contains a source path directory or
    basename, reporting only the field name. RED: adding a temporary
    source-path-carrying field to `Health` fails with `Health field
    "ProbeLeak" leaks source path detail`, and the failure message contains no
    path. Restored.
- Evidence at this state: `gofmt -l internal/session` clean,
  `go test -mod=vendor -count=1 ./...` 548 ok across 16 packages,
  `go test -mod=vendor -race ./internal/session` ok,
  `go vet -mod=vendor ./...` clean, `git diff --check` clean.
  `internal/session/doctor.go` byte-identical to HEAD.
- Verdict: **PASS.**
