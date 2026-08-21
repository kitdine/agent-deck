---
status: active
created: 2026-08-20
updated: 2026-08-20
---

# Work Signals — Requirements

## Why this topic exists

The desktop menu-bar surface specifies three modules under its `Sessions`
panel — **Activity**, **Workflow**, and **Tooling**, none of which has a value
behind it yet.

The two-level uncaptured form of those modules belongs to `desktop-app` task 3;
what the app renders today is a three-tile placeholder. Either is a starting
point this topic replaces, and both topics are in `v0.5.0`, so [`tasks.md`](tasks.md)
task 5 replaces whichever is there rather than waiting on one.

This topic supplies those values, on **two** surfaces: the menu-bar panel and
the CLI. Both are first-class and neither is derived from the other. Every other
derived counter in this product is readable from a terminal, and a measurement
only a GUI can show is also a measurement that cannot be scripted, diffed, or
checked against the store.

It is a separate topic rather than a task inside `desktop-app` because the two
answer different questions. `desktop-app` presents aggregates the store already
holds; this topic changes what the store holds — new extraction from raw session
logs, new persisted columns, a narrowed privacy boundary, and new wire fields.

## This document set replaces an earlier one

A first design pass at this topic was written, reviewed for fourteen rounds, and
discarded on 2026-08-20 without being committed. It is preserved under
[`docs/archive/work-signals-superseded-2026-08-20/`](../../archive/work-signals-superseded-2026-08-20/).

It was discarded because the review rounds were converging on document-internal
consistency while the design underneath rested on four decisions the user had
never been asked about: the classification unit, the privacy boundary around
tool arguments, how cost reaches an activity kind, and the CLI's shape. Six of
the fourteen rounds repaired contradictions that earlier rounds of the same pass
had introduced.

Six decisions are now made, by the user, and recorded in
[`architecture.md`](architecture.md): the four above plus the category set and
the replacement of iteration depth by the rework counter. Everything in this document set derives
from them. Where a decision has a cost, this document states the cost rather
than the conclusion alone.

## The design specimen

The prototype at [`/prototype/`](../../../prototype/) is the design truth for
both surfaces. It moved to the repository root in this topic because it is a
specimen of the whole product, not an asset of one topic, and because this topic
needed to add a CLI surface to it rather than start a second prototype.

Where a document and the prototype disagree, the prototype is right and the
document is repaired. The CLI page renders the command's literal character
output, not a sketch of it, so that its three designable properties — sectioning,
column alignment, and what an undeterminable value prints as — are reviewable in
their final form.

## What already exists

The premise that nothing exists behind these modules is only partly true, and
the difference decides the shape of the work:

| Module | What exists today | What is missing |
| --- | --- | --- |
| Tooling | `internal/activity` parses tool-call transitions and `usage_tool_calls` (schema v13) persists client, session, model, tool name, start/end, status, and duration | Tool-kind grouping, share of calls, and MCP server identity |
| Activity | Nothing. No classification exists at any layer | The classifier, its category set, its turn unit, and cost attribution |
| Workflow | Session start/end times and per-session tool sequences | First-edit latency, files touched, rework count, edits per session, and most-touched file |

Two facts constrain everything below, and both were verified in the source
rather than assumed:

- `internal/activity` states its own boundary in its doc comment: it retains
  "only allowlisted session and tool-call metadata" and deliberately keeps no
  fields for arguments, results, command text, environment, or reasoning. Three
  Workflow metrics need a file path, which is a tool argument.
- **Codex exposes no file path at all.** Its whole tool vocabulary is
  `exec_command`/`exec`/`js`/`write_stdin` plus agent-orchestration and
  `update_plan` calls; reads, writes, and searches all go through the shell, and
  `exec_command`'s arguments are `cmd` and `workdir` only. Verified across twelve
  sessions. Paths are parsed out of the `cmd` string there (`architecture.md`
  Decision 2), which makes `files_touched`, `top_file`, `retries`, and
  `first_edit_seconds` **lower bounds on Codex** — a documented limit of the
  measurement, surfaced on both surfaces rather than hidden.
- `usage_events` records cost per API event with a session id and a timestamp,
  and carries **no turn association**. Codex logs carry a `turn_id`; Claude logs
  carry none.

## Goals

1. Classify each **turn** into one of four activity categories — coding,
   debugging, conversation, delegation — each with at least two subcategories,
   and attribute cost and event count to each.
2. Derive the five workflow metrics the surface names, including the
   most-touched file.
3. Group tool calls into the five tool kinds Decision 7 fixes — `bash`, `read`,
   `edit`, `mcp`, and the `other` residual — report each kind's share of calls,
   and identify the top MCP server.
4. Persist the derived signals in the core database, re-derivable from source
   logs, with a migration that backfills already-indexed sources.
