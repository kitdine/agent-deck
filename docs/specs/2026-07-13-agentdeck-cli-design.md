# AgentDeck CLI Design

**Status:** active; phase-one, version/installation baseline, consolidated Phase
9 CLI usability, and credential-owned provider configuration implementation,
release verification, and independent review complete; unified ASCII list-table
output and machine-bound encrypted SQLite credential storage implemented and
release-verified, awaiting independent review

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
- publish to homebrew-core, sign or notarize archives, or provide system-wide
  privileged installers; binary distribution starts with GitHub Releases and a
  personal Homebrew tap as defined in Release and Distribution;
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
agentdeck provider list|show|status|current|add|update|remove|use|recover
agentdeck credential list|show|add|update|remove

agentdeck usage scan|summary|stats|sessions|diagnose|rebuild
agentdeck price status|list|update|history|override

agentdeck session scan|list|search|show
agentdeck session exclude|rebuild|purge-index

agentdeck extension scan|list|show|doctor
agentdeck extension adopt|enable|disable|release

agentdeck run codex|claude -- <client arguments>
agentdeck watch [--interval <duration>] [--domains usage,session,extension]

agentdeck backup create|list|inspect|restore
agentdeck doctor [--full]
agentdeck state migrate
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

Default `text` output is a human-facing interactive contract, not a serialized
representation of internal DTOs. List and metric collections use the shared
bordered ASCII grid with aligned, scannable columns;
single-resource commands use labeled fields; empty results state that no items
were found; mutation commands report the completed action without exposing
secrets or internal envelope fields. Raw JSON objects or arrays must appear only
when the caller explicitly selects `--format json`. The JSON envelope and field
names remain stable independently of text presentation.

Usage text may use sparse Emoji section titles to separate the summary, token
totals, model coverage, and session table. Session token components remain
separate columns rather than a packed cell. If one or more events cannot be
priced, the complete `catalog_base_cost` and `provider_cost` remain unavailable;
the output separately labels known priced subtotals, priced/unpriced event
counts, and per-model coverage. This exposes verified priced work without
presenting a partial amount as a complete total.

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
- `credentialvault` owns private key-file lifecycle, machine-bound key
  derivation, authenticated encryption, and non-disclosure errors.
- `platform` owns state paths, stable machine identity, filesystem replacement,
  process discovery, and OS-specific client paths.
- `output` owns stable JSON envelopes, text rendering, warnings, and errors.

Domain packages do not parse another domain's database tables directly. Shared
operations are exposed through narrow store interfaces and tested contracts.

## Local State

The default state root is:

```text
~/.agentdeck/
├── agentdeck.sqlite3
├── sessions.sqlite3
├── credential.key
└── backups/
    ├── codex/
    ├── claude/
    └── portable/
```

The root and subdirectories use mode `0700`. SQLite databases, WAL and journal
sidecars, the credential key file, backups, locks, and temporary files use mode
`0600`.

`agentdeck.sqlite3` stores provider definitions, client mappings, credential
metadata and authenticated ciphertext, multiplier snapshots, provider
selections, exact runs, usage events, model prices, extension inventory,
managed-extension state, settings, schema metadata, and operation journal
records. Credential plaintext is never stored in SQLite.

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
state root, credential key file, backups, client files, or project files.

## Build Identity and Installation

`internal/buildinfo` is the single source of binary identity. It exposes string
variables with safe development defaults and a value object used by the CLI and
backup service:

```text
version    dev
commit     unknown
branch     unknown
build_time unknown
go_version runtime.Version()
```

Make builds inject `VERSION`, `COMMIT`, `BRANCH`, and `BUILD_TIME` with
`-ldflags -X`. Version is the nearest Git tag, falling back to `v0.0.0`, with a
`-dev` suffix unless HEAD is exactly at a clean tag. Commit is the full Git SHA,
branch is the current Git branch, and build time is the actual UTC build time in
`YYYY-MM-DD HH:MM:SS` form. Callers may explicitly override every injected
value. Direct Go builds outside Make retain the stable development defaults.

Both `agentdeck version` and `agentdeck --version` emit the same build identity.
Text output uses five fixed support-facing lines in this order:

```text
Release Version: <version>
Git Commit Hash: <commit>
Git Branch: <branch>
Go Version: <go_version>
UTC Build Time: <build_time>
```

`--format json` uses the existing versioned envelope with command `version` and
a data object containing exactly `version`, `commit`, `branch`, `build_time`,
and `go_version`. `--version` is a root-only flag and accepts the same global
output flags as the `version` command. NDJSON remains exclusive to `watch`.

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
execute install or uninstall against the real user home. Release archives,
checksums, and Homebrew tap distribution are defined in Release and
Distribution below. Signed or notarized archives and system-wide privileged
installation remain later release work. Managed shell completion installation
is the next user-local usability phase defined below.

### Release and Distribution

The binary distribution channel is GitHub Releases on the public
`github.com/kitdine/agent-deck` repository plus a personal Homebrew tap.
`homebrew-core` submission is out of scope for now.

`make release-archive` packages the existing `build-all` outputs after
stripping. For version `<ver>` (from the same Git-tag derivation as build
identity) it produces exactly:

```text
dist/agentdeck_<ver>_darwin_arm64.tar.gz    (contains one file: agentdeck)
dist/agentdeck_<ver>_darwin_amd64.tar.gz    (contains one file: agentdeck)
dist/agentdeck_<ver>_checksums.txt          (SHA-256 per archive)
```

Archives contain the stripped binary — the same artifact class the
`check-arm64-size` gate measures. Packaging is deterministic and local; it
performs no network access and no uploads.

