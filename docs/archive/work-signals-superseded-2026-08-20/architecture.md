---
status: active
created: 2026-08-20
updated: 2026-08-20
---

# Work Signals — Architecture

The requirement is in [`requirements.md`](requirements.md). The surface these
signals feed is specified in
[`../desktop-app/ux/menubar.md`](../desktop-app/ux/menubar.md) and demonstrated
by [`../desktop-app/ux/prototype/interactive-v7/`](../desktop-app/ux/prototype/interactive-v7/);
this document decides how the values behind it are produced, stored, and
projected.

## What the source logs actually give us

Every decision below rests on what `internal/activity/activity.go` can see, so
that is stated first. These are properties of the current parser and of the two
clients' log formats, verified against the code, not assumptions.

| Fact | Where | Consequence |
| --- | --- | --- |
| Tool calls are already parsed from both clients — `function_call`, `custom_tool_call`, `mcp_tool_call`, `web_search_call`, `computer_call` for Codex; `tool_use` / `tool_result` for Claude | `parseCodex`, `parseClaude` | Tooling needs no new extraction pass, only new fields on the existing one |
| Tool calls are already persisted with client, session, model, tool name, start, end, status, and duration | `usage_tool_calls`, schema v13 | The Tooling module's call counts are derivable today |
| **Codex reports no tool failure.** Its output items are recorded with a hardcoded `"completed"`; only Claude's `tool_result.is_error` produces `"failed"` | `parseCodex` vs `parseClaude` | Any classifier rule keyed on failure rate would silently classify all Codex work as non-debugging. Ruled out below |
| **`turn_id` is Codex-only.** It comes from Codex's `turn_context` event; the Claude parser never sets it, and no table persists it | `Parser.turnID`, `usage_tool_calls` schema | Turn-level cost attribution is unavailable for Claude. Decided below |
| **Tool arguments exist in the source and are deliberately discarded.** Codex's `function_call` carries `arguments`, Claude's `tool_use` carries `input`; the parser reads neither, and `Detail` documents that it "deliberately has no fields for arguments, results, command text, environment, or reasoning" | `internal/activity/activity.go:20-24` | File-path-derived Workflow metrics require crossing a stated privacy boundary. Decided below |
| Usage cost events carry client, session, model, timestamp, and tokens — but no turn and no tool | `usage_events`, schema v1 with later columns | Per-tool cost is an attribution, never a measurement. Decided below |

## Decision 1 — the privacy boundary for file paths

Two Workflow values the surface names need a file identity: `Files touched` (a
distinct count) and `Most touched` (a displayed name, rendered by the prototype
as `tasks.md ×4`). Both come from a tool argument, which the current extractor
refuses to retain.

**Decided: extract file paths, persist neither the path nor the directory.**

For each tool call classified as touching a file, the extractor derives two
values and keeps only those:

| Persisted | Derivation | Why it is safe to keep |
| --- | --- | --- |
| `path_digest` | `sha256` over the cleaned absolute path, truncated to 16 bytes hex | Supports distinct counting and per-file tallying without being reversible to a path |
| `base_name` | The final path element only, `filepath.Base`, rejected if longer than the existing `maxMetadataLength` of 256 or if it fails the parser's existing safe-string check | Supports `Most touched`. It exposes no directory, no user name, no project tree, and nothing above the file itself |

What is **not** persisted, and must be covered by a test asserting absence: the
full path, any parent directory, the raw argument object, any other argument
key, tool results, and command text. The argument object is parsed, read for the
path key, and dropped in the same function — it never reaches a struct field
that outlives the call.

The residual exposure is a bare file name. That is accepted: the `Sessions`
panel already displays project names, so a file's own name does not widen the
surface's disclosure class. The gain is that both metrics become real instead of
permanently pending.

