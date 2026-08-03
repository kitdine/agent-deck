---
status: historical
created: 2026-07-29
retired: 2026-07-31
---

# Shell Integration

Target release: `v0.2.1-rc.2`, then stable `v0.2.1`.

Stable `v0.2.1` promotion is paused. `v0.2.1-rc.1` proves that
`shell-init` emits valid wrappers, but it does not provide a complete path
from installing AgentDeck to having those wrappers active. This plan closes
that product gap before another release candidate.

## Goal

Make project-attribution shell integration an explicit, coherent lifecycle
whose every step can be verified:

```text
brew install agentdeck
agentdeck provider set-wrapper <name> --url <url> --kind headroom
agentdeck provider use <name> --via
  -> configures every shell in use, names each file, and prints the one-line
     command for this session
open a new shell, or run that command
agentdeck shell status                     # optional self-check
```

There is no separate setup step in the ordinary path: the switch that makes
attribution meaningful is also the moment the shell gets configured (see
Alternatives 5). `agentdeck shell setup` remains supported for configuring a
shell before any route exists, for repairing a removed or damaged block, and for
non-interactive invocations where nothing is written automatically.

A `--via` switch that leaves no client eligible configures nothing and says so;
no output may imply that attribution is happening when it is not.

`agentdeck shell setup|status|remove` is the entire user-visible surface.
`shell-init` becomes a hidden compatibility primitive rather than something a
user is expected to know about; it cannot be deleted, for the reasons recorded
under its own heading.

**What "seamless" can and cannot mean here.** A one-time explicit step is
unavoidable: the injected value is the current directory's base name, so only
the shell knows it, and package installation can neither choose the right
startup file nor activate an already-running shell. What this plan does commit
to is that the step happens at the one moment the user's intent is unambiguous,
and nothing is ever required of them again — the `--via` switch configures every
shell they use, upgrading AgentDeck touches no startup file, later route changes
need no shell action and announce what changed, and a user who never routes
through a Headroom wrapper pays no per-invocation cost and gets no startup-file
write at all.

## Problem

Two independent gaps produced the same user-visible confusion. Fixing only the
first one leaves the second one intact.

**Gap 1: no installation lifecycle.** The released command behaves like this:

```text
agentdeck shell-init zsh
    -> prints codex() and claude() definitions
    -> does not evaluate them
    -> does not update .zshrc
    -> cannot change its parent shell
```

That behavior is technically sound but the user journey is not. The command
name and short help, "Generate project attribution shell helpers", do not tell
the user what remains to do. Homebrew installs command completion only. A user
must infer `eval`, choose and edit a startup file, restart the shell, and invent
their own verification step.

**Gap 2: the wrappers only act under conditions nothing reports.** Injection
requires all three conditions in `Service.RunProjectEnvironment`
(`internal/provider/service.go:710-721`): the client's current selection is
`ViaWrapper`, the wrapper URL still equals the selection endpoint, and the
wrapper's kind is `headroom`. A user whose functions are installed and sourced
can still get no attribution forever, and today nothing tells them why. A
lifecycle that reports only "configured" and "sourced" would show two green
states in exactly that situation.

So the defect is the missing lifecycle *and* the missing eligibility report.
Wrapper generation itself is sound.

## Evidence Baseline

Gathered on 2026-07-29 at `2db056b`, before implementation:

- `agentdeck shell-init zsh` from installed `v0.2.1-rc.1` prints both
  `codex()` and `claude()`.
- Evaluating that output in an isolated zsh makes both names shell functions.
- Running `shell-init` without evaluating its output leaves both names resolved
  to their binaries.
- The inspected user shell startup files contain no `agentdeck shell-init`
  activation line.
- `packaging/homebrew/agentdeck.rb.tmpl` installs bash, zsh, and fish command
  completion but no attribution wrapper.
- `docs/specs/cli-design.md` deliberately defines `shell-init` as stdout-only
  and says AgentDeck writes no file for this mechanism.
- `Service.RunProjectEnvironment` (`internal/provider/service.go:710-721`) is
  the single eligibility judgement: current selection `ViaWrapper`, wrapper URL
  still equal to the selection endpoint, and `WrapperKind == headroom`.
  `Service.ProjectAttributionGuidance` (lines 728-740) already repeats the same
  three checks, so a third copy must not be written.
- `writeProjectEnvironment` (`cmd/agentdeck/main.go:641-673`) returns `nil` with
  no output on five distinct failure paths — working-directory lookup, state
  root, read-only open, ineligibility, and a missing variable. That silence is
  correct for the wrapper's fail-open behavior, but it cannot be reused to
  explain *why* nothing was injected.
- The generated bash/zsh functions (`cmd/agentdeck/main.go:710-728`) and fish
  functions (lines 686-708) fork `command agentdeck shell-init
  --project-environment <client>` on **every** `codex` or `claude` invocation,
  which opens the state database read-only each time.
- `ProjectAttributionAdvisory` (`internal/provider/service.go:139`) still tells
  the user that "launches outside `agentdeck run` are not attributed". The
  shipped wrappers made that false. It is emitted by
  `reportProjectAttributionGuidance` (`cmd/agentdeck/main.go:1018-1023`) from
  both `provider use` (line 842) and `set-wrapper` (line 872), and it is
  unconditional: it does not know whether the user's shell is configured.
- `provider use` emits nothing when a switch *leaves* an eligible route, even
  though `reportDroppedWrapperKind` (`cmd/agentdeck/main.go:1025-1034`) already
  establishes the pattern for the analogous `set-wrapper --kind` case.
- `packaging/homebrew/agentdeck.rb.tmpl` has no `caveats` block and no
  `post_install` step; it installs the binary and generates completions, and
  its `test do` block asserts the three completion files exist.
- A managed startup-file block mechanism already exists, in shell rather than
  Go: `scripts/manage-install.sh:145-184` performs shell detection and default
  startup-file resolution (`${ZDOTDIR:-$HOME}/.zshrc`,
  `${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish`, `$HOME/.bash_profile`),
  and `scripts/test-completion-install.sh` already covers tampered, symlinked,
  duplicated, dangling, unowned, rollback, and interrupted cases for the
  completion block.

No `v0.2.1` stable tag or GitHub Release exists. A local L4
`make release-verify` completed successfully at the baseline tree, but that
evidence does not resolve this user-flow defect and does not authorize release.

## Alternatives

### 1. Keep stdout-only behavior and improve documentation

Improve `shell-init --help` and README examples, but require users to edit their
own startup files.

Rejected as the primary path. It makes the primitive clearer but still leaves
installation, persistence, removal, and verification as unrelated manual
steps. `shell-init` remains available for users who deliberately want this
control.

### 2. Modify startup files during package installation

Have Homebrew's `post_install`, or another installer step, write the managed
block automatically so no user action is required.

This is **technically available**, and two objections that earlier revisions of
this plan raised do not survive scrutiny. They are recorded as retracted so they
are not re-argued:

- *Retracted:* "the installer cannot tell which file to write." `post_install`
  runs as the invoking user with a correct `$HOME`, so `ZDOTDIR`,
  `XDG_CONFIG_HOME`, and the in-use detection rule defined under `shell setup`
  are all available there.
