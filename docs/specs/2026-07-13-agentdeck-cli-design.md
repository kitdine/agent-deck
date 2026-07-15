# AgentDeck CLI Design

**Status:** active, phase-one and version/installation baseline implementation
and independent review complete; interactive CLI and shell completion usability
implementation, release verification, and independent review complete

## Product Definition

AgentDeck is a small local tool for managing Codex and Claude provider
configuration, usage cost, session history, and extensions. The first release
is a single Go CLI named `agentdeck`. A later macOS menubar application will use
the same versioned JSON interface instead of reimplementing the core domains.

The first release targets macOS users. Core packages must remain portable so
that Windows and Linux adapters can be added later without changing domain
contracts.

## Goals

The first CLI release will:

- switch Codex and Claude providers while preserving unrelated client state;
- calculate local session cost from Codex and Claude JSONL usage records;
- apply a provider multiplier to money without changing token counts;
- index selected user-visible session content for local search;
- discover and diagnose plugins, MCP servers, and skills through native
  client adapters;
- create an encrypted portable backup of AgentDeck state and credentials;
- identify every binary build with a version, commit, build time, and Go
  version suitable for support diagnostics;
- provide explicit user-local install and ownership-checked uninstall targets;
- expose stable JSON for a future Swift menubar application;
- remain on-demand by default, with an optional foreground watcher.

## Non-Goals

The first release will not:

- run a daemon or install a LaunchAgent;
- implement a GUI;
- query provider billing, subscription, or usage APIs;
- reconcile estimates against invoices;
- support custom model prices;
- install, update, uninstall, or resolve dependencies for extensions;
- merge a backup into an existing AgentDeck database;
- publish Homebrew formulae, release archives, checksums, or system-wide
  installers as part of the version and installation baseline;
- maintain compatibility aliases for the legacy Python and Bash commands.

## User Interface

One executable owns every user-facing command:

```text
agentdeck provider ...
agentdeck usage ...
agentdeck session ...
agentdeck extension ...
agentdeck run ...
agentdeck watch ...
agentdeck backup ...
agentdeck doctor
agentdeck version
agentdeck --version
```

The command groups are:

```text
agentdeck provider list|show|add|edit|remove|use|status
agentdeck provider credential add|update|remove|list
agentdeck provider recover

agentdeck usage scan|summary|sessions|diagnose|rebuild
agentdeck usage price status|update|history

agentdeck session scan|list|search|show
agentdeck session exclude|rebuild|purge-index

agentdeck extension scan|list|show|doctor
agentdeck extension adopt|enable|disable|release

agentdeck run codex|claude -- <client arguments>
agentdeck watch [--interval <duration>]

agentdeck backup create|list|inspect|restore
agentdeck doctor [--full]
agentdeck version
```

Global flags are:

```text
--format text|json|ndjson
--no-color
--state-dir <path>
--quiet
--version
```

`--state-dir` exists for tests and portable isolated execution. It does not
change Codex or Claude source paths unless their adapter-specific test paths are
also explicitly overridden. NDJSON remains valid only for `watch`.

## Architecture

The implementation uses focused Go packages:

```text
cmd/agentdeck
internal/provider
internal/usage
internal/session
internal/extension
internal/watch
internal/backup
internal/doctor
internal/store
internal/platform
internal/output
internal/buildinfo
```

The canonical module is `github.com/kitdine/agent-deck`. Dependencies are
committed under `vendor/` and every test, vet, and build command uses
`-mod=vendor`; module downloads are limited to the explicit maintenance flow
that runs `go mod tidy` followed by `go mod vendor`.

`cmd/agentdeck` uses `spf13/cobra` for the command hierarchy, global flags,
argument validation, and help text. Command handlers adapt Cobra input to
focused internal services; domain packages do not depend on Cobra.

- `provider` validates definitions, selects credentials, updates native client
  configuration, and records provider selection history.
- `usage` parses usage events, attributes runs, imports price catalogs, and
  calculates cost.
- `session` extracts approved visible conversation fields and maintains the
  local FTS index.
