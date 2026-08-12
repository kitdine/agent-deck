---
status: active
plan: terminal-presentation-remediation
task: session-show-activity-presentation
---

# Review: Session Show Activity Presentation

## Scope

- `cmd/agentdeck/session_show_text.go`
- `cmd/agentdeck/session_show_text_test.go`
- existing CLI safe-metadata and JSON coverage in `cmd/agentdeck/main_test.go`
- task-local plan and documentation status

## Development evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestRenderSessionShowText -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestSessionShowActivityLines -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestSessionShowActivityReadsOnlySafeMetadata -count=1`
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies.

## Round 1 — 2026-08-11

Verdict: FAIL

### Findings

1. **P1 blocking — standard and compact records violate the approved information order.** The candidate emits `STARTED` before `TOOL` and `STATUS`, while the observable contract fixes `CALL`, then `TOOL`/`STATUS`, then `STARTED`/`MODEL`/completion boundary. The standard path also delegates the whole record to the generic field wrapper, so a normal 80-column record is not reliably the required two grouped lines.
2. **P2 non-blocking — one safe-empty record disables the wide table for the complete page.** `sessionShowActivityTableLines` returns false when any record has no optional fields. At wide geometry this collapses every otherwise tabular record instead of representing the empty record explicitly in a labeled `STATE` cell.
3. **P2 non-blocking — redundant duration text and missing order assertions leave the 80-column contract unprotected.** Per-call duration includes both the human duration and raw milliseconds, making the ordinary record needlessly long. Tests cover labels and maximum width but do not assert the contract's layout-specific order, grouped standard lines, or wide safe-empty behavior.

No P0, P3, or nit findings were identified in this round. Broad verification stopped after the decisive P1 reproducer; the unchanged development evidence remains recorded but does not close Review.

### Required repair

- Add layout-specific field ordering and a dedicated standard two-group renderer.
- Keep wide table mode when an empty record is present by rendering an explicit `STATE` column.
- Use the concise per-call duration value and strengthen observable layout tests without binding incidental spacing.

## Repair after Round 1 — 2026-08-11

- Reordered standard and compact records to `CALL`, `TOOL`/`STATUS`, `STARTED`/`MODEL`, then `DURATION` or valid `COMPLETED`.
- Added grouped standard rendering and concise per-call duration values. Activity timestamps use the active zone abbreviation so the normal 80-column record remains two lines while fixed zones such as `UTC+8` stay explicit.
- Preserved wide table mode for safe-empty records with a labeled `STATE` column.
- Added assertions for standard grouping, compact order, wide safe-empty rows, hostile Unicode/control cells, invalid timestamps, optional-field omission, and content-bounded wide output.
- Updated the existing wide-page, display-zone, and CLI duration assertions to the approved responsive contract without weakening privacy, pagination, or JSON checks.

## Round 2 — 2026-08-11

Verdict: PASS

### Finding closure

1. **P1 closed.** Standard and compact layouts now follow the approved information order. A representative 80-column record is exactly two semantic groups; longer values wrap without losing labels.
2. **P2 closed.** Wide tables retain all records and represent a safe-empty call with an explicit `STATE` cell.
3. **P2 closed.** Per-call duration is concise, and layout-specific tests protect order, grouping, width, sanitization, empty state, and bounded wide output.

### Final evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestRenderSessionShowText -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestSessionShowActivityLines -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run=TestSessionShowActivityReadsOnlySafeMetadataOnDemand -count=1`
- PASS: scoped `git diff --check` for Task 1 tracked paths.
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies.

No open blocking, non-blocking, P0-P3, or nit findings remain. Full-suite and compiled-binary acceptance remain deferred to the plan's final exact-state aggregate gate.



## Review reset — 2026-08-11

All earlier PASS and Test conclusions are invalid by user direction. Every round above remains historical audit data only and provides no current Review or Test gate. The next review must start at `Reset Round R1` over the complete Task 1 candidate and may not reuse Round 2 as a baseline verdict.

## Reset Round R1 — 2026-08-11

- Candidate content state: baseline
  `5afc0a18142f7e08137349d39ad961bbc7315f4b` plus the current uncommitted
  Task 1 production/test diff, SHA-256
  `ea44cdd57f11b33f05bc9bd8c971325d91187948ce9f45e5faa4e7d54dead2f9`.
- Candidate identity command: `git diff --binary --full-index
  5afc0a18142f7e08137349d39ad961bbc7315f4b --
  cmd/agentdeck/main_test.go cmd/agentdeck/session_show_text.go
  cmd/agentdeck/session_show_text_test.go | shasum -a 256` (exit 0). A
  differently scoped command without `--full-index` produces different patch
  bytes and is not evidence of content drift.
