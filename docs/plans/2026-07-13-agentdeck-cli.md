# AgentDeck CLI Phase-One Implementation Plan

**Status:** active, phase-one and version/installation baseline implementation
and independent review complete; interactive CLI and shell completion usability
implementation, release verification, and independent review complete; unified
ASCII collection tables and machine-bound encrypted SQLite credential storage
implemented and release-verified, awaiting independent review

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
- [x] Add an opt-in `session show --activity` source reader that exposes only
      allowlisted tool name/time/status/duration metadata and stores none of it
      in the purgeable session index.
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

**Implementation status (2026-07-15):** baseline implementation and independent
re-review passed. Post-review build identity and forced-upgrade diagnostics have
been hardened and await independent review. Release preparation is pending.
This phase adds binary identity and a safe source-install workflow without
publishing Homebrew formulae or release artifacts. The executable installation,
installation ownership metadata, AgentDeck state root, caller working directory,
and Codex/Claude state remain separate ownership boundaries.

- [x] Remove the local empty `bin/`, `config/`, and `tests/` directories left by
      legacy cleanup. Git does not track these directories, so this is workspace
      hygiene rather than a repository content change.
- [x] Add `internal/buildinfo` with stable development defaults for version,
      commit, branch, and build time plus the runtime Go version.
- [x] Add `agentdeck version` and root-only `agentdeck --version` with matching
      text output and the existing JSON envelope contract.
- [x] Inject tag-derived `VERSION`, full `COMMIT`, current `BRANCH`, and actual
      UTC `BUILD_TIME` through Make `-ldflags`, while preserving explicit
      caller overrides and direct-Go-build development defaults.
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
- [x] Harden post-review forced upgrades to validate both binary version
      contracts and print old/new identities and SHA-256 values before
      replacement; reject unrelated executables even when paired with a forged
      version 1 manifest. Render text identities as five labeled lines while
      retaining legacy single-line upgrade compatibility. Await independent
      review of this follow-up.

## Phase 9: Interactive CLI and Shell Completion Usability

**Implementation status (2026-07-16):** the consolidated Phase 9 baseline and
the subsequent credential-owned provider configuration follow-up have passed
their complete verification gates and independent re-reviews. Unified ASCII
grid output for collection-shaped text results and machine-bound encrypted
SQLite credential storage are approved and pending implementation.
`docs/cli-manual.md` remains the active implemented command contract. The Codex
`official` baseline remains built in and outside provider/credential
persistence; Claude official is not implemented. Tests use temporary homes,
synthetic machine identities, and isolated encrypted stores, never real HOME,
auth.json, or credential key files.

**Follow-up implementation status (2026-07-21):** The remaining re-review
remediation is implemented and has passed the required targeted and complete
verification; independent re-review remains pending. Usage/session watch
deletion counts now use logical records/documents, Doctor has an exact
four-state schema contract and a working
text/JSON `state migrate`, and Session has CLI-level text/JSON pagination
coverage. Ordinary, `.system` child, and `.system` directory links now have a
full target-change/switch, dangling, recovery, cycle, inventory, drift, and
adoption lifecycle matrix. No install, commit, or push has been made.
The latest constrained re-review repair replaces positional Session document
comparison with deterministic logical sequence counting and adds real
Session/Usage scan-to-Watch output coverage plus activity next-page regression
coverage. Affected targeted tests, full Go test/race/vet, Darwin arm64/amd64
builds, `check-arm64-size`, `release-verify`, and `git diff --check` passed on
2026-07-21. No Doctor or Extension behavior was changed, and no real-HOME
install, commit, or push was made.
The second re-review follow-up now trims common Session document prefixes and
suffixes before exact differencing and adds deterministic 10,000-document
window tests plus non-gating benchmarks. Affected targeted tests, full Go
test/race/vet, Darwin arm64/amd64 builds, `check-arm64-size`, `release-verify`,
and `git diff --check` passed on 2026-07-21. Doctor, Extension, activity
pagination, and Watch output contracts remain unchanged; no real-HOME install,
commit, or push was made.

- [x] Replace the credential reader's terminal rejection with a shared,
      injectable no-echo reader used by `provider add` and top-level `credential
      add|update`; retain exactly one-line stdin input for automation.
