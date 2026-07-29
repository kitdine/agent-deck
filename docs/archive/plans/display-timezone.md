---
status: historical
created: 2026-07-27
retired: 2026-07-29
---

# Display Timezone Plan

Render instants in the machine's zone wherever the audience is a person, and
nowhere else. Storage and JSON already carry UTC and keep carrying it.

**Specification:** `docs/specs/cli-design.md` v16, "Time Representation" under
`## Output and Errors`.

## Why

The storage half of this rule is already true and was migrated deliberately:
every timestamp is normalized to UTC RFC 3339 with nanoseconds at the boundary
where it enters AgentDeck (`usage.go` event ingest, `activity.go` tool calls,
the `store` writers, session sources, backups, price catalogs), range queries
compare UTC-formatted arguments, and schema v10 rewrote existing
`usage_events.event_at` and recomputed session bounds. Nothing here changes any
of that.

The display half is inconsistent. `usage stats` and `usage summary` resolve
their ranges and buckets in the machine zone and name that zone in their header.
Other instant-bearing surfaces print the raw stored instant: `provider current`
shows `2026-07-28T02:44:22.736011Z` in its `SELECTED AT` column, and `session
list|show`, `backup list|inspect|create`, and `price history` do the same. A user
reading those has to convert UTC in their head while the usage report two
commands away already localized for them. `session search` has no instant in
its result contract, so it has no timestamp to localize or zone to name.

Found while fixing two `cmd/agentdeck` tests that failed on any host west of
UTC. That fix introduced the seam this plan builds on, now named
`displayLocation()` in `cmd/agentdeck/main.go`, which resolves the zone once
and which tests swap.

## Invariants

- **JSON and NDJSON do not change.** Not the values, not the precision, not the
  field names. The GUI JSON contract fixture must stay byte-identical for every
  command; if a task's diff touches it, the task is wrong.
- **No stored value changes.** No migration, no rewrite, no new column. This
  plan touches renderers and the helpers they call.
- **Localization never fails a command.** A value that will not parse as an
  instant renders unchanged. A read command must not exit non-zero over
  presentation.
- **Every localized output names its zone.** A local time with no zone is worse
  than a UTC instant: it is ambiguous the moment it is pasted anywhere.
- **Command inputs keep their current meaning.** `--from/--to` are local dates
  today and stay local dates.
- **Text precision is seconds.** Sub-second digits stay in JSON.

## Tasks

### `display-clock`

Lift the zone seam and the formatting helpers into one place, with no
user-visible change yet. `reportLocation` is no longer usage-specific — rename
it and `usageTimezoneName` to display-neutral names, and add the one function
every renderer will call: parse a stored instant (or take a `time.Time`), render
it in the display zone to the second, and return the input unchanged when it
does not parse.

Files: `cmd/agentdeck/main.go`, `cmd/agentdeck/reporting_clock_test.go`.

Acceptance: no command's output changes; the existing zone guards still pass;
the new helper has direct tests for a UTC input, a fractional-second input, an
empty string, and a non-timestamp string.

### `provider-and-session-surfaces`

Localize the instant-bearing surfaces users read most: `provider current` and
`provider status <name>` (`SELECTED AT`), `session list`, `session show`
(`first`/`last`), and `session show --activity` (`STARTED`). Inspect `session
search`, but leave it unchanged because its result contract carries no instant.
Grid columns with timestamps name the zone in the header cell; the `session
show` detail fields name it after the value.

Files: `cmd/agentdeck/main.go`, `cmd/agentdeck/*_test.go`.

Acceptance: each listed instant-bearing surface renders local time and names the
zone; `session search` remains unchanged and gains no zone because it carries no
instant; JSON for the affected commands is unchanged; tests pin the zone through
the seam rather than depending on the host.

Dev complete (2026-07-29): wired the shared display clock into every
instant-bearing provider and session text surface in this task. `provider
current` and named `provider status` localize `SELECTED AT` and name the zone in
the table header. `session list` localizes `FIRST`/`LAST`, `session show`
localizes those detail values and appends the zone only after successfully
parsed instants, and `session show --activity` localizes `STARTED` with the zone
in the header. Invalid or empty detail timestamps remain byte-for-byte
unchanged.

