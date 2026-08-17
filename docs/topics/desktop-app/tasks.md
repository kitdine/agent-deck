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
  envelope, and switch operation ownership. Both were repaired in Round 8 and
  await independent Re-review; see the Documents matrix below.
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
| ux/widget.md | [x] | [ ] |
| tasks.md | [x] | [ ] |

`requirements.md` Review Round 1 (2026-08-17): **FAIL**. The boundary still
limits the named menu-bar outcome to current-day usage while the drafted
surfaces require bounded historical analytics, and it gives the Widget no
functional user-visible acceptance outcome. Both findings are recorded in
[`reviews/requirements.md`](reviews/requirements.md). Its `Review` cell remains
unchecked. Re-review Round 2 closed both original findings but found R2-F1: the
new prohibition on every other `breakdown` also forbids the non-temporal
breakdowns required by the authorized composition and trust questions. Re-review
Round 3 narrowed R2-F1 to one omission: the repaired authorization lists every
required non-temporal dimension except provider. Round 4 (2026-08-17) closed it —
`runtime provider` is authorized in both Goals and the Acceptance boundary, and
attribution quality is stated as reportable per client and per runtime provider,
matching what `ux/menubar.md`, `ux/widget.md`, and `architecture.md` all
already require. Repair complete, `REOPEN` pending independent Re-review; later
document review stays blocked until it passes.

`ux/menubar.md` and `ux/widget.md` are both drafted as of 2026-08-16.
`ux/menubar.md` now carries rendered specimens for healthy, loading, retained
offline, partial with incomplete pricing, the switch confirmation and its
in-flight and failed states, the 280 pt narrow bound, and empty — the readiness
condition it previously failed while stating geometry only as numbers.
`ux/widget.md` is new: it specifies both widget families, the App Intent
configuration, the surface/qualifier table over cache presence, version support
and age, copy in both languages, timeline construction, and the negative
privacy assertions.

Both remain unreviewed, so neither surface may enter development yet. The
document set is audited by `make check-topic-docs`, which compares this matrix
against the files on disk and against the surfaces `requirements.md` names.

The foundation runtime contract in `architecture.md` was reviewed and approved
under the previous per-task-design convention; that history is in
[`reviews/macos-app-foundation.md`](reviews/macos-app-foundation.md). The
menu-bar contract failed independent Design Review Round 3 on six blocking
findings recorded in
[`reviews/menubar-experience.md`](reviews/menubar-experience.md); those findings
span both `ux/menubar.md` and the menu-bar section of `architecture.md`, so the
`Review` cell for each stays unticked until the repair passes.

## Tasks

This matrix predates the staged progression, so it was written before
`architecture.md` and `ux/menubar.md` existed in reviewable form — the early
decomposition the progression now forbids. It is not being rebuilt from scratch:
tasks 1 and 2 are delivered and independently reviewed, and discarding a
decomposition that already produced verified work would cost more than it
corrects.

Instead, decomposition happens properly once the Documents matrix is green.
Tasks 1 and 2 then enter stage 5 as fixed inputs — their anchors, boundaries,
and evidence stay as they are — and tasks 3 through 6 are re-derived from the
reviewed specification rather than assumed from this list. A task whose scope
the specification does not support is dropped or re-cut then, which is the point
of decomposing after the design exists.

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

Menu-bar design Review Round 3 (2026-08-16): **FAIL** on six bounded contract
findings. Round 4 repaired all six and recorded the post-migration blob mapping.
Independent Re-review Round 5 (2026-08-16): **FAIL**. R3-F1, R3-F4, R3-F5, and
R3-F6 are closed; R3-F2's transport matrix and R3-F3's retry transition remain
open, and R5-F1 newly identifies conflicting ownership of the dynamic
`switch_in_flight` reason.

Round 6 (2026-08-16): repair complete, `REOPEN` pending independent Re-review.
The transport matrix is now total by construction with an explicit catch-all,
the controller carries a complete transition table making retry and dismiss
bounded exceptions to the non-idle refusal, and `switch_in_flight` is removed
from the wire and respecified as a host-only presentation overlay. Consequential
UX repairs followed: `Cancel` on a finished failure became `Dismiss`,
`indeterminate` was aligned to the same two actions as `failed`, and three
manual checklist items were added. R5-N1 was recorded but not authorized, and is
untouched.

Independent Re-review Round 7 (2026-08-16): **FAIL**. R3-F2 and R5-F1's
wire-ownership defect are closed, but R3-F3 remains open because terminal states
do not retain the complete credential/wrapper target required by same-target
retry. R7-F1 newly records that the architecture applies `Switch in progress`
to every non-idle terminal state while the UX limits it to `inFlight`.

Round 8 (2026-08-17): repair complete, `REOPEN` pending independent Re-review.
Both findings are closed — every non-idle controller state now carries the
complete resolved option so retry reads its target from the state, and the
overlay applies in `inFlight` alone. The same round absorbed a design review of
the rendered prototype: the popover lost invented window chrome, Settings, Quit
and provider switching moved into the footer menu, and both surfaces were
rederived around the four questions the usage data answers — magnitude,
composition, trust, rhythm — with widget size selecting depth rather than
subject. `architecture.md`'s App Group projection was extended to carry the
fields those surfaces asked for.

The target was documents, not task 3, which has no implementation. Task 3 stays
blocked until a later Re-review passes, because it cannot be developed against
a specification whose latest verdict is not `PASS`.

Next action:

```text
修复：`desktop-app` / `requirements.md`（R2-F1：provider 维度）
```

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
开发：`desktop-app` / `<task-anchor>`
```

Read `AGENTS.md`, this topic's [requirements](requirements.md) and
[architecture](architecture.md), the named task, the current release and
versioning contract in `docs/specs/cli-design.md`, every file the task names,
and verification routing. Tick `Dev` only after the task's selected verification
passes. An independent reviewer records a PASS round under
`reviews/<task-anchor>.md` before ticking `Review`.
