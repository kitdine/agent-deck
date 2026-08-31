---
status: active
created: 2026-08-20
updated: 2026-08-31
---

# Sessions Panel — Work Signals

The menu-bar `Sessions` panel in its **captured** state.

The surface's geometry, filters, state model, and card chrome belong to
[`../../desktop-app/ux/menubar.md`](../../desktop-app/ux/menubar.md) and are not
restated here. This document covers only what changes when the three modules
have values, and it derives from the prototype at
[`/prototype/`](../../../../prototype/) — open `?surface=` and select the
`会话` / `Sessions` tab.

Where this document and the prototype disagree, the prototype is right.

## The two levels

**Level one** is the three summary cards. Their layout is the one `desktop-app`
task 3 `menubar-experience` builds for the uncaptured state — that task is not
delivered yet, so this is a layout this document inherits from a sibling topic's
contract rather than from shipped code. Each card carries one figure:

| Card | Summary figure |
| --- | --- |
| Activity | The leading category and its share — `编码 52%` / `Coding 52%` |
| Workflow | First edit, median — `首次编辑 2 分钟` / `First edit 2m` |
| Tooling | Total calls — `82 工具调用` / `82 Tool calls` |

**Level two** is one detail view per card, entered by clicking the card and left
by the `返回` / `Back` control, which returns focus to the card that opened it.

## Activity detail

Four rows, always four, in the fixed order coding → debugging → conversation →
delegation. Each row carries a share bar, the attributed cost, and the event
count, and each row expands to reveal its subcategories.

Subcategories are the second level of this view and not a third view: clicking a
row rotates its caret and inserts an indented block beneath it, ruled on the
left. Only one row is expanded at a time.

The subcategory rows carry share and cost, not the bar. A bar nested under a bar
reads as a progress indicator rather than a proportion at 280 pt, which is why
the prototype indents and drops it.

Four rows is a constraint, not a preference. Eleven peer rows do not read at this
width, which is why the categories stay at four and the detail lives one level
down. This is why Decision 3 requires every category to have at least two
subcategories: a single indented row under a parent reads as a rendering fault.

The four shares sum to 100%. Each parent's subcategory shares sum to that
parent's share.

## Workflow detail

A four-cell metric grid, then one inline row.

| Cell | Value | Note line |
| --- | --- | --- |
| 首次编辑 / First edit | duration | 中位数 / median |
| 触及文件 / Files touched | count | — |
| 返工 / Rework | count | 改后验证再改 / edit, verify, edit again |
| 每会话编辑 / Edits per session | count | — |

The inline row is 最常改动 / Most touched: a base name and its edit count,
`tasks.md ×4`.

The rework cell carries a note line because the word alone does not say what was
counted, and a number whose definition is invisible gets read as whatever the
reader assumes. The note is the shortest true statement of Decision 6's rule.

Only the base name is shown, because only the base name exists — Decision 2
persists no directory. Two files with the same name in different directories are
distinct rows in the store and can therefore appear as the same label here; that
is a consequence of the privacy boundary and is accepted rather than papered
over.

## Tooling detail

One row per tool kind, ordered by call count descending, each carrying the call
count and that kind's share of calls. `other`, when non-empty, is a row like any
other and always sorts last regardless of count, because it is a residual rather
than a peer.

Then one inline row: 主要 MCP 服务 / Top MCP server, with its call count.

**No cost column.** The prototype showed one and was changed on 2026-08-20:
a tool call consumes no tokens, so any figure there would be an apportionment of
the turn's cost displayed in the position where this product displays measured
amounts.

## States

The names below are the **prototype's stage states** —
`normal`, `empty`, `aged`, `partial`, `unavailable`. They are not
`menubar.md`'s state model: that
document replaced a list of names with three surfaces — `loadingSurface`,
`dataSurface`, `errorSurface` — plus orthogonal qualifiers. The mapping between
the two belongs to `menubar.md`; what this table fixes is what the three modules
render at each specimen state, which is what the prototype can be checked
against.

| Stage state | Activity / Workflow / Tooling |
| --- | --- |
| `normal` | Captured rendering, as above |
| `empty` (no spend today) | The panel's existing empty note; the signal cards are not rendered |
| `aged` | Captured rendering, unchanged. Staleness is the panel's own banner and says nothing about these three modules |
| `partial` | Captured rendering, unchanged. See below |
| `unavailable` | The uncaptured rendering: the `待采集` / `Not captured yet` flag on the signal heading, and a banner inside each detail view |