- `extension` defines the common inventory and capability model while native
  Codex and Claude adapters preserve their own formats.
- `watch` coordinates non-blocking incremental scans and emits versioned change
  events without owning domain data.
- `backup` creates, authenticates, inspects, and restores encrypted portable
  archives without persisting plaintext credentials.
- `doctor` coordinates read-only checks through narrow store and domain
  interfaces rather than parsing another domain's tables directly.
- `store` owns SQLite schemas, migrations, transactions, locks, permissions,
  and backups.
- `platform` owns state paths, filesystem replacement, process discovery,
  Keychain access, and future OS-specific secret-store adapters.
- `output` owns stable JSON envelopes, text rendering, warnings, and errors.

Domain packages do not parse another domain's database tables directly. Shared
operations are exposed through narrow store interfaces and tested contracts.

## Local State

The default state root is:

```text
~/.agentdeck/
├── agentdeck.sqlite3
├── sessions.sqlite3
└── backups/
    ├── codex/
    ├── claude/
    └── portable/
```

The root and subdirectories use mode `0700`. SQLite databases, WAL and journal
sidecars, backups, locks, and temporary files use mode `0600`.

`agentdeck.sqlite3` stores provider definitions, client mappings, credential
references, multiplier snapshots, provider selections, exact runs, usage
events, model prices, extension inventory, managed-extension state, settings,
schema metadata, and operation journal records.

`sessions.sqlite3` stores the separately purgeable and rebuildable session
metadata and FTS5 index. Keeping visible conversation text out of the core
database lets a user remove the index without losing providers, costs, or
backup configuration.

No `providers.json` is used by AgentDeck. Legacy files are not imported or
modified automatically.

The state root is AgentDeck's persistent working directory, but the process
does not change its current working directory to the state root. Project-scoped
extension discovery and full diagnostics continue to observe the directory from
which the user invoked AgentDeck. The following locations have separate
ownership and lifecycles:

```text
~/.local/bin/agentdeck                 # installed executable by default
~/.local/share/agentdeck/              # installation ownership manifest
~/.agentdeck/                          # persistent AgentDeck user state
<invocation directory>/                # project context, never owned by AgentDeck
~/.codex/ and ~/.claude/               # client state, accessed only by contract
```

Uninstall removes only validated installation artifacts. It never removes the
state root, backups, Keychain credentials, client files, or project files.

## Build Identity and Installation

`internal/buildinfo` is the single source of binary identity. It exposes string
variables with safe development defaults and a value object used by the CLI and
backup service:

```text
version    dev
commit     unknown
build_time unknown
go_version runtime.Version()
```

Make builds inject `VERSION`, `COMMIT`, and `BUILD_TIME` with `-ldflags -X`.
Release values use a semantic version, the full Git commit SHA, and an RFC3339
UTC timestamp. Local builds keep the stable defaults unless the caller supplies
values; they do not inject the current clock automatically, preserving
reproducible local builds.

Both `agentdeck version` and `agentdeck --version` emit the same build identity.
Text output is concise and human-readable. `--format json` uses the existing
versioned envelope with command `version` and a data object containing exactly
`version`, `commit`, `build_time`, and `go_version`. `--version` is a root-only
flag and accepts the same global output flags as the `version` command. NDJSON
remains exclusive to `watch`.

Backup creation records the same `buildinfo.Version` in
`manifest.agentdeck_version`; it no longer supplies a separate hard-coded
development value. Inspect and restore continue treating that field as archive
provenance rather than an instruction to replace the running binary.

The Make installation contract is user-local and opt-in:

```text
PREFIX  defaults to $HOME/.local
BINDIR  defaults to $PREFIX/bin
DATADIR defaults to $PREFIX/share/agentdeck
```

`make install` builds the binary, creates owner-writable destination
directories, stages the executable in `BINDIR`, sets mode `0755`, verifies its
SHA-256, and atomically renames it into place. It writes an ownership manifest
containing the canonical installed path and SHA-256. It refuses an existing
destination by default. `FORCE=1` explicitly authorizes replacement at that one
path and refreshes the manifest; it does not authorize changes to any other
executable or legacy alias. An interrupted install may leave only recognizable
temporary files or a manifest mismatch that makes uninstall fail closed.

