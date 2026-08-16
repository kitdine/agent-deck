---
status: historical
created: 2026-08-07
retired: 2026-08-16
scope: usage-report-presentation/usage-interactive-viewer
source_sufficiency: sufficient
---

# Usage Interactive Viewer

This document is the delivered Usage interactive contract originally approved
for `usage-report-presentation` Task 5, `usage-interactive-viewer`. It narrows
the broader
[terminal rendering contract](terminal-rendering-design.md) to
`usage` only. The shared raw-frame behavior and Session alignment described
here were delivered in response to the v0.4.0-rc.1 manual-acceptance failure.

## 1. Goal, scope, and non-goals

### Goal

Add an explicit, read-only `agentdeck usage stats --interactive` mode that lets
the user compare usage dimensions, move selection, and inspect the selected
record without losing section, page, or viewport context.

### In scope

- Seven sections: Overview, Trend, Models, Clients, Providers, Cache, Coverage.
- Selection-driven detail: changing selection must change the visible detail
  title, fields, status, and explanatory text, not only the row highlight.
- Section-local logical page, selection, viewport, and content state.
- Wide, standard, compact, and minimum-geometry behavior.
- A restrained ANSI palette plus `NO_COLOR` and `--no-color` degradation.
- Explicit TTY validation, cancelable input, bounded Escape decoding, resize,
  raw-mode ownership, cursor and previous-screen restoration.
- Pure state tests, PTY lifecycle tests, and compiled-binary isolated-HOME
  acceptance with synthetic local usage data.

### Preserved boundaries

- The broader terminal-rendering contract owns the approved non-interactive
  `usage stats` remediation. This viewer consumes the same primitives but does
  not redefine its responsive panel layout.
- Existing filters, range resolution, scan behavior, `--top`, warnings,
  pricing calculations, and report values remain the source of viewer data.
- JSON remains byte-shape compatible and never enters the interactive renderer.
- Session detail data, sections, paging, and privacy remain unchanged. Shared
  raw-frame lifecycle and the new root session browser are defined only by the
  broader terminal-rendering contract.

### Non-goals

- No `usage summary`, `usage sessions`, or `usage diagnose` interactive mode.
- No session rendering redesign or contract synchronization.
- No new aggregate, filter, stored field, SQLite migration, pricing rule, DTO,
  credential path, provider behavior, or desktop contract.
- No implicit interactive mode merely because stdin and stdout are TTYs.
- No third-party TUI framework or new runtime dependency.

## 2. Terminal matrix

| Mode | Entry conditions | Required behavior |
| --- | --- | --- |
| JSON | Any `--format json` invocation | Existing machine-readable path; no ANSI, cursor control, prompts, geometry-dependent truncation, or interactive state. |
| Ordinary text | No `--interactive` | Existing Tasks 2–3 renderer; redirected output remains copyable and deterministic. |
| Interactive color | `--interactive`, text format, TTY stdin/stdout, usable `TERM`, at least 48x10 | Full-screen read-only viewer with bounded paging and complete terminal restoration. |
| Interactive no-color | Interactive conditions plus `NO_COLOR` or `--no-color` | Same state, labels, ordering, markers, bars, warnings, and keymap with ANSI color removed. |
| Unsupported interactive | Non-TTY stdin/stdout, non-text format, `TERM=dumb`, or below 48x10 | Reject before raw mode, cursor hiding, alternate screen, signal registration, or data mutation; point to ordinary `usage stats`. |

Geometry bands are based on visible terminal cells:

- wide: at least 120 columns;
- standard: 80–119 columns;
- compact: 48–79 columns;
- minimum interactive height: 10 rows.

Rows reserved for title, section tabs, range context, status/warning, and help
are subtracted before calculating the content viewport. A valid viewer always
has at least one content row. CJK, emoji, combining marks, ANSI sequences, and
sanitized controls are measured by visible width, not byte or rune count.

## 3. Interaction states and transitions

The usage viewer owns terminal-independent state:

```text
active section
  logical page
  selected row or none
  viewport offset
  loaded | empty | warning | partial | stale | error
```

- Each section retains its own page, selection, and viewport while the viewer
  is open.
- Section change loads or renders only that section's current bounded page.
- Page Up or Page Down changes the current section's logical page and resets
  selection and viewport to its first row.
- Up or Down changes only selection and the corresponding detail content.
- Home or End selects the first or last row on the current page.
- Resize preserves section, logical page, and selected record identity; it
  adjusts only the viewport required to keep selection visible.
- Empty sections show an explicit empty label, select no row, and render no
  fabricated detail.
- Partial, stale, and warning states remain durable in the footer until the
  page is replaced or the viewer exits.
