---
status: active
created: 2026-10-08
updated: 2026-10-09
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
