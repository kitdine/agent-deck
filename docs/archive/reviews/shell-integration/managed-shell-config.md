---
status: historical
plan: shell-integration
task: managed-shell-config
retired: 2026-07-31
---

# Review log — shell-integration / managed-shell-config

## Round 1 — 2026-07-29

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed file-set SHA-256 `8391664210b795714d906131b0a9fd10c4c87378632ecd513fbc371f0f0be7fb`
- Reviewer: Codex
- Scope: `cmd/agentdeck/main.go`, `cmd/agentdeck/shell_init_test.go`, `internal/shellconfig/`, `scripts/test-completion-install.sh`; Task 2 acceptance, startup-file mutation safety, CLI result contracts, and regression value
- Findings:
  - [P1] `atomicReplace` renames the temporary file over the startup file before opening and syncing the parent directory. If `openDir` or directory `Sync` fails, the function returns an error even though the startup file has already changed, with no rollback path. This violates the atomic editor's failure contract and can make a multi-shell run report a shell as failed after configuring it. Preserve enough original state to roll back post-rename failures, or otherwise redesign the result contract so a completed mutation cannot be reported as an unchanged failure. Add injected `openDir` and directory-sync failure tests for both existing and previously missing startup files, asserting the reported outcome matches the final bytes.
  - [P2] No-argument `shell remove` unnecessarily requires successful invoking-shell detection. The CLI returns the detection error before calling `Manager.Remove`, and `Manager.targets` independently rejects an empty invocation. Removal can discover every existing default startup file without knowing the invoking shell, so an invocation from `sh`, automation, or an unrecognized parent chain fails instead of clearing configured blocks. Separate setup target selection from remove target selection and add manager plus CLI regressions where detection fails but default zsh, fish, and bash blocks are still removed and reported.
  - [P2] The wrong-ownership regression replaces `Manager.ownsFile` with a function that always returns false, so it proves only that `readStartup` honors the seam. A regression in the production Unix UID extraction — including an implementation that always returns true — would leave the test green and permit mutation of another user's regular file. Add a Unix-specific unit test that exercises `ownedByCurrentUser` with `syscall.Stat_t` values for the current UID and a different UID.
- Test review:
  - Strong coverage exists for idempotence, missing final newlines, marker/hash conflicts, symlink refusal, pre-rename write/sync/rename failures, multi-shell continuation, completion-block coexistence, and absent/present-failing AgentDeck guards.
  - Missing behavioral protection is concentrated in post-rename failures, removal without invocation detection, and the real Unix ownership predicate described above.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./internal/shellconfig` passed in 4.998s.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./cmd/agentdeck` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build bash scripts/test-completion-install.sh` passed.
  - The earlier L3 development evidence on the same unchanged product/test file set also includes full vendored tests, targeted race, full vet, and the completion installer integration script.
- Verdict: REOPEN

## Round 2 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed file-set SHA-256 `67a951efa2c2c1e611bae7838155edc7382a807642033025feb1a435dc81368e`
- Reviewer: Codex
- Scope: Round 1 findings and regression risk in `cmd/agentdeck/main.go`, `cmd/agentdeck/shell_init_test.go`, `internal/shellconfig/`, and `scripts/test-completion-install.sh`
- Finding resolution:
  - [CLOSED] Post-rename directory open, sync, and close failures now trigger rollback. Existing targets are restored from a same-directory, synced backup; previously missing targets are removed. Existing/missing open and sync failure tests assert failed outcomes, original final state, and no temporary artifacts.
  - [CLOSED] Setup and remove now use distinct default-target selection semantics. No-argument remove neither invokes nor requires invoking-shell detection and reports all zsh, fish, `.bash_profile`, and `.bashrc` defaults while leaving missing files skipped.
  - [CLOSED] Unix-specific tests directly exercise `ownedByCurrentUser` with current, different, missing, and unexpected UID metadata.
- Findings: none at P1 or P2 severity.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run 'Test(PostRenameFailuresRestoreOriginalStateAndRemoveTemporaryArtifacts|NoArgumentRemoveClearsEveryConfiguredDefaultFile|OwnedByCurrentUserChecksUnixUID)$' -v ./internal/shellconfig` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor -run '^TestShellRemoveWithoutArgumentsDoesNotRequireInvokingShellDetection$' -v ./cmd/agentdeck` passed.
  - Reused L3 evidence from the immediately preceding fix workflow on the same unchanged product/test state: targeted Task 2 packages, full vendored suite, targeted race, full vet, and `scripts/test-completion-install.sh` all passed.
- Verdict: PASS
