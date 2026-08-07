---
status: active
created: 2026-08-06
---

# Usage Report Presentation

Target release: `v0.4.0`.

This plan makes the `usage` report family readable as one surface. It covers
`usage stats`, `usage summary`, `usage sessions`, and `usage diagnose`, unifies
their visual language, and adds an explicit interactive mode consistent with the
session viewer.

The retired [readability plan](../archive/plans/usage-stats-readability.md)
already bounded *how much* these reports print, through per-section caps,
`--top`, a 48-bucket trend window, and the two-column layout. It reduced
`usage stats --period 30d --group-by hour` from 832 lines to 142. This plan
addresses the remaining problem: a bounded report is still hard to read, because
the values it prints cannot reliably be compared to each other.

## Source Sufficiency

Design input is `SUFFICIENT`.

- The living CLI contract already states that default `text` output is a
  human-facing interactive contract rather than a serialized view of internal
  DTOs, and already fixes the `NO_COLOR`, `--no-color`, `TERM=dumb`,
  redirected-output, timezone, and pricing-coverage behavior this plan must
  preserve.
- Every defect below was read from current rendering code and from a real
  `usage stats --period today` run on 2026-08-06, not inferred.
- The report data itself is sufficient. No decision here requires a new stored
  field, a new aggregate, or a change to pricing or attribution.
- The session experience plan defines the terminal state machine this plan's
  interactive mode reuses, so no TUI dependency decision remains open.

No user decision remains that changes this design's architecture or scope.

## Measured Baseline

The family currently runs **two unrelated rendering systems**.

| Command | Renderer | Width aware | Color | Bars |
| --- | --- | --- | --- | --- |
| `usage stats` | `statsTextRenderer` (`cmd/agentdeck/usage_stats_text.go`) | yes, clamped 48-160 | yes | yes |
| `usage summary` | three `output.WriteASCIITable` tables | no | no | no |
| `usage sessions` | one 14-column `output.WriteASCIITable` | no | no | no |
| `usage diagnose` | one `METRIC`/`VALUE` table | no | no | no |

`output.WriteASCIITable` sizes every column from its widest cell and never
consults terminal width, so `usage sessions` — `CLIENT`, `SESSION`, `FIRST`,
`LAST`, seven token columns, two cost columns, `STATUS` — overflows and wraps
into unreadable fragments on an ordinary terminal. `usage stats` is the only
command in the family that already adapts.

Defects recorded from `usage stats --period today` on 2026-08-06, 122 events,
100% priced:

1. **Bar baselines are inconsistent within one screen.** `MODELS` scales each
   bar against the largest share in the section; `CLIENTS` and `PROVIDERS` scale
   against 100. A 51.9% model bar and a 46.5% model bar therefore render nearly
   identical, a 1.7% model collapses to a sliver, and a 53.5% provider bar fills
   only half its track. Bar length carries no consistent meaning.
2. **The detail line repeats and cannot be scanned.** It restates the share
   already printed beside the bar, then packs seven `·`-separated fields with no
   column alignment, dimmed, so it reads only left to right.
3. **Sibling sections have unequal depth.** `CLIENTS` prints a bar and a share
   only, while the structurally identical `MODELS` and `PROVIDERS` rows carry
   token, cost, pricing-status, and session detail.
4. **The two-column split ignores content volume.** It depends only on terminal
   width, so a single-bucket `--period today --group-by hour` run leaves the
   `TREND` column nearly empty while the right column wraps repeatedly.
5. **`CACHE HIT RATE` is an unstructured text block.** Per-model and per-session
   rows sit at the same level, session rows carry a full session ID plus a
   dedicated copyable command, and the block consumes a large share of visible
   height in a visual language shared with nothing above it.
6. **The footer restates KPI-class values in a second form.** `AVG COST`,
   `PEAK`, and `PRICED` are separated from the header cards that carry the same
   class of value, and `AVG COST` never names what it is averaged over.

## Goals

- Give every bar in the family one defined, documented meaning, so two bars on
  one screen can be compared without reading their numbers.