A GitHub Actions release workflow (`.github/workflows/release.yml`) triggers on
pushed `v*` tags. It runs on a macOS arm64 runner with full Git history so tag
derivation works, builds with the vendored module set, runs
`make release-verify` as the gate, then `make release-archive`, and creates the
GitHub Release with the two archives and the checksum file attached. Release
notes follow the repository release-note structure and are finalized at tag
time. The workflow needs only `contents: write` permission. A separate minimal
CI workflow runs `make verify` on pushes and pull requests. Pushing tags and
publishing releases remain explicitly authorized manual decisions; the
workflow only automates the mechanics after a tag is pushed.

The Homebrew tap lives in the separate repository `kitdine/homebrew-tap` with
`Formula/agentdeck.rb`. The formula installs prebuilt binaries: `on_arm` and
`on_intel` blocks reference the release archives by URL with their SHA-256
values, installation is `bin.install "agentdeck"`, and the formula `test`
block asserts the text version contract. Users install with
`brew install kitdine/tap/agentdeck`. A tap-installed binary reports the tag
version, not `dev`. Formula version and checksums are updated manually per
release; automated tap bumping from the release workflow is later work.
Completion-script installation through the formula is later work; tap installs
provide the bare binary and `make install` remains the completion-managed
source path.

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
installation whose version 1 or version 2 manifest and owned artifacts validate.
That manifest check proves ownership and absence of post-install tampering, not
binary provenance. Before replacement, the installer also executes both the
installed and staged binaries, requires a valid AgentDeck text version contract,
and prints their identities and SHA-256 values. It is not generic permission to
replace unrelated files and does not substitute for signed release provenance.

Uninstall validates every version 2 artifact and the exact managed block before
removing anything. Changes outside the block are allowed. A missing, duplicated,
or edited block, a path mismatch, or an artifact hash mismatch removes nothing.
After validation, uninstall atomically removes only the managed block, generated
completion, binary, and manifest, leaving the rc file and all unrelated content
in place. Binary-only version 1 manifests remain readable for upgrade and
uninstall compatibility.

## Provider and Credential Management

A custom provider is a named logical group for one or more credentials, its
aggregate supported clients, authentication mode, and client-specific model
mappings. Each credential owns its base endpoint, client bindings, and one
non-negative decimal cost multiplier. An absent multiplier means `1`. Boolean,
negative, non-finite, and non-numeric values are invalid. Codex additionally
exposes an immutable built-in provider named `official`. It is always visible to
`provider list|show`, is not stored in the `providers` table, has no endpoint or
credential reference, and never initializes or accesses the encrypted
credential store.

The multiplier applies only after base price calculation:

```text
provider_cost = catalog_base_cost * multiplier_snapshot
```

Raw tokens and catalog base cost never change when a multiplier changes. Each
successful provider selection and exact run stores its own multiplier snapshot,
so later credential edits do not rewrite historical cost.

Custom providers may have multiple named credentials. A credential name is
unique within its provider, one credential may bind to Codex and Claude, and a
provider/client may have multiple credentials. Users select by provider name and
credential short name. `--credential` is the only user-facing shorthand flag.
The immutable logical reference is always
`<provider>-<credential>-ref`, including `<provider>-default-ref`; client
bindings do not participate in the reference. `--credential-ref` and
`--credential-clients` are not part of the public CLI. Credential records store
references and metadata in `provider_credentials` and authenticated ciphertext
in the one-to-one `credential_secrets` table. The secret table stores the
credential ID, algorithm version, key version, random nonce, ciphertext, and
update time. It never stores plaintext.

The first credential write lazily creates `<state-dir>/credential.key` through
an exclusive atomic `0600` write. The versioned key file contains a 256-bit
random seed. AgentDeck combines that seed with a stable platform machine
identity through HKDF-SHA256 and derives an AES-256-GCM key; macOS uses
`IOPlatformUUID`, not hostname or a boot-scoped identifier. The raw machine
identity is never persisted. A derived key ID in SQLite detects a different
machine before decryption. Each credential uses a fresh random nonce and binds
its logical reference and format version as authenticated associated data.

This model protects a copied database when the private key file or machine
identity is absent. It does not claim to protect against a process running as
the same OS user that can read both the database and key file. Obfuscating a
salt or deriving a key from public machine data alone is not considered
encryption-key protection.

Credential endpoints use one stored base form. URL parsing and trailing-slash
normalization happen before persistence. When the credential binds Codex and
the normalized URL path ends in `/v1`, AgentDeck removes that final segment
before storing it. Codex configuration always appends exactly one `/v1` to the
stored base; Claude configuration uses the stored base unchanged. A
Claude-only credential therefore preserves a user-supplied final `/v1`, while a
credential bound to both clients follows the Codex-aware normalization rule.
Endpoints must be absolute URLs with a scheme and host. Userinfo, query strings,
and fragments are rejected so credentials cannot be embedded in endpoint
metadata and client configuration cannot receive ambiguous suffixes.

`provider add` is the primary one-step setup flow for both provider creation and
later credential addition:

```text
agentdeck provider add <provider> --credential <short-name> \
  --endpoint <base-endpoint> --clients <clients> [--multiplier <decimal>]
```

If the provider does not exist, the command creates its logical definition and
the credential. If the provider exists and the named credential does not, the
command merges the new client bindings into the provider's aggregate clients
and adds the credential. If the named credential already has identical
endpoint, multiplier, and bindings and its secret is present, the command is a
successful no-op and does not read another secret. Different metadata is an
`invalid_provider` error directing the user to `credential update`. Existing
metadata with a missing secret directs the user to `credential update --rotate`
instead of repairing it implicitly. Provider and credential existence,
normalized metadata, bindings, logical reference, database collisions, and
encrypted-secret row collisions are checked before prompting for a new value.

