---
status: active
topic: work-signals
subject: work-signal-extraction
---

# Review log — work-signals / work-signal-extraction

## Round 1 — 2026-08-27

- Reviewed state:
  - HEAD `9472b05d8adc2d4f16e4aafbef6625682ebd5f46`, working tree uncommitted
  - `internal/activity/extraction.go` `1e3f49e2400617f01c17126fcc44fcf0ad96c58c` (new)
  - `internal/activity/activity.go` `6f974a51b33c0499cb8a189351aed2e6c8c465ea`
  - `internal/store/migrations.go` `d4e2fc656a0872dfa579a6f1e5c9fa22736fced3`
  - `internal/usage/usage.go` `d743f85483ca821eef646da8add6827105d43528`
  - `internal/usage/usage_test.go` `82c9465c13d4f3de442b7326bb01084471f2cd7f`
- Reviewer: claude-code, independent of the implementer
- Method: contract-first. Read Decisions 1, 2, 7 and 8 and `tasks.md` task 1,
  then checked each clause against the delivered source rather than against the
  development note. Ran the affected packages, the fixture and CLI packages,
  full `vet`, and the privacy script. For the finding below, the structural claim
  it rests on was verified independently: every `fmt.Errorf` in `internal/usage`
  and `internal/activity` was read to confirm what it can carry, and the
  `Warnings` slices were traced to their only writers.
- Scope: schema v20, `internal/activity` extraction and turn segmentation,
  `internal/usage` persistence and parser-version backfill, the two canonical
  desktop fixtures, and the tests for all of it. Production code, tests,
  configuration, and fixtures remained read-only.

### Findings

#### P1

- **[R1-F1] The negative test asserts four of the six surfaces its own
  completion criterion names, and the two it omits are covered by argument
  rather than by assertion.** `internal/usage/usage_test.go`
  `TestUsageParserVersionBackfillsWorkSignalExtractionWithoutSensitiveContent`
  checks the five sentinels against the database text, the `Page` JSON, the scan
  result JSON, and the source-file cache. `tasks.md` task 1 and Decision 2 both
  require **emitted log lines** and **error and warning strings** as well.
  - Behavior risk: not a live leak — see Evidence — but the guarantee is
    unprotected on those two surfaces, which is the state Decision 2 names when
    it says the narrowed boundary "is worth nothing unasserted". Any later change
    that formats a tool argument into an error or a scan warning would ship
    green. The compounding problem is that the CEv1 criterion
    `work-signals:work-signal-extraction / privacy-boundary` states the six
    surfaces and already carries a `pass`, so the gate reports a verification
    that was never performed on two of them.
  - Evidence: the recorded evidence check for that criterion names its own
    scope as "database tables, source cache, scan output, and Page/Detail JSON"
    and substitutes, for the rest, the structural claim that "the parse path has
    no logger/warning/error sink for tool input". That claim is currently true
    and was verified here rather than taken on trust: every `fmt.Errorf` in
    `internal/usage/usage.go` carries only a source path, a timestamp, a model or
    catalog field, or a wrapped `err`; `internal/activity`'s four error sites are
    fixed strings plus a wrapped `err`; the only `Warnings` writer in the scan
    path appends the constant `"defaulted 5m cache creation TTL"`. Structural
    truth today is what the test is supposed to hold true tomorrow.
  - Bounded remediation: extend the existing negative test to capture the two
    remaining surfaces and assert the same five sentinels as substrings against
    them — the scan's emitted diagnostics and warning strings, and the error
    text produced when the same source is made to fail (a truncated or malformed
    line in the sentinel-bearing fixture is enough to exercise the error path
    without a second fixture). No production change is implied. Then re-record
    the `privacy-boundary` evidence so its check matches the criterion's
    statement.

#### P2

None.

### What was checked and found sound

Recorded so a later round does not re-derive it.

- **Decision 8 / schema.** Migration `version: 20` matches the decision's DDL
  clause for clause: three columns on `usage_tool_calls`, `turn_index` on
  `usage_events`, `usage_tool_files` with the composite primary key and the
  digest index, and `usage_work_signals` with both indexes. The number was read
  from the code rather than predicted, which is what R4-F1 on the document set
  asked for. `usage_work_signals` is created and not written, correctly: task 2
  owns writing it.
- **Parser version and backfill.** `usageParserVersion` 4 → 5 with a comment
  stating why re-reads are required. The negative test exercises the real
  backfill path by nulling `turn_index`, deleting the tool calls, and rolling
  the stored parser version back before re-scanning.
