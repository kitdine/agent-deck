---
status: historical
retired: 2026-07-26
plan: repository-test-gaps
task: session-index-transition-atomicity
---

# Review log — repository-test-gaps / session-index-transition-atomicity

## Round 1 — 2026-07-23

- Reviewed state:
  `b734b02b2fbedfc7952ab30038c869d7fde41611b3eebb5341843ea04bc5bf06`
  (staged content manifest)
- Reviewer: `review_session_atomicity_r1` (`gpt-5.6-sol`, high)
- Scope: `internal/session/session_test.go`; failed ReplaceDocuments, Exclude,
  and Rebuild transitions plus successful project/path/session/client
  exclusion boundaries
- Findings:
  - [high] ReplaceDocuments was seeded before failure injection, so the
    transaction-external synthetic source insert on a first failed call was not
    protected.
  - [medium] Rebuild used one source and no exclusions, so it did not prove
    preservation after an earlier source succeeded or detect exclusion loss.
- Confirmed production RED candidates:
  - Exclude commits its control row and document deletion before a later
    metadata-delete failure.
  - Rebuild clears the prior index before a fallible scan.
  - Project/path exclusion over-deletes fallback-source documents by matching
    only `(client, session_id)`.
- Evidence: existing-source ReplaceDocuments and session/client exclusion
  boundaries PASS; focused and package tests reproduce only Exclude/Rebuild and
  project/path failures; exact staged path and manifest verified
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state:
  `cce7277dfd7006e3d3eb366360d7516822dedbc92488398d31574889f3262d0b`
  (staged content manifest)
- Reviewer: `review_session_atomicity_r2` (`gpt-5.6-sol`, high)
- Scope: Round 1 closure, complete candidate, deterministic repeated
  reproduction, and independent failing-layer classification
- Findings: no remaining test-quality finding
- Prior finding closure:
  - A fresh-store first-insert failure detects the transaction-external
    synthetic source leak and compares all four tables plus List/Search.
  - Rebuild uses two deterministically ordered real sources, retains a seeded
    exclusion, and fails on the later source only after earlier metadata exists.
- Blocker:
  `session-index-atomic-transitions-and-source-boundaries`
  - First failed ReplaceDocuments leaks the synthetic source.
  - Failed Exclude and Rebuild leave partial index state.
  - Project/path exclusions over-delete a fallback-source document sharing the
    same client/session identity.
  - Scope: task-local; unrelated frozen tasks remain safe to continue.
  - Resume: separately deliver the production fix, then `new-baseline`.
- Evidence: the five failing cases and three passing controls reproduced
  identically across three runs; the complete package is RED only for the
  confirmed production behavior; exact staged path and manifest verified
- Verdict: BLOCKED

## New-baseline resume — 2026-07-24

- Historical verdict: Round 2 remains BLOCKED evidence for baseline
  `94437ab70273d90ff01dd19e9f64a9b358e2c709`; it is not rewritten as PASS.
- Production resolution:
  `3c80e4a9ad025375d337a7ef8f9cda065bc797f5` made index transitions
  atomic and enforced exact source ownership and was delivered to local
  `main`.
- Resumed baseline:
  `4f614d34d09260a52df6bd333f6dad26134e96ac`
- Authorization package:
  `bbf49cd178e1223c0b10ee59ea60f13f3c2e80818d63aa2b2f4a666b861e0710`
- Old staged candidate: immutable evidence only; manifest
  `cce7277dfd7006e3d3eb366360d7516822dedbc92488398d31574889f3262d0b`,
  patch SHA-256
  `ecdda189118e27eb9bfb1e1681b560130651c66822ca2bbf7c8d3e3580c92fe3`.
- Resume state at reconstruction: pending focused/package verification, staged
  manifest binding, and a fresh read-only review. No new-baseline PASS is
  claimed.

## New-baseline Round 1 — 2026-07-24

- Reviewed manifest:
  `cce7277dfd7006e3d3eb366360d7516822dedbc92488398d31574889f3262d0b`
- Reviewer: `review_session_newbaseline_r1` (`gpt-5.6-sol`, high)
- Findings:
  - Rename checked only metadata `SourcePath`; it could lose moved documents
    and FTS visibility while still passing.
  - Missing-source controls checked joined visibility but could leave orphaned
    physical document/FTS rows.
- Evidence: focused transition/control tests `-count=3` and complete package
  PASS; exact staged path and manifest.
- Verdict: NEEDS-FIX

## New-baseline Round 2 — 2026-07-24

- Reviewed manifest:
  `e57a1574a7c2a52eea5427c7edd7e2bb65d95434a2a8b4cdc6eed58f726bc4cc`
- Reviewer: `review_session_newbaseline_r2` (`gpt-5.6-sol`, high)
- Prior finding closure:
  - Rename now proves exact old/new ownership across sources, metadata, and
    documents plus `Show.Documents` and FTS `Search` visibility.
  - Single, fallback, duplicate, and final missing-source transitions prove
    exact physical cleanup or retention across all three tables and search.
- Verification: ten focused transitions `-count=3` and complete
  `internal/session` PASS.
- Source task commit:
  `3a6b7aa48f3f213a9b262fde024f2d44d912651d`
- Audit integration commit:
  `2da75f63dcbe8cf7e829c97d5ddfabf6696ad028`
- Verdict: PASS

## Replacement Delivery Review Round 1 — 2026-07-26

- Reviewed manifest:
  `e57a1574a7c2a52eea5427c7edd7e2bb65d95434a2a8b4cdc6eed58f726bc4cc`
- Replacement delivery parent:
  `02eec76513929fb321361858a00cc71d9ecad387`
- Replacement delivery commit:
  `7168079230adf8bb1fdf05b2d563f1f1782023e1`
- Findings: none.
- Verification: ten focused transition/control scenarios `-count=3` and
  complete `internal/session` PASS.
- Protected boundary: full rollback for ReplaceDocuments, Exclude, and Rebuild;
  exact project/path/session/client ownership; fallback, rename,
  equal-length-rewrite, missing-source, List/Show/Search, and FTS visibility.
- Commit identity: exact authorized message, valid SSH signature, reviewed
  index/commit/audit manifest equality, and clean hook state.
- Production changes: none.
- Verdict: PASS
