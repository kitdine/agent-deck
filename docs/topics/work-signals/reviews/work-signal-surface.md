---
status: active
topic: work-signals
subject: work-signal-surface
---

# Review log — work-signals / work-signal-surface

## Round 1 — 2026-08-31

- Reviewed state:
  - HEAD `7a1160a86e549a3ae3532bbfe8b782fdbfbfef82`, working tree uncommitted
  - scoped manifest
    `e498ce918890e53bbb376693e188d66ec0844b064e0748b4813e6df8afdd1fd1`,
    recomputed independently from the WorkUnit's own `target_state_recipe` and
    byte-identical to the state the Development evidence is bound to, so this
    round judges exactly the state that was claimed to pass
- Reviewer: Claude Code, independent formal Review after the Development route
  closed. The implementation was produced by Codex; this review shares neither
  its context nor its role.
- Method: contract-first review against `tasks.md` task 5 and
  `ux/session-work-signals.md`, with the prototype treated as the authority
  wherever the two disagree. The presentation model, the SwiftUI views, both
  test files, and the localization catalog were read directly. Because this
  surface only consumes a wire contract, the producer side was re-read read-only
  to establish what the panel can actually be handed:
  `internal/desktop/desktop.go`, `internal/usage/signals_report.go`,
  `internal/usage/signals_metrics.go`, and the sibling CLI renderer
  `cmd/agentdeck/usage_signals_text.go`. The four canonical fixtures were decoded
  with `jq` to see which scope combinations any test can reach. Single-agent
  repository review; no multi-agent panel and no external scoring skill, whose
  anchors do not fit a GUI-contract target. Production code, tests, fixtures,
  and configuration stayed read-only.
- Scope: `apps/macos/AgentDeckApp/{DesktopCopy.swift,Localizable.xcstrings,
  MenuBarPanelViews.swift,MenuBarViewModel.swift}`,
  `apps/macos/AgentDeckAppTests/{AppTestFixtures.swift,MenuBarChromeTests.swift,
  MenuBarViewModelTests.swift}`, the topic's `tasks.md`,
  `ux/session-work-signals.md`, `acceptance/work-signal-surface.md`, and the
  Work Signals row of `docs/status.md`.

## 📋 work-signal-surface Review report

📊 Overall score: 6/10

✅ Verdict: FAIL

### 🔴 Serious issues — must fix

[`apps/macos/AgentDeckApp/MenuBarViewModel.swift:832`] **[R1-F1] The panel
requires all three families to carry an item for the selected scope, so a
captured scope that simply had no tool call renders the legacy "Not captured
yet" surface and tells the user the snapshot lacks fields it actually has.**

- Behavior risk: `workSignalPanel` looks up one item per family for the selected
  `period × client` (`MenuBarViewModel.swift:818-820`), returns `.empty` only
  when all three are missing (`:821`), and otherwise falls through
  `guard let activity, let workflow, let tooling` to `.uncaptured` (`:832`).
  `.uncaptured` is the state the contract reserves for a payload that predates
  the capture: the heading gets the `待采集` / `Not captured yet` flag, all three
  summary figures disappear, and every detail view is replaced by the banner
  "These fields are not in the snapshot yet". The producer, however, projects the
  three families independently. `internal/desktop/desktop.go:587`, `:593`, and
  `:603` each append their scope's item only when that family's own `Available`
  is true, and the three flags have three different bases:
  `internal/usage/signals_report.go:203` sets activity from classified turns,
  `:129` sets workflow from at least one session in scope, and `:288` sets
  tooling from `total > 0` over `usage_tool_calls`. A scope with classified turns
  and zero tool calls — a day, or a client filter, spent in `conversation` turns,
  which is a first-class category of this very topic — therefore emits activity
  and workflow items and no tooling item, and the whole panel reports that the
  capture is missing. The state table in `ux/session-work-signals.md` maps that
  scope to the captured rendering with `—` for what was not measured, and
  reserves the uncaptured rendering for `unavailable`; `—` is defined there as
  "the scope produced nothing to measure", which is precisely the honest
  rendering this code replaces with a false one. The same collapse runs the other
  way: a scope whose turns are all still `pending` yields no items at all, which
  the code reads as `empty` and hides the three cards with no explanation, while
  the panel above them still lists sessions and spend.
