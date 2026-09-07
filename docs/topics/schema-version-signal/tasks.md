---
status: active
created: 2026-08-30
updated: 2026-09-06
---

# Schema Version Signal — Tasks

This file is the only status authority for this topic.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar-schema-signal.md | [x] | [x] |
| architecture.md | [ ] | [ ] |
| tasks.md | [ ] | [ ] |

The set is a claim, and this matrix is the only place it lives. Two rows are
deliberately unticked rather than absent: the documents are required and not yet
written, and an empty row is what makes that visible. `check-topic-docs.sh`
reports them as gaps until they are written, which is the intended state for a
topic at its boundary stage; the audit is ratified when this file reaches
review, not before.

Why each row exists, against the review question that justifies it:

- `requirements.md` — one boundary question for one coherent behavior change.
- `ux/menubar-schema-signal.md` — the menu-bar Health panel, the four tab
  badges, and the footer are user-visible states with no presentation rule for
  this condition today. `requirements.md` names this surface by path, so the
  audit requires the row.
- `architecture.md` — the error carrier, the stable code, the two version
  fields, and the precedence over `state_busy` are contracts, and "are the
  contracts specified" is one question.
- `tasks.md` — one decomposition.

No second `ux/` row is declared. The CLI renders this condition through the
error-code contract and `docs/specs/cli-design.md`, not through a surface
document, matching how `cli-error-classification` handled the same question. The
widget extension reads the same snapshot as the menu bar and its presentation is
decided inside `ux/menubar-schema-signal.md`; if that document concludes the
widget needs its own state set, this matrix gains a row and returns here for
re-ratification. It concluded on 2026-09-06 that it does not: decision D8 keeps
the widget's existing `Data unavailable` state, on the ground that
`WidgetDesktopSnapshotV1` decodes no `health` at all, so presenting the cause
would mean extending the widget projection. No row is added.

## Task breakdown

Not yet decomposed. Anchors are defined at stage 8
(`设计：schema-version-signal / tasks.md`) and are developable only after this
file passes review — before that the matrix is a draft, and tasks created from a
draft assert work that may never exist.
