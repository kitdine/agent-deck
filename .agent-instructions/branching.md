# Branch Model and Integration

Read this file for branch, merge, version-assembly, or release-line decisions.
It defines project integration policy, not authorization to execute Git actions.
`AGENTS.md` and [Project Rules](project-rules.md) retain scope, commit, signing,
push, and history-rewrite boundaries. The shared workflow Skill owns phase
execution, review presentation, and completion receipts.

## What a topic is

A feature topic owns one coherent behavior change and has no version segment in
its name. A version-contract topic owns active membership; [Project Status](../docs/status.md)
projects it, while [Roadmap](../docs/roadmap.md) owns later planning and backlog.
Changing intended membership does not by itself change feature content, branch
identity, or evidence validity.

## Branches

| Branch | Purpose | Lifecycle |
| --- | --- | --- |
| `feature/<topic>` | The topic's implementation and code-bound records | Create or remove only under explicit authorization; retain while unassembled work or required history still depends on it |
| `main` | Released content and authorized version assembly | Permanent integration line |
| `release/vX.Y.x` | Patches to an existing release line | Create from the appropriate release tag when authorized; retain for its patch lifecycle |

A reviewed feature is not automatically selected for release or authorized to
merge. Resolve the actual working and target branches before a branch operation.
Do not move existing dirty work or create a branch merely to enforce a diagram.

Branch protection and PR requirements are remote configuration. Verify them when
the operation depends on them; do not infer current protection from the old
v0.5.0 transition note. Creating protection rules or changing branch strategy
requires explicit authorization, just like branch creation itself.

Workflow files own their current triggers and artifact checks. Consult
`.github/workflows/ci.yml`, `release-preflight.yml`, and `release.yml` for actual
behavior rather than assuming branch names alone enforce release policy.

## Merging is a contract topic action

Completing feature work does not merge it. The version-contract topic selects
what to assemble and records that work under its existing `assemble` task.
A typical contract topic also has its contract/documentation closure task:

```text
docs/topics/vX-Y-Z-contract/tasks.md
  assemble
  vX-Y-Z-contract
```

Select whole coherent feature topics. If a feature must ship in independent
slices, resolve that decomposition before assembly rather than silently merging
half of a reviewed topic. Update active membership in the contract topic and its
status projection; do not maintain another membership list in the roadmap.

### Excluding and deferring topics

| Situation | Treatment |
| --- | --- |
| Unmerged topic is not selected | Leave it out of the assembly list and preserve its branch |
| Another topic is promoted first | Change the authorized assembly selection; do not rewrite the deferred topic merely for ordering |
| Already merged work must be excluded | Prepare an explicit removal/revert and later reintroduction plan; do not reset published history or assume exclusion is cost-free |

A revert preserves ancestry but changes effective content. Later reintroduction
may require reverting the revert or new commits; that needs its own scope and
review. Do not claim that a revert makes future delivery impossible, or that
ancestry alone proves the reverted behavior is present.

Plan synchronization of a deferred branch when upstream changes affect its
contracts or assembly readiness. Execute only authorized branch operations,
not a periodic merge solely because time elapsed.

## Merge direction

Distinguish the operation's purpose:

| Operation | Direction |
| --- | --- |
| Propagate a supported-release fix | Oldest affected release line → `main` → applicable feature lines |
| Refresh a feature's integration base | `main` → `feature/<topic>` |
| Assemble a selected feature | `feature/<topic>` → `main`, under the version-contract task |

Do not merge a newer development line into an older release line as a shortcut
for backporting a fix. Start a shared fix at the oldest affected supported line
when applicable, then propagate it with preserved ancestry and explicit scope.
A special backport or alternative history strategy requires an explicit decision.

## Prohibited operations

The default project policy prohibits cross-line rebase, squash, and cherry-pick
because they discard or replace the ancestry used for traceability. Do not use
them to simplify integration without an explicitly authorized policy exception
and the applicable history-rewrite authority.

```bash
git tag --contains <commit>
git log --ancestry-path <commit>..<release-tag>
```

These queries establish ancestry, not semantic equivalence or whether later
commits reverted a feature. Verify effective content separately where required.
Evidence reuse depends on the observed content and assessments, not the merge
strategy's name.

