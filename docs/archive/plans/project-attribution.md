---
status: historical
created: 2026-07-28
retired: 2026-07-29
---

# Project Attribution Plan

Let a user tell a Headroom wrapper which project a client launch belongs to, so
one proxy in front of one upstream can report per project instead of seeing every
project on the machine as one stream.

AgentDeck labels the launches it owns, manages the provider configuration that
makes labelling possible, and documents the rest so a user can apply it
themselves. It does not write into files it does not already own.

**Specification:** `docs/specs/cli-design.md`, sections "Provider Wrappers",
"Owned Client Configuration Fields", and a new "Project Attribution" section that
task `attribution-contract` adds. The spec version rises with that task.

## Why

A wrapper is configured with one upstream address and serves every client on the
machine through it. It cannot see which project a request came from, because
nothing in the request says so. AgentDeck is in a position to know: `agentdeck
run` owns the client process and the directory it was launched in.

Headroom defines the protocol already — `X-Headroom-Project`, or a `/p/<name>/`
path prefix — and its own `wrap` command fills it in. A user who switches
providers with AgentDeck and launches clients with `agentdeck run` has no way to
get the same reporting without abandoning one tool for the other.

Two things shape the design, and both are why this is a plan rather than a patch:

- **Attribution is vendor-specific.** `X-Headroom-Project` means something to
  Headroom and nothing to a plain nginx or a logging proxy. AgentDeck's wrapper
  concept is deliberately protocol-agnostic — the spec says AgentDeck "cannot see
  a wrapper's own upstream configuration and never probes it". Sending a
  Headroom-shaped header to every wrapper would quietly contradict that.
- **A project is per directory; a client config file is per machine.** A project
  name written into a global config file freezes one project into a file every
  project reads. AgentDeck therefore never writes a project name anywhere: it
  supplies one per launch, and documents the per-project files a user may write
  themselves.

## Evidence

Verified on `main` at `8c053c9`, and against Headroom's published design:

- `internal/session/session.go:848` — AgentDeck already derives a project from
  the clients' own session records: `first(a["cwd"], a["project"], b["cwd"],
  b["project"])`, for both Codex and Claude.
- `internal/session/session.go:859-864` — `normalizeProject` is
  `filepath.Clean(v)`. The stored identity is the full cleaned working-directory
  path, not a name.
- `~/.claude/projects/` on this machine holds entries like
  `-Users-jobshen-go-src-github-com-kitdine-agent-deck` — Claude's own project
  identity is likewise the full path, with separators replaced.
- `cmd/agentdeck/main.go` `newRunCommand` — `agentdeck run` builds the child with
  `exec.CommandContext` and never sets `child.Env`, so the client inherits this
  process's environment. This is the one place AgentDeck owns a client process.
- `internal/provider/config.go` `WriteCodexConfig` — assigns
  `providers["custom"] = map[string]any{...}`. `[model_providers.custom]` is the
  block AgentDeck manages and it describes whichever provider is currently
  selected, so it is rewritten on every switch by design. A header mapping
  belongs to the provider it was written for and correctly does not outlive it.
- `internal/provider/service.go` `UseCredential` — `via` already resolves the
  wrapper URL and the completed selection records `ViaWrapper`, so the route is
  available without new plumbing.
- `cmd/agentdeck/main.go:591` — `agentdeck completion <bash|fish|zsh>` already
  emits shell integration, and `scripts/test-completion-install.sh` plus the
  Homebrew formula already install it. `shell-helpers` follows that shape rather
  than inventing a second one.
- Headroom [issue #802](https://github.com/headroomlabs-ai/headroom/issues/802) —
  header `X-Headroom-Project` or `/p/<url-encoded-name>/`; header wins; the value
  is trimmed, control-stripped, URL-decoded and length-capped on receipt; absent
  means `project = None` with aggregates unchanged. Its `wrap codex` uses
  `env_http_headers = { "X-Headroom-Project" = "HEADROOM_PROJECT" }` plus a
  per-launch variable; its `wrap claude` appends to `ANTHROPIC_CUSTOM_HEADERS`.
- Headroom [v0.27.0](https://github.com/headroomlabs-ai/headroom/releases/tag/v0.27.0)
  — "percent-encode non-ASCII cwd names in X-Headroom-Project header (#1071)".

Upstream references belong here and in `docs/specs/cli-manual.md`. They must not
reach command output; see `attribution-guidance`.

## Scope Decisions

**Three delivery mechanisms, one of which AgentDeck performs.**

| Mechanism | Who applies it | Covers |
| --- | --- | --- |
| Environment supplied per launch | AgentDeck, in `agentdeck run` | Launches AgentDeck makes |
| Environment set by a shell function | The user, after `agentdeck shell-init` | `claude` / `codex` invoked directly |
| A project-scoped settings file | The user, following our documentation | Whatever reads that file, including an app, if it reads it |

The third is documented, never written. `.claude/settings.local.json` inside a
user's repository is a legitimate way to attribute a project — it is the only one
that is per project rather than per launch — but writing inside someone's
repository is not something AgentDeck does, and the value it would hold is a
project name. We supply the recipe; the user applies it.

**Attribution is opt-in and wrapper-scoped.** A wrapper URL must be declared as
speaking Headroom's protocol before any attribution exists. Nothing is inferred
from the URL, the provider name, or the presence of a wrapper.

**The Codex mapping is ordinary provider management.** Codex emits a header only
if `env_http_headers` maps it, so a `--via` switch to a `headroom` wrapper writes
that mapping into the custom provider block and every other switch rewrites the
block without it. That is the same lifecycle as `base_url` and the bearer token
already have. The mapping names an environment variable, never a project, so
nothing about a project is persisted by it.

**GUI apps are not a target of this plan.** They are covered, if at all, by the
third mechanism above, at the user's own hand. Claude's app may or may not pick
up a project-scoped settings file without a restart; that is recorded as an open
question in the Backlog rather than answered here. The ChatGPT app is
lower-priority still and is not implemented for now, for the reason recorded
there.

## Tasks

### `headroom-wrapper-kind`

Let a wrapper URL carry an explicit declaration that it speaks Headroom's
attribution protocol. Without the declaration nothing downstream in this plan can
be reached, and a wrapper that has none behaves exactly as it does today.

Files: `internal/store/migrations.go`, `internal/store/providers.go`,
`internal/provider/service.go`, `cmd/agentdeck/main.go`.

Surface: `provider set-wrapper <name> --url <url> [--kind headroom|plain]`,
defaulting to `plain`; reported additively in `provider list|show|status` JSON
and text next to the existing `wrapper_url`.

The declaration lives on the wrapper as a column rather than in the generic
`settings` table: this task already opens a migration, and `provider show` is
where a user looks for what a wrapper does.

Acceptance: an existing database opens with every wrapper reading back as
`plain`; a `plain` wrapper produces byte-identical behavior to today on every
command; `--clear` removes the declaration along with the URL.

Done (2026-07-28): migration 16 adds a nullable `providers.wrapper_kind`
(`CurrentSchemaVersion` 15→16); the built-in provider's declaration reuses the
generic settings table beside its URL as `official.wrapper_kind`.
`provider.WrapperKindPlain`/`WrapperKindHeadroom` are the vocabulary and
`NormalizeWrapperKind` is the one validator — it resolves `""` to `plain` rather
than failing, because that is what both an omitted flag and a pre-existing row
carry. `SetProviderWrapper` and `SetOfficialWrapperURL` gained a `kind`
parameter rather than a separate setter, so a protocol cannot be written without
an address or outlive one: clearing the URL clears the declaration atomically —
in one statement for a stored provider, in one transaction for the built-in
provider's two settings rows — and re-declaring a URL without a kind drops the
previous one.
`Service.SetWrapper` validates the kind on the same path that normalizes the
URL, so a rejected declaration reaches no store call.

Reporting is additive by construction. `reportedWrapperKind` maps both `plain`
and unset to `""`, and `Provider.WrapperKind` is `omitempty`, so the JSON of a
wrapper that declared nothing — every wrapper that predates this field — has no
new key. Text annotates rather than adding a column or a line:
`wrapper: <url> (headroom)` on `provider show` and the same cell content in
`provider list`'s existing `WRAPPER` column, both unchanged when nothing is
declared. `--kind` is rejected with `--clear` (exit 2), because it describes the
URL being stored and has nothing to describe while clearing.

Scope note on this task's own wording: the original text said "the kind **and**
the preference" as if two fields. It is one. The separate preference belonged to
the `attribution-consent` task that was removed when the file-writing delivery
form was dropped, and `--kind headroom|plain` is now itself the opt-in. The task
text above was corrected rather than a redundant second column added.

Verification: `go test -mod=vendor ./...` (exit 0, 16 packages) and
`go vet -mod=vendor ./...` (clean), both run once after the final edit;
`gofmt -l internal cmd` and `git diff --check` clean. Targeted evidence:
`internal/store/wrapper_kind_test.go` (L3 migration-execution check — replays
`testdata/agentdeck-v6.sql` through `Open` and asserts the pre-existing provider
reads back with no declaration and its v15 snapshot fields intact, plus kind
round-trip, clear-with-URL for both storage paths, and the re-declare case),
`internal/provider/wrapper_kind_test.go` (vocabulary, default resolution,
rejection before any write), and `cmd/agentdeck/provider_wrapper_kind_test.go`
(both provider kinds through the CLI, the byte-identical criterion across
`provider show`/`list`, explicit `plain` indistinguishable from omission, clear,
and both input errors). The additive guarantee was confirmed RED: making
`reportedWrapperKind` return its argument unchanged fails
`TestUndeclaredWrapperIsReportedExactlyAsBefore`,
`TestReportedWrapperKindHidesTheDefault`, and
`TestSetWrapperWithoutAKindReportsNothingNew`.

Review Round 1 (2026-07-28): `REOPEN`, one P2 + two P3 + one nit, no
correctness or security defect. `Dev` unticked until the P2 closes. All three
acceptance criteria were independently reproduced, including a real 15→16
upgrade probe and a zero-diff differential run of every provider reporting
surface against a binary built from `HEAD`. The P2: a URL-only `set-wrapper`
silently drops an explicit `--kind headroom`, which contradicts `provider
update`'s documented partial-patch semantics and would silently disable
attribution once `run-env-injection` lands. See
`docs/reviews/project-attribution/headroom-wrapper-kind.md`.

Fix round (2026-07-28), closing all three Round 1 findings:

- **[P2]** Replacement semantics kept, silence removed. `set-wrapper` sets the
  whole wrapper — omitting `--kind` returns the declaration to the default,
  exactly as omitting it on a first call does — because this CLI's partial update
  is `provider update` and `set-wrapper` has no partial form. The defect was that
  the loss was invisible, so `Service.SetWrapper` now also returns the
  non-default declaration a call replaced, and `reportDroppedWrapperKind` prints
  `advisory: wrapper kind reset to plain (was headroom); pass --kind headroom to
  keep it` under the rules every other advisory follows. It fires only when a
  non-default declaration was actually dropped: a first set, `plain`→`plain`,
  `headroom`→`headroom`, and `--clear` are all silent, the last because removing
  the wrapper outright is already what the user asked for. The dropped value is a
  second return value rather than a field on `DefinitionResult`, since that type
  is the JSON payload and the advisory must never enter the envelope. The
  previous declaration is read before the write and a read failure only costs the
  advisory, so the write path's error behavior is unchanged.
- **[P3-1]** Closed by making the store's "no declaration" value the one the
  service actually sends, rather than by retargeting a test at an unreachable
  branch. `storedWrapperKind` is the write-side mirror of `reportedWrapperKind`:
  the default persists as absence, so one logical state now has one on-disk
  encoding and a wrapper written today is byte-identical in storage to one that
  predates the field. Verified against the built binary: an undeclared
  `set-wrapper` leaves `wrapper_kind` `NULL` and writes only the URL settings row
  for the built-in provider.
- **[P3-2]** Closed in code rather than by weakening the note.
  `SetOfficialWrapperURL` now performs its delete-then-insert pair inside one
  `BeginTx`/`Commit`, following the same pattern `AddProviderWithCredential`
  already uses in this file, so a declaration cannot survive a half-applied write
  of the URL it describes. `TestOfficialWrapperWriteLeavesNoOrphanedDeclaration`
  walks every reachable combination and asserts the pair is never observable in
  that state. The Done note's "same statement" wording was corrected above, since
  the two paths reach atomicity differently.
- **[nit]** Case sensitivity left alone, as recorded.

Verification: `go test -mod=vendor ./...` (exit 0, 16 packages) and
`go vet -mod=vendor ./...` (clean), run once after the final edit; `gofmt -l
internal cmd` and `git diff --check` clean. The review's differential probe was
re-run against the fixed build — six provider surfaces × text and JSON, old
binary vs fixed binary, timestamps normalized — and is **still an empty diff**,
so the advisory reaches stderr without touching any reporting surface and the
byte-identical acceptance criterion survives the fix.

Two existing call sites were updated for the new signatures
(`internal/provider/route_composition_test.go`, `internal/store/store_test.go`),
and `cmd/agentdeck/main_test.go`'s `TestStateMigrateTextAndJSONUpgradeSchema12`
fixture now drops `wrapper_kind` alongside `wrapper_url` before replaying from a
simulated v12 state.

### `project-identity`

Derive the project from the launch directory using the rule the session parser
already uses, so AgentDeck has one definition of "project" rather than two.

Files: `internal/session/session.go` (export or lift the existing rule),
new `internal/provider/project.go`.

- Identity is `filepath.Clean(cwd)`, matching `normalizeProject` and matching
  what both clients record.
- The wire value is the identity's **base name**, percent-encoded. Two
  directories with the same name collide in the proxy's report; sending the full
  path would make it unique but discloses the machine's directory layout to the
  proxy, and Headroom's own `wrap` deliberately sends only the base name.
- Encoding must escape control characters. A directory name containing a newline
  would otherwise append a second header to `ANTHROPIC_CUSTOM_HEADERS`.

Acceptance: the identity for a given directory equals what `session` stores for a
session whose `cwd` is that directory, proven by a test that calls both; a
directory with no name of its own attributes nothing; a name containing a
newline, a quote, or non-ASCII round-trips through the wrapper's documented
decode.

Dev complete (2026-07-28): `session.NormalizeProject` is now the exported single
definition of the cleaned full-path identity already stored by the session
parser. `provider.ProjectIdentity` reuses it directly, while
`provider.ProjectWireValue` exposes only the base name through
`url.PathEscape`; empty paths, filesystem roots, and relative directory
references `.` and `..` produce no attribution value. No project identity or
wire value is persisted.

The new `internal/provider` → `internal/session` package dependency is
intentional: this task chose to export the rule from the package that already
owns and stores the session project identity rather than lift a second
definition elsewhere. If later attribution tasks cause this dependency to
spread, the shared rule can be moved down into a neutral lower-level package;
this task does not pre-emptively change that structure.

Targeted coverage in `internal/provider/project_test.go` scans a synthetic Claude
session and compares its stored project with the launch-directory identity,
checks that only the base name reaches the wire, locks newline and quote escaping
plus non-ASCII UTF-8 encoding, decodes each value with `url.PathUnescape`, and
proves nameless directories attribute nothing. The targeted test was observed
RED before implementation with undefined `ProjectIdentity` and
`ProjectWireValue`, then passed after implementation. Verification after the
final code edit: `go test -mod=vendor ./internal/provider -run TestProject`
(exit 0), `go test -mod=vendor ./...` (exit 0, 16 packages), and `gofmt -l` on
the three touched Go files (no output).

Fix round (2026-07-28): `ProjectWireValue` now replaces the bare `+` that
`url.PathEscape` permits with `%2B`, so both path-style `unquote` and form-style
`unquote_plus` decoders recover the same project name. Coverage adds
`my+project`, `c++`, an explicit no-bare-plus assertion, and `..` as a nameless
directory reference while retaining every `url.PathUnescape` round trip and the
full-path `ProjectIdentity` comparison. A behavioral mutation replaced `%2B`
with `+` while keeping the code compilable; the two new plus cases failed with
their unescaped values, then passed after the fix was restored. Final fix
verification: `go test -mod=vendor ./internal/provider -run TestProject` (exit
0), `go test -mod=vendor ./...` (exit 0, 16 packages),
`go vet -mod=vendor ./...` (clean), and `gofmt -l` on the three touched Go files
(no output).

### `run-env-injection`

Supply the value to the client process AgentDeck launches, and manage the Codex
provider configuration that lets Codex act on it.

Files: `cmd/agentdeck/main.go`, `internal/provider/project.go`,
`internal/provider/config.go`, `internal/provider/service.go`.

- Claude: append `X-Headroom-Project: <value>` to `ANTHROPIC_CUSTOM_HEADERS` in
  the child environment. No file is written.
- Codex: set the variable in the child environment, and write the
  `env_http_headers` mapping that names it into `[model_providers.custom]` on a
  `--via` switch to a `headroom` wrapper, with every other switch rewriting the
  block without it.
- A value the user already set always wins, for both clients.
- Attribution never fails a run: an unresolvable route, directory, or preference
  drops the header and launches the client anyway.

Acceptance: a `plain` wrapper and a direct route both leave the child environment
untouched; the Codex mapping is present only for a `headroom` wrapper on `--via`;
all three Codex writers agree on one representation of `env_http_headers`, proven
by a test that runs them against each other's output — TOML cannot hold both a
sub-table and an inline key for one field, and `toml.Unmarshal` rejects the
combination with `key env_http_headers should be a table, not a value`.

Dev complete (2026-07-28): `agentdeck run` now resolves the completed provider
selection immediately before starting the child and adds attribution only when
that selection used a wrapper whose current declaration is `headroom`. Route,
provider, wrapper-kind, and working-directory lookup failures all leave
`exec.Cmd.Env` unset, so the child inherits the original environment and still
launches. Codex receives `HEADROOM_PROJECT=<encoded-base-name>`; Claude receives
an appended `X-Headroom-Project: <encoded-base-name>` line in
`ANTHROPIC_CUSTOM_HEADERS`. An existing Codex variable or case-insensitive
Claude project header wins, while unrelated Claude custom headers and every
other environment entry are preserved.

Codex provider selection now carries a `ProjectAttribution` intent into all
three writers. The canonical representation is the inline
`env_http_headers = { "X-Headroom-Project" = "HEADROOM_PROJECT" }` field.
Headroom `--via` writes it for custom and built-in providers; direct and plain
routes remove it. A pre-existing sub-table representation is removed before the
canonical field is written, preventing the invalid TOML state where the same
key is both a table and an inline value. The writer test runs wrapper, custom,
and built-in writers over each other's output and validates TOML plus exact
presence or absence after every transition.

Targeted coverage in `internal/provider/project_test.go` exercises direct,
plain-wrapper, Headroom-wrapper, and Headroom-to-direct transitions; custom and
built-in wrapper storage; Codex and Claude injection; user-value precedence;
nameless directories; unrelated environment preservation; and the service-to-
writer mapping decision. `internal/provider/config_test.go` covers the shared
canonical representation and sub-table normalization. Verification after the
final code edit: `go test -count=1 -mod=vendor ./internal/provider -run Project`
(exit 0), `go test -mod=vendor ./...` (exit 0, 16 packages), and `gofmt` on the
touched Go files (clean).

Fix round (2026-07-28): launch-time attribution now requires the completed
selection's endpoint to equal the provider's current wrapper URL in addition to
the current `headroom` declaration. A replaced custom or built-in wrapper
therefore leaves both Codex and Claude child environments untouched. Codex
project-header rewriting now reads the existing `env_http_headers` mapping,
changes only `X-Headroom-Project`, preserves every unrelated string mapping,
and writes one canonical inline field across wrapper, custom, and built-in
transitions. A disabled rewrite with no project key is a byte-for-byte no-op for
the unrelated field. Sub-table removal validates the table path through the
TOML parser, so basic-quoted, literal-quoted, and whitespace-separated dotted
paths normalize without producing a table/inline conflict.

Behavioral RED was recorded before the production fix with
`go test -count=1 -mod=vendor ./internal/provider -run Project`: all four
custom/built-in × Codex/Claude stale-route cases injected attribution, the
cross-writer fixture lost `X-Unrelated`, and all three equivalent sub-table
spellings failed with `key env_http_headers should be a table, not a value`.
After the fix the same targeted command passed, as did
`go test -count=1 -mod=vendor ./internal/provider`. Final verification:
`go test -mod=vendor ./...` (exit 0), `go vet -mod=vendor ./...` (exit 0),
`gofmt -l` on the six run-env-injection Go files (no output), and
`git diff --check` (exit 0).

Second fix round (2026-07-29): the shared Codex custom-table writer now uses the
same TOML-semantic path test as sub-table removal, so quoted or
whitespace-separated `model_providers.custom` spellings enter the managed table
without creating a duplicate canonical table. The probe also recognizes the
last element of an array-of-tables, retaining that writer behavior. Project
header field removal now parses the individual key/value line and recognizes
bare, basic-quoted, and literal-quoted `env_http_headers` keys before emitting
the canonical inline field.

Behavioral RED covered a fully quoted outer table plus sub-table
(`table custom already exists`) and both quoted inline key forms
(`key env_http_headers already defined`). After the semantic matcher change all
three cases passed with `X-Unrelated` preserved, as did the existing
array-of-tables tests in the complete provider package. Final verification:
`go test -count=1 -mod=vendor ./internal/provider -run Project` (exit 0),
`go test -mod=vendor ./...` (exit 0), `go vet -mod=vendor ./...` (exit 0),
`gofmt -l` on the six run-env-injection Go files (no output), and
`git diff --check` (exit 0).

### `attribution-guidance`

Tell the user that there is something for them to do, and where it is written
down. Do not tell them the whole story on stderr.

Files: `cmd/agentdeck/main.go`, `internal/provider/service.go`,
`docs/specs/cli-manual.md`.

- After `provider set-wrapper --kind headroom` succeeds, and after a `--via`
  switch to such a wrapper, print a short advisory on stderr under the existing
  rules: never in the JSON envelope, no effect on exit status, suppressed by
  `--quiet`.
- The advisory states what AgentDeck attributes (`agentdeck run`), that other
  launches are not attributed, and **one** link to this project's own
  documentation. Nothing else. It is an advisory, not a manual page.
- **No third-party content in command output.** No Headroom issue numbers, no
  upstream release notes, no external URLs. Those belong in
  `docs/specs/cli-manual.md`, which this task extends with the full explanation:
  the three mechanisms, the `.claude/settings.local.json` recipe the user applies
  themselves, and the upstream references.
- The link is to this repository on GitHub, since the project has no website yet.
  It is a single constant with one definition, so a future website is one edit.
- The advisory is printed by the two named commands only, not on every run.

Acceptance: command output contains no external hostname other than this
project's own repository; the JSON envelope of a guided command is byte-identical
to an unguided one apart from `generated_at`; `--quiet` suppresses it; the manual
section covers all three mechanisms.

Dev complete (2026-07-29): successful explicit `--kind headroom`
`set-wrapper` calls now emit one short project-attribution advisory, as do
successful `provider use --via` switches whose completed selection still
matches the provider's current Headroom wrapper URL and kind. Read-back
failures, stale routes, direct routes, and plain wrappers produce no guidance.
The advisory has one AgentDeck manual URL from a single production constant,
contains no upstream issue or release content, goes only to stderr, and is
suppressed by `--quiet` without changing command status or JSON data.

`docs/specs/cli-manual.md` now explains the three application mechanisms,
includes the user-owned `.claude/settings.local.json` recipe, and keeps
Headroom protocol references in documentation only. Behavioral coverage in
`cmd/agentdeck/project_attribution_guidance_test.go` exercises both named
commands, negative route cases, exactly one project link, JSON isolation,
byte-identical JSON serialization apart from `generated_at`, successful exit
status, and quiet suppression. The older wrapper-reset test now names only the
reset advisory it owns, so the new guidance does not weaken that contract.

RED was compilable: before production changes,
`go test -count=1 -mod=vendor ./cmd/agentdeck -run ProjectAttributionGuidance`
failed both new tests with guidance count `0` on successful Headroom
`set-wrapper`. After implementation the same command passed. Final targeted
verification: `go test -count=1 -mod=vendor ./cmd/agentdeck` (exit 0) and
`go test -count=1 -mod=vendor ./internal/provider` (exit 0).
`gofmt -l` on the four affected Go files produced no output, and
`git diff --check` exited 0.

Review-fix complete (2026-07-29): the JSON/quiet contract now compares the
outputs of two real `provider use --via` CLI invocations after removing only
`generated_at`; the previous synthetic `writeResult` comparison was removed.
The stderr assertion now extracts the actual project-attribution advisory,
requires exactly one matching line and one URL equal to the production
`ProjectAttributionGuideURL`, and checks that actual line for third-party
hostname, issue, and release content. The test no longer carries a second
hard-coded documentation URL. The manual continues to describe all three
mechanisms and the Claude settings recipe, but now accurately presents the
shell-function path as user-maintained rather than advertising the unimplemented
`agentdeck shell-init`.

Mutation evidence was compilable and behavioral: temporarily appending
`https://third-party.example/issues/1` to the production advisory made both
`ProjectAttributionGuidance` tests fail with `project attribution advisory URLs
= 2`; restoring the advisory returned the targeted test to GREEN. Final
verification:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run ProjectAttributionGuidance` passed.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` passed.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/provider_wrapper_kind_test.go
  cmd/agentdeck/project_attribution_guidance_test.go
  internal/provider/service.go` produced no output.
- `rtk git diff --check` exited 0.

Review complete (2026-07-29): Round 3 independently confirmed all three Round
1 findings closed. The real CLI JSON comparison, actual stderr advisory
validation, two-URL mutation evidence, and corrected user-maintained
shell-function documentation all satisfy the task contract. The focused test,
Go formatting check, and diff check passed; the unchanged full package result
was reused. Verdict: PASS.

### `shell-helpers`

Ship the environment recipe as something installable rather than something to
copy by hand. The mechanism is exactly two things: derive the project name from
the current directory, and export the client's variable before exec.

Files: `cmd/agentdeck/main.go`, plus whatever the completion command's install
path already uses (`scripts/test-completion-install.sh`, the Homebrew formula).

- `agentdeck shell-init <bash|fish|zsh>` writes a shell function to stdout, in
  the same shape as the existing `agentdeck completion` command, so the install
  story and its test are the ones the project already has.
- The function wraps `claude` and `codex`: it derives the project name by the
  same rule as `project-identity`, exports the client's variable, and execs the
  real binary. It is a no-op when the variable is already set, matching the "user
  value wins" rule.
- For Codex the variable is only half of it — Codex emits the header only if the
  `env_http_headers` mapping is present, which a `--via` switch to a `headroom`
  wrapper writes. The manual must say so, or a user will set the variable and see
  nothing.
- Emitting the function writes nothing and requires no wrapper to exist. Whether
  the user sources it, and from which file, is entirely theirs.
- The function must not embed a project name, an endpoint, or any credential —
  only the derivation.

Acceptance: the emitted script is valid in each named shell and passes each
shell's own syntax check; sourcing it and launching a client produces the same
header value that `agentdeck run` would have produced from the same directory,
proven by comparing both against one fixture directory.

Dev complete (2026-07-29): added `agentdeck shell-init <bash|fish|zsh>` as a
stdout-only command that emits fail-open `codex` and `claude` wrapper functions.
The functions call back into a hidden resolver at each launch, so project
identity and percent encoding stay in the existing Go helper; the shared
route-independent environment helper also keeps the same Codex/Claude
user-value precedence and Claude unrelated-header preservation as
`agentdeck run`. Emission neither opens AgentDeck state nor requires a
configured wrapper, and the generated text contains no project, endpoint, or
credential. The manual now explains how to source each shell's output and that
Codex needs the managed `env_http_headers` mapping written by a Headroom
`provider use --via` switch.

Behavioral RED was
`rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
-mod=vendor ./cmd/agentdeck -run ShellInit`: all supported-shell cases failed
with `accepts 0 arg(s), received 2`, and the unsupported-shell assertion failed
against that same missing-command behavior. After implementation:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run ShellInit` passed.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run GUIJSONContractFixture` passed.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./internal/provider -run Project` passed.
- The originally recorded `scripts/test-completion-install.sh` result was
  invalidated by review round 5: the script had exited before its helper and
  completion-install assertions. After the round 5 harness repair, a bare,
  unpiped run exited 0 and verified the real built binary, all three shells,
  helper route/fail-open assertions, and the subsequent completion
  install/uninstall/rollback checks.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/contract_test.go cmd/agentdeck/shell_init_test.go
  internal/provider/project.go internal/provider/service.go` printed nothing.
- `rtk git diff --check` exited 0.

Review round 1 (2026-07-29): **REOPEN**. The helpers attribute every launch
regardless of wrapper kind, so a sourced Claude helper sends
`X-Headroom-Project` to whatever upstream is currently selected — proven with an
empty state directory and no provider configured — which contradicts
"Attribution is opt-in and wrapper-scoped" and makes the acceptance comparison
against `agentdeck run` hold only in the one configured state the install test
exercises. Four P2 findings also remain: the new `leafCommands` exclusion drops
`shell-init` from three contract tests and the GUI fixture even though its error
envelope already conforms; the help catalog entry uses one-space indentation
where every other command uses two; and the recorded evidence covers only three
`-run` selections for a change that edits a helper shared by five call sites.
Findings and independent verification live in
`docs/reviews/project-attribution/shell-helpers.md`.

Fix round (2026-07-29): closed the seven recorded findings without changing
the public `shell-init <bash|fish|zsh>` surface. The hidden runtime resolver now
accepts no shell positional argument, opens existing AgentDeck state read-only,
and delegates to `RunProjectEnvironment`; missing state, unreadable state,
missing selection, stale route, and non-Headroom wrapper all produce no value
and leave client launch fail-open. This preserves the plan's opt-in
wrapper-scoped decision rather than documenting a Claude-specific deviation.

`completion` and `shell-init` are intentionally classified together as commands
that emit sourceable shell programs rather than GUI JSON data. A named,
commented predicate now owns that contract-test exclusion, and a dedicated test
pins `shell-init`'s JSON syntax-error envelope (`command: shell-init`,
`code: invalid_argument`). The help entry is back inside the catalog map
literal with the standard two-space argument/example indentation. Generated
fish output and the manual record fish 3.4 as the minimum version.

Regression coverage added:

- behavioral RED:
  `rtk proxy env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run
  TestShellInitProjectEnvironmentRequiresHeadroomSelection -v` failed because
  an empty state still resolved `my%2Bproject`; it passed after the route-aware
  resolver change;
- the install test now compares both Codex and Claude helpers with
  `agentdeck run` in a state with no provider selection and confirms both inject
  nothing, then repeats the existing Headroom-route equality checks;
- bash, fish, and zsh each prove the real client still starts when `agentdeck`
  is absent, exits non-zero, or exits successfully with empty output.

Final verification after the fix:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck ./internal/provider ./internal/session` passed.
- The install-script result recorded in this round was later invalidated by
  review round 5 because the script had aborted before those assertions. The
  repaired script was subsequently run bare without a pipeline and exited 0;
  that current result covers every shell's `-n`, configured/unconfigured route
  comparisons, fail-open cases, and the original completion-install suite.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/contract_test.go cmd/agentdeck/shell_init_test.go
  internal/provider/project.go internal/provider/service.go` printed nothing.
- `rtk git diff --check` exited 0.

The pre-existing `AGENTS.md` workflow-policy change was not modified for this
repair and remains a separate logical change that must be committed separately
from shell-helpers. No commit was made in this fix round.

Review round 3 (2026-07-29): **REOPEN**. The P1 is closed by construction —
the resolver now delegates to the same `RunProjectEnvironment` predicate as
`agentdeck run`, and the route-independent export was removed rather than left
as a parallel path. Probing a built binary confirmed all three selection states
agree, including a stale `via` selection whose wrapper kind was reset to
`plain`. Three of the four P2 findings are closed; the help-shape finding is
partially fixed (leading indentation corrected, one space of description offset
remains) and is downgraded to P3. One new P2 remains:
`docs/specs/cli-manual.md:145` still promises the helper "不要求预先配置
wrapper", which the fix made untrue, so a user would source it, see no header,
and find no documented reason. Findings and evidence live in
`docs/reviews/project-attribution/shell-helpers.md`.

Fix after review round 3 (2026-07-29): corrected the manual to distinguish
script generation from attribution activation. Generating or sourcing the
script requires no wrapper configuration; runtime attribution activates only
when the current selection uses a wrapper declared `headroom`, otherwise the
helper silently launches without injection. The adjacent Codex paragraph now
states only its additional `env_http_headers` mapping requirement without
repeating the `provider use --via` instruction.

The completion-install test now adds the missing configured-provider/direct-
switch state before the existing `--via` state. Real `agentdeck run` and
sourced bash, fish, and zsh helpers all launch successfully and produce empty
Codex and Claude attribution values. This was a coverage gap over already
correct behavior, so the new assertion was GREEN before any production
behavior change rather than a defect RED. The optional help-shape cleanup also
aligns the description column with `completion`.

Verification after the final edit:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` passed in 24.703s.
- This round's install-script pass claim was invalidated by review round 5:
  the script had stopped before the direct-switch comparison. After creating
  both client config fixtures and neutralizing the expected no-selection
  failures, the bare unpiped script run exited 0 and executed the direct-switch
  comparison plus all later completion-install checks.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/contract_test.go cmd/agentdeck/shell_init_test.go` printed
  nothing.
- `rtk git diff --check` exited 0.

No commit was made.

Review round 5 (2026-07-29): **REOPEN**. Both round 3 findings are closed —
the manual now separates script generation from attribution activation, and the
configured-provider/direct-switch state is written into the install test — but
that install test has never run any of its assertions. Measured without a pipe,
`scripts/test-completion-install.sh` exits 1: under `set -euo pipefail` the
no-selection `agentdeck run` substitution aborts the script, and behind it
`provider use --config-path` fails because `test_shell_helpers` declares
`codex_config` and `claude_config` without ever creating those files. Since the
function runs before `install_for`, the abort also skips the completion install,
uninstall, and rollback checks that worked before this task. Every shell-level
claim recorded across the Dev note and rounds 2 through 4 — including my own
round 3 line, which misread a piped exit status — is therefore unsupported; the
Go-level assertions in `cmd/agentdeck/shell_init_test.go` did run and did pass.
A patched scratch copy creating both config files and tolerating the
no-selection failures exits 0, so both defects are in the harness and no
production behavior needs to change. Findings and evidence live in
`docs/reviews/project-attribution/shell-helpers.md`.

Fix after review round 5 (2026-07-29): repaired only
`scripts/test-completion-install.sh`. `test_shell_helpers` now creates
`config.toml` with `model = "synthetic"` and `settings.json` with `{}` before
the first provider switch, matching the Go fixture. The no-selection
substitutions remain as a narrow empty-output/fail-open check and explicitly
tolerate the expected failed `agentdeck run`; they are not treated as launch
equivalence evidence. The configured-provider/direct-switch case remains the
strong proof that both real `agentdeck run` and every sourced helper launch
successfully without injection.

Verification was rerun without a pipeline:

- `rtk proxy bash scripts/test-completion-install.sh` reached process
  completion with the tool reporting raw exit code 0. Because
  `test_shell_helpers` runs first under `set -euo pipefail`, this also confirms
  every later completion install/uninstall, tamper rejection, rollback, and
  interruption-cleanup check executed successfully.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` passed in 25.035s.