5. Extend the desktop wire projection additively, without raising
   `wire_version`, so a host carrying the new decoding reads an older payload as
   `available: false`, and a host built before this topic ignores the unknown
   families and keeps rendering whatever uncaptured form it was built with.
   `wire_version` must stay unchanged because the host guards it with an exact
   equality check, so raising it makes an older host reject the whole snapshot
   rather than degrade.
6. Replace the uncaptured cards that `desktop-app` task 3 delivers with the
   captured rendering, in both languages and both appearances.
7. Ship the same signals on the CLI: as part of `agentdeck usage stats`'s
   default output, and through a dedicated `agentdeck usage signals` command
   that filters.

## The privacy boundary, as narrowed

This is the one non-goal this topic changes, so it is stated here rather than
left to be discovered in `architecture.md`.

**What may now be read** at derivation time, in process: tool arguments, user
message text, and shell command strings. The classifier needs them; a classifier
that sees only which tools a turn called cannot separate coding from debugging,
because both are edit-plus-execute.

**What may be persisted** is unchanged in kind and narrowed in one place:

| Value | Persisted | Why |
| --- | --- | --- |
| File base name (`tasks.md`) | Yes | The surface names a most-touched file, and a digest cannot be displayed |
| File path digest, salted with the machine identity | Yes | Distinct-file counting needs an identity that is opaque and does not travel: an unsalted digest of a path is recoverable from a candidate list and is identical on every machine |
| Directory structure, absolute or relative path | No | Not needed by any metric on either surface |
| User message text | No | Read to classify; only the resulting category is stored |
| Shell command string | No | Read to classify; only the resulting category is stored |
| Tool results, environment, reasoning | No | Unchanged |

The cost of this is precise and is accepted: the package's doc comment stops
saying it never touches arguments and starts saying what it never retains. That
is a weaker guarantee, and it is only worth anything if it is tested. Task 1
therefore carries a mandatory negative test, and it is a completion condition
below, not a nice-to-have.

## Non-goals

- No new network access. Classification is local and offline.
- No retention of the values marked **No** in the table above.
- No cross-device aggregation. Signals are per-machine.
- No machine-learning model, no external classifier service, and no new
  dependency for classification.
- No change to the Widget surface. The three modules ship on the menu-bar panel
  and the CLI.
- No cost figure on the Tooling module. A tool call consumes no tokens; the turn
  that issued it does. Printing a dollar amount next to `bash` would present an
  apportionment as a measurement.

## Acceptance boundary

This topic is complete when all of the following hold:

1. For a machine with indexed Codex and Claude sessions, the menu-bar `Sessions`
   panel renders Activity, Workflow, and Tooling with real values in both
   languages and both appearances, and no card reads as uncaptured except where a
   signal is genuinely unavailable for the selected scope.
2. `agentdeck usage stats` includes the three sections in its default output
   with no flag, and `agentdeck usage signals` renders them for a scope that
   filters by module, category, client, and period.
3. A figure read in the panel is reproducible from a terminal by naming the same
   `--period` and `--client`, for the three periods both surfaces share.
4. Every displayed number is traceable to source-log evidence and reproducible
   by re-deriving from the same sources.
5. A signal that cannot be determined for a scope renders as unavailable rather
   than as zero, on both surfaces, and the CLI still exits `0`.
6. A test asserts that a source log whose tool arguments carry a full path, a
   directory name, a command string, a user message, and a result body leaves
   **only** the salted digest and the base name behind. The assertion covers the
   database, emitted log lines, error and warning strings, the `Page`/`Detail`
   JSON, and the source-file cache — the guarantee is "dropped before the record
   is constructed", which is broader than the store — and it matches on
   **substrings**, because `internal/activity` truncates metadata at
   `maxMetadataLength = 256` and a truncated path fragment would pass a
   whole-string absence check.
7. The migration backfills already-indexed sources, and its behavior on a
   database with no prior tool-call rows is defined and tested.
8. A payload predating the new wire families decodes as `available: false` and
   `wire_version` is unchanged.
9. Both filters — client scope and period — govern the three modules exactly as
   they govern the rest of the panel.

## Backlog / future ideas

- **The three sections in `usage stats --interactive`.** The text form and
  `usage signals` carry them; the interactive viewer
  (`cmd/agentdeck/usage_stats_viewer.go`) does not, because a navigable module
  there needs panes, key bindings, and a detail level the text form has no
  equivalent for. This is deferred work with a recorded reason, not a capability
  judged unnecessary.
- Work signals on the Widget surface, if a size ever justifies them.
- Trend comparison of a signal against the previous equivalent period.
- User-defined categories beyond the fixed four and their subcategories.
- Per-branch or per-pull-request attribution, which the turn unit would support
  and which no surface currently asks for.
