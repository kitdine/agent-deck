---
status: historical
retired: 2026-07-26
plan: repository-test-gaps
task: genprices-network-commit-errors
---

# Review log — repository-test-gaps / genprices-network-commit-errors

## Round 1 — 2026-07-23

- Reviewed state:
  `fd9d01abdf5debf438769739306552f25b8acc7d777bec0db84c07d5e77dbbdb`
  (staged content manifest)
- Reviewer: `review_genprices_r1` (`gpt-5.6-sol`, high)
- Scope: `tools/genprices/main_test.go`; HTTP transport/status/size failures,
  upstream commit resolution, invalid catalog generation, and output
  preservation
- Findings:
  - [blocking] malformed latest-commit JSON is returned without
    commit-resolution context, and non-SHA `main` is accepted long enough to
    make a second catalog request.
  - [medium] the non-SHA case expected downstream generator wording instead of
    the resolver-layer invalid-SHA contract.
  - [medium] `maxCatalogBytes+1` was rejected, but exact
    `maxCatalogBytes` acceptance was not protected.
- Blocker candidate:
  `genprices-latest-commit-resolver-validation-001`
  - Expected: classify malformed commit JSON, reject empty/non-SHA revisions
    after only the commit API request, and preserve the existing output.
  - Actual: malformed JSON is returned raw; `main` is used for a catalog
    request before downstream rejection; output remains unchanged.
  - Scope: task-local production defect, pending test-only Round 1 corrections
    and fresh re-review.
  - Resume if confirmed: separate production-fix workflow, then
    `new-baseline`.
- Evidence: HTTP transport/non-2xx/oversize and invalid-catalog tests PASS;
  focused latest-commit and package tests reproduce only the two deterministic
  failures; exact staged path and manifest verified
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state:
  `2f2e9c4cff9cbec5678d8ebbc69a606e024e8ae2fae49638c8e0a52b339266d7`
  (staged content manifest)
- Reviewer: `review_genprices_r2` (`gpt-5.6-sol`, high)
- Scope: Round 1 closure, complete failure candidate, suite-level boundary
  mutation analysis, and independent failing-layer classification
- Findings:
  - [blocking] `latestCommit` returns malformed JSON errors without
    resolver-layer context and accepts non-SHA `main`, allowing a catalog
    request before downstream rejection.
- Prior finding closure:
  - Non-SHA input now requires the exact resolver-layer invalid-SHA error while
    retaining one-request and byte-preservation assertions.
  - Exact `maxCatalogBytes` acceptance and `maxCatalogBytes+1` rejection are
    both protected. Together their exact assertions catch off-by-one rejection
    and truncated-reader mutations.
- Blocker:
  `genprices-latest-commit-resolver-validation-001`
  - Expected: classify malformed commit JSON and reject non-SHA revisions
    before any catalog request.
  - Actual: malformed JSON is raw; `main` reaches catalog generation and
    retrieval before rejection.
  - Scope: task-local; unrelated frozen tasks remain safe to continue.
  - Resume: separately deliver the production fix, then `new-baseline`.
- Evidence: exact-size, HTTP-failure, and invalid-catalog tests PASS; focused
  latest-commit and complete package runs are RED only for the two confirmed
  production behaviors; exact staged path and manifest verified
- Verdict: BLOCKED

## New-baseline resume — 2026-07-24

- Historical verdict: Round 2 remains BLOCKED evidence for baseline
  `94437ab70273d90ff01dd19e9f64a9b358e2c709`; it is not rewritten as PASS.
- Production resolution:
  `c4abf8700757c5429b6c24d139b077dde01a0183` added resolver error
  classification and SHA validation and was delivered to local `main`.
- Resumed baseline:
  `4f614d34d09260a52df6bd333f6dad26134e96ac`
- Authorization package:
  `bbf49cd178e1223c0b10ee59ea60f13f3c2e80818d63aa2b2f4a666b861e0710`
- Old staged candidate: immutable evidence only; manifest
  `2f2e9c4cff9cbec5678d8ebbc69a606e024e8ae2fae49638c8e0a52b339266d7`,
  patch SHA-256
  `ff6c4980dfe7b024603a24ac4a1110fcda80c8fcde2e1c55b6d455a0d0dcf256`.
- Resume state at reconstruction: pending focused/package verification, staged
  manifest binding, and a fresh read-only review. No new-baseline PASS is
  claimed.

## New-baseline Round 1 — 2026-07-24

- Reviewed manifest:
  `2f2e9c4cff9cbec5678d8ebbc69a606e024e8ae2fae49638c8e0a52b339266d7`
- Reviewer: `review_genprices_newbaseline_r1` (`gpt-5.6-sol`, high)
- Findings:
  - Latest-commit transport failure did not prove resolver-layer wrapping,
    sentinel identity, one commit-API request, and unchanged output.
  - An explicit invalid pin did not prove zero network requests and unchanged
    output before downstream generation.
- Evidence: focused `TestRun` and complete package PASS; exact staged path and
  manifest.
- Verdict: NEEDS-FIX

## New-baseline Round 2 — 2026-07-24

- Reviewed manifest:
  `5138ed04b1a4b00ca67c9c4558acd29f7c9b7ccd680f2c8c4b7780a9fad7bab7`
