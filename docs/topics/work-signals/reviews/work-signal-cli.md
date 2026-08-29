---
status: active
topic: work-signals
subject: work-signal-cli
---

# Review log — work-signals / work-signal-cli

## Round 1 — 2026-08-29

- Reviewed state:
  - HEAD `276986e2b97b39872411f4fdb2d49794ae696739`, working tree uncommitted
  - scoped manifest `3265b5efc772bfc8956c5a611801f362cc55e63bf98d79f83d8581044dae675b`
- Reviewer: Claude Code, independent formal Review after the Development route
  closed. The implementation was produced by Codex; this review shares neither
  its context nor its role.
- Method: contract-first review against `tasks.md` task 3, `ux/cli-work-signals.md`,
  and architecture Decisions 9 and 10. CodeGraph located the existing usage
  render and command-wiring paths; the new report, text, and command files were
  then read directly. Rendered output was verified by building the binary and
  running `usage stats`, `usage signals` (default, `--sub`, `--activity`,
  `--kind` repeated, empty scope), and `session show --activity` against a
  constructed session log, because this contract is about what the commands
  print and source inspection alone cannot confirm section order or column
  alignment. The project's own JSON-contract checker
  (`cmd/agentdeck/contract_test.go` plus the `gui-json-contract.json` fixture)
  was inspected for what it holds for this command. One bounded mutation of two
  `json:` tags, reverted immediately and verified byte-identical by SHA-256,
  tested whether the field-name criterion has a falsifier.
- Scope: the task 3 implementation, tests, the GUI JSON contract fixture,
  `tasks.md`, and `docs/status.md`. Production code, tests, configuration, and
  fixtures were read-only apart from the reverted mutation named above.

## 📋 work-signal-cli Review report

📊 Overall score: 8/10

✅ Verdict: FAIL

### 🔴 Serious issues — must fix

[`cmd/agentdeck/testdata/phase7/gui-json-contract.json:2372`,
`cmd/agentdeck/e2e_test.go:75`] **[R1-F1] The `usage signals` JSON field names
have no falsifier, so the acceptance criterion they satisfy cannot fail.**

- Behavior risk: the task's acceptance criteria require that "JSON field names
  and units match the wire projection exactly", and Decision 9's closing
  paragraph plus the CLI contract's "Reproducibility against the panel" section
  make that identity the whole point — a figure read in the panel and the same
  figure read from this JSON must have the same name. Task 4 implements the wire
  side independently from Decision 9. Today the names are correct, but nothing in
  the repository would fail if one drifted, so the divergence between the two
  surfaces would first surface when a person compares them by eye — which is
  exactly the manual check the contract exists to replace.
- Evidence: the fixture registers the command with `"success_schema": null`,
  because `e2e_test.go`'s scripted walk never calls
  `runJSON("usage.signals", …)`; `contract.Success` is only ever assigned from a
  live invocation, so an unvisited command records nothing. The seven other null
  entries are mutating or environment-dependent commands
  (`extension.enable`, `usage.hook.*`, `desktop.refresh-indexes`);
  `usage.signals` is a pure read like `usage.stats`, whose success schema *is*
  captured. No test asserts any inner field name — every grep hit for
  `first_edit_seconds`, `top_mcp_server`, `edits_per_session`, and `cost_basis`
  in `*_test.go` is a failure-message string, not an assertion on emitted JSON.
  The decisive check:

  ```text
  # renamed json:"top_mcp_server" -> "mcp_server_top"
  #         json:"first_edit_seconds" -> "first_edit_secs"
  scripts/run-go-test.sh ./...
    -> PASS (exit 0)
  ```

  Two wire-projection field names were renamed and the entire repository suite
  still passed. The file was restored and re-verified at SHA-256
  `64189bc494e2fab9cc6b5c6372e8980f57d46db48f2adc7c8cb3f5b93980b2b3`.

💡 Bounded remediation: add one `runJSON("usage.signals", …)` call to the
`phase7` end-to-end walk at a point where the scanned state produces a populated
payload, and regenerate the fixture so `usage.signals` carries a real
`success_schema`. That places the field names under the checker the project
already ships for this exact class. No production behavior changes; the names
are already correct.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- Section placement is exactly what the contract specifies, confirmed from
  rendered output rather than argued from source: `▦ ACTIVITY BY WEEKDAY / HOUR`,
  then `🧭 WORK KIND`, `🧱 WORKFLOW`, `🔧 TOOLING`, then `COVERAGE`. Reaching it
  required lifting `coverage` out of the responsive panel grid, which the
  implementation does only while the signal sections are present, leaving the
  interactive viewer and the `usage stats` JSON payload untouched as Decision 10
  and the CLI contract require.
