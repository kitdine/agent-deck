# AgentDeck CLI Design

**Status:** active, approved for development

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
```

Global flags are:

```text
--format text|json
--no-color
--state-dir <path>
--quiet
```

`--state-dir` exists for tests and portable isolated execution. It does not
change Codex or Claude source paths unless their adapter-specific test paths are
also explicitly overridden.

## Architecture

The implementation uses focused Go packages:

```text
cmd/agentdeck
internal/provider
internal/usage
internal/session
internal/extension
internal/store
internal/platform
internal/output
```

- `provider` validates definitions, selects credentials, updates native client
  configuration, and records provider selection history.
- `usage` parses usage events, attributes runs, imports price catalogs, and
  calculates cost.
- `session` extracts approved visible conversation fields and maintains the
  local FTS index.
- `extension` defines the common inventory and capability model while native
  Codex and Claude adapters preserve their own formats.
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
that performs incremental scans at a configurable interval and exits when the
process is stopped. With `--format ndjson`, it emits versioned change events for
future GUI consumption.

When no source changed, watch does not write SQLite. If another process owns a
scan write lock, it skips that interval instead of blocking provider or query
commands.

## Backup and Device Migration

`agentdeck backup create` produces one passphrase-encrypted `.adb` bundle. The
bundle uses the age scrypt recipient format rather than a custom cipher.
Passphrases are read without terminal echo and never accepted as command-line
arguments.

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

Phase-one restore accepts only an absent or empty state root. It streams and
validates the complete archive, stages only database entries in a private
temporary directory, and keeps the decrypted credential entry in memory. It
refuses unknown schemas or secret-store conflicts, imports credentials into the
target platform store, and commits database files with owner-only permissions.
A failed restore removes only state created by that restore.

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
configuration drift, price provenance, unpriced models, source readability,
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

The committed Python and Bash implementation remains a behavioral reference
and fixture source during Go development. After each equivalent Go command has
passed its tests and independent review, legacy repository entrypoints may be
removed. AgentDeck does not provide compatibility aliases.

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
