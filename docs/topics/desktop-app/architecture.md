---
status: active
created: 2026-08-06
updated: 2026-08-16
---

# Native macOS Desktop App — Architecture

Development design for the desktop topic: process shape, the Go/Swift wire
contract, the macOS security boundary, packaging and distribution, repository
layout, and the per-task contracts for `macos-app-foundation` and the menu-bar
switch surface.

This document is normative. Product behavior must follow these contracts even
when an earlier implementation differs from them. Goals, non-goals, and the
decisions this design implements are in [`requirements.md`](requirements.md);
menu-bar presentation, copy, and accessibility are in
[`ux/menubar.md`](ux/menubar.md).

## System shape

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

## Foundation runtime

Normative contract for task 2 `macos-app-foundation`: the native host, shared
desktop runtime, helper-execution boundary, and private snapshot cache that the
menu-bar and Widget tasks consume.

### Purpose

Define the native macOS host, shared desktop runtime, helper-execution boundary,
and private snapshot cache that later menu-bar and Widget tasks can consume.

This document is normative. Product behavior must follow this contract even
when an earlier implementation differs from it.

### Scope

The foundation delivers:

- a macOS 26 menu-bar application shell;
- a UI-independent shared Swift runtime;
- a single-flight snapshot refresh coordinator owned by the application;
- execution of the `agentdeck` helper embedded in the application bundle;
- strict decoding of the versioned desktop snapshot contract;
- a presentation-safe App Group snapshot projection for later Widget use;
- privacy-safe structured logging;
- an unsigned, offline-capable local build and isolated Swift tests.

### Non-goals

This task does not deliver:

- menu-bar summaries, controls, settings, or interaction design;
- a Widget extension, Widget timeline, or Widget presentation;
- automatic periodic refresh or launch-at-login behavior;
- update UI, update download, installation, or rollback;
- signing, notarization, universal release packaging, or publication;
- a daemon, background service, network listener, or new source of truth;
- mutation or migration of AgentDeck, Codex, or Claude state.

### Decisions

| Area | Decision |
| --- | --- |
| Platform | The v0.5.0 native application targets macOS 26. |
| Application shape | The host is a menu-bar-only SwiftUI application with no Dock presence. The foundation scene is a placeholder only. |
| Runtime ownership | One application-owned coordinator owns refresh state, helper execution, decoding, and cache publication. |
| Helper | The host executes only the `agentdeck` binary embedded in its own bundle. It never resolves a helper through `PATH` and never uses a shell. |
| Data authority | The Go helper and existing AgentDeck state remain authoritative. Swift state and the App Group file are disposable projections. |
| Shared storage | The application is the sole writer to `group.com.kitdine.agentdeck`; future extensions are read-only consumers. |
| Sandboxing | v0.5.0 is a directly distributed, non-App-Store application and does not enable App Sandbox. Hardened Runtime and release signing belong to distribution work. |
| Networking | Foundation refresh is local-only and opens no sockets. |
| Dependencies | The Swift foundation uses Apple frameworks only and must build without package downloads. |

### Runtime architecture

#### Application host

The application host MUST:

- own exactly one refresh coordinator for the application lifetime;
- inject the coordinator state into later scenes instead of letting views launch
  processes or read files directly;
- start one initial refresh after application startup;
- remain usable when App Group storage is unavailable;
- expose a placeholder menu-bar scene without implementing later menu-bar UX.

The application host MUST NOT contain wire parsing, process management, cache
serialization, or diagnostic-string construction.

#### Shared desktop runtime

The shared runtime contains four independent boundaries:

1. helper execution;
2. wire-envelope decoding and validation;
3. refresh-state coordination;
4. App Group projection and persistence.

The shared runtime MUST NOT import SwiftUI, WidgetKit, or AppKit. Its public
state must be usable by both application and extension presentation layers.

#### Embedded helper

The release application contains one same-release `agentdeck` helper under its
bundle's Helpers directory. Local unsigned builds may contain only the host
architecture; universal assembly is deferred to distribution work.