- **Canonical fixtures.** Exactly one count replacement per file, `19` → `20`,
  and nothing else. `TestCanonicalFixturesAreReproducibleProducerOutput` passes.
  This is the acceptance clause task 1 gained after the document round, and it
  is met in the shape it names.
- **Decision 1 / turn boundaries.** Codex increments on a changed `turn_id`;
  Claude opens a turn only through `ClaudeUserTurnBoundary`, which excludes
  `tool_result` continuations, `isMeta`, and the synthetic wrapper prefixes, and
  the index advances only when an assistant entry follows — the "no assistant
  API call, no turn" rule. Session changes reset the counter on both clients.
  The pending-user state is persisted through the append cursor and has its own
  test.
- **Decision 2 / what is retained.** `Record` gained `TurnIndex`, `ToolKind`,
  `MCPServer`, and `Files`, and nothing that carries raw input. `File` holds only
  the digest, the base name, and the write direction. `base_name` is truncated to
  128 bytes on a UTF-8 boundary. The digest is `sha256(machine identity ‖ path)`,
  and `collectFiles` returns nothing when the machine identity is unavailable,
  so an unsalted digest cannot reach the database. Salt variance is asserted:
  the same path under `machine-a` and `machine-b` yields different digests.
- **Decision 2 / comments.** The package doc comment and the `Record` comment
  are both rewritten to state what is *retained*; `Detail`'s comment is
  untouched, which is what the decision specifies.
- **Decision 7 / tool kinds.** The mapping is a table, not a heuristic, and
  unknown tools land in `other`. Codex `mcp` keys on the item type
  `mcp_tool_call`, not on a tool name, which is the row that reads differently
  from the others. Shell calls resolve to the strongest kind among their parsed
  commands — any written path makes the call `edit`, all-read-shaped makes it
  `read`, and everything else falls to `bash`, so the unclassifiable direction
  is `bash` and never `edit`.
- **Codex extraction paths.** All three are present and tested: `apply_patch`
  headers, `exec_command`'s `cmd`, and `tools.exec_command({cmd: …})` literals
  inside an `exec` payload, the last through a brace-matching scan that respects
  quoting and escapes.

### Evidence

```text
scripts/run-go-test.sh ./internal/activity/... ./internal/store/... ./internal/usage/...
  -> PASS (log agentdeck-go-test.zyWb89)
scripts/run-go-test.sh ./internal/desktop/... ./cmd/...
  -> PASS (log agentdeck-go-test.fuFCUU)
make vet          -> exit 0
check-privacy.sh  -> exit 0
```

Source inspection for R1-F1: all `fmt.Errorf` sites in `internal/usage/usage.go`
and `internal/activity`, and every writer of a `Warnings` slice reachable from
the scan path.

- Completion gate: FAILED — `work-signals:work-signal-extraction` carries seven
  `pass` records at content state `4f92ca8a`, but the `privacy-boundary`
  criterion's statement covers six surfaces while its own recorded check covers
  four. A current-state fail record is bound to that criterion; the remaining six
  criteria are unaffected by this finding.
- Verdict: REOPEN

## Round 1 — repair — 2026-08-27

- Repairer: Codex
- Scope: R1-F1 only. Production code, migrations, fixtures, and unrelated tests
  were unchanged.
- Repaired state: `internal/usage/usage_test.go` extends
  `TestUsageParserVersionBackfillsWorkSignalExtractionWithoutSensitiveContent`
  with the two omitted output classes.

### R1-F1 — CLOSED

The existing five sentinels now run through one assertion set covering every
surface the finding names:

- scan progress diagnostics serialized from `scanProgressRecorder`;
- emitted standard-log lines captured around the scan;
- `Summary.Warnings` serialized as the warning-string surface;
- a deterministic stale-inventory failure on the same sentinel-bearing source,
  whose returned error text is asserted;
- the already-covered database/source cache, scan result, and `Page`/`Detail`
  JSON surfaces.

The error case changes only the captured inventory identity before
`ScanInventory`, forcing `validateSnapshot` to return `errUsageSourceChanged`.
It does not invent an error containing the sentinels and it uses no second
fixture. Every output is checked with `strings.Contains` against the full path,
directory fragment, command string, user message, and result body.

### Evidence

```text
scripts/run-go-test.sh -run TestUsageParserVersionBackfillsWorkSignalExtractionWithoutSensitiveContent ./internal/usage
  -> PASS
scripts/run-go-test.sh ./internal/usage/...
  -> PASS
make vet
  -> exit 0
make check-privacy
  -> exit 0
```

