---
status: proposed
created: 2026-08-06
scope: usage-and-session
---

# Terminal Rendering Experience — Usage and Session

This proposed design applies the AgentDeck terminal-experience profile to the
`usage` report family and the `session` inspection surfaces. It is a design
gate, not implementation authority. Existing JSON, persistence, pricing,
privacy, and exit-code contracts remain authoritative until an approved
implementation task changes a human-facing text or interactive contract.

## 1. Goal, scope, and non-goals

### Goal

Give `usage` and `session` one observable terminal language while preserving
their different user jobs:

- `usage`: compare totals, trends, dimensions, cost, cache, and coverage with
  minimum scanning effort;
- `session`: inspect chronology, approved documents, safe activity, and token
  usage without losing the current section, page, or selection.

The shared layer owns terminal capability detection, visible-cell geometry,
sanitization, section hierarchy, bars, aligned rows, viewport calculation,
fallbacks, and lifecycle cleanup. Domain renderers continue to own field
selection, ordering, labels, pagination, and privacy.

### In scope

- `usage stats`, `usage summary`, `usage sessions`, and `usage diagnose` text;
- `session show` text and `session show --interactive`;
- the planned explicit `usage stats --interactive` surface;
- shared command-layer rendering primitives and interaction state contracts.

### Non-goals

- No JSON, DTO, SQLite, pricing, attribution, provider, credential, or source
  ownership change.
- No new aggregate, filter, stored field, or exposed private value.
- No TUI framework or runtime dependency.
- No implicit interactive mode when stdin/stdout happen to be TTYs.
- No desktop implementation, specification-version raise, release, commit, or
  push authority.

## 2. Terminal matrix

| Mode | Input/output | Geometry source | Required behavior |
| --- | --- | --- | --- |
| JSON | any | none | Stable machine output; no ANSI, prompts, cursor control, truncation, or terminal-dependent shape. |
| Redirected text | non-TTY stdout | valid `COLUMNS`, otherwise 100 | Deterministic line mode, no color or cursor control; all meaning remains copyable. |
| TTY text | ordinary command | terminal size, then valid `COLUMNS` | Width-aware line mode; never enters raw mode or alternate screen. |
| Interactive | explicit `--interactive`, TTY stdin and stdout | live columns and rows | Full-screen read-only state machine with bounded paging and complete cleanup. |
| Dumb/no-color | `TERM=dumb`, `NO_COLOR`, or `--no-color` | line-mode width | Plain labels and ASCII-safe decoration; meaning never depends on color, icon, or fill glyph. |

Interactive mode rejects non-TTY stdin/stdout and `TERM=dumb` before raw mode,
with an actionable message pointing to the ordinary text command. It does not
silently change mode.

Geometry bands are behavioral, not device classes:

- wide: at least 120 columns;
- standard: 80–119 columns;
- compact: 48–79 columns;
- narrow line mode: 1–47 columns;
- interactive minimum: 48 columns by 10 rows.

Line mode remains usable below 48 columns and must never render to a synthetic
48-column width. Interactive mode below its minimum fails before terminal
ownership changes.

## 3. Interaction states and transitions

Every section has an explicit content state: `loaded`, `empty`, `unavailable`,
`warning`, `partial`, `stale`, or `error`. Empty and unavailable are labeled;
they are not represented by an unexplained blank region.

Interactive state is:

```text
mode
  section
    logical page
    selected row
    viewport offset
    content state
```

Page, selection, and viewport are independent per section. Switching sections
restores that section's last state. Resize keeps the logical page and selected
row, then adjusts only the viewport needed to keep selection visible.

- Section change loads only the target section's current bounded page.
- Page change resets that section's selection and viewport to the first row.
- Row navigation never changes logical page.
- An empty page selects no row and renders an explicit empty state.
- Stale, partial, and warning states remain visible until the page is replaced
  or the viewer exits.
