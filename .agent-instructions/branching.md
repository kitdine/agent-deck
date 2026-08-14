# Branch Model and Integration

Read this file only when current work crosses a branch, merge, or version-line
boundary. Repository plans, contracts, review records, and CEv1 evidence remain
authoritative for their respective concerns.

## Branches

| Branch | Purpose | Lifecycle |
| --- | --- | --- |
| `main` | Integration trunk. Holds work that completed review, and carries the release tags. | Permanent |
| `feature/vX.Y.Z` | One version's feature development. Any number may exist at once. | Created when that version's development starts; deleted after the version is released and merged |
| `release/vX.Y.x` | Patch line for an already released version. | Created only when a released version needs a patch; kept for later patches |

`main` is not protected yet. Until it is, `v0.5.0` development continues on
`main` directly and `main` is in effect that version's feature line. Protection
and a required pull request are enabled after `v0.5.0` is released; from then on
`main` receives merges only.

No workflow change is required by this model. `ci.yml` already runs on
`branches: ["**"]`, `release-preflight.yml` takes an exact `target_sha` with no
branch concept, and `release.yml` triggers on `v*` tags and verifies that the
tag's commit has preflight evidence for that SHA.

## Merge direction

Merges flow one way, from the oldest line toward the newest:

```text
release/v0.5.x ──→ main ──→ feature/v0.6.0 ──→ feature/v0.7.0
```

Never merge backward. A fix belongs first to the oldest line that needs it, and
then flows forward. Fixing it on a newer line and copying it back produces two
commits with the same content and unrelated ancestry, which conflicts again on
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

Determine the class before merging, because it decides how much work follows:

```bash
git merge-base --is-ancestor main feature/v0.6.0   # exit 0 means fast-forwardable
```

| Class | Tree | Plan | Review | Evidence |
| --- | --- | --- | --- | --- |
| Fast-forward | Unchanged | No | No | None needed. The downstream evidence already binds this exact tree |
| Three-way, no conflict | New | No | **Yes**, limited to the intersection | Integration evidence |
| Three-way, conflicted | New, and contains hand-written code | **Yes** | **Yes**, the resolution is the subject | Integration evidence plus the review record |

Fast-forward is the preferred path: keep the upstream an ancestor of the
downstream by syncing regularly, and integration costs nothing.

A clean three-way merge still needs review. Its tree is the first content state
that contains both sides, so the interaction between them has never been
exercised by either side's tests. It needs no plan, because no one made a design
decision — git combined two already reviewed change sets mechanically.

A conflicted merge needs a plan, because resolving a conflict means writing new
code and making design decisions that neither side's review approved. That code
has no plan, no review, and no test coverage until this work provides them, and
it can introduce a defect present on neither side.

## Integration review scope

Review the intersection only. Never re-review either side; both already hold
task-level review records, and a second verdict on the same content competes
with the first.

```bash
B=$(git merge-base feature/v0.6.0 main)
comm -12 <(git diff --name-only $B feature/v0.6.0 | sort) \
         <(git diff --name-only $B main | sort)              # files both sides touched
git diff --diff-filter=U --name-only                          # conflicted files
```

Scope is those files, plus call sites where one side changed an interface the
other side consumes, plus every line written to resolve a conflict. Files
touched by only one side with no consuming relationship are out of scope.

State the exclusions explicitly in the review record and point at the review
that already covers them, so a reader can tell the difference between "not
reviewed" and "reviewed elsewhere".

## Where integration work is recorded

Integration belongs to the **target version's contract plan**, not to a separate
plan of its own. A merge is a cost that version pays to integrate, so recording
it anywhere else fragments the version's history.

```text
docs/plans/v0-6-0-contract.md
  ### 1. `v0-6-0-contract`        contract and documentation closure
  ### 2. `integration-from-main`  integrating what main already carries

docs/reviews/v0-6-0-contract/
  v0-6-0-contract.md
  integration-from-main.md
```

This reuses the existing `vX-Y-Z-contract` structure — `v0-4-0-contract` and its
review directory are the precedent — so no new directory level, plan type, or
frontmatter field is introduced. The review record's `plan:` field names a plan
file that actually exists, which keeps the derivation rule in
`docs/reviews/README.md` intact.

One task anchor covers every merge into that version. Repeated syncs accumulate
as `## Round 1..N` in the same review file, which is what the round mechanism is
already for. A clean merge appends a round whose scope notes that there was no
conflict; a conflicted merge appends a round that also records the conflicted
files and the resolution decisions.

## Review record shape

Use the standard template from `docs/reviews/README.md`. Only `Reviewed state`
and `Scope` differ:

```markdown
## Round 1 — YYYY-MM-DD
- Reviewed state: <merge tree>
  - Parents: <feature tree> (feature/v0.6.0), <main tree> (main)
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

Use the existing CEv1 fields. This requires no schema change:

| Meaning | Field |
| --- | --- |
| Evidence scope | `work_unit_id`, matching the plan task anchor |
| Kind of unit | `unit_kind: integration` |
| Resulting state | `target_content_state` — the merge tree |
| Source states | `from_state_id`, `to_state_id`, `previous_target_content_state` |
| Link to the verdict | `review_record`, `review_verdict` |

Relate the parent states with `CEv1Relation` rather than inventing a property.
Never add fields to `CEv1Node`; the standing authority in
`.agent-instructions/evidence.md` covers idempotent upserts, not schema changes.

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

A already released or already merged contract cannot anticipate values a later
version will introduce, so never ask an earlier version to reserve a degradation
rule for an unknown future value. When a later version renames or redefines a
contract value, that later version re-enumerates the consumers present on the
target branch at the time and decides whether the wire version must be raised.

## Patching a released version

```text
                    [v0.5.0]
                       │
release/v0.5.x ────────●──●──[v0.5.1]
                                 │
main           ──●──●──●──●──────┴─merge──●
                                            │
feature/v0.6.0 ────●──●──●──●──────────────┴─merge──●
```

1. Branch from the **tag**, not from `main`:
   `git switch -c release/v0.5.x v0.5.0`
2. Fix it on that branch through the normal workflow: bounded plan,
   development, verification, independent review.
3. Release `v0.5.1` from that branch — push, dispatch preflight for that SHA,
   tag, and let the release workflow run. No workflow needs to know about the
   branch.
4. Forward merge into `main`, then classify and handle the merge.
5. Forward merge `main` into each active `feature/*`, classifying each.
6. Keep `release/v0.5.x` for later patches.

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
| Contract closure | `vX-Y-Z-contract` task 1 and its review |
| Content produced by merging | The same contract plan's integration task and its review |
| Release validation | Release WorkUnit and the preflight artifact |

Cross-check each review record's `Reviewed state` tree against CEv1 by
`target_content_state`.

## Archival

The version's feature plans and its contract plan retire together under the
existing rules in `docs/README.md`, and the contract plan carries the
integration record with it. Add one entry per version to
`docs/archive/README.md`, and when a merge produced behavior that neither source
plan described, say so explicitly — that sentence is what keeps the change from
becoming invisible to a later audit:

```markdown
Integration: merging the v0.5.0 desktop work required conflict resolution on the
snapshot quality contract, recorded as task `integration-from-main` in the
contract plan and reviewed in
`reviews/v0-6-0-contract/integration-from-main.md`. It introduced <behavior>,
which neither source plan described.
```

Branch operations grant no delivery authority. Creating or deleting a branch,
merging, pushing, tagging, releasing, and enabling branch protection each retain
their existing explicit authorization boundaries.