- `rtk git diff --check` exited 0.

All earlier install-script pass claims in this task note have been corrected to
record their invalidation and this current bare exit-0 result. No product code
was changed and no commit was made.

Review complete (2026-07-29): Round 7 independently confirmed every finding from
rounds 1, 3 and 5 closed. A bare, unpiped run of the install script exits 0, and
`bash -x` shows 77 executed `test` assertions comparing real values — the
`my%2Bproject` wire value and the newline-bearing Claude header across
`agentdeck run` and all three shells, nine fail-open cases, and the
direct-switch and no-selection equalities — followed by the previously blocked
completion install, rollback, interruption, and tamper checks. Go sources are
unchanged since round 5; the full affected packages, `gofmt`, and
`git diff --check` all pass. The unrelated `AGENTS.md` workflow-policy diff
remains in the worktree and must be committed separately. Verdict: PASS.

### `attribution-contract`

Write the contract down. Raise `docs/specs/cli-design.md`'s version, add the
"Project Attribution" section, and extend "Owned Client Configuration Fields"
with the Codex mapping. Reconcile `docs/specs/cli-manual.md` with what shipped.

Files: `docs/specs/cli-design.md`, `docs/specs/cli-manual.md`, `docs/README.md`.

The limits are part of the contract, not a footnote: a client launched outside
`agentdeck run` is attributed only if the user installed the shell helper or
wrote a settings file themselves; GUI apps are not attributed by AgentDeck; and a
wrapper that is not marked `headroom` is never sent an attribution header.

