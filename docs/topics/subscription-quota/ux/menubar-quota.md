---
status: active
created: 2026-09-08
updated: 2026-09-09
---

# Subscription Quota — Menu-bar Popover

Scope: the quota tab this topic adds to the menu-bar popover, and the change the
fifth tab forces on the tab strip itself. Everything else in the popover is
unchanged and out of scope.

`requirements.md` names this surface. It does not decide the layout; this
document does, and `architecture.md` must provision the fields it requests or
refuse each one with a reason.

## Where the specimens come from

The specimen is the prototype at [`/prototype/`](../../../../prototype/), which
[`prototype/README.md`](../../../../prototype/README.md) declares the single
design truth for every product surface. This document does not carry a second
one. **Where this document and the prototype disagree, the prototype is right.**

```bash
cd prototype && npm install && npm run dev -- --port 4175
```

| What | How |
| --- | --- |
| Q1 — the gate, Codex on a third-party provider | Open the popover; **额度状态 / Quota state = 门禁 / Gate** |
| Q2 — both clients `official`, every field present | Select **两侧 official / Both official** |
| Q2b — Codex Plus: the main limit carries both a 5-hour and a 7-day window | Select **Codex Plus** |
| Q3 — Claude via the `/usage` prose route | Select **Claude /usage** |
| Q4 — the prose shape changed and did not parse | Select **解析失败 / Parse failed** |
| Q5 — both clients stale | Select **额度过期 / Quota stale** |
| Q6 — never probed | Select **尚未探测 / Never probed** |
| Q7 — quota reading turned off | Select **读取已关闭 / Reading off** |
| Any of the above at the narrow bound | select `280 pt` in the stage bar |
| The reset-credit popover opening left, right, or overlaid | set **锚点 / Anchor** to 贴右 / 居中 / 贴左 and hover the allowance row |
| Any of the above in `en` | select `EN` in the stage bar |
| The overflow and truncation gauge | append `&measure=1` |

Quota read state is a **visible, independent switch group** in the prototype
stage bar; a reviewer does not need to know or type a `quota=` parameter. The
query parameter remains only as a stable deep link for automated review. It is
orthogonal to `state=`: usage data can be stale while quota is normal, or vice
versa, so folding both into the existing state group would make valid
combinations impossible.

**The visible quota-state control defaults to the gate.** The prototype's `PROVIDER` fixture
selects `aigocode` for Codex, so in the default specimen Codex is not probed at
all. That is not missing data; it is `requirements.md`'s first acceptance clause
visible on the default specimen. The variants that need Codex reporting carry
their own provider routes, and the popover footer reads the same object, so the
footer and the panel cannot claim different providers.

## The tab strip, which this topic changes

**Quota is the first tab, and it is the tab the panel opens on.** Those two are
one decision, not two: a tab placed first but not selected by default would use
position to say it matters most and the default to say it does not. The order is
quota, usage, breakdown, attribution, sessions.

The strip was a four-column grid. A fifth tab wrapped onto a second row, which
costs more vertical space than the labels are worth. The rule is now:

- **420 pt:** five equal columns, icon plus label, at 11 px with a 4 px icon gap.
  The previous 12 px / 5 px held four labels and **not** five: the gauge measured
  the English strip 14 px wider than its container — `over 14px (408/394)`.
  Tightening the type is forced by the fifth tab, not a restyle taken in passing.
- **280 pt:** icons only. The label moves to `aria-label` and `title`; reading
  order and the accessible name are unchanged.

Each label also carries `text-overflow: ellipsis`, so a future label longer than
its column truncates visibly and the gauge reports it, rather than pushing the
strip past the panel edge where nothing was watching.

Labels are dropped rather than truncated because a truncated tab label is worse
than none — `归` and `Attrib…` both read as a different word, while an icon with
an accessible name reads as itself. The warning dot each tab can carry is
unaffected at either width.

This is a change to a strip the `desktop-app` topic owns. It is in this topic's
scope only because adding the fifth tab forces it; no other tab behavior changes.