- `--activity` renormalizes where the contract says it must and nowhere else:
  the filter is applied to events and turns before the cost fold, so
  `--activity debugging` renders `Debugging 100%` with `Repair 100%` beneath it
  while cost and event counts stay the filtered scope's real values.
- The three availability conditions each render their own documented form and
  exit `0` — a whole-section message for Activity and Tooling, all five Workflow
  rows present with `—`. `—` and `0` stay distinct end to end, and
  `WorkflowMetrics`' pointer-per-metric design is what makes that distinction
  representable rather than conventional.
- The privacy boundary is enforced structurally and tested adversarially: the
  most-touched file reaches the surface as `usage_tool_files.base_name`, and
  `TestUsageSignalsAndSessionShowReadTheSameSafeDerivation` asserts the absolute
  path, its parent directory, the user's message text, and the `file_path` key
  are all absent from real rendered output.
- `session show --activity` omits the `SIGNALS` line entirely when no signal row
  exists, and drops the leading category word when `cost_basis` is `none`,
  because `SessionActivityCategory` returns early without setting `Kind` rather
  than letting a dominant-kind fold guess one.
- Field names and units do match Decision 9 exactly where they are emitted —
  verified field by field against the projection. R1-F1 is about the absence of a
  test that could fail, not about a name being wrong.

### 📝 Summary

The reviewed candidate implements task 3's contract accurately: every rendering,
flag, filter, availability state, and privacy rule the CLI document specifies was
checked against real output and holds. The one blocking finding is not a defect
in what the command prints but in what the repository can prove about it — the
acceptance criterion that binds this surface to the wire projection is asserted
by inspection alone, and a two-tag mutation passed the full suite. Under this
project's rule that an argument cannot fail when the structure changes, that
criterion needs the checker the project already ships. Residual uncertainty is
limited to the cross-surface figure guarantee, which cannot be fully tested until
task 4 exists; R1-F1's repair is what will make that comparison mechanical rather
than manual.

### Evidence

```text
scripts/run-go-test.sh ./...
  -> PASS (candidate as delivered)
scripts/check-privacy.sh / check-whitespace.sh / check-topic-docs.sh
  -> exit 0
go vet ./...
  -> clean
built binary, constructed session log:
  usage stats --period all
    -> ▦ ACTIVITY BY WEEKDAY / HOUR, 🧭 WORK KIND, 🧱 WORKFLOW, 🔧 TOOLING, COVERAGE
  usage signals --period all --sub
    -> four categories in Decision 3 order, subcategories under └, no bar
  usage signals --period all --activity debugging --kind activity
    -> Debugging 100% / └ Repair 100%
  usage signals --period all --client codex          (empty scope)
    -> per-section emptiness forms, all five workflow rows "—", exit 0
  usage signals --format json                        (empty scope)
    -> available:false on all three families, usage envelope, exit 0
  session show rev-1 --client claude --activity
    -> SIGNALS  Debugging · 2 tool calls · 1 file · first edit 4s
scripts/run-go-test.sh ./...                         (two json tags renamed)
  -> PASS  <- R1-F1: the criterion has no falsifier
```

- Completion gate: FAILED — R1-F1 disproves the `json-contract` criterion for
  this candidate content state. The prior Development `pass` evidence remains an
  immutable record of what was checked and is superseded rather than rewritten.
- Verdict: REOPEN

### Repair disposition — 2026-08-29

- R1-F1 closed: the phase7 synthetic Codex log now contains one classified user
  turn and one MCP tool call, and the end-to-end walk invokes
  `usage signals --period all --sub` immediately after `usage scan`. Regenerating
  the canonical fixture through `TestIsolatedEndToEndFlow` replaces the null
  `usage.signals.success_schema` with a populated schema covering Activity
  `kinds[].sub[]`, all Workflow field names, Tooling `rows[]`,
  `top_mcp_server`, and `top_mcp_calls`. Production behavior and JSON names are
  unchanged.
- Evidence: with the new invocation and the old fixture,
  `TestIsolatedEndToEndFlow` failed on the command-contract comparison. After
  producer regeneration, `TestGUIJSONContractFixture` and
  `TestIsolatedEndToEndFlow` pass together. A bounded mutation of
  `top_mcp_server` to `mcp_server_top` and `first_edit_seconds` to
  `first_edit_secs` then made the same E2E test fail and printed both wrong names
  in the actual schema. Restoring the tags returned
  `internal/usage/signals_report.go` to SHA-256
  `64189bc494e2fab9cc6b5c6372e8980f57d46db48f2adc7c8cb3f5b93980b2b3`,
  the focused checks pass again, and `scripts/run-go-test.sh ./...` passes on the
  restored candidate.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 2 — 2026-08-29