- *Retracted:* "it cannot activate an already-running shell." True, but
  `shell setup` cannot either. This is not a difference between the
  alternatives.
- *Retracted:* "a leftover block after uninstall breaks later shell startups."
  The guarded block body makes it inert.

Rejected on three consequences that do survive, in order of weight:

- **An upgrade re-imposes a choice the user already made.** `post_install` runs
  on every `brew upgrade`, and it cannot distinguish "this user deleted the block
  because they do not want it" from "this user has not been set up yet". So a
  deliberate removal gets silently undone at the next upgrade. Fixing that
  requires a persisted "declined" state that `shell remove` writes and
  `post_install` reads — turning one mechanism into four (`post_install`, the
  declined marker, an opt-out variable, idempotent rewriting) in service of
  something a single command already does.
- **Uninstall leaves a block the user never agreed to.** The guard makes it
  harmless, not absent, and a formula has no uninstall hook to remove it (that is
  a cask stanza). "Written without consent" plus "cannot be undone automatically"
  is worse than either alone.
- **Installation happens where this must not.** CI runners, container builds, and
  `brew bundle` all have a writable `$HOME`, so the block would be written there
  too and baked into images.

Homebrew's own convention — caveats, not dotfile edits — points the same way, but
it is a soft argument for a self-owned tap and is not load-bearing here.

The underlying wish is legitimate; only the timing is wrong. At
`brew install` time AgentDeck knows nothing about this user: not whether they have
a wrapper, whether it speaks Headroom, or whether they want attribution at all.
So automatic setup there can only be indiscriminate. Alternative 5 uses a moment
when the intent is unambiguous instead.

One related idea *is* adopted, just not at this layer: detecting every shell the
user has and configuring all of them rather than only the invoking one. That
belongs to `shell setup`, where the write is authorized and reportable per file.

The same reasoning rejects having AgentDeck write a project-scoped
`.claude/settings.local.json` to avoid the shell entirely: that is a user
project file, which `docs/specs/cli-design.md:235` forbids AgentDeck from
writing, and it has no Codex equivalent.

### 3. Add an explicit shell-integration lifecycle

Add `agentdeck shell setup`, `agentdeck shell status`, and
`agentdeck shell remove`. Keep `shell-init` as the documented low-level
generator.

Selected. The mutating action is explicit, ownership is narrow and reversible,
the current-shell limitation is stated rather than hidden, and package
installation remains non-mutating.

Alternatives 5 later added an automatic path on top of this, and the two are
consistent: the automatic write is still triggered by a command the user typed,
still limited to interactive invocations, still reported file by file, and still
reversible by the same `shell remove` — which additionally records the refusal.
Package installation remains the only layer that never writes.

### 4. Refuse setup until a Headroom route exists

Make `shell setup` an error when no client currently resolves through a
`headroom` wrapper.

Rejected. Provider selection is mutable state; a user may legitimately
configure the shell first and route later, and a route can be switched away and
back at any time. Refusing would also make the shell lifecycle depend on
provider state in a way that is invisible from the startup file. Instead, setup
succeeds and *reports* ineligibility, and `shell status` carries eligibility as
a first-class state.

### 5. Configure the shell when the user first opts into a Headroom route

Have `provider use <name> --via` install the managed block itself, the first time
a switch makes a client eligible.

**Selected**, and owned by task 7. Unlike alternative 2, this timing is
defensible: the user has just stated the intent the wrappers serve, AgentDeck is
executing a command they typed, and only users who actually need the integration
are ever touched. Objection A against alternative 2 disappears — a switch is not
re-run behind the user's back the way `post_install` is — and B weakens to "a
feature the user deliberately enabled left a trace".

Objection C does **not** disappear on its own, and this is the part the timing
argument does not cover: `provider use --via` is also run by scripts, provisioning
steps, and CI. Writing a startup file there would reproduce exactly the problem
that rejected alternative 2. So automatic setup happens only when the invocation
looks like a person at a terminal, and every non-interactive signal falls back to
the advisory and writes nothing.

What this costs, and what the design therefore commits to:

- `provider use` becomes a command that can edit shell startup files. That is
  stated in its output, in its help, and in the specification; it is never a side
  effect discovered later.
- It needs an opt-out that survives: a flag for one invocation, plus a persisted
  preference so a user who removes the block is not re-configured on the next
  `--via` switch. This is the same "declined" state alternative 2 needed, but here
  it is written by an explicit user action rather than inferred by a package
  manager.
- It never fails a switch that already succeeded. A startup-file write error
  degrades to the advisory telling the user to run `shell setup`.
- Only the first eligible switch writes; later switches must not rewrite a block
  that is already correct.

`agentdeck shell setup` remains fully supported and is no longer the ordinary
path: it is for configuring the shell before any route exists, for repairing a
removed or damaged block, and for the non-interactive cases above.

## Eligibility, Activation, and Cost

Three separate facts decide whether a `codex` invocation carries attribution.
Every one of them must be independently observable, because each fails in a way
the others cannot explain.

| Fact | Owned by | Failure mode when only this one is wrong |
| --- | --- | --- |
| Persistent configuration | the startup file | New shells never define the functions |
| Current-session activation | the running shell | This shell still resolves the binaries |
| Route eligibility | provider selection state | Functions run, fork AgentDeck, and inject nothing |

**Activation cannot be inferred from an exported variable alone.** Environment
variables are inherited by child processes; shell functions are not. A constant
`AGENTDECK_SHELL_INTEGRATION=1` would therefore still be set inside a `bash`
subshell, a `tmux` pane's new shell, or any script started from a configured
zsh, while `codex` there resolves to the binary — reporting active exactly when
the user is most likely to be confused. The marker must instead identify the
shell process that evaluated the script, and `shell status` must compare it
against its own parent process. A mismatch reports inactive and says the value
came from an ancestor shell.

The marker stays non-sensitive: a process identifier and the shell name, with no
project identity, path, provider, endpoint, credential, or routing value.

**Eligibility must be reported per client, with a reason.** `codex` and
`claude` are routed independently, so one may be eligible while the other is
not. The report reuses `Service.RunProjectEnvironment`'s judgement rather than
restating it, and distinguishes at least: eligible; no wrapper route; wrapper
not declared `headroom`; endpoint no longer matches the wrapper URL; and
undetermined because state could not be read. The last case is a diagnostic
error, so `shell status` must not reach it through
`writeProjectEnvironment`'s silent-`nil` path.

**The per-invocation cost is real.** Each wrapped `codex` or `claude` call forks
AgentDeck and opens the state database read-only. Task 5 measures the added
latency once on a real machine and records it, and the manual discloses it next
to the installation instructions.

**No cache may make attribution happen.** Provider selection can change between
two invocations in the same shell, so a cache that answers "eligible" from
stale state would attribute a launch to a route it no longer uses — silently,
which is worse than the cost. Every positive answer therefore comes from the
full check.

**The wrappers stay installed unconditionally and decide per invocation.**
Nothing installs or uninstalls them as routes change; they are defined once and
resolve their behavior at call time, in this order:

1. Is `agentdeck` on `PATH`? If not, invoke the real client unchanged. This
   mirrors the guard in the managed block and keeps a wrapper harmless after the
   binary is removed from a still-running shell.