Top-level `credential add|update` use the same credential service,
normalization, collision checks, reader, encryption path, and SQLite
transactions as `provider add`. `credential add` accepts the same
credential-owned metadata. `provider update` infers the credential when exactly
one exists and otherwise requires `--credential`; it updates the same
credential metadata as `credential update`. Provider/credential creation,
metadata updates with optional secret rotation, and removal commit metadata and
ciphertext atomically in one SQLite transaction. They require no external
secret-store compensation or recovery journal.

When stdin is a terminal, credential commands print a reference-specific prompt
to stderr, read with terminal echo disabled, and emit a newline after input.
When stdin is not a terminal, they read exactly one line for automation and do
not print a prompt. Empty credentials are `invalid_argument` failures. Secret
values are never accepted as command-line arguments or environment variables
and never appear in plaintext in stdout, stderr, JSON envelopes, logs,
databases, fixtures, or shell history.

A custom provider switch validates the provider, selected credential, client
binding, endpoint, and multiplier before changing client state. Its primary CLI
is:

```text
agentdeck provider use <name> [--client codex|claude] [--credential <name>]
```

Unique client and credential choices are inferred. Codex resolves to
`~/.codex/config.toml` and Claude resolves to `~/.claude/settings.json`; an
advanced `--config-path` flag supports non-standard installations. The CLI
never asks users to choose a backup path. Each switch creates a unique private
backup at
`<state-dir>/client-backups/<client>/<operation-id>.redacted.toml|json`, records
that path in the pending operation, atomically replaces only the documented
client configuration fields, and commits the completed operation plus an
immutable selection snapshot in one database transaction. Endpoint and
multiplier come from the selected credential, not the provider group. Provider,
credential, client, endpoint, and multiplier attribution always come from that
same completed operation. A failed or incomplete selection leaves the prior
completed selection authoritative. A successful no-op selection is still
recorded because it expresses operator intent for sessions started afterward.

Selecting `official` is Codex-only and uses Codex's existing OpenAI or ChatGPT
login state. AgentDeck keeps `model_provider = "custom"`, sets
`[model_providers.custom].name = "official"`, and removes `base_url` and
`experimental_bearer_token` from that table. If the custom table or its `name`
field is absent, AgentDeck creates the missing structure. Existing `name`
spacing and inline comments are preserved while its value changes; all other
TOML fields, comments, ordering, and formatting remain unchanged. Missing
transport fields are a successful no-op.
AgentDeck never reads, checks, writes, backs up, or deletes
`~/.codex/auth.json`; Codex alone owns authentication. The built-in provider
does not create a provider record, credential reference, encrypted secret row,
or credential key file. It does create the same operation-linked immutable
selection snapshot as custom
providers, containing `official`, Codex, no credential, and multiplier `1`.
Historical attribution treats the completed transaction time as the switch
boundary. Claude has no built-in `official` provider.

Deleting a custom provider is allowed after use. The live definition and all
credential metadata and ciphertext are removed in one SQLite transaction,
while selection snapshots retain historical name, endpoint, credential name,
client, and multiplier attribution. Provider and credential deletion no longer
create external-secret recovery entries.

`provider list` and `provider show` read provider definitions without accessing
or decrypting credential ciphertext. Because endpoint and multiplier are
credential-owned, `provider list` shows provider name, type, aggregate clients,
and credential count rather than a single endpoint or multiplier. Credential
readiness belongs to `provider status` and top-level `credential` commands and
checks secret-row presence without decrypting values. Text `credential list`
contains `PROVIDER`, `NAME`, `REFERENCE`, `ENDPOINT`, `MULTIPLIER`, `CLIENTS`,
and `READY`; credential detail and JSON expose the same non-secret metadata. Output
never reports credential values or private compatibility references. Provider
definition JSON contains aggregate `clients` and `credential_count`, but no
endpoint, multiplier, credential reference, or nested credential details.
Provider status exposes credential detail only through the plural `credentials`
collection and has no deprecated singular `credential` projection.

Every collection-shaped `text` result uses one shared ASCII grid renderer. This
includes provider list/status/recovery collections, credential lists, session
list/search/document collections, extension lists, backup lists, and price
history. The renderer uses only `+`, `-`, and `|`, adds one space of horizontal
cell padding, and draws a horizontal separator around the header and every data
row. It does not use Unicode box-drawing characters. Empty collections keep
their existing concise prose instead of rendering an empty grid, and detail
views keep their labeled-field layout.

Grid column widths are calculated from terminal display width rather than byte
length so CJK and other wide text remain aligned. Cells are left-aligned and are
not truncated or wrapped. The implementation uses one small width-focused
dependency rather than a full table UI framework, vendors it through the normal
dependency workflow, and must continue to pass the release size gate. JSON and
NDJSON contracts are unchanged.

Text `provider status` uses the columns `CODEX ACTIVE` and `CLAUDE ACTIVE`.
Each active cell contains the selected credential shorthand; inactive cells and
the built-in `official` credential display `-`. Detail status includes one row
per client with active state, shorthand, and selection time. The additive JSON
`active[].selected_at` field retains the selection timestamp.

`provider current` returns the latest completed selection for each client as
`client`, `provider`, optional credential shorthand, and `selected_at`.
`official` has no credential. Current/status reporting reads only selection and
credential metadata and never reads or decrypts credential values.