- Reviewed state:
  - HEAD `276986e2b97b39872411f4fdb2d49794ae696739`, working tree uncommitted
  - scoped manifest recorded with this round's completion evidence
- Reviewer: Claude Code, independent formal Re-review. Round 1's reviewer and
  this round's are the same role but the repair was performed by Codex; the
  finding's closure was re-derived rather than accepted from the repair report.
- Method: finding-by-finding disposition against R1-F1 only. The repair's own
  claim that the mutation now fails was not taken as evidence — the identical
  two-tag rename from Round 1 was re-applied to `internal/usage/signals_report.go`
  and the checker re-run, then the file was restored and re-verified by SHA-256.
  The repair's containment was checked by comparing every production file's diff
  statistic and every untracked file's length against Round 1, so Round 1's
  verification of the other ten criteria is reused rather than repeated.
- Scope: R1-F1 and any regression caused by the repair. Production code, tests,
  configuration, and fixtures were read-only apart from the reverted mutation.

## 📋 work-signal-cli Re-review report

📊 Overall score: 9/10

✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- **R1-F1 — CLOSED.** The `phase7` walk now scans a Codex log carrying a
  `user_message` and an `mcp_tool_call`, so `runJSON("usage.signals", …)` records
  a populated payload rather than an empty one. The regenerated fixture pins every
  field name Decision 9 specifies: `cost_basis`, `kinds[]{kind, share, cost,
  events}`, `sub[]`, all seven workflow keys, and `rows[]{kind, calls, share}`
  with `top_mcp_server` and `top_mcp_calls` — the last two only reachable because
  the MCP call was added, since both are `omitempty` and would otherwise have
  vanished from the captured schema.
- The falsifier was confirmed to work rather than assumed. Re-applying Round 1's
  exact rename produced a named failure:

  ```text
  e2e_test.go:288: command contracts differ; actual contracts:
    "usage.signals": {
        "mcp_server_top": "string",
        "first_edit_secs": null,
  ```

  The checker does not merely fail; it prints the drifted names, which is what
  makes the next reader's diagnosis immediate.
- The repair is contained. The fixture diff is 71 insertions and zero deletions,
  and `usage.signals` is the only command entry it touches — adding two events to
  the shared `phase7` log perturbed no other captured schema. Every production
  file's diff statistic and every untracked file's length is identical to Round 1,
  so no production behavior changed and Round 1's findings on the other criteria
  stand unaltered.
- Round 1's ten remaining criteria required no re-verification under the
  reuse rule, and re-running the full suite confirmed the repaired candidate is
  green end to end.

### 📝 Summary

Finding-by-finding disposition: **R1-F1 — closed**; no finding carried forward,
regressed, or newly raised. The acceptance criterion binding this surface's JSON
field names to Decision 9 is now held by the checker the project already ships,
and the closure was established by reproducing Round 1's own falsification
attempt and watching it fail this time. The repair changed only test data and one
end-to-end invocation, leaving every rendering, flag, filter, availability, and
privacy behavior Round 1 verified exactly as it was.

Residual uncertainty, named rather than deferred: the five workflow metrics are
captured as `null` in the `phase7` scenario, so their key names are pinned but
their value types are not — a `*int` to `*string` change on those five would pass.
This is not recorded as a finding because the criterion is about field names and
units, the names are pinned, and Decision 9's decode on the panel side will bind
the types once task 4 exists. The cross-surface figure guarantee itself remains
untestable until that task lands, which is a property of the schedule rather than
of this candidate.

### Evidence

```text
scripts/run-go-test.sh ./...                          (repaired candidate)
  -> PASS
scripts/check-privacy.sh / check-whitespace.sh / check-topic-docs.sh
  -> exit 0
go vet ./...
  -> clean
go test ./cmd/agentdeck -run TestIsolatedEndToEndFlow  (two json tags renamed)
  -> FAIL, naming "mcp_server_top" and "first_edit_secs"   <- R1-F1 closed
internal/usage/signals_report.go restored and verified
  -> sha256 64189bc494e2fab9cc6b5c6372e8980f57d46db48f2adc7c8cb3f5b93980b2b3
containment: fixture diff +71/-0, only the usage.signals entry;
  production diff statistics and untracked file lengths identical to Round 1
```

- Completion gate: VERIFIED — all eleven criteria are re-bound to this round's
  final synchronized candidate. The Round 1 `fail` on `json-contract` remains an
  immutable record of the superseded state.
- Verdict: PASS
