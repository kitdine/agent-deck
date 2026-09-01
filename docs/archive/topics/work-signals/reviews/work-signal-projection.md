---
status: historical
topic: work-signals
subject: work-signal-projection
retired: 2026-09-01
---

# Review log — work-signals / work-signal-projection

## Round 1 — 2026-08-29

- Reviewed state:
  - HEAD `a13ede6f82be8313e2af0adfdb8934bc2c2ba5d5`, working tree uncommitted
  - scoped manifest recorded with this round's completion evidence
- Reviewer: Claude Code, independent formal Review after the Development route
  closed. The implementation was produced by Codex; this review shares neither
  its context nor its role.
- Method: contract-first review against `tasks.md` task 4 and architecture
  Decision 9. The Go producer, the Swift decoder, both test files, and all four
  canonical fixtures were read directly, and the fixtures were decoded to confirm
  the keyed shape and the Client × Period coverage rather than inferring it. The
  project's two checkers for this class were run: `scripts/run-go-test.sh ./...`
  and `scripts/test-macos-app.sh`. **`xcodebuild` is unavailable on this machine**
  — `xcode-select` points at Command Line Tools — so the XCTest target could not
  be executed here and the wrapper took its documented `AgentDeckFoundationVerifier`
  fallback. The Swift-side finding below is therefore established from Swift's
  synthesized-`Codable` semantics plus the absence of any assertion, both verified
  by inspection, rather than from an executed mutation. That limitation is named
  here so a later reader can tell an unrun check from a passing one.
- Scope: the task 4 producer, Swift decoder, canonical fixtures, both test files,
  the derived phase7 GUI contract entry, `tasks.md`, and `docs/status.md`.
  Production code, tests, configuration, and fixtures were read-only; three
  fixtures were briefly reverted to HEAD to isolate a pre-existing verifier
  failure and restored byte-identical by SHA-256.

## 📋 work-signal-projection Review report

📊 Overall score: 7/10

✅ Verdict: FAIL

### 🔴 Serious issues — must fix

[`apps/macos/AgentDeckShared/DesktopWire.swift:860`,
`apps/macos/AgentDeckTests/DesktopWireTests.swift:52`] **[R1-F1] Eight wire field
names — the whole Workflow module and the Tooling module's MCP pair — decode into
optionals that nothing asserts, so a name drift silently yields `nil` instead of
failing.**

- Behavior risk: the task's acceptance requires "three families present with
  identical field names on both sides". For the required keys that identity is
  enforced structurally: Swift decodes the producer-generated fixture, so a
  renamed `period`, `client`, `cost_basis`, `kinds`, `calls`, `groups`, or `rows`
  throws. Six workflow metrics (`first_edit_seconds`, `files_touched`, `retries`,
  `edits_per_session`, `top_file`, `top_file_edits`) and the tooling pair
  (`top_mcp_server`, `top_mcp_calls`) are declared `Int?`/`Double?`/`String?`, and
  Swift's synthesized `Codable` decodes a missing key for an `Optional` through
  `decodeIfPresent` — nil, no error. No test reads any of those eight values. A
  developer who renames a producer tag and regenerates the fixture — the normal
  way this changes — leaves the whole repository green while the panel's WORKFLOW
  module renders all five rows as `—` and the TOOLING module drops its MCP line.
  `ux/session-work-signals.md` gives `—` the meaning *undeterminable*, so the
  failure presents as an honest "no data" state while the data exists, which is
  harder to notice than a crash and is the exact confusion Decision 6's
  pointer-per-metric design was built to prevent.
- Evidence: the eight properties are optional in `DesktopWire.swift`
  (`firstEditSeconds: Int?` … `topMCPCalls: Int64?`). Grepping the test target for
  `firstEdit|filesTouched|editsPerSession|topFile|retries` returns nothing, and
  for `topMCP` returns only the four declaration and `CodingKeys` lines in
  `DesktopWire.swift` itself — no assertion anywhere. `DesktopWireTests` asserts
  family availability, the nine `period/client` keys, `kinds.count == 4`, and
  `groups == rows.count`, and stops there. The second mechanism that could have
  caught it does not either: the phase7 GUI contract captured
  `work_signals` with `"items": []` for all three families, so it pins
  `available` and the presence of `items` and no item-level name at all. The
  fixture does carry the values — `top_mcp_server: "codegraph"` and
  `top_mcp_calls: 1` in six of nine tooling positions — so this is unasserted
  data, not absent data.