## Layout and hierarchy

One card per client, always stacked, never side by side — two cards abreast do
not survive 280 pt. Card order is Codex then Claude, matching the client order
used everywhere else in the popover.

**The number of window rows is not fixed.** The sampled `prolite` account
returned one window on its main limit plus two on a per-model limit; a Plus
account carries both a 5-hour and a 7-day window on the main limit, giving four.
Q2b is that specimen. A layout that assumed the sampled shape would be wrong for
the more common plan, so every window the client reports gets a row and the card
grows rather than the design picking a fixed count.

Within a card, top to bottom:

1. **Header** — client name, then the plan as an unobtrusive pill when the
   client reports one, then the source and read time right-aligned.
2. **Window rows** — one per reported window, each a label line
   (label / reset countdown / used share) over a bar.
3. **Reset allowance** — its own row under a rule, with its own heading, opening
   a side popover on hover when the client reports individual credits.
4. **Locally observed reset** — its own row, with its own heading.
5. **Missing fields** — one row each, carrying label, unavailable-or-not-
   applicable, and the reason.

The three reset concepts are structurally separated, not merely worded
differently. A window's natural reset is inside its window row because it
belongs to that window; the official allowance and the locally observed reset
are separate rows with separate headings because they are separate facts. They
are deliberately not merged into one `stat-grid` — a shared grid would imply
they are three readings of one thing.

Bar tone follows used share, on thresholds shared with the widget so a colour
means the same thing on both surfaces: under 75 % neutral, 75–90 % elevated,
90 % and over warning. The thresholds are the same numbers the alert defaults
use, so a bar turning warning and an alert firing are the same event.

## The reset-allowance detail

`剩 3 次` answers how many, not which. The vendor returns each credit with a
title, a status, a grant instant, and an expiry, and an expiry that has not been
surfaced is a credit the user can lose without warning. The allowance row is
therefore a side popover carrying one line per credit — title, status, then
`18天前获得 · 12d后到期`.

**It opens on hover, not on click.** This detail is something to glance at, not
an action to confirm; requiring a click charges every single viewing for a
decision the reader never has to make. Keyboard focus opens it on the same path,
so the panel does not become mouse-only, and Escape closes it.

**The trigger row, the path, and the popover are one continuous region.** This
is the part that is easy to get wrong, and this specimen got it wrong twice
before getting it right — both times in the same direction, and the second time
is the more instructive one.

The first version closed the popover the moment the pointer left the trigger
row, so a reader moving toward the detail lost it on the way and could never
reach the expiry dates the popover exists to show. A popover you cannot put the
pointer into is a tooltip, and a tooltip is the wrong shape for four fields per
credit.

The second version added a transparent 10 px extension of the popover's hover
area, because `10px` is the number in the CSS that separates the popover from
the panel. **That is not the gap the pointer actually crosses.** The popover is
positioned against the panel's outer edge, while the trigger row sits inside a
card, inside the panel's own padding. Measured on the specimen, the trigger's
right edge is at 754 and the popover's left edge at 789: a **35 px** run, of
which the extension covered the last ten. A reader who moved briskly still got
through; one who moved slowly, or paused halfway, still lost it. A fix that
works only above some unstated speed is not a fix, and the document had already
promised the path does not depend on how fast the reader moves.

**The region is therefore defined by the two real rectangles, not by a CSS
number.** While the popover is open, the pointer's position is compared against
the trigger's rect, the popover's rect, and the corridor between them — the
horizontal run separating the two, spanning their combined vertical extent so a
diagonal drift stays inside it. Inside any of the three, the popover stays open
with no timer running at all. This costs nothing in interception: it reads the
pointer position and paints no overlay, so it never takes a hover away from the
panel content underneath.

Two derived facts follow, and both matter more than the mechanism:

- **The gap's width is never assumed.** Card padding, panel padding, panel
  width and the open direction all move it. The corridor is computed from the
  rects at the moment of the move, so the same code is correct at 280 pt and
  420 pt, opening left or right.
