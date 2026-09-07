---
status: active
created: 2026-09-06
updated: 2026-09-07
---

# Menu-Bar Schema Signal — Surface

This is stage 3 of the progression in `docs/documentation-workflow.md` — the
surface framework and its data requirements list — **and stage 7**, where it
absorbs the contract stage's answers. Everything under **Data requirements** was
a request to [`architecture.md`](../architecture.md); each row now records what
that document provisioned or refused, and every rule below is written against
the contract as settled rather than against the request.

`architecture.md` passed review at Round 2 on 2026-09-07. What stage 7 took from
it is listed in its own **What stage 7 must absorb** section, plus one item this
document adds and marks as its own; see **What stage 7 absorbed** below.

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
| S1 or S2 + S4 with an independently unreadable session store | append `&sessions=unavailable` to either schema state, in either language |
| Every degraded state side by side | `?surface=states` |
| The overflow and truncation gauge | append `&measure=1` |

`state=schema` and `state=schemaStacked` are this topic's additions to the
prototype's state list, and the eight strings in **Copy** below are the same
eight keys in the prototype's `src/i18n.js` dictionary, in both languages. The
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

   The `unknown_schema` above is what that measurement returned, and it stays
   recorded as measured. It is **not** the code this surface matches: C1 gives
   the condition its own `schema_ahead`, because `unknown_schema` also covers
   three metadata-damage sites that have no version pair to report. Every rule
   below matches `schema_ahead`.
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
holds, the strip suppresses the `partial` notice and exactly two warning codes:
`provider_unavailable` and `usage_unavailable`. Ground: the failed core-store
open emits both unconditionally (`internal/desktop/desktop.go:252-263`), so
they state symptoms the primary notice already explains.
Warnings that are *not* consequences — `state_close_failed`,
`sessions_unavailable`, `sessions_close_failed`, an unrecognized code — still render, because they are
independent facts.

`loadSessions` runs outside that failed-open branch and emits
`sessions_unavailable` if its own open or list fails
(`internal/desktop/desktop.go:480-497`). `OpenSessionsReadOnly`
(`internal/store/store.go:73-91`) opens the separate `sessions.sqlite3` and
checks its two tables, not the core schema version. Both session warning codes
therefore stay visible for the same reason. Upgrading AgentDeck does not claim
to repair that independent failure. The readable-session S1 specimen has no
session warning; its `sessions=unavailable` variant retains one after the cause
and any health count notice.

The list had a fourth member, `provider_candidates_unavailable`, and stage 7
removed it: `architecture.md` located its only producer at
`internal/desktop/desktop.go:305`, inside `loadProvider`, which runs only in the
`else` branch taken after `OpenReadOnly` **succeeded**. Under this condition
that branch is never entered, so the code cannot appear and suppressing it
suppressed nothing. Harmless either way — it is removed so that a later reader
does not have to re-derive why a code that cannot occur was on a suppression
list.

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

**Which branch is taken is now decided by the contract, not by chance.** C5 adds
a second check, `hook_deliveries`, reported as a `warning` whenever Hook
deliveries were refused for this condition and the condition still holds for
this binary. `Report.add` (`internal/doctor/doctor.go:481-489`) counts every
non-`ok` check, so:

| Situation | `health.problems` | Strip |
| --- | --- | --- |
| Condition holds, no Hook delivery was ever refused — hooks not configured, or none attempted since | 1 | S1: primary notice alone |
| Condition holds and a refusal was recorded | 2 | S4: primary notice, then the count |

Both are ordinary, and the second is the common case on a machine where hooks
are set up — which is the machine `requirements.md` measured. S1 remains the
specimen for the rule; S4 is what a hook-configured user sees.

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

**D9 — The `hook_deliveries` row states its count, and nothing else changes for
it.** *(Added at stage 7. `architecture.md` created this check; nothing in its
absorb list covers how the row reads, and leaving that undecided would put a
bare, unexplained warning on the surface whose whole purpose is to remove
one.)*

