---
status: active
created: 2026-09-08
updated: 2026-09-17
---

# Subscription Quota — Tasks

Target version: `v0.6.0`, selected in `docs/topics/v0-6-0-contract/tasks.md`
under the carrier `ad-subscription-quota`. This decomposition authorizes no
merge, push, PR, tag, or release.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [x] |
| ux/menubar-quota.md | [x] | [x] |
| ux/widget-quota.md | [x] | [x] |
| ux/settings-quota.md | [x] | [x] |
| architecture.md | [x] | [x] |
| tasks.md | [x] | [x] |

Why each row exists, against the review question that justifies it:

- `requirements.md` — one boundary question for one coherent behavior change.
  Its hardest content is not the goals but the source feasibility: what each
  client actually exposes was measured, and what it does not expose is recorded
  as a disposition rather than a gap to fill later.
- `ux/menubar-quota.md` — the popover gains a fifth tab, and the tab strip
  itself changes at the narrow bound. Both clients, their windows, the plan, the
  reset allowance, and every member of the closed failure-reason set are
  user-visible with no presentation rule today.
- `ux/widget-quota.md` — a fifth widget kind at three fixed sizes, on a refresh
  cadence deliberately different from the probe cadence. Size-as-depth and the
  freshness rule are distinct judgments from the popover's.
- `ux/settings-quota.md` — three opt-in decisions, one of which writes to a file
  another tool owns. Consent copy that names its consequence is a design
  question, not a string.
- `architecture.md` — two client adapters with different shapes, a gate, a
  schedule, and a wire addition. Every field the three surfaces request needs a
  written disposition.
- `tasks.md` — this file.

No `n/a` rows: every document kind this project defines applies to this topic.
The shared [Product Prototype](../../../prototype/README.md) carries the
specimens; it is not a topic document and takes no row, per
`docs/documentation-workflow.md`.

## Prototype

The prototype work is done and is part of the design, not a task. It is listed
here because a reviewer needs to know what already exists:

| Change | Where |
| --- | --- |
| Quota fixtures, eight variants including Codex Plus and reading-off, provider routes carried with each; `attribution_confirmed` per client | `prototype/src/data.js` |
| Quota panel, first in the tab order and the default tab | `prototype/src/Popover.jsx` |
| Hover-opened reset-allowance side popover, side chosen from available room | `prototype/src/Popover.jsx`, `prototype/src/styles.css` |
| Stage anchor axis so the left/right/overlay rule is demonstrable | `prototype/src/Stage.jsx`, `prototype/src/App.jsx`, `prototype/src/styles.css` |
| Visible quota-state stage switch; URL retained only as a deep link | `prototype/src/Stage.jsx`, `prototype/src/App.jsx`, `prototype/src/Popover.jsx` |
| Five-column tab strip at tightened type; icon-only at 280 pt | `prototype/src/Popover.jsx`, `prototype/src/styles.css` |
| Fifth widget kind: configured client on small (highest-used window) and medium (all its windows); both clients split into equal halves on large; no size carries the reset allowance | `prototype/src/Widgets.jsx`, `prototype/src/styles.css` |
| Visible small-widget client selector in the stage bar | `prototype/src/Stage.jsx` |
| Subscription-quota settings group, defaults off, disabled-state treatment | `prototype/src/Settings.jsx`, `prototype/src/styles.css` |
| Status-line consent failures modelled as two distinct outcomes — write refused (error tone, switch stays off) and restore incomplete (warning tone, switch off) — on the field's existing failure row | `prototype/src/Settings.jsx`, `prototype/src/i18n.js` |
| Failure-row semantic tones made to actually resolve, over the window's own hint styling | `prototype/src/styles.css` |
| Eighteen settings assertions on the interaction probe, and three older ones repaired to address groups by name instead of hardcoding a layout this group changed | `prototype/src/probe.js`, `prototype/src/Settings.jsx` |
| `zh` and `en` copy | `prototype/src/i18n.js` |
| Size-contract self-checks for the quota card | `prototype/src/contract.js` |
| Horizontal-overflow check added to the gauge; widget board added to its ellipsis coverage | `prototype/src/measure.js` |
| Repairs for `MB-Q-F1`, `MB-Q-F2`, `MB-Q-F3` | `prototype/src/Popover.jsx`, `prototype/src/styles.css` |
| Reading-off card, and the missing word derived from the reason instead of a passed flag | `prototype/src/Popover.jsx` |
| Claude account-attribution line, on its own row under the card header | `prototype/src/Popover.jsx`, `prototype/src/styles.css` |
| Visible Claude account-attribution line on all three widget sizes, short form; Codex carries none | `prototype/src/Widgets.jsx`, `prototype/src/styles.css`, `prototype/src/i18n.js` |
| Six attribution-presence assertions on the contract board, and its per-size expectations made to follow the configured client instead of assuming Codex | `prototype/src/contract.js` |
| Widget no-data card reports its real reason instead of a hardcoded `never_probed` | `prototype/src/Widgets.jsx` |
| Reset-allowance popover made a continuous hover region, computed from the live trigger and popover rects rather than a CSS constant | `prototype/src/Popover.jsx`, `prototype/src/styles.css` |
| Seventeen quota assertions added to the interaction probe; probe addresses tabs by `data-tab`, reads the language at assertion time, and renders its own crash | `prototype/src/probe.js`, `prototype/src/Popover.jsx`, `prototype/src/Stage.jsx` |
| Visible quota-state and small-widget client controls; URL values retained only as deep links | `prototype/src/Stage.jsx`, `prototype/README.md` |

Recorded results at design time: `npm run build:single` passes; the gauge
reports `NO OVERFLOW` across the Popover matrix and the key Widget/Settings
specimens, including the two-client Plus and one-client gate shapes; the contract
board reports `ALL PASS` for both Plus and gate contracts, and for the
reading-off variant. The interaction probe runs to completion in both languages
at 81 PASS / 1 FAIL over 82 assertions; every quota and settings assertion
passes. The one remaining failure is the sessions work-signal detail banner, which belongs to
that surface rather than to this topic and is recorded rather than repaired
here. The four settings-window assertions that failed in earlier rounds were
repaired under `ux/settings-quota.md`, whose group had invalidated them.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `quota-domain` | [x] | [x] |
| 2. `codex-adapter` | [x] | [x] |
| 3. `claude-adapters` | [x] | [x] |
| 4. `gate-and-schedule` | [x] | [x] |
| 5. `quota-alerts` | [x] | [x] |
| 6. `wire-and-cli` | [x] | [x] |
| 7. `desktop-surfaces` | [x] | [x] |

### 1. `quota-domain`

**Result:** the domain model exists and the three reset semantics cannot be
confused by construction.

**Files:** new `internal/quota/model.go`, `internal/quota/state.go`,
`internal/quota/store.go`, and focused `*_test.go` files in `internal/quota/`.
This task owns the domain and persistence; client adapters, scheduling, alerts,
wire/CLI, and Swift surfaces stay in later tasks.

Expanded by Round 1 review finding QD-R1-F5, with operator approval: the quota
schema (`quota_windows`, `quota_envelopes`) is registered as
`internal/store/migrations.go` version 24 rather than owned by a package-local
`CREATE TABLE IF NOT EXISTS`, so it versions like every other production table.
That also touches `internal/store/store.go` (`CurrentSchemaVersion`) and the
schema-count-dependent fixtures/tests it ripples into
(`cmd/agentdeck/main_test.go`'s schema-12-upgrade test,
`desktop/fixtures/v1/snapshot-complete.json` and `snapshot-empty-client.json`).

- Types for the per-client envelope, windows, reset allowance, billing, and the
  closed reason set from `architecture.md` C6.
- `window_key` built per C6's table — the Codex `limitId` with `_secondary` on
  that slot, Claude's two payload names — and opaque to every surface.
- Storage of the latest observation per `(client, account_id, window_key)` plus
  one prior observation, with no time series — C7.
- `stale` computed from `allowed_age = max(window_minutes / 10, 2 × probe
  interval)` against the client's shortest window — C5.
- `observed_reset_at` derived only from a decrease in used share, never from a
  passed `resets_at`.
