---
status: active
created: 2026-08-15
---

# Menu-Bar Experience Design

Target: v0.5.0
Task: `menubar-experience`
Owning plan: `docs/plans/desktop-app.md`
Consumes: `docs/specs/macos-app-foundation-design.md`

## Purpose

Define the menu-bar presentation, quick actions, preferences, update
notification, state model, localization, and accessibility contract for the
AgentDeck macOS host.

This document is normative. Product behavior must follow this contract even
when an earlier implementation differs from it.

## Scope

This task delivers:

- an additive `provider.candidates` section in desktop wire v1;
- a Go provider-switch command surface the desktop host can invoke;
- menu-bar summaries for provider, usage, cost, recent sessions, warnings, and
  health;
- safe provider quick actions with explicit confirmation and result reporting;
- manual refresh plus an opt-in periodic refresh preference;
- an `SMAppService` login-item preference;
- an opt-in stable-release update check that only opens the official download
  page;
- six presentation states derived from foundation coordinator state;
- English and Simplified Chinese localization;
- accessibility, keyboard, motion, contrast, and layout behavior with XCTest
  assertions and a manual verification checklist.

## Non-goals

This task does not deliver:

- the WidgetKit extension, its timelines, or its App Intents;
- signing, notarization, universal packaging, Cask, or DMG publication;
- automatic update download, installation, replacement, or relaunch;
- a `~/.local/bin/agentdeck` link action;
- credential creation, editing, or deletion;
- provider definition or wrapper management;
- session browsing beyond the bounded recent-session summary;
- a second refresh state machine, data store, or aggregation layer in Swift.

## Decisions

| Area | Decision |
| --- | --- |
| Candidate source | Desktop wire v1 gains an additive `provider.candidates` array. Go owns candidate discovery, eligibility, and redaction. |
| Switch surface | The host performs a switch through a dedicated helper invocation, not through the snapshot command. |
| Switch safety | Every switch requires explicit user confirmation and reports a typed result. The host never switches implicitly or on refresh. |
| Localization | The UI ships `en` and `zh-Hans` through a String Catalog. All numbers, currency, dates, and relative times use the viewer's locale. |
| Accessibility evidence | Derivable behavior is asserted in XCTest. Assistive-technology and perceptual behavior uses the manual checklist in this document. |
| Refresh cadence | Manual refresh always exists. Periodic refresh is opt-in, defaults off, and uses the snapshot's `next_refresh_at`. |
| Login item | `SMAppService.mainApp` only. Enable and disable are idempotent and never install a daemon. |
| Update check | Opt-in, defaults off, at most once per 24 hours automatic plus unlimited manual. It only compares stable versions and opens a page. |
| Preference storage | Preferences live in the application's own `UserDefaults` domain, never in `~/.agentdeck` and never in the App Group cache. |

## Wire contract extension

### Additive `provider.candidates`

`data.provider` gains one additive array. Existing fields are unchanged, so a
`wire_version` raise is not required and existing decoders keep working.

```json
"provider": {
  "available": true,
  "routes": [ { "client": "codex", "provider": "official",
                "selected_at": "2026-08-13T09:55:00Z", "via_wrapper": false } ],
  "candidates": [
    { "provider": "official", "built_in": true, "clients": ["codex", "claude"],
      "credentials": [], "has_wrapper": false, "ready": true },
    { "provider": "aigocode", "built_in": false, "clients": ["codex"],
      "credentials": [ { "name": "work", "clients": ["codex"], "present": true } ],
      "has_wrapper": true, "ready": true }
  ]
}
```

Each candidate carries only:

- `provider`: the provider name;
- `built_in`: whether this is the built-in `official` provider;
- `clients`: the clients this provider can serve;
- `credentials`: credential shorthand name, its client bindings, and whether
  its secret row is present;
- `has_wrapper`: whether a wrapper URL is configured, never the URL itself;
- `ready`: whether any switch to this provider can be attempted;
- `options`: the fully resolved switch options this candidate expands into.

A candidate is a **grouping for display**, not a mutation target. It may serve
several clients, carry several credentials, and support both a direct and a
wrapper route, so provider-level `ready` cannot say whether one specific switch
is possible. Go therefore expands every executable combination, and the host
never composes one itself:

```json
"options": [
  { "client": "codex", "provider": "aigocode", "credential": "work",
    "via_wrapper": false, "ready": true, "reason_code": null },
  { "client": "codex", "provider": "aigocode", "credential": "work",
    "via_wrapper": true, "ready": true, "reason_code": null },
  { "client": "claude", "provider": "aigocode", "credential": null,
    "via_wrapper": false, "ready": false, "reason_code": "credential_missing" }
]
```

Each option is exactly one executable switch: `(client, provider, credential?,
via_wrapper, ready, reason_code?)`. It maps one-to-one onto the canonical
invocation's arguments, so the option the user confirmed and the command that runs
are provably the same target. `reason_code` is a fixed code with localized copy
in both languages, never a message; the defined codes are
`credential_missing`, `credential_client_mismatch`, `wrapper_not_configured`,
`already_selected`, and `switch_in_flight`.

Selection therefore happens at the option level. A candidate offering more than
one option presents them as distinct rows — credential and route are user choices,
never inferred — and a `ready: false` option is listed and disabled with its
localized reason rather than hidden.

The candidate list MUST NOT contain endpoints, wrapper URLs, credential
references, credential values, multipliers, model mappings, configuration
contents, or paths. `credentials` reports presence only; it never decrypts.

`candidates` is `[]` when the provider section is unavailable.

**Candidate discovery failure MUST NOT hide the current routes.** `routes` and
`candidates` answer different questions from different sources: what is in effect
now, and what could be switched to. Losing the second is an inconvenience; losing
the first removes the surface's primary answer.

| Failure | `available` | `routes` | `candidates` | Presentation |
| --- | --- | --- | --- | --- |
| Route read fails | `false` | `[]` | `[]` | Section shows unavailable; the existing `provider_unavailable` warning applies |
| Candidate discovery fails, routes readable | **`true`** | populated | `[]` | Current routes shown normally; switching disabled with a localized reason |

An empty `candidates` with `available: true` therefore means "switching is not
offered right now", not "no provider information". The host renders the switch
affordance as disabled with `Switching unavailable` / `暂时无法切换` rather than
omitting it, so the capability's absence is visible instead of silent.

A candidate whose `ready` is `false` is listed but not selectable, with a
localized reason derived from its own fields — a credential whose `present` is
`false` reads `Credential missing` / `缺少凭据` — never a bare disabled row.

Compatibility must hold in **both** directions, and only one of them is
self-evident. A current v1 decoder ignores the unknown `candidates` key, so a new
producer works with an old consumer. The reverse is the risk: a new Swift model
that declares `candidates` non-optional would fail to decode an existing v1
payload that predates the field, and every stored App Group snapshot is exactly
such a payload.

Therefore, without raising `wire_version`:

- the producer always encodes `candidates` as an array, empty when there is
  nothing to offer;
- a decoder treats a **missing** `candidates` as `[]`, and a present value that
  is not an array as invalid;
- the same rule applies to `options` within a candidate.

Fixtures under `desktop/fixtures/v1` gain the field in the complete and partial
examples, and **one legacy fixture without `candidates` is retained** so the
old-payload path stays covered. Replacing every fixture would delete the only
regression signal for the direction that can actually break. Go contract tests
and the Swift decoder consume the same files, and the Swift tests assert that the
legacy fixture decodes with an empty candidate list.

### Switch command surface

The desktop host performs a provider switch through:

```text
agentdeck --quiet --format json provider use <name> --client <codex|claude> \
  [--credential <name>] [--via] --no-shell-setup
```

This exact argument list is the only canonical invocation. It uses the existing
command; the host adds no new mutation command.

`--quiet` is required, not optional. `--format json` alone does **not** silence
the advisory reporters: `provider use` calls `reportEffectiveRoute` and
`reportSwitchAdvisories` after a successful switch, and both return early only on
`opts.quiet`, so without the flag a successful switch writes the effective
endpoint and restart guidance to stderr. That would put an endpoint on a surface
this design forbids from ever presenting one, and it would break the rule below
that a successful switch produces no stderr at all.

`--no-shell-setup` is passed explicitly so the behavior does not depend on any
suppression remaining implicit.

Required stream behavior, which the implementation MUST test with the exact
arguments above:

| Outcome | stdout | stderr | exit |
| --- | --- | --- | --- |
| Success | One envelope with no `error` | **Empty** | `0` |
| Failure | Empty | One envelope with `error` | non-zero |

The host MUST:

- resolve the exact provider, client, and credential from the confirmed
  candidate rather than from free-form input;