- A load or render error exits through the same cleanup path as normal exit,
  then reports the error after terminal restoration.

Interactive rows are paged in groups of 20. Existing `--top` semantics apply
before pagination for the sections it already controls: omitted keeps the
current default cap, positive N is the ceiling, and `--top 0` is uncapped.
Trend and Clients retain their existing independence from shared `--top` caps.

## 4. Layout, color, viewport, and degradation

### Information hierarchy

Every frame renders in this order:

1. `AGENTDECK · USAGE` and `INTERACTIVE · READ ONLY`;
2. section tabs;
3. selected range, metric, filters, local-zone label, and scan/partial state;
4. four KPI values: Tokens, Cost, Sessions, Priced;
5. active section rows;
6. selected-record detail;
7. page, row, durable warnings, and key help.

### Section behavior

- Overview: high-level token, cost, and session signals with selected-signal
  context.
- Trend: magnitude bars whose named peak is the fixed full-scale baseline.
- Models, Clients, and Providers: fixed 100% share bars and selected-dimension
  tokens, cost, sessions, cache, and pricing completeness where available.
- Cache: model/session cache accounting using existing approved report fields;
  no prompt or source content.
- Coverage: exact, fallback/partial, and unpriced totals with explicit labels.

Share is printed once. At wide and standard widths, comparable numeric fields
align. At compact widths, secondary fields move to labeled continuation lines
or the selected detail block.

### Responsive layout

- Wide: row list and selected detail appear side by side. A section may use a
  balanced secondary region only when both regions remain readable.
- Standard: row list remains primary; selected detail stays beside it only if
  minimum field widths fit, otherwise it stacks below.
- Compact: rows keep identity and primary value; secondary values move below;
  selected detail stacks under the list; help collapses to `? help · q quit`.
- Short height: remove decorative blank rows and rules first, then compact help;
  never hide warning/status or the selected row.

### Terminal palette

Selected Detail cards consume structured fields supplied by the Usage adapter:
label, value, semantic role, priority, and an optional spanning note. The
renderer does not parse concatenated display strings or infer roles from
keywords. Labels use the information role; values use their explicit token,
cost, session, success, warning, error, or neutral role.

Optional zero components, unavailable optional fields, and values already
carried by the selected row are omitted. Required pricing, partial, warning,
stale, or error states remain explicit even when another numeric value is zero.
After normalization, when no non-redundant field or required note remains, the
renderer emits no title-only card and its rows return to the list viewport.

The card uses two field columns only when both retain useful minimum
visible-cell widths; otherwise it stacks one aligned label/value field per row.
Long notes span the card and wrap. Usage's compact row/value/bar layout remains
the visual baseline and does not stretch merely to fill a wide terminal.

Use the existing standard ANSI style mechanism rather than introducing a color
library:

- title, active section, selection marker: cyan;
- token values and token-oriented trend: cyan;
- cost values: yellow;
- session counts: magenta;
- cache and exact coverage: green;
- partial, fallback, stale, and unpriced warnings: yellow plus explicit text;
- inactive labels, borders, and help: default/muted terminal color.

Bars use the section's accent, while their label and numeric value remain
visible text. Selection uses both `>` and color. Partial or unpriced state uses
both a warning label and color. With color disabled, ANSI bytes disappear but
the hierarchy, marker, bars, labels, ordering, and values remain intact.

## 5. Input lifecycle and keymap

| Key | Behavior |
| --- | --- |
| Left / Shift-Tab | Previous section |
| Right / Tab | Next section |
| Up / Down | Move selection on the current page and update detail |
| Home / End | Select first / last row on current page |
| Page Up / Page Down | Previous / next bounded logical page |
| `?` | Toggle expanded help without changing section, page, or selection |
| `q` / standalone Escape | Exit through normal cleanup |
| Ctrl-C / EOF / context cancellation | Exit or return the existing command error only after cleanup |

Usage and Session now reuse one terminal-neutral lifecycle: cancelable polling,
the 35ms bounded Escape ambiguity window, key decoding, resize notification,
raw-mode ownership, alternate-screen ownership, and cleanup. Each raw-mode
logical row is emitted with explicit CRLF and redraw clears stale cells. Usage
continues to own its state and renderer; Session retains its domain adapter.

Setup and cleanup order is contractual:

1. validate format, TTYs, `TERM`, and minimum geometry;
2. acquire the initial bounded usage page without holding an exclusive state
   lock across user think time;
3. enable raw mode, enter the alternate screen, hide cursor, and register
   resize handling;
4. on every exit path, stop or detach input and signal readers;
5. restore raw mode, cursor, signal handlers, and the previous screen;
6. only then print a terminal-independent error, when one exists.

