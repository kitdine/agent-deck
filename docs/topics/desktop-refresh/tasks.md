---
status: active
created: 2026-09-19
updated: 2026-09-19
---

# Desktop Refresh — Tasks

This is the topic-local execution and status authority for `desktop-refresh`.
Its origin is the `ad-desktop-refresh` planning carrier and
`ad-bug-widget-refresh-stale`. The delivered `snapshot-performance` topic is a
dependency and retained authority, not a task repeated here.

Workspace: `agent-deck.desktop-refresh`; branch `feature/desktop-refresh`.
Implementation, Git delivery, integration, retirement, and release remain
separate boundaries.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar-refresh.md | [x] | [x] |
| ux/widget-refresh.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

`requirements.md` passed independent Review Round 1 for its exact document blob,
and its document gate is CEv1 VERIFIED. The menu-bar refresh framework passed
Re-review Round 2 after `MR-R1-F1` closed, and its exact framework/specimen gate
is CEv1 VERIFIED. The Widget refresh framework and shared-prototype specimens are
approved by Review Round 1, and their exact framework/specimen gate is CEv1
VERIFIED. The architecture passed independent Re-review Round 3, was delivered
by signed commit `c0c6daf`, and completed its CEv1-VERIFIED dependency-safe
coordination closure after both final-surface deliveries. Menu-bar
final-surface reconciliation passed Review Round 3 for its architecture-bound
document and specimens. Widget final reconciliation passed Review Round 2 for its
architecture-bound document and rebound specimens. The implementation
decomposition below passed Re-review Round 2 for its synchronized document state;
exactly the five named implementation tasks and their Development authorization
Gates may now be created. The existing Settings layout and periodic-refresh
control are preserved, so no separate settings UX document is applicable.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `widget-publication-foundation` | [x] | [x] |
| 2. `refresh-coordination-and-scheduling` | [ ] | [ ] |
| 3. `widget-loading-and-timeline` | [ ] | [ ] |
| 4. `menubar-refresh-presentation` | [ ] | [ ] |
| 5. `refresh-integration-acceptance` | [ ] | [ ] |

Implementation Beads tasks are created only after this document passes review.
Use one work-product task per anchor and the existing document task lifecycle;
do not create separate development/review tasks or revive the planning carrier
as an implementation task.

### Dependency graph

```text
1. widget-publication-foundation
   ├─> 2. refresh-coordination-and-scheduling ─> 4. menubar-refresh-presentation ─┐
   └─> 3. widget-loading-and-timeline ────────────────────────────────────────────┼─> 5. refresh-integration-acceptance
                                                                                  ┘
```

Tasks 2 and 3 may run in parallel after Task 1. Task 4 waits for the coordinator
state contract from Task 2. Task 5 waits for Tasks 2, 3, and 4 and owns the only
topic-wide native acceptance/reconciliation boundary.

### 1. `widget-publication-foundation`

Deliver the shared, testable publication substrate without changing Widget
surface copy or the full-refresh cadence.

**Files and ownership**

- `apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift` and focused new shared
  files for `AppGroupSnapshotBytes`, `WidgetSnapshotPublisher`,
  `WidgetSemanticProjection`, `AppGroupWidgetKind`, and publisher result/state.
- `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift` only for the narrow
  publisher dependency/invocation hunks that replace direct store writes; Task 2
  owns the coordinator state machine in the same file.
- `apps/macos/AgentDeckApp/AgentDeckApp.swift` only for a shared kind-ID/
  `WidgetTimelineReloader` adapter if needed; the scheduler remains Task 2.
- `apps/macos/AgentDeckTests/AppGroupSnapshotStoreTests.swift`, publisher/
  semantic-diff tests, and shared test fixtures.

**Required result**

- One 8 MiB `lstat → O_NOFOLLOW → fstat → N+1 read → post-fstat` byte loader is
  reusable by host comparison and, later, Widget loading.
