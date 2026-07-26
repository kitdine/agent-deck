---
status: historical
retired: 2026-07-26
plan: repository-test-gaps
task: usage-price-refresh-failure-boundaries
---

# Review log — repository-test-gaps / usage-price-refresh-failure-boundaries

## Round 1 — 2026-07-23

- Reviewed state:
  `f4f1d366097db9bdd89a1620e3d1b48830da5c3a2282f324f7de9a66831a1573`
  (staged content manifest)
- Reviewer: `review_usage_price_refresh_r1` (`gpt-5.6-sol`, high)
- Scope: `internal/usage/usage_test.go`; HTTP and catalog-validation retry
  policy, cancellation, size bounds, and preservation of the selected catalog
- Findings:
  - [high] Retry counts referenced the production constant and sampled too few
    statuses, so a changed retry budget or misclassified ordinary 4xx could
    pass. Pin literal three attempts and the complete status boundary.
  - [medium] Oversize and cancellation cases began with empty state and did not
    prove that an existing usable catalog survives those failure branches.
  - [high] Valid JSON with no accepted direct-provider records was not required
    to fail once as a permanent validation error. Current production may retry
    this case; if reproduced, preserve RED as a task-local production blocker.
- Evidence: complete `internal/usage` package PASS; exact staged path and
  manifest verified; cancellation interaction was deterministic
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state:
  `0a5ce2e960c348c48820bec0040e38337c9087c048efcbf297c51717ed1e0965`
  (staged content manifest)
- Reviewer: `review_usage_price_refresh_r2` (`gpt-5.6-sol`, high)
- Scope: Round 1 closure, repaired retry/state/cancellation coverage, and
  independent failing-layer classification
- Findings: no remaining test-quality finding
- Prior finding closure:
  - Attempt counts use literal three and cover every 4xx/5xx status boundary.
  - Oversize and cancellation failures preserve a seeded price history,
    status, and selected price list.
  - Cancellation is interaction-driven and bounded to one second.
- Blocker:
  `usage-price-refresh-permanent-validation-retried`
  - Production marks every `liteLLMCatalog` error retryable.
  - Valid JSON with no accepted direct-provider records therefore makes three
    requests instead of one.
  - Exact failure:
    `usage_test.go:1087: catalog without direct providers requests = 3, want 1`
  - Scope: task-local; the other 14 independent Wave 1 tasks may continue.
  - Resume: separate production-fix workflow, then `new-baseline`.
- Evidence: exact staged path and manifest verified; focused reproduction and
  complete `internal/usage` package reproduce the same deterministic failure;
  no network, environment, fixture, or test defect found
- Verdict: BLOCKED

## New-baseline resume — 2026-07-24

- Historical verdict: Round 2 remains BLOCKED evidence for baseline
  `94437ab70273d90ff01dd19e9f64a9b358e2c709`; it is not rewritten as PASS.
- Production resolution:
  `571a0e3ba454e9789c0dae3932dc2e296bb684d8` narrowed retryable catalog
  parse failures and was delivered to local `main`.
- Resumed baseline:
  `4f614d34d09260a52df6bd333f6dad26134e96ac`
- Authorization package:
  `bbf49cd178e1223c0b10ee59ea60f13f3c2e80818d63aa2b2f4a666b861e0710`
- Old staged candidate: immutable evidence only; manifest
  `0a5ce2e960c348c48820bec0040e38337c9087c048efcbf297c51717ed1e0965`,
  patch SHA-256
  `979f2f603b62352879227cef812a769831ecb56b0716332dda4a2781cbf586b0`.
- Resume state at reconstruction: pending focused/package verification, staged
  manifest binding, and a fresh read-only review. No new-baseline PASS is
  claimed.

## New-baseline Round 1 — 2026-07-24

- Reviewed manifest:
  `0a5ce2e960c348c48820bec0040e38337c9087c048efcbf297c51717ed1e0965`
- Reviewer: `review_usage_newbaseline_r1` (`gpt-5.6-sol`, high)
- Finding: the candidate protected truncated JSON retries and semantic
  permanent failure, but did not prove that complete non-truncated malformed
  JSON is permanent. Treating every `json.SyntaxError` as retryable could still
  pass.
- Required repair: pinned malformed catalog body `}`, literal one request,
  parse error, and unchanged seeded history/status/effective prices.
- Evidence: exact staged path and manifest; focused tests PASS.
- Verdict: NEEDS-FIX

## New-baseline Round 2 — 2026-07-24

- Reviewed manifest:
  `f6e2b01be165b42ff5806a14ff291daab6967295e1a91f8b351505bcfc524bf4`
- Reviewer: `review_usage_newbaseline_r2` (`gpt-5.6-sol`, high)
- Prior finding closure: complete malformed `}` now returns after one request,
  exposes the syntax error, and preserves all seeded price state. Truncated
  JSON, semantic failure, status boundaries, cancellation identity, and exact
  counts remain protected.
- Verification: focused `TestUpdateLiteLLM`, complete `internal/usage`, and
  targeted cancellation `-race` PASS.
- Source task commit:
  `b8de77419943d32810ad6aef290a1f706a559185`
- Audit integration commit:
  `af41d5840bc78c299be5ea3049c599567d993125`
- Verdict: PASS

## Replacement Delivery Review Round 1 — 2026-07-26

- Reviewed manifest:
  `f6e2b01be165b42ff5806a14ff291daab6967295e1a91f8b351505bcfc524bf4`
- Replacement delivery parent:
  `4f614d34d09260a52df6bd333f6dad26134e96ac`
- Replacement delivery commit:
  `39650636fc92f884ecda5081f5d28ec22b583153`
- Findings: none.
- Verification: focused `TestUpdateLiteLLM`, complete `internal/usage`, and
  targeted cancellation `-race` PASS.
- Commit identity: exact authorized message, valid SSH signature, reviewed
  index/commit/audit manifest equality, and clean hook state.
- Production changes: none.
- Verdict: PASS
