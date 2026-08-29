---
status: active
topic: work-signals
subject: activity-classification
---

# Review log — work-signals / activity-classification

## Round 1 — 2026-08-27

- Reviewed state:
  - HEAD `71f54c1538c43c2d30911bdebbf83d4e460951f7`, working tree uncommitted
  - CEv1 candidate `a7f47f0480363958d16d64669c4932e872667e6978e8603adb84ae76c75afb28`
  - scoped manifest `8488fdc596e3c30a380be44be593fe8b884b7e76d5ae85050d201c886ca80a0d`
- Reviewer: Codex, independent formal Review after the Development route closed
- Method: contract-first review against `tasks.md` task 2 and architecture
  Decisions 3–6 and 11. CodeGraph located the existing scan and migration impact
  paths; the untracked classifier, persistence, cost, metrics, and test files
  were then read directly. Dynamic SQL, the schema replacement, reset and
  duplicate-source paths, cost joins, metric scopes, and regression protection
  were checked from source. One Go overlay test, kept outside the repository and
  removed after execution, exercised the exact incremental boundary that source
  inspection identified.
- Scope: the task 2 implementation, tests, schema v21/parser v6, both canonical
  desktop fixtures, `tasks.md`, and `docs/status.md`. Production code, tests,
  configuration, and fixtures remained read-only.

## 📋 activity-classification Review report

📊 Overall score: 6/10

✅ Verdict: FAIL

### 🔴 Serious issues — must fix

[`internal/usage/signals.go:25`, `internal/usage/usage.go:1102`,
`internal/usage/usage.go:1427`] **[R1-F1] Command-derived `testing` and
`maintenance` intent is lost when a turn crosses an incremental scan boundary.**

- Behavior risk: Decision 3 says a command naming a test runner classifies an
  edited turn as `coding/testing`, and Decision 11 says classification is a pure
  function of the persisted message reduction plus `usage_tool_calls`. The
  implementation folds `Record.CommandHint` only into `state.signals` created
  during the same scan. `usage_tool_calls` persists `command_read` but not the
  bounded command hint, and `turnShape` reconstructs neither `TestingCmd` nor
  `ChoreCmd`. When the user message is scanned first and the assistant's edit and
  test command arrive later, the pending row promotes as `coding/feature`, the
  visible fallback, instead of `coding/testing`. The same loss affects the
  command-shaped `maintenance` rule.
- Evidence: the focused overlay reproducer scanned a Claude user message alone,
  then appended one assistant entry containing `Write` and `Bash` with
  `go test ./...`. The required wrapper reported:

  ```text
  scripts/run-go-test.sh -overlay=/private/tmp/agentdeck_review_overlay.json \
    -run TestReviewCrossScanCommandHintSurvives ./internal/usage
    -> FAIL
    cross-scan turn = "classified" "coding"/"feature",
       want classified coding/testing
  ```

  The repository test `TestClassificationSurvivesAScanBoundary` covers only a
  message-derived `debugging` classification, so the existing suite cannot
  falsify loss of command-derived subcategories.

💡 Bounded remediation: persist only the bounded `testing`/`chore`/empty hint
with each tool call, or derive the same bounded fact deterministically from
other persisted tool-row fields without retaining command text. Restore
`TestingCmd` and `ChoreCmd` in `turnShape`, then add incremental split-scan
regressions for both command-derived subcategories. Keep the schema and privacy
boundary inside task 2; no surface or downstream task change is required.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- The message scan implements whole-word earliest-match precedence, including
  the required `add error handling` versus `fix the add button` distinction.
- The schema replacement matches Decision 11's pending/classified and
  source-ownership shape, parser version 6 triggers the required backfill, and
  each canonical fixture changes only schema count `20` → `21`.
- Pending visibility, same-scan promotion, reset, chat-only preservation,
  duplicate-source agreement, structural cost joins, and nullable workflow
  metric states have focused source and regression coverage. These strengths do
  not close R1-F1's uncovered incremental command-hint path.

### 📝 Summary

