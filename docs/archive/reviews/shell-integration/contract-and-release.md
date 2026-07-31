---
status: historical
plan: shell-integration
task: contract-and-release
retired: 2026-07-31
---

# Review log — shell-integration / contract-and-release

## Round 1 — 2026-07-30

- Reviewed state: `a75a0c6` plus reviewed file-set SHA-256
  `0a52b3dfcd3745ca7b40e87461d9bea14fc1ac4e6d3ca42277442b16e743b55d`
- Reviewer: Codex
- Scope: Task 8's changes in `docs/specs/cli-design.md`,
  `docs/specs/cli-manual.md`, `docs/plans/runtime-provider-attribution.md`,
  `docs/plans/shell-integration.md`, and `docs/README.md`, checked against the
  reviewed Task 1-7 records and the current shellconfig, provider-gate,
  renderer, advisory, and backup implementation.
- Findings:
  - [P1] `docs/specs/cli-design.md:823-826` gives the reusable lifecycle an
    unconditional atomic multi-file rollback guarantee. The explicit
    `agentdeck shell setup` path calls `Manager.Setup`
    (`cmd/agentdeck/main.go:733-737`), which intentionally processes and
    reports targets independently (`internal/shellconfig/config.go:137-160`);
    only interactive switch-time setup calls the all-or-nothing
    `Manager.SetupIfUnconfigured` path (`cmd/agentdeck/main.go:1600-1608`,
    `internal/shellconfig/config.go:163-167`). The living contract therefore
    promises rollback that the public lifecycle does not implement and also
    conflicts with its preceding continue-after-failure rule. Limit the
    all-or-nothing guarantee to automatic `provider use --via` setup, and state
    that explicit multi-target setup preserves successful targets while
    reporting all failures and returning a failing overall status.
  - [P2] The stable status vocabulary is inconsistent with the implemented
    output and with the v0.2.2 consumer plan. `docs/specs/cli-design.md:812-814`
    and `docs/specs/cli-manual.md:174` name the activation state `inherited`,
    while JSON exposes `inherited_from_ancestor`
    (`internal/shellconfig/status.go:19-24`) and text renders
    `inactive (marker inherited from ancestor shell)`
    (`cmd/agentdeck/main.go:909-912`). Meanwhile
    `docs/plans/runtime-provider-attribution.md:114-123` still says the shared
    persistent vocabulary is `configured`, `absent`, `drifted`, and
    `conflicting`, contradicting the new living contract's
    `absent/configured/modified/invalid`. Record the exact JSON and text
    activation forms, and make the hook plan inherit the four persistent state
    names from the living contract unless it explicitly documents a justified
    divergence.
  - [P2] The negative-gate repair wording is stronger than the implementation.
    `docs/specs/cli-design.md:221-228` and `:1588-1593`, plus
    `docs/specs/cli-manual.md:195-201`, say a later provider switch repairs or
    reconstructs the marker. `Service.UseCredential` deliberately ignores
    `RefreshProjectAttributionGate` failure after the selection commits
    (`internal/provider/service.go:1007-1011`), so a permission or durability
    failure leaves the marker missing or stale while the provider switch still
    succeeds. Describe refresh as best-effort: failure never rolls back or
    fails the completed switch, inconsistent state remains diagnosable through
    `shell status` and `doctor`, and a later successful refresh may repair it.
- Test and evidence review:
  - Task 8 is L0. No Go, installer, race, release, or artifact-install gate is
    required or treated as documentation proof.
  - `rtk git diff --check` passed on the reviewed state.
  - Targeted source checks confirmed the public resolver alias, hidden
    `shell-init`, managed-block presence guards, switch-time setup conditions,
    marker path/mode, portable-backup allowlist, advisory paths, and
    version-21/changelog agreement.
  - The plan matrix remains Task 8 Dev `[x]`, Review `[ ]`; `docs/README.md`
    remains `8/8 developed, 7/8 reviewed`, so the active status is consistent
    with this verdict.
- Positive notes:
  - The version was derived from the delivery-time value rather than
    hard-coded in the plan, and the changelog row matches it.
  - Resolver compatibility, presence-guard/uninstall behavior, in-use shell
    selection, switch-time write gates, corrected route advisories, cost shape,
    and portable-backup exclusion all have clear living-spec and manual
    coverage.
  - The runtime-attribution plan now links the living lifecycle contract rather
    than establishing a separate authority.
- Verdict: REOPEN

## Round 2 — 2026-07-31

- Reviewed state: `a75a0c6` plus reviewed file-set SHA-256
  `1fbf37f62e7c9578afa0366db0cd4a20508a03978f729020c27bd05e05071680`