- Evidence: the three producer lines and the three `Available` bases above were
  each read at the cited location. The sibling first-class surface for the same
  data disagrees with the panel: `cmd/agentdeck/usage_signals_text.go:64` and
  `:134` render each family independently and print
  `No tool call in the selected scope.` for exactly this case, so the project
  already treats a tooling-less captured scope as a named, expected state rather
  than as a missing capture. Nothing in the delivered tests or fixtures can reach
  the divergence: every `work_signals` block in `desktop/fixtures/v1/` carries
  identical scope coverage across the three families (nine positions each in
  `snapshot-complete.json`, six each in `snapshot-empty-client.json`), and
  `WireFixture.workSignals` builds all three families from the same
  `periods × clients` product, so the state test
  (`MenuBarViewModelTests.swift:334`) only ever exercises all-present or
  all-absent.
- 💡 Bounded remediation: decide the presentation state per family instead of
  by unanimity. Keep `.uncaptured` for the family-level `available: false` and
  the missing-`work_signals` payload it already handles; when the payload is
  captured, render the captured surface and let each family that has no item for
  the selected scope produce `—` figures, which the formatters already emit for
  `nil`. Distinguish "captured but nothing in this scope" from "no spend in this
  scope" using the panel's own session statistics rather than item presence. Add
  one model test whose fixture omits a single family's item for one scope; it
  fails today with `.uncaptured` and passes once the state is per-family.

### 🟡 Suggested improvements — recommended

[`docs/topics/work-signals/ux/session-work-signals.md:179`] **[R1-F2] The
reviewed UX contract was materially rewritten during this task's development
while its review record, its Documents matrix cell, and its own frontmatter all
still describe the earlier text.**

- Behavior risk: the acceptance paragraph now says actual VoiceOver, TCC, and
  system accessibility automation are "not run and are not completion
  requirements". The substance is an operator decision recorded on 2026-08-31 and
  is not in question. The bookkeeping is: `reviews/documents.md:1776` binds the
  set's latest PASS to `ux/session-work-signals.md` at blob
  `7f0654bef23fbab8af3e67e38351f08156475989` and the working tree now holds
  `a0d5342bafdb4b89e811d73930a9068a42fcb52d`; the `Documents` matrix still shows
  that row as `Review [x]`; and the frontmatter still reads `updated: 2026-08-20`
  against `docs/documentation-workflow.md:40`, which requires the field to move
  when a current document materially changes. A ticked cell that points at a PASS
  rendered over different text is the failure mode this topic has already paid
  for twice, and the next reader of the contract has no signal that its last
  paragraph was never reviewed. The same paragraph's neighbour compounds it: the
  `Copy` section still says "New strings, and only these" over fourteen keys
  while the delivered catalog adds thirty-five, so a later reader who takes that
  list as the macOS catalog delta — task 6 `work-signals-contract` is exactly
  that reader — will find it wrong.
- Evidence: `git hash-object` on the working tree against the blob recorded at
  `reviews/documents.md:1776`; the frontmatter date; the matrix row at
  `tasks.md:17`; and the twenty-one keys added to `DesktopCopy.swift` beyond the
  contract's table, each of which matches an approved prototype entry in
  `prototype/src/i18n.js:54-93` verbatim in both languages.
- 💡 Bounded remediation: append one round to `reviews/documents.md` covering the
  new blob — the change is narrow and its substance is already decided, so the
  round is a ratification, not a re-litigation — and set the frontmatter to
  `updated: 2026-08-31`. In the same pass, restate the `Copy` section as a delta
  against the prototype dictionary, which is what it actually is, so it stops
  reading as a claim about the shipped catalog.

### 🟢 Strengths

- The two-level structure matches the prototype element for element rather than
  approximately: the parent Activity row carries `cost · events` with the share
  right-aligned and the bar beneath, the subcategory row carries share and cost
  and no bar, `Most touched` prints `tasks.md ×4`, and Top MCP prints
  `codegraph · 7` — each identical to `prototype/src/Popover.jsx:508-625`.
