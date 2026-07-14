# AgentDeck CLI Phase-One Implementation Plan

**Status:** active, phase-one and version/installation baseline implementation
and independent review complete; release preparation pending

**Specification:**
`docs/specs/2026-07-13-agentdeck-cli-design.md`

**Goal:** Replace the repository's separate Python and Bash entrypoints with
one small Go CLI while preserving the approved provider and usage contracts and
adding local session search, native extension inventory, secure backup, and a
stable JSON boundary for the future macOS application.

## Scope Boundaries

- Implement one `agentdeck` executable and focused internal Go packages.
- Target macOS first while keeping core domain interfaces cross-platform.
- Do not execute install, uninstall, or overwrite operations against the real
  user home during development or automated verification. Install targets are
  validated only with an isolated temporary `PREFIX`.
- Do not add a daemon, LaunchAgent, GUI, provider usage API, custom pricing, or
  extension installation/update/removal.
- Do not add legacy command aliases or automatically import legacy state.
- Remove repository legacy entrypoints only after equivalent Go behavior,
  tests, and independent review are complete.
- Keep implementation, review, fix, re-review, and delivery as separate stages.
- Do not commit or push without explicit authorization.

## Phase 1: Prove the Go and SQLite Foundation

- [x] Create the Go module, `cmd/agentdeck`, and the internal package layout.
- [x] Establish `github.com/kitdine/agent-deck` as the canonical module path,
      use Cobra for the CLI command tree, and commit Go dependencies under vendor.
- [x] Add a driver spike proving FTS5 queries, SQLite online backup, WAL,
      transactional migrations, lock handling, and owner-only sidecar permissions.
- [x] Build stripped macOS arm64 and amd64 binaries and record their sizes.
- [x] Reject the candidate driver before business implementation if a mandatory
      SQLite capability is absent.
- [x] Define state-root resolution, test overrides, clock and filesystem
      interfaces, stable errors, and output envelopes.
- [x] Add schema version checks, explicit migrations, unknown-new-version
      refusal, and interrupted-transaction tests.

### Phase 1 Verification Record

- Candidate: `modernc.org/sqlite v1.53.0`, a pure-Go driver.
- `go test ./...`, `go test -race ./...`, and `go vet ./...` passed with
  isolated Go caches on 2026-07-13.
- The driver spike passed FTS5 search, online backup, WAL mode, transactional
  migration rollback, schema-version refusal, state-lock, and owner-only mode
  tests. Review follow-up also covers atomic bootstrap, singleton schema
  metadata, migration-lock integration, lock ownership, and pre-driver
  private SQLite file creation. A lock-release failure now closes the opened
  store and is returned to the caller.
- Stripped `darwin/arm64` binary: 1,641,330 bytes (below 25 MiB target).
- Stripped `darwin/amd64` binary: 1,698,176 bytes.

## Phase 2: Implement Provider and Credential Management

- [x] Define provider, client mapping, credential reference, selection,
      multiplier snapshot, operation journal, and settings tables.
- [x] Implement macOS Keychain storage behind a platform secret-store
      interface, without accepting secrets in process arguments.
- [x] Implement provider list, show, add, edit, remove, status, and credential
      commands.
- [x] Port structured Codex TOML and Claude JSON mutation behavior from the
      existing contract while preserving unrelated fields.
- [x] Implement redacted backups, temporary writes, atomic replacement,
      operation-journal recovery, and configuration fingerprint checks.
- [x] Record successful no-op selections and exact decimal multipliers.
- [x] Cover missing credentials, invalid multipliers, permission failures,
      concurrent mutation, interruption at each journal state, and secret leakage.

## Phase 3: Port Usage, Pricing, and Run Attribution

**Implementation status (2026-07-13):** implemented; contract hardening and
independent review completed in Phase 7. Usage records use completed
source-byte snapshots rather than timestamps for exact bindings; external
client observation downgrades ambiguous runs to estimated attribution. Catalog
overrides retain official provenance and effective times, while JSON reports
make incomplete automatic scans explicit.

- [x] Port table-driven money and attribution contracts before wiring file
      discovery or reporting.
- [x] Implement Codex and Claude parsers using the existing synthetic fixtures,
      including Claude five-minute and one-hour cache-write components.
- [x] Implement incremental source tracking for append, partial lines, equal
      prefix rewrite, growing rewrite, truncate, replacement, and archive move.
- [x] Upsert stable event snapshots without double counting.
- [x] Implement exact run start/range binding/end as short transactions and
      preserve bindings through rebuild.
- [x] Detect cross-process overlap, downgrade ambiguous data to estimated, and
      recover stale wrapper runs.