- Reviewer: Codex
- Scope: Round 1's three findings in `docs/specs/cli-design.md`,
  `docs/specs/cli-manual.md`, and
  `docs/plans/runtime-provider-attribution.md`; Task 8's delivery-time version
  coordination with the other active specification-changing plan,
  `docs/plans/credential-and-pricing-hardening.md`; and the plan/index
  retirement gate.
- Finding resolution:
  - [CLOSED] The living lifecycle now distinguishes the explicit
    `Manager.Setup` behavior from automatic `SetupIfUnconfigured`: explicit
    multi-target setup retains successful targets, reports every failure, and
    returns a failing overall status, while all-or-nothing rollback is promised
    only for automatic switch-time setup. The manual explicitly says successful
    targets are retained.
  - [CLOSED] Status vocabulary now matches the implementation. JSON uses
    `active`, `inactive`, and `inherited_from_ancestor`; text renders the last
    case as `inactive (marker inherited from ancestor shell)`. The v0.2.2 Hook
    plan now inherits `absent/configured/modified/invalid` in both its status
    description and reusable-lifecycle paragraph, and requires any future
    divergence to be explicit and justified.
  - [CLOSED] Marker refresh is now documented as best-effort after selection
    commit. The living spec and manual state that refresh failure neither
    rolls back nor fails the completed provider switch, leaves missing/stale
    state diagnosable, and only a later successful refresh reconstructs the
    marker after restore.
- Findings:
  - [P2] The other active specification-changing plan still hard-codes the
    version Task 8 has now consumed. `docs/plans/shell-integration.md:1224-1225`
    explicitly requires delivery-time increment rather than hard-coding
    version 21 because another active plan shared it, and the living
    specification is now version 21. However,
    `docs/plans/credential-and-pricing-hardening.md:16-17` still says its
    contract changes raise the specification to version 21, and its
    `contract-record` task repeats the same instruction at `:309-310`.
    Executing that plan would add contract changes without increasing the
    version. Replace both occurrences with the delivery-time rule already used
    by this plan and `runtime-provider-attribution.md`: increment from whatever
    specification version is current when that contract task is delivered.
- Evidence:
  - `rtk git diff --check` passed before this review-artifact update.
  - Version/changelog discovery found exactly one frontmatter `version: 21`
    and one top version-21 changelog row.
  - Targeted source comparison reconfirmed the explicit/automatic setup split,
    activation wire/text values, and ignored best-effort gate-refresh error.
  - Targeted document checks found no short `inherited` state or old shared
    `missing/drifted/conflicting` vocabulary in the repaired lifecycle
    sections.
  - Task 8 remains Dev `[x]`, Review `[ ]`; `docs/README.md` remains
    `8/8 developed, 7/8 reviewed`; the plan remains active and unarchived.
- Positive notes:
  - All three Round 1 findings are closed without expanding product behavior or
    weakening the L0/L4 boundary.
  - The remaining finding is a narrow active-plan coordination defect, not a
    defect in the shipped shell contract or implementation.
- Verdict: REOPEN

## Round 3 — 2026-07-31

- Reviewed state: `a75a0c6` plus reviewed file-set SHA-256
  `789c418f9e4ff4fce31044c8dacdb76f3b4b8a6a92440de7a5ecf3bdb2e11710`
- Reviewer: Codex
- Scope: Round 2's active-plan version-collision finding in
  `docs/plans/credential-and-pricing-hardening.md`, plus the unchanged Task 8
  living contracts, plan matrix, documentation index, and retirement gate.
- Finding resolution:
  - [CLOSED] Both hard-coded version-21 instructions now require
    `hardening-contract` to increment `docs/specs/cli-design.md` once from
    whatever version is current at delivery. The same delivery-time rule is
    now used by all three active specification-changing plans, while the living
    specification and its top changelog row remain version 21.
- Findings: none at P1 or P2 severity.
- Evidence:
  - `rtk git diff --check` passed.
  - The targeted legacy-pattern check found no `specification version 21` or
    `to version 21` in `credential-and-pricing-hardening.md`.
  - Discovery across `credential-and-pricing-hardening.md`,
    `runtime-provider-attribution.md`, and `shell-integration.md` found the
    delivery-time increment rule in every specification-changing contract
    task.
  - The Task 8 L0 evidence remains correctly scoped: no L4 release gate,
    product test, or released-artifact installation is treated as proof of the
    documentation contract.
- Positive notes:
  - The fix changed only the two stale version instructions and did not
    prematurely modify the living specification.
  - All Round 1 and Round 2 findings are closed, and no active-plan version
    collision remains.
- Verdict: PASS
