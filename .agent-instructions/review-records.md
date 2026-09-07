# Review Records

Read this file when creating or updating a review record. The
`development-workflow` Skill owns Review and Re-review execution, report
structure, scoring, section order, language, finding policy, next instructions,
checkpoints, and completion receipts. This file adds project record locations,
metadata, and lifecycle; it does not define another report format.

## Structure

Topic records live under the topic's `reviews/` directory, named after the
reviewed document or task:

```
docs/topics/<topic>/reviews/requirements.md
docs/topics/<topic>/reviews/ux-<surface>.md
docs/topics/<topic>/reviews/architecture.md
docs/topics/<topic>/reviews/tasks.md
docs/topics/<topic>/reviews/<task-anchor>.md
```

- A document's record is named after that document, with `ux/<surface>.md`
  flattened to `ux-<surface>.md`.
- A task's record is named after its anchor in `tasks.md`'s Tasks matrix
  (`store-boundaries`, `menubar-experience`, …).
- Each review pass appends a `## Round N` section to that one file, so the whole
  history — first pass, reopen, re-review — stays in one place and in round order.

Lane A reviews append rounds to the existing fix record. Follow
[Fix records](../docs/documentation-workflow.md#fix-records) for its location
and lifecycle; do not create a separate topic or review file.

Topic records travel with the topic when it retires.

Create a record lazily, when its first review actually happens. Do not pre-create
empty files or fabricate rounds for unreviewed work.

A `vX-Y-Z-contract` topic's reviews directory also carries that version's
integration reviews under its `assemble` task anchor, because merging the
version's topic branches is a task of the contract topic rather than a topic of
its own. See `.agent-instructions/branching.md` for the merge classes and what
each requires.

## Link to the status matrices

A `Review` cell in `tasks.md` may be ticked `[x]` only when the matching record's
latest applicable round is `Verdict: PASS`. An earlier `PASS` followed by a later
`FAIL` (historically `REOPEN`) does not qualify. The matrix cell is the summary; the record is the audit
trail. A `Verdict: FAIL` round returns that document or task to work and lists
the findings that must close before the next pass.

Documents and tasks use the same records directory and the same rounds, but their
matrices differ: a document has `Draft` and `Review`, a task has `Dev` and
`Review`. See the Status section of `docs/documentation-workflow.md`.

## Findings must reach a carrier before PASS

Apply the Skill's finding policy: every finding against the target must be
closed before PASS, including low-severity findings. A user's explicit decision
may close a finding; silence or an agent's preference may not. An out-of-scope
finding needs a concrete carrier and does not block this target by itself.

Give every finding an ID — `<round-prefix><N>-F<n>`, the shape already in use
(`A6-F1`, `DW-R11-F2`, `D1-F1`) — and before a round may end in `PASS`, every
finding in that record must be in one of these states:

| State | How it is written |
| --- | --- |
| Closed in the record | The disposition names the finding ID and its resolution, such as `A1-F1 closed:` or `DW-R11-F2 -> repaired in candidate.` |
| Superseded in the record | The disposition names the original finding ID, why it was superseded, and the replacement finding ID. Follow the replacement to its closure or valid carrier before PASS. |
| Closed by explicit user decision | Name the finding ID, mark it `CLOSED`, and record the user's decision and its source. Preserve the finding history; do not infer approval or re-raise a declined finding. |
| Carried elsewhere | `-> open`, **followed on the same finding by a carrier**: a Beads issue ID, or `roadmap.md Backlog: <item>` |

A bare `-> open`, `follow-up`, `后续处理`, or `待定` is **not** a destination.
A review record retires with its topic; once it is under `docs/archive/`, nobody
opens it looking for outstanding work. The record is where a finding is stated,
not where it is remembered.

**Automated checks support review; they do not define finding disposition.**
Before PASS, verify each finding's disposition or carrier in the review
record, including any replacement chain. An empty Hook report alone does
not establish that all findings are accounted for.

Historical rationale: commit `dd09acc96ad12b94e67a32bea30ba2007cfdd769`
records a corrected audit error and the Hook limitations observed at that
time. Consult it for provenance, not as a description of current Hook behavior.

### What separates REOPEN from PASS-with-a-carrier

New records use the Skill's `PASS`/`FAIL` vocabulary. Historical `REOPEN`
records remain unchanged and are read as failed review outcomes. Severity
prioritizes repair; it does not permit an open in-scope finding to pass:

| The finding points at | Verdict |
| --- | --- |
| A defect **in the change under review** | `FAIL` — repair under the next authorized repair stage |
| Something **outside that change** — the same class of defect at another entry point, a pre-existing condition, a process gap | `PASS`, with a carrier |

## Retirement

Topic review records retire with their topic. Move the topic directory to
`docs/archive/topics/<topic>/`, set `status: historical`, and add `retired:`
with the retirement date. Never archive topic reviews separately or delete
review history. Lane A records follow the linked Fix records lifecycle.

## Template

Use the current Review or Re-review report structure from the
`development-workflow` Skill. Do not copy that template into this authority.
Preserve its score, verdict, icons, sections, and presentation language.
Status-only updates follow the Skill's status-summary rule and the project's
status-document responsibilities; they do not duplicate full reports.

A topic record retains its `status`, `topic`, and `subject` frontmatter. Append
one `## Round N — YYYY-MM-DD` per review; a Lane A fix uses its existing
`## Review — Round N` convention. Nested Skill report headings are part of that
round, not separate rounds. Preserve earlier content and increment N for each
new round, rather than restarting at Round 2.

Include these project fields within the Skill report, preserving existing
metadata rather than replacing it to normalize formatting:

- Reviewed state: the content identity required by [Evidence](evidence.md).
  An uncommitted document records HEAD and its scoped blob hash; other dirty
  candidates use HEAD plus the scoped content fingerprint.
- Reviewer, Method, and Scope: who reviewed which files/behavior, using which
  method, with any material limitations.
- Findings: complete stable IDs such as `DW-R11-F2`, severity, location, behavior
  risk, evidence, bounded remediation, and explicit disposition or carrier.
- Evidence: commands, observed results, and the exact content state they cover.
- Completion gate: `VERIFIED`, `NOT_VERIFIED`, `FAILED`, `BLOCKED`, or
  `NOT_REQUIRED`, obtained from the applicable evidence boundary.

Place evidence metadata in the report's summary when suitable. Keep machine
values, finding IDs, paths, and quoted evidence unchanged when localizing prose.
The reader accepts `Verdict` or `结论`/`裁决`/`评审结论`/`复评结论`, and
`Completion gate` or `完成门禁`/`证据门禁`/`验收门禁`, with `:` or `：`.
Do not put live verdict/gate declarations in fenced examples, omit required
metadata, or emit contradictory declarations within one round.

For Re-review, account for every previous finding as closed, still open,
regressed, superseded, or closed by explicit user decision; also record new
findings. Recheck current content before carrying any finding forward.

A review verdict and a completion gate are independent. PASS checks the matching
Review cell, but a non-VERIFIED required gate leaves the evidence boundary open.
Use NOT_REQUIRED only when the subject has no project-defined evidence gate.
Resolve dispatch transitions through [Beads coordination](beads.md), and retain
the Skill's checkpoint and post-phase-blocker behavior.

For a failed review, leave Review unchecked and name the unresolved findings.
Distinguish document Draft/Review from task Dev/Review; change readiness fields
only when the subject's lifecycle requires it. Lane A fixes have no topic matrix.
Token-bound completion receipts belong to the active dialogue, not persistent
historical records; a stored receipt cannot authorize or complete a later turn.