C5's check arrives in `health.checks[]` like any other, so the Health detail
renders it with no work. Three decisions, all narrow:

- **The row shows one cause line: the number of dropped deliveries.** Ground:
  `count` is the only thing this check carries that a user can act on the
  knowledge of, and a row reading `hook_deliveries  警告` with nothing under it
  is exactly the bare-symptom shape D1 exists to replace. Its own
  `HealthCheckRowView` instance needs the same new prose disclosure branch as
  D4; no disclosure area is shared across check rows.
- **No recovery line and no copy button**, for D5's reason and C6's: there is no
  command, and the upgrade that resolves it is already named on the database row.
- **It does not enter the notice strip.** Ground: the primary notice already
  names the cause, and the count notice already points at Health. A third strip
  row about a consequence of the same cause is the competition this surface
  exists to end. The check is visible where check detail belongs.

**The check name is not localized, and this surface does not change that.**
`healthDetail` (`MenuBarViewModel.swift:1262-1274`) passes `check.name` through
untouched and `HealthCheckRowView` renders it with `Text(row.name)`
(`MenuBarSurfaceView.swift:540-553`, the `statusRow` at `:547`), so every row today reads
`state_permissions`, `state_lock`, `database` — raw identifiers in both
languages. `hook_deliveries` joins them and looks exactly like its neighbours.
Localizing check names is a real gap, and it is a pre-existing one covering
every check; taking it on here would be this topic paying for a defect it did
not introduce.

**Both rows need the stable code and numeric fields on the row model.** `HealthCheckRow`
(`MenuBarViewModel.swift:271-277`) carries `id`, `name`, `status`, `severity`,
and `recovery` — none of `code`, `count`, or `supported_count`.
`healthDetail` must carry `check.code`, `check.count`, and
`check.supportedCount` from `DesktopHealthCheckV1` into the row model (the wire
key for the last field is `supported_count`). Match `code == schema_ahead`
for D4's stored `count`, `supported_count`, and recovery prose; match
`code == hook_deliveries_dropped` for D9's dropped-delivery `count`, with no
recovery prose. Neither predicate matches `name` or localized status. The
optional fields stay absent when the check does not carry them; no zero is
invented for an absent number. This is one shared projection change for both
rows, plus the view changes below, not a second wire request.

## Presentation states

| ID | State | Rule |
| --- | --- | --- |
| S1 | Condition present, snapshot current, no Hook refusal recorded | The full treatment below: primary notice, expanded Health row, attributed panel bodies and footer, badged item |
| S2 | Condition present **and** helper unreachable or failing | The existing `offline` / `failing` notice ranks first and the primary notice follows it. Ground: a snapshot we could not refresh cannot assert the condition still holds, only that it held when the snapshot was taken |
| S3 | Condition present, snapshot aged or stale | Unchanged freshness treatment (`freshnessText`, `MenuBarViewModel.swift:336-342`). The primary notice makes no claim of liveness beyond the snapshot's own `generated_at` |
| S4 | Condition present with other problems (`health.problems > 1`), including the ordinary case where a Hook refusal was recorded | Primary notice first, health count notice second, per D3. The `hook_deliveries` row states its count in Health, per D9 |
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

Suppressed while the condition holds: the `partial` notice and the two
consequence warning codes (D2). Suppression is a presentation rule only — the
snapshot still carries `partial: true` and every warning code, and the CLI still
reports them.

The primary notice re-uses `noticeRow` unchanged: severity symbol, caption text
that wraps (`fixedSize(horizontal: false, vertical: true)`), chevron, tinted
14 %-opacity fill with a 55 %-opacity stroke, `rowMinimumHeight` 28 pt.

### Health detail row

The row for this condition uses the existing disclosure geometry, expanded by
default like every recovery row today (`isExpanded = true`,
`MenuBarSurfaceView.swift:509`), but requires a new view branch. Today the sole
disclosure predicate is non-empty `row.recovery` (`:512`), and its expanded
content is only `recoveryRow(recovery)` (`:528-532`). Neither D4 nor D9 carries
a recovery command. Change the predicate to **has cause prose or has recovery
prose or has a non-empty recovery command**, and add a prose branch with normal
proportional caption text and no copy button:

- **line 1** — the check name and the localized `Failed`, unchanged;
- **line 2** — the cause: the stored version and the supported version;
- **line 3** — the recovery, in prose, with no monospace and no copy button (D5).

Lines 2 and 3 use the existing disclosure indentation
(`padding(.leading, rowMinimumHeight)`) in that new branch. Do not put prose in
`row.recovery`: that field remains a shell command and retains the monospaced
`recoveryRow` and its copy button for checks such as `schema_outdated`.

The `hook_deliveries` row C5 adds uses the same shape with one cause line and no
recovery line (D9). Every other check row is untouched.

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
| `schemaSignalHookDropped` | %1$lld Hook deliveries were dropped while this AgentDeck could not open its database | 本 AgentDeck 无法打开数据库期间，%1$lld 次 Hook 投递被丢弃 |

The last row is stage 7's, added with D9 for the check C5 introduced; the seven
above it are unchanged from the framework round.

These same eight keys, under the same names and with the same values in both
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
says *upgrade AgentDeck* and never *download the new version*. The eight strings
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

Six rows, all true, none identifying the core-store cause. The session warning
is an independent failure in this throwaway-state variant, not another symptom
of that cause; on the reporter's readable session store it was absent.

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

### S1 with an independently unreadable session store, both languages

Indexes `?state=schema&sessions=unavailable&lang=zh&width=420` and its `lang=en`
variant. The ordinary S1 specimen above has a readable session store; this
variant adds exactly one warning after the primary notice:

```text
⛔ 此 AgentDeck 比其数据库旧，无法读取本地数据  ›
⚠ 无法读取会话数据

⛔ This AgentDeck is older than its database, so local data cannot be read  ›
⚠ Session data could not be read
```

The same switch on `state=schemaStacked` retains the warning after offline,
the schema cause, and the health count. It neither adds a doctor check nor
changes `health.problems`; it is a desktop warning from the independent store.
The warning uses the existing `warningSessionsUnavailable` copy, not a ninth
schema-specific string. Both variants are also measured at 280 pt.

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
the helper is unreachable *and* a second check is not passing, so the full
ranking is visible in one specimen. Stage 7 changed what that second check is:
the state used to carry an invented `usage_index` warning, and now carries the
`hook_deliveries` warning C5 guarantees will actually be there, so the specimen
shows a combination the product can produce rather than one it cannot:

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

### S4 — Health detail with the Hook row, 420 pt, both languages

Indexes `?state=schemaStacked&width=420` with the primary notice clicked. The
`hook_deliveries` row precedes `database` because `doctor` runs the record check
before it opens the store (C5); the order below is what the prototype renders,
not a preference.

```
┌────────────────────────────────────────────────────────┐
│ ‹ 返回                              运行状况           │
├────────────────────────────────────────────────────────┤
│ ✓ state_permissions                          正常      │
│ ✓ state_lock                                 正常      │
│ ⚠ hook_deliveries                            警告   ⌄  │
│      本 AgentDeck 无法打开数据库期间，137 次           │
│      Hook 投递被丢弃                                   │
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
│ ⚠ hook_deliveries                         Warning   ⌄  │
│      137 Hook deliveries were dropped while this       │
│      AgentDeck could not open its database             │
│ ⛔ database                                Failed   ⌄  │
│      Database version 999 · this app supports 23       │
│      Upgrade AgentDeck to open this database           │
│ These checks come from agentdeck doctor.               │
└────────────────────────────────────────────────────────┘
```

One cause line, no recovery line, no copy button (D9).

### S6 — the reverse condition, unchanged, for contrast

```
│ ⚠ schema                                     警告   ⌄  │
│      agentdeck state migrate            [复制修复命令] │
```

## Data requirements

Per the progression, this surface named what it needs and `architecture.md`
answered each. Everything below is read from the desktop wire snapshot; the App
Group projection is the widget's cache and is never a menu-bar data source. The
**Verdict** column is the contract's answer as settled at its Round 2 PASS; the
request each row made is kept beside it so the two can be read against each
other.