- Verification level: focused test-only repair. The previous full-suite, race,
  fixture-producer, and cross-build evidence remains valid because production
  code, fixtures, dependencies, toolchain, and environment did not change.
- Verdict: REOPEN — repair complete, awaiting independent Re-review

## Round 2 — independent re-review — 2026-08-27

- Reviewed state:
  - HEAD `9472b05d8adc2d4f16e4aafbef6625682ebd5f46`, working tree uncommitted
  - `internal/usage/usage_test.go` `f88f9d0923ccf0864551be0598b1dc05f493d6ab` (was `82c9465c`)
  - `internal/usage/usage.go` `d743f85483ca821eef646da8add6827105d43528` (unchanged)
  - `internal/activity/extraction.go` `1e3f49e2400617f01c17126fcc44fcf0ad96c58c` (unchanged)
- Reviewer: claude-code, independent of the repairer
- Method: R1-F1 checked against the repaired test rather than against the repair
  note. The four capture points the repair adds were each traced to their wiring
  — where the sink is installed, when it is installed relative to the scan, and
  whether the value it captures can be non-empty — because a capture that is
  never reached asserts nothing, which is the same defect R1-F1 raised in a
  different form. Diff scope confirmed against the Round 1 blobs. Prior
  full-suite, race, fixture-producer and cross-build evidence reused, since the
  repair is test-only and confined to one package.
- Scope: R1-F1 and regressions caused by its repair. Production code, tests
  other than the repaired one, configuration, and fixtures remained read-only.

### Finding dispositions

#### R1-F1 — CLOSED

The negative test now runs the same five sentinels through seven outputs, adding
the two surfaces the criterion named and the finding required:

| Surface | How it is captured | Non-empty in this test |
| --- | --- | --- |
| Emitted log lines | `log.SetOutput(&emittedLogs)` around the second `Scan`, restored by `defer` | No — see below |
| Warning strings | `Summary.Warnings`, serialized | Structurally reachable |
| Error strings | `scanErr.Error()` from a real `errUsageSourceChanged` | Yes |
| Scan diagnostics | `service.Progress` recorder installed before the scan | Yes |
| Database, source cache, scan result, `Page`/`Detail` JSON | As in Round 1 | Yes |

Three properties make this a real assertion rather than a restatement of the
structural argument it replaces:

- **The sinks are installed, not described.** `log.SetOutput` is process-global
  and wraps the scan, so any `log.*` call added to the scan path in future lands
  in the buffer and fails the test. That is the regression protection R1-F1
  asked for, and it does not depend on there being output today.
- **The error is provoked, not fabricated.** The repair mutates the captured
  inventory identity and re-runs `ScanInventory`, and the test asserts
  `errors.Is(scanErr, errUsageSourceChanged)` before asserting on its text. No
  second fixture, and no error constructed to contain the sentinels.
- **The sentinels remain distinguishable.** `secretDirectory` appears only in
  the sed target path, never in the source path that errors legitimately carry,
  so an assertion on error text can still fail for the right reason.

Recorded so a later round does not mistake it for a gap: **`emittedLogs` is
empty at this state**, because neither `internal/usage` nor `internal/activity`
calls the standard logger on the scan path — re-verified this round. The
assertion is forward-looking by construction. This is the difference from what
Round 1 rejected: a structural argument cannot fail when the structure changes,
and this capture can.

The repair also re-recorded the CEv1 `privacy-boundary` evidence, whose check now
enumerates all seven surfaces and matches the criterion's statement. That was the
second half of R1-F1's bounded remediation.

#### Newly blocking findings

None.

#### Regressions

None. The implementation blobs are byte-identical to Round 1
(`extraction.go` `1e3f49e2`, `usage.go` `d743f854`), so every Round 1 "checked
and found sound" item stands without re-derivation, and the repair added 47 test
lines in one package and changed nothing else.

### Evidence

```text
scripts/run-go-test.sh ./internal/usage/... ./internal/activity/... ./internal/store/...
  -> PASS (log agentdeck-go-test.urD4Mo)
make vet          -> exit 0
check-privacy.sh  -> exit 0
```

Reused from Round 1, valid because production code, fixtures, dependencies and
toolchain are unchanged: `./internal/desktop/... ./cmd/...` PASS
(log agentdeck-go-test.fuFCUU), including
`TestCanonicalFixturesAreReproducibleProducerOutput`.

- Completion gate: VERIFIED — all seven criteria of
  `work-signals:work-signal-extraction` answer `pass` at the repaired state. The
  Round 1 `fail` record stays bound to the superseded state `4f92ca8a` and is
  superseded rather than rewritten.
- Verdict: PASS