The resolved helper URL MUST:

- be derived from the application bundle, not caller input or environment;
- resolve to a regular executable file within the bundle after symlink
  resolution;
- fail closed when missing, non-executable, or outside the bundle;
- have no fallback to an installed or `PATH`-resolved binary.

### Refresh contract

#### State model

The coordinator exposes these semantic states:

| State | Meaning |
| --- | --- |
| `uninitialized` | No refresh has completed and no prior snapshot exists. |
| `refreshing(previous?)` | One helper invocation is active; an earlier valid snapshot may remain available. |
| `ready(snapshot)` | A valid complete or partial snapshot is current. |
| `degraded(previous?, issue)` | Refresh failed; a previous valid snapshot is retained when available. |

Presentation code may derive loading, fresh, partial, stale, and unavailable
labels from these states, but may not invent a second refresh state machine.

#### Concurrency

- At most one helper invocation may be active per application.
- A refresh requested while one is active coalesces with that operation unless
  the caller explicitly requests replacement.
- Replacement cancels and reaps the prior process before publishing a new
  result.
- Cancellation caused by replacement or application shutdown is not reported
  as a product failure.
- Only the latest non-cancelled generation may update coordinator state or the
  App Group cache.

#### Lifecycle

- The application performs one initial refresh after startup.
- Later menu-bar work may request manual refresh through the coordinator.
- Later Widget work may request a timeline reload after successful cache
  publication, but may not invoke the helper.
- Periodic scheduling and launch-at-login remain outside this task.

### Helper execution contract

The helper runner invokes the versioned desktop snapshot command defined by the
CLI contract with a bounded recent-session limit.

| Limit | Value |
| --- | --- |
| Default recent-session count | 5 |
| Accepted recent-session range | 1 through 20 |
| Default wall-clock timeout | 10 seconds |
| Maximum captured stdout | 256 KiB |
| Maximum captured stderr | 256 KiB |

The runner MUST:

- pass arguments as an array without shell parsing;
- give the child no semantic dependence on the current working directory;
- forward only the home, locale, temporary-directory, and documented
  AgentDeck/client-root environment needed by the CLI contract;
- reject invalid recent-session limits before launch;
- capture stdout and stderr concurrently to avoid pipe deadlock;
- enforce each output limit while the process is running;
- terminate and reap the process on timeout or cancellation;
- return typed failure categories rather than raw process text.

Raw stdout and stderr MUST NOT be logged, included in user-facing errors, or
written to the App Group cache. A non-zero exit is a failed refresh even if
stdout happens to contain decodable JSON.

### Wire validation

The decoder MUST validate the desktop snapshot as an untrusted boundary even
though the helper is bundled.

It MUST reject:

- malformed or oversized JSON;
- a missing or unsupported envelope schema version;
- a command discriminator other than the desktop snapshot command;
- a missing or unsupported data wire version;
- a malformed `generated_at` timestamp;
- missing required fields or invalid field types;
- envelope combinations prohibited by the CLI contract.

Unknown additive fields are ignored for forward compatibility. A valid partial
envelope is accepted and remains visibly partial. Malformed or unsupported
output never replaces the last valid in-memory or persisted snapshot.

### Presentation-safe App Group projection

#### Data minimization

The shared cache is a Widget-oriented projection, not a copy of the wire
envelope. It may contain only:

- cache schema version and source wire version;
- generation time, partial state, last successful refresh time, and next
  suggested refresh time;
- client identifiers and selected provider display identifiers;
- per-period usage totals for the three supported periods — `today`, `7d`,
  `30d` — **for each client scope**, each with counts, session count, pricing
  completeness, and cost strings;
- token components — input, output, cached-read, cache-write — at total,
  bucket, model, and client level, matching what `usage stats` already returns;
- a bounded daily series of at most 90 buckets of
  `(date, tokens, cost, sessions)` for the trend charts, plus each period's
  `peak` bucket and average — **one series per client scope**, for `all` and for
  each client present, so at most 3 series and 270 buckets;
