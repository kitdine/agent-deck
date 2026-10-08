---
status: active
created: 2026-08-25
updated: 2026-10-08
---

# AgentDeck Roadmap and Backlog

This file is the authority for later version direction, unapproved planning
intake, and withdrawn candidates. Active version membership is owned by the
applicable `vX-Y-Z-contract` topic and projected in `status.md`. Moving an item
between these sections is a planning decision, not implementation authorization.
## v0.6.0 — Trusted usage and subscription visibility

Stable `v0.6.0` was published on 2026-09-30 from `a5e969d7`. The five feature
areas below are delivered membership, retained for provenance. Release
was closed by the operator with `ad-bug-cask-widget-registration-missing`
transferred to the next release;
current evidence and channel status belong to [Project Status](status.md#release).

Re-planned by the operator on 2026-09-05 after stable v0.5.0. This decision
reuses the version number with a new scope; the old cancelled cost-truthfulness
epic remains historical. The iteration entry is `ad-v060-iteration`.

The five areas below were selected on 2026-09-05, subsequently designed,
reviewed and assembled, and shipped in v0.6.0. Their delivered membership and
exceptions are preserved in the archived version contract; this section no
longer dispatches design or assembly work. Cost and price transparency was
removed on 2026-09-26 and is now a proposed v0.7.0 member below.

| Feature | Planning carrier | Reason and boundary |
| --- | --- | --- |
| Schema compatibility and Hook failure visibility | `ad-schema-compatibility` | Prevent permanent schema refusal from silently losing Hook observations; reuse the existing schema-version-signal document tasks. |
| Menu-bar and Widget refresh | `ad-desktop-refresh` | Make freshness and errors truthful; target approximately 1 minute for the menu bar and 3–5 minutes for widgets with change-driven updates. |
| Snapshot performance | `ad-snapshot-performance` | Reduce repeated aggregation before increasing refresh frequency; verify invalidation and output equivalence. |
| Actionable health recovery | `ad-health-recovery` | Resolve lock recovery guidance and stale extension inventory with cause-specific actions; retain the two origin bugs. |
| Codex/Claude subscription accounts, quota, reset and alerts | `ad-subscription-quota` | Make subscription capacity actionable, including Codex reset-count information, reset times, and opt-in reminders. |

The delivered subscription contract distinguishes supported reset allowances,
natural quota-window resets and locally observed resets. Unsupported fields
remain unavailable with a reason. Account isolation, polling budgets and
notification deduplication are guaranteed by the living CLI design/manual;
remaining defects are in the v0.6.5 inventory. No reset action, account login
switching, automatic updater or plaintext credential storage was selected.

Cost transparency and structured session search are excluded from v0.6.0 and
remain unassigned candidates. Credits conversion, Context Efficiency, Linux,
a public adapter protocol, full extension observability and multi-device
aggregation remain later candidates. Existing withdrawal decisions remain
effective.

The old v0.7.0 subscription epic/plan/Gate are superseded by this selection;
their obsolete coordination records are closed and retained as history. The remaining old version rows are
directional candidates rather than a required release order.

## Roadmap

The v0.6.0 selection above records the published scope. The operator requested
the single v0.6.5 defect-repair iteration on 2026-09-30. Scope remains bounded
to compatible repairs; any proposed new product contract requires explicit
disposition against Version Number Semantics. Later rows retain the 2026-08-13
planning references for traceability; their version numbers and ordering are
provisional and can be reconsidered. Each feature needs a bounded topic before
development starts.

| Version | Theme | Scope |
| --- | --- | --- |
| `v0.6.0` | Trusted usage and subscription visibility | The five feature areas selected above; subscription quota includes Codex reset-count information. |
| `v0.6.5` | Concentrated defect repair | Nineteen unique Bugs, prioritised P1/P2/P3; one issue/PR per repair, local independent and GitHub Codex review, then one aggregate release. Feature candidates remain separate. |
| `v0.7.0` (proposed) | Scan performance, pricing redesign and work sessions | Long-scan performance, models.dev pricing and subscription credits estimates, auxiliary-session isolation, and Codex work-signal repair; proposed on 2026-10-08, with pricing scope open for redesign. |
| `v0.8.0` | Boundary consolidation and Linux | Versioned client adapter contract, Linux machine identity, de-darwin PTY tests, Linux CI matrix and release artifacts. |
| `v0.9.0` | Observability completion | Extension enabled state, cross-client duplication and drift, source authenticity, structured session search filters, wrapper health probing, richer desktop session window. |
| `v1.0.0` | Multi-device and trust | Device dimension, backup merge import, read-only aggregation views, CLI archive signing and notarization. |

Direction decisions that shape this roadmap:

- Client breadth stays at Codex and Claude. An explicit versioned client adapter
  contract is extracted so a later out-of-process plugin model can add clients
  externally; no third client is added in-tree.
- Each machine remains its own authoritative store. Cross-device support is
  read-only aggregation, never bidirectional synchronization.
- Proactive behavior — alerting and scheduled evaluation — is hosted by the
  menu-bar app. No daemon, LaunchAgent, or network listener is introduced, and
  alert rules stay in Go.
- The CLI targets macOS and Linux; the GUI stays macOS-only. Capability layering
  is explicit rather than accidental.
- Cost has three coexisting dimensions. Third-party API with a multiplier and
  official API are real spend computed locally; official subscription is quota,
  requires network access, and is therefore handled inside the app. Equivalent
  API cost is retained as a reference baseline for every mode.
- Extension work is bounded to cross-client observability. Each client already
  owns its own management surface; no tool reports the cross-client view.

## v0.6.5 — Concentrated defect repair

The operator selected **v0.6.5** on 2026-09-30: address as many verified defects
as practical in one iteration, with one issue/branch/PR per repair and one
aggregate version acceptance/publication. The order below is execution order,
not a proposal to publish separate repair releases. No bug implementation is
started by this planning/coordination change.

The selected scope is delivered through the archived version contract. On
2026-10-08 the operator accepted RC2 and selected its exact source for stable
`v0.6.5` promotion. Current publication and distribution evidence belongs to
[Project Status](status.md#release). The inventory below preserves intake and
delivery provenance; deferred performance and feature candidates remain later
work.

Restore existing contracts and preserve data, schema and external-client
configuration ownership. A proposed new command, typed code, persisted format,
output semantics, default filtering rule or changed user-visible number/count
for unchanged input requires an explicit scope/compatibility disposition;
do not silently expand the defects release. Auxiliary-session classification
has therefore been split out as a feature rather than attached to the metadata
repair. Version-number semantics remain in the stable CLI design contract.

The operator's 2026-10-07 B decision retains v0.6.5 and permits only PR24's
already-delivered `session_sources.parser_context` column and additive migration
as a narrow schema exception. The [primary version contract](specs/cli-design.md#version-number-semantics)
owns its exact scope and mandatory compatibility checks. This does not select
another schema change, feature, product repair or release operation.

### Reconciliation actually performed

The normal work queue changed from 40 unresolved records to **28: 19 unique
bugs, 3 feature candidates, and 6 retained version-planning records**. Sixteen
physical records were closed: thirteen previously listed records plus three
obsolete authorization Gates that were visible through dependency inspection.
A distinct feature was added when the combined session issue was split. Closed
history is retained; old designs are not relabelled implemented.

| Closed records | Count | Evidence/disposition |
| --- | --- | --- |
| `ad-sp-shared-ingestion`, `ad-sp-generation-checkpoints`, `ad-sp-derived-snapshot-cache`, `ad-sp-unchanged-refresh`, `ad-sp-acceptance-and-reconciliation` | 5 | Superseded by the delivered three-task snapshot-performance replan; current matrix and Historical task disposition identify the replacement owners. |
| `ad-switcheff-attribution-dev`, `ad-switcheff-no-route-dev` | 2 | Withdrawn old definitions; the archived switch-effectiveness-boundary topic records the completed replacement contracts. |
| `ad-switcheff-attribution-gate`, `ad-switcheff-no-route-gate`, `ad-whx` | 3 | Cancelled obsolete approval requests before closing blocked old tasks. Cancellation does not authorize their old Design/Development scope. |
| `ad-v060`, `ad-v070-plan`, `ad-v070` | 3 | Cancelled cost-truthfulness version and superseded subscription version; distinct from the already released/closed `ad-v060-iteration`. |
| `ad-bug-prototype-probe-pending-banner` | 1 | Duplicate of the retained `ad-bug-prototype-probe-pending-assertion`; both name the same current probe assertion. |
| `ad-progression-stage7-no-review-or-evidence-step`, `ad-lane-a-gate-vs-direct-instruction` | 2 | Current stage 7a and real-stage authorization rules already address these historical rule gaps; source provenance includes `736894f5` and current AGENTS.md. |

### Feature candidates and actual stages

The five v0.6.0 areas are delivered: schema-version-signal, snapshot-performance,
desktop-refresh, health-recovery and subscription-quota. Their planning carriers
and implementation tasks are closed. Native/performance acceptance exceptions
remain their recorded exceptions, not new unfinished implementation tasks.

| Retained feature | Stage and implemented baseline | Current disposition |
| --- | --- | --- |
| `ad-cost-transparency` | Planning intake only; no approved topic/decomposition. Existing price catalogs and incomplete-cost presentation do not fulfill its full cost-basis/source/tier transparency goal. | Valid future feature, deferred and unversioned; older model/rate/context examples must be revalidated during design. Not a v0.6.5 repair. |
| `ad-antigravity-support` | Planning intake only; no full provider/session/usage/extension/backup integration. `e84ce1f` delivered workflow actor attribution, not Antigravity product support. | Deferred; the old in-tree third-client direction conflicts with current client breadth. Re-scope through the external adapter direction in v0.8.0 before design; do not call it implemented. |
| `ad-auxiliary-session-classification` | New planning carrier split from the combined session Bug. No approved default filtering, counting, accounting or UX contract exists. | Deferred, unversioned; retain auxiliary usage and source logs. The existing metadata Bug owns only canonical ID/cwd/fallback/index repair. |

Future version plans remain: `ad-v080`/`ad-v080-plan`,
`ad-v090`/`ad-v090-plan`, and `ad-v100`/`ad-v100-plan`. Keep their existing
design prerequisites; the operator's immediate v0.6.5 choice does not start
these features or renumber the future directions.

### Unique Bug inventory and applied priority

At the 2026-09-30 intake all nineteen retained Bugs were labelled `v0.6.5`,
open and unclaimed; that historical state is not the current repair queue. The
2026-10-06 [contract ledger](archive/topics/v0-6-5-contract/tasks.md#current-member-dispositions)
records eighteen delivered repairs and one bounded administrative not-a-defect,
with all nineteen Beads carriers closed. Aggregate Task2/3 evidence and release
readiness remain separate. Reproduce a new current signal before reopening or
implementing another repair.

P1 addresses broken installed functionality, incorrect project/data ownership,
cross-account state, external configuration consistency and scan failure. P2
addresses truthful presentation, validation and workflow integrity. P3 is a
lower-impact or not-yet-confirmed case. Diagnosis/disposition can close a false
or already-fixed finding with evidence; silence or inability to reproduce once
does not prove it resolved.

| Priority | Beads Bug | Scoped next action |
| --- | --- | --- |
| P1 | `ad-bug-cask-widget-registration-missing` | Normal Cask install/upgrade/RC-to-stable registration lifecycle and an installed-path regression; do not pre-register manually before testing. |
| P1 | `ad-bug-session-project-attribution-observer-noise` | Canonical Codex session ID/cwd, fallback precedence and old-index reparse. Auxiliary filtering moved to the separate feature. |
| P1 | `ad-bug-quota-portable-restore-account-bound-state` | Cross-account portable restore fixture; clear only account-bound quota state and retain user/provider data. |
| P1 | `ad-bug-quota-statusline-cross-installation-conflict` | Two isolated state roots; preserve managed-route ownership and avoid recursive prior-command chaining. |
| P1 | `ad-bug-quota-reading-off-route-not-restored-on-save-failure` | Restore the external route after a precommit settings-save failure; retain postcommit error semantics. |
| P1 | `ad-bug-usage-scan-busy-snapshot` | Deterministic external-write reproducer and a scoped publication transaction fix, not a global locking redesign. |
| P2 | `ad-bug-widget-publisher-unavailable-silent` | Unavailable-container producer failure must reach the existing publication/health presentation. |
| P2 | `ad-bug-quota-parse-failure-source-misattribution` | Failed-attempt source metadata through canonical wire and native consumer. |
| P2 | `ad-bug-widget-large-quota-header-client-label` | Reachable quota intent and large-family header matching the clients actually presented. |
| P2 | `ad-bug-widget-cost-incomplete-contrast` | Current prototype contrast reproducer; separately verify native behavior before a native claim. |
| P2 | `ad-bug-prototype-probe-pending-assertion` | Canonical issue for the duplicate pair; detect both normal absence and pending presence. |
| P2 | `ad-bug-prototype-statchip-fidelity` | Match native shrinking and hour-window semantics in the authoritative specimen. |
| P2 | `ad-bug-arch-sessions-unavailable-not-a-consequence` | Correct the active contract's independent session-store condition without rewriting review history. |
| P2 | `ad-bug-hook-gate-ignores-content-state` | Current-source Hook fixture for stale review evidence and changed subjects. |
| P2 | `ad-bug-hook-check3-directory-entries` | Untracked-directory fixture; verify relevant Markdown becomes visible. |
| P2 | `ad-bug-hook-check5-lifecycle-word-match` | Distinguish actual obsolete-state contracts from quoted or ordinary-language uses. |
| P2 | `ad-bug-lsregister-collision-no-recovery-path` | Settle a bounded recovery-guidance repair first; any new doctor API/code is a separately assessed product decision. Distinct from missing installer registration. |
| P3 | `ad-bug-footer-routes-narrow-truncation` | Native 280 pt reproduction and existing-policy check before deciding a fix. |
| P3 | `ad-bug-disttest-fixture-registrations-linger` | Bounded fixture registration cleanup; isolated fixture identifiers do not prove production collisions. |

### One issue, one PR: bounded repair workflow

Current-iteration feature and fix PRs integrate into verified `main` first.
Release publication and explicitly scoped supported-release maintenance are
separately authorized operations. Maintenance may start on the oldest affected
supported release line and then propagate to main with preserved ancestry and
integration review. The superseded 2026-09-30 release-first decision established
`release/v0.6.x` from v0.6.0 `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52`;
completed patch-line deliveries remain true history. The 2026-10-05 approved
reconciliation returns that history to main without requiring publication first.
Keep canonical main untouched while preparing authorized isolated candidates.

Beads remains the issue/dispatch authority. Put the exact Beads ID, reproducer,
scope, local review pointer, test/evidence state and disposition into each PR.
If a corresponding GitHub issue exists, link it as well; do not manufacture a
second independently maintained task matrix for every Beads issue.

1. **Triage and boundary.** Establish the current cause and Lane. Lane A has
   one fix record containing observation, cause, scope, verification and review;
   it does not need a feature's requirements/UX/architecture/tasks document set.
   New product decisions use Lane B instead of expanding the repair silently.
2. **Implementation.** One issue gets one `fix/<slug>` branch and one PR. Use
   impact-selected checks and a failing regression. Preserve dirty work and
   freeze a concrete commit/content state before independent review.
3. **Local independent review.** A cold-context, separate reviewer examines
   that exact state. A separate worktree alone does not establish independence.
   Keep the verdict and evidence with the issue's fix record; the main agent
   owns status/evidence/Beads writes.
4. **GitHub Codex review.** Request `@codex review` on the PR and retain the
   resulting review for the current head. A request/eyes reaction is not a
   completed review. The official service focuses on P0/P1; it complements
   local review and tests rather than replacing local P2/P3 coverage. Material
   repairs after review require current-head reassessment. Do not request
   cloud `@codex fix` unless write/delegation authority is separately granted.
5. **Delivery.** Fix verified findings, pass applicable CI and evidence gates,
   then merge the current-iteration issue PR into `main`. Confirm the delivered boundary before closing the
   Beads issue; admin dispositions say duplicate/superseded/not-a-defect rather
   than claiming a product fix. Keep per-issue traceability in the version list.
6. **Single release.** Merge ready issue PRs throughout this one iteration.
   Afterwards reconcile the final selected list and run release-owned preflight
   once for the final aggregate state, reusing unaffected per-issue evidence.
   Validate normal installation, account/state safety and configured Widgets,
   then publish v0.6.5 from the separately authorized verified candidate through
   the release workflow. Main integration precedes publication; supported-release
   maintenance retains its explicit propagation obligations. Internal order
   or parallel PRs do not create separate release batches.

### Worktree decision

The selected mode is one reusable serial Fix workspace, one issue branch and
one PR per repair. Execution rules have one owner:
[Serial Lane A Fix workspace entry](../.agent-instructions/branching.md#serial-lane-a-fix-workspace-entry).
They define entry/reentry, verified current-main bases by default (supported-release
bases only for explicitly selected maintenance), issue bindings, safe switch boundaries,
owner conflicts, CodeGraph preparation and late-review return. The independent
reviewer needs cold context and frozen content, not a permanent extra worktree.

[The v0.6.5 contract](archive/topics/v0-6-5-contract/tasks.md) owns adoption readiness and
version execution; this roadmap does not approve that task's implementation or
mirror its progress. Use the rules only after their required review and delivery.
Parallel issue work remains a separately authorized choice.


## v0.7.0 — Scan performance, pricing redesign and work sessions (proposed)

On 2026-10-08 the operator requested a task design for four areas after stable
v0.6.5. This is planning intake, not an approved implementation matrix or a
release commitment. The historical `ad-v070` and `ad-v070-plan` concern the
superseded subscription-quota plan; keep them closed and unchanged. Use
`ad-v070-iteration` for this proposal and retain existing issue identities.
A later `v0-7-0-contract` topic will own approved active membership; it does
not originate the feature topics' requirements, UX or architecture.

### Proposed scope and decisions

| Area | Carrier | Boundary |
| --- | --- | --- |
| Long-scan performance | `ad-bug-menubar-long-scan-timeout` | Explain full rereads and queued requests, bound repeated scan work, and distinguish waiting/progress from failure. Preserve usage/session equivalence, cancellation and ownership. |
| Price-system and subscription-analysis redesign | `ad-cost-transparency` | At minimum: migrate pricing to models.dev with multi-tier support; add subscription credits estimates; resolve auto-code-review calculation; handle A/B experiment models in billing and presentation. The whole topic requires fresh design; scope stays open. |
| Auxiliary-session isolation | `ad-auxiliary-session-classification` | Identify auxiliary work without deleting sources or suppressing its usage/cost. The operator selected default isolation from work-session counts, project counts and durations, with a switch to inspect/include it. |
| Codex work-signal ingestion | `ad-bug-codex-work-signals-missing` | Restore Activity/Workflow/Tooling for current Codex message envelopes and recover historical derivations. Proposed Lane A; lane confirmation and implementation remain separate. |

The auxiliary-session default is selected. Classification evidence, override
persistence, switch placement and cross-surface behavior remain design decisions.
Proposed rules: retain unknown origins in the ordinary work view; explain
auxiliary classifications; offer Work/Auxiliary/All scopes; keep total usage and
cost complete and label their scope when it differs from work KPIs. A project
directory name alone must not silently establish auxiliary identity. Disabling
the filter must restore the complete underlying collection.

### Price-system redesign: minimum intake, not a frozen scope

The operator clarified on 2026-10-08 that `ad-cost-transparency` itself may be
incomplete and the entire topic needs careful redesign later. Its prior title,
description and acceptance are historical inputs, not a ceiling on this version.
Credits are now expressly included, superseding the old tracker exclusion.
The user's minimum requested content is:

1. **Rebuild pricing around models.dev.** The reported motivation is GPT
   multi-tier billing missing from the current LiteLLM representation. Validate
   the proposed source's coverage, tier schema and dates against actual model
   pricing and request evidence. A source swap alone is not acceptance.
2. **Add credits estimates for subscription analysis and monetary estimates.**
   Define sources, rules, applicable plans/models, uncertainty and the relationship
   to displayed money. Estimated credits must remain distinguishable from actual
   subscription debits or invoices.
3. **Resolve auto-code-review calculation.** Preserve this user-supplied name.
   The old `codex-auto-review` / `gpt-5.4` mapping is an investigation lead;
   do not equate the names or hard-code a model without evidence.
4. **Handle A/B-rollout models in pricing and presentation.** Preserve observed
   identities and distinguish display names, proven aliases, experimental IDs,
   supported prices and explicit unknown/fallback states.

This list is deliberately non-exhaustive. Revisit amount semantics, model/rate
correctness, request-level tier evidence, source/version freshness, historical
recalculation, privacy and upgrade compatibility during requirements/UX/
architecture design. Legacy `272K` and model-price examples are hypotheses.
LiteLLM investigation now supports migration comparison, not the target design.
Context Efficiency remains undecided; it is not silently included or permanently
excluded. No implementation count, effort or schedule estimate is established.

Antigravity, Linux, automatic updating and multi-device work remain outside the
currently proposed four-theme version scope.

### Design agenda and prospective delivery units

These units order discovery and acceptance; they are not Development-ready
Beads tasks. Create implementation dispatch only after the owning topic's
required requirements, UX, architecture and decomposition are approved.

| Unit | Deliverable and dependency | Observable acceptance | Verification boundary |
| --- | --- | --- | --- |
| P1. Scan baseline and budgets | Profile first import, forced/parser-version rebuild, unchanged scan, one-file append and queued requests; separate discovery, parsing, SQL and snapshot costs. Start from `internal/ingest/ingest.go`, usage/session ingestion and the embedded helper. | Reproducible input near the observed 5,071-file scale; input identity and repeated p50/p95, CPU, memory, bytes-read and queue-wait measurements. Establish the original reread trigger or preserve uncertainty. Approve numerical latency/resource budgets before performance implementation. | Isolated benchmarks and receipts; no rebuild of the live user databases. |
| W1. Codex work-signal compatibility | Restore real user-turn reduction for current/legacy envelopes in `internal/usage/usage.go`, with `internal/activity` and signal consumers. Owned by `ad-bug-codex-work-signals-missing`. | Classified turns and consistent Activity/Workflow/Tooling. Cover messages before/after turn context, append boundaries, duplicate envelopes, injected context and pending turns. Preserve true zero versus unavailable. | L2: isolated JSONL-to-SQLite regressions, affected usage/activity/session/desktop tests and Go core regression. |
| W2. Historical signal recovery | Define targeted replay/version invalidation for W1; budget it using P1. Same Bug, not a second issue. Preserve source ownership and idempotence. | Previously indexed affected sources gain correct derivations; a second scan creates no duplicates. Existing event identities/token totals, credentials, providers, exclusions and sources are preserved. Unaffected sources avoid unnecessary replay. | L3: replay, rewrite, crash/cancellation, ownership and privacy checks on isolated copies. |
| P2. Scan and waiting behavior | Apply measured fixes from P1, including W2's recovery cost. Define long-running/queued/failed/cancelled UX and helper contracts. | No-op scans parse no transcript bodies; append reads are bounded to changed data and documented anchors. Full/incremental results agree. A healthy background scan crossing 120 seconds is not reported as failed solely because a requester timed out. Approved resource/latency budgets pass repeated measurements. | L3: affected race checks, scale tests, native progress and cancellation acceptance. |
| C0. Re-design the whole pricing topic | Re-open requirements discovery around the four minimum inputs above, existing issue history and actual pricing/subscription/model evidence. Produce coherent requirements, UX, architecture and only then implementation decomposition. | All four minimum items have explicit contracts and acceptance; additional discovered requirements have a disposition. Validate source/tier/credits/model mappings, unknown handling and historical-data implications. No fixed package count or effort estimate before this work. | Design evidence and independent review first; select L2/L3 for actual changes after scope is known. |
| A1. Auxiliary identity and filters | Define evidence-backed user/auxiliary/unknown classification and the selected default. Keep canonical session identity/project attribution separate. | User work is retained, known observer samples isolated, unknown samples visible, and switching reversible. Replay/late metadata preserves identity and classification provenance. | L2; L3 for persisted migration. Mixed/duplicate sources and late-metadata fixtures. |
| A2. Work-view consistency | Apply A1 to session lists, work/project counts and duration aggregates; coordinate with W1 and the redesigned pricing topic for signal/cost scope labels. | Work KPIs share the selected scope. Work + Auxiliary reconciles with All under documented unknown handling. Total usage/cost stays complete. Different time/event scopes are explained rather than forced into equal counts. | L2 plus CLI/native mixed-session acceptance and cross-surface comparisons. |
| V1. Version reconciliation and acceptance | After selected deliveries pass their own gates, reconcile the v0.7.0 contract, upgrade compatibility and release criteria. | Combined scale tests cover scan/replay/filter/pricing; normal upgrade preserves original event identities and protected inputs. Operator accepts native signals, filtering and cost explanations. Historical waivers remain historical. | Separately authorized version assembly and L4 release validation; reuse unchanged valid evidence. |

Recommended dependencies: P1 informs W2/P2's budget; W1 → W2 and W2 is included
in P2's acceptance. C0 precedes any pricing implementation decomposition.
A1 → A2, with A2 consuming W1 and the pricing topic's settled scope semantics.
V1 waits for selected deliveries. Start with P1/W1; pricing and auxiliary
requirements may be explored independently. This is dependency guidance,
not parallel-agent authority or an effort estimate.
Proposed feature owners are `scan-performance`, `cost-transparency` and
`auxiliary-session-classification`; the work-signal Bug keeps a bounded repair
carrier. Existing deferred carriers remain unclaimed until their work starts.

The current-message Bug was reproduced on v0.6.5 source: Codex usage and tool
calls are stored but no Codex `usage_work_signals` rows exist. The parser notes
only legacy `event_msg/user_message`; a current transcript contains
`response_item/message(role=user)` and no legacy messages. Workflow/tooling
joins require classified rows. A controlled one-event/one-edit comparison
produced no signal for the current envelope, versus a classified activity,
2-second first edit and one tool call for the legacy envelope. The Bug carries
the evidence; this planning record does not claim a product repair.



## Backlog

The operator selected long-scan performance as a main priority for the next
version on 2026-10-07 and proposed v0.7.0 on 2026-10-08 as recorded above;
the implementation plan remains unapproved. `ad-bug-menubar-long-scan-timeout` is deferred (Lane C). An installed
v0.6.5-rc.1 round processed 5,071 files in 1,540,148 ms while menu-bar requests
hit their 120-second timeout; the background round ultimately completed.
Investigate full/rebuild and incremental scan cost, queued requests, and the
presentation of ongoing work versus genuine failure. The trigger for that
full reread remains unconfirmed. This planning candidate does not start
performance implementation during the Widget centering repair.

These candidates have no approved implementation plan. Promote each into a
bounded plan before development; do not expand an active plan opportunistically.
Candidates that carry a delivery version live in the Roadmap above. An item
labelled as planning intake for a version is only scheduled for separate design
and disposition while that version is planned; it is not yet part of that
version's delivery scope.

Pricing intake moved out of v0.6.0 and remaining candidates (keep them separate
until design evidence justifies merging them):

The earlier v0.6.0 cost-truthfulness release was cancelled and its attribution
scope shipped in v0.5.0. The new v0.6.0 selection above supersedes that release
sequence. Pricing investigations below belong to the proposed v0.7.0 cost-transparency design;
credits are now explicitly included in that redesign; Context Efficiency remains
undecided. The attribution defect is
no longer a Backlog candidate:
it was delivered by the archived
[`usage-attribution-precision`](archive/topics/usage-attribution-precision/tasks.md)
topic. That topic corrects the current contract that reserves `exact` for
`agentdeck run` while hardcoding otherwise determinable Hook routes as
`estimated`; a determinable event classified as `inferred` is not publishable.
There is no later attribution item to reconcile from this checklist.

Pricing catalogs and tiers remain independent investigations within the
cost-transparency candidate proposed for v0.7.0.
Credits are part of the new minimum intake; Context Efficiency is undecided.
Subscription discovery
is selected into v0.6.0 with quota, reset information and reminders.

- [ ] Model the two public-API price tiers for the GPT-5.6 family around the
  `272K` context boundary. Determine the tier for each request from the concrete
  context evidence in its JSONL record rather than from a session-wide or model
  name assumption; confirm the exact boundary semantics during design.
- [ ] Audit and correct the current public-API price assigned to `gpt-5.6-sol`,
  including its model identity and input, cached-input, and output rates.
- [ ] Diagnose why the latest LiteLLM price catalog produces incorrect AgentDeck
  prices. Keep upstream catalog data, catalog retrieval/versioning, parsing, and
  model-name matching as distinct hypotheses until evidence identifies the
  failing layer.
- [ ] Resolve the user-reported `auto-code-review` calculation problem; the old
  `codex-auto-review` / `gpt-5.4` mapping is an unverified historical lead. Retain the
  [OpenAI credit rate card](https://help.openai.com/en/articles/11481834-chatgpt-rate-card-business-enterpriseedu-credit-based-pricing)
  as the cited public source and define freshness/fallback behavior during
  design.
- [ ] Preserve public API-equivalent pricing as the cost estimator's baseline,
  while evaluating a separate credit-denominated pricing and presentation
  format. Never present credits, API-equivalent cost, or an actual subscription
  charge as interchangeable values without an authoritative conversion.
- [ ] Define a `Context Efficiency` diagnostic before choosing a presentation
  surface. Evaluate model, token volume, cache hit, context size, a `>272K`
  marker, long-context multiplier, credit cost, and API-equivalent cost; define
  an observable meaning for "useful context" and "wasted context" rather than
  deriving those values from input-token count alone.
- [x] Deliver the selected v0.6.0 Codex and Claude subscription feature above.
  Keep account discovery and quota/reset sources independently verifiable even
  when they share a user-facing feature.

- [ ] Revisit ChatGPT app project attribution only if the app exposes a stable,
  reachable project configuration surface.

- [x] Give `state_busy` cause-specific, safe recovery guidance. The selected
  `health-recovery` topic resolved the `state.lock`/`scan.lock`, read-only doctor
  and live-owner safety decisions. It was integrated through PR #9 at main
  `3f29e26`; origin issue `ad-bug-state-busy-recovery-guidance` is closed.

The five quota/status-line/portable-restore findings formerly deferred from
PR #5 Round 13 were promoted to the single
[v0.6.5 inventory](#unique-bug-inventory-and-applied-priority). Their current
priority and dispatch disposition live there and in Beads; original review
provenance remains in the issue histories. They are no longer duplicate
unchecked Lane C entries in this unversioned Backlog.

## Withdrawn Candidates

Recorded so they are not rediscovered as gaps. Reopen only if the stated reason
stops holding.

- **Desktop update check.** Withdrawn from `v0.5.0` entirely on 2026-08-18 — no
  menu item, no preference, no copy, no network request. The desktop app is
  installed through the Cask or a direct download, both of which already carry an
  upgrade path, so an in-app check would add the product's only outbound request
  to duplicate one.
- **Homebrew core submission.** Not important to this project; the personal tap
  already serves stable and release-candidate channels.
- **Claude subscription/account switching.** Technically reachable — the login
  state is a single per-system-user macOS Keychain entry — but withdrawn: OAuth
  refresh tokens rotate server-side so a saved snapshot silently expires and
  cannot be validated offline, persisting another product's credential
  contradicts this project's no-plaintext-credential rule, cross-application
  Keychain access is hard to justify in the trust model, and a failed write
  leaves the user unable to authenticate with no rollback path.
- **Extension mutation lifecycle.** The preview/plan/apply/ownership/rollback
  engine and its GUI, previously planned as two whole releases, chase each
  client's evolving extension format and duplicate management surfaces the
  clients already ship. Replaced by cross-client observability in `v0.9.0`.
