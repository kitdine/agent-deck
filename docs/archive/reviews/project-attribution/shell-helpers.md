---
status: historical
plan: project-attribution
task: shell-helpers
---

# Review log — project-attribution / shell-helpers

## Round 1 — 2026-07-29

Independent review of the uncommitted worktree implementing
`agentdeck shell-init <bash|fish|zsh>`.

### Findings

- [P1] The emitted helpers attribute every launch regardless of whether any
  wrapper is declared `headroom`, contradicting the plan's "Attribution is
  opt-in and wrapper-scoped" decision. `provider.ProjectEnvironment` is
  deliberately route-independent and the hidden resolver never consults provider
  state, so with an empty state directory and no provider configured at all,
  `shell-init bash --project-environment claude` still printed
  `X-Headroom-Project: my-proj`. Because `ANTHROPIC_CUSTOM_HEADERS` needs no
  managed mapping, a sourced helper sends the directory name to whatever
  upstream Claude is currently pointed at, including `api.anthropic.com` and
  non-Headroom relays. `agentdeck run` in the same state injects nothing, so the
  acceptance criterion "produces the same header value that `agentdeck run`
  would have produced from the same directory" holds only for the single
  configured state that `scripts/test-completion-install.sh` exercises. Codex is
  materially safer here only by accident — it emits no header without the
  managed `env_http_headers` mapping.

- [P2] `leafCommands` in `cmd/agentdeck/contract_test.go:68` now hardcodes a
  second exclusion for `shell-init`, removing it from
  `TestEveryLeafSyntaxErrorUsesStableJSON`,
  `TestEveryLeafCommandHasActionableHelp`,
  `TestHelpOmitsLegacyProviderCredentialAndSessionExamples`, and the GUI
  contract fixture's `LeafCommands` list. The exclusion is not technically
  required: the built binary already answers `--format json shell-init` with
  exit 2 and a conforming envelope
  (`"command":"shell-init"`, `"code":"invalid_argument"`), so the effect is to
  avoid regenerating the fixture rather than to describe a real contract gap.

- [P2] The help catalog entry for `shell-init` deviates from every other command
  in user-visible output. `main.go:565-569` indents the argument line and the
  example with one space, so `agentdeck shell-init --help` prints
  ` bash|fish|zsh Supported shell.` where `completion` and the rest print
  `  bash|fish|zsh  Supported shell.`. The entry is also assigned after the
  `entries` map literal instead of inside it, breaking the catalog's single
  shape.

- [P2] The recorded evidence does not cover the change's blast radius. The Dev
  note lists only `-run ShellInit`, `-run GUIJSONContractFixture`, and
  `./internal/provider -run Project`, while the edited `leafCommands` helper is
  shared by five call sites across three test files. Independent re-running of
  the full affected packages passed (see below), so this is an evidence gap
  rather than an observed regression.

- [P3] The "Attribution never fails a launch" invariant has no test. The
  fail-open branch — `agentdeck` absent from `PATH`, exiting non-zero, or
  printing nothing — is only exercised implicitly.
  `scripts/test-completion-install.sh` always has a working binary on `PATH`.

- [P3] `agentdeck shell-init <shell> --project-environment <client>` validates
  and then ignores the shell argument; the resolver output is identical for
  `bash`, `fish`, and `zsh`. The hidden interface's shape is harder to read than
  it needs to be.

- [P3] The fish branch uses `"$(...)"`, which requires fish 3.4 or newer, and
  the minimum version is recorded nowhere. Probed on fish 4.8.1: the
  `set -l ... "$(cmd)"` / `set -l ... $status` pair is correct — `$status` does
  carry the substitution's exit status (`set -l v "$(sh -c "exit 3")"` left
  `status=3`).