## Merge classification

Classify the actual references and resulting content, not the topic's position
in an assembly list. Fast-forwardability follows ancestry; it is not restricted
to the first topic assembled.

```bash
git merge-base --is-ancestor <target-ref> <source-ref>
git merge-base --is-ancestor <source-ref> <target-ref>
```

| Class | Content and review treatment | Evidence treatment |
| --- | --- | --- |
| Already integrated / no-op | Record the actual ancestry result; create no artificial merge or review round solely for a no-op | Reuse applicable evidence; still resolve any independently required containing-unit boundary |
| Fast-forward | Target advances to the source's existing tree; do not assume the source was reviewed merely because the move is fast-forwardable | Reuse only evidence valid for that exact target; resolve missing evidence without automatically rerunning all checks |
| Three-way, no textual conflict | Review the integration interactions under project policy; no textual conflict is not proof of behavioral compatibility | Evaluate the result's integration criteria and reuse unaffected child evidence |
| Three-way, with conflict resolution | Review the resolution and affected interactions; route new design decisions through their authorized design/review boundary | Bind evidence to the final result and record the relevant parent identities and impact assessments |

Inspect the actual result tree instead of declaring that every merge necessarily
creates different content. A three-way operation still follows its integration
review policy even when some or all result content already has reusable evidence.

## Integration review scope

Review newly introduced interactions and conflict resolutions. Reuse valid
parent reviews; do not reopen unchanged, already covered behavior merely because
it participated in a merge. Conversely, a historical parent PASS does not waive
review of behavior whose assumptions the integration changed.

Use changed-file overlap as a locator, not the complete scope definition. Include
consumers of changed interfaces, configuration/data-contract interactions, and
hand-written resolutions even when Git reports no same-file conflict.

```bash
merge_base=$(git merge-base <source-ref> <target-ref>)
git diff --name-only "$merge_base" <source-ref>
git diff --name-only "$merge_base" <target-ref>
git diff --diff-filter=U --name-only
```

Name the reviewed interactions and excluded unchanged subjects, with pointers to
the evidence covering those exclusions. Do not declare an entire integration safe
solely from an empty textual-conflict list or a successful merge command.

## Where integration work is recorded

Version assembly uses the version-contract topic's `assemble` task and record:

```text
docs/topics/vX-Y-Z-contract/reviews/assemble.md
```

Append rounds for that subject without replacing another record's history.
Feature-base synchronization remains scoped to the affected feature and its
authorized integration record; do not record an unrelated branch operation as
though it were an assembly into the current release.

## Review record shape

Use the shared Skill's full Review/Re-review format and
[Review Records](review-records.md). Add integration metadata within that format:

- Target/source reference names and their pre-operation commit/tree identities.
- Actual result commit/tree or scoped uncommitted candidate identity.
- Operation class: no-op, fast-forward, clean three-way, or conflict resolution.
- Changed interfaces, consuming paths, conflict resolutions, and covered exclusions.

A fast-forward has old/new ref states, not a newly created two-parent merge commit.
Record actual parents for a merge commit rather than inventing parents for every
operation. Preserve record-local round numbering and original review history.

## Integration evidence

Follow [Evidence](evidence.md) and the shared profile; do not add schema or relation
kinds as part of integration. Resolve the applicable integration WorkUnit from
the project hierarchy rather than overwriting a task WorkUnit's identity or kind.

Create or resolve ContentState records for the relevant parents and result.
For the Neo4j gate, supply `namespace`, `work_unit_id`, and `target_content_state`;
these last two values are stable node IDs. The Git tree is content metadata,
not a replacement for the target ContentState ID.

Use canonical Change, Evidence, and ImpactAssessment records and the profile's
permitted relationships for lineage. Do not invent a generic parent edge merely
because every relationship shares the `CEv1Relation` storage type.

Historical observations remain facts about their original states. Their reuse
for an integration target is a separate judgment: assess candidate impacts,
preserve valid dependencies, and reject invalidated or unresolved evidence.
A merge neither invalidates everything automatically nor guarantees that all
parent evidence remains applicable.

