# AgentDeck

[English](README.md) | [简体中文](README_zh.md)

> A local, macOS-first CLI for managing Codex and Claude providers, usage,
> sessions, extensions, diagnostics, and encrypted backups from one place.

AgentDeck is for developers who use multiple Codex or Claude providers and want
one local control plane without moving credentials or session data to a hosted
service. Provider definitions live in SQLite, credentials stay in macOS
Keychain, and client session logs remain read-only source data.

> **Pre-release:** Phase One and the version/installation baseline have completed
> implementation and independent review. Interactive credential input and
> managed shell completion installation have passed full release verification;
> initial review findings are addressed and independent re-review has passed.
> AgentDeck is not yet available through Homebrew.

```bash
make build
./dist/agentdeck --help
```

## Why AgentDeck

- **One CLI:** manage provider selection, usage, sessions, extensions,
  diagnostics, and backups through a single command tree.
- **Local by default:** normal commands do not require a hosted AgentDeck
  service or expose a network port.
- **Credential isolation:** provider secrets are stored in macOS Keychain, not
  in AgentDeck databases or client configuration backups.
- **Recoverable changes:** provider configuration writes use journals and
  redacted backups so interrupted operations can be diagnosed and recovered.
- **Scriptable output:** commands support text and JSON output; `watch` also
  supports versioned NDJSON events.

## Current Capabilities

| Command | What it does |
| --- | --- |
| `agentdeck provider` | Manage providers, Keychain credential references, provider selection, status, and recovery. |
| `agentdeck usage` | Scan local usage records, summarize cost and token data, diagnose attribution, and manage price catalogs. |
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

- macOS for the current Keychain-backed credential implementation
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

## Install From Source

Install the current source build for the local user:

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
agentdeck version
```

Installation detects the fish, zsh, or bash process that invoked Make, generates
that shell's completion, and adds one marked AgentDeck source block to its user
rc file. Override detection or the rc path when needed:

```bash
make install COMPLETION_SHELL=fish
make install COMPLETION_SHELL=zsh COMPLETION_RC="$HOME/.config/zsh/.zshrc"
make install COMPLETION_SHELL=none
```

`COMPLETION_SHELL=none` explicitly installs only the binary. Detection failure,
an unsafe rc path, or a conflicting managed block stops installation without
leaving a partial binary or rc change.

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

Safely remove an unchanged installation with:

```bash
make uninstall
```

Uninstall verifies the binary, generated completion, and exact managed rc block
before removing anything. Changes outside the block are preserved. It fails
closed if an owned artifact or block changed and never deletes the rc file,
`~/.agentdeck/`, Keychain credentials, client configuration, or backups.
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

Source builds report `dev`, `unknown`, and `unknown` unless `VERSION`, `COMMIT`,
and `BUILD_TIME` are explicitly supplied to Make. Release tooling can inject
those values without changing the runtime contract.

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

The following example uses a fake endpoint and credential reference. `provider
add` prompts once without terminal echo and stores the value in macOS Keychain
while creating the provider:

```bash
./dist/agentdeck provider add work https://api.example.com/v1 work-codex 1 codex
./dist/agentdeck provider show work
```

`provider credential add` and `provider credential update` remain available for
independent pre-provisioning and rotation. Automation may supply exactly one
credential line through stdin; credentials are never accepted as CLI arguments
or environment variables.

Selecting a provider requires the client configuration path and a destination
for the redacted backup:

```bash
./dist/agentdeck provider use \
  work codex \
  "$HOME/.codex/config.toml" \
  "$HOME/.agentdeck/codex-config.redacted.toml"
```

AgentDeck modifies only documented provider fields and preserves unrelated
client configuration.

## Local Data and Privacy

By default, AgentDeck uses `~/.agentdeck/` as its persistent state directory:

```text
~/.agentdeck/
├── agentdeck.sqlite3   # provider, usage, extension, and backup metadata
└── sessions.sqlite3    # separately purgeable visible session index
```

Use `--state-dir <path>` for isolated state. AgentDeck keeps the caller's
current directory as project context for extension discovery; it does not use
the installation directory or change into `~/.agentdeck/` while running.

- Provider credential values stay in macOS Keychain.
- Codex and Claude session logs are read-only inputs.
- The session index stores only approved visible conversation fields.
- Purging the session index does not delete client source logs.
- Normal commands do not probe provider hosts or access the network.
- Network access is reserved for the explicit `agentdeck usage price update`
  command.
- Automated tests use temporary homes, synthetic logs, and fake secret stores.

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
- [Phase One implementation plan](docs/plans/2026-07-13-agentdeck-cli.md)
- [CLI design and contracts](docs/specs/2026-07-13-agentdeck-cli-design.md)

The repository code, tests, configuration, Git history, and active documents
above are the sources of truth.

## Distribution Roadmap

Homebrew integration begins only after the first versioned release provides
signed or checksummed macOS archives for arm64 and amd64. The planned workflow
is:

```bash
brew tap kitdine/tap
brew install agentdeck
```

These commands are not available yet. The future formula will install immutable
release archives rather than build from a moving branch.

## Contributing

Keep changes scoped to the active design and plan, preserve privacy boundaries,
and run `make release-verify` before proposing delivery. See [AGENTS.md](AGENTS.md)
for repository-specific development and authorization rules.

## License

This repository does not currently include a license file.
