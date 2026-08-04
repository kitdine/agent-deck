---
status: active
plan: credential-key-and-cache-pricing
task: cache-creation-ttl-default
---

# Review log — credential-key-and-cache-pricing / cache-creation-ttl-default

## Round 1 — 2026-08-04

- Reviewed state: `HEAD` `882da86adbc23fa685cf882d85b16a828bd332c2`; reviewed product/test file SHA-256 values: `internal/usage/usage.go` `0b31ea68bf584ade37ce73918e1c9a7b5040da4b5a5022902246935f3f1e7e5c` and `internal/usage/usage_test.go` `f0d131526b0138409edbfe81e9edaa6e6cbc2cb407a517b02928152643ca9640`.
- Reviewer: Codex (review-only round; no product code, tests, or configuration changed).
- Scope: data-driven five-minute cache-creation default, conservative contradictory-shape handling, disclosure propagation, pricing completeness, consumer consistency, and regression value.

### Findings

**M1 — `usage stats` drops the default-TTL disclosure and presents estimated costs as ordinary complete costs.** `Calculate` correctly emits `defaulted 5m cache creation TTL` for the positive-total/zero-breakdown shape, and `summarizeEvents` forwards that marker into summary and session warnings. The independent `Stats` path, however, passes the `Result` only into `statsAccumulator.add`; neither `StatsReport` nor its text/JSON rendering retains `Result.Warnings`. An affected stats event therefore becomes fully priced and disappears from `unpriced_models`, but its cost is indistinguishable from a cost backed by a reported TTL breakdown. This violates the task's explicit requirement that the default be disclosed rather than silently guessed, and the acceptance rule that an affected event be marked as resting on the default TTL.

**Required fix:** propagate the default-TTL disclosure through `usage stats` without adding it to any grouping key. Use the existing warnings vocabulary and ensure both text and JSON stats output expose the marker while preserving the new cost, coverage, and `unpriced_models` behavior.

**Required regression coverage:** add an end-to-end stats case for a positive total with two zero TTL buckets that asserts the five-minute cost, 100% pricing coverage, absence from `unpriced_models`, and disclosure in both text and JSON. Retain direct calculation coverage for a normal reported breakdown (no marker), a partial non-zero breakdown (remainder unpriced), and zero-total-with-breakdown behavior. Add the plan-required identical-shape assertion for dotted and hyphenated model spellings so the data-driven boundary is protected rather than inferred from implementation inspection.

**M2 — the required recomputed cold-start coverage figure is not recorded.** The task acceptance explicitly requires recomputed coverage figures in the plan evidence, and the development section says only that affected events are now fully priced. It does not record the recomputed percentage or counts replacing the baseline 95.1% figure, so the acceptance evidence is incomplete.

**Required closure:** recompute the cold-start coverage from the same aggregate-only baseline method, record the resulting numerator, denominator, and percentage in the task's development/fix evidence, and update any pinned coverage assertion from that result rather than adjusting it by hand.

### Evidence

- Full-context task diff and CodeGraph call-path review of `Calculate -> summarizeEvents` and `Calculate -> Stats -> statsAccumulator.add`.
- `internal/usage/usage.go:482-486` creates the marker; `internal/usage/usage.go:3143-3148` forwards it only for summary/session aggregation; `internal/usage/usage.go:2696-2727` feeds stats accumulators without retaining warnings.
- `StatsReport` has no warnings field, and the `usage.stats` text/JSON renderers therefore have no route for the marker.
- New tests cover direct default and partial shapes plus summary warning aggregation, but do not cover stats disclosure, dotted-versus-hyphenated equivalence, or the stated zero-total-with-breakdown invariant.
- The task's Development Evidence contains no replacement coverage percentage or counts for the baseline 95.1% expectation.
- Broad verification stopped after the medium contract finding had a decisive source reproducer. Recorded development test evidence was not rerun.
- Verdict: REOPEN

### Repair response — 2026-08-04

- `StatsReport` now collects calculation warnings once per report; text renders
  a `WARNINGS` section, while JSON retains the warning in both report data and
  the standard output envelope.
- Regression coverage proves the actual `usage stats` CLI text/JSON route, the
  dotted and hyphenated default-TTL spellings, the zero-total reported-breakdown
  invariant, and default-TTL handling through the bundled cold-start fixture.
- The frozen aggregate-only baseline was recomputed from the reviewed counts:
  `5,007,775,405 / 5,259,503,075 = 95.213851%`, rounded and recorded as 95.2%.
- Targeted `internal/usage` and `cmd/agentdeck` tests plus the full vendored
  suite passed. Review remains **REOPEN** pending independent re-review.

## Round 2 — 2026-08-04