2. Ask `agentdeck shell env <client>` once. Empty output means not eligible for
   any reason, so invoke the real client unchanged.
3. Otherwise set that client's variable to the returned value and invoke the real
   client.

Step 2 is the only AgentDeck call, and it both decides and returns the value.

**A cheap negative gate sits in front of step 2, because it can only suppress.**
For the majority of users — everyone not routing through a Headroom wrapper —
that fork buys nothing on every single `codex` invocation forever. The wrappers
therefore test one AgentDeck-owned marker file before forking, and invoke the
real client directly when it is absent:

- The marker lives in the state root, is mode `0600`, and is **empty**: its
  existence is the whole signal. It carries no project, path, provider,
  endpoint, credential, or route value.
- `provider use` maintains it: created when a completed selection is eligible,
  removed when no client is eligible any more. It is derived machine-local state,
  so it is excluded from portable backups.
- It is authoritative only for "no". When present, the wrapper still forks and
  performs the full three-condition check, so a stale marker cannot cause a
  wrong attribution — only an unnecessary fork.
- Every drift path — a hand-edited `~/.codex/config.toml`, a restored backup, an
  interrupted switch — therefore fails toward *not* attributing, which is the
  same direction the wrappers already fail in.
- `shell status` and `doctor` run the full check regardless of the marker, so a
  marker that disagrees with real eligibility is reported as a fixable
  inconsistency with the command that repairs it, rather than being invisible.

The task implementing this decides and documents how the marker is named and how
a missing-but-eligible state is reported. It is part of the delivery, not an
optional optimization: without it, the overwhelmingly common case — a user with
no Headroom wrapper who nonetheless has the integration installed — pays for
AgentDeck on every client launch forever.

Two consequences the implementing task must handle explicitly:

- **The marker is machine-local derived state, not a source of truth.** It is
  excluded from portable backups, and a restore onto a new machine legitimately
  arrives without it. The first `provider use` on that machine re-establishes it.
- **It must survive being wrong.** Deleting it by hand, or a crash between the
  selection commit and the marker write, degrades to "attribution stops until the
  next `provider use`", which `shell status` and `doctor` both surface.

## Command Surface

### `agentdeck shell-init <bash|fish|zsh>`

This top-level command stays backward compatible but becomes **hidden**: it
disappears from help listings and from every recommended path, while remaining
callable. The user-visible surface is `agentdeck shell setup|status|remove`.

It cannot simply be deleted, for three independent reasons:

1. The managed block's body *is* `eval "$(command agentdeck shell-init zsh)"`.
   That indirection is what makes upgrading AgentDeck require no dotfile
   rewrite. Inlining the function bodies instead would freeze each user's
   wrappers at the version that ran `shell setup`.
2. The generated function bodies call `shell-init --project-environment
   <client>` at run time. Removing the top-level command would move that
   interface out from under already-installed blocks.
3. `v0.2.1-rc.1` shipped through the Homebrew RC tap, and the manual told users
   to add `eval "$(agentdeck shell-init zsh)"` themselves. Deleting the command
   would break every such shell's startup with `command not found`.

Required short help, kept accurate for anyone who calls it deliberately:

```text
Print shell wrapper functions to stdout; does not install or activate them
```

Long help and examples must cover:

- stdout is a shell program, not a status report;
- running the command alone changes no shell state and writes no file;
- bash and zsh current-session activation:

  ```text
  eval "$(agentdeck shell-init zsh)"
  ```

- fish current-session activation:

  ```text
  agentdeck shell-init fish | source
  ```

- persistent installation uses `agentdeck shell setup`;
- the generated functions remain fail-open: if AgentDeck cannot resolve an
  eligible Headroom route, they invoke the real `codex` or `claude` command
  without attribution injection.

`shell-init` remains outside the JSON data contract because its stdout must be
directly sourceable. Non-text `--format` values must fail with a clear
`invalid_argument` error instead of emitting shell code under a JSON request.

### `agentdeck shell env <codex|claude>`

The resolver the generated wrappers call on every invocation, promoted from the
hidden `shell-init --project-environment <client>` flag to a documented command
because installed startup files already depend on it.

It answers "what should this client's attribution variable be, here, right now"
in **one** call, so the wrapper needs no parser:

- eligible: print the value for that client's variable — the percent-encoded
  project base name for Codex, the header line for Claude — with no trailing
  newline ambiguity the shell must strip;
- not eligible, for any of the three reasons, or no project identity: print
  nothing and exit `0`. Absence of output is the signal;
- unreadable state, missing state root, or an unusable working directory: print
  nothing and exit `0` as well, because the wrapper must never fail a launch;
- an unsupported client argument: the standard error envelope with exit `2`.

Deliberately *not* built on `provider current`. That command's
`CurrentSelection` (`internal/provider/service.go:681-704`) carries `ViaWrapper`
and `Endpoint` but no `wrapper_kind`, so it cannot decide eligibility on its
own; a wrapper would have to parse structured output without a guaranteed `jq`
and then call AgentDeck a second time anyway to get the encoded value, since the
encoding rules live in `ProjectWireValue`
(`internal/provider/project.go:27-40`). One call that returns the final value
keeps the encoding in Go and the shell side trivial.

`shell-init --project-environment <client>` remains as a hidden alias with
identical behavior, because every managed block written by `v0.2.1-rc.1` users
calls it.

### `agentdeck shell setup [bash|fish|zsh]`

**With no shell argument, configure every supported shell the user actually
uses, not only the invoking one.** One person routinely uses more than one shell
— a fish login shell plus zsh for interactive work is an ordinary setup — and
configuring only the shell that happened to run the command reproduces the same
"why isn't it active here" confusion this plan exists to remove.

A shell is treated as in use when either its default startup file already
exists, or it is the shell that invoked this command. The invoking shell is
always included, and its startup file is created if absent. A shell that is
merely installed, with no startup file and no involvement in this invocation, is
left alone.

An explicit positional argument restricts the operation to that shell.

Options:

- `--shell <bash|fish|zsh>` is the flag form of the positional argument, so
  scripts do not have to rely on argument position.
- `--rc <path>` selects a non-default startup file, and therefore implies
  exactly one shell.
- No implicit package-manager or first-run invocation is allowed. Running
  `shell setup` is the user's authorization to update the selected files.

Multi-shell behavior:

- Each shell is reported on its own line with its own path and outcome, so
  "configured two, unchanged one" is legible.
- One shell's failure never prevents the others from being attempted. The exit
  status reflects that something failed, and the output names which.
- Every per-shell rule below — idempotence, hash verification, refusal to
  overwrite an edited block — applies per file, independently.

Default startup files:

| Shell | Default |
| --- | --- |
| zsh | `${ZDOTDIR:-$HOME}/.zshrc` |
| fish | `${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish` |
| bash login shell | `$HOME/.bash_profile` |
| bash non-login shell | `$HOME/.bashrc` |

The command installs one versioned AgentDeck-owned block. The body **must guard
on AgentDeck's presence before invoking it**, because a startup file outlives the
binary:

