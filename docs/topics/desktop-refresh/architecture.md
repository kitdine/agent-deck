---
status: active
created: 2026-09-19
updated: 2026-09-19
---

# Desktop Refresh — Architecture

## Purpose and status

This document provisions the reviewed [requirements](requirements.md),
[menu-bar framework](ux/menubar-refresh.md), and
[Widget framework](ux/widget-refresh.md). It specifies scheduling,
attempt/publication state, App Group atomicity, kind-scoped semantic comparison,
typed Widget load outcomes, clocks, concurrency, privacy, compatibility, and
verification. It is design only; this stage changes no production code.

## Verified current system

Current source establishes the starting constraints:

- `internal/desktop.Service.Build` stamps `generated_at` at build start and
  `next_refresh_at` five minutes later through `RefreshInterval`.
- `AgentDeckApplicationDelegate.startPeriodicRefresh()` wakes every 30 seconds,
  independently invokes quota refresh when enabled, and starts a full refresh
  only when the opt-in periodic preference is enabled and wire
  `next_refresh_at` is due.
- `DesktopRefreshCoordinator` owns one cancellable full-refresh task, a
  generation counter, latest decoded snapshot, scan progress, quota-only
  publication, and App Group writes.
- `AppGroupSnapshotStore.write` writes and verifies a private temporary file,
  atomically renames it, verifies the destination, then unconditionally calls
  `WidgetCenter.reloadAllTimelines()`.
- `AppGroupDesktopSnapshotV1` already carries successful snapshot time,
  next-refresh hint, partial/issues, all Widget usage data, and subscription
  observations. The Widget decoder intentionally ignores provider, sessions,
  and health.
- `WidgetSnapshotLoader` turns every read error into `nil`; an entry cannot
  distinguish missing, container, decode, or schema failures.
- Each Widget provider emits one entry at `Date()` and clamps its next request
  to 15–60 minutes. Views evaluate age against `entry.date`, so age advances
  only when WidgetKit asks for another entry.
- `DesktopPresentationState` treats every `.degraded(previous, issue)` as stale.
  That is wrong for `.storageUnavailable`: menu-bar data succeeded and only
  Widget publication failed.
- The explicit `--reload-widget-timelines` app mode already knows all five kind
  identifiers. It remains a diagnostic/acceptance entry point.

## Decisions

| ID | Decision |
| --- | --- |
| A1 | Full automatic refresh is scheduled by the app from terminal completion using an injected monotonic clock. The opt-in preference remains off by default. |
| A2 | Go `next_refresh_at` stays wire-v1 compatibility data and changes to a one-minute host suggestion. It no longer owns the app deadline. |
| A3 | One coordinator owns a full lane and a quota lane. Each lane is single-flight; cross-lane requests join or create at most one priority-aware follow-up under the explicit arbitration table below. |
| A4 | Successful menu-bar snapshot, full-refresh attempt, and Widget publication outcome are independent observable states. |
| A5 | A serial `WidgetSnapshotPublisher` advances an admission barrier before suspension, writes every still-current projection, compares kind-specific semantic projections, and reloads only affected kinds. |
| A6 | `generated_at`, `next_refresh_at`, JSON byte order, temporary identity, and non-rendered fields do not alone trigger Widget reload. |
| A7 | Widget timelines clamp the hint to 3–5 minutes and default to four minutes when absent/malformed. Each invocation emits one entry at its actual time. |
| A8 | Widget entries carry a typed load outcome: placeholder, loaded, missing, container unavailable, unreadable, or unsupported version. |
| A9 | No App Group field or schema bump is required. Attempt/publication and load-failure states are process-local. |
| A10 | Quota-only publication retains full snapshot `generated_at`; quota footer freshness remains the oldest displayed quota observation/reason. |
| A11 | Host absence is not probed or persisted. A readable projection ages; a missing/failing projection keeps its actual cause. |
| A12 | Clocks, reloaders, storage operations, and wake events are injected for deterministic tests; native timing remains separate acceptance. |

## Component model