- **Nothing can expire while the popover is being read.** Leaving the trigger
  still schedules a close, to absorb the beat between `mouseout` on the row and
  the next pointer event; any position inside the region cancels it. A close
  that fires during reading would be the original defect wearing a longer
  number.

The region closes when the pointer leaves all of it, when the pointer leaves the
window entirely, when Escape is pressed, when the panel scrolls, or when the
quota state or panel width changes — the last two because the popover is
positioned from a rect captured at open time, and a stale position is worse than
a closed popover.

**The side is chosen from the room actually available**, not fixed. The menu-bar
icon can sit anywhere along the screen's width, so a hardcoded right-side popover
would open off-screen for a user whose icon is near the right edge. The rule is:
open right when the panel's right edge plus the popover width and gap still fit
inside the viewport; otherwise open left when that fits; otherwise overlay the
panel's own right edge. Overlaying covers half the panel, which is worse than
opening beside it — and better than rendering off-screen where it cannot be read
at all.

The main panel never grows or moves, so window usage and the `剩 3 次` row stay
visible while the reader inspects the detail. The trigger carries
`aria-haspopup="dialog"` and `aria-expanded`; the popover is a labelled dialog.

When a client reports a count but no per-credit detail, the row keeps its shape
and simply does not open — the affordance is consistent, and whether it responds
is the only difference. The vendor's credit `id` and marketing `description` are
not displayed: the first is an account-scoped identifier, the second is copy this
product did not write.

The prototype adds an `锚点 / Anchor` stage control for exactly this rule. The
stage centres the panel by default, and when it is centred the room on the left
always equals the room on the right, so the left branch can never appear — a rule
the specimen cannot show is a rule nobody reviewed. Anchoring the panel left or
right makes all three outcomes visible.

## Countdown, not clock time

Reset instants render as time remaining — `5d3h后重置`, `resets in 2h40m` — not
as `Sep 15 at 8pm`. The useful question is how long until the window frees up.
A clock time makes the reader do the subtraction, and across a timezone or a DST
boundary they will do it wrong. The underlying value stays an instant; only the
presentation is relative.

## Copy

Every string is a key in the prototype's `src/i18n.js`, in both languages. The
table is the contract; the prototype is where they are rendered.

| Key | `zh-Hans` | `en-US` |
| --- | --- | --- |
| `tabs.quota` | 额度 | Quota |
| `tabQuestions.quota` | 还剩多少 | How much is left |
| `quota.title` | 订阅额度 | Subscription quota |
| `quota.plan` | 套餐 | Plan |
| `quota.windowMins(300)` | 5 小时窗 | 5h window |
| `quota.windowMins(10080)` | 7 天窗 | 7d window |
| `quota.resetsIn(eta)` | `{eta}后重置` | `resets in {eta}` |
| `quota.resetsNow` | 即将重置 | resetting now |
| `quota.allowance` | 官方重置次数 | Official resets |
| `quota.allowanceRemaining(n)` | 剩 {n} 次 | {n} left |
| `quota.allowanceTotalUnknown` | 总数未提供 | total not reported |
| `quota.creditStatus.available` | 可用 | Available |
| `quota.creditGranted(rel)` | `{rel}获得` | `granted {rel}` |
| `quota.creditExpires(eta)` | `{eta}后到期` | `expires in {eta}` |
| `quota.observedReset` | 本地观察到重置 | Locally observed reset |
| `quota.sources.codex_app_server` | Codex app-server | Codex app-server |
| `quota.sources.claude_statusline` | Claude 状态栏 | Claude status line |
| `quota.sources.claude_usage_prose` | claude /usage | claude /usage |
| `quota.observedAt(rel)` | `{rel}读取` | `read {rel}` |
| `quota.justRead` | 刚刚读取 | just read |
| `quota.stale` | 已过期 | Stale |
| `quota.notApplicable` | 不适用 | Not applicable |
| `quota.readingOff` | 未读取 | Not read |
| `quota.readingOffHint` | 在设置中开启「读取额度」后开始读取 | Turn on Read quota in Settings to start reading |
| `quota.attributionUnconfirmed` | 无法确认账号归属 | account attribution unconfirmed |
| `quota.unavailable` | 不可用 | Unavailable |
| `quota.noWindows` | 无可显示的额度窗口 | No quota window to show |
| `quota.alertsOff` | 提醒已关闭 | Alerts off |

