---
status: historical
retired: 2026-07-26
plan: repository-test-gaps
task: watch-lifecycle-and-release
---

# Review log — repository-test-gaps / watch-lifecycle-and-release

## Round 1 — 2026-07-23

- Reviewed state:
  `f7cd3072498b2cd19172c54398eae727f0048adb3a2a6ab9a1d2e731638ac906`
  (staged content manifest)
- Reviewer: `review_watch_lifecycle_r1` (`gpt-5.6-sol`, high)
- Scope: `internal/watch/watch_test.go`; lock lifetime and release, combined
  errors, Run cancellation/emit failure, and missing-root fingerprints
- Findings:
  - [high] Single-source release counts did not prove the lock remained held
    through scan/persistence, cover nil persistence, or detect per-source
    release in a multi-source poll.
  - [medium] No success-plus-release-error case protected the final release
    branch.
  - [medium] Emit failure used `errors.Is` rather than requiring the exact
    callback error identity.
- Evidence: complete package PASS across 20 repeated runs and race PASS across
  10 runs; exact staged path and manifest verified; cancellation tests were
  race-free and channel-synchronized
- Verdict: REOPEN

## Round 2 — 2026-07-24

- Reviewed state:
  `0889f0f353edfdc93d9b1ce8d5204dc40a238e0d0ed82031604de4134df48cf2`
  (staged content manifest)
- Reviewer: `review_watch_lifecycle_r2` (`gpt-5.6-sol`, high)
- Scope: Round 1 closure plus the complete watch lifecycle candidate
- Findings: none
- Prior finding closure:
  - Explicit acquired/held/released state protects lock lifetime, nil
    persistence, two-source success, and later-source failure with one lock
    lifecycle for the complete poll.
  - An otherwise successful poll surfaces the final release error, while
    combined operation/release failures preserve both identities.
  - Emit failure requires the exact callback error identity.
- Evidence: complete `internal/watch` package PASS; package race count 10 PASS;
  exact staged path and manifest verified
- Source task commit:
  `d378ca08d4b537887fdaf1196a2d772bf8e63ace`
- Audit integrated commit:
  `23b8840e8af09ed69b39bf442c98e3e76a766c16`
- Signature and manifest: SSH signature GOOD; source and audit manifests both
  `0889f0f353edfdc93d9b1ce8d5204dc40a238e0d0ed82031604de4134df48cf2`
- Correction: the prior audit-transition text recorded an incorrect SHA; history
  is not rewritten. The actual signed audit commit is
  `23b8840e8af09ed69b39bf442c98e3e76a766c16`.
- Verdict: PASS