`make uninstall` reads the manifest and removes the executable only when the
requested install path and current SHA-256 both match. A missing, malformed,
path-mismatched, or hash-mismatched manifest fails closed. It removes the
manifest but leaves unrecorded installation directories in place. It never uses
a broad or recursive removal and never touches `~/.agentdeck/`.

Development and automated tests use an isolated temporary `PREFIX`; they do not
execute install or uninstall against the real user home. Homebrew, signed
archives, release checksums, and system-wide privileged installation remain
later release work. Managed shell completion installation is the next
user-local usability phase defined below.

### Managed Shell Completion Installation

`agentdeck completion fish|zsh|bash` remains a standard output-only generator.
It never edits shell configuration by itself. `make install` owns the persistent
installation workflow and generates exactly one completion script under:

```text
$PREFIX/share/agentdeck/completions/agentdeck.<shell>
```

The installer walks its process ancestry to identify the actual invoking
`fish`, `zsh`, or `bash` process rather than trusting Make's recipe shell or the
login-shell-only `SHELL` variable. `COMPLETION_SHELL=fish|zsh|bash|none`
provides a deterministic override; `none` explicitly opts out. Detection
failure stops before any installation change. `COMPLETION_RC=<path>` overrides
the default configuration path. Defaults are:

```text
fish  ${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish
zsh   ${ZDOTDIR:-$HOME}/.zshrc
bash  $HOME/.bash_profile for a detected login shell, otherwise $HOME/.bashrc
```

The installer atomically inserts one exact managed block while preserving all
unrelated bytes and the existing file mode. It records whether a separator
newline was required for an rc file that lacked a trailing newline, so forced
upgrade and uninstall can remove exactly the bytes owned by AgentDeck:

```text
# >>> agentdeck completion >>>
source "<shell-escaped canonical completion path>"
# <<< agentdeck completion <<<
```

Paths containing a newline or NUL are rejected, and the source path is escaped
for the detected shell. A symlinked rc path is accepted only when its canonical
target is a regular file owned by the current user; the symlink itself remains
unchanged. Missing rc files are created only at the resolved per-user default or
an explicit `COMPLETION_RC` path. Duplicate, partial, foreign, or modified
AgentDeck markers fail closed.

The install ownership manifest advances to version 2 and records canonical
paths plus SHA-256 values for the binary, generated completion, and exact
managed block, along with the selected shell, rc path, and separator ownership.
Installation stages all artifacts before changing the rc file and restores the
original rc on any later failure. `FORCE=1` upgrades only an existing AgentDeck
installation whose version 1 or version 2 manifest and owned artifacts validate;
it is not generic permission to replace unrelated files.

Uninstall validates every version 2 artifact and the exact managed block before
removing anything. Changes outside the block are allowed. A missing, duplicated,
or edited block, a path mismatch, or an artifact hash mismatch removes nothing.
After validation, uninstall atomically removes only the managed block, generated
completion, binary, and manifest, leaving the rc file and all unrelated content
in place. Binary-only version 1 manifests remain readable for upgrade and
uninstall compatibility.

## Provider and Credential Management

A provider definition contains its name, endpoint, supported clients,
authentication mode, model mapping, and one non-negative decimal cost
multiplier. An absent multiplier means `1`. Boolean, negative, non-finite, and
non-numeric values are invalid.

The multiplier applies only after base price calculation:

```text
provider_cost = catalog_base_cost * multiplier_snapshot
```

Raw tokens and catalog base cost never change when a multiplier changes. Each
successful provider selection and exact run stores its own multiplier snapshot,
so later provider edits do not rewrite historical cost.

Provider records store credential references, never credential values. On
macOS, credentials are stored in Keychain under AgentDeck-owned service names.
Future Windows and Linux implementations will use Credential Manager and
Secret Service through the same platform interface.