- Reviewer: `review_genprices_newbaseline_r2` (`gpt-5.6-sol`, high)
- Prior finding closure:
  - Sentinel transport failure now preserves `errors.Is`, exact resolver
    context, one commit URL request, and output bytes.
  - Explicit invalid `main` is rejected with the exact validation error before
    any fetch and preserves output bytes.
- Verification: `TestRun`, `TestCheckMode`, and complete `tools/genprices`
  PASS.
- Source task commit:
  `f35c654006e14b5ac285096aa5a548fbe36757d8`
- Audit integration commit:
  `9fb887325b4c7afdd3c06c71c1b4ca4cd48b2fed`
- Verdict: PASS

## Delivery Task Review Round 1 — 2026-07-25

- Reviewed delivery head:
  `725ab5aed94c3a38d7f9c8d7ebc8016e63569b33`
- Reviewed manifest:
  `5138ed04b1a4b00ca67c9c4558acd29f7c9b7ccd680f2c8c4b7780a9fad7bab7`
- Finding:
  - [high] Four failed check-mode paths asserted their error and network
    behavior but never reread `outPath`. A broken implementation could
    overwrite, truncate, empty, or replace the artifact before returning the
    expected error while every test remained green.
  - Affected paths:
    `TestCheckModeFailsWhenArtifactDiffersFromRegeneration`, all five
    `TestCheckModeFailsBeforeAnyNetworkWhenPinIsUnusable` subtests,
    `TestCheckModeHonorsAnExplicitCommit`, and
    `TestCheckModeRejectsAGapfillThatFailsTheCurationBar`.
- Confirmed contracts: latest-commit error context and sentinel identity,
  invalid explicit-pin zero-network rejection, exact size boundary, request
  URLs/counts, and existing failure-output checks remained correct.
- Failed delivery disposition: branch/worktree/head and staged candidate are
  retained immutable evidence; no in-place delivery repair or reuse.
- Verdict: NEEDS-FIX

## New-baseline Round 3 — 2026-07-25

- Reviewed manifest:
  `82e983adca576925eccb8832355a30d86ede4c88614fb8c4f4c50fede6d33972`
- Prior finding closure:
  - `readArtifact` captures the exact pre-call artifact bytes.
  - `requireArtifactBytes` rereads and compares exact post-call bytes.
  - Every confirmed failing check-mode path now snapshots before `run` and
    asserts exact byte preservation after the expected error.
  - Existing error, request, and zero-network assertions remain intact.
- Verification: focused `TestCheckMode` and complete `tools/genprices` PASS;
  exact staged path, no unstaged owned content, no untracked content, diff
  check, and gofmt PASS.
- Source task repair commit:
  `e825d24a00c18917ea6025ba1ab125a2a447b662`
- Audit test integration commit:
  `f65856d2f5da0569b76697bec13544f978985352`
- Production changes: none.
- Residual uncertainty: transient writes restored to identical bytes and
  metadata-only changes are outside the byte-preservation contract.
- Verdict: PASS

## Aggregate Review Round 3 — 2026-07-25

- Reviewed audit head:
  `aab3d418e69eb92559a76a46db125597ad48aaa4`
- Reviewed tree:
  `f0e423001e2a8dbd4f559264c53d42fc32594e14`
- Scope: exact 21 paths, comprising 17 audit documents and four test files;
  production code unchanged.
- Finding closure: the failed-delivery artifact-preservation gap is closed by
  the Round 3 candidate; all four source-to-audit manifests, messages, parents,
  and SSH signatures pass.
- Ledger and review state: 16 modules, 15 tasks, every task review terminal
  PASS, no exclusion, unconfirmed module, needs-tests entry, open blocker, or
  awaiting-human state.
- Verification: fresh full tests, full race, vet, atomic repository coverage,
  and diff check PASS. Coverage is 81.9%; fresh reviewer-run profile SHA-256
  `bf63621bc88000f58c1b91df87f7d2feb1c7496940a4d373bf79eb3b887f622e`.
- Failed delivery remains intact at
  `725ab5aed94c3a38d7f9c8d7ebc8016e63569b33`, with its staged manifest
  `5138ed04b1a4b00ca67c9c4558acd29f7c9b7ccd680f2c8c4b7780a9fad7bab7`.
- Residual uncertainty: final byte preservation does not observe transient
  writes restored to identical bytes or metadata-only changes; no blocker
  follows from that bounded contract.
- Replacement delivery eligibility: yes, after this PASS state is recorded by
  the reviewed Commit E/F sequence.
- Verdict: PASS

## Replacement Delivery Review Round 1 — 2026-07-26

- Reviewed manifest:
  `82e983adca576925eccb8832355a30d86ede4c88614fb8c4f4c50fede6d33972`
- Replacement delivery parent:
  `3968d703fc5ed94378fbb917c187543655a1ffbb`
- Replacement delivery commit:
  `02eec76513929fb321361858a00cc71d9ecad387`
- Findings: none.
- Verification: focused `TestRun`, focused `TestCheckMode`, and complete
  `tools/genprices` PASS.
- Commit identity: exact authorized message, valid SSH signature, reviewed
  index/commit/source-repair/audit manifest equality, and clean hook state.
- Production changes: none.
- Residual uncertainty: byte equality does not observe transient writes
  restored to the same bytes or metadata-only changes; no blocker follows.
- Verdict: PASS