- `readExisting()` yields known projection or typed unknown baseline without
  blocking a valid repair write.
- Private atomic writer preserves `0700`/`0600`, rejects unsafe/oversized output,
  defines rename commit point, and distinguishes `failedBeforeCommit` from
  `indeterminateAfterCommit`.
- Publisher actor advances a non-rollback admission barrier before suspension,
  rechecks after every resume and before rename, tracks last verified generation,
  serializes writes, and performs all-kind recovery from unknown baseline.
- Canonical per-kind semantic views cover all possible client/period intents;
  generated/schedule/encoding and Widget-invisible fields are excluded.
- Successful writes request only affected kinds through an injected reloader;
  empty diff and every failure request none.
- The existing explicit `--reload-widget-timelines` command remains all-kind.
- Existing full/quota publication call sites are switched to this one publisher
  in the same task, and the unconditional store-owned reload path is removed.

Task 1 changes publication only; the existing refresh scheduling/attempt model
continues to call the new publisher until Task 2 replaces that model. There is
never a parallel legacy/new publisher or a second long-lived publication design.

**Verification — L3**

- N/N+1, file growth, path replacement, symlink/non-regular, bounded allocation,
  malformed/unsupported previous file, first publication, permissions and
  durability failures.
- Newer pre-commit failure followed by older queued/incoming writes; older
  suspended write after newer admission; post-commit indeterminate and restart
  from valid/invalid disk.
- Field-by-field semantic diff table for five kinds, shared partial/schema cases,
  every intent scope, unknown-baseline all-kind recovery, and exact reloader set.
- Relevant Shared tests with deterministic suspended-operation interleavings;
  full macOS suite is deferred to Task 5 after consumers are wired.

**Excluded:** app cadence, quota/full arbitration, menu presentation, Widget
timeline/view changes, App Group schema bump, and the separate contrast carrier.

### 2. `refresh-coordination-and-scheduling`

Wire the publication foundation into one completion-based app scheduler and one
full/quota coordinator. Depends on Task 1.

**Files and ownership**

- `internal/desktop/desktop.go` and focused tests for the one-minute wire hint.
- `apps/macos/AgentDeckApp/AgentDeckApp.swift`, `DesktopPreferences.swift`,
  `DesktopCopy.swift`, the String Catalog entry for the periodic-refresh note,
  and a focused scheduler/wake source file. `SettingsWindowView` changes only if
  the existing key/association cannot carry the reviewed copy unchanged.
- `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift` and focused coordinator,
  quota-arbiter, attempt/publication state files as the implementation shape
  requires.
- `apps/macos/AgentDeckTests/DesktopRefreshCoordinatorTests.swift`, scheduler/
  preference tests, and Go desktop tests.

**Required result**

- Opt-in automatic full refresh uses injected monotonic `completion + 60s`, the
  bounded 30-second evaluator, and one wake catch-up; off remains the default.
- Startup/manual/provider-switch remain available while periodic is off.
- One full lane and one quota lane implement the reviewed join/priority/
  one-pending table. One `QuotaOperationID` owner performs probe→deliver→ack→
  fetch→publish side effects once; joiners only await its result.
- Manual/provider full requests produce at most one follow-up and provider switch
  cannot be satisfied by a pre-switch generation.
- Publication generations are allocated centrally after staleness checks and use
  Task 1's already-wired publisher; no direct store/reload path is reintroduced.
- `latestSnapshot`, `fullAttempt`, and `widgetPublication` are independent;
  quota-only splice retains full `generated_at` and cannot clear full failure.
- Definite and indeterminate publication results remain distinct internally and
  recover only on a newer verified publication.
- The Settings note is exactly the reviewed approximately-one-minute English/
  Chinese copy in the same existing control and accessibility association.

**Verification — L3**

- Go `next_refresh_at = generated_at + 1m` with wire compatibility regression.
- Fake-clock preference/wake/enable/disable/time-jump table and ten deterministic
  terminal-to-next-start cycles with no overlap/replay.
