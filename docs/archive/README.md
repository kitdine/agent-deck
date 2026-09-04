---
status: active
created: 2026-07-22
---

# Archived Documents

Last updated: 2026-09-04

## 2026-09-01 retirement: the `v0.5.0` feature topics

The five topics the [`v0-5-0-contract`](topics/v0-5-0-contract/tasks.md)
assembly list selected retired together when that topic's closure task raised
`docs/specs/cli-design.md` to version 28. Each reached its terminal reviewed
state first; the move carries each topic's `reviews/` directory with it, because
the records live inside the topic.

`topics/desktop-app/` — the native macOS line. Six tasks: the wire contract,
the application foundation, the menu-bar experience, the WidgetKit extension,
unified signed distribution with the Homebrew Cask and direct DMG, and the
topic's own contract reconciliation. All six independently reviewed and
delivered.

`topics/work-signals/` — activity classification, workflow metrics, and
tool-call attribution on two first-class surfaces: the menu-bar `Sessions`
panel's three captured modules and `agentdeck usage signals`. A prototype task
plus six implementation tasks, all reviewed.

`topics/cli-error-classification/` — stable typed not-found codes, and no
storage text in a documented JSON contract. It is the source of `v0.5.0`'s one
compatibility break; the version-level announcement lives in `cli-design.md`'s
Error-Code Compatibility section, which names the old `runtime_error` and the
five codes replacing it.

`topics/switch-effectiveness-boundary/` — one client-neutral Hook delivery
operation persists every accepted Codex or Claude delivery before any route
effect, and effective-route effects stay event-specific. Three code-bearing
tasks reviewed. Its fourth task, `real-session-acceptance`, is `n/a`/`n/a`: the
operator waived it on 2026-08-26, so the procedure was never executed and no
review record exists. That distinction travels with the topic — tasks 1-3 rest
on reviewed automated evidence, the waived assumption rests on standing
operator experience.

`topics/usage-attribution-precision/` — determinable effective routes resolve
as `exact`, every event carries one of six attribution reasons, and the
calculable catalog base for `before_adoption` and `coverage_gap` is reported
separately from real provider spend, which no unattributed event may enter.
Three tasks reviewed.

Integration: none. `assemble` classified every selected topic and found nothing
to merge — no `feature/*` branch existed, because `main` is unprotected and was
`v0.5.0`'s feature line. No merge tree was produced and therefore no
`unit_kind: integration` evidence exists; the reasoning is in
[`topics/v0-5-0-contract/reviews/assemble.md`](topics/v0-5-0-contract/reviews/assemble.md).
No merge produced behavior that no source topic described.

The `v0-5-0-contract` topic itself stayed live through its closure review and
retired separately on 2026-09-04 after the exact `acb8384` tree was published
as stable `v0.5.0`. Its `assemble` and contract review records moved with it.


## 2026-08-16 retirement: two terminal design documents left in `docs/specs/`

`specs/terminal-rendering-design.md` and `specs/usage-interactive-viewer-design.md`
moved out of the live `docs/specs/` tree. Both were task designs whose owning
work — `terminal-presentation-remediation` and `usage-report-presentation` — had
already retired, and the topic structure adopted the same day makes explicit
that `docs/specs/` holds only contracts the product guarantees. The first
document said so about itself: "It is a design gate, not implementation
authority."

Their guaranteed behavior was verified present in the living authorities before
the move: the 260-cell canvas, one-to-four column bands, 60% balance and 15%
height-reduction fallback, target panel width, and KPI grids in
`docs/specs/cli-design.md:1341-1347`; interactive admission (text, TTY,
non-dumb `TERM`, 48x10, checked before raw mode), the shared keymap, terminal
restoration on Ctrl-C/EOF/cancel/resize/error, the no-color label contract, the
session browser's Escape hierarchy, and `COLUMNS`/ASCII degradation in
`docs/specs/cli-manual.md:422, 432, 528, 531-533, 644, 685, 696, 708`.

What retires with them is the design argument the living contracts do not
carry: rejected alternatives and their reasons, the verification-design risk
table, the column tie-breaker placements, and the delivered-implementation
status notes.

## 2026-08-12 retirement: terminal presentation remediation

