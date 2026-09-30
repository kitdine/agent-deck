---
status: active
created: 2026-08-25
updated: 2026-09-30
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

These are five selected feature areas for topic design, not an approved
implementation breakdown. Beads carries planning coordination; this section
owns the selection. A version-contract topic assembles the delivered topics
later. Do not create development tasks before each topic's tasks.md passes.
On 2026-09-26 the operator moved cost and price transparency out of v0.6.0;
`ad-cost-transparency` remains an independent planning carrier without an
assigned delivery version.

| Feature | Planning carrier | Reason and boundary |
| --- | --- | --- |
| Schema compatibility and Hook failure visibility | `ad-schema-compatibility` | Prevent permanent schema refusal from silently losing Hook observations; reuse the existing schema-version-signal document tasks. |
| Menu-bar and Widget refresh | `ad-desktop-refresh` | Make freshness and errors truthful; target approximately 1 minute for the menu bar and 3–5 minutes for widgets with change-driven updates. |
| Snapshot performance | `ad-snapshot-performance` | Reduce repeated aggregation before increasing refresh frequency; verify invalidation and output equivalence. |
| Actionable health recovery | `ad-health-recovery` | Resolve lock recovery guidance and stale extension inventory with cause-specific actions; retain the two origin bugs. |
| Codex/Claude subscription accounts, quota, reset and alerts | `ad-subscription-quota` | Make subscription capacity actionable, including Codex reset-count information, reset times, and opt-in reminders. |

Subscription design must verify each field's source and supported semantics.
Codex reset counts are explicitly in scope: distinguish any official total,
used or remaining reset allowance from natural quota-window resets and locally
observed reset events. Unsupported fields report unavailable with a reason,
never zero or an invented count. A missing critical source is a named delivery
gap requiring disposition, not permission to silently remove this feature.
Account/plan discovery, quota retrieval and reminder evaluation may have separate
sources. Authentication, staleness, polling budgets, account isolation and
notification deduplication are part of the design. No reset action, account
login switching, automatic app updater, or plaintext credential persistence is
included.

Design subscription-source feasibility early alongside schema and performance
work. Refresh delivery depends on snapshot performance. Subscription reminder
presentation retains truthful billing labels; later cost-transparency design
may refine those labels but does not block this version.

Cost transparency and structured session search are excluded from v0.6.0 and
remain unassigned candidates. Credits conversion, Context Efficiency, Linux,
a public adapter protocol, full extension observability and multi-device
aggregation remain later candidates. Existing withdrawal decisions remain
effective.

The old v0.7.0 subscription epic/plan/Gate are superseded by this selection;
their historical records remain parked. The remaining old version rows are
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

Restore existing contracts and preserve data, schema and external-client
configuration ownership. A proposed new command, typed code, persisted format,
output semantics, default filtering rule or changed user-visible number/count
for unchanged input requires an explicit scope/compatibility disposition;
do not silently expand the defects release. Auxiliary-session classification
has therefore been split out as a feature rather than attached to the metadata
repair. Version-number semantics remain in the stable CLI design contract.

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

All nineteen retained Bugs are labelled `v0.6.5` and unclaimed. `open` means
scheduled intake, not a reproduced current defect, approved Lane, or finished
repair. Reproduce historical findings before writing production changes.

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
   then merge the issue PR. Confirm the delivered boundary before closing the
   Beads issue; admin dispositions say duplicate/superseded/not-a-defect rather
   than claiming a product fix. Keep per-issue traceability in the version list.
6. **Single release.** Merge ready issue PRs throughout this one iteration.
   Afterwards reconcile the final selected list and run release-owned preflight
   once for the final aggregate state, reusing unaffected per-issue evidence.
   Validate normal installation, account/state safety and configured Widgets,
   then publish v0.6.5 through the authorised release workflow. Internal order
   or parallel PRs do not create separate release batches.

### Worktree decision — PROPOSED

An issue, branch and PR do not require a permanently retained worktree. The
Git checkout is only one part of the cost: copied build outputs/indexes and a
full feature-document lifecycle for a small repair are the avoidable weight.

