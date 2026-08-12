---
status: historical
plan: terminal-presentation-remediation
task: usage-interactive-detail
retired: 2026-08-12
---

# Review: Usage Interactive Detail

## Review invalidation — 2026-08-12

The user rejected every earlier Task 3 review conclusion because it reused or
continued prior review history. All earlier rounds below remain audit history
only. They cannot establish the current candidate's verdict, close a finding,
or satisfy completion evidence.

## 📋 Full Reset Review R1 — 2026-08-12

📊 Overall score: 4/10

✅ Verdict: FAIL

Candidate identity: base commit
`033fe579e687474dcd2bf96ffc682dd75e3eb2a9` plus the complete Task 3
candidate. Initial content SHA-256 values:

- `cmd/agentdeck/usage_stats_viewer.go`:
  `890159217bb5fd9bf97bde01b87967b4bb5a9d741f178001beaebb9f2064ac70`
- `cmd/agentdeck/usage_stats_viewer_test.go`:
  `27483a9d6d828ccf3b9707654c7ca5f43c5c6c3b2819e403a51e9264eec99074`
- `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`:
  `8a3a27a939a133c38fb6dab678b7c12139f50b286f9076fba766f1a261df3821`

### 🔴 Serious issues must fix

1. **P1 blocking — rendering an empty Usage section panics.**
   [`cmd/agentdeck/usage_stats_viewer.go:650`]

   - Behavior risk: a valid report with an empty Models, Clients, Providers, or
     Cache section crashes the full-screen viewer instead of showing the
     required `no selection` empty state. This violates the empty-Detail and
     interactive lifecycle contracts.
   - Evidence: fresh
     `TestUsageViewerRenderFitsRequiredGeometriesAndEmptySections` execution
     passes all required non-empty geometry subtests, then panics at 48×10 with
     `runtime error: index out of range [0] with length 0`. The renderer guards
     initial Detail construction with `len(s.rows) > 0`, but the stacked branch
     later calls `usageViewerDetailWithin(s.rows[selected], ...)`
     unconditionally.

💡 Bounded remediation: do not enter either bounded-Detail branch unless a
selected row and rendered Detail exist. Preserve the empty section's complete
body budget and add or retain a direct regression proving 48×10 empty-section
rendering does not panic, leaves no Detail block, and reports `no selection`.

### 🟡 Suggested improvements recommended

None in this round. Broad verification stopped after the decisive blocking
reproducer, as required by the review policy.

### 🟢 Strengths

- The complete candidate uses explicit structured Detail roles rather than
  display-text keyword parsing.
- Existing geometry cases before the empty-section transition stayed within
  their visible width and height budgets.

### 📝 Summary

This was a new, independent review of the complete Task 3 candidate, not a
continuation of any earlier round. The current candidate is unsafe for a valid
empty Usage section and therefore cannot pass review or CEv1. No prior PASS or
test result is reused.

Repair: guard empty-row Detail budgeting and preserve the required empty state.

## 📋 Full Reset Re-review R1 — 2026-08-12

📊 Overall score: 7/10

✅ Verdict: FAIL

### 🔴 Serious issues must fix

None. Full Reset Review R1's P1 production panic is closed.

### 🟡 Suggested improvements recommended

1. **P2 non-blocking — hostile-label regression treats replacement whitespace
   multiplicity as selected identity.**
   [`cmd/agentdeck/usage_stats_viewer_test.go:677`]

   - Disposition: new test-harness finding.
   - Behavior risk: the Task 3 gate fails even though the rendered Detail title
     remains safe and preserves every visible identifying word. This masks
     later Usage regressions and falsely reports an identity loss.
   - Evidence: the `c0` fixture is `mo\x01\ndel`. The single-line sanitizer
     replaces both controls with spaces, while the frozen shared Detail wrapper
     intentionally normalizes word separators, producing `DETAIL · mo del`.
     A quoted byte diagnostic confirmed the title is exactly
     `44 45 54 41 49 4c 20 c2 b7 20 6d 6f 20 64 65 6c`, with no control byte
     and no visible word loss. The test instead requires `DETAIL · mo  del`.