Acceptance: every behavior the other tasks shipped is described, and every
behavior described is shipped.

Dev complete (2026-07-29): raised `docs/specs/cli-design.md` to version 20,
added the "Project Attribution" contract, and extended "Owned Client
Configuration Fields" with the Codex
`env_http_headers["X-Headroom-Project"] = "HEADROOM_PROJECT"` mapping and its
preservation boundary. The contract records the shipped identity, route
eligibility, fail-open, user-value precedence, transport, delivery, advisory,
and persistence behavior. It explicitly limits non-`agentdeck run` attribution
to a user-installed shell helper or user-written settings, excludes GUI app
launches, and forbids AgentDeck-generated attribution headers for wrappers not
declared `headroom`. `docs/specs/cli-manual.md` and `docs/README.md` now match
the delivered behavior and review state.

L0 verification after the final documentation edit:

- The following local-link check passed with
  `local Markdown links OK (4 files)`; every relative Markdown target exists
  (the checked files contain no local-anchor links):

  ```bash
  rtk proxy ruby -e 'ARGV.each { |src| File.read(src).scan(/\[[^\]]*\]\(([^)]+)\)/).flatten.each { |target| next if target.match?(/\A(?:https?:|mailto:|#)/); path = File.expand_path(target.split("#", 2).first, File.dirname(src)); abort("#{src}: missing #{target}") unless File.exist?(path) } }; puts "local Markdown links OK (#{ARGV.length} files)"' docs/specs/cli-design.md docs/specs/cli-manual.md docs/README.md docs/plans/project-attribution.md
  ```