The reviewed uncommitted candidate is structurally broad and most named task
boundaries are implemented, but its incremental classifier does not preserve
all inputs needed to recompute the eleven-subcategory contract. The decisive
reproducer makes the Task incomplete, so broader verification stopped after the
finding as required. Residual uncertainty is limited to paths not needed to
establish this FAIL; they return to independent Re-review after R1-F1 is repaired.

- Completion gate: FAILED — R1-F1 disproves the
  `incremental-classifier-state` and `subcategory-contract` criteria for this
  candidate content state. The prior Development `pass` evidence remains an
  immutable record of what was checked and does not override the review
  reproducer.
- Verdict: REOPEN

## Round 2 — independent re-review — 2026-08-27

- Reviewed state:
  - HEAD `71f54c1538c43c2d30911bdebbf83d4e460951f7`, working tree uncommitted
  - repair candidate `d86c7975227bdc92a3903938de85b88d92841876b960fd9fd7c47a180b90be48`
  - scoped manifest `dfe0c2ff05bd51bb3528ef0bc40e354ced5022a7bd354bf315d2f9808248c573`
- Reviewer: Codex, independent Re-review against the recorded R1-F1 reproducer
- Method: finding-first. Re-read the mutated persistence, reconstruction,
  migration, and regression-test paths; used CodeGraph to confirm the current
  `Record` → `upsertToolActivityTx` → `usage_tool_calls` → `turnShape` path; ran
  the two repository regressions that replace the Round 1 overlay diagnostic,
  the affected migration test, and the full L2 Go suite. The repository already
  contained the bounded repair when Re-review began, but no Repair section or
  Beads transition had been synchronized; this round records the observed repair
  state without inventing an author or a prior completion receipt.
- Scope: R1-F1 and regressions caused by persisting and reconstructing the
  bounded `command_hint`. Production code, tests, migrations, configuration,
  and fixtures remained read-only during Re-review.

## 📋 activity-classification Re-review report

📊 Overall score: 9/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- **R1-F1 — CLOSED.** Schema v21 adds `usage_tool_calls.command_hint` with an
  empty default; `upsertToolActivityTx` writes only `Record.CommandHint`, whose
  parser-side producer reduces command text to `testing`, `chore`, or empty.
- `turnShape` now reads `command_hint` from the persisted tool row and restores
  `TestingCmd`/`ChoreCmd`, so classification remains a pure function of the
  stored message reduction plus the turn's stored tool rows across scans.
- `TestClassificationRecoversTestingCommandHintAcrossAScanBoundary` reproduces
  the exact Round 1 failure shape and now reaches `coding/testing`.
  `TestClassificationRecoversChoreCommandHintAcrossAScanBoundary` covers the
  sibling `coding/maintenance` path rather than assuming the first fix covers
  both constants.
- The full Go suite passes after the production, schema, and test changes, and
  the existing privacy regression continues to reject retained command text.

### 📝 Summary

R1-F1 is closed in repair candidate `d86c7975…`: the bounded command hint now
survives the incremental boundary, both affected subcategories have direct
regressions, and no new blocking finding was found. The Round 1 failure evidence
remains attached to its superseded state. Residual uncertainty is limited to
runtime environments outside this L2 task boundary; no release, installation,
or real-state acceptance is part of this Task.

### Evidence

```text
scripts/run-go-test.sh -run \
  'TestClassificationRecovers(Testing|Chore)CommandHintAcrossAScanBoundary|TestV20AndV21MigrationsAddWorkSignalStorage' \
  ./internal/usage ./internal/store
  -> PASS (log sha256 9b0b4721648bfe1ecf5773debbd3653b2c87ab611a19a93e8fcccb38cc017c9e)

scripts/run-go-test.sh ./...
  -> PASS (log sha256 313bc26d4a268853699923f90aa41e4e3094cdc21f1f8d2654aa1447389690ea)
```

- Completion gate: VERIFIED — all ten criteria are re-bound to the final
  synchronized candidate; the R1-F1 fail records remain on the superseded Round
  1 state.
- Verdict: PASS