`plans/terminal-presentation-remediation.md` and
`reviews/terminal-presentation-remediation/` retired together after all five
tasks completed fresh independent review and exact-state CEv1 gates. The work
delivered responsive labeled `session show --activity`, structured semantic
Usage and Session Detail, bounded responsive Session browser/viewer geometry,
height-derived acquisition, stable resize identity, and complete interactive
terminal lifecycle behavior. Its living contracts are in
`docs/specs/cli-design.md` and `docs/specs/cli-manual.md`; the two design
documents this entry originally pointed to were retired on 2026-08-16, recorded
below.

Final acceptance used a compiled current binary across synthetic isolated state
and isolated copies of approved real state, covering narrow through wide
geometries, color/no-color, live resize, browser/detail return, direct detail,
all documented exits and cleanup, JSON invariance, privacy, and source/database
hash invariance. The first aggregate L4 attempt exposed a `TERM=dumb` PTY test
harness hang; Task 4 was reopened, repaired without changing product behavior,
fully re-reviewed, signed, and commit-bound CEv1 VERIFIED before the complete
`release-verify` gate passed. Retirement closes the development plan only;
same-SHA preflight, RC publication, Homebrew distribution, and installation are
subsequent authorized delivery stages, not evidence that publication already
occurred.

## Why this directory exists

This directory holds documents that are no longer the current entry point for
development, but are not deleted because they still carry useful history:
implementation rationale, superseded contracts, or one-off investigation
records.

It mirrors the live structure. A topic retires as one directory under
`topics/<topic>/`, carrying its own reviews, because they live inside it.
Retired contracts go under `specs/`.

Entries below `2026-08-16` predate the topic structure and preserve the layout
they retired in: execution trackers under `plans/`, review records under
`reviews/<plan-topic>/`. Those paths are left as they were; they are history,
not a current convention.

Filenames keep the topic only; `status: historical` and `retired:` in each
document's frontmatter carry the rest.

## Criteria for archiving a document

Move a document here instead of leaving it `active` in `docs/topics/` or
`docs/specs/` once any of the following is true:

- It describes a system, contract, or plan that has been replaced by a newer
  active document or by the current code.
- It is a one-off investigation, incident record, or phased plan whose
  conclusions have already been absorbed into a living document.
- Leaving it in the main `docs/` tree risks being mistaken for the current
  source of truth.

## 2026-08-11 retirement: RC3 terminal UX acceptance remediation

`plans/rc3-terminal-ux-remediation.md` and
`reviews/rc3-terminal-ux-remediation/` retired together after all four tasks
reached Review PASS. The work repaired Claude model indexing, restored a bright
semantic Usage palette and always-present adaptive heatmap, rebuilt the root and
detail Session interactive experience, and replaced ordinary `session show`
tables with bounded labeled sections. Living behavior is now recorded in
`docs/specs/cli-manual.md` and
`docs/specs/terminal-rendering-design.md`. Final verification included
full tests, race, vet, compiled-current-binary isolated-real-state scans,
multi-width text and JSON checks, and a real PTY browser-to-Tokens lifecycle.
Retirement closes the development plan only: the immutable `v0.4.0-rc.2`
release is unchanged, while `v0.4.0-rc.3` still requires successful same-SHA
preflight evidence before tag and prerelease publication.
The first post-retirement preflight (`31490703746`) reopened the final task on
fresh-test and display-zone findings; Round 4 closed both and requires a new
same-SHA preflight rather than reusing that failed run.

## 2026-08-10 retirement: v0.4.0 contract closure

`plans/v0-4-0-contract.md` and `reviews/v0-4-0-contract/` preserve the
version-wide reconciliation after the session-experience and
usage-report-presentation feature lines passed review. The living CLI design
was raised to version 24 exactly once, the bounded session DTO dependency for
`desktop-wire-contract` was satisfied, and the active documentation index was
synchronized. This historical plan ends at `v0-4-0-contract` Review PASS. It
does not select an RC or stable release; that remains an explicit user-owned
decision after commit-bound technical preflight evidence is available.

## 2026-08-10 retirement: usage report presentation

