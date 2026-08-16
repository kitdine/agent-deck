---
status: active
created: 2026-08-06
updated: 2026-08-16
---

# v0.5.0 Contract Closure — Tasks

Target version: `v0.5.0`.

This contract topic owns only the final reconciliation after the
[native desktop app topic](../desktop-app/tasks.md) is fully reviewed. It owns
no product behavior and does not decide whether the completed version proceeds
to an RC, a stable release, or remains unpublished. It originates no
requirement, surface, or architecture of its own, so it carries only this file.
The corrected ownership model is recorded by the historical
[v0.4.0 contract closure](../../archive/plans/v0-4-0-contract.md).

This file is the only status authority for this topic.

## Scope

`v0.5.0` carries one feature line: the desktop topic's six tasks. That topic
owns the behavior it delivers and writes its own feature-contract text. This
topic reconciles the complete version, raises the living specification exactly
once, and checks that its documentation and version identities agree.

The later technical preflight and any RC or stable publication are separate
commit-bound workflows. They are not tasks here and require their own explicit
authorization.

## Entry condition

Every task in the desktop topic must have Review PASS, including its own feature
contract task. This topic does not start early and does not absorb unfinished
desktop work.

## Later preflight considerations

These are not tasks in this topic, but the later exact-SHA technical preflight
must preserve them when the desktop release gate is implemented:

- signed application bundle, WidgetKit extension, Homebrew Cask, direct DMG,
  signing, and notarization identities;
- external-client access to AgentDeck state through the embedded helper and its
  privacy boundary;
- notification-only network behavior, uninstall/state preservation, and
  Gatekeeper behavior;
- fresh install, upgrade, uninstall, DMG launch, and embedded CLI identity on an
  isolated copy of real AgentDeck state.

Passing those checks still does not select RC or stable publication.

## Task breakdown

### `v0-5-0-contract`

- Reconcile the complete `v0.5.0` behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`, on top of the feature-contract text already landed
  by the desktop topic.
- Raise the specification version exactly once and record one version-level
  changelog entry.
- Confirm every desktop task has independent Review PASS and that release
  identities — app version, CLI version, wire-contract version, and Cask
  version — agree.
- Synchronize the documentation index and archive lifecycle state.
- Verification level: L2 contract state.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [ ] |

A `vX-Y-Z-contract` topic needs only this file; it reconciles what other topics
already delivered.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| `v0-5-0-contract` | [ ] | [ ] |

The task is blocked until the desktop topic is fully reviewed. Commit boundaries
follow task boundaries. This topic does not authorize commits, pushes,
certificate creation, secret changes, preflight dispatch, release publication,
Homebrew tap changes, local installation, or external distribution.

## Terminal boundary

This topic ends at `v0-5-0-contract` Review PASS. After an authorized commit and
push, the manual `release-preflight` workflow may establish exact-SHA technical
evidence. The user then decides RC, stable release, or no publication.

## Starting a task

Start the task only after its entry condition is met:

```text
进入开发：`v0-5-0-contract` / `v0-5-0-contract`
```

Read `AGENTS.md`, this task, the desktop topic's Tasks matrix, the specification
versioning contract in `docs/specs/cli-design.md`, and verification routing.
Tick `Dev` only after selected verification passes. An independent reviewer
records a PASS round under `reviews/v0-5-0-contract.md` before ticking `Review`.
