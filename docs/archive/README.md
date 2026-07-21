**Status:** reference

# Archived Documents

Last updated: 2026-07-22

## Why this directory exists

This directory holds documents that are no longer the current entry point for
development, but are not deleted because they still carry useful history:
implementation rationale, superseded designs, or one-off investigation
records.

## Criteria for archiving a document

Move a document here instead of leaving it `active` in `docs/plans/` or
`docs/specs/` once any of the following is true:

- It describes a system, contract, or plan that has been replaced by a newer
  active document or by the current code.
- It is a one-off investigation, incident record, or phased plan whose
  conclusions have already been absorbed into a living document.
- Leaving it in the main `docs/` tree risks being mistaken for the current
  source of truth.

## Rules

- Archiving means **moving**, not deleting. Content is preserved as-is.
- Do not use this directory as a starting point for new work — start from
  `docs/README.md`.
- Read a file here only when you need historical context: why a decision was
  made, what an old contract looked like, or the background of a removed
  feature.
- If an archived document becomes relevant again, copy or rewrite its content
  back into `docs/`; do not treat this directory as a live source of truth.
- `docs/README.md` must not re-list individual archived files — this file is
  the index for archived material so the main doc index doesn't need to grow
  for documents nobody should open by default.

## 2026-07-22 archive batch

**Legacy AI provider mode and session cost tracking** — superseded by
AgentDeck CLI (historical commit `3fcc121` held the removed implementation;
the replacement passed independent review). Both plan/spec pairs were already
marked `historical` in `docs/README.md` but had not actually been moved out
of `docs/plans/` and `docs/specs/`; this batch physically relocates them to
match that status and establishes this archive directory.

- `2026-07-13-ai-provider-mode-plan.md` (was `docs/plans/2026-07-13-ai-provider-mode.md`)
- `2026-07-13-ai-provider-mode-design.md` (was `docs/specs/2026-07-13-ai-provider-mode-design.md`)
- `2026-07-13-ai-provider-session-cost-plan.md` (was `docs/plans/2026-07-13-ai-provider-session-cost.md`)
- `2026-07-13-ai-provider-session-cost-design.md` (was `docs/specs/2026-07-13-ai-provider-session-cost-design.md`)

Current conclusions and requirements for the functionality these described
now live in `docs/plans/2026-07-13-agentdeck-cli.md` and
`docs/specs/2026-07-13-agentdeck-cli-design.md`.