- pass arguments as an array through the foundation helper runner;
- treat `state_busy`, `invalid_argument`, and every other stable error code as a
  typed failure category;
- trigger one replacement refresh after a successful switch;
- leave presented state unchanged after a failed switch, then surface the typed
  failure.

The host MUST NOT retry a failed switch automatically, switch more than one
client per confirmation, or infer a credential when the candidate offers more
than one.

#### Result envelope

The switch needs its own Swift type, `ProviderUseEnvelopeV1`. It cannot reuse
`DesktopWireEnvelopeV1`, whose `command` is fixed to `desktop.snapshot` and whose
`data` is a non-optional `DesktopSnapshotV1`; `provider use` emits
`command: "provider.use"` with `data: null` on both success and failure, so the
existing decoder cannot represent it.

```json
{
  "schema_version": 1,
  "command": "provider.use",
  "generated_at": "<RFC 3339>",
  "data": null,
  "warnings": [],
  "partial": false,
  "error": { "code": "<stable code>", "message": "<discarded>" }
}
```

The type fixes `schema_version` to `1` and `command` to `provider.use`, decodes
`data` as ignored-and-nullable, and retains only `error.code`. It never retains
`error.message`; see the prerequisite below.

Outcome is decided by the envelope, never by the exit status alone:

| Streams and status | Host behavior |
| --- | --- |
| stdout envelope, no `error`, empty stderr, exit `0` | **Success.** Trigger one replacement refresh |
| stderr envelope with `error`, non-zero exit | **Typed failure.** Map `error.code` to a localized message |
| Envelopes on both streams, or `error` present with exit `0`, or absent with non-zero exit | **Indeterminate.** Never guess which is authoritative |
| Unknown `schema_version` or `command`, malformed, truncated, or no valid JSON on either stream | **Opaque failure.** Never parse the text |

An indeterminate outcome is not a failure and not a success: the configuration
may or may not have been applied. The host MUST force a replacement refresh and
reconcile from the resulting snapshot rather than asserting either result.

The current `EmbeddedHelperRunner` cannot serve this contract as written: it
guards on a zero exit status and throws before decoding, so a stderr failure
envelope is unreachable through it. The switch path therefore needs a runner
entry point that captures both streams and the exit status and hands all three to
`ProviderUseEnvelopeV1` for classification. The snapshot path keeps the existing
strict behavior unchanged.

One CLI defect is recorded here as a prerequisite rather than silently absorbed
by the GUI, because a GUI workaround would hide it from every other consumer: a
failed switch for an unknown provider reports `error.code: runtime_error`, which
no specification defines as a stable code, and its `error.message` carries the
underlying storage text `sql: no rows in result set`.

Until that is fixed, the host treats any `error` as a failure regardless of code
and displays the code verbatim beside a generic localized explanation, never the
raw message. A message that may contain internal storage text MUST NOT be shown
to the user or written to a log.

#### Operation ownership

One **app-owned** `SwitchController` owns switching, not the view model and not a
per-client object. It outlives the menu-bar window, because the window is
dismissed whenever the user clicks elsewhere in the menu bar and an in-flight
configuration change must not depend on a presentation lifetime. The view model
observes it; views never hold it.

It is **globally single-flight**: at most one switch exists across all clients.
A per-client limit would be wrong here — a switch rewrites client configuration
and completes in well under a second, so concurrency buys nothing and costs the
ability to reason about which configuration state is current.

State: `idle`, `inFlight(client, provider)`, `succeeded(client, provider)`,
`failed(code)`, or `indeterminate`.

| Concern | Rule |
| --- | --- |
| Serialization | One switch app-wide. A request while not `idle` is refused, not queued |
| Double submit | Structurally impossible: confirmation's controls are disabled on entry to `inFlight`, so no second submit can be issued |
| Cancellation | The controller never cancels an in-flight switch, and MUST NOT invoke the runner through a cancellable task whose cancellation terminates the helper. `EmbeddedHelperRunner`'s `withTaskCancellationHandler` calls `running.terminate()` on cancel, which would kill the helper mid-write; the switch path must not be cancelled by window dismissal, view teardown, or task cancellation |
| Window dismissal | Detaches presentation only. On reopen the controller's current state is displayed, including a result that became terminal while closed |
| Timeout after launch | Becomes `indeterminate`, never `failed`. The helper may already have written the client configuration, so the controller forces a replacement refresh and reconciles health and recovery state from the snapshot |
| Success lifetime | Cleared by the next completed replacement refresh, or after 10 seconds, whichever comes first |
| Failure and indeterminate lifetime | Retained until the user retries or dismisses. These are never cleared by a timer, because a failure the user did not see is a failure that gets repeated |

