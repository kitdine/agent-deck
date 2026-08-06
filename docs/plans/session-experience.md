---
status: active
created: 2026-08-06
---

# Session Experience

Target release: `v0.4.0`.

This plan makes session inspection the next high-value CLI release and fixes the
data and presentation contracts the native `v0.5.0` desktop app will consume.
It combines five previously independent Backlog candidates because their order
and shared DTO boundary are inseparable: search result time, scan progress,
invocation-level usage, readable `session show`, and an interactive viewer.

## Source Sufficiency

Design input is `SUFFICIENT`.

- The living CLI contract already defines the session privacy allowlist,
  source-level ownership, UTC storage and JSON transport, local-zone text,
  deterministic collection ordering, paging flags, and activity privacy.
- The Backlog identifies every required user-visible improvement and the
  unresolved time, token, pagination, and interaction decisions.
- Current code stores session metadata and approved documents in the purgeable
  `sessions.sqlite3`, stores normalized usage events in the core database, and
  already converts Codex cumulative token snapshots into per-event deltas during
  ingestion.
- The `v0.5.0` desktop plan requires bounded, stable session DTOs but deliberately
  leaves their definition to this release.

No user decision remains that changes this design's architecture or scope.

## Goals

- Make long session scans visibly active without changing deterministic stdout.
- Give every searchable document its own UTC event instant and make recent
  matches discoverable and pageable.
- Expose chronological, normalized usage-event token and cost details for one
  session without inventing unsupported prompt-turn boundaries.
- Make non-interactive `session show` readable at narrow and wide terminal widths.
- Add an explicit, keyboard-driven interactive viewer that pages each section
  independently and never loads an unbounded session into memory.
- Establish privacy-bounded Go DTOs suitable for the `v0.5.0` desktop wire
  contract without making Swift read AgentDeck databases or source logs.

## Non-Goals

- No Swift, menu-bar, WidgetKit, App bundle, Cask, signing, or notarization work.
- No change to which prompt, reply, or activity content may be indexed or shown.
- No indexing of tool arguments, tool results, commands, environment, hidden
  reasoning, system prompts, developer instructions, attachments, or binaries.
- No attempt to reconstruct a universal client-independent prompt/response turn.
- No rewrite of stored usage events, provider attribution, or historical prices.
- No session content in `agentdeck.sqlite3`; approved text remains only in the
  separately purgeable `sessions.sqlite3`.
- No TTY-default behavior change: scripts and ordinary `session show` remain
  non-interactive unless `--interactive` is explicitly supplied.
- No new TUI framework dependency in `v0.4.0`.

## Decisions

### 1. Search results use the matched document instant

`session search` gains the instant attached to the matched approved document,
not the session's `first_at` or `last_at` bound.

The session bounds describe the whole logical session and can be hours or days
away from the matching message. Repeating either bound on every result would be
easy to implement but semantically misleading, would sort resumed sessions
poorly, and would not support precise desktop navigation.

`session.Document` therefore gains:

```go
EventAt string `json:"event_at"`
```

The value is normalized to UTC RFC 3339 nanoseconds when the approved document
enters the index. JSON preserves that UTC value. Text renders it in the machine
zone to seconds and names the zone in the table header. If a supported source
record has no parseable instant, `event_at` is empty, text renders `—`, and the
document sorts after all known instants without borrowing a session bound.

The FTS table adds `event_at UNINDEXED`. The session parser version increases and
the rebuildable session-index migration recreates affected FTS/source state so
all documents are reparsed from read-only source logs. The core schema version is
unchanged; no session text moves into the core database.

Search ordering becomes:

1. known `event_at` descending;
2. unknown instants last;
3. client ascending;
4. session ID ascending;
5. selected source document order ascending.

`session search` gains `--page`, `--limit`, and `--all` with the same validation
and mutual-exclusion rules as other session collections. Text defaults to 20
results and prints the existing pagination footer and copyable next command.
JSON without explicit paging remains complete for compatibility; explicit paging
adds top-level `pagination.search` metadata.

The existing `client`, `session_id`, `kind`, and `text` JSON fields remain. Text
continues to show a bounded one-line approved-text snippet; no new source path or
content field is exposed.

### 2. Session scan progress reuses one shared CLI progress primitive

The existing delayed usage-scan progress renderer becomes a small command-layer
primitive rather than duplicating timers, terminal detection, synchronization,
and cleanup for sessions.

The session package reports structured progress only:

