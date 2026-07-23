---
status: active
created: 2026-07-22
---

# Archived Documents

Last updated: 2026-07-23

## Why this directory exists

This directory holds documents that are no longer the current entry point for
development, but are not deleted because they still carry useful history:
implementation rationale, superseded contracts, or one-off investigation
records.

It mirrors the live structure: retired execution trackers go under
`plans/`, retired contracts under `specs/`, and a retired plan's review records
under `reviews/<plan-topic>/`. Filenames keep the topic only; `status:
historical` and `retired:` in each document's frontmatter carry the rest.

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

## 2026-07-22 retirement: the phase-one CLI plan

`plans/agentdeck-cli.md` (was `docs/plans/2026-07-13-agentdeck-cli.md`)

This was the project's single execution tracker from the initial Go rewrite
through v0.1.0 and ten follow-up rounds. It was retired at roughly 950 lines,
with every task complete and independently reviewed.

It was retired for size, not for being wrong. A tracker that spans phase-one
bootstrapping, credential encryption, price catalogs, release automation, and
unscoped future ideas makes "is this still current?" expensive to answer, and
the convention that kept appending follow-up sections to it was the direct
cause. That convention has been changed — see `docs/README.md`.

Its role is now split:

- `docs/README.md` — the documentation index and execution baseline: what is
  active, what is open, what is deferred.
- `docs/plans/usage-scan-performance.md` — the one design that was
  still unimplemented when the plan was retired.
- `docs/plans/test-coverage.md` — test coverage work.

Delivered contracts described by the retired plan remain active in
`docs/specs/cli-design.md`, which was **not** retired: a
specification describes the currently-standing system and stays active as long
as that system stands, whereas a plan tracks finite work and retires when the
work is done. That spec also dropped its date prefix in the same pass — a date
implies a snapshot, but a contract is revised in place, so it now carries a
version and changelog instead.

## 2026-07-22 archive batch

**Legacy AI provider mode and session cost tracking** — superseded by
AgentDeck CLI (historical commit `3fcc121` held the removed implementation;
the replacement passed independent review). Both plan/spec pairs were already
marked `historical` in `docs/README.md` but had not actually been moved out
of `docs/plans/` and `docs/specs/`; this batch physically relocates them to
match that status and establishes this archive directory.

- `plans/ai-provider-mode.md` (was `docs/plans/2026-07-13-ai-provider-mode.md`)
- `specs/ai-provider-mode.md` (was `docs/specs/2026-07-13-ai-provider-mode-design.md`)
- `plans/ai-provider-session-cost.md` (was `docs/plans/2026-07-13-ai-provider-session-cost.md`)
- `specs/ai-provider-session-cost.md` (was `docs/specs/2026-07-13-ai-provider-session-cost-design.md`)

Current conclusions and requirements for the functionality these described
now live in `docs/README.md` and
`docs/specs/cli-design.md`.

## 2026-07-22 retirement: usage scan performance and progress

`plans/usage-scan-performance.md` and
`reviews/usage-scan-performance/` were retired together after all six tasks
passed independent review. The plan delivered the linear line-splitting fix,
the `(client, session_id)` usage-event index, delayed stderr progress,
parser-version reread context, stored-aggregate `--no-scan` reporting for stats
and summary, and a controlled same-fixture A/B remeasurement.

The final paired measurement recorded a 5.40x mean cold-scan improvement on the
same frozen fixture. Current behavior remains authoritative in
`docs/specs/cli-design.md` and `docs/specs/cli-manual.md`; the completed plan and
its review rounds remain here only as implementation and measurement history.

## 2026-07-22 retirement: usage stats display readability

`plans/usage-stats-readability.md` and
`reviews/usage-stats-readability/` were retired together after all five tasks
passed independent review. The plan delivered bounded text rankings, a recent
48-bucket trend window, the shared `--top` override, width-aware one-line
detail compaction, and a controlled same-snapshot A/B remeasurement.

The final paired measurement reduced `usage stats --period all` from 139 to
120 lines and `usage stats --period 30d --group-by hour` from 832 to 142 lines.
Current output contracts remain authoritative in `docs/specs/cli-design.md`
and `docs/specs/cli-manual.md`; the completed plan and review rounds remain
here only as implementation and measurement history.

## 2026-07-23 retirement: price catalog coverage

`plans/price-coverage.md` and `reviews/price-coverage/` were retired together
after all five tasks passed review. The plan delivered the content-derived
bundled `catalog_version` guard, the curated gap-fill as a separate input that
regeneration cannot drop, release-time regeneration from a pinned LiteLLM
commit with a reproducibility check, a no-network cold-start coverage test, and
an explicitly disclosed equivalent-estimate price for `gpt-5.3-codex-spark`.

Cold-start coverage on the same frozen snapshot went from **7.4% to 95.1%** of
tokens fully priced (2 models to 112). The residual is deliberate:
`codex-auto-review` stays unpriced as a probable pseudo-model, and the
`cache_creation_tokens` gap on the dotted Claude spellings is a
token-classification concern, not a catalog one. Both are carried forward in
the Backlog of `docs/README.md`, since archiving this plan would otherwise be
the only place they were written down.

Two things are worth reading here rather than rediscovering:

- **Why Spark is priced by an estimate rather than a vendor rate.** OpenAI
  publishes no rate for it, and every aggregator carrying a figure traces back
  to one unconfirmed row. The plan's `## Price Confidence` section records that
  analysis; the accepted resolution is the `equivalent_estimate` contract, whose
  standing rules now live in `docs/specs/cli-design.md`.
- **Why the bundled catalog's own effective date is a constant.** A curated
  model dated earlier than the catalog dragged the catalog's date back, and
  since same-layer catalogs are ranked by that date, a previously installed
  catalog then outranked the newer one on every shared model. Round-4 of
  `reviews/price-coverage/spark-gapfill.md` records the reproduction and the
  fix.

Review independence is documented unevenly and deliberately: tasks 1 and 3–5
were reviewed by a separate reviewer, while task 2's later rounds (4–6) were
performed in the same session as the implementation at the user's direction.
Each of those rounds states that caveat inline.

Current behavior remains authoritative in `docs/specs/cli-design.md` (version
14); the completed plan and its review rounds remain here only as
implementation, measurement, and decision history.
