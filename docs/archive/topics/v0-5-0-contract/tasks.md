---
status: historical
created: 2026-08-06
updated: 2026-09-04
retired: 2026-09-04
---

# v0.5.0 Contract Closure — Tasks

Target version: `v0.5.0`.

This contract topic owns only the final reconciliation after every topic in the
[assembly list](#assembly-list) has reached the terminal reviewed state the
[entry condition](#entry-condition) defines. The
[native desktop app topic](../desktop-app/tasks.md) is one of those five and not
a gate of its own; those two sections are the single place the prerequisite set
is maintained, and it is deliberately not restated here. It owns no product
behavior and does not decide whether the completed version proceeds to an RC, a
stable release, or remains unpublished. It originates no requirement, surface,
or architecture of its own, so it carries only this file.
The corrected ownership model is recorded by the historical
[v0.4.0 contract closure](../../plans/v0-4-0-contract.md).

This file is the only status authority for this topic.

## Assembly list

This list decides what `v0.5.0` contains. A topic carries no version number of
its own, so membership exists here and in
[the active-topic status](../../../status.md#active-development) and nowhere else. Changing the
list is how a topic is added or deferred; no commit, branch, review record, or
evidence moves when it changes.

| Topic | Included | Reason |
| --- | --- | --- |
| [`desktop-app`](../desktop-app/tasks.md) | **Yes** | All six tasks ship together; a topic is merged whole or not at all. |
| [`work-signals`](../work-signals/tasks.md) | **Yes** | Restores the captured activity, workflow, and tooling data behind the desktop Sessions surface and adds the matching CLI surface. Its six implementation tasks and contract task ship as one topic. |
| [`cli-error-classification`](../cli-error-classification/requirements.md) | **Yes** | It changes the documented JSON error contract, turning `runtime_error` into specific not-found codes. That is an observable break for any consumer matching the old code, so it ships in the same tag as the desktop line rather than trailing it: the desktop surface reads those codes, and a version whose UI classifies errors one way while its CLI classifies them another is the worse outcome. The break is announced once, in this version's notes. |
| [`switch-effectiveness-boundary`](../switch-effectiveness-boundary/requirements.md) | **Yes** | One client-neutral Hook operation must persist every accepted Codex or Claude delivery before applying an event-specific route effect. Real-session evidence still fixes Claude's state machine: only `no key -> first key` may apply live; key rotation and removal retain the prior route until restart. Its four tasks cover the shared ledger, advisory/file contract, effective-route policy, and cross-client real-lifecycle acceptance. |
| [`usage-attribution-precision`](../usage-attribution-precision/requirements.md) | **Yes** | The cancelled `v0.6.0` attribution line moves into `v0.5.0` as this active topic, not as a future Backlog reconciliation. It promotes determinable effective routes to `exact`, publishes six attribution-reason counts, separates timeline gaps from real provider spend, and reports their calculable catalog base independently. Pricing, credit, Context Efficiency, and subscription candidates remain independent. A determinable event downgraded to `inferred`, or an unattributed event included in provider spend, is a release blocker and cannot ship. |

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

Every task in each selected topic must have reached its terminal reviewed state
in that topic's own status authority — the desktop topic including its own
feature contract task; work-signals including both CLI and desktop surfaces;
error classification including its contract change; switch effectiveness
including its shared Hook delivery ledger, advisory/file contract, and
effective-route policy; and attribution precision including the effective-route
state machine and quality redesign. Terminal means Review PASS everywhere except
the one case recorded below.

`switch-effectiveness-boundary`'s fourth task, `real-session-acceptance`, is
`n/a` / `n/a` because the operator waived it on 2026-08-26, and
[that topic's `tasks.md`](../switch-effectiveness-boundary/tasks.md) — its only
status authority — is where that decision lives. **The waiver is this gate's
terminal state for that task.** This entry condition does not require a Review
PASS for it, does not treat the waiver as one, and does not reopen or rewrite
it. The limitation the waiver itself records travels with it and is not softened
here: the manual first-key, key-rotation, key-removal, and restart procedure was
not executed, no review record exists for it and none should be created, and
that behavior rests on standing operator experience rather than on the recorded
provider-audit evidence the procedure specifies. Switch effectiveness's other
three tasks still require independent Review PASS like every other selected
task.

Non-attribution planning candidates remain separate. The attribution gate must
prove that no determinable event is downgraded to `inferred`. This topic does
not start early and does not absorb unfinished work from any of them.

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
- **File lists below are stated against the commit baseline**, not against the
  working tree. A path that exists only as uncommitted work is a file this task
  creates, because this task's commit is what first puts it under version
  control. Both tasks in this topic are worked in a tree other topics write to
  at the same time, so the lists exist to bound what each one stages.
- Files, existing at the commit baseline: this `tasks.md` (its `Tasks` matrix
  row for `assemble` only) and `docs/status.md` (the single `v0.5.0` Contract
  Closure row, in the one content-state step the ownership bullet below assigns
  to this task). A merge commit carries whatever the merged topic branches
  already own and already reviewed; `assemble` adds no product file, no
  specification text, and no changelog entry of its own — those are task 2's.
- Creates: `reviews/assemble.md`, one round appended per merge, and the record
  stating "nothing to merge" when that is the outcome. Integration evidence is a
  `completion-evidence/v1` record with `unit_kind: integration` bound to the
  merge tree; it is not a file in this repository, so it appears in no list here.
- Hunk ownership on the two shared files. **`docs/status.md` carries exactly one
  row for this topic — the `v0.5.0` Contract Closure row — and no task-specific
  `assemble` row exists or is to be created.** A second row would put one topic's
  status in two places, which is the thing that row is the single authority
  against. The two tasks therefore divide that one row by sequential content
  state rather than by row identity: `assemble` owns only the hunk carrying it
  from its baseline at the time the task starts to *assembly complete, final
  contract pending*, and stages nothing past that step. Every later state of the
  same row through final contract closure is task 2's, as are the `Documents`
  matrix in this `tasks.md` and every version-identity or specification line in
  `docs/status.md`. Because `assemble` commits first, task 2 takes its hunk
  against the baseline `assemble` has already committed, so the two tasks never
  hold a competing edit of the same line in the shared worktree.
- Verification level and merge class requirements come from
  `.agent-instructions/branching.md`, which selects them per merge class; this
  task does not fix a level in advance.

### 2. `v0-5-0-contract`

- Reconcile the complete `v0.5.0` behavior into `docs/specs/cli-design.md` and
  `docs/specs/cli-manual.md`, on top of the feature-contract text already landed
  by the selected topics.
- Raise the specification version exactly once and record one version-level
  changelog entry, including the error-contract break under a compatibility
  heading. **The changelog is the `## Changelog` table in
  `docs/specs/cli-design.md`**, raised together with that file's `version:`
  frontmatter. This repository has no `CHANGELOG.md`, and release notes are the
  separately authorized release workflow's; this task writes neither.
- Confirm every task in every selected topic has reached the terminal reviewed
  state the [entry condition](#entry-condition) defines, including the recorded
  `real-session-acceptance` waiver it names. That section is the single
  statement of the gate; this bullet does not restate it.
- Confirm the release identities agree, each read from its own authority rather
  than from a value copied into this file:
  - app version — `AGENTDECK_MARKETING_VERSION` in
    `apps/macos/Config/AgentDeck.xcconfig`, which a release build overrides from
    `Makefile`'s `APP_VERSION` (the latest tag with its `v` stripped) through
    `scripts/build-macos-app.sh`;
  - CLI version — `internal/buildinfo`'s `Version`, injected by `Makefile`'s
    `BUILD_LDFLAGS` from `git describe --tags --abbrev=0`, so the Git tag is the
    authority and no tracked file carries the number;
  - wire-contract version — `desktop.WireVersion` in `internal/desktop/desktop.go`,
    with `DesktopSnapshotV1.wireVersion` in
    `apps/macos/AgentDeckShared/DesktopWire.swift` and the `desktop/fixtures/v1/`
    fixtures as its consumers;
  - Cask version — the `@VERSION@` substitution in
    `packaging/homebrew/agentdeck.rb.tmpl`, rendered by
    `scripts/render-homebrew-formula.sh` from the same tag.
  A disagreement is a finding against the task that set the value. This task
  changes no version in product code or build configuration.
- Synchronize the documentation index and archive lifecycle state, per the
  Document Lifecycle rules in `docs/documentation-workflow.md`: one
  `git mv docs/topics/<topic>/ docs/archive/topics/<topic>/` per completed
  selected topic, `status: historical` plus `retired:` in each moved document,
  and one concise entry per topic in `docs/archive/README.md`. `docs/README.md`
  changes only if the documentation topology itself changes.
- Files, existing at the commit baseline: `docs/specs/cli-design.md` (the
  reconciled contract text, the `version:` raise, and exactly one new `Changelog`
  row), `docs/specs/cli-manual.md` (the reconciled command text),
  `docs/status.md` (the same single Contract Closure row, in its later content
  state — the hunk from the baseline `assemble` committed through final contract
  closure — plus the version status lines; **not** the earlier step task 1
  owns), `docs/archive/README.md`, `docs/README.md` only
  under the condition above, this `tasks.md` (its `Documents` and `Tasks`
  matrices), and every document of each selected topic that the `git mv` above
  moves and re-stamps.
- **Consumers of the moved paths.** The retirement above changes where five
  topics live, so it also changes every citation of those paths. The side that
  moves a path updates its consumers — `.agent-instructions/branching.md` states
  the same rule for a contract change — so they belong to this task rather than
  to whoever next trips over a broken link. Each is a link-only or comment-only
  hunk; none carries a behavior change. **Each is tracked at the commit
  baseline**, which is what makes "this hunk and not the rest of the file"
  deliverable at all — Git can stage part of a modification, but an addition is
  all-or-nothing:
  - `docs/roadmap.md` — the one `usage-attribution-precision` link.
  - `internal/usage/routes.go` — the two `switch-effectiveness-boundary/architecture.md`
    documentation-path comments only. **No Go statement, signature, or test
    changes**; this is the one production file this task may touch, and only
    because a moved document's path is written inside it.
  - `reviews/assemble.md` — the five archived-topic links in its ancestry table
    only. No round, verdict, finding, or disposition text changes.
- Not this task's, and not to be staged with it. Both entries follow the same
  attribution rule: an edit this task made does not become this task's to
  deliver when the file belongs to someone else.
  - The pre-existing uncommitted hunk in `work-signals/tasks.md`, authored by
    the already-closed work-signals task 6 delivery sync. It travels into the
    archive with the document it belongs to, and it is neither modified nor
    discarded here.
  - The two `cli-error-classification` link corrections in
    `docs/topics/schema-version-signal/requirements.md`. That file is **absent
    from HEAD** — `git cat-file -e HEAD:<path>` reports it exists on disk but not
    in `HEAD`, and `git status --short` returns the whole topic as a single `??`
    — so it has no baseline against which a hunk exists. Staging it would add an
    entire unreviewed requirements document to the final-contract commit; not
    staging it would drop the link fix. Neither is a commit boundary this task
    can hold, so the edits stay in the working tree, correct and unstaged, and
    belong to the `schema-version-signal` task that will first commit that file.
    The authorization to make the edit was real; it is the deliverability that
    is not this task's.
- Creates: `reviews/v0-5-0-contract.md`, when this task's first review round
  runs. The archived paths under `docs/archive/topics/` are the lifecycle move's
  result rather than files this task authors, which is why they are described in
  the bullet above instead of listed here.
- Verification level: L2 contract state.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [x] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

A `vX-Y-Z-contract` topic needs only this file: it reconciles what other topics
already delivered and originates no requirement, surface, or architecture of its
own. The three rows are stated rather than omitted so the emptiness reads as a
decision.

`tasks.md` Review Round 1 (2026-08-31): **REOPEN** on R1-F1, R1-F2, and R1-F3.
The assembly list and merge-class routing are useful, but the entry condition
cannot be satisfied while it requires Review PASS for switch effectiveness's
operator-waived `real-session-acceptance` task, the two task definitions do not
name their complete Files/creates and identity authorities, and the opening
ownership premise names only desktop while the actual gate covers five selected
topics. The Review cell remains unticked. `reviews/tasks.md` owns the findings
and bounded remediation.

`tasks.md` Repair Round 1 (2026-09-01): R1-F1, R1-F2, and R1-F3 are closed in
this content state. The entry condition now names the recorded 2026-08-26
operator waiver as `real-session-acceptance`'s terminal state while preserving
its stated limitation, both tasks carry commit-baseline `Files` / `Creates`
lists with shared-file hunk ownership and named identity authorities, and the
opening premise routes the prerequisite set to the assembly list and entry
condition instead of naming desktop alone. The Review cell stays unticked
pending independent Re-review; see [`reviews/tasks.md`](reviews/tasks.md).

`tasks.md` Re-review Round 2 (2026-09-01): **REOPEN** on R1-F2. R1-F1 and
R1-F3 are closed, but the two task definitions still assign the sole
`docs/status.md` Contract Closure row inconsistently: `assemble` claims this
topic's stage row while task 2 also owns the `v0-5-0-contract` row and excludes
an `assemble` row that does not exist. The Review cell remains unticked;
[`reviews/tasks.md`](reviews/tasks.md) owns the finding disposition and bounded
remediation.

`tasks.md` Repair Round 2 (2026-09-01): R1-F2 is closed. Both task definitions
now state that `docs/status.md` carries exactly one Contract Closure row, that
no task-specific `assemble` row exists or is to be created, and that the two
tasks divide that single row by sequential content state — `assemble` through
*assembly complete, final contract pending*, task 2 from the baseline `assemble`
committed through final contract closure. No other `Files` list, identity
authority, or entry-condition text changed. The Review cell stays unticked
pending independent Re-review; see [`reviews/tasks.md`](reviews/tasks.md).

`tasks.md` Re-review Round 3 (2026-09-01): **PASS**. R1-F1 and R1-F3 remain
closed, and R1-F2 is now closed by assigning the single Contract Closure status
row sequentially across the two task baselines without inventing a separate
`assemble` row. The Review cell is ticked and this topic is now developable;
[`reviews/tasks.md`](reviews/tasks.md) owns the complete finding disposition and
evidence.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `assemble` | [x] | [x] |
| 2. `v0-5-0-contract` | [x] | [x] |

`assemble` Review Round 1 (2026-09-01): **REOPEN** on R1-F1. The
nothing-to-merge classification, ancestry proof, empty integration scope, and
absence of `unit_kind: integration` evidence are correct, but the delivery
boundary says this task staged its files while the Git index is empty. The
Review cell remains unticked; [`reviews/assemble.md`](reviews/assemble.md) owns
the finding and bounded remediation.

`assemble` Repair Round 1 (2026-09-01): R1-F1 is closed. The delivery boundary no
longer claims the task staged its files; it states that the Git index remained
empty and that Development carries no staging or commit authority, so isolating
the index belongs to the commit checkpoint. The classification, ancestry,
integration-scope, and integration-evidence sections are unchanged. The Review
cell stays unticked pending independent Re-review.

`assemble` Re-review Round 2 (2026-09-01): **PASS**. R1-F1 is closed: the live
Delivery boundary describes only the candidate changes and explicitly states
that the Git index remained empty. The previously verified nothing-to-merge,
ancestry, integration-scope, and integration-evidence conclusions remain
unchanged. The Review cell is ticked; final contract work is now the next task.

`v0-5-0-contract` Development (2026-09-01): delivered. `cli-design.md` is at
version 28 with one new Changelog row and a new Error-Code Compatibility
section; the entry condition and the four release identities are confirmed, with
three of the four tag-derived and therefore unresolved until the authorized
`v0.5.0` tag exists; the five selected topics are retired under
`docs/archive/topics/` with every document `status: historical`. The archive move
also repointed the consumers of the moved paths, under an explicit user
authorization recorded in this round. **Three are this task's to deliver**,
because each is tracked at the commit baseline: `docs/roadmap.md`, two
`internal/usage/routes.go` documentation-path comments, and the five
archived-topic links in `reviews/assemble.md`. The Consumers bullet above names
them, so the ratified boundary and the change set agree instead of the plan
trailing the delivery. **A fourth edit is not**: the two link corrections in the
untracked `schema-version-signal/requirements.md` stay in the working tree and
belong to that topic's own task, for the reason the bullet above records — a
file absent from HEAD has no hunk to isolate. A relative
link resolver run against a clean `git archive` of HEAD and against the working
tree returns identical broken sets except that five `reviews/assemble.md` links,
already broken at HEAD by a wrong relative depth, are now fixed: the move
introduced no new broken link. `git mv` necessarily stages its 62 renames, so the
index is not empty; nothing is committed or pushed. The `Review` cell stays
unticked pending independent review.

`v0-5-0-contract` Review Round 1 (2026-09-01): **REOPEN** on R1-F1. The
Development candidate changed four authorized consumer paths outside the
ratified Task 2 `Files` list while its scoped-delivery criterion still requires
only ratified Files; `reviews/assemble.md` is the fourth path and is also absent
from the Development summary's gap list. The Review cell remains unticked;
[`reviews/v0-5-0-contract.md`](reviews/v0-5-0-contract.md) owns the finding and
bounded remediation.

`v0-5-0-contract` Re-review Round 2 (2026-09-01): **REOPEN** on R2-F1. R1-F1
is closed, but `docs/topics/schema-version-signal/requirements.md` is absent
from HEAD and remains part of an untracked, unreviewed topic, so its two link
edits cannot be isolated as Task 2 hunks: Git can only add the whole file. The
Review cell remains unticked; [`reviews/v0-5-0-contract.md`](reviews/v0-5-0-contract.md)
owns the new finding and bounded remediation.

`v0-5-0-contract` Re-review Round 3 (2026-09-01): **PASS**. R1-F1 and R2-F1
are closed: Task 2 owns exactly three tracked consumers, while the untracked
schema-version-signal requirements edit remains correct in the working tree but
belongs to its own Task and is excluded from the 71-file final-contract
candidate. The other four criteria remain verified. The Review cell is ticked;
all Tasks in this topic have passed review.

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