- Full-first, quota-first, all full-vs-quota phase crossings, periodic/manual
  quota joins, pending priority upgrade, failure terminal and tick-storm cases.
- Assert one side-effect invocation per `QuotaOperationID`, stable alert-ID/
  acknowledgment boundary, publication ordering, stale completion rejection,
  and Task 1 result propagation.
- Relevant Go full package suite and App/Shared targeted tests with deterministic
  concurrency/interleaving coverage.

**Excluded:** SwiftUI presentation, Widget entry/view code, native timing claims,
new background service, and quota semantics beyond arbitration/publishing.

### 3. `widget-loading-and-timeline`

Implement the reviewed Widget-local load, time, and failure surface contracts.
Depends on Task 1 and may run in parallel with Task 2.

**Files and ownership**

- `apps/macos/AgentDeckWidget/WidgetSnapshot.swift`, `WidgetTimeline.swift`,
  `WidgetDomain.swift`, `WidgetViews.swift`, `WidgetCopy.swift`, intents only if
  required by unchanged configuration plumbing, and Widget target localization.
- `apps/macos/AgentDeckWidgetTests/WidgetTimelineTests.swift`,
  `WidgetPresentationTests.swift`, render/geometry/accessibility tests and
  synthetic fixtures.
- Shared byte-loader use from Task 1; no duplicate Widget-only reader.

**Required result**

- Entry carries `WidgetLoadOutcome` rather than optional snapshot + swallowed
  error: placeholder, loaded, missing, container unavailable, unreadable with
  fixed category, and unsupported version.
- Reader performs minimal schema-header/full decode on Task 1's bounded bytes;
  no `Data(contentsOf:)`, path/content logging, network, helper or database.
- Timeline minimum/default/maximum is exactly 3/4/5 minutes; malformed/failed
  hints use 4m, past uses 3m, and every invocation emits one actual-time entry.
- Generic age uses projection `generated_at`; quota keeps oldest observation/
  reason. Host absence adds no field/label and sleep may jump directly to old.
- All five kinds × three families render each typed failure with reviewed copy,
  no zero/retained values, and unchanged frame/header/footer order.
- Placeholder, partial, empty, pricing, attribution and quota states retain their
  existing ownership. The contrast bug remains external.

**Verification — L3**

- Typed outcome table including N/N+1/unsafe/decode/schema and container nil.
- Timeline boundaries and all three provider families under injected dates.
- Age edges 14:59, 15:00, 6:00:00, >6h, sleep jump and host-absent projection.
- 5 kinds × 3 families × 2 languages for four failures plus readable age states,
  exact accessible labels/order, geometry, Dynamic Type depth degradation,
  light/dark/reduced-motion checks.
- Full Widget target test suite; native WidgetKit timing remains Task 5.

**Excluded:** host scheduler/coordinator, publication diff, Widget-triggered
refresh action, persisted failure field, App Group schema bump and contrast fix.

### 4. `menubar-refresh-presentation`

Implement the final menu-bar/Settings contract using Task 2's independent state.
Depends on Task 2.

**Files and ownership**

- `apps/macos/AgentDeckApp/MenuBarViewModel.swift`, `MenuBarSurfaceView.swift`,
  remaining refresh copy/localization keys, and relevant app models/helpers.
  The periodic Settings note is consumed and verified here but owned by Task 2.
- App tests for ViewModel derivation, copy/localization, accessibility, focus,
  layout and Settings descriptions.
- No Widget target files and no scheduler logic beyond consumption.

**Required result**

- Header age always uses successful snapshot time. Running/failure keeps prior
  data; first failure shows no fabricated values.
- One stable refresh control preserves keyboard focus, exposes `aria`/native
  disabled semantics without duplicate trigger, and shows success for 1.6s.