Every leaf command has a concise action description. Commands with positional
arguments additionally provide an `Arguments` section defining every value and
an `Examples` section with copyable invocations. `provider add` help explicitly
shows both first-provider creation and later credential addition, defines
`--credential` as the short name, and documents client-aware `/v1`
normalization. Help must expose defaults, managed paths, safety effects, and
advanced overrides without requiring users to consult source code.

Codex authentication/session files, including `auth.json`, and unrelated Claude
settings are never modified. Provider switching performs no endpoint health
check.

## Usage Collection and Attribution

Usage scanning reads existing JSONL files read-only:

```text
~/.codex/sessions/**/*.jsonl
~/.codex/archived_sessions/*.jsonl
~/.claude/projects/**/*.jsonl
```

The importer retains timestamps, logical session and event identifiers, model
IDs, token components, source identity, byte ranges, parser version,
attribution metadata, and an allowlisted tool-call record containing only tool
name, start/completion time, terminal status, and duration when derivable. It
does not retain prompt text, response text, tool arguments or results, command
text, attachments, environment data, reasoning, or credentials in the usage
database.

Codex treats `total_token_usage` as a cumulative snapshot and imports the
non-negative component-wise delta from the previous valid cumulative snapshot
as one model-invocation usage event. Multiple invocations inside the same logical turn remain separate even
when they share a timestamp; the stable event identity combines the session,
turn, model, timestamp, and canonical last/total usage snapshots rather than
collapsing the turn to one row. The retained `event_id` remains the logical turn
ID so invocation events can still be grouped by turn. The previous cumulative
input, cached-input, and output values are persisted per source/session in the
source cursor, so append scans and process restarts continue the same delta.
A missing/first/reset cumulative snapshot safely falls back to a valid
`last_token_usage`; a missing total invalidates the baseline until a new valid
total establishes it. An unchanged cumulative snapshot emits no usage event.
Cached input is a subset of input and is subtracted
before ordinary input pricing. Reasoning output is diagnostic and is not added
to output twice. Current Codex `session_meta.payload.id` and the legacy
`payload.session_id` are both accepted.

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

Usage source cursors record a parser version. Schema v11 initializes existing
sources to an outdated version so the next normal usage scan atomically rebuilds
each source with invocation-level Codex events; new sources record the current
parser version immediately. A parser-only rebuild preserves completed exact-run
attribution by reapplying the recorded source byte ranges to the new event keys.
If one source cannot be rebuilt, its previous cursor, events, sessions, and run
bindings remain usable.

Schema v12 adds the persisted Codex cumulative cursor. Parser version 2 forces
source-atomic rebuilds so source rebuild, mutation, and parser invalidation all
restart cumulative state from byte zero. Stable Codex event identity uses the
logical session/turn/model/timestamp and canonical usage snapshots, never the
source path, so archived copies do not duplicate cost.

Schema v13 adds source-owned safe tool-call metadata. Parser version 3 rebuilds
each source atomically to populate it. Tool-call identity uses the logical
client/session call ID, never the source path; duplicate archives therefore do
not duplicate activity, and candidate-only orphan recovery re-homes tool
ownership when an original or archive disappears. The last real source removal
deletes its tool metadata. Source mutation, parser-version rebuild, and failed
candidate retry retain the same atomicity and unchanged-source isolation
guarantees as usage events.

When an event-owning source disappears, its events remain temporarily orphaned
until the same scan can re-home matching stable keys. Recovery combines each
inventory entry's client with its persisted source-session cursor and force
rebuilds only unchanged sources for affected client/session pairs. Added,
appended, and mutated paths continue through their normal incremental
classification, unrelated unchanged sources are never opened, and a source
with no candidate copy is removed without scanning unrelated history. A failed
candidate read preserves the orphan events, run bindings, session aggregation,
and cumulative cursor state for retry and prevents checkpoint advancement.

Each inventory classifies paths as added, appended, mutated, or removed. Normal
scan and watch pass only classified paths to the content scanner; an unchanged
historical file is not opened. Processing and checkpointing use the same stable
inventory. If the same source identity grows while it is being scanned, the
scanner revalidates the bounded snapshot bytes and cursor anchor, commits that
stable prefix, and leaves the later suffix visible to the next poll. Truncate,
replacement, identity change, or changes inside the validated snapshot remain
hard mutations rather than being accepted as append-only growth. Byte
revalidation applies even when the final size and modification time match the
inventory entry, while remaining bounded to the cursor anchor and the snapshot
suffix instead of rereading the complete historical prefix.
Reset/replacement and removed-source cleanup are source-level transactions that
atomically update source state, events, exact-run source metadata, and session
aggregation. `watch --domains usage` does not create or open the session store.

`usage rebuild` also operates one source transaction at a time instead of
deleting all rebuildable usage tables before scanning. A failed source keeps
its prior events, cursor, event-to-run bindings, exact-run source ranges, and
session aggregation; successful sources may still complete. Duplicate stable
event keys use deterministic canonical-path ownership. Rebuild processes the
same priority from highest to lowest, and a lower-priority source cannot update
an event still owned by a higher-priority source whose transaction failed.
The command returns success with
`partial: true` and a stable `usage_source_unstable` or
`usage_source_rebuild_failed` warning, and does not advance the usage watch
checkpoint until every source succeeds. A forced rebuild of unchanged content
does not increment `source_resets` and preserves valid event-to-run bindings and
exact-run source ranges, while a detected mutation continues to invalidate
bindings whose byte ranges can no longer be trusted. Text `--quiet` still emits
partial rebuild warnings and suppresses only a complete warning-free success.

`usage summary` without an argument covers all history. The `daily`, `weekly`,
and `monthly` shortcuts cover today, the current Monday-based week, and the
current month in the machine timezone.