```text
AgentDeckApplicationDelegate
  └─ DesktopRefreshScheduler (monotonic deadlines, wake/preferences)
       └─ DesktopRefreshCoordinator (coalescing, generation barrier)
            ├─ DesktopHost (scan + read-only snapshot)
            ├─ DesktopQuotaRefreshing (independent schedule/probe)
            └─ WidgetSnapshotPublisher actor
                 ├─ AppGroupSnapshotStore (private atomic bytes)
                 ├─ WidgetSemanticDiff (affected kind set)
                 └─ WidgetTimelineReloader (WidgetCenter adapter)

WidgetKit provider
  └─ WidgetSnapshotLoader
       ├─ WidgetSnapshotReader
       └─ WidgetLoadOutcome
            └─ AgentDeckWidgetEntry (actual invocation Date)
                 └─ WidgetSurfaceModel / AgentDeckWidgetView
```

The scheduler owns *when* a full refresh is requested. The coordinator owns
*which request runs* and user-facing attempt/publication state. The publisher
owns *which projection reaches the App Group* and *which kinds are notified*.
The Widget extension owns its local read outcome and presentation only.

## A1–A3: scheduling and request coalescing

### Full-refresh schedule

Introduce `DesktopRefreshScheduler` with injected monotonic clock, wake source,
and request closure:

1. Startup requests one full refresh independent of the periodic preference.
2. With periodic refresh enabled, a terminal full attempt sets its next deadline
   to `completion + 60 seconds`.
3. The evaluator wakes on the existing bounded 30-second tick and on macOS wake.
   Application-controlled delay is therefore at most 30 seconds.
4. A missed deadline produces one request, never one per elapsed interval.
5. Disabling clears the full deadline. Re-enabling schedules from the current
   terminal state and never backfills disabled time.
6. Startup, manual, provider-switch, and quota behavior remain available while
   the preference is off.

Scheduling uses monotonic duration. Wall-clock timestamps remain for display and
wire compatibility; clock/time-zone changes cannot create a burst. Go
`RefreshInterval` becomes one minute so `next_refresh_at` remains a truthful
host hint for older consumers. The new app scheduler does not use it as its
authoritative terminal deadline. The localized Settings note must describe the
approximately-one-minute behavior; control and layout do not change.

### Coordinator request policy

All full triggers enter:

```text
requestFullRefresh(trigger: startup | periodic | wake | manual | providerSwitch)
```

- With no active generation, start one.
- Periodic, wake, and startup requests during an active generation join/no-op.
- Manual/provider-switch during an active generation set one follow-up flag;
  duplicates coalesce.
- A provider-switch follow-up cannot be satisfied by a refresh begun before the
  switch completed.
- No trigger runs a parallel full refresh or cancels useful work merely to start
  the same scan again.
- At terminal completion, run the one follow-up or notify the scheduler.

The existing full generation guard remains. Publication has its own monotonically
increasing generation so an older full/quota write cannot overwrite or report
failure for a newer accepted projection.

### Full/quota bidirectional arbitration

The coordinator owns one `QuotaLane` in addition to the full lane:

```text
QuotaLane
  idle
  running(operationID, owner: standalone | full(fullGeneration), priority,
          joiners, publicationMode)
  pending(priority: periodic | manual)   // at most one bit of follow-up work
```

`manual` has higher pending priority than `periodic`, but it does not preempt an
in-flight probe: a probe already running now is fresh enough to satisfy a manual
joiner. `publicationMode` is fixed when the operation starts:

- `standalone` performs the quota-only publication transaction;
- `deferToFull` returns the persisted/fetched quota result to its owning full
  generation, which publishes only the final full snapshot.

| Arrival/current state | Required action |
| --- | --- |
| Quota request; both lanes idle | Start one standalone quota operation |
| Quota request; quota operation running | Join the same operation; do not enqueue or repeat side effects |
| Full reaches quota phase; quota operation already running standalone | Join and await that operation before scan/snapshot; standalone publication may finish, then full later publishes its newer whole snapshot |
| Full reaches quota phase; quota lane idle | Start one `deferToFull` operation owned by that full generation |
| Quota request; full active but its quota phase has not begun/is running | Join the full-owned operation (or its reserved operation when the actor next advances it) |
| Quota request; full active and its quota phase already completed | Set one pending quota operation for after the full terminal state; repeated periodic ticks collapse, manual upgrades pending priority |
| Full/manual/provider-switch arrives while standalone quota runs | Full generation starts but blocks at its quota phase by joining the operation; no second probe |
| Full follow-up and pending quota both exist at full terminal | Start the full follow-up first; pending quota joins that generation's quota phase, with manual priority preserved |