- [x] Keep `provider add` as the one-step initial setup command that creates the
      provider and Keychain credential together. Retain credential subcommands
      for independent pre-provisioning and rotation, and remove the redundant
      two-command setup sequence from user documentation.
- [x] Reduce `provider use` to `<name>` plus an inferred or explicit `--client`, automatically resolve
      the standard client configuration path, and create unique `0600`
      redacted backups under the active state directory. Retain only the
      advanced `--config-path` override.
- [x] Give every leaf command a concise action description and every positional
      command explicit argument definitions plus copyable examples. Await
      independent review of this help/usability follow-up.
- [x] Replace generic DTO serialization in default `text` mode with explicit
      human-readable renderers. Start with provider `list|status|show`, then
      cover other list, detail, diagnostic, and mutation results consistently.
      Use terminal tables for collections, labeled fields for details, clear
      empty states, and concise action confirmations. Preserve the existing
      versioned envelope exclusively for explicit `--format json`; never expose
      credentials or add color-dependent meaning. Add text golden tests and
      text-vs-JSON separation tests before marking this complete.
- [x] Separate provider definition reads from credential readiness checks.
      `provider list|show` must query SQLite only; `provider status` and
      top-level `credential list` may explicitly access Keychain. Document or
      defer the doctor Keychain check instead of hiding it behind a quick read.
- [x] Add an immutable built-in `official` provider/auth mode for Codex that
      preserves the existing ChatGPT login and requires no AgentDeck credential.
      It remains outside the provider table and Keychain. Switching sets the
      managed custom provider name to `official` and removes the two managed
      custom transport fields without reading or modifying `auth.json`. Claude
      official mode remains unsupported. Do not fake official through the current
      credential-backed custom-provider schema.
- [x] Audit positional and required flag inputs against inference/defaults.
      Derive provider credential references, default multiplier to 1, retain
      existing edit values, derive remove credential ownership, infer a sole
      provider client/session client, derive the canonical price URL from its
      pinned commit, and allow `agentdeck run <client> --` with no child args.
      Normalize endpoint semantics per client and reject/document Codex URLs
      that would otherwise become `/v1/v1`.
- [x] Split usage scan accounting into normal non-usage lines, unsupported or
      incomplete usage records, malformed JSON, imported events, and updated
      events. Keep enough aggregate diagnostics to explain counts without
      exposing session contents or source paths.
- [x] Replace doctor's warning/error boolean conflation with clear terminal
      severity. A warning such as unpriced historical models should report a
      degraded cost-coverage state, not imply corrupt core state. Recovery
      commands must perform or clearly lead to an actual remedy.
- [x] Hide or disable Cobra's generated PowerShell completion command until
      PowerShell is inside the supported-shell contract.
- [x] Add named multi-credential provider/client bindings and one shared
      credential service. `provider add` orchestrates the first credential;
      top-level `credential add` adds later credentials.
- [x] Make standalone usage scan persist the watch checkpoint, read zero source
      content for unchanged files, and read only an anchor plus suffix for
      append-only files. Add `watch --domains` for independent domain bootstrap.
- [x] Keep `agentdeck completion fish|zsh|bash` output-only. Extend `make
      install` to detect the actual invoking shell through process ancestry,
      with `COMPLETION_SHELL=fish|zsh|bash|none` and `COMPLETION_RC=<path>`
      overrides.
- [x] Generate one shell-specific completion under
      `$PREFIX/share/agentdeck/completions/` and atomically add one marked source
      block to the selected shell rc without changing unrelated content or file
      mode.
- [x] Reject unsafe paths, foreign or duplicate markers, and unowned symlink
      targets. Roll back the rc file and staged artifacts if installation cannot
      complete coherently.
- [x] Upgrade the ownership manifest to version 2 with binary, completion, rc,
      managed-block, and SHA-256 ownership evidence. Keep version 1
      upgrade/uninstall compatibility, require validated ownership and a valid
      AgentDeck version contract before `FORCE=1` replaces an installation,
      and display old/new identities and hashes before replacement.
- [x] Make uninstall validate every owned artifact and the exact managed block
      before removing anything; preserve the rc file, all content outside the
      block, AgentDeck state, Keychain data, backups, and client configuration.
- [x] Add fake-TTY credential tests, secret non-disclosure tests, shell detection
      and override tests, marker/symlink/rollback/tamper tests, and real
      fish/zsh/bash completion smoke tests under temporary homes and prefixes.
