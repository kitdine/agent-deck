---
status: active
plan: rc3-terminal-ux-remediation
task: usage-visual-system
---

- Reviewed state: uncommitted candidate based on
  `3a175beb135ff4785a037b4efc2bb61ea9961f83`.
- Reviewer: Codex fresh review pass.
- Scope: all-range Activity generation, Usage text and interactive palette,
  semantic bars, responsive viewer layout, no-color behavior, and regression
  coverage.
- Findings:
  - [P2] The new Activity tab is covered only below the PTY boundary. Add a
    real Darwin PTY navigation test that decodes terminal keys, reaches the
    Activity tab, observes the 1H heatmap, and exits cleanly.
  - [nit] Removing the old range gate left `naturalDayCount` unreachable in
    production. Remove the stale helper so the code no longer carries an
    obsolete heatmap-suppression policy.
  - [nit] `usageViewerSections` is an independently sized slice even though
    section-local state now uses `usageViewerSectionCount`. Bind the tab table
    to that compile-time count and make Coverage an explicit switch case so a
    future enum/table drift cannot silently render the wrong section.
- Evidence:
  - `go test -mod=vendor ./internal/usage ./cmd/agentdeck` — PASS.
  - `git diff --check` — PASS.
  - Complete task diff inspected; 48/100/140-column visual candidates sampled.
- Verdict: REOPEN

- Reviewed state: repaired uncommitted candidate based on
  `3a175beb135ff4785a037b4efc2bb61ea9961f83`.
- Reviewer: Codex fresh re-review pass.
- Scope: Round 1 findings plus the complete all-range Activity and bright Usage
  visual-system change.
- Findings: none. Activity is compile-time integrated as the eighth section,
  its real PTY route renders after terminal key decoding, and the obsolete
  range-suppression helper is gone. Absolute KPIs have no track; trend, share,
  cache, and coverage tracks require a defined comparison basis.
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck -run '^TestRunUsageStatsViewerPTYNavigatesToActivityHeatmap$' -count=1 -v` — PASS.
  - `go test -mod=vendor ./internal/usage ./cmd/agentdeck` — PASS.
  - `git diff --check` — PASS.
- Verdict: PASS