The reason strings are a closed set. Free text is not permitted here, because
the surface must be able to render every reason in both languages:

| Reason | `zh-Hans` | `en-US` |
| --- | --- | --- |
| `not_reported` | 该客户端不提供此字段 | this client does not report it |
| `not_official` | 当前 provider 不是 official，不探测 | provider is not official; not probed |
| `never_probed` | 尚未成功探测 | never probed successfully |
| `probe_failed` | 探测失败 | probe failed |
| `parse_failed` | 输出格式无法识别 | output shape not recognized |
| `not_consented` | 状态栏通路未启用 | status-line route not enabled |
| `probe_disabled` | 读取额度已关闭 | quota reading is off |

**`不适用`, `不可用` and `未读取` are three words for three facts.** Not
applicable means the product deliberately did not ask, because the provider is
not `official`. Unavailable means it asked and the client did not answer. Not
read means the user turned reading off, so nothing was asked of anything.
Rendering the gate as "unavailable" would tell the user something is broken when
nothing is; rendering the user's own switch as either of the other two would
hide the fact that the fix is one setting away.

With reading off the card keeps its shape rather than disappearing — the client
name, `未读取`, and its reason. A tab that empties itself teaches the user the
feature is gone; a tab that says why teaches them where the switch is. Retained
observations are not rendered in this state, per `requirements.md` clause 2:
they exist so that turning the switch back on is instant, not so that a figure
survives the decision to stop reading it.

**Claude figures carry their attribution limitation on its own line, directly
under the header.** It is a property of every figure on that card, not of a
missing field, so it is neither a reason row nor a fourth missing-word: a
`.quota-missing` row would come with a rule above it and read as one more thing
the client failed to report, when nothing failed.

Appending it to the source line — `Claude 状态栏 · 2分钟前读取 · 无法确认账号归属`
— was the first form and was rejected on the measurement rather than on taste.
That line already carries source, age, and `已过期`; at 280 pt a fourth clause
pushes the age out of the visible run, and the age is the field the reader needs
most on a card whose whole problem is that its numbers may be old. A separate
10 px line costs one row and cannot displace anything.

Codex cards carry nothing equivalent, and that asymmetry is the point: the
statement is only true where it is true, so it stays informative instead of
becoming decoration a reader learns to skip.

The line appears on every Claude card that names a source — including the
parse-failure card, whose header still reports the route and the instant of the
failed attempt, and the stale card, where attribution and age are two separate
reasons to distrust the number rather than one. The single exception is the
reading-off card: nothing was read from any account, so there is no attribution
to qualify, and adding the line there would attach a caveat to a figure that
does not exist.

The English window labels are compact — `5h window`, not `5-hour window` — for a
measured reason, not a stylistic one: the gauge reported the long form short by
16 px at 420 pt and 19 px at 280 pt. Chinese was already compact.

## Narrow bound

At 280 pt, one rule beyond the tab strip: a per-limit window label sheds its
model name and keeps the window span. `GPT-5.3-Codex-Spark · 5 小时窗` was
measured 101 px short of its cell; the full label survives in `title` and the
accessible name. The model name is a qualifier, the window is the fact, and a
truncated model name reads as a different model.

Everything else is identical at both widths. The gauge reports no overflow and
no truncation inside this panel across all six variants in both languages at
both widths.

## Data requirements

What this surface asks `architecture.md` to provision. Each must be provisioned
or refused with a reason; a refusal is a real answer and this document will
adopt it.