The two lifetimes differ deliberately: a success is confirmed by the refreshed
data that replaces it, while a failure has no such successor and must wait to be
read.

## Presentation architecture

### Layering

```text
MenuBarExtra scene
    |
    | reads
    v
MenuBarViewModel            (@MainActor, @Observable)
    |
    | derives presentation from, and requests operations on
    v
DesktopRefreshCoordinator   (foundation-owned, unchanged authority)
```

`MenuBarViewModel` is the only place that converts coordinator state into
presentation values. It owns no refresh state machine, performs no process
execution, no wire decoding, and no App Group access. Views read the view model
and never compute freshness, formatting, or availability themselves.

Preference state (`periodic refresh`, `login item`, `update check`) lives in a
separate `DesktopPreferences` type so presentation and preference persistence
stay independently testable.

### Presentation state

Presentation is derived from foundation coordinator state. No new state machine is
introduced, but the six names are **not** mutually exclusive, so treating them as
one enum leaves real combinations undefined: `degraded(previous, timeout)` is both
stale and offline, and `degraded(previous, invalidWire)` is both stale and error.

The model is therefore one **surface** plus independent **qualifiers**.

The surface answers only one question — is there a snapshot to show?

| Surface | Condition |
| --- | --- |
| `loadingSurface` | No snapshot has ever been retained |
| `dataSurface` | A snapshot is retained, whatever its age or issue |
| `errorSurface` | No snapshot is retained and the coordinator is `degraded` |

**A retained snapshot is always shown.** No issue category, including
`invalidWire`, replaces existing data with an empty surface; the issue becomes a
qualifier on top of it. This resolves the earlier contradiction where the error
copy demanded showing nothing while the retention rule demanded showing the
snapshot.

Qualifiers are orthogonal and may all apply at once:

| Qualifier | Condition |
| --- | --- |
| `stale` | Coordinator is `refreshing` or `degraded` while a previous snapshot is retained |
| `aged` | Retained snapshot's `generated_at` is older than 15 minutes |
| `partial` | Snapshot's `partial` flag is set, or any section reports `available: false` |
| `offline` | Issue category is helper launch, missing helper, or timeout |
| `failing` | Issue category is invalid wire or storage unavailable |
| `empty` | Not `partial`, every section is `available: true`, and none carries rows, tokens, or costs |

Exhaustive truth table over coordinator state, issue category, and whether a
previous snapshot exists:

| Coordinator | Previous snapshot | Surface | Qualifiers |
| --- | --- | --- | --- |
| `uninitialized` | — | `loadingSurface` | none |
| `refreshing` | no | `loadingSurface` | none |
| `refreshing` | yes | `dataSurface` | `stale`, plus `aged`/`partial`/`empty` as they hold |
| `ready` | — | `dataSurface` | `partial` or `empty` as they hold |
| `degraded`, launch/missing/timeout | yes | `dataSurface` | `stale` + `offline`, plus `aged`/`partial` |
| `degraded`, launch/missing/timeout | no | `errorSurface` | `offline` |
| `degraded`, invalid wire/storage | yes | `dataSurface` | `stale` + `failing`, plus `aged`/`partial` |
| `degraded`, invalid wire/storage | no | `errorSurface` | `failing` |

`empty` and `partial` are mutually exclusive by construction: `empty` requires
every section available, which `partial` denies.

Fixed qualifier order wherever more than one is shown — freshness first, then
reachability, then completeness: `stale`, `aged`, `offline`, `failing`,
`partial`, `empty`. The accessibility value announces them in the same order, and
the menu-bar glyph badges only when `offline` or `failing` holds, regardless of
which surface is showing.

Staleness age is computed from the snapshot's `generated_at`. A retained
snapshot older than 15 minutes is additionally labeled with its age. The host
never discards a valid snapshot because of age.

### Sections

The menu-bar window presents, in order:

1. **Provider** — one row per client with its current provider, wrapper
   indication, and selection time. Clients with no recorded selection show an
   explicit unconfigured label, not a blank.
2. **Usage** — the current local-day range, token totals, and event/session
   counts.