```text
# >>> agentdeck shell integration >>>
# managed-hash: <sha256-of-managed-body>
command -v agentdeck >/dev/null 2>&1 && eval "$(command agentdeck shell-init zsh)"
# <<< agentdeck shell integration <<<
```

Fish guards with its own idiom:

```text
if type -q agentdeck
    command agentdeck shell-init fish | source
end
```

The guard is not an optimization. `brew uninstall` cannot run an AgentDeck
cleanup step (see "Installation and Upgrade Flow"), so an unguarded block would
turn every later shell startup into `agentdeck: command not found` for anyone who
removes the binary without running `agentdeck shell remove` first. With the
guard, a leftover block is inert: the shell starts silently and `codex` and
`claude` resolve to their real binaries.

The guard must not swallow real failures. Only AgentDeck's absence is silent; a
present-but-failing `shell-init` must not be hidden by the same test.

On success, text output must distinguish persistent configuration from current
activation, and must state route eligibility rather than leaving the user to
discover it later:

```text
zsh:  configured /path/to/.zshrc
fish: configured /path/to/.config/fish/config.fish
bash: skipped, no startup file and not the invoking shell
Both configured shells will be active in new sessions.
To activate it in this session (zsh):
  eval "$(agentdeck shell-init zsh)"
Attribution: codex eligible; claude has no Headroom wrapper route.
Until a client routes through a wrapper declared headroom, its wrapper runs and
injects nothing.
```

The current-session activation line names only the invoking shell, because that
is the only shell this process could possibly be inside.

The command must never claim that the current parent shell changed, and must
never imply that attribution is in effect for an ineligible client.

Ineligibility never fails the command: setup before routing is a legitimate
order of operations, per rejected alternative 4.

Repeated setup is idempotent. If the installed block matches the requested
state, report it unchanged. If an older valid AgentDeck block is present,
replace it atomically. If the block was edited, duplicated, truncated, or has
an invalid stored hash, refuse to overwrite it and explain how to inspect or
remove it manually.

### `agentdeck shell status [bash|fish|zsh]`

With no argument, report every supported shell that has a startup file or is the
invoking shell, using the same in-use rule as `setup`. An argument restricts the
report to one shell. Route eligibility is shell-independent and is reported once.

For each reported shell, give the three independent states from "Eligibility,
Activation, and Cost":

1. **Persistent configuration**: absent, configured, modified, or invalid,
   including shell and startup-file path.
2. **Current session activation**: active, inactive, or inherited-from-ancestor.
3. **Route eligibility, per client**: eligible, no wrapper route, wrapper not
   `headroom`, endpoint drifted, or undetermined.

The sourced script exports a non-sensitive marker identifying the shell process
that evaluated it — its process identifier and the shell name, nothing else.
`shell status` compares that identifier with its own parent process:

- identifiers match: the calling shell evaluated the script, report **active**;
- marker present but identifiers differ: an ancestor shell was configured and
  this shell inherited only the variable, so functions are absent here. Report
  **inactive** and name the cause;
- marker absent: report **inactive**.

The comparison errs toward inactive by design. An invocation reached through an
intermediate process — `zsh -c`, a pipeline subshell, `env`, a wrapper script —
may not have the evaluating shell as its parent, and will report inactive even
though the interactive shell is configured. That is the safe direction and is
not a defect: the failure mode being avoided is reporting active when `codex`
resolves to the binary.

Eligibility comes from the same judgement `agentdeck run` and the generated
wrappers use, and its undetermined state must surface the underlying read
failure instead of being silently flattened.

`shell status` never resolves or prints a project value. Reporting eligibility
must not require computing the attribution payload.

Text output example:

```text
zsh   /path/to/.zshrc                      configured   session: inactive (marker inherited from an ancestor shell)
fish  /path/to/.config/fish/config.fish    configured   session: active
bash  /path/to/.bashrc                     absent       session: inactive
Attribution: codex eligible; claude wrapper is not declared headroom
Next action: eval "$(agentdeck shell-init zsh)"
```

At most one shell can report `session: active`, since only the invoking shell's
marker can match this process's parent.

Normal `--format json` output must expose stable fields for all three states,
with the per-shell states as a collection and the per-client eligibility reason
as a stable enumerated value rather than prose. Inspection does not modify any
startup file or AgentDeck state.

### `agentdeck shell remove [bash|fish|zsh]`

Remove only the valid AgentDeck-owned integration block. Preserve every other
byte of the startup file, including the independently managed completion block.

Removal is symmetric with setup: with no argument it removes the block from every
supported shell that has one, so a user who configured three shells does not have
to remember which three. Per-shell results are reported individually, and one
refusal does not prevent the other removals.

If no block exists, succeed idempotently and report unchanged. If the block is
modified or invalid, refuse automatic removal rather than deleting user-edited
content.

The command cannot remove functions already loaded in the parent shell. Its
success output must state that new shells are clean and print a shell-specific
current-session deactivation command. Each printed command must tolerate an
absent function and must not delete a same-named function the user defined
themselves, so the unconditional form is not acceptable: `unfunction codex
claude` fails in zsh when either name is not a function, and neither
`unfunction` nor `unset -f` can tell an AgentDeck wrapper from a user's own.

| Shell | Current-session deactivation |
| --- | --- |
| zsh | guarded per name, e.g. `(( $+functions[codex] )) && unfunction codex` |
| bash | guarded per name, e.g. `declare -F codex >/dev/null && unset -f codex` |
| fish | `functions -q codex; and functions --erase codex` |

The task decides the exact snippets; the requirement is that they are
name-by-name, guarded, and copyable. Whether the wrapper can be distinguished
from a user-defined function of the same name must be settled explicitly during
implementation: if it cannot, the output says so rather than implying a
targeted removal.

## Safe File Ownership

Shell startup files are user-owned and require the same caution as provider
configuration:

- accept only a regular file owned by the current user, never a symlink;
- resolve the selected path before mutation and reject newline-bearing or
  otherwise unsafe paths;
- create a missing startup file with mode `0600`;
- preserve an existing file's mode and unrelated content;
- write a same-directory temporary file, flush it, and atomically replace the
  destination;
- make the managed body deterministic and verify its stored SHA-256 before
  update or removal;
- roll back on a failed replacement;
- never touch completion markers or any non-AgentDeck block;
- never persist generated project values or credentials.

The block is self-describing. No machine-specific startup-file path is added to
portable backup data.

## Relationship to the Managed Completion Installer

A managed startup-file block mechanism already exists, but in shell, not Go:
`scripts/manage-install.sh:145-184` detects the invoking shell and resolves the
default startup file, and `scripts/test-completion-install.sh` already covers
tampered, symlinked, duplicated, dangling, unowned, rollback, and interrupted
cases. The Go implementation of `shell setup` cannot call into it — Homebrew
installs never run that script — so this plan knowingly creates a **second**
implementation of the same class of behavior rather than reusing one.

That is accepted, with conditions:

- Shell detection and default startup-file paths must produce the same answers
  as `manage-install.sh` for every supported shell, including `ZDOTDIR`,
  `XDG_CONFIG_HOME`, and the bash login/non-login split. Any deliberate
  divergence is a documented decision, not an accident.
- The two block markers must be distinct, and neither implementation may parse,
  rewrite, validate, or hash the other's block.
