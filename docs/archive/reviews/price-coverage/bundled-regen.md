---
status: historical
plan: price-coverage
task: bundled-regen
retired: 2026-07-23
---

# Review log — price-coverage / bundled-regen

## Round 1 — 2026-07-23

- Reviewed state: same price-coverage worktree identified by SHA-256
  `f1061aabdd19cbf7baaddc384d9ab38d2e76ffb715ec8995741e34e6fcaa91c9`.
- Reviewer: Codex
- Scope: pinned-source generation, reuse of the runtime LiteLLM conversion,
  deterministic artifact production, stable effective date, curated merge,
  provenance separation, reproducibility command, release ergonomics, and
  regression coverage.
- Findings:
  - [P1] The completion note and tool comment say the generator logic is
    unit-tested without network, but no test calls `GenerateBundledCatalog`,
    `tools/genprices.run`, or the reproducibility path. Current tests inspect
    the already-generated artifact; they do not protect the generator's key
    transformations. A regression in stable `effective_from`, curated merge,
    `sources` versus `generated_from`, filtering, deterministic bytes, or
    version stamping can therefore remain green until someone performs a
    network regeneration. Add a synthetic LiteLLM fixture test that invokes
    `GenerateBundledCatalog` twice and pins filtering, both vendors, stable
    dates, curated overwrite, provenance fields, byte determinism, and version
    verification; add focused tests for invalid/unpinned commits and the check
    path.
  - [P2] `make check-prices-reproducible` does not self-discover the recorded
    commit as the completion note claims. The Makefile shells out to `python3`
    to read `generated_from[0].commit_sha`, introducing an undeclared runtime
    dependency into an otherwise Go release tool; `genprices -check` with an
    omitted commit instead resolves current `main`, which is not the committed
    artifact's pin. Move recorded-commit discovery into the Go tool for check
    mode, remove the Python subprocess, and test malformed/missing/valid
    `generated_from` metadata. Also correct the tool comment's `COMMIT=` example
    to the actual `LITELLM_COMMIT=` Make variable.
- Evidence: read-only inspection of `bundled_catalog_gen.go`, the full
  `tools/genprices` command, Make targets, all price-coverage tests, generated
  metadata, spec, and completion note. The generated catalog currently has
  111 models from both providers, excludes Spark, carries the expected bundled
  source sentinel and pinned LiteLLM origin, and passes the recorded L3 checks;
  those current-artifact facts do not replace missing generator regression
  coverage.
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state: repaired price-coverage worktree identified by SHA-256
  `1f9dc78452627729ef7e5d87ed789514468f3a04a494ad0f29c3bec9884982ed`.
- Reviewer: Codex
- Scope: closure of generator and check-path coverage findings, pin discovery,
  Makefile portability, completion-note accuracy, and newly introduced issues.
- Findings:
  - [closed] `bundled_catalog_gen_test.go` now invokes the real generator over
    a synthetic upstream document and protects direct-provider filtering,
    exact per-million conversion, stable dates, curated merge/override,
    provenance separation, deterministic bytes, version stamping, commit
    validation, and unusable upstream rejection. The recorded effective-date
    mutation supplies a meaningful RED check.
  - [partially closed] The Python subprocess is removed, the Make variable is
    corrected, and `RecordedLiteLLMCommit` has good pure-helper coverage for
    valid, missing, malformed, ambiguous, and invalid pins. However Round 1
    also required a no-network test of the actual check path. There is still no
    `tools/genprices/main_test.go`, and no test invokes `run(..., check=true)`;
    therefore argument/path wiring from check mode through artifact pin
    discovery, avoidance of the latest-main endpoint, recorded-snapshot fetch,
    and byte comparison remains unprotected. Add an injectable fetch/client
    seam or a smaller command orchestration helper and test the full check-mode
    path with temporary artifact/gap-fill files and a fake fetcher, including
    success, output mismatch, malformed recorded metadata, and proof that the
    latest-main resolver is not called.
- Evidence: read-only inspection of the new generator tests, recorded-commit
  helper/tests, Make target, full `tools/genprices` command, and repository
  search confirming the tool directory still contains only `main.go`.
- Verdict: REOPEN

## Round 3 — 2026-07-23

- Reviewed state: explicit-file SHA-256
  `7d029feed3ac663eccaeb138bbe65afa4d42d4e98b898adc1822cc9c81e30ec9`.
- Reviewer: Codex
- Scope: closure of the actual check-path coverage finding, network-call
  ordering, pin selection, byte comparison, and counterpart command modes.
- Findings:
  - [closed] `tools/genprices` now has one injectable `fetcher` seam, and the
    tests drive the real `run(..., check=true)` orchestration rather than only
    its metadata helper. With no explicit commit, the test requires exactly one
    recorded-snapshot request and explicitly rejects any latest-main request.
  - [closed] The suite proves a byte-identical artifact passes, a hand-edited
    artifact fails, and missing, malformed, empty, branch-name, or ambiguous
    pins fail before any network request. Counterpart cases prove an explicit
    commit is honored and unpinned write mode still resolves latest main and
    records the resolved SHA.
  - [closed] Invalid curated input is rejected before network access, preserving
    the curation gate at the command boundary as well as in the helper.
  - No new blocking or medium-severity finding was found in the repaired
    command orchestration or its tests.
- Evidence: read-only inspection of `tools/genprices/main.go` and
  `main_test.go`, the recorded mutation RED/GREEN evidence, explicit-file
  content identity, and a clean `rtk git diff --check`. The existing broader
  build evidence was not mechanically rerun during re-review.
- Verdict: PASS
