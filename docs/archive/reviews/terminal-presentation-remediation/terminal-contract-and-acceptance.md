---
status: historical
plan: terminal-presentation-remediation
task: terminal-contract-and-acceptance
retired: 2026-08-12
---

# Review: Terminal Contract and Acceptance

## 📋 Full Review R1 — 2026-08-12

📊 Overall score: 7/10

✅ Verdict: FAIL

Reviewed content: Task 5 living-contract candidate on commit
`2ad6558c8436d45ce40990e65823ee15f3bb373b`, before compiled-binary acceptance.
No Task 1-4 verdict is reused as a Task 5 review conclusion.

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

1. **P2 — the broader terminal contract retained a fixed-page state model next
   to the new dynamic-acquisition model.**

   [`docs/specs/2026-08-06-terminal-rendering-design.md`]

   - Behavior risk: a later implementation or acceptance pass could interpret
     section state as a fixed 20-row page and regress height-derived acquisition
     or stable-identity resize behavior.
   - Evidence: the existing Session bullet said each section owns independent
     `page, selection, viewport`, while the new contract and final Task 4 code
     distinguish logical acquisition position, stable identity, visual viewport,
     and geometry-derived limits.
   - Bounded remediation: replace the stale state bullet with the final four-part
     model; do not change product code.

2. **P2 — the empty-Detail wording did not state the renderer's normalized
   field/note boundary.**

   [`docs/specs/2026-08-06-terminal-rendering-design.md`]
   [`docs/specs/2026-08-07-usage-interactive-viewer-design.md`]

   - Behavior risk: "no supplementary content" could be read as hiding a
     required warning note, or as allowing a title-only card to reserve height.
   - Evidence: `renderTerminalDetailModel` normalizes fields and notes, returns
     nil only when both are empty, and preserves required status notes.
   - Bounded remediation: define the boundary as no normalized field and no
     required note, and explicitly prohibit a title-only card.

### 🟢 Strengths

- The candidate reconciles ordinary Activity, Usage interactive Detail, root
  Session browser, and nested/direct Session Detail in living documents rather
  than copying old review conclusions.
- The geometry, semantic color/no-color, privacy, JSON, resize, and cleanup
  requirements remain observable and map to existing product or PTY oracles.

### 📝 Summary

Task 5 is not Review PASS. Two non-blocking P2 documentation findings are still
mandatory under the approved workflow. Compiled-binary synthetic and isolated
real-state acceptance, final aggregate verification, and exact-state CEv1 remain
open after repair and full re-review.

Repair: close the two contract consistency findings, then perform a complete
re-review before runtime acceptance.

## 📋 Full Re-review R2 — 2026-08-12

📊 Overall score: 8/10

✅ Verdict: FAIL

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

1. **P2 new — the Usage living contract still described a pre-implementation
   RC/session-alignment decision instead of the current delivery gate.**

   [`docs/specs/2026-08-07-usage-interactive-viewer-design.md`]

   - Disposition: new in the complete R2 surface review.
   - Behavior risk: release operators could treat a compiled RC as the evidence
     source, skip current-candidate acceptance, or believe Session alignment is
     still a future decision despite Tasks 2-4 being complete.
   - Evidence: the verification table still named a compiled RC decision gate;
     the final sections still said `Expected implementation`, described current
     conflicts that no longer exist, and deferred session alignment.
   - Bounded remediation: mark the document as a delivered reference, describe
     the delivered boundary and resolved shared lifecycle, and replace the RC
     gate with compiled-current-candidate acceptance bound to same-SHA preflight.

### 🟢 Strengths

- R1 finding 1 is closed: Session state now consistently separates logical
  acquisition position, stable selected identity, and visual viewport.
- R1 finding 2 is closed: empty Detail is defined at the normalized field/note
  boundary; required notes remain visible and title-only cards reserve no rows.
- Usage's fixed 20-row logical page remains intentional and is not conflated
  with Session's height-derived acquisition limit.

### 📝 Summary

Both R1 findings are closed, but complete R2 found one new P2 lifecycle/status
finding. Task 5 remains FAIL until the Usage living contract reflects delivered
behavior and the current-candidate, same-SHA release evidence path.

Repair: reconcile the Usage living contract lifecycle and release-evidence gate,
then perform another complete re-review.

## 📋 Full Re-review R3 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- R1 fixed-state wording is replaced by the implemented split between logical
  acquisition position, stable selected identity, and visual viewport.
- R1 empty-Detail wording now matches the normalized field/note early return:
  required notes remain visible and a title alone does not reserve rows.
- R2 lifecycle wording is closed: the Usage document is a delivered reference,
  shared Session alignment is current behavior, and the release gate begins
  with the compiled current candidate rather than an already-published RC.
- Usage retains its intentional 20-row logical page; Session alone derives
  bounded acquisition from current body height. The contracts do not conflate
  these two models.
- Ordinary Activity, structured semantic Detail, responsive Session layout,
  color/no-color, privacy, JSON invariance, resize, and terminal cleanup each
  map to an observable unit, PTY, compiled-binary, or documentation oracle.

