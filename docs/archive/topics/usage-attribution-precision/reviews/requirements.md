---
status: historical
topic: usage-attribution-precision
subject: requirements.md
retired: 2026-09-01
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

## Round 2 — 2026-08-26

- Reviewed state: HEAD `2b9dd2bd43d130ea31c6065df11275756e38605f` plus committed
  document blob `0abe112cccb025e40e69ac6c531e93a3621f10be` (`architecture.md`
  `2174860af3646359e86f7cfaa8ace1611dbd1b14`, `tasks.md`
  `1651a86c465f42b049bd75992726c873ea247862` re-reviewed in the same pass).
- Reviewer: claude-code, independently re-reviewing the Round 1 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; each finding checked
  against the source its claim names, then a check for consequences the repair
  introduces.
- Scope: R1-F1 through R1-F3, and any newly blocking repair regression.
- Findings:
  - **R1-F1 — CLOSED.** The baseline row is relabelled "Open `agentdeck run`
    processes (`usage diagnose` key `exact_runs`)", the decisive observation now
    rests on the 128-event `exact` count, and the text states outright that the
    key counts `ended_at IS NULL` rows and is retained as idle-state context
    rather than as attribution evidence.
  - **R1-F2 — CLOSED.** The decision now reads "Attribution quality is derived
    at read time", acknowledges that `usage_session_routes.quality` is persisted
    and that every writer stores `estimated`, and states that the resolver stops
    treating that value as its verdict with no migration, write-path change, or
    backfill. The Non-Goal was updated to match.
  - **R1-F3 — CLOSED, and better scoped than the finding asked.** The acceptance
    boundary now separates the two vocabularies instead of conflating them:
    `usage summary` reports `exact`/`estimated`/`unattributed` plus a reason
    split, while the desktop `Presentation` contract maps those qualities to
    `determinable`/`inferred`/`unattributed` and is explicitly forbidden from
    using `provider = unknown` as an alternate quality test — the inversion
    Round 1 recorded as A1-F3.
  - **[P1] R2-F1 — the recomputation decision authorizes exactly the case
    A2-F1 shows it should not.** "Existing data is re-attributed on upgrade, so
    cost totals change. The release notes must state this"
    (`requirements.md:70-72`) treats re-attribution as uniformly an improvement
    and reduces the obligation to disclosure. It holds for the great majority of
    the corpus, but not for routes recording a transition the running session
    provably did not adopt: those are re-attributed to a *higher* confidence on
    evidence this project has already ruled wrong. Measured on the live store,
    that is 4 route rows and 534 of 12,929 Claude events (4.13%), which move
    from `estimated` to `exact` and, on the desktop, from `inferred` to
    `determinable`. This document owns the decision that authorizes the
    recomputation and owns the acceptance boundary, so it is where the limit
    belongs: re-attribution may raise confidence only where the evidence
    supports the higher claim. `architecture.md` owns the mechanism; see A2-F1
    for the measured classification and the read-time rule that separates the
    cases.
- Evidence: `usage.go:1484` remains the sole definition of the `exact_runs`
  key and the repaired text now describes it correctly. `migrations.go:107`,
  `routes.go:402,416`, and `usage.go:2620` still establish the persisted-quality
  path the repaired decision now acknowledges. `presentation.go:361-367` is the
  existing `determinable`/`inferred`/`unattributed` mapping the acceptance
  boundary now names. The A2-F1 counts and their per-group classification are
  recorded in `reviews/architecture.md` Round 2.
- Completion gate: NOT_REQUIRED — a document review round records no completion
  evidence; the document boundary is crossed only on `PASS`.
- Verdict: REOPEN

### Repair disposition — Round 2 — 2026-08-26

- R2-F1 closed: recomputation may raise confidence only when prior effective
  state proves the route was adopted; legacy keyed rotation/removal stays
  `estimated`, coarse cutoff rules are forbidden, and release-note plus
  acceptance wording now covers quality-distribution changes.
- Verdict: REOPEN — repair complete, awaiting independent Re-review.

## Round 3 — 2026-08-26

- Reviewed state: HEAD `2b9dd2bd43d130ea31c6065df11275756e38605f` plus the
  uncommitted document blob `647c0963d5a012b0bb513645042747461d921080`
  (`architecture.md` `6a43f5f3b4cf6e71042390265e542a647919dda2`, `tasks.md`
  `9f08908761ae0cc63a593190a5cbdbe94aa17fff` re-reviewed in the same pass);
  subject digest `085936427f11e06ad6081ff8d85b7314221bc3caa4ee712a9d26b0a278bcf7b8`.
- Reviewer: claude-code, independently re-reviewing the Round 2 repair authored
  by `codex`; this workflow turn kept production code, tests, and configuration
  read-only.
- Method: Formal REREVIEW under `development-workflow`; the Round 2 finding was
  checked against the repaired decision and acceptance boundary, and the
  numbers both now assert were re-measured against the operator's live store.
- Scope: R2-F1, and any newly blocking repair regression.
- Findings:
  - **R2-F1 — CLOSED.** The decision is now "Recomputation is evidence-bounded
    and must be disclosed": confidence may rise only where the effective-state
    evidence supports the stronger claim, a legacy Claude `ConfigChange`
    recording rotation or removal from an already-keyed session stays
    `estimated`, and "a cutoff or other coarse provenance rule may not stand in
    for that classification" — which is the substitution Round 2 rejected. The
    acceptance boundary carries the matching criterion and names the six-group
    table in `reviews/architecture.md` Round 2 as the reference classification.
    Disclosure was widened from cost totals alone to quality distributions,
    which is the part of the recomputation this topic actually changes.
- Evidence: the repaired decision is at `requirements.md:70-77` and its
  acceptance criterion at `:120-125`. The Non-Goal still reads "no change to the
  existing route-quality writer", which stays consistent with the read-side-only
  `routes.go` scope the architecture and task 2 now define. Re-measured on a
  read-only copy of the live store: the fifteen `ConfigChange` rows with no
  preceding session route resolve through the provider timeline at session start
  as ten `official -> official`, three `official -> sssaicode`, and two
  `cubence -> cubence`, so all fifteen are legitimately `exact`; with the nine
  same-provider and three first-key rows that is 27 `ConfigChange` rows at
  `exact` and 4 at `estimated`, matching what both documents now assert. The
  store copy was inspected read-only and deleted afterwards.
- Completion gate: VERIFIED — CEv1 gate
  `usage-attribution-precision:requirements.md` records this PASS against exact
  content state
  `085936427f11e06ad6081ff8d85b7314221bc3caa4ee712a9d26b0a278bcf7b8`.
- Verdict: PASS