| Requested | Why the surface needs it | Per client |
| --- | --- | --- |
| `applicable` plus the reason when false | Separates the gate from a failure | both |
| Window list: stable key, optional model label, window length, used share, reset instant | One row each; the length drives the label, the instant drives the countdown | both |
| `plan` or a reason | Header pill, or a missing-field row | Codex reports; Claude does not |
| Reset allowance remaining, plus whether a total exists | Its own row; the total's absence is displayed, not computed around | Codex only |
| Locally observed reset instant | Its own row; the third reset semantics | Codex only in the specimen |
| `source` identifying which route produced the figure | Header; the user can tell statusline from prose | both |
| `observed_at` and a `stale` flag | Header; freshness is never implied | both |
| A per-client `failure` reason from the closed set | Chooses the missing-field copy | both |
| `attribution_confirmed` | Header; false adds the attribution statement | both |

The surface asks for no account identifier, and it must not be given one.
Account isolation happens before the payload reaches it: Codex figures from
another account must not arrive, rather than arriving and being filtered here.
Claude has no identifier to isolate on, so the payload tells the surface that
much with `attribution_confirmed: false` and the surface says so. A boolean the
surface renders is checkable; a limitation each surface is trusted to remember
is not.

### Where the missing word comes from

The three words are not chosen at each render site. One function maps the reason
to the word, and every missing-field row goes through it:

```
not_official    → 不适用 / Not applicable
probe_disabled  → 未读取 / Not read
everything else → 不可用 / Unavailable
```

The alternative the specimen started with — a `notApplicable` boolean passed
down to each row — was three call sites deciding the same thing separately, and
three chances to pass the wrong one. The cost of passing it wrong is not a
cosmetic slip: it tells the user something is broken when nothing is, or hides a
switch they could flip. Deriving the word from the reason makes that class of
defect unrepresentable, and it is why `probe_disabled` needed no new flag.

## Rendered specimens

What a specimen settles, per `docs/documentation-workflow.md`: hierarchy, copy,
state coverage, and wrap or truncation at the narrow bound. It settles nothing
about real typography, Dynamic Type, VoiceOver order, or notification delivery —
those need the manual acceptance `tasks.md` assigns.

The sketches below are the same states in text, kept because this contract is
reviewed as text and cited by blob hash. Each names the prototype URL it
indexes; **they are an index to the prototype, not a substitute for it**, and
where the two disagree the prototype is right.

### Q1 — the gate, at 420 pt, `zh-Hans`

Indexes `?tab=quota&lang=zh&width=420&quota=normal`. Codex is on `aigocode`,
so it is not probed. Three rows say so, each with the same reason.

```
┌──────────────────────────────────────────────────────┐
│ ▣ 用量   ◑ 构成   ⛉ 归因   </> 会话   ◔ 额度        │
├──────────────────────────────────────────────────────┤
│ ┌──────────────────────────────────────────────────┐ │
│ │ Codex                                        —   │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 无可显示的额度窗口                        不适用 │ │
│ │ 当前 provider 不是 official，不探测              │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 官方重置次数                              不适用 │ │
│ │ 当前 provider 不是 official，不探测              │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 套餐                                      不适用 │ │
│ │ 当前 provider 不是 official，不探测              │ │
│ └──────────────────────────────────────────────────┘ │
│ ┌──────────────────────────────────────────────────┐ │
│ │ Claude              Claude 状态栏 · 2分钟前读取  │ │
│ │ 无法确认账号归属                                 │ │
│ │ 5 小时窗            3h20m后重置             22%  │ │
│ │ ████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │ │
│ │ 7 天窗              6d5h后重置             3.0%  │ │
│ │ █░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 官方重置次数                              不可用 │ │
│ │ 该客户端不提供此字段                             │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 套餐                                      不可用 │ │
│ │ 该客户端不提供此字段                             │ │
│ └──────────────────────────────────────────────────┘ │
│                      提醒已关闭                      │
└──────────────────────────────────────────────────────┘
```

Both cards carry a missing reset allowance, and they carry *different* words for
it: `不适用` for the client that was not asked, `不可用` for the client that was
asked and does not report it. Reading the two cards side by side is how a
reviewer checks that distinction survives.

