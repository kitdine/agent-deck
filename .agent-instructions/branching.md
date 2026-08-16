# Branch Model and Integration

Read this file only when current work crosses a branch, merge, or version
assembly boundary. Repository plans, contracts, review records, and CEv1
evidence remain authoritative for their respective concerns.

## What a plan is

A feature plan owns one coherent behavior change and carries no version number.
Which version ships it is decided by a version contract plan, which lists its
included feature plans. This split is already the repository rule; the branch
model follows it rather than adding a second notion of version ownership.

Branches for feature work are therefore named after the plan topic, never after
a version. A plan whose target version changes needs no branch, commit, review
record, or evidence change.

## Branches

| Branch | Naming | Purpose | Lifecycle |
| --- | --- | --- | --- |
| `feature/<plan-topic>` | Matches `docs/plans/<topic>.md` | One plan's complete set of tasks | Created when that plan's development starts; deleted after the plan is assembled into a version |
| `main` | — | Released content plus the version currently being assembled | Permanent |
| `release/vX.Y.x` | The released version | Patches to an already released version | Created only when a released version needs a patch; kept for later patches |

`main` is not the sum of all work. It is what users already have, plus what is
being prepared for them next. A plan that has passed review but has not been
assigned to a version is deliberately absent from `main`, because it has not
shipped and is not being shipped yet.

`main` is not protected yet. Until it is, `v0.5.0` development continues on
`main` directly and `main` is in effect that version's feature line. Protection
and a required pull request are enabled after `v0.5.0` is released.

No workflow change is required by this model. `ci.yml` runs on
`branches: ["**"]`, `release-preflight.yml` takes an exact `target_sha` with no
branch concept, and `release.yml` triggers on `v*` tags and verifies that the
tag's commit has preflight evidence for that SHA. Nothing in the release path
inspects a branch name.

## Merging is a contract plan action

**A feature plan finishing does not merge anything.** When its last task passes
review, its branch stays where it is. Merging into `main` is a task of the
version contract plan that includes it:

```text
docs/plans/vX-Y-Z-contract.md
  ### 1. `assemble`          merge the selected feature branches, review the integrations
  ### 2. `vX-Y-Z-contract`   contract and documentation closure
```

This is what keeps version membership reversible. There is no point in the
workflow at which an unassigned plan gets merged, so excluding a plan from a
version never requires undoing a merge.

Two rules follow:

- **A plan is merged whole or not at all.** Half a plan is not a coherent
  behavior change. If some of a plan's tasks might ship in a different version
  than the rest, that plan should have been two plans.
- **Version membership lives only in the contract plan and the roadmap in
  `docs/README.md`.** Rescheduling changes those two documents and nothing else
  — no commit, branch, review record, or evidence moves.

### Excluding and deferring plans

Deferring a plan costs nothing as long as it was never merged:

| Situation | Action |
| --- | --- |
| Plan not needed in this version | Leave it out of the contract plan's assembly list. Its branch stays |
| A later plan is promoted ahead of an earlier one | Assemble the promoted one; leave the others' branches alone |
| Plan already merged but no longer wanted | **No clean option.** `revert` propagates forward and will cancel the feature again when it is finally released; `reset` rewrites history and is prohibited |

The third row is the reason merging belongs to the contract plan. Prevention is
the only clean answer.

A deferred branch outlives one or more releases, so merge `main` into it
regularly rather than letting divergence accumulate to a single large conflict
at assembly time. Those merges happen on the feature branch, so they never
disturb `main` or a released version.

## Merge direction

Merges flow one way, from the oldest line toward the newest:

```text
release/v0.5.x ──→ main ──→ feature/<topic>
```

Never merge backward. A fix belongs first to the oldest line that needs it and
then flows forward. Fixing it on a newer line and copying it back produces two
commits with identical content and unrelated ancestry, which conflicts again on
every later merge.

## Prohibited operations

Never `rebase`, `squash`, or `cherry-pick` across these branches.

The reason is traceability, not evidence preservation. Every merge produces a
new tree that no prior evidence covers, so no merge strategy protects existing
evidence. What the prohibition protects is ancestry, and ancestry is what makes
these queries answerable:

```bash
git tag --contains <commit>                   # which releases contain a change
git log --ancestry-path <commit>..v0.6.0      # prove it is in that lineage
```

`cherry-pick` is the most damaging: the copied commit has no ancestry link, so
`git tag --contains` cannot find it and forward traceability breaks. Which
releases carry a given patch then becomes unanswerable by tooling.