`session search` remains unchanged because its result contract is
`CLIENT`/`SESSION`/`KIND`/`TEXT`; `session.Document` contains no instant to
localize. Adding session bounds there would be a search-output redesign outside
this task. A focused test pins that boundary so a zone is not fabricated where
no timestamp exists. JSON for all changed renderer inputs retains the original
UTC RFC 3339 values.

Behavioral RED:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run
  TestProviderAndSessionTextSurfacesUseDisplayZone -v` failed for provider
  current, provider status detail, session list, and session show/activity
  because text still exposed stored UTC values and unnamed columns.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run
  TestSessionShowLeavesInvalidDisplayTimesUnchanged -v` failed because the
  first implementation appended the display zone to unparseable and empty
  detail values.
- `TestSessionSearchTextHasNoInstantToLocalize` passed before the production
  change, intentionally protecting an existing no-instant boundary rather than
  claiming a regression.

GREEN and final development verification:

- all three focused display-time tests passed;
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` passed;
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./...` passed.

Review round 1 (2026-07-29): **REOPEN**. One P2 documentation-contract finding
remains. The task description and “each listed surface” acceptance criterion
still require `session search` to render local time and name the zone, while the
implementation note correctly records that its `CLIENT`/`SESSION`/`KIND`/`TEXT`
rows contain no instant and must remain unchanged under the v20 system
contract. Reconcile the task’s Why, surface list, and acceptance text with that
decision; no product change is indicated unless search output is deliberately
redesigned. Product, JSON, privacy, and existing integration checks passed
independent review. Full evidence is in
`docs/reviews/display-timezone/provider-and-session-surfaces.md`.

Review fix round 2 (2026-07-29): addressed the P2 documentation-contract
finding by limiting the Why, task scope, and acceptance criterion to
instant-bearing surfaces and explicitly preserving the no-instant `session
search` boundary. No product or test changes were made. Review remains
unchecked pending independent re-review.

Re-review round 3 (2026-07-29): **PASS**. The Round 1 P2 is closed, the
no-instant search boundary agrees across the v20 specification, task scope,
acceptance criterion, implementation, and focused test, and the possible search
timestamp redesign is isolated in `Backlog / Future Feature Ideas`. No new
medium-or-higher findings remain. Full evidence is in
`docs/reviews/display-timezone/provider-and-session-surfaces.md`.

### `backup-and-price-surfaces`

The same treatment for `backup list` (`MODIFIED`), `backup inspect` and
`backup create` (`created`), and `price history` (`EFFECTIVE`). These read
`time.Time` values and stored strings respectively, so both helper entry points
get exercised here.

Files: `cmd/agentdeck/main.go`, `cmd/agentdeck/*_test.go`,
`docs/specs/cli-manual.md`.

Acceptance: as above, plus the CLI manual documents the rendering rule once and
notes per surface that its timestamps are local. Sweep for any instant-bearing
text renderer these three tasks missed and either localize it or record why it
stays UTC.

Dev complete (2026-07-29): localized `backup list` `MODIFIED`, `backup
inspect`/`backup create` `created`, and `price history` `EFFECTIVE`. `backup
create` preserves its existing completion line and appends the localized
manifest creation time. `price status` inherits the localized embedded catalog
history.

