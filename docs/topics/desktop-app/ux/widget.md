---
status: active
created: 2026-08-16
updated: 2026-08-16
---

# Widget Experience

Surface: the AgentDeck WidgetKit widget.
Task: `desktop-widget`.
Reads only the presentation-safe App Group projection specified in
[`../architecture.md`](../architecture.md#presentation-safe-app-group-projection).

## Purpose

Define what the widget shows, in which families, in every state its data can be
in, in both shipped languages, and what it must never show.

This document is normative. Product behavior must follow this contract even
when an earlier implementation differs from it.

## What the widget may know

The widget is a sandboxed extension. It never launches the helper, never reads
an AgentDeck database, and never sees the wire envelope. Its only input is the
App Group projection, whose permitted contents `architecture.md` fixes:

- cache schema version and source wire version;
- generation time, partial state, and last successful refresh time;
- client identifiers and selected provider display identifiers;
- aggregate usage totals, counts, pricing completeness, and cost strings;
- aggregate session availability and count;
- aggregate health status and problem/warning/error counts;
- allowlisted presentation-safe issue codes.

**Everything this document specifies is derived from that list.** Where a design
idea would need something outside it — a session title, a provider endpoint, a
per-model breakdown — the idea is rejected here rather than being specified and
discovered as impossible during implementation. That rejection is recorded in
Non-goals below.

## Scope

- `systemSmall` and `systemMedium` widget families.
- An App Intent configuration choosing which client the widget reports.
- Timeline construction, refresh cadence, and the placeholder/snapshot/timeline
  entry points.
- Every state the projection can present, including unavailable, stale, partial,
  and empty.
- English and Simplified Chinese copy for every string.

## Non-goals

- No `systemLarge` or `accessory*` family in this task. The projection carries
  aggregates only, and a large family would either repeat the medium layout at a
  larger size or demand per-session detail the widget may not read.
- No interactivity beyond the widget's own configuration and a tap that opens
  the app. There is no switch action: a mutation needs the helper, and the
  widget cannot invoke it.
- No session titles, model names, provider endpoints, or per-model breakdown.
  Not a layout decision — the projection excludes them.
- No independent refresh. The widget renders what the host published; it never
  causes a refresh, because that would require the helper.
- No color-only status. Every state carries text and a symbol, matching
  [`menubar.md`](menubar.md).

## Configuration

One App Intent parameter, `Client`:

| Value | `en` | `zh-Hans` |
| --- | --- | --- |
| `all` (default) | `All clients` | `全部客户端` |
| `codex` | `Codex` | `Codex` |
| `claude` | `Claude` | `Claude` |

A configured client absent from the projection is not an error: the widget
renders the unavailable state for that client and keeps the configuration, since
the client may reappear on the next publication.

## Surface and qualifiers

The widget reuses the menu-bar model rather than inventing a second one, because
a user seeing both must not see two different vocabularies for one condition.
See [`menubar.md`](menubar.md) for the derivation.

| Surface | Condition |
| --- | --- |
| `placeholder` | Redacted skeleton for the widget gallery; never real data |
| `dataSurface` | A projection exists and its cache schema version is supported |
| `unavailableSurface` | No projection, unsupported cache version, or malformed data |

| Qualifier | Condition |
| --- | --- |
| `stale` | Projection `generated_at` is 15 minutes to 6 hours old |
| `aged` | Projection `generated_at` is more than 6 hours old |
| `partial` | Projection's partial flag is set |
| `empty` | Not partial, and every aggregate the family shows is zero |

`stale` and `aged` are mutually exclusive. `aged` exists because a widget is
glanced at, not read: a figure six hours old presented like a live one is the
failure mode the menu bar avoids by being opened deliberately.

Exhaustive over cache presence, version support, and age:

| Cache | Version | Age | Surface | Qualifiers |
| --- | --- | --- | --- | --- |
| absent | — | — | `unavailableSurface` | none |
| present | unsupported | — | `unavailableSurface` | none |
| present | supported | < 15 min | `dataSurface` | `partial`/`empty` as they hold |
| present | supported | 15 min – 6 h | `dataSurface` | `stale`, plus `partial`/`empty` |
| present | supported | > 6 h | `dataSurface` | `aged`, plus `partial`/`empty` |

An unsupported cache version renders unavailable and never attempts a partial
read, matching the fail-closed rule the foundation already applies.

## Layout

### `systemSmall`

```text
┌──────────────────────────┐
│ AgentDeck          ▲     │   ▲ = status symbol, omitted when healthy
│                          │
│ $12.47                   │   today's cost, .largeTitle, tint
│ today · Codex            │   scope line, .caption, secondary
│                          │
│ Updated 3m ago           │   freshness, .caption2, secondary
└──────────────────────────┘
```

The small family answers one question — what has today cost — because a widget
this size cannot answer two without becoming a table nobody reads at a glance.

### `systemMedium`

```text
┌───────────────────────────────────────────────────┐
│ AgentDeck · All clients                    ▲      │
│                                                   │
│ $12.47            1.2M tokens         8 sessions  │
│ today             today               today       │
│                                                   │
│ Codex $9.10 · Claude $3.37      Updated 3m ago    │
└───────────────────────────────────────────────────┘
```

The per-client split line appears only when `Client` is `all` and more than one
client reports a cost. With a single client configured, that line is omitted
rather than repeating the total.

The reviewable prototype for both families, at their real point sizes and in
both appearances, is
[`prototype/desktop-surfaces.html`](prototype/desktop-surfaces.html). The
sketches here index those states for a reader following the text.

### Degraded specimens

```text
stale                         aged
┌──────────────────────┐      ┌──────────────────────┐
│ AgentDeck            │      │ AgentDeck        ⚠   │
│ $12.47               │      │ $12.47               │
│ today · Codex        │      │ today · Codex        │
│ Updated 2h ago       │      │ Last updated 9h ago  │
└──────────────────────┘      └──────────────────────┘

partial                       empty
┌──────────────────────┐      ┌──────────────────────┐
│ AgentDeck        ⚠   │      │ AgentDeck            │
│ $12.47               │      │ $0.00                │
│ today · Codex        │      │ today · Codex        │
│ Some data unavailable│      │ No activity today    │
└──────────────────────┘      └──────────────────────┘

unavailable
┌──────────────────────┐
│ AgentDeck        ⚠   │
│ ——                   │
│ Open AgentDeck       │
│ to refresh           │
└──────────────────────┘
```

`empty` shows `$0.00` rather than an em dash: zero spend is a fact, while an em
dash means unknown. `unavailable` shows the em dash because it is unknown.

## Copy

| Element | `en` | `zh-Hans` |
| --- | --- | --- |
| Title | `AgentDeck` | `AgentDeck` |
| Scope, all clients | `today · All clients` | `今天 · 全部客户端` |
| Scope, one client | `today · Codex` | `今天 · Codex` |
| Tokens label | `tokens` | `tokens` |
| Sessions label | `sessions` | `会话` |
| Fresh | `Updated <relative>` | `<相对时间>更新` |
| `stale` | `Updated <relative>` | `<相对时间>更新` |
| `aged` | `Last updated <relative>` | `上次更新于<相对时间>` |
| `partial` | `Some data unavailable` | `部分数据不可用` |
| `empty` | `No activity today` | `今天没有活动` |
| `unavailable` | `Open AgentDeck to refresh` | `打开 AgentDeck 以刷新` |
| Incomplete pricing | `Cost incomplete` | `成本不完整` |

Freshness and degraded wording match `menubar.md` exactly. A user who reads
`Some data unavailable` in the menu bar and something else in the widget would
reasonably conclude they are two different conditions.

`empty` says `today` and never `in this snapshot`, because the widget shows only
current-day aggregates and has no other snapshot to contrast with — the
distinction `menubar.md` draws does not arise here.

## Cost completeness

When the projection reports pricing as incomplete, the cost figure carries the
`Cost incomplete` label. The widget MUST NOT present an incomplete cost as if it
were complete, and MUST NOT hide the figure: a partially priced total is still
the best available answer, and suppressing it would read as zero spend.

## Timeline

| Entry point | Behavior |
| --- | --- |
| `placeholder` | Redacted skeleton with representative shapes, never real values |
| `snapshot` | Current projection if readable, otherwise the placeholder |
| `timeline` | One entry for now, plus refresh-after at the projection's next suggested refresh time, clamped to 15 minutes minimum and 60 minutes maximum |

The clamp exists in both directions: below 15 minutes WidgetKit budgets would be
spent on data the host has not republished, and above 60 minutes a widget can
silently drift far past its `aged` threshold without asking to be refreshed.

The widget reloads its timeline when the host signals a successful publication.
It never polls the cache and never invokes the helper.

## Accessibility

- Every value has an accessibility label naming what it measures; `$12.47` alone
  is not a label.
- The status symbol is never the only carrier of a state; qualifier text is
  always present.
- Qualifiers are announced in the same fixed order as the menu bar:
  `stale`/`aged`, `partial`, `empty`.
- The layout survives the largest accessibility Dynamic Type size by dropping
  the secondary metrics in `systemMedium` before truncating the cost.
- Full contrast compliance in light and dark appearance, verified in the manual
  checklist rather than asserted here.

## Security and privacy

- The widget reads the App Group projection and nothing else. It has no path to
  the databases, credentials, client configuration, or raw session sources.
- No prohibited value from `architecture.md`'s exclusion list may appear in a
  rendered string, an accessibility label, or a log.
- The widget writes nothing, anywhere.

## Verification

Verification level L3.

### Swift

- Every row of the surface/qualifier truth table, including an unsupported cache
  version rendering unavailable rather than a partial read.
- Both families at every state, including the omitted per-client line for a
  single configured client.
- Configuration: each `Client` value, and a configured client absent from the
  projection.
- Timeline: placeholder never contains real values; refresh-after honors both
  clamp bounds.
- Copy: both catalogs resolve every key; no key falls back to its identifier.
- Privacy: negative assertions that no excluded field appears in any rendered
  string or accessibility label.

### Manual checklist

Recorded in the review record with the observed result for each item on
macOS 26:

1. Both families render correctly in light and dark appearance.
2. Both families remain legible at the largest accessibility Dynamic Type size,
   dropping secondary metrics rather than truncating the cost.
3. VoiceOver announces each value with a meaningful label, and qualifiers in the
   fixed order.
4. Increase Contrast and grayscale both keep every status distinguishable.
5. The gallery placeholder shows no real data.
6. A widget left untouched past six hours shows `aged`, not a stale-looking
   fresh figure.
7. With the host never launched, the widget shows unavailable rather than an
   empty or zeroed layout.
8. `en` and `zh-Hans` both render without truncation or clipping.

### Not verifiable in this task

Signing, notarization, and Cask or DMG installation belong to
`unified-desktop-distribution`. Record them as out of scope rather than as
untested risk.

## Downstream contracts

`unified-desktop-distribution` delivers the App Group entitlement in release
artifacts; without it the widget renders unavailable, which is the correct
behavior rather than a crash.

`desktop-app-contract` reconciles the delivered widget behavior into
`docs/specs/cli-design.md` and `docs/specs/cli-manual.md`.
