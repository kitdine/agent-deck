---
status: historical
plan: shell-integration
task: switch-time-setup
retired: 2026-07-31
---

# Review log — shell-integration / switch-time-setup

## Round 1 — 2026-07-30

- Reviewed state: `5b622b21edc322a0ca815fbbc303c1dfa554fc79` plus reviewed file-set SHA-256 `5b8064b1e1d86e2de909cdd0ced0bbe8fe496f6f1004d18930174569dceb9359`
- Reviewer: Codex
- Scope: `cmd/agentdeck/main.go`, `cmd/agentdeck/switch_time_shell_setup_test.go`, the Task 7 adjustments in `cmd/agentdeck/shell_init_test.go` and `internal/shellconfig/status_test.go`, Task 2's managed-block editor, Task 6's generated wrapper and eligibility/advisory path, and Task 7's plan scope and acceptance criteria
- Findings:
  - [P1] Automatic setup is state-root blind. `provider use --via` completes against `opts.stateRoot()`, but `maybeSetupShellIntegration` calls the existing editor without that state root (`cmd/agentdeck/main.go:1578`), whose managed body invokes bare `agentdeck shell-init` (`internal/shellconfig/config.go:716-724`). The activation line also emits a bare command (`cmd/agentdeck/main.go:1598-1603`, `:1705-1710`), and even a manually state-root-qualified `shell-init` generates wrapper resolver calls that omit `--state-dir` (`cmd/agentdeck/main.go:1168,1182,1197,1206`). With `--state-dir <custom>`, the switch, provider selection, and negative gate live under `<custom>`, while new sessions and the advertised current-session command read the default `~/.agentdeck` store. The command therefore reports that new shell sessions are covered although attribution is inert or resolves a different provider state. Thread the active state root through the managed block, activation command, and both generated resolver calls with shell-safe quoting, preserving the default-root path and managed-block upgrade rules.
  - [P1] The failure rollback does not uphold the no-partial-write contract and bypasses Task 2's safety guarantees. `rollbackAutomaticShellSetup` ignores every `Manager.Remove`/cleanup error (`cmd/agentdeck/main.go:1616-1635`), then uses an unchecked `ReadFile`/`os.Remove` pair for a path that was originally missing. A rollback failure silently leaves a newly configured block or empty startup file, while a concurrent replacement between the read and remove can be deleted. This path has neither Task 2's ownership/snapshot checks nor directory durability sync. Move automatic multi-file setup/rollback into a shellconfig all-or-nothing primitive, or add an equivalent exact-state rollback using the editor; never delete a startup path through raw best-effort `os.Remove`. Cover both rollback failure and concurrent-change preservation while keeping the completed provider switch successful and degrading only to the advisory.
