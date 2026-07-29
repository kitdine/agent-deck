---
status: historical
plan: display-timezone
task: backup-and-price-surfaces
---

# Review log — display-timezone / backup-and-price-surfaces

## Round 1 — 2026-07-29

- Reviewed state: base `5635162`, uncommitted `backup-and-price-surfaces`
  implementation, its test additions, the CLI manual update, and the
  development-status update.
- Reviewer: Claude.
- Scope: backup list/inspect/create text timestamps, price history/status/list
  provenance timestamps, the required instant-bearing renderer sweep, the new
  `renderDisplayTimeWithZone` and `displayZoneName` helpers, the new
  `backup.create` text path, JSON/NDJSON preservation, focused and existing
  renderer tests, CLI manual accuracy, plan acceptance, and documentation-index
  state.

### Findings

- [P2] The required renderer sweep missed one instant-bearing text surface.
  `usage stats --activity` renders `MODEL ACTIVITY` through
  `cmd/agentdeck/usage_stats_text.go:550-551`, which concatenates
  `activity.FirstAt` and `activity.LastAt` directly into the line
  `range <first> - <last>`. Those values are canonical stored UTC written by
  `internal/usage/usage.go:2131` (`parsed.UTC().Format(time.RFC3339Nano)`), so
  the line prints a raw UTC instant with nanoseconds, names no zone, and is not
  second-precision. The same screen already prints the machine zone in its
  metadata line (`cmd/agentdeck/usage_stats_text.go:296`), so one stats report
  now shows a localized header beside an unlocalized UTC range — exactly the
  inconsistency this plan exists to remove. It also contradicts the global rule
  this task just wrote into `docs/specs/cli-manual.md`, which states that
  human-facing text renders instants in the machine zone to the second and
  always names the zone. The task's acceptance criterion requires sweeping for
  any missed instant-bearing text renderer and either localizing it or
  recording why it stays UTC; the development note lists price-list provenance,
  usage sessions, watch, and the deliberate `version` UTC exception, but not
  this surface, so it is neither localized nor justified. Either localize both
  values through the shared display clock and name the zone, or record why this
  particular range stays UTC.

  Evidence: a temporary in-package probe rendering `modelActivityLines` with
  the display zone pinned to `UTC+8` produced
  `range 2026-07-20T16:00:00.123456789Z - 2026-07-20T18:30:00.987654321Z`. The
  probe was deleted after the observation; no product or test file was
  modified by this review.

- [P3] `renderBackupCreateText` (`cmd/agentdeck/main.go:2877-2884`) fails the
  command when its type assertions do not hold, so a presentation problem would
  surface after the backup archive was already written successfully. The single
  caller (`cmd/agentdeck/main.go:1741`) always passes the expected shape, so
  this is defensive risk rather than a live defect, but a mutation that has
  already changed disk state should prefer degrading its text over returning a
  non-zero exit.

- [P3] `renderDisplayTimeWithZone` guards only the `string` branch. A zero
  `time.Time` — for example a manifest decoded without `created_at` — renders as
  `0001-01-01 08:00:00 UTC+8` rather than staying recognizably absent. The
  string path returns unparseable input unchanged, so the two entry points are
  asymmetric at their degenerate input.

- [P3] The CLI manual gains per-surface notes for this task's surfaces
  (`backup create/list/inspect`, `price history/status/list --verbose`,
  `usage sessions`, `watch`) but not for the task-2 surfaces
  (`provider current`, `provider status`, `session list`, `session show`,
  `session show --activity`). They are covered by the new global rule, so the
  contract is not wrong, only unevenly documented.

### Test review

- `TestBackupAndPriceTextSurfacesUseDisplayZone` asserts both directions for
  every case: the localized value and zone label must appear in text, the
  stored UTC instant must disappear from text, and the same data rendered as
  JSON must still contain the stored UTC instant and must not contain the
  localized form. That two-sided assertion is what makes the JSON invariant
  actually protected rather than assumed.
- `TestInstantBearingTextRendererSweepUsesDisplayZone` covers the three
  surfaces the sweep did find, using the real envelope writers
  (`writePriceEnvelope`, `writeUsageEnvelope`) and `renderWatchText` rather than
  re-implementing rendering, so it would fail if those paths regressed.
- Both tests pin the zone through `usePinnedDisplayZone`, so they do not depend
  on the host zone.
- `TestExtensionAndBackupTextContracts` was updated to expect the localized
  `created` line while its JSON assertion continues to require UTC; the
  contract test was tightened rather than loosened.
- Gap consistent with the P2 above: no test covers the `usage stats --activity`
  range line, which is why the sweep omission was not caught.

### Strengths

- The `backup.create` text path is added as an explicit branch before the
  generic mutation renderer and preserves the existing completion line, quiet
  behavior, and `mutationResource` resolution, so the pre-existing contract is
  extended rather than replaced.
- `renderDisplayTimeWithZone` refuses to append a zone to a value it could not
  parse, so an unparseable stored string cannot acquire a false zone label.
- Table surfaces name the zone once in the header cell and detail surfaces name
  it after the value, matching the rule documented in the manual.
- No storage, migration, schema, JSON, NDJSON, or GUI contract fixture is
  touched; the diff is confined to `cmd/agentdeck` renderers, their tests, and
  documentation.
- The `version` `UTC Build Time` exception is reasoned and recorded in the
  manual rather than silently skipped.