```go
type ScanProgress struct {
    Processed int
    Total     int
    Documents int
    Skipped   int
}
```

It never reports paths, project names, session IDs, indexed text, parser errors
containing source content, or other private identifiers.

Progress behavior:

- stderr only; stdout and JSON envelopes remain byte-deterministic;
- hidden during the short anti-flicker delay when a scan finishes quickly;
- one in-place line on a TTY, refreshed at the existing bounded interval;
- complete monotonic lines without cursor-control bytes on non-TTY stderr;
- fully suppressed by `--quiet`;
- final progress line closed before the existing `Completed session.scan.` or
  rebuild result is written;
- clean shutdown on success, error, context cancellation, zero sources, and
  panic-free early return.

Both `session scan` and `session rebuild` use the same progress contract. The
session package accepts a callback or options value and remains independent of
terminal detection and rendering.

### 3. Invocation detail means one normalized stored usage event

AgentDeck must not claim a prompt/response turn where the client provides no
stable turn identity. In this release, an invocation row is explicitly one
normalized usage event owned by the selected logical session.

- Codex cumulative `total_token_usage` snapshots are already converted to
  deltas during ingestion. The read path uses stored deltas and never subtracts
  them again.
- When Codex supplies a valid `last_token_usage`, it remains the fallback for the
  first snapshot, reset, or decreasing cumulative sequence under the existing
  parser contract.
- Claude cache-read, cache-creation total, five-minute write, and one-hour write
  components remain distinct.
- Duplicate source ownership and failed-source filtering reuse the current
  authoritative usage-event read path.
- Event pricing uses the existing event-time catalog and provider-attribution
  resolver. Historical stored rows are not rewritten.

The public DTO is privacy bounded:

```go
type SessionInvocation struct {
    Sequence             int              `json:"sequence"`
    EventAt              string           `json:"event_at"`
    Model                string           `json:"model"`
    Tokens               map[string]int64 `json:"tokens"`
    CatalogBaseCost      *string          `json:"catalog_base_cost"`
    ProviderCost         *string          `json:"provider_cost"`
    KnownCatalogBaseCost string           `json:"known_catalog_base_cost"`
    KnownProviderCost    string           `json:"known_provider_cost"`
    Unpriced             []string         `json:"unpriced_components"`
    Warnings             []string         `json:"warnings"`
}
```

`Sequence` is the one-based position in deterministic chronological order. It
is presentation metadata, not a persistent identity. Ordering is UTC `event_at`
ascending followed by the internal stable event key; the key, source path,
source offset, run ID, credential reference, and raw source payload are never
exposed.

The token map uses the existing stable component names:

```text
input_tokens
cached_input_tokens
output_tokens
cache_read_tokens
cache_creation_tokens
cache_write_5m_tokens
cache_write_1h_tokens
```

Nullable total costs mean complete pricing is unavailable. Known-cost fields
remain the sum of priced components, and unpriced components and warnings explain
partial coverage. The existing disclosed five-minute default for unbucketed
Claude cache creation applies exactly as it does in other usage reports.

`session show` gains `--tokens`. In non-interactive mode it adds a complete
session usage summary followed by the requested invocation page. Without
explicit paging, JSON remains complete, consistent with existing session JSON;
text defaults to 20. Explicit paging adds `pagination.invocations`.

### 4. Explicit paging reaches storage and source readers

The current command often loads complete collections and slices them afterward.
That remains compatible for legacy JSON requests with no explicit paging, but it
is not acceptable for an interactive viewer.

This release adds bounded service methods for:

- FTS search count and page;
- selected-source approved document count and page;
- usage invocation count, summary, and page;
- activity detail count/page plus a complete aggregate computed without retaining
  all detail rows.

SQL-backed collections use deterministic `COUNT` and `LIMIT/OFFSET` queries.
Activity remains source-on-demand and non-persistent: its reader streams the
selected source once, accumulates the complete safe summary, and retains only the
requested page. No activity row is written to either database.

The existing `--page`, `--limit`, and `--all` meaning is preserved. For ordinary
non-interactive `session show`, one explicit page number continues to apply to
every requested collection and the JSON pagination map continues to report each
collection independently. The interactive viewer maintains a separate page for
Documents, Activity, and Tokens.

### 5. `session show` gets a sectioned responsive text renderer

The text renderer moves into a dedicated command-layer file and renders:

```text
SESSION
  identity, client, project, model, time range, duration

DOCUMENTS
  approved user/final-assistant entries and page state

ACTIVITY                 only with --activity
  complete summary, per-tool counts, safe detail page

TOKENS                   only with --tokens
  complete token/cost summary, normalized invocation page, coverage warnings
```

