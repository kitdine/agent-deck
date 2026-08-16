---
status: active
created: 2026-08-13
updated: 2026-08-16
---

# Usage Attribution Precision — Requirements

Target version: `v0.6.0`. This is the first bounded topic promoted out of the
[`v0.6.0` cost-truthfulness scope](../../README.md#roadmap), and the other
`v0.6.0` items depend on it: pricing backfill, `codex-auto-review`
classification, `unpriced` disambiguation, and layered cost presentation all
require knowing which provider and multiplier an event belongs to.

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
| `EXACT_RUNS` reported by `usage diagnose` | **0** |
| Priced / unpriced events | 76,220 / 2,852 |
| Known catalog subtotal / provider subtotal | 7851.823874510 / 7696.001072830 |

`EXACT_RUNS 0` is the decisive observation: `exact` is only reachable through
`usage_runs.exact = 1`, which only `agentdeck run` creates. Usage hooks are
`configured` for both clients, yet cannot produce `exact` by construction.

## Confirmed Decisions

- **`exact` means determinable, not process-bound.** The quality dimension
  describes whether the provider and multiplier for an event are determinable,
  not which code path recorded them. Binding it to `agentdeck run` is why the
  share is 0.16%.
- **Quality becomes three values: `exact`, `estimated`, `unattributed`.**
  `historical` is renamed because it described a fallback, not a property of
  the data.
- **Time semantics differ per client and this is intentional.** Codex loads
  provider configuration at process start, so a Codex event is attributed by
  its session start boundary. Claude applies configuration immediately, so a
  Claude event is attributed by its own event time against the route sequence.
- **Attribution stays derived at read time.** No schema migration and no
  persisted quality column; the resolver already computes it per read.
- **Recomputation is accepted and must be disclosed.** Existing data is
  re-attributed on upgrade, so cost totals change. The release notes must state
  this, following the `v0.3.0` cache-creation precedent.
- **Unattributed cost is never folded into a real-spend total.** It is reported
  as its own bucket so a total can no longer silently include multiplier-`1`
  guesses.

## Non-Goals

- No pricing changes. Cache-write backfill, `codex-auto-review` classification,
  and `unpriced` disambiguation are separate `v0.6.0` items that consume this
  topic's output.
- No change to how routes are captured. Hook lifecycle, `RecordSessionRoute`
  triggering conditions, and `agentdeck run` behavior stay as they are; only
  the interpretation of what was captured changes.
- No persisted attribution column and no schema migration.
- No budget rules, alerting, or menu-bar surface. Those depend on the desktop
  app and are separate `v0.6.0` items.
- No attempt to reconstruct attribution for events predating adoption.

## Surfaces and contracts

This topic adds no interactive surface, so it carries no `ux/` document. It does
change the observable `usage summary` JSON and text output, which is a contract
change specified in [`architecture.md`](architecture.md).

## Acceptance boundary

- A Codex session that spans a provider switch without restarting attributes
  every one of its events to the provider loaded at process start.
- A Claude session whose provider changes mid-session attributes events before
  and after the change to their respective providers.
- No event is priced at multiplier `1` while being counted toward a real-spend
  total.
- `usage summary` distinguishes determinable, inferred, and unattributed
  totals, and the CLI manual documents the new meanings.
- The release notes state that cost totals can change for existing data.
