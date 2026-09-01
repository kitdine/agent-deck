---
status: historical
created: 2026-08-20
updated: 2026-08-20
retired: 2026-09-01
---

# CLI — Work Signals

`agentdeck usage stats`, `agentdeck usage signals`, and the one added line on
`agentdeck session show --activity`.

This surface has a prototype: [`/prototype/`](../../../../../prototype/), page
`?surface=cli`. It renders the commands' literal character output rather than a
sketch, because a terminal offers only three designable properties — sectioning,
column alignment, and what an undeterminable value prints as — and all three are
invisible in a sketch. Six tabs cover the six output shapes below.

The output is English only, like the rest of the CLI. The panel is bilingual;
this is not.

## Where the signals appear

**`usage stats` includes the three sections in its default output, with no
flag.** There is no `--signals` switch: a measurement a user must know to ask for
is a measurement most users never see, and the panel shows these three without
being asked.

`usage stats` renders through `renderUsageStatsWithOptions`
(`cmd/agentdeck/usage_stats_layout.go`) and its sections are
`📊 USAGE STATS · <RANGE>`, the TOKENS/COST/SESSIONS stat row, `🗓 TREND`,
`🤖 MODELS`, `CLIENTS`, `PROVIDERS`, `CACHE HIT RATE`,
`▦ ACTIVITY BY WEEKDAY / HOUR`, `COVERAGE`, `UNPRICED MODELS`, and
`DETAIL COMMANDS`. (`📊 USAGE SUMMARY`, `🪙 TOKEN TOTALS`, and
`🧾 MODEL COVERAGE` belong to `usage summary`, which is a different command.)

The three sections are inserted **after `▦ ACTIVITY BY WEEKDAY / HOUR` and
before `COVERAGE`**, so the two activity-shaped sections are adjacent rather than
scattered.

### The `ACTIVITY` name collides, and this document renames

`usage stats` already prints `▦ ACTIVITY BY WEEKDAY / HOUR`, which is about
*when* work happened. This topic's section is about *what kind* of work it was.
Two sections whose names both begin `ACTIVITY` on one screen is a defect of this
combined output, not of either section.

The existing section keeps its name — it is shipped and is not this topic's to
rename. This topic's section is titled **`🧭 WORK KIND`** inside `usage stats`.
Under `usage signals`, where there is no collision, it is titled
`🧭 ACTIVITY`, matching the panel's module name and the JSON field.

That the same data has two section titles depending on the command is a cost, and
it is the smaller one: renaming a shipped section breaks scripts that grep for
it, and leaving both as `ACTIVITY` makes the default output ambiguous at a
glance.

### The interactive viewer

`usage stats --interactive` (`cmd/agentdeck/usage_stats_viewer.go`) is a separate
rendering and **does not carry the three sections** in this topic. Adding a
navigable module to that viewer is its own design problem — panes, key bindings,
and a detail level the text form does not have — and this topic does not solve
it. The viewer is unchanged; the text form and `usage signals` carry the
signals. It is recorded in `requirements.md`'s Backlog so it stays owed rather
than quietly dropped.

**`usage signals` is the dedicated command**: the three sections alone, plus the
filters that `usage stats` has no room for. It is what a script reads and what a
person reaches for when the stats preamble is noise.

## `usage signals` flags

Reused from the `usage` group with unchanged semantics: `--period`, `--client`,
`--format`, `--no-color`.

Added by this command:

| Flag | Effect |
| --- | --- |
| `--kind <activity\|workflow\|tooling>` | Render one section instead of three. Repeatable |
| `--sub` | Expand every activity category into its subcategories |
| `--activity <category\|subcategory>` | Restrict the scope to turns of that category or subcategory. Implies `--sub` when given a category |

**`--activity` renormalizes.** Shares are recomputed against the filtered scope,
not against the unfiltered whole: `--activity debugging` prints `Debugging 100%`
with its subcategories summing to 100 beneath it, which is what the prototype's
`filter` tab shows. Cost and event counts are **not** renormalized — they are the
filtered scope's real values.

This is the one place where a figure here is not comparable with the panel, and
it is called out for that reason: the panel has no equivalent filter, so
`--activity` output has no panel figure to reproduce.

There is **no `--top`**, and the reason is that there is nothing to cap. Every
table in these three sections is bounded by its vocabulary rather than by data:
Activity has exactly four rows, its
subcategories at most four under one parent, Workflow has five fixed rows, and
Decision 7 fixes the tool kinds at five. There is no long tail for a cap to cut.
If a future section acquires an unbounded table — a per-file list, say — it takes
`--top` with the same defaults `usage stats` uses.

## Sections

### `🧭 ACTIVITY`

One row per category, in Decision 3's fixed order, each with a share bar, share,
attributed cost, and event count:

```
Coding          █████░░░░░   52%   $2.74  21 events
```

