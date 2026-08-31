---
status: active
created: 2026-08-20
updated: 2026-08-31
---

# Work Signals — Tasks

This file is the only status authority for this topic.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| ux/session-work-signals.md | [x] | [x] |
| ux/cli-work-signals.md | [x] | [x] |
| acceptance/work-signal-surface.md | [x] | [ ] |
| tasks.md | [x] | [x] |

**Re-opened 2026-08-27 (second time) by two blockers found entering task 2.**
Both were confirmed against the repository before any document changed, and both
are contradictions this set carried rather than implementation problems.

The first is self-evidencing: Decision 3 ran category precedence on the *presence*
of a fault word and, three paragraphs later, told implementers to resolve
`coding`'s subcategories by earliest match — citing `add error handling` as the
case that motivated it. A subcategory rule cannot rescue a message the category
rule already sent to `debugging`, so task 2's acceptance for that exact message
was unreachable from this document. Decision 3 now runs one message scan that
decides both levels.

The second is a gap rather than a contradiction. Decision 1 carries the turn
boundary across an incremental scan, but nothing carried the message-derived
classification, and `usage_work_signals` had no pending state, no source
ownership, and no reset path. A turn split across two scans would have been
classified from tool shape alone, making `debugging` unreachable at any
boundary. Decision 11 is new and owns this.

All five `Review` cells untick under the set's one-verdict rule. Decision 8's
`usage_work_signals` DDL is left as written and marked superseded, because it is
what the committed schema contains.

Combined Review Round 6 (2026-08-27): **REOPEN** on R6-F1 through R6-F3.
Decision 3's single message scan closes the category contradiction, and Decision
11 recognizes that classifier state must survive incremental scans. The new
ownership rule, pending-row key, and migration acceptance are not yet
implementable without source loss, an unstated provisional index, or a
deterministic canonical-fixture failure. `reviews/documents.md` owns the exact
findings, evidence, and bounded remediation. All five `Review` cells remain
unticked.

Independent Re-review Round 7 (2026-08-27): **REOPEN**. R6-F2 and R6-F3 are
closed. R6-F1 remains open because Decision 11 says the lexicographically smaller
source wins while the delivered event/tool comparison and its regression test
select the larger path. Implementing the repaired text would make signals choose
a different owner from events and tool calls. `reviews/documents.md` owns the
exact disposition and evidence. All five `Review` cells remain unticked.

Independent Re-review Round 8 (2026-08-27): **PASS**. The second R6-F1 repair
now states the delivered sorting-last ownership rule, anchors it in the existing
live-versus-archive example, and requires one cross-table assertion over signals,
events, and tool calls. R6-F1, R6-F2, and R6-F3 are all closed with no new
finding. All five `Review` cells tick together; task 2 is the next implementation
task. `reviews/documents.md` owns the exact disposition and evidence.

**Re-opened 2026-08-27 by a design change, repaired under Round 4's R4-F1,
and PASSED independent Re-review Round 5.**
Decision 8 named the migration `v19`, which `switch-effectiveness-boundary` had
since landed and shipped, so the number was wrong and this task set said to write
it. The first repair replaced one literal with another and left the document
saying both things at once — read the number from the code, and also it is v20
with a fixed `19` → `20` fixture diff. Round 4 caught that. Decision 8 and task 1
now state only the procedure, `next` is defined by what the implementer reads,
and no acceptance clause carries a version literal. Task 1 also carries the
canonical-fixture regeneration that raising the count forces.

All five `Review` cells moved together: the design change and Round 4 `REOPEN`
unticked the set, and Round 5 `PASS` ticked the set. The three unchanged documents
were re-read against the repaired pair rather than re-argued.

## How this document set is reviewed

**The five documents are reviewed together, in one round, against each other.**
This is a deliberate departure from the project's per-document review, made by
the user on 2026-08-20, and it applies to this topic's document phase only.

The reason is the failure it replaces. The discarded first pass reviewed one
document at a time in dependency order. Every time an upstream contract was
repaired, documents downstream of it that had already passed became wrong, their
`Review` cells were untied, and they went back for another round — fourteen
rounds across four documents, six of them repairing damage the process itself had
caused. Reviewing a set whose members constrain each other one member at a time
does not converge; it oscillates.

So: one round, all five documents, one verdict. A finding is raised against
whichever document owns the fact, and every document the finding touches is
repaired in the same round before the round closes. The round's record is
[`reviews/documents.md`](reviews/) — one record for the set, not one per file.

The verdict is a single `PASS` or `REOPEN` for the set. A `PASS` ticks all five
`Review` cells at once. A `REOPEN` ticks none of them, however few documents the
findings touched, because a set that disagrees with itself is not partially
correct.

Whether this becomes the project's general review process is decided after this
topic, not by this topic. Here it is a temporary measure with a named reason.

### What counts as a finding

