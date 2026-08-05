---
status: historical
plan: runtime-provider-attribution
task: hook-config-lifecycle
retired: 2026-08-04
---

# Review log — runtime-provider-attribution / hook-config-lifecycle

## Round 1 — 2026-08-03

- Reviewed state: worktree on `53f185b57694f98f30d4f94c1a8f56451099df71`;
  `cmd/agentdeck/main.go` blob `c67983827a75f304e150f2bba572c4f45a054dda`;
  new package `internal/usagehook/` — `config.go` blob
  `b5c0d6bb3749d24646f7f9633abb0d9f09deaa75`, `config_test.go` blob
  `b2c4cf4dbb446c969e3c843b0e9238e9caf80856`, `event.go` blob
  `20bcaf008a0ce3369669eda3e57dd75fb7c45280`, `event_test.go` blob
  `8cd6861752c89537045e85fad0062f0faff79641`, plus `owner_unix.go` and
  `owner_windows.go` (all untracked); and the plan's Status edit.
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: the `usage hook setup|status|remove` lifecycle, owned-entry
  matching, the atomic write and rollback path, the hidden `usage hook event`
  handler, CLI wiring and help, and the task's five acceptance criteria and L3
  verification contract.

### Findings

**P1 — the shipped test suite fails; `Dev` was ticked anyway.**
`GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./cmd/agentdeck`
fails three tests, all of them pre-existing contract fixtures that the new
command group invalidates:

- `TestEveryLeafCommandHasActionableHelp` (`cmd/agentdeck/main_test.go:331,335,338`)
  — `usage hook event` has no `Short`, no `Arguments:` block, and no example.
  `Hidden: true` does not exempt it; `leafCommands` walks it.
- `TestGUIJSONContractFixture` (`cmd/agentdeck/contract_test.go:153`) — the
  pinned leaf-command list gains `usage.hook.event`, `usage.hook.remove`,
  `usage.hook.setup`, `usage.hook.status` and the fixture was not updated.
- `TestIsolatedEndToEndFlow` (`cmd/agentdeck/e2e_test.go:284`) — command
  contracts differ for the same reason.

The plan's "Starting Task" says to tick `Dev` only after the required
verification passes, and this task declares L3 — targeted tests plus the full
vendored suite. Whether the fixtures should be updated or the hidden command
excluded is a design choice, but the task cannot be `Dev`-complete while three
shipped tests are red.

**P1 — `usage hook` enforces mode `0600` on `~/.claude/settings.json`, which
is not AgentDeck's file to re-permission, and the established writer for that
same file deliberately preserves its mode.**
`internal/provider/config.go:733` ends its atomic replace with
`os.Chmod(path, info.Mode().Perm())` — AgentDeck writes the Claude settings
file without changing its permissions. The new package instead treats any mode
other than `0600` as `invalid` (`config.go:178-183`, `:331-333`) and rewrites
the file at `privateFileMode` (`config.go:297`, `:252`).

On this machine `~/.claude/settings.json` is mode `0644`. Measured with a
build of the worktree against a temporary `HOME` holding a `0644` settings
file:

- `usage hook status` → `claude …/settings.json: invalid (hook configuration
  file permissions must be 600)` **and exits 1**. A read-only status command
  reports a healthy default installation as broken.
- `usage hook setup` → succeeds and leaves the file at mode `0600`,
  silently changing the permissions of a file the user and Claude Code own.
- `usage hook remove` on a `0644` file that contains no AgentDeck entry →
  `failed (hook configuration file permissions must be 600)`, exit 1. There is
  nothing to remove and nothing unsafe to read, yet the command errors.

`TestOwnershipAndPermissionsAreHandled` (`config_test.go:288-311`) pins this as
intended ("secured file mode"), so it is a decision rather than an oversight —
but it is an undocumented divergence from the convention the plan explicitly
says to follow, and the plan's Failure and Safety Rules do not authorize
changing an existing file's mode. `~/.codex/hooks.json` is a different case: it
is plausibly AgentDeck-adjacent and `~/.codex/config.toml` is already `0600`.
`~/.claude/settings.json` is not.