- Account-change handling discards rather than merges — C8.
- Claude storage keys on `(client, window_key)` with no account component, and
  every payload carries `attribution_confirmed` — false for Claude, true for
  Codex — C8.
- Per-client selection assigns `source` from the chosen observation: the newest
  usable observation wins, with the status-line route preferred to the prose
  route at equal age, and that source travels unchanged to the payload — C5.

**Verification:** L1. The reason set is exhaustive over its declared members,
`probe_disabled` included; `observed_reset_at` is not set when `resets_at`
passes without a decrease; an account change discards; `attribution_confirmed`
is false for every Claude payload and true for Codex; `window_key` is
constructed for both clients including a Codex `secondary` slot; `allowed_age`
is checked at both window lengths, at each selectable interval including the
one where the floor takes over, and one minute either side of the threshold;
equal-age Claude observations select the status-line route and retain its
`source`. No surface, adapter, or scheduler work here.

### 2. `codex-adapter`

**Depends on:** task 1.

**Result:** `account/rateLimits/read` reaches the domain model.

**Files:** new `internal/quota/codex.go`,
`internal/quota/codex_test.go`, and captured JSON-RPC fixtures under
`internal/quota/testdata/`. This task changes no command, desktop wire,
scheduler, alert, or Swift surface file.

- The three-step handshake, stdin held open until the matching `id` or the
  deadline — C2. Process exit is not an empty answer.
- Field mapping per C2's table, including the refusals: `spendControlReached`,
  `rateLimitReachedType`, `individualLimit`, `rateLimitUpsell` are not carried.
- `reset_allowance.total` is null with `not_reported`.
- `plan` is opaque: no code branches on its value.
- Failure classification separates spawn failure, non-zero exit, deadline,
  malformed JSON, and a well-formed response without `rateLimits` — the last is
  `not_reported`, not a probe failure.

**Verification:** L1 against captured fixtures, including the malformed and
missing-`rateLimits` cases. No test spawns `codex`.

Clarified by a Round 1 review follow-up on CA-R1-F1, approved by the operator
on 2026-09-11 during that repair turn (in-session `AskUserQuestion`, not a
Beads comment alone): the real `GetAccountRateLimitsResponse` protocol schema
(`codex app-server generate-json-schema`, codex-cli 0.154.0) shows
`rateLimitsByLimitId` as one of two parallel views — the required top-level
`rateLimits` snapshot is itself "a backward-compatible single-bucket view"
with its own `primary`/`secondary`. C2's table names only
`rateLimitsByLimitId[k].primary/.secondary` as the window source and does not
address this duality.

Decision, exact scope: when `rateLimitsByLimitId` is **absent or null** (an
older backend, or an account with no bucketed view), `parseCodexResult` falls
back to `rateLimits.primary`/`.secondary`, using `rateLimits.limitId` for the
window key when present, or the placeholder limitId `"codex"` when it is
also absent — chosen because it matches the per-limit key this topic's own
fixtures and requirements.md's observed sample use for the account's main
limit. When `rateLimitsByLimitId` is present as an **explicit empty object**
(`{}`), this does **not** apply: an empty object is a bucketed view the
backend supplied and found empty, distinct from no bucketed view existing,
and yields zero windows. (Round 2 review finding CA-R2-F1 found the code,
`CodexResult`'s doc comment, and `parseCodexResult`'s doc comment disagreeing
on whether "empty" also falls back — repaired to all agree with the exact
scope recorded here.) Neither case is a probe failure.

This decision fixes the `window_key` later tasks 5 (alert dedup) and 6 (wire)
will see for an unbucketed account, and departs from what C2's table names as
the sole window source; reconciling `architecture.md` C2 itself, if desired,
is that document's own review and is not part of this task.

### 3. `claude-adapters`

**Depends on:** task 1.

**Result:** both Claude routes reach the domain model, and the status-line
registration is safe to consent to.

**Files:** new `internal/quota/claude.go`,
`internal/quota/claude_test.go`, `internal/quota/statusline.go`, and
`internal/quota/statusline_test.go`; `internal/usagehook/config.go` and
`internal/usagehook/config_test.go` for surgical register/restore; new
`cmd/agentdeck/quota_capture.go` and `cmd/agentdeck/quota_capture_test.go`, plus
`cmd/agentdeck/main.go` and `cmd/agentdeck/main_test.go`, for the status-line-only
capture entry. End-user quota output and the desktop wire remain task 6.

- Prose parser, all-or-nothing, resolving the reset instant against the named
  zone rather than the local one — C4.
- The "what's contributing" section is discarded.
- Status-line capture and chaining — C3. The prior command's stdout passes
  through byte for byte; a failure in either the prior command or the capture
  does not affect the other.
- Registration and restore through the existing surgical editing in
  `internal/usagehook/config.go`; the prior value is recorded before the write.
- Restore reports rather than overwrites when the file changed underneath.
- A missing `rate_limits` is `not_reported`, never a failure.
- `window_minutes` supplied from the window name as a local constant — 300 and
  10080 — and `not_reported` for a name outside those two; never presented as a
  vendor field — C3.

**Verification:** L1 for the parser against captured prose including a shape
change; L2 for chaining and for register/restore against a temporary
settings file. No test invokes `claude`.

### 4. `gate-and-schedule`

**Depends on:** tasks 2 and 3.

**Result:** probes happen when they should and not otherwise.

**Files:** new `internal/quota/gate.go`, `internal/quota/gate_test.go`,
`internal/quota/scheduler.go`, and `internal/quota/scheduler_test.go`;
`internal/desktop/desktop.go` and `internal/desktop/desktop_test.go` for
refresh-trigger integration; `apps/macos/AgentDeckApp/DesktopPreferences.swift`
and `apps/macos/AgentDeckAppTests/DesktopPreferencesTests.swift`, plus
`apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift` and
`apps/macos/AgentDeckTests/EmbeddedHelperRunnerTests.swift`, for the
reading/interval control path. This task does not render those controls or add
the subscription wire shape.

- The reading switch precedes the provider gate: with `quotaProbe` off no
  scheduler runs, no subprocess is spawned by any trigger, and every field
  reports `probe_disabled` while stored observations are retained — C1, C9.
- The gate on `provider.Service.Current` against `OfficialProviderName`, with
  suppression when an available observed provider disagrees, *provided* that
  observation is not older than the current selection itself — an observation
  predating the switch must never suppress a probe after it (GS-R1-F1) — C1.
- Manual refresh probes with the snapshot, bypassing the interval and backoff
  but neither the reading switch nor the provider gate; background refresh
  respects the quota interval and backoff — C9. (GS-R1-F2: an earlier version
  also applied backoff to manual; C9 names only the reading switch and the
  provider gate as still applying to a user-initiated refresh.)
- Geometric backoff to a bounded maximum, reset on success. Only a background
  failure advances it; a manual failure is recorded but does not (GS-R2-F1) —
  manual is neither gated by backoff nor a contributor to it.
- Single-flight per client, in-process only — a best-effort guard against a
  future concurrent caller, not a cross-process guarantee (GS-R1-F3). This
  task's one production caller, `RefreshQuota`, visits both clients
  sequentially, so the guard does not currently trigger in production; a
  cross-process lock was considered and rejected as out of this task's scope.
- The no-credential property asserted over this topic's packages — C0.
- Turning reading off while a status-line route is installed must eventually
  unregister it through the C3 restore, clear the consent flag, and report
  `restore incomplete` when the file changed underneath, with stored
  observations left untouched — C9. **Deferred to task 6** (GS-R1-F4): this
  transition needs a CLI surface for `usagehook.RestoreStatusLine` that does
  not exist yet, and task 6 is the task that adds one (`cmd/agentdeck/
  quota.go`). This task's `DesktopPreferences.swift` stores the reading
  switch and interval only; it does not perform this transition.

**Verification:** L2 for the scheduler with a controlled clock, including the
reading-off case across every trigger, the retention of stored observations
across a toggle, manual bypassing both the interval and backoff, and the
stale-observation case (GS-R1-F1); L1 for the gate including the disagreement
case; L1 for the credential assertion. The unregister-on-off transition's
verification moves to task 6 along with the behavior itself.

