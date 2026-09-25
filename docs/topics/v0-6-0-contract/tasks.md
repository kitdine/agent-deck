---
status: active
created: 2026-09-08
updated: 2026-09-14
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
| Schema compatibility and Hook failure visibility | `ad-schema-compatibility` | `schema-version-signal`; integrated through PR #3 at main `f2b7d23`, with its reviewed manual acceptance waivers retained |
| Menu-bar and Widget refresh | `ad-desktop-refresh` | `desktop-refresh`; integrated through PR #6 at main `4bc0756`, with merge-result gate VERIFIED 2/2 and native acceptance retained as BLOCKED/no-waiver |
| Snapshot performance | `ad-snapshot-performance` | `snapshot-performance`; integrated through PR #4 at main `4dd10f4`, with the reviewed performance and native acceptance exceptions retained |
| Cost and price transparency | `ad-cost-transparency` | Topic decomposition/delivery pending; distinguish actual-spend and API-equivalent estimates, with separate catalog/model/tier audits |
| Actionable health recovery | `ad-health-recovery` | `health-recovery`; 5/5 tasks delivered, signed PR repair `6c15672` independently reviewed in Round 2 with exact candidate topic gate VERIFIED 5/5; fifth integration batch candidate, retaining both origin bugs |
| Codex/Claude subscription accounts, quota, reset and alerts | `ad-subscription-quota` | `subscription-quota`; integrated through PR #5 at main `7f84749`, with fail-closed hosted-XCTest isolation preserved and deferred P2 dispositions kept explicit |

Bind the one remaining area to its actual reviewed topic before assembly;
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

`snapshot-performance` is the second candidate: feature branch
`feature/snapshot-performance`, workspace `agent-deck.snapshot-performance`,
commit `7e455a8f1d6cf99152ea75c925f9194542ad9303`, tree
`74f6231890fa9895a83c73f100d392c224230ccc`. Current `main`
`f2b7d23accfbb0ba1e940ef77ee794cab0cf7c7f` is its merge base and ancestor, so
the local reference class is fast-forward with no main-to-feature synchronization
required. The topic boundary is VERIFIED 3/3 at its delivery commit `cbeaa4b2`;
the later `xctest-state-isolation` repair is independently reviewed and VERIFIED
5/5 at the branch head. Integration review must preserve the recorded cold-import,
unchanged-refresh CPU, final 20-sample, V01-V19 and manual/native acceptance
exceptions as accepted limitations, not technical passing results.

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

### Fifth batch candidate — health-recovery — 2026-09-25

`health-recovery` is selected in the reviewed assembly list. Its five Tasks are
reviewed and delivered; the signed source repair is
`feature/health-recovery` `6c15672c20ed57a631a7da7e9c7930dc66333fe2` / tree
`9af754df84bfac8c376566cf72aa718d985bd4f8`. The independent Round 2
source review passed; its record and topic handoff were delivered by signed
docs-only commit `543d074d9b97d62050a5ad8b36e75c373c33c9ee` / tree
`43d9fb2b7231f52b90001853db4e448bee364d91`. Its five-criterion topic
gate is VERIFIED for that immutable ContentState recorded on the WorkUnit.
The old `3e7ab8b` gate is not relabeled.

The actual remote target is `origin/main`
`a396c2f2158f579e70da3aaf9bd0bff084fc1f1a` / tree
`0372bc4a929a62b7b26e73f6c8307f7704169a53`, which is the source merge
base and ancestor. This local reference class is direct fast-forward with no
target-only product change or textual conflict. Local `main` also has the
separate, protected-branch rule commit `e84ce1f` that is not on remote main;
do not treat it as integrated, and refresh the target comparison if it lands.

This batch adds cause-specific lock and extension diagnosis, explicit bounded
inventory synchronization, additive Go/Swift desktop wire and bilingual
menu-bar copy actions. Integration review must inspect the resulting CLI and
desktop health contracts against the inherited schema/Hook, refresh and quota
consumers, despite the ancestry-only merge class. The native VoiceOver reading
order remains BLOCKED/no-waiver, and real installed-client observation remains
SIMULATED; neither is a technical PASS.

`v0-6-0-contract:integration:health-recovery` is the batch WorkUnit. Round 7
independent integration review passed; its two required criteria are queried
for each exact source/status candidate, with the current target on the WorkUnit.
The source-record commit changes no product or tests. No batch integration
commit, push, PR, merge, retirement, contract closure or release has been
performed. Aggregate `assemble` remains open for this batch and cost transparency.

### Historical fourth batch candidate — desktop-refresh — 2026-09-20

`desktop-refresh` is delivered on `feature/desktop-refresh` at signed commit
`cc4151bdcae0689dd025da763b2a3b84eab10d54` / tree
`246ced1b44ae26df83159a548bf37ed9d4bea75d`. Current target `main` is
`7f84749f8bfa96496602560df0b4d5da09e6fd9d` / tree
`41cc1390c6a4949c4fbe5cde087925be12f356b7`, is the merge base and an ancestor
of the source, so the local operation class is direct fast-forward. There is no
target-side divergence, conflict resolution or need for a main-to-feature
synchronization merge.

The topic matrix is 5/5 implemented and reviewed. Task 5 is delivered by the
source HEAD, its commit-bound WorkUnit is `delivered`, and all five required
criteria are target-bound PASS. The batch candidate preserves the already
integrated schema, snapshot-performance and subscription-quota baselines while
adding the completion-driven app scheduler/coordinator/publisher path, typed
bounded Widget loading, 3/4/5-minute timeline requests, semantic targeted reload
and reconciled English/Chinese stable contracts.

Round 6 integration review passed for the exact local candidate and covered the shared App lifecycle and
`EmbeddedHelperRunner` boundary inherited from subscription-quota, App Group
publication/reader compatibility, the Go wire hint consumed by macOS, and the
menu/Widget presentation recovery paths. The reviewed candidate preserves Task 5's native
acceptance accounting: real ten-cycle timing, installed Widget callbacks and
intents, real sleep/wake, VoiceOver, Increase Contrast and gallery observations
remain BLOCKED with no waiver and are not technical PASS.

The integration WorkUnit is
`v0-6-0-contract:integration:desktop-refresh`, with required criteria
`integration-readiness` and `source-continuity`. Round 6 records PASS and the
exact synchronized candidate gate is VERIFIED 2/2. No push,
PR creation, merge, retirement, version closure or release is performed. The
aggregate `assemble` Task remains open for this batch and the two other selected
areas that have not yet produced integrated topics.

### Delivered baseline inherited from main

The version plan was delivered through PR #2. `schema-version-signal` was
integrated through PR #3 at `f2b7d23`, `snapshot-performance` through PR #4 at
`4dd10f4`, and `subscription-quota` through PR #5 at current main `7f84749`.
Their integration histories, exact result identities and retained limitations
remain in [the batch integration review](reviews/assemble.md), Beads and CEv1;
this fourth-batch preparation does not relabel or replace that evidence.