- Both blocks must be able to coexist in one startup file in either order, and
  `shell setup`, `shell remove`, completion install, and completion uninstall
  must each leave the other block byte-identical.
- Where the shell implementation's safety rules are stricter than a naive Go
  port would be, the Go side adopts the stricter rule.

## Installation and Upgrade Flow

Homebrew continues to install the binary and command completions without
editing startup files. Its formula adds a caveat, and the caveat must be
conditional rather than a blanket recommendation, because a user with no
Headroom wrapper gains nothing from the integration and pays a fork plus a
read-only database open on every `codex` or `claude` invocation:

```text
Project-attribution wrappers are optional and only act when a provider routes
through a wrapper declared headroom. If you use one, this configures every
shell you use:
  agentdeck shell setup
To undo it later:
  agentdeck shell remove
Command completion is already installed and needs no further action.
```

The repository-local installer prints the same conditional next step after
installing the binary and completion. It does not invoke setup automatically.
No installation path may present `shell setup` as a required step.

**Uninstall cannot clean up after itself.** A Homebrew *formula* has no
uninstall hook — `post_install` and `caveats` exist, but removal only deletes the
keg's files. The `uninstall` and `zap` stanzas belong to casks, and AgentDeck
ships as a CLI formula. Removal is therefore covered from two directions, both
required:

- `agentdeck shell remove` is the explicit path, and the caveat names it as the
  way to undo `shell setup`;
- the guarded block body makes a block that outlives the binary inert rather than
  fatal, so a user who only runs `brew uninstall` still gets working shells.

The same applies to the repository-local uninstall path: it may remove what it
installed, but it must not assume it is the only way AgentDeck leaves a machine.

Upgrading AgentDeck does not need to rewrite the startup file because the
managed block calls the installed `agentdeck shell-init` dynamically. A later
block-schema change is applied only through another explicit, idempotent
`agentdeck shell setup`.

## Lifecycle Walkthrough

The delivered behavior, end to end. This is the baseline task 4 documents and
task 5 verifies; every row is a state a real user reaches.

### Fresh install

```text
brew install kitdine/tap/agentdeck
  -> binary installed
  -> bash, zsh, fish completion installed
  -> caveat: attribution wrappers are optional; if you use a Headroom wrapper,
     run "agentdeck shell setup"; undo with "agentdeck shell remove"
  -> no startup file touched
```

Nothing about attribution works yet, and nothing pretends to. A user who ignores
the caveat has exactly today's AgentDeck.

### Upgrade

```text
brew upgrade kitdine/tap/agentdeck
  -> binary replaced, completions refreshed
  -> startup files untouched, because the managed block calls shell-init
     dynamically rather than embedding the wrapper text
  -> already-running shells keep the wrappers they loaded; new shells pick up
     the new binary's wrappers automatically
```

No `shell setup` re-run is required by an upgrade. Only a change to the managed
block's own schema would need one, and that is applied by an explicit,
idempotent `shell setup`.

### Enabling the integration

The ordinary path does not use a separate command. The first `--via` switch that
makes a client eligible configures the shell, when run interactively:

```text
agentdeck provider use <headroom-wrapper-provider> --via
  -> selection written, marker created
  -> configures every shell in use (existing startup file, or the invoking shell)
  -> reports each shell, its path, and its outcome
  -> states that new sessions are covered and prints the one-line command for
     this session
```

The explicit command remains, for the cases the automatic path deliberately does
not cover:

```text
agentdeck shell setup
  -> same configuration, same per-shell report
  -> also reports current route eligibility per client
  -> clears a previously declined preference
```

Use it to configure a shell before any route exists, to repair a block that was
removed or edited, or after any non-interactive switch. Nothing is written
automatically when stderr is not a terminal, when `--format json` or `ndjson` is
requested, when `--quiet` is passed, when `--no-shell-setup` is passed, or when
the user previously ran `shell remove`.

Once configured, the wrappers are permanent and unconditional. What they *do* is
decided per invocation, which is the next section.

### What a `codex` or `claude` invocation does

| Route state | Marker | Wrapper behavior | Cost |
| --- | --- | --- | --- |
| No wrapper configured, or a direct switch | absent | invoke the real client unchanged | one `test -e` |
| Wrapper configured but declared `plain` | absent | invoke the real client unchanged | one `test -e` |
| Wrapper declared `headroom`, but the switch was not `--via` | absent | invoke the real client unchanged | one `test -e` |
| Wrapper declared `headroom`, selected `--via`, endpoint still matches | present | inject this project's attribution, then invoke the real client | one `test -e` plus one `shell env` call |
| Eligible for one client only | present | inject for the eligible client; the other client's wrapper injects nothing | one `test -e`; a `shell env` call only for the client being launched |
| Endpoint drifted away from the wrapper URL | present, stale | `shell env` returns empty, so invoke the real client unchanged | one `test -e` plus one `shell env` call |
| AgentDeck no longer on `PATH` | not consulted | invoke the real client unchanged | one `command -v` |
| The user already set the variable themselves | present | leave their value alone and invoke the real client | one `test -e` plus one `shell env` call |

Nothing in this table requires the user to act, restart a shell, or re-run
`setup`. The wrappers follow provider state.

### Switching routes

```text
agentdeck provider use <headroom-wrapper-provider> --via     # already configured
  -> selection written
  -> marker created
  -> advisory: attribution is in effect
  -> no startup file rewritten, because a valid block is already installed
  -> nothing to do in any running shell

agentdeck provider use <headroom-wrapper-provider> --via     # not configured yet
  -> as above, plus the shell configuration described under "Enabling the
     integration" when the invocation is interactive
  -> when it is not, the advisory names "agentdeck shell setup" instead

agentdeck provider use <something-else>          # or a non-headroom wrapper
  -> selection written
  -> marker removed when no client is eligible any more
  -> advisory, only if the integration is configured: the wrappers stay
     installed but stop injecting from now on
  -> no startup file written or removed; the wrappers are left in place because
     the next switch back must need no action
  -> nothing to do in any running shell
```

### Disabling and uninstalling

```text
agentdeck shell remove
  -> removes the block from every configured shell, reporting each
  -> preserves the completion block and every unrelated byte
  -> records that the integration was declined, so no later --via switch
     reinstalls it; "agentdeck shell setup" is what reverses that
  -> new shells are clean; prints the guarded per-name command to drop the
     functions from this session

brew uninstall kitdine/tap/agentdeck
  -> binary and completions removed
  -> a startup-file block that was never removed stays behind, and is inert:
     its presence guard finds no agentdeck and the shell starts silently
```

A formula cannot clean the block up itself, which is exactly why the guard is
mandatory rather than cosmetic.

## Error and Output Contracts

- Unsupported shell and unsafe startup-file paths are input errors.
- A modified, duplicated, or malformed managed block is a conflict error with
  no file change.
- File permission, ownership, temporary-write, flush, rename, and rollback
  failures are operational errors.
- Text output always names every selected shell, its startup-file path, its
  persistent state, the current-session state where applicable, per-client route
  eligibility, and the exact next action.
- A multi-shell operation reports per-shell outcomes and returns a non-zero
  status when any shell failed, without hiding the shells that succeeded.