`usage stats` defaults to `period=7d`, `group-by=auto`, and `metric=tokens`.
It accepts `today`, `7d`, `30d`, `week`, `month`, `6m`, and `all`, or an
inclusive local-date `--from/--to` pair. Explicit grouping supports hour, day,
Monday-based week, and month; `period=week` is the current local Monday 00:00
through now and remains distinct from rolling `7d`. Filters accept client,
exact model, and exact runtime provider name. One indexed
`event_at` range query loads filtered events once, then one aggregation pass
produces totals, trend buckets, model ranking, client share, runtime provider
ranking, averages, peak,
pricing coverage, and activity. Schema v10 adds the time-range index and does
not add a persistent statistics table. Migration and all new writes canonicalize
`usage_events.event_at` to UTC RFC3339Nano and recompute session first/last from
those canonical events. Summary ranges, stats ranges, earliest-event lookup, and
session boundaries therefore compare absolute time rather than raw RFC3339 text
that may contain different offsets. The range report performs one event load,
one effective-price load, and one metadata-only provider-timeline load; run
multiplier, session attribution, provider snapshots, and price selection are
resolved during the single in-memory aggregation without per-event SQL or
credential-value access.

The runtime provider dimension groups events by the provider configuration
selected through AgentDeck (for example `official` or a custom relay), not by
the price-catalog vendor. Each event's provider name is derived during the same
in-memory aggregation, mirroring the existing attribution quality branches: an
exact run-bound event uses the recorded `usage_runs.provider`; an estimated
event uses the provider-timeline snapshot at its session start; an event whose
session predates every recorded provider selection is grouped as `unknown`.
`unknown` is an explicit unattributed bucket, never silently mapped to
`official`, and `--provider unknown` selects exactly those events. Provider
dimensions are keyed per client — the same provider name under Codex and
Claude denotes different vendors and different cache-rate semantics, so they
are never merged across clients. No schema change stores provider on events;
the dimension is derived, and `usage_events` stays as is. The `--provider`
value is an open set and is not enumerated at parse time; a non-empty value
filters the whole report — totals, buckets, models, clients, providers, cache
sessions, activity, peak, and coverage all reflect only matching events, the
same global-filter semantics as `--client` and `--model`. Tool-call activity
rows carry no run binding, so under a provider filter they are attributed by
the session-start snapshot alone; this session-level approximation is the only
attribution difference from token events and applies only when `--provider` is
set.

The stable stats JSON data object contains `range`, `timezone`, `totals`,
`buckets`, `models`, `clients`, `providers`, all cache-relevant
`cache_sessions`, `activity`,
`peak`, `coverage`, and sorted `unpriced_models`. Totals, buckets, models,
clients, providers, and cache sessions expose input, output, cached-read, and
cache-write
components. Codex model/session cache hit rate is cached input divided by
input. Claude logical input is ordinary input plus cache read plus cache write,
and its model/session hit rate is cache read divided by that logical input;
cache writes remain a token volume rather than a second hit-rate percentage.
Mixed totals and buckets expose components without inventing one cross-client
cache rate. Pricing completeness affects only cost fields and cost ranking:
unpriced models continue to participate in tokens, shares, sessions, events,
cache, activity, and tool counts. `providers` entries are client-scoped
dimensions sorted like models (known metric value descending, then client,
then name) and expose the same share, cost, cache, session, and event fields.

Text always
uses the approved responsive Balanced layout: compact token/cost/session KPIs,
bar-based trend and all-model sections, client share, a PROVIDERS ranking
directly after the client share section, model/session cache hit
analysis, and an average/peak/priced footer. PROVIDERS rows are labeled
`<Client>/<provider>` with the same proportional bars and share labels as the
client section and an empty-state line when the range has no providers. Cache
text shows all relevant
models and the first ten deterministically sorted sessions, reports the omitted
count, and gives each session a copyable
`agentdeck session show <id> --client <client> --activity` command. Wide
terminals use two columns; narrow terminals stack the same sections without
exceeding the detected width. Ranges spanning at least seven
local calendar days include a full-width 7-by-24 activity heatmap at the bottom;
hour ranges omit it. TTY color is optional and `--no-color` or redirected output
contains no ANSI escapes. `timezone` is a stable IANA identifier when the
machine zone can be resolved and otherwise an explicit `UTC+HH:MM` offset. Hour
bucket boundaries retain their RFC3339 offsets so both hours in a DST fold
remain distinct.

`usage stats --model <model> --activity` adds that model's active session/day
range, safe tool-call totals, completion/failure counts, available durations,
and deterministic tool-name distribution. No tool arguments or results are
read into the report.

For `metric=cost`, complete average, metric, share, and peak values are nullable.
They are present only when the applicable events are fully priced; the parallel
`known_average_cost_per_session`, `known_metric_value`, `known_share`, and
`known_value` fields always describe the known priced subtotal. Stats text marks
known partial values with `KNOWN` only beside the affected cost and lists
deterministic model/component gaps in `UNPRICED MODELS`; it uses unavailable
when no amount is known and has no generic partial-cost footnote.

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

An explicit `agentdeck price update` is the only normal command in this
domain that accesses the network. Runtime scans and reports use the latest
validated local catalog.

