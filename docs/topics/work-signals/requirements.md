---
status: active
created: 2026-08-20
updated: 2026-08-20
---

# Work Signals — Requirements

## Why this topic exists, and why it exists separately

The desktop menu-bar surface specifies three modules under its `Sessions`
panel — **Activity**, **Workflow**, and **Tooling** — in
[`../desktop-app/ux/menubar.md`](../desktop-app/ux/menubar.md), derived from the
reviewable prototype at
[`../desktop-app/ux/prototype/interactive-v7/`](../desktop-app/ux/prototype/interactive-v7/).
`menubar-experience` ships all three, with their real headings and layout, in a
`Not captured yet` state, because no projection field supplies their values.

This topic supplies those values and turns the pending cards into real ones.

It is a separate topic rather than a task inside `desktop-app` because the two
answer different questions. `desktop-app` presents aggregates the store already
holds; this topic changes what the store holds — new extraction from raw session
logs, new persisted columns, a new privacy boundary decision, and new wire
fields. Folding a data-pipeline change into a presentation task would hide it.

**It is not a deferral.** Both topics are in `v0.5.0`. A 2026-08-18 decision
recorded the capability as "refused in this topic" and moved it to the
repository Backlog; that removed a committed feature from the version without
asking and was reversed on 2026-08-20.

## What is already there

The premise that no field exists behind these modules is only partly true, and
the difference decides the shape of the work:

| Module | What exists today | What is missing |
| --- | --- | --- |
| Tooling | `internal/activity` parses tool-call transitions from raw logs and `usage_tool_calls` (schema v13) persists client, session, model, tool name, start/end, status, and duration. `activity.Summary.ByTool` already counts calls per tool | Cost attributed per tool, tool-kind grouping (`bash`/`read`/`edit`/`mcp`), and MCP server identity |
| Activity | Nothing. No classification of what a session was doing exists at any layer | The classifier itself, its kind set, and its cost/event attribution |
| Workflow | Session start/end times and per-session tool sequences | First-edit latency, files touched, iteration depth, edits per session, and most-touched file |

`internal/activity` states its own boundary in its doc comment: it retains "only
allowlisted session and tool-call metadata" and deliberately keeps no fields for
arguments, results, command text, environment, or reasoning. Two Workflow
metrics — files touched and most-touched file — need a file path, which is a
tool argument. That collision is the central design question of this topic and
is decided in [`architecture.md`](architecture.md), not here.

## Goals

1. Classify each session's work into the four activity kinds the surface names —
   coding, debugging, conversation, delegation — and attribute cost and event
   count to each.
2. Derive the four workflow metrics and the most-touched file the surface names.
   All five are derivable — [`architecture.md`](architecture.md) Decisions 1, 2,
   and 4 show how: Decision 1 supplies the file identity, Decision 2 defines the
   classification unit that `iteration_depth` divides by and that
   `first_edit_seconds` measures from, and Decision 4 stores both — so none of
   them may be left permanently pending. If one is ever
   found to be underivable, removing it from the surface is a scope reduction of a
   `v0.5.0` feature: it requires explicit user approval, a recorded decision in
   [`tasks.md`](tasks.md), and a matching change to the document that owns the
   surface. It is never a call an implementation task makes on its own. See "It is
   not a deferral" above for why this gate exists.
3. Attribute cost per tool kind, group tool calls into the four kinds the surface
   names, and identify the top MCP server.
4. Persist the derived signals in the core database, re-derivable from source
   logs, with a migration that backfills already-indexed sources.
5. Extend the desktop wire projection additively, without raising
   `wire_version`, so a host carrying the new decoding reads a payload that
   predates the new families as `available: false`, and a host built before this
   topic ignores the unknown families and keeps rendering its shipped
   `Not captured yet` form. `wire_version` must stay unchanged because the host
   guards it with an exact equality check, so raising it makes an older host
   reject the whole snapshot rather than degrade.
6. Replace the `Not captured yet` cards in the shipped menu-bar app with real
   values, keeping the prototype's layout, copy, and both languages unchanged.
7. Deliver the signals on **two** first-class surfaces — the menu-bar panel and
   the CLI — designed independently and neither derived from the other. Every
   other derived counter in this product is readable from a terminal; a
   measurement only a GUI can show is also a measurement that cannot be
   scripted, diffed, or checked.

## Non-goals

- No new network access. Classification is local and offline, like every other
  AgentDeck derivation.
- No retention of prompt text, tool arguments beyond what
  [`architecture.md`](architecture.md) explicitly authorizes, results, command
  text, or reasoning.
- No cross-device aggregation. Signals are per-machine, like all other stores.
- No machine-learning model, no external classifier service, and no dependency
  added for classification.
- No change to the Widget surface. The three modules ship on the menu-bar panel
  and the CLI; the Widget is out of scope.
- No change to the prototype. It is the design specimen and stays as it is.

## Acceptance boundary

This topic is complete when all of the following hold:

1. For a machine with indexed Codex and Claude sessions, the menu-bar
   `Sessions` panel renders Activity, Workflow, and Tooling with real values in
   both languages and both appearances, and no card reads `Not captured yet`
   except where a signal is genuinely unavailable for the selected scope.
   `agentdeck usage signals` renders the same three modules for the same scope,
   and a figure read in the app is reproducible from a terminal by naming the
   same `--period` and `--client`.
2. Every displayed number is traceable to source-log evidence and is
   reproducible by re-deriving from the same sources.
3. A signal that cannot be determined for a scope renders as unavailable rather
   than as zero, following the surface's existing unavailable-versus-empty rule.
4. The privacy boundary decided in `architecture.md` is enforced in code and
   covered by tests that assert what is *not* persisted.
5. The migration backfills already-indexed sources, and its behavior on a
   database with no prior tool-call rows is defined and tested.
6. A payload predating the new wire families decodes as `available: false` and
   `wire_version` is unchanged.
7. Both filters — client scope and period — govern the three modules exactly as
   they govern the rest of the panel.

## Backlog / Future Feature Ideas

- Work signals on the Widget surface, if a size ever justifies them.
- Trend comparison of a signal against the previous equivalent period.
- User-defined activity kinds beyond the fixed four.
