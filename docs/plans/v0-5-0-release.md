---
status: active
created: 2026-08-06
---

# v0.5.0 Release

Target release: `v0.5.0`.

This is a **release plan**, not a feature plan. It owns no product behavior.
The two kinds of contract task, and why the release contract does not belong to
a feature plan, are defined once in the
[v0.4.0 release plan](v0-4-0-release.md#two-kinds-of-contract-task).

## Scope

`v0.5.0` carries one feature line, the [native desktop app](desktop-app.md),
with six tasks. That plan writes its own feature contract text for the behavior
it delivers. This plan reconciles the result, raises the specification version
once, validates the release candidate, and prepares the release notes.

## Entry Condition

Every task in the desktop plan has Review PASS, including its own feature
contract task. `v0.4.0` must already be released, because the desktop wire
contract consumes the session DTOs that release stabilizes.

## Release Characteristics

These determine the gates below and must be re-verified rather than assumed:

- The release adds a **new signed distribution surface**: an application bundle,
  a WidgetKit extension, a Homebrew Cask, a direct-download DMG, signing, and
  notarization.
- It adds **external-client access** to AgentDeck state through an embedded
  helper, which is a new trust boundary rather than a new report.
- It adds a **notification-only update check**, which reaches the network on a
  path no prior release had.
- Uninstall, state preservation, and Gatekeeper behavior become user-visible
  contracts for the first time.

Because this release adds an external access path and a signed distribution
surface, it requires at least one `-rc.N` validated against real local
AgentDeck state before stable promotion.

## Tasks

### 1. `v0-5-0-contract`

- Reconcile the complete `v0.5.0` behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`, on top of the feature contract text the desktop
  plan already landed.
- **Raise the specification version exactly once** and record one changelog
  entry covering the whole release.
- Confirm every desktop task has an independent Review PASS and that all release
  identities — app version, CLI version, wire-contract version, Cask version —
  agree.
- Verification level: L2 for contract state.

### 2. `v0-5-0-release-candidate`

- Build and validate `v0.5.0-rc.1` against real local AgentDeck state before
  stable promotion.
- Verify fresh Cask install, upgrade, uninstall, direct DMG installation,
  optional user CLI link, completion loading, state preservation, Gatekeeper
  behavior, and both arm64 and Intel execution.
- Confirm the update check only notifies and opens the official download page,
  and reaches no other network destination.
- Record the RC evidence, including what was validated and what could not be.
- Verification level: L4 through the expanded aggregate release gate.

### 3. `v0-5-0-release`

- Write release notes recording compatibility, migration, uninstall, privacy,
  signing, and update-notification behavior.
- Publish the tag and GitHub Release, then promote the Cask and Formula.
- **Requires explicit authorization immediately before execution**, including
  any certificate or secret handling. Neither this plan nor a Review PASS grants
  it.
- Verification level: L4.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `v0-5-0-contract` | [ ] | [ ] |
| 2. `v0-5-0-release-candidate` | [ ] | [ ] |
| 3. `v0-5-0-release` | [ ] | [ ] |

Task 1 is blocked until the desktop plan is fully reviewed. Task 2 depends on
task 1. Task 3 depends on task 2 and on explicit release authorization.

Commit boundaries follow task boundaries. This plan does not authorize commits,
pushes, certificate creation, secret changes, release publication, Homebrew tap
changes, local installation, or external distribution.

## Starting Task

Turn a Status row into scoped work by naming its anchor:

```text
进入开发：`v0-5-0-release` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's named task, the desktop plan's Status matrix, the
specification versioning contract in `docs/specs/cli-design.md`, and
verification routing. Tick `Dev` only after selected verification passes. An
independent reviewer records a PASS round under
`docs/reviews/v0-5-0-release/<task-anchor>.md` before ticking `Review`.
