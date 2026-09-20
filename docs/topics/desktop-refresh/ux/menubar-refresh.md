---
status: active
created: 2026-09-19
updated: 2026-09-19
---

# Menu-Bar Refresh — Surface Framework

## Purpose and stage

This framework defines how the existing AgentDeck menu-bar popover presents a
refresh attempt without losing the successful-data time or replacing readable
prior data. It implements the surface boundary in
[`requirements.md`](../requirements.md); it does not choose scheduler, wire, or
App Group implementation.

This is the stage-3 framework design. Architecture must provision the data named
below, then this document returns for final-surface reconciliation before task
decomposition. The shared [product prototype](../../../../prototype/README.md)
is the visual authority; no second menu-bar surface is introduced.

## Inherited contracts

- Preserve the existing 420 pt and 280 pt popover, five tabs, client/period
  filters, provider footer, semantic colors, English/Chinese localization, and
  Settings-owned periodic-refresh preference.
- Preserve the existing surface-plus-qualifiers rule: readable prior data stays
  visible during refresh and after a failed refresh.
- Reuse `snapshot-performance`'s scan live region and detailed scan stages. This
  document adds no competing progress timeline and does not replay scan events.
- Keep successful data time, refresh attempt outcome, and presentation time
  independent. A failed attempt never advances the successful time.
- Keep Widget/App Group publication separate from menu-bar snapshot acceptance.

## Information hierarchy

The existing hierarchy is sufficient and stays fixed:

1. **Header:** brand, successful-data age, and the refresh action/attempt state.
2. **Scan status, when active:** the existing coalesced live region below the
   header.
3. **Filters and hero:** readable prior or newly accepted data.
4. **Notice strip:** refresh failure, Widget publication failure, partial data,
   health, and other existing causes in the content scroll region.
5. **Panels and provider footer:** unchanged ownership and layout.

Data age remains visible in the header while an attempt runs or fails. The
refresh control may change icon, label, enabled state, and action, but it never
replaces the age label. At 280 pt the control's visible text remains visually
hidden under the existing narrow rule; its full accessible label remains. The
failure action therefore becomes a clickable warning icon at 280 pt, with the
full failure sentence still visible in the notice strip below.

## Presentation model

The surface derives one successful-data presentation and one attempt
presentation. They are not one mutually exclusive enum.

| Successful data | Attempt/publication | Header | Content | Notice |
| --- | --- | --- | --- | --- |
| None | Initial/running | No age; spinner `Refreshing…` | Existing loading surface | Existing scan status when available |
| Readable | Idle | Actual relative age + `Refresh` | Current snapshot | Existing independent notices only |
| Readable | Running | Preserved relative age + spinner `Refreshing…` | Previous snapshot remains interactive except refresh action | Existing scan status when available |
| Readable | Full refresh failed | Preserved relative age + warning `Retry` | Previous snapshot remains | `Refresh failed · showing previous data` |
| None | Full refresh failed | No age + warning `Retry` | No-data error surface | `First refresh failed · no data is available yet` |
| Fresh menu-bar data | App Group publication failed | `Updated just now` + ordinary `Refresh` | Fresh menu-bar snapshot | `Menu-bar data is current · Widgets may be out of date` |
| Fresh | Succeeded/recovered | `Updated just now` + transient check `Updated` | New snapshot | No refresh-failure notice |

The running row also covers a catch-up after sleep. The UI does not add a
separate `wake` badge or sentence: the old successful-data age plus the running
attempt already tells the complete truth without requiring a new trigger field.
The scheduler contract, not a visual label, proves that only one catch-up runs.

`Updated just now` appears only after a new snapshot was successfully decoded
and accepted by the menu bar. App Group publication failure does not revoke that
menu-bar success; its notice narrows the failure to Widgets.

## Copy contract

