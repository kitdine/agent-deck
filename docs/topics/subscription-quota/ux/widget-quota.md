---
status: active
created: 2026-09-08
updated: 2026-09-09
---

# Subscription Quota — Widgets

Scope: the fifth widget kind this topic adds, at the three native sizes. The
existing four kinds are unchanged and out of scope.

The board goes from four kinds and twelve surfaces to five and fifteen. The
prototype's board subtitle changes with it, in both languages, because a
subtitle that still says twelve is a specimen that lies.

## Where the specimens come from

The specimen is the prototype at [`/prototype/`](../../../../prototype/).
**Where this document and the prototype disagree, the prototype is right.**

| What | How |
| --- | --- |
| All fifteen widgets | Open `?surface=widgets&lang=zh` |
| Change the quota payload | Use the visible **额度状态 / Quota state** switch in the stage bar |
| Change the small widget's client | Use the visible **小号端 / Small client** switch: Codex or Claude |
| In `en` | Switch language in the stage bar |
| The overflow gauge | append `&measure=1` |
| The full two-client size contract | append `&contract=1` while `额度状态 = Codex Plus` and `小号端 = Codex` |
| The absent-client contract | append `&contract=1` while `额度状态 = 门禁` and `小号端 = Codex` |

The query parameters remain only as stable deep links for review automation; a
human reviewer does not have to know or type them. Both quota state and the
small-widget client are visible controls in the same stage bar as width,
appearance, and language.

Widget cards are fixed sizes taken from the real WidgetKit families: small
148 × 148, medium 286 × 148, large 286 × 330. Nothing scrolls. A card that does
not fit is a design defect, not a runtime scroll, which is why the gauge and the
contract board both run against this surface.

All three sizes read the same selected quota payload. Showing the gate on one
size and full data on another would put contradictory claims about the same
account on one board. Only the **small client selection** differs: it chooses
which one of the available client blocks small projects, not a different data
snapshot.

## Size as depth

Size decides how many clients and windows fit, with one explicit configuration
choice for small.

| Size | Client treatment | Window treatment |
| --- | --- | --- |
| small | One configured client (`Codex` or `Claude`), chosen in the Widget configuration. | **The window with the highest used share**, as span, big percentage, bar, reset countdown. |
| medium | The same configured client. | Every window that client reports. |
| large | **Both clients**, split top and bottom into equal halves, each vertically centred in its half. | Every window of each client. |

## The two states the boundary adds

`requirements.md` clauses 1, 2 and 11 reach every surface, and a widget is where
they are hardest, because a widget has no room to explain itself and no place to
put a switch.

**Reading off.** A widget whose payload reports `probe_disabled` shows the
client name and `未读取` / `Not read` in place of the figure, at every size. It
does not go blank, and it does not fall back to a retained observation — the
retained value exists to make the switch cheap to turn back on, not to keep a
widget populated after the user stopped the reading that fed it. Small has room
for exactly this: a name and a state.

**Claude attribution.** `attribution_confirmed: false` is a permanent property
of Claude figures, not an error state. **All three sizes show it, and all three
show it on screen** — `账号未确认` / `account unconfirmed`, a 9 px dim line that
reads as a qualifier rather than a warning, because nothing is wrong.

An earlier version of this document put it in the accessible name and the
widget's `title` at small, reasoning that 148 × 148 has no room. That was wrong
twice over, and both errors are worth keeping written down. `requirements.md`
clause 11 asks the surface to *state* the limitation where the figures are; a
string only a screen reader can reach does not protect the person reading the
number with their eyes, which is most people looking at a widget. And Open
question 6 authorised choosing what a size shows *instead* — it never authorised
showing the number without the qualifier. Narrowing a requirement is a product
decision, and a UX document does not get to take one by asserting a layout
constraint.

The constraint also turned out not to be real. Measured on the specimen, the
short form fits at every size in both languages with no truncation and no
overflow. It sits:

| Size | Where | Why there |
| --- | --- | --- |
| small | Below the reset countdown, above the footer's freshness | It qualifies the one number on the card, and sits next to the freshness it must not be confused with |
| medium | Below the window rows, above the footer's freshness | Same relationship, one client, more rows |
| large | On the Claude block's header row, beside the client name | Two clients share one card and one footer, so the statement has to belong visibly to one of them |

**Codex carries nothing equivalent, at any size.** Its `accountId` confirms
attribution, so a line there would be decoration, and decoration next to a
qualifier teaches readers to skip both. On large this is directly visible: two
blocks, one qualified and one not.

