---
status: active
created: 2026-09-08
updated: 2026-09-10
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
| 4. `gate-and-schedule` | [ ] | [ ] |
| 5. `quota-alerts` | [ ] | [ ] |
| 6. `wire-and-cli` | [ ] | [ ] |
| 7. `desktop-surfaces` | [ ] | [ ] |

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
- Turning reading off while a status-line route is installed unregisters it
  through the C3 restore, clears the consent flag, and reports `restore
  incomplete` when the file changed underneath. Stored observations are
  untouched — C9.
- The gate on `provider.Service.Current` against `OfficialProviderName`, with
  suppression when an available observed provider disagrees — C1.
- Manual refresh probes with the snapshot, bypassing the interval but neither
  the reading switch nor the provider gate; background refresh respects the
  quota interval — C9.
- Geometric backoff to a bounded maximum, reset on success.
- Single-flight per client.
- The no-credential property asserted over this topic's packages — C0.

**Verification:** L2 for the scheduler with a controlled clock, including the
reading-off case across every trigger, the retention of stored observations
across a toggle, and the unregister-on-off transition against a temporary
settings file including its restore-incomplete path; L1 for the gate including
the disagreement case; L1 for the credential assertion.

### 5. `quota-alerts`

**Depends on:** tasks 1 and 4.

**Result:** opt-in alerts that do not repeat.

**Files:** new `internal/quota/alerts.go`,
`internal/quota/alerts_test.go`, `internal/quota/notifier_darwin.go`, and
`internal/quota/notifier_darwin_test.go`. This task owns evaluation,
deduplication, and delivery, not Settings presentation or any other Swift
surface.

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

**Verification:** L1 for the payload shape including the absent-section case;
L2 for the CLI surface and its exit codes. Reconcile
`docs/specs/cli-design.md` in task 7's closure, not here.

### 7. `desktop-surfaces`

**Depends on:** task 6.

**Result:** the three surfaces match their approved documents.

**Files:** `apps/macos/AgentDeckApp/DesktopPreferences.swift`,
`SettingsWindowView.swift`, `MenuBarSurfaceView.swift`, `MenuBarPanelViews.swift`,
`MenuBarViewModel.swift`, `DesktopCopy.swift`, and `Localizable.xcstrings`, with
focused files in `AgentDeckAppTests`;
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
- The specification revision described in `requirements.md` — Contract changes —
  narrowing `docs/specs/cli-design.md:53-55`. Read the then-current revision;
  do not prescribe a revision number, following task 2 (`v0-6-0-contract`) in
  `docs/topics/v0-6-0-contract/tasks.md`, which owns that instruction.

**Verification:** L2 for the Swift surfaces against the documents, plus the
manual acceptance below. Where the implementation and a ux document disagree,
the prototype decides; where the prototype and a document disagree, the
prototype is right and the document is corrected.

## Manual acceptance

Named here because no specimen settles them. Each needs an explicit result —
performed, or waived by the operator with the waiver recorded:

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
Task 3 `claude-adapters` passed Round 3 re-review on 2026-09-11 after two
failed rounds; its delivery state is tracked in Beads
`ad-sq-claude-adapters-dev`. The statusLine prior value lives in AgentDeck's
own state (a sidecar file under the state directory), never in
`~/.claude/settings.json` alongside the `statusLine` key itself;
`quota.MaxStatusLinePayloadBytes` bounds only capture's own parsing, never
what the status-line chain forwards to the prior command. See
[`reviews/claude-adapters.md`](reviews/claude-adapters.md) for the findings,
evidence, and completion gate.
Tasks 4–7 exist in Beads (`ad-sq-gate-and-schedule-dev` through
`ad-sq-desktop-surfaces-dev`) with dependency ordering matching this file; none
have started.

The base of this worktree is `4737076`; `main` has since advanced by ten
commits, including the assembled `schema-version-signal` surfaces. The surface
documents describe this branch's prototype state. Reconciliation belongs to the
`v0-6-0-contract` topic's `assemble` task, not to this topic.
