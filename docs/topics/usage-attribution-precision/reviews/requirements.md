---
status: active
topic: usage-attribution-precision
subject: requirements.md
---

# Review log — usage-attribution-precision / requirements.md

## Round 1 — 2026-08-26

- Reviewed state: HEAD `5f2189550348a5a3f65fca42d6b92e8d07b2b5ac` plus the
  uncommitted document blob `42bc63dc0d7890d2f2ffd1be57162fa0842dd46f`
  (`architecture.md` `fa92fc1d2b9d220855c69dd6b9b1ff97b380a964`, `tasks.md`
  `46877d60171e1dd584743174f991626170360011` reviewed in the same pass).
- Reviewer: claude-code, independently reviewing the design authored by
  `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal document Review under `development-workflow`. Every claim the
  document makes about current behavior was checked against the source it
  names, rather than accepted as stated.
- Scope: the Evidence Baseline and its decisive observation, the Confirmed
  Decisions, the Non-Goals' consumption of the upstream switch topic, and the
  acceptance boundary's surface claims.
- Findings:
  - **[P1] R1-F1 — the Evidence Baseline's decisive observation measures
    something other than exactness.** The document reports
    ``EXACT_RUNS`` `= 0` and then reasons from it: "`EXACT_RUNS 0` is the
    decisive observation: `exact` is only reachable through
    `usage_runs.exact = 1`" (`requirements.md:34-40`). The metric behind that
    number is `internal/usage/usage.go:1484`:
    `"exact_runs": "SELECT COUNT(*) FROM usage_runs WHERE ended_at IS NULL"`.
    It counts runs that have **not ended** — open runs at the instant of
    measurement — and never reads the `exact` column at all. A value of `0`
    therefore means only that no `agentdeck run` was in flight when the
    baseline was taken, which is true on any idle machine and proves nothing
    about determinability. The document's own table contradicts the reading it
    draws: 128 events resolved `exact` (`requirements.md:31`), which is
    reachable only through `usage_runs.exact = 1` rows, so such rows plainly
    exist. Lines 41-42 sense the tension and warn the two "must not be
    collapsed into one percentage", but they never say what `exact_runs`
    actually counts, and the document still calls it decisive. The topic's
    central decision — `exact` is process-bound and therefore vanishingly rare
    — is independently and correctly supported by the `128 (0.16%)` row; it
    must rest on that, and this observation must be removed or restated with
    its real definition.
  - **[P1] R1-F2 — "no persisted quality column" is false for the resolution
    step this topic is about to change.** Confirmed Decisions state
    "Attribution stays derived at read time. No schema migration and no
    persisted quality column; the resolver already computes it per read"
    (`requirements.md:63-64`). `usage_session_routes` has carried
    `quality TEXT NOT NULL` since its creating migration
    (`internal/store/migrations.go:107`); `recordSessionRouteConn` hardcodes
    the literal `"estimated"` into every inserted row
    (`internal/usage/routes.go:402,416`); and the resolver does not compute
    that value but assigns the stored one straight through
    (`internal/usage/usage.go:2620`, `attribution.quality = route.quality`).
    This is not a wording slip: `architecture.md`'s target order promotes this
    exact step to `exact`, and the decision that must be made — derive the
    promotion at read time and let the stored column go unused, or change what
    is written and backfill the rows already stored as `estimated` — is
    foreclosed by a premise that says the column does not exist. See A1-F1 for
    the architecture-side half.
  - **[P2] R1-F3 — the acceptance boundary names a vocabulary that already
    exists on a different surface.** "`usage summary` distinguishes
    determinable, inferred, and unattributed totals"
    (`requirements.md:110-111`) reads as new work, but those three exact words
    are already implemented in `addPresentationQuality`
    (`internal/usage/presentation.go:361-367`), with definitions that differ
    from this topic's: `determinable` currently means `quality == "exact"`,
    and `unattributed` currently means `provider == "unknown"` — not the
    `historical` fallthrough this document renames. That accumulator feeds
    `Presentation`, whose only consumer is `internal/desktop/desktop.go`, not
    `usage summary`. The requirement therefore names the wrong surface and
    silently re-introduces a live term without reconciling the existing
    meaning. The behavioral consequence is recorded as A1-F3.
- Evidence: `usage.go:1484` is the sole definition of the `exact_runs`
  diagnose key, and `rg` finds no `EXACT_RUNS` identifier anywhere in the Go
  source. `StartRun` (`usage.go:1851`) is called only from `main.go:3482`, so
  the document's claim that only `agentdeck run` creates these rows is
  correct; the `exact` column defaults to `1` and is downgraded to `0` on
  client overlap (`usage.go:1885-1891`). `migrations.go:107`, `routes.go:416`,
  and `usage.go:2620` establish the persisted-quality path. The Non-Goals'
  description of the upstream switch topic was checked against the merged
  implementation and is accurate: only `route_effect=advance` and the
  mismatch-`unknown` case set `writeRoute`, while `retain` and `none` write no
  route.
- Completion gate: NOT_REQUIRED — a document review round records no
  completion evidence; the document boundary is crossed only on `PASS`.
- Verdict: REOPEN

### Repair disposition — 2026-08-26

- R1-F1 closed: the baseline now identifies the 128 exact events (0.16%) as the
  decisive observation and defines `exact_runs = 0` accurately as an idle-time
  count of open runs, explicitly excluding it from the quality argument.
- R1-F2 closed: the requirements acknowledge the persisted
  `usage_session_routes.quality` column and choose read-time derivation while
  leaving the legacy writer and existing rows unchanged.
- R1-F3 closed: the acceptance boundary now separates `usage summary`'s
  `exact`/`estimated`/`unattributed` contract from the desktop Presentation
  vocabulary and requires the desktop mapping to follow attribution quality.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.