- `rtk git diff --check` exited 0.

Review round 1 (2026-07-29): **REOPEN**. Two P2 findings. The fail-open sentence
in `cli-design.md:709-712` promises that no completed selection "never blocks
client launch", but `agentdeck run codex --` in that state exits 1 with
`no provider selection for client` and never launches the client — the guarantee
holds only for the `shell-init` helper, so one half of the described behavior is
not shipped. Separately, "Provider Wrappers" still lists only
`set-wrapper --url`, `--clear`, and `use --via`, omitting the `--kind
headroom|plain` declaration, its `plain` default, its removal by `--clear`, and
its additive reporting — all shipped by `headroom-wrapper-kind` — even though the
new section relies on the phrase "declared `headroom`" throughout. Two P3 items:
the `docs/README.md` rollup departs from the file's own `active — X/N done`
format, and the contract does not record `shell-init`'s stdout-only output
boundary. The Codex mapping preservation rule and the Claude launch-environment-
only cell were both confirmed correct end to end. Findings and evidence live in
`docs/reviews/project-attribution/attribution-contract.md`.

Review fix round 1 (2026-07-29): split the fail-open contract by delivery path.
`agentdeck run` now documents completed selection as a launch precondition and
limits attribution fail-open behavior to a process it has already decided to
start; `shell-init` documents silent no-injection plus real-client execution for
every lookup failure or ineligible route. "Provider Wrappers" now defines
`--kind headroom|plain`, the `plain` default, `--clear` removal of URL and kind,
and additive kind reporting in `provider list|show|status`. The documentation
index again uses its prescribed `active — 5/6 done` rollup. The spec records the
intentional output-contract decision: `shell-init`, like `completion`, emits a
sourceable program on stdout and is outside the GUI JSON data contract, while
argument failures retain the standard error envelope.

