---
status: active
created: 2026-08-20
updated: 2026-08-20
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
| tasks.md | [x] | [x] |

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
- Add schema **v19** — including the `usage_tool_files` table — and bump
  `usage_source_files.parser_version` so indexed sources re-scan and all four
  tables backfill.
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

- Contract: [`architecture.md`](architecture.md) Decisions 3, 4, 5, and 6.
- Implement the four-category classifier with its fixed precedence, the eleven
  subcategories, the earliest-match rule inside `coding`, and the visible
  fallback. Write `usage_work_signals`.
- Implement cost attribution from `turn_index` with the `cost_basis`
  discriminator, including `none`.
- Implement the session-level reduction of Decision 5 and the five workflow
  metrics of Decision 6, including the rework counter with its read-shaped
  command exclusion.
- Tests MUST cover: each category rule matching and being outranked; a Codex-only
  and a Claude-only fixture producing comparable classifications from the same
  work despite different boundary markers, including a Claude turn with no tool
  call classified as `conversation`; `add error handling` classified as `feature`
  rather than `debugging`; and the assertion that no rule consults tool failure
  status, which would bias Codex systematically.
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
- Manual acceptance on real macOS 26 covers both appearances, both languages, the
  280 pt narrow bound, VoiceOver order through both levels including an expanded
  subcategory block, and focus return from a detail view to its card. Recorded
  under `acceptance/`.
- Depends on task 4.
- The uncaptured form this task replaces belongs to `desktop-app` task 3. If it
  is there when this task starts, replace it; if it is not, build it here. Both
  topics are in `v0.5.0` and the piece is expected either way, so this is not
  tracked as a cross-topic dependency.
- Verification level: **L2** plus manual visual and accessibility acceptance.

### 6. `work-signals-contract`

- Reconcile delivered behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`: schema v19, the narrowed privacy boundary, the new
  command and flags, and the additive wire families.
- Depends on tasks 1 through 5.
- Verification level: **L2**.

## Tasks

| Task | Implemented | Reviewed |
| --- | --- | --- |
| 0. `work-signal-prototype` | [x] | [x] |
| 1. `work-signal-extraction` | [ ] | [ ] |
| 2. `activity-classification` | [ ] | [ ] |
| 3. `work-signal-cli` | [ ] | [ ] |
| 4. `work-signal-projection` | [ ] | [ ] |
| 5. `work-signal-surface` | [ ] | [ ] |
| 6. `work-signals-contract` | [ ] | [ ] |

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