- 💡 Bounded remediation: assert the decoded values for known fixture positions in
  `DesktopWireTests` — the workflow item seeded with an edit (`today/all` and
  `today/codex` carry `main.go`) for the six metrics, and a `claude` position for
  `topMCPServer == "codegraph"` / `topMCPCalls == 1`. A renamed producer tag then
  decodes to nil and the assertion fails. Test-only; no production or wire change,
  and the names are already correct.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- The families are genuinely keyed item lists, not one object per family, and the
  complete fixture carries all nine `period × client` positions in the fixed
  order. Decision 9's reason for the shape — a single unkeyed set cannot answer
  two filter positions — is satisfied rather than approximated.
- The bounds are producer-enforced and fail closed. `loadWorkSignals` rejects an
  activity item without exactly four kinds, a kind with more than four
  subcategories, and a tooling item whose `groups` disagrees with `len(rows)` or
  exceeds five, warning `work_signals_unavailable` instead of emitting a payload
  that violates the contract.
- `wire_version` is untouched, and the legacy path is correct in both directions:
  `snapshot-legacy.json` retains no `work_signals`, `decodeIfPresent(…) ?? .unavailable`
  turns that into an unavailable family rather than an error, and a malformed
  `work_signals` still throws — the new `work-signals` mutation case in the
  malformed-payload test covers that.
- Field names are shared rather than restated: `WorkSignalActivityItem.Kinds` and
  `WorkSignalToolingItem.Rows` reuse `usage.SignalActivityKind` and
  `usage.SignalToolRow` directly, so the CLI's JSON and the wire projection cannot
  drift apart in the nested structures at all.
- The unavailable path is initialized rather than left to a zero value.
  `emptySessionsSnapshot` seeds `emptyWorkSignalsSnapshot()`, so a failed store
  open, a bound violation, or an unavailable session index all render
  `items: []` rather than a null, and `loadSessions` preserves what
  `loadWorkSignals` computed instead of overwriting it.
- The empty-client fixture omits the three positions with no data rather than
  presenting synthetic zeros, which is what Decision 9 permits and what keeps a
  measured `0` distinct from an absent measurement.

### 📝 Summary

The reviewed candidate implements Decision 9's shape faithfully — keyed families,
enforced bounds, an unchanged wire version, a correct legacy default, and nested
types shared with the CLI so the two surfaces cannot disagree where it matters
most. The blocking finding is the same class the last two tasks closed: an
acceptance criterion held by inspection instead of by a check that can fail.
Here it is narrower than task 3's but its failure mode is worse — a drift on any
of eight names does not break the payload, it blanks the panel's entire Workflow
module while every test stays green.

Residual uncertainty is named rather than deferred. `xcodebuild` is unavailable
on this machine, so the XCTest target was not executed in this review; the
development report records 39/39 focused Swift tests passing, and that report is
input, not evidence. The Go suite and both fixture-parity paths were executed
here. The CLT fallback verifier fails on `expected two index refreshes followed by
one snapshot read` — reverting the three fixtures to HEAD reproduces the identical
failure, and the candidate touches neither `AgentDeckVerification` nor
`EmbeddedHelperRunner`, so it is pre-existing and outside this task. It is
recorded here because it is the reason the macOS wrapper does not currently exit
zero, and a later reader should not attribute that to task 4.

### Evidence

```text
scripts/run-go-test.sh ./...
  -> PASS
scripts/check-privacy.sh / check-topic-docs.sh / go vet ./...
  -> exit 0 / clean
scripts/check-whitespace.sh
  -> exit 0 (an earlier failure was this review's own apps/macos/.build output,
     which scripts/test-macos-app.sh writes into the worktree untracked and
     un-ignored; it was removed)
scripts/test-macos-app.sh
  -> FAIL "expected two index refreshes followed by one snapshot read"
     identical failure with the three fixtures reverted to HEAD -> pre-existing
xcodebuild -scheme AgentDeck test
  -> NOT RUN: xcode-select points at Command Line Tools on this machine
fixture decode:
  snapshot-complete.json      -> 9/9 period x client positions per family
  snapshot-empty-client.json  -> 6 positions, the empty client omitted
  snapshot-partial.json       -> available:false, items:[] on all three
  snapshot-legacy.json        -> work_signals absent, decodes .unavailable
grep for assertions on the eight optional fields
  -> none in apps/macos/AgentDeckTests   <- R1-F1
fixtures reverted to HEAD and restored, verified by sha256
  -> c955c2bd / f9760026 / 56a6ac1c
```