- Test review:
  - The first-switch/second-switch tests cover all in-use zsh, fish, and bash targets, per-file reporting, unchanged bytes on the second switch, and the current-session activation sentence.
  - Non-TTY, quiet, JSON, NDJSON rejection, invocation-only `--no-shell-setup`, persisted remove/setup preference, tampered block, successful multi-target rollback, and status-level wrong-ownership rejection are covered.
  - The tests use an explicit `--state-dir` but inspect only emitted block text; they never source the resulting wrapper against a deliberately different default state, so the first P1 remains invisible.
  - The rollback test exercises only a successful cleanup. It does not inject cleanup failure or a path replacement between setup and rollback, so the second P1 remains invisible.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./cmd/agentdeck ./internal/shellconfig` passed.
  - Reused from the immediately preceding unchanged product/test state: Task 7 targeted `./cmd/agentdeck ./internal/shellconfig ./internal/provider`, full `go test -count=1 -mod=vendor ./...`, relevant three-package `-race`, and `go vet -mod=vendor ./...` all passed.
  - Custom-state reproduction used an isolated temporary HOME and binary. `shell setup --state-dir <custom>` wrote `command agentdeck shell-init zsh` with no state root. `shell-init zsh` without the flag embedded `<HOME>/.agentdeck/project-attribution.enabled`, while the qualified command embedded `<custom>/project-attribution.enabled`; the generated resolver calls remained unqualified in both cases.
- Out of scope: the managed wrapper's missing `agentdeck` PATH guard remains assigned to Task 5 and is not a Task 7 finding.
- Verdict: REOPEN

## Round 2 — 2026-07-30

- Reviewed state: `5b622b2` plus reviewed file-set SHA-256 `d8dd55e31a35ab2c0ec012d527083023b0f808c8c303b3d52ec05d91e735a69f`
- Reviewer: Claude Opus 5
- Scope: Round 1's two P1 findings in `cmd/agentdeck/main.go`, `cmd/agentdeck/switch_time_shell_setup_test.go`, `cmd/agentdeck/shell_init_test.go`, `internal/shellconfig/config.go`, `internal/shellconfig/config_test.go`, `internal/shellconfig/status.go`, `internal/shellconfig/status_test.go`
- Finding resolution:
  - [CLOSED] The active state root is now threaded through all three paths the finding named. `Environment.StateRoot` reaches `managedBody`, which appends a quoted `--state-dir` to the block's `command agentdeck`; `shellInitScript` takes the state root and applies it to both generated resolver calls as well as the gate path; and `shellActivationCommand` takes it for the current-session line at both call sites. `managedVersion` moved to 2, with `legacyManagedBody` keeping released version-1 blocks valid so they upgrade rather than being refused as modified, and `validManagedBodyForShell` accepting only a body whose quoted argument round-trips through the same escape. Verified end to end with a built binary against a state root named `cus'tom`: the written block and the generated script both carry `--state-dir '…/cus'"'"'tom'`, `zsh -n` accepts both, and sourcing the script defines `codex` and `claude`.
  - [CLOSED] Automatic setup and rollback moved inside the editor. `Manager.SetupIfUnconfigured` prepares every target — replacement and same-directory backup — before committing any rename, verifies each target is unchanged immediately before commit, and on any commit, inspection, or directory-sync failure rolls previously committed targets back in reverse order. `restorePreparedSetup` restores an existing target from its backup and, for a target that was originally missing, renames the installed file back to its temporary path before removing it, so no startup path is ever deleted through a raw best-effort `os.Remove`; `rollbackAutomaticShellSetup` and its unchecked `ReadFile`/`os.Remove` pair are gone from `cmd/agentdeck/main.go`. Rollback failures are joined into the returned error and surfaced per file, and `cleanupPreparedSetup` deliberately keeps artifacts for a target whose rollback failed.
- Findings:
  - [P2] The unqualified default-root form the fix built is unreachable, so every user's managed block now hardcodes an absolute home path. `managedBody` emits bare `command agentdeck` only when `Environment.StateRoot` is empty, and `validManagedBodyForShell` explicitly accepts that form, but all three construction sites (`cmd/agentdeck/main.go:730`, `:1568`, `:1662`) pass `opts.stateRoot()`, which returns `~/.agentdeck` rather than an empty string when `--state-dir` was not given. Verified: a plain `shell setup` in an isolated `HOME` writes `command agentdeck --state-dir '<home>/.agentdeck' shell-init zsh`. Nothing breaks on the machine that ran setup, and the completion installer already writes absolute paths, so this is a design decision rather than a defect — but it is currently an accidental one: the block stops following `HOME`, so synced dotfiles or a changed home silently point the wrapper at another user's state root, and the bare branch plus its validation arm read as intent that no caller honors. Either pass an empty state root when the user did not supply `--state-dir`, so default installations keep a home-independent block, or delete the bare-form branch and its acceptance arm and record in the plan that the block deliberately pins an absolute state root.
