---
status: historical
retired: 2026-07-26
plan: repository-test-gaps
task: backup-invalid-archive-no-target-mutation
---

# Review log — repository-test-gaps / backup-invalid-archive-no-target-mutation

## Round 1 — 2026-07-23

- Reviewed state:
  `5b4565ec9418c8aea1f58973c1b4b203b4f1363cd264cef8fff6aa76d5461d77`
  (staged content manifest)
- Reviewer: `review_backup_invalid_archive_r1` (`gpt-5.6-sol`, high)
- Scope: `internal/backup/backup_test.go`; invalid-archive validation order,
  target preservation, error classification, isolation, and secret leakage
- Findings:
  - [high] Writable targets plus final-state assertions could not detect target
    operations that happen before validation and are then perfectly rolled
    back. Add an unusable-target ordering probe that must still return
    `ErrInvalidArchive`.
  - [high] The archive contained no fake credential and the error was not
    checked for leakage. Add a synthetic credential and assert that neither it
    nor the synthetic passphrase appears in an error, without echoing those
    values in failure output.
- Evidence: focused invalid-archive test PASS; complete `internal/backup`
  package PASS; exact staged path and manifest verified; no production changes
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state:
  `e3ead06d9c1c03bade98aa2dea69cf233d34d924fbb92181973925f61a942192`
  (staged content manifest)
- Reviewer: `review_backup_invalid_archive_r2` (`gpt-5.6-sol`, high)
- Scope: Round 1 closure plus the complete
  `internal/backup/backup_test.go` candidate
- Findings: none
- Prior finding closure:
  - An invalid archive paired with a target below a regular-file parent still
    returns `ErrInvalidArchive`, proving validation happens before target-path
    operations; the parent file remains unchanged.
  - A valid source archive includes a synthetic credential before a later
    structural corruption; errors contain neither that value nor the synthetic
    passphrases, and failure messages do not echo them.
- Evidence: focused invalid-archive test PASS; complete `internal/backup`
  package PASS; exact staged path and manifest verified
- Source task commit:
  `b4391a67d0f5d1205d464904297cf63b65d13c35`
- Audit integrated commit:
  `3960db50ef5baa5d3aa3d558439b507a5bc80709`
- Signature and manifest: SSH signature GOOD; source and audit manifests both
  `e3ead06d9c1c03bade98aa2dea69cf233d34d924fbb92181973925f61a942192`
- Verdict: PASS