- Completion gate: FAILED — R1-F1 disproves the `swift-decoding-and-legacy`
  criterion for this candidate content state. The prior Development `pass`
  evidence remains an immutable record of what was checked.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-30

- Repairer: Codex
- Scope: R1-F1 only. Production code, wire schema, canonical fixtures, the
  derived GUI contract, and unrelated tests are unchanged.
- Method: confirmed the eight optional Swift properties and their populated
  producer-fixture positions, then added exact decoded-value assertions to the
  existing complete-fixture test. No optional was made required because null is
  a valid per-metric state; the test protects field identity without changing
  the wire contract.
- Repaired state: HEAD `a13ede6f82be8313e2af0adfdb8934bc2c2ba5d5`,
  working tree uncommitted; the synchronized candidate is bound by the Repair
  completion-evidence record.

### R1-F1 — CLOSED

`testPeriodScopedAttributionAndSessionsDecodeForEveryPeriod` now unwraps the
complete fixture's `today/codex` Workflow item and asserts
`firstEditSeconds == 0`, `filesTouched == 1`, `retries == 0`,
`editsPerSession == 1.0`, `topFile == "main.go"`, and `topFileEdits == 1`.
It also unwraps the `today/claude` Tooling item and asserts
`topMCPServer == "codegraph"` and `topMCPCalls == 1`.

Those are the eight optional keys named by the finding. If a Go JSON tag is
renamed and the fixture is regenerated, synthesized `Codable` still yields
`nil`, but the corresponding assertion now fails. The correction is therefore
test-only and falsifies the exact silent-drift scenario without rejecting valid
null metric values.

### Evidence

```text
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild ...
  -only-testing:AgentDeckSharedTests/DesktopWireTests/
  testPeriodScopedAttributionAndSessionsDecodeForEveryPeriod test
  -> PASS
scripts/run-go-test.sh ./...
  -> PASS
bash scripts/test-macos-app.sh
  -> product build PASS; verifier FAIL
     "expected two index refreshes followed by one snapshot read"
     same pre-existing failure isolated by Review Round 1
```

The default Command Line Tools path has no XCTest module, so the focused test
used the installed full Xcode by command-local `DEVELOPER_DIR`; no system
selection or environment configuration was changed.

- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — re-review — 2026-08-30

- Reviewed state:
  - HEAD `a13ede6f82be8313e2af0adfdb8934bc2c2ba5d5`, working tree uncommitted
  - the Repair round's 12-path manifest was recomputed independently from the
    working tree before this round wrote anything, and reproduced the digest the
    Repair evidence is bound to, so the content judged here is exactly the
    content the repair recorded
  - this round's own scoped manifest is recorded with its completion evidence
- Reviewer: Claude Code, independent Re-review. The repair was produced by Codex;
  this round shares neither its context nor its role, and its report was treated
  as input rather than as evidence.
- Method: R1-F1 was re-checked against the current content rather than against
  the repair narrative — the eight assertions were read in place, the two fixture
  positions they name were decoded, and the Swift `CodingKeys` were compared to
  the producer tags. The finding's failure mode was then falsified directly: the
  three producer tags `top_file`, `top_mcp_server`, and `first_edit_seconds` were
  renamed in `snapshot-complete.json`, the focused test was run, and it failed on
  exactly the three assertions with `("nil") is not equal to ("Optional(...)")`.
  The fixture was restored from a byte backup and re-verified by SHA-256.
  **Round 1's residual uncertainty is closed rather than carried:** `xcodebuild`
  is still not the selected toolchain on this machine, but the installed full
  Xcode was addressed by a command-local `DEVELOPER_DIR`, so the XCTest target
  ran here for the first time — 39/39 in `AgentDeckSharedTests`, and the
  aggregate scheme as well. No system selection was changed.
- Scope: R1-F1 and the content it names. The production side was confirmed
  unchanged since Round 1 by diff and by the recomputed manifest, so Round 1's
  verification of the other eight criteria is reused rather than rerun.
  Everything remained read-only apart from the fixture mutation described above,
  which was restored byte-identical.

## 📋 work-signal-projection Re-review report

📊 Overall score: 9/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- **[R1-F1] CLOSED — and closed by a check that was shown to fail, not argued to
  fail.** `testPeriodScopedAttributionAndSessionsDecodeForEveryPeriod` now unwraps
  the complete fixture's `today/codex` Workflow item and asserts all six optional
  metrics, and its `today/claude` Tooling item for `topMCPServer` and
  `topMCPCalls`. Renaming three producer tags in the fixture produced three
  assertion failures rather than a green suite, which is the exact scenario the
  finding described: the eight names are now held by a falsifier instead of by
  inspection.