### Q7 — quota reading turned off, at 420 pt, `zh-Hans`

Indexes `?tab=quota&lang=zh&width=420&quota=readingOff`. The switch is off, so
nothing was asked of either client. Both cards collapse to one row, because the
alternative — plan, allowance and windows each reporting `未读取` separately —
makes the reader read the same sentence three times per client to learn one
fact about the whole panel.

```
┌──────────────────────────────────────────────────────┐
│ ┌──────────────────────────────────────────────────┐ │
│ │ Codex                                          — │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 订阅额度                                  未读取 │ │
│ │ 读取额度已关闭                                   │ │
│ └──────────────────────────────────────────────────┘ │
│ ┌──────────────────────────────────────────────────┐ │
│ │ Claude                                         — │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 订阅额度                                  未读取 │ │
│ │ 读取额度已关闭                                   │ │
│ └──────────────────────────────────────────────────┘ │
│            在设置中开启「读取额度」后开始读取         │
└──────────────────────────────────────────────────────┘
```

The panel footer replaces `提醒已关闭` rather than adding to it. With reading
off no reminder can be evaluated anyway, so two lines would state one
consequence twice; the line that survives is the one that names the switch. It
appears once for the panel, not once per card — two copies read as two switches.

The source slot holds `—` rather than a stale route name. The card was not
produced by a route, and naming the last one would be the same defect as showing
the retained figure: presenting what *was* true as what is.

### Q2 — both `official`, at 420 pt, `zh-Hans`

Indexes `?tab=quota&lang=zh&width=420&quota=bothOfficial`. Every field Codex
reports is present, including the per-model limits and the reset allowance.

