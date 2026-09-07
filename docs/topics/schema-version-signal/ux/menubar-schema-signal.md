---
status: active
created: 2026-09-06
updated: 2026-09-06
---

# Menu-Bar Schema Signal — Surface

This is stage 3 of the progression in `docs/documentation-workflow.md`: the
surface framework and its data requirements list. Everything under **Data
requirements** is a request to `architecture.md`, which provisions each field or
refuses it with a stated ground. Stage 7 returns here to absorb any refusal.

The boundary this surface serves is
[`requirements.md`](../requirements.md), whose acceptance item 6 reads: *the
menu-bar surfaces attribute their unavailability to this condition rather than
presenting bare unavailability, to whatever extent the contract stage
provisions.* This document decides what "attribute" means for each element, and
resolves the topic's third open question — whether the widget extension presents
the condition.

Source coordinates are line numbers at `8d283cd`, each stated with the symbol
that owns it.

## Where the specimens come from

The specimen for this condition is the prototype at
[`/prototype/`](../../../../prototype/), which
[`prototype/README.md`](../../../../prototype/README.md) declares the single
design truth for every product surface. This document does not carry a second
one. **Where this document and the prototype disagree, the prototype is right.**

```bash
cd prototype && npm install && npm run dev -- --port 4175
```

| What | URL |
| --- | --- |
| S1 — the condition alone, 420 pt | `?state=schema&lang=zh&width=420` |
| S1 at the narrow bound | `?state=schema&lang=zh&width=280` |
| S1 in `en` | `?state=schema&lang=en&width=420` |
| S2 + S4 — the condition stacked with other problems | `?state=schemaStacked&lang=zh&width=420` |
| Every degraded state side by side | `?surface=states` |
| The overflow and truncation gauge | append `&measure=1` |

`state=schema` and `state=schemaStacked` are this topic's additions to the
prototype's state list, and the seven strings in **Copy** below are the same
seven keys in the prototype's `src/i18n.js` dictionary, in both languages. The
`width` switch is also this topic's addition: 420 pt is the panel's normal
width and 280 pt is the narrow bound the implementation exposes as
`AGENTDECK_TEST_WIDTH=280`. It exists so that "does this line survive the narrow
bound" is a question the specimen stage answers rather than one deferred to a
real render.

## Purpose

One permanent condition — the core database's schema version exceeds the version
this binary supports — currently reaches the menu bar as six unexplained
symptoms and no cause. The reported instance is exactly that: `database 失败`,
four warning-badged tabs, `服务商不可用`, and nothing naming what happened or
what to do.

This surface makes the cause the primary thing the user reads, once, and makes
every symptom that follows from it stop competing with it.

## Scope

In scope, all in the menu-bar app target (`apps/macos/AgentDeckApp`):

- the notice strip (`NoticeStripView`, `MenuBarSurfaceView.swift:414-465`);
- the Health detail rows (`HealthDetailView` / `HealthCheckRowView`,
  `MenuBarSurfaceView.swift:467-571`);
- the four panel tab marks and the panel bodies' unavailable row
  (`panelTabs` / `panelHasUnavailableData`, `MenuBarViewModel.swift:452-477`;
  `UnavailableRow`, `MenuBarPanelViews.swift:5-13`);
- the footer route text and the provider popover
  (`FooterView`, `MenuBarSurfaceView.swift:573-608`; `footer`,
  `MenuBarViewModel.swift:1148-1155`);
- the menu-bar item glyph and its accessibility label
  (`menuBarBadged` / `menuBarAccessibilityLabel`,
  `MenuBarViewModel.swift:1315-1321`);
- a decision, with its ground, on the WidgetKit extension;
- the specimen for all of the above in `prototype/`, which this document indexes
  rather than duplicates.

Out of scope: the CLI's rendering of this condition, which
`requirements.md` and `docs/specs/cli-design.md` own; every menu-bar state that
is not this condition; the settings window; and any change to panel layout,
geometry, typography, or palette. This surface adds **no new geometry and no new
visual vocabulary** — it re-uses the notice row, the expandable health row, the
existing tab mark, and the existing menu-bar badge.

