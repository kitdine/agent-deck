---
status: active
created: 2026-08-06
updated: 2026-08-26
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

## Assembly list

This list decides what `v0.5.0` contains. A topic carries no version number of
its own, so membership exists here and in
[the active-topic status](../../status.md#active-development) and nowhere else. Changing the
list is how a topic is added or deferred; no commit, branch, review record, or
evidence moves when it changes.

| Topic | Included | Reason |
| --- | --- | --- |
| [`desktop-app`](../desktop-app/tasks.md) | **Yes** | All six tasks ship together; a topic is merged whole or not at all. |
| [`work-signals`](../work-signals/tasks.md) | **Yes** | Restores the captured activity, workflow, and tooling data behind the desktop Sessions surface and adds the matching CLI surface. Its six implementation tasks and contract task ship as one topic. |
| [`cli-error-classification`](../cli-error-classification/requirements.md) | **Yes** | It changes the documented JSON error contract, turning `runtime_error` into specific not-found codes. That is an observable break for any consumer matching the old code, so it ships in the same tag as the desktop line rather than trailing it: the desktop surface reads those codes, and a version whose UI classifies errors one way while its CLI classifies them another is the worse outcome. The break is announced once, in this version's notes. |
| [`switch-effectiveness-boundary`](../switch-effectiveness-boundary/requirements.md) | **Yes** | One client-neutral Hook operation must persist every accepted Codex or Claude delivery before applying an event-specific route effect. Real-session evidence still fixes Claude's state machine: only `no key -> first key` may apply live; key rotation and removal retain the prior route until restart. Its four tasks cover the shared ledger, advisory/file contract, effective-route policy, and cross-client real-lifecycle acceptance. |
| [`usage-attribution-precision`](../usage-attribution-precision/requirements.md) | **Yes** | The cancelled `v0.6.0` attribution line moves into `v0.5.0` as this active topic, not as a future Backlog reconciliation. It corrects the contract that reserves `exact` for `agentdeck run` while hardcoding determinable Hook routes as `estimated`. Pricing, credit, Context Efficiency, and subscription candidates remain independent. A determinable event downgraded to `inferred` is a release blocker and cannot ship. |

Excluding a topic costs nothing here and everything later: a topic already
merged but no longer wanted has no clean removal, because `revert` propagates
forward and `reset` rewrites history. See `.agent-instructions/branching.md`.

## Scope

`v0.5.0` therefore carries five feature lines: desktop app, work signals, CLI
error classification, switch effectiveness, and usage-attribution precision.
Each topic owns the behavior it delivers and writes its own feature-contract
text. This topic merges the selected branches, reconciles the complete version,
raises the living specification exactly once, and checks that its documentation
and version identities agree.

The error-classification line is a breaking change to a documented JSON
contract. This version's notes must announce it under a compatibility heading,
naming the old `runtime_error` code and each code replacing it, so a consumer
matching the old value learns of the break from the release rather than from a
failure.

The later technical preflight and any RC or stable publication are separate
commit-bound workflows. They are not tasks here and require their own explicit
authorization.

## Entry condition

Every task in each selected topic must have Review PASS — the desktop topic
including its own feature contract task; work-signals including both CLI and
desktop surfaces; error classification including its contract change; switch
effectiveness including its first-key, key-rotation, key-removal, and restart
real-session acceptance; and attribution precision including the effective-route
state machine and quality redesign. Non-attribution planning candidates remain
separate. The attribution gate must prove that no determinable event is
downgraded to `inferred`. This topic does not start early and does not absorb
unfinished work from any of them.

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
- one macOS candidate Build allocated by the manual preflight, recorded in its
  exact-SHA manifest, shared by the App, Widget, and embedded framework, and
  reused by any RC or stable publication of that candidate; ordinary
  development and CI runs do not consume this counter.

Passing those checks still does not select RC or stable publication.

## Task breakdown

### 1. `assemble`

- Merge the branches of every topic marked **Yes** in the assembly list, in
  dependency order, classifying each merge before it happens.
- Review the intersection only. Neither side's already-reviewed behavior is
  re-reviewed; state the exclusions and point at the reviews that cover them.
- Record integration evidence with `unit_kind: integration` bound to the merge
  tree, and append each merge as a round in `reviews/assemble.md`.
- Nothing to merge is a valid outcome while `v0.5.0` development happens
  directly on `main`; say so in the record rather than skipping the task.
- Verification level and merge class requirements come from
  `.agent-instructions/branching.md`.

### 2. `v0-5-0-contract`

- Reconcile the complete `v0.5.0` behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`, on top of the feature-contract text already landed
  by the selected topics.
- Raise the specification version exactly once and record one version-level
  changelog entry, including the error-contract break under a compatibility
  heading.
- Confirm every task in every selected topic has independent Review PASS and that
  release identities — app version, CLI version, wire-contract version, and Cask
  version — agree.
- Synchronize the documentation index and archive lifecycle state.
- Verification level: L2 contract state.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [ ] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

A `vX-Y-Z-contract` topic needs only this file: it reconciles what other topics
already delivered and originates no requirement, surface, or architecture of its
own. The three rows are stated rather than omitted so the emptiness reads as a
decision.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `assemble` | [ ] | [ ] |
| 2. `v0-5-0-contract` | [ ] | [ ] |

Both tasks are blocked until every selected topic is fully reviewed, and
`assemble` precedes the contract task. Commit boundaries follow task boundaries. This topic does not authorize commits, pushes,
certificate creation, secret changes, preflight dispatch, release publication,
Homebrew tap changes, local installation, or external distribution.

## Terminal boundary

This topic ends at `v0-5-0-contract` Review PASS. After an authorized commit and
push, the manual `release-preflight` workflow may establish exact-SHA technical
evidence. The user then decides RC, stable release, or no publication.

## Starting a task

Start the task only after its entry condition is met:

```text
开发：`v0-5-0-contract` / `<task-anchor>`
```

Read `AGENTS.md`, this task, the assembly list above, the desktop topic's Tasks
matrix, the specification versioning contract in `docs/specs/cli-design.md`, and
verification routing. `assemble` additionally requires
`.agent-instructions/branching.md`.
Tick `Dev` only after selected verification passes. An independent reviewer
records a PASS round under `reviews/<task-anchor>.md` before ticking `Review`.