- Every one of the thirty-five catalog additions carries the prototype's approved
  Simplified Chinese verbatim, including the ones the contract's table does not
  list, and the two keys the contract deliberately withholds
  (`sessions.shareOfCalls`, `sessions.times`) are genuinely absent — the bare
  percentage and the hardcoded `×` are rendered without them.
- `cost_basis` is handled the same way on both surfaces. `none` blanks share and
  cost while leaving the event count, exactly as
  `cmd/agentdeck/usage_signals_text.go:82-85` does, and `partial` is given no
  panel treatment at all, which is what the contract asks for and what the new
  test pins.
- Measured zero is kept distinct from unavailable structurally, not by
  convention: each formatter takes an optional and only `nil` becomes `—`, and
  `testWorkSignalFormattingKeepsMeasuredZeroDistinctFromUnavailable` asserts both
  halves for all four formatters.
- The expanded-state and focus contract is carried by native affordances rather
  than reconstructed — `DisclosureGroup` for the single expanded row,
  `@AccessibilityFocusState` for the detail heading and the return target — and
  `WorkSignalNavigationState` is a value type whose one-open-row and
  return-to-opening-card rules are unit-testable without a running panel.
- The narrow bound is rendered at the real content width. `ViewThatFits` keeps
  the chevron only when the header fits, and the attachments are produced at
  256 pt, which is the 280 pt panel minus its `MenuBarGeometry.padding` on both
  sides.

### 📝 Summary

The reviewed candidate is HEAD `7a1160a` plus the uncommitted task 5 scope,
manifest `e498ce91…`, verified to be the same state the Development evidence
claims. Everything the contract specifies about the captured rendering is
present and correct — the fixed orders, the subcategory omission rule, the cost
bases, the two withheld strings, the bilingual copy, the narrow layout, and the
navigation state — and the automated checks pass: `MenuBarChromeTests` 12/12,
the three new `MenuBarViewModelTests` cases, `scripts/check-topic-docs.sh` clean
for this topic, `make check-whitespace` and `git diff --check` clean, and no Go
file changed, so the Development L2 result stands for an unchanged Go tree.

The verdict is `FAIL` on R1-F1. The panel's state is decided by the presence of
all three family items at once, which is not how the producer emits them, and
the failure mode is the one this topic has treated as blocking twice before:
a state that looks honest and is not. A captured scope with no tool call is a
condition the CLI already names in words, and the panel answers it with "these
fields are not in the snapshot yet". No fixture or test can reach it, which is
why nine criteria answered `pass` over a state that does not satisfy
`state-and-legacy`. R1-F2 is minor and cheap, but it is open, so it is named
here rather than carried: a contract document changed after its PASS, and three
records still describe the text it replaced.

Residual uncertainty is stated rather than resolved. Actual VoiceOver speech,
TCC, and system accessibility-setting automation were not executed here either —
by the operator's 2026-08-31 decision they are not completion requirements, and
the acceptance record says so plainly instead of implying they passed — so the
accessibility claims in this review are structural and textual only. One
pre-existing App test at `MenuBarViewModelTests.swift:415`
(`testProviderWithMultipleReadyTargetsUsesOneRowAndASecondLevel`) fails on this
machine because a Chinese localization meets a hardcoded English expectation; Task 4's re-review
already recorded that as an environment fact and this candidate does not touch
that path.

- Evidence:
  - `env DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild
    -project apps/macos/AgentDeck.xcodeproj -scheme AgentDeck -configuration
    Debug -derivedDataPath apps/macos/build/DerivedData CODE_SIGNING_ALLOWED=NO
    CODE_SIGNING_REQUIRED=NO CODE_SIGN_IDENTITY=
    -only-testing:AgentDeckAppTests/MenuBarChromeTests test` — 12/12 passed,
    including the two new state/format cases and the eight narrow renderings.
  - the same command with `-only-testing:AgentDeckAppTests/MenuBarViewModelTests`
    — 31 executed, 1 failure, the pre-existing localization case named above; all
    three new work-signal cases passed.
  - `bash scripts/check-topic-docs.sh` — no gap for `work-signals`; the two
    reported gaps belong to the concurrent `schema-version-signal` topic, which
    is outside this candidate's manifest.
  - `make check-whitespace` and `git diff --check` — both clean.
  - `jq` over `desktop/fixtures/v1/*.json` — per-family scope coverage is
    identical in every fixture, which is why R1-F1 is unreachable from the
    delivered tests.
  - `git hash-object` against `reviews/documents.md:1776` for R1-F2.
  - manifest recomputation from the WorkUnit's `target_state_recipe`, matching
    `e498ce91…`.