`internal/activity`'s package doc comment and the `Detail` comment both state
the old absolute boundary and MUST be updated in the same change, because a
comment that contradicts the code is how the 2026-08-18 premise ("no field
behind it today") got written.

Which tool calls count as touching a file is a fixed, client-specific mapping,
not a heuristic over argument shapes:

| Client | Tool names treated as file-touching | Path key |
| --- | --- | --- |
| Claude | `Edit`, `Write`, `NotebookEdit`, `Read` | `file_path`, then `notebook_path` |
| Codex | `apply_patch`, `write_file`, `read_file`, and the `shell` call only when its command is a single-file redirect the extractor can parse unambiguously | `path`, then `file_path` |

A tool not in the mapping contributes no file identity, even if its arguments
happen to contain something path-shaped. An unrecognized or unparsable argument
contributes nothing and is not an error.

## Decision 2 — the activity classifier

The surface fixes four kinds — coding, debugging, conversation, delegation —
and the prototype attributes a share, a cost, and an event count to each.

**Decided: a deterministic rule set over the tool sequence of one turn, with a
fixed precedence and no scoring.** No model, no dependency, no heuristics over
free text.

The unit of classification is a **turn** in both clients. It is the same object
in both — one assistant reply and whatever tool calls that reply made — and only
its boundary marker differs, because the two logs mark it differently:

| Client | A turn is | Boundary marker |
| --- | --- | --- |
| Codex | The span opened by a `turn_context` event, up to the next one | `turn_context`, whose `turn_id` the parser already reads and currently discards (`Parser.turnID`) |
| Claude | One assistant message carrying a non-synthetic `model`, **together with the tool calls that follow it**, up to the next such message | The assistant message itself, whose `model` the parser already tracks |

What this requires of the extractor, stated because the current parser does not
do it: `parseClaude` reads `message.model` and skips `<synthetic>`, so it
already sees every boundary — but it only uses the value to stamp a model onto
tool calls and emits nothing for the message itself. A turn that made no tool
call therefore produces no record at all today. Task 1 must emit a turn on the
boundary, not on the first call inside it, or `conversation` stays unreachable
for exactly the reason this decision was rewritten to fix. Codex needs no
equivalent change: `turn_context` is already a distinct event the parser handles.

**A turn with no tool call is still a turn.** That is the whole point of
anchoring Claude's turn on the assistant *message* rather than on the run of
calls: an earlier draft defined the Claude unit as "the maximal run of tool calls
between two consecutive assistant messages", and a reply that called no tool
forms an empty run — so no unit existed to carry a classification, and
`conversation` was unreachable for every Claude user while the Activity module
renders four kinds whose shares must sum. That is the same per-client bias this
decision refuses failure rate for, arriving through the unit definition instead
of through a rule.

Because the unit is now one object with two markers rather than two objects, the
surfaces may say `turn` to a user of either client, and `iteration_depth`'s
`turns / edit` — the prototype's own label — is accurate for both.

Precedence is evaluated top to bottom; the first match wins, and the fourth row
is a default rather than a test, so **every turn carries exactly one kind**:

| Order | Kind | Rule |
| --- | --- | --- |
| 1 | `delegation` | The turn contains a sub-agent dispatch tool — Claude `Task`, Codex `mcp_tool_call` targeting a known agent-spawning server. Delegation outranks everything because the work was handed off, whatever the delegating turn also did |
| 2 | `debugging` | The turn contains at least one file-touching *write* call **and** at least one test/diagnostic execution — a `shell`/`Bash` call whose recorded tool name is the execution tool and which is followed in the same turn by another write to a file already written earlier in the same session. Repetition on the same file is the signal, and it is available without reading any command text |
| 3 | `coding` | The turn contains any file-touching write call |
| 4 | `conversation` | **Anything else.** A turn with no tool call at all, and equally a turn that only read, only searched, or only ran something without writing |

Row 4 is written as a default because it is the row an implementer codes as
`else`. An earlier draft tested it for "no tool call at all" and left the
read-only turn to a sentence of prose beneath the table — two different rules,
one of which was the artifact anyone would actually implement, and a literal
reading left read-only turns carrying no kind at all, which no surface can
render.

Classifying a read-only turn as `conversation` is a definition, not a
measurement: from the user's point of view nothing was produced. It is stated in
the surfaces' help text rather than left for the user to infer.

**Failure rate is explicitly not a rule input**, because Codex reports no
failures and a rule keyed on it would produce a systematic per-client bias that
looks like a finding about the user's work.

## Decision 3 — cost attribution

Per-tool and per-activity-kind cost cannot be measured. `usage_events` carries
cost per model per session with no tool and no turn dimension
(`internal/store/migrations.go:39`), so every figure below is an attribution
derived from timestamps, and the contract's job is to make the derivation
explicit and its basis visible rather than to imply a precision it does not
have.

**Decided: attribute, label the attribution, and never present it as measured.**

A turn has no cost of its own to divide, so the first thing to state is how it
acquires one. `usage_events` rows carry `client`, `session_id`, `event_at`, and
tokens; a turn is a time span within one session. **A turn's cost is the sum of
the events whose `event_at` falls in that turn's span**, where the span runs
from the turn's boundary marker up to the next turn's, and the last turn of a
session runs to the session's end. Events are point-in-time rows, so no event
belongs to two turns and none needs splitting.

Two boundary cases follow from that and are decided here rather than left to an
implementer:

| Case | Rule |
| --- | --- |
| A turn's span contains no event | The turn has no cost. Its tool calls carry none, and its signal row records `cost_basis: none` — it is not a zero, and it does not borrow from a neighbour |
| An event precedes the session's first turn boundary | It is attributed to the first turn. The alternative is discarding it, which would make the sum of a session's turns quietly smaller than the session's own cost |

With that established:

| Scope | Rule |
| --- | --- |
| Codex | Turn-level. `usage_tool_calls` gains a `turn_id` column (the parser already computes the value and discards it); the turn's cost, obtained as above, is split evenly across the tool calls in that turn |
| Claude | Session-level. The session's cost is split **across its turns, weighted by each turn's tool-call count**; within a turn it is then split evenly across that turn's calls. A turn with no tool call receives no weight and no cost |
| Either, when no cost event covers the scope | The signal reports call counts and no cost, and the surface renders the cost column as unavailable — not as `$0.00` |

The Claude rule is stated as a split across *turns* rather than across *calls*
because the unit that needs a cost is the turn — `usage_work_signals` holds one
row per turn (Decision 4), and the Activity module sums per kind, which is a
property of turns. Read as a split across calls, the per-turn weighting would be
redundant and the rule would degenerate to an even per-call split; the two
readings agree on a session's total and disagree on every per-kind figure the
moment turns differ in size. No tie-break is needed for turns of equal count,
since equal weights divide evenly.

Why the two clients differ at all: Codex marks turn boundaries in its log and
Claude does not carry per-turn usage, so Codex can be attributed at the finer
scope and Claude cannot. The difference is in precision, not in method, and it
is reported rather than hidden.

The distinction is carried into the wire as a per-record `cost_basis` of
`turn` | `session` | `none`, so the host can render the existing `≈`
incompleteness treatment truthfully rather than the surface inventing a
precision claim. A single displayed figure mixing both bases reports the weaker
one.

## Decision 4 — storage

One migration, schema **v19**, additive only:

```sql
ALTER TABLE usage_tool_calls ADD COLUMN turn_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_tool_calls ADD COLUMN tool_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_tool_calls ADD COLUMN path_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_tool_calls ADD COLUMN base_name TEXT NOT NULL DEFAULT '';

CREATE TABLE usage_work_signals (
  signal_key   TEXT PRIMARY KEY,
  client       TEXT NOT NULL,
  session_id   TEXT NOT NULL,
  turn_key     TEXT NOT NULL,
  started_at   TEXT NOT NULL,
  activity_kind TEXT NOT NULL,
  tool_calls   INTEGER NOT NULL DEFAULT 0,
  write_calls  INTEGER NOT NULL DEFAULT 0,
  cost_basis   TEXT NOT NULL,
  source_path  TEXT NOT NULL,
  source_offset INTEGER NOT NULL
);
CREATE INDEX usage_work_signals_started_at ON usage_work_signals(started_at);
CREATE INDEX usage_work_signals_client_session ON usage_work_signals(client, session_id);
```

One row per turn, keyed by `turn_key` — Codex's `turn_id` where the log supplies
one, and a digest of the anchoring assistant message where it does not, per
Decision 2's two boundary markers. The column was called `group_key` while
Claude's unit was a run of tool calls; it is named for the unit it actually
holds now, so the schema and the classifier do not use two words for one thing.

`tool_kind` is the four-value grouping the surface renders — `bash`, `read`,
`edit`, `mcp` — derived from the tool name by a fixed per-client table, with an
unrecognized tool contributing to call totals but to no kind row. Note that the
tooling family's `groups` field in Decision 5 counts *these* kinds, not turns —
it is the prototype's `tool groups` label, and the two senses of "group" are
unrelated.

**Backfill.** Already-indexed sources carry no `turn_id`, `tool_kind`, or file
identity, and no signal rows exist. The migration bumps the existing
`usage_source_files.parser_version` so the next scan re-reads indexed sources
and fills both tables, exactly as the `v0.4.1` cache-write backfill did. A
database with no `usage_tool_calls` rows migrates to an empty, valid state, and
the surfaces render every module unavailable rather than empty — the distinction
`../desktop-app/ux/menubar.md` already fixes.

`sessions.sqlite3` is untouched. These are derived counters, not visible session
text, so they belong in the core database with the other usage aggregates.

## Decision 5 — the wire projection

Additive, and `wire_version` is **not** raised, matching the rule
`../desktop-app/architecture.md` already establishes for `presentation`.

`data.sessions` gains one family per module, each under the existing
`{ available, items }` shape and each carrying the `Client` × `Period` product
so both filters govern them exactly as they govern the rest of the panel:

| Field | Bound and shape |
| --- | --- |
| `sessions.work_signals.activity.items[]` | One record per `(client scope, period, kind)`, carrying `kind`, `share`, `cost`, `events`, and `cost_basis`. Exactly the four kinds, always all four, a kind with no work carrying zeros and `share: 0` |
| `sessions.work_signals.workflow.items[]` | One record per `(client scope, period)`, carrying `first_edit_seconds` (median), `files_touched`, `iteration_depth`, `edits_per_session`, `top_file_base_name`, and `top_file_count`. Any single value may be null when undeterminable for that scope |
| `sessions.work_signals.tooling.items[]` | One record per `(client scope, period)`, carrying `calls`, `groups`, `top_mcp_server`, `top_mcp_calls`, `share_of_cost`, `cost_basis`, and a bounded `rows[]` of at most the four tool kinds with `kind`, `calls`, and `cost` |

**Which period a signal falls in.** The families carry the `Client` × `Period`
product, so the membership rule has to be stated or the product means nothing:
**a turn belongs to the period its `started_at` falls in**, on the local
calendar, half-open as everywhere else — `start <= started_at < end`.
Decision 4's `usage_work_signals.started_at` is the column, and it is already
indexed for exactly this.

This is deliberately **not** the rule the panel uses for its session statistics.
`internal/desktop/desktop.go:498-503` assigns a *session* to a period by its
last event, and that stays as it is: the two rules bucket different objects, and
a session and a turn are not the same thing. What must never happen is the panel
and the CLI bucketing *the same* object differently — a turn straddling
midnight landing in one period on one surface and another period on the other
would silently break `requirements.md` Acceptance item 1, which promises that a
figure read in the app is reproducible from a terminal — a promise that binds to
the three periods both surfaces share, per Decision 6's `Filters` row, and that
this rule is what makes exact within them.

A turn is a span, so `started_at` is a choice rather than an inevitability. It is
chosen because it is the only bound the storage already has, and because a turn
is attributed to when the work began — the same convention the session rule
would give for a session that started and ended in one period, which is nearly
all of them.

Bounds are producer-enforced, as everywhere else in this contract: the host
selects a record and formats it, and performs no aggregation. `top_file_base_name`
is the only free-form string in the three families and carries the same
safe-string and 256-character constraints the extractor applied before storing
it.

A payload without `work_signals` decodes as `available: false` for all three, and
the host renders exactly the `Not captured yet` treatment
`menubar-experience` already ships. That is the compatibility property that lets
this topic land after `menubar-experience` without a coordinated release.

## Decision 6 — two readers, neither derived from the other

The signals have **two** first-class surfaces, and this is a contract decision
rather than a scoping one. The menu-bar panel is specified in
[`ux/session-work-signals.md`](ux/session-work-signals.md); the terminal surface
is specified in [`ux/cli-work-signals.md`](ux/cli-work-signals.md) and adds
`agentdeck usage signals` plus one line on
`agentdeck session show --activity`.

Both read the same store through the same derivation. Neither is generated from
the other, and neither is the fallback for the other:

| Property | Binding |
| --- | --- |
| Field names and units | Identical across the wire projection and the CLI's JSON, so a value correlates across readers without a mapping table |
| Filters | **A subset correspondence, not an identity.** `--client` is an identity. `--period` is not: the projection emits three periods (`internal/desktop/desktop.go:480-487`) and the CLI accepts the `usage` group's seven (`cmd/agentdeck/main.go:3095`), so `requirements.md` Acceptance item 1's reproducibility guarantee binds to those three, where it is exact. The other four have no panel counterpart to disagree with |
| Null and unavailable semantics | Identical. Neither surface may substitute a zero for an unknown, and `cost_basis` governs both |
| Bounds | Producer-enforced once, in the derivation. Neither surface aggregates, and **nothing caps either reader** — every family carries every row the derivation produced. There is no `--top`: these families are already bounded, and `ux/cli-work-signals.md` records why the flag must not return. `--module` is the only flag that changes which families appear |
| Copy | Independent. A terminal has no greyed-out card, so the unavailable states are worded rather than styled, and that wording is the CLI document's to decide |

The consequence for task order: the CLI surface depends on the derivation, not
on the wire projection, so it MUST NOT be scheduled as a follow-on to the GUI
work or built by reading the Swift host's behavior.

## Boundaries this topic does not move

- No network access is added. Classification, attribution, and storage are local.
- No plaintext credential, prompt, tool result, command text, or directory path
  is persisted.
- `wire_version` is unchanged and every addition is additive.
- File modes, database locations, and the `~/.agentdeck/` permission rules are
  untouched.
- The prototype is not modified; it is the specimen this topic is judged against.
