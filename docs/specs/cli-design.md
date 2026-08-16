---
status: active
version: 26
created: 2026-07-14
---

# AgentDeck CLI Design

This document describes what the system does and must keep doing. It is revised
in place as the contract changes and is not superseded by a dated replacement.
Execution state — what is built, in flight, or deferred — lives in
`docs/README.md`, not here.

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
- select, store, or refresh a client account, plan, or OAuth token; probe a
  configured endpoint to verify that a relay or wrapper reaches the upstream its
  provider names or forwards whatever a client's own authentication requires; or
  model a chain of proxies behind the one address a client can be pointed at;
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

Usage reports use the shared width-aware command-layer primitives for section
titles, bars, aligned fields, and responsive rows. Text remains a presentation
contract: layout may change without changing JSON fields or values. Session
token components remain separate fields; at narrow widths secondary fields move
to continuation lines and long identity values are wrapped losslessly. If one
or more events cannot be priced, the complete `catalog_base_cost` and
`provider_cost` remain unavailable; text labels known subtotals, priced/unpriced
event counts, and per-model coverage without presenting a partial amount as a
complete total.

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
├── project-attribution.enabled
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

`project-attribution.enabled` is an empty `0600` machine-local derived-state
marker. Its absence lets managed client functions bypass AgentDeck without
starting a process; its presence means only that a full eligibility check may
be needed. It is not a source of truth and is never included in a portable
backup. After committing a selection, a provider switch makes a best-effort
refresh from current selections. Refresh failure never rolls back or fails the
completed switch and may leave the marker missing or stale. A stale marker can
cause only an unnecessary resolver process, while a missing marker can
temporarily suppress otherwise eligible shell attribution; both are diagnosed
by `shell status` and `doctor`, and a later successful refresh may repair them.

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

A manual GitHub Actions preflight (`.github/workflows/release-preflight.yml`)
accepts an exact pushed commit SHA and an isolated-real-state evidence ID. It
runs aggregate `make release-verify`, builds and verifies commit-bound candidate
artifacts, and uploads evidence without publishing a tag or release. A
successful preflight does not select RC or stable publication.

A GitHub Actions release workflow (`.github/workflows/release.yml`) triggers on
pushed `v*` tags and rejects a tag unless that exact commit SHA has successful
preflight evidence. It reuses same-SHA L4 and isolated-real-state results, then
runs only version-specific `make release-artifact-verify` for embedded identity,
checksums, installation, and distribution before creating the GitHub Release
with the two archives and the checksum file attached. Release
notes follow the repository release-note structure and are finalized at tag
time. `make release-tag TAG=<tag> RELEASE_NOTES=<file>` creates the annotated
tag with `--cleanup=verbatim` and verifies that its message exactly matches the
source file, preserving Markdown headings. In Actions, release-note extraction
force-fetches the remote annotated tag object into a private ref, verifies that
it resolves to `GITHUB_SHA`, and supplies the extracted message through
`gh release create --notes-file`. It does not use `--notes-from-tag`, because
tag-event checkout may leave the public local tag ref pointing at the peeled
commit and make GitHub CLI fall back to that commit message. The release job
needs only `contents: write` permission. A separate minimal CI workflow runs
`make verify` on pushes and pull requests. Pushing tags and publishing releases
remain explicitly authorized manual decisions; the workflow only automates the
mechanics after a tag is pushed.

The Homebrew tap lives in the separate repository `kitdine/homebrew-tap` and
keeps stable and release-candidate channels under distinct formula identities.
`Formula/agentdeck.rb` is the stable channel, installed with
`brew install kitdine/tap/agentdeck`. `Formula/agentdeck-rc.rb` is the opt-in RC
channel, installed with `brew install kitdine/tap/agentdeck-rc`; it accepts only
strict `v<major>.<minor>.<patch>-rc.<number>` tags. Both formulae install the
released binary as `agentdeck` and generate bash, zsh, and fish completions under
the standard Homebrew paths, so they must not be installed together. The RC
formula deliberately does not use `conflicts_with`: under Homebrew's formula
trust model that declaration loads the stable formula even after it has been
uninstalled, while direct installation trusts only the requested RC formula.
Switching channels is therefore an explicit uninstall/install operation.
Homebrew removes only its Cellar and linked artifacts and does not remove
AgentDeck state under `~/.agentdeck/`. Once the RC formula is installed,
`brew update && brew upgrade kitdine/tap/agentdeck-rc` follows later candidates.

#### Version Number Semantics

The tag shape above states syntax; this states what each position promises
during the `0.x` line before any `v1.0.0`. MAJOR stays `0` for the whole line;
raising it is a separate, explicit stability declaration, not a consequence of
any single change.

MINOR (`0.Y.0`) is required whenever a release adds, removes, or renames a
command, subcommand, or flag; migrates the schema of `agentdeck.sqlite3` or
`sessions.sqlite3`; changes stdout text, JSON, NDJSON, or exit-code semantics;
changes a user-visible number — cost, coverage, token attribution, or a count —
for unchanged input; adds, removes, or renames a stable typed error code; ships
persisted data an earlier release cannot read; or rewrites rather than
clarifies a promised behavior in this document.

PATCH (`0.Y.Z`) covers everything else: defect fixes that restore already
promised behavior, performance and robustness work, internal refactors,
documentation accuracy corrections, and improved diagnostic or error message
text that keeps its typed code and exit code. A patch release must be safe to
downgrade from — schema, persisted formats, and the stdout contract stay
byte-compatible with the prior release.

Rewording an error message, including replacing leaked internal text with
actionable guidance, is PATCH; adding or renaming the typed `code` in the JSON
error envelope is MINOR, because "Output and Errors" pins typed error codes
through stable fixtures. This document's own `version:` frontmatter is
independent of the release version: raising it does not oblige a minor
release, and a minor release does not oblige raising it — a patch release may
raise it when it adds or clarifies a rule without changing promised behavior.

Any version that touches persisted data, the pricing read path, or a
configuration file owned by an external client requires a manual technical
preflight bound to the exact pushed commit before any tag. The preflight runs
L4 and cites validation against an isolated copy of real local data. Its result
does not choose a release channel; RC, stable release, or no publication remains
an explicit user decision.

For either a stable or supported RC release, a dependent macOS job renders the
matching formula from the released checksum asset, installs it from an isolated
temporary tap, runs `brew test`, and smoke-loads each generated completion.
Only after those checks pass does it use the repository secret
`HOMEBREW_TAP_TOKEN` to push a formula-specific branch and open a pull request
in `kitdine/homebrew-tap`: stable tags target `Formula/agentdeck.rb` on
`agentdeck-<tag>`, while RC tags target `Formula/agentdeck-rc.rb` on
`agentdeck-rc-<tag>`. The RC pull request does not change the stable formula.
The token must be fine-grained to that repository with Contents and Pull
requests write access. Other prerelease identifiers remain GitHub-only. A
manual `workflow_dispatch` accepts an existing stable or RC tag and runs only
this Homebrew verification/PR job; it does not recreate or edit the GitHub
Release. A tap-installed binary reports the selected tag version, not `dev`,
and each formula's `test` block asserts that contract plus all three completion
paths.

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
aggregate supported clients, authentication mode, an optional wrapper URL, and
client-specific model mappings. Each credential owns its base endpoint, client
bindings, and one non-negative decimal cost multiplier. An absent multiplier
means `1`. Boolean, negative, non-finite, and non-numeric values are invalid.
AgentDeck additionally exposes an immutable built-in provider named `official`,
selectable for Codex and Claude. It is always visible to `provider list|show`,
has no endpoint or credential reference, and never initializes or accesses the
encrypted credential store. Its only stored state is an optional wrapper URL;
it holds no provider definition row, credential, or ciphertext.

