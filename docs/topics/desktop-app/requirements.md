---
status: active
created: 2026-08-06
updated: 2026-08-18
---

# Native macOS Desktop App — Requirements

Version membership is decided by the [`v0.5.0` contract
topic](../v0-5-0-contract/tasks.md#assembly-list) and mirrored in
[the active-topic status](../../status.md#active-development); this topic
carries no version of its own.

This topic delivers the first native AgentDeck desktop surface after the
`v0.4.0` session-experience feature line documented in the living
[CLI design](../../specs/cli-design.md) and
[manual](../../specs/cli-manual.md). It covers the menu-bar application,
WidgetKit extension, shared Go/Swift contract, unified desktop package,
Homebrew Cask, direct download, signing, notarization, and release validation.

The desktop app is another front end to the existing AgentDeck product. It
does not replace the Go core, create a second state store, or redefine provider,
usage, session, credential, extension, or client-configuration behavior.

## Confirmed Decisions

- The application is native Swift built with the latest stable Xcode and Apple
  SDK available when implementation starts. Release builds never depend on a
  beta Xcode or beta SDK.
- The deployment target is macOS 26.
- The primary Apple frameworks are SwiftUI, `MenuBarExtra`, WidgetKit, App
  Intents, Observation, Swift Concurrency, Swift Charts, OSLog, and
  `SMAppService` where login-item management is needed.
- The Homebrew Cask token is `agentdeck-app`.
- Desktop distribution uses both Homebrew Cask and direct GitHub download.
- The app performs **no update check** in this version. It does not query a
  release endpoint, compare versions, notify about a newer release, or open a
  download page. Consequently the desktop surfaces make no network request at
  all.
- `AgentDeck.app` contains the Swift menu-bar host, WidgetKit extension, and a
  signed universal Go `agentdeck` helper built from the same tag.
- The existing `agentdeck` Homebrew Formula and architecture-specific CLI
  archives remain available for headless, automation, recovery, and existing
  CLI users.
- Formula and Cask are alternative installations. They must not expose two
  independently versioned `agentdeck` executables to the same user state.
- The repository remains one repository with one version tag and one GitHub
  Release per product version.

## Release Sequence

The accepted near-term release order is:

1. `v0.4.0`: session scan progress, search time contract, redesigned session
   text, invocation-level token detail, interactive session viewer, and stable
   desktop-facing session DTOs.
2. `v0.5.0`: this topic — native menu-bar app, desktop widget, application
   package, Cask, direct download, signing, notarization, and release gates.

Releases after `v0.5.0` were re-planned on 2026-08-13 and are recorded in
[the roadmap](../../roadmap.md#roadmap). This topic no longer
defines them.

The extension lifecycle direction previously recorded here — a Go domain engine
with preview, plan, apply, ownership, drift detection, atomic mutation, and
rollback, plus GUI management for Skills, Hooks, Plugins, and MCP servers — was
withdrawn. Extension work is now bounded to cross-client observability, because
each client already owns its own extension management surface while no tool
reports the cross-client view. The specialized `usage hook` lifecycle remains
separate from arbitrary third-party Hooks.

## Goals

- Show current provider, usage, and cost — bounded to today, the trailing
  7 days, and the trailing 30 days, plus a daily trend of at most 90 buckets
  and a 7x24 hour-of-week rhythm view — active or recent sessions, and
  important health state from a persistent menu-bar surface. No other
  historical period or temporal granularity is authorized. Usage and cost may
  additionally be broken down by model, client, **runtime provider**, token
  component, attribution quality (determinable, inferred, unattributed), and
  pricing coverage, as needed to answer the composition and trust questions;
  presentation of these breakdowns is owned by `ux/menubar.md` and
  `ux/widget.md`. Attribution quality may be broken down by both client and
  runtime provider. Aggregating usage by provider is authorized here as its own
  dimension, because showing which provider is currently selected is a routing
  fact and says nothing about whether spend may be attributed to one — and the
  trust question is per-provider by nature: a provider whose events resolve to
  `unknown` is exactly what a determinability figure is meant to expose.
- Provide safe quick actions, initially including provider switching and links
  into detailed session or diagnostic views.
- Give users one dedicated settings window to control whether AgentDeck launches
  at login, whether it refreshes periodically, what the menu-bar item reports,
  and whether that value follows the popover's client filter or covers all
  clients. These four preferences are the complete settings scope for this
  version; `ux/settings.md` owns their defaults, copy, interaction details, and
  failure presentation.
- Publish a privacy-bounded desktop snapshot for WidgetKit through an App
  Group, answering the same four bounded spend questions as the menu bar:
  how much am I spending (magnitude), where does it go (composition), is the
  number real (trust), and when do I actually work (rhythm). The Widget must
  render real product information for the question it answers, not merely
  publish and isolate data; `ux/widget.md` owns the widget count, sizing, and
  presentation of each question.
- Keep all authoritative behavior in the Go implementation and expose a small,
  versioned, typed desktop wire contract to Swift.
- Ship one signed, notarized, universal `AgentDeck.app` that works identically
  through Homebrew Cask and direct download.
- Preserve the existing CLI-only installation and release path.

## Non-Goals

- No App Store distribution in `v0.5.0`.
- No Electron, Tauri, Catalyst, embedded browser, or cross-platform UI toolkit.
- No Swift rewrite of AgentDeck storage, pricing, provider, credential,
  attribution, session, or extension logic.
- No direct Swift or Widget access to `agentdeck.sqlite3`, `sessions.sqlite3`,
  `credential.key`, Codex configuration, Claude configuration, or raw session
  sources.
- No network listener, local HTTP server, background daemon, privileged helper,
  kernel extension, or system extension.
- No update check of any kind, and therefore no automatic download, installation,
  or notification.
- No Skills, Plugins, MCP, or arbitrary Hooks mutation in `v0.5.0`.
- No deletion of user state during App, Cask, Formula, or CLI-link removal.

## Surfaces and contracts

This topic adds three user-visible surfaces, each named by its document so an
audit can compare this list against what exists:

- the menu-bar host — [`ux/menubar.md`](ux/menubar.md)
- the settings window — [`ux/settings.md`](ux/settings.md)
- the WidgetKit widget — [`ux/widget.md`](ux/widget.md)

The settings window is listed separately because it is judged by a different
question against different evidence: the reading surface must give every data
state a presentation rule, while the settings window has no data states and must
instead give every preference a default, an idempotent effect, and an honest
failure presentation.

It introduces new contracts — the desktop wire
contract, the foundation runtime boundary, the App Group projection, and the
packaging and distribution layout — specified in
[`architecture.md`](architecture.md). The document set itself is declared in
[`tasks.md`](tasks.md)'s Documents matrix, not here.

## Acceptance boundary

- The menu bar presents provider, usage, cost, sessions, warnings, and health
  from a single Go-owned snapshot, with no second aggregation layer in Swift.
  Its client and period filters govern **every** panel they sit above; content
  no filter can govern — the fixed 30-day rhythm window — is presented outside
  them and states its own window. Usage and cost analytics are bounded to today, 7-day, and 30-day
  periods, a daily trend of at most 90 buckets, and a 7x24 hour-of-week rhythm
  view, plus breakdowns by model, client, runtime provider, token component,
  attribution quality, and pricing coverage. Attribution quality is reportable
  per client and per runtime provider.
- The Widget renders only from the redacted App Group projection and can reach
  no AgentDeck database, credential, client configuration, or raw session
  source. It renders real product information answering at least one of the
  four bounded spend questions — magnitude, composition, trust, rhythm — not
  merely data publication and isolation; the widget count, sizing, and
  presentation of each question are owned by `ux/widget.md`.
  **Known defect:** this criterion is not met by the shipped build. Its App Group
  identifier lacks a team-ID prefix, so macOS 26 refuses the Widget's container
  and all twelve configurations render the unavailable state. The open P1
  `desktop-widget` finding `DW-R3-F1` owns the repair. Its Apple Developer Team
  prerequisite is satisfied; the installed signed repair candidate receives
  approved host and Widget container access, and all twelve configurations now
  render data. Independent Re-review still owns closure. The criterion above
  remains the contract.
- One signed, notarized, universal `AgentDeck.app` installs and behaves
  identically through Homebrew Cask and direct download, on arm64 and Intel.
- The App, embedded helper, standalone CLI archives, Cask, Formula, release
  title, and tag all report the same release version and source commit.
- The existing CLI-only Formula path keeps working, and no install or uninstall
  path deletes user state.
- The settings window exposes exactly four product preferences: launch at login,
  periodic refresh, menu-bar value, and menu-bar scope. Together they let users
  control whether the app starts with login and refreshes periodically, what the
  menu-bar item reports, and whether that value follows the popover's client
  filter or covers all clients. No other preference is included in this
  version.
- Every preference the app exposes has a default, is idempotent, reports the real
  post-operation state rather than the requested one, and presents an operating
  system refusal in place rather than reverting silently.
- The delivered behavior is reconciled into the living specification and manual
  by this topic's contract task, without raising the specification version.

## Backlog / Future Feature Ideas

- App Store distribution and App Sandbox redesign, if ever desired.
- Any in-app update check, and beyond it automatic download and installation.
  Reintroducing even the check reopens the app's network boundary, so it needs
  its own design rather than a preference toggle.
- Prerelease update channel selection.
- Rich desktop session windows beyond the menu-bar and Widget scope.
- Skills/Hooks GUI lifecycle management, if the withdrawn extension-mutation
  direction is ever reopened.
- Plugins/MCP GUI lifecycle management, under the same condition.