There is never more than one running quota operation and one pending quota bit.
A suspended call at probe, notification delivery, acknowledgement, fetch, or
publication remains `running`; a timer tick cannot enter those side effects again.

Each `QuotaOperationID` has one owner task. Only that owner may call, in order:

```text
probe once → deliver due alerts once → acknowledge delivered IDs once
           → fetch subscription once → publish at most once
```

Joiners await the same terminal result and never replay a step. There is no
automatic in-operation retry; a later accepted operation may retry an unacked
durable alert under the quota subsystem's existing stable alert-ID/deduplication
contract. This is exactly-once invocation per `QuotaOperationID`, not an
unsupported promise of crash-proof exactly-once OS notification delivery between
`deliver` and durable `ack`.

At quota terminal state, resolve all joiners once. A joined full generation
continues to scan/snapshot. A pending operation starts immediately only when no
full generation owns the quota boundary; otherwise it is consumed by the full
follow-up rule above. Failure follows the same wake-up rules and does not create
an implicit retry/catch-up queue.

Publication generation is allocated centrally only after a quota result passes
the latest-full-generation recheck and is ready to write. Standalone quota,
full final publication, and pending quota therefore share one strictly ordered
publication sequence; a request's arrival order alone never reserves a write.

## A4: independent menu-bar state

The coordinator exposes three independent values:

```text
latestSnapshot: DesktopWireEnvelopeV1?
fullAttempt: idle | running | succeeded(completedAt) | failed(issue, completedAt)
widgetPublication: neverPublished | succeeded(generation)
                 | failedBeforeCommit(generation, issue)
                 | indeterminateAfterCommit(generation, issue)
```

`fullAttempt.succeeded` is transient and clears after 1.6 seconds, matching the
reviewed shared prototype; its timer is cancelled/replaced by the next attempt. Failure persists
until the next accepted full attempt reaches its own result. Publication failure
persists until a newer projection publishes successfully. Both definite and
indeterminate publication failures use the reviewed “Widgets may be out of date”
surface; the distinction controls recovery and evidence, not extra user copy.

| State | Surface/qualifiers |
| --- | --- |
| No snapshot + running | loading surface; running action; no age |
| Snapshot + running | data surface; `stale`; real snapshot age |
| Snapshot + full failure | data surface; `stale` + issue; original age |
| No snapshot + full failure | error surface; typed retry action |
| Snapshot + full success + publish success | data surface; current age; transient success action |
| Snapshot + publication failure | data surface; **not stale**; Widget-publication notice; no generic read-failure claim |

Publication failure is not `.failing` and does not badge the menu-bar item. It
is a popover notice because menu-bar data succeeded. `.storageUnavailable` is
narrowed/renamed internally to Widget publication failure or removed from
`fullAttempt`; it must never make fresh menu-bar data stale.

The ViewModel derives header action, age, notice, and one polite/atomic refresh
announcement. Existing scan progress remains the detailed live region and must
not be duplicated.

## A4: full-refresh transaction

One accepted full generation:

1. Marks `fullAttempt = running`, preserving `latestSnapshot`.
2. Performs manual quota probe when applicable, without an intermediate quota
   App Group publication inside this full transaction.
3. Runs shared scan and read-only snapshot through `DesktopHost`.
4. Validates/decodes the complete wire result.
5. If current, sets `latestSnapshot` and `fullAttempt = succeeded`.
6. Increments publication generation and publishes the complete projection.
7. Records publication success/failure independently; failure does not roll back
   menu data or its successful time.
8. Completes follow-up/scheduler handling.

Helper, scan, timeout, and wire failures terminate before step 5, set
`fullAttempt = failed`, retain prior `latestSnapshot`, and do not publish.

## A4/A10: quota-only transaction

Quota retains its independent preference and interval; it does not inherit the
one-minute full cadence.

1. Enter the quota lane and either start or join exactly as the bidirectional
   arbitration table specifies.
2. The owner alone probes, delivers/acks alerts, and fetches subscription once.
3. Recheck full generation and `latestSnapshot` after every `await`.
4. A `deferToFull` owner returns to its full generation without an intermediate
   write. A current standalone owner replaces only subscription, then obtains
   the next publication generation.
5. Publish at most once through the serial publisher; wake all joiners with the
   same terminal result, then apply the one-pending rule.