Rounds 1 and 2 produced 39 findings, and a third of them were of one shape: a
cited line number had moved, or a paragraph describing an earlier draft had gone
stale. None of those would have changed what an implementer builds. They cost a
review round each and they are self-inflicted, because the documents invited them.

Two rules remove the whole class:

- **No line numbers in these documents.** Cite a file, a type, a function, or a
  section. A line number is correct for about a day and then generates a finding
  that teaches nothing.
- **No archaeology.** These documents state the contract as it is. "An earlier
  draft said X", "this was corrected on date Y", and counts of what a previous
  pass measured belong in the review record, which exists for exactly that. A
  document that narrates its own history gives every later edit a second place to
  go stale.

And the bar for raising one:

> A finding must name something that would make two competent implementers build
> different things, or make a built thing wrong.

Anything else — a clearer sentence, a better word, a stale aside — is fixed in
passing by whoever notices, with no finding, no disposition row, and no round.
Only P1 blocks a `PASS`. A P2 is repaired but does not hold the set; a P3 is not
raised at all.

This is not a lower standard. Rounds 1 and 2 each found real defects — a wire
shape that could not answer the panel's filters, a Codex tool vocabulary that was
invented, an extraction path that would have produced empty metrics on the
client this user runs most. Those are what review is for, and none of them would
have been found faster by also counting string-catalog keys.

## Current state, 2026-08-20

The document set is reviewed and **PASSED** as of 2026-08-20; all five `Review`
cells are ticked. It took three rounds, recorded in
[`reviews/documents.md`](reviews/documents.md). Three P2s are carried into
implementation rather than held for another round, and they are listed in that
record's Round 3 closing note. The set was rewritten from scratch on 2026-08-20
after the user discarded the first pass; the superseded set and its fourteen
review rounds are in
[`docs/archive/work-signals-superseded-2026-08-20/`](../../archive/work-signals-superseded-2026-08-20/).

Implementation starts at task 1.

The prototype work in task 0 is **done and verified in a browser**, ahead of the
document review, because the documents derive from the prototype and cannot be
reviewed against a specimen that does not yet show what they describe.

## Task breakdown

### 0. `work-signal-prototype` — done

- Moved the prototype from `docs/topics/desktop-app/ux/prototype/interactive-v7/`
  to [`/prototype/`](../../../prototype/) at the repository root. A prototype is a
  specimen of the whole product, not an asset of one topic, and a second copy
  under this topic would have created a second design truth.
- Replaced the pending capture fixture with a captured one carrying four
  categories and eleven subcategories, and **retained** the pending fixture — it
  is what the `unavailable` state renders and what an older snapshot decodes to.
- Activity detail rows now expand to their subcategories; Tooling lost its cost
  column for Decision 4's reason; iteration depth became rework.
- Added the `?surface=cli` page rendering the six CLI output shapes as literal
  character output.
- Verified in a browser at both levels, not merely built: the CLI page, the
  subcategory expansion under `--sub`, and the panel's expanded Activity row.
- `desktop-app`'s documents still cite the old path. They are rewritten against
  the delivered implementation when that topic finishes; a pointer at the old
  location says where the prototype went.

### 1. `work-signal-extraction`

- Contract: [`architecture.md`](architecture.md) Decisions 1, 2, 7, and 8.
- Extend `internal/activity` to segment turns on both clients per Decision 1, and
  to retain `turn_index`, `tool_kind`, `mcp_server`, and one `usage_tool_files`
  row per file per call. Take what Decision 2 permits and drop the rest in the
  same function that read it.
- Implement Decision 2's three Codex extraction paths: `apply_patch` patch
  headers, `exec_command`'s `cmd`, and the `tools.exec_command({cmd: …})`
  literals inside an `exec` JavaScript payload. `apply_patch` is a first-class
  edit signal, not an `other`: it accounts for 3,816 calls across 93 sessions in
  the corpus and the migration backfills them.
- Port CodeBurn's `bash-utils.ts` write/read classification. The unclassifiable
  direction is `bash`, never `edit`.
- Record `turn_index` on `usage_events` during the same scan. This is what makes
  Decision 4 structural rather than a timestamp guess.
- Add the next schema migration — including the `usage_tool_files` table — and
  bump `usage_source_files.parser_version` so indexed sources re-scan and all
  four tables backfill. **Read `CurrentSchemaVersion` and the last entry in
  `internal/store/migrations.go` at implementation time and append the next
  one.** This task names no version number, and neither does its acceptance:
  `next` below means whatever that read returns. Decision 8 states why the
  number lives in the code and not in the design.
- Regenerate the two canonical desktop fixtures through the official producer,
  never by hand. They embed the migration count as a doctor check, so raising it
  fails `TestCanonicalFixturesAreReproducibleProducerOutput` and with it the
  whole Go suite. Acceptance: in each of the two files, the count that was there
  before this task replaced by `next`, and no other difference. A larger diff
  means the producer output moved for some other reason, and that is a finding,
  not something to accept.
