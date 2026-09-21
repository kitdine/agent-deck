---
status: active
created: 2026-09-19
updated: 2026-09-19
---

# Desktop Refresh — Requirements

## Purpose and origin

Make AgentDeck's menu-bar and Widget freshness claims match the data that each
surface can actually read, while increasing useful refresh frequency without
turning unchanged data into repeated Widget reloads.

This topic is the `ad-desktop-refresh` planning entry selected for v0.6.0 in
the [roadmap](../../roadmap.md). Its defect origin is
`ad-bug-widget-refresh-stale`: a development machine's stale sandbox container
prevented the Widget extension from reading the current App Group, while the
Widget continued to render old values as "just updated." That local container
incident did not affect released users, but it exposed general product defects:
Widget reads discard errors, freshness is evaluated against the timeline entry
time instead of the current presentation time, and successful App Group writes
reload every Widget timeline whether displayed content changed or not.

The performance portion of the same origin bug was delivered separately by
[`snapshot-performance`](../snapshot-performance/requirements.md). This topic
uses that result; it does not reopen scan-engine, cache, or aggregation design.
The origin bug remains open until the refresh behavior is implemented and
delivered under this topic's eventual task boundary.

## Supported premises

The current source establishes these premises for design and later review:

- `DesktopRefreshCoordinator` already serializes full refreshes, retains the
  previous snapshot on failure, and publishes successful snapshots to the App
  Group. Its quota-only path can update the subscription section independently.
- `AgentDeckApplicationDelegate` checks every 30 seconds, but full periodic
  refresh remains an opt-in preference and follows the snapshot's
  `next_refresh_at`. A missed due time causes one refresh after the app resumes.
- `AppGroupSnapshotStore.write` atomically replaces the snapshot and then calls
  `reloadAllTimelines()` unconditionally.
- Every Widget provider currently emits one timeline entry. It suppresses all
  snapshot-read errors with `try?` and schedules another read between 15 and 60
  minutes later.
- `AgentDeckWidgetView` constructs `WidgetSurfaceModel` with `entry.date` as
  `now`. The generic freshness ladder therefore cannot advance while that entry
  remains displayed.
- The Widget remains a sandboxed projection reader. It does not read AgentDeck's
  private databases, run the embedded helper, or own a background service.
- WidgetKit scheduling and rendering are controlled by macOS. AgentDeck can
  request a date or reload, but cannot promise an exact on-screen deadline when
  the OS delays or suppresses Widget work.

These are current-code observations, not constraints that architecture must
preserve when a narrower replacement is required.

## Goals

1. Refresh menu-bar data approximately once per minute while the app is running
   and the existing periodic-refresh preference is enabled.
2. Give every Widget a 3–5 minute best-effort opportunity to reread the App
   Group snapshot, independent of whether the host app has new content.
3. Request an earlier Widget update after a successful App Group publication
   only when the projection that a Widget can display has materially changed.
4. Base every freshness statement on the most recent successful observation,
   never on the time a refresh was attempted, a file was reread, or a timeline
   entry was created.
5. Make unavailable, failed, aging, stale, and recovered states observable and
   consistent across menu-bar and Widget surfaces.
6. Coalesce refresh work across timers, manual refresh, wake, provider changes,
   and quota-only updates so one trigger cannot create parallel or catch-up
   storms.

## Refresh contract

### Successful data, attempts, and presentation time

The design keeps three concepts distinct:

- **Successful data time** is the timestamp attached to the last successfully
  decoded and accepted data. It is the only timestamp used to say when values
  were updated or to compute their age.
- **Refresh attempt outcome** records whether the latest attempt is running,
  succeeded, or failed and, for failure, the stable reason category. A failed
  attempt does not advance successful data time.
- **Presentation time** is the actual time at which a surface evaluates the
  data. It advances even when no new snapshot arrives.