The splice keeps full snapshot `generated_at` and cannot clear
`fullAttempt.failed`. A successful whole-file publish may clear matching Widget
publication failure. Only material subscription difference reloads quota.

## A5: atomic Widget publisher

Split bytes from reload policy:

```text
AppGroupSnapshotStore
  readExisting() -> known(snapshot) | unknown(fixedReason)
  writeAtomically(snapshot) throws

AppGroupSnapshotBytes
  readBounded(url, maximumBytes: 8 MiB) -> Data | typed failure

WidgetSnapshotPublisher actor
  publish(snapshot, generation)
    -> published(affectedKinds)
     | superseded
     | failedBeforeCommit(retainedVerifiedGeneration, issue)
     | indeterminateAfterCommit(attemptedGeneration, issue)

WidgetTimelineReloader
  reload(kinds: Set<AppGroupWidgetKind>)
```

The actor tracks an admission barrier `highestAcceptedGeneration` separately
from disk fact `lastVerifiedGeneration`. At the synchronous start of
`publish(snapshot, generation)`, before any file access, allocation, suspension,
or write:

```text
guard generation > highestAcceptedGeneration else return superseded
highestAcceptedGeneration = generation
```

Generation IDs are unique, so equality is also superseded. The admission
barrier never rolls back, including after `failedBeforeCommit`; this prevents an
older queued request from writing or clearing the newer user-visible failure.
`lastVerifiedGeneration` changes only after complete destination verification
and durability success.

If publisher work has any suspension point, it rechecks
`generation == highestAcceptedGeneration` after each resume and immediately
before rename. A superseded operation removes only its own temp artifact and
returns without write, reload, baseline mutation, or user-state mutation. Actor
serialization alone is not treated as mailbox ordering or stale-write proof.

For an accepted generation:

1. Read/decode previous projection through the shared bounded reader for
   comparison. Missing/invalid/unsafe/oversized old content yields an unknown
   semantic baseline and does not block a valid new write.
2. Derive old/new semantic projections and affected kinds.
3. Encode sorted JSON to a private `0600` temp file in the `0700` directory;
   apply the same 8 MiB bound and verify regular type/no symlink.
4. Recheck the admission barrier, then atomically rename the verified temp file
   over the destination. Successful rename is the publication **commit point**;
   the admission barrier was already advanced at acceptance.
5. Verify the destination and synchronize file/directory under the durability
   contract.
6. On complete success, set `lastVerifiedGeneration`, retain the new semantic
   baseline, and reload affected kinds.

Every accepted snapshot is written even when the kind set is empty, because
successful time and recovery state advance. Empty means no immediate reload;
fallback timeline sees the new file. The explicit CLI reload mode remains
all-kind; normal publication uses the affected set.

Failure semantics are divided at the commit point:

- Before rename succeeds, remove the temp file best-effort, return
  `failedBeforeCommit`, keep the previous verified disk/baseline and
  `lastVerifiedGeneration`, keep the newer admission barrier in force, and
  request no reload. Older queued/incoming generations are superseded.
- After rename succeeds, any destination verification or file/directory sync
  failure returns `indeterminateAfterCommit`. Do not attempt an unprovable
  rollback, do not reload, keep `lastVerifiedGeneration` unchanged, set the
  in-memory semantic baseline to `unknown`, and keep
  `highestAcceptedGeneration = attemptedGeneration` so older work cannot write.
- With an unknown baseline, the next newer successful publication treats all
  five kinds as affected, establishes a new verified baseline/generation, reloads
  all five once, and clears `widgetPublication.indeterminateAfterCommit`.
- A Widget fallback timeline may independently read new, old, or unreadable
  content during the indeterminate interval; its typed loader reports only what
  it actually observes.

On process restart there are no surviving older tasks. The publisher reads the
current destination: a valid supported projection becomes the initial comparison
baseline; missing/malformed/unsupported content yields unknown, so the next
success reloads all five. Orphan temp cleanup is fixed-name/prefix, bounded,
best-effort and logged by fixed code. Cleanup failure never changes a verified
publication result and never deletes the destination.

### Shared host/Widget bounded bytes

`AppGroupSnapshotStore.readExisting()` and `WidgetSnapshotReader.read()` must
both call the same `AppGroupSnapshotBytes.readBounded` helper in the shared
target. Neither may retain `Data(contentsOf:)` or another unbounded convenience
read. The helper owns the exact 8 MiB limit and the full
`lstat → O_NOFOLLOW open → fstat → N+1 read → post-fstat` sequence specified
below.