## 6. Fallback and accessibility

- Ordinary `usage stats` is the copyable and screen-reader fallback.
- JSON is the complete automation and desktop-consumption path.
- Color, glyph fill, cursor position, and timing never carry unique meaning.
- `NO_COLOR` and `--no-color` remove ANSI style only; they do not change data,
  page size, selected row, status, warnings, or keys.
- Sanitization removes embedded ANSI/control sequences before fitting while
  preserving visible labels needed to identify rows.
- No mouse, animation, rapid timing, or destructive confirmation is required.
- Redirected text and JSON never emit alternate-screen, cursor, raw-mode, or
  key-help sequences.

## 7. Verification design

Runtime acceptance covers 60x18, 80x24, 120x32, and 180x40 using the compiled
current candidate, including color/no-color equivalence, live resize, selected
identity preservation, and complete terminal cleanup. Synthetic isolated state
is followed by a read-only isolated copy of realistic local data whose source
hashes must remain unchanged. This acceptance supplements, and does not replace,
the pure state, visible-cell, PTY lifecycle, privacy, and JSON assertions below.

| Risk | Required observable evidence |
| --- | --- |
| State bleed | Pure tests prove section-local page, selection, viewport, Home/End, empty pages, and selection-driven detail. |
| Width or height corruption | Visible-cell cases at 48x10, 60x18, 80x24, 100x24, and 140x32, including CJK, emoji, combining marks, ANSI, and controls. |
| Color-only meaning | Strip ANSI and compare labels, markers, values, ordering, warnings, page, and selection identity; cover `NO_COLOR` and `--no-color`. |
| Bar semantic drift | Fixed-100% share and named-peak magnitude fixtures reuse the same numeric report values as ordinary text. |
| Ordinary output regression | Existing Tasks 1–3 fixtures and width assertions remain authoritative; interactive work does not rewrite expected line output. |
| JSON regression | Same report state yields existing JSON shape and values with no cursor or ANSI bytes. `--interactive --format json` is rejected. |
| Terminal ownership | PTY tests cover startup, standalone Escape, arrows, paging, resize, Ctrl-C, EOF, cancellation, load error, render error, raw mode, cursor, alternate screen, and input-reader exit. |
| Session regression | Existing session state, key-decoder, token-summary, privacy, and PTY lifecycle tests remain green if shared mechanics move; no intentional snapshot update. |
| Runtime integration | Build the current binary and exercise `usage stats --interactive` against an isolated HOME containing synthetic local usage data; also run ordinary text and JSON against the same state. |
| Release-preflight entry gate | Exercise the compiled current candidate against synthetic isolated state and read-only isolated copies of realistic local data. Record layout, color/no-color, paging, live resize, cleanup, and source-hash invariance; bind that evidence to the exact candidate SHA before same-SHA `release-preflight`. |

Screenshots or recordings may compare frames, but cannot replace lifecycle,
state, cleanup, privacy, or machine-output assertions.

## 8. Decisions, assumptions, and implementation boundary

### Alternatives considered

1. Reuse the hard-coded `sessionViewerState` and renderer. Rejected: usage has
   seven sections and structured selected details rather than four line pages.
2. Reuse or extract only terminal-neutral mechanics and add a usage-owned state
   and renderer. Existing session behavior remains behind an adapter.
   **Selected.**
3. Copy the complete session terminal loop into usage. Rejected: Escape,
   resize, cancellation, and cleanup behavior would immediately diverge.
4. Add a third-party TUI framework. Rejected: it broadens dependencies and
   state ownership beyond Task 5.

### Delivered implementation boundary

- `cmd/agentdeck/main.go` validates the stats-only `--interactive` route
  without changing ordinary report or JSON routing.
- Command-layer Usage viewer files own Usage state, report adaptation,
  rendering, and tests.
- Terminal-neutral input and lifecycle mechanics are shared with Session while
  each domain retains its own state and data adapter.
- Existing Usage render primitives supply visible width, ANSI styling, bars,
  fitting, and no-color decisions.

### Resolved implementation state

- `usage stats --interactive` now has explicit terminal-height, viewport,
  selection, and routing behavior.
- Usage and Session share cancelable key decoding, resize notification,
  raw-mode ownership, alternate-screen ownership, and cleanup while retaining
  domain-specific state and rendering.
- Both interactive surfaces restore the previous screen on every exit path.

### Assumptions and remaining release evidence

- The approved prototype remains presentation evidence, not runtime or cleanup
  evidence.
- Runtime and cleanup claims require current compiled-binary synthetic and
  isolated-real-state acceptance plus exact-state CEv1 and aggregate evidence.
- No unresolved product choice blocks the delivered interaction contract.
