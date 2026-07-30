---
status: active
plan: shell-integration
task: route-change-advisories
---

# Review log — shell-integration / route-change-advisories

## Round 1 — 2026-07-30

- Reviewed state: `5b622b2` plus reviewed file-set SHA-256 `4bbeef8ab82fd0bbf22157dfc9a09b2eeb18d23410e4c332350f3653a6d63605`
- Reviewer: Claude Opus 5
- Scope: `cmd/agentdeck/main.go`, `internal/provider/service.go`, `internal/provider/project_gate.go`, `internal/doctor/doctor.go`, their tests, `cmd/agentdeck/project_attribution_guidance_test.go`, `cmd/agentdeck/shell_init_test.go`, `internal/backup/backup_test.go`, and the four onboarding texts plus both assertion sets; Task 6 advisory matrix, advisory-accuracy correction, negative-gate marker ownership, and the task 5 cost-disclosure revision this task was required to make
- Findings:
  - [P1] `shellQuote` (`cmd/agentdeck/main.go:1925`) emits an invalid POSIX single-quoted string, and this task made it load-bearing by using it to embed the gate path into every generated wrapper (`:1134`). It replaces `'` with `'\"'\"'` where the correct escape is `'"'"'` (or `'\''`), so a state root whose path contains a single quote produces a script that silently loses half its content instead of failing. Reproduced with the built binary: `agentdeck --state-dir "$TMP/o'brien" shell-init zsh` parses without error under `bash -n`, `zsh -n`, and `fish -n`, but after sourcing it, `declare -f codex` shows the entire `claude()` definition absorbed into `codex`'s body as quoted text and `declare -f claude` prints nothing — the Claude wrapper is never defined, and Codex's gate test compares against a corrupted path, so neither client can ever be attributed. No error reaches the user, and `shell status` still reports the gate consistent, so nothing surfaces it. Before this task the same function only formatted a recovery command for display, which is why it survived. Fix `shellQuote` and add a regression that generates a script for a state root containing a single quote and asserts, for bash and fish, that both wrapper functions are defined and that the embedded path round-trips to the exact gate path.
  - [P2] Nothing exercises the quoting or path-embedding of the gate at all, for any path. `TestShellInitNegativeGateControlsForkButNotAttributionDecision` writes the gate under a plain `t.TempDir()` and `scripts/test-completion-install.sh` uses plain fixture homes, so every existing assertion passes with any quoting implementation that happens to be correct for shell-safe paths. This is the coverage gap that let the P1 through, and it is separate from fixing the escape: the regression must assert the generated script's behavior for a path needing escaping, not merely that the current path works.
  - [P2] `ProjectAttributionAdvisory` (`internal/provider/service.go:149`) was corrected but is now route- and configuration-blind at its remaining call site. `provider use` moved to `reportRouteChangeAttributionGuidance`, leaving `set-wrapper --kind headroom` as the only emitter of the constant, where it states that "managed shell integration attributes eligible Codex and Claude launches through Headroom wrappers" regardless of whether any client is eligible, whether a marker exists, or whether any shell is configured. The task removed the false sentence it was required to remove, so this is not the original defect, but a user who runs `set-wrapper --kind headroom` with no route selected and no shell configured is told that attribution works. Either make this call site reuse the eligibility-and-configuration reads this task already added, or narrow the wording to describe the mechanism's precondition rather than its effect.