Without `--commit`, the updater resolves the current LiteLLM `main` commit
through the GitHub API, validates the returned 40-character SHA, and downloads
the catalog from the canonical raw URL pinned to that SHA. An optional
`--commit` skips discovery for reproducible imports and rollback. The importer
never records a mutable `main` URL as catalog provenance. Explicit invalid
commit overrides fail before AgentDeck opens state or initializes the HTTP
client. The production HTTP client applies a 60-second total timeout while
still honoring request-context cancellation. Commit discovery and pinned raw
catalog retrieval make at most three attempts for transient transport/read
failures, HTTP 408/429/5xx responses, and truncated JSON. Invalid non-transient
responses fail without importing state, and a catalog is persisted only after a
complete response passes parsing and direct-provider validation.

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
time. For LiteLLM, `content_sha256` hashes the exact downloaded raw catalog and
the same value is returned by update, status, and history. Versions are
immutable and older prices remain available.

When a source provides no verified price effective date, a newly imported
version becomes effective at retrieval time and is never backdated. Events use
the latest catalog whose `effective_from` is not later than the event. If that
historical result lacks a compatible model or one price component, the current
effective merged catalog fills only the missing values. Components already
calculable at event time are never overwritten or repriced, and this fallback
adds no estimate/fallback marker. A model absent from both historical and
current local catalogs remains unpriced. Missing components preserve
their token counts and produce `unpriced` output instead of a partial-looking
complete total. Summary and session JSON preserve those nullable complete
totals and additionally expose `known_catalog_base_cost` and
`known_provider_cost`; summary JSON also exposes priced/unpriced event counts
and deterministic per-client/model coverage. Claude catalog matching accepts
dot-versus-hyphen version punctuation only when both names begin with
`claude-`; other model names still require exact or explicit alias matching.

`agentdeck price override --file <official-components.json>` imports a
local JSON array of official overrides. Each item requires `model`, direct
`provider`, `source_url`, UTC `effective_from`, and a non-empty decimal
`prices` component map. It never accesses the network; provenance is retained
as an immutable `official` catalog layer and only supplied components override
the compatible catalog components.

All prices use decimal USD per one million tokens. Calculations avoid binary
floating point; SQLite stores monetary totals as integer USD nanounits and JSON
renders decimal strings.

`price list [model] [--provider openai|anthropic]` renders the current
component-wise merged effective catalog with the explicit unit `USD / 1M
tokens`. Default status, history, list, update, and override text use readable
tables and omit long URLs and complete digests. JSON and `--verbose` text retain
full component provenance, including source URL, catalog version, commit, hash,
and effective time. Status determines top-level provenance, active catalogs,
availability, model count, and component count from the same current absolute
time. A future-only catalog is unavailable; when current and future catalogs
coexist, every status field describes only the current effective set. Catalog
and model RFC3339 timestamps are parsed before filtering and precedence sorting,
so valid offsets cannot change effective-time semantics.

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
Purging also clears only the session watch checkpoint, so the next session watch
bootstraps the deleted index without invalidating usage or extension checkpoints.
Session IDs shared by Codex and Claude are ambiguous unless `--client` is given.
`session show <id> --client <client> --activity` opens the selected source only
on demand and displays the same allowlisted tool name, timestamps, status, and
duration metadata. It never persists activity in `sessions.sqlite3` and never
returns tool arguments, results, command text, environment, or reasoning.

Session text collections are bounded for readable terminal use. `session list`
and the document and activity-detail collections in `session show` default to
page one with 20 rows in text mode. They accept positive `--page` and `--limit`
values plus `--all`; `--all` is mutually exclusive with explicit paging. Client
filters apply before pagination. Ordering remains deterministic: sessions use
descending last activity followed by client and session ID, documents retain
source order, and activity detail uses start time followed by stable call
identity. Session metadata is always shown even when its document page is empty.

Each text collection ends with `Showing <first>-<last> of <total>` and, when
more rows exist, a copyable command for the next page. `session show --activity`
first displays an aggregate over the complete selected session: total,
completed, failed, and incomplete calls; total and average known duration; and
deterministically sorted per-tool counts. Only the activity detail rows are
paged. Model-level activity remains available through
`usage stats --model <model> --activity`.

JSON without paging flags retains the complete existing data collections for
compatibility. Explicit paging applies the same query limits as text and adds
an optional top-level `pagination` object keyed by collection (`sessions`,
`documents`, or `activity`). Each entry contains page, limit, total, shown,
has-more, and next-page values. The additive pagination metadata never contains
source paths or session content.

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

Skill discovery follows valid directory symlinks for an ordinary skill, a
child of `.system`, and the `.system` directory itself. The logical extension
ID is derived from its native namespace rather than the resolved target path.
Target content changes or target switches update the live fingerprint and
managed drift deterministically. A dangling link or cycle fails discovery
before inventory replacement, preserving the previous inventory row, managed
state, and adopted fingerprint. Recovery resumes discovery with adoption still
intact. Other hidden skill directories remain excluded. Watch fingerprinting
records broken or cyclic links without recursively traversing a cycle.

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

Watch `changes` uses the same logical unit for additions, updates, and removals.
Usage reports logical usage events or records; session reports currently visible
documents; extension reports inventory entries. Removing one duplicate source
while another source owns or takes over the same logical data emits zero
changes. Removing the final source emits the number of logical records or
documents that actually disappear, never the number of removed source paths.
Session document sequences are compared deterministically by approved kind and
text rather than array position or source path. Inserting or deleting one
document at any position emits one logical change, replacing one document emits
one update, and repeated text does not turn a single edit into shifted updates.
Before exact sequence differencing, identical prefixes and suffixes are removed
so unchanged and isolated-edit scans process only the changed document window.

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
`credentials.json` is derived exclusively from current `provider_credentials`
rows joined to their `credential_secrets` rows. A provider with zero
credentials or a credential without ciphertext contributes no secret. The
machine-bound `credential.key` file is never included in a portable backup.
The legacy provider-level credential column is never used as a fallback
ownership source.