L0 verification after the round 1 fixes:

- The local Markdown link command recorded above, extended with
  `docs/reviews/project-attribution/attribution-contract.md`, printed
  `local Markdown links OK (5 files)` and exited 0.
- `rtk proxy ruby -e 'old_ref = /specs\/cli-design\.md`?\s+v19/i;
  current = "Currently version " + "19"; hits = Dir.glob("docs/**/*.md")
  .flat_map { |path| File.readlines(path, chomp: true).each_with_index
  .filter_map { |line, i| "#{path}:#{i + 1}:#{line}" if line.match?(old_ref) ||
  line.include?(current) } }; design = File.read("docs/specs/cli-design.md");
  abort("cli-design frontmatter is not version 20") unless
  design.match?(/^version: 20$/); abort(hits.join("\n")) unless hits.empty?;
  puts "cli-design version 20; no live references to v19"'` printed
  `cli-design version 20; no live references to v19` and exited 0.
- `rtk git diff --check` exited 0.

## Out of Scope

- The `/p/<name>/` URL-prefix form. AgentDeck's wrapper URL is one stored base
  that both clients derive from; folding a per-launch project into it would make
  the stored URL depend on where a command ran.
- Writing any file AgentDeck does not already own, including project-scoped
  settings files and shell profiles. Documenting them is in scope; writing them
  is not.
