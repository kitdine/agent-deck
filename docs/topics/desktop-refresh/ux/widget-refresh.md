---
status: active
created: 2026-09-19
updated: 2026-09-19
---

# Widget Refresh — Surface Framework

## Purpose and stage

This framework defines how every AgentDeck Widget presents snapshot age, typed
App Group read failures, host absence, semantic publication changes, unchanged
publication, and recovery. It implements the Widget boundary in
[`requirements.md`](../requirements.md) without choosing timeline, digest, wire,
or storage implementation.

The stage-3 framework passed Review Round 1 and was delivered in `8279718`.
This revision is the stage-7 final-surface reconciliation against reviewed
architecture blob `d5ff801efa8b0d15fb50e939d2726dd6284ca58e` and signed
architecture commit `c0c6daf`. The shared
[product prototype](../../../../prototype/README.md) remains the visual authority
for all five Widget kinds and three native sizes.

## Inherited contracts

- Preserve the existing five kinds (`magnitude`, `composition`, `trust`,
  `rhythm`, `quota`), three size-as-depth layouts, client configuration,
  semantic colors, and English/Chinese copy.
- A Widget reads only the privacy-bounded App Group projection. It never starts
  the host, helper, scan, network request, or database access.
- A readable supported projection renders its values regardless of age. Age
  adds truthful footer copy; it does not discard valid data.
- Quota uses the oldest displayed quota observation in its footer. A Widget
  timeline read never retimestamps quota data.
- Placeholder skeletons remain gallery/system placeholders and are never used
  as runtime error or loading data.

## Design principles

1. **Reread is not success.** Constructing a timeline entry or rereading an
   unchanged file does not make the data fresh.
2. **Age advances without the host.** Every 3–5 minute timeline opportunity
   evaluates the preserved successful time against the actual new entry time.
3. **Host absence is not directly observable.** A readable old snapshot shows
   its age; the Widget does not invent a `Host offline` badge or process probe.
4. **Read failures replace values.** If the current entry cannot safely decode
   the projection, it shows a typed unavailable surface, never retained figures
   labeled with the entry time and never zero-valued success.
5. **Publication change is quiet.** Changed content atomically becomes the next
   visible values; unchanged publication adds no toast, badge, or animation.
6. **Recovery clears only the load failure.** A succeeding read restores data
   and its own successful time; quota, partial, empty, and pricing qualifiers
   continue to reflect their independent fields.

## Runtime surface model

| Load outcome | Projection age | Surface | Footer/qualifier |
| --- | --- | --- | --- |
| Placeholder request | — | Existing redacted placeholder | No production freshness claim |
| Readable, supported | < 15 min | Existing data surface | `Updated <relative>`; `Updated just now` only for actual successful time |
| Readable, supported | 15 min–6 h | Existing data surface | `Updated <relative>` plus existing `aging` qualifier semantics |
| Readable, supported | > 6 h | Existing data surface | `Last updated <relative>` plus existing `old` qualifier semantics |
| Snapshot missing | — | Typed unavailable surface | `No Widget data yet` / `Open AgentDeck to refresh` |
| App Group container unavailable | — | Typed unavailable surface | `Widget storage unavailable` / `Open AgentDeck to retry` |
| Read/decode/malformed failure | — | Typed unavailable surface | `Widget data could not be read` / `Will retry on the next refresh` |
| Unsupported projection version | — | Typed unavailable surface | `Widget data is from a newer AgentDeck` / `Upgrade AgentDeck to refresh` |

`aging` and `old` remain mutually exclusive. Partial and empty still derive from
the readable projection and may compose with either age tier. A typed load
failure has no partial/empty qualifier because those describe decoded data.

## Host absence and sleep

The Widget has no host-presence field and must not add one. If the host is absent
but the projection is readable, the Widget follows the same fresh → aging → old
ladder using successful data time. If no projection has ever been published, it
shows the missing state. A container or decode failure keeps its own typed cause;
it is not rewritten as host absence.

After sleep, the next WidgetKit entry evaluates age from wall-clock time and may
jump directly from fresh to old. No missed entries are replayed and no animation
is required. The host may later publish and request a reload; until WidgetKit
delivers the new entry, the current card continues to tell the truth about the
snapshot it actually displays.

## Changed, unchanged, and recovered publication

| Event | Visible result |
| --- | --- |
| Material projection change affecting this kind | WidgetKit reload is requested for the affected kind; the next accepted entry atomically replaces values and successful time |
| Publication with no material fields changed | No immediate reload request and no visible event; the current entry/age stays until the 3–5 minute fallback reread |
| Scheduling or encoding bytes changed only | Same as unchanged; never treated as a semantic Widget change |
| Recovery from missing/read/container/version failure | Next successful supported read replaces the unavailable surface with data and the preserved successful-data time |
| Quota-only material change | Only quota reload is requested; other kinds retain their current entries |

There is no `Updated because values changed` copy. A Widget is a current reading,
not an activity log. `changed`, `unchanged`, and `recovered` exist as prototype
stage labels so reviewers can compare the outcomes; they are not product chips.

## Copy contract