- [x] Update `README.md` and `README_zh.md` with the one-step provider flow,
      output-only completion generator, automatic installation behavior,
      overrides, safe uninstall, and troubleshooting guidance.
- [x] Rerun affected targeted tests, `git diff --check`, real temporary-shell
      smoke tests, and the complete `make release-verify` gate after the
      consolidated fixes; clean generated artifacts and do not install into the
      real user home.
- [x] Complete the earlier installer-focused independent review, address
      login-shell detection, dangling rc symlink, exact rc restoration, and
      directory-ownership findings, and pass that re-review.
- [x] Address the consolidated Phase 9 review findings: unify credential
      creation and compensation, make completed operations own attribution,
      preserve historical snapshots across provider deletion, make usage/watch
      source-incremental and transactional, repair session checkpoint bootstrap,
      enumerate all doctor credentials, and add explicit text/quiet/help/golden
      contracts plus v6-to-v7 and portable backup fixtures.
- [x] Address the remaining Phase 9 re-review findings: make multi-entry
      provider removal restart-safe at every Keychain mutation, preserve v6
      completed-operation attribution after provider deletion, order session
      purge checkpoint invalidation safely, diagnose provider-use journal
      transition failures from the real config fingerprint, and make zero
      named credentials consistently unavailable across status, doctor, and
      provider use.
- [x] Preserve provider-removal recovery entries across failed compensation and
      make restart recovery idempotent; bind migrated v6 selections only to the
      containing completed operation; and share read-only provider-use external
      state classification between recover and doctor text/JSON diagnostics.
- [x] Exclude v6 selections inside failed or incomplete `provider.use` windows
      from the legacy NULL fallback while retaining genuinely pre-journal
      selections, and verify attribution before, during, and after an
      interrupted switch even after custom-provider deletion.
- [x] Pass independent re-review of the consolidated Phase 9 fixes and promote
      `docs/cli-manual.md` to the active command contract after that approval.

### Credential-Owned Provider Configuration Follow-Up (2026-07-16)

- [x] Make `provider add` create a missing provider and credential, add a missing
      named credential when the provider already exists, and return a no-prompt
      successful no-op when both metadata and secret already match. Direct
      metadata drift to `credential update` and a missing secret to
      `credential update --rotate`.
- [x] Make endpoint, multiplier, and client bindings credential-owned. Snapshot
      the selected credential's endpoint and multiplier on completed provider
      selection while keeping provider list/show definition reads SQLite-only.
- [x] Define `--credential` as the only credential shorthand. Generate and
      expose the complete `<provider>-<credential>-ref`, including `default`,
      without a client component or caller-supplied reference.
- [x] Normalize Codex-bound endpoint input by accepting an optional final `/v1`,
      storing the base without it, and writing exactly one `/v1` to Codex config.
      Preserve a final `/v1` for Claude-only credentials.
- [x] Migrate schema v6/v7 metadata to schema v8 credential rows without touching
      Keychain inside the SQLite migration. Retain private legacy storage
      references until an explicit compensated credential write moves the secret
      to its canonical reference.
- [x] Update CLI help, text/JSON output, README files, the active command manual,
      schema fixtures, migration tests, provider/credential regression tests, and
      backup/readiness paths without accessing real HOME, auth.json, or Keychain.
- [x] Run affected targeted tests, `git diff --check`, full tests, race tests,
      `go vet`, Darwin arm64/amd64 builds, the arm64 size gate, and
      `make release-verify`; clean generated binaries afterward.
- [x] Complete independent review and address its schema v8 canonicalization,
      backup credential ownership, provider DTO/status duplication,
      case-insensitive `official` reservation, unsafe endpoint, regression
      fixture, and documentation findings.
- [x] Make `provider use official` set
      `[model_providers.custom].name = "official"`, creating the table or field
      when absent while preserving unrelated TOML bytes, and cover replacement,
      insertion, idempotency, drift detection, and custom-provider round trips.
- [x] Complete independent re-review of this follow-up.
- [x] Add one shared ASCII grid renderer using `+`, `-`, and `|`, one-space cell
      padding, per-row separators, and terminal-display-width alignment. Use a
      small width-focused dependency rather than a full table UI framework and
      refresh vendored dependency metadata through the official workflow.