Under `--sub`, each category is followed by its subcategories, indented with
`└`, carrying share, cost, and events but no bar. The bar is the parent's
proportion of the whole; nesting a second bar inside it invites the reader to
compare two different denominators.

### `🧱 WORKFLOW`

Five aligned label/value rows, using the same label-then-value alignment the
usage family already renders with:

```
FIRST EDIT      2m (median)
FILES TOUCHED   7
REWORK          3  (edit, verify, edit again)
EDITS / SESSION 4
MOST TOUCHED    tasks.md ×4
```

`REWORK` carries its definition inline for the same reason the panel's cell
carries a note line: the label alone does not say what was counted.

### `🔧 TOOLING`

One row per tool kind with calls and share of calls, then the top MCP server.
No cost column, for Decision 4's reason.

## `session show --activity`

One added line, omitted entirely when the session has no signal row:

```
SIGNALS         Coding · 12 tool calls · 3 files · first edit 4m
```

The leading word is the session-level category, which Decision 5 defines as the
category holding the largest share of the session's attributed cost. When the
session's `cost_basis` is `none` that reduction has no input, and the line prints
the three counted values without a category rather than guessing one.

## Availability

`—` means undeterminable. `0` means measured and none. They are different and the
output never substitutes one for the other.

Each section states its own emptiness in the form that suits it, and the rule is
per section rather than global:

| Section | Nothing in scope | Rule |
| --- | --- | --- |
| `🧭 ACTIVITY` / `🧭 WORK KIND` | `No turn in the selected scope.` under the heading | A whole-section message, because with no turn there are no rows to dash |
| `🧱 WORKFLOW` | All five rows print, each with `—` | The rows are a fixed vocabulary, so their absence would hide which metrics exist |
| `🔧 TOOLING` | `No tool call in the selected scope.` under the heading | Same reason as Activity: the rows are data-driven |

A row is never dropped from `🧱 WORKFLOW`: a disappearing row makes the reader
wonder whether the metric exists at all.

| Condition | Text | JSON | Exit |
| --- | --- | --- | --- |
| Data present | Sections as above | `available: true` with values | 0 |
| Nothing in scope | Per the table above | `available: false` | 0 |
| Present but partially attributed | Values render; no marker anywhere | `cost_basis: "partial"` | 0 |

All exit `0`. An empty scope is a true answer, not a failure.

The text form does not mark `partial` on a value line, because there is nowhere
honest to put it: an unattributed event belongs to no row, so marking a row would
claim the shortfall is known per row. A reader who needs the distinction reads
`--format json`, which is the pair a script must be able to separate and can. The
panel makes the same choice and shows nothing either.

## `--format json`

Emitted through the existing usage envelope, using **the same field names and
units as the wire projection** in Decision 9. A figure read in the panel and the
same figure read from this JSON have the same name.

```json
{
  "period": "7d",
  "client": "all",
  "activity": {
    "available": true,
    "cost_basis": "turn",
    "kinds": [
      {
        "kind": "coding",
        "share": 52.0,
        "cost": 2.74,
        "events": 21,
        "sub": [{ "kind": "feature", "share": 24.0, "cost": 1.26, "events": 9 }]
      }
    ]
  },
  "workflow": {
    "available": true,
    "first_edit_seconds": 132,
    "files_touched": 7,
    "retries": 3,
    "edits_per_session": 4.0,
    "top_file": "tasks.md",
    "top_file_edits": 4
  },
  "tooling": {
    "available": true,
    "calls": 82,
    "groups": 4,
    "rows": [{ "kind": "bash", "calls": 32, "share": 39.0 }],
    "top_mcp_server": "codegraph",
    "top_mcp_calls": 5
  }
}
```
The sample shows one entry per list for shape; the real payload carries all four
categories and every non-empty tool kind. `sub` is present only under `--sub` or
`--activity`.

The one structural difference from Decision 9's wire shape is the consequence of
having one scope per invocation: the wire nests each family's values in an
`items[]` array keyed by `period` and `client`, because the panel holds every
filter position at once; the CLI renders one position and therefore lifts those
two keys to the top level and the values into the family object. Field names and
units are identical.

Durations are seconds in JSON and human units in text, matching the rest of the
usage family.

## Reproducibility against the panel

For `today`, `7d`, and `30d` with the same `--client`, every figure here equals
the figure the panel shows. Those three are the periods both surfaces have.

`usage signals` accepts periods the panel has no control for. Those carry no
cross-surface guarantee, because there is no panel figure to compare them to —
not because they are less trustworthy.

## Degradation

Narrow terminals and `--no-color` are handled by the existing usage text
primitives, unchanged. The share bar and the aligned columns degrade exactly as
the same primitives already degrade elsewhere in the usage family. This surface adds no new
degradation behavior and must not introduce any.

## Privacy in the output

The most-touched file prints its base name only. No path, no directory, no
command string, and no message text reaches this output, because Decision 2
keeps none of them in the store. This is the boundary being visible at the
surface, not an additional rule applied here.