Rules:

- session metadata always appears, even when a requested page is empty;
- each requested section has an explicit empty, unavailable, partial, or stale
  state rather than disappearing silently;
- timestamps follow the living time-representation contract;
- durations use consistent compact units while JSON stays milliseconds;
- narrow output prioritizes time, kind/tool/model, status, tokens, and cost and
  moves secondary values into detail lines rather than wrapping dense tables;
- wide output may use stable additional columns but never changes JSON;
- `NO_COLOR`, `--no-color`, `TERM=dumb`, and redirected output remain readable;
- pricing and activity warnings appear in their owning section;
- pagination footer immediately follows its collection.

The metadata and collection JSON shapes are preserved except for documented
additive fields and collections. No field is removed or renamed in `v0.4.0`.

### 6. Interactive viewing is explicit and read-only

The command is:

```text
agentdeck session show <session-id> --client <client> --interactive
```

`--interactive` is valid only for text output with TTY stdin and stdout. It is
mutually exclusive with `--page`, `--limit`, `--all`, `--activity`, and
`--tokens`; the viewer always makes Overview, Documents, Activity, and Tokens
available and loads detail pages only when selected.

Required controls:

```text
Left/Right or Tab/Shift-Tab   change section
Up/Down                      move selection
PageUp/PageDown              change page
Home/End                     first/last row in loaded page
q or Escape                  exit
```

The footer always shows available keys, current section, page, total rows,
staleness, and warnings. Resize recomputes layout without changing selection or
page. The viewer restores terminal mode and cursor visibility on normal exit,
error, cancellation, interrupt, and resize handling.

Implementation uses the already vendored terminal primitives and a small
AgentDeck-owned state machine. It adds no general TUI dependency. Key decoding,
state transitions, viewport calculation, rendering, and terminal lifecycle are
separate testable components.

The viewer is read-only. It does not rescan, mutate exclusions, change provider,
open source files beyond the existing selected-source safe readers, or expose
copyable raw session content beyond the same approved documents ordinary
`session show` already returns.

### 7. Desktop-facing DTO boundary

The reusable Go service boundary consists of:

- session summary metadata without source path;
- paged approved documents with UTC `event_at`;
- safe activity summary and page;
- session usage summary and paged normalized invocations;
- pagination, warnings, partial, stale, and generated-time metadata.

Internal source-selection structs may retain `SourcePath`, but the desktop-facing
projection must not. Swift will consume a versioned aggregate defined by the
`v0.5.0` `desktop-wire-contract` task; it will not depend directly on internal
database structs or the interactive renderer.

This release guarantees that every needed session component has a bounded,
privacy-reviewed Go API and deterministic JSON representation. It does not add
Swift code or commit the final desktop snapshot command prematurely.

## Failure and Safety Rules

- Index migration failure leaves source logs untouched and returns a typed read
  or schema error; it never partially exposes prohibited content.
- A malformed or unsupported source record fails closed and contributes no
  document or invocation fields beyond existing parser rules.
- Usage database failure may leave the Tokens section unavailable while approved
  session metadata/documents remain usable; text and JSON report partial state.
- Activity source disappearance or mutation uses the existing stale-session
  contract and never silently substitutes a different source.
- Viewer terminal setup failure returns before entering raw mode; cleanup is
  idempotent after raw mode starts.
- Progress failure cannot fail or cancel the underlying scan.
- No new log, warning, error, snapshot, or test fixture contains real source
  paths, credentials, prompts, replies, tool arguments, or environment values.

## Verification Strategy

- Parser/schema fixtures: exact document instant normalization, missing and
  malformed instants, parser-version rebuild, FTS source ownership, exclusions,
  incremental append, archive-copy ownership, and database mode.
- Search fixtures: time ordering, unknown-time ordering, client filter, FTS
  snippet safety, text timezone, JSON UTC, paging compatibility, and next command.
- Progress fake-clock fixtures: fast/slow, TTY/non-TTY, quiet, zero-source,
  cancellation, error, final line ordering, no paths/content, and goroutine exit.
- Usage fixtures: Codex first/cumulative/reset/decreasing snapshots, multiple
  token events in one apparent turn, Claude cache components, unpriced and
  defaulted cache creation, provider routing, duplicate source ownership, and
  failed-source exclusion.
- Show renderer fixtures: widths 60/100/140, no color, every section combination,
  empty page, partial usage, stale activity, warnings, pagination, and privacy.
