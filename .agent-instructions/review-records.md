# Review Records

Read this file only when creating or updating a review record. Every reviewed
document and every reviewed task leaves a traceable record, so a ticked `Review`
cell in a topic's status matrix is always backed by an auditable trail: which
content state was reviewed, by whom, what was found, and the verdict.

## Structure

Records live inside the topic they belong to, named after what they review:

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

Records travel with the topic when it retires, because they live inside it.

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
`REOPEN` does not qualify. The matrix cell is the summary; the record is the audit
trail. A `Verdict: REOPEN` round returns that document or task to work and lists
the findings that must close before the next pass.

Documents and tasks use the same records directory and the same rounds, but their
matrices differ: a document has `Draft` and `Review`, a task has `Dev` and
`Review`. See the Status section of `docs/documentation-workflow.md`.

## Findings must reach a carrier before PASS

A `PASS` does not require zero findings. It requires zero **ownerless** findings.

Give every finding an ID — `<round-prefix><N>-F<n>`, the shape already in use
(`A6-F1`, `DW-R11-F2`, `D1-F1`) — and before a round may end in `PASS`, every
finding in that record must be in one of these states:

| State | How it is written |
| --- | --- |
| Closed in a later round | that round names the ID: `A1-F1 closed:`, `DW-R11-F2 -> repaired in candidate.` |
| Carried elsewhere | `-> open`, **followed on the same finding by a carrier**: a Beads issue ID, or `roadmap.md Backlog: <item>` |

A bare `-> open`, `follow-up`, `后续处理`, or `待定` is **not** a destination.
A review record retires with its topic; once it is under `docs/archive/`, nobody
opens it looking for outstanding work. The record is where a finding is stated,
not where it is remembered.

**Why the rule is structural, and why its first example was wrong.** The
argument above stands on its own: a record retires with its topic, so a bare
`-> open` stops existing the moment the directory moves. It needs no incident
to justify it, which matters, because the incident this section originally
cited did not happen.

A 2026-09-02 audit across every review record reported 103 finding IDs with one
apparent exception — `A6-F1`, raised as a blocking `[P1]` in Round 6 of
`docs/archive/topics/switch-effectiveness-boundary/reviews/architecture.md` —
and this rule was written around it as a live `[P1]` nobody had accounted for.
Round 8 of that same record disposes of it: `A6-F1 — SUPERSEDED`, on the user's
own runtime and persistence correction, replaced by `A8-F1`, whose three parts
are each marked `CLOSED` in Round 14 before Round 20 passed. The audit missed it
because its closure vocabulary is `repaired|closed|resolved|addressed|fixed`
plus four Chinese equivalents, and `SUPERSEDED` is in none of them. That single
false positive produced a `[P1]` chore chasing a settled question, a reservation
recorded against delivered attribution work, and this paragraph.

So the episode teaches the opposite of what it was written to teach, and the
lesson is worth more than the one it replaced: **the mechanical check is not the
rule.** `beads-consistency.py` decides closure by matching a word list against a
whole line, which both invents ownerless findings whose disposition it does not
recognise and misses real ones whenever a summary line names several IDs beside
a single closure word. Both directions are carried by
`ad-bug-ownerless-findings-same-line`. Until they are fixed — and after — a round
satisfies this rule when the record itself names the disposition or the carrier,
not when the hook prints an empty list.

### What separates REOPEN from PASS-with-a-carrier

The verdict turns on what the finding points at, not on how severe it is:

| The finding points at | Verdict |
| --- | --- |
| A defect **in the change under review** | `REOPEN` — fix it in this round |
| Something **outside that change** — the same class of defect at another entry point, a pre-existing condition, a process gap | `PASS`, with a carrier |

Without this split the rule collapses into "any finding blocks", and that has a
predictable cost: raising a finding then obliges the reviewer to run another
round, so borderline observations stop being written down. A rule that
suppresses findings protects nothing.

## Retirement

Records retire with their topic. One `git mv` of `docs/topics/<topic>/` to
`docs/archive/topics/<topic>/` moves them, then set each record's frontmatter to
`status: historical` and add `retired:` with the retirement date. Reviews are
never archived separately from the topic they belong to, and never deleted.

## Template

```
---
status: active
topic: <topic>
subject: <document path or task anchor>
---

# Review log — <topic> / <subject>

## Round 1 — YYYY-MM-DD
- Reviewed state: <commit SHA or tree hash; for an uncommitted design or contract,
  the HEAD SHA plus each reviewed document's blob hash>
- Reviewer: <agent or person>
- Method: <how the review was conducted, and any tool whose scope did not match
  the target>
- Scope: <files and behavior actually reviewed>
- Findings:
  - [P1] <defect> -> <resolution or follow-up>
  - [nit] <minor> -> <resolution>
- Evidence: <commands run, results>
- Completion gate: VERIFIED | NOT_VERIFIED | FAILED | BLOCKED | NOT_REQUIRED
- Verdict: PASS | REOPEN
```

A design or contract review records both HEAD and the blob hash, because the
document alone does not identify the state it was judged against. The `Method`
line exists so a later reader can tell a repository-verified finding from a
tool's unverified score; `development-workflow`'s review reference owns the
dimensions for each target class.

`Completion gate` records the independently queried evidence boundary for the
same round. It never changes the review verdict: PASS with a non-VERIFIED gate
still checks the topic's Review cell, but the Beads task remains `in_review`
until the gate reaches VERIFIED. `NOT_REQUIRED` is only for a subject whose
project decomposition defines no completion-evidence gate.

A `PASS` round ends with the plan's `Review` cell ticked. A `REOPEN` round names
the unclosed findings and reverts the task to `Dev`; the next pass is `Round 2`
in the same file.