3. **Cost** — provider cost and catalog base cost. When `pricing_complete` is
   false, the known-cost values are shown with an explicit incomplete-pricing
   qualifier and the unpriced-component count. A cost is never presented as
   complete when the snapshot says it is not.
4. **Recent sessions** — at most the snapshot's rows, each with client, project
   basename, model, and relative last-activity time. Session identifiers are
   never displayed.
5. **Health** — aggregate status with problem, warning, and error counts. Failed
   checks list name, status, and code. Recovery commands are shown as copyable
   text and are never executed by the app.
6. **Warnings** — snapshot-level warning codes rendered as localized
   explanations. An unrecognized code is shown verbatim as a code, never
   silently dropped.

Every section that reports `available: false` shows an explicit unavailable
label in place of its values.

### Actions

| Action | Behavior |
| --- | --- |
| Refresh now | Requests a replacement refresh. Disabled while a refresh is active. |
| Switch provider | Opens a confirmation naming the client, provider, credential, and wrapper route. Performs the switch only after confirmation. |
| Copy recovery command | Copies the snapshot's recovery command text to the pasteboard. |
| Open Settings | Opens the preferences scene. |
| Check for updates | Performs one manual update check. |
| Quit | Terminates the application. |

No action mutates AgentDeck state except `Switch provider`. No action executes a
recovery command, opens a database, or reads a client configuration file.

## Preferences

| Preference | Default | Behavior |
| --- | --- | --- |
| Periodic refresh | Off | When on, schedules the next refresh from `next_refresh_at`. Never runs while the menu is closed if the app is inactive-suspended; a missed interval refreshes once on reactivation. |
| Launch at login | Off | Registers or unregisters `SMAppService.mainApp`. The UI reports the service's actual current status, including `requiresApproval`. |
| Update check | Off | When on, permits at most one automatic check per 24 hours. |

Preference operations MUST be idempotent, MUST report the real post-operation
state rather than the requested state, and MUST NOT install a launch daemon,
launch agent, or privileged helper.

## Update check

The host may issue one unauthenticated `GET` to the official AgentDeck GitHub
latest-stable-release endpoint. The request MUST NOT carry AgentDeck state,
usage, provider, session, machine identifier, credential, or custom tracking
header.

The host compares the running application's version with the returned stable
tag. Prereleases are ignored. When a newer stable version exists, the host shows
the version and offers to open the official release page through the system
browser.

Network, HTTP, decoding, comparison, and browser-open failures are non-fatal,
never reduce local snapshot availability, and never block any other action. The
app never downloads, installs, replaces, relaunches, or requests privilege.

Update-check outcome MUST NOT be written to the App Group cache.

## Localization

The UI ships one String Catalog with `en` and `zh-Hans`. Every user-visible
string is localized, including state qualifiers, warning-code explanations,
health statuses, confirmation text, and accessibility labels.

- Numbers, token counts, and costs use locale-aware formatting. Cost strings
  arrive as decimal strings and are formatted for display without changing the
  underlying value or rounding a value the snapshot reported as exact.
- Timestamps and relative times use locale-aware formatting from the snapshot's
  RFC 3339 values.
- Provider names, client names, model names, project basenames, warning codes,
  health-check names, and recovery commands are data and are never translated.
- Layout uses leading/trailing alignment so the window is correct under
  right-to-left system settings even though no RTL language ships in v0.5.0.

## Menu-bar item

The item in the menu bar is the only part of this app visible at all times, so it
is specified before the window.

| Aspect | Specification |
| --- | --- |
| Glyph | One SF Symbol rendered as a template image, so macOS owns light, dark, and highlight appearance. No custom bitmap, no color glyph. |
| Base glyph | `gauge.with.needle` — it reads as a live local measurement rather than a document or a cloud service. |
| Status overlay | None in the normal case. The glyph gains a badge only for `error` and `offline`, using `exclamationmark.triangle.fill` as the badged variant. `stale` and `partial` are window-level qualifiers and MUST NOT badge the menu bar. |
| Text | Never. No token count, cost, or provider name is rendered in the menu bar. Cost in particular MUST NOT sit permanently on screen where a screen share or a passer-by can read it. |
| Animation | None, ever — including during refresh. A moving menu-bar item draws attention that the underlying event does not warrant. |
| Accessibility label | Localized app name plus, when badged, the state: for example `AgentDeck — 离线` / `AgentDeck — offline`. |