- Test review:
  - The four advisory cells are each covered at the command layer with distinct assertions, including the negative case where leaving an eligible route with no shell configured must print nothing, and `assertRouteAdvisorySafe` checks no project value, endpoint, or credential reaches stderr. `TestProjectAttributionAdvisoryUnreadableStartupDegradesToSetup` covers the required degradation. `--quiet` plus `--format json` are asserted to leave stderr empty and keep the guide URL out of the stdout envelope.
  - The marker is well covered: `TestProviderUseMaintainsProjectAttributionGateAcrossClients` walks creation on the first eligible client, persistence while one client remains eligible, and removal when the last one leaves; `TestProjectAttributionGateReportsMissingStaleAndInvalidStates` covers missing, stale, and non-regular-file states and proves a failing gate write *and* a failing gate removal both leave the switch successful. `assertProjectAttributionGateFile` pins empty contents and `0600`.
  - Backup exclusion is correct by construction rather than by a new code path: `backup.Service.Create` builds an explicit allowlist (`internal/backup/backup.go:109-146`), so the added assertion is a regression guard, which is the right shape.
  - Cross-shell gate behavior is genuinely covered: `scripts/test-completion-install.sh` now generates each shell's script with the fixture `HOME`, so the pre-`--via` loop exercises all three shells with the gate absent (no injection, no fork) and the post-`--via` loop exercises all three with it present (injection). Fish's structurally different gate block is therefore protected.
  - The gap is entirely the quoting one described above.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor ./...` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build bash scripts/test-completion-install.sh` passed, covering the regenerated wrappers in bash, zsh, and fish with the gate both absent and present.
  - P1 reproduction: built `./cmd/agentdeck`, ran `shell-init zsh` and `shell-init fish` against a state dir named `o'brien`; all shells accepted the scripts, and `bash -c 'source …; declare -f codex; declare -f claude'` showed `claude` undefined and its text inside `codex`. A corrected `'"'"'` escape was verified to parse and separate correctly under both `bash -n` and `fish -n`.
  - The task 5 cost-disclosure revision required by `docs/plans/shell-integration.md:1062-1067` is present in all four texts — formula caveat, `scripts/manage-install.sh:509-518`, `README.md:160-163`, `docs/specs/cli-manual.md:145-151` — with matching updates in `scripts/test-completion-install.sh` and `scripts/test-release-distribution.sh`.
- Note, already routed elsewhere: the wrappers still lack the `agentdeck`-on-`PATH` guard from the resolution order, folded into task 5 on 2026-07-30. The gate does not change that: a leftover gate file keeps the fish wrapper forking and printing `Unknown command` after `brew uninstall`.
- Verdict: REOPEN

## Round 2 — 2026-07-30

- Reviewed state: `5b622b21edc322a0ca815fbbc303c1dfa554fc79` plus reviewed
  four-file-set SHA-256
  `478691f7e2163d422a8316ee60a9bc17da85f0e278c0d660af234af5f6cf5f40`
- Reviewer: Codex
- Scope: Round 1's three findings in `cmd/agentdeck/main.go`,
  `cmd/agentdeck/shell_init_test.go`,
  `cmd/agentdeck/project_attribution_guidance_test.go`, and
  `internal/provider/service.go`
- Finding resolution:
  - [CLOSED] `shellQuote` now uses the POSIX-safe `'"'"'` sequence. This also
    corrects the shared quoting used by session next-command display.
  - [CLOSED] `TestShellInitQuotesGatePathAndDefinesBothWrappers` generates bash
    and fish scripts from a state root containing a single quote, checks each
    with `-n`, sources each script and requires both `codex` and `claude`
    functions, and evaluates both embedded gate expressions through the target
    shell to compare the resulting bytes with
    `provider.ProjectAttributionGatePath(stateDir)`.
  - [CLOSED] the remaining `set-wrapper --kind headroom` advisory now describes
    eligibility route, marker, and configured-shell prerequisites instead of
    asserting attribution is already active; the exact advisory contract and
    JSON/quiet behavior remain covered.
- Findings: none at P1 or P2 severity.
- Test review:
  - The quote-path regression protects observable generated-script behavior,
    not only source text: a malformed quote that swallows the second function
    fails the function-existence checks, while a syntactically valid but wrong
    path fails the target-shell byte comparison.
  - Advisory tests pin the prerequisite wording, its single AgentDeck guide
    URL, stderr-only placement, and `--quiet` suppression without coupling the
    test to provider-use state branching.
  - The known missing `agentdeck` PATH guard remains assigned to Task 5 and is
    outside this re-review.
- Evidence:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor
    ./cmd/agentdeck -run TestShellInitQuotesGatePathAndDefinesBothWrappers`
    passed.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor
    ./cmd/agentdeck -run TestProjectAttributionGuidance` passed.
  - Reused from the immediately preceding fix workflow on the same unchanged
    product/test state: `go test -count=1 -mod=vendor ./...`,
    `go vet -mod=vendor ./...`, and
    `bash scripts/test-completion-install.sh` all passed.
- Verdict: PASS
