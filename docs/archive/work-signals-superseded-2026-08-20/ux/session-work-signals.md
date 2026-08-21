---
status: active
created: 2026-08-20
updated: 2026-08-20
---

# Work Signals — Sessions panel, captured state

## What this document is, and is not

The **surface** is the menu-bar `Sessions` panel, and it belongs to
[`../../desktop-app/ux/menubar.md`](../../desktop-app/ux/menubar.md): its
geometry, its filters, its state model, its qualifier ordering, and its
`Not captured yet` treatment are specified there and are not restated or
re-decided here.

This document specifies the one thing that document cannot: what the three
modules look like **once values exist**. It is the captured-state half of the
same surface.

The design source is the reviewable prototype at
[`../../desktop-app/ux/prototype/interactive-v7/`](../../desktop-app/ux/prototype/interactive-v7/) —
`src/Popover.jsx` for structure and `src/i18n.js` for copy. Every element and
every string below is taken from it, **except the three additions the Copy
section names** — the attributed-cost note, the kind-definition help, and the
period-neutral share-of-cost label. Those three are stated there as additions
precisely so this sentence stays checkable: everything else can be found in
`src/i18n.js`. The prototype's sample values under
`PENDING_CAPTURE` in `src/data.js` are demonstration data that define the
**field shape**; they are never copied into the app.

## The two levels

The panel shows three summary rows. Selecting one opens a detail view for that
module with a `Back` control, exactly as the prototype does. The detail view
replaces the panel content; it does not overlay it and does not change the
popover's height.

### Summary rows

| Module | Summary line, captured | Source field |
| --- | --- | --- |
| Activity | The highest-share kind and its share — `Coding 58%` | `activity.items[]`, the record with the largest `share` |
| Workflow | `First edit` and its median duration — `First edit 2m` | `workflow.items[].first_edit_seconds` |
| Tooling | The call count and its unit — `82 Tool calls` | `tooling.items[].calls` |

### Activity detail

Four rows, always all four kinds, in fixed `coding` → `debugging` →
`conversation` → `delegation` order — not sorted by value, so the rows do not
reorder between periods. Each row carries its label, a proportional bar at
`share`, and a value of `<cost> · <events>`. The four bars use the surface's
existing four-colour series palette in that same fixed order, so a kind keeps
one colour across every period and client scope.

A kind with no work renders at `share: 0` with a zero cost and a zero event
count. It is not hidden: the absence of debugging in a period is itself the
information.

### Workflow detail

A four-cell metric grid followed by one inline row:

| Cell | Label `en` | Label `zh-Hans` | Note under the value |
| --- | --- | --- | --- |
| 1 | `First edit` | `首次编辑` | `median` / `中位数` |
| 2 | `Files touched` | `触及文件` | — |
| 3 | `Iteration depth` | `迭代深度` | `turns / edit` / `轮次/编辑` |
| 4 | `Edits / session` | `每会话编辑` | — |

Inline row: `Most touched` / `最常改动`, valued `<base name> ×<count>`. The name
is a bare file name and never a path — that is a storage guarantee, decided in
[`../architecture.md`](../architecture.md), not a truncation performed here.

### Tooling detail

A row per tool kind — at most four, in fixed `bash` → `read` → `edit` → `mcp`
order — each carrying its label, `<calls> Tool calls`, and its cost. Followed by
three inline rows:

| Inline row | Value | Source field |
| --- | --- | --- |
| `tool groups` / `工具组` | `<groups>` | `tooling.items[].groups` |
| `Share of cost` / `占成本` | `<share_of_cost>%` | `tooling.items[].share_of_cost` |
| `Top MCP server` / `主要 MCP 服务` | `<server> · <calls>` | `tooling.items[].top_mcp_server`, `top_mcp_calls` |

`groups` and `share_of_cost` are carried by
[`../architecture.md`](../architecture.md) Decision 5 and rendered by
[`cli-work-signals.md`](cli-work-signals.md); a field the projection computes and
neither surface shows is a field that should not be on the wire, so the panel
shows them rather than dropping them.