Restricting the badge to `error` and `offline` is deliberate: those two mean the
data on display cannot be trusted at all, while a stale or partial snapshot is
still usable and is qualified where the user actually reads it.

## Visual contract

Values are the specification, not suggestions, because an unbounded menu-bar
window is a real failure mode: this surface has six sections whose content grows
with health checks and warnings.

### Geometry

| Property | Value |
| --- | --- |
| Window width | Fixed 340 pt at default Dynamic Type |
| Narrow bound | 280 pt — the minimum width every layout MUST remain correct at |
| Maximum height | 560 pt, or 70% of the shortest visible screen's height, whichever is smaller |
| Content overflow | The section stack scrolls vertically inside the height bound. The window never grows past it and never clips a row |
| Row minimum height | 28 pt, so a pointer target stays comfortable |

### Typography and spacing

Use semantic text styles only, so Dynamic Type and Increase Contrast work without
per-view arithmetic. No fixed point sizes for text.

| Element | Style |
| --- | --- |
| Section header | `.subheadline` with `.secondary` foreground |
| Row label | `.body` |
| Row value | `.body`, monospaced digits for numbers, tokens, costs, and times |
| Qualifier and warning text | `.caption` |
| Recovery command | `.caption` monospaced |

Spacing uses a 4 pt scale: 4 within a row, 8 between rows, 16 between sections,
and 12 window padding. No other values.

### Color and status

Semantic colors only — `.primary`, `.secondary`, `.tint`, and the system
semantic status colors. No fixed hex values anywhere.

Every status carries a text label and an SF Symbol shape in addition to color, so
the status survives grayscale, Increase Contrast, and color-vision differences:

| Status | Symbol | Color role |
| --- | --- | --- |
| Healthy | `checkmark.circle` | `.secondary` |
| Warning | `exclamationmark.triangle` | Semantic warning |
| Error | `xmark.octagon` | Semantic error |
| Unavailable / unconfigured | `minus.circle` | `.secondary` |

`.secondary` for healthy is intentional: a healthy system should be quiet rather
than decorated with green.

### Density control

Two sections grow without bound and MUST be bounded in presentation:

| Section | Rule |
| --- | --- |
| Health | Show aggregate status plus at most 3 failed checks, then a localized "and N more" row that expands in place |
| Warnings | Show at most 3 warnings, then the same expandable overflow row |

Recent sessions are already bounded by the snapshot. Sections whose data is
absent collapse to a single unavailable row rather than an empty header.

## Interaction states and copy

The surface and qualifier model above defines *derivation*. This defines what the
user reads. Each surface has one copy; each qualifier has its own, and they
compose in the fixed order.

| Surface | `en` | `zh-Hans` | Data shown |
| --- | --- | --- | --- |
| `loadingSurface` | `Loading…` | `加载中…` | Nothing yet, no spinner beyond the system idiom |
| `dataSurface` | No surface copy; qualifiers speak | 同上 | The retained snapshot in full, always |
| `errorSurface` | The specific failure, never a generic message | 同上 | No data exists to show, plus a retry affordance |

| Qualifier | `en` | `zh-Hans` |
| --- | --- | --- |
| `stale` | `Updated <relative>` | `<相对时间>更新` |
| `aged` | `Last updated <relative>` | `上次更新于<相对时间>` |
| `offline` | `Cannot reach the AgentDeck helper` | `无法连接 AgentDeck 助手` |
| `failing` | `Data could not be read` | `无法读取数据` |
| `partial` | `Some data unavailable` | `部分数据不可用` |
| `empty` | `No local activity today` | `今天没有本地活动` |

The user never reads the words "stale" or "offline" as labels. Saying when data
was updated is honest and actionable; calling it stale is a judgment the user did
not ask for. The reachability copy names the helper, because the network is not
what failed.

`aged` replaces `stale`'s wording rather than adding to it — one freshness
statement, not two. Every other combination appears in the fixed order:
`stale`/`aged`, `offline`, `failing`, `partial`, `empty`.

### Update check copy

Manual and automatic checks MUST read differently, because the user initiated one
and not the other:

| Situation | `en` | `zh-Hans` |
| --- | --- | --- |
| Manual, up to date | `You're on the latest version` | `已是最新版本` |
| Manual, newer exists | `Version <v> is available` with an `Open download page` action | `有新版本 <v>` + `打开下载页` |
| Manual, check failed | `Could not check for updates` | `无法检查更新` |
| Automatic, newer exists | Same wording, shown as a passive row — never a modal, never a badge | 同上 |
| Automatic, up to date or failed | Silent. Nothing is shown | 同上 |

