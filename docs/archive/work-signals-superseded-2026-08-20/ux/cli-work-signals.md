---
status: active
created: 2026-08-20
updated: 2026-08-20
---

# Work Signals — CLI surface

## Why this is its own surface, not a GUI leftover

The three signals are derived counters over the local store, and every other
derived counter in this product is readable from a terminal. A signal that only
the menu-bar app can see would be the first measurement in AgentDeck that cannot
be checked, scripted, or diffed without a GUI — which also makes it the first one
that cannot be verified.

So the CLI is designed here as a first-class surface with its own state set and
its own copy, not as a JSON dump of what the wire happens to carry. The binding
rule runs the other way round from the usual assumption: the wire projection and
the CLI both read the same store, and **neither is derived from the other**.

## Command shape

One new subcommand under the existing `usage` group, which is where every other
cross-session aggregate already lives:

```text
agentdeck usage signals [flags]
```

It reuses the group's established flags with identical meaning — a flag that
means one thing in `usage stats` MUST NOT mean another here:

| Flag | Meaning | Default |
| --- | --- | --- |
| `--period` | `today`, `7d`, `30d`, `week`, `month`, `6m`, `all` | `7d` |
| `--from` / `--to` | Inclusive local date range, both required together | unset |
| `--client` | `codex` or `claude` | unset, meaning both |
| `--module` | `activity`, `workflow`, `tooling`, or `all` | `all` |
| `--no-scan` | Use the stored aggregate without scanning sources | `false` |
| `--format` | The global `text` / `json` selector | `text` |

`--period` and `--client` carry the menu-bar panel's two filters, and the
correspondence is the point of the flag choice — but it is a **subset
correspondence**, not an identity, and stating it as an identity would promise a
reproducibility that does not hold everywhere:

- **The panel's period set is a subset of this one.** The projection emits three
  periods — `today`, `7d`, `30d` (`internal/desktop/desktop.go:475-495`) — while
  this command accepts the `usage` group's seven. `requirements.md` Acceptance
  item 1's reproducibility guarantee is therefore bound to those three: a figure
  read in the app is reproducible from the terminal by naming the same
  `--client` and one of the three periods the panel can display. `week`, `month`,
  `6m`, and `all` have no panel counterpart to disagree with.
- **`--client` is an identity.** Both surfaces scope by `codex`, `claude`, or
  both, over the same client field.

Within those three periods the guarantee is exact, and that is what MUST NOT
drift. Making it exact requires one thing of the derivation, stated here because
this is the document whose guarantee depends on it:

> **Both surfaces MUST bucket a work signal by the same rule.** A signal record
> is per **turn**, not per session and not per event, and the rule is that a turn
> belongs to the period its `started_at` falls in. This is deliberately *not* the
> rule the panel uses for its session statistics — `desktop.go:498-503` assigns a
> *session* to a period by its last event — and the two rules coexist because
> they bucket different objects. What must not happen is the panel and the CLI
> bucketing the *same* object differently, which is what would make a turn
> straddling a period boundary appear in one period here and another there.

That rule is a data-contract decision and belongs to
[`../architecture.md`](../architecture.md) Decision 5, which states it in the
same terms, half-open on the local calendar. The block above states what this
document's guarantee requires; Decision 5 is where it is decided, and the two
agree.

The unit is called a **turn** throughout this document, never a "group".
`architecture.md` Decision 2 named it `turn` in both clients and Decision 4
renamed its column `turn_key`, precisely so that the word `groups` — which this
document also uses, at the Tooling line and in the JSON, for the count of tool
kinds — cannot be mistaken for it. The two senses are unrelated and only one of
them is spelled `group` here.

`--module` exists because the three modules answer different questions and a
script usually wants one. It selects which sections render and which families
`--format json` emits; it never changes how a rendered value is computed.