- Full failure notice and Retry are separate from partial data. Publication
  definite/indeterminate share the reviewed Widget-out-of-date notice, never mark
  menu data stale, and never badge the menu-bar item.
- Scan progress remains the single detailed live region; generic announcements
  do not duplicate it.
- Periodic Settings note is exactly the reviewed English/Chinese approximately-
  one-minute copy; control remains opt-in/off-default.
- Recovery clears only the matching attempt/publication notice; unrelated health,
  schema, sessions, partial, quota and provider states remain.

**Verification — L2 with native-App risk checks**

- Exhaustive snapshot presence × attempt × publication derivation table and
  notice composition/priority.
- Keyboard retry/running/success focus, one live announcement, narrow 280 pt,
  Settings 460 pt, both languages/themes, localization key completeness.
- App target regression suite and isolated acceptance-window render checks.
- Native VoiceOver/Dynamic Type/Increase Contrast and real scheduler timing are
  retained for Task 5, not reported PASS here.

**Excluded:** scheduler/publisher internals, Widget views, provider/quota product
semantics, new Settings control, menu-bar item badge redesign and update check.

### 5. `refresh-integration-acceptance`

Integrate and accept the complete refresh contract. Depends on Tasks 2, 3, and 4.
This task fixes only integration defects caused by those tasks; new product
decisions return to the owning design document.

**Files and ownership**

- Cross-target App/Shared/Widget tests and narrowly required acceptance harness
  updates under `scripts/` and `apps/macos/AgentDeckVerification`.
- Stable contract reconciliation in `docs/specs/cli-design.md` and applicable
  manual/help text; topic review/status artifacts remain workflow-owned.
- No unrelated release, packaging, notification, quota-source or performance
  work.

**Required result**

- Full app path uses only the new scheduler/coordinator/publisher; Widget uses
  only typed bounded loading/timeline; no legacy unconditional reload path.
- Semantic changes reload only affected installed kinds; unchanged publication
  reloads none; invalid/unknown baseline recovery reloads all five once.
- Full/quota concurrency, publication failure/indeterminate/recovery and Widget
  local outcomes remain truthful across process/target boundaries.
- Stable specifications match delivered one-minute host hint, 60–90s app cadence,
  3/4/5m Widget requests, schema-v1 compatibility, security/privacy and retained
  native limitations.

**Verification — L3 integration/native acceptance**

- Final `scripts/test-macos-app.sh` Shared/App/Widget suites in an isolated HOME,
  relevant race/concurrency instrumentation, whitespace and diff checks.
- Ten consecutive controlled active-app full cycles: every terminal-to-next-start
  interval requested/started within 60–90s, no overlap or catch-up burst.
- At least five granted Widget timeline callbacks across representative kinds:
  record requested 3–5m dates and actual callback/render times separately. OS
  delay remains an explicit limitation, not a falsified product PASS.
- Real sleep/wake proves one catch-up; host absence ages readable data; missing,
  unsafe/oversized/read/schema failures and newer successful recovery are
  observed without private-state leakage.
- Installed representative Widget intents prove per-kind changed reload,
  unchanged no reload, quota-only isolation and unknown-baseline all-kind
  recovery.
- Native English/Chinese, light/dark, Dynamic Type, VoiceOver, Increase Contrast,
  focus and absence of unexpected cross-kind reload are recorded as performed,
  blocked or waived—not silently converted to PASS.

**Excluded:** release/tag/push/install authority, performance-target reopening,
daemon/file watcher, App Group schema fields, host-presence probing, and
`ad-bug-widget-cost-incomplete-contrast` repair.

## Review boundary

`tasks.md` review decides whether these five anchors cover every approved
requirement, final surface, architecture invariant, dependency, file owner and
verification level without overlapping responsibility or adding scope. Run
`bash scripts/check-topic-docs.sh`; a PASS allows creation of exactly these five
implementation Beads tasks and their Development authorization Gates. It does
not authorize implementation, commit, push, release, installation or native
waivers.