**P2 — `remove` is not a round trip: it leaves empty event arrays that were
never there.**
`prepareRemove` (`config.go:347-361`) assigns `hooks[event] = updated` for every
event in `hookEvents(client)`, including events the document never contained,
and `updateEvent` returns an empty non-nil slice. Measured on a settings file
whose `hooks` object originally held only `PreToolUse`:

```
after setup + remove:
  hooks keys: ConfigChange:[]  PreToolUse:[1 entry]  SessionEnd:[]  SessionStart:[]
```

`~/.codex/hooks.json`, which did not exist before setup, is left as
`{"hooks":{"SessionStart":[]}}`. Leaving the *file* is deliberate per the
Decision ("leaves an otherwise empty valid document rather than guessing
whether AgentDeck owns the file"), but leaving three empty keys inside a file
that predates AgentDeck is residue, and it means `setup` followed by `remove`
does not restore the user's document.

**P2 — top-level key order of the user's configuration file is not preserved.**
`encodeDocument` (`config.go:529-540`) marshals `map[string]json.RawMessage`,
and Go sorts map keys. A settings file written as `env, model, hooks, theme`
comes back as `env, hooks, model, theme`; the same applies to the `hooks`
object's own event order and to the keys inside AgentDeck's own entry. Values
and formatting survive — the file stays 2-space indented and every unrelated
key and hook is intact — so this is not a data-loss defect. But
`~/.claude/settings.json` is a hand-edited file with other writers, and a
lifecycle command that reorders the whole document produces a large, confusing
diff for the user and for any VCS they keep it in.

**P2 — the acceptance criterion "verified in both orders" is only half tested.**
The task requires that "`~/.claude/settings.json` survives both writers: a
provider switch preserves AgentDeck-owned hook entries, and hook setup and
remove preserve the provider `env` object, verified in both orders".
`TestSetupAndRemovePreserveClaudeEnvironment` (`config_test.go:104`) covers the
hook-writer direction. Nothing anywhere exercises the provider-switch
direction: `grep -rn "usagehook\|usage hook" --include=*_test.go cmd/
internal/provider/` returns nothing. That direction is the riskier one, because
it runs through a different writer (`internal/provider/config.go`) that this
task did not touch and that now also disagrees with it about file mode.

**P3 — running `setup` twice with different global flags reports the user's own
tool as a third-party modification.**
`runUsageHookLifecycle` (`main.go` in the diff) embeds `--state-dir` into the
installed command, and `classifyEntry` (`config.go:487`) matches candidates by
the `" usage hook event <client>"` suffix while requiring byte equivalence for
`exact`. Measured:

```
usage hook setup --client claude                       -> configured
--state-dir /tmp/st usage hook setup --client claude   -> failed (AgentDeck hook
    claude/SessionStart was modified; refusing to overwrite it)
```

It is recoverable (`remove` with the original invocation works) and the
suffix-based candidate matching is a good design, but the message blames the
user for an edit AgentDeck made, and `setup` is documented as idempotent.

### Nits (non-blocking)

- `readSnapshot(path string, _ bool)` (`config.go:636`) and
  `desiredEntry(client Client, _ string)` (`config.go:591`) each carry an unused
  parameter. The second suggests per-event commands were planned; today all
  events install the same command and the handler distinguishes them from
  `hook_event_name` in the payload, which is fine — but the signature implies
  otherwise.
- `Summary.HasFailures` (`config.go:84`) relies on Go operator precedence
  (`a || b && c`) without parentheses.
- No `Development evidence` section was recorded under the task, unlike every
  other completed task in this repository's plans.

### Acceptance criteria

| Criterion | Verdict |
| --- | --- |
| setup followed by setup is a no-op | Met for an identical invocation (`TestSetupIsIdempotentAndSecuresFiles`); see P3 for differing global flags |
| remove preserves unrelated hooks | Met (`TestRemovePreservesUnrelatedHooksAndTopLevelFields`, confirmed by measurement); see P2 for the residue it adds |
| `shell-init` output byte-identical | No `shell-init` or `shellconfig` code was touched; not separately re-verified this round |
| `settings.json` survives both writers, both orders | **Not met** — one direction untested (P2) |
| no package installation path writes hook configuration | Met; no installer path was touched |

### Evidence

- Full-context source review of the new package, the CLI wiring, and the diff
  against the plan's Decision, Failure and Safety Rules, and acceptance list.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/usagehook ./cmd/agentdeck`
  -> **FAIL** (`internal/usagehook` passes; `cmd/agentdeck` fails three tests).
- `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./internal/usagehook ./cmd/agentdeck`
  -> PASS.
- Behavioral measurement with a build of this worktree against temporary
  `HOME` directories: the `0644` status/setup/remove outcomes, the setup+remove
  residue, the key reordering, and the `--state-dir` refusal quoted above.
- `git diff --check` -> PASS.
- The full vendored suite was not run: `cmd/agentdeck` already fails, so the L3
  aggregate cannot pass and running it would add no information.

- Verdict: REOPEN (two P1, three P2, one P3)

## Round 2 — 2026-08-03 (re-review)

- Reviewed state: worktree on `53f185b57694f98f30d4f94c1a8f56451099df71`;
  `cmd/agentdeck/main.go` blob `f28fe334b757392f1eb6b05b9aead86448c965ff`,
  `internal/usagehook/config.go` blob
  `c4520555c03d8b3c495ec973b194f1db8f177bca`,
  `internal/usagehook/config_test.go` blob
  `c34de092e999ad26507eb7bbdb9cdad26297e143`, and the newly touched
  `cmd/agentdeck/testdata/phase7/gui-json-contract.json` blob
  `beed5f0650363fed0ba50d1ac2a69ada058dc11c` (all uncommitted), plus the plan's
  development-evidence addition.
- Reviewer: Codex (review-only round; no product code or tests changed).
- Method note: Round 1 recorded blob hashes with `git hash-object` without
  `-w`, so those objects were never stored and a direct diff against Round 1 was
  not possible. Finding closure was therefore verified by re-running every
  Round 1 measurement against a build of the current worktree, which is the
  stronger check anyway. This round stores its blobs with `-w`.

### Finding closure

- **P1 (three failing `cmd/agentdeck` contract tests) — CLOSED.**
  `go test -count=1 ./cmd/agentdeck` now passes. `usage hook event` gained a
  `Short`, an `Arguments:` block, and an example while staying `Hidden`, so
  `TestEveryLeafCommandHasActionableHelp` is satisfied honestly rather than by
  exempting the command; the GUI JSON contract fixture gained the four
  `usage.hook.*` leaves (+72 lines).
- **P1 (mode `0600` forced on `~/.claude/settings.json`) — CLOSED.** Measured
  against a `0644` settings file with a build of this worktree:
  `status` -> `absent`, exit 0 (was `invalid`, exit 1); `setup` -> `configured`,
  exit 0, **file still mode 644** (was silently re-permissioned to 600);
  `remove` -> `removed`, exit 0, still mode 644 (was `failed`, exit 1). The
  behavior now matches `internal/provider/config.go:733`, which preserves the
  file's own mode.
- **P2 (remove leaves empty event arrays) — CLOSED.** After `setup` then
  `remove` on a document whose `hooks` object held only `PreToolUse`, the
  `hooks` object holds only `PreToolUse` again; no `ConfigChange`,
  `SessionEnd`, or `SessionStart` residue remains.
- **P2 (top-level key order not preserved) — CLOSED.** `sort` is now used in
  the encoder path and the measured round trip preserves the original
  `env, model, hooks, theme` order. The round trip is semantically equal and
  byte-identical except that the `hooks` value is re-emitted compact; every
  unrelated key keeps its original bytes.
- **P2 (both-writers acceptance only half tested) — CLOSED in code, not yet
  executed.** `TestClaudeHookAndProviderWritersPreserveEachOther`
  (`config_test.go:147`) exercises both orders against the real
  `provider.WriteClaudeConfig`, not a stand-in. It cannot currently run; see the
  new P1 below.
- **P3 (`--state-dir` reported as third-party modification) — CLOSED.**
  Measured: `setup --client claude` then
  `--state-dir /tmp/st usage hook setup --client claude` -> `unchanged`,
  exit 0 (was `failed`, exit 1).
- Nits: the unused `desiredEntry` event parameter and the `HasFailures`
  precedence were both addressed; a `Development evidence` section was added to
  the plan.

### New findings

**P1 — the `internal/usagehook` test package does not compile, and the plan
records verification evidence that contradicts this.**

```
vet: internal/usagehook/config_test.go:647:76: too many arguments in call to
     manager.desiredEntry
     have (Client, string)
     want (Client)
```

Removing the unused `event` parameter from `desiredEntry` (`config.go:832`)
updated all six call sites in `config.go` but left the one in `config_test.go`.
Consequences:

- `go test -mod=vendor -count=1 ./internal/usagehook` -> `[build failed]`. The
  entire new package currently has **zero executed test coverage**, including
  every test written to close the Round 1 findings.
- `go test -mod=vendor -count=1 ./...` -> FAIL.
- `go vet -mod=vendor ./...` -> FAIL.

The plan's new evidence block claims all three of these PASS, plus `-race`.
Those four claims are false against the reviewed tree. This is the same failure
mode as Round 1's first P1 — `Dev` ticked while the suite is red — except that
it now carries a written assertion to the contrary, which is worse than the
unticked case because it would let a later reader skip re-verification.

The product code itself compiles: `go build ./cmd/agentdeck` succeeds, which is
why all six behavioral measurements above were possible. The fix is one line in
a test file, but the evidence block must be re-derived from an actual run, not
edited to match.

### Evidence

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/usagehook ./cmd/agentdeck`
  -> `cmd/agentdeck` PASS (`34.284s`); `internal/usagehook` **build failed**.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
  -> FAIL (same build failure).
- `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` -> FAIL
  (same).
- `GOCACHE=/private/tmp/agent-deck-go-build go build -mod=vendor ./cmd/agentdeck`
  -> PASS.
- `make check-privacy` -> PASS (exit 0).
- `git diff --check` -> PASS.
- Behavioral measurement with that build against temporary `HOME` directories,
  repeating every Round 1 measurement: `0644` status/setup/remove outcomes and
  resulting file modes, setup+remove round-trip fidelity (key order, residue,
  byte comparison), and the `--state-dir` re-setup outcome.
- `-race` was not run: the package does not build, so the result would carry no
  information beyond the build failure already recorded.

- Verdict: REOPEN (one P1; all six Round 1 findings closed)

## Round 3 — 2026-08-03 (re-review)

- Reviewed state: worktree on `53f185b57694f98f30d4f94c1a8f56451099df71`;
  `internal/usagehook/config_test.go` blob
  `47d677d81a3b2f465b38430676e9760d732d96cb` (changed since Round 2).
  `cmd/agentdeck/main.go` blob `f28fe334b757392f1eb6b05b9aead86448c965ff`,
  `internal/usagehook/config.go` blob
  `c4520555c03d8b3c495ec973b194f1db8f177bca`, and the GUI contract fixture blob
  `beed5f0650363fed0ba50d1ac2a69ada058dc11c` are byte-identical to Round 2.
  All blobs stored with `git hash-object -w`.
- Reviewer: Codex (review-only round; no product code or tests changed).
- Scope: closure of the Round 2 P1 (build failure and contradicted evidence),
  independent re-derivation of every claim in the plan's evidence block, and
  confirmation that the now-executable tests actually pin the Round 1 repairs.

### Finding closure

- **P1 (test package does not compile; plan evidence contradicts the tree) —
  CLOSED.** `config_test.go:647` now calls `manager.desiredEntry(client)`, and
  `grep -rn "desiredEntry(client, " internal/usagehook/` returns nothing, so no
  stale call site remains. The evidence block was rewritten, and all four of its
  claims were re-derived independently this round rather than read:

  | Claim in the plan | Independently measured |
  | --- | --- |
  | `go test -count=1 ./internal/usagehook` -> PASS | PASS (`1.095s`) |
  | `go test -count=1 ./...` -> PASS | PASS (no FAIL lines) |
  | `go test -count=1 -race ./internal/usagehook ./cmd/agentdeck` -> PASS | PASS (`2.115s`, `110.089s`) |
  | `go vet ./...` -> PASS | PASS (exit 0) |

  The block no longer claims a `make check-privacy` or `git diff --check`
  result; both were nonetheless run this round and pass.

### Verification that the executed tests pin the Round 1 repairs

Round 2 could confirm the Round 1 repairs only by external measurement, because
no test in the package could run. All 16 tests now execute and pass, and they
pin the repaired behavior rather than the original behavior:

- `TestOwnershipAndPermissionsAreHandled` (`config_test.go:402-428`) now writes
  a `0644` file and asserts `status` -> `absent`, `setup` -> `configured` with
  the file **still `0644`** ("existing file mode = %o, want 644"), and
  `remove` -> `removed`. In Round 1 this same test asserted the file had been
  re-permissioned to `0600`. The pinned contract was inverted deliberately.
- `TestClaudeLifecyclePreservesOrderingAndRemovesOnlyManagedEvents` pins key
  ordering and managed-event-only removal — Round 1's two ordering/residue P2s.
- `TestClaudeHookAndProviderWritersPreserveEachOther` pins both writer orders
  against the real `provider.WriteClaudeConfig` — Round 1's half-tested
  acceptance criterion.
- `TestStateDirVariantsRemainManaged` pins the `--state-dir` variant as managed
  — Round 1's P3.

### Round 1 behavioral measurements

Not repeated. `cmd/agentdeck/main.go` and `internal/usagehook/config.go` are
byte-identical to the blobs measured in Round 2, so the six runtime
measurements recorded there remain bound to this exact product content state.
Only a test file changed since.

### Acceptance criteria

| Criterion | Verdict |
| --- | --- |
| setup followed by setup is a no-op | Met (`TestSetupIsIdempotentAndSecuresFiles`, `TestStateDirVariantsRemainManaged`) |
| remove preserves unrelated hooks | Met (`TestRemovePreservesUnrelatedHooksAndTopLevelFields`, `TestClaudeLifecyclePreservesOrderingAndRemovesOnlyManagedEvents`) |
| `shell-init` output byte-identical | Met; no `shell-init` or `shellconfig` code was touched, and the full suite including the shellconfig byte-identity tests passes |
| `settings.json` survives both writers, both orders | Met (`TestClaudeHookAndProviderWritersPreserveEachOther`) |
| no package installation path writes hook configuration | Met; no installer path was touched |

### Remaining observations (non-blocking, no action required)

- The `hooks` value is re-emitted compact while every other key keeps its
  original bytes, so a hand-formatted `hooks` block loses its inner whitespace.
  Recorded in Round 2; harmless and not worth churn.
- `readSnapshot(path string, _ bool)` still carries an unused parameter. The
  sibling `desiredEntry` case was cleaned up and broke the build; leaving this
  one alone until it has a reason to change is the safer call.

### Evidence

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/usagehook`
  -> PASS (`1.095s`); `-v` run confirms 16 tests executed, 0 failures.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...`
  -> PASS.
- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -race ./internal/usagehook ./cmd/agentdeck`
  -> PASS (`2.115s`, `110.089s`).
- `GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...` -> PASS.
- `make check-privacy` -> PASS.
- `git diff --check` -> PASS.

- Verdict: PASS