| Meaning | English | Chinese |
| --- | --- | --- |
| Fresh | `Updated just now` / `Updated <relative>` | `刚刚更新` / `<相对时间>更新` |
| Aging | `Updated <relative>` | `<相对时间>更新` |
| Old | `Last updated <relative>` | `上次更新于<相对时间>` |
| Missing title | `No Widget data yet` | `还没有小组件数据` |
| Missing footer | `Open AgentDeck to refresh` | `打开 AgentDeck 以刷新` |
| Container title | `Widget storage unavailable` | `小组件存储不可用` |
| Container footer | `Open AgentDeck to retry` | `打开 AgentDeck 以重试` |
| Read/decode title | `Widget data could not be read` | `无法读取小组件数据` |
| Read/decode footer | `Will retry on the next refresh` | `等待下次刷新重试` |
| Unsupported title | `Widget data is from a newer AgentDeck` | `小组件数据来自较新版本的 AgentDeck` |
| Unsupported footer | `Upgrade AgentDeck to refresh` | `升级 AgentDeck 后刷新` |

Failure copy names the local Widget boundary rather than claiming the network or
provider failed. No error surface offers an in-Widget button; Widget families are
read-only, and the named recovery happens through AgentDeck or the next timeline.

## Size and family behavior

Every failure state uses the existing frame header and footer. The body centers
one warning glyph plus the typed title. Small may wrap to two lines; medium and
large do not add diagnostics merely because they have more room. A larger Widget
adds data depth only when data exists, not error verbosity.

All five kinds share the typed load outcome. On readable data:

- the four usage/activity kinds use snapshot successful time in the footer;
- quota retains its oldest displayed quota observation/reason footer;
- partial, empty, pricing, attribution, and quota-staleness qualifiers keep their
  existing copy and precedence; and
- the 3–5 minute Widget reread time is never displayed as a data observation.

## Timeline and scheduling presentation

Each provider requests its next opportunity no earlier than three minutes and no
later than five minutes after the current entry. This is a request contract, not
an exact macOS render promise. Every invocation returns one current entry; it
does not precompute future values or replay missed intervals.

The design records requested and observed native timing separately. A browser
specimen can prove copy, layout, and deterministic age derivation, but cannot
prove WidgetKit grants the request or that a host reload renders immediately.

## Data requirements

| Element/decision | Required data | Owner/disposition for architecture |
| --- | --- | --- |
| Generic footer age | Last successfully accepted projection `generated_at` plus actual entry time | Existing fields are candidates; architecture verifies clock and parse behavior |
| Quota footer age | Oldest displayed quota `observed_at`, or typed quota reason | Existing subscription projection; unaffected by Widget reread time |
| Missing surface | Typed `fileNotFound` distinct from container/read errors | Widget loader must preserve this local outcome |
| Container surface | Typed App Group/container-unavailable outcome | Widget loader must preserve this local outcome |
| Read/decode surface | Typed I/O, malformed JSON, and supported-schema decode failure category | UX combines these because the recovery/copy is identical; architecture retains diagnostics for tests/logging |
| Unsupported surface | Decoded schema version mismatch distinct from malformed input | Existing schema check is the candidate |
| Age transition | Actual invocation/entry time, not prior entry date | Timeline provider owns current time injection |
| Fallback request | Next request date in the 3–5 minute window | Timeline policy owns it; not stored in the projection |
| Changed affected kinds | Canonical semantic comparison per Widget kind | Host publication contract owns it; not exposed as UI data |
| Recovery | Successful read for a newer entry and its successful timestamp | Widget load outcome replaces only its prior failure |

The surface refuses process-presence probing, missed-interval counts, a second
Widget-refresh timestamp, helper/network errors not present in the projection,
and any private database or credential field.

## Final architecture reconciliation

The reviewed architecture resolves every framework request without adding a
Widget element or changing copy:

| Framework request | Final disposition |
| --- | --- |
| Generic footer age | Existing projection `generated_at` plus actual injected entry date |
| Quota footer | Existing oldest displayed quota observation/reason; Widget reread time remains excluded |
| Typed failure surfaces | Process-local `WidgetLoadOutcome` with missing, container unavailable, unreadable, and unsupported-version failures |
| Read/decode safety | Shared 8 MiB bounded loader used by host comparison and Widget reader; oversized/unsafe remain diagnostic subcategories of reviewed unreadable UI |
| 3–5 minute opportunity | Timeline minimum 3m, default 4m, maximum 5m; one actual-time entry per invocation |
| Host absence | Explicitly unobservable; readable data follows the same age ladder, with no product label |
| Changed publication | Canonical per-kind semantic comparison requests only affected kind reloads |
| Unchanged publication | Empty affected-kind set; no immediate reload, event chip, or freshness reset |
| Unknown old baseline | Next verified publication reloads all five kinds; the current Widget still presents its actual local load outcome |
| Recovery | A newer loaded outcome atomically replaces the typed failure; unrelated quota/partial/empty qualifiers retain ownership |

Publisher admission barriers, quota/full arbitration, definite versus
indeterminate host publication results, and generation recovery are intentionally
not exposed in Widget copy. The extension reports only the file it actually reads.

