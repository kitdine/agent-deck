---
status: historical
plan: credential-and-pricing-hardening
task: session-health-doc-accuracy
retired: 2026-08-03
---

# Review log — credential-and-pricing-hardening / session-health-doc-accuracy

## Round 1 — 2026-08-03

- Reviewed state: worktree on `20ef02cb7f5783c508195a703e568605aea5a839`;
  `internal/session/doctor.go` blob
  `c7be9b7f155cfd5bb12110a9119a716593dd85eb` and
  `internal/session/doctor_test.go` blob
  `db628154f8103cafe172c0d2e155c86d989f86df` (uncommitted), plus the plan's
  development-evidence and Status edits.
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: the rewritten `CheckHealth` doc comment, the rewritten sidecar-pin
  comment in `TestCheckHealthCompatibleIndexReportsOKWithoutMutatingTheDatabaseFile`,
  their agreement with the observed `mode=ro` WAL behavior, the absence of any
  production behavior change, and the task's L0 verification contract.
- Findings:
  - **P1 — `internal/session/doctor.go:22` misstates `immutable=1`.** The comment
    says `immutable=1` "would reject concurrent watcher or scanner writes".
    `immutable=1` is a read-side assertion: SQLite disables locking and change
    detection and assumes the file cannot change, so it rejects nothing. When the
    file does change concurrently the documented consequence is incorrect query
    results or `SQLITE_CORRUPT` — the spurious integrity failure the plan
    (`docs/plans/credential-and-pricing-hardening.md:132-134`) records as the
    rejection reason. The task's whole purpose is to stop the comment from
    asserting something untrue, so replacing one inaccurate claim with another
    fails the acceptance criterion "the comment and the shipped test agree with
    observed behavior".
  - P2 — none.
  - Nits (non-blocking):
    - `doctor.go:23`: "nolock=1 could return a stale snapshot" matches the plan's
      wording but understates SQLite's own warning about concurrent writers.
    - `doctor.go:21-23`: the colon construction leaves the reader to infer that
      the two DSN parameters are *rejected alternatives*; the plan asked the
      comment to "note why the two alternatives were rejected".
    - `doctor_test.go:86`: the failure message still reads "expected observed
      %s sidecar"; "observed" is leftover phrasing from the old
      records-unintended-behavior framing the comment above now drops.
    - Out of scope for this task: `internal/store/store.go:151` carries the same
      "without creating state" shape for the core database. Its "no enabling WAL"
      claim is accurate, but an already-WAL core database would materialize
      sidecars there too. Worth a backlog entry, not a change here.
- Evidence:
  - Full-context source and diff review against the task contract -> P1 found.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/session`
    -> PASS (`1.090s`).
  - `git diff --check` -> PASS.
  - No production behavior change confirmed by inspection: only comment lines
    differ in `doctor.go`; the sidecar pin, its `0600` mode assertions, and the
    main-database digest assertion in `doctor_test.go` are unchanged.
- Verdict: REOPEN (one P1 accuracy defect; nits non-blocking)

## Round 2 — 2026-08-03 (re-review)

- Reviewed state: worktree on `20ef02cb7f5783c508195a703e568605aea5a839`;
  `internal/session/doctor.go` blob
  `4c5db594d7a2233298f9e591d5c75ef003f453b8` (changed since Round 1) and
  `internal/session/doctor_test.go` blob
  `db628154f8103cafe172c0d2e155c86d989f86df` (unchanged since Round 1).
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: closure of the Round 1 P1 finding, absence of new findings, and
  confirmation that the task still changes no production behavior.
- Finding closure:
  - **P1 (`immutable=1` misstated) — CLOSED.** `doctor.go:22-23` now reads
    "immutable=1 assumes the file cannot change and can yield incorrect results
    or SQLITE_CORRUPT during concurrent watcher or scanner writes", which matches
    SQLite's documented semantics and the plan's rejection reason at
    `docs/plans/credential-and-pricing-hardening.md:132-134`. The comment no
    longer claims the parameter rejects writes.
  - Round 1 nits were not addressed and remain non-blocking: the `nolock=1`
    wording still tracks the plan rather than SQLite's stronger warning; the
    rejected-alternative framing is still implied by punctuation; and
    `doctor_test.go:86` still says "expected observed %s sidecar". The
    `internal/store/store.go:151` observation stays out of this task's scope.
- No new findings. The `doctor.go` diff remains comment-only (6 insertions,
  2 deletions, all inside the `CheckHealth` doc comment); `mode=ro`, the
  `full`/non-`full` branches, and every assertion in `doctor_test.go` are
  untouched.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/session`
    -> PASS (`0.934s`, rerun against the corrected blob).
  - `git diff --check` -> PASS.
  - `git diff -U8 -- internal/session/doctor.go` -> comment-only change confirmed.
- Verdict: PASS
