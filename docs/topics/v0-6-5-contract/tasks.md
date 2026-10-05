---
status: active
created: 2026-09-30
updated: 2026-10-05
---

# v0.6.5 Contract — Tasks

Target: one concentrated defect-repair iteration and one final `v0.6.5` release.
The operator's 2026-10-03 decision adds one bounded Lane B exception:
`launchservices-recovery`, originating in
`ad-bug-lsregister-collision-no-recovery-path`, ships with v0.6.5 after its own
requirements, terminal surface, architecture and decomposition pass independent
review. All other membership and feature exclusions remain in force.
Origin: the operator's version selection in [Roadmap](../../roadmap.md#v065--concentrated-defect-repair)
and explicit request to design this version contract. Input is sufficient for
version decomposition; individual Bug causes, lanes and implementation scopes
still require current-state triage. This reviewed plan selects membership and
tasks; it grants no Git delivery or release authority.

## Documents

| Document | Draft | Review |
| --- | --- | --- |
| tasks.md | [x] | [x] |
| requirements.md | n/a | n/a |
| architecture.md | n/a | n/a |
| `ux/` | n/a | n/a |

This is a version-contract topic, not a new product feature. Its only design
document is `tasks.md`; Bug repairs retain their own Lane A fix records or Lane B
topic contracts. The decomposition reviewer ratifies this document set.
Review history belongs in `reviews/tasks.md` when review occurs. This file owns
unmerged version execution status; [Project Status](../../status.md) continues
to describe the integrated baseline. Beads owns issue dispatch, and CEv1 owns
exact-state evidence; no status is inferred from another system's checkbox.

## Working context and release baseline

- Contract workspace: `v0-6-5-contract`, branch `feature/v0-6-5-contract`, path
  resolved through its local workflow binding. Creation base is local main
  `f4d9f44e3517f323872b2179f10c4dd45c506acb`.
- Published product baseline: peeled `v0.6.0` commit
  `a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52`. The contract's newer main base
  contains closeout/governance history. This is creation provenance, not the
  current integration base; new iteration work selects verified current main.
- The original 2026-09-30 intake found no release line. On 2026-10-03 a read-only
  remote-ref query confirmed `release/v0.6.x` at
  `24623ec8bf0e172bf8e630ebe203163e76634403`. At that entry the local release ref was older and was not a valid source.
  The operator selected that patch-line baseline for the doctor topic. These
  are historical entry facts, not a live routing instruction.
- Under the 2026-10-05 approved correction, normal current-iteration feature/fix
  branches start from verified current main and PRs target main. Publication is
  separately authorized; it is not a prerequisite for main propagation.
  Explicit supported-release maintenance may instead use the oldest affected
  supported release line and subsequently propagate with preserved ancestry.
- The contract branch carries planning documents, not an intermediate product
  integration tree. The approved `feature/reconcile-release-main` workspace
  prepares release history propagation from main `868519d13902ad1f59c9688c252f92d6297c9d15`
  and release `8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30`. This is partial
  propagation under `assemble`, not complete membership assembly or publication.
  Keep main governance; do not merge newer main wholesale into an older release.

## Selected Bug inventory

The nineteen existing Bug carriers are the version's diagnosis/repair membership.
Live Beads inspection on 2026-09-30 found each `open`, unassigned, with priorities
P1: 6, P2: 11, P3: 2. These are intake facts, not nineteen reproduced defects or
approved repair plans. Preserve their descriptions and provenance. Start with P1,
normally installer Widget registration; reorder within priority for verified
dependencies. Do not start a Bug under this document-design command.

