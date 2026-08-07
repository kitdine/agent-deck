---
status: active
plan: usage-report-presentation
task: usage-render-primitives
---

# Review log — usage-report-presentation / usage-render-primitives

## Round 1 — 2026-08-07

- Reviewed state: uncommitted worktree on HEAD
  `d447ac0f96964024d09f03d2d8db1b37ff6ce991`; scoped file SHA-256:
  - `usage_stats_text.go`: `9987cc257cb48b0379c3406a838947c4a64ab4789a8574697b84a724c6c22930`
  - `usage_stats_text_test.go`: `dc1f7a1002a245c690ffa87dbd65b2d208fad27db0beeab5c4d4ed25495ed0c9`
  - `usage_text_primitives.go`: `732b906b0dd5fd640208a879d94ef4f54417820daff386abe9af1e909d9963fa`
  - `usage_text_primitives_test.go`: `42c54633f622b12abaea35667c5a530bf8d8f610f861cc3c0548c19b7aeb42fb`
- Reviewer: Codex
- Scope: width and color detection, section-title and bar primitives, aligned
  column helpers, responsive table helpers, stats renderer integration, and
  regression-test protection for the Task 1 byte-identical refactor contract.

## 📋 Review report — usage-render-primitives

### 📊 Overall score: 7/10

### ✅ Verdict: FAIL

### 🔴 Serious issues — must fix

[`cmd/agentdeck/usage_stats_text_test.go:17`] The required multi-width
byte-identical acceptance gate is missing.

- Behavior risk: Task 1 is explicitly a pure refactor whose existing
  `usage stats` output must remain byte-identical at every fixture width. The
  expanded 48/60/72/100/140/160 loop checks required labels and width bounds,
  but would remain green if spacing, wrapping, section placement, or another
  byte changed. The only exact golden assertion at line 929 covers 100 columns.
- Evidence: the plan names golden widths 48, 60, 100, 140, and 160 and makes
  byte identity the Task 1 acceptance gate. Repository search found only
  `testdata/usage-stats-balanced.txt`, consumed by the 100-column golden test.
  All current usage tests pass, demonstrating that the existing suite does not
  close the missing-width evidence requirement.

💡 Bounded remediation: add exact golden/baseline assertions for 48, 60, 100,
140, and 160 columns using the same fixture. Keep 100 if desired, and add the
four missing widths. The expected bytes must be captured from the pre-refactor
renderer or independently normalized against the pre-refactor content state,
not generated from the implementation under test.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- Production changes are narrowly mechanical: terminal option detection,
  section styling, bar tracks, column joining, and responsive width arithmetic
  remain equivalent at their current call sites.
- The new primitive test directly covers uncolored track/title output and the
  responsive fit boundary.
- `TestUsageStatsBalancedTextLayout` adds useful 60- and 160-column width and
  content checks.
- Targeted `TestUsage` package tests pass on the reviewed content state.

### 📝 Summary

Reviewed the four Task 1 files identified above. No production correctness,
security, persistence, or JSON change was found in the mechanical extraction,
and the targeted usage tests pass. Verdict is FAIL because the task's explicit
multi-width byte-identical acceptance gate is not implemented; therefore the
pure-refactor claim is not adequately protected. Broader verification was not
run after this decisive blocking finding.

- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsage -count=1` — PASS.
  - Read-only scoped diff, CodeGraph call-path inspection, plan contract, and
    exact golden-fixture inventory.
- Verdict: REOPEN

## Round 2 — 2026-08-07

- Reviewed state: uncommitted worktree on HEAD
  `d447ac0f96964024d09f03d2d8db1b37ff6ce991`; scoped file SHA-256:
  - `usage_stats_text.go`: `9987cc257cb48b0379c3406a838947c4a64ab4789a8574697b84a724c6c22930`
  - `usage_stats_text_test.go`: `4aa7f0cb0e535b92cd322d1e8d9566fd0f079017c8cc47ffd72f2b2185fc8fec`
  - `usage_text_primitives.go`: `732b906b0dd5fd640208a879d94ef4f54417820daff386abe9af1e909d9963fa`
  - `usage_text_primitives_test.go`: `42c54633f622b12abaea35667c5a530bf8d8f610f861cc3c0548c19b7aeb42fb`
- Reviewer: Codex
- Scope: Round 1 multi-width byte-identical fixture finding, unchanged
  primitive extraction, targeted usage tests, and L2 repository verification.

## 📋 Re-review report — usage-render-primitives

### 📊 Overall score: 10/10

### ✅ Verdict: PASS

### 🔴 Serious issues — must fix

None.

### 🟡 Suggested improvements — recommended

None.

### 🟢 Strengths

- **Closed — Round 1 multi-width byte-identical gate:**
  `TestUsageStatsBalancedBaselinesAtFixtureWidths` now covers the required 48,
  60, 100, 140, and 160 columns with fixed SHA-256 values.
- The baseline constants were independently executed against an exported,
  unmodified HEAD source tree. All five match the pre-refactor renderer, so the
  expected values are not generated from or self-certified by the extraction
  under test.
- Only `usage_stats_text_test.go` changed after Round 1. Production extraction
  and primitive-test hashes remain identical to the reviewed state.
- Targeted usage tests, the L2 repository test suite, and diff check pass.

### 📝 Summary

The sole Round 1 finding is closed. No finding remains open, regressed, or newly
blocking. The fixed multi-width baselines prove the pure refactor preserves the
documented output bytes at every required fixture width. The reviewed candidate
passes targeted and L2 verification; residual risk is limited to the normal
uncommitted-candidate delivery boundary.

- Evidence:
  - Exported unmodified HEAD plus a temporary read-only baseline test:
    `go test -mod=vendor ./cmd/agentdeck -run TestRereviewUsageStatsPreRefactorBaselines -count=1` — PASS.
  - `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck -run TestUsage -count=1` — PASS.
  - `env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...` — PASS.
  - `git diff --check` on scoped product, test, plan, review, and index files — PASS.
- Verdict: PASS