Authentication is decided by which provider is selected, not by a separate
setting. A custom provider always writes its selected credential into client
configuration; the built-in provider never writes one and leaves the client on
the login it already owns — Codex's own OpenAI or ChatGPT login, or Claude's
own subscription login. There is no third combination: a custom upstream
without a credential cannot authenticate, and a client's own login is only
meaningful against that client's own vendor.

The multiplier applies only after base price calculation:

```text
provider_cost = catalog_base_cost * multiplier_snapshot
```

Raw tokens and catalog base cost never change when a multiplier changes. Each
successful provider selection and exact run stores its own multiplier snapshot,
so later credential edits do not rewrite historical cost.

### Provider Wrappers

A wrapper is a proxy the user runs in front of one upstream — a local or LAN
compression, logging, or routing layer. It is an optional provider-owned URL,
not an entity of its own, and any provider may carry one, including the built-in
`official`.

The URL is stored once per provider, not per credential and not per client. Per
provider, because a wrapper instance is configured with one upstream address, so
it aligns with the service a provider names rather than with one of that
provider's keys; every credential under that provider reaches the same upstream
through it. Not per client, because one wrapper instance serves both client
protocols on one address, and the existing client-aware `/v1` rule already
resolves both forms from one stored base — Codex appends exactly one `/v1`,
Claude uses the base unchanged. Wrapper URLs are otherwise validated and
always normalized like a Codex-bound credential endpoint, regardless of which
clients the provider actually serves: a trailing `/v1` is stripped
unconditionally, never preserved the way a Claude-only credential's endpoint
would be.

The route is chosen per switch and never stored as an attachment:

```text
agentdeck provider set-wrapper <provider> --url <url> [--kind headroom|plain]
agentdeck provider set-wrapper <provider> --clear
agentdeck provider use <name> [--via]
```

Wrapper kind defaults to `plain`; `--clear` removes both the wrapper URL and its
kind declaration. `provider list`, `provider show`, and `provider status` report
the kind as additive wrapper metadata.

`--via` writes the provider's wrapper URL as the endpoint field; without it the
switch is direct. Both directions are ordinary switches, so inserting or
removing a proxy changes no stored configuration, and a wrapper configured once
can never silently route a later switch that did not ask for it. The route is
recorded in the selection snapshot and reported as part of the effective route,
because once written, `--via` and a direct switch are indistinguishable in the
client file.

A wrapper overrides the endpoint field alone. Provider identity, credential,
multiplier, and usage attribution are unchanged by it, because inserting a layer
in front of an account does not change which account is billed. This holds for
`official` too: subscription usage routed through a proxy stays attributed to
`official` instead of splitting one subscription into a second provider name.

AgentDeck cannot see a wrapper's own upstream configuration and never probes it.
A wrapper must front the upstream its provider names; nothing validates that,
and a wrapper pointed elsewhere misattributes every event it carries. The same
limit applies when a provider's credentials do not share one endpoint: one
wrapper cannot front two upstreams, and AgentDeck does not check for it.

AgentDeck models one override and no chain: a client can be pointed at exactly
one address, and any further hop behind it is configured inside the proxies
themselves.

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

A provider's wrapper URL is not part of this flow; it is set separately with
`provider set-wrapper` so that adding or rotating a credential never changes
routing, and changing routing never touches a secret.

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
agentdeck provider use <name> [--client codex|claude] [--credential <name>] \
  [--via]
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

### Owned Client Configuration Fields

AgentDeck owns exactly two transport fields per client. For Codex it also owns
one project-attribution mapping entry and never writes, clears, or reorders any
other field or header mapping:

| Client | Endpoint field | Credential field | Project-attribution mapping |
| --- | --- | --- | --- |
| Codex | `[model_providers.custom].base_url` | `[model_providers.custom].experimental_bearer_token` | `[model_providers.custom].env_http_headers["X-Headroom-Project"] = "HEADROOM_PROJECT"` |
| Claude | `env.ANTHROPIC_BASE_URL` | `env.ANTHROPIC_AUTH_TOKEN` | None; attribution is launch-environment only |

The Codex mapping stores an environment-variable name, never a project value.
A successful `--via` switch writes it only when the selected wrapper is
declared `headroom`; every other switch removes only
`X-Headroom-Project` from the managed mapping while preserving all unrelated
header mappings.

Two independent rules decide what those two fields receive, symmetrically for
both clients. The selected provider decides the credential field; the route
decides the endpoint field:

```text
credential field = the decrypted secret, for a custom provider
                   else removed, for the built-in provider

endpoint field   = the provider's wrapper URL, with --via
                   else the selected credential's endpoint, for a custom provider
                   else removed, for the built-in provider
```

All four combinations are valid:

| | Direct | `--via` |
| --- | --- | --- |
| Built-in `official` | Both fields removed; the client uses its own login against its own vendor | Wrapper URL written, credential field removed; the client's own login reaches the vendor through the proxy |
| Custom provider | Credential's endpoint and its secret written | Wrapper URL written, the same secret still written; the proxy forwards it to the upstream that issued it |

Switching a custom provider between direct and `--via` changes the endpoint
field alone and leaves the written credential byte-identical.

Removing a field that is already absent is a successful no-op. Codex keeps
`model_provider = "custom"`, sets `[model_providers.custom].name` to the
provider name, and creates the custom table or its `name` field when either is
missing; existing `name` spacing and inline comments are preserved while its
value changes, and all other TOML fields, comments, ordering, and formatting
remain unchanged. Claude keeps its `env` object even when the last owned key is
removed, and every unowned key inside and outside `env` is carried through
unchanged. AgentDeck never reads, checks, writes, backs up, or deletes
`~/.codex/auth.json`, and never reads or writes Claude's stored login
credentials; each client alone owns its authentication.

Claude recognizes credential sources AgentDeck does not own, and any of them
overrides a built-in-provider selection without changing a field AgentDeck may
touch. When a Claude switch selects `official` and `env.ANTHROPIC_API_KEY` or an
`apiKeyHelper` setting is present, AgentDeck completes the switch and reports
the conflicting source on stderr. It never removes a field it does not own to
force its own selection to win.

A Claude client reads `~/.claude/settings.json` while it runs, so a switch that
changes owned keys takes effect in an already-running session without a
restart and can reset that session's negotiated capabilities mid-conversation.
A successful Claude switch therefore reports on stderr that running Claude
sessions should be restarted. The advisory is informational; it does not change
the exit status or the JSON envelope.


AgentDeck changes Codex provider configuration on disk but cannot update
configuration already loaded by a running Codex client. Every successful Codex
switch therefore reports on stderr that the operator should start a new Codex
session or restart the running one to ensure the switch is applied. This
application-boundary advisory does not claim that Codex live-reloads provider
configuration, and it does not copy Claude's live-settings or conflicting-source
language. It follows the same informational, `--quiet`, exit-status, and JSON
envelope rules as the Claude advisories and effective-route line.

### Project Attribution

Project attribution is optional and scoped to an explicitly declared Headroom
wrapper. A client route is eligible only when its latest completed selection
used `--via`, the selected endpoint still equals the provider's current
wrapper URL, and the wrapper kind is still `headroom`. Eligibility is evaluated
per client and is observable through `agentdeck shell status`.

For `agentdeck run`, a completed selection for the requested client is also a
precondition for launching it: without one, the command exits with an error and
does not start the client. Once `agentdeck run` has decided to launch, failure
to derive or inject attribution does not block the child process. Managed shell
functions are fail-open: unreadable state, no completed selection, a direct
selection, a stale wrapper URL, or any other ineligible route omits attribution
and still executes the real client.

The project identity is the same cleaned full working-directory path used by
session indexing. Only its basename reaches the wire, percent-encoded as a URL
path segment; a literal `+` is encoded as `%2B` so path-style and form-style
decoders agree. Empty or nameless directory references produce no value.
AgentDeck never persists the full identity or wire value in its database or in
client configuration.

