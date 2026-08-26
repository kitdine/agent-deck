---
status: active
created: 2026-08-13
updated: 2026-08-26
---

# Usage Attribution Precision — Requirements

This is the first bounded topic promoted out of the
[`v0.5.0` cost-truthfulness planning intake](../../roadmap.md#backlog). Version membership
is decided by that version's contract topic and the Roadmap, not here; this
topic carries no version of its own. Independent `v0.5.0` cost-truthfulness
candidates such as pricing correction, `codex-auto-review` classification, and
layered cost presentation consume its output, because they first need to know
which provider and multiplier an event belongs to.

The question this topic answers is not what an event costs. It is **whether the
recorded cost is real**. Today 26.8% of events are priced at multiplier `1`
with `provider = unknown` because attribution fell through to a default, and
another 73% are labelled `estimated` regardless of how strong the evidence
actually was.

## Evidence Baseline

Measured on 2026-08-13 against the real local store with the released `v0.4.1`
binary:

| Signal | Value |
| --- | --- |
| Events / sessions / source files | 79,072 / 270 / 785 |
| `exact` attribution | 128 (0.16%) |
| `estimated` attribution | 57,729 |
| `historical` attribution | 21,215 (26.8%, forced to multiplier `1`) |
| Open `agentdeck run` processes (`usage diagnose` key `exact_runs`) | 0 |
| Priced / unpriced events | 76,220 / 2,852 |
| Known catalog subtotal / provider subtotal | 7851.823874510 / 7696.001072830 |

The decisive observation is the 128-event `exact` count (0.16%): the released
resolver promotes only events bound to `usage_runs.exact = 1`, while Hook-captured
effective routes remain `estimated`. The diagnose key `exact_runs` does **not**
measure that population; its query counts rows whose `ended_at IS NULL`, so the
observed value `0` says only that no `agentdeck run` process was open when the
baseline was taken. It is retained above as idle-state context and is not used as
evidence for attribution quality. Implementation acceptance must separately
report event count, run count, provider-cost amount, and provider-cost share for
each quality bucket so a claim such as `exact cost = 0 / 0%` names its denominator
and can be reproduced.

## Confirmed Decisions

- **`exact` means determinable, not process-bound.** The quality dimension
  describes whether the provider and multiplier for an event are determinable,
  not which code path recorded them. Binding it to `agentdeck run` is why the
  share is 0.16%.
- **Quality becomes three values: `exact`, `estimated`, `unattributed`.**
  `historical` is renamed because it described a fallback, not a property of
  the data.
- **Time semantics use effective session boundaries, not file-write time.**
  Codex loads provider configuration at process start. Claude may adopt exactly
  one live transition: a session that started without an API key can take its
  first key. An already-keyed Claude session does not adopt key rotation or key
  removal until restart. Claude events position against effective session routes
  by event time; when no such route exists, both clients fall back to the
  provider selection at session start.
- **Attribution quality is derived at read time.** The existing
  `usage_session_routes.quality` column remains persisted as legacy capture data
  and every current writer stores `estimated`; the target resolver deliberately
  stops treating that value as its verdict. No schema migration, write-path
  change, or backfill is required.
- **Recomputation is accepted and must be disclosed.** Existing data is
  re-attributed on upgrade, so cost totals change. The release notes must state
  this, following the `v0.3.0` cache-creation precedent.
- **Unattributed cost is never folded into a real-spend total.** It is reported
  as its own bucket so a total can no longer silently include multiplier-`1`
  guesses.

## Non-Goals

- No pricing changes. Cache-write backfill, `codex-auto-review` classification,
  and `unpriced` disambiguation remain separate planning candidates that consume
  this topic's output.
- No independent redesign of route capture. This topic consumes the reviewed
  client-neutral Hook observation and effective-route boundary from
  [`switch-effectiveness-boundary`](../switch-effectiveness-boundary/tasks.md):
  every accepted Codex or Claude delivery is persisted through one shared
  operation, while only `route_effect=advance` or the retained mismatch-unknown
  behavior appends an effective route. Codex remains restart-only; only
  `no key -> first key` may add a live Claude route; matched key rotation and
  removal retain the prior route. This topic reads effective routes, never the
  observation stream, and leaves `agentdeck run` behavior unchanged.
- No new persisted attribution column, no change to the existing route-quality
  writer, and no schema or data migration.
- No budget rules, alerting, or new menu-bar interaction. The existing desktop
  `Presentation` quality payload is nevertheless in scope for semantic alignment
  and canonical-fixture regeneration because it consumes the same resolver.
- No attempt to reconstruct attribution for events predating adoption.

## Surfaces and contracts

This topic adds no interactive surface. It does change the observable
`usage summary` JSON and text output, which is a contract change specified in
[`architecture.md`](architecture.md). The document set itself is declared in
[`tasks.md`](tasks.md)'s Documents matrix, not here.

## Acceptance boundary

- A Codex session that spans a provider switch without restarting attributes
  every one of its events to the provider loaded at process start.
- A Claude session that starts without an API key and adopts its first key
  attributes later events to that custom provider from the effective
  `ConfigChange` boundary.
- A Claude session already using key A remains attributed to A across a file
  change to key B or to no key. Only a restarted session adopts B or the
  subscription selection through a fresh `SessionStart` boundary.
- No event is priced at multiplier `1` while being counted toward a real-spend
  total.
- `usage summary` reports `exact`, `estimated`, and `unattributed` counts plus a
  machine-readable reason split that distinguishes `before_adoption` from
  `coverage_gap`; its text form and the CLI manual document the same meanings.
- The existing desktop `Presentation` contract maps those qualities to
  `determinable`, `inferred`, and `unattributed` respectively, without using
  `provider = unknown` as an alternate quality test.
- The release notes state that cost totals can change for existing data.