- Reviewed state: `HEAD` `882da86adbc23fa685cf882d85b16a828bd332c2`; changed since Round 1: `internal/usage/usage.go` SHA-256 `c9dcf57d76d7eab58a0bf687bf6fe6ffd63838ef377a645bcae0117eb0da405c`, `internal/usage/usage_test.go` `57d80c4afb8ec793ef12ae7824705a7293348d274da13f2ee75466fb9e564fd3`, `internal/usage/bundled_coverage_test.go` `3b8e5ce7642cec09802ed4e38df3a4a47aeb26719b6ff19b2476c2bce7dba94f`, `cmd/agentdeck/main.go` `036a8702f5087e97b51b6003d33007b1d1c7debf14966467e942efea7f446a08`, `cmd/agentdeck/main_test.go` `07f83b8b5c5af3dd45108874e831c2cde252a52664534d0af1c695c9008b1c0a`, `cmd/agentdeck/usage_stats_text.go` `174f1a9cd437220ef8edad55e48814817ea438bcb76c3f4ec866027c419f1bb4`, and `cmd/agentdeck/usage_stats_text_test.go` `6e39c5bd0d9225ed44dd8c46e89663cdd10894e51739b460cfd5679067d11a62`.
- Reviewer: Codex (re-review-only round; no product code, tests, or configuration changed).
- Scope: Round 1 M1 stats disclosure propagation, M2 frozen-baseline recomputation, and the required regression scenarios.
- M1 closure: `Stats` now deduplicates and sorts calculation warnings, `StatsReport` carries them, the CLI includes them in the standard JSON envelope and report data, and the text renderer emits a bounded `WARNINGS` section. Service and real CLI-route tests assert the five-minute cost, complete coverage, JSON disclosure, and one text marker.
- M2 closure: the plan records the frozen-baseline numerator increase, denominator, exact percentage, and rounded 95.2% result; `docs/README.md` is updated consistently.

### Remaining finding

**M3 — the claimed dotted-versus-hyphenated regression test does not compare equivalent spellings with identical token shapes.** `TestCalculateDefaultsCacheCreationTTLRegardlessOfModelSpelling` runs `claude-haiku-4.5` and `claude-opus-4.8`: these are two different models and both names use dotted spelling. The cold-start fixture contains `claude-opus-4-8` and `claude-opus-4.8`, but deliberately gives the hyphenated row a reported non-zero TTL breakdown and the dotted row a defaulted zero breakdown. Both tests can therefore pass if a future implementation branches on the exact model spelling, which is the regression the acceptance criterion explicitly requires the suite to prevent.

**Required closure:** use one equivalent dotted/hyphenated pair, for example `claude-opus-4.8` and `claude-opus-4-8`, with the exact same positive cache-creation total and zero TTL buckets. Assert identical five-minute costs, no unpriced component, and the same disclosure marker for both. If the bundled cold-start fixture is intended to supply this protection, give the pair identical token shapes there and assert the same observable pricing/disclosure behavior rather than only per-model priced status.

### Test-review evidence

- Severity: medium.
- Location: `internal/usage/usage_test.go:135-147` and `internal/usage/bundled_coverage_test.go:34-94`.
- Behavior risk: model-name or punctuation branching can re-enter the resolver while the suite stays green, violating the task's data-driven boundary.
- How the current tests can miss the defect: no test invokes equivalent dotted and hyphenated spellings with identical inputs.
- Recommended verification: run the corrected focused spelling test, then the task's targeted packages and L2 full vendor suite after the final test edit.
- Broad verification stopped after this medium regression-value gap was established from the exact test inputs. Repair verification recorded by development was not rerun.
- Verdict: REOPEN

### Repair response — Round 2, 2026-08-04

- `TestCalculateDefaultsCacheCreationTTLRegardlessOfModelSpelling` now compares
  `claude-opus-4.8` with `claude-opus-4-8` using the identical positive total
  and zero TTL buckets, asserting equal complete costs, no unpriced component,
  and the same default-TTL warning.
- The bundled cold-start fixture gives the same pair the same default-TTL
  token shape and asserts summary disclosure, preventing punctuation-specific
  behavior from passing merely as a priced model.
- Targeted `internal/usage` and `cmd/agentdeck` tests passed. Review remains
  **REOPEN** pending independent re-review.

## Round 3 — 2026-08-04

- Reviewed state: `HEAD` `882da86adbc23fa685cf882d85b16a828bd332c2`; changed since Round 2: `internal/usage/usage_test.go` SHA-256 `7f7084036b786cd677fb9a54e8988b89d8851151e7dfe075fd9c8b16b745f42b`, `internal/usage/bundled_coverage_test.go` `777079fe4cf46e03f0c760751b3b5575108bb724fe1048ef3685d2e46dc10bdb`, and `docs/plans/credential-key-and-cache-pricing.md` `98b81c0ab0f80346e40b64535b967428f514eef07e63cdfe5afad621c491d699`. Reviewed product and other repair files retain their Round 2 hashes.
- Reviewer: Codex (re-review-only round; no product code, tests, or configuration changed).
- Scope: Round 2 M3 equivalent-spelling regression coverage and the task's L2 verification gate.
- M3 closure: `TestCalculateDefaultsCacheCreationTTLRegardlessOfModelSpelling` now compares `claude-opus-4.8` and `claude-opus-4-8` with identical positive cache-creation totals and zero TTL buckets. It asserts the five-minute catalog cost, equal provider cost, equal empty unpriced components, and equal disclosure warnings. The bundled cold-start fixture also gives the equivalent pair the same defaulted token shape.
- New product findings: none.