Eligible launches use one header with client-specific environment transport:

| Client | Launch environment | Request header |
| --- | --- | --- |
| Codex | `HEADROOM_PROJECT=<wire-value>` | `X-Headroom-Project`, through the managed `env_http_headers` mapping |
| Claude | `ANTHROPIC_CUSTOM_HEADERS`, preserving unrelated lines and appending `X-Headroom-Project: <wire-value>` | `X-Headroom-Project` |

A user-supplied value always wins. Codex leaves an existing
`HEADROOM_PROJECT` unchanged. Claude leaves an existing
`X-Headroom-Project` header unchanged, matched case-insensitively, and
preserves every unrelated custom-header line.

#### Resolver and compatibility primitive

`agentdeck shell env <codex|claude>` is the supported resolver used by managed
shell functions. For an eligible route it writes only the final environment
value to stdout. An ineligible route, unreadable state, or unavailable project
value produces empty stdout and exit status `0`; the real client must still be
launched. An unsupported client is a standard invalid-argument error.

The hidden `agentdeck shell-init --project-environment <client>` form remains a
byte-equivalent alias of `agentdeck shell env <client>` while any released
managed block or generated wrapper may call it. It is not a second contract and
must not diverge.

`agentdeck shell-init <bash|fish|zsh>` remains callable but is hidden from
recommended help. It writes sourceable `codex` and `claude` functions to
stdout only: running it does not install, activate, or persist anything. Bash
and zsh may use `eval "$(agentdeck shell-init <shell>)"`; fish uses
`agentdeck shell-init fish | source`. Persistent configuration uses
`agentdeck shell setup`.

This compatibility primitive cannot be removed while any of these dependencies
exist:

- managed blocks call it dynamically so a binary upgrade can change generated
  functions without rewriting startup files;
- generated wrappers and older managed blocks call the hidden resolver alias;
- `v0.2.1-rc.1` exposed manual sourcing through the Homebrew RC and manual, so
  deleting it would break already configured shell startup.

Like `completion`, `shell-init` is outside the GUI JSON data contract because
stdout is a shell program. Argument errors still use the standard error
envelope (`exit 2`, `command: shell-init`, `code: invalid_argument`).

#### Managed shell lifecycle

The reusable lifecycle shape is:

```text
agentdeck shell setup [bash|fish|zsh] [--rc <path>]
agentdeck shell status [bash|fish|zsh] [--rc <path>]
agentdeck shell remove [bash|fish|zsh] [--rc <path>]
```

With no target, every shell in use is covered. A shell is in use when its
default startup file already exists or it is the invoking shell. The invoking
shell is always included and its missing default startup file may be created;
another installed shell is not included merely because its executable exists.
Zsh uses `${ZDOTDIR:-$HOME}/.zshrc`. Fish uses
`${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish`. Bash chooses
`.bash_profile` or `.bashrc` from the invoking shell's login state and also
includes existing default Bash startup files. An explicit shell limits the
operation to that shell, including an absent default file; `--shell` is the
flag equivalent of the positional shell. `--rc` selects a non-default startup
file and therefore requires a single-shell operation.

`setup` atomically installs an AgentDeck-owned managed block. It is idempotent:
an identical block is `unchanged`, and a recognized older block may be upgraded
safely. Duplicate, truncated, edited, or hash-invalid managed regions are not
overwritten. `status` is read-only and reports persistent state as
`absent`, `configured`, `modified`, or `invalid`; current-session JSON
activation state as `active`, `inactive`, or `inherited_from_ancestor`; and
each client's route eligibility with its reason. Text renders the inherited
case as `inactive (marker inherited from ancestor shell)`. An explicit missing
shell still produces one `absent` result.

`remove` deletes only a validated AgentDeck-owned managed block, preserving
every other byte, including command-completion blocks. Absence is an idempotent
success. Edited or invalid managed regions are not automatically removed.
Removal prints a missing-safe current-session deactivation command; executing
it succeeds whether the functions are present or absent.

Explicit multi-target lifecycle operations report every target and continue
after an individual failure. Explicit `shell setup` preserves successful
targets; any failed target makes the overall command fail. The all-or-nothing
rollback guarantee applies only to the automatic switch-time setup described
below, which restores each target's original bytes and existence state without
overwriting a concurrent replacement. Package installation and uninstallation
never invoke this lifecycle implicitly.

Managed blocks guard AgentDeck's presence. Bash and zsh use
`command -v agentdeck >/dev/null 2>&1 && ...`; fish uses
`if type -q agentdeck`. If the binary is absent from `PATH`, the block is
silent and inert and the real clients remain usable, including after package
uninstall. Only binary absence is suppressed: when AgentDeck is found, a
generation or source failure remains visible.

#### Switch-time setup and advisories

After a successful `provider use --via` leaves at least one client eligible,
the command may write startup files only when all of these are true: output is
text, `--quiet` is not set, stderr is a TTY, `--no-shell-setup` is not set,
the user has not persistently declined, and no valid managed block is installed.
It then runs the same all-or-nothing setup over every shell in use. JSON,
NDJSON, non-TTY, quiet, and explicitly suppressed invocations never write
startup files. `shell remove` records the persistent decline; an explicit
`shell setup` clears it. Automatic setup failure never rolls back the completed
provider switch and degrades only to the unconfigured advisory.

Attribution advisories are informational stderr only, are suppressed by
`--quiet`, never enter JSON output, and never change exit status:

- switching into an eligible route with configured integration says attribution
  is in effect, new shells are persistently covered, and gives the current-shell
  activation command;
- switching into an eligible route without configured integration suggests one
  `agentdeck shell setup`;
- switching out of an eligible route with configured integration says the
  functions remain installed but immediately stop injecting;
- other route/configuration combinations emit no attribution advisory.

`provider set-wrapper` guidance describes the prerequisites—an eligible route,
the derived marker, and configured shell integration—and never claims that
attribution is already active merely because wrapper metadata was changed.

Managed integration can attribute directly launched CLI clients. AgentDeck
does not attribute GUI app launches and makes no claim that a GUI app reads
project-scoped settings. A user may independently maintain project-scoped
settings such as `.claude/settings.local.json`; AgentDeck documents that recipe
but never creates or modifies the file.

#### Negative gate and invocation cost

Managed functions first test
`<state-root>/project-attribution.enabled`. When it is absent they execute the
real client without starting AgentDeck. When present they start one AgentDeck
resolver process and perform one read-only database open per client invocation;
the resolver still performs the complete per-client eligibility check. Exact
benchmark results and host method belong in the archived implementation plan,
not this living contract.
### Selecting the Built-in Provider

Selecting `official` returns the client to its own login state: Codex's
existing OpenAI or ChatGPT login, or Claude's existing subscription login. It
accepts `--client codex` and `--client claude`, takes no `--credential`, and
always removes the owned credential field. It removes the owned endpoint field
too on a direct switch; with `--via` the endpoint field carries the built-in
provider's own wrapper URL, which is how a client reaches its vendor under its
own login through a user-run proxy. The built-in provider does not create a
credential reference, encrypted secret row, or credential key file, and its
wrapper URL is the only state stored for it. It does create the same
operation-linked immutable selection snapshot as custom providers, containing
`official`, the selected client, no credential, the route taken, and multiplier
`1`. Historical attribution treats the completed
transaction time as the switch boundary.

Selecting `official` is not account or subscription management. AgentDeck
returns the client to whatever login that client already holds and never
enumerates, selects, stores, or refreshes an account, plan, or OAuth token.

Deleting a custom provider is allowed after use. The live definition and all
credential metadata and ciphertext are removed in one SQLite transaction,
while selection snapshots retain historical name, endpoint, credential name,
client, and multiplier attribution. Provider and credential deletion no longer
create external-secret recovery entries.