- An undetermined eligibility state is reported as a problem with its cause, not
  silently rendered as ineligible. `shell env` is the one exception, and it is
  deliberate: it is called on every client launch, so it stays silent and exits
  `0` rather than ever putting text between the user and their client.
- JSON output never includes startup-file contents or generated wrapper text.
- `--quiet` suppresses success narration but not errors.
- No command prints project identity, headers, credentials, or provider
  secrets. Reporting eligibility never computes or emits a project value.

## Tasks

### 1. `shell-command-surface`

Add the `shell` command group with `setup`, `status`, `remove`, and `env`. Hide
`shell-init` while keeping it working, and strengthen its short help, long help,
examples, and non-text format rejection.

Acceptance:

- `agentdeck shell env codex` prints the value for an eligible route, prints
  nothing and exits `0` for every ineligible or unreadable case, and returns the
  standard envelope with exit `2` for an unsupported client;
- `shell-init --project-environment <client>` remains available as a hidden alias
  and produces byte-identical output to `shell env <client>`;
- `shell-init` no longer appears in `agentdeck --help`, while
  `agentdeck shell-init zsh` still runs and its script output is unchanged apart
  from the activation marker and the presence guard;
- `agentdeck shell-init --help` states stdout-only, no install, and no
  activation without requiring the manual.
- Help states that the wrappers act only under an eligible Headroom route and
  that they otherwise invoke the real client unchanged.
- Each supported shell has a copyable current-session example.
- `agentdeck shell --help` presents setup, status, and remove as one lifecycle,
  and states that they cover every shell in use by default.
- `--shell` and the positional argument are equivalent, and `--rc` is rejected
  when it would apply to more than one shell.
- Existing `shell-init <shell>` source output stays backward compatible apart
  from the activation marker.

Because an installed managed block calls `agentdeck shell-init` dynamically, the
generated function bodies' call to `shell-init --project-environment <client>`
becomes a long-lived interface the moment any user's startup file references it.
This task records that: the flag stays hidden, but its accepted clients, stdout
shape, silent-empty-output behavior, and exit codes may not change
incompatibly while a released managed block depends on them.

Verification: L1 targeted `cmd/agentdeck` help, argument, format, and output
tests.

### 2. `managed-shell-config`

Implement the versioned, hashed managed-block parser and atomic startup-file
editor in Go, under the conditions in "Relationship to the Managed Completion
Installer". Port the shell detection and default-path rules from
`scripts/manage-install.sh:145-184` deliberately, matching its answers rather
than inventing new ones.

Acceptance:

- setup is idempotent and preserves unrelated bytes and file mode;
- remove deletes only a valid integration block;
- missing files, missing final newlines, duplicate markers, modified blocks,
  symlinks, wrong ownership, and injected file-operation failures have explicit
  regression coverage;
- shell detection and default startup-file resolution agree with
  `manage-install.sh` for zsh with and without `ZDOTDIR`, fish with and without
  `XDG_CONFIG_HOME`, and both bash login and non-login cases;
- the in-use rule selects exactly the shells with an existing startup file plus
  the invoking shell, creates the invoking shell's file when absent, and leaves an
  installed-but-unused shell untouched;
- a multi-shell run reports each shell separately, and a failure or refusal on one
  shell neither aborts nor silently swallows the others, with the exit status
  reflecting the failure;
- `remove` with no argument clears every configured shell and reports each;
- with both a completion block and an integration block present in one file, in
  both orders, each of setup, remove, completion install, and completion
  uninstall leaves the other block byte-identical;
- the installed block guards on AgentDeck's presence for every supported shell,
  and sourcing a block with no `agentdeck` on `PATH` starts the shell silently
  with `codex` and `claude` still resolving to their real binaries;
- a present-but-failing `agentdeck` is not silenced by that guard.

Verification: L3 targeted package tests, full vendored suite, race, and vet
because shell startup-file mutation and rollback are involved.

### 3. `activation-and-eligibility-status`

Add the process-identifying activation marker to generated scripts and implement
all three reported states.

Acceptance:

- sourcing each generated script makes `shell status` report current-session
  active in that shell;
- a child shell started from a configured shell reports inactive and names the
  inherited marker as the reason, even though the variable is set;
- an installed but not yet sourced block reports configured plus inactive;
- a modified block reports conflict without exposing its contents;
- eligibility is reported per client and reuses
  `Service.RunProjectEnvironment`'s judgement, without adding a third copy of
  the three checks;
- each ineligibility reason — no wrapper route, wrapper not `headroom`, endpoint
  drift — is reported distinctly, and an unreadable state surfaces as an
  undetermined diagnostic rather than as ineligible;
- no status path prints or computes a project value;
- a status run covering several shells reports at most one as session-active, and
  reports eligibility once rather than per shell;
- JSON and text agree on the per-shell collection — shell, path, configuration
  state, activation state — and on the per-client eligibility reason.

Verification: L2 targeted command and renderer tests plus full vendored suite.

### 4. `installation-onboarding`

Add Homebrew caveats and repository-local installer narration. Update README
installation instructions and CLI manual so the same path appears everywhere,
including its prerequisites.

Acceptance:

- package installation never edits shell integration automatically;
- install output presents `agentdeck shell setup` as optional and conditional on
  using a Headroom-declared wrapper, never as a required step;
- documentation separates command completion from attribution wrappers;
- documentation states the prerequisites and the measured per-invocation cost
  from task 5, so declining the integration is an informed choice;
- no text implies `shell-init` alone activates anything.

Verification: L1 formula-rendering, installer-output, and documentation checks.

### 5. `cross-shell-acceptance`

Extend isolated installation coverage for bash, zsh, and fish.

This task also carries one product fix folded in on 2026-07-30, because its
acceptance is what would have caught it: the generated wrappers omit step 1 of
the resolution order under "Eligibility, Activation, and Cost" — the
`agentdeck`-on-`PATH` test before forking. The fish bodies
(`cmd/agentdeck/main.go:1074`, `:1084`) call
`command agentdeck shell-init --project-environment <client>` inside a command
substitution with no `type -q agentdeck` guard, so with the block installed and
`agentdeck` off `PATH` — the state after a bare `brew uninstall` — every `codex`
invocation prints `Unknown command` and a source excerpt to stderr before running
the real client. The bash and zsh bodies (`:1098`, `:1107`) are silent only
because their `2>/dev/null` catches the shell's own error, so all three get the
explicit guard rather than only fish. Recorded in
`docs/reviews/shell-integration/installation-onboarding.md` Round 1.

Acceptance for every supported shell:

- each generated wrapper tests for `agentdeck` on `PATH` before forking it, and
  invoking a wrapped client with `agentdeck` absent produces empty stderr as well
  as an unchanged client launch;
- `scripts/test-completion-install.sh`'s `unavailable` mode captures stderr and
  asserts it is empty, so the guard cannot regress behind a stdout-only
  assertion;

- setup writes the expected block, and a single no-argument `shell setup` in a
  home configured for two shells configures both and leaves the third alone;
- a fresh shell loads `codex` and `claude` as functions;
- eligible Headroom routes inject the expected client-specific environment;
- direct or ineligible routes invoke the real client without injection;
- status distinguishes configured, active, inherited-marker, and per-client
  eligibility states;
