---
status: active
plan: shell-integration
task: shell-command-surface
---

# Review log — shell-integration / shell-command-surface

## Round 1 — 2026-07-29

- Reviewed state: `2db056ba311719c9841ba76e82f890b6f50d5a84` plus code diff SHA-256 `a42ce580c4767b3b878006bb81eabd5241c2e7dd5c5bef532bf0c11af9a18606`
- Reviewer: Codex
- Scope: `cmd/agentdeck/main.go`, `cmd/agentdeck/contract_test.go`, `cmd/agentdeck/shell_init_test.go`; `shell-command-surface` acceptance and CLI/JSON output contracts
- Findings:
  - [P1] `shell setup`, `shell status`, and `shell remove` use a help-only success handler. A valid invocation exits `0` and prints help without performing the named operation; `--format json` prints the same plain text. `shellLifecycleSurfaceOnlyAnnotation` then excludes these runnable commands from GUI/E2E contract enumeration, so the false success is not protected by the package contract tests. Replace the success handler with an explicit non-zero, standard-envelope unavailable error until tasks 2/3 install real handlers, and add regression coverage proving valid lifecycle commands cannot report success before doing their work.
- Evidence: `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./cmd/agentdeck` passed on the reviewed code state; isolated `go run ... shell setup --shell zsh` and `go run ... --format json shell setup --shell zsh` both returned exit `0` with identical help output and no setup behavior.
- Verdict: REOPEN

## Round 2 — 2026-07-29

- Reviewed state: `2db056ba311719c9841ba76e82f890b6f50d5a84` plus code diff SHA-256 `c05ff8df9623a70ae51df879b28b0b37af91ce46a1466c0f57f62d7e270a9f69`
- Reviewer: Codex
- Scope: Round 1 P1 fix in `cmd/agentdeck/main.go` and `cmd/agentdeck/shell_init_test.go`; text/JSON lifecycle command behavior and contract-test interaction
- Findings:
  - [closed P1] Valid `shell setup`, `shell status`, and `shell remove` invocations now return a non-zero runtime error until their real handlers exist. Text output no longer prints help as success; JSON uses the standard envelope. Regression coverage exercises all three commands in both formats.
  - No new findings.
- Evidence: `go test -count=1 -mod=vendor ./cmd/agentdeck -run TestShellLifecycleSurfaceDoesNotReportSuccessBeforeHandlersExist` passed; isolated text and JSON `shell setup --shell zsh` invocations returned exit `1`, with JSON `command: shell.setup` and `code: runtime_error`; the broader `go test -mod=vendor ./cmd/agentdeck` result passed in 28.360s on the same code diff state.
- Verdict: PASS