💡 Bounded improvement: compare the Detail title's visible identity after
normalizing whitespace in both expected and actual values. Keep the existing
strict assertions that the selected row is safe, visible words remain, and no
C0/C1 control survives.

### 🟢 Strengths

- Full Reset Review R1 P1 closed: empty sections now retain the whole body
  budget, render `no selection`, and do not index an absent row at 48×10.
- The root-cause classification distinguished a product panic from a later
  over-constrained test without weakening sanitization.

### 📝 Summary

The repaired candidate closes the production panic, but the original complete
Task 3 command now reaches a stale hostile-label assertion and fails. Task 3
remains unverified until that non-blocking finding is repaired and the complete
candidate receives another independent re-review.

Repair: normalize whitespace only for the hostile Detail-title identity
comparison while retaining strict control-safety assertions.

## 📋 Full Reset Re-review R2 — 2026-08-12

📊 Overall score: 8/10

✅ Verdict: FAIL

### 🔴 Serious issues must fix

None. Full Reset Review R1 P1 and Full Reset Re-review R1 P2 are closed.

### 🟡 Suggested improvements recommended

1. **P2 non-blocking — the required Usage geometry matrix omits two wide
   fixtures.** [`cmd/agentdeck/usage_stats_viewer_test.go:468`]

   - Disposition: new.
   - Evidence: the plan requires 48×10, 60×18, 80×24, 100×24, 120×32,
     140×32, and 180×40. `TestUsageViewerRenderFitsRequiredGeometriesAndEmptySections`
     exercises only 48×10, 60×18, 80×24, 100×24, and 140×32.
   - Behavior risk: visible-cell overflow, height budgeting, or the wide
     stacked/side-by-side transition can regress at 120 or 180 columns while
     the task-local gate remains green.

💡 Bounded improvement: add the exact 120×32 and 180×40 fixtures to the same
content/width/height oracle.

2. **P2 non-blocking — the PTY resize test does not observe any resize frame.**
   [`cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go:326`]

   - Disposition: new.
   - Evidence: the test drains all output to `io.Discard`, sleeps after each
     `SIGWINCH`, cancels the context, and only asserts cancellation. It never
     requires the initial Overview frame, the 40×9 too-small frame, or the
     recovered 80×24 Overview frame.
   - Behavior risk: broken resize delivery or redraw can pass the L3 PTY gate,
     leaving a stale/blank alternate screen in the real interactive viewer.

💡 Bounded improvement: replace fixed readiness sleeps with an ordered output
observer for initial `[OVERVIEW]`, `TERMINAL TOO SMALL`, and recovered
`[OVERVIEW]`; report early viewer exit and retain the existing two-second
upper bound rather than increasing it.

### 🟢 Strengths

- Empty-section panic and hostile-label test ownership are both resolved.
- Fresh complete Usage unit tests and all Usage PTY lifecycle tests pass on the
  repaired candidate, including q, navigation, cancellation, Ctrl-C, EOF,
  write failure, and terminal restoration.
- Producer review found structured roles and omission rules consistent across
  Overview, Trend, Activity, dimension, Cache, and Coverage sections.

### 📝 Summary

The repaired candidate is behaviorally improved, but its claimed geometry and
resize evidence is incomplete. Both findings are non-blocking product-risk
coverage defects and must be closed before Task 3 can pass.

Repair: complete the exact geometry matrix and make PTY resize recovery
observable without fixed readiness sleeps or larger timeouts.

## 📋 Full Reset Re-review R3 — 2026-08-12

📊 Overall score: 10/10

✅ Verdict: PASS

Final production and test content SHA-256 values:

- `cmd/agentdeck/usage_stats_viewer.go`:
  `05002a1370aa62672009fb5e16b4dca954ef5ddf9cb5f75b8832ccc4100db66f`
