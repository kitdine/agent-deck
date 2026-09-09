---
status: active
created: 2026-09-08
updated: 2026-09-08
---

# v0.6.0 Contract — Tasks

Target version: `v0.6.0`. This draft establishes version membership and the
integration/contract-closure boundary. It implements no feature and authorizes
no merge, push, PR creation, tag, archive, installation, or release.

This file is the status authority for this contract topic. It is the only
document required: requirements, UX and architecture remain with the feature
topics; this contract adds no product behavior or surface. The Documents matrix
ratifies that document set when this draft is reviewed.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [x] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

## Working context

Workspace `agent-deck.v0-6-0-contract`, branch `feature/v0-6-0-contract`, created
from local main `0d6ce58de0f5ba4e082434dc706efb4d2f94f29c`. No feature branch was
merged to create this workspace. Inherited topic documents on this base do not
describe the latest unmerged feature work; resolve each feature's own binding,
review records, dispatch and exact evidence state before selecting a batch.

The user's 2026-09-08 worktree-status correction applies here even though its
implementation commit `488e787` is still on the schema feature branch: topic
progress stays in its worktree, not in a companion main status commit. Prepare
integration status with the integration change. Do not copy newer governance
files or product content between worktrees as a substitute for authorized merges.

## Assembly list

The selection is inherited from the operator's 2026-09-05 v0.6.0 decision in
[Roadmap](../../roadmap.md#v060--trusted-usage-and-subscription-visibility),
coordinated by `ad-v060-iteration`. This contract proposes to carry that complete
selection; its review establishes the active version membership authority.
Unfinished areas remain selected, not silently deferred. A carrier is not a
completed feature topic or an approved implementation decomposition.

| Selected area | Planning carrier | Topic / current readiness |
| --- | --- | --- |
| Schema compatibility and Hook failure visibility | `ad-schema-compatibility` | `schema-version-signal`; first integration candidate, subject to the exact-state entry checks below |
| Menu-bar and Widget refresh | `ad-desktop-refresh` | Topic decomposition/delivery pending; preserve menu-bar approximately 1 minute and widget 3–5 minute targets with change-driven refresh |
| Snapshot performance | `ad-snapshot-performance` | Topic decomposition/delivery pending; aggregation reuse must verify invalidation and output equivalence before increased refresh frequency |
| Cost and price transparency | `ad-cost-transparency` | Topic decomposition/delivery pending; distinguish actual-spend and API-equivalent estimates, with separate catalog/model/tier audits |
| Actionable health recovery | `ad-health-recovery` | Topic decomposition/delivery pending; cause-specific lock guidance and stale extension inventory, retaining the original bug carriers |
| Codex/Claude subscription accounts, quota, reset and alerts | `ad-subscription-quota` | Topic decomposition/delivery pending; source feasibility, reset-count semantics, freshness, account isolation and opt-in notification deduplication remain required |

Bind the five remaining areas to their actual reviewed topics before assembly;
do not invent topic directories or implementation tasks for them in this file.
If an area yields more than one coherent topic, update this list explicitly.
Adding, excluding or deferring selected scope requires an explicit operator
decision recorded here; readiness alone does not change membership.

Subscription scope includes official reset totals/used/remaining when supported,
natural quota-window resets and locally observed resets as distinct concepts.
Unsupported critical fields remain unavailable with a reason and a disposition,
never zero or silently removed. No reset action, account login switching,
automatic updater or plaintext credential storage is introduced here.

Structured session search, credits conversion, Context Efficiency, Linux, a
public adapter protocol, full extension observability and multi-device
aggregation remain excluded from this version selection. Prior withdrawal
decisions remain in force.

## Entry conditions and incremental assembly

Version membership can be reviewed before all selected topics finish. Each
integration batch selects whole completed topics; it does not wait for unrelated
selected topics to become ready. An incomplete version is not a releasable one.

The contract worktree is a documentation workspace, not an intermediate feature
integration branch. After this document passes review and receives delivery
authorization, its contract PR targets main and establishes the assembly task
and membership there. This PR delivers the version plan, not a task-progress log.
Every later feature PR targets main directly; no feature is merged into
feature/v0-6-0-contract as a prerequisite.

Before accepting a topic into a batch:

- Resolve its live branch, commit/tree, topic matrix, review record and Beads
  delivery boundary. All required tasks must be reviewed and committed.
- Query/reuse its exact-content completion evidence, including explicit waiver
  dispositions. A waiver is an accepted limitation, not an executed passing test.
- Compare the source with the actual target baseline; account for overlapping
  interfaces, configuration, contracts, dependency changes and conflict results.
- Confirm the selected topic is within this reviewed assembly list and obtain
  the explicit Git/PR/merge authority required by AGENTS.md and Branching.

`schema-version-signal` is the first candidate: feature branch
`feature/schema-version-signal`, workspace `agent-deck.schema-version-signal`,
commit `58df42d2ee84923ce43224162260cd07cb0016fa`, tree
`263f3d6bb8f80b160f9a46ec214cb05918f97b4f`. Its six Task deliveries are complete;
the topic gate was VERIFIED (3/3) at that tree. This is a discovery snapshot,
not integration evidence for a different target. The branch also contains
the explicit worktree-status governance correction; include it in integration
scope rather than treating the PR as product files only.

Task 5's manual text-size/narrow-layout and VoiceOver/interaction acceptance
were explicitly waived by the user. Actual larger-text effects, narrow-layout
judgment, speech/disclosure operation and notice click navigation were not
verified. Preserve that decision and its risks through integration and version
closure; do not reopen the accepted waiver or claim those checks ran.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `assemble` | [ ] | [ ] |
| 2. `v0-6-0-contract` | [ ] | [ ] |

### 1. `assemble`

**Result:** the selected topics are integrated into main with preserved ancestry,
reviewed interactions and exact-target evidence. Work may proceed in batches;
this Task remains open until every selected area is integrated or explicitly
deferred through a membership decision.

- For each batch, record selected topics, source/target references and immutable
  identities, actual merge class, interactions, conflicts and evidence in
  `reviews/assemble.md`. Create that review record when review occurs, not now.
- Each batch is a feature/<topic> to main PR owned by this `assemble` task.
  Prepare its integration review record and this contract's batch update in
  the PR's source worktree so they arrive with the product result. Resolve the
  source against current main; any needed main-to-feature synchronization requires
  explicit merge authorization. Do not route feature code through the contract
  branch or create a second task just to move it from that branch to main.
- Push, create the feature PR and merge only under their explicit authorizations.
  The PR carries the selected feature and its integration record/batch update,
  not a standalone progress projection. Include the inherited
  accepted risks and any new integration findings.
- Use current Branching merge classes and review the actual result, not merely
  textual conflict status. No squash, cross-line rebase or cherry-pick without
  an explicit policy exception. Reuse unchanged source evidence; verify affected
  interactions and satisfy actual required CI/remote protection rules.
- Update this contract's batch state, Beads and CEv1 in their own roles. Update
  global project status with the authorized integration result; never generate
  companion main commits for intermediate task progress.
- A partial batch can have its own reviewed integration result without ticking
  this aggregate Task complete. Do not merge an unfinished feature merely to
  finish a batch. A later main change requires an impact assessment before the
  next batch, not an automatic merge or unconditional test rerun.

**Verification:** classify each batch through `.agent-instructions/branching.md`
and the project L0–L4 matrix. Bind integration gates to actual parent/result
identities. No fixed full-suite checklist or release-preflight obligation is
introduced for a documentation-only preparation or unchanged verified content.

### 2. `v0-6-0-contract`

**Depends on:** assembly of all retained selected topics and their required
reviews/evidence, including recorded operator waivers or explicit deferrals.

**Result:** the integrated v0.6.0 behavior and version-level documentation agree.

- Reconcile `docs/specs/cli-design.md` and affected `cli-manual.md` content across
  the assembled topics. Read the then-current spec revision; do not prescribe
  revision 29 or 30 before all feature contract edits have landed.
- Add one version-level specification history entry and raise its frontmatter
  consistently. Include the schema refusal compatibility narrowing from
  runtime_error to schema_ahead at unchanged ordinary error exit 1, and other
  verified compatibility changes from selected topics. No standalone CHANGELOG
  or publication/release notes are created by this task.
- Check version identities against their owners (tag/build metadata, app build
  settings, wire contracts and packaging templates). Do not set product versions
  or allocate a release build merely to make a draft version contract look done.
- Reconcile the index pointer and integrated status in the same authorized
  closure change. Retire only fully delivered topics under the documentation
  lifecycle when that boundary is authorized, preserving review/evidence history.
- Preserve untested user-waived acceptance as such. Topic completion, version
  contract completion and release readiness are separate assertions.

**Verification:** L0 documentation/link/version consistency plus the required
version-contract completion gate, reusing valid integration/feature evidence.
Run product checks only for concrete changed assumptions or uncovered criteria.
Technical preflight, signing/notarization, tags, RC/stable publication and local
installation remain separate exact-SHA workflows requiring explicit authority.

## Current handoff

Document re-review passed; see [the review record](reviews/tasks.md).
The reviewed version plan awaits authorized delivery. No feature has been integrated by this contract,
neither task has started, and no implementation dispatch is created before this
decomposition passes review. The first executable batch candidate is
schema-version-signal; four inherited Documents and six completed Task records
remain in its own feature worktree until authorized integration.

Current scope: version-plan delivery checkpoint; implementation remains unstarted.
