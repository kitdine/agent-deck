---
status: active
created: 2026-07-14
updated: 2026-09-08
---

# AgentDeck Documentation

This stable landing page routes readers to the current authorities without
copying their changing state. Code, tests, configuration, and Git history remain
the source of truth when they disagree with documentation.

## Start Here

| Need | Authority |
| --- | --- |
| Integrated project and release state | [Project Status](status.md) |
| Unmerged topic progress and cross-worktree handoff | The selected worktree's `docs/topics/<topic>/tasks.md` and [Beads](../.agent-instructions/beads.md) |
| Version roadmap, backlog, and withdrawn candidates | [Roadmap and Backlog](roadmap.md) |
| Documentation naming, topic structure, and lifecycle | [Documentation Workflow](documentation-workflow.md) |
| Product, persistence, security, compatibility, and distribution contract | [CLI Design](specs/cli-design.md) |
| Implemented commands, flags, output, and interaction behavior | [CLI Manual](specs/cli-manual.md) |
| Brand asset provenance, hashes, and unresolved legal boundaries | [Brand Asset Provenance](legal/brand-assets.md) |
| Historical topics, plans, and reviews | [Archive](archive/README.md) |

Topic-internal status belongs only to that topic's `tasks.md` and `reviews/`.
This page changes only when the documentation topology or stable entry contract
changes.

## Authoritative Documents

| Document | Status | Authority |
| --- | --- | --- |
| [Project Status](status.md) | Active | Integrated project/release state and version-assembly projection; unmerged topic progress remains in its worktree's `tasks.md`. |
| [Roadmap and Backlog](roadmap.md) | Active | Later version direction, planning intake, and withdrawn candidates. |
| [Product Prototype](../prototype/README.md) | Active | The design truth for every user-visible surface: menu-bar panel, widgets, settings, and CLI output. A document that disagrees with it is repaired. |
| [CLI Design](specs/cli-design.md) | Active, version 29 | System, persistence, security, compatibility, and distribution contracts. |
| [CLI Manual](specs/cli-manual.md) | Active | Implemented commands, flags, output shapes, and interaction behavior. |
| [Documentation Workflow](documentation-workflow.md) | Active | Documentation naming, topic structure, lifecycle, readiness matrices, size policy, and status vocabulary. |
| [Brand Asset Provenance](legal/brand-assets.md) | Active | Generation history, derivative chain, canonical hashes, and the boundary between recorded provenance and unresolved trademark/legal review. |
| [Archived Documents](archive/README.md) | Active index | Retirement history and pointers to historical topics, plans, and reviews. |

A topic's own worktree documents are authoritative while it executes. Resolve its
`tasks.md` through the workspace binding and Beads; inherited Project Status is
not a live dispatch view. Review-record format lives in
`.agent-instructions/review-records.md`.

User-facing entry points are the [English README](../README.md) and
[Chinese README](../README_zh.md). Repository-specific development and
authorization rules live in [AGENTS.md](../AGENTS.md).

## Documentation Workflow

Naming, topic structure, document lifecycle, readiness matrices, task status
rules, size policy, and status vocabulary live in the
[Documentation Workflow](documentation-workflow.md). This stable index points to
the dynamic authorities above and does not copy their current values.

## Compatibility Anchors

Existing topic documents may still link to the former dynamic sections while
their own historical or in-progress changes remain uncommitted. These stable
anchors preserve navigation without copying dynamic content:

<a id="current-state"></a><a id="release"></a><a id="active-development"></a>
[Integrated project and release state](status.md)

<a id="known-residual-risk"></a>
[Known residual risk](status.md#known-residual-risk)

<a id="roadmap"></a><a id="backlog"></a><a id="withdrawn-candidates"></a>
[Roadmap, backlog, and withdrawn candidates](roadmap.md)