- GUI app attribution performed by AgentDeck. Recorded in the Backlog.
- Reading Headroom's `/stats` or surfacing per-project savings inside AgentDeck.
  This plan supplies the label; the proxy owns the report.
- Detecting that traffic actually reaches a Headroom instance. AgentDeck writes
  configuration and does not probe endpoints.

## Invariants

- **Default behavior is unchanged.** A wrapper with no `headroom` kind, or a
  direct route, produces byte-identical behavior, output, and files to today.
- **AgentDeck never writes a project name.** Not to a client config file, not to
  its own database. It supplies one per launch and documents the files a user may
  write themselves. The Codex mapping names an environment variable.
- **AgentDeck writes no file it does not already own.** Guidance, documentation,
  and a script emitted to stdout are not writes.
- **One definition of "project".** The value comes from the rule
  `internal/session` already uses. Do not add a second derivation, in Go or in
  the emitted shell function.
- **No third-party content in command output.** Upstream protocol references live
  in this project's documentation, and command output links only to this project.
- **A value the user set always wins**, whether they set it in their shell, in a
  settings file, or through the helper.
- **Attribution never fails a launch.** It is reporting metadata; a failed lookup
  drops the header and starts the client.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | headroom-wrapper-kind | ✓ | ✓ |
| 2 | project-identity | ✓ | ✓ |
| 3 | run-env-injection | ✓ | ✓ |
| 4 | attribution-guidance | ✓ | ✓ |
| 5 | shell-helpers | ✓ | ✓ |
| 6 | attribution-contract | ✓ | ✓ |