The full sentence used in the popover — `无法确认账号归属` /
`account attribution unconfirmed` — is deliberately not reused here. A widget
has one line to spend, and the short form keeps the same claim without wrapping
onto a second. The two surfaces say the same thing at the length each can
afford; neither says it only to a screen reader.

**The official reset allowance is not a widget field.** A widget answers how much
quota is left; the allowance is an account asset whose meaning depends on each
credit's status and expiry, and a widget has nowhere to open that detail. A bare
count on a surface that cannot answer "which ones, and when do they expire"
raises a question it is then unable to answer. It stays in the popover, where the
hover popover carries the per-credit detail alongside it.

Small and medium differ in depth, not in scope: both answer for the one client
the user configured. Small has room for a single number, so it must be the number
that will stop the work first — the highest used share, not whichever window the
vendor returned first. Medium keeps the same client and spends its extra height
on that client's remaining windows rather than on adding the second client,
because a two-client medium would show each of them shallowly and answer neither
well.

Large is where both clients belong, and the split is **equal halves of the whole
card**, not proportional to window count and not merely equal to each other.
Codex Plus reports four windows and Claude two; sizing by content would make
Codex look twice as important, when their relative importance does not change
with how many windows a vendor happens to expose. Each half centres its own
content so neither reads as an afterthought.

"Equal halves" and "fills the card" are two requirements, and the second is the
one easily lost: a container sized to its content still yields two equal halves,
sitting together at the top with dead space beneath them. Both must hold.

**With only one client, the block is centred rather than stretched.** Equal
halves describes a relationship between two clients; with one there is nothing to
divide against, and stretching it to full height produces a large expanse of
empty tint. The single block keeps its content height and centres in the card, so
the tint stays attached to what it marks.

A client with no usable window is **not rendered** on large. The other client
keeps its space and remains legible; an unavailable placeholder does not consume
half the widget. In the gate specimen, Codex is not `official`, so large contains
only Claude. When no client has a usable window, the widget shows one generic
`无额度数据 / No quota data` state rather than client-specific failure boxes.

Small and medium never substitute the other client when the configured one has
no usable window; they show the same generic no-data state. Choosing Codex and
receiving Claude would make the configuration untrustworthy.

## Client distinction

Large carries both clients at once and needs more than a prefix repeated on each
row. Each client is a block with:

- a persistent left rule — Codex orange, Claude purple;
- a lightly tinted block background;
- the client name in a block header; and
- the plan beside that name when reported.

Colour is not the only carrier: the client name is always present. A reviewer who
cannot distinguish the two tones still sees two named regions. Window rows inside
a block do not repeat the client name — the block already carries it.

**Window rows do carry the quota's own name when the vendor gives one.** Codex
Plus returns four windows in which two spans appear twice: the main limit and a
per-model limit each have a 5-hour and a 7-day window. Labelling by span alone
produces two identical pairs, and the reader cannot tell whether 62 % is the
account's main limit or one model's. The per-model name is what separates them,
so it is part of the row, exactly as it is in the popover. A window with no
vendor name shows its span alone, which is itself the distinction from the named
ones.

Small and medium name their single client in the frame header instead, since
there is no second block to tell it apart from.

## The countdown lives on the label line

Each row is a label line over a bar, and the countdown sits on the label line
between the name and the percentage — not on a third line of its own.

This is a measured constraint, not a preference. With the countdown on its own
line the gauge reported `w-quota-rows` clipped by 16 px on medium and the
allowance line clipped by 19 px on large. Folding it onto the label line removes
both.

Medium's density was measured the same way, and then partly given back. When
medium still carried an allowance line, Codex Plus put four windows plus that
line into 148 pt: the gauge reported the allowance clipped by 14 px, then still
11 px after the gaps were tightened, and only a row label line-height of 1.25
closed it. Removing the allowance from widgets removed that constraint, so the
line-height returned to 1.4 and the gauge confirms four windows still fit. The
history is recorded because the tightened value would otherwise have survived as
a cost paid for a requirement that no longer exists.

## Freshness belongs in the frame footer

Every widget frame already has a footer carrying a freshness line. The quota
card uses it, and it says when the **quota** was read — not when the widget
refreshed.

The two are deliberately different clocks. Widgets target a 3–5 minute refresh
while quota probes run on a longer interval, so a quota card will routinely
display a figure older than its own refresh period. Putting the widget's clock
under a quota number would misdate it. The footer shows the **oldest** displayed
observation, because a card is only as fresh as its stalest figure.