- `cmd/agentdeck/usage_stats_viewer_test.go`:
  `e5649b3b42a5d2418bdd53e99de518fa4de7d5e7ac941ffa2a569e437b6a1e1b`
- `cmd/agentdeck/usage_stats_viewer_pty_darwin_test.go`:
  `b957a97785d46d8ef30b52ccc487a917adf26e8dfe7e9b65ea173c8fdaf2ac4d`

### 🔴 Serious issues must fix

None.

### 🟡 Suggested improvements recommended

None.

### 🟢 Strengths

- Full Reset Review R1 P1 closed: a valid empty Usage section no longer indexes
  an absent selected row; 48×10 renders `no selection` with no empty Detail
  card.
- Full Reset Re-review R1 P2 closed: hostile labels retain every visible
  identity word after frozen shared whitespace normalization, while C0/C1 and
  ANSI controls remain strictly absent.
- Full Reset Re-review R2 geometry finding closed: the exact 48×10, 60×18,
  80×24, 100×24, 120×32, 140×32, and 180×40 matrix now enforces line and frame
  bounds.
- Full Reset Re-review R2 resize finding closed: the PTY test observes the
  initial Overview, 40×9 too-small frame, and recovered 80×24 Overview in
  order, with one existing two-second upper bound and no readiness sleep.
- Structured producer review covers Overview, Trend, Activity, Models,
  Clients, Providers, Cache, and Coverage. Optional zero/unknown fields vanish,
  while partial, unavailable, unpriced, and warning meaning remains explicit in
  text as well as semantic color.
- Color and no-color frames retain the same labels, values, order, selection,
  and warning meaning. q, navigation, Ctrl-C, EOF, cancellation, write failure,
  resize, raw-mode restoration, cursor restoration, alternate-screen cleanup,
  and input release all retain focused PTY coverage.

### 📝 Summary

The complete repaired Task 3 candidate was independently re-reviewed across
all producer adapters, short and wide geometry, structured Detail priority,
hostile visible content, semantic color/no-color equivalence, paging and
selection state, resize, and input/terminal lifecycle. Every blocking,
non-blocking, P0-P3, and nit finding is closed; no prior review verdict was
used as the current conclusion.

Fresh evidence on the final production/test content:

- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestUsageViewer -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestRunUsageStatsViewerPTY -count=1`
- PASS: both commands above with `-race`
- PASS: Task 3 scoped `git diff --check`

Task checkpoint: `usage-interactive-detail`, pending exact staged-tree CEv1
binding and atomic commit.

Commit recommendation: include only the three Usage viewer implementation/test
paths, this replacement review record, and Task 3 plan/index status hunks.

Push recommendation: defer until all five tasks and the final exact plan tree
are verified; no intermediate Task 3 push.

Implement: `session-interactive-responsive-layout` only after Task 3's exact
tree is CEv1 VERIFIED and committed.

## Scope

- `cmd/agentdeck/usage_stats_viewer.go`
- `cmd/agentdeck/usage_stats_viewer_test.go`
- task-local plan and documentation status

## Development evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestUsageViewer -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestRunUsageStatsViewerPTY -count=1`
- Environment: `GOCACHE=/private/tmp/agent-deck-go-build`, vendored dependencies, Darwin PTY.

## Round 1 — 2026-08-11

Verdict: FAIL

### Finding

1. **P2 non-blocking — zero-component and unavailable-pricing boundaries were not falsified.** The producer independently omits zero cache fields and retains an explicit no-priced-cost warning, but the tests covered only all-zero/fully-populated cache data and partial known cost. The previous combined cache predicate could therefore return without a focused regression failure, and an empty cost card could replace the required unavailable state without detection.

No other blocking, non-blocking, P0-P3, or nit findings were identified in this round.

### Required repair

- Prove a non-zero cache-read value does not emit a zero cache-write sibling in both dimension and cache-session Detail producers.
- Prove a cost row with no priced value retains an explicitly warning-colored `UNAVAILABLE` note.

