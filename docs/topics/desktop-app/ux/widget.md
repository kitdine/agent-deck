---
status: active
created: 2026-08-16
updated: 2026-08-17
---

# Widget Experience

Surface: the AgentDeck WidgetKit widgets.
Task: `desktop-widget`.
Reads only the presentation-safe App Group projection specified in
[`../architecture.md`](../architecture.md#presentation-safe-app-group-projection).
Rendered specimens: [`prototype/desktop-surfaces.html`](prototype/desktop-surfaces.html).

## Purpose

Define which widgets exist, why those and not others, what each size of each
one shows, every state the data can be in, and the copy in both shipped
languages.

This document is normative. Product behavior must follow this contract even
when an earlier implementation differs from it.

## How the set is derived

An earlier draft of this document listed widgets one per interesting data field,
which produced seven unrelated cards and no answer to why there were seven. A
field is not a reason for a widget. **A question is.**

The projection carries four kinds of fact, and a user has exactly four questions
about their spend. They correspond one to one, which is what makes the set
closed rather than a list someone can always add to:

| Kind | The question | Fields it reads |
| --- | --- | --- |
| `magnitude` | How much am I spending? | `totals`, `buckets`, `peak`, average |
| `composition` | Where does it go? | `models`, `clients`, token components |
| `trust` | Is the number real? | attribution counts, pricing `coverage` |
| `rhythm` | When do I actually work? | 7×24 `activity` grid, active days |

**Four kinds, and adding a fifth requires a fifth question, not a fifth field.**
A proposal to add one is answered by naming the question it serves and showing
that none of the four already covers it. Per-project and per-session breakdowns
fail that test twice over: they are `composition` questions, and the projection
excludes the identifiers they would need.

The same four are the popover's sections, in the same order, so a user who
learns one surface can read the other. See [`menubar.md`](menubar.md).

## How size is chosen

Size selects **depth, not subject**. A widget of a given kind answers the same
question at every size; a larger family answers it with more of the evidence.
The earlier draft got this wrong — it made `rhythm` a medium and `trust` a small
as though they were different products, so a user who wanted the trust question
at a glance could not have it, and one who wanted rhythm compactly could not
either.

The progression is fixed:

| Size | Shows | Never shows |
| --- | --- | --- |
| `systemSmall` | One headline value plus one supporting line, and at most a sparkline | An axis, a legend, a table, or more than one dimension |
| `systemMedium` | The headline, one comparison across a single axis, and a chart with an axis | A second dimension of breakdown |
| `systemLarge` | The headline, the comparison, the full breakdown, and one secondary dimension | A third dimension, or anything needing scroll |

Four kinds by three sizes is twelve configurations. Each is specified below by
what it adds to the size beneath it, so a reader never has to diff two layouts
to find the difference.

## Configuration

One App Intent parameter per widget, `Client`:

| Value | `en` | `zh-Hans` |
| --- | --- | --- |
| `all` (default) | `All clients` | `全部客户端` |
| `codex` | `Codex` | `Codex` |
| `claude` | `Claude` | `Claude` |

A configured client absent from the projection is not an error: the widget shows
the unavailable state and keeps the configuration, since the client may reappear
on the next publication.

`magnitude` and `composition` additionally take `Period`: `today` (default),
`7d`, `30d`. `trust` and `rhythm` do not — `trust` is always the current period
because a stale trust figure is worse than none, and `rhythm` is always the last
30 days because a one-day rhythm is not a rhythm.

## The four widgets

### `magnitude` — how much am I spending?

| Size | Content |
| --- | --- |
| small | Period cost as the headline, tokens and session count beneath, a 7-bucket sparkline, and the freshness line |
| medium | Adds the three-period comparison — today, 7 days, 30 days, each with cost and tokens — and replaces the sparkline with a 20-bucket bar chart carrying a date axis |
| large | Adds `avg/day`, `peak` with its date, and cache-hit rate as stat chips, and extends the chart to the full 90-bucket bound with gridlines |

Cost leads rather than tokens, because a bill is denominated in money. Tokens
sit immediately beneath at every size, because a cost with no volume beside it
cannot be sanity-checked.

### `composition` — where does it go?

| Size | Content |
| --- | --- |
| small | The single largest model, its share, and a share bar |
| medium | The top four models as rows with token counts, shares, and bars |
| large | Adds the token-component split — input, output, cache read, cache write — as a second dimension, and the per-client subtotals |

Cache write earns its place at `large` rather than being folded into a total: it
is billed, and it reported zero until `v0.4.1` learned to count it. A breakdown
that omits the component the product just corrected is hiding its own
correction.

### `trust` — is the number real?

| Size | Content |
| --- | --- |
| small | Determinable share as the headline, a single bar, and the inferred and unattributed amounts on one line |
| medium | Adds the three quality tiers as rows with their own amounts and shares, and pricing coverage as a second bar |
| large | Adds per-provider quality rows, and the deterministically ordered unpriced model identifiers |

This is the widget no other tracker ships. Every one of them reports a total;
this reports how much of the total is a measurement rather than a guess.
Unattributed cost is shown as its own amount and never folded into the headline,
because a total that silently includes multiplier-`1` guesses is the defect this
product exists to remove.

### `rhythm` — when do I actually work?

| Size | Content |
| --- | --- |
| small | Active-day count over the last 30 days, and the single busiest hour named |
| medium | The 7×24 hour-of-week grid with day labels and quarter-day hour ticks, plus the intensity legend |
| large | Adds the 90-day daily heatmap above the grid, and the quietest and busiest day names for the same last-30-days window as small — a separate figure from the heatmap beside it, not a description of it |

The grid is the aggregate the terminal report already renders, so this widget
shows the product's own view rather than a new invention.

## Surface and qualifiers

The widgets reuse the menu-bar model's surface states and its freshness/
degraded **copy**, because a user seeing both must not read two different
sentences for one condition. See [`menubar.md`](menubar.md) for the
derivation. The two age qualifiers below are widget-local and intentionally
named differently from the menu bar's `stale`/`aged`: a widget has no refresh
state machine, so its age tiers are derived purely from `generated_at`, while
the menu bar's `stale`/`aged` are derived from coordinator refresh state.
Reusing the same qualifier names for two different derivations would invite
implementers to share one Swift type across both surfaces, which would be
wrong on at least one of them. The displayed *copy* still matches the menu
bar's freshness wording exactly (see Copy below) — only the qualifier
identifiers differ.

| Surface | Condition |
| --- | --- |
| `placeholder` | Redacted skeleton for the widget gallery; never real data |
| `dataSurface` | A projection exists and its cache schema version is supported |
| `unavailableSurface` | No projection, unsupported cache version, or malformed data |

| Qualifier | Condition |
| --- | --- |
| `aging` | Projection `generated_at` is 15 minutes to 6 hours old |
| `old` | Projection `generated_at` is more than 6 hours old |
| `partial` | Projection's partial flag is set |
| `empty` | Not partial, and every aggregate the widget shows is zero |

`aging` and `old` are mutually exclusive. `old` exists because a widget is
glanced at, not opened: a six-hour-old figure presented like a live one is the
failure the menu bar avoids by being opened deliberately.

Exhaustive over cache presence, version support, and age:

| Cache | Version | Age | Surface | Qualifiers |
| --- | --- | --- | --- | --- |
| absent | — | — | `unavailableSurface` | none |
| present | unsupported | — | `unavailableSurface` | none |
| present | supported | < 15 min | `dataSurface` | `partial`/`empty` as they hold |
| present | supported | 15 min – 6 h | `dataSurface` | `aging`, plus `partial`/`empty` |
| present | supported | > 6 h | `dataSurface` | `old`, plus `partial`/`empty` |

An unsupported cache version renders unavailable and never attempts a partial
read, matching the fail-closed rule the foundation already applies.

`empty` is per-widget, not global: a day with no spend leaves `magnitude` empty
while `rhythm` still has 30 days of history to draw. A widget shows the empty
copy only when its own subject is empty.

## Copy

| Element | `en` | `zh-Hans` |
| --- | --- | --- |
| `magnitude` title | `Usage` | `用量` |
| `composition` title | `Breakdown` | `构成` |
| `trust` title | `Attribution` | `归因` |
| `rhythm` title | `Activity` | `活动` |
| Scope, all clients | `All clients` | `全部客户端` |
| Scope, one client | `Codex` | `Codex` |
| Fresh / `aging` | `Updated <relative>` | `<相对时间>更新` |
| `old` | `Last updated <relative>` | `上次更新于<相对时间>` |
| `partial` | `Some data unavailable` | `部分数据不可用` |
| `empty`, `magnitude` | `No local activity today` | `今天没有本地活动` |
| `empty`, other kinds | `Nothing to break down yet` | `暂无可分解数据` |
| `unavailable` | `Open AgentDeck to refresh` | `打开 AgentDeck 以刷新` |
| Incomplete pricing | `Cost incomplete` | `成本不完整` |
| Cache-write note | `Cache write is billed` | `写缓存计费` |

Freshness and degraded wording match `menubar.md` exactly. A user who reads
`Some data unavailable` in the menu bar and something else in a widget would
reasonably conclude they are two different conditions.

`empty` differs by kind because the same words would be wrong: `magnitude` can
truthfully say there was no activity today, while `composition` has nothing to
divide and `trust` has nothing to qualify.

## Cost completeness

When the projection reports pricing as incomplete, the cost figure carries the
`Cost incomplete` label. A widget MUST NOT present an incomplete cost as
complete, and MUST NOT hide the figure: a partially priced total is still the
best available answer, and suppressing it would read as zero spend.

## Timeline

| Entry point | Behavior |
| --- | --- |
| `placeholder` | Redacted skeleton with representative shapes, never real values |
| `snapshot` | Current projection if readable, otherwise the placeholder |
| `timeline` | One entry for now, plus refresh-after at the projection's next suggested refresh time, clamped to 15 minutes minimum and 60 minutes maximum |

The clamp binds in both directions: below 15 minutes WidgetKit budget is spent
on data the host has not republished, and above 60 minutes a widget can drift
far past its `old` threshold without asking to be refreshed.

A widget reloads its timeline when the host signals a successful publication. It
never polls the cache and never invokes the helper.

## Accessibility

- Every value has an accessibility label naming what it measures; `$12.47` alone
  is not a label.
- The status symbol is never the only carrier of a state; qualifier text is
  always present.
- Qualifiers are announced in the same freshness-first order the menu bar
  uses: `aging`/`old`, `partial`, `empty`.
- At the largest accessibility Dynamic Type size each size degrades to the one
  beneath it — `large` shows what `medium` shows, `medium` what `small` shows —
  rather than truncating. That rule exists because the size progression is
  already a depth ordering, so dropping depth is the natural degradation.
- Charts carry an accessibility summary naming the range, peak, and trend
  direction, because a bar chart is unreadable to a screen reader otherwise.
- Full contrast compliance in light and dark, verified in the manual checklist.

## Security and privacy

- A widget reads the App Group projection and nothing else. It has no path to
  the databases, credentials, client configuration, or raw session sources.
- No prohibited value from `architecture.md`'s exclusion list may appear in a
  rendered string, an accessibility label, or a log.
- A widget writes nothing, anywhere.

## Data requirements

Per the progression in `docs/README.md`, a surface names the fields it needs and
the contract provisions or refuses each. These are provisioned as of
`architecture.md`'s eighth 2026-08-17 revision, which added the projection's
next suggested refresh time. The seventh revision makes pricing completeness
part of the same client-scoped per-period totals this table reads. The sixth
revision added the client scope this
table now cites on top of the per-period totals, per-period model shares,
per-period client subtotals, 30-day-scoped rhythm-day fields, and
per-quality-tier cost added by earlier revisions.

**Every row below is read at the configured `Client` scope**, because `Client` is
the one parameter all four widgets carry. `all` is a scope like any other, not
the absence of one. Where a row also varies by `Period`, the field is provisioned
for the product of the two, not for either alone.

| Element | Field | Varies by |
| --- | --- | --- |
| Every headline cost and token figure | per-period `totals` (`today`/`7d`/`30d`), each with its four token components | `Client` × `Period` |
| `magnitude` medium/large three-period comparison | all three periods' `totals` in one payload, not a Swift-side reduction of one | `Client` × `Period` |
| `magnitude` session count | per-period session count | `Client` × `Period` |
| `Cost incomplete` label | per-period pricing completeness accompanying the displayed totals | `Client` × `Period` |
| Sparklines and bar charts | bounded daily series, ≤ 90 buckets per scope | `Client` |
| `avg/day`, `peak` | each period's average and `peak` bucket | `Client` × `Period` |
| Cache-hit rate | cached-read over logical input | `Client` × `Period` |
| `composition` model rows, all `Period` values | per-period top-N model shares, ≤ 12 per period per scope | `Client` × `Period` |
| `composition` client rows, all `Period` values | per-period per-client subtotals, one set per supported period | `Period` — the row *is* the client breakdown, so it is read at `all` |
| `trust` quality rows (small/medium amounts, large per-provider rows) | per-client and per-provider attribution-quality `(cost, tokens, count, share)`, current period only | `Client` |
| `trust` coverage and unpriced list | pricing `coverage`, ≤ 12 unpriced identifiers per scope | `Client` |
| `rhythm` grid | 7×24 hour-of-week intensity per scope | `Client` |
| `rhythm` small active-day count | active-day count over the 30-day window | `Client` |
| `rhythm` large busiest/quietest day names | busiest and quietest day names over the 30-day window | `Client` |
| Timeline refresh-after | next suggested refresh time | neither — publication is one event |
| Every freshness line | `generated_at`, last successful refresh | neither — publication is one event |

The `composition` client-rows row is the one element `Client` does not scope: it
enumerates clients, so reading it under a single client would leave one row.
Under `Client = codex` that size shows the codex row alone, which is a
presentation choice this document makes rather than a second field.

Refused, with the ground stated in `architecture.md`: per-session and
per-project breakdowns, because the identifiers are in the exclusion list.

## Verification

Verification level L3.

### Swift

- Every row of the surface/qualifier truth table, including an unsupported cache
  version rendering unavailable rather than a partial read.
- All twelve configurations — four kinds by three sizes — render at their family
  size without clipping.
- The depth rule: each size shows a superset of the size beneath it for the same
  kind, asserted rather than eyeballed.
- Configuration: each `Client` value, each `Period` value on the kinds that take
  one, and a configured client absent from the projection.
- Per-widget `empty`: a zero-spend day leaves `magnitude` empty while `rhythm`
  still renders history.
- Timeline: the placeholder contains no real values; refresh-after honors both
  clamp bounds.
- Copy: both catalogs resolve every key; no key falls back to its identifier.
- Privacy: negative assertions that no excluded field appears in any rendered
  string or accessibility label.

### Manual checklist

Recorded in the review record with the observed result for each item on
macOS 26:

1. All twelve configurations render correctly in light and dark appearance.
2. At the largest accessibility Dynamic Type size each size degrades to the one
   beneath it rather than truncating.
3. VoiceOver announces each value with a meaningful label, qualifiers in the
   fixed order, and a chart summary naming range, peak, and direction.
4. Increase Contrast and grayscale keep every status and every series
   distinguishable.
5. The gallery placeholder shows no real data for any kind or size.
6. A widget left untouched past six hours shows `old`, not a fresh-looking
   figure.
7. With the host never launched, every kind shows unavailable rather than an
   empty or zeroed layout.
8. `en` and `zh-Hans` both render without truncation or clipping at every size.

### Not verifiable in this task

Signing, notarization, and Cask or DMG installation belong to
`unified-desktop-distribution`. Record them as out of scope rather than as
untested risk.

## Downstream contracts

`unified-desktop-distribution` delivers the App Group entitlement in release
artifacts; without it every widget renders unavailable, which is correct
behavior rather than a crash.

`desktop-app-contract` reconciles the delivered widget behavior into
`docs/specs/cli-design.md` and `docs/specs/cli-manual.md`.
