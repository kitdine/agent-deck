---
status: historical
created: 2026-08-06
retired: 2026-08-10
---

# v0.4.0 Contract Closure

Target version: `v0.4.0`.

This plan owned the version-level contract reconciliation after the session
experience and usage report presentation feature plans completed. It owned no
product behavior and did not decide whether the completed version would proceed
to an RC, a stable release, or remain unpublished.

## Contract ownership

| Kind | Owner | Scope | Raises specification version |
| --- | --- | --- | --- |
| Feature contract | The feature plan that delivered behavior | Only that plan's behavior | No |
| Version contract | The version-level contract closure | All reviewed feature plans in the version | Yes, exactly once |

Before 2026-08-06, AgentDeck attached version raises to whichever feature plan
finished last. `v0.3.0` shipped under that historical model and is not rewritten.
For `v0.4.0`, the separate contract closure prevented either feature line from
implicitly owning the release decision.

## Scope

`v0.4.0` contains two independently reviewed feature lines:

- session experience — six tasks, with current behavior in the living CLI
  design and manual;
- usage report presentation — six tasks, with its execution record archived in
  `docs/archive/plans/usage-report-presentation.md` and current behavior in the
  living CLI design and manual.

The contract closure reconciled both lines, raised the CLI design to version 24
exactly once, synchronized the documentation index, and confirmed that the
bounded session DTO unblocked the later `desktop-wire-contract` task.

## Task

### `v0-4-0-contract`

- Reconcile complete `v0.4.0` behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md` on top of each feature plan's contract text.
- Raise the specification version exactly once with one version-level changelog
  entry.
- Confirm both feature plans, their review records, archive lifecycle metadata,
  the documentation index, and the desktop dependency agree.
- Verification level: L2 contract state.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| `v0-4-0-contract` | [x] | [x] |

Round 1 found five archived session-experience review files whose frontmatter
still said `active`. The repair changed only those lifecycle markers. Round 2
re-review passed on 2026-08-10 with the version-24 contract, manual, index,
desktop dependency, feature-plan verdicts, and archive metadata synchronized.

## Terminal boundary

This plan ends at `v0-4-0-contract` Review PASS. A review verdict or technical
verification result cannot choose an RC or stable release. Any later release
preflight is evidence bound to an exact commit SHA; RC versus stable remains a
separate explicit user decision, and commit, push, tag, publication, Formula
promotion, and real-environment changes each retain their own authorization.

An earlier local L4 and isolated-real-state run remains useful technical
preflight evidence for unchanged product content. It is not an RC approval and
is not a task in this plan.