| Option | Benefit | Constraint |
| --- | --- | --- |
| Reuse one serial repair worktree and switch per-issue branches | Lowest checkout/index footprint | One local mutable issue at a time. Every switch requires a clean tree and verified binding; returning to PR findings requires restoring the correct branch/state. A reviewer must not accidentally inspect the next branch. |
| Short-lived per-issue worktrees, with at most two active | Stable branch/path/review binding, easier overlapping work and returning to findings | Some setup cost, but merged issue workspaces must not accumulate. Shared tool caches may be reused under existing rules; mutable indexes/build artifacts remain correctly scoped. |

Recommendation: begin with **one active issue worktree at a time**, adding a
second only for explicit parallel work. Keep that checkout through repair,
local review and PR review, then clean it after verified merge under authorised
cleanup. This keeps independence/bindings straightforward without retaining
nineteen workspaces. Serial reuse is a viable later optimisation once explicit
Fix entry/rebind/return-to-review rules are adopted.

Current Branching documents ordinary topic entry but does not fully define a
reusable Lane A slot. This proposal does not silently change its runtime or
the shared workflow Skill. Before the first v0.6.5 repair, adopt the chosen
Fix workspace policy with the repository rule owner; resolve a valid binding
before product work and do not switch another active actor's checkout.


## Backlog

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
sequence. Pricing investigations below belong to the future cost-transparency design;
credits and Context Efficiency remain unscheduled. The attribution defect is
no longer a Backlog candidate:
it was delivered by the archived
[`usage-attribution-precision`](archive/topics/usage-attribution-precision/tasks.md)
topic. That topic corrects the current contract that reserves `exact` for
`agentdeck run` while hardcoding otherwise determinable Hook routes as
`estimated`; a determinable event classified as `inferred` is not publishable.
There is no later attribution item to reconcile from this checklist.

Pricing catalogs and tiers remain independent investigations within the
unassigned cost-transparency candidate.
Credits and Context Efficiency remain later candidates. Subscription discovery
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
- [ ] Treat `codex-auto-review` as a separately auditable fallback-classification
  candidate whose currently public model mapping is `gpt-5.4`; retain the
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

- [ ] Reinstall the status-line route when a reading-off call's RestoreStatusLine
  succeeds but the subsequent settings save fails before committing. Core state
  keeps recording `ProbeEnabled=true`/`StatusLineConsent=true` while the route is
  actually gone, so the next load claims capture enabled with nothing installed.
  Deferred as Lane C from PR #5 (feature/subscription-quota) review round 13 on
  2026-09-18, carried by `ad-bug-quota-reading-off-route-not-restored-on-save-failure`.
  Mirrors the already-fixed quota-statusline disable path's own compensation.

- [ ] Detect a currently-installed status-line route that belongs to a
  DIFFERENT AgentDeck installation (a different `--state-dir`) before `SetupStatusLine`
  records it as an ordinary chainable prior command. `PriorStatusLineCommand`
  deliberately refuses to chain to any managed AgentDeck command, so silently
  recording another installation's own route this way blanks the user's Claude
  status line for as long as the newer installation stays active, with no error.
  Deferred as Lane C from PR #5 review round 13 on 2026-09-18, carried by
  `ad-bug-quota-statusline-cross-installation-conflict`.

- [ ] Attribute a Claude prose parse failure's projected `source` to the probe
  route that actually failed (prose), not to the last successfully retained
  window's route (which can be status-line). The wire and menu-bar header
  currently misreport which route produced the newer failure. Deferred as Lane C
  from PR #5 review round 13 on 2026-09-18, carried by
  `ad-bug-quota-parse-failure-source-misattribution`.

- [ ] Return `.all` (not the configured single client) as the large quota
  widget's scope label, since `presentedQuotaClients(.systemLarge)` always shows
  both clients regardless of the widget's configured intent -- the header
  currently names only one client while the body shows both. Deferred as Lane C
  from PR #5 review round 13 on 2026-09-18, carried by
  `ad-bug-widget-large-quota-header-client-label`.

- [ ] Clear account-bound quota state (`quota_windows`, `quota_envelopes`, and
  the alert-notice ledger) during portable-backup restore, not only status-line
  consent (already fixed in review round 12). Restoring onto a machine signed
  into a different Codex account currently keeps presenting the source
  machine's quota figures as the target account's own, since the restored
  provider selection still passes the official-provider gate and Codex
  attribution is hard-coded confirmed. Deferred as Lane C from PR #5 review
  round 13 on 2026-09-18, carried by
  `ad-bug-quota-portable-restore-account-bound-state`.

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
