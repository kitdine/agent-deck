---
status: active
created: 2026-10-08
updated: 2026-11-01
---

# v0.7.0 Contract — Tasks

Target: `v0.7.0`, covering scan performance, pricing-system redesign,
auxiliary-session isolation, and Codex work-signal compatibility and recovery.
Origin: the operator's 2026-10-08 four-theme proposal and subsequent explicit
request to design this version contract. The intake is recorded in
[Roadmap](../../roadmap.md#v070--scan-performance-pricing-redesign-and-work-sessions-proposed)
and Beads epic `ad-v070-iteration`.

Input is sufficient for version membership and coordination design. This draft
is ready for independent review; it does not assert that feature requirements,
implementation decomposition, Bug lanes, or product acceptance have passed.
Feature decisions remain with their owning topics. This plan grants no Git
delivery, assembly, publication, installation, or release authority.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [x] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

This version-contract topic requires only `tasks.md`, under
[Documentation Workflow](../../documentation-workflow.md#topic-structure).
Requirements, UX and architecture belong to the feature topics; a Lane A Bug
uses its own fix record. The reviewer ratifies this document set. Create
`reviews/tasks.md` only when review occurs, and later integration/closure
records only when those tasks are reviewed.

This file owns unmerged contract execution status. Beads owns dispatch and
handoff; CEv1 owns exact-content evidence. The inherited
[Project Status](../../status.md) describes integrated state and is updated
with an authorized integration or version-closure change, not this design phase.

## Working context and baseline

- Workspace ID: `v0-7-0-contract`; branch: `feature/v0-7-0-contract`.
  Resolve the path through its local workflow binding and live worktree inventory.
- Creation base: local `main`
  `32df0ae1223796f1e43bbd45a40f1efb9f7ff56e`. At entry, the locally recorded
  `origin/main` had the same identity; no fresh remote-ref query was performed.
- Published compatibility baseline: peeled `v0.6.5` commit
  `819bd4365aef59242a6a38034c379931548d4671`. The newer creation base includes
  project governance and housekeeping. It is provenance, not a permanently
  frozen target for later integration.
- New iteration feature/fix work starts from verified current `main` and targets
  `main` through authorized PRs. Explicit maintenance of an older supported
  release line follows [Branching](../../../.agent-instructions/branching.md)
  separately; this contract creates no `release/v0.7.x` branch.
- The contract branch carries planning and contract records. It is not an
  intermediate tree into which feature code is merged. Each future topic needs
  its own valid workspace entry before implementation.

Historical v0.6.0 subscription-plan carriers `ad-v070` / `ad-v070-plan` are
superseded records, not this iteration's dispatch or approval. Preserve their
history and use `ad-v070-iteration` for this plan's coordination.

## Feature registration — 2026-10-09 (not started)

The operator proposed `opencode-support` for `v0.7.0`. This is a version-local
feature registration only: feature design and development have not started.
Do not create `docs/topics/opencode-support/`, a feature branch/worktree,
requirements/UX/architecture/decomposition documents, or implementation dispatch
from this registration. The existing planning reference is `ad-opencode-support`;
this entry grants no task claim or phase authorization.

| Proposed feature | Target version | Recorded intent | State |
| --- | --- | --- | --- |
| OpenCode V2 support (`opencode-support`) | `v0.7.0` | Read project/session/provider/model/token usage and investigate ChatGPT account quota. Keep local usage, estimated reference cost and shared account quota distinct. | Registered only; no feature design or development |

API versus offline access, credentials/account isolation, token/cache semantics,
duplicate accounting, worktree attribution, affected surfaces and acceptance
remain decisions for a separately authorized feature design. No real credential
access, service change, provider/login management, extension inventory, backup,
Antigravity/Linux/multi-device support, or Git delivery is authorized here.
Project Hook/Skill adaptation is already separate tooling work, not evidence
that this proposed product feature is implemented.

The four previously reviewed selected areas below are not silently replaced or
expanded by this candidate registration. Historical review records retain their
original content identities. The registration and current contract document
passed independent [Round 3](reviews/tasks.md#round-3--2026-10-09) re-review;
the Documents review checkbox refers to this document only, not feature design
or development. Formal membership changes follow this contract's existing
authorization and review boundary.

## Selected membership

The following four areas are selected in this draft for contract review.
Membership approval does not establish feature readiness. A blocked or unfinished
member remains selected until the operator explicitly changes membership.
Material additions, removals, deferrals or independently shippable slices return
this contract to review; readiness or elapsed time cannot silently reduce scope.

| Selected area | Existing planning carrier | Owning work and current readiness |
| --- | --- | --- |
| Long scans, incremental cost and waiting behavior | `ad-bug-menubar-long-scan-timeout` | Proposed feature topic `scan-performance`; carrier is deferred/unassigned. The timeout symptom is established; the cause of the unusually expensive full read is not. Requirements, budgets, UX, architecture and decomposition need fresh design. |
| Pricing-system redesign and subscription estimates | `ad-cost-transparency` | Proposed feature topic `cost-transparency`; carrier is deferred/unassigned. Re-design the whole topic around the four minimum inputs below; its old description and decomposition are not an approved scope ceiling. |
| Auxiliary-session identity and work-view isolation | `ad-auxiliary-session-classification` | Proposed feature topic `auxiliary-session-classification`; carrier is deferred/unassigned. Default isolation with a switch is selected; classification, unknown treatment, persistence and surface contracts still belong to feature design. |
| Codex Activity/Workflow/Tooling compatibility and historical recovery | `ad-bug-codex-work-signals-missing` | Confirmed Lane A fix delivered to `main` by [PR #58](https://github.com/kitdine/agent-deck/pull/58), merge `b450704bc43807487a51a777a90dc4f8793d30dc`. Current/legacy, direct/nested, string/block content and targeted Version 7 recovery passed independent review and exact merge-state CEv1 VERIFIED 5/5. Real-history resource cost and installed native surfaces remain joint acceptance with `scan-performance`; the area remains selected. |

The proposed topic names identify intended owners, not existing worktrees,
approved documents or implementation tasks. Do not duplicate origin Bugs merely
to represent design stages, current-message support and historical replay.
The already-delivered canonical session ID/project-attribution repair remains
separate from auxiliary filtering and is not silently reopened by this version.

### Scan performance and work-signal recovery

The long-scan carrier records a historical 5,071-file scan with 111,699 imported
usage events and `total_ms=1540148`; usage/session processing ultimately completed.
Menu-bar requests timed out at about 120 seconds during running or queued scans.
Later completed rounds took about 10.3 and 9.1 seconds. These are recorded incident
observations, not measurements made by this design or approved latency budgets.
Parser-version changes and locking remain hypotheses for the expensive reread.

Before performance implementation, the feature design must establish a
reproducible baseline and numerical budgets for first import, forced/parser-version
rebuild, unchanged scan, one-file append and queued requests. Record input size
and content identity, hardware/toolchain, repeated p50/p95, CPU, peak memory,
bytes read and queue wait, separating discovery, parsing, SQL and publication.
The baseline should represent the observed scale and include larger inputs.
Use isolated fixtures or sanitized copies, never a rebuild of live user stores.

The work-signal Bug records a controlled comparison: with the same one usage
event and one edit, a current `response_item/message(role=user)` envelope produced
no classified signal and unavailable summaries; the legacy
`event_msg/user_message` envelope produced a classified activity, a two-second
first edit and one tool call. This is prior diagnostic evidence, not a repair PASS.
The repair must cover envelopes before/after turn context, append boundaries,
duplicate envelopes, injected context versus real user input, pending turns and
source ownership; retain the distinction between zero and unavailable.

Historical recovery stays within that Bug: define targeted, idempotent replay or
version invalidation for affected sources. Preserve original event identities,
token totals, session identity, credentials, providers, exclusions and logs.
Prove second-scan stability and unaffected-source skipping. Its resource cost
must fit the approved scan budgets and the performance topic's acceptance.
Do not fix an unavailable work signal by changing prices or auxiliary filters.

The bounded repair above is now integrated. Its Version 8 replay mechanism,
second-scan stability, original event/token/source/run ownership and CLI/desktop
producer agreement were verified on isolated persisted fixtures. The complete
failure, repair, archive and reopening history is retained in the
[delivered fix record](https://github.com/kitdine/agent-deck/blob/b450704bc43807487a51a777a90dc4f8793d30dc/docs/archive/fixes/codex-work-signals-missing.md).
This integration does not certify real-user history scale, numerical scan budgets
or installed native UI; those remain part of the selected combined acceptance.

### Pricing redesign: required intake and scope discovery

The owning topic must re-open requirements discovery before implementation
decomposition. The operator's four minimum requirements are:

1. Rebuild pricing around **models.dev**, validating model coverage, GPT pricing
   tiers, schema, versions/effective dates and the request evidence needed to
   select a tier. A data-source replacement alone does not satisfy acceptance.
2. Add **subscription credits estimates** for subscription analysis and monetary
   estimates. Define sources, applicable plans/models, calculation rules,
   uncertainty and money/credits relationships. Distinguish estimates from
   actual subscription debits, client-reported quota and invoices.
3. Resolve **auto-code-review** calculation. Preserve the observed/user-supplied
   name; historical `codex-auto-review` / `gpt-5.4` mappings are investigation
   inputs until evidence establishes identity and applicable rates.
4. Handle **A/B-rollout models** in pricing and presentation. Preserve observed
   model IDs; distinguish display labels, proven aliases, experimental IDs,
   supported rates and explicit unknown/fallback results.

These are a minimum intake, not a closed feature list or a fixed number of work
packages. The design must also disposition amount semantics, input/cached-input/
output rates, tier evidence, source freshness, historical repricing and snapshots,
privacy, upgrade/rollback and affected CLI/native/subscription surfaces. Legacy
`272K` and rate examples are hypotheses. LiteLLM is migration-comparison input,
not the target system. Context Efficiency is undecided and must receive an
explicit feature-design disposition before decomposition.

No effort estimate, schema, new command, hard model mapping or package count is
approved here. Inability to obtain a required authoritative rate or credits
source must produce a documented unknown/blocked acceptance item, not invented
precision or an implicit reduction of the selected pricing scope.

### Auxiliary-session isolation

The operator selected default isolation of auxiliary work from work-session
counts, project counts and durations, with a switch to inspect/include it.
Retain complete underlying usage and cost; filtering must be reversible and
must not delete raw inputs or canonical session records.

The feature design must decide classification evidence and provenance, unknown
origin handling, overrides and their persistence, switch placement, and CLI/
menu-bar/Widget/settings applicability. A directory name alone cannot establish
auxiliary identity. Classification must preserve canonical identity and remain
stable across duplicate sources, replay and late metadata. Work, auxiliary and
all views must reconcile under the reviewed unknown policy. Explain scope
differences between work KPIs and total usage/cost rather than forcing unlike
time/event measures to match. Include observer/synthetic-model duplicate-metering
checks with the pricing topic.

## Dependencies and acceptance

The roadmap's P1/W1/W2/P2/C0/A1/A2/V1 labels describe design and acceptance order;
they are not approved implementation-task IDs. This contract does not reproduce
or freeze feature decomposition.

| Dependency | Gate before dependent work |
| --- | --- |
| Scan baseline → historical replay and performance implementation | `scan-performance` independently approves representative measurements and numerical resource/latency budgets before applying performance changes. |
| Current-message compatibility → historical signal recovery | The Bug's bounded regression contract passes before acceptance of its historical recovery; final delivery covers both or receives an explicit membership/slicing decision. |
| Historical recovery → final scan acceptance | The performance topic includes recovery/reparse costs and correctness, not only steady-state no-op timing. |
| Pricing redesign → pricing implementation | Required requirements, affected UX, architecture and final decomposition pass their own reviews; all four minimum inputs and discovered additions have explicit dispositions. |
| Auxiliary identity → work-view consistency | Reviewed classification/unknown/override contracts precede aggregate filtering; signal and settled cost-scope semantics are consumed where applicable. |
| All selected deliveries → version closure | Selected members have delivered, independently reviewed results and applicable exact-target evidence, or explicit operator-approved membership dispositions. |

Each member's design owns exact files, fixtures, affected surfaces and numerical
criteria. Initial verification routing, under
[Project Rules](../../../.agent-instructions/project-rules.md#testing-and-verification--测试与验证):

- Scan/waiting and historical replay: L3 for concurrency, cancellation, persisted
  migration and bounded resource behavior, including affected race/vet/native
  checks. A healthy scan beyond the old request timeout must not be displayed
  as failed solely because the requester stopped waiting.
- Current Codex message compatibility: L2 isolated JSONL-to-SQLite regressions,
  usage/activity/session/desktop consumers and Go core regression. Historical
  replay raises the affected path to L3.
- Pricing and auxiliary classification: L2 for shared parser/output/persistence
  contracts; L3 for migration execution or privacy-sensitive inputs. Verify actual
  CLI/native behavior and complete/partial/unknown amounts at the claimed layer.
- Contract documents: L0. Release artifact and installation acceptance: L4 only
  at a separately authorized release boundary.

Use `scripts/run-go-test.sh` for Go verification. Reuse valid exact-content
evidence; the level table is not an unconditional full-suite checklist. Browser
specimens prove design states, not native runtime acceptance.

Exclude Antigravity support, Linux, automatic updating, multi-device work,
invoice reconciliation and unrelated runtime/OS recovery from this selection.
New product decisions stay in their owning topics and receive their required
reviews. v0.6.x native/performance/accessibility exceptions remain historical;
none silently waives v0.7.0 acceptance.

## Entry conditions and incremental assembly

Contract membership can be reviewed while selected features are still being
designed. After this document passes review and its applicable evidence gate,
the document task may be committed and pushed to `feature/v0-7-0-contract`
under explicit authorization. This preserves the reviewed plan on its branch;
it does not create an early contract-document PR or merge the contract into
`main`. Contract-wide PR creation and merge must wait until both `assemble`
and `v0-7-0-contract` have completed independent review and their applicable
evidence gates are VERIFIED. PR creation and merge still require explicit
authorization at that boundary.

Accept whole coherent, completed topics/fixes in incremental batches. For each:
resolve live source/target commits and trees, reviewed matrices and records,
actual delivered scope, origin-Bug disposition and exact-state evidence. Compare
overlapping contracts, persisted state, configuration, pricing/classification
and scan/replay assumptions. Classify the actual merge through Branching; no
textual conflict is not proof of compatibility.

Feature/fix PRs target `main` directly under `assemble`; they are distinct from
the final contract-wide PR. Keep this contract's batch ledger and integration
review in the contract workspace until its final delivery boundary. A member PR
may carry its own product/compatibility records and the integrated status
projection, but must not use those records to deliver the contract branch early.
Preserve ancestry and accepted limitations. No feature is first merged into the
contract branch. Any base refresh, PR, merge or delivery requires its own explicit
authority. Record partial assembly without ticking the aggregate Task complete.

## Tasks

### Recorded integration batch — 2026-10-09

- Member: complete bounded Lane A repair `fix:codex-work-signals-missing`,
  original Bug `ad-bug-codex-work-signals-missing`, PR #58.
- Source: `fix/codex-work-signals-missing` at
  `b07848fe920dcaa2d93f9d11e4e92188f6ee6261`, tree
  `bc1e7612f5c844fc9634ac1a2060831c01e38f49`.
- Actual target before merge: `main` at
  `ecd2bd1076498db042df32ab7f9d6c8f88ae1c06`, tree
  `5f05e187243e94806c30ed67b509ba84f3b53915`. Main advanced from the original
  base through PR #59's OpenCode hook integration before PR #58 landed.
- Result: merge `b450704bc43807487a51a777a90dc4f8793d30dc`, tree
  `f31e51805a2c4fe83bc921035ce1579f6afc75f6`; clean three-way merge with the
  actual target/source as its two parents, preserving ancestry.
- Interactions: the concurrent base changed hooks/runtime registration and
  governance, with no overlap in the six Go production/test blobs, archived
  carrier or three dependency blobs. The actual merged repair blobs match the
  independently reviewed source; Codex parsing, projections and ownership
  contracts are unchanged by that concurrent integration.
- Evidence: final GitHub code review found no major issues, both recorded
  threads are resolved, PR/push CI passed, and immutable main-merge ContentState
  `fix:codex-work-signals-missing:commit:b450704bc43807487a51a777a90dc4f8793d30dc`
  is VERIFIED 5/5 through explicit impact assessment and target-bound reuse.
- Progress: this is partial assembly. The other three selected areas and the
  joint scale/native/combined acceptance remain open. Both aggregate Task
  checkboxes below remain unchecked; no contract-wide PR or version closure is
  implied. The contract checkout remains on its planning branch.

| Task | Dev | Review |
| --- | --- | --- |
| 1. `assemble` | [ ] | [ ] |
| 2. `v0-7-0-contract` | [ ] | [ ] |

These are contract-topic tasks for review. Create their implementation dispatch
only after this matrix is approved and applicable evidence/checkpoint conditions
are satisfied. The present document task is not either implementation task.

### Pending scan-performance batch — 2026-10-31

Implementation has started under dispatch `ad-v0-7-0-contract-assemble` and
CEv1 WorkUnit `v0-7-0-contract:assemble`. This is batch preparation, not an
integration result or an independent review. Both aggregate checkboxes remain
unchecked. The recorded Codex batch above remains integrated.

- Source: `feature/scan-performance` at
  `3e07c892f34d775e60919774f49ae7c85afb755f`, tree
  `f0121cb98e5f44a8025409fc9d2815cf134940b6`; the remote source ref matches.
  All eight source task Dev/Review rows are checked. The delivered topic gate
  for `scan-perf-delivered-3e07c89-state` was queried on 2026-10-31 and is
  VERIFIED for all-task-gates, cross-task-acceptance and user-performance-acceptance,
  with no missing, invalidated or unresolved evidence. Source document wording
  that still says awaiting delivery is historical; the committed-target gate
  records this signed topic-branch delivery, not main integration.
- Intended target: remote `main` at
  `d78c34583bf34175512c31765d4d4358b8049664`. Local `main` remains at
  `b450704bc43807487a51a777a90dc4f8793d30dc`; it must not substitute for the
  remote target. `git ls-remote` confirmed both refs; GitHub compare confirmed
  common base `b450704bc43807487a51a777a90dc4f8793d30dc`, with 41 source-side
  commits and two target-side commits.
- Target-only changes are MIT `LICENSE` and license notices in `README.md`
  and `README_zh.md` (PR #60). The source changes none of these three paths.
  This rules out same-file overlap for the current target delta; it does not
  certify a merge result. The expected operation is a three-way merge, with
  actual conflict status, result commit/tree and independent integration review
  still pending. No source code is merged into the contract branch.
- Reuse the exact delivered source evidence for unchanged product behavior.
  Preserve its accepted P04/RSS tradeoffs and exclusions: installed Widget/manual
  UI and L4 release acceptance are not established by the source acceptance.
  The isolated acceptance preference restoration limitation remains recorded in
  the source Task 8 record. Do not repeat performance sampling merely for assembly.
- The operator authorized member PR creation on 2026-10-31, explicitly excluding
  merge. [PR #61](https://github.com/kitdine/agent-deck/pull/61) is open from
  `feature/scan-performance` to `main`; its returned head/base identities match
  the source/target above. No merge has occurred. Before integration, obtain
  explicit merge authorization, revalidate identities, and inspect the result.
  Preserve the target's license files, the complete reviewed source, and existing
  dirty work in other workspaces. Reconcile integrated status at the authorized
  integration boundary; record the independent result review in
  `reviews/assemble.md` when that review occurs. PR creation adds no merge,
  commit, push, release or installation authority.
- Remaining membership: pricing redesign is still deferred; auxiliary-session
  isolation has reviewed design documents but all nine implementation rows remain
  unchecked. Their delivery and final combined acceptance remain required.
  OpenCode remains the registration described above, without a membership change.

The reviewed contract-document prerequisite was independently queried at delivered
state `c51da1d19b814ebfe7588e30ceb5ba68b3665981eedf85f49593cc6986cd5fc9`:
VERIFIED 3/3. That historical document evidence is not relabeled for this new
batch record. The aggregate assemble gate stays open until its full criteria are
met; readiness of one member cannot close the task or version contract.

### Scan-performance batch review — 2026-11-01

The preparation identities above remain historical. The current source is
`d0bd36ea63bd65da21d5a3c6d18bca351cdc8e70` (tree
`7d4b1e210b7b9c2e322c2b54b1b8bffd51161cd6`); target remains
`d78c34583bf34175512c31765d4d4358b8049664`. GitHub's pre-merge candidate is
`8be912c12e1c20ab4bb2b907dda7ac6ceef271ee`, tree
`b4211bf442304e85c2dfbd4bdc480bff1ca77150`, with those exact target/source
parents. [Assembly Round 1](reviews/assemble.md#round-1--2026-11-01) passes
this batch's integration compatibility review only. Both aggregate matrix
checkboxes remain unchecked; the other members and combined acceptance remain.

The operator explicitly stopped further PR Code Review/P2 repair iterations on
2026-11-01, then requested continuation of assembly review. This removes that
additional review-loop prerequisite; it does not manufacture source acceptance,
waive selected membership or authorize merge. The current eight CI checks pass.
The older no-major-issues receipt covers `fa8fe4e3`, not this source's later
publication-retry repair. That distinction remains in the batch review.

The aggregate assemble evidence gate remains NOT_VERIFIED. PR #61 is open and
unmerged; merging requires explicit authorization. After an authorized merge,
verify the actual resulting identity, reconcile the batch's integration evidence
and integrated status, and continue the remaining selected membership. This
batch review does not close assemble or authorize a contract-wide PR.

### Scan-performance batch merged — 2026-11-01

Under explicit operator authorization, PR #61 merged into remote `main` at
`6fff208ac852b191c0a62df1f5be34c937601cfe` on 2026-11-01 15:25:39 UTC.
Its parents are target `d78c34583bf34175512c31765d4d4358b8049664` and source
`d0bd36ea63bd65da21d5a3c6d18bca351cdc8e70`; tree
`b4211bf442304e85c2dfbd4bdc480bff1ca77150` exactly matches the reviewed
candidate. The pre-merge OPEN statements above are historical.

Post-merge reconciliation is recorded in [Assembly Round 1](reviews/assemble.md#round-1--2026-11-01).
GitHub verifies the server-created merge's PGP signature as valid. The operator
explicitly rejected the agent's SSH-only blocking interpretation for this GitHub
PR merge and instructed correction; no signature exception or rewrite is needed.
Post-merge CI run `41321169520` was running at initial reconciliation;
on 2026-11-03 all four jobs were verified successful and exact-merge evidence
was appended. The erroneous signature blocker is superseded by that scope
correction and the verified GitHub signature. The exact-merge integration gate
is VERIFIED 4/4, with no missing criteria or unresolved candidate impacts; no
aggregate task or source acceptance is newly certified. Local record updates
are not committed or pushed by the merge authorization. No history is rewritten.

### Pending auxiliary-session-classification batch — 2026-11-13

Batch preparation under dispatch `ad-v0-7-0-contract-assemble` (claude-code),
not an integration result or independent review. Both aggregate checkboxes
remain unchecked; the Codex and scan-performance batches above remain integrated.

- Source: `feature/auxiliary-session-classification` at
  `f39f86aa1212de2c674667ee343f09c50dc11236`, tree
  `b1c9df4e54d832aa60fdc4547fe2d69a489935ae`; the remote ref matches. All nine
  source task Dev/Review rows and all seven document Review rows are checked.
  Task 9 passed its Round 4 re-review; its committed tree equals the reviewed
  candidate fingerprint `024b6820…`.
- Intended target: remote `main` at `90535085723f013cb8b24d83f83015e7d60d53b0`,
  tree `5b9e0c027f1324c25f571ce32bf1d72bd4d40ba6`. Merge base
  `660c81586f4c13e04e66dbc05cde5331bf09acec`; 29 source-side and two
  target-side commits.
- Target-side changes since the base are governance only: `AGENTS.md`,
  four `.agent-instructions/` files, `docs/README.md`,
  `docs/documentation-workflow.md`, `docs/roadmap.md` and
  `scripts/hooks/beads-consistency.py` (PR #62 status, PR #63 issue-intake
  ownership). The only same-file overlap is `docs/roadmap.md`: the target
  rewrites the header and adds a Backlog preface; the source adds one Backlog
  entry dated 2026-11-03 for `ad-codex-memory-source-linkage`. `git merge-tree`
  produces tree `c4729f6bb74355b1fd5c21b253c81ee857f1135b` without conflicts.
  The entry predates the 2026-11-07 ownership change and has a Beads carrier,
  so it fits the preface's retained-intake rule. A clean textual merge is not
  proof of compatibility; the integration review uses the actual result.
- Member closure is incomplete, so this batch is not ready for integration
  review:
  - Stable contracts are not reconciled. Source `docs/specs/cli-design.md` and
    `cli-manual.md` contain none of the topic's command or output contracts
    (`session origin-source`, session scope views and flags, origin reason
    codes). The topic's Task 9 assigns this reconciliation to topic closure.
  - No topic-level CEv1 WorkUnit `auxiliary-session-classification` exists;
    WorkUnits `…:durability` and `…:integrated-acceptance` have not reached
    their committed lifecycle state. The scan-performance batch had a VERIFIED
    delivered topic gate before its PR.
  - Operator decision 2026-11-13: the member closes in its own topic. The
    specs reconciliation and topic gate are completed and reviewed in the
    `auxiliary-session-classification` workspace and delivered on its feature
    branch, as scan-performance did; they are not moved to contract Task 2.
    This batch resumes from the resulting source commit, whose identities
    replace the source line above.
- Remaining membership: pricing redesign remains deferred; OpenCode remains a
  registration only. Final combined acceptance remains required.

No PR, merge, commit or push has been performed for this batch.

### Auxiliary-session-classification batch resumed — 2026-11-14

Batch preparation resumed under dispatch `ad-v0-7-0-contract-assemble`
(claude-code) and CEv1 WorkUnit `v0-7-0-contract:assemble`. This supersedes the
source identities and the member-closure finding of the pending record above;
it is not an integration result or an independent review. Both aggregate
checkboxes remain unchecked.

- Source: `feature/auxiliary-session-classification` at
  `855a811ad7b93298236c212ebd13ed4fcf2d58dc`, tree
  `dd2da688951d969aef165444bb767aa696b38e95`; `git ls-remote` returns the same
  commit. All seven document rows and all nine task Dev/Review rows are checked.
  Since the pending record the source gained five commits: the animation
  diagnostic `6a3532b`, the documentation link-checker fix `da0a901`, the
  stable-spec reconciliation `298bd6b`, the animation test repair `74964a2`
  and the Round 6 review record `855a811`. CI run 43558290460 on `855a811`
  succeeded for classify, documentation, desktop and verify.
- Member closure, resolved in the source topic as the operator decided:
  - Stable contracts: `docs/specs/cli-design.md` (version 34) and
    `cli-manual.md` now carry the topic's command, scope, reason-code and DTO
    contracts; the source's Round 5 review passed that reconciliation.
  - Review: source Task 9 Round 6 passed at `74964a2a0a8a4bade0ba6c1cea25e8d0f4d6667a`.
    `855a811` adds only that review record and the topic status over `74964a2`.
  - Evidence: a direct graph read on 2026-11-14 found passing observations
    bound to ContentState
    `auxiliary-session-classification:integrated-acceptance:commit-74964a2a0a8a4bad`
    for all nine `integrated-acceptance` criteria and both topic criteria
    (`cross-task-acceptance`, `stable-contract-reconciliation`). The source
    record reports the Task gate VERIFIED 9/9 and the Topic gate VERIFIED 2/2
    for that target with no invalidated or unresolved evidence; this read did
    not re-derive the invalidation analysis. The topic WorkUnit now exists.
- Intended target: remote `main`, unchanged at
  `90535085723f013cb8b24d83f83015e7d60d53b0`, tree
  `5b9e0c027f1324c25f571ce32bf1d72bd4d40ba6`. Merge base
  `660c81586f4c13e04e66dbc05cde5331bf09acec`; 34 source-side and two
  target-side commits. Neither ref is an ancestor of the other, so the
  operation is a three-way merge.
- Candidate result: `git merge-tree --write-tree` yields tree
  `29940cefbc6274ac4e014f04e72c24d24ba7ef26` without textual conflicts. The
  only same-file overlap is still `docs/roadmap.md`, as analysed above; the
  result keeps the target's header and Backlog preface and the source's
  `ad-codex-memory-source-linkage` entry. The target-side delta remains the nine
  governance paths listed above; the source changes 223 paths.
- Interactions for the integration review, beyond the roadmap overlap:
  the source changes `scripts/check-docs.py` and its contract test while the
  target changes `docs/documentation-workflow.md`, `docs/README.md` and
  `scripts/hooks/beads-consistency.py`. On an export of the candidate tree,
  `python3 scripts/check-docs.py`, `make check-ci-docs-tools` (93 tests) and
  `make check-opencode-hooks` pass. This is L0 for records and tooling; no
  product check was rerun, because the target delta touches no product path.
  The source's Go, native and performance evidence is reused for unchanged
  product behavior.
- Accepted limitations carried from the source: the Settings page-switch test
  asserts that an animated resize is requested, not that intermediate frames
  are drawn, because hosted macOS 15 runners have Reduce Motion on; Round 6
  reuses the earlier native intermediate-height observation rather than a new
  CI frame observation. One local `make test-macos-app` run set on a slow host
  left `SwitchFlowTests.testConfirmationUsesExactlyTheResolvedOption` failing
  while CI passed it; it was not diagnosed. Keyboard and accessibility
  acceptance stay excluded by the operator's earlier decision.
- Residual record state, not changed here: the CEv1 lifecycle attribute of
  WorkUnits `…:durability` and the topic unit still reads `in_progress`, and
  the source topic is not archived.
- Not performed: no PR exists for the source branch, and no merge, commit or
  push was made for this batch. PR creation and merge each need their own
  explicit authorization. Following the scan-performance batch, the batch's
  integration WorkUnit and its ancestry, signature, reviewed-result and
  post-merge CI evidence are recorded at the actual merge, and the integrated
  `docs/status.md` projection lands at that boundary.
- Remaining membership: pricing redesign remains deferred; OpenCode remains a
  registration only. Final combined acceptance remains required.

### Auxiliary-session-classification batch review — 2026-11-14

Independent integration review Round 2 PASS for source `855a811` and target
`9053508`, candidate tree `29940cefbc6274ac4e014f04e72c24d24ba7ef26`.
The full report, evidence reuse boundaries and remaining prerequisites are in
[Assembly Round 2](reviews/assemble.md#round-2--2026-11-14).
This is a batch compatibility verdict, not aggregate completion: both assemble
checkboxes remain unchecked and its required gate remains NOT_VERIFIED.
The member PR and actual merge/result evidence are still pending; pricing
membership/delivery and final combined acceptance remain open. The contract
workspace binding is `v0-7-0-contract`. No commit, push, PR, merge or release was
performed by this review; the inherited batch preparation edits are preserved.

### Auxiliary-session-classification member PR — 2026-11-14

The operator separately authorized creation of [PR #64](https://github.com/kitdine/agent-deck/pull/64),
`feature/auxiliary-session-classification` → `main`, without merging.
GitHub confirmed OPEN/unmerged at source `855a811ad7b93298236c212ebd13ed4fcf2d58dc`
and base `90535085723f013cb8b24d83f83015e7d60d53b0`, matching Round 2.
The PR preserves the source acceptance limits and distinguishes prior product CI
from the latest documents-only run. PR checks and actual integration remain pending;
aggregate assemble stays in_review, with both task checkboxes unchecked.
This ledger update is local and uncommitted; no merge, release or contract-wide PR
was performed or authorized.

### Auxiliary-session-classification PR review and pre-merge batch review — 2026-11-14

Following PR #64 creation, two authorized review-repair commits landed on
`feature/auxiliary-session-classification` to resolve three P2 `@codex review`
findings: `a2b01a3cbab249d88cbca57f24c04ea9fa279cf1` (preserving complete
coverage across helper-classified desktop summary rows in
`internal/desktop/session_scope.go` and preserving non-interactive exit status
when `sessions --fix` leaves unrepairable items in `cmd/agentdeck/main.go`, with
regression tests) and `0feb0c6d8965b6037de4c7e04bee839b96039dd0` (defaulting
blank `session_origin` when loading incremental snapshot rows from pre-v13
databases in `internal/scanruntime/serving.go`, with regression test).
All three review threads (`PRRT_kwDOTe7lus63ZnYu`, `PRRT_kwDOTe7lus63ZnYv`,
`PRRT_kwDOTe7lus63awJD`) are resolved; full product CI runs `43572725411`
(`push`) and `43572728514` (`pull_request`) passed all 8 jobs (`classify`,
`documentation`, `verify`, `desktop`), and Round 2 `@codex review`
(`IC_kwDOTe7lus8AAAABmcUAqA`) completed on `0feb0c6d89` with zero findings.

Independent integration review Round 3 PASS for updated source
`0feb0c6d8965b6037de4c7e04bee839b96039dd0` (tree
`5eb8c6f0a737935b24cebb4016a44749ff7d67e4`), target `main`
`90535085723f013cb8b24d83f83015e7d60d53b0` (tree
`5b9e0c027f1324c25f571ce32bf1d72bd4d40ba6`), merge base
`660c81586f4c13e04e66dbc05cde5331bf09acec` (36 source commits, 2 target commits),
and GitHub's clean pre-merge candidate commit
`4d8c54dd0125481f2d5dbbb514973e68f6195cba` (tree
`8bc3398976468058299173b93b363155f0ce6d8c`, matching local
`git merge-tree --write-tree`). The full report, evidence reuse boundaries,
source CEv1 state (`74964a2a0a8a4bade0ba6c1cea25e8d0f4d6667a` plus PR repair
verification on `0feb0c6d8965b6037de4c7e04bee839b96039dd0`), and remaining
prerequisites are in [Assembly Round 3](reviews/assemble.md#round-3--2026-11-14).
This remains a pre-merge batch compatibility verdict, not aggregate completion:
both assemble checkboxes stay unchecked and its required gate stays
`NOT_VERIFIED` until explicit merge authorization, actual merge/post-merge CI
and signature verification, remaining selected members (`cost-transparency`),
and final combined acceptance complete.

### 1. `assemble`

**Prerequisites:** reviewed contract membership with VERIFIED document evidence,
recorded by an authorized commit on the contract branch, and at least one whole
selected member with independently reviewed delivery and applicable evidence.
An earlier contract PR or merge into `main` is not a prerequisite.
Remaining selected members may still be unfinished; partial batches are allowed.

**Result:** all selected members are integrated into `main` with verified
interactions and preserved ancestry, or explicitly dispositioned by an operator
membership decision. Mere diagnosis, plan delivery or a Beads closure is not a
delivered fix.

**Files/boundary:** selected members' reviewed files, the integrated primary
contract and affected manual, this `tasks.md`, `reviews/assemble.md`, and the
integrated `docs/status.md` projection when that batch lands. Do not create a
second global progress log or include unrelated feature work.

For each batch, record immutable source/target/result identities, actual merge
class, interactions/conflicts, review and CEv1 references, accepted limitations
and remaining members. Integration review uses the actual result; source review
does not substitute for it. Resolve member, integration and aggregate gates at
their real boundaries, retaining missing/failed evidence as such.

**Verification:** L0 for records; select product L2/L3 checks for changed
interactions and uncovered criteria, reusing unaffected valid evidence. Final
combined acceptance covers representative scale with signal replay, auxiliary
filters and pricing together, full/incremental agreement, no-op/append bounds,
queued/cancelled/failed scans, restart safety and preserved event/token identity.
Numerical budgets come from the reviewed owning feature, not this draft.

### 2. `v0-7-0-contract`

**Depends on:** complete selected assembly/dispositions and its independent
review and VERIFIED evidence, with member integrations delivered to `main` and
the aggregate assembly records committed on the contract branch under explicit
authorization. This does not require an earlier contract-wide PR or merge.

**Result:** integrated behavior, version-level documentation and compatibility
claims agree; the contract completion gate is resolved for the actual content.

**Files/boundary:** `docs/specs/cli-design.md`, affected
`docs/specs/cli-manual.md`, this topic's `tasks.md` and
`reviews/v0-7-0-contract.md`, integrated `docs/status.md`, necessary
`docs/README.md` / archive-index pointers, and member-topic retirement metadata
only at the authorized lifecycle boundary.

Reconcile the then-current primary contract and add one consistent version-history
entry/frontmatter revision. State new fields/defaults, command/output compatibility,
unknown/estimated amounts and credits, model/price-source identity, historical
replay and normal-upgrade behavior. Preserve databases, event/token identity,
providers, credentials, exclusions, source logs and user settings. Identify any
intentional changed default or recomputed derived value explicitly.

Verify protected-data and normal-upgrade claims on isolated copies and the
authorized native acceptance route. Missing supported fields remain unavailable
with a reason. Do not equate credits estimates with actual quota, or claim invoice
accuracy. Preserve independent member acceptance and all current residual risks.

**Verification:** L0 contract/link/version consistency plus exact-state contract
completion evidence, reusing valid feature/integration checks. Select additional
product verification only for concrete changed assumptions or uncovered criteria.
Release readiness requires separately authorized exact-SHA L4 preflight, artifact
identity, distribution/normal-upgrade and operator acceptance. Contract PASS does
not authorize or replace tagging, signing/notarization, publication or installation.

## Design handoff

Only `tasks.md` is authored by this phase. Its Draft is ready and Review remains
unchecked at the Design handoff; both implementation tasks are unstarted. The document carrier is
`ad-v0-7-0-contract-doc-tasks-design`; other selected carriers retain their own
status, ownership and histories. No feature implementation dispatch is created.

Review this document against the current roadmap/intake, owning feature boundaries,
primary product contract, Branching and Documentation Workflow. Verify the scoped
document set and L0 checks. The reviewed-document WorkUnit is logically
`v0-7-0-contract:tasks.md`; resolve its canonical CEv1 ID and required criteria at
review entry. Bind the final candidate with the established document recipe:
SHA-256 of `head=<HEAD-SHA>;document=<Git-blob-ID>`. Include any newly required
dependencies in its declared state; this document has no rendered specimen.
Draft readiness is not a review PASS, a verified document gate, topic completion
or release acceptance.

## Review status

The historical [Round 1](reviews/tasks.md#round-1--2026-10-08) and document
delivery at `a170d5547a650fb7fd05231cfbd01fd15ca40906` remain recorded. The
operator's 2026-10-08 correction of the contract PR/merge boundary passed
independent [Round 2](reviews/tasks.md#round-2--2026-10-09) re-review.
`assemble` now records the Codex repair as its first partial integration batch;
`v0-7-0-contract` closure is unstarted. Current evidence and finding
disposition belong to the review record. Document commits/pushes may precede
assembly; the contract-wide PR/merge waits for both implementation tasks to
pass review and their applicable evidence gates.

### Integration status synchronization — 2026-10-09

This update records the delivered Codex member and partial assembly facts only.
It preserves the reviewed membership, acceptance boundaries, document set and
contract-wide PR/merge policy; historical Round 2 evidence retains its original
content identity and the delivered document task stays closed. Exact candidate
evidence reuses the unchanged scope and document-set decisions through an
explicit impact assessment, with fresh scoped L0 checks. The operator separately
authorized this document's commit and push on 2026-10-09. Aggregate assembly and
contract closure remain open.