`plans/usage-report-presentation.md` and
`reviews/usage-report-presentation/` retired together after all six tasks
passed independent review. The plan delivered shared responsive usage
primitives, fixed-baseline share bars, aligned detail, content-aware stats
layout, family-wide report alignment, and the explicit interactive stats
viewer. The active behavior now lives in `docs/specs/cli-design.md` and
`docs/specs/cli-manual.md`; version-wide reconciliation is preserved by the
archived `v0.4.0` contract closure. RC or stable publication remains a separate
user-owned decision.

## 2026-08-06 retirement: session experience

`plans/session-experience.md` and `reviews/session-experience/` retired together
after all six tasks passed independent review. The plan delivered normalized
session document time, aggregate scan progress, invocation usage detail,
sectioned `session show`, the interactive viewer, and the desktop-facing session
DTO boundary. The active behavior now lives in `docs/specs/cli-design.md` and
`docs/specs/cli-manual.md`; version-wide reconciliation is preserved by the
archived `v0.4.0` contract closure. RC or stable publication remains a separate
user-owned decision.

## 2026-08-04 retirement: `v0.3.0` plan batch

`runtime-provider-attribution`, `credential-key-and-cache-pricing`, and
`codex-auto-review-classification` (with their review directories) moved here
after their task evidence and reviews completed. The living contract is now
`docs/specs/cli-design.md` version 23 and `docs/specs/cli-manual.md`. Runtime
attribution delivered managed Codex/Claude usage Hooks, session-route
boundaries, and non-blocking concurrent-run attribution. Credential/pricing
delivered sealed key version 2 with version-1 compatibility plus disclosed
default five-minute cache-creation pricing. The classification probe established
`codex-auto-review` as a dedicated reviewer label but remained inconclusive
about billing; it ships no classification behavior and remains an unscoped
Backlog question pending authoritative billing evidence.

## Rules

- Archiving means **moving**, not deleting. Content is preserved as-is.
- Do not use this directory as a starting point for new work — start from
  `docs/README.md`.
- Read a file here only when you need historical context: why a decision was
  made, what an old contract looked like, or the background of a removed
  feature.
- If an archived document becomes relevant again, copy or rewrite its content
  back into `docs/`; do not treat this directory as a live source of truth.
- `docs/README.md` must not re-list individual archived files — this file is
  the index for archived material so the main doc index doesn't need to grow
  for documents nobody should open by default.

## 2026-08-03 retirement: the `v0.2.2` plan batch

`plans/usage-pricing-read-scalability.md`,
`plans/credential-and-pricing-hardening.md`,
`plans/session-show-stale-index.md`,
`plans/release-versioning-contract.md`, and their `reviews/` directories

All nine tasks across the four plans were delivered and independently reviewed.
The release is scoped but not tagged; these documents retire because their work
is finished and absorbed, not because `v0.2.2` shipped.

**Read scalability.** `usage-pricing-read-scalability` replaced the per-event
pricing lookups behind `usage stats`, full `doctor`, and `usage sessions` with
one shared bounded resolver, then paid off the acceptance debt against the
93,982,720-byte real usage database. The N+1 deferral recorded earlier in
`docs/README.md` is closed by this plan.

**Hardening.** `credential-and-pricing-hardening` synced the credential key's
directory entry so a crash cannot strand ciphertext with no recoverable key,
made an oversized 5xx price response retryable as the contract already promised,
and corrected `CheckHealth`'s documentation to match what a read-only WAL open
actually does. Two of its original six tasks — `key-id-derivation` and
`cache-creation-ttl-default` — are MINOR and moved to
`plans/credential-key-and-cache-pricing.md` for `v0.3.0`; they are not
unfinished work here.

**Message-only fix.** `session-show-stale-index` stopped `session show` from
leaking `sql: no rows in result set` and made it say whether the session index
is behind or the session is genuinely absent. Its review took three rounds: the
second round's attempt to avoid WAL sidecars via `immutable=1` was measured to
read a stale snapshot exactly when the index is behind, and was reverted in
favor of documenting the sidecars and pinning the live-WAL case with a test.

**Versioning contract.** `release-versioning-contract` recorded what each
version-number position means. That rule stays active in
`docs/specs/cli-design.md` and scopes both upcoming releases; the Release
Roadmap section of `docs/README.md` remains the live planning surface.

Three follow-up ideas were consciously left unscoped and are described in the
archived `session-show-stale-index` plan's Future Ideas: dedicated typed error
codes for the two causes, resolving activity detail through core usage state,
and having `usage stats` verify the session index before emitting a copyable
command.