No architecture decision invalidates the nine framework specimens. The only
shared prototype source change since framework review is
`settings.periodicRefreshHint` for menu-bar final reconciliation; `Widgets.jsx`
does not consume that key, and no `widgets.*` copy, Widget layout, state mapping,
or screenshot pixel changes. The framework browser/axe evidence is therefore
reused with an updated manifest binding rather than rerun as a cosmetic ritual.

## Accessibility and layout

- Each frame's accessibility label includes kind, size, and either truthful
  freshness/footer text or the typed failure title. Failure glyphs are decorative
  once the text is exposed.
- Failure meaning never depends on color. Warning tone accompanies, but does not
  replace, the title and footer recovery.
- Small, medium, and large use the same reading order: header, body, footer.
- At accessibility Dynamic Type sizes, existing depth degradation remains; the
  typed error is never truncated to make room for absent data.
- English and Chinese failure titles wrap within the true native proportions;
  no horizontal scrolling, clipping, or minimum-scale text is introduced.
- Widget changes do not create live regions, move focus, or animate. WidgetKit
  replacement is a passive surface update.

## Shared prototype specimens

The Widget gallery adds a `widgetRefresh` stage axis independent of usage,
quota, and menu-bar refresh state. Stage labels are review tooling, not product
copy. Required framework specimens:

| Specimen | URL state | What it proves |
| --- | --- | --- |
| Aging | `surface=widgets&widgetRefresh=aging&lang=en&theme=light` | All frames keep values and show the 18-minute successful-data age |
| Host absent/old | `surface=widgets&widgetRefresh=hostAbsent&lang=zh&theme=dark` | No invented host label; readable data ages to the old wording |
| Unchanged publication | `surface=widgets&widgetRefresh=unchanged&lang=en&theme=dark` | No toast/badge or immediate freshness reset |
| Changed publication | `surface=widgets&widgetRefresh=changed&lang=zh&theme=light` | New values use the ordinary fresh presentation, not an event chip |
| Missing | `surface=widgets&widgetRefresh=missing&lang=en&theme=light` | No zero values; missing copy and recovery in all sizes |
| Container unavailable | `surface=widgets&widgetRefresh=containerUnavailable&lang=zh&theme=dark` | Storage-specific cause remains distinct |
| Read failed | `surface=widgets&widgetRefresh=readFailed&lang=en&theme=dark` | Read/decode copy and next-refresh recovery |
| Unsupported version | `surface=widgets&widgetRefresh=unsupported&lang=zh&theme=light` | Version-specific cause and upgrade recovery |
| Recovered | `surface=widgets&widgetRefresh=recovered&lang=en&theme=light` | Data returns atomically with ordinary freshness and no lingering failure |

Screenshots, browser assertions, and hashes live under
`ux/prototype/widget-refresh/`. Browser evidence proves copy, family/size
coverage, overflow, accessible labels, and visual composition. It does not prove
native WidgetKit scheduling, native Dynamic Type, VoiceOver timing, real App
Group failures, sleep/wake delivery, or targeted host reload behavior.

- [Aging](prototype/widget-refresh/aging-en-light.png)
- [Host absent/old](prototype/widget-refresh/host-absent-zh-dark.png)
- [Unchanged publication](prototype/widget-refresh/unchanged-en-dark.png)
- [Changed publication](prototype/widget-refresh/changed-zh-light.png)
- [Missing snapshot](prototype/widget-refresh/missing-en-light.png)
- [Container unavailable](prototype/widget-refresh/container-unavailable-zh-dark.png)
- [Read failed](prototype/widget-refresh/read-failed-en-dark.png)
- [Unsupported version](prototype/widget-refresh/unsupported-zh-light.png)
- [Recovered](prototype/widget-refresh/recovered-en-light.png)

Browser verification covered all nine URLs and all 15 frames per URL. No frame
had horizontal or vertical overflow. Each typed failure replaced data in 15/15
frames. The four generic data kinds contributed 12 snapshot-age labels while the
three quota sizes retained their independent observation footer. Product labels
contained neither `Host` nor `Unchanged` in those stage scenarios.

Scoped axe WCAG 2A/AA reported zero violations for all four new failure surfaces.
Readable-data states retain one existing `color-contrast` violation on
`.w-note > small` (`Cost remains visibly incomplete`, measured 4.16:1 versus the
required 4.5:1). This node and its colors predate the refresh changes; it is an
explicit out-of-scope baseline risk carried by
`ad-bug-widget-cost-incomplete-contrast`, not a passing accessibility claim. See
[`checks.json`](prototype/widget-refresh/checks.json) and content-bound
[`manifest.json`](prototype/widget-refresh/manifest.json).
Manifest SHA-256: `5cda92a128c6f9cf829389f0c3ae42077d84fef95023e6f9b70bf98a45b0bdfe`.

## Final-surface review boundary

Final review decides whether the framework still matches the settled architecture,
the no-visible-change impact assessment is valid, and the nine existing
specimens/evidence remain applicable to current shared source. It does not
approve implementation or native WidgetKit acceptance. PASS restores this
document's Review cell for decomposition.