### Remaining findings

**M4 — the new end-to-end CLI disclosure test selects no event in non-UTC local timezones.** The fixture stores its event at `2026-07-20T01:00:00Z`, while `usage stats --from 2026-07-20 --to 2026-07-20` resolves through the process display location. In the current `America/Los_Angeles` environment the selected range begins at `2026-07-20T07:00:00Z`, so the event is outside the range and the report has zero events and no warning. The test therefore fails deterministically instead of protecting the intended CLI contract.

**Required closure:** make the test's display timezone deterministic with the existing UTC clock helper, or place the event at an instant guaranteed to fall inside the explicitly selected local day. Keep the assertion on the real CLI text and JSON routes.

**M5 — the checked-in GUI JSON contract fixture was not updated for the new `usage.stats.data.warnings` field.** `TestIsolatedEndToEndFlow` fails at `e2e_test.go:284` because the observed command schema contains the new report-data field while `cmd/agentdeck/testdata/phase7/gui-json-contract.json` still describes only the pre-change data shape. This leaves the L2 package gate red and the machine-readable contract artifact stale.

**Required closure:** regenerate the fixture through the test's documented `UPDATE_AGENTDECK_GOLDEN=1` path, inspect the resulting change to confirm it adds only the intended `usage.stats` data warning shape, then rerun the isolated end-to-end test.

### Verification

- `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/usage ./cmd/agentdeck` -> FAIL; `internal/usage` passed and `cmd/agentdeck` failed.
- Structured failure isolation identified `TestUsageStatsDisclosesDefaultedCacheCreationTTL` and `TestIsolatedEndToEndFlow`.
- Focused evidence confirmed the CLI test's empty report is caused by the local-date/UTC fixture mismatch, and the isolated flow reports `command contracts differ` at `e2e_test.go:284`.
- The full vendored suite was not run because the required targeted gate failed.
- Verdict: REOPEN

### Repair response — Round 3, 2026-08-04

- `TestUsageStatsDisclosesDefaultedCacheCreationTTL` now uses the existing UTC display-clock seam, keeping the synthetic event inside the explicit local-day range on every host while retaining the real CLI text and JSON assertions.
- The documented `UPDATE_AGENTDECK_GOLDEN=1` test path regenerated `gui-json-contract.json`. Inspection confirmed the sole change adds the expected empty `usage.stats.data.warnings` field.
- Both focused tests pass without update mode. Uncached targeted `internal/usage` and `cmd/agentdeck` tests plus the full vendored suite pass. Review remains **REOPEN** pending independent re-review.

## Round 4 — 2026-08-04

- Reviewed state: `HEAD` `882da86adbc23fa685cf882d85b16a828bd332c2`; changed since Round 3: `cmd/agentdeck/main_test.go` SHA-256 `af700a625b076ff9a1b2b2b6e7e5fc783dc962151df70b949076258f61184260`, `cmd/agentdeck/testdata/phase7/gui-json-contract.json` `bb300c62b7e44d9cdbc0376cf630108e4e5923209c12329e3a6679f83423c22c`, and `docs/plans/credential-key-and-cache-pricing.md` `fc81646b08cc2d7f8fb68ac28172f113c35b035353419ac9052c38234b697fed`. Reviewed product and other test files retain their Round 3 hashes.
- Reviewer: Codex (re-review-only round; no product code, tests, or configuration changed).
- Scope: Round 3 M4 deterministic CLI date selection, M5 GUI JSON contract synchronization, and final L2 verification.
- M4 closure: the real CLI disclosure test calls `useUTCDisplayClock(t)` before creating and querying its fixture, so the explicit `2026-07-20` local day contains `2026-07-20T01:00:00Z` on every host. JSON still asserts the marker in both envelope and report data; text still asserts one visible marker.
- M5 closure: the generated GUI contract diff adds only `usage.stats.data.warnings: []`; the envelope warning field remains unchanged and no unrelated command schema moved.
- New findings: none.
- Verification:
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestUsageStatsDisclosesDefaultedCacheCreationTTL ./cmd/agentdeck` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 -run TestIsolatedEndToEndFlow ./cmd/agentdeck` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./internal/usage ./cmd/agentdeck` -> PASS.
  - `GOCACHE=/private/tmp/agent-deck-go-build go test -mod=vendor -count=1 ./...` -> PASS.
- Verdict: PASS