## 2026-07-29 retirement: display timezone

`plans/display-timezone.md` and `reviews/display-timezone/`

All three tasks delivered and independently reviewed: `display-clock` extracted
the shared display zone, timezone-name, and instant-rendering helpers;
`provider-and-session-surfaces` localized provider selection times, session
bounds, and safe activity start times; and `backup-and-price-surfaces`
localized backup and price timestamps and completed the renderer sweep across
price-list provenance, usage session bounds, watch text, and the
`usage stats --activity` model range.

The rule the plan implemented stays active in `docs/specs/cli-design.md`
version 20 under "Time Representation", and the per-surface behavior stays
active in `docs/specs/cli-manual.md`. Text meant for a person renders instants
in the machine's zone to the second and names that zone; storage, JSON, and
NDJSON keep UTC RFC 3339. Two decisions are recorded in the archived plan
rather than the live contract: `version`'s `UTC Build Time` stays UTC because
it is immutable build identity, and `session search` gained no timestamp
because its result contract carries no instant. The possible search redesign
moved to the Backlog in `docs/README.md`. Three P3 review observations were
consciously left open and are described in
`reviews/display-timezone/backup-and-price-surfaces.md`.

## 2026-07-29 retirement: project attribution

`plans/project-attribution.md` and `reviews/project-attribution/`

All six tasks were delivered and independently reviewed: wrapper protocol
declaration, project identity, `agentdeck run` environment injection, operator
guidance, installable shell helpers, and the final attribution contract.
`shell-helpers` passed in review Round 7 after its shell-level harness was
repaired and exercised without a pipeline; `attribution-contract` passed in
Round 2 after its delivery-path and wrapper-kind contracts were reconciled.

The current contract remains active in `docs/specs/cli-design.md` version 20,
and the implemented command surface and user guidance remain active in
`docs/specs/cli-manual.md`. The unresolved question of whether the Claude app
loads project-scoped settings without restart remains an explicit Backlog item
in `docs/README.md`; it is not unfinished plan work.

## 2026-07-27 retirement: the provider wrapper routing plan

`plans/provider-wrapper-routing.md` and `reviews/provider-wrapper-routing/`