`provider list` and `provider show` read provider definitions without accessing
or decrypting credential ciphertext. Because endpoint and multiplier are
credential-owned, `provider list` shows provider name, type, aggregate clients,
credential count, and the provider's optional wrapper URL rather than a single
endpoint or multiplier. Credential
readiness belongs to `provider status` and top-level `credential` commands and
checks secret-row presence without decrypting values. Text `credential list`
contains `PROVIDER`, `NAME`, `REFERENCE`, `ENDPOINT`, `MULTIPLIER`, `CLIENTS`,
and `READY`; credential detail and JSON expose the same non-secret metadata. A
wrapper URL is provider-owned, so it appears in `provider list|show` and an
additive `wrapper_url` JSON field rather than on any credential. Output
never reports credential values or private compatibility references. Provider
definition JSON contains aggregate `clients` and `credential_count`, but no
endpoint, multiplier, credential reference, or nested credential details.
Provider status exposes credential detail only through the plural `credentials`
collection and has no deprecated singular `credential` projection.

Collection-shaped `text` results that use the shared ASCII grid renderer include
provider list/status/recovery collections, credential lists, session
list/search/document collections, extension lists, backup lists, and price
history. The usage report family (`usage summary`, `usage stats`, `usage
sessions`, and `usage diagnose`) is explicitly excluded and follows the
dedicated responsive section, bar, and continuation-line contract below. The
ASCII grid renderer uses only `+`, `-`, and `|`, adds one space of horizontal
cell padding, and draws a horizontal separator around the header and every data
row. It does not use Unicode box-drawing characters. Empty collections keep
their existing concise prose instead of rendering an empty grid, and detail
views keep their labeled-field layout.

For commands using the ASCII grid, column widths are calculated from terminal
display width rather than byte length so CJK and other wide text remain aligned.
Cells are left-aligned and are not truncated or wrapped. The implementation
uses one small width-focused dependency rather than a full table UI framework,
vendors it through the normal dependency workflow, and must continue to pass
the release size gate. JSON and NDJSON contracts are unchanged.

Text `provider status` uses the columns `CODEX ACTIVE` and `CLAUDE ACTIVE`.
Each active cell contains the selected credential shorthand; inactive cells and
the built-in `official` credential display `-`. Detail status includes one row
per client with active state, shorthand, and selection time. The additive JSON
`active[].selected_at` field retains the selection timestamp.

`provider current` returns the latest completed selection for each client as
`client`, `provider`, optional credential shorthand, whether the route went
through the provider's wrapper, the endpoint actually written, and
`selected_at`. `official` has no credential. Reporting the route and the written
endpoint is what keeps a wrapped selection distinguishable from a direct one
after the fact. Current/status reporting reads only selection and
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

Aggregate cache creation without a TTL breakdown remains visible. When
`cache_creation_tokens > 0` and both reported TTL buckets are zero, pricing
uses the documented five-minute cache-write default and marks the resulting
cost as `defaulted 5m cache creation TTL`; the stored token fields remain
unchanged. A non-zero reported breakdown that does not cover the total remains
partially unpriced, and the scanner never redistributes that remainder.

Claude Code's own internal auxiliary model calls — for example automatic
session-title generation, recorded in the project JSONL only as a bare
`"type":"ai-title"` entry — carry no `usage` object at all. These calls are
invisible to this scanner and never appear in `agentdeck usage` output; their
cost is visible only in Claude Code's own in-process `/status` display. This
is a completeness gap in the upstream transcript file, not an importer
defect, and no local parsing change can recover it.

Scanning is incremental per source: a stable per-file cursor plus append
verification means unchanged content is never re-read. A full re-read happens
only when a source is mutated, when a rebuild is forced, or when the stored
parser version differs from the current one — the last of which makes the first
scan after a parser-version release cost as much as a first-ever scan.

Because that full re-read is expensive on real histories, long scans report
progress on standard error. Progress never goes to standard output, so JSON and
NDJSON output stays machine-parseable. Nothing is emitted for the first second,
leaving the common fast path silent; after that the scan reports processed and
total source files. `--quiet` suppresses progress entirely, and output that is
not an interactive terminal carries no ANSI escapes. Progress covers the
implicit scan performed by `usage stats` and `usage summary` as well as
explicit `usage scan` and `usage rebuild`. When a scan is triggered by a parser
version change rather than by new data, the progress output says so, because an
unexplained multi-minute wait after an upgrade is indistinguishable from a hang.

`usage stats` and `usage summary` scan synchronously before reporting so a
report always reflects current sources; `--no-scan` returns the stored
aggregate immediately for callers that prefer staleness to waiting. The scan is
never moved to the background, which would make report contents depend on a
race.

Attribution has three explicit qualities:

- `exact`: `agentdeck run` owns an unambiguous client process lifetime and
  binds only source ranges written during that run.
- `estimated`: an observed lifecycle boundary, or failing that a file-only
  fallback, assigns events to the provider that the boundary — or the session's
  first timestamp — names.
- `historical`: no earlier provider selection exists; multiplier is fixed at
  `1`.

Resolution for a single event is ordered and total. An exact run binding wins.
Otherwise the most recent recorded lifecycle boundary at or before the event's
timestamp applies. Otherwise the file-only fallback attributes the complete
logical session to the provider selected at its first timestamp. A logical
session is therefore split at observed boundaries rather than attributed
wholesale, and re-attributing a resumed session no longer requires
`agentdeck run`: a client with runtime attribution Hooks configured splits its
own sessions, while `run` remains a supported low-level launcher for exact
attribution. Earlier ranges keep their prior provider. An old exact run never
captures later unwrapped events merely because the logical session ID matches.

Overlapping or ambiguous client lifetimes are downgraded to `estimated`; the
tool never silently claims exact attribution. A second managed run for a client
that already has one is accepted rather than refused: both overlapping runs are
downgraded, because losing exactness is a smaller harm than preventing a client
from launching.
The wrapper waits for every started child and propagates a failed client exit
as a runtime failure in text and JSON modes. If wrapper bookkeeping or scanning
cannot prove the source range, it closes the run as `estimated` rather than
leaving an incomplete or falsely exact run.

Client lifecycle Hooks are the supported source of those boundaries.
`agentdeck usage hook setup|status|remove [--client codex|claude|all]` follows
the same lifecycle contract as `agentdeck shell setup|status|remove`: setup
merges only AgentDeck-owned entries and is idempotent, status reports `absent`,
`configured`, `modified`, or `invalid` without changing configuration, and
remove deletes only entries still matching the owned form, never rewriting an
edited or unverifiable region. No package installation path writes Hook
configuration. The two lifecycles deliberately share no implementation: Hooks
merge JSON — Codex `hooks.json`, Claude `settings.json` — while shell
integration owns a text block.

An owned entry is a command entry invoking
`agentdeck usage hook event <codex|claude>`. AgentDeck registers `SessionStart`
for Codex, and `SessionStart`, `ConfigChange`, and `SessionEnd` for Claude.
`configured` requires exactly one owned entry for every registered event; a
duplicate, a partial set, or an altered entry reports `modified`, and hook
configuration that cannot be decoded reports `invalid`. Writing Hook
configuration preserves unrelated hooks and unrelated keys, so a provider
switch and a Hook lifecycle command may touch `~/.claude/settings.json` in
either order without loss.

Codex may still require the user to trust a newly written Hook through its own
`/hooks` approval. AgentDeck reports that limitation and never modifies or
circumvents client trust state.

The handler `agentdeck usage hook event <codex|claude>` is hidden and accepts
only known, bounded lifecycle payloads. It writes nothing to standard output
and fails open at every step: oversized, malformed, or unrecognized input,
unavailable state, and any recording failure all end in success without output,
because no attribution outcome may delay or prevent a client from starting,
resuming, reloading settings, or exiting. When Hooks are absent, untrusted,
disabled, or failing, attribution falls back to the file-only estimated
behavior above; a client is never reported as unattributed merely because its
Hooks never ran.