- A load error exits through the normal cleanup path and reports the error in
  line mode rather than leaving a half-owned terminal.

No interactive state holds the AgentDeck state lock across user think time.

## 4. Layout, geometry, viewport, and degradation

### Shared semantic primitives

The command layer exposes a small domain-neutral set:

- terminal render context: columns, rows, TTY, color, Unicode, and mode;
- visible-cell fitting and control/ANSI sanitization;
- section title and durable status/warning lines;
- share bar and magnitude bar;
- aligned labeled row with optional continuation lines;
- responsive table/stack decision;
- viewport calculation independent from logical paging.

It does not expose a generic widget framework. Usage and session adapters map
their domain values into these primitives.

### Hierarchy

Both domains use the same order:

1. command/surface title;
2. primary identity or KPI summary;
3. section title;
4. primary rows;
5. subordinate continuation/detail rows;
6. durable warning, pagination, and next-action affordance.

Color and emoji may decorate this hierarchy but never create it.

### Width degradation

- Wide: side-by-side sections are allowed only when each side meets its minimum
  width and rendered heights are reasonably balanced.
- Standard: one full-width aligned table or bar list per section.
- Compact: retain identity, primary value, and status; move secondary fields to
  labeled continuation lines.
- Narrow: stack one labeled value per line, remove decorative rules and bar
  tracks before truncating semantic values, and visibly fit every line.

Truncation is visible-cell aware for CJK, emoji, combining marks, and ANSI.
Untrusted control characters and embedded escape sequences are removed or
rendered visibly before width calculation, preventing terminal injection.

### Usage rules

- Share bars always use a fixed 100% baseline across Models, Clients,
  Providers, and cache-hit-rate rows.
- Magnitude bars are used only for series such as Trend and name the series
  peak that defines full scale.
- Share appears once. Tokens, cost, pricing status, sessions, and cache values
  align by column when geometry permits and move to labeled continuations when
  it does not.
- Content volume participates in side-by-side decisions; width alone never
  creates a mostly empty column beside a wrapping column.

### Session rules

- Metadata remains first and is never displaced by an empty paged section.
- Documents, activity, and invocations use the same wide/compact row grammar as
  usage while preserving their current privacy allowlists.
- Interactive sections remain Overview, Documents, Activity, and Tokens, each
  with independent page, selection, and viewport state.

### Interactive row budget

The full-screen frame reserves rows for title, section tabs, context/status,
warning/status footer, and help. The content viewport is calculated only after
those fixed rows are budgeted. At short heights, decoration is removed first,
then help collapses to `? help · q quit`; content still receives at least one
row. Below 48x10, the viewer rejects entry before raw mode.

## 5. Input lifecycle and keymap

The usage and session viewers share one grammar:

| Key | Behavior |
| --- | --- |
| Left / Shift-Tab | Previous section |
| Right / Tab | Next section |
| Up / Down | Move selection in current page |
| Home / End | First / last row in current page |
| Page Up / Page Down | Previous / next bounded logical page |
| `?` | Toggle compact help without changing page or selection |
| `q` / standalone Escape | Exit through normal cleanup |

Standalone Escape remains bounded by the existing ambiguity window for longer
terminal sequences and can never wait indefinitely. A resize observed during
that window must not discard an already recognized Escape.

The first implementation preserves current Ctrl-C, EOF, and command exit-code
semantics; it adds explicit regression tests rather than silently redefining
them during a rendering refactor.

Interactive setup and cleanup order is contractual:

1. validate TTY, `TERM`, and minimum geometry;
2. load the initial bounded page;
3. enter alternate screen, enable raw mode, hide cursor, and register signals;
4. on every exit, stop or detach input and signal readers;
5. restore raw mode, cursor, signal handlers, and previous screen contents.

Cleanup must succeed for normal exit, Escape, Ctrl-C, EOF, cancellation,
SIGWINCH races, load errors, render errors, and closed input.

