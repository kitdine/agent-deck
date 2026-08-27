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

Derive quality at read time from route effect, not provider presence. Load the
existing `hook_event`, the preceding session route, and the provider timeline at
session start in both resolver paths. Promote known-provider `SessionStart`,
same-provider `ConfigChange`, and `no key -> first key` routes to `exact`; keep
key rotation/removal and any unresolved prior state `estimated`, as well as
`provider = unknown`. Replace the fallback quality `historical` with
`unattributed`, ignore the persisted route-quality verdict without backfilling
rows, align desktop quality tiers, propagate spend eligibility, and regenerate
every affected canonical producer fixture.

The table in `reviews/architecture.md` Round 2 is the acceptance fixture. A
focused regression must reproduce its complete classification: all 135
`SessionStart` rows and the other 27 `ConfigChange` rows become `exact`; the four
keyed-to-official removal rows and the 534 events they position stay `estimated`.
It must also prove that a blanket pre-cutoff exclusion would be wrong.

- Depends on task 1.
- Files: `internal/usage/usage.go`, the read-side route helpers in
  `internal/usage/routes.go` (never `recordSessionRouteConn`),
  `internal/usage/presentation.go`, `internal/usage/usage_test.go`,
  `internal/usage/routes_test.go`, `internal/usage/presentation_test.go`,
  `internal/desktop/fixtures_test.go`,
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
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |
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

Design Re-review Round 3 (2026-08-26): all three documents **PASS**. Round 2's
A2-F1, A2-F2, R2-F1, and T2-F1 are closed: step 2 now derives quality from route
effect rather than provider presence, legacy Claude `ConfigChange` rows are
classified by whether the recorded transition was adopted, and task 2 owns that
rule together with the acceptance fixture. The six-group table in
`reviews/architecture.md` Round 2 is normative for accepting the implementation —
all 135 `SessionStart` rows and 27 `ConfigChange` rows reach `exact`, while the
four keyed-to-official removal rows and the 534 events they position stay
`estimated`. CEv1 gates `usage-attribution-precision:{requirements,architecture,tasks}.md`
are VERIFIED. The design is frozen and the three implementation tasks are
dispatchable.

Repair Round 2 (2026-08-26): A2-F1, A2-F2, R2-F1, and T2-F1 are addressed in
the three design documents and review records. The repaired policy classifies
legacy `ConfigChange` effect from the preceding session route or the session-start
provider timeline, never from a coarse cutoff; names both resolver caller sets
accurately; bounds recomputation by evidence; and assigns the classifier plus
the complete reference-fixture regression to task 2. All three documents await
independent Re-review; development remains blocked.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `client-time-semantics` | [x] | [x] |
| 2. `determinability-quality` | [x] | [x] |
| 3. `attribution-observability` | [ ] | [ ] |

Development Task 1 (2026-08-26): `client-time-semantics` is complete and awaits
independent Review. Both pricing paths now use one session-attribution resolver:
effective routes are positioned at event time and provider-timeline fallback at
session start. Both the database lookup and the in-memory route slices compare
parsed route instants, using route ID as the equal-instant tie breaker, closing
the whole-second versus fractional-second divergence.
Parity coverage passes for Claude first-key/rotation/removal/restart, Codex
switch-without-restart/restart, and timeline fallback. Stored quality values,
quality promotion, observability, and `recordSessionRouteConn` are unchanged.

Task 2 Review Round 1 (2026-08-26): **REOPEN** on D1-F1, with D1-F2 open
alongside it. The read-time classifier itself is correct — `routeQuality`
reproduces every row of the six-group reference table, and both resolvers are
driven through all seven cases against a live store — but the rename of the
fallback quality was not carried to one of its consumers: the `usage summary`
text renderer still reads `Counts["historical"]`, a key no producer writes any
more, so it prints `HISTORICAL ATTRIBUTION 0` for every store while the warning
beside it reads `unattributed attribution`, and the attribution rows no longer
sum to `EVENTS`. D1-F2 is that the focused regression's proof against a blanket
pre-cutoff exclusion is arithmetic over the test's own constants rather than
over the classifier's verdicts. The record under
`reviews/determinability-quality.md` owns the findings and evidence. The CEv1
gate is VERIFIED at the reviewed state; a VERIFIED gate is not a review verdict,
and the repair re-records evidence against the new state.