### 📝 Summary

All R1 and R2 findings are closed. The complete Task 5 living-contract candidate
has no remaining blocking, non-blocking, P0-P3, or nit finding. This PASS covers
the contract review only; Task 5 completion remains open until compiled-current-
binary synthetic and isolated-real-state acceptance, the final aggregate gate,
exact-state CEv1, plan closure, and final commit all succeed.

Task checkpoint: `terminal-contract-and-acceptance` contract review PASS on the
uncommitted candidate derived from `2ad6558c8436d45ce40990e65823ee15f3bb373b`;
completion gate NOT_VERIFIED because runtime acceptance and L4 remain pending.

Commit recommendation: defer until Task 5 acceptance, aggregate verification,
CEv1, and plan/review archival are part of one reviewed atomic boundary.

Push recommendation: defer until the final Task 5 commit and Plan gate are
VERIFIED; then publish only the exact accepted branch SHA by direct fast-forward.

## Compiled-current-candidate acceptance — 2026-08-12

Acceptance used `/private/tmp/agent-deck-task5-candidate`, built from product
code at `2ad6558c8436d45ce40990e65823ee15f3bb373b`. Its SHA-256 was
`94681af00f4cab6780fe2d8ad5355a1a5092b3cf3225f90f27b0286718cbee34`.
Only Task 5 contract and review documents changed after that build; no product
code, tests, dependencies, generated files, or build inputs changed.

### Synthetic isolated-state matrix

- Ordinary `session show --activity` passed at 48, 80, 100, 120, and 180
  columns. The layouts retained stable ordinals and labels while moving from
  stacked to compact to bounded tabular presentation.
- JSON output was byte-equivalent across geometries after excluding only the
  contractually variable `generated_at` value. No ANSI or cursor-control bytes
  entered JSON.
- Usage interactive passed at 60x18, 80x24, 120x32, and 180x40 in color and
  no-color modes. The Models Detail card retained explicit labels and semantic
  values, omitted redundant optional content, and returned its height when
  empty. The live-resize path was `60x18 -> 180x40 -> 80x24`.
- Root and direct Session interactive passed the same geometry matrix. The root
  flow exercised browser -> detail -> Activity -> Tokens -> browser; the wide
  view retained a left-aligned 120-cell canvas. Resize preserved the selected
  identity and recomputed the height-derived acquisition/viewport geometry.
- Usage and Session each passed `q`, Escape, Ctrl-C, and EOF exits. A real
  post-raw SQLite loader error also restored the terminal. Alternate-screen,
  cursor, raw-mode, resize-signal, and reader cleanup obligations were met.
- Synthetic private-argument and private-output sentinels did not occur in any
  capture.

Five intermediate failures were harness defects, not product defects: an
inherited `NO_COLOR` value invalidated the color oracle; the Usage navigation
used the wrong tab count; a redundant `STATUS` assertion contradicted omission
rules; the ordinal oracle expected the wrong grammar for one width; and the raw
JSON comparison included `generated_at`. Each oracle was corrected to the
living contract and the affected scenario then passed. No timeout was raised
and no product change was made to hide those failures.

### Isolated real-state matrix

The approved session was referenced only by anonymous hash
`d4e9c67c520f57ba`. Its isolated copy contained 445 approved documents, 467
usage events, and 418 safe tool calls. The source-log SHA-256 was byte-identical
before copy, in the copy, and after acceptance:
`5ae8ce59cd6ee92b7a63d9d1132fbe0cf97cb3b7d0707b3fff4e6e13c2c8dc15`.
The real core and sessions databases also remained byte-identical before and
after acceptance, respectively:

- `845778d5ff0ec53353207e144b2cf3adf4314a0e9689568e0942c405595ba803`
- `8f621d61e589993dc64b66d38bb94a4bfb80eb7ed9be174988e896ff62adbd46`

The compiled binary passed Usage at 180x40 with live resize through 60x18 and
80x24 before Escape; root Session at 60x18/no-color with browser -> Overview ->
browser before `q`; direct Activity at 120x32 before Ctrl-C; and direct
Tokens/pricing at 120x32 before `q`.

One acceptance-process safety finding was found and repaired before final
review: the APFS clone of the core database initially retained encrypted
credential rows. The temporary copy was immediately scrubbed with SQLite
`secure_delete`, credential/provider/settings rows were cleared, `VACUUM` was
run, and the repaired copy reported freelist 0 and `quick_check=ok`. No
`credential.key` was copied or read, no plaintext credential was available,
and no real raw frame was retained. Only four mode-0600 one-line anonymous
capture summaries remain until durable CEv1 recording; sensitive-value and
private-path scans passed. The complete real and synthetic temporary roots must
be deleted after the evidence has been durably recorded.

## Final Full Re-review R4 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