- Rewrite the package doc comment and the `Record` comment — the absolute
  no-arguments claim is on `Record`, not on `Detail`. A comment left
  contradicting the code is what produced a wrong premise on 2026-08-18.
- Tests MUST include the negative one, and its scope is wider than the store: the
  five forbidden values must be absent from the database, emitted log lines, error
  and warning strings, the `Page`/`Detail` JSON, and the source-file cache, since
  Decision 2's guarantee is "dropped before the record is constructed". Assert on
  **substrings**, because `maxMetadataLength = 256` truncation would let a path
  fragment pass a whole-string check. This is a completion condition of Decision 2,
  not of this task alone.
- The digest is salted with the machine identity; a test asserts the same path
  yields a different digest under a different synthetic machine identity.
- Verification level: **L3** — migration execution plus a privacy boundary.

### 2. `activity-classification`

- Contract: [`architecture.md`](architecture.md) Decisions 3, 4, 5, 6, and 11.
- Implement the four-category classifier. The message is scanned **once**, per
  Decision 3, and that scan decides `message_class` and `intent_sub` together;
  the category precedence and the `coding` subcategory both read its result.
  Implement the visible fallback. Write `usage_work_signals`.
- Replace `usage_work_signals` with Decision 11's shape — `state`,
  `message_class`, `intent_sub`, `source_path`, and defaulted
  `activity_kind`/`activity_sub`. Task 1 created the table and never wrote to it,
  so this is a replacement, not a data migration. Read the migration number from
  `migrations.go` at implementation time.
- Regenerate the two canonical desktop fixtures through the official producer,
  never by hand — the same step and the same reason as task 1. Raising the
  migration count changes the doctor schema count they embed, and the byte-for-byte
  producer test fails the whole Go suite until they are regenerated. Acceptance:
  in each of the two files, the count that was there before this task replaced by
  the new one, and no other difference.
- Implement the incremental classifier state of Decision 11: a `pending` row is
  written when a turn-opening message is seen, is invisible to every aggregate,
  and becomes `classified` once an assistant call arrives. Recomputation is
  idempotent, and tool shape is read back from `usage_tool_calls` rather than
  carried in memory across scans.
- Implement cost attribution from `turn_index` with the `cost_basis`
  discriminator, including `none`.
- Implement the session-level reduction of Decision 5 and the five workflow
  metrics of Decision 6, including the rework counter with its read-shaped
  command exclusion.
- Tests MUST cover: each category rule matching and being outranked; a Codex-only
  and a Claude-only fixture producing comparable classifications from the same
  work despite different boundary markers, including a Claude turn with no tool
  call classified as `conversation`; `add error handling` classified as
  `coding/feature` rather than `debugging/repair`, **and** `fix the add button`
  classified as `debugging` — one example alone cannot distinguish the
  earliest-match rule from a hardcoded exception for that phrase; and the
  assertion that no rule consults tool failure status, which would bias Codex
  systematically.
- Tests MUST also cover Decision 11's scan boundary: a fixture whose
  turn-opening message lands in one scan and whose assistant and tool calls land
  in the next, asserting that the message-derived classification survives — the
  same shape as task 1's `TestClaudePendingTurnBoundarySurvivesAppendCursor`, but
  asserting the category rather than the turn index. A `debugging` message split
  this way must still classify as `debugging`, since classifying from tool shape
  alone would make that category unreachable across a boundary. Cover the reset
  path too: re-scanning a rewritten source leaves no row from the old content.
- The classification of a turn is a pure function of its stored message
  reduction and its rows in `usage_tool_calls`. A test asserts that running the
  classifier twice over an unchanged source produces byte-identical rows.
- Tests MUST cover Decision 11's pending index on both clients: a Claude pending
  row keyed to the next index and promoted in place when the assistant entry
  arrives; consecutive Claude user messages with no assistant between them
  replacing one another rather than accumulating, with the last one's intent
  carried by the resulting turn; and a Codex pending row keyed to the current
  index, since `turn_context` precedes the message.
- Tests MUST cover duplicate-source ownership in **both** scan orders — live
  then archive, and archive then live — asserting the same winner each time, and
  that removing the losing source leaves the winning row intact.
- The ownership test MUST assert that `usage_work_signals`, `usage_events`, and
  `usage_tool_calls` name **the same** `source_path` for the same conflict, in
  the same scenario `TestUsageToolActivityFollowsDuplicateSourceOwnership`
  already covers. Asserting the signals table alone against a direction written
  in prose is what let a reversed rule pass its first repair; a cross-table
  assertion fails whichever way it is reversed.
- A chat-only `conversation` turn with no `usage_tool_calls` row must survive
  every reset and orphan path; it is not an orphan.
- Depends on task 1.
- Verification level: **L2**.

### 3. `work-signal-cli`

