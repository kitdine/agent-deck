---
status: active
created: 2026-08-20
updated: 2026-08-20
---

# Work Signals — Tasks

This file is the only status authority for this topic.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| architecture.md | [x] | [x] |
| ux/session-work-signals.md | [x] | [x] |
| ux/cli-work-signals.md | [x] | [ ] |
| tasks.md | [x] | [ ] |

This topic runs the project's normal document review. `desktop-app`'s 2026-08-20
deferral is specific to that topic, whose documents are being re-reviewed against
an implementation that is still moving; nothing here is implemented yet, so
review is what decides whether the boundary and the contracts are right before
anyone writes code.

Review order follows the dependency, not convenience:

1. `requirements.md` — the boundary decides what the surfaces may contain.
2. `ux/session-work-signals.md` and `ux/cli-work-signals.md` — either order; they
   do not depend on each other, and reviewing one says nothing about the other.
3. `architecture.md` — judged against both surfaces, especially the privacy
   boundary, the classifier rules, and the cost-attribution decision.
4. `tasks.md` — judged against all of the above, last.

Records go under [`reviews/`](reviews/), one per document and one per task.

Current review state, 2026-08-20: `requirements.md`, `architecture.md`, and
`ux/cli-work-signals.md` have passed independent re-review.
`ux/session-work-signals.md` was reopened after `architecture.md`'s repair
rewrote Decision 2 beneath it, and is back in repair. This file has not been
reviewed yet and is last, as the order below requires.

Reviewing leaf documents while their contract was still moving is what caused
that reopen: `architecture.md`'s Decision 2 repair invalidated prose in two
surfaces that had already passed. The order below is a dependency order, and it
only holds if a document is reviewed after the contract it derives from has
settled — not merely after it exists.

Two surfaces, so two `ux/` documents — the split test in `docs/README.md` asks
whether reviewing one says anything about the other, and it does not: they have
different state sets and different copy.

- `ux/session-work-signals.md` — the menu-bar `Sessions` panel in its **captured**
  state. That surface's geometry, filters, state model, and `Not captured yet`
  treatment live in [`../desktop-app/ux/menubar.md`](../desktop-app/ux/menubar.md)
  and are not duplicated.
- `ux/cli-work-signals.md` — `agentdeck usage signals` and the single-session line
  on `agentdeck session show --activity`.

Neither surface is derived from the other; see `architecture.md` Decision 6.

## Task breakdown

### 1. `work-signal-extraction`

- Contract: [`architecture.md`](architecture.md) — Decisions 1 and 4.
- Extend `internal/activity` to retain `turn_id` (Codex only), `tool_kind`, and
  the two file-identity values `path_digest` and `base_name`, using the fixed
  per-client tool mapping. Parse the argument object, read the path key, drop the
  object in the same function.
- Add schema **v19**: four columns on `usage_tool_calls` and the
  `usage_work_signals` table with its two indexes.
- Bump `usage_source_files.parser_version` so indexed sources are re-scanned and
  both tables backfill, following the `v0.4.1` cache-write precedent.
- Update the package doc comment and the `Detail` comment, which currently state
  an absolute no-arguments boundary that this task narrows. A comment left
  contradicting the code is what produced the wrong 2026-08-18 premise.
- Tests MUST include a negative one: given a source log whose tool arguments
  carry a full path, a directory name, a command string, and a result body,
  assert the database contains the digest and the bare base name and **none** of
  the other four.
- Verification level: **L3** — migration execution plus a privacy boundary.

### 2. `activity-classification`

- Contract: [`architecture.md`](architecture.md) — Decisions 2 and 3.
- Implement the four-rule classifier with its fixed precedence over the **turn**,
  which Decision 2 defines for both clients with a per-client boundary marker,
  and write `usage_work_signals`. Emitting a turn on the Claude boundary rather
  than on the first tool call inside it is required, not optional: without it a
  turn that called no tool produces no row and `conversation` stays unreachable.
- Implement cost attribution with its per-client basis and the `cost_basis`
  discriminator, including the `none` case where no cost event covers the scope.
- Tests MUST cover: each rule matching and each rule being outranked; a
  Codex-only fixture and a Claude-only fixture producing comparable
  classifications from the same unit despite the different boundary markers,
  including a Claude turn with no tool call classified as `conversation`; and the
  assertion that no rule consults tool failure status, which would bias Codex
  systematically.
- Depends on task 1.
- Verification level: **L2** — persisted derivation feeding a documented
  contract.