Done: **6/6 reviewed; every task is complete and this plan is ready to retire.** The implementer ticks **Dev** once a task is built and
its targeted verification passes; an independent reviewer ticks **Review** once
findings are closed, recording the round in
`docs/reviews/project-attribution/<task-anchor>.md`. A task is done only when
Review is ticked.

Sequencing: task 1 gates everything. Tasks 3, 4 and 5 need 1 and 2 but not each
other. Task 6 is last, because it records what actually shipped.

A usable vertical slice is tasks 1, 2, 3, 6: attribution for both clients
launched through `agentdeck run`, with the limits documented. Tasks 4 and 5 are
what make it usable without reading the manual first.

## Starting a task

Turn any row of the Status matrix into a scoped development instruction through
its anchor:

> **进入开发:project attribution / `<task-anchor>`**
> 阅读 `AGENTS.md`、本 plan `## Tasks` 中 `<task-anchor>` 一条及它命名的文件、本
> plan 的 `## Scope Decisions`、`## Invariants` 与 `## Required Verification`,
> 以及 `docs/specs/cli-design.md` 的 "Provider Wrappers" 与 "Owned Client
> Configuration Fields" 两节。只在该 task 的范围内实现并自测。完成后在 `## Status`
> 勾上该行的 `Dev`,把命令与结果记进该 task 的完成注记;评审留痕到
> `docs/reviews/project-attribution/<task-anchor>.md`。

## Required Verification

L2 for tasks 1, 2 and 3: they change SQLite schema, persisted provider state,
client configuration files, or a JSON/text output contract. Run the affected
targeted tests plus `go test -mod=vendor ./...` once after the final edit of each
task.

Task 1 executes a migration against existing databases, which is an L3 trigger
under `AGENTS.md`; it adds the migration-execution check on an existing-database
fixture on top of L2.

L1 for tasks 4 and 5: stderr advisory and a generated script, with no persisted
or envelope contract change. Targeted tests are sufficient, plus each shell's own
syntax check for the emitted script.

L0 for task 6: documentation. `git diff --check` and a link check.

No task touches concurrency, credential ciphertext, or the build and installer
path beyond reusing the completion command's existing install test, so no race,
cross-build, size, or `release-verify` gate is required. Commands are listed in
`AGENTS.md` under "Testing and Verification".
