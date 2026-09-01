---
status: historical
created: 2026-08-15
updated: 2026-08-18
retired: 2026-09-01
---

# Menu-Bar Experience

Surface: the AgentDeck macOS menu-bar host.
Task: `menubar-experience`.
Consumes the foundation runtime and the menu-bar wire contract extension in
[`../architecture.md`](../architecture.md).

## Purpose

Define the menu-bar presentation, quick actions, state model, localization, and
accessibility contract for the AgentDeck macOS host.

This revision is derived from the reviewable prototype under
[`prototype/interactive-v7/`](prototype/interactive-v7/) rather than the other way
round. Where the prototype and an earlier draft of this document disagreed, the
prototype is what was reviewed, so the document was changed.

This document is normative. Product behavior must follow this contract even
when an earlier implementation differs from it.

The Go-side contracts this surface depends on — the additive
`provider.candidates` section, the switch command surface, its result envelope,
and switch operation ownership — are specified in
[`../architecture.md`](../architecture.md#menu-bar-wire-contract-extension).

## Scope

This task delivers, on the presentation side:

- the reading surface: client and period filters, four filtered panels, the
  unfiltered rhythm block, and the provider footer. The Sessions panel ships its
  statistics, per-project rows, and recent-session rows; its three work-signal
  modules are specified here but not shipped by this task — see Data
  requirements;
- the notice strip and the health detail it opens;
- safe provider quick actions with explicit confirmation and result reporting;
- manual refresh, and the periodic-refresh and login-item preferences the
  settings window presents;
- the menu-bar item and its right-click menu;
- six presentation states derived from foundation coordinator state;
- English and Simplified Chinese localization;
- accessibility, keyboard, motion, contrast, and layout behavior with XCTest
  assertions and a manual verification checklist.

The settings window is a separate surface with its own state set and copy; it is
specified in [`settings.md`](settings.md).

## Non-goals

This task does not deliver:

- the WidgetKit extension, its timelines, or its App Intents;
- signing, notarization, universal packaging, Cask, or DMG publication;
- **any update check**. It is withdrawn from this version: no automatic check, no
  manual check, no version comparison, and no release page. Nothing in the menu,
  the settings window, or the copy refers to updates;
- a `~/.local/bin/agentdeck` link action;
- credential creation, editing, or deletion;
- provider definition or wrapper management;
- session browsing beyond the bounded recent-session summary;
- a second refresh state machine, data store, or aggregation layer in Swift.

## Decisions

| Area | Decision |
| --- | --- |
| Candidate source | Desktop wire v1 gains an additive `provider.candidates` array. Go owns candidate discovery, eligibility, and redaction. |
| Switch surface | The host performs a switch through a dedicated helper invocation, not through the snapshot command. |
| Switch safety | Every switch requires explicit user confirmation and reports a typed result. The host never switches implicitly or on refresh. |
| Localization | The UI ships `en` and `zh-Hans` through a String Catalog. All numbers, currency, dates, and relative times use the viewer's locale. |
| Accessibility evidence | Derivable behavior is asserted in XCTest. Assistive-technology and perceptual behavior uses the manual checklist in this document. |
| Refresh cadence | Launch and manual refresh first update the rebuildable usage and session indexes, then read one snapshot. Periodic refresh is opt-in, defaults off, and uses the snapshot's `next_refresh_at`. A failed scan falls back to the last committed indexes. |
| Login item | `SMAppService.mainApp` only. Enable and disable are idempotent and never install a daemon. |
| Update check | Not in this version. See Non-goals. |
| Filter scope | The client and period filters apply to **every** panel in the reading surface. A filter that governs half the surface, silently ignored by the other half, is the defect this replaces. |
| Unfiltered content | Content the filters cannot govern does not sit among the filtered panels. Rhythm is a fixed 30-day window, so it lives below them as its own block, stating that window. |
| Exits | Settings and Quit belong to the menu-bar item's own menu, not to the reading surface. A popover that carries its own application exits spends its scarcest space on the two things a user needs least often. |
| Preference storage | Preferences live in the application's own `UserDefaults` domain, never in `~/.agentdeck` and never in the App Group cache. |

## Presentation architecture

### Layering

```text
MenuBarExtra scene
    |
    | reads
    v
MenuBarViewModel            (@MainActor, @Observable)
    |
    | derives presentation from, and requests operations on
    v
DesktopRefreshCoordinator   (foundation-owned, unchanged authority)
```

`MenuBarViewModel` is the only place that converts coordinator state into
presentation values. It owns no refresh state machine, performs no process
execution, no wire decoding, and no App Group access. Views read the view model
and never compute freshness, formatting, or availability themselves.

Preference state (`periodic refresh`, `login item`, menu-bar value and scope)
lives in a separate `DesktopPreferences` type so presentation and preference
persistence stay independently testable.

### Presentation state

Presentation is derived from foundation coordinator state. No new state machine is
introduced, but the six names are **not** mutually exclusive, so treating them as
one enum leaves real combinations undefined: `degraded(previous, timeout)` is both
stale and failing, and `degraded(previous, invalidWire)` is both stale and failing.

The model is therefore one **surface** plus independent **qualifiers**.

The surface answers only one question — is there a snapshot to show?

| Surface | Condition |
| --- | --- |
| `loadingSurface` | No snapshot has ever been retained |
| `dataSurface` | A snapshot is retained, whatever its age or issue |
| `errorSurface` | No snapshot is retained and the coordinator is `degraded` |

**A retained snapshot is always shown.** No issue category, including
`invalidWire`, replaces existing data with an empty surface; the issue becomes a
qualifier on top of it. This resolves the earlier contradiction where the error
copy demanded showing nothing while the retention rule demanded showing the
snapshot.

Qualifiers are orthogonal and may all apply at once:

| Qualifier | Condition |
| --- | --- |
| `stale` | Coordinator is `refreshing` or `degraded` while a previous snapshot is retained |
| `aged` | Retained snapshot's `generated_at` is older than 15 minutes |
| `partial` | Snapshot's `partial` flag is set, or any data domain reports `available: false` |
| `offline` | Issue category is helper launch or missing helper |
| `failing` | Issue category is helper timeout, invalid wire, or storage unavailable |
| `empty` | Not `partial`, every data domain is `available: true`, and none carries rows, tokens, or costs |

Exhaustive truth table over coordinator state, issue category, and whether a
previous snapshot exists:

| Coordinator | Previous snapshot | Surface | Qualifiers |
| --- | --- | --- | --- |
| `uninitialized` | — | `loadingSurface` | none |
| `refreshing` | no | `loadingSurface` | none |
| `refreshing` | yes | `dataSurface` | `stale`, plus `aged`/`partial`/`empty` as they hold |
| `ready` | — | `dataSurface` | `partial` or `empty` as they hold |
| `degraded`, launch/missing | yes | `dataSurface` | `stale` + `offline`, plus `aged`/`partial`/`empty` |
| `degraded`, launch/missing | no | `errorSurface` | `offline` |
| `degraded`, timeout/invalid wire/storage | yes | `dataSurface` | `stale` + `failing`, plus `aged`/`partial`/`empty` |
| `degraded`, timeout/invalid wire/storage | no | `errorSurface` | `failing` |

`empty` and `partial` are mutually exclusive by construction: `empty` requires
every data domain available, which `partial` denies.

`empty` is a property of the retained snapshot, not of the refresh outcome, so
it holds on the `degraded` rows exactly as it does on `refreshing` — a snapshot
that was complete and had no rows stays empty when the next refresh fails. The
two are independent: the surface shows an empty snapshot and the issue qualifier
explains that it is also out of date. An `errorSurface` row never carries
`empty`, because `empty` describes a snapshot and those rows have none.

Fixed qualifier order wherever more than one is shown — freshness first, then
reachability, then completeness: `stale`, `aged`, `offline`, `failing`,
`partial`, `empty`. The accessibility value announces them in the same order, and
the menu-bar glyph badges only when `offline` or `failing` holds, regardless of
which surface is showing.

Staleness age is computed from the snapshot's `generated_at`. A retained
snapshot older than 15 minutes is additionally labeled with its age. The host
never discards a valid snapshot because of age.

### Sections

An earlier draft listed six sections in an order nobody had derived, opening
with provider state and treating usage as the second thing a reader wanted. That
was backwards: a user opens a cost tracker to find out what they are spending.

A later draft fixed the order but left a second defect the prototype made
obvious. It put four sections behind one switcher while the client and period
filters above them governed only two: selecting `30 Days` changed magnitude and
composition and did nothing to trust or rhythm, with no disabled state and no
explanation. **A filter that governs half of what sits beneath it is broken
either way** — either it should govern all of it, or what it cannot govern does
not belong beneath it.

So the body is split by what the filters can reach.

**The filtered panels.** Four panels behind one switcher, every one of them
scoped by both the client tabs and the period switcher:

| Order | Panel | The question | Contents |
| --- | --- | --- | --- |
| 1 | **Usage** | How much am I spending? | The trend chart with its readout, plus three stat chips |
| 2 | **Breakdown** | Where does it go? | Model shares with bars, the token-component split naming cache write as billed, and the per-client subtotals |
| 3 | **Attribution** | Is the number real? | Determinable, inferred, and unattributed amounts with shares; pricing coverage with its unpriced identifiers; and the per-provider rows |
| 4 | **Sessions** | How did I work? | Session count, average length, project count; the three work-signal modules; per-project and recent-session rows |

Breakdown follows the prototype structure literally: one static Models card
with at most four dot-labelled rows and one share track per row; one static
Token mix card with a four-segment stack above four separator-delimited rows;
and one `surfaceRaised` Client subtotals row beneath them. These are not
collapsible sections, and the client subtotals are not a third card.

The switcher carries an icon and a name per panel and **no value**. An earlier
draft put each panel's headline number on its own tab, which restated the hero
directly above it and spent 21 pt of a 760 pt surface saying the same thing
twice.

Usage's stat chips change with the period rather than being fixed, because two
of the three are degenerate for a single day: over `today`, `avg/day` and the
peak *day* both equal the day's total. `today` therefore shows the priciest
hour, the event count, and cache hit; `7d` and `30d` show `avg/day`, the peak
day, and cache hit.

Sessions is a filtered panel because sessions carry a client and a time, so both
filters reach them. The three work-signal modules live there rather than under
Usage because what they describe — activity mix, workflow shape, tool calls — is
what happened *inside* the sessions.

**The rhythm block.** Rhythm is a fixed last-30-days window: a one-day rhythm is
not a rhythm, and neither filter can change that. It therefore sits below the
filtered panels as its own block rather than among them, stating its window in
its own heading, and the reader scrolls to it. It carries the 7×24 hour-of-week
grid and the 90-day calendar; the four figures above them — active days, busiest
and quietest weekday, peak window — are derived from the same two grids and
never stated independently of them. Their visible notes define the reductions:
active is the number of days with tokens, busiest and quietest are the weekday
totals with most and fewest tokens, and peak window is the weekday/hour cell
with most tokens.

**Above them** sit the **client tabs**, which filter every panel at once and
carry each client's own subtotal; the **hero**, which states the selected scope's
cost, token total, and event, session, and project counts; and the **period
switcher**. Tabs and switcher filter; they never mutate.

**The notice strip** sits at the top of the scrolling content, above the selected
panel. It carries, in severity order, an unreadable-data notice, a partial-data
notice, and a health notice; it is absent entirely when none holds. It is part of
the content rather than a bar pinned above the footer, because a pinned bar
covers the data the user opened this for, and an inline expansion of it pushes
the footer out of shape. The health notice opens a **health detail** — a second
level of the content area with a back control, listing every check and its
status. Health is therefore never a section: it qualifies the numbers rather than
being a fifth thing to read about spend. An unrecognized warning code is shown
verbatim as a code, never silently dropped, and recovery commands appear as
copyable text the app never executes.

**The freshness line** sits in the header beside the refresh control, because
when the data was updated and the control that updates it are one subject. The
cost-completeness note sits directly under the hero amount, because it explains
the `≈` that amount carries.

**The footer** is not a section because it answers no question about spend. It
carries current provider state and opens the provider menu, and nothing else.
The menu opens upward, so its disclosure points upward.

Every panel that reports `available: false` shows an explicit unavailable label
in place of its values, and a panel whose own subject is empty shows its own
empty copy rather than the whole surface claiming emptiness.

### Chart and heatmap interaction

Four rules govern every plotted region — the trend chart, the 7×24 grid, and the
90-day calendar. They were established against the shipped build on 2026-08-20
and the document had recorded none of them.

**Normalize against the real maximum.** A bar's height is its value over the
largest *positive* value actually present in the selected scope. There is no
floor on the divisor. An earlier build forced a minimum divisor of 1, so a day
whose hourly costs were all under a dollar drew every bar in the bottom sliver
and left the chart area empty — the chart reported the currency unit rather than
the shape of the day, which is the one thing a chart is for.

**The readout does not move.** Idle, hover, pinned, and keyboard selection all
render in the same fixed 26 pt row with the same padding and the same
background. Reading a chart must not reflow it.

**Keyboard selection without a focus ring on the chart.** Left and right select
the adjacent bucket, and the whole-chart macOS focus effect is disabled — an
unsolicited blue frame around the entire plotted region says the region is a
control, when the selection inside it is what the user is moving. The selected
bucket's own indication carries the state, and the accessible value follows the
selection.

**Pointer feedback is visible, not only accessible.** Every plotted cell
outlines on hover, shows its values in the visible readout, and carries native
help text: weekday, hour, tokens, and provider price for the 7×24 grid; date,
tokens, and provider price for the 90-day calendar. Normalized intensity drives
color only and is never presented as a percentage. An earlier build exposed a
percentage through accessibility values alone, which left a pointer user with
an unexplained number rather than the underlying usage and price.

The hourly chart always reserves the full local-day canvas: exactly 24 equal-width
slots for hours `0...23`, with axis labels `00`, `06`, `12`, `18`, and `24`.
Producer-supplied buckets through the current local hour carry the measured values;
later slots are zero-height layout placeholders, not measured zeroes, and never
participate in the priciest-hour selection. This fixed canvas preserves the shape
and scale of a day just after midnight instead of stretching one observed hour into
one full-width block. The visible readout still names the selected observed bucket
exactly.

### Rendered specimens

The reviewable prototype is [`prototype/interactive-v7/`](prototype/interactive-v7/).
It is a running application, not a static page: `npm install && npm run dev --
--port 4175`, then

| Surface | Address |
| --- | --- |
| Menu bar and popover | `http://127.0.0.1:4175/` |
| Twelve widgets | `http://127.0.0.1:4175/?surface=widgets` |
| Every degraded state, side by side | `http://127.0.0.1:4175/?surface=states` |

Its stage controls switch language, appearance, and data state; URL parameters
(`lang`, `theme`, `state`, `tab`, `signal`, `settings`) address any single
combination directly, which is what a review round cites. The panel is drawn at
its contract dimension, 1 px to 1 pt, in both appearances and both languages.

The prototype carries two self-checks, and a review round may run them:
`?surface=widgets&measure=1` reports every clipped container, and `?probe=1`
drives the surface with real events and asserts the interaction rules this
document states — filter propagation, chart hover/pin/keyboard, provider menu and
confirmation, the notice strip and health detail, signal detail entry and exit,
refresh failure and retry, the item's right-click and double-click menus, and the
settings window. A screenshot cannot show any of those.

The sketches below are the same states in text, kept because the contract is
reviewed as text and cited by blob hash. They are an index to the prototype, not
a substitute for it. Neither settles real SF type metrics, Dynamic Type reflow,
VoiceOver order, or measured contrast; those are runtime claims and stay in the
manual checklist.

Healthy, everything available. Width is the 420 pt default.

```text
┌──────────────────────────────────────────────┐
│ ⬢ AgentDeck              Updated just now  ⟳ │
├──────────────────────────────────────────────┤
│ [ All $9.06 ]  Codex $7.53    Claude $1.53   │
│                                              │
│ Today · Tue, Aug 18                    16.4M │
│ ≈$9.06                75 events · 4 sessions │
│ Cost incomplete · 14 unpriced   · 3 projects │
│                                              │
│ [ Today ]   7D        30D                    │
│ [ ▮ Usage ] ◔ Breakdown ⛨ Attribution ⟨⟩ Ses.│
├──────────────────────────────────────────────┤
│ ⚠ 2 checks not passing                     › │
│ ┌──────────────────────────────────────────┐ │
│ │        ▁▂▃▅▆█▆▃▂▁                        │ │
│ │ 00       06       12       18       24 │ │
│ │ Peak 15:00–16:00 · $1.49                 │ │
│ │ PRICIEST HOUR   EVENTS      CACHE HIT    │ │
│ │ $1.49           75          66.1%        │ │
│ └──────────────────────────────────────────┘ │
│ ──────────────────────────────────────────── │
│ ⏱ Rhythm      Last 30 days · not filtered    │
│ ACTIVE 27/30  BUSIEST Tue  QUIETEST Sun      │
│ ▓▒░ 7×24 grid ░▒▓                            │
│ ▓▒░ 90-day calendar ░▒▓            (scrolls) │
├──────────────────────────────────────────────┤
│ Providers  Codex aigocode · Claude official ⌃│
└──────────────────────────────────────────────┘
```

Loading, no snapshot ever retained — the only state that shows no data:

```text
┌──────────────────────────────────────────────┐
│   Loading…                                   │
└──────────────────────────────────────────────┘
```

Retained snapshot with the helper unreachable — `dataSurface` + `stale` +
`offline`. The data stays; the header states its age and the notice strip
explains the condition:

```text
│ ⬢ AgentDeck           Last updated 9h ago  ⟳ │
│ … filters and panels unchanged …             │
│ ⓘ Cannot reach the AgentDeck helper          │
```

Partial snapshot — the unavailable domain is named in place, and its tab is
marked, so the user is told which panel lost data rather than being left to find
it:

```text
│ [ Usage ]  Breakdown  [Attribution •] Sessions│
│ ⓘ Some data unavailable                      │
│ ┌──────────────────────────────────────────┐ │
│ │            ⚠  Some data unavailable      │ │
│ │               Attribution quality        │ │
│ └──────────────────────────────────────────┘ │
```

Health detail, opened from the notice strip — a second level of the content
area, not an expansion pinned above the footer:

```text
│ ‹ Back                              ⚠ Health │
│ State directory permissions                OK│
│ Credential key                             OK│
│ Usage index                           Warning│
│ Price catalog                          Failed│
│ Session index                              OK│
│ These checks come from agentdeck doctor.     │
```

Switch confirmation and its in-flight state, showing that every other row is
disabled with the overlay copy rather than its own reason:

```text
┌────────────────────────────────────────────┐
│ Switch Codex to aigocode using credential  │
│ "work", through the wrapper?               │
│                                            │
│              [ Cancel ]  [ Switch ]        │
└────────────────────────────────────────────┘

in flight
┌────────────────────────────────────────────┐
│ Switching…                                 │
│              [ Cancel ]  [ Switch ]        │  ← both disabled
│ ─────────────────────────────────────────  │
│   Claude → official      Switch in progress│  ← overlay, not its own reason
└────────────────────────────────────────────┘

failed
┌────────────────────────────────────────────┐
│ Switch failed · state_busy                 │
│ Another operation is using AgentDeck state.│
│              [ Dismiss ]  [ Retry ]        │
└────────────────────────────────────────────┘
```

At the 280 pt narrow bound, values wrap onto labelled continuation lines rather
than truncating:

```text
┌──────────────────────────────────┐
│ All ≈$9.06                       │
│                                  │
│ Today                    ≈$9.06  │
│   16.4M tokens                   │
│   75 events · 4 sessions         │
│                                  │
│ [ Usage ] Breakdown              │
│ Attribution  Sessions            │
└──────────────────────────────────┘
```

Empty, distinct from unavailable — a real zero rather than an unknown. The
filtered panels go to zero; rhythm keeps rendering, because a quiet day is only
legible against the days around it, and its 30-day window is not empty just
because today is:

```text
│ [ All $0.00 ]  Codex $0.00   Claude $0.00    │
│ Today · Tue, Aug 18                        0 │
│ $0.00                 0 events · 0 sessions  │
│ ┌──────────────────────────────────────────┐ │
│ │ ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁                 │ │
│ │        No local activity today           │ │
│ │ PRICIEST HOUR   EVENTS      CACHE HIT    │ │
│ │ —               0           —            │ │
│ └──────────────────────────────────────────┘ │
│ ⏱ Rhythm   ACTIVE 27/30 … still rendered     │
```

### Actions

| Action | Where | Behavior |
| --- | --- | --- |
| Refresh now | Header, and `⌘R` | Requests a replacement refresh. Disabled while a refresh is active. |
| Switch provider | Footer menu | Opens a confirmation naming the client, provider, credential, and wrapper route. Performs the switch only after confirmation. |
| Open health detail | Notice strip | Replaces the content area with the check list; a back control returns. |
| Copy recovery command | Health detail | Copies the snapshot's recovery command text to the pasteboard. |
| Open Settings | Item menu, and `⌘,` | Opens the settings window. See [`settings.md`](settings.md). |
| Menu-bar shows | Item menu | Sets the item's value mode without opening settings. |
| About | Item menu | Opens the standard about panel. |
| Quit | Item menu, and `⌘Q` | Terminates the application. |

No action mutates AgentDeck state except `Switch provider`. No action executes a
recovery command, opens a database, or reads a client configuration file.

## Preferences

Preferences are presented by the settings window, which is its own surface:
[`settings.md`](settings.md) specifies each control, its default, its failure
presentation, and its copy. This document owns only what the reading surface does
with them:

| Preference | Effect on this surface |
| --- | --- |
| Periodic refresh | When on, the coordinator schedules the next refresh from `next_refresh_at`; the header's freshness line is the only visible difference. |
| Launch at login | No effect on this surface. |
| Menu-bar value | Selects what the menu-bar item renders — cost, tokens, or icon only. |
| Menu-bar scope | Selects whether the item follows the popover's client filter or stays on all clients. |

Preference operations MUST be idempotent, MUST report the real post-operation
state rather than the requested state, and MUST NOT install a launch daemon,
launch agent, or privileged helper.

## Localization

The UI ships one String Catalog with `en` and `zh-Hans`. Every user-visible
string is localized, including state qualifiers, warning-code explanations,
health statuses, confirmation text, and accessibility labels.

- Numbers, token counts, and costs use locale-aware formatting. Cost strings
  arrive as decimal strings and are formatted for display without changing the
  underlying value or rounding a value the snapshot reported as exact.
- Timestamps and relative times use locale-aware formatting from the snapshot's
  RFC 3339 values.
- Provider names, client names, model names, project basenames, warning codes,
  health-check names, and recovery commands are data and are never translated.
- Layout uses leading/trailing alignment so the window is correct under
  right-to-left system settings even though no RTL language ships in v0.5.0.

## Menu-bar item

The item in the menu bar is the only part of this app visible at all times, so it
is specified before the window.

| Aspect | Specification |
| --- | --- |
| Glyph | The `AgentDeckMenuBarIcon` asset rendered as a monochrome template image, so macOS owns light, dark, and highlight appearance. No color glyph. |
| Base glyph | Three stacked cards form an `A` in negative space, combining the product initial with the deck metaphor while remaining legible at menu-bar size. |
| Status overlay | None in the normal case. The glyph gains a badge only for `error` and `offline`, using `exclamationmark.triangle.fill` as the badged variant. `stale` and `partial` are window-level qualifiers and MUST NOT badge the menu bar. |
| Text | Today's cost by default, matching the rendered prototype. A preference switches the value among cost, tokens, and icon-only when screen-sharing privacy or menu-bar space matters. Provider names never appear here. |
| Scope | The value covers all clients by default. A preference makes it follow the popover's client filter instead; without that preference the item never silently narrows because of something the user did inside the popover and then closed. |
| Incomplete cost | The value carries the same `≈` prefix the hero uses when pricing is incomplete, and nothing else explains it here — the explanation is one click away, and a menu-bar item is not the place to argue. |
| Left click | Toggles the popover. |
| Right click, and double click | Opens the item's own menu: menu-bar value, Settings…, About, Quit. This is where the application's exits live, so the popover does not carry them. |
| Animation | None, ever — including during refresh. A moving menu-bar item draws attention that the underlying event does not warrant. |
| Accessibility label | Localized app name plus, when badged, the state: for example `AgentDeck — 离线` / `AgentDeck — offline`. |

Restricting the badge to `error` and `offline` is deliberate: those two mean the
data on display cannot be trusted at all, while a stale or partial snapshot is
still usable and is qualified where the user actually reads it.

## Visual contract

Values are the specification, not suggestions, because an unbounded menu-bar
window is a real failure mode: this surface has four panels and a rhythm block whose content grows
with health checks and warnings.

### Geometry

| Property | Value |
| --- | --- |
| Window width | Fixed 420 pt at default Dynamic Type |
| Narrow bound | 280 pt — the minimum width every layout MUST remain correct at |
| Maximum height | 760 pt, or the shortest visible screen's height less 72 pt, whichever is smaller |
| Fixed regions | Header, client tabs, hero, period switcher, and panel switcher never scroll. They take their natural height — **no percentage of the window is reserved for them** |
| Scrolling region | The notice strip, the selected panel, and the rhythm block scroll as one column inside the remaining height, with the system scroll indicator hidden |
| Footer | Never scrolls; one row |
| Row minimum height | 28 pt, so a pointer target stays comfortable |
| Chart readout row | Fixed 26 pt, with fixed padding and a fixed background in every state |
| Provider popover | Fixed 250 × 260 pt, **identical at both levels** |

The width and height are the dimensions the prototype and the shipped host both
use. An earlier draft stated 340 pt and 560 pt while the implementation used
420 pt and 760 pt, and the drift went unnoticed because the document stated
geometry only as numbers with nothing rendered at them.

The scrolling region exists because rhythm sits below the filtered panels: the
surface is deliberately taller than one screenful, and scrolling to reach the
unfiltered block is the interaction, not an overflow accident. What must never
scroll is the filter set, because a filter the user cannot see while reading the
result it produced is worse than no filter.

**The scroll indicator is hidden while scrolling input is preserved.** At this
width the system's vertical indicator overlays the right edge of the content —
it clipped the peak card and both heatmap cards — and a popover the user is
already pointing at does not need to be told it scrolls.

**No region reserves space it is not using.** The 40% bound this table
previously stated for the fixed regions was never implemented; `MenuBarGeometry`
has no such constant, and a percentage floor would hold blank vertical space
open above the data whenever the header set happened to be short.

Three geometry values are fixed rather than intrinsic, and each is fixed to stop
a *jump* rather than to control size:

- The **chart readout row** is 26 pt with fixed padding and a fixed background in
  every state. Idle, hover, pinned, and keyboard selection must render at the
  same height, or reading the chart moves the chart.
- The **provider popover** is 250 × 260 pt at both its levels, so opening a
  candidate's executable targets does not resize the popover under the pointer.
- The **rhythm cards** keep the full content width, which is what hiding the
  indicator buys them.

These corrections were made on 2026-08-20 against the shipped implementation.
The document had recorded none of them.

### Typography and spacing

Use semantic text styles only, so Dynamic Type and Increase Contrast work without
per-view arithmetic. No fixed point sizes for text.

| Element | Style |
| --- | --- |
| Section header | `.subheadline` with `.secondary` foreground |
| Row label | `.body` |
| Row value | `.body`, monospaced digits for numbers, tokens, costs, and times |
| Qualifier and warning text | `.caption` |
| Recovery command | `.caption` monospaced |

Spacing uses a 4 pt scale: 4 within a row, 8 between rows, 16 between sections,
and 12 window padding. No other values.

### Color and status

The surface uses **one owned palette**, defined once as
`DesktopVisualTheme` in `apps/macos/AgentDeckApp/MenuBarSurfaceView.swift`, with
an explicit light and dark value for every token. It is not built from
`.primary` / `.secondary` / system semantic colors.

**Corrected 2026-08-20.** This section previously read "Semantic colors only …
No fixed hex values anywhere", which the prototype never did and the shipped app
never did. The prototype defines its own palette in `src/styles.css` and the
implementation mirrors it token for token; the document was describing an
intention that was abandoned before either was built, which is how a
four-colour series chart came to be specified as a single tint.

Why an owned palette rather than the system's: the surface renders a dense,
multi-series data view inside a popover it does not own the background of. The
system semantic set has no four-way categorical series, and `.primary` /
`.secondary` alone cannot express the four surface elevations
(`background`, `surface`, `surfaceRaised`, `surfaceEmphasis`) the layout uses to
group cards. The cost of owning the palette is that contrast is this document's
obligation rather than the system's — see Accessibility.

| Token group | Tokens | Role |
| --- | --- | --- |
| Accent | `accent`, `accentStrong` | The product's own identity colour; interactive emphasis |
| Status | `info`, `good`, `warning`, `error` | The four status roles below |
| Series | `series[0…3]` | Categorical data series only — never status, never emphasis |
| Elevation | `background`, `surface`, `surfaceRaised`, `surfaceEmphasis` | Card and panel grouping |
| Line | `line`, `lineSoft` | Separators and card strokes |
| Text | `text`, `muted`, `dim` | Primary, secondary, and tertiary text |

The **series palette is fixed and ordered**, and it is the same in both
appearances because a categorical hue that shifts between light and dark stops
being an identity:

| Index | Value | Bound to |
| --- | --- | --- |
| 0 | `#6F8FD6` | `gpt-5.6-sol`; Token `input` |
| 1 | `#B07BE0` | `claude-opus-5`; Token `output` |
| 2 | `#3FA08A` | `codex-auto-review`; Token `cacheRead` |
| 3 | `#CF8B5C` | `gpt-5.5` |

These bindings come directly from the prototype's `MODEL_DEFS` and `tokenMix`
roles, so producer ordering cannot change a known model's colour. Unknown model
identifiers fall back to rendered position, wrapping past four. Cache write uses
the status `warning` token (`#D9971A` dark), exactly as the prototype does, not
series index 3. Client subtotals are text in one inline row and carry no series
colour. Series hue identifies a known category or token role; it never encodes a
magnitude or status.

Every status carries a text label and an SF Symbol shape in addition to colour,
so the status survives grayscale, Increase Contrast, and colour-vision
differences:

| Status | Symbol | Token |
| --- | --- | --- |
| Healthy | `checkmark.circle` | `muted` |
| Warning | `exclamationmark.triangle` | `warning` |
| Error | `xmark.octagon` | `error` |
| Unavailable / unconfigured | `minus.circle` | `dim` |

A muted healthy state is intentional: a healthy system should be quiet rather
than decorated with green.

### Density control

Three regions grow without bound and MUST be bounded in presentation:

| Region | Rule |
| --- | --- |
| Notice strip | At most one notice per condition, never one per failing check. The health notice states a count and opens the detail |
| Health detail | Lists every check; it is a second level of the content area and may scroll |
| Warnings | At most 3 in the notice strip, then a localized "and N more" that opens the same detail |

The health count belongs in the strip and the list belongs in the detail because
they answer different questions: whether anything is wrong, and what. An earlier
draft expanded the list in place above the footer, which covered the data and
deformed the footer at the same time.

Recent sessions and per-project rows are already bounded by the snapshot. Panels
whose data is absent collapse to a single unavailable label rather than an empty
header.

## Interaction states and copy

The surface and qualifier model above defines *derivation*. This defines what the
user reads. Each surface has one copy; each qualifier has its own, and they
compose in the fixed order.

| Surface | `en` | `zh-Hans` | Data shown |
| --- | --- | --- | --- |
| `loadingSurface` | `Loading…` | `加载中…` | Nothing yet, no spinner beyond the system idiom |
| `dataSurface` | No surface copy; qualifiers speak | 同上 | The retained snapshot in full, always |
| `errorSurface` | The specific failure, never a generic message | 同上 | No data exists to show, plus a retry affordance |

| Qualifier | `en` | `zh-Hans` |
| --- | --- | --- |
| `stale` | `Updated <relative>` | `<相对时间>更新` |
| `aged` | `Last updated <relative>` | `上次更新于<相对时间>` |
| `offline` | `Cannot reach the AgentDeck helper` | `无法连接 AgentDeck 助手` |
| `failing` | `Data could not be read` | `无法读取数据` |
| `partial` | `Some data unavailable` | `部分数据不可用` |
| `empty` (current) | `No local activity today` | `今天没有本地活动` |
| `empty` (with any freshness or reachability qualifier) | `No activity in this snapshot` | `此快照中没有活动` |

The user never reads the words "stale" or "offline" as labels. Saying when data
was updated is honest and actionable; calling it stale is a judgment the user did
not ask for. The reachability copy names the helper, because the network is not
what failed.

`empty` has two forms because one of them would otherwise lie. It holds on the
`degraded` retained rows, where the app is showing a snapshot it could not
refresh — so `No local activity today` beside `Cannot reach the AgentDeck
helper` would assert something about today that the app currently has no way to
know. Whenever `stale`, `aged`, `offline`, or `failing` accompanies it, `empty`
describes the snapshot instead of the day. Only a current, issue-free surface
may claim the day.

`aged` replaces `stale`'s wording rather than adding to it — one freshness
statement, not two. Every other combination appears in the fixed order:
`stale`/`aged`, `offline`, `failing`, `partial`, `empty`.

Where each of these is *rendered* is fixed too, because a qualifier in the wrong
place reads as being about the wrong thing:

| Copy | Rendered |
| --- | --- |
| `stale`/`aged` freshness | Header, beside the refresh control |
| Cost completeness | Directly under the hero amount, explaining its `≈` |
| `offline`, `failing`, `partial`, health | The notice strip at the top of the content |
| A panel's own unavailable or empty copy | Inside that panel |

| Element | `en` | `zh-Hans` |
| --- | --- | --- |
| Cost incomplete | `Cost incomplete · <n> unpriced` | `成本不完整 · <n> 项未计价` |
| Health notice | `<n> checks not passing` | `<n> 项检查未通过` |
| Health detail title | `Health` | `运行状况` |
| Check status | `OK` / `Warning` / `Failed` | `正常` / `警告` / `失败` |
| Rhythm scope | `Last 30 days · not affected by the filters above` | `近 30 天 · 不受上方筛选影响` |
| Work signals, not yet captured | `Not captured yet` | `待采集` |

The rhythm scope line is not decoration. It is the sentence that keeps the
filters honest: it states, where the user is looking, why the block above
changed and this one did not.

### Provider switch flow

The one mutating action, specified end to end:

1. **Selection** — choosing one `option` opens confirmation, carrying that
   option's exact `(client, provider, credential?, via_wrapper)`. A candidate is
   never selected as a whole, and the row is never itself the commit.
2. **Confirmation** — names client, provider, credential, and wrapper route in
   one sentence, with `Switch` and `Cancel`. `Cancel` is the default focus.
3. **In flight** — the confirmation stays open with its controls disabled and a
   progress indicator. Every other action on the surface is disabled. A second
   submit is impossible because the control is gone, not merely ignored.
   Every option row elsewhere on the surface is disabled with
   `Switch in progress` / `正在切换`. This overlay is host state, not wire
   state: it does not alter an option's `ready`, `reason_code`, or arguments,
   and it is shown *instead of* an option's own reason while it holds, because
   a global block explains more than a per-option one. The overlay belongs to
   `inFlight` alone: the moment the controller reaches `failed` or
   `indeterminate` the rows revert to exactly what the snapshot says, because a
   finished switch is not progress and saying otherwise beside a failure would
   be false.
4. **Success** — the confirmation closes, the switched row updates from the next
   refresh, and a transient row states `Switched <client> to <provider>` /
   `已将 <client> 切换到 <provider>`. It clears on the next refresh or after 10
   seconds.
5. **Failure** — the confirmation stays open, shows the localized failure with
   its code, and offers `Retry` / `重试` and `Dismiss` / `关闭`. The previously
   selected provider is still displayed as current, because it still is.
   `Retry` re-runs the same target and moves the controller atomically back to
   in flight; `Dismiss` clears the result and returns it to idle. The word
   `Cancel` is deliberately not reused here: it means "do not start" on the
   confirmation in step 2, and step 6 states that nothing cancels a switch once
   launched, so offering `Cancel` beside a finished failure would suggest an
   operation could still be called off. Starting a *different* switch requires
   dismissing this one first, so an unread failure cannot be silently
   abandoned. An indeterminate outcome offers the same two actions and reads as
   step 7 describes.
6. **Cancellation** — closing the window during an in-flight switch does not
   cancel it; the operation completes and its outcome appears on next open. The
   app never abandons a half-applied configuration change.

7. **Indeterminate** — a timeout after the helper launched, or a result that
   arrived in a shape the transport contract cannot classify, leaves the
   outcome unknown. The confirmation reports that the result could not be
   confirmed, the app forces a replacement refresh, and the reconciled snapshot
   is what the user reads next. It MUST NOT claim either success or failure.
   It offers `Retry` / `重试` and `Dismiss` / `关闭`, the same two actions as a
   failure, because the controller treats both terminal states identically.

Switching is globally single-flight, as specified under Operation ownership: one
switch app-wide, and a *new* switch requested while one is active is refused
rather than queued. `Retry` on a terminal failure or indeterminate result is not
a new switch — it moves the controller atomically back to in flight for the same
target, so no window exists in which a second switch could interleave.

## Accessibility, motion, contrast, and layout

The menu-bar window MUST:

- give every row an accessibility label, value, and — where the row is
  actionable — a hint, so VoiceOver announces meaning rather than raw glyphs;
- expose every active qualifier (`stale`, `aged`, `offline`, `failing`,
  `partial`, `empty`), in the fixed order, as part of the
  accessible value, not only as color or an icon;
- reach every control by keyboard, with a visible focus indicator and a logical
  focus order matching visual order;
- honor `accessibilityReduceMotion` by replacing any transition or progress
  animation with an immediate state change;
- honor `accessibilityIncreaseContrast` and Dark/Light appearance using semantic
  colors, never fixed hex values;
- never encode meaning in color alone: every status carries text or a shape;
- remain usable at the narrow layout bound and at large Dynamic Type sizes, with
  wrapping rather than truncation for state and warning text.

## Security and privacy

- The host reads only the foundation coordinator's snapshot state and the
  application's own preference domain.
- The switch action is the only mutation, requires confirmation, and passes an
  argument array through the foundation runner with no shell.
- The pasteboard receives only recovery-command text the snapshot already
  contains.
- Session identifiers, endpoints, wrapper URLs, credential references,
  credential values, configuration contents, and file paths are never displayed,
  logged, copied, or persisted.
- Logging keeps the foundation's fixed-classification policy. New events use
  fixed codes for switch attempt, switch outcome category, preference change,
  and preference change. No dynamic value is logged.
- The App Group cache contract is unchanged. Menu-bar work adds no field and
  never writes the cache directly.
- Tests use synthetic snapshots, synthetic candidates, an injected preference
  domain, and a stubbed switch and update transport. No test reads real
  AgentDeck, Codex, Claude, App Group, or network state.

## Verification

Verification level L3.

### Go

- `internal/desktop` candidate construction: available, unavailable, built-in
  and custom providers, multiple credentials, absent secret, configured wrapper,
  and the negative assertion that no endpoint, reference, multiplier, or URL
  reaches the snapshot.
- Fixture reproducibility for the extended complete and partial envelopes.
- `scripts/run-go-test.sh ./...` because this changes a persisted JSON contract.

### Swift

- Every row of the surface-and-qualifier truth table, including the combinations
  that hold together, and the rule that a retained snapshot is always shown.
- Section rendering for available, unavailable, empty, and incomplete-pricing
  inputs, including the assertion that an incomplete cost is never labeled
  complete.
- Warning-code mapping including an unrecognized code.
- Switch flow: confirmation required, exact resolved arguments, success
  triggering one replacement refresh, and each typed failure category leaving
  presented state unchanged.
- Preferences: idempotent enable and disable, real-status reporting, and the
  menu-bar value and scope modes each changing what the item renders.
- The notice strip: each severity present alone and together, in the fixed order,
  and absent entirely when no condition holds.
- Filter propagation: every panel re-reads at the selected client and period, and
  the rhythm block does not.
- Localization: both catalogs resolve every key, and no key falls back to its
  identifier.
- Accessibility-derivable behavior: label/value/hint presence, qualifier in the
  accessible value, focus order, reduce-motion branch, and narrow-layout branch.
- Privacy: negative assertions that prohibited values never appear in presented
  strings, pasteboard content, or log classifications.

### Manual checklist

Recorded in the review record with the observed result for each item on macOS 26:

1. VoiceOver reads every panel and action in a sensible order with meaningful
   labels, and announces which panel the switcher has selected.
2. Full keyboard access reaches every control with a visible focus indicator.
3. Reduce Motion removes transitions and progress animation.
4. Increase Contrast keeps every status legible.
5. Light and Dark appearance switching keeps every status legible.
6. `en` and `zh-Hans` both render without truncation or clipping.
7. At the 280 pt narrow bound, state and warning text wraps rather than
   truncates, at default and at the largest Dynamic Type size.
8. Content exceeding the height bound scrolls inside it; the window neither grows
   past the bound nor clips a row.
9. The menu-bar glyph is unbadged when only `stale`, `aged`, `partial`, or
   `empty` hold, and badged whenever `offline` or `failing` holds, on either
   surface. It never animates and never renders text.
10. The notice strip shows at most one notice per condition; the health detail
    lists every check and returns to the panel the user left.
11. A switch in flight disables every other action, and the confirmation's own
    controls, so a second submit cannot be issued.
12. A failed switch keeps the previous provider displayed as current and shows
    the failure code without the underlying message text.
13. Candidate discovery failure with readable routes still shows current routes,
    with the switch affordance visibly disabled rather than absent.
14. The notice strip is absent when nothing is wrong, and the health detail opens
    and returns without disturbing the selected panel or the scroll position.
15. A successful switch through the canonical invocation produces an empty
    stderr, confirming `--quiet` suppresses the route and advisory reporters.
16. A candidate with several options presents one row per option; credential and
    direct-versus-wrapper are never inferred, and each `ready: false` option is
    listed and disabled with its localized reason.
17. The legacy v1 fixture without `candidates` decodes with an empty candidate
    list, and a present non-array value is rejected.
18. A switch is refused, not queued, while another is in flight anywhere in the
    app.
19. A timeout after helper launch reports an unconfirmed result, forces a
    replacement refresh, and claims neither success nor failure.
20. Window dismissal during an in-flight switch does not terminate the helper,
    and the outcome is visible on reopen.
21. A valid envelope delivered on the wrong stream — an `error` envelope on
    stdout, or a success envelope on stderr — is reported as unconfirmed rather
    than as success or failure, shows no outcome code, and forces a replacement
    refresh.
22. `Retry` on a failed or indeterminate result starts the same target with no
    observable idle state between, and a switch to a *different* target is
    refused until the result is dismissed.
23. While a switch is in flight, every other option row is disabled reading
    `Switch in progress` / `正在切换` in place of its own reason, and each row
    reverts to exactly the snapshot's `ready` and `reason_code` once the
    controller leaves the in-flight state.

### Not verifiable in this task

Signing, notarization, universal helper assembly, Cask and DMG installation, and
Widget behavior belong to later tasks. Record them as out of scope rather than as
untested risk.

## Acceptance criteria

- Desktop wire v1 carries `provider.candidates` additively, Go owns its
  redaction, and both fixtures and both decoders agree.
- The menu bar presents four filtered panels and the unfiltered rhythm block,
  each with explicit unavailable, empty, and incomplete-pricing handling.
- All six presentation states derive from foundation coordinator state with no
  second state machine.
- A provider switch happens only after explicit confirmation, uses exactly the
  confirmed candidate, and reports a typed outcome.
- Manual refresh always works; periodic refresh and login item are opt-in,
  default off, idempotent, and report real state.
- The client and period filters govern every filtered panel; the rhythm block
  states its own fixed window and no filter changes it.
- No surface, menu, or string refers to an update check.
- Every user-visible string resolves in `en` and `zh-Hans`.
- Accessibility-derivable behavior is asserted in XCTest and the manual
  checklist is recorded with observed results.
- No prohibited value appears in presented text, the pasteboard, logs, or the
  App Group cache.

## Data requirements

Per the progression in `docs/documentation-workflow.md`, a surface names the
fields it needs and
the contract provisions or refuses each. Every usage row below is read from the
wire snapshot's `data.usage.presentation` at the selected client scope; the App
Group projection is a downstream widget cache and is never a menu-bar data
source. These fields are provisioned by `architecture.md`'s menu-bar wire
extension:

| Element | Field |
| --- | --- |
| Menu-bar item value | selected `scopes[].periods.items[].totals` cost or token total, per the preference |
| Magnitude hero | selected period `totals` with its four token components, event count, and session count |
| Period switcher | the wire's three `today`/`7d`/`30d` period records; never a Swift-side reduction over `daily` |
| Trend chart | selected scope's bounded `daily` series, ≤ 90 buckets |
| `avg/day`, `peak`, cache-hit chips | selected period's producer-computed `average_per_day`, `peak`, and `cache_hit_share` |
| Composition model rows | selected period's top-N `models`, ≤ 12 |
| Composition token split | selected period's input, output, cached-read, and cache-write totals |
| Trust quality rows | current-period per-client and per-provider `(cost, tokens, count, share)` tiers |
| Trust coverage | current-period pricing `coverage`, ≤ 12 unpriced identifiers |
| Rhythm grid | producer-computed 7×24 hour-of-week intensity for the fixed last-30-days window; every cell also carries tokens and the provider-cost tuple for visible hover detail |
| Client tabs | selected period's per-client subtotals |
| Footer provider state | `provider.routes` |
| Switch menu rows | `provider.candidates[].options` |
| Freshness line | `generated_at`, `next_refresh_at` |

| Session panel statistics | `sessions.items[]` client, model, project, and start/end times, and `sessions.total` |
| Menu-bar item scope | the same per-client subtotals, so the item and the tabs cannot disagree |

Three elements this surface now presents are **not** provisioned by the current
projection. They are named here rather than quietly rendered from invented data,
and the prototype marks each of them `Not captured yet`:

| Element | Ruling in `architecture.md` | What this surface does |
| --- | --- | --- |
| Attribution under a period filter | **Provisioned** — `quality` and `pricing` move to the `Client` × `Period` product | The Attribution panel honors both filters, like the other three |
| Sessions under a period filter | **Provisioned** — `sessions` gains per-period grouped statistics beside its bounded recent list | The Sessions panel honors both filters |
| Work signals — activity mix, workflow shape, tool calls | **Provisioned by [`work-signals`](../../work-signals/architecture.md)**, a sibling `v0.5.0` topic — the classifier over raw session logs is a usage-domain capability, not a presentation one | The three modules are specified here and **shipped by `menubar-experience` in their `Not captured yet` form**: real headings, real layout, an explicit pending state instead of a number. `work-signals` replaces the pending state with values without changing the layout |

The pending modules keep their specification and their `Not captured yet`
treatment, in the prototype and in the shipped app alike, so the gap stays
visible and costed until `work-signals` closes it. What must never happen in the
meantime is the third row rendering plausible numbers: a work-signal module with
no capture behind it is an invented measurement in a product whose reason to
exist is that its measurements are real. That is also why the prototype's own
sample values under `PENDING_CAPTURE` in `prototype/interactive-v7/src/data.js`
are demonstration data and are never copied into the app — they are the *field
shape* `work-signals` must supply, not values to display.

**Corrected 2026-08-20.** The 2026-08-18 text called this a *refusal in this
topic* and the version index moved the capability to Backlog, which cut a
committed `v0.5.0` feature without asking. Reversed: the capability is in
`v0.5.0` and owned by `work-signals`.

Refused, with the ground stated in `architecture.md`: per-session cost. The
projection carries no per-session cost, so no session row states one.

## Downstream contracts

`desktop-widget` reads only the unchanged presentation-safe App Group
projection. The new candidate data stays in the host process.

`unified-desktop-distribution` owns signing, notarization, universal assembly,
and the App Group entitlement in release artifacts.

`desktop-app-contract` reconciles the delivered menu-bar behavior and the
additive candidate contract into `docs/specs/cli-design.md` and
`docs/specs/cli-manual.md`.

`settings.md` owns the settings window this surface opens. The two share the
preference domain and nothing else.

## Approval boundary

Design approval authorizes this contract to become the implementation and review
authority for `menubar-experience`. It does not authorize repair, review, commit,
push, signing, notarization, release, installation, or work on later desktop
tasks.