## 6. Fallback and accessibility

- Ordinary text is the screen-reader and copy/paste fallback for every
  interactive surface.
- Labels accompany colors, bars, icons, stale/partial states, and warnings.
- `NO_COLOR`, `--no-color`, and `TERM=dumb` preserve hierarchy with text and
  ASCII-safe separators.
- Motion and timing never carry meaning; no mandatory mouse behavior exists.
- JSON remains complete even when text caps or geometry omit secondary rows.
- Warnings are durable lines or footer state, never transient animation.
- Session source text and usage labels are sanitized before terminal control
  processing while retaining approved visible content.

## 7. Verification design

| Risk | Observable oracle |
| --- | --- |
| Primitive extraction changes output | Byte-identical fixtures at existing widths before intentional visual changes. |
| Width overflow or Unicode split | Visible-cell tests at 24, 40, 48, 60, 80, 100, 120, 140, and 160 columns with CJK, emoji, combining marks, ANSI, and controls. |
| Usage semantics drift | Share/magnitude baseline fixtures, aligned-column assertions, and unchanged underlying numeric values. |
| Session privacy drift | Existing approved-document/activity/invocation allowlist tests plus control-sequence sanitization tests. |
| Paging/state bleed | Pure state tests proving section-local page, selection, and viewport restoration. |
| Resize corruption | PTY resize tests across wide, compact, and short geometry with selection kept visible and no stale cells. |
| Input/cleanup regression | PTY tests for key sequences, standalone Escape, resize/Escape interleaving, Ctrl-C, EOF, cancellation, render/load error, raw-mode restoration, cursor restoration, and alternate-screen restoration. |
| Fallback regression | Non-TTY and `TERM=dumb` rejection for interactive mode; plain redirected text; byte-identical JSON. |
| Runtime integration | Compile the current binary and run usage and session acceptance against an isolated HOME with synthetic local data. |

Pure tests and PTY behavior are required evidence; screenshots or recordings
may compare frames but cannot replace lifecycle and cleanup assertions.

## 8. Decisions, assumptions, and unresolved questions

### Alternatives considered

1. Keep usage-only primitives and give session separate helpers. This minimizes
   immediate edits but preserves divergent width, sanitization, and hierarchy
   rules. Rejected.
2. Introduce domain-neutral command-layer primitives with thin usage/session
   adapters. This centralizes terminal behavior without moving domain policy or
   adding a framework. **Selected.**
3. Adopt a third-party TUI/widget framework. It would broaden dependencies and
   state ownership beyond the problem. Rejected.

For interactive presentation, persistent alternate-screen frames are selected
over cursor-up rewriting in the main screen because explicit interactive mode
must restore previous screen contents. Ordinary commands remain line-oriented.

### Current conflicts to resolve during implementation

- Usage clamps layout to 48–160 columns while session text uses the reported
  width directly; widths below 48 therefore do not share a degradation rule.
- Usage and session have separate color, fitting, section, and responsive-table
  logic.
- Session interactive state keeps page per section but selection and viewport
  are not section-local.
- Session sanitizes newlines but does not yet define control/ANSI sanitization
  as a shared pre-layout step.
- The current viewer clears the active screen instead of using an alternate
  screen whose previous contents can be restored.

### Implementation sequencing after approval

1. Reconcile any in-progress `usage-render-primitives` work with this contract;
   preserve byte-identical existing fixtures.
2. Extract the shared terminal render context and safe geometry primitives,
   then adapt usage and session without intentional visual changes.
3. Implement approved usage bar/detail/layout changes through the shared layer.
4. Align session line and interactive rendering, including section-local state,
   sanitization, geometry degradation, and alternate-screen cleanup.
5. Add the planned usage interactive viewer by reusing the shared state and
   lifecycle contracts.

No unresolved product decision blocks design review. Implementation remains
blocked until this proposed contract is explicitly approved and the active
usage plan is reconciled with it.