`provider add` is the primary one-step setup flow. After validating its
non-secret arguments, it reads one credential and creates both the Keychain
entry and provider definition through the existing rollback-safe service path.
`provider credential add` and `provider credential update` use the same reader
for independent pre-provisioning and rotation; initial setup documentation does
not require a redundant `credential add` before `provider add`.

When stdin is a terminal, credential commands print a reference-specific prompt
to stderr, read with terminal echo disabled, and emit a newline after input.
When stdin is not a terminal, they read exactly one line for automation and do
not print a prompt. Empty credentials are `invalid_argument` failures. Secret
values are never accepted as command-line arguments or environment variables
and never appear in stdout, stderr, JSON envelopes, logs, databases, fixtures,
or shell history.

A provider switch validates the provider, client mapping, multiplier, and
credential before changing client state. It creates a redacted backup, records
a pending operation, atomically replaces only the documented Codex or Claude
configuration fields, records the provider selection, and completes the
operation. A successful no-op selection is still recorded because it expresses
operator intent for sessions started afterward.

Codex authentication/session files and unrelated Claude settings are never
modified. Provider switching performs no endpoint health check.

## Usage Collection and Attribution

Usage scanning reads existing JSONL files read-only:

```text
~/.codex/sessions/**/*.jsonl
~/.codex/archived_sessions/*.jsonl
~/.claude/projects/**/*.jsonl
```

The importer retains timestamps, logical session and event identifiers, model
IDs, token components, source identity, byte ranges, parser version, and
attribution metadata. It does not retain prompt text, response text, tool data,
attachments, environment data, or credentials in the usage database.

Codex uses current-turn usage rather than cumulative totals. Cached input is a
subset of input and is subtracted before ordinary input pricing. Reasoning
output is diagnostic and is not added to output twice.

Claude retains these components independently:

```text
input_tokens
output_tokens
cache_creation.ephemeral_5m_input_tokens
cache_creation.ephemeral_1h_input_tokens
cache_read_input_tokens
```

Aggregate cache creation without a TTL breakdown remains visible but unpriced.
The scanner never guesses whether it was a five-minute or one-hour write.

Attribution has three explicit qualities:

- `exact`: `agentdeck run` owns an unambiguous client process lifetime and
  binds only source ranges written during that run.
- `estimated`: file-only fallback assigns the complete logical session to the
  provider selected at the session's first timestamp.
- `historical`: no earlier provider selection exists; multiplier is fixed at
  `1`.

Resuming a logical session through `agentdeck run` starts a new exact run with
the provider active at resume time. Earlier run ranges keep their prior
provider. An old exact run never captures later unwrapped events merely because
the logical session ID matches.

Overlapping or ambiguous client lifetimes are downgraded to `estimated`; the
tool never silently claims exact attribution.
The wrapper waits for every started child and propagates a failed client exit
as a runtime failure in text and JSON modes. If wrapper bookkeeping or scanning
cannot prove the source range, it closes the run as `estimated` rather than
leaving an incomplete or falsely exact run.

Incremental import tracks inode or platform file identity, path, cursor,
partial line state, size, modification time, and prefix hashes. It detects
append, equal-length prefix rewrite, growing rewrite, truncate, replacement,
archive move, and interrupted final lines. Stable event keys prevent duplicate
counting when files move or snapshots repeat.

## Price Catalog

The default operational source is the LiteLLM catalog displayed by
`https://models.litellm.ai/` and published at:

```text
https://raw.githubusercontent.com/BerriAI/litellm/main/
model_prices_and_context_window.json
```

LiteLLM is an aggregated reference source, not an official invoice source.
User-facing output therefore calls the pre-multiplier amount
`catalog_base_cost`, not `official_base_cost`.

An explicit `agentdeck usage price update` is the only normal command in this
domain that accesses the network. Runtime scans and reports use the latest
validated local catalog.

The importer:

- accepts only direct `openai` and `anthropic` records for the default catalog;
- does not mix Bedrock, Vertex, Azure, OpenRouter, or other channel pricing;
- maps OpenAI input, cached input, and output prices;
- maps Anthropic input, output, cache read, five-minute cache creation, and
  one-hour cache creation independently;
