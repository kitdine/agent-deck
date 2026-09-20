---
status: active
created: 2026-09-19
updated: 2026-09-19
---

# Desktop Refresh — Tasks

This is the topic-local execution and status authority for `desktop-refresh`.
Its origin is the `ad-desktop-refresh` planning carrier and
`ad-bug-widget-refresh-stale`. The delivered `snapshot-performance` topic is a
dependency and retained authority, not a task repeated here.

Workspace: `agent-deck.desktop-refresh`; branch `feature/desktop-refresh`.
Implementation, Git delivery, integration, retirement, and release remain
separate boundaries.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar-refresh.md | [x] | [x] |
| ux/widget-refresh.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [ ] | [ ] |

`requirements.md` passed independent Review Round 1 for its exact document blob,
and its document gate is CEv1 VERIFIED. The menu-bar refresh framework passed
Re-review Round 2 after `MR-R1-F1` closed, and its exact framework/specimen gate
is CEv1 VERIFIED. The Widget refresh framework and shared-prototype specimens are
approved by Review Round 1, and their exact framework/specimen gate is CEv1
VERIFIED. The architecture passed independent Re-review Round 3 for its exact
document blob and is pending its task delivery checkpoint. Both final-surface
reconciliations and task decomposition remain unfinished. Unchecked Draft cells
intentionally denote unwritten documents, not missing stubs. The existing
Settings layout and periodic-refresh control are preserved, so no separate
settings UX document is applicable.

## Tasks

Implementation tasks are not declared during boundary design. This section will
receive the Tasks matrix only after the requirements, both final surface designs,
and architecture have passed their applicable reviews. No Beads implementation
task may be created from this placeholder.
