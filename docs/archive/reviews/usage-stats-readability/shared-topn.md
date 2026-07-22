---
status: historical
plan: usage-stats-readability
task: shared-topn
retired: 2026-07-22
---

# Review log — usage-stats-readability / shared-topn

## Round 1 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `041e1aa881ced5e566a4f39adb3455d200944b2ff389897f17151689c8b78743`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md`.
- Reviewer: Codex
- Scope: shared top-N helper and footer behavior for MODELS, PROVIDERS,
  UNPRICED MODELS, per-model CACHE, and the refactored cache-session cap;
  preservation of CLIENTS and JSON completeness; regression-test strength;
  completion-note and status accuracy.
- Findings:
  - [P2] The four new cap tests assert only the rendered row count and footer.
    They do not assert that the retained rows are the existing-order prefix or
    that row N+1 is absent. An implementation that renders the wrong eight or
    twelve rows can still pass. Add retained-boundary and omitted-tail
    assertions for every newly capped section.
  - [P2] The `OnlyInText` tests use the caller-side report slice length as a
    proxy for JSON completeness, but do not execute and decode the actual usage
    stats JSON output. That assertion can remain green if output routing later
    serializes truncated data. Exercise the existing JSON output path with the
    same fixtures and require every row, including the text-omitted tail, to be
    present.
- Evidence: read-only inspection of the relevant production, test, and plan
  diffs. No tests were rerun in this review round; the development evidence
  recorded in the plan was not independently revalidated here.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `bc51b8fd9b506226a01232cd9ec6a9f5602d79ecb537797ad35f5e3bd010b919`
  over the relevant diff in `cmd/agentdeck/usage_stats_text.go`,
  `cmd/agentdeck/usage_stats_text_test.go`, and
  `docs/plans/usage-stats-readability.md` before this verdict update.
- Reviewer: Codex
- Scope: closure of both Round 1 P2 findings, regression value of the repaired
  text-cap tests, actual JSON-envelope completeness, and newly introduced
  problems in the shared-topn scope.
- Findings:
  - [closed] Each newly capped section now requires every row in the original
    prefix through N to be present and row N+1 to be absent. The recorded RED
    mutation shifts the selected window and makes all four assertions fail, so
    the tests protect the ordering contract rather than only the row count.
  - [closed] `usageStatsJSONReport` executes `writeUsageEnvelope` with the JSON
    format, decodes the envelope, and verifies the complete row count plus the
    text-omitted tail for models, providers, unpriced models, per-model cache,
    and cache sessions.
  - New findings: none.
- Evidence: read-only inspection of the repaired production, test, and plan
  diffs. The repair's targeted package test, full `./...` test, package vet,
  RED mutation, and `git diff --check` results are recorded in the plan and
  bind to the reviewed relevant content; this round did not mechanically rerun
  them.
- Verdict: PASS