Host comparison maps results as follows:

| Bounded/decode result | `readExisting()` semantic baseline | Effect on new valid publication |
| --- | --- | --- |
| Valid supported projection | `known(snapshot)` | Compare normally |
| Missing | `unknown(missing)` | Continue write; successful publish reloads all five |
| Symlink/non-regular | `unknown(unsafeFile)` | Continue only through safe new temp/atomic replace; reload all five on success |
| N+1/growth beyond N | `unknown(oversized)` | Never retain oversized bytes; continue valid bounded write; reload all five |
| I/O/decode malformed | `unknown(unreadable)` | Continue valid write; reload all five |
| Unsupported prior schema | `unknown(unsupportedVersion)` | Continue valid v1 write; reload all five |

An unknown old baseline is not a publication failure and never prevents a valid
new projection from repairing disk state. Fixed reason is available to tests and
privacy-safe diagnostics only; it does not add menu-bar or Widget copy.

## A5–A6: semantic kind comparison

Define equatable canonical views of only fields each kind renders. They are
transient shared-host values, not another cache.

| Kind | Included semantic inputs |
| --- | --- |
| `magnitude` | all client/period totals, token components, sessions, pricing completeness, daily series, average/peak/cache figures |
| `composition` | all client/period model rows, shares, token mix, client subtotals, pricing completeness |
| `trust` | all client/provider attribution-quality amounts, counts/shares, coverage and unpriced IDs |
| `rhythm` | all scoped active-day, busiest/quietest, 7×24 and 90-day fields |
| `quota` | complete subscription client/window/reset/failure/stale/reason projection |

Schema support, usage availability, and top-level `partial` changes affect every
kind whose surface/qualifier changes; whole-projection incompatibility affects
all five. Compare all possible configurations, not only a selected intent.

Exclude `generated_at`, `next_refresh_at`, encoding order, file metadata,
provider/session/health fields the Widget cannot render, and process-local state.
If previous projection is absent/malformed/unsupported, successful new
publication reloads all five kinds so unavailable cards recover.

## A7: typed Widget load outcome

Replace optional snapshot plus `isPlaceholder` with:

```text
WidgetLoadOutcome
  placeholder
  loaded(WidgetDesktopSnapshotV1)
  failed(WidgetLoadFailure)

WidgetLoadFailure
  missing
  containerUnavailable
  unreadable(category: io | decode | oversized | unsafeFile)
  unsupportedSchemaVersion(found: Int)
```

The UI combines I/O and decode under reviewed unreadable copy, while tests and
fixed-code diagnostics retain the category. No path, JSON fragment, credential,
or dynamic error prose enters UI/logs.

The shared host/Widget byte reader applies one constant:

```text
AppGroupSnapshotLimits.maximumBytes = 8 * 1024 * 1024  // 8 MiB
```

The projection is deliberately much smaller in scope than the 64 MiB helper
snapshot transport: it contains bounded presentation aggregates, not raw source
records. Eight MiB is the fixed compatibility/security contract, not a measured
current file-size claim.

Bounded read order is mandatory for both host comparison and Widget loading:

1. `lstat`/resource preflight: existing regular file, not symlink, advertised
   size ≤ N; missing maps to `missing`, unsafe type to `unsafeFile`, size > N to
   `oversized`.
2. Open with `O_RDONLY | O_NOFOLLOW`, then `fstat` the descriptor and recheck
   regular type and size ≤ N. This closes path replacement between preflight and
   open.
3. Read from the descriptor in bounded chunks into at most N+1 bytes. Observing
   byte N+1 is `oversized`; never allocate/read the entire file first.
4. Post-read `fstat`; growth beyond N is `oversized`. Atomic replacement after
   open is safe because the descriptor names one inode; mutation of that inode
   during read is rejected when size/bytes disagree.
5. Decode the minimal schema header, reject version mismatch, then decode the
   supported full payload.

Exactly N bytes are admitted to decoding; N+1 is rejected before JSON decoding.
The writer applies the same bound to encoded bytes before creating/renaming a
temp file; an oversized new projection is `failedBeforeCommit`.