An automatic check that finds nothing MUST stay silent. Reporting a successful
no-op is noise.

### Provider switch flow

The one mutating action, specified end to end:

1. **Selection** — choosing one `option` opens confirmation, carrying that
   option's exact `(client, provider, credential?, via_wrapper)`. A candidate is
   never selected as a whole, and the row is never itself the commit.
2. **Confirmation** — names client, provider, credential, and wrapper route in
   one sentence, with `Switch` and `Cancel`. `Cancel` is the default focus.
3. **In flight** — the confirmation stays open with its controls disabled and a
   progress indicator. Every other action on the surface is disabled. A second
   submit is impossible because the control is gone, not merely ignored.
4. **Success** — the confirmation closes, the switched row updates from the next
   refresh, and a transient row states `Switched <client> to <provider>` /
   `已将 <client> 切换到 <provider>`. It clears on the next refresh or after 10
   seconds.
5. **Failure** — the confirmation stays open, shows the localized failure with
   its code, and offers retry or cancel. The previously selected provider is
   still displayed as current, because it still is.
6. **Cancellation** — closing the window during an in-flight switch does not
   cancel it; the operation completes and its outcome appears on next open. The
   app never abandons a half-applied configuration change.

7. **Indeterminate** — a timeout after the helper launched leaves the outcome
   unknown. The confirmation reports that the result could not be confirmed,
   offers to reopen after a refresh, and the app forces a replacement refresh.
   It MUST NOT claim either success or failure.

Switching is globally single-flight, as specified under Operation ownership: one
switch app-wide, and a request while one is active is refused rather than queued.

## Accessibility, motion, contrast, and layout

The menu-bar window MUST:

- give every row an accessibility label, value, and — where the row is
  actionable — a hint, so VoiceOver announces meaning rather than raw glyphs;
- expose every active qualifier (`stale`, `aged`, `offline`, `failing`,
  `partial`, `empty`), in the fixed order, as part of the
  accessible value, not only as color or an icon;
- reach every control by keyboard, with a visible focus indicator and a logical
  focus order matching visual order;
- honor `accessibilityReduceMotion` by replacing any transition or progress
  animation with an immediate state change;
- honor `accessibilityIncreaseContrast` and Dark/Light appearance using semantic
  colors, never fixed hex values;
- never encode meaning in color alone: every status carries text or a shape;
- remain usable at the narrow layout bound and at large Dynamic Type sizes, with
  wrapping rather than truncation for state and warning text.

## Security and privacy

- The host reads only the foundation coordinator's snapshot state and the
  application's own preference domain.
- The switch action is the only mutation, requires confirmation, and passes an
  argument array through the foundation runner with no shell.
- The pasteboard receives only recovery-command text the snapshot already
  contains.
- Session identifiers, endpoints, wrapper URLs, credential references,
  credential values, configuration contents, and file paths are never displayed,
  logged, copied, or persisted.
- Logging keeps the foundation's fixed-classification policy. New events use
  fixed codes for switch attempt, switch outcome category, preference change,
  and update-check outcome category. No dynamic value is logged.
- The App Group cache contract is unchanged. Menu-bar work adds no field and
  never writes the cache directly.
- Tests use synthetic snapshots, synthetic candidates, an injected preference
  domain, and a stubbed switch and update transport. No test reads real
  AgentDeck, Codex, Claude, App Group, or network state.

## Verification

Verification level L3.

### Go

- `internal/desktop` candidate construction: available, unavailable, built-in
  and custom providers, multiple credentials, absent secret, configured wrapper,
  and the negative assertion that no endpoint, reference, multiplier, or URL
  reaches the snapshot.
- Fixture reproducibility for the extended complete and partial envelopes.
- `scripts/run-go-test.sh ./...` because this changes a persisted JSON contract.

### Swift

- Every row of the surface-and-qualifier truth table, including the combinations
  that hold together, and the rule that a retained snapshot is always shown.
- Section rendering for available, unavailable, empty, and incomplete-pricing
  inputs, including the assertion that an incomplete cost is never labeled
  complete.
- Warning-code mapping including an unrecognized code.
- Switch flow: confirmation required, exact resolved arguments, success
  triggering one replacement refresh, and each typed failure category leaving
  presented state unchanged.