The required renderer sweep found three additional human-readable instants:
`price list --verbose` provenance `EFFECTIVE`, `usage sessions` `FIRST`/`LAST`,
and the `watch` text prefix. They now use the same display clock and name the
zone. JSON and NDJSON retain their original UTC RFC 3339 values, including
backup manifests, price provenance, usage session bounds, and watch
`generated_at`. The CLI manual documents the shared rule and each affected
surface. The sweep also reviewed `version`'s explicitly labeled `UTC Build
Time`: it remains UTC because it is immutable release/support identity injected
in a fixed UTC layout and must compare identically across machines, rather than
a runtime domain instant.

Behavioral RED:

- `TestBackupAndPriceTextSurfacesUseDisplayZone` failed all five backup,
  price-history, and price-status text cases while their JSON values remained
  UTC.
- `TestInstantBearingTextRendererSweepUsesDisplayZone` failed the price-list
  provenance, usage-sessions, and watch text cases.

GREEN and final development verification:

- both focused display-time tests passed;
- the existing backup renderer contract test was updated to pin the display
  zone while its JSON assertion continues to require UTC;
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` passed;
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./...` passed.

Review round 1 (2026-07-29): **REOPEN**. One P2 sweep-completeness finding
remains. `usage stats --activity` still prints its `MODEL ACTIVITY` range as
raw stored UTC with nanoseconds and no zone
(`cmd/agentdeck/usage_stats_text.go:550-551` over the canonical UTC values from
`internal/usage/usage.go:2131`), on the same screen whose metadata line already
names the machine zone. That contradicts the global rendering rule this task
added to `docs/specs/cli-manual.md`, and the task's sweep acceptance criterion
requires every missed instant-bearing text renderer to be either localized or
recorded as a justified UTC exception. Three P3 observations (a fail-the-command
type assertion in `renderBackupCreateText`, no zero-`time.Time` guard in
`renderDisplayTimeWithZone`, and per-surface manual notes missing for the task-2
surfaces) are recorded but do not block. Product behavior, JSON and NDJSON
preservation, storage invariance, and regression coverage for the surfaces in
scope passed independent review, and `go test ./...` passed independently. Full
evidence is in `docs/reviews/display-timezone/backup-and-price-surfaces.md`.

Review fix round 2 (2026-07-29): addressed the P2 sweep-completeness finding.
`usage stats --activity` now renders its `MODEL ACTIVITY` range through the
shared display clock and names the zone once after the pair, via the new
`statsActivityRange` helper. Both bounds must parse before either is rewritten,
so a partially parseable range keeps its stored values and gains no zone. The
`MODEL ACTIVITY` range is now recorded per surface in the CLI manual, completing
the sweep list alongside price-list provenance, usage sessions, watch, and the
deliberate `version` UTC exception. `StatsModelActivity` JSON is unchanged.

Behavioral RED: the new
`TestInstantBearingTextRendererSweepUsesDisplayZone/usage_stats_model_activity_range`
subtest failed because the range line still concatenated the stored UTC values.
Its sibling `usage stats model activity keeps unparseable range unchanged` was
green before the fix, protecting the existing no-fabricated-zone boundary.

GREEN: both subtests pass and
`rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
-mod=vendor ./cmd/agentdeck` passes.

The three P3 observations from round 1 were deliberately not changed in this
round: the `renderBackupCreateText` failure mode and the zero-`time.Time`
rendering both need an explicit expected-behavior decision rather than a
mechanical edit, and adding per-surface manual notes for the task-2 surfaces
would reopen a task that already passed re-review. Review remains unchecked
pending independent re-review.

Re-review round 3 (2026-07-29): **PASS**. The Round 1 P2 is closed: an
independent re-sweep of every instant-bearing field reference in
`cmd/agentdeck/main.go` and `cmd/agentdeck/usage_stats_text.go` found no
renderer still printing a stored instant outside the display clock, and the
manual now records every localized surface plus the deliberate `version` UTC
exception. Both range bounds must parse before either is rewritten, so a range
cannot be half-localized or carry a zone it does not describe; empty and
unparseable bounds were verified to render unchanged with no zone. The three
P3 observations stay open by decision, with reasons recorded, and none blocks.
No new findings. Full evidence is in
`docs/reviews/display-timezone/backup-and-price-surfaces.md`.

## Out of Scope

- A `--timezone` flag or any per-invocation override. The machine zone is the
  contract; `TZ` already selects it at process start.
- Changing what `--from/--to` mean.
- Localizing JSON, NDJSON, or the envelope's `generated_at`.
- Relative rendering ("3 hours ago"). It is a different decision with its own
  ambiguity, and nothing in the backlog asks for it.

## Backlog / Future Feature Ideas

- Redesign `session search` results to carry an instant before adding timezone
  presentation. Decide whether that instant is the matched entry time or the
  session's `FIRST`/`LAST` bounds, then define the text and JSON contracts,
  sorting, and pagination compatibility. If approved, text should render the
  new instant in the machine zone while JSON keeps UTC. Until that separate
  design is approved, search remains `CLIENT`/`SESSION`/`KIND`/`TEXT` with no
  timestamp or zone label.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | display-clock | ✓ | ✓ |
| 2 | provider-and-session-surfaces | ✓ | ✓ |
| 3 | backup-and-price-surfaces | ✓ | ✓ |

All three tasks are delivered and independently reviewed. The plan is complete
and ready to retire into `docs/archive/plans/` once its delivery is committed.