Host `readExisting()` maps every non-valid result to unknown baseline as tabled
above. Widget file-not-found maps to missing; nil App Group to container
unavailable; bounded I/O/JSON/oversized/unsafe-file failures remain distinct
fixed diagnostic codes but share the reviewed unreadable UI. Logs may include
fixed code, observed byte count capped at N+1, and the constant limit—never path
or content.

`WidgetSurfaceModel` derives placeholder/data/typed unavailable from the outcome
instead of guessing from nil. All kinds/families use the frame-level failure
title/footer; larger sizes add no diagnostics. Quota no-data/reason applies only
after a projection loaded successfully.

## A7–A8: Widget timeline and clocks

`WidgetTimelinePolicy` becomes:

```text
minimum = 3 minutes
default = 4 minutes
maximum = 5 minutes
```

Clamp a parseable hint to `[now + 3m, now + 5m]`; absent/malformed/failed-load
uses `now + 4m`; past becomes `now + 3m`. All providers emit one entry.

Entry `date` is the actual invocation time from an injected date provider. Views
use it as presentation `now`, so each granted invocation recomputes age. The bug
is closed by 3–5 minute opportunity plus actual entry time, not a view timer or
future fabricated data.

| Clock | Meaning |
| --- | --- |
| Snapshot `generated_at` | Last accepted full menu-bar snapshot; generic Widget age |
| Quota `observed_at` | Vendor observation age; oldest displayed owns quota footer |
| Widget entry `date` | Actual timeline invocation/presentation time |
| App monotonic deadline | Full schedule only; never displayed/persisted |
| Wall-clock `next_refresh_at` | Wire-v1 hint; clamped by Widget, not app deadline |

Host absence adds no field. Sleep may jump directly to old. Requested/actual
WidgetKit times remain separate native evidence.

## A9: compatibility and schema

No App Group bump is needed: timestamps/data already exist; Widget failure is
local; host attempt/publication is local; semantic comparison is transient; kind
IDs are application constants.

Wire stays v1. `next_refresh_at` changes within its hint role from five minutes
to one minute. Older Widgets remain safe under their 15-minute clamp; older apps
may follow the faster hint only with existing opt-in enabled. Integration review
must assess this timing effect instead of assuming unchanged version means no
compatibility impact.

## Security, privacy, and failure containment

- Keep the existing redacted App Group allowlist. Add no process presence,
  private path, credential, raw event/session, or diagnostic prose.
- Store/reader reject symlinks, non-regular files, wrong schema, oversized or
  malformed input while preserving `0700` directory and `0600` file rules.
- Publisher generation and actor serialization prevent stale whole-file writes.
- Only successful atomic publication requests reload.
- Logging uses fixed codes plus counts/kinds; never projection bodies, paths,
  provider output, or credentials.
- Widget remains read-only and opens no network/helper.
- Tests use isolated App Group roots, synthetic projections, clocks/reloaders/
  wake events, and no real user state.

## Surface requirement provisioning

### Menu bar

| Need | Provision |
| --- | --- |
| Successful-data age | Existing `latestSnapshot.data.generatedAt` + injected date |
| Running/succeeded/failed action | New local `fullAttempt` |
| Retained/first-use failure | `latestSnapshot` presence + `fullAttempt.failed` |
| Widget publication failure | Separate local `widgetPublication.failed` |
| Scan progress | Existing `scanProgress` |
| Matching recovery | Attempt/publication generation and independent result |

### Widget

| Need | Provision |
| --- | --- |
| Generic age | Existing `generated_at` + actual entry date |
| Quota age/reason | Existing subscription observations/reasons |
| Missing/container/read/schema distinctions | New local `WidgetLoadOutcome` |
| 3–5 minute opportunity | Revised timeline policy |
| Host absence | Refused as unobservable; age is sufficient |
| Changed affected kinds | Host-local semantic comparison |
| Unchanged publication | Empty affected-kind set after successful write |
| Recovery | Loaded outcome after newer successful read/publication |

Every reviewed request is provisioned or explicitly refused; implementation has
no remaining user-visible state or persisted-field decision to invent.

## Verification contract

This topic is L3 because it changes scheduling, async coalescing, shared atomic
storage, WidgetKit reload behavior, and native surfaces.

### Go

- `RefreshInterval`/`next_refresh_at` are exactly one minute under injected time;
  wire version and other snapshot semantics remain stable.
- Existing desktop/snapshot differential/cache suites remain green; this topic
  does not reopen performance targets.