- Preferences: idempotent enable and disable, real-status reporting, and the
  24-hour automatic update-check bound with manual checks unaffected.
- Update check: newer stable, same version, older version, prerelease ignored,
  and each failure category being non-fatal.
- Localization: both catalogs resolve every key, and no key falls back to its
  identifier.
- Accessibility-derivable behavior: label/value/hint presence, qualifier in the
  accessible value, focus order, reduce-motion branch, and narrow-layout branch.
- Privacy: negative assertions that prohibited values never appear in presented
  strings, pasteboard content, or log classifications.

### Manual checklist

Recorded in the review record with the observed result for each item on macOS 26:

1. VoiceOver reads every section and action in a sensible order with meaningful
   labels.
2. Full keyboard access reaches every control with a visible focus indicator.
3. Reduce Motion removes transitions and progress animation.
4. Increase Contrast keeps every status legible.
5. Light and Dark appearance switching keeps every status legible.
6. `en` and `zh-Hans` both render without truncation or clipping.
7. At the 280 pt narrow bound, state and warning text wraps rather than
   truncates, at default and at the largest Dynamic Type size.
8. Content exceeding the height bound scrolls inside it; the window neither grows
   past the bound nor clips a row.
9. The menu-bar glyph is unbadged when only `stale`, `aged`, `partial`, or
   `empty` hold, and badged whenever `offline` or `failing` holds, on either
   surface. It never animates and never renders text.
10. Health and warning sections show at most three rows plus a working overflow
    row that expands in place.
11. A switch in flight disables every other action, and the confirmation's own
    controls, so a second submit cannot be issued.
12. A failed switch keeps the previous provider displayed as current and shows
    the failure code without the underlying message text.
13. Candidate discovery failure with readable routes still shows current routes,
    with the switch affordance visibly disabled rather than absent.
14. An automatic update check that finds nothing shows nothing.
15. A successful switch through the canonical invocation produces an empty
    stderr, confirming `--quiet` suppresses the route and advisory reporters.
16. A candidate with several options presents one row per option; credential and
    direct-versus-wrapper are never inferred, and each `ready: false` option is
    listed and disabled with its localized reason.
17. The legacy v1 fixture without `candidates` decodes with an empty candidate
    list, and a present non-array value is rejected.
18. A switch is refused, not queued, while another is in flight anywhere in the
    app.
19. A timeout after helper launch reports an unconfirmed result, forces a
    replacement refresh, and claims neither success nor failure.
20. Window dismissal during an in-flight switch does not terminate the helper,
    and the outcome is visible on reopen.

### Not verifiable in this task

Signing, notarization, universal helper assembly, Cask and DMG installation, and
Widget behavior belong to later tasks. Record them as out of scope rather than as
untested risk.

## Acceptance criteria

- Desktop wire v1 carries `provider.candidates` additively, Go owns its
  redaction, and both fixtures and both decoders agree.
- The menu bar presents all six sections with explicit unavailable, empty, and
  incomplete-pricing handling.
- All six presentation states derive from foundation coordinator state with no
  second state machine.
- A provider switch happens only after explicit confirmation, uses exactly the
  confirmed candidate, and reports a typed outcome.
- Manual refresh always works; periodic refresh, login item, and update check
  are opt-in, default off, idempotent, and report real state.
- The update check only compares stable versions and opens a page.
- Every user-visible string resolves in `en` and `zh-Hans`.
- Accessibility-derivable behavior is asserted in XCTest and the manual
  checklist is recorded with observed results.
- No prohibited value appears in presented text, the pasteboard, logs, or the
  App Group cache.

## Downstream contracts

`desktop-widget` reads only the unchanged presentation-safe App Group
projection. The new candidate data stays in the host process.

`unified-desktop-distribution` owns signing, notarization, universal assembly,
and the App Group entitlement in release artifacts. The update check's endpoint
and the release page it opens must agree with the release assets that task
publishes.

`desktop-app-contract` reconciles the delivered menu-bar behavior, the additive
candidate contract, and the update-check behavior into `docs/specs/cli-design.md`
and `docs/specs/cli-manual.md`.

## Approval boundary

Design approval authorizes this contract to become the implementation and review
authority for `menubar-experience`. It does not authorize repair, review, commit,
push, signing, notarization, release, installation, or work on later desktop
tasks.
