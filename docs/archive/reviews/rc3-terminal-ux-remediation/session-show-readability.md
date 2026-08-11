---
status: historical
plan: rc3-terminal-ux-remediation
task: session-show-readability
retired: 2026-08-11
---

- Reviewed state: uncommitted candidate based on `253ef4aaf4eea7a60e5a8ddd264cc42f7f754eef`.
- Reviewer: Codex fresh review pass.
- Scope: ordinary `session show` metadata, Documents, Activity, Tokens, invocations, pagination, geometry, sanitization, JSON separation, and living contracts.
- Findings:
  - [P2] Activity detail rows disappear when `ActivitySummary` is nil and `--activity` is not reflected in the local boolean, even though safe detail values are present. Preserve the prior `len(value.Activity) > 0` section gate.
  - [P2] Summary and invocation pricing classification ignore a non-nil catalog-base cost when provider cost and known subtotals are unavailable, incorrectly reporting catalog-only priced data as `unpriced` instead of `partial`.
  - [nit] The document wrapping test checks only a repeated prefix, so truncating the latter part of approved text would still pass. Add a unique end marker assertion.
  - [nit] The visible-width matrix covers CJK and emoji but not combining marks required by the terminal contract. Add combining-mark content to the same geometry oracle.
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck` — PASS on the development candidate.
  - Complete Task 4 production, test, manual, and terminal-contract diff inspected.
  - `git diff --check` — PASS.
- Verdict: REOPEN

## Round 2 — 2026-08-11

- Reviewed state: repaired uncommitted candidate based on `253ef4aaf4eea7a60e5a8ddd264cc42f7f754eef`.
- Reviewer: Codex fresh re-review pass.
- Scope: all Round 1 findings plus the complete ordinary `session show` text renderer, tests, pagination, JSON separation, and living terminal contracts.
- Findings: none. Activity details remain visible without an aggregate summary; catalog-only cost is classified `partial`; document assertions prove the unique tail survives wrapping; combining-mark geometry is covered.
- Evidence:
  - `go test -mod=vendor ./...` — PASS on the final production candidate.
  - `go test -mod=vendor -race ./...` — PASS on the final production candidate.
  - `go vet -mod=vendor ./...` — PASS on the final production candidate.
  - Current compiled binary `/private/tmp/agentdeck-rc3-task4` — isolated-real-state Session and Usage scans PASS without modifying source logs.
  - Real `session show` at 48, 100, and 140 columns — all sections ordered, metadata explicit, UTF-8 valid, no ANSI/control leakage, and zero visible-width overflow.
  - Real JSON output at 48, 100, and 140 columns — `data` and shape identical; independent invocations differ only in the expected top-level `generated_at` envelope field.
  - Real PTY root browser → detail → `[TOKENS]` → Escape → q — PASS; alternate-screen enter/exit balanced and transcript mode `0600`.
- Verdict: PASS

## Round 3 — 2026-08-11 release-preflight

- Reviewed state: committed tree `044b9ac2d8425885b888fbea7f1cfffcf05e37ca`, commit `61f79c3127a08d11235d6c6053b30439587c5db2`.
- Evidence: release-preflight run `31490703746` failed in fresh `make release-verify`; all four failing test groups reproduce locally with `-count=1`.
- Findings:
  - [P1] Documents, Activity, and Invocations localize record-level timestamps without naming the display zone, violating the human-readable time contract.
  - [P1] Pagination, duration, metadata-label, and invalid-time assertions still bind to the superseded Session renderer, so cached local tests concealed deterministic fresh-suite failures.
- Classification: mixed production and test defects; CI/macOS-only, toolchain, and flaky alternatives rejected by the deterministic local reproducer.
- Verdict: REOPEN

## Round 4 — 2026-08-11

- Reviewed state: repaired uncommitted candidate based on `61f79c3127a08d11235d6c6053b30439587c5db2`.
- Reviewer: Codex fresh re-review pass.
- Scope: both Round 3 findings, all affected Session text routes, fresh-suite assertions, time-zone and invalid-time branches, JSON separation, and living contracts.
- Findings: none. Parseable Documents, Activity, and Invocations timestamps carry the complete display zone in wrapping values; invalid or empty record times remain `—` without a fabricated zone. Pagination, grouped durations, summaries, metadata labels, and next-page commands are asserted against the new grammar without weakening privacy checks.
- Evidence:
  - All five original/new focused tests with `-count=1` — PASS.
  - Final-content `make release-verify` — PASS.
  - Current compiled binary `/private/tmp/agentdeck-rc3-preflight-fix` with reused isolated-real-state index — 48/100/140-column text has zero overflow, all sections present, and complete zone markers.
  - Stable historical real session JSON at 48/100/140 columns — payload identical after excluding only the per-command `generated_at` envelope field.
  - A live-session JSON difference was limited to newly appended Activity data between reads; same-time 48/100 payloads were identical and the stable-source three-width control rejected a geometry regression.
- Verdict: PASS
