---
status: active
plan: shell-integration
task: activation-and-eligibility-status
---

# Review log — shell-integration / activation-and-eligibility-status

## Round 1 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed file-set SHA-256 `4f616ca91731d0a2e6c996fda0804ba94a11e7ac222682f139413d4c17a5c651`
- Reviewer: Codex
- Scope: `cmd/agentdeck/main.go`, `cmd/agentdeck/shell_init_test.go`, `internal/provider/service.go`, `internal/provider/project_test.go`, `internal/shellconfig/config.go`, `internal/shellconfig/status.go`, `internal/shellconfig/status_test.go`; Task 3 activation, configuration inspection, route eligibility, text/JSON, privacy, and read-only contracts
- Findings:
  - [P2] No-argument `shell status` does not apply the setup `in-use` selection rule. `Manager.targets` deliberately returns zsh/fish and one fallback bash target with `selected: false` so setup can report skipped entries, but `Manager.Status` inspects and emits every target without checking `selected`. A user invoking zsh with only `.zshrc` therefore also sees absent fish and bash entries even though those startup files do not exist and neither shell is in use. `TestShellStatusTextAndJSONReportConfigurationActivationAndEligibilityOnce` currently requires at least three shell results and protects the incorrect behavior. Filter unselected targets only for no-argument status while retaining explicit missing-shell status as one `absent` result. Add Manager and CLI regressions for invoking-only selection, an additional existing startup file, and explicit missing-shell inspection.
- Test review:
  - Strong coverage directly exercises generated bash/fish/zsh marker PIDs, configured/absent/modified/invalid states, inactive and inherited markers, at-most-one active shell including bash login/non-login startup files, all route eligibility reasons, unreadable-state diagnostics, and text/JSON privacy.
  - The remaining gap is concentrated in no-argument in-use filtering; the current renderer test asserts the opposite behavior.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run 'Test(ProjectRouteEligibilityExplainsEveryRouteState|RunProjectEnvironmentRequiresAHeadroomViaRouteAndHonorsUserValues)$' -v ./internal/provider` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run 'TestStatusReports(ConfigurationAndActivationWithoutExposingModifiedBytes|AtMostTheInvokingShellActive|OnlyTheInvokingBashStartupFileActive)$' -v ./internal/shellconfig` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run 'TestShell(StatusTextAndJSONReportConfigurationActivationAndEligibilityOnce|StatusReportsUnreadableStateAsUndeterminedWithoutCreatingIt|InitScriptsMarkTheEvaluatingShellProcess)$' -v ./cmd/agentdeck` passed.
  - The immediately preceding development evidence on the same unchanged product/test file set includes the targeted three-package suite and full vendored suite.
- Verdict: REOPEN

## Round 2 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed file-set SHA-256 `14cfce65790f6f1853d48f690f956a3316426a6dc6e849608aa8d99857e4ebca`
- Reviewer: Codex
- Scope: Round 1 no-argument in-use filtering finding and regression risk in `internal/shellconfig/status.go`, `internal/shellconfig/status_test.go`, and `cmd/agentdeck/shell_init_test.go`
- Finding resolution:
  - [CLOSED] `Manager.Status` now skips `selected: false` placeholder targets only when no shell is explicitly requested. Invoking-only status reports one shell, another existing default startup file adds that shell, and an explicit missing shell remains one `absent` result.
  - [CLOSED] Manager and CLI tests cover all three required selection scenarios; the previous CLI assertion requiring at least three shells was removed.
- Findings: none at P1 or P2 severity.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run '^TestStatusFiltersDefaultTargetsButKeepsExplicitMissingShell$' -v ./internal/shellconfig` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run '^TestShellStatusTextAndJSONReportConfigurationActivationAndEligibilityOnce$' -v ./cmd/agentdeck` passed.
  - Reused evidence from the immediately preceding fix workflow on the same unchanged product/test state: Task 3 targeted three-package tests and the full vendored suite passed.
- Verdict: PASS