- Completion gate: NOT_VERIFIED — `work-signals:work-signal-surface`. This round
  recorded `fail` evidence for `state-and-legacy` against the post-synchronization
  candidate
  `11e514ab42f04046c661682569b12fac2c6001b7cc398396851e4ee8fe84c88f`,
  superseding the Development `pass` on `e498ce91…`. The re-queried gate answers
  `fail` for `state-and-legacy` and carries no evidence yet for the other eight
  criteria at that state, so the Task boundary stays open. The other eight were
  not disproved by this review.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-31

- Repairer: Codex
- Scope: R1-F1 and R1-F2 only. The captured layout, copy values, cost-basis
  behavior, narrow rendering, Task 4 wire/producer, and unrelated
  `schema-version-signal` work are unchanged.
- Repaired state: HEAD `7a1160a86e549a3ae3532bbfe8b782fdbfbfef82`,
  working tree uncommitted; the synchronized Repair candidate is recorded in
  completion evidence after task/status synchronization.

### R1-F1 — CLOSED

`workSignalPanel` now decides no-scope emptiness from the selected session
statistics rather than from Work Signal item presence. It tracks family-level
uncaptured state in `uncapturedSections`:

- `available: false` keeps only that family on the retained Not captured yet
  path and leaves captured sibling families visible;
- `available: true` with no selected-scope item stays captured and renders `—`
  for that family's summary and details;
- a scope with sessions but no items from any family renders three captured
  `—` cards instead of disappearing as empty;
- a true zero-session scope still omits the cards.

The regression fixture can omit only `today/all` Tooling, omit all three
`today/all` items while retaining two sessions, or mark only Tooling unavailable.
`testCapturedWorkSignalScopeKeepsEachMissingFamilyHonest` asserts all three
boundaries and fails on the reviewed implementation's unanimity rule.

### R1-F2 — CLOSED

`ux/session-work-signals.md` now has `updated: 2026-08-31` and calls its Copy
table the design delta against the approved prototype dictionary, not an
exhaustive macOS catalog diff. The existing prototype labels imported verbatim
by the app are named as such.

`reviews/documents.md` Round 9 records the bookkeeping ratification on the
independent Task Review R1-F2 basis without presenting Codex as an independent
reviewer of its own work. The new document blob is
`0b8556212d82640312777464787aa145c7c94528`; its Document WorkUnit gate is
VERIFIED at content state `050f4faf8e5d1d2e92123857ab7041902a362f5a5ff5e15fe592c21e6582413c`.

### Evidence

```text
xcodebuild ...
  -only-testing:AgentDeckAppTests/MenuBarViewModelTests/
  testCapturedWorkSignalScopeKeepsEachMissingFamilyHonest
  -> PASS
xcodebuild ... -only-testing:AgentDeckAppTests
                 -only-testing:AgentDeckSharedTests
  -> PASS
work-signals ux/session-work-signals.md Document gate
  -> VERIFIED
```

The Development Go L2 evidence is reused because R1-F1 changes only Swift
presentation/tests and R1-F2 changes only topic documents; no Go source,
dependency, fixture, configuration, or toolchain input changed.

- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — re-review — 2026-08-31

- Reviewed state:
  - HEAD `7a1160a86e549a3ae3532bbfe8b782fdbfbfef82`, working tree uncommitted
  - Repair candidate manifest
    `d5e2af9dd1e4c336bdbff2ef791572e1536818048cc2f097260e9d07bba9bf12`,
    recomputed from the WorkUnit's `target_state_recipe` and matching the state
    the Repair evidence is bound to
  - changed since Round 1: `MenuBarPanelViews.swift` `3268da0e -> c5572ecf`,
    `MenuBarViewModel.swift` `f3b36862 -> 346eb7c6`, `AppTestFixtures.swift`
    `3fe1711a -> fb70f050`, `MenuBarViewModelTests.swift` `88b175d8 -> bd78a3fe`,
    `ux/session-work-signals.md` `a0d5342b -> 0b855621`, plus `tasks.md`,
    `docs/status.md`, and `reviews/documents.md`. `DesktopCopy.swift`,
    `Localizable.xcstrings`, `MenuBarChromeTests.swift`, and the acceptance
    record are byte-identical to Round 1, so that round's verification of copy,
    formatting, and the narrow renderings is reused.
