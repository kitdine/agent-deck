---
status: active
plan: rc3-terminal-ux-remediation
task: session-model-index
---

# Review log — rc3-terminal-ux-remediation / session-model-index

## Round 1 — 2026-08-11

- Reviewed state: uncommitted candidate based on `cb2cd7f4c5aa1c9880d357bdee1948865f7775c1`.
- Reviewer: Codex fresh review pass.
- Scope: Claude model extraction, parser-version re-read, List/Show metadata,
  regression coverage, plan and documentation index.
- Findings:
  - [P1] The first implementation allowed nested `message.model` through a
    generic metadata helper used by Codex. Narrow it to Claude assistant records
    so untrusted or unrelated nested message shapes cannot affect Codex metadata.
  - [nit] `docs/README.md` listed the new plan twice. Keep one authoritative row.
- Evidence: focused Session tests passed; `git diff --check` passed; complete
  task diff inspected.
- Verdict: REOPEN

## Round 2 — 2026-08-11

- Reviewed state: repaired uncommitted candidate based on
  `cb2cd7f4c5aa1c9880d357bdee1948865f7775c1`.
- Reviewer: Codex fresh re-review pass.
- Scope: Round 1 findings plus the complete Claude parser-version repair.
- Findings: none. Nested model extraction is Claude-assistant-only, a user
  message cannot supply the model, and an unchanged legacy-version source is
  reparsed before List and Show expose metadata.
- Evidence:
  - `go test -mod=vendor ./internal/session -run TestScanClaudeIndexesNestedModelAndRereadsLegacyParserVersion` — PASS.
  - `go test -mod=vendor ./internal/session -run TestScanClaudeAllowlistAndExclusion` — PASS.
  - `go test -mod=vendor ./internal/session` — PASS.
  - `git diff --check` — PASS.
- Verdict: PASS