| Priority | Existing Beads Bug | Bounded diagnosis / acceptance target | Verification routing |
| --- | --- | --- | --- |
| P1 | `ad-bug-cask-widget-registration-missing` | Normal Cask install/upgrade/RC-to-stable/uninstall lifecycle; installed-path PlugInKit registration and configured timelines, without preregistration or user-data deletion. | L3 installer; final L4 installation acceptance |
| P1 | `ad-bug-session-project-attribution-observer-noise` | Canonical Codex ID/cwd, fallback precedence and old-index reparse; retain raw logs and usage. Auxiliary filtering is excluded. | L2 session/index |
| P1 | `ad-bug-quota-portable-restore-account-bound-state` | Cross-account restore clears account-bound quota state while preserving user/provider data. | L3 account/privacy/restore |
| P1 | `ad-bug-quota-statusline-cross-installation-conflict` | Two isolated installation roots preserve managed-route ownership and avoid recursive prior-command chaining. | L3 external configuration |
| P1 | `ad-bug-quota-reading-off-route-not-restored-on-save-failure` | Precommit settings-save failure restores the external route; retain postcommit semantics. | L3 external configuration/failure |
| P1 | `ad-bug-usage-scan-busy-snapshot` | Deterministic external-write reproducer and scoped publication transaction repair. | L3 concurrency/persistence |
| P2 | `ad-bug-widget-publisher-unavailable-silent` | Unavailable-container publication failure reaches existing health presentation. | L2 producer/consumer |
| P2 | `ad-bug-quota-parse-failure-source-misattribution` | Failed-attempt source metadata remains truthful through canonical wire and native consumer. | L2 wire/desktop |
| P2 | `ad-bug-widget-large-quota-header-client-label` | Reachable quota intent and large-family header match displayed clients. | L1 native Widget; raise if wire changes |
| P2 | `ad-bug-widget-cost-incomplete-contrast` | Reproduce authoritative prototype contrast; prove native behavior separately for any native claim. | L1 rendered/native surface |
| P2 | `ad-bug-prototype-probe-pending-assertion` | Canonical probe detects normal absence and pending presence. | L1 prototype probe |
| P2 | `ad-bug-prototype-statchip-fidelity` | Specimen matches native shrinking and hour-window semantics. | L1 rendered prototype |
| P2 | `ad-bug-arch-sessions-unavailable-not-a-consequence` | Active contract states the independent session-store condition; preserve review history. | L0 contract documentation |
| P2 | `ad-bug-hook-gate-ignores-content-state` | Current-source fixtures detect stale review evidence and changed subjects. | L2 shared Hook contract; cross-runtime fixtures |
| P2 | `ad-bug-hook-check3-directory-entries` | New untracked-directory entries expose relevant Markdown to the scoped observer. | L1 Hook fixtures; both runtimes |
| P2 | `ad-bug-hook-check5-lifecycle-word-match` | Distinguish obsolete lifecycle assertions from quotations and ordinary language. | L1 Hook fixtures; both runtimes |
| P2 | `ad-bug-lsregister-collision-no-recovery-path` | Operator-selected Lane B `launchservices-recovery`: new read-only doctor diagnosis and truthful manual recovery guidance, with independently reviewed feature contracts before implementation. | L2 doctor/health/output core contract, failure-first fixtures and isolated macOS read-only acceptance |
| P3 | `ad-bug-footer-routes-narrow-truncation` | Real native 280 pt reproduction and existing-policy check before repair or Lane B decision. 2026-10-05 bounded administrative evidence/disposition: [P3 carrier](../../fixes/footer-routes-narrow-truncation.md); no implemented repair or general native layout PASS. | L1 native surface; L0 administrative documents |
| P3 | `ad-bug-disttest-fixture-registrations-linger` | Bounded fixture registration cleanup with production identifiers unaffected. | L3 distribution lifecycle |