- Reviewer: Claude Code, independent Re-review. The repair was produced by
  Codex; this round shares neither its context nor its role, and the repair's
  own account of itself was treated as input rather than as evidence.
- Method: each Round 1 finding was re-verified against the current content
  before being dispositioned. R1-F1 was not accepted on the new test passing —
  a passing test proves nothing about whether it can fail — so the reviewed
  defect was reproduced in place: the unanimity rule was reinstated as a single
  bounded edit to the state expression, the new regression test was run against
  it, and the file was then restored and verified byte-identical by SHA-256 and
  by `git hash-object`. R1-F2 was checked against the repository and the
  evidence store rather than against the repair note: the document blob, the
  frontmatter, the reworded Copy section, the Round 9 record, and the Document
  WorkUnit gate and its digest recipe were each read directly.
- Scope: the two changed production files, the two changed test files, the three
  changed topic documents, and the Document/Task completion evidence for this
  topic. Production code, tests, fixtures, and configuration are read-only; the
  one bounded mutation described above was reverted within this round.

## 📋 work-signal-surface Re-review report

📊 Overall score: 9/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

**[R1-F1] closed, and closed with a falsifier rather than with an assertion.**
`workSignalPanel` no longer decides anything by unanimity. Emptiness now comes
from the selected scope's session statistics (`MenuBarViewModel.swift:810`),
which is what the contract's `empty` row actually means, and each family is
tracked separately in `uncapturedSections` from its own `available` flag
(`:823-826`), with the per-scope item looked up only for a family that is
available (`:827-835`). A family that is captured but has no item for the
selected scope now renders `—` through the same formatters the contract already
relies on, and `.uncaptured` survives only when all three families are
unavailable (`:938`) — which is exactly the legacy payload the state was
reserved for. The three boundaries the finding named are asserted by
`testCapturedWorkSignalScopeKeepsEachMissingFamilyHonest`
(`MenuBarViewModelTests.swift:350-386`): tooling item missing, all three items
missing with sessions present, and one family unavailable. Reinstating the old
unanimity rule as a one-line edit made that test fail three times — at lines
357, 371, and 383, each `"uncaptured" is not equal to "captured"` — and the file
was restored byte-identical (`sha256
e58fd99b63555852ae99a5763fbab522c76106c77086bf8869c2e9c579db3b89`, blob
`346eb7c603d7d4c82c2ae7e1c3d18b1070af0b07`). The regression is therefore held by
a test that can fail, not by a description of one.

**The new emptiness predicate is the more faithful one, not merely a different
one.** `sessionStatistics` and `filteredRecentSessions` key on the same rule —
the session's last event inside the selected period, under the same client
filter — so a scope that hides the signal cards is the same scope in which the
panel shows its existing empty note. The old rule could hide the cards while the
panel above them listed sessions and spend; that contradiction is now
structurally unavailable.

**[R1-F2] closed on the repository, not on the note.** `updated: 2026-08-31` is
present; the Copy section now describes itself as the design delta against the
approved prototype dictionary and names the imported prototype labels
separately; `reviews/documents.md` Round 9 records the ratification and is
explicit that Codex recorded it rather than independently reviewing its own
work, which is the shape Round 1's remediation asked for. The document's
CEv1 gate is genuinely VERIFIED: its single required criterion carries `pass`
at content state `050f4faf…`, and that digest reproduces from the recorded
recipe — `printf '%s' 'head=7a1160a8…;document=0b855621…' | shasum -a 256`
returns it exactly. The new Copy paragraph is also complete rather than
approximate: the sixteen keys the delta table does not list are precisely the
Activity category names, the five Workflow metric and note labels, the six
Tooling labels, and the legacy pending hint — every one of them accounted for
by the categories the paragraph names.