- a bounded 7×24 hour-of-week activity grid of relative intensity, the same
  aggregate the terminal report already renders, plus the active-day count and
  the busiest and quietest day names, each scoped to the last 30 days — the
  same window `rhythm` defaults to everywhere else — computed as a one-time
  reduction over the most recent 30 entries of that scope's daily series the
  producer already holds, not a second query; **one grid and one reduction per
  client scope**, so at most 3 grids and 504 cells;
- per-period top-N model shares: at most 12 entries of
  `(model, tokens, cost, share)` for each of the three supported periods **and
  each client scope**, deterministically ordered — the product `composition`
  needs, since it takes both `Period` and `Client`;
- per-period per-client subtotals: cost and token totals for each client, for
  each of the three supported periods;
- per-client and per-provider attribution-quality subtotals for the current
  period only — determinable, inferred, and unattributed each as
  `(cost, tokens, count, share)` — because `trust` is fixed to the current
  period and never takes `Period`;
- pricing coverage per client scope: priced and unpriced counts, and at most 12
  deterministically ordered unpriced model identifiers, for `all` and for each
  client present, because `trust` takes `Client` and its coverage bar and
  identifier list must render under a single client;
- aggregate health status and problem/warning/error counts;
- allowlisted presentation-safe issue codes.

Everything after the fourth bullet was added on 2026-08-17 because the surface
documents asked for it. The reasoning is recorded because the earlier, thinner
list was not defended anywhere: **a model identifier, a daily token total, an
hour-of-week intensity, and an attribution count are aggregates over events, not
content of them.** None names a session, a path, a prompt, a project, or a
credential, and each is already computed by `usage stats` for the terminal
report, so the projection copies a number the product already publishes rather
than deriving new knowledge inside a sandboxed extension.

The per-period totals and per-period model shares are a second, same-day
revision, made because `reviews/ux-widget.md`'s W-F1 found the first revision
had provisioned only one period's aggregate while `ux/widget.md`'s `magnitude`
medium/large and `composition`'s `Period` parameter both require the other two
periods' totals and model shares simultaneously, and while `ux/menubar.md`'s
period switcher named the same gap under R9-F2/Round 14. A single-period
projection cannot answer either without a second aggregation layer in Swift,
which `requirements.md` and `ux/menubar.md` both forbid; computing three
periods in Go once, at publication time, is the same class of one-time
reduction as the existing daily series and 7×24 grid, not a new capability.
The active-day count and busiest/quietest day names close the matching gap
W-F3 found in `rhythm`, for the same reason: a Swift-side reduction over the
90-day series would be a second aggregation layer, so the producer computes it
once instead.

The per-client subtotals split into two bullets in a third, same-day revision,
because `reviews/ux-widget.md`'s Round 3 (W-F5) found the single prior bullet
served two consumers with different period semantics: `composition`'s large
size takes `Period` and needs per-client subtotals for each of the three
periods, while `trust` never takes `Period` and needs only the current
period's attribution-quality breakdown. Making the whole bullet per-period
would have given `trust` two periods of data it never asked for and cannot
display; keeping it single would have left `composition` exactly where W-F5
found it. The split assigns each consumer only the shape it uses.

The active-day count and busiest/quietest day names changed window in a
fourth, same-day revision, from 90 days to 30, because Round 3's W-F6 found
they had been provisioned over the window the newly-added 90-day heatmap
uses, not the window `ux/widget.md:88` states as `rhythm`'s default and
`:135` states explicitly for the active-day count. The 90-day heatmap itself
needs no separate field — it renders directly from the already-provisioned
bounded daily series — so nothing here serves it; both reduced fields exist
only for the 30-day-scoped elements that named them.

