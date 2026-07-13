# Local Tools Documentation

**Status:** active

This directory is the documentation index for `local-tools`. Specifications
define approved behavior; plans track implementation work. Repository code,
tests, configuration, and Git history remain the source of truth when they
disagree with a document.

## Active Work

| Feature | Specification | Plan | State |
| --- | --- | --- | --- |
| AI provider mode | [spec](specs/2026-07-13-ai-provider-mode-design.md) | [plan](plans/2026-07-13-ai-provider-mode.md) | implementation present in the worktree; verification and delivery remain separate |
| Local AI provider session cost tracking | [spec](specs/2026-07-13-ai-provider-session-cost-design.md) | [plan](plans/2026-07-13-ai-provider-session-cost.md) | implementation in progress; core calculation, local import, attribution, and CLI contracts are covered by synthetic tests |

## Document Lifecycle

- `active`: current requirements or unfinished implementation work.
- `reference`: implemented behavior retained as a durable contract.
- `historical`: superseded material kept only for context.

Update the closest active specification and plan when behavior or execution
state changes. Do not create review snapshots when an active document can hold
the result.