| Element | Field it needs | Requested | Verdict |
| --- | --- | --- | --- |
| The condition predicate — every rule in this document is gated on it | A stable, documented way to recognize this condition in one snapshot | That whatever code it lands on be contractual, and be the one the app matches | **Provisioned as `schema_ahead`** (C1). A new value, not a promotion of `unknown_schema`: that string is also returned from three metadata-damage sites (`migrations.go:366`, `:413`, `:418`) which carry no version pair, so a code covering all five would be documented false for three of them. It gets a row in `cli-design.md`'s stable `error.code` table |
| Health row, cause line — the two numbers | The stored schema version **and** the version this binary supports, as integers on the check | Both, separately, as numbers | **Provisioned as `count` and `supported_count`** (C2), added to all three carriers — `doctor.Check`, `desktop.HealthCheck`, `DesktopHealthCheckV1`. The stored version stays in the existing `count`, matching `schema_outdated`; the new key pairs with it by name rather than implying a `version` sibling that does not exist. Additive to wire v1 |
| Notice-strip suppression (D2) | Knowledge that `provider_unavailable` and `usage_unavailable` are consequences of this condition; session warnings are independent | Preferred: the snapshot states the cause once at envelope level. Fallback: the app derives it | **Refused; the fallback applies.** Ground given: the suppression rule is presentation, not wire — the wire owns "this section could not be read", while "therefore do not also show a notice about it" is D2's decision; an envelope-level cause would need a consequence taxonomy with one member; and the derivation is exact rather than heuristic, because `internal/desktop/desktop.go:254-255` emits `provider_unavailable` and `usage_unavailable` unconditionally when `OpenReadOnly` fails. This surface accepts the refusal — its own words for the fallback, that deriving it "hard-codes a rule the wire owns", were wrong about who owns it. The refusal removed the unreachable provider-candidates code. UX-R3-F1 separately removes `sessions_unavailable` after tracing its independent producer; the architecture text's contrary claim remains tracked by `ad-bug-arch-sessions-unavailable-not-a-consequence` and is not adopted here |
| Health row, recovery line | Nothing from the wire | That `recovery_command` stay **absent** for this check | **Provisioned as requested** (C6). `recovery_command` keeps its command contract; the recovery reaches the CLI through the error message and the doctor text line, and reaches this surface as app-side copy keyed off the code |
| Panel bodies, footer, popover, menu-bar badge | The same condition predicate as row 1 | Same as row 1 | Same as row 1 |
| Clearing (S5) | Nothing | No field, no sticky state | **Confirmed.** The check's absence from the next snapshot is the whole mechanism |
| Widget | — | Not requested (D8) | **Confirmed.** `WidgetDesktopSnapshotV1` is not extended |
| The `hook_deliveries` row's count (D9) | The dropped-delivery count on that check | *Not requested at stage 3 — the check did not exist yet* | **Already provisioned on the wire.** C5 puts `code: hook_deliveries_dropped` and `count` on the check. The App must project `code`, `count`, and `supported_count` as specified in D9; C2 supplies wire fields, not an existing App row-model extension |

Two requests provisioned, one refused with its fallback taken, two confirmations,
and one field this surface did not know to ask for. Nothing in this surface now
depends on data the contract does not carry.

## Accessibility

- The primary notice is one `accessibilityElement(children: .combine)` row, as
  every notice row already is, and carries the same sentence a sighted reader
  gets plus the existing `accessibilityHint(healthTitle)` that says it opens
  Health.
