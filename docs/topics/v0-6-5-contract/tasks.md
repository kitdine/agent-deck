---
status: active
created: 2026-09-30
updated: 2026-10-07
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
binding `agent-deck.fix.<slug>`. Task 1 was independently reviewed and delivered through PR #22 into main
at `59aa33a33b3568bd3d6e7e9840103086cf0ccfd8`. Its earlier pending-delivery
handoff below is historical. The approved current-main routing correction was
subsequently delivered through PR #46; this closeout starts no new Bug repair.
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
| 2. `assemble` | [x] | [x] |
| 3. `v0-6-5-contract` | [x] | [x] |

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

## Current closeout — 2026-10-06

Workspace `agent-deck.v0-6-5-closeout`, branch `feature/v0-6-5-closeout`,
creation base and actual remote main `71e8047e0dd8d638be8587ff1a016c38a9cea635`
(tree `cef4236123e512252d340e07b2cbcd7dae7d1842`). PR #46 is merged,
with parents `868519d13902ad1f59c9688c252f92d6297c9d15` and
`909d731847362d2d6453c7a5ede9870a0c35079e`; release
`8b1f82b09664f32337dbb9ed9f3e4f14cc2b4d30` is an ancestor. Actual postmerge CI
[37383050159](https://github.com/kitdine/agent-deck/actions/runs/37383050159)
attempt 1 succeeded in all four jobs. There is no PR46 implementation or merge
left to repeat. Its completed 5/5 integration gate remains bounded.

The real user authorized this documentation/evidence closeout, scoped safe
verification and cold independent review on 2026-10-06. Commit, push, PR creation,
merge, release, installation and OS registration remain unauthorized. Task2 and document independent reviews passed; actual synchronized gates
returned VERIFIED6/6 and3/3 respectively after scoped preservation assessment.
Task3 now has the compatibility blocker recorded below; final handoff targets
remain separately bound after synchronization. See [assembly accounting](reviews/assemble.md#aggregate-task2-evidence-ledger--2026-10-06).

### Current member dispositions

All nineteen Beads members were read back closed. Eighteen have delivered repairs;
footer retains bounded administrative not-a-defect. These observations do not
prove the new aggregate gate. Full source/merge objects, final-head review
comments and actual CI results are retained in the operator-local audit bundle
`/tmp/agentdeck-v065-closeout-audit-20261006/`; it is provenance, not a portable
download link. Applicable gates below name their historical delivered targets.

| Priority | Existing Bug | Disposition and carrier | Actual source / merge | Historical required gate |
| --- | --- | --- | --- | --- |
| P1 | `ad-bug-cask-widget-registration-missing` | delivered bounded repair; [PR #23](https://github.com/kitdine/agent-deck/pull/23); [carrier](../../archive/fixes/cask-widget-registration-missing.md) | `39212a5d9fe15c4cea6651517b4675163ef576cb` / `6f22e76da9f720fed8d00f5a031e796bcb4f6cd6` | `fix:cask-widget-registration-missing`, 4/4 at `fix:cask-widget-registration-missing:merge:6f22e76da9f720fed8d00f5a031e796bcb4f6cd6` |
| P1 | `ad-bug-session-project-attribution-observer-noise` | delivered bounded repair; [PR #24](https://github.com/kitdine/agent-deck/pull/24); [carrier](../../archive/fixes/session-project-attribution-observer-noise.md) | `3821613a8dbe2d71a0df3f6d988ad4011b74ea4b` / `6e8c1c4d2942782152f5e6b5de89b0942fc57ce4` | `fix:session-project-attribution-observer-noise`, 4/4 at `fix:session-project-attribution-observer-noise:merge:6e8c1c4d2942782152f5e6b5de89b0942fc57ce4` |
| P1 | `ad-bug-quota-portable-restore-account-bound-state` | delivered bounded repair; [PR #27](https://github.com/kitdine/agent-deck/pull/27); [carrier](../../archive/fixes/quota-portable-restore-account-bound-state.md) | `482cb353d40c3fd2f347e4a9aca5fd2c887257ca` / `ab5ec486e67edb5fd48745b54a302697b1db7ae9` | `fix:quota-portable-restore-account-bound-state`, 4/4 at `fix:quota-portable-restore-account-bound-state:commit:ab5ec486e67edb5fd48745b54a302697b1db7ae9` |
| P1 | `ad-bug-quota-statusline-cross-installation-conflict` | delivered bounded repair; [PR #26](https://github.com/kitdine/agent-deck/pull/26); [carrier](../../archive/fixes/quota-statusline-cross-installation-conflict.md) | `58515d5ce2320e624b8ba92e3ccf830b5e1307b4` / `e6e37d97ec7fb25e1da7b4efcf440d0cc33abf39` | `fix:quota-statusline-cross-installation-conflict`, 4/4 at `fix:quota-statusline-cross-installation-conflict:commit:e6e37d97ec7fb25e1da7b4efcf440d0cc33abf39` |
| P1 | `ad-bug-quota-reading-off-route-not-restored-on-save-failure` | delivered bounded repair; [PR #25](https://github.com/kitdine/agent-deck/pull/25); [carrier](../../archive/fixes/quota-reading-off-route-not-restored-on-save-failure.md) | `9b442318fdb4c5fb8e7b0d0746f8d93ffbdefa1c` / `570cc9e2c818f6a4da38365194c3cd5c7bcf4e93` | `fix:quota-reading-off-route-not-restored-on-save-failure`, 4/4 at `fix:quota-reading-off-route-not-restored-on-save-failure:commit:570cc9e2c818f6a4da38365194c3cd5c7bcf4e93` |
| P1 | `ad-bug-usage-scan-busy-snapshot` | delivered bounded repair; [PR #28](https://github.com/kitdine/agent-deck/pull/28); [carrier](../../archive/fixes/usage-scan-busy-snapshot.md) | `78e68d141773e89ca322a9a83704804679c37229` / `26662ba018444eb7758aba995fa678d47da5aa77` | `fix:usage-scan-busy-snapshot`, 4/4 at `fix:usage-scan-busy-snapshot:commit:26662ba018444eb7758aba995fa678d47da5aa77` |
| P2 | `ad-bug-widget-publisher-unavailable-silent` | delivered bounded repair; [PR #34](https://github.com/kitdine/agent-deck/pull/34); followups #35; [carrier](../../archive/fixes/widget-publisher-unavailable-silent.md) | `1b75cd6966641cc5e144332772393b3f1572002d` / `62863c804f4af665af767c5a54ecc366c74609c8` | `fix:widget-publisher-unavailable-silent`, 7/7 at `fix:widget-publisher-unavailable-silent:delivered:67a3248b6cf3e244ee7facef3597ad8b31ea8803` |
| P2 | `ad-bug-quota-parse-failure-source-misattribution` | delivered bounded repair; [PR #34](https://github.com/kitdine/agent-deck/pull/34); followups #35; [carrier](../../archive/fixes/quota-parse-failure-source-misattribution.md) | `1b75cd6966641cc5e144332772393b3f1572002d` / `62863c804f4af665af767c5a54ecc366c74609c8` | `fix:quota-parse-failure-source-misattribution`, 6/6 at `fix:quota-parse-failure-source-misattribution:delivered:67a3248b6cf3e244ee7facef3597ad8b31ea8803` |
| P2 | `ad-bug-widget-large-quota-header-client-label` | delivered bounded repair; [PR #32](https://github.com/kitdine/agent-deck/pull/32); [carrier](../../archive/fixes/widget-large-quota-header-client-label.md) | `f42cf93130f7a26291e7efe2cafc9533ca73951f` / `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b` | `fix:widget-large-quota-header-client-label`, 3/3 at `urn:agentdeck:widget-batch:merge:4c0cbd6270589812d618c02754eb1c0ac5dd9a5b` |
| P2 | `ad-bug-widget-cost-incomplete-contrast` | delivered bounded repair; [PR #32](https://github.com/kitdine/agent-deck/pull/32); [carrier](../../archive/fixes/widget-cost-incomplete-contrast.md) | `f42cf93130f7a26291e7efe2cafc9533ca73951f` / `4c0cbd6270589812d618c02754eb1c0ac5dd9a5b` | `fix:widget-cost-incomplete-contrast`, 3/3 at `urn:agentdeck:widget-batch:merge:4c0cbd6270589812d618c02754eb1c0ac5dd9a5b` |
| P2 | `ad-bug-prototype-probe-pending-assertion` | delivered bounded repair; [PR #33](https://github.com/kitdine/agent-deck/pull/33); [carrier](../../archive/fixes/prototype-probe-pending-assertion.md) | `5a96475d9e1cbe79f8f856b7a6c6e6d00d093721` / `951b16dcb036c8e2b24ca3b2fcb0a2e367324efa` | `urn:ce:agent-deck:work-unit:fix:prototype-probe-pending-assertion`, 5/5 at `urn:ce:agent-deck:content-state:git:951b16dcb036c8e2b24ca3b2fcb0a2e367324efa` |
| P2 | `ad-bug-prototype-statchip-fidelity` | delivered bounded repair; [PR #33](https://github.com/kitdine/agent-deck/pull/33); [carrier](../../archive/fixes/prototype-statchip-fidelity.md) | `5a96475d9e1cbe79f8f856b7a6c6e6d00d093721` / `951b16dcb036c8e2b24ca3b2fcb0a2e367324efa` | `urn:ce:agent-deck:work-unit:fix:prototype-statchip-fidelity`, 6/6 at `urn:ce:agent-deck:content-state:git:951b16dcb036c8e2b24ca3b2fcb0a2e367324efa` |
| P2 | `ad-bug-arch-sessions-unavailable-not-a-consequence` | delivered bounded repair; [PR #38](https://github.com/kitdine/agent-deck/pull/38); followups #39; [carrier](../../archive/fixes/arch-sessions-unavailable-not-a-consequence.md) | `c426b249098a844f5ec61fa97c2fdfac2aa1e151` / `132fca40c16928c9e7846a298bc8e583ff700c84` | `fix:arch-sessions-unavailable-not-a-consequence`, 5/5 at `fix:arch-sessions-unavailable-not-a-consequence:state:archive-merge-24623ec8` |
| P2 | `ad-bug-hook-gate-ignores-content-state` | delivered bounded repair; [PR #31](https://github.com/kitdine/agent-deck/pull/31); [carrier](../../archive/fixes/hook-gate-ignores-content-state.md) | `c9e59d775d6842e0c8b3be09ce3358c5001cc9f9` / `2eed6aff500c0691e8f02ad80976bc8e5dff2b58` | `fix:hook-gate-ignores-content-state`, 3/3 at `hook-consistency-batch:commit:2eed6aff500c0691e8f02ad80976bc8e5dff2b58` |
| P2 | `ad-bug-hook-check3-directory-entries` | delivered bounded repair; [PR #31](https://github.com/kitdine/agent-deck/pull/31); [carrier](../../archive/fixes/hook-check3-directory-entries.md) | `c9e59d775d6842e0c8b3be09ce3358c5001cc9f9` / `2eed6aff500c0691e8f02ad80976bc8e5dff2b58` | `fix:hook-check3-directory-entries`, 3/3 at `hook-consistency-batch:commit:2eed6aff500c0691e8f02ad80976bc8e5dff2b58` |
| P2 | `ad-bug-hook-check5-lifecycle-word-match` | delivered bounded repair; [PR #31](https://github.com/kitdine/agent-deck/pull/31); [carrier](../../archive/fixes/hook-check5-lifecycle-word-match.md) | `c9e59d775d6842e0c8b3be09ce3358c5001cc9f9` / `2eed6aff500c0691e8f02ad80976bc8e5dff2b58` | `fix:hook-check5-lifecycle-word-match`, 3/3 at `hook-consistency-batch:commit:2eed6aff500c0691e8f02ad80976bc8e5dff2b58` |
| P2 | `ad-bug-lsregister-collision-no-recovery-path` | delivered Lane B diagnosis/guidance; [PR #42](https://github.com/kitdine/agent-deck/pull/42); followups #44/#45; [carrier](../../archive/topics/launchservices-recovery/tasks.md) | `4f1283ad19cdbfdc49344f51aac53dd4f81fb190` / `f84e58a9ed8085b37d8bea605070ead5d837fc5f` | `launchservices-recovery`, 2/2 at `launchservices-recovery:state:31e761c02810898d7e362348f6be2fce2e47b60c8167e01c69f02a4be0df16f6` |
| P3 | `ad-bug-footer-routes-narrow-truncation` | bounded administrative not-a-defect; no product repair; followups #46; [carrier](../../fixes/footer-routes-narrow-truncation.md) | PR46 source `909d731847362d2d6453c7a5ede9870a0c35079e` / merge `71e8047e0dd8d638be8587ff1a016c38a9cea635` | no product WorkUnit; applicable `v0-6-5-contract:tasks.md` document gate |
| P3 | `ad-bug-disttest-fixture-registrations-linger` | delivered bounded repair; [PR #36](https://github.com/kitdine/agent-deck/pull/36); followups #37; [carrier](../../archive/fixes/disttest-fixture-registrations-linger.md) | `391c9f15b38f05b69c23c97bd1346f4c752e207e` / `41f43651cb5b436b7ff374fe46b39e21f92e2e0a` | `fix:disttest-fixture-registrations-linger`, 5/5 at `fix:disttest-fixture-registrations-linger:state:archive-final-merge-4b4b67a6` |

The original intake table remains version membership and acceptance routing; it
is not nineteen unfinished product tasks. Browser evidence does not establish
native acceptance. LaunchServices performance remains FAIL with issue #43 as
its existing excluded carrier. Actual system recovery, WidgetTimeline/collision
causality, expanded P3/native and final installed release acceptance are not
claimed. See the exact recovery/impact ledger and final interactions in the
assembly record.

### Declared aggregate acceptance and dispatch

Criteria were registered before any passing result, under namespace
`github.com/kitdine/agent-deck`. The Topic directly contains the tasks.md document
and Tasks1/2/3 WorkUnits. Document design gates, independent review, completion
evidence, Beads status and authorized delivery remain separate.

- `v0-6-5-contract:assemble` / Beads `ad-v065c-assemble-dev`:
  - `membership`: All nineteen selected Bug IDs have supported exact dispositions; delivered repairs and bounded footer administrative not-a-defect remain distinct; no silently dropped selected Bug.
  - `continuity`: Every delivered member has immutable source/merge identity, preserved ancestry into actual main71e8047, applicable independent review and actual source/result CI/delivery continuity; exemptions and historical failures are preserved.
  - `evidence-applicability`: The nine missing runtime/delivery originals and three early review snapshots each have one bounded recovery result, expected identity and explicit impact on current required criteria. Reuse has scope-aware support; unavailable raw evidence is never reconstructed as a historical PASS or covered by an unauthorized new waiver.
  - `interactions`: Final exact candidate account/restore/route, session/index, scan/publication, health/wire/Widget, read-only Doctor, Hook/CI and integration-resolution interactions are assessed with source/reusable/current evidence; no textual-conflict-only safety inference.
  - `verification`: Required scoped L0 format/link/discovery/review-record/whitespace checks pass, and any uncovered behavior scope has applicable non-destructive verification bound to unchanged code/test/dependency/config/toolchain inputs.
  - `review`: Cold-context independent aggregate review covers the exact final Task2 subject; all in-scope findings are closed; PR46 bounded review and per-issue gates are inputs, not aggregate verdicts.
- `v0-6-5-contract:v0-6-5-contract` / Beads `ad-v065c-contract-dev`:
  - `contract`: Final documented candidate, shipped contracts and approved compatibility exceptions agree: the read-only LaunchServices output exception and only PR24's additive `session_sources.parser_context TEXT NOT NULL DEFAULT ''` column migration. Commands/flags/exits and all non-excepted schemas/persisted formats remain unchanged; actual upgrade, v0.6.0 readback and old/new round-trip safety must pass. No release/version allocation is inferred.
  - `safety`: Account/configuration/data/privacy boundaries and delivered/admin/deferred distinctions are supported; performance FAIL/#43, native/installation/Widget limits and accepted residual risks remain explicit, without new production operation or waiver.
  - `lifecycle`: Task/status/roadmap/document pointers accurately reflect final candidate and uncommitted delivery. Two Widget/three Hook carrier retirements occur only after applicable evidence supports them and preserve every body/link/review round; historical FAIL remains.
  - `verification`: Task3 required L0 contract/link/version/discovery/whitespace checks pass for its final subject; conditional spec/manual/index changes occur only for verified inconsistencies.
  - `review`: Cold-context independent final contract review is PASS for exact Task3/documents with all in-scope findings closed, after actual Task2 required gate VERIFIED.
- `v0-6-5-contract` / Beads `ad-v065c`:
  - `hierarchy`: Actual document gate and Tasks1/2/3 criteria have applicable target-bound evidence in the declared direct hierarchy; Task2 precedes Task3. No historical gate, PR46 integration or Beads status substitutes for child coverage.
  - `coherence`: Final contract/status/member dispositions, delivery boundary and preserved release/native/performance/safety limitations are coherent at the exact topic candidate and independently reviewed; Topic evidence does not claim Release VERIFIED, publication or installed acceptance.

Task3 cannot start until the actual Task2 required gate is VERIFIED. No result
is inferred from registration or historical gates. Candidate evidence must use
HEAD plus the exact scoped subjects/supporting evidence identity, with explicit
reassessment after review/status/retirement changes; delivery remains pending
under this user authorization.

## Earlier Task3 contract assessment — historical 2026-10-06

Actual synchronized boundary results: Task2 accounting VERIFIED6/6; Task3 and
Topic FAILED; current document readiness FAILED. This is an uncommitted candidate
awaiting the external compatibility decision, with all five retirements held.

Task3 began after actual current Task2 VERIFIED6/6 at
`57b2d2cc8cd3fc263b5d3c3bc3acc31caabe64625909bd0e05b95a362b191e2d`
and document VERIFIED3/3. Its final contract result remains incomplete:
`V065-COMPAT-R1-F1` requires an authorized compatibility/version decision before
Task3 and Topic can pass. No new waiver is selected.

### B decision and current compatibility scope — 2026-10-07

The real user's current handoff explicitly approves B: retain `v0.6.5` and add
only the already-delivered PR24 `session_sources.parser_context` column and
additive migration as a narrow version-contract exception. The amended
[primary version contract](../../specs/cli-design.md#version-number-semantics)
and manual disclose the exception; every other schema/format subject retains
the general rule. Decision Gate `ad-v065c-compatibility-gate` was actually read
back `closed`; Task3 and document were `in_progress`, assigned to `codex`.
That decision resolves policy direction, not runtime compatibility. No product
implementation, new waiver, release or production operation is authorized.

The earlier FAILED targets and independent review Rounds 1/2 remain unchanged.
`V065-COMPAT-R1-F1` closed in independent contract Round 3 after actual
compatibility evidence. Independent Round 4 closes the separate lifecycle
finding and scoped L0 gap and passes the pre-retirement contract/document.
The synchronized pre-retirement gates actually passed before moving the five
carriers. Final retired-content Task2/Task3/document/Topic evidence is separately
bound and queried; delivery and Release remain independent boundaries. The old criterion's phrase "compatibility
exception agree" is interpreted against the expressly approved current
exceptions; no historical criterion or observation is rewritten. Current
evidence must explicitly cover the column exception and preserve every
non-excepted invariant, rather than treating policy approval as a passing test.

### Final contract and compatibility

Existing commands/flags/exits and the bounded read-only Doctor addition remain
under [CLI Design](../../specs/cli-design.md#launchservices-registration-diagnosis).
Five Doctor states, optional local registration details, warning/count effects,
500ms/1.5s quick/full acquisition budgets and path-free desktop health remain
bounded. Fixed-output/count scripts need the approved v0.6.5 disclosure; no
automatic OS recovery or executable cleanup action is introduced.

**V065-COMPAT-R1-F1 — CLOSED in independent Round 3:** published v0.6.0
`a5e969d7ad60cfaaee7fec13cbe1704d4cdb2c52` lacked
`session_sources.parser_context`. PR24 source
`3821613a8dbe2d71a0df3f6d988ad4011b74ea4b` / merge
`6e8c1c4d2942782152f5e6b5de89b0942fc57ce4` adds the column in both CREATE TABLE
and an executed ALTER TABLE migration; main71 retains it. The
[primary version rule](../../specs/cli-design.md#version-number-semantics)
previously required MINOR for this migration; the separate 2026-10-07 B decision
now expressly excepts only this column. The Doctor-output exception itself
still grants no database/persisted-format change. Unchanged core schema30 does
not establish unchanged sessions schema. This existing
product/version-contract discrepancy was not introduced by the documentation
batch. Historical PR24 review/gates and its repaired symptom remain facts about
their original scope.

The exact Git diff and isolated existing
`TestSessionParserContextUpgradePreservesIndexAndCore` result are retained in
`/tmp/agentdeck-v065-closeout-20261006/`. The test observes restoring the column
while preserving indexed content/exclusions and core DB bytes. A passing safety
test alone did not establish downgrade or round-trip compatibility. The approved
follow-up uses real v0.6.0/current source and only synthetic isolated state to
verify upgrade, old-version readback, both-version scans and round trips,
indexed documents/exclusions, core DB bytes and account/configuration guards.
The new isolated result is PASS for ten named stages: v0.6.0 seed, additive
upgrade, current scan, v0.6.0 read-only readback, old read/write open and scan,
current-version return, append, old-version rescan and current-version return.
Parser versions switch 5/6; visible FTS documents remain identical across each
round trip (two documents, then three after append), both exclusions remain,
integrity/foreign-key checks pass and old read-only access leaves the index
bytes unchanged. Core DB and synthetic account/configuration guards preserve
exact bytes and 0600 permissions. The test does not exercise real-account
restore or an installed binary. Result/log/snapshot digests and exact source
identities are in operator-local
`/tmp/agentdeck-v065-b-20261007/compatibility-results.json` and
`identity-start.json`. The prepared v060 directory was partial; a fresh
immutable archive was used without changing it. All compiled source/vendor
blobs match the baseline/current commits (2517/2529 files respectively).

Initial no-test overlay and quoted-FTS-query setup failures remain in their
original logs and are excluded from PASS evidence. No product file, test,
dependency, provider, Hook or account setting changed. Independent exact-state
contract re-review and current gates remain required. No production migration
or installation ran; no runtime damage was established by the earlier finding.

### Account, configuration and data safety

- PR27 clears source-account quota state during portable restore while retaining
  user/provider data; live cross-account upgrade acceptance remains separate.
- PR25/26 preserve managed-route ownership, unrelated fields, prior commands,
  compensation and concurrent/deleted-file guards. Current isolated regressions
  support these boundaries without changing live external settings.
- PR24 preserves raw logs, usage and exclusions with transactional index reparse;
  its additive sessions schema change is recorded above. PR28 bounds publication
  reservation without a global locking redesign or live-data repair.
- Widget/health/quota repairs preserve typed failure/source and existing consumers;
  browser/formatter fixtures remain distinct from native UI acceptance.
- Plaintext/derived-key non-zeroing risk, performance FAIL/#43, native icon and
  opt-in schema skip, WidgetTimeline/recovery/collision causality and footer
  unmeasured states retain their existing limits. No new waiver/native PASS exists.

### Lifecycle and release boundaries

Two Widget and three Hook carriers are now historical under `docs/archive/fixes/`,
with `retired: 2026-10-07`. Retirement followed actual pre-retirement Task2 VERIFIED6/6,
document VERIFIED3/3 and Task3 VERIFIED5/5. Every body/review-history byte is
preserved; current links point at the archive. Main separately assesses the
retired metadata/link/body impact and binds the final required gates.
This topic remains active/uncommitted through its delivery boundary. Task2,
Task3 and document independent reviews passed; separately authorized delivery
is still pending, with no fictitious Beads closure or Release acceptance.

Final release SHA, version/build allocation, L4 release-verify, same-SHA preflight,
normal notarized Cask install/upgrade/RC-to-stable/uninstall, real account/config/
data safety and installed-path PlugInKit/configured Widgets remain separately
selected and authorized. No commit/push/PR/merge/tag/release/deployment,
production install or OS registration is executed by this closeout.

## Current B closeout — 2026-10-07

The real-user B scope is implemented only in documentation; HEAD remains
`71e8047e0dd8d638be8587ff1a016c38a9cea635` and there is no product/test/dependency
change. Task2 nineteen-member accounting is reused after explicit impact
assessment. Task3/document independent R3 FAIL and R4 PASS are both retained;
all in-scope findings are closed. Actual synthetic compatibility has ten named
PASS stages, while real-data/install/native/release checks remain unperformed.

Five delivered Widget/Hook records were retired only after actual pre-retirement
required gates passed. Final L0 checks cover the changed and incoming local
links, document discovery, review structure, whitespace and diff; body/history
preservation and current-child roll-ups are assessed for the exact final
candidate. Topic gate evaluation is distinct from its still-open delivery
boundary. The topic stays active pending authorized delivery.

Current manifests, raw checks, review reports and gate envelopes are retained in
operator-local `/tmp/agentdeck-v065-b-20261007/`; they are not portable links or
substitutes for the canonical CE graph. No commit/push/PR/merge/release/install
or OS registration was performed.

## Earlier integration handoff — historical 2026-10-05

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

This is the original planning checkpoint, not a current unfinished-repair list.
PR #22 delivered Task 1; the current member ledger above supersedes its pending
delivery and nineteen-unimplemented statements without changing their history.

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