- Make row detail scannable down a column instead of readable only across a line.
- Render the whole family through one width-aware primitive set, so
  `usage sessions` and `usage summary` stop overflowing narrow terminals.
- Give sibling sections equal information depth.
- Let section layout follow content volume, not terminal width alone.
- Add an explicit keyboard-driven interactive mode for `usage stats` that pages
  each section independently.
- Change no JSON shape, no stored value, and no pricing or attribution rule.

## Non-Goals

- No change to any JSON field, name, type, ordering, or presence. Every change
  in this plan is a `text` change.
- No change to stored usage events, pricing catalogs, provider attribution,
  cost arithmetic, or coverage classification. Presentation may relabel a value;
  it may not recompute one.
- No new aggregate, dimension, filter, or stored field.
- No change to which values are safe to display. The session-ID and command
  affordances in `CACHE HIT RATE` are relocated, not newly exposed or newly
  hidden.
- No new TUI framework dependency; the interactive mode reuses the session
  viewer's state machine.
- No change to `usage scan` and `usage rebuild`, which are operation commands
  rather than reports. Their result tables stay as they are.
- No TTY-default behavior change: reports remain non-interactive unless
  `--interactive` is explicitly supplied.
- No specification version raise. See task 6.

## Decisions

### 1. Two bar kinds, each with one fixed baseline

The family defines exactly two bar kinds, and every bar declares which it is.

**Share bars** measure a percentage of a whole: `MODELS`, `CLIENTS`,
`PROVIDERS`, and any future share dimension. They scale against a fixed 100%
track. A 51.9% bar always fills 51.9% of its track, in every section and every
run, so bar length is comparable across sections and across invocations.

**Magnitude bars** measure an absolute quantity in tokens or cost: `TREND`.
They scale against the largest value in their own series, because a trend's
purpose is relative shape over time, and its section header names the peak that
defines full scale.

Scaling `MODELS` against its own maximum is the single largest readability
defect in the current output, and it is exactly the rule that share bars drop.

Share-bar rules:

- a value of zero renders an empty track;
- a non-zero value that scales below one cell renders exactly one cell, so a
  small-but-present share is visibly distinct from absence;
- the `unavailable` cost state keeps its current label and renders an empty
  track rather than a misleading zero-length fill;
- the track is a fixed width within one section so fills align vertically.

### 2. Row detail becomes aligned columns

The dimmed `·`-separated detail line is replaced by fixed columns that align
vertically across the rows of a section.

- The share is printed once, beside the bar, never repeated in detail.
- Columns carry tokens, cost, pricing status, and sessions, each right-aligned
  in its own field so magnitudes line up digit by digit.
- Secondary values that do not fit the current width — tool calls, cache hit
  rate, wrapper-carried events — move to an explicit continuation line rather
  than being packed into the primary line or silently dropped.
- Detail stays visually subordinate to its row, but stops relying on dimming
  alone to establish hierarchy, because dim text is the least reliable attribute
  across terminal themes.

### 3. Sibling sections carry equal depth

`CLIENTS` gains the same detail columns as `MODELS` and `PROVIDERS`. A section
that structurally resembles another must expose the same class of information,
or the resemblance misleads.

Where a dimension genuinely cannot supply a column, the cell renders an explicit
unavailable marker instead of being omitted, so the column stays readable.

### 4. Layout follows content volume

The two-column split becomes content aware. It compares the rendered height of
the two columns and falls back to stacked full-width sections when the split
would leave one column mostly empty — the single-bucket `--period today` case —
or when either column would wrap repeatedly at the available width.

Width remains a hard constraint; content volume becomes an additional one. The
existing 48/100/160 clamp, `COLUMNS` handling, and terminal detection are
unchanged.

### 5. `CACHE HIT RATE` becomes a structured section

The block adopts the family's section language:

- per-model cache rows become ordinary rows with a share bar for hit rate and
  aligned read/write detail columns, consistent with every other section;
- per-session rows become a subordinate, separately capped list under an
  explicit sub-heading, so they cannot dominate the section;