- Safety-source evidence: the read-only safety source's `git diff --binary
  --full-index` remained byte-identical to the recorded
  `safety-source-dirty-all.patch` (`cmp`, exit 0), and `shasum -a 256 -c
  SHA256SUMS` exited 0.
- Finding evidence: focused source queries exited 0 and showed that pagination
  is not passed to `sessionShowActivityLines`, while its standard, compact, and
  wide-table paths each render a local `index+1`; a separate query showed that
  `StartedAt` is formatted to `—` before optional-sentinel filtering. The
  smallest existing optional-field test exited 0 but preserves the invalid-time
  dash and does not cover unknown/unavailable start-time omission.
- Reviewed scope: the complete `session-show-activity-presentation` Task 1
  candidate, including ordinary `session show --activity` rendering,
  pagination behavior, optional-field omission, terminal-width degradation,
  JSON/privacy invariants, and its unit/CLI regression coverage.
- Prior test results and every earlier review verdict are invalid audit history;
  none are evidence for this round.

### Findings

1. **P1 blocking — paged output loses stable absolute ordinals.** Rendering a
   later page slice restarts activity labels at `CALL 1` instead of preserving
   each call's absolute ordinal in the complete result. This violates the
   stable-ordinal presentation contract and makes page-to-page references
   ambiguous.
2. **P2 non-blocking — unavailable optional start time is retained.** An
   optional `StartedAt` value that is unknown or unavailable is converted to
   an em dash and displayed rather than omitted. This violates the optional
   field omission contract and adds empty information to the record.

Repair must close both findings, add focused regression coverage for later-page
absolute numbering and unavailable `StartedAt` omission, and run fresh targeted
tests against the repaired exact content state before full re-review. Task 5
owns reconciliation into the living terminal contract documents.

**Verdict: FAIL.** One P1 blocking and one P2 non-blocking finding remain open.
No P0, P3, or nit findings were identified.

## Repair after Reset Round R1 — 2026-08-11

- Passed the activity pagination start ordinal into all wide, standard, and
  compact render paths, with page-aware integration coverage at 48, 80, and 180
  columns.
- Filtered semantic `unknown` and `unavailable` `StartedAt` values before time
  formatting while preserving the explicit em dash for other malformed safe
  timestamps.
- Added focused coverage for absolute later-page ordinals, sentinel omission,
  JSON/privacy-preserving CLI behavior, and responsive widths.
- Fresh verification passed:
  - `go test -mod=vendor ./cmd/agentdeck -run TestRenderSessionShowText -count=1`
  - `go test -mod=vendor ./cmd/agentdeck -run TestSessionShowActivityLines -count=1`
  - `go test -mod=vendor ./cmd/agentdeck -run TestSessionShowActivityReadsOnlySafeMetadata -count=1`
  - scoped `git diff --check`
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies.

## Reset Round R2 — 2026-08-11

**Verdict: FAIL.** The two Reset R1 findings were closed, but full candidate
review found one new P2 non-blocking issue: direct
`(page-1)*limit+1` arithmetic could overflow for a validated extreme page value
and render a negative or wrapped `CALL` ordinal. No P0, P1, P3, or nit finding
was added.

## Repair after Reset Round R2 — 2026-08-11

- Added a saturating page-first-ordinal helper consistent with the session
  pagination layer's extreme-page safety contract.
- Added `TestSessionShowPageFirstOrdinalSaturatesHugePages` and reran the complete
  Task 1 targeted verification set.

## Reset Round R3 — 2026-08-11

Candidate content state: baseline
`5afc0a18142f7e08137349d39ad961bbc7315f4b` plus Task 1 production/test diff
SHA-256 `ad15504ae7d6a73bfbe944ed835fa41d9792eb2226d84043d0bcb213dd79c1d6`,
computed with the same `git diff --binary --full-index` Task 1 path set used by
Reset R1.

**Verdict: PASS.**

- Reset R1 P1 closed: page 3 with limit 10 renders absolute call 21 in compact,
  standard, and wide modes; no page-local call 1 is emitted.
- Reset R1 P2 closed: `unknown`, `unavailable`, and case/space variants consume
  no `STARTED` field or em-dash value at all required layouts.
- Reset R2 P2 closed: huge-page arithmetic saturates at the maximum positive
  ordinal and cannot wrap negative.
- The complete renderer, pagination integration, optional-field, ordering,
  display-zone, visible-cell, hostile-input, JSON-invariance, and privacy-safe
  metadata surfaces were re-reviewed. No open blocking, non-blocking, P0-P3, or
  nit findings remain.
- Fresh final evidence, all exit 0:
  - `go test -mod=vendor ./cmd/agentdeck -run TestRenderSessionShowText -count=1`
  - `go test -mod=vendor ./cmd/agentdeck -run TestSessionShow -count=1`
  - `go test -mod=vendor ./cmd/agentdeck -run TestSessionShowActivityReadsOnlySafeMetadata -count=1`
  - scoped `git diff --check`
- Safety-source dirty patch remained byte-identical to the baseline capture.
  Living contract reconciliation remains owned by Task 5.