- Contract: [`ux/cli-work-signals.md`](ux/cli-work-signals.md) and Decision 10.
- Add the three sections to `usage stats`'s default output, with no flag,
  inserted after `▦ ACTIVITY BY WEEKDAY / HOUR` and before `COVERAGE`. Inside
  `usage stats` the first section is titled `🧭 WORK KIND`, because
  `▦ ACTIVITY BY WEEKDAY / HOUR` already occupies the name; under
  `usage signals` it is `🧭 ACTIVITY`. `usage stats --interactive` is unchanged
  and carries no signal sections.
- Add `agentdeck usage signals` with `--kind`, `--sub`, and `--activity`, reusing
  the `usage` group's `--period`, `--client`, `--format`, and `--no-color`
  unchanged. There is no `--top`.
- Add the one `SIGNALS` line to `session show --activity`, omitted when the
  session has no signal row.
- Emit `--format json` through the existing usage envelope with the same field
  names and units as the wire projection.
- Cover the three availability conditions the CLI document defines, each exiting
  `0`, and the `—` versus `0` distinction.
- **Depends on task 2, not on task 4.** This surface reads the derivation
  directly. Scheduling it behind the GUI, or building it by reading the Swift
  host's behavior, is the dependency Decision 10 forbids.
- Verification level: **L2**.

### 4. `work-signal-projection`

- Contract: [`architecture.md`](architecture.md) Decision 9.
- Add the three `sessions.work_signals.*` families to `internal/desktop` as
  **keyed item lists** — each family an `items[]` whose entries carry `period` and
  `client`, following `SessionsPeriods`/`SessionsPeriodItem`
  (`internal/desktop/desktop.go`). A single unkeyed object per family
  cannot answer two filter positions, and the panel's filters change both. Decode
  them in `apps/macos/AgentDeckShared/DesktopWire.swift`.
- Extend the shared fixtures under `desktop/fixtures/v1/` from the same canonical
  examples on both sides, and retain a fixture **without** the new families that
  decodes as `available: false`.
- MUST NOT raise `wire_version`.
- Depends on task 2.
- Verification level: **L2**.

### 5. `work-signal-surface`

- Contract: [`ux/session-work-signals.md`](ux/session-work-signals.md).
- Replace the uncaptured cards with the two-level captured rendering: three
  summary cards, three detail views, the expandable Activity rows, the fixed
  orders, and the state table.
- Both languages ship together; add only the strings the `ux` document lists,
  including `sessions.toolKinds.other`.
- The uncaptured rendering is **retained**, not deleted.
- Acceptance on real macOS 26 covers both appearances, both languages, the
  280 pt narrow bound, native expanded-state structure, textual alternatives,
  and the detail-navigation return target. Actual VoiceOver, TCC, and system
  accessibility-setting automation are explicitly not run and not required;
  the non-execution is recorded under `acceptance/`.
- Depends on task 4.
- The uncaptured form this task replaces belongs to `desktop-app` task 3. If it
  is there when this task starts, replace it; if it is not, build it here. Both
  topics are in `v0.5.0` and the piece is expected either way, so this is not
  tracked as a cross-topic dependency.
- Verification level: **L2** plus manual visual and accessibility acceptance.

### 6. `work-signals-contract`