## Repair after Round 1 — 2026-08-11

- Added one-sided cache fixtures for both producer families and asserted read remains while zero write is absent.
- Added a no-priced-cost fixture and asserted the required warning role, status, and message survive rendering.

## Round 2 — 2026-08-11

Verdict: PASS

### Finding closure

1. **P2 closed.** The exact zero-sibling and unavailable-pricing regressions now fail focused assertions. Usage producers emit structured fields and notes with explicit roles and priorities; selected-row-redundant and optional zero data are omitted without losing pricing or warning state.

### Final evidence

- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestUsageViewer -count=1`
- PASS: `go test -mod=vendor ./cmd/agentdeck -run TestRunUsageStatsViewerPTY -count=1`
- PASS: scoped `git diff --check`.

No open blocking, non-blocking, P0-P3, or nit findings remain. Final combined compiled-binary and isolated-real-state acceptance remains owned by `terminal-contract-and-acceptance`.

## Review reset — 2026-08-11

All earlier PASS and Test conclusions are invalid by user direction. Every round
above remains historical audit data only and provides no current Review or Test
gate. The next review must start as `Reset Round R1` over the complete Task 3
candidate after Task 2's shared Detail contract obtains a new legal PASS and is
frozen.

## Reset Round R1 — 2026-08-12

Verdict: FAIL

### Finding

1. **P1 blocking — Usage PTY tests inherit an unsupported empty `TERM` and the
   Activity test hides the resulting early viewer exit.** In the current test
   environment `TERM` is unset. The production entry correctly rejects that
   before terminal ownership, but the Activity reader waits only for a frame and
   does not observe the `done` error, so the focused test reports a navigation
   timeout and the aggregate command can hang during cleanup.

No other findings were classified in this round because broad verification
stopped at the decisive PTY failure.

### Root-cause classification

- Test isolation/readiness defect, high confidence.
- With the command environment set to `TERM=xterm-256color`, the unchanged
  focused PTY test passes in 0.03 seconds.
- Production rendering is independently proven by the direct 80×24 Activity
  frame; it contains `[ACTIVITY]` and `1H BUCKET`.

### Required repair

- Give every Usage PTY test a supported terminal type through a task-local
  helper instead of inheriting the runner environment.
- Wait for the initial Overview frame before navigation and report a viewer
  early-exit error directly.
- Do not increase timeout values or change production behavior.

## Repair after Reset Round R1 — 2026-08-12

- Added `openUsageStatsViewerPTY`, which sets `TERM=xterm-256color` before
  opening the shared Darwin PTY fixture.
- The Activity acceptance now sends navigation input only after observing the
  initial Overview frame and reports an early viewer exit immediately.

## Reset Round R2 — 2026-08-12

Verdict: FAIL

### Finding

1. **P1 blocking — short stacked Detail truncation can hide pricing state.**
   At 48×10 the Usage renderer truncates the shared Detail frame after fields
   have already been placed before notes. Selecting Cost can therefore retain
   cost fields while dropping the required `PARTIAL`/`UNAVAILABLE` note and its
   explanation.

No other blocking, non-blocking, P0-P3, or nit findings were added in this
round. Broader verification stopped after the decisive 48×10 reproducer.

### Required repair

- Preserve the Detail title and primary warning/error note lines before optional
  field lines only when the Usage frame must truncate.
- Preserve the shared renderer's normal full ordering when the card fits.
- Add a 48×10 Cost regression for `DETAIL · COST`, `PARTIAL`, and the provider
  cost completeness explanation.

## Repair after Reset Round R2 — 2026-08-12

- Added a Usage-owned bounded Detail helper; the shared Task 2 renderer remains
  unchanged.
- Short frames keep Detail identity and semantic warning state first, then fill
  remaining lines from the normal field ordering.
- Added the focused 48×10 selected-Cost regression.