There is no `--top`. `usage stats` has one because its lists grow without bound;
none of these do. The tool-kind list is bounded at four by
[`../architecture.md`](../architecture.md) Decision 5, below any cap a user would
set, and there is no file list to cap — the workflow family carries
`top_file_base_name` and `top_file_count`, one file, not a ranking. A flag with
nothing to act on is the same defect this document turns `--group-by` down for
two paragraphs below, and symmetry with `usage stats` is not a reason to ship
one.

Rejected: a `--group-by` bucketing flag. These are per-period summaries, not a
trend, and offering a bucket that the underlying record set has no dimension for
would produce a flag that silently does nothing.

## Text output

Follows the `usage stats` conventions already in the codebase — an emoji-led
uppercase section heading with a rule to the right margin, a proportional bar
where a share exists, and a detail line beneath each bar.

```text
🧭 WORK SIGNALS · LAST 7 DAYS
Aug 14, 2026 - Aug 20, 2026 · Asia/Shanghai · codex + claude · 214 sessions

🎯 ACTIVITY ────────────────────────────────────────────────────────────────────
Coding                  ████████████████████████████████████  58.0%
58.0% · $3.06 · 24 events
Debugging               █████████████░░░░░░░░░░░░░░░░░░░░░░░  21.0%
21.0% · $1.11 · 9 events
Conversation            ███████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  12.0%
12.0% · $0.63 · 6 events
Delegation              █████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   9.0%
9.0% · $0.48 · 4 events
Cost attributed by session · work with no tool call counts as Conversation

🔁 WORKFLOW ────────────────────────────────────────────────────────────────────
First edit         2m median
Files touched      7
Iteration depth    4.2 turns / edit
Edits / session    4
Most touched       tasks.md ×4

🔧 TOOLING ─────────────────────────────────────────────────────────────────────
Bash                    ████████████████████████████████████  32 calls · $0.68
Read                    ██████████████████████████████░░░░░░  27 calls · $0.31
Edit                    ███████████████░░░░░░░░░░░░░░░░░░░░░  14 calls · $0.22
MCP                     ██████████░░░░░░░░░░░░░░░░░░░░░░░░░░   9 calls · $0.17
82 calls in 4 groups · 16.2% of cost · top MCP server codegraph · 5 calls
```

Rules that bind the rendering:

- The four activity kinds always print, in the fixed `coding` → `debugging` →
  `conversation` → `delegation` order, a kind with no work printing at `0.0%`
  with a zero cost. The tool kinds print only when they have calls. This is the
  same asymmetry the GUI surface fixes, for the same reason: activity kinds
  partition all work, tool kinds do not.
- The `Cost attributed by session` line prints only when any contributing record
  has a `cost_basis` of `session`, and states the weaker basis when bases mix. It
  is the terminal's form of the app's attributed-cost note, and it never prints
  beside a cost the store measured directly.
- A cost that no event covers prints as `—`, never as `$0.00`. A workflow value
  that is undeterminable prints as `—` on its own line while the others still
  print.
- `Most touched` prints a bare file name. It is stored that way; the CLI does
  not truncate a path, because it never receives one.
- `--no-color` and the narrow-terminal degradation follow the existing usage
  text primitives; bars collapse before labels do.
- **The activity count is labelled `events`, the wire's own field name, not
  `turns`.** `../architecture.md` Decision 2 classifies over a turn in both
  clients, so `turns` would also be accurate; the field name is preferred because
  it is what both readers share, and Decision 6 makes their wording about one
  number identical. The Conversation note is worded the same way, in the same
  terms the panel uses ([`session-work-signals.md`](session-work-signals.md)).
  `Iteration depth`'s `turns / edit` note is the prototype's own string, which
  the panel ships verbatim and this surface keeps — it is accurate for both
  clients under Decision 2 and needs no qualification.

## Empty and unavailable states

The distinction the GUI draws is drawn here too, in words rather than in
treatment, because a terminal has no greyed-out card:

