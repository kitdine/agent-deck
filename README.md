# AgentDeck

[English](README.md) | [简体中文](README_zh.md)

> A local, macOS-first CLI for managing Codex and Claude providers, usage,
> sessions, extensions, diagnostics, and encrypted backups from one place.

AgentDeck is for developers who use multiple Codex or Claude providers and want
one local control plane without moving credentials or session data to a hosted
service. Custom provider definitions and authenticated credential ciphertext
live in SQLite, protected by a private machine-bound key file; client session
logs remain read-only source data. The built-in Codex `official` provider uses
Codex's existing OpenAI or ChatGPT login.

> **Stable release:** [`v0.4.0`](https://github.com/kitdine/agent-deck/releases/tag/v0.4.0)
> is published for Darwin arm64 and amd64 at commit
> `6b7663b51f22903445798dd7db637cbcaab1a422`. Its same-SHA preflight,
> release artifacts, stable Homebrew formula, formula test, and bash, zsh, and
> fish completions passed verification. See the
> [documentation index](docs/README.md) for current development status.

```bash
make build
./dist/agentdeck --help
```

## Why AgentDeck

- **One CLI:** manage provider selection, usage, sessions, extensions,
  diagnostics, and backups through a single command tree.
- **Local by default:** normal commands do not require a hosted AgentDeck
  service or expose a network port.
- **Credential isolation:** AgentDeck stores custom provider secrets only as
  AES-256-GCM ciphertext in SQLite; a private random seed and stable machine
  identity derive the local encryption key.
- **Recoverable changes:** provider configuration writes use journals and
  redacted backups so interrupted operations can be diagnosed and recovered.
- **Scriptable output:** commands support text and JSON output; `watch` also
  supports versioned NDJSON events.

## Current Capabilities

| Command | What it does |
| --- | --- |
| `agentdeck provider` | Manage providers, completed-operation selection snapshots, status, and recovery. |
| `agentdeck credential` | Manage multiple named encrypted credentials and their provider/client bindings without exposing values. |
| `agentdeck usage` | Incrementally scan local usage records, summarize cost and token data, and diagnose attribution. |
| `agentdeck price` | Inspect, update, and override local price catalogs. |
| `agentdeck session` | Scan and search approved visible session text, manage exclusions, rebuild the index, or purge it independently. |
| `agentdeck extension` | Discover plugins, MCP servers, and skills; inspect health; explicitly adopt or release management state. |
| `agentdeck watch` | Run foreground incremental usage, session, and extension scans with versioned NDJSON events. |
| `agentdeck backup` | Create, list, authenticate, inspect, and restore age-encrypted portable backups. |
| `agentdeck doctor` | Run quick or full read-only diagnostics with actionable recovery guidance. |
| `agentdeck run` | Associate an exact Codex or Claude subprocess execution with its usage record. |
| `agentdeck version` | Report version, commit, build time, and Go runtime identity for diagnostics. |

Native extension enable and disable remain read-only unless an adapter exposes
an unambiguous native toggle. Portable restore only targets an absent or empty
AgentDeck state root.

## Requirements

- macOS for the current stable machine-identity adapter
- Go `1.26.0`
- GNU Make

Dependencies are committed under `vendor/`, so normal builds and tests do not
download Go modules.

## Build From Source

Build the local development binary:

```bash
make build
./dist/agentdeck --help
```

Build both supported macOS architectures:

```bash
make build-all
```

The binaries are written to:

```text
dist/agentdeck_darwin_arm64
dist/agentdeck_darwin_amd64
```

## Install With Homebrew

Install the latest versioned macOS release from the AgentDeck tap:

```bash
brew install kitdine/tap/agentdeck
agentdeck version
```

The stable formula installs bash, zsh, and fish completion scripts in
Homebrew's standard completion directories without editing shell rc files.
Command completion is separate from the optional project-attribution wrappers
described below.

After a release-candidate tap PR is merged, opt into that channel with:

```bash
brew uninstall kitdine/tap/agentdeck
brew install kitdine/tap/agentdeck-rc
agentdeck version
```

Stable and RC formulae install the same executable and completion paths, so
they must not be installed together. The explicit uninstall/install sequence
above enforces that switch without making Homebrew load or trust the other
channel's formula. AgentDeck state under `~/.agentdeck/` is not managed or
removed by Homebrew. Later candidates upgrade normally with `brew update &&
brew upgrade kitdine/tap/agentdeck-rc`; switch back by uninstalling
`agentdeck-rc` and reinstalling `agentdeck`.

The source-based workflow below additionally supports a managed rc-file block
for command completion. It does not install project-attribution wrappers.

## Install From Source

Install the current source build for the local user:

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
agentdeck version
```

Installation detects the fish, zsh, or bash process that invoked Make, generates
that shell's command completion, and adds one marked completion-only AgentDeck
source block to its user rc file. Override detection or the rc path when needed:

```bash
make install COMPLETION_SHELL=fish
make install COMPLETION_SHELL=zsh COMPLETION_RC="$HOME/.config/zsh/.zshrc"
make install COMPLETION_SHELL=none
```

`COMPLETION_SHELL=none` explicitly installs only the binary. Detection failure,
an unsafe rc path, or a conflicting managed block stops installation without
leaving a partial binary or rc change.

## Optional Project Attribution

Installing AgentDeck never installs project-attribution wrappers. The
prerequisite for attribution is a `codex` or `claude` provider route selected
with `provider use --via` whose wrapper was explicitly declared as Headroom
with `provider set-wrapper --kind headroom`. Without that route, shell
integration provides no attribution benefit.

With no eligibility marker, the wrappers invoke the real client without
starting AgentDeck. When the marker exists, each invocation starts one
AgentDeck process and performs one read-only database access. On the measured
Intel macOS 26.6 host, marker-present paths added roughly 0.1-0.2 seconds per
invocation; this is an environment-specific order of magnitude, not a
performance guarantee.
To use this integration, configure every shell you use with:

```bash
agentdeck shell setup
```

After setup, optionally verify the three independent states with:

```bash
agentdeck shell status
```

The status output distinguishes persistent configuration, activation in the
current shell session, and per-client route eligibility.

Setup is optional. Skipping it leaves shell-based project attribution
disabled and does not affect normal AgentDeck use. Command completion is
handled separately and never requires `shell setup`. To undo the integration
later:

```bash
agentdeck shell remove
```

`agentdeck shell-init <bash|fish|zsh>` only prints a shell program to stdout.
Running it by itself neither activates integration nor installs anything.
Sourcing that output can activate wrappers for the current shell only; use the
explicit `shell setup` command for persistent configuration.

The default installation paths are separate from AgentDeck user state:

```text
~/.local/bin/agentdeck                  # executable
~/.local/share/agentdeck/               # ownership manifest
~/.agentdeck/                           # databases, indexes, and backups
```

Use a different user-local prefix when needed:

```bash
make install PREFIX="$HOME/tools/agentdeck"
```

Keep using the same `PREFIX` for that installation's upgrade and uninstall:

```bash
make install PREFIX="$HOME/tools/agentdeck" FORCE=1
make uninstall PREFIX="$HOME/tools/agentdeck"
```

Installation refuses to overwrite an existing binary or manifest. After
reviewing the binary identity, explicitly authorize an upgrade with:

```bash
make install FORCE=1
agentdeck version
```

For a forced upgrade, the manifest hash first proves that the installed file is
unchanged, then the installer verifies that both executables implement the
AgentDeck version contract. It prints the old and new build identities and
SHA-256 values before replacing anything. The manifest is ownership and tamper
evidence, not release authenticity; source installs are not signed releases.

Safely remove an unchanged installation with:

```bash
make uninstall
```

Uninstall verifies the binary, generated completion, and exact managed rc block
before removing anything. Changes outside the block are preserved. It fails
closed if an owned artifact or block changed and never deletes the rc file,
`~/.agentdeck/`, encrypted credentials, client configuration, or backups.
Empty installation directories may remain because they are not manifest-owned
artifacts and uninstall does not claim ownership of them.

## Quick Start

Start with read-only diagnostics and command discovery:

```bash
./dist/agentdeck doctor
./dist/agentdeck provider --help
./dist/agentdeck usage --help
```

Scan and inspect local usage and sessions:

```bash
./dist/agentdeck usage scan
./dist/agentdeck usage summary
./dist/agentdeck session scan
./dist/agentdeck session search "project name"
```

Discover local extensions:

```bash
./dist/agentdeck extension scan
./dist/agentdeck extension list
```

Use JSON for automation or NDJSON for the foreground watcher:

```bash
./dist/agentdeck --format json provider list
./dist/agentdeck --format ndjson watch
```

Run `agentdeck <command> --help` before a state-changing operation to inspect
its exact arguments and safety constraints.

## Build Identity

Include the build identity when reporting a problem:

```bash
agentdeck version
agentdeck --version
agentdeck --format json version
```

Text output is split into support-friendly labeled lines:

```text
Release Version: v0.0.0-dev
Git Commit Hash: <full commit SHA>
Git Branch: main
Go Version: go1.26.5
UTC Build Time: 2026-07-15 08:33:55
```

Make derives build identity from the nearest Git tag (or `v0.0.0`) plus `-dev`
unless HEAD is an exact clean tag, the full commit SHA, the current branch, and
the actual UTC build time. `VERSION`, `COMMIT`, `BRANCH`, and `BUILD_TIME`
remain explicitly overridable. Direct `go build` outside Make retains the safe
`dev`/`unknown` defaults.

## Shell Completion

The completion command is an output-only generator. `make install` handles
persistent activation; use the generator directly for a temporary shell or a
custom completion manager:

```bash
agentdeck completion fish | source
agentdeck completion zsh > /tmp/_agentdeck
agentdeck completion bash > /tmp/agentdeck.bash
```

If persistent completion is missing, inspect the managed block between
`# >>> agentdeck completion >>>` and `# <<< agentdeck completion <<<` in the
selected rc file. Do not hand-edit that block: install and uninstall deliberately
fail closed when it no longer matches the ownership manifest.

## Provider Setup Example

`official` is built in for both Codex and Claude and appears in `provider list`
and `provider show` without a database record or AgentDeck credential. It
reuses each client's existing vendor login state:

```bash
./dist/agentdeck provider show official
./dist/agentdeck provider use official
./dist/agentdeck provider use official --client claude
```

AgentDeck sets `[model_providers.custom].name = "official"` and removes the
custom base URL and bearer token in `~/.codex/config.toml`; it never reads,
modifies, or deletes `~/.codex/auth.json`. For Claude, a direct `official`
switch removes only AgentDeck-owned endpoint and token fields from the managed
settings file, preserves all other fields, and never reads or writes Claude's
stored login credentials. It reports a restart advisory and any detected
unmanaged credential source that would override the selection without deleting
that source.

The following example uses a fake endpoint. `provider add` prompts once without
terminal echo, generates the complete `work-default-ref`, stores authenticated
ciphertext in SQLite, and binds the same credential to both clients:

```bash
./dist/agentdeck provider add work --endpoint https://api.example.com --clients codex,claude
./dist/agentdeck provider show work
```

`--credential` is the only credential shorthand flag; it is not a reference.
AgentDeck always generates `<provider>-<credential>-ref`, including the
`default` component, and never adds a client component. Running `provider add`
again for an existing provider adds a missing named credential:

```bash
./dist/agentdeck provider add work --credential codex --endpoint https://api.example.com/v1 --clients codex --multiplier 0.4
```

Endpoint, multiplier, and client bindings belong to each credential. A final
`/v1` is accepted for a Codex-bound credential and removed from the stored base;
Codex configuration adds exactly one `/v1`. A Claude-only endpoint ending in
`/v1` is preserved. Endpoints with userinfo, a query string, or a fragment are
rejected. An identical existing credential is a no-prompt successful no-op;
changed metadata must use `credential update`.

Top-level `credential add` is the explicit existing-provider entry point, and
`credential update --rotate` rotates one through the same credential service.
Validation and encryption complete before one SQLite transaction commits
credential metadata and ciphertext together. Automation may supply exactly one
credential line through stdin; credentials are never accepted as CLI arguments
or environment variables.

Selecting a provider requires only its name and client. AgentDeck resolves
`~/.codex/config.toml` or `~/.claude/settings.json` automatically and creates a
unique redacted backup under the active AgentDeck state directory:

```bash
./dist/agentdeck provider use work --client codex
./dist/agentdeck credential add work --credential personal --endpoint https://api.example.com --clients codex
./dist/agentdeck provider use work --client codex --credential personal
```

Only a completed provider-use operation becomes active attribution. Deleting a
used custom provider removes its live definition, credential metadata, and
ciphertext while retaining immutable historical usage snapshots.

For a non-standard client installation, override only the configuration path:

```bash
./dist/agentdeck provider use work --client codex --config-path /custom/codex/config.toml
```

Managed backups use
`<state-dir>/client-backups/<client>/<operation-id>.redacted.toml|json`, mode
`0600`, and are recorded in the operation journal. Users do not choose or reuse
backup paths.

AgentDeck modifies only documented provider fields and preserves unrelated
client configuration.

## Local Data and Privacy

By default, AgentDeck uses `~/.agentdeck/` as its persistent state directory:

```text
~/.agentdeck/
├── agentdeck.sqlite3   # provider, usage, extension, and backup metadata
├── credential.key      # private machine-bound credential seed, mode 0600
├── sessions.sqlite3    # separately purgeable visible session index
└── client-backups/     # managed redacted provider-switch backups
```

Use `--state-dir <path>` for isolated state. AgentDeck keeps the caller's
current directory as project context for extension discovery; it does not use
the installation directory or change into `~/.agentdeck/` while running.

- Within AgentDeck state, custom provider credential values exist persistently
  only as authenticated ciphertext in SQLite; `credential.key` is never included
  in portable backups. Codex `official` needs no AgentDeck credential.
- AgentDeck never manages Codex `auth.json`.
- Codex and Claude session logs are read-only inputs.
- The session index stores only approved visible conversation fields.
- Purging the session index does not delete client source logs; it clears only
  the session watch checkpoint so the next session watch bootstraps the index.
- Usage/watch inventory processes only added, appended, mutated, or removed
  sources. `watch --domains usage` does not open the session store.
- Default text uses explicit human renderers; `--quiet` suppresses only
  successful non-essential text output and never suppresses JSON or errors.
- Normal commands do not probe provider hosts or access the network.
- Network access is reserved for the explicit `agentdeck price update`
  command.
- Automated tests use temporary homes, synthetic machine identities, synthetic
  logs, and isolated encrypted credential stores.

## Development and Verification

Run targeted checks while developing, then use the complete release gate before
delivery:

```bash
make test
make test-race
make vet
make release-verify
```

`make release-verify` runs the Go test suite, race detector, `go vet`, both
macOS builds, the stripped arm64 size gate, and the repository privacy scan.

Clean generated binaries with:

```bash
make clean
```

## Project Structure

```text
cmd/agentdeck/   Cobra CLI entrypoint and end-to-end contract tests
internal/        Provider, usage, session, extension, backup, and platform code
scripts/         Release privacy checks
docs/specs/      Approved behavior and architecture contracts
docs/plans/      Execution status and completion gates
vendor/          Committed Go dependencies
```

## Documentation and Status

- [Documentation index](docs/README.md)
- [Execution status](docs/README.md)
- [CLI design and contracts](docs/specs/cli-design.md)

The repository code, tests, configuration, Git history, and active documents
above are the sources of truth.

## Release Distribution

Versioned GitHub Releases provide checksum-protected macOS archives for arm64
and amd64. The stable `kitdine/tap/agentdeck` formula and opt-in
`kitdine/tap/agentdeck-rc` formula install those immutable artifacts rather than
building from a moving branch. Each supported stable or strict `vX.Y.Z-rc.N`
release validates its rendered formula plus bash, zsh, and fish completions,
then opens a formula-specific update pull request in the tap repository. An RC
pull request never changes the stable formula.

## Contributing

Keep changes scoped to the active design and plan, preserve privacy boundaries,
and run `make release-verify` before proposing delivery. See [AGENTS.md](AGENTS.md)
for repository-specific development and authorization rules.

## License

This repository does not currently include a license file.