## Measured starting state

Reproduced 2026-09-06 against a throwaway state directory whose
`schema_metadata.version` was set to `999`, using a build of `8d283cd`:

```
$ agentdeck --state-dir <tmp> desktop snapshot --format json
partial:  true
warnings: ["provider_unavailable", "sessions_unavailable", "usage_unavailable"]
health:   {"available": true, "status": "unhealthy", "healthy": false,
           "problems": 1, "warnings": 0, "errors": 1,
           "checks": [{"name": "state_permissions", "status": "ok"},
                      {"name": "state_lock",        "status": "ok"},
                      {"name": "database", "status": "error",
                       "code": "unknown_schema"}]}
```

On the reporter's machine `sessions.sqlite3` was a separate, readable store, so
`sessions_unavailable` was absent and the other two warnings were present; both
variants are specified below.

Three facts about the current app follow from that payload and from the code,
and they are what the design has to work with:

1. **The cause is in the payload and nothing renders it.** `healthDetail`
   (`MenuBarViewModel.swift:1262-1274`) maps each check to `name`, a localized
   `status`, a severity, and `recovery_command`. It never reads `code`. So
   `unknown_schema` reaches the app and dies there, and the row shows the raw
   check name beside `失败`.
2. **The two version numbers are not in the payload at all.**
   `DesktopHealthCheckV1` (`DesktopWire.swift:941-953`) carries one `count`, and
   this check sets none. The row could not state the numbers even if it read
   them.
3. **The symptoms outnumber and outrank the cause.** The strip renders, in
   order, `partial`, the health count notice, and up to three warning rows
   (`notices`, `MenuBarViewModel.swift:376-411`) — four to five rows, all true,
   none causal. Meanwhile all four tabs mark (`activeScope` is nil, so
   `panelHasUnavailableData` is true for every panel) and the footer falls to
   `footerProviderUnavailable` because `provider.routes` is empty.

## Decisions

**D1 — The cause gets one row, at the top, and it is an error.** A single notice
states the condition and opens Health. Ground: the condition is one fact about
the binary, not one fact per domain; the user needs to read it once.

**D2 — Its consequences stop being separate notices.** While the condition
holds, the strip suppresses the `partial` notice and the
`provider_unavailable`, `provider_candidates_unavailable`, `usage_unavailable`,
and `sessions_unavailable` warning rows. Ground: each states a symptom the
primary notice already explains, and a strip that lists five true statements
about one cause is the defect this topic exists to fix. Warnings that are *not*
consequences — `state_close_failed`, `sessions_close_failed`, an unrecognized
code — still render, because they are independent facts.

The trade this accepts, stated so a reviewer can reject it: if provider state
were *also* unreadable for an unrelated reason at the same moment, that reason
is hidden. It is hidden today too — the warning code carries no reason — and
while the condition holds no provider read can succeed anyway, so there is no
independent fact to act on.

**D3 — The health count notice is not suppressed, only outranked.** When the
schema check is the only problem (`health.problems == 1`), the primary notice
replaces the count notice, which would say "1 项检查未通过" about the row the
primary notice already names. When `problems > 1`, both render, the primary
notice first. Ground: the count is real information exactly when there is
something else to count.

**D4 — The Health row states the two numbers and the recovery in prose.** The
`database` row expands, like a row with a recovery command does today, into a
cause line carrying the stored and supported versions and a recovery line naming
the upgrade.

**D5 — The recovery is not rendered as a copyable command.** `HealthCheckRowView`
renders `recovery_command` monospaced beside a *Copy recovery command* button
(`MenuBarSurfaceView.swift:555-570`). This condition has no command to run —
`schema_outdated`'s `agentdeck state migrate` is a command; "upgrade AgentDeck"
is an instruction — so the row uses the prose treatment and no copy button.
Ground: a copy button that yields an English sentence in a Chinese UI is worse
than no button, and offering one implies a command exists.