The attribution-quality subtotals gained cost and tokens alongside their
existing counts in a fifth, same-day revision, because Round 5's W-F8 found
`ux/widget.md`'s `trust` small/medium display quality-tier **amounts**
(`:121-122`, and `:129`'s own stated reason for the widget's existence) while
this bullet had provisioned only **counts** — a shape mismatch this
document's own Data requirements table had also gotten wrong, since it named
counts too. No other bullet carries cost broken out by attribution quality:
per-period totals are a single aggregate, per-period model shares are keyed
by model, and per-period client subtotals are keyed by client — none has a
quality-tier dimension. Each tier now carries `(cost, tokens, count, share)`,
the same shape as the other per-dimension breakdowns in this list, still
scoped to the current period only for the same reason the split recorded
above gave it that scope.

The projection's usage families gained a **client scope** in a sixth, same-day
revision, because
Round 7's W-F9 found `Client` — the one configuration parameter every widget
carries (`ux/widget.md:73-80`) — provisioned on only three of them. Under
`Client = codex` the daily series, the 7×24 grid, the model shares, the
per-period totals, the session count and the pricing coverage all had no
per-client form, so `magnitude`, `composition` and `rhythm` had nothing to
render at any size, and `trust` lost its coverage bar. The projection had been
extended along `Period` (W-F1), along the client subtotals `composition` names
(W-F5) and along the rhythm window (W-F6) without anyone asking what the third
dominating parameter governs.

Two of those bullets take the **product** of the two parameters rather than one
scope each. `composition` takes both `Period` and `Client`, so its model shares
are provisioned for each period *and* each scope — twelve entries per cell, nine
cells; and `magnitude`'s per-period totals are provisioned the same way.
Provisioning either parameter alone would have left the cross term exactly where
W-F9 found it, which is the failure mode this topic has now hit four times: the
repair answered the named rows and not the set the parameter governs.

The two per-period totals bullets were consolidated in a seventh, same-day
revision because Round 9's W-F10 found they split one `Client` × `Period` cell
across contradictory scopes: one carried pricing completeness without a client
scope, while the other carried a client-scoped session count without pricing
completeness. Totals, session count, completeness, and cost strings now travel
together for every client and period, so `Cost incomplete` qualifies the same
number the widget displays. The former aggregate session availability/count
bullet had no independent widget consumer: its count duplicated this per-period
field, while the projection's existing `partial` state already communicates an
unavailable source. It was removed rather than retained as unowned cache data.

The next suggested refresh time joined the projection in an eighth, same-day
revision because Round 11's W-F11 found `ux/widget.md` used it as the baseline
for WidgetKit's refresh-after request while the projection's allowlist omitted
it. The producer already receives this scalar timestamp in the wire snapshot;
projecting it beside `generated_at` gives the extension the declared baseline
without adding a client or period dimension, and the widget still clamps that
value to its documented 15-to-60-minute range.

**Client scope** means `all` plus each client actually present, so at most three
scopes with the two clients this product supports, and the `all` scope is the
value every bullet already carried. A client absent from the projection is not
an error (`ux/widget.md:81-83`), so the scope count follows the data rather than
a fixed list.

The bounds are part of the contract, not an implementation detail. A projection
is read by an extension on every timeline refresh and persists across launches;
an unbounded series or model list would grow the cache without limit and make
its read cost a function of history. Per client scope: ninety daily buckets and
one 7×24 grid; twelve models per period, thirty-six model-share entries across
the three supported periods; and twelve unpriced identifiers. Across at most
three scopes that is 270 buckets, 504 grid cells and 108 model-share entries —
about three times the single-scope ceiling, on an aggregate that carries no
per-event data at any scope. The producer truncates deterministically rather
than sampling, within each scope independently, so a busy client cannot consume
another's budget.

It MUST NOT persist:

- session identifiers, titles, prompts, paths, working directories, or raw
  session content;
- provider URLs, headers, credentials, account configuration, or raw provider
  records;
- raw helper stderr or stdout;
- raw warning, error, health-problem, or health-check text;
- arbitrary paths or environment values.

Detailed diagnostics may exist transiently in application memory, subject to
the same logging policy, but are not shared with extensions.

#### Cache schema

The App Group cache schema is versioned independently from the Go wire schema.
Readers reject an unsupported cache version and treat the cache as unavailable.
They never attempt an in-place migration.

#### Persistence