Retrying, rereading an unchanged file, writing schedule metadata, or constructing
a new timeline entry must not relabel old values as newly updated. A surface may
retain last-known-good values after a failure only when it preserves their
successful data time and simultaneously exposes the current failure. Otherwise
it renders an explicit unavailable state.

### Menu-bar cadence

The existing `Periodic refresh` preference remains explicit, opt-in, and off by
default. This topic does not silently enable background full refresh for users
who disabled or never enabled it. Startup and manual refresh retain their
existing behavior regardless of that preference.

When the preference is enabled and the app is active:

- the nominal next full-refresh due time is 60 seconds after the preceding full
  refresh reaches a terminal state;
- a bounded scheduler check may add at most 30 seconds of application-controlled
  delay in an isolated acceptance run;
- only one full refresh runs at a time; when work exceeds the nominal interval,
  the next interval begins at terminal completion rather than overlapping work;
- multiple triggers that become due together coalesce into one refresh; and
- a missed due time during sleep or suspension produces one catch-up refresh on
  wake, never one replay per missed interval.

OS suspension, unavailable external clients, and a refresh that itself exceeds
the cadence are reported as measured limitations, not disguised as a passing
one-minute result. The scheduler must not busy-poll to compensate.

### Widget cadence and change-driven reload

Each Widget provider requests another timeline opportunity no earlier than
three minutes and no later than five minutes after its current entry in the
controlled scheduling contract. At that opportunity it rereads the App Group
snapshot and evaluates age against the new actual entry time. macOS may render
later; acceptance records requested and observed timing separately.

A successful App Group publication also compares the new projection with the
last published projection:

- A **material Widget change** is any change to a value, availability, failure,
  partial-state, qualifier, or other semantic field rendered by at least one
  Widget kind.
- Successful data time and scheduling fields alone are not a material value
  change. Time-driven aging and stale transitions belong to the Widget timeline
  rather than an unconditional host reload.
- A material change requests reload only for the affected Widget kinds. A shared
  change may affect all kinds; unchanged projected content requests no reload.
- The comparison is based on a canonical semantic projection, not encoder byte
  order, temporary file identity, or an unrelated private snapshot field.

The App Group snapshot may still be atomically replaced when required for a
new successful data time or contract state. Suppressing a Widget reload is not
permission to skip durable publication or to reuse an invalid projection.

Quota-only publication follows the same rule: changed quota values, availability,
failure, or staleness request the quota Widget's reload; an unchanged probe does
not reload unrelated Widget kinds or retimestamp non-quota data.

### Failure, aging, and recovery

The surface design and contract must cover at least:

- App Group container unavailable;
- snapshot file missing;
- file read or decode failure;
- unsupported projection schema;
- successful read of aging or stale data;
- host refresh failure while a previous snapshot remains readable;
- App Group publication failure after the menu bar accepted fresh data; and
- successful recovery from every transient state above.

A Widget read failure becomes a typed load outcome rather than `nil` from
discarded error information. The Widget must not show previous values with
"just updated" after that failure. A publication failure leaves the previously
published snapshot at its original successful data time so the Widget can age
it truthfully; the menu bar exposes its existing storage-degraded state.

The next successful read or publication clears only the failure it actually
recovered. A quota-only success cannot clear an unrelated full-refresh or
storage failure, and a fresh menu-bar snapshot cannot claim that the Widget
read it until App Group publication succeeds.

### Host absence, sleep, and user actions

When the host app is not running, Widgets continue to render only the last
privacy-bounded App Group projection. Their requested 3–5 minute timeline reads
may update age or expose read failure, but they do not launch the host, run a
scan, access private databases, or invent a new successful data time.

After sleep, wall-clock age is evaluated from the preserved successful data
time. The menu bar performs the single overdue refresh described above when
eligible; Widget timelines do not replay every missed entry. Manual refresh and
provider-switch refresh may run while periodic refresh is disabled, remain
single-flight, and use the same publication/change classification.

## Affected surfaces and contract scope

The topic requires these design documents:

- [`ux/menubar-refresh.md`](ux/menubar-refresh.md) — running, successful,
  retained-data failure, storage failure, overdue/wake, manual, and recovery
  presentation in the menu-bar panel. Existing scan-progress UX remains owned
  by `snapshot-performance` and is reused rather than redesigned.
- [`ux/widget-refresh.md`](ux/widget-refresh.md) — all Widget families' loading,
  unavailable, aging, stale, host-absent, changed, unchanged, and recovered
  states, including rendered specimens and accessibility descriptions.
- [`architecture.md`](architecture.md) — monotonic scheduling and wall-clock
  boundaries, typed Widget load outcomes, semantic projection comparison,
  targeted reload ownership, single-flight/coalescing, App Group publication,
  and interactions with full and quota-only refresh paths.

The architecture must decide whether the existing wire/App Group schema can
express the required successful-time and outcome distinctions or needs an
additive versioned field. Any additive projection remains privacy bounded and
backward compatible. Settings layout and the `Periodic refresh` control are not
redesigned; their existing opt-in semantics are a requirement of this topic.

## Non-goals

- Reopening the delivered snapshot-performance scan engine, cache, aggregation,
  or performance-target design.
- Guaranteeing exact Widget render latency that macOS does not expose or
  control.
- Adding a daemon, LaunchAgent, network listener, file watcher, or background
  login item beyond the existing menu-bar host.
- Letting the Widget read private databases, execute the embedded helper, or
  receive credentials, raw sessions, tool payloads, or unredacted provider data.
- Repairing historical developer-machine App Group or LaunchServices state,
  deleting containers, or changing bundle identifiers.
- Changing quota acquisition semantics, notification policy, price meaning,
  session search, provider switching, or unrelated menu-bar/Widget layout.
- Automatically enabling periodic full refresh or removing the user's refresh
  preference.

## Acceptance boundary

| Scenario | Required result |
| --- | --- |
| Periodic refresh enabled | After a terminal full refresh, the next becomes due at 60 seconds and starts within 90 seconds in the controlled active-app test; no overlap occurs. |
| Periodic refresh disabled | No recurring full refresh starts; startup, manual, provider-switch, and independent quota behavior retain their contracts. |
| Unchanged successful refresh | The App Group publication remains valid, no Widget reload is requested, and no displayed value is relabeled as newly changed. |
| Changed successful refresh | Only affected Widget kinds receive a reload request; menu-bar and Widget retain one coherent successful data time for the data they show. |
| Widget fallback | Every provider requests a timeline opportunity in the 3–5 minute window; actual macOS render latency is recorded separately. |
| Read failure | The failure category is preserved and presented; no old value is called "just updated." |
| Time advances without new data | Aging and stale states cross their specified thresholds using actual wall-clock time, including while the host is absent. |
| Sleep or suspension | One overdue full refresh runs after resume when eligible; no catch-up burst occurs; age includes slept time. |
| Full refresh failure | Last-known-good values, when retained, keep their original successful time and are accompanied by the active failure. |
| App Group write failure | Menu bar reports storage degradation; Widget receives no false success and ages the last readable projection. |
| Quota-only update | Only changed quota semantics can request quota Widget reload; unrelated kinds are not reloaded or retimestamped. |
| Recovery | A succeeding operation clears its matching failure without clearing unrelated degraded state. |

Verification must include deterministic clock/scheduler tests, typed read-error
fixtures, change-classification tests for every Widget kind, single-flight and
wake coalescing tests, App Group publication tests, full macOS App/Shared/Widget
regression suites, and native timing/visibility acceptance. Simulated time proves
logic; it does not replace the native macOS observation of requested versus
actual Widget scheduling.

## Review boundary

Requirements review decides whether the refresh objective, preserved preference,
surface set, failure semantics, cadence, exclusions, and acceptance scenarios
are complete and mutually consistent. It does not approve UX copy, additive
wire fields, scheduling implementation, task decomposition, or product code.
Those decisions belong to their declared documents and later review boundaries.
