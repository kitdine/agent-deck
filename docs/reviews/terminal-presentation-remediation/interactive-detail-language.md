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

## Full Reset Review R1 — 2026-08-12

Candidate identity: committed tree
`84b1bc0f092bedb4e088a12aed517218e638d50b`; production SHA-256
`c1bee6a1d6b7a2bc5f4722baaf02d8ee9a77a19c7d542ba6aff168ccad22a6b3`;
test SHA-256
`cc3baf429b649f6ad868cb83d26d01130f8dc1e2fe88c6575771d316a91e8171`.
Task 3 uncommitted files were excluded by testing an archive of the committed
tree.

Verdict: FAIL

### Findings

1. **P1 blocking — compact Detail title loses selected-row identity.**
   `renderTerminalDetailModel` sends the complete `DETAIL · <title>` string to
   `statsFit`, which replaces the tail with an ellipsis at 48 cells. Distinct
   long row identities with a distinguishing suffix can therefore render the
   same title, violating compact identity and selection-meaning preservation.
   A fresh diagnostic rendered
   `DETAIL · provider-xxxxxxxxxxxxxxxxxxxxxxxxxxxxx…` and lost
   `-unique-suffix`.

2. **P2 non-blocking — hard wrapping can split an emoji grapheme.**
   `terminalDetailHardWrapWord` iterates individual runes. A long repeated
   `👩‍💻` value was split between the ZWJ and laptop rune at a line boundary,
   so one visible grapheme no longer survives intact. Existing coverage uses
   single-rune emoji and simple combining marks, but does not protect ZWJ
   clusters.

No additional P0, P3, or nit findings were identified after independently
reviewing the complete structured model, normalization, priority ordering,
single/two-column geometry, empty/semantic-zero handling, role-based palette,
ANSI/control sanitization, no-color equivalence, shared caller boundary, and
existing tests. Cost yellow is the explicit living palette contract and is not
a finding.

### Fresh evidence

- PASS: committed-tree `go test -mod=vendor ./cmd/agentdeck -run
  TestRenderTerminalDetail -count=1`.
- FAIL: independent `TestFullResetReviewLongTitlePreservesSelectionIdentity`.
- FAIL: independent `TestFullResetReviewHardWrapKeepsEmojiGraphemesIntact`.
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored
  dependencies.

### Required repair

- Wrap a long Detail title losslessly across visible-cell-bounded lines while
  retaining the `DETAIL ·` identity prefix.
- Hard-wrap by grapheme clusters using the already vendored Unicode grapheme
  support; do not split ZWJ or combining sequences.
- Add both diagnostics as permanent Task 2 regressions and run a complete fresh
  Task 2 re-review after repair.

## Repair after Full Reset Review R1 — 2026-08-12

- Replaced title truncation with visible-cell-bounded, lossless title wrapping;
  the first line retains `DETAIL ·` and continuations retain every identity
  grapheme.
- Replaced rune-boundary hard wrapping with grapheme-boundary segmentation and
  added permanent long-title and repeated-ZWJ-emoji regressions under the
  plan-prescribed `TestRenderTerminalDetail` gate.

## Full Reset Re-review R1 — 2026-08-12

Verdict: FAIL

### Finding disposition

1. **Full Reset R1 P1 closed.** The 48/60/80/120/180 matrix preserves the
   complete long title, its unique suffix, visible width, and exact
   color/no-color semantic frame.
2. **Full Reset R1 P2 closed.** Repeated `👩‍💻` graphemes remain intact across
   hard-wrap boundaries and within the requested visible width.

### New finding

1. **P2 non-blocking — grapheme repair introduces quadratic wrapping work.**
   The repaired `terminalDetailHardWrapWord` repeatedly calls
   `runewidth.Truncate` on each remaining suffix. `Truncate` first measures the
   complete suffix, so a hostile very long unbroken value is scanned repeatedly
   and can stall an interactive frame. Correct Unicode boundaries must not
   introduce O(n²) rendering behavior.

No other new blocking, P0, P1, P3, or nit findings were identified during the
complete repaired-candidate review.