### Shared/App Swift

- Fake-clock scheduler: opt-in/off, completion+60 seconds, ≤30-second delay,
  enable/disable, clock jump, one wake catch-up, no replay.
- Coordinator table: full-first, quota-first, full-vs-quota at every quota phase,
  periodic/manual quota-vs-quota, joiner wake-up, manual pending upgrade, one
  pending maximum, full-follow-up priority, failure terminal, and tick-storm
  counterexamples. Assert one probe/deliver/ack/fetch/publish invocation per
  `QuotaOperationID` and no joiner side effects.
- Presentation table: snapshot presence × attempt result × publication result;
  publication failure never marks fresh menu data stale/badged.
- Publisher differential: each rendered field changes only expected kinds;
  shared partial/schema changes affect correct set; excluded/generated-only
  changes yield empty.
- Publisher failure matrix: pre-open/temp/encode/rename failure returns
  `failedBeforeCommit` with verified baseline and `lastVerifiedGeneration`
  unchanged while the admission barrier stays advanced; destination
  verify and file/directory sync failure after successful rename returns
  `indeterminateAfterCommit`, advances only highest accepted generation, sets
  baseline unknown, requests no reload, rejects older writes, and makes the next
  success reload all five and recover.
- Admission-order counterexamples: generation N+1 advances the barrier then
  fails before commit; an already queued and a newly arriving generation N both
  return superseded without file/reload/baseline/publication-state mutation.
  Also suspend N before commit, admit N+1, then resume N and prove the mandatory
  recheck rejects it.
- Quota-only changes only quota, retains full `generated_at`, honors generation,
  and cannot clear full-attempt failure.
- Shared host/Widget bounded-reader tests at N and N+1, file growth during read,
  preflight/open path replacement, symlink/non-regular, and allocation/read byte
  ceilings. Host invalid old projection must yield unknown baseline, still allow
  valid repair write, and reload all five after success.
- Private permissions, malformed/unsupported old projection, restart from
  valid/invalid indeterminate disk state, orphan-temp cleanup, first publication,
  and targeted reloader calls.

### Widget Swift

- Typed loader table for placeholder, missing, container, I/O, decode,
  unsupported, unsafe-file, oversized N+1, exactly-N admitted to decoding, and
  loaded. Exercise preflight/open replacement and file-growth races with bounded
  allocation/read instrumentation.
- Timeline boundaries: past, <3m, 3m, default 4m, 5m, >5m, malformed, failure.
- Age at 14:59, 15:00, 6:00:00, >6h using actual entry time; sleep jump and
  host-absent readable projection.
- Five kinds × three families × two languages for each typed failure, without
  zero values/clipping; quota retains its clock.
- Dynamic Type, accessibility/order, reduced motion, light/dark/Increase
  Contrast; existing contrast carrier stays separately classified.

### Native acceptance

- Measure requested and actual Widget timeline times over a controlled 3–5
  minute campaign; OS delay is reported, not converted to PASS.
- Measure full attempts from terminal completion through 60–90 seconds; prove no
  overlap and one wake catch-up.
- Install representative Widget instances. Change one semantic domain at a time
  and observe only affected kind reload requests; unchanged publication has none.
- Exercise sleep/wake, host absence, App Group missing/read/schema failure,
  publication recovery, VoiceOver, Dynamic Type, Increase Contrast, focus, and
  both languages.

Browser specimens remain design evidence, not native proof. The separate
`ad-bug-widget-cost-incomplete-contrast` carrier must not be reported fixed by
refresh implementation.

## Expected implementation ownership

Likely production boundaries, finalized later in `tasks.md`:

- Go desktop refresh hint/tests;
- App scheduler/preferences copy/wake integration;
- shared coordinator state and request generation/coalescing;
- App Group store/publisher/semantic diff/targeted reloader;
- Widget reader/entry/timeline/domain/view/copy;
- App, Shared, Widget tests and native acceptance harness.

No task may add daemon installation, process-presence probing, App Group schema
fields, private projection data, unrelated quota semantics, or the separate
contrast-carrier repair.

## Review boundary

Architecture review decides whether every reviewed surface need has one owner,
implementable contract, compatibility/security disposition, failure behavior,
and verification assignment. It does not approve code or close final-surface
reconciliation. After PASS, both UX documents return for final design against
these settled contracts before decomposition.