Four operator-approved decisions made during implementation, each because the
architecture text required something this task's declared file list did not
by itself provide for:

1. C1's observed-provider cross-check needed a reader for
   `usage_session_observations.observed_provider`, which
   `internal/usage/routes.go` owned but had never exported (only
   `RecordHookDelivery` wrote it). Added a minimal read-only export,
   `usage.Service.LatestObservedProvider(ctx, client) (provider string,
   observedAt time.Time, ok bool, err error)` — `observedAt` was added during
   Round 1 repair (GS-R1-F1 below) — with its own tests in
   `internal/usage/routes_test.go`.
2. Geometric backoff needs state that survives a process restart — every
   `agentdeck desktop snapshot` invocation is a fresh process. Added
   `EnvelopeRecord.BackoffUntil` (`internal/quota/model.go`), persisted as
   `quota_envelopes.backoff_until` in `internal/store/migrations.go` version
   25 (`CurrentSchemaVersion` 24 → 25); `PutEnvelope`/`PutEnvelopeFailure` in
   `internal/quota/store.go` clear/set it alongside `Failure`/`FailureAt`. The
   scheduler derives each next step from `BackoffUntil` and `FailureAt`
   together rather than a separate counter column. `desktop/fixtures/v1/
   snapshot-complete.json` and `snapshot-empty-client.json` were regenerated
   (`AGENTDECK_UPDATE_FIXTURES=1`) because the Doctor `schema` health check's
   `count` field is the schema version number — confirmed by diff that this
   is the only change in either file.
3. Turning `quotaProbe` off must synchronously unregister an installed
   status-line route (C9) via `usagehook.RestoreStatusLine`, which task 3
   built but exposed through no CLI verb. **Deferred to task 6**
   (`wire-and-cli`), which already touches `cmd/agentdeck`. This task's
   `DesktopPreferences.swift` adds `quotaProbeEnabled`/`quotaProbeInterval` as
   control-path storage only; no unregister call is wired yet.
4. Running a probe is a side-effecting operation, but the existing `desktop
   snapshot` command is documented and implemented as strictly read-only
   (`store.OpenReadOnly`; "without scanning sources, creating state, or using
   the network"), and reusing `desktop refresh-indexes` was also rejected.
   Quota probing is its own, independent mechanism instead:
   `internal/desktop.Service.RefreshQuota(ctx, core *store.Store, home
   string, trigger quota.Trigger, probeEnabled bool, interval, maxBackoff
   time.Duration)`, which takes an already-writable `core` and is never
   called from `Build`. No CLI command calls it yet — task 6 adds that
   surface and the real preference plumbing; this lands the mechanism ahead
   of that wiring, the same sequencing task 3 used for
   `usagehook.SetupStatusLine`/`RestoreStatusLine`. Consequently
   `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift` and its tests were
   **not touched** this round: there is nothing for them to call yet. Its
   tests landed in a new co-located `internal/desktop/quota_test.go` rather
   than in `desktop_test.go` itself, to keep the addition reviewable as one
   unit; both are the same package and the same declared file's spirit.

**Round 1 repair (2026-09-12), five operator-approved decisions** — see
[`reviews/gate-and-schedule.md`](reviews/gate-and-schedule.md) for the full
findings:

1. **GS-R1-F1 (high, fixed):** the observed-provider cross-check compared
   against the recorded selection with no regard for which was newer, so a
   Hook observation predating a switch to `official` suppressed every probe
   indefinitely. `LatestObservedProvider` now also returns the observation's
   instant; `desktop.go`'s `quotaObservedOfficial` treats an observation
   older than the current selection's `SelectedAt` as unknown rather than
   disagreeing.
2. **GS-R1-F2 (medium, fixed):** manual refresh was also gated by backoff,
   an unrecorded narrowing of C9's text (which names only the reading switch
   and the provider gate as still applying to manual). `Scheduler.due` now
   lets a manual trigger bypass backoff too; the scope bullet above and this
   section's decision record the change. Single-flight is unaffected.