- [x] Migrate every existing collection-shaped text renderer to the shared grid:
      provider list/status/recover, credential list, session list/search/document
      collections, extension list, backup list, and price history. Preserve prose
      empty states, labeled detail views, and all JSON/NDJSON contracts.
- [x] Split text `provider status` activation into boolean `CODEX ACTIVE` and
      `CLAUDE ACTIVE` columns while retaining the existing JSON `active` array.
- [x] Add shared renderer unit tests, collection text golden/contract coverage,
      CJK alignment coverage, and provider activation matrix coverage; then run
      targeted tests and the complete release verification gate.

### Machine-Bound Encrypted Credential Storage Follow-Up (2026-07-16)

- [x] Add schema v9 `credential_secrets` and derived-key metadata. Store one
      algorithm/key version, random nonce, authenticated ciphertext, and update
      timestamp per credential with foreign-key cascade ownership.
- [x] Add a credential vault that lazily and atomically creates a versioned
      `0600` `<state-dir>/credential.key`, combines its random 256-bit seed with
      stable injected machine identity through HKDF-SHA256, and encrypts values
      with AES-256-GCM plus logical-reference associated data.
- [x] Replace the macOS Keychain adapter and Security framework dependency with
      the encrypted SQLite implementation. Remove external-secret compensation,
      provider-removal recovery entries, `--keep-keychain`, and all user-facing
      Keychain wording.
- [x] Commit provider/credential metadata and encrypted secret creation,
      rotation, and deletion in the same SQLite transaction. Keep
      `provider.use` journaling because client configuration remains external.
- [x] Make list/show/status readiness depend on secret-row presence without
      decrypting. Decrypt only the selected credential for `provider use`, all
      owned credentials for portable backup, and authentication checks for
      `doctor --full`.
- [x] Fail closed for missing/permissive key files, machine/key-ID mismatch,
      unsupported versions, malformed nonces, and AEAD authentication failure.
      Never overwrite ciphertext or automatically regenerate a missing key when
      encrypted rows exist.
- [x] Exclude `credential.key` from portable backups. Backup decrypts only in
      memory inside the age stream; restore generates a target-machine key and
      atomically replaces snapshot ciphertext with target-machine ciphertext.
- [x] Do not add credential migration/reset CLI or destructive install behavior.
      Schema v9 does not migrate Keychain values; pre-release local development
      state was reset out of band with explicit user approval.
- [x] Update the command manual, README files, schema/JSON fixtures, doctor,
      backup, provider, credential, installer, and uninstall contracts. Test
      synthetic machine identities, key permissions and concurrency,
      transaction rollback, corruption, cross-machine restore, zero Keychain
      access, and credential non-disclosure before the complete release gate.

### Automatic Price Update Follow-Up (2026-07-17)

- [x] Make `agentdeck price update` resolve the current LiteLLM `main` commit
      through the GitHub API and download the canonical raw catalog pinned to
      that validated SHA.
- [x] Keep `--commit` as an optional reproducibility and rollback override,
      preserve immutable commit and content-hash provenance, and cover both
      automatic and pinned request paths with isolated HTTP tests and a live
      temporary-state smoke test.
- [x] Address independent review findings: persist the exact downloaded raw
      SHA-256 consistently across update, status, and history; reject invalid
      commit overrides before state or network access; apply a bounded
      production HTTP timeout; and cover API/raw failures and zero-side-effect
      CLI validation.
- [ ] Complete independent review of the automatic price update follow-up.

### Active Usage Log Rebuild Follow-Up (2026-07-17)

- [x] Treat same-identity append-only growth during a scan as a validated stable
      prefix while keeping the appended suffix visible to the next inventory.
- [x] Keep truncate, replacement, identity change, and validated-range mutation
      detection strict; cover repeated concurrent append and real mutation
      paths with deterministic injected filesystem tests.
- [x] Replace the global delete-before-scan rebuild with per-source atomic
      replacement. Preserve prior source events, cursors, run bindings, and
      session aggregation on failure; return partial warnings without advancing
      the watch checkpoint.
- [x] Synchronize the usage specification, CLI manual, output coverage, and
      failure rollback tests, then run targeted and complete release verification.
- [x] Address independent review findings: preserve unchanged event-to-run
      attribution, isolate duplicate event ownership across source transactions,
      revalidate same-metadata snapshot bytes, keep partial warnings visible
      under `--quiet`, and cover checkpoint and bounded-read behavior.