The `en` label deviates from the prototype's `of today's cost` deliberately. That
string predates the panel's period filter, and under a `30d` scope it would
assert "today". The panel's filters are the two the surface already has, so the
label must be period-neutral; `zh-Hans` drops `今日` for the same reason.

A tool kind with no calls in the scope is **omitted** here, unlike Activity's
kinds. The difference is deliberate and it is a claim about meaning: the four
activity kinds partition all work, so a zero is a fact about the period, while
the tool kinds are a grouping of what happened to be called, so an absent kind
is nothing at all.

## Copy

Taken from the prototype's catalogues. `en` and `zh-Hans` ship together; neither
is a fallback for the other.

| Element | `en` | `zh-Hans` |
| --- | --- | --- |
| Module titles | `Activity` / `Workflow` / `Tooling` | `活动` / `工作流` / `工具` |
| Back control | `Back` | `返回` |
| Activity kinds | `Coding` / `Debugging` / `Conversation` / `Delegation` | `编码` / `调试` / `对话` / `委派` |
| Tool kinds | `Bash` / `Read` / `Edit` / `MCP` | `Bash` / `读取` / `编辑` / `MCP` |
| Tool-call unit | `Tool calls` | `工具调用` |
| Workflow labels | `First edit` / `Files touched` / `Iteration depth` / `Edits / session` | `首次编辑` / `触及文件` / `迭代深度` / `每会话编辑` |
| Workflow notes | `median` / `turns / edit` | `中位数` / `轮次/编辑` |
| Most-touched row | `Most touched` | `最常改动` |
| Top MCP row | `Top MCP server` | `主要 MCP 服务` |
| Pending banner, uncaptured state | `These fields are not in the snapshot yet` | `当前快照没有这些字段，需要新增采集` |
| Attributed cost note | `Cost attributed by session` | `成本按会话摊分` |
| Kind definition help | `Work that produced no tool call counts as Conversation` | `未产生工具调用的工作计为对话` |
| Tool-groups row | `tool groups` | `工具组` |
| Share-of-cost row | `Share of cost` | `占成本` |

This topic adds **three** strings to the prototype's set: the attributed-cost
note, the kind-definition help, and the share-of-cost label. Every other row
above already exists in the prototype's catalogues — the pending banner and the
tool-groups label included; both are listed here because they are retained, not
because they are new. The share-of-cost label is a deliberate deviation rather
than a new concept: the prototype's `of today's cost` / `占今日成本` predates the
panel's period filter and would assert "today" under a `30d` scope, so the
period-bound half is dropped. See **Tooling detail** for why.

The kind-definition help states the rule without naming the unit it is evaluated
over, because the surface does not need the unit to state the rule.
`architecture.md` Decision 2 classifies over a **turn in both clients** — one
object with two boundary markers — so naming it would also be correct; the
neutral wording is a choice, not a workaround. An earlier draft of this paragraph
justified it by a difference between the clients that Decision 2 no longer has,
and required this surface not to ship until the architecture resolved it; that
resolution landed, and `iteration_depth`'s `turns / edit` is now positively
correct for both clients rather than tolerated.

## Which state is shown when

The panel's existing `unavailable` / `empty` / `partial` rules govern; these are
the module-specific bindings:

| Condition | Rendering |
| --- | --- |
| The wire family is absent or `available: false` | The prototype's pending banner, unchanged. This is the state `menubar-experience` ships today and it stays reachable — an older snapshot decodes to exactly this |
| The family is available but the scope has no sessions | The panel's existing empty treatment, and its qualifier-aware wording. Not zeros |
| The family is available and `cost_basis` is `none` | Values render; every cost renders as unavailable, never as `$0.00` |
| `cost_basis` is `session` for any contributing record | The attributed-cost note is shown beside the cost column. A figure mixing bases reports the weaker one |
| A single Workflow value is null | That cell alone renders unavailable; the other three still render |

The pending banner and the attributed-cost note never appear together: the first
says there is nothing, the second qualifies something.

## Accessibility

Inherits the surface's rules in `../../desktop-app/ux/menubar.md` and adds
nothing new in kind. Three bindings specific to these modules:

- Each Activity bar has an accessible value stating kind, share, cost, and event
  count. The colour is never the only carrier of the kind — the label is always
  present beside it.
- The summary rows are buttons with an accessible label naming the module they
  open, not just their summary value.
- The `Back` control is the first element in the detail view's focus order, and
  dismissing the detail returns focus to the summary row that opened it.