The application is the sole cache writer. Publication MUST:

1. obtain the App Group container without falling back to a public or
   user-selected path;
2. create or verify a private directory with mode `0700`;
3. encode a deterministic JSON projection;
4. create the temporary file in that directory with mode `0600` before writing;
5. flush the completed file and atomically replace the published cache on the
   same filesystem;
6. verify the final file remains a regular, non-symlink file with mode `0600`.

Setting permissions only after publishing the destination is insufficient.
Write failure leaves the previous valid cache intact.

An unsigned local build may use an explicitly injected temporary directory for
tests and verification. Production code does not silently substitute such a
directory when the App Group container is unavailable.

### Security and privacy

- The helper and application run as the current user and request no privilege
  escalation.
- Foundation behavior is read-only with respect to AgentDeck and client state.
- No component opens a listening port or modifies host networking.
- The helper path, argument set, and cache destination are not influenced by
  snapshot content.
- Cache files and temporary files are protected before data is written.
- Test fixtures use synthetic identifiers, paths, providers, costs, and
  session data.

Logging uses a fixed subsystem and fixed event codes. Public log values are
limited to allowlisted classifications such as refresh completion, partial
completion, timeout, cancellation, invalid output, and unavailable storage.
All other dynamic values are omitted; raw JSON, paths, session data, provider
configuration, and error messages are never logged.

### Build and configuration

The checked-in Xcode project defines:

- the menu-bar application target;
- the UI-independent shared Swift target;
- isolated unit-test targets;
- an optional verification executable for bundle-boundary checks.

The project uses stable bundle and App Group identifiers owned by AgentDeck.
Application and future extension targets that consume the cache share the App
Group entitlement. Test targets use injected temporary directories and do not
require the entitlement.

The local build command MUST:

- use an isolated DerivedData directory;
- build without signing or access to the login keychain;
- build the Go helper from vendored dependencies with isolated Go caches;
- embed the helper in the exact bundle location consumed by the runner;
- perform no network access and no mutation of real AgentDeck state.

Signing, Hardened Runtime validation, notarization, universal assembly, release
asset layout, and update-channel metadata belong to
`unified-desktop-distribution`.

### Acceptance criteria

#### Application integration

- The real application root owns and injects one refresh coordinator.
- Launch initiates one refresh without blocking the main actor.
- The placeholder scene does not implement menu-bar summaries or Widget UI.
- A successful refresh publishes both in-memory state and the safe cache
  projection.
- Failed refresh retains the last valid state and cache.

#### Helper runner

- Tests cover the bundled path, missing helper, invalid recent limit, launch
  failure, timeout, cancellation, non-zero exit, output overflow, malformed
  output, and successful complete and partial output.
- Tests prove no shell or `PATH` fallback is used.
- Cancellation and timeout tests prove the child is reaped.

#### Wire and state

- Tests cover unsupported envelope and data versions, wrong command,
  malformed timestamp, missing fields, additive unknown fields, complete
  output, and partial output.
- Coordinator tests cover initial refresh, single-flight coalescing,
  replacement cancellation, latest-generation wins, and stale-data retention.

#### Cache and privacy

- Tests prove directory and file permissions are secure before publication,
  replacement is atomic, and a failed write preserves the previous cache.
- Tests prove the cache excludes every prohibited sensitive field and raw
  diagnostic string.
- Tests cover unavailable App Group storage, unsupported cache versions,
  malformed cache data, and concurrent readers during replacement.
- Log tests prove only fixed allowlisted classifications are emitted.

#### Build boundary

- The unsigned macOS 26 application and Swift tests build from an isolated
  environment without network access.
- Bundle inspection proves the helper is present, executable, inside the bundle,
  and selected without `PATH` lookup.
- Automated tests use an isolated synthetic home and never read real AgentDeck,
  Codex, Claude, or App Group state.

### Downstream contracts

`menubar-experience` consumes only coordinator state and refresh operations. It
does not launch the helper, decode wire data, or read the cache directly.