```
┌──────────────────────────────────────────────────────┐
│ ┌──────────────────────────────────────────────────┐ │
│ │ Codex  prolite     Codex app-server · 刚刚读取   │ │
│ │ 7 天窗              5d3h后重置              79%  │ │
│ │ ███████████████████████████████████░░░░░░░░░░░░░ │ │
│ │ GPT-5.3-Codex-Spark · 5 小时窗  2h40m后重置 12%  │ │
│ │ █████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │ │
│ │ GPT-5.3-Codex-Spark · 7 天窗    6d后重置   4.0%  │ │
│ │ █░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 官方重置次数                              剩 3 次│ │
│ │ 总数未提供                                       │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 本地观察到重置                             4天前 │ │
│ └──────────────────────────────────────────────────┘ │
│ ┌──────────────────────────────────────────────────┐ │
│ │ Claude              Claude 状态栏 · 2分钟前读取  │ │
│ │ 无法确认账号归属                                 │ │
│ │ … as in Q1 …                                     │ │
│ └──────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

The three reset facts are all on this one specimen and all differently labelled:
`5d3h后重置` inside a window row, `官方重置次数 剩 3 次` on its own row, and
`本地观察到重置 4天前` on another. `总数未提供` sits under the allowance rather
than being computed — the vendor lists only currently visible credits.

### Q4 — the prose route did not parse, at 420 pt

Indexes `?tab=quota&lang=zh&width=420&quota=parseFailed`. Codex is unaffected.
Claude emits no number at all.

```
│ ┌──────────────────────────────────────────────────┐ │
│ │ Claude               claude /usage · 30秒前读取  │ │
│ │ 无法确认账号归属                                 │ │
│ │ ──────────────────────────────────────────────── │ │
│ │ 无可显示的额度窗口                        不可用 │ │
│ │ 输出格式无法识别                                 │ │
│ └──────────────────────────────────────────────────┘ │
```

The read time is the instant of the *failed* attempt, and the previously good
percentages are gone rather than re-presented. A stale number under a fresh
timestamp is the failure this state exists to prevent.

### Q6 — never probed, and Q5 — stale

`?quota=neverProbed` renders `尚未成功探测` and no read time; `?quota=stale`
renders the last figures with `· 已过期` appended to the header and the real
observation age. These are three distinct presentations — never observed, failed
just now, and observed but too old — and no two of them share copy.

### The narrow bound

Indexes any of the above with `width=280`. The tab strip is icons only, and
`GPT-5.3-Codex-Spark · 5 小时窗` becomes `5 小时窗`. Nothing else moves.

```
┌────────────────────────────────┐
│  ▣    ◑    ⛉   </>   ◔        │
├────────────────────────────────┤
│ Codex prolite   刚刚读取       │
│ 7 天窗      5d3h后重置    79%  │
│ ██████████████████████░░░░░░░  │
│ 5 小时窗    2h40m后重置   12%  │
│ ███░░░░░░░░░░░░░░░░░░░░░░░░░░  │
└────────────────────────────────┘
```

## What the specimen does not settle

Handed to `tasks.md` as manual acceptance:

- Real SF typography metrics and Dynamic Type at accessibility sizes.
- VoiceOver order across five tabs, and whether an icon-only tab announces its
  accessible name at the narrow bound.
- Whether a bar's tone is distinguishable for a red-green colour-blind reader;
  tone is never the only carrier — the percentage is always present as text.
- Live appearance switching between light and dark while the panel is open.

## Verification

Prototype-level, run and recorded at design time rather than deferred:

| Check | Command | Result |
| --- | --- | --- |
| Overflow, horizontal overflow and truncation, eight quota states × two languages × two widths | `?quota=…&lang=…&width=…&measure=1` | `NO OVERFLOW`, 32/32 |
| The quota tab's interaction contract, asserted with real events | `?probe=1&lang=zh`, `?probe=1&lang=en` | 17/17 quota assertions pass in both languages |
| **Pausing in the gap** — the review's decisive counterexample, at its own coordinates | `?tab=quota&quota=codexPlus&lang=en&width=280`, real mouse to `(765,280)` | Open after 1 s and after 3 s |
| Pausing elsewhere on the run: `(772,289)`, `(758,300)`, `(786,270)` | same, real mouse | Open after 3 s at every point |
| Crossing slowly, nine steps of ~6 px at 400 ms each | same, real mouse | Open throughout, open after 2 s of reading, closes on leaving |
| The measured geometry the repair is built on | live rects at 280 pt | Trigger right `754`, popover left `789` — a `35 px` run, matching the review |
| The same slow path on all three placements | `anchor=left` (opens right), `anchor=right` (opens left), and a widened panel forcing `overlay` | Gap `35 px` on both sides; pausing in it keeps the popover open, leaving closes it, in all three |
| The popover opens right when the panel is anchored left | `?quota=codexPlus&anchor=left`, hover the allowance row | Opens right |
| The popover opens left when the panel is anchored right | `?quota=codexPlus&anchor=right`, hover the allowance row | Opens left |
| The popover overlays when neither side fits | 760 px viewport, panel centred | Overlays the panel's right edge |
| Prototype builds as the single-file specimen | `npm run build:single` | Passes |

### The interaction probe now covers this tab

The probe previously asserted nothing about quota, so every claim on this page
rested on a screenshot. Seventeen assertions were added, in the order a reader meets
them: the tab selects; the Claude card carries the attribution line and the
Codex card does not; hovering the allowance row opens the detail popover and
lists the credits; there really is a run of blank space to cross, and the
pointer may stop in the middle of it for half a second without losing the
popover; arriving inside keeps it open and populated; it closes only after the
pointer leaves the whole region; keyboard focus alone opens it and Escape
closes it; and in the reading-off state both cards
collapse to one row, the word is `未读取` rather than either of the other two,
no percentage is rendered anywhere in the panel, and the settings hint appears
exactly once.

Two defects in the probe itself were repaired to get there, and both are the
same kind of defect this document keeps finding — a tool that fails silently:

- **The probe had been crashing since the quota tab was added.** It addressed
  tabs by `nth-child`, and the quota tab took position 1, so it clicked quota
  while expecting usage, read an empty bar list, and threw on the next line. A
  thrown probe rendered nothing at all, which is indistinguishable from a probe
  that was never run. Tabs are now addressed by `data-tab`, and the probe
  renders `PROBE CRASHED after N checks` with the stack when it throws.
- **Its language lookup read `<html lang>` at entry**, before React had set it,
  so an `en` run silently compared against the Chinese copy table. It is read at
  assertion time now.

Five assertions still fail, and they are named rather than left for a reader to
find: one on the work-signal detail banner in the sessions tab, and four on the
settings window, which hardcode two switches and four preference controls and
were invalidated when the subscription-quota settings group added its own — and
deliberately disabled — dependent controls. Both sets belong to their own
surfaces: `ux/settings-quota.md` owns the four, and none is a claim about this
tab. This document does not repair them, and does not claim the probe is green.

**The pointer assertions move real coordinates, and that is the whole point.**
The first version of this regression dispatched "leave the trigger" and "enter
the popover" back to back, which is not a path — the pointer never occupied the
space between the two rectangles, so the assertions passed while a real mouse
resting in that space lost the popover. This is the more useful lesson of the
round: a synthetic event pair proves the handlers are wired, never that the
geometry is traversable. The regression now computes both rects at run time,
moves through the midpoint of the run between them, and waits there.

They were then validated the way this document validates any gauge: the corridor
test was backed out, the probe re-run, and it reported exactly the three
failures that describe the defect — pausing on the run, arriving inside, and
reading without it closing — while the surrounding assertions stayed green.
Then the repair was restored. A regression that has never been seen to fail is a
comment, not a test.

The overflow gauge runs once, ~600 ms after load, so it does not observe a
popover opened later by hover. The three placement rows above are therefore
verified by inspection at each anchor rather than by the gauge, and this
limitation is stated rather than left for a reader to discover as a silent gap.

The gauge itself was extended by this topic. It previously reported only widget
containers that scroll and elements with `text-overflow: ellipsis`, which meant
content that simply ran past the panel edge was invisible to it — that is how
the header defect below survived every earlier sweep, and how the English tab
strip read as fitting when it was 14 px too wide. The added check compares
`scrollWidth` against `clientWidth` for every element inside a panel, excluding
containers that are meant to scroll and elements hidden for screen readers only.
It was verified by disabling a repair, confirming the gauge reported it, and
re-enabling — a check that has never been seen to fail proves nothing.

## Findings repaired here

The gauge reported three defects at the narrow bound that this topic did not
introduce. On the operator's 2026-09-08 instruction they are repaired inside
this change rather than carried to a separate lane, because all three are the
same defect this topic already had to solve — at the narrow bound a qualifier
competes with the fact it qualifies, and the fact loses.

| Finding | Measured | Repair |
| --- | --- | --- |
| `MB-Q-F1` | The rhythm stat grid is four columns; at 280 pt each gets ~52 pt while `15–17 时` needs 54 — `short by 11px`, and `15–17h` short by 3 px | A stat grid denser than three columns reflows to two columns at the narrow bound. Nothing is dropped; the panel scrolls. Three-column grids are unaffected because they already fit. |
| `MB-Q-F2` | The provider footer `Codex aigocode · Claude official` short by 2 px at 280 pt in `en` | The `服务商` / `Providers` label column is dropped at the narrow bound, returning ~55 pt. The label becomes part of the button's `aria-label`, which now also names the providers. |
| `MB-Q-F3` | Found while repairing the first two: at 280 pt in `en` the header's refresh label runs past the panel's right edge by 42 px. Reproduces on `?tab=usage`, so it predates this topic. | The refresh button shows its icon alone at the narrow bound. Its `aria-label` already read `Refresh ⌘R`, so nothing is lost to a screen reader. |

All three reproduce on `?tab=usage&lang=en&width=280&measure=1` with no quota
involvement, which is how they were confirmed to predate this topic rather than
follow from it. All three are gone from that URL now.

`MB-Q-F3` is also why the gauge was extended: it was visible in a screenshot
while the gauge reported `NO OVERFLOW`, and a specimen tool that misses what a
screenshot shows will be trusted exactly until it matters.