All seven tasks are delivered and independently reviewed, so the tracker has no
remaining work to track. Its conclusions already live in documents that stay
active: the contract in `docs/specs/cli-design.md` v15 ("Provider Wrappers",
"Owned Client Configuration Fields", "Selecting the Built-in Provider", and the
runtime provider dimension's route sentence), and the implemented surface in
`docs/specs/cli-manual.md` (`provider set-wrapper`, `provider use --via`, the
Claude switch advisories and their detection boundary, and the route metadata on
the provider dimension).

What shipped: every provider, including the built-in `official`, may carry one
wrapper URL; `provider use --via` routes a single switch through it without
storing an attachment; the built-in provider became selectable for Claude as
well as Codex; a completed Claude switch reports a restart advisory and any
unowned credential source that would override an `official` selection; and the
route travels into usage attribution as reported metadata that never becomes a
grouping key.

The review directory holds every round, including three reopened tasks —
`claude-writer-routes`, `route-composition` (a `doctor` drift regression found
end to end), and `switch-advisories` — plus the fix and re-review rounds that
closed them.

Two follow-ups deliberately left the plan rather than expanding it: the Claude
subscription/account switching idea and the display-side timezone decision, both
in `docs/README.md`'s Backlog.

## 2026-07-22 retirement: the phase-one CLI plan

`plans/agentdeck-cli.md` (was `docs/plans/2026-07-13-agentdeck-cli.md`)

This was the project's single execution tracker from the initial Go rewrite
through v0.1.0 and ten follow-up rounds. It was retired at roughly 950 lines,
with every task complete and independently reviewed.

It was retired for size, not for being wrong. A tracker that spans phase-one
bootstrapping, credential encryption, price catalogs, release automation, and
unscoped future ideas makes "is this still current?" expensive to answer, and
the convention that kept appending follow-up sections to it was the direct
cause. That convention has been changed — see `docs/README.md`.

Its role is now split:

- `docs/README.md` — the documentation index and execution baseline: what is
  active, what is open, what is deferred.
- `docs/plans/usage-scan-performance.md` — the one design that was
  still unimplemented when the plan was retired.
- `docs/plans/test-coverage.md` — test coverage work.

Delivered contracts described by the retired plan remain active in
`docs/specs/cli-design.md`, which was **not** retired: a
specification describes the currently-standing system and stays active as long
as that system stands, whereas a plan tracks finite work and retires when the
work is done. That spec also dropped its date prefix in the same pass — a date
implies a snapshot, but a contract is revised in place, so it now carries a
version and changelog instead.

## 2026-07-22 archive batch

**Legacy AI provider mode and session cost tracking** — superseded by
AgentDeck CLI (historical commit `3fcc121` held the removed implementation;
the replacement passed independent review). Both plan/spec pairs were already
marked `historical` in `docs/README.md` but had not actually been moved out
of `docs/plans/` and `docs/specs/`; this batch physically relocates them to
match that status and establishes this archive directory.

- `plans/ai-provider-mode.md` (was `docs/plans/2026-07-13-ai-provider-mode.md`)
- `specs/ai-provider-mode.md` (was `docs/specs/2026-07-13-ai-provider-mode-design.md`)
- `plans/ai-provider-session-cost.md` (was `docs/plans/2026-07-13-ai-provider-session-cost.md`)
- `specs/ai-provider-session-cost.md` (was `docs/specs/2026-07-13-ai-provider-session-cost-design.md`)

Current conclusions and requirements for the functionality these described
now live in `docs/README.md` and
`docs/specs/cli-design.md`.

## 2026-07-22 retirement: usage scan performance and progress

`plans/usage-scan-performance.md` and
`reviews/usage-scan-performance/` were retired together after all six tasks
passed independent review. The plan delivered the linear line-splitting fix,
the `(client, session_id)` usage-event index, delayed stderr progress,
parser-version reread context, stored-aggregate `--no-scan` reporting for stats
and summary, and a controlled same-fixture A/B remeasurement.

The final paired measurement recorded a 5.40x mean cold-scan improvement on the
same frozen fixture. Current behavior remains authoritative in
`docs/specs/cli-design.md` and `docs/specs/cli-manual.md`; the completed plan and
its review rounds remain here only as implementation and measurement history.

## 2026-07-22 retirement: usage stats display readability

`plans/usage-stats-readability.md` and
`reviews/usage-stats-readability/` were retired together after all five tasks
passed independent review. The plan delivered bounded text rankings, a recent
48-bucket trend window, the shared `--top` override, width-aware one-line
detail compaction, and a controlled same-snapshot A/B remeasurement.

The final paired measurement reduced `usage stats --period all` from 139 to
120 lines and `usage stats --period 30d --group-by hour` from 832 to 142 lines.
Current output contracts remain authoritative in `docs/specs/cli-design.md`
and `docs/specs/cli-manual.md`; the completed plan and review rounds remain
here only as implementation and measurement history.

## 2026-07-23 retirement: price catalog coverage

`plans/price-coverage.md` and `reviews/price-coverage/` were retired together
after all five tasks passed review. The plan delivered the content-derived
bundled `catalog_version` guard, the curated gap-fill as a separate input that
regeneration cannot drop, release-time regeneration from a pinned LiteLLM
commit with a reproducibility check, a no-network cold-start coverage test, and
an explicitly disclosed equivalent-estimate price for `gpt-5.3-codex-spark`.

Cold-start coverage on the same frozen snapshot went from **7.4% to 95.1%** of
tokens fully priced (2 models to 112). The residual is deliberate:
`codex-auto-review` stays unpriced as a probable pseudo-model, and the
`cache_creation_tokens` gap on the dotted Claude spellings is a
token-classification concern, not a catalog one. Both are carried forward in
the Backlog of `docs/README.md`, since archiving this plan would otherwise be
the only place they were written down.

Two things are worth reading here rather than rediscovering:

- **Why Spark is priced by an estimate rather than a vendor rate.** OpenAI
  publishes no rate for it, and every aggregator carrying a figure traces back
  to one unconfirmed row. The plan's `## Price Confidence` section records that
  analysis; the accepted resolution is the `equivalent_estimate` contract, whose
  standing rules now live in `docs/specs/cli-design.md`.
- **Why the bundled catalog's own effective date is a constant.** A curated
  model dated earlier than the catalog dragged the catalog's date back, and
  since same-layer catalogs are ranked by that date, a previously installed
  catalog then outranked the newer one on every shared model. Round-4 of
  `reviews/price-coverage/spark-gapfill.md` records the reproduction and the
  fix.

Review independence is documented unevenly and deliberately: tasks 1 and 3–5
were reviewed by a separate reviewer, while task 2's later rounds (4–6) were
performed in the same session as the implementation at the user's direction.
Each of those rounds states that caveat inline.

Current behavior remains authoritative in `docs/specs/cli-design.md` (version
14); the completed plan and its review rounds remain here only as
implementation, measurement, and decision history.

## 2026-07-23 retirement: high-value test coverage

`plans/test-coverage.md` and `reviews/test-coverage/` were retired together
after all six tasks passed review. The plan added focused regression tests for
evidenced high-risk gaps rather than chasing a coverage number: store open,
backup, and scan-lock boundaries; provider configuration persistence; usage
run-state attribution; provider backup redaction and config retention;
credential-vault initialization, non-overwrite, and recovery; and the read-only
`session.CheckHealth` diagnostic.

Review independence is documented deliberately: tasks 1–3 were reviewed by a
separate cold-context reviewer. Tasks 4–6 had their round-1 findings repaired in
the same session as the review, because the implementer subagents hit a session
limit before the fix round; each of those review rounds states that caveat
inline. Tasks 5 and 6 each drew a genuine `REOPEN` first (a recovery test that
proved "the vault works" rather than "it recovered *this* key", and a missing
sidecar assertion), both independently reproduced before repair.

One finding is carried forward rather than fixed, because this was a test-only
queue: `session.CheckHealth` opens the WAL-mode session index read-only yet
still materializes `sessions.sqlite3-wal`/`-shm` sidecars in the state root,
contradicting its "without creating, migrating, or changing it" doc comment. No
privacy or data-integrity boundary breaks — committed bytes are unchanged and
the sidecars are `0600` inside the `0700` root — so it lives in the Backlog of
`docs/README.md`, and a shipped test in `internal/session/doctor_test.go` pins
the current behavior so a future fix cannot land silently.

Current behavior remains authoritative in `docs/specs/cli-design.md` and
`docs/specs/cli-manual.md`; the completed plan and its review rounds remain here
only as implementation and decision history.

## 2026-07-24 retirement: repository test-gap production fixes

`plans/repository-test-gap-production-fixes.md` and
`reviews/repository-test-gap-production-fixes/` were retired together after all
four production blockers exposed by the active repository-wide test-gap
workflow were repaired and independently reviewed.

The signed commits stop retries for permanent LiteLLM catalog validation
failures (`571a0e3`), reject non-decimal multiplier syntax (`e934f00`), validate
resolved and explicit generator commits before catalog fetch (`c4abf87`), and
make session index transitions atomic with exact source ownership
(`3c80e4a`). Full tests, the relevant race suites, `go vet`, and diff checking
passed after the final production edit.

This retirement closes only the separate production-fix workflow. The original
repository-wide test-gap plan and its review trail remain active on
`audit/repository-test-gaps-20260723`. Resuming those four regression-test tasks
requires a fresh `new-baseline` authorization package; the old-baseline task and
audit commits must not be rebased or treated as authoritative delivery inputs.
No push was performed.

## 2026-07-26 retirement: repository-wide test-gap closure

`plans/repository-test-gaps.md` and
`reviews/repository-test-gaps/` preserve the complete fifteen-task behavioral
test-gap workflow. Eleven tasks were delivered in the earlier reviewed partial
delivery. Four production defects exposed by the remaining tests were repaired
in a separate workflow, after which the test-only work resumed from clean
baseline `4f614d34d09260a52df6bd333f6dad26134e96ac`.

The new-baseline audit completed at frozen reviewed head
`2571307d8410c2b4874bc1f8fb53fef91707c129`. Aggregate Review Round 3 passed
with 16 adequately protected modules, 15 independently reviewed tasks, no
exclusions or unresolved ledger entries, and no blocker. The four final
replacement-delivery task commits are, in order:

- usage `39650636fc92f884ecda5081f5d28ec22b583153`
- providermeta `3968d703fc5ed94378fbb917c187543655a1ffbb`
- genprices `02eec76513929fb321361858a00cc71d9ecad387`
- session `7168079230adf8bb1fdf05b2d563f1f1782023e1`

Full tests, full race, `go vet`, and atomic coverage passed at the replacement
task head. Total statement coverage is 81.9%; profile SHA-256
`0ae5afc81ecbcae30fb747ea60b41f16e3570c1a3ea13722093660751627f54b`.
Production code is unchanged.

Archive Review Round 1 found two documentation-only accuracy issues: the
current-state date and a sentence that prematurely implied pending gates had
proceeded. Both are corrected. Delivery Aggregate Review Round 1 passed the
pre-correction candidate, but the corrected content requires fresh Archive and
Delivery Aggregate Review Round 2 before retirement. Archive Review Round 2
passed; Delivery Aggregate Review Round 2 then found that the replacement
delivery and retirement dates still preceded the actual 2026-07-26
Asia/Shanghai commit and coverage evidence. All final-state and retirement
dates are now consistently 2026-07-26, while the 2026-07-25 audit events remain
historically unchanged.

Archive and Delivery Aggregate Review Round 3 both passed, and the reviewed
delivery-state resolver authorized the final documentation commit, plan
retirement, target fast-forward, and cleanup. Final commit
`9bb88477c9655a08a0dfd26bb00e20d433db251e` was pushed to `origin/main`.
After the push was verified, the temporary task, repair, delivery, and audit
worktrees and refs were removed. The archived plan, task review records, and
authorization-package identities remain the durable workflow record.


## 2026-08-02 retirement: doctor quick diagnostics

`plans/doctor-quick-diagnostics.md` and
`reviews/doctor-quick-diagnostics/` preserve the one-task `v0.2.1` quick/full
diagnostic boundary fix found while validating installed `v0.2.1-rc.2`. Quick
doctor still checks price-catalog availability but reserves
`PriceDiagnostics`, `price_provenance`, and `unpriced_models` for `--full`.
Round 1 reopened one full-mode regression-coverage gap; Round 2 passed after a
test-only fixture and assertion directly protected result code
`unpriced_models`. The production boundary did not change during the fix.
Real-state quick acceptance remains unverified because the managed approval
reviewer rejected the read-only command with its own request-format error.
Full-doctor and `usage sessions` pricing-read scalability remain active
`v0.2.2` work. No commit, push, release, or local installed-binary update was
performed as part of this retirement.

## 2026-07-31 retirement: shell integration

`plans/shell-integration.md` and `reviews/shell-integration/` preserve the
eight-task shell-attribution workflow: explicit setup/status/remove/env
lifecycle, safe managed startup-file blocks, activation and per-client
eligibility status, installation onboarding, cross-shell acceptance,
route-change advisories, switch-time setup, and the final living-contract
migration. Every task passed independent review; Task 8 required three review
rounds before closing its lifecycle, status-vocabulary, best-effort marker, and
cross-plan specification-version findings.

Stable behavior is authoritative in `docs/specs/cli-design.md` version 21 and
`docs/specs/cli-manual.md`. Exact benchmark results, rejected alternatives,
implementation evidence, and review history remain in the archived records.
Task 8 itself was L0 documentation work: this retirement does not claim release
readiness. L4 and real RC2 artifact installation remain delivery gates, and the
two `v0.2.2` plans are now unblocked. No commit or push was performed as part
of retirement.

## 2026-08-20 supersession: work-signals first design pass

`work-signals-superseded-2026-08-20/` preserves the first design pass at the
`work-signals` topic — `architecture.md`, `tasks.md`, both `ux/` surfaces, and
the four review records covering fourteen rounds — none of which was ever
committed.

It is archived rather than deleted because the review rounds are the evidence
for why the topic was restarted. The rounds converged on document-internal
consistency while the design itself rested on decisions the user had never been
asked about: the classification unit, the privacy boundary around tool
arguments, cost attribution, and the CLI's shape. Six of the fourteen rounds
repaired contradictions introduced by earlier rounds of the same pass.

The user directed a full restart on 2026-08-20 and made those decisions
explicitly. The replacement document set derives from them and from the
CodeBurn implementation the user named as a reference. Nothing in the archived
set is authoritative; `docs/topics/work-signals/` is.