- [x] Implement session-start estimated fallback and historical multiplier `1`.
- [x] Import bundled catalog data into immutable price versions.
- [x] Implement explicit LiteLLM update with direct-provider filtering, Git
      commit pinning, content hashing, validation, official component overrides,
      and atomic version creation.
- [x] Select prices by event time and effective time; retain unknown and
      incomplete components as unpriced.
- [x] Implement scan, summary, sessions, diagnose, rebuild, price status,
      price update, and price history.
- [x] Add full text/JSON contracts for tokens, counts, multiplier, catalog base
      cost, provider cost, attribution warnings, partial scans, and unpriced data.

## Phase 4: Add Local Session Search

**Implementation status (2026-07-14):** implemented; independent review
completed in Phase 7. `session_documents` and `session_metadata`
now retain source ownership. `session_sources` persists stable identity, cursor,
raw partial-line bytes, size, modification time, and prefix hash. `list`,
`show`, and `search` materialize one session view with active sources preferred
over archives and archives as fallback. Only a verified append for the same
source identity resumes from its cursor; truncation, equal-length rewrites,
identity changes, and moves rebuild only the affected source. Session commands
serialize on the state lock and open only the separately purgeable
`sessions.sqlite3` database; they never create or migrate the core
provider/usage database.

- [x] Define the separate session database, source-level schema, parser version,
      exclusions, cursor, document, and FTS5 tables.
- [x] Define explicit extraction allowlists for Codex and Claude visible prompts
      and final assistant replies.
- [x] Add counterexample fixtures for system/developer messages, hidden
      reasoning, tool arguments/results, credentials, attachments, images,
      binaries, and environment fields.
- [x] Implement source-level incremental scan, stable document replacement,
      project/path normalization, and source mutation handling.
- [x] Implement list, search, show, exclude, rebuild, and purge-index.
- [x] Verify source logs remain read-only and normal output never exposes
      excluded or prohibited content.

## Phase 5: Add Native Extension Adapters

- **Implementation status (2026-07-14):** re-review passed. The completed
  remediation separates native Codex and Claude plugin, MCP, and skill sources,
  compares live fingerprints
  during read-only diagnosis, preserves prior inventory when discovery fails,
  and completes stable JSON success and error contracts. Current sources expose
  no unambiguous native enable/disable toggle, so those commands refuse mutation
  as `extension_read_only` rather than changing client configuration.

- [x] Define canonical extension IDs, kinds, scopes, capabilities, diagnostics,
      fingerprints, and management state.
- [x] Implement Codex discovery adapters for plugins, MCP servers, and skills.
- [x] Implement Claude discovery adapters for plugins, MCP servers, and skills.
- [x] Preserve native source formats and store inventory metadata rather than
      extension content.
- [x] Implement scan, list, show, and doctor before mutation commands.
- [x] Implement adopt and release; retain enable/disable commands only for a
      future unambiguous adapter toggle and refuse them otherwise.
- [x] Mark unsupported native toggles read-only and test that no mutation is
      attempted.
- [x] Verify diagnostics redact environment values, credentials, and private
      extension configuration.

## Phase 6: Add Watch, Backup, and Doctor

- **Implementation status (2026-07-14):** implementation, review remediation,
  and re-review are complete. Watch persists source
  fingerprints and leaves unchanged restarts read-only, restore preserves or
  compensates state-root permissions, and doctor validates complete catalog
  provenance and distinct unpriced models. Backup passphrases use hidden terminal
  input or one-line stdin input for automation and are never accepted through
  arguments or environment variables.

- [x] Implement the foreground watcher with incremental polling and versioned
      NDJSON events.
- [x] Ensure unchanged watch iterations do not write the database and busy
      scans do not block other commands.
- [x] Implement age passphrase encryption using a maintained library and a
      versioned `.adb` manifest.
- [x] Snapshot SQLite through the online backup API and stream credentials into
      the encrypted archive without plaintext temporary files.
- [x] Exclude sessions by default and include them only with an explicit flag.
- [x] Implement list, authenticated inspect, and restore to an empty state root.
- [x] Roll back only credentials and files created by a failed restore.
- [x] Implement quick and full read-only doctor checks across state, provider,
      usage, prices, sessions, extensions, locks, and pending operations.
- [x] Add explicit recovery command guidance without a generic auto-fix mode.

## Phase 7: Contract and Release Hardening

