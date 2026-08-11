---
status: active
plan: rc3-terminal-ux-remediation
task: session-interactive-experience
---

- Reviewed state: uncommitted candidate based on `ecebb62361603a77a53c4be6f2524fa0bed3ba55`.
- Reviewer: Codex fresh review pass.
- Scope: root Session browser, structured Overview/Documents/Activity/Tokens detail, semantic color/no-color rendering, responsive geometry, lazy state, and PTY lifecycle.
- Findings:
  - [P2] Token summary or invocation read failures now escape the full-screen viewer instead of preserving the prior durable unavailable/partial state. Convert both failures into an explicit recoverable Tokens page so raw-mode cleanup is not the user's only feedback.
  - [P2] The compact root-browser selected preview starts with redundant selection/client text, so a long model can truncate the Project value even though compact rows omit both Model and Project. Give compact geometry fixed MODEL and PROJECT budgets.
  - [nit] An empty Tokens section reports `PROVIDER COST unpriced · PRICING unpriced` and a `complete` footer. Use `not applicable` plus an explicit `empty` status when there are no normalized invocations.
  - [nit] Multiple durable Tokens warnings are joined without readable spacing. Render warning boundaries with a visible separator.
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck` — PASS on the development candidate.
  - `go test -mod=vendor ./cmd/agentdeck -run TestSessionViewerTokens` — PASS after the in-review warning-state normalization; unrelated tested content unchanged.
  - Complete production and test diff inspected against the Task 3 acceptance contract.
- Verdict: REOPEN

- Reviewed state: repaired uncommitted candidate based on `ecebb62361603a77a53c4be6f2524fa0bed3ba55`.
- Reviewer: Codex fresh re-review pass.
- Scope: Round 1 findings plus the complete root-browser and structured Session detail change.
- Findings: none. Tokens failures remain inside an explicit empty/partial/warning page; compact geometry reserves independent Model and Project value budgets; empty invocation sets are `not applicable` and `empty`; warning boundaries remain readable with and without color.
- Evidence:
  - `go test -mod=vendor ./cmd/agentdeck` — PASS, including Darwin PTY root-to-detail-to-Tokens navigation, Escape return, Ctrl-C, EOF, resize, and terminal-state restoration.
  - `git diff --check` — PASS.
  - Complete repaired production and regression-test diff re-inspected; JSON and ordinary Session text routes are unchanged.
- Verdict: PASS