| Meaning | English | Chinese |
| --- | --- | --- |
| Running action | `Refreshing…` | `刷新中…` |
| Successful action | `Updated` | `已更新` |
| Failed action | `Retry` with accessible prefix `Refresh failed` | `重试`，无障碍前缀为`刷新失败` |
| Current age | `Updated just now` / `Updated <relative>` | `刚刚更新` / `<相对时间>更新` |
| Aged data | `Last updated <relative>` | `上次更新于<相对时间>` |
| Retained-data failure | `Refresh failed · showing previous data` | `刷新失败 · 正在显示上次数据` |
| First-use failure | `First refresh failed · no data is available yet` | `首次刷新失败 · 还没有可显示的数据` |
| First-use body | `No data available yet` | `还没有可显示的数据` |
| Widget publication failure | `Menu-bar data is current · Widgets may be out of date` | `菜单栏数据已更新 · 小组件可能仍是旧数据` |

The notice says what failed and what remains trustworthy. It does not use
`Some data unavailable` for a failed full refresh whose retained snapshot was
complete, and it does not use `Data could not be read` for an App Group write
failure after the menu bar successfully accepted data.

## Notice composition and priority

Refresh notices use the existing full-width notice strip and wrap at the narrow
bound. Their order is:

1. permanent/root-cause notices such as schema refusal;
2. refresh or Widget-publication outcome;
3. independent health and session warnings;
4. data-domain partial notices.

Only one refresh-outcome row appears. A retained-data failure may coexist with
health or partial data because those describe different facts. A successful
retry removes only the matching refresh row; unrelated warnings remain.

## Interaction sequences

### Manual or periodic refresh with prior data

1. The refresh action becomes disabled and shows a spinner. Keyboard focus does
   not move.
2. The successful-data age and all readable content remain visible.
3. Detailed scan stages, when available, update in the existing live region.
4. Success atomically replaces the snapshot, resets the age, and shows the
   transient `Updated` action before returning to `Refresh`.
5. Failure leaves the prior snapshot and age unchanged, adds the retained-data
   notice, and changes the action to `Retry`.

### First refresh

The loading surface contains no fabricated zeros. On failure it becomes the
first-use error surface with one notice and the same header Retry action. Retry
is single-flight. Its success replaces the error surface atomically; the error
copy does not linger as a toast or historical log.

### App Group publication failure

The menu-bar snapshot is already successful and remains fully interactive. The
Widget-publication notice appears once in the content strip. The ordinary manual
refresh action is the retry path; no second Widget-only control is introduced.
A later successful publication clears this notice without changing unrelated
health or partial notices.

### Sleep and resume

On resume, the displayed relative age includes slept time. If a catch-up is
eligible, the ordinary running presentation begins once. No missed intervals are
listed or replayed. If the app cannot begin immediately, the aged timestamp is
still the truthful surface; no fabricated `Refreshing…` appears before work is
accepted.

## Data requirements

| Element | Required data | Owner/disposition for architecture |
| --- | --- | --- |
| Header age | Timestamp of last successfully decoded and accepted menu-bar snapshot; presentation clock | Existing snapshot `generated_at` is the candidate; architecture verifies its exact semantics |
| Refresh action | Attempt phase: idle, running, succeeded, failed | Existing coordinator state is the candidate; architecture maps transient success and terminal failure |
| Retained-data failure | Whether a readable prior snapshot exists plus full-refresh failure category | Existing coordinator `degraded(previous, issue)` is the candidate |
| First-use failure | No prior snapshot plus full-refresh failure category | Existing coordinator state is the candidate |
| Widget publication failure | App Group publication outcome distinct from menu-bar decode/acceptance | Existing `storageUnavailable` issue is the candidate; architecture must preserve the fresh menu-bar snapshot |
| Scan live region | Existing scan stage and safe committed counts | Reuse `snapshot-performance`; no new field |
| Notice clearing | Which terminal operation recovered which issue | Architecture defines generation-safe ownership; a quota-only success cannot clear full-refresh/storage failure |

No UI element needs the number of missed intervals, a wake trigger label, a
Widget render time, or a second freshness timestamp. If architecture cannot
derive a row without ambiguity, it must provision a typed additive field or
return here with a stated refusal; the surface must not infer it from timers.