A recorded boundary carries the logical session, the observed instant, and the
provider, multiplier, and wrapper state of the completed selection in effect,
together with the event that produced it. It stores no client payload beyond
the validated event shape. Boundaries are always `estimated`: a Hook never
claims exact attribution. A boundary equal to that session's most recent one is
not recorded twice, so duplicate delivery is harmless.

Two events establish boundaries. `SessionStart` does so for both clients, but
only when its transcript validates and a completed provider selection exists; a
`compact` source establishes none, because compaction continues the same
routing. For Claude, `ConfigChange` also does, and only for a user-settings
change to the managed settings file: AgentDeck reconciles that file against the
current selection and records either the matching provider, or provider
`unknown` when the two disagree or no selection exists, rather than guessing
which side is stale. A `ConfigChange` boundary is therefore possible where
`SessionStart` would record nothing. `SessionEnd` is registered but records no
boundary.

Schema v17 creates `usage_session_routes` for those boundaries and drops the
single-active-run index, which is what makes an overlapping managed run a
downgrade rather than a refusal.

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
event uses the most recent lifecycle boundary at or before it, or the
provider-timeline snapshot at its session start when no boundary applies; an
event whose session predates every recorded provider selection is grouped as
`unknown`.
`unknown` is an explicit unattributed bucket, never silently mapped to
`official`, and `--provider unknown` selects exactly those events. Each
selection snapshot also records whether the route went through the provider's
wrapper. That route is reported metadata and never a grouping key: a wrapper
carries no billing relationship, so events routed through one stay under their
provider's name and `--provider <name>` selects them whether the route was
wrapped or direct. Subscription traffic routed through a proxy therefore stays
under `official` at multiplier `1` rather than appearing as a separate
provider. Provider
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

Text always uses the approved responsive balanced layout and the shared
command-layer primitives. `MODELS`, `CLIENTS`, and `PROVIDERS` use share bars
with a fixed 100% baseline; `TREND` uses magnitude bars whose full scale is the
named peak of that series. Share is printed once, while tokens, cost, pricing
status, and sessions align in detail columns. `CLIENTS` exposes the same detail
depth as the other dimensions. Cache is a structured model section followed by
a capped subordinate session list; copyable detail commands render in a
separate full-width block, and the complete
session identifier remains available without making the primary row unbounded.
KPI values, including `AVG COST / SESSION`, `PEAK`, and `PRICED`, are stated in
the header region once. The renderer uses up to 260 visible cells and permits
at most one column below 120 cells, two at 120–179, three at 180–239, and four
at 240 or more. It assigns whole `TREND`, `MODELS`, `CLIENTS`, `PROVIDERS`,
`CACHE`, and `COVERAGE` panels to columns and falls back when the shortest
column would be below 60% of the tallest or an added column would reduce
maximum height by less than 15%. Target panel width is approximately 56–80
cells. KPI cards use 2×3 below 120 cells, 3×2 at 120–179, and 6×1 at 180 or
more. Three or more consecutive zero-valued Trend buckets collapse into one
range row in text only. Heatmaps, model activity, detail commands, and warnings
remain full-width. Narrow terminals keep identity and primary values and move
secondary fields to lossless continuation lines without exceeding detected
width. JSON shape and values never depend on this layout. Ranges spanning at
least seven local
calendar days include a full-width 7-by-24 activity heatmap; hour ranges omit
it. TTY color is optional and `--no-color` or redirected output contains no ANSI
escapes. `timezone` is a stable IANA identifier when the machine zone can be
resolved and otherwise an explicit `UTC+HH:MM` offset. Hour bucket boundaries
retain their RFC3339 offsets so both hours in a DST fold remain distinct.

`usage stats --interactive` is explicit and read-only. It requires text output,
TTY stdin/stdout, a usable `TERM`, and at least 48x10; this preflight happens
before opening the store, scanning, or entering raw mode. The viewer exposes
Overview, Trend, Models, Clients, Providers, Cache, and Coverage, with
section-local page, selection, and viewport state and 20-row pages. Left/Right
or Tab changes section, Up/Down/Home/End changes selection, PageUp/PageDown
changes page, `?` toggles help, and `q` or standalone Escape exits. Resize,
Ctrl-C, EOF, cancellation, and errors restore the terminal before reporting.
`--top` is applied before interactive paging; JSON is rejected before state
creation and ordinary `usage stats` is the fallback for unsupported terminals.

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

Time-limited introductory pricing (a lower rate that reverts to a higher
standard rate on a published future date) is imported as whatever component
LiteLLM currently reports and used as-is; the catalog does not track or warn
about a scheduled future rate change. A first-party surface such as Claude
Code's own `/status` panel computes its own estimate from its own local price
table and can disagree with `agentdeck usage`/`agentdeck price list` even
when both are nominally using "current" Anthropic pricing — for example,
`/status` was observed applying Claude Sonnet 5's post-introductory standard
rate before that rate took effect, overstating session cost by roughly 30%
relative to the LiteLLM-sourced introductory rate this catalog held for the
same period. Treat catalog-derived costs as the LiteLLM-sourced reference,
not as a guaranteed match for any other tool's on-screen estimate.

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

### Bundled Catalog

A `bundled` catalog compiled into the binary prices usage on a fresh install
before any network call. It is the lowest-precedence layer: any `litellm` or
`official` catalog with a later `catalog_effective` outranks it, so shipping a
bundled price can never suppress fresher upstream data.

The bundled catalog is a **generated, reviewed build artifact and is never
hand-edited**. `tools/genprices` (invoked as `make prices-regen`, optionally
`LITELLM_COMMIT=<sha>`) rebuilds it by downloading a LiteLLM snapshot pinned to
an explicit commit SHA, applying the same direct-provider filter and component
mapping the network importer uses, merging the curated gap-fill described
below, and writing the result. Generation is a pure function of its inputs: the
artifact records the pinned commit rather than a wall-clock retrieval time, so
`make check-prices-reproducible` can prove the committed file is exactly what
its recorded inputs produce and a hand-edit cannot pass unnoticed. Regeneration
is a release-time step owned by the release preparer; ordinary builds, tests,
and commands never run it and never reach the network for it.

Every generated bundled model carries a stable effective date rather than the
generation time. The bundled layer is the period-agnostic fallback of last
resort, and dating it at build time would leave all pre-build usage unpriced on
a fresh install, which is the gap the layer exists to close.

**The bundled catalog's own effective date is that same stable date and is
never derived from the models it carries.** Catalogs in one layer are ranked by
effective date, and installed catalogs are outranked rather than deleted, so a
catalog date that moved with its contents would let one early-dated model hand
every shared model back to a previously installed bundled catalog on upgrade.
Model rows keep their own effective dates, which gate history; the catalog date
is a precedence key. The current clock still caps it, so a catalog is never
recorded as effective in the future.

**The bundled `catalog_version` carries a digest of the catalog's own
contents.** Catalog import is keyed on the version string, so reusing a version
after changing prices would silently retain stale prices and a stale
`content_sha256` on every already-installed copy. The digest covers the
semantic catalog (models, providers, prices, aliases, effective dates) and
ignores both formatting and the version field itself; a content change without
a regenerated version fails a shipped guard rather than reaching users.

### Curated Gap-Fill

Some models are never priced upstream — LiteLLM's `chatgpt/` namespace, for
example, lists subscription-surface entries carrying no cost fields at all, so
widening the direct-provider filter to admit them would import models with no
prices and change nothing. A separate curated gap-fill input therefore carries
vendor prices for such models and is merged over the generated catalog, so
regenerating from upstream cannot drop an entry upstream has no priced row for.

