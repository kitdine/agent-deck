---
status: historical
plan: terminal-presentation-remediation
task: session-interactive-responsive-layout
retired: 2026-08-12
---

# Review: Session Interactive Responsive Layout

## Review invalidation — 2026-08-12

All Task 4 conclusions from any earlier workspace or conversation are audit
history only. This record starts a new independent review of the complete
candidate migrated onto the frozen Task 1-3 branch.

## 📋 Full Reset Review R1 — 2026-08-12

📊 Overall score: 7/10

✅ Verdict: FAIL

Candidate identity: base commit
`94d2c2bd36dc662a2b5a3aef91b62e05f4550c9e` plus the complete nine-path
Task 4 payload. Migrated diff SHA-256:
`d67f9e308b8a19daefae0ebed32cea1f9c3a4a244c7e70207164b2e27e89279d`.

### 🔴 Serious issues must fix

None. Fresh Session viewer, browser, and current PTY baselines pass.

### 🟡 Suggested improvements recommended

1. **P2 non-blocking — nested/direct Session geometry coverage omits the
   authoritative wide and tall fixtures.**
   [`cmd/agentdeck/session_viewer_test.go:366`]

   - Behavior risk: the viewer can regress at the 120-column split threshold,
     at 180-column bounded canvas, or in tall dynamic acquisition without the
     Task 4 gate failing.
   - Evidence: the terminal matrix requires 48×10, 60×18, 80×24, 100×24,
     120×32, 140×32, and 180×40. The viewer test exercises 48×10, 60×12,
     80×24, 120×24, and 140×32. Root browser coverage is broader, but cannot
     prove nested/direct Detail budgeting.

💡 Bounded improvement: replace the incidental sizes with the exact required
matrix and keep visible-cell width and total-frame-height assertions at every
fixture. Assert the 180-column visual content remains inside the 120-cell
Session canvas.

2. **P2 non-blocking — PTY resize test does not observe the rendered frames or
   selected identity.**
   [`cmd/agentdeck/session_viewer_pty_darwin_test.go:80`]

   - Behavior risk: resize may reload the correct acquisition limit while
     failing to redraw, hiding the selected row, or leaving a stale frame; the
     current L3 test would remain green.
   - Evidence: output is drained to `io.Discard`. The test only reads loader
     limit values `14 → 36 → 20`, despite its name claiming identity is kept.

💡 Bounded improvement: observe ordered 60×18, 180×40, and 80×24 frames from
the PTY; require the stable selected identity remains visible after each
resize, while retaining exact one-load-per-geometry assertions and the existing
two-second upper bounds. Do not add readiness sleeps or increase timeouts.

### 🟢 Strengths

- The migrated payload exactly matches the frozen read-only source fingerprint
  and does not overwrite the shared Detail renderer or Task 1-3 documents.
- Root Session browser caps its canvas at 120 cells, uses two complete compact
  record lines below 80 columns, and has direct 180-column left-alignment
  assertions.
- Session state uses geometry-derived acquisition limits, stable identities,
  independent section page/selection/viewport state, and bounded reflow.
- Existing unit and PTY baselines cover structured semantic Detail, empty-card
  height reclaim, standalone Escape, q, Ctrl-C, EOF, cancellation, raw-mode and
  alternate-screen restoration, nested browser return, and state-lock release.

### 📝 Summary

This is a complete independent review of the migrated Task 4 candidate across
root browser, nested/direct viewer, structured Overview/Documents/Activity/
Tokens data, dynamic acquisition, responsive rendering, resize, and terminal
lifecycle. No earlier verdict is reused. Two non-blocking but required L3
coverage gaps remain, so Task 4 is not verified.

Repair: complete the exact nested/direct geometry matrix and make resize-frame
identity observable without sleeps or larger timeouts.

## 📋 Full Reset Re-review R2 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

Final Task 4 diff SHA-256:
`9adeeb67495822adec26c18230ccea4c330a51ef2769f14ad5ecb3c11f7e1567`.

### 🔴 Serious issues must fix

None.

### 🟡 Suggested improvements recommended

None.

### 🟢 Strengths

- Full Reset Review R1 geometry finding closed: nested/direct Session viewer
  now exercises the exact 48×10, 60×18, 80×24, 100×24, 120×32, 140×32,
  and 180×40 matrix. Every frame remains within its terminal width and height;
  180-column visual content remains within the 120-cell Session canvas.
- Full Reset Review R1 PTY finding closed: ordered 60×18, 180×40, and 80×24
  frames now prove loader limits `14 → 36 → 20` and keep `stable-06` visibly
  selected after both resizes. The test retains the existing two-second upper
  bound, adds no readiness sleep, and rejects extra reloads.
- Root browser stays left aligned at 180 columns and uses two complete compact
  record lines from 48 through 79 columns. Taller terminals display more
  complete records; shorter terminals display fewer without a fixed 20-row
  viewport.
- Overview, Documents, Activity, and Tokens producers use structured fields,
  notes, priorities, and semantic roles. Optional/redundant content disappears;
  partial, failed, incomplete, unpriced, warning, and safe-metadata states remain
  explicit words as well as color.
- A synthetic frame review at 60×18, 80×24, 120×32, and 180×40 confirmed
  readable list/Detail hierarchy, bounded left alignment, explicit Activity
  status, and no empty Detail region. The temporary harness was removed and is
  not used as lifecycle proof.