This is also why the recovery prose is **app-side copy keyed off the stable
code**, not a wire string: it must exist in both shipped languages, and
`recovery_command` is a shell string that is never translated.

**D6 — The tab marks stay; the panel bodies gain the reason.** No fifth badge
state and no change to `panelHasUnavailableData`: the data really is
unavailable, and the mark is correct. What changes is the body copy behind the
mark, from a bare `不可用` to an attributed one. Ground: the mark is a pointer,
not an explanation, and four marks that each explain themselves would restate
the primary notice four times.

**D7 — The menu-bar item is badged.** While the condition holds, the item shows
the existing `exclamationmark.triangle.fill` badge (`MenuBarItemController.swift:76`)
and its accessibility label names the condition. Ground: the item is the only
surface visible without opening anything, and today the condition leaves it
showing an empty title — `menuBarText` returns nil when no subtotal and no
totals exist (`MenuBarViewModel.swift:1296-1313`) — which is byte-identical to
the *Icon only* preference. A permanent condition that is indistinguishable from
a preference is the silent-failure shape this topic exists to remove. This
re-uses the badge that already exists; it adds no third glyph.

**The predicate belongs to the App layer.** `menuBarBadged`
(`MenuBarViewModel.swift:1315`) gains this condition; `presentation.isBadged`
(`AgentDeckShared/EmbeddedHelperRunner.swift:861-863`) does not change. The two
are not the same question: `isBadged` is
`qualifiers.contains(.offline) || qualifiers.contains(.failing)` — a statement
about the helper's *refresh state*, which under this condition is entirely
healthy. This condition is a statement about the snapshot's *content*
(`health.checks[].code`). Pushing content semantics into a refresh-state
property would change the answer `isBadged` gives every other consumer of the
Shared type, to say something that property does not mean. The cost is that
Shared and App no longer agree on "is it badged", and that is the correct
split: they are answering different questions, and only the App layer reads
snapshot content. `architecture.md` may overrule this if it provisions the
condition somewhere the Shared layer already sees.

**D8 — The widget extension does not present the condition.** It keeps its
existing `Data unavailable` state. Grounds, in order of weight:

1. It cannot see the condition. `WidgetDesktopSnapshotV1`
   (`WidgetSnapshot.swift:3-19`) decodes `schema_version`, `generated_at`,
   `next_refresh_at`, `partial`, and `usage` — no `health`, no `provider`.
   Presenting the cause means extending the widget projection to carry health,
   which is a widget-contract change this topic does not need.
2. It is not silent today. `UnavailableWidget` (`WidgetViews.swift:1272-1285`)
   already renders `Data unavailable` / `数据不可用`, so nothing false is shown
   — what is missing is the reason.
3. It cannot carry the reason usefully. A widget has no room for a two-number
   cause plus a recovery sentence, cannot open the Health detail, and cannot be
   the place a user acts.

This is a deliberate narrowing of acceptance item 6 to the menu-bar surfaces,
with the ground stated, as item 6 permits. `tasks.md` therefore gains **no**
widget row. Revisit if the widget projection ever carries health for another
reason.

## Presentation states

| ID | State | Rule |
| --- | --- | --- |
| S1 | Condition present, snapshot current | The full treatment below: primary notice, expanded Health row, attributed panel bodies and footer, badged item |
| S2 | Condition present **and** helper unreachable or failing | The existing `offline` / `failing` notice ranks first and the primary notice follows it. Ground: a snapshot we could not refresh cannot assert the condition still holds, only that it held when the snapshot was taken |
| S3 | Condition present, snapshot aged or stale | Unchanged freshness treatment (`freshnessText`, `MenuBarViewModel.swift:336-342`). The primary notice makes no claim of liveness beyond the snapshot's own `generated_at` |
| S4 | Condition present with other problems (`health.problems > 1`) | Primary notice first, health count notice second, per D3 |
| S5 | Condition cleared | Nothing is sticky. The next snapshot without the check restores every element to its normal state with no residue, no dismissal record, and no "recently recovered" copy |
| S6 | Reverse condition (`schema_outdated`) | Unchanged: a `warning` row with `count` and the copyable `agentdeck state migrate`. The two conditions must never be presented alike; naming the wrong recovery is worse than naming none |