- the copyable per-session command moves out of the row body into the section's
  footer affordance, printed once, rather than once per session;
- session identifiers are shown at a bounded display length while the footer
  command carries the full identifier needed to act on it.

The existing `statsCacheSessionsCap` and `--top` override semantics are
preserved.

### 6. KPI values are stated once, in one place

The `AVG COST`, `PEAK`, and `PRICED` footer merges into the header KPI region,
because those values are the same class as `TOKENS`, `COST`, and `SESSIONS` and
splitting them across the screen forces the reader to hold context.

Every KPI names its own basis. `AVG COST` becomes explicit about what it
averages over, which the current label never states. The underlying computation
is unchanged; only the label and placement change.

### 7. One primitive set for the whole family

The width, color, section-title, bar-track, aligned-column, and responsive-table
primitives currently private to `statsTextRenderer` become a shared command-layer
primitive set. `usage summary`, `usage sessions`, and `usage diagnose` render
through it instead of calling `output.WriteASCIITable` directly.

`usage sessions` becomes responsive. Its 14 columns are ranked: identity, time
bounds, dominant token components, cost, and status stay in the primary row at
narrow widths, and the remaining token components move to a continuation line.
Wide terminals may restore the full column set. JSON is untouched, so scripted
consumers are unaffected.

`output.WriteASCIITable` itself is not modified or removed. It remains correct
for the operation commands and for any caller that wants content-sized columns.

### 8. Interactive mode is explicit and read-only

The command is:

```text
agentdeck usage stats --interactive
```

`--interactive` is valid only for text output with TTY stdin and stdout. It is
mutually exclusive with `--format json` and `--top`; the viewer always makes
Trend, Models, Clients, Providers, Cache, and Coverage available and pages each
independently.

It reuses the key decoding, state machine, viewport calculation, and terminal
lifecycle delivered by the session experience plan's `interactive-session-viewer`
task, with the same controls, the same footer contract, the same resize
behavior, and the same terminal restoration guarantees on normal exit, error,
cancellation, and interrupt. It adds no TUI dependency.

The viewer is read-only. It does not rescan, mutate state, change provider, or
expose any value the non-interactive report does not already print.

## Failure and Safety Rules

- A presentation change may never alter a number. Where this plan relabels or
  relocates a value, the value itself is byte-identical to what the current
  renderer computes.
- Every JSON envelope stays byte-deterministic. No task in this plan writes to a
  JSON code path.
- `NO_COLOR`, `--no-color`, `TERM=dumb`, and redirected output remain fully
  readable without color, and no section may depend on color or dimming to be
  understood.
- Degenerate inputs render an explicit state rather than an empty screen: zero
  events, one bucket, one model, a single 100% share, all-unpriced data, and
  `unavailable` cost all have defined output.
- Terminal setup failure in the viewer returns before entering raw mode; cleanup
  is idempotent once raw mode starts.
- No new log, warning, error, snapshot, or test fixture contains real source
  paths, credentials, prompts, replies, tool arguments, or environment values.

## Verification Strategy

- Golden-width fixtures at 48, 60, 100, 140, and 160 for every command in the
  family, with and without color.
- Bar-scaling fixtures: share bars at 0%, a sub-cell non-zero share, 50%, 100%,
  an all-equal-share set, a single-dimension 100% set, and the `unavailable`
  cost state; magnitude bars for one bucket, flat series, and single-spike
  series.
- Column-alignment fixtures asserting that detail columns align vertically
  across rows and that continuation lines appear exactly at the documented
  thresholds.
- Layout fixtures: single-bucket stacking, balanced two-column, and the wrap
  threshold that forces stacking.
- JSON invariance: every changed command asserts byte-identical JSON before and
  after, which is the primary regression gate for this plan.
- Family consistency fixtures asserting `usage sessions` no longer exceeds the
  target width and that its column ranking is stable.
- Interactive state-machine fixtures: every key, independent section pages,
  resize, cancellation, terminal restoration, and rejection of non-TTY and JSON
  modes.
- Final L2 evidence: targeted `cmd/agentdeck` tests plus vendored
  `go test ./...` on the final relevant content state. Task 5 adds a race gate
  if it introduces terminal concurrency or signal handling.