- The repair kept the wire contract intact. No optional was promoted to required,
  so a genuinely absent metric still decodes as `nil` and still renders `—` with
  the meaning `ux/session-work-signals.md` gives it. The correction protects field
  identity without narrowing the value domain — a narrower fix here would have
  traded a silent-drift bug for a spurious decode failure on honest missing data.
- The repair stayed inside its boundary. `DesktopWire.swift`, the three producer
  fixtures, the derived phase7 GUI contract, and the Go producer are unchanged;
  the diff since Round 1 is the eight assertions and their two `XCTUnwrap`
  bindings, and the recomputed manifest confirms it rather than taking it on
  report.
- Round 1's unrun check is now run. The XCTest target that could not execute
  during Round 1 executes and passes here, which converts the development
  report's 39/39 claim from an input into a verified result.

### 📝 Summary

One finding was recorded and one finding is closed. R1-F1 was the whole of the
disposition matrix, and it closes on evidence rather than on the repair's account
of itself: the assertions exist at the named positions, the fixture carries the
values they expect, and a simulated producer-tag rename makes them fail. Nothing
was carried forward, nothing regressed, and no new blocking finding was raised.

Two observations are recorded as facts about the environment rather than as
findings against this task, because neither is on the candidate's path. The
aggregate Xcode scheme fails one App test,
`MenuBarViewModelTests.testProviderWithMultipleReadyTargetsUsesOneRowAndASecondLevel`,
whose hardcoded English `wrapper` / `direct` expectation receives this machine's
Chinese localization; the strings come from the unmodified
`AgentDeckApp/Localizable.xcstrings`, and the candidate's `DesktopWire.swift`
diff is purely additive work-signal structures with no deletion, so no task 4
code is on that path. And `scripts/check-topic-docs.sh` now exits 1 on
`schema-version-signal`, an untracked topic directory created on this machine at
07:39 local — after the repair recorded that same checker passing — by concurrent
work on another topic. It is outside this candidate's manifest and does not
touch it.

Residual uncertainty is small and named. The CLT fallback verifier still fails
the `expected two index refreshes followed by one snapshot read` assertion that
Round 1 isolated as pre-existing by reverting the fixtures to HEAD; the candidate
touches neither `AgentDeckVerification` nor `EmbeddedHelperRunner`, and this
round's Xcode run confirms the XCTest path is clean, so the fallback verifier
remains the machine's problem and not this task's. The score rises from 7 to 9
rather than to 10 because the acceptance clause is now protected at two fixture
positions, not at every position that carries the eight values.

### Evidence

```text
manifest digest recomputed from the working tree before this round wrote
  -> matches the Repair round's recorded candidate state
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild
  -only-testing:AgentDeckSharedTests/DesktopWireTests/
  testPeriodScopedAttributionAndSessionsDecodeForEveryPeriod test
  -> PASS
same, with "top_file" / "top_mcp_server" / "first_edit_seconds" renamed in
  snapshot-complete.json
  -> FAILED, 3 assertions:
     DesktopWireTests.swift:73 ("nil") is not equal to ("Optional(0)")
     DesktopWireTests.swift:77 ("nil") is not equal to ("Optional("main.go")")
     DesktopWireTests.swift:83 ("nil") is not equal to ("Optional("codegraph")")
  fixture restored from backup, sha256 c955c2bd... unchanged, 0 "renamed" tokens
DEVELOPER_DIR=... xcodebuild -only-testing:AgentDeckSharedTests test
  -> ** TEST SUCCEEDED **, 39/39, DesktopWireTests 11/11
DEVELOPER_DIR=... xcodebuild -scheme AgentDeck test
  -> 1 failure, AgentDeckAppTests/MenuBarViewModelTests localization, pre-existing
scripts/run-go-test.sh ./...
  -> PASS
go vet ./... / scripts/check-privacy.sh / scripts/check-whitespace.sh
  -> clean / 0 / 0
scripts/check-topic-docs.sh
  -> exit 1 on schema-version-signal, an untracked topic created concurrently
     at 07:39 local, outside this candidate's manifest
```

- Completion gate: VERIFIED — nine of nine criteria carry `pass` evidence bound
  to this round's post-synchronization content state.
- Verdict: PASS