- Browser Enter → detail → Escape → browser, direct detail, standalone Escape,
  q, Ctrl-C, EOF, cancellation, resize, write/load failure, raw-mode restore,
  cursor/alternate-screen cleanup, input release, and state-lock release retain
  fresh unit or PTY evidence.
- Ordinary `session show --activity` remains covered independently: responsive
  text rendering, Activity lines, and safe-metadata privacy tests all pass after
  the Task 4 candidate.

### 📝 Summary

The complete repaired Task 4 candidate was independently re-reviewed across
root and direct entry points, bounded browser layout, dynamic height and lazy
acquisition, section-local state, structured Detail semantics and color,
short/wide resize, privacy, and complete terminal lifecycle. Both R1 findings
are closed. No blocking, non-blocking, P0-P3, or nit finding remains, and no
earlier review verdict is reused.

Fresh evidence on the final product/test content:

- PASS: Session viewer/data/browser targeted unit suites
- PASS: Session viewer and root-browser Darwin PTY suites
- PASS: Session viewer and root-browser PTY suites with `-race`
- PASS: `TestRenderSessionShowText`
- PASS: `TestSessionShowActivityLines`
- PASS: `TestSessionShowActivityReadsOnlySafeMetadata`
- PASS: Task 4 scoped `git diff --check`
- PASS: safety-source payload remains byte-frozen at
  `d67f9e308b8a19daefae0ebed32cea1f9c3a4a244c7e70207164b2e27e89279d`

Task checkpoint: `session-interactive-responsive-layout`, pending exact
staged-tree CEv1 binding and atomic commit.

Commit recommendation: include only the nine Session implementation/test paths,
this independent review record, and Task 4 plan/index status hunks.

Push recommendation: defer until Task 5 and final exact-tree plan closure pass;
no intermediate Task 4 push.

Implement: `terminal-contract-and-acceptance` after Task 4's exact tree is CEv1
VERIFIED and committed.

## Post-PASS Full Reset Review R3 — 2026-08-12

📊 Overall score: 8/10

✅ Verdict: FAIL

Trigger: Task 5's first final L4 `release-verify` attempt failed in
`make test`. A direct reproduction showed `./cmd/agentdeck` timing out after
601.624 seconds. A two-minute diagnostic timeout identified
`TestRunSessionViewerPTYExitResizeAndRestore` as the active test.

### 🔴 Serious issues — must fix

1. **P1 — Session PTY lifecycle tests can turn an immediate startup error into
   a ten-minute package hang.**

   [`cmd/agentdeck/session_viewer_pty_darwin_test.go`]

   - Root cause: the execution environment has `TERM=dumb`; the affected tests
     did not isolate TERM. `runSessionViewer` therefore correctly returned
     `--interactive requires a usable terminal` before calling the loader.
     The test waited unconditionally on `<-ready` and never observed the
     buffered `done` error, so the fast error surfaced only as Go's default
     ten-minute timeout.
   - Scope: `TestRunSessionViewerPTYExitResizeAndRestore` and
     `TestRunSessionViewerPTYCancellationRestoresTerminal` shared the unbounded
     ready wait. Other PTY tests in the same file already set
     `TERM=xterm-256color` explicitly.
   - Product disposition: no product hang was observed. The viewer exited
     before its loader and no product viewer goroutine remained in the timeout
     stack. The contract requires rejecting `TERM=dumb`, so weakening that
     product check would be incorrect.
   - Required repair: isolate TERM in both tests and replace the unbounded
     receive with a bounded wait that reports loader readiness, early viewer
     exit, or a two-second readiness timeout.

### 🟡 Suggested improvements — recommended

None beyond the required P1 repair.

### 📝 Summary

The earlier R2 PASS is invalidated for Task completion by a new decisive L4
finding. Task 4 is reopened; its prior review remains audit history only. Task 5
and Plan closure are paused until repair, complete re-review, commit-bound CEv1,
and Task 5 rebind all succeed.

## Post-PASS Full Reset Re-review R4 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🔍 R3 finding disposition

- Both affected tests now set `TERM=xterm-256color` within `t.Setenv`, so they
  remain hermetic even when the invoking environment is `TERM=dumb`.
- `waitForSessionViewerPTYReady` observes loader readiness, early viewer
  return, and a two-second timeout. A future startup regression reports the
  actual error immediately instead of hanging the package.
- No production code changed; the valid unusable-terminal rejection remains
  intact.

### 🧪 Fresh repair evidence

- PASS with external `TERM=dumb`:
  `TestRunSessionViewerPTYExitResizeAndRestore` (1.734s).
- PASS with external `TERM=dumb`:
  `TestRunSessionViewerPTYCancellationRestoresTerminal` (1.568s).
- PASS with external `TERM=dumb`: full `./cmd/agentdeck` package (70.879s),
  closing the original failing component.
- PASS with external `TERM=dumb`: all `TestRunSessionViewerPTY*` under `-race`
  (3.730s).
- PASS: `gofmt` and Task 4 scoped `git diff --check`.

### 📝 Summary

The complete Task 4 surface has been independently re-reviewed after the L4
finding. The repaired PTY harness preserves all responsive layout, resize,
identity, terminal lifecycle, privacy, and error contracts from R2 while making
startup failures deterministic and bounded. No blocking, non-blocking, P0-P3,
or nit finding remains. Task 4 Review PASS is restored pending an atomic signed
repair commit and commit-bound CEv1 verification.