- The Health row's combined accessibility element **must be extended** to
  cover the new cause and recovery prose: today only `statusRow` is combined
  (`MenuBarSurfaceView.swift:552`), with disclosure content outside it. The
  intended expanded-row order is name, status, cause, recovery; D9's separate
  row reads name, status, cause. Preserve an operable disclosure control and
  expanded/collapsed state without duplicate speech. This is an implementation
  requirement, not a claim about today's VoiceOver behavior; verify it on device.
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
- `DesktopCopyTests` gains eight keys to walk and no new rule.
- **D4 and D9 require a projection change and view changes.** The type
  (`MenuBarViewModel.swift:271-277`) is constructed in exactly one place,
  `healthDetail` (`:1265`), and no test builds one directly; the rows are only
  ever read back through `model.healthDetail`. So
  `MenuBarViewModelTests.swift:491-495`, which asserts `rows.count == 3` and each
  row's `status` and `recovery`, stays true and unmodified — it never reads a
  count. The implementation scope is: carry `code`, `count`, and
  `supported_count` through that initializer; derive localized prose using the
  two stable-code predicates in D9; broaden `HealthCheckRowView`'s disclosure
  condition to include prose; add the proportional, button-free prose branch
  while preserving the command branch; and extend accessibility grouping to
  include the prose and retain an operable disclosure. Existing row assertions
  remain valid, but new projection, disclosure, and accessibility checks are
  required. One initializer is not the size of the whole change.

## Verification

Automated, in the app target:

- notice-strip composition under the condition — the primary notice is present,
  first after any `offline`/`failing`, and the two consequence codes and the
  `partial` notice are absent; an independently present `sessions_unavailable`
  warning remains after the cause and any health count, and is absent when the
  session store is readable;
- D3 — with `problems == 1` the count notice is absent, with `problems > 1` it
  follows the primary notice, covering both the no-refusal and the
  refusal-recorded shapes the contract produces;
- the Health row renders both version numbers and the recovery prose, and
  exposes no copy affordance, while a `schema_outdated` row still does; both
  prose rows can expand with `recovery_command` absent;
- D9 — the `hook_deliveries` row renders its count as one cause line, with no
  recovery line and no copy affordance, and adds no notice-strip row;
- panel bodies, footer text, popover text, and the menu-bar badge and
  accessibility label under the condition;
- S5 — a following snapshot without the check leaves no residue;
- `DesktopCopyTests` over the eight new keys in both languages, including the
  no-update-check assertion.

These need one new fixture beside `WireFixture.failingHealth`: a health payload
carrying the schema-excess check with `count` and `supported_count`, the
`hook_deliveries` warning with its own `count`, the two consequence warnings,
and `partial: true`. The field decision that fixes its shape is settled — C2 for
the two version keys, C5 for the Hook check — so the fixture can now be written.
A second variant without the `hook_deliveries` check covers the `problems == 1`
branch of D3.
For each variant, cover `sessions_unavailable` both absent and present; it is
an independent warning, not a third consequence. Assert stable-code selection
with a different check name, both version fields on the schema row, and only
the count cause on `hook_deliveries_dropped`.

In the prototype, before any of that, because that is where this surface's
specimen lives. **Stage 7 ran all of it against the re-rendered states**; the
results below are observations, not instructions:

- `?state=schema` and `?state=schemaStacked` at 280 pt and 420 pt, both
  languages, with `&measure=1` — the gauge reported `NO OVERFLOW` on all eight
  combinations, `document.title` `overflow:0` each time. The new cause line for
  `hook_deliveries` is inside that result;
- `?state=schemaStacked&lang=zh&width=420` — the strip read `无法连接本地助手`,
  then `此 AgentDeck 比其数据库旧，无法读取本地数据`, then `2 项检查未通过`, in
  that order;
- `?state=schema&lang=zh&width=420` — the count notice was absent, as D3
  requires at `problems == 1`, and Health showed three rows;
- Health detail under `schemaStacked`, both languages — `state_permissions`,
  `state_lock`, `hook_deliveries` with its count line, then `database` with the
  two numbers and the recovery line, matching the S4 specimen above verbatim;
- `?surface=states` — this condition's two cells render beside the other six
  degraded states, the second now labelled with the stacked case.

`?probe=1` reports `1 FAILED` on the default state, unchanged by this round: the
failing assertion is `详情里标注了待采集`, already carried by
`ad-bug-prototype-probe-pending-banner` and `ad-bug-prototype-probe-pending-assertion`.
The three health-detail assertions in that run pass.

