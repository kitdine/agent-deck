---
status: active
created: 2026-08-06
updated: 2026-08-16
---

# Native macOS Desktop App — Requirements

Version membership is decided by the [`v0.5.0` contract
topic](../v0-5-0-contract/tasks.md#assembly-list) and mirrored in
[the documentation index](../../README.md#active-development); this topic
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
- The direct-download app checks only whether a newer stable release exists,
  then offers to open the release download page. It does not download, install,
  replace, or relaunch itself.
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
[the documentation index](../../README.md#roadmap). This topic no longer
defines them.

The extension lifecycle direction previously recorded here — a Go domain engine
with preview, plan, apply, ownership, drift detection, atomic mutation, and
rollback, plus GUI management for Skills, Hooks, Plugins, and MCP servers — was
withdrawn. Extension work is now bounded to cross-client observability, because
each client already owns its own extension management surface while no tool
reports the cross-client view. The specialized `usage hook` lifecycle remains
separate from arbitrary third-party Hooks.

## Goals

- Show current provider, current-day usage, cost, active or recent sessions,
  and important health state from a persistent menu-bar surface.
- Provide safe quick actions, initially including provider switching and links
  into detailed session or diagnostic views.
- Publish a privacy-bounded desktop snapshot for WidgetKit through an App Group.
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
- No automatic update download or installation.
- No Skills, Plugins, MCP, or arbitrary Hooks mutation in `v0.5.0`.
- No deletion of user state during App, Cask, Formula, or CLI-link removal.

## Surfaces and contracts

This topic adds two user-visible surfaces, each named by its document so an
audit can compare this list against what exists:

- the menu-bar host — [`ux/menubar.md`](ux/menubar.md)
- the WidgetKit widget — [`ux/widget.md`](ux/widget.md)

It introduces new contracts — the desktop wire
contract, the foundation runtime boundary, the App Group projection, and the
packaging and distribution layout — specified in
[`architecture.md`](architecture.md). The document set itself is declared in
[`tasks.md`](tasks.md)'s Documents matrix, not here.

## Acceptance boundary

- The menu bar presents provider, usage, cost, recent sessions, warnings, and
  health from a single Go-owned snapshot, with no second aggregation layer in
  Swift.
- The Widget renders only from the redacted App Group projection and can reach
  no AgentDeck database, credential, client configuration, or raw session
  source.
- One signed, notarized, universal `AgentDeck.app` installs and behaves
  identically through Homebrew Cask and direct download, on arm64 and Intel.
- The App, embedded helper, standalone CLI archives, Cask, Formula, release
  title, and tag all report the same release version and source commit.
- The existing CLI-only Formula path keeps working, and no install or uninstall
  path deletes user state.
- The delivered behavior is reconciled into the living specification and manual
  by this topic's contract task, without raising the specification version.

## Backlog / Future Feature Ideas

- App Store distribution and App Sandbox redesign, if ever desired.
- Automatic update download and installation; `v0.5.0` only opens the download
  page.
- Prerelease update channel selection.
- Rich desktop session windows beyond the menu-bar and Widget scope.
- Skills/Hooks GUI lifecycle management, if the withdrawn extension-mutation
  direction is ever reopened.
- Plugins/MCP GUI lifecycle management, under the same condition.