- [ ] Complete independent re-review of the active usage log rebuild follow-up.

### Usage Output Readability Follow-Up (2026-07-20)

- [x] Keep default terminal output as text and render usage metric/list results
      with the shared ASCII grid, using sparse Emoji section titles only for
      navigation.
- [x] Split session token components into separate columns and expose known
      priced subtotals plus deterministic model coverage without changing the
      nullable complete-total contract.
- [x] Treat only Claude dot/hyphen version punctuation as equivalent, keep
      unknown Codex models unpriced, and retry bounded transient/truncated price
      catalog responses without importing partial state.
- [x] Cover default text, explicit JSON, table rendering, partial price totals,
      model matching, and retry behavior with isolated regression tests.
- [ ] Complete independent re-review of the usage output readability follow-up.

### Usage Analytics and Current State Follow-Up (2026-07-20)

- [x] Fill missing historical model/component prices from the current effective
      catalog without repricing already-known components or changing provider
      attribution and multiplier rules.
- [x] Add `provider current`, credential shorthand active cells, and per-client
      provider detail activation without reading credential values.
- [x] Give price status/history/list/update/override dedicated readable tables,
      current effective price filtering, explicit units, and verbose/JSON full
      provenance.
- [x] Add local-calendar usage summary shortcuts and a balanced indexed usage
      stats report with one range scan/aggregation, filters, stable GUI-ready
      JSON, and conditional activity heatmap.
- [x] Add schema v10 event-time indexing and focused service/CLI regression
      coverage; synchronize the active specification and CLI manual.
- [x] Remediate review findings for absolute-time event/session ranges, nullable
      complete versus known partial stats cost, fixed bulk stats metadata loads,
      stable timezone/DST output, and same-time price status semantics.
- [x] Repair Codex usage collection so every model invocation inside one turn is
      retained, accept current session metadata IDs, and use schema v11 parser
      invalidation to rebuild legacy sources without losing exact run attribution.
- [x] Remediate the usage re-review findings with schema v12/parser v2 persisted
      Codex cumulative deltas, restart/reset/archive-copy regression coverage,
      component-complete stats, client-specific cache rates, nullable partial
      cost JSON, deterministic unpriced models, compact text output, and stable
      fixtures. Keep `--period week` as the current local Monday-based week.
- [x] Scope orphan event recovery to persisted client/session candidate sources,
      preserve retryable ownership and attribution state, and prove unrelated
      unchanged histories remain unopened when either duplicate or final sources
      are removed.
- [x] Add schema v13/parser v3 safe tool metadata, model/session cache-hit
      analysis, all-model text ranking, complete cache-session JSON, opt-in
      model activity detail, and source-owned duplicate/removal recovery without
      retaining tool inputs, outputs, commands, environment, or reasoning.
- [x] Keep unpriced models in every non-cost aggregation and replace client
      read/write percentages with model/session cache hit rates plus cache-write
      token volume.
- [x] Complete independent re-review of the usage analytics and current state
      follow-up.

## Required Verification

Once Go source exists, the release gate includes:

```bash
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -race ./...
rtk lint env GOCACHE=/private/tmp/agent-deck-go-build go vet -mod=vendor ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOOS=darwin GOARCH=arm64 go build -mod=vendor -trimpath ./cmd/agentdeck
rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOOS=darwin GOARCH=amd64 go build -mod=vendor -trimpath ./cmd/agentdeck
rtk test make check-arm64-size
rtk test make release-verify
rtk git diff --check
```

Targeted package and integration tests run before the full gate. The final
commands must be run after the last edit. Any unavailable cross-build,
machine-identity/key-file, or real-runtime validation is reported as residual
risk rather than inferred from source inspection.

## Completion Gate

Phase one, Phase 8, and the consolidated Phase 9 implementation and independent
review are complete. Phase 9 is accepted and its CLI manual is active. Release
preparation remains a separate stage. The automatic price update follow-up is
implemented, review-remediated, and verified but awaits independent re-review.
The active usage log rebuild follow-up is review-remediated but awaits
independent re-review.
The usage output readability follow-up is implemented but awaits independent
re-review.
The usage analytics and current state follow-up is review-remediated but awaits
independent re-review.
Implementation or review approval does not authorize installation into the real
user home, push, release, or modification of real user state.