### Independent verification

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./...` — PASS (all 16 packages).
- Independent renderer sweep across `cmd/agentdeck/main.go`,
  `cmd/agentdeck/usage_stats_text.go`, and `internal/output`: every remaining
  `time.Parse`/`Format` site was traced to either the shared display clock, the
  `usage stats` range and bucket labels (whose stored strings already carry the
  machine-zone offset from `internal/usage/usage.go:2727` and
  `usage.go:2751`, so they render local wall-clock correctly), the deliberate
  `version` UTC exception, or a non-display use. The single unexplained
  surface is the P2 above.
- `provider.recover`, `credential.list`, and the cache-session block were
  checked and carry no rendered instant, so they correctly gain no zone.
- Manual accuracy spot-checked against the implementation for each surface it
  newly documents.

**Verdict: REOPEN.** Product behavior, JSON preservation, and regression
coverage for the surfaces in scope are sound, but the task's sweep acceptance
criterion is not yet met: one instant-bearing text renderer is neither
localized nor recorded as a documented UTC exception.

## Round 2 — 2026-07-29 fix

- Addressed the Round 1 P2 sweep-completeness finding.
- `cmd/agentdeck/usage_stats_text.go` now routes the `MODEL ACTIVITY` range
  through the new `statsActivityRange` helper, which localizes both bounds with
  the shared `renderDisplayTime` and appends `displayZoneName()` once for the
  pair. Both values must parse before either is rewritten, so a range with one
  unparseable bound keeps its stored values and gains no zone label — the same
  no-fabricated-zone rule the other detail surfaces follow.
- `docs/specs/cli-manual.md` now records the `MODEL ACTIVITY` range per surface
  on the `usage stats` row, so the sweep list is complete: backup, price
  history/status/list provenance, usage sessions, watch, model activity, and
  the deliberate `version` UTC exception.
- `usage.StatsModelActivity` and its JSON encoding are unchanged; the fix is
  confined to the text renderer.
- Regression coverage added to `TestInstantBearingTextRendererSweepUsesDisplayZone`:
  `usage stats model activity range` (RED before the fix — the line still
  concatenated the stored UTC values) and `usage stats model activity keeps
  unparseable range unchanged` (green before the fix, protecting the existing
  boundary).
- Verification: `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test
  -count=1 -mod=vendor ./cmd/agentdeck` — PASS.
- The three Round 1 P3 observations were intentionally left unchanged. The
  `renderBackupCreateText` failure mode and the zero-`time.Time` rendering each
  need an explicit expected-behavior decision rather than a mechanical edit,
  and back-filling per-surface manual notes for the task-2 surfaces would
  reopen a task that already passed re-review.
- Review remains unchecked pending independent re-review.

## Round 3 — 2026-07-29 re-review

- Reviewed state: the Round 1 implementation plus the Round 2 fix to
  `cmd/agentdeck/usage_stats_text.go`, its two new subtests, and the manual,
  plan, review-log, and documentation-index updates.
- Reviewer: Claude.

### Round 1 finding closure

- [P2] **CLOSED.** `modelActivityLines` now delegates to `statsActivityRange`,
  which localizes both bounds through the shared `renderDisplayTime` and names
  the zone once for the pair. An independent re-sweep of every instant-bearing
  field reference in `cmd/agentdeck/main.go` and
  `cmd/agentdeck/usage_stats_text.go` found no remaining renderer that prints a
  stored instant without the display clock; the only surviving raw reference is
  the `activity.FirstAt != ""` presence check at
  `cmd/agentdeck/usage_stats_text.go:550`. The CLI manual now carries the
  per-surface note on the `usage stats` row, so every localized surface and the
  deliberate `version` UTC exception are recorded.
- [P3 ×3] Unchanged by design, with the reasons recorded in the plan and in
  Round 2 above. None of them blocks: the `renderBackupCreateText` assertion
  has one caller that always passes the correct shape, the zero-`time.Time`
  path needs an expected-behavior decision, and the task-2 manual notes are
  already covered by the global rule. They remain open observations rather than
  silently dropped findings.

### Fix quality

- The guard order is correct: both bounds are parsed before either is
  rewritten, so a range cannot be half-localized or carry a zone label that
  describes only one of its values.
- Boundary behavior verified with a temporary in-package probe (deleted after
  the observation, no product or test file modified): an empty `LastAt` and a
  fully unparseable pair both render unchanged with no zone, while a valid pair
  renders `range 2026-07-21 00:00:00 - 2026-07-21 02:30:00 UTC+8`.
- The committed subtests would fail under the three mutations that matter:
  dropping the zone label, localizing only one bound, and appending a zone to
  an unparseable value.
- `usage.StatsModelActivity` and its JSON encoding are untouched; the fix is
  confined to 14 lines of text rendering.

### Findings

- No remaining medium-or-higher findings.
- No new correctness, regression, security, privacy, performance, or
  documentation-contract issue found.

### Independent verification

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run
  TestInstantBearingTextRendererSweepUsesDisplayZone -v` — PASS.
- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck` — PASS.
- The Round 2 full `./...` run remains bound to this content state: every
  changed Go file predates that run and nothing but documentation changed
  afterwards.
- `go vet -mod=vendor ./cmd/agentdeck` clean and `git diff --check` passes.

### Repository-state observation

Untracked files unrelated to this plan appeared in the repository root during
the session (`.claude-plugin/`, `config.json`, `events.jsonl`, `labels/`,
`sessions/`, `statuses/`, `views.json`). They are a `craft-workspace` tool's
workspace state, not output of this task or its tests. This review neither
removed, ignored, nor staged them.

**Verdict: PASS.**
