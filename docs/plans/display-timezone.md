---
status: active
created: 2026-07-27
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
Every other surface prints the raw stored instant: `provider current` shows
`2026-07-28T02:44:22.736011Z` in its `SELECTED AT` column, and `session
list|search|show`, `backup list|inspect|create`, and `price history` do the
same. A user reading those has to convert UTC in their head while the usage
report two commands away already localized for them.

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

Localize the surfaces users read most: `provider current` and
`provider status <name>` (`SELECTED AT`), `session list`, `session search`,
`session show` (`first`/`last`), and `session show --activity` (`STARTED`).
Grid columns name the zone in the header cell; the `session show` detail fields
name it after the value.

Files: `cmd/agentdeck/main.go`, `cmd/agentdeck/*_test.go`.

Acceptance: each listed surface renders local time and names the zone; JSON for
the same commands is unchanged; tests pin the zone through the seam rather than
depending on the host.

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

## Out of Scope

- A `--timezone` flag or any per-invocation override. The machine zone is the
  contract; `TZ` already selects it at process start.
- Changing what `--from/--to` mean.
- Localizing JSON, NDJSON, or the envelope's `generated_at`.
- Relative rendering ("3 hours ago"). It is a different decision with its own
  ambiguity, and nothing in the backlog asks for it.

## Status

| # | Task | Dev | Review |
|---|------|:---:|:------:|
| 1 | display-clock | ✓ | ✓ |
| 2 | provider-and-session-surfaces | | |
| 3 | backup-and-price-surfaces | | |
