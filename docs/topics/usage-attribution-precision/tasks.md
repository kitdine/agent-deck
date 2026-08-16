---
status: active
created: 2026-08-13
updated: 2026-08-16
---

# Usage Attribution Precision — Tasks

This file is the only status authority for this topic.

## Task breakdown

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

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [ ] |
| architecture.md | [x] | [ ] |
| tasks.md | [x] | [ ] |
| `ux/` | n/a | n/a |

The `ux/` row is stated rather than omitted so a reader can tell a decision from
an oversight: no command gains a new surface. `usage summary` keeps its existing
text and JSON shape while the values and their labels change, which
`architecture.md` owns as a contract change.

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
进入开发：`usage-attribution-precision` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the attribution contract in
`docs/specs/cli-design.md`, and verification routing. Tick `Dev` only after the
task's selected verification passes. An independent reviewer records a PASS
round under `reviews/<task-anchor>.md` before ticking `Review`.
