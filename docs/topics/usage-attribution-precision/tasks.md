---
status: active
created: 2026-08-13
updated: 2026-08-26
---

# Usage Attribution Precision — Tasks

This file is the only status authority for this topic.

## Task breakdown

### 1. `client-time-semantics`

Make both attribution read paths follow effective session state and one shared
resolution-reason policy. Codex resolves from its `SessionStart` boundary.
Claude resolves effective routes by event time, but the provider-timeline
fallback remains positioned at session start: a global selection change is not
proof that an already-running session adopted it. Add parity coverage for
`readPriceResolver.priceForEvent` and `Service.priceForEvent`, including all
three Claude authentication transitions — `no key -> first key`, `key A -> key
B`, and `key -> no key` — plus restart and a Codex session that spans a switch.

- Prerequisite: `switch-effectiveness-boundary` tasks 1 and 3 have Review PASS,
  so every accepted client Hook uses the shared observation operation, only the
  first-key transition writes a matched live Claude route, Codex remains
  restart-only, and unadopted changes retain the prior route. This task reads the
  effective-route stream only. The persisted route-quality writer is unchanged.
- Files: `internal/usage/usage.go`, `internal/usage/session_usage.go`,
  `internal/usage/usage_test.go`, `internal/usage/routes_test.go`.
- Verification level: L2.

### 2. `determinability-quality`

Derive quality at read time: promote known-provider effective routes to `exact`,
keep `provider = unknown` routes `estimated`, and replace the fallback quality
`historical` with `unattributed`. Ignore the persisted route-quality value as a
resolver verdict without changing its writer or backfilling rows. Align desktop
quality tiers strictly from attribution quality, propagate spend eligibility,
and regenerate every affected canonical producer fixture.

- Depends on task 1.
- Files: `internal/usage/usage.go`, `internal/usage/presentation.go`, their
  focused tests, `internal/desktop/fixtures_test.go`,
  `internal/desktop/desktop_test.go`, `desktop/fixtures/v1/*.json`, and affected
  macOS presentation/Widget tests. Regenerate fixtures with
  `AGENTDECK_UPDATE_FIXTURES=1 go test ./internal/desktop`; never hand-edit them.
- Verification level: L2.

### 3. `attribution-observability`

Expose why an event received its quality so the result is auditable rather than
asserted. Add the six-key `attribution_reasons` JSON object and matching text
section; distinguish `before_adoption` from `coverage_gap` with a timeline
existence check; exclude every non-spend-eligible event from provider-cost
totals; and report attributable real spend separately from
`unattributed_catalog_base_cost`. Reconcile the warning strings, CLI contracts,
desktop summary payload, and release-note input with the same field semantics.

- Depends on tasks 1 and 2.
- Files: `internal/usage/usage.go`, `internal/usage/presentation.go`,
  `internal/store/providers.go`, `cmd/agentdeck/` summary renderers and tests,
  `internal/desktop/` producer tests, `desktop/fixtures/v1/*.json`,
  `docs/specs/cli-manual.md`, `docs/specs/cli-design.md`, and the v0.5.0
  release-note input.
- Verification level: L2.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [ ] |
| architecture.md | [x] | [ ] |
| tasks.md | [x] | [ ] |
| `ux/` | n/a | n/a |

The `ux/` row is stated rather than omitted so a reader can tell a decision from
an oversight: no command, prompt, or navigation surface is added. Existing
`usage summary` text/JSON and desktop presentation payloads do change, but their
schema, copy, quality mapping, cost boundary, and producer-fixture rules are
fully specified by `architecture.md`; they do not require a separate interaction
design.

Design Review Round 1 (2026-08-26): all three documents **REOPEN**.
`requirements.md` fails on R1-F1 through R1-F3, `architecture.md` on A1-F1
through A1-F5, and this file on T1-F1 through T1-F3. The blocking themes are
that the Evidence Baseline's decisive observation measures open runs rather
than exactness, that the promotion of a route to `exact` has no stated
mechanism against a persisted column hardcoded to `estimated`, that a second
resolver and the desktop presentation surface and its canonical fixtures are
absent from the design and every `Files` list, and that the unattributed split
has no observable contract shape. The records under `reviews/` own the
findings and evidence. Development stays blocked until the documents pass.

Repair Round 1 (2026-08-26): R1-F1..F3, A1-F1..F5, and T1-F1..F3 are addressed
in the three design documents and their review records. The repaired design now
rests the baseline on event quality, chooses read-time derivation across both
resolvers, makes the unattributed reason and real-spend shapes observable, and
includes presentation code plus canonical fixture generation in task scope.
All three documents await independent Re-review; development remains blocked.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `client-time-semantics` | [ ] | [ ] |
| 2. `determinability-quality` | [ ] | [ ] |
| 3. `attribution-observability` | [ ] | [ ] |

Tasks are strictly sequential. Commit boundaries follow task boundaries. This
topic does not authorize commits, pushes, release preparation, preflight
dispatch, or publication.

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
开发：`usage-attribution-precision` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the attribution contract in
`docs/specs/cli-design.md`, and verification routing. Tick `Dev` only after the
task's selected verification passes. An independent reviewer records a PASS
round under `reviews/<task-anchor>.md` before ticking `Review`.