## Accessibility and motion

- Successful-data age is readable text, not color, and is not a live region;
  minute-by-minute age changes must not repeatedly interrupt VoiceOver.
- Attempt status uses one polite, atomic announcement. Detailed scan updates stay
  in their existing coalesced live region; the two regions must not announce the
  same phrase twice.
- Starting, completing, or failing a refresh never moves keyboard focus. Retry
  remains in the same logical header position and carries the complete accessible
  label `Refresh failed. Retry` / `刷新失败。重试`.
- Spinner motion stops under Reduce Motion; the label and disabled state still
  communicate progress.
- Notices include icon and text, wrap rather than truncate at 280 pt and large
  Dynamic Type, and retain contrast in light, dark, and Increase Contrast modes.
- Closing the popover does not cancel accepted work. Reopening shows the current
  attempt state, not replayed announcements.

## Shared prototype specimens

The shared prototype adds a refresh-state axis orthogonal to usage and quota
state. Stage controls are design tooling, not product UI. These framework
specimens are required:

| Specimen | URL state | What it proves |
| --- | --- | --- |
| Running with prior data | `refresh=refreshing&lang=en&theme=light&width=420&tab=usage` | Age remains beside disabled running action; prior content remains |
| Wake catch-up, narrow | `refresh=wake&lang=zh&theme=dark&width=280&tab=usage` | Slept age remains truthful; no extra wake badge; header fits |
| Failure with prior data | `refresh=failed&lang=en&theme=light&width=280&tab=usage` | Compact Retry plus retained-data notice; no false partial claim |
| First-use failure | `refresh=firstFailure&lang=zh&theme=dark&width=420&tab=usage` | No fabricated cards or successful age; one actionable error |
| Widget publication failure | `refresh=storageFailed&lang=en&theme=dark&width=420&tab=usage` | Fresh menu-bar time and Widget-specific notice coexist |
| Recovery | `refresh=recovered&lang=zh&theme=light&width=280&tab=usage` | New data time and transient success clear only refresh failure |

Browser screenshots and their manifest live under
`ux/prototype/menubar-refresh/`. They establish hierarchy, copy, wrapping,
keyboard-visible controls, and browser accessibility structure. They do not
establish native typography, VoiceOver timing, Dynamic Type, AppKit focus,
WidgetKit scheduling, or real sleep/wake behavior.

- [Running with prior data](prototype/menubar-refresh/running-prior-en-light-420.png)
- [Wake catch-up, narrow](prototype/menubar-refresh/wake-catchup-zh-dark-280.png)
- [Failure with prior data](prototype/menubar-refresh/failed-prior-en-light-280.png)
- [First-use failure](prototype/menubar-refresh/first-failure-zh-dark-420.png)
- [Widget publication failure](prototype/menubar-refresh/storage-failure-en-dark-420.png)
- [Recovery, narrow](prototype/menubar-refresh/recovered-zh-light-280.png)

Browser verification covered all six URLs. Panel, header, and scroll content had
equal client/scroll widths at 420 and 280 pt. Every state exposed exactly one
polite live region. Keyboard Retry preserved focus through refreshing and
success, and the refresh-failure notice cleared. Scoped axe WCAG 2A/AA reported
zero violations in all six states. Five data specimens retain the existing
`aria-prohibited-attr` incomplete on the unchanged 90-day calendar; the first-use
error surface has zero incomplete checks. See
[`checks.json`](prototype/menubar-refresh/checks.json) and the content-bound
[`manifest.json`](prototype/menubar-refresh/manifest.json).
Manifest SHA-256: `64083a6a4bad190debac0c96547d3648e59b83a3b0bf11c7115ca634783a5752`.

## Framework review boundary

Framework review decides whether all requirements states have truthful hierarchy,
copy, composition, interaction, accessibility, and data requirements. It does
not approve new wire fields or implementation. After architecture settles every
data requirement, this document returns for final-surface reconciliation and a
new or explicitly reused applicable review before task decomposition.