`desktop-widget` reads only the presentation-safe App Group projection. It does
not launch the helper or receive raw diagnostic text.

`unified-desktop-distribution` is responsible for matching helper and app
architectures, release signing, Hardened Runtime, notarization, and delivery of
the App Group entitlement in release artifacts.

### Approval boundary

Design approval authorizes this contract to become the implementation and
review authority for `macos-app-foundation`. It does not authorize repair,
review, commit, push, signing, notarization, release, installation, or work on
later desktop tasks.

## Menu-bar wire contract extension

Normative contract for the Go-side surface task 3 `menubar-experience` consumes.
It extends the wire contract task 1 delivered, additively; it does not raise
`wire_version` or reopen that task's review. The presentation that consumes it
is specified in [`ux/menubar.md`](ux/menubar.md).

### Additive `usage.presentation`

`data.usage` gains one additive `presentation` object. The existing summary
fields remain unchanged. This object is the source for the menu bar's four usage
sections and the presentation-safe usage values in the App Group projection; the
menu bar never reads that projection back.

Go computes this aggregate once while producing the wire snapshot. The host may
select the active `Client` and `Period`, format already-computed values, and copy
the allowlisted values into the App Group projection. It MUST NOT sum daily
buckets, regroup model or quality rows, calculate shares, or otherwise create a
second aggregation layer in Swift.

Every collection below is a non-null array. Collection families use the explicit
`{ available, items }` shape; `pricing` and `rhythm` carry `available` beside
their named values. One unavailable family therefore does not erase the other
sections. `available: true` with empty `items` means a valid empty result, while
`available: false` means the producer could not supply that family. A family
failure leaves the other families intact and is covered by the snapshot's
existing warning and partial-result semantics.

| Field | Bound and semantic shape |
| --- | --- |
| `presentation.available` | Whether any presentation aggregate was produced. When `false`, every family is unavailable and every collection is empty. |
| `scopes[]` | Exactly three records when `presentation.available` is true, in `all`, `codex`, `claude` order. `all` is an explicit scope, not a missing client; a client with no data keeps its record with unavailable families. |
| `scopes[].periods.items[]` | Exactly `today`, `7d`, and `30d`, in that order, when `periods.available` is true. Each record carries `totals`, `average_per_day`, `peak`, `cache_hit_share`, and `models`. This is the `Client` × `Period` product, not two independent partial views. |
| `periods.items[].totals` | Total tokens plus input, output, cached-read, and cache-write components; event and session counts; the existing usage cost tuple (`catalog_base_cost`, `provider_cost`, `known_catalog_base_cost`, `known_provider_cost`); `pricing_complete`; and `unpriced_components`. |
| `periods.items[].average_per_day` and `periods.items[].peak` | Producer-computed values for that exact scope and period. `peak` is one dated bucket; neither value is recomputed from `daily.items` by the host. |
| `periods.items[].cache_hit_share` | Producer-computed cached-read over logical input for that exact scope and period. |
| `periods.items[].models[]` | At most 12 deterministically ordered `(model, token components, cost tuple, share)` rows for that exact scope and period. |
| `scopes[].daily.items[]` | At most 90 ascending dated buckets per client scope when `daily.available` is true. Each bucket carries token components, event and session counts, and the cost tuple needed by the bounded trend chart. |
| `scopes[].quality.items[]` | Current-period only when `quality.available` is true: one client-scope aggregate plus deterministic per-provider records. Each record carries determinable, inferred, and unattributed tiers as `(cost tuple, tokens, count, share)`. |
| `scopes[].pricing` | Current-period `available`, priced and unpriced counts, plus at most 12 deterministically ordered unpriced model identifiers for that client scope. |
| `scopes[].rhythm` | `available`, the producer-computed 7×24 grid for the last 30 days (exactly 168 cells when available), active-day count, and busiest and quietest day names used by the downstream projection. |
| `client_subtotals.items[]` | At most six deterministic `(period, client, totals)` records when `client_subtotals.available` is true: one for each supported period and concrete client. These drive the client tabs without a host-side regrouping. |