For a different target state, use supported target-bound roll-up evidence after
scope-aware assessment; retain child observations in their original states.
Do not relabel old evidence or rerun every child check merely to reach a new gate.
An integration VERIFIED result establishes its required integration criteria,
not feature completeness, release authorization, or full release readiness.

## Contract changes and consumer responsibility

The side changing a contract must enumerate and update consumers that exist in
its relevant supported context. Integration is not a way to hide a consumer
omitted from an approved task's scope.

Before assembling a deferred feature, assess the current target contracts and
adapt affected consumers under the proper task/design authority. Do not require
an earlier release to have anticipated unknown future values, or silently turn
merge authorization into permission for a new product decision.

## Where documents live

Retain the distinction between canonical coordination documents and code-bound
records, while recording the actual branch and content identity for each claim:

| Document | Location responsibility |
| --- | --- |
| `AGENTS.md`, `.agent-instructions/*` | Canonical project governance on `main`; synchronize branch copies only through authorized work |
| Stable index, cross-topic status, roadmap | Canonical coordination on `main`; they do not prove unmerged feature content is present there |
| Topic requirements, UX, architecture, decomposition | Planning authorities on `main` under the existing convention; topic-local implementation status changes with the branch where work occurs |
| Task/integration review of branch code | Stored with or explicitly bound to that code's reviewed state |
| Document review | Bound to the reviewed document and any dependent specimen, not to an unrelated branch's latest round |
| Shipped behavior in specifications/manuals | Reconciled with the implementation being delivered |

A path existing on two branches is not permission to combine their current state
or round history. Identify which branch/state a review, matrix, or handoff refers
to. Planning documents on `main` grant neither implementation authority nor
release membership by themselves.

## Patching a released version

1. Resolve the oldest affected supported release and the exact tag/base commit.
2. Create or select the authorized `release/vX.Y.x` line; do not branch from newer
   `main` content merely for convenience.
3. Follow the confirmed bug lane. Lane A uses its fix record and independent
   review; a new contract decision follows the appropriate topic progression.
4. Resolve the required evidence, authorize delivery, and run the existing
   commit/push/preflight/tag/publication checkpoints for that exact candidate.
5. Plan and execute authorized forward propagation to `main` and affected feature
   lines, classifying each operation and retaining outstanding handoff work.
6. Retain or remove the patch branch only under its authorized lifecycle.

## Releasing

Normal assembled-version tags are created from the authorized `main` candidate;
supported-release patch tags may be created from the authorized release line.
Do not move a patch onto `main` merely to satisfy an inaccurate "all tags on main"
rule. In either case, identify the exact commit and its required preflight and
release evidence before publishing.

Feature selection belongs to the version-contract assembly decision. Review PASS
or CEv1 VERIFIED does not authorize a tag, release channel, push, or publication.
The actual workflow files and Project Rules define their required checks and
separate authorization boundaries.

## Auditing a version

Use commit ancestry, effective content, scoped review records, and evidence
together:

```bash
git log --oneline <previous-tag>..<release-tag>
git tag --contains <commit>
```

Locate the feature/fix records, the contract topic's assembly and closure records,
and the release evidence through their current or archived pointers. Bind each
claim to its actual reviewed content; resolve the ContentState ID before querying
CEv1 rather than passing a tree hash as though it were that ID.

Check reversions or later changes when the question is whether behavior ships,
not merely whether an original commit is an ancestor. Do not infer a missing
feature from an absent working-tree document without checking its branch/archive.

## Archival

[Documentation Workflow](../docs/documentation-workflow.md) owns retirement and
archive-index rules. Retire each subject only when its required lifecycle boundary
is satisfied; do not impose a competing blanket requirement that every feature
and version-contract topic retire simultaneously.

Keep integration records with their owning contract/topic record and preserve
pointers to feature records that retired earlier. If integration introduced
approved behavior not described by a source topic, record its ownership,
contract, review, and evidence so it remains discoverable in the version audit.

Historical branch-model rationale and the v0.5.0 transition assumptions remain
available in commit `36e4b0e87f0ac0eda603e022afd52db26517a043`, file
`.agent-instructions/branching.md`; they are not current remote configuration.

Creating/deleting branches, merging, rewriting history, pushing, tagging,
publishing, or changing branch protection retains its explicit authorization
boundary. This document does not execute or authorize those actions by itself.