- Interactive state-machine fixtures: every key, independent section pages,
  resize, cancellation, terminal restoration, unsupported non-TTY/JSON modes,
  and bounded page acquisition.
- Go/desktop DTO fixtures: canonical JSON decoded by a strict independent fixture
  parser before Swift exists, with source-path and sensitive-field rejection.
- Final L2 evidence: targeted package/CLI tests plus vendored `go test ./...` on
  the final relevant content state. Add the relevant race, privacy, cross-build,
  and size gates if implementation changes concurrency, privacy, dependencies,
  or binary size risk.
- Before stable release, run `v0.4.0-rc.1` against isolated copies of real local
  state and source logs because the release changes a persisted session-index
  format and reads event-time pricing for a new surface.

## Tasks

### 1. `session-document-time`

- Add document `event_at`, parser normalization, rebuildable FTS schema upgrade,
  search ordering, text timezone rendering, search pagination, and compatibility
  fixtures.
- Update living contracts only for behavior delivered by this task.
- Verification level: L2.

### 2. `session-scan-progress`

- Extract the shared delayed scan-progress primitive, add session progress
  callbacks, connect scan/rebuild, and cover TTY, non-TTY, quiet, cancellation,
  error, privacy, and final-output ordering.
- Verification level: L2 because stderr/text behavior is a shared CLI contract.

### 3. `session-usage-detail`

- Add bounded session usage summary/invocation APIs, `--tokens`, event-time
  pricing and coverage output, invocation pagination, Codex/Claude normalization
  fixtures, and privacy tests.
- Do not change stored usage rows or pricing rules.
- Verification level: L3 because this crosses pricing attribution and session
  privacy boundaries.

### 4. `session-show-layout`

- Add bounded document/activity readers, the sectioned responsive renderer,
  consistent empty/partial/stale states, per-section pagination, and width
  fixtures.
- Preserve existing JSON fields and activity allowlist.
- Verification level: L2.

### 5. `interactive-session-viewer`

- Implement explicit `--interactive`, the terminal state machine, lazy independent
  section paging, responsive rendering, resize handling, cleanup, and terminal
  integration tests without a new TUI dependency.
- Verification level: L3 if terminal concurrency or signal handling requires a
  race gate; otherwise L2 plus targeted PTY acceptance.

### 6. `v0-4-0-contract`

- Reconcile the complete release behavior into the living CLI design and manual,
  raise the specification version once, verify desktop DTO readiness, close all
  review records, and prepare RC downgrade/rebuild/privacy notes.
- This task runs only after every other task in the plan has Review PASS.
- Verification level: L2 for contract state; release publication later uses L4.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `session-document-time` | [ ] | [ ] |
| 2. `session-scan-progress` | [ ] | [ ] |
| 3. `session-usage-detail` | [ ] | [ ] |
| 4. `session-show-layout` | [ ] | [ ] |
| 5. `interactive-session-viewer` | [ ] | [ ] |
| 6. `v0-4-0-contract` | [ ] | [ ] |

Tasks 1 and 2 may proceed independently. Task 3 may proceed after its DTO names
are checked against task 1's shared session projection. Task 4 depends on tasks 1
and 3. Task 5 depends on the bounded APIs from tasks 1, 3, and 4. Task 6 runs
last. The `v0.5.0` desktop plan's `desktop-wire-contract` remains blocked until
task 6 passes review.

Commit boundaries follow task boundaries. This plan does not authorize commits,
pushes, release tags, RC publication, real-state mutation, or desktop work.

## Backlog / Future Feature Ideas

- Cross-session interactive navigation and saved viewer filters.
- Mouse support and selectable/copy-mode affordances after keyboard behavior is
  stable.
- Client-supplied exact prompt-turn grouping if both Codex and Claude expose a
  durable turn identity; until then invocation means normalized usage event.
- Desktop deep links into one document or invocation after the `v0.5.0` wire
  contract defines an opaque privacy-safe locator.

## Starting Task

Turn a Status row into scoped development by naming its anchor:

```text
进入开发：`session-experience` / `<task-anchor>`
```

Read `AGENTS.md`, this plan's Decisions and named task, the current Session Search
and Time Representation contracts in `docs/specs/cli-design.md`, the Session
manual, every file the task names, and verification routing. Tick `Dev` only
after selected verification passes. An independent reviewer records a PASS round
under `docs/reviews/session-experience/<task-anchor>.md` before ticking `Review`.
