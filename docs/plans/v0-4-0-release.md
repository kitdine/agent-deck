---
status: active
created: 2026-08-06
---

# v0.4.0 Release

Target release: `v0.4.0`.

This is a **release plan**, not a feature plan. It owns no product behavior.
It exists because some work belongs to a version rather than to any feature,
and that work must not be attached to a feature plan that happens to finish
last.

## Two Kinds of Contract Task

A contract task reconciles delivered behavior with the living specification.
There are two kinds, and they belong to different owners:

| Kind | Owner | Scope | Raises spec version |
| --- | --- | --- | --- |
| Feature contract | The feature plan that delivered the behavior | Only what that plan delivered | No |
| Release contract | The release plan for that version | The whole version, across every plan in it | Yes, exactly once |

A feature plan writes the contract text for its own behavior. It never raises
the specification version, because a version raise describes a release, and a
release is not owned by whichever feature plan happened to finish last.

Before 2026-08-06 this project attached the release contract to a feature plan:
`v0-3-0-contract` lived in `runtime-provider-attribution`, `v0-4-0-contract` in
`session-experience`, and `v0-5-0-contract` in `desktop-app`. That coupling made
a release gate look like a feature deliverable and made the last feature plan
implicitly responsible for the release. `v0.3.0` already shipped that way and is
not rewritten; `v0.4.0` and `v0.5.0` are corrected here.

## Scope

`v0.4.0` carries two independent feature lines:

- session experience — six tasks complete, with its contract absorbed into
  [the CLI design](../specs/cli-design.md) and
  [manual](../specs/cli-manual.md);
- [usage report presentation](usage-report-presentation.md) — six tasks.

Each writes its own feature contract text. This plan reconciles the result,
raises the specification version once, validates the release candidate, and
prepares the release notes.

## Entry Condition

Every task in both feature plans has Review PASS, including each plan's own
feature contract task. This plan does not start early, and it does not absorb
unfinished feature work.

## Release Characteristics

These determine the gates below and must be re-verified rather than assumed:

- The release changes a **persisted session-index format**. The session parser
  version increases and the rebuildable session-index migration recreates
  affected FTS and source state.
- It **reads event-time pricing for a new surface** (invocation-level cost),
  without rewriting stored rows or historical prices.
- It adds **additive JSON fields and collections** only. No field is removed or
  renamed.
- The usage presentation line is **text-only** and changes no stored value, so
  it contributes no downgrade or migration risk of its own.

Because the first two touch persisted data and the pricing read path, this
release requires at least one `-rc.N` validated against real local data, under
the same rule that governed `v0.2.1` and `v0.3.0`.

## Tasks

### 1. `v0-4-0-contract`

- Reconcile the complete `v0.4.0` behavior across both feature lines into
  `docs/specs/cli-design.md` and `docs/specs/cli-manual.md`, on top of the
  feature contract text each plan already landed.
- **Raise the specification version exactly once** and record one changelog
  entry covering the whole release.
- Confirm every task in both plans has an independent Review PASS and that the
  documentation index, plan status matrices, and review records agree.
- Verify the desktop-facing DTO readiness that `v0.5.0` depends on, and state
  explicitly whether `desktop-wire-contract` is unblocked.
- Verification level: L2 for contract state.

### 2. `v0-4-0-release-candidate`

- Build and validate `v0.4.0-rc.1` against **isolated copies** of real local
  AgentDeck state and source logs. Never against the real state root.
- Exercise the session-index migration and rebuild path on a copied database,
  confirming source logs are untouched and that a failed migration leaves no
  partially exposed content.
- Confirm invocation-level cost output against known event-time prices, and
  confirm no stored usage row or historical price was rewritten.
- Record the RC evidence, including what was validated and what could not be.
- Verification level: L4 via `make release-verify` as the aggregate gate.

### 3. `v0-4-0-release`

- Write release notes covering both feature lines, the additive JSON fields, the
  session-index format change, the rebuild requirement, and any downgrade
  consequence the RC surfaced.
- Publish the tag and GitHub Release, then promote the Homebrew Formula.
- **Requires explicit authorization immediately before execution.** Neither this
  plan nor a Review PASS grants it.
- Verification level: L4.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `v0-4-0-contract` | [ ] | [ ] |
| 2. `v0-4-0-release-candidate` | [ ] | [ ] |
| 3. `v0-4-0-release` | [ ] | [ ] |

Task 1 is blocked until both feature plans are fully reviewed. Task 2 depends on
task 1. Task 3 depends on task 2 and on explicit release authorization.

Commit boundaries follow task boundaries. This plan does not authorize commits,
pushes, release tags, RC publication, or real-state mutation.

## Starting Task

Turn a Status row into scoped work by naming its anchor:

```text
进入开发：`v0-4-0-release` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's named task, both feature plans' Status matrices,
the specification versioning contract in `docs/specs/cli-design.md`, and
verification routing. Tick `Dev` only after selected verification passes. An
independent reviewer records a PASS round under
`docs/reviews/v0-4-0-release/<task-anchor>.md` before ticking `Review`.
