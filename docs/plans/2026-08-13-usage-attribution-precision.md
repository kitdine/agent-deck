---
status: active
created: 2026-08-13
updated: 2026-08-13
---

# Usage Attribution Precision

Target release: `v0.6.0`. This is the first bounded plan promoted out of the
[`v0.6.0` cost-truthfulness scope](../README.md#roadmap), and the other
`v0.6.0` items depend on it: pricing backfill, `codex-auto-review`
classification, `unpriced` disambiguation, and layered cost presentation all
require knowing which provider and multiplier an event belongs to.

The question this plan answers is not what an event costs. It is **whether the
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

## Architecture

### Current resolution order

`readPriceResolver.priceForEvent` (`internal/usage/usage.go`) resolves in four
steps, and only the first yields `exact`:

```text
1. usage_runs.exact = 1                  -> exact       (agentdeck run only; observed 0)
2. sessionRouteAt(client, sid, eventAt)  -> estimated   (quality hardcoded in recordSessionRoute)
3. timeline.SnapshotAt(client, sessionStartAt(event))
                                         -> estimated   (hardcoded)
4. fallthrough                           -> historical, multiplier "1", provider "unknown"
```

Step 2 already matches by event time. Step 3 applies `sessionStartAt` to both
clients, which is correct for Codex and wrong for Claude.

### Target resolution order

```text
1. usage_runs.exact = 1                          -> exact
2. route sequence for the session, positioned by client semantics
     codex : the SessionStart route in effect at session start
     claude: the latest route at or before the event time
                                                 -> exact when the positioned
                                                    route is unambiguous
                                                 -> estimated when ambiguous
3. timeline.SnapshotAt positioned by client semantics
     codex : session start time
     claude: event time
                                                 -> estimated
4. no timeline coverage at the positioned time   -> unattributed
```

Step 2 becomes `exact` because a route is an observation of the provider that
was actually in effect, not a guess:

- **Codex** activates configuration only on restart, and `RecordSessionRoute`
  writes on `SessionStart`. The recorded snapshot is therefore the exact
  configuration the process loaded and will keep for its lifetime.
- **Claude** activates immediately, and mid-session changes are separately
  recorded as `ConfigChange` routes. Positioning by event time inside that
  route sequence yields the provider actually in effect.

A route is **ambiguous**, and stays `estimated`, when the positioned route
records `provider = unknown` — which `RecordClaudeConfigChange` writes
deliberately when the managed settings file did not match a completed
selection.

### Unattributed boundary

Step 4 currently conflates two different states. They must be separated,
because only the first is genuinely unknowable:

| State | Meaning | Reportable |
| --- | --- | --- |
| Before adoption | Event predates any provider selection AgentDeck recorded | Yes, as an explicitly bounded bucket |
| Coverage gap | Timeline exists but has no entry at the positioned time | Yes, and it is a defect signal worth surfacing |

Neither may contribute to a real-spend total, and neither may silently use
multiplier `1` as if it were a known rate.

### Contract impact

`usage summary` exposes attribution counts in its JSON `counts` object and
emits `estimated attribution` / `historical attribution` warnings. Renaming
`historical` to `unattributed` and changing what `exact` counts are both
observable contract changes. The CLI manual, the CLI design contract, and the
release notes must be reconciled in the same task that changes the values.

## Non-Goals

- No pricing changes. Cache-write backfill, `codex-auto-review` classification,
  and `unpriced` disambiguation are separate `v0.6.0` items that consume this
  plan's output.
- No change to how routes are captured. Hook lifecycle, `RecordSessionRoute`
  triggering conditions, and `agentdeck run` behavior stay as they are; only
  the interpretation of what was captured changes.
- No persisted attribution column and no schema migration.
- No budget rules, alerting, or menu-bar surface. Those depend on the desktop
  app and are separate `v0.6.0` items.
- No attempt to reconstruct attribution for events predating adoption.

## Tasks

### 1. `client-time-semantics`

Separate the positioning rule per client: Codex resolves by session start
boundary, Claude resolves by event time against the route sequence. Correct the
step-3 fallback that currently applies `sessionStartAt` to both. Add coverage
for a Claude session whose provider changes mid-session and a Codex session
that spans a switch without restarting.

- Files: `internal/usage/usage.go`, `internal/usage/routes.go`, tests.
- Verification level: L2.

### 2. `determinability-quality`

Redefine `exact` as determinable, promote unambiguous routes to `exact`, keep
ambiguous ones `estimated`, and replace `historical` with `unattributed` split
into the before-adoption and coverage-gap states. Reconcile the JSON `counts`
keys, warning strings, text rendering, CLI manual, and CLI design contract.

- Depends on task 1.
- Files: `internal/usage/usage.go`, `cmd/agentdeck/` renderers,
  `docs/specs/cli-manual.md`, `docs/specs/cli-design.md`, tests.
- Verification level: L2.

### 3. `attribution-observability`

Expose why an event received its quality so the result is auditable rather than
asserted: report the resolution step that produced the attribution, and report
unattributed cost as its own bucket separate from any real-spend total.

- Depends on tasks 1 and 2.
- Files: `internal/usage/usage.go`, `internal/usage/session_usage.go`,
  `cmd/agentdeck/` renderers, tests.
- Verification level: L2.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `client-time-semantics` | [ ] | [ ] |
| 2. `determinability-quality` | [ ] | [ ] |
| 3. `attribution-observability` | [ ] | [ ] |

Tasks are strictly sequential. Commit boundaries follow task boundaries. This
plan does not authorize commits, pushes, release preparation, preflight
dispatch, or publication.

## Acceptance

- A Codex session that spans a provider switch without restarting attributes
  every one of its events to the provider loaded at process start.
- A Claude session whose provider changes mid-session attributes events before
  and after the change to their respective providers.
- No event is priced at multiplier `1` while being counted toward a real-spend
  total.
- `usage summary` distinguishes determinable, inferred, and unattributed
  totals, and the CLI manual documents the new meanings.
- The release notes state that cost totals can change for existing data.

## Starting Task

Turn a Status row into scoped development by naming its anchor:

```text
进入开发：`2026-08-13-usage-attribution-precision` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's Architecture and named task, the attribution
contract in `docs/specs/cli-design.md`, and verification routing. Tick `Dev`
only after the task's selected verification passes. An independent reviewer
records a PASS round under
`docs/reviews/2026-08-13-usage-attribution-precision/<task-anchor>.md` before
ticking `Review`.