## Presentation rules

### Notice strip

Order while the condition holds:

1. `offline` or `failing`, if present (S2);
2. **the schema signal** — error severity, `opensHealthDetail: true`, chevron;
3. the health count notice, only when `health.problems > 1` (D3);
4. warnings that are not consequences of the condition, bounded at three plus
   the existing `noticeMore` row.

Suppressed while the condition holds: the `partial` notice and the four
consequence warning codes (D2). Suppression is a presentation rule only — the
snapshot still carries `partial: true` and every warning code, and the CLI still
reports them.

The primary notice re-uses `noticeRow` unchanged: severity symbol, caption text
that wraps (`fixedSize(horizontal: false, vertical: true)`), chevron, tinted
14 %-opacity fill with a 55 %-opacity stroke, `rowMinimumHeight` 28 pt.

### Health detail row

The row for this condition renders in the existing expandable shape, expanded by
default like every recovery row today (`isExpanded = true`,
`MenuBarSurfaceView.swift:509`):

- **line 1** — the check name and the localized `Failed`, unchanged;
- **line 2** — the cause: the stored version and the supported version;
- **line 3** — the recovery, in prose, with no monospace and no copy button (D5).

Lines 2 and 3 sit in the existing indented disclosure area
(`padding(.leading, rowMinimumHeight)`). Every other check row is untouched.

### Panel tab marks and panel bodies

Marks unchanged. Each unavailable panel body renders the attributed variant of
`sectionUnavailable` while the condition holds; every other unavailable cause
keeps today's copy.

### Footer and provider popover

- Footer route text: the attributed short form, still one line with middle
  truncation, still preceded by the `Providers` caption.
- Provider popover: `switchingUnavailable` is replaced by the attributed
  sentence, which has room to be a full sentence because the popover is 250 pt
  wide and wraps.

### Menu-bar item

Badged glyph, per D7, plus an accessibility label naming the condition. The
title continues to render whatever `menuBarText` yields, which under this
condition is empty.

## Copy

Both shipped languages, `en` and `zh-Hans`. Every key below joins
`DesktopCopy.allKeys` (`DesktopCopy.swift:217-277`), because that inventory is
what `DesktopCopyTests.testEveryKeyResolvesInBothShippedLanguages` walks — a key
outside it is a key nobody proves resolves.

| Key | `en` (the key itself) | `zh-Hans` |
| --- | --- | --- |
| `schemaSignalNotice` | This AgentDeck is older than its database, so local data cannot be read | 此 AgentDeck 比其数据库旧，无法读取本地数据 |
| `schemaSignalCause` | Database version %1$lld · this app supports %2$lld | 数据库版本 %1$lld · 此应用支持 %2$lld |
| `schemaSignalRecovery` | Upgrade AgentDeck to open this database | 升级 AgentDeck 后才能打开该数据库 |
| `schemaSignalSectionUnavailable` | Unavailable · AgentDeck is older than its database | 不可用 · AgentDeck 比其数据库旧 |
| `schemaSignalFooter` | Unavailable · app is older | 不可用 · 应用版本过旧 |
| `schemaSignalSwitchUnavailable` | Switching is unavailable while AgentDeck is older than its database | AgentDeck 比其数据库旧，暂时无法切换服务商 |
| `badgedSchemaSignal` | AgentDeck — older than its database | AgentDeck — 版本比数据库旧 |

These same seven keys, under the same names and with the same values in both
languages, are in the prototype's `src/i18n.js` dictionary, which is where they
were rendered and read. Changing one without the other reintroduces the second
design truth this document exists inside a project that has already refused.