### Fresh evidence

- PASS: isolated repaired candidate `go test -mod=vendor ./cmd/agentdeck -run
  TestRenderTerminalDetail -count=1`.
- PASS: the standard gate lists and executes all six Task 2 tests, including
  both new regressions.
- PASS: Task 2 scoped `git diff --check`.

### Required repair

- Segment the value in one pass with the already pinned and vendored UAX29
  grapheme iterator.
- Record the existing `uax29/v2` module as a direct dependency because Task 2
  production code imports it; do not upgrade it or alter vendored contents.
- Run fresh targeted verification and another complete Full Reset re-review.

## Repair after Full Reset Re-review R1 — 2026-08-12

- Replaced repeated suffix truncation with one-pass UAX29 grapheme iteration,
  preserving linear work for long unbroken hostile values.
- Promoted the already pinned and vendored `github.com/clipperhouse/uax29/v2`
  module from indirect to direct dependency metadata; version and vendor content
  remain unchanged.

## 📋 Full Reset Re-review R2 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

### 🔴 Serious issues that must be fixed

None.

### 🟡 Suggested improvements

None.

### 🟢 Strengths

- Full Reset Review R1 P1 is closed: long selected-row titles preserve their
  complete distinguishing identity at 48, 60, 80, 120, and 180 cells without
  ellipsis or overflow, and color/no-color frames remain semantically exact.
- Full Reset Review R1 P2 is closed: repeated ZWJ emoji graphemes remain intact
  across hard-wrap boundaries.
- Full Reset Re-review R1 P2 is closed: wrapping now performs one UAX29 pass
  rather than repeatedly scanning suffixes.
- Structured roles still determine color without display-text parsing; empty
  optional fields disappear, semantic zero and status-only warnings remain,
  and empty models consume no Detail rows.

### 📝 Summary

The complete repaired Task 2 candidate was independently re-reviewed across the
entire structured model, normalization and stable priority ordering,
single/two-column geometry, long title and field/note wrapping, CJK/emoji/ZWJ
visible-cell behavior, empty/semantic-zero handling, semantic palette,
ANSI/control sanitization, no-color equivalence, dependency metadata, shared
caller boundary, and all test protection. Every Full Reset finding is closed;
no open blocking, non-blocking, P0-P3, or nit findings remain.

Candidate base: commit `6494cb61625e830797204774e9bdbc0f98cfbbe7`, tree
`84b1bc0f092bedb4e088a12aed517218e638d50b`, plus the reviewed Task 2 repair.
Final repaired content SHA-256:

- `go.mod`: `b702f0c0cd98a623e9591401d7c69e121912287c2d11691c33865dbfa801dc4c`
- production: `403163d5267febe023130d8b51c1ac778b7605b342f9b34cd46cc0070c922cae`
- tests: `5cf663b678d75e471e5a761a5a23fe1ccdb15fa5ccd8bcf3dc69b70699d11ec9`

Fresh evidence from a committed-tree archive with only the reviewed Task 2
repair overlaid:

- PASS: `go test -mod=vendor ./cmd/agentdeck -run
  TestRenderTerminalDetail -count=1`.
- PASS: the standard gate lists all six Task 2 tests, including both new
  regressions.
- PASS: `go list -mod=vendor ./cmd/agentdeck`.
- PASS: Task 2 scoped `git diff --check`.
- PASS: `vendor/modules.txt` and vendored UAX29 content are unchanged; the
  pinned version remains `v2.2.0`.
- The diagnostic `go list -m all -mod=vendor` was rejected by Go because module
  enumeration is unsupported in vendor mode; it is not treated as evidence or
  a product failure.

Task checkpoint: `interactive-detail-language` is Review PASS; its CEv1 gate
must be rebound to the exact staged repair tree before commit. Commit
recommendation: one Task 2 repair/review/status commit containing only the
shared Detail repair, regressions, direct-dependency classification, review
record, and task-local plan/index status. Push recommendation: defer until the
entire five-task plan reaches its final same-SHA delivery gate.