**Implementation status (2026-07-14):** re-review passed. The completed
remediation adds snake_case
override and recovery DTOs, empty recovery arrays, real offline price update and
override goldens, both run clients, per-command success and typed-error schema
goldens driven by actual Cobra executions, actual watch event goldens, exact-run
restore assertions, a single HTTP adapter, and executable privacy regressions
for tracked and untracked non-ignored files plus generated artifacts. Privacy
enumeration and scanning now fail closed without echoing matched content, and
the command tests are separated by contract, release-gate, and end-to-end
responsibility. Final review remediation also propagates failed client exits,
closes unprovable wrapper runs as estimated, and refuses to overwrite an
existing portable backup. The Go replacement has passed independent review,
and the superseded repository-local legacy entrypoints and fixtures have been
removed without touching real user scripts.

- [x] Add stable JSON schema fixtures for every command consumed by the future
      GUI and NDJSON fixtures for watch events.
- [x] Run unit, integration, race, permission, interruption, privacy, and
      end-to-end tests using temporary homes and synthetic client binaries.
- [x] Build macOS arm64 and amd64 and verify the stripped arm64 size target.
- [x] Verify no command except explicit price update accesses the network.
- [x] Scan tracked files, fixtures, databases, backups, and captured output for
      credentials and prohibited session content.
- [x] Run an isolated end-to-end flow covering provider selection, exact run,
      usage scan, session search, extension inventory, backup, and empty-root
      restore.
- [x] Independently review the Go replacement before removing any legacy
      repository entrypoint.
- [x] Remove legacy repository entrypoints and obsolete fixtures only after
      their replacement contracts are proven; leave real `~/.local/bin/` scripts
      untouched.
- [x] Synchronize the specification, documentation index, `AGENTS.md`, and this
      plan with the reviewed implementation state.

## Phase 8: Version and Installation Baseline

**Implementation status (2026-07-15):** implemented, full release verification
passed, and independent re-review passed; release preparation is pending. This
phase adds binary identity and a safe source-install workflow without publishing
Homebrew formulae or release artifacts. The executable installation,
installation ownership metadata, AgentDeck state root, caller working directory,
and Codex/Claude state remain separate ownership boundaries.

- [x] Remove the local empty `bin/`, `config/`, and `tests/` directories left by
      legacy cleanup. Git does not track these directories, so this is workspace
      hygiene rather than a repository content change.
- [x] Add `internal/buildinfo` with stable development defaults for version,
      commit, and build time plus the runtime Go version.
- [x] Add `agentdeck version` and root-only `agentdeck --version` with matching
      text output and the existing JSON envelope contract.
- [x] Inject `VERSION`, `COMMIT`, and `BUILD_TIME` through Make `-ldflags`; do
      not inject a changing local timestamp unless the caller supplies it.
- [x] Pass the shared binary version into backup creation so
      `manifest.agentdeck_version` is not independently hard-coded.
- [x] Add user-local `make install` and `make uninstall` targets. Default to
      `PREFIX=$(HOME)/.local`, refuse implicit overwrite, require `FORCE=1` for
      replacement, and record the installed path and SHA-256 under
      `$(PREFIX)/share/agentdeck/`.
- [x] Make uninstall fail closed unless the manifest path and installed binary
      hash match. Never remove `~/.agentdeck/`, Keychain credentials, backups,
      client configuration, project files, or unrelated executables.
- [x] Add tests for development defaults, injected identity, text/JSON parity,
      backup provenance, isolated install, overwrite refusal, forced upgrade,
      tampered-binary refusal, and successful ownership-checked uninstall.
- [x] Update `README.md` and `README_zh.md` in place with the state-directory
      distinction, current source installation, version diagnostics, upgrade,
      uninstall, and the explicit absence of Homebrew availability.
- [x] Run targeted version, backup, and installation tests followed by the full
      `make release-verify` gate. All install tests use a temporary `PREFIX`.
- [x] Complete independent review, address the version flag/format and test
      coverage findings, and pass independent re-review.

## Required Verification

Once Go source exists, the release gate includes:

```bash
rtk test go test ./...
rtk test go test -race ./...
rtk lint go vet ./...
rtk test env GOOS=darwin GOARCH=arm64 go build -trimpath ./cmd/agentdeck
rtk test env GOOS=darwin GOARCH=amd64 go build -trimpath ./cmd/agentdeck
rtk test make release-verify
```

Targeted package and integration tests run before the full gate. The final
commands must be run after the last edit. Any unavailable cross-build, Keychain,
or real-runtime validation is reported as residual risk rather than inferred
from source inspection.

## Completion Gate

Phase one and Phase 8 implementation and independent review are complete. All
phase-one specification acceptance criteria have fresh evidence, the superseded
repository-local entrypoints have been removed, and the version/installation
baseline has passed full release verification and independent re-review. Release
preparation remains a separate stage; review approval does not authorize
installation into the real user home, push, release, or modification of real
user state.