When nothing has been probed, the footer carries the reason instead of a time,
and the body says so rather than rendering zeroes.

## Copy

| Key | `zh-Hans` | `en-US` |
| --- | --- | --- |
| `widgets.kinds.quota.name` / `.title` | 额度 | Quota |
| `widgets.kinds.quota.question` | 还剩多少？ | How much is left? |
| `widgets.quotaNoData` | 无额度数据 | No quota data |
| `quota.attributionShort` | 账号未确认 | account unconfirmed |
| `widgets.boardSubtitle` | 五个问题，三种深度，十五个原生尺寸 | Five questions, three depths, fifteen native surfaces |

Window spans, countdowns, and the read time reuse the `quota.*` keys tabled in
[`menubar-quota.md`](menubar-quota.md); the two surfaces must not drift into two
vocabularies for one fact. The allowance keys are popover-only.

Tone thresholds are the same as the popover's — under 75 % neutral, 75–90 %
elevated, 90 % and over warning — so a colour means one thing across the product.

## Data requirements

Identical to the popover's, with three differences: small needs a persisted
client selection (`codex` or `claude`) owned by the Widget configuration; the
widget needs no missing-field reason copy beyond the generic no-data case,
because unavailable clients do not render per-field rows; and it needs the
payload sorted or sortable by used share so "tightest" is resolvable without the
widget re-deriving it from raw windows.

`architecture.md` owns whether the tightest window is computed in the wire
payload or in the widget. This document requires only that both surfaces agree,
since a menu bar saying the 7-day window is tightest while the widget says the
5-hour window is the same class of contradiction as the provider footer.

## Rendered specimens

Indexes `?surface=widgets&lang=zh`; use the visible quota-state and small-client
switches in the stage bar. Sketches index the prototype; where they disagree the
prototype is right.

```
小，配置 = Codex            中，配置 = Codex，该端全部窗口
┌──────────────────┐        ┌──────────────────────────────────┐
│ ◔ 额度    Codex  │        │ ◔ 额度                    Codex │
│ 7 天窗           │        │ 5 小时窗              3h   45%  │
│ 62%              │        │ 7 天窗                4d   62%  │
│ █████████░░░░░░  │        │ GPT-5.3-Codex-Spark · 5 小时窗   │
│ 4d后重置         │        │                       2h   12%  │
│ <1m读取          │        │ GPT-5.3-Codex-Spark · 7 天窗     │
└──────────────────┘        │                       6d  4.0%  │
                            │ <1m读取                         │
   ↑ 已用最高的窗口          └──────────────────────────────────┘
                               ↑ 带额度名称，无重置次数

大，两端上下等分，各自居中
┌──────────────────────────────────────┐
│ ◔ 额度                   全部客户端 │
│ ┃ Codex                         plus │
│ ┃ 5 小时窗              3h     45%  │  上半
│ ┃ 7 天窗                4d     62%  │
│ ┃ GPT-5.3-Codex-Spark · 5 小时窗    │
│ ┃                       2h     12%  │
│ ┃ GPT-5.3-Codex-Spark · 7 天窗      │
│ ┃                       6d    4.0%  │
│ ────────────────────────────────────│
│ ┃ Claude              账号未确认    │  下半
│ ┃ 5 小时窗              3h     22%  │
│ ┃ 7 天窗                6d5h   3.0%  │
│ <1m读取                             │
└──────────────────────────────────────┘

小，配置 = Claude            中，配置 = Claude
┌──────────────────┐        ┌──────────────────────────────────┐
│ ◔ 额度   Claude  │        │ ◔ 额度                   Claude │
│ 5 小时窗         │        │ 5 小时窗              3h   22%  │
│ 22%              │        │ 7 天窗               6d5h  3.0%  │
│ ███░░░░░░░░░░░░  │        │ 账号未确认                       │
│ 3h后重置         │        │ 2m读取                          │
│ 账号未确认       │        └──────────────────────────────────┘
│ 2m读取           │
└──────────────────┘           ↑ 说明在数字与新鲜度之间

   ↑ 短式在卡上，不在 title 里
```

Only the Claude cards carry the line. Reading the small pair side by side is how
a reviewer checks that: the same layout, the same freshness row, and one extra
line on exactly the client that cannot prove whose account the number describes.