- Test review:
  - Round 1's two invisibility gaps are closed by tests that would fail if the fixes regressed. `TestProviderUseViaBindsCustomStateRootAcrossManagedActivationAndWrappers` covers the switch-time path against a deliberately non-default state root; `TestSetupUpgradesCompatibleManagedBlocksToRequestedStateRoot` covers both a released version-1 block and a version-2 block bound to a different state root, upgrading each and then asserting the second run is `Unchanged`, with a state root containing a single quote.
  - Rollback now has adversarial coverage rather than only the happy path: `TestSetupIfUnconfiguredRollsBackCommittedMissingTarget`, `TestSetupIfUnconfiguredReportsRollbackCleanupFailureAfterRestoringTargets`, `TestSetupIfUnconfiguredReportsRollbackReplacementFailureAfterFallbackRestore`, and `TestSetupIfUnconfiguredPreservesConcurrentReplacement` cover injected cleanup failure, injected replacement failure with fallback restore, and a concurrent change that must be preserved rather than clobbered.
  - The task's own acceptance list remains covered from Round 1: first/second switch, per-file reporting, non-TTY, `--quiet`, JSON, NDJSON, `--no-shell-setup`, the persisted declined preference, and the tampered-block refusal that writes nothing.
  - No test pins the default-root block content, which is why the P2 above is a decision left implicit rather than a covered contract.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./...` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -race ./cmd/agentdeck ./internal/shellconfig ./internal/provider` passed.
  - Custom-state reproduction of Round 1's first finding, now negative: built `./cmd/agentdeck`, ran `shell setup --state-dir "<tmp>/cus'tom"` in an isolated `HOME`; the block, the generated wrapper's gate test, and both resolver calls all carry the quoted custom root, `zsh -n` accepts the rc file and the script, and sourcing defines both wrappers.
  - Default-root observation for the P2: a plain `shell setup` in a second isolated `HOME` embedded `--state-dir '<home>/.agentdeck'`.
- Verdict: REOPEN

## Round 3 — 2026-07-30

- Reviewed state: `5b622b2` plus reviewed file-set SHA-256 `8ffd2a274b8e5cc046d25c8fea33f1650bbeb577837056aa9f964afb215560cc`
- Reviewer: Claude Opus 5
- Scope: Round 2's default-root finding in `cmd/agentdeck/main.go`, `cmd/agentdeck/switch_time_shell_setup_test.go`, and `internal/shellconfig/config.go`
- Finding resolution:
  - [CLOSED] Option A. `commandOptions.shellStateRoot` (`cmd/agentdeck/main.go:1264`) returns the raw `--state-dir` value, so it is empty exactly when the user did not supply one, and it now feeds all three `shellconfig.Environment` construction sites (`:727`, `:1570`, `:1661`), `shellInitScript` (`:1104`), and `shellActivationCommand` (`:1596`, `:1631`). The gate path still comes from the resolved `stateDir`, so the wrapper's `test -f` keeps an absolute path while the qualifier disappears. Verified with a built binary in two isolated homes: a plain `shell setup` writes `eval "$(command agentdeck shell-init zsh)"` with no state root, its generated script's resolver call is likewise unqualified while the gate path is the resolved `<home>/.agentdeck/project-attribution.enabled`, a `--state-dir "<tmp>/cus'tom"` setup writes the quoted qualified form, and a second default `shell setup` reports `unchanged`.
- Findings: none at P1 or P2 severity.
- Test review:
  - `TestDefaultAndCustomShellStateRootFormsArePortableAndIdempotent` pins the decision as a contract rather than leaving it implicit: it compares the extracted managed body byte-for-byte against the expected bare and qualified forms, asserts the startup file is unchanged on a second setup for both, asserts the generated script omits `--state-dir` for the default root and contains the quoted root for a custom one, and checks both activation-command forms. A regression to either form fails it.
  - Round 1's and Round 2's closed findings keep their coverage: the custom-root switch-time test, the version-1 and version-2 upgrade cases, and the four adversarial `SetupIfUnconfigured` rollback tests are unchanged.
  - Residual, not a finding: the default-root case asserts only the absence of `--state-dir` in the generated script and does not also pin the resolved gate path there; that path is covered by `TestShellInitQuotesGatePathAndDefinesBothWrappers` and was confirmed manually.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./...` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -race ./cmd/agentdeck ./internal/shellconfig ./internal/provider` passed.
  - Two-home binary reproduction described above, including the idempotent `unchanged` rerun.
- Verdict: PASS