The bounds, privacy exclusions, cost semantics, and deterministic truncation are
the same as the corresponding App Group projection fields above. The projection
is still not a copy of the wire envelope: the host writes only this allowlisted
usage subset plus the separately allowlisted cache metadata, provider display
state, health summary, and issue codes.

Compatibility holds in both directions without raising `wire_version`. The
producer always emits `presentation`; an older decoder ignores it. A new decoder
treats a missing object in a legacy v1 payload as `available: false` with empty
families, while a present object with a wrong field type is invalid. Fixtures
therefore retain one legacy payload without `presentation`, and current complete
and partial fixtures carry it with every collection bound asserted.

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
and `already_selected`.

**`switch_in_flight` is deliberately not among them.** Every wire `reason_code`
states something the snapshot producer can know: a credential row is absent, a
wrapper URL is unconfigured, a provider is already selected. Whether a switch is
running is transient state owned by the host's `SwitchController`, created after
the snapshot was generated and gone before the next one. Encoding it in the wire
would put a value in the payload that is stale the moment it is written, and the
App Group cache would persist that staleness across launches.

It is therefore a **host-only presentation overlay**, and it applies in
`inFlight` **only** — not in every non-idle state. `failed` and `indeterminate`
are also not `idle`, and an earlier draft keyed the overlay on "not idle", which
would have shown `Switch in progress` beside a switch that had already finished
and failed. A terminal state is not progress. Those states present their own
result and their retry and dismiss actions instead, per
[`ux/menubar.md`](ux/menubar.md).

While `inFlight`, the host disables every option row and states why, without
altering the Go-resolved tuple: an option's `ready`, `reason_code`, and arguments
are unchanged, and the same option becomes selectable again when the controller
leaves `inFlight`. Precedence is fixed — the in-flight overlay is shown instead
of an option's own `reason_code`, because a global block explains more than a
per-option one while it holds. Its copy is `Switch in progress` /
`正在切换`, specified with the rest of the switch flow in
[`ux/menubar.md`](ux/menubar.md).

The rule this preserves: Go decides what a switch *is*, the host decides whether
one can be *started right now*. Those are different questions with different
owners, and only the first belongs in the wire.

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

Outcome is decided by the envelope, never by the exit status alone.

Exactly two input shapes are conclusive. Each requires its envelope on its
canonical stream, agreeing with the exit status, with the other stream empty:

| Streams and status | Host behavior |
| --- | --- |
| stdout envelope, no `error`, empty stderr, exit `0` | **Success.** Trigger one replacement refresh |
| stderr envelope with `error`, non-zero exit, empty stdout | **Typed failure.** Map `error.code` to a localized message |

Everything else is inconclusive, and the classification is total by
construction: a decodable envelope that does not match a conclusive row above is
**indeterminate**; anything that does not decode is **opaque**.

| Inconclusive input | Class |
| --- | --- |
| Envelopes on both streams | Indeterminate |
| `error` present with exit `0`, or absent with non-zero exit | Indeterminate |
| A valid envelope on the wrong stream — an `error` envelope on stdout, or a success envelope on stderr — whatever the exit status | Indeterminate |
| Any other decodable envelope not matching a conclusive row | Indeterminate |
| Unknown `schema_version` or `command`, malformed, truncated, or no valid JSON on either stream | Opaque failure. Never parse the text |

The catch-all row exists so that no input can fall outside the table. A valid
envelope on the wrong stream is a transport violation, not a result: the helper
plainly reached the point of emitting an envelope, so the configuration may have
been written, and treating a misplaced `error` envelope as a typed failure would
report "nothing happened" about a mutation that may have happened. It is
therefore never reported as success or failure, and its `error.code`, if any, is
not shown as an outcome.

An indeterminate outcome is not a failure and not a success: the configuration
may or may not have been applied. The host MUST force a replacement refresh and
reconcile from the resulting snapshot rather than asserting either result. An
opaque failure is distinct because nothing decodable was produced; it too forces
a replacement refresh, and it reports no code.

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

