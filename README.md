# AgentDeck

> Pre-release: AgentDeck is under active development and is not yet published
> through Homebrew.

AgentDeck is a local macOS-first CLI for managing Codex and Claude provider
configuration, inspecting locally recorded usage, and searching selected visible
session text. It keeps provider definitions in local SQLite state, stores
credentials in macOS Keychain, and reads client session logs without modifying
them.

## Current capabilities

- `agentdeck provider`: manage providers, credentials, selection, status, and
  recovery.
- `agentdeck usage`: scan local usage records and inspect usage summaries and
  price catalogs.
- `agentdeck session`: scan, search, exclude, rebuild, and purge the local
  session index.
- `agentdeck run`: associate a Codex or Claude process with an exact usage run.

Native extension inventory, watch mode, encrypted backup, and doctor commands
are planned but are not available in this pre-release build.

## Build from source

Requirements: Go `1.26.0` and GNU Make. Dependencies are committed under
`vendor/`, so normal builds do not download modules.

```bash
make build
./dist/agentdeck --help
```

Build both supported macOS architectures:

```bash
make build-all
```

This creates:

```text
dist/agentdeck_darwin_arm64
dist/agentdeck_darwin_amd64
```

Run the full local verification suite, including the legacy behavior-reference
tests:

```bash
make verify
```

## Planned Homebrew distribution

Homebrew integration will begin only after the first versioned release provides
signed or checksummed macOS archives for both architectures. The planned user
workflow is:

```bash
brew tap kitdine/tap
brew install agentdeck
```

The future `kitdine/homebrew-tap` formula will install release archives rather
than build from a moving Git branch. Each release must provide a semantic
version, arm64 and amd64 archive URLs, SHA-256 values, and a smoke test based on
`agentdeck --help`.

## Documentation and status

- [Documentation index](docs/README.md)
- [Phase-one implementation plan](docs/plans/2026-07-13-agentdeck-cli.md)
- [CLI design](docs/specs/2026-07-13-agentdeck-cli-design.md)

## Privacy and safety

- Provider credentials are kept in macOS Keychain, not in AgentDeck databases.
- Client logs are read-only source data.
- The session index stores only approved visible conversation fields and can be
  purged without deleting source logs.
- Network access is reserved for the explicit usage-price update command.