Curated entries join the **`bundled`** layer, never `official`. A fresher
`litellm` catalog outranks `bundled`, so if upstream later starts pricing the
model, upstream wins automatically and the curated row stops mattering; an
`official` entry would outrank upstream indefinitely and freeze a stale price.

The curation bar is enforced, not advisory. A normal `vendor_rate` entry
requires a real `https` vendor rate-card `source_url`, a confirmed rate, a
named human verifier with the date checked, a parseable effective date, and at
least one valid decimal component. A model that is absent, unreleased, or
cannot be identified confidently stays pending and unpriced; agreement among
aggregators that all derive from one another is not confirmation.

A narrower `equivalent_estimate` entry is allowed for a real, released
subscription-only model that has no separately published API rate. It must name
the vendor-priced predecessor or basis model in `basis_model`, carry an
explicit `pricing_note`, and use that basis model's vendor rate rather than
inventing an unrelated figure. The estimate is an equivalent token-cost value,
not the user's subscription invoice. For an estimate, `verified_by` and
`verified_on` record who approved the estimate and its basis and when; unlike
a `vendor_rate` entry, which attests to a rate a named person read off a vendor
rate card, this may be a project role, because what is being attested is a
disclosed derivation rather than an observed vendor figure.

Estimated metadata does not change `usage stats`, summaries, sessions, or their
JSON contracts. It is exposed only by `price list` (including an exact-model
filter): text marks the effective price `ESTIMATED` and names its basis, while
price-list JSON returns the machine-readable price kind, basis model, and note.
The marker applies only while at least one effective component comes from the
estimated bundled entry. If a fresher `litellm` or `official` catalog supplies
all components, normal provenance wins and the estimate marker disappears.
This metadata is derived from the compiled gap-fill plus effective component
provenance and does not require a database migration.

**The disclosure therefore travels with the binary, not with the database.**
Price rows live in the core database and move with a portable backup; the
estimate marker, basis, and note exist only in the running binary's compiled
gap-fill. Reading such a database with a binary whose gap-fill does not carry
that entry — an older release, or a later one that retired the entry because
upstream began publishing a rate — renders the price as an ordinary price with
no marker and no note. This is the accepted cost of deriving the metadata
instead of storing it, not a defect; storing it would require the migration
this contract deliberately avoids.

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

### Session Experience Contract

Each `session search` document carries its own normalized UTC `event_at`.
Search orders parseable instants newest first, then keeps missing or invalid
instants in stable source order; text renders parseable instants in the
process display zone and renders absent or invalid values as `—`.

`session scan` and `session rebuild` may report aggregate progress on stderr:
processed and total source-file counts, approved-document count, and skipped
source count. Progress never names a source or includes source content, stays
off stdout so JSON remains machine-readable, and `--quiet` suppresses it.

Non-interactive `session show` is a bounded, sectioned read view. It always
renders metadata and approved `DOCUMENTS`; `--activity` adds only the existing
safe activity aggregate and detail collection, and `--tokens` adds complete
normalized `usage` plus chronologically ordered invocation rows with their
token components, pricing completeness, warnings, and invocation pagination.
Explicit `--page`/`--limit` pagination is represented by the named collection
in JSON (`documents`, `activity`, or `invocations`); without explicit paging,
JSON retains the complete collection for compatibility.

`session show --interactive` is a read-only terminal view with independent
OVERVIEW, DOCUMENTS, ACTIVITY, and TOKENS pages. It requires text output and
TTY stdin/stdout, and is mutually exclusive with `--activity`, `--tokens`,
`--page`, `--limit`, and `--all`. It is a human interface, not a JSON or
desktop-wire surface.

`session --interactive` is the explicit indexed-session browser. It performs no
implicit scan, never displays source paths, and shows a copyable
`agentdeck session scan` hint when the index is empty. Up/Down/Home/End select,
PageUp/PageDown page, Enter opens the existing detail viewer, Escape returns
from detail to the list, Escape on the list exits, and `q` exits from either
level. The state lock is released before either interactive entry waits for
user input.

The v0.4.0 desktop-facing session DTO boundary is the normal versioned JSON
envelope and its `data`: session metadata and approved documents; optional
safe activity summary/detail; optional complete usage summary and normalized
invocations; named pagination; and envelope `warnings`/`partial` semantics.
Usage pricing and attribution warnings belong to `data.usage.warnings`; each
normalized invocation carries its own `data.invocations[].warnings`. Top-level
envelope `warnings` and `partial` remain command-level state and do not replace
either usage-level warning collection. Timestamps are UTC RFC 3339 strings,
token values are integers, and monetary values are decimal strings or
unavailable. A v0.5.0 desktop client can consume these bounded DTOs as its
session dependency, but its later wire-contract task must still define the
coherent desktop snapshot, wire version, and Go-owned redaction; it must not
parse text or interactive rendering.

### Desktop Snapshot Contract

`agentdeck --format json desktop snapshot --wire-version 1 --recent-limit N`
is the v0.5.0 desktop refresh boundary. `N` defaults to `5` and must be from
`1` through `20`. The command accepts no positional arguments and rejects text
or NDJSON output. Unsupported wire versions return typed code
`unsupported_wire_version`; an out-of-range recent limit returns
`invalid_recent_limit`. Both are input errors with exit status `2` and the
normal JSON error envelope on stderr.

A successful invocation exits `0` and returns the normal output envelope with
`command: "desktop.snapshot"`. `data.wire_version` is independently versioned
from the top-level automation `schema_version`. The v1 data object contains one
coherent refresh result with:

- `generated_at` and `next_refresh_at` UTC RFC 3339 timestamps; v1 suggests a
  five-minute refresh interval but does not schedule or run a background task;
- `provider.available` and privacy-bounded routes containing only client,
  provider name, selected time, and whether the selected route used a wrapper;
- `usage.available`, the current local-day half-open range, bounded token/count
  totals, nullable complete and known costs, explicit `pricing_complete`, an
  unpriced-component count, and usage-level warnings;
- `sessions.available`, total indexed sessions, and at most `recent-limit`
  stable recent rows containing client, session ID, project basename, model,
  and first/last times;
- `health.available`, aggregate doctor status/counts, and safe check name,
  status, code, count, and recovery command.

Every section always exists and owns an `available` flag. Failure to read one
local section does not discard the other sections: the command still exits
`0`, sets envelope `partial: true`, and adds a stable warning code from
`provider_unavailable`, `usage_unavailable`, `sessions_unavailable`, or
`health_unavailable`. A failed read-only database close adds
`state_close_failed` or `sessions_close_failed` without discarding the already
decoded section. Empty collections encode as `[]` or `{}`, never `null`.
The command performs no implicit usage/session scan, does not create a missing
state root or database, applies no migration, changes no permission, changes
no committed SQLite contents, and performs no network request. Reading an
existing WAL-mode core or session database may materialize owner-only `-wal`
and `-shm` sidecars inside the existing state root so the snapshot can observe
concurrent committed changes correctly.

Go constructs the snapshot through dedicated DTOs; it never serializes domain
objects directly. The contract excludes credentials and references, provider
or wrapper endpoints and headers, configuration contents, source paths, raw
session content, prompts, responses, tool arguments/results, machine identity,
and full project paths. Canonical complete and partial v1 envelopes live under
`desktop/fixtures/v1`; Go contract tests consume them, and the macOS foundation
must reuse those same files for Swift `Codable` decoding rather than copying
fixture values.

Update discovery is separate from `desktop snapshot`. The desktop host may
perform an automatic check only after explicit opt-in and at most once per 24
hours; a user-initiated manual check is also allowed. It may issue an
unauthenticated request only to the official AgentDeck GitHub latest stable
release API and may send no AgentDeck state, usage, provider, session, machine
identifier, credential, or custom tracking header. It may compare compatible
stable versions and offer to open the official release page. Network, HTTP,
decoding, or browser-open failures are non-fatal and never reduce local desktop
snapshot availability. The app never downloads, installs, replaces, relaunches,
or requests privilege as part of this check.

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

