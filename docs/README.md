# AgentDeck Documentation

**Status:** active

This directory is the documentation index for `AgentDeck`. Specifications
define approved behavior; plans track implementation work. Repository code,
tests, configuration, and Git history remain the source of truth when they
disagree with a document.

## Active Work

| Feature | Specification | Plan | State |
| --- | --- | --- | --- |
| AgentDeck CLI phase one and release baseline | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [plan](plans/2026-07-13-agentdeck-cli.md) | active; reviewed baselines complete; unified ASCII tables and machine-bound encrypted SQLite credential storage release-verified, awaiting independent review |
| CLI command manual and usability audit | [manual](cli-manual.md) | [Phase 9 follow-up](plans/2026-07-13-agentdeck-cli.md) | active implemented contract synchronized with the current tables, usage cost coverage, credential storage, automatic price update, and active-log-safe usage rebuild; price, usage rebuild, and output readability re-reviews remain pending |
| Usage stats runtime provider dimension | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [follow-up](plans/2026-07-13-agentdeck-cli.md) | implemented and L2 verified 2026-07-21; independent review pending |
| GitHub release v0.1.0 and Homebrew tap distribution | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [follow-up](plans/2026-07-13-agentdeck-cli.md) | repository packaging, workflows, and install docs implemented and release-verified 2026-07-21; push/tag/release/tap actions pending explicit authorization |

## Reference and Historical Work

| Feature | Specification | Plan | State |
| --- | --- | --- | --- |
| Legacy AI provider mode | [spec](specs/2026-07-13-ai-provider-mode-design.md) | [plan](plans/2026-07-13-ai-provider-mode.md) | historical contract; repository implementation removed after AgentDeck review |
| Legacy local session cost tracking | [spec](specs/2026-07-13-ai-provider-session-cost-design.md) | [plan](plans/2026-07-13-ai-provider-session-cost.md) | historical contract; repository implementation and fixtures removed after AgentDeck review |

## Document Lifecycle

- `active`: current requirements or unfinished implementation work.
- `reference`: implemented behavior retained as a durable contract.
- `historical`: superseded material kept only for context.

Update the closest active specification and plan when behavior or execution
state changes. Do not create review snapshots when an active document can hold
the result.