- [P3] The worktree carries an unrelated 24-line `AGENTS.md` change (the "Review
  Artifact Finalization" section and the review read-only wording). It is not
  part of this task and should not ride along in the shell-helpers commit.

### Strengths

- The callback resolver keeps the "One definition of project" invariant intact:
  percent-encoding and the identity rule stay in Go and are not restated in
  shell.
- `scripts/test-completion-install.sh` compares real launches rather than
  strings — a `my+project` fixture that actually exercises the `+` → `%2B`
  divergence, a fake `codex`/`claude` on `PATH`, and a pre-existing
  `Other-Header: keep` proving Claude header preservation — against
  `agentdeck run` output from the same directory, in all three shells, each
  after its own `-n` syntax check.
- `TestShellInitEmitsDynamicClientWrappersForSupportedShells` asserts the state
  directory is never created, pinning "emission opens no AgentDeck state", and
  asserts the script embeds no project, endpoint, or credential.

### Independent verification

- `env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor
  ./cmd/agentdeck ./internal/provider ./internal/session` — all passed
  (24.3s / 5.3s / 3.0s).
- Built the worktree binary and probed the resolver directly in a temporary
  directory with an empty `--state-dir`: Codex printed `my-proj`, Claude printed
  `X-Headroom-Project: my-proj`, and the state directory stayed empty.
- Probed `--format json shell-init` and `--format json shell-init powershell`:
  both exit 2 with a conforming error envelope.
- Compared `shell-init --help` and `completion --help` byte-for-byte on the
  argument and example lines.
- `fish --version` = 4.8.1; probed command-substitution status semantics.

**Verdict: REOPEN.** One P1 and four P2 findings remain open.

## Round 2 — 2026-07-29 fix

All Round 1 findings were addressed in the working tree for independent
re-review:

- **[P1] Route-aware resolver:** the hidden resolver now opens existing state
  read-only and calls `RunProjectEnvironment`, so it emits only for the current,
  non-stale selection whose wrapper is still declared `headroom`. Missing or
  unreadable state and every ineligible route return empty without blocking the
  client. A behavior test first failed with `my%2Bproject` from an empty state,
  then passed after the fix. The install script now compares helper and
  `agentdeck run` output for both clients in an unconfigured state as well as
  the configured Headroom state.
- **[P2] Shell-program contract classification:** `completion` and
  `shell-init` remain outside the GUI data fixture through a named, documented
  `emitsShellScript` predicate rather than an inline hardcoded exclusion. A
  dedicated test asserts `shell-init` syntax errors keep exit 2, empty stdout,
  `command: shell-init`, and `code: invalid_argument`.
- **[P2] Help shape:** `shell-init` is again inside the help catalog map
  literal; its argument and example strings use the standard two-space
  indentation.
- **[P3] Fail-open launch:** bash, fish, and zsh install-path tests each source
  the generated helper and prove the fake client starts when `agentdeck` is
  absent, returns status 23, or returns status 0 with empty output.
- **[P3] Hidden interface:** generated functions now call
  `agentdeck shell-init --project-environment <client>` without a meaningless
  shell positional argument. A source comment explains that only public script
  generation requires the shell argument.
- **[P3] Fish floor:** generated fish output and the CLI manual state fish 3.4
  or newer is required.
- **[P3] Change separation:** the existing `AGENTS.md` workflow-policy diff was
  not changed and remains outside the shell-helpers logical change; no commit
  was made.

Verification:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck ./internal/provider ./internal/session` — passed.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build bash
  scripts/test-completion-install.sh` — passed.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/contract_test.go cmd/agentdeck/shell_init_test.go
  internal/provider/project.go internal/provider/service.go` — no output.
- `rtk git diff --check` — passed.

Status: awaiting independent re-review; the plan's Review cell remains
unchecked.

## Round 3 — 2026-07-29

### Re-review

Independent verification of every Round 1 finding against the current worktree.

**[P1] Route-aware resolver — closed.** `writeProjectEnvironment`
(`cmd/agentdeck/main.go:641`) now resolves through `store.OpenReadOnly` and
`Service.RunProjectEnvironment`, the same predicate `agentdeck run` uses, so the
two cannot diverge by construction rather than by parallel logic.
`internal/provider/project.go` and `internal/provider/service.go` are back at
their `HEAD` content — the route-independent `ProjectEnvironment` export was
removed rather than left as a second path. `stateRoot()` is pure path
resolution, and `OpenReadOnly` uses `mode=ro`, so no state is created. Probed on
a built binary with a real provider, wrapper, and selections:

| State | resolver | `agentdeck run` |
| --- | --- | --- |
| headroom wrapper, direct switch (no `--via`) | empty | empty |
| headroom wrapper, `--via` switch | `X-Headroom-Project: my-proj` | same |
| selection still `via`, wrapper kind reset to `plain` | empty | — |

The third row confirms a stale selection stops attributing immediately.

**[P2] Shell-program contract classification — closed.** The inline exclusion is
replaced by the named, commented `emitsShellScript` predicate
(`cmd/agentdeck/contract_test.go:81`), and
`TestShellInitSyntaxErrorUsesStableJSONEnvelope` asserts exit 2, empty stdout,
`command: shell-init`, and `code: invalid_argument` directly.

**[P2] Help shape — partially fixed, downgraded to P3.** The entry is back
inside the `entries` map literal and the leading indentation is now two spaces.
The column gap inside the line was not: `shell-init` prints
`  bash|fish|zsh Supported shell.` against `completion`'s
`  bash|fish|zsh  Supported shell.`. First-column alignment — the visible
defect — is fixed; one space of description offset remains and does not block.

**[P2] Evidence coverage — closed.** Independently re-run below.

**[P3] Fail-open — closed.** `scripts/test-completion-install.sh` sources the
generated helper in bash, fish, and zsh and proves the fake client still starts
when `agentdeck` is absent from `PATH`, exits 23, or exits 0 with empty output.

**[P3] Hidden interface — closed.** `Args` takes zero positionals in the
resolver path, and a comment explains why only script generation needs a shell.

**[P3] Fish floor — closed.** `# Requires fish 3.4 or newer.` is emitted in the
generated fish script and stated in `docs/specs/cli-manual.md`.

**[P3] Change separation — carried to delivery.** `AGENTS.md` still shows its
unrelated 24-line workflow-policy diff in the worktree. Nothing was committed,
so this cannot be verified at review time; it remains a commit-staging
obligation.

### New findings

- [P2] `docs/specs/cli-manual.md:145` still tells the reader the shell helper
  "也不要求预先配置 wrapper". That was true of the reviewed-and-rejected
  behavior and is now wrong: the resolver emits nothing unless the current
  selection routes through a wrapper declared `headroom`. A user who follows the
  manual will source the helper, see no header, and have no documented reason
  why. The neighboring Codex `env_http_headers` sentence is still correct but
  now reads as the only precondition.

- [P3] The install script's "unconfigured state" comparison is weaker than it
  looks. With no provider at all, `agentdeck run codex --` fails with
  `no provider selection for client`, so the assertion compares an empty output
  caused by a failed launch against the helper's empty output. The regression
  worth automating is a *configured* provider on a direct switch; that state was
  verified by hand this round (both empty) but nothing pins it.

### Independent verification

- `env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor
  ./cmd/agentdeck ./internal/provider ./internal/session ./internal/store` —
  all passed (28.5s / 7.3s / 4.0s / 5.2s).
- `env GOCACHE=/private/tmp/agent-deck-go-build bash
  scripts/test-completion-install.sh` — exit 0.
- `gofmt -l` on the five listed Go files — no output.
- `git diff --check` — exit 0.
- Built the worktree binary and probed the three selection states in the table
  above, plus `shell-init --help` against `completion --help` byte for byte.

**Verdict: REOPEN.** The P1 and three of the four P2 findings are closed. One
new P2 documentation-behavior contradiction remains, introduced by the P1 fix.

## Round 4 — 2026-07-29 fix

Both Round 3 findings were addressed for independent re-review:

- **[P2] Manual precondition:** the shell-helper section now says generation
  requires no wrapper, while attribution activation requires the current
  selection to use a wrapper declared `headroom`; an ineligible route silently
  skips injection without affecting launch. The neighboring Codex paragraph
  retains only the additional managed `env_http_headers` mapping requirement.
- **[P3] Configured direct-switch regression:** the completion-install test
  configures a provider and Headroom wrapper, switches Codex and Claude
  directly without `--via`, proves both real `agentdeck run` launches succeed
  with empty attribution, then sources each generated bash, fish, and zsh
  helper and proves the same empty result. This supplements rather than removes
  the no-selection comparison.
- **[P3, optional] Help alignment:** the shell argument description now uses
  the same two-space column gap as `completion`.

The direct-switch behavior was already correct, so its newly added regression
assertion passed before any production behavior change; it closes an untested
state rather than demonstrating a defect RED.

Verification:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` — passed in 24.703s.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build bash
  scripts/test-completion-install.sh` — passed.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/contract_test.go cmd/agentdeck/shell_init_test.go` — no output.
- `rtk git diff --check` — passed.

Status: awaiting independent re-review; the plan's Review cell remains
unchecked. No commit was made.

## Round 5 — 2026-07-29

### Re-review

Both Round 3 findings are closed in content, and the optional item was taken:

- **[P2] Manual accuracy — closed.** `docs/specs/cli-manual.md:145-147` now
  separates the two claims: generating the script needs no wrapper, while
  attribution taking effect requires the current selection to route through a
  wrapper declared `headroom` (`provider use --via`), and says explicitly that
  an unmet precondition silently injects nothing without affecting the launch.
  Lines 155-157 demote the Codex `env_http_headers` mapping to an additional
  Codex-only requirement instead of the only one.
- **[P3] Direct-switch regression — present.** `test_shell_helpers` now
  configures the provider and Headroom wrapper, switches both clients *without*
  `--via`, asserts `direct_codex`/`direct_claude` are empty, and compares each
  sourced helper against them in all three shells, before the `--via` switch
  and the existing configured-route assertions.
- **[P3] Help alignment — closed.** `shell-init --help` and `completion --help`
  now print the identical `  bash|fish|zsh  Supported shell.` column.

### New finding

- [P1] **`scripts/test-completion-install.sh` has never executed a single one of
  its assertions, and it now also blocks the completion-install checks that
  worked before this task.** Run without a pipe, the script exits 1.
  `bash -x` puts the first stop at the very first new statement: under
  `set -euo pipefail` (line 2), `unconfigured_codex=$(… run codex --)` inherits
  the non-zero status of `agentdeck run` failing with
  `no provider selection for client`, so the script aborts before
  `test -z` ever runs. Neutralizing only that, the next stop is
  `provider use helper --client codex --config-path "$codex_config"`, which
  exits 1 with `open …/config.toml: no such file or directory` — the function
  declares `codex_config` and `claude_config` but never creates the files.
  Reproduced standalone: `provider use --config-path` against a missing file
  exits 1 for both clients. The Go test's `configureHeadroomSelections`
  (`cmd/agentdeck/shell_init_test.go:176-187`) does write both files first,
  which is why the Go coverage is real and the shell coverage is not.

  Because `test_shell_helpers` is invoked before `install_for`, the abort also
  skips the bash/zsh/fish completion install, uninstall, and rollback checks
  this script existed for. That is a regression beyond this task's scope.

  Consequence for the record: the "install script passed" line in the Dev note
  and in Round 2, Round 4, and my own Round 3 verification is not supported.
  Round 3's `bash scripts/test-completion-install.sh — exit 0` was my
  measurement error: the run was piped into `tail`, so the reported status was
  `tail`'s. Corrected here. Every behavior those notes describe as proven by
  this script — the `-n` syntax checks, the `my%2Bproject` equality, the Claude
  header preservation, the fail-open cases, the direct-switch comparison — is
  currently unproven by it. The equivalent Go-level assertions in
  `cmd/agentdeck/shell_init_test.go` did run and did pass.

  Candidate fix verified out-of-tree: on a scratch copy with the two config
  files created after the fake-client `chmod` and the two unconfigured `run`
  substitutions allowed to fail, the whole script — shell helpers *and* the
  original completion install suite — exits 0. So both defects are in the test
  harness; no production behavior needs to change.

### Independent verification

- `env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor
  ./cmd/agentdeck ./internal/provider ./internal/session ./internal/store` —
  all passed (25.5s / 7.9s / 3.1s / 2.9s).
- `env GOCACHE=/private/tmp/agent-deck-go-build bash
  scripts/test-completion-install.sh` — **exit 1**, measured without a pipe.
- `bash -x` traces isolating both abort points, plus standalone reproduction of
  `provider use --config-path <missing file>` exiting 1.
- Patched scratch copy of the script — exit 0.
- `gofmt -l` on the five listed Go files — no output.
- `git diff --check` — exit 0.
- Read `docs/specs/cli-manual.md:136-157` and compared `shell-init --help` with
  `completion --help` byte for byte.

**Verdict: REOPEN.** Both Round 3 findings are closed, but the task's shell-level
acceptance evidence does not exist and the pre-existing completion-install
coverage is currently blocked.

## Round 6 — 2026-07-29 fix

The Round 5 test-harness finding was fixed without product-code changes:

- `test_shell_helpers` now creates the Codex and Claude config fixtures before
  the first `provider use`, with the same contents as the Go setup:
  `model = "synthetic"` and `{}`.
- The no-selection `agentdeck run` substitutions tolerate their expected
  non-zero status and retain the empty-output assertions. They remain useful
  only for the missing-selection/fail-open edge; the configured direct-switch
  case supplies successful-launch equivalence.
- Every unsupported historical install-script pass statement in the plan was
  corrected rather than retained as evidence.

Verification:

- `rtk proxy bash scripts/test-completion-install.sh` — run without a pipeline;
  the process exited 0. The sequential `set -euo pipefail` script therefore
  completed the shell-helper assertions and all subsequent completion
  install/uninstall, tamper, rollback, and interruption checks.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` — passed in 25.035s.
- `rtk git diff --check` — passed.

Status: awaiting independent re-review; the plan's Review cell remains
unchecked. No commit was made.

## Round 7 — 2026-07-29

### Re-review

The Round 5 P1 is closed, and closed in the way that matters: the script now
runs, and it runs its assertions rather than merely exiting 0.

**Harness repair — verified.** `test_shell_helpers` writes
`model = "synthetic"` to `$codex_config` and `{}` to `$claude_config` before the
first `provider use`, matching `configureHeadroomSelections`. The two
no-selection `agentdeck run` substitutions are wrapped in `) || true`, and their
`test -z` assertions are kept rather than dropped. The `|| true` appears exactly
twice in the diff and touches no `test` statement.

**Assertions genuinely execute.** A bare, unpiped run exits 0. `bash -x` shows
77 `test` invocations comparing real values, not tautologies against unset
variables:

| Assertion | Count | What it pins |
| --- | --- | --- |
| `test my%2Bproject = my%2Bproject` | 4 | Codex wire value, `agentdeck run` plus three shells |
| `test $'Other-Header: keep\nX-Headroom-Project: my%2Bproject' = …` | 4 | Claude header preservation with a real embedded newline |
| `test client-started = client-started` | 9 | fail-open, three shells × absent/exit-23/empty |
| `test '' = ''` | 12 | direct-switch and no-selection helper/`run` equality |
| `test -z ''` | 4 | both no-selection baselines |

**The blocked pre-existing coverage is restored.** The same trace reaches
`install_for` and its successors: `test 640 = 640`, `test -L …/config.fish`,
and the `rollback`, `interrupt`, `dangling`, `duplicate`, and `unowned`
`test '!' -e …` checks all execute. The regression Round 5 flagged beyond this
task's scope is gone.

**Record corrected.** The plan note now states that the original install-script
result was invalidated by Round 5 and records the current bare exit-0 run in its
place, and says explicitly that all earlier pass claims were corrected. No
unverified description remains.

**Carried to delivery, not a review finding.** `AGENTS.md` still carries its
unrelated 24-line workflow-policy diff. Nothing has been committed, so this
remains a commit-staging obligation: it must not ride along in the
shell-helpers commit.

### Independent verification

- `bash scripts/test-completion-install.sh` — **exit 0**, measured with no pipe.
- `bash -x` on the same script — 77 executed `test` assertions, tabulated above.
- `env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1 -mod=vendor
  ./cmd/agentdeck ./internal/provider ./internal/session ./internal/store` —
  all passed (23.8s / 5.1s / 2.8s / 5.5s).
- `gofmt -l` on the five listed Go files — no output.
- `git diff --check` — exit 0.
- `git diff --stat` confirms Go sources are unchanged since Round 5
  (`main.go` +127, `contract_test.go` +8); only the script (+3 lines) and the
  documentation notes moved.

**Verdict: PASS.** Every finding from Rounds 1, 3, and 5 is closed. The task's
acceptance criterion — a sourced helper producing the same header value as
`agentdeck run` from one fixture directory, in each named shell after that
shell's own syntax check — is now backed by assertions that actually run.
