---
status: active
created: 2026-08-06
---

# Native macOS Desktop App

Target release: `v0.5.0`.

This plan delivers the first native AgentDeck desktop surface after the
`v0.4.0` session-experience feature line documented in the living
[CLI design](../specs/cli-design.md) and [manual](../specs/cli-manual.md). It covers the menu-bar application,
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
2. `v0.5.0`: this plan — native menu-bar app, desktop widget, application
   package, Cask, direct download, signing, notarization, and release gates.
3. `v0.6.0`: Skills and third-party Hooks lifecycle core, GUI management, and
   thin deterministic CLI recovery/automation commands.
4. `v0.7.0`: Plugins and MCP server lifecycle adapters and their GUI surfaces.

The lifecycle releases use a Go domain engine with preview, plan, apply,
ownership, drift detection, atomic mutation, rollback, and doctor behavior.
The GUI is the primary interactive management surface; the CLI remains a thin
non-interactive automation and recovery surface. The existing specialized
`usage hook` lifecycle remains separate from arbitrary third-party Hooks.

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

## Architecture

### Application processes

```text
AgentDeck.app / SwiftUI menu-bar host
    |
    | versioned request and JSON response
    v
AgentDeck.app/Contents/Helpers/agentdeck
    |
    | existing Go services and stores
    v
~/.agentdeck and approved Codex/Claude read or mutation boundaries

SwiftUI menu-bar host
    |
    | redacted snapshot in App Group
    v
AgentDeckWidget.appex / WidgetKit
```

The menu-bar host invokes only the helper embedded in its own signed App bundle,
never an arbitrary `agentdeck` found on `PATH`. This binds GUI behavior and wire
contract to the same release. The helper continues to use the normal AgentDeck
state root and client configuration paths.

The Swift host must not concatenate shell commands. It launches the helper with
an argument array, bounded environment, bounded stdout/stderr capture, explicit
timeouts, cancellation, and typed exit handling. Secrets and raw session content
must never enter OSLog or the widget snapshot.

### Desktop wire contract

The first implementation must decide whether existing public JSON commands are
sufficient or a dedicated desktop snapshot command is necessary. If a new
command is added, it must be a versioned product contract rather than an
unreviewed private parser dependency.

The contract must provide one coherent snapshot for the menu-bar refresh cycle,
including:

- AgentDeck and wire-contract versions;
- current provider and client routing state;
- bounded usage summary and pricing completeness;
- bounded recent-session summary using the `v0.4.0` session DTO contract;
- doctor or integration state safe for routine display;
- warnings and partial-result semantics;
- generated time and next suggested refresh time.

Swift decodes the response into explicit `Codable`, `Sendable` types and rejects
unsupported wire versions. Go owns all semantic aggregation and redaction.

### macOS security boundary

The menu-bar host is Developer ID signed, notarized, hardened, and not App
Sandboxed because the product must access user-owned AgentDeck, Codex, and Claude
paths outside an application container. Entitlements remain minimal.

The WidgetKit extension remains sandboxed. It receives only a redacted App Group
snapshot written atomically by the host. The snapshot excludes credentials,
credential references, provider headers, configuration file contents, source
paths, raw session content, prompts, responses, and tool arguments.

Login launch uses `SMAppService` and is opt-in. The app must explain current
state, make enable/disable idempotent, and never install a separate daemon.

### Update notification

At most once per documented interval, plus an explicit manual check, the host may
query the official AgentDeck GitHub release endpoint for the latest stable tag.
Prereleases are ignored unless a later design introduces an explicit opt-in
channel. The request sends no AgentDeck state, usage, provider, session, or
machine identifier.

When a newer compatible stable version exists, the app shows release version and
offers to open its official download page through the system browser. Network,
HTTP, parsing, or browser-open failure is non-fatal and never blocks normal app
use. There is no background download, package replacement, relaunch, or privilege
request.

## Packaging and Distribution

### App bundle

The intended signed layout is:

```text
AgentDeck.app/
  Contents/
    MacOS/AgentDeck
    Helpers/agentdeck
    PlugIns/AgentDeckWidget.appex
    Resources/completions/
    Info.plist
```

The Go helper is built separately for arm64 and amd64, joined into a universal
Mach-O binary, then signed before the enclosing Widget and App signatures. The
release workflow signs nested code in inside-out order, creates the distribution
image, submits it for notarization, staples the result, and verifies Gatekeeper
assessment before publication.

### Release assets

One annotated tag produces one GitHub Release containing at least:

```text
AgentDeck_v0.5.0_universal.dmg
AgentDeck_v0.5.0_universal.zip
agentdeck_v0.5.0_darwin_arm64.tar.gz
agentdeck_v0.5.0_darwin_amd64.tar.gz
AgentDeck_v0.5.0_checksums.txt
```

The App, embedded helper, standalone CLI archives, Cask, Formula, release title,
and tag must report the same release version and source commit.

### Homebrew channels

- `kitdine/tap/agentdeck` remains the CLI-only Formula.
- `kitdine/tap/agentdeck-app` is the full desktop Cask.
- The Cask installs `AgentDeck.app` and exposes its embedded helper and shell
  completions through supported Homebrew Cask artifacts.
- Formula and Cask installation together must be rejected or produce an explicit
  migration path before either can own the global `agentdeck` command.
- Cask uninstall removes App-owned artifacts only and preserves `~/.agentdeck`
  and unrelated Codex, Claude, and shell configuration.

The Cask update remains a pull request into `kitdine/homebrew-tap`, analogous to
the current Formula flow. Publication never directly pushes the tap default
branch.

### Direct download

The same notarized DMG published for the Cask is the direct-download artifact.
Dragging the App into `/Applications` is sufficient for GUI and Widget use.

An optional Settings action may expose the embedded helper through
`~/.local/bin/agentdeck`. It creates a link rather than copying a second binary.
Before mutation it must classify an existing path as absent, same App-owned link,
recognized AgentDeck Formula/Cask installation, recognized legacy AgentDeck
installation, modified, or unknown. It must never overwrite an unknown file,
symlink, or legacy script. Removal touches only the exact link it created.

## Repository Structure Assessment

### Current structure

The repository is a Go module with one CLI entry point under `cmd/agentdeck`,
domain packages under `internal`, Homebrew Formula material under
`packaging/homebrew`, shell release tooling under `scripts`, and one tag-triggered
workflow that builds architecture-specific CLI archives.

Those boundaries are suitable for the Go core and should not be moved merely to
make room for Swift. Moving `cmd`, `internal`, or existing scripts would create a
large path-only diff, invalidate references and review history, and add no
desktop capability.

### Recommended additions

```text
apps/
  macos/
    AgentDeck.xcodeproj/
    AgentDeckApp/
    AgentDeckWidget/
    AgentDeckShared/
    AgentDeckTests/
    AgentDeckUITests/
    Config/

packaging/
  homebrew/
    agentdeck.rb.tmpl
  cask/
    agentdeck-app.rb.tmpl
  macos/
    AgentDeck.entitlements
    AgentDeckWidget.entitlements
    ExportOptions.plist

scripts/
  build-macos-app.sh
  package-macos-app.sh
  notarize-macos-app.sh
  render-homebrew-cask.sh
  test-desktop-distribution.sh

cmd/agentdeck/                  existing Go CLI, unchanged location
internal/                       existing Go domain core, unchanged location
```

The committed Xcode project is the canonical Apple project. Shared Swift models,
helper invocation, App Group storage, and release-version handling live under
`AgentDeckShared`; the Widget target may import only that privacy-bounded layer.
No third-party project generator is required for the first release.

### Build-system changes

The root `Makefile` remains the cross-language entry point and gains explicit,
composable targets for Swift tests, desktop build, universal helper assembly,
App packaging, signing verification, and desktop distribution verification.
Existing Go build, Formula, install, privacy, and release gates remain intact.

Unsigned local builds must work without release secrets. Signing and notarization
are release-only operations with explicit inputs and fail closed when required
identity or credentials are missing.

### CI and release workflow changes

CI gains an independent macOS desktop job that selects a pinned stable Xcode,
builds all Swift targets, runs Swift unit tests, validates the Go/Swift wire
fixtures, and builds the App without publishing it.

The release workflow keeps the existing CLI release job and adds a desktop job
that:

1. builds both Go helper architectures and verifies their identity;
2. creates and verifies the universal helper;
3. archives the Swift App and Widget for macOS 26;
4. signs, notarizes, staples, and assesses the App and DMG;
5. verifies embedded-helper, App, Widget, tag, and commit identity;
6. uploads desktop assets beside existing CLI assets;
7. renders and install-tests the `agentdeck-app` Cask;
8. opens a Cask-specific tap pull request.

Required release secrets, certificate handling, App Group identifiers, bundle
identifiers, signing team, and notarization credentials must be documented and
tested without exposing their values. Pull requests and ordinary CI never gain
release-secret access.

## Tasks

### 1. `desktop-wire-contract`

- Finalize the `v0.4.0` session DTO dependency and define the versioned desktop
  snapshot request/response contract.
- Keep Go aggregation, authorization, privacy filtering, partial results, and
  warnings authoritative.
