---
status: active
created: 2026-07-22
---

# Review Records

Every review of a plan task leaves a traceable record here, so a ticked
`Review` cell in a plan's status matrix is always backed by an auditable trail:
which content state was reviewed, by whom, what was found, and the verdict.

## Structure

This directory mirrors `docs/plans/` by plan topic, one file per task:

```
docs/reviews/<plan-topic>/<task-anchor>.md
```

- `<plan-topic>` matches the plan filename: `docs/plans/test-coverage.md` →
  `docs/reviews/test-coverage/`.
- `<task-anchor>` matches the task's anchor name in that plan's `## Status`
  matrix (`store-boundaries`, `line-slice`, …).
- Each review pass appends a `## Round N` section to that one file, so a task's
  whole review history — first pass, reopen, re-review — stays in one place and
  in round order.

Create a task's file lazily, when its first review actually happens. Do not
pre-create empty files or fabricate rounds for unreviewed tasks.

## Link to the plan matrix

A plan's `Review` cell may be ticked `✅` only when this directory holds a
`Verdict: PASS` round for that task. The matrix cell is the summary; the review
file is the audit trail. A `Verdict: REOPEN` round sends the task back to `Dev`
in the matrix and lists the findings that must close before the next pass.

## Retirement

When a plan retires to `docs/archive/plans/`, move its review directory the same
way: `git mv docs/reviews/<plan-topic> docs/archive/reviews/<plan-topic>` and
set each file's frontmatter to `status: historical`. Reviews are archived with
the plan they belong to, never deleted.

## Template

```
---
status: active
plan: <plan-topic>
task: <task-anchor>
---

# Review log — <plan-topic> / <task-anchor>

## Round 1 — YYYY-MM-DD
- Reviewed state: <commit SHA or tree hash>
- Reviewer: <agent or person>
- Scope: <files and behavior actually reviewed>
- Findings:
  - [P1] <defect> -> <resolution or follow-up>
  - [nit] <minor> -> <resolution>
- Evidence: <commands run, results>
- Verdict: PASS | REOPEN
```

A `PASS` round ends with the plan's `Review` cell ticked. A `REOPEN` round names
the unclosed findings and reverts the task to `Dev`; the next pass is `Round 2`
in the same file.