Every non-idle state carries the **complete resolved option**, written here as
`opt` = `(client, provider, credential?, via_wrapper)` — the same tuple the
canonical invocation takes:

```text
idle
inFlight(opt)
succeeded(opt)
failed(opt, code)
indeterminate(opt)
```

`failed` and `indeterminate` carry `opt` and not merely a code, because retry
re-runs the same target and there is nowhere else to read it from. A terminal
state holding only `failed(code)` would force the controller to recover the
target from presentation state, which is exactly the direction of dependency
this design forbids: the view model observes the controller, never the reverse.
The window may be closed when the user retries, so the presentation may not
exist at that moment.

| Concern | Rule |
| --- | --- |
| Serialization | One switch app-wide. A *new* switch requested while the controller is not `idle` is refused, not queued. Retry and dismiss are not new switches; see the transition table below |
| Double submit | Structurally impossible: confirmation's controls are disabled on entry to `inFlight`, so no second submit can be issued |
| Cancellation | The controller never cancels an in-flight switch, and MUST NOT invoke the runner through a cancellable task whose cancellation terminates the helper. `EmbeddedHelperRunner`'s `withTaskCancellationHandler` calls `running.terminate()` on cancel, which would kill the helper mid-write; the switch path must not be cancelled by window dismissal, view teardown, or task cancellation |
| Window dismissal | Detaches presentation only. On reopen the controller's current state is displayed, including a result that became terminal while closed |
| Timeout after launch | Becomes `indeterminate`, never `failed`. The helper may already have written the client configuration, so the controller forces a replacement refresh and reconciles health and recovery state from the snapshot |
| Success lifetime | Cleared by the next completed replacement refresh, or after 10 seconds, whichever comes first |
| Failure and indeterminate lifetime | Retained until the user retries or dismisses. These are never cleared by a timer, because a failure the user did not see is a failure that gets repeated |

The two lifetimes differ deliberately: a success is confirmed by the refreshed
data that replaces it, while a failure has no such successor and must wait to be
read.

##### Allowed transitions

Retaining a terminal failure until the user acts, while refusing any request
made when not `idle`, would deadlock the controller: `failed` and
`indeterminate` are not `idle`, so the retry the UX offers would be refused by
the rule meant to stop concurrent switches. The serialization rule governs *new*
switches. Recovery from a terminal state is a bounded exception, and this table
is the complete set of transitions:

| From | Event | To | Note |
| --- | --- | --- | --- |
| `idle` | Confirmed switch for `opt` | `inFlight(opt)` | The only entry point for a new switch |
| `inFlight(opt)` | Conclusive success | `succeeded(opt)` | |
| `inFlight(opt)` | Typed failure | `failed(opt, code)` | `opt` is carried forward, not discarded |
| `inFlight(opt)` | Indeterminate, opaque, or timeout after launch | `indeterminate(opt)` | Forces a replacement refresh |
| `inFlight(opt)` | Any switch request | `inFlight(opt)` | Refused; this is what serialization protects |
| `succeeded(opt)` | Replacement refresh completes, or 10 s elapse | `idle` | Whichever comes first |
| `failed(opt, _)` / `indeterminate(opt)` | **Retry** | `inFlight(opt)` | Atomic, and `opt` comes from the state itself. No intermediate `idle` is observable, so no other switch can interleave |
| `failed(opt, _)` / `indeterminate(opt)` | **Dismiss** | `idle` | Clears the result without retrying |
| `failed(opt, _)` / `indeterminate(opt)` | A switch request for `opt' ≠ opt` | refused | Recovery is bounded to retry and dismiss; starting a different switch requires dismissing first, so the user cannot silently abandon an unread failure |

Retry takes its target from `opt` in the state and never from what the user is
looking at. The menu may be closed, the snapshot may have been replaced, and the
option list may have been re-derived between the failure and the retry; none of
that changes which switch is being retried.

Retry is atomic rather than a reset followed by a start, because an observable
`idle` between them would be a window in which serialization is not held, which
is exactly the property the rule exists to guarantee.