Reviewed content: the full Task 5 living-contract candidate, all R1-R3 finding
dispositions, the compiled candidate identity, synthetic PTY matrix, isolated
real-state matrix, source/database invariance, privacy scans, failure-path
cleanup, and harness-defect diagnoses. No Task 1-4 review conclusion was reused
as a Task 5 verdict.

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- Every ordinary Activity, Usage Detail, and Session responsive-layout
  requirement maps to a living contract plus a unit, PTY, compiled-binary, or
  source-invariance oracle.
- Narrow, standard, wide, short, tall, color, no-color, resize, navigation,
  empty-Detail height recovery, and all documented exit paths were exercised.
- The selected identity, logical acquisition position, visual viewport, and
  bounded readable canvas behaved as separate concepts under resize.
- JSON stayed geometry-independent and terminal-control-free; real source logs
  and real databases stayed byte-identical.
- The encrypted-row clone issue and all five harness defects have explicit
  dispositions. None remains open and none was concealed by a product patch or
  a longer timeout.

### 📝 Summary

The complete Task 5 contract and compiled-current-binary acceptance surface has
no remaining blocking, non-blocking, P0-P3, or nit finding. Contract review and
Acceptance are PASS. Task 5 is not yet complete: the one final L4
`release-verify` run and exact-state CEv1 Task/Plan gates remain pending.

## Final Full Re-review R5 after Task 4 repair — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

Reviewed content: the complete Task 5 contracts and acceptance evidence, the
first L4 failure, Task 4 Post-PASS Review R3 finding and Full Reset Re-review R4
repair disposition, signed commit `e4174af1175dae17eb71517bb51ef118ede84a7f`,
tree `ba99579614ae834aa6ea4dbf07fac8c3442007d7`, and its commit-bound CEv1
`VERIFIED` gate.

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🔍 L4 failure and acceptance rebind

- The first L4 run failed only in `make test`: `cmd/agentdeck` reached Go's
  default ten-minute timeout. Focused diagnostics located the unbounded PTY
  readiness wait and external `TERM=dumb` leak in Task 4 test harness code.
- Task 4 repaired both affected tests, completed a new full R4 re-review, passed
  focused, package, and PTY race verification with external `TERM=dumb`, and
  reached commit-bound CEv1 `VERIFIED`. No product code was changed to conceal
  the failure and no timeout was increased.
- The diff from the accepted product baseline `2ad6558c8436d45ce40990e65823ee15f3bb373b`
  to `e4174af1175dae17eb71517bb51ef118ede84a7f` contains only one `_test.go`
  file plus Task 4 review/status documents. Go product source, dependencies,
  generated files, and build inputs are byte-unchanged.
- The compiled candidate SHA-256
  `94681af00f4cab6780fe2d8ad5355a1a5092b3cf3225f90f27b0286718cbee34`
  therefore remains the exact product content exercised by synthetic and
  isolated-real-state acceptance. That Acceptance is rebound to the new Task 4
  commit baseline; no visual or runtime oracle was invalidated.

### 📝 Summary

The complete Task 5 surface has no remaining blocking, non-blocking, P0-P3, or
nit finding after the Task 4 repair. Full Re-review R5 and compiled-current-
candidate Acceptance are PASS. Task 5 remains open only for a fresh final L4
`release-verify` on the exact current candidate and Task/Plan CEv1 gates.

### Final L4 result

The repaired exact candidate passed the aggregate gate:

```text
env -u VERSION \
  GOCACHE=/private/tmp/agent-deck-go-build \
  GOMODCACHE=/private/tmp/agent-deck-go-mod \
  make release-verify
```

Result: PASS, exit code 0. The aggregate covered full tests, race, vet,
darwin/arm64 and darwin/amd64 builds, arm64 size, installer, privacy, release
distribution, and release-preflight contract tests. The candidate content was
HEAD `e4174af1175dae17eb71517bb51ef118ede84a7f` plus Task 5 scoped fingerprint
`2106ba31fa730acfd2ee6d81dcaaf6e0d64c29e55ce2b3244e3d3ffcac10d43a`.
Task 5 Review, Test, and Acceptance are now PASS; exact-state CEv1 and archival
closure remain pending.

## Documentation Lifecycle Full Re-review R6 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

Reviewed content: archival relocation of the completed plan and all five review
records, historical/retired frontmatter, living-contract retention, active
index removal, archive index explanation, and separation between development
closure and later same-SHA delivery stages.

### Findings

None. No blocking, non-blocking, P0-P3, or nit finding remains.

### Evidence boundary

This is an L0 documentation-lifecycle review. Product code, test code,
dependencies, generated files, and the compiled candidate are unchanged, so the
existing focused, PTY, real-state, and L4 evidence remains valid. The archival
paths and final documentation fingerprint require a new CEv1 rebind before the
outer Plan gate can be marked VERIFIED.

Candidate gate query: PASS. The outer Plan required two criteria and satisfied
both, all five Task WorkUnits were `VERIFIED`, and the Plan target matched
archival fingerprint
`89abb1d104f0e1b860da997fa29c13506769438fdbd1e97be52c747060fc73d9`.
The final signed commit rebinds this unchanged reviewed content to an immutable
Git tree as a delivery integrity check.