### `partial` is not rendered on the panel

Decision 4 produces three `cost_basis` values. `turn` needs no marker, `none`
renders as unavailable, and **`partial` gets no panel treatment at all**: the
figures render exactly as at `turn`. Only `--format json`'s `cost_basis` field
distinguishes them, which is the reader that needs to.

A marker was drafted and rejected. The panel is 280 pt wide, the shortfall is not
knowable per row, and a caveat line under four rows buys a precision the reader
cannot act on.

The uncaptured rendering at `unavailable` is **retained, not deleted**. It is
what an older snapshot decodes to, and after this topic ships it stays reachable
— a host carrying the new decoding still meets payloads that predate it.

Within the captured rendering, a single unavailable figure prints as `—`. It is
never a zero. A zero means measured-and-none; `—` means the scope produced
nothing to measure. Both surfaces follow the same rule.

A subcategory with no signal for the selected client — `delegation/workflow`
under Codex, per Decision 3 — is **omitted from the expanded list**, not shown as
a zero row. Its parent row still renders.

## Copy

Both languages ship together. The table below is the **design delta against the
approved prototype dictionary**: these are the strings introduced or changed by
the work-signals design itself, and only these belong to that delta.

The shipping macOS catalog also imports the prototype's already-approved labels
needed to render this surface — Activity category names, Workflow metric labels,
Tooling labels, `Back`, and the legacy pending hint. Those are verbatim ports of
existing prototype copy rather than new product wording, so this table is not an
exhaustive diff of every key newly entering `DesktopCopy`.

| Key | 中文 | English |
| --- | --- | --- |
| `sessions.retries` | 返工 | Rework |
| `sessions.retriesNote` | 改后验证再改 | edit, verify, edit again |
| `sessions.toolKinds.other` | 其他 | Other |
| `sessions.subKinds.feature` | 新增 | Feature |
| `sessions.subKinds.refactoring` | 重构 | Refactoring |
| `sessions.subKinds.testing` | 测试 | Testing |
| `sessions.subKinds.maintenance` | 维护 | Maintenance |
| `sessions.subKinds.investigation` | 定位 | Investigation |
| `sessions.subKinds.repair` | 修复 | Repair |
| `sessions.subKinds.exploration` | 查阅 | Exploration |
| `sessions.subKinds.brainstorming` | 构思 | Brainstorming |
| `sessions.subKinds.planning` | 计划 | Planning |
| `sessions.subKinds.subagent` | 子 Agent | Subagent |
| `sessions.subKinds.workflow` | 技能流程 | Skill / workflow |

Removed: `sessions.iterationDepth`, `sessions.turnsPerEdit`, and
`sessions.shareOfCost` — the first two with the metric they named, the third with
the Tooling cost column.

Deliberately **not** added: `sessions.shareOfCalls` and `sessions.times`. The
Tooling row prints a bare percentage and the most-touched row prints `tasks.md ×4`
with a hardcoded `×` in both languages. Neither needs a string, and listing one
that nothing renders is how a catalog grows keys no surface uses.

`sessions.toolKinds.other` is required rather than optional: Decision 7 renders
`other` as its own row when non-empty, and on Codex it routinely is. Without the
key that row renders as a missing string in both languages.

## Accessibility

- The four Activity rows are buttons with `aria-expanded`, and the expanded
  subcategory block follows its parent in DOM order, so VoiceOver reads parent
  then children without a jump.
- Entering a detail view moves focus to its heading; `返回` / `Back` returns
  focus to the card that opened it.
- Share is never carried by the bar alone. Every bar has its percentage as text
  beside it, and every colour-coded row has a text label.

Acceptance on real macOS 26 covers both appearances, both languages, the 280 pt
narrow bound, native expanded-state structure, textual alternatives, and the
detail-navigation return target through synthetic state and rendered/XCTest
evidence. Actual VoiceOver speech, TCC changes, or system accessibility-setting
automation are deliberately **not run and are not completion requirements**.
The acceptance record names that non-execution explicitly rather than implying
those system-level checks passed.