## Merge classification

Classify before merging; the class decides how much work follows.

```bash
git merge-base --is-ancestor main feature/<topic>   # exit 0 means fast-forwardable
```

| Class | Tree | Plan | Review | Evidence |
| --- | --- | --- | --- | --- |
| Fast-forward | Unchanged | No | No | None needed. Downstream evidence already binds this exact tree |
| Three-way, no conflict | New | No | **Yes**, intersection only | Integration evidence |
| Three-way, conflicted | New, contains hand-written code | **Yes** | **Yes**, the resolution is the subject | Integration evidence plus the review record |

Only the first plan assembled into a version can fast-forward; once `main` has
that plan's commits, the next assembly is a three-way merge unless the branch
first merges `main`. Keeping deferred branches synced with `main` is what makes
later assemblies cheap.

A clean three-way merge still needs review. Its tree is the first content state
containing both sides, so their interaction has never been exercised by either
side's tests. It needs no plan, because nobody made a design decision — git
combined two already reviewed change sets mechanically.

A conflicted merge needs a plan, because resolving a conflict means writing new
code and making design decisions no review approved. That code has no plan, no
review, and no test coverage until this work provides them, and it can introduce
a defect present on neither side.

## Integration review scope

Review the intersection only. Never re-review either side; both already hold
task-level review records, and a second verdict on the same content competes
with the first.

```bash
B=$(git merge-base feature/<topic> main)
comm -12 <(git diff --name-only $B feature/<topic> | sort) \
         <(git diff --name-only $B main | sort)              # files both sides touched
git diff --diff-filter=U --name-only                          # conflicted files
```

Scope is those files, plus call sites where one side changed an interface the
other side consumes, plus every line written to resolve a conflict. Files
touched by only one side with no consuming relationship are out of scope.

State the exclusions explicitly in the review record and point at the review
that already covers them, so a reader can tell "not reviewed" from "reviewed
elsewhere".

The consuming call sites matter most for a long-deferred plan. Git reports no
conflict when one side renames a contract value and the other side reads it from
a different file, so a semantic mismatch survives a clean merge and only this
review catches it.

## Where integration work is recorded

Integration belongs to the target version's contract plan, as the `assemble`
task. It is not a plan of its own.

```text
docs/reviews/vX-Y-Z-contract/
  assemble.md              every merge into this version
  vX-Y-Z-contract.md
```

This reuses the existing `vX-Y-Z-contract` structure — `v0-4-0-contract` and its
review directory are the precedent — so no new directory level, plan type, or
frontmatter field appears. The review record's `plan:` field names a plan file
that exists, keeping the derivation rule in `docs/reviews/README.md` intact.

One task anchor covers every merge into that version. Assembling several plans,
or repeatedly syncing one, accumulates as `## Round 1..N` in the same file, which
is what the round mechanism is already for.

## Review record shape

Use the standard template from `docs/reviews/README.md`. Only `Reviewed state`
and `Scope` differ:

```markdown
## Round 1 — YYYY-MM-DD
- Reviewed state: <merge tree>
  - Parents: <feature tree> (feature/<topic>), <main tree> (main)
  - Merge type: fast-forward | three-way-clean | three-way-conflict
- Scope: content this merge newly produced. Neither parent's already reviewed
  behavior is re-reviewed.
  - Conflicted files: ...
  - Consuming call sites: ...
  - Out of scope: ... (covered by <plan-topic>/<task-anchor>.md)
```

Recording both parent trees is the only structural difference from an ordinary
review record, because an integration verdict is about a transition between two
verified states rather than about one state.

## Integration evidence

Use the existing CEv1 fields. No schema change is required or permitted.

| Meaning | Field |
| --- | --- |
| Evidence scope | `work_unit_id`, matching the plan task anchor |
| Kind of unit | `unit_kind: integration` |
| Resulting state | `target_content_state` — the merge tree |
| Source states | `from_state_id`, `to_state_id`, `previous_target_content_state` |
| Link to the verdict | `review_record`, `review_verdict` |

Relate the parent states with `CEv1Relation` rather than inventing a property.
The standing authority in `.agent-instructions/evidence.md` covers idempotent
upserts, not schema changes.

A merge does not invalidate anything. A plan's task evidence states that its
tree passed review, which stays true forever; it simply does not cover the merge
tree, because that is different content. A long-deferred plan therefore
accumulates a chain of integration states — each sync produces a new tree
needing its own integration evidence — while its original task evidence remains
valid and cited.

What an integration gate means is narrow, and must not be widened:

| Claim | Established by |
| --- | --- |
| This merge did not break either side's verified behavior | The integration gate |
| The version's features are correct | Task and Plan WorkUnits |
| The result is releasable | Release WorkUnit, preflight, and L4 |

## Contract changes and consumer responsibility

**The side changing a contract updates every consumer that exists when the
change is made.** Integration handles two sides that are each correct but
interact badly; it does not absorb a consumer a plan failed to enumerate. A
missed consumer means that version's plan scope was incomplete, and it is fixed
inside that version.

This applies in both directions, which matters for a deferred plan. When a plan
waits out a release, the contracts it consumes may have changed on `main` in the
meantime. The deferred plan is then the party whose scope is incomplete relative
to the branch it is joining, and it re-enumerates its consumers and adapts
before assembly — the work belongs to that plan, not to the merge.

An already released or already merged contract cannot anticipate values a later
version will introduce, so never ask an earlier version to reserve a degradation
rule for an unknown future value.

## Where documents live

Documents that constrain every branch, or that describe work not yet done, live
on `main`. Documents describing what a branch has already done live with that
code.

| Document | Branch |
| --- | --- |
| `AGENTS.md`, `.agent-instructions/*`, `docs/reviews/README.md` | `main` — they bind all branches |
| `docs/README.md` index, roadmap, backlog | `main` — single source of truth |
| `docs/plans/<topic>.md`, `docs/specs/<topic>-design.md` | `main` — they describe intent and change no behavior |
| `docs/reviews/<plan-topic>/<task-anchor>.md` | With the code — a verdict binds a tree on that branch |
| Behavior text in `cli-design.md` and `cli-manual.md` | With the code — it must ship with the implementation it describes |

A plan document on `main` authorizes no development and implies no version
membership. Its `Dev` and `Review` cells are ticked on the branch where the work
happens, and those ticks reach `main` with the assembly merge.

## Patching a released version

```text
                    [v0.5.0]
                       │
release/v0.5.x ────────●──●──[v0.5.1]
                                 │
main           ──●──●──●──●──────┴─merge──●
                                            │
feature/<topic> ───●──●──●──●──────────────┴─merge──●
```

1. Branch from the **tag**, not from `main`:
   `git switch -c release/v0.5.x v0.5.0`
2. Fix it there through the normal workflow: bounded plan, development,
   verification, independent review.
3. Release `v0.5.1` from that branch — push, dispatch preflight for that SHA,
   tag, and let the release workflow run. No workflow needs to know the branch.
4. Forward merge into `main`, then classify and handle that merge.
5. Forward merge `main` into each active `feature/*`, classifying each.
6. Keep `release/v0.5.x` for later patches.

## Releasing

Tags are cut on `main`. A version's content is everything on `main` at the tag,
so no selection step exists at release time — selection already happened when
the contract plan chose what to assemble.

1. `assemble` merges the selected feature branches and their integrations pass
   review
2. The contract task closes the version's contracts and documentation
3. Push, dispatch preflight for that exact SHA, tag, release

## Auditing a version

Every change in a release is reachable, with nothing recorded outside these
places:

```bash
git log --oneline v0.5.0..v0.6.0          # commits the version introduced
ls docs/archive/reviews/<plan-topic>/ \
   docs/archive/reviews/vX-Y-Z-contract/  # every review record, integration included
git tag --contains <commit>               # which releases carry a change
```

| Change origin | Recorded in |
| --- | --- |
| Feature work | Each feature plan and its review directory |
| Content produced by merging | The contract plan's `assemble` task and its review |
| Contract closure | The contract plan's contract task and its review |
| Release validation | Release WorkUnit and the preflight artifact |

Cross-check each review record's `Reviewed state` tree against CEv1 by
`target_content_state`.

## Archival

A version's feature plans and its contract plan retire together under the rules
in `docs/README.md`, and the contract plan carries the integration record with
it. Add one entry per version to `docs/archive/README.md`. When a merge produced
behavior no source plan described, say so explicitly — that sentence is what
keeps the change from becoming invisible to a later audit:

```markdown
Integration: assembling the desktop work required conflict resolution on the
snapshot quality contract, recorded as task `assemble` in the contract plan and
reviewed in `reviews/v0-6-0-contract/assemble.md`. It introduced <behavior>,
which no source plan described.
```

Branch operations grant no delivery authority. Creating or deleting a branch,
merging, pushing, tagging, releasing, and enabling branch protection each retain
their existing explicit authorization boundaries.