| Condition | Output |
| --- | --- |
| The store has no signal rows at all — never scanned, or a database predating schema v19 | `No work signals captured yet. Run 'agentdeck usage scan' to derive them from indexed sources.` and exit `0` |
| Signals exist but the selected `--period` and `--client` scope has no sessions | `No sessions in this range.` under the header, and exit `0` |
| Signals exist for the scope but one module has no rows | That section's heading prints with `—` beneath it; the other sections render normally |
| A scan was attempted and did not complete | The existing `scan_incomplete` warning, through the existing envelope, unchanged |

Neither empty case is an error. Exit codes and the warning envelope follow the
`usage` group's established contract and this surface introduces no new code.

**Every state above has a `--format json` shape, and they are not the same
shape.** JSON is the scripted reader, so the states a script most needs to
detect are the ones it must not have to infer from prose:

| Condition | `--format json` |
| --- | --- |
| No signal rows at all | All three families present with `available: false` and an empty `items`. The families are always keys, so a script tests `available`, never key presence |
| Scope has no sessions | All three families `available: true` with an empty `items`. This is the state a script must be able to tell from the one above, and `available` is what tells it |
| One module has no rows | That family alone `available: true` with an empty `items`; the others carry theirs |
| Scan incomplete | The families as above, plus the existing `scan_incomplete` warning in the envelope, which is where `usage` already puts it |

`available: false` means the derivation has nothing to say; an empty `items`
under `available: true` means it looked and the scope was empty. Collapsing the
two would make "never captured" and "a quiet week" indistinguishable to the only
reader that cannot see the difference in prose.

`--module` filters the JSON as it filters the text: an unselected family is
absent from the payload entirely, rather than present-but-empty, because a
script that asked for one module and got three has to filter them itself. This
is the one place key presence carries meaning, and it carries the caller's own
argument back rather than a fact about the data.

## JSON output

`--format json` emits through the existing usage envelope with the command's
output name, carrying the same three families the wire projection carries and
the same field names, so a value can be correlated across the two readers
without a mapping table:

```json
{
  "activity": {
    "available": true,
    "items": [
      {"kind": "coding", "share": 58.0, "cost": "3.06", "events": 24, "cost_basis": "session"}
    ]
  },
  "workflow": {
    "available": true,
    "items": [
      {"first_edit_seconds": 120, "files_touched": 7, "iteration_depth": 4.2,
       "edits_per_session": 4, "top_file_base_name": "tasks.md", "top_file_count": 4}
    ]
  },
  "tooling": {
    "available": true,
    "items": [
      {"calls": 82, "groups": 4, "share_of_cost": 16.2, "cost_basis": "session",
       "top_mcp_server": "codegraph", "top_mcp_calls": 5,
       "rows": [{"kind": "bash", "calls": 32, "cost": "0.68"}]
    }]
  }
}
```

Identical field names, identical units, identical null semantics as the wire
contract in [`../architecture.md`](../architecture.md). Costs are decimal
strings, as everywhere else in this CLI's JSON. Nothing caps the JSON: every
family carries every row the derivation produced, and `--module` is the only
flag that changes which families appear.

## Single-session signals

`agentdeck session show <id> --activity` already prints that session's safe tool
metadata. It gains the session's own classification, because "what was this one
session doing" is the question that surface exists to answer:

```text
SIGNALS   Coding · 12 tool calls · 3 files · first edit 4m
```

One line, printed under the existing activity summary, omitted entirely when the
session has no signal row. It carries no cost, because per-session cost is
already printed by that command and repeating it under a different attribution
basis would put two numbers for one thing on one screen.

## What this surface must not do

- It MUST NOT print a value the store did not derive, including a zero standing
  in for an unknown.
- It MUST NOT print a path, a directory, a command, a tool argument, or a tool
  result. The store does not hold them, and this surface adds no second reader
  that could.
- It MUST NOT reach the network, and `--no-scan` MUST remain a pure read.