3. **GS-R1-F3 (medium, fixed as a documentation correction, not a behavior
   change):** single-flight's package-level guard neither "joins" a running
   probe (a second caller returns immediately with pre-probe state) nor
   reaches across the separate OS processes each `agentdeck desktop snapshot`
   invocation actually is — and this task's one production caller never even
   calls it concurrently. Operator chose to record this as an accepted,
   disclosed in-process-only degradation (scope bullet above,
   `scheduler.go`'s doc comment) rather than build a new cross-process lock.
4. **GS-R1-F4 (medium, fixed):** this file contradicted itself — the scope
   and verification lines still required the status-line unregister-on-off
   transition in full, while decision 3 above had already deferred it to
   task 6. The scope and verification lines above are now updated to match
   the deferral; task 6's own section gained the corresponding scope bullet
   and verification line.
5. **GS-R1-F5 (low, fixed):** `RefreshQuota` discarded the `Reason`
   `quota.Allowed` computed per client. It now returns `map[quota.Client]
   quota.Reason` (empty per client when the gate allowed that cycle's
   attempt) instead of a bare discard, so a future caller does not have to
   re-derive C1's full gate — including the timestamp-gated observed-provider
   check from GS-R1-F1 — from scratch to learn why a client wasn't probed.

New/changed tests for this round: `internal/quota/scheduler_test.go`
(`TestSchedulerManualBypassesBackoff`, replacing the prior
`TestSchedulerManualStillRespectsBackoff`); `internal/desktop/quota_test.go`
(`TestRefreshQuotaIgnoresStaleObservedProviderPredatingCurrentSelection`,
`TestRefreshQuotaSuppressesOnCurrentDisagreeingObservation`,
`TestRefreshQuotaReturnsGateOutcomePerClient`); `internal/usage/routes_test.go`
(updated for `LatestObservedProvider`'s new `observedAt` return).

**Round 2 repair (2026-09-12), one operator-approved decision** — see
[`reviews/gate-and-schedule.md`](reviews/gate-and-schedule.md):

1. **GS-R2-F1 (low, fixed):** GS-R1-F2 made manual bypass backoff as a
   *consumer*, but a manual failure still advanced the same background
   backoff chain as a *producer* — a few manual retries in quick succession
   could push the next background probe out by as much as `MaxBackoff`, an
   unrecorded mirror of GS-R1-F2's own narrowing. Fixed: `recordFailure` now
   takes the trigger, and a manual failure leaves `BackoffUntil` exactly as
   it already was rather than advancing it; only a background failure still
   does. New test: `internal/quota/scheduler_test.go`'s
   `TestSchedulerManualFailureDoesNotAdvanceBackgroundBackoff`, reproducing
   the review's exact scenario (one background failure, four rapid manual
   retries, a background attempt still due at the original deadline).

**Round 3 repair (2026-09-12), one operator-approved decision** — see
[`reviews/gate-and-schedule.md`](reviews/gate-and-schedule.md):

1. **GS-R3-F1 (medium, fixed):** the Round 2 fix left `BackoffUntil`
   untouched by a manual failure but still moved `FailureAt` forward.
   `nextBackoff` derives the prior step's length from `BackoffUntil.Sub(
   FailureAt)` (decision 2 above), so moving only one half of that pair
   shrank — and could invert — the derived step, corrupting every later
   background failure's doubling: repeated manual retries could compress
   backoff all the way back to the base interval on a persistently failing
   endpoint, defeating the protection backoff exists for. Fixed: a manual
   failure now leaves `FailureAt` untouched too (reusing the envelope's
   existing value), so the pair moves together only on a background failure;
   only `Failure` (the reason) reflects a manual attempt. This does not
   revisit decision 2 — no counter or second anchor field was added. New
   tests: `internal/quota/scheduler_test.go`'s
   `TestSchedulerManualFailureDoesNotCorruptBackoffStepDerivation` (a manual
   retry between two background failures must not change the second
   failure's doubled step) and
   `TestSchedulerRepeatedManualRetriesBetweenBackgroundFailuresStillDoubleCorrectly`
   (five cycles of background-failure-then-manual-retry must still climb
   5m/10m/20m/40m/1h, not flatten to 5m every cycle).
2. **GS-R3-F2 (low, fixed):** `recordFailure`'s doc comment retained a
   sentence from before the Round 2 fix claiming a manual failure "advances
   C9's geometric backoff," directly above a newer sentence saying the
   opposite. Removed the stale sentence; the comment now states only the
   current (correct) behavior.

**Round 4 repair (2026-09-13)** — see
[`reviews/gate-and-schedule.md`](reviews/gate-and-schedule.md):

1. **GS-R4-F1 (medium, fixed):** Round 3's fix has a manual failure inherit
   `FailureAt` from the envelope. With nothing to inherit — never probed, or
   the last probe succeeded and `PutEnvelope` cleared it — that value is
   zero, and `PutEnvelopeFailure` formatted it unconditionally, persisting
   `0001-01-01T00:00:00Z` beside a real `Failure`. Fixed with the review's
   prescribed direction: `PutEnvelopeFailure` now stores a zero `failureAt`
   as empty, matching its own `backoffUntil` guard and `PutEnvelope`'s
   handling of both. The read side already treated empty as zero. This
   touches `internal/quota/store.go`, a task 1 file, as decision 2 already
   did. New tests: `internal/quota/store_test.go`'s
   `TestStorePutEnvelopeFailureStoresZeroFailureAtAsAbsent` and
   `internal/quota/scheduler_test.go`'s
   `TestSchedulerManualFailureWithNoPriorFailureStoresNoFailureInstant`
   (both review cases: never probed, and a success followed by a manual
   failure).

**Round 5 repair (2026-09-13)** — see
[`reviews/gate-and-schedule.md`](reviews/gate-and-schedule.md):

1. **GS-R5-F1 (low, fixed):** both GS-R4-F1 regression tests asserted
   `FailureAt.IsZero()` through `Store.Envelope()`, but the pre-fix text
   `0001-01-01T00:00:00Z` also parses back to a zero `time.Time`, so neither
   test failed with the guard removed. Both now read `failure_at` and
   `backoff_until` as stored text via a new `rawEnvelopeInstants` helper in
   `internal/quota/store_test.go` and assert empty strings. Confirmed as a
   negative control with `go test -overlay` substituting a `store.go` whose
   only difference is the removed guard: all three cases fail on
   `failure_at="0001-01-01T00:00:00Z"`, and pass against the real file.

**Swift verification note:** this environment has only Xcode Command Line
Tools, not full Xcode. `swiftc -parse` on both changed Swift files is clean
(syntax only). `swift test` cannot run at all — XCTest is unavailable outside
Xcode (confirmed: the SwiftPM test invocation itself fails with "no such
module 'XCTest'", on files this task never touched). The Foundation-only
verifier (`bash scripts/test-macos-app.sh`) built successfully but reported a
pre-existing, unrelated runtime failure ("expected two index refreshes
followed by one snapshot read") in `AgentDeckShared`/`AgentDeckVerification`
files this task did not modify (confirmed via `git ls-files --modified
apps/macos`) — noted here rather than silently passed over, but fixing it is
out of this task's scope. Full XCTest execution of
`DesktopPreferencesTests.swift` is manual acceptance pending real Xcode.

### 5. `quota-alerts`

**Depends on:** tasks 1 and 4.

**Result:** opt-in alerts that do not repeat.

**Files:** new `internal/quota/alerts.go` and
`internal/quota/alerts_test.go`. This task owns evaluation, deduplication,
due-alert identification, and the acknowledgement ledger, not delivery,
Settings presentation, or any other Swift surface — delivery belongs to the
App, per task 7 and `MA-F2` below.

- Off by default, and with alerts off the evaluator does not run — C10. The
  same holds with reading off, whatever the alert switch says, because the
  evaluator has nothing to run on — C9.
- Threshold crossing deduplicated per `(client, window_key, threshold, window
  instance)`, the instance identified by `resets_at`.
- Reset notice once per window occurrence, driven by the observed decrease.
- Notification content names client, window, and figure, and carries no account
  identifier.

**Verification:** L2 across simulated window occurrences: the same threshold
fires once, then again after a reset; repeated probes above a threshold produce
nothing; nothing is evaluated with reading off. Real notification delivery is
manual acceptance, named below.

Four operator-approved decisions made during implementation. Decisions 1–3 are
also recorded in architecture.md C10 (the ledger paragraph, and the Same
instance and Crossing readings); decision 4 is recorded only here:

1. **Deduplication ledger.** Every probe or refresh runs as its own process, so
   "once per occurrence" needs persisted state. Added `quota_alert_notices` as
   `internal/store/migrations.go` version 26 (`CurrentSchemaVersion` 25 → 26),
   keyed by client, window, kind, threshold, and instance in epoch seconds; its
   read/write methods live in `internal/quota/alerts.go`, so `store.go` is
   unchanged. `desktop/fixtures/v1/snapshot-complete.json` and
   `snapshot-empty-client.json` were regenerated for the Doctor `schema` count,
   and `cmd/agentdeck/main_test.go`'s schema-12 upgrade test now also drops
   the new table when it downgrades a current database to version 12.
   A notice is recorded only after delivery succeeds, and pruned 31 days after
   its occurrence ended. The ledger holds no account identifier or figure.
2. **Instance tolerance.** Two `resets_at` within 15 minutes are the same
   occurrence, because the routes report one occurrence at different
   precision. A window without `resets_at` gets no threshold notice.
3. **At or above, not edge-triggered.** A threshold notice is due when the used
   share is at or above the threshold with none sent for that occurrence, so a
   crossing written by the status-line capture is not missed, and alerts
   switched on while a window is already above a threshold notify once. A reset
   notice is sent only for an `observed_reset_at` in the window's current
   occurrence.
4. **No caller yet at authoring time; delivery moved to the App by `MA-F2`
   (2026-09-16)** (see Manual acceptance below). The evaluator exposes
   `DueAlerts`, `AcknowledgeAlert`, and `ValidateAlertIDs` — not
   `EvaluateAlerts`/`OSANotifier`, both removed. Task 6's
   `desktop quota-refresh` calls `DueAlerts` after each refresh and returns
   the due alerts on the wire; its new `desktop quota-alerts ack` calls
   `AcknowledgeAlert`/`ValidateAlertIDs` for the ids the App's delivery
   accepted. `scheduler.go` and `statusline.go` are unchanged. The App
   (task 7) delivers each notification through `UNUserNotificationCenter` via
   `QuotaAlertNotifier.swift` and owns the localized copy; this task's helper
   computes and records due alerts but never delivers.

**Round 1 repair (2026-09-13)** — see
[`reviews/quota-alerts.md`](reviews/quota-alerts.md):

1. **QA-R1-F1 (high, fixed):** the evaluator treated the latest stored window
   as the current occurrence even after its `resets_at` had passed. When probes
   stop succeeding the last good window stays stored, so alerts switched on
   later reported a past figure, and once the ledger entry was pruned after 31
   days every evaluation notified again. A window whose `resets_at` is more
   than the 15-minute instance tolerance in the past now sends neither notice;
   with only ongoing occurrences evaluated, pruning can no longer remove an
   entry the evaluator could still match. New test
   `TestEvaluateAlertsIgnoresAnOccurrenceThatHasEnded` (repeated evaluation
   before and after retention; alerts switched on 47 hours after the
   occurrence ended).
2. **QA-R1-F2 (medium, fixed):** reset notices were deduplicated by
   `observed_reset_at`, which any same-source drop moves forward, so a small
   drop later in the same occurrence notified "has reset" a second time. The
   ledger instance for a reset notice is now the occurrence's `resets_at`,
   with the same tolerance as thresholds; `observed_reset_at` only decides
   whether a reset was observed in this occurrence. A window without
   `resets_at` gets no reset notice. `TestEvaluateAlertsResetNoticeOncePerObservedReset`
   became `TestEvaluateAlertsResetNoticeOncePerOccurrence` and now drops 0.5
   points inside the same occurrence; new test
   `TestEvaluateAlertsNoResetNoticeWithoutResetsAt`.

**Round 2 repair (2026-09-13)** — see
[`reviews/quota-alerts.md`](reviews/quota-alerts.md):

1. **QA-R2-F1 (medium, fixed):** introduced by QA-R1-F2's fix. With reset
   notices deduplicated per occurrence, `resetInCurrentOccurrence` accepted
   any `observed_reset_at` for a window whose length is not reported, so one
   real reset's sticky `observed_reset_at` notified "has reset" again in each
   later occurrence, even one where usage only rose. Of the review's two
   directions, the conservative one: a window of unknown length gets no reset
   notice, since its current occurrence cannot be bounded — a missed notice
   rather than a false one. Recorded in architecture.md C10. New test
   `TestEvaluateAlertsNoResetNoticeWhenWindowLengthUnknown` (real reset, then
   a later occurrence with no decrease: no notice).

**Round 3 repair (2026-09-13)** — see
[`reviews/quota-alerts.md`](reviews/quota-alerts.md):

1. **QA-R3-F1 (low, fixed):** architecture.md C10 introduced four readings as
   "Three readings … (quota-alerts task, operator-approved)", but only the
   first two (Same instance, Crossing) are operator decisions (decisions 2 and
   3 above); the last two came from review repairs, including QA-R2-F1's
   narrowing that a window of unknown length gets no reset notice, which no
   operator decided. Took the review's option (a), text only: the intro now
   counts four and says which are which, and each bullet names its source —
   operator-approved, or QA-R1-F1, or QA-R1-F2 and QA-R2-F1. No code or test
   changed.

**Round 4 repair (2026-09-13)** — see
[`reviews/quota-alerts.md`](reviews/quota-alerts.md):

1. **QA-R4-F1 (low, fixed):** the introduction to task 5's four decisions
   said they were "recorded in architecture.md C10 as well", but decision 4
   (no caller yet; English notification text) appears nowhere in
   architecture.md. Took the review's option (a), text only: the introduction
   now says decisions 1–3 are also recorded in C10 and names where, and that
   decision 4 is recorded only in this file. No contract text was added to
   C10, and no code or test changed.

**Round 6 repair (2026-09-16)** — see
[`reviews/quota-alerts.md`](reviews/quota-alerts.md):

1. **QA-R6-F1 (medium, fixed):** this task's `Files`/ownership line, decision
   4, and task 6's call-boundary bullet and decision 2 still described the
   deleted `notifier_darwin.go`/`notifier_darwin_test.go`, `EvaluateAlerts`,
   and `OSANotifier` as the current delivery mechanism, contradicting `MA-F2`'s
   App-delivery outcome and the current code (`DueAlerts`, `AcknowledgeAlert`,
   `ValidateAlertIDs`, and `QuotaAlertNotifier.swift`). Text only, no product
   code changed: task 5's Files/ownership now name only `alerts.go`/
   `alerts_test.go` and the acknowledgement ledger; decision 4 describes the
   current `DueAlerts`/ack flow and the App's `UNUserNotificationCenter`
   delivery; task 6's bullet and decision 2 describe `DueAlerts` and
   `desktop quota-alerts ack` instead of the removed calls; and task 7's
   `Files` and scope now name `QuotaAlertNotifier.swift` and the notification
   delivery/localization/permission-denied ownership explicitly.

### 6. `wire-and-cli`

**Depends on:** tasks 1–5.

**Result:** the data leaves the process in both supported shapes.

**Files:** `internal/desktop/desktop.go` and
`internal/desktop/desktop_test.go` for the additive subscription wire section;
new `cmd/agentdeck/quota.go` and `cmd/agentdeck/quota_test.go`, plus
`cmd/agentdeck/main.go` and `cmd/agentdeck/main_test.go`, for text/JSON command
routing; `apps/macos/AgentDeckShared/DesktopWire.swift` and
`apps/macos/AgentDeckTests/DesktopWireTests.swift` for the matching optional
decoder. Focused fixtures stay beside those tests.

- The additive `subscription` section at unchanged `WireVersion = 1` — C11.
  `account_id` and `billing.balance` are not carried.
- `tightest_window_key` resolved once when the payload is built, null with no
  windows; `windows[]` keeps vendor order rather than being sorted — C11.
- Tightest-window resolution in the payload, so the popover and the widget
  cannot disagree.
- `agentdeck quota` in text and `--json` forms — C12. The gate and probe
  failures are payload states at exit 0; non-zero is reserved for conditions
  that prevent producing a payload.
- `attribution_confirmed` per client on the wire, and the reading-off state
  reaching both the wire and the CLI as `probe_disabled` rather than an absent
  section — C8, C11.
- The CLI page of the prototype is the rendered form of the text output.
- Turning `quotaProbe` off unregisters an installed status-line route through
  the C3 restore, clears the consent flag, and reports `restore incomplete`
  when the file changed underneath; stored observations are untouched — C9.
  Deferred here from task 4 (`gate-and-schedule`'s GS-R1-F4): this needs a
  CLI verb over `usagehook.RestoreStatusLine`, which only this task's
  `cmd/agentdeck/quota.go` addition provides. `DesktopPreferences.swift`'s
  `quotaProbeEnabled`/`quotaProbeInterval` (added in task 4) are this
  transition's trigger; this task wires the call itself.
- Calling `quota.DueAlerts` after each quota refresh with the user's alert
  settings (`quotaAlerts`, `quotaThresholds`, `quotaResetNotice`) and
  returning the due alerts on the `desktop quota-refresh` wire, plus a new
  `desktop quota-alerts ack` command calling `quota.AcknowledgeAlert` and
  `quota.ValidateAlertIDs` for the ids the App's delivery accepted — C10.
  Deferred here from task 5 (`quota-alerts`), which provides the evaluator and
  ledger but neither caller nor delivery; delivery belongs to the App, per
  task 7.

**Verification:** L1 for the payload shape including the absent-section case;
L2 for the CLI surface and its exit codes, including the unregister-on-off
transition against a temporary settings file and its restore-incomplete path
(moved here from task 4's verification per GS-R1-F4). Reconcile
`docs/specs/cli-design.md` in task 7's closure, not here.

Three operator-approved decisions made during implementation:

1. **Preferences live in core state.** `agentdeck quota` run from a terminal
   must present reading off by default, and it cannot read the app's
   UserDefaults. The settings group (reading, interval, alerts, thresholds,
   reset notice, status-line consent) is stored in the existing core
   `settings` table under `quota.*` keys, through new
   `internal/quota/settings.go`; no migration. The CLI, the desktop snapshot,
   and the quota refresh all read it. A stored value that does not parse is an
   error, not a silent default.
2. **One public read command; writes under `desktop`.** `agentdeck quota` is
   public and read-only (C12): it reads stored state, probes nothing, and on a
   fresh installation with no state reports reading off. The desktop-driven
   writes sit beside `desktop refresh-indexes`: `desktop quota-refresh`
   (`RefreshQuota`, then `DueAlerts`, returning the due alerts on the wire for
   the App to deliver and acknowledge via `desktop quota-alerts ack`),
   `desktop quota-settings` (turning reading off restores an installed
   status-line route, clears consent, and reports `restore_incomplete` when the
   file changed; a user's own `statusLine` is left alone), and
   `desktop quota-statusline enable|disable` (enable requires reading on).
   `quota capture` stays hidden. This closes the two items deferred here from
   tasks 4 and 5.
3. **Swift callers belong to task 7.** This task delivers the Go commands and
   `DesktopWire.swift`'s optional decoder; `EmbeddedHelperRunner`,
   `DesktopPreferences`, and the refresh coordinator calling these commands
   are added to task 7's scope below.

Implementation choices within that scope, recorded so a reviewer need not
derive them: the section builder lives in new `internal/desktop/subscription.go`
beside `desktop.go`, and the CLI tests in `cmd/agentdeck/quota_test.go`; the
wire's `source` values use the prototype's names (`codex_app_server`,
`claude_statusline`, `claude_usage_prose`) mapped from the domain's; reading
off keeps `applicable: true` with `failure: probe_disabled`, as the
prototype's reading-off variant does, while a provider other than official
sets `applicable: false`; a parse failure after the last success presents no
figure (requirements.md clause 6), while a probe failure keeps the last figure
with its real age (C9); client-level `observed_reset_at` is the most recent
window's; backoff is bounded at one hour; `internal/quota/notifier_unsupported.go`
gives non-darwin builds a nil notifier. The prototype's CLI page has no quota
specimen, so the text output follows the CLI's existing plain-English form.
The complete canonical desktop fixture is seeded with quota data so the Swift
decoder is exercised on every field.

**Round 1 repair (2026-09-13)** — see
[`reviews/wire-and-cli.md`](reviews/wire-and-cli.md):

1. **WC-R1-F1 (high, fixed):** task 4's `Scheduler.recordFailure` deliberately
   leaves a manual failure's `FailureAt` at whatever it already was (GS-R3-F1),
   to protect the background backoff chain — after a success clears it, that
   leaves it zero. `subscriptionClient` judged "did this failure happen after
   the last known-good observation" by comparing `FailureAt` against
   `observedAt`, so a zero `FailureAt` could never be after anything: a manual
   failure right after a success vanished from both the wire and the CLI, and
   a manual parse failure kept re-presenting the prior figure instead of
   showing none (requirements.md clause 6). Took the review's option (a):
   `PutEnvelope` clears `Failure` only on a genuine success, so `Failure != ""`
   alone already proves the envelope's own route failed more recently than its
   own `ObservedAt` — true for a manual failure exactly as for a background
   one — without comparing timestamps at all. The one route that bypasses the
   envelope, Claude's status-line, is handled separately: a status-line window
   newer than the envelope's `ObservedAt` is a success the envelope never
   recorded, and it still supersedes a stale prose failure. New test
   `TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible` drives
   `desktop quota-refresh --manual` success then failure for both clients and
   asserts Codex's `probe_failed` stays visible with its figure kept, and
   Claude's `parse_failed` shows no figure.
2. **WC-R1-F2 (low, fixed):** the CLI text line concatenated `"last probe "`
   with `quotaReasonPhrase(client.Failure)`, and the only reason that reaches
   that branch, `probe_failed`, phrases as `"probe failed"`, producing
   `"last probe probe failed"`. Dropped the redundant word: the line now reads
   `"last " + quotaReasonPhrase(...)`, giving `"last probe failed"`. Asserted
   in the same new test above.

**Round 2 repair (2026-09-13)** — see
[`reviews/wire-and-cli.md`](reviews/wire-and-cli.md):

1. **WC-R2-F1 (medium, fixed; new, exposed by the Round 1 repair):** the
   Round 1 fix let a manual failure reach the parse-failure branch, but that
   branch still took the attempt instant from `FailureAt` — which task 4's
   `recordFailure` deliberately leaves untouched for a manual failure
   (GS-R3-F1), and which is zero after a success clears it or when the client
   was never probed. A manual parse failure therefore showed the right reason
   and correctly withheld the figure, but `observed_at` came back `null`,
   missing requirements.md clause 6's "the observation instant of the failed
   attempt". Operator-approved (asked because it touches task 1's schema and
   task 4's scheduler): took the review's option (b) — a new
   `EnvelopeRecord.FailureObservedAt` field, independent of the
   `FailureAt`/`BackoffUntil` backoff-chain pair, that `recordFailure` writes
   on every failed attempt, manual or background alike. `internal/store`
   schema version 26 → 27 (`ALTER TABLE quota_envelopes ADD COLUMN
   failure_observed_at`); `PutEnvelopeFailure` gained a `failureObservedAt`
   parameter, always the real attempt instant; `PutEnvelope` clears it on a
   success in the same write that clears `Failure`/`FailureAt`/`BackoffUntil`.
   `subscriptionClient`'s parse-failure branch now reads `FailureObservedAt`
   instead of `FailureAt`. `FailureAt`/`BackoffUntil` themselves are
   unchanged — a manual failure still leaves them exactly as they were, so
   the backoff-step derivation GS-R3-F1 protects is untouched. Canonical
   desktop fixtures regenerated for the schema-count bump
   (`AGENTDECK_UPDATE_FIXTURES=1`); no fixture content other than that count
   changed. New/extended tests: `TestStorePutEnvelopeFailureRoundTripsFailureObservedAt`
   and the extended `TestStorePutEnvelopeFailureStoresZeroFailureAtAsAbsent`
   (`internal/quota/store_test.go`); the renamed
   `TestSchedulerManualFailureWithNoPriorFailureStoresNoBackoffChainInstant`
   now also asserts `FailureObservedAt` is written while `FailureAt`/
   `BackoffUntil` stay empty (`internal/quota/scheduler_test.go`); the
   existing `TestDesktopQuotaRefreshManualFailureAfterSuccessStaysVisible`
   (`cmd/agentdeck/quota_test.go`) now also asserts a non-null `observed_at`
   on the manual parse failure and that the CLI text carries `attempted`.

### 7. `desktop-surfaces`

**Depends on:** task 6.

**Result:** the three surfaces match their approved documents.

**Files:** `apps/macos/AgentDeckApp/DesktopPreferences.swift`,
`SettingsWindowView.swift`, `MenuBarSurfaceView.swift`, `MenuBarPanelViews.swift`,
`MenuBarViewModel.swift`, `DesktopCopy.swift`, `Localizable.xcstrings`, and
`QuotaAlertNotifier.swift` (quota alert notification delivery, added by
`MA-F2`), with focused files in `AgentDeckAppTests`;
`apps/macos/AgentDeckShared/AppGroupSnapshotStore.swift` and its tests;
`apps/macos/AgentDeckWidget/WidgetDomain.swift`,
`apps/macos/AgentDeckWidget/WidgetIntents.swift`,
`apps/macos/AgentDeckWidget/WidgetSnapshot.swift`,
`apps/macos/AgentDeckWidget/WidgetViews.swift`,
`apps/macos/AgentDeckWidget/WidgetCopy.swift`, the Widget
`apps/macos/AgentDeckWidget/Localizable.xcstrings`, and focused files under
`apps/macos/AgentDeckWidgetTests/`; and
`docs/specs/cli-design.md` for the approved contract narrowing. Task 6 owns the
Go wire, CLI rendering, and Swift decoder. The existing `prototype/` remains the
approved presentation reference rather than an implementation output, so task 7
consumes it and does not rebuild or edit it. Where task 4 and task 7 share
preference files, task 4 owns control-path behavior and task 7 owns presentation.

- Menu-bar quota tab, five-column strip,
  icon-only at the narrow bound, and the hover-opened reset-credit popover whose
  side follows available room, per `ux/menubar-quota.md`.
- The reading-off card and the Claude attribution statement on every surface
  that shows a figure, at the placement each surface document settles — the
  popover header; below the reset countdown on small Widget; below the window
  rows on medium; and in the Claude block header on large. Small may also carry
  the statement in its accessible name, but never instead of the visible line.
- Fifth widget kind: small and medium project the one configured client — the
  highest-used window on small, all of its windows on medium; large splits both
  clients into equal centred halves and omits a client with no usable window; no
  size carries the reset allowance, which stays in the popover; freshness stays
  in the frame footer, per `ux/widget-quota.md`.
- Settings group with defaults off, dependency shown as disabled, and consent
  copy naming the chained command, per `ux/settings-quota.md`.
- The Swift callers of task 6's desktop commands, moved here by task 6's
  decision 3: the settings group writes through `desktop quota-settings` and
  `desktop quota-statusline enable|disable` (core state is the authority; the
  app's UserDefaults at most mirror it), and a refresh runs
  `desktop quota-refresh` — with `--manual` for a user-initiated refresh — so
  probes and alert evaluation follow C9 and C10. This touches
  `apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift` and its tests and the
  app's refresh coordination, in addition to the files listed above.
- Quota alert notification delivery and its localized copy, per `MA-F2` and
  architecture.md C10 ("Delivery belongs to the app"): `QuotaAlertNotifier.swift`
  requests permission, posts each due alert through `UNUserNotificationCenter`,
  and returns the accepted ids; the refresh coordinator in
  `EmbeddedHelperRunner.swift` then calls `desktop quota-alerts ack` for those
  ids only. This task also owns the permission-denied warning row and its
  "open notification settings" entry point in Settings.
- The specification revision described in `requirements.md` — Contract changes —
  narrowing `docs/specs/cli-design.md:53-55`. Read the then-current revision;
  do not prescribe a revision number, following task 2 (`v0-6-0-contract`) in
  `docs/topics/v0-6-0-contract/tasks.md`, which owns that instruction.

**Verification:** L2 for the Swift surfaces against the documents, plus the
manual acceptance below. Where the implementation and a ux document disagree,
the prototype decides; where the prototype and a document disagree, the
prototype is right and the document is corrected.

**Round 1 repair (2026-09-14)** — see
[`reviews/desktop-surfaces.md`](reviews/desktop-surfaces.md):

1. **DS-R1-F1 (medium, repaired after Round 2 remained open):** the systemLarge
   quota Widget stacks Codex and Claude vertically. Both client blocks receive
   the remaining body height through equal `maxHeight` slots and are vertically
   centred inside their own slot; the divider exists only between two rendered
   clients, and a one-client result uses a single slot with no empty reserved
   half. The regression renders the real `AgentDeckWidgetView`, captures each
   slot and intrinsic client-content frame through SwiftUI geometry preferences,
   and asserts equal slot heights plus matching content/slot midpoints. Changing
   the production slot back to `.top` moves those midpoints apart and fails the
   test; this replaces the earlier enum-only protection that Round 2 rejected.
2. **DS-R1-F2 (medium, repaired):** quota freshness no longer reads the desktop
   snapshot generation clock. Each Widget family derives the oldest
   `observed_at` among the clients/windows it actually presents; both footer
   time and aging qualifiers use that instant. With no successful observation,
   the footer reports the applicable closed reason instead of saying it was
   updated now. The regression separates a fresh snapshot from older Codex and
   Claude observations and asserts the oldest displayed observation wins.
3. **DS-R1-F3 (medium, repaired):** settings writes now stage user intent
   optimistically and coalesce overlapping changes into the latest complete
   desired state. A response cannot overwrite a newer pending intent; after it
   completes, the queued full state is submitted. A failed write keeps the
   desired state for a later retrying change while retaining the explicit error
   row. Suspended-transport tests cover overlapping changes and failure/retry,
   including final agreement between core response, controller, and local
   reading/interval mirrors.

## Manual acceptance

Named here because no specimen settles them. Each needs an explicit result —
performed, or waived by the operator with the waiver recorded:

**Task 7 operator waiver (2026-09-14):** the operator explicitly waived every
`desktop-surfaces` manual-acceptance row below for this implementation round.
No row is represented as performed. Task 3 and task 5 rows are outside this
waiver and retain their existing owners and status.

**Task 3 and task 5 rows, isolated run (2026-09-16).** Performed by
claude-code on the operator's instruction, never against real state: a
temporary HOME and state directory, the branch's own `agentdeck` build first on
a PATH that contains no `codex` or `claude`, a copy of the operator's real
`~/.claude/settings.json` (which has a multi-line `statusLine` chaining
`python3 ~/.claude/statusline.py`) and of that script, and `env -i`. The real
file was verified unchanged afterwards; the copies were deleted.

| Row | Result |
| --- | --- |
| Consent flow, task 3 | Enable is refused with reading off; enable writes only `statusLine`; re-enable is `unchanged`; the chained output equals the prior command's; both Claude windows are captured; an edited AgentDeck entry is removed with `restore_incomplete`; a foreign value is left byte for byte. **Finding `MA-F1`:** disable and reading-off restored the prior value compacted onto one line — equal as JSON, not byte for byte. |
| Notification delivery, task 5 | A threshold notice was delivered: `usernoted` completed the request, and with a Focus mode active it was deferred into Notification Centre rather than interrupting. A second refresh sent nothing. **Finding `MA-F2`:** `osascript` posts as Script Editor (`com.apple.ScriptEditor2`), so the sender is wrong, the permission cannot be controlled for AgentDeck, and a disabled Script Editor still reports success, recording a notice nobody saw. The permission-denied state was not executed, because that needs a real system setting changed. |

**`MA-F1` repaired in candidate** (operator decision, 2026-09-16: repair on the
topic branch). `internal/usagehook/config.go` records the prior value's exact
bytes as a JSON string beside the decoded value, and restores from them;
records written before carry only the value and still restore.
`TestRestoreStatusLineRestoresFormattedPriorByteForByte` failed before the
repair and passes after it; the isolated consent run then restored byte for
byte in both the disable and reading-off paths. Record: `reviews/claude-adapters.md`.

**`MA-F2` repaired in candidate** (operator decisions, 2026-09-16: the app
delivers; acknowledgement after delivery; permission requested when alerts are
turned on; a denial keeps the switch on and shows a warning row with a link to
System Settings; copy follows the app language). Contract: architecture.md C10
"Delivery belongs to the app" and ux/settings-quota.md "When notifications are
not allowed". Files: `internal/quota/alerts.go` (`DueAlerts`,
`AcknowledgeAlert`, `ValidateAlertIDs`; `notifier_*.go` removed),
`cmd/agentdeck/quota.go` (`quota-refresh` returns `alerts`; new
`desktop quota-alerts ack --id`), the GUI JSON contract fixture,
`EmbeddedHelperRunner.swift` (alert wire type, acknowledgement transport, and
the coordinator posting then acknowledging only accepted ids), new
`AgentDeckApp/QuotaAlertNotifier.swift`, `QuotaSettingsController.swift`,
`SettingsWindowView.swift`, `MenuBarItemController.swift`, `AgentDeckApp.swift`,
`DesktopCopy.swift`, `Localizable.xcstrings`, and their tests. Record:
`reviews/quota-alerts.md`. Real delivery under AgentDeck's own bundle identity,
including the permission prompt, a denial, and Focus, remains manual acceptance
for the next review.

**Task 5 App-delivery follow-up (2026-09-16): FAILED / BLOCKED.** A separately
signed App with its own bundle id, temporary HOME/database, isolated defaults,
and no production App Group was used; the production App and state were not
touched. Even after Developer ID signing and LaunchServices launch, no system
permission prompt appeared and the App was absent from System Settings ›
Notifications, so permission and Focus remained blocked. The native denied-state
row did render with the alert switch still on, but its warning, action, and the
following threshold controls visibly overlapped (`MA-F3`). Full evidence and the
screenshot digest are in `reviews/quota-alerts.md`. The temporary App was stopped,
unregistered, and deleted after the run.

**`MA-F3` repaired in candidate (2026-09-16).** The failed acceptance bundle was
not a valid notification client: only a team-signed App outside `/tmp` reached
the permission prompt in the controlled comparison. New
`scripts/run-macos-acceptance-app.sh` builds an independent bundle identifier,
removes the Widget extension, applies a real team signature, runs from the
ignored `apps/macos/build/acceptance/` directory, and isolates all AgentDeck and
client state. The visible overlap had a separate AppKit cause: the Settings
window used the SwiftUI fitting height only once, before the asynchronous denied
row appeared. `FittingSizeHostingController` now resizes the reusable window when
that fitting size changes. The new real-window regression failed at unchanged
`734.0` pt before the fix and passes after it. Permission allow/deny and Focus
still require a fresh native acceptance run during independent re-review.

**`MA-F3` independently re-reviewed and accepted (2026-09-16).** Round 4 PASS
confirmed the dynamic Settings sizing and the isolated acceptance launcher.
On `com.kitdine.agentdeck.acceptance.ma3rereview`, the operator confirmed the
system permission identity, delivery, Focus deferral, repeat-refresh deduplication,
permission-off warning and System Settings entry point. The repaired native
window shows the warning, action, thresholds and reset switch without overlap.
The temporary App was stopped, unregistered and deleted; its isolated HOME was
removed. Record: `reviews/desktop-surfaces.md`.

The `MA-F1` repair changed task 3 content; its independent Round 4 re-review
passed on 2026-09-16. Task 6 passed independent Round 4 re-review. Task 5 passed
Round 7 re-review, and the shared post-repair acceptance closes its verification
gap. Task 7 passed independent Round 4 re-review with MA-F3 and its manual
acceptance closed.

| What | Owning task |
| --- | --- |
| Real SF typography and Dynamic Type at accessibility sizes, five tabs | 7 |
| VoiceOver order across five tabs; an icon-only tab announcing its name at 280 pt | 7 |
| Quota-bar tone distinguishable for a red-green colour-blind reader, with the percentage retained as the non-colour carrier | 7 |
| Accessibility reading order inside each Widget size | 7 |
| A disabled settings switch announcing as unavailable, and how the dependency is conveyed non-visually | 7 |
| Real WidgetKit rendering, tint, and system refresh behavior | 7 |
| Real notification delivery, including Do Not Disturb and permission states | 5 |
| Consent flow end to end against a real `~/.claude/settings.json` with an existing `statusLine`, including disable and restore | 3 |
| Live light/dark switching while the panel is open | 7 |

## Findings repaired in this change

Three narrow-bound defects the gauge reported, none introduced by this topic and
all reproducing on `?tab=usage&lang=en&width=280&measure=1`. On the operator's
2026-09-08 instruction they are repaired here rather than carried to a separate
lane, so no bug carrier is created and no lane decision is needed:

| Finding | Detail | Repair |
| --- | --- | --- |
| `MB-Q-F1` | Four-column stat grid at 280 pt: `15–17 时` short by 11 px, `15–17h` by 3 px | Grids denser than three columns reflow to two at the narrow bound |
| `MB-Q-F2` | Provider footer short by 2 px at 280 pt in `en` | The label column is dropped at the narrow bound; it survives in the button's accessible name |
| `MB-Q-F3` | Header refresh label runs 42 px past the panel edge at 280 pt in `en` | Icon-only refresh at the narrow bound; `aria-label` already carried the name |

`MB-Q-F3` was invisible to the gauge, which checked only scrollable widget
containers and `text-overflow: ellipsis` elements. The gauge now also compares
`scrollWidth` against `clientWidth` inside every panel, and that addition
immediately found a fourth defect this topic had introduced without noticing:
the English tab strip was 14 px wider than its container at 420 pt, because five
labels do not fit at the type size four labels fit at. Both are repaired.

The gauge change was verified by disabling a repair, confirming the gauge
reported it, and re-enabling. A check that has never been observed to fail is
not evidence.

## Current handoff

**2026-09-16/17:** manual acceptance of tasks 3 and 5 found `MA-F1` and `MA-F2`,
and the task 5 follow-up found task 7 layout defect `MA-F3` (see Manual
acceptance). Task 3 passed independent Round 4 re-review and `MA-F1` was
delivered in signed commit `f9f7646` on `feature/subscription-quota`. Task 5
passed independent Round 7 re-review before the App-delivery follow-up; tasks 6
and 7 passed independent Round 4 re-review. `MA-F2` (tasks 5/6/7) and `MA-F3`
(task 7) were delivered together in signed commit `dc4d556` on
`feature/subscription-quota`. All task Review cells are checked, all seven
tasks are closed in Beads with CEv1 task gates VERIFIED at their delivered
commit content, and the topic gate's three criteria (documents-passed,
tasks-reviewed-and-committed, manual-acceptance-dispositions) are VERIFIED at
`dc4d556`. Neither commit has been pushed; assembly into `main` is tracked by
the `v0-6-0-contract` topic's `assemble` task, not here.
Everything below describes the state before these repairs except where the
task-specific paragraphs say otherwise.

All six documents are drafted and have passed review. The five upstream records
are:
[`requirements.md`](reviews/requirements.md),
[`ux/menubar-quota.md`](reviews/ux-menubar-quota.md),
[`ux/widget-quota.md`](reviews/ux-widget-quota.md),
[`ux/settings-quota.md`](reviews/ux-settings-quota.md), and
[`architecture.md`](reviews/architecture.md). This `tasks.md` decomposition
passed Round 3 re-review on 2026-09-10; see
[`reviews/tasks.md`](reviews/tasks.md) for the complete record and evidence gate.

Task 1 `quota-domain` passed Round 4 re-review on 2026-09-11 after three
failed rounds; its delivery state is tracked in Beads `ad-sq-quota-domain-dev`.
The schema expansion into `internal/store` described in task 1's
Files note is part of the reviewed content. See
[`reviews/quota-domain.md`](reviews/quota-domain.md) for the findings, their
dispositions, the evidence, and the completion gate.

Task 2 `codex-adapter` passed Round 3 re-review on 2026-09-11 after two failed
rounds; its delivery state is tracked in Beads `ad-sq-codex-adapter-dev`. The
`rateLimits` fallback decision is recorded in task 2's own section above. See
[`reviews/codex-adapter.md`](reviews/codex-adapter.md) for the findings,
evidence, and completion gate.
Task 3 `claude-adapters` passed Round 4 re-review on 2026-09-16 after isolated
manual acceptance found and repair closed `MA-F1`; its delivery state is tracked
in Beads `ad-sq-claude-adapters-dev`. The statusLine prior value lives in
AgentDeck's own state (a sidecar file under the state directory), never in
`~/.claude/settings.json` alongside the `statusLine` key itself. New sidecars
retain the prior value's exact JSON bytes for byte-for-byte restoration, while
legacy sidecars remain compatible; `quota.MaxStatusLinePayloadBytes` bounds only
capture's own parsing, never what the status-line chain forwards to the prior command. See
[`reviews/claude-adapters.md`](reviews/claude-adapters.md) for the findings,
evidence, and completion gate.
Task 4 `gate-and-schedule` passed Round 6 re-review on 2026-09-13 after five
failed rounds; its delivery state is tracked in Beads
`ad-sq-gate-and-schedule-dev`. All ten findings (GS-R1-F1 through GS-R5-F1)
are closed. Four operator-approved decisions
made during implementation are recorded in task 4's own section above: a new
read-only `usage.Service.LatestObservedProvider` export, a new persisted
`EnvelopeRecord.BackoffUntil` field (schema version 24 → 25), the status-line
unregister-on-off transition deferred to task 6, and quota probing landing as
its own independent `desktop.Service.RefreshQuota` mechanism — not on `Build`,
not on `refresh-indexes` — with no CLI caller yet. See that section for the
Swift verification limitations in this environment (no full Xcode), and
[`reviews/gate-and-schedule.md`](reviews/gate-and-schedule.md) for the
findings, the reproducers, and the completion gate.

Task 5 `quota-alerts` was delivered in signed commit `dbd119f`, then reopened
after manual acceptance invalidated its notification-delivery path. The repaired
candidate passed Round 7 re-review; the subsequent repaired App-delivery
acceptance passed permission, delivery, Focus, deduplication and denied-state
layout checks, so its completion gate is VERIFIED. The repair was delivered in
signed commit `dc4d556`; Beads `ad-sq-quota-alerts-dev` is closed. See
[`reviews/quota-alerts.md`](reviews/quota-alerts.md) for the full finding
dispositions, evidence, and completion gate.

Task 6 `wire-and-cli` was delivered in signed commit `cfcc395`, then reopened
for MA-F2. The repaired candidate passed Round 4 re-review and was delivered in
signed commit `dc4d556`; Beads `ad-sq-wire-and-cli-dev` is closed. WC-R1-F1,
WC-R1-F2, and WC-R2-F1 remain closed. The Round 2
repair's operator-approved storage change (`EnvelopeRecord.FailureObservedAt`,
schema version 26 → 27) is recorded in task 6's own section above. See
[`reviews/wire-and-cli.md`](reviews/wire-and-cli.md) for the full evidence and
completion gate.

Task 7 `desktop-surfaces` was delivered in signed commit `f7f6865`, then reopened
for MA-F2/MA-F3. The repaired candidate passed Round 4 re-review with real native
notification acceptance; see [`reviews/desktop-surfaces.md`](reviews/desktop-surfaces.md).
The repair was delivered in signed commit `dc4d556`; Beads
`ad-sq-desktop-surfaces-dev` is closed. The implementation retains the
fail-closed hosted-test guard:
`AgentDeckAppTests` cannot construct a real-home helper without an isolated
`AGENTDECK_TEST_HOME`.

The base of this worktree is `4737076`; `main` has since advanced by ten
commits, including the assembled `schema-version-signal` surfaces. The surface
documents describe this branch's prototype state. Reconciliation belongs to the
`v0-6-0-contract` topic's `assemble` task, not to this topic.