A raw state-directory copy is usable only with the same stable machine identity.
Cross-machine transfer uses portable backup/restore so credential values are
authenticated inside the age stream and re-encrypted for the target machine.

Normal backups exclude the rebuildable session database. They never include
original client JSONL, authentication files, attachments, environment data, or
internal rollback backups.

`backup list` reports only local `.adb` file metadata. `backup inspect` requires
the passphrase and authenticates the encrypted stream, manifest, entry allowlist,
and recorded hashes before returning archive metadata.

Phase-one restore accepts only an absent or empty state root. It streams and
validates the complete archive, stages only database entries in a private
temporary directory, and keeps the decrypted credential entry in memory. It
refuses unknown schemas, creates a new target-machine credential key, replaces
snapshot ciphertext with target-machine AES-GCM ciphertext in one transaction,
and commits database and key files with owner-only permissions.
An existing empty root is committed at mode `0700`; a failed restore restores
its original mode and reports a failed permission rollback. A failed restore
removes only state and key material created by that restore.

Restore does not modify Codex or Claude configuration. The user explicitly
runs `agentdeck provider use` after checking the restored providers.

## Transactions, Recovery, and Concurrency

SQLite-only mutations use short transactions. Filesystem mutations use an
operation journal with these states:

```text
prepared -> external_written -> completed
        \-> failed
```

For `provider.use`, `prepared` persists the target configuration path, its
pre-write fingerprint, and the redacted backup path before the client file is
changed. If recording `external_written` fails, failure recording is attempted;
after a process restart `provider recover` compares the persisted fingerprint
with the real client file and distinguishes an interruption before the write
from one after the write. Because the backup is intentionally redacted and
cannot recreate a bearer credential, recovery diagnoses an external write
rather than claiming to restore the client file automatically. Failed and
`external_written` operations remain visible to doctor with explicit recovery
guidance. Doctor uses the same read-only fingerprint classifier as recovery and
reports transition/failure codes without changing the operation journal.

Provider and credential creation, rotation, and deletion mutate credential
metadata and ciphertext together in short SQLite transactions. Provider removal
uses foreign-key cascades for live credential metadata and ciphertext while
selection snapshots retain historical attribution. These operations no longer
use external-secret operation journals. The `provider.use` journal remains
because native client configuration is still external filesystem state.

The database uses WAL. One state root permits only one migration, provider
switch, extension mutation, restore, or rebuild at a time. Reads may run while
short scan transactions commit. Locks time out with `state_busy`; processes are
never killed to acquire a lock.

Migrations are explicit and ordered. Known older schemas migrate
transactionally. Unknown newer schemas are rejected. Migration or rebuild
failure preserves the last usable database. The v6-to-v7 provider selection
backfill associates a selection only with a completed `provider.use` whose
started/updated time window contains `selected_at`. A selection inside any
failed or incomplete `provider.use` window is discarded instead of becoming an
authoritative `operation_id = NULL` fallback. Only a historical selection that
belongs to no `provider.use` window retains that compatibility fallback. The
backfill enforces at most one selection per completed operation before provider
IDs can become `NULL` on definition deletion.

Schema v8 adds base endpoint, normalized multiplier, and canonical logical
reference to every credential. Its transaction determines Codex ownership from
credential client bindings, removes a final `/v1` for Codex-bound credentials,
preserves it for Claude-only credentials, lowercases the provider and credential
components of `<provider>-<credential>-ref`, and rewrites valid multipliers to
the canonical 12-decimal representation. The logical-reference unique index is
created only after every row is backfilled, so canonical collisions fail the
whole migration without leaving a partial schema.

Schema v9 creates `credential_secrets` and the derived-key metadata used by the
machine-bound encrypted store. It never reads or migrates Keychain values. This
is an unreleased development transition: existing local development state is
reset out of band with explicit user approval, and AgentDeck adds no migration
or reset CLI. Known older database schemas may still migrate their non-secret
metadata, but no credential is ready until it receives a new encrypted secret.
Installation and source upgrades never delete the state root automatically.

## Output and Errors

Human-readable text is the default. Each collection, detail, empty result,
mutation, doctor report, and usage report has an explicit renderer; internal Go
DTO reflection is not a user-facing contract. Optional costs print their decimal
value or `unavailable`, never pointer representations. `--quiet` suppresses only
successful non-essential text mutation output. JSON and errors are unaffected.
JSON is the stable automation and GUI contract. Watch uses NDJSON.

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

Doctor must remain usable before an upgrade migration has run. It reads the
stored schema version before domain queries, reports `schema_outdated` with the
stored and supported versions, and runs only checks whose tables and columns
exist at that version. A table introduced by a later schema, including
`usage_tool_calls` in schema v13, is reported as not yet applicable rather than
queried. Doctor never migrates, creates, chmods, or otherwise repairs state; its
schema warning gives an explicit current-version state command as the recovery
path. A schema newer than the binary remains an `unknown_schema` error. Raw SQL
errors caused only by a known older schema must never escape from quick or full
doctor output.

Quick and full mode share the exact schema-state matrix: schema 12 reports one
`schema_outdated` schema check with count 12 and recovery command
`agentdeck state migrate`; complete schema 13 reports one `ok` schema check with
count 13; schema 13 without `usage_tool_calls` reports only
`schema_incompatible`; and a future schema reports `unknown_schema` without a
recovery command. Text and JSON never expose raw SQL, SQLite query text, or
driver errors. A successful explicit migration has normal text output and JSON
`migrated: true`, and upgrades both the stored version and required tables.