**UX-R3 repair measurement, 2026-09-07:** the eight combinations above were
re-run, then repeated with `sessions=unavailable` (16 total). Every gauge title
was `overflow:0`. The readable-session variants retain the one-row S1 and
three-row stacked strips; the unreadable-session variants add exactly the
localized session warning last, producing two and four rows respectively.
The existing English warning is `Session data could not be read`, matching
`DesktopCopy.warningSessionsUnavailable`; the Chinese warning is
`无法读取会话数据`. Entering Health from the narrow English stacked variant
still shows the Hook count 137 and database versions 999/23, with no copy button.
The earlier probe result above is historical and was not re-run for this repair.

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

## Open questions — all closed at stage 7

The four questions this document put to `architecture.md`, with the answers it
gave. None remains open.

1. **The stable code** → `schema_ahead` (C1). Contractual, in
   `cli-design.md`'s stable `error.code` table, and always on the check for this
   condition.
2. **One field or two for the version numbers, and their names** → two,
   `count` and `supported_count` (C2), on all three carriers.
3. **Whether the envelope states the cause once** → no. Refused with its ground,
   the fallback taken; see the Data requirements row. The refusal was accepted
   rather than merely absorbed: the reason this document gave for preferring the
   envelope was that the app would otherwise hard-code "a rule the wire owns",
   and that premise was wrong.
4. **Whether `recovery_command` stays absent** → yes (C6), as asked.

The badge predicate's layer was never an open question — D7 answers it, and
`architecture.md` did not provision the condition anywhere the Shared layer
sees, so D7 stands unchanged.

## What stage 7 absorbed

Three items from `architecture.md`'s **What stage 7 must absorb**, plus one this
document adds and marks as its own:

1. **The prototype now carries the contract's code and key names.**
   `prototype/src/data.js` `HEALTH_SCHEMA` and `HEALTH_SCHEMA_STACKED` set
   `code: "schema_ahead"`, `count: 999`, `supported_count: 23`, and
   `prototype/src/Popover.jsx` matches `schema_ahead` for the expansion and the
   cause/recovery pair. Every specimen in **Rendered specimens** was re-rendered
   from the new state; the measurements under **Verification** are from that
   run, not the framework round's.
2. **`health.problems` is 2 when a Hook refusal was recorded**, so a
   hook-configured user sees S4 where the framework round drew S1. Absorbed in
   D3, which now states which branch is taken by contract rather than leaving it
   to the reader, and in the S1/S4 rows of **Presentation states**.
3. **`provider_candidates_unavailable` left D2's suppression list**, with the
   located ground. Stage 7 left three codes; UX-R3-F1 repair subsequently
   removed the independent `sessions_unavailable`, leaving two.
4. **D9 is this document's own addition.** C5 created a check that lands on this
   surface, and nothing in the contract's absorb list decides how its row reads.
   Left undecided it would have put a bare `hook_deliveries  警告` on the panel —
   the exact shape D1 exists to remove — so stage 7 decided it here rather than
   letting the implementation choose. It is the only new decision in this round,
   and it is new since the framework passed review.

## Approval boundary

Approval of this document authorizes it as the presentation authority for this
condition on the menu-bar surfaces, and records the widget decision (D8) and the
Hook-row decision (D9). Every field it depends on is provisioned by
`architecture.md`, which passed review at Round 2; nothing here waits on a
contract answer any more.

It does not authorize implementation, the `cli-design.md` edits, commit, push,
or release. The `prototype/` edit named under **What stage 7 absorbed** is
already made — it is this stage's own work product, not a pending action — and
it is the reason every specimen above is current.

The user requested a direct review after stage 7; Round 3 returned FAIL.
UX-R3-F1 through UX-R3-F4 are repaired in this candidate and await independent
re-review. The Review cell remains unchecked. Only after that re-review passes
and its evidence boundary is satisfied does the progression return to stage 8,
the decomposition (`设计：schema-version-signal / tasks.md`).