- Reconcile delivered behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`: the schema version task 1 actually landed — read it
  from the delivered migration, do not predict it here — the narrowed privacy
  boundary, the new command and flags, and the additive wire families.
- Depends on tasks 1 through 5.
- Verification level: **L2**.

## Tasks

| Task | Implemented | Reviewed |
| --- | --- | --- |
| 0. `work-signal-prototype` | [x] | [x] |
| 1. `work-signal-extraction` | [x] | [x] |
| 2. `activity-classification` | [x] | [x] |
| 3. `work-signal-cli` | [x] | [x] |
| 4. `work-signal-projection` | [x] | [x] |
| 5. `work-signal-surface` | [x] | [x] |
| 6. `work-signals-contract` | [ ] | [ ] |

Task 1 Development (2026-08-27): schema v20 adds `turn_index` to usage events,
turn/tool-kind/MCP fields to tool calls, `usage_tool_files`, and the reserved
`usage_work_signals` table. Parser version 5 re-reads indexed Codex and Claude
sources; Codex extracts `apply_patch.input`, JSON-string `exec_command.arguments`,
and `exec.input` JavaScript `tools.exec_command({cmd: ...})` literals. Raw paths,
directories, commands, user messages, and results are reduced before `Record`
construction to a machine-salted digest, a capped base name, and write direction.

Task 2 Development (2026-08-27): schema v21 replaces the reserved signal table
with Decision 11's pending/classified and source-ownership shape and stores only
the bounded read-shaped-shell reduction needed by Decision 6. Parser version 6
re-reads version 5 sources so existing installations backfill classifications.
The deterministic four-category/eleven-subcategory classifier, structural turn
cost attribution, session reduction, and five workflow metrics are implemented;
cross-scan state, both-client comparability, cross-table duplicate ownership in
both scan orders, losing-source removal, reset/replay, cost bases, tie-breaks,
and unavailable metric states have regression coverage. Targeted activity,
usage, and store tests plus the full Go suite pass; Review remains pending.

Task 2 Review Round 1 (2026-08-27): **REOPEN** on R1-F1. A focused incremental
reproducer shows that a command-derived `testing` or `maintenance` hint is folded
only into signals created during the same scan and is not reconstructed from
persisted tool rows. A split user/assistant turn therefore falls back to
`coding/feature`; `reviews/activity-classification.md` owns the exact finding,
evidence, and bounded repair. Both task cells remain unticked until Repair and
independent Re-review close the finding.

Task 2 independent Re-review Round 2 (2026-08-27): **PASS**. R1-F1 is closed:
schema v21 persists only the bounded `testing`/`chore`/empty command hint,
`turnShape` reconstructs both command-shaped subcategories from stored tool
rows, and direct split-scan regressions cover `coding/testing` and
`coding/maintenance`. The focused repair checks and full L2 Go suite pass; all
ten CEv1 criteria are VERIFIED for the synchronized candidate.
Claude pending user boundaries survive append cursors through the existing source
state. The canonical producer changed only the complete and empty-client schema
counts from 19 to 20. L3 verification passed: the full Go suite, race on the
three affected packages, full vet, darwin arm64/amd64 builds, topic/document
hygiene, and the cross-surface privacy regression. Independent Review is next.

Task 3 Development (2026-08-29): `usage signals` now reads the persisted
derivation directly and reuses the usage family's period, client, format, and
no-color semantics. Its repeatable `--kind`, `--sub`, and `--activity` flags
select bounded families, expand subcategories, and filter every family to the
same turns; filtered shares renormalize while cost and event counts remain
absolute, and the command has no `--top`. Non-interactive `usage stats` inserts
`WORK KIND`, `WORKFLOW`, and `TOOLING` after its weekday/hour heatmap and before
`COVERAGE`; the interactive viewer is unchanged. `session show --activity`
adds the one `SIGNALS` line when a classified session exists and omits its
category when cost has no basis. Text preserves measured zero versus em-dash
unavailability, JSON uses the existing envelope and wire-family field names,
and only bounded base names reach the surface. The canonical GUI JSON contract
was regenerated through `TestIsolatedEndToEndFlow`. Focused command, renderer,
filtering, availability, privacy, and session regressions pass, as do the full
affected packages and repository-wide L2 Go suite. Independent Review is next.

Task 3 Review Round 1 (2026-08-29): **REOPEN** on R1-F1. Every rendering rule the
CLI document states was verified against real output rather than source: section
order inside `usage stats`, the four categories in Decision 3's order, `--sub`
indentation, `--activity` renormalizing shares while leaving cost and events
absolute, the three availability forms each exiting `0`, the `—` versus `0`
distinction, and the `SIGNALS` line's omission rules. The JSON field names match
Decision 9 field by field. What does not hold is the acceptance criterion binding
them: `gui-json-contract.json` registers `usage.signals` with a null
`success_schema` because the end-to-end walk never invokes it, and renaming two
`json:` tags left the entire repository suite passing. The criterion is asserted
by inspection where the project already ships a checker that could falsify it —
the same class of finding as Task 1's R1-F1. `reviews/work-signal-cli.md` owns
the finding and its bounded remediation. Both task cells remain unticked until
Repair and independent Re-review close it.

Task 3 Repair Round 1 (2026-08-29): **R1-F1 closed, awaiting independent
Re-review.** The phase7 E2E walk now invokes a populated `usage.signals` payload
and its producer-generated GUI JSON contract records every nested Activity,
Workflow, and Tooling field name. The exact two-tag mutation from Review now
fails the checker; restoring those tags returns the production source to its
pre-diagnostic SHA-256. The focused contract tests and repository-wide L2 Go
suite pass. Production behavior is unchanged; the review record owns the full
disposition and falsifier evidence.

Task 3 independent Re-review Round 2 (2026-08-29): **PASS**. R1-F1 is closed, and
the closure was re-derived rather than accepted: Round 1's exact two-tag rename
was re-applied and the checker now fails while naming the drifted fields, then the
source was restored to its pre-diagnostic SHA-256. The regenerated fixture pins
every Decision 9 field name, including `top_mcp_server` and `top_mcp_calls`, which
are reachable only because the walk's log gained an MCP call. The repair is
contained — the fixture diff is additive and touches only the `usage.signals`
entry, and every production file's diff statistic is identical to Round 1 — so
Round 1's verification of the other ten criteria is reused unchanged. No finding
was carried forward, regressed, or newly raised. Both task cells tick; task 4
`work-signal-projection` is the next implementation task.

Task 4 Development (2026-08-29): `sessions.work_signals` now carries additive
Activity, Workflow, and Tooling families as producer-bounded keyed item lists.
The Go producer reuses the CLI's `usage.Service.Signals` derivation over the
same fixed `today` / `7d` / `30d` × `all` / `codex` / `claude` order used by
session periods, enforces four Activity kinds and at most five non-empty Tooling
rows, and keeps family availability separate from empty positions. The Swift
wire decoder consumes every Decision 9 field and defaults a missing
`work_signals` object to three unavailable families, so the retained legacy
fixture still decodes at `wire_version: 1`. Complete, partial, and empty-client
fixtures were regenerated through the Go producer; the derived GUI command
schema fixture was regenerated through its own phase7 producer. The full Go
suite and `AgentDeckSharedTests` pass (39/39). The aggregate Xcode scheme reaches
and passes the Task 4 tests, then fails one pre-existing App test whose hardcoded
English `wrapper` / `direct` expectation receives the active Chinese
localization; no Task 4 code is on that failure path. Independent Review is
next.

Task 4 Review Round 1 (2026-08-29): **REOPEN** on R1-F1. The keyed family shape,
the producer-enforced bounds, the unchanged `wire_version`, the legacy default,
and the nested types shared with the CLI all hold, checked against the decoded
fixtures rather than argued. What does not hold is the acceptance clause on
identical field names: six workflow metrics and the tooling MCP pair decode into
Swift optionals that no test reads, so a renamed producer tag would blank the
panel's whole Workflow module while every test stays green. Required keys are
protected by the decode itself; these eight are not, and the phase7 GUI contract
captured `items: []`, pinning no item-level name either.
`reviews/work-signal-projection.md` owns the finding, its bounded test-only
remediation, and the two verification limits this round hit — `xcodebuild` is
unavailable on this machine, and the CLT fallback verifier's failure was shown
pre-existing by reverting the fixtures to HEAD. Both task cells remain unticked.

Task 4 Repair Round 1 (2026-08-30): **R1-F1 closed, awaiting independent
Re-review.** `DesktopWireTests` now unwraps the complete fixture's
`today/codex` Workflow item and asserts all six optional metric values, then
unwraps `today/claude` Tooling and asserts `top_mcp_server` and
`top_mcp_calls`. A producer-tag rename now decodes the affected value as `nil`
and fails the exact assertion instead of rendering honest-looking unavailable
data. The focused XCTest passes through the installed full Xcode toolchain and
the repository-wide Go L2 suite passes. Production code, wire schema, canonical
fixtures, and the derived GUI contract are unchanged; the CLT fallback verifier
still reaches the pre-existing refresh/read assertion recorded by Review.

Task 4 Re-review Round 1 (2026-08-30): **PASS.** R1-F1 is closed on evidence
rather than on the repair's account of itself. The eight assertions exist at the
named fixture positions, and renaming `top_file`, `top_mcp_server`, and
`first_edit_seconds` in `snapshot-complete.json` made exactly those assertions
fail with `nil` against the expected value, then the fixture was restored
byte-identical — the acceptance clause on identical field names is now held by a
falsifier instead of by inspection. Round 1's residual uncertainty is closed too:
the installed full Xcode was addressed by a command-local `DEVELOPER_DIR`, so
`AgentDeckSharedTests` ran here for the first time and passed 39/39. The
production side is unchanged since Review Round 1 by diff and by the recomputed
manifest, so that round's verification of the other eight criteria is reused. Two
environment facts are recorded as non-findings: the aggregate scheme's single App
test failure is this machine's Chinese localization meeting a hardcoded English
expectation on a path the additive `DesktopWire.swift` diff never touches, and
`scripts/check-topic-docs.sh` now exits 1 on `schema-version-signal`, an untracked
topic created concurrently and outside this candidate's manifest. Both task cells
tick; task 5 `work-signal-surface` is the next implementation task.

Task 5 Development (2026-08-31): the Sessions panel now reads Task 4's selected
Client × Period work-signal item directly and renders the captured two-level
surface: three summary cards; fixed, singly expandable Activity rows; the
Workflow metric grid and most-touched row; and call-sorted Tooling rows with
`other` last and no cost column. Legacy/unavailable families retain the shipped
Not captured yet cards and gain a detail banner; an empty selected scope omits
the cards. `partial` renders like `turn`, `none` renders unavailable values, and
measured zero remains distinct from `—`. The approved prototype copy is present
in both shipped catalogs, native DisclosureGroup/accessibility focus preserves
parent-child order and the opening-card return target, and 420/280 pt Light/Dark
English/Chinese rendered attachments were generated and visually inspected.
Focused state, navigation, localization and rendering tests, complete App and
Shared XCTest, and the repository-wide Go L2 suite pass. By explicit operator
decision on 2026-08-31, actual VoiceOver speech, TCC, and system accessibility
automation are not run and are not acceptance gates; the record states that
non-execution instead of presenting it as tested. `acceptance/work-signal-surface.md`
owns the evidence and policy disposition. Task 5 Development is complete and
awaits independent Review.

Task 5 Review Round 1 (2026-08-31): **REOPEN** on R1-F1, with R1-F2 also open.
The captured rendering itself holds — fixed orders, the subcategory omission
rule, the three cost bases, the two deliberately withheld strings, both
catalogs, the 256 pt content width, and the navigation/expanded-state model were
each verified against `ux/session-work-signals.md` and the prototype, and the
focused App tests pass. The panel's *state* does not. It requires an item from
all three families for the selected scope and otherwise renders the legacy
`Not captured yet` surface, while the producer projects the three families
independently — activity and workflow from classified turns, tooling from
`total > 0` over tool calls — so a captured scope that simply had no tool call
is reported as a snapshot missing its fields. The sibling CLI names that same
scope `No tool call in the selected scope.`, and no canonical fixture or test
can reach the divergence, which is why nine CEv1 criteria answered `pass` over a
state that does not satisfy `state-and-legacy`. R1-F2 is the unratified change
to `ux/session-work-signals.md`: its acceptance paragraph was rewritten during
development while the set's last PASS still binds the earlier blob, the
Documents matrix still ticks it, and its frontmatter still reads
`updated: 2026-08-20`. `reviews/work-signal-surface.md` owns the exact findings,
evidence, and bounded remediation. Both task cells remain unticked until Repair
and independent Re-review close both findings.

Task 5 Repair Round 1 (2026-08-31): **R1-F1 and R1-F2 closed, awaiting
independent Re-review.** Empty scope now comes from selected session statistics,
while each Work Signal family independently records unavailable versus captured
with no item. A captured scope with no Tooling item keeps Activity and Workflow,
shows `—` for Tooling, and never claims the snapshot lacks fields; a sessionful
scope with no family items shows three captured `—` cards. The focused
asymmetric-family regression and complete App/Shared XCTest pass. The UX
frontmatter and Copy-delta wording are corrected, `reviews/documents.md` Round 9
ratifies the independently reviewed operator decision, and the new Document gate
is VERIFIED. `reviews/work-signal-surface.md` owns the full finding-to-change
mapping. Task 5's Dev cell ticks; Review remains pending.

Task 5 Re-review Round 1 (2026-08-31): **PASS.** R1-F1 is closed on a falsifier
rather than on the repair's account of itself: reinstating the unanimity rule as
a one-line edit made `testCapturedWorkSignalScopeKeepsEachMissingFamilyHonest`
fail at all three of its boundaries, and the file was restored byte-identical by
SHA-256 and `git hash-object`. Emptiness now comes from the selected scope's
session statistics — the same last-event-in-period rule the panel's own empty
note uses — and each family carries its own uncaptured state, so a captured
scope with no tool call keeps Activity and Workflow and renders `—` for Tooling
instead of claiming the snapshot lacks fields. R1-F2 is closed against the
repository rather than the note: `updated: 2026-08-31`, the Copy table restated
as the design delta against the prototype dictionary with all sixteen imported
labels accounted for, `reviews/documents.md` Round 9, and a Document gate whose
digest reproduces from its recorded recipe. Nothing regressed: the full scheme
passes under an explicit English locale (Shared 39, App 59, Widget 22), the
eight narrow renderings were re-executed against the changed views, and no Go
file changed. The single non-English App failure is this machine's Chinese
localization against a hardcoded English expectation, isolated by the English
run. Both task cells tick; the Task gate is VERIFIED. Task 6
`work-signals-contract` is the last remaining implementation task.

Task 1 Review Round 1 (2026-08-27): **REOPEN** on R1-F1. Schema v20, the parser
version bump and backfill, both fixture regenerations, Decision 1's turn
boundaries on both clients, Decision 2's retained set and rewritten comments,
Decision 7's kind table, and all three Codex extraction paths were verified
against the source and hold. The negative test does not: it asserts the five
sentinels against the database, the source cache, the scan result and the `Page`
JSON, while task 1 and Decision 2 also require emitted log lines and error and
warning strings. Those two surfaces are covered by a structural argument instead
— true today, and independently re-verified this round, but it is the assertion
that has to hold tomorrow, and the CEv1 `privacy-boundary` criterion states all
six surfaces while carrying a `pass`. The record under
`reviews/work-signal-extraction.md` owns the finding and its bounded remediation.

Task 1 Re-review Round 2 (2026-08-27): **PASS**. The test-only repair added the
two missing surfaces and two more besides — emitted log lines, `Summary.Warnings`,
scan progress diagnostics, and the text of a provoked `errUsageSourceChanged` —
so the same five sentinels now run through seven outputs. The sinks are installed
rather than described: `log.SetOutput` wraps the scan, and the error is provoked
by mutating the captured inventory identity and asserted with `errors.Is` before
its text is checked. `emittedLogs` is empty at this state, which is recorded in
the review rather than treated as a gap — neither package logs on the scan path,
so the assertion is forward-looking by construction, and unlike the structural
argument it replaces it can fail when the structure changes. Implementation blobs
are byte-identical to Round 1, so every earlier verified item stands. All seven
CEv1 criteria answer `pass`; the gate is VERIFIED.

Task 1 Repair Round 1 (2026-08-27): **R1-F1 closed, awaiting independent
Re-review.** The existing privacy regression now asserts the same five
sentinels against emitted scan diagnostics/log lines, warning strings, and a
deterministic error from the same source, in addition to its existing database,
source-cache, scan-result, and Page/Detail JSON coverage. Production code and
fixtures were unchanged; the focused test, full usage package, vet, and privacy
gate pass. The review record owns the exact disposition and evidence.

Task 0 Review Round 1 (2026-08-27): **REOPEN** on W1-F1. The move, the captured
four-category/eleven-subcategory fixture, the dropped Tooling cost column, rework
replacing iteration depth, the six CLI shapes, the `--sub` expansion and the
panel's two-level Activity expansion were all verified in a browser and hold.
The pending-capture fixture does not: it is retained in `data.js` but no state
can render it, because `unavailable` replaces the whole panel body before the
Sessions panel mounts, so `.pending-flag` never appears on any of the five
states. That fails the reachability half of its acceptance clause, and the
bullet above claiming the fixture "is what the `unavailable` state renders" is
not true of the delivered code. The record under
`reviews/work-signal-prototype.md` owns the finding and evidence. No CEv1
WorkUnit exists for this task yet; it is created and answered at re-review.

Task 0 Re-review Round 2 (2026-08-27): **REOPEN** on W2-F1. W1-F1 is closed —
the Sessions tab now mounts in the `unavailable` state, the board carries
exactly one pending flag, and the Activity detail renders the pending banner
over the retained fixture with no subcategories. But `SessionsPanel` draws its
content from unfiltered fixture data, so that specimen now also lists four
sessions by name, a 59-minute average, three projects and their per-project
rows — real numbers in the state whose Usage tab still says the snapshot could
not be read, and whose own pending banner says the snapshot *was* read and
lacks the new fields. The board states the rule three sections below:
「占位骨架绝不含真实数字」. The conflation Round 1 named was widened rather than
separated.

Task 0 Re-review Round 3 (2026-08-27): **REOPEN** on W3-F1. The Round 2 repair
delivered its stated remedy exactly — the unavailable Sessions panel no longer
prints session counts, per-project rows or recent sessions — but that remedy was
the wrong one, and the review proposed it, so the correction belongs to the
record rather than to the repair. W2-F1's headline stands: the specimen's own
caption says 「没有快照」 and its notice says 「数据读取失败」, and three lines
below them the panel shows `编码 58%`, `首次编辑 2 分钟`, `82 工具调用`. A
specimen that says there is no snapshot cannot display values decoded from one.
The two separations named in Round 1 — a sixth `SURFACE_STATES` entry for the
pending-capture case, or a module-level availability input for the Sessions
panel — remain what closes it.

Task 0 Re-review Round 4 (2026-08-27): **PASS**. The repair took the first
separation: `SURFACE_STATES` gains a sixth `pending` entry, and `signalsFor`,
the pending flag and the detail banner key on it instead of on `unavailable`.
The tab bypass and the Round 2 suppressions are both gone, so `unavailable` is
wholly unavailable again — its Sessions tab carries zero signal cards and the
panel-wide placeholder — while `pending` is its own specimen with its own
caption, a readable hero and session list, and only the work-signal modules
flagged 待采集. The captured path is untouched: `normal` still expands 编码 52%
into its four subcategories. The CEv1 gate for this task was created on this
round and is VERIFIED across all six criteria.

## The dependency graph is a fork

Tasks 1 and 2 are the derivation. Then task 3 (CLI) and tasks 4→5 (GUI) proceed
independently from task 2, and task 6 closes over all of them.

Delivering the CLI first is a valid order and is the one that makes the derived
numbers checkable while the GUI work is still in flight. It is also the order
that would have caught the session-level-category gap in the discarded pass,
where a CLI line consumed a reduction no contract defined.

## Relationship to `desktop-app`

The two topics are siblings in `v0.5.0`.

- Task 5 is the only task touching `desktop-app`'s Swift surface. It owns the
  Sessions-panel work-signal views and nothing else in that target.
- Task 4 is additive on the wire, so a host built before it decodes a newer
  payload as `available: false`.
- Task 0 moved the prototype out of `desktop-app`. That topic's documents are
  rewritten against the delivered implementation when it finishes, which is when
  their prototype links are corrected.

## Starting a task

Read [`requirements.md`](requirements.md), then the decisions of
[`architecture.md`](architecture.md) the task cites, then the prototype for any
surface question. Resolve the Beads task by anchor from live state.
