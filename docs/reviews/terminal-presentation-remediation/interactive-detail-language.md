---
status: active
plan: terminal-presentation-remediation
task: interactive-detail-language
---

# Review: Interactive Detail Language

## Scope

- `cmd/agentdeck/terminal_detail.go`
- `cmd/agentdeck/terminal_detail_test.go`
- compatibility call paths in the uncommitted Usage and Session producer drafts
- task-local plan and documentation status

## Development evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestRenderTerminalDetail -count=1`
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies.

## Round 1 — 2026-08-11

Verdict: FAIL

### Finding

1. **P1 blocking — unbroken long values and notes are truncated rather than wrapped losslessly.** The structured renderer delegates plain values to `statsWrap`. A single CJK/emoji or ASCII token wider than the available cell remains one over-wide line, then `statsFit` replaces its tail with an ellipsis. This violates the approved long-note/full-width contract and can discard approved visible text. Current tests assert line width but not content preservation, so they do not expose the loss.

No other blocking, non-blocking, P0-P3, or nit findings were identified in this round. Broad verification stopped after the decisive reproducer.

### Required repair

- Add a terminal-cell-aware, word-friendly hard wrapper for sanitized Detail labels, values, and notes.
- Prove unbroken ASCII, CJK, emoji, and combining-mark content survives wrapping without raw controls or geometry overflow.
- Compare ANSI-stripped color output with no-color output to protect semantic label/value/order equivalence.

## Repair after Round 1 — 2026-08-11

- Added a word-friendly, rune/cell-aware hard wrapper for sanitized Detail labels, values, and notes; visible content is never replaced by an ellipsis at supported interactive widths.
- Added long unbroken ASCII, CJK, emoji, and combining-mark conservation assertions.
- Required ANSI-stripped color output to equal no-color output exactly, protecting labels, values, order, notes, and status words.

## Round 2 — 2026-08-11

Verdict: PASS

### Finding closure

1. **P1 closed.** Long fields and notes now wrap losslessly by terminal cells. Tests prove content conservation and width bounds at narrow and wide geometries.

### Final evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestRenderTerminalDetail -count=1`
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies.

No open blocking, non-blocking, P0-P3, or nit findings remain. Producer migration and producer-specific PTY coverage remain owned by Tasks 3 and 4.

## Review reset — 2026-08-12

All earlier PASS and test conclusions above are historical audit data only and
do not satisfy the reset review gate. The final shared-renderer compatibility
bridge removal was classified as Task 2 scope before this reset round.

## Reset Round R1 — 2026-08-12

Verdict: FAIL

### Finding

1. **P2 non-blocking — the required geometry matrix is incomplete.** The
   renderer test exercises only 48- and 80-column frames. It does not prove the
   frozen shared Detail contract at 60, 120, and 180 columns, so regressions in
   the single/two-column threshold or wide layouts could pass unnoticed.

No other blocking, non-blocking, P0-P3, or nit findings were identified in this
round.

### Required repair

- Exercise 48/60/80/120/180 columns with the same hostile structured model.
- At every width, prove visible-cell bounds, semantic content conservation, and
  exact ANSI-stripped/color versus no-color frame equivalence.

## Repair after Reset Round R1 — 2026-08-12

- Added a five-width geometry matrix using long ASCII, CJK, emoji, and combining
  marks across explicit Session, token, cost, and warning roles.
- Required exact color/no-color semantic frame equality and content conservation
  at every width.

## Reset Round R2 — 2026-08-12

Verdict: FAIL

### Finding

1. **P2 non-blocking — the geometry repair does not assert the selected layout.**
   The five-width matrix proves line bounds and content conservation, but it
   would still pass if 48/60 columns incorrectly switched to two columns or if
   80/120/180 columns regressed to a single column. The Reset R1 threshold and
   wide-layout risk therefore remains incompletely protected.

No other blocking, non-blocking, P0-P3, or nit findings were identified in this
round.

### Required repair

- Assert from rendered output that 48/60-column frames keep Session and token
  fields on separate rows.
- Assert that 80/120/180-column frames place the same two useful fields in one
  two-column row.

## Repair after Reset Round R2 — 2026-08-12

- Added observable single/two-column row assertions to every geometry-matrix
  width without coupling the test to the renderer's internal threshold helper.

## Reset Round R3 — 2026-08-12

Verdict: FAIL

### Finding

1. **P2 non-blocking — absence coverage does not protect required warning and
   semantic-zero retention.** The empty-card test proves blank values consume
   zero lines, but it does not prove that a status-only required warning keeps
   the card visible or that the meaningful string value `0` survives
   normalization. Both are explicit shared Detail contract boundaries.

No other blocking, non-blocking, P0-P3, or nit findings were identified in this
round.

### Required repair

- Render a model containing a semantic zero and a status-only warning.
- Assert both labeled zero and warning status remain visible in no-color output.

## Repair after Reset Round R3 — 2026-08-12

- Extended the absence test to prove `CACHE TOKENS 0` and a status-only
  `UNPRICED` warning remain visible while truly empty models still return no
  Detail lines.

## Reset Round R4 — 2026-08-12

Verdict: FAIL

### Finding

1. **P2 non-blocking — hostile-input coverage omits note and status inputs.**
   Title, label, and value sanitization are asserted, but producer-supplied note
   text and status are not. A regression in either path could therefore expose
   ANSI/control sequences or forged terminal rows without failing this task's
   tests.

No other blocking, non-blocking, P0-P3, or nit findings were identified in this
round.

### Required repair

- Include CSI, newline, and carriage-return input in note text and status.
- Assert controls are absent while the sanitized visible words remain.

## Repair after Reset Round R4 — 2026-08-12

- Extended the hostile-input model and assertion across title, label, value,
  status, and note inputs.

## Reset Round R5 — 2026-08-12

Verdict: PASS

### Finding closure

1. **Reset R1/R2 P2 closed.** The 48/60/80/120/180 matrix now proves visible
   bounds, lossless content, exact color/no-color frames, single-column narrow
   layouts, and two-column wide layouts.
2. **Reset R3 P2 closed.** Empty models consume zero lines while a semantic zero
   and status-only required warning remain visible.
3. **Reset R4 P2 closed.** Hostile title, label, value, status, and note inputs
   are sanitized without losing their visible identifying text.

### Final evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestRenderTerminalDetail -count=1`
- PASS: `git diff --check`
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies.
- Production SHA-256: `c1bee6a1d6b7a2bc5f4722baaf02d8ee9a77a19c7d542ba6aff168ccad22a6b3`.
- Test SHA-256: `cc3baf429b649f6ad868cb83d26d01130f8dc1e2fe88c6575771d316a91e8171`.

No open blocking, non-blocking, P0-P3, or nit findings remain. Task 2 freezes
the shared Detail contract; Usage and Session producer-specific behavior and
PTY coverage remain owned by Tasks 3 and 4.

## Review invalidation — 2026-08-12

The user rejected Reset R1-R5 as an incomplete continuation of prior review
history rather than a new complete review of the entire Task 2 candidate.
Every verdict, finding closure, and test result above remains audit history only
and cannot support Review PASS, Task completion, or CEv1 verification.

Task 2 returns to `NOT_VERIFIED`. The next legal gate is **Full Reset Review
R1**, which must independently inspect the complete production implementation,
complete test protection, every acceptance criterion, and all blocking,
non-blocking, P0-P3, and nit surfaces. It may not reuse the old PASS conclusion
or close a surface merely because an earlier round mentioned it.