Task 2 Repair Round 1 (2026-08-26): D1-F1 through D1-F5 are repaired and await
independent Re-review. The text renderer now reads and labels `unattributed`,
the six-group regression couples its blanket-cutoff proof to classifier-exact
`ConfigChange` events, and the dual-resolver table covers both missing-prior
estimated branches. The Codex restriction now states its process-start reason,
and the exact-run branch no longer retains a dead provider check. Focused tests
and the full repository Go suite pass; the repaired CEv1 state is recorded
separately from the still-REOPEN review verdict.

Task 2 Re-review Round 2 (2026-08-26): **PASS**. D1-F1 through D1-F5 are all
closed. `usage summary` renders `UNATTRIBUTED ATTRIBUTION` from the live count
and a CLI regression asserts the rendered row is non-zero, so the three
attribution rows sum to `EVENTS` again and no live reader of the retired
`historical` key remains. The six-group regression now derives its
blanket-cutoff proof from the classifier's own verdicts and asserts 2,442
classifier-exact `ConfigChange` events. The dual-resolver live-store table
covers a move away from an already-keyed timeline prior. The Codex restriction
states its process-start reason and the exact-run dead condition is gone. The
CEv1 gate is VERIFIED across all seven criteria, re-recorded against this
round's post-synchronization state and superseding the repair-round records.

Tasks are strictly sequential. Commit boundaries follow task boundaries. This
topic does not authorize commits, pushes, release preparation, preflight
dispatch, or publication.

Task 1 Review Round 1 (2026-08-26): **REOPEN** on C1-F1 and C1-F2; Re-review
Round 2 (2026-08-26): **PASS**, both closed, CEv1 gate VERIFIED across all five
criteria. Original Round 1 finding text follows. The shared
time-positioning helper is right, but the RFC3339Nano ordering hazard the
implementation documents is fixed only on the `Service.priceForEvent` path;
`loadReadPriceResolver` still loads routes with a SQL string `ORDER BY
observed_at`, so `sort.Search` runs over an unsorted slice and the two read
paths can resolve the same event to different providers — the divergence this
task exists to remove. The record under `reviews/client-time-semantics.md` owns
the findings and evidence.

Task 1 Repair Round 1 (2026-08-26): C1-F1 and C1-F2 are addressed. In-memory
routes are re-sorted by parsed instant and ID before binary search, a mixed
fraction-length regression proves parity with the database path, and the dead
raw-string lookup is marked as forbidden pending task 2's read-side replacement.
Task 1 awaits independent Re-review; its Review cell remains open.

Development Task 2 (2026-08-26): `determinability-quality` is complete and
awaits independent Review. Both resolvers ignore persisted route quality and
derive `exact`/`estimated` from `hook_event` plus prior effective state;
unavailable fallback is now `unattributed`. The six-group reference regression
reproduces 162 exact versus 4 estimated routes and 12,395 exact versus 534
estimated Claude events, while proving a blanket cutoff would misclassify all
2,976 ConfigChange-positioned events. Presentation tiers map strictly from
quality, propagate cost incompleteness for non-spend-eligible events, and the
desktop fixtures were regenerated through their producer. Stable JSON schema
and session warning tests were updated; task 3 reason counts, CLI text
reconciliation, and real-spend total separation remain unimplemented.

Verification: focused classification/presentation tests, full `internal/usage`,
canonical desktop producer reproducibility, and `scripts/run-go-test.sh ./...`
pass. The Command Line Tools synthetic macOS verifier successfully builds and
decodes the snapshot before failing its unrelated stale 3-invocation assertion
against the current 2-invocation refresh contract; XCTest is unavailable in
this environment, so Widget XCTest was not run.

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