**Nothing regressed.** The full scheme passes under an explicit English locale —
`AgentDeckSharedTests` 39/39, `AgentDeckAppTests` 59/59, `AgentDeckWidgetTests`
22/22, `** TEST SUCCEEDED **` — and the eight narrow renderings in
`MenuBarChromeTests` were re-executed against the changed views rather than
reused. The fixture's three new knobs all default to `false`, so every
pre-existing case runs on unchanged data.

### 📝 Summary

Finding-by-finding: **R1-F1 — closed**, verified by reproducing the defect and
watching the new test fail on it, then restoring the file byte-identically.
**R1-F2 — closed**, verified against the document blob, the frontmatter, the
rewritten Copy section, the Round 9 record, and the Document gate's own
recomputed digest. No finding is carried forward, none regressed, and none is
new.

The reviewed content is HEAD `7a1160a` plus the repair candidate `d5e2af9d…`.
The repair is narrow in the right way: it changed the state machine and the
tests that pin it, and left the captured layout, the copy values, the cost-basis
behavior, the narrow rendering, and Task 4's wire and producer untouched — all
verified byte-identical or re-executed rather than assumed.

One correction to this record's own Round 1: it reported thirty-five catalog
additions and twenty-one keys beyond the contract's table. The counts are thirty
and sixteen (`git diff` adds thirty `static let sessions…` declarations; the
delta table lists fourteen). The finding, its reasoning, and its remediation are
unaffected — the repaired paragraph cites no count — but the numbers are
corrected here rather than left standing in the audit trail.

Residual uncertainty, stated rather than resolved. Actual VoiceOver speech, TCC
changes, and system accessibility-setting automation were not executed in this
round either; by the operator's 2026-08-31 decision they are not completion
requirements, and the acceptance record says so plainly instead of implying they
passed, so every accessibility claim here remains structural and textual.
`AgentDeckTests` is not among the scheme's testables and was not run; it holds
`DesktopWireTests`, whose subject — Task 4's wire — is byte-identical to its
reviewed and committed state. `acceptance/work-signal-surface.md` stays
`Review [ ]` in the Documents matrix, which is this project's established
handling of an acceptance record — `desktop-app`'s
`acceptance/menubar-experience.md` sits the same way and its closing review
column reads `—`; it is a task artifact, not a design contract awaiting its own
round, and it is recorded as a non-finding rather than as a gap.

- Evidence:
  - falsifier: unanimity rule reinstated at
    `MenuBarViewModel.swift:938`, `-only-testing:AgentDeckAppTests/
    MenuBarViewModelTests/testCapturedWorkSignalScopeKeepsEachMissingFamilyHonest`
    -> 3 failures at lines 357/371/383; file restored, `git hash-object`
    `346eb7c6…` and `sha256 e58fd99b…` both match the pre-mutation copy.
  - `xcodebuild … -testLanguage en -testRegion US test` (Xcode 26.4 `17E192`)
    -> `** TEST SUCCEEDED **`; Shared 39, App 59, Widget 22, 0 failures.
  - the same scheme without `-testLanguage` -> 1 failure,
    `MenuBarViewModelTests.swift:454`, this machine's Chinese localization
    against a hardcoded English expectation; the English run above isolates it
    as environmental.
  - `bash scripts/check-topic-docs.sh` -> no gap for `work-signals`; the two
    reported gaps belong to the concurrent `schema-version-signal` topic.
  - `make check-whitespace` and `git diff --check` -> clean.
  - `git diff --name-only -- '*.go'` -> empty, so the Development L2 Go result
    stands for an unchanged Go tree.
  - `printf '%s' 'head=…;document=0b855621…' | shasum -a 256` ->
    `050f4faf…`, matching the Document content state whose single required
    criterion carries `pass`.
  - manifest recomputation -> `d5e2af9d…`, matching the Repair evidence binding.
- Completion gate: VERIFIED — `work-signals:work-signal-surface`. This round
  recorded independent Re-review `pass` evidence for all nine required criteria
  against the post-synchronization candidate
  `756a7b6a728e99e8c915263cabdf601563a4675b69dbb9dbad5b615d93941da4`,
  superseding the Repair evidence on `d5e2af9d…`, and the re-queried gate
  answers `pass` on all nine.
- Verdict: PASS