Portable backups also exclude the machine-local derived
`project-attribution.enabled` negative-gate marker. Its absence immediately
after restore is valid. A later provider switch attempts the same best-effort
refresh; only a successful refresh reconstructs it from current selection
state, and refresh failure does not fail the completed switch.

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

### Time Representation

Instants cross one boundary and only one: they are stored and transported in
UTC, and rendered in the machine's zone only when the audience is a person.

Storage is UTC RFC 3339 with nanoseconds, normalized at the boundary where a
value enters AgentDeck rather than wherever it is later read — a client log
timestamp is parsed and converted on ingest, and every generated timestamp is
taken in UTC. Range queries compare UTC-formatted arguments against those
stored values, so no comparison depends on the offset a source happened to
write. Schema v10 already migrated `usage_events.event_at` and recomputed
session bounds under this rule; it holds for every table, not only that one.

JSON and NDJSON keep the stored UTC instant unchanged. They are the automation
and GUI contract, and an envelope whose timestamps shifted with the host that
produced it could not be compared, cached, or replayed across machines.

Human-readable text renders instants in the machine's zone, to the second;
sub-second precision is retained in JSON and dropped in text, where it is never
actionable. Every text output that shows an instant names the zone it used:
grid columns carry it in the header cell, detail fields carry it after the
value, and usage reports keep the zone name they already print in their header.
An output that shows no instant gains nothing. A value that cannot be parsed as
an instant is rendered unchanged rather than failing the command, because a
read command must not fail over presentation.

The zone is the machine's, resolved once per process, with no per-invocation
override; `TZ` selects it the way it selects any program's local zone. Command
*inputs* are unaffected: `usage stats --from/--to` continue to name local dates,
which is what makes "yesterday" mean the user's yesterday.

This is a presentation contract. It changes no stored value, requires no
migration, and leaves every JSON field byte-identical to what the same state
produced before.

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
- Permit network access only for an explicit price update or an opt-in desktop
  stable-release update check. Both are user-initiated or explicitly enabled,
  send no local state, and are off by default in the desktop app.
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
33. Collection-shaped text results outside the usage report family use the
    shared `+`, `-`, and `|` ASCII grid with per-row separators and
    terminal-display-width alignment. Usage reports use their dedicated
    responsive primitives; prose empty states, labeled details, JSON, and
    NDJSON remain unchanged.
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
50. Project attribution is emitted only for a current completed `--via`
    selection whose endpoint still matches a wrapper declared `headroom`.
    Outside `agentdeck run`, attribution requires a user-installed shell helper
    or user-written settings; GUI app launches are not attributed by AgentDeck.

51. `shell env` is the supported fail-open resolver, while hidden `shell-init`
    remains callable and byte-compatible for released startup-file consumers.
52. Shell setup, status, and remove share one in-use-shell, idempotence,
    ownership, conflict, and multi-target lifecycle contract; package
    installation and uninstallation do not run it.
53. Interactive text `provider use --via` may configure startup files only
    under the documented opt-in conditions and never rolls back a completed
    provider switch when shell setup fails.
54. The project-attribution marker is a `0600` machine-local negative gate,
    not eligibility truth, and is excluded from portable backups.
55. A release position change is MINOR when it adds, removes, or renames a
    command, subcommand, or flag; migrates the
    `agentdeck.sqlite3` or `sessions.sqlite3` schema; changes stdout text,
    JSON, NDJSON, or exit-code semantics; changes a user-visible number for
    unchanged input; adds, removes, or renames a stable typed error code;
    ships persisted data an earlier release cannot read; or
    rewrites rather than clarifies a promised behavior in this document.
    PATCH covers everything else, including reworded error-message text that
    keeps its typed code and exit code, and must remain safe to downgrade
    from: schema, persisted formats, and the stdout contract stay
    byte-compatible. This document's own `version:` is independent of the
    release version; either may advance without the other.

**Credential key compatibility.** The key-file format remains version 1. Sealed
credential rows support versions 1 and 2: version 1 compares its legacy
`hex(sha256(key)[:16])` ID, while version 2 derives its persisted 16-byte ID
from bytes 32..48 of the unchanged HKDF stream whose bytes 0..32 remain the
AES key. New seals and ordinary rewrites use version 2; existing version-1
ciphertext remains readable and is never rewritten automatically. Unsupported
sealed versions fail closed as `credential_key_version_unsupported`, and a
version-2 row is unreadable by `v0.2.x` until rewritten by a compatible build.

## Changelog

Every entry is a change to the contract this document defines, not a record of
implementation work. Add a row and raise the version whenever behavior promised
here changes; do not create a dated copy of this file.