- maps `cache_creation_input_token_cost` to five-minute cache writes;
- maps `cache_creation_input_token_cost_above_1hr` to one-hour cache writes;
- uses explicit model aliases and never guesses by model-name prefix;
- rejects missing required fields rather than inferring a price.

Official vendor data may supplement or override a LiteLLM component when its
source and effective date are explicitly recorded. Every imported catalog
version stores source kind, source URL, LiteLLM Git commit SHA when applicable,
retrieval timestamp, content SHA-256, currency, schema version, and effective
time. Versions are immutable and older prices remain available.

When a source provides no verified price effective date, a newly imported
version becomes effective at retrieval time and is never backdated. Events use
the latest catalog whose `effective_from` is not later than the event. An event
earlier than every compatible version is unpriced. Missing components preserve
their token counts and produce `unpriced` output instead of a partial-looking
complete total.

`agentdeck usage price override --file <official-components.json>` imports a
local JSON array of official overrides. Each item requires `model`, direct
`provider`, `source_url`, UTC `effective_from`, and a non-empty decimal
`prices` component map. It never accesses the network; provenance is retained
as an immutable `official` catalog layer and only supplied components override
the compatible catalog components.

All prices use decimal USD per one million tokens. Calculations avoid binary
floating point; SQLite stores monetary totals as integer USD nanounits and JSON
renders decimal strings.

## Session Search

Session scanning reads Codex and Claude source logs read-only and stores only:

- session metadata, client, project or worktree, model, and timestamps;
- user-visible prompts;
- final user-visible assistant replies;
- normalized searchable text and FTS5 snippets.

It must not index system prompts, developer-only instructions, hidden
reasoning, tool arguments, tool results, credentials, authentication fields,
attachments, images, binaries, or shell environment data.

Users can exclude a project, path, session, or client. Exclusions apply during
incremental scan and rebuild. `purge-index` removes the session database without
changing source logs. The index is local, mode `0600`, and excluded from normal
portable backups unless `--include-sessions` is requested.

## Extension Management

Codex and Claude native configuration remains authoritative. AgentDeck adapters
discover plugins, MCP servers, and skills without inventing a replacement
manifest format.

A canonical extension ID is:

```text
<client>:<kind>:<scope>:<native-id>
```

Examples are `codex:mcp:user:github` and
`claude:plugin:user:claude-mem`.

The core inventory stores identity, kind, scope, source path, version when
available, enabled state, capability flags, diagnostics, configuration
fingerprint, and management state. It does not copy extension content.

Discovery and diagnosis are read-only. `adopt` explicitly records which native
entry AgentDeck may manage. `enable` and `disable` are permitted only when the
native adapter exposes an unambiguous toggle and the configuration fingerprint
still matches. Skills without a native enable/disable mechanism are reported
as `read-only`. `release` removes AgentDeck management state without modifying
or deleting the extension.

Phase one performs no extension installation, update, uninstall, marketplace
mutation, or dependency resolution.

## Foreground Watch

There is no daemon or LaunchAgent. `agentdeck watch` is a foreground process
that performs incremental usage, session, and extension scans at a configurable
interval and exits when the process is stopped. With `--format ndjson`, it emits
versioned change events for future GUI consumption. Events identify the changed
domain and scan result but never include native configuration content or session
text.

Watch persists only source metadata fingerprints. On process restart it reads
those fingerprints through the core database's read-only path and opens write
databases only after a source changed. When no source changed, watch does not
write SQLite or refresh extension inventory timestamps. If another process owns
a scan write lock, it skips that interval instead of blocking provider or query
commands.

## Backup and Device Migration

`agentdeck backup create` produces one passphrase-encrypted `.adb` bundle. The
bundle is an age-encrypted tar stream created with the maintained
`filippo.io/age` library and its scrypt recipient format rather than a custom
cipher.
Backup creation never replaces an existing destination; callers must select a
new path when the requested `.adb` already exists.
Passphrases are read without terminal echo when stdin is a terminal and as one
line from stdin for non-interactive automation. They are never accepted as
command-line arguments or environment variables.

The encrypted archive contains:

```text
manifest.json
agentdeck.sqlite3
credentials.json
sessions.sqlite3        # only with --include-sessions
```

SQLite snapshots are created through the online backup API, never by copying a
live database or WAL files. The manifest records backup schema, AgentDeck
version, creation time, source platform, database schema versions, included
components, and SHA-256 for every entry. Credential plaintext exists only in
the encrypted stream and memory, not in a temporary plaintext file.

Normal backups exclude the rebuildable session database. They never include
original client JSONL, authentication files, attachments, environment data, or
internal rollback backups.

`backup list` reports only local `.adb` file metadata. `backup inspect` requires
the passphrase and authenticates the encrypted stream, manifest, entry allowlist,
and recorded hashes before returning archive metadata.

Phase-one restore accepts only an absent or empty state root. It streams and
validates the complete archive, stages only database entries in a private
temporary directory, and keeps the decrypted credential entry in memory. It
refuses unknown schemas or secret-store conflicts, imports credentials into the
target platform store, and commits database files with owner-only permissions.
An existing empty root is committed at mode `0700`; a failed restore restores
its original mode and reports a failed permission rollback. A failed restore
removes only state created by that restore.

Restore does not modify Codex or Claude configuration. The user explicitly
runs `agentdeck provider use` after checking the restored providers.

## Transactions, Recovery, and Concurrency

SQLite-only mutations use short transactions. Filesystem mutations use an
operation journal with these states:

```text
prepared -> external_written -> database_committed -> completed
```

Recovery removes unused temporary files, restores redacted client backups when
an external write was not committed, or completes an operation whose database
and client state already agree. An ambiguous operation blocks new writes and
is reported by `agentdeck doctor`.

The database uses WAL. One state root permits only one migration, provider
switch, extension mutation, restore, or rebuild at a time. Reads may run while
short scan transactions commit. Locks time out with `state_busy`; processes are
never killed to acquire a lock.

Migrations are explicit and ordered. Known older schemas migrate
transactionally. Unknown newer schemas are rejected. Migration or rebuild
failure preserves the last usable database.

## Output and Errors

Human-readable text is the default and may improve over time. JSON is the
stable automation and GUI contract. Watch uses NDJSON.

Normal JSON uses:

```json
{
  "schema_version": 1,
  "command": "usage.summary",
  "generated_at": "2026-07-13T12:00:00Z",
  "data": {},
  "warnings": [],
  "partial": false
}
```

Token counts are integers, money is a decimal string, timestamps are UTC RFC
3339, and enums and IDs are stable strings. JSON contains no color codes,
progress animation, localized field names, or sensitive fields.

Stable JSON fixtures enumerate every Cobra leaf command and verify real success
and error envelopes, including complete command paths, data field names, empty
arrays, typed error codes, and exit codes. NDJSON fixtures compare actual watch
event serialization and pin the event field allowlist without recording session
text, native configuration, paths, or credentials.

Exit codes are:

```text
0  success, including explicitly returned non-fatal warnings
1  runtime, state, database, or filesystem failure
2  invalid command syntax or user input
```

Malformed individual JSONL records are skipped and counted. An explicit scan
fails on an unreadable source but retains prior committed data. A summary whose
automatic scan fails may return the last committed data with `partial: true`
and `scan_incomplete`. Database corruption, unknown schema, or failed migration
returns no potentially misleading summary.

Text and JSON must explicitly report estimated attribution, historical data,
unknown models, unpriced components, and incomplete scans.

## Doctor

`agentdeck doctor` is read-only. It checks state permissions, database schema,
pending operations, stale locks, credential references, provider/client
configuration drift, complete price provenance (including LiteLLM pinned commit,
canonical URL, and SHA-256), distinct unpriced models, source readability,
usage cursors, incomplete exact runs, session FTS availability, extension
fingerprints, duplicate IDs, and missing paths.

`--full` additionally performs full SQLite integrity checks and traverses all
indexed sources. Neither mode accesses the network, prints credentials, or
prints session text.

There is no generic `doctor --fix` in phase one. Recovery uses explicit
commands such as `provider recover`, `usage rebuild`, `session rebuild`, or
`extension release`.