- remove preserves unrelated startup content and completion, and its printed
  deactivation command succeeds when the functions are absent;
- a fresh shell after remove resolves the real commands;
- switching into and out of an eligible route produces the task 6 advisories in
  a real shell, and the wrappers' behavior follows the switch without any further
  user action;
- a first interactive `--via` switch in an unconfigured isolated home configures
  the shell, and a fresh shell then loads the wrappers with no `shell setup` run
  at any point;
- the same switch run with `--quiet` writes no startup file, and the fresh shell
  afterwards has no wrappers;
- with no eligible route the wrapper invokes the real client without forking
  AgentDeck at all, and a route switch flips that behavior with no shell restart;
- with the block installed and `agentdeck` removed from `PATH` — the state after
  a bare `brew uninstall` — a fresh shell starts with no error output and both
  clients run unchanged.

This task also measures the per-invocation overhead the wrapper adds — one
AgentDeck fork plus one read-only database open — on a real machine, for an
eligible route, an ineligible route with the marker present, and an ineligible
route gated by an absent marker, and records the numbers in this plan. The
measurement is evidence for the disclosure in task 4, not a performance gate.

Measured on 2026-07-30 using the current worktree binary on an Intel
`darwin/amd64` host running macOS 26.6 and Go 1.26.5. The harness sourced the
generated bash wrapper, invoked a no-op external `codex`, and took the median
of 9 batches of 200 invocations. The direct-client median was 33.83 ms per
invocation. Results:

| Wrapper path | Median total | Added versus direct client |
| --- | ---: | ---: |
| eligibility marker absent | 31.62 ms | not applicable; the gate starts no AgentDeck process |
| marker present, route ineligible | 145.85 ms | +112.02 ms |
| route eligible | 185.84 ms | +152.02 ms |

These are disclosure measurements for this host and sandbox, not a performance
gate or a cross-machine guarantee.

Disclosure decision: the exact measurements remain in this plan, where their
host and method are recorded. The Homebrew formula caveat,
`scripts/manage-install.sh`, `README.md`, and `docs/specs/cli-manual.md`
describe the marker-dependent cost shape: no AgentDeck process when the marker
is absent; one AgentDeck process and one read-only database access per
invocation when it is present. Those user-facing texts include only the
host-qualified order of magnitude (roughly 0.1-0.2 seconds on the measured
Intel macOS 26.6 host), and their assertions pin that structural disclosure
rather than the exact table values.

Verification: L3 isolated-home integration tests. The final release candidate
uses the project L4 `make release-verify` aggregate gate once after the last
relevant edit.

### 6. `route-change-advisories`

Make provider switching tell the user what changed for attribution, so nobody
has to run `shell status` to find out. This closes a live accuracy defect as
well: `ProjectAttributionAdvisory` (`internal/provider/service.go:139`) still
says launches outside `agentdeck run` are not attributed, which stopped being
true when the wrappers shipped.

The advisory becomes aware of both the route and the shell configuration state:

| Switch | Shell integration configured | Not configured |
| --- | --- | --- |
| Into an eligible Headroom `--via` selection | attribution is in effect; new shells carry it, and name the one-line command for the current session | attribution needs a one-time `agentdeck shell setup` |
| Out of it, to a direct route or a non-`headroom` wrapper | the wrappers remain installed but stop injecting from now on | say nothing; it is irrelevant |

Follow the established advisory rules, which `reportDroppedWrapperKind`
(`cmd/agentdeck/main.go:1025-1034`) already demonstrates for the analogous
`set-wrapper` case: stderr only, never in the JSON envelope, no effect on exit
status, suppressed by `--quiet`, and never a reason to fail a switch that
already succeeded.

Acceptance:

- each of the four cells produces its own distinct output, verified through the
  command layer rather than only at the service layer;
- switching out of an eligible route warns only when the shell integration is
  actually configured;
- reading the shell configuration state must not fail a completed switch: an
  unreadable startup file degrades to the unconfigured wording;
- no advisory prints a project value, endpoint, or credential;
- `--quiet` and JSON output are unchanged.

Writing the managed block on a first eligible switch is task 7, which builds on
the route and configuration-state reads this task adds. Keep the two separable:
the advisory must be correct on its own, including for every case where task 7
deliberately writes nothing.

This task also owns the negative-gate marker from "Eligibility, Activation, and
Cost", because `provider use` is where eligibility changes:

- created when a completed selection leaves at least one client eligible, removed
  when none is;
- written with the state root's `0700`/`0600` rules, empty, and excluded from
  portable backups;
- a marker write or removal failure never fails a switch that already succeeded,
  and is surfaced by `shell status` and `doctor` rather than by the switch;
- covered by a regression that a stale or missing marker changes only whether a
  fork happens, never which attribution value is produced.

Verification: L2 targeted `internal/provider` and `cmd/agentdeck` tests plus the
full vendored suite, since switch output and state-root files are involved.

### 7. `switch-time-setup`

Implement Alternatives 5: `provider use --via` configures the shell itself on the
first switch that makes a client eligible, so the ordinary path needs no separate
`shell setup`.

Scope:

- On a completed switch that leaves at least one client eligible, and only when no
  valid managed block is already installed, install it using task 2's editor and
  the same in-use shell detection as `shell setup`.
- **Interactive-only.** Write nothing, and fall back to task 6's advisory, when
  the invocation is not a person at a terminal. Treat a non-TTY stderr,
  `--format json` or `--format ndjson`, and `--quiet` as non-interactive; the task
  decides whether any further signal is needed and documents the exact rule. This
  is what keeps Alternatives 2's CI objection from returning through a different
  door.
- `--no-shell-setup` suppresses it for one invocation.
- A persisted preference suppresses it permanently, written by
  `agentdeck shell remove` so that removing the block is understood as declining
  it. `agentdeck shell setup` clears that preference, because running setup is the
  opposite statement.
- Output names every file written, in the same per-shell form `shell setup` uses,
  and states that new shells are covered plus the one-line command for this
  session.
- Any detection, permission, hash-conflict, or write failure degrades to the
  advisory. A switch that already succeeded never fails, and never reports failure,
  because of shell configuration.

Acceptance:

- a first eligible interactive `--via` switch configures every in-use shell and
  reports each file;
- a second eligible switch with a valid block already present writes nothing and
  reports nothing about configuration;
- `--quiet`, `--format json`, `--format ndjson`, and a non-TTY stderr each write
  nothing and fall back to the advisory, verified per signal;
- `--no-shell-setup` writes nothing for that invocation only;
- after `shell remove`, later eligible switches write nothing until
  `shell setup` runs again;
- an unwritable startup file, a wrong-ownership file, and a tampered block each
  leave the switch successful with an advisory and no partial write;
- JSON output for `provider use` gains no startup-file content and no wrapper
  text.

Verification: L3 targeted `internal/provider`, `cmd/agentdeck`, and startup-file
editor tests, plus the full vendored suite, race, and vet, because a
non-interactive path now mutates user files.

### 8. `contract-and-release`

Update the living specification and manual only after implementation behavior
is reviewed:

- replace the current "writes no file" shell-helper rule with the explicit
  setup/status/remove contract, scoped to the AgentDeck-owned managed block;
- state that the integration is optional, that the wrappers act only under an
  eligible Headroom route, and that eligibility is observable per client;
- preserve stdout-only `shell-init` as a hidden compatibility primitive and
  record why it cannot be removed;
- define `agentdeck shell env <codex|claude>` as the supported resolver contract,
  including its silent-empty-exit-`0` behavior, and record that
  `shell-init --project-environment` stays a hidden alias for as long as
  released managed blocks call it;
- state that the managed block guards on AgentDeck's presence, and that a block
  left behind by an uninstall is inert rather than fatal;
- state that the lifecycle commands cover every shell in use by default, and
  define the in-use rule;
- write the `setup`/`status`/`remove` conventions down as reusable rather than
  shell-specific — subcommand shape, state vocabulary, idempotence, "remove
  touches only AgentDeck-owned entries", refusal to overwrite an edited managed
  region, and no setup from any package installation path — because
  [the runtime provider attribution plan](../../plans/runtime-provider-attribution.md) adds a
  second lifecycle over client hook files in `v0.2.2` and is required to follow
  them;
- define `provider use --via` as a command that may write shell startup files,
  with its interactive-only condition, its `--no-shell-setup` flag, and the
  declined preference that `shell remove` sets and `shell setup` clears;
- correct the project-attribution advisory contract, which currently promises
  that launches outside `agentdeck run` are not attributed;
- document the negative-gate marker as machine-local derived state excluded from
  portable backups, and add it to the state-root layout the specification
  already enumerates;
- raise `docs/specs/cli-design.md` from whatever version is current at delivery,
  rather than hard-coding version 21 shared with another active plan;
- update this plan and `docs/README.md` with exact verification evidence;
- create per-task review records and retire this plan only after all tasks pass.

Because `v0.2.1-rc.1` does not contain this behavior, stable `v0.2.1` must not
promote that tree. Publish `v0.2.1-rc.2` only after tasks 1-7 pass review and
L4 verification. Stable promotion then requires real Homebrew installation and
fresh-shell activation verification from the RC2 artifact.

Verification: L0 documentation checks for the task itself; release readiness is
proved later by L4 and real artifact installation, not by documentation.

Development evidence (2026-07-30, Task 8 L0):

- read the specification's delivery-time version `20`, incremented it once to
  `21`, and added the matching top changelog row;
- migrated the stable resolver, compatibility, managed lifecycle, in-use
  targeting, switch-time setup, advisory, negative-gate, cost-shape, and
  portable-backup contracts into `docs/specs/cli-design.md`, with actionable
  guidance in `docs/specs/cli-manual.md`;
- linked `docs/plans/runtime-provider-attribution.md` to the reusable living
  lifecycle contract rather than copying a second rule set;
- `rtk git diff --check` passed;
- targeted discovery checks for `shell env`, hidden `shell-init`,
  `project-attribution.enabled`, `--no-shell-setup`, version/changelog
  agreement, and active-document links passed.

No L4 release gate or released-artifact installation was run for this
documentation task. Those remain delivery evidence after Task 8 review, not
evidence supplied by the contract documents themselves.

## Release Scoping

This work belongs to `v0.2.1`, not to `v0.2.2`, for one reason that expires:
`shell-init` has never shipped in a stable release. Right now, hiding it, moving
the recommended path to `shell setup`, and demoting
`--project-environment` to an alias are all internal rearrangements. The moment
stable `v0.2.1` ships `shell-init` as the documented way to do this, every one of
those becomes a change to a stable interface, with the migration notes and
compatibility window that implies.

The second reason is the user journey itself. `v0.2.1-rc.1`'s behavior was judged
a product defect, not merely an incomplete feature. Promoting it to stable means
shipping that defect deliberately and fixing it one release later, after some
users have already formed an opinion about the feature.

Against that, delay costs little here: nothing in `v0.2.1` is a security or
data-integrity fix, the tap is self-owned with no external release commitment,
and the RC pipeline has already been exercised twice.

**Decided on 2026-07-29: all eight tasks ship in `v0.2.1`.** The scope grew twice
during design — first with eligibility reporting and route-change advisories, then
with switch-time setup — and each growth pushes the release out further. That was
accepted deliberately, on the reasoning above: the interface window closes at
stable release, and `v0.2.1` has nothing time-critical in it.

Recorded for the case where that decision is revisited under schedule pressure:
the smallest set that still delivers a correct journey is tasks 1, 2, 3, 4, and 8
— install, inspect, remove, and documentation that matches — with tasks 5, 6, and
7 following in `v0.2.2`, provided that the false sentence in
`ProjectAttributionAdvisory` is corrected in `v0.2.1` anyway (shipping a stable
release that misstates its own behavior is worse than shipping a less helpful
advisory) and that basic cross-shell verification still precedes the RC. Splitting
that way would be a schedule decision, not a design change: the command surface
and contracts stay as specified either way.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `shell-command-surface` | [x] | [x] |
| 2. `managed-shell-config` | [x] | [x] |
| 3. `activation-and-eligibility-status` | [x] | [x] |
| 4. `installation-onboarding` | [x] | [x] |
| 5. `cross-shell-acceptance` | [x] | [x] |
| 6. `route-change-advisories` | [x] | [x] |
| 7. `switch-time-setup` | [x] | [x] |
| 8. `contract-and-release` | [x] | [x] |

Order: tasks 1-3 define the command and persistence contract. Task 4 may start
after task 1 settles user-facing wording. Task 6 needs task 3's eligibility and
configuration-state reads and owns the negative-gate marker. Task 7 needs task 2's
startup-file editor and task 6's state reads. Task 5 follows tasks 2, 3, 4, 6, and
7, because its acceptance covers the advisories, the marker, and switch-time
setup. Task 8 runs last and records only reviewed behavior.

Commit boundaries follow task boundaries. This plan does not itself authorize
implementation, commits, pushes, tags, releases, Homebrew tap changes, or local
shell-profile mutation.

## Document Size

This plan is longer than the "a few hundred lines" guidance in
`docs/README.md`'s Document Conventions, and that is deliberate rather than
drift. The length comes from recorded design argument — rejected alternatives
with their reasons, one reason that was retracted when a better mechanism
appeared, and the eligibility, cost, multi-shell, and switch-time-setup
judgements — not from an accumulation of unrelated work. The task count is eight
and every task serves the one goal in the Goal section.

Do not split it. When the last task passes review, its stable contracts move into
`docs/specs/cli-design.md` and the manual, and the whole plan retires to
`docs/archive/plans/`, which is where this argument belongs afterwards.

## Starting Task

Turn a Status row into a scoped development instruction by naming its anchor:

> 进入开发：`shell-integration` / `<task-anchor>`

Read, in order: `AGENTS.md`; this plan's Problem, Alternatives, Command
Surface, and named task; the current project-attribution contract in
`docs/specs/cli-design.md`; every file the task names; and the verification
routing in `AGENTS.md`. Implement only that task. Tick `Dev` after its required
verification passes. An independent reviewer records a round in
`docs/reviews/shell-integration/<task-anchor>.md` and ticks `Review` only on
`Verdict: PASS`.
