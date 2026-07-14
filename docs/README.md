# AgentDeck Documentation

**Status:** active

This directory is the documentation index for `AgentDeck`. Specifications
define approved behavior; plans track implementation work. Repository code,
tests, configuration, and Git history remain the source of truth when they
disagree with a document.

## Active Work

| Feature | Specification | Plan | State |
| --- | --- | --- | --- |
| AgentDeck CLI phase one | [spec](specs/2026-07-13-agentdeck-cli-design.md) | [plan](plans/2026-07-13-agentdeck-cli.md) | active; Phase 7 re-review passed; legacy entrypoint removal remains pending and unauthorized |

## Reference and Historical Work

| Feature | Specification | Plan | State |
| --- | --- | --- | --- |
| Legacy AI provider mode | [spec](specs/2026-07-13-ai-provider-mode-design.md) | [plan](plans/2026-07-13-ai-provider-mode.md) | reference implementation; superseded by AgentDeck architecture |
| Legacy local session cost tracking | [spec](specs/2026-07-13-ai-provider-session-cost-design.md) | [plan](plans/2026-07-13-ai-provider-session-cost.md) | reference behavior and fixtures; superseded by AgentDeck architecture |

## Document Lifecycle

- `active`: current requirements or unfinished implementation work.
- `reference`: implemented behavior retained as a durable contract.
- `historical`: superseded material kept only for context.

Update the closest active specification and plan when behavior or execution
state changes. Do not create review snapshots when an active document can hold
the result.
