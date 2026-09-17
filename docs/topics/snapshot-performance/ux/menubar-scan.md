---
status: active
created: 2026-09-10
updated: 2026-09-11
---

# Menu-bar Scan Interaction Contract

Interaction draft for the revised snapshot-performance scope. Use the existing
shared product prototype and menu-bar hierarchy; do not create a second surface.
`tasks.md` owns readiness. The shared specimen was browser-verified on 2026-09-11;
current review is recorded in [the menu-bar review](../reviews/ux-menubar-scan.md).

## Refresh lifecycle

Opening/requesting refresh immediately shows the existing loading/refreshing
state. A usable prior snapshot stays visible while waiting/checking/importing.
Without prior data, use the loading surface, never zero-valued success cards.
The App waits for both scan domains and then requests read-only snapshot.
Replace the displayed snapshot atomically after validated complete/partial decode.
Closing the popover detaches foreground progress; the worker continues and never
reopens the surface. Reopen attaches with a current state, not a replay of every
old event. Existing refresh scheduling and Widget policies stay unchanged.

## Presentation states

| Stage | English | Chinese |
| --- | --- | --- |
| Waiting | Waiting for current scan | 等待当前扫描 |
| Checking | Checking source files | 检查源文件 |
| Importing | Importing · usage/session committed counts | 正在导入 · 用量/会话已提交数 |
| Statistics | Calculating statistics | 计算统计数据 |
| Failed with prior data | Existing refresh-failure/retry presentation | 沿用刷新失败与重试呈现 |
| Complete | Existing updated-time/snapshot presentation | 沿用更新时间与快照呈现 |

Unknown totals are omitted. Show independent domain failure truthfully; a completed
usage domain cannot conceal session failure. Progress contains aggregate safe
counts only. A timeout may show prior data under existing fallback rules, never
claim a successful fresh result. No new refresh interval or cancellation control.

## Accessibility and specimen acceptance

Use an accessible status/live region with coalesced stage announcements; do not
announce every file update or move keyboard focus on progress. Preserve existing
refresh control, reading order and failure access. Both languages and themes must
fit 420 and 280 widths without losing stage or error meaning.

## Verified shared specimen

Open the existing product prototype with `?scan=waiting`. Its stage now selects
waiting/checking/importing/usage-complete/statistics/completed/partial/failed,
previous snapshot presence, language, theme and width. Next stage is deterministic;
play demonstrates timed progress. Stage controls are outside the product panel.
The worker model belongs to App, so toggling the menu-bar item closes/reopens the
popover without cancelling the model. No real worker or user state is accessed.

- [First use](prototype/scan/menubar-first-zh-dark-420.png): no fabricated amounts,
  unknown totals omitted, existing loading surface used.
- [Import, Chinese 280](prototype/scan/menubar-import-zh-dark-280.png): committed
  domain counts above the prior snapshot; visible content fits the narrow width.
- [Statistics](prototype/scan/menubar-statistics-en-light-420.png): both indexes
  complete but prior snapshot remains until statistics finish.
- [Failure, English 280](prototype/scan/menubar-failed-en-light-280.png): separate
  domain outcomes, prior snapshot retained and retry available.

The partial specimen illustrates successful usage with failed sessions and no new
validated snapshot publication: retain the previous snapshot, or show unavailable
when none exists. It does not manufacture a cross-database success or a new zero
snapshot. Existing generic partial-snapshot rendering remains available on the
shared prototype's original state surface. Both snapshot generations use the
existing fixed synthetic fixture; their values are not production measurements.

Browser checks: 14 independent interaction assertions per language (28 total),
including first use, close/background advance/reopen, statistics-before-publication,
retry retaining the new prior snapshot, independent failure and stable focus.
A 128-combination matrix covered eight phases × two languages × two themes × two
widths × prior/no prior; no new progress-region overflow or fabricated initial
cards. Screenshots were visually inspected against the existing surface.

Actual browser Accessibility.getFullAXTree reports one status with live=polite
and atomic=true; counts are outside it. New scan-region axe WCAG 2A/AA checks in
both themes report 0 violations/0 incomplete. Full existing panel audit reports
0 violations but one incomplete aria-prohibited-attr on the unchanged calendar;
that pre-existing region is not claimed fully audited by this design work.
Native VoiceOver order/announcement timing, Dynamic Type, Swift streaming and
actual close/reconnect behavior remain task 3 acceptance, not browser evidence.

[Specimen manifest](prototype/scan/manifest.json) and
[browser check results](prototype/scan/checks.json) bind source and images.
Manifest SHA-256: `cf26b5d5f7afd30bf57eca9f4e7fa6c373eaec0b786e2be5eb02c7de973def8f`.