Levels are initial impact routes, not a substitute for per-issue scope discovery.
Each implementation plan names exact files, regression and affected subsystem.
Apply [Project Rules](../../../.agent-instructions/project-rules.md#testing-and-verification--测试与验证),
including `scripts/run-go-test.sh` for Go tests and the full Go suite when a core
contract changes. Shared Hook changes need their own relevant runtime fixtures;
documentation changes alone do not require product test suites.

All selected carriers must reach a recorded disposition before aggregate closure:
delivered repair, evidence-backed already-fixed/not-a-defect/duplicate, or an
explicit operator-approved deferral/removal. Failure to reproduce once is not
resolution. A blocked item remains selected and open unless membership changes
explicitly; priority or time pressure cannot silently drop it. Each disposition
records exact issue identity, fix/topic record, immutable source/result state,
PR/merge identity when applicable, review and evidence references, or the explicit
administrative decision. This table is version membership, not a duplicate Bug
lifecycle matrix. Material membership changes return this contract to review.

Excluded: `ad-cost-transparency`, `ad-antigravity-support`,
`ad-auxiliary-session-classification`, and future v0.8/v0.9/v1.0 feature plans.
No default auxiliary-session filtering, usage deletion, project/worktree grouping
policy, global locking redesign, or unrelated feature expansion is selected.
New doctor APIs remain excluded except the explicitly selected
`launchservices-recovery` contract. New decisions return to their owning Lane B
topic before code work.

### Approved operator decision — 2026-10-03

The real user selected P2 option 2 and explicitly stated “v0.6.5一起做”. The
current full-flow instruction authorizes adjusting the repair-only scope,
membership treatment and new-doctor-API exclusion, recording that decision and
independently reviewing the changed version scope before implementation. This
supersedes the earlier recommendation to assign the doctor API to an unspecified
later version. It selects no other feature or Bug and does not close the origin.

The real user subsequently approved the exact bounded primary-version-contract
proposal on 2026-10-03 (decision source: `Sentinel_986c24bbd3b081918c2d40986edea07c`,
“批准1”, reaffirmed in the independent resume instruction). The
[primary Version Number Semantics](../../specs/cli-design.md#version-number-semantics)
now expressly except only the declared read-only LaunchServices doctor/health
checks, codes, optional diagnostic fields and consequent warning counts from
the MINOR/PATCH output guarantee for v0.6.5. Scripts relying on fixed output or
counts may require adaptation; release documentation must disclose this.
Commands/flags, exit codes, databases, persisted formats and existing-check
semantics remain unchanged. All other subjects retain the general version rule.
Approval resolved the decision Gate; the amendment subsequently passed independent
re-review and exact-state evidence through PR #40. The feature followed its own
contract and implementation gates before its historical PR #42 delivery.

The version carrier remains this contract topic. The feature carrier is a
separate ordinary Lane B topic, `launchservices-recovery`, historically with its own
`feature/launchservices-recovery` workspace created from the verified patch-line
commit above. It never used the serial Lane A Fix slot. Its reviewed design defines
host/appex identity, actual conflict versus legitimate copies/stale/unknown
registrations, bounded enumeration and failure semantics, quick/full inclusion,
stable check/code/output and health behavior, manual guidance versus executable
`recovery_command`, and fixture/live read-only acceptance. Existing
`extension_duplicate_id` inventory semantics cannot be reused for OS resources.

Doctor remains read-only. No automatic unregister, system-registration cleanup,
daemon restart, real database rebuild, user-data write or new OS privilege is
selected. Native P3 footer and full-popover acceptance remain separate. This
amendment does not waive any independent contract, implementation, review,
exact-state CEv1 or remote delivery gate. A design PASS is not defect recovery.

The limited version-contract amendment was delivered through PR #40; the doctor
feature was delivered through PR #42 to `release/v0.6.x` and subsequently retired
under its own records. Preserve that release-line history and the approved
compatibility exception. Current work propagates the delivered history to main;
normal new feature PRs target main. No direct shared-release push, squash, rebase
or cherry-pick is selected. Release/publication remains separately authorized.

## One issue, one PR and serial workspace entry

The selected serial policy is implemented in
[Branching](../../../.agent-instructions/branching.md#serial-lane-a-fix-workspace-entry)
with default slot `.worktrees/fix`, entry `进入工作：fix / <slug>` and per-issue
binding `agent-deck.fix.<slug>`. Task 1's independent review has passed;
its delivery remains a prerequisite before the first repair uses these new rules.
The contract workspace remains distinct. An independent reviewer needs cold
context and the frozen content state, not a permanent extra checkout. No parallel
issue work or subagent invocation is authorized by this plan.

For each retained Bug:

1. Reproduce against its supported release context, verify existing contract and
   current cause, and settle the Lane under [Beads](../../../.agent-instructions/beads.md).
   Lane A uses one `docs/fixes/<slug>.md`; new behavior uses reviewed Lane B inputs.
   Keep the original issue as dispatch carrier; do not create separate Repair or
   Re-review tasks. False/duplicate/already-fixed findings receive administrative
   evidence and disposition, without a fabricated code PR.
2. For Lane A, resolve a valid issue binding and authorized `fix/<slug>` branch
   from the verified current main head (or an explicitly selected supported-release
   maintenance base). Lane B uses its ordinary feature workspace
   and reviewed requirements/surface/architecture/tasks progression; it must not
   claim implementation or turn draft contracts into product behavior. Claim
   only the approved work product under its actual stage authority.
   Establish a failing regression, implement the bounded repair, and freeze its
   source/content state with impact-selected verification.
3. Obtain local independent review of that frozen state and applicable CEv1
   gate. Bind Lane A evidence to `fix:<slug>`; Lane B retains its own hierarchy.
   A producer's self-check is not independent PASS. Reviewer invocation still
   needs actual Review/delegation authority; this design does not dispatch one.
4. Under separate delivery authority, commit/push/create the issue PR targeting
   `main` for current-iteration work. An explicitly scoped supported-release
   maintenance PR targets its selected release line. Include the exact Beads ID, reproducer, scope, fix/review
   pointers, verification state and retained risks; link a GitHub issue if one
   exists. Request GitHub `@codex review` only when explicitly authorized. Await
   actual current-head review; an eyes reaction is not completion. Do not invoke
   cloud `@codex fix` without separate write/delegation authority.
5. Resolve actionable findings, applicable CI/protection and evidence gates for
   the final head; reassess material changes. Classify integration against the
   actual selected target and record resulting evidence before authorized merge.
   Preserve ancestry; no squash, cross-line rebase or cherry-pick without an
   explicit exception. Verify delivered result before closing the Bug.
6. Freeze the owning workspace through review, repairs and verified delivery.
   Lane B retains its distinct feature binding; the following serial-slot rules
   apply only to Lane A. Before
   the next issue, ensure a clean tree, no conflicting active owner or unresolved
   local edits, the verified selected base (current main by default; a supported-release
   base only for explicitly selected maintenance), and a new verified binding.
   Never force-switch or reset dirty work. A late finding returns to its issue's
   branch and exact state; it cannot be repaired in the next issue's checkout.

Merge ready issue PRs throughout the iteration; release only once after final
membership reconciliation. Each issue has its own gate and traceability; one
aggregate release does not waive any per-issue review.

## Tasks

| Task | Dev | Review |
| --- | --- | --- |
| 1. `fix-workspace-policy` | [x] | [x] |
| 2. `assemble` | [ ] | [ ] |
| 3. `v0-6-5-contract` | [ ] | [ ] |

These are approved aggregate anchors. Create implementation dispatch after
this decomposition's required evidence/checkpoint conditions; the nineteen existing Bugs remain their own
dispatch units. The document task is `ad-v0-6-5-contract-doc-tasks-design`.
An aggregate Review tick requires the latest applicable independent PASS; Dev
and Review ticks do not by themselves establish delivery or release readiness.

### 1. `fix-workspace-policy`

**Result:** executable, reviewed serial Lane A workspace-entry rules, with a
stable slot identity and issue-specific branch/binding contract.

**Files:** `.agent-instructions/branching.md` owns the serial entry policy;
`.agent-instructions/toolchain.md` and `.agent-instructions/beads.md` change only
where necessary for binding/index and handoff consistency. Reconcile the roadmap
proposal with the delivered rule through a pointer, without copying execution
status. Update this task's matrix and create `reviews/fix-workspace-policy.md`
when reviewed. No shared Skill/runtime source, Hook implementation or product
code is included; if they are required, stop and obtain the distinct scope.

Define entry/reentry for `fix / <slug>`: source ref and exact verified selected base
(current main by default; a supported-release base only for explicitly selected maintenance), reuse
of one designated writable slot, collision/active-owner handling, clean-tree
switch boundaries, separate branch per issue, issue-specific binding identity,
per-slot CodeGraph init/sync after branch changes, late-review return and
non-destructive exit. A checkout and binding grant no claim, delivery or exclusive
lease. Branch/worktree changes still need real user entry/delivery authority.
Decide the exact supported entry command and slot naming in this rule task;
do not execute a proposed command before those rules are reviewed and delivered.

**Verification:** L0 rule/link/discovery consistency, `make check-whitespace`,
`git diff --check`; inspect the complete clean/dirty/conflict/late-return cases
against the existing workflow binding contract. Any actual runtime claim needs
separately authorized acceptance. Required task gate: reviewed policy coverage
and authority/binding consistency for its exact candidate/delivered state.

### 2. `assemble`

**Depends on:** reviewed/delivered Task 1 and this contract plan; explicit
per-issue workspace/delivery authority and selected integration base. Individual Bugs proceed only
after their own triage, lane, review and evidence prerequisites.

**Result:** main contains all retained repairs, with every inventory
item accounted for and every actual integration reviewed at its result state.

**Files:** owning `docs/fixes/<slug>.md` or Lane B topic files and affected product
files are determined by each Bug's approved scope; this contract's membership and
`reviews/assemble.md` retain source/target/result identities, merge class,
interactions and per-issue dispositions. Reconcile integrated status with actual
authorized integration, not a separate task-progress projection. Product repairs
go to main for this iteration, never through the contract branch. Explicit
supported-release maintenance retains its separate line and propagation route.

Select verified current main for the iteration and follow the issue flow above.
The approved release-to-main reconciliation is one partial propagation batch;
it neither completes this task nor starts issue #43 or another repair batch. Record
interactions across account-bound state, session indexing, scan publication,
health/wire consumers, Widget installation and Hook evidence. Do not equate no
textual conflict with behavioral compatibility. Record review/evidence pointers
with each delivered repair; carry unchanged valid evidence forward and assess
only affected dependencies on subsequent merges.

**Verification:** each issue uses its actual L0–L3 impact scope; integrations use
Branching's parent/result classification and CEv1 integration boundaries.
Aggregate task criteria: complete membership dispositions, per-issue review and
delivery continuity, and verified integration interactions at the final main
integration candidate. No partial issue batch marks this aggregate Task complete. Release
preflight is reserved for the later final release boundary.

### 3. `v0-6-5-contract`

**Depends on:** aggregate assembly and the exact-state required gates, with no
undisposed selected Bug. Explicit deferrals retain their decision and risks.

**Result:** the final patch candidate, shipped contracts and version documentation
agree, with main integration and separately authorized release readiness.

**Files:** reconcile this matrix, `reviews/v0-6-5-contract.md`, affected
`docs/specs/cli-design.md` and `cli-manual.md`, `docs/status.md`, `docs/README.md`
and roadmap pointers only at their actual integration/closure boundaries.
Version/build/packaging files change only under separately authorized release
preparation for the identified candidate; no tag or build number is allocated
merely to complete this document task.

Record delivered repairs versus administrative dispositions/deferred cases,
compatibility and account/configuration/data safety, current known limitations,
and source/result/evidence identities. Reconcile version history only where
actual shipped contract changes require it; do not force a schema/wire bump for
a patch version. Retirement follows the delivered subjects' own lifecycle and
preserves review/evidence history. Confirm the Topic's task hierarchy and gate
before claiming the version-contract topic complete.

**Verification:** L0 document/link/version consistency and the contract task and
topic gates, reusing applicable issue/integration evidence. Check product paths
only for changed assumptions or missing criteria. Contract completion is not
Release VERIFIED, publication, or a waiver of native/performance gaps.

## Final release and supported-release propagation

After assembly and closure, obtain separate release preparation/publication
authority. Resolve the Release WorkUnit, final candidate SHA, required preflight
and release criteria under Evidence and actual workflow files. Run L4
`make release-verify` and authorized same-SHA preflight once for the final
aggregate state; reuse unaffected per-issue results. Material candidate changes
invalidate only affected evidence and require the corresponding reassessment.

Verify normal installation/upgrade/RC-to-stable, account/state safety and
configured Widgets against the actual installed paths. Manual `pluginkit -a`
before the test cannot establish installer PASS. Preserve v0.6.0's FAILED normal
Cask local-install evidence and historical native/performance exceptions; only
new exact-state evidence can establish v0.6.5 acceptance. Simulated checks,
waivers and unrun native checks remain explicitly identified.

Publish one `v0.6.5` from the verified, separately selected release candidate
only under tag/publication authority, after current-iteration main integration.
Explicit supported-release maintenance uses separately authorized release-line
→ main → affected feature-line propagation, preserving ancestry and reviewing
actual interactions. Historical patch-line deliveries and their evidence remain
unchanged. No publication is a prerequisite for the current reconciliation.

## Current integration handoff — 2026-10-05

The routing correction and bounded P3 disposition have passed independent document
and bounded integration re-review. See [Tasks review](reviews/tasks.md) and
[partial integration review](reviews/assemble.md) for the exact reviewed states.
This synchronization records that result; current-target evidence and signed
local delivery remain separate boundaries. Exact-head hosted CI/review, the
supervisor check before remote merge, and post-merge evidence remain pending.

Aggregate assemble, topic and release completion remain open. The P3 carrier
retains only the original sample/measured-environment administrative disposition;
the deferred performance issue #43 remains outside this propagation scope.

## Earlier planning handoff — historical 2026-09-30

Decomposition review passed in [tasks review](reviews/tasks.md), Round 1.
Task 1 passed [independent review](reviews/fix-workspace-policy.md), Round 1; its dispatch is
`ad-fix-workspace-policy` and task WorkUnit is
`v0-6-5-contract:fix-workspace-policy`. The boundary comprises Branching,
Toolchain, Beads, the roadmap pointer and this task matrix. L0 checks and exact
candidate identity are handed off through the task. Authorized Task 1 delivery
remains pending; Tasks 2/3 and all nineteen Bugs remain unimplemented here. No release
line, Fix slot, issue branch or Git delivery was created by this implementation.
The document WorkUnit is `v0-6-5-contract:tasks.md`, with required criteria for
decomposition/membership coverage, release-base and authority consistency, and
document-set/verification readiness. The review record binds HEAD plus the final
document blob and records the separate document gate. No aggregate task, topic
or release completion is claimed by this document PASS. The earlier document
PASS remains provenance for the decomposition; Task 1's implementation/status
changes require their own review and target-state gate rather than relabelling
the earlier document evidence.