**The copy is constrained by a shipped test, not only by taste.**
`DesktopCopyTests.testNoStringOffersAnUpdateCheck` fails any shipped string
containing `check for update`, `checking for update`, `latest version`,
`new version`, `release page`, `download page`, `检查更新`, `更新检查`,
`新版本`, `最新版本`, or `下载页`, because `v0.5.0` withdrew the update check
and no string may offer one. Naming the recovery is not performing it —
`requirements.md`'s non-goal says so explicitly — so the recovery copy above
says *upgrade AgentDeck* and never *download the new version*. The seven strings
are written to pass that assertion unchanged; it is a constraint on the wording,
not an argument against the recovery.

`%1$lld` and `%2$lld` are positional because the two numbers read in the
opposite order in neither language today, but a future translation may reorder
them; `t(_:_:)` formats through `String(format:locale:arguments:)`
(`DesktopCopy.swift:315-317`), which honours positional specifiers.

## Rendered specimens

What a specimen settles here, per `docs/documentation-workflow.md`: hierarchy,
copy, state coverage, and wrap or truncation at the narrow bound. It settles
nothing about real typography, Dynamic Type, or VoiceOver order — those need the
manual acceptance the verification section requires.

The specimen that settles those four things is the prototype, at the URLs in
**Where the specimens come from**. The sketches below are the same states in
text, kept because this contract is reviewed as text and cited by blob hash.
Each one names the prototype URL it indexes; **they are an index to the
prototype, not a substitute for it**, and where the two disagree the prototype
is right.

### S0 — today, at 420 pt, `zh-Hans` (the reported state, for comparison)

Indexes the pre-condition surface, `?state=partial&lang=zh&width=420`, plus the
warning rows the reported payload carried.

```
┌────────────────────────────────────────────────────────┐
│ ◆ AgentDeck                     3 分钟前更新       ↻   │
│ [ 全部 ]                                               │
│ —                                                      │
│ [今天] [7天] [30天]                                    │
│ [用量⚠] [构成⚠] [归因⚠] [会话⚠]                       │
├────────────────────────────────────────────────────────┤
│ ⚠ 部分数据不可用                                       │
│ ⛔ 1 项检查未通过                                   ›  │
│ ⚠ 无法读取服务商状态                                   │
│ ⚠ 无法读取用量数据                                     │
│ ⚠ 无法读取会话数据                                     │
│ ⊖ 不可用                                               │
├────────────────────────────────────────────────────────┤
│ 服务商  服务商不可用                                ⌃  │
└────────────────────────────────────────────────────────┘
```

Six rows, all true, none causal.

### S1 — designed, at 420 pt, `zh-Hans`

Indexes `?state=schema&lang=zh&width=420`.

```
┌────────────────────────────────────────────────────────┐
│ ◆ AgentDeck                     3 分钟前更新       ↻   │
│ [ 全部 ]                                               │
│ —                                                      │
│ [今天] [7天] [30天]                                    │
│ [用量⚠] [构成⚠] [归因⚠] [会话⚠]                       │
├────────────────────────────────────────────────────────┤
│ ⛔ 此 AgentDeck 比其数据库旧，无法读取本地数据       ›  │
│ ⊖ 不可用 · AgentDeck 比其数据库旧                      │
├────────────────────────────────────────────────────────┤
│ 服务商  不可用 · 应用版本过旧                       ⌃  │
└────────────────────────────────────────────────────────┘
```

### S1 — designed, at 420 pt, `en`

Indexes `?state=schema&lang=en&width=420`.

```
┌────────────────────────────────────────────────────────┐
│ ◆ AgentDeck                    Updated 3m ago      ↻   │
│ [ All ]                                                │
│ —                                                      │
│ [Today] [7D] [30D]                                     │
│ [Usage⚠] [Breakdown⚠] [Attribution⚠] [Sessions⚠]       │
├────────────────────────────────────────────────────────┤
│ ⛔ This AgentDeck is older than its database, so    ›  │
│    local data cannot be read                           │
│ ⊖ Unavailable · AgentDeck is older than its database   │
├────────────────────────────────────────────────────────┤
│ Providers  Unavailable · app is older              ⌃   │
└────────────────────────────────────────────────────────┘
```

