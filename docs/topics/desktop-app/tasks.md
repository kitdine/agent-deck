---
status: active
created: 2026-08-06
updated: 2026-08-16
---

# Native macOS Desktop App — Tasks

This file is the only status authority for this topic.

## Task breakdown

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

- Contract: [`architecture.md`](architecture.md#foundation-runtime) — approved.
- Add the Xcode project, macOS 26 targets, bundle identifiers, entitlements,
  shared Swift layer, helper runner, App Group snapshot store, OSLog policy, and
  unsigned local build path.
- Prove the host executes only its embedded helper and handles timeout,
  cancellation, unsupported wire version, partial data, and helper failure.
- Add Swift unit tests without reading real AgentDeck or client state.
- Verification level: L3 for new build and application boundaries.

### 3. `menubar-experience`

- Contracts: [`ux/menubar.md`](ux/menubar.md) for presentation, and
  [`architecture.md`](architecture.md#menu-bar-wire-contract-extension) for the
  additive `provider.candidates` section, the switch command surface, its result
  envelope, and switch operation ownership. Both are under design repair; see
  the Documents matrix below.
- Implement provider, usage, cost, recent-session, warning, and health summaries.
- Add safe provider quick actions, refresh behavior, login-item preference, and
  newer-version notification that opens the official download page only.
- Define loading, stale, offline, partial, empty, and error states.
- Verify VoiceOver, keyboard navigation, reduced motion, high contrast, locale,
  narrow layout, and appearance changes on macOS 26.
- The `provider.candidates` extension is additive to task 1's delivered
  contract; it does not raise `wire_version` or reopen that task's review.
- Ship English and Simplified Chinese user-visible strings.
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

- Reconcile **this topic's** delivered behavior into the living specs and manual:
  the wire contract, menu-bar app, widget, packaging, and distribution behavior
  actually delivered.
- Close all review records for this topic and confirm the app, CLI,
  wire-contract, and package identities it produces agree with each other.
- **This task does not raise the specification version, run technical preflight,
  choose a release channel, or write release notes.** The version-wide raise is
  owned by the [v0.5.0 contract closure](../v0-5-0-contract/tasks.md). Preflight
  and any RC or stable publication remain separate, explicitly authorized
  workflows.
- Runs only after every other task in this topic has Review PASS.
- Verification level: L2 for contract state.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| requirements.md | [x] | [ ] |
| architecture.md | [x] | [ ] |
| ux/menubar.md | [x] | [ ] |
| tasks.md | [x] | [ ] |

The foundation runtime contract in `architecture.md` was reviewed and approved
under the previous per-task-design convention; that history is in
[`reviews/macos-app-foundation.md`](reviews/macos-app-foundation.md). The
menu-bar contract failed independent Design Review Round 3 on six blocking
findings recorded in
[`reviews/menubar-experience.md`](reviews/menubar-experience.md); those findings
span both `ux/menubar.md` and the menu-bar section of `architecture.md`, so the
`Review` cell for each stays unticked until the repair passes.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `desktop-wire-contract` | [x] | [x] |
| 2. `macos-app-foundation` | [x] | [x] |
| 3. `menubar-experience` | [ ] | [ ] |
| 4. `desktop-widget` | [ ] | [ ] |
| 5. `unified-desktop-distribution` | [ ] | [ ] |
| 6. `desktop-app-contract` | [ ] | [ ] |

`desktop-wire-contract` Review Round 1 (2026-08-13): **FAIL**. The `Review` cell
remained unchecked pending the bounded filesystem-contract and
documentation-index Repair recorded in
[`reviews/desktop-wire-contract.md`](reviews/desktop-wire-contract.md).

`desktop-wire-contract` Re-review Round 2 (2026-08-13): **PASS**. Both Round 1
blockers are closed and the `Review` cell is synchronized.

`macos-app-foundation` development (2026-08-13): **COMPLETE**. The unsigned
Xcode build embeds the AgentDeck helper and shared framework; 10 isolated
XCTest cases passed.

`macos-app-foundation` Re-review Round 3 (2026-08-14): **PASS**. R2-F1 is
closed, all earlier findings remain closed, and 19 XCTest cases pass. Task 3
`menubar-experience` is the next task.

`menubar-experience` independent Design Review Round 3 (2026-08-16): **FAIL**.
The design requires six bounded contract repairs before Development: quiet
helper invocation, switch-envelope decoding, application-owned single-flight
operation lifetime, exact redacted switch options, legacy wire-v1 decoding, and
deterministic presentation-state precedence. Task 3 `Dev` and `Review` remain
unchecked. Next action: `修复：menubar-experience / R3-F1–R3-F6`.

Task 1 was blocked on the `v0.4.0` session DTO contract; that dependency is now
satisfied. Task 2 consumes task 1. Tasks 3 and 4 depend on task 2 and may
proceed independently after the shared snapshot contract is fixed. Task 5
integrates tasks 2-4. Task 6 runs last within this topic, and in turn gates the
[v0.5.0 contract closure](../v0-5-0-contract/tasks.md).

Commit boundaries follow task boundaries. This topic does not authorize commits,
pushes, certificate creation, secret changes, release publication, Homebrew tap
changes, local installation, or external distribution.

## Starting a task

Turn a status row into scoped development by naming its anchor:

```text
进入开发：`desktop-app` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the current release and
versioning contract in `docs/specs/cli-design.md`, every file the task names,
and verification routing. Tick `Dev` only after the task's selected verification
passes. An independent reviewer records a PASS round under
`reviews/<task-anchor>.md` before ticking `Review`.