`--full` additionally performs full SQLite integrity checks and traverses all
indexed sources. Neither mode accesses the network, prints credentials, or
prints session text.
Credential readiness checks enumerate every applicable named credential and
client binding rather than stopping after the first credential.
Quick diagnostics check credential-key existence, exact `0600` permissions,
derived key ID, supported algorithm/key versions, nonce shape, and secret-row
ownership without decrypting values. `doctor --full` additionally authenticates
every credential ciphertext without printing plaintext. Missing key material,
machine mismatch, unsupported format, and AEAD authentication failure report
`credential_key_missing`, `credential_key_machine_mismatch`,
`credential_key_version_unsupported`, or `credential_ciphertext_invalid` and
never trigger automatic key replacement.
Pending `provider.use` checks distinguish external-write transition failure,
selection completion failure, and a prepared journal whose client file
fingerprint proves that the external write already occurred.

There is no generic `doctor --fix` in phase one. An older compatible core
schema reports `schema_outdated` and directs users to explicit `state migrate`;
a current-version database with a missing required table reports
`schema_incompatible` without an invented recovery command. Recovery uses explicit
commands such as `provider recover`, `usage rebuild`, `session rebuild`, or
`extension release`.

## Security and Privacy

- Open Codex and Claude session sources read-only.
- Use parameterized SQL and structured TOML/JSON parsing.
- Validate source identities and configuration fingerprints before mutation.
- Persist credential values only as authenticated ciphertext in SQLite or
  inside the passphrase-encrypted portable backup stream; never persist
  plaintext.
- Keep the machine-bound credential seed private at mode `0600`, never include
  it in portable backups, and never regenerate it while ciphertext exists.
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
stable machine identity and process/config paths are platform adapters.

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
4. Credential plaintext never appears in databases, output, rollback backups,
   logs, or process arguments; SQLite stores only machine-bound authenticated
   ciphertext.
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
23. Usability tests run against temporary homes, synthetic machine identities,
    isolated encrypted credential stores, and real shell processes without
    modifying real user key files or rc files.
24. Make builds report tag-derived version, full commit, branch, actual UTC
    build time, and Go runtime; forced upgrades validate and display both binary
    identities and hashes before replacement.
25. Provider add creates a missing provider or adds a missing named credential
    to an existing provider, while an identical existing credential is a
    no-prompt successful no-op.
26. Credential endpoint, multiplier, and client bindings are credential-owned;
    completed selections snapshot the selected credential's values.
27. Credential references always use `<provider>-<credential>-ref`, never a
    caller-supplied storage name or client component.
28. Codex-bound endpoint input accepts either the base or a final `/v1`, stores
    one canonical base, and writes exactly one `/v1` to Codex configuration.
29. Endpoint validation rejects userinfo, query strings, and fragments before
    provider or credential persistence.
30. Provider definition JSON exposes aggregate clients and credential count;
    credential detail appears only in credential resources and the plural
    provider-status collection.
31. Portable backup includes only credentials owned by current credential rows;
    providers with zero credentials and retained orphan secrets add nothing.
32. Schema v8 transactionally canonicalizes logical references, endpoints, and
    multiplier precision before enforcing logical-reference uniqueness.
33. Every collection-shaped text result uses the shared `+`, `-`, and `|` ASCII
    grid with per-row separators and terminal-display-width alignment, while
    prose empty states, labeled details, JSON, and NDJSON remain unchanged.
34. Text provider status reports credential shorthand in independent `CODEX
    ACTIVE` and `CLAUDE ACTIVE` columns, using `-` for inactive or built-in
    official credentials, while JSON retains the `active` collection.
35. Schema v9 stores one AES-256-GCM ciphertext row per credential and commits
    metadata, rotation, and deletion atomically without external-secret
    compensation.
36. The lazily created `0600` credential key combines a random 256-bit seed with
    stable machine identity through HKDF-SHA256 and is excluded from portable
    backups.
37. Missing, mismatched, permissive, unknown-version, or authentication-failing
    key material fails closed without overwriting ciphertext or exposing secret
    material.
38. Portable restore generates a target-machine key and re-encrypts credentials;
    install and upgrade paths never reset state automatically.
39. Historical pricing fills only absent compatible model/components from the
    current catalog and never replaces a component available at event time.
40. Provider current/status expose credential shorthand and selection time
    without reading or decrypting credential values.
41. Price status/history/list/update/override have dedicated text tables; JSON
    and verbose text retain complete provenance.
42. Usage summary local-calendar shortcuts preserve the all-history default.
43. Usage stats uses the event-time index, one event range scan, and one
    aggregation pass to produce the stable balanced report and JSON contract.
44. Codex usage retains every invocation-level `token_count` event inside a
    logical turn and deduplicates stable copies without using the source path as
    event identity.
45. Schema v11 parser-version invalidation automatically rebuilds legacy usage
    sources while preserving source-atomic rollback and exact byte-range run
    attribution.
46. Schema v12 persists Codex cumulative snapshots per source/session; parser
    rebuilds and mutations restart the baseline, while append scans and process
    restarts continue component-wise deltas without archive-copy duplication.
47. Usage stats preserves token components and nullable cost completeness,
    reports model/session cache-hit semantics, and deterministically lists
    unpriced models and missing components without excluding them from non-cost
    analytics.
48. Schema v13/parser v3 stores only allowlisted source-owned tool metadata,
    deduplicates archives by stable logical call identity, and follows the same
    candidate-only orphan recovery and final-source cleanup as usage events.
49. Text lists all models and at most ten cache sessions while JSON returns all;
    model and session activity detail exposes no arguments, results, command
    text, environment, or reasoning.