The `en` notice wraps to two lines at 420 pt; the row is already
`fixedSize(horizontal: false, vertical: true)`, so wrapping is the existing
behavior and not a new one.

### S1 — Health detail, at 420 pt, both languages

Indexes `?state=schema&width=420` with the primary notice clicked, in each
language.

```
┌────────────────────────────────────────────────────────┐
│ ‹ 返回                              运行状况           │
├────────────────────────────────────────────────────────┤
│ ✓ state_permissions                          正常      │
│ ✓ state_lock                                 正常      │
│ ⛔ database                                  失败   ⌄  │
│      数据库版本 999 · 此应用支持 23                    │
│      升级 AgentDeck 后才能打开该数据库                 │
│ 这些检查来自 agentdeck doctor。                        │
└────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────┐
│ ‹ Back                                 Health          │
├────────────────────────────────────────────────────────┤
│ ✓ state_permissions                            OK      │
│ ✓ state_lock                                   OK      │
│ ⛔ database                                Failed   ⌄  │
│      Database version 999 · this app supports 23       │
│      Upgrade AgentDeck to open this database           │
│ These checks come from agentdeck doctor.               │
└────────────────────────────────────────────────────────┘
```

No monospace and no *Copy recovery command* button on these two lines (D5);
compare a `schema_outdated` row, which keeps both.

### S1 — designed, at 280 pt (the narrow bound), `zh-Hans`

Indexes `?state=schema&lang=zh&width=280`.

```
┌────────────────────────────────┐
│ ◆ AgentDeck      3 分钟前   ↻  │
│ [ 全部 ]                       │
│ —                              │
│ [今天] [7天] [30天]            │
│ [用量⚠][构成⚠][归因⚠][会话⚠]  │
├────────────────────────────────┤
│ ⛔ 此 AgentDeck 比其数据库   ›  │
│    旧，无法读取本地数据        │
│ ⊖ 不可用 · AgentDeck 比其      │
│   数据库旧                     │
├────────────────────────────────┤
│ 服务商  不可用 · 应用版本过旧⌃ │
└────────────────────────────────┘
```

The footer is the only element with a truncation risk: `routesText` is
`lineLimit(1)` with `.truncationMode(.middle)`
(`MenuBarSurfaceView.swift:586-589`), and `不可用 · 应用版本过旧` is the
longest string this surface puts there.

**It fits, measured rather than asserted.** The prototype's gauge
(`?state=schema&width=280&measure=1`, extended by this topic to report text
clipped by `text-overflow: ellipsis`, which a container-overflow check cannot
see) reports `NO OVERFLOW` at 280 pt in both languages, and the footer element
measures:

| Language | Footer string | Column | Text | Headroom |
| --- | --- | --- | --- | --- |
| `zh-Hans` | `不可用 · 应用版本过旧` | 189 px | 108.67 px | **80.33 px** |
| `en` | `Unavailable · app is older` | 173 px | 135.86 px | **37.14 px** |

Two things this settles beyond the question asked. The gauge fails when it
should — substituting a deliberately over-long footer string reports
`TRUNCATED @280pt strong … short by 226px`, so `NO OVERFLOW` is an assertion and
not a tautology. And the string this surface puts in the footer is *shorter*
than the one the footer carries today: at 280 pt in `en`, the existing
`Codex aigocode · Claude official` route text truncates by 2 px while the
attributed string clears by 37. Attribution does not make this row tighter.

The fallback is therefore not expected to be needed, and it stays written down
for the real render: the unattributed `footerProviderUnavailable` plus the
popover sentence, which the popover carries either way. What remains in the
manual checklist is what a specimen cannot settle — real SF type metrics and
Dynamic Type — not the question of whether the string is too long.

### S2 and S4 — condition stacked with other problems, 420 pt, `zh-Hans`

Indexes `?state=schemaStacked&lang=zh&width=420`, which carries both at once —
the helper is unreachable *and* a second check is failing, so the full ranking
is visible in one specimen:

```
│ ⛔ 无法连接本地助手                                    │
│ ⛔ 此 AgentDeck 比其数据库旧，无法读取本地数据       ›  │
│ ⛔ 2 项检查未通过                                   ›  │
```

Each rule on its own, for reference. S2 alone — the refresh state outranks the
cause:

```
│ ⛔ 无法连接本地助手                                    │
│ ⛔ 此 AgentDeck 比其数据库旧，无法读取本地数据       ›  │
```

S4 alone — the count notice returns once there is something else to count (D3):

```
│ ⛔ 此 AgentDeck 比其数据库旧，无法读取本地数据       ›  │
│ ⛔ 2 项检查未通过                                   ›  │
```

### S6 — the reverse condition, unchanged, for contrast

```
│ ⚠ schema                                     警告   ⌄  │
│      agentdeck state migrate            [复制修复命令] │
```

## Data requirements

Per the progression, this surface names what it needs and `architecture.md`
provisions or refuses each. Everything below is read from the desktop wire
snapshot; the App Group projection is the widget's cache and is never a
menu-bar data source.

| Element | Field it needs | Status today |
| --- | --- | --- |
| The condition predicate — every rule in this document is gated on it | A stable, documented way to recognize this condition in one snapshot | **Partly present.** `health.checks[].code` already carries `unknown_schema`, and the app can match it. What is missing is that the code be *contractual*: `requirements.md`'s Contract changes puts the stable code in `architecture.md`'s hands, and this surface asks that whatever code it lands on be the one the app matches |
| Health row, cause line — the two numbers | The stored schema version **and** the version this binary supports, as integers on the check | **Requested — absent.** `DesktopHealthCheckV1` carries one `count` (`DesktopWire.swift:941-953`), mirroring `desktop.HealthCheck` (`desktop.go:214-220`) and `doctor.Check` (`doctor.go:23-29`), and this check sets none. `requirements.md` already declares the addition in scope across those three carriers; this surface is the element that renders it and needs both numbers, separately, as numbers |
| Notice-strip suppression (D2) | Knowledge that `provider_unavailable`, `provider_candidates_unavailable`, `usage_unavailable`, and `sessions_unavailable` are consequences of this condition | **Requested, with a fallback.** Preferred: the snapshot states the cause once, at envelope level, so the app suppresses by attribution rather than by a hard-coded code list. Fallback if refused: the app derives it — condition present implies those four codes are consequences — which works but hard-codes a rule the wire owns |
| Health row, recovery line | Nothing from the wire | **Refused by this surface.** The prose is app-side copy keyed off the code (D5). `recovery_command` must stay command-shaped and must be **absent** for this check; if it arrives carrying prose, this row still will not render it, and an untranslated English sentence would reach a Chinese UI |
| Panel bodies, footer, popover, menu-bar badge | The same condition predicate as row 1 | Same as row 1 |
| Clearing (S5) | Nothing | The absence of the check in the next snapshot is sufficient; no field, no sticky state |
| Widget | — | **Not requested** (D8). The widget projection is not extended by this topic |

Two requests, one refusal, one contractual confirmation. Nothing in this surface
depends on data that does not exist and is not asked for here.

## Accessibility

- The primary notice is one `accessibilityElement(children: .combine)` row, as
  every notice row already is, and carries the same sentence a sighted reader
  gets plus the existing `accessibilityHint(healthTitle)` that says it opens
  Health.
- The Health row's cause and recovery lines are inside the row's combined
  element, so the reader hears name, status, cause, recovery in that order.
- The tab marks keep `panelUnavailableMark`, so an assistive reader is told a
  panel is unavailable and the notice above tells it why.
- The menu-bar item's accessibility label becomes `badgedSchemaSignal`, ranked
  after `badgedOffline` and `badgedFailing` in
  `menuBarAccessibilityLabel` (`MenuBarViewModel.swift:1317-1321`), matching the
  S2 precedence rule.