### 3. `work-signal-projection`

- Contract: [`architecture.md`](architecture.md) — Decision 5.
- Add the three `sessions.work_signals.*` families to the desktop projection in
  `internal/desktop`, producer-computed and producer-bounded across the
  `Client` × `Period` product, and decode them in
  `apps/macos/AgentDeckShared/DesktopWire.swift`.
- Extend the shared fixtures under `desktop/fixtures/v1/` from the same canonical
  examples on both sides, and retain a fixture **without** the new families that
  decodes as `available: false`.
- MUST NOT raise `wire_version`.
- Depends on task 2.
- Verification level: **L2** — a JSON contract with two implementations.

### 4. `work-signal-surface`

- Contract: [`ux/session-work-signals.md`](ux/session-work-signals.md).
- Replace the shipped `Not captured yet` cards with the two-level captured
  rendering: three summary rows, three detail views, the fixed orders, the fixed
  palette binding, and the state table's five bindings.
- Both languages ship together; add only the new strings the `ux` document names
  in its Copy section, and no others.
- The uncaptured rendering is **retained**, not deleted — it is the state an
  older snapshot decodes to.
- Manual acceptance on real macOS 26 covers both appearances, both languages,
  the 280 pt narrow bound, VoiceOver order through both levels, and focus return
  from a detail view to the row that opened it. Recorded under `acceptance/`.
- Depends on task 3.
- Verification level: **L2** plus manual visual and accessibility acceptance.

### 5. `work-signal-cli`

- Contract: [`ux/cli-work-signals.md`](ux/cli-work-signals.md) and
  [`architecture.md`](architecture.md) — Decision 6.
- Add `agentdeck usage signals` with the six flags that document fixes, reusing
  the `usage` group's existing flag semantics unchanged. There is no `--top`; the
  document records why it must not be added for symmetry with `usage stats`.
- Render the three text sections in the established `usage stats` style, with the
  fixed activity-kind order, the omit-when-empty tool-kind rule, the attributed-cost
  line, the `—` treatment for undeterminable values, and the narrow-terminal and
  `--no-color` degradation the existing text primitives provide.
- Emit `--format json` through the existing usage envelope with **the same field
  names and units as the wire projection**. Nothing caps either reader.
- Add the one `SIGNALS` line to `session show --activity`, omitted when the
  session has no signal row.
- Cover the four empty/unavailable states from the CLI document, each exiting `0`.
- **Depends on task 2, not on task 3.** This surface reads the derivation
  directly; scheduling it behind the GUI work, or building it by reading the
  Swift host's behavior, is exactly the dependency Decision 6 forbids. It may run
  in parallel with tasks 3 and 4.
- Verification level: **L2** — a documented CLI text and JSON contract.

### 6. `work-signals-contract`

- Reconcile the delivered behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`, including the schema v19 migration, the retained
  privacy boundary as narrowed, and the additive wire families.
- Depends on tasks 1 through 5.
- Verification level: **L2**.

## Tasks

| Task | Implemented | Reviewed |
| --- | --- | --- |
| 1. `work-signal-extraction` | [ ] | [ ] |
| 2. `activity-classification` | [ ] | [ ] |
| 3. `work-signal-projection` | [ ] | [ ] |
| 4. `work-signal-surface` | [ ] | [ ] |
| 5. `work-signal-cli` | [ ] | [ ] |
| 6. `work-signals-contract` | [ ] | [ ] |

## Relationship to `desktop-app`

The two topics are siblings in `v0.5.0` and neither blocks the other's delivery:

- `menubar-experience` ships the three modules in their uncaptured form. That is
  a complete state, not a placeholder awaiting this topic.
- This topic's task 3 is additive on the wire, so a host built before it decodes
  a newer payload as `available: false` and renders the uncaptured form.
- Task 4 is the only task that touches `desktop-app`'s Swift surface. It owns the
  Sessions-panel work-signal views and nothing else in that target.

The dependency graph is a fork, not a line: tasks 1 and 2 are the derivation,
then tasks 3→4 (GUI) and task 5 (CLI) proceed independently from task 2, and
task 6 closes over all of them. Delivering the CLI first is a valid order and is
the one that makes the derived numbers checkable while the GUI work is still in
flight.

## Starting a task

Read [`requirements.md`](requirements.md), then the contract section of
[`architecture.md`](architecture.md) this task cites, then the prototype for any
surface question. Resolve the Beads task by anchor from live state.