| Version | Date | Contract change |
| --- | --- | --- |
| 26 | 2026-08-16 | Records the `v0.4.1` patch, which shipped on 2026-08-13 without a row. Codex `cache_write_input_tokens` is captured into a `cache_write_tokens` column and already-indexed Codex sources are re-scanned on upgrade, so Codex cache-write token volumes that previously reported zero now report their real values and any total derived from them changes for existing data. The cache-write semantics themselves are unchanged: a cache write remains a token volume rather than a second hit-rate percentage, and pricing still uses the documented five-minute cache-write default. |
| 25 | 2026-08-13 | Adds the v0.5.0 `desktop snapshot` wire v1 contract: JSON-only request flags, coherent Go-owned provider/usage/session/health response, privacy redaction, per-section availability, partial warning semantics, stable input error codes, read-only/no-network behavior, shared Go/Swift canonical fixtures, and opt-in privacy-bounded stable-release update-check connectivity. |
| 24 | 2026-08-10 | Establishes the v0.4.0 version contract across the completed session-experience and usage-report-presentation lines: session search/show gains bounded, additive document, activity, usage, invocation, pagination, and interactive-viewer surfaces; usage reports use responsive text presentation without changing JSON values, pricing, or attribution; the session parser/index format change requires rebuildable-index migration and an exact-commit technical preflight citing isolated-real-state validation before the user selects RC, stable release, or no publication; invocation-level pricing reads event-time prices without rewriting stored usage or historical price rows. The bounded session DTO is the v0.5.0 desktop dependency and unblocks `desktop-wire-contract`, which still owns the later coherent snapshot, wire version, and Go-owned redaction contract. |
| 23 | 2026-08-04 | Adds managed `usage hook setup|status|remove` lifecycle commands for Codex and Claude session-route boundaries: setup/removal touch only AgentDeck-owned JSON entries, status exposes absent/configured/modified/invalid and observable trust limits, and handlers are silent, bounded, fail-open. Attribution resolves in a fixed order — exact run binding, then the most recent lifecycle boundary at or before the event, then the session-start fallback — so a Hook-configured client splits its own resumed sessions and `run` is no longer required to re-attribute one; `run` remains a supported low-level exact-attribution launcher, and an overlapping managed run downgrades both runs to estimated rather than being refused. Schema v17 adds `usage_session_routes` and drops the single-active-run index. Sealed credential key version 2 derives its key ID without changing existing AES key bytes, retains version-1 reads, and makes new ciphertext unsafe to downgrade to `v0.2.x`. Claude cache-creation totals with positive total and two zero TTL buckets now use the disclosed default five-minute rate; affected historical cost/coverage numbers change without rewriting stored events. `codex-auto-review` billing remains unresolved and stays an unpriced Backlog item rather than receiving an inferred price or mapping. |
| 22 | 2026-08-02 | Defines version number semantics for the `0.x` line: MAJOR stays `0` until an explicit stability declaration; MINOR triggers cover command/flag/typed-error-code changes, schema migration, stdout/JSON/NDJSON/exit-code semantic changes, user-visible number changes for unchanged input, unsafe-to-downgrade persisted formats, and rewritten (not merely clarified) promised behavior; PATCH covers everything else and must stay safe to downgrade from; error-message wording is PATCH while typed error codes are MINOR; this document's own `version:` is independent of the release version; releases touching persisted data, the pricing read path, or external-client configuration ship at least one `-rc.N` validated against real local data first. |
| 21 | 2026-07-30 | Shell attribution gains the supported `shell env` resolver, hidden `shell-init` compatibility guarantees, presence-guarded managed blocks, reusable setup/status/remove lifecycle, in-use-shell targeting, interactive switch-time setup, corrected route-change advisories, and the machine-local portable-backup-excluded negative gate. |
| 20 | 2026-07-29 | Project attribution is opt-in and Headroom-wrapper-scoped. Eligible launches derive the cleaned full-path identity used by session indexing, expose only its safely encoded basename, and use client-specific environment transport without persisting the value. Codex owns only the `X-Headroom-Project` to `HEADROOM_PROJECT` mapping while preserving unrelated mappings. Outside `agentdeck run`, users must install the emitted shell helper or write settings themselves; AgentDeck does not attribute GUI app launches, and wrappers not declared `headroom` never receive AgentDeck-generated attribution headers. |
| 19 | 2026-07-28 | Every successful Codex provider switch reports a stderr advisory to start a new session or restart the running one, because AgentDeck updates the configuration file but cannot update configuration already loaded by a running client. The note deliberately differs from Claude's live-settings and conflicting-credential advisories, remains informational, and is suppressed by `--quiet`. |
| 18 | 2026-07-28 | The RC formula relies on the documented uninstall/install channel switch instead of Homebrew's `conflicts_with` DSL. Homebrew 6 loads the referenced stable formula while resolving that declaration, but direct formula installation trusts only the requested RC formula, so a user who correctly removed stable could still be blocked by tap trust. Omitting the declaration avoids broad tap trust without weakening the explicit no-coexistence rule. |
| 17 | 2026-07-28 | Homebrew distribution gains an opt-in `agentdeck-rc` formula in the existing tap. Stable users remain on `agentdeck`; strict `vX.Y.Z-rc.N` tags render, install-test, and propose only `Formula/agentdeck-rc.rb`, while other prereleases remain GitHub-only. Because both channels install the same binary and completion paths, switching is an explicit uninstall/install operation; subsequent RCs use normal `brew update` and `brew upgrade`. |
| 16 | 2026-07-27 | Time representation is one boundary: instants are stored and transported in UTC RFC 3339 with nanoseconds, normalized where a value enters AgentDeck, and rendered in the machine's zone only in human-readable text, to the second, with every text output naming the zone it used. JSON and NDJSON keep the stored UTC instant, because an envelope whose timestamps shift with the producing host cannot be compared across machines. Command inputs are unchanged: `usage stats --from/--to` still name local dates. No stored value changes and no migration is required. |
| 15 | 2026-07-26 | Any provider, including the built-in `official`, may carry an optional wrapper URL, and `provider use --via` routes one switch through it. The wrapper overrides the endpoint field alone, so a proxy in front of a relay still writes and forwards that relay's own credential, and subscription traffic through a proxy stays attributed to `official` at multiplier `1` instead of splitting into a second provider name. The URL is provider-owned rather than credential-owned because a wrapper instance is configured with one upstream address, and is not per client because one instance serves both client protocols on one address, always normalized like a Codex-bound credential endpoint regardless of which clients the provider actually serves. The route is chosen per switch rather than stored as an attachment, so inserting or removing a proxy changes no stored configuration and a configured wrapper never silently routes a switch that did not ask for it; the selection snapshot records which route was written. The built-in `official` provider becomes selectable for Claude as well as Codex. Owned client fields are enumerated per client, with everything else carried through unchanged. A Claude switch reports a running-session restart advisory and any conflicting credential source it does not own, rather than deleting fields it never wrote. |
| 14 | 2026-07-23 | Record that equivalent-estimate disclosure travels with the binary rather than the database: price rows move with a portable backup while the marker, basis, and note come from the running binary's compiled gap-fill, so a binary without that entry renders the price undisclosed. Accepted cost of deriving the metadata instead of storing it. The price-list estimate marker occupies its own column so the model column stays a copy-pasteable identifier. |
| 13 | 2026-07-23 | The bundled catalog's own effective date is the stable fallback date rather than the earliest date among its models, so a curated early-dated entry cannot lower the catalog's precedence and hand shared models back to a previously installed bundled catalog on upgrade. Model rows keep their own effective dates. An equivalent estimate is disclosed only for prices served by the catalog compiled into the running binary, and its `verified_by` may name a project role because it attests to a derivation rather than an observed vendor rate. |
| 12 | 2026-07-23 | Curated gap-fill may carry an explicitly marked equivalent estimate for a real released subscription-only model that has no published API rate. The estimate must name its vendor-priced basis model and explain that it is not an actual subscription invoice; only `price list` exposes the marker and basis, and fresher upstream pricing automatically removes it. Absent, unreleased, or unidentified models remain unpriced. |
| 11 | 2026-07-23 | Bundled price catalog becomes a generated, reproducible build artifact rebuilt from a pinned LiteLLM commit and never hand-edited, with a content-derived `catalog_version` so a price change cannot ship under a reused version and silently keep stale prices on installed copies, and a stable fallback effective date so a fresh install prices pre-build usage. Adds a curated gap-fill input for models upstream never prices: merged over the generated catalog so regeneration cannot drop it, held in the `bundled` layer so future upstream pricing automatically wins, and gated on a real vendor rate-card URL plus a named human verifier — an unconfirmed rate stays unpriced. |
| 10 | 2026-07-22 | Long scans report progress on stderr with a one-second delay, honor `--quiet`, emit no ANSI escapes off-TTY, and name a parser-version-triggered re-read. `usage stats` and `usage summary` gain `--no-scan` and keep scanning synchronously by default. Record two upstream measurement limits: Claude Code `ai-title` calls carry no usage object, and a first-party `/status` estimate may disagree with the catalog during introductory pricing. |
| 9 | 2026-07-21 | Release and distribution contract: tag-annotation notes extraction, completion-aware Homebrew formula rendering, isolated brew verification, and automated tap pull requests for stable tags only. |
| 8 | 2026-07-21 | Usage stats runtime provider dimension: derived per-client `providers` array, `--provider` global filter over an open value set, `unknown` as an explicit unattributed bucket, and the PROVIDERS text ranking. |
| 7 | 2026-07-20 | Usage output readability and analytics: shared ASCII grid for collection output, split session token components, nullable complete versus known partial cost, deterministic unpriced-model reporting, and Claude-only dot/hyphen model matching. |
| 6 | 2026-07-17 | Active-log-safe usage rebuild: per-source atomic replacement, preserved run bindings, partial warnings without advancing the watch checkpoint, and same-metadata snapshot revalidation. |
| 5 | 2026-07-17 | Automatic price updates: LiteLLM `main` commit resolution through the GitHub API, pinned raw catalog download, immutable commit and content-hash provenance, and bounded retry. |
| 4 | 2026-07-16 | Machine-bound encrypted credential storage: schema v9 `credential_secrets`, AES-256-GCM with reference-bound associated data, HKDF machine binding, and fail-closed key handling. Replaces the macOS Keychain adapter. |
| 3 | 2026-07-16 | Credential-owned provider configuration and the unified ASCII list-table output contract. |
| 2 | 2026-07-14 | Local session search: separate purgeable session database, extraction allowlists, and source-level incremental scan. |
| 1 | 2026-07-13 | Initial contract: provider and credential management, usage collection and attribution, price catalog, extensions, backup, output envelopes, and doctor. |