- No animation is added, so nothing new to gate on Reduce Motion.

## What existing behavior this changes, and what it does not

Every rule here is gated on the condition being present, so no existing
expectation about a snapshot without it changes. In particular:

- `MenuBarViewModelTests.testNoticeStripOrdersUnreadableThenPartialThenHealth`
  (`MenuBarViewModelTests.swift:118-136`) asserts the order
  `failing, partial, health, warning.sessions_unavailable`. Its fixture is
  `WireFixture.failingHealth` (`AppTestFixtures.swift:469-477`), whose checks are
  `schema/ok`, `usage/usage_stale`, `prices/prices_missing` — no schema-excess
  check — so the assertion stays true and unmodified.
- `MenuBarViewModelTests.testUnavailableDomainsMarkTheirPanelAndRenderUnavailable`
  (`:157-171`) asserts marks and `panelUnavailableMark`, neither of which this
  surface changes.
- `DesktopCopyTests` gains seven keys to walk and no new rule.

## Verification

Automated, in the app target:

- notice-strip composition under the condition — the primary notice is present,
  first after any `offline`/`failing`, and the four consequence codes and the
  `partial` notice are absent;
- D3 — with `problems == 1` the count notice is absent, with `problems > 1` it
  follows the primary notice;
- the Health row renders both version numbers and the recovery prose, and
  exposes no copy affordance, while a `schema_outdated` row still does;
- panel bodies, footer text, popover text, and the menu-bar badge and
  accessibility label under the condition;
- S5 — a following snapshot without the check leaves no residue;
- `DesktopCopyTests` over the seven new keys in both languages, including the
  no-update-check assertion.

These need one new fixture beside `WireFixture.failingHealth`: a health payload
carrying the schema-excess check with both version numbers, plus the four
consequence warnings and `partial: true`. The fixture's shape is fixed by
`architecture.md`'s field decision, so it is written after that stage.

In the prototype, before any of that, because that is where this surface's
specimen lives:

- `?state=schema&width=280&measure=1` and `?state=schema&width=420&measure=1`,
  both languages — the gauge must report `NO OVERFLOW`;
- `?state=schemaStacked` — the strip must read `offline`, then the cause, then
  the count, in that order;
- `?surface=states` — this condition's two cells render beside the other six
  degraded states.

Manual, for what neither the automated tests nor the prototype can settle —
real SF type metrics, Dynamic Type, and VoiceOver
(`AGENTDECK_TEST_WIDTH=280`, `AGENTDECK_TEST_LOCALE=en|zh-Hans`):

- the primary notice's wrap at both widths in both languages under real type;
- the footer string under real SF metrics at 280 pt, where the prototype
  measured 80 px of headroom in `zh-Hans` and 37 px in `en` — a confirmation,
  not an open question;
- VoiceOver order over the notice, the Health row, and the menu-bar item;
- the badge at both menu-bar value preferences, including *Icon only*, where the
  badge is the only signal.

## Open questions for `architecture.md`

1. The stable code. This surface matches on it and does not care what it is,
   only that it is contractual and that a check for this condition always
   carries it.
2. One field or two for the version numbers, and their names. The surface needs
   both as integers on the check; it takes no position on whether that is a
   second count field, a pair, or a nested object.
3. Whether the envelope states the cause once, so suppression is attribution
   rather than a hard-coded code list (Data requirements, row 3). A refusal here
   is survivable and the fallback is written down.
4. Whether `recovery_command` stays absent for this check. This surface asks
   that it does and states why; a decision to put prose there does not change
   what this row renders.

The badge predicate's layer is **not** an open question — D7 answers it, and
`architecture.md` only revisits it if it provisions the condition somewhere the
Shared layer already sees.

## Approval boundary

Approval of this framework authorizes it as the presentation authority for this
condition on the menu-bar surfaces, and records the widget decision (D8). It
does not authorize implementation, contract changes, commit, push, or release,
and it does not settle any field this document requests — that is
`architecture.md`'s to provision or refuse, after which stage 7 returns here.