## Security and Privacy

- Open Codex and Claude session sources read-only.
- Use parameterized SQL and structured TOML/JSON parsing.
- Validate source identities and configuration fingerprints before mutation.
- Never print or persist credential values outside Keychain and encrypted
  backup streams.
- Never place prompts or responses in the usage database.
- Keep indexed visible conversation text isolated in `sessions.sqlite3`.
- Redact credentials from rollback backups and diagnostics.
- Do not expose network ports or change host networking.
- Permit network access only for an explicit price update.
- Use synthetic logs, temporary homes, and fake credentials in tests.

## Performance and Platform Constraints

The first implementation begins with a pure-Go SQLite spike. The selected
driver must support FTS5, online backup, WAL, transactional migrations, and
macOS arm64/amd64 builds. A stripped arm64 binary has a target ceiling of 25
MiB. Failure to meet a mandatory capability causes a driver change before
business implementation expands.

With no process running, AgentDeck has zero idle resource use. Watch avoids
database writes when inputs are unchanged. Core packages avoid macOS-only types;
Keychain and process/config paths are platform adapters.

## Legacy Transition

The Python and Bash implementation served as a behavioral reference and fixture
source during Go development. After equivalent Go commands passed their tests
and independent review, the superseded repository-local entrypoints and legacy
fixtures were removed. Their historical specifications and plans remain as the
durable record. AgentDeck does not provide compatibility aliases.

Development and cleanup must not delete, overwrite, or reinstall existing
scripts under the user's `~/.local/bin/`. AgentDeck does not automatically
import the legacy `providers.json`, usage database, or real client settings.

## Acceptance Criteria

1. One `agentdeck` binary exposes every phase-one command and stable JSON.
2. macOS arm64 and amd64 builds pass with FTS5 and SQLite online backup.
3. Provider switching preserves unrelated native settings and recovers from an
   interrupted external write.
4. Credentials remain in Keychain and do not appear in databases, output,
   rollback backups, logs, or process arguments.
5. Usage import is incremental and idempotent across append, rewrite,
   truncate, replacement, and archive move.
6. Multipliers change only final money, never tokens or catalog base cost.
7. Claude five-minute and one-hour cache writes are independently priced.
8. Wrapper resume creates a new exact run; unwrapped fallback remains visibly
   estimated; unattributable history uses multiplier `1`.
9. LiteLLM catalog versions are filtered, validated, pinned, hashed, retained,
   and labeled aggregated rather than official.
10. Unknown models and missing components retain tokens and remain unpriced.
11. Session search indexes only approved visible conversation fields and can be
    excluded, rebuilt, or purged without touching source logs.
12. Extension adapters preserve native formats and mutate only explicitly
    adopted entries with supported capabilities.
13. Encrypted backup round-trips core state and credentials into an empty state
    root; sessions are included only on request.
14. Unknown schemas, failed migrations, interrupted transactions, permission
    failures, and concurrent writers preserve the last usable state.
15. Tests and command output contain no real credentials, prompts, tool data,
    hidden reasoning, or attachments.
16. No daemon, GUI, provider usage API, extension installation, or custom model
    price enters phase one.
17. Text and JSON version commands report one build identity, and injected
    release metadata is also recorded in newly created backup manifests.
18. User-local install refuses implicit replacement; uninstall removes only an
    unchanged binary proven by its ownership manifest and preserves all user
    state.
19. Release verification covers development defaults, injected metadata,
    isolated install, forced upgrade, tamper refusal, and cleanup without
    writing to the real user home.
20. Provider creation and credential rotation accept no-echo terminal input and
    one-line non-interactive stdin without exposing credential values.
21. Source installation detects or explicitly selects fish, zsh, or bash,
    installs its generated completion, and activates it through one managed rc
    block without changing unrelated shell configuration.
22. Version 2 uninstall validates the binary, completion, and managed block
    before removing any artifact, while version 1 manifests remain compatible.
23. Usability tests run against temporary homes, fake secret stores, and real
    shell processes without modifying the real user Keychain or rc files.
