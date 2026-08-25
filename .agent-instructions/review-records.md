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
- Verdict: PASS | REOPEN
```

A design or contract review records both HEAD and the blob hash, because the
document alone does not identify the state it was judged against. The `Method`
line exists so a later reader can tell a repository-verified finding from a
tool's unverified score; `development-workflow`'s review reference owns the
dimensions for each target class.

A `PASS` round ends with the plan's `Review` cell ticked. A `REOPEN` round names
the unclosed findings and reverts the task to `Dev`; the next pass is `Round 2`
in the same file.