The sketches wrap the named rows for legibility in plain text; the prototype
keeps each row on one line and the gauge confirms none is truncated.

The two halves of large are equal in height regardless of window count, and each
centres its own content. In a one-client state the absent block is not drawn and
the remaining client takes the frame; no empty half, divider, client heading, or
unavailable row remains on behalf of the absent client.

## Verification

Run at design time and recorded, not deferred:

| Check | Command | Result |
| --- | --- | --- |
| No card overflows its fixed size, eight quota states × two languages × two client selections | `?surface=widgets&quota=…&widgetClient=…&lang=…&measure=1` | `NO OVERFLOW`, 32/32 |
| The two-client size contract holds, both languages, **both client selections** | `?surface=widgets&quota=codexPlus&widgetClient=codex` and `…&widgetClient=claude`, `&contract=1` | `ALL PASS` |
| The absent-client contract holds | `?surface=widgets&quota=normal&widgetClient=codex&contract=1` | `ALL PASS` |
| The reading-off state holds | `?surface=widgets&quota=readingOff&widgetClient=codex&contract=1` | `ALL PASS` |
| The attribution line is present, visible, and on the right card | contract assertions below, both languages, both client selections | `ALL PASS` |
| Single-file build | `npm run build:single` | Passes |

The contract board runs two specimens. With Codex Plus and the configured client
set to Codex, it asserts independently of the rendering code that: small renders
the configured client and exactly one window, and that window is the highest used
share (62 %, not the 45 % window the vendor returns first); medium renders the
same single client with all four of its windows and no second client block; large
renders both clients in Codex-then-Claude order with four and two windows
respectively, the **two halves equal in height**, and the split **filling the
card body**.

Those last two are separate assertions on purpose. The equal-height check alone
passed while the halves were bunched at the top with empty space below — content
sized, so equal to each other and wrong. The fill assertion was verified by
removing the layout fix and confirming it fails while equal-height still passes,
then restoring it.

With the gate state and the configured client still Codex, it asserts that large
contains only Claude, that the lone block is not stretched to the full card
height, and that neither small nor medium substitutes Claude for the selected but
unavailable Codex. Every rendered block must carry a plain-text
client name, and no row label may be eaten by `text-overflow`.

The gauge's ellipsis check did not cover the widget board until this round, which
mattered the moment window rows started carrying per-model names — the longest
labels on the surface were the ones nothing was watching. The board is now in
that root list, and the coverage was proved by temporarily capping the label
column, confirming the gauge reported four truncations with their exact pixel
shortfalls, then reverting. A check never observed to fail is not evidence.

**Six assertions cover the attribution line**, and they exist because the
previous round's evidence was a gauge reporting `NO OVERFLOW` on a board where
the line was absent entirely. A layout gauge proves nothing about whether
required content is there. They assert that the Claude block on large carries
it, that the Codex block does not, that small and medium carry it when
configured to Claude and do not when configured to Codex, and that it is neither
clipped by `text-overflow` nor pushed outside its card. Presence is checked on
the **rendered text with a non-zero box** — a `title` attribute or an accessible
name would not satisfy it, which is the point.

They were validated by suppressing the line and re-running: three failures when
configured to Claude, one when configured to Codex — the Claude block on large,
which is visible in either configuration — then restoring it.

**The board now runs under both client selections**, and making that possible
exposed a second defect in the board itself. Two assertions compared small's
headline and medium's row count against Codex's numbers while their labels said
"the configured client", so the contract had only ever been run in its default
configuration; pointing it at Claude made it report its own assumption as a
product failure. Expected values now follow the configuration. This is the same
class of defect as the menu-bar probe addressing tabs by position: a check that
silently only works in one configuration is a check with an unstated precondition.

One assertion checks that **none** of the three sizes contains the allowance
label. It looks redundant next to a rule stated in prose, and it is the one most
worth keeping: prose does not stop someone from adding the count back as an
apparently harmless line, and the widget still has nowhere to open the detail
that would make it answerable.

Expected counts are copied from the tables above rather than imported from
`Widgets.jsx`: an assertion that shares a constant with the code it tests can
never fail.

## What the specimen does not settle

- Real WidgetKit rendering, its own font metrics, and its tint behaviour.
- Whether the system refreshes a widget often enough for the footer's read time
  to stay honest, and what the card shows when the system declines to refresh.
- Lock-screen and StandBy families, which this topic does not add.
- Accessibility reading order inside a widget.

These belong to `tasks.md` as manual acceptance.
