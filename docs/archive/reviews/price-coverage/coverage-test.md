---
status: historical
plan: price-coverage
task: coverage-test
retired: 2026-07-23
---

# Review log — price-coverage / coverage-test

## Round 1 — 2026-07-23

- Reviewed state: same price-coverage worktree identified by SHA-256
  `f1061aabdd19cbf7baaddc384d9ab38d2e76ffb715ec8995741e34e6fcaa91c9`.
- Reviewer: Codex
- Scope: fresh-store bundled-only behavior, representative real-model
  coverage, deliberate exclusions, both vendors and punctuation aliases,
  anti-stub guard, cold-start measurement comparability, and residual claims.
- Findings:
  - [P1] The documented “7.5% to 93.6%” improvement is not a controlled
    before/after measurement. The 7.5% baseline used the earlier 5.19 B-token
    database state, while the new table uses a later snapshot: Spark alone
    changed from 69,406,886 to 77,509,947 tokens, and the new totals no longer
    equal the baseline total. The new 93.6% observation may be correct, but the
    causal delta cannot be attributed solely to the catalog while input data
    also changed. Run the old two-model bundled catalog and the new generated
    catalog against fresh copies of one frozen database snapshot with identical
    code, query, and layer stripping, record snapshot/catalog identities and
    exact commands, and recompute both percentages from that paired fixture.
    If the old artifact cannot be reproduced, relabel 7.5% as a non-comparable
    historical baseline and remove the “went from” claim.
- Evidence: the fresh-store unit test itself follows the real bundled import
  path without network and meaningfully protects the named models, deliberate
  Spark/pseudo-model exclusions, both vendors, and dotted Claude spellings.
  `TestBundledCatalogIsNotAStub` also guards breadth and the generated prefix.
  The finding is limited to the separately recorded real-data percentage and
  its causal wording, not those tests.
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state: repaired price-coverage worktree identified by SHA-256
  `1f9dc78452627729ef7e5d87ed789514468f3a04a494ad0f29c3bec9884982ed`.
- Reviewer: Codex
- Scope: closure of the same-input measurement finding, catalog-only
  isolation, arithmetic and residual attribution, and original test quality.
- Findings:
  - [closed] The replacement measurement uses fresh copies of one frozen
    snapshot for both sides, strips all catalog layers, imports either the old
    two-model or new 111-model bundled artifact, and then queries both with one
    shared binary and identical `--no-scan` arguments. Source inspection
    confirms the stats `--no-scan` path does not call `ImportBundledCatalog`,
    so the shared binary cannot contaminate the old side with its embedded new
    catalog.
  - [closed] Both sides report the same 5,259,503,075 input tokens. The paired
    fully-priced result is now 7.4% versus 93.6%; the old 7.5% is explicitly
    retained only as a non-comparable historical observation. Catalog and
    snapshot identities, isolation method, commands, matched coverage, and
    residual causes are recorded.
  - No new blocking or medium-severity findings in the fresh-store coverage
    tests or controlled measurement.
- Evidence: read-only inspection of the repaired measurement, bundled import
  call sites, stats `--no-scan` execution path, generated metadata, and the
  existing representative-model tests. The real database and session sources
  were not accessed during this review.
- Verdict: PASS

## Round 3 — 2026-07-23

- Reviewed state: explicit-file SHA-256
  `7d029feed3ac663eccaeb138bbe65afa4d42d4e98b898adc1822cc9c81e30ec9`.
- Findings: no regression in the representative bundled-only coverage guard or
  the paired-measurement account. Spark and `codex-auto-review` remain explicit
  exclusions rather than silently disappearing from the fixture.
- Evidence: current plan and coverage-test contract, explicit-file content
  identity, and a clean `rtk git diff --check`; the real state database and
  session sources were not accessed.
- Verdict: PASS