- Add Go fixtures and Swift decoding fixtures from the same canonical examples.
- Document allowed desktop update-check connectivity and privacy behavior in the
  living specification when implementation makes it real.
- Verification level: L2 because this adds a stable JSON/exit-code contract.

### 2. `macos-app-foundation`

- Add the Xcode project, macOS 26 targets, bundle identifiers, entitlements,
  shared Swift layer, helper runner, App Group snapshot store, OSLog policy, and
  unsigned local build path.
- Prove the host executes only its embedded helper and handles timeout,
  cancellation, unsupported wire version, partial data, and helper failure.
- Add Swift unit tests without reading real AgentDeck or client state.
- Verification level: L3 for new build and application boundaries.

### 3. `menubar-experience`

- Implement provider, usage, cost, recent-session, warning, and health summaries.
- Add safe provider quick actions, refresh behavior, login-item preference, and
  newer-version notification that opens the official download page only.
- Define loading, stale, offline, partial, empty, and error states.
- Verify VoiceOver, keyboard navigation, reduced motion, high contrast, locale,
  narrow layout, and appearance changes on macOS 26.
- Verification level: L3 including rendered and interactive acceptance.

### 4. `desktop-widget`

- Add WidgetKit timelines and App Intent configuration backed only by the
  redacted App Group snapshot.
- Define stale age, privacy redaction, placeholder, snapshot, timeline, and
  unavailable-host states.
- Prove the Widget cannot read AgentDeck databases, credentials, client config,
  or raw session sources.
- Verification level: L3 including extension sandbox and privacy checks.

### 5. `unified-desktop-distribution`

- Build the universal helper and full App bundle, sign nested code, notarize and
  staple the DMG, publish direct-download assets, render the `agentdeck-app`
  Cask, and add Formula-to-Cask migration and mutual-exclusion behavior.
- Preserve CLI-only Formula archives and tests.
- Verify fresh Cask install, upgrade, uninstall, direct DMG installation,
  optional user CLI link, completion loading, state preservation, Gatekeeper,
  arm64, and Intel behavior.
- Verification level: L4 through an expanded aggregate release gate.

### 6. `desktop-app-contract`

- Reconcile **this plan's** delivered behavior into the living specs and manual:
  the wire contract, menu-bar app, widget, packaging, and distribution behavior
  actually delivered.
- Close all review records for this plan and confirm the app, CLI, wire-contract,
  and package identities this plan produces agree with each other.
- **This task does not raise the specification version, run the release
  candidate, or write release notes.** Those are release-level actions owned by
  the [v0.5.0 release plan](v0-5-0-release.md), which also carries the
  `v0.5.0-rc.1` requirement this plan triggers by adding external-client access
  and a new signed distribution surface.
- Runs only after every other task in this plan has Review PASS.
- Verification level: L2 for contract state.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `desktop-wire-contract` | [ ] | [ ] |
| 2. `macos-app-foundation` | [ ] | [ ] |
| 3. `menubar-experience` | [ ] | [ ] |
| 4. `desktop-widget` | [ ] | [ ] |
| 5. `unified-desktop-distribution` | [ ] | [ ] |
| 6. `desktop-app-contract` | [ ] | [ ] |

Task 1 was blocked on the `v0.4.0` session DTO contract; that dependency is now
satisfied and `desktop-wire-contract` is unblocked. Task 2 consumes task 1.
Tasks 3 and 4 depend on task 2 and may proceed independently after the shared
snapshot contract is fixed. Task 5 integrates tasks 2-4. Task 6 runs last within
this plan, and in turn gates the [v0.5.0 release plan](v0-5-0-release.md).

Commit boundaries follow task boundaries. The plan does not authorize commits,
pushes, certificate creation, secret changes, release publication, Homebrew tap
changes, local installation, or external distribution.

## Backlog / Future Feature Ideas

- App Store distribution and App Sandbox redesign, if ever desired.
- Automatic update download and installation; `v0.5.0` only opens the download
  page.
- Prerelease update channel selection.
- Rich desktop session windows beyond the menu-bar and Widget scope.
- `v0.6.0` Skills/Hooks GUI lifecycle management.
- `v0.7.0` Plugins/MCP GUI lifecycle management.

## Starting Task

Turn a Status row into scoped development by naming its anchor:

```text
进入开发：`desktop-app` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's Architecture and named task, the current release
and versioning contract in `docs/specs/cli-design.md`, every file the task names,
and verification routing. Tick `Dev` only after the task's selected verification
passes. An independent reviewer records a PASS round under
`docs/reviews/desktop-app/<task-anchor>.md` before ticking `Review`.
