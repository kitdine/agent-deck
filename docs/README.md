# AgentDeck Documentation

**Status:** active

This directory is the documentation index for `AgentDeck`. Specifications
define approved behavior; plans track implementation work. Repository code,
tests, configuration, and Git history remain the source of truth when they
disagree with a document.

## Active Work

| Feature | Specification | Plan | State |
| --- | --- | --- | --- |
| AgentDeck CLI phase one and release baseline | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [plan](plans/2026-07-13-agentdeck-cli.md) | active; reviewed baselines complete; unified ASCII tables and machine-bound encrypted SQLite credential storage release-verified and independent-review complete 2026-07-22 (L3, `-race`) |
| CLI command manual and usability audit | [manual](cli-manual.md) | [Phase 9 follow-up](plans/2026-07-13-agentdeck-cli.md) | active implemented contract synchronized with the current tables, usage cost coverage, credential storage, automatic price update, and active-log-safe usage rebuild; price, usage rebuild, and output readability reviews all passed 2026-07-22 |
| Usage stats runtime provider dimension | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [follow-up](plans/2026-07-13-agentdeck-cli.md) | implemented and L2 verified 2026-07-21; independent review passed 2026-07-22 |
| GitHub release v0.1.0 and Homebrew tap distribution | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [follow-up](plans/2026-07-13-agentdeck-cli.md) | v0.1.0 published 2026-07-21; future-tag notes hardening, completion-aware formula rendering, isolated brew verification, automated tap PRs, and a v0.1.0 migration dispatch are implemented and release-verified; independent review and external delivery remain |

## Archived Work

Superseded and one-off documents live under `docs/archive/`, not in this
index. Open `docs/archive/README.md` only when you need historical context —
why a decision was made, what a removed feature's contract looked like, or
the background of a superseded plan. Do not start new work from that
directory, and do not re-list its individual files here.

## Document Lifecycle

- `active`: current requirements or unfinished implementation work.
- `reference`: implemented behavior retained as a durable contract.
- `historical`: superseded material kept only for context; archive it (see
  `docs/archive/README.md`) rather than leaving it active in `docs/plans/` or
  `docs/specs/`.

Update the closest active specification and plan when behavior or execution
state changes. Do not create review snapshots when an active document can hold
the result.

## Naming Convention

- `docs/specs/YYYY-MM-DD-<topic>-design.md` — requirements and architecture
  contract for a feature. `YYYY-MM-DD` is the date the contract was first
  approved, not a per-revision timestamp; keep updating the same file as the
  contract evolves.
- `docs/plans/YYYY-MM-DD-<topic>.md` — the execution tracker for that
  feature: phased checklists plus dated `### <Topic> Follow-Up (YYYY-MM-DD)`
  subsections for later remediation/review passes. Add a new dated
  subsection to the existing plan instead of creating a new plan file when
  follow-up work lands on an already-planned feature — this is how
  `docs/plans/2026-07-13-agentdeck-cli.md` has stayed the single execution
  tracker across multiple release and review cycles rather than
  fragmenting into one file per follow-up.
- Untriaged future ideas that do not yet have an approved spec go in that
  same living plan's `## Backlog / Future Feature Ideas` section as
  unchecked items, not in a new file. Promote a backlog item to its own
  `_design.md` / plan entry only once it is actually being scoped.
- `docs/archive/<original-filename-with-plan-or-design-suffix>.md` — where
  superseded or one-off documents move once archived; see
  `docs/archive/README.md` for the criteria and process.