Verification is L2 for tasks 1-4 and 6 because the text output of these commands
is a documented human-facing CLI contract with no persistence, pricing, or
credential exposure. No task in this plan touches a schema, a stored value, or a
credential path, so no task routes to L3 on data risk alone.

## Tasks

### 1. `usage-render-primitives`

- Extract the width, color, section-title, bar-track, aligned-column, and
  responsive-table primitives out of `statsTextRenderer` into a shared
  command-layer primitive set.
- This task is a pure refactor: `usage stats` output must remain byte-identical
  at every fixture width, which is its acceptance gate.
- Verification level: L2.

### 2. `usage-bar-and-detail-semantics`

- Implement decision 1 and decision 2: share bars on a fixed 100% track with the
  documented zero and sub-cell rules, magnitude bars keyed to a named peak, and
  aligned detail columns with the share printed once.
- Implement decision 3: `CLIENTS` gains equal detail depth.
- Verification level: L2. Depends on task 1.

### 3. `usage-stats-layout`

- Implement decision 4, decision 5, and decision 6: content-aware column
  splitting, the structured `CACHE HIT RATE` section with its subordinate capped
  session list and single footer affordance, and KPI consolidation with an
  explicit `AVG COST` basis.
- Verification level: L2. Depends on task 2.

### 4. `usage-family-alignment`

- Implement decision 7: migrate `usage summary`, `usage sessions`, and
  `usage diagnose` onto the shared primitives and make `usage sessions`
  responsive with a documented column ranking.
- Assert JSON invariance for all three commands.
- Verification level: L2. Depends on task 1; independent of tasks 2 and 3.

### 5. `usage-interactive-viewer`

- Implement decision 8 by reusing the session viewer's terminal state machine.
- Verification level: L3 if terminal concurrency or signal handling requires a
  race gate; otherwise L2 plus targeted PTY acceptance.
- **Blocked on the session experience plan's `interactive-session-viewer`
  reaching Review PASS.**

### 6. `usage-presentation-contract`

- Reconcile **this plan's** delivered behavior into `docs/specs/cli-design.md`
  and `docs/specs/cli-manual.md`, close all review records, and update the
  documentation index.
- **This task does not raise the specification version.** That is a release-level
  action owned by the [v0.4.0 release plan](v0-4-0-release.md), which runs after
  both `v0.4.0` feature lines land their contract text.
- Runs only after every other task in this plan has Review PASS.
- Verification level: L2.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `usage-render-primitives` | [x] | [x] |
| 2. `usage-bar-and-detail-semantics` | [ ] | [ ] |
| 3. `usage-stats-layout` | [ ] | [ ] |
| 4. `usage-family-alignment` | [ ] | [ ] |
| 5. `usage-interactive-viewer` | [ ] | [ ] |
| 6. `usage-presentation-contract` | [ ] | [ ] |

Tasks 1 and 4 form the family-consistency path; tasks 2 and 3 form the
`usage stats` semantics path. Both start from task 1. Task 5 is blocked on the
session experience plan. Task 6 runs last within this plan, and in turn gates
the [v0.4.0 release plan](v0-4-0-release.md).

Commit boundaries follow task boundaries. This plan does not authorize commits,
pushes, release tags, RC publication, real-state mutation, or desktop work.

## Backlog / Future Feature Ideas

- Sparkline or braille density rendering for `TREND` once the bar contract is
  stable.
- A saved user preference for column ranking in `usage sessions`.
- Applying the shared primitive set to non-`usage` command families, once it has
  survived one release in this one.

## Starting Task

Turn a Status row into scoped development by naming its anchor:

```text
进入开发：`usage-report-presentation` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's Decisions and named task, the usage text output
contract in `docs/specs/cli-design.md`, the Usage manual section, every file the
task names, and verification routing. Tick `Dev` only after selected
verification passes. An independent reviewer records a PASS round under
`docs/reviews/usage-report-presentation/<task-anchor>.md` before ticking
`Review`.
